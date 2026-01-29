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
dnstm install                              # Install all (interactive mode selection)
dnstm install --mode single                # Install with single-tunnel mode
dnstm install --mode multi                 # Install with multi-tunnel mode
dnstm install --dnstt                      # Install dnstt-server only
dnstm install --slipstream                 # Install slipstream-server only
dnstm install --shadowsocks                # Install ssserver only
dnstm install --microsocks                 # Install microsocks only
```

This command:
- Creates the dnstm system user
- Initializes router configuration and directories
- Sets operating mode (single or multi)
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
dnstm router switch [name]                 # Switch active instance (single mode)
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

## Mode Command

Show or switch operating mode (subcommand of `router`).

```bash
dnstm router mode              # Show current mode
dnstm router mode single       # Switch to single-tunnel mode
dnstm router mode multi        # Switch to multi-tunnel mode
```

## Switch Command

Switch active instance in single-tunnel mode (subcommand of `router`).

```bash
dnstm router switch            # Interactive picker
dnstm router switch <name>     # Switch to named instance
```

## SSH Users

Manage SSH tunnel users. Launches the standalone sshtun-user tool.

```bash
dnstm ssh-users            # Launch sshtun-user management tool
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
# Install and initialize
sudo dnstm install --mode single

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
# Install in multi mode
sudo dnstm install --mode multi

# Add instances
sudo dnstm instance add ss1 --type slipstream-shadowsocks --domain t1.example.com
sudo dnstm instance add dnstt1 --type dnstt-socks --domain t2.example.com

# Start all
sudo dnstm router start
```

### Switch Between Instances

```bash
# Switch to single mode
sudo dnstm router mode single

# Switch active instance
sudo dnstm router switch ss1
```
