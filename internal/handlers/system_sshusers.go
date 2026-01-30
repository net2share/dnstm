package handlers

import (
	"os"
	"syscall"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/transport"
)

func init() {
	actions.SetSystemHandler(actions.ActionSSHUsers, HandleSSHUsers)
}

// HandleSSHUsers launches the sshtun-user binary.
func HandleSSHUsers(ctx *actions.Context) error {
	if !transport.IsSSHTunUserInstalled() {
		return actions.NewActionError(
			"sshtun-user is not installed",
			"Run 'dnstm install' first",
		)
	}

	// Get the binary path
	binary := transport.SSHTunUserBinary

	// Use syscall.Exec to replace the current process with sshtun-user
	// This allows sshtun-user to run in fully interactive mode
	if err := syscall.Exec(binary, []string{binary}, os.Environ()); err != nil {
		return err
	}

	// This line is never reached as Exec replaces the process
	return nil
}
