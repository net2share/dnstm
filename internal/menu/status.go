package menu

import (
	"fmt"
	"strings"

	"github.com/net2share/dnstm/internal/mtproxy"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/sshtunnel"
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
	lines = append(lines, tui.Header("Tunnel Providers:"))
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

	// Add SSH tunnel hardening status
	lines = append(lines, "")
	lines = append(lines, tui.Header("SSH Tunnel Hardening:"))
	lines = append(lines, "")
	lines = append(lines, buildSSHTunnelStatusString())

	// Add microsocks status
	lines = append(lines, "")
	lines = append(lines, tui.Header("SOCKS Proxy:"))
	lines = append(lines, "")
	lines = append(lines, buildMicrosocksStatusString())

	// Add MTProxy status
	lines = append(lines, "")
	lines = append(lines, tui.Header("MTProxy (Telegram):"))
	lines = append(lines, "")
	lines = append(lines, buildMTProxyStatusString())

	tui.PrintBox("Status", lines)
}

func buildSSHTunnelStatusString() string {
	boldName := bold + "Hardening:" + reset

	sshStatus := sshtunnel.GetStatus()
	if !sshStatus.Configured {
		return fmt.Sprintf("  %s Not configured", boldName)
	}

	var states []string
	states = append(states, green+"Configured"+reset)

	if sshStatus.UserCount == 0 {
		states = append(states, "No users")
	} else {
		userStr := fmt.Sprintf("%d user(s)", sshStatus.UserCount)
		if sshStatus.PasswordAuthCount > 0 && sshStatus.KeyAuthCount > 0 {
			userStr += fmt.Sprintf(" (%d password, %d key)", sshStatus.PasswordAuthCount, sshStatus.KeyAuthCount)
		} else if sshStatus.PasswordAuthCount > 0 {
			userStr += " (password auth)"
		} else if sshStatus.KeyAuthCount > 0 {
			userStr += " (key auth)"
		}
		states = append(states, userStr)
	}

	return fmt.Sprintf("  %s %s", boldName, strings.Join(states, ", "))
}

func buildMicrosocksStatusString() string {
	boldName := bold + "Microsocks:" + reset

	if !proxy.IsMicrosocksInstalled() {
		return fmt.Sprintf("  %s Not installed", boldName)
	}

	var states []string
	states = append(states, "Installed")

	if proxy.IsMicrosocksRunning() {
		states = append(states, green+"Running"+reset)
	} else {
		states = append(states, yellow+"Stopped"+reset)
	}

	return fmt.Sprintf("  %s %s", boldName, strings.Join(states, ", "))
}

func buildMTProxyStatusString() string {
	boldName := bold + "MTProxy:" + reset

	if !mtproxy.IsMTProxyInstalled() {
		return fmt.Sprintf("  %s Not installed", boldName)
	}

	var states []string
	states = append(states, "Installed")

	if mtproxy.IsMTProxyRunning() {
		states = append(states, green+"Running"+reset)
	} else {
		states = append(states, yellow+"Stopped"+reset)
	}

	return fmt.Sprintf("  %s %s", boldName, strings.Join(states, ", "))
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
