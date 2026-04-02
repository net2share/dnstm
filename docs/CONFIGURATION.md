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
        "cert": "/etc/dnstm/tunnels/tunnel-1/cert.pem",
        "key": "/etc/dnstm/tunnels/tunnel-1/key.pem"
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
        "private_key": "/etc/dnstm/tunnels/tunnel-2/server.key"
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

With optional username/password authentication:

```json
{
  "tag": "socks",
  "type": "socks",
  "address": "127.0.0.1:1080",
  "socks": {
    "user": "myuser",
    "password": "mypass"
  }
}
```

Authentication can also be configured via CLI: `dnstm backend auth -t socks --user myuser --password mypass`

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
    "cert": "/etc/dnstm/tunnels/my-tunnel/cert.pem",
    "key": "/etc/dnstm/tunnels/my-tunnel/key.pem"
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
    "private_key": "/etc/dnstm/tunnels/my-tunnel/server.key"
  }
}
```

**Note:** DNSTT does not support the `shadowsocks` backend type.

### VayDNS

Next-generation DNS tunnel using Curve25519 keys with KCP transport.

```json
{
  "tag": "my-tunnel",
  "transport": "vaydns",
  "backend": "socks",
  "domain": "t.example.com",
  "port": 5312,
  "vaydns": {
    "mtu": 1232,
    "private_key": "/etc/dnstm/tunnels/my-tunnel/server.key",
    "idle_timeout": "10s",
    "keep_alive": "2s",
    "clientid_size": 2,
    "queue_size": 512
  }
}
```

VayDNS with dnstt-compatible wire format:

```json
{
  "tag": "my-compat-tunnel",
  "transport": "vaydns",
  "backend": "socks",
  "domain": "t.example.com",
  "port": 5313,
  "vaydns": {
    "mtu": 1232,
    "dnstt_compat": true
  }
}
```

**VayDNS configuration fields:**

| Field             | Type   | Default                              | Description                                                                                                  |
| ----------------- | ------ | ------------------------------------ | ------------------------------------------------------------------------------------------------------------ |
| `mtu`             | int    | 1232                                 | MTU size (512-1400)                                                                                          |
| `private_key`     | string | (auto-generated)                     | Path to Curve25519 private key                                                                               |
| `idle_timeout`    | string | `10s` (native) / `2m` (dnstt-compat) | Connection idle timeout                                                                                      |
| `keep_alive`      | string | `2s` (native) / `10s` (dnstt-compat) | Keepalive interval (must be < idle_timeout)                                                                  |
| `fallback`        | string | (empty)                              | UDP endpoint for non-DNS packets                                                                             |
| `dnstt_compat`    | bool   | false                                | Enable dnstt-compatible wire format (8-byte client IDs)                                                      |
| `clientid_size`   | int    | 2                                    | Client ID size in bytes (1-8, ignored when dnstt_compat is true)                                             |
| `queue_size`      | int    | 512                                  | Packet queue size (min 32)                                                                                   |
| `kcp_window_size` | int    | 0                                    | KCP window size (0 = queue_size/2, must be ≤ queue_size)                                                     |
| `queue_overflow`  | string | `drop`                               | Queue overflow strategy: `drop` or `block`                                                                   |
| `log_level`       | string | `info`                               | Server log level: `debug`, `info`, `warning`, `error`                                                        |
| `record_type`     | string | `txt`                                | DNS record type: `txt`, `cname`, `a`, `aaaa`, `mx`, `ns`, `srv` (must be `txt` when dnstt_compat is enabled) |

**Note:** VayDNS does not support the `shadowsocks` backend type.

## Transport-Backend Compatibility

| Transport  | socks | ssh | shadowsocks | custom |
| ---------- | ----- | --- | ----------- | ------ |
| slipstream | ✓     | ✓   | ✓           | ✓      |
| dnstt      | ✓     | ✓   | ✗           | ✓      |
| vaydns     | ✓     | ✓   | ✗           | ✓      |

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

| Field     | Description                                      |
| --------- | ------------------------------------------------ |
| `mode`    | Operating mode: `single` or `multi`              |
| `active`  | Active tunnel tag (single mode only)             |
| `default` | Default route for unmatched domains (multi mode) |

## Directory Structure

```
/etc/dnstm/
├── config.json           # Main configuration (JSON)
└── tunnels/              # Per-tunnel directories
    └── <tag>/
        ├── cert.pem      # TLS certificate (Slipstream)
        ├── key.pem       # TLS private key (Slipstream)
        ├── server.key    # Curve25519 private key (DNSTT, VayDNS)
        ├── server.pub    # Curve25519 public key (DNSTT, VayDNS)
        └── config.json   # Shadowsocks config for SIP003
```

## Certificates (Slipstream)

**Location**: `/etc/dnstm/tunnels/<tag>/cert.pem` and `key.pem`

Properties:

- ECDSA P-256 algorithm
- 10-year validity
- Self-signed
- Auto-generated per tunnel if not provided

View fingerprint:

```bash
dnstm tunnel status <tag>
```

## Keys (DNSTT, VayDNS)

**Location**: `/etc/dnstm/tunnels/<tag>/server.key` and `server.pub`

Auto-generated per tunnel if not provided. Both DNSTT and VayDNS use Curve25519 key pairs.

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
- `/etc/dnstm/tunnels/` - 750
- `/etc/dnstm/tunnels/<tag>/` - 750

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
- `vaydns-server` - VayDNS transport
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
      "domain": "t1.example.com",
      "port": 5310
    },
    {
      "tag": "my-vaydns",
      "transport": "vaydns",
      "backend": "socks",
      "domain": "t2.example.com",
      "port": 5311,
      "vaydns": {
        "mtu": 1232,
        "idle_timeout": "15s",
        "keep_alive": "3s"
      }
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

### Example Config (With SOCKS5 Authentication)

Enable username/password authentication on the built-in SOCKS5 proxy:

```json
{
  "backends": [
    {
      "tag": "socks",
      "type": "socks",
      "socks": {
        "user": "myuser",
        "password": "mypass"
      }
    }
  ],
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

The SOCKS5 credentials are applied to microsocks during `config load` and included in generated `dnst://` share URLs.
