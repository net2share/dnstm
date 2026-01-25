package menu

import (
	"fmt"
	"strings"

	"github.com/net2share/dnstm/internal/tunnel"
	_ "github.com/net2share/dnstm/internal/tunnel/dnstt"
	_ "github.com/net2share/dnstm/internal/tunnel/slipstream"
	"github.com/net2share/go-corelib/tui"
)

// ANSI escape codes for styling
const (
	bold      = "\033[1m"
	reset     = "\033[0m"
	green     = "\033[32m"
	cyan      = "\033[36m"
	yellow    = "\033[33m"
	boldGreen = "\033[1;32m"
)

// ShowOverallStatus displays the combined status of all providers.
func ShowOverallStatus() {
	fmt.Println()

	globalCfg, _ := tunnel.LoadGlobalConfig()
	activeProvider := tunnel.ProviderType("")
	if globalCfg != nil {
		activeProvider = globalCfg.ActiveProvider
	}

	var lines []string
	lines = append(lines, tui.Header("Provider Status:"))
	lines = append(lines, "")

	for _, pt := range tunnel.Types() {
		provider, err := tunnel.Get(pt)
		if err != nil {
			continue
		}

		status, _ := provider.Status()
		statusStr := buildStatusString(status, provider.DisplayName(), activeProvider == pt)
		lines = append(lines, statusStr)
	}

	tui.PrintBox("Overall Status", lines)
}

// GetStatusLine returns a brief status line for the main menu header.
func GetStatusLine() string {
	globalCfg, _ := tunnel.LoadGlobalConfig()
	activeProvider := tunnel.ProviderType("")
	if globalCfg != nil {
		activeProvider = globalCfg.ActiveProvider
	}

	var parts []string

	for _, pt := range tunnel.Types() {
		provider, err := tunnel.Get(pt)
		if err != nil {
			continue
		}

		status, _ := provider.Status()
		if status == nil {
			continue
		}

		var partStr string
		if status.Installed {
			if activeProvider == pt && status.Running {
				partStr = fmt.Sprintf("%s active", provider.DisplayName())
			} else if status.Running {
				partStr = fmt.Sprintf("%s running (not active)", provider.DisplayName())
			} else {
				partStr = fmt.Sprintf("%s installed (stopped)", provider.DisplayName())
			}
		} else {
			partStr = fmt.Sprintf("%s not installed", provider.DisplayName())
		}
		parts = append(parts, partStr)
	}

	if len(parts) == 0 {
		return "No providers available"
	}

	return strings.Join(parts, ", ")
}

func buildStatusString(status *tunnel.ProviderStatus, displayName string, isActive bool) string {
	// Make provider name bold
	boldName := bold + displayName + ":" + reset

	if status == nil {
		return fmt.Sprintf("  %s Unknown", boldName)
	}

	var states []string

	if status.Installed {
		states = append(states, "Installed")
	} else {
		return fmt.Sprintf("  %s Not installed", boldName)
	}

	if status.Running {
		states = append(states, "Running")
	} else {
		states = append(states, "Stopped")
	}

	if isActive {
		// Make "Active DNS Handler" green
		states = append(states, boldGreen+"Active DNS Handler"+reset)
	}

	return fmt.Sprintf("  %s %s", boldName, strings.Join(states, ", "))
}
