package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/mtproxy"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/dnstm/internal/types"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var instanceCmd = &cobra.Command{
	Use:   "instance",
	Short: "Manage transport instances",
	Long:  "Manage individual transport instances in the DNS tunnel router",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return requireInstalled()
	},
}

var instanceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all instances",
	Long:  "List all configured transport instances",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		if !router.IsInitialized() {
			return fmt.Errorf("router not initialized. Run 'dnstm router init' first")
		}

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if len(cfg.Transports) == 0 {
			fmt.Println("No instances configured")
			return nil
		}

		fmt.Println()
		modeName := router.GetModeDisplayName(cfg.Mode)
		fmt.Printf("Mode: %s\n\n", modeName)
		fmt.Printf("%-16s %-24s %-8s %-20s %s\n", "NAME", "TYPE", "PORT", "DOMAIN", "STATUS")
		fmt.Println(strings.Repeat("-", 80))

		for name, transport := range cfg.Transports {
			instance := router.NewInstance(name, transport)
			status := "Stopped"
			if instance.IsActive() {
				status = "Running"
			}

			// Add marker for active/default instance
			marker := ""
			if cfg.IsSingleMode() && cfg.Single.Active == name {
				marker = " *"
			} else if cfg.IsMultiMode() && cfg.Routing.Default == name {
				marker = " (default)"
			}

			typeName := types.GetTransportTypeDisplayName(transport.Type)
			fmt.Printf("%-16s %-24s %-8d %-20s %s%s\n", name, typeName, transport.Port, transport.Domain, status, marker)
		}

		if cfg.IsSingleMode() {
			fmt.Println("\n* = active instance")
		}
		fmt.Println()

		return nil
	},
}

var instanceAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new instance",
	Long:  "Add a new transport instance interactively or via flags",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		if !router.IsInitialized() {
			return fmt.Errorf("router not initialized. Run 'dnstm router init' first")
		}

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Check flags for non-interactive mode
		transportType, _ := cmd.Flags().GetString("type")
		domain, _ := cmd.Flags().GetString("domain")

		if transportType != "" && domain != "" {
			// Non-interactive mode
			return addInstanceNonInteractive(cmd, args, cfg)
		}

		// Interactive mode
		return addInstanceInteractive(args, cfg)
	},
}

func addInstanceInteractive(args []string, cfg *router.Config) error {
	fmt.Println()
	tui.PrintInfo("Adding new transport instance...")
	fmt.Println()

	// Select transport type (ask first)
	transportType, err := tui.RunMenu(tui.MenuConfig{
		Title: "Transport Type",
		Options: []tui.MenuOption{
			{Label: "Slipstream + Shadowsocks (Recommended)", Value: string(types.TypeSlipstreamShadowsocks)},
			{Label: "Slipstream SOCKS", Value: string(types.TypeSlipstreamSocks)},
			{Label: "Slipstream SSH", Value: string(types.TypeSlipstreamSSH)},
			{Label: "Slipstream + MTProxy (Telegram)", Value: string(types.TypeSlipstreamMTProxy)},
			{Label: "DNSTT SOCKS", Value: string(types.TypeDNSTTSocks)},
			{Label: "DNSTT SSH", Value: string(types.TypeDNSTTSSH)},
			{Label: "DNSTT + MTProxy (Telegram, via socat)", Value: string(types.TypeDNSTTMTProxy)},
		},
	})
	if err != nil {
		return err
	}
	if transportType == "" {
		return nil
	}

	// Install required binaries if needed
	if err := transport.EnsureBinariesInstalled(types.TransportType(transportType)); err != nil {
		return fmt.Errorf("failed to install required binaries: %w", err)
	}

	// Get or generate name
	var name string
	if len(args) > 0 {
		name = args[0]
	}

	suggestedName := router.GenerateUniqueName(cfg.Transports)
	if name == "" {
		var confirmed bool
		name, confirmed, err = tui.RunInput(tui.InputConfig{
			Title:       "Instance Name",
			Description: fmt.Sprintf("Leave empty for auto-generated name (%s)", suggestedName),
			Placeholder: suggestedName,
		})
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
		if name == "" {
			name = suggestedName
		}
	}

	name = router.NormalizeName(name)
	if err := router.ValidateName(name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}

	if _, exists := cfg.Transports[name]; exists {
		return fmt.Errorf("instance %s already exists", name)
	}

	// Get domain
	var domain string
	for {
		var confirmed bool
		domain, confirmed, err = tui.RunInput(tui.InputConfig{
			Title:       "Domain",
			Description: "e.g., t1.example.com",
		})
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
		if domain == "" {
			tui.PrintError("Domain is required")
			continue
		}
		break
	}

	// Build transport config
	transportCfg := &types.TransportConfig{
		Type:   types.TransportType(transportType),
		Domain: domain,
	}

	// Type-specific configuration
	microsocksAddr := cfg.GetMicrosocksAddress()
	switch types.TransportType(transportType) {
	case types.TypeSlipstreamShadowsocks:
		if err := configureSlipstreamShadowsocks(transportCfg); err != nil {
			return err
		}
	case types.TypeSlipstreamSocks:
		if err := configureSlipstreamSocks(transportCfg, microsocksAddr); err != nil {
			return err
		}
	case types.TypeSlipstreamSSH:
		if err := configureSlipstreamSSH(transportCfg); err != nil {
			return err
		}
	case types.TypeSlipstreamMTProxy:
		if err := configureSlipstreamMTProxy(transportCfg); err != nil {
			return err
		}
	case types.TypeDNSTTSocks:
		if err := configureDNSTTSocks(transportCfg, microsocksAddr); err != nil {
			return err
		}
	case types.TypeDNSTTSSH:
		if err := configureDNSTTSSH(transportCfg); err != nil {
			return err
		}
	case types.TypeDNSTTMTProxy:
		if err := configureDNSTTMTProxy(transportCfg); err != nil {
			return err
		}
		tui.PrintStatus("Installing socat bridge for DNSTT...")
		if err := mtproxy.InstallBridge(); err != nil {
			return fmt.Errorf("failed to install MTProxy bridge: %w", err)
		}
	}

	// Allocate port
	if cfg.Transports == nil {
		cfg.Transports = make(map[string]*types.TransportConfig)
	}
	port, err := router.AllocatePort(cfg.Transports)
	if err != nil {
		return fmt.Errorf("failed to allocate port: %w", err)
	}
	transportCfg.Port = port

	// Create the instance
	return createInstance(name, transportCfg, cfg)
}

func addInstanceNonInteractive(cmd *cobra.Command, args []string, cfg *router.Config) error {
	transportType, _ := cmd.Flags().GetString("type")
	domain, _ := cmd.Flags().GetString("domain")
	targetAddr, _ := cmd.Flags().GetString("target")
	password, _ := cmd.Flags().GetString("password")
	method, _ := cmd.Flags().GetString("method")

	// Get name
	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		name = router.GenerateUniqueName(cfg.Transports)
	}

	name = router.NormalizeName(name)
	if err := router.ValidateName(name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}

	if _, exists := cfg.Transports[name]; exists {
		return fmt.Errorf("instance %s already exists", name)
	}

	// Install required binaries if needed
	if err := transport.EnsureBinariesInstalled(types.TransportType(transportType)); err != nil {
		return fmt.Errorf("failed to install required binaries: %w", err)
	}

	// Build config
	transportCfg := &types.TransportConfig{
		Type:   types.TransportType(transportType),
		Domain: domain,
	}

	microsocksAddr := cfg.GetMicrosocksAddress()
	switch types.TransportType(transportType) {
	case types.TypeSlipstreamShadowsocks:
		if password == "" {
			password = generatePassword()
		}
		if method == "" {
			method = "aes-256-gcm"
		}
		transportCfg.Shadowsocks = &types.ShadowsocksConfig{
			Password: password,
			Method:   method,
		}
	case types.TypeSlipstreamSocks:
		// Auto-use microsocks unless explicitly specified
		if targetAddr == "" {
			if microsocksAddr == "" {
				return fmt.Errorf("microsocks not configured. Run 'dnstm install' first")
			}
			targetAddr = microsocksAddr
		}
		transportCfg.Target = &types.TargetConfig{Address: targetAddr}
	case types.TypeSlipstreamSSH:
		if targetAddr == "" {
			targetAddr = "127.0.0.1:" + osdetect.DetectSSHPort()
		}
		transportCfg.Target = &types.TargetConfig{Address: targetAddr}
	case types.TypeDNSTTSocks:
		// Auto-use microsocks unless explicitly specified
		if targetAddr == "" {
			if microsocksAddr == "" {
				return fmt.Errorf("microsocks not configured. Run 'dnstm install' first")
			}
			targetAddr = microsocksAddr
		}
		transportCfg.Target = &types.TargetConfig{Address: targetAddr}
		transportCfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
	case types.TypeDNSTTSSH:
		if targetAddr == "" {
			targetAddr = "127.0.0.1:" + osdetect.DetectSSHPort()
		}
		transportCfg.Target = &types.TargetConfig{Address: targetAddr}
		transportCfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
	case types.TypeSlipstreamMTProxy:
		transportCfg.Target = &types.TargetConfig{Address: fmt.Sprintf("%s:%s", mtproxy.MTProxyBindAddr, mtproxy.MTProxyPort)}
	case types.TypeDNSTTMTProxy:
		transportCfg.Target = &types.TargetConfig{Address: fmt.Sprintf("%s:%s", mtproxy.MTProxyBindAddr, mtproxy.MTProxyBridgePort)}
		transportCfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
	default:
		return fmt.Errorf("unknown transport type: %s", transportType)
	}

	// Allocate port
	if cfg.Transports == nil {
		cfg.Transports = make(map[string]*types.TransportConfig)
	}
	port, err := router.AllocatePort(cfg.Transports)
	if err != nil {
		return fmt.Errorf("failed to allocate port: %w", err)
	}
	transportCfg.Port = port

	return createInstance(name, transportCfg, cfg)
}

func createInstance(name string, transportCfg *types.TransportConfig, cfg *router.Config) error {
	fmt.Println()

	totalSteps := 4
	currentStep := 0

	// Step 1: Generate certificates/keys
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Generating cryptographic material...")
	var fingerprint string
	var publicKey string
	if types.IsSlipstreamType(transportCfg.Type) {
		certMgr := certs.NewManager()
		certInfo, err := certMgr.GetOrCreate(transportCfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to generate certificate: %w", err)
		}
		fingerprint = certInfo.Fingerprint
		tui.PrintStatus("TLS certificate ready")
	} else if types.IsDNSTTType(transportCfg.Type) {
		keyMgr := keys.NewManager()
		keyInfo, err := keyMgr.GetOrCreate(transportCfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
		}
		publicKey = keyInfo.PublicKey
		tui.PrintStatus("Curve25519 keys ready")
	}

	// Step 2: Create instance and service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating systemd service...")
	instance := router.NewInstance(name, transportCfg)
	if err := instance.CreateService(); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	tui.PrintStatus("Service created")

	// Step 3: Set permissions
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Setting permissions...")
	if err := instance.SetPermissions(); err != nil {
		tui.PrintWarning("Permission warning: " + err.Error())
	} else {
		tui.PrintStatus("Permissions set")
	}

	// Step 4: Save config
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Saving configuration...")
	if cfg.Transports == nil {
		cfg.Transports = make(map[string]*types.TransportConfig)
	}
	cfg.Transports[name] = transportCfg

	// Handle mode-specific config
	if cfg.IsSingleMode() {
		// In single mode, set as active if first instance
		if cfg.Single.Active == "" {
			cfg.Single.Active = name
		}
	} else {
		// In multi mode, set as default if first instance
		if cfg.Routing.Default == "" {
			cfg.Routing.Default = name
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	tui.PrintStatus("Configuration saved")

	// Start the instance
	if err := instance.Enable(); err != nil {
		tui.PrintWarning("Failed to enable service: " + err.Error())
	}
	if err := instance.Start(); err != nil {
		tui.PrintWarning("Failed to start instance: " + err.Error())
	} else {
		tui.PrintStatus("Instance started")
	}

	fmt.Println()
	tui.PrintSuccess(fmt.Sprintf("Instance '%s' created and started!", name))
	fmt.Println()

	// Show connection info
	var infoLines []string
	infoLines = append(infoLines, tui.KV("Instance:  ", name))
	infoLines = append(infoLines, tui.KV("Domain:    ", transportCfg.Domain))
	infoLines = append(infoLines, tui.KV("Port:      ", fmt.Sprintf("%d", transportCfg.Port)))

	if fingerprint != "" {
		infoLines = append(infoLines, "")
		infoLines = append(infoLines, "Certificate Fingerprint:")
		infoLines = append(infoLines, certs.FormatFingerprint(fingerprint))
	}
	if publicKey != "" {
		infoLines = append(infoLines, "")
		infoLines = append(infoLines, "Public Key:")
		infoLines = append(infoLines, publicKey)
	}

	tui.PrintBox("Connection Info", infoLines)
	fmt.Println()

	return nil
}

func configureSlipstreamShadowsocks(cfg *types.TransportConfig) error {
	password, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Shadowsocks Password",
		Description: "Leave empty to auto-generate",
	})
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("cancelled")
	}
	if password == "" {
		password = generatePassword()
	}

	method, err := tui.RunMenu(tui.MenuConfig{
		Title: "Encryption Method",
		Options: []tui.MenuOption{
			{Label: "AES-256-GCM (Recommended)", Value: "aes-256-gcm"},
			{Label: "ChaCha20-IETF-Poly1305", Value: "chacha20-ietf-poly1305"},
		},
	})
	if err != nil {
		return err
	}
	if method == "" {
		return fmt.Errorf("cancelled")
	}

	cfg.Shadowsocks = &types.ShadowsocksConfig{
		Password: password,
		Method:   method,
	}

	return nil
}

func configureSlipstreamSocks(cfg *types.TransportConfig, microsocksAddr string) error {
	if microsocksAddr == "" {
		return fmt.Errorf("microsocks not configured. Run install first")
	}
	cfg.Target = &types.TargetConfig{Address: microsocksAddr}
	return nil
}

func configureSlipstreamSSH(cfg *types.TransportConfig) error {
	sshPort := osdetect.DetectSSHPort()
	defaultAddr := "127.0.0.1:" + sshPort

	targetAddr, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Target Address",
		Description: fmt.Sprintf("SSH server address (default: %s)", defaultAddr),
		Placeholder: defaultAddr,
	})
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("cancelled")
	}
	if targetAddr == "" {
		targetAddr = defaultAddr
	}

	cfg.Target = &types.TargetConfig{Address: targetAddr}
	return nil
}
func configureSlipstreamMTProxy(cfg *types.TransportConfig) error {
	proxyUrl, err := installMtProxy(cfg)
	if err != nil {
		return fmt.Errorf("failed to configure MTProxy: %w", err)
	}
	// Target is the local MTProxy endpoint that the tunnel forwards to
	// Slipstream client supports raw TCP, so no bridge needed
	cfg.Target = &types.TargetConfig{Address: fmt.Sprintf("%s:%s", mtproxy.MTProxyBindAddr, mtproxy.MTProxyPort)}

	// Show connection URL to user
	fmt.Println()
	tui.PrintBox("Slipstream + MTProxy (direct)", []string{
		"Slipstream client supports raw TCP tunnel (no bridge needed)",
		"",
		"Client-side Telegram config:",
		"  Type:   MTProto Proxy",
		"  Server: 127.0.0.1 (via slipstream-client)",
		"  Port:   " + mtproxy.MTProxyPort,
		"  Secret: dd<your-secret>",
		"",
		"Or for direct connection (without DNS tunnel):",
		proxyUrl,
	})
	fmt.Println()

	return nil
}

func configureDNSTTSocks(cfg *types.TransportConfig, microsocksAddr string) error {
	if microsocksAddr == "" {
		return fmt.Errorf("microsocks not configured. Run install first")
	}
	cfg.Target = &types.TargetConfig{Address: microsocksAddr}
	cfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
	return nil
}

func configureDNSTTSSH(cfg *types.TransportConfig) error {
	sshPort := osdetect.DetectSSHPort()
	defaultAddr := "127.0.0.1:" + sshPort

	targetAddr, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Target Address",
		Description: fmt.Sprintf("SSH server address (default: %s)", defaultAddr),
		Placeholder: defaultAddr,
	})
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("cancelled")
	}
	if targetAddr == "" {
		targetAddr = defaultAddr
	}

	cfg.Target = &types.TargetConfig{Address: targetAddr}
	cfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
	return nil
}
func configureDNSTTMTProxy(cfg *types.TransportConfig) error {
	proxyUrl, err := installMtProxy(cfg)
	if err != nil {
		return fmt.Errorf("failed to configure MTProxy: %w", err)
	}

	// Install socat bridge for DNSTT (required because dnstt-client provides SOCKS5, not raw TCP)
	tui.PrintStatus("Installing socat bridge for DNSTT...")
	if err := mtproxy.InstallBridge(); err != nil {
		return fmt.Errorf("failed to install MTProxy bridge: %w", err)
	}

	// Target is the bridge port, which forwards to MTProxy
	cfg.Target = &types.TargetConfig{Address: fmt.Sprintf("%s:%s", mtproxy.MTProxyBindAddr, mtproxy.MTProxyBridgePort)}
	cfg.DNSTT = &types.DNSTTConfig{MTU: 1232}

	// Show connection URL to user
	fmt.Println()
	tui.PrintBox("DNSTT + MTProxy (via socat bridge)", []string{
		"Server-side: socat bridge (port " + mtproxy.MTProxyBridgePort + ") → MTProxy (port " + mtproxy.MTProxyPort + ")",
		"",
		"Client-side Telegram config:",
		"  Type:   MTProto Proxy",
		"  Server: 127.0.0.1 (via dnstt-client)",
		"  Port:   " + mtproxy.MTProxyBridgePort,
		"  Secret: dd<your-secret>",
		"",
		"Or for direct connection (without DNS tunnel):",
		proxyUrl,
	})
	fmt.Println()

	return nil
}

func generatePassword() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a less secure but functional approach
		// This should never happen with crypto/rand
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func installMtProxy(cfg *types.TransportConfig) (string, error) {
	secret, err := mtproxy.GenerateSecret()
	if err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}

	tui.PrintStatus(fmt.Sprintf("Using MTProxy secret: %s", secret))

	progressFn := func(downloaded, total int64) {
		if total > 0 {
			percent := float64(downloaded) / float64(total) * 100
			fmt.Printf("\rDownloading: %.1f%%", percent)
		}
	}

	if err := mtproxy.InstallMTProxy(secret, progressFn); err != nil {
		return "", fmt.Errorf("failed to install MTProxy: %w", err)
	}

	if err := mtproxy.ConfigureMTProxy(secret); err != nil {
		return "", fmt.Errorf("failed to configure MTProxy: %w", err)
	}

	// Format proxy URL with domain from config
	domain := "your-domain.com"
	if cfg != nil && cfg.Domain != "" {
		domain = cfg.Domain
	}

	proxyUrl := mtproxy.FormatProxyURL(secret, domain)
	return proxyUrl, nil
}

var instanceRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an instance",
	Long:  "Remove a transport instance and its configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		name := args[0]

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		transportCfg, exists := cfg.Transports[name]
		if !exists {
			return fmt.Errorf("instance %s not found", name)
		}

		// Warn if removing the active instance in single mode
		if cfg.IsSingleMode() && cfg.Single.Active == name {
			fmt.Println()
			tui.PrintWarning("This is the currently active instance.")
			if len(cfg.Transports) > 1 {
				tui.PrintInfo("After removal, run 'dnstm switch <name>' to activate another instance.")
			} else {
				tui.PrintInfo("After removal, no transport will be active. Add a new instance to continue.")
			}
			fmt.Println()
		}

		// Confirm
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			confirm, err := tui.RunConfirm(tui.ConfirmConfig{
				Title:       fmt.Sprintf("Remove instance '%s'?", name),
				Description: "This will stop the service and remove all configuration",
			})
			if err != nil {
				return err
			}
			if !confirm {
				tui.PrintInfo("Cancelled")
				return nil
			}
		}

		fmt.Println()
		tui.PrintInfo("Removing instance...")
		fmt.Println()

		totalSteps := 3
		currentStep := 0

		// Step 1: Stop and remove service
		currentStep++
		tui.PrintStep(currentStep, totalSteps, "Removing service...")
		instance := router.NewInstance(name, transportCfg)
		if err := instance.RemoveService(); err != nil {
			tui.PrintWarning("Service removal warning: " + err.Error())
		} else {
			tui.PrintStatus("Service removed")
		}

		// Step 2: Remove config directory
		currentStep++
		tui.PrintStep(currentStep, totalSteps, "Removing configuration...")
		if err := instance.RemoveConfigDir(); err != nil {
			tui.PrintWarning("Config removal warning: " + err.Error())
		} else {
			tui.PrintStatus("Configuration removed")
		}

		// Step 3: Update config
		currentStep++
		tui.PrintStep(currentStep, totalSteps, "Updating router configuration...")
		delete(cfg.Transports, name)

		// Update Routing.Default if needed (multi mode)
		if cfg.Routing.Default == name {
			cfg.Routing.Default = ""
			for n := range cfg.Transports {
				cfg.Routing.Default = n
				break
			}
		}

		// Update Single.Active if needed (single mode)
		if cfg.Single.Active == name {
			cfg.Single.Active = ""
			for n := range cfg.Transports {
				cfg.Single.Active = n
				break
			}
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		tui.PrintStatus("Configuration updated")

		fmt.Println()
		tui.PrintSuccess(fmt.Sprintf("Instance '%s' removed!", name))
		fmt.Println()

		return nil
	},
}

var instanceStartCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Start an instance",
	Long:  "Start or restart a transport instance. If already running, restarts to pick up changes.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		name := args[0]

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		transportCfg, exists := cfg.Transports[name]
		if !exists {
			return fmt.Errorf("instance %s not found", name)
		}

		instance := router.NewInstance(name, transportCfg)

		if err := instance.Enable(); err != nil {
			tui.PrintWarning("Failed to enable service: " + err.Error())
		}

		isRunning := instance.IsActive()
		if isRunning {
			if err := instance.Restart(); err != nil {
				return fmt.Errorf("failed to restart instance: %w", err)
			}
			tui.PrintSuccess(fmt.Sprintf("Instance '%s' restarted", name))
		} else {
			if err := instance.Start(); err != nil {
				return fmt.Errorf("failed to start instance: %w", err)
			}
			tui.PrintSuccess(fmt.Sprintf("Instance '%s' started", name))
		}
		return nil
	},
}

var instanceStopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop an instance",
	Long:  "Stop a transport instance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		name := args[0]

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		transportCfg, exists := cfg.Transports[name]
		if !exists {
			return fmt.Errorf("instance %s not found", name)
		}

		instance := router.NewInstance(name, transportCfg)

		if err := instance.Stop(); err != nil {
			return fmt.Errorf("failed to stop instance: %w", err)
		}

		tui.PrintSuccess(fmt.Sprintf("Instance '%s' stopped", name))
		return nil
	},
}

var instanceLogsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "Show instance logs",
	Long:  "Show recent logs from a transport instance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		name := args[0]
		lines, _ := cmd.Flags().GetInt("lines")

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		transportCfg, exists := cfg.Transports[name]
		if !exists {
			return fmt.Errorf("instance %s not found", name)
		}

		instance := router.NewInstance(name, transportCfg)

		logs, err := instance.GetLogs(lines)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}

		fmt.Println(logs)
		return nil
	},
}

var instanceStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show instance status",
	Long:  "Show status and configuration for a transport instance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		name := args[0]

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		transportCfg, exists := cfg.Transports[name]
		if !exists {
			return fmt.Errorf("instance %s not found", name)
		}

		instance := router.NewInstance(name, transportCfg)
		fmt.Println(instance.GetFormattedInfo())

		// Show certificate/key info
		if types.IsSlipstreamType(transportCfg.Type) {
			certMgr := certs.NewManager()
			certInfo := certMgr.Get(transportCfg.Domain)
			if certInfo != nil {
				fmt.Println("Certificate Fingerprint:")
				fmt.Println(certs.FormatFingerprint(certInfo.Fingerprint))
				fmt.Println()
			}
		} else if types.IsDNSTTType(transportCfg.Type) {
			keyMgr := keys.NewManager()
			keyInfo := keyMgr.Get(transportCfg.Domain)
			if keyInfo != nil {
				fmt.Println("Public Key:")
				fmt.Println(keyInfo.PublicKey)
				fmt.Println()
			}
		}

		return nil
	},
}

var instanceReconfigureCmd = &cobra.Command{
	Use:   "reconfigure <name>",
	Short: "Reconfigure an instance",
	Long:  "Reconfigure an existing transport instance interactively (including rename)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		name := args[0]

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		transportCfg, exists := cfg.Transports[name]
		if !exists {
			return fmt.Errorf("instance %s not found", name)
		}

		fmt.Println()
		tui.PrintInfo(fmt.Sprintf("Reconfiguring '%s'...", name))
		tui.PrintInfo("Press Enter to keep current value, or type a new value.")
		fmt.Println()

		changed := false
		renamed := false
		newName := name

		// Check if instance is running before we start
		oldInstance := router.NewInstance(name, transportCfg)
		wasRunning := oldInstance.IsActive()

		// Name (rename)
		inputName, confirmed, err := tui.RunInput(tui.InputConfig{
			Title:       "Instance Name",
			Description: fmt.Sprintf("Current: %s", name),
			Value:       name,
		})
		if err != nil {
			return err
		}
		if !confirmed {
			tui.PrintInfo("Cancelled")
			return nil
		}
		if inputName != "" && inputName != name {
			inputName = router.NormalizeName(inputName)
			if err := router.ValidateName(inputName); err != nil {
				return fmt.Errorf("invalid name: %w", err)
			}
			if _, exists := cfg.Transports[inputName]; exists {
				return fmt.Errorf("instance with name '%s' already exists", inputName)
			}
			newName = inputName
			renamed = true
			changed = true
		}

		// Domain
		newDomain, confirmed, err := tui.RunInput(tui.InputConfig{
			Title:       "Domain",
			Description: fmt.Sprintf("Current: %s", transportCfg.Domain),
			Value:       transportCfg.Domain,
		})
		if err != nil {
			return err
		}
		if !confirmed {
			tui.PrintInfo("Cancelled")
			return nil
		}
		if newDomain != "" && newDomain != transportCfg.Domain {
			transportCfg.Domain = newDomain
			changed = true
		}

		// Type-specific configuration
		switch transportCfg.Type {
		case types.TypeSlipstreamShadowsocks:
			// Password
			newPassword, confirmed, err := tui.RunInput(tui.InputConfig{
				Title:       "Password",
				Description: "Current: (hidden) - Leave empty to keep current",
			})
			if err != nil {
				return err
			}
			if !confirmed {
				tui.PrintInfo("Cancelled")
				return nil
			}
			if newPassword != "" {
				if transportCfg.Shadowsocks == nil {
					transportCfg.Shadowsocks = &types.ShadowsocksConfig{}
				}
				transportCfg.Shadowsocks.Password = newPassword
				changed = true
			}

			// Method
			currentMethod := "aes-256-gcm"
			if transportCfg.Shadowsocks != nil && transportCfg.Shadowsocks.Method != "" {
				currentMethod = transportCfg.Shadowsocks.Method
			}
			methodOptions := []tui.MenuOption{
				{Label: "AES-256-GCM", Value: "aes-256-gcm"},
				{Label: "ChaCha20-IETF-Poly1305", Value: "chacha20-ietf-poly1305"},
			}
			selected := 0
			if currentMethod == "chacha20-ietf-poly1305" {
				selected = 1
			}
			newMethod, err := tui.RunMenu(tui.MenuConfig{
				Title:       "Method",
				Description: fmt.Sprintf("Current: %s", currentMethod),
				Options:     methodOptions,
				Selected:    selected,
			})
			if err != nil {
				return err
			}
			if newMethod == "" {
				tui.PrintInfo("Cancelled")
				return nil
			}
			if newMethod != currentMethod {
				if transportCfg.Shadowsocks == nil {
					transportCfg.Shadowsocks = &types.ShadowsocksConfig{}
				}
				transportCfg.Shadowsocks.Method = newMethod
				changed = true
			}

		case types.TypeSlipstreamSocks, types.TypeDNSTTSocks:
			// SOCKS modes auto-use microsocks
			microsocksAddr := cfg.GetMicrosocksAddress()
			if microsocksAddr != "" && (transportCfg.Target == nil || transportCfg.Target.Address != microsocksAddr) {
				if transportCfg.Target == nil {
					transportCfg.Target = &types.TargetConfig{}
				}
				transportCfg.Target.Address = microsocksAddr
				changed = true
			}

		case types.TypeSlipstreamSSH, types.TypeDNSTTSSH:
			// SSH modes - allow changing target
			currentTarget := "127.0.0.1:" + osdetect.DetectSSHPort()
			if transportCfg.Target != nil && transportCfg.Target.Address != "" {
				currentTarget = transportCfg.Target.Address
			}
			newTarget, confirmed, err := tui.RunInput(tui.InputConfig{
				Title:       "Target Address",
				Description: fmt.Sprintf("Current: %s", currentTarget),
				Value:       currentTarget,
			})
			if err != nil {
				return err
			}
			if !confirmed {
				tui.PrintInfo("Cancelled")
				return nil
			}
			if newTarget != "" && newTarget != currentTarget {
				if transportCfg.Target == nil {
					transportCfg.Target = &types.TargetConfig{}
				}
				transportCfg.Target.Address = newTarget
				changed = true
			}
		}

		if !changed {
			tui.PrintInfo("No changes made")
			return nil
		}

		// Handle rename
		if renamed {
			// Stop and remove old service
			oldInstance.Stop()
			oldInstance.RemoveService()
			oldInstance.RemoveConfigDir()

			// Update config map
			delete(cfg.Transports, name)
			cfg.Transports[newName] = transportCfg

			// Update Single.Active if it referenced the old name
			if cfg.Single.Active == name {
				cfg.Single.Active = newName
			}
			// Update Routing.Default if it referenced the old name
			if cfg.Routing.Default == name {
				cfg.Routing.Default = newName
			}
		}

		// Save config
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		// Create new service
		newInstance := router.NewInstance(newName, transportCfg)
		if err := newInstance.CreateService(); err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		newInstance.SetPermissions()

		// Start if it was running before
		if wasRunning {
			newInstance.Enable()
			if err := newInstance.Start(); err != nil {
				return fmt.Errorf("failed to start: %w", err)
			}
			if renamed {
				tui.PrintSuccess(fmt.Sprintf("Instance renamed to '%s' and restarted!", newName))
			} else {
				tui.PrintSuccess(fmt.Sprintf("Instance '%s' reconfigured and restarted!", newName))
			}
		} else {
			if renamed {
				tui.PrintSuccess(fmt.Sprintf("Instance renamed to '%s'!", newName))
			} else {
				tui.PrintSuccess(fmt.Sprintf("Instance '%s' reconfigured!", newName))
			}
		}

		return nil
	},
}

func init() {
	instanceCmd.AddCommand(instanceListCmd)
	instanceCmd.AddCommand(instanceAddCmd)
	instanceCmd.AddCommand(instanceRemoveCmd)
	instanceCmd.AddCommand(instanceStartCmd)
	instanceCmd.AddCommand(instanceStopCmd)
	instanceCmd.AddCommand(instanceLogsCmd)
	instanceCmd.AddCommand(instanceStatusCmd)
	instanceCmd.AddCommand(instanceReconfigureCmd)

	// Flags for instance add
	instanceAddCmd.Flags().StringP("type", "t", "", "Transport type (slipstream-shadowsocks, slipstream-socks, slipstream-ssh, dnstt-socks, dnstt-ssh)")
	instanceAddCmd.Flags().StringP("domain", "d", "", "Domain name")
	instanceAddCmd.Flags().String("target", "", "Target address (for socks/ssh modes)")
	instanceAddCmd.Flags().String("password", "", "Shadowsocks password (for slipstream-shadowsocks)")
	instanceAddCmd.Flags().String("method", "", "Shadowsocks encryption method (for slipstream-shadowsocks)")

	// Flags for instance remove
	instanceRemoveCmd.Flags().BoolP("force", "f", false, "Skip confirmation")

	// Flags for instance logs
	instanceLogsCmd.Flags().IntP("lines", "n", 50, "Number of log lines to show")
}
