package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var modeCmd = &cobra.Command{
	Use:   "mode [single|multi]",
	Short: "Show or set operating mode",
	Long: `Show or set the operating mode of dnstm.

Modes:
  single  Single-tunnel mode with iptables NAT redirect
  multi   Multi-tunnel mode with DNS router

Without arguments, shows the current mode.`,
	Args: cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireInstalled()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// No args - show current mode
		if len(args) == 0 {
			return showCurrentMode(cfg)
		}

		// Switch mode
		newMode := router.Mode(args[0])
		if newMode != router.ModeSingle && newMode != router.ModeMulti {
			return fmt.Errorf("invalid mode '%s'. Use 'single' or 'multi'", args[0])
		}

		return switchMode(cfg, newMode)
	},
}

var modeSingleCmd = &cobra.Command{
	Use:   "single",
	Short: "Switch to single-tunnel mode",
	Long: `Switch to single-tunnel mode.

In single-tunnel mode:
  - One tunnel is active at a time
  - DNS traffic is redirected via iptables NAT
  - Lower overhead (no DNS router process)
  - Use 'dnstm switch <instance>' to change active tunnel`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		return switchMode(cfg, router.ModeSingle)
	},
}

var modeMultiCmd = &cobra.Command{
	Use:   "multi",
	Short: "Switch to multi-tunnel mode",
	Long: `Switch to multi-tunnel mode.

In multi-tunnel mode:
  - All tunnels run simultaneously
  - DNS router handles domain-based routing
  - Each domain is routed to its designated tunnel`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		return switchMode(cfg, router.ModeMulti)
	},
}

func showCurrentMode(cfg *router.Config) error {
	fmt.Println()

	modeName := router.GetModeDisplayName(cfg.Mode)
	var lines []string
	lines = append(lines, fmt.Sprintf("Mode: %s", modeName))

	if cfg.IsSingleMode() {
		if cfg.Single.Active != "" {
			lines = append(lines, fmt.Sprintf("Active instance: %s", cfg.Single.Active))
			if transport, ok := cfg.Transports[cfg.Single.Active]; ok {
				lines = append(lines, fmt.Sprintf("Domain: %s", transport.Domain))
			}
		} else {
			lines = append(lines, "Active instance: (none)")
		}
		lines = append(lines, "")
		lines = append(lines, "Use 'dnstm switch <instance>' to change active tunnel")
	} else {
		lines = append(lines, fmt.Sprintf("Instances: %d", len(cfg.Transports)))
		if cfg.Routing.Default != "" {
			lines = append(lines, fmt.Sprintf("Default route: %s", cfg.Routing.Default))
		}
	}

	tui.PrintBox("Operating Mode", lines)
	fmt.Println()

	return nil
}

func switchMode(cfg *router.Config, newMode router.Mode) error {
	if cfg.Mode == newMode {
		modeName := router.GetModeDisplayName(newMode)
		tui.PrintInfo(fmt.Sprintf("Already in %s mode", modeName))
		return nil
	}

	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	fmt.Println()

	newModeName := router.GetModeDisplayName(newMode)
	oldModeName := router.GetModeDisplayName(cfg.Mode)
	tui.PrintInfo(fmt.Sprintf("Switching from %s to %s mode...", oldModeName, newModeName))
	fmt.Println()

	if err := r.SwitchMode(newMode); err != nil {
		return fmt.Errorf("failed to switch mode: %w", err)
	}

	fmt.Println()
	tui.PrintSuccess(fmt.Sprintf("Switched to %s mode!", newModeName))
	fmt.Println()

	// Show next steps
	if newMode == router.ModeSingle {
		tui.PrintInfo("Commands for single-tunnel mode:")
		fmt.Println("  dnstm switch <instance>  - Switch active tunnel")
		fmt.Println("  dnstm start              - Start active tunnel")
		fmt.Println("  dnstm stop               - Stop active tunnel")
	} else {
		tui.PrintInfo("Commands for multi-tunnel mode:")
		fmt.Println("  dnstm router status      - Show router and instance status")
		fmt.Println("  dnstm router start       - Start router and all instances")
		fmt.Println("  dnstm router stop        - Stop router and all instances")
	}
	fmt.Println()

	return nil
}

func init() {
	modeCmd.AddCommand(modeSingleCmd)
	modeCmd.AddCommand(modeMultiCmd)
}
