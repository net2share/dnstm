package integration

import (
	"testing"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/testutil"
)

// TestSingleMode_TunnelSwitch tests that switching tunnels in single mode
// properly updates the active tunnel.
func TestSingleMode_TunnelSwitch(t *testing.T) {
	// Create test environment
	env := testutil.NewTestEnv(t)

	// Create base config with two tunnels
	cfg := &config.Config{
		Log:    config.LogConfig{Level: "info"},
		Listen: config.ListenConfig{Address: "0.0.0.0:53"},
		Proxy:  config.ProxyConfig{Port: 1080},
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{
				Tag:       "tunnel-a",
				Transport: config.TransportSlipstream,
				Backend:   "socks",
				Domain:    "a.example.com",
				Port:      5310,
				Slipstream: &config.SlipstreamConfig{
					Cert: "/etc/dnstm/certs/a_example_com_cert.pem",
					Key:  "/etc/dnstm/certs/a_example_com_key.pem",
				},
			},
			{
				Tag:       "tunnel-b",
				Transport: config.TransportDNSTT,
				Backend:   "socks",
				Domain:    "b.example.com",
				Port:      5311,
				DNSTT: &config.DNSTTConfig{
					MTU:        1232,
					PrivateKey: "/etc/dnstm/keys/b_example_com_server.key",
				},
			},
		},
		Route: config.RouteConfig{
			Mode:   "single",
			Active: "tunnel-a",
		},
	}

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Test switching active tunnel
	t.Run("switch_to_tunnel_b", func(t *testing.T) {
		// Reload config
		cfg, err := env.ReadConfig()
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		// Verify initial state
		if cfg.Route.Active != "tunnel-a" {
			t.Errorf("expected active tunnel 'tunnel-a', got '%s'", cfg.Route.Active)
		}

		// Simulate switch by updating config
		cfg.Route.Active = "tunnel-b"
		if err := env.WriteConfig(cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		// Reload and verify
		cfg, err = env.ReadConfig()
		if err != nil {
			t.Fatalf("failed to reload config: %v", err)
		}

		if cfg.Route.Active != "tunnel-b" {
			t.Errorf("expected active tunnel 'tunnel-b', got '%s'", cfg.Route.Active)
		}
	})

	t.Run("switch_back_to_tunnel_a", func(t *testing.T) {
		cfg, err := env.ReadConfig()
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		cfg.Route.Active = "tunnel-a"
		if err := env.WriteConfig(cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		cfg, err = env.ReadConfig()
		if err != nil {
			t.Fatalf("failed to reload config: %v", err)
		}

		if cfg.Route.Active != "tunnel-a" {
			t.Errorf("expected active tunnel 'tunnel-a', got '%s'", cfg.Route.Active)
		}
	})
}

// TestSingleMode_SwitchWithMockSystemd tests the full switch flow with mock systemd.
func TestSingleMode_SwitchWithMockSystemd(t *testing.T) {
	env := testutil.NewTestEnv(t)
	mock := env.MockSystemd

	// Create mock services for two tunnels
	mock.CreateService("dnstm-tunnel-a", service.ServiceConfig{
		Name:      "dnstm-tunnel-a",
		ExecStart: "/usr/local/bin/slipstream-server",
	})
	mock.CreateService("dnstm-tunnel-b", service.ServiceConfig{
		Name:      "dnstm-tunnel-b",
		ExecStart: "/usr/local/bin/dnstt-server",
	})

	// Initially tunnel-a is active and running
	if err := mock.StartService("dnstm-tunnel-a"); err != nil {
		t.Fatalf("failed to start tunnel-a: %v", err)
	}

	// Verify tunnel-a is running
	if !mock.IsServiceActive("dnstm-tunnel-a") {
		t.Error("expected tunnel-a to be active")
	}

	// Simulate switch: stop tunnel-a, start tunnel-b
	if err := mock.StopService("dnstm-tunnel-a"); err != nil {
		t.Fatalf("failed to stop tunnel-a: %v", err)
	}

	if err := mock.StartService("dnstm-tunnel-b"); err != nil {
		t.Fatalf("failed to start tunnel-b: %v", err)
	}

	// Verify tunnel-a is stopped and tunnel-b is running
	if mock.IsServiceActive("dnstm-tunnel-a") {
		t.Error("expected tunnel-a to be inactive")
	}
	if !mock.IsServiceActive("dnstm-tunnel-b") {
		t.Error("expected tunnel-b to be active")
	}
}

// TestModeSwitch_ServiceRegeneration tests that switching modes updates configuration.
func TestModeSwitch_ServiceRegeneration(t *testing.T) {
	env := testutil.NewTestEnv(t)

	// Create config with two tunnels
	cfg := &config.Config{
		Log:    config.LogConfig{Level: "info"},
		Listen: config.ListenConfig{Address: "0.0.0.0:53"},
		Proxy:  config.ProxyConfig{Port: 1080},
		Backends: []config.BackendConfig{
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []config.TunnelConfig{
			{
				Tag:       "tunnel-a",
				Transport: config.TransportSlipstream,
				Backend:   "socks",
				Domain:    "a.example.com",
				Port:      5310,
			},
			{
				Tag:       "tunnel-b",
				Transport: config.TransportDNSTT,
				Backend:   "socks",
				Domain:    "b.example.com",
				Port:      5311,
			},
		},
		Route: config.RouteConfig{
			Mode:   "single",
			Active: "tunnel-a",
		},
	}

	if err := env.WriteConfig(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	t.Run("single_to_multi", func(t *testing.T) {
		cfg, err := env.ReadConfig()
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		// Switch to multi mode
		cfg.Route.Mode = "multi"
		cfg.Route.Default = cfg.Route.Active
		cfg.Route.Active = ""

		if err := env.WriteConfig(cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		cfg, err = env.ReadConfig()
		if err != nil {
			t.Fatalf("failed to reload config: %v", err)
		}

		if cfg.Route.Mode != "multi" {
			t.Errorf("expected mode 'multi', got '%s'", cfg.Route.Mode)
		}
		if cfg.Route.Default != "tunnel-a" {
			t.Errorf("expected default 'tunnel-a', got '%s'", cfg.Route.Default)
		}
	})

	t.Run("multi_to_single", func(t *testing.T) {
		cfg, err := env.ReadConfig()
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		// Switch to single mode
		cfg.Route.Mode = "single"
		cfg.Route.Active = cfg.Route.Default
		cfg.Route.Default = ""

		if err := env.WriteConfig(cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		cfg, err = env.ReadConfig()
		if err != nil {
			t.Fatalf("failed to reload config: %v", err)
		}

		if cfg.Route.Mode != "single" {
			t.Errorf("expected mode 'single', got '%s'", cfg.Route.Mode)
		}
		if cfg.Route.Active != "tunnel-a" {
			t.Errorf("expected active 'tunnel-a', got '%s'", cfg.Route.Active)
		}
	})
}
