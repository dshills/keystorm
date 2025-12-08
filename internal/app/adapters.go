// Package app provides adapter implementations that bridge the app layer
// with the dispatcher's execution context interfaces.
package app

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/engine"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input/mode"
)

// Compile-time interface checks.
var (
	_ execctx.EngineInterface        = (*EngineExecAdapter)(nil)
	_ execctx.CursorManagerInterface = (*CursorManagerAdapter)(nil)
	_ execctx.ModeManagerInterface   = (*ModeExecAdapter)(nil)
	_ execctx.HistoryInterface       = (*HistoryAdapter)(nil)
	_ execctx.RendererInterface      = (*RendererAdapter)(nil)
)

// EngineExecAdapter adapts engine.Engine to execctx.EngineInterface.
type EngineExecAdapter struct {
	eng *engine.Engine
}

// NewEngineExecAdapter creates a new engine adapter for execctx.
func NewEngineExecAdapter(eng *engine.Engine) *EngineExecAdapter {
	return &EngineExecAdapter{eng: eng}
}

// Insert inserts text at the given offset.
func (a *EngineExecAdapter) Insert(offset buffer.ByteOffset, text string) (buffer.EditResult, error) {
	endOffset, err := a.eng.Insert(offset, text)
	if err != nil {
		return buffer.EditResult{}, err
	}
	// Construct EditResult from the operation
	return buffer.EditResult{
		OldRange: buffer.Range{Start: offset, End: offset},
		NewRange: buffer.Range{Start: offset, End: endOffset},
		Delta:    int64(len(text)),
	}, nil
}

// Delete removes text between start and end offsets.
func (a *EngineExecAdapter) Delete(start, end buffer.ByteOffset) (buffer.EditResult, error) {
	oldText := a.eng.TextRange(start, end)
	err := a.eng.Delete(start, end)
	if err != nil {
		return buffer.EditResult{}, err
	}
	return buffer.EditResult{
		OldRange: buffer.Range{Start: start, End: end},
		NewRange: buffer.Range{Start: start, End: start},
		OldText:  oldText,
	}, nil
}

// Replace replaces text between start and end with new text.
func (a *EngineExecAdapter) Replace(start, end buffer.ByteOffset, text string) (buffer.EditResult, error) {
	oldText := a.eng.TextRange(start, end)
	newEnd, err := a.eng.Replace(start, end, text)
	if err != nil {
		return buffer.EditResult{}, err
	}
	return buffer.EditResult{
		OldRange: buffer.Range{Start: start, End: end},
		NewRange: buffer.Range{Start: start, End: newEnd},
		OldText:  oldText,
		Delta:    int64(len(text)) - int64(end-start),
	}, nil
}

// Text returns the full document text.
func (a *EngineExecAdapter) Text() string {
	return a.eng.Text()
}

// TextRange returns text in the given range.
func (a *EngineExecAdapter) TextRange(start, end buffer.ByteOffset) string {
	return a.eng.TextRange(start, end)
}

// LineText returns the text of the given line.
func (a *EngineExecAdapter) LineText(line uint32) string {
	return a.eng.LineText(line)
}

// Len returns the total byte length.
func (a *EngineExecAdapter) Len() buffer.ByteOffset {
	return a.eng.Len()
}

// LineCount returns the number of lines.
func (a *EngineExecAdapter) LineCount() uint32 {
	return a.eng.LineCount()
}

// LineStartOffset returns the start offset of a line.
func (a *EngineExecAdapter) LineStartOffset(line uint32) buffer.ByteOffset {
	return a.eng.LineStartOffset(line)
}

// LineEndOffset returns the end offset of a line.
func (a *EngineExecAdapter) LineEndOffset(line uint32) buffer.ByteOffset {
	return a.eng.LineEndOffset(line)
}

// LineLen returns the length of a line.
func (a *EngineExecAdapter) LineLen(line uint32) uint32 {
	return uint32(a.eng.LineLen(line))
}

// OffsetToPoint converts a byte offset to a point (line, column).
func (a *EngineExecAdapter) OffsetToPoint(offset buffer.ByteOffset) buffer.Point {
	return a.eng.OffsetToPoint(offset)
}

// PointToOffset converts a point to a byte offset.
func (a *EngineExecAdapter) PointToOffset(point buffer.Point) buffer.ByteOffset {
	return a.eng.PointToOffset(point)
}

// RevisionID returns the current revision ID.
func (a *EngineExecAdapter) RevisionID() buffer.RevisionID {
	return a.eng.RevisionID()
}

// Snapshot returns a read-only snapshot of the engine.
func (a *EngineExecAdapter) Snapshot() execctx.EngineReader {
	return &engineReaderAdapter{eng: a.eng}
}

// engineReaderAdapter provides read-only access to the engine.
type engineReaderAdapter struct {
	eng *engine.Engine
}

func (r *engineReaderAdapter) Text() string                            { return r.eng.Text() }
func (r *engineReaderAdapter) TextRange(s, e buffer.ByteOffset) string { return r.eng.TextRange(s, e) }
func (r *engineReaderAdapter) LineText(line uint32) string             { return r.eng.LineText(line) }
func (r *engineReaderAdapter) Len() buffer.ByteOffset                  { return r.eng.Len() }
func (r *engineReaderAdapter) LineCount() uint32                       { return r.eng.LineCount() }
func (r *engineReaderAdapter) LineStartOffset(line uint32) buffer.ByteOffset {
	return r.eng.LineStartOffset(line)
}
func (r *engineReaderAdapter) LineEndOffset(line uint32) buffer.ByteOffset {
	return r.eng.LineEndOffset(line)
}
func (r *engineReaderAdapter) LineLen(line uint32) uint32 { return uint32(r.eng.LineLen(line)) }
func (r *engineReaderAdapter) OffsetToPoint(o buffer.ByteOffset) buffer.Point {
	return r.eng.OffsetToPoint(o)
}
func (r *engineReaderAdapter) PointToOffset(p buffer.Point) buffer.ByteOffset {
	return r.eng.PointToOffset(p)
}

// CursorManagerAdapter adapts cursor.CursorSet to execctx.CursorManagerInterface.
// It holds a reference to the engine so cursor modifications can be synced back.
//
// NOTE: engine.Cursors() returns a clone of the cursor set for thread safety.
// This adapter works on that clone and syncs changes back via SetCursors()
// after each mutating operation. SetCursors() also clones internally,
// maintaining the engine's thread-safety invariant.
type CursorManagerAdapter struct {
	eng     *engine.Engine
	cursors *cursor.CursorSet
}

// NewCursorManagerAdapter creates a new cursor manager adapter.
// It receives the engine to allow syncing cursor changes back.
func NewCursorManagerAdapter(eng *engine.Engine) *CursorManagerAdapter {
	return &CursorManagerAdapter{
		eng:     eng,
		cursors: eng.Cursors(), // Gets a clone for local modifications
	}
}

func (a *CursorManagerAdapter) Primary() cursor.Selection { return a.cursors.Primary() }
func (a *CursorManagerAdapter) SetPrimary(sel cursor.Selection) {
	a.cursors.SetPrimary(sel)
	a.syncToEngine()
}
func (a *CursorManagerAdapter) All() []cursor.Selection { return a.cursors.All() }
func (a *CursorManagerAdapter) Add(sel cursor.Selection) {
	a.cursors.Add(sel)
	a.syncToEngine()
}
func (a *CursorManagerAdapter) Clear() {
	a.cursors.Clear()
	a.syncToEngine()
}
func (a *CursorManagerAdapter) Count() int         { return a.cursors.Count() }
func (a *CursorManagerAdapter) IsMulti() bool      { return a.cursors.IsMulti() }
func (a *CursorManagerAdapter) HasSelection() bool { return a.cursors.HasSelection() }
func (a *CursorManagerAdapter) SetAll(sels []cursor.Selection) {
	a.cursors.SetAll(sels)
	a.syncToEngine()
}
func (a *CursorManagerAdapter) MapInPlace(f func(sel cursor.Selection) cursor.Selection) {
	a.cursors.MapInPlace(f)
	a.syncToEngine()
}
func (a *CursorManagerAdapter) Clone() *cursor.CursorSet { return a.cursors.Clone() }
func (a *CursorManagerAdapter) Clamp(maxOffset cursor.ByteOffset) {
	a.cursors.Clamp(maxOffset)
	a.syncToEngine()
}

// syncToEngine writes the cursor set back to the engine.
func (a *CursorManagerAdapter) syncToEngine() {
	if a.eng != nil {
		a.eng.SetCursors(a.cursors)
	}
}

// ModeExecAdapter adapts mode.Manager to execctx.ModeManagerInterface.
type ModeExecAdapter struct {
	manager *mode.Manager
}

// NewModeExecAdapter creates a new mode manager adapter for execctx.
func NewModeExecAdapter(manager *mode.Manager) *ModeExecAdapter {
	return &ModeExecAdapter{manager: manager}
}

// Current returns the current mode wrapped as ModeInterface.
func (a *ModeExecAdapter) Current() execctx.ModeInterface {
	if a.manager == nil {
		return nil
	}
	m := a.manager.Current()
	return &modeWrapper{mode: m}
}

// CurrentName returns the current mode name.
func (a *ModeExecAdapter) CurrentName() string {
	if a.manager == nil {
		return ""
	}
	return a.manager.Current().Name()
}

// Switch switches to a named mode.
func (a *ModeExecAdapter) Switch(name string) error {
	if a.manager == nil {
		return nil
	}
	return a.manager.SetInitialMode(name)
}

// Push pushes a new mode onto the stack (delegates to Switch for now).
func (a *ModeExecAdapter) Push(name string) error {
	return a.Switch(name)
}

// Pop pops the current mode from the stack (no-op for now).
func (a *ModeExecAdapter) Pop() error {
	return nil
}

// IsMode returns true if the current mode matches the given name.
func (a *ModeExecAdapter) IsMode(name string) bool {
	return a.CurrentName() == name
}

// IsAnyMode returns true if the current mode matches any of the given names.
func (a *ModeExecAdapter) IsAnyMode(names ...string) bool {
	current := a.CurrentName()
	for _, name := range names {
		if current == name {
			return true
		}
	}
	return false
}

// modeWrapper wraps mode.Mode to implement execctx.ModeInterface.
type modeWrapper struct {
	mode mode.Mode
}

func (w *modeWrapper) Name() string        { return w.mode.Name() }
func (w *modeWrapper) DisplayName() string { return w.mode.DisplayName() }

// HistoryAdapter adapts engine history to execctx.HistoryInterface.
type HistoryAdapter struct {
	eng *engine.Engine
}

// NewHistoryAdapter creates a new history adapter.
func NewHistoryAdapter(eng *engine.Engine) *HistoryAdapter {
	return &HistoryAdapter{eng: eng}
}

func (a *HistoryAdapter) BeginGroup(name string) {
	if a.eng != nil {
		a.eng.BeginUndoGroup(name)
	}
}

func (a *HistoryAdapter) EndGroup() {
	if a.eng != nil {
		a.eng.EndUndoGroup()
	}
}

func (a *HistoryAdapter) CancelGroup() {
	// CancelGroup not directly supported, use EndGroup
	if a.eng != nil {
		a.eng.EndUndoGroup()
	}
}

func (a *HistoryAdapter) IsGrouping() bool {
	// Engine doesn't expose grouping state directly
	return false
}

func (a *HistoryAdapter) CanUndo() bool {
	if a.eng != nil {
		return a.eng.CanUndo()
	}
	return false
}

func (a *HistoryAdapter) CanRedo() bool {
	if a.eng != nil {
		return a.eng.CanRedo()
	}
	return false
}

func (a *HistoryAdapter) UndoCount() int {
	// Engine doesn't expose undo count directly
	if a.eng != nil && a.eng.CanUndo() {
		return 1 // At least one undo available
	}
	return 0
}

func (a *HistoryAdapter) RedoCount() int {
	// Engine doesn't expose redo count directly
	if a.eng != nil && a.eng.CanRedo() {
		return 1 // At least one redo available
	}
	return 0
}

// RendererAdapter adapts the renderer to execctx.RendererInterface.
type RendererAdapter struct {
	renderer RendererInterface
}

// RendererInterface defines the renderer methods we need.
// This interface is satisfied by *renderer.RendererExecWrapper.
type RendererInterface interface {
	ScrollTo(line, col uint32)
	CenterOnLine(line uint32)
	Redraw()
	RedrawLines(lines []uint32)
	VisibleLineRange() (start, end uint32)
}

// NewRendererAdapter creates a new renderer adapter.
func NewRendererAdapter(renderer RendererInterface) *RendererAdapter {
	return &RendererAdapter{renderer: renderer}
}

func (a *RendererAdapter) ScrollTo(line, col uint32) {
	if a.renderer != nil {
		a.renderer.ScrollTo(line, col)
	}
}

func (a *RendererAdapter) CenterOnLine(line uint32) {
	if a.renderer != nil {
		a.renderer.CenterOnLine(line)
	}
}

func (a *RendererAdapter) Redraw() {
	if a.renderer != nil {
		a.renderer.Redraw()
	}
}

func (a *RendererAdapter) RedrawLines(lines []uint32) {
	if a.renderer != nil {
		a.renderer.RedrawLines(lines)
	}
}

func (a *RendererAdapter) VisibleLineRange() (start, end uint32) {
	if a.renderer != nil {
		return a.renderer.VisibleLineRange()
	}
	return 0, 0
}

// NullRenderer is a no-op renderer for testing.
type NullRenderer struct{}

func (NullRenderer) ScrollTo(line, col uint32)             {}
func (NullRenderer) CenterOnLine(line uint32)              {}
func (NullRenderer) Redraw()                               {}
func (NullRenderer) RedrawLines(lines []uint32)            {}
func (NullRenderer) VisibleLineRange() (start, end uint32) { return 0, 100 }

// RendererExecWrapper wraps a renderer.Renderer to implement RendererInterface.
// Uses minimal interface to avoid coupling to specific renderer implementation.
type RendererExecWrapper struct {
	scroller interface {
		ScrollToReveal(line uint32, col int, smooth bool)
		CenterOnLine(line uint32, smooth bool)
	}
	dirtyer interface {
		MarkDirty()
	}
}

// NewRendererExecWrapper creates a wrapper that adapts the renderer.
func NewRendererExecWrapper(r interface {
	ScrollToReveal(line uint32, col int, smooth bool)
	CenterOnLine(line uint32, smooth bool)
	MarkDirty()
}) *RendererExecWrapper {
	return &RendererExecWrapper{
		scroller: r,
		dirtyer:  r,
	}
}

func (w *RendererExecWrapper) ScrollTo(line, col uint32) {
	if w.scroller != nil {
		w.scroller.ScrollToReveal(line, int(col), false)
	}
}

func (w *RendererExecWrapper) CenterOnLine(line uint32) {
	if w.scroller != nil {
		w.scroller.CenterOnLine(line, false)
	}
}

func (w *RendererExecWrapper) Redraw() {
	if w.dirtyer != nil {
		w.dirtyer.MarkDirty()
	}
}

func (w *RendererExecWrapper) RedrawLines(lines []uint32) {
	// Simplified: just mark dirty for now
	if w.dirtyer != nil {
		w.dirtyer.MarkDirty()
	}
}

func (w *RendererExecWrapper) VisibleLineRange() (start, end uint32) {
	// TODO: Need to expose viewport's VisibleLineRange on Renderer
	// For now, return a reasonable default
	return 0, 100
}
