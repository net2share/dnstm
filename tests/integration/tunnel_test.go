package integration

import (
	"strings"
	"testing"

	"github.com/net2share/dnstm/internal/config"
)

func TestTunnelAdd_Slipstream(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Add a slipstream tunnel
	loaded.Tunnels = append(loaded.Tunnels, config.TunnelConfig{
		Tag:       "test-slip",
		Transport: config.TransportSlipstream,
		Backend:   "socks",
		Domain:    "test.example.com",
		Port:      5310,
		Enabled:   boolPtr(true),
		Slipstream: &config.SlipstreamConfig{
			Cert: "/path/to/cert.pem",
			Key:  "/path/to/key.pem",
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

	tunnel := reloaded.GetTunnelByTag("test-slip")
	if tunnel == nil {
		t.Fatal("tunnel not found")
	}

	if tunnel.Transport != config.TransportSlipstream {
		t.Errorf("transport = %v, want %v", tunnel.Transport, config.TransportSlipstream)
	}

	if tunnel.Backend != "socks" {
		t.Errorf("backend = %q, want 'socks'", tunnel.Backend)
	}

	if tunnel.Domain != "test.example.com" {
		t.Errorf("domain = %q, want 'test.example.com'", tunnel.Domain)
	}
}

func TestTunnelAdd_DNSTT(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Add a DNSTT tunnel
	loaded.Tunnels = append(loaded.Tunnels, config.TunnelConfig{
		Tag:       "test-dnstt",
		Transport: config.TransportDNSTT,
		Backend:   "socks",
		Domain:    "dnstt.example.com",
		Port:      5311,
		Enabled:   boolPtr(true),
		DNSTT: &config.DNSTTConfig{
			MTU:        1200,
			PrivateKey: "/path/to/key",
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

	tunnel := reloaded.GetTunnelByTag("test-dnstt")
	if tunnel == nil {
		t.Fatal("tunnel not found")
	}

	if tunnel.Transport != config.TransportDNSTT {
		t.Errorf("transport = %v, want %v", tunnel.Transport, config.TransportDNSTT)
	}

	if tunnel.DNSTT == nil {
		t.Fatal("DNSTT config is nil")
	}

	if tunnel.DNSTT.MTU != 1200 {
		t.Errorf("MTU = %d, want 1200", tunnel.DNSTT.MTU)
	}
}

func TestTunnelAdd_DNSTT_ShadowsocksIncompatibility(t *testing.T) {

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{
				Tag:  "ss",
				Type: config.BackendShadowsocks,
				Shadowsocks: &config.ShadowsocksConfig{
					Password: "test",
				},
			},
		},
		Tunnels: []config.TunnelConfig{
			{
				Tag:       "dnstt-with-ss",
				Transport: config.TransportDNSTT,
				Backend:   "ss",
				Domain:    "test.example.com",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for DNSTT + Shadowsocks combination")
	}

	if !strings.Contains(err.Error(), "shadowsocks") {
		t.Errorf("error = %q, expected to mention shadowsocks", err.Error())
	}
}

func TestTunnelAdd_VayDNS(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	loaded.Tunnels = append(loaded.Tunnels, config.TunnelConfig{
		Tag:       "test-vaydns",
		Transport: config.TransportVayDNS,
		Backend:   "socks",
		Domain:    "vaydns.example.com",
		Port:      5312,
		Enabled:   boolPtr(true),
		VayDNS: &config.VayDNSConfig{
			MTU:         1200,
			PrivateKey:  "/path/to/key",
			IdleTimeout: "10s",
			KeepAlive:   "2s",
		},
	})

	if err := env.WriteConfig(loaded); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	reloaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	tunnel := reloaded.GetTunnelByTag("test-vaydns")
	if tunnel == nil {
		t.Fatal("tunnel not found")
	}

	if tunnel.Transport != config.TransportVayDNS {
		t.Errorf("transport = %v, want %v", tunnel.Transport, config.TransportVayDNS)
	}

	if tunnel.VayDNS == nil {
		t.Fatal("VayDNS config is nil")
	}

	if tunnel.VayDNS.MTU != 1200 {
		t.Errorf("MTU = %d, want 1200", tunnel.VayDNS.MTU)
	}

	if tunnel.VayDNS.IdleTimeout != "10s" {
		t.Errorf("IdleTimeout = %q, want '10s'", tunnel.VayDNS.IdleTimeout)
	}

	if tunnel.VayDNS.KeepAlive != "2s" {
		t.Errorf("KeepAlive = %q, want '2s'", tunnel.VayDNS.KeepAlive)
	}

	if !tunnel.IsVayDNS() {
		t.Error("IsVayDNS() should return true")
	}
}

func TestTunnelAdd_VayDNS_ShadowsocksIncompatibility(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{
				Tag:  "ss",
				Type: config.BackendShadowsocks,
				Shadowsocks: &config.ShadowsocksConfig{
					Password: "test",
				},
			},
		},
		Tunnels: []config.TunnelConfig{
			{
				Tag:       "vaydns-with-ss",
				Transport: config.TransportVayDNS,
				Backend:   "ss",
				Domain:    "test.example.com",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for VayDNS + Shadowsocks combination")
	}

	if !strings.Contains(err.Error(), "shadowsocks") {
		t.Errorf("error = %q, expected to mention shadowsocks", err.Error())
	}
}

func TestTunnelAdd_VayDNS_WithFallback(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{
		Tag:       "vaydns-fallback",
		Transport: config.TransportVayDNS,
		Backend:   "socks",
		Domain:    "fb.example.com",
		Port:      5313,
		VayDNS: &config.VayDNSConfig{
			MTU:      1232,
			Fallback: "127.0.0.1:8888",
		},
	})

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	tunnel := loaded.GetTunnelByTag("vaydns-fallback")
	if tunnel == nil {
		t.Fatal("tunnel not found")
	}

	if tunnel.VayDNS == nil {
		t.Fatal("VayDNS config is nil")
	}

	if tunnel.VayDNS.Fallback != "127.0.0.1:8888" {
		t.Errorf("Fallback = %q, want '127.0.0.1:8888'", tunnel.VayDNS.Fallback)
	}
}

func TestTunnelAdd_SlipstreamPlus(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	loaded.Tunnels = append(loaded.Tunnels, config.TunnelConfig{
		Tag:       "test-plus",
		Transport: config.TransportSlipstreamPlus,
		Backend:   "socks",
		Domain:    "plus.example.com",
		Port:      5314,
		Enabled:   boolPtr(true),
		SlipstreamPlus: &config.SlipstreamPlusConfig{
			Cert:               "/path/to/cert",
			Key:                "/path/to/key",
			MaxConnections:     128,
			IdleTimeoutSeconds: 45,
			Fallback:           "127.0.0.1:9999",
		},
	})

	if err := env.WriteConfig(loaded); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	reloaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	tunnel := reloaded.GetTunnelByTag("test-plus")
	if tunnel == nil {
		t.Fatal("tunnel not found")
	}
	if tunnel.Transport != config.TransportSlipstreamPlus {
		t.Errorf("transport = %v, want %v", tunnel.Transport, config.TransportSlipstreamPlus)
	}
	if tunnel.SlipstreamPlus == nil {
		t.Fatal("SlipstreamPlus config is nil")
	}
	if tunnel.SlipstreamPlus.MaxConnections != 128 {
		t.Errorf("MaxConnections = %d, want 128", tunnel.SlipstreamPlus.MaxConnections)
	}
	if tunnel.SlipstreamPlus.IdleTimeoutSeconds != 45 {
		t.Errorf("IdleTimeoutSeconds = %d, want 45", tunnel.SlipstreamPlus.IdleTimeoutSeconds)
	}
	if tunnel.SlipstreamPlus.Fallback != "127.0.0.1:9999" {
		t.Errorf("Fallback = %q, want '127.0.0.1:9999'", tunnel.SlipstreamPlus.Fallback)
	}
	if !tunnel.IsSlipstreamPlus() {
		t.Error("IsSlipstreamPlus() should return true")
	}
}

func TestTunnelList(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Tunnels = []config.TunnelConfig{
		{Tag: "tunnel-a", Transport: config.TransportSlipstream, Backend: "socks", Domain: "a.example.com", Port: 5310},
		{Tag: "tunnel-b", Transport: config.TransportSlipstream, Backend: "socks", Domain: "b.example.com", Port: 5311},
		{Tag: "tunnel-c", Transport: config.TransportDNSTT, Backend: "socks", Domain: "c.example.com", Port: 5312},
	}

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if len(loaded.Tunnels) != 3 {
		t.Errorf("len(Tunnels) = %d, want 3", len(loaded.Tunnels))
	}
}

func TestTunnelEnabledState(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Tunnels = []config.TunnelConfig{
		{Tag: "test-tunnel", Transport: config.TransportSlipstream, Backend: "socks", Domain: "test.example.com", Port: 5310},
	}

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	tunnel := loaded.GetTunnelByTag("test-tunnel")
	if tunnel == nil {
		t.Fatal("tunnel not found")
	}

	// Default should be enabled
	if !tunnel.IsEnabled() {
		t.Error("tunnel should be enabled by default")
	}

	// Disable it
	enabled := false
	tunnel.Enabled = &enabled

	if tunnel.IsEnabled() {
		t.Error("tunnel should be disabled")
	}

	// Enable it again
	enabled = true
	tunnel.Enabled = &enabled

	if !tunnel.IsEnabled() {
		t.Error("tunnel should be enabled")
	}
}

func TestTunnelRemove(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Tunnels = []config.TunnelConfig{
		{Tag: "keep-this", Transport: config.TransportSlipstream, Backend: "socks", Domain: "keep.example.com", Port: 5310},
		{Tag: "remove-this", Transport: config.TransportSlipstream, Backend: "socks", Domain: "remove.example.com", Port: 5311},
	}

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Remove "remove-this"
	newTunnels := make([]config.TunnelConfig, 0)
	for _, tunnel := range loaded.Tunnels {
		if tunnel.Tag != "remove-this" {
			newTunnels = append(newTunnels, tunnel)
		}
	}
	loaded.Tunnels = newTunnels

	if err := env.WriteConfig(loaded); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Reload and verify
	reloaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	if len(reloaded.Tunnels) != 1 {
		t.Errorf("len(Tunnels) = %d, want 1", len(reloaded.Tunnels))
	}

	if reloaded.GetTunnelByTag("remove-this") != nil {
		t.Error("tunnel should be removed")
	}

	if reloaded.GetTunnelByTag("keep-this") == nil {
		t.Error("other tunnel should still exist")
	}
}

func TestTunnelValidation(t *testing.T) {
	tests := []struct {
		name    string
		tunnel  config.TunnelConfig
		wantErr string
	}{
		{
			name: "missing transport",
			tunnel: config.TunnelConfig{
				Tag:     "no-transport",
				Backend: "socks",
				Domain:  "test.example.com",
			},
			wantErr: "transport is required",
		},
		{
			name: "invalid transport",
			tunnel: config.TunnelConfig{
				Tag:       "invalid-transport",
				Transport: "invalid",
				Backend:   "socks",
				Domain:    "test.example.com",
			},
			wantErr: "unknown transport",
		},
		{
			name: "missing backend",
			tunnel: config.TunnelConfig{
				Tag:       "no-backend",
				Transport: config.TransportSlipstream,
				Domain:    "test.example.com",
			},
			wantErr: "backend is required",
		},
		{
			name: "missing domain",
			tunnel: config.TunnelConfig{
				Tag:       "no-domain",
				Transport: config.TransportSlipstream,
				Backend:   "socks",
			},
			wantErr: "domain is required",
		},
		{
			name: "backend not found",
			tunnel: config.TunnelConfig{
				Tag:       "bad-backend",
				Transport: config.TransportSlipstream,
				Backend:   "nonexistent",
				Domain:    "test.example.com",
			},
			wantErr: "not found",
		},
		{
			name: "port too low",
			tunnel: config.TunnelConfig{
				Tag:       "low-port",
				Transport: config.TransportSlipstream,
				Backend:   "socks",
				Domain:    "test.example.com",
				Port:      80,
			},
			wantErr: "port must be between",
		},
		{
			name: "port too high",
			tunnel: config.TunnelConfig{
				Tag:       "high-port",
				Transport: config.TransportSlipstream,
				Backend:   "socks",
				Domain:    "test.example.com",
				Port:      70000,
			},
			wantErr: "port must be between",
		},
		{
			name: "dnstt mtu too low",
			tunnel: config.TunnelConfig{
				Tag:       "low-mtu",
				Transport: config.TransportDNSTT,
				Backend:   "socks",
				Domain:    "test.example.com",
				DNSTT:     &config.DNSTTConfig{MTU: 100},
			},
			wantErr: "dnstt.mtu must be between",
		},
		{
			name: "dnstt mtu too high",
			tunnel: config.TunnelConfig{
				Tag:       "high-mtu",
				Transport: config.TransportDNSTT,
				Backend:   "socks",
				Domain:    "test.example.com",
				DNSTT:     &config.DNSTTConfig{MTU: 2000},
			},
			wantErr: "dnstt.mtu must be between",
		},
		{
			name: "vaydns mtu too low",
			tunnel: config.TunnelConfig{
				Tag:       "vaydns-low-mtu",
				Transport: config.TransportVayDNS,
				Backend:   "socks",
				Domain:    "test.example.com",
				VayDNS:    &config.VayDNSConfig{MTU: 100},
			},
			wantErr: "vaydns.mtu must be between",
		},
		{
			name: "vaydns mtu too high",
			tunnel: config.TunnelConfig{
				Tag:       "vaydns-high-mtu",
				Transport: config.TransportVayDNS,
				Backend:   "socks",
				Domain:    "test.example.com",
				VayDNS:    &config.VayDNSConfig{MTU: 2000},
			},
			wantErr: "vaydns.mtu must be between",
		},
		{
			name: "slipstream-plus bad fallback",
			tunnel: config.TunnelConfig{
				Tag:            "plus-bad-fb",
				Transport:      config.TransportSlipstreamPlus,
				Backend:        "socks",
				Domain:         "test.example.com",
				SlipstreamPlus: &config.SlipstreamPlusConfig{Fallback: "not-host-port"},
			},
			wantErr: "slipstream_plus.fallback must be host:port",
		},
		{
			name: "slipstream-plus negative max_connections",
			tunnel: config.TunnelConfig{
				Tag:            "plus-neg-mc",
				Transport:      config.TransportSlipstreamPlus,
				Backend:        "socks",
				Domain:         "test.example.com",
				SlipstreamPlus: &config.SlipstreamPlusConfig{MaxConnections: -1},
			},
			wantErr: "max_connections must not be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Backends: []config.BackendConfig{
					{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
				},
				Tunnels: []config.TunnelConfig{tt.tunnel},
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

func TestTunnelDuplicatePorts(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{Tag: "tunnel-a", Transport: config.TransportSlipstream, Backend: "socks", Domain: "a.example.com", Port: 5310},
			{Tag: "tunnel-b", Transport: config.TransportSlipstream, Backend: "socks", Domain: "b.example.com", Port: 5310}, // Duplicate port
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate ports")
	}

	if !strings.Contains(err.Error(), "already used") {
		t.Errorf("error = %q, expected 'already used'", err.Error())
	}
}

func TestTunnelDuplicateDomains(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{Tag: "tunnel-a", Transport: config.TransportSlipstream, Backend: "socks", Domain: "same.example.com", Port: 5310},
			{Tag: "tunnel-b", Transport: config.TransportSlipstream, Backend: "socks", Domain: "same.example.com", Port: 5311}, // Duplicate domain
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate domains")
	}

	if !strings.Contains(err.Error(), "already used") {
		t.Errorf("error = %q, expected 'already used'", err.Error())
	}
}

func TestGetEnabledTunnels(t *testing.T) {
	enabled := true
	disabled := false

	cfg := &config.Config{
		Tunnels: []config.TunnelConfig{
			{Tag: "enabled-1", Enabled: &enabled},
			{Tag: "disabled", Enabled: &disabled},
			{Tag: "enabled-2", Enabled: nil}, // nil = enabled by default
		},
	}

	tunnels := cfg.GetEnabledTunnels()
	if len(tunnels) != 2 {
		t.Errorf("len(EnabledTunnels) = %d, want 2", len(tunnels))
	}

	tags := make(map[string]bool)
	for _, tunnel := range tunnels {
		tags[tunnel.Tag] = true
	}

	if !tags["enabled-1"] || !tags["enabled-2"] {
		t.Errorf("expected enabled-1 and enabled-2, got %v", tags)
	}

	if tags["disabled"] {
		t.Error("disabled tunnel should not be in enabled list")
	}
}
