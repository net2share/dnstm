package actions

func init() {
	// Register router parent action (submenu)
	Register(&Action{
		ID:                ActionRouter,
		Use:               "router",
		Short:             "Manage DNS tunnel router",
		Long:              "Manage the multi-transport DNS router",
		MenuLabel:         "Router",
		IsSubmenu:         true,
		RequiresInstalled: true,
	})

	// Register router.status action
	Register(&Action{
		ID:                ActionRouterStatus,
		Parent:            ActionRouter,
		Use:               "status",
		Short:             "Show router status",
		Long:              "Show the status of the router, DNS router, and all tunnels",
		MenuLabel:         "Status",
		RequiresRoot:      true,
		RequiresInstalled: true,
	})

	// Register router.start action
	Register(&Action{
		ID:                ActionRouterStart,
		Parent:            ActionRouter,
		Use:               "start",
		Short:             "Start the router",
		Long:              "Start or restart tunnels based on current mode.\n\nIf already running, restarts to pick up any configuration changes.\n\nIn single-tunnel mode: starts the active tunnel.\nIn multi-tunnel mode: starts DNS router and all enabled tunnels.",
		MenuLabel:         "Start/Restart",
		RequiresRoot:      true,
		RequiresInstalled: true,
	})

	// Register router.stop action
	Register(&Action{
		ID:                ActionRouterStop,
		Parent:            ActionRouter,
		Use:               "stop",
		Short:             "Stop the router",
		Long:              "Stop tunnels based on current mode.\n\nIn single-tunnel mode: stops the active tunnel.\nIn multi-tunnel mode: stops DNS router and all tunnels.",
		MenuLabel:         "Stop",
		RequiresRoot:      true,
		RequiresInstalled: true,
	})

	// Register router.restart action
	Register(&Action{
		ID:                ActionRouterRestart,
		Parent:            ActionRouter,
		Use:               "restart",
		Short:             "Restart the router",
		Long:              "Restart all tunnels based on current mode.",
		MenuLabel:         "Restart",
		RequiresRoot:      true,
		RequiresInstalled: true,
	})

	// Register router.logs action
	Register(&Action{
		ID:                ActionRouterLogs,
		Parent:            ActionRouter,
		Use:               "logs",
		Short:             "Show router logs",
		Long:              "Show recent logs from DNS router",
		MenuLabel:         "Logs",
		RequiresRoot:      true,
		RequiresInstalled: true,
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

	// Register router.mode action
	Register(&Action{
		ID:                ActionRouterMode,
		Parent:            ActionRouter,
		Use:               "mode [single|multi]",
		Short:             "Show or set operating mode",
		Long:              "Show or set the operating mode of dnstm.\n\nModes:\n  single  Single-tunnel mode with direct port 53 binding\n  multi   Multi-tunnel mode with DNS router\n\nWithout arguments, shows the current mode.",
		MenuLabel:         "Mode",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Inputs: []InputField{
			{
				Name:     "mode",
				Label:    "Operating Mode",
				Type:     InputTypeSelect,
				Required: true,
				Options:  OperatingModeOptions(),
			},
		},
	})

	// Register router.switch action
	Register(&Action{
		ID:                ActionRouterSwitch,
		Parent:            ActionRouter,
		Use:               "switch",
		Short:             "Switch active tunnel",
		Long:              "Switch the active tunnel in single-tunnel mode.\n\nUse --tag/-t to specify the tunnel, or run without flags for an interactive picker.\n\nThis command is only available in single-tunnel mode.\nUse 'dnstm router mode single' to switch to single-tunnel mode first.",
		MenuLabel:         "Switch Active",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag to switch to",
			Required:    false,
			PickerFunc:  TunnelPicker,
		},
		ShowInMenu: func(ctx *Context) bool {
			// Only show in single mode
			return ctx.Config != nil && ctx.Config.IsSingleMode()
		},
	})
}

// SetRouterHandler sets the handler for a router action.
func SetRouterHandler(actionID string, handler Handler) {
	SetHandler(actionID, handler)
}
