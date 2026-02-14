package handlers

import (
	"fmt"
	"os"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetConfigHandler(actions.ActionConfigValidate, HandleConfigValidate)
}

// HandleConfigValidate validates a configuration file.
func HandleConfigValidate(ctx *actions.Context) error {
	filePath := ctx.GetArg(0)
	if filePath == "" {
		return actions.NewActionError("file path required", "Usage: dnstm config validate <file>")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return actions.NewActionError(
			fmt.Sprintf("file not found: %s", filePath),
			"Please provide a valid config.json file path",
		)
	}

	ctx.Output.Println()
	ctx.Output.Info(fmt.Sprintf("Validating %s...", filePath))
	ctx.Output.Println()

	// Load the configuration from the file
	cfg, err := config.LoadFromPath(filePath)
	if err != nil {
		ctx.Output.Error(fmt.Sprintf("Parse error: %s", err.Error()))
		return nil
	}

	ctx.Output.Status("JSON syntax: OK")

	// Add built-in backends before validation so users can reference them
	cfg.EnsureBuiltinBackends()

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		ctx.Output.Error(fmt.Sprintf("Validation error: %s", err.Error()))
		return nil
	}

	ctx.Output.Status("Configuration: Valid")

	ctx.Output.Println()
	ctx.Output.Success("Configuration file is valid!")
	ctx.Output.Println()

	// Show summary
	ctx.Output.Info("Summary:")
	ctx.Output.Printf("  Mode:     %s\n", GetModeDisplayName(cfg.Route.Mode))
	ctx.Output.Printf("  Backends: %d\n", len(cfg.Backends))
	ctx.Output.Printf("  Tunnels:  %d\n", len(cfg.Tunnels))

	// Show backends
	if len(cfg.Backends) > 0 {
		ctx.Output.Println()
		ctx.Output.Info("Backends:")
		for _, b := range cfg.Backends {
			typeName := config.GetBackendTypeDisplayName(b.Type)
			ctx.Output.Printf("  - %s (%s)\n", b.Tag, typeName)
		}
	}

	// Show tunnels
	if len(cfg.Tunnels) > 0 {
		ctx.Output.Println()
		ctx.Output.Info("Tunnels:")
		for _, t := range cfg.Tunnels {
			transportName := config.GetTransportTypeDisplayName(t.Transport)
			status := "stopped"
			if router.NewTunnel(&t).IsActive() {
				status = "running"
			}
			ctx.Output.Printf("  - %s (%s → %s, %s)\n", t.Tag, transportName, t.Backend, status)
		}
	}

	ctx.Output.Println()

	return nil
}
