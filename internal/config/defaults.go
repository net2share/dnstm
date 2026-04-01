package config

import (
	"fmt"
	"net"
)

const (
	// DefaultPortStart is the start of the port range for tunnel allocation.
	DefaultPortStart = 5310
	// DefaultPortEnd is the end of the port range for tunnel allocation.
	DefaultPortEnd = 5399
)

// ApplyDefaults fills in missing optional values with defaults.
func (c *Config) ApplyDefaults() {
	// Log defaults
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Timestamp == nil {
		t := true
		c.Log.Timestamp = &t
	}

	// Listen defaults
	if c.Listen.Address == "" {
		c.Listen.Address = "0.0.0.0:53"
	}

	// Route defaults
	if c.Route.Mode == "" {
		c.Route.Mode = "single"
	}

	// Tunnel defaults
	usedPorts := c.getUsedPorts()
	for i := range c.Tunnels {
		t := &c.Tunnels[i]

		// Auto-allocate port if not set
		if t.Port == 0 {
			t.Port = allocatePort(usedPorts)
			usedPorts[t.Port] = true
		}

		// Enabled defaults to true
		if t.Enabled == nil {
			enabled := true
			t.Enabled = &enabled
		}

		// Transport-specific defaults
		if t.Transport == TransportDNSTT {
			if t.DNSTT == nil {
				t.DNSTT = &DNSTTConfig{}
			}
			if t.DNSTT.MTU == 0 {
				t.DNSTT.MTU = 1232
			}
		}
		if t.Transport == TransportSlipstreamPlus {
			if t.SlipstreamPlus == nil {
				t.SlipstreamPlus = &SlipstreamPlusConfig{}
			}
			if t.SlipstreamPlus.MaxConnections == 0 {
				t.SlipstreamPlus.MaxConnections = 256
			}
			if t.SlipstreamPlus.IdleTimeoutSeconds == 0 {
				t.SlipstreamPlus.IdleTimeoutSeconds = 60
			}
		}
		if t.Transport == TransportVayDNS {
			if t.VayDNS == nil {
				t.VayDNS = &VayDNSConfig{}
			}
			if t.VayDNS.MTU == 0 {
				t.VayDNS.MTU = 1232
			}
			if !t.VayDNS.DnsttCompat && t.VayDNS.ClientIDSize == 0 {
				t.VayDNS.ClientIDSize = 2
			}
			if t.VayDNS.IdleTimeout == "" {
				if t.VayDNS.DnsttCompat {
					t.VayDNS.IdleTimeout = "2m"
				} else {
					t.VayDNS.IdleTimeout = "60s"
				}
			}
			if t.VayDNS.KeepAlive == "" {
				t.VayDNS.KeepAlive = "10s"
			}
			if t.VayDNS.QueueSize == 0 {
				t.VayDNS.QueueSize = 512
			}
			// KCPWindowSize 0 means queue-size/2 — leave it as 0 (vaydns-server handles default)
			// QueueOverflow "" means "drop" — leave empty (vaydns-server handles default)
			// LogLevel "" means "info" — leave empty (vaydns-server handles default)
		}
	}

	// Backend shadowsocks method default
	for i := range c.Backends {
		b := &c.Backends[i]
		if b.Type == BackendShadowsocks && b.Shadowsocks != nil {
			if b.Shadowsocks.Method == "" {
				b.Shadowsocks.Method = "aes-256-gcm"
			}
		}
	}

	// Route active/default defaults to first enabled tunnel
	if c.Route.Active == "" || c.Route.Default == "" {
		for _, t := range c.Tunnels {
			if t.IsEnabled() {
				if c.Route.Active == "" {
					c.Route.Active = t.Tag
				}
				if c.Route.Default == "" {
					c.Route.Default = t.Tag
				}
				break
			}
		}
	}
}

// getUsedPorts returns a map of all ports currently in use by tunnels.
func (c *Config) getUsedPorts() map[int]bool {
	ports := make(map[int]bool)
	for _, t := range c.Tunnels {
		if t.Port != 0 {
			ports[t.Port] = true
		}
	}
	return ports
}

// allocatePort finds the next available port in the tunnel port range.
// It checks both the config (usedPorts) and system (TCP/UDP binding).
func allocatePort(usedPorts map[int]bool) int {
	for port := DefaultPortStart; port <= DefaultPortEnd; port++ {
		if !usedPorts[port] && IsPortFree(port) {
			return port
		}
	}
	// Fallback to ports above the range
	for port := DefaultPortEnd + 1; port < 65535; port++ {
		if !usedPorts[port] && IsPortFree(port) {
			return port
		}
	}
	return 0 // Should not happen
}

// IsPortFree checks if a port is free on the system (both TCP and UDP on 127.0.0.1).
func IsPortFree(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()

	udpLn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return false
	}
	udpLn.Close()

	return true
}

// AllocateNextPort allocates the next available port for a new tunnel.
func (c *Config) AllocateNextPort() int {
	return allocatePort(c.getUsedPorts())
}

// EnsureBuiltinBackends ensures the default socks and ssh backends exist.
func (c *Config) EnsureBuiltinBackends() {
	hasSocks := false
	hasSSH := false

	for _, b := range c.Backends {
		if b.Tag == "socks" && b.Type == BackendSOCKS {
			hasSocks = true
		}
		if b.Tag == "ssh" && b.Type == BackendSSH {
			hasSSH = true
		}
	}

	// Determine socks port from config or default to 1080
	socksPort := 1080
	if c.Proxy.Port != 0 {
		socksPort = c.Proxy.Port
	}

	if !hasSocks {
		c.Backends = append([]BackendConfig{{
			Tag:     "socks",
			Type:    BackendSOCKS,
			Address: fmt.Sprintf("127.0.0.1:%d", socksPort),
		}}, c.Backends...)
	}

	if !hasSSH {
		c.Backends = append([]BackendConfig{{
			Tag:     "ssh",
			Type:    BackendSSH,
			Address: "127.0.0.1:22",
		}}, c.Backends...)
	}
}

// UpdateSocksBackendPort updates the socks backend port to match the proxy port.
func (c *Config) UpdateSocksBackendPort(port int) {
	for i := range c.Backends {
		if c.Backends[i].Tag == "socks" && c.Backends[i].Type == BackendSOCKS {
			c.Backends[i].Address = fmt.Sprintf("127.0.0.1:%d", port)
			return
		}
	}
}
