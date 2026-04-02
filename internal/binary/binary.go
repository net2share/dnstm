// Package binary provides binary download and management for external tools.
package binary

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/net2share/dnstm/internal/log"
	"github.com/net2share/go-corelib/binman"
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
	BinaryVayDNSServer          BinaryType = "vaydns-server"
	BinarySlipstreamPlusServer  BinaryType = "slipstream-plus-server"

	// Client binaries (used in testing)
	BinaryDNSTTClient           BinaryType = "dnstt-client"
	BinarySlipstreamClient      BinaryType = "slipstream-client"
	BinarySSLocal               BinaryType = "sslocal"
	BinaryVayDNSClient          BinaryType = "vaydns-client"
	BinarySlipstreamPlusClient  BinaryType = "slipstream-plus-client"
)

// BinaryDef defines how to obtain a binary.
type BinaryDef struct {
	Type          BinaryType
	EnvVar        string              // Environment variable for custom path
	URLPattern    string              // Download URL pattern with {version}, {os}, {arch} placeholders
	PinnedVersion string              // Expected version for this dnstm release
	Archive       bool                // If true, URL points to a tar.xz archive
	ArchiveDir    string              // Directory inside archive where binary is located
	Platforms     map[string][]string // Supported os -> []arch
	SkipUpdate    bool                // If true, skip in update process
	ChecksumURL   string              // URL pattern for checksum file (empty = skip verification)

	// archMappings is populated at init() for custom placeholder expansion.
	archMappings map[string]binman.ArchMapping
}

// Static arch mappings for shadowsocks-rust.
var shadowsocksArchMappings = map[string]binman.ArchMapping{
	"ssarch": {
		"linux/amd64":  "x86_64-unknown-linux-gnu",
		"linux/arm64":  "aarch64-unknown-linux-gnu",
		"darwin/amd64": "x86_64-apple-darwin",
		"darwin/arm64": "aarch64-apple-darwin",
	},
}

// DefaultBinaries contains definitions for all supported binaries.
var DefaultBinaries = map[BinaryType]BinaryDef{
	// Server binaries - versions pinned per dnstm release
	BinaryDNSTTServer: {
		Type:        BinaryDNSTTServer,
		EnvVar:      "DNSTM_DNSTT_SERVER_PATH",
		URLPattern:  "https://github.com/net2share/dnstt/releases/download/latest/dnstt-server-{os}-{arch}{ext}",
		ChecksumURL: "https://github.com/net2share/dnstt/releases/download/latest/checksums.sha256",
		SkipUpdate:  true,
		Platforms: map[string][]string{
			"linux":   {"amd64", "arm64"},
			"darwin":  {"amd64", "arm64"},
			"windows": {"amd64", "arm64"},
		},
	},
	BinarySlipstreamServer: {
		Type:          BinarySlipstreamServer,
		EnvVar:        "DNSTM_SLIPSTREAM_SERVER_PATH",
		URLPattern:    "https://github.com/net2share/slipstream-rust-build/releases/download/{version}/slipstream-server-{os}-{arch}",
		ChecksumURL:   "https://github.com/net2share/slipstream-rust-build/releases/download/{version}/SHA256SUMS",
		PinnedVersion: "v2026.02.22.1",
		Platforms: map[string][]string{
			"linux": {"amd64", "arm64"},
		},
	},
	BinarySSServer: {
		Type:          BinarySSServer,
		EnvVar:        "DNSTM_SSSERVER_PATH",
		URLPattern:    "https://github.com/shadowsocks/shadowsocks-rust/releases/download/{version}/shadowsocks-{version}.{ssarch}.tar.xz",
		ChecksumURL:   "https://github.com/shadowsocks/shadowsocks-rust/releases/download/{version}/shadowsocks-{version}.{ssarch}.tar.xz.sha256",
		PinnedVersion: "v1.24.0",
		Archive:       true,
		Platforms: map[string][]string{
			"linux":  {"amd64", "arm64"},
			"darwin": {"amd64", "arm64"},
		},
	},
	BinaryMicrosocks: {
		Type:          BinaryMicrosocks,
		EnvVar:        "DNSTM_MICROSOCKS_PATH",
		URLPattern:    "https://github.com/net2share/microsocks-build/releases/download/{version}/microsocks-{microsocksarch}",
		ChecksumURL:   "https://github.com/net2share/microsocks-build/releases/download/{version}/SHA256SUMS",
		PinnedVersion: "v1.0.5",
		Platforms: map[string][]string{
			"linux": {"amd64", "arm64"},
		},
	},
	BinarySSHTunUser: {
		Type:          BinarySSHTunUser,
		EnvVar:        "DNSTM_SSHTUN_USER_PATH",
		URLPattern:    "https://github.com/net2share/sshtun-user/releases/download/{version}/sshtun-user-linux-{arch}",
		PinnedVersion: "v0.3.5",
		Platforms: map[string][]string{
			"linux": {"amd64", "arm64"},
		},
	},
	BinaryVayDNSServer: {
		Type:          BinaryVayDNSServer,
		EnvVar:        "DNSTM_VAYDNS_SERVER_PATH",
		URLPattern:    "https://github.com/net2share/vaydns/releases/download/{version}/vaydns-server-{os}-{arch}{ext}",
		ChecksumURL:   "https://github.com/net2share/vaydns/releases/download/{version}/vaydns-server-{os}-{arch}.sha256",
		PinnedVersion: "v0.2.7",
		Platforms: map[string][]string{
			"linux":   {"amd64", "arm64"},
			"darwin":  {"amd64", "arm64"},
			"windows": {"amd64"},
		},
	},
	BinarySlipstreamPlusServer: {
		Type:          BinarySlipstreamPlusServer,
		EnvVar:        "DNSTM_SLIPSTREAM_PLUS_SERVER_PATH",
		URLPattern:    "https://github.com/Fox-Fig/slipstream-rust-deploy/releases/latest/download/slipstream-server-{os}-{arch}",
		PinnedVersion: "latest",
		Platforms: map[string][]string{
			"linux": {"amd64", "arm64"},
		},
	},

	// Client binaries - pinned versions for testing only
	BinaryDNSTTClient: {
		Type:          BinaryDNSTTClient,
		EnvVar:        "DNSTM_TEST_DNSTT_CLIENT_PATH",
		URLPattern:    "https://github.com/net2share/dnstt/releases/download/latest/dnstt-client-{os}-{arch}{ext}",
		ChecksumURL:   "https://github.com/net2share/dnstt/releases/download/latest/checksums.sha256",
		PinnedVersion: "latest",
		Platforms: map[string][]string{
			"linux":   {"amd64", "arm64"},
			"darwin":  {"amd64", "arm64"},
			"windows": {"amd64", "arm64"},
		},
	},
	BinarySlipstreamClient: {
		Type:          BinarySlipstreamClient,
		EnvVar:        "DNSTM_TEST_SLIPSTREAM_CLIENT_PATH",
		URLPattern:    "https://github.com/net2share/slipstream-rust-build/releases/download/{version}/slipstream-client-{os}-{arch}",
		ChecksumURL:   "https://github.com/net2share/slipstream-rust-build/releases/download/{version}/SHA256SUMS",
		PinnedVersion: "v2026.02.05",
		Platforms: map[string][]string{
			"linux": {"amd64", "arm64"},
		},
	},
	BinarySSLocal: {
		Type:          BinarySSLocal,
		EnvVar:        "DNSTM_TEST_SSLOCAL_PATH",
		URLPattern:    "https://github.com/shadowsocks/shadowsocks-rust/releases/download/{version}/shadowsocks-{version}.{ssarch}.tar.xz",
		ChecksumURL:   "https://github.com/shadowsocks/shadowsocks-rust/releases/download/{version}/shadowsocks-{version}.{ssarch}.tar.xz.sha256",
		PinnedVersion: "v1.23.0",
		Archive:       true,
		Platforms: map[string][]string{
			"linux":  {"amd64", "arm64"},
			"darwin": {"amd64", "arm64"},
		},
	},
	BinaryVayDNSClient: {
		Type:          BinaryVayDNSClient,
		EnvVar:        "DNSTM_TEST_VAYDNS_CLIENT_PATH",
		URLPattern:    "https://github.com/net2share/vaydns/releases/download/{version}/vaydns-client-{os}-{arch}{ext}",
		ChecksumURL:   "https://github.com/net2share/vaydns/releases/download/{version}/vaydns-client-{os}-{arch}.sha256",
		PinnedVersion: "v0.2.7",
		Platforms: map[string][]string{
			"linux":   {"amd64", "arm64"},
			"darwin":  {"amd64", "arm64"},
			"windows": {"amd64"},
		},
	},
	BinarySlipstreamPlusClient: {
		Type:          BinarySlipstreamPlusClient,
		EnvVar:        "DNSTM_TEST_SLIPSTREAM_PLUS_CLIENT_PATH",
		URLPattern:    "https://github.com/Fox-Fig/slipstream-rust-deploy/releases/latest/download/slipstream-client-{os}-{arch}",
		PinnedVersion: "latest", // Manual bump only
		Platforms: map[string][]string{
			"linux": {"amd64", "arm64"},
		},
	},
}

func init() {
	// Populate arch mappings for shadowsocks binaries (static).
	for _, bt := range []BinaryType{BinarySSServer, BinarySSLocal} {
		def := DefaultBinaries[bt]
		def.archMappings = shadowsocksArchMappings
		DefaultBinaries[bt] = def
	}

	// Populate arch mappings for microsocks (runtime libc detection).
	msDef := DefaultBinaries[BinaryMicrosocks]
	msDef.archMappings = computeMicrosocksArchMappings()
	DefaultBinaries[BinaryMicrosocks] = msDef
}

// computeMicrosocksArchMappings detects libc at runtime and returns the appropriate mappings.
func computeMicrosocksArchMappings() map[string]binman.ArchMapping {
	libc := detectLibc()
	m := binman.ArchMapping{}

	if libc == "glibc" {
		m["linux/amd64"] = "x86_64-linux-gnu"
		m["linux/arm64"] = "aarch64-linux-gnu"
	} else {
		m["linux/amd64"] = "x86_64-linux-musl"
		m["linux/arm64"] = "aarch64-linux-musl"
	}

	return map[string]binman.ArchMapping{
		"microsocksarch": m,
	}
}

// detectLibc detects whether the system uses glibc or musl.
func detectLibc() string {
	if _, err := os.Stat("/etc/alpine-release"); err == nil {
		return "musl"
	}
	if _, err := os.Stat("/lib/x86_64-linux-gnu"); err == nil {
		return "glibc"
	}
	if _, err := os.Stat("/lib/aarch64-linux-gnu"); err == nil {
		return "glibc"
	}
	if _, err := os.Stat("/lib64/ld-linux-x86-64.so.2"); err == nil {
		return "glibc"
	}
	return "glibc"
}

// toBinmanDef converts a local BinaryDef to a binman.BinaryDef.
func toBinmanDef(def BinaryDef) binman.BinaryDef {
	archiveType := ""
	if def.Archive {
		archiveType = "tar.xz"
	}
	return binman.BinaryDef{
		Name:          string(def.Type),
		EnvOverride:   def.EnvVar,
		URLPattern:    def.URLPattern,
		PinnedVersion: def.PinnedVersion,
		ArchiveType:   archiveType,
		ChecksumURL:   def.ChecksumURL,
		Platforms:      def.Platforms,
		SkipUpdate:    def.SkipUpdate,
		ArchMappings:  def.archMappings,
	}
}

const (
	// DefaultInstallDir is the default directory for production binaries.
	DefaultInstallDir = "/usr/local/bin"
	// DefaultTestBinDir is the default directory for test binaries.
	DefaultTestBinDir = "tests/.testbin"
)

// Manager handles binary resolution and downloading.
type Manager struct {
	bm     *binman.Manager
	binDir string
}

// NewManager creates a new binary manager with a specific directory.
func NewManager(binDir string) *Manager {
	return &Manager{
		bm:     binman.NewManager(binDir),
		binDir: binDir,
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
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
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
func (m *Manager) GetPath(binType BinaryType) (string, error) {
	def, ok := DefaultBinaries[binType]
	if !ok {
		return "", fmt.Errorf("unknown binary type: %s", binType)
	}

	bd := toBinmanDef(def)
	if !m.bm.IsPlatformSupported(bd) {
		return "", fmt.Errorf("binary %s not supported on %s/%s", binType, runtime.GOOS, runtime.GOARCH)
	}

	return m.bm.ResolvePath(bd)
}

// EnsureInstalled ensures a binary is available, downloading if necessary.
func (m *Manager) EnsureInstalled(binType BinaryType) (string, error) {
	def, ok := DefaultBinaries[binType]
	if !ok {
		return "", fmt.Errorf("unknown binary type: %s", binType)
	}

	bd := toBinmanDef(def)
	if !m.bm.IsPlatformSupported(bd) {
		return "", fmt.Errorf("binary %s not supported on %s/%s", binType, runtime.GOOS, runtime.GOARCH)
	}

	path, err := m.bm.EnsureInstalled(bd, nil)
	if err != nil {
		return "", fmt.Errorf("failed to install %s: %w", binType, err)
	}

	log.Debug("binary %s: available at %s", binType, path)
	return path, nil
}

// DownloadVersion downloads a specific version of a binary, replacing any existing one.
func (m *Manager) DownloadVersion(binType BinaryType, version string) error {
	def, ok := DefaultBinaries[binType]
	if !ok {
		return fmt.Errorf("unknown binary type: %s", binType)
	}

	bd := toBinmanDef(def)
	if !m.bm.IsPlatformSupported(bd) {
		return fmt.Errorf("binary %s not supported on %s/%s", binType, runtime.GOOS, runtime.GOARCH)
	}

	return m.bm.Download(bd, version, nil)
}

// EnsureDir creates the binary directory if it doesn't exist.
func (m *Manager) EnsureDir() error {
	return m.bm.EnsureDir()
}

// BinDir returns the binary directory path.
func (m *Manager) BinDir() string {
	return m.binDir
}

// GetDef returns the binary definition for a binary type.
func GetDef(binType BinaryType) (BinaryDef, bool) {
	def, ok := DefaultBinaries[binType]
	return def, ok
}

// ServerBinaries returns definitions for all server binaries (excluding client/test binaries).
func ServerBinaries() []BinaryDef {
	serverTypes := []BinaryType{
		BinaryDNSTTServer, BinarySlipstreamServer, BinarySSServer,
		BinaryMicrosocks, BinarySSHTunUser, BinaryVayDNSServer,
	}
	var defs []BinaryDef
	for _, bt := range serverTypes {
		if def, ok := DefaultBinaries[bt]; ok {
			defs = append(defs, def)
		}
	}
	return defs
}

// CopyToDir copies a binary from srcPath to the manager's binDir.
func (m *Manager) CopyToDir(srcPath string, binType BinaryType) (string, error) {
	if err := m.EnsureDir(); err != nil {
		return "", err
	}

	destName := string(binType)
	if runtime.GOOS == "windows" {
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
