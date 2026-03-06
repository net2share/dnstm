package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/network"
	"github.com/spf13/cobra"
)

var dnsrouterCmd = &cobra.Command{
	Use:    "dnsrouter",
	Short:  "DNS router commands",
	Hidden: true,
}

var dnsrouterServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the DNS router server",
	RunE:  runDNSRouterServe,
}



func init() {
	rootCmd.AddCommand(dnsrouterCmd)
	dnsrouterCmd.AddCommand(dnsrouterServeCmd)
	rootCmd.PersistentFlags().StringVarP(&config.ConfigDir, "confdir", "C", "", "Config directory")
	rootCmd.PersistentFlags().StringVarP(&config.TunnelsDir, "tunnelsdir", "T", "", "Tunnels directory")

}

func runDNSRouterServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Derive routes from enabled tunnels
	var routes []dnsrouter.Route
	for _, t := range cfg.Tunnels {
		if t.IsEnabled() {
			routes = append(routes, dnsrouter.Route{
				Domain:  t.Domain,
				Backend: fmt.Sprintf("127.0.0.1:%d", t.Port),
			})
		}
	}

	// Derive default backend
	defaultBackend := ""
	if cfg.Route.Default != "" {
		if t := cfg.GetTunnelByTag(cfg.Route.Default); t != nil {
			defaultBackend = fmt.Sprintf("127.0.0.1:%d", t.Port)
		}
	}

	// Resolve listen address (0.0.0.0 → external IP)
	listenAddr := network.ResolveListenAddress(cfg.Listen.Address)

	// Create forwarder using factory
	forwarder, err := dnsrouter.NewForwarder(
		dnsrouter.ForwarderTypeNative,
		dnsrouter.ForwarderConfig{
			ListenAddr:     listenAddr,
			Routes:         routes,
			DefaultBackend: defaultBackend,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create forwarder: %w", err)
	}

	// Start forwarder
	if err := forwarder.Start(); err != nil {
		return fmt.Errorf("failed to start forwarder: %w", err)
	}

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("DNS router running. Press Ctrl+C to stop.")
	<-sigCh

	log.Printf("Shutting down...")
	return forwarder.Stop()
}
