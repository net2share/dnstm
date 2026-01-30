package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
)

func init() {
	actions.SetBackendHandler(actions.ActionBackendList, HandleBackendList)
}

// HandleBackendList lists all configured backends.
func HandleBackendList(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Backends) == 0 {
		ctx.Output.Println("No backends configured")
		return nil
	}

	ctx.Output.Println()

	// Print header
	ctx.Output.Printf("%-16s %-16s %-24s %s\n", "TAG", "TYPE", "ADDRESS", "STATUS")
	ctx.Output.Separator(70)

	// Print backends
	for _, b := range cfg.Backends {
		typeName := config.GetBackendTypeDisplayName(b.Type)
		address := b.Address
		if b.Type == config.BackendShadowsocks {
			address = "[SIP003 plugin]"
		}

		status := "External"
		if b.IsManaged() {
			status = "Managed"
		}
		if b.IsBuiltIn() {
			status = "Built-in"
		}

		ctx.Output.Printf("%-16s %-16s %-24s %s\n",
			b.Tag, typeName, address, status)
	}

	ctx.Output.Println()

	return nil
}
