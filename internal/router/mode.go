package router

import (
	"fmt"
	"log"
	"time"

	"github.com/net2share/dnstm/internal/network"
)

// ModeSnapshot captures the state before a mode switch for rollback.
type ModeSnapshot struct {
	Mode            Mode
	ActiveInstance  string
	DefaultRoute    string
	RunningServices []string
	ConfigYAML      []byte
}

// SwitchMode switches the operating mode of dnstm.
// This handles all necessary state transitions including:
// - Stopping current services
// - Regenerating service files with correct bindings
// - Updating firewall rules
// - Starting services for the new mode
func (r *Router) SwitchMode(newMode Mode) error {
	currentMode := r.config.Mode

	if currentMode == newMode {
		return nil // Already in requested mode
	}

	switch newMode {
	case ModeSingle:
		return r.switchToSingleMode()
	case ModeMulti:
		return r.switchToMultiMode()
	default:
		return fmt.Errorf("unknown mode: %s", newMode)
	}
}

// captureSnapshot captures current state for potential rollback.
func (r *Router) captureSnapshot() (*ModeSnapshot, error) {
	snapshot := &ModeSnapshot{
		Mode:            r.config.Mode,
		ActiveInstance:  r.config.Single.Active,
		DefaultRoute:    r.config.Routing.Default,
		RunningServices: make([]string, 0),
	}

	// Track running services
	for name, instance := range r.instances {
		if instance.IsActive() {
			snapshot.RunningServices = append(snapshot.RunningServices, name)
		}
	}
	if r.dnsrouter.IsActive() {
		snapshot.RunningServices = append(snapshot.RunningServices, "dnsrouter")
	}

	return snapshot, nil
}

// rollback attempts to restore previous state after a failed mode switch.
func (r *Router) rollback(snapshot *ModeSnapshot, reason string) error {
	log.Printf("[warning] rolling back mode switch: %s", reason)

	// Restore config values
	r.config.Mode = snapshot.Mode
	r.config.Single.Active = snapshot.ActiveInstance
	r.config.Routing.Default = snapshot.DefaultRoute

	// Try to regenerate services with original mode
	serviceMode := ServiceModeMulti
	if snapshot.Mode == ModeSingle {
		serviceMode = ServiceModeSingle
	}

	// Regenerate the active instance if in single mode
	if snapshot.Mode == ModeSingle && snapshot.ActiveInstance != "" {
		if instance, ok := r.instances[snapshot.ActiveInstance]; ok {
			if err := instance.RegenerateService(serviceMode); err != nil {
				log.Printf("[warning] rollback: failed to regenerate %s: %v", snapshot.ActiveInstance, err)
			}
		}
	} else {
		// Regenerate all instances for multi mode
		for name, instance := range r.instances {
			if err := instance.RegenerateService(serviceMode); err != nil {
				log.Printf("[warning] rollback: failed to regenerate %s: %v", name, err)
			}
		}
	}

	// Try to restart previously running services
	for _, name := range snapshot.RunningServices {
		if name == "dnsrouter" {
			if err := r.dnsrouter.Start(); err != nil {
				log.Printf("[warning] rollback: failed to start dnsrouter: %v", err)
			}
		} else if instance, ok := r.instances[name]; ok {
			if err := instance.Start(); err != nil {
				log.Printf("[warning] rollback: failed to start %s: %v", name, err)
			}
		}
	}

	// Save config
	if err := r.config.Save(); err != nil {
		log.Printf("[warning] rollback: failed to save config: %v", err)
	}

	return fmt.Errorf("mode switch failed: %s (rollback attempted)", reason)
}

// switchToSingleMode transitions from multi to single mode.
// In single mode, the transport binds directly to EXTERNAL_IP:53.
func (r *Router) switchToSingleMode() error {
	snapshot, _ := r.captureSnapshot()

	// 1. Stop dnsrouter if running
	if r.dnsrouter.IsActive() {
		if err := r.dnsrouter.Stop(); err != nil {
			return fmt.Errorf("failed to stop DNS router: %w", err)
		}
	}

	// 2. Stop all instances
	for name, instance := range r.instances {
		if instance.IsActive() {
			if err := instance.Stop(); err != nil {
				return fmt.Errorf("failed to stop instance %s: %w", name, err)
			}
		}
	}

	// 3. Determine active instance
	active := r.config.Single.Active
	if active == "" && len(r.config.Transports) > 0 {
		// Pick first available instance
		for name := range r.config.Transports {
			active = name
			break
		}
		r.config.Single.Active = active
	}

	// 4. Wait for port 53 to become available
	if !network.WaitForPortAvailable(53, 10*time.Second) {
		// Try to kill the process
		if err := network.KillProcessOnPort(53); err != nil {
			if !network.WaitForPortAvailable(53, 5*time.Second) {
				return r.rollback(snapshot, "port 53 unavailable")
			}
		}
	}

	// 5. Remove NAT rules (no longer needed - transport binds directly)
	network.ClearNATOnly()
	network.AllowPort53()

	// 6. Regenerate active instance for single mode (EXTERNAL_IP:53)
	if active != "" {
		if instance, ok := r.instances[active]; ok {
			if err := instance.RegenerateService(ServiceModeSingle); err != nil {
				return r.rollback(snapshot, fmt.Sprintf("failed to regenerate %s: %v", active, err))
			}
		}
	}

	// 7. Update config mode
	r.config.Mode = ModeSingle

	// 8. Save config
	if err := r.config.Save(); err != nil {
		return r.rollback(snapshot, fmt.Sprintf("failed to save config: %v", err))
	}

	// 9. Start active instance if any
	if active != "" {
		if instance, ok := r.instances[active]; ok {
			if err := instance.Start(); err != nil {
				return r.rollback(snapshot, fmt.Sprintf("failed to start %s: %v", active, err))
			}
		}
	}

	return nil
}

// switchToMultiMode transitions from single to multi mode.
// In multi mode, transports bind to 127.0.0.1:PORT and DNS router handles routing.
func (r *Router) switchToMultiMode() error {
	snapshot, _ := r.captureSnapshot()

	// 1. Stop active instance if running
	if r.config.Single.Active != "" {
		if instance, ok := r.instances[r.config.Single.Active]; ok {
			if instance.IsActive() {
				if err := instance.Stop(); err != nil {
					return fmt.Errorf("failed to stop instance %s: %w", r.config.Single.Active, err)
				}
			}
		}
	}

	// 2. Wait for port 53 to become available
	if !network.WaitForPortAvailable(53, 10*time.Second) {
		if err := network.KillProcessOnPort(53); err != nil {
			if !network.WaitForPortAvailable(53, 5*time.Second) {
				return r.rollback(snapshot, "port 53 unavailable")
			}
		}
	}

	// 3. Remove NAT firewall rules but keep port 53 open for dnsrouter
	network.ClearNATOnly()
	network.AllowPort53()

	// 4. Regenerate ALL instances for multi mode (127.0.0.1:PORT)
	for name, instance := range r.instances {
		if err := instance.RegenerateService(ServiceModeMulti); err != nil {
			log.Printf("[warning] failed to regenerate %s: %v", name, err)
			// Continue - best effort for instances
		}
	}

	// 5. Update config mode
	r.config.Mode = ModeMulti

	// 6. Set default route if not set
	if r.config.Routing.Default == "" && len(r.config.Transports) > 0 {
		// Use previous active or first available
		if r.config.Single.Active != "" {
			r.config.Routing.Default = r.config.Single.Active
		} else {
			for name := range r.config.Transports {
				r.config.Routing.Default = name
				break
			}
		}
	}

	// 7. Regenerate DNS router config
	if err := r.regenerateDNSRouterConfig(); err != nil {
		return r.rollback(snapshot, fmt.Sprintf("failed to generate DNS router config: %v", err))
	}

	// 8. Save config
	if err := r.config.Save(); err != nil {
		return r.rollback(snapshot, fmt.Sprintf("failed to save config: %v", err))
	}

	// 9. Create DNS router service if needed
	if !r.dnsrouter.IsServiceInstalled() {
		if err := r.dnsrouter.CreateService(); err != nil {
			return r.rollback(snapshot, fmt.Sprintf("failed to create DNS router service: %v", err))
		}
	}

	// 10. Start all backend instances FIRST (before dnsrouter)
	for name, instance := range r.instances {
		if err := instance.Start(); err != nil {
			return r.rollback(snapshot, fmt.Sprintf("failed to start instance %s: %v", name, err))
		}
	}

	// 11. Start DNS router AFTER instances are ready
	if err := r.dnsrouter.Start(); err != nil {
		return r.rollback(snapshot, fmt.Sprintf("failed to start DNS router: %v", err))
	}

	return nil
}

// SwitchActiveInstance switches the active instance in single mode.
// This regenerates service files with correct bindings.
func (r *Router) SwitchActiveInstance(name string) error {
	if !r.config.IsSingleMode() {
		return fmt.Errorf("switch is only available in single mode; use 'dnstm mode single' first")
	}

	// Validate instance exists
	_, ok := r.config.Transports[name]
	if !ok {
		return fmt.Errorf("instance '%s' does not exist", name)
	}

	newInstance, ok := r.instances[name]
	if !ok {
		return fmt.Errorf("instance '%s' not found", name)
	}

	currentActive := r.config.Single.Active

	// Nothing to do if already active
	if currentActive == name {
		return nil
	}

	// 1. Stop current active instance and reset to multi-ready
	if currentActive != "" {
		if oldInstance, ok := r.instances[currentActive]; ok {
			if oldInstance.IsActive() {
				if err := oldInstance.Stop(); err != nil {
					return fmt.Errorf("failed to stop current instance %s: %w", currentActive, err)
				}
			}
			// Reset old instance to multi-mode binding
			if err := oldInstance.RegenerateService(ServiceModeMulti); err != nil {
				log.Printf("[warning] failed to reset %s to multi mode: %v", currentActive, err)
			}
		}
	}

	// 2. Wait for port 53 to become available
	if !network.WaitForPortAvailable(53, 10*time.Second) {
		if err := network.KillProcessOnPort(53); err != nil {
			if !network.WaitForPortAvailable(53, 5*time.Second) {
				return fmt.Errorf("port 53 is not available")
			}
		}
	}

	// 3. Regenerate new active for single mode (EXTERNAL_IP:53)
	if err := newInstance.RegenerateService(ServiceModeSingle); err != nil {
		return fmt.Errorf("failed to regenerate %s for single mode: %w", name, err)
	}

	// 4. Update config
	r.config.Single.Active = name
	if err := r.config.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// 5. Start new active instance
	if err := newInstance.Start(); err != nil {
		return fmt.Errorf("failed to start instance %s: %w", name, err)
	}

	return nil
}

// GetModeDisplayName returns a human-readable name for the mode.
func GetModeDisplayName(m Mode) string {
	switch m {
	case ModeSingle:
		return "Single-tunnel"
	case ModeMulti:
		return "Multi-tunnel"
	default:
		return string(m)
	}
}
