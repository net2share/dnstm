package cmd

import (
	"fmt"
	"strconv"

	"github.com/net2share/dnstm/internal/installer"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var (
	installNSSubdomain string
	installMTU         string
	installMode        string
	installPort        string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install/configure dnstt server",
	RunE:  runInstall,
}

func init() {
	installCmd.Flags().StringVar(&installNSSubdomain, "ns-subdomain", "", "NS subdomain (e.g., t.example.com)")
	installCmd.Flags().StringVar(&installMTU, "mtu", "1232", "MTU value (512-1400)")
	installCmd.Flags().StringVar(&installMode, "mode", "ssh", "Tunnel mode (ssh|socks)")
	installCmd.Flags().StringVar(&installPort, "port", "", "Target port (default: 22 for SSH, 1080 for SOCKS)")
}

func runInstall(cmd *cobra.Command, args []string) error {
	if err := osdetect.RequireRoot(); err != nil {
		return err
	}

	installer.PrintBanner(Version, BuildTime)

	osInfo, err := osdetect.Detect()
	if err != nil {
		tui.PrintWarning("Could not detect OS: " + err.Error())
	} else {
		tui.PrintInfo(fmt.Sprintf("Detected OS: %s", osInfo.PrettyName))
	}

	archInfo := installer.DetectArch()
	tui.PrintInfo(fmt.Sprintf("Architecture: %s", archInfo.Arch))

	// Check if CLI mode (ns-subdomain provided)
	if cmd.Flags().Changed("ns-subdomain") {
		return runInstallCLI(osInfo, archInfo, cmd)
	}

	return installer.RunInteractive(osInfo, archInfo)
}

func runInstallCLI(osInfo *osdetect.OSInfo, archInfo *installer.ArchInfo, cmd *cobra.Command) error {
	if installNSSubdomain == "" {
		return fmt.Errorf("--ns-subdomain is required for CLI installation")
	}

	// Validate MTU
	mtu, err := strconv.Atoi(installMTU)
	if err != nil || mtu < 512 || mtu > 1400 {
		return fmt.Errorf("--mtu must be a number between 512 and 1400")
	}

	// Validate mode
	if installMode != "ssh" && installMode != "socks" {
		return fmt.Errorf("--mode must be 'ssh' or 'socks'")
	}

	// Set default port if not provided
	if installPort == "" {
		if installMode == "ssh" {
			installPort = osdetect.DetectSSHPort()
		} else {
			installPort = "1080"
		}
	}

	return installer.RunCLI(osInfo, archInfo, installNSSubdomain, installMTU, installMode, installPort)
}
