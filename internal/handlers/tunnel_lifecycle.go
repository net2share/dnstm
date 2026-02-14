package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelStart, HandleTunnelStart)
	actions.SetTunnelHandler(actions.ActionTunnelStop, HandleTunnelStop)
	actions.SetTunnelHandler(actions.ActionTunnelRestart, HandleTunnelRestart)
}

// HandleTunnelStart enables and starts a tunnel.
func HandleTunnelStart(ctx *actions.Context) error {
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

	// Single mode guard: must be the active tunnel
	if cfg.IsSingleMode() && cfg.Route.Active != tag {
		return fmt.Errorf("tunnel '%s' is not the active tunnel. Switch with: dnstm router switch -t %s", tag, tag)
	}

	tunnel := router.NewTunnel(tunnelCfg)
	isRunning := tunnel.IsActive()

	if isRunning {
		beginProgress(ctx, fmt.Sprintf("Restart Tunnel: %s", tag))
	} else {
		beginProgress(ctx, fmt.Sprintf("Start Tunnel: %s", tag))
	}

	// Enable in config
	enabled := true
	tunnelCfg.Enabled = &enabled
	if err := cfg.Save(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to save config: %w", err))
	}

	// Enable systemd service
	if err := tunnel.Enable(); err != nil {
		ctx.Output.Warning("Failed to enable service: " + err.Error())
	}

	// Multi mode: regenerate DNS router config and restart DNS router
	if cfg.IsMultiMode() {
		if err := regenerateDNSRouter(cfg); err != nil {
			ctx.Output.Warning("Failed to update DNS router: " + err.Error())
		}
	}

	// Start or restart
	if isRunning {
		ctx.Output.Info("Restarting tunnel...")
		if err := tunnel.Restart(); err != nil {
			// Roll back enabled state
			rollbackEnabled(tunnelCfg, cfg, false)
			return failProgress(ctx, fmt.Errorf("failed to restart tunnel: %w", err))
		}
		ctx.Output.Success(fmt.Sprintf("Tunnel '%s' restarted", tag))
	} else {
		ctx.Output.Info("Starting tunnel...")
		if err := tunnel.Start(); err != nil {
			// Roll back enabled state
			rollbackEnabled(tunnelCfg, cfg, false)
			return failProgress(ctx, fmt.Errorf("failed to start tunnel: %w", err))
		}
		ctx.Output.Success(fmt.Sprintf("Tunnel '%s' started", tag))
	}

	endProgress(ctx)
	return nil
}

// HandleTunnelStop stops and disables a tunnel.
func HandleTunnelStop(ctx *actions.Context) error {
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

	tunnel := router.NewTunnel(tunnelCfg)

	// Guard: if not running, just inform
	if !tunnel.IsActive() {
		ctx.Output.Info(fmt.Sprintf("Tunnel '%s' is not running", tag))
		return nil
	}

	beginProgress(ctx, fmt.Sprintf("Stop Tunnel: %s", tag))
	ctx.Output.Info("Stopping tunnel...")

	// Stop the tunnel
	if err := tunnel.Stop(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to stop tunnel: %w", err))
	}

	// Disable systemd service
	if err := tunnel.Disable(); err != nil {
		ctx.Output.Warning("Failed to disable service: " + err.Error())
	}

	// Disable in config
	enabled := false
	tunnelCfg.Enabled = &enabled
	if err := cfg.Save(); err != nil {
		ctx.Output.Warning("Failed to save config: " + err.Error())
	}

	// Multi mode: regenerate DNS router config and restart DNS router
	if cfg.IsMultiMode() {
		if err := regenerateDNSRouter(cfg); err != nil {
			ctx.Output.Warning("Failed to update DNS router: " + err.Error())
		}
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' stopped", tag))

	// Warn if stopping the active tunnel in single mode
	if cfg.IsSingleMode() && cfg.Route.Active == tag {
		ctx.Output.Warning("This is the active tunnel in single mode. No tunnel will be serving traffic.")
	}

	endProgress(ctx)
	return nil
}

// HandleTunnelRestart restarts a running tunnel.
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

	// Guard: must be running
	if !tunnel.IsActive() {
		return fmt.Errorf("tunnel '%s' is not running. Use start instead", tag)
	}

	beginProgress(ctx, fmt.Sprintf("Restart Tunnel: %s", tag))
	ctx.Output.Info("Restarting tunnel...")

	if err := tunnel.Restart(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to restart tunnel: %w", err))
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' restarted", tag))
	endProgress(ctx)
	return nil
}

// regenerateDNSRouter regenerates DNS router config and restarts it.
func regenerateDNSRouter(cfg *config.Config) error {
	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}
	if err := r.RegenerateDNSRouterConfig(); err != nil {
		return fmt.Errorf("failed to regenerate DNS router config: %w", err)
	}
	dnsRouter := r.GetDNSRouterService()
	if dnsRouter.IsActive() {
		if err := dnsRouter.Restart(); err != nil {
			return fmt.Errorf("failed to restart DNS router: %w", err)
		}
	}
	return nil
}

// rollbackEnabled rolls back the Enabled config field and saves.
func rollbackEnabled(tunnelCfg *config.TunnelConfig, cfg *config.Config, value bool) {
	tunnelCfg.Enabled = &value
	_ = cfg.Save()
}
