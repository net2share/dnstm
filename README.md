# DNS Tunnel Manager (dnstm)

A tool to deploy and manage DNS tunnel servers on Linux. Supports multiple providers:

- **[Shadowsocks](https://github.com/shadowsocks/shadowsocks-rust)** - Shadowsocks with Slipstream DNS tunnel plugin (SIP003)
- **[Slipstream](https://github.com/Mygod/slipstream-rust)** - Modern DNS tunnel with TLS encryption
- **[DNSTT](https://www.bamsoftware.com/software/dnstt/)** - Classic DNS tunnel with Curve25519 keys

All providers can be installed simultaneously, with one active at a time handling DNS queries.

## Features

- Interactive menu and full CLI command support
- Multiple DNS tunnel providers (Slipstream, DNSTT)
- Switch between providers with a single command
- Auto-generates cryptographic credentials (TLS certs or Curve25519 keys)
- Configures firewall rules (UFW, firewalld, iptables)
- Sets up systemd services with security hardening
- SSH tunnel mode with integrated user management via [sshtun-user](https://github.com/net2share/sshtun-user)
- Optional microsocks SOCKS5 proxy for SOCKS mode
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

The main menu shows all providers with their status:

```
> Shadowsocks →
  Slipstream (active) →
  DNSTT →
  Manage SSH tunnel users
  Manage SOCKS proxy
  Status
  Exit
```

Each provider submenu offers:
- Install/Reconfigure
- Service status
- Logs
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
# Install Shadowsocks (interactive)
sudo dnstm shadowsocks install

# Install Shadowsocks (CLI mode)
sudo dnstm shadowsocks install --domain t.example.com

# Install Slipstream (interactive)
sudo dnstm slipstream install

# Install Slipstream (CLI mode)
sudo dnstm slipstream install --domain t.example.com --mode ssh

# Install DNSTT (interactive)
sudo dnstm dnstt install

# Install DNSTT (CLI mode)
sudo dnstm dnstt install --ns-subdomain t.example.com --mtu 1232 --mode ssh
```

#### Provider Management

```bash
# Switch active DNS handler
sudo dnstm switch shadowsocks
sudo dnstm switch slipstream
sudo dnstm switch dnstt

# View logs
sudo dnstm shadowsocks logs
sudo dnstm slipstream logs
sudo dnstm dnstt logs

# Show configuration
sudo dnstm shadowsocks config
sudo dnstm slipstream config
sudo dnstm dnstt config

# Restart service
sudo dnstm shadowsocks restart
sudo dnstm slipstream restart
sudo dnstm dnstt restart
```

#### Uninstall Commands

```bash
# Uninstall provider
sudo dnstm shadowsocks uninstall
sudo dnstm slipstream uninstall
sudo dnstm dnstt uninstall
```

#### SSH Tunnel Users

```bash
# Manage SSH tunnel users (opens submenu)
sudo dnstm ssh-users

# Uninstall SSH tunnel hardening and users
sudo dnstm ssh-users uninstall
```

#### SOCKS Proxy

```bash
# Install/reinstall SOCKS proxy
sudo dnstm socks install

# Uninstall SOCKS proxy
sudo dnstm socks uninstall

# Show SOCKS proxy status
sudo dnstm socks status
```

### Install Options

**Shadowsocks:**

| Option | Description |
| ------ | ----------- |
| `--domain <domain>` | Tunnel domain (e.g., t.example.com) |
| `--password <pass>` | Shadowsocks password (auto-generated if empty) |
| `--method <method>` | Encryption method (default: aes-256-gcm) |

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

### Shadowsocks (SIP003)

[Shadowsocks](https://github.com/shadowsocks/shadowsocks-rust) with [Slipstream](https://github.com/Mygod/slipstream-rust) as a SIP003 plugin. This combines Shadowsocks encryption with DNS tunneling.

**Features:**
- AEAD encryption (aes-256-gcm, chacha20-ietf-poly1305)
- Slipstream plugin handles DNS tunnel transport
- Listens on port 5302 (NAT redirected from 53)
- Configuration stored in `/etc/shadowsocks-slipstream/`

**Installation creates:**
- Shadowsocks config with Slipstream plugin
- TLS certificate for Slipstream (shared with Slipstream provider)
- systemd service (`shadowsocks-slipstream`)

**Client connection:**
Requires a Shadowsocks client with Slipstream plugin support:
```
Server:      t.example.com (via DNS resolver)
Port:        53
Password:    <generated-password>
Method:      aes-256-gcm
Plugin:      slipstream
Plugin Opts: domain=t.example.com;fingerprint=<SHA256>
```

### Slipstream

[Slipstream](https://github.com/Mygod/slipstream-rust) is a modern DNS tunnel implementation with TLS encryption.

**Features:**
- TLS encryption with auto-generated ECDSA P-256 certificates
- Listens on port 5301 (NAT redirected from 53)
- Configuration stored in `/etc/slipstream/`

**Installation creates:**
- Self-signed TLS certificate (10-year validity)
- systemd service (`slipstream-server`)

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

**Client connection:**
```bash
dnstt-client -udp RESOLVER_IP:53 -pubkey-file server.pub t.example.com 127.0.0.1:8000
```

For SSH tunnel mode:
```bash
ssh -o ProxyCommand="dnstt-client -udp RESOLVER_IP:53 -pubkey-file server.pub t.example.com 127.0.0.1:8000" user@localhost
```

## Switching Providers

All providers can be installed simultaneously. Only one handles DNS queries at a time.

```bash
# Switch to Shadowsocks
sudo dnstm switch shadowsocks

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

### SSH Mode (Recommended)

In SSH mode, the tunnel forwards SSH traffic. After installation, configure SSH hardening separately:

1. Run `sudo dnstm ssh-users` to access the SSH users menu
2. Apply sshd hardening configuration
3. Create restricted tunnel users

Tunnel users can only create local (`-L`) and SOCKS (`-D`) tunnels, with no shell access.

### SOCKS Mode (Legacy)

In SOCKS mode, you need to install the microsocks SOCKS5 proxy separately:

```bash
sudo dnstm socks install
```

Clients connect directly to the proxy without SSH authentication. This mode has more obvious network fingerprints and is recommended only for testing or temporary use.

## Configuration

### Global Config

The active provider is tracked in `/etc/dnstm/dnstm.conf`:

```
ACTIVE_PROVIDER="slipstream"
```

### Shadowsocks Config

Stored in `/etc/shadowsocks-slipstream/config.json`:

```json
{
    "server": "0.0.0.0",
    "server_port": 5302,
    "password": "<generated-password>",
    "method": "aes-256-gcm",
    "plugin": "/usr/local/bin/slipstream-server",
    "plugin_opts": "domain=t.example.com;cert=...;key=..."
}
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
# Uninstall Shadowsocks
sudo dnstm shadowsocks uninstall

# Uninstall Slipstream
sudo dnstm slipstream uninstall

# Uninstall DNSTT
sudo dnstm dnstt uninstall
```

The uninstall process removes:
- Provider service and binary
- Configuration files and credentials
- Firewall rules for that provider

SSH tunnel users and SOCKS proxy are managed separately and are not affected by provider uninstall. To remove them:

```bash
# Remove SOCKS proxy
sudo dnstm socks uninstall

# Remove SSH tunnel users and hardening
sudo dnstm ssh-users uninstall
```

If uninstalling the active provider while another is installed, dnstm automatically switches to the other provider.

## Building from Source

```bash
git clone https://github.com/net2share/dnstm.git
cd dnstm
go build -o dnstm .
```

## Architecture

```
/etc/dnstm/                    # Global config (active provider)
/etc/shadowsocks-slipstream/   # Shadowsocks config
/etc/slipstream/               # Slipstream config and TLS certs (shared with Shadowsocks)
/etc/dnstt/                    # DNSTT config and keys
/usr/local/bin/                # Provider binaries, microsocks
```

All providers use NAT PREROUTING rules to redirect port 53 to their respective ports:
- Shadowsocks: 53 → 5302
- Slipstream: 53 → 5301
- DNSTT: 53 → 5300

All provider services run under a shared `dnstm` system user.
