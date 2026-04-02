package updater

import (
	"fmt"

	"github.com/net2share/dnstm/internal/binary"
	"github.com/net2share/go-corelib/binman"
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

// GetDnstmLatestVersion fetches the latest dnstm release version from GitHub.
func GetDnstmLatestVersion() (string, error) {
	release, err := binman.GetLatestRelease("net2share/dnstm")
	if err != nil {
		return "", err
	}
	return release.TagName, nil
}

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
			manifest = NewManifest()
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
		binary.BinaryVayDNSServer,
		binary.BinarySlipstreamPlusServer,
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
func PerformSelfUpdate(latestVersion string, statusFn StatusFunc) error {
	return binman.SelfUpdate(binman.SelfUpdateConfig{
		Repo:       "net2share/dnstm",
		URLPattern: "https://github.com/net2share/dnstm/releases/download/{version}/dnstm-{os}-{arch}",
		StatusFn:   binman.StatusFunc(statusFn),
	}, latestVersion)
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
		manifest = NewManifest()
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
