# Testing Guide

This document describes the testing strategy for dnstm.

## Overview

dnstm uses a three-tier testing approach:

| Level | Purpose | Dependencies | Speed |
|-------|---------|--------------|-------|
| Unit | Test individual functions | None | Fast (~1s) |
| Integration | Test CLI commands with mocked systemd | None | Fast (~1s) |
| E2E | Test actual tunnel connectivity | Auto-downloaded | Slow (~15s+) |

## Test File Naming

- `*_test.go` - Test files live alongside the code they test
- `internal/testutil/` - Shared test utilities
- `tests/integration/` - Integration tests for CLI commands
- `tests/e2e/` - End-to-end tests with real binaries

## Running Tests

| Command | What it runs | Requirements |
|---------|--------------|--------------|
| `make test` | Unit + Integration | None |
| `make test-unit` | Unit tests only | None |
| `make test-integration` | Integration tests only | None |
| `make test-e2e` | E2E tests | Internet (first run) |
| `make test-all` | Unit + Integration + E2E | Internet (first run) |
| `make test-coverage` | Unit + Integration with coverage | None |
| `make test-real` | All tests with real systemd | Root |

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
- `sslocal`, `ssserver` from [shadowsocks-rust](https://github.com/shadowsocks/shadowsocks-rust/releases)

## Binary Management

### Automatic Download

On first E2E test run, binaries are downloaded to `tests/.testbin/`. Subsequent runs use cached binaries.

### Custom Binary Paths

Use environment variables to override binary paths:

**Server binaries (used in production and tests):**

| Variable | Binary |
|----------|--------|
| `DNSTM_DNSTT_SERVER_PATH` | dnstt-server |
| `DNSTM_SLIPSTREAM_SERVER_PATH` | slipstream-server |
| `DNSTM_SSSERVER_PATH` | ssserver |
| `DNSTM_MICROSOCKS_PATH` | microsocks |

**Client binaries (test only):**

| Variable | Binary |
|----------|--------|
| `DNSTM_TEST_DNSTT_CLIENT_PATH` | dnstt-client |
| `DNSTM_TEST_SLIPSTREAM_CLIENT_PATH` | slipstream-client |
| `DNSTM_TEST_SSLOCAL_PATH` | sslocal |

Example:
```bash
export DNSTM_TEST_DNSTT_CLIENT_PATH=/usr/local/bin/dnstt-client
make test-e2e
```

### Supported Platforms

| Binary | Linux | macOS | Windows |
|--------|-------|-------|---------|
| dnstt-* | amd64, arm64 | amd64, arm64 | amd64, arm64 |
| slipstream-* | amd64, arm64 | - | - |
| ss* | amd64, arm64 | amd64, arm64 | - |
| microsocks | manual install | manual install | - |

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
          go-version: '1.24'

      - name: Run unit tests
        run: make test-unit

      - name: Run integration tests
        run: make test-integration

      - name: Run E2E tests
        run: make test-e2e
```
