package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/types"
)

func init() {
	actions.SetRouterHandler(actions.ActionRouterStatus, HandleRouterStatus)
}

// HandleRouterStatus shows the router status.
func HandleRouterStatus(ctx *actions.Context) error {
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

	// Build status output
	var lines []string
	modeName := router.GetModeDisplayName(cfg.Mode)
	lines = append(lines, fmt.Sprintf("Mode: %s", modeName))

	if cfg.IsSingleMode() {
		// Single-tunnel mode status
		lines = append(lines, "")
		if cfg.Single.Active != "" {
			instance := r.GetInstance(cfg.Single.Active)
			if instance != nil {
				status := actions.SymbolStopped + " Stopped"
				if instance.IsActive() {
					status = actions.SymbolRunning + " Running"
				}
				typeName := types.GetTransportTypeDisplayName(instance.Type)
				lines = append(lines, fmt.Sprintf("Active: %s (%s) %s", cfg.Single.Active, typeName, status))
				lines = append(lines, fmt.Sprintf("  %s %s %s 127.0.0.1:%d", actions.SymbolBranch, instance.Domain, actions.SymbolArrow, instance.Port))
			}
		} else {
			lines = append(lines, "Active: (none)")
		}

		// Show other instances
		if len(cfg.Transports) > 1 {
			lines = append(lines, "")
			lines = append(lines, "Other instances:")
			for name, transport := range cfg.Transports {
				if name == cfg.Single.Active {
					continue
				}
				typeName := types.GetTransportTypeDisplayName(transport.Type)
				lines = append(lines, fmt.Sprintf("  %-16s %s", name, typeName))
			}
		}
	} else {
		// Multi-tunnel mode status
		svc := r.GetDNSRouterService()
		routerStatus := actions.SymbolStopped + " Stopped"
		if svc.IsActive() {
			routerStatus = actions.SymbolRunning + " Running"
		}
		if !svc.IsServiceInstalled() {
			routerStatus = actions.SymbolError + " Not installed"
		}
		lines = append(lines, fmt.Sprintf("DNS Router: %s (port 53)", routerStatus))
		lines = append(lines, "")
		lines = append(lines, "Instances:")

		instances := r.GetAllInstances()
		if len(instances) == 0 {
			lines = append(lines, "  No instances configured")
		} else {
			for name, instance := range instances {
				status := actions.SymbolStopped + " Stopped"
				if instance.IsActive() {
					status = actions.SymbolRunning + " Running"
				}
				if !instance.IsInstalled() {
					status = actions.SymbolError + " Not installed"
				}

				typeName := types.GetTransportTypeDisplayName(instance.Type)
				defaultMarker := ""
				if cfg.Routing.Default == name {
					defaultMarker = " (default)"
				}
				lines = append(lines, fmt.Sprintf("  %-16s %-24s %s%s", name, typeName, status, defaultMarker))
				lines = append(lines, fmt.Sprintf("    %s %s %s 127.0.0.1:%d", actions.SymbolBranch, instance.Domain, actions.SymbolArrow, instance.Port))
			}
		}
	}

	ctx.Output.Box("Router Status", lines)
	ctx.Output.Println()

	return nil
}
