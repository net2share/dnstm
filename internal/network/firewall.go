package network

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const DnsttPort = "5300"

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

func ConfigureFirewall() error {
	fwType := DetectFirewall()

	switch fwType {
	case FirewallFirewalld:
		return configureFirewalld()
	case FirewallUFW:
		return configureUFW()
	case FirewallIptables, FirewallNone:
		return configureIptables()
	}

	return nil
}

func configureFirewalld() error {
	cmds := [][]string{
		{"firewall-cmd", "--permanent", "--add-port=53/udp"},
		{"firewall-cmd", "--permanent", "--add-port=53/tcp"},
		{"firewall-cmd", "--permanent", "--add-port=" + DnsttPort + "/udp"},
		{"firewall-cmd", "--permanent", "--add-port=" + DnsttPort + "/tcp"},
		{"firewall-cmd", "--permanent", "--add-masquerade"},
		{"firewall-cmd", "--permanent", "--direct", "--add-rule", "ipv4", "nat", "PREROUTING", "0", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", DnsttPort},
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
	// Allow port 53 for external DNS queries
	// Allow port 5300 because after NAT PREROUTING redirects 53->5300,
	// packets arrive at INPUT chain with dport 5300
	cmds := [][]string{
		{"ufw", "allow", "53/udp"},
		{"ufw", "allow", "53/tcp"},
		{"ufw", "allow", DnsttPort + "/udp"},
		{"ufw", "allow", DnsttPort + "/tcp"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Run()
	}

	// Add NAT rules to /etc/ufw/before.rules for persistence
	if err := addUFWNatRules(); err != nil {
		// Fall back to direct iptables if UFW config fails
		return configureIptables()
	}

	// Reload UFW to apply the NAT rules
	exec.Command("ufw", "reload").Run()

	return nil
}

const ufwBeforeRulesPath = "/etc/ufw/before.rules"
const dnsttNatMarker = "# NAT table rules for dnstt"

func addUFWNatRules() error {
	content, err := os.ReadFile(ufwBeforeRulesPath)
	if err != nil {
		return err
	}

	// Check if NAT rules already exist
	if strings.Contains(string(content), dnsttNatMarker) {
		return nil
	}

	// NAT rules to prepend before the *filter section
	natRules := fmt.Sprintf(`%s - redirect port 53 to %s
*nat
:PREROUTING ACCEPT [0:0]
-A PREROUTING -p udp --dport 53 -j REDIRECT --to-ports %s
-A PREROUTING -p tcp --dport 53 -j REDIRECT --to-ports %s
COMMIT

`, dnsttNatMarker, DnsttPort, DnsttPort, DnsttPort)

	// Prepend NAT rules to the file
	newContent := natRules + string(content)

	if err := os.WriteFile(ufwBeforeRulesPath, []byte(newContent), 0640); err != nil {
		return err
	}

	return nil
}

func configureIptables() error {
	clearIptablesRules()

	rules := [][]string{
		{"-t", "nat", "-A", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", DnsttPort},
		{"-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", "53", "-j", "REDIRECT", "--to-ports", DnsttPort},
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
	rules := [][]string{
		{"-t", "nat", "-D", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", DnsttPort},
		{"-t", "nat", "-D", "PREROUTING", "-p", "tcp", "--dport", "53", "-j", "REDIRECT", "--to-ports", DnsttPort},
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

func ConfigureIPv6() error {
	fwType := DetectFirewall()

	if fwType == FirewallUFW {
		return addUFWNatRulesIPv6()
	}

	// Direct ip6tables for non-UFW systems
	rules := [][]string{
		{"-t", "nat", "-A", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", DnsttPort},
		{"-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", "53", "-j", "REDIRECT", "--to-ports", DnsttPort},
	}

	for _, args := range rules {
		exec.Command("ip6tables", args...).Run()
	}

	return nil
}

const ufwBefore6RulesPath = "/etc/ufw/before6.rules"

func addUFWNatRulesIPv6() error {
	content, err := os.ReadFile(ufwBefore6RulesPath)
	if err != nil {
		return err
	}

	// Check if NAT rules already exist
	if strings.Contains(string(content), dnsttNatMarker) {
		return nil
	}

	// NAT rules to prepend before the *filter section
	natRules := fmt.Sprintf(`%s - redirect port 53 to %s (IPv6)
*nat
:PREROUTING ACCEPT [0:0]
-A PREROUTING -p udp --dport 53 -j REDIRECT --to-ports %s
-A PREROUTING -p tcp --dport 53 -j REDIRECT --to-ports %s
COMMIT

`, dnsttNatMarker, DnsttPort, DnsttPort, DnsttPort)

	// Prepend NAT rules to the file
	newContent := natRules + string(content)

	if err := os.WriteFile(ufwBefore6RulesPath, []byte(newContent), 0640); err != nil {
		return err
	}

	// Reload UFW to apply
	exec.Command("ufw", "reload").Run()

	return nil
}
