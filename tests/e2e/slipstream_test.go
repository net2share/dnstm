package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/net2share/dnstm/internal/certs"
)

func TestSlipstream_LocalMode(t *testing.T) {
	env := NewE2EEnv(t)

	// Generate certificate
	certsDir := filepath.Join(env.ConfigDir, "certs")
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		t.Fatalf("failed to create certs dir: %v", err)
	}

	certPath := filepath.Join(certsDir, "test_cert.pem")
	keyPath := filepath.Join(certsDir, "test_key.pem")

	fingerprint, err := certs.GenerateCertificate(certPath, keyPath, "test.example.com")
	if err != nil {
		t.Fatalf("failed to generate certificate: %v", err)
	}

	t.Logf("Generated certificate with fingerprint: %s", fingerprint)

	// Allocate ports
	socksPort, err := env.AllocatePort()
	if err != nil {
		t.Fatalf("failed to allocate SOCKS port: %v", err)
	}

	serverPort, err := env.AllocatePort()
	if err != nil {
		t.Fatalf("failed to allocate server port: %v", err)
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

	// Start slipstream-server
	slipstreamServerPath := env.GetBinaryPath("slipstream-server")
	slipstreamServerCmd := exec.Command(
		slipstreamServerPath,
		"--dns-listen-host", "127.0.0.1",
		"--dns-listen-port", itoa(serverPort),
		"--cert", certPath,
		"--key", keyPath,
		"--target-address", "127.0.0.1:"+itoa(socksPort),
		"--domain", "test.example.com",
	)
	if err := slipstreamServerCmd.Start(); err != nil {
		t.Skipf("failed to start slipstream-server: %v", err)
	}
	env.Processes = append(env.Processes, slipstreamServerCmd)

	// Wait for slipstream-server to start (DNS servers listen on UDP)
	if err := env.WaitForUDPPort(serverPort, 5*time.Second); err != nil {
		t.Skipf("slipstream-server failed to start: %v", err)
	}

	// Allocate port for slipstream-client
	clientPort, err := env.AllocatePort()
	if err != nil {
		t.Fatalf("failed to allocate client port: %v", err)
	}

	// Start slipstream-client (use --authoritative for direct local connection)
	slipstreamClientPath := env.GetBinaryPath("slipstream-client")
	slipstreamClientCmd := exec.Command(
		slipstreamClientPath,
		"--tcp-listen-host", "127.0.0.1",
		"--tcp-listen-port", itoa(clientPort),
		"--authoritative", "127.0.0.1:"+itoa(serverPort),
		"--domain", "test.example.com",
		"--cert", certPath,
	)
	if err := slipstreamClientCmd.Start(); err != nil {
		t.Skipf("failed to start slipstream-client: %v", err)
	}
	env.Processes = append(env.Processes, slipstreamClientCmd)

	// Wait for slipstream-client to start (listens on TCP)
	if err := env.WaitForPort(clientPort, 5*time.Second); err != nil {
		t.Fatalf("slipstream-client failed to start: %v", err)
	}

	t.Log("Slipstream tunnel established successfully")

	// Test connectivity through the tunnel
	err = env.TestSOCKSProxy("127.0.0.1:"+itoa(clientPort), "https://httpbin.org/ip")
	if err != nil {
		t.Errorf("SOCKS proxy test failed: %v", err)
	} else {
		t.Log("SOCKS proxy test passed")
	}
}

func TestSlipstream_WithShadowsocks(t *testing.T) {
	env := NewE2EEnv(t)

	// Generate certificate
	certsDir := filepath.Join(env.ConfigDir, "certs")
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		t.Fatalf("failed to create certs dir: %v", err)
	}

	certPath := filepath.Join(certsDir, "test_cert.pem")
	keyPath := filepath.Join(certsDir, "test_key.pem")

	_, err := certs.GenerateCertificate(certPath, keyPath, "test.example.com")
	if err != nil {
		t.Fatalf("failed to generate certificate: %v", err)
	}

	// Allocate ports
	ssServerPort, err := env.AllocatePort()
	if err != nil {
		t.Fatalf("failed to allocate SS server port: %v", err)
	}

	ssLocalPort, err := env.AllocatePort()
	if err != nil {
		t.Fatalf("failed to allocate SS local port: %v", err)
	}

	slipServerPath := env.GetBinaryPath("slipstream-server")
	slipClientPath := env.GetBinaryPath("slipstream-client")

	// Create ssserver config
	ssPassword := "test-password-123"
	ssMethod := "aes-256-gcm"

	ssServerConfig := env.WriteFile("ss-server.json", `{
  "server": "127.0.0.1",
  "server_port": `+itoa(ssServerPort)+`,
  "password": "`+ssPassword+`",
  "method": "`+ssMethod+`",
  "plugin": "`+slipServerPath+`",
  "plugin_opts": "cert=`+certPath+`;key=`+keyPath+`;domain=test.example.com"
}`)

	ssLocalConfig := env.WriteFile("ss-local.json", `{
  "server": "127.0.0.1",
  "server_port": `+itoa(ssServerPort)+`,
  "local_address": "127.0.0.1",
  "local_port": `+itoa(ssLocalPort)+`,
  "password": "`+ssPassword+`",
  "method": "`+ssMethod+`",
  "plugin": "`+slipClientPath+`",
  "plugin_opts": "authoritative=127.0.0.1:`+itoa(ssServerPort)+`;domain=test.example.com;cert=`+certPath+`"
}`)

	// Start ssserver
	ssServerPath := env.GetBinaryPath("ssserver")
	ssServerCmd := exec.Command(ssServerPath, "-c", ssServerConfig)
	if err := ssServerCmd.Start(); err != nil {
		t.Skipf("failed to start ssserver: %v", err)
	}
	env.Processes = append(env.Processes, ssServerCmd)

	// Wait for ssserver to start (slipstream plugin listens on UDP)
	if err := env.WaitForUDPPort(ssServerPort, 5*time.Second); err != nil {
		t.Skipf("ssserver failed to start: %v", err)
	}

	// Start sslocal
	ssLocalPath := env.GetBinaryPath("sslocal")
	ssLocalCmd := exec.Command(ssLocalPath, "-c", ssLocalConfig)
	if err := ssLocalCmd.Start(); err != nil {
		t.Skipf("failed to start sslocal: %v", err)
	}
	env.Processes = append(env.Processes, ssLocalCmd)

	// Wait for sslocal to start
	if err := env.WaitForPort(ssLocalPort, 10*time.Second); err != nil {
		t.Fatalf("sslocal failed to start: %v", err)
	}

	t.Log("Slipstream + Shadowsocks tunnel established successfully")

	// Test connectivity
	err = env.TestSOCKSProxy("127.0.0.1:"+itoa(ssLocalPort), "https://httpbin.org/ip")
	if err != nil {
		t.Errorf("SOCKS proxy test failed: %v", err)
	} else {
		t.Log("SOCKS proxy test passed")
	}
}
