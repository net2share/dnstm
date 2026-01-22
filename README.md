# DNS Tunnel Manager (dnstm)

A tool to deploy and manage [dnstt](https://www.bamsoftware.com/software/dnstt/) DNS tunnel servers on Linux.

## Features

- Interactive CLI wizard for easy setup
- Downloads and installs dnstt-server binary
- Generates Curve25519 key pairs
- Configures firewall rules (UFW, firewalld, iptables)
- Sets up systemd service with security hardening
- Optional Dante SOCKS proxy setup
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

Run the tool as root:

```bash
sudo dnstm
```

The interactive menu provides options to:

1. **Install/Update dnstt** - Download the latest dnstt-server binary
2. **Configure** - Set up domain, keys, and tunnel mode
3. **Start/Stop/Restart** - Manage the dnstt service
4. **View Status** - Check service status and configuration
5. **Setup Dante Proxy** - Optional SOCKS5 proxy for SSH tunneling

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

## Building from Source

```bash
git clone https://github.com/net2share/dnstm.git
cd dnstm
go build -o dnstm .
```

## License

MIT
