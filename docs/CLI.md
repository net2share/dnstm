# CLI Reference

All commands require root privileges (`sudo`).

## Interactive Mode

Run without arguments for the interactive menu:

```bash
sudo dnstm
```

The menu structure mirrors the CLI commands exactly. When optional arguments are not provided, the CLI will fall back to interactive mode for selection.

## Install Command

Install all components and configure the system.

```bash
dnstm install                              # Interactive install with confirmation
dnstm install --force                      # Install without confirmation prompts
dnstm install --mode single                # Explicitly set single-tunnel mode
dnstm install --mode multi                 # Install with multi-tunnel mode
```

| Flag | Description |
|------|-------------|
| `--force`, `-f` | Skip confirmation prompts |
| `--mode`, `-m` | Operating mode: `single` (default) or `multi` |

This command:
- Creates the dnstm system user
- Initializes router configuration and directories
- Sets operating mode (single or multi)
- Creates default backends (socks, ssh)
- Creates DNS router service
- Downloads and installs transport binaries
- Installs and starts the microsocks SOCKS5 proxy
- Configures firewall rules (port 53 UDP/TCP)

**Note:** Other commands require installation to be completed first.

## Router Commands

Manage the DNS tunnel router.

```bash
dnstm router status                        # Show router status
dnstm router start                         # Start all tunnels
dnstm router stop                          # Stop all tunnels
dnstm router logs [-n lines]               # Show DNS router logs
dnstm router mode [single|multi]           # Show or switch mode
dnstm router switch [tag]                  # Switch active tunnel (single mode)
```

## Tunnel Commands

Manage DNS tunnels (previously called instances).

```bash
dnstm tunnel list                          # List all tunnels
dnstm tunnel add [tag] [flags]             # Add new tunnel
dnstm tunnel remove <tag> [--force]        # Remove tunnel
dnstm tunnel start <tag>                   # Start tunnel
dnstm tunnel stop <tag>                    # Stop tunnel
dnstm tunnel restart <tag>                 # Restart tunnel
dnstm tunnel enable <tag>                  # Enable tunnel
dnstm tunnel disable <tag>                 # Disable tunnel
dnstm tunnel logs <tag> [-n lines]         # Show tunnel logs
dnstm tunnel status <tag>                  # Show tunnel status with cert/key info
dnstm tunnel reconfigure <tag>             # Reconfigure tunnel (including rename)
```

### Tunnel Add Flags

```bash
dnstm tunnel add my-tunnel \
  --transport slipstream \
  --backend ss-primary \
  --domain t.example.com
```

| Flag | Description |
|------|-------------|
| `--transport`, `-t` | Transport type: `slipstream` or `dnstt` |
| `--backend`, `-b` | Backend tag to forward traffic to |
| `--domain`, `-d` | Domain name |
| `--port`, `-p` | Port number (auto-allocated if not specified) |
| `--mtu` | MTU for DNSTT (default: 1232) |

### Interactive Fallback

When required flags are not provided, commands fall back to interactive mode:

```bash
dnstm tunnel add              # Opens interactive add flow
dnstm tunnel remove           # Shows tunnel picker
dnstm router switch           # Shows tunnel picker
```

## Backend Commands

Manage backend services that tunnels forward traffic to.

```bash
dnstm backend list                         # List all backends
dnstm backend available                    # Show available backend types
dnstm backend add [flags]                  # Add new backend
dnstm backend remove <tag>                 # Remove backend
dnstm backend status <tag>                 # Show backend status
```

### Backend Add Flags

```bash
# Add a Shadowsocks backend
dnstm backend add \
  --type shadowsocks \
  --tag ss-primary \
  --password "my-password" \
  --method aes-256-gcm

# Add a custom target backend
dnstm backend add \
  --type custom \
  --tag web-server \
  --address 127.0.0.1:8080
```

| Flag | Description |
|------|-------------|
| `--type`, `-t` | Backend type: `shadowsocks` or `custom` |
| `--tag`, `-n` | Unique identifier for the backend |
| `--address`, `-a` | Target address (for custom backends) |
| `--password`, `-p` | Shadowsocks password (auto-generated if empty) |
| `--method`, `-m` | Shadowsocks encryption method |

### Backend Types

| Type | Description | Addable |
|------|-------------|---------|
| `socks` | Built-in SOCKS5 proxy (microsocks at 127.0.0.1:1080) | No (built-in) |
| `ssh` | Built-in SSH server (127.0.0.1:22) | No (built-in) |
| `shadowsocks` | Shadowsocks server (slipstream only, uses SIP003 plugin) | Yes |
| `custom` | Custom target address | Yes |

**Notes:**
- SOCKS and SSH backends are created automatically during installation and cannot be added manually.
- DNSTT transport does not support the `shadowsocks` backend type.

## Config Commands

Manage configuration files.

```bash
dnstm config export [-o file]              # Export current config to stdout or file
dnstm config load <file>                   # Load and deploy config from file
dnstm config validate <file>               # Validate config file without deploying
```

### Config Export

```bash
# Export to stdout
dnstm config export

# Export to file
dnstm config export -o backup.json
```

### Config Load

```bash
# Load from file (validates and saves to /etc/dnstm/config.json)
dnstm config load my-config.json
```

### Config Validate

```bash
# Validate without deploying
dnstm config validate my-config.json
```

## Mode Command

Show or switch operating mode (subcommand of `router`).

```bash
dnstm router mode              # Show current mode
dnstm router mode single       # Switch to single-tunnel mode
dnstm router mode multi        # Switch to multi-tunnel mode
```

**Single-tunnel mode:**
- One tunnel active at a time
- Transport binds directly to external IP:53
- Lower overhead (no DNS router process)

**Multi-tunnel mode:**
- All enabled tunnels run simultaneously
- DNS router handles domain-based routing
- Each domain routes to its designated tunnel

## Switch Command

Switch active tunnel in single-tunnel mode (subcommand of `router`).

```bash
dnstm router switch            # Interactive picker
dnstm router switch <tag>      # Switch to named tunnel
```

## SSH Users

Manage SSH tunnel users. Launches the standalone sshtun-user tool.

```bash
dnstm ssh-users            # Launch sshtun-user management tool
```

## Update Command

Check for and install updates to dnstm and transport binaries.

```bash
dnstm update                           # Check and install updates (interactive)
dnstm update --check                   # Check only, don't install
dnstm update --force                   # Skip confirmation prompts
dnstm update --self                    # Only update dnstm itself
dnstm update --binaries                # Only update transport binaries
```

| Flag | Description |
|------|-------------|
| `--check` | Dry-run: show available updates without installing |
| `--force` | Skip confirmation prompts |
| `--self` | Only update dnstm itself |
| `--binaries` | Only update transport binaries |

The update process:
- Checks for newer dnstm version on GitHub
- Compares installed binary versions against pinned versions
- Stops affected services before updating
- Downloads and installs new versions
- Restarts previously running services

## Uninstall

Remove all dnstm components. Can be run from interactive menu or CLI.

```bash
dnstm uninstall [--force]
```

This removes:
- All tunnel services
- DNS router and microsocks services
- Configuration files (`/etc/dnstm/`)
- Transport binaries

**Note:** The dnstm binary is kept for easy reinstallation. To fully remove: `rm /usr/local/bin/dnstm`

## Examples

### Quick Setup

```bash
# Install and initialize
sudo dnstm install --mode single

# Add Shadowsocks backend
sudo dnstm backend add \
  --type shadowsocks \
  --tag ss-primary \
  --password "my-password"

# Add Slipstream tunnel
sudo dnstm tunnel add main \
  --transport slipstream \
  --backend ss-primary \
  --domain t.example.com

# Start
sudo dnstm router start

# Check status
sudo dnstm router status
```

### Multiple Tunnels

```bash
# Install in multi mode
sudo dnstm install --mode multi

# Add tunnels with different transports
sudo dnstm tunnel add slipstream-1 \
  --transport slipstream \
  --backend ss-primary \
  --domain t1.example.com

sudo dnstm tunnel add dnstt-1 \
  --transport dnstt \
  --backend socks \
  --domain t2.example.com

# Start all
sudo dnstm router start
```

### Switch Between Tunnels

```bash
# Switch to single mode
sudo dnstm router mode single

# Switch active tunnel
sudo dnstm router switch slipstream-1
```

### Export and Restore Configuration

```bash
# Export current config
sudo dnstm config export -o backup.json

# Validate before deploying
dnstm config validate backup.json

# Load on another server
sudo dnstm config load backup.json
sudo dnstm router start
```

### Reconfigure Tunnel

```bash
# Rename and reconfigure a tunnel
sudo dnstm tunnel reconfigure my-tunnel

# This opens an interactive flow to modify:
# - Tunnel tag (rename)
# - Domain
# - Backend
# - Transport-specific settings (MTU for DNSTT)
```
