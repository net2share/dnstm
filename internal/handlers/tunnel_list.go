package handlers

import (
	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelList, HandleTunnelList)
}

// HandleTunnelList lists all configured tunnels.
func HandleTunnelList(ctx *actions.Context) error {
	cfg, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	if len(cfg.Tunnels) == 0 {
		ctx.Output.Println("No tunnels configured")
		return nil
	}

	ctx.Output.Println()
	modeName := GetModeDisplayName(cfg.Route.Mode)
	ctx.Output.Printf("Mode: %s\n\n", modeName)

	// Print header
	ctx.Output.Printf("%-16s %-12s %-16s %-8s %-20s %s\n", "TAG", "TRANSPORT", "BACKEND", "PORT", "DOMAIN", "STATUS")
	ctx.Output.Separator(90)

	// Print tunnels
	for _, t := range cfg.Tunnels {
		tunnel := router.NewTunnel(&t)
		status := "Stopped"
		if tunnel.IsActive() {
			status = "Running"
		}

		// Add marker for active/default tunnel
		marker := ""
		if cfg.IsSingleMode() && cfg.Route.Active == t.Tag {
			marker = " *"
		} else if cfg.IsMultiMode() && cfg.Route.Default == t.Tag {
			marker = " (default)"
		}

		transportName := config.GetTransportTypeDisplayName(t.Transport)
		ctx.Output.Printf("%-16s %-12s %-16s %-8d %-20s %s%s\n",
			t.Tag, transportName, t.Backend, t.Port, t.Domain, status, marker)
	}

	if cfg.IsSingleMode() {
		ctx.Output.Println("\n* = active tunnel")
	}
	ctx.Output.Println()

	return nil
}
