# Client Setup

## Prerequisites

Download client binaries:
- [slipstream-client](https://github.com/Mygod/slipstream-rust/releases)
- [dnstt-client](https://www.bamsoftware.com/software/dnstt/)
- [sslocal](https://github.com/shadowsocks/shadowsocks-rust/releases) (for Shadowsocks)

## Connection Info

Get connection details from the server:

```bash
dnstm instance config <name>
```

This shows:
- Domain
- Port
- Certificate fingerprint (Slipstream)
- Public key (DNSTT)
- Password (Shadowsocks)

## Slipstream + Shadowsocks

### 1. Copy Certificate

```bash
scp root@server:/etc/dnstm/certs/domain_cert.pem ./
```

### 2. Connect with sslocal

```bash
sslocal \
  -s "127.0.0.1:5310" \
  -b "127.0.0.1:1080" \
  -m "aes-256-gcm" \
  -k "PASSWORD" \
  --plugin "slipstream-client" \
  --plugin-opts "domain=DOMAIN;resolver=1.1.1.1:53;cert=/path/to/cert.pem"
```

### 3. Test

```bash
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

## Slipstream SOCKS

### 1. Copy Certificate

```bash
scp root@server:/etc/dnstm/certs/domain_cert.pem ./
```

### 2. Connect

```bash
slipstream-client \
  --domain DOMAIN \
  --resolver 1.1.1.1:53 \
  --tcp-listen-port 1080 \
  --cert /path/to/cert.pem
```

### 3. Test

```bash
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

## Slipstream SSH

### 1. Copy Certificate

```bash
scp root@server:/etc/dnstm/certs/domain_cert.pem ./
```

### 2. Start Client

```bash
slipstream-client \
  --domain DOMAIN \
  --resolver 1.1.1.1:53 \
  --tcp-listen-port 2222 \
  --cert /path/to/cert.pem
```

### 3. SSH Through Tunnel

```bash
ssh -p 2222 user@127.0.0.1
```

### 4. SOCKS Proxy via SSH

```bash
ssh -D 1080 -p 2222 user@127.0.0.1
```

Then use `127.0.0.1:1080` as SOCKS5 proxy.

## DNSTT SOCKS

### 1. Get Public Key

From server:
```bash
dnstm instance config <name>
```

Or copy the key file:
```bash
scp root@server:/etc/dnstm/keys/domain_server.pub ./
```

### 2. Connect

```bash
dnstt-client \
  -udp 1.1.1.1:53 \
  -pubkey PUBLIC_KEY \
  DOMAIN \
  127.0.0.1:1080
```

Or with key file:
```bash
dnstt-client \
  -udp 1.1.1.1:53 \
  -pubkey-file server.pub \
  DOMAIN \
  127.0.0.1:1080
```

### 3. Test

```bash
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

## DNSTT SSH

### 1. Get Public Key

```bash
dnstm instance config <name>
```

### 2. SSH via ProxyCommand

```bash
ssh -o ProxyCommand="dnstt-client -udp 1.1.1.1:53 -pubkey PUBLIC_KEY DOMAIN 127.0.0.1:%p" user@localhost
```

### 3. SOCKS Proxy via SSH

```bash
ssh -D 1080 -o ProxyCommand="dnstt-client -udp 1.1.1.1:53 -pubkey PUBLIC_KEY DOMAIN 127.0.0.1:%p" user@localhost
```

## DNS Resolvers

Use any public DNS resolver:
- `1.1.1.1` (Cloudflare)
- `8.8.8.8` (Google)
- `9.9.9.9` (Quad9)

TCP mode (if UDP blocked):
- Slipstream: `--resolver 1.1.1.1:53` (auto-detects TCP)
- DNSTT: `-dot 1.1.1.1:853` (DNS-over-TLS)

## Troubleshooting

### Connection Timeout

1. Check DNS records resolve correctly:
   ```bash
   dig +short NS t.example.com
   ```

2. Verify server is running:
   ```bash
   dnstm router status
   ```

3. Check server logs:
   ```bash
   dnstm instance logs <name>
   ```

### Certificate Mismatch

Copy the latest certificate from server:
```bash
scp root@server:/etc/dnstm/certs/domain_cert.pem ./
```

### Wrong Public Key

Get the correct key:
```bash
dnstm instance config <name>
```

### Slow Connection

DNSTT: Try increasing MTU (up to 1400):
```yaml
dnstt:
  mtu: 1400
```

Slipstream generally performs better than DNSTT.
