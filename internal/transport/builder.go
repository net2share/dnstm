package transport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/types"
)

const (
	SlipstreamBinary = "/usr/local/bin/slipstream-server"
	DNSTTBinary      = "/usr/local/bin/dnstt-server"
	SSServerBinary   = "/usr/local/bin/ssserver"
	ConfigDir        = "/etc/dnstm"
)

// BuildOptions configures how the transport should bind.
type BuildOptions struct {
	BindHost string // "127.0.0.1" for multi mode, or external IP for single mode
	BindPort int    // 53 for single mode, cfg.Port for multi mode
}

// BuildResult contains the result of building a transport command.
type BuildResult struct {
	Binary    string
	Args      []string
	ConfigDir string
	ExecStart string
}

// Builder builds command lines for transport instances.
type Builder struct {
	certMgr *certs.Manager
	keyMgr  *keys.Manager
}

// NewBuilder creates a new transport builder.
func NewBuilder() *Builder {
	return &Builder{
		certMgr: certs.NewManager(),
		keyMgr:  keys.NewManager(),
	}
}

// Build builds the command line for a transport instance.
// If opts is nil, defaults to multi-mode binding (127.0.0.1:cfg.Port).
func (b *Builder) Build(name string, cfg *types.TransportConfig, opts *BuildOptions) (*BuildResult, error) {
	// Default to multi-mode binding if no options provided
	if opts == nil {
		opts = &BuildOptions{
			BindHost: "127.0.0.1",
			BindPort: cfg.Port,
		}
	}

	switch cfg.Type {
	case types.TypeSlipstreamShadowsocks:
		return b.buildSlipstreamShadowsocks(name, cfg, opts)
	case types.TypeSlipstreamSocks:
		return b.buildSlipstreamSocks(name, cfg, opts)
	case types.TypeSlipstreamSSH:
		return b.buildSlipstreamSSH(name, cfg, opts)
	case types.TypeDNSTTSocks:
		return b.buildDNSTTSocks(name, cfg, opts)
	case types.TypeDNSTTSSH:
		return b.buildDNSTTSSH(name, cfg, opts)
	default:
		return nil, fmt.Errorf("unknown transport type: %s", cfg.Type)
	}
}

// buildSlipstreamShadowsocks builds the command for Shadowsocks with Slipstream plugin.
func (b *Builder) buildSlipstreamShadowsocks(name string, cfg *types.TransportConfig, opts *BuildOptions) (*BuildResult, error) {
	certInfo, err := b.certMgr.GetOrCreate(cfg.Domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate: %w", err)
	}

	method := cfg.Shadowsocks.Method
	if method == "" {
		method = "aes-256-gcm"
	}

	// Create instance config directory
	configDir := filepath.Join(ConfigDir, "instances", name)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}
	// Set ownership so dnstm user can access
	if err := system.ChownDirToDnstm(configDir); err != nil {
		return nil, fmt.Errorf("failed to set config directory ownership: %w", err)
	}

	// Write Shadowsocks config file
	// dns-listen-host and dns-listen-port tell slipstream-server where to listen for DNS queries
	pluginOpts := fmt.Sprintf("domain=%s;dns-listen-host=%s;dns-listen-port=%d;cert=%s;key=%s",
		cfg.Domain, opts.BindHost, opts.BindPort, certInfo.CertPath, certInfo.KeyPath)
	ssConfig := map[string]interface{}{
		"server":      opts.BindHost,
		"server_port": opts.BindPort,
		"password":    cfg.Shadowsocks.Password,
		"method":      method,
		"mode":        "tcp_only",
		"plugin":      SlipstreamBinary,
		"plugin_opts": pluginOpts,
		"plugin_mode": "tcp_only",
	}

	configPath := filepath.Join(configDir, "config.json")
	data, err := json.MarshalIndent(ssConfig, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}
	// Set ownership so dnstm user can read
	if err := system.ChownToDnstm(configPath); err != nil {
		return nil, fmt.Errorf("failed to set config file ownership: %w", err)
	}

	execStart := fmt.Sprintf("%s -c %s", SSServerBinary, configPath)

	return &BuildResult{
		Binary:    SSServerBinary,
		Args:      []string{"-c", configPath},
		ConfigDir: configDir,
		ExecStart: execStart,
	}, nil
}

// buildSlipstreamSocks builds the command for Slipstream standalone SOCKS mode.
func (b *Builder) buildSlipstreamSocks(name string, cfg *types.TransportConfig, opts *BuildOptions) (*BuildResult, error) {
	certInfo, err := b.certMgr.GetOrCreate(cfg.Domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate: %w", err)
	}

	configDir := filepath.Join(ConfigDir, "instances", name)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}
	// Set ownership so dnstm user can access
	if err := system.ChownDirToDnstm(configDir); err != nil {
		return nil, fmt.Errorf("failed to set config directory ownership: %w", err)
	}

	// slipstream-server --dns-listen-host HOST --domain t.example.com --dns-listen-port PORT --target-address 127.0.0.1:1080 --cert cert.pem --key key.pem
	// In single mode: binds to EXTERNAL_IP:53
	// In multi mode: binds to 127.0.0.1:cfg.Port (DNS router forwards traffic)
	args := []string{
		"--dns-listen-host", opts.BindHost,
		"--domain", cfg.Domain,
		"--dns-listen-port", fmt.Sprintf("%d", opts.BindPort),
		"--target-address", cfg.Target.Address,
		"--cert", certInfo.CertPath,
		"--key", certInfo.KeyPath,
	}

	execStart := fmt.Sprintf("%s %s", SlipstreamBinary, formatArgs(args))

	return &BuildResult{
		Binary:    SlipstreamBinary,
		Args:      args,
		ConfigDir: configDir,
		ExecStart: execStart,
	}, nil
}

// buildSlipstreamSSH builds the command for Slipstream standalone SSH mode.
func (b *Builder) buildSlipstreamSSH(name string, cfg *types.TransportConfig, opts *BuildOptions) (*BuildResult, error) {
	// Same as SOCKS mode but pointing to SSH port
	return b.buildSlipstreamSocks(name, cfg, opts)
}

// buildDNSTTSocks builds the command for DNSTT SOCKS mode.
func (b *Builder) buildDNSTTSocks(name string, cfg *types.TransportConfig, opts *BuildOptions) (*BuildResult, error) {
	keyInfo, err := b.keyMgr.GetOrCreate(cfg.Domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	configDir := filepath.Join(ConfigDir, "instances", name)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}
	// Set ownership so dnstm user can access
	if err := system.ChownDirToDnstm(configDir); err != nil {
		return nil, fmt.Errorf("failed to set config directory ownership: %w", err)
	}

	mtu := "1232"
	if cfg.DNSTT != nil && cfg.DNSTT.MTU > 0 {
		mtu = fmt.Sprintf("%d", cfg.DNSTT.MTU)
	}

	// dnstt-server -udp HOST:PORT -privkey-file key.key -mtu 1232 t.example.com 127.0.0.1:1080
	// In single mode: binds to EXTERNAL_IP:53
	// In multi mode: binds to 127.0.0.1:cfg.Port (DNS router forwards traffic)
	args := []string{
		"-udp", fmt.Sprintf("%s:%d", opts.BindHost, opts.BindPort),
		"-privkey-file", keyInfo.PrivateKeyPath,
		"-mtu", mtu,
		cfg.Domain,
		cfg.Target.Address,
	}

	execStart := fmt.Sprintf("%s %s", DNSTTBinary, formatArgs(args))

	return &BuildResult{
		Binary:    DNSTTBinary,
		Args:      args,
		ConfigDir: configDir,
		ExecStart: execStart,
	}, nil
}

// buildDNSTTSSH builds the command for DNSTT SSH mode.
func (b *Builder) buildDNSTTSSH(name string, cfg *types.TransportConfig, opts *BuildOptions) (*BuildResult, error) {
	// Same as SOCKS mode but pointing to SSH port
	return b.buildDNSTTSocks(name, cfg, opts)
}

// GetCertInfo returns certificate info for a domain.
func (b *Builder) GetCertInfo(domain string) (*certs.CertInfo, error) {
	return b.certMgr.GetOrCreate(domain)
}

// GetKeyInfo returns key info for a domain.
func (b *Builder) GetKeyInfo(domain string) (*keys.KeyInfo, error) {
	return b.keyMgr.GetOrCreate(domain)
}

// formatArgs joins args with spaces.
func formatArgs(args []string) string {
	return strings.Join(args, " ")
}

// RequiresBinary checks if the required binary is installed for a transport type.
func RequiresBinary(t types.TransportType) (string, bool) {
	switch t {
	case types.TypeSlipstreamShadowsocks:
		if _, err := os.Stat(SSServerBinary); err != nil {
			return "ssserver", false
		}
		if _, err := os.Stat(SlipstreamBinary); err != nil {
			return "slipstream-server", false
		}
		return "", true
	case types.TypeSlipstreamSocks, types.TypeSlipstreamSSH:
		if _, err := os.Stat(SlipstreamBinary); err != nil {
			return "slipstream-server", false
		}
		return "", true
	case types.TypeDNSTTSocks, types.TypeDNSTTSSH:
		if _, err := os.Stat(DNSTTBinary); err != nil {
			return "dnstt-server", false
		}
		return "", true
	default:
		return "", true
	}
}
