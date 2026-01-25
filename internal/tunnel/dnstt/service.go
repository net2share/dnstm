package dnstt

import (
	"fmt"

	"github.com/net2share/dnstm/internal/service"
)

// CreateService creates the systemd service for dnstt-server.
func CreateService(cfg *Config) error {
	targetAddr := "127.0.0.1:" + cfg.TargetPort

	execStart := fmt.Sprintf("/usr/local/bin/%s -udp :%s -privkey-file %s -mtu %s %s %s",
		BinaryName, Port, cfg.PrivateKeyFile, cfg.MTU, cfg.NSSubdomain, targetAddr)

	svcCfg := &service.ServiceConfig{
		Name:        ServiceName,
		Description: "dnstt DNS Tunnel Server",
		User:        ServiceUser,
		Group:       ServiceUser,
		ExecStart:   execStart,
		ConfigDir:   ConfigDir,
	}

	return service.CreateGenericService(svcCfg)
}

// SetPermissions sets the correct permissions on key files.
func SetPermissions(cfg *Config) error {
	return service.SetServicePermissions(
		ServiceUser,
		ServiceUser,
		cfg.PrivateKeyFile,
		cfg.PublicKeyFile,
		ConfigDir,
	)
}

// Enable enables the dnstt-server service.
func Enable() error {
	return service.EnableService(ServiceName)
}

// Start starts the dnstt-server service.
func Start() error {
	return service.StartService(ServiceName)
}

// Stop stops the dnstt-server service.
func Stop() error {
	return service.StopService(ServiceName)
}

// Restart restarts the dnstt-server service.
func Restart() error {
	return service.RestartService(ServiceName)
}

// IsActive checks if the dnstt-server service is active.
func IsActive() bool {
	return service.IsServiceActive(ServiceName)
}

// IsEnabled checks if the dnstt-server service is enabled.
func IsEnabled() bool {
	return service.IsServiceEnabled(ServiceName)
}

// IsServiceInstalled checks if the dnstt-server service unit exists.
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

// Disable disables the dnstt-server service.
func Disable() error {
	return service.DisableService(ServiceName)
}

// Remove removes the dnstt-server service unit file.
func Remove() error {
	return service.RemoveService(ServiceName)
}
