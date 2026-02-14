package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelRemove, HandleTunnelRemove)
}

// HandleTunnelRemove removes a tunnel.
func HandleTunnelRemove(ctx *actions.Context) error {
	cfg, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	tag, err := RequireTag(ctx, "tunnel")
	if err != nil {
		return err
	}

	tunnelCfg := cfg.GetTunnelByTag(tag)
	if tunnelCfg == nil {
		return actions.TunnelNotFoundError(tag)
	}

	// Track if removing the active tunnel in single mode (for warning after removal)
	wasActiveSingleMode := cfg.IsSingleMode() && cfg.Route.Active == tag
	remainingTunnels := len(cfg.Tunnels) - 1

	// Confirmation is handled by the adapter (CLI or menu)
	// The handler assumes confirmation has already been obtained

	beginProgress(ctx, fmt.Sprintf("Remove Tunnel: %s", tag))
	if !ctx.IsInteractive {
		ctx.Output.Println()
	}

	ctx.Output.Info("Removing tunnel...")

	totalSteps := 3
	currentStep := 0

	// Step 1: Stop and remove service
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing service...")
	tunnel := router.NewTunnel(tunnelCfg)
	if err := tunnel.RemoveService(); err != nil {
		ctx.Output.Warning("Service removal warning: " + err.Error())
	} else {
		ctx.Output.Status("Service removed")
	}

	// Step 2: Remove config directory
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing configuration...")
	if err := tunnel.RemoveConfigDir(); err != nil {
		ctx.Output.Warning("Config removal warning: " + err.Error())
	} else {
		ctx.Output.Status("Configuration removed")
	}

	// Step 3: Update config
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Updating router configuration...")

	// Remove tunnel from config
	var newTunnels []config.TunnelConfig
	for _, t := range cfg.Tunnels {
		if t.Tag != tag {
			newTunnels = append(newTunnels, t)
		}
	}
	cfg.Tunnels = newTunnels

	// Update Route.Default if needed (multi mode)
	if cfg.Route.Default == tag {
		cfg.Route.Default = ""
		if len(cfg.Tunnels) > 0 {
			cfg.Route.Default = cfg.Tunnels[0].Tag
		}
	}

	// Clear Route.Active if removing the active tunnel (single mode)
	if cfg.Route.Active == tag {
		cfg.Route.Active = ""
	}

	if err := cfg.Save(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to save config: %w", err))
	}
	ctx.Output.Status("Configuration updated")

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' removed!", tag))

	// Warn after removal if it was the active tunnel in single mode
	if wasActiveSingleMode {
		ctx.Output.Warning("This was the active tunnel in single mode. No tunnel will be serving traffic.")
		if remainingTunnels > 0 {
			ctx.Output.Info("Use 'dnstm router switch -t <tag>' to activate another tunnel.")
		}
	}

	endProgress(ctx)
	if !ctx.IsInteractive {
		ctx.Output.Println()
	}

	return nil
}
