# Configuration

## Main Configuration

**Path**: `/etc/dnstm/config.yaml`

```yaml
version: "1"
mode: single  # or "multi"

single:
  active: instance1  # Active instance in single-mode

listen:
  address: "0.0.0.0:53"

transports:
  instance1:
    type: slipstream-shadowsocks
    domain: t1.example.com
    port: 5300
    shadowsocks:
      password: "generated-password"
      method: aes-256-gcm

  instance2:
    type: slipstream-ssh
    domain: t2.example.com
    port: 5301
    target:
      address: "127.0.0.1:22"

  instance3:
    type: dnstt-socks
    domain: t3.example.com
    port: 5302
    target:
      address: "127.0.0.1:1080"
    dnstt:
      mtu: 1232

routing:
  default: instance1

proxy:
  port: 1080  # Microsocks SOCKS5 proxy port
```

## Transport Types

### Slipstream + Shadowsocks

```yaml
type: slipstream-shadowsocks
domain: t.example.com
port: 5300
shadowsocks:
  password: "your-password"
  method: aes-256-gcm  # or chacha20-ietf-poly1305
```

### Slipstream SOCKS/SSH

```yaml
type: slipstream-socks  # or slipstream-ssh
domain: t.example.com
port: 5301
target:
  address: "127.0.0.1:1080"  # or 127.0.0.1:22 for SSH
```

### DNSTT SOCKS/SSH

```yaml
type: dnstt-socks  # or dnstt-ssh
domain: t.example.com
port: 5302
target:
  address: "127.0.0.1:1080"  # or 127.0.0.1:22 for SSH
dnstt:
  mtu: 1232  # 512-1400
```

## Directory Structure

```
/etc/dnstm/
├── config.yaml           # Main configuration
├── dnsrouter.yaml        # DNS router config (multi-mode)
├── certs/                # TLS certificates (Slipstream)
│   ├── domain_cert.pem
│   └── domain_key.pem
├── keys/                 # Curve25519 keys (DNSTT)
│   ├── domain_server.key
│   └── domain_server.pub
└── instances/            # Per-instance configs
    └── <name>/
```

## Certificates (Slipstream)

**Directory**: `/etc/dnstm/certs/`

Files named by domain (dots replaced with underscores):
- `t_example_com_cert.pem`
- `t_example_com_key.pem`

Properties:
- ECDSA P-256 algorithm
- 10-year validity
- Self-signed

View fingerprint:
```bash
dnstm instance status <name>
```

## Keys (DNSTT)

**Directory**: `/etc/dnstm/keys/`

Files named by domain:
- `t_example_com_server.key` (private)
- `t_example_com_server.pub` (public)

View public key:
```bash
dnstm instance status <name>
```

## Port Allocation

Ports auto-allocated starting from 5300:
- First instance: 5300
- Second instance: 5301
- etc.

Port 53 is used by:
- Active transport (single-mode, binds directly)
- DNS router (multi-mode)

## User and Permissions

Services run as `dnstm` system user:
- UID: auto-allocated
- Home: `/etc/dnstm`
- Shell: `/usr/sbin/nologin`

Directory permissions:
- `/etc/dnstm/` - 755
- `/etc/dnstm/certs/` - 750
- `/etc/dnstm/keys/` - 750
- `/etc/dnstm/instances/` - 750

## Firewall Rules

### UFW

```bash
ufw allow 53/udp
ufw allow 53/tcp
```

### firewalld

```bash
firewall-cmd --permanent --add-port=53/udp
firewall-cmd --permanent --add-port=53/tcp
```

## Binaries

Transport binaries are stored in `/usr/local/bin/`:
- `dnstm` - CLI tool
- `slipstream-server` - Slipstream transport
- `dnstt-server` - DNSTT transport
- `ssserver` - Shadowsocks server
- `microsocks` - SOCKS5 proxy
- `sshtun-user` - SSH user management tool
