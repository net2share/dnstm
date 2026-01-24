// Package cmd provides the Cobra CLI for dnstm.
package cmd

import (
	"os"

	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/spf13/cobra"
)

// Version and BuildTime are set at build time.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "dnstm",
	Short: "DNS Tunnel Manager",
	Long:  "DNS Tunnel Manager - https://github.com/net2share/dnstm",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}
		menu.Version = Version
		menu.BuildTime = BuildTime
		return menu.RunInteractive()
	},
}

func init() {
	rootCmd.Version = Version

	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(sshUsersCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// SetVersionInfo sets version information for the CLI.
func SetVersionInfo(version, buildTime string) {
	Version = version
	BuildTime = buildTime
	rootCmd.Version = version + " (built " + buildTime + ")"
}
