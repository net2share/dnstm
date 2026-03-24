package transport

import (
	"github.com/net2share/dnstm/internal/binary"
)

// IsInstalled checks if all required transport binaries are installed.
func IsInstalled() bool {
	mgr := binary.NewDefaultManager()
	binaries := []binary.BinaryType{
		binary.BinaryDNSTTServer,
		binary.BinarySlipstreamServer,
		binary.BinarySSServer,
	}

	for _, bin := range binaries {
		if _, err := mgr.GetPath(bin); err != nil {
			return false
		}
	}

	return true
}

// GetMissingBinaries returns a list of missing transport binaries.
func GetMissingBinaries() []string {
	mgr := binary.NewDefaultManager()
	var missing []string

	binaries := []binary.BinaryType{
		binary.BinaryDNSTTServer,
		binary.BinarySlipstreamServer,
		binary.BinarySSServer,
	}

	for _, bin := range binaries {
		if _, err := mgr.GetPath(bin); err != nil {
			missing = append(missing, string(bin))
		}
	}

	return missing
}
