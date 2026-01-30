// Package menu provides the interactive menu for dnstm.
package menu

import (
	"errors"
	"fmt"
	"os"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/dnstm/internal/types"
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

// buildInstanceSummary builds a summary string for the main menu header.
func buildInstanceSummary() string {
	cfg, err := router.Load()
	if err != nil || cfg == nil {
		return ""
	}

	total := len(cfg.Transports)
	running := 0
	for name, transport := range cfg.Transports {
		instance := router.NewInstance(name, transport)
		if instance.IsActive() {
			running++
		}
	}

	if cfg.IsSingleMode() && cfg.Single.Active != "" {
		return fmt.Sprintf("Instances: %d | Running: %d | Active: %s", total, running, cfg.Single.Active)
	}
	return fmt.Sprintf("Instances: %d | Running: %d", total, running)
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
			// Build instance summary for header
			header = buildInstanceSummary()

			// Fully installed - show all options
			options = append(options, tui.MenuOption{Label: "Instances →", Value: actions.ActionInstance})
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
	case actions.ActionInstance:
		return runInstanceMenu()
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

// runInstanceMenu shows the instance submenu with special handling for list navigation.
func runInstanceMenu() error {
	for {
		fmt.Println()

		options := []tui.MenuOption{
			{Label: "Add", Value: actions.ActionInstanceAdd},
			{Label: "List →", Value: "list"},
			{Label: "Back", Value: "back"},
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Instances",
			Options: options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		switch choice {
		case actions.ActionInstanceAdd:
			if err := RunAction(actions.ActionInstanceAdd); err != nil && err != errCancelled {
				tui.PrintError(err.Error())
			}
			tui.WaitForEnter()
		case "list":
			if err := runInstanceListMenu(); err != errCancelled {
				tui.WaitForEnter()
			}
		}
	}
}

// runInstanceListMenu shows all instances and allows selecting one to manage.
func runInstanceListMenu() error {
	for {
		cfg, err := router.Load()
		if err != nil {
			tui.PrintError("Failed to load config: " + err.Error())
			return nil
		}

		if len(cfg.Transports) == 0 {
			tui.PrintInfo("No instances configured. Add one first.")
			tui.WaitForEnter()
			return errCancelled
		}

		var options []tui.MenuOption
		for name, transport := range cfg.Transports {
			instance := router.NewInstance(name, transport)
			status := "○"
			if instance.IsActive() {
				status = "●"
			}
			typeName := types.GetTransportTypeDisplayName(transport.Type)
			label := fmt.Sprintf("%s %s (%s)", status, name, typeName)
			options = append(options, tui.MenuOption{Label: label, Value: name})
		}
		options = append(options, tui.MenuOption{Label: "Back", Value: "back"})

		selected, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Select Instance",
			Options: options,
		})
		if err != nil || selected == "" || selected == "back" {
			return errCancelled
		}

		if err := runInstanceManageMenu(selected); err != errCancelled {
			tui.WaitForEnter()
		}
	}
}

// runInstanceManageMenu shows management options for a specific instance.
func runInstanceManageMenu(name string) error {
	for {
		cfg, err := router.Load()
		if err != nil {
			tui.PrintError("Failed to load config: " + err.Error())
			return nil
		}

		transport, exists := cfg.Transports[name]
		if !exists {
			tui.PrintError(fmt.Sprintf("Instance '%s' not found", name))
			return nil
		}

		instance := router.NewInstance(name, transport)
		status := "Stopped"
		if instance.IsActive() {
			status = "Running"
		}

		options := []tui.MenuOption{
			{Label: "Status", Value: "status"},
			{Label: "Logs", Value: "logs"},
			{Label: "Start/Restart", Value: "start"},
			{Label: "Stop", Value: "stop"},
			{Label: "Reconfigure", Value: "reconfigure"},
			{Label: "Remove", Value: "remove"},
			{Label: "Back", Value: "back"},
		}

		typeName := types.GetTransportTypeDisplayName(transport.Type)
		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:       fmt.Sprintf("%s (%s)", name, status),
			Description: fmt.Sprintf("%s → %s:%d", typeName, transport.Domain, transport.Port),
			Options:     options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		// Execute the action with the instance name as argument
		actionID := "instance." + choice
		if err := runInstanceAction(actionID, name); err != nil {
			if err == errCancelled {
				continue
			}
			tui.PrintError(err.Error())
			tui.WaitForEnter()
		} else {
			// Check if instance was removed
			if choice == "remove" {
				return errCancelled
			}
			// Check if instance was renamed
			if choice == "reconfigure" {
				cfg, err = router.Load()
				if err != nil || cfg == nil {
					// Config unavailable, go back to list
					return errCancelled
				}
				if _, exists := cfg.Transports[name]; !exists {
					// Instance was renamed, go back to list
					return errCancelled
				}
			}
			tui.WaitForEnter()
		}
	}
}

// runInstanceAction runs an instance action with the given name as argument.
func runInstanceAction(actionID, instanceName string) error {
	// Special handling for actions that need the instance name
	switch actionID {
	case actions.ActionInstanceStatus, actions.ActionInstanceLogs, actions.ActionInstanceStart,
		actions.ActionInstanceStop, actions.ActionInstanceRemove, actions.ActionInstanceReconfigure:
		return runActionWithArgs(actionID, []string{instanceName})
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
	if actionID == actions.ActionInstanceRemove && action.Confirm != nil {
		name := args[0]
		confirm, err := tui.RunConfirm(tui.ConfirmConfig{
			Title:       fmt.Sprintf("Remove '%s'?", name),
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
