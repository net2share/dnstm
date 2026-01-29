package dnsrouter

import (
	"testing"
)

func TestNewForwarder(t *testing.T) {
	cfg := ForwarderConfig{
		ListenAddr:     "127.0.0.1:15353",
		Routes:         []Route{{Domain: "example.com", Backend: "127.0.0.1:5310"}},
		DefaultBackend: "127.0.0.1:5310",
	}

	tests := []struct {
		name  string
		ftype ForwarderType
	}{
		{"native forwarder", ForwarderTypeNative},
		{"empty type defaults to native", ""},
		{"unknown type defaults to native", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewForwarder(tt.ftype, cfg)
			if err != nil {
				t.Fatalf("NewForwarder() error = %v", err)
			}
			if f == nil {
				t.Fatal("NewForwarder() returned nil")
			}

			// Verify it implements DNSForwarder
			var _ DNSForwarder = f

			// Verify routes are set
			routes := f.GetRoutes()
			if len(routes) != 1 {
				t.Errorf("GetRoutes() = %d routes, want 1", len(routes))
			}
			if routes[0].Domain != "example.com" {
				t.Errorf("GetRoutes()[0].Domain = %q, want %q", routes[0].Domain, "example.com")
			}

			// Verify default backend
			if f.GetDefaultBackend() != "127.0.0.1:5310" {
				t.Errorf("GetDefaultBackend() = %q, want %q", f.GetDefaultBackend(), "127.0.0.1:5310")
			}
		})
	}
}

func TestRouterImplementsDNSForwarder(t *testing.T) {
	// This test verifies at compile time that Router implements DNSForwarder
	var _ DNSForwarder = (*Router)(nil)
}
