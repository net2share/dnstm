# Configuration

## Main Configuration

**Path**: `/etc/dnstm/config.json`

```json
{
  "log": {
    "level": "info",
    "output": "",
    "timestamp": true
  },
  "listen": {
    "address": "0.0.0.0:53"
  },
  "proxy": {
    "port": 1080
  },
  "backends": [
    {
      "tag": "socks",
      "type": "socks",
      "address": "127.0.0.1:1080"
    },
    {
      "tag": "ssh",
      "type": "ssh",
      "address": "127.0.0.1:22"
    },
    {
      "tag": "ss-primary",
      "type": "shadowsocks",
      "shadowsocks": {
        "password": "generated-password",
        "method": "aes-256-gcm"
      }
    }
  ],
  "tunnels": [
    {
      "tag": "tunnel-1",
      "enabled": true,
      "transport": "slipstream",
      "backend": "ss-primary",
      "domain": "t1.example.com",
      "port": 5310,
      "slipstream": {
        "cert": "/etc/dnstm/certs/t1_example_com_cert.pem",
        "key": "/etc/dnstm/certs/t1_example_com_key.pem"
      }
    },
    {
      "tag": "tunnel-2",
      "enabled": true,
      "transport": "dnstt",
      "backend": "socks",
      "domain": "t2.example.com",
      "port": 5311,
      "dnstt": {
        "mtu": 1232,
        "private_key": "/etc/dnstm/keys/t2_example_com_server.key"
      }
    }
  ],
  "route": {
    "mode": "single",
    "active": "tunnel-1",
    "default": "tunnel-1"
  }
}
```

## Backend Types

### SOCKS5 Backend

Forward traffic to a SOCKS5 proxy (e.g., microsocks).

```json
{
  "tag": "socks",
  "type": "socks",
  "address": "127.0.0.1:1080"
}
```

### SSH Backend

Forward traffic to an SSH server.

```json
{
  "tag": "ssh",
  "type": "ssh",
  "address": "127.0.0.1:22"
}
```

### Shadowsocks Backend

Use Shadowsocks encryption (Slipstream only, via SIP003 plugin).

```json
{
  "tag": "ss-primary",
  "type": "shadowsocks",
  "shadowsocks": {
    "password": "your-password",
    "method": "aes-256-gcm"
  }
}
```

Supported methods:
- `aes-256-gcm` (recommended)
- `chacha20-ietf-poly1305`

### Custom Backend

Forward traffic to any custom address.

```json
{
  "tag": "web-server",
  "type": "custom",
  "address": "192.168.1.100:8080"
}
```

## Transport Types

### Slipstream

High-performance DNS tunnel with TLS encryption.

```json
{
  "tag": "my-tunnel",
  "transport": "slipstream",
  "backend": "ss-primary",
  "domain": "t.example.com",
  "port": 5310,
  "slipstream": {
    "cert": "/etc/dnstm/certs/t_example_com_cert.pem",
    "key": "/etc/dnstm/certs/t_example_com_key.pem"
  }
}
```

Slipstream supports all backend types including Shadowsocks.

### DNSTT

Classic DNS tunnel using Curve25519 keys.

```json
{
  "tag": "my-tunnel",
  "transport": "dnstt",
  "backend": "socks",
  "domain": "t.example.com",
  "port": 5311,
  "dnstt": {
    "mtu": 1232,
    "private_key": "/etc/dnstm/keys/t_example_com_server.key"
  }
}
```

**Note:** DNSTT does not support the `shadowsocks` backend type.

## Transport-Backend Compatibility

| Transport | socks | ssh | shadowsocks | custom |
|-----------|-------|-----|-------------|--------|
| slipstream | ✓ | ✓ | ✓ | ✓ |
| dnstt | ✓ | ✓ | ✗ | ✓ |

## Route Configuration

```json
{
  "route": {
    "mode": "single",
    "active": "tunnel-1",
    "default": "tunnel-1"
  }
}
```

| Field | Description |
|-------|-------------|
| `mode` | Operating mode: `single` or `multi` |
| `active` | Active tunnel tag (single mode only) |
| `default` | Default route for unmatched domains (multi mode) |

## Directory Structure

```
/etc/dnstm/
├── config.json           # Main configuration (JSON)
├── dnsrouter.yaml        # DNS router config (multi-mode, internal)
├── certs/                # TLS certificates (Slipstream)
│   ├── t_example_com_cert.pem
│   └── t_example_com_key.pem
├── keys/                 # Curve25519 keys (DNSTT)
│   ├── t_example_com_server.key
│   └── t_example_com_server.pub
└── tunnels/              # Per-tunnel configs
    └── <tag>/
        └── config.json   # Shadowsocks config for SIP003
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
dnstm tunnel status <tag>
```

## Keys (DNSTT)

**Directory**: `/etc/dnstm/keys/`

Files named by domain:
- `t_example_com_server.key` (private)
- `t_example_com_server.pub` (public)

View public key:
```bash
dnstm tunnel status <tag>
```

## Port Allocation

Ports auto-allocated starting from 5310:
- First tunnel: 5310
- Second tunnel: 5311
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
- `/etc/dnstm/tunnels/` - 750

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

## Config Management Commands

```bash
# Export current config to file
dnstm config export -o backup.json

# Validate a config file
dnstm config validate backup.json

# Load config from file
dnstm config load backup.json
```

## Loading Configuration from File

The `config load` command provides a quick way to deploy a complete configuration.

**Prerequisites:** Run `dnstm install` first to set up the system user, directories, and services.

### Behavior

1. **Cleanup**: Existing tunnel services are stopped and removed
2. **Validation**: Config file is validated before applying
3. **Crypto Material**:
   - If cert/key paths are provided, they are validated (must exist and be readable by dnstm user)
   - If no paths provided, certificates (Slipstream) or keys (DNSTT) are auto-generated
4. **Services**: Tunnel services are created and the router is started automatically
5. **Output**: Displays connection info (fingerprints/public keys) and file paths

### Example Workflow

```bash
# 1. Install dnstm
dnstm install --mode multi

# 2. Load config (tunnels start immediately)
dnstm config load config.json
```

### Example Config (No Cert/Key Paths)

When cert/key paths are omitted, they are auto-generated:

```json
{
  "tunnels": [
    {
      "tag": "my-slip",
      "transport": "slipstream",
      "backend": "socks",
      "domain": "t.example.com",
      "port": 5310
    }
  ],
  "route": {
    "mode": "multi"
  }
}
```

### Example Config (With Existing Certs)

Provide paths to use existing certificates:

```json
{
  "tunnels": [
    {
      "tag": "my-slip",
      "transport": "slipstream",
      "backend": "socks",
      "domain": "t.example.com",
      "port": 5310,
      "slipstream": {
        "cert": "/path/to/cert.pem",
        "key": "/path/to/key.pem"
      }
    }
  ],
  "route": {
    "mode": "multi"
  }
}
```

**Note:** Both `cert` and `key` must be provided together. Files must be readable by the dnstm user.
