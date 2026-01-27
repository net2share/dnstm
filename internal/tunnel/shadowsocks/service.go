package shadowsocks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/net2share/dnstm/internal/service"
)

// CreateService creates the systemd service for shadowsocks-slipstream.
func CreateService(cfg *Config) error {
	configPath := filepath.Join(ConfigDir, SSConfigFile)
	execStart := fmt.Sprintf("/usr/local/bin/%s -c %s", BinaryName, configPath)

	servicePath := service.GetServicePath(ServiceName)

	// Note: We use ReadWritePaths for ConfigDir because the slipstream plugin
	// may need to write certificate files on first run. ProtectSystem=strict
	// blocks writes to /etc by default, so we explicitly allow our config dir.
	serviceContent := fmt.Sprintf(`[Unit]
Description=Shadowsocks with Slipstream DNS Tunnel Plugin
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
ExecStart=%s
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=%s
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictRealtime=yes
RestrictSUIDSGID=yes
MemoryDenyWriteExecute=yes
LockPersonality=yes

[Install]
WantedBy=multi-user.target
`, ServiceUser, ServiceUser, execStart, ConfigDir)

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	exec.Command("systemctl", "daemon-reload").Run()

	return nil
}

// SetPermissions sets the correct permissions on configuration files.
func SetPermissions(cfg *Config) error {
	// Set ownership of key files
	if cfg.KeyFile != "" {
		exec.Command("chown", ServiceUser+":"+ServiceUser, cfg.KeyFile).Run()
		exec.Command("chmod", "600", cfg.KeyFile).Run()
	}
	if cfg.CertFile != "" {
		exec.Command("chown", ServiceUser+":"+ServiceUser, cfg.CertFile).Run()
		exec.Command("chmod", "644", cfg.CertFile).Run()
	}

	// Set ownership of config directory
	exec.Command("chown", "-R", ServiceUser+":"+ServiceUser, ConfigDir).Run()

	return nil
}

// Enable enables the shadowsocks-slipstream service.
func Enable() error {
	return service.EnableService(ServiceName)
}

// Start starts the shadowsocks-slipstream service.
func Start() error {
	return service.StartService(ServiceName)
}

// Stop stops the shadowsocks-slipstream service.
func Stop() error {
	return service.StopService(ServiceName)
}

// Restart restarts the shadowsocks-slipstream service.
func Restart() error {
	return service.RestartService(ServiceName)
}

// IsActive checks if the shadowsocks-slipstream service is active.
func IsActive() bool {
	return service.IsServiceActive(ServiceName)
}

// IsEnabled checks if the shadowsocks-slipstream service is enabled.
func IsEnabled() bool {
	return service.IsServiceEnabled(ServiceName)
}

// IsServiceInstalled checks if the shadowsocks-slipstream service unit exists.
func IsServiceInstalled() bool {
	return service.IsServiceInstalled(ServiceName)
}

// GetStatus returns the systemctl status output.
func GetStatus() (string, error) {
	return service.GetServiceStatus(ServiceName)
}

// GetLogs returns recent logs from the service.
func GetLogs(lines int) (string, error) {
	return service.GetServiceLogs(ServiceName, lines)
}

// Disable disables the shadowsocks-slipstream service.
func Disable() error {
	return service.DisableService(ServiceName)
}

// Remove removes the shadowsocks-slipstream service unit file.
func Remove() error {
	return service.RemoveService(ServiceName)
}
