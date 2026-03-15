package config

import "fmt"

// TransportType defines the type of transport.
type TransportType string

const (
	TransportSlipstream TransportType = "slipstream"
	TransportDNSTT      TransportType = "dnstt"
	TransportMasterDNS  TransportType = "masterdns"
)

// TunnelConfig configures a DNS tunnel.
type TunnelConfig struct {
	Tag        string            `json:"tag"`
	Enabled    *bool             `json:"enabled,omitempty"`
	Transport  TransportType     `json:"transport"`
	Backend    string            `json:"backend"`
	Domain     string            `json:"domain"`
	Port       int               `json:"port,omitempty"`
	Slipstream *SlipstreamConfig `json:"slipstream,omitempty"`
	DNSTT      *DNSTTConfig      `json:"dnstt,omitempty"`
	MasterDNS  *MasterDNSConfig  `json:"masterdns,omitempty"`
}

// SlipstreamConfig holds Slipstream-specific configuration.
type SlipstreamConfig struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

// DNSTTConfig holds DNSTT-specific configuration.
type DNSTTConfig struct {
	MTU        int    `json:"mtu,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
}

// MasterDNSConfig holds MasterDnsVPN-specific configuration.
type MasterDNSConfig struct {
	EncryptionKey    string `json:"encryption_key,omitempty"`
	EncryptionMethod int    `json:"encryption_method"` // 0=None,1=XOR,2=ChaCha20,3=AES-128-GCM,4=AES-192-GCM,5=AES-256-GCM
}

// IsEnabled returns true if the tunnel is enabled.
func (t *TunnelConfig) IsEnabled() bool {
	return t.Enabled == nil || *t.Enabled
}

// GetMTU returns the MTU for DNSTT tunnels, with a default of 1232.
func (t *TunnelConfig) GetMTU() int {
	if t.DNSTT != nil && t.DNSTT.MTU > 0 {
		return t.DNSTT.MTU
	}
	return 1232 // Default
}

// IsSlipstream returns true if this is a Slipstream tunnel.
func (t *TunnelConfig) IsSlipstream() bool {
	return t.Transport == TransportSlipstream
}

// IsDNSTT returns true if this is a DNSTT tunnel.
func (t *TunnelConfig) IsDNSTT() bool {
	return t.Transport == TransportDNSTT
}

// IsMasterDNS returns true if this is a MasterDnsVPN tunnel.
func (t *TunnelConfig) IsMasterDNS() bool {
	return t.Transport == TransportMasterDNS
}

// GetTransportTypes returns all available transport types.
func GetTransportTypes() []TransportType {
	return []TransportType{
		TransportSlipstream,
		TransportDNSTT,
		TransportMasterDNS,
	}
}

// GetTransportTypeDisplayName returns a human-readable name for a transport type.
func GetTransportTypeDisplayName(t TransportType) string {
	switch t {
	case TransportSlipstream:
		return "Slipstream"
	case TransportDNSTT:
		return "DNSTT"
	case TransportMasterDNS:
		return "MasterDNS"
	default:
		return string(t)
	}
}

// GetMasterDNSEncryptionMethodName returns a human-readable name for a MasterDNS encryption method.
func GetMasterDNSEncryptionMethodName(method int) string {
	switch method {
	case 0:
		return "None"
	case 1:
		return "XOR (recommended)"
	case 2:
		return "ChaCha20"
	case 3:
		return "AES-128-GCM"
	case 4:
		return "AES-192-GCM"
	case 5:
		return "AES-256-GCM"
	default:
		return fmt.Sprintf("Unknown(%d)", method)
	}
}
