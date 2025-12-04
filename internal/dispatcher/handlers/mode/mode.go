// Package mode provides handlers for mode switching operations.
package mode

import (
	"unicode"
	"unicode/utf8"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/cursor"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for mode operations.
const (
	ActionNormal          = "mode.normal"          // Escape - switch to normal mode
	ActionInsert          = "mode.insert"          // i - insert before cursor
	ActionInsertLineStart = "mode.insertLineStart" // I - insert at first non-blank
	ActionAppend          = "mode.append"          // a - append after cursor
	ActionAppendLineEnd   = "mode.appendLineEnd"   // A - append at end of line
	ActionOpenBelow       = "mode.openBelow"       // o - open line below
	ActionOpenAbove       = "mode.openAbove"       // O - open line above
	ActionVisual          = "mode.visual"          // v - visual character mode
	ActionVisualLine      = "mode.visualLine"      // V - visual line mode
	ActionVisualBlock     = "mode.visualBlock"     // Ctrl-V - visual block mode
	ActionCommand         = "mode.command"         // : - command line mode
	ActionReplace         = "mode.replace"         // R - replace mode
	ActionReplaceChar     = "mode.replaceChar"     // r - replace single character
)

// ModeHandler handles mode switching operations.
type ModeHandler struct{}

// NewModeHandler creates a new mode handler.
func NewModeHandler() *ModeHandler {
	return &ModeHandler{}
}

// Namespace returns the mode namespace.
func (h *ModeHandler) Namespace() string {
	return "mode"
}

// CanHandle returns true if this handler can process the action.
func (h *ModeHandler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionNormal, ActionInsert, ActionInsertLineStart,
		ActionAppend, ActionAppendLineEnd, ActionOpenBelow, ActionOpenAbove,
		ActionVisual, ActionVisualLine, ActionVisualBlock,
		ActionCommand, ActionReplace, ActionReplaceChar:
		return true
	}
	return false
}

// HandleAction processes a mode action.
func (h *ModeHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	switch action.Name {
	case ActionNormal:
		return h.switchToNormal(ctx)
	case ActionInsert:
		return h.switchToInsert(ctx)
	case ActionInsertLineStart:
		return h.insertLineStart(ctx)
	case ActionAppend:
		return h.append(ctx)
	case ActionAppendLineEnd:
		return h.appendLineEnd(ctx)
	case ActionOpenBelow:
		return h.openBelow(ctx)
	case ActionOpenAbove:
		return h.openAbove(ctx)
	case ActionVisual:
		return h.switchToVisual(ctx)
	case ActionVisualLine:
		return h.switchToVisualLine(ctx)
	case ActionVisualBlock:
		return h.switchToVisualBlock(ctx)
	case ActionCommand:
		return h.switchToCommand(ctx)
	case ActionReplace:
		return h.switchToReplace(ctx)
	case ActionReplaceChar:
		return h.replaceChar(ctx, action.Args.Text)
	default:
		return handler.Errorf("unknown mode action: %s", action.Name)
	}
}

// switchToNormal switches to normal mode.
func (h *ModeHandler) switchToNormal(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("normal"); err != nil {
			return handler.Error(err)
		}
	}

	// Collapse selection to cursor (Vim behavior)
	if ctx.Cursors != nil {
		selections := ctx.Cursors.All()
		for i, sel := range selections {
			if !sel.IsEmpty() {
				// Collapse to start of selection
				selections[i] = cursor.NewCursorSelection(sel.Range().Start)
			}
		}
		ctx.Cursors.SetAll(selections)
	}

	return handler.Success().WithModeChange("normal")
}

// switchToInsert switches to insert mode at current cursor position.
func (h *ModeHandler) switchToInsert(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("insert"); err != nil {
			return handler.Error(err)
		}
	}
	return handler.Success().WithModeChange("insert")
}

// insertLineStart moves to first non-blank character and enters insert mode.
func (h *ModeHandler) insertLineStart(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Engine == nil || ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}

	engine := ctx.Engine
	cursors := ctx.Cursors

	selections := cursors.All()
	for i, sel := range selections {
		point := engine.OffsetToPoint(sel.Head)
		lineStart := engine.LineStartOffset(point.Line)
		lineText := engine.LineText(point.Line)

		// Find first non-blank character
		offset := lineStart
		for j, r := range lineText {
			if !unicode.IsSpace(r) {
				offset = lineStart + buffer.ByteOffset(j)
				break
			}
		}

		selections[i] = cursor.NewCursorSelection(offset)
	}
	cursors.SetAll(selections)

	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("insert"); err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithModeChange("insert")
}

// append moves cursor right by one character and enters insert mode.
func (h *ModeHandler) append(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Engine == nil || ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}

	engine := ctx.Engine
	cursors := ctx.Cursors
	text := engine.Text()
	textLen := buffer.ByteOffset(len(text))

	selections := cursors.All()
	for i, sel := range selections {
		offset := sel.Head

		// Move right unless at end of buffer
		if offset < textLen {
			_, size := utf8.DecodeRuneInString(text[offset:])
			if size > 0 {
				offset += buffer.ByteOffset(size)
			}
		}

		selections[i] = cursor.NewCursorSelection(offset)
	}
	cursors.SetAll(selections)

	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("insert"); err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithModeChange("insert")
}

// appendLineEnd moves to end of line and enters insert mode.
func (h *ModeHandler) appendLineEnd(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Engine == nil || ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}

	engine := ctx.Engine
	cursors := ctx.Cursors

	selections := cursors.All()
	for i, sel := range selections {
		point := engine.OffsetToPoint(sel.Head)
		offset := engine.LineEndOffset(point.Line)
		selections[i] = cursor.NewCursorSelection(offset)
	}
	cursors.SetAll(selections)

	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("insert"); err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithModeChange("insert")
}

// openBelow inserts a new line below current line and enters insert mode.
func (h *ModeHandler) openBelow(ctx *execctx.ExecutionContext) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("openBelow")
		defer ctx.History.EndGroup()
	}

	// Process selections in reverse to maintain offsets
	selections := cursors.All()
	sortSelectionsReverse(selections)

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

	// Reverse to restore original order
	reverseSelections(newSelections)
	cursors.SetAll(newSelections)

	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("insert"); err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithRedraw().WithModeChange("insert")
}

// openAbove inserts a new line above current line and enters insert mode.
func (h *ModeHandler) openAbove(ctx *execctx.ExecutionContext) handler.Result {
	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	engine := ctx.Engine
	cursors := ctx.Cursors

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("openAbove")
		defer ctx.History.EndGroup()
	}

	// Process selections in reverse to maintain offsets
	selections := cursors.All()
	sortSelectionsReverse(selections)

	var newSelections []cursor.Selection
	var affectedLines []uint32

	for _, sel := range selections {
		point := engine.OffsetToPoint(sel.Head)
		lineStart := engine.LineStartOffset(point.Line)

		// Insert newline before current line
		_, err := engine.Insert(lineStart, "\n")
		if err != nil {
			return handler.Error(err)
		}

		// New cursor position is at start of new line (which is now at lineStart)
		newSelections = append(newSelections, cursor.NewCursorSelection(lineStart))

		affectedLines = append(affectedLines, point.Line, point.Line+1)
	}

	// Reverse to restore original order
	reverseSelections(newSelections)
	cursors.SetAll(newSelections)

	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("insert"); err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithRedraw().WithModeChange("insert")
}

// switchToVisual switches to visual character mode.
func (h *ModeHandler) switchToVisual(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	// Start selection at current cursor position
	selections := ctx.Cursors.All()
	for i, sel := range selections {
		// For visual mode, anchor = head initially (empty selection)
		selections[i] = cursor.NewSelection(sel.Head, sel.Head)
	}
	ctx.Cursors.SetAll(selections)

	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("visual"); err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithModeChange("visual")
}

// switchToVisualLine switches to visual line mode.
func (h *ModeHandler) switchToVisualLine(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Engine == nil || ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingEngine)
	}

	engine := ctx.Engine
	cursors := ctx.Cursors

	// Select entire current line for each cursor
	selections := cursors.All()
	for i, sel := range selections {
		point := engine.OffsetToPoint(sel.Head)
		lineStart := engine.LineStartOffset(point.Line)
		lineEnd := engine.LineEndOffset(point.Line)

		// Include newline if not at end of buffer
		lineCount := engine.LineCount()
		if point.Line+1 < lineCount {
			lineEnd++ // Include the newline character
		}

		selections[i] = cursor.NewSelection(lineStart, lineEnd)
	}
	cursors.SetAll(selections)

	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("visual-line"); err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithModeChange("visual-line")
}

// switchToVisualBlock switches to visual block mode.
func (h *ModeHandler) switchToVisualBlock(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.Cursors == nil {
		return handler.Error(execctx.ErrMissingCursors)
	}

	// Start block selection at current cursor position
	selections := ctx.Cursors.All()
	for i, sel := range selections {
		selections[i] = cursor.NewSelection(sel.Head, sel.Head)
	}
	ctx.Cursors.SetAll(selections)

	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("visual-block"); err != nil {
			return handler.Error(err)
		}
	}

	return handler.Success().WithModeChange("visual-block")
}

// switchToCommand switches to command-line mode.
func (h *ModeHandler) switchToCommand(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("command"); err != nil {
			return handler.Error(err)
		}
	}
	return handler.Success().WithModeChange("command")
}

// switchToReplace switches to replace mode.
func (h *ModeHandler) switchToReplace(ctx *execctx.ExecutionContext) handler.Result {
	if ctx.ModeManager != nil {
		if err := ctx.ModeManager.Switch("replace"); err != nil {
			return handler.Error(err)
		}
	}
	return handler.Success().WithModeChange("replace")
}

// replaceChar replaces the character under the cursor with the given character.
func (h *ModeHandler) replaceChar(ctx *execctx.ExecutionContext, char string) handler.Result {
	if char == "" {
		return handler.NoOp()
	}

	if err := ctx.ValidateForEdit(); err != nil {
		return handler.Error(err)
	}

	engine := ctx.Engine
	cursors := ctx.Cursors
	count := ctx.GetCount()

	if ctx.History != nil && cursors.Count() > 1 {
		ctx.History.BeginGroup("replaceChar")
		defer ctx.History.EndGroup()
	}

	text := engine.Text()
	textLen := buffer.ByteOffset(len(text))

	// Process selections in reverse to maintain offsets
	selections := cursors.All()
	sortSelectionsReverse(selections)

	var affectedLines []uint32

	for _, sel := range selections {
		offset := sel.Head

		// Replace count characters
		for i := 0; i < count && offset < textLen; i++ {
			// Get the size of the character to replace
			_, charSize := utf8.DecodeRuneInString(text[offset:])
			if charSize == 0 {
				break
			}

			// Don't replace newlines (Vim behavior)
			if text[offset] == '\n' {
				break
			}

			endOffset := offset + buffer.ByteOffset(charSize)

			// Replace the character
			_, err := engine.Replace(offset, endOffset, char)
			if err != nil {
				return handler.Error(err)
			}

			// Track affected line
			point := engine.OffsetToPoint(offset)
			affectedLines = append(affectedLines, point.Line)

			// Move to next character (use new char length)
			offset += buffer.ByteOffset(len(char))

			// Refresh text after edit
			text = engine.Text()
			textLen = buffer.ByteOffset(len(text))
		}
	}

	return handler.Success().WithRedrawLines(uniqueLines(affectedLines)...)
}

// sortSelectionsReverse sorts selections by position in descending order.
func sortSelectionsReverse(selections []cursor.Selection) {
	for i := 0; i < len(selections)-1; i++ {
		for j := i + 1; j < len(selections); j++ {
			if selections[i].Head < selections[j].Head {
				selections[i], selections[j] = selections[j], selections[i]
			}
		}
	}
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
