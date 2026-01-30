package actions

import (
	"github.com/net2share/dnstm/internal/types"
)

// TransportTypeOptions returns the available transport type options.
func TransportTypeOptions() []SelectOption {
	return []SelectOption{
		{
			Label:       "Slipstream + Shadowsocks",
			Value:       string(types.TypeSlipstreamShadowsocks),
			Description: "Encrypted SOCKS5 proxy over DNS",
			Recommended: true,
		},
		{
			Label:       "Slipstream SOCKS",
			Value:       string(types.TypeSlipstreamSocks),
			Description: "Direct SOCKS5 proxy over DNS",
		},
		{
			Label:       "Slipstream SSH",
			Value:       string(types.TypeSlipstreamSSH),
			Description: "SSH over DNS tunnel",
		},
		{
			Label:       "DNSTT SOCKS",
			Value:       string(types.TypeDNSTTSocks),
			Description: "DNSTT-based SOCKS5 proxy",
		},
		{
			Label:       "DNSTT SSH",
			Value:       string(types.TypeDNSTTSSH),
			Description: "DNSTT-based SSH tunnel",
		},
	}
}

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
func GetTransportTypeByValue(value string) types.TransportType {
	return types.TransportType(value)
}

// ValidTransportTypes returns all valid transport type values.
func ValidTransportTypes() []string {
	return []string{
		string(types.TypeSlipstreamShadowsocks),
		string(types.TypeSlipstreamSocks),
		string(types.TypeSlipstreamSSH),
		string(types.TypeDNSTTSocks),
		string(types.TypeDNSTTSSH),
	}
}
