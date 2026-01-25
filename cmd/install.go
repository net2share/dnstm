package cmd

import (
	"fmt"
	"strconv"

	"github.com/net2share/dnstm/internal/menu"
	"github.com/net2share/dnstm/internal/tunnel"
	"github.com/net2share/dnstm/internal/tunnel/dnstt"
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

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install a DNS tunnel provider",
	Long:  "Install a DNS tunnel provider (dnstt or slipstream)",
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

func init() {
	// DNSTT flags
	installDnsttCmd.Flags().StringVar(&dnsttNSSubdomain, "ns-subdomain", "", "NS subdomain (e.g., t.example.com)")
	installDnsttCmd.Flags().StringVar(&dnsttMTU, "mtu", "1232", "MTU value (512-1400)")
	installDnsttCmd.Flags().StringVar(&dnsttMode, "mode", "ssh", "Tunnel mode (ssh|socks)")
	installDnsttCmd.Flags().StringVar(&dnsttPort, "port", "", "Target port (default: 22 for SSH, 1080 for SOCKS)")

	// Slipstream flags
	installSlipstreamCmd.Flags().StringVar(&slipstreamDomain, "domain", "", "Domain (e.g., t.example.com)")
	installSlipstreamCmd.Flags().StringVar(&slipstreamMode, "mode", "ssh", "Tunnel mode (ssh|socks)")
	installSlipstreamCmd.Flags().StringVar(&slipstreamPort, "port", "", "Target port (default: 22 for SSH, 1080 for SOCKS)")

	installCmd.AddCommand(installDnsttCmd)
	installCmd.AddCommand(installSlipstreamCmd)
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

	// Add created user info if available
	if result.CreatedUser != nil {
		lines = append(lines, "")
		lines = append(lines, tui.Header("SSH Tunnel User Created:"))
		lines = append(lines, tui.KV("  Username: ", result.CreatedUser.Username))
		lines = append(lines, tui.KV("  Auth:     ", result.CreatedUser.AuthMode))
		if result.CreatedUser.Password != "" {
			lines = append(lines, tui.KV("  Password: ", result.CreatedUser.Password))
		}
	}

	tui.PrintBox("Installation Complete!", lines)

	tui.PrintInfo("Useful commands:")
	fmt.Println(tui.KV(fmt.Sprintf("  systemctl status %s  ", provider.ServiceName()), "- Check service status"))
	fmt.Println(tui.KV(fmt.Sprintf("  journalctl -u %s -f  ", provider.ServiceName()), "- View live logs"))
	fmt.Println(tui.KV("  dnstm                          ", "- Open this menu"))
}
