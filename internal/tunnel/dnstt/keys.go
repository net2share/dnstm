package dnstt

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/curve25519"
)

// GenerateKeys creates a new Curve25519 key pair for dnstt.
// Keys are stored as 64-character hex strings (32 bytes).
func GenerateKeys(privateKeyPath, publicKeyPath string) (publicKey string, err error) {
	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0750); err != nil {
		return "", fmt.Errorf("failed to create key directory: %w", err)
	}

	// Generate 32 random bytes for private key
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Clamp the private key for Curve25519
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Derive public key
	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privateKey)

	// Encode as hex (dnstt expects 64-character hex strings)
	privateKeyHex := hex.EncodeToString(privateKey[:])
	publicKeyHex := hex.EncodeToString(pubKey[:])

	if err := os.WriteFile(privateKeyPath, []byte(privateKeyHex+"\n"), 0600); err != nil {
		return "", fmt.Errorf("failed to write private key: %w", err)
	}

	if err := os.WriteFile(publicKeyPath, []byte(publicKeyHex+"\n"), 0644); err != nil {
		return "", fmt.Errorf("failed to write public key: %w", err)
	}

	return publicKeyHex, nil
}

// ReadPublicKey reads the public key from a file.
func ReadPublicKey(publicKeyPath string) (string, error) {
	data, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", err
	}

	// Strip trailing newline
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	return string(data), nil
}

// KeysExist checks if both key files exist.
func KeysExist(privateKeyPath, publicKeyPath string) bool {
	_, err1 := os.Stat(privateKeyPath)
	_, err2 := os.Stat(publicKeyPath)
	return err1 == nil && err2 == nil
}
