package integration

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Set up test environment
	os.Setenv("DNSTM_TEST_MODE", "1")

	// Run tests
	code := m.Run()

	os.Exit(code)
}
