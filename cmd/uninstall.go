package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Completely uninstall dnstm",
	Long: `Remove all dnstm components from the system.

This will:
  - Stop and remove all instance services
  - Stop and remove DNS router service
  - Stop and remove microsocks service
  - Remove all configuration in /etc/dnstm
  - Remove dnstm user
  - Remove transport binaries (dnstt-server, slipstream-server, ssserver, microsocks)
  - Remove firewall rules

Note: The dnstm binary itself is kept for easy reinstallation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Println()
			tui.PrintWarning("This will remove all dnstm components from your system:")
			fmt.Println("  - All instance services and configurations")
			fmt.Println("  - DNS router and microsocks services")
			fmt.Println("  - Transport binaries (dnstt-server, slipstream-server, ssserver, microsocks)")
			fmt.Println("  - The dnstm user")
			fmt.Println("  - Firewall rules")
			fmt.Println()
			tui.PrintInfo("Note: The dnstm binary will be kept for easy reinstallation.")
			fmt.Println()

			var confirm bool
			err := huh.NewConfirm().
				Title("Are you sure you want to uninstall everything?").
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

		return PerformFullUninstall()
	},
}

// PerformFullUninstall removes all dnstm components from the system.
// It can be called from both CLI and interactive menu.
func PerformFullUninstall() error {
	fmt.Println()
	tui.PrintInfo("Performing full uninstall...")
	fmt.Println()

	totalSteps := 8
	currentStep := 0

	// Step 1: Stop all services (including microsocks)
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Stopping all services...")
	cfg, _ := router.Load()
	if cfg != nil {
		for name, transport := range cfg.Transports {
			instance := router.NewInstance(name, transport)
			if instance.IsActive() {
				instance.Stop()
			}
		}
	}
	svc := dnsrouter.NewService()
	if svc.IsActive() {
		svc.Stop()
	}
	// Stop microsocks service if running
	if proxy.IsMicrosocksRunning() {
		proxy.StopMicrosocks()
	}
	tui.PrintStatus("Services stopped")

	// Step 2: Remove instance services
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing instance services...")
	if cfg != nil {
		for name, transport := range cfg.Transports {
			instance := router.NewInstance(name, transport)
			instance.RemoveService()
		}
	}
	tui.PrintStatus("Instance services removed")

	// Step 3: Remove DNS router service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing DNS router service...")
	svc.Remove()
	tui.PrintStatus("DNS router service removed")

	// Step 4: Remove /etc/dnstm entirely
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing configuration directory...")
	os.RemoveAll("/etc/dnstm")
	tui.PrintStatus("Configuration removed")

	// Step 5: Remove dnstm user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing dnstm user...")
	system.RemoveDnstmUser()
	tui.PrintStatus("User removed")

	// Step 6: Remove microsocks service and binary
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing microsocks service...")
	proxy.UninstallMicrosocks()
	tui.PrintStatus("Microsocks removed")

	// Step 7: Remove transport binaries
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing transport binaries...")
	binaries := []string{
		"/usr/local/bin/dnstt-server",
		"/usr/local/bin/slipstream-server",
		"/usr/local/bin/ssserver",
	}
	for _, bin := range binaries {
		if _, err := os.Stat(bin); err == nil {
			os.Remove(bin)
		}
	}
	tui.PrintStatus("Binaries removed")

	// Step 8: Remove firewall rules
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing firewall rules...")
	network.ClearNATOnly()
	network.RemoveAllFirewallRules()
	tui.PrintStatus("Firewall rules removed")

	fmt.Println()
	tui.PrintSuccess("Uninstallation complete!")
	tui.PrintInfo("All dnstm components have been removed.")
	fmt.Println()
	tui.PrintInfo("Note: The dnstm binary is still available for reinstallation.")
	fmt.Println("      To fully remove: rm /usr/local/bin/dnstm")
	fmt.Println()

	return nil
}

func init() {
	uninstallCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
}
