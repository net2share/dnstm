package cmd

import (
	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/sshtunnel"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/spf13/cobra"
)

var sshUsersCmd = &cobra.Command{
	Use:   "ssh-users",
	Short: "Manage SSH tunnel users",
	RunE:  runSSHUsers,
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
