package config

import "fmt"

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
func allocatePort(usedPorts map[int]bool) int {
	for port := DefaultPortStart; port <= DefaultPortEnd; port++ {
		if !usedPorts[port] {
			return port
		}
	}
	// Fallback to ports above the range
	for port := DefaultPortEnd + 1; port < 65535; port++ {
		if !usedPorts[port] {
			return port
		}
	}
	return 0 // Should not happen
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
