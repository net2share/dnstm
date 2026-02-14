package handlers

import (
	"fmt"
	"io"
	"os"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/binary"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/dnstm/internal/updater"
)

const installPath = "/usr/local/bin/dnstm"

func init() {
	actions.SetSystemHandler(actions.ActionInstall, HandleInstall)
}

// HandleInstall performs system installation.
func HandleInstall(ctx *actions.Context) error {
	force := ctx.GetBool("force")

	// Check if already installed
	if router.IsInitialized() && !force {
		return fmt.Errorf("dnstm is already installed. Use --force to reinstall")
	}

	modeStr := ctx.GetString("mode")

	// Default to single mode if not specified
	if modeStr == "" {
		modeStr = "single"
	}
	if modeStr != "single" && modeStr != "multi" {
		return fmt.Errorf("invalid mode: %s (must be 'single' or 'multi')", modeStr)
	}

	if ctx.IsInteractive {
		ctx.Output.BeginProgress("Install dnstm")
	} else {
		ctx.Output.Println()
	}

	ctx.Output.Info("Installing dnstm components...")

	// Step 0: Ensure dnstm binary is installed at the standard path
	if err := ensureDnstmInstalled(ctx); err != nil {
		return fmt.Errorf("failed to install dnstm binary: %w", err)
	}

	// Step 1: Create dnstm user
	ctx.Output.Info("Creating dnstm user...")
	if err := system.CreateDnstmUser(); err != nil {
		return fmt.Errorf("failed to create dnstm user: %w", err)
	}
	ctx.Output.Status("dnstm user ready")

	// Step 2: Initialize router
	ctx.Output.Info("Initializing router...")
	if err := router.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize router: %w", err)
	}
	ctx.Output.Status("Router initialized")

	// Step 3: Set operating mode and ensure built-in backends
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg.Route.Mode = modeStr
	cfg.EnsureBuiltinBackends()
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	ctx.Output.Status(fmt.Sprintf("Mode set to %s", GetModeDisplayName(cfg.Route.Mode)))

	// Step 4: Create DNS router service
	svc := dnsrouter.NewService()
	if err := svc.CreateService(); err != nil {
		ctx.Output.Warning("DNS router service: " + err.Error())
	} else {
		ctx.Output.Status("DNS router service created")
	}

	// Step 5: Install binaries
	ctx.Output.Println()
	ctx.Output.Info("Installing transport binaries...")

	// Status callback routes output through the context
	statusFn := func(msg string) { ctx.Output.Status(msg) }

	if err := transport.EnsureDnsttInstalledWithStatus(statusFn); err != nil {
		return fmt.Errorf("failed to install dnstt-server: %w", err)
	}

	if err := transport.EnsureSlipstreamInstalledWithStatus(statusFn); err != nil {
		return fmt.Errorf("failed to install slipstream-server: %w", err)
	}

	if err := transport.EnsureShadowsocksInstalledWithStatus(statusFn); err != nil {
		return fmt.Errorf("failed to install ssserver: %w", err)
	}

	if err := transport.EnsureSSHTunUserInstalledWithStatus(statusFn); err != nil {
		ctx.Output.Warning("sshtun-user: " + err.Error())
	}

	if !proxy.IsMicrosocksInstalled() {
		ctx.Output.Info("Installing microsocks...")
		if err := proxy.InstallMicrosocks(nil); err != nil {
			return fmt.Errorf("failed to install microsocks: %w", err)
		}
	}
	// Ensure microsocks service is configured and running
	if !proxy.IsMicrosocksRunning() {
		ctx.Output.Info("Configuring microsocks service...")
		port, err := proxy.FindAvailablePort()
		if err != nil {
			ctx.Output.Warning("Could not find available port: " + err.Error())
		} else {
			cfg.Proxy.Port = port
			cfg.UpdateSocksBackendPort(port)
			if err := cfg.Save(); err != nil {
				ctx.Output.Warning("Failed to save proxy port: " + err.Error())
			}
			if err := proxy.ConfigureMicrosocks(port); err != nil {
				ctx.Output.Warning("microsocks service config: " + err.Error())
			} else {
				if err := proxy.StartMicrosocks(); err != nil {
					ctx.Output.Warning("microsocks service start: " + err.Error())
				} else {
					ctx.Output.Status(fmt.Sprintf("microsocks installed and running on port %d", port))
				}
			}
		}
	} else {
		ctx.Output.Status("microsocks already running")
	}

	// Step 6: Configure firewall
	ctx.Output.Println()
	ctx.Output.Info("Configuring firewall...")
	network.ClearNATOnly()
	if err := network.AllowPort53(); err != nil {
		ctx.Output.Warning("Firewall configuration: " + err.Error())
	} else {
		ctx.Output.Status("Firewall configured (port 53 UDP/TCP)")
	}

	// Step 7: Create version manifest
	if err := createVersionManifest(ctx); err != nil {
		ctx.Output.Warning("Failed to create version manifest: " + err.Error())
	}

	ctx.Output.Success("Installation complete!")

	// Show next steps (different for CLI vs interactive)
	if ctx.IsInteractive {
		ctx.Output.Println()
		ctx.Output.Info("Next: Select 'Backends' > 'Add' for custom backends (optional)")
		ctx.Output.Info("Next: Select 'Tunnels' > 'Add' to create a tunnel")
		ctx.Output.EndProgress()
	} else {
		ctx.Output.Println()
		ctx.Output.Info("Next steps:")
		ctx.Output.Println("  1. Add backend (optional): dnstm backend add")
		ctx.Output.Println("  2. Add tunnel: dnstm tunnel add")
		ctx.Output.Println()
	}

	return nil
}

// ensureDnstmInstalled copies the current binary to /usr/local/bin/dnstm if needed.
// This ensures services always use the correct binary path.
func ensureDnstmInstalled(ctx *actions.Context) error {
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable: %w", err)
	}

	// If already running from install path, nothing to do
	if currentExe == installPath {
		ctx.Output.Status("dnstm binary already at " + installPath)
		return nil
	}

	// Check if install path exists and is the same file
	destInfo, err := os.Stat(installPath)
	if err == nil {
		srcInfo, err := os.Stat(currentExe)
		if err == nil && os.SameFile(srcInfo, destInfo) {
			ctx.Output.Status("dnstm binary already at " + installPath)
			return nil
		}
	}

	// Copy current binary to install path
	ctx.Output.Info("Installing dnstm binary to " + installPath + "...")

	src, err := os.Open(currentExe)
	if err != nil {
		return fmt.Errorf("failed to open source binary: %w", err)
	}
	defer src.Close()

	// Create temp file first, then rename (atomic)
	tmpPath := installPath + ".tmp"
	dst, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	dst.Close()

	// Rename temp to final (atomic on same filesystem)
	if err := os.Rename(tmpPath, installPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to install binary: %w", err)
	}

	ctx.Output.Status("dnstm binary installed to " + installPath)
	return nil
}

// createVersionManifest creates the initial version manifest after installation.
// Uses pinned versions from binary definitions as the source of truth.
func createVersionManifest(ctx *actions.Context) error {
	manifest := &updater.VersionManifest{}

	binaries := []binary.BinaryType{
		binary.BinarySlipstreamServer,
		binary.BinarySSServer,
		binary.BinaryMicrosocks,
		binary.BinarySSHTunUser,
	}

	for _, binType := range binaries {
		def, ok := binary.GetDef(binType)
		if !ok || def.PinnedVersion == "" {
			continue
		}
		manifest.SetVersion(string(binType), def.PinnedVersion)
	}

	return manifest.Save()
}
