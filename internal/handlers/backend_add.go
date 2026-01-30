package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
)

func init() {
	actions.SetBackendHandler(actions.ActionBackendAdd, HandleBackendAdd)
}

// HandleBackendAdd adds a new backend.
func HandleBackendAdd(ctx *actions.Context) error {
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

	// Check if tag already exists
	if cfg.GetBackendByTag(tag) != nil {
		return actions.BackendExistsError(tag)
	}

	// Get backend type
	backendType := config.BackendType(ctx.GetString("type"))
	if backendType == "" {
		return fmt.Errorf("backend type is required")
	}

	// Create backend config
	backend := config.BackendConfig{
		Tag:  tag,
		Type: backendType,
	}

	// Handle type-specific fields
	switch backendType {
	case config.BackendSOCKS, config.BackendSSH, config.BackendCustom:
		address := ctx.GetString("address")
		if address == "" {
			// Provide defaults for built-in types
			switch backendType {
			case config.BackendSOCKS:
				address = "127.0.0.1:1080"
			case config.BackendSSH:
				address = GetDefaultSSHAddress()
			default:
				return fmt.Errorf("address is required for type %s", backendType)
			}
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
		return fmt.Errorf("unknown backend type: %s", backendType)
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
