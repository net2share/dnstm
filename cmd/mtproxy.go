package cmd

import (
	"fmt"
	"strings"

	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/mtproxy"
	"github.com/net2share/dnstm/internal/tunnel"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var mtproxyCmd = &cobra.Command{
	Use:   "mtproxy",
	Short: "Manage MTProxy (Telegram proxy)",
	Long:  "Manage the MTProxy server for Telegram tunneling",
}

var mtproxyInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start MTProxy",
	Long:  "Download, install, and start the MTProxy server",
	RunE:  runMTProxyInstall,
}

var mtproxyUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall MTProxy",
	Long:  "Stop and remove the MTProxy server",
	RunE:  runMTProxyUninstall,
}

var mtproxyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show MTProxy status",
	Long:  "Display the current status of the MTProxy server",
	RunE:  runMTProxyStatus,
}

func init() {
	mtproxyCmd.AddCommand(mtproxyInstallCmd)
	mtproxyCmd.AddCommand(mtproxyUninstallCmd)
	mtproxyCmd.AddCommand(mtproxyStatusCmd)
}

func runMTProxyInstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	globalCfg, err := tunnel.LoadGlobalConfig()
	if err != nil {
		return err
	}
	domainName := ""
	if globalCfg != nil {
		switch globalCfg.ActiveProvider {
		case tunnel.ProviderDNSTT:
			provider, err := tunnel.Get(tunnel.ProviderDNSTT)
			if err != nil {
				return err
			}
			dnsttCfg, err := provider.GetConfig()
			if err != nil {
				return err
			}
			mapped := parseConf(dnsttCfg)
			domainName = mapped["NS Subdomain"]
		case tunnel.ProviderSlipstream:
			provider, err := tunnel.Get(tunnel.ProviderSlipstream)
			if err != nil {
				return err
			}
			slipstreamCfg, err := provider.GetConfig()
			if err != nil {
				return err
			}
			mapped := parseConf(slipstreamCfg)
			domainName = mapped["Domain"]
		}
	}
	secret, err := mtproxy.GenerateSecret()
	if err != nil {
		return fmt.Errorf("failed to generate secret: %w", err)
	}
	tui.PrintSuccess(fmt.Sprintf("Generated MTProxy secret: %s", secret))

	progressFn := func(downloaded, total int64) {
		if total > 0 {
			percent := float64(downloaded) / float64(total) * 100
			fmt.Printf("\rDownloading: %.1f%%", percent)
		}
	}

	if err := mtproxy.InstallMTProxy(secret, progressFn); err != nil {
		return fmt.Errorf("failed to install MTProxy: %w", err)
	}

	if err := mtproxy.ConfigureMTProxy(secret); err != nil {
		return fmt.Errorf("failed to configure MTProxy: %w", err)
	}

	proxyUrl := mtproxy.FormatProxyURL(secret, domainName)
	tui.PrintBox("MTProxy Installation Complete", []string{
		fmt.Sprintf("Secret: %s", secret),
		fmt.Sprintf("Port: %s", mtproxy.MTProxyPort),
		"",
		fmt.Sprintf("Proxy URL: %s", proxyUrl),
	})

	return nil
}

func parseConf(conf string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(conf, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}
	return result
}
func runMTProxyUninstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	fmt.Println()

	if !mtproxy.IsMtProxyInstalled() {
		tui.PrintInfo("MTProxy is not installed")
		return nil
	}

	tui.PrintInfo("Removing MTProxy...")
	if err := mtproxy.UninstallMTProxy(); err != nil {
		return fmt.Errorf("failed to uninstall MTProxy: %w", err)
	}

	tui.PrintSuccess("MTProxy removed!")
	return nil
}

func runMTProxyStatus(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	fmt.Println()

	installed := mtproxy.IsMtProxyInstalled()
	running := mtproxy.IsMTProxyRunning()

	lines := []string{
		tui.KV("Installed: ", boolToYesNo(installed)),
		tui.KV("Running:   ", boolToYesNo(running)),
	}

	if installed {
		lines = append(lines, tui.KV("Port:      ", mtproxy.MTProxyPort))
		lines = append(lines, tui.KV("Stats Port:", mtproxy.MTProxyStatsPort))
	}

	tui.PrintBox("MTProxy Status", lines)

	return nil
}
