package config

// TransportType defines the type of transport.
type TransportType string

const (
	TransportSlipstream     TransportType = "slipstream"
	TransportSlipstreamPlus TransportType = "slipstream-plus"
	TransportDNSTT          TransportType = "dnstt"
	TransportVayDNS         TransportType = "vaydns"
)

// TunnelConfig configures a DNS tunnel.
type TunnelConfig struct {
	Tag            string                `json:"tag"`
	Enabled        *bool                 `json:"enabled,omitempty"`
	Transport      TransportType         `json:"transport"`
	Backend        string                `json:"backend"`
	Domain         string                `json:"domain"`
	Port           int                   `json:"port,omitempty"`
	Slipstream     *SlipstreamConfig     `json:"slipstream,omitempty"`
	SlipstreamPlus *SlipstreamPlusConfig `json:"slipstream_plus,omitempty"`
	DNSTT          *DNSTTConfig          `json:"dnstt,omitempty"`
	VayDNS         *VayDNSConfig         `json:"vaydns,omitempty"`
}

// SlipstreamConfig holds Slipstream-specific configuration.
type SlipstreamConfig struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

// SlipstreamPlusConfig holds Slipstream Plus-specific configuration.
type SlipstreamPlusConfig struct {
	Cert               string `json:"cert,omitempty"`
	Key                string `json:"key,omitempty"`
	MaxConnections     int    `json:"max_connections,omitempty"`
	IdleTimeoutSeconds int    `json:"idle_timeout_seconds,omitempty"`
	Fallback           string `json:"fallback,omitempty"`
	ResetSeed          string `json:"reset_seed,omitempty"`
}

// DNSTTConfig holds DNSTT-specific configuration.
type DNSTTConfig struct {
	MTU        int    `json:"mtu,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
}

// VayDNSConfig holds VayDNS-specific configuration.
type VayDNSConfig struct {
	MTU            int    `json:"mtu,omitempty"`
	PrivateKey     string `json:"private_key,omitempty"`
	IdleTimeout    string `json:"idle_timeout,omitempty"`
	KeepAlive      string `json:"keep_alive,omitempty"`
	Fallback       string `json:"fallback,omitempty"`
	DnsttCompat    bool   `json:"dnstt_compat,omitempty"`
	ClientIDSize   int    `json:"clientid_size,omitempty"`
	QueueSize      int    `json:"queue_size,omitempty"`
	KCPWindowSize  int    `json:"kcp_window_size,omitempty"`
	QueueOverflow  string `json:"queue_overflow,omitempty"`
	LogLevel       string `json:"log_level,omitempty"`
	RecordType     string `json:"record_type,omitempty"`
}

// ValidVayDNSRecordTypes returns the valid record types for VayDNS.
var ValidVayDNSRecordTypes = []string{"txt", "cname", "a", "aaaa", "mx", "ns", "srv"}

// ResolvedVayDNSIdleTimeout returns the idle-timeout string for vaydns-server, applying defaults when empty.
func (v *VayDNSConfig) ResolvedVayDNSIdleTimeout() string {
	if v == nil {
		return "60s"
	}
	if v.IdleTimeout != "" {
		return v.IdleTimeout
	}
	if v.DnsttCompat {
		return "2m"
	}
	return "60s"
}

// ResolvedVayDNSKeepAlive returns the keepalive string for vaydns-server, applying defaults when empty.
func (v *VayDNSConfig) ResolvedVayDNSKeepAlive() string {
	if v == nil {
		return "10s"
	}
	if v.KeepAlive != "" {
		return v.KeepAlive
	}
	return "10s"
}

// VayDNSClientIDSizeForFlag returns the value for -clientid-size, or 0 if the flag must be omitted (-dnstt-compat).
func (v *VayDNSConfig) VayDNSClientIDSizeForFlag() int {
	if v == nil {
		return 2
	}
	if v.DnsttCompat {
		return 0
	}
	if v.ClientIDSize <= 0 {
		return 2
	}
	return v.ClientIDSize
}

// IsEnabled returns true if the tunnel is enabled.
func (t *TunnelConfig) IsEnabled() bool {
	return t.Enabled == nil || *t.Enabled
}

// GetMTU returns the MTU for DNSTT/VayDNS tunnels, with a default of 1232.
func (t *TunnelConfig) GetMTU() int {
	if t.DNSTT != nil && t.DNSTT.MTU > 0 {
		return t.DNSTT.MTU
	}
	if t.VayDNS != nil && t.VayDNS.MTU > 0 {
		return t.VayDNS.MTU
	}
	return 1232 // Default
}

// IsSlipstream returns true if this is a Slipstream tunnel.
func (t *TunnelConfig) IsSlipstream() bool {
	return t.Transport == TransportSlipstream
}

// IsSlipstreamPlus returns true if this is a Slipstream Plus tunnel.
func (t *TunnelConfig) IsSlipstreamPlus() bool {
	return t.Transport == TransportSlipstreamPlus
}

// IsDNSTT returns true if this is a DNSTT tunnel.
func (t *TunnelConfig) IsDNSTT() bool {
	return t.Transport == TransportDNSTT
}

// IsVayDNS returns true if this is a VayDNS tunnel.
func (t *TunnelConfig) IsVayDNS() bool {
	return t.Transport == TransportVayDNS
}

// GetTransportTypes returns all available transport types.
func GetTransportTypes() []TransportType {
	return []TransportType{
		TransportSlipstream,
		TransportSlipstreamPlus,
		TransportDNSTT,
		TransportVayDNS,
	}
}

// GetTransportTypeDisplayName returns a human-readable name for a transport type.
func GetTransportTypeDisplayName(t TransportType) string {
	switch t {
	case TransportSlipstream:
		return "Slipstream"
	case TransportSlipstreamPlus:
		return "Slipstream Plus"
	case TransportDNSTT:
		return "DNSTT"
	case TransportVayDNS:
		return "VayDNS"
	default:
		return string(t)
	}
}
