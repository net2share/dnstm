// Package sshtunnel provides SSH tunnel user management integration for dnstm.
// This is a thin wrapper around sshtun-user's pkg/cli module.
package sshtunnel

import (
	"github.com/net2share/sshtun-user/pkg/cli"
)

// CreatedUserInfo holds information about a created tunnel user
type CreatedUserInfo struct {
	Username string
	AuthMode string
	Password string // Only set if password auth and auto-generated
}

// ConfigureAndCreateUser auto-configures sshd hardening and prompts for user creation.
// Used during dnstm SSH mode installation. Returns user info if created, nil otherwise.
func ConfigureAndCreateUser() *CreatedUserInfo {
	userInfo := cli.ConfigureAndCreateUser()
	if userInfo == nil {
		return nil
	}
	return &CreatedUserInfo{
		Username: userInfo.Username,
		AuthMode: userInfo.AuthMode,
		Password: userInfo.Password,
	}
}

// ShowMenu displays the SSH tunnel users submenu.
func ShowMenu() {
	cli.ShowUserManagementMenu()
}

// IsConfigured checks if SSH tunnel hardening has been applied.
func IsConfigured() bool {
	return cli.IsConfigured()
}

// UninstallAll performs complete uninstall of SSH tunnel users and config.
func UninstallAll() error {
	return cli.UninstallAllNonInteractive()
}
