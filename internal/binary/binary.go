// Package binary provides binary download and management for external tools.
package binary

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/net2share/dnstm/internal/log"
	"github.com/ulikunitz/xz"
)

// BinaryType identifies a binary.
type BinaryType string

const (
	// Server binaries (used in production)
	BinaryDNSTTServer      BinaryType = "dnstt-server"
	BinarySlipstreamServer BinaryType = "slipstream-server"
	BinarySSServer         BinaryType = "ssserver"
	BinaryMicrosocks       BinaryType = "microsocks"
	BinarySSHTunUser       BinaryType = "sshtun-user"

	// Client binaries (used in testing)
	BinaryDNSTTClient      BinaryType = "dnstt-client"
	BinarySlipstreamClient BinaryType = "slipstream-client"
	BinarySSLocal          BinaryType = "sslocal"
)

// BinaryDef defines how to obtain a binary.
type BinaryDef struct {
	Type       BinaryType
	EnvVar     string              // Environment variable for custom path
	URLPattern string              // Download URL pattern with {os}, {arch} placeholders
	Archive    bool                // If true, URL points to an archive
	ArchiveDir string              // Directory inside archive where binary is located
	Platforms  map[string][]string // Supported os -> []arch
}

// DefaultBinaries contains definitions for all supported binaries.
var DefaultBinaries = map[BinaryType]BinaryDef{
	BinaryDNSTTServer: {
		Type:       BinaryDNSTTServer,
		EnvVar:     "DNSTM_DNSTT_SERVER_PATH",
		URLPattern: "https://github.com/net2share/dnstt/releases/download/latest/dnstt-server-{os}-{arch}{ext}",
		Platforms: map[string][]string{
			"linux":   {"amd64", "arm64"},
			"darwin":  {"amd64", "arm64"},
			"windows": {"amd64", "arm64"},
		},
	},
	BinaryDNSTTClient: {
		Type:       BinaryDNSTTClient,
		EnvVar:     "DNSTM_TEST_DNSTT_CLIENT_PATH",
		URLPattern: "https://github.com/net2share/dnstt/releases/download/latest/dnstt-client-{os}-{arch}{ext}",
		Platforms: map[string][]string{
			"linux":   {"amd64", "arm64"},
			"darwin":  {"amd64", "arm64"},
			"windows": {"amd64", "arm64"},
		},
	},
	BinarySlipstreamServer: {
		Type:       BinarySlipstreamServer,
		EnvVar:     "DNSTM_SLIPSTREAM_SERVER_PATH",
		URLPattern: "https://github.com/net2share/slipstream-rust-build/releases/download/v2026.02.05/slipstream-server-{os}-{arch}",
		Platforms: map[string][]string{
			"linux": {"amd64", "arm64"},
		},
	},
	BinarySlipstreamClient: {
		Type:       BinarySlipstreamClient,
		EnvVar:     "DNSTM_TEST_SLIPSTREAM_CLIENT_PATH",
		URLPattern: "https://github.com/net2share/slipstream-rust-build/releases/download/v2026.02.05/slipstream-client-{os}-{arch}",
		Platforms: map[string][]string{
			"linux": {"amd64", "arm64"},
		},
	},
	BinarySSServer: {
		Type:       BinarySSServer,
		EnvVar:     "DNSTM_SSSERVER_PATH",
		URLPattern: "https://github.com/shadowsocks/shadowsocks-rust/releases/download/v1.23.0/shadowsocks-v1.23.0.{ssarch}.tar.xz",
		Archive:    true,
		Platforms: map[string][]string{
			"linux":  {"amd64", "arm64"},
			"darwin": {"amd64", "arm64"},
		},
	},
	BinarySSLocal: {
		Type:       BinarySSLocal,
		EnvVar:     "DNSTM_TEST_SSLOCAL_PATH",
		URLPattern: "https://github.com/shadowsocks/shadowsocks-rust/releases/download/v1.23.0/shadowsocks-v1.23.0.{ssarch}.tar.xz",
		Archive:    true,
		Platforms: map[string][]string{
			"linux":  {"amd64", "arm64"},
			"darwin": {"amd64", "arm64"},
		},
	},
	BinaryMicrosocks: {
		Type:       BinaryMicrosocks,
		EnvVar:     "DNSTM_MICROSOCKS_PATH",
		URLPattern: "https://github.com/net2share/microsocks-build/releases/download/v1.0.5/microsocks-{microsocksarch}",
		Platforms: map[string][]string{
			"linux": {"amd64", "arm64"},
		},
	},
	BinarySSHTunUser: {
		Type:       BinarySSHTunUser,
		EnvVar:     "DNSTM_SSHTUN_USER_PATH",
		URLPattern: "https://github.com/net2share/sshtun-user/releases/download/v0.3.4/sshtun-user-linux-{arch}",
		Platforms: map[string][]string{
			"linux": {"amd64", "arm64"},
		},
	},
}

const (
	// DefaultInstallDir is the default directory for production binaries.
	DefaultInstallDir = "/usr/local/bin"
	// DefaultTestBinDir is the default directory for test binaries.
	DefaultTestBinDir = "tests/.testbin"
)

// Manager handles binary resolution and downloading.
type Manager struct {
	binDir string
	os     string
	arch   string
}

// NewManager creates a new binary manager with a specific directory.
func NewManager(binDir string) *Manager {
	return &Manager{
		binDir: binDir,
		os:     runtime.GOOS,
		arch:   runtime.GOARCH,
	}
}

// NewDefaultManager creates a binary manager that auto-detects the environment.
// In test mode, uses tests/.testbin. In production, uses /usr/local/bin.
func NewDefaultManager() *Manager {
	if isTestEnvironment() {
		return NewManager(getTestBinDir())
	}
	return NewManager(DefaultInstallDir)
}

// isTestEnvironment detects if we're running in a test environment.
func isTestEnvironment() bool {
	// Check if running under go test
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	// Check if binary name ends with .test
	if strings.HasSuffix(os.Args[0], ".test") {
		return true
	}
	return false
}

// getTestBinDir finds the test binary directory by looking for go.mod.
func getTestBinDir() string {
	dir, _ := os.Getwd()
	for dir != "/" {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, DefaultTestBinDir)
		}
		dir = filepath.Dir(dir)
	}
	return DefaultTestBinDir
}

// GetPath returns the path to an existing binary. Does NOT download.
// Resolution order:
// 1. Environment variable (if set and file exists)
// 2. Already in binDir
// Returns error if binary is not found.
func (m *Manager) GetPath(binType BinaryType) (string, error) {
	def, ok := DefaultBinaries[binType]
	if !ok {
		return "", fmt.Errorf("unknown binary type: %s", binType)
	}

	// Check if platform is supported
	if !m.isPlatformSupported(def) {
		return "", fmt.Errorf("binary %s not supported on %s/%s", binType, m.os, m.arch)
	}

	// Check environment variable first
	if def.EnvVar != "" {
		if envPath := os.Getenv(def.EnvVar); envPath != "" {
			if _, err := os.Stat(envPath); err == nil {
				return envPath, nil
			}
			return "", fmt.Errorf("env var %s set to %s but file not found", def.EnvVar, envPath)
		}
	}

	// Check if already in binDir
	binPath := filepath.Join(m.binDir, string(binType))
	if m.os == "windows" {
		binPath += ".exe"
	}
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	return "", fmt.Errorf("binary %s not found (run 'dnstm install' or set %s)", binType, def.EnvVar)
}

// EnsureInstalled ensures a binary is available, downloading if necessary.
// This should be called by install commands and test setup.
func (m *Manager) EnsureInstalled(binType BinaryType) (string, error) {
	def, ok := DefaultBinaries[binType]
	if !ok {
		return "", fmt.Errorf("unknown binary type: %s", binType)
	}

	// Check if platform is supported
	if !m.isPlatformSupported(def) {
		return "", fmt.Errorf("binary %s not supported on %s/%s", binType, m.os, m.arch)
	}

	// Check environment variable first
	if def.EnvVar != "" {
		if envPath := os.Getenv(def.EnvVar); envPath != "" {
			if _, err := os.Stat(envPath); err == nil {
				log.Debug("binary %s: using path from env var %s: %s", binType, def.EnvVar, envPath)
				return envPath, nil
			}
			return "", fmt.Errorf("env var %s set to %s but file not found", def.EnvVar, envPath)
		}
	}

	// Check if already in binDir
	binPath := filepath.Join(m.binDir, string(binType))
	if m.os == "windows" {
		binPath += ".exe"
	}
	if _, err := os.Stat(binPath); err == nil {
		log.Debug("binary %s: using cached binary: %s", binType, binPath)
		return binPath, nil
	}

	// Download if URL pattern is provided
	if def.URLPattern == "" {
		return "", fmt.Errorf("binary %s not found and no download URL available (install manually or set %s)", binType, def.EnvVar)
	}

	log.Debug("binary %s: downloading to %s", binType, binPath)
	if err := m.download(def, binPath); err != nil {
		return "", fmt.Errorf("failed to download %s: %w", binType, err)
	}

	return binPath, nil
}

// EnsureDir creates the binary directory if it doesn't exist.
func (m *Manager) EnsureDir() error {
	return os.MkdirAll(m.binDir, 0755)
}

// BinDir returns the binary directory path.
func (m *Manager) BinDir() string {
	return m.binDir
}

// isPlatformSupported checks if the binary is available for current platform.
func (m *Manager) isPlatformSupported(def BinaryDef) bool {
	archs, ok := def.Platforms[m.os]
	if !ok {
		return false
	}
	for _, a := range archs {
		if a == m.arch {
			return true
		}
	}
	return false
}


// download fetches a binary from its URL.
func (m *Manager) download(def BinaryDef, destPath string) error {
	if err := m.EnsureDir(); err != nil {
		return err
	}

	url := m.buildURL(def)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	if def.Archive {
		return m.extractFromArchive(resp.Body, def, destPath)
	}

	return m.saveToFile(resp.Body, destPath)
}

// buildURL constructs the download URL for current platform.
func (m *Manager) buildURL(def BinaryDef) string {
	url := def.URLPattern

	// Standard replacements
	url = strings.ReplaceAll(url, "{os}", m.os)
	url = strings.ReplaceAll(url, "{arch}", m.arch)

	// Windows extension
	ext := ""
	if m.os == "windows" {
		ext = ".exe"
	}
	url = strings.ReplaceAll(url, "{ext}", ext)

	// Shadowsocks uses different arch naming
	ssArch := m.getShadowsocksArch()
	url = strings.ReplaceAll(url, "{ssarch}", ssArch)

	// Microsocks uses different arch naming
	microsocksArch := m.getMicrosocksArch()
	url = strings.ReplaceAll(url, "{microsocksarch}", microsocksArch)

	return url
}

// getShadowsocksArch returns the shadowsocks-rust architecture string.
func (m *Manager) getShadowsocksArch() string {
	switch {
	case m.os == "linux" && m.arch == "amd64":
		return "x86_64-unknown-linux-gnu"
	case m.os == "linux" && m.arch == "arm64":
		return "aarch64-unknown-linux-gnu"
	case m.os == "darwin" && m.arch == "amd64":
		return "x86_64-apple-darwin"
	case m.os == "darwin" && m.arch == "arm64":
		return "aarch64-apple-darwin"
	case m.os == "windows" && m.arch == "amd64":
		return "x86_64-pc-windows-msvc"
	default:
		return fmt.Sprintf("%s-unknown-%s", m.arch, m.os)
	}
}

// getMicrosocksArch returns the microsocks architecture string.
func (m *Manager) getMicrosocksArch() string {
	libc := m.detectLibc()

	if libc == "glibc" {
		switch m.arch {
		case "amd64":
			return "x86_64-linux-gnu"
		case "arm64":
			return "aarch64-linux-gnu"
		}
	}

	// musl builds for Alpine or fallback
	switch m.arch {
	case "amd64":
		return "x86_64-linux-musl"
	case "arm64":
		return "aarch64-linux-musl"
	case "arm":
		return "arm-linux-musleabihf"
	case "386":
		return "i686-linux-musl"
	default:
		return "x86_64-linux-musl"
	}
}

// detectLibc detects whether the system uses glibc or musl.
func (m *Manager) detectLibc() string {
	// Check for Alpine Linux (uses musl)
	if _, err := os.Stat("/etc/alpine-release"); err == nil {
		return "musl"
	}

	// Check for glibc indicators
	if _, err := os.Stat("/lib/x86_64-linux-gnu"); err == nil {
		return "glibc"
	}
	if _, err := os.Stat("/lib/aarch64-linux-gnu"); err == nil {
		return "glibc"
	}
	if _, err := os.Stat("/lib64/ld-linux-x86-64.so.2"); err == nil {
		return "glibc"
	}

	// Default to glibc for most systems
	return "glibc"
}

// saveToFile writes content to a file with executable permissions.
func (m *Manager) saveToFile(r io.Reader, path string) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, r)
	return err
}

// extractFromArchive extracts a specific binary from a tar.xz archive.
func (m *Manager) extractFromArchive(r io.Reader, def BinaryDef, destPath string) error {
	// Decompress xz
	xzReader, err := xz.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create xz reader: %w", err)
	}

	// Read tar
	tarReader := tar.NewReader(xzReader)

	binaryName := string(def.Type)
	if m.os == "windows" {
		binaryName += ".exe"
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Look for the binary (may be in a subdirectory)
		baseName := filepath.Base(header.Name)
		if baseName == binaryName && header.Typeflag == tar.TypeReg {
			return m.saveToFile(tarReader, destPath)
		}
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}

// CopyToDir copies a binary from srcPath to the manager's binDir.
func (m *Manager) CopyToDir(srcPath string, binType BinaryType) (string, error) {
	if err := m.EnsureDir(); err != nil {
		return "", err
	}

	destName := string(binType)
	if m.os == "windows" {
		destName += ".exe"
	}
	destPath := filepath.Join(m.binDir, destName)

	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	return destPath, nil
}
