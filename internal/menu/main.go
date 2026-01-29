// Package menu provides the interactive menu for dnstm.
package menu

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/net2share/dnstm/internal/certs"
	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/mtproxy"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/sshtunnel"
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

		var options []huh.Option[string]

		if !installed {
			// Not installed - show install option first and limited menu
			missing := transport.GetMissingBinaries()
			tui.PrintWarning("dnstm not installed")
			fmt.Printf("Missing: %v\n\n", missing)

			options = append(options, huh.NewOption("Install (Required)", "install"))
			options = append(options, huh.NewOption("Exit", "exit"))
		} else {
			// Fully installed - show all options
			options = append(options, huh.NewOption("Router →", "router"))
			options = append(options, huh.NewOption("Instance →", "instance"))
			options = append(options, huh.NewOption("Mode →", "mode"))
			options = append(options, huh.NewOption("Switch Active", "switch"))
			options = append(options, huh.NewOption("Install", "install"))
			options = append(options, huh.NewOption("SSH Users →", "ssh-users"))
			options = append(options, huh.NewOption("SOCKS Proxy →", "socks"))
			options = append(options, huh.NewOption("MTProxy →", "mtproxy"))
			options = append(options, huh.NewOption("Uninstall", "uninstall"))
			options = append(options, huh.NewOption("Exit", "exit"))
		}

		var choice string
		err := huh.NewSelect[string]().
			Title("DNS Tunnel Manager").
			Options(options...).
			Value(&choice).
			Run()

		if err != nil {
			return err
		}

		if choice == "exit" {
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
	case "mode":
		return runModeMenu()
	case "switch":
		return runSwitchMenu()
	case "ssh-users":
		sshtunnel.ShowMenu()
		return errCancelled
	case "install":
		return runInstallBinaries()
	case "socks":
		RunSOCKSProxyMenu()
		return errCancelled
	case "mtproxy":
		RunMTProxyMenu()
		return errCancelled
	case "uninstall":
		return runUninstallMenu()
	}
	return nil
}

// ============================================================================
// Router Menu (matches: dnstm router)
// ============================================================================

func runRouterMenu() error {
	for {
		fmt.Println()

		var options []huh.Option[string]
		if !router.IsInitialized() {
			options = append(options, huh.NewOption("Init", "init"))
		} else {
			options = append(options, huh.NewOption("Status", "status"))
			options = append(options, huh.NewOption("Start", "start"))
			options = append(options, huh.NewOption("Stop", "stop"))
			options = append(options, huh.NewOption("Restart", "restart"))
			options = append(options, huh.NewOption("Logs", "logs"))
			options = append(options, huh.NewOption("Config", "config"))
			options = append(options, huh.NewOption("Reset", "reset"))
		}
		options = append(options, huh.NewOption("Back", "back"))

		var choice string
		err := huh.NewSelect[string]().
			Title("Router").
			Options(options...).
			Value(&choice).
			Run()

		if err != nil || choice == "back" {
			return errCancelled
		}

		switch choice {
		case "init":
			runRouterInit()
		case "status":
			runRouterStatus()
		case "start":
			runRouterStart()
		case "stop":
			runRouterStop()
		case "restart":
			runRouterRestart()
		case "logs":
			runRouterLogs()
		case "config":
			runRouterConfig()
		case "reset":
			runRouterReset()
		}
		tui.WaitForEnter()
	}
}

func runRouterInit() {
	var modeStr string
	err := huh.NewSelect[string]().
		Title("Operating Mode").
		Options(
			huh.NewOption("Single-tunnel (Recommended)", "single"),
			huh.NewOption("Multi-tunnel (Experimental)", "multi"),
		).
		Value(&modeStr).
		Run()
	if err != nil {
		return
	}

	fmt.Println()
	tui.PrintInfo("Initializing router...")

	if err := router.Initialize(); err != nil {
		tui.PrintError("Failed to initialize: " + err.Error())
		return
	}

	if err := system.CreateDnstmUser(); err != nil {
		tui.PrintError("Failed to create user: " + err.Error())
		return
	}

	cfg, _ := router.Load()
	cfg.Mode = router.Mode(modeStr)
	cfg.Save()

	svc := dnsrouter.NewService()
	svc.CreateService()

	tui.PrintSuccess("Router initialized!")
}

func runRouterStatus() {
	if !router.IsInitialized() {
		tui.PrintInfo("Router not initialized. Run 'Init' first.")
		return
	}

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
				status := "Stopped"
				if instance.IsActive() {
					status = "Running"
				}
				fmt.Printf("Active: %s (%s)\n", cfg.Single.Active, status)
			}
		} else {
			fmt.Println("Active: (none)")
		}
	} else {
		svc := r.GetDNSRouterService()
		status := "Stopped"
		if svc.IsActive() {
			status = "Running"
		}
		fmt.Printf("DNS Router: %s\n", status)
	}

	fmt.Printf("Instances: %d\n", len(cfg.Transports))
}

func runRouterStart() {
	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		return
	}

	r, _ := router.New(cfg)
	fmt.Println()
	tui.PrintInfo("Starting...")

	if err := r.Start(); err != nil {
		tui.PrintError("Failed to start: " + err.Error())
		return
	}

	tui.PrintSuccess("Started!")
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

func runRouterRestart() {
	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		return
	}

	r, _ := router.New(cfg)
	fmt.Println()
	tui.PrintInfo("Restarting...")

	if err := r.Restart(); err != nil {
		tui.PrintError("Failed to restart: " + err.Error())
		return
	}

	tui.PrintSuccess("Restarted!")
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

func runRouterConfig() {
	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		return
	}

	fmt.Println()
	fmt.Printf("Mode: %s\n", router.GetModeDisplayName(cfg.Mode))
	fmt.Printf("Listen: %s\n", cfg.Listen.Address)
	if cfg.IsSingleMode() {
		fmt.Printf("Active: %s\n", cfg.Single.Active)
	}
	if cfg.Routing.Default != "" {
		fmt.Printf("Default Route: %s\n", cfg.Routing.Default)
	}
}

func runRouterReset() {
	fmt.Println()
	tui.PrintWarning("This will reset the router to initial state:")
	fmt.Println("  - Stop and remove all instance services")
	fmt.Println("  - Remove DNS router service")
	fmt.Println("  - Reset configuration")

	var confirm bool
	huh.NewConfirm().Title("Proceed?").Value(&confirm).Run()
	if !confirm {
		tui.PrintInfo("Cancelled")
		return
	}

	cfg, _ := router.Load()
	if cfg != nil {
		for name, transport := range cfg.Transports {
			instance := router.NewInstance(name, transport)
			instance.Stop()
			instance.RemoveService()
		}
	}

	svc := dnsrouter.NewService()
	svc.Stop()
	svc.Remove()

	network.ClearNATOnly()
	network.RemoveAllFirewallRules()

	defaultCfg := router.Default()
	defaultCfg.Save()

	tui.PrintSuccess("Router reset!")
}

// ============================================================================
// Instance Menu (matches: dnstm instance)
// ============================================================================

func runInstanceMenu() error {
	for {
		fmt.Println()

		var options []huh.Option[string]
		options = append(options, huh.NewOption("List", "list"))
		options = append(options, huh.NewOption("Add", "add"))
		options = append(options, huh.NewOption("Remove", "remove"))
		options = append(options, huh.NewOption("Start", "start"))
		options = append(options, huh.NewOption("Stop", "stop"))
		options = append(options, huh.NewOption("Restart", "restart"))
		options = append(options, huh.NewOption("Logs", "logs"))
		options = append(options, huh.NewOption("Config", "config"))
		options = append(options, huh.NewOption("Status", "status"))
		options = append(options, huh.NewOption("Back", "back"))

		var choice string
		err := huh.NewSelect[string]().
			Title("Instance").
			Options(options...).
			Value(&choice).
			Run()

		if err != nil || choice == "back" {
			return errCancelled
		}

		switch choice {
		case "list":
			runInstanceList()
		case "add":
			runInstanceAdd()
		case "remove":
			runInstanceRemove()
		case "start":
			runInstanceStart()
		case "stop":
			runInstanceStop()
		case "restart":
			runInstanceRestart()
		case "logs":
			runInstanceLogs()
		case "config":
			runInstanceConfig()
		case "status":
			runInstanceStatus()
		}
		tui.WaitForEnter()
	}
}

func runInstanceList() {
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

	// Name
	suggestedName := router.GenerateUniqueName(cfg.Transports)
	var name string
	huh.NewInput().
		Title("Instance Name").
		Description(fmt.Sprintf("Leave empty for: %s", suggestedName)).
		Placeholder(suggestedName).
		Value(&name).
		Run()
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

	// Domain
	var domain string
	for domain == "" {
		huh.NewInput().Title("Domain").Description("e.g., t1.example.com").Value(&domain).Run()
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
		var password string
		huh.NewInput().Title("Password").Description("Leave empty to auto-generate").Value(&password).Run()
		if password == "" {
			password = generatePassword()
		}
		var method string
		huh.NewSelect[string]().Title("Method").Options(
			huh.NewOption("AES-256-GCM", "aes-256-gcm"),
			huh.NewOption("ChaCha20-IETF-Poly1305", "chacha20-ietf-poly1305"),
		).Value(&method).Run()
		transportCfg.Shadowsocks = &types.ShadowsocksConfig{Password: password, Method: method}

	case types.TypeSlipstreamSocks, types.TypeSlipstreamSSH:
		defaultAddr := "127.0.0.1:1080"
		if transportType == string(types.TypeSlipstreamSSH) {
			defaultAddr = "127.0.0.1:" + osdetect.DetectSSHPort()
		}
		var target string
		huh.NewInput().Title("Target Address").Placeholder(defaultAddr).Value(&target).Run()
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

	case types.TypeDNSTTSocks, types.TypeDNSTTSSH:
		defaultAddr := "127.0.0.1:1080"
		if transportType == string(types.TypeDNSTTSSH) {
			defaultAddr = "127.0.0.1:" + osdetect.DetectSSHPort()
		}
		var target string
		huh.NewInput().Title("Target Address").Placeholder(defaultAddr).Value(&target).Run()
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

	// Confirm
	fmt.Println()
	fmt.Printf("Name:   %s\n", name)
	fmt.Printf("Type:   %s\n", types.GetTransportTypeDisplayName(transportCfg.Type))
	fmt.Printf("Domain: %s\n", transportCfg.Domain)
	fmt.Printf("Port:   %d\n", transportCfg.Port)

	var confirm bool
	huh.NewConfirm().Title("Create instance?").Value(&confirm).Run()
	if !confirm {
		tui.PrintInfo("Cancelled")
		return
	}

	// Create
	fmt.Println()
	system.CreateDnstmUser()

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
	if fingerprint != "" {
		fmt.Println("\nCertificate Fingerprint:")
		fmt.Println(certs.FormatFingerprint(fingerprint))
	}
	if publicKey != "" {
		fmt.Println("\nPublic Key:")
		fmt.Println(publicKey)
	}
}

func selectInstance(title string) (string, *types.TransportConfig, *router.Config) {
	cfg, err := router.Load()
	if err != nil || len(cfg.Transports) == 0 {
		tui.PrintInfo("No instances configured")
		return "", nil, nil
	}

	var options []huh.Option[string]
	for name := range cfg.Transports {
		options = append(options, huh.NewOption(name, name))
	}

	var selected string
	huh.NewSelect[string]().Title(title).Options(options...).Value(&selected).Run()
	if selected == "" {
		return "", nil, nil
	}

	return selected, cfg.Transports[selected], cfg
}

func runInstanceRemove() {
	name, transport, cfg := selectInstance("Select instance to remove")
	if name == "" {
		return
	}

	var confirm bool
	huh.NewConfirm().Title(fmt.Sprintf("Remove '%s'?", name)).Value(&confirm).Run()
	if !confirm {
		return
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
}

func runInstanceStart() {
	name, transport, _ := selectInstance("Select instance to start")
	if name == "" {
		return
	}

	instance := router.NewInstance(name, transport)
	instance.Enable()
	if err := instance.Start(); err != nil {
		tui.PrintError("Failed to start: " + err.Error())
		return
	}
	tui.PrintSuccess(fmt.Sprintf("Instance '%s' started!", name))
}

func runInstanceStop() {
	name, transport, _ := selectInstance("Select instance to stop")
	if name == "" {
		return
	}

	instance := router.NewInstance(name, transport)
	if err := instance.Stop(); err != nil {
		tui.PrintError("Failed to stop: " + err.Error())
		return
	}
	tui.PrintSuccess(fmt.Sprintf("Instance '%s' stopped!", name))
}

func runInstanceRestart() {
	name, transport, _ := selectInstance("Select instance to restart")
	if name == "" {
		return
	}

	instance := router.NewInstance(name, transport)
	if err := instance.Restart(); err != nil {
		tui.PrintError("Failed to restart: " + err.Error())
		return
	}
	tui.PrintSuccess(fmt.Sprintf("Instance '%s' restarted!", name))
}

func runInstanceLogs() {
	name, transport, _ := selectInstance("Select instance")
	if name == "" {
		return
	}

	instance := router.NewInstance(name, transport)
	logs, err := instance.GetLogs(50)
	if err != nil {
		tui.PrintError("Failed to get logs: " + err.Error())
		return
	}
	fmt.Println()
	fmt.Println(logs)
}

func runInstanceConfig() {
	name, transport, _ := selectInstance("Select instance")
	if name == "" {
		return
	}

	instance := router.NewInstance(name, transport)
	fmt.Println()
	fmt.Println(instance.GetFormattedInfo())

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

func runInstanceStatus() {
	name, transport, _ := selectInstance("Select instance")
	if name == "" {
		return
	}

	instance := router.NewInstance(name, transport)
	status, err := instance.GetStatus()
	if err != nil {
		tui.PrintError("Failed to get status: " + err.Error())
		return
	}
	fmt.Println()
	fmt.Println(status)
}

// ============================================================================
// Mode Menu (matches: dnstm mode)
// ============================================================================

func runModeMenu() error {
	if !router.IsInitialized() {
		tui.PrintInfo("Router not initialized. Initialize it first via Router → Init")
		tui.WaitForEnter()
		return errCancelled
	}

	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		tui.WaitForEnter()
		return errCancelled
	}

	fmt.Println()
	fmt.Printf("Current mode: %s\n\n", router.GetModeDisplayName(cfg.Mode))

	var options []huh.Option[string]
	options = append(options, huh.NewOption("Single-tunnel Mode", "single"))
	options = append(options, huh.NewOption("Multi-tunnel Mode (Experimental)", "multi"))
	options = append(options, huh.NewOption("Back", "back"))

	var choice string
	huh.NewSelect[string]().Title("Select Mode").Options(options...).Value(&choice).Run()

	if choice == "back" {
		return errCancelled
	}

	newMode := router.Mode(choice)
	if cfg.Mode == newMode {
		tui.PrintInfo(fmt.Sprintf("Already in %s mode", router.GetModeDisplayName(newMode)))
		tui.WaitForEnter()
		return errCancelled
	}

	r, _ := router.New(cfg)
	fmt.Println()
	tui.PrintInfo(fmt.Sprintf("Switching to %s mode...", router.GetModeDisplayName(newMode)))

	if err := r.SwitchMode(newMode); err != nil {
		tui.PrintError("Failed to switch mode: " + err.Error())
		tui.WaitForEnter()
		return errCancelled
	}

	tui.PrintSuccess(fmt.Sprintf("Switched to %s mode!", router.GetModeDisplayName(newMode)))
	tui.WaitForEnter()
	return errCancelled
}

// ============================================================================
// Switch Menu (matches: dnstm switch)
// ============================================================================

func runSwitchMenu() error {
	if !router.IsInitialized() {
		tui.PrintInfo("Router not initialized")
		tui.WaitForEnter()
		return errCancelled
	}

	cfg, err := router.Load()
	if err != nil {
		tui.PrintError("Failed to load config: " + err.Error())
		tui.WaitForEnter()
		return errCancelled
	}

	if !cfg.IsSingleMode() {
		tui.PrintInfo("Switch is only available in single-tunnel mode")
		tui.WaitForEnter()
		return errCancelled
	}

	if len(cfg.Transports) == 0 {
		tui.PrintInfo("No instances configured")
		tui.WaitForEnter()
		return errCancelled
	}

	fmt.Println()
	if cfg.Single.Active != "" {
		fmt.Printf("Current active: %s\n\n", cfg.Single.Active)
	}

	var options []huh.Option[string]
	for name, transport := range cfg.Transports {
		typeName := types.GetTransportTypeDisplayName(transport.Type)
		label := fmt.Sprintf("%s (%s)", name, typeName)
		if name == cfg.Single.Active {
			label += " [current]"
		}
		options = append(options, huh.NewOption(label, name))
	}
	options = append(options, huh.NewOption("Back", "back"))

	var selected string
	huh.NewSelect[string]().Title("Select Instance").Options(options...).Value(&selected).Run()

	if selected == "back" || selected == cfg.Single.Active {
		return errCancelled
	}

	r, _ := router.New(cfg)
	fmt.Println()
	tui.PrintInfo(fmt.Sprintf("Switching to '%s'...", selected))

	if err := r.SwitchActiveInstance(selected); err != nil {
		tui.PrintError("Failed to switch: " + err.Error())
		tui.WaitForEnter()
		return errCancelled
	}

	tui.PrintSuccess(fmt.Sprintf("Switched to '%s'!", selected))
	tui.WaitForEnter()
	return errCancelled
}

// ============================================================================
// Install (matches: dnstm install)
// ============================================================================

func runInstallBinaries() error {
	fmt.Println()
	tui.PrintInfo("Installing transport binaries...")
	fmt.Println()

	// Install all binaries
	tui.PrintInfo("Installing dnstt-server...")
	if err := transport.EnsureBinariesInstalled(types.TypeDNSTTSocks); err != nil {
		tui.PrintError("Failed to install dnstt-server: " + err.Error())
	}

	tui.PrintInfo("Installing slipstream-server...")
	if err := transport.EnsureBinariesInstalled(types.TypeSlipstreamSocks); err != nil {
		tui.PrintError("Failed to install slipstream-server: " + err.Error())
	}

	tui.PrintInfo("Installing ssserver (shadowsocks)...")
	if err := transport.EnsureBinariesInstalled(types.TypeSlipstreamShadowsocks); err != nil {
		tui.PrintError("Failed to install ssserver: " + err.Error())
	}

	tui.PrintInfo("Installing microsocks...")
	if err := installMicrosocks(); err != nil {
		tui.PrintError("Failed to install microsocks: " + err.Error())
	}

	fmt.Println()
	tui.PrintSuccess("Binary installation complete!")
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

	var confirm bool
	huh.NewConfirm().Title("Proceed with uninstall?").Value(&confirm).Run()
	if !confirm {
		tui.PrintInfo("Cancelled")
		tui.WaitForEnter()
		return errCancelled
	}

	// Run uninstall
	fmt.Println()

	// Stop all services
	tui.PrintInfo("Stopping all services...")
	cfg, _ := router.Load()
	if cfg != nil {
		for name, t := range cfg.Transports {
			instance := router.NewInstance(name, t)
			if instance.IsActive() {
				instance.Stop()
			}
		}
	}
	svc := dnsrouter.NewService()
	if svc.IsActive() {
		svc.Stop()
	}
	if proxy.IsMicrosocksRunning() {
		proxy.StopMicrosocks()
	}
	tui.PrintStatus("Services stopped")

	// Remove instance services
	tui.PrintInfo("Removing instance services...")
	if cfg != nil {
		for name, t := range cfg.Transports {
			instance := router.NewInstance(name, t)
			instance.RemoveService()
		}
	}
	tui.PrintStatus("Instance services removed")

	// Remove DNS router service
	tui.PrintInfo("Removing DNS router service...")
	svc.Remove()
	tui.PrintStatus("DNS router service removed")

	// Remove config directory
	tui.PrintInfo("Removing configuration...")
	os.RemoveAll("/etc/dnstm")
	tui.PrintStatus("Configuration removed")

	// Remove dnstm user
	tui.PrintInfo("Removing dnstm user...")
	system.RemoveDnstmUser()
	tui.PrintStatus("User removed")

	// Remove microsocks
	tui.PrintInfo("Removing microsocks...")
	proxy.UninstallMicrosocks()
	tui.PrintStatus("Microsocks removed")

	// Remove transport binaries
	tui.PrintInfo("Removing transport binaries...")
	binaries := []string{
		"/usr/local/bin/dnstt-server",
		"/usr/local/bin/slipstream-server",
		"/usr/local/bin/ssserver",
	}
	for _, bin := range binaries {
		os.Remove(bin)
	}
	tui.PrintStatus("Binaries removed")

	// Remove firewall rules
	tui.PrintInfo("Removing firewall rules...")
	network.ClearNATOnly()
	network.RemoveAllFirewallRules()
	tui.PrintStatus("Firewall rules removed")

	fmt.Println()
	tui.PrintSuccess("Uninstallation complete!")
	tui.PrintInfo("All dnstm components have been removed.")
	fmt.Println()
	tui.PrintInfo("Note: The dnstm binary is still available for reinstallation.")
	fmt.Println("      To fully remove: rm /usr/local/bin/dnstm")

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

	if err := mtproxy.InstallMTProxy(secret, progressFn); err != nil {
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
