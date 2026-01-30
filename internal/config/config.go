// Package config provides configuration types and loading for dnstm.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ConfigDir  = "/etc/dnstm"
	ConfigFile = "config.json"
	CertsDir   = "/etc/dnstm/certs"
	KeysDir    = "/etc/dnstm/keys"
	TunnelsDir = "/etc/dnstm/tunnels"
)

// Config is the main dnstm configuration.
type Config struct {
	Log      LogConfig       `json:"log,omitempty"`
	Listen   ListenConfig    `json:"listen,omitempty"`
	Proxy    ProxyConfig     `json:"proxy,omitempty"`
	Backends []BackendConfig `json:"backends,omitempty"`
	Tunnels  []TunnelConfig  `json:"tunnels,omitempty"`
	Route    RouteConfig     `json:"route,omitempty"`
}

// ProxyConfig configures the built-in SOCKS proxy (microsocks).
type ProxyConfig struct {
	Port int `json:"port,omitempty"`
}

// LogConfig configures logging behavior.
type LogConfig struct {
	Level     string `json:"level,omitempty"`
	Output    string `json:"output,omitempty"`
	Timestamp *bool  `json:"timestamp,omitempty"`
}

// ListenConfig configures the DNS listener.
type ListenConfig struct {
	Address string `json:"address,omitempty"`
}

// RouteConfig configures routing mode and active tunnel.
type RouteConfig struct {
	Mode    string `json:"mode,omitempty"`
	Active  string `json:"active,omitempty"`
	Default string `json:"default,omitempty"`
}

// Load reads the configuration from disk.
func Load() (*Config, error) {
	return LoadFromPath(filepath.Join(ConfigDir, ConfigFile))
}

// LoadFromPath reads the configuration from a specific path.
func LoadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
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
	return c.SaveToPath(filepath.Join(ConfigDir, ConfigFile))
}

// SaveToPath writes the configuration to a specific path.
func (c *Config) SaveToPath(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0640); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Default returns a default configuration.
func Default() *Config {
	return &Config{
		Log: LogConfig{
			Level: "info",
		},
		Listen: ListenConfig{
			Address: "0.0.0.0:53",
		},
		Backends: []BackendConfig{},
		Tunnels:  []TunnelConfig{},
		Route: RouteConfig{
			Mode: "single",
		},
	}
}

// IsSingleMode returns true if running in single-tunnel mode.
func (c *Config) IsSingleMode() bool {
	return c.Route.Mode == "" || c.Route.Mode == "single"
}

// IsMultiMode returns true if running in multi-tunnel mode.
func (c *Config) IsMultiMode() bool {
	return c.Route.Mode == "multi"
}

// GetBackendByTag returns a backend by its tag.
func (c *Config) GetBackendByTag(tag string) *BackendConfig {
	for i := range c.Backends {
		if c.Backends[i].Tag == tag {
			return &c.Backends[i]
		}
	}
	return nil
}

// GetTunnelByTag returns a tunnel by its tag.
func (c *Config) GetTunnelByTag(tag string) *TunnelConfig {
	for i := range c.Tunnels {
		if c.Tunnels[i].Tag == tag {
			return &c.Tunnels[i]
		}
	}
	return nil
}

// GetActiveTunnel returns the active tunnel tag in single mode.
func (c *Config) GetActiveTunnel() string {
	if c.IsSingleMode() {
		return c.Route.Active
	}
	return c.Route.Default
}

// SetActiveTunnel sets the active tunnel in single mode.
func (c *Config) SetActiveTunnel(tag string) error {
	if tag != "" {
		if c.GetTunnelByTag(tag) == nil {
			return fmt.Errorf("tunnel '%s' does not exist", tag)
		}
	}
	c.Route.Active = tag
	return nil
}

// GetEnabledTunnels returns all enabled tunnels.
func (c *Config) GetEnabledTunnels() []*TunnelConfig {
	var tunnels []*TunnelConfig
	for i := range c.Tunnels {
		if c.Tunnels[i].IsEnabled() {
			tunnels = append(tunnels, &c.Tunnels[i])
		}
	}
	return tunnels
}

// GetTunnelsUsingBackend returns tunnels that reference a specific backend.
func (c *Config) GetTunnelsUsingBackend(backendTag string) []*TunnelConfig {
	var tunnels []*TunnelConfig
	for i := range c.Tunnels {
		if c.Tunnels[i].Backend == backendTag {
			tunnels = append(tunnels, &c.Tunnels[i])
		}
	}
	return tunnels
}

// ConfigExists checks if the config file exists.
func ConfigExists() bool {
	_, err := os.Stat(filepath.Join(ConfigDir, ConfigFile))
	return err == nil
}

// GetConfigPath returns the path to the config file.
func GetConfigPath() string {
	return filepath.Join(ConfigDir, ConfigFile)
}
