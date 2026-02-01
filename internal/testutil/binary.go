package testutil

import (
	"fmt"

	"github.com/net2share/dnstm/internal/binary"
)

// TestBinaryManager manages binaries for testing.
// It's a thin wrapper around the binary.Manager that auto-detects test environment.
type TestBinaryManager struct {
	manager *binary.Manager
}

// NewTestBinaryManager creates a binary manager for tests.
func NewTestBinaryManager() *TestBinaryManager {
	return &TestBinaryManager{
		manager: binary.NewDefaultManager(),
	}
}

// EnsureTestBinaries ensures all required test binaries are available.
// This should be called once during test setup.
func (m *TestBinaryManager) EnsureTestBinaries() error {
	if err := m.manager.EnsureDir(); err != nil {
		return fmt.Errorf("failed to create test bin dir: %w", err)
	}

	// Required binaries for E2E tests
	required := []binary.BinaryType{
		binary.BinaryDNSTTClient,
		binary.BinaryDNSTTServer,
		binary.BinarySlipstreamClient,
		binary.BinarySlipstreamServer,
		binary.BinarySSLocal,
		binary.BinarySSServer,
		binary.BinaryMicrosocks,
	}

	for _, binType := range required {
		if _, err := m.manager.EnsureInstalled(binType); err != nil {
			return fmt.Errorf("failed to ensure %s: %w", binType, err)
		}
	}

	return nil
}

// GetPath returns the path to a test binary.
func (m *TestBinaryManager) GetPath(binType binary.BinaryType) (string, error) {
	return m.manager.GetPath(binType)
}

// MustGetPath returns the path to a test binary or panics.
func (m *TestBinaryManager) MustGetPath(binType binary.BinaryType) string {
	path, err := m.GetPath(binType)
	if err != nil {
		panic(fmt.Sprintf("failed to get binary %s: %v", binType, err))
	}
	return path
}

// BinDir returns the test binary directory.
func (m *TestBinaryManager) BinDir() string {
	return m.manager.BinDir()
}

// ClientBinaries returns the list of client binary types used in tests.
func ClientBinaries() []binary.BinaryType {
	return []binary.BinaryType{
		binary.BinaryDNSTTClient,
		binary.BinarySlipstreamClient,
		binary.BinarySSLocal,
	}
}

// ServerBinaries returns the list of server binary types used in tests.
func ServerBinaries() []binary.BinaryType {
	return []binary.BinaryType{
		binary.BinaryDNSTTServer,
		binary.BinarySlipstreamServer,
		binary.BinarySSServer,
	}
}
