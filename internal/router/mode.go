package router

import (
	"fmt"
	"log"
	"time"

	"github.com/net2share/dnstm/internal/network"
)

// ModeSnapshot captures the state before a mode switch for rollback.
type ModeSnapshot struct {
	Mode            string
	ActiveTunnel    string
	DefaultRoute    string
	RunningServices []string
}

// SwitchMode switches the operating mode of dnstm.
func (r *Router) SwitchMode(newMode string) error {
	currentMode := r.config.Route.Mode

	if currentMode == newMode {
		return nil // Already in requested mode
	}

	switch newMode {
	case "single":
		return r.switchToSingleMode()
	case "multi":
		return r.switchToMultiMode()
	default:
		return fmt.Errorf("unknown mode: %s", newMode)
	}
}

// captureSnapshot captures current state for potential rollback.
func (r *Router) captureSnapshot() (*ModeSnapshot, error) {
	snapshot := &ModeSnapshot{
		Mode:            r.config.Route.Mode,
		ActiveTunnel:    r.config.Route.Active,
		DefaultRoute:    r.config.Route.Default,
		RunningServices: make([]string, 0),
	}

	// Track running services
	for tag, tunnel := range r.tunnels {
		if tunnel.IsActive() {
			snapshot.RunningServices = append(snapshot.RunningServices, tag)
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
	r.config.Route.Mode = snapshot.Mode
	r.config.Route.Active = snapshot.ActiveTunnel
	r.config.Route.Default = snapshot.DefaultRoute

	// Try to restart previously running services
	for _, tag := range snapshot.RunningServices {
		if tag == "dnsrouter" {
			if err := r.dnsrouter.Start(); err != nil {
				log.Printf("[warning] rollback: failed to start dnsrouter: %v", err)
			}
		} else if tunnel, ok := r.tunnels[tag]; ok {
			if err := tunnel.Start(); err != nil {
				log.Printf("[warning] rollback: failed to start %s: %v", tag, err)
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
func (r *Router) switchToSingleMode() error {
	snapshot, _ := r.captureSnapshot()

	// 1. Stop dnsrouter if running
	if r.dnsrouter.IsActive() {
		if err := r.dnsrouter.Stop(); err != nil {
			return fmt.Errorf("failed to stop DNS router: %w", err)
		}
	}

	// 2. Stop all tunnels
	for tag, tunnel := range r.tunnels {
		if tunnel.IsActive() {
			if err := tunnel.Stop(); err != nil {
				return fmt.Errorf("failed to stop tunnel %s: %w", tag, err)
			}
		}
	}

	// 3. Determine active tunnel
	active := r.config.Route.Active
	if active == "" && len(r.config.Tunnels) > 0 {
		// Pick first enabled tunnel
		for _, t := range r.config.Tunnels {
			if t.IsEnabled() {
				active = t.Tag
				break
			}
		}
		r.config.Route.Active = active
	}

	// 4. Wait for port 53 to become available
	if !network.WaitForPortAvailable(53, 10*time.Second) {
		if err := network.KillProcessOnPort(53); err != nil {
			if !network.WaitForPortAvailable(53, 5*time.Second) {
				return r.rollback(snapshot, "port 53 unavailable")
			}
		}
	}

	// 5. Remove NAT rules (no longer needed - transport binds directly)
	network.ClearNATOnly()
	network.AllowPort53()

	// 6. Update config mode
	r.config.Route.Mode = "single"

	// 7. Save config
	if err := r.config.Save(); err != nil {
		return r.rollback(snapshot, fmt.Sprintf("failed to save config: %v", err))
	}

	// 8. Start active tunnel if any
	if active != "" {
		if tunnel, ok := r.tunnels[active]; ok {
			if err := tunnel.Start(); err != nil {
				return r.rollback(snapshot, fmt.Sprintf("failed to start %s: %v", active, err))
			}
		}
	}

	return nil
}

// switchToMultiMode transitions from single to multi mode.
func (r *Router) switchToMultiMode() error {
	// Validate: each tunnel must have a unique domain in multi-mode
	domains := make(map[string]string)
	for _, t := range r.config.Tunnels {
		if existing, ok := domains[t.Domain]; ok {
			return fmt.Errorf("cannot switch to multi-mode: tunnels '%s' and '%s' share the same domain '%s'", existing, t.Tag, t.Domain)
		}
		domains[t.Domain] = t.Tag
	}

	snapshot, _ := r.captureSnapshot()

	// 1. Stop active tunnel if running
	if r.config.Route.Active != "" {
		if tunnel, ok := r.tunnels[r.config.Route.Active]; ok {
			if tunnel.IsActive() {
				if err := tunnel.Stop(); err != nil {
					return fmt.Errorf("failed to stop tunnel %s: %w", r.config.Route.Active, err)
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

	// 4. Update config mode
	r.config.Route.Mode = "multi"

	// 5. Set default route if not set
	if r.config.Route.Default == "" && len(r.config.Tunnels) > 0 {
		// Use previous active or first enabled
		if r.config.Route.Active != "" {
			r.config.Route.Default = r.config.Route.Active
		} else {
			for _, t := range r.config.Tunnels {
				if t.IsEnabled() {
					r.config.Route.Default = t.Tag
					break
				}
			}
		}
	}

	// 6. Regenerate DNS router config
	if err := r.regenerateDNSRouterConfig(); err != nil {
		return r.rollback(snapshot, fmt.Sprintf("failed to generate DNS router config: %v", err))
	}

	// 7. Save config
	if err := r.config.Save(); err != nil {
		return r.rollback(snapshot, fmt.Sprintf("failed to save config: %v", err))
	}

	// 8. Create DNS router service if needed
	if !r.dnsrouter.IsServiceInstalled() {
		if err := r.dnsrouter.CreateService(); err != nil {
			return r.rollback(snapshot, fmt.Sprintf("failed to create DNS router service: %v", err))
		}
	}

	// 9. Start all enabled tunnels FIRST (before dnsrouter)
	for tag, tunnel := range r.tunnels {
		if tunnel.Config.IsEnabled() {
			if err := tunnel.Start(); err != nil {
				return r.rollback(snapshot, fmt.Sprintf("failed to start tunnel %s: %v", tag, err))
			}
		}
	}

	// 10. Start DNS router AFTER tunnels are ready
	if err := r.dnsrouter.Start(); err != nil {
		return r.rollback(snapshot, fmt.Sprintf("failed to start DNS router: %v", err))
	}

	return nil
}

// SwitchActiveTunnel switches the active tunnel in single mode.
func (r *Router) SwitchActiveTunnel(tag string) error {
	if !r.config.IsSingleMode() {
		return fmt.Errorf("switch is only available in single mode; use 'dnstm router mode single' first")
	}

	// Validate tunnel exists
	if r.config.GetTunnelByTag(tag) == nil {
		return fmt.Errorf("tunnel '%s' does not exist", tag)
	}

	newTunnel, ok := r.tunnels[tag]
	if !ok {
		return fmt.Errorf("tunnel '%s' not found", tag)
	}

	currentActive := r.config.Route.Active

	// Nothing to do if already active
	if currentActive == tag {
		return nil
	}

	// 1. Stop current active tunnel
	if currentActive != "" {
		if oldTunnel, ok := r.tunnels[currentActive]; ok {
			if oldTunnel.IsActive() {
				if err := oldTunnel.Stop(); err != nil {
					return fmt.Errorf("failed to stop current tunnel %s: %w", currentActive, err)
				}
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

	// 3. Update config
	r.config.Route.Active = tag
	if err := r.config.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// 4. Start new active tunnel
	if err := newTunnel.Start(); err != nil {
		return fmt.Errorf("failed to start tunnel %s: %w", tag, err)
	}

	return nil
}
