package actions

import (
	"fmt"

	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/types"
)

func init() {
	// Register instance parent action (submenu)
	Register(&Action{
		ID:                ActionInstance,
		Use:               "instance",
		Short:             "Manage transport instances",
		Long:              "Manage individual transport instances in the DNS tunnel router",
		MenuLabel:         "Instances",
		IsSubmenu:         true,
		RequiresInstalled: true,
	})

	// Register instance.list action
	Register(&Action{
		ID:                ActionInstanceList,
		Parent:            ActionInstance,
		Use:               "list",
		Short:             "List all instances",
		Long:              "List all configured transport instances",
		MenuLabel:         "List",
		RequiresRoot:      true,
		RequiresInstalled: true,
		// Handler is set in handlers package via SetHandler
	})

	// Register instance.status action
	Register(&Action{
		ID:                ActionInstanceStatus,
		Parent:            ActionInstance,
		Use:               "status <name>",
		Short:             "Show instance status",
		Long:              "Show status and configuration for a transport instance",
		MenuLabel:         "Status",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "name",
			Description: "Instance name",
			Required:    true,
			PickerFunc:  InstancePicker,
		},
	})

	// Register instance.logs action
	Register(&Action{
		ID:                ActionInstanceLogs,
		Parent:            ActionInstance,
		Use:               "logs <name>",
		Short:             "Show instance logs",
		Long:              "Show recent logs from a transport instance",
		MenuLabel:         "Logs",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "name",
			Description: "Instance name",
			Required:    true,
			PickerFunc:  InstancePicker,
		},
		Inputs: []InputField{
			{
				Name:      "lines",
				Label:     "Number of lines",
				ShortFlag: 'n',
				Type:      InputTypeNumber,
				Default:   "50",
			},
		},
	})

	// Register instance.start action
	Register(&Action{
		ID:                ActionInstanceStart,
		Parent:            ActionInstance,
		Use:               "start <name>",
		Short:             "Start an instance",
		Long:              "Start or restart a transport instance. If already running, restarts to pick up changes.",
		MenuLabel:         "Start/Restart",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "name",
			Description: "Instance name",
			Required:    true,
			PickerFunc:  InstancePicker,
		},
	})

	// Register instance.stop action
	Register(&Action{
		ID:                ActionInstanceStop,
		Parent:            ActionInstance,
		Use:               "stop <name>",
		Short:             "Stop an instance",
		Long:              "Stop a transport instance",
		MenuLabel:         "Stop",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "name",
			Description: "Instance name",
			Required:    true,
			PickerFunc:  InstancePicker,
		},
	})

	// Register instance.remove action
	Register(&Action{
		ID:                ActionInstanceRemove,
		Parent:            ActionInstance,
		Use:               "remove <name>",
		Short:             "Remove an instance",
		Long:              "Remove a transport instance and its configuration",
		MenuLabel:         "Remove",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "name",
			Description: "Instance name",
			Required:    true,
			PickerFunc:  InstancePicker,
		},
		Confirm: &ConfirmConfig{
			Message:   "Remove instance?",
			DefaultNo: true,
			ForceFlag: "force",
		},
	})

	// Register instance.add action
	Register(&Action{
		ID:                ActionInstanceAdd,
		Parent:            ActionInstance,
		Use:               "add [name]",
		Short:             "Add a new instance",
		Long:              "Add a new transport instance interactively or via flags",
		MenuLabel:         "Add",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "name",
			Description: "Instance name (optional, auto-generated if not provided)",
			Required:    false,
		},
		Inputs: []InputField{
			{
				Name:      "type",
				Label:     "Transport Type",
				ShortFlag: 't',
				Type:      InputTypeSelect,
				Required:  true,
				Options:   TransportTypeOptions(),
			},
			{
				Name:      "domain",
				Label:     "Domain",
				ShortFlag: 'd',
				Type:      InputTypeText,
				Required:  true,
			},
			{
				Name:  "password",
				Label: "Shadowsocks Password",
				Type:  InputTypePassword,
				ShowIf: func(ctx *Context) bool {
					return ctx.GetString("type") == string(types.TypeSlipstreamShadowsocks)
				},
			},
			{
				Name:    "method",
				Label:   "Encryption Method",
				Type:    InputTypeSelect,
				Options: EncryptionMethodOptions(),
				ShowIf: func(ctx *Context) bool {
					return ctx.GetString("type") == string(types.TypeSlipstreamShadowsocks)
				},
			},
			{
				Name:  "target",
				Label: "Target Address",
				Type:  InputTypeText,
				ShowIf: func(ctx *Context) bool {
					t := ctx.GetString("type")
					return t == string(types.TypeSlipstreamSSH) || t == string(types.TypeDNSTTSSH)
				},
			},
		},
	})

	// Register instance.reconfigure action
	Register(&Action{
		ID:                ActionInstanceReconfigure,
		Parent:            ActionInstance,
		Use:               "reconfigure <name>",
		Short:             "Reconfigure an instance",
		Long:              "Reconfigure an existing transport instance interactively (including rename)",
		MenuLabel:         "Reconfigure",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "name",
			Description: "Instance name",
			Required:    true,
			PickerFunc:  InstancePicker,
		},
	})
}

// InstancePicker provides interactive instance selection.
func InstancePicker(ctx *Context) (string, error) {
	if ctx.Config == nil {
		cfg, err := router.Load()
		if err != nil {
			return "", err
		}
		ctx.Config = cfg
	}

	if len(ctx.Config.Transports) == 0 {
		return "", NoInstancesError()
	}

	// Build options for picker
	var options []SelectOption
	for name, transport := range ctx.Config.Transports {
		instance := router.NewInstance(name, transport)
		status := SymbolStopped
		if instance.IsActive() {
			status = SymbolRunning
		}
		typeName := types.GetTransportTypeDisplayName(transport.Type)
		label := fmt.Sprintf("%s %s (%s)", status, name, typeName)
		options = append(options, SelectOption{
			Label: label,
			Value: name,
		})
	}

	// The actual picker is implemented by the adapter (CLI or menu)
	// This function returns empty to signal that a picker should be shown
	ctx.Set("_picker_options", options)
	return "", nil
}

// SetInstanceHandler sets the handler for an instance action.
func SetInstanceHandler(actionID string, handler Handler) {
	SetHandler(actionID, handler)
}
