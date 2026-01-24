package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/download"
	"github.com/net2share/dnstm/internal/keys"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/service"
	"github.com/net2share/dnstm/internal/sshtunnel"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

var version string
var buildTime string

// Command represents the CLI command to run.
type Command string

const (
	CommandNone      Command = ""
	CommandInstall   Command = "install"
	CommandStatus    Command = "status"
	CommandLogs      Command = "logs"
	CommandConfig    Command = "config"
	CommandRestart   Command = "restart"
	CommandUninstall Command = "uninstall"
	CommandSSHUsers  Command = "ssh-users"
)

// Options holds the parsed command-line options.
type Options struct {
	Command     Command
	ShowHelp    bool
	ShowVersion bool
	// Install options
	NSSubdomain string
	MTU         string
	TunnelMode  string
	TargetPort  string
	// Uninstall options
	RemoveSSHUsers *bool // nil = not specified (error in CLI mode), true/false = specified
}

// ArchInfo contains architecture information for dnstt downloads.
type ArchInfo struct {
	Arch      string
	DnsttArch string
}

// detectArch returns architecture info with dnstt-specific naming.
func detectArch() *ArchInfo {
	arch := osdetect.GetArch()
	dnsttArch := arch

	switch arch {
	case "amd64":
		dnsttArch = "linux-amd64"
	case "arm64":
		dnsttArch = "linux-arm64"
	case "arm":
		dnsttArch = "linux-armv7"
	case "386":
		dnsttArch = "linux-386"
	}

	return &ArchInfo{
		Arch:      arch,
		DnsttArch: dnsttArch,
	}
}

// printBanner displays the application banner.
func printBanner() {
	titleColor := color.New(color.FgCyan, color.Bold)
	banner := `
    ____  _   _______  ________  ___
   / __ \/ | / / ___/ /_  __/  |/  /
  / / / /  |/ /\__ \   / / / /|_/ /
 / /_/ / /|  /___/ /  / / / /  / /
/_____/_/ |_//____/  /_/ /_/  /_/
`
	titleColor.Println(banner)
	fmt.Printf("DNS Tunnel Manager v%s (built %s)\n\n", version, buildTime)
}

func printUsage() {
	fmt.Printf(`dnstm v%s (built %s)
DNS Tunnel Manager - https://github.com/net2share/dnstm

Usage: dnstm [command] [options]

Commands:
  install             Install/configure dnstt server
  status              Show service status
  logs                View service logs
  config              Show current configuration
  restart             Restart the dnstt service
  uninstall           Uninstall dnstt server
  ssh-users           Manage SSH tunnel users

If no command is specified, an interactive menu is shown.

Install Options:
  --ns-subdomain <domain>   NS subdomain (e.g., t.example.com)
  --mtu <value>             MTU value (512-1400, default: 1232)
  --mode <ssh|socks>        Tunnel mode (default: ssh)
  --port <port>             Target port (SSH port for ssh mode, default: 22)

Uninstall Options:
  --remove-ssh-users        Also remove SSH tunnel users and sshd config
  --keep-ssh-users          Keep SSH tunnel users and sshd config

Global Options:
  --help, -h                Show this help message
  --version, -v             Show version information

Examples:
  dnstm                     Run the interactive menu
  dnstm install             Interactive installation wizard
  dnstm install --ns-subdomain t.example.com --mode ssh
                            Full CLI installation
  dnstm status              Check service status
  dnstm ssh-users           Manage SSH tunnel users
  dnstm uninstall --remove-ssh-users
                            Uninstall dnstt and SSH tunnel config
  dnstm --help              Show this help
`, version, buildTime)
}

func parseArgs(args []string) (*Options, error) {
	opts := &Options{
		Command: CommandNone,
	}

	i := 0

	// Check for subcommand as first argument
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		switch args[0] {
		case "install":
			opts.Command = CommandInstall
			i++
		case "status":
			opts.Command = CommandStatus
			i++
		case "logs":
			opts.Command = CommandLogs
			i++
		case "config":
			opts.Command = CommandConfig
			i++
		case "restart":
			opts.Command = CommandRestart
			i++
		case "uninstall":
			opts.Command = CommandUninstall
			i++
		case "ssh-users":
			opts.Command = CommandSSHUsers
			i++
		default:
			return nil, fmt.Errorf("unknown command: %s\nRun 'dnstm --help' for usage", args[0])
		}
	}

	// Parse remaining arguments
	for ; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--help", "-h":
			opts.ShowHelp = true
		case "--version", "-v":
			opts.ShowVersion = true
		case "--ns-subdomain":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--ns-subdomain requires a value")
			}
			i++
			opts.NSSubdomain = args[i]
		case "--mtu":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--mtu requires a value")
			}
			i++
			opts.MTU = args[i]
		case "--mode":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--mode requires a value")
			}
			i++
			opts.TunnelMode = args[i]
			if opts.TunnelMode != "ssh" && opts.TunnelMode != "socks" {
				return nil, fmt.Errorf("--mode must be 'ssh' or 'socks'")
			}
		case "--port":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--port requires a value")
			}
			i++
			opts.TargetPort = args[i]
		case "--remove-ssh-users":
			t := true
			opts.RemoveSSHUsers = &t
		case "--keep-ssh-users":
			f := false
			opts.RemoveSSHUsers = &f
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return nil, fmt.Errorf("unknown option: %s\nRun 'dnstm --help' for usage", arg)
			}
			return nil, fmt.Errorf("unexpected argument: %s", arg)
		}
	}

	return opts, nil
}

func Run(v, bt string, args []string) error {
	version = v
	buildTime = bt

	opts, err := parseArgs(args)
	if err != nil {
		return err
	}

	if opts.ShowHelp {
		printUsage()
		return nil
	}

	if opts.ShowVersion {
		fmt.Printf("dnstm v%s (built %s)\n", version, buildTime)
		return nil
	}

	// Commands that don't require root
	// (none currently, but structure is here)

	if !osdetect.IsRoot() {
		return fmt.Errorf("this program must be run as root")
	}

	osInfo, err := osdetect.Detect()
	if err != nil {
		tui.PrintWarning("Could not detect OS: " + err.Error())
	}

	archInfo := detectArch()

	// Route to command handlers
	switch opts.Command {
	case CommandNone:
		printBanner()
		if osInfo != nil {
			tui.PrintInfo(fmt.Sprintf("Detected OS: %s", osInfo.PrettyName))
		}
		tui.PrintInfo(fmt.Sprintf("Architecture: %s", archInfo.Arch))
		return showMainMenu(osInfo, archInfo)

	case CommandInstall:
		printBanner()
		if osInfo != nil {
			tui.PrintInfo(fmt.Sprintf("Detected OS: %s", osInfo.PrettyName))
		}
		tui.PrintInfo(fmt.Sprintf("Architecture: %s", archInfo.Arch))
		return runInstallCommand(osInfo, archInfo, opts)

	case CommandStatus:
		return runStatusCommand()

	case CommandLogs:
		return runLogsCommand()

	case CommandConfig:
		return runConfigCommand()

	case CommandRestart:
		return runRestartCommand()

	case CommandUninstall:
		printBanner()
		return runUninstallCommand(opts)

	case CommandSSHUsers:
		printBanner()
		sshtunnel.ShowMenu()
		return nil

	default:
		return fmt.Errorf("unknown command")
	}
}

func showMainMenu(osInfo *osdetect.OSInfo, archInfo *ArchInfo) error {
	for {
		fmt.Println()

		// Check current state
		isInstalled := service.IsInstalled() && config.Exists()

		// Build menu options based on state
		var options []tui.MenuOption

		if isInstalled {
			options = []tui.MenuOption{
				{Key: "1", Label: "Reconfigure dnstt server"},
				{Key: "2", Label: "Check service status"},
				{Key: "3", Label: "View service logs"},
				{Key: "4", Label: "Show configuration info"},
				{Key: "5", Label: "Restart service"},
				{Key: "6", Label: "Manage SSH tunnel users"},
				{Key: "7", Label: "Uninstall"},
				{Key: "0", Label: "Exit"},
			}
		} else {
			// Not installed - show menu with install and SSH users
			options = []tui.MenuOption{
				{Key: "1", Label: "Install dnstt server"},
				{Key: "2", Label: "Manage SSH tunnel users"},
				{Key: "0", Label: "Exit"},
			}
		}

		tui.ShowMenu(options)
		choice := tui.Prompt("Select option")

		if isInstalled {
			switch choice {
			case "1":
				if err := runInstallInteractive(osInfo, archInfo); err != nil {
					tui.PrintError(err.Error())
				}
				tui.WaitForEnter()
			case "2":
				showStatus()
			case "3":
				showLogs()
			case "4":
				showConfig()
			case "5":
				restartService()
			case "6":
				sshtunnel.ShowMenu()
			case "7":
				runUninstall()
				tui.WaitForEnter()
			case "0", "q", "quit", "exit":
				tui.PrintInfo("Goodbye!")
				return nil
			default:
				tui.PrintError("Invalid option")
			}
		} else {
			switch choice {
			case "1":
				if err := runInstallInteractive(osInfo, archInfo); err != nil {
					tui.PrintError(err.Error())
					tui.WaitForEnter()
				}
			case "2":
				sshtunnel.ShowMenu()
			case "0", "q", "quit", "exit":
				tui.PrintInfo("Goodbye!")
				return nil
			default:
				tui.PrintError("Invalid option")
			}
		}
	}
}

func runInstallCommand(osInfo *osdetect.OSInfo, archInfo *ArchInfo, opts *Options) error {
	// Check if any install options were provided
	hasAnyOption := opts.NSSubdomain != "" || opts.MTU != "" || opts.TunnelMode != "" || opts.TargetPort != ""

	if !hasAnyOption {
		// No options provided - run interactive mode
		return runInstallInteractive(osInfo, archInfo)
	}

	// Some options provided - validate all required options are present
	if opts.NSSubdomain == "" {
		return fmt.Errorf("--ns-subdomain is required for CLI installation")
	}

	// Set defaults for optional parameters
	if opts.MTU == "" {
		opts.MTU = "1232"
	}
	if opts.TunnelMode == "" {
		opts.TunnelMode = "ssh"
	}
	if opts.TargetPort == "" {
		if opts.TunnelMode == "ssh" {
			opts.TargetPort = osdetect.DetectSSHPort()
		} else {
			opts.TargetPort = "1080"
		}
	}

	// Validate MTU
	mtu, err := strconv.Atoi(opts.MTU)
	if err != nil || mtu < 512 || mtu > 1400 {
		return fmt.Errorf("--mtu must be a number between 512 and 1400")
	}

	// Run CLI installation
	return runInstallCLI(osInfo, archInfo, opts)
}

func runInstallInteractive(osInfo *osdetect.OSInfo, archInfo *ArchInfo) error {
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{
			MTU:        "1232",
			TunnelMode: "ssh",
		}
	}

	fmt.Println()
	tui.PrintInfo("Starting installation wizard...")
	fmt.Println()

	// Step 1: Get NS subdomain
	cfg.NSSubdomain = tui.PromptWithDefault("Enter NS subdomain (e.g., t.example.com)", cfg.NSSubdomain)
	if cfg.NSSubdomain == "" {
		return fmt.Errorf("NS subdomain is required")
	}

	// Step 2: Get MTU
	currentMTU := 1232
	if cfg.MTU != "" {
		if parsed, err := strconv.Atoi(cfg.MTU); err == nil {
			currentMTU = parsed
		}
	}
	mtuInt := tui.PromptInt("Enter MTU value (512-1400)", currentMTU, 512, 1400)
	cfg.MTU = fmt.Sprintf("%d", mtuInt)

	// Step 3: Get tunnel mode
	cfg.TunnelMode = tui.PromptChoice("Select tunnel mode", []string{"ssh", "socks"}, cfg.TunnelMode)

	// Step 4: Get target port for SSH mode
	if cfg.TunnelMode == "ssh" {
		defaultPort := osdetect.DetectSSHPort()
		cfg.TargetPort = tui.PromptWithDefault("Enter SSH port", defaultPort)
	} else {
		cfg.TargetPort = "1080"
	}

	// Set key file paths
	cfg.PrivateKeyFile, cfg.PublicKeyFile = config.GetKeyFilenames(cfg.NSSubdomain)

	fmt.Println()
	if !tui.Confirm("Proceed with installation?", true) {
		return fmt.Errorf("installation cancelled")
	}

	return runInstallation(osInfo, archInfo, cfg)
}

func runInstallCLI(osInfo *osdetect.OSInfo, archInfo *ArchInfo, opts *Options) error {
	cfg := &config.Config{
		NSSubdomain: opts.NSSubdomain,
		MTU:         opts.MTU,
		TunnelMode:  opts.TunnelMode,
		TargetPort:  opts.TargetPort,
	}

	cfg.PrivateKeyFile, cfg.PublicKeyFile = config.GetKeyFilenames(cfg.NSSubdomain)

	return runInstallation(osInfo, archInfo, cfg)
}

func runInstallation(osInfo *osdetect.OSInfo, archInfo *ArchInfo, cfg *config.Config) error {
	totalSteps := 7
	currentStep := 0

	// Step 1: Download dnstt-server
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Downloading dnstt-server binary...")

	if !download.IsDnsttInstalled() {
		checksums, _ := download.FetchChecksums(archInfo.DnsttArch)

		tmpPath, err := download.DownloadDnsttServer(archInfo.DnsttArch, tui.PrintProgress)
		tui.ClearLine()
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		if checksums.SHA256 != "" || checksums.MD5 != "" {
			if err := download.VerifyChecksums(tmpPath, checksums); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("checksum verification failed: %w", err)
			}
			tui.PrintStatus("Checksum verified")
		}

		if err := download.InstallBinary(tmpPath); err != nil {
			return fmt.Errorf("installation failed: %w", err)
		}
	}
	tui.PrintStatus("dnstt-server binary installed")

	// Step 2: Create dnstt user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating dnstt user...")
	if err := system.CreateDnsttUser(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	tui.PrintStatus("User 'dnstt' created")

	// Step 3: Generate keys
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Generating cryptographic keys...")

	var publicKey string
	var err error
	if keys.KeysExist(cfg.PrivateKeyFile, cfg.PublicKeyFile) {
		if tui.Confirm("Keys already exist. Regenerate?", false) {
			publicKey, err = keys.Generate(cfg.PrivateKeyFile, cfg.PublicKeyFile)
			if err != nil {
				return fmt.Errorf("key generation failed: %w", err)
			}
		} else {
			publicKey, _ = keys.ReadPublicKey(cfg.PublicKeyFile)
		}
	} else {
		publicKey, err = keys.Generate(cfg.PrivateKeyFile, cfg.PublicKeyFile)
		if err != nil {
			return fmt.Errorf("key generation failed: %w", err)
		}
	}
	tui.PrintStatus("Keys generated")

	// Step 4: Save configuration
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Saving configuration...")
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	tui.PrintStatus("Configuration saved")

	// Step 5: Configure firewall
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Configuring firewall...")
	if err := network.ConfigureFirewall(); err != nil {
		tui.PrintWarning("Firewall configuration warning: " + err.Error())
	} else {
		tui.PrintStatus("Firewall configured")
	}

	if osdetect.HasIPv6() {
		network.ConfigureIPv6()
		tui.PrintStatus("IPv6 rules configured")
	}

	// Step 6: Setup Dante (if SOCKS mode) or SSH tunnel hardening (if SSH mode)
	var createdUser *sshtunnel.CreatedUserInfo
	currentStep++
	if cfg.TunnelMode == "socks" {
		tui.PrintStep(currentStep, totalSteps, "Setting up Dante SOCKS proxy...")

		if !proxy.IsDanteInstalled() {
			if osInfo != nil {
				if err := proxy.InstallDante(osInfo.PackageManager); err != nil {
					return fmt.Errorf("failed to install Dante: %w", err)
				}
			}
		}

		if err := proxy.ConfigureDante(); err != nil {
			return fmt.Errorf("failed to configure Dante: %w", err)
		}

		if err := proxy.StartDante(); err != nil {
			tui.PrintWarning("Dante start warning: " + err.Error())
		}
		tui.PrintStatus("Dante SOCKS proxy configured")
		cfg.SSHTunnelEnabled = "false"
	} else {
		tui.PrintStep(currentStep, totalSteps, "Setting up SSH tunnel hardening...")
		createdUser = sshtunnel.ConfigureAndCreateUser()
		if sshtunnel.IsConfigured() {
			cfg.SSHTunnelEnabled = "true"
			tui.PrintStatus("SSH tunnel hardening configured")
		} else {
			cfg.SSHTunnelEnabled = "false"
			tui.PrintStatus("SSH mode selected (tunnel hardening failed)")
		}
	}

	// Save config again to persist SSHTunnelEnabled
	if err := cfg.Save(); err != nil {
		tui.PrintWarning("Failed to save SSH tunnel state: " + err.Error())
	}

	// Step 7: Create and start systemd service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Creating systemd service...")

	if err := service.CreateService(cfg); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	if err := service.SetPermissions(); err != nil {
		tui.PrintWarning("Permission setting warning: " + err.Error())
	}

	if err := service.Enable(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	if err := service.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	tui.PrintStatus("Service started")

	// Show success information
	showSuccessInfo(cfg, publicKey, createdUser)

	return nil
}

func showSuccessInfo(cfg *config.Config, publicKey string, createdUser *sshtunnel.CreatedUserInfo) {
	lines := []string{
		fmt.Sprintf("NS Subdomain: %s", cfg.NSSubdomain),
		fmt.Sprintf("Tunnel Mode:  %s", cfg.TunnelMode),
		fmt.Sprintf("MTU:          %s", cfg.MTU),
		"",
		"Public Key (for client):",
		publicKey,
	}

	// Add created user info if available
	if createdUser != nil {
		lines = append(lines, "")
		lines = append(lines, "SSH Tunnel User Created:")
		lines = append(lines, fmt.Sprintf("  Username: %s", createdUser.Username))
		lines = append(lines, fmt.Sprintf("  Auth:     %s", createdUser.AuthMode))
		if createdUser.Password != "" {
			lines = append(lines, fmt.Sprintf("  Password: %s", createdUser.Password))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "DNS Configuration Required:")
	lines = append(lines, "  A record:  tns.yourdomain.com -> <server-ip>")
	lines = append(lines, "  NS record: t.yourdomain.com   -> tns.yourdomain.com")

	tui.PrintBox("Installation Complete!", lines)

	tui.PrintInfo("Useful commands:")
	fmt.Println("  systemctl status dnstt-server  - Check service status")
	fmt.Println("  journalctl -u dnstt-server -f  - View live logs")
	fmt.Println("  dnstm                          - Open this menu")
}

func runStatusCommand() error {
	if !service.IsInstalled() {
		return fmt.Errorf("dnstt is not installed")
	}
	status, _ := service.Status()
	fmt.Println(status)
	return nil
}

func runLogsCommand() error {
	if !service.IsInstalled() {
		return fmt.Errorf("dnstt is not installed")
	}
	logs, err := service.GetLogs(50)
	if err != nil {
		return err
	}
	fmt.Println(logs)
	return nil
}

func runConfigCommand() error {
	if !config.Exists() {
		return fmt.Errorf("dnstt is not configured")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	publicKey := ""
	if cfg.PublicKeyFile != "" {
		publicKey, _ = keys.ReadPublicKey(cfg.PublicKeyFile)
	}

	fmt.Printf("NS Subdomain:    %s\n", cfg.NSSubdomain)
	fmt.Printf("Tunnel Mode:     %s\n", cfg.TunnelMode)
	fmt.Printf("MTU:             %s\n", cfg.MTU)
	fmt.Printf("Target Port:     %s\n", cfg.TargetPort)
	fmt.Printf("Private Key:     %s\n", cfg.PrivateKeyFile)
	fmt.Printf("Public Key File: %s\n", cfg.PublicKeyFile)
	fmt.Println()
	fmt.Println("Public Key (for client):")
	fmt.Println(publicKey)

	return nil
}

func runRestartCommand() error {
	if !service.IsInstalled() {
		return fmt.Errorf("dnstt is not installed")
	}
	fmt.Println("Restarting dnstt-server...")
	if err := service.Restart(); err != nil {
		return err
	}
	fmt.Println("Service restarted successfully")
	return nil
}

func showStatus() {
	fmt.Println()
	status, _ := service.Status()
	fmt.Println(status)
	tui.WaitForEnter()
}

func showLogs() {
	fmt.Println()
	logs, err := service.GetLogs(50)
	if err != nil {
		tui.PrintError(err.Error())
	} else {
		fmt.Println(logs)
	}
	tui.WaitForEnter()
}

func showConfig() {
	cfg, err := config.Load()
	if err != nil {
		tui.PrintError("Failed to load configuration: " + err.Error())
		tui.WaitForEnter()
		return
	}

	publicKey := ""
	if cfg.PublicKeyFile != "" {
		publicKey, _ = keys.ReadPublicKey(cfg.PublicKeyFile)
	}

	lines := []string{
		fmt.Sprintf("NS Subdomain:    %s", cfg.NSSubdomain),
		fmt.Sprintf("Tunnel Mode:     %s", cfg.TunnelMode),
		fmt.Sprintf("MTU:             %s", cfg.MTU),
		fmt.Sprintf("Target Port:     %s", cfg.TargetPort),
		fmt.Sprintf("Private Key:     %s", cfg.PrivateKeyFile),
		fmt.Sprintf("Public Key File: %s", cfg.PublicKeyFile),
		"",
		"Public Key (for client):",
		publicKey,
	}

	tui.PrintBox("Current Configuration", lines)

	if service.IsActive() {
		tui.PrintStatus("Service is running")
	} else {
		tui.PrintWarning("Service is not running")
	}

	tui.WaitForEnter()
}

func restartService() {
	tui.PrintInfo("Restarting service...")
	if err := service.Restart(); err != nil {
		tui.PrintError(err.Error())
	} else {
		tui.PrintStatus("Service restarted successfully")
	}
	tui.WaitForEnter()
}

func runUninstallCommand(opts *Options) error {
	// Check if SSH users option was specified in CLI mode
	if opts.RemoveSSHUsers == nil {
		// No option specified - this is an error in CLI mode
		// Check if any other args were provided to determine if this is CLI mode
		// If we got here with no option, it means user ran "dnstm uninstall" without flags
		// which should be interactive mode
		_, err := runUninstallInteractive()
		return err
	}

	// CLI mode - run non-interactive uninstall
	return runUninstallCLI(*opts.RemoveSSHUsers)
}

// uninstallResult indicates what happened during uninstall
type uninstallResult int

const (
	uninstallCancelled uninstallResult = iota
	uninstallCompleted
)

func runUninstallInteractive() (uninstallResult, error) {
	fmt.Println()
	tui.PrintWarning("This will completely remove dnstt from your system:")
	fmt.Println("  - Stop and remove the dnstt-server service")
	fmt.Println("  - Remove the dnstt-server binary")
	fmt.Println("  - Remove all configuration files and keys")
	fmt.Println("  - Remove firewall rules")
	fmt.Println("  - Remove the dnstt system user")
	fmt.Println()

	if !tui.Confirm("Are you sure you want to uninstall?", false) {
		tui.PrintInfo("Uninstall cancelled")
		return uninstallCancelled, nil
	}

	// Ask about SSH tunnel users
	removeSSHUsers := false
	if sshtunnel.IsConfigured() {
		fmt.Println()
		tui.PrintInfo("SSH tunnel hardening is configured on this system.")
		removeSSHUsers = tui.Confirm("Also remove SSH tunnel users and sshd hardening config?", false)
	}

	performUninstall(removeSSHUsers)
	return uninstallCompleted, nil
}

func runUninstallCLI(removeSSHUsers bool) error {
	performUninstall(removeSSHUsers)
	return nil
}

func performUninstall(removeSSHUsers bool) {
	fmt.Println()
	totalSteps := 5
	if removeSSHUsers {
		totalSteps = 6
	}
	currentStep := 0

	// Step 1: Stop and remove service
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Stopping and removing service...")
	if service.IsActive() {
		service.Stop()
	}
	if service.IsEnabled() {
		service.Disable()
	}
	service.Remove()
	tui.PrintStatus("Service removed")

	// Step 2: Remove binary
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing dnstt-server binary...")
	download.RemoveBinary()
	tui.PrintStatus("Binary removed")

	// Step 3: Remove configuration and keys
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing configuration and keys...")
	config.RemoveAll()
	tui.PrintStatus("Configuration removed")

	// Step 4: Remove firewall rules
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing firewall rules...")
	network.RemoveFirewallRules()
	tui.PrintStatus("Firewall rules removed")

	// Step 5: Remove dnstt user
	currentStep++
	tui.PrintStep(currentStep, totalSteps, "Removing dnstt user...")
	system.RemoveDnsttUser()
	tui.PrintStatus("User removed")

	// Step 6: Remove SSH tunnel users and config (if requested)
	if removeSSHUsers {
		currentStep++
		tui.PrintStep(currentStep, totalSteps, "Removing SSH tunnel users and config...")
		if err := sshtunnel.UninstallAll(); err != nil {
			tui.PrintWarning("SSH tunnel uninstall warning: " + err.Error())
		} else {
			tui.PrintStatus("SSH tunnel config removed")
		}
	}

	fmt.Println()
	tui.PrintSuccess("Uninstallation complete!")
	tui.PrintInfo("All dnstt components have been removed from your system.")
}

// runUninstall is called from interactive menu, returns true only if uninstall was completed
func runUninstall() bool {
	result, err := runUninstallInteractive()
	if err != nil {
		tui.PrintError(err.Error())
		return false
	}
	return result == uninstallCompleted
}

// Helper to check if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
