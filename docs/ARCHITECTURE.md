# Architecture

## Overview

dnstm manages DNS tunnel services on Linux servers. It supports two transport protocols (Slipstream and DNSTT) with five transport types.

## Transport Types

| Type | Protocol | Target | Use Case |
|------|----------|--------|----------|
| `slipstream-shadowsocks` | Slipstream | Shadowsocks | Encrypted proxy with obfuscation |
| `slipstream-socks` | Slipstream | SOCKS proxy | Direct SOCKS5 access |
| `slipstream-ssh` | Slipstream | SSH server | SSH tunneling |
| `dnstt-socks` | DNSTT | SOCKS proxy | Direct SOCKS5 access |
| `dnstt-ssh` | DNSTT | SSH server | SSH tunneling |

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
- Switch transports with `dnstm switch <name>`

### Multi-Tunnel Mode

> **Note:** Slipstream transports may have ~20% lower throughput in multi-mode due to DNS router overhead.

```
┌─────────────────────────────────────────────────┐
│                    Server                        │
│                                                  │
│  Port 53 ──► DNS Router ──┬──► Transport 1      │
│                           │      :5300           │
│                           ├──► Transport 2      │
│                           │      :5301           │
│                           └──► Transport N      │
│                                  :530N           │
└─────────────────────────────────────────────────┘
```

- All transports run simultaneously
- DNS router on port 53 routes queries by domain
- Each transport runs on its own port (5300+)
- Domain-based routing

## Components

### Router (`/etc/dnstm/config.yaml`)

Central configuration managing:
- Operating mode (single/multi)
- Transport instances
- Routing rules

### DNS Router Service (`dnstm-dnsrouter`)

Runs in multi-mode only. Listens on port 53 and routes DNS queries to appropriate transport instances.

### Transport Instances (`dnstm-<name>`)

Individual systemd services for each configured transport. Each runs on an auto-allocated port (5300+).

### Certificate Manager (`/etc/dnstm/certs/`)

Manages TLS certificates for Slipstream transports:
- ECDSA P-256 keys
- 10-year validity
- SHA256 fingerprints for client verification

### Key Manager (`/etc/dnstm/keys/`)

Manages Curve25519 key pairs for DNSTT transports:
- 64-character hex strings
- Public key for client verification

## Directory Structure

```
/etc/dnstm/
├── config.yaml           # Main router configuration
├── dnsrouter.yaml        # DNS router config (multi-mode)
├── certs/                # TLS certificates
│   ├── domain_cert.pem
│   └── domain_key.pem
├── keys/                 # DNSTT keys
│   ├── domain_server.key
│   └── domain_server.pub
└── instances/            # Per-instance configs
    └── <name>/

/usr/local/bin/
├── dnstm                 # CLI binary
├── slipstream-server     # Slipstream binary
├── dnstt-server          # DNSTT binary
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
- Transport ports (5300+ for multi-mode backends)
