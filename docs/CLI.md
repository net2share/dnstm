# CLI Reference

All commands require root privileges (`sudo`).

## Interactive Mode

Run without arguments for the interactive menu:

```bash
sudo dnstm
```

The menu structure mirrors the CLI commands exactly.

## Install Command

Install all components and configure the system.

```bash
dnstm install                              # Install all components
dnstm install --dnstt                      # Install dnstt-server only
dnstm install --slipstream                 # Install slipstream-server only
dnstm install --shadowsocks                # Install ssserver only
dnstm install --microsocks                 # Install microsocks only
```

This command:
- Creates the dnstm system user
- Downloads and installs transport binaries
- Installs and starts the microsocks SOCKS5 proxy
- Configures firewall rules (port 53 UDP/TCP)

**Note:** Other commands require installation to be completed first.

## Router Commands

Manage the DNS tunnel router.

```bash
dnstm router init [--mode single|multi]   # Initialize router
dnstm router status                        # Show router status
dnstm router start                         # Start all tunnels
dnstm router stop                          # Stop all tunnels
dnstm router restart                       # Restart all tunnels
dnstm router logs [-n lines]               # Show DNS router logs
dnstm router config                        # Show configuration
dnstm router reset [--force]               # Reset to initial state
```

## Instance Commands

Manage transport instances.

```bash
dnstm instance list                        # List all instances
dnstm instance add [name] [flags]          # Add new instance
dnstm instance remove <name> [--force]     # Remove instance
dnstm instance start <name>                # Start instance
dnstm instance stop <name>                 # Stop instance
dnstm instance restart <name>              # Restart instance
dnstm instance logs <name> [-n lines]      # Show instance logs
dnstm instance config <name>               # Show instance config
dnstm instance status <name>               # Show instance status
```

### Instance Add Flags

```bash
dnstm instance add myinstance \
  --type slipstream-shadowsocks \
  --domain t.example.com \
  --password "optional-password" \
  --method aes-256-gcm
```

| Flag | Description |
|------|-------------|
| `--type`, `-t` | Transport type (required for CLI mode) |
| `--domain`, `-d` | Domain name (required for CLI mode) |
| `--target` | Target address (for socks/ssh types) |
| `--password` | Shadowsocks password (auto-generated if empty) |
| `--method` | Shadowsocks encryption method |

Transport types:
- `slipstream-shadowsocks` - Slipstream + Shadowsocks (recommended)
- `slipstream-socks` - Slipstream with SOCKS target
- `slipstream-ssh` - Slipstream with SSH target
- `dnstt-socks` - DNSTT with SOCKS target
- `dnstt-ssh` - DNSTT with SSH target

## Mode Commands

Show or switch operating mode.

```bash
dnstm mode              # Show current mode
dnstm mode single       # Switch to single-tunnel mode
dnstm mode multi        # Switch to multi-tunnel mode
```

## Switch Command

Switch active instance in single-tunnel mode.

```bash
dnstm switch            # Interactive picker
dnstm switch <name>     # Switch to named instance
dnstm switch list       # List available instances
```

## SSH Users

Manage SSH tunnel users.

```bash
dnstm ssh-users            # Open management menu
dnstm ssh-users uninstall  # Remove all users and hardening
```

## SOCKS Proxy

Manage microsocks SOCKS5 proxy. Automatically installed by `dnstm install`.

```bash
dnstm socks status     # Show proxy status
dnstm socks install    # Reinstall proxy
dnstm socks uninstall  # Remove proxy
```

## Uninstall

Remove all dnstm components. Can be run from interactive menu or CLI.

```bash
dnstm uninstall [--force]
```

This removes:
- All instance services
- DNS router and microsocks services
- Configuration files (`/etc/dnstm/`)
- Transport binaries

**Note:** The dnstm binary is kept for easy reinstallation. To fully remove: `rm /usr/local/bin/dnstm`

## Examples

### Quick Setup

```bash
# Initialize
sudo dnstm router init --mode single

# Add Shadowsocks instance
sudo dnstm instance add ss1 \
  --type slipstream-shadowsocks \
  --domain t.example.com

# Start
sudo dnstm router start

# Check status
sudo dnstm router status
```

### Multiple Instances

```bash
# Initialize in multi mode
sudo dnstm router init --mode multi

# Add instances
sudo dnstm instance add ss1 --type slipstream-shadowsocks --domain t1.example.com
sudo dnstm instance add dnstt1 --type dnstt-socks --domain t2.example.com

# Start all
sudo dnstm router start
```

### Switch Between Instances

```bash
# Switch to single mode
sudo dnstm mode single

# Switch active instance
sudo dnstm switch ss1
```
