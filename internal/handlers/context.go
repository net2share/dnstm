// Package handlers provides the business logic for dnstm actions.
package handlers

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/go-corelib/osdetect"
)

// CheckRequirements validates common action requirements.
func CheckRequirements(ctx *actions.Context, requireInstalled, requireInitialized bool) error {
	if requireInstalled && !transport.IsInstalled() {
		missing := transport.GetMissingBinaries()
		return actions.NotInstalledError(missing)
	}

	if requireInitialized && !router.IsInitialized() {
		return actions.NotInitializedError()
	}

	return nil
}

// LoadConfig loads and caches the router configuration.
func LoadConfig(ctx *actions.Context) (*router.Config, error) {
	if ctx.Config != nil {
		return ctx.Config, nil
	}

	cfg, err := router.Load()
	if err != nil {
		return nil, err
	}
	ctx.Config = cfg
	return cfg, nil
}

// GetInstanceByName retrieves an instance by name from the config.
func GetInstanceByName(ctx *actions.Context, name string) (*router.Instance, error) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return nil, err
	}

	transportCfg, exists := cfg.Transports[name]
	if !exists {
		return nil, actions.NotFoundError(name)
	}

	return router.NewInstance(name, transportCfg), nil
}

// GetAllInstances returns all configured instances.
func GetAllInstances(ctx *actions.Context) (map[string]*router.Instance, error) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return nil, err
	}

	instances := make(map[string]*router.Instance)
	for name, transportCfg := range cfg.Transports {
		instances[name] = router.NewInstance(name, transportCfg)
	}

	return instances, nil
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

// RequireInstances returns an error if no instances are configured.
func RequireInstances(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return err
	}

	if len(cfg.Transports) == 0 {
		return actions.NoInstancesError()
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
