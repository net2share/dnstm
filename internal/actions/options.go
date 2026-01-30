package actions

import (
	"github.com/net2share/dnstm/internal/config"
)

// EncryptionMethodOptions returns the available Shadowsocks encryption methods.
func EncryptionMethodOptions() []SelectOption {
	return []SelectOption{
		{
			Label:       "AES-256-GCM",
			Value:       "aes-256-gcm",
			Description: "Recommended for most systems",
			Recommended: true,
		},
		{
			Label:       "ChaCha20-IETF-Poly1305",
			Value:       "chacha20-ietf-poly1305",
			Description: "Better for ARM/mobile devices",
		},
		{
			Label:       "AES-128-GCM",
			Value:       "aes-128-gcm",
			Description: "Lighter encryption",
		},
	}
}

// OperatingModeOptions returns the available operating mode options.
func OperatingModeOptions() []SelectOption {
	return []SelectOption{
		{
			Label:       "Single-tunnel",
			Value:       "single",
			Description: "One tunnel at a time, lower overhead",
			Recommended: true,
		},
		{
			Label:       "Multi-tunnel",
			Value:       "multi",
			Description: "Multiple tunnels with DNS-based routing",
		},
	}
}

// GetTransportTypeByValue returns the transport type for a value.
func GetTransportTypeByValue(value string) config.TransportType {
	return config.TransportType(value)
}

// ValidTransportTypes returns all valid transport type values.
func ValidTransportTypes() []string {
	return []string{
		string(config.TransportSlipstream),
		string(config.TransportDNSTT),
	}
}
