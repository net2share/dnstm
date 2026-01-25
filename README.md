# DNS Tunnel Manager (dnstm)

A tool to deploy and manage DNS tunnel servers on Linux. Supports multiple providers:

- **[Slipstream](https://github.com/Mygod/slipstream-rust)** - Modern DNS tunnel with TLS encryption
- **[DNSTT](https://www.bamsoftware.com/software/dnstt/)** - Classic DNS tunnel with Curve25519 keys

Both providers can be installed simultaneously, with one active at a time handling DNS queries.

## Features

- Interactive menu and full CLI command support
- Multiple DNS tunnel providers (Slipstream, DNSTT)
- Switch between providers with a single command
- Auto-generates cryptographic credentials (TLS certs or Curve25519 keys)
- Configures firewall rules (UFW, firewalld, iptables)
- Sets up systemd services with security hardening
- SSH tunnel mode with integrated user management via [sshtun-user](https://github.com/net2share/sshtun-user)
- Optional Dante SOCKS proxy setup for SOCKS mode
- Supports multiple architectures (amd64, arm64)

## Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/net2share/dnstm/main/install.sh | sudo bash
```

## Requirements

- Linux (Debian/Ubuntu, RHEL/CentOS/Fedora)
- Root access
- systemd
- A domain with DNS records pointing to your server

## DNS Setup

Before running dnstm, configure your DNS records:

1. Create an A record pointing to your server:

   ```
   ns.example.com.  IN  A  YOUR_SERVER_IP
   ```

2. Create an NS record for the tunnel subdomain:
   ```
   t.example.com.   IN  NS  ns.example.com.
   ```

## Usage

### Interactive Menu

Run without arguments for an interactive menu:

```bash
sudo dnstm
```

The main menu shows both providers with their status:

```
Status: Slipstream not installed, DNSTT active

> Slipstream →
  DNSTT (active) →
  View Overall Status
  Manage SSH tunnel users
  Exit
```

Each provider submenu offers:
- Install/Reconfigure
- Check service status
- View logs
- Show configuration
- Restart service
- Set as Active DNS Handler (if installed but not active)
- Uninstall

### CLI Commands

```bash
# Show help
dnstm --help

# View combined status of all providers
sudo dnstm status

# View status of specific provider
sudo dnstm status slipstream
sudo dnstm status dnstt
```

#### Install Commands

```bash
# Install Slipstream (interactive)
sudo dnstm install slipstream

# Install Slipstream (CLI mode)
sudo dnstm install slipstream --domain t.example.com --mode ssh

# Install DNSTT (interactive)
sudo dnstm install dnstt

# Install DNSTT (CLI mode)
sudo dnstm install dnstt --ns-subdomain t.example.com --mtu 1232 --mode ssh
```

#### Provider Management

```bash
# Switch active DNS handler
sudo dnstm switch slipstream
sudo dnstm switch dnstt

# View logs
sudo dnstm logs slipstream
sudo dnstm logs dnstt

# Show configuration
sudo dnstm config slipstream
sudo dnstm config dnstt

# Restart service
sudo dnstm restart slipstream
sudo dnstm restart dnstt
```

#### Uninstall Commands

```bash
# Uninstall provider (interactive)
sudo dnstm uninstall slipstream
sudo dnstm uninstall dnstt

# Uninstall and remove SSH tunnel users
sudo dnstm uninstall slipstream --remove-ssh-users

# Uninstall but keep SSH tunnel users
sudo dnstm uninstall dnstt --keep-ssh-users
```

#### SSH Tunnel Users

```bash
# Manage SSH tunnel users (opens submenu)
sudo dnstm ssh-users
```

### Install Options

**Slipstream:**

| Option | Description |
| ------ | ----------- |
| `--domain <domain>` | Tunnel domain (e.g., t.example.com) |
| `--mode <ssh\|socks>` | Tunnel mode (default: ssh) |
| `--port <port>` | Target port (default: 22 for ssh, 1080 for socks) |

**DNSTT:**

| Option | Description |
| ------ | ----------- |
| `--ns-subdomain <domain>` | NS subdomain (e.g., t.example.com) |
| `--mtu <value>` | MTU value (512-1400, default: 1232) |
| `--mode <ssh\|socks>` | Tunnel mode (default: ssh) |
| `--port <port>` | Target port (default: 22 for ssh, 1080 for socks) |

## Providers

### Slipstream

[Slipstream](https://github.com/Mygod/slipstream-rust) is a modern DNS tunnel implementation with TLS encryption.

**Features:**
- TLS encryption with auto-generated ECDSA P-256 certificates
- Listens on port 5301 (NAT redirected from 53)
- Configuration stored in `/etc/slipstream/`

**Installation creates:**
- Self-signed TLS certificate (10-year validity)
- systemd service (`slipstream-server`)
- System user (`slipstream`)

**Client connection:**
```bash
slipstream-client --domain t.example.com --fingerprint <SHA256_FINGERPRINT> \
  --dns-server RESOLVER_IP:53 --local-address 127.0.0.1:8000
```

### DNSTT

[DNSTT](https://www.bamsoftware.com/software/dnstt/) is a classic DNS tunnel using Curve25519 encryption.

**Features:**
- Curve25519 key-based encryption
- Configurable MTU (512-1400)
- Listens on port 5300 (NAT redirected from 53)
- Configuration stored in `/etc/dnstt/`

**Installation creates:**
- Curve25519 key pair (64-char hex strings)
- systemd service (`dnstt-server`)
- System user (`dnstt`)

**Client connection:**
```bash
dnstt-client -udp RESOLVER_IP:53 -pubkey-file server.pub t.example.com 127.0.0.1:8000
```

For SSH tunnel mode:
```bash
ssh -o ProxyCommand="dnstt-client -udp RESOLVER_IP:53 -pubkey-file server.pub t.example.com 127.0.0.1:8000" user@localhost
```

## Switching Providers

Both providers can be installed simultaneously. Only one handles DNS queries at a time.

```bash
# Switch to Slipstream
sudo dnstm switch slipstream

# Switch to DNSTT
sudo dnstm switch dnstt
```

The switch command:
1. Stops the current active provider
2. Updates firewall NAT rules (53 → new provider port)
3. Starts the new provider
4. Updates global config (`/etc/dnstm/dnstm.conf`)

## Tunnel Modes

### SSH Mode (default)

In SSH mode, the tunnel forwards SSH traffic. During installation, dnstm automatically:

1. Applies sshd hardening configuration
2. Configures fail2ban for brute-force protection
3. Prompts to create a restricted tunnel user

Tunnel users can only create local (`-L`) and SOCKS (`-D`) tunnels, with no shell access.

Manage SSH tunnel users anytime via `sudo dnstm ssh-users`.

### SOCKS Mode

In SOCKS mode, dnstm installs a Dante SOCKS5 proxy. Clients connect directly to the proxy without SSH authentication.

## Configuration

### Global Config

The active provider is tracked in `/etc/dnstm/dnstm.conf`:

```
ACTIVE_PROVIDER="slipstream"
```

### Slipstream Config

Stored in `/etc/slipstream/slipstream-server.conf`:

```
DOMAIN="t.example.com"
DNS_LISTEN_PORT="5301"
TARGET_ADDRESS="127.0.0.1:22"
CERT_FILE="/etc/slipstream/t_example_com_cert.pem"
KEY_FILE="/etc/slipstream/t_example_com_key.pem"
TUNNEL_MODE="ssh"
```

### DNSTT Config

Stored in `/etc/dnstt/dnstt-server.conf`:

```
NS_SUBDOMAIN="t.example.com"
MTU_VALUE="1232"
TUNNEL_MODE="ssh"
PRIVATE_KEY_FILE="/etc/dnstt/t_example_com_server.key"
PUBLIC_KEY_FILE="/etc/dnstt/t_example_com_server.pub"
TARGET_PORT="22"
```

## Uninstall

Each provider can be uninstalled independently:

```bash
# Uninstall Slipstream
sudo dnstm uninstall slipstream

# Uninstall DNSTT
sudo dnstm uninstall dnstt
```

The uninstall process removes:
- Provider service and binary
- Configuration files and credentials
- Firewall rules for that provider
- Provider system user
- (Optionally) SSH tunnel users and sshd hardening config

If uninstalling the active provider while another is installed, dnstm automatically switches to the other provider.

## Building from Source

```bash
git clone https://github.com/net2share/dnstm.git
cd dnstm
go build -o dnstm .
```

## Architecture

```
/etc/dnstm/           # Global config (active provider)
/etc/slipstream/      # Slipstream config and TLS certs
/etc/dnstt/           # DNSTT config and keys
/usr/local/bin/       # Provider binaries
```

Both providers use NAT PREROUTING rules to redirect port 53 to their respective ports:
- Slipstream: 53 → 5301
- DNSTT: 53 → 5300
