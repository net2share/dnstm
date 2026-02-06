package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
)

func init() {
	actions.SetBackendHandler(actions.ActionBackendAdd, HandleBackendAdd)
}

// HandleBackendAdd adds a new backend.
// Inputs are collected by the action system in order: type → tag → type-specific fields
func HandleBackendAdd(ctx *actions.Context) error {
	cfg, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	// Get values from context (collected by action system)
	backendType := config.BackendType(ctx.GetString("type"))
	if backendType == "" {
		return fmt.Errorf("backend type is required")
	}

	tag := ctx.GetString("tag")
	if tag == "" {
		tag = router.GenerateUniqueBackendTag(cfg.Backends)
	}

	// Normalize and validate tag
	tag = router.NormalizeTag(tag)
	if err := router.ValidateTag(tag); err != nil {
		return fmt.Errorf("invalid tag: %w", err)
	}

	// Check if tag already exists
	if cfg.GetBackendByTag(tag) != nil {
		return actions.BackendExistsError(tag)
	}

	// Create backend config
	backend := config.BackendConfig{
		Tag:  tag,
		Type: backendType,
	}

	// Handle type-specific fields
	// Note: SOCKS and SSH are built-in backends and cannot be added manually
	switch backendType {
	case config.BackendSOCKS, config.BackendSSH:
		return fmt.Errorf("%s backends are built-in and cannot be added manually", backendType)

	case config.BackendCustom:
		address := ctx.GetString("address")
		if address == "" {
			return fmt.Errorf("address is required for custom backend")
		}
		backend.Address = address

	case config.BackendShadowsocks:
		password := ctx.GetString("password")
		if password == "" {
			password = GeneratePassword()
		}

		method := ctx.GetString("method")
		if method == "" {
			method = "aes-256-gcm"
		}

		backend.Shadowsocks = &config.ShadowsocksConfig{
			Password: password,
			Method:   method,
		}

	default:
		return fmt.Errorf("unknown backend type: %s (use 'shadowsocks' or 'custom')", backendType)
	}

	// Add backend to config
	cfg.Backends = append(cfg.Backends, backend)

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Display result
	if ctx.IsInteractive {
		infoCfg := actions.InfoConfig{
			Title: fmt.Sprintf("Backend '%s' added", tag),
		}

		section := actions.InfoSection{
			Rows: []actions.InfoRow{
				{Key: "Type", Value: string(backendType)},
			},
		}

		switch backendType {
		case config.BackendShadowsocks:
			section.Rows = append(section.Rows,
				actions.InfoRow{Key: "Method", Value: backend.Shadowsocks.Method},
				actions.InfoRow{Key: "Password", Value: backend.Shadowsocks.Password},
			)
		case config.BackendCustom:
			section.Rows = append(section.Rows,
				actions.InfoRow{Key: "Address", Value: backend.Address},
			)
		}

		infoCfg.Sections = append(infoCfg.Sections, section)
		return ctx.Output.ShowInfo(infoCfg)
	}

	if backendType == config.BackendShadowsocks && ctx.GetString("password") == "" {
		ctx.Output.Printf("Generated password: %s\n", backend.Shadowsocks.Password)
	}
	ctx.Output.Success(fmt.Sprintf("Backend '%s' added", tag))

	return nil
}
