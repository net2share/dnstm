package actions

// OutputWriter defines the interface for action output.
// This abstracts the output mechanism so handlers work in both CLI and TUI.
type OutputWriter interface {
	// Print outputs a line of text.
	Print(msg string)
	// Printf outputs formatted text.
	Printf(format string, args ...interface{})
	// Println outputs a line with newline.
	Println(args ...interface{})

	// Info outputs an informational message.
	Info(msg string)
	// Success outputs a success message.
	Success(msg string)
	// Warning outputs a warning message.
	Warning(msg string)
	// Error outputs an error message.
	Error(msg string)

	// Status outputs a status update (checkmark + message).
	Status(msg string)
	// Step outputs a step progress message.
	Step(current, total int, msg string)

	// Box outputs content in a bordered box.
	Box(title string, lines []string)
	// KV formats a key-value pair.
	KV(key, value string) string

	// Table outputs a table with headers and rows.
	Table(headers []string, rows [][]string)
	// Separator outputs a horizontal separator line.
	Separator(length int)
}

// Standard symbols for output.
const (
	SymbolSuccess = "✓"
	SymbolError   = "✗"
	SymbolWarning = "⚠"
	SymbolInfo    = "ℹ"
	SymbolRunning = "●"
	SymbolStopped = "○"
	SymbolArrow   = "→"
	SymbolBranch  = "└─"
)

// Standard column widths for tables.
const (
	ColWidthName   = 16
	ColWidthType   = 24
	ColWidthPort   = 8
	ColWidthDomain = 20
	ColWidthStatus = 10
)

// GetPickerOptions retrieves picker options from context after PickerFunc is called.
// Returns nil if no options are available.
func GetPickerOptions(ctx *Context) []SelectOption {
	optionsVal, ok := ctx.Values["_picker_options"]
	if !ok {
		return nil
	}
	options, ok := optionsVal.([]SelectOption)
	if !ok {
		return nil
	}
	return options
}
