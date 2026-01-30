package handlers

import (
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

	// Print formatted info
	ctx.Output.Println()
	ctx.Output.Println(tunnel.GetFormattedInfo())

	// Show enabled status
	if tunnelCfg.IsEnabled() {
		ctx.Output.Printf("Enabled:   Yes\n")
	} else {
		ctx.Output.Printf("Enabled:   No\n")
	}
	ctx.Output.Println()

	// Show certificate/key info based on transport type
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

	// Show backend info
	cfg, _ := LoadConfig(ctx)
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
