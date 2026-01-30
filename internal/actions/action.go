// Package actions provides the unified action system for dnstm CLI and menu.
package actions

import (
	"context"

	"github.com/net2share/dnstm/internal/router"
)

// InputType defines the type of input field.
type InputType int

const (
	// InputTypeText is a text input field.
	InputTypeText InputType = iota
	// InputTypePassword is a password input field (hidden).
	InputTypePassword
	// InputTypeSelect is a single-select dropdown.
	InputTypeSelect
	// InputTypeNumber is a numeric input field.
	InputTypeNumber
	// InputTypeBool is a boolean flag (CLI-only, not shown in interactive mode).
	InputTypeBool
)

// SelectOption defines an option for select inputs.
type SelectOption struct {
	// Label is the display text for this option.
	Label string
	// Value is the value to use when selected.
	Value string
	// Description provides additional context for the option.
	Description string
	// Recommended marks this option as the recommended choice.
	Recommended bool
}

// InputField defines an input field for an action.
type InputField struct {
	// Name is the field identifier (used as flag name and context key).
	Name string
	// Label is the human-readable label for the field.
	Label string
	// Description provides additional context for the field.
	Description string
	// Type is the input field type.
	Type InputType
	// Required indicates if the field must be provided.
	Required bool
	// Default is the default value if not provided.
	Default string
	// Placeholder is shown when the field is empty.
	Placeholder string
	// Options are the available choices for select inputs.
	Options []SelectOption
	// OptionsFunc dynamically generates options based on context.
	OptionsFunc func(ctx *Context) []SelectOption
	// ShortFlag is the single-character flag alias (e.g., 't' for --type).
	ShortFlag rune
	// ShowIf conditionally shows this field based on context.
	ShowIf func(ctx *Context) bool
	// Validate validates the field value.
	Validate func(value string) error
	// DefaultFunc dynamically generates the default value.
	DefaultFunc func(ctx *Context) string
}

// ConfirmConfig defines confirmation settings for an action.
type ConfirmConfig struct {
	// Message is the confirmation prompt.
	Message string
	// Description provides additional context.
	Description string
	// DefaultNo sets the default to "no" (safer).
	DefaultNo bool
	// ForceFlag is the flag name to skip confirmation (e.g., "force").
	ForceFlag string
}

// ArgsSpec defines the positional arguments for an action.
type ArgsSpec struct {
	// Name is the argument name shown in usage.
	Name string
	// Description describes the argument.
	Description string
	// Required indicates if the argument must be provided.
	Required bool
	// PickerFunc provides interactive selection when arg is not provided.
	PickerFunc func(ctx *Context) (string, error)
}

// Handler is the function signature for action handlers.
type Handler func(ctx *Context) error

// Action defines a command/menu action.
type Action struct {
	// ID is the unique action identifier (e.g., "instance.list").
	ID string
	// Parent is the parent action ID (e.g., "instance" for "instance.list").
	Parent string
	// Use is the command usage string (e.g., "list", "add [name]").
	Use string
	// Short is the short description.
	Short string
	// Long is the long description.
	Long string
	// MenuLabel is the label to show in menus (defaults to Short).
	MenuLabel string
	// Args defines positional arguments.
	Args *ArgsSpec
	// Inputs defines the input fields.
	Inputs []InputField
	// Confirm defines confirmation requirements.
	Confirm *ConfirmConfig
	// Handler is the business logic function.
	Handler Handler
	// RequiresRoot indicates if root privileges are required.
	RequiresRoot bool
	// RequiresInstalled indicates if transport binaries must be installed.
	RequiresInstalled bool
	// Hidden hides from CLI help and menu.
	Hidden bool
	// ShowInMenu controls visibility in interactive menu.
	ShowInMenu func(ctx *Context) bool
	// IsSubmenu indicates this is a parent action (submenu).
	IsSubmenu bool
}

// Context provides the execution context for action handlers.
type Context struct {
	// Ctx is the standard Go context.
	Ctx context.Context
	// Config is the loaded router configuration.
	Config *router.Config
	// Args contains positional arguments.
	Args []string
	// Values contains collected input values.
	Values map[string]interface{}
	// Output is the output writer for the action.
	Output OutputWriter
	// IsInteractive indicates if running in interactive mode.
	IsInteractive bool
}

// GetString returns a string value from the context.
func (c *Context) GetString(key string) string {
	if v, ok := c.Values[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt returns an integer value from the context.
func (c *Context) GetInt(key string) int {
	if v, ok := c.Values[key]; ok {
		switch i := v.(type) {
		case int:
			return i
		case int64:
			return int(i)
		case float64:
			return int(i)
		}
	}
	return 0
}

// GetBool returns a boolean value from the context.
func (c *Context) GetBool(key string) bool {
	if v, ok := c.Values[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetArg returns the positional argument at the given index.
func (c *Context) GetArg(index int) string {
	if index >= 0 && index < len(c.Args) {
		return c.Args[index]
	}
	return ""
}

// HasArg returns true if a positional argument exists at the given index.
func (c *Context) HasArg(index int) bool {
	return index >= 0 && index < len(c.Args)
}

// Set sets a value in the context.
func (c *Context) Set(key string, value interface{}) {
	if c.Values == nil {
		c.Values = make(map[string]interface{})
	}
	c.Values[key] = value
}

// Reload reloads the router configuration.
func (c *Context) Reload() error {
	cfg, err := router.Load()
	if err != nil {
		return err
	}
	c.Config = cfg
	return nil
}
