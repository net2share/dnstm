package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the dnstt service",
	RunE:  runRestart,
}

func runRestart(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	if !service.IsInstalled() {
		return fmt.Errorf("dnstt is not installed")
	}

	fmt.Println("Restarting dnstt-server...")
	if err := service.Restart(); err != nil {
		return err
	}
	fmt.Println("Service restarted successfully")
	return nil
}
