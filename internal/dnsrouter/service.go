package dnsrouter

import (
	"fmt"
	"path/filepath"

	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/system"
)

const (
	ServiceName = "dnstm-dnsrouter"
	BinaryName  = "dnstm-dnsrouter"
	ConfigDir   = "/etc/dnstm"
	ConfigFile  = "dnsrouter.yaml"
)

// Service manages the DNS router as a systemd service.
type Service struct {
	binaryPath string
	configPath string
}

// NewService creates a new DNS router service manager.
func NewService() *Service {
	return &Service{
		binaryPath: getBinaryPath(),
		configPath: filepath.Join(ConfigDir, ConfigFile),
	}
}

func getBinaryPath() string {
	// Always use the installed path for systemd services
	// This prevents issues when running from development locations
	return "/usr/local/bin/dnstm"
}

// CreateService creates the systemd service for the DNS router.
func (s *Service) CreateService() error {
	cfg := &service.ServiceConfig{
		Name:             ServiceName,
		Description:      "DNSTM DNS Router",
		User:             system.DnstmUser,
		Group:            system.DnstmUser,
		ExecStart:        fmt.Sprintf("%s dnsrouter serve", s.binaryPath),
		ReadOnlyPaths:    []string{"/etc/dnstm"},
		BindToPrivileged: true,
	}

	return service.CreateGenericService(cfg)
}

// Start starts the DNS router service.
func (s *Service) Start() error {
	return service.StartService(ServiceName)
}

// Stop stops the DNS router service.
func (s *Service) Stop() error {
	return service.StopService(ServiceName)
}

// Restart restarts the DNS router service.
func (s *Service) Restart() error {
	return service.RestartService(ServiceName)
}

// Enable enables the DNS router service to start on boot.
func (s *Service) Enable() error {
	return service.EnableService(ServiceName)
}

// Disable disables the DNS router service from starting on boot.
func (s *Service) Disable() error {
	return service.DisableService(ServiceName)
}

// GetStatus returns the systemctl status output.
func (s *Service) GetStatus() (string, error) {
	return service.GetServiceStatus(ServiceName)
}

// GetLogs returns recent logs from the service.
func (s *Service) GetLogs(lines int) (string, error) {
	return service.GetServiceLogs(ServiceName, lines)
}

// IsActive checks if the DNS router service is active.
func (s *Service) IsActive() bool {
	return service.IsServiceActive(ServiceName)
}

// IsEnabled checks if the DNS router service is enabled.
func (s *Service) IsEnabled() bool {
	return service.IsServiceEnabled(ServiceName)
}

// IsServiceInstalled checks if the DNS router service unit exists.
func (s *Service) IsServiceInstalled() bool {
	return service.IsServiceInstalled(ServiceName)
}

// Remove removes the DNS router service.
func (s *Service) Remove() error {
	if s.IsActive() {
		s.Stop()
	}
	if s.IsEnabled() {
		s.Disable()
	}
	return service.RemoveService(ServiceName)
}

// StatusString returns a human-readable status string.
func (s *Service) StatusString() string {
	if s.IsActive() {
		return "Running"
	}
	return "Stopped"
}

// EnsureUser ensures the dnstm user exists.
func EnsureUser() error {
	return system.CreateDnstmUser()
}
