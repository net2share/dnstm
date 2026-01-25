package menu

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/go-corelib/tui"
)

// RunSOCKSProxyMenu shows the SOCKS proxy management menu.
func RunSOCKSProxyMenu() error {
	for {
		fmt.Println()

		installed := proxy.IsMicrosocksInstalled()
		running := proxy.IsMicrosocksRunning()

		// Show status line
		statusLine := getSOCKSStatusLine(installed, running)
		tui.PrintInfo(fmt.Sprintf("Status: %s", statusLine))

		options := buildSOCKSMenuOptions(installed, running)
		var choice string

		err := huh.NewSelect[string]().
			Title("SOCKS Proxy (microsocks)").
			Options(options...).
			Value(&choice).
			Run()

		if err != nil {
			return err
		}

		if choice == "back" {
			return nil
		}

		err = handleSOCKSChoice(choice, installed, running)
		if errors.Is(err, errCancelled) {
			continue
		}
		if err != nil {
			tui.PrintError(err.Error())
		}
		tui.WaitForEnter()
	}
}

func buildSOCKSMenuOptions(installed, running bool) []huh.Option[string] {
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

func handleSOCKSChoice(choice string, installed, running bool) error {
	switch choice {
	case "install":
		return installSOCKSProxy()
	case "uninstall":
		return uninstallSOCKSProxy()
	case "stop":
		return stopSOCKSProxy()
	case "restart":
		return restartSOCKSProxy()
	case "status":
		showSOCKSStatus()
		return nil
	}
	return nil
}

func installSOCKSProxy() error {
	fmt.Println()

	isReinstall := proxy.IsMicrosocksInstalled()

	tui.PrintStep(1, 3, "Downloading microsocks...")
	if err := proxy.InstallMicrosocks(tui.PrintProgress); err != nil {
		return fmt.Errorf("failed to install microsocks: %w", err)
	}
	tui.ClearLine()
	tui.PrintStatus("microsocks downloaded")

	tui.PrintStep(2, 3, "Configuring microsocks service...")
	if err := proxy.ConfigureMicrosocks(); err != nil {
		return fmt.Errorf("failed to configure microsocks: %w", err)
	}
	tui.PrintStatus("microsocks configured")

	tui.PrintStep(3, 3, "Starting microsocks service...")
	if err := proxy.StartMicrosocks(); err != nil {
		return fmt.Errorf("failed to start microsocks: %w", err)
	}
	tui.PrintStatus("microsocks started")

	fmt.Println()
	if isReinstall {
		tui.PrintSuccess("SOCKS proxy reinstalled and running!")
	} else {
		tui.PrintSuccess("SOCKS proxy installed and running!")
	}
	tui.PrintInfo(fmt.Sprintf("Listening on %s:%s", proxy.MicrosocksBindAddr, proxy.MicrosocksPort))

	return nil
}

func uninstallSOCKSProxy() error {
	fmt.Println()

	if !proxy.IsMicrosocksInstalled() {
		tui.PrintInfo("microsocks is not installed")
		return nil
	}

	var confirm bool
	err := huh.NewConfirm().
		Title("Are you sure you want to uninstall the SOCKS proxy?").
		Value(&confirm).
		Run()
	if err != nil {
		return errCancelled
	}

	if !confirm {
		tui.PrintInfo("Uninstall cancelled")
		return errCancelled
	}

	tui.PrintInfo("Removing microsocks...")
	if err := proxy.UninstallMicrosocks(); err != nil {
		return fmt.Errorf("failed to uninstall microsocks: %w", err)
	}

	tui.PrintSuccess("SOCKS proxy removed!")
	return nil
}

func stopSOCKSProxy() error {
	fmt.Println()
	tui.PrintInfo("Stopping microsocks...")
	if err := proxy.StopMicrosocks(); err != nil {
		return fmt.Errorf("failed to stop microsocks: %w", err)
	}
	tui.PrintStatus("microsocks stopped")
	return nil
}

func restartSOCKSProxy() error {
	fmt.Println()
	tui.PrintInfo("Restarting microsocks...")
	if err := proxy.RestartMicrosocks(); err != nil {
		return fmt.Errorf("failed to restart microsocks: %w", err)
	}
	tui.PrintStatus("microsocks restarted")
	return nil
}

func showSOCKSStatus() {
	fmt.Println()

	installed := proxy.IsMicrosocksInstalled()
	running := proxy.IsMicrosocksRunning()

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
		lines = append(lines, tui.KV("Bind:      ", proxy.MicrosocksBindAddr))
		lines = append(lines, tui.KV("Port:      ", proxy.MicrosocksPort))
	}

	tui.PrintBox("SOCKS Proxy Status", lines)
}

func getSOCKSStatusLine(installed, running bool) string {
	if !installed {
		return "Not installed"
	}
	if running {
		return fmt.Sprintf("Running on %s:%s", proxy.MicrosocksBindAddr, proxy.MicrosocksPort)
	}
	return "Installed but stopped"
}
