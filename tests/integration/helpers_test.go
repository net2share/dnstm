package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/service"
	"github.com/spf13/cobra"
)

// TestEnv provides an isolated environment for integration tests.
type TestEnv struct {
	T           *testing.T
	ConfigDir   string
	MockSystemd *service.MockSystemdManager
	OriginalDir string
	Cleanup     []func()
}

// NewTestEnv creates a new test environment.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	// Create temp directories
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "dnstm")

	for _, subdir := range []string{"", "tunnels", "keys", "certs"} {
		if err := os.MkdirAll(filepath.Join(configDir, subdir), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	stateDir := filepath.Join(tmpDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}

	// Create mock systemd manager
	mockSystemd := service.NewMockSystemdManager(stateDir)
	service.SetDefaultManager(mockSystemd)

	env := &TestEnv{
		T:           t,
		ConfigDir:   configDir,
		MockSystemd: mockSystemd,
		Cleanup:     []func(){},
	}

	// Register cleanup
	t.Cleanup(func() {
		service.ResetDefaultManager()
		for _, fn := range env.Cleanup {
			fn()
		}
	})

	return env
}

// WriteConfig writes a config to the test environment.
func (e *TestEnv) WriteConfig(cfg *config.Config) error {
	return cfg.SaveToPath(filepath.Join(e.ConfigDir, "config.json"))
}

// ReadConfig reads the config from the test environment.
func (e *TestEnv) ReadConfig() (*config.Config, error) {
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
			{Tag: "socks", Type: config.BackendSOCKS, Address: "127.0.0.1:1080"},
			{Tag: "ssh", Type: config.BackendSSH, Address: "127.0.0.1:22"},
		},
		Tunnels: []config.TunnelConfig{},
	}
}

// ConfigWithTunnel returns a config with a sample tunnel.
func (e *TestEnv) ConfigWithTunnel() *config.Config {
	cfg := e.DefaultConfig()
	cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{
		Tag:       "test-tunnel",
		Transport: config.TransportSlipstream,
		Backend:   "socks",
		Domain:    "test.example.com",
		Port:      5310,
		Enabled:   boolPtr(true),
	})
	cfg.Route.Active = "test-tunnel"
	return cfg
}

// ActionRunner provides a way to run actions in tests.
type ActionRunner struct {
	env    *TestEnv
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// NewActionRunner creates a new action runner for tests.
func NewActionRunner(env *TestEnv) *ActionRunner {
	return &ActionRunner{
		env:    env,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
}

// RunAction runs an action by ID with the given arguments and values.
func (r *ActionRunner) RunAction(actionID string, args []string, values map[string]interface{}) error {
	action := actions.Get(actionID)
	if action == nil {
		return &ActionNotFoundError{ID: actionID}
	}

	// Build context
	ctx := &actions.Context{
		Ctx:           context.Background(),
		Args:          args,
		Values:        values,
		Output:        &testOutput{stdout: r.stdout, stderr: r.stderr},
		IsInteractive: false,
	}

	// Load config if it exists
	cfg, err := r.env.ReadConfig()
	if err == nil {
		ctx.Config = cfg
	}

	// Run handler
	if action.Handler == nil {
		return nil
	}

	return action.Handler(ctx)
}

// Stdout returns the captured stdout.
func (r *ActionRunner) Stdout() string {
	return r.stdout.String()
}

// Stderr returns the captured stderr.
func (r *ActionRunner) Stderr() string {
	return r.stderr.String()
}

// Reset clears the captured output.
func (r *ActionRunner) Reset() {
	r.stdout.Reset()
	r.stderr.Reset()
}

// ActionNotFoundError is returned when an action is not found.
type ActionNotFoundError struct {
	ID string
}

func (e *ActionNotFoundError) Error() string {
	return "action not found: " + e.ID
}

// testOutput implements the OutputWriter interface for testing.
type testOutput struct {
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func (o *testOutput) Print(msg string) {
	o.stdout.WriteString(msg)
}

func (o *testOutput) Printf(format string, args ...interface{}) {
	o.stdout.WriteString(fmt.Sprintf(format, args...))
}

func (o *testOutput) Println(args ...interface{}) {
	o.stdout.WriteString(fmt.Sprint(args...) + "\n")
}

func (o *testOutput) Success(msg string) {
	o.stdout.WriteString("[SUCCESS] " + msg + "\n")
}

func (o *testOutput) Error(msg string) {
	o.stderr.WriteString("[ERROR] " + msg + "\n")
}

func (o *testOutput) Warning(msg string) {
	o.stdout.WriteString("[WARNING] " + msg + "\n")
}

func (o *testOutput) Info(msg string) {
	o.stdout.WriteString("[INFO] " + msg + "\n")
}

func (o *testOutput) Status(msg string) {
	o.stdout.WriteString("[STATUS] " + msg + "\n")
}

func (o *testOutput) Step(current, total int, msg string) {
	o.stdout.WriteString(fmt.Sprintf("[%d/%d] %s\n", current, total, msg))
}

func (o *testOutput) Box(title string, lines []string) {
	o.stdout.WriteString("=== " + title + " ===\n")
	for _, line := range lines {
		o.stdout.WriteString(line + "\n")
	}
}

func (o *testOutput) KV(key, value string) string {
	return key + ": " + value
}

func (o *testOutput) Table(headers []string, rows [][]string) {
	for _, h := range headers {
		o.stdout.WriteString(h + "\t")
	}
	o.stdout.WriteString("\n")
	for _, row := range rows {
		for _, cell := range row {
			o.stdout.WriteString(cell + "\t")
		}
		o.stdout.WriteString("\n")
	}
}

func (o *testOutput) Separator(length int) {
	for i := 0; i < length; i++ {
		o.stdout.WriteString("-")
	}
	o.stdout.WriteString("\n")
}

func (o *testOutput) BeginProgress(title string) {
	o.stdout.WriteString("[PROGRESS] " + title + "\n")
}

func (o *testOutput) EndProgress() {
	o.stdout.WriteString("[/PROGRESS]\n")
}

func (o *testOutput) DismissProgress() {
	o.stdout.WriteString("[/PROGRESS]\n")
}

func (o *testOutput) IsProgressActive() bool {
	return false
}

func (o *testOutput) ShowInfo(cfg actions.InfoConfig) error {
	o.stdout.WriteString("=== " + cfg.Title + " ===\n")
	if cfg.Description != "" {
		o.stdout.WriteString(cfg.Description + "\n")
	}
	for _, section := range cfg.Sections {
		o.stdout.WriteString("--- " + section.Title + " ---\n")
		for _, row := range section.Rows {
			if row.Key != "" {
				o.stdout.WriteString(row.Key + ": " + row.Value + "\n")
			}
		}
	}
	return nil
}

// CLIRunner runs CLI commands for testing.
type CLIRunner struct {
	env    *TestEnv
	cmd    *cobra.Command
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// NewCLIRunner creates a new CLI runner.
func NewCLIRunner(env *TestEnv, root *cobra.Command) *CLIRunner {
	return &CLIRunner{
		env:    env,
		cmd:    root,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
}

// Run executes a CLI command.
func (r *CLIRunner) Run(args ...string) error {
	r.stdout.Reset()
	r.stderr.Reset()

	r.cmd.SetOut(r.stdout)
	r.cmd.SetErr(r.stderr)
	r.cmd.SetArgs(args)

	return r.cmd.Execute()
}

// Stdout returns the captured stdout.
func (r *CLIRunner) Stdout() string {
	return r.stdout.String()
}

// Stderr returns the captured stderr.
func (r *CLIRunner) Stderr() string {
	return r.stderr.String()
}

func boolPtr(b bool) *bool {
	return &b
}
