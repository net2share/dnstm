package network

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	DnsttPort      = "5300"
	SlipstreamPort = "5301"
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

// ConfigureFirewall configures the firewall for dnstt (backward compatible wrapper).
func ConfigureFirewall() error {
	return ConfigureFirewallForPort(DnsttPort)
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

func configureFirewalld() error {
	return configureFirewalldForPort(DnsttPort)
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

func configureUFW() error {
	return configureUFWForPort(DnsttPort)
}

func configureUFWForPort(port string) error {
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

	// Add NAT rules to /etc/ufw/before.rules for persistence
	if err := addUFWNatRulesForPort(port); err != nil {
		// Fall back to direct iptables if UFW config fails
		return configureIptablesForPort(port)
	}

	// Reload UFW to apply the NAT rules
	exec.Command("ufw", "reload").Run()

	return nil
}

const ufwBeforeRulesPath = "/etc/ufw/before.rules"
const dnstmNatMarker = "# NAT table rules for dnstm"
const dnsttNatMarker = "# NAT table rules for dnstt" // Legacy marker for backward compat

func addUFWNatRules() error {
	return addUFWNatRulesForPort(DnsttPort)
}

func addUFWNatRulesForPort(port string) error {
	content, err := os.ReadFile(ufwBeforeRulesPath)
	if err != nil {
		return err
	}

	// Check if NAT rules already exist (check both old and new markers)
	if strings.Contains(string(content), dnstmNatMarker) || strings.Contains(string(content), dnsttNatMarker) {
		// Remove existing rules first, then add new ones
		removeUFWNatRules(ufwBeforeRulesPath)
		content, _ = os.ReadFile(ufwBeforeRulesPath)
	}

	// NAT rules to prepend before the *filter section
	natRules := fmt.Sprintf(`%s - redirect port 53 to %s
*nat
:PREROUTING ACCEPT [0:0]
-A PREROUTING -p udp --dport 53 -j REDIRECT --to-ports %s
-A PREROUTING -p tcp --dport 53 -j REDIRECT --to-ports %s
COMMIT

`, dnstmNatMarker, port, port, port)

	// Prepend NAT rules to the file
	newContent := natRules + string(content)

	if err := os.WriteFile(ufwBeforeRulesPath, []byte(newContent), 0640); err != nil {
		return err
	}

	return nil
}

func configureIptables() error {
	return configureIptablesForPort(DnsttPort)
}

func configureIptablesForPort(port string) error {
	// Clear any existing rules for both ports
	clearIptablesRulesForPort(DnsttPort)
	clearIptablesRulesForPort(SlipstreamPort)

	rules := [][]string{
		{"-t", "nat", "-A", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port},
		{"-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port},
	}

	for _, args := range rules {
		cmd := exec.Command("iptables", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("iptables command failed: %s: %w", string(output), err)
		}
	}

	return saveIptablesRules()
}

func clearIptablesRules() {
	clearIptablesRulesForPort(DnsttPort)
}

func clearIptablesRulesForPort(port string) {
	rules := [][]string{
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

// ConfigureIPv6 configures IPv6 firewall rules for dnstt (backward compatible wrapper).
func ConfigureIPv6() error {
	return ConfigureIPv6ForPort(DnsttPort)
}

// ConfigureIPv6ForPort configures IPv6 firewall rules for the given port.
func ConfigureIPv6ForPort(port string) error {
	fwType := DetectFirewall()

	if fwType == FirewallUFW {
		return addUFWNatRulesIPv6ForPort(port)
	}

	// Direct ip6tables for non-UFW systems
	// Clear any existing rules first
	clearIp6tablesRulesForPort(DnsttPort)
	clearIp6tablesRulesForPort(SlipstreamPort)

	rules := [][]string{
		{"-t", "nat", "-A", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port},
		{"-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", "53", "-j", "REDIRECT", "--to-ports", port},
	}

	for _, args := range rules {
		exec.Command("ip6tables", args...).Run()
	}

	return nil
}

const ufwBefore6RulesPath = "/etc/ufw/before6.rules"

func addUFWNatRulesIPv6() error {
	return addUFWNatRulesIPv6ForPort(DnsttPort)
}

func addUFWNatRulesIPv6ForPort(port string) error {
	content, err := os.ReadFile(ufwBefore6RulesPath)
	if err != nil {
		return err
	}

	// Check if NAT rules already exist (check both old and new markers)
	if strings.Contains(string(content), dnstmNatMarker) || strings.Contains(string(content), dnsttNatMarker) {
		// Remove existing rules first, then add new ones
		removeUFWNatRules(ufwBefore6RulesPath)
		content, _ = os.ReadFile(ufwBefore6RulesPath)
	}

	// NAT rules to prepend before the *filter section
	natRules := fmt.Sprintf(`%s - redirect port 53 to %s (IPv6)
*nat
:PREROUTING ACCEPT [0:0]
-A PREROUTING -p udp --dport 53 -j REDIRECT --to-ports %s
-A PREROUTING -p tcp --dport 53 -j REDIRECT --to-ports %s
COMMIT

`, dnstmNatMarker, port, port, port)

	// Prepend NAT rules to the file
	newContent := natRules + string(content)

	if err := os.WriteFile(ufwBefore6RulesPath, []byte(newContent), 0640); err != nil {
		return err
	}

	// Reload UFW to apply
	exec.Command("ufw", "reload").Run()

	return nil
}

// RemoveFirewallRules removes all firewall rules for dnstt (backward compatible wrapper).
func RemoveFirewallRules() {
	RemoveFirewallRulesForPort(DnsttPort)
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

// RemoveAllFirewallRules removes firewall rules for all providers.
func RemoveAllFirewallRules() {
	fwType := DetectFirewall()

	switch fwType {
	case FirewallFirewalld:
		removeFirewalldRulesForPort(DnsttPort)
		removeFirewalldRulesForPort(SlipstreamPort)
	case FirewallUFW:
		removeUFWRulesForPort(DnsttPort)
		removeUFWRulesForPort(SlipstreamPort)
	case FirewallIptables, FirewallNone:
		clearIptablesRulesForPort(DnsttPort)
		clearIptablesRulesForPort(SlipstreamPort)
		clearIp6tablesRulesForPort(DnsttPort)
		clearIp6tablesRulesForPort(SlipstreamPort)
		saveIptablesRules()
	}
}

func removeFirewalldRules() {
	removeFirewalldRulesForPort(DnsttPort)
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

func removeUFWRules() {
	removeUFWRulesForPort(DnsttPort)
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

	for _, line := range lines {
		if strings.Contains(line, dnstmNatMarker) || strings.Contains(line, dnsttNatMarker) {
			inNatBlock = true
			continue
		}
		if inNatBlock {
			if line == "COMMIT" {
				inNatBlock = false
				continue
			}
			if strings.HasPrefix(line, "*nat") ||
				strings.HasPrefix(line, ":PREROUTING") ||
				strings.HasPrefix(line, "-A PREROUTING") {
				continue
			}
			// Empty line after COMMIT
			if line == "" {
				continue
			}
		}
		newLines = append(newLines, line)
	}

	os.WriteFile(filePath, []byte(strings.Join(newLines, "\n")), 0640)
}

func clearIp6tablesRules() {
	clearIp6tablesRulesForPort(DnsttPort)
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
