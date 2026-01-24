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

func DownloadDnsttServer(baseURL, arch string, progressFn func(downloaded, total int64)) (string, error) {
	binaryName := fmt.Sprintf("dnstt-server-%s", arch)
	url := fmt.Sprintf("%s/%s", baseURL, binaryName)

	tmpFile, err := os.CreateTemp("", "dnstt-server-*")
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

func FetchChecksums(baseURL, arch string) (*Checksums, error) {
	checksums := &Checksums{}
	binaryName := fmt.Sprintf("dnstt-server-%s", arch)

	url := fmt.Sprintf("%s/checksums.sha256", baseURL)
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

func InstallBinary(tmpPath string) error {
	destPath := filepath.Join(InstallDir, "dnstt-server")

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

func IsDnsttInstalled() bool {
	_, err := os.Stat(filepath.Join(InstallDir, "dnstt-server"))
	return err == nil
}

func RemoveBinary() {
	os.Remove(filepath.Join(InstallDir, "dnstt-server"))
}
