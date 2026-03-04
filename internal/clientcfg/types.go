package clientcfg

// ClientConfig is the JSON payload embedded in a dnst:// URL.
type ClientConfig struct {
	Version   int             `json:"v"`
	Tag       string          `json:"tag"`
	Transport TransportConfig `json:"transport"`
	Backend   BackendConfig   `json:"backend"`
}

// TransportConfig describes the DNS transport layer.
type TransportConfig struct {
	Type   string `json:"type"`             // "slipstream" or "dnstt"
	Domain string `json:"domain"`           // NS domain
	Cert   string `json:"cert,omitempty"`   // PEM string (slipstream)
	PubKey string `json:"pubkey,omitempty"` // 64-char hex (dnstt)
}

// BackendConfig describes the backend service behind the tunnel.
type BackendConfig struct {
	Type     string `json:"type"`               // "socks", "ssh", "shadowsocks"
	User     string `json:"user,omitempty"`     // ssh
	Password string `json:"password,omitempty"` // ssh, shadowsocks
	Key      string `json:"key,omitempty"`      // ssh (private key PEM)
	Method   string `json:"method,omitempty"`   // shadowsocks
}
