package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/system"
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

	// Get or generate name
	var name string
	if len(args) > 0 {
		name = args[0]
	}

	suggestedName := router.GenerateUniqueName(cfg.Transports)
	if name == "" {
		err := huh.NewInput().
			Title("Instance Name").
			Description(fmt.Sprintf("Leave empty for auto-generated name (%s)", suggestedName)).
			Placeholder(suggestedName).
			Value(&name).
			Run()
		if err != nil {
			return err
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

	// Select transport type
	var transportType string
	err := huh.NewSelect[string]().
		Title("Transport Type").
		Options(
			huh.NewOption("Slipstream + Shadowsocks (Recommended)", string(types.TypeSlipstreamShadowsocks)),
			huh.NewOption("Slipstream SOCKS", string(types.TypeSlipstreamSocks)),
			huh.NewOption("Slipstream SSH", string(types.TypeSlipstreamSSH)),
			huh.NewOption("DNSTT SOCKS", string(types.TypeDNSTTSocks)),
			huh.NewOption("DNSTT SSH", string(types.TypeDNSTTSSH)),
		).
		Value(&transportType).
		Run()
	if err != nil {
		return err
	}

	// Install required binaries if needed
	if err := transport.EnsureBinariesInstalled(types.TransportType(transportType)); err != nil {
		return fmt.Errorf("failed to install required binaries: %w", err)
	}

	// Get domain
	var domain string
	for {
		err = huh.NewInput().
			Title("Domain").
			Description("e.g., t1.example.com").
			Value(&domain).
			Run()
		if err != nil {
			return err
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
	switch types.TransportType(transportType) {
	case types.TypeSlipstreamShadowsocks:
		if err := configureSlipstreamShadowsocks(transportCfg); err != nil {
			return err
		}
	case types.TypeSlipstreamSocks:
		if err := configureSlipstreamSocks(transportCfg); err != nil {
			return err
		}
	case types.TypeSlipstreamSSH:
		if err := configureSlipstreamSSH(transportCfg); err != nil {
			return err
		}
	case types.TypeDNSTTSocks:
		if err := configureDNSTTSocks(transportCfg); err != nil {
			return err
		}
	case types.TypeDNSTTSSH:
		if err := configureDNSTTSSH(transportCfg); err != nil {
			return err
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

	// Show summary
	summaryLines := []string{
		tui.KV("Name:     ", name),
		tui.KV("Type:     ", types.GetTransportTypeDisplayName(transportCfg.Type)),
		tui.KV("Domain:   ", transportCfg.Domain),
		tui.KV("Port:     ", fmt.Sprintf("%d", transportCfg.Port)),
	}
	if transportCfg.Shadowsocks != nil {
		summaryLines = append(summaryLines, tui.KV("Method:   ", transportCfg.Shadowsocks.Method))
	}
	if transportCfg.Target != nil {
		summaryLines = append(summaryLines, tui.KV("Target:   ", transportCfg.Target.Address))
	}
	tui.PrintBox("Instance Summary", summaryLines)

	// Confirm
	confirm := true
	err = huh.NewConfirm().
		Title("Create instance?").
		Value(&confirm).
		Run()
	if err != nil {
		return err
	}
	if !confirm {
		tui.PrintInfo("Cancelled")
		return nil
	}

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
	case types.TypeSlipstreamSocks, types.TypeSlipstreamSSH:
		if targetAddr == "" {
			if types.TransportType(transportType) == types.TypeSlipstreamSSH {
				targetAddr = "127.0.0.1:" + osdetect.DetectSSHPort()
			} else {
				targetAddr = "127.0.0.1:1080"
			}
		}
		transportCfg.Target = &types.TargetConfig{Address: targetAddr}
	case types.TypeDNSTTSocks, types.TypeDNSTTSSH:
		if targetAddr == "" {
			if types.TransportType(transportType) == types.TypeDNSTTSSH {
				targetAddr = "127.0.0.1:" + osdetect.DetectSSHPort()
			} else {
				targetAddr = "127.0.0.1:1080"
			}
		}
		transportCfg.Target = &types.TargetConfig{Address: targetAddr}
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

	totalSteps := 5
	currentStep := 0

	// Step 1: Create dnstm user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating dnstm user...")
	if err := system.CreateDnstmUser(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	tui.PrintStatus("User 'dnstm' created")

	// Step 2: Generate certificates/keys
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

	// Step 3: Create instance and service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating systemd service...")
	instance := router.NewInstance(name, transportCfg)
	if err := instance.CreateService(); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	tui.PrintStatus("Service created")

	// Step 4: Set permissions
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Setting permissions...")
	if err := instance.SetPermissions(); err != nil {
		tui.PrintWarning("Permission warning: " + err.Error())
	} else {
		tui.PrintStatus("Permissions set")
	}

	// Step 5: Save config
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

	fmt.Println()
	tui.PrintSuccess(fmt.Sprintf("Instance '%s' created successfully!", name))
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

	tui.PrintInfo("To start the instance: dnstm instance start " + name)
	fmt.Println()

	return nil
}

func configureSlipstreamShadowsocks(cfg *types.TransportConfig) error {
	var password string
	err := huh.NewInput().
		Title("Shadowsocks Password").
		Description("Leave empty to auto-generate").
		Value(&password).
		Run()
	if err != nil {
		return err
	}
	if password == "" {
		password = generatePassword()
	}

	var method string
	err = huh.NewSelect[string]().
		Title("Encryption Method").
		Options(
			huh.NewOption("AES-256-GCM (Recommended)", "aes-256-gcm"),
			huh.NewOption("ChaCha20-IETF-Poly1305", "chacha20-ietf-poly1305"),
		).
		Value(&method).
		Run()
	if err != nil {
		return err
	}

	cfg.Shadowsocks = &types.ShadowsocksConfig{
		Password: password,
		Method:   method,
	}

	return nil
}

func configureSlipstreamSocks(cfg *types.TransportConfig) error {
	var targetAddr string
	defaultAddr := "127.0.0.1:1080"
	err := huh.NewInput().
		Title("Target Address").
		Description(fmt.Sprintf("SOCKS proxy address (default: %s)", defaultAddr)).
		Placeholder(defaultAddr).
		Value(&targetAddr).
		Run()
	if err != nil {
		return err
	}
	if targetAddr == "" {
		targetAddr = defaultAddr
	}

	cfg.Target = &types.TargetConfig{Address: targetAddr}
	return nil
}

func configureSlipstreamSSH(cfg *types.TransportConfig) error {
	sshPort := osdetect.DetectSSHPort()
	defaultAddr := "127.0.0.1:" + sshPort

	var targetAddr string
	err := huh.NewInput().
		Title("Target Address").
		Description(fmt.Sprintf("SSH server address (default: %s)", defaultAddr)).
		Placeholder(defaultAddr).
		Value(&targetAddr).
		Run()
	if err != nil {
		return err
	}
	if targetAddr == "" {
		targetAddr = defaultAddr
	}

	cfg.Target = &types.TargetConfig{Address: targetAddr}
	return nil
}

func configureDNSTTSocks(cfg *types.TransportConfig) error {
	var targetAddr string
	defaultAddr := "127.0.0.1:1080"
	err := huh.NewInput().
		Title("Target Address").
		Description(fmt.Sprintf("SOCKS proxy address (default: %s)", defaultAddr)).
		Placeholder(defaultAddr).
		Value(&targetAddr).
		Run()
	if err != nil {
		return err
	}
	if targetAddr == "" {
		targetAddr = defaultAddr
	}

	cfg.Target = &types.TargetConfig{Address: targetAddr}
	cfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
	return nil
}

func configureDNSTTSSH(cfg *types.TransportConfig) error {
	sshPort := osdetect.DetectSSHPort()
	defaultAddr := "127.0.0.1:" + sshPort

	var targetAddr string
	err := huh.NewInput().
		Title("Target Address").
		Description(fmt.Sprintf("SSH server address (default: %s)", defaultAddr)).
		Placeholder(defaultAddr).
		Value(&targetAddr).
		Run()
	if err != nil {
		return err
	}
	if targetAddr == "" {
		targetAddr = defaultAddr
	}

	cfg.Target = &types.TargetConfig{Address: targetAddr}
	cfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
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
			confirm := false
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Remove instance '%s'?", name)).
				Description("This will stop the service and remove all configuration").
				Value(&confirm).
				Run()
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
	Long:  "Start a transport instance",
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

		if err := instance.Start(); err != nil {
			return fmt.Errorf("failed to start instance: %w", err)
		}

		tui.PrintSuccess(fmt.Sprintf("Instance '%s' started", name))
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

var instanceRestartCmd = &cobra.Command{
	Use:   "restart <name>",
	Short: "Restart an instance",
	Long:  "Restart a transport instance",
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

		if err := instance.Restart(); err != nil {
			return fmt.Errorf("failed to restart instance: %w", err)
		}

		tui.PrintSuccess(fmt.Sprintf("Instance '%s' restarted", name))
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

var instanceConfigCmd = &cobra.Command{
	Use:   "config <name>",
	Short: "Show instance configuration",
	Long:  "Show the configuration for a transport instance",
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

var instanceStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show instance status",
	Long:  "Show detailed status for a transport instance",
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

		status, err := instance.GetStatus()
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}

		fmt.Println(status)
		return nil
	},
}

func init() {
	instanceCmd.AddCommand(instanceListCmd)
	instanceCmd.AddCommand(instanceAddCmd)
	instanceCmd.AddCommand(instanceRemoveCmd)
	instanceCmd.AddCommand(instanceStartCmd)
	instanceCmd.AddCommand(instanceStopCmd)
	instanceCmd.AddCommand(instanceRestartCmd)
	instanceCmd.AddCommand(instanceLogsCmd)
	instanceCmd.AddCommand(instanceConfigCmd)
	instanceCmd.AddCommand(instanceStatusCmd)

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
