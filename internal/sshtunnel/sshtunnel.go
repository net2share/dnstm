// Package sshtunnel provides SSH tunnel user management integration for dnstm.
// This is a thin wrapper around sshtun-user's pkg/cli module.
package sshtunnel

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/net2share/sshtun-user/pkg/cli"
	"github.com/net2share/sshtun-user/pkg/fail2ban"
	"github.com/net2share/sshtun-user/pkg/sshdconfig"
	"github.com/net2share/sshtun-user/pkg/tunneluser"
)

// CreatedUserInfo holds information about a created tunnel user
type CreatedUserInfo struct {
	Username string
	AuthMode string
	Password string // Only set if password auth and auto-generated
}

// ConfigureAndCreateUser auto-configures sshd hardening and prompts for user creation.
// Used during dnstm SSH mode installation. Returns user info if created, nil otherwise.
// If existing tunnel users exist, asks if user wants to add a new one.
func ConfigureAndCreateUser() *CreatedUserInfo {
	fmt.Println()
	tui.PrintInfo("Applying sshd hardening configuration...")

	// Apply sshd hardening
	if err := sshdconfig.Configure(); err != nil {
		tui.PrintError("Failed to configure sshd: " + err.Error())
		return nil
	}
	tui.PrintStatus("sshd hardening applied")

	// Configure fail2ban
	osInfo, _ := osdetect.Detect()
	if err := fail2ban.SetupWithFeedback(osInfo); err != nil {
		tui.PrintWarning("fail2ban setup warning: " + err.Error())
	}

	// Check for existing tunnel users
	existingUsers, err := tunneluser.List()
	if err != nil {
		tui.PrintWarning("Could not check for existing users: " + err.Error())
	}

	fmt.Println()

	// If existing users, ask if they want to add a new one
	if len(existingUsers) > 0 {
		tui.PrintInfo(fmt.Sprintf("Found %d existing SSH tunnel user(s)", len(existingUsers)))

		var addNew bool
		err := huh.NewConfirm().
			Title("Do you want to add a new SSH tunnel user?").
			Affirmative("Yes").
			Negative("No").
			Value(&addNew).
			Run()
		if err != nil {
			return nil
		}

		if !addNew {
			return nil
		}
	}

	// Create tunnel user
	userInfo, err := createTunnelUser()
	if err != nil {
		tui.PrintError(err.Error())
		return nil
	}

	return userInfo
}

// createTunnelUser prompts for username and creates a tunnel user.
func createTunnelUser() (*CreatedUserInfo, error) {
	var username string
	err := huh.NewInput().
		Title("Username").
		Description("Enter username for tunnel user").
		Value(&username).
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("username required")
			}
			if tunneluser.Exists(s) {
				return fmt.Errorf("user '%s' already exists", s)
			}
			return nil
		}).
		Run()
	if err != nil {
		return nil, err
	}

	var authMode string
	err = huh.NewSelect[string]().
		Title("Authentication Method").
		Options(
			huh.NewOption("Password - simpler, suitable for shared access", "password"),
			huh.NewOption("SSH Key - more secure, user provides public key", "key"),
		).
		Value(&authMode).
		Run()
	if err != nil {
		return nil, err
	}

	cfg := &tunneluser.Config{
		Username: username,
	}

	userInfo := &CreatedUserInfo{
		Username: username,
		AuthMode: authMode,
	}

	if authMode == "key" {
		cfg.AuthMode = tunneluser.AuthModeKey
		var key string
		err = huh.NewInput().
			Title("SSH Public Key").
			Description(fmt.Sprintf("Enter public key for '%s' (from ~/.ssh/id_ed25519.pub)", username)).
			Value(&key).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("public key is required for key-based auth")
				}
				if err := tunneluser.ValidatePublicKey(s); err != nil {
					return fmt.Errorf("invalid public key format: %w", err)
				}
				return nil
			}).
			Run()
		if err != nil {
			return nil, err
		}
		cfg.PublicKey = key
	} else {
		cfg.AuthMode = tunneluser.AuthModePassword
		var password string
		err = huh.NewInput().
			Title("Password").
			Description(fmt.Sprintf("Enter password for '%s' (leave empty to auto-generate)", username)).
			Value(&password).
			Run()
		if err != nil {
			return nil, err
		}

		if password == "" {
			generated, err := tunneluser.GeneratePassword()
			if err != nil {
				return nil, fmt.Errorf("failed to generate password: %w", err)
			}
			password = generated
			tui.PrintBox("Generated Password (save this now!)", []string{tui.Code(password)})
		}
		cfg.Password = password
		userInfo.Password = password
	}

	if err := tunneluser.Create(cfg); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if cfg.AuthMode == tunneluser.AuthModeKey {
		if err := sshdconfig.AddAuthorizedKeysDirective(); err != nil {
			tui.PrintWarning("Could not add AuthorizedKeysFile directive: " + err.Error())
		}
	}

	fmt.Println()
	tui.PrintSuccess(fmt.Sprintf("User '%s' created successfully!", username))

	return userInfo, nil
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

// StatusInfo contains SSH tunnel hardening status information.
type StatusInfo struct {
	Configured       bool
	UserCount        int
	PasswordAuthCount int
	KeyAuthCount     int
}

// GetStatus returns the current SSH tunnel hardening status.
func GetStatus() *StatusInfo {
	info := &StatusInfo{
		Configured: IsConfigured(),
	}

	users, err := tunneluser.List()
	if err != nil {
		return info
	}

	info.UserCount = len(users)
	for _, u := range users {
		switch u.AuthMode {
		case tunneluser.AuthModePassword:
			info.PasswordAuthCount++
		case tunneluser.AuthModeKey:
			info.KeyAuthCount++
		}
	}

	return info
}
