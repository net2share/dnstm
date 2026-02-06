# Development Guide

This document describes development practices for dnstm.

For setting up the testing environment and running tests, see the [Testing Guide](TESTING.md).

## Binary Version Pinning

Transport binaries (slipstream-server, ssserver, microsocks, sshtun-user) have their versions pinned in the codebase. Each dnstm release specifies exact binary versions it expects.

### How It Works

Binary definitions in `internal/binary/binary.go` include a `PinnedVersion` field:

```go
BinarySlipstreamServer: {
    Type:          BinarySlipstreamServer,
    URLPattern:    "https://github.com/.../releases/download/{version}/slipstream-server-{os}-{arch}",
    PinnedVersion: "v2026.02.05",
    // ...
},
```

The update system compares installed versions (from `/etc/dnstm/versions.json`) against these pinned versions. Only dnstm's own version is checked via GitHub API.

### Updating Binary Versions

To update a transport binary version:

1. Update the `PinnedVersion` in `internal/binary/binary.go`
2. Update the test expectations in `internal/binary/binary_test.go` if URL patterns changed
3. Release a new dnstm version

Users running `dnstm update` will then receive the new binary versions.

### Version Manifest

The manifest at `/etc/dnstm/versions.json` tracks installed binary versions:

```json
{
  "slipstream-server": "v2026.02.05",
  "ssserver": "v1.24.0",
  "microsocks": "v1.0.5",
  "sshtun-user": "v0.3.4",
  "updated_at": "2026-02-06T10:00:00Z"
}
```

This manifest is created during `dnstm install` and updated after each binary update.

## Building

```bash
# Build the binary (includes version info from git)
make build

# Build and show version
make dev-build

# Install to /usr/local/bin
make install
```

## Code Style

```bash
# Format code
make fmt

# Run linter
make lint

# Check for issues (format + lint + unit tests)
make check
```

## Writing Tests

### Unit Test Example

```go
func TestValidatePort(t *testing.T) {
    tests := []struct {
        port    int
        wantErr bool
    }{
        {port: 5310, wantErr: false},
        {port: 80, wantErr: true},
    }

    for _, tt := range tests {
        t.Run(fmt.Sprintf("port_%d", tt.port), func(t *testing.T) {
            err := ValidatePort(tt.port)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidatePort(%d) error = %v, wantErr %v",
                    tt.port, err, tt.wantErr)
            }
        })
    }
}
```

### Integration Test Example

```go
func TestBackendAdd(t *testing.T) {
    env := NewTestEnv(t)

    cfg := env.DefaultConfig()
    cfg.Backends = append(cfg.Backends, config.BackendConfig{
        Tag:     "my-backend",
        Type:    config.BackendCustom,
        Address: "192.168.1.1:8080",
    })

    if err := env.WriteConfig(cfg); err != nil {
        t.Fatalf("failed to write config: %v", err)
    }

    loaded, err := env.ReadConfig()
    if err != nil {
        t.Fatalf("failed to read config: %v", err)
    }

    if loaded.GetBackendByTag("my-backend") == nil {
        t.Error("backend not found")
    }
}
```

### E2E Test Example

```go
func TestSlipstream_LocalMode(t *testing.T) {
    env := NewE2EEnv(t)

    // Generate certificate
    certPath, keyPath := generateCert(t, env)

    // Start server and client
    serverPort := startServer(t, env, certPath, keyPath)
    clientPort := startClient(t, env, serverPort)

    // Test connectivity
    err := env.TestSOCKSProxy(
        "127.0.0.1:"+itoa(clientPort),
        "https://httpbin.org/ip",
    )
    if err != nil {
        t.Errorf("SOCKS test failed: %v", err)
    }
}
```

## Mock Systemd

The `MockSystemdManager` provides a complete in-memory implementation of systemd operations for testing.

### Usage

```go
// Create mock
mock := service.NewMockSystemdManager("/tmp/state")

// Use in tests
mock.CreateService("my-service", service.ServiceConfig{
    Description: "My Service",
    ExecStart:   "/usr/bin/myapp",
})

mock.StartService("my-service")
mock.EnableService("my-service")

// Assertions
if !mock.IsServiceActive("my-service") {
    t.Error("service should be active")
}

// Simulate failures
mock.SimulateFailure("my-service")
```

### Setting as Default

```go
func TestWithMockSystemd(t *testing.T) {
    mock := service.NewMockSystemdManager("")
    service.SetDefaultManager(mock)
    defer service.ResetDefaultManager()

    // Tests now use mock
}
```

## Test Utilities

### Port Allocation

```go
import "github.com/net2share/dnstm/internal/testutil"

port, err := testutil.AllocatePort()
ports, err := testutil.AllocatePorts(3)
err := testutil.WaitForPort(port, 5*time.Second)
err := testutil.WaitForPortClosed(port, 5*time.Second)
```

### Test Environment

```go
import "github.com/net2share/dnstm/internal/testutil"

env := testutil.NewTestEnv(t)

// Write config
cfg := env.DefaultConfig()
env.WriteConfig(cfg)

// Read config
loaded, err := env.ReadConfig()

// Create dummy certs/keys
certPath, keyPath := env.CreateDummyCert("example.com")
pubPath, privPath := env.CreateDummyKey("example.com")
```

## Action System

dnstm uses a unified action system for both CLI and interactive menu modes.

### Defining an Action

```go
Register(&Action{
    ID:     ActionBackendAdd,
    Parent: ActionBackend,
    Use:    "add",
    Short:  "Add a new backend",
    // Inputs are ordered for interactive flow: type → tag → type-specific fields
    Inputs: []InputField{
        {
            Name:      "type",
            Label:     "Backend Type",
            Type:      InputTypeSelect,
            Required:  true,
            Options:   BackendTypeOptions(),
        },
        {
            Name:      "tag",
            Label:     "Tag",
            Type:      InputTypeText,
            Required:  true,
        },
        {
            Name:      "address",
            Label:     "Address",
            Type:      InputTypeText,
            Required:  true,
            ShowIf: func(ctx *Context) bool {
                return ctx.GetString("type") == "custom"
            },
        },
    },
})
```

### Input Types

- `InputTypeText` - Text input field
- `InputTypePassword` - Hidden text input
- `InputTypeNumber` - Numeric input
- `InputTypeSelect` - Single-select dropdown
- `InputTypeBool` - Boolean flag (CLI-only)

### Conditional Inputs

Use `ShowIf` to conditionally show inputs based on previous selections:

```go
{
    Name:  "password",
    Label: "Password",
    ShowIf: func(ctx *Context) bool {
        return ctx.GetString("type") == "shadowsocks"
    },
}
```

### Default Values

```go
{
    Name:    "port",
    Label:   "Port",
    Default: "5310",
}

// Or dynamic defaults
{
    Name:        "tag",
    Label:       "Tag",
    DefaultFunc: func(ctx *Context) string {
        return generateUniqueTag()
    },
}
```
