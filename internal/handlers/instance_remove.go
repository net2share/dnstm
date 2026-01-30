package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetInstanceHandler(actions.ActionInstanceRemove, HandleInstanceRemove)
}

// HandleInstanceRemove removes an instance.
func HandleInstanceRemove(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	name := ctx.GetArg(0)
	if name == "" {
		return actions.NewActionError("instance name required", "Usage: dnstm instance remove <name>")
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	transportCfg, exists := cfg.Transports[name]
	if !exists {
		return actions.NotFoundError(name)
	}

	// Warn if removing the active instance in single mode
	if cfg.IsSingleMode() && cfg.Single.Active == name {
		ctx.Output.Println()
		ctx.Output.Warning("This is the currently active instance.")
		if len(cfg.Transports) > 1 {
			ctx.Output.Info("After removal, run 'dnstm router switch <name>' to activate another instance.")
		} else {
			ctx.Output.Info("After removal, no transport will be active. Add a new instance to continue.")
		}
		ctx.Output.Println()
	}

	// Confirmation is handled by the adapter (CLI or menu)
	// The handler assumes confirmation has already been obtained

	ctx.Output.Println()
	ctx.Output.Info("Removing instance...")
	ctx.Output.Println()

	totalSteps := 3
	currentStep := 0

	// Step 1: Stop and remove service
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing service...")
	instance := router.NewInstance(name, transportCfg)
	if err := instance.RemoveService(); err != nil {
		ctx.Output.Warning("Service removal warning: " + err.Error())
	} else {
		ctx.Output.Status("Service removed")
	}

	// Step 2: Remove config directory
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing configuration...")
	if err := instance.RemoveConfigDir(); err != nil {
		ctx.Output.Warning("Config removal warning: " + err.Error())
	} else {
		ctx.Output.Status("Configuration removed")
	}

	// Step 3: Update config
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Updating router configuration...")
	delete(cfg.Transports, name)

	// Update Routing.Default if needed (multi mode)
	if cfg.Routing.Default == name {
		cfg.Routing.Default = ""
		for n := range cfg.Transports {
			cfg.Routing.Default = n
			break
		}
	}

	// Update Single.Active if needed (single mode)
	if cfg.Single.Active == name {
		cfg.Single.Active = ""
		for n := range cfg.Transports {
			cfg.Single.Active = n
			break
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	ctx.Output.Status("Configuration updated")

	ctx.Output.Println()
	ctx.Output.Success(fmt.Sprintf("Instance '%s' removed!", name))
	ctx.Output.Println()

	return nil
}
