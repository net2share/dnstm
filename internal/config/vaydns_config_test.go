package config

import (
	"strings"
	"testing"
)

func TestVayDNSConfig_ResolvedVayDNSIdleTimeout(t *testing.T) {
	tests := []struct {
		name string
		v    *VayDNSConfig
		want string
	}{
		{"nil", nil, "10s"},
		{"default native", &VayDNSConfig{}, "10s"},
		{"default compat", &VayDNSConfig{DnsttCompat: true}, "2m"},
		{"explicit overrides compat default", &VayDNSConfig{DnsttCompat: true, IdleTimeout: "90s"}, "90s"},
		{"explicit native", &VayDNSConfig{IdleTimeout: "30s"}, "30s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if g := tt.v.ResolvedVayDNSIdleTimeout(); g != tt.want {
				t.Errorf("ResolvedVayDNSIdleTimeout() = %q, want %q", g, tt.want)
			}
		})
	}
}

func TestVayDNSConfig_ResolvedVayDNSKeepAlive(t *testing.T) {
	tests := []struct {
		name string
		v    *VayDNSConfig
		want string
	}{
		{"nil", nil, "2s"},
		{"default native", &VayDNSConfig{}, "2s"},
		{"default compat", &VayDNSConfig{DnsttCompat: true}, "10s"},
		{"explicit", &VayDNSConfig{KeepAlive: "3s"}, "3s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if g := tt.v.ResolvedVayDNSKeepAlive(); g != tt.want {
				t.Errorf("ResolvedVayDNSKeepAlive() = %q, want %q", g, tt.want)
			}
		})
	}
}

func TestVayDNSConfig_VayDNSClientIDSizeForFlag(t *testing.T) {
	tests := []struct {
		name string
		v    *VayDNSConfig
		want int
	}{
		{"nil", nil, 2},
		{"native default", &VayDNSConfig{}, 2},
		{"native explicit", &VayDNSConfig{ClientIDSize: 4}, 4},
		{"compat omits flag", &VayDNSConfig{DnsttCompat: true}, 0},
		{"compat ignores clientid", &VayDNSConfig{DnsttCompat: true, ClientIDSize: 4}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if g := tt.v.VayDNSClientIDSizeForFlag(); g != tt.want {
				t.Errorf("VayDNSClientIDSizeForFlag() = %d, want %d", g, tt.want)
			}
		})
	}
}

func TestValidate_VayDNSSessionTiming(t *testing.T) {
	base := Config{
		Backends: []BackendConfig{
			{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
		},
	}

	t.Run("keepalive must be less than idle", func(t *testing.T) {
		cfg := base
		cfg.Tunnels = []TunnelConfig{{
			Tag:       "t",
			Transport: TransportVayDNS,
			Backend:   "socks",
			Domain:    "d.example.com",
			VayDNS:    &VayDNSConfig{IdleTimeout: "5s", KeepAlive: "5s"},
		}}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "keep_alive must be less") {
			t.Fatalf("Validate() = %v, want keep_alive error", err)
		}
	})

	t.Run("invalid idle_timeout", func(t *testing.T) {
		cfg := base
		cfg.Tunnels = []TunnelConfig{{
			Tag:       "t",
			Transport: TransportVayDNS,
			Backend:   "socks",
			Domain:    "d.example.com",
			VayDNS:    &VayDNSConfig{IdleTimeout: "bogus", KeepAlive: "1s"},
		}}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "invalid vaydns.idle_timeout") {
			t.Fatalf("Validate() = %v, want parse error", err)
		}
	})

	t.Run("clientid_size with dnstt_compat", func(t *testing.T) {
		cfg := base
		cfg.Tunnels = []TunnelConfig{{
			Tag:       "t",
			Transport: TransportVayDNS,
			Backend:   "socks",
			Domain:    "d.example.com",
			VayDNS:    &VayDNSConfig{DnsttCompat: true, ClientIDSize: 4},
		}}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "clientid_size cannot be set with dnstt_compat") {
			t.Fatalf("Validate() = %v, want compat conflict error", err)
		}
	})

	t.Run("negative clientid_size", func(t *testing.T) {
		cfg := base
		cfg.Tunnels = []TunnelConfig{{
			Tag:       "t",
			Transport: TransportVayDNS,
			Backend:   "socks",
			Domain:    "d.example.com",
			VayDNS:    &VayDNSConfig{ClientIDSize: -1},
		}}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "clientid_size must not be negative") {
			t.Fatalf("Validate() = %v, want clientid error", err)
		}
	})
}
