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

// RealSystemdManager implements SystemdManager using actual systemd commands.
type RealSystemdManager struct{}

// NewRealSystemdManager creates a new RealSystemdManager.
func NewRealSystemdManager() *RealSystemdManager {
	return &RealSystemdManager{}
}

// CreateService implements SystemdManager.
func (m *RealSystemdManager) CreateService(name string, cfg ServiceConfig) error {
	cfg.Name = name
	return CreateGenericService(&cfg)
}

// RemoveService implements SystemdManager.
func (m *RealSystemdManager) RemoveService(name string) error {
	return RemoveService(name)
}

// StartService implements SystemdManager.
func (m *RealSystemdManager) StartService(name string) error {
	return StartService(name)
}

// StopService implements SystemdManager.
func (m *RealSystemdManager) StopService(name string) error {
	return StopService(name)
}

// RestartService implements SystemdManager.
func (m *RealSystemdManager) RestartService(name string) error {
	return RestartService(name)
}

// EnableService implements SystemdManager.
func (m *RealSystemdManager) EnableService(name string) error {
	return EnableService(name)
}

// DisableService implements SystemdManager.
func (m *RealSystemdManager) DisableService(name string) error {
	return DisableService(name)
}

// IsServiceActive implements SystemdManager.
func (m *RealSystemdManager) IsServiceActive(name string) bool {
	return IsServiceActive(name)
}

// IsServiceEnabled implements SystemdManager.
func (m *RealSystemdManager) IsServiceEnabled(name string) bool {
	return IsServiceEnabled(name)
}

// IsServiceInstalled implements SystemdManager.
func (m *RealSystemdManager) IsServiceInstalled(name string) bool {
	return IsServiceInstalled(name)
}

// GetServiceStatus implements SystemdManager.
func (m *RealSystemdManager) GetServiceStatus(name string) (string, error) {
	return GetServiceStatus(name)
}

// GetServiceLogs implements SystemdManager.
func (m *RealSystemdManager) GetServiceLogs(name string, lines int) (string, error) {
	return GetServiceLogs(name, lines)
}

// DaemonReload implements SystemdManager.
func (m *RealSystemdManager) DaemonReload() error {
	return DaemonReload()
}

// Ensure RealSystemdManager implements SystemdManager.
var _ SystemdManager = (*RealSystemdManager)(nil)

// GetServicePath returns the systemd service file path for a service name.
func GetServicePath(serviceName string) string {
	return fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
}

// runSystemctl executes a systemctl command and returns a formatted error on failure.
func runSystemctl(action, serviceName string) error {
	cmd := exec.Command("systemctl", action, serviceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to %s service: %s: %w", action, strings.TrimSpace(string(output)), err)
	}
	return nil
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

	return DaemonReload()
}

// EnableService enables a systemd service.
func EnableService(serviceName string) error {
	return runSystemctl("enable", serviceName)
}

// DisableService disables a systemd service.
func DisableService(serviceName string) error {
	return runSystemctl("disable", serviceName)
}

// StartService starts a systemd service.
func StartService(serviceName string) error {
	return runSystemctl("start", serviceName)
}

// StopService stops a systemd service.
func StopService(serviceName string) error {
	return runSystemctl("stop", serviceName)
}

// RestartService restarts a systemd service.
func RestartService(serviceName string) error {
	return runSystemctl("restart", serviceName)
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

// RemoveService removes a systemd service unit file and reloads daemon.
func RemoveService(serviceName string) error {
	servicePath := GetServicePath(serviceName)
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}
	return DaemonReload()
}

// SetServicePermissions sets permissions for service files.
func SetServicePermissions(user, group string, privateKeyFile, publicKeyFile, configDir string) error {
	ownership := user + ":" + group

	if privateKeyFile != "" {
		if err := exec.Command("chown", ownership, privateKeyFile).Run(); err != nil {
			return fmt.Errorf("failed to chown private key: %w", err)
		}
		if err := exec.Command("chmod", "600", privateKeyFile).Run(); err != nil {
			return fmt.Errorf("failed to chmod private key: %w", err)
		}
	}
	if publicKeyFile != "" {
		if err := exec.Command("chown", ownership, publicKeyFile).Run(); err != nil {
			return fmt.Errorf("failed to chown public key: %w", err)
		}
		if err := exec.Command("chmod", "644", publicKeyFile).Run(); err != nil {
			return fmt.Errorf("failed to chmod public key: %w", err)
		}
	}

	if err := exec.Command("chown", "-R", ownership, configDir).Run(); err != nil {
		return fmt.Errorf("failed to chown config directory: %w", err)
	}

	return nil
}

// DaemonReload reloads systemd daemon.
func DaemonReload() error {
	return exec.Command("systemctl", "daemon-reload").Run()
}
