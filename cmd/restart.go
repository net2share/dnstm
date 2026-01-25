package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/tunnel"
	_ "github.com/net2share/dnstm/internal/tunnel/dnstt"
	_ "github.com/net2share/dnstm/internal/tunnel/slipstream"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart <provider>",
	Short: "Restart a DNS tunnel service",
	Long:  "Restart a DNS tunnel provider service (dnstt or slipstream)",
	Args:  cobra.ExactArgs(1),
	RunE:  runRestart,
}

func runRestart(cmd *cobra.Command, args []string) error {
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

	fmt.Printf("Restarting %s...\n", provider.ServiceName())
	if err := provider.Restart(); err != nil {
		return err
	}
	fmt.Println("Service restarted successfully")
	return nil
}
