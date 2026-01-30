package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
)

func init() {
	actions.SetInstanceHandler(actions.ActionInstanceStart, HandleInstanceStart)
	actions.SetInstanceHandler(actions.ActionInstanceStop, HandleInstanceStop)
}

// HandleInstanceStart starts or restarts an instance.
func HandleInstanceStart(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	name := ctx.GetArg(0)
	if name == "" {
		return actions.NewActionError("instance name required", "Usage: dnstm instance start <name>")
	}

	instance, err := GetInstanceByName(ctx, name)
	if err != nil {
		return err
	}

	if err := instance.Enable(); err != nil {
		ctx.Output.Warning("Failed to enable service: " + err.Error())
	}

	isRunning := instance.IsActive()
	if isRunning {
		if err := instance.Restart(); err != nil {
			return fmt.Errorf("failed to restart instance: %w", err)
		}
		ctx.Output.Success(fmt.Sprintf("Instance '%s' restarted", name))
	} else {
		if err := instance.Start(); err != nil {
			return fmt.Errorf("failed to start instance: %w", err)
		}
		ctx.Output.Success(fmt.Sprintf("Instance '%s' started", name))
	}

	return nil
}

// HandleInstanceStop stops an instance.
func HandleInstanceStop(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	name := ctx.GetArg(0)
	if name == "" {
		return actions.NewActionError("instance name required", "Usage: dnstm instance stop <name>")
	}

	instance, err := GetInstanceByName(ctx, name)
	if err != nil {
		return err
	}

	if err := instance.Stop(); err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Instance '%s' stopped", name))
	return nil
}
