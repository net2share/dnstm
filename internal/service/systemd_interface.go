package service

// ServiceStatus represents the current status of a systemd service.
type ServiceStatus string

const (
	StatusRunning  ServiceStatus = "running"
	StatusStopped  ServiceStatus = "stopped"
	StatusFailed   ServiceStatus = "failed"
	StatusNotFound ServiceStatus = "not-found"
)

// SystemdManager defines the interface for managing systemd services.
// This allows for mocking in tests and decoupling from the actual systemd implementation.
type SystemdManager interface {
	// CreateService creates a new systemd service with the given configuration.
	CreateService(name string, cfg ServiceConfig) error

	// RemoveService stops, disables, and removes a systemd service.
	RemoveService(name string) error

	// StartService starts a systemd service.
	StartService(name string) error

	// StopService stops a systemd service.
	StopService(name string) error

	// RestartService restarts a systemd service.
	RestartService(name string) error

	// EnableService enables a systemd service to start on boot.
	EnableService(name string) error

	// DisableService disables a systemd service from starting on boot.
	DisableService(name string) error

	// IsServiceActive returns true if the service is currently running.
	IsServiceActive(name string) bool

	// IsServiceEnabled returns true if the service is enabled to start on boot.
	IsServiceEnabled(name string) bool

	// IsServiceInstalled returns true if the service unit file exists.
	IsServiceInstalled(name string) bool

	// GetServiceStatus returns the systemctl status output for diagnostics.
	GetServiceStatus(name string) (string, error)

	// GetServiceLogs returns recent logs from journalctl.
	GetServiceLogs(name string, lines int) (string, error)

	// DaemonReload reloads the systemd daemon to pick up new/changed unit files.
	DaemonReload() error
}

// defaultManager is the package-level manager instance.
var defaultManager SystemdManager

// DefaultManager returns the default SystemdManager implementation.
// Uses real systemd in production, can be overridden for testing.
func DefaultManager() SystemdManager {
	if defaultManager == nil {
		defaultManager = NewRealSystemdManager()
	}
	return defaultManager
}

// SetDefaultManager overrides the default manager (for testing).
func SetDefaultManager(m SystemdManager) {
	defaultManager = m
}

// ResetDefaultManager resets to the real systemd manager.
func ResetDefaultManager() {
	defaultManager = nil
}
