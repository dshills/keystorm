package integration

import "errors"

// Sentinel errors for the integration package.
var (
	// ErrManagerClosed is returned when operations are attempted on a closed manager.
	ErrManagerClosed = errors.New("integration manager is closed")

	// ErrProcessNotFound is returned when a process ID is not found.
	ErrProcessNotFound = errors.New("process not found")

	// ErrProcessAlreadyStarted is returned when trying to start an already running process.
	ErrProcessAlreadyStarted = errors.New("process already started")

	// ErrProcessNotStarted is returned when operations require a started process.
	ErrProcessNotStarted = errors.New("process not started")

	// ErrProcessExited is returned when a process has already exited.
	ErrProcessExited = errors.New("process has exited")

	// ErrSupervisorShutdown is returned when the supervisor is shutting down.
	ErrSupervisorShutdown = errors.New("supervisor is shutting down")

	// ErrInvalidConfiguration is returned for invalid configuration values.
	ErrInvalidConfiguration = errors.New("invalid configuration")

	// ErrComponentNotInitialized is returned when a component is not initialized.
	ErrComponentNotInitialized = errors.New("component not initialized")

	// ErrWorkspaceNotSet is returned when workspace root is required but not set.
	ErrWorkspaceNotSet = errors.New("workspace root not set")
)
