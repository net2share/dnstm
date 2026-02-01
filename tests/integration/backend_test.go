package integration

import (
	"strings"
	"testing"

	"github.com/net2share/dnstm/internal/config"
)

func TestBackendList_Empty(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Backends = []config.BackendConfig{} // Empty backends

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Note: This test is limited because handlers check for installed binaries
	// and use global config path. In a real integration test, we would
	// need to mock those or run in a container.

	// Verify the config was written with empty backends
	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if len(loaded.Backends) != 0 {
		t.Errorf("len(Backends) = %d, want 0", len(loaded.Backends))
	}
}

func TestBackendList_WithBackends(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Verify config was written
	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if len(loaded.Backends) != 2 {
		t.Errorf("len(Backends) = %d, want 2", len(loaded.Backends))
	}

	// Check for expected backends
	hasSocks := false
	hasSSH := false
	for _, b := range loaded.Backends {
		if b.Tag == "socks" && b.Type == config.BackendSOCKS {
			hasSocks = true
		}
		if b.Tag == "ssh" && b.Type == config.BackendSSH {
			hasSSH = true
		}
	}

	if !hasSocks {
		t.Error("expected socks backend")
	}
	if !hasSSH {
		t.Error("expected ssh backend")
	}
}

func TestBackendAdd_Shadowsocks(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Simulate adding a shadowsocks backend
	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	loaded.Backends = append(loaded.Backends, config.BackendConfig{
		Tag:  "my-ss",
		Type: config.BackendShadowsocks,
		Shadowsocks: &config.ShadowsocksConfig{
			Method:   "aes-256-gcm",
			Password: "test-password",
		},
	})

	if err := env.WriteConfig(loaded); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Reload and verify
	reloaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	ss := reloaded.GetBackendByTag("my-ss")
	if ss == nil {
		t.Fatal("shadowsocks backend not found")
	}

	if ss.Type != config.BackendShadowsocks {
		t.Errorf("ss.Type = %v, want %v", ss.Type, config.BackendShadowsocks)
	}

	if ss.Shadowsocks == nil {
		t.Fatal("shadowsocks config is nil")
	}

	if ss.Shadowsocks.Method != "aes-256-gcm" {
		t.Errorf("method = %q, want 'aes-256-gcm'", ss.Shadowsocks.Method)
	}
}

func TestBackendAdd_Custom(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	loaded.Backends = append(loaded.Backends, config.BackendConfig{
		Tag:     "my-custom",
		Type:    config.BackendCustom,
		Address: "192.168.1.100:8080",
	})

	if err := env.WriteConfig(loaded); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Reload and verify
	reloaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	custom := reloaded.GetBackendByTag("my-custom")
	if custom == nil {
		t.Fatal("custom backend not found")
	}

	if custom.Address != "192.168.1.100:8080" {
		t.Errorf("address = %q, want '192.168.1.100:8080'", custom.Address)
	}
}

func TestBackendRemove(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Backends = append(cfg.Backends, config.BackendConfig{
		Tag:     "removable",
		Type:    config.BackendCustom,
		Address: "127.0.0.1:9999",
	})

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Verify it exists
	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if loaded.GetBackendByTag("removable") == nil {
		t.Fatal("backend should exist before removal")
	}

	// Remove it
	newBackends := make([]config.BackendConfig, 0)
	for _, b := range loaded.Backends {
		if b.Tag != "removable" {
			newBackends = append(newBackends, b)
		}
	}
	loaded.Backends = newBackends

	if err := env.WriteConfig(loaded); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Verify it's gone
	reloaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	if reloaded.GetBackendByTag("removable") != nil {
		t.Error("backend should be removed")
	}
}

func TestBackendRemove_BuiltInProtection(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Check that built-in backends are marked as such
	socks := loaded.GetBackendByTag("socks")
	if socks == nil {
		t.Fatal("socks backend not found")
	}

	if !socks.IsBuiltIn() {
		t.Error("socks should be marked as built-in")
	}
}

func TestBackendRemove_InUseProtection(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{
		Tag:       "test-tunnel",
		Transport: config.TransportSlipstream,
		Backend:   "socks", // Using the socks backend
		Domain:    "test.example.com",
		Port:      5310,
	})

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Check tunnels using the backend
	tunnelsUsing := loaded.GetTunnelsUsingBackend("socks")
	if len(tunnelsUsing) != 1 {
		t.Errorf("expected 1 tunnel using socks, got %d", len(tunnelsUsing))
	}

	if tunnelsUsing[0].Tag != "test-tunnel" {
		t.Errorf("expected test-tunnel, got %s", tunnelsUsing[0].Tag)
	}
}

func TestBackendValidation(t *testing.T) {
	tests := []struct {
		name    string
		backend config.BackendConfig
		wantErr string
	}{
		{
			name: "missing type",
			backend: config.BackendConfig{
				Tag:     "no-type",
				Address: "127.0.0.1:1080",
			},
			wantErr: "type is required",
		},
		{
			name: "invalid type",
			backend: config.BackendConfig{
				Tag:     "invalid",
				Type:    "invalid-type",
				Address: "127.0.0.1:1080",
			},
			wantErr: "unknown type",
		},
		{
			name: "socks missing address",
			backend: config.BackendConfig{
				Tag:  "socks-no-addr",
				Type: config.BackendSOCKS,
			},
			wantErr: "address is required",
		},
		{
			name: "shadowsocks missing config",
			backend: config.BackendConfig{
				Tag:  "ss-no-config",
				Type: config.BackendShadowsocks,
			},
			wantErr: "shadowsocks config is required",
		},
		{
			name: "shadowsocks missing password",
			backend: config.BackendConfig{
				Tag:         "ss-no-pw",
				Type:        config.BackendShadowsocks,
				Shadowsocks: &config.ShadowsocksConfig{},
			},
			wantErr: "password is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Backends: []config.BackendConfig{tt.backend},
			}

			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected validation error")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}
