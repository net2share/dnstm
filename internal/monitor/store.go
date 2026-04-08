package monitor

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ClientTimeout is how long since last query before a client is considered disconnected.
// dnstt sends keepalive polls every few seconds even when idle, so 30s is safe.
const ClientTimeout = 30 * time.Second

// CaptureResult holds the results of a packet capture session.
type CaptureResult struct {
	Duration      time.Duration            `json:"duration"`
	Tunnels       map[string]*TunnelResult `json:"tunnels"`
	TotalQueries  uint64                   `json:"total_queries"`
	TotalBytesIn  uint64                   `json:"total_bytes_in"`
	TotalBytesOut uint64                   `json:"total_bytes_out"`
	ActiveClients int                      `json:"active_clients"`
	TotalClients  int                      `json:"total_clients"`
	PeakClients   int                      `json:"peak_clients"`
	History       []DataPoint              `json:"history,omitempty"`
}

// DataPoint is a single time series sample of connected users.
type DataPoint struct {
	Time          time.Time `json:"t"`
	ActiveClients int       `json:"n"`
}

// MaxHistory is the maximum number of data points kept.
// At 2s intervals, 900 points = 30 minutes of history.
const MaxHistory = 900

// TunnelResult holds capture results for a single tunnel domain.
type TunnelResult struct {
	Domain        string          `json:"domain"`
	TotalQueries  uint64          `json:"total_queries"`
	TotalBytesIn  uint64          `json:"total_bytes_in"`
	TotalBytesOut uint64          `json:"total_bytes_out"`
	QueriesPerSec float64         `json:"queries_per_sec"`
	ActiveClients int             `json:"active_clients"`
	TotalClients  int             `json:"total_clients"`
	PeakClients   int             `json:"peak_clients"`
	Clients       []*ClientResult `json:"clients"`
}

// ClientResult holds per-client stats.
// A "client" is identified by its dnstt ClientID (session), not by resolver IP.
type ClientResult struct {
	ClientID   string    `json:"client_id"`
	Queries    uint64    `json:"queries"`
	BytesIn    uint64    `json:"bytes_in"`
	BytesOut   uint64    `json:"bytes_out"`
	BytesTotal uint64    `json:"bytes_total"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
	Active     bool      `json:"active"`
}

// ClientSummary holds aggregate stats across all clients for a tunnel.
type ClientSummary struct {
	Count          int
	MinBytesTotal  uint64
	MaxBytesTotal  uint64
	MedianBytes    uint64
	MinQueries     uint64
	MaxQueries     uint64
	MedianQueries  uint64
	MinDuration    time.Duration
	MaxDuration    time.Duration
	MedianDuration time.Duration
}

// Collector accumulates packets during a capture session.
// Safe for concurrent use from capture goroutines.
type Collector struct {
	mu       sync.Mutex
	Domains  map[string]bool // registered tunnel domains (lowercase)
	tunnels  map[string]*tunnelCollector
	totalQ   atomic.Uint64
	totalIn  atomic.Uint64
	totalOut atomic.Uint64
}

type tunnelCollector struct {
	mu          sync.Mutex
	domain      string
	clients     map[string]*clientCollector
	peakClients int // highest concurrent active clients observed
	queries     atomic.Uint64
	bytesIn     atomic.Uint64
	bytesOut    atomic.Uint64
}

type clientCollector struct {
	mu        sync.Mutex
	clientID  string
	queries   uint64
	bytesIn   uint64
	bytesOut  uint64
	firstSeen time.Time
	lastSeen  time.Time
}

// NewCollector creates a new packet collector for the given tunnel domains.
func NewCollector(domains []string) *Collector {
	c := &Collector{
		Domains: make(map[string]bool),
		tunnels: make(map[string]*tunnelCollector),
	}
	for _, d := range domains {
		d = strings.ToLower(d)
		c.Domains[d] = true
		c.tunnels[d] = &tunnelCollector{
			domain:  d,
			clients: make(map[string]*clientCollector),
		}
	}
	return c
}

// Restore seeds the collector with previously saved stats so history survives restarts.
// Should be called before starting the capture loop.
func (c *Collector) Restore(prev *CaptureResult) {
	if prev == nil {
		return
	}

	c.totalQ.Store(prev.TotalQueries)
	c.totalIn.Store(prev.TotalBytesIn)
	c.totalOut.Store(prev.TotalBytesOut)

	for domain, tr := range prev.Tunnels {
		tc := c.findTunnel(domain)
		if tc == nil {
			continue
		}

		tc.queries.Store(tr.TotalQueries)
		tc.bytesIn.Store(tr.TotalBytesIn)
		tc.bytesOut.Store(tr.TotalBytesOut)
		tc.peakClients = tr.PeakClients

		tc.mu.Lock()
		for _, cr := range tr.Clients {
			tc.clients[cr.ClientID] = &clientCollector{
				clientID:  cr.ClientID,
				queries:   cr.Queries,
				bytesIn:   cr.BytesIn,
				bytesOut:  cr.BytesOut,
				firstSeen: cr.FirstSeen,
				lastSeen:  cr.LastSeen,
			}
		}
		tc.mu.Unlock()
	}
}

// RecordQuery records an incoming DNS query with extracted dnstt ClientID.
func (c *Collector) RecordQuery(domain string, clientID string, resolverIP string, size int) {
	c.totalQ.Add(1)
	c.totalIn.Add(uint64(size))

	tc := c.findTunnel(domain)
	if tc == nil {
		return
	}

	tc.queries.Add(1)
	tc.bytesIn.Add(uint64(size))

	if clientID == "" {
		return
	}

	now := time.Now()
	tc.mu.Lock()
	cc, exists := tc.clients[clientID]
	if !exists {
		cc = &clientCollector{clientID: clientID, firstSeen: now}
		tc.clients[clientID] = cc
	}
	tc.mu.Unlock()

	cc.mu.Lock()
	cc.lastSeen = now
	cc.queries++
	cc.bytesIn += uint64(size)
	cc.mu.Unlock()
}

// RecordResponse records an outgoing DNS response.
func (c *Collector) RecordResponse(domain string, dstIP string, size int) {
	c.totalOut.Add(uint64(size))

	tc := c.findTunnel(domain)
	if tc == nil {
		return
	}
	tc.bytesOut.Add(uint64(size))
}

func (c *Collector) findTunnel(queryDomain string) *tunnelCollector {
	queryDomain = strings.ToLower(queryDomain)

	if tc, ok := c.tunnels[queryDomain]; ok {
		return tc
	}

	for d, tc := range c.tunnels {
		if strings.HasSuffix(queryDomain, "."+d) {
			return tc
		}
	}

	return nil
}

// Result builds the CaptureResult snapshot from collected data.
// Can be called repeatedly — it reads atomics and takes locks momentarily.
func (c *Collector) Result(duration time.Duration) *CaptureResult {
	secs := duration.Seconds()
	if secs == 0 {
		secs = 1
	}

	now := time.Now()

	result := &CaptureResult{
		Duration:      duration,
		Tunnels:       make(map[string]*TunnelResult),
		TotalQueries:  c.totalQ.Load(),
		TotalBytesIn:  c.totalIn.Load(),
		TotalBytesOut: c.totalOut.Load(),
	}

	totalActive := 0
	totalSeen := 0
	totalPeak := 0

	for domain, tc := range c.tunnels {
		tr := &TunnelResult{
			Domain:        domain,
			TotalQueries:  tc.queries.Load(),
			TotalBytesIn:  tc.bytesIn.Load(),
			TotalBytesOut: tc.bytesOut.Load(),
			QueriesPerSec: float64(tc.queries.Load()) / secs,
		}

		tc.mu.Lock()
		for _, cc := range tc.clients {
			cc.mu.Lock()
			active := now.Sub(cc.lastSeen) < ClientTimeout
			cr := &ClientResult{
				ClientID:   cc.clientID,
				Queries:    cc.queries,
				BytesIn:    cc.bytesIn,
				BytesOut:   cc.bytesOut,
				BytesTotal: cc.bytesIn + cc.bytesOut,
				FirstSeen:  cc.firstSeen,
				LastSeen:   cc.lastSeen,
				Active:     active,
			}
			cc.mu.Unlock()
			tr.Clients = append(tr.Clients, cr)

			if active {
				tr.ActiveClients++
			}
		}
		tc.mu.Unlock()

		tr.TotalClients = len(tr.Clients)

		// Update peak concurrent clients for this tunnel
		tc.mu.Lock()
		if tr.ActiveClients > tc.peakClients {
			tc.peakClients = tr.ActiveClients
		}
		tr.PeakClients = tc.peakClients
		tc.mu.Unlock()

		// Sort: active clients first (by traffic desc), then inactive (by traffic desc)
		sort.Slice(tr.Clients, func(i, j int) bool {
			if tr.Clients[i].Active != tr.Clients[j].Active {
				return tr.Clients[i].Active // active first
			}
			return tr.Clients[i].BytesTotal > tr.Clients[j].BytesTotal
		})

		totalActive += tr.ActiveClients
		totalSeen += tr.TotalClients
		if tr.PeakClients > totalPeak {
			totalPeak = tr.PeakClients
		}
		result.Tunnels[domain] = tr
	}

	result.ActiveClients = totalActive
	result.TotalClients = totalSeen
	result.PeakClients = totalPeak
	return result
}

// Summary computes min/max/median stats across all clients for a tunnel.
func (tr *TunnelResult) Summary() *ClientSummary {
	n := len(tr.Clients)
	if n == 0 {
		return &ClientSummary{}
	}

	now := time.Now()
	bytesVals := make([]uint64, n)
	queryVals := make([]uint64, n)
	durVals := make([]time.Duration, n)
	for i, c := range tr.Clients {
		bytesVals[i] = c.BytesTotal
		queryVals[i] = c.Queries
		// For active clients, session is still ongoing
		if c.Active {
			durVals[i] = now.Sub(c.FirstSeen)
		} else {
			durVals[i] = c.LastSeen.Sub(c.FirstSeen)
		}
	}

	sort.Slice(bytesVals, func(i, j int) bool { return bytesVals[i] < bytesVals[j] })
	sort.Slice(queryVals, func(i, j int) bool { return queryVals[i] < queryVals[j] })
	sort.Slice(durVals, func(i, j int) bool { return durVals[i] < durVals[j] })

	return &ClientSummary{
		Count:          n,
		MinBytesTotal:  bytesVals[0],
		MaxBytesTotal:  bytesVals[n-1],
		MedianBytes:    bytesVals[n/2],
		MinQueries:     queryVals[0],
		MaxQueries:     queryVals[n-1],
		MedianQueries:  queryVals[n/2],
		MinDuration:    durVals[0],
		MaxDuration:    durVals[n-1],
		MedianDuration: durVals[n/2],
	}
}

// FormatBytes formats a byte count into a human-readable string.
func FormatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
