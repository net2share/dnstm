package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/tunnel"
	"github.com/net2share/dnstm/internal/tunnel/shadowsocks"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

// Shadowsocks install flags
var (
	shadowsocksDomain   string
	shadowsocksPassword string
	shadowsocksMethod   string
)

var shadowsocksCmd = &cobra.Command{
	Use:   "shadowsocks",
	Short: "Manage Shadowsocks DNS tunnel provider",
	Long:  "Manage the Shadowsocks DNS tunnel provider with Slipstream plugin (SIP003)",
}

var shadowsocksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install/configure Shadowsocks server",
	Long:  "Install and configure the Shadowsocks server with Slipstream DNS tunnel plugin",
	RunE:  runShadowsocksInstall,
}

var shadowsocksUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Shadowsocks server",
	Long:  "Remove the Shadowsocks server and its configuration",
	RunE:  runShadowsocksUninstall,
}

var shadowsocksStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Shadowsocks status",
	Long:  "Display the current status of the Shadowsocks server",
	RunE:  runShadowsocksStatus,
}

var shadowsocksConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show Shadowsocks configuration",
	Long:  "Display the current Shadowsocks server configuration",
	RunE:  runShadowsocksConfig,
}

func init() {
	// Install flags
	shadowsocksInstallCmd.Flags().StringVar(&shadowsocksDomain, "domain", "", "Domain (e.g., t.example.com)")
	shadowsocksInstallCmd.Flags().StringVar(&shadowsocksPassword, "password", "", "Shadowsocks password (auto-generated if not provided)")
	shadowsocksInstallCmd.Flags().StringVar(&shadowsocksMethod, "method", "aes-256-gcm", "Encryption method (aes-256-gcm, chacha20-ietf-poly1305, aes-128-gcm)")

	shadowsocksCmd.AddCommand(shadowsocksInstallCmd)
	shadowsocksCmd.AddCommand(shadowsocksUninstallCmd)
	shadowsocksCmd.AddCommand(shadowsocksStatusCmd)
	shadowsocksCmd.AddCommand(shadowsocksConfigCmd)
}

func runShadowsocksInstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	osInfo, err := osdetect.Detect()
	if err != nil {
		tui.PrintWarning("Could not detect OS: " + err.Error())
	} else {
		tui.PrintInfo(fmt.Sprintf("Detected OS: %s", osInfo.PrettyName))
	}

	arch := osdetect.GetArch()
	tui.PrintInfo(fmt.Sprintf("Architecture: %s", arch))

	provider := shadowsocks.NewProvider()

	// Check if CLI mode (domain provided)
	if cmd.Flags().Changed("domain") {
		return runShadowsocksInstallCLI(provider, cmd)
	}

	// Interactive mode
	result, err := provider.RunInteractiveInstall()
	if err != nil {
		return err
	}
	if result != nil {
		showShadowsocksInstallSuccess(provider, result)
	}
	return nil
}

func runShadowsocksInstallCLI(provider *shadowsocks.Provider, cmd *cobra.Command) error {
	if shadowsocksDomain == "" {
		return fmt.Errorf("--domain is required for CLI installation")
	}

	// Validate method
	validMethods := []string{"aes-256-gcm", "chacha20-ietf-poly1305", "aes-128-gcm"}
	methodValid := false
	for _, m := range validMethods {
		if shadowsocksMethod == m {
			methodValid = true
			break
		}
	}
	if !methodValid {
		return fmt.Errorf("--method must be one of: aes-256-gcm, chacha20-ietf-poly1305, aes-128-gcm")
	}

	cfg := &tunnel.InstallConfig{
		Domain: shadowsocksDomain,
	}

	result, err := provider.Install(cfg)
	if err != nil {
		return err
	}
	if result != nil {
		showShadowsocksInstallSuccess(provider, result)
	}
	return nil
}

func runShadowsocksUninstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	menu.Version = Version
	menu.BuildTime = BuildTime
	menu.PrintBanner()

	provider := shadowsocks.NewProvider()
	if !provider.IsInstalled() {
		tui.PrintInfo("Shadowsocks is not installed")
		return nil
	}

	tui.PrintInfo("Uninstalling Shadowsocks...")
	if err := provider.Uninstall(); err != nil {
		return err
	}

	tui.PrintSuccess("Shadowsocks has been uninstalled")
	return nil
}

func runShadowsocksStatus(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	provider := shadowsocks.NewProvider()
	status, err := provider.Status()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(tui.Header("Shadowsocks Status"))
	fmt.Println()

	if !status.Installed {
		tui.PrintInfo("Shadowsocks is not installed")
		return nil
	}

	statusText := "Stopped"
	if status.Running {
		statusText = "Running"
	}

	activeText := "No"
	if status.Active {
		activeText = "Yes"
	}

	fmt.Println(tui.KV("Installed: ", "Yes"))
	fmt.Println(tui.KV("Status:    ", statusText))
	fmt.Println(tui.KV("Enabled:   ", fmt.Sprintf("%v", status.Enabled)))
	fmt.Println(tui.KV("Active:    ", activeText))

	// Show service status
	serviceStatus, err := provider.GetServiceStatus()
	if err == nil {
		fmt.Println()
		fmt.Println(tui.Header("Service Status"))
		fmt.Println(serviceStatus)
	}

	return nil
}

func runShadowsocksConfig(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	provider := shadowsocks.NewProvider()
	if !provider.IsInstalled() {
		tui.PrintInfo("Shadowsocks is not installed")
		return nil
	}

	config, err := provider.GetConfig()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(tui.Header("Shadowsocks Configuration"))
	fmt.Println()
	fmt.Println(config)

	return nil
}

func showShadowsocksInstallSuccess(provider *shadowsocks.Provider, result *tunnel.InstallResult) {
	// Load config to get password
	cfg, _ := shadowsocks.Load()

	lines := []string{
		tui.KV("Domain:     ", result.Domain),
		tui.KV("Method:     ", cfg.Method),
		tui.KV("Password:   ", cfg.Password),
		tui.KV("Port:       ", shadowsocks.Port),
	}

	if result.Fingerprint != "" {
		lines = append(lines, "")
		lines = append(lines, tui.Header("Certificate SHA256 Fingerprint:"))
		lines = append(lines, tui.Value(shadowsocks.FormatFingerprint(result.Fingerprint)))
	}

	lines = append(lines, "")
	lines = append(lines, tui.Header("Client Configuration:"))
	lines = append(lines, tui.KV("  Server:      ", result.Domain+" (via DNS resolver)"))
	lines = append(lines, tui.KV("  Port:        ", "53"))
	lines = append(lines, tui.KV("  Password:    ", "<shown above>"))
	lines = append(lines, tui.KV("  Method:      ", cfg.Method))
	lines = append(lines, tui.KV("  Plugin:      ", "slipstream"))
	if result.Fingerprint != "" {
		lines = append(lines, tui.KV("  Plugin Opts: ", fmt.Sprintf("domain=%s;fingerprint=%s", result.Domain, result.Fingerprint[:32]+"...")))
	}

	tui.PrintBox("Installation Complete!", lines)

	// Show next steps guidance
	fmt.Println()
	tui.PrintInfo("Next steps:")
	fmt.Println("  Configure your Shadowsocks client with the above settings")
	fmt.Println("  Use any Shadowsocks client with Slipstream plugin support")

	fmt.Println()
	tui.PrintInfo("Useful commands:")
	fmt.Println(tui.KV(fmt.Sprintf("  systemctl status %s  ", provider.ServiceName()), "- Check service status"))
	fmt.Println(tui.KV(fmt.Sprintf("  journalctl -u %s -f  ", provider.ServiceName()), "- View live logs"))
	fmt.Println(tui.KV("  dnstm                              ", "- Open main menu"))
}
