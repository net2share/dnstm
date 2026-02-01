package handlers

import (
	"fmt"

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
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetArg(0)
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel status <tag>")
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
	enabledStr := "No"
	if tunnelCfg.IsEnabled() {
		enabledStr = "Yes"
	}
	mainSection := actions.InfoSection{
		Rows: []actions.InfoRow{
			{Key: "Transport", Value: config.GetTransportTypeDisplayName(tunnelCfg.Transport)},
			{Key: "Backend", Value: tunnelCfg.Backend},
			{Key: "Domain", Value: tunnelCfg.Domain},
			{Key: "Port", Value: fmt.Sprintf("%d", tunnelCfg.Port)},
			{Key: "Service", Value: tunnel.ServiceName},
			{Key: "Status", Value: tunnel.StatusString()},
			{Key: "Enabled", Value: enabledStr},
		},
	}
	infoCfg.Sections = append(infoCfg.Sections, mainSection)

	// Show certificate/key info based on transport type
	if tunnelCfg.Transport == config.TransportSlipstream {
		certMgr := certs.NewManager()
		certInfo := certMgr.Get(tunnelCfg.Domain)
		if certInfo != nil {
			certSection := actions.InfoSection{
				Title: "Certificate Fingerprint",
				Rows: []actions.InfoRow{
					{Value: certs.FormatFingerprint(certInfo.Fingerprint)},
				},
			}
			infoCfg.Sections = append(infoCfg.Sections, certSection)
		}
	} else if tunnelCfg.Transport == config.TransportDNSTT {
		keyMgr := keys.NewManager()
		keyInfo := keyMgr.Get(tunnelCfg.Domain)
		if keyInfo != nil {
			keySection := actions.InfoSection{
				Title: "Public Key",
				Rows: []actions.InfoRow{
					{Value: keyInfo.PublicKey},
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

	if tunnelCfg.IsEnabled() {
		ctx.Output.Printf("Enabled:   Yes\n")
	} else {
		ctx.Output.Printf("Enabled:   No\n")
	}
	ctx.Output.Println()

	if tunnelCfg.Transport == config.TransportSlipstream {
		certMgr := certs.NewManager()
		certInfo := certMgr.Get(tunnelCfg.Domain)
		if certInfo != nil {
			ctx.Output.Println("Certificate Fingerprint:")
			ctx.Output.Println(certs.FormatFingerprint(certInfo.Fingerprint))
			ctx.Output.Println()
		}
	} else if tunnelCfg.Transport == config.TransportDNSTT {
		keyMgr := keys.NewManager()
		keyInfo := keyMgr.Get(tunnelCfg.Domain)
		if keyInfo != nil {
			ctx.Output.Println("Public Key:")
			ctx.Output.Println(keyInfo.PublicKey)
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
			ctx.Output.Println()
		}
	}

	return nil
}
