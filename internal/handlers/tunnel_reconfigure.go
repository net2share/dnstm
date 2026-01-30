package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/go-corelib/tui"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelReconfigure, HandleTunnelReconfigure)
}

// HandleTunnelReconfigure reconfigures an existing tunnel.
func HandleTunnelReconfigure(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	tag := ctx.GetArg(0)
	if tag == "" {
		return actions.NewActionError("tunnel tag required", "Usage: dnstm tunnel reconfigure <tag>")
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	tunnelCfg := cfg.GetTunnelByTag(tag)
	if tunnelCfg == nil {
		return actions.TunnelNotFoundError(tag)
	}

	ctx.Output.Println()
	ctx.Output.Info(fmt.Sprintf("Reconfiguring '%s'...", tag))
	ctx.Output.Info("Press Enter to keep current value, or type a new value.")
	ctx.Output.Println()

	changed := false
	renamed := false
	newTag := tag

	// Check if tunnel is running before we start
	oldTunnel := router.NewTunnel(tunnelCfg)
	wasRunning := oldTunnel.IsActive()

	// Tag (rename)
	inputTag, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Tunnel Tag",
		Description: fmt.Sprintf("Current: %s", tag),
		Value:       tag,
	})
	if err != nil {
		return err
	}
	if !confirmed {
		ctx.Output.Info("Cancelled")
		return nil
	}
	if inputTag != "" && inputTag != tag {
		inputTag = router.NormalizeName(inputTag)
		if err := router.ValidateName(inputTag); err != nil {
			return fmt.Errorf("invalid tag: %w", err)
		}
		if cfg.GetTunnelByTag(inputTag) != nil {
			return actions.TunnelExistsError(inputTag)
		}
		newTag = inputTag
		renamed = true
		changed = true
	}

	// Domain
	newDomain, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Domain",
		Description: fmt.Sprintf("Current: %s", tunnelCfg.Domain),
		Value:       tunnelCfg.Domain,
	})
	if err != nil {
		return err
	}
	if !confirmed {
		ctx.Output.Info("Cancelled")
		return nil
	}
	if newDomain != "" && newDomain != tunnelCfg.Domain {
		tunnelCfg.Domain = newDomain
		changed = true
	}

	// Backend selection
	backendOptions := buildBackendOptions(cfg, tunnelCfg.Transport)
	if len(backendOptions) > 0 {
		// Find current selection index
		selected := 0
		for i, opt := range backendOptions {
			if opt.Value == tunnelCfg.Backend {
				selected = i
				break
			}
		}

		newBackend, err := tui.RunMenu(tui.MenuConfig{
			Title:       "Backend",
			Description: fmt.Sprintf("Current: %s", tunnelCfg.Backend),
			Options:     backendOptions,
			Selected:    selected,
		})
		if err != nil {
			return err
		}
		if newBackend == "" {
			ctx.Output.Info("Cancelled")
			return nil
		}
		if newBackend != tunnelCfg.Backend {
			tunnelCfg.Backend = newBackend
			changed = true
		}
	}

	// DNSTT-specific: MTU
	if tunnelCfg.Transport == config.TransportDNSTT {
		currentMTU := tunnelCfg.GetMTU()
		newMTUStr, confirmed, err := tui.RunInput(tui.InputConfig{
			Title:       "MTU",
			Description: fmt.Sprintf("Current: %d", currentMTU),
			Value:       fmt.Sprintf("%d", currentMTU),
		})
		if err != nil {
			return err
		}
		if !confirmed {
			ctx.Output.Info("Cancelled")
			return nil
		}
		if newMTUStr != "" {
			var newMTU int
			fmt.Sscanf(newMTUStr, "%d", &newMTU)
			if newMTU > 0 && newMTU != currentMTU {
				if tunnelCfg.DNSTT == nil {
					tunnelCfg.DNSTT = &config.DNSTTConfig{}
				}
				tunnelCfg.DNSTT.MTU = newMTU
				changed = true
			}
		}
	}

	if !changed {
		ctx.Output.Info("No changes made")
		return nil
	}

	// Handle rename
	if renamed {
		// Stop and remove old service
		oldTunnel.Stop()
		oldTunnel.RemoveService()
		oldTunnel.RemoveConfigDir()

		// Update tunnel tag
		tunnelCfg.Tag = newTag

		// Update Route.Active if it referenced the old tag
		if cfg.Route.Active == tag {
			cfg.Route.Active = newTag
		}
		// Update Route.Default if it referenced the old tag
		if cfg.Route.Default == tag {
			cfg.Route.Default = newTag
		}
	}

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create new service with appropriate mode
	newTunnel := router.NewTunnel(tunnelCfg)

	// Determine service mode based on current router mode
	serviceMode := router.ServiceModeMulti
	if cfg.IsSingleMode() {
		// Is this the active tunnel?
		isActive := cfg.Route.Active == newTag
		if isActive {
			serviceMode = router.ServiceModeSingle
		}
	}

	// Get backend for service creation
	backend := cfg.GetBackendByTag(tunnelCfg.Backend)
	if backend == nil {
		return actions.BackendNotFoundError(tunnelCfg.Backend)
	}

	if err := createTunnelService(tunnelCfg, backend, serviceMode); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	if err := newTunnel.SetPermissions(); err != nil {
		ctx.Output.Warning("Permission warning: " + err.Error())
	}

	// Start if it was running before
	if wasRunning {
		if err := newTunnel.Enable(); err != nil {
			ctx.Output.Warning("Failed to enable service: " + err.Error())
		}
		if err := newTunnel.Start(); err != nil {
			return fmt.Errorf("failed to start: %w", err)
		}
		if renamed {
			ctx.Output.Success(fmt.Sprintf("Tunnel renamed to '%s' and restarted!", newTag))
		} else {
			ctx.Output.Success(fmt.Sprintf("Tunnel '%s' reconfigured and restarted!", newTag))
		}
	} else {
		if renamed {
			ctx.Output.Success(fmt.Sprintf("Tunnel renamed to '%s'!", newTag))
		} else {
			ctx.Output.Success(fmt.Sprintf("Tunnel '%s' reconfigured!", newTag))
		}
	}

	return nil
}
