// Package handlers provides the business logic for dnstm actions.
package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/go-corelib/osdetect"
)

// CheckRequirements validates common action requirements.
func CheckRequirements(ctx *actions.Context, requireInstalled, requireInitialized bool) error {
	if requireInstalled && !transport.IsInstalled() {
		missing := transport.GetMissingBinaries()
		return actions.NotInstalledError(missing)
	}

	if requireInitialized && !config.ConfigExists() {
		return actions.NotInitializedError()
	}

	return nil
}

// LoadConfig loads and caches the configuration.
func LoadConfig(ctx *actions.Context) (*config.Config, error) {
	if ctx.Config != nil {
		return ctx.Config, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	ctx.Config = cfg
	return cfg, nil
}

// GetTunnelByTag retrieves a tunnel by tag from the config.
func GetTunnelByTag(ctx *actions.Context, tag string) (*config.TunnelConfig, error) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return nil, err
	}

	tunnel := cfg.GetTunnelByTag(tag)
	if tunnel == nil {
		return nil, actions.TunnelNotFoundError(tag)
	}

	return tunnel, nil
}

// GetBackendByTag retrieves a backend by tag from the config.
func GetBackendByTag(ctx *actions.Context, tag string) (*config.BackendConfig, error) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return nil, err
	}

	backend := cfg.GetBackendByTag(tag)
	if backend == nil {
		return nil, actions.BackendNotFoundError(tag)
	}

	return backend, nil
}

// RequireSingleMode returns an error if not in single mode.
func RequireSingleMode(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return err
	}

	if !cfg.IsSingleMode() {
		return actions.SingleModeOnlyError()
	}

	return nil
}

// RequireTunnels returns an error if no tunnels are configured.
func RequireTunnels(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return err
	}

	if len(cfg.Tunnels) == 0 {
		return actions.NoTunnelsError()
	}

	return nil
}

// RequireBackends returns an error if no backends are configured.
func RequireBackends(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return err
	}

	if len(cfg.Backends) == 0 {
		return actions.NoBackendsError()
	}

	return nil
}

// RequireRoot checks for root privileges.
func RequireRoot() error {
	return osdetect.RequireRoot()
}

// GeneratePassword generates a random base64-encoded password.
func GeneratePassword() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

// GetDefaultSSHAddress returns the default SSH server address.
func GetDefaultSSHAddress() string {
	return "127.0.0.1:" + osdetect.DetectSSHPort()
}

// RequireConfig checks installation/initialization requirements and loads config.
func RequireConfig(ctx *actions.Context) (*config.Config, error) {
	if err := CheckRequirements(ctx, true, true); err != nil {
		return nil, err
	}
	return LoadConfig(ctx)
}

// RequireTag gets a tag value from context, returning a standardized error if empty.
func RequireTag(ctx *actions.Context, entity string) (string, error) {
	tag := ctx.GetString("tag")
	if tag == "" {
		return "", actions.NewActionError(
			fmt.Sprintf("%s tag required", entity),
			fmt.Sprintf("Usage: dnstm %s <command> -t <tag>", entity),
		)
	}
	return tag, nil
}

// beginProgress starts a progress view in interactive mode.
func beginProgress(ctx *actions.Context, title string) {
	if ctx.IsInteractive {
		ctx.Output.BeginProgress(title)
	}
}

// endProgress ends a progress view in interactive mode.
func endProgress(ctx *actions.Context) {
	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	}
}

// failProgress shows an error in the progress view and returns the error.
// Unlike the previous pattern, this always returns the error instead of swallowing it.
func failProgress(ctx *actions.Context, err error) error {
	if ctx.IsInteractive {
		ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
		ctx.Output.EndProgress()
	}
	return err
}

// GetModeDisplayName returns a human-readable mode name.
func GetModeDisplayName(mode string) string {
	switch mode {
	case "single":
		return "Single-tunnel"
	case "multi":
		return "Multi-tunnel"
	default:
		return mode
	}
}
