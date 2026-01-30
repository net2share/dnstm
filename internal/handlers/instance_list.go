package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/types"
)

func init() {
	actions.SetInstanceHandler(actions.ActionInstanceList, HandleInstanceList)
}

// HandleInstanceList lists all configured instances.
func HandleInstanceList(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Transports) == 0 {
		ctx.Output.Println("No instances configured")
		return nil
	}

	ctx.Output.Println()
	modeName := router.GetModeDisplayName(cfg.Mode)
	ctx.Output.Printf("Mode: %s\n\n", modeName)

	// Print header
	ctx.Output.Printf("%-16s %-24s %-8s %-20s %s\n", "NAME", "TYPE", "PORT", "DOMAIN", "STATUS")
	ctx.Output.Separator(80)

	// Print instances
	for name, transport := range cfg.Transports {
		instance := router.NewInstance(name, transport)
		status := "Stopped"
		if instance.IsActive() {
			status = "Running"
		}

		// Add marker for active/default instance
		marker := ""
		if cfg.IsSingleMode() && cfg.Single.Active == name {
			marker = " *"
		} else if cfg.IsMultiMode() && cfg.Routing.Default == name {
			marker = " (default)"
		}

		typeName := types.GetTransportTypeDisplayName(transport.Type)
		ctx.Output.Printf("%-16s %-24s %-8d %-20s %s%s\n",
			name, typeName, transport.Port, transport.Domain, status, marker)
	}

	if cfg.IsSingleMode() {
		ctx.Output.Println("\n* = active instance")
	}
	ctx.Output.Println()

	return nil
}
