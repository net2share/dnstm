package actions

import (
	"errors"
	"fmt"
)

// Common errors for action handling.
var (
	// ErrCancelled indicates the user cancelled the operation.
	ErrCancelled = errors.New("cancelled")

	// ErrNotInitialized indicates the router is not initialized.
	ErrNotInitialized = errors.New("router not initialized")

	// ErrNotInstalled indicates transport binaries are not installed.
	ErrNotInstalled = errors.New("transport binaries not installed")

	// ErrInstanceNotFound indicates the instance was not found.
	ErrInstanceNotFound = errors.New("instance not found")

	// ErrInstanceExists indicates the instance already exists.
	ErrInstanceExists = errors.New("instance already exists")

	// ErrInvalidMode indicates an invalid operating mode.
	ErrInvalidMode = errors.New("invalid operating mode")

	// ErrNoInstances indicates no instances are configured.
	ErrNoInstances = errors.New("no instances configured")

	// ErrSingleModeOnly indicates the action is only available in single mode.
	ErrSingleModeOnly = errors.New("only available in single-tunnel mode")

	// ErrMultiModeOnly indicates the action is only available in multi mode.
	ErrMultiModeOnly = errors.New("only available in multi-tunnel mode")
)

// ActionError represents a structured error with a hint.
type ActionError struct {
	// Message is the main error message.
	Message string
	// Hint provides a suggestion for resolution.
	Hint string
	// Err is the underlying error, if any.
	Err error
}

// Error implements the error interface.
func (e *ActionError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s\n%s", e.Message, e.Hint)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *ActionError) Unwrap() error {
	return e.Err
}

// NewActionError creates a new ActionError.
func NewActionError(message, hint string) *ActionError {
	return &ActionError{
		Message: message,
		Hint:    hint,
	}
}

// WrapError wraps an error with a message and hint.
func WrapError(err error, message, hint string) *ActionError {
	return &ActionError{
		Message: message,
		Hint:    hint,
		Err:     err,
	}
}

// NotFoundError creates an instance not found error.
func NotFoundError(name string) *ActionError {
	return &ActionError{
		Message: fmt.Sprintf("instance '%s' not found", name),
		Hint:    "Use 'dnstm instance list' to see available instances",
		Err:     ErrInstanceNotFound,
	}
}

// ExistsError creates an instance already exists error.
func ExistsError(name string) *ActionError {
	return &ActionError{
		Message: fmt.Sprintf("instance '%s' already exists", name),
		Hint:    "Choose a different name or remove the existing instance",
		Err:     ErrInstanceExists,
	}
}

// NotInitializedError creates a router not initialized error.
func NotInitializedError() *ActionError {
	return &ActionError{
		Message: "router not initialized",
		Hint:    "Run 'dnstm install' first",
		Err:     ErrNotInitialized,
	}
}

// NotInstalledError creates a transport binaries not installed error.
func NotInstalledError(missing []string) *ActionError {
	return &ActionError{
		Message: fmt.Sprintf("transport binaries not installed. Missing: %v", missing),
		Hint:    "Run 'dnstm install' first",
		Err:     ErrNotInstalled,
	}
}

// SingleModeOnlyError creates an error for single-mode-only actions.
func SingleModeOnlyError() *ActionError {
	return &ActionError{
		Message: "this command is only available in single-tunnel mode",
		Hint:    "Use 'dnstm router mode single' to switch modes first",
		Err:     ErrSingleModeOnly,
	}
}

// NoInstancesError creates an error for no instances configured.
func NoInstancesError() *ActionError {
	return &ActionError{
		Message: "no instances configured",
		Hint:    "Use 'dnstm instance add' to create one",
		Err:     ErrNoInstances,
	}
}
