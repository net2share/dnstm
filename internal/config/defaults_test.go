package config

import (
	"testing"
)

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{
		Backends: []BackendConfig{
			{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []TunnelConfig{
			{Tag: "tunnel-a", Transport: TransportSlipstream, Backend: "socks", Domain: "a.example.com"},
			{Tag: "tunnel-b", Transport: TransportDNSTT, Backend: "socks", Domain: "b.example.com"},
		},
	}

	cfg.ApplyDefaults()

	// Check log defaults
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want 'info'", cfg.Log.Level)
	}
	if cfg.Log.Timestamp == nil || !*cfg.Log.Timestamp {
		t.Error("Log.Timestamp should default to true")
	}

	// Check listen defaults
	if cfg.Listen.Address != "0.0.0.0:53" {
		t.Errorf("Listen.Address = %q, want '0.0.0.0:53'", cfg.Listen.Address)
	}

	// Check route defaults
	if cfg.Route.Mode != "single" {
		t.Errorf("Route.Mode = %q, want 'single'", cfg.Route.Mode)
	}
	if cfg.Route.Active != "tunnel-a" {
		t.Errorf("Route.Active = %q, want 'tunnel-a'", cfg.Route.Active)
	}
	if cfg.Route.Default != "tunnel-a" {
		t.Errorf("Route.Default = %q, want 'tunnel-a'", cfg.Route.Default)
	}

	// Check tunnel defaults
	for _, tunnel := range cfg.Tunnels {
		if tunnel.Enabled == nil || !*tunnel.Enabled {
			t.Errorf("Tunnel %q: Enabled should default to true", tunnel.Tag)
		}
		if tunnel.Port == 0 {
			t.Errorf("Tunnel %q: Port should be auto-allocated", tunnel.Tag)
		}
	}

	// Check DNSTT-specific defaults
	dnsttTunnel := cfg.GetTunnelByTag("tunnel-b")
	if dnsttTunnel.DNSTT == nil {
		t.Error("DNSTT tunnel should have DNSTT config")
	} else if dnsttTunnel.DNSTT.MTU != 1232 {
		t.Errorf("DNSTT MTU = %d, want 1232", dnsttTunnel.DNSTT.MTU)
	}
}

func TestApplyDefaults_PreserveExisting(t *testing.T) {
	enabled := false
	cfg := &Config{
		Log: LogConfig{
			Level: "debug",
		},
		Listen: ListenConfig{
			Address: "127.0.0.1:5353",
		},
		Route: RouteConfig{
			Mode:   "multi",
			Active: "custom-active",
		},
		Backends: []BackendConfig{
			{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []TunnelConfig{
			{Tag: "tunnel", Transport: TransportSlipstream, Backend: "socks", Domain: "test.example.com", Port: 9999, Enabled: &enabled},
		},
	}

	cfg.ApplyDefaults()

	// Should preserve existing values
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want 'debug'", cfg.Log.Level)
	}
	if cfg.Listen.Address != "127.0.0.1:5353" {
		t.Errorf("Listen.Address = %q, want '127.0.0.1:5353'", cfg.Listen.Address)
	}
	if cfg.Route.Mode != "multi" {
		t.Errorf("Route.Mode = %q, want 'multi'", cfg.Route.Mode)
	}
	if cfg.Route.Active != "custom-active" {
		t.Errorf("Route.Active = %q, want 'custom-active'", cfg.Route.Active)
	}

	tunnel := cfg.Tunnels[0]
	if tunnel.Port != 9999 {
		t.Errorf("Tunnel.Port = %d, want 9999", tunnel.Port)
	}
	if tunnel.Enabled == nil || *tunnel.Enabled {
		t.Error("Tunnel.Enabled should stay false")
	}
}

func TestApplyDefaults_ShadowsocksMethod(t *testing.T) {
	cfg := &Config{
		Backends: []BackendConfig{
			{Tag: "ss", Type: BackendShadowsocks, Shadowsocks: &ShadowsocksConfig{Password: "secret"}},
		},
	}

	cfg.ApplyDefaults()

	if cfg.Backends[0].Shadowsocks.Method != "aes-256-gcm" {
		t.Errorf("Shadowsocks.Method = %q, want 'aes-256-gcm'", cfg.Backends[0].Shadowsocks.Method)
	}
}

func TestApplyDefaults_PortAllocation(t *testing.T) {
	cfg := &Config{
		Backends: []BackendConfig{
			{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
		},
		Tunnels: []TunnelConfig{
			{Tag: "tunnel-a", Transport: TransportSlipstream, Backend: "socks", Domain: "a.example.com"},
			{Tag: "tunnel-b", Transport: TransportSlipstream, Backend: "socks", Domain: "b.example.com"},
			{Tag: "tunnel-c", Transport: TransportSlipstream, Backend: "socks", Domain: "c.example.com", Port: 5315}, // Pre-assigned
			{Tag: "tunnel-d", Transport: TransportSlipstream, Backend: "socks", Domain: "d.example.com"},
		},
	}

	cfg.ApplyDefaults()

	// Check that ports are unique
	ports := make(map[int]string)
	for _, tunnel := range cfg.Tunnels {
		if existing, ok := ports[tunnel.Port]; ok {
			t.Errorf("Port %d used by both %q and %q", tunnel.Port, existing, tunnel.Tag)
		}
		ports[tunnel.Port] = tunnel.Tag

		// All ports should be in valid range
		if tunnel.Port < DefaultPortStart || tunnel.Port > 65535 {
			t.Errorf("Tunnel %q: Port %d outside valid range", tunnel.Tag, tunnel.Port)
		}
	}

	// Tunnel-c should keep its assigned port
	if cfg.Tunnels[2].Port != 5315 {
		t.Errorf("Tunnel-c port = %d, want 5315", cfg.Tunnels[2].Port)
	}
}

func TestAllocateNextPort(t *testing.T) {
	cfg := &Config{
		Tunnels: []TunnelConfig{
			{Tag: "tunnel-a", Port: 5310},
			{Tag: "tunnel-b", Port: 5311},
		},
	}

	port := cfg.AllocateNextPort()
	if port != 5312 {
		t.Errorf("AllocateNextPort() = %d, want 5312", port)
	}
}

func TestAllocateNextPort_Empty(t *testing.T) {
	cfg := &Config{}

	port := cfg.AllocateNextPort()
	if port != DefaultPortStart {
		t.Errorf("AllocateNextPort() = %d, want %d", port, DefaultPortStart)
	}
}

func TestAllocateNextPort_Gap(t *testing.T) {
	cfg := &Config{
		Tunnels: []TunnelConfig{
			{Tag: "tunnel-a", Port: 5310},
			{Tag: "tunnel-b", Port: 5312}, // Gap at 5311
		},
	}

	port := cfg.AllocateNextPort()
	if port != 5311 {
		t.Errorf("AllocateNextPort() = %d, want 5311 (fill gap)", port)
	}
}

func TestEnsureBuiltinBackends(t *testing.T) {
	cfg := &Config{
		Proxy: ProxyConfig{Port: 1080},
		Backends: []BackendConfig{
			{Tag: "custom", Type: BackendCustom, Address: "192.168.1.1:8080"},
		},
	}

	cfg.EnsureBuiltinBackends()

	// Should add socks and ssh
	if len(cfg.Backends) != 3 {
		t.Errorf("len(Backends) = %d, want 3", len(cfg.Backends))
	}

	socks := cfg.GetBackendByTag("socks")
	if socks == nil {
		t.Fatal("expected 'socks' backend to be added")
	}
	if socks.Type != BackendSOCKS {
		t.Errorf("socks.Type = %v, want %v", socks.Type, BackendSOCKS)
	}
	if socks.Address != "127.0.0.1:1080" {
		t.Errorf("socks.Address = %q, want '127.0.0.1:1080'", socks.Address)
	}

	ssh := cfg.GetBackendByTag("ssh")
	if ssh == nil {
		t.Fatal("expected 'ssh' backend to be added")
	}
	if ssh.Type != BackendSSH {
		t.Errorf("ssh.Type = %v, want %v", ssh.Type, BackendSSH)
	}
}

func TestEnsureBuiltinBackends_AlreadyExists(t *testing.T) {
	cfg := &Config{
		Backends: []BackendConfig{
			{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:8888"},
			{Tag: "ssh", Type: BackendSSH, Address: "127.0.0.1:2222"},
		},
	}

	cfg.EnsureBuiltinBackends()

	// Should not duplicate
	if len(cfg.Backends) != 2 {
		t.Errorf("len(Backends) = %d, want 2", len(cfg.Backends))
	}

	// Should preserve original values
	socks := cfg.GetBackendByTag("socks")
	if socks.Address != "127.0.0.1:8888" {
		t.Errorf("socks.Address = %q, want '127.0.0.1:8888'", socks.Address)
	}
}

func TestUpdateSocksBackendPort(t *testing.T) {
	cfg := &Config{
		Backends: []BackendConfig{
			{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
		},
	}

	cfg.UpdateSocksBackendPort(9999)

	socks := cfg.GetBackendByTag("socks")
	if socks.Address != "127.0.0.1:9999" {
		t.Errorf("socks.Address = %q, want '127.0.0.1:9999'", socks.Address)
	}
}

func TestUpdateSocksBackendPort_NotFound(t *testing.T) {
	cfg := &Config{
		Backends: []BackendConfig{
			{Tag: "ssh", Type: BackendSSH, Address: "127.0.0.1:22"},
		},
	}

	// Should not panic
	cfg.UpdateSocksBackendPort(9999)
}

func TestDefaultPortConstants(t *testing.T) {
	if DefaultPortStart != 5310 {
		t.Errorf("DefaultPortStart = %d, want 5310", DefaultPortStart)
	}
	if DefaultPortEnd != 5399 {
		t.Errorf("DefaultPortEnd = %d, want 5399", DefaultPortEnd)
	}
	if DefaultPortEnd < DefaultPortStart {
		t.Error("DefaultPortEnd should be >= DefaultPortStart")
	}
}
