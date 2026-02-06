package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetRouterHandler(actions.ActionRouterSwitch, HandleRouterSwitch)
}

// HandleRouterSwitch switches the active tunnel in single mode.
func HandleRouterSwitch(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check mode
	if !cfg.IsSingleMode() {
		return actions.SingleModeOnlyError()
	}

	// Check if there are tunnels to switch to
	if len(cfg.Tunnels) == 0 {
		return actions.NoTunnelsError()
	}

	tunnelTag := ctx.GetString("tag")

	// If only one tunnel, just make sure it's active
	if len(cfg.Tunnels) == 1 {
		tag := cfg.Tunnels[0].Tag
		if cfg.Route.Active == tag {
			ctx.Output.Info(fmt.Sprintf("'%s' is already the active tunnel", tag))
			return nil
		}
		tunnelTag = tag
	}

	// If no tunnel tag provided, the adapter should have shown a picker
	if tunnelTag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm router switch -t <tag>")
	}

	// Verify tunnel exists
	tunnel := cfg.GetTunnelByTag(tunnelTag)
	if tunnel == nil {
		return actions.TunnelNotFoundError(tunnelTag)
	}

	// Check if already active
	if cfg.Route.Active == tunnelTag {
		ctx.Output.Info(fmt.Sprintf("'%s' is already the active tunnel", tunnelTag))
		return nil
	}

	// Create router and switch
	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		ctx.Output.BeginProgress("Switch Active Tunnel")
	} else {
		ctx.Output.Println()
	}

	ctx.Output.Info(fmt.Sprintf("Switching to '%s'...", tunnelTag))

	if err := r.SwitchActiveTunnel(tunnelTag); err != nil {
		if ctx.IsInteractive {
			ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
			ctx.Output.EndProgress()
			return nil // Error already shown in progress view
		}
		return fmt.Errorf("failed to switch tunnel: %w", err)
	}

	// Show success
	transportName := config.GetTransportTypeDisplayName(tunnel.Transport)

	ctx.Output.Success(fmt.Sprintf("Switched to '%s'", tunnelTag))
	ctx.Output.Println()
	ctx.Output.Status(fmt.Sprintf("Transport: %s", transportName))
	ctx.Output.Status(fmt.Sprintf("Backend: %s", tunnel.Backend))
	ctx.Output.Status(fmt.Sprintf("Domain: %s", tunnel.Domain))
	ctx.Output.Status(fmt.Sprintf("Port: %d", tunnel.Port))

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	} else {
		ctx.Output.Println()
	}

	return nil
}
