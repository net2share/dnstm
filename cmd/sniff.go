package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/net2share/dnstm/internal/monitor"
	"github.com/spf13/cobra"
)

var sniffCmd = &cobra.Command{
	Use:    "sniff",
	Short:  "Sniff DNS traffic and write stats (used internally)",
	Hidden: true,
	Args:   cobra.MinimumNArgs(1),
	RunE:   runSniff,
}

func init() {
	rootCmd.AddCommand(sniffCmd)
	sniffCmd.Flags().String("tag", "", "Tunnel tag (used for stats file naming)")
	sniffCmd.Flags().Int("port", 53, "DNS port to sniff")
	sniffCmd.Flags().String("metrics-address", "", "Address to serve Prometheus metrics on (e.g. :9100)")
}

func runSniff(cmd *cobra.Command, args []string) error {
	tag, _ := cmd.Flags().GetString("tag")
	port, _ := cmd.Flags().GetInt("port")
	domains := args

	if tag == "" {
		// Derive tag from first domain
		tag = strings.ReplaceAll(domains[0], ".", "-")
	}

	statsFile := monitor.StatsFilePath(tag)

	log.Printf("Sniffing port %d for domains: %v", port, domains)
	log.Printf("Writing stats to: %s", statsFile)

	// Ensure stats dir exists
	_ = os.MkdirAll(monitor.RunDir, 0755)

	// Open raw socket once — keep it for the lifetime of the process
	fd, err := monitor.OpenRawSocket()
	if err != nil {
		return fmt.Errorf("failed to open raw socket: %w", err)
	}
	defer syscall.Close(fd)

	metricsAddr, _ := cmd.Flags().GetString("metrics-address")

	coll := monitor.NewCollector(domains)

	// Restore previous stats so history survives restarts
	var prevDuration time.Duration
	var history []monitor.DataPoint
	if prev, err := monitor.ReadStats(tag); err == nil && prev != nil {
		coll.Restore(prev)
		prevDuration = prev.Duration
		history = prev.History
		log.Printf("Restored previous stats: %d queries, %d sessions, peak %d, uptime %s, %d history points",
			prev.TotalQueries, prev.TotalClients, prev.PeakClients, prev.Duration.Round(time.Second), len(history))
	}

	start := time.Now()

	// Start Prometheus metrics HTTP server if address was provided
	if metricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", monitor.MetricsHandler(coll, start))
		srv := &http.Server{Addr: metricsAddr, Handler: mux}
		go func() {
			log.Printf("Serving Prometheus metrics on %s/metrics", metricsAddr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Metrics server error: %v", err)
			}
		}()
	}

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Write stats periodically in background
	writeTicker := time.NewTicker(2 * time.Second)
	defer writeTicker.Stop()

	// Packet capture in background
	stopCh := make(chan struct{})
	go func() {
		monitor.CaptureLoop(fd, port, coll, stopCh)
	}()

	log.Printf("Sniffer running.")

	for {
		select {
		case <-writeTicker.C:
			result := coll.Result(prevDuration + time.Since(start))
			// Append data point to history
			history = append(history, monitor.DataPoint{
				Time:          time.Now(),
				ActiveClients: result.ActiveClients,
			})
			// Trim to max history size
			if len(history) > monitor.MaxHistory {
				history = history[len(history)-monitor.MaxHistory:]
			}
			result.History = history
			writeStats(statsFile, result)
		case <-sigCh:
			log.Printf("Shutting down...")
			close(stopCh)
			// Final write
			result := coll.Result(prevDuration + time.Since(start))
			history = append(history, monitor.DataPoint{
				Time:          time.Now(),
				ActiveClients: result.ActiveClients,
			})
			if len(history) > monitor.MaxHistory {
				history = history[len(history)-monitor.MaxHistory:]
			}
			result.History = history
			writeStats(statsFile, result)
			return nil
		}
	}
}

func writeStats(path string, result *monitor.CaptureResult) {
	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("Failed to marshal stats: %v", err)
		return
	}
	// Write atomically via temp file
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		log.Printf("Failed to write stats: %v", err)
		return
	}
	os.Rename(tmp, path)
}
