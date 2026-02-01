package installer

import (
	"os"
	"path/filepath"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/router"
)

// CleanupResult contains the results of a cleanup operation.
type CleanupResult struct {
	TunnelsRemoved   []string
	TunnelErrors     map[string]error
	RouterStopped    bool
	RouterStopError  error
	DirsRemoved      int
}

// CleanupTunnelsAndRouter removes all tunnel services and stops the DNS router.
// This is used by both config load (to prepare for new config) and uninstall.
// If removeDirs is true, tunnel directories are also removed.
func CleanupTunnelsAndRouter(removeDirs bool) *CleanupResult {
	result := &CleanupResult{
		TunnelsRemoved: []string{},
		TunnelErrors:   make(map[string]error),
	}

	// Load existing config to get list of tunnels
	cfg, err := router.Load()
	if err != nil {
		// No config, nothing to clean up
		return result
	}

	// Stop and remove all tunnel services
	for _, t := range cfg.Tunnels {
		tunnel := router.NewTunnel(&t)
		if err := tunnel.RemoveService(); err != nil {
			result.TunnelErrors[t.Tag] = err
		} else {
			result.TunnelsRemoved = append(result.TunnelsRemoved, t.Tag)
		}
	}

	// Stop the DNS router service
	routerSvc := dnsrouter.NewService()
	if routerSvc.IsActive() {
		if err := routerSvc.Stop(); err != nil {
			result.RouterStopError = err
		} else {
			result.RouterStopped = true
		}
	}

	// Remove tunnel directories if requested
	if removeDirs {
		tunnelsDir := config.TunnelsDir
		if entries, err := os.ReadDir(tunnelsDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					tunnelDir := filepath.Join(tunnelsDir, entry.Name())
					if os.RemoveAll(tunnelDir) == nil {
						result.DirsRemoved++
					}
				}
			}
		}
	}

	return result
}
