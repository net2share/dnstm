package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	RunE:  runConfig,
}

func runConfig(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	if !config.Exists() {
		return fmt.Errorf("dnstt is not configured")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	publicKey := ""
	if cfg.PublicKeyFile != "" {
		publicKey, _ = keys.ReadPublicKey(cfg.PublicKeyFile)
	}

	fmt.Printf("NS Subdomain:    %s\n", cfg.NSSubdomain)
	fmt.Printf("Tunnel Mode:     %s\n", cfg.TunnelMode)
	fmt.Printf("MTU:             %s\n", cfg.MTU)
	fmt.Printf("Target Port:     %s\n", cfg.TargetPort)
	fmt.Printf("Private Key:     %s\n", cfg.PrivateKeyFile)
	fmt.Printf("Public Key File: %s\n", cfg.PublicKeyFile)
	fmt.Println()
	fmt.Println("Public Key (for client):")
	fmt.Println(publicKey)

	return nil
}
