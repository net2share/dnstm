package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
)

func init() {
	actions.SetBackendHandler(actions.ActionBackendRemove, HandleBackendRemove)
}

// HandleBackendRemove removes a backend.
func HandleBackendRemove(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get tag from args
	tag := ctx.GetArg(0)
	if tag == "" {
		return fmt.Errorf("backend tag is required")
	}

	// Check if backend exists
	backend := cfg.GetBackendByTag(tag)
	if backend == nil {
		return actions.BackendNotFoundError(tag)
	}

	// Check if backend is built-in
	if backend.IsBuiltIn() && (tag == "socks" || tag == "ssh") {
		return fmt.Errorf("cannot remove built-in backend '%s'", tag)
	}

	// Check if backend is in use by any tunnels
	tunnelsUsingBackend := cfg.GetTunnelsUsingBackend(tag)
	if len(tunnelsUsingBackend) > 0 {
		var tunnelTags []string
		for _, t := range tunnelsUsingBackend {
			tunnelTags = append(tunnelTags, t.Tag)
		}
		return actions.BackendInUseError(tag, tunnelTags)
	}

	// Find and remove the backend
	var newBackends []config.BackendConfig
	for _, b := range cfg.Backends {
		if b.Tag != tag {
			newBackends = append(newBackends, b)
		}
	}
	cfg.Backends = newBackends

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Backend '%s' removed", tag))

	return nil
}
