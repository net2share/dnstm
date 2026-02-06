// Package updater provides update functionality for dnstm and its binaries.
package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// VersionManifestFile is the filename for the version manifest.
	VersionManifestFile = "versions.json"
)

// VersionManifest stores installed versions of transport binaries.
type VersionManifest struct {
	SlipstreamServer string    `json:"slipstream-server,omitempty"`
	SSServer         string    `json:"ssserver,omitempty"`
	Microsocks       string    `json:"microsocks,omitempty"`
	SSHTunUser       string    `json:"sshtun-user,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// GetManifestPath returns the path to the version manifest file.
func GetManifestPath() string {
	return filepath.Join("/etc/dnstm", VersionManifestFile)
}

// LoadManifest loads the version manifest from disk.
func LoadManifest() (*VersionManifest, error) {
	path := GetManifestPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &VersionManifest{}, nil
		}
		return nil, err
	}

	var manifest VersionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// Save saves the version manifest to disk.
func (m *VersionManifest) Save() error {
	path := GetManifestPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	m.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
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
	}
}

// CompareVersions compares two version strings.
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
//
// Handles both semver (v1.23.0) and date-based (v2026.01.29) versions.
func CompareVersions(v1, v2 string) int {
	// Normalize: remove 'v' prefix
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Empty version is always older
	if v1 == "" && v2 == "" {
		return 0
	}
	if v1 == "" {
		return -1
	}
	if v2 == "" {
		return 1
	}

	// Dev/unknown versions are always older than any real version
	if isDevVersion(v1) && !isDevVersion(v2) {
		return -1
	}
	if !isDevVersion(v1) && isDevVersion(v2) {
		return 1
	}
	if isDevVersion(v1) && isDevVersion(v2) {
		return 0
	}

	// Check if date-based (YYYY.MM.DD format)
	datePattern := regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}$`)
	if datePattern.MatchString(v1) && datePattern.MatchString(v2) {
		return strings.Compare(v1, v2)
	}

	// Parse as semver
	parts1 := parseVersion(v1)
	parts2 := parseVersion(v2)

	// Compare each part
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}

	return 0
}

// isDevVersion returns true for non-release versions like "dev", "unknown", etc.
func isDevVersion(v string) bool {
	switch v {
	case "dev", "unknown", "latest":
		return true
	}
	// No digits at all means not a real version
	for _, c := range v {
		if c >= '0' && c <= '9' {
			return false
		}
	}
	return true
}

// parseVersion extracts numeric parts from a version string.
func parseVersion(v string) []int {
	// Split by non-numeric characters
	re := regexp.MustCompile(`[^\d]+`)
	parts := re.Split(v, -1)

	var result []int
	for _, p := range parts {
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		result = append(result, n)
	}

	return result
}

// IsNewer returns true if newVersion is newer than currentVersion.
func IsNewer(currentVersion, newVersion string) bool {
	return CompareVersions(currentVersion, newVersion) < 0
}
