package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/livestats"
	"github.com/net2share/dnstm/internal/monitor"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/service"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelStats, HandleTunnelStats)
}

// HandleTunnelStats shows tunnel usage stats from the background sniffer.
func HandleTunnelStats(ctx *actions.Context) error {
	cfg, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	tag := ctx.GetString("tag")

	var tunnelCfgs []config.TunnelConfig
	var tunnels []*router.Tunnel
	if tag != "" {
		tc, err := GetTunnelByTag(ctx, tag)
		if err != nil {
			return err
		}
		tunnelCfgs = []config.TunnelConfig{*tc}
		tunnels = []*router.Tunnel{router.NewTunnel(tc)}
	} else {
		if len(cfg.Tunnels) == 0 {
			return actions.NoTunnelsError()
		}
		tunnelCfgs = cfg.Tunnels
		for i := range cfg.Tunnels {
			tunnels = append(tunnels, router.NewTunnel(&cfg.Tunnels[i]))
		}
	}

	// Auto-setup: if any dnstt tunnel doesn't have a running sniffer, start one
	snifferJustStarted := false
	for i, t := range tunnels {
		tc := tunnelCfgs[i]
		if tc.Transport != config.TransportDNSTT {
			continue
		}
		if !monitor.IsSnifferRunning(t.Tag) {
			ctx.Output.Printf("Starting monitor for %s...\n", t.Tag)
			if err := monitor.StartSniffer(t.Tag, []string{tc.Domain}, monitor.ReadMetricsConf(t.Tag)); err != nil {
				ctx.Output.Printf("  Warning: failed to start sniffer: %v\n", err)
			} else {
				snifferJustStarted = true
			}
		}
	}

	// If we just started a sniffer in CLI mode, give it a moment to collect data
	if snifferJustStarted && !ctx.IsInteractive {
		ctx.Output.Printf("Waiting for data...\n")
		time.Sleep(3 * time.Second)
	}

	if ctx.IsInteractive {
		return showStatsInteractive(ctx, tunnels)
	}
	return showStatsCLI(ctx, tunnels)
}

func showStatsInteractive(ctx *actions.Context, tunnels []*router.Tunnel) error {
	return livestats.Run(tunnels)
}

func showStatsCLI(ctx *actions.Context, tunnels []*router.Tunnel) error {
	ctx.Output.Println()

	for _, t := range tunnels {
		printTunnelCLI(ctx, t)
	}

	return nil
}

func printTunnelCLI(ctx *actions.Context, t *router.Tunnel) {
	active := service.IsServiceActive(t.ServiceName)
	status := "Stopped"
	if active {
		status = "Running"
	}

	ctx.Output.Printf("--- %s [%s] ---\n", t.Tag, status)
	ctx.Output.Printf("  Domain:            %s\n", t.Domain)

	result, err := monitor.ReadStats(t.Tag)
	if err != nil {
		ctx.Output.Printf("  Stats:             Error reading stats: %v\n\n", err)
		return
	}
	if result == nil {
		if !monitor.IsSnifferRunning(t.Tag) {
			ctx.Output.Printf("  Monitor:           Not running\n")
			ctx.Output.Printf("  Hint:              Run 'dnstm tunnel stats -t %s' to start it\n\n", t.Tag)
		} else {
			ctx.Output.Printf("  Stats:             Waiting for data...\n\n")
		}
		return
	}

	tr := findTunnelResult(t.Domain, result)
	if tr == nil || tr.TotalQueries == 0 {
		ctx.Output.Printf("  (no traffic yet)\n\n")
		return
	}

	ctx.Output.Printf("  Uptime:            %s\n", result.Duration.Round(1e9)) // round to seconds
	ctx.Output.Printf("  Queries:           %d (%.1f/sec)\n", tr.TotalQueries, tr.QueriesPerSec)
	ctx.Output.Printf("  Bandwidth In:      %s\n", monitor.FormatBytes(tr.TotalBytesIn))
	ctx.Output.Printf("  Bandwidth Out:     %s\n", monitor.FormatBytes(tr.TotalBytesOut))
	ctx.Output.Printf("  Connected Users:   %d\n", tr.ActiveClients)
	ctx.Output.Printf("  Peak Concurrent:   %d\n", tr.PeakClients)
	if tr.TotalClients > tr.ActiveClients {
		ctx.Output.Printf("  Total Sessions:    %d\n", tr.TotalClients)
	}

	if s := tr.Summary(); s.Count > 0 {
		ctx.Output.Println()
		ctx.Output.Println("  Per-User Traffic:")
		ctx.Output.Printf("    Min:             %s (%d queries)\n", monitor.FormatBytes(s.MinBytesTotal), s.MinQueries)
		ctx.Output.Printf("    Median:          %s (%d queries)\n", monitor.FormatBytes(s.MedianBytes), s.MedianQueries)
		ctx.Output.Printf("    Max:             %s (%d queries)\n", monitor.FormatBytes(s.MaxBytesTotal), s.MaxQueries)
		ctx.Output.Println()
		ctx.Output.Println("  Session Length:")
		ctx.Output.Printf("    Min:             %s\n", formatDuration(s.MinDuration))
		ctx.Output.Printf("    Median:          %s\n", formatDuration(s.MedianDuration))
		ctx.Output.Printf("    Max:             %s\n", formatDuration(s.MaxDuration))
	}

	if len(tr.Clients) > 0 {
		ctx.Output.Println()
		ctx.Output.Println("  Users (by traffic):")
		limit := len(tr.Clients)
		if limit > 15 {
			limit = 15
		}
		for _, c := range tr.Clients[:limit] {
			marker := " "
			if !c.Active {
				marker = "-"
			}
			ctx.Output.Printf("   %s %s  %s (%d queries)\n", marker, c.ClientID, monitor.FormatBytes(c.BytesTotal), c.Queries)
		}
		if len(tr.Clients) > 15 {
			ctx.Output.Printf("    ... and %d more\n", len(tr.Clients)-15)
		}
	}
	ctx.Output.Println()
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func findTunnelResult(domain string, result *monitor.CaptureResult) *monitor.TunnelResult {
	if result == nil || result.Tunnels == nil {
		return nil
	}
	domain = strings.ToLower(domain)

	if tr, ok := result.Tunnels[domain]; ok {
		return tr
	}
	for d, tr := range result.Tunnels {
		if strings.HasSuffix(domain, "."+d) || strings.HasSuffix(d, "."+domain) {
			return tr
		}
	}
	return nil
}
