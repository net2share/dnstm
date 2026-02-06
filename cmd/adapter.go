package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/handlers"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

// BuildCobraCommand builds a Cobra command from an action.
func BuildCobraCommand(action *actions.Action) *cobra.Command {
	cmd := &cobra.Command{
		Use:    action.Use,
		Short:  action.Short,
		Long:   action.Long,
		Hidden: action.Hidden,
	}

	// Add flags for inputs
	for _, input := range action.Inputs {
		switch input.Type {
		case actions.InputTypeText, actions.InputTypePassword:
			if input.ShortFlag != 0 {
				cmd.Flags().StringP(input.Name, string(input.ShortFlag), input.Default, input.Label)
			} else {
				cmd.Flags().String(input.Name, input.Default, input.Label)
			}
		case actions.InputTypeNumber:
			defaultVal := 0
			if input.Default != "" {
				if v, err := strconv.Atoi(input.Default); err == nil {
					defaultVal = v
				}
			}
			if input.ShortFlag != 0 {
				cmd.Flags().IntP(input.Name, string(input.ShortFlag), defaultVal, input.Label)
			} else {
				cmd.Flags().Int(input.Name, defaultVal, input.Label)
			}
		case actions.InputTypeSelect:
			if input.ShortFlag != 0 {
				cmd.Flags().StringP(input.Name, string(input.ShortFlag), input.Default, input.Label)
			} else {
				cmd.Flags().String(input.Name, input.Default, input.Label)
			}
		case actions.InputTypeBool:
			// Boolean flags are CLI-only (not shown in interactive mode)
			cmd.Flags().Bool(input.Name, false, input.Label)
		}
	}

	// Handle confirmation flag
	if action.Confirm != nil && action.Confirm.ForceFlag != "" {
		cmd.Flags().BoolP(action.Confirm.ForceFlag, "f", false, "Skip confirmation")
	}

	// If this is a submenu (parent action), just set up the command without a RunE
	if action.IsSubmenu {
		return cmd
	}

	// Set up the run function
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Check root requirement
		if action.RequiresRoot {
			if err := osdetect.RequireRoot(); err != nil {
				return err
			}
		}

		// Check installed requirement
		if action.RequiresInstalled {
			if err := requireInstalled(); err != nil {
				return err
			}
		}

		// Build context
		ctx := &actions.Context{
			Ctx:           context.Background(),
			Args:          args,
			Values:        make(map[string]interface{}),
			Output:        handlers.NewTUIOutput(),
			IsInteractive: false,
		}

		// Load config if needed
		if router.IsInitialized() {
			cfg, _ := router.Load()
			ctx.Config = cfg
		}

		// Collect values from flags
		for _, input := range action.Inputs {
			switch input.Type {
			case actions.InputTypeText, actions.InputTypePassword, actions.InputTypeSelect:
				val, _ := cmd.Flags().GetString(input.Name)
				ctx.Values[input.Name] = val
			case actions.InputTypeNumber:
				val, _ := cmd.Flags().GetInt(input.Name)
				ctx.Values[input.Name] = val
			case actions.InputTypeBool:
				val, _ := cmd.Flags().GetBool(input.Name)
				ctx.Values[input.Name] = val
			}
		}

		// Handle confirmation flag
		if action.Confirm != nil && action.Confirm.ForceFlag != "" {
			force, _ := cmd.Flags().GetBool(action.Confirm.ForceFlag)
			ctx.Values[action.Confirm.ForceFlag] = force
		}

		// Handle required argument with picker fallback
		if action.Args != nil && action.Args.Required && len(args) == 0 {
			if action.Args.PickerFunc != nil {
				// Show picker for interactive mode
				selected, err := runPicker(ctx, action)
				if err != nil {
					return err
				}
				if selected == "" {
					tui.PrintInfo("Cancelled")
					return nil
				}
				ctx.Args = []string{selected}
			} else {
				return fmt.Errorf("%s required", action.Args.Name)
			}
		}

		// Handle confirmation
		if action.Confirm != nil {
			force := ctx.GetBool(action.Confirm.ForceFlag)
			if !force {
				confirm, err := tui.RunConfirm(tui.ConfirmConfig{
					Title:       action.Confirm.Message,
					Description: action.Confirm.Description,
					Default:     !action.Confirm.DefaultNo,
				})
				if err != nil {
					return err
				}
				if !confirm {
					tui.PrintInfo("Cancelled")
					return nil
				}
			}
		}

		// Run the handler
		if action.Handler == nil {
			return fmt.Errorf("no handler for action %s", action.ID)
		}

		return action.Handler(ctx)
	}

	return cmd
}

// runPicker shows an interactive picker for selecting an instance.
func runPicker(ctx *actions.Context, action *actions.Action) (string, error) {
	// Call the picker function to populate options
	_, err := action.Args.PickerFunc(ctx)
	if err != nil {
		return "", err
	}

	// Get options from context using shared helper
	options := actions.GetPickerOptions(ctx)
	if len(options) == 0 {
		return "", fmt.Errorf("no options available")
	}

	// Convert to tui options
	var tuiOptions []tui.MenuOption
	for _, opt := range options {
		tuiOptions = append(tuiOptions, tui.MenuOption{
			Label: opt.Label,
			Value: opt.Value,
		})
	}

	// Show picker
	selected, err := tui.RunMenu(tui.MenuConfig{
		Title:   fmt.Sprintf("Select %s", action.Args.Name),
		Options: tuiOptions,
	})
	if err != nil {
		return "", err
	}

	return selected, nil
}

// BuildAllCommands builds all Cobra commands from registered actions.
func BuildAllCommands() []*cobra.Command {
	var commands []*cobra.Command

	// Build top-level commands
	for _, action := range actions.TopLevel() {
		cmd := BuildCobraCommand(action)

		// Add child commands
		for _, child := range actions.GetChildren(action.ID) {
			childCmd := BuildCobraCommand(child)
			cmd.AddCommand(childCmd)
		}

		commands = append(commands, cmd)
	}

	return commands
}

// RegisterActionsWithRoot adds all action-based commands to a root command.
func RegisterActionsWithRoot(root *cobra.Command) {
	for _, cmd := range BuildAllCommands() {
		root.AddCommand(cmd)
	}
}
