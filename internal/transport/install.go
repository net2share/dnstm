package transport

import (
	"fmt"

	"github.com/net2share/dnstm/internal/binary"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/log"
	"github.com/net2share/dnstm/internal/types"
	"github.com/net2share/go-corelib/tui"
)

// StatusFunc is a callback for reporting installation status messages.
type StatusFunc func(message string)

// EnsureBinariesInstalled checks and installs required binaries for a transport type.
// This is the legacy function that accepts types.TransportType.
func EnsureBinariesInstalled(t types.TransportType) error {
	switch t {
	case types.TypeSlipstreamShadowsocks:
		if err := EnsureSlipstreamInstalled(); err != nil {
			return err
		}
		return EnsureShadowsocksInstalled()
	case types.TypeSlipstreamSocks, types.TypeSlipstreamSSH:
		return EnsureSlipstreamInstalled()
	case types.TypeDNSTTSocks, types.TypeDNSTTSSH:
		return EnsureDnsttInstalled()
	default:
		return nil
	}
}

// EnsureTransportBinariesInstalled checks and installs required binaries for a transport type.
// This function accepts the new config.TransportType.
func EnsureTransportBinariesInstalled(transport config.TransportType) error {
	switch transport {
	case config.TransportSlipstream:
		return EnsureSlipstreamInstalled()
	case config.TransportDNSTT:
		return EnsureDnsttInstalled()
	default:
		return nil
	}
}

// EnsureBackendBinariesInstalled checks and installs required binaries for a backend type.
func EnsureBackendBinariesInstalled(backend config.BackendType) error {
	switch backend {
	case config.BackendShadowsocks:
		return EnsureShadowsocksInstalled()
	default:
		return nil
	}
}

// EnsureDnsttInstalled installs dnstt-server if not present.
func EnsureDnsttInstalled() error {
	return EnsureDnsttInstalledWithStatus(nil)
}

// EnsureDnsttInstalledWithStatus installs dnstt-server with status callback.
func EnsureDnsttInstalledWithStatus(statusFn StatusFunc) error {
	return ensureBinaryInstalled(binary.BinaryDNSTTServer, "dnstt-server", statusFn)
}

// EnsureSlipstreamInstalled installs slipstream-server if not present.
func EnsureSlipstreamInstalled() error {
	return EnsureSlipstreamInstalledWithStatus(nil)
}

// EnsureSlipstreamInstalledWithStatus installs slipstream-server with status callback.
func EnsureSlipstreamInstalledWithStatus(statusFn StatusFunc) error {
	return ensureBinaryInstalled(binary.BinarySlipstreamServer, "slipstream-server", statusFn)
}

// EnsureShadowsocksInstalled installs ssserver if not present.
func EnsureShadowsocksInstalled() error {
	return EnsureShadowsocksInstalledWithStatus(nil)
}

// EnsureShadowsocksInstalledWithStatus installs ssserver with status callback.
func EnsureShadowsocksInstalledWithStatus(statusFn StatusFunc) error {
	return ensureBinaryInstalled(binary.BinarySSServer, "ssserver", statusFn)
}

// EnsureSSHTunUserInstalled installs sshtun-user if not present.
func EnsureSSHTunUserInstalled() error {
	return EnsureSSHTunUserInstalledWithStatus(nil)
}

// EnsureSSHTunUserInstalledWithStatus installs sshtun-user with status callback.
func EnsureSSHTunUserInstalledWithStatus(statusFn StatusFunc) error {
	return ensureBinaryInstalled(binary.BinarySSHTunUser, "sshtun-user", statusFn)
}

// IsSSHTunUserInstalled checks if sshtun-user binary is installed.
func IsSSHTunUserInstalled() bool {
	mgr := binary.NewDefaultManager()
	_, err := mgr.GetPath(binary.BinarySSHTunUser)
	return err == nil
}

// ensureBinaryInstalled uses the binary manager to ensure a binary is available.
func ensureBinaryInstalled(binType binary.BinaryType, displayName string, statusFn StatusFunc) error {
	mgr := binary.NewDefaultManager()

	// EnsureInstalled downloads if needed
	path, err := mgr.EnsureInstalled(binType)
	if err != nil {
		return fmt.Errorf("failed to install %s: %w", displayName, err)
	}

	log.Debug("%s installed at %s", displayName, path)

	// Route status through callback if provided, otherwise use direct tui output
	if statusFn != nil {
		statusFn(fmt.Sprintf("%s installed", displayName))
	} else {
		tui.PrintStatus(fmt.Sprintf("%s installed", displayName))
	}
	return nil
}
