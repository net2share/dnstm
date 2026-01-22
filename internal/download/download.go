package download

import (
	"crypto/md5"
	"crypto/sha1"
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
	DnsttBaseURL = "https://dnstt.network"
	InstallDir   = "/usr/local/bin"
)

type Checksums struct {
	MD5    string
	SHA1   string
	SHA256 string
}

func DownloadDnsttServer(arch string, progressFn func(downloaded, total int64)) (string, error) {
	binaryName := fmt.Sprintf("dnstt-server-%s", arch)
	url := fmt.Sprintf("%s/%s", DnsttBaseURL, binaryName)

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

func FetchChecksums(arch string) (*Checksums, error) {
	checksums := &Checksums{}

	types := []struct {
		name string
		ptr  *string
	}{
		{"md5", &checksums.MD5},
		{"sha1", &checksums.SHA1},
		{"sha256", &checksums.SHA256},
	}

	binaryName := fmt.Sprintf("dnstt-server-%s", arch)

	for _, ct := range types {
		url := fmt.Sprintf("%s/%s.%s", DnsttBaseURL, binaryName, ct.name)
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			if err == nil {
				parts := strings.Fields(string(data))
				if len(parts) > 0 {
					*ct.ptr = parts[0]
				}
			}
		}
	}

	return checksums, nil
}

func VerifyChecksums(filePath string, expected *Checksums) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	md5Hash := md5.New()
	sha1Hash := sha1.New()
	sha256Hash := sha256.New()

	multiWriter := io.MultiWriter(md5Hash, sha1Hash, sha256Hash)

	if _, err := io.Copy(multiWriter, file); err != nil {
		return fmt.Errorf("failed to compute checksums: %w", err)
	}

	md5Sum := hex.EncodeToString(md5Hash.Sum(nil))
	sha1Sum := hex.EncodeToString(sha1Hash.Sum(nil))
	sha256Sum := hex.EncodeToString(sha256Hash.Sum(nil))

	if expected.MD5 != "" && md5Sum != expected.MD5 {
		return fmt.Errorf("MD5 checksum mismatch: expected %s, got %s", expected.MD5, md5Sum)
	}

	if expected.SHA1 != "" && sha1Sum != expected.SHA1 {
		return fmt.Errorf("SHA1 checksum mismatch: expected %s, got %s", expected.SHA1, sha1Sum)
	}

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
