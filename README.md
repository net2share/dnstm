# DNS Tunnel Manager (dnstm)

A tool to deploy and manage [dnstt](https://www.bamsoftware.com/software/dnstt/) DNS tunnel servers on Linux.

## Features

- Interactive menu and full CLI command support
- Downloads and installs dnstt-server binary
- Generates Curve25519 key pairs
- Configures firewall rules (UFW, firewalld, iptables)
- Sets up systemd service with security hardening
- SSH tunnel mode with integrated user management via [sshtun-user](https://github.com/net2share/sshtun-user)
- Optional Dante SOCKS proxy setup for SOCKS mode
- Supports multiple architectures (amd64, arm64, armv7, 386)

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

**When dnstt is not installed:**
1. Install dnstt server
2. Manage SSH tunnel users

**When dnstt is installed:**
1. Reconfigure dnstt server
2. Check service status
3. View service logs
4. Show configuration info
5. Restart service
6. Manage SSH tunnel users
7. Uninstall

### CLI Commands

```bash
# Show help
dnstm --help

# Install with interactive wizard
sudo dnstm install

# Install with CLI options (non-interactive)
sudo dnstm install --ns-subdomain t.example.com --mode ssh

# Check service status
sudo dnstm status

# View service logs
sudo dnstm logs

# Show current configuration
sudo dnstm config

# Restart the service
sudo dnstm restart

# Manage SSH tunnel users (opens submenu)
sudo dnstm ssh-users

# Uninstall (interactive - asks about SSH users)
sudo dnstm uninstall

# Uninstall and remove SSH tunnel users
sudo dnstm uninstall --remove-ssh-users

# Uninstall but keep SSH tunnel users
sudo dnstm uninstall --keep-ssh-users
```

### Install Options

| Option | Description |
| ------ | ----------- |
| `--ns-subdomain <domain>` | NS subdomain (e.g., t.example.com) |
| `--mtu <value>` | MTU value (512-1400, default: 1232) |
| `--mode <ssh\|socks>` | Tunnel mode (default: ssh) |
| `--port <port>` | Target port (default: 22 for ssh, 1080 for socks) |

### Global Options

| Option | Description |
| ------ | ----------- |
| `--help`, `-h` | Show help message |
| `--version`, `-v` | Show version |

## Tunnel Modes

### SSH Mode (default)

In SSH mode, dnstt tunnels SSH traffic. During installation, dnstm automatically:

1. Applies sshd hardening configuration
2. Configures fail2ban for brute-force protection
3. Prompts to create a restricted tunnel user

Tunnel users can only create local (`-L`) and SOCKS (`-D`) tunnels, with no shell access.

Manage SSH tunnel users anytime via `sudo dnstm ssh-users` or menu option 6.

### SOCKS Mode

In SOCKS mode, dnstt runs a Dante SOCKS5 proxy. Clients connect directly to the proxy without SSH.

## Configuration

Configuration is stored in `/etc/dnstt/dnstt-server.conf`:

```
NS_SUBDOMAIN="t.example.com"
MTU_VALUE="1232"
TUNNEL_MODE="ssh"
PRIVATE_KEY_FILE="/etc/dnstt/t_example_com_server.key"
PUBLIC_KEY_FILE="/etc/dnstt/t_example_com_server.pub"
TARGET_PORT="22"
```

## Client Setup

After server setup, connect using the dnstt client:

```bash
dnstt-client -udp RESOLVER_IP:53 -pubkey-file server.pub t.example.com 127.0.0.1:LOCAL_PORT
```

For SSH tunnel mode:

```bash
ssh -o ProxyCommand="dnstt-client -udp RESOLVER_IP:53 -pubkey-file server.pub t.example.com 127.0.0.1:8000" user@localhost
```

For SOCKS mode (with Dante):

```bash
dnstt-client -udp RESOLVER_IP:53 -pubkey-file server.pub t.example.com 127.0.0.1:1080
# Then configure your application to use SOCKS5 proxy at 127.0.0.1:1080
```

## Uninstall

```bash
# Interactive uninstall (asks about SSH tunnel users)
sudo dnstm uninstall

# Uninstall everything including SSH tunnel users and config
sudo dnstm uninstall --remove-ssh-users

# Uninstall dnstt but keep SSH tunnel users
sudo dnstm uninstall --keep-ssh-users
```

The uninstall process removes:
- dnstt-server service and binary
- Configuration files and keys
- Firewall rules
- dnstt system user
- (Optionally) SSH tunnel users and sshd hardening config

## Building from Source

```bash
git clone https://github.com/net2share/dnstm.git
cd dnstm
go build -o dnstm .
```
