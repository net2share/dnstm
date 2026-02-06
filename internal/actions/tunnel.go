package actions

import (
	"fmt"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/go-corelib/tui"
)

func init() {
	// Register tunnel parent action (submenu)
	Register(&Action{
		ID:                ActionTunnel,
		Use:               "tunnel",
		Short:             "Manage tunnels",
		Long:              "Manage DNS tunnel deployments",
		MenuLabel:         "Tunnels",
		IsSubmenu:         true,
		RequiresInstalled: true,
	})

	// Register tunnel.list action
	Register(&Action{
		ID:                ActionTunnelList,
		Parent:            ActionTunnel,
		Use:               "list",
		Short:             "List all tunnels",
		Long:              "List all configured DNS tunnels",
		MenuLabel:         "List",
		RequiresRoot:      true,
		RequiresInstalled: true,
	})

	// Register tunnel.status action
	Register(&Action{
		ID:                ActionTunnelStatus,
		Parent:            ActionTunnel,
		Use:               "status",
		Short:             "Show tunnel status",
		Long:              "Show status and configuration for a tunnel",
		MenuLabel:         "Status",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// Register tunnel.logs action
	Register(&Action{
		ID:                ActionTunnelLogs,
		Parent:            ActionTunnel,
		Use:               "logs",
		Short:             "Show tunnel logs",
		Long:              "Show recent logs from a tunnel",
		MenuLabel:         "Logs",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
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

	// Register tunnel.start action
	Register(&Action{
		ID:                ActionTunnelStart,
		Parent:            ActionTunnel,
		Use:               "start",
		Short:             "Start a tunnel",
		Long:              "Start or restart a tunnel. If already running, restarts to pick up changes.",
		MenuLabel:         "Start/Restart",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// Register tunnel.stop action
	Register(&Action{
		ID:                ActionTunnelStop,
		Parent:            ActionTunnel,
		Use:               "stop",
		Short:             "Stop a tunnel",
		Long:              "Stop a tunnel",
		MenuLabel:         "Stop",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// Register tunnel.restart action
	Register(&Action{
		ID:                ActionTunnelRestart,
		Parent:            ActionTunnel,
		Use:               "restart",
		Short:             "Restart a tunnel",
		Long:              "Restart a tunnel",
		MenuLabel:         "Restart",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// Register tunnel.enable action
	Register(&Action{
		ID:                ActionTunnelEnable,
		Parent:            ActionTunnel,
		Use:               "enable",
		Short:             "Enable a tunnel",
		Long:              "Enable a tunnel (auto-starts in multi mode)",
		MenuLabel:         "Enable",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// Register tunnel.disable action
	Register(&Action{
		ID:                ActionTunnelDisable,
		Parent:            ActionTunnel,
		Use:               "disable",
		Short:             "Disable a tunnel",
		Long:              "Disable a tunnel (stops it in multi mode)",
		MenuLabel:         "Disable",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// Register tunnel.remove action
	Register(&Action{
		ID:                ActionTunnelRemove,
		Parent:            ActionTunnel,
		Use:               "remove",
		Short:             "Remove a tunnel",
		Long:              "Remove a tunnel and its configuration",
		MenuLabel:         "Remove",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
		Confirm: &ConfirmConfig{
			Message:   "Remove tunnel?",
			DefaultNo: true,
			ForceFlag: "force",
		},
	})

	// Register tunnel.add action
	Register(&Action{
		ID:                ActionTunnelAdd,
		Parent:            ActionTunnel,
		Use:               "add",
		Short:             "Add a new tunnel",
		Long:              "Add a new DNS tunnel interactively or via flags",
		MenuLabel:         "Add",
		RequiresRoot:      true,
		RequiresInstalled: true,
		Inputs: []InputField{
			{
				Name:        "tag",
				Label:       "Tag",
				ShortFlag:   't',
				Type:        InputTypeText,
				Description: "Tunnel tag (auto-generated if omitted)",
				ShowIf:      func(ctx *Context) bool { return !ctx.IsInteractive },
			},
			{
				Name:        "transport",
				Label:       "Transport",
				Type:        InputTypeSelect,
				Required:    true,
				Options:     TransportOptions(),
				Description: "The transport protocol to use",
				ShowIf:      func(ctx *Context) bool { return !ctx.IsInteractive },
			},
			{
				Name:        "backend",
				Label:       "Backend",
				ShortFlag:   'b',
				Type:        InputTypeSelect,
				Required:    true,
				OptionsFunc: BackendOptions,
				DescriptionFunc: func(ctx *Context) string {
					transport := config.TransportType(ctx.GetString("transport"))
					if transport == config.TransportSlipstream {
						cfg, err := config.Load()
						if err == nil {
							hasShadowsocks := false
							for _, b := range cfg.Backends {
								if b.Type == config.BackendShadowsocks {
									hasShadowsocks = true
									break
								}
							}
							if !hasShadowsocks {
								return tui.WarnStyle.Render("⚠ No Shadowsocks backend configured. For best performance with Slipstream, add one via Backends → Add")
							}
						}
						return "Shadowsocks recommended for Slipstream"
					}
					return "The backend to forward traffic to"
				},
				ShowIf: func(ctx *Context) bool { return !ctx.IsInteractive },
			},
			{
				Name:      "domain",
				Label:     "Domain",
				ShortFlag: 'd',
				Type:      InputTypeText,
				Required:  true,
				ShowIf:    func(ctx *Context) bool { return !ctx.IsInteractive },
			},
			{
				Name:        "port",
				Label:       "Port",
				ShortFlag:   'p',
				Type:        InputTypeNumber,
				Description: "Internal port for multi mode (ignored in single mode)",
				DefaultFunc: func(ctx *Context) string {
					cfg, err := config.Load()
					if err != nil {
						return fmt.Sprintf("%d", config.DefaultPortStart)
					}
					port := cfg.AllocateNextPort()
					if port == 0 {
						return fmt.Sprintf("%d", config.DefaultPortStart)
					}
					return fmt.Sprintf("%d", port)
				},
				ShowIf: func(ctx *Context) bool { return !ctx.IsInteractive },
			},
			{
				Name:    "mtu",
				Label:   "MTU",
				Type:    InputTypeNumber,
				Default: "1232",
				ShowIf:  func(ctx *Context) bool { return !ctx.IsInteractive },
			},
		},
	})

}

// TunnelPicker provides interactive tunnel selection.
func TunnelPicker(ctx *Context) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	if len(cfg.Tunnels) == 0 {
		return "", NoTunnelsError()
	}

	var options []SelectOption
	for _, t := range cfg.Tunnels {
		status := SymbolStopped
		if t.IsEnabled() {
			status = SymbolRunning
		}
		transportName := config.GetTransportTypeDisplayName(t.Transport)
		label := fmt.Sprintf("%s %s (%s → %s)", status, t.Tag, transportName, t.Backend)
		options = append(options, SelectOption{
			Label: label,
			Value: t.Tag,
		})
	}

	ctx.Set("_picker_options", options)
	return "", nil
}

// TransportOptions returns the available transport options.
func TransportOptions() []SelectOption {
	return []SelectOption{
		{
			Label:       "Slipstream",
			Value:       string(config.TransportSlipstream),
			Description: "High-performance DNS tunnel with TLS",
		},
		{
			Label:       "DNSTT",
			Value:       string(config.TransportDNSTT),
			Description: "Classic DNS tunnel (dnstt-server)",
		},
	}
}

// BackendOptions returns backend options based on context.
func BackendOptions(ctx *Context) []SelectOption {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	transport := config.TransportType(ctx.GetString("transport"))
	var options []SelectOption

	for _, b := range cfg.Backends {
		// Check compatibility
		if transport == config.TransportDNSTT && b.Type == config.BackendShadowsocks {
			continue // DNSTT doesn't support shadowsocks
		}

		typeName := config.GetBackendTypeDisplayName(b.Type)
		label := fmt.Sprintf("%s (%s)", b.Tag, typeName)

		// Mark recommended backend
		recommended := false
		if transport == config.TransportSlipstream && b.Type == config.BackendShadowsocks {
			recommended = true
		} else if transport == config.TransportDNSTT && b.Type == config.BackendSOCKS {
			recommended = true
		}

		options = append(options, SelectOption{
			Label:       label,
			Value:       b.Tag,
			Recommended: recommended,
		})
	}

	return options
}

// SetTunnelHandler sets the handler for a tunnel action.
func SetTunnelHandler(actionID string, handler Handler) {
	SetHandler(actionID, handler)
}

// NoTunnelsError returns an error indicating no tunnels exist.
func NoTunnelsError() error {
	return fmt.Errorf("no tunnels configured")
}
