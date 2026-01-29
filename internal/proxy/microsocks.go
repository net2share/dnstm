package proxy

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/go-corelib/osdetect"
)

const (
	MicrosocksRepoAPI     = "https://api.github.com/repos/net2share/microsocks-build/releases/latest"
	MicrosocksBinaryName  = "microsocks"
	MicrosocksServiceName = "microsocks"
	MicrosocksBindAddr    = "127.0.0.1"
	MicrosocksInstallDir  = "/usr/local/bin"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// InstallMicrosocks downloads and installs the microsocks binary from the latest GitHub release.
func InstallMicrosocks(progressFn func(downloaded, total int64)) error {
	// Fetch latest release info
	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch latest release: %w", err)
	}

	// Find the correct binary for this architecture
	binaryName := getBinaryName()
	var binaryURL, checksumURL string

	for _, asset := range release.Assets {
		if asset.Name == binaryName {
			binaryURL = asset.BrowserDownloadURL
		}
		if asset.Name == "SHA256SUMS" {
			checksumURL = asset.BrowserDownloadURL
		}
	}

	if binaryURL == "" {
		return fmt.Errorf("no binary found for architecture: %s", binaryName)
	}

	// Fetch checksums
	var expectedChecksum string
	if checksumURL != "" {
		expectedChecksum, _ = fetchChecksum(checksumURL, binaryName)
	}

	// Download binary
	tmpPath, err := downloadBinary(binaryURL, progressFn)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Verify checksum if available
	if expectedChecksum != "" {
		if err := verifyChecksum(tmpPath, expectedChecksum); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Install binary
	return installBinary(tmpPath)
}

func fetchLatestRelease() (*githubRelease, error) {
	resp, err := http.Get(MicrosocksRepoAPI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func fetchChecksum(url, binaryName string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch checksums: %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := parts[0]
			filename := strings.TrimPrefix(parts[1], "*")
			if filename == binaryName {
				return hash, nil
			}
		}
	}

	return "", fmt.Errorf("checksum not found for %s", binaryName)
}

func downloadBinary(url string, progressFn func(downloaded, total int64)) (string, error) {
	tmpFile, err := os.CreateTemp("", "microsocks-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	resp, err := http.Get(url)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	var written int64
	if progressFn != nil {
		written, err = io.Copy(tmpFile, &progressReader{
			reader:     resp.Body,
			total:      resp.ContentLength,
			progressFn: progressFn,
		})
	} else {
		written, err = io.Copy(tmpFile, resp.Body)
	}

	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	if written == 0 {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("downloaded file is empty")
	}

	return tmpFile.Name(), nil
}

type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	progressFn func(downloaded, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)
	if pr.progressFn != nil {
		pr.progressFn(pr.downloaded, pr.total)
	}
	return n, err
}

func verifyChecksum(filePath, expected string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}

func installBinary(tmpPath string) error {
	destPath := filepath.Join(MicrosocksInstallDir, MicrosocksBinaryName)

	if err := os.MkdirAll(MicrosocksInstallDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	input, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read temp file: %w", err)
	}

	if err := os.WriteFile(destPath, input, 0755); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	os.Remove(tmpPath)
	return nil
}

func getBinaryName() string {
	arch := osdetect.GetArch()
	libc := detectLibc()

	// Prefer GNU/glibc builds for standard Linux distributions
	// Fall back to musl for Alpine or when glibc is not available
	if libc == "glibc" {
		switch arch {
		case "amd64":
			return "microsocks-x86_64-linux-gnu"
		case "arm64":
			return "microsocks-aarch64-linux-gnu"
		}
	}

	// musl builds for Alpine, other archs, or fallback
	switch arch {
	case "amd64":
		return "microsocks-x86_64-linux-musl"
	case "arm64":
		return "microsocks-aarch64-linux-musl"
	case "arm":
		return "microsocks-arm-linux-musleabihf"
	case "386":
		return "microsocks-i686-linux-musl"
	}

	return "microsocks-x86_64-linux-musl"
}

// detectLibc detects whether the system uses glibc or musl
func detectLibc() string {
	// Check for Alpine Linux (uses musl)
	if _, err := os.Stat("/etc/alpine-release"); err == nil {
		return "musl"
	}

	// Check ldd version output for glibc indicator
	// Most glibc systems will have ldd that mentions "GNU libc" or "GLIBC"
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

// ConfigureMicrosocks creates the systemd service for microsocks with the specified port.
func ConfigureMicrosocks(port int) error {
	return service.CreateGenericService(&service.ServiceConfig{
		Name:             MicrosocksServiceName,
		Description:      "Microsocks SOCKS5 Proxy",
		User:             "nobody",
		Group:            "nogroup",
		ExecStart:        fmt.Sprintf("/usr/local/bin/microsocks -i %s -p %d -q", MicrosocksBindAddr, port),
		ReadOnlyPaths:    []string{"/usr/local/bin"},
		BindToPrivileged: false,
	})
}

// FindAvailablePort finds an available port in the range 10000-60000.
func FindAvailablePort() (int, error) {
	// Try random ports in the high range to avoid conflicts
	for i := 0; i < 100; i++ {
		port := 10000 + rand.Intn(50000) // Range: 10000-60000
		if isPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("could not find available port")
}

// isPortAvailable checks if a port is available for binding.
func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// StartMicrosocks enables and starts the microsocks service.
func StartMicrosocks() error {
	if err := service.EnableService(MicrosocksServiceName); err != nil {
		return err
	}
	return service.StartService(MicrosocksServiceName)
}

// RestartMicrosocks restarts the microsocks service.
func RestartMicrosocks() error {
	return service.RestartService(MicrosocksServiceName)
}

// StopMicrosocks stops the microsocks service.
func StopMicrosocks() error {
	return service.StopService(MicrosocksServiceName)
}

// IsMicrosocksInstalled checks if the microsocks binary is installed.
func IsMicrosocksInstalled() bool {
	_, err := os.Stat(filepath.Join(MicrosocksInstallDir, MicrosocksBinaryName))
	return err == nil
}

// IsMicrosocksRunning checks if the microsocks service is active.
func IsMicrosocksRunning() bool {
	return service.IsServiceActive(MicrosocksServiceName)
}

// UninstallMicrosocks removes the microsocks binary and service.
func UninstallMicrosocks() error {
	if service.IsServiceActive(MicrosocksServiceName) {
		service.StopService(MicrosocksServiceName)
	}
	if service.IsServiceEnabled(MicrosocksServiceName) {
		service.DisableService(MicrosocksServiceName)
	}
	service.RemoveService(MicrosocksServiceName)
	os.Remove(filepath.Join(MicrosocksInstallDir, MicrosocksBinaryName))
	return nil
}
