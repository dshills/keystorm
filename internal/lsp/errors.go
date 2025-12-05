package lsp

import (
	"errors"
	"fmt"
)

// Standard errors returned by the LSP client.
var (
	// ErrNotStarted indicates the client has not been started.
	ErrNotStarted = errors.New("lsp client not started")

	// ErrAlreadyStarted indicates the client is already running.
	ErrAlreadyStarted = errors.New("lsp client already started")

	// ErrShutdown indicates the client has been shut down.
	ErrShutdown = errors.New("lsp client shut down")

	// ErrNoServer indicates no server is configured for the language.
	ErrNoServer = errors.New("no server configured for language")

	// ErrServerNotReady indicates the server is not ready to handle requests.
	ErrServerNotReady = errors.New("server not ready")

	// ErrNotSupported indicates the server does not support the requested feature.
	ErrNotSupported = errors.New("feature not supported by server")

	// ErrDocumentNotOpen indicates the document is not open.
	ErrDocumentNotOpen = errors.New("document not open")

	// ErrDocumentAlreadyOpen indicates the document is already open.
	ErrDocumentAlreadyOpen = errors.New("document already open")

	// ErrTimeout indicates a request timed out.
	ErrTimeout = errors.New("request timed out")

	// ErrServerCrashed indicates the server process terminated unexpectedly.
	ErrServerCrashed = errors.New("server crashed")

	// ErrInvalidResponse indicates an invalid response from the server.
	ErrInvalidResponse = errors.New("invalid response from server")
)

// RPCError represents a JSON-RPC error from the server.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *RPCError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("rpc error %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// Standard JSON-RPC error codes.
const (
	// JSON-RPC standard errors
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603

	// LSP-specific errors
	CodeServerNotInitialized = -32002
	CodeUnknownErrorCode     = -32001
	CodeRequestCancelled     = -32800
	CodeContentModified      = -32801
	CodeServerCancelled      = -32802
	CodeRequestFailed        = -32803
)

// ServerError represents an error related to server lifecycle.
type ServerError struct {
	LanguageID string
	Err        error
}

// Error implements the error interface.
func (e *ServerError) Error() string {
	return fmt.Sprintf("server %s: %v", e.LanguageID, e.Err)
}

// Unwrap returns the underlying error.
func (e *ServerError) Unwrap() error {
	return e.Err
}
