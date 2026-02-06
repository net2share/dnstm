package router

import (
	"fmt"

	"github.com/net2share/dnstm/internal/config"
)

// IsPortAvailable checks if a port is available for use.
func IsPortAvailable(port int, cfg *config.Config) bool {
	// Check if port is in the valid range
	if port < config.DefaultPortStart || port > config.DefaultPortEnd {
		return false
	}

	// Check if port is already used by an existing tunnel
	for _, t := range cfg.Tunnels {
		if t.Port == port {
			return false
		}
	}

	// Check if port is actually free on the system
	return config.IsPortFree(port)
}

// ValidatePort checks if a port is valid for use.
func ValidatePort(port int) error {
	if port < 1024 {
		return fmt.Errorf("port %d is a privileged port (< 1024)", port)
	}

	if port > 65535 {
		return fmt.Errorf("port %d is out of range (> 65535)", port)
	}

	if port < config.DefaultPortStart || port > config.DefaultPortEnd {
		return fmt.Errorf("port %d is outside the router range (%d-%d)", port, config.DefaultPortStart, config.DefaultPortEnd)
	}

	return nil
}

// GetPortRange returns the port range as a string.
func GetPortRange() string {
	return fmt.Sprintf("%d-%d", config.DefaultPortStart, config.DefaultPortEnd)
}
