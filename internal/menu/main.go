// Package menu provides the interactive menu for dnstm.
package menu

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/installer"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/sshtunnel"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

// errCancelled is returned when user cancels/backs out (no WaitForEnter needed).
var errCancelled = errors.New("cancelled")

// Version and BuildTime are set by cmd package.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// RunInteractive shows the main interactive menu.
func RunInteractive() error {
	installer.PrintBanner(Version, BuildTime)

	osInfo, err := osdetect.Detect()
	if err != nil {
		tui.PrintWarning("Could not detect OS: " + err.Error())
	} else {
		tui.PrintInfo(fmt.Sprintf("Detected OS: %s", osInfo.PrettyName))
	}

	archInfo := installer.DetectArch()
	tui.PrintInfo(fmt.Sprintf("Architecture: %s", archInfo.Arch))

	return runMenuLoop(osInfo, archInfo)
}

func runMenuLoop(osInfo *osdetect.OSInfo, archInfo *installer.ArchInfo) error {
	for {
		fmt.Println()

		// Check current state
		isInstalled := service.IsInstalled() && config.Exists()

		options := buildMenuOptions(isInstalled)
		var choice string

		err := huh.NewSelect[string]().
			Title("DNS Tunnel Manager").
			Options(options...).
			Value(&choice).
			Run()

		if err != nil {
			return err
		}

		if choice == "exit" {
			tui.PrintInfo("Goodbye!")
			return nil
		}

		err = handleChoice(choice, osInfo, archInfo, isInstalled)
		if errors.Is(err, errCancelled) {
			continue
		}
		if err != nil {
			tui.PrintError(err.Error())
		}
		tui.WaitForEnter()
	}
}

func buildMenuOptions(isInstalled bool) []huh.Option[string] {
	if isInstalled {
		return []huh.Option[string]{
			huh.NewOption("Reconfigure dnstt server", "install"),
			huh.NewOption("Check service status", "status"),
			huh.NewOption("View service logs", "logs"),
			huh.NewOption("Show configuration", "config"),
			huh.NewOption("Restart service", "restart"),
			huh.NewOption("Manage SSH tunnel users", "ssh-users"),
			huh.NewOption("Uninstall", "uninstall"),
			huh.NewOption("Exit", "exit"),
		}
	}
	return []huh.Option[string]{
		huh.NewOption("Install dnstt server", "install"),
		huh.NewOption("Manage SSH tunnel users", "ssh-users"),
		huh.NewOption("Exit", "exit"),
	}
}

func handleChoice(choice string, osInfo *osdetect.OSInfo, archInfo *installer.ArchInfo, isInstalled bool) error {
	switch choice {
	case "install":
		return installer.RunInteractive(osInfo, archInfo)
	case "status":
		showStatus()
		return nil
	case "logs":
		showLogs()
		return nil
	case "config":
		showConfig()
		return nil
	case "restart":
		restartService()
		return nil
	case "ssh-users":
		sshtunnel.ShowMenu()
		return errCancelled // Submenu handles its own flow
	case "uninstall":
		installer.RunUninstallInteractive()
		return errCancelled // Submenu handles its own flow
	}
	return nil
}

func showStatus() {
	fmt.Println()
	status, _ := service.Status()
	fmt.Println(status)
}

func showLogs() {
	fmt.Println()
	logs, err := service.GetLogs(50)
	if err != nil {
		tui.PrintError(err.Error())
	} else {
		fmt.Println(logs)
	}
}

func showConfig() {
	cfg, err := config.Load()
	if err != nil {
		tui.PrintError("Failed to load configuration: " + err.Error())
		return
	}

	publicKey := ""
	if cfg.PublicKeyFile != "" {
		publicKey, _ = keys.ReadPublicKey(cfg.PublicKeyFile)
	}

	lines := []string{
		fmt.Sprintf("NS Subdomain:    %s", cfg.NSSubdomain),
		fmt.Sprintf("Tunnel Mode:     %s", cfg.TunnelMode),
		fmt.Sprintf("MTU:             %s", cfg.MTU),
		fmt.Sprintf("Target Port:     %s", cfg.TargetPort),
		fmt.Sprintf("Private Key:     %s", cfg.PrivateKeyFile),
		fmt.Sprintf("Public Key File: %s", cfg.PublicKeyFile),
		"",
		"Public Key (for client):",
		publicKey,
	}

	tui.PrintBox("Current Configuration", lines)

	if service.IsActive() {
		tui.PrintStatus("Service is running")
	} else {
		tui.PrintWarning("Service is not running")
	}
}

func restartService() {
	tui.PrintInfo("Restarting service...")
	if err := service.Restart(); err != nil {
		tui.PrintError(err.Error())
	} else {
		tui.PrintStatus("Service restarted successfully")
	}
}
