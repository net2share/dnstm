package shadowsocks

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/download"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/tunnel"
	slipstreamPkg "github.com/net2share/dnstm/internal/tunnel/slipstream"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

// Provider implements the tunnel.Provider interface for Shadowsocks.
type Provider struct{}

// NewProvider creates a new Shadowsocks provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the provider identifier.
func (p *Provider) Name() tunnel.ProviderType {
	return tunnel.ProviderShadowsocks
}

// DisplayName returns a human-readable name.
func (p *Provider) DisplayName() string {
	return "Shadowsocks-Slipstream (SIP003)"
}

// Port returns the port this provider listens on.
func (p *Provider) Port() string {
	return Port
}

// Status returns the current status of the provider.
func (p *Provider) Status() (*tunnel.ProviderStatus, error) {
	globalCfg, _ := tunnel.LoadGlobalConfig()
	isActive := globalCfg != nil && globalCfg.ActiveProvider == tunnel.ProviderShadowsocks

	return &tunnel.ProviderStatus{
		Installed:   p.IsInstalled(),
		Running:     IsActive(),
		Enabled:     IsEnabled(),
		Active:      isActive,
		ConfigValid: Exists(),
	}, nil
}

// IsInstalled checks if Shadowsocks is installed.
func (p *Provider) IsInstalled() bool {
	return IsSsserverInstalled() && IsServiceInstalled()
}

// Install performs the Shadowsocks installation.
func (p *Provider) Install(cfg *tunnel.InstallConfig) (*tunnel.InstallResult, error) {
	// Convert tunnel.InstallConfig to shadowsocks.Config
	ssCfg := &Config{
		Domain: cfg.Domain,
		Method: DefaultMethod,
	}

	// Generate password if not provided
	password, err := GeneratePassword()
	if err != nil {
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}
	ssCfg.Password = password

	ssCfg.CertFile, ssCfg.KeyFile = GetCertFilenames(ssCfg.Domain)

	return p.performInstallation(ssCfg)
}

func (p *Provider) performInstallation(cfg *Config) (*tunnel.InstallResult, error) {
	totalSteps := 8
	currentStep := 0

	// Step 1: Download ssserver
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Downloading ssserver binary...")

	if !IsSsserverInstalled() {
		if err := DownloadShadowsocks(tui.PrintProgress); err != nil {
			tui.ClearLine()
			return nil, fmt.Errorf("download failed: %w", err)
		}
		tui.ClearLine()
	}
	tui.PrintStatus("ssserver binary installed")

	// Step 2: Download slipstream-server if not already installed
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Checking slipstream-server binary...")

	if !download.IsBinaryInstalled(SlipstreamBin) {
		tui.PrintStep(currentStep, totalSteps, "Downloading slipstream-server binary...")

		archInfo := detectArch()
		binCfg := &download.BinaryConfig{
			BaseURL:      slipstreamPkg.ReleaseURL,
			BinaryName:   SlipstreamBin,
			Arch:         archInfo,
			ChecksumFile: "SHA256SUMS",
		}

		checksums, _ := download.FetchChecksumsForBinary(binCfg)

		tmpPath, err := download.DownloadBinary(binCfg, tui.PrintProgress)
		tui.ClearLine()
		if err != nil {
			return nil, fmt.Errorf("slipstream download failed: %w", err)
		}

		if checksums.SHA256 != "" {
			if err := download.VerifyChecksums(tmpPath, checksums); err != nil {
				os.Remove(tmpPath)
				return nil, fmt.Errorf("slipstream checksum verification failed: %w", err)
			}
			tui.PrintStatus("Slipstream checksum verified")
		}

		if err := download.InstallBinaryAs(tmpPath, SlipstreamBin); err != nil {
			return nil, fmt.Errorf("slipstream installation failed: %w", err)
		}
	}
	tui.PrintStatus("slipstream-server binary installed")

	// Step 3: Create shadowsocks user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating dnstm user...")
	if err := system.CreateShadowsocksUser(); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	tui.PrintStatus("User 'dnstm' created")

	// Step 4: Generate TLS certificate
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Generating TLS certificate...")

	var fingerprint string
	var err error
	if slipstreamPkg.CertsExist(cfg.CertFile, cfg.KeyFile) {
		var regenerate bool
		huh.NewConfirm().
			Title("Certificates already exist. Regenerate?").
			Value(&regenerate).
			Run()

		if regenerate {
			fingerprint, err = slipstreamPkg.GenerateCertificate(cfg.CertFile, cfg.KeyFile, cfg.Domain)
			if err != nil {
				return nil, fmt.Errorf("certificate generation failed: %w", err)
			}
		} else {
			fingerprint, _ = slipstreamPkg.ReadCertificateFingerprint(cfg.CertFile)
		}
	} else {
		fingerprint, err = slipstreamPkg.GenerateCertificate(cfg.CertFile, cfg.KeyFile, cfg.Domain)
		if err != nil {
			return nil, fmt.Errorf("certificate generation failed: %w", err)
		}
	}
	tui.PrintStatus("TLS certificate generated")

	// Step 5: Generate password if not set
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Generating secure password...")
	if cfg.Password == "" {
		password, err := GeneratePassword()
		if err != nil {
			return nil, fmt.Errorf("failed to generate password: %w", err)
		}
		cfg.Password = password
	}
	tui.PrintStatus("Password configured")

	// Step 6: Save configuration
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Saving configuration...")
	if err := cfg.Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}
	if err := cfg.WriteSSConfig(); err != nil {
		return nil, fmt.Errorf("failed to write shadowsocks config: %w", err)
	}
	tui.PrintStatus("Configuration saved")

	// Step 7: Configure firewall
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

	// Step 8: Create and start systemd service
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
	tunnel.SetActiveProvider(tunnel.ProviderShadowsocks)

	return &tunnel.InstallResult{
		Fingerprint: fingerprint,
		Domain:      cfg.Domain,
		TunnelMode:  "shadowsocks",
	}, nil
}

// Uninstall removes Shadowsocks.
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

	// Step 2: Remove ssserver binary
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing ssserver binary...")
	RemoveSsserver()
	tui.PrintStatus("Binary removed")

	// Step 3: Remove configuration
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing configuration...")
	RemoveAll()
	tui.PrintStatus("Configuration removed")

	// Step 4: Remove firewall rules
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing firewall rules...")
	network.RemoveFirewallRulesForPort(Port)
	tui.PrintStatus("Firewall rules removed")

	// Step 5: Clean up dnstm user (only if no other providers installed)
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Cleaning up dnstm user...")
	system.RemoveDnstmUserIfOrphaned(tunnel.AnyInstalled)
	if system.DnstmUserExists() {
		tui.PrintStatus("User kept (other providers installed)")
	} else {
		tui.PrintStatus("User removed")
	}

	return nil
}

// Start starts the Shadowsocks service.
func (p *Provider) Start() error {
	return Start()
}

// Stop stops the Shadowsocks service.
func (p *Provider) Stop() error {
	return Stop()
}

// Restart restarts the Shadowsocks service.
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
		fingerprint, err := slipstreamPkg.ReadCertificateFingerprint(cfg.CertFile)
		if err == nil {
			result += fmt.Sprintf("\nCertificate SHA256 Fingerprint:\n%s\n", slipstreamPkg.FormatFingerprint(fingerprint))
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
			Method: DefaultMethod,
		}
	}

	fmt.Println()
	tui.PrintInfo("Starting Shadowsocks installation wizard...")
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

	// Step 2: Get encryption method
	method := cfg.Method
	if method == "" {
		method = DefaultMethod
	}
	err = huh.NewSelect[string]().
		Title("Encryption Method").
		Options(
			huh.NewOption("AES-256-GCM (Recommended)", "aes-256-gcm"),
			huh.NewOption("ChaCha20-IETF-Poly1305", "chacha20-ietf-poly1305"),
			huh.NewOption("AES-128-GCM", "aes-128-gcm"),
		).
		Value(&method).
		Run()
	if err != nil {
		return nil, err
	}
	cfg.Method = method

	// Step 3: Get password (optional, auto-generate by default)
	currentPassword := cfg.Password
	var password string
	input := huh.NewInput().
		Title("Password").
		Value(&password)
	if currentPassword != "" {
		input.Description("Current: (hidden) - press Enter to keep, or enter new password").
			Placeholder("(keep current)")
	} else {
		input.Description("Leave empty to auto-generate secure password")
	}
	err = input.Run()
	if err != nil {
		return nil, err
	}
	if password == "" {
		if currentPassword != "" {
			password = currentPassword
		} else {
			// Auto-generate
			password, err = GeneratePassword()
			if err != nil {
				return nil, fmt.Errorf("failed to generate password: %w", err)
			}
			tui.PrintInfo("Password will be auto-generated")
		}
	}
	cfg.Password = password

	// Set cert file paths
	cfg.CertFile, cfg.KeyFile = GetCertFilenames(cfg.Domain)

	// Show summary before confirmation
	summaryLines := []string{
		tui.KV("Domain:     ", cfg.Domain),
		tui.KV("Method:     ", cfg.Method),
		tui.KV("Password:   ", "(configured)"),
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
