package service

import (
	"strings"
	"testing"
	"time"
)

func TestMockSystemdManager_CreateAndRemove(t *testing.T) {
	mock := NewMockSystemdManager("")

	err := mock.CreateService("test-service", ServiceConfig{
		Description: "Test Service",
		User:        "dnstm",
		Group:       "dnstm",
		ExecStart:   "/usr/bin/test",
	})
	if err != nil {
		t.Fatalf("CreateService failed: %v", err)
	}

	if !mock.IsServiceInstalled("test-service") {
		t.Error("service should be installed after create")
	}

	err = mock.RemoveService("test-service")
	if err != nil {
		t.Fatalf("RemoveService failed: %v", err)
	}

	if mock.IsServiceInstalled("test-service") {
		t.Error("service should not be installed after remove")
	}
}

func TestMockSystemdManager_StartStop(t *testing.T) {
	mock := NewMockSystemdManager("")

	mock.CreateService("test-service", ServiceConfig{
		Description: "Test Service",
		ExecStart:   "/usr/bin/test",
	})

	// Not active initially
	if mock.IsServiceActive("test-service") {
		t.Error("service should not be active initially")
	}

	// Start
	if err := mock.StartService("test-service"); err != nil {
		t.Fatalf("StartService failed: %v", err)
	}

	if !mock.IsServiceActive("test-service") {
		t.Error("service should be active after start")
	}

	// Stop
	if err := mock.StopService("test-service"); err != nil {
		t.Fatalf("StopService failed: %v", err)
	}

	if mock.IsServiceActive("test-service") {
		t.Error("service should not be active after stop")
	}
}

func TestMockSystemdManager_EnableDisable(t *testing.T) {
	mock := NewMockSystemdManager("")

	mock.CreateService("test-service", ServiceConfig{
		Description: "Test Service",
		ExecStart:   "/usr/bin/test",
	})

	// Not enabled initially
	if mock.IsServiceEnabled("test-service") {
		t.Error("service should not be enabled initially")
	}

	// Enable
	if err := mock.EnableService("test-service"); err != nil {
		t.Fatalf("EnableService failed: %v", err)
	}

	if !mock.IsServiceEnabled("test-service") {
		t.Error("service should be enabled after enable")
	}

	// Disable
	if err := mock.DisableService("test-service"); err != nil {
		t.Fatalf("DisableService failed: %v", err)
	}

	if mock.IsServiceEnabled("test-service") {
		t.Error("service should not be enabled after disable")
	}
}

func TestMockSystemdManager_Restart(t *testing.T) {
	mock := NewMockSystemdManager("")

	mock.CreateService("test-service", ServiceConfig{
		Description: "Test Service",
		ExecStart:   "/usr/bin/test",
	})

	// Restart should start it if stopped
	if err := mock.RestartService("test-service"); err != nil {
		t.Fatalf("RestartService failed: %v", err)
	}

	if !mock.IsServiceActive("test-service") {
		t.Error("service should be active after restart")
	}
}

func TestMockSystemdManager_GetStatus(t *testing.T) {
	mock := NewMockSystemdManager("")

	mock.CreateService("test-service", ServiceConfig{
		Description: "My Test Service",
		ExecStart:   "/usr/bin/test",
	})

	status, err := mock.GetServiceStatus("test-service")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}

	if !strings.Contains(status, "test-service") {
		t.Error("status should contain service name")
	}

	if !strings.Contains(status, "My Test Service") {
		t.Error("status should contain description")
	}
}

func TestMockSystemdManager_GetLogs(t *testing.T) {
	mock := NewMockSystemdManager("")

	mock.CreateService("test-service", ServiceConfig{
		Description: "Test Service",
		ExecStart:   "/usr/bin/test",
	})

	mock.StartService("test-service")
	mock.StopService("test-service")

	logs, err := mock.GetServiceLogs("test-service", 10)
	if err != nil {
		t.Fatalf("GetServiceLogs failed: %v", err)
	}

	if !strings.Contains(logs, "started") {
		t.Error("logs should contain 'started'")
	}

	if !strings.Contains(logs, "stopped") {
		t.Error("logs should contain 'stopped'")
	}
}

func TestMockSystemdManager_NotFound(t *testing.T) {
	mock := NewMockSystemdManager("")

	// Operations on non-existent service should fail
	if err := mock.StartService("nonexistent"); err == nil {
		t.Error("StartService should fail for nonexistent service")
	}

	if err := mock.StopService("nonexistent"); err == nil {
		t.Error("StopService should fail for nonexistent service")
	}

	if mock.IsServiceActive("nonexistent") {
		t.Error("IsServiceActive should return false for nonexistent service")
	}

	if mock.IsServiceEnabled("nonexistent") {
		t.Error("IsServiceEnabled should return false for nonexistent service")
	}

	if mock.IsServiceInstalled("nonexistent") {
		t.Error("IsServiceInstalled should return false for nonexistent service")
	}
}

func TestMockSystemdManager_SimulateFailure(t *testing.T) {
	mock := NewMockSystemdManager("")

	mock.CreateService("test-service", ServiceConfig{
		Description: "Test Service",
		ExecStart:   "/usr/bin/test",
	})

	mock.StartService("test-service")

	if err := mock.SimulateFailure("test-service"); err != nil {
		t.Fatalf("SimulateFailure failed: %v", err)
	}

	// Service should not be active after failure
	if mock.IsServiceActive("test-service") {
		t.Error("service should not be active after simulated failure")
	}
}

func TestMockSystemdManager_GetServices(t *testing.T) {
	mock := NewMockSystemdManager("")

	mock.CreateService("service-1", ServiceConfig{ExecStart: "/bin/test"})
	mock.CreateService("service-2", ServiceConfig{ExecStart: "/bin/test"})
	mock.CreateService("service-3", ServiceConfig{ExecStart: "/bin/test"})

	services := mock.GetServices()
	if len(services) != 3 {
		t.Errorf("expected 3 services, got %d", len(services))
	}
}

func TestMockSystemdManager_Reset(t *testing.T) {
	mock := NewMockSystemdManager("")

	mock.CreateService("service-1", ServiceConfig{ExecStart: "/bin/test"})
	mock.CreateService("service-2", ServiceConfig{ExecStart: "/bin/test"})

	mock.Reset()

	services := mock.GetServices()
	if len(services) != 0 {
		t.Errorf("expected 0 services after reset, got %d", len(services))
	}
}

func TestMockSystemdManager_DaemonReload(t *testing.T) {
	mock := NewMockSystemdManager("")

	// DaemonReload should not error
	if err := mock.DaemonReload(); err != nil {
		t.Errorf("DaemonReload failed: %v", err)
	}
}

func TestMockSystemdManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock with state dir
	mock1 := NewMockSystemdManager(tmpDir)
	mock1.CreateService("persistent-service", ServiceConfig{
		Description: "Persistent Service",
		ExecStart:   "/bin/test",
	})
	mock1.StartService("persistent-service")
	mock1.EnableService("persistent-service")

	// Create new mock from same state dir
	mock2 := NewMockSystemdManager(tmpDir)

	// State should be loaded
	if !mock2.IsServiceInstalled("persistent-service") {
		t.Error("service should be installed after reload")
	}

	if !mock2.IsServiceActive("persistent-service") {
		t.Error("service should be active after reload")
	}

	if !mock2.IsServiceEnabled("persistent-service") {
		t.Error("service should be enabled after reload")
	}
}

func TestDefaultManager(t *testing.T) {
	// Reset first
	ResetDefaultManager()

	// Get default manager (should be real systemd)
	manager := DefaultManager()
	if manager == nil {
		t.Fatal("DefaultManager returned nil")
	}

	// Should be RealSystemdManager
	if _, ok := manager.(*RealSystemdManager); !ok {
		t.Error("DefaultManager should return RealSystemdManager by default")
	}

	// Override with mock
	mock := NewMockSystemdManager("")
	SetDefaultManager(mock)

	manager2 := DefaultManager()
	if manager2 != mock {
		t.Error("DefaultManager should return the set mock")
	}

	// Reset
	ResetDefaultManager()
	manager3 := DefaultManager()
	if _, ok := manager3.(*RealSystemdManager); !ok {
		t.Error("DefaultManager should return RealSystemdManager after reset")
	}
}

func TestServiceStatus(t *testing.T) {
	// Test status constants
	if StatusRunning != "running" {
		t.Errorf("StatusRunning = %q, want 'running'", StatusRunning)
	}
	if StatusStopped != "stopped" {
		t.Errorf("StatusStopped = %q, want 'stopped'", StatusStopped)
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %q, want 'failed'", StatusFailed)
	}
	if StatusNotFound != "not-found" {
		t.Errorf("StatusNotFound = %q, want 'not-found'", StatusNotFound)
	}
}

func TestMockSystemdManager_Concurrency(t *testing.T) {
	mock := NewMockSystemdManager("")

	// Create services concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			mock.CreateService("service-"+string(rune('0'+n)), ServiceConfig{ExecStart: "/bin/test"})
			mock.StartService("service-" + string(rune('0'+n)))
			time.Sleep(10 * time.Millisecond)
			mock.StopService("service-" + string(rune('0'+n)))
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all services were created
	services := mock.GetServices()
	if len(services) != 10 {
		t.Errorf("expected 10 services, got %d", len(services))
	}
}
