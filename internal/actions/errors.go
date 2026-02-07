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

	// ErrTunnelNotFound indicates the tunnel was not found.
	ErrTunnelNotFound = errors.New("tunnel not found")

	// ErrTunnelExists indicates the tunnel already exists.
	ErrTunnelExists = errors.New("tunnel already exists")

	// ErrBackendNotFound indicates the backend was not found.
	ErrBackendNotFound = errors.New("backend not found")

	// ErrBackendExists indicates the backend already exists.
	ErrBackendExists = errors.New("backend already exists")

	// ErrBackendInUse indicates the backend is in use by tunnels.
	ErrBackendInUse = errors.New("backend in use by tunnels")

	// ErrInvalidMode indicates an invalid operating mode.
	ErrInvalidMode = errors.New("invalid operating mode")

	// ErrNoTunnels indicates no tunnels are configured.
	ErrNoTunnels = errors.New("no tunnels configured")

	// ErrNoBackends indicates no backends are configured.
	ErrNoBackends = errors.New("no backends configured")

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

// TunnelNotFoundError creates a tunnel not found error.
func TunnelNotFoundError(tag string) *ActionError {
	return &ActionError{
		Message: fmt.Sprintf("tunnel '%s' not found", tag),
		Hint:    "Use 'dnstm tunnel list' to see available tunnels",
		Err:     ErrTunnelNotFound,
	}
}

// TunnelExistsError creates a tunnel already exists error.
func TunnelExistsError(tag string) *ActionError {
	return &ActionError{
		Message: fmt.Sprintf("tunnel '%s' already exists", tag),
		Hint:    "Choose a different tag or remove the existing tunnel",
		Err:     ErrTunnelExists,
	}
}

// BackendNotFoundError creates a backend not found error.
func BackendNotFoundError(tag string) *ActionError {
	return &ActionError{
		Message: fmt.Sprintf("backend '%s' not found", tag),
		Hint:    "Use 'dnstm backend list' to see available backends",
		Err:     ErrBackendNotFound,
	}
}

// BackendExistsError creates a backend already exists error.
func BackendExistsError(tag string) *ActionError {
	return &ActionError{
		Message: fmt.Sprintf("backend '%s' already exists", tag),
		Hint:    "Choose a different tag or remove the existing backend",
		Err:     ErrBackendExists,
	}
}

// BackendInUseError creates a backend in use error.
func BackendInUseError(tag string, tunnels []string) *ActionError {
	return &ActionError{
		Message: fmt.Sprintf("backend '%s' is in use by tunnels: %v", tag, tunnels),
		Hint:    "Remove the tunnels first",
		Err:     ErrBackendInUse,
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

// NoBackendsError creates an error for no backends configured.
func NoBackendsError() *ActionError {
	return &ActionError{
		Message: "no backends configured",
		Hint:    "Use 'dnstm backend add' to create one",
		Err:     ErrNoBackends,
	}
}
