package cmd

import (
	"fmt"
	"strings"

	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/types"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var switchCmd = &cobra.Command{
	Use:   "switch [instance]",
	Short: "Switch active tunnel instance",
	Long: `Switch the active tunnel instance in single-tunnel mode.

Without arguments, shows an interactive picker.
With an instance name, switches to that instance directly.

This command is only available in single-tunnel mode.
Use 'dnstm mode single' to switch to single-tunnel mode first.`,
	Args: cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireInstalled()
	},
	RunE: runSwitch,
}

func runSwitch(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	if !router.IsInitialized() {
		return fmt.Errorf("router not initialized. Run 'dnstm router init' first")
	}

	cfg, err := router.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check mode
	if !cfg.IsSingleMode() {
		return fmt.Errorf("switch is only available in single-tunnel mode\nUse 'dnstm mode single' to switch modes first")
	}

	// Check if there are instances to switch to
	if len(cfg.Transports) == 0 {
		return fmt.Errorf("no instances configured\nUse 'dnstm instance add' to create one")
	}

	if len(cfg.Transports) == 1 {
		// Only one instance - just make sure it's active
		for name := range cfg.Transports {
			if cfg.Single.Active == name {
				tui.PrintInfo(fmt.Sprintf("'%s' is already the active instance", name))
				return nil
			}
			args = []string{name}
			break
		}
	}

	var instanceName string
	if len(args) > 0 {
		instanceName = args[0]
	} else {
		// Interactive mode - show picker
		instanceName, err = selectInstance(cfg)
		if err != nil {
			return err
		}
		if instanceName == "" {
			tui.PrintInfo("Cancelled")
			return nil
		}
	}

	// Verify instance exists
	if _, ok := cfg.Transports[instanceName]; !ok {
		return fmt.Errorf("instance '%s' not found\nUse 'dnstm instance list' to see available instances", instanceName)
	}

	// Check if already active
	if cfg.Single.Active == instanceName {
		tui.PrintInfo(fmt.Sprintf("'%s' is already the active instance", instanceName))
		return nil
	}

	// Create router and switch
	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	fmt.Println()
	tui.PrintInfo(fmt.Sprintf("Switching to '%s'...", instanceName))
	fmt.Println()

	if err := r.SwitchActiveInstance(instanceName); err != nil {
		return fmt.Errorf("failed to switch instance: %w", err)
	}

	// Show success
	transport := cfg.Transports[instanceName]
	typeName := types.GetTransportTypeDisplayName(transport.Type)

	fmt.Println()
	tui.PrintSuccess(fmt.Sprintf("Switched to '%s'!", instanceName))
	fmt.Println()

	var lines []string
	lines = append(lines, tui.KV("Instance: ", instanceName))
	lines = append(lines, tui.KV("Type:     ", typeName))
	lines = append(lines, tui.KV("Domain:   ", transport.Domain))
	lines = append(lines, tui.KV("Port:     ", fmt.Sprintf("%d", transport.Port)))
	tui.PrintBox("Active Tunnel", lines)
	fmt.Println()

	return nil
}

func selectInstance(cfg *router.Config) (string, error) {
	// Build options
	var options []tui.MenuOption
	for name, transport := range cfg.Transports {
		typeName := types.GetTransportTypeDisplayName(transport.Type)
		label := fmt.Sprintf("%-16s %s", name, typeName)
		if name == cfg.Single.Active {
			label += " (current)"
		}
		options = append(options, tui.MenuOption{Label: label, Value: name})
	}

	selected, err := tui.RunMenu(tui.MenuConfig{
		Title:       "Select Instance",
		Description: "Choose which tunnel to activate",
		Options:     options,
	})
	if err != nil {
		return "", err
	}

	return selected, nil
}

var switchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available instances",
	Long:  "List all available instances that can be switched to",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if len(cfg.Transports) == 0 {
			fmt.Println("No instances configured")
			return nil
		}

		fmt.Println()
		fmt.Printf("%-16s %-24s %-8s %-20s %s\n", "NAME", "TYPE", "PORT", "DOMAIN", "STATUS")
		fmt.Println(strings.Repeat("-", 80))

		for name, transport := range cfg.Transports {
			status := ""
			if cfg.IsSingleMode() && cfg.Single.Active == name {
				status = "Active"
			} else if cfg.IsMultiMode() && cfg.Routing.Default == name {
				status = "Default"
			}
			typeName := types.GetTransportTypeDisplayName(transport.Type)
			fmt.Printf("%-16s %-24s %-8d %-20s %s\n", name, typeName, transport.Port, transport.Domain, status)
		}
		fmt.Println()

		return nil
	},
}

func init() {
	switchCmd.AddCommand(switchListCmd)
}
