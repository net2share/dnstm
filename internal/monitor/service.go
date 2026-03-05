package monitor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// RunDir is where stats JSON and PID files are stored.
// Lives under the config directory so it works cross-platform.
const RunDir = "/etc/dnstm/run"

// SnifferName returns a label for a tunnel's sniffer process.
func SnifferName(tag string) string {
	return tag + "-monitor"
}

// pidFilePath returns the PID file path for a tunnel's sniffer.
func pidFilePath(tag string) string {
	return filepath.Join(RunDir, tag+"-monitor.pid")
}

// binaryPath returns the path to the currently running dnstm binary.
// Falls back to looking in PATH.
func binaryPath() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		// Resolve symlinks
		exe, err = filepath.EvalSymlinks(exe)
		if err == nil {
			return exe, nil
		}
	}
	// Fallback: find in PATH
	return exec.LookPath("dnstm")
}

// metricsConfPath returns the path to the metrics config file for a tunnel.
func metricsConfPath(tag string) string {
	return filepath.Join(RunDir, tag+"-metrics.conf")
}

// WriteMetricsConf persists the metrics address so the TUI can read it.
func WriteMetricsConf(tag, addr string) error {
	if addr == "" {
		os.Remove(metricsConfPath(tag))
		return nil
	}
	return os.WriteFile(metricsConfPath(tag), []byte(addr), 0644)
}

// ReadMetricsConf returns the persisted metrics address for a tunnel (empty if none).
func ReadMetricsConf(tag string) string {
	data, err := os.ReadFile(metricsConfPath(tag))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// StartSniffer launches "dnstm sniff" as a detached background process.
// It writes a PID file so we can check status and stop it later.
// If metricsAddr is non-empty, the sniffer will serve Prometheus metrics on that address.
func StartSniffer(tag string, domains []string, metricsAddr string) error {
	if IsSnifferRunning(tag) {
		return nil // already running
	}

	bin, err := binaryPath()
	if err != nil {
		return fmt.Errorf("cannot find dnstm binary: %w", err)
	}

	if err := os.MkdirAll(RunDir, 0755); err != nil {
		return fmt.Errorf("failed to create run directory: %w", err)
	}

	args := []string{"sniff", "--tag", tag}
	if metricsAddr != "" {
		args = append(args, "--metrics-address", metricsAddr)
	}
	args = append(args, domains...)

	// Persist metrics config so other commands can discover it
	_ = WriteMetricsConf(tag, metricsAddr)

	cmd := exec.Command(bin, args...)
	// Detach: new process group, no stdin/stdout/stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Direct logs to a file so we can debug if needed
	logFile := filepath.Join(RunDir, tag+"-monitor.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		cmd.Stdout = f
		cmd.Stderr = f
	}

	if err := cmd.Start(); err != nil {
		if f != nil {
			f.Close()
		}
		return fmt.Errorf("failed to start sniffer: %w", err)
	}

	// Write PID file
	pid := cmd.Process.Pid
	if err := os.WriteFile(pidFilePath(tag), []byte(strconv.Itoa(pid)), 0644); err != nil {
		// Kill the process we just started since we can't track it
		_ = cmd.Process.Kill()
		if f != nil {
			f.Close()
		}
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Release the process so it survives our exit
	_ = cmd.Process.Release()
	// Don't close the log file — the child process is using it
	return nil
}

// StopSniffer stops a running sniffer process by sending SIGTERM.
func StopSniffer(tag string) error {
	pid, err := readPid(tag)
	if err != nil {
		return nil // no PID file = not running
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		cleanupPid(tag)
		return nil
	}

	// Send SIGTERM for graceful shutdown (final stats write)
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		cleanupPid(tag)
		return nil // process already gone
	}

	cleanupPid(tag)
	return nil
}

// IsSnifferRunning checks if the sniffer process is alive.
func IsSnifferRunning(tag string) bool {
	pid, err := readPid(tag)
	if err != nil {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal 0 checks if process exists without actually sending a signal
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		cleanupPid(tag)
		return false
	}
	return true
}

// RemoveSniffer stops the sniffer and cleans up all its files.
func RemoveSniffer(tag string) error {
	_ = StopSniffer(tag)
	// Clean up files
	os.Remove(pidFilePath(tag))
	os.Remove(StatsFilePath(tag))
	os.Remove(metricsConfPath(tag))
	os.Remove(filepath.Join(RunDir, tag+"-monitor.log"))
	return nil
}

func readPid(tag string) (int, error) {
	data, err := os.ReadFile(pidFilePath(tag))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func cleanupPid(tag string) {
	os.Remove(pidFilePath(tag))
}
