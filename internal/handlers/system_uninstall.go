package handlers

import (
	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/installer"
)

func init() {
	actions.SetSystemHandler(actions.ActionUninstall, HandleUninstall)
}

// HandleUninstall performs a full system uninstall.
func HandleUninstall(ctx *actions.Context) error {
	// Note: Confirmation is handled by the adapter before calling the handler
	return installer.PerformFullUninstall()
}
