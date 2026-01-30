package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
)

func init() {
	actions.SetInstanceHandler(actions.ActionInstanceLogs, HandleInstanceLogs)
}

// HandleInstanceLogs shows logs for a specific instance.
func HandleInstanceLogs(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	name := ctx.GetArg(0)
	if name == "" {
		return actions.NewActionError("instance name required", "Usage: dnstm instance logs <name>")
	}

	instance, err := GetInstanceByName(ctx, name)
	if err != nil {
		return err
	}

	lines := ctx.GetInt("lines")
	if lines == 0 {
		lines = 50 // default
	}

	logs, err := instance.GetLogs(lines)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	ctx.Output.Println(logs)
	return nil
}
