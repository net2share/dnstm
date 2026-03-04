package handlers

import (
	"fmt"
	"path/filepath"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelStatus, HandleTunnelStatus)
}

// HandleTunnelStatus shows status for a specific tunnel.
func HandleTunnelStatus(ctx *actions.Context) error {
	if _, err := RequireConfig(ctx); err != nil {
		return err
	}

	tag, err := RequireTag(ctx, "tunnel")
	if err != nil {
		return err
	}

	tunnelCfg, err := GetTunnelByTag(ctx, tag)
	if err != nil {
		return err
	}

	tunnel := router.NewTunnel(tunnelCfg)
	cfg, _ := LoadConfig(ctx)

	// Build info config
	infoCfg := actions.InfoConfig{
		Title: fmt.Sprintf("Tunnel: %s", tag),
	}

	// Main info section
	mainSection := actions.InfoSection{
		Rows: []actions.InfoRow{
			{Key: "Transport", Value: config.GetTransportTypeDisplayName(tunnelCfg.Transport)},
			{Key: "Backend", Value: tunnelCfg.Backend},
			{Key: "Domain", Value: tunnelCfg.Domain},
			{Key: "Port", Value: fmt.Sprintf("%d", tunnelCfg.Port)},
			{Key: "Service", Value: tunnel.ServiceName},
			{Key: "Status", Value: tunnel.StatusString()},
		},
	}
	if tunnelCfg.Transport == config.TransportDNSTT && tunnelCfg.DNSTT != nil {
		mainSection.Rows = append(mainSection.Rows, actions.InfoRow{
			Key: "MTU", Value: fmt.Sprintf("%d", tunnelCfg.DNSTT.MTU),
		})
	}
	infoCfg.Sections = append(infoCfg.Sections, mainSection)

	// Show certificate/key info based on transport type
	tunnelDir := filepath.Join(config.TunnelsDir, tunnelCfg.Tag)
	if tunnelCfg.Transport == config.TransportSlipstream {
		certPath := filepath.Join(tunnelDir, "cert.pem")
		if tunnelCfg.Slipstream != nil && tunnelCfg.Slipstream.Cert != "" {
			certPath = tunnelCfg.Slipstream.Cert
		}
		fingerprint, err := certs.ReadCertificateFingerprint(certPath)
		if err == nil {
			certSection := actions.InfoSection{
				Title: "Certificate Fingerprint",
				Rows: []actions.InfoRow{
					{Value: certs.FormatFingerprint(fingerprint)},
				},
			}
			infoCfg.Sections = append(infoCfg.Sections, certSection)
		}
	} else if tunnelCfg.Transport == config.TransportDNSTT {
		pubKeyPath := filepath.Join(tunnelDir, "server.pub")
		pubKey, err := keys.ReadPublicKey(pubKeyPath)
		if err == nil {
			keySection := actions.InfoSection{
				Title: "Public Key",
				Rows: []actions.InfoRow{
					{Value: pubKey},
				},
			}
			infoCfg.Sections = append(infoCfg.Sections, keySection)
		}
	}

	// Show backend info
	if cfg != nil {
		backend := cfg.GetBackendByTag(tunnelCfg.Backend)
		if backend != nil {
			backendSection := actions.InfoSection{
				Title: "Backend Info",
				Rows: []actions.InfoRow{
					{Key: "Type", Value: config.GetBackendTypeDisplayName(backend.Type)},
				},
			}
			if backend.Address != "" {
				backendSection.Rows = append(backendSection.Rows, actions.InfoRow{
					Key: "Address", Value: backend.Address,
				})
			}
			if backend.Type == config.BackendSOCKS {
				if backend.HasSocksAuth() {
					backendSection.Rows = append(backendSection.Rows,
						actions.InfoRow{Key: "Auth", Value: "Enabled"},
						actions.InfoRow{Key: "User", Value: backend.Socks.User},
						actions.InfoRow{Key: "Password", Value: backend.Socks.Password},
					)
				} else {
					backendSection.Rows = append(backendSection.Rows,
						actions.InfoRow{Key: "Auth", Value: "Disabled"},
					)
				}
			}
			infoCfg.Sections = append(infoCfg.Sections, backendSection)
		}
	}

	// Display using TUI in interactive mode
	if ctx.IsInteractive {
		return ctx.Output.ShowInfo(infoCfg)
	}

	// CLI mode - print to console
	ctx.Output.Println()
	ctx.Output.Println(tunnel.GetFormattedInfo())

	if tunnelCfg.Transport == config.TransportSlipstream {
		certPath := filepath.Join(tunnelDir, "cert.pem")
		if tunnelCfg.Slipstream != nil && tunnelCfg.Slipstream.Cert != "" {
			certPath = tunnelCfg.Slipstream.Cert
		}
		fingerprint, err := certs.ReadCertificateFingerprint(certPath)
		if err == nil {
			ctx.Output.Println("Certificate Fingerprint:")
			ctx.Output.Println(certs.FormatFingerprint(fingerprint))
			ctx.Output.Println()
		}
	} else if tunnelCfg.Transport == config.TransportDNSTT {
		pubKeyPath := filepath.Join(tunnelDir, "server.pub")
		pubKey, err := keys.ReadPublicKey(pubKeyPath)
		if err == nil {
			ctx.Output.Println("Public Key:")
			ctx.Output.Println(pubKey)
			ctx.Output.Println()
		}
	}

	if cfg != nil {
		backend := cfg.GetBackendByTag(tunnelCfg.Backend)
		if backend != nil {
			ctx.Output.Println("Backend Info:")
			ctx.Output.Printf("  Type:    %s\n", config.GetBackendTypeDisplayName(backend.Type))
			if backend.Address != "" {
				ctx.Output.Printf("  Address: %s\n", backend.Address)
			}
			if backend.Type == config.BackendSOCKS {
				if backend.HasSocksAuth() {
					ctx.Output.Printf("  Auth:     Enabled\n")
					ctx.Output.Printf("  User:     %s\n", backend.Socks.User)
					ctx.Output.Printf("  Password: %s\n", backend.Socks.Password)
				} else {
					ctx.Output.Printf("  Auth:     Disabled\n")
				}
			}
			ctx.Output.Println()
		}
	}

	return nil
}
