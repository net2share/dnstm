package handlers

import (
	"fmt"
	"strings"

	"github.com/net2share/dnstm/internal/actions"
	"github.com/net2share/go-corelib/tui"
)

// TUIOutput implements OutputWriter using the tui package.
type TUIOutput struct{}

// NewTUIOutput creates a new TUI output writer.
func NewTUIOutput() *TUIOutput {
	return &TUIOutput{}
}

// Print outputs a line of text.
func (t *TUIOutput) Print(msg string) {
	fmt.Print(msg)
}

// Printf outputs formatted text.
func (t *TUIOutput) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// Println outputs a line with newline.
func (t *TUIOutput) Println(args ...interface{}) {
	fmt.Println(args...)
}

// Info outputs an informational message.
func (t *TUIOutput) Info(msg string) {
	tui.PrintInfo(msg)
}

// Success outputs a success message.
func (t *TUIOutput) Success(msg string) {
	tui.PrintSuccess(msg)
}

// Warning outputs a warning message.
func (t *TUIOutput) Warning(msg string) {
	tui.PrintWarning(msg)
}

// Error outputs an error message.
func (t *TUIOutput) Error(msg string) {
	tui.PrintError(msg)
}

// Status outputs a status update.
func (t *TUIOutput) Status(msg string) {
	tui.PrintStatus(msg)
}

// Step outputs a step progress message.
func (t *TUIOutput) Step(current, total int, msg string) {
	tui.PrintStep(current, total, msg)
}

// Box outputs content in a bordered box.
func (t *TUIOutput) Box(title string, lines []string) {
	tui.PrintBox(title, lines)
}

// KV formats a key-value pair.
func (t *TUIOutput) KV(key, value string) string {
	return tui.KV(key, value)
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
	format := strings.Join(formatParts, "") + "\n"

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
	fmt.Println(strings.Repeat("-", length))
}

// Verify TUIOutput implements OutputWriter.
var _ actions.OutputWriter = (*TUIOutput)(nil)
