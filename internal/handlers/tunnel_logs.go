package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelLogs, HandleTunnelLogs)
}

// HandleTunnelLogs shows logs for a specific tunnel.
func HandleTunnelLogs(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetString("tag")
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel logs -t <tag>")
	}

	tunnelCfg, err := GetTunnelByTag(ctx, tag)
	if err != nil {
		return err
	}

	tunnel := router.NewTunnel(tunnelCfg)

	lines := ctx.GetInt("lines")
	if lines == 0 {
		lines = 50 // default
	}

	logs, err := tunnel.GetLogs(lines)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	ctx.Output.Println(logs)
	return nil
}
