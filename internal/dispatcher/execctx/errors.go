package execctx

import "errors"

// Context validation errors.
var (
	// ErrMissingEngine indicates the engine is required but not set.
	ErrMissingEngine = errors.New("execution context: engine is required")

	// ErrMissingCursors indicates cursors are required but not set.
	ErrMissingCursors = errors.New("execution context: cursors are required")

	// ErrReadOnly indicates the buffer is read-only.
	ErrReadOnly = errors.New("execution context: buffer is read-only")

	// ErrMissingModeManager indicates mode manager is required but not set.
	ErrMissingModeManager = errors.New("execution context: mode manager is required")

	// ErrMissingHistory indicates history is required but not set.
	ErrMissingHistory = errors.New("execution context: history is required")

	// ErrMissingRenderer indicates renderer is required but not set.
	ErrMissingRenderer = errors.New("execution context: renderer is required")

	// ErrMissingMotion indicates a motion, text object, or selection is required for the operator.
	ErrMissingMotion = errors.New("execution context: operator requires motion, text object, or selection")
)
