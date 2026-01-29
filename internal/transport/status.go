package transport

import (
	"os"
)

// IsInstalled checks if all required transport binaries are installed.
func IsInstalled() bool {
	binaries := []string{
		DNSTTBinary,
		SlipstreamBinary,
		SSServerBinary,
	}

	for _, bin := range binaries {
		if _, err := os.Stat(bin); err != nil {
			return false
		}
	}

	return true
}

// GetMissingBinaries returns a list of missing transport binaries.
func GetMissingBinaries() []string {
	var missing []string

	binaries := map[string]string{
		"dnstt-server":      DNSTTBinary,
		"slipstream-server": SlipstreamBinary,
		"ssserver":          SSServerBinary,
	}

	for name, path := range binaries {
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, name)
		}
	}

	return missing
}
