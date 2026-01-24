package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show service status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	if !service.IsInstalled() {
		return fmt.Errorf("dnstt is not installed")
	}

	status, _ := service.Status()
	fmt.Println(status)
	return nil
}
