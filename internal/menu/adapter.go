package menu

import (
	"context"
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/handlers"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/go-corelib/tui"
)

// isInfoViewAction checks if an action uses TUI info/progress view and shouldn't require WaitForEnter.
func isInfoViewAction(actionID string) bool {
	switch actionID {
	// Info views
	case actions.ActionRouterStatus, actions.ActionTunnelStatus, actions.ActionBackendStatus, actions.ActionBackendAvailable:
		return true
	// Progress views
	case actions.ActionRouterSwitch, actions.ActionRouterStart, actions.ActionRouterStop,
		actions.ActionRouterMode,
		actions.ActionTunnelAdd, actions.ActionTunnelRemove,
		actions.ActionTunnelStart, actions.ActionTunnelStop, actions.ActionTunnelRestart,
		actions.ActionTunnelEnable, actions.ActionTunnelDisable,
		actions.ActionInstall, actions.ActionUninstall:
		return true
	}
	return false
}

// newActionContext creates a new action context with args.
func newActionContext(args []string) *actions.Context {
	ctx := &actions.Context{
		Ctx:           context.Background(),
		Args:          args,
		Values:        make(map[string]interface{}),
		Output:        handlers.NewTUIOutput(),
		IsInteractive: true,
	}

	// Load config
	if router.IsInitialized() {
		cfg, _ := config.Load()
		ctx.Config = cfg
	}

	return ctx
}

// BuildMenuOptions builds menu options from child actions.
func BuildMenuOptions(parentID string) []tui.MenuOption {
	var options []tui.MenuOption

	// Load config for ShowInMenu checks
	var cfg *config.Config
	if router.IsInitialized() {
		cfg, _ = config.Load()
	}

	children := actions.GetChildren(parentID)
	for _, action := range children {
		// Check ShowInMenu condition
		if action.ShowInMenu != nil {
			ctx := &actions.Context{Config: cfg}
			if !action.ShowInMenu(ctx) {
				continue
			}
		}

		// Skip hidden actions
		if action.Hidden {
			continue
		}

		label := action.MenuLabel
		if label == "" {
			label = action.Short
		}

		// Add arrow for submenus
		if action.IsSubmenu {
			label += " →"
		}

		options = append(options, tui.MenuOption{
			Label: label,
			Value: action.ID,
		})
	}

	return options
}

// RunAction executes an action in interactive mode.
func RunAction(actionID string) error {
	action := actions.Get(actionID)
	if action == nil {
		return fmt.Errorf("unknown action: %s", actionID)
	}

	// Build context
	ctx := &actions.Context{
		Ctx:           context.Background(),
		Values:        make(map[string]interface{}),
		Output:        handlers.NewTUIOutput(),
		IsInteractive: true,
	}

	// Load config
	if router.IsInitialized() {
		cfg, _ := config.Load()
		ctx.Config = cfg
	}

	// Handle argument collection
	if action.Args != nil {
		if action.Args.PickerFunc != nil {
			// Use picker for selection from existing items
			selected, err := runPickerForAction(ctx, action)
			if err != nil {
				if err == actions.ErrCancelled {
					return errCancelled
				}
				return err
			}
			// For pickers, empty selection always means user cancelled (selected Back or pressed ESC)
			if selected == "" {
				return errCancelled
			}
			ctx.Args = []string{selected}
		} else if action.Args.Required {
			// Prompt for text input when no picker is available and arg is required
			value, confirmed, err := tui.RunInput(tui.InputConfig{
				Title:       action.Args.Name,
				Description: action.Args.Description,
			})
			if err != nil {
				return err
			}
			if !confirmed {
				return errCancelled
			}
			if value == "" {
				return fmt.Errorf("%s is required", action.Args.Name)
			}
			ctx.Args = []string{value}
		}
		// For optional args without picker, let the handler deal with it
	}

	// Collect inputs interactively
	for _, input := range action.Inputs {
		// Check ShowIf condition
		if input.ShowIf != nil && !input.ShowIf(ctx) {
			continue
		}

		var value interface{}
		var err error

		switch input.Type {
		case actions.InputTypeText, actions.InputTypePassword:
			var val string
			var confirmed bool
			baseDescription := input.Description
			defaultVal := input.Default
			if input.DefaultFunc != nil {
				defaultVal = input.DefaultFunc(ctx)
			}

			// Loop until valid input or cancel
			var validationErr error
			for {
				description := baseDescription
				if defaultVal != "" && description != "" {
					description = fmt.Sprintf("%s (default: %s)", description, defaultVal)
				} else if defaultVal != "" {
					description = fmt.Sprintf("Default: %s", defaultVal)
				}
				// Show validation error from previous attempt
				if validationErr != nil {
					errMsg := tui.WarnStyle.Render("⚠ " + validationErr.Error())
					if description != "" {
						description = fmt.Sprintf("%s\n%s", description, errMsg)
					} else {
						description = errMsg
					}
				}

				val, confirmed, err = tui.RunInput(tui.InputConfig{
					Title:       input.Label,
					Description: description,
					Placeholder: input.Placeholder,
					Value:       defaultVal,
				})
				if err != nil {
					return err
				}
				if !confirmed {
					return errCancelled
				}
				// Apply default if empty
				if val == "" && defaultVal != "" {
					val = defaultVal
				}
				// Check if required and still empty
				if input.Required && val == "" {
					validationErr = fmt.Errorf("%s is required", input.Label)
					continue
				}
				// Run custom validation if defined
				if input.Validate != nil && val != "" {
					if validationErr = input.Validate(val); validationErr != nil {
						continue
					}
				}
				break
			}
			value = val

		case actions.InputTypeNumber:
			var val string
			var confirmed bool
			baseDescription := input.Description
			defaultVal := input.Default
			if input.DefaultFunc != nil {
				defaultVal = input.DefaultFunc(ctx)
			}

			// Loop until valid input or cancel
			var validationErr error
			for {
				description := baseDescription
				if defaultVal != "" && description != "" {
					description = fmt.Sprintf("%s (default: %s)", description, defaultVal)
				} else if defaultVal != "" {
					description = fmt.Sprintf("Default: %s", defaultVal)
				}
				// Show validation error from previous attempt
				if validationErr != nil {
					errMsg := tui.WarnStyle.Render("⚠ " + validationErr.Error())
					if description != "" {
						description = fmt.Sprintf("%s\n%s", description, errMsg)
					} else {
						description = errMsg
					}
				}

				val, confirmed, err = tui.RunInput(tui.InputConfig{
					Title:       input.Label,
					Description: description,
					Placeholder: defaultVal,
					Value:       defaultVal,
				})
				if err != nil {
					return err
				}
				if !confirmed {
					return errCancelled
				}
				// Apply default if empty
				if val == "" && defaultVal != "" {
					val = defaultVal
				}
				// Check if required and still empty
				if input.Required && val == "" {
					validationErr = fmt.Errorf("%s is required", input.Label)
					continue
				}
				// Run custom validation if defined
				if input.Validate != nil && val != "" {
					if validationErr = input.Validate(val); validationErr != nil {
						continue
					}
				}
				break
			}
			var intVal int
			fmt.Sscanf(val, "%d", &intVal)
			value = intVal

		case actions.InputTypeSelect:
			var tuiOptions []tui.MenuOption
			options := input.Options
			if input.OptionsFunc != nil {
				options = input.OptionsFunc(ctx)
			}
			for _, opt := range options {
				label := opt.Label
				if opt.Recommended {
					label += " (Recommended)"
				}
				tuiOptions = append(tuiOptions, tui.MenuOption{
					Label: label,
					Value: opt.Value,
				})
			}
			// Add back option if not required
			if !input.Required {
				tuiOptions = append(tuiOptions, tui.MenuOption{
					Label: "Skip",
					Value: "",
				})
			}
			// Get description (static or dynamic)
			selectDescription := input.Description
			if input.DescriptionFunc != nil {
				selectDescription = input.DescriptionFunc(ctx)
			}
			val, err := tui.RunMenu(tui.MenuConfig{
				Title:       input.Label,
				Description: selectDescription,
				Options:     tuiOptions,
			})
			if err != nil {
				return err
			}
			// For required fields, empty value means user cancelled (no Skip option available)
			// For optional fields, empty value means user selected Skip
			if val == "" {
				if input.Required {
					return errCancelled
				}
				continue // Skip this optional field
			}
			value = val

		case actions.InputTypeBool:
			// Boolean flags are CLI-only (e.g., --force, --all).
			// In interactive mode, they are skipped as their default is false.
			// For yes/no prompts in interactive mode, use action.Confirm or InputTypeSelect.
			continue
		}

		ctx.Values[input.Name] = value
	}

	// Handle confirmation
	if action.Confirm != nil {
		confirm, err := tui.RunConfirm(tui.ConfirmConfig{
			Title:       action.Confirm.Message,
			Description: action.Confirm.Description,
			Default:     !action.Confirm.DefaultNo,
		})
		if err != nil {
			return err
		}
		if !confirm {
			return errCancelled
		}
	}

	// Run the handler
	if action.Handler == nil {
		return fmt.Errorf("no handler for action %s", action.ID)
	}

	return action.Handler(ctx)
}

// runPickerForAction shows a picker for an action's argument.
func runPickerForAction(ctx *actions.Context, action *actions.Action) (string, error) {
	// Call the picker function to populate options
	_, err := action.Args.PickerFunc(ctx)
	if err != nil {
		return "", err
	}

	// Get options from context using shared helper
	options := actions.GetPickerOptions(ctx)
	if len(options) == 0 {
		return "", actions.NoTunnelsError()
	}

	// Convert to tui options
	var tuiOptions []tui.MenuOption
	for _, opt := range options {
		tuiOptions = append(tuiOptions, tui.MenuOption{
			Label: opt.Label,
			Value: opt.Value,
		})
	}
	tuiOptions = append(tuiOptions, tui.MenuOption{Label: "Back", Value: ""})

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

// RunSubmenu runs a submenu loop for a parent action.
func RunSubmenu(parentID string) error {
	action := actions.Get(parentID)
	if action == nil {
		return fmt.Errorf("unknown action: %s", parentID)
	}

	for {
		// Build options dynamically based on current state
		var options []tui.MenuOption

		// Load config for dynamic menu building
		var cfg *config.Config
		if router.IsInitialized() {
			cfg, _ = config.Load()
		}

		// For router submenu, build options manually to include mode and switch labels
		if parentID == actions.ActionRouter {
			modeName := "unknown"
			isSingleMode := false
			if cfg != nil {
				modeName = handlers.GetModeDisplayName(cfg.Route.Mode)
				isSingleMode = cfg.IsSingleMode()
			}

			options = append(options, tui.MenuOption{
				Label: fmt.Sprintf("Mode: %s", modeName),
				Value: actions.ActionRouterMode,
			})

			// Switch Active is only relevant in single mode
			if isSingleMode {
				activeLabel := "Switch Active: (none)"
				if cfg != nil && cfg.Route.Active != "" {
					activeLabel = fmt.Sprintf("Switch Active: %s", cfg.Route.Active)
				}
				options = append(options, tui.MenuOption{Label: activeLabel, Value: actions.ActionRouterSwitch})
			}

			options = append(options,
				tui.MenuOption{Label: "Status", Value: actions.ActionRouterStatus},
				tui.MenuOption{Label: "Start/Restart", Value: actions.ActionRouterStart},
				tui.MenuOption{Label: "Stop", Value: actions.ActionRouterStop},
				tui.MenuOption{Label: "Logs", Value: actions.ActionRouterLogs},
			)
		} else {
			options = BuildMenuOptions(parentID)
		}

		options = append(options, tui.MenuOption{Label: "Back", Value: "back"})

		title := action.MenuLabel
		if title == "" {
			title = action.Short
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:   title,
			Options: options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		// Check if choice is a submenu
		childAction := actions.Get(choice)
		if childAction != nil && childAction.IsSubmenu {
			if err := RunSubmenu(choice); err != errCancelled {
				tui.WaitForEnter()
			}
			continue
		}

		// Run the action
		if err := RunAction(choice); err != nil {
			if err == errCancelled {
				continue
			}
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
		} else {
			// Skip WaitForEnter for actions that use TUI info view
			if !isInfoViewAction(choice) {
				tui.WaitForEnter()
			}
		}
	}
}
