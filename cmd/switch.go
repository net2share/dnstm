package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/tunnel"
	_ "github.com/net2share/dnstm/internal/tunnel/dnstt"
	_ "github.com/net2share/dnstm/internal/tunnel/shadowsocks"
	_ "github.com/net2share/dnstm/internal/tunnel/slipstream"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var switchCmd = &cobra.Command{
	Use:   "switch <provider>",
	Short: "Switch active DNS tunnel provider",
	Long:  "Switch the active DNS handler to a different provider (dnstt, slipstream, or shadowsocks)",
	Args:  cobra.ExactArgs(1),
	RunE:  runSwitch,
}

func runSwitch(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	// Parse provider type
	pt, err := tunnel.ParseProviderType(args[0])
	if err != nil {
		return err
	}

	provider, err := tunnel.Get(pt)
	if err != nil {
		return err
	}

	if !provider.IsInstalled() {
		return fmt.Errorf("%s is not installed", provider.DisplayName())
	}

	// Check if already active
	globalCfg, _ := tunnel.LoadGlobalConfig()
	if globalCfg != nil && globalCfg.ActiveProvider == pt {
		tui.PrintInfo(fmt.Sprintf("%s is already the active DNS handler", provider.DisplayName()))
		return nil
	}

	tui.PrintInfo(fmt.Sprintf("Switching DNS routing to %s...", provider.DisplayName()))

	// Get current active provider
	var currentProvider tunnel.Provider
	var currentPort string
	if globalCfg != nil {
		currentProvider, _ = tunnel.Get(globalCfg.ActiveProvider)
		if currentProvider != nil {
			currentPort = currentProvider.Port()
		}
	}

	newPort := provider.Port()

	// Stop current provider if running
	if currentProvider != nil {
		status, _ := currentProvider.Status()
		if status != nil && status.Running {
			tui.PrintInfo(fmt.Sprintf("Stopping %s...", currentProvider.DisplayName()))
			currentProvider.Stop()
		}
	}

	// Switch firewall rules
	if currentPort != "" && currentPort != newPort {
		tui.PrintInfo("Switching firewall rules...")
		if err := network.SwitchDNSRouting(currentPort, newPort); err != nil {
			tui.PrintWarning("Firewall switch warning: " + err.Error())
		}
	}

	// Update global config
	if err := tunnel.SetActiveProvider(pt); err != nil {
		return fmt.Errorf("failed to update active provider: %w", err)
	}

	// Start new provider
	tui.PrintInfo(fmt.Sprintf("Starting %s...", provider.DisplayName()))
	if err := provider.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	tui.PrintSuccess(fmt.Sprintf("%s is now the active DNS handler!", provider.DisplayName()))
	return nil
}
