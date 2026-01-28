package shadowsocks

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/net2share/dnstm/internal/download"
)

const (
	GitHubReleasesAPI = "https://api.github.com/repos/shadowsocks/shadowsocks-rust/releases/latest"
)

// GitHubRelease represents a GitHub release response.
type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a release asset.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// DownloadShadowsocks downloads the latest shadowsocks-rust from GitHub releases.
func DownloadShadowsocks(progressFn func(downloaded, total int64)) error {
	// Fetch latest release info
	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}

	// Find the correct asset for our architecture
	assetName := getBinaryAssetName()
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, assetName) && strings.HasSuffix(asset.Name, ".tar.xz") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("could not find shadowsocks-rust binary for architecture %s", runtime.GOARCH)
	}

	// Download the tarball
	tmpFile, err := os.CreateTemp("", "shadowsocks-*.tar.xz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	if progressFn != nil {
		_, err = io.Copy(tmpFile, &progressReader{
			reader:     resp.Body,
			total:      resp.ContentLength,
			progressFn: progressFn,
		})
	} else {
		_, err = io.Copy(tmpFile, resp.Body)
	}
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Close the file before extracting
	tmpFile.Close()

	// Extract the tarball
	if err := extractTarball(tmpFile.Name()); err != nil {
		return fmt.Errorf("failed to extract tarball: %w", err)
	}

	return nil
}

// fetchLatestRelease fetches the latest release info from GitHub.
func fetchLatestRelease() (*GitHubRelease, error) {
	req, err := http.NewRequest("GET", GitHubReleasesAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// getBinaryAssetName returns the asset name pattern for the current architecture.
func getBinaryAssetName() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64-unknown-linux-gnu"
	case "arm64":
		return "aarch64-unknown-linux-gnu"
	default:
		return "x86_64-unknown-linux-gnu"
	}
}

// extractTarball extracts the shadowsocks tarball and installs ssserver.
func extractTarball(tarballPath string) error {
	// Create temp directory for extraction
	tmpDir, err := os.MkdirTemp("", "shadowsocks-extract-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract using tar (xz compression)
	cmd := exec.Command("tar", "-xJf", tarballPath, "-C", tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar extraction failed: %s: %w", string(output), err)
	}

	// Find ssserver binary in extracted files
	var ssserverPath string
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == BinaryName {
			ssserverPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && ssserverPath == "" {
		return fmt.Errorf("failed to find %s in archive: %w", BinaryName, err)
	}

	if ssserverPath == "" {
		return fmt.Errorf("ssserver binary not found in archive")
	}

	// Install the binary
	destPath := filepath.Join(download.InstallDir, BinaryName)
	input, err := os.ReadFile(ssserverPath)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	if err := os.WriteFile(destPath, input, 0755); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	return nil
}

// IsSsserverInstalled checks if ssserver is installed.
func IsSsserverInstalled() bool {
	return download.IsBinaryInstalled(BinaryName)
}

// RemoveSsserver removes the ssserver binary.
func RemoveSsserver() {
	download.RemoveBinaryByName(BinaryName)
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
