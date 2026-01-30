package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/types"
)

func init() {
	actions.SetRouterHandler(actions.ActionRouterSwitch, HandleRouterSwitch)
}

// HandleRouterSwitch switches the active instance in single mode.
func HandleRouterSwitch(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check mode
	if !cfg.IsSingleMode() {
		return actions.SingleModeOnlyError()
	}

	// Check if there are instances to switch to
	if len(cfg.Transports) == 0 {
		return actions.NoInstancesError()
	}

	instanceName := ctx.GetArg(0)

	// If only one instance, just make sure it's active
	if len(cfg.Transports) == 1 {
		for name := range cfg.Transports {
			if cfg.Single.Active == name {
				ctx.Output.Info(fmt.Sprintf("'%s' is already the active instance", name))
				return nil
			}
			instanceName = name
			break
		}
	}

	// If no instance name provided, the adapter should have shown a picker
	if instanceName == "" {
		return actions.NewActionError("instance name required", "Usage: dnstm router switch <instance>")
	}

	// Verify instance exists
	transport, ok := cfg.Transports[instanceName]
	if !ok {
		return actions.NotFoundError(instanceName)
	}

	// Check if already active
	if cfg.Single.Active == instanceName {
		ctx.Output.Info(fmt.Sprintf("'%s' is already the active instance", instanceName))
		return nil
	}

	// Create router and switch
	r, err := router.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	ctx.Output.Println()
	ctx.Output.Info(fmt.Sprintf("Switching to '%s'...", instanceName))
	ctx.Output.Println()

	if err := r.SwitchActiveInstance(instanceName); err != nil {
		return fmt.Errorf("failed to switch instance: %w", err)
	}

	// Show success
	typeName := types.GetTransportTypeDisplayName(transport.Type)

	ctx.Output.Println()
	ctx.Output.Success(fmt.Sprintf("Switched to '%s'!", instanceName))
	ctx.Output.Println()

	var lines []string
	lines = append(lines, ctx.Output.KV("Instance: ", instanceName))
	lines = append(lines, ctx.Output.KV("Type:     ", typeName))
	lines = append(lines, ctx.Output.KV("Domain:   ", transport.Domain))
	lines = append(lines, ctx.Output.KV("Port:     ", fmt.Sprintf("%d", transport.Port)))
	ctx.Output.Box("Active Tunnel", lines)
	ctx.Output.Println()

	return nil
}
