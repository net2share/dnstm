package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetRouterHandler(actions.ActionRouterStart, HandleRouterStart)
	actions.SetRouterHandler(actions.ActionRouterStop, HandleRouterStop)
}

// HandleRouterStart starts or restarts the router.
func HandleRouterStart(ctx *actions.Context) error {
	cfg, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	modeName := GetModeDisplayName(cfg.Route.Mode)
	isRunning := r.IsRunning()

	if isRunning {
		beginProgress(ctx, "Restart Router")
	} else {
		beginProgress(ctx, "Start Router")
	}
	if !ctx.IsInteractive {
		ctx.Output.Println()
	}

	if isRunning {
		ctx.Output.Info(fmt.Sprintf("Restarting in %s mode...", modeName))
	} else {
		ctx.Output.Info(fmt.Sprintf("Starting in %s mode...", modeName))
	}

	if err := r.Restart(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to start: %w", err))
	}

	if isRunning {
		ctx.Output.Success("Restarted!")
	} else {
		ctx.Output.Success("Started!")
	}

	endProgress(ctx)
	if !ctx.IsInteractive {
		ctx.Output.Println()
	}

	return nil
}

// HandleRouterStop stops the router.
func HandleRouterStop(ctx *actions.Context) error {
	cfg, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	beginProgress(ctx, "Stop Router")
	if !ctx.IsInteractive {
		ctx.Output.Println()
	}

	ctx.Output.Info("Stopping...")

	if err := r.Stop(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to stop: %w", err))
	}

	ctx.Output.Success("Stopped!")

	endProgress(ctx)
	if !ctx.IsInteractive {
		ctx.Output.Println()
	}

	return nil
}
