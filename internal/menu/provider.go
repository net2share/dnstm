package menu

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/tunnel"
	"github.com/net2share/dnstm/internal/tunnel/dnstt"
	"github.com/net2share/dnstm/internal/tunnel/slipstream"
	"github.com/net2share/go-corelib/tui"
)

// InteractiveInstaller is an interface for providers that support interactive installation.
type InteractiveInstaller interface {
	RunInteractiveInstall() (*tunnel.InstallResult, error)
}

// RunProviderMenu shows the submenu for a specific provider.
func RunProviderMenu(pt tunnel.ProviderType) error {
	provider, err := tunnel.Get(pt)
	if err != nil {
		return err
	}

	for {
		fmt.Println()

		status, _ := provider.Status()
		isInstalled := status != nil && status.Installed

		globalCfg, _ := tunnel.LoadGlobalConfig()
		isActiveProvider := globalCfg != nil && globalCfg.ActiveProvider == pt

		options := buildProviderMenuOptions(provider, isInstalled, isActiveProvider)
		var choice string

		menuTitle := fmt.Sprintf("%s Server", provider.DisplayName())
		err := huh.NewSelect[string]().
			Title(menuTitle).
			Options(options...).
			Value(&choice).
			Run()

		if err != nil {
			return err
		}

		if choice == "back" {
			return nil
		}

		err = handleProviderChoice(choice, provider, isInstalled)
		if errors.Is(err, errCancelled) {
			continue
		}
		if err != nil {
			tui.PrintError(err.Error())
		}
		tui.WaitForEnter()
	}
}

func buildProviderMenuOptions(provider tunnel.Provider, isInstalled bool, isActiveProvider bool) []huh.Option[string] {
	var options []huh.Option[string]

	if isInstalled {
		options = append(options, huh.NewOption("Reconfigure", "install"))
		options = append(options, huh.NewOption("Service status", "status"))
		options = append(options, huh.NewOption("Logs", "logs"))
		options = append(options, huh.NewOption("Show configuration", "config"))
		options = append(options, huh.NewOption("Restart service", "restart"))

		if !isActiveProvider {
			options = append(options, huh.NewOption("Set as Active DNS Handler", "set-active"))
		}

		options = append(options, huh.NewOption("Uninstall", "uninstall"))
	} else {
		options = append(options, huh.NewOption("Install", "install"))
	}

	options = append(options, huh.NewOption("Back", "back"))

	return options
}

func handleProviderChoice(choice string, provider tunnel.Provider, isInstalled bool) error {
	switch choice {
	case "install":
		return runProviderInstall(provider)
	case "status":
		showProviderStatus(provider)
		return nil
	case "logs":
		showProviderLogs(provider)
		return nil
	case "config":
		showProviderConfig(provider)
		return nil
	case "restart":
		restartProviderService(provider)
		return nil
	case "set-active":
		return setAsActiveProvider(provider)
	case "uninstall":
		return runProviderUninstall(provider)
	}
	return nil
}

func runProviderInstall(provider tunnel.Provider) error {
	// Check if provider supports interactive installation
	if installer, ok := provider.(InteractiveInstaller); ok {
		result, err := installer.RunInteractiveInstall()
		if err != nil {
			return err
		}
		if result != nil {
			showInstallSuccess(provider, result)
		}
		return nil
	}

	// Fall back to basic install if interactive not supported
	return fmt.Errorf("interactive installation not supported for %s", provider.DisplayName())
}

func showProviderStatus(provider tunnel.Provider) {
	fmt.Println()
	status, _ := provider.GetServiceStatus()
	fmt.Println(status)
}

func showProviderLogs(provider tunnel.Provider) {
	fmt.Println()
	logs, err := provider.GetLogs(50)
	if err != nil {
		tui.PrintError(err.Error())
	} else {
		fmt.Println(logs)
	}
}

func showProviderConfig(provider tunnel.Provider) {
	configStr, err := provider.GetConfig()
	if err != nil {
		tui.PrintError("Failed to load configuration: " + err.Error())
		return
	}

	lines := []string{configStr}
	tui.PrintBox("Current Configuration", lines)

	status, _ := provider.Status()
	if status != nil && status.Running {
		tui.PrintStatus("Service is running")
	} else {
		tui.PrintWarning("Service is not running")
	}
}

func restartProviderService(provider tunnel.Provider) {
	tui.PrintInfo("Restarting service...")
	if err := provider.Restart(); err != nil {
		tui.PrintError(err.Error())
	} else {
		tui.PrintStatus("Service restarted successfully")
	}
}

func setAsActiveProvider(provider tunnel.Provider) error {
	fmt.Println()
	tui.PrintInfo(fmt.Sprintf("Switching DNS routing to %s...", provider.DisplayName()))

	// Get current active provider
	globalCfg, _ := tunnel.LoadGlobalConfig()
	var currentProvider tunnel.Provider
	var currentPort string
	if globalCfg != nil {
		currentProvider, _ = tunnel.Get(globalCfg.ActiveProvider)
		if currentProvider != nil {
			currentPort = currentProvider.Port()
		}
	}

	newPort := provider.Port()

	// Stop current provider if running
	if currentProvider != nil {
		status, _ := currentProvider.Status()
		if status != nil && status.Running {
			tui.PrintInfo(fmt.Sprintf("Stopping %s...", currentProvider.DisplayName()))
			currentProvider.Stop()
		}
	}

	// Switch firewall rules
	if currentPort != "" && currentPort != newPort {
		tui.PrintInfo("Switching firewall rules...")
		if err := network.SwitchDNSRouting(currentPort, newPort); err != nil {
			tui.PrintWarning("Firewall switch warning: " + err.Error())
		}
	}

	// Update global config
	if err := tunnel.SetActiveProvider(provider.Name()); err != nil {
		return fmt.Errorf("failed to update active provider: %w", err)
	}

	// Start new provider
	tui.PrintInfo(fmt.Sprintf("Starting %s...", provider.DisplayName()))
	if err := provider.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	tui.PrintSuccess(fmt.Sprintf("%s is now the active DNS handler!", provider.DisplayName()))
	return nil
}

func runProviderUninstall(provider tunnel.Provider) error {
	fmt.Println()
	tui.PrintWarning(fmt.Sprintf("This will completely remove %s from your system:", provider.DisplayName()))
	fmt.Println("  - Stop and remove the service")
	fmt.Println("  - Remove the binary")
	fmt.Println("  - Remove all configuration files")
	fmt.Println("  - Remove firewall rules")
	fmt.Println("  - Remove the system user")
	fmt.Println()

	var confirm bool
	err := huh.NewConfirm().
		Title("Are you sure you want to uninstall?").
		Value(&confirm).
		Run()
	if err != nil {
		return errCancelled
	}

	if !confirm {
		tui.PrintInfo("Uninstall cancelled")
		return errCancelled
	}

	// Check if this is the active provider
	globalCfg, _ := tunnel.LoadGlobalConfig()
	if globalCfg != nil && globalCfg.ActiveProvider == provider.Name() {
		// Find another installed provider to switch to
		var otherProvider tunnel.Provider
		for _, pt := range tunnel.Types() {
			if pt == provider.Name() {
				continue
			}
			p, _ := tunnel.Get(pt)
			if p != nil && p.IsInstalled() {
				otherProvider = p
				break
			}
		}

		if otherProvider != nil {
			tui.PrintInfo(fmt.Sprintf("Switching active provider to %s...", otherProvider.DisplayName()))
			tunnel.SetActiveProvider(otherProvider.Name())
		}
	}

	fmt.Println()
	if err := provider.Uninstall(); err != nil {
		return err
	}

	fmt.Println()
	tui.PrintSuccess("Uninstallation complete!")
	tui.PrintInfo(fmt.Sprintf("All %s components have been removed from your system.", provider.DisplayName()))

	return errCancelled // Return to parent menu
}

func showInstallSuccess(provider tunnel.Provider, result *tunnel.InstallResult) {
	lines := []string{
		tui.KV("Domain:       ", result.Domain),
		tui.KV("Tunnel Mode:  ", result.TunnelMode),
	}

	if result.MTU != "" {
		lines = append(lines, tui.KV("MTU:          ", result.MTU))
	}

	// Show public key (DNSTT) or fingerprint (Slipstream)
	if result.PublicKey != "" {
		lines = append(lines, "")
		lines = append(lines, tui.Header("Public Key (for client):"))
		lines = append(lines, tui.Value(result.PublicKey))
	}

	if result.Fingerprint != "" {
		lines = append(lines, "")
		lines = append(lines, tui.Header("Certificate SHA256 Fingerprint:"))
		lines = append(lines, tui.Value(formatFingerprint(result.Fingerprint)))
	}

	tui.PrintBox("Installation Complete!", lines)

	// Show next steps guidance based on tunnel mode
	fmt.Println()
	tui.PrintInfo("Next steps:")
	if result.TunnelMode == "socks" {
		fmt.Println("  Run 'dnstm socks install' to set up the SOCKS proxy")
	} else if result.TunnelMode == "mtproto" {
		fmt.Println("  Run 'dnstm mtproxy install' to set up MTProxy")
		if result.MTProxySecret != "" {
			fmt.Println()
			tui.PrintInfo("MTProxy Configuration:")
			fmt.Printf("  Secret: %s\n", result.MTProxySecret)
			fmt.Printf("  Port:   8443\n")
		}
	} else {
		fmt.Println("  1. Run 'dnstm ssh-users' to configure SSH hardening")
		fmt.Println("  2. Create tunnel users with the SSH users menu")
	}

	fmt.Println()
	tui.PrintInfo("Useful commands:")
	fmt.Println(tui.KV(fmt.Sprintf("  systemctl status %s  ", provider.ServiceName()), "- Check service status"))
	fmt.Println(tui.KV(fmt.Sprintf("  journalctl -u %s -f  ", provider.ServiceName()), "- View live logs"))
	fmt.Println(tui.KV("  dnstm                          ", "- Open this menu"))
}

func formatFingerprint(fingerprint string) string {
	if fingerprint == "" {
		return ""
	}

	// Import from slipstream package
	return slipstream.FormatFingerprint(fingerprint)
}

// Ensure the providers are registered
var _ = dnstt.NewProvider
var _ = slipstream.NewProvider
