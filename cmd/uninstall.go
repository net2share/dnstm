package cmd

import (
	"github.com/net2share/dnstm/internal/installer"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/spf13/cobra"
)

var (
	uninstallRemoveSSH bool
	uninstallKeepSSH   bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall dnstt server",
	RunE:  runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallRemoveSSH, "remove-ssh-users", false, "Also remove SSH tunnel users and sshd config")
	uninstallCmd.Flags().BoolVar(&uninstallKeepSSH, "keep-ssh-users", false, "Keep SSH tunnel users and sshd config")
	uninstallCmd.MarkFlagsMutuallyExclusive("remove-ssh-users", "keep-ssh-users")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	installer.PrintBanner(Version, BuildTime)

	// Check if CLI mode (flags provided)
	if cmd.Flags().Changed("remove-ssh-users") || cmd.Flags().Changed("keep-ssh-users") {
		return installer.RunUninstallCLI(uninstallRemoveSSH)
	}

	// Interactive mode
	_, err := installer.RunUninstallInteractive()
	return err
}
