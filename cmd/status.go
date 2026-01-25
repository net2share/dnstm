package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/tunnel"
	_ "github.com/net2share/dnstm/internal/tunnel/dnstt"
	_ "github.com/net2share/dnstm/internal/tunnel/slipstream"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [provider]",
	Short: "Show service status",
	Long:  "Show the status of a DNS tunnel provider. If no provider is specified, shows combined status.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	// If no provider specified, show combined status
	if len(args) == 0 {
		menu.ShowOverallStatus()
		return nil
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

	status, _ := provider.GetServiceStatus()
	fmt.Println(status)
	return nil
}
