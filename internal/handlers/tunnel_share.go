package handlers

import (
	"fmt"
	"os"
	"time"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/clientcfg"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/system"
	"golang.org/x/crypto/ssh"
)

func init() {
	actions.SetTunnelHandler(actions.ActionTunnelShare, HandleTunnelShare)
}

// HandleTunnelShare generates a dnst:// URL for client configuration.
func HandleTunnelShare(ctx *actions.Context) error {
	cfg, err := RequireConfig(ctx)
	if err != nil {
		return err
	}

	tag, err := RequireTag(ctx, "tunnel")
	if err != nil {
		return err
	}

	tunnelCfg := cfg.GetTunnelByTag(tag)
	if tunnelCfg == nil {
		return actions.TunnelNotFoundError(tag)
	}

	backend := cfg.GetBackendByTag(tunnelCfg.Backend)
	if backend == nil {
		return actions.BackendNotFoundError(tunnelCfg.Backend)
	}

	opts := clientcfg.GenerateOptions{
		NoCert: ctx.GetBool("no-cert"),
	}

	// Collect and validate SSH-specific inputs
	if backend.Type == config.BackendSSH {
		opts.User = ctx.GetString("user")
		opts.Password = ctx.GetString("password")
		opts.PrivateKey = ctx.GetString("key")

		if opts.User == "" {
			hint := "Provide --user flag"
			if ctx.IsInteractive {
				hint = "Enter a valid system user"
			}
			return actions.NewActionError("SSH user is required", hint)
		}
		if !system.UserExists(opts.User) {
			hint := "Provide a valid system user with --user"
			if ctx.IsInteractive {
				hint = "The user must exist on this system"
			}
			return actions.NewActionError(
				fmt.Sprintf("user '%s' does not exist on this system", opts.User), hint,
			)
		}
		if opts.Password == "" && opts.PrivateKey == "" {
			hint := "Provide --password or --key flag"
			if ctx.IsInteractive {
				hint = "Provide a password or path to a private key"
			}
			return actions.NewActionError("SSH password or private key is required", hint)
		}

		// Validate credentials by attempting SSH connection
		addr := backend.Address
		if addr == "" {
			addr = GetDefaultSSHAddress()
		}

		if opts.Password != "" {
			if err := validateSSHPassword(addr, opts.User, opts.Password); err != nil {
				return actions.NewActionError(
					fmt.Sprintf("SSH authentication failed for '%s'", opts.User),
					"Check the password and try again",
				)
			}
		}

		if opts.PrivateKey != "" {
			if err := validateSSHKey(addr, opts.User, opts.PrivateKey); err != nil {
				return actions.NewActionError(
					fmt.Sprintf("SSH key authentication failed for '%s': %v", opts.User, err),
					"Check the private key path and ensure its public key is in authorized_keys",
				)
			}
		}
	}

	clientCfg, err := clientcfg.Generate(tunnelCfg, backend, opts)
	if err != nil {
		return fmt.Errorf("failed to generate client config: %w", err)
	}

	url, err := clientcfg.Encode(clientCfg)
	if err != nil {
		return fmt.Errorf("failed to encode client config: %w", err)
	}

	if ctx.IsInteractive {
		// Print directly to terminal (not TUI) so the URL is easily selectable
		fmt.Println()
		fmt.Printf("Share: %s\n\n", tag)
		fmt.Println(url)
		fmt.Println()
		fmt.Printf("Transport: %s\n", config.GetTransportTypeDisplayName(tunnelCfg.Transport))
		fmt.Printf("Backend:   %s\n", config.GetBackendTypeDisplayName(backend.Type))
		fmt.Printf("Domain:    %s\n", tunnelCfg.Domain)
		fmt.Println()
		fmt.Print("Press Enter to continue...")
		fmt.Scanln()
		return nil
	}

	ctx.Output.Println(url)
	return nil
}

// validateSSHAuth attempts an SSH connection with the given auth methods.
func validateSSHAuth(addr, user string, methods ...ssh.AuthMethod) error {
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            user,
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("authentication failed")
	}
	client.Close()
	return nil
}

// validateSSHPassword attempts an SSH connection with password auth.
func validateSSHPassword(addr, user, password string) error {
	return validateSSHAuth(addr, user, ssh.Password(password))
}

// validateSSHKey attempts an SSH connection with private key auth.
func validateSSHKey(addr, user, keyPath string) error {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("cannot read key file: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	return validateSSHAuth(addr, user, ssh.PublicKeys(signer))
}
