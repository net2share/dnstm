# Architecture

## Overview

dnstm manages DNS tunnel services on Linux servers. It supports three transport protocols (Slipstream, DNSTT, and VayDNS) and four backend types.

## Transport Types

| Transport    | Description                                                |
| ------------ | ---------------------------------------------------------- |
| `slipstream` | High-performance DNS tunnel with TLS encryption            |
| `dnstt`      | Classic DNS tunnel with Curve25519 encryption              |
| `vaydns`     | Next-gen DNS tunnel with Curve25519 keys and KCP transport |

Transports forward traffic to backends:

| Backend Type  | Description                        | Transport Support |
| ------------- | ---------------------------------- | ----------------- |
| `socks`       | Built-in SOCKS5 proxy (microsocks) | All               |
| `ssh`         | Built-in SSH server                | All               |
| `shadowsocks` | Shadowsocks server (SIP003 plugin) | Slipstream only   |
| `custom`      | Custom target address              | All               |

## Operating Modes

### Single-Tunnel Mode

```
┌─────────────────────────────────────────────────┐
│                    Server                        │
│                                                  │
│  Port 53 ──────────────────► Active Transport   │
│                                   :53            │
│                                     │            │
│                                     ▼            │
│                              Target Service      │
│                              (SSH/SOCKS/SS)      │
└─────────────────────────────────────────────────┘
```

- One transport handles DNS queries at a time
- Active transport binds directly to port 53 on the external IP
- Lower overhead (no router process, no NAT)
- Switch tunnels with `dnstm router switch -t <tag>`

### Multi-Tunnel Mode

> **Note:** Multi-mode overhead is typically minimal. Performance varies by transport and connection method. See [Benchmarks](BENCHMARKS-v0.5.0.md) for details.

```
┌─────────────────────────────────────────────────┐
│                    Server                        │
│                                                  │
│  Port 53 ──► DNS Router ──┬──► Transport 1      │
│                           │      :5310           │
│                           ├──► Transport 2      │
│                           │      :5311           │
│                           └──► Transport N      │
│                                  :531N           │
└─────────────────────────────────────────────────┘
```

- All transports run simultaneously
- DNS router on port 53 routes queries by domain
- Each transport runs on its own port (5310+)
- Domain-based routing

## Components

### Router (`/etc/dnstm/config.json`)

Central configuration managing:

- Operating mode (single/multi)
- Tunnels and backends
- Routing rules

### DNS Router Service (`dnstm-dnsrouter`)

Runs in multi-mode only. Listens on port 53 and routes DNS queries to appropriate tunnels.

### Tunnel Services (`dnstm-<tag>`)

Individual systemd services for each configured tunnel. Each runs on an auto-allocated port (5310+).

### Crypto Material (per-tunnel)

Each tunnel stores its cryptographic material in `/etc/dnstm/tunnels/<tag>/`:

**Slipstream** — TLS certificates:

- `cert.pem`, `key.pem` (ECDSA P-256, 10-year validity, self-signed)
- SHA256 fingerprints for client verification

**DNSTT** — Curve25519 key pairs:

- `server.key`, `server.pub` (64-character hex strings)
- Public key for client verification

**VayDNS** — Curve25519 key pairs:

- `server.key`, `server.pub` (same format as DNSTT)
- Public key for client verification
- Supports dnstt-compatible wire format for backward compatibility

## Directory Structure

```
/etc/dnstm/
├── config.json           # Main router configuration
├── certs/                # TLS certificates
│   ├── domain_cert.pem
│   └── domain_key.pem
├── keys/                 # DNSTT keys
│   ├── domain_server.key
│   └── domain_server.pub
└── tunnels/              # Per-tunnel configs
    └── <tag>/

/usr/local/bin/
├── dnstm                 # CLI binary
├── slipstream-server     # Slipstream binary
├── dnstt-server          # DNSTT binary
├── vaydns-server         # VayDNS binary
├── ssserver              # Shadowsocks binary
├── microsocks            # SOCKS proxy binary
└── sshtun-user           # SSH user management tool
```

## Service Management

All services run under the `dnstm` system user with:

- `PrivateTmp=true`
- `ProtectSystem=strict`
- `ProtectHome=true`
- `NoNewPrivileges=true`
- `AmbientCapabilities=CAP_NET_BIND_SERVICE`

## Firewall Integration

Supports:

- UFW
- firewalld
- iptables (direct)

Configures:

- Port 53 UDP/TCP for DNS
- Transport ports (5310+ for multi-mode backends)
