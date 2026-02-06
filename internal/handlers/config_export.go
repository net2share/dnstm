package handlers

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/net2share/dnstm/internal/actions"
)

func init() {
	actions.SetConfigHandler(actions.ActionConfigExport, HandleConfigExport)
}

// HandleConfigExport exports the current configuration.
func HandleConfigExport(ctx *actions.Context) error {
	cfg, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	// Marshal to pretty JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Check if output file is specified
	outputFile := ctx.GetString("file")
	if outputFile != "" {
		// Write to file
		if err := os.WriteFile(outputFile, data, 0640); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}

		ctx.Output.Success(fmt.Sprintf("Configuration exported to %s", outputFile))
		return nil
	}

	// Output to stdout
	fmt.Println(string(data))

	return nil
}
