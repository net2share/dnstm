package router

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/system"
)

// Tunnel represents a running DNS tunnel.
type Tunnel struct {
	Tag         string
	Transport   config.TransportType
	Backend     string
	Domain      string
	Port        int
	ServiceName string
	Config      *config.TunnelConfig
}

// NewTunnel creates a new tunnel from configuration.
func NewTunnel(cfg *config.TunnelConfig) *Tunnel {
	return &Tunnel{
		Tag:         cfg.Tag,
		Transport:   cfg.Transport,
		Backend:     cfg.Backend,
		Domain:      cfg.Domain,
		Port:        cfg.Port,
		ServiceName: GetServiceName(cfg.Tag),
		Config:      cfg,
	}
}

// Start starts the tunnel service.
func (t *Tunnel) Start() error {
	return service.StartService(t.ServiceName)
}

// Stop stops the tunnel service.
func (t *Tunnel) Stop() error {
	return service.StopService(t.ServiceName)
}

// Restart restarts the tunnel service.
func (t *Tunnel) Restart() error {
	return service.RestartService(t.ServiceName)
}

// Enable enables the tunnel service to start on boot.
func (t *Tunnel) Enable() error {
	return service.EnableService(t.ServiceName)
}

// Disable disables the tunnel service from starting on boot.
func (t *Tunnel) Disable() error {
	return service.DisableService(t.ServiceName)
}

// GetLogs returns recent logs from the tunnel.
func (t *Tunnel) GetLogs(lines int) (string, error) {
	return service.GetServiceLogs(t.ServiceName, lines)
}

// GetStatus returns the systemctl status output.
func (t *Tunnel) GetStatus() (string, error) {
	return service.GetServiceStatus(t.ServiceName)
}

// IsActive checks if the tunnel is currently running.
func (t *Tunnel) IsActive() bool {
	return service.IsServiceActive(t.ServiceName)
}

// IsServiceEnabled checks if the tunnel service is enabled to start on boot.
func (t *Tunnel) IsServiceEnabled() bool {
	return service.IsServiceEnabled(t.ServiceName)
}

// IsInstalled checks if the tunnel service is installed.
func (t *Tunnel) IsInstalled() bool {
	return service.IsServiceInstalled(t.ServiceName)
}

// RemoveService removes the systemd service for this tunnel.
func (t *Tunnel) RemoveService() error {
	if t.IsActive() {
		t.Stop()
	}
	if t.IsServiceEnabled() {
		t.Disable()
	}
	return service.RemoveService(t.ServiceName)
}

// SetPermissions sets the correct permissions for the tunnel files.
func (t *Tunnel) SetPermissions() error {
	configDir := filepath.Join(ConfigDir, "tunnels", t.Tag)

	// Set ownership of tunnel config directory
	if err := exec.Command("chown", "-R", system.DnstmUser+":"+system.DnstmUser, configDir).Run(); err != nil {
		log.Printf("[warning] failed to set ownership on %s: %v", configDir, err)
	}
	if err := exec.Command("chmod", "750", configDir).Run(); err != nil {
		log.Printf("[warning] failed to set permissions on %s: %v", configDir, err)
	}

	return nil
}

// GetConfigDir returns the tunnel-specific config directory.
func (t *Tunnel) GetConfigDir() string {
	return filepath.Join(ConfigDir, "tunnels", t.Tag)
}

// RemoveConfigDir removes the tunnel-specific config directory.
func (t *Tunnel) RemoveConfigDir() error {
	configDir := t.GetConfigDir()
	return os.RemoveAll(configDir)
}

// StatusString returns a human-readable status string.
func (t *Tunnel) StatusString() string {
	if t.IsActive() {
		return "Running"
	}
	if t.IsInstalled() {
		return "Stopped"
	}
	return "Not installed"
}

// GetFormattedInfo returns formatted information about the tunnel.
func (t *Tunnel) GetFormattedInfo() string {
	return fmt.Sprintf(`Tag:       %s
Transport: %s
Backend:   %s
Domain:    %s
Port:      %d
Service:   %s
Status:    %s
`,
		t.Tag,
		config.GetTransportTypeDisplayName(t.Transport),
		t.Backend,
		t.Domain,
		t.Port,
		t.ServiceName,
		t.StatusString(),
	)
}

