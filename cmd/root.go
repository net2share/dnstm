// Package cmd provides the Cobra CLI for dnstm.
package cmd

import (
	"fmt"
	"os"
	"strings"

	// Import handlers to register them with actions
	_ "github.com/net2share/dnstm/internal/handlers"

	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/dnstm/internal/version"
	"github.com/net2share/go-corelib/osdetect"
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

var rootCmd = &cobra.Command{
	Use:   "dnstm",
	Short: "DNS Tunnel Manager",
	Long:  "DNS Tunnel Manager - https://github.com/net2share/dnstm",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}
		menu.InitTUI()
		return menu.RunInteractive()
	},
}

func init() {
	rootCmd.Version = version.Version

	// Register all action-based commands
	RegisterActionsWithRoot(rootCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// SetVersionInfo sets version information for the CLI.
func SetVersionInfo(ver, buildTime string) {
	version.Set(ver, buildTime)
	rootCmd.Version = ver + " (built " + buildTime + ")"
}
