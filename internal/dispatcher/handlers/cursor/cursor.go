// Package cursor provides handlers for cursor movement operations.
package cursor

import (
	"unicode/utf8"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for cursor movements.
const (
	ActionMoveLeft      = "cursor.moveLeft"
	ActionMoveRight     = "cursor.moveRight"
	ActionMoveUp        = "cursor.moveUp"
	ActionMoveDown      = "cursor.moveDown"
	ActionMoveLineStart = "cursor.moveLineStart"
	ActionMoveLineEnd   = "cursor.moveLineEnd"
	ActionMoveFirstLine = "cursor.moveFirstLine"
	ActionMoveLastLine  = "cursor.moveLastLine"
)

// Handler implements namespace-based cursor movement handling.
type Handler struct{}

// NewHandler creates a new cursor handler.
func NewHandler() *Handler {
	return &Handler{}
}

// Namespace returns the cursor namespace.
func (h *Handler) Namespace() string {
	return "cursor"
}

// CanHandle returns true if this handler can process the action.
func (h *Handler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionMoveLeft, ActionMoveRight, ActionMoveUp, ActionMoveDown,
		ActionMoveLineStart, ActionMoveLineEnd, ActionMoveFirstLine, ActionMoveLastLine:
		return true
	}
	return false
}

// HandleAction processes a cursor action.
func (h *Handler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	// Validate context
	if ctx.Engine == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	count := ctx.GetCount()

	switch action.Name {
	case ActionMoveLeft:
		return h.moveLeft(ctx, count)
	case ActionMoveRight:
		return h.moveRight(ctx, count)
	case ActionMoveUp:
		return h.moveUp(ctx, count)
	case ActionMoveDown:
		return h.moveDown(ctx, count)
	case ActionMoveLineStart:
		return h.moveLineStart(ctx)
	case ActionMoveLineEnd:
		return h.moveLineEnd(ctx)
	case ActionMoveFirstLine:
		return h.moveFirstLine(ctx)
	case ActionMoveLastLine:
		return h.moveLastLine(ctx)
	default:
		return handler.Errorf("unknown cursor action: %s", action.Name)
	}
}

// moveLeft moves cursor left by count characters.
func (h *Handler) moveLeft(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	text := engine.Text()

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		newHead := sel.Head
		for i := 0; i < count && newHead > 0; i++ {
			// Find the previous rune boundary
			newHead = prevRuneStart(text, newHead)
		}

		// In normal mode, collapse selection; in visual mode, extend
		if ctx.HasSelection() {
			return sel.Extend(newHead)
		}
		return sel.MoveTo(newHead)
	})

	return handler.Success().WithRedraw()
}

// moveRight moves cursor right by count characters.
func (h *Handler) moveRight(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	text := engine.Text()
	maxOffset := engine.Len()

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		newHead := sel.Head
		for i := 0; i < count && newHead < maxOffset; i++ {
			// Skip to next rune
			_, size := utf8.DecodeRuneInString(text[newHead:])
			if size == 0 {
				break
			}
			newHead += buffer.ByteOffset(size)
		}

		// Clamp to buffer length
		if newHead > maxOffset {
			newHead = maxOffset
		}

		if ctx.HasSelection() {
			return sel.Extend(newHead)
		}
		return sel.MoveTo(newHead)
	})

	return handler.Success().WithRedraw()
}

// moveUp moves cursor up by count lines.
func (h *Handler) moveUp(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		point := engine.OffsetToPoint(sel.Head)

		// Calculate target line
		targetLine := uint32(0)
		if int(point.Line) > count {
			targetLine = point.Line - uint32(count)
		}

		// Preserve column, clamp to line length
		lineLen := engine.LineLen(targetLine)
		targetCol := point.Column
		if targetCol > lineLen {
			targetCol = lineLen
		}

		newOffset := engine.PointToOffset(buffer.Point{Line: targetLine, Column: targetCol})

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// moveDown moves cursor down by count lines.
func (h *Handler) moveDown(ctx *execctx.ExecutionContext, count int) handler.Result {
	engine := ctx.Engine
	lineCount := engine.LineCount()

	// Handle empty buffer
	if lineCount == 0 {
		return handler.NoOp()
	}

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		point := engine.OffsetToPoint(sel.Head)

		// Calculate target line
		targetLine := point.Line + uint32(count)
		if targetLine >= lineCount {
			targetLine = lineCount - 1
		}

		// Preserve column, clamp to line length
		lineLen := engine.LineLen(targetLine)
		targetCol := point.Column
		if targetCol > lineLen {
			targetCol = lineLen
		}

		newOffset := engine.PointToOffset(buffer.Point{Line: targetLine, Column: targetCol})

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// moveLineStart moves cursor to the start of the current line.
func (h *Handler) moveLineStart(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		point := engine.OffsetToPoint(sel.Head)
		newOffset := engine.LineStartOffset(point.Line)

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// moveLineEnd moves cursor to the end of the current line.
func (h *Handler) moveLineEnd(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		point := engine.OffsetToPoint(sel.Head)
		newOffset := engine.LineEndOffset(point.Line)

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// moveFirstLine moves cursor to the first line of the buffer.
func (h *Handler) moveFirstLine(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		newOffset := engine.LineStartOffset(0)

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// moveLastLine moves cursor to the last line of the buffer.
func (h *Handler) moveLastLine(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	lineCount := engine.LineCount()

	// Handle empty buffer
	if lineCount == 0 {
		return handler.NoOp()
	}

	lastLine := lineCount - 1

	ctx.Cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		newOffset := engine.LineStartOffset(lastLine)

		if ctx.HasSelection() {
			return sel.Extend(newOffset)
		}
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedraw()
}

// prevRuneStart finds the start of the previous rune before offset.
func prevRuneStart(text string, offset buffer.ByteOffset) buffer.ByteOffset {
	if offset <= 0 {
		return 0
	}

	textLen := buffer.ByteOffset(len(text))
	if offset > textLen {
		offset = textLen
	}

	// Move back at least one byte
	offset--

	// Continue moving back while we're in the middle of a multi-byte rune
	for offset > 0 && !utf8.RuneStart(text[offset]) {
		offset--
	}

	return offset
}
