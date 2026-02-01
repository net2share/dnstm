package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/installer"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/system"
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

	// Determine the proxy port to use:
	// 1. If proxy.port is explicitly set in config, use it
	// 2. If a socks backend with localhost address is specified, use its port
	// 3. Otherwise preserve existing system proxy port
	userSpecifiedPort := newCfg.Proxy.Port
	if userSpecifiedPort == 0 {
		// Check if user specified a socks backend with a localhost address
		for _, backend := range newCfg.Backends {
			if backend.Tag == "socks" && backend.Type == config.BackendSOCKS {
				if strings.HasPrefix(backend.Address, "127.0.0.1:") {
					parts := strings.Split(backend.Address, ":")
					if len(parts) == 2 {
						if port, err := strconv.Atoi(parts[1]); err == nil && port > 0 {
							userSpecifiedPort = port
							break
						}
					}
				}
			}
		}
	}

	if userSpecifiedPort != 0 {
		newCfg.Proxy.Port = userSpecifiedPort
	} else {
		existingCfg, err := config.Load()
		if err == nil && existingCfg.Proxy.Port != 0 {
			newCfg.Proxy.Port = existingCfg.Proxy.Port
		}
	}

	// Add built-in backends before validation so users can reference them
	newCfg.EnsureBuiltinBackends()

	// Validate the configuration
	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	ctx.Output.Status("Configuration validated")

	// Clean up existing setup before loading new config
	ctx.Output.Println()
	ctx.Output.Info("Cleaning up existing configuration...")
	cleanupResult := installer.CleanupTunnelsAndRouter(true) // Remove tunnel dirs too
	for _, tag := range cleanupResult.TunnelsRemoved {
		ctx.Output.Status(fmt.Sprintf("Removed tunnel service: %s", tag))
	}
	for tag, err := range cleanupResult.TunnelErrors {
		ctx.Output.Warning(fmt.Sprintf("Failed to remove tunnel %s: %v", tag, err))
	}
	if cleanupResult.RouterStopped {
		ctx.Output.Status("DNS router stopped")
	}
	ctx.Output.Status("Cleanup complete")

	// Apply defaults
	newCfg.ApplyDefaults()

	// Save to the system config location
	if err := newCfg.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	ctx.Output.Status("Configuration saved to " + config.GetConfigPath())

	// Reconfigure microsocks if the proxy port was explicitly specified
	if newCfg.Proxy.Port != 0 && proxy.IsMicrosocksInstalled() {
		if err := proxy.ConfigureMicrosocks(newCfg.Proxy.Port); err != nil {
			ctx.Output.Warning(fmt.Sprintf("Failed to reconfigure microsocks: %v", err))
		} else {
			if err := proxy.RestartMicrosocks(); err != nil {
				ctx.Output.Warning(fmt.Sprintf("Failed to restart microsocks: %v", err))
			} else {
				ctx.Output.Status(fmt.Sprintf("Microsocks reconfigured on port %d", newCfg.Proxy.Port))
			}
		}
	}

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

	// Save config again to persist any updated cert/key paths
	if err := newCfg.Save(); err != nil {
		return fmt.Errorf("failed to save updated configuration: %w", err)
	}

	ctx.Output.Println()
	ctx.Output.Success("Configuration loaded successfully!")
	ctx.Output.Println()

	// Show summary
	ctx.Output.Info("Summary:")
	ctx.Output.Printf("  Config:   %s\n", config.GetConfigPath())
	ctx.Output.Printf("  Mode:     %s\n", GetModeDisplayName(newCfg.Route.Mode))
	ctx.Output.Printf("  Backends: %d\n", len(newCfg.Backends))
	ctx.Output.Printf("  Tunnels:  %d\n", len(newCfg.Tunnels))
	ctx.Output.Println()

	// Start the router automatically
	ctx.Output.Info("Starting router...")
	r, err := router.New(newCfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	if err := r.Start(); err != nil {
		return fmt.Errorf("failed to start router: %w", err)
	}

	ctx.Output.Success("Router started!")
	ctx.Output.Println()

	// Show connection info for each tunnel
	ctx.Output.Info("Connection Info:")
	certMgr := certs.NewManager()
	keyMgr := keys.NewManager()
	for _, tunnel := range newCfg.Tunnels {
		ctx.Output.Printf("\n  %s (%s):\n", tunnel.Tag, tunnel.Domain)
		if tunnel.Transport == config.TransportSlipstream {
			if info := certMgr.Get(tunnel.Domain); info != nil {
				ctx.Output.Printf("    Fingerprint: %s\n", certs.FormatFingerprint(info.Fingerprint))
				ctx.Output.Printf("    Cert:        %s\n", info.CertPath)
				ctx.Output.Printf("    Key:         %s\n", info.KeyPath)
			}
		} else if tunnel.Transport == config.TransportDNSTT {
			if info := keyMgr.Get(tunnel.Domain); info != nil {
				ctx.Output.Printf("    Public Key:   %s\n", info.PublicKey)
				ctx.Output.Printf("    Private Key:  %s\n", info.PrivateKeyPath)
				ctx.Output.Printf("    Public File:  %s\n", info.PublicKeyPath)
			}
		}
	}
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

	// Handle crypto material based on transport type
	if tunnelCfg.Transport == config.TransportSlipstream {
		certMgr := certs.NewManager()

		// Initialize slipstream config if nil
		if tunnelCfg.Slipstream == nil {
			tunnelCfg.Slipstream = &config.SlipstreamConfig{}
		}

		// Check if paths are provided in config
		certProvided := tunnelCfg.Slipstream.Cert != ""
		keyProvided := tunnelCfg.Slipstream.Key != ""

		if certProvided || keyProvided {
			// Both must be provided if one is provided
			if !certProvided || !keyProvided {
				return fmt.Errorf("both cert and key paths must be provided for tunnel %s", tunnelCfg.Tag)
			}

			// Validate cert file exists and is readable by dnstm user
			if _, err := os.Stat(tunnelCfg.Slipstream.Cert); err != nil {
				return fmt.Errorf("certificate file not found: %s", tunnelCfg.Slipstream.Cert)
			}
			canRead, err := system.CanDnstmUserReadFile(tunnelCfg.Slipstream.Cert)
			if err != nil {
				return fmt.Errorf("failed to check certificate permissions: %w", err)
			}
			if !canRead {
				return fmt.Errorf("dnstm user cannot read certificate file: %s", tunnelCfg.Slipstream.Cert)
			}

			// Validate key file exists and is readable by dnstm user
			if _, err := os.Stat(tunnelCfg.Slipstream.Key); err != nil {
				return fmt.Errorf("key file not found: %s", tunnelCfg.Slipstream.Key)
			}
			canRead, err = system.CanDnstmUserReadFile(tunnelCfg.Slipstream.Key)
			if err != nil {
				return fmt.Errorf("failed to check key permissions: %w", err)
			}
			if !canRead {
				return fmt.Errorf("dnstm user cannot read key file: %s", tunnelCfg.Slipstream.Key)
			}

			ctx.Output.Status(fmt.Sprintf("Using provided certificate for %s", tunnelCfg.Domain))
		} else {
			// No paths provided, generate new certificate
			certInfo, err := certMgr.GetOrCreate(tunnelCfg.Domain)
			if err != nil {
				return fmt.Errorf("failed to generate certificate: %w", err)
			}
			tunnelCfg.Slipstream.Cert = certInfo.CertPath
			tunnelCfg.Slipstream.Key = certInfo.KeyPath
			ctx.Output.Status(fmt.Sprintf("Generated certificate for %s", tunnelCfg.Domain))
		}
	} else if tunnelCfg.Transport == config.TransportDNSTT {
		keyMgr := keys.NewManager()

		// Initialize DNSTT config if nil
		if tunnelCfg.DNSTT == nil {
			tunnelCfg.DNSTT = &config.DNSTTConfig{MTU: 1232}
		}

		// Check if private key path is provided
		if tunnelCfg.DNSTT.PrivateKey != "" {
			// Validate key file exists and is readable by dnstm user
			if _, err := os.Stat(tunnelCfg.DNSTT.PrivateKey); err != nil {
				return fmt.Errorf("private key file not found: %s", tunnelCfg.DNSTT.PrivateKey)
			}
			canRead, err := system.CanDnstmUserReadFile(tunnelCfg.DNSTT.PrivateKey)
			if err != nil {
				return fmt.Errorf("failed to check key permissions: %w", err)
			}
			if !canRead {
				return fmt.Errorf("dnstm user cannot read private key file: %s", tunnelCfg.DNSTT.PrivateKey)
			}

			ctx.Output.Status(fmt.Sprintf("Using provided key for %s", tunnelCfg.Domain))
		} else {
			// No key path provided, generate new keys
			keyInfo, err := keyMgr.GetOrCreate(tunnelCfg.Domain)
			if err != nil {
				return fmt.Errorf("failed to generate keys: %w", err)
			}
			tunnelCfg.DNSTT.PrivateKey = keyInfo.PrivateKeyPath
			ctx.Output.Status(fmt.Sprintf("Generated keys for %s", tunnelCfg.Domain))
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

