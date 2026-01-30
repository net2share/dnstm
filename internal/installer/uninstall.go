package installer

import (
	"fmt"
	"os"

	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/go-corelib/tui"
)

// PerformFullUninstall removes all dnstm components from the system.
func PerformFullUninstall() error {
	fmt.Println()
	tui.PrintInfo("Performing full uninstall...")
	fmt.Println()

	totalSteps := 7
	currentStep := 0

	// Step 1: Remove all tunnels (stops, disables, removes services)
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing all tunnels...")
	cfg, _ := router.Load()
	if cfg != nil {
		for _, t := range cfg.Tunnels {
			tunnel := router.NewTunnel(&t)
			tunnel.RemoveService() // Stops, disables, and removes systemd service
		}
	}
	tui.PrintStatus("Tunnels removed")

	// Step 2: Remove DNS router service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing DNS router service...")
	svc := dnsrouter.NewService()
	svc.Stop()
	svc.Remove()
	tui.PrintStatus("DNS router service removed")

	// Step 3: Remove microsocks service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing microsocks...")
	proxy.StopMicrosocks()
	proxy.UninstallMicrosocks()
	tui.PrintStatus("Microsocks removed")

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

	// Step 6: Remove transport binaries
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing transport binaries...")
	binaries := []string{
		"/usr/local/bin/dnstt-server",
		"/usr/local/bin/slipstream-server",
		"/usr/local/bin/ssserver",
		"/usr/local/bin/sshtun-user",
	}
	for _, bin := range binaries {
		if _, err := os.Stat(bin); err == nil {
			os.Remove(bin)
		}
	}
	tui.PrintStatus("Binaries removed")

	// Step 7: Remove firewall rules
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
