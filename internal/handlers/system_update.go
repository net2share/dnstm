package handlers

import (
	"fmt"
	"os"
	"strings"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/dnstm/internal/updater"
	"github.com/net2share/dnstm/internal/version"
)

func init() {
	actions.SetSystemHandler(actions.ActionUpdate, HandleUpdate)
}

// HandleUpdate handles the update action.
func HandleUpdate(ctx *actions.Context) error {
	force := ctx.GetBool("force")
	selfOnly := ctx.GetBool("self")
	binariesOnly := ctx.GetBool("binaries")
	checkOnly := ctx.GetBool("check")

	opts := updater.UpdateOptions{
		Force:        force,
		SelfOnly:     selfOnly,
		BinariesOnly: binariesOnly,
		DryRun:       checkOnly,
	}

	currentVersion := version.Version

	// Phase 1: Check for updates (in progress view for TUI)
	if ctx.IsInteractive {
		ctx.Output.BeginProgress("Update")
	}
	ctx.Output.Info("Checking for updates...")

	report, err := updater.CheckForUpdates(currentVersion, opts)
	if err != nil {
		if ctx.IsInteractive {
			ctx.Output.EndProgress()
		}
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !report.HasUpdates() {
		if len(report.Warnings) > 0 {
			for _, w := range report.Warnings {
				ctx.Output.Warning(w)
			}
			ctx.Output.Warning("Could not check for updates")
		} else {
			ctx.Output.Status("Everything is up to date")
		}
		if ctx.IsInteractive {
			ctx.Output.EndProgress()
		}
		return nil
	}

	// Display available updates
	displayUpdateReport(ctx, report)

	// If dry-run, stop here
	if checkOnly {
		if ctx.IsInteractive {
			ctx.Output.EndProgress()
		}
		return nil
	}

	// Require --force to install updates
	if !force {
		if ctx.IsInteractive {
			ctx.Output.EndProgress()
		}
		ctx.Output.Info("Run with --force to install updates")
		return nil
	}

	// Phase 2: Perform updates (in progress view for TUI)
	if ctx.IsInteractive {
		ctx.Output.BeginProgress("Updating")
	} else {
		ctx.Output.Println()
	}

	statusFn := func(msg string) {
		ctx.Output.Status(msg)
	}

	// Perform dnstm self-update (if needed and not binaries-only)
	if report.DnstmUpdate != nil && !binariesOnly {
		if err := updater.PerformSelfUpdate(report.DnstmUpdate.Latest, statusFn); err != nil {
			if ctx.IsInteractive {
				ctx.Output.EndProgress()
			}
			return fmt.Errorf("self-update failed: %w", err)
		}
	}

	// Perform binary updates (if needed and not self-only)
	if len(report.BinaryUpdates) > 0 && !selfOnly {
		if err := updater.PerformBinaryUpdates(report.BinaryUpdates, statusFn); err != nil {
			if ctx.IsInteractive {
				ctx.Output.EndProgress()
			}
			return fmt.Errorf("binary update failed: %w", err)
		}
	}

	ctx.Output.Success("Update completed successfully")

	// If dnstm itself was updated, exit so the user runs the new version
	if report.DnstmUpdate != nil && !binariesOnly {
		ctx.Output.Info("Please restart dnstm to use the new version")
		if ctx.IsInteractive {
			ctx.Output.EndProgress()
		}
		os.Exit(0)
	}

	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	}
	return nil
}

// displayUpdateReport shows available updates to the user.
func displayUpdateReport(ctx *actions.Context, report *updater.UpdateReport) {
	ctx.Output.Println()
	ctx.Output.Info("Available updates:")

	if report.DnstmUpdate != nil {
		ctx.Output.Status(fmt.Sprintf("dnstm: %s -> %s",
			report.DnstmUpdate.Current,
			report.DnstmUpdate.Latest))
	}

	for _, update := range report.BinaryUpdates {
		current := update.CurrentVersion
		if current == "" {
			current = "(unknown)"
		}
		ctx.Output.Status(fmt.Sprintf("%s: %s -> %s",
			update.Binary,
			current,
			update.LatestVersion))

		if len(update.AffectedServices) > 0 {
			ctx.Output.Info(fmt.Sprintf("  will restart: %s", strings.Join(update.AffectedServices, ", ")))
		}
	}
}

