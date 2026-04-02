package transport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/net2share/dnstm/internal/binary"
	"github.com/net2share/dnstm/internal/config"
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

// VayDNSBinaryPath returns the path to vaydns-server.
func VayDNSBinaryPath() string {
	path, _ := getBinManager().GetPath(binary.BinaryVayDNSServer)
	return path
}

// SlipstreamPlusBinaryPath returns the path to slipstream-plus-server.
func SlipstreamPlusBinaryPath() string {
	path, _ := getBinManager().GetPath(binary.BinarySlipstreamPlusServer)
	return path
}

// BuildOptions configures how the transport should bind.
type BuildOptions struct {
	BindHost string // "127.0.0.1" for multi mode, or external IP for single mode
	BindPort int    // 53 for single mode, cfg.Port for multi mode
}

// Builder builds command lines for transport instances.
type Builder struct{}

// NewBuilder creates a new transport builder.
func NewBuilder() *Builder {
	return &Builder{}
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
	case config.TransportVayDNS:
		return b.buildVayDNSTunnel(tunnel, backend, targetAddr, opts, result)
	case config.TransportSlipstreamPlus:
		return b.buildSlipstreamPlusTunnel(tunnel, backend, targetAddr, opts, result)
	default:
		return nil, fmt.Errorf("unknown transport type: %s", tunnel.Transport)
	}
}

// buildSlipstreamTunnel builds a Slipstream-based tunnel service.
func (b *Builder) buildSlipstreamTunnel(tunnel *config.TunnelConfig, backend *config.BackendConfig, targetAddr string, opts *BuildOptions, result *TunnelBuildResult) (*TunnelBuildResult, error) {
	// Read cert/key paths from tunnel config (already set before builder is called)
	if tunnel.Slipstream == nil || tunnel.Slipstream.Cert == "" || tunnel.Slipstream.Key == "" {
		return nil, fmt.Errorf("slipstream cert/key paths not set for tunnel %s", tunnel.Tag)
	}

	certPath := tunnel.Slipstream.Cert
	keyPath := tunnel.Slipstream.Key

	result.ReadPaths = append(result.ReadPaths, certPath, keyPath)

	// Slipstream + Shadowsocks uses ssserver with slipstream as plugin (SIP003)
	if backend.Type == config.BackendShadowsocks {
		return b.buildSlipstreamShadowsocksTunnel(tunnel, backend, certPath, keyPath, opts, result)
	}

	// Slipstream standalone mode (SOCKS, SSH, or custom target)
	args := []string{
		"--dns-listen-host", opts.BindHost,
		"--domain", tunnel.Domain,
		"--dns-listen-port", fmt.Sprintf("%d", opts.BindPort),
		"--target-address", targetAddr,
		"--cert", certPath,
		"--key", keyPath,
	}

	result.ExecStart = fmt.Sprintf("%s %s", SlipstreamBinaryPath(), strings.Join(args, " "))
	return result, nil
}

// buildSlipstreamShadowsocksTunnel builds a Slipstream+Shadowsocks tunnel using SIP003 plugin mode.
func (b *Builder) buildSlipstreamShadowsocksTunnel(tunnel *config.TunnelConfig, backend *config.BackendConfig, certPath, keyPath string, opts *BuildOptions, result *TunnelBuildResult) (*TunnelBuildResult, error) {
	if backend.Shadowsocks == nil {
		return nil, fmt.Errorf("shadowsocks backend missing configuration")
	}

	method := backend.Shadowsocks.Method
	if method == "" {
		method = "aes-256-gcm"
	}

	// Build plugin options
	pluginOpts := fmt.Sprintf("domain=%s;dns-listen-host=%s;dns-listen-port=%d;cert=%s;key=%s",
		tunnel.Domain, opts.BindHost, opts.BindPort, certPath, keyPath)

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

// buildSlipstreamPlusTunnel builds a Slipstream Plus-based tunnel service.
func (b *Builder) buildSlipstreamPlusTunnel(tunnel *config.TunnelConfig, backend *config.BackendConfig, targetAddr string, opts *BuildOptions, result *TunnelBuildResult) (*TunnelBuildResult, error) {
	if tunnel.SlipstreamPlus == nil || tunnel.SlipstreamPlus.Cert == "" || tunnel.SlipstreamPlus.Key == "" {
		return nil, fmt.Errorf("slipstream-plus cert/key paths not set for tunnel %s", tunnel.Tag)
	}

	certPath := tunnel.SlipstreamPlus.Cert
	keyPath := tunnel.SlipstreamPlus.Key
	result.ReadPaths = append(result.ReadPaths, certPath, keyPath)

	if backend.Type == config.BackendShadowsocks {
		return b.buildSlipstreamPlusShadowsocksTunnel(tunnel, backend, certPath, keyPath, opts, result)
	}

	args := []string{
		"--dns-listen-host", opts.BindHost,
		"--domain", tunnel.Domain,
		"--dns-listen-port", fmt.Sprintf("%d", opts.BindPort),
		"--target-address", targetAddr,
		"--cert", certPath,
		"--key", keyPath,
	}
	if tunnel.SlipstreamPlus.MaxConnections > 0 {
		args = append(args, "--max-connections", fmt.Sprintf("%d", tunnel.SlipstreamPlus.MaxConnections))
	}
	if tunnel.SlipstreamPlus.IdleTimeoutSeconds > 0 {
		args = append(args, "--idle-timeout-seconds", fmt.Sprintf("%d", tunnel.SlipstreamPlus.IdleTimeoutSeconds))
	}
	if tunnel.SlipstreamPlus.Fallback != "" {
		args = append(args, "--fallback", tunnel.SlipstreamPlus.Fallback)
	}
	if tunnel.SlipstreamPlus.ResetSeed != "" {
		args = append(args, "--reset-seed", tunnel.SlipstreamPlus.ResetSeed)
		result.ReadPaths = append(result.ReadPaths, tunnel.SlipstreamPlus.ResetSeed)
	}

	result.ExecStart = fmt.Sprintf("%s %s", SlipstreamPlusBinaryPath(), strings.Join(args, " "))
	return result, nil
}

// buildSlipstreamPlusShadowsocksTunnel builds a Slipstream Plus + Shadowsocks tunnel using SIP003 plugin mode.
func (b *Builder) buildSlipstreamPlusShadowsocksTunnel(tunnel *config.TunnelConfig, backend *config.BackendConfig, certPath, keyPath string, opts *BuildOptions, result *TunnelBuildResult) (*TunnelBuildResult, error) {
	if backend.Shadowsocks == nil {
		return nil, fmt.Errorf("shadowsocks backend missing configuration")
	}

	method := backend.Shadowsocks.Method
	if method == "" {
		method = "aes-256-gcm"
	}

	pluginOpts := fmt.Sprintf("domain=%s;dns-listen-host=%s;dns-listen-port=%d;cert=%s;key=%s",
		tunnel.Domain, opts.BindHost, opts.BindPort, certPath, keyPath)
	if tunnel.SlipstreamPlus.MaxConnections > 0 {
		pluginOpts += fmt.Sprintf(";max-connections=%d", tunnel.SlipstreamPlus.MaxConnections)
	}
	if tunnel.SlipstreamPlus.IdleTimeoutSeconds > 0 {
		pluginOpts += fmt.Sprintf(";idle-timeout-seconds=%d", tunnel.SlipstreamPlus.IdleTimeoutSeconds)
	}
	if tunnel.SlipstreamPlus.Fallback != "" {
		pluginOpts += fmt.Sprintf(";fallback=%s", tunnel.SlipstreamPlus.Fallback)
	}

	ssConfig := map[string]interface{}{
		"server":      opts.BindHost,
		"server_port": opts.BindPort,
		"password":    backend.Shadowsocks.Password,
		"method":      method,
		"mode":        "tcp_only",
		"plugin":      SlipstreamPlusBinaryPath(),
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

	// Read key path from tunnel config (already set before builder is called)
	if tunnel.DNSTT == nil || tunnel.DNSTT.PrivateKey == "" {
		return nil, fmt.Errorf("dnstt private key path not set for tunnel %s", tunnel.Tag)
	}

	privKeyPath := tunnel.DNSTT.PrivateKey
	result.ReadPaths = append(result.ReadPaths, privKeyPath)

	mtu := "1232"
	if tunnel.DNSTT.MTU > 0 {
		mtu = fmt.Sprintf("%d", tunnel.DNSTT.MTU)
	}

	// Build dnstt-server command
	args := []string{
		"-udp", fmt.Sprintf("%s:%d", opts.BindHost, opts.BindPort),
		"-privkey-file", privKeyPath,
		"-mtu", mtu,
		tunnel.Domain,
		targetAddr,
	}

	result.ExecStart = fmt.Sprintf("%s %s", DNSTTBinaryPath(), strings.Join(args, " "))
	return result, nil
}

// buildVayDNSTunnel builds a VayDNS-based tunnel service.
func (b *Builder) buildVayDNSTunnel(tunnel *config.TunnelConfig, backend *config.BackendConfig, targetAddr string, opts *BuildOptions, result *TunnelBuildResult) (*TunnelBuildResult, error) {
	if backend.Type == config.BackendShadowsocks {
		return nil, fmt.Errorf("VayDNS transport does not support Shadowsocks backend")
	}

	if tunnel.VayDNS == nil || tunnel.VayDNS.PrivateKey == "" {
		return nil, fmt.Errorf("vaydns private key path not set for tunnel %s", tunnel.Tag)
	}

	privKeyPath := tunnel.VayDNS.PrivateKey
	result.ReadPaths = append(result.ReadPaths, privKeyPath)

	mtu := "1232"
	if tunnel.VayDNS.MTU > 0 {
		mtu = fmt.Sprintf("%d", tunnel.VayDNS.MTU)
	}

	args := []string{
		"-udp", fmt.Sprintf("%s:%d", opts.BindHost, opts.BindPort),
		"-privkey-file", privKeyPath,
		"-mtu", mtu,
		"-domain", tunnel.Domain,
		"-upstream", targetAddr,
		"-idle-timeout", tunnel.VayDNS.ResolvedVayDNSIdleTimeout(),
		"-keepalive", tunnel.VayDNS.ResolvedVayDNSKeepAlive(),
	}

	if tunnel.VayDNS.Fallback != "" {
		args = append(args, "-fallback", tunnel.VayDNS.Fallback)
	}
	if tunnel.VayDNS.DnsttCompat {
		args = append(args, "-dnstt-compat")
	}
	if n := tunnel.VayDNS.VayDNSClientIDSizeForFlag(); n > 0 {
		args = append(args, "-clientid-size", strconv.Itoa(n))
	}
	if tunnel.VayDNS.QueueSize > 0 && tunnel.VayDNS.QueueSize != 512 {
		args = append(args, "-queue-size", strconv.Itoa(tunnel.VayDNS.QueueSize))
	}
	if tunnel.VayDNS.KCPWindowSize > 0 {
		args = append(args, "-kcp-window-size", strconv.Itoa(tunnel.VayDNS.KCPWindowSize))
	}
	if tunnel.VayDNS.QueueOverflow != "" && tunnel.VayDNS.QueueOverflow != "drop" {
		args = append(args, "-queue-overflow", tunnel.VayDNS.QueueOverflow)
	}
	if tunnel.VayDNS.LogLevel != "" && tunnel.VayDNS.LogLevel != "info" {
		args = append(args, "-log-level", tunnel.VayDNS.LogLevel)
	}
	if tunnel.VayDNS.RecordType != "" && tunnel.VayDNS.RecordType != "txt" {
		args = append(args, "-record-type", tunnel.VayDNS.RecordType)
	}

	result.ExecStart = fmt.Sprintf("%s %s", VayDNSBinaryPath(), strings.Join(args, " "))
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
