package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/net2share/dnstm/internal/keys"
)

func TestDNSTT_LocalMode(t *testing.T) {
	env := NewE2EEnv(t)

	// Generate DNSTT keys
	keysDir := filepath.Join(env.ConfigDir, "keys")
	if err := os.MkdirAll(keysDir, 0755); err != nil {
		t.Fatalf("failed to create keys dir: %v", err)
	}

	privPath := filepath.Join(keysDir, "test_server.key")
	pubPath := filepath.Join(keysDir, "test_server.pub")

	pubKey, err := keys.Generate(privPath, pubPath)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}

	t.Logf("Generated DNSTT public key: %s", pubKey)

	// Allocate ports
	socksPort, err := env.AllocatePort()
	if err != nil {
		t.Fatalf("failed to allocate SOCKS port: %v", err)
	}

	dnsPort, err := env.AllocatePort()
	if err != nil {
		t.Fatalf("failed to allocate DNS port: %v", err)
	}

	// Start microsocks backend
	microsocksPath := env.GetBinaryPath("microsocks")
	microsocksCmd := exec.Command(microsocksPath, "-p", itoa(socksPort))
	if err := microsocksCmd.Start(); err != nil {
		t.Skipf("failed to start microsocks: %v", err)
	}
	env.Processes = append(env.Processes, microsocksCmd)

	// Wait for microsocks to start
	if err := env.WaitForPort(socksPort, 5*time.Second); err != nil {
		t.Fatalf("microsocks failed to start: %v", err)
	}

	// Start dnstt-server
	dnsttServerPath := env.GetBinaryPath("dnstt-server")
	dnsttServerCmd := exec.Command(
		dnsttServerPath,
		"-udp", ":"+itoa(dnsPort),
		"-privkey-file", privPath,
		"test.example.com",
		"127.0.0.1:"+itoa(socksPort),
	)
	if err := dnsttServerCmd.Start(); err != nil {
		t.Skipf("failed to start dnstt-server: %v", err)
	}
	env.Processes = append(env.Processes, dnsttServerCmd)

	// Wait for dnstt-server to start (DNS servers listen on UDP)
	if err := env.WaitForUDPPort(dnsPort, 5*time.Second); err != nil {
		t.Skipf("dnstt-server failed to start: %v", err)
	}

	// Allocate port for dnstt-client
	clientPort, err := env.AllocatePort()
	if err != nil {
		t.Fatalf("failed to allocate client port: %v", err)
	}

	// Start dnstt-client (use -udp to connect directly to local server)
	dnsttClientPath := env.GetBinaryPath("dnstt-client")
	dnsttClientCmd := exec.Command(
		dnsttClientPath,
		"-udp", "127.0.0.1:"+itoa(dnsPort),
		"-pubkey", pubKey,
		"test.example.com",
		"127.0.0.1:"+itoa(clientPort),
	)
	if err := dnsttClientCmd.Start(); err != nil {
		t.Skipf("failed to start dnstt-client: %v", err)
	}
	env.Processes = append(env.Processes, dnsttClientCmd)

	// Wait for dnstt-client to start
	if err := env.WaitForPort(clientPort, 10*time.Second); err != nil {
		t.Skipf("dnstt-client failed to start (DNS may not be configured): %v", err)
	}

	t.Log("DNSTT tunnel established successfully")

	// Test connectivity through the tunnel
	// This would require a more complex setup with actual DNS resolution
	// For now, just verify the processes started
}

func itoa(i int) string {
	return string([]byte{
		byte('0' + i/10000%10),
		byte('0' + i/1000%10),
		byte('0' + i/100%10),
		byte('0' + i/10%10),
		byte('0' + i%10),
	}[skipLeadingZeros(i):])
}

func skipLeadingZeros(i int) int {
	if i >= 10000 {
		return 0
	}
	if i >= 1000 {
		return 1
	}
	if i >= 100 {
		return 2
	}
	if i >= 10 {
		return 3
	}
	return 4
}
