package config

import (
	"fmt"
	"regexp"
)

var tagRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if err := c.validateTagUniqueness(); err != nil {
		return err
	}

	if err := c.validateBackends(); err != nil {
		return err
	}

	if err := c.validateTunnels(); err != nil {
		return err
	}

	if err := c.validateRoute(); err != nil {
		return err
	}

	return nil
}

// validateTagUniqueness ensures all tags are unique within their scope.
func (c *Config) validateTagUniqueness() error {
	// Check backend tags
	backendTags := make(map[string]bool)
	for i, b := range c.Backends {
		if b.Tag == "" {
			return fmt.Errorf("backends[%d]: tag is required", i)
		}
		if !tagRegex.MatchString(b.Tag) {
			return fmt.Errorf("backend '%s': tag must start with a letter and contain only alphanumeric characters, underscores, and hyphens", b.Tag)
		}
		if backendTags[b.Tag] {
			return fmt.Errorf("duplicate backend tag: %s", b.Tag)
		}
		backendTags[b.Tag] = true
	}

	// Check tunnel tags
	tunnelTags := make(map[string]bool)
	for i, t := range c.Tunnels {
		if t.Tag == "" {
			return fmt.Errorf("tunnels[%d]: tag is required", i)
		}
		if !tagRegex.MatchString(t.Tag) {
			return fmt.Errorf("tunnel '%s': tag must start with a letter and contain only alphanumeric characters, underscores, and hyphens", t.Tag)
		}
		if tunnelTags[t.Tag] {
			return fmt.Errorf("duplicate tunnel tag: %s", t.Tag)
		}
		tunnelTags[t.Tag] = true
	}

	return nil
}

// validateBackends validates all backend configurations.
func (c *Config) validateBackends() error {
	for _, b := range c.Backends {
		if b.Type == "" {
			return fmt.Errorf("backend '%s': type is required", b.Tag)
		}

		switch b.Type {
		case BackendSOCKS, BackendSSH, BackendCustom:
			if b.Address == "" {
				return fmt.Errorf("backend '%s': address is required for type %s", b.Tag, b.Type)
			}
		case BackendShadowsocks:
			if b.Shadowsocks == nil {
				return fmt.Errorf("backend '%s': shadowsocks config is required for type %s", b.Tag, b.Type)
			}
			if b.Shadowsocks.Password == "" {
				return fmt.Errorf("backend '%s': shadowsocks.password is required", b.Tag)
			}
			if err := validateShadowsocksMethod(b.Shadowsocks.Method); err != nil {
				return fmt.Errorf("backend '%s': %w", b.Tag, err)
			}
		default:
			return fmt.Errorf("backend '%s': unknown type %s", b.Tag, b.Type)
		}
	}

	return nil
}

// validateTunnels validates all tunnel configurations.
func (c *Config) validateTunnels() error {
	usedPorts := make(map[int]string)
	usedDomains := make(map[string]string)

	for _, t := range c.Tunnels {
		if t.Transport == "" {
			return fmt.Errorf("tunnel '%s': transport is required", t.Tag)
		}

		if t.Transport != TransportSlipstream && t.Transport != TransportDNSTT {
			return fmt.Errorf("tunnel '%s': unknown transport %s", t.Tag, t.Transport)
		}

		if t.Backend == "" {
			return fmt.Errorf("tunnel '%s': backend is required", t.Tag)
		}

		if t.Domain == "" {
			return fmt.Errorf("tunnel '%s': domain is required", t.Tag)
		}

		// Check backend reference
		backend := c.GetBackendByTag(t.Backend)
		if backend == nil {
			return fmt.Errorf("tunnel '%s': backend '%s' not found", t.Tag, t.Backend)
		}

		// Check transport-backend compatibility
		if err := validateTransportBackendCompatibility(t.Transport, backend.Type); err != nil {
			return fmt.Errorf("tunnel '%s': %w", t.Tag, err)
		}

		// Check port uniqueness (if port is set)
		if t.Port != 0 {
			if t.Port < 1024 || t.Port > 65535 {
				return fmt.Errorf("tunnel '%s': port must be between 1024 and 65535", t.Tag)
			}
			if existing, ok := usedPorts[t.Port]; ok {
				return fmt.Errorf("tunnel '%s': port %d already used by %s", t.Tag, t.Port, existing)
			}
			usedPorts[t.Port] = t.Tag
		}

		// Check domain uniqueness
		if existing, ok := usedDomains[t.Domain]; ok {
			return fmt.Errorf("tunnel '%s': domain '%s' already used by %s", t.Tag, t.Domain, existing)
		}
		usedDomains[t.Domain] = t.Tag

		// Validate DNSTT-specific config
		if t.Transport == TransportDNSTT && t.DNSTT != nil {
			if t.DNSTT.MTU != 0 && (t.DNSTT.MTU < 512 || t.DNSTT.MTU > 1400) {
				return fmt.Errorf("tunnel '%s': dnstt.mtu must be between 512 and 1400", t.Tag)
			}
		}
	}

	return nil
}

// validateRoute validates route configuration.
func (c *Config) validateRoute() error {
	// Validate mode
	if c.Route.Mode != "" && c.Route.Mode != "single" && c.Route.Mode != "multi" {
		return fmt.Errorf("route.mode must be 'single' or 'multi'")
	}

	// In single mode, validate active tunnel exists
	if c.IsSingleMode() && c.Route.Active != "" {
		if c.GetTunnelByTag(c.Route.Active) == nil {
			return fmt.Errorf("route.active: tunnel '%s' does not exist", c.Route.Active)
		}
	}

	// Validate default route exists
	if c.Route.Default != "" {
		if c.GetTunnelByTag(c.Route.Default) == nil {
			return fmt.Errorf("route.default: tunnel '%s' does not exist", c.Route.Default)
		}
	}

	return nil
}

// validateTransportBackendCompatibility checks if a transport and backend are compatible.
func validateTransportBackendCompatibility(transport TransportType, backend BackendType) error {
	// DNSTT doesn't support shadowsocks (no SIP003 plugin support)
	if transport == TransportDNSTT && backend == BackendShadowsocks {
		return fmt.Errorf("dnstt transport does not support shadowsocks backend (no SIP003 plugin support)")
	}
	return nil
}

// validateShadowsocksMethod validates the shadowsocks encryption method.
func validateShadowsocksMethod(method string) error {
	if method == "" {
		return nil // Default will be applied
	}
	validMethods := []string{
		"aes-256-gcm",
		"aes-128-gcm",
		"chacha20-ietf-poly1305",
	}
	for _, m := range validMethods {
		if method == m {
			return nil
		}
	}
	return fmt.Errorf("invalid shadowsocks method '%s', must be one of: aes-256-gcm, aes-128-gcm, chacha20-ietf-poly1305", method)
}

// GetSupportedShadowsocksMethods returns the list of supported shadowsocks methods.
func GetSupportedShadowsocksMethods() []string {
	return []string{
		"aes-256-gcm",
		"aes-128-gcm",
		"chacha20-ietf-poly1305",
	}
}
