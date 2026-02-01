package config

import (
	"strings"
	"testing"
)

func TestValidate_TagUniqueness(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "valid unique backend tags",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "backend-a", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
					{Tag: "backend-b", Type: BackendSSH, Address: "127.0.0.1:22"},
				},
			},
			wantErr: "",
		},
		{
			name: "duplicate backend tags",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "duplicate", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
					{Tag: "duplicate", Type: BackendSSH, Address: "127.0.0.1:22"},
				},
			},
			wantErr: "duplicate backend tag",
		},
		{
			name: "valid unique tunnel tags",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
				},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel-a", Transport: TransportSlipstream, Backend: "socks", Domain: "a.example.com"},
					{Tag: "tunnel-b", Transport: TransportSlipstream, Backend: "socks", Domain: "b.example.com"},
				},
			},
			wantErr: "",
		},
		{
			name: "duplicate tunnel tags",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
				},
				Tunnels: []TunnelConfig{
					{Tag: "duplicate", Transport: TransportSlipstream, Backend: "socks", Domain: "a.example.com"},
					{Tag: "duplicate", Transport: TransportSlipstream, Backend: "socks", Domain: "b.example.com"},
				},
			},
			wantErr: "duplicate tunnel tag",
		},
		{
			name: "empty backend tag",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
				},
			},
			wantErr: "tag is required",
		},
		{
			name: "empty tunnel tag",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
				},
				Tunnels: []TunnelConfig{
					{Tag: "", Transport: TransportSlipstream, Backend: "socks", Domain: "a.example.com"},
				},
			},
			wantErr: "tag is required",
		},
		{
			name: "invalid tag format - starts with number",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "123-backend", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
				},
			},
			wantErr: "tag must start with a letter",
		},
		{
			name: "invalid tag format - special characters",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "backend.with.dots", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
				},
			},
			wantErr: "tag must start with a letter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Validate() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestValidate_Backends(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "valid socks backend",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"},
				},
			},
			wantErr: "",
		},
		{
			name: "valid ssh backend",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "ssh", Type: BackendSSH, Address: "127.0.0.1:22"},
				},
			},
			wantErr: "",
		},
		{
			name: "valid shadowsocks backend",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "ss", Type: BackendShadowsocks, Shadowsocks: &ShadowsocksConfig{Password: "secret", Method: "aes-256-gcm"}},
				},
			},
			wantErr: "",
		},
		{
			name: "valid custom backend",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "custom", Type: BackendCustom, Address: "192.168.1.1:8080"},
				},
			},
			wantErr: "",
		},
		{
			name: "missing type",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "noType", Address: "127.0.0.1:1080"},
				},
			},
			wantErr: "type is required",
		},
		{
			name: "unknown type",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "unknown", Type: "invalid-type", Address: "127.0.0.1:1080"},
				},
			},
			wantErr: "unknown type",
		},
		{
			name: "socks missing address",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "socks", Type: BackendSOCKS},
				},
			},
			wantErr: "address is required",
		},
		{
			name: "shadowsocks missing config",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "ss", Type: BackendShadowsocks},
				},
			},
			wantErr: "shadowsocks config is required",
		},
		{
			name: "shadowsocks missing password",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "ss", Type: BackendShadowsocks, Shadowsocks: &ShadowsocksConfig{}},
				},
			},
			wantErr: "shadowsocks.password is required",
		},
		{
			name: "shadowsocks invalid method",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "ss", Type: BackendShadowsocks, Shadowsocks: &ShadowsocksConfig{Password: "secret", Method: "invalid-method"}},
				},
			},
			wantErr: "invalid shadowsocks method",
		},
		{
			name: "shadowsocks empty method (valid, will use default)",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "ss", Type: BackendShadowsocks, Shadowsocks: &ShadowsocksConfig{Password: "secret"}},
				},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Validate() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestValidate_Tunnels(t *testing.T) {
	validBackend := BackendConfig{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"}

	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "valid slipstream tunnel",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportSlipstream, Backend: "socks", Domain: "test.example.com", Port: 5310},
				},
			},
			wantErr: "",
		},
		{
			name: "valid dnstt tunnel",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportDNSTT, Backend: "socks", Domain: "test.example.com", Port: 5310},
				},
			},
			wantErr: "",
		},
		{
			name: "missing transport",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Backend: "socks", Domain: "test.example.com"},
				},
			},
			wantErr: "transport is required",
		},
		{
			name: "unknown transport",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: "invalid", Backend: "socks", Domain: "test.example.com"},
				},
			},
			wantErr: "unknown transport",
		},
		{
			name: "missing backend",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportSlipstream, Domain: "test.example.com"},
				},
			},
			wantErr: "backend is required",
		},
		{
			name: "missing domain",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportSlipstream, Backend: "socks"},
				},
			},
			wantErr: "domain is required",
		},
		{
			name: "backend not found",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportSlipstream, Backend: "nonexistent", Domain: "test.example.com"},
				},
			},
			wantErr: "backend 'nonexistent' not found",
		},
		{
			name: "dnstt with shadowsocks backend",
			cfg: &Config{
				Backends: []BackendConfig{
					{Tag: "ss", Type: BackendShadowsocks, Shadowsocks: &ShadowsocksConfig{Password: "secret"}},
				},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportDNSTT, Backend: "ss", Domain: "test.example.com"},
				},
			},
			wantErr: "dnstt transport does not support shadowsocks",
		},
		{
			name: "port too low",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportSlipstream, Backend: "socks", Domain: "test.example.com", Port: 80},
				},
			},
			wantErr: "port must be between 1024 and 65535",
		},
		{
			name: "port too high",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportSlipstream, Backend: "socks", Domain: "test.example.com", Port: 70000},
				},
			},
			wantErr: "port must be between 1024 and 65535",
		},
		{
			name: "duplicate ports",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel-a", Transport: TransportSlipstream, Backend: "socks", Domain: "a.example.com", Port: 5310},
					{Tag: "tunnel-b", Transport: TransportSlipstream, Backend: "socks", Domain: "b.example.com", Port: 5310},
				},
			},
			wantErr: "port 5310 already used by",
		},
		{
			name: "duplicate domains",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel-a", Transport: TransportSlipstream, Backend: "socks", Domain: "test.example.com", Port: 5310},
					{Tag: "tunnel-b", Transport: TransportSlipstream, Backend: "socks", Domain: "test.example.com", Port: 5311},
				},
			},
			wantErr: "domain 'test.example.com' already used by",
		},
		{
			name: "dnstt mtu too low",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportDNSTT, Backend: "socks", Domain: "test.example.com", DNSTT: &DNSTTConfig{MTU: 100}},
				},
			},
			wantErr: "dnstt.mtu must be between 512 and 1400",
		},
		{
			name: "dnstt mtu too high",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportDNSTT, Backend: "socks", Domain: "test.example.com", DNSTT: &DNSTTConfig{MTU: 2000}},
				},
			},
			wantErr: "dnstt.mtu must be between 512 and 1400",
		},
		{
			name: "dnstt valid mtu",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels: []TunnelConfig{
					{Tag: "tunnel", Transport: TransportDNSTT, Backend: "socks", Domain: "test.example.com", DNSTT: &DNSTTConfig{MTU: 1200}},
				},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Validate() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestValidate_Route(t *testing.T) {
	validBackend := BackendConfig{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080"}
	validTunnel := TunnelConfig{Tag: "tunnel-a", Transport: TransportSlipstream, Backend: "socks", Domain: "test.example.com"}

	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "valid single mode",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels:  []TunnelConfig{validTunnel},
				Route:    RouteConfig{Mode: "single", Active: "tunnel-a"},
			},
			wantErr: "",
		},
		{
			name: "valid multi mode",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels:  []TunnelConfig{validTunnel},
				Route:    RouteConfig{Mode: "multi", Default: "tunnel-a"},
			},
			wantErr: "",
		},
		{
			name: "empty mode (defaults to single)",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels:  []TunnelConfig{validTunnel},
				Route:    RouteConfig{},
			},
			wantErr: "",
		},
		{
			name: "invalid mode",
			cfg: &Config{
				Route: RouteConfig{Mode: "invalid"},
			},
			wantErr: "route.mode must be 'single' or 'multi'",
		},
		{
			name: "single mode with nonexistent active tunnel",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels:  []TunnelConfig{validTunnel},
				Route:    RouteConfig{Mode: "single", Active: "nonexistent"},
			},
			wantErr: "route.active: tunnel 'nonexistent' does not exist",
		},
		{
			name: "nonexistent default tunnel",
			cfg: &Config{
				Backends: []BackendConfig{validBackend},
				Tunnels:  []TunnelConfig{validTunnel},
				Route:    RouteConfig{Mode: "multi", Default: "nonexistent"},
			},
			wantErr: "route.default: tunnel 'nonexistent' does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Validate() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestValidateShadowsocksMethod(t *testing.T) {
	validMethods := []string{
		"aes-256-gcm",
		"aes-128-gcm",
		"chacha20-ietf-poly1305",
	}

	for _, method := range validMethods {
		t.Run("valid_"+method, func(t *testing.T) {
			err := validateShadowsocksMethod(method)
			if err != nil {
				t.Errorf("validateShadowsocksMethod(%q) unexpected error: %v", method, err)
			}
		})
	}

	t.Run("valid_empty", func(t *testing.T) {
		err := validateShadowsocksMethod("")
		if err != nil {
			t.Errorf("validateShadowsocksMethod('') unexpected error: %v", err)
		}
	})

	invalidMethods := []string{"rc4", "aes-256-cfb", "invalid"}
	for _, method := range invalidMethods {
		t.Run("invalid_"+method, func(t *testing.T) {
			err := validateShadowsocksMethod(method)
			if err == nil {
				t.Errorf("validateShadowsocksMethod(%q) expected error", method)
			}
		})
	}
}

func TestGetSupportedShadowsocksMethods(t *testing.T) {
	methods := GetSupportedShadowsocksMethods()
	if len(methods) != 3 {
		t.Errorf("expected 3 methods, got %d", len(methods))
	}

	expectedMethods := map[string]bool{
		"aes-256-gcm":            true,
		"aes-128-gcm":            true,
		"chacha20-ietf-poly1305": true,
	}

	for _, m := range methods {
		if !expectedMethods[m] {
			t.Errorf("unexpected method: %q", m)
		}
	}
}
