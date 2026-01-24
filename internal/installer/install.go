// Package installer provides installation and uninstallation logic for dnstm.
package installer

import (
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/download"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/sshtunnel"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

// ArchInfo contains architecture information for dnstt downloads.
type ArchInfo struct {
	Arch      string
	DnsttArch string
}

// ASCII art banner for dnstm
const dnstmBanner = `
    ____  _   _______  ________  ___
   / __ \/ | / / ___/ /_  __/  |/  /
  / / / /  |/ /\__ \   / / / /|_/ /
 / /_/ / /|  /___/ /  / / / /  / /
/_____/_/ |_//____/  /_/ /_/  /_/
`

// PrintBanner displays the dnstm banner with version info.
func PrintBanner(version, buildTime string) {
	tui.PrintBanner(tui.BannerConfig{
		AppName:   "DNS Tunnel Manager",
		Version:   version,
		BuildTime: buildTime,
		ASCII:     dnstmBanner,
	})
}

// DetectArch returns architecture info with dnstt-specific naming.
func DetectArch() *ArchInfo {
	arch := osdetect.GetArch()
	dnsttArch := arch

	switch arch {
	case "amd64":
		dnsttArch = "linux-amd64"
	case "arm64":
		dnsttArch = "linux-arm64"
	}

	return &ArchInfo{
		Arch:      arch,
		DnsttArch: dnsttArch,
	}
}

// RunInteractive runs the interactive installation wizard.
func RunInteractive(osInfo *osdetect.OSInfo, archInfo *ArchInfo) error {
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

	// Step 1: Get NS subdomain (loop until valid)
	currentNS := cfg.NSSubdomain
	for {
		var nsSubdomain string
		input := huh.NewInput().
			Title("NS Subdomain").
			Value(&nsSubdomain)
		if currentNS != "" {
			input.Description("Current: " + currentNS + " (press Enter to keep)").
				Placeholder(currentNS)
		} else {
			input.Description("e.g., t.example.com")
		}
		err = input.Run()
		if err != nil {
			return err
		}
		if nsSubdomain == "" {
			if currentNS != "" {
				nsSubdomain = currentNS
			} else {
				tui.PrintError("NS subdomain is required")
				continue
			}
		}
		cfg.NSSubdomain = nsSubdomain
		break
	}

	// Step 2: Get MTU (loop until valid)
	currentMTU := "1232"
	if cfg.MTU != "" {
		currentMTU = cfg.MTU
	}
	for {
		var mtuStr string
		input := huh.NewInput().
			Title("MTU Value").
			Value(&mtuStr).
			Placeholder(currentMTU)
		if cfg.MTU != "" {
			input.Description(fmt.Sprintf("Current: %s (512-1400, press Enter to keep)", currentMTU))
		} else {
			input.Description("512-1400, default: 1232")
		}
		err = input.Run()
		if err != nil {
			return err
		}
		if mtuStr == "" {
			mtuStr = currentMTU
		}
		mtu, err := strconv.Atoi(mtuStr)
		if err != nil || mtu < 512 || mtu > 1400 {
			tui.PrintError("MTU must be a number between 512 and 1400")
			continue
		}
		cfg.MTU = mtuStr
		break
	}

	// Step 3: Get tunnel mode (pre-select current if reconfiguring)
	tunnelMode := cfg.TunnelMode
	if tunnelMode == "" {
		tunnelMode = "ssh"
	}
	err = huh.NewSelect[string]().
		Title("Tunnel Mode").
		Options(
			huh.NewOption("SSH Tunnel", "ssh"),
			huh.NewOption("SOCKS Proxy", "socks"),
		).
		Value(&tunnelMode).
		Run()
	if err != nil {
		return err
	}
	cfg.TunnelMode = tunnelMode

	// Step 4: Set target port based on mode
	if cfg.TunnelMode == "ssh" {
		cfg.TargetPort = osdetect.DetectSSHPort()
	} else {
		cfg.TargetPort = "1080"
	}

	// Set key file paths
	cfg.PrivateKeyFile, cfg.PublicKeyFile = config.GetKeyFilenames(cfg.NSSubdomain)

	// Show summary before confirmation
	summaryLines := []string{
		tui.KV("NS Subdomain:  ", cfg.NSSubdomain),
		tui.KV("MTU:           ", cfg.MTU),
		tui.KV("Tunnel Mode:   ", cfg.TunnelMode),
		tui.KV("Target Port:   ", cfg.TargetPort),
	}
	tui.PrintBox("Installation Summary", summaryLines)

	// Confirm installation (default to Yes)
	confirm := true
	err = huh.NewConfirm().
		Title("Proceed with installation?").
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()
	if err != nil {
		return err
	}

	if !confirm {
		tui.PrintInfo("Installation cancelled")
		return nil
	}

	return performInstallation(osInfo, archInfo, cfg)
}

// RunCLI runs the CLI installation with provided options.
func RunCLI(osInfo *osdetect.OSInfo, archInfo *ArchInfo, nsSubdomain, mtu, mode, port string) error {
	cfg := &config.Config{
		NSSubdomain: nsSubdomain,
		MTU:         mtu,
		TunnelMode:  mode,
		TargetPort:  port,
	}

	cfg.PrivateKeyFile, cfg.PublicKeyFile = config.GetKeyFilenames(cfg.NSSubdomain)

	return performInstallation(osInfo, archInfo, cfg)
}

func performInstallation(osInfo *osdetect.OSInfo, archInfo *ArchInfo, cfg *config.Config) error {
	totalSteps := 7
	currentStep := 0

	// Step 1: Download dnstt-server
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Downloading dnstt-server binary...")

	if !download.IsDnsttInstalled() {
		checksums, _ := download.FetchChecksums(config.DnsttReleaseURL, archInfo.DnsttArch)

		tmpPath, err := download.DownloadDnsttServer(config.DnsttReleaseURL, archInfo.DnsttArch, tui.PrintProgress)
		tui.ClearLine()
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		if checksums.SHA256 != "" {
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
	var err error
	if keys.KeysExist(cfg.PrivateKeyFile, cfg.PublicKeyFile) {
		var regenerate bool
		huh.NewConfirm().
			Title("Keys already exist. Regenerate?").
			Value(&regenerate).
			Run()

		if regenerate {
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

	// Step 6: Setup Dante (if SOCKS mode) or SSH tunnel hardening (if SSH mode)
	var createdUser *sshtunnel.CreatedUserInfo
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
		cfg.SSHTunnelEnabled = "false"
	} else {
		tui.PrintStep(currentStep, totalSteps, "Setting up SSH tunnel hardening...")
		createdUser = sshtunnel.ConfigureAndCreateUser()
		if sshtunnel.IsConfigured() {
			cfg.SSHTunnelEnabled = "true"
			tui.PrintStatus("SSH tunnel hardening configured")
		} else {
			cfg.SSHTunnelEnabled = "false"
			tui.PrintStatus("SSH mode selected (tunnel hardening failed)")
		}
	}

	// Save config again to persist SSHTunnelEnabled
	if err := cfg.Save(); err != nil {
		tui.PrintWarning("Failed to save SSH tunnel state: " + err.Error())
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
	showSuccessInfo(cfg, publicKey, createdUser)

	return nil
}

func showSuccessInfo(cfg *config.Config, publicKey string, createdUser *sshtunnel.CreatedUserInfo) {
	lines := []string{
		tui.KV("NS Subdomain: ", cfg.NSSubdomain),
		tui.KV("Tunnel Mode:  ", cfg.TunnelMode),
		tui.KV("MTU:          ", cfg.MTU),
		"",
		tui.Header("Public Key (for client):"),
		tui.Value(publicKey),
	}

	// Add created user info if available
	if createdUser != nil {
		lines = append(lines, "")
		lines = append(lines, tui.Header("SSH Tunnel User Created:"))
		lines = append(lines, tui.KV("  Username: ", createdUser.Username))
		lines = append(lines, tui.KV("  Auth:     ", createdUser.AuthMode))
		if createdUser.Password != "" {
			lines = append(lines, tui.KV("  Password: ", createdUser.Password))
		}
	}

	tui.PrintBox("Installation Complete!", lines)

	tui.PrintInfo("Useful commands:")
	fmt.Println(tui.KV("  systemctl status dnstt-server  ", "- Check service status"))
	fmt.Println(tui.KV("  journalctl -u dnstt-server -f  ", "- View live logs"))
	fmt.Println(tui.KV("  dnstm                          ", "- Open this menu"))
}
