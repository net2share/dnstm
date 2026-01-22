package system

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type OSInfo struct {
	ID             string
	PrettyName     string
	PackageManager string
}

type ArchInfo struct {
	Arch      string
	DnsttArch string
}

func DetectOS() (*OSInfo, error) {
	info := &OSInfo{}

	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			info.ID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		}
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			info.PrettyName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}

	info.PackageManager = GetPackageManager()

	return info, nil
}

func DetectArch() *ArchInfo {
	arch := runtime.GOARCH
	dnsttArch := arch

	switch arch {
	case "amd64":
		dnsttArch = "linux-amd64"
	case "arm64":
		dnsttArch = "linux-arm64"
	case "arm":
		dnsttArch = "linux-armv7"
	case "386":
		dnsttArch = "linux-386"
	}

	return &ArchInfo{
		Arch:      arch,
		DnsttArch: dnsttArch,
	}
}

func GetPackageManager() string {
	managers := []struct {
		cmd  string
		name string
	}{
		{"apt", "apt"},
		{"dnf", "dnf"},
		{"yum", "yum"},
	}

	for _, m := range managers {
		if _, err := exec.LookPath(m.cmd); err == nil {
			return m.name
		}
	}

	return ""
}

func IsRoot() bool {
	return os.Geteuid() == 0
}

func HasSystemd() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func HasIPv6() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() == nil && ipnet.IP.To16() != nil {
					return true
				}
			}
		}
	}

	return false
}

func DetectSSHPort() string {
	file, err := os.Open("/etc/ssh/sshd_config")
	if err != nil {
		return "22"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.ToLower(fields[0]) == "port" {
			return fields[1]
		}
	}

	return "22"
}

func GetDefaultInterface() (string, error) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan() // skip header

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == "00000000" {
			return fields[0], nil
		}
	}

	return "", scanner.Err()
}
