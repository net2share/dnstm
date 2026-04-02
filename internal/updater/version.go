// Package updater provides update functionality for dnstm and its binaries.
package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/net2share/go-corelib/binman"
)

const (
	// VersionManifestFile is the filename for the version manifest.
	VersionManifestFile = "versions.json"
)

// VersionManifest wraps binman.VersionManifest with DNSTM-specific path handling.
type VersionManifest struct {
	SlipstreamServer     string    `json:"slipstream-server,omitempty"`
	SlipstreamPlusServer string    `json:"slipstream-plus-server,omitempty"`
	SSServer             string    `json:"ssserver,omitempty"`
	Microsocks           string    `json:"microsocks,omitempty"`
	SSHTunUser           string    `json:"sshtun-user,omitempty"`
	VayDNSServer         string    `json:"vaydns-server,omitempty"`
	UpdatedAt            time.Time `json:"updated_at"`
	m *binman.VersionManifest
}

// GetManifestPath returns the path to the version manifest file.
func GetManifestPath() string {
	return filepath.Join("/etc/dnstm", VersionManifestFile)
}

// NewManifest creates a new empty version manifest.
func NewManifest() *VersionManifest {
	return &VersionManifest{m: binman.NewManifest()}
}

// LoadManifest loads the version manifest from disk, migrating from old format if needed.
func LoadManifest() (*VersionManifest, error) {
	path := GetManifestPath()

	// Migrate old format before loading (no-op if already new format or file missing).
	migrateManifestIfNeeded(path)

	m, err := binman.LoadManifest(path)
	if err != nil {
		return nil, err
	}

	return &VersionManifest{m: m}, nil
}

// Save saves the version manifest to disk.
func (vm *VersionManifest) Save() error {
	return vm.m.Save(GetManifestPath())
}

// GetVersion returns the installed version for a binary.
func (m *VersionManifest) GetVersion(binaryName string) string {
	switch binaryName {
	case "slipstream-server":
		return m.SlipstreamServer
	case "ssserver":
		return m.SSServer
	case "microsocks":
		return m.Microsocks
	case "sshtun-user":
		return m.SSHTunUser
	case "vaydns-server":
		return m.VayDNSServer
	case "slipstream-plus-server":
		return m.SlipstreamPlusServer
	default:
		return ""
	}
}

// SetVersion sets the installed version for a binary.
func (m *VersionManifest) SetVersion(binaryName, version string) {
	switch binaryName {
	case "slipstream-server":
		m.SlipstreamServer = version
	case "ssserver":
		m.SSServer = version
	case "microsocks":
		m.Microsocks = version
	case "sshtun-user":
		m.SSHTunUser = version
	case "vaydns-server":
		m.VayDNSServer = version
	case "slipstream-plus-server":
		m.SlipstreamPlusServer = version
	}
func (vm *VersionManifest) GetVersion(name string) string {
	return vm.m.GetVersion(name)
}

// SetVersion sets the installed version for a binary.
func (vm *VersionManifest) SetVersion(name, version string) {
	vm.m.SetVersion(name, version)
}

// CompareVersions compares two version strings.
// Returns -1 if v1 < v2, 0 if equal, 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	return binman.CompareVersions(v1, v2)
}

// IsNewer returns true if newVersion is newer than currentVersion.
func IsNewer(currentVersion, newVersion string) bool {
	return binman.IsNewer(currentVersion, newVersion)
}

// legacyManifest represents the old flat manifest format used before binman migration.
type legacyManifest struct {
	SlipstreamServer string    `json:"slipstream-server,omitempty"`
	SSServer         string    `json:"ssserver,omitempty"`
	Microsocks       string    `json:"microsocks,omitempty"`
	SSHTunUser       string    `json:"sshtun-user,omitempty"`
	VayDNSServer     string    `json:"vaydns-server,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// migrateManifestIfNeeded detects the old flat manifest format and converts it
// to the new binman format. This is a no-op if the file doesn't exist or is
// already in the new format.
func migrateManifestIfNeeded(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // File doesn't exist, nothing to migrate
	}

	// Try to detect format by looking for the "versions" key
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}

	// If "versions" key exists, it's already the new format
	if _, ok := raw["versions"]; ok {
		return
	}

	// Old format detected — read as legacy struct
	var old legacyManifest
	if err := json.Unmarshal(data, &old); err != nil {
		return
	}

	// Convert to new format
	m := binman.NewManifest()
	if old.SlipstreamServer != "" {
		m.SetVersion("slipstream-server", old.SlipstreamServer)
	}
	if old.SSServer != "" {
		m.SetVersion("ssserver", old.SSServer)
	}
	if old.Microsocks != "" {
		m.SetVersion("microsocks", old.Microsocks)
	}
	if old.SSHTunUser != "" {
		m.SetVersion("sshtun-user", old.SSHTunUser)
	}
	if old.VayDNSServer != "" {
		m.SetVersion("vaydns-server", old.VayDNSServer)
	}
	m.UpdatedAt = old.UpdatedAt

	// Save in new format (overwrites old file)
	_ = m.Save(path)
}
