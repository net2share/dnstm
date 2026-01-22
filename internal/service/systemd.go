package service

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/net2share/dnstm/internal/config"
)

const (
	ServiceName = "dnstt-server"
	ServicePath = "/etc/systemd/system/dnstt-server.service"
)

func CreateService(cfg *config.Config) error {
	targetAddr := "127.0.0.1:" + cfg.TargetPort

	serviceContent := fmt.Sprintf(`[Unit]
Description=dnstt DNS Tunnel Server
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=dnstt
Group=dnstt
ExecStart=/usr/local/bin/dnstt-server -udp :%s -privkey-file %s -mtu %s %s %s
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictRealtime=yes
RestrictSUIDSGID=yes
MemoryDenyWriteExecute=yes
LockPersonality=yes

# Allow binding to privileged ports
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

# Read access to key files
ReadOnlyPaths=/etc/dnstt

[Install]
WantedBy=multi-user.target
`, config.DnsttPort, cfg.PrivateKeyFile, cfg.MTU, cfg.NSSubdomain, targetAddr)

	if err := os.WriteFile(ServicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	exec.Command("systemctl", "daemon-reload").Run()

	return nil
}

func SetPermissions() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Set ownership of key files to dnstt user
	if cfg.PrivateKeyFile != "" {
		exec.Command("chown", "dnstt:dnstt", cfg.PrivateKeyFile).Run()
		exec.Command("chmod", "600", cfg.PrivateKeyFile).Run()
	}
	if cfg.PublicKeyFile != "" {
		exec.Command("chown", "dnstt:dnstt", cfg.PublicKeyFile).Run()
		exec.Command("chmod", "644", cfg.PublicKeyFile).Run()
	}

	// Set ownership of config directory
	exec.Command("chown", "-R", "dnstt:dnstt", config.ConfigDir).Run()

	return nil
}

func Enable() error {
	cmd := exec.Command("systemctl", "enable", ServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable service: %s: %w", string(output), err)
	}
	return nil
}

func Start() error {
	cmd := exec.Command("systemctl", "start", ServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start service: %s: %w", string(output), err)
	}
	return nil
}

func Stop() error {
	cmd := exec.Command("systemctl", "stop", ServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop service: %s: %w", string(output), err)
	}
	return nil
}

func Restart() error {
	cmd := exec.Command("systemctl", "restart", ServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart service: %s: %w", string(output), err)
	}
	return nil
}

func IsActive() bool {
	cmd := exec.Command("systemctl", "is-active", ServiceName)
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output)) == "active"
}

func IsEnabled() bool {
	cmd := exec.Command("systemctl", "is-enabled", ServiceName)
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output)) == "enabled"
}

func IsInstalled() bool {
	_, err := os.Stat(ServicePath)
	return err == nil
}

func Status() (string, error) {
	cmd := exec.Command("systemctl", "status", ServiceName, "--no-pager", "-l")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func GetLogs(lines int) (string, error) {
	cmd := exec.Command("journalctl", "-u", ServiceName, "-n", fmt.Sprintf("%d", lines), "--no-pager")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	return string(output), nil
}
