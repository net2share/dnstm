package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/net2share/dnstm/internal/binary"
	"github.com/net2share/dnstm/internal/testutil"
)

// E2EEnv provides an environment for E2E tests.
type E2EEnv struct {
	T         *testing.T
	TempDir   string
	BinDir    string
	ConfigDir string
	Processes []*exec.Cmd
	mu        sync.Mutex
}

// NewE2EEnv creates a new E2E test environment.
func NewE2EEnv(t *testing.T) *E2EEnv {
	t.Helper()

	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	configDir := filepath.Join(tmpDir, "config")

	for _, dir := range []string{binDir, configDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	env := &E2EEnv{
		T:         t,
		TempDir:   tmpDir,
		BinDir:    binDir,
		ConfigDir: configDir,
		Processes: make([]*exec.Cmd, 0),
	}

	t.Cleanup(env.Cleanup)

	return env
}

// StartProcess starts a process and tracks it for cleanup.
func (e *E2EEnv) StartProcess(name string, args ...string) (*exec.Cmd, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start %s: %w", name, err)
	}

	e.Processes = append(e.Processes, cmd)
	return cmd, nil
}

// StartBackgroundProcess starts a process in the background with output capture.
func (e *E2EEnv) StartBackgroundProcess(name string, args ...string) (*exec.Cmd, io.ReadCloser, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	cmd := exec.Command(name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start %s: %w", name, err)
	}

	e.Processes = append(e.Processes, cmd)
	return cmd, stdout, nil
}

// Cleanup stops all running processes.
func (e *E2EEnv) Cleanup() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, cmd := range e.Processes {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}
}

// WaitForPort waits for a port to become available.
func (e *E2EEnv) WaitForPort(port int, timeout time.Duration) error {
	return testutil.WaitForPort(port, timeout)
}

// WaitForPortClosed waits for a port to close.
func (e *E2EEnv) WaitForPortClosed(port int, timeout time.Duration) error {
	return testutil.WaitForPortClosed(port, timeout)
}

// WaitForUDPPort waits for a UDP port to become available (server listening).
func (e *E2EEnv) WaitForUDPPort(port int, timeout time.Duration) error {
	return testutil.WaitForUDPPort(port, timeout)
}

// AllocatePort allocates an available port.
func (e *E2EEnv) AllocatePort() (int, error) {
	return testutil.AllocatePort()
}

// TestSOCKSProxy tests connectivity through a SOCKS5 proxy.
func (e *E2EEnv) TestSOCKSProxy(socksAddr string, targetURL string) error {
	// Create a SOCKS5 dialer
	dialer, err := NewSOCKS5Dialer(socksAddr)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Create an HTTP client using the SOCKS5 dialer
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// Make a request
	resp, err := client.Get(targetURL)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// SOCKS5Dialer provides a simple SOCKS5 dialer.
type SOCKS5Dialer struct {
	addr string
}

// NewSOCKS5Dialer creates a new SOCKS5 dialer.
func NewSOCKS5Dialer(addr string) (*SOCKS5Dialer, error) {
	return &SOCKS5Dialer{addr: addr}, nil
}

// Dial connects through the SOCKS5 proxy.
func (d *SOCKS5Dialer) Dial(network, addr string) (net.Conn, error) {
	// Connect to SOCKS5 proxy
	conn, err := net.DialTimeout("tcp", d.addr, 10*time.Second)
	if err != nil {
		return nil, err
	}

	// SOCKS5 handshake
	// Version 5, 1 auth method, no auth
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		conn.Close()
		return nil, err
	}

	// Read server choice
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		conn.Close()
		return nil, err
	}
	if buf[0] != 0x05 || buf[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 auth failed")
	}

	// Parse target address
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	var port int
	fmt.Sscanf(portStr, "%d", &port)

	// Build connect request
	req := []byte{
		0x05, // Version
		0x01, // Connect
		0x00, // Reserved
		0x03, // Domain name
		byte(len(host)),
	}
	req = append(req, []byte(host)...)
	req = append(req, byte(port>>8), byte(port&0xff))

	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}

	// Read response
	resp := make([]byte, 10)
	if _, err := io.ReadFull(conn, resp[:4]); err != nil {
		conn.Close()
		return nil, err
	}

	if resp[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 connect failed: %d", resp[1])
	}

	// Read rest of response based on address type
	switch resp[3] {
	case 0x01: // IPv4
		io.ReadFull(conn, resp[4:10])
	case 0x03: // Domain
		var length byte
		io.ReadFull(conn, resp[4:5])
		length = resp[4]
		io.ReadFull(conn, make([]byte, int(length)+2))
	case 0x04: // IPv6
		io.ReadFull(conn, make([]byte, 18))
	}

	return conn, nil
}

// WriteFile writes a file in the temp directory.
func (e *E2EEnv) WriteFile(name, content string) string {
	path := filepath.Join(e.ConfigDir, name)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.T.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.T.Fatalf("failed to write file: %v", err)
	}
	return path
}

// GetBinaryPath returns the path to a binary using the test binary manager.
func (e *E2EEnv) GetBinaryPath(name string) string {
	// Map name to binary type
	binType := nameToBinaryType(name)
	if binType == "" {
		e.T.Fatalf("unknown binary: %s", name)
	}

	path, err := testBinManager.GetPath(binType)
	if err != nil {
		e.T.Fatalf("failed to get binary %s: %v", name, err)
	}
	return path
}

// nameToBinaryType converts a binary name to its BinaryType.
func nameToBinaryType(name string) binary.BinaryType {
	switch name {
	case "dnstt-client":
		return binary.BinaryDNSTTClient
	case "dnstt-server":
		return binary.BinaryDNSTTServer
	case "slipstream-client":
		return binary.BinarySlipstreamClient
	case "slipstream-server":
		return binary.BinarySlipstreamServer
	case "sslocal":
		return binary.BinarySSLocal
	case "ssserver":
		return binary.BinarySSServer
	case "microsocks":
		return binary.BinaryMicrosocks
	default:
		return ""
	}
}
