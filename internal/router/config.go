package router

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/net2share/dnstm/internal/types"
	"gopkg.in/yaml.v3"
)

const (
	ConfigDir  = "/etc/dnstm"
	ConfigFile = "config.yaml"
	CertsDir   = "/etc/dnstm/certs"
	KeysDir    = "/etc/dnstm/keys"
)

// Mode defines the operating mode of dnstm.
type Mode string

const (
	// ModeSingle runs one tunnel at a time with iptables NAT redirect.
	ModeSingle Mode = "single"
	// ModeMulti runs multiple tunnels with domain-based DNS routing.
	ModeMulti Mode = "multi"
)

// Re-export types for convenience
type TransportType = types.TransportType
type TransportConfig = types.TransportConfig
type ShadowsocksConfig = types.ShadowsocksConfig
type DNSTTConfig = types.DNSTTConfig
type TargetConfig = types.TargetConfig

const (
	TypeSlipstreamShadowsocks = types.TypeSlipstreamShadowsocks
	TypeSlipstreamSocks       = types.TypeSlipstreamSocks
	TypeSlipstreamSSH         = types.TypeSlipstreamSSH
	TypeDNSTTSocks            = types.TypeDNSTTSocks
	TypeDNSTTSSH              = types.TypeDNSTTSSH
)

// Config is the main router configuration.
type Config struct {
	Version      string                            `yaml:"version"`
	Mode         Mode                              `yaml:"mode"`
	Single       SingleConfig                      `yaml:"single,omitempty"`
	Listen       ListenConfig                      `yaml:"listen"`
	Proxy        ProxyConfig                       `yaml:"proxy,omitempty"`
	Certificates map[string]*CertConfig            `yaml:"certificates,omitempty"`
	Transports   map[string]*types.TransportConfig `yaml:"transports,omitempty"`
	Routing      RoutingConfig                     `yaml:"routing"`
}

// ProxyConfig holds settings for the local SOCKS proxy (microsocks).
type ProxyConfig struct {
	Port int `yaml:"port,omitempty"`
}

// SingleConfig holds settings for single-tunnel mode.
type SingleConfig struct {
	Active string `yaml:"active,omitempty"`
}

// ListenConfig configures the DNS listener.
type ListenConfig struct {
	Address string `yaml:"address"`
}

// CertConfig holds certificate paths and fingerprint.
type CertConfig struct {
	Cert        string `yaml:"cert"`
	Key         string `yaml:"key"`
	Fingerprint string `yaml:"fingerprint,omitempty"`
}

// RoutingConfig configures domain routing.
type RoutingConfig struct {
	Default string `yaml:"default,omitempty"`
}

// Load reads the configuration from disk.
func Load() (*Config, error) {
	configPath := filepath.Join(ConfigDir, ConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Ensure maps are initialized
	if cfg.Certificates == nil {
		cfg.Certificates = make(map[string]*CertConfig)
	}
	if cfg.Transports == nil {
		cfg.Transports = make(map[string]*types.TransportConfig)
	}

	// Default to single mode for backward compatibility
	if cfg.Mode == "" {
		cfg.Mode = ModeSingle
	}

	return &cfg, nil
}

// LoadOrDefault reads the configuration from disk, or returns a default config if not found.
func LoadOrDefault() (*Config, error) {
	cfg, err := Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, err
	}
	return cfg, nil
}

// Save writes the configuration to disk.
func (c *Config) Save() error {
	// Use 0755 to allow dnstm user to traverse the directory
	if err := os.MkdirAll(ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(ConfigDir, ConfigFile)
	if err := os.WriteFile(configPath, data, 0640); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("version is required")
	}

	// Validate mode
	if c.Mode != "" && c.Mode != ModeSingle && c.Mode != ModeMulti {
		return fmt.Errorf("mode must be '%s' or '%s'", ModeSingle, ModeMulti)
	}

	// In single mode, validate active instance exists
	if c.Mode == ModeSingle && c.Single.Active != "" {
		if _, ok := c.Transports[c.Single.Active]; !ok {
			return fmt.Errorf("single.active instance '%s' does not exist", c.Single.Active)
		}
	}

	// Listen address only required in multi mode
	if c.Mode == ModeMulti && c.Listen.Address == "" {
		return fmt.Errorf("listen address is required in multi mode")
	}

	usedPorts := make(map[int]string)
	for name, transport := range c.Transports {
		if transport.Type == "" {
			return fmt.Errorf("transport %s: type is required", name)
		}

		if transport.Domain == "" {
			return fmt.Errorf("transport %s: domain is required", name)
		}

		if transport.Port == 0 {
			return fmt.Errorf("transport %s: port is required", name)
		}

		if transport.Port < 1024 || transport.Port > 65535 {
			return fmt.Errorf("transport %s: port must be between 1024 and 65535", name)
		}

		if existing, ok := usedPorts[transport.Port]; ok {
			return fmt.Errorf("transport %s: port %d already used by %s", name, transport.Port, existing)
		}
		usedPorts[transport.Port] = name

		if err := validateTransportConfig(name, transport); err != nil {
			return err
		}
	}

	if c.Routing.Default != "" {
		if _, ok := c.Transports[c.Routing.Default]; !ok {
			return fmt.Errorf("routing default '%s' does not exist", c.Routing.Default)
		}
	}

	return nil
}

func validateTransportConfig(name string, cfg *types.TransportConfig) error {
	switch cfg.Type {
	case types.TypeSlipstreamShadowsocks:
		if cfg.Shadowsocks == nil {
			return fmt.Errorf("transport %s: shadowsocks config is required for type %s", name, cfg.Type)
		}
		if cfg.Shadowsocks.Password == "" {
			return fmt.Errorf("transport %s: shadowsocks password is required", name)
		}
	case types.TypeSlipstreamSocks, types.TypeSlipstreamSSH:
		if cfg.Target == nil {
			return fmt.Errorf("transport %s: target config is required for type %s", name, cfg.Type)
		}
		if cfg.Target.Address == "" {
			return fmt.Errorf("transport %s: target address is required", name)
		}
	case types.TypeDNSTTSocks, types.TypeDNSTTSSH:
		if cfg.Target == nil {
			return fmt.Errorf("transport %s: target config is required for type %s", name, cfg.Type)
		}
		if cfg.Target.Address == "" {
			return fmt.Errorf("transport %s: target address is required", name)
		}
	default:
		return fmt.Errorf("transport %s: unknown type %s", name, cfg.Type)
	}

	return nil
}

// Default returns a default configuration.
func Default() *Config {
	return &Config{
		Version: "1",
		Mode:    ModeSingle,
		Single:  SingleConfig{},
		Listen: ListenConfig{
			Address: "0.0.0.0:53",
		},
		Certificates: make(map[string]*CertConfig),
		Transports:   make(map[string]*types.TransportConfig),
		Routing:      RoutingConfig{},
	}
}

// IsSingleMode returns true if running in single-tunnel mode.
func (c *Config) IsSingleMode() bool {
	return c.Mode == "" || c.Mode == ModeSingle
}

// IsMultiMode returns true if running in multi-tunnel mode.
func (c *Config) IsMultiMode() bool {
	return c.Mode == ModeMulti
}

// GetActiveInstance returns the active instance name in single mode.
// In multi mode, returns the default route.
func (c *Config) GetActiveInstance() string {
	if c.IsSingleMode() {
		return c.Single.Active
	}
	return c.Routing.Default
}

// SetActiveInstance sets the active instance in single mode.
func (c *Config) SetActiveInstance(name string) error {
	if name != "" {
		if _, ok := c.Transports[name]; !ok {
			return fmt.Errorf("instance '%s' does not exist", name)
		}
	}
	c.Single.Active = name
	return nil
}

// ConfigExists checks if the config file exists.
func ConfigExists() bool {
	configPath := filepath.Join(ConfigDir, ConfigFile)
	_, err := os.Stat(configPath)
	return err == nil
}

// GetConfigPath returns the path to the config file.
func GetConfigPath() string {
	return filepath.Join(ConfigDir, ConfigFile)
}

// IsSlipstreamType returns true if the transport type uses Slipstream.
func IsSlipstreamType(t TransportType) bool {
	return types.IsSlipstreamType(t)
}

// IsDNSTTType returns true if the transport type uses DNSTT.
func IsDNSTTType(t TransportType) bool {
	return types.IsDNSTTType(t)
}

// GetTransportTypeDisplayName returns a human-readable name for a transport type.
func GetTransportTypeDisplayName(t TransportType) string {
	return types.GetTransportTypeDisplayName(t)
}

// GetMicrosocksAddress returns the configured microsocks SOCKS proxy address.
// Returns empty string if not configured.
func (c *Config) GetMicrosocksAddress() string {
	if c.Proxy.Port == 0 {
		return ""
	}
	return fmt.Sprintf("127.0.0.1:%d", c.Proxy.Port)
}
