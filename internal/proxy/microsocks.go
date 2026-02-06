package proxy

import (
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strings"

	"github.com/net2share/dnstm/internal/binary"
	"github.com/net2share/dnstm/internal/service"
)

const (
	MicrosocksServiceName = "microsocks"
	MicrosocksBindAddr    = "127.0.0.1"
)

// InstallMicrosocks downloads and installs the microsocks binary.
func InstallMicrosocks(progressFn func(downloaded, total int64)) error {
	mgr := binary.NewDefaultManager()
	_, err := mgr.EnsureInstalled(binary.BinaryMicrosocks)
	return err
}

// ConfigureMicrosocks creates the systemd service for microsocks with the specified port.
func ConfigureMicrosocks(port int) error {
	mgr := binary.NewDefaultManager()
	binaryPath, err := mgr.GetPath(binary.BinaryMicrosocks)
	if err != nil {
		return fmt.Errorf("microsocks binary not found: %w", err)
	}

	return service.CreateGenericService(&service.ServiceConfig{
		Name:             MicrosocksServiceName,
		Description:      "Microsocks SOCKS5 Proxy",
		User:             "nobody",
		Group:            getNobodyGroup(),
		ExecStart:        fmt.Sprintf("%s -i %s -p %d -q", binaryPath, MicrosocksBindAddr, port),
		ReadOnlyPaths:    []string{binaryPath},
		BindToPrivileged: false,
	})
}

// FindAvailablePort finds an available port in the range 10000-60000.
func FindAvailablePort() (int, error) {
	// Try random ports in the high range to avoid conflicts
	for i := 0; i < 100; i++ {
		port := 10000 + rand.Intn(50000) // Range: 10000-60000
		if isPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("could not find available port")
}

// isPortAvailable checks if a port is available for binding.
func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// StartMicrosocks enables and starts the microsocks service.
func StartMicrosocks() error {
	if err := service.EnableService(MicrosocksServiceName); err != nil {
		return err
	}
	return service.StartService(MicrosocksServiceName)
}

// RestartMicrosocks restarts the microsocks service.
func RestartMicrosocks() error {
	return service.RestartService(MicrosocksServiceName)
}

// StopMicrosocks stops the microsocks service.
func StopMicrosocks() error {
	return service.StopService(MicrosocksServiceName)
}

// IsMicrosocksInstalled checks if the microsocks binary is installed.
func IsMicrosocksInstalled() bool {
	mgr := binary.NewDefaultManager()
	_, err := mgr.GetPath(binary.BinaryMicrosocks)
	return err == nil
}

// IsMicrosocksRunning checks if the microsocks service is active.
func IsMicrosocksRunning() bool {
	return service.IsServiceActive(MicrosocksServiceName)
}

// getNobodyGroup returns the appropriate "nobody" group for the current system.
// Debian/Ubuntu use "nogroup", RHEL/Fedora use "nobody".
func getNobodyGroup() string {
	// Check if nogroup exists (Debian/Ubuntu)
	out, err := exec.Command("getent", "group", "nogroup").Output()
	if err == nil && strings.HasPrefix(string(out), "nogroup:") {
		return "nogroup"
	}
	// Fall back to nobody (RHEL/Fedora)
	return "nobody"
}

// UninstallMicrosocks removes the microsocks binary and service.
func UninstallMicrosocks() error {
	if service.IsServiceActive(MicrosocksServiceName) {
		service.StopService(MicrosocksServiceName)
	}
	if service.IsServiceEnabled(MicrosocksServiceName) {
		service.DisableService(MicrosocksServiceName)
	}
	service.RemoveService(MicrosocksServiceName)
	// Note: We don't remove the binary as it's managed by the binary manager
	return nil
}
