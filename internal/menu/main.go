// Package menu provides the interactive menu for dnstm.
package menu

import (
	"errors"
	"fmt"
	"os"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

// errCancelled is returned when user cancels/backs out.
var errCancelled = errors.New("cancelled")

// Version and BuildTime are set by cmd package.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

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

	return runMainMenu()
}

// buildTunnelSummary builds a summary string for the main menu header.
func buildTunnelSummary() string {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return ""
	}

	total := len(cfg.Tunnels)
	running := 0
	for _, t := range cfg.Tunnels {
		tunnel := router.NewTunnel(&t)
		if tunnel.IsActive() {
			running++
		}
	}

	if cfg.IsSingleMode() && cfg.Route.Active != "" {
		return fmt.Sprintf("Tunnels: %d | Running: %d | Active: %s", total, running, cfg.Route.Active)
	}
	return fmt.Sprintf("Tunnels: %d | Running: %d", total, running)
}

func runMainMenu() error {
	firstRun := true
	for {
		if !firstRun {
			tui.ClearScreen()
			PrintBanner()
		}
		firstRun = false

		fmt.Println()

		// Check if transport binaries are installed
		installed := transport.IsInstalled()

		var options []tui.MenuOption
		var header string

		if !installed {
			// Not installed - show install option first and limited menu
			missing := transport.GetMissingBinaries()
			tui.PrintWarning("dnstm not installed")
			fmt.Printf("Missing: %v\n\n", missing)

			options = append(options, tui.MenuOption{Label: "Install (Required)", Value: actions.ActionInstall})
			options = append(options, tui.MenuOption{Label: "Exit", Value: "exit"})
		} else {
			// Build tunnel summary for header
			header = buildTunnelSummary()

			// Fully installed - show all options
			options = append(options, tui.MenuOption{Label: "Tunnels →", Value: actions.ActionTunnel})
			options = append(options, tui.MenuOption{Label: "Backends →", Value: actions.ActionBackend})
			options = append(options, tui.MenuOption{Label: "Router →", Value: actions.ActionRouter})
			options = append(options, tui.MenuOption{Label: "SSH Users →", Value: actions.ActionSSHUsers})
			options = append(options, tui.MenuOption{Label: "Uninstall", Value: actions.ActionUninstall})
			options = append(options, tui.MenuOption{Label: "Exit", Value: "exit"})
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Header:  header,
			Title:   "DNS Tunnel Manager",
			Options: options,
		})
		if err != nil {
			return err
		}

		if choice == "" || choice == "exit" {
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

func handleMainMenuChoice(choice string) error {
	switch choice {
	case actions.ActionRouter:
		return RunSubmenu(actions.ActionRouter)
	case actions.ActionTunnel:
		return runTunnelMenu()
	case actions.ActionBackend:
		return RunSubmenu(actions.ActionBackend)
	case actions.ActionSSHUsers:
		return RunAction(actions.ActionSSHUsers)
	case actions.ActionInstall:
		if err := RunAction(actions.ActionInstall); err != nil && err != errCancelled {
			return err
		}
		tui.WaitForEnter()
		return errCancelled
	case actions.ActionUninstall:
		if err := RunAction(actions.ActionUninstall); err != nil && err != errCancelled {
			return err
		}
		os.Exit(0)
	}
	return nil
}

// runTunnelMenu shows the tunnel submenu with special handling for list navigation.
func runTunnelMenu() error {
	for {
		fmt.Println()

		options := []tui.MenuOption{
			{Label: "Add", Value: actions.ActionTunnelAdd},
			{Label: "List →", Value: "list"},
			{Label: "Back", Value: "back"},
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Tunnels",
			Options: options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		switch choice {
		case actions.ActionTunnelAdd:
			if err := RunAction(actions.ActionTunnelAdd); err != nil && err != errCancelled {
				tui.PrintError(err.Error())
			}
			tui.WaitForEnter()
		case "list":
			if err := runTunnelListMenu(); err != errCancelled {
				tui.WaitForEnter()
			}
		}
	}
}

// runTunnelListMenu shows all tunnels and allows selecting one to manage.
func runTunnelListMenu() error {
	for {
		cfg, err := config.Load()
		if err != nil {
			tui.PrintError("Failed to load config: " + err.Error())
			return nil
		}

		if len(cfg.Tunnels) == 0 {
			tui.PrintInfo("No tunnels configured. Add one first.")
			tui.WaitForEnter()
			return errCancelled
		}

		var options []tui.MenuOption
		for _, t := range cfg.Tunnels {
			tunnel := router.NewTunnel(&t)
			status := "○"
			if tunnel.IsActive() {
				status = "●"
			}
			transportName := config.GetTransportTypeDisplayName(t.Transport)
			label := fmt.Sprintf("%s %s (%s → %s)", status, t.Tag, transportName, t.Backend)
			options = append(options, tui.MenuOption{Label: label, Value: t.Tag})
		}
		options = append(options, tui.MenuOption{Label: "Back", Value: "back"})

		selected, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Select Tunnel",
			Options: options,
		})
		if err != nil || selected == "" || selected == "back" {
			return errCancelled
		}

		if err := runTunnelManageMenu(selected); err != errCancelled {
			tui.WaitForEnter()
		}
	}
}

// runTunnelManageMenu shows management options for a specific tunnel.
func runTunnelManageMenu(tag string) error {
	for {
		cfg, err := config.Load()
		if err != nil {
			tui.PrintError("Failed to load config: " + err.Error())
			return nil
		}

		tunnelCfg := cfg.GetTunnelByTag(tag)
		if tunnelCfg == nil {
			tui.PrintError(fmt.Sprintf("Tunnel '%s' not found", tag))
			return nil
		}

		tunnel := router.NewTunnel(tunnelCfg)
		status := "Stopped"
		if tunnel.IsActive() {
			status = "Running"
		}

		options := []tui.MenuOption{
			{Label: "Status", Value: "status"},
			{Label: "Logs", Value: "logs"},
			{Label: "Start/Restart", Value: "start"},
			{Label: "Stop", Value: "stop"},
			{Label: "Enable", Value: "enable"},
			{Label: "Disable", Value: "disable"},
			{Label: "Reconfigure", Value: "reconfigure"},
			{Label: "Remove", Value: "remove"},
			{Label: "Back", Value: "back"},
		}

		transportName := config.GetTransportTypeDisplayName(tunnelCfg.Transport)
		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:       fmt.Sprintf("%s (%s)", tag, status),
			Description: fmt.Sprintf("%s → %s:%d", transportName, tunnelCfg.Domain, tunnelCfg.Port),
			Options:     options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		// Execute the action with the tunnel tag as argument
		actionID := "tunnel." + choice
		if err := runTunnelAction(actionID, tag); err != nil {
			if err == errCancelled {
				continue
			}
			tui.PrintError(err.Error())
			tui.WaitForEnter()
		} else {
			// Check if tunnel was removed
			if choice == "remove" {
				return errCancelled
			}
			// Check if tunnel was renamed
			if choice == "reconfigure" {
				cfg, err = config.Load()
				if err != nil || cfg == nil {
					// Config unavailable, go back to list
					return errCancelled
				}
				if cfg.GetTunnelByTag(tag) == nil {
					// Tunnel was renamed, go back to list
					return errCancelled
				}
			}
			tui.WaitForEnter()
		}
	}
}

// runTunnelAction runs a tunnel action with the given tag as argument.
func runTunnelAction(actionID, tunnelTag string) error {
	// Special handling for actions that need the tunnel tag
	switch actionID {
	case actions.ActionTunnelStatus, actions.ActionTunnelLogs, actions.ActionTunnelStart,
		actions.ActionTunnelStop, actions.ActionTunnelRestart, actions.ActionTunnelEnable,
		actions.ActionTunnelDisable, actions.ActionTunnelRemove, actions.ActionTunnelReconfigure:
		return runActionWithArgs(actionID, []string{tunnelTag})
	default:
		return RunAction(actionID)
	}
}

// runActionWithArgs runs an action with predefined arguments.
func runActionWithArgs(actionID string, args []string) error {
	action := actions.Get(actionID)
	if action == nil {
		return fmt.Errorf("unknown action: %s", actionID)
	}

	// Build context with args
	ctx := newActionContext(args)

	// Handle confirmation for remove action
	if actionID == actions.ActionTunnelRemove && action.Confirm != nil {
		tag := args[0]
		confirm, err := tui.RunConfirm(tui.ConfirmConfig{
			Title:       fmt.Sprintf("Remove '%s'?", tag),
			Description: "This will stop the service and remove all configuration",
		})
		if err != nil {
			return err
		}
		if !confirm {
			tui.PrintInfo("Cancelled")
			return errCancelled
		}
	}

	if action.Handler == nil {
		return fmt.Errorf("no handler for action %s", actionID)
	}

	return action.Handler(ctx)
}
