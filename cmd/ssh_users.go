package cmd

import (
	"os"
	"syscall"

	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var sshUsersCmd = &cobra.Command{
	Use:   "ssh-users",
	Short: "Manage SSH tunnel users",
	Long:  "Launch sshtun-user for managing SSH tunnel users and hardening",
	RunE:  runSSHUsers,
}

func runSSHUsers(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	if !transport.IsSSHTunUserInstalled() {
		tui.PrintError("sshtun-user is not installed. Run 'dnstm install' first.")
		return nil
	}

	// Get the binary path
	binary := transport.SSHTunUserBinary

	// Use syscall.Exec to replace the current process with sshtun-user
	// This allows sshtun-user to run in fully interactive mode
	if err := syscall.Exec(binary, []string{binary}, os.Environ()); err != nil {
		return err
	}

	// This line is never reached as Exec replaces the process
	return nil
}
