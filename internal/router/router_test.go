package router

import (
	"fmt"
	"strings"
	"testing"

	"github.com/net2share/dnstm/internal/config"
)

func TestValidatePort(t *testing.T) {
	tests := []struct {
		port    int
		wantErr bool
		errText string
	}{
		{port: 5310, wantErr: false},
		{port: 5350, wantErr: false},
		{port: 5399, wantErr: false},
		{port: 80, wantErr: true, errText: "privileged port"},
		{port: 1023, wantErr: true, errText: "privileged port"},
		{port: 70000, wantErr: true, errText: "out of range"},
		{port: 1080, wantErr: true, errText: "outside the router range"},
		{port: 5400, wantErr: true, errText: "outside the router range"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("port_%d", tt.port), func(t *testing.T) {
			err := ValidatePort(tt.port)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePort(%d) expected error", tt.port)
				} else if !strings.Contains(err.Error(), tt.errText) {
					t.Errorf("ValidatePort(%d) error = %q, want containing %q", tt.port, err.Error(), tt.errText)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePort(%d) unexpected error: %v", tt.port, err)
				}
			}
		})
	}
}

func TestGetPortRange(t *testing.T) {
	pr := GetPortRange()
	expected := "5310-5399"
	if pr != expected {
		t.Errorf("GetPortRange() = %q, want %q", pr, expected)
	}
}

func TestIsPortAvailable(t *testing.T) {
	cfg := &config.Config{
		Tunnels: []config.TunnelConfig{
			{Tag: "tunnel-a", Port: 5310},
			{Tag: "tunnel-b", Port: 5311},
		},
	}

	// Used port
	if IsPortAvailable(5310, cfg) {
		t.Error("IsPortAvailable(5310) should be false (already used)")
	}

	// Available port (assuming 5320 is not bound on the system)
	if !IsPortAvailable(5320, cfg) {
		t.Error("IsPortAvailable(5320) should be true")
	}

	// Out of range
	if IsPortAvailable(1080, cfg) {
		t.Error("IsPortAvailable(1080) should be false (out of range)")
	}
}

func TestAllocatePort(t *testing.T) {
	cfg := &config.Config{
		Tunnels: []config.TunnelConfig{
			{Tag: "tunnel-a", Port: 5310},
			{Tag: "tunnel-b", Port: 5311},
		},
	}

	port, err := AllocatePort(cfg)
	if err != nil {
		t.Fatalf("AllocatePort failed: %v", err)
	}

	// Should not be one of the used ports
	if port == 5310 || port == 5311 {
		t.Errorf("AllocatePort returned used port: %d", port)
	}

	// Should be in valid range
	if port < BasePort || port > MaxPort {
		t.Errorf("AllocatePort returned port outside range: %d", port)
	}
}

func TestPortConstants(t *testing.T) {
	if BasePort != 5310 {
		t.Errorf("BasePort = %d, want 5310", BasePort)
	}
	if MaxPort != 5399 {
		t.Errorf("MaxPort = %d, want 5399", MaxPort)
	}
}

func TestGenerateName(t *testing.T) {
	names := make(map[string]bool)

	for i := 0; i < 100; i++ {
		name := GenerateName()

		// Should be adjective-noun format
		parts := strings.Split(name, "-")
		if len(parts) != 2 {
			t.Errorf("GenerateName() = %q, expected adjective-noun format", name)
		}

		names[name] = true
	}

	// Should generate variety (at least 10 unique in 100 attempts)
	if len(names) < 10 {
		t.Errorf("GenerateName() only produced %d unique names in 100 attempts", len(names))
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		errText string
	}{
		{name: "swift-tunnel", wantErr: false},
		{name: "my-tunnel-123", wantErr: false},
		{name: "abc", wantErr: false},
		{name: "", wantErr: true, errText: "cannot be empty"},
		{name: "ab", wantErr: true, errText: "at least 3"},
		{name: strings.Repeat("a", 64), wantErr: true, errText: "at most 63"},
		{name: "123-tunnel", wantErr: true, errText: "start with a lowercase letter"},
		{name: "Tunnel", wantErr: true, errText: "lowercase"},
		{name: "tunnel_name", wantErr: true, errText: "lowercase"},
		{name: "tunnel.name", wantErr: true, errText: "lowercase"},
		{name: "coredns", wantErr: true, errText: "reserved"},
		{name: "router", wantErr: true, errText: "reserved"},
		{name: "default", wantErr: true, errText: "reserved"},
		{name: "all", wantErr: true, errText: "reserved"},
		{name: "none", wantErr: true, errText: "reserved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateName(%q) expected error", tt.name)
				} else if !strings.Contains(err.Error(), tt.errText) {
					t.Errorf("ValidateName(%q) error = %q, want containing %q", tt.name, err.Error(), tt.errText)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateName(%q) unexpected error: %v", tt.name, err)
				}
			}
		})
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "MyTunnel", expected: "mytunnel"},
		{input: "my_tunnel", expected: "my-tunnel"},
		{input: "my tunnel", expected: "my-tunnel"},
		{input: "My_Tunnel Name", expected: "my-tunnel-name"},
		{input: "already-normalized", expected: "already-normalized"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateUniqueTag(t *testing.T) {
	cfg := &config.Config{
		Tunnels: []config.TunnelConfig{},
	}

	tag := GenerateUniqueTag(cfg)
	if tag == "" {
		t.Error("GenerateUniqueTag returned empty string")
	}

	// Should validate
	if err := ValidateName(tag); err != nil {
		t.Errorf("GenerateUniqueTag returned invalid name: %v", err)
	}
}

func TestGenerateUniqueTag_Collision(t *testing.T) {
	// Create a config with a tunnel that has a generated-style tag
	cfg := &config.Config{
		Tunnels: []config.TunnelConfig{
			{Tag: "swift-tunnel"},
			{Tag: "quick-stream"},
		},
	}

	tags := make(map[string]bool)
	for i := 0; i < 50; i++ {
		tag := GenerateUniqueTag(cfg)
		if cfg.GetTunnelByTag(tag) != nil {
			t.Errorf("GenerateUniqueTag returned existing tag: %s", tag)
		}
		tags[tag] = true
	}
}

func TestSuggestSimilarTags(t *testing.T) {
	cfg := &config.Config{
		Tunnels: []config.TunnelConfig{
			{Tag: "swift-tunnel"},
			{Tag: "swift-tunnel-2"},
		},
	}

	suggestions := SuggestSimilarTags("swift-tunnel", cfg, 3)

	if len(suggestions) < 1 {
		t.Error("SuggestSimilarTags returned no suggestions")
	}

	// Suggestions should not include existing tags
	for _, s := range suggestions {
		if s == "swift-tunnel" || s == "swift-tunnel-2" {
			t.Errorf("SuggestSimilarTags included existing tag: %s", s)
		}
	}

	// Suggestions should be unique
	seen := make(map[string]bool)
	for _, s := range suggestions {
		if seen[s] {
			t.Errorf("SuggestSimilarTags returned duplicate: %s", s)
		}
		seen[s] = true
	}
}

func TestGetServiceName(t *testing.T) {
	tests := []struct {
		tag      string
		expected string
	}{
		{tag: "swift-tunnel", expected: "dnstm-swift-tunnel"},
		{tag: "my-tunnel", expected: "dnstm-my-tunnel"},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			result := GetServiceName(tt.tag)
			if result != tt.expected {
				t.Errorf("GetServiceName(%q) = %q, want %q", tt.tag, result, tt.expected)
			}
		})
	}
}

func TestGenerateUniqueTunnelTag(t *testing.T) {
	tunnels := []config.TunnelConfig{
		{Tag: "swift-tunnel"},
		{Tag: "quick-stream"},
	}

	tag := GenerateUniqueTunnelTag(tunnels)
	if tag == "" {
		t.Error("GenerateUniqueTunnelTag returned empty string")
	}

	// Should not be an existing tag
	for _, tunnel := range tunnels {
		if tunnel.Tag == tag {
			t.Errorf("GenerateUniqueTunnelTag returned existing tag: %s", tag)
		}
	}
}

func TestServiceMode(t *testing.T) {
	if ServiceModeSingle != "single" {
		t.Errorf("ServiceModeSingle = %q, want 'single'", ServiceModeSingle)
	}
	if ServiceModeMulti != "multi" {
		t.Errorf("ServiceModeMulti = %q, want 'multi'", ServiceModeMulti)
	}
}

func TestServiceGenerator_GetBindOptions_Multi(t *testing.T) {
	sg := NewServiceGenerator()

	cfg := &config.TunnelConfig{
		Tag:    "test-tunnel",
		Port:   5320,
		Domain: "test.example.com",
	}

	opts, err := sg.GetBindOptions(cfg, ServiceModeMulti)
	if err != nil {
		t.Fatalf("GetBindOptions failed: %v", err)
	}

	if opts.BindHost != "127.0.0.1" {
		t.Errorf("BindHost = %q, want '127.0.0.1'", opts.BindHost)
	}
	if opts.BindPort != 5320 {
		t.Errorf("BindPort = %d, want 5320", opts.BindPort)
	}
}
