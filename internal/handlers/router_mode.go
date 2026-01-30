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
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// No args - show current mode
	if !ctx.HasArg(0) {
		return showCurrentMode(ctx, cfg)
	}

	// Switch mode
	modeStr := ctx.GetArg(0)
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
	if cfg.Route.Mode == newMode {
		modeName := GetModeDisplayName(newMode)
		ctx.Output.Info(fmt.Sprintf("Already in %s mode", modeName))
		return nil
	}

	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	ctx.Output.Println()

	newModeName := GetModeDisplayName(newMode)
	oldModeName := GetModeDisplayName(cfg.Route.Mode)
	ctx.Output.Info(fmt.Sprintf("Switching from %s to %s mode...", oldModeName, newModeName))
	ctx.Output.Println()

	if err := r.SwitchMode(newMode); err != nil {
		return fmt.Errorf("failed to switch mode: %w", err)
	}

	ctx.Output.Println()
	ctx.Output.Success(fmt.Sprintf("Switched to %s mode!", newModeName))
	ctx.Output.Println()

	// Show next steps
	if newMode == "single" {
		ctx.Output.Info("Commands for single-tunnel mode:")
		ctx.Output.Println("  dnstm router switch <tag>  - Switch active tunnel")
		ctx.Output.Println("  dnstm router start         - Start active tunnel")
		ctx.Output.Println("  dnstm router stop          - Stop active tunnel")
	} else {
		ctx.Output.Info("Commands for multi-tunnel mode:")
		ctx.Output.Println("  dnstm router status      - Show router and tunnel status")
		ctx.Output.Println("  dnstm router start       - Start router and all tunnels")
		ctx.Output.Println("  dnstm router stop        - Stop router and all tunnels")
	}
	ctx.Output.Println()

	return nil
}
