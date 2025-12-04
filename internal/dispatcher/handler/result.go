package handler

import (
	"fmt"

	"github.com/dshills/keystorm/internal/engine/buffer"
)

// ResultStatus indicates the outcome of an action.
type ResultStatus uint8

const (
	// StatusOK indicates successful execution.
	StatusOK ResultStatus = iota
	// StatusNoOp indicates the action had no effect.
	StatusNoOp
	// StatusError indicates an error occurred.
	StatusError
	// StatusAsync indicates the operation is running asynchronously.
	StatusAsync
	// StatusCancelled indicates the operation was cancelled.
	StatusCancelled
)

// String returns a string representation of the status.
func (s ResultStatus) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusNoOp:
		return "no-op"
	case StatusError:
		return "error"
	case StatusAsync:
		return "async"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// Edit represents a text edit for result tracking.
type Edit struct {
	// Range is the range that was modified.
	Range buffer.Range
	// NewText is the text that was inserted.
	NewText string
	// OldText is the text that was replaced.
	OldText string
}

// CursorDelta describes cursor position change.
type CursorDelta struct {
	// BytesDelta is the change in byte offset.
	BytesDelta int64
	// LinesDelta is the change in line number.
	LinesDelta int
	// ColumnDelta is the change in column.
	ColumnDelta int
}

// ScrollTarget specifies a scroll destination.
type ScrollTarget struct {
	// Line is the target line number.
	Line uint32
	// Column is the target column number.
	Column uint32
	// Center indicates whether to center the view on the target.
	Center bool
}

// ViewUpdate describes required view updates.
type ViewUpdate struct {
	// ScrollTo specifies a scroll destination.
	ScrollTo *ScrollTarget
	// CenterLine specifies a line to center the view on.
	CenterLine *uint32
	// Redraw indicates whether the entire view needs redrawing.
	Redraw bool
	// RedrawLines specifies specific lines that need redrawing.
	RedrawLines []uint32
}

// Result represents the outcome of handling an action.
type Result struct {
	// Status indicates the result status.
	Status ResultStatus

	// Error contains any error that occurred.
	Error error

	// Message is an optional status message for display.
	Message string

	// Edits contains text edits that were applied.
	Edits []Edit

	// CursorDelta indicates how the cursor moved.
	CursorDelta CursorDelta

	// ModeChange indicates a mode transition (empty if no change).
	ModeChange string

	// ViewUpdate indicates required view updates.
	ViewUpdate ViewUpdate

	// Data holds handler-specific return data.
	Data map[string]interface{}
}

// IsOK returns true if the result indicates success.
func (r Result) IsOK() bool {
	return r.Status == StatusOK
}

// IsError returns true if the result indicates an error.
func (r Result) IsError() bool {
	return r.Status == StatusError
}

// Success creates a successful result.
func Success() Result {
	return Result{Status: StatusOK}
}

// SuccessWithMessage creates a successful result with a message.
func SuccessWithMessage(msg string) Result {
	return Result{Status: StatusOK, Message: msg}
}

// SuccessWithData creates a successful result with data.
func SuccessWithData(key string, value interface{}) Result {
	return Result{
		Status: StatusOK,
		Data:   map[string]interface{}{key: value},
	}
}

// NoOp creates a no-operation result.
func NoOp() Result {
	return Result{Status: StatusNoOp}
}

// NoOpWithMessage creates a no-operation result with a message.
func NoOpWithMessage(msg string) Result {
	return Result{Status: StatusNoOp, Message: msg}
}

// Error creates an error result.
func Error(err error) Result {
	return Result{Status: StatusError, Error: err}
}

// Errorf creates an error result with a formatted message.
func Errorf(format string, args ...interface{}) Result {
	return Result{
		Status: StatusError,
		Error:  fmt.Errorf(format, args...),
	}
}

// Async creates an async result.
func Async() Result {
	return Result{Status: StatusAsync}
}

// AsyncWithMessage creates an async result with a message.
func AsyncWithMessage(msg string) Result {
	return Result{Status: StatusAsync, Message: msg}
}

// Cancelled creates a cancelled result.
func Cancelled() Result {
	return Result{Status: StatusCancelled}
}

// CancelledWithMessage creates a cancelled result with a message.
func CancelledWithMessage(msg string) Result {
	return Result{Status: StatusCancelled, Message: msg}
}

// WithMessage returns a copy of the result with the specified message.
func (r Result) WithMessage(msg string) Result {
	r.Message = msg
	return r
}

// WithModeChange returns a copy of the result with a mode change.
func (r Result) WithModeChange(mode string) Result {
	r.ModeChange = mode
	return r
}

// WithViewUpdate returns a copy of the result with a view update.
func (r Result) WithViewUpdate(vu ViewUpdate) Result {
	r.ViewUpdate = vu
	return r
}

// WithScrollTo returns a copy of the result with a scroll target.
func (r Result) WithScrollTo(line, col uint32, center bool) Result {
	r.ViewUpdate.ScrollTo = &ScrollTarget{
		Line:   line,
		Column: col,
		Center: center,
	}
	return r
}

// WithCenterLine returns a copy of the result centered on a line.
func (r Result) WithCenterLine(line uint32) Result {
	r.ViewUpdate.CenterLine = &line
	return r
}

// WithRedraw returns a copy of the result requesting a full redraw.
func (r Result) WithRedraw() Result {
	r.ViewUpdate.Redraw = true
	return r
}

// WithRedrawLines returns a copy of the result with specific lines to redraw.
func (r Result) WithRedrawLines(lines ...uint32) Result {
	r.ViewUpdate.RedrawLines = append(r.ViewUpdate.RedrawLines, lines...)
	return r
}

// WithEdit returns a copy of the result with an edit added.
func (r Result) WithEdit(edit Edit) Result {
	r.Edits = append(r.Edits, edit)
	return r
}

// WithEdits returns a copy of the result with edits added.
func (r Result) WithEdits(edits []Edit) Result {
	r.Edits = append(r.Edits, edits...)
	return r
}

// WithCursorDelta returns a copy of the result with cursor delta.
func (r Result) WithCursorDelta(delta CursorDelta) Result {
	r.CursorDelta = delta
	return r
}

// WithData returns a copy of the result with data added.
func (r Result) WithData(key string, value interface{}) Result {
	if r.Data == nil {
		r.Data = make(map[string]interface{})
	}
	r.Data[key] = value
	return r
}

// GetData retrieves a value from the result data.
func (r Result) GetData(key string) (interface{}, bool) {
	if r.Data == nil {
		return nil, false
	}
	v, ok := r.Data[key]
	return v, ok
}

// GetDataString retrieves a string value from the result data.
func (r Result) GetDataString(key string) string {
	if v, ok := r.GetData(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetDataInt retrieves an int value from the result data.
func (r Result) GetDataInt(key string) int {
	if v, ok := r.GetData(key); ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

// GetDataBool retrieves a bool value from the result data.
func (r Result) GetDataBool(key string) bool {
	if v, ok := r.GetData(key); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
