// Package execctx provides the execution context for action handlers.
package execctx

import (
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// EngineInterface abstracts the text engine for handlers.
type EngineInterface interface {
	// Text operations
	Insert(offset buffer.ByteOffset, text string) (buffer.EditResult, error)
	Delete(start, end buffer.ByteOffset) (buffer.EditResult, error)
	Replace(start, end buffer.ByteOffset, text string) (buffer.EditResult, error)

	// Read operations
	Text() string
	TextRange(start, end buffer.ByteOffset) string
	LineText(line uint32) string
	Len() buffer.ByteOffset
	LineCount() uint32

	// Line operations
	LineStartOffset(line uint32) buffer.ByteOffset
	LineEndOffset(line uint32) buffer.ByteOffset
	LineLen(line uint32) uint32

	// Position conversion
	OffsetToPoint(offset buffer.ByteOffset) buffer.Point
	PointToOffset(point buffer.Point) buffer.ByteOffset

	// Snapshotting
	Snapshot() EngineReader
	RevisionID() buffer.RevisionID
}

// EngineReader provides read-only access to the engine.
type EngineReader interface {
	Text() string
	TextRange(start, end buffer.ByteOffset) string
	LineText(line uint32) string
	Len() buffer.ByteOffset
	LineCount() uint32
	LineStartOffset(line uint32) buffer.ByteOffset
	LineEndOffset(line uint32) buffer.ByteOffset
	LineLen(line uint32) uint32
	OffsetToPoint(offset buffer.ByteOffset) buffer.Point
	PointToOffset(point buffer.Point) buffer.ByteOffset
}

// CursorManagerInterface abstracts cursor management for handlers.
type CursorManagerInterface interface {
	// Primary cursor
	Primary() cursor.Selection
	SetPrimary(sel cursor.Selection)

	// Multi-cursor
	All() []cursor.Selection
	Add(sel cursor.Selection)
	Clear()
	Count() int
	IsMulti() bool

	// Selection state
	HasSelection() bool

	// Bulk operations
	SetAll(sels []cursor.Selection)
	MapInPlace(f func(sel cursor.Selection) cursor.Selection)

	// Utility
	Clone() *cursor.CursorSet
	Clamp(maxOffset cursor.ByteOffset)
}

// ModeManagerInterface abstracts mode management for handlers.
type ModeManagerInterface interface {
	// Current mode
	Current() ModeInterface
	CurrentName() string

	// Mode transitions
	Switch(name string) error
	Push(name string) error
	Pop() error

	// Mode queries
	IsMode(name string) bool
	IsAnyMode(names ...string) bool
}

// ModeInterface represents an editor mode.
type ModeInterface interface {
	Name() string
	DisplayName() string
}

// HistoryInterface abstracts undo/redo for handlers.
type HistoryInterface interface {
	// Grouping for compound edits
	BeginGroup(name string)
	EndGroup()
	CancelGroup()
	IsGrouping() bool

	// Undo/redo availability
	CanUndo() bool
	CanRedo() bool
	UndoCount() int
	RedoCount() int
}

// RendererInterface abstracts rendering for handlers.
type RendererInterface interface {
	// Scrolling
	ScrollTo(line, col uint32)
	CenterOnLine(line uint32)

	// Redrawing
	Redraw()
	RedrawLines(lines []uint32)

	// View info
	VisibleLineRange() (start, end uint32)
}

// ExecutionContext provides context for action execution.
// It contains references to all editor subsystems needed by handlers.
type ExecutionContext struct {
	// Engine provides access to the text buffer.
	Engine EngineInterface

	// Cursors provides access to cursor/selection state.
	Cursors CursorManagerInterface

	// ModeManager provides mode state.
	ModeManager ModeManagerInterface

	// History provides undo/redo grouping.
	History HistoryInterface

	// Renderer provides view operations.
	Renderer RendererInterface

	// Input provides the input context (mode, pending state, etc.).
	Input *input.Context

	// Buffer metadata
	FilePath string
	FileType string

	// Execution options
	Count  int  // Repeat count (1 if not specified)
	DryRun bool // If true, don't apply changes (for preview)

	// Data holds handler-specific context data.
	Data map[string]interface{}
}

// New creates a new execution context.
func New() *ExecutionContext {
	return &ExecutionContext{
		Count: 1,
		Data:  make(map[string]interface{}),
	}
}

// NewWithInputContext creates a new execution context from an input context.
func NewWithInputContext(inputCtx *input.Context) *ExecutionContext {
	ctx := New()
	ctx.Input = inputCtx

	if inputCtx != nil {
		// Extract count from input context
		if inputCtx.PendingCount > 0 {
			ctx.Count = inputCtx.PendingCount
		}

		// Extract file info
		ctx.FilePath = inputCtx.FilePath
		ctx.FileType = inputCtx.FileType
	}

	return ctx
}

// WithEngine returns the context with the engine set.
func (ctx *ExecutionContext) WithEngine(engine EngineInterface) *ExecutionContext {
	ctx.Engine = engine
	return ctx
}

// WithCursors returns the context with cursors set.
func (ctx *ExecutionContext) WithCursors(cursors CursorManagerInterface) *ExecutionContext {
	ctx.Cursors = cursors
	return ctx
}

// WithModeManager returns the context with mode manager set.
func (ctx *ExecutionContext) WithModeManager(mm ModeManagerInterface) *ExecutionContext {
	ctx.ModeManager = mm
	return ctx
}

// WithHistory returns the context with history set.
func (ctx *ExecutionContext) WithHistory(history HistoryInterface) *ExecutionContext {
	ctx.History = history
	return ctx
}

// WithRenderer returns the context with renderer set.
func (ctx *ExecutionContext) WithRenderer(renderer RendererInterface) *ExecutionContext {
	ctx.Renderer = renderer
	return ctx
}

// WithCount returns the context with repeat count set.
func (ctx *ExecutionContext) WithCount(count int) *ExecutionContext {
	if count > 0 {
		ctx.Count = count
	}
	return ctx
}

// WithDryRun returns the context with dry run mode enabled.
func (ctx *ExecutionContext) WithDryRun(dryRun bool) *ExecutionContext {
	ctx.DryRun = dryRun
	return ctx
}

// GetCount returns the repeat count, defaulting to 1.
func (ctx *ExecutionContext) GetCount() int {
	if ctx.Count <= 0 {
		return 1
	}
	return ctx.Count
}

// Mode returns the current mode name.
func (ctx *ExecutionContext) Mode() string {
	if ctx.Input != nil {
		return ctx.Input.Mode
	}
	if ctx.ModeManager != nil {
		return ctx.ModeManager.CurrentName()
	}
	return ""
}

// HasSelection returns true if there is an active selection.
func (ctx *ExecutionContext) HasSelection() bool {
	if ctx.Cursors != nil {
		return ctx.Cursors.HasSelection()
	}
	if ctx.Input != nil {
		return ctx.Input.HasSelection
	}
	return false
}

// IsReadOnly returns true if the buffer is read-only.
func (ctx *ExecutionContext) IsReadOnly() bool {
	if ctx.Input != nil {
		return ctx.Input.IsReadOnly
	}
	return false
}

// IsModified returns true if the buffer has unsaved changes.
func (ctx *ExecutionContext) IsModified() bool {
	if ctx.Input != nil {
		return ctx.Input.IsModified
	}
	return false
}

// PendingOperator returns the pending operator, if any.
func (ctx *ExecutionContext) PendingOperator() string {
	if ctx.Input != nil {
		return ctx.Input.PendingOperator
	}
	return ""
}

// PendingRegister returns the pending register, if any.
func (ctx *ExecutionContext) PendingRegister() rune {
	if ctx.Input != nil {
		return ctx.Input.PendingRegister
	}
	return 0
}

// SetData sets a context data value.
func (ctx *ExecutionContext) SetData(key string, value interface{}) {
	if ctx.Data == nil {
		ctx.Data = make(map[string]interface{})
	}
	ctx.Data[key] = value
}

// GetData retrieves a context data value.
func (ctx *ExecutionContext) GetData(key string) (interface{}, bool) {
	if ctx.Data == nil {
		return nil, false
	}
	v, ok := ctx.Data[key]
	return v, ok
}

// GetDataString retrieves a string value from context data.
func (ctx *ExecutionContext) GetDataString(key string) string {
	if v, ok := ctx.GetData(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetDataInt retrieves an int value from context data.
func (ctx *ExecutionContext) GetDataInt(key string) int {
	if v, ok := ctx.GetData(key); ok {
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

// GetDataBool retrieves a bool value from context data.
func (ctx *ExecutionContext) GetDataBool(key string) bool {
	if v, ok := ctx.GetData(key); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// Validate checks that the context has all required components.
func (ctx *ExecutionContext) Validate() error {
	// Engine is required for most operations
	if ctx.Engine == nil {
		return ErrMissingEngine
	}
	return nil
}

// ValidateForEdit checks that the context is valid for editing operations.
func (ctx *ExecutionContext) ValidateForEdit() error {
	if err := ctx.Validate(); err != nil {
		return err
	}
	if ctx.Cursors == nil {
		return ErrMissingCursors
	}
	if ctx.IsReadOnly() {
		return ErrReadOnly
	}
	return nil
}
