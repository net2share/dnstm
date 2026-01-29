package dnsrouter

import (
	"errors"
	"strings"
)

// DNS header is 12 bytes, followed by the question section
const dnsHeaderSize = 12

var (
	ErrPacketTooShort   = errors.New("packet too short")
	ErrInvalidLabel     = errors.New("invalid DNS label")
	ErrLabelTooLong     = errors.New("DNS label too long")
	ErrNameTooLong      = errors.New("DNS name too long")
	ErrPointerLoop      = errors.New("DNS pointer loop detected")
	ErrNoQuestionSection = errors.New("no question section")
)

// ExtractQueryName extracts the query name from a raw DNS packet.
// It performs minimal parsing - just reads the first question's QNAME.
// This function does NOT modify the packet in any way.
func ExtractQueryName(packet []byte) (string, error) {
	if len(packet) < dnsHeaderSize+1 {
		return "", ErrPacketTooShort
	}

	// Check QDCOUNT (bytes 4-5) - must be at least 1
	qdcount := int(packet[4])<<8 | int(packet[5])
	if qdcount == 0 {
		return "", ErrNoQuestionSection
	}

	// Parse the first question's QNAME starting at byte 12
	name, _, err := parseName(packet, dnsHeaderSize)
	if err != nil {
		return "", err
	}

	return strings.ToLower(name), nil
}

// parseName parses a DNS name at the given offset.
// Returns the name and the offset after the name.
func parseName(packet []byte, offset int) (string, int, error) {
	var labels []string
	visited := make(map[int]bool)
	origOffset := offset
	jumped := false

	for {
		if offset >= len(packet) {
			return "", 0, ErrPacketTooShort
		}

		// Check for pointer loop
		if visited[offset] {
			return "", 0, ErrPointerLoop
		}
		visited[offset] = true

		length := int(packet[offset])

		// End of name
		if length == 0 {
			if !jumped {
				origOffset = offset + 1
			}
			break
		}

		// Check for pointer (compression)
		if length&0xC0 == 0xC0 {
			if offset+1 >= len(packet) {
				return "", 0, ErrPacketTooShort
			}
			pointer := int(packet[offset]&0x3F)<<8 | int(packet[offset+1])
			if !jumped {
				origOffset = offset + 2
			}
			offset = pointer
			jumped = true
			continue
		}

		// Regular label
		if length > 63 {
			return "", 0, ErrLabelTooLong
		}

		offset++
		if offset+length > len(packet) {
			return "", 0, ErrPacketTooShort
		}

		labels = append(labels, string(packet[offset:offset+length]))
		offset += length

		// Note: We don't enforce the 253-char limit here because:
		// 1. DNS tunneling protocols (DNSTT, slipstream) encode data in long names
		// 2. We only need to extract the domain suffix for routing
		// 3. The packet bounds check ensures we don't read beyond the packet
	}

	return strings.Join(labels, "."), origOffset, nil
}

// MatchDomainSuffix checks if the query name matches a domain suffix.
// For example, "test.example.com" matches suffix "example.com".
func MatchDomainSuffix(queryName, suffix string) bool {
	queryName = strings.ToLower(queryName)
	suffix = strings.ToLower(suffix)

	// Exact match
	if queryName == suffix {
		return true
	}

	// Check if queryName ends with ".suffix"
	if strings.HasSuffix(queryName, "."+suffix) {
		return true
	}

	return false
}
