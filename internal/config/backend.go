package config

import "os"

// BackendType defines the type of backend.
type BackendType string

const (
	BackendSOCKS       BackendType = "socks"
	BackendSSH         BackendType = "ssh"
	BackendShadowsocks BackendType = "shadowsocks"
	BackendCustom      BackendType = "custom"
)

// BackendConfig configures a backend service.
type BackendConfig struct {
	Tag         string             `json:"tag"`
	Type        BackendType        `json:"type"`
	Address     string             `json:"address,omitempty"`
	Shadowsocks *ShadowsocksConfig `json:"shadowsocks,omitempty"`
}

// ShadowsocksConfig holds Shadowsocks-specific configuration.
type ShadowsocksConfig struct {
	Method   string `json:"method,omitempty"`
	Password string `json:"password"`
}

// IsManaged returns true if dnstm manages this backend type.
func (b *BackendConfig) IsManaged() bool {
	switch b.Type {
	case BackendSOCKS, BackendShadowsocks:
		return true
	default:
		return false
	}
}

// IsBuiltIn returns true if this is a built-in backend type.
func (b *BackendConfig) IsBuiltIn() bool {
	return b.Type == BackendSOCKS || b.Type == BackendSSH
}

// BackendCategory defines the management category of a backend type.
type BackendCategory string

const (
	CategoryBuiltIn BackendCategory = "builtin" // Always installed (socks, shadowsocks)
	CategorySystem  BackendCategory = "system"  // External system service (ssh)
	CategoryCustom  BackendCategory = "custom"  // User-provided
)

// BackendTypeInfo provides metadata about a backend type.
type BackendTypeInfo struct {
	Type        BackendType
	Name        string
	Description string
	Category    BackendCategory
	Binary      string
}

// BackendTypeRegistry maps backend types to their metadata.
var BackendTypeRegistry = map[BackendType]BackendTypeInfo{
	BackendSOCKS: {
		Type:        BackendSOCKS,
		Name:        "SOCKS5",
		Description: "Built-in SOCKS5 proxy (microsocks)",
		Category:    CategoryBuiltIn,
		Binary:      "/usr/local/bin/microsocks",
	},
	BackendSSH: {
		Type:        BackendSSH,
		Name:        "SSH",
		Description: "System SSH server",
		Category:    CategorySystem,
	},
	BackendShadowsocks: {
		Type:        BackendShadowsocks,
		Name:        "Shadowsocks",
		Description: "Shadowsocks proxy (SIP003)",
		Category:    CategoryBuiltIn,
		Binary:      "/usr/local/bin/ssserver",
	},
	BackendCustom: {
		Type:        BackendCustom,
		Name:        "Custom",
		Description: "Custom TCP service",
		Category:    CategoryCustom,
	},
}

// IsInstalled returns true if the backend type's binary is available.
func (info *BackendTypeInfo) IsInstalled() bool {
	if info.Category == CategorySystem || info.Category == CategoryCustom {
		return true
	}
	if info.Binary == "" {
		return false
	}
	_, err := os.Stat(info.Binary)
	return err == nil
}

// GetBackendTypeInfo returns metadata for a backend type.
func GetBackendTypeInfo(t BackendType) *BackendTypeInfo {
	if info, ok := BackendTypeRegistry[t]; ok {
		return &info
	}
	return nil
}

// GetBackendTypes returns all available backend types.
func GetBackendTypes() []BackendType {
	return []BackendType{
		BackendSOCKS,
		BackendSSH,
		BackendShadowsocks,
		BackendCustom,
	}
}

// GetBackendTypeDisplayName returns a human-readable name for a backend type.
func GetBackendTypeDisplayName(t BackendType) string {
	if info := GetBackendTypeInfo(t); info != nil {
		return info.Name
	}
	return string(t)
}
