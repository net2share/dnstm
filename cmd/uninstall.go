package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/sshtunnel"
	"github.com/net2share/dnstm/internal/tunnel"
	_ "github.com/net2share/dnstm/internal/tunnel/dnstt"
	_ "github.com/net2share/dnstm/internal/tunnel/slipstream"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var (
	uninstallRemoveSSH bool
	uninstallKeepSSH   bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall a DNS tunnel provider",
	Long:  "Uninstall a DNS tunnel provider (dnstt or slipstream)",
}

var uninstallDnsttCmd = &cobra.Command{
	Use:   "dnstt",
	Short: "Uninstall DNSTT server",
	Long:  "Completely remove the DNSTT DNS tunnel server",
	RunE:  runUninstallDnstt,
}

var uninstallSlipstreamCmd = &cobra.Command{
	Use:   "slipstream",
	Short: "Uninstall Slipstream server",
	Long:  "Completely remove the Slipstream DNS tunnel server",
	RunE:  runUninstallSlipstream,
}

func init() {
	// DNSTT uninstall flags
	uninstallDnsttCmd.Flags().BoolVar(&uninstallRemoveSSH, "remove-ssh-users", false, "Also remove SSH tunnel users and sshd config")
	uninstallDnsttCmd.Flags().BoolVar(&uninstallKeepSSH, "keep-ssh-users", false, "Keep SSH tunnel users and sshd config")
	uninstallDnsttCmd.MarkFlagsMutuallyExclusive("remove-ssh-users", "keep-ssh-users")

	// Slipstream uninstall flags (reset for this command)
	uninstallSlipstreamCmd.Flags().BoolVar(&uninstallRemoveSSH, "remove-ssh-users", false, "Also remove SSH tunnel users and sshd config")
	uninstallSlipstreamCmd.Flags().BoolVar(&uninstallKeepSSH, "keep-ssh-users", false, "Keep SSH tunnel users and sshd config")
	uninstallSlipstreamCmd.MarkFlagsMutuallyExclusive("remove-ssh-users", "keep-ssh-users")

	uninstallCmd.AddCommand(uninstallDnsttCmd)
	uninstallCmd.AddCommand(uninstallSlipstreamCmd)
}

func runUninstallDnstt(cmd *cobra.Command, args []string) error {
	return runUninstallProvider(cmd, tunnel.ProviderDNSTT)
}

func runUninstallSlipstream(cmd *cobra.Command, args []string) error {
	return runUninstallProvider(cmd, tunnel.ProviderSlipstream)
}

func runUninstallProvider(cmd *cobra.Command, pt tunnel.ProviderType) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	provider, err := tunnel.Get(pt)
	if err != nil {
		return err
	}

	if !provider.IsInstalled() {
		return fmt.Errorf("%s is not installed", provider.DisplayName())
	}

	// Check if CLI mode (flags provided)
	if cmd.Flags().Changed("remove-ssh-users") || cmd.Flags().Changed("keep-ssh-users") {
		return runUninstallCLI(provider)
	}

	// Interactive mode
	return runUninstallInteractive(provider)
}

func runUninstallInteractive(provider tunnel.Provider) error {
	fmt.Println()
	tui.PrintWarning(fmt.Sprintf("This will completely remove %s from your system:", provider.DisplayName()))
	fmt.Println("  - Stop and remove the service")
	fmt.Println("  - Remove the binary")
	fmt.Println("  - Remove all configuration files")
	fmt.Println("  - Remove firewall rules")
	fmt.Println("  - Remove the system user")
	fmt.Println()

	var confirm bool
	err := huh.NewConfirm().
		Title("Are you sure you want to uninstall?").
		Value(&confirm).
		Run()
	if err != nil {
		return err
	}

	if !confirm {
		tui.PrintInfo("Uninstall cancelled")
		return nil
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
			return err
		}
	}

	// Check if this is the active provider
	handleActiveProviderSwitch(provider)

	fmt.Println()
	if err := provider.Uninstall(removeSSHUsers); err != nil {
		return err
	}

	fmt.Println()
	tui.PrintSuccess("Uninstallation complete!")
	tui.PrintInfo(fmt.Sprintf("All %s components have been removed from your system.", provider.DisplayName()))

	return nil
}

func runUninstallCLI(provider tunnel.Provider) error {
	// Check if this is the active provider
	handleActiveProviderSwitch(provider)

	fmt.Println()
	if err := provider.Uninstall(uninstallRemoveSSH); err != nil {
		return err
	}

	fmt.Println()
	tui.PrintSuccess("Uninstallation complete!")
	tui.PrintInfo(fmt.Sprintf("All %s components have been removed from your system.", provider.DisplayName()))

	return nil
}

func handleActiveProviderSwitch(provider tunnel.Provider) {
	globalCfg, _ := tunnel.LoadGlobalConfig()
	if globalCfg != nil && globalCfg.ActiveProvider == provider.Name() {
		// Find another installed provider to switch to
		var otherProvider tunnel.Provider
		for _, pt := range tunnel.Types() {
			if pt == provider.Name() {
				continue
			}
			p, _ := tunnel.Get(pt)
			if p != nil && p.IsInstalled() {
				otherProvider = p
				break
			}
		}

		if otherProvider != nil {
			tui.PrintInfo(fmt.Sprintf("Switching active provider to %s...", otherProvider.DisplayName()))
			tunnel.SetActiveProvider(otherProvider.Name())
		}
	}
}
