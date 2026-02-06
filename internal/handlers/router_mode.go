package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetRouterHandler(actions.ActionRouterMode, HandleRouterMode)
}

// HandleRouterMode shows or sets the operating mode.
func HandleRouterMode(ctx *actions.Context) error {
	cfg, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	// Get mode from input (interactive) or args (CLI)
	var modeStr string
	if ctx.IsInteractive {
		modeStr = ctx.GetString("mode")
	} else if ctx.HasArg(0) {
		modeStr = ctx.GetArg(0)
	}

	// No mode specified - show current mode (CLI only)
	if modeStr == "" {
		return showCurrentMode(ctx, cfg)
	}

	// Validate mode
	if modeStr != "single" && modeStr != "multi" {
		return actions.NewActionError(
			fmt.Sprintf("invalid mode '%s'", modeStr),
			"Use 'single' or 'multi'",
		)
	}

	return switchMode(ctx, cfg, modeStr)
}

func showCurrentMode(ctx *actions.Context, cfg *config.Config) error {
	ctx.Output.Println()

	modeName := GetModeDisplayName(cfg.Route.Mode)
	var lines []string
	lines = append(lines, fmt.Sprintf("Mode: %s", modeName))

	if cfg.IsSingleMode() {
		if cfg.Route.Active != "" {
			lines = append(lines, fmt.Sprintf("Active tunnel: %s", cfg.Route.Active))
			if tunnel := cfg.GetTunnelByTag(cfg.Route.Active); tunnel != nil {
				lines = append(lines, fmt.Sprintf("Domain: %s", tunnel.Domain))
			}
		} else {
			lines = append(lines, "Active tunnel: (none)")
		}
		lines = append(lines, "")
		lines = append(lines, "Use 'dnstm router switch <tag>' to change active tunnel")
	} else {
		lines = append(lines, fmt.Sprintf("Tunnels: %d", len(cfg.Tunnels)))
		if cfg.Route.Default != "" {
			lines = append(lines, fmt.Sprintf("Default route: %s", cfg.Route.Default))
		}
	}

	ctx.Output.Box("Operating Mode", lines)
	ctx.Output.Println()

	return nil
}

func switchMode(ctx *actions.Context, cfg *config.Config, newMode string) error {
	newModeName := GetModeDisplayName(newMode)

	if cfg.Route.Mode == newMode {
		ctx.Output.Info(fmt.Sprintf("Already in %s mode", newModeName))
		return nil
	}

	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	oldModeName := GetModeDisplayName(cfg.Route.Mode)

	beginProgress(ctx, fmt.Sprintf("Switch to %s", newModeName))
	if !ctx.IsInteractive {
		ctx.Output.Println()
	}

	ctx.Output.Info(fmt.Sprintf("Switching from %s to %s...", oldModeName, newModeName))

	if err := r.SwitchMode(newMode); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to switch mode: %w", err))
	}

	ctx.Output.Success(fmt.Sprintf("Switched to %s!", newModeName))

	// Show next steps (different for CLI vs TUI)
	if ctx.IsInteractive {
		ctx.Output.Println()
		if newMode == "single" {
			ctx.Output.Info("Next: Select 'Switch Active' to choose a tunnel")
		} else {
			ctx.Output.Info("Next: Select 'Start/Restart' to start all tunnels")
		}
		ctx.Output.EndProgress()
	} else {
		ctx.Output.Println()
		ctx.Output.Info("Next steps:")
		if newMode == "single" {
			ctx.Output.Println("  dnstm router switch <tag>  - Select active tunnel")
			ctx.Output.Println("  dnstm router start         - Start the tunnel")
		} else {
			ctx.Output.Println("  dnstm router start   - Start all tunnels")
			ctx.Output.Println("  dnstm router status  - View status")
		}
		ctx.Output.Println()
	}

	return nil
}
