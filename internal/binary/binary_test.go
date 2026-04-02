package binary

import (
	"os"
	"runtime"
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

func TestToBinmanDef(t *testing.T) {
	// Test that toBinmanDef correctly converts local BinaryDef to binman.BinaryDef
	def := DefaultBinaries[BinaryDNSTTClient]
	bd := toBinmanDef(def)

	if bd.Name != string(BinaryDNSTTClient) {
		t.Errorf("Name = %s, want %s", bd.Name, string(BinaryDNSTTClient))
	}
	if bd.EnvOverride != def.EnvVar {
		t.Errorf("EnvOverride = %s, want %s", bd.EnvOverride, def.EnvVar)
	}
	if bd.URLPattern != def.URLPattern {
		t.Errorf("URLPattern = %s, want %s", bd.URLPattern, def.URLPattern)
	}
}

func TestToBinmanDef_Archive(t *testing.T) {
	def := DefaultBinaries[BinarySSServer]
	bd := toBinmanDef(def)

	if bd.ArchiveType != "tar.xz" {
		t.Errorf("ArchiveType = %s, want tar.xz", bd.ArchiveType)
	}
}

func TestArchMappings_Shadowsocks(t *testing.T) {
	def := DefaultBinaries[BinarySSServer]
	if def.archMappings == nil {
		t.Fatal("SSServer archMappings should be populated by init()")
	}

	ssarch, ok := def.archMappings["ssarch"]
	if !ok {
		t.Fatal("SSServer should have ssarch mapping")
	}

	expected := map[string]string{
		"linux/amd64":  "x86_64-unknown-linux-gnu",
		"linux/arm64":  "aarch64-unknown-linux-gnu",
		"darwin/amd64": "x86_64-apple-darwin",
		"darwin/arm64": "aarch64-apple-darwin",
	}

	for platform, want := range expected {
		if got := ssarch[platform]; got != want {
			t.Errorf("ssarch[%s] = %s, want %s", platform, got, want)
		}
	}
}

func TestArchMappings_Microsocks(t *testing.T) {
	def := DefaultBinaries[BinaryMicrosocks]
	if def.archMappings == nil {
		t.Fatal("Microsocks archMappings should be populated by init()")
	}

	msarch, ok := def.archMappings["microsocksarch"]
	if !ok {
		t.Fatal("Microsocks should have microsocksarch mapping")
	}

	// Should have at least linux/amd64 mapping
	if _, ok := msarch["linux/amd64"]; !ok {
		t.Error("Microsocks should have linux/amd64 mapping")
	}
}

func TestServerBinaries(t *testing.T) {
	defs := ServerBinaries()
	if len(defs) != 6 {
		t.Errorf("ServerBinaries() returned %d, want 6", len(defs))
	}

	// Check VayDNS is included
	found := false
	for _, def := range defs {
		if def.Type == BinaryVayDNSServer {
			found = true
			break
		}
	}
	if !found {
		t.Error("ServerBinaries() should include VayDNS")
	}
}

func TestChecksumURLs(t *testing.T) {
	// Verify all server binaries except sshtun-user have checksum URLs
	for _, def := range ServerBinaries() {
		if def.Type == BinarySSHTunUser {
			if def.ChecksumURL != "" {
				t.Errorf("SSHTunUser should have no ChecksumURL, got %s", def.ChecksumURL)
			}
			continue
		}
		if def.ChecksumURL == "" {
			t.Errorf("%s should have a ChecksumURL", def.Type)
		}
	}
}

func TestDetectLibc(t *testing.T) {
	// detectLibc should return either "glibc" or "musl"
	libc := detectLibc()
	if libc != "glibc" && libc != "musl" {
		t.Errorf("detectLibc() = %s, want glibc or musl", libc)
	}
}

func TestPlatformSupport(t *testing.T) {
	mgr := NewManager(t.TempDir())

	// DNSTT should be supported on current platform
	_, err := mgr.GetPath(BinaryDNSTTClient)
	// Error is expected (binary not found), but should NOT be "not supported"
	if err != nil {
		platform := runtime.GOOS + "/" + runtime.GOARCH
		if err.Error() == "binary dnstt-client not supported on "+platform {
			t.Errorf("DNSTT should be supported on %s", platform)
		}
	}
}
