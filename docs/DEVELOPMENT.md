# Development Guide

## Prerequisites

- Go 1.21 or later
- Linux environment (for testing)

## Building

Build for current platform:
```bash
go build -o dnstm .
```

Build with version information:
```bash
go build -ldflags="-X main.Version=dev -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o dnstm .
```

Cross-compile for different architectures:
```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o dnstm-linux-amd64 .

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o dnstm-linux-arm64 .

# Linux ARMv7
GOOS=linux GOARCH=arm GOARM=7 go build -o dnstm-linux-armv7 .

# Linux 386
GOOS=linux GOARCH=386 go build -o dnstm-linux-386 .
```

## Running

The application requires root privileges:
```bash
sudo ./dnstm
```

For development, you can test individual packages:
```bash
go test ./...
```

## Project Structure

```
dnstm/
├── main.go                 # Entry point
├── go.mod                  # Go module definition
├── install.sh              # Curl installer script
├── internal/
│   ├── app/               # Main application logic
│   ├── config/            # Configuration management
│   ├── download/          # Binary download and verification
│   ├── keys/              # Key generation
│   ├── network/           # Firewall configuration
│   ├── proxy/             # Dante proxy setup
│   ├── service/           # Systemd service management
│   ├── system/            # System detection utilities
│   └── ui/                # CLI interface
├── .github/
│   └── workflows/
│       ├── ci.yml         # CI on all branches
│       ├── release.yml    # Release on tags
│       └── release-please.yml  # Automatic versioning
└── docs/
    └── DEVELOPMENT.md     # This file
```

## Debugging

Run with verbose output:
```bash
sudo ./dnstm 2>&1 | tee debug.log
```

Check systemd service logs:
```bash
journalctl -u dnstt-server -f
```

Check firewall rules:
```bash
# iptables
sudo iptables -t nat -L -n -v

# UFW
sudo ufw status verbose

# firewalld
sudo firewall-cmd --list-all
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Run vet: `go vet ./...`
6. Commit using conventional commits
7. Push and create a pull request

### Conventional Commits

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automatic versioning:

- `feat:` - New feature (minor version bump)
- `fix:` - Bug fix (patch version bump)
- `docs:` - Documentation changes
- `chore:` - Maintenance tasks
- `refactor:` - Code refactoring
- `test:` - Test changes
- `ci:` - CI/CD changes

Examples:
```
feat: add IPv6 support for firewall rules
fix: correct key file permissions
docs: update client setup instructions
chore: update dependencies
```

Breaking changes (major version bump when v1.0.0+):
```
feat!: change configuration format
fix!: rename config keys
```

## Release Process

1. Merge PRs to main with conventional commit messages
2. Release Please automatically creates/updates a Release PR
3. When the Release PR is merged, a new release is created
4. GitHub Actions builds binaries and attaches them to the release
