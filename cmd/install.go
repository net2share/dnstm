package cmd

import (
	"fmt"
	"strconv"

	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/tunnel"
	"github.com/net2share/dnstm/internal/tunnel/dnstt"
	"github.com/net2share/dnstm/internal/tunnel/shadowsocks"
	"github.com/net2share/dnstm/internal/tunnel/slipstream"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

// DNSTT install flags
var (
	dnsttNSSubdomain string
	dnsttMTU         string
	dnsttMode        string
	dnsttPort        string
)

// Slipstream install flags
var (
	slipstreamDomain string
	slipstreamMode   string
	slipstreamPort   string
)

// Shadowsocks install flags (for install subcommand)
var (
	installShadowsocksDomain   string
	installShadowsocksPassword string
	installShadowsocksMethod   string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install a DNS tunnel provider",
	Long:  "Install a DNS tunnel provider (dnstt, slipstream, or shadowsocks)",
}

var installDnsttCmd = &cobra.Command{
	Use:   "dnstt",
	Short: "Install/configure DNSTT server",
	Long:  "Install and configure the DNSTT DNS tunnel server",
	RunE:  runInstallDnstt,
}

var installSlipstreamCmd = &cobra.Command{
	Use:   "slipstream",
	Short: "Install/configure Slipstream server",
	Long:  "Install and configure the Slipstream DNS tunnel server",
	RunE:  runInstallSlipstream,
}

var installShadowsocksCmd = &cobra.Command{
	Use:   "shadowsocks",
	Short: "Install/configure Shadowsocks server",
	Long:  "Install and configure the Shadowsocks server with Slipstream DNS tunnel plugin",
	RunE:  runInstallShadowsocks,
}

func init() {
	// DNSTT flags
	installDnsttCmd.Flags().StringVar(&dnsttNSSubdomain, "ns-subdomain", "", "NS subdomain (e.g., t.example.com)")
	installDnsttCmd.Flags().StringVar(&dnsttMTU, "mtu", "1232", "MTU value (512-1400)")
	installDnsttCmd.Flags().StringVar(&dnsttMode, "mode", "ssh", "Tunnel mode (ssh|socks|mtproto)")
	installDnsttCmd.Flags().StringVar(&dnsttPort, "port", "", "Target port (default: 22 for SSH, 1080 for SOCKS, 8443 for MTProxy)")

	// Slipstream flags
	installSlipstreamCmd.Flags().StringVar(&slipstreamDomain, "domain", "", "Domain (e.g., t.example.com)")
	installSlipstreamCmd.Flags().StringVar(&slipstreamMode, "mode", "ssh", "Tunnel mode (ssh|socks|mtproto)")
	installSlipstreamCmd.Flags().StringVar(&slipstreamPort, "port", "", "Target port (default: 22 for SSH, 1080 for SOCKS, 8443 for MTProxy)")

	// Shadowsocks flags
	installShadowsocksCmd.Flags().StringVar(&installShadowsocksDomain, "domain", "", "Domain (e.g., t.example.com)")
	installShadowsocksCmd.Flags().StringVar(&installShadowsocksPassword, "password", "", "Shadowsocks password (auto-generated if not provided)")
	installShadowsocksCmd.Flags().StringVar(&installShadowsocksMethod, "method", "aes-256-gcm", "Encryption method (aes-256-gcm, chacha20-ietf-poly1305, aes-128-gcm)")

	installCmd.AddCommand(installDnsttCmd)
	installCmd.AddCommand(installSlipstreamCmd)
	installCmd.AddCommand(installShadowsocksCmd)
}

func runInstallDnstt(cmd *cobra.Command, args []string) error {
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

	provider := dnstt.NewProvider()

	// Check if CLI mode (ns-subdomain provided)
	if cmd.Flags().Changed("ns-subdomain") {
		return runInstallDnsttCLI(provider, cmd)
	}

	// Interactive mode
	result, err := provider.RunInteractiveInstall()
	if err != nil {
		return err
	}
	if result != nil {
		showInstallSuccess(provider, result)
	}
	return nil
}

func runInstallDnsttCLI(provider *dnstt.Provider, cmd *cobra.Command) error {
	if dnsttNSSubdomain == "" {
		return fmt.Errorf("--ns-subdomain is required for CLI installation")
	}

	// Validate MTU
	mtu, err := strconv.Atoi(dnsttMTU)
	if err != nil || mtu < 512 || mtu > 1400 {
		return fmt.Errorf("--mtu must be a number between 512 and 1400")
	}

	// Validate mode
	if dnsttMode != "ssh" && dnsttMode != "socks" {
		return fmt.Errorf("--mode must be 'ssh' or 'socks'")
	}

	// Set default port if not provided
	port := dnsttPort
	if port == "" {
		if dnsttMode == "ssh" {
			port = osdetect.DetectSSHPort()
		} else {
			port = "1080"
		}
	}

	cfg := &tunnel.InstallConfig{
		Domain:     dnsttNSSubdomain,
		MTU:        dnsttMTU,
		TunnelMode: dnsttMode,
		TargetPort: port,
	}

	result, err := provider.Install(cfg)
	if err != nil {
		return err
	}
	if result != nil {
		showInstallSuccess(provider, result)
	}
	return nil
}

func runInstallSlipstream(cmd *cobra.Command, args []string) error {
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

	provider := slipstream.NewProvider()

	// Check if CLI mode (domain provided)
	if cmd.Flags().Changed("domain") {
		return runInstallSlipstreamCLI(provider, cmd)
	}

	// Interactive mode
	result, err := provider.RunInteractiveInstall()
	if err != nil {
		return err
	}
	if result != nil {
		showInstallSuccess(provider, result)
	}
	return nil
}

func runInstallSlipstreamCLI(provider *slipstream.Provider, cmd *cobra.Command) error {
	if slipstreamDomain == "" {
		return fmt.Errorf("--domain is required for CLI installation")
	}

	// Validate mode
	if slipstreamMode != "ssh" && slipstreamMode != "socks" {
		return fmt.Errorf("--mode must be 'ssh' or 'socks'")
	}

	// Set default port if not provided
	port := slipstreamPort
	if port == "" {
		if slipstreamMode == "ssh" {
			port = osdetect.DetectSSHPort()
		} else {
			port = "1080"
		}
	}

	cfg := &tunnel.InstallConfig{
		Domain:     slipstreamDomain,
		TunnelMode: slipstreamMode,
		TargetPort: port,
	}

	result, err := provider.Install(cfg)
	if err != nil {
		return err
	}
	if result != nil {
		showInstallSuccess(provider, result)
	}
	return nil
}

func runInstallShadowsocks(cmd *cobra.Command, args []string) error {
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
		return runInstallShadowsocksCLI(provider, cmd)
	}

	// Interactive mode
	result, err := provider.RunInteractiveInstall()
	if err != nil {
		return err
	}
	if result != nil {
		showShadowsocksSuccess(provider, result)
	}
	return nil
}

func runInstallShadowsocksCLI(provider *shadowsocks.Provider, cmd *cobra.Command) error {
	if installShadowsocksDomain == "" {
		return fmt.Errorf("--domain is required for CLI installation")
	}

	// Validate method
	validMethods := []string{"aes-256-gcm", "chacha20-ietf-poly1305", "aes-128-gcm"}
	methodValid := false
	for _, m := range validMethods {
		if installShadowsocksMethod == m {
			methodValid = true
			break
		}
	}
	if !methodValid {
		return fmt.Errorf("--method must be one of: aes-256-gcm, chacha20-ietf-poly1305, aes-128-gcm")
	}

	cfg := &tunnel.InstallConfig{
		Domain: installShadowsocksDomain,
	}

	result, err := provider.Install(cfg)
	if err != nil {
		return err
	}
	if result != nil {
		showShadowsocksSuccess(provider, result)
	}
	return nil
}

func showShadowsocksSuccess(provider *shadowsocks.Provider, result *tunnel.InstallResult) {
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
		lines = append(lines, tui.KV("  Plugin Opts: ", fmt.Sprintf("domain=%s;fingerprint=%s...", result.Domain, result.Fingerprint[:32])))
	}

	tui.PrintBox("Installation Complete!", lines)

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

func showInstallSuccess(provider tunnel.Provider, result *tunnel.InstallResult) {
	lines := []string{
		tui.KV("Domain:       ", result.Domain),
		tui.KV("Tunnel Mode:  ", result.TunnelMode),
	}

	if result.MTU != "" {
		lines = append(lines, tui.KV("MTU:          ", result.MTU))
	}

	// Show public key (DNSTT) or fingerprint (Slipstream)
	if result.PublicKey != "" {
		lines = append(lines, "")
		lines = append(lines, tui.Header("Public Key (for client):"))
		lines = append(lines, tui.Value(result.PublicKey))
	}

	if result.Fingerprint != "" {
		lines = append(lines, "")
		lines = append(lines, tui.Header("Certificate SHA256 Fingerprint:"))
		lines = append(lines, tui.Value(slipstream.FormatFingerprint(result.Fingerprint)))
	}

	tui.PrintBox("Installation Complete!", lines)

	// Show next steps guidance based on tunnel mode
	fmt.Println()
	tui.PrintInfo("Next steps:")
	if result.TunnelMode == "socks" {
		fmt.Println("  Run 'dnstm socks install' to set up the SOCKS proxy")
	} else if result.TunnelMode == "mtproto" {
		fmt.Println("  Run 'dnstm mtproxy install' to set up MTProxy")
		if result.MTProxySecret != "" {
			fmt.Println()
			tui.PrintInfo("MTProxy Configuration:")
			fmt.Printf("  Secret: %s\n", result.MTProxySecret)
			fmt.Printf("  Port:   8443\n")
		}
	} else {
		fmt.Println("  1. Run 'dnstm ssh-users' to configure SSH hardening")
		fmt.Println("  2. Create tunnel users with the SSH users menu")
	}

	fmt.Println()
	tui.PrintInfo("Useful commands:")
	fmt.Println(tui.KV(fmt.Sprintf("  systemctl status %s  ", provider.ServiceName()), "- Check service status"))
	fmt.Println(tui.KV(fmt.Sprintf("  journalctl -u %s -f  ", provider.ServiceName()), "- View live logs"))
	fmt.Println(tui.KV("  dnstm                          ", "- Open this menu"))
}
