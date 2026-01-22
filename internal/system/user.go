package system

import (
	"fmt"
	"os/exec"
	"os/user"
)

const DnsttUser = "dnstt"

func CreateDnsttUser() error {
	if _, err := user.Lookup(DnsttUser); err == nil {
		return nil
	}

	cmd := exec.Command("useradd",
		"--system",
		"--no-create-home",
		"--shell", "/usr/sbin/nologin",
		DnsttUser,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create user: %s: %w", string(output), err)
	}

	return nil
}

func DnsttUserExists() bool {
	_, err := user.Lookup(DnsttUser)
	return err == nil
}
