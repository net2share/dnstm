package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/tunnel"
	_ "github.com/net2share/dnstm/internal/tunnel/dnstt"
	_ "github.com/net2share/dnstm/internal/tunnel/slipstream"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <provider>",
	Short: "View service logs",
	Long:  "View logs for a DNS tunnel provider (dnstt or slipstream)",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

func runLogs(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	// Parse provider type
	pt, err := tunnel.ParseProviderType(args[0])
	if err != nil {
		return err
	}

	provider, err := tunnel.Get(pt)
	if err != nil {
		return err
	}

	if !provider.IsInstalled() {
		return fmt.Errorf("%s is not installed", provider.DisplayName())
	}

	logs, err := provider.GetLogs(50)
	if err != nil {
		return err
	}
	fmt.Println(logs)
	return nil
}
