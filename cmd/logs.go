package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View service logs",
	RunE:  runLogs,
}

func runLogs(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	if !service.IsInstalled() {
		return fmt.Errorf("dnstt is not installed")
	}

	logs, err := service.GetLogs(50)
	if err != nil {
		return err
	}
	fmt.Println(logs)
	return nil
}
