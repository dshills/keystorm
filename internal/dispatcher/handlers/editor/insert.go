// Package editor provides handlers for text editing operations.
package editor

import (
	"sort"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for insert operations.
const (
	ActionInsertChar      = "editor.insertChar"
	ActionInsertText      = "editor.insertText"
	ActionInsertNewline   = "editor.insertNewline"
	ActionInsertLineAbove = "editor.insertLineAbove"
	ActionInsertLineBelow = "editor.insertLineBelow"
	ActionInsertTab       = "editor.insertTab"
)

// InsertHandler handles text insertion operations.
type InsertHandler struct{}

// NewInsertHandler creates a new insert handler.
func NewInsertHandler() *InsertHandler {
	return &InsertHandler{}
}

// Namespace returns the editor namespace.
func (h *InsertHandler) Namespace() string {
	return "editor"
}

// CanHandle returns true if this handler can process the action.
func (h *InsertHandler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionInsertChar, ActionInsertText, ActionInsertNewline,
		ActionInsertLineAbove, ActionInsertLineBelow, ActionInsertTab:
		return true
	}
	return false
}

// HandleAction processes an insert action.
func (h *InsertHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	switch action.Name {
	case ActionInsertChar:
		return h.insertChar(ctx, action.Args.Text)
	case ActionInsertText:
		return h.insertText(ctx, action.Args.Text)
	case ActionInsertNewline:
		return h.insertNewline(ctx)
	case ActionInsertLineAbove:
		return h.insertLineAbove(ctx)
	case ActionInsertLineBelow:
		return h.insertLineBelow(ctx)
	case ActionInsertTab:
		return h.insertTab(ctx)
	default:
		return handler.Errorf("unknown insert action: %s", action.Name)
	}
}

// insertChar inserts a single character at all cursor positions.
func (h *InsertHandler) insertChar(ctx *execctx.ExecutionContext, text string) handler.Result {
	if text == "" {
		return handler.NoOp()
	}

	return h.insertText(ctx, text)
}

// insertText inserts text at all cursor positions.
func (h *InsertHandler) insertText(ctx *execctx.ExecutionContext, text string) handler.Result {
	if text == "" {
		return handler.NoOp()
	}

	engine := ctx.Engine
	cursors := ctx.Cursors

	// Begin undo group for multi-cursor edits
	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("insert")
		defer ctx.History.EndGroup()
	}

	// Get all cursor positions, sorted in reverse order to maintain offsets
	selections := cursors.All()
	sortSelectionsReverseInsert(selections)

	textLen := buffer.ByteOffset(len(text))
	var affectedLines []uint32

	for _, sel := range selections {
		// If there's a selection, delete it first
		insertOffset := sel.Head
		if !sel.IsEmpty() {
			r := sel.Range()
			_, err := engine.Delete(r.Start, r.End)
			if err != nil {
				return handler.Error(err)
			}
			insertOffset = r.Start
		}

		// Insert the text
		result, err := engine.Insert(insertOffset, text)
		if err != nil {
			return handler.Error(err)
		}

		// Track affected lines
		startPoint := engine.OffsetToPoint(result.NewRange.Start)
		endPoint := engine.OffsetToPoint(result.NewRange.End)
		for line := startPoint.Line; line <= endPoint.Line; line++ {
			affectedLines = append(affectedLines, line)
		}
	}

	// Update cursor positions - all cursors move right by text length
	cursors.MapInPlace(func(sel cursor.Selection) cursor.Selection {
		newOffset := sel.Head + textLen
		return sel.MoveTo(newOffset)
	})

	return handler.Success().WithRedrawLines(uniqueLines(affectedLines)...)
}

// insertNewline inserts a newline at all cursor positions.
func (h *InsertHandler) insertNewline(ctx *execctx.ExecutionContext) handler.Result {
	return h.insertText(ctx, "\n")
}

// insertLineAbove inserts a new line above the cursor and moves to it.
func (h *InsertHandler) insertLineAbove(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil {
		ctx.History.BeginGroup("insertLineAbove")
		defer ctx.History.EndGroup()
	}

	selections := cursors.All()
	sortSelectionsReverseInsert(selections)

	var affectedLines []uint32

	for i, sel := range selections {
		point := engine.OffsetToPoint(sel.Head)
		lineStart := engine.LineStartOffset(point.Line)

		// Insert newline at start of current line
		_, err := engine.Insert(lineStart, "\n")
		if err != nil {
			return handler.Error(err)
		}

		// Update this selection to point to the new line
		selections[i] = sel.MoveTo(lineStart)

		affectedLines = append(affectedLines, point.Line, point.Line+1)
	}

	// Set updated selections
	cursors.SetAll(selections)

	return handler.Success().WithRedraw().WithModeChange("insert")
}

// insertLineBelow inserts a new line below the cursor and moves to it.
func (h *InsertHandler) insertLineBelow(ctx *execctx.ExecutionContext) handler.Result {
	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil {
		ctx.History.BeginGroup("insertLineBelow")
		defer ctx.History.EndGroup()
	}

	selections := cursors.All()
	sortSelectionsReverseInsert(selections)

	var newSelections []cursor.Selection
	var affectedLines []uint32

	for _, sel := range selections {
		point := engine.OffsetToPoint(sel.Head)
		lineEnd := engine.LineEndOffset(point.Line)

		// Insert newline at end of current line
		_, err := engine.Insert(lineEnd, "\n")
		if err != nil {
			return handler.Error(err)
		}

		// New cursor position is at start of new line
		newOffset := lineEnd + 1
		newSelections = append(newSelections, cursor.NewCursorSelection(newOffset))

		affectedLines = append(affectedLines, point.Line, point.Line+1)
	}

	// Reverse to maintain order
	reverseSelections(newSelections)
	cursors.SetAll(newSelections)

	return handler.Success().WithRedraw().WithModeChange("insert")
}

// insertTab inserts a tab or spaces at cursor positions.
func (h *InsertHandler) insertTab(ctx *execctx.ExecutionContext) handler.Result {
	// TODO: Check editor config for tab vs spaces preference
	// For now, insert a tab character
	return h.insertText(ctx, "\t")
}

// sortSelectionsReverseInsert sorts selections by position in descending order.
// This ensures edits don't affect subsequent cursor positions.
// Note: This is a local version; delete.go has the canonical sortSelectionsReverse.
func sortSelectionsReverseInsert(selections []cursor.Selection) {
	sort.Slice(selections, func(i, j int) bool {
		return selections[i].Head > selections[j].Head
	})
}

// reverseSelections reverses the order of selections.
func reverseSelections(selections []cursor.Selection) {
	for i, j := 0, len(selections)-1; i < j; i, j = i+1, j-1 {
		selections[i], selections[j] = selections[j], selections[i]
	}
}

// uniqueLines returns unique line numbers from a slice.
func uniqueLines(lines []uint32) []uint32 {
	if len(lines) == 0 {
		return nil
	}

	seen := make(map[uint32]bool)
	result := make([]uint32, 0, len(lines))

	for _, line := range lines {
		if !seen[line] {
			seen[line] = true
			result = append(result, line)
		}
	}

	return result
}
