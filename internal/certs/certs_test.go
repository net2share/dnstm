package certs

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "test_cert.pem")
	keyPath := filepath.Join(tmpDir, "test_key.pem")
	domain := "test.example.com"

	fingerprint, err := GenerateCertificate(certPath, keyPath, domain)
	if err != nil {
		t.Fatalf("GenerateCertificate failed: %v", err)
	}

	// Fingerprint should be 64 hex characters (SHA256)
	if len(fingerprint) != 64 {
		t.Errorf("fingerprint length = %d, want 64", len(fingerprint))
	}

	// Should be valid hex
	_, err = hex.DecodeString(fingerprint)
	if err != nil {
		t.Errorf("fingerprint is not valid hex: %v", err)
	}

	// Files should exist
	if _, err := os.Stat(certPath); err != nil {
		t.Errorf("certificate file not found: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("key file not found: %v", err)
	}

	// Key file should have restricted permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("failed to stat key file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("key file permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestGenerateCertificate_CertContent(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "test_cert.pem")
	keyPath := filepath.Join(tmpDir, "test_key.pem")
	domain := "test.example.com"

	_, err := GenerateCertificate(certPath, keyPath, domain)
	if err != nil {
		t.Fatalf("GenerateCertificate failed: %v", err)
	}

	// Read and parse certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode PEM block")
	}
	if block.Type != "CERTIFICATE" {
		t.Errorf("PEM type = %q, want 'CERTIFICATE'", block.Type)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	// Verify certificate properties
	if cert.Subject.CommonName != domain {
		t.Errorf("CommonName = %q, want %q", cert.Subject.CommonName, domain)
	}

	// Check SAN
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != domain {
		t.Errorf("DNSNames = %v, want [%q]", cert.DNSNames, domain)
	}

	// Check validity period (10 years)
	expectedExpiry := time.Now().AddDate(10, 0, 0)
	daysDiff := int(cert.NotAfter.Sub(expectedExpiry).Hours() / 24)
	if daysDiff < -1 || daysDiff > 1 {
		t.Errorf("NotAfter = %v, expected ~%v", cert.NotAfter, expectedExpiry)
	}

	// Check key usage
	if cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Error("expected KeyUsageDigitalSignature")
	}

	// Check extended key usage
	hasServerAuth := false
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			hasServerAuth = true
			break
		}
	}
	if !hasServerAuth {
		t.Error("expected ExtKeyUsageServerAuth")
	}

	// Verify it's ECDSA P-256
	if cert.PublicKeyAlgorithm != x509.ECDSA {
		t.Errorf("PublicKeyAlgorithm = %v, want ECDSA", cert.PublicKeyAlgorithm)
	}
}

func TestGenerateCertificate_KeyContent(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "test_cert.pem")
	keyPath := filepath.Join(tmpDir, "test_key.pem")
	domain := "test.example.com"

	_, err := GenerateCertificate(certPath, keyPath, domain)
	if err != nil {
		t.Fatalf("GenerateCertificate failed: %v", err)
	}

	// Read and parse key
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("failed to read key: %v", err)
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		t.Fatal("failed to decode PEM block")
	}
	if block.Type != "EC PRIVATE KEY" {
		t.Errorf("PEM type = %q, want 'EC PRIVATE KEY'", block.Type)
	}

	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse EC private key: %v", err)
	}

	// Verify it's P-256
	if key.Curve.Params().Name != "P-256" {
		t.Errorf("curve = %q, want 'P-256'", key.Curve.Params().Name)
	}
}

func TestGenerateCertificate_Uniqueness(t *testing.T) {
	tmpDir := t.TempDir()

	fingerprints := make(map[string]bool)
	for i := 0; i < 5; i++ {
		certPath := filepath.Join(tmpDir, "cert"+string(rune('0'+i))+".pem")
		keyPath := filepath.Join(tmpDir, "key"+string(rune('0'+i))+".pem")

		fingerprint, err := GenerateCertificate(certPath, keyPath, "test.example.com")
		if err != nil {
			t.Fatalf("GenerateCertificate failed: %v", err)
		}

		if fingerprints[fingerprint] {
			t.Errorf("duplicate fingerprint generated: %s", fingerprint)
		}
		fingerprints[fingerprint] = true
	}
}

func TestReadCertificateFingerprint(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "test_cert.pem")
	keyPath := filepath.Join(tmpDir, "test_key.pem")
	domain := "test.example.com"

	expectedFingerprint, err := GenerateCertificate(certPath, keyPath, domain)
	if err != nil {
		t.Fatalf("GenerateCertificate failed: %v", err)
	}

	fingerprint, err := ReadCertificateFingerprint(certPath)
	if err != nil {
		t.Fatalf("ReadCertificateFingerprint failed: %v", err)
	}

	if fingerprint != expectedFingerprint {
		t.Errorf("fingerprint = %q, want %q", fingerprint, expectedFingerprint)
	}
}

func TestReadCertificateFingerprint_NotFound(t *testing.T) {
	_, err := ReadCertificateFingerprint("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadCertificateFingerprint_InvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "invalid.pem")

	if err := os.WriteFile(certPath, []byte("not valid pem"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := ReadCertificateFingerprint(certPath)
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestCertsExist(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "test_cert.pem")
	keyPath := filepath.Join(tmpDir, "test_key.pem")

	// Neither exists
	if CertsExist(certPath, keyPath) {
		t.Error("CertsExist should return false when files don't exist")
	}

	// Only cert exists
	if err := os.WriteFile(certPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if CertsExist(certPath, keyPath) {
		t.Error("CertsExist should return false when only cert exists")
	}

	// Both exist
	if err := os.WriteFile(keyPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if !CertsExist(certPath, keyPath) {
		t.Error("CertsExist should return true when both files exist")
	}
}

func TestManager_GetPaths(t *testing.T) {
	m := NewManagerWithDir("/test/certs")

	tests := []struct {
		domain   string
		wantCert string
		wantKey  string
	}{
		{
			domain:   "example.com",
			wantCert: "/test/certs/example_com_cert.pem",
			wantKey:  "/test/certs/example_com_key.pem",
		},
		{
			domain:   "sub.domain.example.com",
			wantCert: "/test/certs/sub_domain_example_com_cert.pem",
			wantKey:  "/test/certs/sub_domain_example_com_key.pem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			cert, key := m.GetPaths(tt.domain)
			if cert != tt.wantCert {
				t.Errorf("cert path = %q, want %q", cert, tt.wantCert)
			}
			if key != tt.wantKey {
				t.Errorf("key path = %q, want %q", key, tt.wantKey)
			}
		})
	}
}

func TestManager_GetOrCreate(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	domain := "test.example.com"

	// First call should create
	info1, err := m.GetOrCreate(domain)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	if info1 == nil {
		t.Fatal("expected non-nil CertInfo")
	}
	if info1.Domain != domain {
		t.Errorf("domain = %q, want %q", info1.Domain, domain)
	}
	if info1.Fingerprint == "" {
		t.Error("expected non-empty fingerprint")
	}

	// Second call should return same cert
	info2, err := m.GetOrCreate(domain)
	if err != nil {
		t.Fatalf("GetOrCreate (second call) failed: %v", err)
	}
	if info2.Fingerprint != info1.Fingerprint {
		t.Errorf("fingerprint changed: %q -> %q", info1.Fingerprint, info2.Fingerprint)
	}
}

func TestManager_Get(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	domain := "test.example.com"

	// Before generation
	info := m.Get(domain)
	if info != nil {
		t.Error("expected nil before cert generation")
	}

	// After generation
	_, err := m.Generate(domain)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	info = m.Get(domain)
	if info == nil {
		t.Fatal("expected non-nil after generation")
	}
	if info.Domain != domain {
		t.Errorf("domain = %q, want %q", info.Domain, domain)
	}
}

func TestManager_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	domain := "test.example.com"

	info, err := m.Generate(domain)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if info == nil {
		t.Fatal("expected non-nil CertInfo")
	}
	if info.Domain != domain {
		t.Errorf("domain = %q, want %q", info.Domain, domain)
	}
	if len(info.Fingerprint) != 64 {
		t.Errorf("fingerprint length = %d, want 64", len(info.Fingerprint))
	}

	// Verify file paths
	certPath, keyPath := m.GetPaths(domain)
	if info.CertPath != certPath {
		t.Errorf("cert path = %q, want %q", info.CertPath, certPath)
	}
	if info.KeyPath != keyPath {
		t.Errorf("key path = %q, want %q", info.KeyPath, keyPath)
	}
}

func TestManager_List(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	// Create multiple certs
	domains := []string{"a.example.com", "b.example.com", "c.example.com"}
	for _, domain := range domains {
		if _, err := m.Generate(domain); err != nil {
			t.Fatalf("Generate(%q) failed: %v", domain, err)
		}
	}

	// List should return all
	certs := m.List()
	if len(certs) != len(domains) {
		t.Errorf("List() returned %d certs, want %d", len(certs), len(domains))
	}

	// Verify all domains are present
	found := make(map[string]bool)
	for _, cert := range certs {
		found[cert.Domain] = true
	}
	for _, domain := range domains {
		if !found[domain] {
			t.Errorf("domain %q not found in list", domain)
		}
	}
}

func TestManager_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	domain := "test.example.com"

	// Generate first
	if _, err := m.Generate(domain); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify cert exists
	if m.Get(domain) == nil {
		t.Fatal("expected cert to exist before delete")
	}

	// Delete
	if err := m.Delete(domain); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify cert is gone
	if m.Get(domain) != nil {
		t.Error("expected cert to be deleted")
	}

	// Delete again should not error
	if err := m.Delete(domain); err != nil {
		t.Errorf("Delete of nonexistent cert failed: %v", err)
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager()

	// Should use default base dir
	cert, _ := m.GetPaths("test.com")
	if !strings.HasPrefix(cert, BaseDir) {
		t.Errorf("cert path = %q, expected prefix %q", cert, BaseDir)
	}
}

func TestFormatFingerprint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: "01:23:45:67:89:AB:CD:EF:01:23:45:67:89:AB:CD:EF:01:23:45:67:89:AB:CD:EF:01:23:45:67:89:AB:CD:EF",
		},
		{
			input:    "invalid",
			expected: "invalid",
		},
		{
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := FormatFingerprint(tt.input)
			if result != tt.expected {
				t.Errorf("FormatFingerprint(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatFingerprint_UpperCase(t *testing.T) {
	input := "aabbccdd" + strings.Repeat("00", 28)
	result := FormatFingerprint(input)

	// Should be uppercase
	if strings.ContainsAny(result, "abcdef") {
		t.Errorf("FormatFingerprint should return uppercase, got %q", result)
	}
}
