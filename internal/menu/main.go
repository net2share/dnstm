// Package menu provides the interactive menu for dnstm.
package menu

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/sshtunnel"
	"github.com/net2share/dnstm/internal/tunnel"
	_ "github.com/net2share/dnstm/internal/tunnel/dnstt"
	_ "github.com/net2share/dnstm/internal/tunnel/shadowsocks"
	_ "github.com/net2share/dnstm/internal/tunnel/slipstream"
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

// ASCII art banner for dnstm
const dnstmBanner = `
    ____  _   _______  ________  ___
   / __ \/ | / / ___/ /_  __/  |/  /
  / / / /  |/ /\__ \   / / / /|_/ /
 / /_/ / /|  /___/ /  / / / /  / /
/_____/_/ |_//____/  /_/ /_/  /_/
`

// PrintBanner displays the dnstm banner with version info.
func PrintBanner() {
	tui.PrintBanner(tui.BannerConfig{
		AppName:   "DNS Tunnel Manager",
		Version:   Version,
		BuildTime: BuildTime,
		ASCII:     dnstmBanner,
	})
}

// RunInteractive shows the main interactive menu.
func RunInteractive() error {
	PrintBanner()

	osInfo, err := osdetect.Detect()
	if err != nil {
		tui.PrintWarning("Could not detect OS: " + err.Error())
	} else {
		tui.PrintInfo(fmt.Sprintf("Detected OS: %s", osInfo.PrettyName))
	}

	arch := osdetect.GetArch()
	tui.PrintInfo(fmt.Sprintf("Architecture: %s", arch))

	return runMenuLoop()
}

func runMenuLoop() error {
	firstRun := true
	for {
		if !firstRun {
			// Clear screen when returning to main menu from submenus
			tui.ClearScreen()
			PrintBanner()
		}
		firstRun = false

		fmt.Println()

		options := buildMainMenuOptions()
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

		err = handleMainMenuChoice(choice)
		if errors.Is(err, errCancelled) {
			continue
		}
		if err != nil {
			tui.PrintError(err.Error())
			tui.WaitForEnter()
		}
	}
}

func buildMainMenuOptions() []huh.Option[string] {
	var options []huh.Option[string]

	// Add provider submenus
	for _, pt := range tunnel.Types() {
		provider, err := tunnel.Get(pt)
		if err != nil {
			continue
		}

		status, _ := provider.Status()
		label := provider.DisplayName()

		if status != nil && status.Installed {
			if status.Active && status.Running {
				label += " (active)"
			} else if status.Running {
				label += " (running)"
			} else {
				label += " (installed)"
			}
		}

		label += " →"
		options = append(options, huh.NewOption(label, string(pt)))
	}

	// Add common options
	options = append(options, huh.NewOption("Manage SSH tunnel users", "ssh-users"))
	options = append(options, huh.NewOption("Manage SOCKS proxy", "socks"))
	options = append(options, huh.NewOption("Status", "status"))
	options = append(options, huh.NewOption("Exit", "exit"))

	return options
}

func handleMainMenuChoice(choice string) error {
	// Check if it's a provider type
	pt, err := tunnel.ParseProviderType(choice)
	if err == nil {
		return RunProviderMenu(pt)
	}

	switch choice {
	case "status":
		ShowOverallStatus()
		tui.WaitForEnter()
		return errCancelled
	case "ssh-users":
		sshtunnel.ShowMenu()
		return errCancelled // Submenu handles its own flow
	case "socks":
		RunSOCKSProxyMenu()
		return errCancelled // Submenu handles its own flow
	}

	return nil
}
