package history

import (
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
)

// GroupScope provides a convenient way to group commands using defer.
// Usage:
//
//	func doComplexEdit(h *History, buf *buffer.Buffer, cursors *cursor.CursorSet) {
//	    defer h.GroupScope("Complex Edit").End()
//	    // ... multiple edits ...
//	}
type GroupScope struct {
	history *History
	active  bool
}

// GroupScope starts a new group scope.
// Call End() or use with defer to properly close the group.
func (h *History) GroupScope(name string) *GroupScope {
	h.BeginGroup(name)
	return &GroupScope{
		history: h,
		active:  true,
	}
}

// End ends the group scope.
// Safe to call multiple times; only the first call has effect.
func (g *GroupScope) End() {
	if g.active {
		g.history.EndGroup()
		g.active = false
	}
}

// Cancel cancels the group scope without creating a compound command.
// Note: Commands already executed still affect the buffer.
func (g *GroupScope) Cancel() {
	if g.active {
		g.history.CancelGroup()
		g.active = false
	}
}

// Transaction executes a function within a grouped undo context.
// If the function returns an error, the group is cancelled.
// Otherwise, the group is ended normally.
func (h *History) Transaction(name string, fn func() error) error {
	h.BeginGroup(name)

	err := fn()
	if err != nil {
		h.CancelGroup()
		return err
	}

	h.EndGroup()
	return nil
}

// ExecuteGrouped executes multiple commands as a single undo unit.
func (h *History) ExecuteGrouped(name string, buf *buffer.Buffer, cursors *cursor.CursorSet, cmds ...Command) error {
	if len(cmds) == 0 {
		return nil
	}

	if len(cmds) == 1 {
		// Single command doesn't need grouping
		return h.Execute(cmds[0], buf, cursors)
	}

	h.BeginGroup(name)
	for _, cmd := range cmds {
		if err := h.Execute(cmd, buf, cursors); err != nil {
			h.CancelGroup()
			return err
		}
	}
	h.EndGroup()
	return nil
}

// Checkpoint represents a point in history that can be returned to.
type Checkpoint struct {
	undoDepth int
}

// CreateCheckpoint creates a checkpoint at the current history position.
func (h *History) CreateCheckpoint() Checkpoint {
	h.mu.Lock()
	defer h.mu.Unlock()
	return Checkpoint{undoDepth: len(h.undoStack)}
}

// UndoToCheckpoint undoes all operations since the checkpoint.
func (h *History) UndoToCheckpoint(cp Checkpoint, buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	for h.UndoCount() > cp.undoDepth {
		if err := h.Undo(buf, cursors); err != nil {
			return err
		}
	}
	return nil
}

// RedoToCheckpoint redoes all operations up to the checkpoint depth.
// Note: This only works if the redo stack has the operations.
func (h *History) RedoToCheckpoint(cp Checkpoint, buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	for h.UndoCount() < cp.undoDepth && h.CanRedo() {
		if err := h.Redo(buf, cursors); err != nil {
			return err
		}
	}
	return nil
}
