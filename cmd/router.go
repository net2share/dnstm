package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var routerCmd = &cobra.Command{
	Use:   "router",
	Short: "Manage DNS tunnel router",
	Long:  "Manage the multi-transport DNS router",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Allow init command without installation check
		if cmd.Name() == "init" {
			return nil
		}
		return requireInstalled()
	},
}

var routerInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the router",
	Long: `Initialize the router by creating default configuration.

Use --mode to set the initial operating mode:
  single  Single-tunnel mode (default) - one tunnel at a time with NAT redirect
  multi   Multi-tunnel mode - multiple tunnels with DNS router`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		modeStr, _ := cmd.Flags().GetString("mode")

		fmt.Println()
		tui.PrintInfo("Initializing DNS tunnel router...")
		fmt.Println()

		totalSteps := 3
		currentStep := 0

		// Step 1: Create dnstm user (needed before directory ownership)
		currentStep++
		tui.PrintStep(currentStep, totalSteps, "Creating dnstm user...")
		if err := system.CreateDnstmUser(); err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
		tui.PrintStatus("User 'dnstm' created")

		// Step 2: Create directories and default config
		currentStep++
		tui.PrintStep(currentStep, totalSteps, "Creating configuration directories...")
		if err := router.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize: %w", err)
		}
		tui.PrintStatus("Configuration directories created")

		// Step 3: Set mode and create services
		currentStep++
		if modeStr != "" {
			mode := router.Mode(modeStr)
			if mode != router.ModeSingle && mode != router.ModeMulti {
				return fmt.Errorf("invalid mode '%s'. Use 'single' or 'multi'", modeStr)
			}

			cfg, err := router.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			cfg.Mode = mode
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			tui.PrintStep(currentStep, totalSteps, fmt.Sprintf("Mode set to %s...", router.GetModeDisplayName(mode)))
		} else {
			tui.PrintStep(currentStep, totalSteps, "Using default single-tunnel mode...")
		}

		// Create DNS router service (needed for multi mode, optional for single)
		svc := dnsrouter.NewService()
		if err := svc.CreateService(); err != nil {
			tui.PrintWarning("DNS router service: " + err.Error())
		} else {
			tui.PrintStatus("DNS router service created")
		}

		fmt.Println()
		tui.PrintSuccess("Router initialized successfully!")
		fmt.Println()
		tui.PrintInfo("Next steps:")
		fmt.Println("  1. Add transport instances: dnstm instance add")
		fmt.Println("  2. Start the router: dnstm router start")
		fmt.Println()
		tui.PrintInfo("To change mode later: dnstm mode [single|multi]")
		fmt.Println()

		return nil
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
	Long: `Start tunnels based on current mode.

In single-tunnel mode: starts the active instance.
In multi-tunnel mode: starts DNS router and all instances.`,
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
		modeName := router.GetModeDisplayName(cfg.Mode)
		tui.PrintInfo(fmt.Sprintf("Starting in %s mode...", modeName))
		fmt.Println()

		if err := r.Start(); err != nil {
			return fmt.Errorf("failed to start: %w", err)
		}

		fmt.Println()
		tui.PrintSuccess("Started!")
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

var routerRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the router",
	Long:  "Restart tunnels based on current mode",
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
		tui.PrintInfo("Restarting...")
		fmt.Println()

		if err := r.Restart(); err != nil {
			return fmt.Errorf("failed to restart: %w", err)
		}

		fmt.Println()
		tui.PrintSuccess("Restarted!")
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

var routerConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show router configuration",
	Long:  "Show the current router configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		if !router.IsInitialized() {
			return fmt.Errorf("router not initialized")
		}

		// Read and display the config file
		configPath := router.GetConfigPath()
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read config: %w", err)
		}

		fmt.Println("# Configuration file:", configPath)
		fmt.Println(strings.TrimSpace(string(data)))

		return nil
	},
}

var routerResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset router to initial state",
	Long: `Reset the router by removing all instances and services.

This will:
  - Stop all services (instances + DNS router)
  - Remove all instance service files
  - Remove DNS router service file
  - Delete instance configurations
  - Reset config.yaml to default state
  - Remove firewall rules`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		if !router.IsInitialized() {
			return fmt.Errorf("router not initialized")
		}

		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Println()
			tui.PrintWarning("This will reset the router to its initial state:")
			fmt.Println("  - Stop and remove all instance services")
			fmt.Println("  - Stop and remove DNS router service")
			fmt.Println("  - Delete all instance configurations")
			fmt.Println("  - Reset config.yaml to defaults")
			fmt.Println("  - Remove firewall rules")
			fmt.Println()

			var confirm bool
			err := huh.NewConfirm().
				Title("Are you sure you want to reset?").
				Value(&confirm).
				Run()
			if err != nil {
				return err
			}
			if !confirm {
				tui.PrintInfo("Cancelled")
				return nil
			}
		}

		return performRouterReset()
	},
}

func performRouterReset() error {
	fmt.Println()
	tui.PrintInfo("Resetting router...")
	fmt.Println()

	totalSteps := 6
	currentStep := 0

	// Step 1: Load config and stop all services
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Stopping all services...")

	cfg, err := router.Load()
	if err != nil {
		tui.PrintWarning("Could not load config: " + err.Error())
	} else {
		// Stop all instances
		for name, transport := range cfg.Transports {
			instance := router.NewInstance(name, transport)
			if instance.IsActive() {
				instance.Stop()
			}
		}
	}

	// Stop DNS router
	svc := dnsrouter.NewService()
	if svc.IsActive() {
		svc.Stop()
	}
	tui.PrintStatus("Services stopped")

	// Step 2: Remove instance services
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing instance services...")
	if cfg != nil {
		for name, transport := range cfg.Transports {
			instance := router.NewInstance(name, transport)
			if err := instance.RemoveService(); err != nil {
				tui.PrintWarning(fmt.Sprintf("Failed to remove %s service: %v", name, err))
			}
		}
	}
	tui.PrintStatus("Instance services removed")

	// Step 3: Remove DNS router service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing DNS router service...")
	if err := svc.Remove(); err != nil {
		tui.PrintWarning("Failed to remove DNS router service: " + err.Error())
	}
	tui.PrintStatus("DNS router service removed")

	// Step 4: Delete instance config directories
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing instance configurations...")
	instancesDir := "/etc/dnstm/instances"
	if err := os.RemoveAll(instancesDir); err != nil {
		tui.PrintWarning("Failed to remove instances directory: " + err.Error())
	}
	// Recreate empty directory with proper ownership
	os.MkdirAll(instancesDir, 0750)
	system.ChownDirToDnstm(instancesDir)

	// Remove dnsrouter.yaml
	os.Remove("/etc/dnstm/dnsrouter.yaml")
	tui.PrintStatus("Instance configurations removed")

	// Step 5: Reset config.yaml
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Resetting configuration...")
	defaultCfg := router.Default()
	if err := defaultCfg.Save(); err != nil {
		return fmt.Errorf("failed to save default config: %w", err)
	}
	tui.PrintStatus("Configuration reset")

	// Step 6: Remove firewall rules
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing firewall rules...")
	// Import network package via router - use ClearNATOnly which handles cleanup
	// We need to call this directly
	clearFirewallRules()
	tui.PrintStatus("Firewall rules removed")

	fmt.Println()
	tui.PrintSuccess("Router reset complete!")
	fmt.Println()
	tui.PrintInfo("To start fresh: dnstm instance add")
	fmt.Println()

	return nil
}

func clearFirewallRules() {
	// Clear NAT rules and remove port permissions
	network.ClearNATOnly()
	network.RemoveAllFirewallRules()
}

func init() {
	routerCmd.AddCommand(routerInitCmd)
	routerCmd.AddCommand(routerStatusCmd)
	routerCmd.AddCommand(routerStartCmd)
	routerCmd.AddCommand(routerStopCmd)
	routerCmd.AddCommand(routerRestartCmd)
	routerCmd.AddCommand(routerLogsCmd)
	routerCmd.AddCommand(routerConfigCmd)
	routerCmd.AddCommand(routerResetCmd)

	routerInitCmd.Flags().StringP("mode", "m", "", "Operating mode: single or multi (default: single)")
	routerLogsCmd.Flags().IntP("lines", "n", 50, "Number of log lines to show")
	routerResetCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
}
