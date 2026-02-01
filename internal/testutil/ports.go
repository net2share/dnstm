package testutil

import (
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	portMu   sync.Mutex
	lastPort = 15000 // Start allocation from this port
)

// AllocatePort finds an available port and returns it.
// Uses a sequential allocation strategy with fallback to random ports.
func AllocatePort() (int, error) {
	portMu.Lock()
	defer portMu.Unlock()

	// Try sequential ports first
	for i := 0; i < 100; i++ {
		port := lastPort + 1 + i
		if port > 65535 {
			port = 15000 + (port - 65535)
		}
		if isPortAvailable(port) {
			lastPort = port
			return port, nil
		}
	}

	// Fall back to system-allocated port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to allocate port: %w", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	lastPort = addr.Port
	return addr.Port, nil
}

// AllocatePorts allocates n consecutive-ish ports.
func AllocatePorts(n int) ([]int, error) {
	ports := make([]int, n)
	for i := 0; i < n; i++ {
		port, err := AllocatePort()
		if err != nil {
			return nil, err
		}
		ports[i] = port
	}
	return ports, nil
}

// isPortAvailable checks if a port is available for binding.
func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()

	// Also check UDP
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return false
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return false
	}
	conn.Close()

	return true
}

// WaitForPort waits for a port to become available for connection.
// This is useful for waiting for a server to start.
func WaitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("port %d not available after %v", port, timeout)
}

// WaitForUDPPort waits for a UDP port to become available.
// This is useful for waiting for DNS servers to start.
func WaitForUDPPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		// Try to bind to the port - if it fails with "address already in use", the server is listening
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		conn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			// Port is in use - server is listening
			return nil
		}
		conn.Close()
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("UDP port %d not in use after %v", port, timeout)
}

// WaitForPortClosed waits for a port to become free (not listening).
func WaitForPortClosed(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			// Port is closed
			return nil
		}
		conn.Close()
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("port %d still in use after %v", port, timeout)
}

// ResetPortCounter resets the port counter (for test isolation).
func ResetPortCounter() {
	portMu.Lock()
	defer portMu.Unlock()
	lastPort = 15000
}
