# Testing Guide

This document describes the testing strategy for dnstm.

## Overview

dnstm uses a three-tier testing approach:

| Level       | Purpose                               | Dependencies    | Speed        |
| ----------- | ------------------------------------- | --------------- | ------------ |
| Unit        | Test individual functions             | None            | Fast (~1s)   |
| Integration | Test CLI commands with mocked systemd | None            | Fast (~1s)   |
| E2E         | Test actual tunnel connectivity       | Auto-downloaded | Slow (~15s+) |

## Test File Naming

- `*_test.go` - Test files live alongside the code they test
- `internal/testutil/` - Shared test utilities
- `tests/integration/` - Integration tests for CLI commands
- `tests/e2e/` - End-to-end tests with real binaries

## Running Tests

| Command                 | What it runs                     | Requirements         |
| ----------------------- | -------------------------------- | -------------------- |
| `make test`             | Unit + Integration               | None                 |
| `make test-unit`        | Unit tests only                  | None                 |
| `make test-integration` | Integration tests only           | None                 |
| `make test-e2e`         | E2E tests                        | Internet (first run) |
| `make test-all`         | Unit + Integration + E2E         | Internet (first run) |
| `make test-coverage`    | Unit + Integration with coverage | None                 |
| `make test-real`        | All tests with real systemd      | Root                 |

### Unit Tests

```bash
make test-unit

# Or specific packages
go test -v ./internal/config/...
go test -v ./internal/keys/...
```

### Integration Tests

```bash
make test-integration
```

Uses `MockSystemdManager` - no root privileges or external binaries required.

### E2E Tests

```bash
make test-e2e
```

E2E tests automatically download required binaries on first run to `tests/.testbin/`.

**Binaries downloaded:**

- `dnstt-client`, `dnstt-server` from [net2share/dnstt](https://github.com/net2share/dnstt/releases)
- `slipstream-client`, `slipstream-server` from [net2share/slipstream-rust-build](https://github.com/net2share/slipstream-rust-build/releases)
- `vaydns-client`, `vaydns-server` from [net2share/vaydns](https://github.com/net2share/vaydns/releases)
- `sslocal`, `ssserver` from [shadowsocks-rust](https://github.com/shadowsocks/shadowsocks-rust/releases)

## Binary Management

### Automatic Download

On first E2E test run, binaries are downloaded to `tests/.testbin/`. Subsequent runs use cached binaries.

### Custom Binary Paths

Use environment variables to override binary paths:

**Server binaries (used in production and tests):**

| Variable                       | Binary            |
| ------------------------------ | ----------------- |
| `DNSTM_DNSTT_SERVER_PATH`      | dnstt-server      |
| `DNSTM_SLIPSTREAM_SERVER_PATH` | slipstream-server |
| `DNSTM_VAYDNS_SERVER_PATH`     | vaydns-server     |
| `DNSTM_SSSERVER_PATH`          | ssserver          |
| `DNSTM_MICROSOCKS_PATH`        | microsocks        |

**Client binaries (test only):**

| Variable                            | Binary            |
| ----------------------------------- | ----------------- |
| `DNSTM_TEST_DNSTT_CLIENT_PATH`      | dnstt-client      |
| `DNSTM_TEST_SLIPSTREAM_CLIENT_PATH` | slipstream-client |
| `DNSTM_TEST_VAYDNS_CLIENT_PATH`     | vaydns-client     |
| `DNSTM_TEST_SSLOCAL_PATH`           | sslocal           |

Example:

```bash
export DNSTM_TEST_DNSTT_CLIENT_PATH=/usr/local/bin/dnstt-client
make test-e2e
```

### Supported Platforms

| Binary        | Linux          | macOS          | Windows      |
| ------------- | -------------- | -------------- | ------------ |
| dnstt-\*      | amd64, arm64   | amd64, arm64   | amd64, arm64 |
| slipstream-\* | amd64, arm64   | -              | -            |
| vaydns-\*     | amd64, arm64   | amd64, arm64   | amd64        |
| ss\*          | amd64, arm64   | amd64, arm64   | -            |
| microsocks    | manual install | manual install | -            |

## Troubleshooting

### Binary Download Fails

Check internet connectivity. The test will print which binary failed to download.

To use local binaries instead:

```bash
export DNSTM_TEST_DNSTT_CLIENT_PATH=/path/to/dnstt-client
export DNSTM_DNSTT_SERVER_PATH=/path/to/dnstt-server
```

### microsocks Not Available

microsocks must be installed manually (no auto-download):

```bash
# Fedora/RHEL
sudo dnf install microsocks

# Debian/Ubuntu
sudo apt install microsocks

# From source
git clone https://github.com/rofl0r/microsocks
cd microsocks && make && sudo make install
```

### Port Already in Use

```bash
lsof -i :5310
```

### E2E Tests Timeout

```bash
go test -v -timeout 10m ./tests/e2e/...
```

## Remote E2E Tests

The script `scripts/remote-e2e.sh` automates the full remote testing workflow against a server with dnstm deployed. It builds, deploys, installs, creates tunnels, and validates connectivity across all transport/backend combinations.

### Prerequisites

Local machine:

- `go` (for building the binary)
- `jq`, `curl`
- `slipstream-client`, `dnstt-client`, `sslocal` in `$PATH`
- SSH access to the target server

Remote server:

- NS records pointing test domains to the server IP
- SSH root access

### Setup

Copy the example config and fill in your SSH target, domains, and credentials:

```bash
cp scripts/e2e-config.json.example scripts/e2e-config.json
```

```json
{
  "ssh_target": "user@host-or-ssh-alias",
  "dns_resolver": "8.8.8.8",
  "domains": {
    "dnstt_socks": "dnstt-socks.example.com",
    "dnstt_ssh": "dnstt-ssh.example.com",
    "slip_socks": "slip-socks.example.com",
    "slip_ssh": "slip-ssh.example.com",
    "slip_ss": "slip-ss.example.com",
    "vaydns_socks": "vaydns-socks.example.com",
    "vaydns_ssh": "vaydns-ssh.example.com"
  },
  "shadowsocks": {
    "multi": { "method": "aes-256-gcm", "password": "..." },
    "single": { "method": "chacha20-ietf-poly1305", "password": "..." }
  },
  "socks_auth": {
    "user": "your-socks-user",
    "password": "your-socks-password"
  }
}
```

- `ssh_target`: SSH config alias or `user@host` (required)
- `dns_resolver`: public DNS resolver for client connections (default: `8.8.8.8`)
- `socks_auth`: SOCKS5 authentication credentials for auth tests (optional, enables auth tests in `single` and `config-reload` phases)

### Usage

```bash
# Run all phases
./scripts/remote-e2e.sh

# Custom config file
./scripts/remote-e2e.sh -c my-config.json

# Run only a specific phase
./scripts/remote-e2e.sh --phase multi
```

### Phases

| Phase           | What it tests                                                                                                                                 |
| --------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `single`        | Fresh install, `tunnel add` for all tunnel types, each tested individually in single mode, SOCKS auth enable/disable with multiple transports |
| `multi`         | Setup multi-mode state via `config load`, test 3 tunnels simultaneously                                                                       |
| `mode-switch`   | Switch multi→single→multi, verify tunnels work after each switch                                                                              |
| `config-load`   | Clean reinstall, `config load` with multi config, verify tunnels work                                                                         |
| `config-reload` | `config load` single config (with SOCKS auth) over existing multi, validate cleanup and auth enforcement                                      |

Each phase is standalone — it sets up its own prerequisite state and can be run independently via `--phase`.

### Output

After all phases complete, the script generates `temp/e2e-connections.md` with connection commands for each tunnel on the server, along with fetched certs/pubkeys in `temp/certs/`.

### Tunnel types tested

| Tag          | Transport  | Backend     | Test method                            |
| ------------ | ---------- | ----------- | -------------------------------------- |
| slip-socks   | Slipstream | SOCKS       | `slipstream-client` → curl             |
| slip-ssh     | Slipstream | SSH         | `slipstream-client` → `ssh -D` → curl  |
| slip-ss      | Slipstream | Shadowsocks | `slipstream-client` → `sslocal` → curl |
| dnstt-socks  | DNSTT      | SOCKS       | `dnstt-client` → curl                  |
| dnstt-ssh    | DNSTT      | SSH         | `dnstt-client` → `ssh -D` → curl       |
| vaydns-socks | VayDNS     | SOCKS       | `vaydns-client` → curl                 |
| vaydns-ssh   | VayDNS     | SSH         | `vaydns-client` → `ssh -D` → curl      |

All tests validate that `curl -x socks5h://127.0.0.1:PORT https://httpbin.org/ip` returns the server's public IP.

### Remote E2E Notes

- Each test uses a unique local port (incrementing from 10800) to avoid conflicts
- Certificates and public keys are fetched from the remote and cached per phase
- Client processes are automatically cleaned up after each test and on script exit
- Individual test failures are counted but don't abort the run

## CI Integration

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"

      - name: Run unit tests
        run: make test-unit

      - name: Run integration tests
        run: make test-integration

      - name: Run E2E tests
        run: make test-e2e
```
