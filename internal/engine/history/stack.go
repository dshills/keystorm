package history

import (
	"errors"
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
)

// Common errors for history operations.
var (
	ErrNothingToUndo = errors.New("nothing to undo")
	ErrNothingToRedo = errors.New("nothing to redo")
)

// undoEntry wraps a command with metadata.
type undoEntry struct {
	command   Command
	timestamp time.Time
}

// History manages undo/redo state for a buffer.
type History struct {
	mu sync.Mutex

	undoStack []*undoEntry
	redoStack []*undoEntry

	// Grouping state
	grouping  bool
	groupName string
	groupCmds []Command

	// Configuration
	maxEntries int
}

// NewHistory creates a new history manager.
func NewHistory(maxEntries int) *History {
	if maxEntries <= 0 {
		maxEntries = 1000 // Default
	}
	return &History{
		maxEntries: maxEntries,
	}
}

// Execute runs a command and adds it to the undo stack.
func (h *History) Execute(cmd Command, buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	if err := cmd.Execute(buf, cursors); err != nil {
		return err
	}

	h.Push(cmd)
	return nil
}

// Push adds a command to the undo stack.
// Clears the redo stack.
func (h *History) Push(cmd Command) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.grouping {
		h.groupCmds = append(h.groupCmds, cmd)
		return
	}

	h.pushLocked(cmd)
}

// pushLocked adds a command without acquiring the lock.
func (h *History) pushLocked(cmd Command) {
	h.undoStack = append(h.undoStack, &undoEntry{
		command:   cmd,
		timestamp: time.Now(),
	})

	// Clear redo stack
	h.redoStack = nil

	// Enforce max entries
	if len(h.undoStack) > h.maxEntries {
		// Remove oldest entries
		excess := len(h.undoStack) - h.maxEntries
		h.undoStack = h.undoStack[excess:]
	}
}

// Undo undoes the last command.
// The lock is released during command execution to avoid holding it during
// potentially long-running buffer operations.
func (h *History) Undo(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	h.mu.Lock()
	if len(h.undoStack) == 0 {
		h.mu.Unlock()
		return ErrNothingToUndo
	}

	entry := h.undoStack[len(h.undoStack)-1]
	h.undoStack = h.undoStack[:len(h.undoStack)-1]
	h.mu.Unlock()

	// Execute undo without holding the lock
	if err := entry.command.Undo(buf, cursors); err != nil {
		// Restore entry on failure
		h.mu.Lock()
		h.undoStack = append(h.undoStack, entry)
		h.mu.Unlock()
		return err
	}

	h.mu.Lock()
	h.redoStack = append(h.redoStack, entry)
	h.mu.Unlock()
	return nil
}

// Redo redoes the last undone command.
// The lock is released during command execution to avoid holding it during
// potentially long-running buffer operations.
func (h *History) Redo(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	h.mu.Lock()
	if len(h.redoStack) == 0 {
		h.mu.Unlock()
		return ErrNothingToRedo
	}

	entry := h.redoStack[len(h.redoStack)-1]
	h.redoStack = h.redoStack[:len(h.redoStack)-1]
	h.mu.Unlock()

	// Execute redo without holding the lock
	if err := entry.command.Execute(buf, cursors); err != nil {
		// Restore entry on failure
		h.mu.Lock()
		h.redoStack = append(h.redoStack, entry)
		h.mu.Unlock()
		return err
	}

	h.mu.Lock()
	h.undoStack = append(h.undoStack, entry)
	h.mu.Unlock()
	return nil
}

// CanUndo returns true if undo is available.
func (h *History) CanUndo() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.undoStack) > 0
}

// CanRedo returns true if redo is available.
func (h *History) CanRedo() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.redoStack) > 0
}

// UndoCount returns the number of undo operations available.
func (h *History) UndoCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.undoStack)
}

// RedoCount returns the number of redo operations available.
func (h *History) RedoCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.redoStack)
}

// BeginGroup starts a command group.
// Commands pushed while grouping will be combined into a single undo unit.
func (h *History) BeginGroup(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.grouping {
		// Already grouping, ignore nested calls
		return
	}

	h.grouping = true
	h.groupName = name
	h.groupCmds = nil
}

// EndGroup finishes a command group.
// All commands since BeginGroup are combined into a CompoundCommand.
func (h *History) EndGroup() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.grouping {
		return
	}

	h.grouping = false

	if len(h.groupCmds) == 0 {
		h.groupCmds = nil
		return
	}

	// Create compound command
	compound := &CompoundCommand{
		Name:     h.groupName,
		Commands: h.groupCmds,
	}

	h.pushLocked(compound)
	h.groupCmds = nil
}

// CancelGroup cancels a command group without adding to history.
// Note: Commands already executed still affect the buffer!
func (h *History) CancelGroup() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.grouping = false
	h.groupCmds = nil
}

// IsGrouping returns true if currently in a command group.
func (h *History) IsGrouping() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.grouping
}

// Clear removes all undo/redo history.
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.undoStack = nil
	h.redoStack = nil
	h.grouping = false
	h.groupCmds = nil
}

// UndoInfo returns info about available undo operations.
func (h *History) UndoInfo() []OperationInfo {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]OperationInfo, len(h.undoStack))
	for i, entry := range h.undoStack {
		result[i] = OperationInfo{
			Description: entry.command.Description(),
			Timestamp:   entry.timestamp,
		}
	}
	return result
}

// RedoInfo returns info about available redo operations.
func (h *History) RedoInfo() []OperationInfo {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]OperationInfo, len(h.redoStack))
	for i, entry := range h.redoStack {
		result[i] = OperationInfo{
			Description: entry.command.Description(),
			Timestamp:   entry.timestamp,
		}
	}
	return result
}

// PeekUndo returns info about the next undo operation without removing it.
func (h *History) PeekUndo() (OperationInfo, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.undoStack) == 0 {
		return OperationInfo{}, false
	}

	entry := h.undoStack[len(h.undoStack)-1]
	return OperationInfo{
		Description: entry.command.Description(),
		Timestamp:   entry.timestamp,
	}, true
}

// PeekRedo returns info about the next redo operation without removing it.
func (h *History) PeekRedo() (OperationInfo, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.redoStack) == 0 {
		return OperationInfo{}, false
	}

	entry := h.redoStack[len(h.redoStack)-1]
	return OperationInfo{
		Description: entry.command.Description(),
		Timestamp:   entry.timestamp,
	}, true
}

// SetMaxEntries changes the maximum number of undo entries.
// If the current stack is larger, oldest entries are removed.
func (h *History) SetMaxEntries(max int) {
	if max <= 0 {
		max = 1000
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.maxEntries = max

	if len(h.undoStack) > max {
		excess := len(h.undoStack) - max
		h.undoStack = h.undoStack[excess:]
	}
}

// MaxEntries returns the maximum number of undo entries.
func (h *History) MaxEntries() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.maxEntries
}
