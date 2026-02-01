package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_LoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create a config
	cfg := &Config{
		Listen: ListenConfig{Address: "127.0.0.1:5353"},
		Route:  RouteConfig{Mode: "single"},
		Backends: []BackendConfig{
			{Tag: "test-backend", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []TunnelConfig{
			{
				Tag:       "test-tunnel",
				Transport: TransportSlipstream,
				Backend:   "test-backend",
				Domain:    "test.example.com",
				Port:      5310,
			},
		},
	}

	// Save it
	if err := cfg.SaveToPath(configPath); err != nil {
		t.Fatalf("SaveToPath failed: %v", err)
	}

	// Load it back
	loaded, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath failed: %v", err)
	}

	// Verify fields
	if loaded.Listen.Address != cfg.Listen.Address {
		t.Errorf("Listen.Address = %q, want %q", loaded.Listen.Address, cfg.Listen.Address)
	}
	if loaded.Route.Mode != cfg.Route.Mode {
		t.Errorf("Route.Mode = %q, want %q", loaded.Route.Mode, cfg.Route.Mode)
	}
	if len(loaded.Backends) != 1 {
		t.Errorf("len(Backends) = %d, want 1", len(loaded.Backends))
	}
	if len(loaded.Tunnels) != 1 {
		t.Errorf("len(Tunnels) = %d, want 1", len(loaded.Tunnels))
	}
}

func TestConfig_LoadNonexistent(t *testing.T) {
	_, err := LoadFromPath("/nonexistent/path/config.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestConfig_LoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	if err := os.WriteFile(configPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConfig_Default(t *testing.T) {
	cfg := Default()

	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want 'info'", cfg.Log.Level)
	}
	if cfg.Listen.Address != "0.0.0.0:53" {
		t.Errorf("Listen.Address = %q, want '0.0.0.0:53'", cfg.Listen.Address)
	}
	if cfg.Route.Mode != "single" {
		t.Errorf("Route.Mode = %q, want 'single'", cfg.Route.Mode)
	}
}

func TestConfig_ModeChecks(t *testing.T) {
	tests := []struct {
		mode         string
		wantSingle   bool
		wantMulti    bool
	}{
		{"", true, false},
		{"single", true, false},
		{"multi", false, true},
	}

	for _, tt := range tests {
		cfg := &Config{Route: RouteConfig{Mode: tt.mode}}

		if got := cfg.IsSingleMode(); got != tt.wantSingle {
			t.Errorf("Mode %q: IsSingleMode() = %v, want %v", tt.mode, got, tt.wantSingle)
		}
		if got := cfg.IsMultiMode(); got != tt.wantMulti {
			t.Errorf("Mode %q: IsMultiMode() = %v, want %v", tt.mode, got, tt.wantMulti)
		}
	}
}

func TestConfig_GetBackendByTag(t *testing.T) {
	cfg := &Config{
		Backends: []BackendConfig{
			{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
			{Tag: "ssh", Type: BackendSSH, Address: "127.0.0.1:22"},
		},
	}

	// Found
	backend := cfg.GetBackendByTag("socks")
	if backend == nil {
		t.Fatal("expected to find 'socks' backend")
	}
	if backend.Type != BackendSOCKS {
		t.Errorf("backend.Type = %v, want %v", backend.Type, BackendSOCKS)
	}

	// Not found
	if cfg.GetBackendByTag("nonexistent") != nil {
		t.Error("expected nil for nonexistent backend")
	}
}

func TestConfig_GetTunnelByTag(t *testing.T) {
	cfg := &Config{
		Tunnels: []TunnelConfig{
			{Tag: "tunnel-a", Domain: "a.example.com"},
			{Tag: "tunnel-b", Domain: "b.example.com"},
		},
	}

	// Found
	tunnel := cfg.GetTunnelByTag("tunnel-a")
	if tunnel == nil {
		t.Fatal("expected to find 'tunnel-a'")
	}
	if tunnel.Domain != "a.example.com" {
		t.Errorf("tunnel.Domain = %q, want 'a.example.com'", tunnel.Domain)
	}

	// Not found
	if cfg.GetTunnelByTag("nonexistent") != nil {
		t.Error("expected nil for nonexistent tunnel")
	}
}

func TestConfig_GetEnabledTunnels(t *testing.T) {
	enabled := true
	disabled := false

	cfg := &Config{
		Tunnels: []TunnelConfig{
			{Tag: "enabled-1", Enabled: &enabled},
			{Tag: "disabled", Enabled: &disabled},
			{Tag: "enabled-2", Enabled: nil}, // nil defaults to enabled
		},
	}

	tunnels := cfg.GetEnabledTunnels()
	if len(tunnels) != 2 {
		t.Errorf("len(EnabledTunnels) = %d, want 2", len(tunnels))
	}

	tags := map[string]bool{}
	for _, tunnel := range tunnels {
		tags[tunnel.Tag] = true
	}
	if !tags["enabled-1"] || !tags["enabled-2"] {
		t.Errorf("expected enabled-1 and enabled-2, got %v", tags)
	}
}

func TestConfig_GetTunnelsUsingBackend(t *testing.T) {
	cfg := &Config{
		Tunnels: []TunnelConfig{
			{Tag: "tunnel-1", Backend: "socks"},
			{Tag: "tunnel-2", Backend: "ssh"},
			{Tag: "tunnel-3", Backend: "socks"},
		},
	}

	tunnels := cfg.GetTunnelsUsingBackend("socks")
	if len(tunnels) != 2 {
		t.Errorf("len(TunnelsUsingBackend) = %d, want 2", len(tunnels))
	}
}

func TestConfig_SetActiveTunnel(t *testing.T) {
	cfg := &Config{
		Tunnels: []TunnelConfig{
			{Tag: "tunnel-a"},
			{Tag: "tunnel-b"},
		},
	}

	// Valid tunnel
	if err := cfg.SetActiveTunnel("tunnel-a"); err != nil {
		t.Errorf("SetActiveTunnel('tunnel-a') failed: %v", err)
	}
	if cfg.Route.Active != "tunnel-a" {
		t.Errorf("Route.Active = %q, want 'tunnel-a'", cfg.Route.Active)
	}

	// Invalid tunnel
	if err := cfg.SetActiveTunnel("nonexistent"); err == nil {
		t.Error("expected error for nonexistent tunnel")
	}

	// Empty (clear active)
	if err := cfg.SetActiveTunnel(""); err != nil {
		t.Errorf("SetActiveTunnel('') failed: %v", err)
	}
}
