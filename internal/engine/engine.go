package engine

import (
	"io"
	"sync"

	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/engine/history"
	"github.com/dshills/keystorm/internal/engine/rope"
	"github.com/dshills/keystorm/internal/engine/tracking"
)

// Re-export commonly used types for convenience.
type (
	// ByteOffset is a byte position in the buffer.
	ByteOffset = buffer.ByteOffset

	// Point represents a line/column position.
	Point = buffer.Point

	// PointUTF16 represents a UTF-16 line/column position (for LSP).
	PointUTF16 = buffer.PointUTF16

	// Range represents a byte range in the buffer.
	Range = buffer.Range

	// Edit represents an edit operation.
	Edit = buffer.Edit

	// EditResult contains information about a completed edit.
	EditResult = buffer.EditResult

	// Selection represents a cursor selection.
	Selection = cursor.Selection

	// LineEnding specifies the line ending style.
	LineEnding = buffer.LineEnding

	// RevisionID uniquely identifies a buffer revision.
	RevisionID = buffer.RevisionID

	// SnapshotID uniquely identifies a named snapshot.
	SnapshotID = tracking.SnapshotID

	// Change represents a tracked change.
	Change = tracking.Change

	// ChangeType categorizes changes.
	ChangeType = tracking.ChangeType

	// DiffResult contains the result of a diff operation.
	DiffResult = tracking.DiffResult

	// DiffOptions configures diff computation.
	DiffOptions = tracking.DiffOptions

	// Command is an undoable edit command.
	Command = history.Command
)

// Re-export constants.
const (
	LineEndingLF   = buffer.LineEndingLF
	LineEndingCRLF = buffer.LineEndingCRLF
	LineEndingCR   = buffer.LineEndingCR

	ChangeInsert  = tracking.ChangeInsert
	ChangeDelete  = tracking.ChangeDelete
	ChangeReplace = tracking.ChangeReplace
)

// appliedEditCommand represents an edit that has already been applied to the buffer.
// It stores the information needed to undo/redo the edit.
type appliedEditCommand struct {
	oldRange      Range
	newRange      Range
	oldText       string
	newText       string
	cursorsBefore []Selection
	cursorsAfter  []Selection
}

// Execute re-applies the edit (used for redo).
func (c *appliedEditCommand) Execute(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	// Replace old text with new text
	_, err := buf.Replace(c.oldRange.Start, c.oldRange.End, c.newText)
	if err != nil {
		return err
	}
	cursors.SetAll(c.cursorsAfter)
	return nil
}

// Undo reverses the edit.
func (c *appliedEditCommand) Undo(buf *buffer.Buffer, cursors *cursor.CursorSet) error {
	// Replace new text with old text
	_, err := buf.Replace(c.newRange.Start, c.newRange.End, c.oldText)
	if err != nil {
		return err
	}
	cursors.SetAll(c.cursorsBefore)
	return nil
}

// Description returns a human-readable description.
func (c *appliedEditCommand) Description() string {
	if c.oldRange.IsEmpty() {
		return "Insert"
	}
	if c.newText == "" {
		return "Delete"
	}
	return "Replace"
}

// Engine is the main facade for the text editor engine.
// It combines buffer management, cursor handling, undo/redo,
// and change tracking into a unified, thread-safe API.
//
// All operations are thread-safe and can be called from multiple goroutines.
type Engine struct {
	mu sync.RWMutex

	// Core components
	buf     *buffer.Buffer
	cursors *cursor.CursorSet
	history *history.History
	tracker *tracking.Tracker

	// Configuration
	tabWidth       int
	lineEnding     buffer.LineEnding
	maxUndoEntries int
	maxChanges     int
	maxRevisions   int
	readOnly       bool

	// Initialization
	initContent string
}

// New creates a new Engine with the given options.
func New(opts ...Option) *Engine {
	e := &Engine{
		tabWidth:       DefaultTabWidth,
		lineEnding:     buffer.LineEndingLF,
		maxUndoEntries: DefaultMaxUndoEntries,
		maxChanges:     DefaultMaxChanges,
		maxRevisions:   DefaultMaxRevisions,
	}

	// Apply options to get configuration
	for _, opt := range opts {
		opt(e)
	}

	// Create buffer with configured options
	bufOpts := []buffer.Option{
		buffer.WithTabWidth(e.tabWidth),
		buffer.WithLineEnding(e.lineEnding),
	}
	if e.initContent != "" {
		e.buf = buffer.NewBufferFromString(e.initContent, bufOpts...)
	} else {
		e.buf = buffer.NewBuffer(bufOpts...)
	}

	// Create cursor set at start of buffer
	e.cursors = cursor.NewCursorSetAt(0)

	// Create history manager
	e.history = history.NewHistory(e.maxUndoEntries)

	// Create change tracker
	e.tracker = tracking.NewTracker(
		tracking.WithMaxChanges(e.maxChanges),
		tracking.WithMaxRevisions(e.maxRevisions),
	)

	return e
}

// NewFromReader creates an Engine from an io.Reader.
func NewFromReader(r io.Reader, opts ...Option) (*Engine, error) {
	e := &Engine{
		tabWidth:       DefaultTabWidth,
		lineEnding:     buffer.LineEndingLF,
		maxUndoEntries: DefaultMaxUndoEntries,
		maxChanges:     DefaultMaxChanges,
		maxRevisions:   DefaultMaxRevisions,
	}

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	// Create buffer from reader
	bufOpts := []buffer.Option{
		buffer.WithTabWidth(e.tabWidth),
		buffer.WithLineEnding(e.lineEnding),
	}
	var err error
	e.buf, err = buffer.NewBufferFromReader(r, bufOpts...)
	if err != nil {
		return nil, err
	}

	// Create cursor set at start
	e.cursors = cursor.NewCursorSetAt(0)

	// Create history manager
	e.history = history.NewHistory(e.maxUndoEntries)

	// Create change tracker
	e.tracker = tracking.NewTracker(
		tracking.WithMaxChanges(e.maxChanges),
		tracking.WithMaxRevisions(e.maxRevisions),
	)

	return e, nil
}

// ============================================================================
// Read Operations (Buffer interface)
// ============================================================================

// Text returns the full buffer content.
// For large buffers, prefer using TextRange or iterators.
func (e *Engine) Text() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.Text()
}

// TextRange returns text in the given byte range.
func (e *Engine) TextRange(start, end ByteOffset) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.TextRange(start, end)
}

// Len returns the total byte length of the buffer.
func (e *Engine) Len() ByteOffset {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.Len()
}

// LineCount returns the number of lines.
func (e *Engine) LineCount() uint32 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.LineCount()
}

// LineText returns the text of a specific line (without newline).
func (e *Engine) LineText(line uint32) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.LineText(line)
}

// LineLen returns the length of a specific line in bytes (without newline).
func (e *Engine) LineLen(line uint32) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.LineLen(line)
}

// ByteAt returns the byte at the given offset.
func (e *Engine) ByteAt(offset ByteOffset) (byte, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.ByteAt(offset)
}

// RuneAt returns the rune at the given byte offset.
func (e *Engine) RuneAt(offset ByteOffset) (rune, int) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.RuneAt(offset)
}

// IsEmpty returns true if the buffer is empty.
func (e *Engine) IsEmpty() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.IsEmpty()
}

// ============================================================================
// Position Conversion
// ============================================================================

// OffsetToPoint converts a byte offset to line/column.
func (e *Engine) OffsetToPoint(offset ByteOffset) Point {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.OffsetToPoint(offset)
}

// PointToOffset converts line/column to byte offset.
func (e *Engine) PointToOffset(point Point) ByteOffset {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.PointToOffset(point)
}

// OffsetToPointUTF16 converts a byte offset to UTF-16 line/column.
func (e *Engine) OffsetToPointUTF16(offset ByteOffset) PointUTF16 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.OffsetToPointUTF16(offset)
}

// PointUTF16ToOffset converts UTF-16 line/column to byte offset.
func (e *Engine) PointUTF16ToOffset(point PointUTF16) ByteOffset {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.PointUTF16ToOffset(point)
}

// LineStartOffset returns the byte offset of the start of a line.
func (e *Engine) LineStartOffset(line uint32) ByteOffset {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.LineStartOffset(line)
}

// LineEndOffset returns the byte offset of the end of a line (before newline).
func (e *Engine) LineEndOffset(line uint32) ByteOffset {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.LineEndOffset(line)
}

// ============================================================================
// Write Operations
// ============================================================================

// Insert inserts text at the given offset.
// Returns the end position of the inserted text.
func (e *Engine) Insert(offset ByteOffset, text string) (ByteOffset, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.readOnly {
		return 0, ErrReadOnly
	}

	return e.insertLocked(offset, text)
}

// insertLocked performs insertion without acquiring the lock.
func (e *Engine) insertLocked(offset ByteOffset, text string) (ByteOffset, error) {
	// Capture state before change
	beforeRope := e.buf.Snapshot().Rope()
	cursorsBefore := e.cursors.All()

	// Apply the edit
	endPos, err := e.buf.Insert(offset, text)
	if err != nil {
		return 0, err
	}

	// Record change for tracking
	change := tracking.NewInsertChange(offset, text, e.buf.RevisionID())
	e.tracker.RecordChange(e.buf.RevisionID(), change, beforeRope)

	// Update cursors
	edit := Edit{Range: Range{Start: offset, End: offset}, NewText: text}
	cursor.TransformCursorSet(e.cursors, edit)

	// Record for undo with full state
	cmd := &appliedEditCommand{
		oldRange:      Range{Start: offset, End: offset},
		newRange:      Range{Start: offset, End: endPos},
		oldText:       "",
		newText:       text,
		cursorsBefore: cursorsBefore,
		cursorsAfter:  e.cursors.All(),
	}
	e.history.Push(cmd)

	return endPos, nil
}

// Delete removes text in the given range.
func (e *Engine) Delete(start, end ByteOffset) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.readOnly {
		return ErrReadOnly
	}

	return e.deleteLocked(start, end)
}

// deleteLocked performs deletion without acquiring the lock.
func (e *Engine) deleteLocked(start, end ByteOffset) error {
	// Capture state before change
	beforeRope := e.buf.Snapshot().Rope()
	oldText := e.buf.TextRange(start, end)
	cursorsBefore := e.cursors.All()

	// Apply the edit
	if err := e.buf.Delete(start, end); err != nil {
		return err
	}

	// Record change for tracking
	change := tracking.NewDeleteChange(start, end, oldText, e.buf.RevisionID())
	e.tracker.RecordChange(e.buf.RevisionID(), change, beforeRope)

	// Update cursors
	edit := Edit{Range: Range{Start: start, End: end}, NewText: ""}
	cursor.TransformCursorSet(e.cursors, edit)

	// Record for undo with full state
	cmd := &appliedEditCommand{
		oldRange:      Range{Start: start, End: end},
		newRange:      Range{Start: start, End: start},
		oldText:       oldText,
		newText:       "",
		cursorsBefore: cursorsBefore,
		cursorsAfter:  e.cursors.All(),
	}
	e.history.Push(cmd)

	return nil
}

// Replace replaces text in the given range with new text.
// Returns the end position of the replacement text.
func (e *Engine) Replace(start, end ByteOffset, text string) (ByteOffset, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.readOnly {
		return 0, ErrReadOnly
	}

	return e.replaceLocked(start, end, text)
}

// replaceLocked performs replacement without acquiring the lock.
func (e *Engine) replaceLocked(start, end ByteOffset, text string) (ByteOffset, error) {
	// Capture state before change
	beforeRope := e.buf.Snapshot().Rope()
	oldText := e.buf.TextRange(start, end)
	cursorsBefore := e.cursors.All()

	// Apply the edit
	endPos, err := e.buf.Replace(start, end, text)
	if err != nil {
		return 0, err
	}

	// Record change for tracking
	change := tracking.NewReplaceChange(start, end, oldText, text, e.buf.RevisionID())
	e.tracker.RecordChange(e.buf.RevisionID(), change, beforeRope)

	// Update cursors
	edit := Edit{Range: Range{Start: start, End: end}, NewText: text}
	cursor.TransformCursorSet(e.cursors, edit)

	// Record for undo with full state
	cmd := &appliedEditCommand{
		oldRange:      Range{Start: start, End: end},
		newRange:      Range{Start: start, End: endPos},
		oldText:       oldText,
		newText:       text,
		cursorsBefore: cursorsBefore,
		cursorsAfter:  e.cursors.All(),
	}
	e.history.Push(cmd)

	return endPos, nil
}

// ApplyEdit applies a single edit operation.
func (e *Engine) ApplyEdit(edit Edit) (EditResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.readOnly {
		return EditResult{}, ErrReadOnly
	}

	// Capture state before change
	beforeRope := e.buf.Snapshot().Rope()
	oldText := e.buf.TextRange(edit.Range.Start, edit.Range.End)
	cursorsBefore := e.cursors.All()

	// Apply the edit
	result, err := e.buf.ApplyEdit(edit)
	if err != nil {
		return EditResult{}, err
	}

	// Determine change type and record
	change := tracking.FromBufferEdit(result, edit.NewText, e.buf.RevisionID())
	e.tracker.RecordChange(e.buf.RevisionID(), change, beforeRope)

	// Update cursors
	cursor.TransformCursorSet(e.cursors, edit)

	// Record for undo with full state
	cmd := &appliedEditCommand{
		oldRange:      edit.Range,
		newRange:      result.NewRange,
		oldText:       oldText,
		newText:       edit.NewText,
		cursorsBefore: cursorsBefore,
		cursorsAfter:  e.cursors.All(),
	}
	e.history.Push(cmd)

	return result, nil
}

// ApplyEdits applies multiple edits atomically.
// Edits must be in reverse order (highest offset first).
func (e *Engine) ApplyEdits(edits []Edit) error {
	if len(edits) == 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.readOnly {
		return ErrReadOnly
	}

	// Capture state before change
	beforeRope := e.buf.Snapshot().Rope()
	cursorsBefore := e.cursors.All()

	// Collect old texts for tracking and undo
	changes := make([]tracking.Change, len(edits))
	oldTexts := make([]string, len(edits))
	for i, edit := range edits {
		oldText := e.buf.TextRange(edit.Range.Start, edit.Range.End)
		oldTexts[i] = oldText
		changes[i] = tracking.Change{
			Range:   edit.Range,
			OldText: oldText,
			NewText: edit.NewText,
		}
	}

	// Apply all edits
	if err := e.buf.ApplyEdits(edits); err != nil {
		return err
	}

	// Update change types and revision
	revID := e.buf.RevisionID()
	for i := range changes {
		changes[i].RevisionID = revID
		if changes[i].Range.IsEmpty() {
			changes[i].Type = tracking.ChangeInsert
		} else if changes[i].NewText == "" {
			changes[i].Type = tracking.ChangeDelete
		} else {
			changes[i].Type = tracking.ChangeReplace
		}
	}

	// Record all changes
	e.tracker.RecordChanges(revID, changes, beforeRope)

	// Update cursors for each edit
	for _, edit := range edits {
		cursor.TransformCursorSet(e.cursors, edit)
	}

	// Create a compound command for atomic undo
	// We need to create commands in reverse order for proper undo
	cmds := make([]Command, len(edits))
	delta := ByteOffset(0)
	for i, edit := range edits {
		// Calculate the new range after all subsequent edits have been applied
		oldLen := edit.Range.End - edit.Range.Start
		newLen := ByteOffset(len(edit.NewText))
		adjustedStart := edit.Range.Start + delta
		cmds[i] = &appliedEditCommand{
			oldRange:      edit.Range,
			newRange:      Range{Start: adjustedStart, End: adjustedStart + newLen},
			oldText:       oldTexts[i],
			newText:       edit.NewText,
			cursorsBefore: cursorsBefore,
			cursorsAfter:  e.cursors.All(),
		}
		delta += newLen - oldLen
	}

	// Push compound command
	compound := history.NewCompoundCommand("multi-edit", cmds...)
	e.history.Push(compound)

	return nil
}

// ============================================================================
// Undo/Redo Operations
// ============================================================================

// Undo undoes the last operation.
func (e *Engine) Undo() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.readOnly {
		return ErrReadOnly
	}

	return e.history.Undo(e.buf, e.cursors)
}

// Redo redoes the last undone operation.
func (e *Engine) Redo() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.readOnly {
		return ErrReadOnly
	}

	return e.history.Redo(e.buf, e.cursors)
}

// CanUndo returns true if undo is available.
func (e *Engine) CanUndo() bool {
	return e.history.CanUndo()
}

// CanRedo returns true if redo is available.
func (e *Engine) CanRedo() bool {
	return e.history.CanRedo()
}

// UndoCount returns the number of available undo operations.
func (e *Engine) UndoCount() int {
	return e.history.UndoCount()
}

// RedoCount returns the number of available redo operations.
func (e *Engine) RedoCount() int {
	return e.history.RedoCount()
}

// BeginUndoGroup starts a new undo group.
// All operations until EndUndoGroup will be undone as a single unit.
func (e *Engine) BeginUndoGroup(name string) {
	e.history.BeginGroup(name)
}

// EndUndoGroup ends the current undo group.
func (e *Engine) EndUndoGroup() {
	e.history.EndGroup()
}

// CancelUndoGroup cancels the current undo group without recording.
func (e *Engine) CancelUndoGroup() {
	e.history.CancelGroup()
}

// ClearHistory removes all undo/redo history.
func (e *Engine) ClearHistory() {
	e.history.Clear()
}

// ============================================================================
// Command Execution
// ============================================================================

// Execute runs a command and adds it to undo history.
func (e *Engine) Execute(cmd Command) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.readOnly {
		return ErrReadOnly
	}

	return e.history.Execute(cmd, e.buf, e.cursors)
}

// ============================================================================
// Cursor Operations
// ============================================================================

// Cursors returns the cursor set for direct manipulation.
// The returned CursorSet is safe for concurrent read operations.
func (e *Engine) Cursors() *cursor.CursorSet {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cursors.Clone()
}

// SetCursors replaces the cursor set.
func (e *Engine) SetCursors(cs *cursor.CursorSet) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cursors = cs.Clone()
}

// PrimaryCursor returns the primary cursor offset.
func (e *Engine) PrimaryCursor() ByteOffset {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cursors.PrimaryCursor()
}

// PrimarySelection returns the primary selection.
func (e *Engine) PrimarySelection() Selection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cursors.Primary()
}

// SetPrimaryCursor sets the primary cursor position.
func (e *Engine) SetPrimaryCursor(offset ByteOffset) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cursors.Set(cursor.NewCursorSelection(offset))
}

// SetPrimarySelection sets the primary selection.
func (e *Engine) SetPrimarySelection(sel Selection) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cursors.Set(sel)
}

// CursorCount returns the number of cursors.
func (e *Engine) CursorCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cursors.Count()
}

// HasMultipleCursors returns true if there are multiple cursors.
func (e *Engine) HasMultipleCursors() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cursors.IsMulti()
}

// AddCursor adds a new cursor at the given offset.
func (e *Engine) AddCursor(offset ByteOffset) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cursors.Add(cursor.NewCursorSelection(offset))
}

// AddSelection adds a new selection.
func (e *Engine) AddSelection(sel Selection) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cursors.Add(sel)
}

// ClearSecondary removes all cursors except the primary.
func (e *Engine) ClearSecondary() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cursors.Clear()
}

// ClampCursors ensures all cursors are within valid buffer range.
func (e *Engine) ClampCursors() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cursors.Clamp(e.buf.Len())
}

// ============================================================================
// Snapshot and Tracking Operations
// ============================================================================

// CreateSnapshot creates a named snapshot of the current state.
func (e *Engine) CreateSnapshot(name string) SnapshotID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.tracker.CreateSnapshot(name, e.buf.Snapshot().Rope(), e.buf.RevisionID())
}

// GetSnapshot retrieves a snapshot by ID.
func (e *Engine) GetSnapshot(id SnapshotID) (*tracking.Snapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.GetSnapshot(id)
}

// GetSnapshotByName retrieves a snapshot by name.
func (e *Engine) GetSnapshotByName(name string) (*tracking.Snapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.GetSnapshotByName(name)
}

// GetSnapshotText returns the full text from a snapshot.
func (e *Engine) GetSnapshotText(id SnapshotID) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.GetSnapshotText(id)
}

// DeleteSnapshot removes a snapshot.
func (e *Engine) DeleteSnapshot(id SnapshotID) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tracker.DeleteSnapshot(id)
}

// DeleteSnapshotByName removes a snapshot by name.
func (e *Engine) DeleteSnapshotByName(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tracker.DeleteSnapshotByName(name)
}

// ListSnapshots returns all snapshots.
func (e *Engine) ListSnapshots() []*tracking.Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.ListSnapshots()
}

// SnapshotCount returns the number of snapshots.
func (e *Engine) SnapshotCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.SnapshotCount()
}

// ============================================================================
// Change Tracking Operations
// ============================================================================

// RevisionID returns the current buffer revision.
func (e *Engine) RevisionID() RevisionID {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.RevisionID()
}

// ChangesSince returns all changes since a revision.
func (e *Engine) ChangesSince(rev RevisionID) []Change {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.ChangesSince(rev)
}

// ChangesSinceWithLimit returns up to limit changes since a revision.
func (e *Engine) ChangesSinceWithLimit(rev RevisionID, limit int) []Change {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.ChangesSinceWithLimit(rev, limit)
}

// ChangesBetween returns changes between two revisions.
func (e *Engine) ChangesBetween(startRev, endRev RevisionID) []Change {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.ChangesBetween(startRev, endRev)
}

// LatestChanges returns the most recent N changes.
func (e *Engine) LatestChanges(n int) []Change {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.LatestChanges(n)
}

// ChangeCount returns the number of tracked changes.
func (e *Engine) ChangeCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.ChangeCount()
}

// ============================================================================
// Diff Operations
// ============================================================================

// DiffSinceSnapshot returns changes since a snapshot.
func (e *Engine) DiffSinceSnapshot(id SnapshotID) ([]Change, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.DiffSinceSnapshot(id)
}

// ComputeDiffSinceSnapshot computes a line-level diff from a snapshot to current state.
func (e *Engine) ComputeDiffSinceSnapshot(id SnapshotID, opts DiffOptions) (DiffResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.ComputeDiffSinceSnapshot(id, e.buf.Snapshot().Rope(), opts)
}

// ComputeDiffBetweenSnapshots computes a line-level diff between two snapshots.
func (e *Engine) ComputeDiffBetweenSnapshots(fromID, toID SnapshotID, opts DiffOptions) (DiffResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.ComputeDiffBetweenSnapshots(fromID, toID, opts)
}

// ============================================================================
// AI Context Operations
// ============================================================================

// GetAIContext returns a summary suitable for AI context.
func (e *Engine) GetAIContext(opts tracking.AIContextOptions) tracking.AIContext {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tracker.GetAIContext(e.buf.Snapshot().Rope(), opts)
}

// ============================================================================
// Configuration
// ============================================================================

// TabWidth returns the tab width.
func (e *Engine) TabWidth() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.TabWidth()
}

// SetTabWidth sets the tab width.
func (e *Engine) SetTabWidth(width int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buf.SetTabWidth(width)
}

// LineEnding returns the line ending style.
func (e *Engine) LineEnding() LineEnding {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.LineEnding()
}

// SetLineEnding sets the line ending style.
func (e *Engine) SetLineEnding(ending LineEnding) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buf.SetLineEnding(ending)
}

// IsReadOnly returns true if the engine is read-only.
func (e *Engine) IsReadOnly() bool {
	return e.readOnly
}

// ============================================================================
// Buffer Snapshot
// ============================================================================

// Snapshot returns a read-only snapshot of the current buffer state.
func (e *Engine) Snapshot() *buffer.Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.Snapshot()
}

// Rope returns the underlying rope (read-only, for advanced operations).
func (e *Engine) Rope() rope.Rope {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.buf.Snapshot().Rope()
}

// ============================================================================
// Clear and Reset
// ============================================================================

// Clear removes all content from the buffer and resets history.
func (e *Engine) Clear() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.readOnly {
		return ErrReadOnly
	}

	// Clear buffer content
	if e.buf.Len() > 0 {
		if err := e.buf.Delete(0, e.buf.Len()); err != nil {
			return err
		}
	}

	// Reset cursors
	e.cursors = cursor.NewCursorSetAt(0)

	// Clear history
	e.history.Clear()

	// Clear tracking
	e.tracker.Clear()

	return nil
}

// SetContent replaces all content and resets history.
func (e *Engine) SetContent(content string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.readOnly {
		return ErrReadOnly
	}

	// Replace buffer content
	_, err := e.buf.Replace(0, e.buf.Len(), content)
	if err != nil {
		return err
	}

	// Reset cursors to start
	e.cursors = cursor.NewCursorSetAt(0)

	// Clear history
	e.history.Clear()

	// Clear tracking
	e.tracker.Clear()

	return nil
}
