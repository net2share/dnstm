package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/types"
)

func init() {
	actions.SetInstanceHandler(actions.ActionInstanceStatus, HandleInstanceStatus)
}

// HandleInstanceStatus shows status for a specific instance.
func HandleInstanceStatus(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	name := ctx.GetArg(0)
	if name == "" {
		return actions.NewActionError("instance name required", "Usage: dnstm instance status <name>")
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	transportCfg, exists := cfg.Transports[name]
	if !exists {
		return actions.NotFoundError(name)
	}

	instance, err := GetInstanceByName(ctx, name)
	if err != nil {
		return err
	}

	// Print formatted info
	ctx.Output.Println(instance.GetFormattedInfo())

	// Show certificate/key info
	if types.IsSlipstreamType(transportCfg.Type) {
		certMgr := certs.NewManager()
		certInfo := certMgr.Get(transportCfg.Domain)
		if certInfo != nil {
			ctx.Output.Println("Certificate Fingerprint:")
			ctx.Output.Println(certs.FormatFingerprint(certInfo.Fingerprint))
			ctx.Output.Println()
		}
	} else if types.IsDNSTTType(transportCfg.Type) {
		keyMgr := keys.NewManager()
		keyInfo := keyMgr.Get(transportCfg.Domain)
		if keyInfo != nil {
			ctx.Output.Println("Public Key:")
			ctx.Output.Println(keyInfo.PublicKey)
			ctx.Output.Println()
		}
	}

	return nil
}
