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
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetString("tag")
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel remove -t <tag>")
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	tunnelCfg := cfg.GetTunnelByTag(tag)
	if tunnelCfg == nil {
		return actions.TunnelNotFoundError(tag)
	}

	// Warn if removing the active tunnel in single mode
	if cfg.IsSingleMode() && cfg.Route.Active == tag {
		ctx.Output.Println()
		ctx.Output.Warning("This is the currently active tunnel.")
		if len(cfg.Tunnels) > 1 {
			ctx.Output.Info("After removal, run 'dnstm router switch <tag>' to activate another tunnel.")
		} else {
			ctx.Output.Info("After removal, no transport will be active. Add a new tunnel to continue.")
		}
		ctx.Output.Println()
	}

	// Confirmation is handled by the adapter (CLI or menu)
	// The handler assumes confirmation has already been obtained

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		ctx.Output.BeginProgress(fmt.Sprintf("Remove Tunnel: %s", tag))
	} else {
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

	// Update Route.Active if needed (single mode)
	if cfg.Route.Active == tag {
		cfg.Route.Active = ""
		if len(cfg.Tunnels) > 0 {
			cfg.Route.Active = cfg.Tunnels[0].Tag
		}
	}

	if err := cfg.Save(); err != nil {
		if ctx.IsInteractive {
			ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
			ctx.Output.EndProgress()
			return nil // Error already shown in progress view
		}
		return fmt.Errorf("failed to save config: %w", err)
	}
	ctx.Output.Status("Configuration updated")

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' removed!", tag))

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	} else {
		ctx.Output.Println()
	}

	return nil
}
