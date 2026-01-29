package router

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/dnstm/internal/types"
)

// Instance represents a running transport instance.
type Instance struct {
	Name        string
	Type        types.TransportType
	Domain      string
	Port        int
	ServiceName string
	Config      *types.TransportConfig
}

// NewInstance creates a new instance from configuration.
func NewInstance(name string, cfg *types.TransportConfig) *Instance {
	return &Instance{
		Name:        name,
		Type:        cfg.Type,
		Domain:      cfg.Domain,
		Port:        cfg.Port,
		ServiceName: GetServiceName(name),
		Config:      cfg,
	}
}

// Start starts the instance service.
func (i *Instance) Start() error {
	return service.StartService(i.ServiceName)
}

// Stop stops the instance service.
func (i *Instance) Stop() error {
	return service.StopService(i.ServiceName)
}

// Restart restarts the instance service.
func (i *Instance) Restart() error {
	return service.RestartService(i.ServiceName)
}

// Enable enables the instance service to start on boot.
func (i *Instance) Enable() error {
	return service.EnableService(i.ServiceName)
}

// Disable disables the instance service from starting on boot.
func (i *Instance) Disable() error {
	return service.DisableService(i.ServiceName)
}

// GetLogs returns recent logs from the instance.
func (i *Instance) GetLogs(lines int) (string, error) {
	return service.GetServiceLogs(i.ServiceName, lines)
}

// GetStatus returns the systemctl status output.
func (i *Instance) GetStatus() (string, error) {
	return service.GetServiceStatus(i.ServiceName)
}

// IsActive checks if the instance is currently running.
func (i *Instance) IsActive() bool {
	return service.IsServiceActive(i.ServiceName)
}

// IsEnabled checks if the instance is enabled to start on boot.
func (i *Instance) IsEnabled() bool {
	return service.IsServiceEnabled(i.ServiceName)
}

// IsInstalled checks if the instance service is installed.
func (i *Instance) IsInstalled() bool {
	return service.IsServiceInstalled(i.ServiceName)
}

// CreateService creates the systemd service for this instance.
// Uses default multi-mode binding (127.0.0.1:cfg.Port).
func (i *Instance) CreateService() error {
	return i.CreateServiceWithMode(ServiceModeMulti)
}

// CreateServiceWithMode creates the systemd service with the specified binding mode.
func (i *Instance) CreateServiceWithMode(mode ServiceMode) error {
	sg := NewServiceGenerator()
	opts, err := sg.GetBindOptions(i.Config, mode)
	if err != nil {
		return fmt.Errorf("failed to get bind options: %w", err)
	}

	builder := transport.NewBuilder()
	result, err := builder.Build(i.Name, i.Config, opts)
	if err != nil {
		return fmt.Errorf("failed to build transport: %w", err)
	}

	return i.createServiceUnit(result)
}

// RegenerateService stops the service, regenerates it with new binding mode, and optionally restarts.
func (i *Instance) RegenerateService(mode ServiceMode) error {
	wasActive := i.IsActive()

	// Stop if running
	if wasActive {
		if err := i.Stop(); err != nil {
			return fmt.Errorf("failed to stop service: %w", err)
		}
	}

	// Remove existing service
	if i.IsInstalled() {
		if err := service.RemoveService(i.ServiceName); err != nil {
			return fmt.Errorf("failed to remove service: %w", err)
		}
	}

	// Create service with new mode
	if err := i.CreateServiceWithMode(mode); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	return nil
}

// createServiceUnit creates the systemd unit file.
func (i *Instance) createServiceUnit(result *transport.BuildResult) error {
	cfg := &service.ServiceConfig{
		Name:             i.ServiceName,
		Description:      fmt.Sprintf("DNSTM Router Instance - %s", i.Name),
		User:             system.DnstmUser,
		Group:            system.DnstmUser,
		ExecStart:        result.ExecStart,
		BindToPrivileged: true,
	}

	// Determine ReadWritePaths or ReadOnlyPaths based on transport type
	if i.Type == types.TypeSlipstreamShadowsocks {
		// Shadowsocks plugin may need to write
		cfg.ReadWritePaths = []string{result.ConfigDir, CertsDir}
	} else {
		cfg.ReadOnlyPaths = []string{result.ConfigDir, CertsDir}
	}

	return service.CreateGenericService(cfg)
}

// RemoveService removes the systemd service for this instance.
func (i *Instance) RemoveService() error {
	if i.IsActive() {
		i.Stop()
	}
	if i.IsEnabled() {
		i.Disable()
	}
	return service.RemoveService(i.ServiceName)
}

// SetPermissions sets the correct permissions for the instance files.
func (i *Instance) SetPermissions() error {
	configDir := filepath.Join(ConfigDir, "instances", i.Name)

	// Set ownership of instance config directory
	if err := exec.Command("chown", "-R", system.DnstmUser+":"+system.DnstmUser, configDir).Run(); err != nil {
		log.Printf("[warning] failed to set ownership on %s: %v", configDir, err)
	}
	if err := exec.Command("chmod", "750", configDir).Run(); err != nil {
		log.Printf("[warning] failed to set permissions on %s: %v", configDir, err)
	}

	return nil
}

// GetConfigDir returns the instance-specific config directory.
func (i *Instance) GetConfigDir() string {
	return filepath.Join(ConfigDir, "instances", i.Name)
}

// RemoveConfigDir removes the instance-specific config directory.
func (i *Instance) RemoveConfigDir() error {
	configDir := i.GetConfigDir()
	return os.RemoveAll(configDir)
}

// StatusString returns a human-readable status string.
func (i *Instance) StatusString() string {
	if i.IsActive() {
		return "Running"
	}
	if i.IsInstalled() {
		return "Stopped"
	}
	return "Not installed"
}

// GetFormattedInfo returns formatted information about the instance.
func (i *Instance) GetFormattedInfo() string {
	return fmt.Sprintf(`Name:     %s
Type:     %s
Domain:   %s
Port:     %d
Service:  %s
Status:   %s
`,
		i.Name,
		types.GetTransportTypeDisplayName(i.Type),
		i.Domain,
		i.Port,
		i.ServiceName,
		i.StatusString(),
	)
}
