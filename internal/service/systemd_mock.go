package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MockSystemdManager is a mock implementation of SystemdManager for testing.
type MockSystemdManager struct {
	mu       sync.RWMutex
	services map[string]*mockServiceState
	stateDir string // Optional: persist state for debugging
}

// mockServiceState represents the state of a mocked service.
type mockServiceState struct {
	Config    ServiceConfig `json:"config"`
	Status    ServiceStatus `json:"status"`
	Enabled   bool          `json:"enabled"`
	Logs      []string      `json:"logs"`
	CreatedAt time.Time     `json:"created_at"`
	StartedAt *time.Time    `json:"started_at,omitempty"`
}

// NewMockSystemdManager creates a new MockSystemdManager.
// If stateDir is provided, state will be persisted as JSON for debugging.
func NewMockSystemdManager(stateDir string) *MockSystemdManager {
	m := &MockSystemdManager{
		services: make(map[string]*mockServiceState),
		stateDir: stateDir,
	}
	if stateDir != "" {
		os.MkdirAll(stateDir, 0755)
		m.loadState()
	}
	return m
}

// CreateService implements SystemdManager.
func (m *MockSystemdManager) CreateService(name string, cfg ServiceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg.Name = name
	m.services[name] = &mockServiceState{
		Config:    cfg,
		Status:    StatusStopped,
		Enabled:   false,
		Logs:      []string{},
		CreatedAt: time.Now(),
	}

	m.addLog(name, "Service created")
	m.saveState()
	return nil
}

// RemoveService implements SystemdManager.
func (m *MockSystemdManager) RemoveService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.services, name)
	m.saveState()
	return nil
}

// StartService implements SystemdManager.
func (m *MockSystemdManager) StartService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, exists := m.services[name]
	if !exists {
		return fmt.Errorf("service %s not found", name)
	}

	now := time.Now()
	svc.Status = StatusRunning
	svc.StartedAt = &now
	m.addLog(name, "Service started")
	m.saveState()
	return nil
}

// StopService implements SystemdManager.
func (m *MockSystemdManager) StopService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, exists := m.services[name]
	if !exists {
		return fmt.Errorf("service %s not found", name)
	}

	svc.Status = StatusStopped
	svc.StartedAt = nil
	m.addLog(name, "Service stopped")
	m.saveState()
	return nil
}

// RestartService implements SystemdManager.
func (m *MockSystemdManager) RestartService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, exists := m.services[name]
	if !exists {
		return fmt.Errorf("service %s not found", name)
	}

	now := time.Now()
	svc.Status = StatusRunning
	svc.StartedAt = &now
	m.addLog(name, "Service restarted")
	m.saveState()
	return nil
}

// EnableService implements SystemdManager.
func (m *MockSystemdManager) EnableService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, exists := m.services[name]
	if !exists {
		return fmt.Errorf("service %s not found", name)
	}

	svc.Enabled = true
	m.addLog(name, "Service enabled")
	m.saveState()
	return nil
}

// DisableService implements SystemdManager.
func (m *MockSystemdManager) DisableService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, exists := m.services[name]
	if !exists {
		return fmt.Errorf("service %s not found", name)
	}

	svc.Enabled = false
	m.addLog(name, "Service disabled")
	m.saveState()
	return nil
}

// IsServiceActive implements SystemdManager.
func (m *MockSystemdManager) IsServiceActive(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	svc, exists := m.services[name]
	if !exists {
		return false
	}
	return svc.Status == StatusRunning
}

// IsServiceEnabled implements SystemdManager.
func (m *MockSystemdManager) IsServiceEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	svc, exists := m.services[name]
	if !exists {
		return false
	}
	return svc.Enabled
}

// IsServiceInstalled implements SystemdManager.
func (m *MockSystemdManager) IsServiceInstalled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.services[name]
	return exists
}

// GetServiceStatus implements SystemdManager.
func (m *MockSystemdManager) GetServiceStatus(name string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	svc, exists := m.services[name]
	if !exists {
		return "", fmt.Errorf("service %s not found", name)
	}

	status := fmt.Sprintf("● %s.service - %s\n", name, svc.Config.Description)
	status += fmt.Sprintf("   Loaded: loaded\n")
	status += fmt.Sprintf("   Active: %s\n", svc.Status)
	if svc.StartedAt != nil {
		status += fmt.Sprintf("   Started: %s\n", svc.StartedAt.Format(time.RFC3339))
	}
	status += fmt.Sprintf("   Enabled: %v\n", svc.Enabled)

	return status, nil
}

// GetServiceLogs implements SystemdManager.
func (m *MockSystemdManager) GetServiceLogs(name string, lines int) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	svc, exists := m.services[name]
	if !exists {
		return "", fmt.Errorf("service %s not found", name)
	}

	// Return the last 'lines' log entries
	logs := svc.Logs
	if len(logs) > lines {
		logs = logs[len(logs)-lines:]
	}

	result := ""
	for _, log := range logs {
		result += log + "\n"
	}
	return result, nil
}

// DaemonReload implements SystemdManager.
func (m *MockSystemdManager) DaemonReload() error {
	// No-op for mock
	return nil
}

// addLog adds a log entry to a service (must be called with lock held).
func (m *MockSystemdManager) addLog(name, message string) {
	svc, exists := m.services[name]
	if !exists {
		return
	}
	entry := fmt.Sprintf("%s %s: %s", time.Now().Format(time.RFC3339), name, message)
	svc.Logs = append(svc.Logs, entry)
}

// saveState persists the current state to disk if stateDir is set.
func (m *MockSystemdManager) saveState() {
	if m.stateDir == "" {
		return
	}

	data, err := json.MarshalIndent(m.services, "", "  ")
	if err != nil {
		return
	}

	path := filepath.Join(m.stateDir, "mock_systemd_state.json")
	os.WriteFile(path, data, 0644)
}

// loadState loads the state from disk if stateDir is set.
func (m *MockSystemdManager) loadState() {
	if m.stateDir == "" {
		return
	}

	path := filepath.Join(m.stateDir, "mock_systemd_state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	json.Unmarshal(data, &m.services)
}

// Reset clears all mock state.
func (m *MockSystemdManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.services = make(map[string]*mockServiceState)
	m.saveState()
}

// GetServices returns all registered service names (for test assertions).
func (m *MockSystemdManager) GetServices() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.services))
	for name := range m.services {
		names = append(names, name)
	}
	return names
}

// GetServiceConfig returns the config for a service (for test assertions).
func (m *MockSystemdManager) GetServiceConfig(name string) (ServiceConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	svc, exists := m.services[name]
	if !exists {
		return ServiceConfig{}, false
	}
	return svc.Config, true
}

// SimulateFailure marks a service as failed (for testing failure scenarios).
func (m *MockSystemdManager) SimulateFailure(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, exists := m.services[name]
	if !exists {
		return fmt.Errorf("service %s not found", name)
	}

	svc.Status = StatusFailed
	m.addLog(name, "Service failed (simulated)")
	m.saveState()
	return nil
}

// Ensure MockSystemdManager implements SystemdManager.
var _ SystemdManager = (*MockSystemdManager)(nil)
