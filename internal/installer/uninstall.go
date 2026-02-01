package installer

import (
	"os"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/system"
)

// PerformFullUninstall removes all dnstm components from the system.
func PerformFullUninstall(output actions.OutputWriter, isInteractive bool) error {
	// Start progress view in interactive mode
	if isInteractive {
		output.BeginProgress("Uninstall")
	} else {
		output.Println()
	}

	output.Info("Performing full uninstall...")

	totalSteps := 7
	currentStep := 0

	// Step 1: Remove all tunnels (stops, disables, removes services)
	currentStep++
	output.Step(currentStep, totalSteps, "Removing all tunnels...")
	cfg, _ := router.Load()
	if cfg != nil {
		for _, t := range cfg.Tunnels {
			tunnel := router.NewTunnel(&t)
			tunnel.RemoveService() // Stops, disables, and removes systemd service
		}
	}
	output.Status("Tunnels removed")

	// Step 2: Remove DNS router service
	currentStep++
	output.Step(currentStep, totalSteps, "Removing DNS router service...")
	svc := dnsrouter.NewService()
	svc.Stop()
	svc.Remove()
	output.Status("DNS router service removed")

	// Step 3: Remove microsocks service
	currentStep++
	output.Step(currentStep, totalSteps, "Removing microsocks...")
	proxy.StopMicrosocks()
	proxy.UninstallMicrosocks()
	output.Status("Microsocks removed")

	// Step 4: Remove /etc/dnstm entirely
	currentStep++
	output.Step(currentStep, totalSteps, "Removing configuration directory...")
	os.RemoveAll("/etc/dnstm")
	output.Status("Configuration removed")

	// Step 5: Remove dnstm user
	currentStep++
	output.Step(currentStep, totalSteps, "Removing dnstm user...")
	system.RemoveDnstmUser()
	output.Status("User removed")

	// Step 6: Remove transport binaries
	currentStep++
	output.Step(currentStep, totalSteps, "Removing transport binaries...")
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
	output.Status("Binaries removed")

	// Step 7: Remove firewall rules
	currentStep++
	output.Step(currentStep, totalSteps, "Removing firewall rules...")
	network.ClearNATOnly()
	network.RemoveAllFirewallRules()
	output.Status("Firewall rules removed")

	output.Success("Uninstallation complete!")
	output.Info("All dnstm components have been removed.")
	output.Info("Note: The dnstm binary is still available for reinstallation.")
	output.Info("      To fully remove: rm /usr/local/bin/dnstm")

	if isInteractive {
		output.EndProgress()
	} else {
		output.Println()
	}

	return nil
}
