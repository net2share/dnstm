package dnsrouter

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the DNS router configuration.
type Config struct {
	Listen        string        `yaml:"listen"`
	ForwarderType string        `yaml:"forwarder_type,omitempty"` // "native", "coredns", "ebpf" (default: "native")
	Timeout       string        `yaml:"timeout,omitempty"`
	Routes        []RouteConfig `yaml:"routes"`
	Default       string        `yaml:"default,omitempty"`
}

// RouteConfig is a single route configuration.
type RouteConfig struct {
	Domain  string `yaml:"domain"`
	Backend string `yaml:"backend"`
}

// RouteInput is used to build routes without depending on transport types.
type RouteInput struct {
	Domain  string
	Backend string
}

// LoadConfig loads the DNS router configuration.
func LoadConfig() (*Config, error) {
	configPath := filepath.Join(ConfigDir, ConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves the DNS router configuration.
func SaveConfig(cfg *Config) error {
	// Use 0755 to allow dnstm user to traverse
	if err := os.MkdirAll(ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(ConfigDir, ConfigFile)
	// Use 0644 to allow dnstm user to read
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GenerateConfig generates a DNS router config from route inputs.
// The listenAddr should already be resolved (caller handles external IP detection).
// Routes should already have proper backend addresses (e.g., "127.0.0.1:5300").
func GenerateConfig(listenAddr string, routes []RouteInput, defaultBackend string) *Config {
	cfg := &Config{
		Listen:  listenAddr,
		Routes:  make([]RouteConfig, 0, len(routes)),
		Default: defaultBackend,
	}

	for _, route := range routes {
		cfg.Routes = append(cfg.Routes, RouteConfig{
			Domain:  route.Domain,
			Backend: route.Backend,
		})
	}

	return cfg
}

// ToRoutes converts the config to Route slice for the router.
func (c *Config) ToRoutes() []Route {
	routes := make([]Route, len(c.Routes))
	for i, r := range c.Routes {
		routes[i] = Route{
			Domain:  r.Domain,
			Backend: r.Backend,
		}
	}
	return routes
}
