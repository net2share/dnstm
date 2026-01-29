package router

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/types"
)

// Router orchestrates multiple transport instances and the DNS router.
type Router struct {
	config    *Config
	instances map[string]*Instance
	dnsrouter *dnsrouter.Service
	certMgr   *certs.Manager
}

// New creates a new router from configuration.
func New(cfg *Config) (*Router, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	r := &Router{
		config:    cfg,
		instances: make(map[string]*Instance),
		dnsrouter: dnsrouter.NewService(),
		certMgr:   certs.NewManager(),
	}

	// Create instances from config
	for name, transportCfg := range cfg.Transports {
		r.instances[name] = NewInstance(name, transportCfg)
	}

	return r, nil
}

// Start starts the router based on the current mode.
// In single mode: starts the active instance (binds directly to EXTERNAL_IP:53).
// In multi mode: starts the DNS router and all instances.
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

// startSingleMode starts the active instance which binds directly to EXTERNAL_IP:53.
// No DNAT is needed since the transport binds directly to the external interface.
func (r *Router) startSingleMode() error {
	active := r.config.Single.Active
	if active == "" {
		return fmt.Errorf("no active instance configured; use 'dnstm instance add' first")
	}

	_, ok := r.config.Transports[active]
	if !ok {
		return fmt.Errorf("active instance '%s' not found", active)
	}

	instance, ok := r.instances[active]
	if !ok {
		return fmt.Errorf("active instance '%s' not initialized", active)
	}

	// Always regenerate service for single mode to ensure correct binding (EXTERNAL_IP:53)
	// This handles cases where instance was created in multi-mode or mode changed
	if err := instance.RegenerateService(ServiceModeSingle); err != nil {
		return fmt.Errorf("failed to configure service for single mode: %w", err)
	}

	// Clear any stale NAT rules (transport binds directly to external IP, no NAT needed)
	network.ClearNATOnly()
	// Ensure firewall allows port 53
	network.AllowPort53()

	// Start the instance
	if err := instance.Start(); err != nil {
		return fmt.Errorf("failed to start instance %s: %w", active, err)
	}

	return nil
}

// startMultiMode starts the DNS router and all instances.
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

	// Start all backend instances FIRST (before dnsrouter)
	// This ensures backends are ready to receive traffic when dnsrouter starts
	for name, instance := range r.instances {
		if err := instance.Start(); err != nil {
			return fmt.Errorf("failed to start instance %s: %w", name, err)
		}
	}

	// Start DNS router AFTER instances are ready
	if err := r.dnsrouter.Start(); err != nil {
		return fmt.Errorf("failed to start DNS router: %w", err)
	}

	return nil
}

// Stop stops the router based on the current mode.
// In single mode: stops the active instance and removes NAT rules.
// In multi mode: stops all instances and the DNS router.
func (r *Router) Stop() error {
	if r.config.IsSingleMode() {
		return r.stopSingleMode()
	}
	return r.stopMultiMode()
}

// stopSingleMode stops the active instance.
// No NAT cleanup needed since we bind directly to external IP.
func (r *Router) stopSingleMode() error {
	var lastErr error

	active := r.config.Single.Active
	if active != "" {
		if instance, ok := r.instances[active]; ok {
			if err := instance.Stop(); err != nil {
				lastErr = fmt.Errorf("failed to stop instance %s: %w", active, err)
			}
		}
	}

	return lastErr
}

// stopMultiMode stops all instances and the DNS router.
func (r *Router) stopMultiMode() error {
	var lastErr error

	// Stop all instances
	for name, instance := range r.instances {
		if err := instance.Stop(); err != nil {
			lastErr = fmt.Errorf("failed to stop instance %s: %w", name, err)
		}
	}

	// Stop DNS router
	if err := r.dnsrouter.Stop(); err != nil {
		lastErr = fmt.Errorf("failed to stop DNS router: %w", err)
	}

	return lastErr
}

// Restart restarts all services based on current mode.
func (r *Router) Restart() error {
	if err := r.Stop(); err != nil {
		return err
	}
	return r.Start()
}

// AddInstance adds a new transport instance.
func (r *Router) AddInstance(name string, cfg *types.TransportConfig) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	if _, exists := r.instances[name]; exists {
		return fmt.Errorf("instance %s already exists", name)
	}

	if cfg.Port == 0 {
		port, err := AllocatePort(r.config.Transports)
		if err != nil {
			return err
		}
		cfg.Port = port
	} else if !IsPortAvailable(cfg.Port, r.config.Transports) {
		return fmt.Errorf("port %d is not available", cfg.Port)
	}

	// Generate or reuse certificate/keys
	if err := r.ensureCryptoMaterial(cfg); err != nil {
		return err
	}

	// Create instance
	instance := NewInstance(name, cfg)

	// Create systemd service
	if err := instance.CreateService(); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// Set permissions
	if err := instance.SetPermissions(); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Update config
	r.config.Transports[name] = cfg
	r.instances[name] = instance

	// In single mode: auto-set as active if first instance
	if r.config.IsSingleMode() {
		if r.config.Single.Active == "" {
			r.config.Single.Active = name
		}
	} else {
		// In multi mode: regenerate DNS router config
		if err := r.regenerateDNSRouterConfig(); err != nil {
			return fmt.Errorf("failed to regenerate DNS router config: %w", err)
		}
		// Set as default if first instance
		if r.config.Routing.Default == "" {
			r.config.Routing.Default = name
		}
	}

	// Save config
	if err := r.config.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// RemoveInstance removes a transport instance.
func (r *Router) RemoveInstance(name string) error {
	instance, exists := r.instances[name]
	if !exists {
		return fmt.Errorf("instance %s not found", name)
	}

	// Remove service
	if err := instance.RemoveService(); err != nil {
		return fmt.Errorf("failed to remove service: %w", err)
	}

	// Remove instance config directory
	if err := instance.RemoveConfigDir(); err != nil {
		return fmt.Errorf("failed to remove config directory: %w", err)
	}

	// Update config
	delete(r.config.Transports, name)
	delete(r.instances, name)

	// Handle mode-specific cleanup
	if r.config.IsSingleMode() {
		// Update active instance if needed
		if r.config.Single.Active == name {
			r.config.Single.Active = ""
			// Set to first available instance
			for n := range r.config.Transports {
				r.config.Single.Active = n
				break
			}
		}
	} else {
		// Update default route if needed
		if r.config.Routing.Default == name {
			r.config.Routing.Default = ""
			// Set to first available instance
			for n := range r.config.Transports {
				r.config.Routing.Default = n
				break
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

// GetInstance returns an instance by name.
func (r *Router) GetInstance(name string) *Instance {
	return r.instances[name]
}

// GetAllInstances returns all instances.
func (r *Router) GetAllInstances() map[string]*Instance {
	return r.instances
}

// GetConfig returns the current configuration.
func (r *Router) GetConfig() *Config {
	return r.config
}

// Reload reloads the configuration and regenerates the DNS router config.
func (r *Router) Reload() error {
	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	r.config = cfg

	// Recreate instances
	r.instances = make(map[string]*Instance)
	for name, transportCfg := range cfg.Transports {
		r.instances[name] = NewInstance(name, transportCfg)
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
func (r *Router) ensureCryptoMaterial(cfg *types.TransportConfig) error {
	if types.IsSlipstreamType(cfg.Type) {
		certInfo, err := r.certMgr.GetOrCreate(cfg.Domain)
		if err != nil {
			return fmt.Errorf("failed to get certificate: %w", err)
		}

		// Update certificate config
		if r.config.Certificates == nil {
			r.config.Certificates = make(map[string]*CertConfig)
		}
		r.config.Certificates[cfg.Domain] = &CertConfig{
			Cert:        certInfo.CertPath,
			Key:         certInfo.KeyPath,
			Fingerprint: certInfo.Fingerprint,
		}
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

	// Convert transports to RouteInputs
	routes := r.convertToRouteInputs()

	// Get default backend
	defaultBackend := ""
	if r.config.Routing.Default != "" {
		if transport, ok := r.config.Transports[r.config.Routing.Default]; ok {
			defaultBackend = fmt.Sprintf("127.0.0.1:%d", transport.Port)
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

// convertToRouteInputs converts transports to dnsrouter.RouteInput slice.
func (r *Router) convertToRouteInputs() []dnsrouter.RouteInput {
	routes := make([]dnsrouter.RouteInput, 0, len(r.config.Transports))
	for _, transport := range r.config.Transports {
		routes = append(routes, dnsrouter.RouteInput{
			Domain:  transport.Domain,
			Backend: fmt.Sprintf("127.0.0.1:%d", transport.Port),
		})
	}
	return routes
}

// SetDefaultRoute sets the default routing instance.
func (r *Router) SetDefaultRoute(name string) error {
	if name != "" {
		if _, exists := r.instances[name]; !exists {
			return fmt.Errorf("instance %s not found", name)
		}
	}

	r.config.Routing.Default = name

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
	if err := os.MkdirAll(ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", ConfigDir, err)
	}

	// Create subdirectories with 0750 (owned by dnstm, so accessible to dnstm)
	subdirs := []string{CertsDir, KeysDir, filepath.Join(ConfigDir, "instances")}
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
	if !ConfigExists() {
		cfg := Default()
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save default config: %w", err)
		}
	}

	return nil
}

// IsInitialized checks if the router has been initialized.
func IsInitialized() bool {
	return ConfigExists()
}

// GetDNSRouterService returns the DNS router service.
func (r *Router) GetDNSRouterService() *dnsrouter.Service {
	return r.dnsrouter
}
