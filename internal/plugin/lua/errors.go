package lua

import "errors"

// Errors for Lua state operations.
var (
	// ErrStateClosed is returned when operating on a closed state.
	ErrStateClosed = errors.New("lua state is closed")

	// ErrExecutionTimeout is returned when execution times out.
	ErrExecutionTimeout = errors.New("lua execution timeout")

	// ErrInstructionLimit is returned when instruction limit is exceeded.
	ErrInstructionLimit = errors.New("lua instruction limit exceeded")
)
