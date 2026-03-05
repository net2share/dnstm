package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/monitor"
	"github.com/net2share/go-corelib/tui"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelMetrics, HandleTunnelMetrics)
}

// HandleTunnelMetrics enables or disables the Prometheus metrics endpoint for a tunnel.
func HandleTunnelMetrics(ctx *actions.Context) error {
	_, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	tag, err := RequireTag(ctx, "tunnel")
	if err != nil {
		return err
	}

	tc, err := GetTunnelByTag(ctx, tag)
	if err != nil {
		return err
	}

	if tc.Transport != config.TransportDNSTT {
		return fmt.Errorf("metrics monitoring is only available for dnstt tunnels")
	}

	currentAddr := monitor.ReadMetricsConf(tag)

	if ctx.IsInteractive {
		return handleMetricsInteractive(ctx, tag, tc, currentAddr)
	}
	return handleMetricsCLI(ctx, tag, tc, currentAddr)
}

func handleMetricsCLI(ctx *actions.Context, tag string, tc *config.TunnelConfig, currentAddr string) error {
	enable := ctx.GetBool("enable")
	address := ctx.GetString("address")

	if currentAddr != "" {
		ctx.Output.Printf("Metrics currently enabled on %s/metrics\n", currentAddr)
	} else {
		ctx.Output.Printf("Metrics currently disabled\n")
	}

	if !enable {
		// Disable metrics
		if currentAddr == "" {
			ctx.Output.Printf("Already disabled, nothing to do.\n")
			return nil
		}
		return restartSniffer(ctx, tag, tc, "")
	}

	// Enable metrics
	return restartSniffer(ctx, tag, tc, address)
}

func handleMetricsInteractive(ctx *actions.Context, tag string, tc *config.TunnelConfig, currentAddr string) error {
	// Show current status
	if currentAddr != "" {
		ctx.Output.Printf("Metrics currently enabled on %s/metrics\n\n", currentAddr)
	} else {
		ctx.Output.Printf("Metrics currently disabled\n\n")
	}

	// Choose enable or disable
	options := []tui.MenuOption{
		{Label: "Enable", Value: "enable"},
		{Label: "Disable", Value: "disable"},
		{Label: "Back", Value: "back"},
	}

	choice, err := tui.RunMenu(tui.MenuConfig{
		Title:       "Prometheus Metrics",
		Description: fmt.Sprintf("Tunnel: %s (%s)", tag, tc.Domain),
		Options:     options,
	})
	if err != nil || choice == "" || choice == "back" {
		return nil
	}

	if choice == "disable" {
		if currentAddr == "" {
			ctx.Output.Printf("Already disabled.\n")
			return nil
		}
		return restartSniffer(ctx, tag, tc, "")
	}

	// Enable — prompt for address
	defaultAddr := currentAddr
	if defaultAddr == "" {
		defaultAddr = ":9100"
	}

	address, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Metrics Address",
		Description: "Address to serve Prometheus metrics on",
		Placeholder: ":9100",
		Value:       defaultAddr,
	})
	if err != nil || !confirmed {
		return nil
	}
	if address == "" {
		address = ":9100"
	}

	return restartSniffer(ctx, tag, tc, address)
}

func restartSniffer(ctx *actions.Context, tag string, tc *config.TunnelConfig, metricsAddr string) error {
	// Stop existing sniffer
	if monitor.IsSnifferRunning(tag) {
		ctx.Output.Printf("Stopping monitor...\n")
		if err := monitor.StopSniffer(tag); err != nil {
			return fmt.Errorf("failed to stop sniffer: %w", err)
		}
	}

	// Update persisted config
	if err := monitor.WriteMetricsConf(tag, metricsAddr); err != nil {
		return fmt.Errorf("failed to write metrics config: %w", err)
	}

	// Start with new config
	ctx.Output.Printf("Starting monitor...\n")
	if err := monitor.StartSniffer(tag, []string{tc.Domain}, metricsAddr); err != nil {
		return fmt.Errorf("failed to start sniffer: %w", err)
	}

	if metricsAddr != "" {
		ctx.Output.Success(fmt.Sprintf("Metrics enabled on %s/metrics", metricsAddr))
	} else {
		ctx.Output.Success("Metrics disabled")
	}
	return nil
}
