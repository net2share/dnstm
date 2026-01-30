package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/types"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

func init() {
	actions.SetInstanceHandler(actions.ActionInstanceReconfigure, HandleInstanceReconfigure)
}

// HandleInstanceReconfigure reconfigures an existing instance.
func HandleInstanceReconfigure(ctx *actions.Context) error {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	name := ctx.GetArg(0)
	if name == "" {
		return actions.NewActionError("instance name required", "Usage: dnstm instance reconfigure <name>")
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	transportCfg, exists := cfg.Transports[name]
	if !exists {
		return actions.NotFoundError(name)
	}

	ctx.Output.Println()
	ctx.Output.Info(fmt.Sprintf("Reconfiguring '%s'...", name))
	ctx.Output.Info("Press Enter to keep current value, or type a new value.")
	ctx.Output.Println()

	changed := false
	renamed := false
	newName := name

	// Check if instance is running before we start
	oldInstance := router.NewInstance(name, transportCfg)
	wasRunning := oldInstance.IsActive()

	// Name (rename)
	inputName, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Instance Name",
		Description: fmt.Sprintf("Current: %s", name),
		Value:       name,
	})
	if err != nil {
		return err
	}
	if !confirmed {
		ctx.Output.Info("Cancelled")
		return nil
	}
	if inputName != "" && inputName != name {
		inputName = router.NormalizeName(inputName)
		if err := router.ValidateName(inputName); err != nil {
			return fmt.Errorf("invalid name: %w", err)
		}
		if _, exists := cfg.Transports[inputName]; exists {
			return actions.ExistsError(inputName)
		}
		newName = inputName
		renamed = true
		changed = true
	}

	// Domain
	newDomain, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Domain",
		Description: fmt.Sprintf("Current: %s", transportCfg.Domain),
		Value:       transportCfg.Domain,
	})
	if err != nil {
		return err
	}
	if !confirmed {
		ctx.Output.Info("Cancelled")
		return nil
	}
	if newDomain != "" && newDomain != transportCfg.Domain {
		transportCfg.Domain = newDomain
		changed = true
	}

	// Type-specific configuration
	switch transportCfg.Type {
	case types.TypeSlipstreamShadowsocks:
		// Password
		newPassword, confirmed, err := tui.RunInput(tui.InputConfig{
			Title:       "Password",
			Description: "Current: (hidden) - Leave empty to keep current",
		})
		if err != nil {
			return err
		}
		if !confirmed {
			ctx.Output.Info("Cancelled")
			return nil
		}
		if newPassword != "" {
			if transportCfg.Shadowsocks == nil {
				transportCfg.Shadowsocks = &types.ShadowsocksConfig{}
			}
			transportCfg.Shadowsocks.Password = newPassword
			changed = true
		}

		// Method
		currentMethod := "aes-256-gcm"
		if transportCfg.Shadowsocks != nil && transportCfg.Shadowsocks.Method != "" {
			currentMethod = transportCfg.Shadowsocks.Method
		}
		methodOptions := []tui.MenuOption{
			{Label: "AES-256-GCM", Value: "aes-256-gcm"},
			{Label: "ChaCha20-IETF-Poly1305", Value: "chacha20-ietf-poly1305"},
		}
		selected := 0
		if currentMethod == "chacha20-ietf-poly1305" {
			selected = 1
		}
		newMethod, err := tui.RunMenu(tui.MenuConfig{
			Title:       "Method",
			Description: fmt.Sprintf("Current: %s", currentMethod),
			Options:     methodOptions,
			Selected:    selected,
		})
		if err != nil {
			return err
		}
		if newMethod == "" {
			ctx.Output.Info("Cancelled")
			return nil
		}
		if newMethod != currentMethod {
			if transportCfg.Shadowsocks == nil {
				transportCfg.Shadowsocks = &types.ShadowsocksConfig{}
			}
			transportCfg.Shadowsocks.Method = newMethod
			changed = true
		}

	case types.TypeSlipstreamSocks, types.TypeDNSTTSocks:
		// SOCKS modes auto-use microsocks
		microsocksAddr := cfg.GetMicrosocksAddress()
		if microsocksAddr != "" && (transportCfg.Target == nil || transportCfg.Target.Address != microsocksAddr) {
			if transportCfg.Target == nil {
				transportCfg.Target = &types.TargetConfig{}
			}
			transportCfg.Target.Address = microsocksAddr
			changed = true
		}

	case types.TypeSlipstreamSSH, types.TypeDNSTTSSH:
		// SSH modes - allow changing target
		currentTarget := "127.0.0.1:" + osdetect.DetectSSHPort()
		if transportCfg.Target != nil && transportCfg.Target.Address != "" {
			currentTarget = transportCfg.Target.Address
		}
		newTarget, confirmed, err := tui.RunInput(tui.InputConfig{
			Title:       "Target Address",
			Description: fmt.Sprintf("Current: %s", currentTarget),
			Value:       currentTarget,
		})
		if err != nil {
			return err
		}
		if !confirmed {
			ctx.Output.Info("Cancelled")
			return nil
		}
		if newTarget != "" && newTarget != currentTarget {
			if transportCfg.Target == nil {
				transportCfg.Target = &types.TargetConfig{}
			}
			transportCfg.Target.Address = newTarget
			changed = true
		}
	}

	if !changed {
		ctx.Output.Info("No changes made")
		return nil
	}

	// Handle rename
	if renamed {
		// Stop and remove old service
		oldInstance.Stop()
		oldInstance.RemoveService()
		oldInstance.RemoveConfigDir()

		// Update config map
		delete(cfg.Transports, name)
		cfg.Transports[newName] = transportCfg

		// Update Single.Active if it referenced the old name
		if cfg.Single.Active == name {
			cfg.Single.Active = newName
		}
		// Update Routing.Default if it referenced the old name
		if cfg.Routing.Default == name {
			cfg.Routing.Default = newName
		}
	}

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create new service with appropriate mode
	newInstance := router.NewInstance(newName, transportCfg)

	// Determine service mode based on current router mode
	serviceMode := router.ServiceModeMulti
	if cfg.IsSingleMode() {
		// Is this the active instance?
		isActive := cfg.Single.Active == newName
		if isActive {
			serviceMode = router.ServiceModeSingle
		}
	}

	if err := newInstance.CreateServiceWithMode(serviceMode); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	if err := newInstance.SetPermissions(); err != nil {
		ctx.Output.Warning("Permission warning: " + err.Error())
	}

	// Start if it was running before
	if wasRunning {
		if err := newInstance.Enable(); err != nil {
			ctx.Output.Warning("Failed to enable service: " + err.Error())
		}
		if err := newInstance.Start(); err != nil {
			return fmt.Errorf("failed to start: %w", err)
		}
		if renamed {
			ctx.Output.Success(fmt.Sprintf("Instance renamed to '%s' and restarted!", newName))
		} else {
			ctx.Output.Success(fmt.Sprintf("Instance '%s' reconfigured and restarted!", newName))
		}
	} else {
		if renamed {
			ctx.Output.Success(fmt.Sprintf("Instance renamed to '%s'!", newName))
		} else {
			ctx.Output.Success(fmt.Sprintf("Instance '%s' reconfigured!", newName))
		}
	}

	return nil
}
