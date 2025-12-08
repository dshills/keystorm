// Package app provides the main application structure and coordination.
package app

import "errors"

// Application errors.
var (
	// ErrQuit signals that the application should exit normally.
	ErrQuit = errors.New("quit requested")

	// ErrAlreadyRunning indicates the application is already running.
	ErrAlreadyRunning = errors.New("application already running")

	// ErrNotRunning indicates the application is not running.
	ErrNotRunning = errors.New("application not running")

	// ErrNoActiveDocument indicates no document is currently active.
	ErrNoActiveDocument = errors.New("no active document")

	// ErrDocumentNotFound indicates a document was not found.
	ErrDocumentNotFound = errors.New("document not found")

	// ErrDocumentAlreadyOpen indicates a document is already open.
	ErrDocumentAlreadyOpen = errors.New("document already open")

	// ErrUnsavedChanges indicates there are unsaved changes.
	ErrUnsavedChanges = errors.New("unsaved changes")

	// ErrInitialization indicates an initialization failure.
	ErrInitialization = errors.New("initialization failed")

	// ErrShutdownTimeout indicates shutdown timed out.
	ErrShutdownTimeout = errors.New("shutdown timed out")
)
