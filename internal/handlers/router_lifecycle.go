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
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	ctx.Output.Println()
	modeName := GetModeDisplayName(cfg.Route.Mode)
	isRunning := r.IsRunning()

	if isRunning {
		ctx.Output.Info(fmt.Sprintf("Restarting in %s mode...", modeName))
	} else {
		ctx.Output.Info(fmt.Sprintf("Starting in %s mode...", modeName))
	}
	ctx.Output.Println()

	if err := r.Restart(); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	ctx.Output.Println()
	if isRunning {
		ctx.Output.Success("Restarted!")
	} else {
		ctx.Output.Success("Started!")
	}
	ctx.Output.Println()

	return nil
}

// HandleRouterStop stops the router.
func HandleRouterStop(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	ctx.Output.Println()
	ctx.Output.Info("Stopping...")
	ctx.Output.Println()

	if err := r.Stop(); err != nil {
		return fmt.Errorf("failed to stop: %w", err)
	}

	ctx.Output.Println()
	ctx.Output.Success("Stopped!")
	ctx.Output.Println()

	return nil
}
