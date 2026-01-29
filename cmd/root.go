// Package cmd provides the Cobra CLI for dnstm.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

// requireInstalled checks if transport binaries are installed.
func requireInstalled() error {
	if !transport.IsInstalled() {
		missing := transport.GetMissingBinaries()
		return fmt.Errorf("transport binaries not installed. Missing: %s\nRun 'dnstm install' first", strings.Join(missing, ", "))
	}
	return nil
}

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
		tui.SetAppInfo("dnstm", Version, BuildTime)
		return menu.RunInteractive()
	},
}

func init() {
	rootCmd.Version = Version

	// Main commands (order matches menu)
	rootCmd.AddCommand(instanceCmd)
	rootCmd.AddCommand(routerCmd)
	rootCmd.AddCommand(sshUsersCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(mtproxyCmd)

	// Utilities
	rootCmd.AddCommand(installCmd)
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
