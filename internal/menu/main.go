// Package menu provides the interactive menu for dnstm.
package menu

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/installer"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/mtproxy"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/dnstm/internal/types"
	"github.com/net2share/go-corelib/osdetect"
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

// RunInteractive shows the main interactive menu.
func RunInteractive() error {
	PrintBanner()

	osInfo, err := osdetect.Detect()
	if err != nil {
		tui.PrintWarning("Could not detect OS: " + err.Error())
	} else {
		tui.PrintInfo(fmt.Sprintf("Detected OS: %s", osInfo.PrettyName))
	}

	arch := osdetect.GetArch()
	tui.PrintInfo(fmt.Sprintf("Architecture: %s", arch))

	return runMainMenu()
}

// buildInstanceSummary builds a summary string for the main menu header.
func buildInstanceSummary() string {
	cfg, err := router.Load()
	if err != nil || cfg == nil {
		return ""
	}

	total := len(cfg.Transports)
	running := 0
	for name, transport := range cfg.Transports {
		instance := router.NewInstance(name, transport)
		if instance.IsActive() {
			running++
		}
	}

	if cfg.IsSingleMode() && cfg.Single.Active != "" {
		return fmt.Sprintf("Instances: %d | Running: %d | Active: %s", total, running, cfg.Single.Active)
	}
	return fmt.Sprintf("Instances: %d | Running: %d", total, running)
}

func runMainMenu() error {
	firstRun := true
	for {
		if !firstRun {
			tui.ClearScreen()
			PrintBanner()
		}
		firstRun = false

		fmt.Println()

		// Check if transport binaries are installed
		installed := transport.IsInstalled()

		var options []tui.MenuOption
		var header string

		if !installed {
			// Not installed - show install option first and limited menu
			missing := transport.GetMissingBinaries()
			tui.PrintWarning("dnstm not installed")
			fmt.Printf("Missing: %v\n\n", missing)

			options = append(options, tui.MenuOption{Label: "Install (Required)", Value: "install"})
			options = append(options, tui.MenuOption{Label: "Exit", Value: "exit"})
		} else {
			// Build instance summary for header
			header = buildInstanceSummary()

			// Fully installed - show all options
			options = append(options, tui.MenuOption{Label: "Instances →", Value: "instance"})
			options = append(options, tui.MenuOption{Label: "Router →", Value: "router"})
			options = append(options, tui.MenuOption{Label: "SSH Users →", Value: "ssh-users"})
			options = append(options, tui.MenuOption{Label: "MTProxy →", Value: "mtproxy"})
			options = append(options, tui.MenuOption{Label: "Uninstall", Value: "uninstall"})
			options = append(options, tui.MenuOption{Label: "Exit", Value: "exit"})
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Header:  header,
			Title:   "DNS Tunnel Manager",
			Options: options,
		})

		if choice == "" || choice == "exit" {
			tui.PrintInfo("Goodbye!")
			return nil
		}

		err = handleMainMenuChoice(choice)
		if errors.Is(err, errCancelled) {
			continue
		}
		if err != nil {
			tui.PrintError(err.Error())
			tui.WaitForEnter()
		}
	}
}

func handleMainMenuChoice(choice string) error {
	switch choice {
	case "router":
		return runRouterMenu()
	case "instance":
		return runInstanceMenu()
	case "ssh-users":
		return runSSHUsersMenu()
	case "install":
		return runInstallBinaries()
	case "mtproxy":
		RunMTProxyMenu()
		return errCancelled
	case "mtproxy":
		RunMTProxyMenu()
		return errCancelled
	case "uninstall":
		return runUninstallMenu()
	}
	return nil
}

// runSSHUsersMenu launches the sshtun-user binary in interactive mode.
func runSSHUsersMenu() error {
	if !transport.IsSSHTunUserInstalled() {
		tui.PrintError("sshtun-user is not installed. Run 'Install' first.")
		tui.WaitForEnter()
		return errCancelled
	}

	// Get the binary path
	binary := transport.SSHTunUserBinary

	// Use syscall.Exec to replace the current process with sshtun-user
	// This allows sshtun-user to run in fully interactive mode
	if err := syscall.Exec(binary, []string{binary}, os.Environ()); err != nil {
		tui.PrintError("Failed to launch sshtun-user: " + err.Error())
		tui.WaitForEnter()
		return errCancelled
	}

	// This line is never reached as Exec replaces the process
	return nil
}

// ============================================================================
// Router Menu (matches: dnstm router)
// ============================================================================

func runRouterMenu() error {
	for {
		fmt.Println()

		if !router.IsInitialized() {
			tui.PrintWarning("Router not initialized. Run 'Install' from the main menu first.")
			tui.WaitForEnter()
			return errCancelled
		}

		cfg, _ := router.Load()
		modeName := "unknown"
		isSingleMode := false
		if cfg != nil {
			modeName = router.GetModeDisplayName(cfg.Mode)
			isSingleMode = cfg.IsSingleMode()
		}

		options := []tui.MenuOption{
			{Label: fmt.Sprintf("Mode: %s", modeName), Value: "mode"},
		}

		// Switch Active is only relevant in single mode
		if isSingleMode {
			activeLabel := "Switch Active: (none)"
			if cfg.Single.Active != "" {
				activeLabel = fmt.Sprintf("Switch Active: %s", cfg.Single.Active)
			}
			options = append(options, tui.MenuOption{Label: activeLabel, Value: "switch"})
		}

		options = append(options,
			tui.MenuOption{Label: "Status", Value: "status"},
			tui.MenuOption{Label: "Start/Restart", Value: "start"},
			tui.MenuOption{Label: "Stop", Value: "stop"},
			tui.MenuOption{Label: "Logs", Value: "logs"},
			tui.MenuOption{Label: "Back", Value: "back"},
		)

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Router",
			Options: options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		switch choice {
		case "status":
			runRouterStatus()
			tui.WaitForEnter()
		case "start":
			runRouterStart()
			tui.WaitForEnter()
		case "stop":
			runRouterStop()
			tui.WaitForEnter()
		case "logs":
			runRouterLogs()
			tui.WaitForEnter()
		case "mode":
			if err := runModeMenu(); err != errCancelled {
				tui.WaitForEnter()
			}
		case "switch":
			if err := runSwitchMenu(); err != errCancelled {
				tui.WaitForEnter()
			}
		}
	}
}

func runRouterStatus() {
	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		return
	}

	r, _ := router.New(cfg)

	fmt.Println()
	modeName := router.GetModeDisplayName(cfg.Mode)
	fmt.Printf("Mode: %s\n", modeName)

	if cfg.IsSingleMode() {
		if cfg.Single.Active != "" {
			instance := r.GetInstance(cfg.Single.Active)
			if instance != nil {
				status := "○ Stopped"
				if instance.IsActive() {
					status = "● Running"
				}
				typeName := router.GetTransportTypeDisplayName(instance.Type)
				fmt.Printf("Active: %s (%s) %s\n", cfg.Single.Active, typeName, status)
				fmt.Printf("  └─ %s → 127.0.0.1:%d\n", instance.Domain, instance.Port)
			}
		} else {
			fmt.Println("Active: (none)")
		}

		// Show other instances
		if len(cfg.Transports) > 1 {
			fmt.Println()
			fmt.Println("Other instances:")
			for name, transport := range cfg.Transports {
				if name == cfg.Single.Active {
					continue
				}
				typeName := router.GetTransportTypeDisplayName(transport.Type)
				fmt.Printf("  %-16s %s\n", name, typeName)
			}
		}
	} else {
		svc := r.GetDNSRouterService()
		status := "○ Stopped"
		if svc.IsActive() {
			status = "● Running"
		}
		fmt.Printf("DNS Router: %s (port 53)\n", status)
		fmt.Println()
		fmt.Println("Instances:")

		instances := r.GetAllInstances()
		if len(instances) == 0 {
			fmt.Println("  No instances configured")
		} else {
			for name, instance := range instances {
				instStatus := "○ Stopped"
				if instance.IsActive() {
					instStatus = "● Running"
				}
				typeName := router.GetTransportTypeDisplayName(instance.Type)
				defaultMarker := ""
				if cfg.Routing.Default == name {
					defaultMarker = " (default)"
				}
				fmt.Printf("  %-16s %-20s %s%s\n", name, typeName, instStatus, defaultMarker)
				fmt.Printf("    └─ %s → 127.0.0.1:%d\n", instance.Domain, instance.Port)
			}
		}
	}
}

func runRouterStart() {
	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		return
	}

	r, _ := router.New(cfg)
	fmt.Println()

	isRunning := r.IsRunning()
	if isRunning {
		tui.PrintInfo("Restarting...")
	} else {
		tui.PrintInfo("Starting...")
	}

	if err := r.Restart(); err != nil {
		tui.PrintError("Failed to start: " + err.Error())
		return
	}

	if isRunning {
		tui.PrintSuccess("Restarted!")
	} else {
		tui.PrintSuccess("Started!")
	}
}

func runRouterStop() {
	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		return
	}

	r, _ := router.New(cfg)
	fmt.Println()
	tui.PrintInfo("Stopping...")

	if err := r.Stop(); err != nil {
		tui.PrintError("Failed to stop: " + err.Error())
		return
	}

	tui.PrintSuccess("Stopped!")
}

func runRouterLogs() {
	svc := dnsrouter.NewService()
	logs, err := svc.GetLogs(50)
	if err != nil {
		tui.PrintError("Failed to get logs: " + err.Error())
		return
	}
	fmt.Println()
	fmt.Println(logs)
}

// ============================================================================
// Instance Menu (matches: dnstm instance)
// ============================================================================

func runInstanceMenu() error {
	for {
		fmt.Println()

		options := []tui.MenuOption{
			{Label: "Add", Value: "add"},
			{Label: "List →", Value: "list"},
			{Label: "Back", Value: "back"},
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Instances",
			Options: options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		switch choice {
		case "add":
			runInstanceAdd()
			tui.WaitForEnter()
		case "list":
			if err := runInstanceListMenu(); err != errCancelled {
				tui.WaitForEnter()
			}
		}
	}
}

// runInstanceListMenu shows all instances and allows selecting one to manage.
func runInstanceListMenu() error {
	for {
		cfg, err := router.Load()
		if err != nil {
			tui.PrintError("Failed to load config: " + err.Error())
			return nil
		}

		if len(cfg.Transports) == 0 {
			tui.PrintInfo("No instances configured. Add one first.")
			tui.WaitForEnter()
			return errCancelled
		}

		var options []tui.MenuOption
		for name, transport := range cfg.Transports {
			instance := router.NewInstance(name, transport)
			status := "○"
			if instance.IsActive() {
				status = "●"
			}
			typeName := types.GetTransportTypeDisplayName(transport.Type)
			label := fmt.Sprintf("%s %s (%s)", status, name, typeName)
			options = append(options, tui.MenuOption{Label: label, Value: name})
		}
		options = append(options, tui.MenuOption{Label: "Back", Value: "back"})

		selected, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Select Instance",
			Options: options,
		})
		if err != nil || selected == "" || selected == "back" {
			return errCancelled
		}

		if err := runInstanceManageMenu(selected); err != errCancelled {
			tui.WaitForEnter()
		}
	}
}

// runInstanceManageMenu shows management options for a specific instance.
func runInstanceManageMenu(name string) error {
	for {
		cfg, err := router.Load()
		if err != nil {
			tui.PrintError("Failed to load config: " + err.Error())
			return nil
		}

		transport, exists := cfg.Transports[name]
		if !exists {
			tui.PrintError(fmt.Sprintf("Instance '%s' not found", name))
			return nil
		}

		instance := router.NewInstance(name, transport)
		status := "Stopped"
		if instance.IsActive() {
			status = "Running"
		}

		options := []tui.MenuOption{
			{Label: "Status", Value: "status"},
			{Label: "Logs", Value: "logs"},
			{Label: "Start/Restart", Value: "start"},
			{Label: "Stop", Value: "stop"},
			{Label: "Reconfigure", Value: "reconfigure"},
			{Label: "Remove", Value: "remove"},
			{Label: "Back", Value: "back"},
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:       fmt.Sprintf("%s (%s)", name, status),
			Description: fmt.Sprintf("%s → %s:%d", types.GetTransportTypeDisplayName(transport.Type), transport.Domain, transport.Port),
			Options:     options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		switch choice {
		case "status":
			runInstanceStatusByName(name, transport)
			tui.WaitForEnter()
		case "logs":
			runInstanceLogsByName(name, transport)
			tui.WaitForEnter()
		case "start":
			runInstanceStartByName(name, transport)
			tui.WaitForEnter()
		case "stop":
			runInstanceStopByName(name, transport)
			tui.WaitForEnter()
		case "reconfigure":
			newName, changed := runInstanceReconfigure(name, transport, cfg)
			if changed {
				tui.WaitForEnter()
				if newName != name {
					// Instance was renamed, go back to list
					return errCancelled
				}
			}
		case "remove":
			removed := runInstanceRemoveByName(name, transport, cfg)
			if removed {
				// Check if there are remaining instances
				cfg, _ = router.Load()
				if len(cfg.Transports) > 0 {
					tui.WaitForEnter()
				}
				return errCancelled // Go back to list since instance was removed
			}
			tui.WaitForEnter()
		}
	}
}

func runInstanceListCLI() {
	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		return
	}

	if len(cfg.Transports) == 0 {
		fmt.Println("No instances configured")
		return
	}

	fmt.Println()
	fmt.Printf("%-16s %-24s %-8s %-20s %s\n", "NAME", "TYPE", "PORT", "DOMAIN", "STATUS")
	for name, transport := range cfg.Transports {
		instance := router.NewInstance(name, transport)
		status := "Stopped"
		if instance.IsActive() {
			status = "Running"
		}
		marker := ""
		if cfg.IsSingleMode() && cfg.Single.Active == name {
			marker = " *"
		}
		typeName := types.GetTransportTypeDisplayName(transport.Type)
		fmt.Printf("%-16s %-24s %-8d %-20s %s%s\n", name, typeName, transport.Port, transport.Domain, status, marker)
	}
}

// runInstanceStartByName starts a specific instance by name.
func runInstanceStartByName(name string, transport *types.TransportConfig) {
	instance := router.NewInstance(name, transport)
	instance.Enable()

	isRunning := instance.IsActive()
	if isRunning {
		if err := instance.Restart(); err != nil {
			tui.PrintError("Failed to restart: " + err.Error())
			return
		}
		tui.PrintSuccess(fmt.Sprintf("Instance '%s' restarted!", name))
	} else {
		if err := instance.Start(); err != nil {
			tui.PrintError("Failed to start: " + err.Error())
			return
		}
		tui.PrintSuccess(fmt.Sprintf("Instance '%s' started!", name))
	}
}

// runInstanceStopByName stops a specific instance by name.
func runInstanceStopByName(name string, transport *types.TransportConfig) {
	instance := router.NewInstance(name, transport)
	if err := instance.Stop(); err != nil {
		tui.PrintError("Failed to stop: " + err.Error())
		return
	}
	tui.PrintSuccess(fmt.Sprintf("Instance '%s' stopped!", name))
}

// runInstanceLogsByName shows logs for a specific instance.
func runInstanceLogsByName(name string, transport *types.TransportConfig) {
	instance := router.NewInstance(name, transport)
	logs, err := instance.GetLogs(50)
	if err != nil {
		tui.PrintError("Failed to get logs: " + err.Error())
		return
	}
	fmt.Println()
	fmt.Println(logs)
}

// runInstanceStatusByName shows status for a specific instance.
func runInstanceStatusByName(name string, transport *types.TransportConfig) {
	instance := router.NewInstance(name, transport)
	fmt.Println()
	fmt.Println(instance.GetFormattedInfo())

	// Show certificate or key info
	if types.IsSlipstreamType(transport.Type) {
		certMgr := certs.NewManager()
		if certInfo := certMgr.Get(transport.Domain); certInfo != nil {
			fmt.Println("Certificate Fingerprint:")
			fmt.Println(certs.FormatFingerprint(certInfo.Fingerprint))
		}
	} else if types.IsDNSTTType(transport.Type) {
		keyMgr := keys.NewManager()
		if keyInfo := keyMgr.Get(transport.Domain); keyInfo != nil {
			fmt.Println("Public Key:")
			fmt.Println(keyInfo.PublicKey)
		}
	}
}

// runInstanceRemoveByName removes a specific instance. Returns true if removed.
func runInstanceRemoveByName(name string, transport *types.TransportConfig, cfg *router.Config) bool {
	confirm, _ := tui.RunConfirm(tui.ConfirmConfig{Title: fmt.Sprintf("Remove '%s'?", name)})
	if !confirm {
		return false
	}

	instance := router.NewInstance(name, transport)
	instance.Stop()
	instance.RemoveService()
	instance.RemoveConfigDir()

	delete(cfg.Transports, name)
	if cfg.Single.Active == name {
		cfg.Single.Active = ""
		for n := range cfg.Transports {
			cfg.Single.Active = n
			break
		}
	}
	if cfg.Routing.Default == name {
		cfg.Routing.Default = ""
		for n := range cfg.Transports {
			cfg.Routing.Default = n
			break
		}
	}
	cfg.Save()

	tui.PrintSuccess(fmt.Sprintf("Instance '%s' removed!", name))
	return true
}

// runInstanceReconfigure allows reconfiguring an existing instance.
// Returns (newName, changed) - newName is the instance name after reconfigure (may be renamed),
// changed indicates if any changes were made and WaitForEnter should be called.
func runInstanceReconfigure(name string, transport *types.TransportConfig, cfg *router.Config) (string, bool) {
	fmt.Println()
	tui.PrintInfo(fmt.Sprintf("Reconfiguring '%s'...", name))
	tui.PrintInfo("Press Enter to keep current value, or type a new value.")
	fmt.Println()

	changed := false
	renamed := false
	newName := name

	// Check if instance is running before we start
	oldInstance := router.NewInstance(name, transport)
	wasRunning := oldInstance.IsActive()

	// Name (rename)
	inputName, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Instance Name",
		Description: fmt.Sprintf("Current: %s", name),
		Value:       name,
	})
	if err != nil || !confirmed {
		return "", false
	}
	if inputName != "" && inputName != name {
		inputName = router.NormalizeName(inputName)
		if err := router.ValidateName(inputName); err != nil {
			tui.PrintError("Invalid name: " + err.Error())
			return name, true
		}
		if _, exists := cfg.Transports[inputName]; exists {
			tui.PrintError("Instance with that name already exists")
			return name, true
		}
		newName = inputName
		renamed = true
		changed = true
	}

	// Domain
	newDomain, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Domain",
		Description: fmt.Sprintf("Current: %s", transport.Domain),
		Value:       transport.Domain,
	})
	if err != nil || !confirmed {
		return "", false
	}
	if newDomain != "" && newDomain != transport.Domain {
		transport.Domain = newDomain
		changed = true
	}

	// Type-specific configuration
	switch transport.Type {
	case types.TypeSlipstreamShadowsocks:
		// Password
		currentPassword := ""
		if transport.Shadowsocks != nil {
			currentPassword = transport.Shadowsocks.Password
		}
		newPassword, confirmed, err := tui.RunInput(tui.InputConfig{
			Title:       "Password",
			Description: "Current: (hidden) - Leave empty to keep current",
		})
		if err != nil || !confirmed {
			return "", false
		}
		if newPassword != "" && newPassword != currentPassword {
			if transport.Shadowsocks == nil {
				transport.Shadowsocks = &types.ShadowsocksConfig{}
			}
			transport.Shadowsocks.Password = newPassword
			changed = true
		}

		// Method
		currentMethod := "aes-256-gcm"
		if transport.Shadowsocks != nil && transport.Shadowsocks.Method != "" {
			currentMethod = transport.Shadowsocks.Method
		}
		methodOptions := []tui.MenuOption{
			{Label: "AES-256-GCM", Value: "aes-256-gcm"},
			{Label: "ChaCha20-IETF-Poly1305", Value: "chacha20-ietf-poly1305"},
		}
		// Set current as selected
		selected := 0
		if currentMethod == "chacha20-ietf-poly1305" {
			selected = 1
		}
		newMethod, err := tui.RunMenu(tui.MenuConfig{
			Title:       "Method",
			Description: fmt.Sprintf("Current: %s", currentMethod),
			Options:     methodOptions,
			Selected:    selected,
		})
		if err != nil || newMethod == "" {
			return "", false
		}
		if newMethod != currentMethod {
			if transport.Shadowsocks == nil {
				transport.Shadowsocks = &types.ShadowsocksConfig{}
			}
			transport.Shadowsocks.Method = newMethod
			changed = true
		}

	case types.TypeSlipstreamSocks, types.TypeDNSTTSocks:
		// SOCKS modes auto-use microsocks, update to current config
		microsocksAddr := cfg.GetMicrosocksAddress()
		if microsocksAddr != "" && (transport.Target == nil || transport.Target.Address != microsocksAddr) {
			if transport.Target == nil {
				transport.Target = &types.TargetConfig{}
			}
			transport.Target.Address = microsocksAddr
			changed = true
		}

	case types.TypeSlipstreamSSH, types.TypeDNSTTSSH:
		// SSH modes - allow changing target
		currentTarget := "127.0.0.1:" + osdetect.DetectSSHPort()
		if transport.Target != nil && transport.Target.Address != "" {
			currentTarget = transport.Target.Address
		}
		newTarget, confirmed, err := tui.RunInput(tui.InputConfig{
			Title:       "Target Address",
			Description: fmt.Sprintf("Current: %s", currentTarget),
			Value:       currentTarget,
		})
		if err != nil || !confirmed {
			return "", false
		}
		if newTarget != "" && newTarget != currentTarget {
			if transport.Target == nil {
				transport.Target = &types.TargetConfig{}
			}
			transport.Target.Address = newTarget
			changed = true
		}
	}

	if !changed {
		tui.PrintInfo("No changes made")
		return name, true
	}

	// Handle rename
	if renamed {
		// Stop and remove old service
		oldInstance.Stop()
		oldInstance.RemoveService()
		oldInstance.RemoveConfigDir()

		// Update config map
		delete(cfg.Transports, name)
		cfg.Transports[newName] = transport

		// Update Single.Active if it referenced the old name
		if cfg.Single.Active == name {
			cfg.Single.Active = newName
		}
		// Update Routing.Default if it referenced the old name
		if cfg.Routing.Default == name {
			cfg.Routing.Default = newName
		}
	}

	// Save config
	cfg.Save()

	// Create new service
	newInstance := router.NewInstance(newName, transport)
	if err := newInstance.CreateService(); err != nil {
		tui.PrintError("Failed to create service: " + err.Error())
		return newName, true
	}
	newInstance.SetPermissions()

	// Start if it was running before
	if wasRunning {
		newInstance.Enable()
		if err := newInstance.Start(); err != nil {
			tui.PrintError("Failed to start: " + err.Error())
			return newName, true
		}
		if renamed {
			tui.PrintSuccess(fmt.Sprintf("Instance renamed to '%s' and restarted!", newName))
		} else {
			tui.PrintSuccess(fmt.Sprintf("Instance '%s' reconfigured and restarted!", newName))
		}
	} else {
		if renamed {
			tui.PrintSuccess(fmt.Sprintf("Instance renamed to '%s'!", newName))
		} else {
			tui.PrintSuccess(fmt.Sprintf("Instance '%s' reconfigured!", newName))
		}
	}

	return newName, true
}

func runInstanceAdd() {
	if !router.IsInitialized() {
		if err := router.Initialize(); err != nil {
			tui.PrintError("Failed to initialize router: " + err.Error())
			return
		}
	}

	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		return
	}

	fmt.Println()
	tui.PrintInfo("Adding new instance...")

	// Transport type (ask first)
	transportType, err := tui.RunMenu(tui.MenuConfig{
		Title: "Transport Type",
		Options: []tui.MenuOption{
			{Label: "Slipstream + Shadowsocks (Recommended)", Value: string(types.TypeSlipstreamShadowsocks)},
			{Label: "Slipstream SOCKS", Value: string(types.TypeSlipstreamSocks)},
			{Label: "Slipstream SSH", Value: string(types.TypeSlipstreamSSH)},
			{Label: "Slipstream + MTProxy (Telegram)", Value: string(types.TypeSlipstreamMTProxy)},
			{Label: "DNSTT SOCKS", Value: string(types.TypeDNSTTSocks)},
			{Label: "DNSTT SSH", Value: string(types.TypeDNSTTSSH)},
			{Label: "DNSTT + MTProxy (Telegram, via socat)", Value: string(types.TypeDNSTTMTProxy)},
		},
	})
	if err != nil || transportType == "" {
		return
	}

	if missing, ok := transport.RequiresBinary(types.TransportType(transportType)); !ok {
		tui.PrintError(fmt.Sprintf("Required binary '%s' is not installed", missing))
		return
	}

	// Name
	suggestedName := router.GenerateUniqueName(cfg.Transports)
	name, confirmed, err := tui.RunInput(tui.InputConfig{
		Title:       "Instance Name",
		Description: fmt.Sprintf("Leave empty for: %s", suggestedName),
		Placeholder: suggestedName,
	})
	if err != nil || !confirmed {
		return
	}
	if name == "" {
		name = suggestedName
	}
	name = router.NormalizeName(name)

	if err := router.ValidateName(name); err != nil {
		tui.PrintError("Invalid name: " + err.Error())
		return
	}
	if _, exists := cfg.Transports[name]; exists {
		tui.PrintError("Instance already exists")
		return
	}

<<<<<<< HEAD
=======
	// Transport type
	var transportType string
	huh.NewSelect[string]().
		Title("Transport Type").
		Options(
			huh.NewOption("Slipstream + Shadowsocks (Recommended)", string(types.TypeSlipstreamShadowsocks)),
			huh.NewOption("Slipstream SOCKS", string(types.TypeSlipstreamSocks)),
			huh.NewOption("Slipstream SSH", string(types.TypeSlipstreamSSH)),
			huh.NewOption("Slipstream + MTProxy (Telegram)", string(types.TypeSlipstreamMTProxy)),
			huh.NewOption("DNSTT SOCKS", string(types.TypeDNSTTSocks)),
			huh.NewOption("DNSTT SSH", string(types.TypeDNSTTSSH)),
			huh.NewOption("DNSTT + MTProxy (Telegram, via socat)", string(types.TypeDNSTTMTProxy)),
		).
		Value(&transportType).
		Run()

	if missing, ok := transport.RequiresBinary(types.TransportType(transportType)); !ok {
		tui.PrintError(fmt.Sprintf("Required binary '%s' is not installed", missing))
		return
	}

>>>>>>> 9a0a006ed4332a5eef00f2e07c0f861b9c8405db
	// Domain
	var domain string
	for domain == "" {
		domain, confirmed, err = tui.RunInput(tui.InputConfig{
			Title:       "Domain",
			Description: "e.g., t1.example.com",
		})
		if err != nil || !confirmed {
			return
		}
		if domain == "" {
			tui.PrintError("Domain is required")
		}
	}

	// Build config
	transportCfg := &types.TransportConfig{
		Type:   types.TransportType(transportType),
		Domain: domain,
	}

	switch types.TransportType(transportType) {
	case types.TypeSlipstreamShadowsocks:
		password, confirmed, err := tui.RunInput(tui.InputConfig{
			Title:       "Password",
			Description: "Leave empty to auto-generate",
		})
		if err != nil || !confirmed {
			return
		}
		if password == "" {
			password = generatePassword()
		}
		method, err := tui.RunMenu(tui.MenuConfig{
			Title: "Method",
			Options: []tui.MenuOption{
				{Label: "AES-256-GCM", Value: "aes-256-gcm"},
				{Label: "ChaCha20-IETF-Poly1305", Value: "chacha20-ietf-poly1305"},
			},
		})
		if err != nil || method == "" {
			return
		}
		transportCfg.Shadowsocks = &types.ShadowsocksConfig{Password: password, Method: method}

	case types.TypeSlipstreamSocks:
		// Auto-use configured microsocks
		microsocksAddr := cfg.GetMicrosocksAddress()
		if microsocksAddr == "" {
			tui.PrintError("Microsocks not configured. Run install first.")
			return
		}
		transportCfg.Target = &types.TargetConfig{Address: microsocksAddr}

	case types.TypeSlipstreamSSH:
		defaultAddr := "127.0.0.1:" + osdetect.DetectSSHPort()
		target, confirmed, err := tui.RunInput(tui.InputConfig{
			Title:       "Target Address",
			Placeholder: defaultAddr,
		})
		if err != nil || !confirmed {
			return
		}
		if target == "" {
			target = defaultAddr
		}
		transportCfg.Target = &types.TargetConfig{Address: target}
	case types.TypeSlipstreamMTProxy:
		proxyUrl, err := configureAndInstallMTProxy(transportCfg)
		if err != nil {
			tui.PrintError("Failed to configure MTProxy: " + err.Error())
			return
		}
		// Target is the local MTProxy endpoint - Slipstream supports raw TCP, no bridge needed
		transportCfg.Target = &types.TargetConfig{Address: fmt.Sprintf("%s:%s", mtproxy.MTProxyBindAddr, mtproxy.MTProxyPort)}
		// Show connection URL
		fmt.Println()
		tui.PrintBox("Slipstream + MTProxy (direct)", []string{
			"Slipstream client supports raw TCP tunnel (no bridge needed)",
			"",
			"Client-side Telegram config:",
			"  Type:   MTProto Proxy",
			"  Server: 127.0.0.1 (via slipstream-client)",
			"  Port:   " + mtproxy.MTProxyPort,
			"  Secret: dd<your-secret>",
			"",
			"Or for direct connection (without DNS tunnel):",
			proxyUrl,
		})

<<<<<<< HEAD
	case types.TypeDNSTTSocks:
		// Auto-use configured microsocks
		microsocksAddr := cfg.GetMicrosocksAddress()
		if microsocksAddr == "" {
			tui.PrintError("Microsocks not configured. Run install first.")
			return
		}
		transportCfg.Target = &types.TargetConfig{Address: microsocksAddr}
		transportCfg.DNSTT = &types.DNSTTConfig{MTU: 1232}

	case types.TypeDNSTTSSH:
		defaultAddr := "127.0.0.1:" + osdetect.DetectSSHPort()
		target, confirmed, err := tui.RunInput(tui.InputConfig{
			Title:       "Target Address",
			Placeholder: defaultAddr,
		})
		if err != nil || !confirmed {
			return
=======
	case types.TypeDNSTTSocks, types.TypeDNSTTSSH:
		defaultAddr := "127.0.0.1:1080"
		if transportType == string(types.TypeDNSTTSSH) {
			defaultAddr = "127.0.0.1:" + osdetect.DetectSSHPort()
>>>>>>> 9a0a006ed4332a5eef00f2e07c0f861b9c8405db
		}
		if target == "" {
			target = defaultAddr
		}
		transportCfg.Target = &types.TargetConfig{Address: target}
		transportCfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
	case types.TypeDNSTTMTProxy:
		proxyUrl, err := configureAndInstallMTProxyWithBridge(transportCfg)
		if err != nil {
			tui.PrintError("Failed to configure MTProxy: " + err.Error())
			return
		}
		// Target is the bridge port which forwards to MTProxy
		transportCfg.Target = &types.TargetConfig{Address: fmt.Sprintf("%s:%s", mtproxy.MTProxyBindAddr, mtproxy.MTProxyBridgePort)}
		transportCfg.DNSTT = &types.DNSTTConfig{MTU: 1232}
		// Show connection URL
		fmt.Println()
		tui.PrintBox("DNSTT + MTProxy (via socat bridge)", []string{
			"Server-side: socat bridge (port " + mtproxy.MTProxyBridgePort + ") → MTProxy (port " + mtproxy.MTProxyPort + ")",
			"",
			"Client-side Telegram config:",
			"  Type:   MTProto Proxy",
			"  Server: 127.0.0.1 (via dnstt-client)",
			"  Port:   " + mtproxy.MTProxyBridgePort,
			"  Secret: dd<your-secret>",
			"",
			"Or for direct connection (without DNS tunnel):",
			proxyUrl,
		})
	}

	// Port
	if cfg.Transports == nil {
		cfg.Transports = make(map[string]*types.TransportConfig)
	}
	port, _ := router.AllocatePort(cfg.Transports)
	transportCfg.Port = port

	// Create
	fmt.Println()

	var fingerprint, publicKey string
	if types.IsSlipstreamType(transportCfg.Type) {
		certMgr := certs.NewManager()
		certInfo, _ := certMgr.GetOrCreate(transportCfg.Domain)
		fingerprint = certInfo.Fingerprint
	} else if types.IsDNSTTType(transportCfg.Type) {
		keyMgr := keys.NewManager()
		keyInfo, _ := keyMgr.GetOrCreate(transportCfg.Domain)
		publicKey = keyInfo.PublicKey
	}

	instance := router.NewInstance(name, transportCfg)
	if err := instance.CreateService(); err != nil {
		tui.PrintError("Failed to create service: " + err.Error())
		return
	}
	instance.SetPermissions()

	cfg.Transports[name] = transportCfg
	if cfg.IsSingleMode() && cfg.Single.Active == "" {
		cfg.Single.Active = name
	}
	if cfg.IsMultiMode() && cfg.Routing.Default == "" {
		cfg.Routing.Default = name
	}
	cfg.Save()

	tui.PrintSuccess(fmt.Sprintf("Instance '%s' created!", name))

	// Start the instance
	instance.Enable()
	if err := instance.Start(); err != nil {
		tui.PrintWarning("Failed to start instance: " + err.Error())
	} else {
		tui.PrintSuccess(fmt.Sprintf("Instance '%s' started!", name))
	}

	if fingerprint != "" {
		fmt.Println("\nCertificate Fingerprint:")
		fmt.Println(certs.FormatFingerprint(fingerprint))
	}
	if publicKey != "" {
		fmt.Println("\nPublic Key:")
		fmt.Println(publicKey)
	}
}

// ============================================================================
// Mode Menu (matches: dnstm mode)
// ============================================================================

func runModeMenu() error {
	if !router.IsInitialized() {
		tui.PrintInfo("Router not initialized. Initialize it first via Router → Init")
		return nil // needs WaitForEnter
	}

	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		return nil // needs WaitForEnter
	}

	fmt.Println()
	fmt.Printf("Current mode: %s\n\n", router.GetModeDisplayName(cfg.Mode))

	options := []tui.MenuOption{
		{Label: "Single-tunnel Mode", Value: "single"},
		{Label: "Multi-tunnel Mode (Experimental)", Value: "multi"},
		{Label: "Back", Value: "back"},
	}

	choice, _ := tui.RunMenu(tui.MenuConfig{
		Title:   "Select Mode",
		Options: options,
	})

	if choice == "" || choice == "back" {
		return errCancelled // no WaitForEnter needed
	}

	newMode := router.Mode(choice)
	if cfg.Mode == newMode {
		tui.PrintInfo(fmt.Sprintf("Already in %s mode", router.GetModeDisplayName(newMode)))
		return nil // needs WaitForEnter
	}

	r, _ := router.New(cfg)
	fmt.Println()
	tui.PrintInfo(fmt.Sprintf("Switching to %s mode...", router.GetModeDisplayName(newMode)))

	if err := r.SwitchMode(newMode); err != nil {
		tui.PrintError("Failed to switch mode: " + err.Error())
		return nil // needs WaitForEnter
	}

	tui.PrintSuccess(fmt.Sprintf("Switched to %s mode!", router.GetModeDisplayName(newMode)))
	return nil // needs WaitForEnter
}

// ============================================================================
// Switch Menu (matches: dnstm switch)
// ============================================================================

func runSwitchMenu() error {
	if !router.IsInitialized() {
		tui.PrintInfo("Router not initialized")
		return nil // needs WaitForEnter
	}

	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		return nil // needs WaitForEnter
	}

	if !cfg.IsSingleMode() {
		tui.PrintInfo("Switch is only available in single-tunnel mode")
		return nil // needs WaitForEnter
	}

	if len(cfg.Transports) == 0 {
		tui.PrintInfo("No instances configured")
		return nil // needs WaitForEnter
	}

	fmt.Println()
	if cfg.Single.Active != "" {
		fmt.Printf("Current active: %s\n\n", cfg.Single.Active)
	}

	var options []tui.MenuOption
	for name, transport := range cfg.Transports {
		typeName := types.GetTransportTypeDisplayName(transport.Type)
		label := fmt.Sprintf("%s (%s)", name, typeName)
		if name == cfg.Single.Active {
			label += " [current]"
		}
		options = append(options, tui.MenuOption{Label: label, Value: name})
	}
	options = append(options, tui.MenuOption{Label: "Back", Value: "back"})

	selected, _ := tui.RunMenu(tui.MenuConfig{
		Title:   "Select Instance",
		Options: options,
	})

	if selected == "" || selected == "back" || selected == cfg.Single.Active {
		return errCancelled // no WaitForEnter needed
	}

	r, _ := router.New(cfg)
	fmt.Println()
	tui.PrintInfo(fmt.Sprintf("Switching to '%s'...", selected))

	if err := r.SwitchActiveInstance(selected); err != nil {
		tui.PrintError("Failed to switch: " + err.Error())
		return nil // needs WaitForEnter
	}

	tui.PrintSuccess(fmt.Sprintf("Switched to '%s'!", selected))
	return nil // needs WaitForEnter
}

// ============================================================================
// Install (matches: dnstm install)
// ============================================================================

func runInstallBinaries() error {
	fmt.Println()
	tui.PrintInfo("Installing dnstm...")
	fmt.Println()

	// Step 1: Create dnstm user
	tui.PrintInfo("Creating dnstm user...")
	if err := system.CreateDnstmUser(); err != nil {
		tui.PrintError("Failed to create user: " + err.Error())
		tui.WaitForEnter()
		return errCancelled
	}
	tui.PrintStatus("dnstm user ready")

	// Step 2: Initialize router (create directories)
	tui.PrintInfo("Initializing router...")
	if err := router.Initialize(); err != nil {
		tui.PrintError("Failed to initialize router: " + err.Error())
		tui.WaitForEnter()
		return errCancelled
	}
	tui.PrintStatus("Router initialized")

	// Step 3: Select operating mode
	modeStr, err := tui.RunMenu(tui.MenuConfig{
		Title: "Operating Mode",
		Options: []tui.MenuOption{
			{Label: "Single-tunnel (Recommended)", Value: "single"},
			{Label: "Multi-tunnel (Experimental)", Value: "multi"},
		},
	})
	if err != nil || modeStr == "" {
		tui.PrintInfo("Cancelled")
		tui.WaitForEnter()
		return errCancelled
	}

	cfg, _ := router.Load()
	if cfg != nil {
		cfg.Mode = router.Mode(modeStr)
		cfg.Save()
	}
	tui.PrintStatus(fmt.Sprintf("Mode set to %s", router.GetModeDisplayName(router.Mode(modeStr))))

	// Step 4: Create DNS router service
	svc := dnsrouter.NewService()
	if err := svc.CreateService(); err != nil {
		tui.PrintWarning("DNS router service: " + err.Error())
	} else {
		tui.PrintStatus("DNS router service created")
	}

	// Step 5: Install transport binaries
	fmt.Println()
	tui.PrintInfo("Installing transport binaries...")
	fmt.Println()

	if err := transport.EnsureDnsttInstalled(); err != nil {
		tui.PrintError("Failed to install dnstt-server: " + err.Error())
	}

	if err := transport.EnsureSlipstreamInstalled(); err != nil {
		tui.PrintError("Failed to install slipstream-server: " + err.Error())
	}

	if err := transport.EnsureShadowsocksInstalled(); err != nil {
		tui.PrintError("Failed to install ssserver: " + err.Error())
	}

	if err := transport.EnsureSSHTunUserInstalled(); err != nil {
		tui.PrintWarning("sshtun-user: " + err.Error())
	}

	tui.PrintInfo("Installing microsocks...")
	if err := installMicrosocks(); err != nil {
		tui.PrintError("Failed to install microsocks: " + err.Error())
	} else {
		// Configure and start microsocks service
		if !proxy.IsMicrosocksRunning() {
			// Find available port and save to config
			port, err := proxy.FindAvailablePort()
			if err != nil {
				tui.PrintWarning("Could not find available port: " + err.Error())
			} else {
				cfg, _ = router.Load()
				if cfg != nil {
					cfg.Proxy.Port = port
					cfg.Save()
				}
				if err := proxy.ConfigureMicrosocks(port); err != nil {
					tui.PrintWarning("microsocks service config: " + err.Error())
				} else {
					if err := proxy.StartMicrosocks(); err != nil {
						tui.PrintWarning("microsocks service start: " + err.Error())
					} else {
						tui.PrintStatus(fmt.Sprintf("microsocks installed and running on port %d", port))
					}
				}
			}
		} else {
			tui.PrintStatus("microsocks already running")
		}
	}

	// Step 6: Configure firewall
	fmt.Println()
	tui.PrintInfo("Configuring firewall...")
	network.ClearNATOnly()
	if err := network.AllowPort53(); err != nil {
		tui.PrintWarning("Firewall configuration: " + err.Error())
	} else {
		tui.PrintStatus("Firewall configured (port 53 UDP/TCP)")
	}

	fmt.Println()
	tui.PrintSuccess("Installation complete!")
	fmt.Println()
	tui.PrintInfo("Next steps:")
	fmt.Println("  1. Add instance: Instance → Add")
	fmt.Println("  2. Start router: Router → Start")
	fmt.Println()

	tui.WaitForEnter()
	return errCancelled
}

func installMicrosocks() error {
	if isMicrosocksInstalled() {
		tui.PrintStatus("microsocks already installed")
		return nil
	}
	return doInstallMicrosocks()
}

func isMicrosocksInstalled() bool {
	_, err := os.Stat("/usr/local/bin/microsocks")
	return err == nil
}

func doInstallMicrosocks() error {
	if err := proxy.InstallMicrosocks(nil); err != nil {
		return err
	}
	tui.PrintStatus("microsocks installed")
	return nil
}

// ============================================================================
// Uninstall Menu (matches: dnstm uninstall)
// ============================================================================

func runUninstallMenu() error {
	fmt.Println()
	tui.PrintWarning("This will remove all dnstm components:")
	fmt.Println("  - All instance services")
	fmt.Println("  - DNS router and microsocks services")
	fmt.Println("  - Configuration files")
	fmt.Println("  - Transport binaries")
	fmt.Println()
	tui.PrintInfo("Note: The dnstm binary will be kept for easy reinstallation.")
	fmt.Println()

	confirm, _ := tui.RunConfirm(tui.ConfirmConfig{Title: "Proceed with uninstall?"})
	if !confirm {
		tui.PrintInfo("Cancelled")
		tui.WaitForEnter()
		return errCancelled
	}

	installer.PerformFullUninstall()

	tui.WaitForEnter()
	os.Exit(0)
	return nil
}

// ============================================================================
// Helpers
// ============================================================================

func generatePassword() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.StdEncoding.EncodeToString(bytes)
}

func configureAndInstallMTProxy(cfg *types.TransportConfig) (string, error) {
	secret, err := mtproxy.GenerateSecret()
	if err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}

	tui.PrintStatus(fmt.Sprintf("Using MTProxy secret: %s", secret))

	progressFn := func(downloaded, total int64) {
		if total > 0 {
			percent := float64(downloaded) / float64(total) * 100
			fmt.Printf("\rDownloading: %.1f%%", percent)
		}
	}

	if err := mtproxy.InstallMTProxy(progressFn); err != nil {
		return "", fmt.Errorf("failed to install MTProxy: %w", err)
	}

	if err := mtproxy.ConfigureMTProxy(secret); err != nil {
		return "", fmt.Errorf("failed to configure MTProxy: %w", err)
	}

	domain := "your-domain.com"
	if cfg != nil && cfg.Domain != "" {
		domain = cfg.Domain
	}

	proxyUrl := mtproxy.FormatProxyURL(secret, domain)
	return proxyUrl, nil
}

// configureAndInstallMTProxyWithBridge installs MTProxy and the socat bridge for DNSTT
func configureAndInstallMTProxyWithBridge(cfg *types.TransportConfig) (string, error) {
	// First install MTProxy
	proxyUrl, err := configureAndInstallMTProxy(cfg)
	if err != nil {
		return "", err
	}

	// Then install the socat bridge
	tui.PrintStatus("Installing socat bridge for DNSTT...")
	if err := mtproxy.InstallBridge(); err != nil {
		return "", fmt.Errorf("failed to install MTProxy bridge: %w", err)
	}

	return proxyUrl, nil
}
