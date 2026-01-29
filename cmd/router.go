package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var routerCmd = &cobra.Command{
	Use:   "router",
	Short: "Manage DNS tunnel router",
	Long:  "Manage the multi-transport DNS router",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return requireInstalled()
	},
}

var routerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show router status",
	Long:  "Show the status of the router, DNS router, and all transport instances",
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

		r, err := router.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to create router: %w", err)
		}

		fmt.Println()

		// Build status output
		var lines []string
		modeName := router.GetModeDisplayName(cfg.Mode)
		lines = append(lines, fmt.Sprintf("Mode: %s", modeName))

		if cfg.IsSingleMode() {
			// Single-tunnel mode status
			lines = append(lines, "")
			if cfg.Single.Active != "" {
				instance := r.GetInstance(cfg.Single.Active)
				if instance != nil {
					status := "○ Stopped"
					if instance.IsActive() {
						status = "● Running"
					}
					typeName := router.GetTransportTypeDisplayName(instance.Type)
					lines = append(lines, fmt.Sprintf("Active: %s (%s) %s", cfg.Single.Active, typeName, status))
					lines = append(lines, fmt.Sprintf("  └─ %s → 127.0.0.1:%d", instance.Domain, instance.Port))
				}
			} else {
				lines = append(lines, "Active: (none)")
			}

			// Show other instances
			if len(cfg.Transports) > 1 {
				lines = append(lines, "")
				lines = append(lines, "Other instances:")
				for name, transport := range cfg.Transports {
					if name == cfg.Single.Active {
						continue
					}
					typeName := router.GetTransportTypeDisplayName(transport.Type)
					lines = append(lines, fmt.Sprintf("  %-16s %s", name, typeName))
				}
			}
		} else {
			// Multi-tunnel mode status
			svc := r.GetDNSRouterService()
			routerStatus := "○ Stopped"
			if svc.IsActive() {
				routerStatus = "● Running"
			}
			if !svc.IsServiceInstalled() {
				routerStatus = "✗ Not installed"
			}
			lines = append(lines, fmt.Sprintf("DNS Router: %s (port 53)", routerStatus))
			lines = append(lines, "")
			lines = append(lines, "Instances:")

			instances := r.GetAllInstances()
			if len(instances) == 0 {
				lines = append(lines, "  No instances configured")
			} else {
				for name, instance := range instances {
					status := "○ Stopped"
					if instance.IsActive() {
						status = "● Running"
					}
					if !instance.IsInstalled() {
						status = "✗ Not installed"
					}

					typeName := router.GetTransportTypeDisplayName(instance.Type)
					defaultMarker := ""
					if cfg.Routing.Default == name {
						defaultMarker = " (default)"
					}
					lines = append(lines, fmt.Sprintf("  %-16s %-24s %s%s", name, typeName, status, defaultMarker))
					lines = append(lines, fmt.Sprintf("    └─ %s → 127.0.0.1:%d", instance.Domain, instance.Port))
				}
			}
		}

		tui.PrintBox("Router Status", lines)
		fmt.Println()

		return nil
	},
}

var routerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the router",
	Long: `Start or restart tunnels based on current mode.

If already running, restarts to pick up any configuration changes.

In single-tunnel mode: starts the active instance.
In multi-tunnel mode: starts DNS router and all instances.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		if !router.IsInitialized() {
			return fmt.Errorf("router not initialized. Run 'dnstm install' first")
		}

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		r, err := router.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to create router: %w", err)
		}

		fmt.Println()
		modeName := router.GetModeDisplayName(cfg.Mode)
		isRunning := r.IsRunning()

		if isRunning {
			tui.PrintInfo(fmt.Sprintf("Restarting in %s mode...", modeName))
		} else {
			tui.PrintInfo(fmt.Sprintf("Starting in %s mode...", modeName))
		}
		fmt.Println()

		if err := r.Restart(); err != nil {
			return fmt.Errorf("failed to start: %w", err)
		}

		fmt.Println()
		if isRunning {
			tui.PrintSuccess("Restarted!")
		} else {
			tui.PrintSuccess("Started!")
		}
		fmt.Println()

		return nil
	},
}

var routerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the router",
	Long: `Stop tunnels based on current mode.

In single-tunnel mode: stops the active instance and removes NAT rules.
In multi-tunnel mode: stops DNS router and all instances.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		if !router.IsInitialized() {
			return fmt.Errorf("router not initialized")
		}

		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		r, err := router.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to create router: %w", err)
		}

		fmt.Println()
		tui.PrintInfo("Stopping...")
		fmt.Println()

		if err := r.Stop(); err != nil {
			return fmt.Errorf("failed to stop: %w", err)
		}

		fmt.Println()
		tui.PrintSuccess("Stopped!")
		fmt.Println()

		return nil
	},
}

var routerLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show router logs",
	Long:  "Show recent logs from DNS router",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		lines, _ := cmd.Flags().GetInt("lines")

		svc := dnsrouter.NewService()
		logs, err := svc.GetLogs(lines)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}

		fmt.Println(logs)
		return nil
	},
}

func init() {
	routerCmd.AddCommand(routerStatusCmd)
	routerCmd.AddCommand(routerStartCmd)
	routerCmd.AddCommand(routerStopCmd)
	routerCmd.AddCommand(routerLogsCmd)
	routerCmd.AddCommand(modeCmd)
	routerCmd.AddCommand(switchCmd)

	routerLogsCmd.Flags().IntP("lines", "n", 50, "Number of log lines to show")
}
