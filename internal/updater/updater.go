package updater

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/net2share/dnstm/internal/binary"
)

// UpdateOptions configures the update behavior.
type UpdateOptions struct {
	Force        bool // Skip confirmation prompts
	SelfOnly     bool // Only update dnstm
	BinariesOnly bool // Only update transport binaries
	DryRun       bool // Check only, don't update
}

// UpdateReport contains information about available updates.
type UpdateReport struct {
	DnstmUpdate   *VersionInfo
	BinaryUpdates []BinaryUpdate
	Warnings      []string
}

// VersionInfo contains version details.
type VersionInfo struct {
	Current string
	Latest  string
}

// BinaryUpdate contains update information for a binary.
type BinaryUpdate struct {
	Binary           binary.BinaryType
	CurrentVersion   string
	LatestVersion    string
	AffectedServices []string
}

// HasUpdates returns true if there are any available updates.
func (r *UpdateReport) HasUpdates() bool {
	return r.DnstmUpdate != nil || len(r.BinaryUpdates) > 0
}

// UpdateCount returns the total number of available updates.
func (r *UpdateReport) UpdateCount() int {
	count := len(r.BinaryUpdates)
	if r.DnstmUpdate != nil {
		count++
	}
	return count
}

// StatusFunc is a callback for reporting status messages.
type StatusFunc func(message string)

// CheckForUpdates checks for available updates without applying them.
func CheckForUpdates(currentDnstmVersion string, opts UpdateOptions) (*UpdateReport, error) {
	report := &UpdateReport{}

	// Check dnstm updates
	if !opts.BinariesOnly {
		latestDnstm, err := GetDnstmLatestVersion()
		if err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("Failed to check dnstm version: %v", err))
		} else if IsNewer(currentDnstmVersion, latestDnstm) {
			report.DnstmUpdate = &VersionInfo{
				Current: currentDnstmVersion,
				Latest:  latestDnstm,
			}
		}
	}

	// Check binary updates
	if !opts.SelfOnly {
		manifest, err := LoadManifest()
		if err != nil {
			manifest = &VersionManifest{}
		}

		report.BinaryUpdates = checkBinaryUpdates(manifest)
	}

	return report, nil
}

// checkBinaryUpdates compares installed versions against pinned versions in the codebase.
func checkBinaryUpdates(manifest *VersionManifest) []BinaryUpdate {
	var updates []BinaryUpdate

	binariesToCheck := []binary.BinaryType{
		binary.BinarySlipstreamServer,
		binary.BinarySSServer,
		binary.BinaryMicrosocks,
		binary.BinarySSHTunUser,
	}

	for _, binType := range binariesToCheck {
		def, ok := binary.GetDef(binType)
		if !ok || def.SkipUpdate || def.PinnedVersion == "" {
			continue
		}

		currentVersion := manifest.GetVersion(string(binType))

		if IsNewer(currentVersion, def.PinnedVersion) {
			affectedServices := GetActiveServicesForBinary(binType)
			updates = append(updates, BinaryUpdate{
				Binary:           binType,
				CurrentVersion:   currentVersion,
				LatestVersion:    def.PinnedVersion,
				AffectedServices: affectedServices,
			})
		}
	}

	return updates
}

// PerformSelfUpdate downloads and replaces the dnstm binary on disk.
// The caller continues updating binaries in the current process.
func PerformSelfUpdate(latestVersion string, statusFn StatusFunc) error {
	if statusFn != nil {
		statusFn(fmt.Sprintf("Downloading dnstm %s...", latestVersion))
	}

	// Determine download URL
	arch := runtime.GOARCH
	osName := runtime.GOOS
	downloadURL := fmt.Sprintf(
		"https://github.com/net2share/dnstm/releases/download/%s/dnstm-%s-%s",
		latestVersion, osName, arch,
	)

	// Download to temp file
	tmpFile, err := os.CreateTemp("", "dnstm-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := http.Get(downloadURL)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Get the target path
	targetPath := "/usr/local/bin/dnstm"
	currentExe, err := os.Executable()
	if err == nil {
		// Use the same path as current executable if possible
		resolved, err := filepath.EvalSymlinks(currentExe)
		if err == nil && filepath.Dir(resolved) == "/usr/local/bin" {
			targetPath = resolved
		}
	}

	if statusFn != nil {
		statusFn("Installing new version...")
	}

	// Remove the running binary first (Linux keeps it in memory for the
	// current process but frees the path for a new file).
	os.Remove(targetPath)

	// Move new binary into place
	if err := os.Rename(tmpPath, targetPath); err != nil {
		// Rename failed (cross-device), try copy
		if err := copyFile(tmpPath, targetPath); err != nil {
			return fmt.Errorf("failed to install binary: %w", err)
		}
	}

	if statusFn != nil {
		statusFn(fmt.Sprintf("dnstm updated to %s", latestVersion))
	}

	return nil
}

// PerformBinaryUpdates updates the specified binaries.
func PerformBinaryUpdates(updates []BinaryUpdate, statusFn StatusFunc) error {
	if len(updates) == 0 {
		return nil
	}

	// Collect all services that need to be stopped
	var servicesToStop []string
	for _, update := range updates {
		servicesToStop = append(servicesToStop, update.AffectedServices...)
	}

	// Stop affected services
	var stoppedServices []string
	if len(servicesToStop) > 0 {
		if statusFn != nil {
			statusFn(fmt.Sprintf("Stopping %d affected service(s)...", len(servicesToStop)))
		}
		stoppedServices = StopServices(servicesToStop)
	}

	// Update each binary
	manifest, _ := LoadManifest()
	if manifest == nil {
		manifest = &VersionManifest{}
	}

	mgr := binary.NewDefaultManager()

	for _, update := range updates {
		if statusFn != nil {
			statusFn(fmt.Sprintf("Updating %s to %s...", update.Binary, update.LatestVersion))
		}

		// Download the new version
		if err := mgr.DownloadVersion(update.Binary, update.LatestVersion); err != nil {
			if statusFn != nil {
				statusFn(fmt.Sprintf("Failed to update %s: %v", update.Binary, err))
			}
			continue
		}

		// Update manifest
		manifest.SetVersion(string(update.Binary), update.LatestVersion)
	}

	// Save manifest
	if err := manifest.Save(); err != nil {
		if statusFn != nil {
			statusFn(fmt.Sprintf("Warning: failed to update version manifest: %v", err))
		}
	}

	// Restart stopped services
	if len(stoppedServices) > 0 {
		if statusFn != nil {
			statusFn(fmt.Sprintf("Restarting %d service(s)...", len(stoppedServices)))
		}
		if err := StartServices(stoppedServices); err != nil {
			if statusFn != nil {
				statusFn(fmt.Sprintf("Warning: failed to restart some services: %v", err))
			}
		}
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	return err
}
