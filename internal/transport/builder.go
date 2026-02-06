package transport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/net2share/dnstm/internal/binary"
	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/system"
)

const (
	ConfigDir = "/etc/dnstm"
)

// Binary path getters using the binary manager.
// These return the path based on the current environment (test vs production).
var (
	binManager *binary.Manager
)

func getBinManager() *binary.Manager {
	if binManager == nil {
		binManager = binary.NewDefaultManager()
	}
	return binManager
}

// SlipstreamBinaryPath returns the path to slipstream-server.
func SlipstreamBinaryPath() string {
	path, _ := getBinManager().GetPath(binary.BinarySlipstreamServer)
	return path
}

// DNSTTBinaryPath returns the path to dnstt-server.
func DNSTTBinaryPath() string {
	path, _ := getBinManager().GetPath(binary.BinaryDNSTTServer)
	return path
}

// SSServerBinaryPath returns the path to ssserver.
func SSServerBinaryPath() string {
	path, _ := getBinManager().GetPath(binary.BinarySSServer)
	return path
}

// SSHTunUserBinaryPath returns the path to sshtun-user.
func SSHTunUserBinaryPath() string {
	path, _ := getBinManager().GetPath(binary.BinarySSHTunUser)
	return path
}

// BuildOptions configures how the transport should bind.
type BuildOptions struct {
	BindHost string // "127.0.0.1" for multi mode, or external IP for single mode
	BindPort int    // 53 for single mode, cfg.Port for multi mode
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

// GetCertInfo returns certificate info for a domain.
func (b *Builder) GetCertInfo(domain string) (*certs.CertInfo, error) {
	return b.certMgr.GetOrCreate(domain)
}

// GetKeyInfo returns key info for a domain.
func (b *Builder) GetKeyInfo(domain string) (*keys.KeyInfo, error) {
	return b.keyMgr.GetOrCreate(domain)
}

// TunnelBuildResult contains the result of building a tunnel service.
type TunnelBuildResult struct {
	ExecStart    string
	ConfigDir    string
	ReadPaths    []string
	WritePaths   []string
	BindToPort53 bool
}

// CreateService creates a systemd service for the tunnel.
func (r *TunnelBuildResult) CreateService(serviceName string) error {
	cfg := &service.ServiceConfig{
		Name:             serviceName,
		Description:      fmt.Sprintf("dnstm tunnel: %s", serviceName),
		User:             system.DnstmUser,
		Group:            system.DnstmUser,
		ExecStart:        r.ExecStart,
		ReadOnlyPaths:    r.ReadPaths,
		ReadWritePaths:   r.WritePaths,
		BindToPrivileged: r.BindToPort53,
	}
	return service.CreateGenericService(cfg)
}

// BuildTunnelService builds the service configuration for a tunnel with the new config types.
// This bridges between the new config types and the existing builder logic.
func (b *Builder) BuildTunnelService(tunnel *config.TunnelConfig, backend *config.BackendConfig, opts *BuildOptions) (*TunnelBuildResult, error) {
	if opts == nil {
		opts = &BuildOptions{
			BindHost: "127.0.0.1",
			BindPort: tunnel.Port,
		}
	}

	result := &TunnelBuildResult{
		BindToPort53: opts.BindPort == 53,
	}

	// Create tunnel config directory
	configDir := filepath.Join(ConfigDir, "tunnels", tunnel.Tag)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := system.ChownDirToDnstm(configDir); err != nil {
		return nil, fmt.Errorf("failed to set config directory ownership: %w", err)
	}
	result.ConfigDir = configDir

	// Get target address from backend
	targetAddr := backend.Address
	if targetAddr == "" {
		// Default addresses based on backend type
		switch backend.Type {
		case config.BackendSOCKS:
			targetAddr = "127.0.0.1:1080"
		case config.BackendSSH:
			targetAddr = "127.0.0.1:22"
		}
	}

	switch tunnel.Transport {
	case config.TransportSlipstream:
		return b.buildSlipstreamTunnel(tunnel, backend, targetAddr, opts, result)
	case config.TransportDNSTT:
		return b.buildDNSTTTunnel(tunnel, backend, targetAddr, opts, result)
	default:
		return nil, fmt.Errorf("unknown transport type: %s", tunnel.Transport)
	}
}

// buildSlipstreamTunnel builds a Slipstream-based tunnel service.
func (b *Builder) buildSlipstreamTunnel(tunnel *config.TunnelConfig, backend *config.BackendConfig, targetAddr string, opts *BuildOptions, result *TunnelBuildResult) (*TunnelBuildResult, error) {
	// Get certificate
	certInfo, err := b.certMgr.GetOrCreate(tunnel.Domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate: %w", err)
	}

	result.ReadPaths = append(result.ReadPaths, certInfo.CertPath, certInfo.KeyPath)

	// Slipstream + Shadowsocks uses ssserver with slipstream as plugin (SIP003)
	if backend.Type == config.BackendShadowsocks {
		return b.buildSlipstreamShadowsocksTunnel(tunnel, backend, certInfo, opts, result)
	}

	// Slipstream standalone mode (SOCKS, SSH, or custom target)
	args := []string{
		"--dns-listen-host", opts.BindHost,
		"--domain", tunnel.Domain,
		"--dns-listen-port", fmt.Sprintf("%d", opts.BindPort),
		"--target-address", targetAddr,
		"--cert", certInfo.CertPath,
		"--key", certInfo.KeyPath,
	}

	result.ExecStart = fmt.Sprintf("%s %s", SlipstreamBinaryPath(), strings.Join(args, " "))
	return result, nil
}

// buildSlipstreamShadowsocksTunnel builds a Slipstream+Shadowsocks tunnel using SIP003 plugin mode.
func (b *Builder) buildSlipstreamShadowsocksTunnel(tunnel *config.TunnelConfig, backend *config.BackendConfig, certInfo *certs.CertInfo, opts *BuildOptions, result *TunnelBuildResult) (*TunnelBuildResult, error) {
	if backend.Shadowsocks == nil {
		return nil, fmt.Errorf("shadowsocks backend missing configuration")
	}

	method := backend.Shadowsocks.Method
	if method == "" {
		method = "aes-256-gcm"
	}

	// Build plugin options
	pluginOpts := fmt.Sprintf("domain=%s;dns-listen-host=%s;dns-listen-port=%d;cert=%s;key=%s",
		tunnel.Domain, opts.BindHost, opts.BindPort, certInfo.CertPath, certInfo.KeyPath)

	// Write Shadowsocks config file
	ssConfig := map[string]interface{}{
		"server":      opts.BindHost,
		"server_port": opts.BindPort,
		"password":    backend.Shadowsocks.Password,
		"method":      method,
		"mode":        "tcp_only",
		"plugin":      SlipstreamBinaryPath(),
		"plugin_opts": pluginOpts,
		"plugin_mode": "tcp_only",
	}

	configPath := filepath.Join(result.ConfigDir, "config.json")
	data, err := json.MarshalIndent(ssConfig, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}
	if err := system.ChownToDnstm(configPath); err != nil {
		return nil, fmt.Errorf("failed to set config file ownership: %w", err)
	}

	result.ExecStart = fmt.Sprintf("%s -c %s", SSServerBinaryPath(), configPath)
	result.ReadPaths = append(result.ReadPaths, configPath)

	return result, nil
}

// buildDNSTTTunnel builds a DNSTT-based tunnel service.
func (b *Builder) buildDNSTTTunnel(tunnel *config.TunnelConfig, backend *config.BackendConfig, targetAddr string, opts *BuildOptions, result *TunnelBuildResult) (*TunnelBuildResult, error) {
	// DNSTT doesn't support Shadowsocks
	if backend.Type == config.BackendShadowsocks {
		return nil, fmt.Errorf("DNSTT transport does not support Shadowsocks backend")
	}

	// Get keys
	keyInfo, err := b.keyMgr.GetOrCreate(tunnel.Domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	result.ReadPaths = append(result.ReadPaths, keyInfo.PrivateKeyPath)

	mtu := "1232"
	if tunnel.DNSTT != nil && tunnel.DNSTT.MTU > 0 {
		mtu = fmt.Sprintf("%d", tunnel.DNSTT.MTU)
	}

	// Build dnstt-server command
	args := []string{
		"-udp", fmt.Sprintf("%s:%d", opts.BindHost, opts.BindPort),
		"-privkey-file", keyInfo.PrivateKeyPath,
		"-mtu", mtu,
		tunnel.Domain,
		targetAddr,
	}

	result.ExecStart = fmt.Sprintf("%s %s", DNSTTBinaryPath(), strings.Join(args, " "))
	return result, nil
}

// RegenerateTunnelService regenerates a tunnel's systemd service with new bind options.
// This is used when switching active tunnels in single mode.
func (b *Builder) RegenerateTunnelService(tunnel *config.TunnelConfig, backend *config.BackendConfig, opts *BuildOptions) error {
	serviceName := fmt.Sprintf("dnstm-%s", tunnel.Tag)

	// Stop the service if it's running
	if service.IsServiceActive(serviceName) {
		if err := service.StopService(serviceName); err != nil {
			return fmt.Errorf("failed to stop service: %w", err)
		}
	}

	// Remove the old service
	if service.IsServiceInstalled(serviceName) {
		if err := service.RemoveService(serviceName); err != nil {
			return fmt.Errorf("failed to remove old service: %w", err)
		}
	}

	// Build and create the new service
	result, err := b.BuildTunnelService(tunnel, backend, opts)
	if err != nil {
		return fmt.Errorf("failed to build service: %w", err)
	}

	if err := result.CreateService(serviceName); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	return nil
}
