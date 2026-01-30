package actions

func init() {
	// Register config parent action (submenu)
	Register(&Action{
		ID:                ActionConfig,
		Use:               "config",
		Short:             "Manage configuration",
		Long:              "Load, export, and validate configuration files",
		MenuLabel:         "Config",
		IsSubmenu:         true,
		RequiresInstalled: true,
	})

	// Register config.load action
	Register(&Action{
		ID:                ActionConfigLoad,
		Parent:            ActionConfig,
		Use:               "load <file>",
		Short:             "Load configuration from file",
		Long:              "Load and deploy configuration from a JSON file",
		MenuLabel:         "Load",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "file",
			Description: "Path to config.json file",
			Required:    true,
		},
	})

	// Register config.export action
	Register(&Action{
		ID:                ActionConfigExport,
		Parent:            ActionConfig,
		Use:               "export",
		Short:             "Export current configuration",
		Long:              "Export current configuration to stdout or file",
		MenuLabel:         "Export",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Inputs: []InputField{
			{
				Name:        "file",
				Label:       "Output file",
				ShortFlag:   'o',
				Type:        InputTypeText,
				Description: "Optional output file path (stdout if not specified)",
			},
		},
	})

	// Register config.validate action
	Register(&Action{
		ID:                ActionConfigValidate,
		Parent:            ActionConfig,
		Use:               "validate <file>",
		Short:             "Validate configuration file",
		Long:              "Validate a configuration file without deploying",
		MenuLabel:         "Validate",
		RequiresRoot:      false,
		RequiresInstalled: false,
		Args: &ArgsSpec{
			Name:        "file",
			Description: "Path to config.json file",
			Required:    true,
		},
	})
}

// SetConfigHandler sets the handler for a config action.
func SetConfigHandler(actionID string, handler Handler) {
	SetHandler(actionID, handler)
}
