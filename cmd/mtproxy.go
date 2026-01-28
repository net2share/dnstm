package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/mtproxy"
	"github.com/net2share/dnstm/internal/tunnel"
	"github.com/net2share/dnstm/internal/tunnel/dnstt"
	"github.com/net2share/dnstm/internal/tunnel/slipstream"
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

	domainName, existingSecret := getProviderInfo()

	var secret string
	if existingSecret != "" {
		secret = existingSecret
		tui.PrintSuccess(fmt.Sprintf("Using existing MTProxy secret: %s", secret))
	} else {
		var err error
		secret, err = mtproxy.GenerateSecret()
		if err != nil {
			return fmt.Errorf("failed to generate secret: %w", err)
		}
		tui.PrintSuccess(fmt.Sprintf("Generated new MTProxy secret: %s", secret))
		if err := setConfigSecret(secret); err != nil {
			return fmt.Errorf("failed to set MTProxy secret in provider config: %w", err)
		}
	}

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

func getProviderInfo() (domain, secret string) {
	globalCfg, err := tunnel.LoadGlobalConfig()
	if err != nil || globalCfg == nil {
		return "", ""
	}

	switch globalCfg.ActiveProvider {
	case tunnel.ProviderDNSTT:
		cfg, err := dnstt.Load()
		if err != nil {
			return "", ""
		}
		domain = cfg.NSSubdomain
		secret = cfg.MTProxySecret

	case tunnel.ProviderSlipstream:
		cfg, err := slipstream.Load()
		if err != nil {
			return "", ""
		}
		domain = cfg.Domain
		secret = cfg.MTProxySecret
	default:
		return "", ""
	}

	return domain, secret
}

func setConfigSecret(secret string) error {
	globalCfg, err := tunnel.LoadGlobalConfig()
	if err != nil || globalCfg == nil {
		return fmt.Errorf("failed to load global config")
	}

	switch globalCfg.ActiveProvider {
	case tunnel.ProviderDNSTT:
		cfg, err := dnstt.Load()
		if err != nil {
			return fmt.Errorf("failed to load DNSTT config: %w", err)
		}
		cfg.MTProxySecret = secret
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save DNSTT config: %w", err)
		}

	case tunnel.ProviderSlipstream:
		cfg, err := slipstream.Load()
		if err != nil {
			return fmt.Errorf("failed to load Slipstream config: %w", err)
		}
		cfg.MTProxySecret = secret
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save Slipstream config: %w", err)
		}
	default:
		return fmt.Errorf("no active provider to set secret for")
	}

	return nil
}

func runMTProxyUninstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	fmt.Println()

	if !mtproxy.IsMTProxyInstalled() {
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

	installed := mtproxy.IsMTProxyInstalled()
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
