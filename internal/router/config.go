package router

import (
	"github.com/net2share/dnstm/internal/config"
)

// Re-export constants from config package
const (
	ConfigDir  = config.ConfigDir
	ConfigFile = config.ConfigFile
	CertsDir   = config.CertsDir
	KeysDir    = config.KeysDir
	TunnelsDir = config.TunnelsDir
)

// Mode defines the operating mode of dnstm.
type Mode string

const (
	// ModeSingle runs one tunnel at a time with direct binding to external IP.
	ModeSingle Mode = "single"
	// ModeMulti runs multiple tunnels with domain-based DNS routing.
	ModeMulti Mode = "multi"
)

// Re-export types for convenience
type Config = config.Config
type BackendConfig = config.BackendConfig
type TunnelConfig = config.TunnelConfig
type BackendType = config.BackendType
type TransportType = config.TransportType
type LogConfig = config.LogConfig
type ListenConfig = config.ListenConfig
type RouteConfig = config.RouteConfig
type ShadowsocksConfig = config.ShadowsocksConfig
type SlipstreamConfig = config.SlipstreamConfig
type DNSTTConfig = config.DNSTTConfig

// Re-export backend constants
const (
	BackendSOCKS       = config.BackendSOCKS
	BackendSSH         = config.BackendSSH
	BackendShadowsocks = config.BackendShadowsocks
	BackendCustom      = config.BackendCustom
)

// Re-export transport constants
const (
	TransportSlipstream = config.TransportSlipstream
	TransportDNSTT      = config.TransportDNSTT
)

// CertConfig holds certificate paths and fingerprint.
type CertConfig struct {
	Cert        string
	Key         string
	Fingerprint string
}

// Load reads the configuration from disk.
func Load() (*Config, error) {
	return config.Load()
}

// LoadOrDefault reads the configuration from disk, or returns a default config if not found.
func LoadOrDefault() (*Config, error) {
	return config.LoadOrDefault()
}

// Default returns a default configuration.
func Default() *Config {
	return config.Default()
}

// ConfigExists checks if the config file exists.
func ConfigExists() bool {
	return config.ConfigExists()
}

// GetConfigPath returns the path to the config file.
func GetConfigPath() string {
	return config.GetConfigPath()
}

