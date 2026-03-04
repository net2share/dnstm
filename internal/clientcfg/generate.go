package clientcfg

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/keys"
)

// GenerateOptions carries runtime inputs not stored in server config.
type GenerateOptions struct {
	// SSH backend fields
	User       string
	Password   string
	PrivateKey string // path to SSH private key

	// Slipstream options
	NoCert bool // skip embedding certificate
}

// Generate builds a ClientConfig from server-side tunnel and backend config.
func Generate(tunnel *config.TunnelConfig, backend *config.BackendConfig, opts GenerateOptions) (*ClientConfig, error) {
	cfg := &ClientConfig{
		Version: 1,
		Tag:     tunnel.Tag,
	}

	// Build transport config
	cfg.Transport.Type = string(tunnel.Transport)
	cfg.Transport.Domain = tunnel.Domain

	tunnelDir := filepath.Join(config.TunnelsDir, tunnel.Tag)

	switch tunnel.Transport {
	case config.TransportSlipstream:
		if !opts.NoCert {
			certPath := filepath.Join(tunnelDir, "cert.pem")
			if tunnel.Slipstream != nil && tunnel.Slipstream.Cert != "" {
				certPath = tunnel.Slipstream.Cert
			}
			certPEM, err := os.ReadFile(certPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read certificate: %w", err)
			}
			cfg.Transport.Cert = string(certPEM)
		}

	case config.TransportDNSTT:
		pubKeyPath := filepath.Join(tunnelDir, "server.pub")
		pubKey, err := keys.ReadPublicKey(pubKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read public key: %w", err)
		}
		cfg.Transport.PubKey = pubKey
	}

	// Build backend config
	cfg.Backend.Type = string(backend.Type)

	switch backend.Type {
	case config.BackendSOCKS:
		if backend.HasSocksAuth() {
			cfg.Backend.User = backend.Socks.User
			cfg.Backend.Password = backend.Socks.Password
		}

	case config.BackendSSH:
		cfg.Backend.User = opts.User
		cfg.Backend.Password = opts.Password
		if opts.PrivateKey != "" {
			keyData, err := os.ReadFile(opts.PrivateKey)
			if err != nil {
				return nil, fmt.Errorf("failed to read private key: %w", err)
			}
			cfg.Backend.Key = string(keyData)
		}

	case config.BackendShadowsocks:
		if backend.Shadowsocks == nil {
			return nil, fmt.Errorf("shadowsocks config is missing")
		}
		cfg.Backend.Method = backend.Shadowsocks.Method
		cfg.Backend.Password = backend.Shadowsocks.Password
	}

	return cfg, nil
}
