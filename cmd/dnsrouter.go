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
	Use:   "dnsrouter",
	Short: "DNS router commands",
}

var dnsrouterServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the DNS router server",
	Long: `Start the minimal DNS router that forwards raw packets.

This router extracts only the query name for routing decisions,
then forwards the entire DNS packet unchanged to the backend.
This preserves all DNS data including QUIC-over-DNS tunneling.`,
	RunE: runDNSRouterServe,
}

var dnsrouterTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test DNS packet parsing",
	RunE:  runDNSRouterTest,
}

func init() {
	rootCmd.AddCommand(dnsrouterCmd)
	dnsrouterCmd.AddCommand(dnsrouterServeCmd)
	dnsrouterCmd.AddCommand(dnsrouterTestCmd)
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

func runDNSRouterTest(cmd *cobra.Command, args []string) error {
	// Test the parser with some example packets
	fmt.Println("DNS Router Parser Test")
	fmt.Println("======================")

	// Example: test domain matching
	testCases := []struct {
		query  string
		suffix string
		expect bool
	}{
		{"test.example.com", "example.com", true},
		{"example.com", "example.com", true},
		{"foo.bar.example.com", "example.com", true},
		{"test.other.com", "example.com", false},
		{"exampleXcom", "example.com", false},
	}

	for _, tc := range testCases {
		result := dnsrouter.MatchDomainSuffix(tc.query, tc.suffix)
		status := "✓"
		if result != tc.expect {
			status = "✗"
		}
		fmt.Printf("%s MatchDomainSuffix(%q, %q) = %v (expected %v)\n",
			status, tc.query, tc.suffix, result, tc.expect)
	}

	return nil
}
