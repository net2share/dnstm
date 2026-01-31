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
	if err := CheckRequirements(ctx, true, true); err != nil {
		return err
	}

	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get values from context (collected by action system)
	backendType := config.BackendType(ctx.GetString("type"))
	if backendType == "" {
		return fmt.Errorf("backend type is required")
	}

	tag := ctx.GetString("tag")
	if tag == "" {
		return fmt.Errorf("backend tag is required")
	}

	// Normalize and validate tag
	tag = router.NormalizeName(tag)
	if err := router.ValidateName(tag); err != nil {
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
			ctx.Output.Printf("Generated password: %s\n", password)
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

	ctx.Output.Success(fmt.Sprintf("Backend '%s' added", tag))

	return nil
}
