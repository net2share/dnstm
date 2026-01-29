package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/installer"
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

			confirm, err := tui.RunConfirm(tui.ConfirmConfig{
				Title: "Are you sure you want to uninstall everything?",
			})
			if err != nil {
				return err
			}
			if !confirm {
				tui.PrintInfo("Cancelled")
				return nil
			}
		}

		return installer.PerformFullUninstall()
	},
}

func init() {
	uninstallCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
}
