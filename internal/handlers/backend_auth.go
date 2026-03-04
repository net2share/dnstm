package handlers

import (
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/proxy"
)

func init() {
	actions.SetBackendHandler(actions.ActionBackendAuth, HandleBackendAuth)
}

// HandleBackendAuth enables, disables, or changes SOCKS5 authentication.
func HandleBackendAuth(ctx *actions.Context) error {
	cfg, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	tag, err := RequireTag(ctx, "backend")
	if err != nil {
		return err
	}

	backend := cfg.GetBackendByTag(tag)
	if backend == nil {
		return actions.BackendNotFoundError(tag)
	}

	if backend.Type != config.BackendSOCKS {
		return fmt.Errorf("backend '%s' is not a SOCKS backend", tag)
	}

	disable := ctx.GetBool("disable")

	if disable {
		backend.Socks = nil
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		if err := proxy.ReconfigureMicrosocks(cfg.Proxy.Port, "", ""); err != nil {
			return fmt.Errorf("failed to reconfigure microsocks: %w", err)
		}

		ctx.Output.Success("SOCKS5 authentication disabled")
		return nil
	}

	user := ctx.GetString("user")
	password := ctx.GetString("password")

	if user == "" || password == "" {
		return fmt.Errorf("both user and password are required to enable authentication")
	}

	backend.Socks = &config.SocksConfig{
		User:     user,
		Password: password,
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if err := proxy.ReconfigureMicrosocks(cfg.Proxy.Port, user, password); err != nil {
		return fmt.Errorf("failed to reconfigure microsocks: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("SOCKS5 authentication enabled (user: %s)", user))
	return nil
}
