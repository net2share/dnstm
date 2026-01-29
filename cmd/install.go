package cmd

import (
	"fmt"

	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/system"
	"github.com/net2share/dnstm/internal/transport"
	"github.com/net2share/dnstm/internal/types"
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
  - Download and install transport binaries:
    - dnstt-server (DNSTT transport)
    - slipstream-server (Slipstream transport)
    - ssserver (Shadowsocks server)
    - microsocks (SOCKS5 proxy)
  - Configure firewall rules (port 53 UDP/TCP)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := osdetect.RequireRoot(); err != nil {
			return err
		}

		all, _ := cmd.Flags().GetBool("all")
		dnstt, _ := cmd.Flags().GetBool("dnstt")
		slipstream, _ := cmd.Flags().GetBool("slipstream")
		shadowsocks, _ := cmd.Flags().GetBool("shadowsocks")
		microsocks, _ := cmd.Flags().GetBool("microsocks")

		// If no specific flags, install all
		if !dnstt && !slipstream && !shadowsocks && !microsocks {
			all = true
		}

		fmt.Println()
		tui.PrintInfo("Installing dnstm components...")
		fmt.Println()

		// Step 1: Create dnstm user
		tui.PrintInfo("Creating dnstm user...")
		if err := system.CreateDnstmUser(); err != nil {
			tui.PrintWarning("User creation: " + err.Error())
		} else {
			tui.PrintStatus("dnstm user ready")
		}

		// Step 2: Install binaries
		tui.PrintInfo("Installing transport binaries...")
		fmt.Println()

		if all || dnstt {
			if err := transport.EnsureBinariesInstalled(types.TypeDNSTTSocks); err != nil {
				return fmt.Errorf("failed to install dnstt-server: %w", err)
			}
		}

		if all || slipstream {
			if err := transport.EnsureBinariesInstalled(types.TypeSlipstreamSocks); err != nil {
				return fmt.Errorf("failed to install slipstream-server: %w", err)
			}
		}

		if all || shadowsocks {
			if err := transport.EnsureBinariesInstalled(types.TypeSlipstreamShadowsocks); err != nil {
				return fmt.Errorf("failed to install shadowsocks: %w", err)
			}
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
				if err := proxy.ConfigureMicrosocks(); err != nil {
					tui.PrintWarning("microsocks service config: " + err.Error())
				} else {
					if err := proxy.StartMicrosocks(); err != nil {
						tui.PrintWarning("microsocks service start: " + err.Error())
					} else {
						tui.PrintStatus("microsocks installed and running")
					}
				}
			} else {
				tui.PrintStatus("microsocks already running")
			}
		}

		// Step 3: Configure firewall (open port 53 without NAT rules)
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
		fmt.Println("  1. Initialize router: dnstm router init")
		fmt.Println("  2. Add instance: dnstm instance add")
		fmt.Println("  3. Start router: dnstm router start")
		fmt.Println()

		return nil
	},
}

func init() {
	installCmd.Flags().Bool("all", false, "Install all binaries (default)")
	installCmd.Flags().Bool("dnstt", false, "Install dnstt-server only")
	installCmd.Flags().Bool("slipstream", false, "Install slipstream-server only")
	installCmd.Flags().Bool("shadowsocks", false, "Install ssserver only")
	installCmd.Flags().Bool("microsocks", false, "Install microsocks only")
}
