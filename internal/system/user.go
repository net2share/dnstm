package system

import (
	"fmt"
	"os/exec"
	"os/user"
)

const (
	// DnstmUser is the shared system user for all dnstm services.
	DnstmUser = "dnstm"

	// Legacy user constants for backward compatibility
	DnsttUser       = DnstmUser
	SlipstreamUser  = DnstmUser
	ShadowsocksUser = DnstmUser
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

// CreateDnstmUser creates the shared dnstm system user.
func CreateDnstmUser() error {
	return CreateSystemUser(DnstmUser)
}

// DnstmUserExists checks if the dnstm user exists.
func DnstmUserExists() bool {
	return UserExists(DnstmUser)
}

// RemoveDnstmUserIfOrphaned removes the dnstm user only if no providers are installed.
// This should be called during uninstall instead of provider-specific remove functions.
func RemoveDnstmUserIfOrphaned(checkInstalled func() bool) {
	if checkInstalled() {
		// At least one provider is still installed, keep the user
		return
	}
	RemoveSystemUser(DnstmUser)
}

// RemoveDnstmUser removes the dnstm user unconditionally.
func RemoveDnstmUser() {
	RemoveSystemUser(DnstmUser)
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

// CreateShadowsocksUser creates the shadowsocks system user.
func CreateShadowsocksUser() error {
	return CreateSystemUser(ShadowsocksUser)
}

// ShadowsocksUserExists checks if the shadowsocks user exists.
func ShadowsocksUserExists() bool {
	return UserExists(ShadowsocksUser)
}

// RemoveShadowsocksUser removes the shadowsocks user.
func RemoveShadowsocksUser() {
	RemoveSystemUser(ShadowsocksUser)
}
