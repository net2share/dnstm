package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
)

func init() {
	actions.SetBackendHandler(actions.ActionBackendStatus, HandleBackendStatus)
}

// HandleBackendStatus shows backend status and configuration.
func HandleBackendStatus(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get tag from args
	tag := ctx.GetArg(0)
	if tag == "" {
		return fmt.Errorf("backend tag is required")
	}

	// Get backend
	backend := cfg.GetBackendByTag(tag)
	if backend == nil {
		return actions.BackendNotFoundError(tag)
	}

	// Get tunnels using this backend
	tunnelsUsing := cfg.GetTunnelsUsingBackend(tag)

	// Build info config
	infoCfg := actions.InfoConfig{
		Title: fmt.Sprintf("Backend: %s", tag),
	}

	// Main info section
	mainSection := actions.InfoSection{
		Rows: []actions.InfoRow{
			{Key: "Type", Value: config.GetBackendTypeDisplayName(backend.Type)},
			{Key: "Address", Value: getBackendAddress(backend)},
			{Key: "Category", Value: getBackendCategory(backend)},
			{Key: "Removable", Value: fmt.Sprintf("%v", !backend.IsBuiltIn() || (tag != "socks" && tag != "ssh"))},
		},
	}
	infoCfg.Sections = append(infoCfg.Sections, mainSection)

	// Show shadowsocks config if applicable
	if backend.Shadowsocks != nil {
		ssSection := actions.InfoSection{
			Title: "Shadowsocks Configuration",
			Rows: []actions.InfoRow{
				{Key: "Method", Value: backend.Shadowsocks.Method},
				{Key: "Password", Value: backend.Shadowsocks.Password},
			},
		}
		infoCfg.Sections = append(infoCfg.Sections, ssSection)
	}

	// Show tunnels using this backend
	tunnelSection := actions.InfoSection{
		Title: fmt.Sprintf("Tunnels Using This Backend (%d)", len(tunnelsUsing)),
	}
	if len(tunnelsUsing) == 0 {
		tunnelSection.Rows = []actions.InfoRow{{Value: "None"}}
	} else {
		for _, t := range tunnelsUsing {
			status := "disabled"
			if t.IsEnabled() {
				status = "enabled"
			}
			tunnelSection.Rows = append(tunnelSection.Rows, actions.InfoRow{
				Value: fmt.Sprintf("• %s (%s, %s)", t.Tag, t.Domain, status),
			})
		}
	}
	infoCfg.Sections = append(infoCfg.Sections, tunnelSection)

	// Display using TUI in interactive mode
	if ctx.IsInteractive {
		return ctx.Output.ShowInfo(infoCfg)
	}

	// CLI mode - print to console
	ctx.Output.Println()
	ctx.Output.Box(fmt.Sprintf("Backend: %s", tag), []string{
		ctx.Output.KV("Type", config.GetBackendTypeDisplayName(backend.Type)),
		ctx.Output.KV("Address", getBackendAddress(backend)),
		ctx.Output.KV("Category", getBackendCategory(backend)),
		ctx.Output.KV("Removable", fmt.Sprintf("%v", !backend.IsBuiltIn() || (tag != "socks" && tag != "ssh"))),
	})

	if backend.Shadowsocks != nil {
		ctx.Output.Println()
		ctx.Output.Println("Shadowsocks Configuration:")
		ctx.Output.Printf("  Method:   %s\n", backend.Shadowsocks.Method)
		ctx.Output.Printf("  Password: %s\n", backend.Shadowsocks.Password)
	}

	ctx.Output.Println()
	if len(tunnelsUsing) == 0 {
		ctx.Output.Println("No tunnels using this backend")
	} else {
		ctx.Output.Printf("Tunnels using this backend (%d):\n", len(tunnelsUsing))
		for _, t := range tunnelsUsing {
			status := "disabled"
			if t.IsEnabled() {
				status = "enabled"
			}
			ctx.Output.Printf("  - %s (%s, %s)\n", t.Tag, t.Domain, status)
		}
	}
	ctx.Output.Println()

	return nil
}

func getBackendAddress(b *config.BackendConfig) string {
	if b.Type == config.BackendShadowsocks {
		return "[SIP003 plugin mode]"
	}
	return b.Address
}

func getBackendCategory(b *config.BackendConfig) string {
	info := config.GetBackendTypeInfo(b.Type)
	if info == nil {
		return "unknown"
	}
	switch info.Category {
	case config.CategoryBuiltIn:
		return "Built-in (managed)"
	case config.CategorySystem:
		return "System (external)"
	case config.CategoryCustom:
		return "Custom (external)"
	default:
		return string(info.Category)
	}
}
