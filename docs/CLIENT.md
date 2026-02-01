# Client Setup

## Prerequisites

Download client binaries:
- [slipstream-client](https://github.com/Mygod/slipstream-rust/releases)
- [dnstt-client](https://www.bamsoftware.com/software/dnstt/)
- [sslocal](https://github.com/shadowsocks/shadowsocks-rust/releases) (for Shadowsocks)

## Connection Info

Get connection details from the server:

```bash
dnstm tunnel status <name>
```

This shows:
- Domain
- Port
- Certificate fingerprint (Slipstream)
- Public key (DNSTT)
- Password and method (Shadowsocks)

## Certificate/Key Files

Certificate and key files are stored on the server:
- Slipstream certificates: `/etc/dnstm/certs/<domain>_cert.pem`
- DNSTT public keys: `/etc/dnstm/keys/<domain>_server.pub`

Domain names use underscores (e.g., `a.example.com` → `a_example_com_cert.pem`).

## Slipstream + Shadowsocks

### 1. Get Connection Info

```bash
# On server
dnstm tunnel status <name>
```

Note the domain, password, and encryption method.

### 2. Copy Certificate

```bash
scp root@server:/etc/dnstm/certs/<domain>_cert.pem ./cert.pem
```

### 3. Start Tunnel and Connect

```bash
# Start slipstream tunnel (creates local TCP port)
slipstream-client -d DOMAIN -r 8.8.8.8:53 --cert cert.pem -l 5201 &

# Connect sslocal through the tunnel
sslocal -s 127.0.0.1:5201 -k "PASSWORD" -m METHOD -b 127.0.0.1:1080
```

### 4. Test

```bash
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

## Slipstream SOCKS

### 1. Copy Certificate

```bash
scp root@server:/etc/dnstm/certs/<domain>_cert.pem ./cert.pem
```

### 2. Connect

```bash
slipstream-client -d DOMAIN -r 8.8.8.8:53 --cert cert.pem -l 1080
```

The tunnel acts directly as a SOCKS5 proxy (connects to microsocks on server).

### 3. Test

```bash
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

## Slipstream SSH

### 1. Copy Certificate

```bash
scp root@server:/etc/dnstm/certs/<domain>_cert.pem ./cert.pem
```

### 2. Start Tunnel

```bash
slipstream-client -d DOMAIN -r 8.8.8.8:53 --cert cert.pem -l 2222
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

### 5. Test with curl

```bash
# Start SSH with dynamic port forwarding in background
ssh -D 1080 -f -N -p 2222 user@127.0.0.1

# Test connection
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

## DNSTT SOCKS

### 1. Get Public Key

From server:
```bash
dnstm tunnel status <name>
```

Copy the public key (64 hex digits).

### 2. Connect

```bash
dnstt-client -udp 8.8.8.8:53 -pubkey PUBLIC_KEY DOMAIN 127.0.0.1:1080
```

Or with key file:
```bash
scp root@server:/etc/dnstm/keys/<domain>_server.pub ./
dnstt-client -udp 8.8.8.8:53 -pubkey-file <domain>_server.pub DOMAIN 127.0.0.1:1080
```

### 3. Test

```bash
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

## DNSTT SSH

### 1. Get Public Key

```bash
dnstm tunnel status <name>
```

### 2. Start Tunnel

```bash
dnstt-client -udp 8.8.8.8:53 -pubkey PUBLIC_KEY DOMAIN 127.0.0.1:2222
```

### 3. SSH Through Tunnel

```bash
ssh -p 2222 user@127.0.0.1
```

### 4. Alternative: SSH via ProxyCommand

```bash
ssh -o ProxyCommand="dnstt-client -udp 8.8.8.8:53 -pubkey PUBLIC_KEY DOMAIN 127.0.0.1:%p" user@localhost
```

### 5. SOCKS Proxy via SSH

```bash
ssh -D 1080 -p 2222 user@127.0.0.1
```

### 6. Test with curl

```bash
# Start SSH with dynamic port forwarding in background
ssh -D 1080 -f -N -p 2222 user@127.0.0.1

# Test connection
curl -x socks5h://127.0.0.1:1080 https://httpbin.org/ip
```

## DNS Resolvers

Use any public DNS resolver. Recommended order:
- `8.8.8.8` (Google) - most reliable
- `9.9.9.9` (Quad9)
- `1.1.1.1` (Cloudflare)

If UDP is blocked, use DNS-over-TLS or DNS-over-HTTPS:
- DNSTT: `-dot 8.8.8.8:853` or `-doh https://dns.google/dns-query`

## Troubleshooting

### Connection Timeout

1. Verify server is running:
   ```bash
   dnstm router status
   ```

2. Check server logs:
   ```bash
   dnstm tunnel logs <name>
   ```

3. Try a different DNS resolver (8.8.8.8 vs 1.1.1.1)

### Certificate Mismatch (Slipstream)

Copy the latest certificate from server:
```bash
scp root@server:/etc/dnstm/certs/<domain>_cert.pem ./cert.pem
```

### Wrong Public Key (DNSTT)

Get the correct key:
```bash
dnstm tunnel status <name>
```

### Slow Connection

DNSTT is slower than Slipstream due to protocol overhead. For better performance, use Slipstream transports.

### Slipstream Connection Disconnects

Check the client output for errors. Common issues:
- Certificate mismatch: re-copy the certificate
- DNS propagation: try a different resolver
- Server not running: check `dnstm router status`
