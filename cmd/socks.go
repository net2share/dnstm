package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var socksCmd = &cobra.Command{
	Use:   "socks",
	Short: "Manage SOCKS proxy (microsocks)",
	Long:  "Manage the microsocks SOCKS5 proxy for DNS tunneling",
}

var socksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start SOCKS proxy",
	Long:  "Download, install, and start the microsocks SOCKS5 proxy",
	RunE:  runSOCKSInstall,
}

var socksUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Stop and remove SOCKS proxy",
	Long:  "Stop and completely remove the microsocks SOCKS5 proxy",
	RunE:  runSOCKSUninstall,
}

var socksStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SOCKS proxy status",
	Long:  "Display the current status of the microsocks SOCKS5 proxy",
	RunE:  runSOCKSStatus,
}

func init() {
	socksCmd.AddCommand(socksInstallCmd)
	socksCmd.AddCommand(socksUninstallCmd)
	socksCmd.AddCommand(socksStatusCmd)
}

func runSOCKSInstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	fmt.Println()

	isReinstall := proxy.IsMicrosocksInstalled()

	tui.PrintStep(1, 3, "Downloading microsocks...")
	if err := proxy.InstallMicrosocks(tui.PrintProgress); err != nil {
		return fmt.Errorf("failed to install microsocks: %w", err)
	}
	tui.ClearLine()
	tui.PrintStatus("microsocks downloaded")

	tui.PrintStep(2, 3, "Configuring microsocks service...")
	if err := proxy.ConfigureMicrosocks(); err != nil {
		return fmt.Errorf("failed to configure microsocks: %w", err)
	}
	tui.PrintStatus("microsocks configured")

	tui.PrintStep(3, 3, "Starting microsocks service...")
	if err := proxy.StartMicrosocks(); err != nil {
		return fmt.Errorf("failed to start microsocks: %w", err)
	}
	tui.PrintStatus("microsocks started")

	fmt.Println()
	if isReinstall {
		tui.PrintSuccess("SOCKS proxy reinstalled and running!")
	} else {
		tui.PrintSuccess("SOCKS proxy installed and running!")
	}
	tui.PrintInfo(fmt.Sprintf("Listening on %s:%s", proxy.MicrosocksBindAddr, proxy.MicrosocksPort))

	return nil
}

func runSOCKSUninstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	fmt.Println()

	if !proxy.IsMicrosocksInstalled() {
		tui.PrintInfo("microsocks is not installed")
		return nil
	}

	tui.PrintInfo("Removing microsocks...")
	if err := proxy.UninstallMicrosocks(); err != nil {
		return fmt.Errorf("failed to uninstall microsocks: %w", err)
	}

	tui.PrintSuccess("SOCKS proxy removed!")
	return nil
}

func runSOCKSStatus(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	fmt.Println()

	installed := proxy.IsMicrosocksInstalled()
	running := proxy.IsMicrosocksRunning()

	lines := []string{
		tui.KV("Installed: ", boolToYesNo(installed)),
		tui.KV("Running:   ", boolToYesNo(running)),
	}

	if installed {
		lines = append(lines, tui.KV("Bind:      ", proxy.MicrosocksBindAddr))
		lines = append(lines, tui.KV("Port:      ", proxy.MicrosocksPort))
	}

	tui.PrintBox("SOCKS Proxy Status", lines)

	return nil
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
