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

	tag := ctx.GetString("tag")
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel start -t <tag>")
	}

	tunnelCfg, err := GetTunnelByTag(ctx, tag)
	if err != nil {
		return err
	}

	tunnel := router.NewTunnel(tunnelCfg)
	isRunning := tunnel.IsActive()

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		if isRunning {
			ctx.Output.BeginProgress(fmt.Sprintf("Restart Tunnel: %s", tag))
		} else {
			ctx.Output.BeginProgress(fmt.Sprintf("Start Tunnel: %s", tag))
		}
	}

	if err := tunnel.Enable(); err != nil {
		ctx.Output.Warning("Failed to enable service: " + err.Error())
	}

	if isRunning {
		ctx.Output.Info("Restarting tunnel...")
		if err := tunnel.Restart(); err != nil {
			if ctx.IsInteractive {
				ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
				ctx.Output.EndProgress()
				return nil // Error already shown in progress view
			}
			return fmt.Errorf("failed to restart tunnel: %w", err)
		}
		ctx.Output.Success(fmt.Sprintf("Tunnel '%s' restarted", tag))
	} else {
		ctx.Output.Info("Starting tunnel...")
		if err := tunnel.Start(); err != nil {
			if ctx.IsInteractive {
				ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
				ctx.Output.EndProgress()
				return nil // Error already shown in progress view
			}
			return fmt.Errorf("failed to start tunnel: %w", err)
		}
		ctx.Output.Success(fmt.Sprintf("Tunnel '%s' started", tag))
	}

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	}

	return nil
}

// HandleTunnelStop stops a tunnel.
func HandleTunnelStop(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetString("tag")
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel stop -t <tag>")
	}

	tunnelCfg, err := GetTunnelByTag(ctx, tag)
	if err != nil {
		return err
	}

	tunnel := router.NewTunnel(tunnelCfg)

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		ctx.Output.BeginProgress(fmt.Sprintf("Stop Tunnel: %s", tag))
	}

	ctx.Output.Info("Stopping tunnel...")

	if err := tunnel.Stop(); err != nil {
		if ctx.IsInteractive {
			ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
			ctx.Output.EndProgress()
			return nil // Error already shown in progress view
		}
		return fmt.Errorf("failed to stop tunnel: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' stopped", tag))

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	}

	return nil
}

// HandleTunnelRestart restarts a tunnel.
func HandleTunnelRestart(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetString("tag")
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel restart -t <tag>")
	}

	tunnelCfg, err := GetTunnelByTag(ctx, tag)
	if err != nil {
		return err
	}

	tunnel := router.NewTunnel(tunnelCfg)

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		ctx.Output.BeginProgress(fmt.Sprintf("Restart Tunnel: %s", tag))
	}

	ctx.Output.Info("Restarting tunnel...")

	if err := tunnel.Restart(); err != nil {
		if ctx.IsInteractive {
			ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
			ctx.Output.EndProgress()
			return nil // Error already shown in progress view
		}
		return fmt.Errorf("failed to restart tunnel: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' restarted", tag))

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	}

	return nil
}

// HandleTunnelEnable enables a tunnel.
func HandleTunnelEnable(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetString("tag")
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel enable -t <tag>")
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	tunnelCfg := cfg.GetTunnelByTag(tag)
	if tunnelCfg == nil {
		return actions.TunnelNotFoundError(tag)
	}

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		ctx.Output.BeginProgress(fmt.Sprintf("Enable Tunnel: %s", tag))
	}

	ctx.Output.Info("Enabling tunnel...")

	// Set enabled to true
	enabled := true
	tunnelCfg.Enabled = &enabled

	if err := cfg.Save(); err != nil {
		if ctx.IsInteractive {
			ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
			ctx.Output.EndProgress()
			return nil // Error already shown in progress view
		}
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
			ctx.Output.Status("Tunnel started")
		}
	}

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	}

	return nil
}

// HandleTunnelDisable disables a tunnel.
func HandleTunnelDisable(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetString("tag")
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel disable -t <tag>")
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	tunnelCfg := cfg.GetTunnelByTag(tag)
	if tunnelCfg == nil {
		return actions.TunnelNotFoundError(tag)
	}

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		ctx.Output.BeginProgress(fmt.Sprintf("Disable Tunnel: %s", tag))
	}

	ctx.Output.Info("Disabling tunnel...")

	// Set enabled to false
	enabled := false
	tunnelCfg.Enabled = &enabled

	if err := cfg.Save(); err != nil {
		if ctx.IsInteractive {
			ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
			ctx.Output.EndProgress()
			return nil // Error already shown in progress view
		}
		return fmt.Errorf("failed to save config: %w", err)
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

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	}

	return nil
}
