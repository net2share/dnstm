package slipstream

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// GenerateCertificate creates a self-signed ECDSA P-256 certificate.
// Returns the SHA256 fingerprint of the certificate.
func GenerateCertificate(certPath, keyPath, domain string) (fingerprint string, err error) {
	if err := os.MkdirAll(filepath.Dir(certPath), 0750); err != nil {
		return "", fmt.Errorf("failed to create cert directory: %w", err)
	}

	// Generate ECDSA P-256 private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   domain,
			Organization: []string{"Slipstream DNS Tunnel"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years validity
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Calculate SHA256 fingerprint
	hash := sha256.Sum256(certDER)
	fingerprint = hex.EncodeToString(hash[:])

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal private key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})

	// Write certificate file
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return "", fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write key file
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return "", fmt.Errorf("failed to write private key: %w", err)
	}

	return fingerprint, nil
}

// ReadCertificateFingerprint reads a certificate and returns its SHA256 fingerprint.
func ReadCertificateFingerprint(certPath string) (string, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return "", err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	hash := sha256.Sum256(block.Bytes)
	return hex.EncodeToString(hash[:]), nil
}

// CertsExist checks if both certificate files exist.
func CertsExist(certPath, keyPath string) bool {
	_, err1 := os.Stat(certPath)
	_, err2 := os.Stat(keyPath)
	return err1 == nil && err2 == nil
}

// FormatFingerprint formats a fingerprint for display (with colons).
func FormatFingerprint(fingerprint string) string {
	if len(fingerprint) != 64 {
		return fingerprint
	}

	result := ""
	for i := 0; i < len(fingerprint); i += 2 {
		if i > 0 {
			result += ":"
		}
		result += fingerprint[i : i+2]
	}
	return result
}
