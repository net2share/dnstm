package slipstream

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/download"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/tunnel"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

// Provider implements the tunnel.Provider interface for Slipstream.
type Provider struct{}

// NewProvider creates a new Slipstream provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the provider identifier.
func (p *Provider) Name() tunnel.ProviderType {
	return tunnel.ProviderSlipstream
}

// DisplayName returns a human-readable name.
func (p *Provider) DisplayName() string {
	return "Slipstream"
}

// Port returns the port this provider listens on.
func (p *Provider) Port() string {
	return Port
}

// Status returns the current status of the provider.
func (p *Provider) Status() (*tunnel.ProviderStatus, error) {
	globalCfg, _ := tunnel.LoadGlobalConfig()
	isActive := globalCfg != nil && globalCfg.ActiveProvider == tunnel.ProviderSlipstream

	return &tunnel.ProviderStatus{
		Installed:   p.IsInstalled(),
		Running:     IsActive(),
		Enabled:     IsEnabled(),
		Active:      isActive,
		ConfigValid: Exists(),
	}, nil
}

// IsInstalled checks if Slipstream is installed.
func (p *Provider) IsInstalled() bool {
	return download.IsBinaryInstalled(BinaryName) && IsServiceInstalled()
}

// Install performs the Slipstream installation.
func (p *Provider) Install(cfg *tunnel.InstallConfig) (*tunnel.InstallResult, error) {
	// Convert tunnel.InstallConfig to slipstream.Config
	slipCfg := &Config{
		Domain:        cfg.Domain,
		DNSListenPort: Port,
		TunnelMode:    cfg.TunnelMode,
		TargetPort:    cfg.TargetPort,
	}

	slipCfg.CertFile, slipCfg.KeyFile = GetCertFilenames(slipCfg.Domain)
	slipCfg.TargetAddress = "127.0.0.1:" + slipCfg.TargetPort

	return p.performInstallation(slipCfg)
}

func (p *Provider) performInstallation(cfg *Config) (*tunnel.InstallResult, error) {
	archInfo := detectArch()

	totalSteps := 6
	currentStep := 0

	// Step 1: Download slipstream-server
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Downloading slipstream-server binary...")

	if !download.IsBinaryInstalled(BinaryName) {
		binCfg := &download.BinaryConfig{
			BaseURL:      ReleaseURL,
			BinaryName:   BinaryName,
			Arch:         archInfo,
			ChecksumFile: "SHA256SUMS",
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
	tui.PrintStatus("slipstream-server binary installed")

	// Step 2: Create slipstream user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating slipstream user...")
	if err := system.CreateSlipstreamUser(); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	tui.PrintStatus("User 'slipstream' created")

	// Step 3: Generate TLS certificate
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Generating TLS certificate...")

	var fingerprint string
	var err error
	if CertsExist(cfg.CertFile, cfg.KeyFile) {
		var regenerate bool
		huh.NewConfirm().
			Title("Certificates already exist. Regenerate?").
			Value(&regenerate).
			Run()

		if regenerate {
			fingerprint, err = GenerateCertificate(cfg.CertFile, cfg.KeyFile, cfg.Domain)
			if err != nil {
				return nil, fmt.Errorf("certificate generation failed: %w", err)
			}
		} else {
			fingerprint, _ = ReadCertificateFingerprint(cfg.CertFile)
		}
	} else {
		fingerprint, err = GenerateCertificate(cfg.CertFile, cfg.KeyFile, cfg.Domain)
		if err != nil {
			return nil, fmt.Errorf("certificate generation failed: %w", err)
		}
	}
	tui.PrintStatus("TLS certificate generated")

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
	tunnel.SetActiveProvider(tunnel.ProviderSlipstream)

	return &tunnel.InstallResult{
		Fingerprint:   fingerprint,
		Domain:        cfg.Domain,
		TunnelMode:    cfg.TunnelMode,
		MTProxySecret: cfg.MTProxySecret,
	}, nil
}

// Uninstall removes Slipstream.
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
	tui.PrintStep(currentStep, totalSteps, "Removing slipstream-server binary...")
	download.RemoveBinaryByName(BinaryName)
	tui.PrintStatus("Binary removed")

	// Step 3: Remove configuration and certificates
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing configuration and certificates...")
	RemoveAll()
	tui.PrintStatus("Configuration removed")

	// Step 4: Remove firewall rules
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing firewall rules...")
	network.RemoveFirewallRulesForPort(Port)
	tui.PrintStatus("Firewall rules removed")

	// Step 5: Remove slipstream user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing slipstream user...")
	system.RemoveSlipstreamUser()
	tui.PrintStatus("User removed")

	return nil
}

// Start starts the Slipstream service.
func (p *Provider) Start() error {
	return Start()
}

// Stop stops the Slipstream service.
func (p *Provider) Stop() error {
	return Stop()
}

// Restart restarts the Slipstream service.
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

	if cfg.CertFile != "" {
		fingerprint, err := ReadCertificateFingerprint(cfg.CertFile)
		if err == nil {
			result += fmt.Sprintf("\nCertificate SHA256 Fingerprint:\n%s\n", FormatFingerprint(fingerprint))
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
			DNSListenPort: Port,
			TunnelMode:    "ssh",
		}
	}

	fmt.Println()
	tui.PrintInfo("Starting Slipstream installation wizard...")
	fmt.Println()

	// Step 1: Get domain (loop until valid)
	currentDomain := cfg.Domain
	for {
		var domain string
		input := huh.NewInput().
			Title("Domain").
			Value(&domain)
		if currentDomain != "" {
			input.Description("Current: " + currentDomain + " (press Enter to keep)").
				Placeholder(currentDomain)
		} else {
			input.Description("e.g., t.example.com")
		}
		err = input.Run()
		if err != nil {
			return nil, err
		}
		if domain == "" {
			if currentDomain != "" {
				domain = currentDomain
			} else {
				tui.PrintError("Domain is required")
				continue
			}
		}
		cfg.Domain = domain
		break
	}

	// Step 2: Get tunnel mode (pre-select current if reconfiguring)
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

	// Step 3: Set target port based on mode
	if cfg.TunnelMode == "ssh" {
		cfg.TargetPort = osdetect.DetectSSHPort()
	} else if cfg.TunnelMode == "socks" {
		cfg.TargetPort = "1080"
	} else if cfg.TunnelMode == "mtproto" {
		cfg.TargetPort = "8443"
		// Generate MTProxy secret if not already set
		if cfg.MTProxySecret == "" {
			secret, err := generateMTProxySecret()
			if err != nil {
				return nil, fmt.Errorf("failed to generate MTProxy secret: %w", err)
			}
			cfg.MTProxySecret = secret
			tui.PrintSuccess(fmt.Sprintf("Generated MTProxy secret: %s", secret))
		}
	}

	cfg.TargetAddress = "127.0.0.1:" + cfg.TargetPort

	// Set cert file paths
	cfg.CertFile, cfg.KeyFile = GetCertFilenames(cfg.Domain)

	// Show summary before confirmation
	summaryLines := []string{
		tui.KV("Domain:        ", cfg.Domain),
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
