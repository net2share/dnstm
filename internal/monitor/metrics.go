package monitor

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// MetricsHandler returns an HTTP handler that serves Prometheus metrics
// from the given Collector. Zero overhead between scrapes — all computation
// happens on-demand when /metrics is hit.
func MetricsHandler(coll *Collector, startTime time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		duration := time.Since(startTime)
		result := coll.Result(duration)

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		var b strings.Builder

		// -- Gauges --

		writeHelp(&b, "dnstm_active_clients", "gauge", "Number of currently connected dnstt clients")
		for domain, tr := range result.Tunnels {
			writeGauge(&b, "dnstm_active_clients", float64(tr.ActiveClients), "domain", domain)
		}

		writeHelp(&b, "dnstm_peak_clients", "gauge", "Peak concurrent dnstt clients observed")
		for domain, tr := range result.Tunnels {
			writeGauge(&b, "dnstm_peak_clients", float64(tr.PeakClients), "domain", domain)
		}

		writeHelp(&b, "dnstm_uptime_seconds", "gauge", "Sniffer uptime in seconds")
		writeGauge(&b, "dnstm_uptime_seconds", duration.Seconds())

		// -- Counters --

		writeHelp(&b, "dnstm_queries_total", "counter", "Total DNS queries observed")
		for domain, tr := range result.Tunnels {
			writeCounter(&b, "dnstm_queries_total", float64(tr.TotalQueries), "domain", domain)
		}

		writeHelp(&b, "dnstm_bytes_in_total", "counter", "Total bytes received (query payloads)")
		for domain, tr := range result.Tunnels {
			writeCounter(&b, "dnstm_bytes_in_total", float64(tr.TotalBytesIn), "domain", domain)
		}

		writeHelp(&b, "dnstm_bytes_out_total", "counter", "Total bytes sent (response payloads)")
		for domain, tr := range result.Tunnels {
			writeCounter(&b, "dnstm_bytes_out_total", float64(tr.TotalBytesOut), "domain", domain)
		}

		writeHelp(&b, "dnstm_sessions_total", "counter", "Total unique client sessions observed")
		for domain, tr := range result.Tunnels {
			writeCounter(&b, "dnstm_sessions_total", float64(tr.TotalClients), "domain", domain)
		}

		// -- Histograms --

		// Session duration histogram (only for inactive/completed sessions)
		// Active sessions are excluded since their duration is still growing
		writeHelp(&b, "dnstm_session_duration_seconds", "histogram", "Duration of completed client sessions in seconds")
		for domain, tr := range result.Tunnels {
			var durations []float64
			now := time.Now()
			for _, c := range tr.Clients {
				var d time.Duration
				if c.Active {
					d = now.Sub(c.FirstSeen)
				} else {
					d = c.LastSeen.Sub(c.FirstSeen)
				}
				durations = append(durations, d.Seconds())
			}
			writeHistogram(&b, "dnstm_session_duration_seconds",
				durationBuckets, durations, "domain", domain)
		}

		// Per-session traffic histogram
		writeHelp(&b, "dnstm_session_bytes", "histogram", "Total bytes per client session")
		for domain, tr := range result.Tunnels {
			var bytesVals []float64
			for _, c := range tr.Clients {
				bytesVals = append(bytesVals, float64(c.BytesTotal))
			}
			writeHistogram(&b, "dnstm_session_bytes",
				bytesBuckets, bytesVals, "domain", domain)
		}

		w.Write([]byte(b.String()))
	})
}

// Bucket boundaries for histograms.
// Duration: 10s, 30s, 1m, 5m, 15m, 30m, 1h, 2h, 6h, 12h, 24h
var durationBuckets = []float64{
	10, 30, 60, 300, 900, 1800, 3600, 7200, 21600, 43200, 86400,
}

// Bytes: 1KB, 10KB, 100KB, 1MB, 10MB, 100MB, 500MB, 1GB
var bytesBuckets = []float64{
	1024, 10240, 102400, 1048576, 10485760, 104857600, 524288000, 1073741824,
}

func writeHelp(b *strings.Builder, name, typ, help string) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s %s\n", name, typ)
}

func writeGauge(b *strings.Builder, name string, value float64, labels ...string) {
	fmt.Fprintf(b, "%s%s %g\n", name, formatLabels(labels), value)
}

func writeCounter(b *strings.Builder, name string, value float64, labels ...string) {
	fmt.Fprintf(b, "%s%s %g\n", name, formatLabels(labels), value)
}

func writeHistogram(b *strings.Builder, name string, buckets []float64, values []float64, labels ...string) {
	sort.Float64s(values)
	labelStr := formatLabels(labels)

	var sum float64
	for _, v := range values {
		sum += v
	}

	cumCount := 0
	vi := 0
	for _, bound := range buckets {
		for vi < len(values) && values[vi] <= bound {
			cumCount++
			vi++
		}
		le := fmt.Sprintf("%g", bound)
		if labels != nil {
			// Merge le into existing labels
			allLabels := append(labels, "le", le)
			fmt.Fprintf(b, "%s_bucket%s %d\n", name, formatLabels(allLabels), cumCount)
		} else {
			fmt.Fprintf(b, "%s_bucket{le=\"%s\"} %d\n", name, le, cumCount)
		}
	}
	// +Inf bucket
	if labels != nil {
		allLabels := append(labels, "le", "+Inf")
		fmt.Fprintf(b, "%s_bucket%s %d\n", name, formatLabels(allLabels), len(values))
	} else {
		fmt.Fprintf(b, "%s_bucket{le=\"+Inf\"} %d\n", name, len(values))
	}

	fmt.Fprintf(b, "%s_sum%s %g\n", name, labelStr, sum)
	fmt.Fprintf(b, "%s_count%s %d\n", name, labelStr, len(values))
}

func formatLabels(pairs []string) string {
	if len(pairs) == 0 {
		return ""
	}
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", pairs[i], escapeLabelValue(pairs[i+1])))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escapeLabelValue(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// FormatMetricValue formats a float64 for Prometheus output, handling special values.
func FormatMetricValue(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 1) {
		return "+Inf"
	}
	if math.IsInf(v, -1) {
		return "-Inf"
	}
	return fmt.Sprintf("%g", v)
}
