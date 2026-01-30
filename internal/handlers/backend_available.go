package handlers

import (
	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
)

func init() {
	actions.SetBackendHandler(actions.ActionBackendAvailable, HandleBackendAvailable)
}

// HandleBackendAvailable shows available backend types.
func HandleBackendAvailable(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, false); err != nil {
		return err
	}

	ctx.Output.Println()
	ctx.Output.Println("Available Backend Types")
	ctx.Output.Separator(60)
	ctx.Output.Println()

	// Print header
	ctx.Output.Printf("%-16s %-16s %s\n", "TYPE", "STATUS", "DESCRIPTION")
	ctx.Output.Separator(60)

	for _, t := range config.GetBackendTypes() {
		info := config.GetBackendTypeInfo(t)
		if info == nil {
			continue
		}

		status := "available"
		switch info.Category {
		case config.CategoryBuiltIn:
			if info.IsInstalled() {
				status = "[installed]"
			} else {
				status = "[not installed]"
			}
		case config.CategorySystem:
			status = "[system]"
		case config.CategoryCustom:
			status = "[always]"
		}

		ctx.Output.Printf("%-16s %-16s %s\n",
			info.Type, status, info.Description)
	}

	ctx.Output.Println()

	return nil
}
