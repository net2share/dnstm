package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/dnsrouter"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install transport binaries and configure system",
	Long: `Install all transport binaries and configure the system for DNS tunneling.

This will:
  - Create dnstm system user
  - Initialize router configuration and directories
  - Set operating mode (single or multi)
  - Create DNS router service
  - Download and install transport binaries:
    - dnstt-server (DNSTT transport)
    - slipstream-server (Slipstream transport)
    - ssserver (Shadowsocks server)
    - microsocks (SOCKS5 proxy)
  - Configure firewall rules (port 53 UDP/TCP)

Use --mode to set the operating mode:
  single  Single-tunnel mode (default) - one tunnel at a time
  multi   Multi-tunnel mode - multiple tunnels with DNS router`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		all, _ := cmd.Flags().GetBool("all")
		dnstt, _ := cmd.Flags().GetBool("dnstt")
		slipstream, _ := cmd.Flags().GetBool("slipstream")
		shadowsocks, _ := cmd.Flags().GetBool("shadowsocks")
		microsocks, _ := cmd.Flags().GetBool("microsocks")
		modeStr, _ := cmd.Flags().GetString("mode")

		// If no specific flags, install all
		if !dnstt && !slipstream && !shadowsocks && !microsocks {
			all = true
		}

		// Validate mode if provided
		if modeStr != "" && modeStr != "single" && modeStr != "multi" {
			return fmt.Errorf("invalid mode '%s'. Use 'single' or 'multi'", modeStr)
		}
		if modeStr == "" {
			modeStr = "single" // default
		}

		fmt.Println()
		tui.PrintInfo("Installing dnstm components...")
		fmt.Println()

		// Step 1: Create dnstm user (must happen before router init for directory ownership)
		tui.PrintInfo("Creating dnstm user...")
		if err := system.CreateDnstmUser(); err != nil {
			return fmt.Errorf("failed to create dnstm user: %w", err)
		}
		tui.PrintStatus("dnstm user ready")

		// Step 2: Initialize router (create directories with proper ownership)
		tui.PrintInfo("Initializing router...")
		if err := router.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize router: %w", err)
		}
		tui.PrintStatus("Router initialized")

		// Step 3: Set operating mode
		cfg, err := router.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg.Mode = router.Mode(modeStr)
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		tui.PrintStatus(fmt.Sprintf("Mode set to %s", router.GetModeDisplayName(cfg.Mode)))

		// Step 4: Create DNS router service
		svc := dnsrouter.NewService()
		if err := svc.CreateService(); err != nil {
			tui.PrintWarning("DNS router service: " + err.Error())
		} else {
			tui.PrintStatus("DNS router service created")
		}

		// Step 5: Install binaries
		fmt.Println()
		tui.PrintInfo("Installing transport binaries...")
		fmt.Println()

		if all || dnstt {
			if err := transport.EnsureDnsttInstalled(); err != nil {
				return fmt.Errorf("failed to install dnstt-server: %w", err)
			}
		}

		if all || slipstream {
			if err := transport.EnsureSlipstreamInstalled(); err != nil {
				return fmt.Errorf("failed to install slipstream-server: %w", err)
			}
		}

		if all || shadowsocks {
			if err := transport.EnsureShadowsocksInstalled(); err != nil {
				return fmt.Errorf("failed to install ssserver: %w", err)
			}
		}

		// Always install sshtun-user for SSH user management
		if err := transport.EnsureSSHTunUserInstalled(); err != nil {
			tui.PrintWarning("sshtun-user: " + err.Error())
		}

		if all || microsocks {
			if !proxy.IsMicrosocksInstalled() {
				tui.PrintInfo("Installing microsocks...")
				if err := proxy.InstallMicrosocks(nil); err != nil {
					return fmt.Errorf("failed to install microsocks: %w", err)
				}
			}
			// Always ensure microsocks service is configured and running
			// This is needed for SOCKS-type transports
			if !proxy.IsMicrosocksRunning() {
				tui.PrintInfo("Configuring microsocks service...")
				// Find available port and save to config
				port, err := proxy.FindAvailablePort()
				if err != nil {
					tui.PrintWarning("Could not find available port: " + err.Error())
				} else {
					cfg.Proxy.Port = port
					if err := cfg.Save(); err != nil {
						tui.PrintWarning("Failed to save proxy port: " + err.Error())
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

		// Step 6: Configure firewall (open port 53 without NAT rules)
		// NAT rules are not needed in the new architecture:
		// - Single mode: transport binds directly to external IP:53
		// - Multi mode: DNS router binds directly to external IP:53
		fmt.Println()
		tui.PrintInfo("Configuring firewall...")
		// First clear any stale NAT rules from before.rules
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
		fmt.Println("  1. Add instance: dnstm instance add")
		fmt.Println("  2. Start router: dnstm router start")
		fmt.Println()

		return nil
	},
}

func init() {
	installCmd.Flags().StringP("mode", "m", "", "Operating mode: single or multi (default: single)")
	installCmd.Flags().Bool("all", false, "Install all binaries (default)")
	installCmd.Flags().Bool("dnstt", false, "Install dnstt-server only")
	installCmd.Flags().Bool("slipstream", false, "Install slipstream-server only")
	installCmd.Flags().Bool("shadowsocks", false, "Install ssserver only")
	installCmd.Flags().Bool("microsocks", false, "Install microsocks only")
}
