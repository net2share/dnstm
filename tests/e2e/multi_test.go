package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/keys"
)

func TestMultiTunnel_Configuration(t *testing.T) {
	// Test that multiple tunnels can be configured
	env := NewE2EEnv(t)

	// Generate crypto material for multiple tunnels
	certsDir := filepath.Join(env.ConfigDir, "certs")
	keysDir := filepath.Join(env.ConfigDir, "keys")

	for _, dir := range []string{certsDir, keysDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	// Generate cert for slipstream tunnel
	slip1Cert := filepath.Join(certsDir, "slip1_cert.pem")
	slip1Key := filepath.Join(certsDir, "slip1_key.pem")
	_, err := certs.GenerateCertificate(slip1Cert, slip1Key, "slip1.example.com")
	if err != nil {
		t.Fatalf("failed to generate certificate: %v", err)
	}

	slip2Cert := filepath.Join(certsDir, "slip2_cert.pem")
	slip2Key := filepath.Join(certsDir, "slip2_key.pem")
	_, err = certs.GenerateCertificate(slip2Cert, slip2Key, "slip2.example.com")
	if err != nil {
		t.Fatalf("failed to generate certificate: %v", err)
	}

	// Generate keys for DNSTT tunnel
	dnsttPriv := filepath.Join(keysDir, "dnstt_server.key")
	dnsttPub := filepath.Join(keysDir, "dnstt_server.pub")
	_, err = keys.Generate(dnsttPriv, dnsttPub)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}

	// Create multi-tunnel config
	cfg := &config.Config{
		Listen: config.ListenConfig{
			Address: "0.0.0.0:53",
		},
		Route: config.RouteConfig{
			Mode:    "multi",
			Default: "slipstream-1",
		},
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
			{Tag: "ssh", Type: config.BackendSSH, Address: "127.0.0.1:22"},
		},
		Tunnels: []config.TunnelConfig{
			{
				Tag:       "slipstream-1",
				Transport: config.TransportSlipstream,
				Backend:   "socks",
				Domain:    "slip1.example.com",
				Port:      5310,
				Slipstream: &config.SlipstreamConfig{
					Cert: slip1Cert,
					Key:  slip1Key,
				},
			},
			{
				Tag:       "slipstream-2",
				Transport: config.TransportSlipstream,
				Backend:   "socks",
				Domain:    "slip2.example.com",
				Port:      5311,
				Slipstream: &config.SlipstreamConfig{
					Cert: slip2Cert,
					Key:  slip2Key,
				},
			},
			{
				Tag:       "dnstt-1",
				Transport: config.TransportDNSTT,
				Backend:   "socks",
				Domain:    "dnstt.example.com",
				Port:      5312,
				DNSTT: &config.DNSTTConfig{
					MTU:        1200,
					PrivateKey: dnsttPriv,
				},
			},
		},
	}

	// Apply defaults
	cfg.ApplyDefaults()

	// Validate
	if err := cfg.Validate(); err != nil {
		t.Fatalf("config validation failed: %v", err)
	}

	// Verify multi-mode settings
	if !cfg.IsMultiMode() {
		t.Error("expected multi mode")
	}

	if len(cfg.Tunnels) != 3 {
		t.Errorf("expected 3 tunnels, got %d", len(cfg.Tunnels))
	}

	// Verify each tunnel has a unique port
	ports := make(map[int]string)
	for _, tunnel := range cfg.Tunnels {
		if existing, ok := ports[tunnel.Port]; ok {
			t.Errorf("duplicate port %d for tunnels %s and %s", tunnel.Port, existing, tunnel.Tag)
		}
		ports[tunnel.Port] = tunnel.Tag
	}

	// Verify each tunnel has a unique domain
	domains := make(map[string]string)
	for _, tunnel := range cfg.Tunnels {
		if existing, ok := domains[tunnel.Domain]; ok {
			t.Errorf("duplicate domain %s for tunnels %s and %s", tunnel.Domain, existing, tunnel.Tag)
		}
		domains[tunnel.Domain] = tunnel.Tag
	}

	t.Log("Multi-tunnel configuration validated successfully")
}

func TestMultiTunnel_EnableDisable(t *testing.T) {
	enabled := true
	disabled := false

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{Tag: "tunnel-1", Transport: config.TransportSlipstream, Backend: "socks", Domain: "t1.example.com", Port: 5310, Enabled: &enabled},
			{Tag: "tunnel-2", Transport: config.TransportSlipstream, Backend: "socks", Domain: "t2.example.com", Port: 5311, Enabled: &disabled},
			{Tag: "tunnel-3", Transport: config.TransportSlipstream, Backend: "socks", Domain: "t3.example.com", Port: 5312}, // nil = enabled
		},
		Route: config.RouteConfig{
			Mode: "multi",
		},
	}

	enabledTunnels := cfg.GetEnabledTunnels()
	if len(enabledTunnels) != 2 {
		t.Errorf("expected 2 enabled tunnels, got %d", len(enabledTunnels))
	}

	// Verify correct tunnels are enabled
	tags := make(map[string]bool)
	for _, tunnel := range enabledTunnels {
		tags[tunnel.Tag] = true
	}

	if !tags["tunnel-1"] {
		t.Error("tunnel-1 should be enabled")
	}
	if tags["tunnel-2"] {
		t.Error("tunnel-2 should be disabled")
	}
	if !tags["tunnel-3"] {
		t.Error("tunnel-3 should be enabled (nil defaults to true)")
	}
}

func TestMultiTunnel_ModeSwitch(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{Tag: "tunnel-a", Transport: config.TransportSlipstream, Backend: "socks", Domain: "a.example.com", Port: 5310},
			{Tag: "tunnel-b", Transport: config.TransportSlipstream, Backend: "socks", Domain: "b.example.com", Port: 5311},
		},
		Route: config.RouteConfig{
			Mode:   "single",
			Active: "tunnel-a",
		},
	}

	// Start in single mode
	if !cfg.IsSingleMode() {
		t.Error("expected single mode initially")
	}

	// Switch to multi mode
	cfg.Route.Mode = "multi"
	cfg.Route.Default = "tunnel-a"

	if !cfg.IsMultiMode() {
		t.Error("expected multi mode after switch")
	}

	// Validate still works
	if err := cfg.Validate(); err != nil {
		t.Errorf("validation failed after mode switch: %v", err)
	}

	// Switch back to single
	cfg.Route.Mode = "single"

	if !cfg.IsSingleMode() {
		t.Error("expected single mode after switch back")
	}
}

func TestMultiTunnel_DefaultRoute(t *testing.T) {
	enabled := true

	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{Tag: "tunnel-a", Transport: config.TransportSlipstream, Backend: "socks", Domain: "a.example.com", Port: 5310, Enabled: &enabled},
			{Tag: "tunnel-b", Transport: config.TransportSlipstream, Backend: "socks", Domain: "b.example.com", Port: 5311, Enabled: &enabled},
		},
		Route: config.RouteConfig{
			Mode:    "multi",
			Default: "tunnel-b",
		},
	}

	// In multi mode, GetActiveTunnel returns Default
	active := cfg.GetActiveTunnel()
	if active != "tunnel-b" {
		t.Errorf("expected active tunnel 'tunnel-b', got %q", active)
	}

	// Change default
	cfg.Route.Default = "tunnel-a"
	active = cfg.GetActiveTunnel()
	if active != "tunnel-a" {
		t.Errorf("expected active tunnel 'tunnel-a', got %q", active)
	}
}
