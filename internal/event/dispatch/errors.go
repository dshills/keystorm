package dispatch

import "errors"

// Sentinel errors for the dispatch package.
var (
	// ErrAlreadyRunning is returned when Start is called on a running dispatcher.
	ErrAlreadyRunning = errors.New("dispatcher is already running")

	// ErrNotRunning is returned when operations are attempted on a stopped dispatcher.
	ErrNotRunning = errors.New("dispatcher is not running")

	// ErrQueueFull is returned when the async queue is full and cannot accept more tasks.
	ErrQueueFull = errors.New("task queue is full")
)
