package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/service"
)

// TestEnv provides a test environment with isolated config and mock services.
type TestEnv struct {
	T           *testing.T
	ConfigDir   string
	BinDir      string
	MockSystemd *service.MockSystemdManager

	cleanup []func()
}

// NewTestEnv creates a new isolated test environment.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	// Create temp directories
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	binDir := filepath.Join(tmpDir, "bin")
	stateDir := filepath.Join(tmpDir, "state")

	for _, dir := range []string{configDir, binDir, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create subdirectories that config expects
	for _, subdir := range []string{"tunnels", "keys", "certs"} {
		if err := os.MkdirAll(filepath.Join(configDir, subdir), 0755); err != nil {
			t.Fatalf("failed to create subdir %s: %v", subdir, err)
		}
	}

	// Create mock systemd manager
	mockSystemd := service.NewMockSystemdManager(stateDir)

	env := &TestEnv{
		T:           t,
		ConfigDir:   configDir,
		BinDir:      binDir,
		MockSystemd: mockSystemd,
		cleanup:     []func(){},
	}

	// Set the default manager to our mock
	service.SetDefaultManager(mockSystemd)

	// Register cleanup with testing framework
	t.Cleanup(func() {
		service.ResetDefaultManager()
		for _, fn := range env.cleanup {
			fn()
		}
	})

	return env
}

// WriteConfig writes a config to the test environment's config directory.
func (e *TestEnv) WriteConfig(cfg *config.Config) error {
	e.T.Helper()
	return cfg.SaveToPath(filepath.Join(e.ConfigDir, "config.json"))
}

// ReadConfig reads the config from the test environment's config directory.
func (e *TestEnv) ReadConfig() (*config.Config, error) {
	e.T.Helper()
	return config.LoadFromPath(filepath.Join(e.ConfigDir, "config.json"))
}

// DefaultConfig returns a minimal valid config for testing.
func (e *TestEnv) DefaultConfig() *config.Config {
	return &config.Config{
		Listen: config.ListenConfig{
			Address: "127.0.0.1:5353",
		},
		Proxy: config.ProxyConfig{
			Port: 1080,
		},
		Route: config.RouteConfig{
			Mode: "single",
		},
		Backends: []config.BackendConfig{
			{
				Tag:  "builtin-socks",
				Type: config.BackendSOCKS,
			},
		},
		Tunnels: []config.TunnelConfig{},
	}
}

// ConfigWithTunnel returns a config with a sample tunnel configured.
func (e *TestEnv) ConfigWithTunnel() *config.Config {
	port, err := AllocatePort()
	if err != nil {
		e.T.Fatalf("failed to allocate port: %v", err)
	}

	cfg := e.DefaultConfig()
	cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{
		Tag:       "test-tunnel",
		Transport: config.TransportSlipstream,
		Backend:   "builtin-socks",
		Domain:    "test.example.com",
		Port:      port,
		Enabled:   boolPtr(true),
		Slipstream: &config.SlipstreamConfig{
			Cert: filepath.Join(e.ConfigDir, "certs", "test.example.com_cert.pem"),
			Key:  filepath.Join(e.ConfigDir, "certs", "test.example.com_key.pem"),
		},
	})
	return cfg
}

// ConfigWithMultipleTunnels returns a config with multiple tunnels.
func (e *TestEnv) ConfigWithMultipleTunnels(count int) *config.Config {
	cfg := e.DefaultConfig()
	cfg.Route.Mode = "multi"

	for i := 0; i < count; i++ {
		port, err := AllocatePort()
		if err != nil {
			e.T.Fatalf("failed to allocate port: %v", err)
		}

		domain := fmt.Sprintf("test%d.example.com", i)
		cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{
			Tag:       fmt.Sprintf("tunnel-%c", 'a'+i),
			Transport: config.TransportSlipstream,
			Backend:   "builtin-socks",
			Domain:    domain,
			Port:      port,
			Enabled:   boolPtr(true),
			Slipstream: &config.SlipstreamConfig{
				Cert: filepath.Join(e.ConfigDir, "certs", domain+"_cert.pem"),
				Key:  filepath.Join(e.ConfigDir, "certs", domain+"_key.pem"),
			},
		})
	}
	return cfg
}

// Cleanup runs all cleanup functions. Called automatically by t.TempDir().
func (e *TestEnv) Cleanup() {
	for _, fn := range e.cleanup {
		fn()
	}
}

// OnCleanup registers a cleanup function.
func (e *TestEnv) OnCleanup(fn func()) {
	e.cleanup = append(e.cleanup, fn)
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}

// CreateTempFile creates a temporary file with content.
func (e *TestEnv) CreateTempFile(name, content string) string {
	path := filepath.Join(e.ConfigDir, name)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.T.Fatalf("failed to create dir for temp file: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.T.Fatalf("failed to create temp file: %v", err)
	}
	return path
}

// CreateDummyCert creates a dummy cert and key file for testing.
func (e *TestEnv) CreateDummyCert(domain string) (certPath, keyPath string) {
	certPath = filepath.Join(e.ConfigDir, "certs", domain+"_cert.pem")
	keyPath = filepath.Join(e.ConfigDir, "certs", domain+"_key.pem")

	// Write dummy content (not valid certs, but sufficient for config tests)
	os.WriteFile(certPath, []byte("-----BEGIN CERTIFICATE-----\ndummy\n-----END CERTIFICATE-----"), 0644)
	os.WriteFile(keyPath, []byte("-----BEGIN EC PRIVATE KEY-----\ndummy\n-----END EC PRIVATE KEY-----"), 0600)

	return certPath, keyPath
}

// CreateDummyKey creates a dummy DNSTT key file for testing.
func (e *TestEnv) CreateDummyKey(domain string) (pubPath, privPath string) {
	pubPath = filepath.Join(e.ConfigDir, "keys", domain+"_server.pub")
	privPath = filepath.Join(e.ConfigDir, "keys", domain+"_server.key")

	// Write dummy hex keys (64 chars each for Curve25519)
	dummyKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.WriteFile(pubPath, []byte(dummyKey), 0644)
	os.WriteFile(privPath, []byte(dummyKey), 0600)

	return pubPath, privPath
}
