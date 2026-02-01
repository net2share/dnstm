package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/net2share/dnstm/internal/config"
)

func TestConfigExport(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.ConfigWithTunnel()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Read the config file directly
	data, err := os.ReadFile(filepath.Join(env.ConfigDir, "config.json"))
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}

	// Verify key fields exist
	if _, ok := parsed["backends"]; !ok {
		t.Error("expected 'backends' in exported config")
	}

	if _, ok := parsed["tunnels"]; !ok {
		t.Error("expected 'tunnels' in exported config")
	}

	if _, ok := parsed["route"]; !ok {
		t.Error("expected 'route' in exported config")
	}
}

func TestConfigValidate_Valid(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.ConfigWithTunnel()
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config failed validation: %v", err)
	}
}

func TestConfigValidate_Empty(t *testing.T) {
	cfg := &config.Config{}

	// Empty config should be valid (just no backends/tunnels)
	if err := cfg.Validate(); err != nil {
		t.Errorf("empty config failed validation: %v", err)
	}
}

func TestConfigLoad_Valid(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Verify loaded config matches what was written
	if loaded.Listen.Address != cfg.Listen.Address {
		t.Errorf("Listen.Address = %q, want %q", loaded.Listen.Address, cfg.Listen.Address)
	}

	if loaded.Route.Mode != cfg.Route.Mode {
		t.Errorf("Route.Mode = %q, want %q", loaded.Route.Mode, cfg.Route.Mode)
	}
}

func TestConfigLoad_InvalidJSON(t *testing.T) {
	env := NewTestEnv(t)

	// Write invalid JSON
	configPath := filepath.Join(env.ConfigDir, "config.json")
	if err := os.WriteFile(configPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	_, err := env.ReadConfig()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConfigSerialization(t *testing.T) {
	env := NewTestEnv(t)

	// Create a complex config
	cfg := &config.Config{
		Log: config.LogConfig{
			Level:  "debug",
			Output: "file",
		},
		Listen: config.ListenConfig{
			Address: "0.0.0.0:53",
		},
		Proxy: config.ProxyConfig{
			Port: 1080,
		},
		Route: config.RouteConfig{
			Mode:    "multi",
			Active:  "tunnel-a",
			Default: "tunnel-a",
		},
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
			{
				Tag:  "ss",
				Type: config.BackendShadowsocks,
				Shadowsocks: &config.ShadowsocksConfig{
					Method:   "aes-256-gcm",
					Password: "secret123",
				},
			},
		},
		Tunnels: []config.TunnelConfig{
			{
				Tag:       "tunnel-a",
				Transport: config.TransportSlipstream,
				Backend:   "socks",
				Domain:    "a.example.com",
				Port:      5310,
				Slipstream: &config.SlipstreamConfig{
					Cert: "/path/to/cert",
					Key:  "/path/to/key",
				},
			},
			{
				Tag:       "tunnel-b",
				Transport: config.TransportDNSTT,
				Backend:   "socks",
				Domain:    "b.example.com",
				Port:      5311,
				DNSTT: &config.DNSTTConfig{
					MTU:        1200,
					PrivateKey: "/path/to/key",
				},
			},
		},
	}

	// Write and read back
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Verify all fields
	if loaded.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want 'debug'", loaded.Log.Level)
	}

	if loaded.Route.Mode != "multi" {
		t.Errorf("Route.Mode = %q, want 'multi'", loaded.Route.Mode)
	}

	if len(loaded.Backends) != 2 {
		t.Errorf("len(Backends) = %d, want 2", len(loaded.Backends))
	}

	if len(loaded.Tunnels) != 2 {
		t.Errorf("len(Tunnels) = %d, want 2", len(loaded.Tunnels))
	}

	// Verify shadowsocks config
	ss := loaded.GetBackendByTag("ss")
	if ss == nil {
		t.Fatal("ss backend not found")
	}
	if ss.Shadowsocks == nil {
		t.Fatal("ss.Shadowsocks is nil")
	}
	if ss.Shadowsocks.Password != "secret123" {
		t.Errorf("ss.Shadowsocks.Password = %q, want 'secret123'", ss.Shadowsocks.Password)
	}

	// Verify DNSTT config
	dnstt := loaded.GetTunnelByTag("tunnel-b")
	if dnstt == nil {
		t.Fatal("tunnel-b not found")
	}
	if dnstt.DNSTT == nil {
		t.Fatal("dnstt.DNSTT is nil")
	}
	if dnstt.DNSTT.MTU != 1200 {
		t.Errorf("dnstt.DNSTT.MTU = %d, want 1200", dnstt.DNSTT.MTU)
	}
}

func TestConfigApplyDefaults(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{
				Tag:       "tunnel-a",
				Transport: config.TransportSlipstream,
				Backend:   "socks",
				Domain:    "a.example.com",
				// No port, no enabled
			},
			{
				Tag:       "tunnel-b",
				Transport: config.TransportDNSTT,
				Backend:   "socks",
				Domain:    "b.example.com",
				// No port, no enabled, no DNSTT config
			},
		},
	}

	cfg.ApplyDefaults()

	// Verify log defaults
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want 'info'", cfg.Log.Level)
	}

	// Verify listen defaults
	if cfg.Listen.Address != "0.0.0.0:53" {
		t.Errorf("Listen.Address = %q, want '0.0.0.0:53'", cfg.Listen.Address)
	}

	// Verify route defaults
	if cfg.Route.Mode != "single" {
		t.Errorf("Route.Mode = %q, want 'single'", cfg.Route.Mode)
	}

	// Verify tunnel defaults
	for _, tunnel := range cfg.Tunnels {
		if tunnel.Port == 0 {
			t.Errorf("Tunnel %q: port should be auto-allocated", tunnel.Tag)
		}
		if tunnel.Enabled == nil || !*tunnel.Enabled {
			t.Errorf("Tunnel %q: should be enabled by default", tunnel.Tag)
		}
	}

	// Verify DNSTT defaults
	dnstt := cfg.GetTunnelByTag("tunnel-b")
	if dnstt.DNSTT == nil {
		t.Fatal("DNSTT config should be created")
	}
	if dnstt.DNSTT.MTU != 1232 {
		t.Errorf("DNSTT.MTU = %d, want 1232", dnstt.DNSTT.MTU)
	}
}

func TestConfigEnsureBuiltinBackends(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Port: 1080},
	}

	cfg.EnsureBuiltinBackends()

	// Should have socks and ssh
	socks := cfg.GetBackendByTag("socks")
	if socks == nil {
		t.Fatal("socks backend not created")
	}
	if socks.Type != config.BackendSOCKS {
		t.Errorf("socks.Type = %v, want %v", socks.Type, config.BackendSOCKS)
	}
	if socks.Address != "127.0.0.1:1080" {
		t.Errorf("socks.Address = %q, want '127.0.0.1:1080'", socks.Address)
	}

	ssh := cfg.GetBackendByTag("ssh")
	if ssh == nil {
		t.Fatal("ssh backend not created")
	}
	if ssh.Type != config.BackendSSH {
		t.Errorf("ssh.Type = %v, want %v", ssh.Type, config.BackendSSH)
	}
}
