package e2e

import (
	"fmt"
	"os"
	"testing"

	"github.com/net2share/dnstm/internal/testutil"
)

var testBinManager *testutil.TestBinaryManager

func TestMain(m *testing.M) {
	// Set up test binary manager
	testBinManager = testutil.NewTestBinaryManager()

	// Ensure all required binaries are available
	// This will download missing binaries on first run
	if err := testBinManager.EnsureTestBinaries(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up test binaries: %v\n", err)
		fmt.Fprintf(os.Stderr, "Set environment variables to use custom paths:\n")
		fmt.Fprintf(os.Stderr, "  DNSTM_TEST_DNSTT_CLIENT_PATH\n")
		fmt.Fprintf(os.Stderr, "  DNSTM_TEST_SLIPSTREAM_CLIENT_PATH\n")
		fmt.Fprintf(os.Stderr, "  DNSTM_TEST_SSLOCAL_PATH\n")
		fmt.Fprintf(os.Stderr, "  DNSTM_DNSTT_SERVER_PATH\n")
		fmt.Fprintf(os.Stderr, "  DNSTM_SLIPSTREAM_SERVER_PATH\n")
		fmt.Fprintf(os.Stderr, "  DNSTM_SSSERVER_PATH\n")
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func toEnvName(name string) string {
	result := ""
	for _, c := range name {
		if c == '-' {
			result += "_"
		} else if c >= 'a' && c <= 'z' {
			result += string(c - 32)
		} else {
			result += string(c)
		}
	}
	return result
}
