package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/dnstm/internal/types"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

func init() {
	actions.SetInstanceHandler(actions.ActionInstanceAdd, HandleInstanceAdd)
}

// HandleInstanceAdd adds a new instance.
func HandleInstanceAdd(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if we have flags for non-interactive mode
	transportType := ctx.GetString("type")
	domain := ctx.GetString("domain")

	if transportType != "" && domain != "" {
		// Non-interactive mode
		return addInstanceNonInteractive(ctx, cfg)
	}

	// Interactive mode
	return addInstanceInteractive(ctx, cfg)
}

func addInstanceInteractive(ctx *actions.Context, cfg *router.Config) error {
	ctx.Output.Println()
	ctx.Output.Info("Adding new transport instance...")
	ctx.Output.Println()

	// Select transport type
	transportType, err := tui.RunMenu(tui.MenuConfig{
		Title: "Transport Type",
		Options: []tui.MenuOption{
			{Label: "Slipstream + Shadowsocks (Recommended)", Value: string(types.TypeSlipstreamShadowsocks)},
			{Label: "Slipstream SOCKS", Value: string(types.TypeSlipstreamSocks)},
			{Label: "Slipstream SSH", Value: string(types.TypeSlipstreamSSH)},
			{Label: "DNSTT SOCKS", Value: string(types.TypeDNSTTSocks)},
			{Label: "DNSTT SSH", Value: string(types.TypeDNSTTSSH)},
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
	if ctx.HasArg(0) {
		name = ctx.GetArg(0)
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
		return actions.ExistsError(name)
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
			ctx.Output.Error("Domain is required")
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
	case types.TypeDNSTTSocks:
		if err := configureDNSTTSocks(transportCfg, microsocksAddr); err != nil {
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

	// Create the instance
	return createInstance(ctx, name, transportCfg, cfg)
}

func addInstanceNonInteractive(ctx *actions.Context, cfg *router.Config) error {
	transportType := ctx.GetString("type")
	domain := ctx.GetString("domain")
	targetAddr := ctx.GetString("target")
	password := ctx.GetString("password")
	method := ctx.GetString("method")

	// Get name
	var name string
	if ctx.HasArg(0) {
		name = ctx.GetArg(0)
	} else {
		name = router.GenerateUniqueName(cfg.Transports)
	}

	name = router.NormalizeName(name)
	if err := router.ValidateName(name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}

	if _, exists := cfg.Transports[name]; exists {
		return actions.ExistsError(name)
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
			password = GeneratePassword()
		}
		if method == "" {
			method = "aes-256-gcm"
		}
		transportCfg.Shadowsocks = &types.ShadowsocksConfig{
			Password: password,
			Method:   method,
		}
	case types.TypeSlipstreamSocks:
		if targetAddr == "" {
			if microsocksAddr == "" {
				return actions.NewActionError(
					"microsocks not configured",
					"Run 'dnstm install' first",
				)
			}
			targetAddr = microsocksAddr
		}
		transportCfg.Target = &types.TargetConfig{Address: targetAddr}
	case types.TypeSlipstreamSSH:
		if targetAddr == "" {
			targetAddr = GetDefaultSSHAddress()
		}
		transportCfg.Target = &types.TargetConfig{Address: targetAddr}
	case types.TypeDNSTTSocks:
		if targetAddr == "" {
			if microsocksAddr == "" {
				return actions.NewActionError(
					"microsocks not configured",
					"Run 'dnstm install' first",
				)
			}
			targetAddr = microsocksAddr
		}
		transportCfg.Target = &types.TargetConfig{Address: targetAddr}
		transportCfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
	case types.TypeDNSTTSSH:
		if targetAddr == "" {
			targetAddr = GetDefaultSSHAddress()
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

	return createInstance(ctx, name, transportCfg, cfg)
}

func createInstance(ctx *actions.Context, name string, transportCfg *types.TransportConfig, cfg *router.Config) error {
	ctx.Output.Println()

	totalSteps := 4
	currentStep := 0

	// Step 1: Generate certificates/keys
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Generating cryptographic material...")
	var fingerprint string
	var publicKey string
	if types.IsSlipstreamType(transportCfg.Type) {
		certMgr := certs.NewManager()
		certInfo, err := certMgr.GetOrCreate(transportCfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to generate certificate: %w", err)
		}
		fingerprint = certInfo.Fingerprint
		ctx.Output.Status("TLS certificate ready")
	} else if types.IsDNSTTType(transportCfg.Type) {
		keyMgr := keys.NewManager()
		keyInfo, err := keyMgr.GetOrCreate(transportCfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
		}
		publicKey = keyInfo.PublicKey
		ctx.Output.Status("Curve25519 keys ready")
	}

	// Step 2: Create instance and service
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Creating systemd service...")
	instance := router.NewInstance(name, transportCfg)

	// Determine service mode based on current router mode
	// In single mode: if this will be the active instance, bind to external IP:53
	// Otherwise, use multi-mode binding (localhost:port)
	serviceMode := router.ServiceModeMulti
	if cfg.IsSingleMode() {
		// Will this be the active instance?
		willBeActive := cfg.Single.Active == "" || cfg.Single.Active == name
		if willBeActive {
			serviceMode = router.ServiceModeSingle
		}
	}

	if err := instance.CreateServiceWithMode(serviceMode); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	ctx.Output.Status("Service created")

	// Step 3: Set permissions
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Setting permissions...")
	if err := instance.SetPermissions(); err != nil {
		ctx.Output.Warning("Permission warning: " + err.Error())
	} else {
		ctx.Output.Status("Permissions set")
	}

	// Step 4: Save config
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Saving configuration...")
	if cfg.Transports == nil {
		cfg.Transports = make(map[string]*types.TransportConfig)
	}
	cfg.Transports[name] = transportCfg

	// Handle mode-specific config
	if cfg.IsSingleMode() {
		if cfg.Single.Active == "" {
			cfg.Single.Active = name
		}
	} else {
		if cfg.Routing.Default == "" {
			cfg.Routing.Default = name
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	ctx.Output.Status("Configuration saved")

	// Start the instance
	if err := instance.Enable(); err != nil {
		ctx.Output.Warning("Failed to enable service: " + err.Error())
	}
	if err := instance.Start(); err != nil {
		ctx.Output.Warning("Failed to start instance: " + err.Error())
	} else {
		ctx.Output.Status("Instance started")
	}

	ctx.Output.Println()
	ctx.Output.Success(fmt.Sprintf("Instance '%s' created and started!", name))
	ctx.Output.Println()

	// Show connection info
	var infoLines []string
	infoLines = append(infoLines, ctx.Output.KV("Instance:  ", name))
	infoLines = append(infoLines, ctx.Output.KV("Domain:    ", transportCfg.Domain))
	infoLines = append(infoLines, ctx.Output.KV("Port:      ", fmt.Sprintf("%d", transportCfg.Port)))

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

	ctx.Output.Box("Connection Info", infoLines)
	ctx.Output.Println()

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
		return actions.ErrCancelled
	}
	if password == "" {
		password = GeneratePassword()
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
		return actions.ErrCancelled
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
		return actions.ErrCancelled
	}
	if targetAddr == "" {
		targetAddr = defaultAddr
	}

	cfg.Target = &types.TargetConfig{Address: targetAddr}
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
		return actions.ErrCancelled
	}
	if targetAddr == "" {
		targetAddr = defaultAddr
	}

	cfg.Target = &types.TargetConfig{Address: targetAddr}
	cfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
	return nil
}
