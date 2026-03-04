package clientcfg

import (
	"encoding/base64"
	"strings"
	"testing"
)

const fakeCertPEM = "-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----\n"
const fakeKeyPEM = "-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n-----END OPENSSH PRIVATE KEY-----\n"

func TestRoundTrip_SlipstreamSocks(t *testing.T) {
	original := &ClientConfig{
		Version: 1,
		Tag:     "slip-main",
		Transport: TransportConfig{
			Type:   "slipstream",
			Domain: "a.puzzleapp.store",
			Cert:   fakeCertPEM,
		},
		Backend: BackendConfig{
			Type: "socks",
		},
	}

	url, err := Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	if !strings.HasPrefix(url, "dnst://") {
		t.Fatalf("URL missing prefix: %s", url)
	}

	decoded, err := Decode(url)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.Version != original.Version {
		t.Errorf("version: got %d, want %d", decoded.Version, original.Version)
	}
	if decoded.Tag != original.Tag {
		t.Errorf("tag: got %q, want %q", decoded.Tag, original.Tag)
	}
	if decoded.Transport.Type != original.Transport.Type {
		t.Errorf("transport.type: got %q, want %q", decoded.Transport.Type, original.Transport.Type)
	}
	if decoded.Transport.Domain != original.Transport.Domain {
		t.Errorf("transport.domain: got %q, want %q", decoded.Transport.Domain, original.Transport.Domain)
	}
	if decoded.Transport.Cert != original.Transport.Cert {
		t.Errorf("transport.cert mismatch")
	}
	if decoded.Backend.Type != original.Backend.Type {
		t.Errorf("backend.type: got %q, want %q", decoded.Backend.Type, original.Backend.Type)
	}
}

func TestRoundTrip_DNSTTSSH(t *testing.T) {
	original := &ClientConfig{
		Version: 1,
		Tag:     "dnstt-ssh",
		Transport: TransportConfig{
			Type:   "dnstt",
			Domain: "q.appai.my",
			PubKey: "c6a85f970db1ad8afb1a1a910a4a2b276e68fa6c2e11c1ad1883e1c8f5c3a1b2",
		},
		Backend: BackendConfig{
			Type:     "ssh",
			User:     "root",
			Password: "secret",
		},
	}

	url, err := Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decoded, err := Decode(url)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.Transport.PubKey != original.Transport.PubKey {
		t.Errorf("transport.pubkey: got %q, want %q", decoded.Transport.PubKey, original.Transport.PubKey)
	}
	if decoded.Backend.User != original.Backend.User {
		t.Errorf("backend.user: got %q, want %q", decoded.Backend.User, original.Backend.User)
	}
	if decoded.Backend.Password != original.Backend.Password {
		t.Errorf("backend.password: got %q, want %q", decoded.Backend.Password, original.Backend.Password)
	}
}

func TestRoundTrip_SlipstreamShadowsocks(t *testing.T) {
	original := &ClientConfig{
		Version: 1,
		Tag:     "slip-ss",
		Transport: TransportConfig{
			Type:   "slipstream",
			Domain: "z.appai.my",
			Cert:   fakeCertPEM,
		},
		Backend: BackendConfig{
			Type:     "shadowsocks",
			Method:   "chacha20-ietf-poly1305",
			Password: "newpass456",
		},
	}

	url, err := Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decoded, err := Decode(url)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.Backend.Method != original.Backend.Method {
		t.Errorf("backend.method: got %q, want %q", decoded.Backend.Method, original.Backend.Method)
	}
	if decoded.Backend.Password != original.Backend.Password {
		t.Errorf("backend.password: got %q, want %q", decoded.Backend.Password, original.Backend.Password)
	}
}

func TestRoundTrip_SSHWithKey(t *testing.T) {
	original := &ClientConfig{
		Version: 1,
		Tag:     "dnstt-ssh-key",
		Transport: TransportConfig{
			Type:   "dnstt",
			Domain: "q.appai.my",
			PubKey: "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		},
		Backend: BackendConfig{
			Type: "ssh",
			User: "root",
			Key:  fakeKeyPEM,
		},
	}

	url, err := Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decoded, err := Decode(url)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.Backend.Key != original.Backend.Key {
		t.Errorf("backend.key: got %q, want %q", decoded.Backend.Key, original.Backend.Key)
	}
	if decoded.Backend.Password != "" {
		t.Errorf("backend.password should be empty, got %q", decoded.Backend.Password)
	}
}

func TestDecode_InvalidPrefix(t *testing.T) {
	_, err := Decode("https://example.com")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestDecode_EmptyPayload(t *testing.T) {
	_, err := Decode("dnst://")
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestDecode_InvalidBase64(t *testing.T) {
	_, err := Decode("dnst://!!!invalid!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecode_InvalidJSON(t *testing.T) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	_, err := Decode("dnst://" + encoded)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDecode_UnsupportedVersion(t *testing.T) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(`{"v":0}`))
	_, err := Decode("dnst://" + encoded)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestEncode_NilConfig(t *testing.T) {
	_, err := Encode(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}
