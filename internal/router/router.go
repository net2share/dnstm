package router

import (
	"fmt"
	"os"

	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/system"
)

// Router orchestrates multiple tunnels and the DNS router.
type Router struct {
	config    *config.Config
	tunnels   map[string]*Tunnel
	dnsrouter *dnsrouter.Service
	certMgr   *certs.Manager
}

// New creates a new router from configuration.
func New(cfg *config.Config) (*Router, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	r := &Router{
		config:    cfg,
		tunnels:   make(map[string]*Tunnel),
		dnsrouter: dnsrouter.NewService(),
		certMgr:   certs.NewManager(),
	}

	// Create tunnels from config
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		r.tunnels[t.Tag] = NewTunnel(t)
	}

	return r, nil
}

// Start starts the router based on the current mode.
// In single mode: starts the active tunnel (binds directly to EXTERNAL_IP:53).
// In multi mode: starts the DNS router and all enabled tunnels.
func (r *Router) Start() error {
	// Ensure dnstm user exists
	if err := system.CreateDnstmUser(); err != nil {
		return fmt.Errorf("failed to create dnstm user: %w", err)
	}

	if r.config.IsSingleMode() {
		return r.startSingleMode()
	}
	return r.startMultiMode()
}

// startSingleMode starts the active tunnel which binds directly to EXTERNAL_IP:53.
func (r *Router) startSingleMode() error {
	active := r.config.Route.Active
	if active == "" {
		return fmt.Errorf("no active tunnel configured; use 'dnstm tunnel add' first")
	}

	tunnel := r.tunnels[active]
	if tunnel == nil {
		return fmt.Errorf("active tunnel '%s' not found", active)
	}

	// Clear any stale NAT rules (transport binds directly to external IP, no NAT needed)
	network.ClearNATOnly()
	// Ensure firewall allows port 53
	network.AllowPort53()

	// Start the tunnel
	if err := tunnel.Start(); err != nil {
		return fmt.Errorf("failed to start tunnel %s: %w", active, err)
	}

	return nil
}

// startMultiMode starts the DNS router and all enabled tunnels.
func (r *Router) startMultiMode() error {
	// Generate DNS router config
	if err := r.regenerateDNSRouterConfig(); err != nil {
		return fmt.Errorf("failed to generate DNS router config: %w", err)
	}

	// Create DNS router service if needed
	if !r.dnsrouter.IsServiceInstalled() {
		if err := r.dnsrouter.CreateService(); err != nil {
			return fmt.Errorf("failed to create DNS router service: %w", err)
		}
	}

	// Clear any stale NAT rules (DNS router binds directly to external IP)
	network.ClearNATOnly()
	// Ensure firewall allows port 53
	network.AllowPort53()

	// Start all enabled tunnels FIRST (before dnsrouter)
	for tag, tunnel := range r.tunnels {
		if tunnel.Config.IsEnabled() {
			if err := tunnel.Start(); err != nil {
				return fmt.Errorf("failed to start tunnel %s: %w", tag, err)
			}
		}
	}

	// Start DNS router AFTER tunnels are ready
	if err := r.dnsrouter.Start(); err != nil {
		return fmt.Errorf("failed to start DNS router: %w", err)
	}

	return nil
}

// Stop stops the router based on the current mode.
func (r *Router) Stop() error {
	if r.config.IsSingleMode() {
		return r.stopSingleMode()
	}
	return r.stopMultiMode()
}

// stopSingleMode stops the active tunnel.
func (r *Router) stopSingleMode() error {
	var lastErr error

	active := r.config.Route.Active
	if active != "" {
		if tunnel, ok := r.tunnels[active]; ok {
			if err := tunnel.Stop(); err != nil {
				lastErr = fmt.Errorf("failed to stop tunnel %s: %w", active, err)
			}
		}
	}

	return lastErr
}

// stopMultiMode stops all tunnels and the DNS router.
func (r *Router) stopMultiMode() error {
	var lastErr error

	// Stop all tunnels
	for tag, tunnel := range r.tunnels {
		if err := tunnel.Stop(); err != nil {
			lastErr = fmt.Errorf("failed to stop tunnel %s: %w", tag, err)
		}
	}

	// Stop DNS router
	if err := r.dnsrouter.Stop(); err != nil {
		lastErr = fmt.Errorf("failed to stop DNS router: %w", err)
	}

	return lastErr
}

// IsRunning returns true if any router services are currently active.
func (r *Router) IsRunning() bool {
	if r.config.IsSingleMode() {
		active := r.config.Route.Active
		if active != "" {
			if tunnel, ok := r.tunnels[active]; ok {
				return tunnel.IsActive()
			}
		}
		return false
	}
	// Multi mode - check dnsrouter or any tunnel
	if r.dnsrouter.IsActive() {
		return true
	}
	for _, tunnel := range r.tunnels {
		if tunnel.IsActive() {
			return true
		}
	}
	return false
}

// Restart restarts all services based on current mode.
func (r *Router) Restart() error {
	if err := r.Stop(); err != nil {
		return err
	}
	return r.Start()
}

// AddTunnel adds a new tunnel.
func (r *Router) AddTunnel(cfg *config.TunnelConfig) error {
	if err := ValidateTag(cfg.Tag); err != nil {
		return err
	}

	if _, exists := r.tunnels[cfg.Tag]; exists {
		return fmt.Errorf("tunnel %s already exists", cfg.Tag)
	}

	if cfg.Port == 0 {
		cfg.Port = r.config.AllocateNextPort()
	}

	// Generate or reuse certificate/keys
	if err := r.ensureCryptoMaterial(cfg); err != nil {
		return err
	}

	// Create tunnel
	tunnel := NewTunnel(cfg)

	// Update config
	r.config.Tunnels = append(r.config.Tunnels, *cfg)
	r.tunnels[cfg.Tag] = tunnel

	// In single mode: auto-set as active if first tunnel
	if r.config.IsSingleMode() {
		if r.config.Route.Active == "" {
			r.config.Route.Active = cfg.Tag
		}
	} else {
		// In multi mode: regenerate DNS router config
		if err := r.regenerateDNSRouterConfig(); err != nil {
			return fmt.Errorf("failed to regenerate DNS router config: %w", err)
		}
		// Set as default if first tunnel
		if r.config.Route.Default == "" {
			r.config.Route.Default = cfg.Tag
		}
	}

	// Save config
	if err := r.config.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// RemoveTunnel removes a tunnel.
func (r *Router) RemoveTunnel(tag string) error {
	tunnel, exists := r.tunnels[tag]
	if !exists {
		return fmt.Errorf("tunnel %s not found", tag)
	}

	// Remove service
	if err := tunnel.RemoveService(); err != nil {
		return fmt.Errorf("failed to remove service: %w", err)
	}

	// Remove tunnel config directory
	if err := tunnel.RemoveConfigDir(); err != nil {
		return fmt.Errorf("failed to remove config directory: %w", err)
	}

	// Remove from config
	var newTunnels []config.TunnelConfig
	for _, t := range r.config.Tunnels {
		if t.Tag != tag {
			newTunnels = append(newTunnels, t)
		}
	}
	r.config.Tunnels = newTunnels
	delete(r.tunnels, tag)

	// Handle mode-specific cleanup
	if r.config.IsSingleMode() {
		// Update active tunnel if needed
		if r.config.Route.Active == tag {
			r.config.Route.Active = ""
			// Set to first available tunnel
			if len(r.config.Tunnels) > 0 {
				r.config.Route.Active = r.config.Tunnels[0].Tag
			}
		}
	} else {
		// Update default route if needed
		if r.config.Route.Default == tag {
			r.config.Route.Default = ""
			// Set to first available tunnel
			if len(r.config.Tunnels) > 0 {
				r.config.Route.Default = r.config.Tunnels[0].Tag
			}
		}

		// Regenerate DNS router config
		if err := r.regenerateDNSRouterConfig(); err != nil {
			return fmt.Errorf("failed to regenerate DNS router config: %w", err)
		}
	}

	// Save config
	if err := r.config.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// GetTunnel returns a tunnel by tag.
func (r *Router) GetTunnel(tag string) *Tunnel {
	return r.tunnels[tag]
}

// GetAllTunnels returns all tunnels.
func (r *Router) GetAllTunnels() map[string]*Tunnel {
	return r.tunnels
}

// GetConfig returns the current configuration.
func (r *Router) GetConfig() *config.Config {
	return r.config
}

// Reload reloads the configuration and regenerates the DNS router config.
func (r *Router) Reload() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	r.config = cfg

	// Recreate tunnels
	r.tunnels = make(map[string]*Tunnel)
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		r.tunnels[t.Tag] = NewTunnel(t)
	}

	// Only regenerate and restart DNS router in multi mode
	if r.config.IsMultiMode() {
		if err := r.regenerateDNSRouterConfig(); err != nil {
			return fmt.Errorf("failed to regenerate DNS router config: %w", err)
		}

		if err := r.dnsrouter.Restart(); err != nil {
			return fmt.Errorf("failed to restart DNS router: %w", err)
		}
	}

	return nil
}

// ensureCryptoMaterial ensures certificates or keys exist for the domain.
func (r *Router) ensureCryptoMaterial(cfg *config.TunnelConfig) error {
	if cfg.Transport == config.TransportSlipstream {
		certInfo, err := r.certMgr.GetOrCreate(cfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to get certificate: %w", err)
		}

		// Update slipstream config with cert paths
		if cfg.Slipstream == nil {
			cfg.Slipstream = &config.SlipstreamConfig{}
		}
		cfg.Slipstream.Cert = certInfo.CertPath
		cfg.Slipstream.Key = certInfo.KeyPath
	}

	return nil
}

// RegenerateDNSRouterConfig regenerates the DNS router configuration.
func (r *Router) RegenerateDNSRouterConfig() error {
	return r.regenerateDNSRouterConfig()
}

// regenerateDNSRouterConfig is the internal implementation.
func (r *Router) regenerateDNSRouterConfig() error {
	// Resolve listen address (0.0.0.0 -> external IP)
	listenAddr := r.resolveListenAddress(r.config.Listen.Address)

	// Convert tunnels to RouteInputs
	routes := r.convertToRouteInputs()

	// Get default backend
	defaultBackend := ""
	if r.config.Route.Default != "" {
		if t := r.config.GetTunnelByTag(r.config.Route.Default); t != nil {
			defaultBackend = fmt.Sprintf("127.0.0.1:%d", t.Port)
		}
	}

	cfg := dnsrouter.GenerateConfig(listenAddr, routes, defaultBackend)

	if err := dnsrouter.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save DNS router config: %w", err)
	}

	return nil
}

// resolveListenAddress resolves a listen address, replacing 0.0.0.0 with external IP.
func (r *Router) resolveListenAddress(addr string) string {
	// Check if address uses 0.0.0.0
	if len(addr) < 8 || addr[:8] != "0.0.0.0:" {
		return addr
	}

	// Extract port
	port := addr[8:]

	// Try to detect external IP
	externalIP, err := network.GetExternalIP()
	if err != nil {
		// Fall back to original address if detection fails
		return addr
	}

	return fmt.Sprintf("%s:%s", externalIP, port)
}

// convertToRouteInputs converts enabled tunnels to dnsrouter.RouteInput slice.
func (r *Router) convertToRouteInputs() []dnsrouter.RouteInput {
	var routes []dnsrouter.RouteInput
	for _, t := range r.config.Tunnels {
		if t.IsEnabled() {
			routes = append(routes, dnsrouter.RouteInput{
				Domain:  t.Domain,
				Backend: fmt.Sprintf("127.0.0.1:%d", t.Port),
			})
		}
	}
	return routes
}

// SetDefaultRoute sets the default routing tunnel.
func (r *Router) SetDefaultRoute(tag string) error {
	if tag != "" {
		if _, exists := r.tunnels[tag]; !exists {
			return fmt.Errorf("tunnel %s not found", tag)
		}
	}

	r.config.Route.Default = tag

	if err := r.regenerateDNSRouterConfig(); err != nil {
		return err
	}

	if err := r.config.Save(); err != nil {
		return err
	}

	return nil
}

// Initialize initializes the router configuration and directories.
func Initialize() error {
	// Create main config directory with 0755 to allow dnstm user to traverse
	if err := os.MkdirAll(config.ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", config.ConfigDir, err)
	}

	// Create subdirectories with 0750 (owned by dnstm, so accessible to dnstm)
	subdirs := []string{config.CertsDir, config.KeysDir, config.TunnelsDir}
	for _, dir := range subdirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		// Set ownership to dnstm user
		if err := system.ChownDirToDnstm(dir); err != nil {
			return fmt.Errorf("failed to set ownership of %s: %w", dir, err)
		}
	}

	// Clear any stale NAT rules from previous configurations
	// The new architecture doesn't use NAT - transports bind directly to external IP
	network.ClearNATOnly()

	// Create default config if not exists
	if !config.ConfigExists() {
		cfg := config.Default()
		// Ensure built-in backends are added
		cfg.EnsureBuiltinBackends()
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save default config: %w", err)
		}
	}

	return nil
}

// IsInitialized checks if the router has been initialized.
func IsInitialized() bool {
	return config.ConfigExists()
}

// GetDNSRouterService returns the DNS router service.
func (r *Router) GetDNSRouterService() *dnsrouter.Service {
	return r.dnsrouter
}

// GetModeDisplayName returns a human-readable name for a mode.
func GetModeDisplayName(mode string) string {
	switch mode {
	case "single":
		return "Single-tunnel"
	case "multi":
		return "Multi-tunnel"
	default:
		return mode
	}
}
