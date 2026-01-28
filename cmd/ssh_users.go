package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/sshtunnel"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var sshUsersCmd = &cobra.Command{
	Use:   "ssh-users",
	Short: "Manage SSH tunnel users",
	RunE:  runSSHUsers,
}

var sshUsersUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall SSH tunnel hardening and users",
	Long:  "Remove all SSH tunnel users, groups, and sshd hardening configuration",
	RunE:  runSSHUsersUninstall,
}

func init() {
	sshUsersCmd.AddCommand(sshUsersUninstallCmd)
}

func runSSHUsers(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()
	sshtunnel.ShowMenu()
	return nil
}

func runSSHUsersUninstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	if !sshtunnel.IsConfigured() {
		tui.PrintInfo("SSH tunnel hardening is not configured")
		return nil
	}

	status := sshtunnel.GetStatus()

	fmt.Println()
	tui.PrintWarning("This will completely remove SSH tunnel configuration:")
	if status.UserCount > 0 {
		fmt.Printf("  - Delete %d tunnel user(s)\n", status.UserCount)
	}
	fmt.Println("  - Remove tunnel groups")
	fmt.Println("  - Remove sshd hardening configuration")
	fmt.Println("  - Clean up authorized keys and deny files")
	fmt.Println()

	var confirm bool
	err := huh.NewConfirm().
		Title("Proceed with complete uninstall?").
		Value(&confirm).
		Run()
	if err != nil {
		return err
	}

	if !confirm {
		tui.PrintInfo("Uninstall cancelled")
		return nil
	}

	fmt.Println()
	if err := sshtunnel.UninstallAll(); err != nil {
		return err
	}

	fmt.Println()
	tui.PrintSuccess("SSH tunnel configuration has been removed")
	return nil
}
