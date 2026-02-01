package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
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

	// Build info config for TUI
	infoCfg := actions.InfoConfig{
		Title: "Router Status",
	}

	modeName := GetModeDisplayName(cfg.Route.Mode)

	if cfg.IsSingleMode() {
		// Single-tunnel mode status
		mainSection := actions.InfoSection{
			Rows: []actions.InfoRow{
				{Key: "Mode", Value: modeName},
			},
		}

		if cfg.Route.Active != "" {
			tunnel := r.GetTunnel(cfg.Route.Active)
			if tunnel != nil {
				status := actions.SymbolStopped + " Stopped"
				if tunnel.IsActive() {
					status = actions.SymbolRunning + " Running"
				}
				transportName := config.GetTransportTypeDisplayName(tunnel.Transport)
				mainSection.Rows = append(mainSection.Rows, actions.InfoRow{
					Key: "Active", Value: fmt.Sprintf("%s (%s) %s", cfg.Route.Active, transportName, status),
				})
				mainSection.Rows = append(mainSection.Rows, actions.InfoRow{
					Value: fmt.Sprintf("  %s %s %s 127.0.0.1:%d", actions.SymbolBranch, tunnel.Domain, actions.SymbolArrow, tunnel.Port),
				})
			}
		} else {
			mainSection.Rows = append(mainSection.Rows, actions.InfoRow{Key: "Active", Value: "(none)"})
		}
		infoCfg.Sections = append(infoCfg.Sections, mainSection)

		// Show other tunnels
		if len(cfg.Tunnels) > 1 {
			otherSection := actions.InfoSection{
				Title: "Other Tunnels",
			}
			for _, t := range cfg.Tunnels {
				if t.Tag == cfg.Route.Active {
					continue
				}
				transportName := config.GetTransportTypeDisplayName(t.Transport)
				otherSection.Rows = append(otherSection.Rows, actions.InfoRow{
					Value: fmt.Sprintf("• %-16s %s", t.Tag, transportName),
				})
			}
			infoCfg.Sections = append(infoCfg.Sections, otherSection)
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

		mainSection := actions.InfoSection{
			Rows: []actions.InfoRow{
				{Key: "Mode", Value: modeName},
				{Key: "DNS Router", Value: fmt.Sprintf("%s (port 53)", routerStatus)},
			},
		}
		infoCfg.Sections = append(infoCfg.Sections, mainSection)

		// Tunnels section
		tunnelSection := actions.InfoSection{
			Title: "Tunnels",
		}

		tunnels := r.GetAllTunnels()
		if len(tunnels) == 0 {
			tunnelSection.Rows = []actions.InfoRow{{Value: "No tunnels configured"}}
		} else {
			for tag, tunnel := range tunnels {
				status := actions.SymbolStopped + " Stopped"
				if tunnel.IsActive() {
					status = actions.SymbolRunning + " Running"
				}
				if !tunnel.IsInstalled() {
					status = actions.SymbolError + " Not installed"
				}

				transportName := config.GetTransportTypeDisplayName(tunnel.Transport)
				defaultMarker := ""
				if cfg.Route.Default == tag {
					defaultMarker = " (default)"
				}
				tunnelSection.Rows = append(tunnelSection.Rows, actions.InfoRow{
					Value: fmt.Sprintf("%-16s %-12s %s%s", tag, transportName, status, defaultMarker),
				})
				tunnelSection.Rows = append(tunnelSection.Rows, actions.InfoRow{
					Value: fmt.Sprintf("  %s %s %s 127.0.0.1:%d", actions.SymbolBranch, tunnel.Domain, actions.SymbolArrow, tunnel.Port),
				})
			}
		}
		infoCfg.Sections = append(infoCfg.Sections, tunnelSection)
	}

	// Display using TUI in interactive mode
	if ctx.IsInteractive {
		return ctx.Output.ShowInfo(infoCfg)
	}

	// CLI mode - print to console (original logic)
	ctx.Output.Println()

	var lines []string
	lines = append(lines, fmt.Sprintf("Mode: %s", modeName))

	if cfg.IsSingleMode() {
		lines = append(lines, "")
		if cfg.Route.Active != "" {
			tunnel := r.GetTunnel(cfg.Route.Active)
			if tunnel != nil {
				status := actions.SymbolStopped + " Stopped"
				if tunnel.IsActive() {
					status = actions.SymbolRunning + " Running"
				}
				transportName := config.GetTransportTypeDisplayName(tunnel.Transport)
				lines = append(lines, fmt.Sprintf("Active: %s (%s) %s", cfg.Route.Active, transportName, status))
				lines = append(lines, fmt.Sprintf("  %s %s %s 127.0.0.1:%d", actions.SymbolBranch, tunnel.Domain, actions.SymbolArrow, tunnel.Port))
			}
		} else {
			lines = append(lines, "Active: (none)")
		}

		if len(cfg.Tunnels) > 1 {
			lines = append(lines, "")
			lines = append(lines, "Other tunnels:")
			for _, t := range cfg.Tunnels {
				if t.Tag == cfg.Route.Active {
					continue
				}
				transportName := config.GetTransportTypeDisplayName(t.Transport)
				lines = append(lines, fmt.Sprintf("  %-16s %s", t.Tag, transportName))
			}
		}
	} else {
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
		lines = append(lines, "Tunnels:")

		tunnels := r.GetAllTunnels()
		if len(tunnels) == 0 {
			lines = append(lines, "  No tunnels configured")
		} else {
			for tag, tunnel := range tunnels {
				status := actions.SymbolStopped + " Stopped"
				if tunnel.IsActive() {
					status = actions.SymbolRunning + " Running"
				}
				if !tunnel.IsInstalled() {
					status = actions.SymbolError + " Not installed"
				}

				transportName := config.GetTransportTypeDisplayName(tunnel.Transport)
				defaultMarker := ""
				if cfg.Route.Default == tag {
					defaultMarker = " (default)"
				}
				lines = append(lines, fmt.Sprintf("  %-16s %-24s %s%s", tag, transportName, status, defaultMarker))
				lines = append(lines, fmt.Sprintf("    %s %s %s 127.0.0.1:%d", actions.SymbolBranch, tunnel.Domain, actions.SymbolArrow, tunnel.Port))
			}
		}
	}

	ctx.Output.Box("Router Status", lines)
	ctx.Output.Println()

	return nil
}
