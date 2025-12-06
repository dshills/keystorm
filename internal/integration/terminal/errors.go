package terminal

import "errors"

// Sentinel errors for the terminal package.
var (
	// ErrTerminalClosed is returned when operations are attempted on a closed terminal.
	ErrTerminalClosed = errors.New("terminal is closed")

	// ErrTerminalNotFound is returned when a terminal ID is not found.
	ErrTerminalNotFound = errors.New("terminal not found")

	// ErrInvalidSize is returned when terminal size is invalid.
	ErrInvalidSize = errors.New("invalid terminal size")

	// ErrPTYNotSupported is returned when PTY is not supported on this platform.
	ErrPTYNotSupported = errors.New("PTY not supported on this platform")

	// ErrShellNotFound is returned when the shell executable is not found.
	ErrShellNotFound = errors.New("shell not found")

	// ErrManagerClosed is returned when operations are attempted on a closed manager.
	ErrManagerClosed = errors.New("terminal manager is closed")
)
