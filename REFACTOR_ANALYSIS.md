# Cross-Project Refactor Analysis

## Summary

This document analyzes the refactoring performed across three related projects to ensure consistency and DRY (Don't Repeat Yourself) compliance.

---

## Project Overview

| Project | Current Branch | Description |
|---------|---------------|-------------|
| `dnstm` | `main` | DNS Tunnel Manager CLI |
| `go-corelib` | `main` | Shared Go utilities library |
| `sshtun-user` | `main` | SSH Tunnel User Manager |

---

## Key Changes Per Project

### 1. go-corelib

**Changes:**
- **NEW** `tui/` package with Lipgloss-based styling:
  - `theme.go` - Shared color theme (Primary, Secondary, Success, Error, Warning, Info, Muted)
  - `styles.go` - Pre-configured lipgloss styles (TitleStyle, SuccessStyle, etc.)
  - `format.go` - Text formatting utilities (Bold, Code, Highlight)
  - `print.go` - Print functions (PrintSuccess, PrintError, PrintWarning, PrintInfo, PrintStatus)
  - `box.go` - Box rendering with lipgloss
  - `helpers.go` - Terminal helpers (WaitForEnter, ClearLine)
  - `progress.go` - Progress/spinner utilities
  - `banner.go` - Consistent banner rendering with BannerConfig
- **NEW** `RequireRoot()` and `ErrNotRoot` in `osdetect` package for standardized root checks
- **REMOVED** legacy fatih/color-based tui code (colors.go, prompt.go, types.go)
- Added `charmbracelet/lipgloss` dependency
- Updated README and release config

**Impact:** Shared TUI library now uses modern Lipgloss styling, providing consistent UI across all consuming projects.

---

### 2. dnstm

**Changes:**
- **NEW** Cobra CLI structure (`cmd/*.go`):
  - `root.go` - Root command with interactive mode fallback
  - `install.go` - Installation command
  - `status.go` - Service status command
  - `logs.go` - Log viewing command
  - `config.go` - Configuration command
  - `restart.go` - Service restart command
  - `uninstall.go` - Uninstall command
  - `ssh_users.go` - SSH user management command
- **NEW** `internal/installer/` package with `PrintBanner()` for app-specific ASCII art
- **NEW** `internal/menu/` package for interactive mode
- **USES** `go-corelib/tui` for all UI rendering (no local internal/ui)
- **USES** `osdetect.RequireRoot()` for standardized root checks
- **DELETED** `internal/app/app.go` (monolithic file)
- Upgraded to Go 1.23.0
- Added `charmbracelet/huh` for interactive prompts
- Added `spf13/cobra` for CLI

---

### 3. sshtun-user

**Changes:**
- **NEW** Cobra CLI structure (`cmd/*.go`):
  - `root.go` - Root command with interactive mode fallback
  - `create.go` - Create user command
  - `update.go` - Update user command
  - `list.go` - List users command
  - `delete.go` - Delete user command
  - `configure.go` - Configure sshd command
  - `uninstall.go` - Uninstall command
- **NEW** `internal/menu/main.go` and `internal/menu/embedded.go` for interactive and embedded modes
- **NEW** `pkg/cli/api.go` - Public API for dnstm integration
- **USES** `go-corelib/tui` for all UI rendering (no local internal/ui)
- **USES** `osdetect.RequireRoot()` for standardized root checks
- **DELETED** `cmd/sshtun-user/main.go`, `internal/cli/cli.go`, `pkg/cli/*.go` (old structure)
- Upgraded to Go 1.23.0
- Added `charmbracelet/huh` for interactive prompts
- Added `spf13/cobra` for CLI

---

## DRY Compliance: Achieved

The common UI code has been extracted to `go-corelib/tui`:

```go
import "github.com/net2share/go-corelib/tui"

// Shared across all projects:
tui.PrintSuccess("Done!")
tui.PrintError("Failed")
tui.PrintWarning("Caution")
tui.PrintInfo("Note")
tui.PrintStatus("Processing...")
tui.PrintBox("Title", lines)
tui.PrintBanner(tui.BannerConfig{...})
tui.PrintSimpleBanner("App Name", version, buildTime)
```

### Root Check Standardization

All projects now use the same root check:
```go
import "github.com/net2share/go-corelib/osdetect"

if err := osdetect.RequireRoot(); err != nil {
    return err  // Returns: "this program must be run as root"
}
```

---

## Consistency Check

| Aspect | dnstm | sshtun-user | go-corelib |
|--------|-------|-------------|------------|
| CLI framework | spf13/cobra | spf13/cobra | N/A |
| UI framework | go-corelib/tui | go-corelib/tui | lipgloss |
| Prompts | charmbracelet/huh | charmbracelet/huh | N/A |
| Go version | 1.23.0 | 1.23.0 | 1.23.0 |
| Root check | osdetect.RequireRoot() | osdetect.RequireRoot() | Provides |
| Project structure | cmd/, internal/ | cmd/, internal/, pkg/ | Package-based |

---

## Suggested Branch Names

| Project | Suggested Branch Name |
|---------|----------------------|
| **go-corelib** | `feat/tui-lipgloss-refactor` |
| **dnstm** | `feat/cobra-cli-refactor` |
| **sshtun-user** | `feat/cobra-cli-refactor` |

---

## Suggested Commit Messages

### go-corelib
```
feat(tui): migrate to lipgloss and add RequireRoot

- Refactor tui package to use charmbracelet/lipgloss instead of fatih/color
- Add theme.go with consistent color palette across consuming projects
- Add banner.go with BannerConfig for standardized app banners
- Add format.go with Bold, Code, Highlight text formatting
- Add RequireRoot() and ErrNotRoot to osdetect for standardized root checks
- Remove legacy fatih/color-based code (colors.go, prompt.go, types.go)

This provides a shared UI foundation for dnstm and sshtun-user.
```

### dnstm
```
feat: rewrite CLI with Cobra and shared go-corelib/tui

- Add Cobra-based CLI with subcommands (install, status, logs, config, restart, uninstall, ssh-users)
- Use go-corelib/tui for all terminal UI (consistent with sshtun-user)
- Use go-corelib/osdetect.RequireRoot() for standardized root checks
- Add charmbracelet/huh for interactive prompts
- Refactor monolithic app.go into modular packages:
  - cmd/ for CLI commands
  - internal/installer/ for installation logic
  - internal/menu/ for interactive menu
- Upgrade to Go 1.23.0
```

### sshtun-user
```
feat: rewrite CLI with Cobra and shared go-corelib/tui

- Add Cobra-based CLI with subcommands (create, update, list, delete, configure, uninstall)
- Use go-corelib/tui for all terminal UI (consistent with dnstm)
- Use go-corelib/osdetect.RequireRoot() for standardized root checks
- Add charmbracelet/huh for interactive prompts
- Refactor into modular packages:
  - cmd/ for CLI commands
  - internal/menu/ for interactive menu
  - pkg/cli/api.go for public API (dnstm integration)
- Add embedded mode support for dnstm integration
- Upgrade to Go 1.23.0
```

---

## Action Items

1. [x] Extract shared UI code to go-corelib/tui
2. [x] Standardize root check with osdetect.RequireRoot()
3. [ ] Remove `replace` directives before releasing (or use tagged versions)
4. [ ] Tag go-corelib release, then update version references in dnstm and sshtun-user

---

## Files Changed Summary

| Project | Added | Modified | Deleted | Net Lines |
|---------|-------|----------|---------|-----------|
| go-corelib | 5 | 6 | 5 | -64 |
| dnstm | 10 | 2 | 1 | +442 |
| sshtun-user | 9 | 2 | 9 | +184 |
