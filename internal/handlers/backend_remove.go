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

	tag := ctx.GetString("tag")
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

	// Start progress view in interactive mode
	if ctx.IsInteractive {
		ctx.Output.BeginProgress(fmt.Sprintf("Remove Backend: %s", tag))
	} else {
		ctx.Output.Println()
	}

	// Save config
	if err := cfg.Save(); err != nil {
		if ctx.IsInteractive {
			ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
			ctx.Output.EndProgress()
			return nil
		}
		return fmt.Errorf("failed to save config: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Backend '%s' removed", tag))

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	} else {
		ctx.Output.Println()
	}

	return nil
}
