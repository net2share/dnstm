package mtproxy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"github.com/net2share/dnstm/internal/download"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

const (
	MTProxyBinaryName      = "mtproto-proxy"
	MTProxyServiceName     = "mtproxy"
	MTProxyBridgeService   = "mtproxy-bridge"
	MTProxyBindAddr        = "127.0.0.1"
	MTProxyPort            = "8443"
	MTProxyBridgePort      = "8444" // socat bridge listens here, forwards to MTProxy
	MTProxyStatsPort       = "8888"
	MTProxyInstallationDir = "/usr/local/bin"
	MTProxyConfigDir       = "/etc/mtproxy"

	MTProxyRepo    = "GetPageSpeed/MTProxy" // community fork of Telegram/MTProxy
	ProxySecretURL = "https://core.telegram.org/getProxySecret"
	ProxyConfigURL = "https://core.telegram.org/getProxyConfig"
)

func GenerateSecret() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
func IsMTProxyInstalled() bool {
	_, err := os.Stat(filepath.Join(MTProxyInstallationDir, MTProxyBinaryName))
	return err == nil
}

func InstallMTProxy(progressFn func(downloaded, total int64)) error {
	binaryPath := filepath.Join(MTProxyInstallationDir, MTProxyBinaryName)

	// Check if binary exists and build if needed
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		tmpDir, err := os.MkdirTemp("", "mtproxy-build-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		tmpFile := filepath.Join(tmpDir, "objs/bin/mtproto-proxy")

		// there is no release in neither official nor community repo, so we build from source
		if err := buildFromSource(progressFn, tmpDir); err != nil {
			return fmt.Errorf("failed to build MTProxy from source: %w", err)
		}

		if err := os.Chmod(tmpFile, 0755); err != nil {
			return fmt.Errorf("failed to set executable permission: %w", err)
		}

		if err := download.CopyFile(tmpFile, binaryPath); err != nil {
			return fmt.Errorf("failed to install MTProxy binary: %w", err)
		}
		tui.PrintSuccess("MTProxy binary installed")
	} else {
		tui.PrintStatus("MTProxy binary already exists")
	}

	// Always ensure config directory and files exist
	if err := os.MkdirAll(MTProxyConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create MTProxy config directory: %w", err)
	}

	configPath := filepath.Join(MTProxyConfigDir, "proxy-multi.conf")
	secretPath := filepath.Join(MTProxyConfigDir, "proxy-secret")

	// Download config files if they don't exist
	if _, err := os.Stat(secretPath); os.IsNotExist(err) {
		tui.PrintStatus("Downloading proxy-secret...")
		if err := download.DownloadFile(ProxySecretURL, secretPath, progressFn); err != nil {
			return fmt.Errorf("failed to download proxy secret: %w", err)
		}
		if err := os.Chmod(secretPath, 0644); err != nil {
			return fmt.Errorf("failed to set permissions on proxy-secret: %w", err)
		}
		tui.PrintSuccess("Downloaded proxy-secret")
	} else {
		tui.PrintStatus("proxy-secret already exists")
		if err := os.Chmod(secretPath, 0644); err != nil {
			return fmt.Errorf("failed to set permissions on proxy-secret: %w", err)
		}
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		tui.PrintStatus("Downloading proxy-multi.conf...")
		if err := download.DownloadFile(ProxyConfigURL, configPath, progressFn); err != nil {
			return fmt.Errorf("failed to download proxy-multi.conf: %w", err)
		}
		if err := os.Chmod(configPath, 0644); err != nil {
			return fmt.Errorf("failed to set permissions on proxy-multi.conf: %w", err)
		}
		tui.PrintSuccess("Downloaded proxy-multi.conf")
	} else {
		tui.PrintStatus("proxy-multi.conf already exists")
		if err := os.Chmod(configPath, 0644); err != nil {
			return fmt.Errorf("failed to set permissions on proxy-multi.conf: %w", err)
		}
	}

	return nil
}

// healthCheckPorts attempts TCP connections to verify ports are listening
func healthCheckPorts(ports []string) error {
	for _, port := range ports {
		address := fmt.Sprintf("127.0.0.1:%s", port)

		conn, err := net.DialTimeout("tcp", address, 3*time.Second)
		if err != nil {
			return fmt.Errorf("port %s is not open: %w", port, err)
		}

		conn.Close()
		tui.PrintSuccess(fmt.Sprintf("Port %s is open and accepting connections", port))
	}
	return nil
}

func ConfigureMTProxy(secret string) error {
	configPath := filepath.Join(MTProxyConfigDir, "proxy-multi.conf")
	binaryPath := filepath.Join(MTProxyInstallationDir, MTProxyBinaryName)

	if err := registerConfigCronJob(configPath); err != nil {
		tui.PrintWarning("Failed to register config update cron job: " + err.Error())
	}

	if err := service.CreateGenericService(&service.ServiceConfig{
		Name:        MTProxyServiceName,
		Description: "Telegram MTProxy Server",
		User:        "nobody",
		Group:       "nogroup",
		ExecStart: fmt.Sprintf("%s -u nobody -p %s -H %s -S %s --aes-pwd %s %s",
			binaryPath,
			MTProxyStatsPort,
			MTProxyPort,
			secret,
			path.Join(MTProxyConfigDir, "proxy-secret"),
			path.Join(MTProxyConfigDir, "proxy-multi.conf")),
		BindToPrivileged: true,
	}); err != nil {
		return fmt.Errorf("failed to create systemd service: %w", err)
	}

	tui.PrintStatus("Starting MTProxy service...")
	if err := service.EnableService(MTProxyServiceName); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	if err := service.StartService(MTProxyServiceName); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	time.Sleep(2 * time.Second)

	if !service.IsServiceActive(MTProxyServiceName) {
		tui.PrintWarning("MTProxy service is not active, checking logs...")
		if logs, err := service.GetServiceLogs(MTProxyServiceName, 20); err == nil {
			fmt.Println("\nRecent logs:")
			fmt.Println(logs)
		}
		return fmt.Errorf("MTProxy service failed to start")
	}

	tui.PrintSuccess("MTProxy service started successfully")

	tui.PrintStatus("Verifying MTProxy is listening on port " + MTProxyPort + "...")
	if err := healthCheckPorts([]string{MTProxyPort}); err != nil {
		tui.PrintWarning("Port check failed: " + err.Error())
	}

	return nil
}

func FormatProxyURL(secret, nsSubdomain string) string {
	return fmt.Sprintf("https://t.me/proxy?server=%s&port=%s&secret=dd%s", nsSubdomain, MTProxyPort, secret)
}

func buildFromSource(progressFn func(downloaded, total int64), tmpDir string) error {
	if err := installBuildDeps(); err != nil {
		return fmt.Errorf("failed to install build dependencies: %w", err)
	}

	tui.PrintStatus("Cloning MTProxy repository...")
	cmd := exec.Command("git", "clone", fmt.Sprintf("https://github.com/%s.git", MTProxyRepo), tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone repository: %w\n%s", err, output)
	}
	tui.PrintSuccess("Repository cloned, building MTProxy...")

	cmd = exec.Command("make")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		// we can run "make clean" but as we remove tmp dir in our Installation process its not required
		return fmt.Errorf("failed to build MTProxy: %w\n%s", err, output)
	}
	tui.PrintSuccess("MTProxy built successfully")
	return nil
}
func installBuildDeps() error {
	tui.PrintStatus("Installing build dependencies...")

	osInfo, err := osdetect.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect Linux distribution: %w", err)
	}

	packageManager := osInfo.PackageManager

	switch packageManager {
	case "apt", "apt-get":
		// Debian/Ubuntu
		tui.PrintStatus("Detected Debian/Ubuntu, installing dependencies...")

		if err := runCommand("apt-get", "update", "-qq"); err != nil {
			tui.PrintWarning(fmt.Sprintf("apt-get update failed: %v", err))
		}

		deps := []string{"install", "-y", "-qq", "git", "curl", "build-essential", "libssl-dev", "zlib1g-dev"}
		if err := runCommand("apt-get", deps...); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}

	case "yum":

		tui.PrintStatus("Detected RHEL/CentOS (yum), installing dependencies...")
		deps := []string{"install", "-y", "-q", "git", "curl", "gcc", "gcc-c++", "make", "openssl-devel", "zlib-devel"}
		if err := runCommand("yum", deps...); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}

	case "dnf":

		tui.PrintStatus("Detected RHEL-based system (dnf), installing dependencies...")
		deps := []string{"install", "-y", "-q", "git", "curl", "gcc", "gcc-c++", "make", "openssl-devel", "zlib-devel"}
		if err := runCommand("dnf", deps...); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}

	case "zypper":

		tui.PrintStatus("Detected openSUSE, installing dependencies...")
		deps := []string{"install", "-y", "git", "curl", "gcc", "gcc-c++", "make", "libopenssl-devel", "zlib-devel"}
		if err := runCommand("zypper", deps...); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}

	default:
		return fmt.Errorf("unsupported package manager: %s (detected from %s)", packageManager, osInfo.PrettyName)
	}

	tui.PrintSuccess("Build dependencies installed")
	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %v failed: %w", name, args, err)
	}
	return nil
}

func registerConfigCronJob(configPath string) error {
	cronContent := fmt.Sprintf(`#!/bin/bash
# MTProxy config update script
curl -s %s -o %s
curl -s %s -o %s
systemctl restart mtproxy
`, ProxySecretURL, path.Join(MTProxyConfigDir, "proxy-secret"), ProxyConfigURL, configPath)

	cronPath := "/etc/cron.daily/mtproxy-update-config"
	if _, err := os.Stat(cronPath); os.IsNotExist(err) {
		if err := os.WriteFile(cronPath, []byte(cronContent), 0755); err != nil {
			return fmt.Errorf("failed to write cron file: %w", err)
		}
		tui.PrintSuccess("MTProxy config update cron job created")
	} else {
		tui.PrintStatus("MTProxy config update cron job already exists")
	}

	osInfo, err := osdetect.Detect()
	if err != nil {
		tui.PrintWarning("Failed to detect OS, skipping cron service reload")
		return nil
	}

	var cronServices []string

	switch osInfo.PackageManager {
	case "apt", "apt-get":
		// Debian based systems use "cron"
		cronServices = []string{"cron"}
	case "yum", "dnf":
		// RHEL/CentOS uses "crond"
		cronServices = []string{"crond"}

	default:
		tui.PrintWarning("Unsupported package manager, cron service reload skipped")
		return nil
	}

	for _, service := range cronServices {
		if err := runCommand("systemctl", "restart", service); err != nil {
			tui.PrintWarning(fmt.Sprintf("Failed to restart %s service: %v", service, err))
		} else {
			tui.PrintSuccess(fmt.Sprintf("%s service restarted successfully", service))
		}
	}

	return nil
}

func IsMTProxyRunning() bool {
	return service.IsServiceActive(MTProxyServiceName)
}

// CheckPort checks if a TCP port is reachable on localhost
func CheckPort(port int) bool {
	address := fmt.Sprintf("localhost:%d", port)
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// StartMTProxy starts the MTProxy service
func StartMTProxy() error {
	return service.StartService(MTProxyServiceName)
}

// StopMTProxy stops the MTProxy service
func StopMTProxy() error {
	return service.StopService(MTProxyServiceName)
}

// RestartMTProxy restarts the MTProxy service
func RestartMTProxy() error {
	return service.RestartService(MTProxyServiceName)
}

func UninstallMTProxy() error {
	// Stop and remove bridge service first
	if err := UninstallBridge(); err != nil {
		fmt.Printf("warning: failed to uninstall bridge: %v", err)
	}

	if service.IsServiceActive(MTProxyServiceName) {
		service.StopService(MTProxyServiceName)
	}
	if service.IsServiceEnabled(MTProxyServiceName) {
		service.DisableService(MTProxyServiceName)
	}
	service.RemoveService(MTProxyServiceName)

	if err := os.Remove(filepath.Join(MTProxyInstallationDir, MTProxyBinaryName)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove MTProxy binary: %w", err)
	}
	if err := os.RemoveAll(MTProxyConfigDir); err != nil {
		return fmt.Errorf("failed to remove config dir: %w", err)
	}
	if err := os.Remove("/etc/cron.daily/mtproxy-update-config"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cron job: %w", err)
	}

	return nil
}

// InstallBridge installs socat and creates a systemd service that bridges TCP to MTProxy.
// This is needed for DNSTT because dnstt-client provides a SOCKS5 interface, not raw TCP.
// The bridge listens on MTProxyBridgePort and forwards raw TCP to MTProxy on MTProxyPort.
func InstallBridge() error {
	// Check if socat is installed
	if _, err := exec.LookPath("socat"); err != nil {
		tui.PrintStatus("Installing socat...")
		osInfo, err := osdetect.Detect()
		if err != nil {
			return fmt.Errorf("failed to detect OS: %w", err)
		}

		var cmd *exec.Cmd
		switch osInfo.PackageManager {
		case "apt", "apt-get":
			cmd = exec.Command("apt-get", "install", "-y", "socat")
		case "dnf":
			cmd = exec.Command("dnf", "install", "-y", "socat")
		default:
			return fmt.Errorf("unsupported package manager: %s", osInfo.PackageManager)
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to install socat: %w\n%s", err, output)
		}
		tui.PrintSuccess("socat installed")
	} else {
		tui.PrintStatus("socat already installed")
	}

	// Create systemd service for the bridge
	serviceContent := fmt.Sprintf(`[Unit]
Description=MTProxy TCP Bridge (socat) for DNSTT
After=network.target mtproxy.service
Requires=mtproxy.service

[Service]
Type=simple
ExecStart=/usr/bin/socat TCP-LISTEN:%s,fork,reuseaddr,bind=%s TCP:%s:%s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`, MTProxyBridgePort, MTProxyBindAddr, MTProxyBindAddr, MTProxyPort)

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", MTProxyBridgeService)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write bridge service file: %w", err)
	}

	// Reload systemd and enable service
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	if err := exec.Command("systemctl", "enable", MTProxyBridgeService).Run(); err != nil {
		return fmt.Errorf("failed to enable bridge service: %w", err)
	}
	if err := exec.Command("systemctl", "start", MTProxyBridgeService).Run(); err != nil {
		return fmt.Errorf("failed to start bridge service: %w", err)
	}

	tui.PrintSuccess(fmt.Sprintf("MTProxy bridge service started (port %s → %s)", MTProxyBridgePort, MTProxyPort))
	return nil
}

// UninstallBridge removes the socat bridge service
func UninstallBridge() error {
	exec.Command("systemctl", "stop", MTProxyBridgeService).Run()
	exec.Command("systemctl", "disable", MTProxyBridgeService).Run()
	os.Remove(fmt.Sprintf("/etc/systemd/system/%s.service", MTProxyBridgeService))
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	tui.PrintStatus("MTProxy bridge service removed")
	return nil
}

// IsBridgeInstalled checks if the bridge service exists
func IsBridgeInstalled() bool {
	_, err := os.Stat(fmt.Sprintf("/etc/systemd/system/%s.service", MTProxyBridgeService))
	return err == nil
}

// IsBridgeRunning checks if the bridge service is active
func IsBridgeRunning() bool {
	return service.IsServiceActive(MTProxyBridgeService)
}

// StartBridge starts the bridge service
func StartBridge() error {
	return service.StartService(MTProxyBridgeService)
}

// StopBridge stops the bridge service
func StopBridge() error {
	return service.StopService(MTProxyBridgeService)
}

// RestartBridge restarts the bridge service
func RestartBridge() error {
	return service.RestartService(MTProxyBridgeService)
}
