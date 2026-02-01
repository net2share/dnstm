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

	modeName := GetModeDisplayName(cfg.Route.Mode)
	isRunning := r.IsRunning()

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		if isRunning {
			ctx.Output.BeginProgress("Restart Router")
		} else {
			ctx.Output.BeginProgress("Start Router")
		}
	} else {
		ctx.Output.Println()
	}

	if isRunning {
		ctx.Output.Info(fmt.Sprintf("Restarting in %s mode...", modeName))
	} else {
		ctx.Output.Info(fmt.Sprintf("Starting in %s mode...", modeName))
	}

	if err := r.Restart(); err != nil {
		if ctx.IsInteractive {
			ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
			ctx.Output.EndProgress()
			return nil // Error already shown in progress view
		}
		return fmt.Errorf("failed to start: %w", err)
	}

	if isRunning {
		ctx.Output.Success("Restarted!")
	} else {
		ctx.Output.Success("Started!")
	}

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	} else {
		ctx.Output.Println()
	}

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

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		ctx.Output.BeginProgress("Stop Router")
	} else {
		ctx.Output.Println()
	}

	ctx.Output.Info("Stopping...")

	if err := r.Stop(); err != nil {
		if ctx.IsInteractive {
			ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
			ctx.Output.EndProgress()
			return nil // Error already shown in progress view
		}
		return fmt.Errorf("failed to stop: %w", err)
	}

	ctx.Output.Success("Stopped!")

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	} else {
		ctx.Output.Println()
	}

	return nil
}
