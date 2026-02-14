package handlers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/go-corelib/tui"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelAdd, HandleTunnelAdd)
}

// HandleTunnelAdd adds a new tunnel.
func HandleTunnelAdd(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if ctx.IsInteractive {
		return addTunnelInteractive(ctx, cfg)
	}
	return addTunnelNonInteractive(ctx, cfg)
}

func addTunnelInteractive(ctx *actions.Context, cfg *config.Config) error {
	// Select transport type
	transportType, err := tui.RunMenu(tui.MenuConfig{
		Title: "Transport Type",
		Options: []tui.MenuOption{
			{Label: "DNSTT", Value: string(config.TransportDNSTT)},
			{Label: "Slipstream", Value: string(config.TransportSlipstream)},
		},
	})
	if err != nil {
		return err
	}
	if transportType == "" {
		return nil
	}

	// Select backend
	backendOptions := buildBackendOptions(cfg, config.TransportType(transportType))
	if len(backendOptions) == 0 {
		return actions.NewActionError(
			"no compatible backends available",
			"Add a backend first with 'dnstm backend add'",
		)
	}

	backendTag, err := tui.RunMenu(tui.MenuConfig{
		Title:   "Backend",
		Options: backendOptions,
	})
	if err != nil {
		return err
	}
	if backendTag == "" {
		return nil
	}

	// Validate backend exists
	backend := cfg.GetBackendByTag(backendTag)
	if backend == nil {
		return actions.BackendNotFoundError(backendTag)
	}

	// Get or generate tag
	tag := ctx.GetString("tag")

	suggestedTag := router.GenerateUniqueTunnelTag(cfg.Tunnels)
	if tag == "" {
		var confirmed bool
		tag, confirmed, err = tui.RunInput(tui.InputConfig{
			Title: "Tunnel Tag",
			Value: suggestedTag,
		})
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
		if tag == "" {
			tag = suggestedTag
		}
	}

	tag = router.NormalizeTag(tag)
	if err := router.ValidateTag(tag); err != nil {
		return fmt.Errorf("invalid tag: %w", err)
	}

	if cfg.GetTunnelByTag(tag) != nil {
		return actions.TunnelExistsError(tag)
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

	// Build tunnel config
	tunnelCfg := &config.TunnelConfig{
		Tag:       tag,
		Transport: config.TransportType(transportType),
		Backend:   backendTag,
		Domain:    domain,
	}

	// Transport-specific configuration
	if tunnelCfg.Transport == config.TransportDNSTT {
		tunnelCfg.DNSTT = &config.DNSTTConfig{MTU: 1232}
	}

	// Allocate port
	port := cfg.AllocateNextPort()
	tunnelCfg.Port = port

	// Create the tunnel
	return createTunnel(ctx, tunnelCfg, cfg)
}

func addTunnelNonInteractive(ctx *actions.Context, cfg *config.Config) error {
	transportStr := ctx.GetString("transport")
	backendTag := ctx.GetString("backend")
	domain := ctx.GetString("domain")
	port := ctx.GetInt("port")
	mtu := ctx.GetInt("mtu")

	if transportStr == "" || backendTag == "" || domain == "" {
		return fmt.Errorf("--transport, --backend, and --domain flags are required\n\nUsage: dnstm tunnel add --transport TYPE -b BACKEND -d DOMAIN [-t TAG]")
	}

	transportType := config.TransportType(transportStr)

	// Validate transport type
	if transportType != config.TransportSlipstream && transportType != config.TransportDNSTT {
		return fmt.Errorf("invalid transport type: %s (must be slipstream or dnstt)", transportType)
	}

	// Validate backend exists and is compatible
	backend := cfg.GetBackendByTag(backendTag)
	if backend == nil {
		return actions.BackendNotFoundError(backendTag)
	}

	// Check transport-backend compatibility
	if transportType == config.TransportDNSTT && backend.Type == config.BackendShadowsocks {
		return actions.NewActionError(
			"incompatible transport and backend",
			"DNSTT transport does not support Shadowsocks backend",
		)
	}

	// Get tag from --tag/-t flag, or auto-generate
	tag := ctx.GetString("tag")
	if tag == "" {
		tag = router.GenerateUniqueTunnelTag(cfg.Tunnels)
	}

	tag = router.NormalizeTag(tag)
	if err := router.ValidateTag(tag); err != nil {
		return fmt.Errorf("invalid tag: %w", err)
	}

	if cfg.GetTunnelByTag(tag) != nil {
		return actions.TunnelExistsError(tag)
	}

	// Build config
	tunnelCfg := &config.TunnelConfig{
		Tag:       tag,
		Transport: transportType,
		Backend:   backendTag,
		Domain:    domain,
	}

	// Transport-specific configuration
	if transportType == config.TransportDNSTT {
		if mtu == 0 {
			mtu = 1232
		}
		tunnelCfg.DNSTT = &config.DNSTTConfig{MTU: mtu}
	}

	// Allocate port
	if port == 0 {
		port = cfg.AllocateNextPort()
	}
	tunnelCfg.Port = port

	return createTunnel(ctx, tunnelCfg, cfg)
}

// promptModeSwitch prompts the user to switch from single to multi mode when adding a second tunnel.
// Returns true if mode was switched, false if user declined.
func promptModeSwitch(ctx *actions.Context, cfg *config.Config) (bool, error) {
	existingTunnel := cfg.Tunnels[0].Tag

	confirm, err := tui.RunConfirm(tui.ConfirmConfig{
		Title: "Switch to multi mode?",
		Description: fmt.Sprintf(
			"You already have tunnel '%s'. Single mode only allows one active tunnel.\nMulti mode allows running multiple tunnels simultaneously with DNS-based routing.",
			existingTunnel,
		),
	})
	if err != nil {
		return false, err
	}

	if !confirm {
		_ = tui.ShowMessage(tui.AppMessage{Type: "info", Message: "Staying in single mode. New tunnel will be added but only one can be active."})
		return false, nil
	}

	r, err := router.New(cfg)
	if err != nil {
		return false, fmt.Errorf("failed to create router: %w", err)
	}

	if err := r.SwitchMode("multi"); err != nil {
		return false, fmt.Errorf("failed to switch mode: %w", err)
	}

	_ = tui.ShowMessage(tui.AppMessage{Type: "info", Message: "Switched to multi mode!"})

	return true, nil
}

func createTunnel(ctx *actions.Context, tunnelCfg *config.TunnelConfig, cfg *config.Config) error {
	// Check if we need to switch to multi mode
	// This happens when adding a second tunnel while in single mode
	if cfg.IsSingleMode() && len(cfg.Tunnels) > 0 {
		if ctx.IsInteractive {
			switchedMode, err := promptModeSwitch(ctx, cfg)
			if err != nil {
				return err
			}
			if switchedMode {
				// Reload config after mode switch (services were regenerated)
				newCfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("failed to reload config after mode switch: %w", err)
				}
				*cfg = *newCfg
			}
		} else {
			// Non-interactive mode: just inform the user
			existingTunnel := cfg.Tunnels[0].Tag
			ctx.Output.Info("Adding tunnel to single mode. Existing active tunnel: " + existingTunnel)
			ctx.Output.Info("New tunnel will be added but not activated. Use 'dnstm router switch' to activate it.")
			ctx.Output.Println()
		}
	}

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		ctx.Output.BeginProgress(fmt.Sprintf("Add Tunnel: %s", tunnelCfg.Tag))
	} else {
		ctx.Output.Println()
	}

	totalSteps := 6
	currentStep := 0

	// Step 1: Install required binaries
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Installing transport binaries...")
	if err := transport.EnsureTransportBinariesInstalled(tunnelCfg.Transport); err != nil {
		return fmt.Errorf("failed to install required binaries: %w", err)
	}
	ctx.Output.Status("Transport binaries ready")

	// Step 2: Generate certificates/keys
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Generating cryptographic material...")
	var fingerprint string
	var publicKey string
	if tunnelCfg.Transport == config.TransportSlipstream {
		certMgr := certs.NewManager()
		certInfo, err := certMgr.GetOrCreate(tunnelCfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to generate certificate: %w", err)
		}
		fingerprint = certInfo.Fingerprint
		// Store cert paths in slipstream config
		tunnelCfg.Slipstream = &config.SlipstreamConfig{
			Cert: certInfo.CertPath,
			Key:  certInfo.KeyPath,
		}
		ctx.Output.Status("TLS certificate ready")
	} else if tunnelCfg.Transport == config.TransportDNSTT {
		keyMgr := keys.NewManager()
		keyInfo, err := keyMgr.GetOrCreate(tunnelCfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
		}
		publicKey = keyInfo.PublicKey
		// Store private key path
		tunnelCfg.DNSTT.PrivateKey = keyInfo.PrivateKeyPath
		ctx.Output.Status("Curve25519 keys ready")
	}

	// Step 3: Create tunnel config directory
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Creating tunnel configuration...")
	tunnelDir := filepath.Join(config.TunnelsDir, tunnelCfg.Tag)
	if err := os.MkdirAll(tunnelDir, 0750); err != nil {
		return fmt.Errorf("failed to create tunnel directory: %w", err)
	}
	ctx.Output.Status("Tunnel directory created")

	// Step 4: Create systemd service
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Creating systemd service...")
	tunnel := router.NewTunnel(tunnelCfg)

	// Determine service mode based on current router mode
	serviceMode := router.ServiceModeMulti
	if cfg.IsSingleMode() {
		// Will this be the active tunnel?
		willBeActive := cfg.Route.Active == "" || cfg.Route.Active == tunnelCfg.Tag
		if willBeActive {
			serviceMode = router.ServiceModeSingle
		}
	}

	// Get backend for service creation
	backend := cfg.GetBackendByTag(tunnelCfg.Backend)
	if backend == nil {
		return actions.BackendNotFoundError(tunnelCfg.Backend)
	}

	if err := createTunnelService(tunnelCfg, backend, serviceMode); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	ctx.Output.Status("Service created")

	// Step 5: Set permissions
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Setting permissions...")
	if err := tunnel.SetPermissions(); err != nil {
		ctx.Output.Warning("Permission warning: " + err.Error())
	} else {
		ctx.Output.Status("Permissions set")
	}

	// Step 6: Save config
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Saving configuration...")
	enabled := true
	tunnelCfg.Enabled = &enabled
	cfg.Tunnels = append(cfg.Tunnels, *tunnelCfg)

	// Handle mode-specific config
	if cfg.IsSingleMode() {
		if cfg.Route.Active == "" {
			cfg.Route.Active = tunnelCfg.Tag
		}
	} else {
		if cfg.Route.Default == "" {
			cfg.Route.Default = tunnelCfg.Tag
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	ctx.Output.Status("Configuration saved")

	// Start the tunnel (and regenerate DNS router in multi mode)
	if err := enableAndStartTunnel(ctx, cfg, tunnel); err != nil {
		ctx.Output.Warning("Failed to start tunnel: " + err.Error())
	} else {
		ctx.Output.Status("Tunnel started")
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' created and started!", tunnelCfg.Tag))
	ctx.Output.Println()

	// Show connection info
	ctx.Output.Status(fmt.Sprintf("Transport: %s", config.GetTransportTypeDisplayName(tunnelCfg.Transport)))
	ctx.Output.Status(fmt.Sprintf("Backend: %s", tunnelCfg.Backend))
	ctx.Output.Status(fmt.Sprintf("Domain: %s", tunnelCfg.Domain))
	ctx.Output.Status(fmt.Sprintf("Port: %d", tunnelCfg.Port))

	if fingerprint != "" {
		ctx.Output.Println()
		ctx.Output.Info("Certificate Fingerprint:")
		ctx.Output.Println(certs.FormatFingerprint(fingerprint))
	}
	if publicKey != "" {
		ctx.Output.Println()
		ctx.Output.Info("Public Key:")
		ctx.Output.Println(publicKey)
	}

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	} else {
		ctx.Output.Println()
	}

	return nil
}

// buildBackendOptions builds menu options for backend selection.
func buildBackendOptions(cfg *config.Config, transportType config.TransportType) []tui.MenuOption {
	var options []tui.MenuOption

	for _, b := range cfg.Backends {
		// Check compatibility
		if transportType == config.TransportDNSTT && b.Type == config.BackendShadowsocks {
			continue // DNSTT doesn't support shadowsocks
		}

		typeName := config.GetBackendTypeDisplayName(b.Type)
		label := fmt.Sprintf("%s (%s)", b.Tag, typeName)

		options = append(options, tui.MenuOption{
			Label: label,
			Value: b.Tag,
		})
	}

	return options
}

// createTunnelService creates the systemd service for a tunnel.
// This is a placeholder that will be fully implemented when transport builder is updated.
func createTunnelService(tunnelCfg *config.TunnelConfig, backend *config.BackendConfig, mode router.ServiceMode) error {
	// TODO: This will be implemented properly in Phase 8 when transport builder is updated
	// For now, create a basic service based on transport type

	tunnel := router.NewTunnel(tunnelCfg)

	// Get bind options based on mode
	sg := router.NewServiceGenerator()
	bindOpts, err := sg.GetBindOptions(tunnelCfg, mode)
	if err != nil {
		return err
	}

	// Build the service using the transport builder
	builder := transport.NewBuilder()
	result, err := builder.BuildTunnelService(tunnelCfg, backend, bindOpts)
	if err != nil {
		return fmt.Errorf("failed to build service: %w", err)
	}

	// Create the systemd service
	if err := result.CreateService(tunnel.ServiceName); err != nil {
		return err
	}

	return nil
}
