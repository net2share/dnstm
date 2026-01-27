// Package tunnel provides a common interface for DNS tunnel providers.
package tunnel

// ProviderType identifies a tunnel provider.
type ProviderType string

const (
	ProviderDNSTT      ProviderType = "dnstt"
	ProviderSlipstream ProviderType = "slipstream"
)

// ProviderStatus represents the installation and runtime status of a provider.
type ProviderStatus struct {
	Installed   bool
	Running     bool
	Enabled     bool
	Active      bool // Is this the active DNS handler?
	ConfigValid bool
}

// InstallConfig contains common installation parameters.
type InstallConfig struct {
	Domain     string // NS subdomain for DNSTT, domain for Slipstream
	TunnelMode string // "ssh" or "socks"
	TargetPort string // Target port (e.g., 22 for SSH, 1080 for SOCKS)
	MTU        string // MTU value (DNSTT only)
}

// InstallResult contains information about a completed installation.
type InstallResult struct {
	PublicKey     string // Hex-encoded public key (DNSTT)
	Fingerprint   string // Certificate fingerprint (Slipstream)
	Domain        string
	TunnelMode    string
	MTU           string // DNSTT only
	MTProxySecret string // MTProxy secret (for mtproto mode)
}

// Provider defines the interface that all tunnel providers must implement.
type Provider interface {
	// Name returns the provider identifier.
	Name() ProviderType

	// DisplayName returns a human-readable name for display.
	DisplayName() string

	// Port returns the port this provider listens on.
	Port() string

	// Status returns the current status of the provider.
	Status() (*ProviderStatus, error)

	// IsInstalled checks if the provider is installed.
	IsInstalled() bool

	// Install performs the installation with the given configuration.
	Install(cfg *InstallConfig) (*InstallResult, error)

	// Uninstall removes the provider.
	Uninstall() error

	// Start starts the provider's service.
	Start() error

	// Stop stops the provider's service.
	Stop() error

	// Restart restarts the provider's service.
	Restart() error

	// GetLogs returns recent logs from the provider's service.
	GetLogs(lines int) (string, error)

	// GetConfig returns the current configuration as formatted text.
	GetConfig() (string, error)

	// GetServiceStatus returns the systemctl status output.
	GetServiceStatus() (string, error)

	// ServiceName returns the systemd service name.
	ServiceName() string

	// ConfigDir returns the configuration directory path.
	ConfigDir() string
}
