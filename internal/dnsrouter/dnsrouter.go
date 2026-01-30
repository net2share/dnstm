package dnsrouter

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// MaxPacketSize is the maximum DNS packet size we handle
	MaxPacketSize = 4096

	// DefaultTimeout is the default upstream query timeout
	DefaultTimeout = 5 * time.Second

	// ConnectionIdleTimeout is how long to keep idle backend connections
	ConnectionIdleTimeout = 60 * time.Second
)

// Buffer pools to reduce allocations
var (
	packetPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, MaxPacketSize)
			return &buf
		},
	}
)

// Route defines a domain suffix to backend mapping.
type Route struct {
	Domain  string // Domain suffix to match (e.g., "example.com")
	Backend string // Backend address (e.g., "127.0.0.1:5310")
}

// pendingQuery represents a query waiting for a response
type pendingQuery struct {
	responseCh chan []byte
	deadline   time.Time
}

// backendConn manages a persistent connection to a backend
type backendConn struct {
	addr    *net.UDPAddr
	conn    *net.UDPConn
	mu      sync.Mutex
	pending map[uint16]*pendingQuery // keyed by DNS transaction ID
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	timeout time.Duration
}

// Router is a minimal DNS router that forwards raw packets.
type Router struct {
	listenAddr     string
	routes         []Route
	defaultBackend string
	timeout        time.Duration

	conn   *net.UDPConn
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Backend connection pool
	backends   map[string]*backendConn
	backendsMu sync.RWMutex

	// Stats (atomic for lock-free updates)
	queriesTotal atomic.Uint64
	errorsTotal  atomic.Uint64
}

// NewRouter creates a new DNS router.
func NewRouter(listenAddr string, routes []Route, defaultBackend string) *Router {
	return &Router{
		listenAddr:     listenAddr,
		routes:         routes,
		defaultBackend: defaultBackend,
		timeout:        DefaultTimeout,
		backends:       make(map[string]*backendConn),
	}
}

// SetTimeout sets the upstream query timeout.
func (r *Router) SetTimeout(timeout time.Duration) {
	r.timeout = timeout
}

// Start starts the DNS router.
func (r *Router) Start() error {
	addr, err := net.ResolveUDPAddr("udp", r.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	r.conn = conn
	r.ctx, r.cancel = context.WithCancel(context.Background())

	r.wg.Add(1)
	go r.serve()

	log.Printf("[dnsrouter] Listening on %s (with connection pooling)", r.listenAddr)
	return nil
}

// Stop stops the DNS router.
func (r *Router) Stop() error {
	if r.cancel != nil {
		r.cancel()
	}
	if r.conn != nil {
		r.conn.Close()
	}

	// Close all backend connections
	r.backendsMu.Lock()
	for _, bc := range r.backends {
		bc.close()
	}
	r.backends = make(map[string]*backendConn)
	r.backendsMu.Unlock()

	r.wg.Wait()
	log.Printf("[dnsrouter] Stopped")
	return nil
}

// serve handles incoming DNS queries.
func (r *Router) serve() {
	defer r.wg.Done()

	buf := make([]byte, MaxPacketSize)

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		// Set read deadline so we can check for context cancellation
		r.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, clientAddr, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if r.ctx.Err() != nil {
				return
			}
			log.Printf("[dnsrouter] Read error: %v", err)
			continue
		}

		// Get a buffer from pool and copy packet
		packetBuf := packetPool.Get().(*[]byte)
		packet := (*packetBuf)[:n]
		copy(packet, buf[:n])

		// Handle the query in a goroutine
		go r.handleQuery(packet, packetBuf, clientAddr)
	}
}

// handleQuery processes a single DNS query.
func (r *Router) handleQuery(packet []byte, packetBuf *[]byte, clientAddr *net.UDPAddr) {
	// Return buffer to pool when done
	defer packetPool.Put(packetBuf)

	r.queriesTotal.Add(1)

	// Extract query name for routing
	queryName, err := ExtractQueryName(packet)
	if err != nil {
		log.Printf("[dnsrouter] Failed to extract query name: %v", err)
		r.errorsTotal.Add(1)
		return
	}

	// Find matching backend
	backend := r.findBackend(queryName)
	if backend == "" {
		log.Printf("[dnsrouter] No backend for query: %s", queryName)
		r.errorsTotal.Add(1)
		return
	}

	// Forward to backend and get response
	response, err := r.forwardQuery(packet, backend)
	if err != nil {
		log.Printf("[dnsrouter] Forward error for %s -> %s: %v", queryName, backend, err)
		r.errorsTotal.Add(1)
		return
	}

	// Send response back to client
	_, err = r.conn.WriteToUDP(response, clientAddr)
	if err != nil {
		log.Printf("[dnsrouter] Write error: %v", err)
		r.errorsTotal.Add(1)
	}
}

// findBackend finds the backend for a query name.
// Returns empty string if no route matches (request will be dropped).
// Note: defaultBackend is kept for display/state preservation only, not for routing.
func (r *Router) findBackend(queryName string) string {
	// Check routes in order (first match wins)
	for _, route := range r.routes {
		if MatchDomainSuffix(queryName, route.Domain) {
			return route.Backend
		}
	}

	// No match - drop the request
	// (defaultBackend is only used for display and mode-switching state preservation)
	return ""
}

// getBackendConn gets or creates a persistent connection to a backend.
func (r *Router) getBackendConn(backend string) (*backendConn, error) {
	// Fast path: check if connection exists
	r.backendsMu.RLock()
	bc, exists := r.backends[backend]
	r.backendsMu.RUnlock()

	if exists {
		return bc, nil
	}

	// Slow path: create new connection
	r.backendsMu.Lock()
	defer r.backendsMu.Unlock()

	// Double-check after acquiring write lock
	if bc, exists = r.backends[backend]; exists {
		return bc, nil
	}

	// Create new backend connection
	addr, err := net.ResolveUDPAddr("udp", backend)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve backend: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to backend: %w", err)
	}

	ctx, cancel := context.WithCancel(r.ctx)
	bc = &backendConn{
		addr:    addr,
		conn:    conn,
		pending: make(map[uint16]*pendingQuery),
		ctx:     ctx,
		cancel:  cancel,
		timeout: r.timeout,
	}

	// Start response reader goroutine
	bc.wg.Add(1)
	go bc.readResponses()

	r.backends[backend] = bc
	log.Printf("[dnsrouter] Created connection pool for backend %s", backend)

	return bc, nil
}

// forwardQuery forwards a raw DNS packet to a backend and returns the response.
func (r *Router) forwardQuery(packet []byte, backend string) ([]byte, error) {
	bc, err := r.getBackendConn(backend)
	if err != nil {
		return nil, err
	}

	return bc.query(packet, r.timeout)
}

// query sends a DNS query and waits for the response
func (bc *backendConn) query(packet []byte, timeout time.Duration) ([]byte, error) {
	if len(packet) < 2 {
		return nil, fmt.Errorf("packet too short")
	}

	// Extract transaction ID (first 2 bytes)
	txid := uint16(packet[0])<<8 | uint16(packet[1])

	// Create response channel
	responseCh := make(chan []byte, 1)
	pq := &pendingQuery{
		responseCh: responseCh,
		deadline:   time.Now().Add(timeout),
	}

	// Register pending query
	bc.mu.Lock()
	if _, exists := bc.pending[txid]; exists {
		bc.mu.Unlock()
		// Transaction ID collision - very rare, fall back to simple approach
		return bc.querySimple(packet, timeout)
	}
	bc.pending[txid] = pq
	bc.mu.Unlock()

	// Ensure cleanup
	defer func() {
		bc.mu.Lock()
		delete(bc.pending, txid)
		bc.mu.Unlock()
	}()

	// Send query
	_, err := bc.conn.Write(packet)
	if err != nil {
		return nil, fmt.Errorf("failed to send query: %w", err)
	}

	// Wait for response
	select {
	case response := <-responseCh:
		return response, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for response")
	case <-bc.ctx.Done():
		return nil, fmt.Errorf("backend connection closed")
	}
}

// querySimple is a fallback for transaction ID collisions
func (bc *backendConn) querySimple(packet []byte, timeout time.Duration) ([]byte, error) {
	// Create a temporary connection for this query
	conn, err := net.DialUDP("udp", nil, bc.addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp connection: %w", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	_, err = conn.Write(packet)
	if err != nil {
		return nil, fmt.Errorf("failed to send query: %w", err)
	}

	buf := make([]byte, MaxPacketSize)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return buf[:n], nil
}

// readResponses reads responses from the backend and dispatches them
func (bc *backendConn) readResponses() {
	defer bc.wg.Done()

	buf := make([]byte, MaxPacketSize)

	for {
		select {
		case <-bc.ctx.Done():
			return
		default:
		}

		// Set read deadline for periodic context checks
		bc.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, err := bc.conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Cleanup expired pending queries
				bc.cleanupExpired()
				continue
			}
			if bc.ctx.Err() != nil {
				return
			}
			log.Printf("[dnsrouter] Backend read error: %v", err)
			continue
		}

		if n < 2 {
			continue
		}

		// Extract transaction ID
		txid := uint16(buf[0])<<8 | uint16(buf[1])

		// Find and dispatch to pending query
		bc.mu.Lock()
		pq, exists := bc.pending[txid]
		if exists {
			delete(bc.pending, txid)
		}
		bc.mu.Unlock()

		if exists {
			// Make a copy of the response
			response := make([]byte, n)
			copy(response, buf[:n])

			// Non-blocking send (query might have timed out)
			select {
			case pq.responseCh <- response:
			default:
			}
		}
	}
}

// cleanupExpired removes expired pending queries
func (bc *backendConn) cleanupExpired() {
	now := time.Now()
	bc.mu.Lock()
	for txid, pq := range bc.pending {
		if now.After(pq.deadline) {
			delete(bc.pending, txid)
		}
	}
	bc.mu.Unlock()
}

// close closes the backend connection
func (bc *backendConn) close() {
	bc.cancel()
	bc.conn.Close()
	bc.wg.Wait()
}

// Stats returns router statistics.
func (r *Router) Stats() (queries, errors uint64) {
	return r.queriesTotal.Load(), r.errorsTotal.Load()
}

// GetRoutes returns the configured routes.
func (r *Router) GetRoutes() []Route {
	return r.routes
}

// GetDefaultBackend returns the default backend.
func (r *Router) GetDefaultBackend() string {
	return r.defaultBackend
}

// BackendStats returns statistics about backend connections
func (r *Router) BackendStats() map[string]int {
	r.backendsMu.RLock()
	defer r.backendsMu.RUnlock()

	stats := make(map[string]int)
	for backend, bc := range r.backends {
		bc.mu.Lock()
		stats[backend] = len(bc.pending)
		bc.mu.Unlock()
	}
	return stats
}
