package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
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
	newMode := router.Mode(modeStr)
	if newMode != router.ModeSingle && newMode != router.ModeMulti {
		return actions.NewActionError(
			fmt.Sprintf("invalid mode '%s'", modeStr),
			"Use 'single' or 'multi'",
		)
	}

	return switchMode(ctx, cfg, newMode)
}

func showCurrentMode(ctx *actions.Context, cfg *router.Config) error {
	ctx.Output.Println()

	modeName := router.GetModeDisplayName(cfg.Mode)
	var lines []string
	lines = append(lines, fmt.Sprintf("Mode: %s", modeName))

	if cfg.IsSingleMode() {
		if cfg.Single.Active != "" {
			lines = append(lines, fmt.Sprintf("Active instance: %s", cfg.Single.Active))
			if transport, ok := cfg.Transports[cfg.Single.Active]; ok {
				lines = append(lines, fmt.Sprintf("Domain: %s", transport.Domain))
			}
		} else {
			lines = append(lines, "Active instance: (none)")
		}
		lines = append(lines, "")
		lines = append(lines, "Use 'dnstm router switch <instance>' to change active tunnel")
	} else {
		lines = append(lines, fmt.Sprintf("Instances: %d", len(cfg.Transports)))
		if cfg.Routing.Default != "" {
			lines = append(lines, fmt.Sprintf("Default route: %s", cfg.Routing.Default))
		}
	}

	ctx.Output.Box("Operating Mode", lines)
	ctx.Output.Println()

	return nil
}

func switchMode(ctx *actions.Context, cfg *router.Config, newMode router.Mode) error {
	if cfg.Mode == newMode {
		modeName := router.GetModeDisplayName(newMode)
		ctx.Output.Info(fmt.Sprintf("Already in %s mode", modeName))
		return nil
	}

	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	ctx.Output.Println()

	newModeName := router.GetModeDisplayName(newMode)
	oldModeName := router.GetModeDisplayName(cfg.Mode)
	ctx.Output.Info(fmt.Sprintf("Switching from %s to %s mode...", oldModeName, newModeName))
	ctx.Output.Println()

	if err := r.SwitchMode(newMode); err != nil {
		return fmt.Errorf("failed to switch mode: %w", err)
	}

	ctx.Output.Println()
	ctx.Output.Success(fmt.Sprintf("Switched to %s mode!", newModeName))
	ctx.Output.Println()

	// Show next steps
	if newMode == router.ModeSingle {
		ctx.Output.Info("Commands for single-tunnel mode:")
		ctx.Output.Println("  dnstm router switch <instance>  - Switch active tunnel")
		ctx.Output.Println("  dnstm router start              - Start active tunnel")
		ctx.Output.Println("  dnstm router stop               - Stop active tunnel")
	} else {
		ctx.Output.Info("Commands for multi-tunnel mode:")
		ctx.Output.Println("  dnstm router status      - Show router and instance status")
		ctx.Output.Println("  dnstm router start       - Start router and all instances")
		ctx.Output.Println("  dnstm router stop        - Stop router and all instances")
	}
	ctx.Output.Println()

	return nil
}
