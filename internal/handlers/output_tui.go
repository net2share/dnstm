package handlers

import (
	"fmt"
	"strings"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/go-corelib/tui"
)

// TUIOutput implements OutputWriter using the tui package.
type TUIOutput struct {
	progressView *tui.ProgressView
}

// NewTUIOutput creates a new TUI output writer.
func NewTUIOutput() *TUIOutput {
	return &TUIOutput{}
}

// Print outputs a line of text.
func (t *TUIOutput) Print(msg string) {
	if t.progressView != nil {
		t.progressView.AddText(msg)
		return
	}
	fmt.Print(msg)
}

// Printf outputs formatted text.
func (t *TUIOutput) Printf(format string, args ...interface{}) {
	if t.progressView != nil {
		t.progressView.AddText(fmt.Sprintf(format, args...))
		return
	}
	fmt.Printf(format, args...)
}

// Println outputs a line with newline.
func (t *TUIOutput) Println(args ...interface{}) {
	if t.progressView != nil {
		if len(args) == 0 {
			t.progressView.AddText("")
		} else {
			t.progressView.AddText(fmt.Sprint(args...))
		}
		return
	}
	fmt.Println(args...)
}

// Info outputs an informational message.
func (t *TUIOutput) Info(msg string) {
	if t.progressView != nil {
		t.progressView.AddInfo(msg)
		return
	}
	tui.PrintInfo(msg)
}

// Success outputs a success message.
func (t *TUIOutput) Success(msg string) {
	if t.progressView != nil {
		t.progressView.AddSuccess(msg)
		return
	}
	tui.PrintSuccess(msg)
}

// Warning outputs a warning message.
func (t *TUIOutput) Warning(msg string) {
	if t.progressView != nil {
		t.progressView.AddWarning(msg)
		return
	}
	tui.PrintWarning(msg)
}

// Error outputs an error message.
func (t *TUIOutput) Error(msg string) {
	if t.progressView != nil {
		t.progressView.AddError(msg)
		return
	}
	tui.PrintError(msg)
}

// Status outputs a status update.
func (t *TUIOutput) Status(msg string) {
	if t.progressView != nil {
		t.progressView.AddStatus(msg)
		return
	}
	tui.PrintStatus(msg)
}

// Step outputs a step progress message.
func (t *TUIOutput) Step(current, total int, msg string) {
	if t.progressView != nil {
		t.progressView.AddInfo(fmt.Sprintf("[%d/%d] %s", current, total, msg))
		return
	}
	tui.PrintStep(current, total, msg)
}

// Box outputs content in a bordered box.
func (t *TUIOutput) Box(title string, lines []string) {
	if t.progressView != nil {
		if title != "" {
			t.progressView.AddText(title)
		}
		for _, line := range lines {
			t.progressView.AddText("  " + line)
		}
		return
	}
	tui.PrintBox(title, lines)
}

// KV formats a key-value pair with a colon separator.
func (t *TUIOutput) KV(key, value string) string {
	return tui.KV(key+": ", value)
}

// Table outputs a table with headers and rows.
func (t *TUIOutput) Table(headers []string, rows [][]string) {
	// Calculate column widths based on content
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Build format string
	var formatParts []string
	for _, w := range widths {
		formatParts = append(formatParts, fmt.Sprintf("%%-%ds", w+2))
	}
	format := strings.Join(formatParts, "")

	if t.progressView != nil {
		// In progress view, output as text
		headerArgs := make([]interface{}, len(headers))
		for i, h := range headers {
			headerArgs[i] = h
		}
		t.progressView.AddText(fmt.Sprintf(format, headerArgs...))
		for _, row := range rows {
			rowArgs := make([]interface{}, len(row))
			for i, cell := range row {
				rowArgs[i] = cell
			}
			t.progressView.AddText(fmt.Sprintf(format, rowArgs...))
		}
		return
	}

	format += "\n"

	// Print headers
	headerArgs := make([]interface{}, len(headers))
	for i, h := range headers {
		headerArgs[i] = h
	}
	fmt.Printf(format, headerArgs...)

	// Print separator
	total := 0
	for _, w := range widths {
		total += w + 2
	}
	t.Separator(total)

	// Print rows
	for _, row := range rows {
		rowArgs := make([]interface{}, len(row))
		for i, cell := range row {
			rowArgs[i] = cell
		}
		fmt.Printf(format, rowArgs...)
	}
}

// Separator outputs a horizontal separator line.
func (t *TUIOutput) Separator(length int) {
	if t.progressView != nil {
		t.progressView.AddText(strings.Repeat("-", length))
		return
	}
	fmt.Println(strings.Repeat("-", length))
}

// ShowInfo displays information in a fullscreen TUI view.
func (t *TUIOutput) ShowInfo(cfg actions.InfoConfig) error {
	// Convert actions.InfoConfig to tui.InfoConfig
	tuiCfg := tui.InfoConfig{
		Title:       cfg.Title,
		Description: cfg.Description,
	}
	for _, section := range cfg.Sections {
		tuiSection := tui.InfoSection{
			Title: section.Title,
		}
		for _, row := range section.Rows {
			tuiSection.Rows = append(tuiSection.Rows, tui.InfoRow{
				Key:     row.Key,
				Value:   row.Value,
				Columns: row.Columns,
			})
		}
		tuiCfg.Sections = append(tuiCfg.Sections, tuiSection)
	}
	return tui.ShowInfo(tuiCfg)
}

// BeginProgress starts a progress view with the given title.
func (t *TUIOutput) BeginProgress(title string) {
	t.progressView = tui.NewProgressView(title)
}

// EndProgress signals progress is complete and waits for user dismissal.
func (t *TUIOutput) EndProgress() {
	if t.progressView != nil {
		t.progressView.Done()
		t.progressView = nil
	}
}

// DismissProgress closes the progress view immediately without waiting for user input.
func (t *TUIOutput) DismissProgress() {
	if t.progressView != nil {
		t.progressView.Dismiss()
		t.progressView = nil
	}
}

// IsProgressActive returns true if a progress view is currently active.
func (t *TUIOutput) IsProgressActive() bool {
	return t.progressView != nil
}

// Verify TUIOutput implements OutputWriter.
var _ actions.OutputWriter = (*TUIOutput)(nil)
