package dnstt

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/download"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/tunnel"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

// Provider implements the tunnel.Provider interface for DNSTT.
type Provider struct{}

// NewProvider creates a new DNSTT provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the provider identifier.
func (p *Provider) Name() tunnel.ProviderType {
	return tunnel.ProviderDNSTT
}

// DisplayName returns a human-readable name.
func (p *Provider) DisplayName() string {
	return "DNSTT"
}

// Port returns the port this provider listens on.
func (p *Provider) Port() string {
	return Port
}

// Status returns the current status of the provider.
func (p *Provider) Status() (*tunnel.ProviderStatus, error) {
	globalCfg, _ := tunnel.LoadGlobalConfig()
	isActive := globalCfg != nil && globalCfg.ActiveProvider == tunnel.ProviderDNSTT

	return &tunnel.ProviderStatus{
		Installed:   p.IsInstalled(),
		Running:     IsActive(),
		Enabled:     IsEnabled(),
		Active:      isActive,
		ConfigValid: Exists(),
	}, nil
}

// IsInstalled checks if DNSTT is installed.
func (p *Provider) IsInstalled() bool {
	return download.IsBinaryInstalled(BinaryName) && IsServiceInstalled()
}

// Install performs the DNSTT installation.
func (p *Provider) Install(cfg *tunnel.InstallConfig) (*tunnel.InstallResult, error) {
	// Convert tunnel.InstallConfig to dnstt.Config
	dnsttCfg := &Config{
		NSSubdomain: cfg.Domain,
		MTU:         cfg.MTU,
		TunnelMode:  cfg.TunnelMode,
		TargetPort:  cfg.TargetPort,
	}

	if dnsttCfg.MTU == "" {
		dnsttCfg.MTU = "1232"
	}

	dnsttCfg.PrivateKeyFile, dnsttCfg.PublicKeyFile = GetKeyFilenames(dnsttCfg.NSSubdomain)

	return p.performInstallation(dnsttCfg)
}

func (p *Provider) performInstallation(cfg *Config) (*tunnel.InstallResult, error) {
	archInfo := detectArch()

	totalSteps := 6
	currentStep := 0

	// Step 1: Download dnstt-server
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Downloading dnstt-server binary...")

	if !download.IsBinaryInstalled(BinaryName) {
		binCfg := &download.BinaryConfig{
			BaseURL:      ReleaseURL,
			BinaryName:   BinaryName,
			Arch:         archInfo,
			ChecksumFile: "checksums.sha256",
		}

		checksums, _ := download.FetchChecksumsForBinary(binCfg)

		tmpPath, err := download.DownloadBinary(binCfg, tui.PrintProgress)
		tui.ClearLine()
		if err != nil {
			return nil, fmt.Errorf("download failed: %w", err)
		}

		if checksums.SHA256 != "" {
			if err := download.VerifyChecksums(tmpPath, checksums); err != nil {
				os.Remove(tmpPath)
				return nil, fmt.Errorf("checksum verification failed: %w", err)
			}
			tui.PrintStatus("Checksum verified")
		}

		if err := download.InstallBinaryAs(tmpPath, BinaryName); err != nil {
			return nil, fmt.Errorf("installation failed: %w", err)
		}
	}
	tui.PrintStatus("dnstt-server binary installed")

	// Step 2: Create dnstt user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating dnstt user...")
	if err := system.CreateDnsttUser(); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	tui.PrintStatus("User 'dnstt' created")

	// Step 3: Generate keys
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Generating cryptographic keys...")

	var publicKey string
	var err error
	if KeysExist(cfg.PrivateKeyFile, cfg.PublicKeyFile) {
		var regenerate bool
		huh.NewConfirm().
			Title("Keys already exist. Regenerate?").
			Value(&regenerate).
			Run()

		if regenerate {
			publicKey, err = GenerateKeys(cfg.PrivateKeyFile, cfg.PublicKeyFile)
			if err != nil {
				return nil, fmt.Errorf("key generation failed: %w", err)
			}
		} else {
			publicKey, _ = ReadPublicKey(cfg.PublicKeyFile)
		}
	} else {
		publicKey, err = GenerateKeys(cfg.PrivateKeyFile, cfg.PublicKeyFile)
		if err != nil {
			return nil, fmt.Errorf("key generation failed: %w", err)
		}
	}
	tui.PrintStatus("Keys generated")

	// Step 4: Save configuration
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Saving configuration...")
	if err := cfg.Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}
	tui.PrintStatus("Configuration saved")

	// Step 5: Configure firewall
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Configuring firewall...")
	if err := network.ConfigureFirewallForPort(Port); err != nil {
		tui.PrintWarning("Firewall configuration warning: " + err.Error())
	} else {
		tui.PrintStatus("Firewall configured")
	}

	if osdetect.HasIPv6() {
		network.ConfigureIPv6ForPort(Port)
		tui.PrintStatus("IPv6 rules configured")
	}

	// Step 6: Create and start systemd service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating systemd service...")

	if err := CreateService(cfg); err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	if err := SetPermissions(cfg); err != nil {
		tui.PrintWarning("Permission setting warning: " + err.Error())
	}

	if err := Enable(); err != nil {
		return nil, fmt.Errorf("failed to enable service: %w", err)
	}

	if err := Start(); err != nil {
		return nil, fmt.Errorf("failed to start service: %w", err)
	}
	tui.PrintStatus("Service started")

	// Update global config to set this as active provider
	tunnel.SetActiveProvider(tunnel.ProviderDNSTT)

	return &tunnel.InstallResult{
		PublicKey:     publicKey,
		Domain:        cfg.NSSubdomain,
		TunnelMode:    cfg.TunnelMode,
		MTU:           cfg.MTU,
		MTProxySecret: cfg.MTProxySecret,
	}, nil
}

// Uninstall removes DNSTT.
func (p *Provider) Uninstall() error {
	totalSteps := 5
	currentStep := 0

	// Step 1: Stop and remove service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Stopping and removing service...")
	if IsActive() {
		Stop()
	}
	if IsEnabled() {
		Disable()
	}
	Remove()
	tui.PrintStatus("Service removed")

	// Step 2: Remove binary
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing dnstt-server binary...")
	download.RemoveBinaryByName(BinaryName)
	tui.PrintStatus("Binary removed")

	// Step 3: Remove configuration and keys
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing configuration and keys...")
	RemoveAll()
	tui.PrintStatus("Configuration removed")

	// Step 4: Remove firewall rules
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing firewall rules...")
	network.RemoveFirewallRulesForPort(Port)
	tui.PrintStatus("Firewall rules removed")

	// Step 5: Remove dnstt user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing dnstt user...")
	system.RemoveDnsttUser()
	tui.PrintStatus("User removed")

	return nil
}

// Start starts the DNSTT service.
func (p *Provider) Start() error {
	return Start()
}

// Stop stops the DNSTT service.
func (p *Provider) Stop() error {
	return Stop()
}

// Restart restarts the DNSTT service.
func (p *Provider) Restart() error {
	return Restart()
}

// GetLogs returns recent logs.
func (p *Provider) GetLogs(lines int) (string, error) {
	return GetLogs(lines)
}

// GetConfig returns the current configuration as formatted text.
func (p *Provider) GetConfig() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}

	result := cfg.GetFormattedConfig()

	if cfg.PublicKeyFile != "" {
		publicKey, err := ReadPublicKey(cfg.PublicKeyFile)
		if err == nil {
			result += fmt.Sprintf("\nPublic Key (for client):\n%s\n", publicKey)
		}
	}

	return result, nil
}

// GetServiceStatus returns the systemctl status output.
func (p *Provider) GetServiceStatus() (string, error) {
	return GetStatus()
}

// ServiceName returns the systemd service name.
func (p *Provider) ServiceName() string {
	return ServiceName
}

// ConfigDir returns the configuration directory path.
func (p *Provider) ConfigDir() string {
	return ConfigDir
}

// RunInteractiveInstall runs the interactive installation wizard.
func (p *Provider) RunInteractiveInstall() (*tunnel.InstallResult, error) {
	cfg, err := Load()
	if err != nil {
		cfg = &Config{
			MTU:        "1232",
			TunnelMode: "ssh",
		}
	}

	fmt.Println()
	tui.PrintInfo("Starting DNSTT installation wizard...")
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
			return nil, err
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
			return nil, err
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
			huh.NewOption("SOCKS Proxy (Legacy)", "socks"),
			huh.NewOption("MTProto Proxy (Telegram)", "mtproto"),
		).
		Value(&tunnelMode).
		Run()
	if err != nil {
		return nil, err
	}

	// Warn about SOCKS mode fingerprinting
	if tunnelMode == "socks" {
		tui.PrintWarning("SOCKS mode has more obvious fingerprints on network traffic.")
		tui.PrintWarning("It is recommended only for temporary use or testing/debugging.")
		fmt.Println()

		confirmSocks := false
		err = huh.NewConfirm().
			Title("Are you sure you want to use SOCKS mode?").
			Affirmative("Yes").
			Negative("No").
			Value(&confirmSocks).
			Run()
		if err != nil {
			return nil, err
		}
		if !confirmSocks {
			tunnelMode = "ssh"
			tui.PrintInfo("Switched to SSH tunnel mode")
		}
	}
	cfg.TunnelMode = tunnelMode

	// Step 4: Set target port based on mode
	if cfg.TunnelMode == "ssh" {
		cfg.TargetPort = osdetect.DetectSSHPort()
	} else if cfg.TunnelMode == "socks" {
		cfg.TargetPort = "1080"
	} else if cfg.TunnelMode == "mtproto" {
		cfg.TargetPort = "8443"
		// Generate MTProxy secret if not already set
		if cfg.MTProxySecret == "" {
			// Import mtproxy package at top of file first
			secret, err := generateMTProxySecret()
			if err != nil {
				return nil, fmt.Errorf("failed to generate MTProxy secret: %w", err)
			}
			cfg.MTProxySecret = secret
			tui.PrintSuccess(fmt.Sprintf("Generated MTProxy secret: %s", secret))
		}
	}

	// Set key file paths
	cfg.PrivateKeyFile, cfg.PublicKeyFile = GetKeyFilenames(cfg.NSSubdomain)

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
		return nil, err
	}

	if !confirm {
		tui.PrintInfo("Installation cancelled")
		return nil, nil
	}

	return p.performInstallation(cfg)
}

// generateMTProxySecret generates a random 32-character hex secret with 'dd' prefix
func generateMTProxySecret() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return "dd" + hex.EncodeToString(bytes), nil
}

func detectArch() string {
	arch := osdetect.GetArch()

	switch arch {
	case "amd64":
		return "linux-amd64"
	case "arm64":
		return "linux-arm64"
	}

	return "linux-" + arch
}

func init() {
	tunnel.Register(NewProvider())
}
