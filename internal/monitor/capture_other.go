//go:build !linux

package monitor

import (
	"fmt"
	"runtime"
	"time"
)

// OpenRawSocket is not supported on non-Linux platforms.
func OpenRawSocket() (int, error) {
	return -1, fmt.Errorf("raw socket capture requires Linux (current: %s)", runtime.GOOS)
}

// CaptureLoop is not supported on non-Linux platforms.
func CaptureLoop(fd int, port int, coll *Collector, stopCh <-chan struct{}) {
}

// Capture is not supported on non-Linux platforms.
func Capture(port int, domains []string, duration time.Duration) (*CaptureResult, error) {
	return nil, fmt.Errorf("packet capture requires Linux (current: %s)", runtime.GOOS)
}
