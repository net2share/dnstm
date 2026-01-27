package installer

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/download"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/sshtunnel"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/go-corelib/tui"
)

// UninstallResult indicates what happened during uninstall.
type UninstallResult int

const (
	UninstallCancelled UninstallResult = iota
	UninstallCompleted
)

// RunUninstallInteractive runs the interactive uninstall process.
func RunUninstallInteractive() (UninstallResult, error) {
	fmt.Println()
	tui.PrintWarning("This will completely remove dnstt from your system:")
	fmt.Println("  - Stop and remove the dnstt-server service")
	fmt.Println("  - Remove the dnstt-server binary")
	fmt.Println("  - Remove all configuration files and keys")
	fmt.Println("  - Remove firewall rules")
	fmt.Println("  - Remove the dnstt system user")
	fmt.Println()

	var confirm bool
	err := huh.NewConfirm().
		Title("Are you sure you want to uninstall?").
		Value(&confirm).
		Run()
	if err != nil {
		return UninstallCancelled, err
	}

	if !confirm {
		tui.PrintInfo("Uninstall cancelled")
		return UninstallCancelled, nil
	}

	// Ask about SSH tunnel users
	removeSSHUsers := false
	if sshtunnel.IsConfigured() {
		fmt.Println()
		tui.PrintInfo("SSH tunnel hardening is configured on this system.")
		err = huh.NewConfirm().
			Title("Also remove SSH tunnel users and sshd hardening config?").
			Value(&removeSSHUsers).
			Run()
		if err != nil {
			return UninstallCancelled, err
		}
	}

	performUninstall(removeSSHUsers)
	return UninstallCompleted, nil
}

// RunUninstallCLI runs the CLI uninstall with provided options.
func RunUninstallCLI(removeSSHUsers bool) error {
	performUninstall(removeSSHUsers)
	return nil
}

func performUninstall(removeSSHUsers bool) {
	fmt.Println()
	totalSteps := 5
	if removeSSHUsers {
		totalSteps = 6
	}
	currentStep := 0

	// Step 1: Stop and remove service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Stopping and removing service...")
	if service.IsActive() {
		service.Stop()
	}
	if service.IsEnabled() {
		service.Disable()
	}
	service.Remove()
	tui.PrintStatus("Service removed")

	// Step 2: Remove binary
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing dnstt-server binary...")
	download.RemoveBinary()
	tui.PrintStatus("Binary removed")

	// Step 3: Remove configuration and keys
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing configuration and keys...")
	config.RemoveAll()
	tui.PrintStatus("Configuration removed")

	// Step 4: Remove firewall rules
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing firewall rules...")
	network.RemoveFirewallRules()
	tui.PrintStatus("Firewall rules removed")

	// Step 5: Clean up dnstm user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Cleaning up dnstm user...")
	system.RemoveDnstmUser()
	tui.PrintStatus("User removed")

	// Step 6: Remove SSH tunnel users and config (if requested)
	if removeSSHUsers {
		currentStep++
		tui.PrintStep(currentStep, totalSteps, "Removing SSH tunnel users and config...")
		if err := sshtunnel.UninstallAll(); err != nil {
			tui.PrintWarning("SSH tunnel uninstall warning: " + err.Error())
		} else {
			tui.PrintStatus("SSH tunnel config removed")
		}
	}

	fmt.Println()
	tui.PrintSuccess("Uninstallation complete!")
	tui.PrintInfo("All dnstt components have been removed from your system.")
}
