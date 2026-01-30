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
)

func init() {
	actions.SetConfigHandler(actions.ActionConfigLoad, HandleConfigLoad)
}

// HandleConfigLoad loads configuration from a file.
func HandleConfigLoad(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, false); err != nil {
		return err
	}

	filePath := ctx.GetArg(0)
	if filePath == "" {
		return actions.NewActionError("file path required", "Usage: dnstm config load <file>")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return actions.NewActionError(
			fmt.Sprintf("file not found: %s", filePath),
			"Please provide a valid config.json file path",
		)
	}

	ctx.Output.Println()
	ctx.Output.Info(fmt.Sprintf("Loading configuration from %s...", filePath))
	ctx.Output.Println()

	// Load the configuration from the file
	newCfg, err := config.LoadFromPath(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Preserve the proxy port from existing config if available
	existingCfg, err := config.Load()
	if err == nil && existingCfg.Proxy.Port != 0 {
		newCfg.Proxy.Port = existingCfg.Proxy.Port
	}

	// Add built-in backends before validation so users can reference them
	newCfg.EnsureBuiltinBackends()

	// Validate the configuration
	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	ctx.Output.Status("Configuration validated")

	// Apply defaults
	newCfg.ApplyDefaults()

	// Save to the system config location
	if err := newCfg.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	ctx.Output.Status("Configuration saved to " + config.GetConfigPath())

	// Create tunnel services for all tunnels
	if len(newCfg.Tunnels) > 0 {
		ctx.Output.Println()
		ctx.Output.Info("Creating tunnel services...")
		for i := range newCfg.Tunnels {
			tunnelCfg := &newCfg.Tunnels[i]
			if err := ensureTunnelService(ctx, tunnelCfg, newCfg); err != nil {
				ctx.Output.Warning(fmt.Sprintf("Failed to create service for %s: %v", tunnelCfg.Tag, err))
			} else {
				ctx.Output.Status(fmt.Sprintf("Service created for %s", tunnelCfg.Tag))
			}
		}
	}

	ctx.Output.Println()
	ctx.Output.Success("Configuration loaded successfully!")
	ctx.Output.Println()

	// Show summary
	ctx.Output.Info("Summary:")
	ctx.Output.Printf("  Mode:     %s\n", GetModeDisplayName(newCfg.Route.Mode))
	ctx.Output.Printf("  Backends: %d\n", len(newCfg.Backends))
	ctx.Output.Printf("  Tunnels:  %d\n", len(newCfg.Tunnels))
	ctx.Output.Println()

	ctx.Output.Info("Run 'dnstm router start' to apply the new configuration.")
	ctx.Output.Println()

	return nil
}

// ensureTunnelService ensures a tunnel has its service and crypto material created.
func ensureTunnelService(ctx *actions.Context, tunnelCfg *config.TunnelConfig, cfg *config.Config) error {
	// Ensure transport binaries are installed
	if err := transport.EnsureTransportBinariesInstalled(tunnelCfg.Transport); err != nil {
		return fmt.Errorf("failed to install transport binaries: %w", err)
	}

	// Create tunnel config directory
	tunnelDir := filepath.Join(config.TunnelsDir, tunnelCfg.Tag)
	if err := os.MkdirAll(tunnelDir, 0750); err != nil {
		return fmt.Errorf("failed to create tunnel directory: %w", err)
	}

	// Generate crypto material if needed
	if tunnelCfg.Transport == config.TransportSlipstream {
		certMgr := certs.NewManager()
		certInfo, err := certMgr.GetOrCreate(tunnelCfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to generate certificate: %w", err)
		}
		// Update cert paths if not already set
		if tunnelCfg.Slipstream == nil {
			tunnelCfg.Slipstream = &config.SlipstreamConfig{}
		}
		if tunnelCfg.Slipstream.Cert == "" {
			tunnelCfg.Slipstream.Cert = certInfo.CertPath
		}
		if tunnelCfg.Slipstream.Key == "" {
			tunnelCfg.Slipstream.Key = certInfo.KeyPath
		}
	} else if tunnelCfg.Transport == config.TransportDNSTT {
		keyMgr := keys.NewManager()
		keyInfo, err := keyMgr.GetOrCreate(tunnelCfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
		}
		// Update key path if not already set
		if tunnelCfg.DNSTT == nil {
			tunnelCfg.DNSTT = &config.DNSTTConfig{MTU: 1232}
		}
		if tunnelCfg.DNSTT.PrivateKey == "" {
			tunnelCfg.DNSTT.PrivateKey = keyInfo.PrivateKeyPath
		}
	}

	// Get backend
	backend := cfg.GetBackendByTag(tunnelCfg.Backend)
	if backend == nil {
		return fmt.Errorf("backend '%s' not found", tunnelCfg.Backend)
	}

	// Determine service mode
	serviceMode := router.ServiceModeMulti
	if cfg.IsSingleMode() {
		willBeActive := cfg.Route.Active == "" || cfg.Route.Active == tunnelCfg.Tag
		if willBeActive {
			serviceMode = router.ServiceModeSingle
		}
	}

	// Create service
	return createTunnelService(tunnelCfg, backend, serviceMode)
}
