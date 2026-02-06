package keys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	BaseDir = "/etc/dnstm/keys"
)

// Manager handles DNSTT key operations.
type Manager struct {
	baseDir string
}

// KeyInfo holds key information.
type KeyInfo struct {
	Domain         string
	PrivateKeyPath string
	PublicKeyPath  string
	PublicKey      string
}

// NewManager creates a new key manager.
func NewManager() *Manager {
	return &Manager{
		baseDir: BaseDir,
	}
}

// NewManagerWithDir creates a new key manager with a custom base directory.
func NewManagerWithDir(baseDir string) *Manager {
	return &Manager{
		baseDir: baseDir,
	}
}

// GetOrCreate returns existing key info for a domain, or creates a new one.
func (m *Manager) GetOrCreate(domain string) (*KeyInfo, error) {
	info := m.Get(domain)
	if info != nil && info.PublicKey != "" {
		return info, nil
	}

	return m.Generate(domain)
}

// Get returns key info for a domain if it exists.
func (m *Manager) Get(domain string) *KeyInfo {
	privPath, pubPath := m.GetPaths(domain)

	if !KeysExist(privPath, pubPath) {
		return nil
	}

	pubKey, err := ReadPublicKey(pubPath)
	if err != nil {
		return nil
	}

	return &KeyInfo{
		Domain:         domain,
		PrivateKeyPath: privPath,
		PublicKeyPath:  pubPath,
		PublicKey:      pubKey,
	}
}

// Generate creates new keys for a domain.
func (m *Manager) Generate(domain string) (*KeyInfo, error) {
	privPath, pubPath := m.GetPaths(domain)

	pubKey, err := Generate(privPath, pubPath)
	if err != nil {
		return nil, err
	}

	return &KeyInfo{
		Domain:         domain,
		PrivateKeyPath: privPath,
		PublicKeyPath:  pubPath,
		PublicKey:      pubKey,
	}, nil
}

// List returns all keys in the base directory.
func (m *Manager) List() []*KeyInfo {
	var keys []*KeyInfo

	files, err := os.ReadDir(m.baseDir)
	if err != nil {
		return keys
	}

	// Find all key files and extract domains
	seen := make(map[string]bool)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), "_server.key") {
			domain := strings.TrimSuffix(file.Name(), "_server.key")
			domain = strings.ReplaceAll(domain, "_", ".")
			if !seen[domain] {
				seen[domain] = true
				if info := m.Get(domain); info != nil {
					keys = append(keys, info)
				}
			}
		}
	}

	return keys
}

// Delete removes keys for a domain.
func (m *Manager) Delete(domain string) error {
	privPath, pubPath := m.GetPaths(domain)

	if err := os.Remove(privPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove private key: %w", err)
	}

	if err := os.Remove(pubPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove public key: %w", err)
	}

	return nil
}

// GetPaths returns the private and public key paths for a domain.
func (m *Manager) GetPaths(domain string) (privPath, pubPath string) {
	sanitized := strings.ReplaceAll(domain, ".", "_")
	privPath = filepath.Join(m.baseDir, sanitized+"_server.key")
	pubPath = filepath.Join(m.baseDir, sanitized+"_server.pub")
	return
}

