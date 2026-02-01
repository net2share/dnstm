package integration

import (
	"testing"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/service"
)

func TestRouterMode_Single(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Route.Mode = "single"

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if !loaded.IsSingleMode() {
		t.Error("expected single mode")
	}

	if loaded.IsMultiMode() {
		t.Error("expected not multi mode")
	}
}

func TestRouterMode_Multi(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Route.Mode = "multi"

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if !loaded.IsMultiMode() {
		t.Error("expected multi mode")
	}

	if loaded.IsSingleMode() {
		t.Error("expected not single mode")
	}
}

func TestRouterMode_Default(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Route.Mode = "" // Empty = default to single

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Empty mode should default to single
	if !loaded.IsSingleMode() {
		t.Error("expected single mode (default)")
	}
}

func TestRouterSwitch_Active(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Tunnels = []config.TunnelConfig{
		{Tag: "tunnel-a", Transport: config.TransportSlipstream, Backend: "socks", Domain: "a.example.com", Port: 5310},
		{Tag: "tunnel-b", Transport: config.TransportSlipstream, Backend: "socks", Domain: "b.example.com", Port: 5311},
	}
	cfg.Route.Mode = "single"
	cfg.Route.Active = "tunnel-a"

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Verify active tunnel
	if loaded.Route.Active != "tunnel-a" {
		t.Errorf("active = %q, want 'tunnel-a'", loaded.Route.Active)
	}

	// Switch to tunnel-b
	if err := loaded.SetActiveTunnel("tunnel-b"); err != nil {
		t.Fatalf("SetActiveTunnel failed: %v", err)
	}

	if loaded.Route.Active != "tunnel-b" {
		t.Errorf("active = %q, want 'tunnel-b'", loaded.Route.Active)
	}
}

func TestRouterSwitch_InvalidTunnel(t *testing.T) {
	env := NewTestEnv(t)

	cfg := env.DefaultConfig()
	cfg.Tunnels = []config.TunnelConfig{
		{Tag: "tunnel-a", Transport: config.TransportSlipstream, Backend: "socks", Domain: "a.example.com", Port: 5310},
	}
	cfg.Route.Mode = "single"

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := env.ReadConfig()
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Try to switch to nonexistent tunnel
	err = loaded.SetActiveTunnel("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent tunnel")
	}
}

func TestRouterValidation_Mode(t *testing.T) {
	cfg := &config.Config{
		Route: config.RouteConfig{Mode: "invalid"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestRouterValidation_Active(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{Tag: "tunnel-a", Transport: config.TransportSlipstream, Backend: "socks", Domain: "a.example.com"},
		},
		Route: config.RouteConfig{
			Mode:   "single",
			Active: "nonexistent",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for nonexistent active tunnel")
	}
}

func TestRouterValidation_Default(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{Tag: "tunnel-a", Transport: config.TransportSlipstream, Backend: "socks", Domain: "a.example.com"},
		},
		Route: config.RouteConfig{
			Mode:    "multi",
			Default: "nonexistent",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for nonexistent default tunnel")
	}
}

func TestRouterGetActiveTunnel_Single(t *testing.T) {
	cfg := &config.Config{
		Route: config.RouteConfig{
			Mode:    "single",
			Active:  "my-active",
			Default: "my-default",
		},
	}

	active := cfg.GetActiveTunnel()
	if active != "my-active" {
		t.Errorf("GetActiveTunnel() = %q, want 'my-active'", active)
	}
}

func TestRouterGetActiveTunnel_Multi(t *testing.T) {
	cfg := &config.Config{
		Route: config.RouteConfig{
			Mode:    "multi",
			Active:  "my-active",
			Default: "my-default",
		},
	}

	active := cfg.GetActiveTunnel()
	// In multi mode, returns default instead of active
	if active != "my-default" {
		t.Errorf("GetActiveTunnel() = %q, want 'my-default'", active)
	}
}

func TestRouterDefaults_ActiveAndDefault(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{Tag: "first-tunnel", Transport: config.TransportSlipstream, Backend: "socks", Domain: "first.example.com"},
			{Tag: "second-tunnel", Transport: config.TransportSlipstream, Backend: "socks", Domain: "second.example.com"},
		},
	}

	cfg.ApplyDefaults()

	// Both Active and Default should be set to the first enabled tunnel
	if cfg.Route.Active != "first-tunnel" {
		t.Errorf("Route.Active = %q, want 'first-tunnel'", cfg.Route.Active)
	}

	if cfg.Route.Default != "first-tunnel" {
		t.Errorf("Route.Default = %q, want 'first-tunnel'", cfg.Route.Default)
	}
}

func TestMockSystemd_ServiceLifecycle(t *testing.T) {
	env := NewTestEnv(t)

	// Create a service
	cfg := env.DefaultConfig()
	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Test mock systemd operations
	serviceName := "dnstm-test-tunnel"

	// Create service
	err := env.MockSystemd.CreateService(serviceName, service.ServiceConfig{
		Description: "Test tunnel",
		User:        "dnstm",
		Group:       "dnstm",
		ExecStart:   "/usr/bin/test",
	})
	if err != nil {
		t.Fatalf("CreateService failed: %v", err)
	}

	// Verify service is installed but not active
	if !env.MockSystemd.IsServiceInstalled(serviceName) {
		t.Error("service should be installed")
	}

	if env.MockSystemd.IsServiceActive(serviceName) {
		t.Error("service should not be active yet")
	}

	// Start service
	if err := env.MockSystemd.StartService(serviceName); err != nil {
		t.Fatalf("StartService failed: %v", err)
	}

	if !env.MockSystemd.IsServiceActive(serviceName) {
		t.Error("service should be active after start")
	}

	// Enable service
	if err := env.MockSystemd.EnableService(serviceName); err != nil {
		t.Fatalf("EnableService failed: %v", err)
	}

	if !env.MockSystemd.IsServiceEnabled(serviceName) {
		t.Error("service should be enabled")
	}

	// Stop service
	if err := env.MockSystemd.StopService(serviceName); err != nil {
		t.Fatalf("StopService failed: %v", err)
	}

	if env.MockSystemd.IsServiceActive(serviceName) {
		t.Error("service should not be active after stop")
	}

	// Remove service
	if err := env.MockSystemd.RemoveService(serviceName); err != nil {
		t.Fatalf("RemoveService failed: %v", err)
	}

	if env.MockSystemd.IsServiceInstalled(serviceName) {
		t.Error("service should not be installed after removal")
	}
}

