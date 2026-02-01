package system

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
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

// ChownToDnstm changes ownership of a file or directory to the dnstm user.
func ChownToDnstm(path string) error {
	u, err := user.Lookup(DnstmUser)
	if err != nil {
		return fmt.Errorf("user %s not found: %w", DnstmUser, err)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("invalid uid: %w", err)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("invalid gid: %w", err)
	}

	return os.Chown(path, uid, gid)
}

// ChownDirToDnstm recursively changes ownership of a directory to the dnstm user.
func ChownDirToDnstm(path string) error {
	u, err := user.Lookup(DnstmUser)
	if err != nil {
		return fmt.Errorf("user %s not found: %w", DnstmUser, err)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("invalid uid: %w", err)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("invalid gid: %w", err)
	}

	// Use chown -R for recursive ownership change
	cmd := exec.Command("chown", "-R", fmt.Sprintf("%d:%d", uid, gid), path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chown failed: %s: %w", string(output), err)
	}

	return nil
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

// CanDnstmUserReadFile checks if the dnstm user can read the specified file.
// Returns true if the file exists and is readable by the dnstm user.
func CanDnstmUserReadFile(path string) (bool, error) {
	u, err := user.Lookup(DnstmUser)
	if err != nil {
		return false, fmt.Errorf("user %s not found: %w", DnstmUser, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	// Get file owner info
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("failed to get file stat")
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	mode := info.Mode()

	// Check if dnstm user owns the file
	if int(stat.Uid) == uid {
		return mode&0400 != 0, nil // Owner read permission
	}

	// Check if dnstm group owns the file
	if int(stat.Gid) == gid {
		return mode&0040 != 0, nil // Group read permission
	}

	// Check world read permission
	return mode&0004 != 0, nil
}
