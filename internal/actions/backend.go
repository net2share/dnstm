package actions

import (
	"fmt"
	"strings"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	// Register backend parent action (submenu)
	Register(&Action{
		ID:                ActionBackend,
		Use:               "backend",
		Short:             "Manage backends",
		Long:              "Manage backend services (socks, ssh, shadowsocks, custom)",
		MenuLabel:         "Backends",
		IsSubmenu:         true,
		RequiresInstalled: true,
	})

	// Register backend.list action
	Register(&Action{
		ID:                ActionBackendList,
		Parent:            ActionBackend,
		Use:               "list",
		Short:             "List all backends",
		Long:              "List all configured backend services",
		MenuLabel:         "List",
		RequiresRoot:      true,
		RequiresInstalled: true,
	})

	// Register backend.available action
	Register(&Action{
		ID:                ActionBackendAvailable,
		Parent:            ActionBackend,
		Use:               "available",
		Short:             "Show available backend types",
		Long:              "Show all available backend types and their installation status",
		MenuLabel:         "Available Types",
		RequiresRoot:      true,
		RequiresInstalled: true,
	})

	// Register backend.status action
	Register(&Action{
		ID:                ActionBackendStatus,
		Parent:            ActionBackend,
		Use:               "status <tag>",
		Short:             "Show backend status",
		Long:              "Show status and configuration for a backend",
		MenuLabel:         "Status",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Backend tag",
			Required:    true,
			PickerFunc:  BackendPicker,
		},
	})

	// Register backend.add action
	// Inputs are ordered for interactive flow: type → tag → type-specific fields
	Register(&Action{
		ID:                ActionBackendAdd,
		Parent:            ActionBackend,
		Use:               "add",
		Short:             "Add a new backend",
		Long:              "Add a new backend service",
		MenuLabel:         "Add",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Inputs: []InputField{
			{
				Name:        "type",
				Label:       "Backend Type",
				ShortFlag:   't',
				Type:        InputTypeSelect,
				Required:    true,
				Options:     BackendTypeOptions(),
				Description: "Type of backend service",
			},
			{
				Name:        "tag",
				Label:       "Tag",
				ShortFlag:   'n',
				Type:        InputTypeText,
				Required:    true,
				Description: "Unique identifier for this backend",
				Validate: func(value string) error {
					normalized := router.NormalizeName(value)
					if err := router.ValidateName(normalized); err != nil {
						// Replace "name" with "tag" in error message
						return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), "name", "tag"))
					}
					return nil
				},
			},
			{
				Name:        "address",
				Label:       "Address",
				ShortFlag:   'a',
				Type:        InputTypeText,
				Required:    true,
				Description: "Backend address (host:port)",
				ShowIf: func(ctx *Context) bool {
					return ctx.GetString("type") == string(config.BackendCustom)
				},
			},
			{
				Name:        "password",
				Label:       "Password",
				ShortFlag:   'p',
				Type:        InputTypePassword,
				Description: "Shadowsocks password (auto-generated if empty)",
				ShowIf: func(ctx *Context) bool {
					return ctx.GetString("type") == string(config.BackendShadowsocks)
				},
			},
			{
				Name:        "method",
				Label:       "Encryption Method",
				ShortFlag:   'm',
				Type:        InputTypeSelect,
				Options:     EncryptionMethodOptions(),
				Description: "Shadowsocks encryption method",
				ShowIf: func(ctx *Context) bool {
					return ctx.GetString("type") == string(config.BackendShadowsocks)
				},
			},
		},
	})

	// Register backend.remove action
	Register(&Action{
		ID:                ActionBackendRemove,
		Parent:            ActionBackend,
		Use:               "remove <tag>",
		Short:             "Remove a backend",
		Long:              "Remove a backend (fails if in use by tunnels)",
		MenuLabel:         "Remove",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Backend tag",
			Required:    true,
			PickerFunc:  BackendPicker,
		},
		Confirm: &ConfirmConfig{
			Message:   "Remove backend?",
			DefaultNo: true,
			ForceFlag: "force",
		},
	})
}

// BackendPicker provides interactive backend selection.
func BackendPicker(ctx *Context) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	if len(cfg.Backends) == 0 {
		return "", fmt.Errorf("no backends configured")
	}

	var options []SelectOption
	for _, b := range cfg.Backends {
		typeName := config.GetBackendTypeDisplayName(b.Type)
		label := fmt.Sprintf("%s (%s)", b.Tag, typeName)
		if b.IsBuiltIn() {
			label += " [built-in]"
		}
		options = append(options, SelectOption{
			Label: label,
			Value: b.Tag,
		})
	}

	ctx.Set("_picker_options", options)
	return "", nil
}

// BackendTypeOptions returns the available backend type options for adding new backends.
// Note: SOCKS and SSH are built-in backends and cannot be added manually.
func BackendTypeOptions() []SelectOption {
	return []SelectOption{
		{
			Label:       "Shadowsocks (SIP003)",
			Value:       string(config.BackendShadowsocks),
			Description: "Shadowsocks proxy with plugin support",
		},
		{
			Label:       "Custom",
			Value:       string(config.BackendCustom),
			Description: "Custom TCP service",
		},
	}
}

// SetBackendHandler sets the handler for a backend action.
func SetBackendHandler(actionID string, handler Handler) {
	SetHandler(actionID, handler)
}
