package keys

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "test_server.key")
	pubPath := filepath.Join(tmpDir, "test_server.pub")

	pubKey, err := Generate(privPath, pubPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Public key should be 64 hex characters
	if len(pubKey) != 64 {
		t.Errorf("public key length = %d, want 64", len(pubKey))
	}

	// Should be valid hex
	_, err = hex.DecodeString(pubKey)
	if err != nil {
		t.Errorf("public key is not valid hex: %v", err)
	}

	// Files should exist
	if _, err := os.Stat(privPath); err != nil {
		t.Errorf("private key file not found: %v", err)
	}
	if _, err := os.Stat(pubPath); err != nil {
		t.Errorf("public key file not found: %v", err)
	}

	// Private key file should have restricted permissions
	info, err := os.Stat(privPath)
	if err != nil {
		t.Fatalf("failed to stat private key: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("private key permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestGenerate_KeyFormat(t *testing.T) {
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "test_server.key")
	pubPath := filepath.Join(tmpDir, "test_server.pub")

	pubKey, err := Generate(privPath, pubPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Read files and verify format
	privData, err := os.ReadFile(privPath)
	if err != nil {
		t.Fatalf("failed to read private key: %v", err)
	}

	pubData, err := os.ReadFile(pubPath)
	if err != nil {
		t.Fatalf("failed to read public key: %v", err)
	}

	// Both should be 64 hex chars + newline
	privHex := strings.TrimSpace(string(privData))
	pubHex := strings.TrimSpace(string(pubData))

	if len(privHex) != 64 {
		t.Errorf("private key length = %d, want 64", len(privHex))
	}
	if len(pubHex) != 64 {
		t.Errorf("public key length = %d, want 64", len(pubHex))
	}

	// Public key should match return value
	if pubHex != pubKey {
		t.Errorf("public key mismatch: file=%q, returned=%q", pubHex, pubKey)
	}
}

func TestGenerate_Uniqueness(t *testing.T) {
	tmpDir := t.TempDir()

	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		privPath := filepath.Join(tmpDir, "test"+string(rune('0'+i))+"_server.key")
		pubPath := filepath.Join(tmpDir, "test"+string(rune('0'+i))+"_server.pub")

		pubKey, err := Generate(privPath, pubPath)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		if keys[pubKey] {
			t.Errorf("duplicate public key generated: %s", pubKey)
		}
		keys[pubKey] = true
	}
}

func TestReadPublicKey(t *testing.T) {
	tmpDir := t.TempDir()
	pubPath := filepath.Join(tmpDir, "test_server.pub")

	// Write a test key
	expectedKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := os.WriteFile(pubPath, []byte(expectedKey+"\n"), 0644); err != nil {
		t.Fatalf("failed to write test key: %v", err)
	}

	key, err := ReadPublicKey(pubPath)
	if err != nil {
		t.Fatalf("ReadPublicKey failed: %v", err)
	}

	if key != expectedKey {
		t.Errorf("key = %q, want %q", key, expectedKey)
	}
}

func TestReadPublicKey_NotFound(t *testing.T) {
	_, err := ReadPublicKey("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestKeysExist(t *testing.T) {
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "test_server.key")
	pubPath := filepath.Join(tmpDir, "test_server.pub")

	// Neither exists
	if KeysExist(privPath, pubPath) {
		t.Error("KeysExist should return false when files don't exist")
	}

	// Only private exists
	if err := os.WriteFile(privPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if KeysExist(privPath, pubPath) {
		t.Error("KeysExist should return false when only private key exists")
	}

	// Both exist
	if err := os.WriteFile(pubPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if !KeysExist(privPath, pubPath) {
		t.Error("KeysExist should return true when both files exist")
	}
}

func TestManager_GetPaths(t *testing.T) {
	m := NewManagerWithDir("/test/keys")

	tests := []struct {
		domain   string
		wantPriv string
		wantPub  string
	}{
		{
			domain:   "example.com",
			wantPriv: "/test/keys/example_com_server.key",
			wantPub:  "/test/keys/example_com_server.pub",
		},
		{
			domain:   "sub.domain.example.com",
			wantPriv: "/test/keys/sub_domain_example_com_server.key",
			wantPub:  "/test/keys/sub_domain_example_com_server.pub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			priv, pub := m.GetPaths(tt.domain)
			if priv != tt.wantPriv {
				t.Errorf("private path = %q, want %q", priv, tt.wantPriv)
			}
			if pub != tt.wantPub {
				t.Errorf("public path = %q, want %q", pub, tt.wantPub)
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
		t.Fatal("expected non-nil KeyInfo")
	}
	if info1.Domain != domain {
		t.Errorf("domain = %q, want %q", info1.Domain, domain)
	}
	if info1.PublicKey == "" {
		t.Error("expected non-empty public key")
	}

	// Second call should return same key
	info2, err := m.GetOrCreate(domain)
	if err != nil {
		t.Fatalf("GetOrCreate (second call) failed: %v", err)
	}
	if info2.PublicKey != info1.PublicKey {
		t.Errorf("public key changed: %q -> %q", info1.PublicKey, info2.PublicKey)
	}
}

func TestManager_Get(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	domain := "test.example.com"

	// Before generation
	info := m.Get(domain)
	if info != nil {
		t.Error("expected nil before key generation")
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
		t.Fatal("expected non-nil KeyInfo")
	}
	if info.Domain != domain {
		t.Errorf("domain = %q, want %q", info.Domain, domain)
	}
	if len(info.PublicKey) != 64 {
		t.Errorf("public key length = %d, want 64", len(info.PublicKey))
	}

	// Verify file paths
	privPath, pubPath := m.GetPaths(domain)
	if info.PrivateKeyPath != privPath {
		t.Errorf("private key path = %q, want %q", info.PrivateKeyPath, privPath)
	}
	if info.PublicKeyPath != pubPath {
		t.Errorf("public key path = %q, want %q", info.PublicKeyPath, pubPath)
	}
}

func TestManager_List(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)

	// Create multiple keys
	domains := []string{"a.example.com", "b.example.com", "c.example.com"}
	for _, domain := range domains {
		if _, err := m.Generate(domain); err != nil {
			t.Fatalf("Generate(%q) failed: %v", domain, err)
		}
	}

	// List should return all
	keys := m.List()
	if len(keys) != len(domains) {
		t.Errorf("List() returned %d keys, want %d", len(keys), len(domains))
	}

	// Verify all domains are present
	found := make(map[string]bool)
	for _, key := range keys {
		found[key.Domain] = true
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

	// Verify keys exist
	if m.Get(domain) == nil {
		t.Fatal("expected keys to exist before delete")
	}

	// Delete
	if err := m.Delete(domain); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify keys are gone
	if m.Get(domain) != nil {
		t.Error("expected keys to be deleted")
	}

	// Delete again should not error
	if err := m.Delete(domain); err != nil {
		t.Errorf("Delete of nonexistent keys failed: %v", err)
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager()

	// Should use default base dir
	priv, _ := m.GetPaths("test.com")
	if !strings.HasPrefix(priv, BaseDir) {
		t.Errorf("private path = %q, expected prefix %q", priv, BaseDir)
	}
}
