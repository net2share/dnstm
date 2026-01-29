package types

// TransportType defines the type of transport.
type TransportType string

const (
	TypeSlipstreamShadowsocks TransportType = "slipstream-shadowsocks"
	TypeSlipstreamSocks       TransportType = "slipstream-socks"
	TypeSlipstreamSSH         TransportType = "slipstream-ssh"
	TypeSlipstreamMTProxy     TransportType = "slipstream-mtproxy"
	TypeDNSTTSocks            TransportType = "dnstt-socks"
	TypeDNSTTSSH              TransportType = "dnstt-ssh"
	TypeDNSTTMTProxy          TransportType = "dnstt-mtproxy"
)

// TransportConfig configures a transport instance.
type TransportConfig struct {
	Type        TransportType      `yaml:"type"`
	Domain      string             `yaml:"domain"`
	Port        int                `yaml:"port"`
	Shadowsocks *ShadowsocksConfig `yaml:"shadowsocks,omitempty"`
	DNSTT       *DNSTTConfig       `yaml:"dnstt,omitempty"`
	MTProxy     *MTProxyConfig     `yaml:"mtproxy,omitempty"`
	Target      *TargetConfig      `yaml:"target,omitempty"`
}

// ShadowsocksConfig holds Shadowsocks-specific configuration.
type ShadowsocksConfig struct {
	Password string `yaml:"password"`
	Method   string `yaml:"method,omitempty"`
}

// DNSTTConfig holds DNSTT-specific configuration.
type DNSTTConfig struct {
	MTU int `yaml:"mtu,omitempty"`
}

// MTProxyConfig holds MTProxy-specific configuration.
type MTProxyConfig struct {
	Secret string `yaml:"secret"`
}

// TargetConfig specifies the target address for forwarding.
type TargetConfig struct {
	Address string `yaml:"address"`
}

// IsSlipstreamType returns true if the transport type uses Slipstream.
func IsSlipstreamType(t TransportType) bool {
	return t == TypeSlipstreamShadowsocks || t == TypeSlipstreamSocks || t == TypeSlipstreamSSH || t == TypeSlipstreamMTProxy
}

// IsDNSTTType returns true if the transport type uses DNSTT.
func IsDNSTTType(t TransportType) bool {
	return t == TypeDNSTTSocks || t == TypeDNSTTSSH || t == TypeDNSTTMTProxy
}

// GetTransportTypeDisplayName returns a human-readable name for a transport type.
func GetTransportTypeDisplayName(t TransportType) string {
	switch t {
	case TypeSlipstreamShadowsocks:
		return "Slipstream + Shadowsocks"
	case TypeSlipstreamSocks:
		return "Slipstream SOCKS"
	case TypeSlipstreamSSH:
		return "Slipstream SSH"
	case TypeSlipstreamMTProxy:
		return "Slipstream + MTProxy"
	case TypeDNSTTSocks:
		return "DNSTT SOCKS"
	case TypeDNSTTSSH:
		return "DNSTT SSH"
	case TypeDNSTTMTProxy:
		return "DNSTT + MTProxy (socat)"
	default:
		return string(t)
	}
}
