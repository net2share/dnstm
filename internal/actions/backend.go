package actions

import (
	"fmt"

	"github.com/net2share/dnstm/internal/config"
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
	Register(&Action{
		ID:                ActionBackendAdd,
		Parent:            ActionBackend,
		Use:               "add <tag>",
		Short:             "Add a new backend",
		Long:              "Add a new backend service",
		MenuLabel:         "Add",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Backend tag (unique identifier)",
			Required:    true,
		},
		Inputs: []InputField{
			{
				Name:      "type",
				Label:     "Backend Type",
				ShortFlag: 't',
				Type:      InputTypeSelect,
				Required:  true,
				Options:   BackendTypeOptions(),
			},
			{
				Name:      "address",
				Label:     "Address",
				ShortFlag: 'a',
				Type:      InputTypeText,
				ShowIf: func(ctx *Context) bool {
					t := ctx.GetString("type")
					return t == string(config.BackendSOCKS) ||
						t == string(config.BackendSSH) ||
						t == string(config.BackendCustom)
				},
			},
			{
				Name:  "password",
				Label: "Shadowsocks Password",
				Type:  InputTypePassword,
				ShowIf: func(ctx *Context) bool {
					return ctx.GetString("type") == string(config.BackendShadowsocks)
				},
			},
			{
				Name:    "method",
				Label:   "Encryption Method",
				Type:    InputTypeSelect,
				Options: EncryptionMethodOptions(),
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

// BackendTypeOptions returns the available backend type options.
func BackendTypeOptions() []SelectOption {
	return []SelectOption{
		{
			Label:       "SOCKS5",
			Value:       string(config.BackendSOCKS),
			Description: "SOCKS5 proxy (microsocks)",
		},
		{
			Label:       "SSH",
			Value:       string(config.BackendSSH),
			Description: "System SSH server",
		},
		{
			Label:       "Shadowsocks",
			Value:       string(config.BackendShadowsocks),
			Description: "Shadowsocks proxy (SIP003)",
			Recommended: true,
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
