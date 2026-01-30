# Development Guide

This document explains the dnstm architecture and how to add new commands.

## Architecture Overview

dnstm uses a **unified action-based architecture** where CLI commands and interactive menu items are defined once and share business logic.

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Interface                          │
├────────────────────────────┬────────────────────────────────────┤
│      CLI (Cobra)           │      Interactive Menu (TUI)        │
│      cmd/adapter.go        │      internal/menu/adapter.go      │
└────────────────────────────┴────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Action System                               │
│                  internal/actions/                              │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐          │
│  │  action.go   │  │ registry.go  │  │  output.go    │          │
│  │ (Core types) │  │ (Registry)   │  │ (OutputWriter)│          │
│  └──────────────┘  └──────────────┘  └───────────────┘          │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐          │
│  │   ids.go     │  │ instance.go  │  │  router.go    │          │
│  │ (Constants)  │  │(Definitions) │  │(Definitions)  │          │
│  └──────────────┘  └──────────────┘  └───────────────┘          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Handlers                                    │
│                  internal/handlers/                             │
│  ┌──────────────────┐  ┌──────────────────┐                     │
│  │ instance_list.go │  │ router_status.go │  ...                │
│  │ (Business logic) │  │ (Business logic) │                     │
│  └──────────────────┘  └──────────────────┘                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Core Packages                               │
│  internal/router, internal/transport, internal/certs, etc.      │
└─────────────────────────────────────────────────────────────────┘
```

## Key Components

### Action IDs (`internal/actions/ids.go`)

All action IDs are defined as constants to prevent typos and enable IDE support:

```go
const (
    ActionInstance            = "instance"
    ActionInstanceList        = "instance.list"
    ActionInstanceAdd         = "instance.add"
    // ... etc
)
```

Always use these constants when referencing actions in code.

### Action (`internal/actions/action.go`)

An `Action` defines a command with:

- **ID**: Unique identifier (use constants from `ids.go`)
- **Parent**: Parent action ID for hierarchical structure (use constants)
- **Use/Short/Long**: Cobra-style command descriptions
- **Args**: Positional argument specification with optional picker
- **Inputs**: Input fields (text, password, select, number)
- **Confirm**: Confirmation prompt configuration
- **Handler**: Business logic function
- **RequiresRoot/RequiresInstalled**: Prerequisites

### Registry (`internal/actions/registry.go`)

Global registry for all actions:

- `Register(action)` - Add an action
- `Get(id)` - Get action by ID
- `ByParent(parentID)` - Get children of a parent action
- `TopLevel()` - Get root-level actions

### Handlers (`internal/handlers/`)

Business logic functions that:

- Receive an `actions.Context` with args, values, config
- Use `OutputWriter` for consistent output
- Return errors (including `actions.ErrCancelled`)

### Adapters

- **CLI Adapter** (`cmd/adapter.go`): Generates Cobra commands from actions
- **Menu Adapter** (`internal/menu/adapter.go`): Generates menu flows from actions

## Adding a New Command

### 1. Add Action ID Constant

First, add the action ID constant to `internal/actions/ids.go`:

```go
const (
    // ... existing constants ...

    // MyFeature actions
    ActionMyFeature          = "myfeature"
    ActionMyFeatureMyCommand = "myfeature.mycommand"
)
```

### 2. Define the Action

In `internal/actions/` (choose appropriate file or create new):

```go
func init() {
    Register(&Action{
        ID:                ActionMyFeatureMyCommand,
        Parent:            ActionMyFeature,  // or "" for top-level
        Use:               "mycommand <arg>",
        Short:             "Short description",
        Long:              "Detailed description",
        MenuLabel:         "Menu Label",
        RequiresRoot:      true,
        RequiresInstalled: true,
        Args: &ArgsSpec{
            Name:        "arg",
            Description: "Argument description",
            Required:    true,
            PickerFunc:  MyPicker,  // Optional: for interactive selection
        },
        Inputs: []InputField{
            {
                Name:      "option",
                Label:     "Option Label",
                ShortFlag: 'o',
                Type:      InputTypeText,
                Required:  true,
            },
        },
        Confirm: &ConfirmConfig{  // Optional
            Message:   "Are you sure?",
            ForceFlag: "force",
        },
    })
}
```

### 3. Create the Handler

In `internal/handlers/myfeature_mycommand.go`:

```go
package handlers

import (
    "github.com/net2share/dnstm/internal/actions"
)

func init() {
    // Register handler with the action using action ID constants
    // Use SetInstanceHandler for instance.* actions
    // Use SetRouterHandler for router.* actions
    // Use SetSystemHandler for system-level actions (install, uninstall, ssh-users)
    actions.SetHandler(actions.ActionMyFeatureMyCommand, HandleMyCommand)
}

func HandleMyCommand(ctx *actions.Context) error {
    // Check requirements
    if err := CheckRequirements(ctx, true, true); err != nil {
        return err
    }

    // Get argument
    arg := ctx.GetArg(0)
    if arg == "" {
        return actions.NewActionError("argument required", "Usage: dnstm myfeature mycommand <arg>")
    }

    // Get input value
    option := ctx.GetString("option")

    // Business logic here...
    ctx.Output.Info("Processing...")

    // Use structured output
    ctx.Output.Success("Done!")

    return nil
}
```

### 4. Build and Test

```bash
# Build
go build ./...

# Test CLI
sudo dnstm myfeature mycommand arg --option value

# Test interactive mode
sudo dnstm  # Navigate to your command
```

## Input Types

| Type                | Description                    | Context Method        |
| ------------------- | ------------------------------ | --------------------- |
| `InputTypeText`     | Text input                     | `ctx.GetString(name)` |
| `InputTypePassword` | Hidden input                   | `ctx.GetString(name)` |
| `InputTypeSelect`   | Dropdown menu                  | `ctx.GetString(name)` |
| `InputTypeNumber`   | Numeric input                  | `ctx.GetInt(name)`    |
| `InputTypeBool`     | Boolean flag (CLI-only)        | `ctx.GetBool(name)`   |

**Note on `InputTypeBool`:** Boolean flags are CLI-only and skipped in interactive mode (defaults to false). For yes/no prompts in interactive mode, use `action.Confirm` for confirmation dialogs or `InputTypeSelect` with Yes/No options.

## Output Methods

The `OutputWriter` interface provides:

```go
ctx.Output.Print(msg)           // Plain text
ctx.Output.Println(args...)     // With newline
ctx.Output.Printf(fmt, args...) // Formatted

ctx.Output.Info(msg)            // ℹ info message
ctx.Output.Success(msg)         // ✓ success message
ctx.Output.Warning(msg)         // ⚠ warning message
ctx.Output.Error(msg)           // ✗ error message

ctx.Output.Status(msg)          // ✓ status update
ctx.Output.Step(curr, total, msg) // [1/4] step progress

ctx.Output.Box(title, lines)    // Bordered box
ctx.Output.Table(headers, rows) // Table output
ctx.Output.Separator(length)    // Horizontal line
```

## Error Handling

Use structured errors for better UX:

```go
// Simple error with hint
return actions.NewActionError("something failed", "Try running X first")

// Instance not found
return actions.NotFoundError(name)

// Instance already exists
return actions.ExistsError(name)

// Cancelled by user
return actions.ErrCancelled
```

## Testing

### Unit Tests

```go
func TestMyHandler(t *testing.T) {
    // Create mock output
    output := &MockOutput{}

    ctx := &actions.Context{
        Args:   []string{"arg1"},
        Values: map[string]interface{}{"option": "value"},
        Output: output,
    }

    err := HandleMyCommand(ctx)

    assert.NoError(t, err)
    assert.Contains(t, output.messages, "Done!")
}
```

### Integration Tests

```bash
# Test CLI non-interactive
dnstm myfeature mycommand arg --option value

# Test CLI with picker fallback
dnstm myfeature mycommand  # Should show picker

# Test in interactive menu
sudo dnstm  # Navigate and test
```

## Best Practices

1. **Use action ID constants**: Always use constants from `ids.go` instead of string literals
2. **Keep handlers focused**: One handler per action, delegate to helper functions
3. **Use context methods**: `ctx.GetString()`, `ctx.GetInt()` for type-safe access
4. **Reload config when needed**: Call `ctx.Reload()` after mutations
5. **Use structured output**: Never use `fmt.Print` directly
6. **Return meaningful errors**: Use `ActionError` with hints
7. **Preserve running state**: Check `wasRunning` before restart operations

## Installation Behavior

### Binary Path

The dnstm binary is installed at `/usr/local/bin/dnstm`. All systemd services reference this path.

When running `dnstm install`:

- If running from a different location (e.g., `/root/dnstm-test`), it automatically copies itself to `/usr/local/bin/dnstm`
- This ensures services always use the correct binary path
- Skips copy if already at the correct path

### First-Time Install

For fresh installs, use the install script:

```bash
curl -sSL https://raw.githubusercontent.com/net2share/dnstm/main/install.sh | sudo bash
```

This downloads the latest release to `/usr/local/bin/dnstm`.

## Remote Testing Workflow

### Deploy New Version

To test changes on a remote server:

```bash
# Build for Linux
GOOS=linux GOARCH=amd64 go build -o dnstm-linux .

# Stop services (binary may be in use)
ssh server "dnstm router stop"

# Copy directly to install path
scp dnstm-linux server:/usr/local/bin/dnstm

# Clean up local build
rm dnstm-linux

# Restart services
ssh server "dnstm router start"
```

### Testing Transports

For each transport type, test with curl through the tunnel:

**Slipstream + Shadowsocks:**

```bash
# Start tunnel
slipstream-client -d DOMAIN -r 8.8.8.8:53 --cert cert.pem -l 5201 &

# Start shadowsocks client
sslocal -s 127.0.0.1:5201 -k "PASSWORD" -m METHOD -b 127.0.0.1:1080 &

# Test
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

**Slipstream SOCKS:**

```bash
slipstream-client -d DOMAIN -r 8.8.8.8:53 --cert cert.pem -l 1080 &
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

**Slipstream/DNSTT SSH:**

```bash
# Start tunnel
slipstream-client -d DOMAIN -r 8.8.8.8:53 --cert cert.pem -l 2222 &
# or for DNSTT:
dnstt-client -udp 8.8.8.8:53 -pubkey PUBKEY DOMAIN 127.0.0.1:2222 &

# Start SSH with dynamic forwarding
ssh -D 1080 -f -N -p 2222 user@127.0.0.1

# Test with curl
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

### Fetching Credentials

Get connection info from server:

```bash
# Slipstream certificate
scp server:/etc/dnstm/certs/<domain>_cert.pem ./cert.pem

# DNSTT public key
ssh server "dnstm instance status <name>" | grep -A1 "Public Key"

# Shadowsocks password
ssh server "grep -A5 '<instance>:' /etc/dnstm/config.yaml"
```
