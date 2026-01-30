package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelStart, HandleTunnelStart)
	actions.SetTunnelHandler(actions.ActionTunnelStop, HandleTunnelStop)
	actions.SetTunnelHandler(actions.ActionTunnelRestart, HandleTunnelRestart)
	actions.SetTunnelHandler(actions.ActionTunnelEnable, HandleTunnelEnable)
	actions.SetTunnelHandler(actions.ActionTunnelDisable, HandleTunnelDisable)
}

// HandleTunnelStart starts or restarts a tunnel.
func HandleTunnelStart(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetArg(0)
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel start <tag>")
	}

	tunnelCfg, err := GetTunnelByTag(ctx, tag)
	if err != nil {
		return err
	}

	tunnel := router.NewTunnel(tunnelCfg)

	if err := tunnel.Enable(); err != nil {
		ctx.Output.Warning("Failed to enable service: " + err.Error())
	}

	isRunning := tunnel.IsActive()
	if isRunning {
		if err := tunnel.Restart(); err != nil {
			return fmt.Errorf("failed to restart tunnel: %w", err)
		}
		ctx.Output.Success(fmt.Sprintf("Tunnel '%s' restarted", tag))
	} else {
		if err := tunnel.Start(); err != nil {
			return fmt.Errorf("failed to start tunnel: %w", err)
		}
		ctx.Output.Success(fmt.Sprintf("Tunnel '%s' started", tag))
	}

	return nil
}

// HandleTunnelStop stops a tunnel.
func HandleTunnelStop(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetArg(0)
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel stop <tag>")
	}

	tunnelCfg, err := GetTunnelByTag(ctx, tag)
	if err != nil {
		return err
	}

	tunnel := router.NewTunnel(tunnelCfg)

	if err := tunnel.Stop(); err != nil {
		return fmt.Errorf("failed to stop tunnel: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' stopped", tag))
	return nil
}

// HandleTunnelRestart restarts a tunnel.
func HandleTunnelRestart(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetArg(0)
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel restart <tag>")
	}

	tunnelCfg, err := GetTunnelByTag(ctx, tag)
	if err != nil {
		return err
	}

	tunnel := router.NewTunnel(tunnelCfg)

	if err := tunnel.Restart(); err != nil {
		return fmt.Errorf("failed to restart tunnel: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' restarted", tag))
	return nil
}

// HandleTunnelEnable enables a tunnel.
func HandleTunnelEnable(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetArg(0)
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel enable <tag>")
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	tunnelCfg := cfg.GetTunnelByTag(tag)
	if tunnelCfg == nil {
		return actions.TunnelNotFoundError(tag)
	}

	// Set enabled to true
	enabled := true
	tunnelCfg.Enabled = &enabled

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' enabled", tag))

	// In multi mode, also start the tunnel
	if cfg.IsMultiMode() {
		tunnel := router.NewTunnel(tunnelCfg)
		if err := tunnel.Enable(); err != nil {
			ctx.Output.Warning("Failed to enable service: " + err.Error())
		}
		if err := tunnel.Start(); err != nil {
			ctx.Output.Warning("Failed to start tunnel: " + err.Error())
		} else {
			ctx.Output.Info("Tunnel started")
		}
	}

	return nil
}

// HandleTunnelDisable disables a tunnel.
func HandleTunnelDisable(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetArg(0)
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel disable <tag>")
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	tunnelCfg := cfg.GetTunnelByTag(tag)
	if tunnelCfg == nil {
		return actions.TunnelNotFoundError(tag)
	}

	// Set enabled to false
	enabled := false
	tunnelCfg.Enabled = &enabled

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' disabled", tag))

	// Also stop the tunnel
	tunnel := router.NewTunnel(tunnelCfg)
	if tunnel.IsActive() {
		if err := tunnel.Stop(); err != nil {
			ctx.Output.Warning("Failed to stop tunnel: " + err.Error())
		} else {
			ctx.Output.Info("Tunnel stopped")
		}
	}

	return nil
}
