package menu

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/mtproxy"
	"github.com/net2share/dnstm/internal/tunnel"
	"github.com/net2share/go-corelib/tui"
)

// RunMTProxyMenu shows the MTProxy management menu.
func RunMTProxyMenu() error {
	for {
		fmt.Println()

		installed := mtproxy.IsMtProxyInstalled()
		running := mtproxy.IsMTProxyRunning()

		// Show status line
		statusLine := getMTProxyStatusLine(installed, running)
		tui.PrintInfo(fmt.Sprintf("Status: %s", statusLine))

		options := buildMTProxyMenuOptions(installed, running)
		var choice string

		err := huh.NewSelect[string]().
			Title("MTProxy (Telegram)").
			Options(options...).
			Value(&choice).
			Run()

		if err != nil {
			return err
		}

		if choice == "back" {
			return nil
		}

		err = handleMTProxyChoice(choice, installed, running)
		if errors.Is(err, errCancelled) {
			continue
		}
		if err != nil {
			tui.PrintError(err.Error())
		}
		tui.WaitForEnter()
	}
}

func buildMTProxyMenuOptions(installed, running bool) []huh.Option[string] {
	var options []huh.Option[string]

	if installed {
		options = append(options, huh.NewOption("Status", "status"))
		options = append(options, huh.NewOption("Reinstall", "install"))
		options = append(options, huh.NewOption("Restart", "restart"))
		if running {
			options = append(options, huh.NewOption("Stop", "stop"))
		}
		options = append(options, huh.NewOption("Uninstall", "uninstall"))
	} else {
		options = append(options, huh.NewOption("Install", "install"))
	}

	options = append(options, huh.NewOption("Back", "back"))

	return options
}

func handleMTProxyChoice(choice string, installed, running bool) error {
	switch choice {
	case "install":
		return installMTProxy()
	case "uninstall":
		return uninstallMTProxy()
	case "stop":
		return stopMTProxy()
	case "restart":
		return restartMTProxy()
	case "status":
		showMTProxyStatus()
		return nil
	}
	return nil
}

func installMTProxy() error {
	fmt.Println()

	isReinstall := mtproxy.IsMtProxyInstalled()

	secret, err := mtproxy.GenerateSecret()
	if err != nil {
		return fmt.Errorf("failed to generate secret: %w", err)
	}

	tui.PrintStep(1, 3, "Installing MTProxy...")
	if err := mtproxy.InstallMTProxy(secret, tui.PrintProgress); err != nil {
		return fmt.Errorf("failed to install MTProxy: %w", err)
	}
	tui.ClearLine()
	tui.PrintStatus("MTProxy installed")

	tui.PrintStep(2, 3, "Configuring MTProxy service...")
	if err := mtproxy.ConfigureMTProxy(secret); err != nil {
		return fmt.Errorf("failed to configure MTProxy: %w", err)
	}
	tui.PrintStatus("MTProxy configured")

	tui.PrintStep(3, 3, "Starting MTProxy service...")
	if err := mtproxy.StartMTProxy(); err != nil {
		return fmt.Errorf("failed to start MTProxy: %w", err)
	}
	tui.PrintStatus("MTProxy started")

	fmt.Println()

	if isReinstall {
		tui.PrintSuccess("MTProxy has been successfully reinstalled.")
	} else {
		tui.PrintSuccess("MTProxy has been successfully installed and started.")
	}

	fmt.Println()
	tui.PrintInfo("MTProxy is now running on port 8443")

	globalCfg, err := tunnel.LoadGlobalConfig()
	if err == nil && globalCfg != nil {
		domainName := ""
		switch globalCfg.ActiveProvider {
		case tunnel.ProviderDNSTT:
			provider, _ := tunnel.Get(tunnel.ProviderDNSTT)
			if provider != nil {
				if dnsttCfg, err := provider.GetConfig(); err == nil {
					mapped := parseConf(dnsttCfg)
					domainName = mapped["NS Subdomain"]
				}
			}
		case tunnel.ProviderSlipstream:
			provider, _ := tunnel.Get(tunnel.ProviderSlipstream)
			if provider != nil {
				if slipstreamCfg, err := provider.GetConfig(); err == nil {
					mapped := parseConf(slipstreamCfg)
					domainName = mapped["Domain"]
				}
			}
		}

		if domainName != "" {
			proxyURL := mtproxy.FormatProxyURL(secret, domainName)
			fmt.Println()
			tui.PrintInfo("Telegram Proxy URL:")
			fmt.Printf("  %s\n", proxyURL)
			fmt.Println()
			tui.PrintInfo("Share this URL with Telegram users or scan the QR code.")
		} else {
			fmt.Println()
			tui.PrintInfo("Note: To generate a proxy URL, configure your DNS subdomain first.")
		}
	} else {
		fmt.Println()
		tui.PrintInfo("Note: To generate a proxy URL, configure your DNS subdomain first.")
	}

	return nil
}

func parseConf(conf string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(conf, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}
	return result
}

func uninstallMTProxy() error {
	fmt.Println()

	if err := mtproxy.UninstallMTProxy(); err != nil {
		return fmt.Errorf("failed to uninstall MTProxy: %w", err)
	}

	tui.PrintSuccess("MTProxy has been successfully uninstalled.")
	return nil
}

func stopMTProxy() error {
	fmt.Println()

	if err := mtproxy.StopMTProxy(); err != nil {
		return fmt.Errorf("failed to stop MTProxy: %w", err)
	}

	tui.PrintSuccess("MTProxy has been stopped.")
	return nil
}

func restartMTProxy() error {
	fmt.Println()

	if err := mtproxy.RestartMTProxy(); err != nil {
		return fmt.Errorf("failed to restart MTProxy: %w", err)
	}

	tui.PrintSuccess("MTProxy has been restarted.")
	return nil
}

func showMTProxyStatus() {
	fmt.Println()

	installed := mtproxy.IsMtProxyInstalled()
	running := mtproxy.IsMTProxyRunning()

	tui.PrintInfo("MTProxy Status:")
	fmt.Printf("  Installed: %v\n", installed)
	fmt.Printf("  Running: %v\n", running)

	if installed && running {
		// Check if ports are accessible
		clientReachable := mtproxy.CheckPort(8443)
		statsReachable := mtproxy.CheckPort(8888)

		fmt.Println()
		tui.PrintInfo("Port Status:")
		fmt.Printf("  Client Port (8443): %s\n", formatPortStatus(clientReachable))
		fmt.Printf("  Stats Port (8888): %s\n", formatPortStatus(statsReachable))
	}
}

func getMTProxyStatusLine(installed, running bool) string {
	if !installed {
		return "Not installed"
	}
	if running {
		return "Running"
	}
	return "Installed but not running"
}

func formatPortStatus(reachable bool) string {
	if reachable {
		return "✓ Reachable"
	}
	return "✗ Not reachable"
}
