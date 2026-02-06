package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/net2share/dnstm/internal/dnsrouter"
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
}

func runDNSRouterServe(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := dnsrouter.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create forwarder using factory (allows easy switching between implementations)
	forwarder, err := dnsrouter.NewForwarder(
		dnsrouter.ForwarderType(cfg.ForwarderType),
		dnsrouter.ForwarderConfig{
			ListenAddr:     cfg.Listen,
			Routes:         cfg.ToRoutes(),
			DefaultBackend: cfg.Default,
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
