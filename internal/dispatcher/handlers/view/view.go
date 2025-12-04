// Package view provides handlers for view and scroll operations.
package view

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for view operations.
const (
	// Scrolling actions
	ActionScrollDown     = "view.scrollDown"     // Ctrl+E - scroll down one line
	ActionScrollUp       = "view.scrollUp"       // Ctrl+Y - scroll up one line
	ActionPageDown       = "view.pageDown"       // Ctrl+F - scroll down one page
	ActionPageUp         = "view.pageUp"         // Ctrl+B - scroll up one page
	ActionHalfPageDown   = "view.halfPageDown"   // Ctrl+D - scroll down half page
	ActionHalfPageUp     = "view.halfPageUp"     // Ctrl+U - scroll up half page
	ActionScrollToTop    = "view.scrollToTop"    // gg with scroll
	ActionScrollToBottom = "view.scrollToBottom" // G with scroll

	// Cursor positioning within view
	ActionMoveToTop    = "view.moveToTop"    // H - move cursor to top of screen
	ActionMoveToMiddle = "view.moveToMiddle" // M - move cursor to middle of screen
	ActionMoveToBottom = "view.moveToBottom" // L - move cursor to bottom of screen

	// View centering
	ActionCenterCursor = "view.centerCursor" // zz - center view on cursor
	ActionTopCursor    = "view.topCursor"    // zt - scroll cursor line to top
	ActionBottomCursor = "view.bottomCursor" // zb - scroll cursor line to bottom
)

// Handler implements namespace-based view handling.
type Handler struct{}

// NewHandler creates a new view handler.
func NewHandler() *Handler {
	return &Handler{}
}

// Namespace returns the view namespace.
func (h *Handler) Namespace() string {
	return "view"
}

// CanHandle returns true if this handler can process the action.
func (h *Handler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionScrollDown, ActionScrollUp, ActionPageDown, ActionPageUp,
		ActionHalfPageDown, ActionHalfPageUp, ActionScrollToTop, ActionScrollToBottom,
		ActionMoveToTop, ActionMoveToMiddle, ActionMoveToBottom,
		ActionCenterCursor, ActionTopCursor, ActionBottomCursor:
		return true
	}
	return false
}

// HandleAction processes a view action.
func (h *Handler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Engine == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}

	count := ctx.GetCount()

	switch action.Name {
	case ActionScrollDown:
		return h.scrollDown(ctx, count)
	case ActionScrollUp:
		return h.scrollUp(ctx, count)
	case ActionPageDown:
		return h.pageDown(ctx, count)
	case ActionPageUp:
		return h.pageUp(ctx, count)
	case ActionHalfPageDown:
		return h.halfPageDown(ctx, count)
	case ActionHalfPageUp:
		return h.halfPageUp(ctx, count)
	case ActionScrollToTop:
		return h.scrollToTop(ctx)
	case ActionScrollToBottom:
		return h.scrollToBottom(ctx)
	case ActionMoveToTop:
		return h.moveToTop(ctx)
	case ActionMoveToMiddle:
		return h.moveToMiddle(ctx)
	case ActionMoveToBottom:
		return h.moveToBottom(ctx)
	case ActionCenterCursor:
		return h.centerCursor(ctx)
	case ActionTopCursor:
		return h.topCursor(ctx)
	case ActionBottomCursor:
		return h.bottomCursor(ctx)
	default:
		return handler.Errorf("unknown view action: %s", action.Name)
	}
}

// scrollDown scrolls the view down by count lines.
func (h *Handler) scrollDown(ctx *execctx.ExecutionContext, count int) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}

	start, _ := ctx.Renderer.VisibleLineRange()
	newStart := start + uint32(count)

	// Clamp to buffer
	lineCount := ctx.Engine.LineCount()
	if lineCount > 0 && newStart >= lineCount {
		newStart = lineCount - 1
	}

	ctx.Renderer.ScrollTo(newStart, 0)

	// Move cursor if it's above the visible area
	if ctx.Cursors != nil {
		cursorLine := ctx.Engine.OffsetToPoint(ctx.Cursors.Primary().Head).Line
		if cursorLine < newStart {
			newOffset := ctx.Engine.LineStartOffset(newStart)
			sel := ctx.Cursors.Primary().MoveTo(newOffset)
			ctx.Cursors.SetPrimary(sel)
		}
	}

	return handler.Success().WithRedraw()
}

// scrollUp scrolls the view up by count lines.
func (h *Handler) scrollUp(ctx *execctx.ExecutionContext, count int) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}

	start, end := ctx.Renderer.VisibleLineRange()
	var newStart uint32
	if start >= uint32(count) {
		newStart = start - uint32(count)
	}

	ctx.Renderer.ScrollTo(newStart, 0)

	// Move cursor if it's below the visible area
	if ctx.Cursors != nil {
		cursorLine := ctx.Engine.OffsetToPoint(ctx.Cursors.Primary().Head).Line
		visibleHeight := end - start
		newEnd := newStart + visibleHeight
		if cursorLine >= newEnd {
			newOffset := ctx.Engine.LineStartOffset(newEnd - 1)
			sel := ctx.Cursors.Primary().MoveTo(newOffset)
			ctx.Cursors.SetPrimary(sel)
		}
	}

	return handler.Success().WithRedraw()
}

// pageDown scrolls down by one full page.
func (h *Handler) pageDown(ctx *execctx.ExecutionContext, count int) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}

	start, end := ctx.Renderer.VisibleLineRange()
	pageSize := end - start
	if pageSize < 1 {
		pageSize = 20 // Default page size
	}

	return h.scrollDown(ctx, int(pageSize)*count)
}

// pageUp scrolls up by one full page.
func (h *Handler) pageUp(ctx *execctx.ExecutionContext, count int) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}

	start, end := ctx.Renderer.VisibleLineRange()
	pageSize := end - start
	if pageSize < 1 {
		pageSize = 20
	}

	return h.scrollUp(ctx, int(pageSize)*count)
}

// halfPageDown scrolls down by half a page.
func (h *Handler) halfPageDown(ctx *execctx.ExecutionContext, count int) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}

	start, end := ctx.Renderer.VisibleLineRange()
	halfPage := (end - start) / 2
	if halfPage < 1 {
		halfPage = 10
	}

	return h.scrollDown(ctx, int(halfPage)*count)
}

// halfPageUp scrolls up by half a page.
func (h *Handler) halfPageUp(ctx *execctx.ExecutionContext, count int) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}

	start, end := ctx.Renderer.VisibleLineRange()
	halfPage := (end - start) / 2
	if halfPage < 1 {
		halfPage = 10
	}

	return h.scrollUp(ctx, int(halfPage)*count)
}

// scrollToTop scrolls to the top of the buffer.
func (h *Handler) scrollToTop(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}

	ctx.Renderer.ScrollTo(0, 0)

	// Move cursor to first line
	if ctx.Cursors != nil {
		newOffset := ctx.Engine.LineStartOffset(0)
		sel := ctx.Cursors.Primary().MoveTo(newOffset)
		ctx.Cursors.SetPrimary(sel)
	}

	return handler.Success().WithRedraw()
}

// scrollToBottom scrolls to the bottom of the buffer.
func (h *Handler) scrollToBottom(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}

	lineCount := ctx.Engine.LineCount()
	if lineCount == 0 {
		return handler.NoOp()
	}

	lastLine := lineCount - 1
	ctx.Renderer.ScrollTo(lastLine, 0)

	// Move cursor to last line
	if ctx.Cursors != nil {
		newOffset := ctx.Engine.LineStartOffset(lastLine)
		sel := ctx.Cursors.Primary().MoveTo(newOffset)
		ctx.Cursors.SetPrimary(sel)
	}

	return handler.Success().WithRedraw()
}

// moveToTop moves the cursor to the top visible line (H in Vim).
func (h *Handler) moveToTop(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	start, _ := ctx.Renderer.VisibleLineRange()
	newOffset := ctx.Engine.LineStartOffset(start)
	sel := ctx.Cursors.Primary().MoveTo(newOffset)
	ctx.Cursors.SetPrimary(sel)

	return handler.Success().WithRedraw()
}

// moveToMiddle moves the cursor to the middle visible line (M in Vim).
func (h *Handler) moveToMiddle(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	start, end := ctx.Renderer.VisibleLineRange()
	middleLine := start + (end-start)/2

	// Clamp to buffer
	lineCount := ctx.Engine.LineCount()
	if lineCount > 0 && middleLine >= lineCount {
		middleLine = lineCount - 1
	}

	newOffset := ctx.Engine.LineStartOffset(middleLine)
	sel := ctx.Cursors.Primary().MoveTo(newOffset)
	ctx.Cursors.SetPrimary(sel)

	return handler.Success().WithRedraw()
}

// moveToBottom moves the cursor to the bottom visible line (L in Vim).
func (h *Handler) moveToBottom(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	_, end := ctx.Renderer.VisibleLineRange()
	bottomLine := end
	if bottomLine > 0 {
		bottomLine-- // end is exclusive
	}

	// Clamp to buffer
	lineCount := ctx.Engine.LineCount()
	if lineCount > 0 && bottomLine >= lineCount {
		bottomLine = lineCount - 1
	}

	newOffset := ctx.Engine.LineStartOffset(bottomLine)
	sel := ctx.Cursors.Primary().MoveTo(newOffset)
	ctx.Cursors.SetPrimary(sel)

	return handler.Success().WithRedraw()
}

// centerCursor centers the view on the cursor line (zz in Vim).
func (h *Handler) centerCursor(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	cursorLine := ctx.Engine.OffsetToPoint(ctx.Cursors.Primary().Head).Line
	ctx.Renderer.CenterOnLine(cursorLine)

	return handler.Success().WithRedraw()
}

// topCursor scrolls so the cursor line is at the top (zt in Vim).
func (h *Handler) topCursor(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	cursorLine := ctx.Engine.OffsetToPoint(ctx.Cursors.Primary().Head).Line
	ctx.Renderer.ScrollTo(cursorLine, 0)

	return handler.Success().WithRedraw()
}

// bottomCursor scrolls so the cursor line is at the bottom (zb in Vim).
func (h *Handler) bottomCursor(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	cursorLine := ctx.Engine.OffsetToPoint(ctx.Cursors.Primary().Head).Line
	start, end := ctx.Renderer.VisibleLineRange()
	visibleHeight := end - start
	if visibleHeight < 1 {
		visibleHeight = 20
	}

	var newStart uint32
	if cursorLine >= visibleHeight-1 {
		newStart = cursorLine - visibleHeight + 1
	}

	ctx.Renderer.ScrollTo(newStart, 0)

	return handler.Success().WithRedraw()
}

// Helper to get cursor position
func getCursorLine(ctx *execctx.ExecutionContext) uint32 {
	if ctx.Cursors == nil {
		return 0
	}
	return ctx.Engine.OffsetToPoint(ctx.Cursors.Primary().Head).Line
}

// Helper to set cursor to line start
func setCursorToLine(ctx *execctx.ExecutionContext, line uint32) {
	if ctx.Cursors == nil {
		return
	}
	newOffset := ctx.Engine.LineStartOffset(line)
	sel := ctx.Cursors.Primary().MoveTo(newOffset)
	ctx.Cursors.SetPrimary(sel)
}

// GetVisibleLineCount returns the number of visible lines, or a default if unavailable.
func GetVisibleLineCount(ctx *execctx.ExecutionContext) uint32 {
	if ctx.Renderer == nil {
		return 20 // Default
	}
	start, end := ctx.Renderer.VisibleLineRange()
	if end > start {
		return end - start
	}
	return 20
}

// EnsureCursorVisible scrolls the view to make the cursor visible.
func EnsureCursorVisible(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Renderer == nil || ctx.Cursors == nil {
		return handler.NoOp()
	}

	cursorLine := getCursorLine(ctx)
	start, end := ctx.Renderer.VisibleLineRange()

	if cursorLine < start {
		ctx.Renderer.ScrollTo(cursorLine, 0)
		return handler.Success().WithRedraw()
	}

	if cursorLine >= end {
		// Calculate new start to put cursor at bottom
		visibleHeight := end - start
		if visibleHeight < 1 {
			visibleHeight = 20
		}
		var newStart uint32
		if cursorLine >= visibleHeight-1 {
			newStart = cursorLine - visibleHeight + 1
		}
		ctx.Renderer.ScrollTo(newStart, 0)
		return handler.Success().WithRedraw()
	}

	return handler.NoOp()
}

// ScrollToLine scrolls to show a specific line, centered if possible.
func ScrollToLine(ctx *execctx.ExecutionContext, line uint32, center bool) handler.Result {
	if ctx.Renderer == nil {
		return handler.Error(execctx.ErrMissingRenderer)
	}

	if center {
		ctx.Renderer.CenterOnLine(line)
	} else {
		ctx.Renderer.ScrollTo(line, 0)
	}

	return handler.Success().WithRedraw()
}

// ScrollToOffset scrolls to show a specific byte offset.
func ScrollToOffset(ctx *execctx.ExecutionContext, offset buffer.ByteOffset, center bool) handler.Result {
	if ctx.Engine == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}

	point := ctx.Engine.OffsetToPoint(offset)
	return ScrollToLine(ctx, point.Line, center)
}
