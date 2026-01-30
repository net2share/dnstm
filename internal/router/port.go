package router

import (
	"fmt"
	"net"

	"github.com/net2share/dnstm/internal/config"
)

const (
	BasePort = config.DefaultPortStart
	MaxPort  = config.DefaultPortEnd
)

// AllocatePort finds the next available port for a tunnel.
func AllocatePort(cfg *config.Config) (int, error) {
	usedPorts := make(map[int]bool)
	for _, t := range cfg.Tunnels {
		usedPorts[t.Port] = true
	}

	for port := BasePort; port <= MaxPort; port++ {
		if !usedPorts[port] && isPortFree(port) {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", BasePort, MaxPort)
}

// IsPortAvailable checks if a port is available for use.
func IsPortAvailable(port int, cfg *config.Config) bool {
	// Check if port is in the valid range
	if port < BasePort || port > MaxPort {
		return false
	}

	// Check if port is already used by an existing tunnel
	for _, t := range cfg.Tunnels {
		if t.Port == port {
			return false
		}
	}

	// Check if port is actually free on the system
	return isPortFree(port)
}

// isPortFree checks if a port is free on the system.
func isPortFree(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()

	// Also check UDP
	udpAddr := fmt.Sprintf("127.0.0.1:%d", port)
	udpLn, err := net.ListenPacket("udp", udpAddr)
	if err != nil {
		return false
	}
	udpLn.Close()

	return true
}

// ValidatePort checks if a port is valid for use.
func ValidatePort(port int) error {
	if port < 1024 {
		return fmt.Errorf("port %d is a privileged port (< 1024)", port)
	}

	if port > 65535 {
		return fmt.Errorf("port %d is out of range (> 65535)", port)
	}

	if port < BasePort || port > MaxPort {
		return fmt.Errorf("port %d is outside the router range (%d-%d)", port, BasePort, MaxPort)
	}

	return nil
}

// GetPortRange returns the port range as a string.
func GetPortRange() string {
	return fmt.Sprintf("%d-%d", BasePort, MaxPort)
}
