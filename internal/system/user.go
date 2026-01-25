package system

import (
	"fmt"
	"os/exec"
	"os/user"
)

const (
	DnsttUser      = "dnstt"
	SlipstreamUser = "slipstream"
)

// CreateSystemUser creates a system user with no home directory and nologin shell.
func CreateSystemUser(username string) error {
	if _, err := user.Lookup(username); err == nil {
		return nil
	}

	cmd := exec.Command("useradd",
		"--system",
		"--no-create-home",
		"--shell", "/usr/sbin/nologin",
		username,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create user: %s: %w", string(output), err)
	}

	return nil
}

// UserExists checks if a system user exists.
func UserExists(username string) bool {
	_, err := user.Lookup(username)
	return err == nil
}

// RemoveSystemUser removes a system user.
func RemoveSystemUser(username string) {
	if _, err := user.Lookup(username); err != nil {
		return
	}

	exec.Command("userdel", username).Run()
}

// CreateDnsttUser creates the dnstt system user (backward compatible wrapper).
func CreateDnsttUser() error {
	return CreateSystemUser(DnsttUser)
}

// DnsttUserExists checks if the dnstt user exists (backward compatible wrapper).
func DnsttUserExists() bool {
	return UserExists(DnsttUser)
}

// RemoveDnsttUser removes the dnstt user (backward compatible wrapper).
func RemoveDnsttUser() {
	RemoveSystemUser(DnsttUser)
}

// CreateSlipstreamUser creates the slipstream system user.
func CreateSlipstreamUser() error {
	return CreateSystemUser(SlipstreamUser)
}

// SlipstreamUserExists checks if the slipstream user exists.
func SlipstreamUserExists() bool {
	return UserExists(SlipstreamUser)
}

// RemoveSlipstreamUser removes the slipstream user.
func RemoveSlipstreamUser() {
	RemoveSystemUser(SlipstreamUser)
}
