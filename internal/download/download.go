package download

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	InstallDir = "/usr/local/bin"
)

type Checksums struct {
	SHA256 string
}

// BinaryConfig contains configuration for downloading a binary.
type BinaryConfig struct {
	BaseURL      string
	BinaryName   string // e.g., "dnstt-server" or "slipstream-server"
	Arch         string // e.g., "linux-amd64"
	ChecksumFile string // e.g., "checksums.sha256" or "SHA256SUMS"
}

// DownloadBinary downloads a binary from the given configuration.
func DownloadBinary(cfg *BinaryConfig, progressFn func(downloaded, total int64)) (string, error) {
	binaryName := fmt.Sprintf("%s-%s", cfg.BinaryName, cfg.Arch)
	url := fmt.Sprintf("%s/%s", cfg.BaseURL, binaryName)

	tmpFile, err := os.CreateTemp("", cfg.BinaryName+"-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	resp, err := http.Get(url)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download: %w", err)
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
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	if written == 0 {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("downloaded file is empty")
	}

	return tmpFile.Name(), nil
}

// DownloadDnsttServer downloads the dnstt-server binary (backward compatible wrapper).
func DownloadDnsttServer(baseURL, arch string, progressFn func(downloaded, total int64)) (string, error) {
	return DownloadBinary(&BinaryConfig{
		BaseURL:      baseURL,
		BinaryName:   "dnstt-server",
		Arch:         arch,
		ChecksumFile: "checksums.sha256",
	}, progressFn)
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

// FetchChecksumsForBinary fetches checksums for a binary using the given configuration.
func FetchChecksumsForBinary(cfg *BinaryConfig) (*Checksums, error) {
	checksums := &Checksums{}
	binaryName := fmt.Sprintf("%s-%s", cfg.BinaryName, cfg.Arch)

	checksumFile := cfg.ChecksumFile
	if checksumFile == "" {
		checksumFile = "checksums.sha256"
	}

	url := fmt.Sprintf("%s/%s", cfg.BaseURL, checksumFile)
	resp, err := http.Get(url)
	if err != nil {
		return checksums, fmt.Errorf("failed to fetch checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return checksums, fmt.Errorf("failed to fetch checksums: %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := parts[0]
			filename := parts[1]
			if filename == binaryName {
				checksums.SHA256 = hash
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return checksums, fmt.Errorf("failed to parse checksums: %w", err)
	}

	return checksums, nil
}

// FetchChecksums fetches checksums for dnstt-server (backward compatible wrapper).
func FetchChecksums(baseURL, arch string) (*Checksums, error) {
	return FetchChecksumsForBinary(&BinaryConfig{
		BaseURL:      baseURL,
		BinaryName:   "dnstt-server",
		Arch:         arch,
		ChecksumFile: "checksums.sha256",
	})
}

func VerifyChecksums(filePath string, expected *Checksums) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	sha256Hash := sha256.New()

	if _, err := io.Copy(sha256Hash, file); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	sha256Sum := hex.EncodeToString(sha256Hash.Sum(nil))

	if expected.SHA256 != "" && sha256Sum != expected.SHA256 {
		return fmt.Errorf("SHA256 checksum mismatch: expected %s, got %s", expected.SHA256, sha256Sum)
	}

	return nil
}

// InstallBinaryAs installs a binary with the given name.
func InstallBinaryAs(tmpPath, binaryName string) error {
	destPath := filepath.Join(InstallDir, binaryName)

	if err := os.MkdirAll(InstallDir, 0755); err != nil {
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

// InstallBinary installs dnstt-server (backward compatible wrapper).
func InstallBinary(tmpPath string) error {
	return InstallBinaryAs(tmpPath, "dnstt-server")
}

// IsBinaryInstalled checks if a binary is installed.
func IsBinaryInstalled(binaryName string) bool {
	_, err := os.Stat(filepath.Join(InstallDir, binaryName))
	return err == nil
}

// IsDnsttInstalled checks if dnstt-server is installed (backward compatible wrapper).
func IsDnsttInstalled() bool {
	return IsBinaryInstalled("dnstt-server")
}

// RemoveBinaryByName removes a binary by name.
func RemoveBinaryByName(binaryName string) {
	os.Remove(filepath.Join(InstallDir, binaryName))
}

// RemoveBinary removes dnstt-server (backward compatible wrapper).
func RemoveBinary() {
	RemoveBinaryByName("dnstt-server")
}

// DownloadFile downloads a file from the given URL to the specified destination path.
func DownloadFile(url, dest string, progressFn func(downloaded, total int64)) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	var reader io.Reader = resp.Body
	if progressFn != nil {
		reader = &progressReader{
			reader:     resp.Body,
			total:      resp.ContentLength,
			progressFn: progressFn,
		}
	}

	if _, err := io.Copy(tmpFile, reader); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync file: %w", err)
	}
	tmpFile.Close()

	// Use file copy instead of rename to handle cross-device moves
	if err := CopyFile(tmpPath, dest); err != nil {
		return fmt.Errorf("failed to move file to destination: %w", err)
	}

	return nil
}

// CopyFile copies a file from src to dst using streaming I/O
func CopyFile(src, dst string) error {

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return nil
}
