// Package dnsrouter provides DNS forwarding with domain-based routing.
//
// # Architecture
//
// The package uses an interface-based design to allow swapping implementations:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                     DNSForwarder Interface                  │
//	├─────────────────────────────────────────────────────────────┤
//	│  Start() error                                              │
//	│  Stop() error                                               │
//	│  Stats() (queries, errors uint64)                           │
//	│  GetRoutes() []Route                                        │
//	│  GetDefaultBackend() string                                 │
//	└─────────────────────────────────────────────────────────────┘
//	                              ▲
//	              ┌───────────────┼───────────────┐
//	              │               │               │
//	      ┌───────┴───────┐ ┌─────┴─────┐ ┌───────┴───────┐
//	      │ Router        │ │ CoreDNS   │ │ eBPF          │
//	      │ (native Go)   │ │ (future)  │ │ (future)      │
//	      └───────────────┘ └───────────┘ └───────────────┘
//
// # Adding a New Implementation
//
// To add a new forwarder implementation (e.g., CoreDNS, eBPF):
//
//  1. Create a new type that implements DNSForwarder interface
//  2. Add a new ForwarderType constant
//  3. Add a case in NewForwarder() factory function
//
// Example for CoreDNS:
//
//	// In coredns_forwarder.go
//	type CoreDNSForwarder struct {
//	    // ... fields
//	}
//
//	func (f *CoreDNSForwarder) Start() error { ... }
//	func (f *CoreDNSForwarder) Stop() error { ... }
//	// ... implement other methods
//
//	// In forwarder.go, add:
//	const ForwarderTypeCoreDNS ForwarderType = "coredns"
//
//	// In NewForwarder(), add:
//	case ForwarderTypeCoreDNS:
//	    return NewCoreDNSForwarder(cfg)
//
// # Configuration
//
// The forwarder type can be configured in dnsrouter.yaml:
//
//	listen: 0.0.0.0:53
//	forwarder_type: native  # or "coredns", "ebpf" when available
//	routes:
//	  - domain: example.com
//	    backend: 127.0.0.1:5310
//
package dnsrouter

// DNSForwarder defines the interface for DNS forwarding implementations.
// Any alternative implementation (e.g., CoreDNS, raw eBPF forwarder)
// should implement this interface to be swappable.
type DNSForwarder interface {
	// Start starts the DNS forwarder.
	Start() error

	// Stop stops the DNS forwarder.
	Stop() error

	// Stats returns query and error counts.
	Stats() (queries, errors uint64)

	// GetRoutes returns the configured routes.
	GetRoutes() []Route

	// GetDefaultBackend returns the default backend address.
	GetDefaultBackend() string
}

// ForwarderConfig contains configuration for creating a DNS forwarder.
type ForwarderConfig struct {
	ListenAddr     string
	Routes         []Route
	DefaultBackend string
}

// ForwarderType identifies the DNS forwarder implementation.
type ForwarderType string

const (
	// ForwarderTypeNative is the built-in Go UDP forwarder with connection pooling.
	ForwarderTypeNative ForwarderType = "native"

	// ForwarderTypeCoreDNS would be CoreDNS-based forwarding (future).
	// ForwarderTypeCoreDNS ForwarderType = "coredns"

	// ForwarderTypeEBPF would be eBPF-based forwarding (future).
	// ForwarderTypeEBPF ForwarderType = "ebpf"
)

// NewForwarder creates a DNS forwarder of the specified type.
// This is the factory function that should be used to create forwarders,
// allowing easy switching between implementations.
func NewForwarder(ftype ForwarderType, cfg ForwarderConfig) (DNSForwarder, error) {
	switch ftype {
	case ForwarderTypeNative:
		return NewRouter(cfg.ListenAddr, cfg.Routes, cfg.DefaultBackend), nil
	// Future implementations:
	// case ForwarderTypeCoreDNS:
	//     return NewCoreDNSForwarder(cfg)
	// case ForwarderTypeEBPF:
	//     return NewEBPFForwarder(cfg)
	default:
		return NewRouter(cfg.ListenAddr, cfg.Routes, cfg.DefaultBackend), nil
	}
}

// Ensure Router implements DNSForwarder
var _ DNSForwarder = (*Router)(nil)
