package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/system"
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
			{Label: "VayDNS", Value: string(config.TransportVayDNS)},
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

	// Get MTU for DNSTT/VayDNS
	mtu := 1232
	if config.TransportType(transportType) == config.TransportDNSTT || config.TransportType(transportType) == config.TransportVayDNS {
		for {
			mtuStr, confirmed, mtuErr := tui.RunInput(tui.InputConfig{
				Title:       "MTU",
				Description: "DNS packet MTU (512-1400)",
				Value:       "1232",
			})
			if mtuErr != nil {
				return mtuErr
			}
			if !confirmed {
				return nil
			}
			if mtuStr == "" {
				mtuStr = "1232"
			}
			parsed, parseErr := strconv.Atoi(mtuStr)
			if parseErr != nil || parsed < 512 || parsed > 1400 {
				ctx.Output.Error("MTU must be a number between 512 and 1400")
				continue
			}
			mtu = parsed
			break
		}
	}

	var vaydnsDnsttCompat bool
	var vaydnsClientIDSize, vaydnsQueueSize int
	var vaydnsIdleTimeout, vaydnsKeepAlive string
	if config.TransportType(transportType) == config.TransportVayDNS {
		confirm, confirmErr := tui.RunConfirm(tui.ConfirmConfig{
			Title:       "DNSTT-compatible wire format?",
			Description: "Enable only if clients use legacy dnstt-compatible mode (-dnstt-compat). Uses longer idle timeout and 8-byte client IDs on the server.",
		})
		if confirmErr != nil {
			return confirmErr
		}
		vaydnsDnsttCompat = confirm

		// clientid_size: only if not dnstt-compat (compat forces 8-byte)
		if !vaydnsDnsttCompat {
			for {
				cidStr, confirmed, cidErr := tui.RunInput(tui.InputConfig{
					Title:       "Client ID Size",
					Description: "Client ID size in bytes (1-8)",
					Value:       "2",
				})
				if cidErr != nil {
					return cidErr
				}
				if !confirmed {
					return nil
				}
				if cidStr == "" {
					cidStr = "2"
				}
				parsed, parseErr := strconv.Atoi(cidStr)
				if parseErr != nil || parsed < 1 || parsed > 8 {
					ctx.Output.Error("Client ID size must be between 1 and 8")
					continue
				}
				vaydnsClientIDSize = parsed
				break
			}
		}

		// idle_timeout
		defaultIdle := "60s"
		if vaydnsDnsttCompat {
			defaultIdle = "2m"
		}
		for {
			idleStr, confirmed, idleErr := tui.RunInput(tui.InputConfig{
				Title:       "Idle Timeout",
				Description: "Session idle timeout (e.g. 60s, 2m)",
				Value:       defaultIdle,
			})
			if idleErr != nil {
				return idleErr
			}
			if !confirmed {
				return nil
			}
			if idleStr == "" {
				idleStr = defaultIdle
			}
			if _, parseErr := time.ParseDuration(idleStr); parseErr != nil {
				ctx.Output.Error("Invalid duration format (e.g. 60s, 2m)")
				continue
			}
			vaydnsIdleTimeout = idleStr
			break
		}

		// keepalive
		for {
			keepStr, confirmed, keepErr := tui.RunInput(tui.InputConfig{
				Title:       "Keepalive Interval",
				Description: "Keepalive ping interval; must be less than idle timeout",
				Value:       "10s",
			})
			if keepErr != nil {
				return keepErr
			}
			if !confirmed {
				return nil
			}
			if keepStr == "" {
				keepStr = "10s"
			}
			keepDur, parseErr := time.ParseDuration(keepStr)
			if parseErr != nil {
				ctx.Output.Error("Invalid duration format (e.g. 10s, 5s)")
				continue
			}
			idleDur, _ := time.ParseDuration(vaydnsIdleTimeout)
			if keepDur >= idleDur {
				ctx.Output.Error("Keepalive must be less than idle timeout")
				continue
			}
			vaydnsKeepAlive = keepStr
			break
		}

		// queue-size
		for {
			qsStr, confirmed, qsErr := tui.RunInput(tui.InputConfig{
				Title:       "Queue Size",
				Description: "Packet queue size for transport (1-65535)",
				Value:       "512",
			})
			if qsErr != nil {
				return qsErr
			}
			if !confirmed {
				return nil
			}
			if qsStr == "" {
				qsStr = "512"
			}
			parsed, parseErr := strconv.Atoi(qsStr)
			if parseErr != nil || parsed < 1 || parsed > 65535 {
				ctx.Output.Error("Queue size must be between 1 and 65535")
				continue
			}
			vaydnsQueueSize = parsed
			break
		}
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
		tunnelCfg.DNSTT = &config.DNSTTConfig{MTU: mtu}
	}
	if tunnelCfg.Transport == config.TransportVayDNS {
		tunnelCfg.VayDNS = &config.VayDNSConfig{
			MTU:          mtu,
			DnsttCompat:  vaydnsDnsttCompat,
			ClientIDSize: vaydnsClientIDSize,
			IdleTimeout:  vaydnsIdleTimeout,
			KeepAlive:    vaydnsKeepAlive,
			QueueSize:    vaydnsQueueSize,
		}
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
	if transportType != config.TransportSlipstream && transportType != config.TransportDNSTT && transportType != config.TransportVayDNS {
		return fmt.Errorf("invalid transport type: %s (must be slipstream, dnstt, or vaydns)", transportType)
	}

	// Validate backend exists and is compatible
	backend := cfg.GetBackendByTag(backendTag)
	if backend == nil {
		return actions.BackendNotFoundError(backendTag)
	}

	// Check transport-backend compatibility
	if (transportType == config.TransportDNSTT || transportType == config.TransportVayDNS) && backend.Type == config.BackendShadowsocks {
		return actions.NewActionError(
			"incompatible transport and backend",
			fmt.Sprintf("%s transport does not support Shadowsocks backend", config.GetTransportTypeDisplayName(transportType)),
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
	if transportType == config.TransportVayDNS {
		if mtu == 0 {
			mtu = 1232
		}
		dnsttCompat := ctx.GetBool("dnstt-compat")
		cid := ctx.GetInt("clientid-size")

		// clientid-size is ignored by vaydns-server when dnstt-compat is set (forced to 8)
		if dnsttCompat && cid != 0 {
			return fmt.Errorf("--clientid-size cannot be used with --dnstt-compat (compat mode forces 8-byte client IDs)")
		}

		v := &config.VayDNSConfig{
			MTU:           mtu,
			DnsttCompat:   dnsttCompat,
			ClientIDSize:  cid,
			IdleTimeout:   ctx.GetString("idle-timeout"),
			KeepAlive:     ctx.GetString("keepalive"),
			Fallback:      ctx.GetString("fallback"),
			QueueSize:     ctx.GetInt("queue-size"),
			KCPWindowSize: ctx.GetInt("kcp-window-size"),
			QueueOverflow: ctx.GetString("queue-overflow"),
			LogLevel:      ctx.GetString("log-level"),
		}
		tunnelCfg.VayDNS = v
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

	// Step 2: Create tunnel config directory
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Creating tunnel configuration...")
	tunnelDir := filepath.Join(config.TunnelsDir, tunnelCfg.Tag)
	if err := os.MkdirAll(tunnelDir, 0750); err != nil {
		return fmt.Errorf("failed to create tunnel directory: %w", err)
	}
	if err := system.ChownDirToDnstm(tunnelDir); err != nil {
		_ = err
	}
	ctx.Output.Status("Tunnel directory created")

	// Step 3: Generate certificates/keys into tunnel directory
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Generating cryptographic material...")
	var fingerprint string
	var publicKey string
	if tunnelCfg.Transport == config.TransportSlipstream {
		certInfo, err := certs.GetOrCreateInDir(tunnelDir, tunnelCfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to generate certificate: %w", err)
		}
		fingerprint = certInfo.Fingerprint
		tunnelCfg.Slipstream = &config.SlipstreamConfig{
			Cert: certInfo.CertPath,
			Key:  certInfo.KeyPath,
		}
		ctx.Output.Status("TLS certificate ready")
	} else if tunnelCfg.Transport == config.TransportDNSTT {
		keyInfo, err := keys.GetOrCreateInDir(tunnelDir)
		if err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
		}
		publicKey = keyInfo.PublicKey
		tunnelCfg.DNSTT.PrivateKey = keyInfo.PrivateKeyPath
		ctx.Output.Status("Curve25519 keys ready")
	} else if tunnelCfg.Transport == config.TransportVayDNS {
		keyInfo, err := keys.GetOrCreateInDir(tunnelDir)
		if err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
		}
		publicKey = keyInfo.PublicKey
		tunnelCfg.VayDNS.PrivateKey = keyInfo.PrivateKeyPath
		ctx.Output.Status("Curve25519 keys ready")
	}

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
		// Check compatibility: DNSTT and VayDNS don't support shadowsocks
		if (transportType == config.TransportDNSTT || transportType == config.TransportVayDNS) && b.Type == config.BackendShadowsocks {
			continue
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
