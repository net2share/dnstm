//go:build linux

package monitor

import (
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
)

var base32Encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

const clientIDLen = 8

// OpenRawSocket creates an AF_PACKET raw socket for sniffing IP packets.
// Must be run as root (needs CAP_NET_RAW).
func OpenRawSocket() (int, error) {
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, int(htons(syscall.ETH_P_IP)))
	if err != nil {
		return -1, fmt.Errorf("failed to create raw socket: %w (are you running as root?)", err)
	}

	// 500ms read timeout for poll-style reads
	tv := syscall.Timeval{Sec: 0, Usec: 500000}
	_ = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)

	return fd, nil
}

// CaptureLoop reads packets from a raw socket and records them in the collector.
// Blocks until stopCh is closed.
func CaptureLoop(fd int, port int, coll *Collector, stopCh <-chan struct{}) {
	buf := make([]byte, 65535)

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK || err == syscall.EINTR {
				continue
			}
			return // socket closed or fatal error
		}
		if n < 20 {
			continue
		}
		processPacket(buf[:n], port, coll)
	}
}

// Capture sniffs DNS traffic for the specified duration and returns results.
// Convenience function for one-shot capture (used by tunnel stats).
func Capture(port int, domains []string, duration time.Duration) (*CaptureResult, error) {
	fd, err := OpenRawSocket()
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)

	coll := NewCollector(domains)
	start := time.Now()
	stopCh := make(chan struct{})

	go func() {
		time.Sleep(duration)
		close(stopCh)
	}()

	CaptureLoop(fd, port, coll, stopCh)

	return coll.Result(time.Since(start)), nil
}

func processPacket(data []byte, port int, coll *Collector) {
	if len(data) < 20 || data[0]>>4 != 4 {
		return
	}

	ihl := int(data[0]&0x0f) * 4
	if ihl < 20 || len(data) < ihl {
		return
	}
	if data[9] != 17 {
		return
	}

	srcIP := net.IPv4(data[12], data[13], data[14], data[15]).String()
	dstIP := net.IPv4(data[16], data[17], data[18], data[19]).String()

	udpData := data[ihl:]
	if len(udpData) < 8 {
		return
	}

	srcPort := binary.BigEndian.Uint16(udpData[0:2])
	dstPort := binary.BigEndian.Uint16(udpData[2:4])
	udpLen := int(binary.BigEndian.Uint16(udpData[4:6]))
	dnsPayload := udpData[8:]

	if len(dnsPayload) < 12 {
		return
	}

	if int(dstPort) == port {
		domain, clientID := extractDnsttQuery(dnsPayload, coll)
		if domain != "" {
			coll.RecordQuery(domain, clientID, srcIP, udpLen)
		}
	} else if int(srcPort) == port {
		domain := extractQueryDomain(dnsPayload)
		if domain != "" {
			coll.RecordResponse(domain, dstIP, udpLen)
		}
	}
}

func extractDnsttQuery(dns []byte, coll *Collector) (string, string) {
	if len(dns) < 12 {
		return "", ""
	}

	labels := parseDNSLabels(dns[12:])
	if len(labels) < 2 {
		return "", ""
	}

	fullDomain := strings.ToLower(strings.Join(labels, "."))

	for tunnelDomain := range coll.Domains {
		suffix := "." + tunnelDomain
		if strings.HasSuffix(fullDomain, suffix) {
			tunnelLabels := strings.Count(tunnelDomain, ".") + 1
			if len(labels) <= tunnelLabels {
				continue
			}
			prefixLabels := labels[:len(labels)-tunnelLabels]
			encoded := strings.ToUpper(strings.Join(prefixLabels, ""))

			decoded := make([]byte, base32Encoding.DecodedLen(len(encoded)))
			n, err := base32Encoding.Decode(decoded, []byte(encoded))
			if err != nil || n < clientIDLen {
				return tunnelDomain, ""
			}

			clientID := fmt.Sprintf("%x", decoded[:clientIDLen])
			return tunnelDomain, clientID
		}

		if fullDomain == tunnelDomain {
			return tunnelDomain, ""
		}
	}

	return "", ""
}

func extractQueryDomain(dns []byte) string {
	if len(dns) < 12 {
		return ""
	}
	labels := parseDNSLabels(dns[12:])
	if len(labels) == 0 {
		return ""
	}
	return strings.ToLower(strings.Join(labels, "."))
}

func parseDNSLabels(data []byte) []string {
	var labels []string
	offset := 0

	for offset < len(data) {
		labelLen := int(data[offset])
		if labelLen == 0 {
			break
		}
		if labelLen&0xc0 == 0xc0 {
			break
		}
		offset++
		if offset+labelLen > len(data) {
			return nil
		}
		labels = append(labels, string(data[offset:offset+labelLen]))
		offset += labelLen
	}

	return labels
}

func htons(v uint16) uint16 {
	return (v << 8) | (v >> 8)
}
