// Package menu provides the interactive menu for dnstm.
package menu

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/dnstm/internal/updater"
	"github.com/net2share/dnstm/internal/version"
	"github.com/net2share/go-corelib/tui"
)

// errCancelled is returned when user cancels/backs out.
var errCancelled = errors.New("cancelled")

// Version and BuildTime are set by cmd package.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

const dnstmBanner = `
    ____  _   _______  ________  ___
   / __ \/ | / / ___/ /_  __/  |/  /
  / / / /  |/ /\__ \   / / / /|_/ /
 / /_/ / /|  /___/ /  / / / /  / /
/_____/_/ |_//____/  /_/ /_/  /_/
`

// PrintBanner displays the dnstm banner with version info.
func PrintBanner() {
	tui.PrintBanner(tui.BannerConfig{
		AppName:   "DNS Tunnel Manager",
		Version:   Version,
		BuildTime: BuildTime,
		ASCII:     dnstmBanner,
	})
}

// InitTUI sets up the TUI environment. Must be called before any interactive menu.
func InitTUI() {
	Version = version.Version
	BuildTime = version.BuildTime
	tui.SetAppInfo("dnstm", version.Version, version.BuildTime)
	tui.BeginSession()
}

// HasInteractiveMenu returns true if the action has a registered interactive submenu.
func HasInteractiveMenu(actionID string) bool {
	switch actionID {
	case actions.ActionTunnel, actions.ActionBackend, actions.ActionRouter:
		return true
	}
	return false
}

// RunSubmenuByID launches the interactive submenu for a parent action.
// Returns nil when user exits.
func RunSubmenuByID(actionID string) error {
	var err error
	switch actionID {
	case actions.ActionTunnel:
		err = runTunnelMenu()
	case actions.ActionBackend:
		err = runBackendMenu()
	case actions.ActionRouter:
		err = RunSubmenu(actions.ActionRouter)
	default:
		return fmt.Errorf("no interactive menu for '%s'", actionID)
	}
	if err == errCancelled {
		return nil
	}
	return err
}

// RunInteractive shows the main interactive menu.
func RunInteractive() error {
	defer tui.EndSession()
	return runMainMenu()
}

// Update check caching for session
var (
	updateCheckCache   *updater.UpdateReport
	updateCheckDone    bool
	updateCheckMutex   sync.Mutex
	updateCheckStarted bool
)

const updateCheckTimeout = 15 * time.Second

// checkForUpdatesBanner returns a banner message if updates are available.
// On first call, runs a background check and waits up to timeout.
// On subsequent calls, returns cached result immediately.
func checkForUpdatesBanner() string {
	updateCheckMutex.Lock()

	if updateCheckDone {
		report := updateCheckCache
		updateCheckMutex.Unlock()
		if report == nil || !report.HasUpdates() {
			return ""
		}
		return formatUpdateBanner(report)
	}

	// Start background check if not started
	if !updateCheckStarted {
		updateCheckStarted = true
		done := make(chan struct{})
		go func() {
			report, _ := updater.CheckForUpdates(version.Version, updater.UpdateOptions{})
			updateCheckMutex.Lock()
			updateCheckCache = report
			updateCheckDone = true
			updateCheckMutex.Unlock()
			close(done)
		}()
		updateCheckMutex.Unlock()

		// Wait for the check to complete or timeout
		select {
		case <-done:
		case <-time.After(updateCheckTimeout):
		}

		// Return whatever we have
		updateCheckMutex.Lock()
		report := updateCheckCache
		updateCheckMutex.Unlock()
		if report == nil || !report.HasUpdates() {
			return ""
		}
		return formatUpdateBanner(report)
	}

	updateCheckMutex.Unlock()
	return ""
}

// formatUpdateBanner formats the update banner message.
func formatUpdateBanner(report *updater.UpdateReport) string {
	if report == nil || !report.HasUpdates() {
		return ""
	}

	var parts []string

	if report.DnstmUpdate != nil {
		parts = append(parts, fmt.Sprintf("dnstm %s -> %s",
			report.DnstmUpdate.Current, report.DnstmUpdate.Latest))
	}

	for _, bu := range report.BinaryUpdates {
		current := bu.CurrentVersion
		if current == "" {
			current = "?"
		}
		parts = append(parts, fmt.Sprintf("%s %s -> %s",
			bu.Binary, current, bu.LatestVersion))
	}

	return fmt.Sprintf("Updates available: %s", strings.Join(parts, ", "))
}

// buildTunnelSummary builds a summary string for the main menu header.
func buildTunnelSummary() string {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return ""
	}

	total := len(cfg.Tunnels)
	running := 0
	for _, t := range cfg.Tunnels {
		tunnel := router.NewTunnel(&t)
		if tunnel.IsActive() {
			running++
		}
	}

	if cfg.IsSingleMode() && cfg.Route.Active != "" {
		return fmt.Sprintf("Tunnels: %d | Running: %d | Active: %s", total, running, cfg.Route.Active)
	}
	return fmt.Sprintf("Tunnels: %d | Running: %d", total, running)
}

func runMainMenu() error {
	for {
		// Check if transport binaries are installed
		installed := transport.IsInstalled()

		var options []tui.MenuOption
		var header string
		var description string

		if !installed {
			// Not installed - show install option first and limited menu
			missing := transport.GetMissingBinaries()
			description = fmt.Sprintf("⚠ dnstm not installed\nMissing: %v", missing)

			options = append(options, tui.MenuOption{Label: "Install (Required)", Value: actions.ActionInstall})
			options = append(options, tui.MenuOption{Label: "Exit", Value: "exit"})
		} else {
			// Build tunnel summary for header
			header = buildTunnelSummary()

			// Check for updates (async, cached)
			if updateBanner := checkForUpdatesBanner(); updateBanner != "" {
				description = updateBanner
			}

			// Fully installed - show all options
			options = append(options, tui.MenuOption{Label: "Tunnels →", Value: actions.ActionTunnel})
			options = append(options, tui.MenuOption{Label: "Backends →", Value: actions.ActionBackend})
			options = append(options, tui.MenuOption{Label: "Router →", Value: actions.ActionRouter})
			options = append(options, tui.MenuOption{Label: "Update", Value: actions.ActionUpdate})
			options = append(options, tui.MenuOption{Label: "Uninstall", Value: actions.ActionUninstall})
			options = append(options, tui.MenuOption{Label: "", Separator: true})
			options = append(options, tui.MenuOption{Label: "External Tools", Separator: true})
			options = append(options, tui.MenuOption{Label: "SSH Users ↗", Value: actions.ActionSSHUsers})
			options = append(options, tui.MenuOption{Label: "", Separator: true})
			options = append(options, tui.MenuOption{Label: "Exit", Value: "exit"})
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Header:      header,
			Title:       "DNSTM",
			Description: description,
			Options:     options,
		})
		if err != nil {
			return err
		}

		if choice == "" || choice == "exit" {
			return nil
		}

		err = handleMainMenuChoice(choice)
		if errors.Is(err, errCancelled) {
			continue
		}
		if err != nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
		}
	}
}

func handleMainMenuChoice(choice string) error {
	switch choice {
	case actions.ActionRouter:
		return RunSubmenu(actions.ActionRouter)
	case actions.ActionTunnel:
		return runTunnelMenu()
	case actions.ActionBackend:
		return runBackendMenu()
	case actions.ActionSSHUsers:
		return RunAction(actions.ActionSSHUsers)
	case actions.ActionInstall:
		if err := RunAction(actions.ActionInstall); err != nil {
			if err != errCancelled {
				return err
			}
			return errCancelled
		}
		// No WaitForEnter needed - progress view handles its own dismissal
		return errCancelled
	case actions.ActionUpdate:
		if err := RunAction(actions.ActionUpdate); err != nil {
			if err == errCancelled {
				return errCancelled
			}
			return err
		}
		// Clear update cache after update
		updateCheckMutex.Lock()
		updateCheckCache = nil
		updateCheckDone = false
		updateCheckStarted = false
		updateCheckMutex.Unlock()
		return errCancelled
	case actions.ActionUninstall:
		if err := RunAction(actions.ActionUninstall); err != nil {
			if err == errCancelled {
				return errCancelled
			}
			return err
		}
		tui.EndSession()
		os.Exit(0)
	}
	return nil
}

// runTunnelMenu shows the tunnel submenu with special handling for list navigation.
func runTunnelMenu() error {
	for {
		options := []tui.MenuOption{
			{Label: "Add", Value: actions.ActionTunnelAdd},
			{Label: "List →", Value: "list"},
			{Label: "Back", Value: "back"},
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Tunnels",
			Options: options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		switch choice {
		case actions.ActionTunnelAdd:
			if err := RunAction(actions.ActionTunnelAdd); err != nil {
				if err != errCancelled {
					_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
				}
			}
			// No WaitForEnter needed - progress view handles its own dismissal
		case "list":
			// List menu handles its own navigation
			_ = runTunnelListMenu()
		}
	}
}

// runTunnelListMenu shows all tunnels and allows selecting one to manage.
func runTunnelListMenu() error {
	for {
		cfg, err := config.Load()
		if err != nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: "Failed to load config: " + err.Error()})
			return nil
		}

		if len(cfg.Tunnels) == 0 {
			_ = tui.ShowMessage(tui.AppMessage{Type: "info", Message: "No tunnels configured. Add one first."})
			return errCancelled
		}

		var options []tui.MenuOption
		for _, t := range cfg.Tunnels {
			tunnel := router.NewTunnel(&t)
			status := "○"
			if tunnel.IsActive() {
				status = "●"
			}
			transportName := config.GetTransportTypeDisplayName(t.Transport)
			label := fmt.Sprintf("%s %s (%s → %s)", status, t.Tag, transportName, t.Backend)
			options = append(options, tui.MenuOption{Label: label, Value: t.Tag})
		}
		options = append(options, tui.MenuOption{Label: "Back", Value: "back"})

		selected, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Select Tunnel",
			Options: options,
		})
		if err != nil || selected == "" || selected == "back" {
			return errCancelled
		}

		if err := runTunnelManageMenu(selected); err != errCancelled {
			tui.WaitForEnter()
		}
	}
}

// runTunnelManageMenu shows management options for a specific tunnel.
func runTunnelManageMenu(tag string) error {
	for {
		cfg, err := config.Load()
		if err != nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: "Failed to load config: " + err.Error()})
			return nil
		}

		tunnelCfg := cfg.GetTunnelByTag(tag)
		if tunnelCfg == nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: fmt.Sprintf("Tunnel '%s' not found", tag)})
			return nil
		}

		tunnel := router.NewTunnel(tunnelCfg)
		status := "Stopped"
		if tunnel.IsActive() {
			status = "Running"
		}

		isRunning := tunnel.IsActive()

		// Build context-aware options
		options := []tui.MenuOption{
			{Label: "Status", Value: "status"},
			{Label: "Logs", Value: "logs"},
		}

		// Only show start/stop/restart for active tunnel (single mode) or any tunnel (multi mode)
		canManage := cfg.IsMultiMode() || (cfg.IsSingleMode() && cfg.Route.Active == tag)
		if canManage {
			if isRunning {
				options = append(options,
					tui.MenuOption{Label: "Restart", Value: "restart"},
					tui.MenuOption{Label: "Stop", Value: "stop"},
				)
			} else {
				options = append(options,
					tui.MenuOption{Label: "Start", Value: "start"},
				)
			}
		}

		options = append(options,
			tui.MenuOption{Label: "Remove", Value: "remove"},
			tui.MenuOption{Label: "Back", Value: "back"},
		)

		transportName := config.GetTransportTypeDisplayName(tunnelCfg.Transport)
		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:       fmt.Sprintf("%s (%s)", tag, status),
			Description: fmt.Sprintf("%s → %s:%d", transportName, tunnelCfg.Domain, tunnelCfg.Port),
			Options:     options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		// Execute the action with the tunnel tag as argument
		actionID := "tunnel." + choice
		if err := runTunnelAction(actionID, tag); err != nil {
			if err == errCancelled {
				continue
			}
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
		} else {
			// Check if tunnel was removed
			if choice == "remove" {
				return errCancelled
			}
			// Skip WaitForEnter for actions that use TUI info/progress view
			if !isInfoViewAction(actionID) {
				tui.WaitForEnter()
			}
		}
	}
}

// runTunnelAction runs a tunnel action with the given tag as argument.
func runTunnelAction(actionID, tunnelTag string) error {
	// Special handling for actions that need the tunnel tag
	switch actionID {
	case actions.ActionTunnelStatus, actions.ActionTunnelLogs, actions.ActionTunnelStart,
		actions.ActionTunnelStop, actions.ActionTunnelRestart, actions.ActionTunnelRemove:
		return runActionWithArgs(actionID, []string{tunnelTag})
	default:
		return RunAction(actionID)
	}
}

// runActionWithArgs runs an action with predefined arguments, handling confirmation if needed.
func runActionWithArgs(actionID string, args []string) error {
	action := actions.Get(actionID)
	if action == nil {
		return fmt.Errorf("unknown action: %s", actionID)
	}

	// Handle confirmation (for actions with a tag argument, include tag in message)
	if action.Confirm != nil && len(args) > 0 {
		tag := args[0]
		confirm, err := tui.RunConfirm(tui.ConfirmConfig{
			Title:       fmt.Sprintf("%s '%s'?", action.Confirm.Message, tag),
			Description: action.Confirm.Description,
			Default:     !action.Confirm.DefaultNo,
		})
		if err != nil {
			return err
		}
		if !confirm {
			return errCancelled
		}
	}

	// Build context and set tag from args
	ctx := newActionContext(nil)
	if action.Args != nil && action.Args.Name == "tag" && len(args) > 0 {
		ctx.Values["tag"] = args[0]
	}

	if action.Handler == nil {
		return fmt.Errorf("no handler for action %s", actionID)
	}

	return action.Handler(ctx)
}

// runBackendMenu shows the backend submenu with special handling for list navigation.
func runBackendMenu() error {
	for {
		options := []tui.MenuOption{
			{Label: "Add", Value: actions.ActionBackendAdd},
			{Label: "List →", Value: "list"},
			{Label: "Available Types", Value: actions.ActionBackendAvailable},
			{Label: "Back", Value: "back"},
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Backends",
			Options: options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		switch choice {
		case actions.ActionBackendAdd:
			if err := RunAction(actions.ActionBackendAdd); err != nil {
				if err != errCancelled {
					_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
				}
			} else if !isInfoViewAction(actions.ActionBackendAdd) {
				tui.WaitForEnter()
			}
		case "list":
			// List menu handles its own navigation
			_ = runBackendListMenu()
		case actions.ActionBackendAvailable:
			if err := RunAction(actions.ActionBackendAvailable); err != nil {
				if err != errCancelled {
					_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
				}
			}
			// No WaitForEnter needed - ShowInfo handles its own dismissal
		}
	}
}

// runBackendListMenu shows all backends and allows selecting one to manage.
func runBackendListMenu() error {
	for {
		cfg, err := config.Load()
		if err != nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: "Failed to load config: " + err.Error()})
			return nil
		}

		if len(cfg.Backends) == 0 {
			_ = tui.ShowMessage(tui.AppMessage{Type: "info", Message: "No backends configured. Add one first."})
			return errCancelled
		}

		var options []tui.MenuOption
		for _, b := range cfg.Backends {
			typeName := config.GetBackendTypeDisplayName(b.Type)
			status := ""
			if b.IsBuiltIn() {
				status = " [built-in]"
			}
			label := fmt.Sprintf("%s (%s)%s", b.Tag, typeName, status)
			options = append(options, tui.MenuOption{Label: label, Value: b.Tag})
		}
		options = append(options, tui.MenuOption{Label: "Back", Value: "back"})

		selected, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Select Backend",
			Options: options,
		})
		if err != nil || selected == "" || selected == "back" {
			return errCancelled
		}

		if err := runBackendManageMenu(selected); err != errCancelled {
			tui.WaitForEnter()
		}
	}
}

// runBackendManageMenu shows management options for a specific backend.
func runBackendManageMenu(tag string) error {
	for {
		cfg, err := config.Load()
		if err != nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: "Failed to load config: " + err.Error()})
			return nil
		}

		backend := cfg.GetBackendByTag(tag)
		if backend == nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: fmt.Sprintf("Backend '%s' not found", tag)})
			return nil
		}

		typeName := config.GetBackendTypeDisplayName(backend.Type)

		options := []tui.MenuOption{
			{Label: "Status", Value: "status"},
		}

		// Only show Remove for non-built-in backends
		if !backend.IsBuiltIn() {
			options = append(options, tui.MenuOption{Label: "Remove", Value: "remove"})
		}

		options = append(options, tui.MenuOption{Label: "Back", Value: "back"})

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:       fmt.Sprintf("%s (%s)", tag, typeName),
			Description: getBackendDescription(backend),
			Options:     options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		// Execute the action with the backend tag as argument
		actionID := "backend." + choice
		if err := runBackendAction(actionID, tag); err != nil {
			if err == errCancelled {
				continue
			}
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
		} else {
			// Check if backend was removed
			if choice == "remove" {
				return errCancelled
			}
			// Skip WaitForEnter for actions that use TUI info/progress view
			if !isInfoViewAction(actionID) {
				tui.WaitForEnter()
			}
		}
	}
}

// getBackendDescription returns a description for a backend.
func getBackendDescription(b *config.BackendConfig) string {
	if b.Type == config.BackendShadowsocks {
		return "SIP003 plugin mode"
	}
	if b.Address != "" {
		return b.Address
	}
	return ""
}

// runBackendAction runs a backend action with the given tag as argument.
func runBackendAction(actionID, backendTag string) error {
	switch actionID {
	case actions.ActionBackendStatus, actions.ActionBackendRemove:
		return runActionWithArgs(actionID, []string{backendTag})
	default:
		return RunAction(actionID)
	}
}
