package transport

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/net2share/dnstm/internal/types"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

const (
	// GitHub release URLs (net2share pre-built binaries)
	dnsttReleaseURL      = "https://github.com/net2share/dnstt/releases/download/latest"
	slipstreamReleaseURL = "https://github.com/net2share/slipstream-rust-build/releases/download/latest"

	// GitHub release API URLs (official repos)
	shadowsocksReleaseAPI = "https://api.github.com/repos/shadowsocks/shadowsocks-rust/releases/latest"

	installDir = "/usr/local/bin"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// EnsureBinariesInstalled checks and installs required binaries for a transport type.
func EnsureBinariesInstalled(t types.TransportType) error {
	switch t {
	case types.TypeSlipstreamShadowsocks:
		if err := ensureSlipstreamInstalled(); err != nil {
			return err
		}
		return ensureShadowsocksInstalled()
	case types.TypeSlipstreamSocks, types.TypeSlipstreamSSH:
		return ensureSlipstreamInstalled()
	case types.TypeDNSTTSocks, types.TypeDNSTTSSH:
		return ensureDnsttInstalled()
	default:
		return nil
	}
}

func ensureDnsttInstalled() error {
	if _, err := os.Stat(DNSTTBinary); err == nil {
		tui.PrintStatus("dnstt-server already installed")
		return nil
	}

	tui.PrintInfo("Installing dnstt-server...")

	arch := getDnsttArch()
	url := fmt.Sprintf("%s/dnstt-server-%s", dnsttReleaseURL, arch)

	tmpPath, err := downloadFile(url, nil)
	if err != nil {
		return fmt.Errorf("failed to download dnstt-server: %w", err)
	}

	if err := installBinary(tmpPath, "dnstt-server"); err != nil {
		return err
	}

	tui.PrintStatus("dnstt-server installed")
	return nil
}

func ensureSlipstreamInstalled() error {
	if _, err := os.Stat(SlipstreamBinary); err == nil {
		tui.PrintStatus("slipstream-server already installed")
		return nil
	}

	tui.PrintInfo("Installing slipstream-server...")

	arch := getSlipstreamArch()
	binaryName := fmt.Sprintf("slipstream-server-%s", arch)
	url := fmt.Sprintf("%s/%s", slipstreamReleaseURL, binaryName)

	// Fetch checksum
	checksumURL := fmt.Sprintf("%s/SHA256SUMS", slipstreamReleaseURL)
	expectedChecksum, _ := fetchChecksum(checksumURL, binaryName)

	tmpPath, err := downloadFile(url, nil)
	if err != nil {
		return fmt.Errorf("failed to download slipstream-server: %w", err)
	}

	if expectedChecksum != "" {
		if err := verifyChecksum(tmpPath, expectedChecksum); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	if err := installBinary(tmpPath, "slipstream-server"); err != nil {
		return err
	}

	tui.PrintStatus("slipstream-server installed")
	return nil
}

func ensureShadowsocksInstalled() error {
	if _, err := os.Stat(SSServerBinary); err == nil {
		tui.PrintStatus("ssserver already installed")
		return nil
	}

	tui.PrintInfo("Installing ssserver (shadowsocks-rust)...")

	release, err := fetchGithubRelease(shadowsocksReleaseAPI)
	if err != nil {
		return fmt.Errorf("failed to fetch shadowsocks release: %w", err)
	}

	// shadowsocks-rust releases are tarballs, e.g., shadowsocks-v1.20.4.x86_64-unknown-linux-gnu.tar.xz
	assetSuffix := getShadowsocksAssetSuffix()
	var tarballURL string

	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, assetSuffix) {
			tarballURL = asset.BrowserDownloadURL
			break
		}
	}

	if tarballURL == "" {
		return fmt.Errorf("no shadowsocks-rust binary found for architecture suffix: %s", assetSuffix)
	}

	tmpPath, err := downloadFile(tarballURL, nil)
	if err != nil {
		return fmt.Errorf("failed to download shadowsocks-rust: %w", err)
	}

	// Extract ssserver from tarball
	ssserverPath, err := extractSsserverFromTarball(tmpPath)
	os.Remove(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to extract ssserver: %w", err)
	}

	if err := installBinary(ssserverPath, "ssserver"); err != nil {
		return err
	}

	tui.PrintStatus("ssserver installed")
	return nil
}

func fetchGithubRelease(apiURL string) (*githubRelease, error) {
	resp, err := http.Get(apiURL)
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

func downloadFile(url string, progressFn func(downloaded, total int64)) (string, error) {
	tmpFile, err := os.CreateTemp("", "dnstm-download-*")
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

func installBinary(tmpPath, binaryName string) error {
	destPath := filepath.Join(installDir, binaryName)

	if err := os.MkdirAll(installDir, 0755); err != nil {
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

func getDnsttArch() string {
	arch := osdetect.GetArch()
	switch arch {
	case "amd64":
		return "linux-amd64"
	case "arm64":
		return "linux-arm64"
	default:
		return "linux-amd64"
	}
}

func getSlipstreamArch() string {
	arch := osdetect.GetArch()
	switch arch {
	case "amd64":
		return "linux-amd64"
	case "arm64":
		return "linux-arm64"
	default:
		return "linux-amd64"
	}
}

func getShadowsocksAssetSuffix() string {
	arch := osdetect.GetArch()
	switch arch {
	case "amd64":
		return "x86_64-unknown-linux-gnu.tar.xz"
	case "arm64":
		return "aarch64-unknown-linux-gnu.tar.xz"
	default:
		return "x86_64-unknown-linux-gnu.tar.xz"
	}
}

func extractSsserverFromTarball(tarballPath string) (string, error) {
	// Use tar to extract ssserver from the tarball
	tmpDir, err := os.MkdirTemp("", "ssserver-extract-*")
	if err != nil {
		return "", err
	}

	// Extract using tar command (xz compressed)
	cmd := fmt.Sprintf("tar -xJf %s -C %s ssserver 2>/dev/null || tar -xf %s -C %s ssserver", tarballPath, tmpDir, tarballPath, tmpDir)
	if err := runCmd(cmd); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to extract tarball: %w", err)
	}

	ssserverPath := filepath.Join(tmpDir, "ssserver")
	if _, err := os.Stat(ssserverPath); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("ssserver not found in tarball")
	}

	return ssserverPath, nil
}

func runCmd(cmd string) error {
	c := exec.Command("sh", "-c", cmd)
	return c.Run()
}
