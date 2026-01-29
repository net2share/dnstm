package keys

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/net2share/dnstm/internal/system"
	"golang.org/x/crypto/curve25519"
)

const ConfigDir = "/etc/dnstt"

// Generate creates a new Curve25519 key pair for dnstt.
// Keys are stored as 64-character hex strings (32 bytes).
func Generate(privateKeyPath, publicKeyPath string) (publicKey string, err error) {
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

	// Set ownership to dnstm user so the service can read the keys
	if err := system.ChownToDnstm(privateKeyPath); err != nil {
		// Non-fatal: log but continue (user might not exist yet)
		_ = err
	}
	if err := system.ChownToDnstm(publicKeyPath); err != nil {
		_ = err
	}
	// Also chown the directory
	if err := system.ChownToDnstm(filepath.Dir(privateKeyPath)); err != nil {
		_ = err
	}

	return publicKeyHex, nil
}

func ReadPublicKey(publicKeyPath string) (string, error) {
	data, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", err
	}

	return string(data[:len(data)-1]), nil
}

func KeysExist(privateKeyPath, publicKeyPath string) bool {
	_, err1 := os.Stat(privateKeyPath)
	_, err2 := os.Stat(publicKeyPath)
	return err1 == nil && err2 == nil
}
