package binary

import (
	"os"
	"testing"
)

func TestGetPath_EnvVarOverride(t *testing.T) {
	mgr := NewManager(t.TempDir())

	// Create a fake binary
	tmpFile := t.TempDir() + "/fake-binary"
	if err := os.WriteFile(tmpFile, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	// Set env var
	os.Setenv("DNSTM_TEST_DNSTT_CLIENT_PATH", tmpFile)
	defer os.Unsetenv("DNSTM_TEST_DNSTT_CLIENT_PATH")

	path, err := mgr.GetPath(BinaryDNSTTClient)
	if err != nil {
		t.Fatalf("GetPath failed: %v", err)
	}

	if path != tmpFile {
		t.Errorf("Expected %s, got %s", tmpFile, path)
	}
}

func TestIsPlatformSupported(t *testing.T) {
	mgr := NewManager(t.TempDir())

	// DNSTT should be supported on current platform (linux/darwin/windows)
	def := DefaultBinaries[BinaryDNSTTClient]
	if !mgr.isPlatformSupported(def) {
		t.Errorf("Expected DNSTT to be supported on %s/%s", mgr.os, mgr.arch)
	}
}

func TestBuildURL(t *testing.T) {
	mgr := &Manager{
		binDir: "/tmp",
		os:     "linux",
		arch:   "amd64",
	}

	tests := []struct {
		binType BinaryType
		want    string
	}{
		{
			BinaryDNSTTClient,
			"https://github.com/net2share/dnstt/releases/download/latest/dnstt-client-linux-amd64",
		},
		{
			BinarySlipstreamServer,
			"https://github.com/net2share/slipstream-rust-build/releases/download/v2026.02.05/slipstream-server-linux-amd64",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.binType), func(t *testing.T) {
			def := DefaultBinaries[tt.binType]
			got := mgr.buildURL(def)
			if got != tt.want {
				t.Errorf("buildURL() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGetShadowsocksArch(t *testing.T) {
	tests := []struct {
		os   string
		arch string
		want string
	}{
		{"linux", "amd64", "x86_64-unknown-linux-gnu"},
		{"linux", "arm64", "aarch64-unknown-linux-gnu"},
		{"darwin", "amd64", "x86_64-apple-darwin"},
		{"darwin", "arm64", "aarch64-apple-darwin"},
	}

	for _, tt := range tests {
		t.Run(tt.os+"-"+tt.arch, func(t *testing.T) {
			mgr := &Manager{os: tt.os, arch: tt.arch}
			got := mgr.getShadowsocksArch()
			if got != tt.want {
				t.Errorf("getShadowsocksArch() = %s, want %s", got, tt.want)
			}
		})
	}
}
