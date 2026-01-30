package service

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ServiceConfig contains configuration for a systemd service.
type ServiceConfig struct {
	Name             string   // Service name (e.g., "dnstt-server", "slipstream-server")
	Description      string
	User             string
	Group            string
	ExecStart        string
	ReadOnlyPaths    []string // Paths that should be read-only
	ReadWritePaths   []string // Paths that should be read-write
	BindToPrivileged bool     // Whether service needs CAP_NET_BIND_SERVICE
}

// GetServicePath returns the systemd service file path for a service name.
func GetServicePath(serviceName string) string {
	return fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
}

// CreateGenericService creates a systemd service with the given configuration.
func CreateGenericService(cfg *ServiceConfig) error {
	servicePath := GetServicePath(cfg.Name)

	// Build paths directives
	var pathsSection string
	for _, p := range cfg.ReadOnlyPaths {
		pathsSection += fmt.Sprintf("ReadOnlyPaths=%s\n", p)
	}
	for _, p := range cfg.ReadWritePaths {
		pathsSection += fmt.Sprintf("ReadWritePaths=%s\n", p)
	}

	// Build capabilities section
	var capsSection string
	if cfg.BindToPrivileged {
		capsSection = "AmbientCapabilities=CAP_NET_BIND_SERVICE\nCapabilityBoundingSet=CAP_NET_BIND_SERVICE\n"
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=%s
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
%s%sProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictRealtime=yes
RestrictSUIDSGID=yes
MemoryDenyWriteExecute=yes
LockPersonality=yes

[Install]
WantedBy=multi-user.target
`, cfg.Description, cfg.User, cfg.Group, cfg.ExecStart, pathsSection, capsSection)

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	exec.Command("systemctl", "daemon-reload").Run()

	return nil
}

// EnableService enables a systemd service.
func EnableService(serviceName string) error {
	cmd := exec.Command("systemctl", "enable", serviceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable service: %s: %w", string(output), err)
	}
	return nil
}

// StartService starts a systemd service.
func StartService(serviceName string) error {
	cmd := exec.Command("systemctl", "start", serviceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start service: %s: %w", string(output), err)
	}
	return nil
}

// StopService stops a systemd service.
func StopService(serviceName string) error {
	cmd := exec.Command("systemctl", "stop", serviceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop service: %s: %w", string(output), err)
	}
	return nil
}

// RestartService restarts a systemd service.
func RestartService(serviceName string) error {
	cmd := exec.Command("systemctl", "restart", serviceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart service: %s: %w", string(output), err)
	}
	return nil
}

// IsServiceActive checks if a service is active.
func IsServiceActive(serviceName string) bool {
	cmd := exec.Command("systemctl", "is-active", serviceName)
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output)) == "active"
}

// IsServiceEnabled checks if a service is enabled.
func IsServiceEnabled(serviceName string) bool {
	cmd := exec.Command("systemctl", "is-enabled", serviceName)
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output)) == "enabled"
}

// IsServiceInstalled checks if a service unit file exists.
func IsServiceInstalled(serviceName string) bool {
	_, err := os.Stat(GetServicePath(serviceName))
	return err == nil
}

// GetServiceStatus returns the systemctl status output for a service.
func GetServiceStatus(serviceName string) (string, error) {
	cmd := exec.Command("systemctl", "status", serviceName, "--no-pager", "-l")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// GetServiceLogs returns recent logs for a service.
func GetServiceLogs(serviceName string, lines int) (string, error) {
	cmd := exec.Command("journalctl", "-u", serviceName, "-n", fmt.Sprintf("%d", lines), "--no-pager")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	return string(output), nil
}

// DisableService disables a systemd service.
func DisableService(serviceName string) error {
	cmd := exec.Command("systemctl", "disable", serviceName)
	cmd.Run()
	return nil
}

// RemoveService removes a systemd service unit file.
func RemoveService(serviceName string) error {
	os.Remove(GetServicePath(serviceName))
	exec.Command("systemctl", "daemon-reload").Run()
	return nil
}

// SetServicePermissions sets permissions for service files.
func SetServicePermissions(user, group string, privateKeyFile, publicKeyFile, configDir string) error {
	// Set ownership of key files
	if privateKeyFile != "" {
		exec.Command("chown", user+":"+group, privateKeyFile).Run()
		exec.Command("chmod", "600", privateKeyFile).Run()
	}
	if publicKeyFile != "" {
		exec.Command("chown", user+":"+group, publicKeyFile).Run()
		exec.Command("chmod", "644", publicKeyFile).Run()
	}

	// Set ownership of config directory
	exec.Command("chown", "-R", user+":"+group, configDir).Run()

	return nil
}

// DaemonReload reloads systemd daemon.
func DaemonReload() error {
	return exec.Command("systemctl", "daemon-reload").Run()
}
