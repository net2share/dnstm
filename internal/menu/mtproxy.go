package menu

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/mtproxy"
	"github.com/net2share/go-corelib/tui"
)

// RunMTProxyMenu shows the MTProxy management menu.
func RunMTProxyMenu() error {
	for {
		fmt.Println()

		installed := mtproxy.IsMTProxyInstalled()
		running := mtproxy.IsMTProxyRunning()

		// Show status line
		statusLine := getMTProxyStatusLine(installed, running)
		tui.PrintInfo(fmt.Sprintf("Status: %s", statusLine))

		options := buildMTProxyMenuOptions(installed, running)
		var choice string

		err := huh.NewSelect[string]().
			Title("MTProxy (Telegram Proxy)").
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
		} else {
			options = append(options, huh.NewOption("Start", "start"))
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
	case "start":
		return startMTProxy()
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

	isReinstall := mtproxy.IsMTProxyInstalled()

	// Get domain from user
	var domain string
	err := huh.NewInput().
		Title("Domain").
		Description("Enter the domain/IP for the proxy URL (e.g., proxy.example.com)").
		Value(&domain).
		Run()
	if err != nil {
		return errCancelled
	}

	if domain == "" {
		tui.PrintError("Domain is required for generating connection URL")
		return errCancelled
	}

	// Generate secret
	secret, err := mtproxy.GenerateSecret()
	if err != nil {
		return fmt.Errorf("failed to generate secret: %w", err)
	}

	tui.PrintStatus(fmt.Sprintf("Using MTProxy secret: %s", secret))

	totalSteps := 3
	currentStep := 0

	// Step 1: Install MTProxy
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Installing MTProxy...")

	progressFn := func(downloaded, total int64) {
		if total > 0 {
			percent := float64(downloaded) / float64(total) * 100
			fmt.Printf("\rDownloading: %.1f%%", percent)
		}
	}

	if err := mtproxy.InstallMTProxy(secret, progressFn); err != nil {
		return fmt.Errorf("failed to install MTProxy: %w", err)
	}
	tui.PrintStatus("MTProxy installed")

	// Step 2: Configure MTProxy
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Configuring MTProxy...")
	if err := mtproxy.ConfigureMTProxy(secret); err != nil {
		return fmt.Errorf("failed to configure MTProxy: %w", err)
	}
	tui.PrintStatus("MTProxy configured")

	// Step 3: Show connection info
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Generating connection URL...")

	proxyUrl := mtproxy.FormatProxyURL(secret, domain)

	fmt.Println()
	if isReinstall {
		tui.PrintSuccess("MTProxy reinstalled and running!")
	} else {
		tui.PrintSuccess("MTProxy installed and running!")
	}
	fmt.Println()

	tui.PrintBox("MTProxy Connection Info", []string{
		tui.KV("Domain:    ", domain),
		tui.KV("Port:      ", mtproxy.MTProxyPort),
		tui.KV("Secret:    ", secret),
		"",
		"Connection URL:",
		proxyUrl,
	})

	fmt.Println()
	tui.PrintInfo("Share the connection URL with Telegram users to connect via this proxy.")

	return nil
}

func uninstallMTProxy() error {
	fmt.Println()

	if !mtproxy.IsMTProxyInstalled() {
		tui.PrintInfo("MTProxy is not installed")
		return nil
	}

	var confirm bool
	err := huh.NewConfirm().
		Title("Are you sure you want to uninstall MTProxy?").
		Description("This will stop and remove MTProxy completely").
		Value(&confirm).
		Run()
	if err != nil {
		return errCancelled
	}

	if !confirm {
		tui.PrintInfo("Uninstall cancelled")
		return errCancelled
	}

	tui.PrintInfo("Removing MTProxy...")
	if err := mtproxy.UninstallMTProxy(); err != nil {
		return fmt.Errorf("failed to uninstall MTProxy: %w", err)
	}

	tui.PrintSuccess("MTProxy removed!")
	return nil
}

func startMTProxy() error {
	fmt.Println()

	if !mtproxy.IsMTProxyInstalled() {
		tui.PrintError("MTProxy is not installed. Install it first.")
		return errCancelled
	}

	if mtproxy.IsMTProxyRunning() {
		tui.PrintInfo("MTProxy is already running")
		return nil
	}

	tui.PrintInfo("Starting MTProxy...")
	if err := mtproxy.StartMTProxy(); err != nil {
		return fmt.Errorf("failed to start MTProxy: %w", err)
	}
	tui.PrintSuccess("MTProxy started!")
	return nil
}

func stopMTProxy() error {
	fmt.Println()
	tui.PrintInfo("Stopping MTProxy...")
	if err := mtproxy.StopMTProxy(); err != nil {
		return fmt.Errorf("failed to stop MTProxy: %w", err)
	}
	tui.PrintStatus("MTProxy stopped")
	return nil
}

func restartMTProxy() error {
	fmt.Println()
	tui.PrintInfo("Restarting MTProxy...")
	if err := mtproxy.RestartMTProxy(); err != nil {
		return fmt.Errorf("failed to restart MTProxy: %w", err)
	}
	tui.PrintStatus("MTProxy restarted")
	return nil
}

func showMTProxyStatus() {
	fmt.Println()

	installed := mtproxy.IsMTProxyInstalled()
	running := mtproxy.IsMTProxyRunning()

	installedStr := "No"
	if installed {
		installedStr = "Yes"
	}

	runningStr := "No"
	if running {
		runningStr = "Yes"
	}

	lines := []string{
		tui.KV("Installed: ", installedStr),
		tui.KV("Running:   ", runningStr),
	}

	if installed {
		lines = append(lines, tui.KV("Port:      ", mtproxy.MTProxyPort))
		lines = append(lines, tui.KV("Stats Port:", mtproxy.MTProxyStatsPort))
	}

	tui.PrintBox("MTProxy Status", lines)
}

func getMTProxyStatusLine(installed, running bool) string {
	if !installed {
		return "Not installed"
	}
	if running {
		return fmt.Sprintf("Running on port %s", mtproxy.MTProxyPort)
	}
	return "Installed but stopped"
}
