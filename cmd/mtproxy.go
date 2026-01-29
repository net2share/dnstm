package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/mtproxy"
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

var mtproxyRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart MTProxy",
	Long:  "Restart the MTProxy server",
	RunE:  runMTProxyRestart,
}

var mtproxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop MTProxy",
	Long:  "Stop the MTProxy server",
	RunE:  runMTProxyStop,
}

var mtproxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start MTProxy",
	Long:  "Start the MTProxy server",
	RunE:  runMTProxyStart,
}

func init() {
	mtproxyCmd.AddCommand(mtproxyInstallCmd)
	mtproxyCmd.AddCommand(mtproxyUninstallCmd)
	mtproxyCmd.AddCommand(mtproxyStatusCmd)
	mtproxyCmd.AddCommand(mtproxyRestartCmd)
	mtproxyCmd.AddCommand(mtproxyStopCmd)
	mtproxyCmd.AddCommand(mtproxyStartCmd)

	// Flags for install
	mtproxyInstallCmd.Flags().StringP("domain", "d", "", "Domain for proxy URL (required for connection URL)")
}

func runMTProxyInstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	fmt.Println()

	// Get domain from flag or prompt
	domain, _ := cmd.Flags().GetString("domain")
	if domain == "" {
		err := huh.NewInput().
			Title("Domain").
			Description("Enter the domain/IP for the proxy URL (e.g., proxy.example.com)").
			Value(&domain).
			Run()
		if err != nil {
			return err
		}
	}

	if domain == "" {
		return fmt.Errorf("domain is required for generating connection URL")
	}

	// Generate secret
	secret, err := mtproxy.GenerateSecret()
	if err != nil {
		return fmt.Errorf("failed to generate secret: %w", err)
	}

	tui.PrintStatus(fmt.Sprintf("Using MTProxy secret: %s", secret))

	totalSteps := 3
	currentStep := 0

	// Step 1: Install MTProxy
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Installing MTProxy...")

	progressFn := func(downloaded, total int64) {
		if total > 0 {
			percent := float64(downloaded) / float64(total) * 100
			fmt.Printf("\rDownloading: %.1f%%", percent)
		}
	}

	if err := mtproxy.InstallMTProxy(progressFn); err != nil {
		return fmt.Errorf("failed to install MTProxy: %w", err)
	}
	tui.PrintStatus("MTProxy installed")

	// Step 2: Configure MTProxy
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Configuring MTProxy...")
	if err := mtproxy.ConfigureMTProxy(secret); err != nil {
		return fmt.Errorf("failed to configure MTProxy: %w", err)
	}
	tui.PrintStatus("MTProxy configured")

	// Step 3: Show connection info
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Generating connection URL...")

	proxyUrl := mtproxy.FormatProxyURL(secret, domain)

	fmt.Println()
	tui.PrintSuccess("MTProxy installed and running!")
	fmt.Println()

	tui.PrintBox("MTProxy Connection Info", []string{
		tui.KV("Domain:    ", domain),
		tui.KV("Port:      ", mtproxy.MTProxyPort),
		tui.KV("Secret:    ", secret),
		"",
		"Connection URL:",
		proxyUrl,
	})

	fmt.Println()
	tui.PrintInfo("Share the connection URL with Telegram users to connect via this proxy.")
	fmt.Println()

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

	// Confirm
	confirm := false
	err := huh.NewConfirm().
		Title("Uninstall MTProxy?").
		Description("This will stop and remove MTProxy completely").
		Value(&confirm).
		Run()
	if err != nil {
		return err
	}
	if !confirm {
		tui.PrintInfo("Cancelled")
		return nil
	}

	tui.PrintInfo("Removing MTProxy...")
	if err := mtproxy.UninstallMTProxy(); err != nil {
		return fmt.Errorf("failed to uninstall MTProxy: %w", err)
	}

	fmt.Println()
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
func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func runMTProxyRestart(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	fmt.Println()

	if !mtproxy.IsMTProxyInstalled() {
		return fmt.Errorf("MTProxy is not installed")
	}

	tui.PrintInfo("Restarting MTProxy...")
	if err := mtproxy.RestartMTProxy(); err != nil {
		return fmt.Errorf("failed to restart MTProxy: %w", err)
	}

	tui.PrintSuccess("MTProxy restarted!")
	return nil
}

func runMTProxyStop(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	fmt.Println()

	if !mtproxy.IsMTProxyInstalled() {
		return fmt.Errorf("MTProxy is not installed")
	}

	if !mtproxy.IsMTProxyRunning() {
		tui.PrintInfo("MTProxy is already stopped")
		return nil
	}

	tui.PrintInfo("Stopping MTProxy...")
	if err := mtproxy.StopMTProxy(); err != nil {
		return fmt.Errorf("failed to stop MTProxy: %w", err)
	}

	tui.PrintSuccess("MTProxy stopped!")
	return nil
}

func runMTProxyStart(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	fmt.Println()

	if !mtproxy.IsMTProxyInstalled() {
		return fmt.Errorf("MTProxy is not installed. Run 'dnstm mtproxy install' first")
	}

	if mtproxy.IsMTProxyRunning() {
		tui.PrintInfo("MTProxy is already running")
		return nil
	}

	tui.PrintInfo("Starting MTProxy...")
	if err := mtproxy.StartMTProxy(); err != nil {
		return fmt.Errorf("failed to start MTProxy: %w", err)
	}

	tui.PrintSuccess("MTProxy started!")
	return nil
}
