package network

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Legacy port constants used for cleaning up old firewall rules.
const (
	legacyDnsttPort       = "5300"
	legacySlipstreamPort  = "5301"
	legacyShadowsocksPort = "5302"
)

type FirewallType int

const (
	FirewallNone FirewallType = iota
	FirewallFirewalld
	FirewallUFW
	FirewallIptables
)

func DetectFirewall() FirewallType {
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		cmd := exec.Command("systemctl", "is-active", "firewalld")
		if err := cmd.Run(); err == nil {
			return FirewallFirewalld
		}
	}

	if _, err := exec.LookPath("ufw"); err == nil {
		cmd := exec.Command("ufw", "status")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "active") {
			return FirewallUFW
		}
	}

	if _, err := exec.LookPath("iptables"); err == nil {
		return FirewallIptables
	}

	return FirewallNone
}

// ConfigureFirewallForPort configures the firewall to redirect port 53 to the given port.
func ConfigureFirewallForPort(port string) error {
	fwType := DetectFirewall()

	switch fwType {
	case FirewallFirewalld:
		return configureFirewalldForPort(port)
	case FirewallUFW:
		return configureUFWForPort(port)
	case FirewallIptables, FirewallNone:
		return configureIptablesForPort(port)
	}

	return nil
}

func configureFirewalldForPort(port string) error {
	cmds := [][]string{
		{"firewall-cmd", "--permanent", "--add-port=53/udp"},
		{"firewall-cmd", "--permanent", "--add-port=53/tcp"},
		{"firewall-cmd", "--permanent", "--add-port=" + port + "/udp"},
		{"firewall-cmd", "--permanent", "--add-port=" + port + "/tcp"},
		{"firewall-cmd", "--permanent", "--add-masquerade"},
		{"firewall-cmd", "--permanent", "--direct", "--add-rule", "ipv4", "nat", "PREROUTING", "0", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port},
		{"firewall-cmd", "--reload"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("firewalld command failed: %s: %w", string(output), err)
		}
	}

	return nil
}

func configureUFWForPort(port string) error {
	// Enable route_localnet to allow DNAT to 127.0.0.1
	enableRouteLocalnet()

	// Allow port 53 for external DNS queries
	// Allow the target port because after NAT PREROUTING redirects 53->port,
	// packets arrive at INPUT chain with dport port
	cmds := [][]string{
		{"ufw", "allow", "53/udp"},
		{"ufw", "allow", "53/tcp"},
		{"ufw", "allow", port + "/udp"},
		{"ufw", "allow", port + "/tcp"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Run()
	}

	// Clear existing NAT PREROUTING rules first to avoid duplicates
	clearAllNatPrerouting()

	// Add NAT rules to /etc/ufw/before.rules for persistence
	if err := addUFWNatRulesForPort(port); err != nil {
		// Fall back to direct iptables if UFW config fails
		return configureIptablesForPort(port)
	}

	// Reload UFW to apply the NAT rules from before.rules
	exec.Command("ufw", "reload").Run()

	return nil
}

const (
	ufwBeforeRulesPath  = "/etc/ufw/before.rules"
	ufwBefore6RulesPath = "/etc/ufw/before6.rules"
	dnstmNatMarker      = "# NAT table rules for dnstm"
	dnsttNatMarker      = "# NAT table rules for dnstt" // Legacy marker for backward compat
)

// addUFWNatRulesToFile is the shared implementation for adding NAT rules to UFW rules files.
func addUFWNatRulesToFile(filePath, targetAddr, port, comment string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Check if NAT rules already exist (check both old and new markers)
	if strings.Contains(string(content), dnstmNatMarker) || strings.Contains(string(content), dnsttNatMarker) {
		removeUFWNatRules(filePath)
		content, _ = os.ReadFile(filePath)
	}

	natRules := fmt.Sprintf(`%s - DNAT port 53 to %s:%s%s
*nat
:PREROUTING ACCEPT [0:0]
-A PREROUTING -p udp --dport 53 -j DNAT --to-destination %s:%s
-A PREROUTING -p tcp --dport 53 -j DNAT --to-destination %s:%s
COMMIT

`, dnstmNatMarker, targetAddr, port, comment, targetAddr, port, targetAddr, port)

	newContent := natRules + string(content)

	return os.WriteFile(filePath, []byte(newContent), 0640)
}

func addUFWNatRulesForPort(port string) error {
	enableRouteLocalnet()
	return addUFWNatRulesToFile(ufwBeforeRulesPath, "127.0.0.1", port, "")
}

func addUFWNatRulesIPv6ForPort(port string) error {
	return addUFWNatRulesToFile(ufwBefore6RulesPath, "[::1]", port, " (IPv6)")
}

func configureIptablesForPort(port string) error {
	// Enable route_localnet to allow DNAT to 127.0.0.1
	enableRouteLocalnet()

	// Clear any existing NAT rules first to avoid duplicates
	clearAllNatPrerouting()

	rules := [][]string{
		{"-t", "nat", "-A", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", "127.0.0.1:" + port},
		{"-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", "53", "-j", "DNAT", "--to-destination", "127.0.0.1:" + port},
	}

	for _, args := range rules {
		cmd := exec.Command("iptables", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("iptables command failed: %s: %w", string(output), err)
		}
	}

	return saveIptablesRules()
}

// enableRouteLocalnet enables the route_localnet sysctl setting
// which is required for DNAT to 127.0.0.1 to work.
func enableRouteLocalnet() {
	// Enable for all interfaces
	exec.Command("sysctl", "-w", "net.ipv4.conf.all.route_localnet=1").Run()
	// Also try to enable for common interface names
	for _, iface := range []string{"eth0", "enp1s0", "ens3", "ens192"} {
		exec.Command("sysctl", "-w", fmt.Sprintf("net.ipv4.conf.%s.route_localnet=1", iface)).Run()
	}
}

// clearAllNatPrerouting clears all NAT PREROUTING rules.
func clearAllNatPrerouting() {
	exec.Command("iptables", "-t", "nat", "-F", "PREROUTING").Run()
}

// clearAllNatOutput clears all NAT OUTPUT rules.
// This is needed because some legacy setups may have OUTPUT rules redirecting DNS.
func clearAllNatOutput() {
	exec.Command("iptables", "-t", "nat", "-F", "OUTPUT").Run()
	exec.Command("ip6tables", "-t", "nat", "-F", "OUTPUT").Run()
}

func clearIptablesRulesForPort(port string) {
	// Try to delete both DNAT and REDIRECT rules (for backward compatibility)
	rules := [][]string{
		{"-t", "nat", "-D", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", "127.0.0.1:" + port},
		{"-t", "nat", "-D", "PREROUTING", "-p", "tcp", "--dport", "53", "-j", "DNAT", "--to-destination", "127.0.0.1:" + port},
		{"-t", "nat", "-D", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port},
		{"-t", "nat", "-D", "PREROUTING", "-p", "tcp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port},
	}

	for _, args := range rules {
		exec.Command("iptables", args...).Run()
	}
}

func saveIptablesRules() error {
	persistPaths := []string{
		"/etc/iptables/rules.v4",
		"/etc/sysconfig/iptables",
	}

	for _, path := range persistPaths {
		dir := path[:strings.LastIndex(path, "/")]
		if _, err := os.Stat(dir); err == nil {
			cmd := exec.Command("iptables-save")
			output, err := cmd.Output()
			if err != nil {
				continue
			}
			if err := os.WriteFile(path, output, 0600); err == nil {
				return nil
			}
		}
	}

	if _, err := exec.LookPath("netfilter-persistent"); err == nil {
		exec.Command("netfilter-persistent", "save").Run()
	}

	return nil
}

// ConfigureIPv6ForPort configures IPv6 firewall rules for the given port.
func ConfigureIPv6ForPort(port string) error {
	fwType := DetectFirewall()

	if fwType == FirewallUFW {
		// Just update the before6.rules file, don't reload
		// The IPv4 config already did the reload
		return addUFWNatRulesIPv6ForPort(port)
	}

	// Direct ip6tables for non-UFW systems
	// Clear any existing rules first
	exec.Command("ip6tables", "-t", "nat", "-F", "PREROUTING").Run()

	rules := [][]string{
		{"-t", "nat", "-A", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", "[::1]:" + port},
		{"-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", "53", "-j", "DNAT", "--to-destination", "[::1]:" + port},
	}

	for _, args := range rules {
		exec.Command("ip6tables", args...).Run()
	}

	return nil
}

// RemoveFirewallRulesForPort removes firewall rules for a specific port.
func RemoveFirewallRulesForPort(port string) {
	fwType := DetectFirewall()

	switch fwType {
	case FirewallFirewalld:
		removeFirewalldRulesForPort(port)
	case FirewallUFW:
		removeUFWRulesForPort(port)
	case FirewallIptables, FirewallNone:
		clearIptablesRulesForPort(port)
		clearIp6tablesRulesForPort(port)
		saveIptablesRules()
	}
}

// RemoveAllFirewallRules removes firewall rules for all legacy ports.
func RemoveAllFirewallRules() {
	legacyPorts := []string{legacyDnsttPort, legacySlipstreamPort, legacyShadowsocksPort}
	fwType := DetectFirewall()

	switch fwType {
	case FirewallFirewalld:
		for _, port := range legacyPorts {
			removeFirewalldRulesForPort(port)
		}
	case FirewallUFW:
		for _, port := range legacyPorts {
			removeUFWRulesForPort(port)
		}
	case FirewallIptables, FirewallNone:
		for _, port := range legacyPorts {
			clearIptablesRulesForPort(port)
			clearIp6tablesRulesForPort(port)
		}
		saveIptablesRules()
	}
}

func removeFirewalldRulesForPort(port string) {
	cmds := [][]string{
		{"firewall-cmd", "--permanent", "--remove-port=53/udp"},
		{"firewall-cmd", "--permanent", "--remove-port=53/tcp"},
		{"firewall-cmd", "--permanent", "--remove-port=" + port + "/udp"},
		{"firewall-cmd", "--permanent", "--remove-port=" + port + "/tcp"},
		{"firewall-cmd", "--permanent", "--direct", "--remove-rule", "ipv4", "nat", "PREROUTING", "0", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port},
		{"firewall-cmd", "--reload"},
	}

	for _, args := range cmds {
		exec.Command(args[0], args[1:]...).Run()
	}
}

func removeUFWRulesForPort(port string) {
	// Remove port rules
	cmds := [][]string{
		{"ufw", "delete", "allow", "53/udp"},
		{"ufw", "delete", "allow", "53/tcp"},
		{"ufw", "delete", "allow", port + "/udp"},
		{"ufw", "delete", "allow", port + "/tcp"},
	}

	for _, args := range cmds {
		exec.Command(args[0], args[1:]...).Run()
	}

	// Remove NAT rules from before.rules
	removeUFWNatRules(ufwBeforeRulesPath)
	removeUFWNatRules(ufwBefore6RulesPath)

	exec.Command("ufw", "reload").Run()
}

func removeUFWNatRules(filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	contentStr := string(content)
	// Check for both old and new markers
	if !strings.Contains(contentStr, dnstmNatMarker) && !strings.Contains(contentStr, dnsttNatMarker) {
		return
	}

	// Remove the NAT block we added
	lines := strings.Split(contentStr, "\n")
	var newLines []string
	inNatBlock := false
	skipEmptyLine := false

	for _, line := range lines {
		if strings.Contains(line, dnstmNatMarker) || strings.Contains(line, dnsttNatMarker) {
			inNatBlock = true
			continue
		}
		if inNatBlock {
			if line == "COMMIT" {
				inNatBlock = false
				skipEmptyLine = true
				continue
			}
			if strings.HasPrefix(line, "*nat") ||
				strings.HasPrefix(line, ":PREROUTING") ||
				strings.HasPrefix(line, "-A PREROUTING") {
				continue
			}
		}
		// Skip one empty line after COMMIT
		if skipEmptyLine && line == "" {
			skipEmptyLine = false
			continue
		}
		newLines = append(newLines, line)
	}

	os.WriteFile(filePath, []byte(strings.Join(newLines, "\n")), 0640)
}

func clearIp6tablesRulesForPort(port string) {
	rules := [][]string{
		{"-t", "nat", "-D", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port},
		{"-t", "nat", "-D", "PREROUTING", "-p", "tcp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port},
	}

	for _, args := range rules {
		exec.Command("ip6tables", args...).Run()
	}
}

// SwitchDNSRouting switches the DNS routing from one port to another.
// This is used when switching between providers.
func SwitchDNSRouting(fromPort, toPort string) error {
	// First, remove rules for the old port
	RemoveFirewallRulesForPort(fromPort)

	// Then, configure rules for the new port
	if err := ConfigureFirewallForPort(toPort); err != nil {
		return err
	}

	// Configure IPv6 if available
	ConfigureIPv6ForPort(toPort)

	return nil
}

// AllowPort53 ensures port 53 is open in the firewall without setting up NAT.
// This is used in multi-mode where the DNS router listens directly on port 53.
func AllowPort53() error {
	fwType := DetectFirewall()

	switch fwType {
	case FirewallFirewalld:
		cmds := [][]string{
			{"firewall-cmd", "--permanent", "--add-port=53/udp"},
			{"firewall-cmd", "--permanent", "--add-port=53/tcp"},
			{"firewall-cmd", "--reload"},
		}
		for _, args := range cmds {
			exec.Command(args[0], args[1:]...).Run()
		}
	case FirewallUFW:
		cmds := [][]string{
			{"ufw", "allow", "53/udp"},
			{"ufw", "allow", "53/tcp"},
		}
		for _, args := range cmds {
			exec.Command(args[0], args[1:]...).Run()
		}
	case FirewallIptables, FirewallNone:
		// For iptables-only systems, ensure the input chain allows port 53
		cmds := [][]string{
			{"-A", "INPUT", "-p", "udp", "--dport", "53", "-j", "ACCEPT"},
			{"-A", "INPUT", "-p", "tcp", "--dport", "53", "-j", "ACCEPT"},
		}
		for _, args := range cmds {
			exec.Command("iptables", args...).Run()
		}
	}

	return nil
}

// ClearNATOnly removes NAT rules without removing UFW allow rules.
// This is used when switching to multi-mode where we want to keep port 53 open
// but remove the DNAT redirect. Also clears OUTPUT NAT rules that may interfere
// with the server's own DNS resolution.
func ClearNATOnly() {
	fwType := DetectFirewall()

	switch fwType {
	case FirewallUFW:
		// Remove NAT rules from before.rules but keep UFW allow rules
		removeUFWNatRules(ufwBeforeRulesPath)
		removeUFWNatRules(ufwBefore6RulesPath)
		// Clear iptables NAT rules (PREROUTING and OUTPUT)
		clearAllNatPrerouting()
		clearAllNatOutput()
		exec.Command("ip6tables", "-t", "nat", "-F", "PREROUTING").Run()
		exec.Command("ufw", "reload").Run()
	case FirewallIptables, FirewallNone:
		clearAllNatPrerouting()
		clearAllNatOutput()
		exec.Command("ip6tables", "-t", "nat", "-F", "PREROUTING").Run()
	case FirewallFirewalld:
		// For firewalld, remove the direct rules for all legacy ports
		for _, port := range []string{legacyDnsttPort, legacySlipstreamPort, legacyShadowsocksPort} {
			exec.Command("firewall-cmd", "--permanent", "--direct", "--remove-rule", "ipv4", "nat", "PREROUTING", "0", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port).Run()
		}
		exec.Command("firewall-cmd", "--reload").Run()
	}
}

// GetExternalIP returns the external (non-loopback, non-private) IP address.
// Falls back to the first non-loopback IP if no external IP is found.
func GetExternalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to get interfaces: %w", err)
	}

	var fallbackIP string

	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip loopback and IPv6 for now
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			// Check if it's a private IP
			if isPrivateIP(ip) {
				// Use as fallback if we don't find an external IP
				if fallbackIP == "" {
					fallbackIP = ip.String()
				}
				continue
			}

			// Found an external IP
			return ip.String(), nil
		}
	}

	// If no external IP found, use the fallback (first non-loopback IP)
	if fallbackIP != "" {
		return fallbackIP, nil
	}

	return "", fmt.Errorf("no suitable IP address found")
}

// isPrivateIP checks if an IP is in a private range.
func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16", // Link-local
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// IsUDPPortAvailable checks if a UDP port is available for binding on the external IP.
func IsUDPPortAvailable(port int) bool {
	// Try to bind on external IP first (for single mode)
	externalIP, err := GetExternalIP()
	if err == nil {
		addr := fmt.Sprintf("%s:%d", externalIP, port)
		conn, err := net.ListenPacket("udp", addr)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}

	// Fall back to checking 0.0.0.0
	addr := fmt.Sprintf(":%d", port)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// WaitForPortAvailable waits for a UDP port to become available.
// Returns true if port becomes available within timeout, false otherwise.
func WaitForPortAvailable(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if IsUDPPortAvailable(port) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// KillProcessOnPort kills any process using the specified port.
// Returns nil if the port becomes available after killing, error otherwise.
func KillProcessOnPort(port int) error {
	// Use fuser to kill processes on the port
	exec.Command("fuser", "-k", fmt.Sprintf("%d/udp", port)).Run()
	exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", port)).Run()

	// Wait for processes to terminate
	time.Sleep(500 * time.Millisecond)

	// Check if port is now available
	if !IsUDPPortAvailable(port) {
		return fmt.Errorf("port %d still in use after killing processes", port)
	}
	return nil
}
