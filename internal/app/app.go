package app

import (
	"fmt"
	"os"
	"strconv"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/download"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/ui"
)

var version string

func Run(v, buildTime string) error {
	version = v

	if !system.IsRoot() {
		return fmt.Errorf("this program must be run as root")
	}

	ui.PrintBanner(version)

	osInfo, err := system.DetectOS()
	if err != nil {
		ui.PrintWarning("Could not detect OS: " + err.Error())
	} else {
		ui.PrintInfo(fmt.Sprintf("Detected OS: %s", osInfo.PrettyName))
	}

	archInfo := system.DetectArch()
	ui.PrintInfo(fmt.Sprintf("Architecture: %s", archInfo.Arch))

	if service.IsInstalled() && config.Exists() {
		return showMainMenu()
	}

	return runInstallation(osInfo, archInfo)
}

func showMainMenu() error {
	for {
		options := []ui.MenuOption{
			{Key: "1", Label: "Install/Reconfigure dnstt server"},
			{Key: "2", Label: "Check service status"},
			{Key: "3", Label: "View service logs"},
			{Key: "4", Label: "Show configuration info"},
			{Key: "5", Label: "Restart service"},
			{Key: "6", Label: "Uninstall"},
			{Key: "0", Label: "Exit"},
		}

		ui.ShowMenu(options)
		choice := ui.Prompt("Select option")

		switch choice {
		case "1":
			osInfo, _ := system.DetectOS()
			archInfo := system.DetectArch()
			if err := runInstallation(osInfo, archInfo); err != nil {
				ui.PrintError(err.Error())
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
			ui.PrintInfo("Goodbye!")
			return nil
		default:
			ui.PrintError("Invalid option")
		}
	}
}

func runInstallation(osInfo *system.OSInfo, archInfo *system.ArchInfo) error {
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{
			MTU:        "1232",
			TunnelMode: "ssh",
		}
	}

	fmt.Println()
	ui.PrintInfo("Starting installation wizard...")
	fmt.Println()

	// Step 1: Get NS subdomain
	cfg.NSSubdomain = ui.PromptWithDefault("Enter NS subdomain (e.g., t.example.com)", cfg.NSSubdomain)
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
	mtuInt := ui.PromptInt("Enter MTU value (512-1400)", currentMTU, 512, 1400)
	cfg.MTU = fmt.Sprintf("%d", mtuInt)

	// Step 3: Get tunnel mode
	cfg.TunnelMode = ui.PromptChoice("Select tunnel mode", []string{"ssh", "socks"}, cfg.TunnelMode)

	// Step 4: Get target port for SSH mode
	if cfg.TunnelMode == "ssh" {
		defaultPort := system.DetectSSHPort()
		cfg.TargetPort = ui.PromptWithDefault("Enter SSH port", defaultPort)
	} else {
		cfg.TargetPort = "1080"
	}

	// Set key file paths
	cfg.PrivateKeyFile, cfg.PublicKeyFile = config.GetKeyFilenames(cfg.NSSubdomain)

	fmt.Println()
	if !ui.Confirm("Proceed with installation?", true) {
		return fmt.Errorf("installation cancelled")
	}

	totalSteps := 7
	currentStep := 0

	// Step 1: Download dnstt-server
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Downloading dnstt-server binary...")

	if !download.IsDnsttInstalled() {
		checksums, _ := download.FetchChecksums(archInfo.DnsttArch)

		tmpPath, err := download.DownloadDnsttServer(archInfo.DnsttArch, ui.PrintProgress)
		ui.ClearLine()
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		if checksums.SHA256 != "" || checksums.MD5 != "" {
			if err := download.VerifyChecksums(tmpPath, checksums); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("checksum verification failed: %w", err)
			}
			ui.PrintStatus("Checksum verified")
		}

		if err := download.InstallBinary(tmpPath); err != nil {
			return fmt.Errorf("installation failed: %w", err)
		}
	}
	ui.PrintStatus("dnstt-server binary installed")

	// Step 2: Create dnstt user
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Creating dnstt user...")
	if err := system.CreateDnsttUser(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	ui.PrintStatus("User 'dnstt' created")

	// Step 3: Generate keys
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Generating cryptographic keys...")

	var publicKey string
	if keys.KeysExist(cfg.PrivateKeyFile, cfg.PublicKeyFile) {
		if ui.Confirm("Keys already exist. Regenerate?", false) {
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
	ui.PrintStatus("Keys generated")

	// Step 4: Save configuration
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Saving configuration...")
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	ui.PrintStatus("Configuration saved")

	// Step 5: Configure firewall
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Configuring firewall...")
	if err := network.ConfigureFirewall(); err != nil {
		ui.PrintWarning("Firewall configuration warning: " + err.Error())
	} else {
		ui.PrintStatus("Firewall configured")
	}

	if system.HasIPv6() {
		network.ConfigureIPv6()
		ui.PrintStatus("IPv6 rules configured")
	}

	// Step 6: Setup Dante (if SOCKS mode)
	currentStep++
	if cfg.TunnelMode == "socks" {
		ui.PrintStep(currentStep, totalSteps, "Setting up Dante SOCKS proxy...")

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
			ui.PrintWarning("Dante start warning: " + err.Error())
		}
		ui.PrintStatus("Dante SOCKS proxy configured")
	} else {
		ui.PrintStep(currentStep, totalSteps, "Skipping Dante setup (SSH mode)...")
		ui.PrintStatus("SSH mode selected")
	}

	// Step 7: Create and start systemd service
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Creating systemd service...")

	if err := service.CreateService(cfg); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	if err := service.SetPermissions(); err != nil {
		ui.PrintWarning("Permission setting warning: " + err.Error())
	}

	if err := service.Enable(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	if err := service.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	ui.PrintStatus("Service started")

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

	ui.PrintBox("Installation Complete!", lines)

	ui.PrintInfo("Useful commands:")
	fmt.Println("  systemctl status dnstt-server  - Check service status")
	fmt.Println("  journalctl -u dnstt-server -f  - View live logs")
	fmt.Println("  dnstm                          - Open this menu")
}

func showStatus() {
	fmt.Println()
	status, _ := service.Status()
	fmt.Println(status)
	ui.WaitForEnter()
}

func showLogs() {
	fmt.Println()
	logs, err := service.GetLogs(50)
	if err != nil {
		ui.PrintError(err.Error())
	} else {
		fmt.Println(logs)
	}
	ui.WaitForEnter()
}

func showConfig() {
	cfg, err := config.Load()
	if err != nil {
		ui.PrintError("Failed to load configuration: " + err.Error())
		ui.WaitForEnter()
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

	ui.PrintBox("Current Configuration", lines)

	if service.IsActive() {
		ui.PrintStatus("Service is running")
	} else {
		ui.PrintWarning("Service is not running")
	}

	ui.WaitForEnter()
}

func restartService() {
	ui.PrintInfo("Restarting service...")
	if err := service.Restart(); err != nil {
		ui.PrintError(err.Error())
	} else {
		ui.PrintStatus("Service restarted successfully")
	}
	ui.WaitForEnter()
}

func runUninstall() bool {
	fmt.Println()
	ui.PrintWarning("This will completely remove dnstt from your system:")
	fmt.Println("  - Stop and remove the dnstt-server service")
	fmt.Println("  - Remove the dnstt-server binary")
	fmt.Println("  - Remove all configuration files and keys")
	fmt.Println("  - Remove firewall rules")
	fmt.Println("  - Remove the dnstt system user")
	fmt.Println()

	if !ui.Confirm("Are you sure you want to uninstall?", false) {
		ui.PrintInfo("Uninstall cancelled")
		ui.WaitForEnter()
		return false
	}

	fmt.Println()
	totalSteps := 5
	currentStep := 0

	// Step 1: Stop and remove service
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Stopping and removing service...")
	if service.IsActive() {
		service.Stop()
	}
	if service.IsEnabled() {
		service.Disable()
	}
	service.Remove()
	ui.PrintStatus("Service removed")

	// Step 2: Remove binary
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Removing dnstt-server binary...")
	download.RemoveBinary()
	ui.PrintStatus("Binary removed")

	// Step 3: Remove configuration and keys
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Removing configuration and keys...")
	config.RemoveAll()
	ui.PrintStatus("Configuration removed")

	// Step 4: Remove firewall rules
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Removing firewall rules...")
	network.RemoveFirewallRules()
	ui.PrintStatus("Firewall rules removed")

	// Step 5: Remove user
	currentStep++
	ui.PrintStep(currentStep, totalSteps, "Removing dnstt user...")
	system.RemoveDnsttUser()
	ui.PrintStatus("User removed")

	fmt.Println()
	ui.PrintSuccess("Uninstallation complete!")
	ui.PrintInfo("All dnstt components have been removed from your system.")

	return true
}
