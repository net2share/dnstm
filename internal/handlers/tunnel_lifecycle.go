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
	isRunning := tunnel.IsActive()

	if isRunning {
		beginProgress(ctx, fmt.Sprintf("Restart Tunnel: %s", tag))
	} else {
		beginProgress(ctx, fmt.Sprintf("Start Tunnel: %s", tag))
	}

	if err := tunnel.Enable(); err != nil {
		ctx.Output.Warning("Failed to enable service: " + err.Error())
	}

	if isRunning {
		ctx.Output.Info("Restarting tunnel...")
		if err := tunnel.Restart(); err != nil {
			return failProgress(ctx, fmt.Errorf("failed to restart tunnel: %w", err))
		}
		ctx.Output.Success(fmt.Sprintf("Tunnel '%s' restarted", tag))
	} else {
		ctx.Output.Info("Starting tunnel...")
		if err := tunnel.Start(); err != nil {
			return failProgress(ctx, fmt.Errorf("failed to start tunnel: %w", err))
		}
		ctx.Output.Success(fmt.Sprintf("Tunnel '%s' started", tag))
	}

	endProgress(ctx)
	return nil
}

// HandleTunnelStop stops a tunnel.
func HandleTunnelStop(ctx *actions.Context) error {
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

	beginProgress(ctx, fmt.Sprintf("Stop Tunnel: %s", tag))
	ctx.Output.Info("Stopping tunnel...")

	if err := tunnel.Stop(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to stop tunnel: %w", err))
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' stopped", tag))
	endProgress(ctx)
	return nil
}

// HandleTunnelRestart restarts a tunnel.
func HandleTunnelRestart(ctx *actions.Context) error {
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

	beginProgress(ctx, fmt.Sprintf("Restart Tunnel: %s", tag))
	ctx.Output.Info("Restarting tunnel...")

	if err := tunnel.Restart(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to restart tunnel: %w", err))
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' restarted", tag))
	endProgress(ctx)
	return nil
}

// HandleTunnelEnable enables a tunnel.
func HandleTunnelEnable(ctx *actions.Context) error {
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

	beginProgress(ctx, fmt.Sprintf("Enable Tunnel: %s", tag))
	ctx.Output.Info("Enabling tunnel...")

	enabled := true
	tunnelCfg.Enabled = &enabled

	if err := cfg.Save(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to save config: %w", err))
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
			ctx.Output.Status("Tunnel started")
		}
	}

	endProgress(ctx)
	return nil
}

// HandleTunnelDisable disables a tunnel.
func HandleTunnelDisable(ctx *actions.Context) error {
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

	beginProgress(ctx, fmt.Sprintf("Disable Tunnel: %s", tag))
	ctx.Output.Info("Disabling tunnel...")

	enabled := false
	tunnelCfg.Enabled = &enabled

	if err := cfg.Save(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to save config: %w", err))
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' disabled", tag))

	// Also stop the tunnel
	tunnel := router.NewTunnel(tunnelCfg)
	if tunnel.IsActive() {
		if err := tunnel.Stop(); err != nil {
			ctx.Output.Warning("Failed to stop tunnel: " + err.Error())
		} else {
			ctx.Output.Status("Tunnel stopped")
		}
	}

	endProgress(ctx)
	return nil
}
