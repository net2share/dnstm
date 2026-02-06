package updater

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		// Semver comparisons
		{"equal semver", "v1.0.0", "v1.0.0", 0},
		{"v1 less than v2", "v1.0.0", "v1.0.1", -1},
		{"v1 greater than v2", "v1.0.1", "v1.0.0", 1},
		{"major version diff", "v1.0.0", "v2.0.0", -1},
		{"minor version diff", "v1.1.0", "v1.2.0", -1},

		// Date-based comparisons
		{"equal date", "v2026.01.29", "v2026.01.29", 0},
		{"date v1 older", "v2025.12.15", "v2026.01.29", -1},
		{"date v1 newer", "v2026.02.01", "v2026.01.29", 1},

		// Dev/unknown versions always older than real versions
		{"dev vs semver", "dev", "v0.6.2", -1},
		{"dev vs date", "dev", "v2026.01.29", -1},
		{"dev vs v0.0.1", "dev", "v0.0.1", -1},
		{"semver vs dev", "v1.0.0", "dev", 1},
		{"dev vs dev", "dev", "dev", 0},
		{"unknown vs semver", "unknown", "v1.0.0", -1},
		{"latest vs semver", "latest", "v1.0.0", -1},

		// Edge cases
		{"empty v1", "", "v1.0.0", -1},
		{"empty v2", "v1.0.0", "", 1},
		{"both empty", "", "", 0},
		{"without v prefix", "1.0.0", "1.0.1", -1},

		// Real-world versions
		{"shadowsocks", "v1.22.0", "v1.23.0", -1},
		{"slipstream", "v2025.11.20", "v2026.01.29", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareVersions(tt.v1, tt.v2)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current string
		new     string
		want    bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.1", "v1.0.0", false},
		{"v1.0.0", "v1.0.0", false},
		{"", "v1.0.0", true},
		{"dev", "v0.6.2", true},
		{"dev", "v0.0.1", true},
		{"v2025.01.01", "v2026.01.29", true},
	}

	for _, tt := range tests {
		t.Run(tt.current+"_"+tt.new, func(t *testing.T) {
			got := IsNewer(tt.current, tt.new)
			if got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.new, got, tt.want)
			}
		})
	}
}
