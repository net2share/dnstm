package dnsrouter

import (
	"testing"
)

func TestExtractQueryName(t *testing.T) {
	tests := []struct {
		name     string
		packet   []byte
		expected string
		wantErr  bool
	}{
		{
			name: "simple domain",
			// DNS query for "example.com"
			// Header: 12 bytes (ID=0x1234, standard query, 1 question)
			// Question: 7example3com0 + QTYPE(1) + QCLASS(1)
			packet: []byte{
				0x12, 0x34, // ID
				0x01, 0x00, // Flags: standard query
				0x00, 0x01, // QDCOUNT: 1
				0x00, 0x00, // ANCOUNT: 0
				0x00, 0x00, // NSCOUNT: 0
				0x00, 0x00, // ARCOUNT: 0
				// Question section
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e', // 7 + "example"
				0x03, 'c', 'o', 'm', // 3 + "com"
				0x00,       // null terminator
				0x00, 0x01, // QTYPE: A
				0x00, 0x01, // QCLASS: IN
			},
			expected: "example.com",
			wantErr:  false,
		},
		{
			name: "subdomain",
			// DNS query for "test.example.com"
			packet: []byte{
				0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x04, 't', 'e', 's', 't',
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',
				0x00,
				0x00, 0x01, 0x00, 0x01,
			},
			expected: "test.example.com",
			wantErr:  false,
		},
		{
			name: "mixed case preserved as lowercase",
			// DNS query for "TeSt.ExAmPlE.CoM"
			packet: []byte{
				0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x04, 'T', 'e', 'S', 't',
				0x07, 'E', 'x', 'A', 'm', 'P', 'l', 'E',
				0x03, 'C', 'o', 'M',
				0x00,
				0x00, 0x01, 0x00, 0x01,
			},
			expected: "test.example.com",
			wantErr:  false,
		},
		{
			name:    "packet too short",
			packet:  []byte{0x12, 0x34, 0x01, 0x00},
			wantErr: true,
		},
		{
			name: "no questions",
			packet: []byte{
				0x12, 0x34, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractQueryName(tt.packet)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractQueryName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ExtractQueryName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchDomainSuffix(t *testing.T) {
	tests := []struct {
		query  string
		suffix string
		want   bool
	}{
		{"example.com", "example.com", true},
		{"test.example.com", "example.com", true},
		{"foo.bar.example.com", "example.com", true},
		{"Example.Com", "example.com", true},
		{"test.EXAMPLE.COM", "example.com", true},
		{"other.com", "example.com", false},
		{"notexample.com", "example.com", false},
		{"exampleXcom", "example.com", false},
		{"", "example.com", false},
		{"example.com", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.query+"_"+tt.suffix, func(t *testing.T) {
			got := MatchDomainSuffix(tt.query, tt.suffix)
			if got != tt.want {
				t.Errorf("MatchDomainSuffix(%q, %q) = %v, want %v",
					tt.query, tt.suffix, got, tt.want)
			}
		})
	}
}

func BenchmarkExtractQueryName(b *testing.B) {
	packet := []byte{
		0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x04, 't', 'e', 's', 't',
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,
		0x00, 0x01, 0x00, 0x01,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractQueryName(packet)
	}
}

func BenchmarkMatchDomainSuffix(b *testing.B) {
	query := "test.foo.bar.example.com"
	suffix := "example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchDomainSuffix(query, suffix)
	}
}
