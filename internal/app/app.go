package app

import (
	"fmt"
	"os"
	"strconv"

	"github.com/fatih/color"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/download"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

var version string

// ArchInfo contains architecture information for dnstt downloads.
type ArchInfo struct {
	Arch      string
	DnsttArch string
}

// detectArch returns architecture info with dnstt-specific naming.
func detectArch() *ArchInfo {
	arch := osdetect.GetArch()
	dnsttArch := arch

	switch arch {
	case "amd64":
		dnsttArch = "linux-amd64"
	case "arm64":
		dnsttArch = "linux-arm64"
	case "arm":
		dnsttArch = "linux-armv7"
	case "386":
		dnsttArch = "linux-386"
	}

	return &ArchInfo{
		Arch:      arch,
		DnsttArch: dnsttArch,
	}
}

// printBanner displays the application banner.
func printBanner(buildTime string) {
	titleColor := color.New(color.FgCyan, color.Bold)
	banner := `
    ____  _   _______  ________  ___
   / __ \/ | / / ___/ /_  __/  |/  /
  / / / /  |/ /\__ \   / / / /|_/ /
 / /_/ / /|  /___/ /  / / / /  / /
/_____/_/ |_//____/  /_/ /_/  /_/
`
	titleColor.Println(banner)
	fmt.Printf("DNS Tunnel Manager v%s (built %s)\n\n", version, buildTime)
}

func Run(v, buildTime string) error {
	version = v

	if !osdetect.IsRoot() {
		return fmt.Errorf("this program must be run as root")
	}

	printBanner(buildTime)

	osInfo, err := osdetect.Detect()
	if err != nil {
		tui.PrintWarning("Could not detect OS: " + err.Error())
	} else {
		tui.PrintInfo(fmt.Sprintf("Detected OS: %s", osInfo.PrettyName))
	}

	archInfo := detectArch()
	tui.PrintInfo(fmt.Sprintf("Architecture: %s", archInfo.Arch))

	if service.IsInstalled() && config.Exists() {
		return showMainMenu()
	}

	return runInstallation(osInfo, archInfo)
}

func showMainMenu() error {
	for {
		options := []tui.MenuOption{
			{Key: "1", Label: "Install/Reconfigure dnstt server"},
			{Key: "2", Label: "Check service status"},
			{Key: "3", Label: "View service logs"},
			{Key: "4", Label: "Show configuration info"},
			{Key: "5", Label: "Restart service"},
			{Key: "6", Label: "Uninstall"},
			{Key: "0", Label: "Exit"},
		}

		tui.ShowMenu(options)
		choice := tui.Prompt("Select option")

		switch choice {
		case "1":
			osInfo, _ := osdetect.Detect()
			archInfo := detectArch()
			if err := runInstallation(osInfo, archInfo); err != nil {
				tui.PrintError(err.Error())
			}
		case "2":
			showStatus()
		case "3":
			showLogs()
		case "4":
			showConfig()
		case "5":
			restartService()
		case "6":
			if runUninstall() {
				return nil
			}
		case "0", "q", "quit", "exit":
			tui.PrintInfo("Goodbye!")
			return nil
		default:
			tui.PrintError("Invalid option")
		}
	}
}

func runInstallation(osInfo *osdetect.OSInfo, archInfo *ArchInfo) error {
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{
			MTU:        "1232",
			TunnelMode: "ssh",
		}
	}

	fmt.Println()
	tui.PrintInfo("Starting installation wizard...")
	fmt.Println()

	// Step 1: Get NS subdomain
	cfg.NSSubdomain = tui.PromptWithDefault("Enter NS subdomain (e.g., t.example.com)", cfg.NSSubdomain)
	if cfg.NSSubdomain == "" {
		return fmt.Errorf("NS subdomain is required")
	}

	// Step 2: Get MTU
	currentMTU := 1232
	if cfg.MTU != "" {
		if parsed, err := strconv.Atoi(cfg.MTU); err == nil {
			currentMTU = parsed
		}
	}
	mtuInt := tui.PromptInt("Enter MTU value (512-1400)", currentMTU, 512, 1400)
	cfg.MTU = fmt.Sprintf("%d", mtuInt)

	// Step 3: Get tunnel mode
	cfg.TunnelMode = tui.PromptChoice("Select tunnel mode", []string{"ssh", "socks"}, cfg.TunnelMode)

	// Step 4: Get target port for SSH mode
	if cfg.TunnelMode == "ssh" {
		defaultPort := osdetect.DetectSSHPort()
		cfg.TargetPort = tui.PromptWithDefault("Enter SSH port", defaultPort)
	} else {
		cfg.TargetPort = "1080"
	}

	// Set key file paths
	cfg.PrivateKeyFile, cfg.PublicKeyFile = config.GetKeyFilenames(cfg.NSSubdomain)

	fmt.Println()
	if !tui.Confirm("Proceed with installation?", true) {
		return fmt.Errorf("installation cancelled")
	}

	totalSteps := 7
	currentStep := 0

	// Step 1: Download dnstt-server
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Downloading dnstt-server binary...")

	if !download.IsDnsttInstalled() {
		checksums, _ := download.FetchChecksums(archInfo.DnsttArch)

		tmpPath, err := download.DownloadDnsttServer(archInfo.DnsttArch, tui.PrintProgress)
		tui.ClearLine()
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		if checksums.SHA256 != "" || checksums.MD5 != "" {
			if err := download.VerifyChecksums(tmpPath, checksums); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("checksum verification failed: %w", err)
			}
			tui.PrintStatus("Checksum verified")
		}

		if err := download.InstallBinary(tmpPath); err != nil {
			return fmt.Errorf("installation failed: %w", err)
		}
	}
	tui.PrintStatus("dnstt-server binary installed")

	// Step 2: Create dnstt user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating dnstt user...")
	if err := system.CreateDnsttUser(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	tui.PrintStatus("User 'dnstt' created")

	// Step 3: Generate keys
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Generating cryptographic keys...")

	var publicKey string
	if keys.KeysExist(cfg.PrivateKeyFile, cfg.PublicKeyFile) {
		if tui.Confirm("Keys already exist. Regenerate?", false) {
			publicKey, err = keys.Generate(cfg.PrivateKeyFile, cfg.PublicKeyFile)
			if err != nil {
				return fmt.Errorf("key generation failed: %w", err)
			}
		} else {
			publicKey, _ = keys.ReadPublicKey(cfg.PublicKeyFile)
		}
	} else {
		publicKey, err = keys.Generate(cfg.PrivateKeyFile, cfg.PublicKeyFile)
		if err != nil {
			return fmt.Errorf("key generation failed: %w", err)
		}
	}
	tui.PrintStatus("Keys generated")

	// Step 4: Save configuration
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Saving configuration...")
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	tui.PrintStatus("Configuration saved")

	// Step 5: Configure firewall
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Configuring firewall...")
	if err := network.ConfigureFirewall(); err != nil {
		tui.PrintWarning("Firewall configuration warning: " + err.Error())
	} else {
		tui.PrintStatus("Firewall configured")
	}

	if osdetect.HasIPv6() {
		network.ConfigureIPv6()
		tui.PrintStatus("IPv6 rules configured")
	}

	// Step 6: Setup Dante (if SOCKS mode)
	currentStep++
	if cfg.TunnelMode == "socks" {
		tui.PrintStep(currentStep, totalSteps, "Setting up Dante SOCKS proxy...")

		if !proxy.IsDanteInstalled() {
			if osInfo != nil {
				if err := proxy.InstallDante(osInfo.PackageManager); err != nil {
					return fmt.Errorf("failed to install Dante: %w", err)
				}
			}
		}

		if err := proxy.ConfigureDante(); err != nil {
			return fmt.Errorf("failed to configure Dante: %w", err)
		}

		if err := proxy.StartDante(); err != nil {
			tui.PrintWarning("Dante start warning: " + err.Error())
		}
		tui.PrintStatus("Dante SOCKS proxy configured")
	} else {
		tui.PrintStep(currentStep, totalSteps, "Skipping Dante setup (SSH mode)...")
		tui.PrintStatus("SSH mode selected")
	}

	// Step 7: Create and start systemd service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating systemd service...")

	if err := service.CreateService(cfg); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	if err := service.SetPermissions(); err != nil {
		tui.PrintWarning("Permission setting warning: " + err.Error())
	}

	if err := service.Enable(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	if err := service.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	tui.PrintStatus("Service started")

	// Show success information
	showSuccessInfo(cfg, publicKey)

	return nil
}

func showSuccessInfo(cfg *config.Config, publicKey string) {
	lines := []string{
		fmt.Sprintf("NS Subdomain: %s", cfg.NSSubdomain),
		fmt.Sprintf("Tunnel Mode:  %s", cfg.TunnelMode),
		fmt.Sprintf("MTU:          %s", cfg.MTU),
		"",
		"Public Key (for client):",
		publicKey,
		"",
		"DNS Configuration Required:",
		"  A record:  tns.yourdomain.com -> <server-ip>",
		"  NS record: t.yourdomain.com   -> tns.yourdomain.com",
	}

	tui.PrintBox("Installation Complete!", lines)

	tui.PrintInfo("Useful commands:")
	fmt.Println("  systemctl status dnstt-server  - Check service status")
	fmt.Println("  journalctl -u dnstt-server -f  - View live logs")
	fmt.Println("  dnstm                          - Open this menu")
}

func showStatus() {
	fmt.Println()
	status, _ := service.Status()
	fmt.Println(status)
	tui.WaitForEnter()
}

func showLogs() {
	fmt.Println()
	logs, err := service.GetLogs(50)
	if err != nil {
		tui.PrintError(err.Error())
	} else {
		fmt.Println(logs)
	}
	tui.WaitForEnter()
}

func showConfig() {
	cfg, err := config.Load()
	if err != nil {
		tui.PrintError("Failed to load configuration: " + err.Error())
		tui.WaitForEnter()
		return
	}

	publicKey := ""
	if cfg.PublicKeyFile != "" {
		publicKey, _ = keys.ReadPublicKey(cfg.PublicKeyFile)
	}

	lines := []string{
		fmt.Sprintf("NS Subdomain:    %s", cfg.NSSubdomain),
		fmt.Sprintf("Tunnel Mode:     %s", cfg.TunnelMode),
		fmt.Sprintf("MTU:             %s", cfg.MTU),
		fmt.Sprintf("Target Port:     %s", cfg.TargetPort),
		fmt.Sprintf("Private Key:     %s", cfg.PrivateKeyFile),
		fmt.Sprintf("Public Key File: %s", cfg.PublicKeyFile),
		"",
		"Public Key (for client):",
		publicKey,
	}

	tui.PrintBox("Current Configuration", lines)

	if service.IsActive() {
		tui.PrintStatus("Service is running")
	} else {
		tui.PrintWarning("Service is not running")
	}

	tui.WaitForEnter()
}

func restartService() {
	tui.PrintInfo("Restarting service...")
	if err := service.Restart(); err != nil {
		tui.PrintError(err.Error())
	} else {
		tui.PrintStatus("Service restarted successfully")
	}
	tui.WaitForEnter()
}

func runUninstall() bool {
	fmt.Println()
	tui.PrintWarning("This will completely remove dnstt from your system:")
	fmt.Println("  - Stop and remove the dnstt-server service")
	fmt.Println("  - Remove the dnstt-server binary")
	fmt.Println("  - Remove all configuration files and keys")
	fmt.Println("  - Remove firewall rules")
	fmt.Println("  - Remove the dnstt system user")
	fmt.Println()

	if !tui.Confirm("Are you sure you want to uninstall?", false) {
		tui.PrintInfo("Uninstall cancelled")
		tui.WaitForEnter()
		return false
	}

	fmt.Println()
	totalSteps := 5
	currentStep := 0

	// Step 1: Stop and remove service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Stopping and removing service...")
	if service.IsActive() {
		service.Stop()
	}
	if service.IsEnabled() {
		service.Disable()
	}
	service.Remove()
	tui.PrintStatus("Service removed")

	// Step 2: Remove binary
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing dnstt-server binary...")
	download.RemoveBinary()
	tui.PrintStatus("Binary removed")

	// Step 3: Remove configuration and keys
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing configuration and keys...")
	config.RemoveAll()
	tui.PrintStatus("Configuration removed")

	// Step 4: Remove firewall rules
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing firewall rules...")
	network.RemoveFirewallRules()
	tui.PrintStatus("Firewall rules removed")

	// Step 5: Remove user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing dnstt user...")
	system.RemoveDnsttUser()
	tui.PrintStatus("User removed")

	fmt.Println()
	tui.PrintSuccess("Uninstallation complete!")
	tui.PrintInfo("All dnstt components have been removed from your system.")

	return true
}
