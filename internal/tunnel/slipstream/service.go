package slipstream

import (
	"fmt"

	"github.com/net2share/dnstm/internal/service"
)

// CreateService creates the systemd service for slipstream-server.
func CreateService(cfg *Config) error {
	// slipstream-server --domain t.example.com --dns-listen-port 5301 --target-address 127.0.0.1:22 --cert cert.pem --key key.pem
	execStart := fmt.Sprintf("/usr/local/bin/%s --domain %s --dns-listen-port %s --target-address %s --cert %s --key %s",
		BinaryName, cfg.Domain, cfg.DNSListenPort, cfg.TargetAddress, cfg.CertFile, cfg.KeyFile)

	svcCfg := &service.ServiceConfig{
		Name:        ServiceName,
		Description: "Slipstream DNS Tunnel Server",
		User:        ServiceUser,
		Group:       ServiceUser,
		ExecStart:   execStart,
		ConfigDir:   ConfigDir,
	}

	return service.CreateGenericService(svcCfg)
}

// SetPermissions sets the correct permissions on certificate files.
func SetPermissions(cfg *Config) error {
	return service.SetServicePermissions(
		ServiceUser,
		ServiceUser,
		cfg.KeyFile, // Private key file
		cfg.CertFile, // Certificate file (public)
		ConfigDir,
	)
}

// Enable enables the slipstream-server service.
func Enable() error {
	return service.EnableService(ServiceName)
}

// Start starts the slipstream-server service.
func Start() error {
	return service.StartService(ServiceName)
}

// Stop stops the slipstream-server service.
func Stop() error {
	return service.StopService(ServiceName)
}

// Restart restarts the slipstream-server service.
func Restart() error {
	return service.RestartService(ServiceName)
}

// IsActive checks if the slipstream-server service is active.
func IsActive() bool {
	return service.IsServiceActive(ServiceName)
}

// IsEnabled checks if the slipstream-server service is enabled.
func IsEnabled() bool {
	return service.IsServiceEnabled(ServiceName)
}

// IsServiceInstalled checks if the slipstream-server service unit exists.
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

// Disable disables the slipstream-server service.
func Disable() error {
	return service.DisableService(ServiceName)
}

// Remove removes the slipstream-server service unit file.
func Remove() error {
	return service.RemoveService(ServiceName)
}
