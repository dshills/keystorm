package keymap

import "github.com/dshills/keystorm/internal/input/mode"

// LoadDefaults loads all default keymaps into the registry.
func LoadDefaults(r *Registry) error {
	keymaps := []*Keymap{
		DefaultNormalKeymap(),
		DefaultInsertKeymap(),
		DefaultVisualKeymap(),
		DefaultCommandKeymap(),
		DefaultGlobalKeymap(),
	}

	for _, km := range keymaps {
		if err := r.Register(km); err != nil {
			return err
		}
	}

	return nil
}

// DefaultNormalKeymap returns default normal mode bindings.
func DefaultNormalKeymap() *Keymap {
	return &Keymap{
		Name:   "default-normal",
		Mode:   mode.ModeNormal,
		Source: "default",
		Bindings: []Binding{
			// Movement - basic
			{Keys: "h", Action: "cursor.moveLeft", Description: "Move left", Category: "Movement"},
			{Keys: "j", Action: "cursor.moveDown", Description: "Move down", Category: "Movement"},
			{Keys: "k", Action: "cursor.moveUp", Description: "Move up", Category: "Movement"},
			{Keys: "l", Action: "cursor.moveRight", Description: "Move right", Category: "Movement"},

			// Movement - words
			{Keys: "w", Action: "cursor.wordForward", Description: "Move to next word", Category: "Movement"},
			{Keys: "W", Action: "cursor.bigWordForward", Description: "Move to next WORD", Category: "Movement"},
			{Keys: "b", Action: "cursor.wordBackward", Description: "Move to previous word", Category: "Movement"},
			{Keys: "B", Action: "cursor.bigWordBackward", Description: "Move to previous WORD", Category: "Movement"},
			{Keys: "e", Action: "cursor.wordEndForward", Description: "Move to end of word", Category: "Movement"},
			{Keys: "E", Action: "cursor.bigWordEndForward", Description: "Move to end of WORD", Category: "Movement"},
			{Keys: "g e", Action: "cursor.wordEndBackward", Description: "Move to end of previous word", Category: "Movement"},

			// Movement - line
			{Keys: "0", Action: "cursor.moveLineStart", Description: "Move to line start", Category: "Movement"},
			{Keys: "$", Action: "cursor.moveLineEnd", Description: "Move to line end", Category: "Movement"},
			{Keys: "^", Action: "cursor.firstNonBlank", Description: "Move to first non-blank", Category: "Movement"},
			{Keys: "g _", Action: "cursor.firstNonBlank", Description: "Move to last non-blank", Category: "Movement"},
			{Keys: "|", Action: "cursor.gotoColumn", Description: "Move to column", Category: "Movement"},

			// Movement - document
			{Keys: "g g", Action: "cursor.moveFirstLine", Description: "Go to document start", Category: "Movement"},
			{Keys: "G", Action: "cursor.moveLastLine", Description: "Go to document end", Category: "Movement"},
			{Keys: "H", Action: "cursor.screenTop", Description: "Move to top of screen", Category: "Movement"},
			{Keys: "M", Action: "cursor.screenMiddle", Description: "Move to middle of screen", Category: "Movement"},
			{Keys: "L", Action: "cursor.screenBottom", Description: "Move to bottom of screen", Category: "Movement"},

			// Movement - search
			{Keys: "f", Action: "cursor.findForward", Description: "Find char forward", Category: "Movement"},
			{Keys: "F", Action: "cursor.findBackward", Description: "Find char backward", Category: "Movement"},
			{Keys: "t", Action: "cursor.tillForward", Description: "Till char forward", Category: "Movement"},
			{Keys: "T", Action: "cursor.tillBackward", Description: "Till char backward", Category: "Movement"},
			{Keys: ";", Action: "cursor.repeatFind", Description: "Repeat last f/t/F/T", Category: "Movement"},
			{Keys: ",", Action: "cursor.repeatFindReverse", Description: "Repeat last f/t/F/T reverse", Category: "Movement"},

			// Movement - matching
			{Keys: "%", Action: "cursor.matchingBracket", Description: "Go to matching bracket", Category: "Movement"},

			// Scrolling
			{Keys: "C-d", Action: "view.halfPageDown", Description: "Scroll half page down", Category: "Scrolling"},
			{Keys: "C-u", Action: "view.halfPageUp", Description: "Scroll half page up", Category: "Scrolling"},
			{Keys: "C-f", Action: "view.pageDown", Description: "Scroll page down", Category: "Scrolling"},
			{Keys: "C-b", Action: "view.pageUp", Description: "Scroll page up", Category: "Scrolling"},
			{Keys: "C-e", Action: "view.scrollDown", Description: "Scroll line down", Category: "Scrolling"},
			{Keys: "C-y", Action: "view.scrollUp", Description: "Scroll line up", Category: "Scrolling"},
			{Keys: "z z", Action: "view.centerCursor", Description: "Center cursor on screen", Category: "Scrolling"},
			{Keys: "z t", Action: "view.topCursor", Description: "Cursor to top of screen", Category: "Scrolling"},
			{Keys: "z b", Action: "view.bottomCursor", Description: "Cursor to bottom of screen", Category: "Scrolling"},

			// Mode switching
			{Keys: "i", Action: "mode.insert", Description: "Enter insert mode", Category: "Mode"},
			{Keys: "I", Action: "mode.insertLineStart", Description: "Insert at line start", Category: "Mode"},
			{Keys: "a", Action: "mode.append", Description: "Append after cursor", Category: "Mode"},
			{Keys: "A", Action: "mode.appendLineEnd", Description: "Append at line end", Category: "Mode"},
			{Keys: "o", Action: "mode.openBelow", Description: "Open line below", Category: "Mode"},
			{Keys: "O", Action: "mode.openAbove", Description: "Open line above", Category: "Mode"},
			{Keys: "v", Action: "mode.visual", Description: "Enter visual mode", Category: "Mode"},
			{Keys: "V", Action: "mode.visualLine", Description: "Enter visual line mode", Category: "Mode"},
			{Keys: "C-v", Action: "mode.visualBlock", Description: "Enter visual block mode", Category: "Mode"},
			{Keys: ":", Action: "mode.command", Description: "Enter command mode", Category: "Mode"},
			{Keys: "R", Action: "mode.replaceMode", Description: "Enter replace mode", Category: "Mode"},

			// Operators
			{Keys: "d", Action: "operator.delete", Description: "Delete", Category: "Operators"},
			{Keys: "c", Action: "operator.change", Description: "Change", Category: "Operators"},
			{Keys: "y", Action: "operator.yank", Description: "Yank (copy)", Category: "Operators"},
			{Keys: ">", Action: "operator.indent", Description: "Indent", Category: "Operators"},
			{Keys: "<", Action: "operator.outdent", Description: "Outdent", Category: "Operators"},
			{Keys: "=", Action: "operator.format", Description: "Format", Category: "Operators"},
			{Keys: "g u", Action: "operator.lowercase", Description: "Lowercase", Category: "Operators"},
			{Keys: "g U", Action: "operator.uppercase", Description: "Uppercase", Category: "Operators"},
			{Keys: "g ~", Action: "operator.toggleCase", Description: "Toggle case", Category: "Operators"},
			{Keys: "g q", Action: "operator.formatText", Description: "Format text", Category: "Operators"},

			// Line operations (doubled operators)
			{Keys: "d d", Action: "editor.deleteLine", Description: "Delete line", Category: "Editing"},
			{Keys: "y y", Action: "editor.yankLine", Description: "Yank line", Category: "Editing"},
			{Keys: "c c", Action: "editor.changeLine", Description: "Change line", Category: "Editing"},
			{Keys: "> >", Action: "editor.indentLine", Description: "Indent line", Category: "Editing"},
			{Keys: "< <", Action: "editor.outdentLine", Description: "Outdent line", Category: "Editing"},
			{Keys: "= =", Action: "editor.formatLine", Description: "Format line", Category: "Editing"},
			{Keys: "g u u", Action: "editor.lowercaseLine", Description: "Lowercase line", Category: "Editing"},
			{Keys: "g U U", Action: "editor.uppercaseLine", Description: "Uppercase line", Category: "Editing"},

			// Quick edits
			{Keys: "x", Action: "editor.deleteChar", Description: "Delete character", Category: "Editing"},
			{Keys: "X", Action: "editor.deleteCharBefore", Description: "Delete character before", Category: "Editing"},
			{Keys: "r", Action: "editor.replaceChar", Description: "Replace character", Category: "Editing"},
			{Keys: "s", Action: "editor.substituteChar", Description: "Substitute character", Category: "Editing"},
			{Keys: "S", Action: "editor.substituteLine", Description: "Substitute line", Category: "Editing"},
			{Keys: "C", Action: "editor.changeToEnd", Description: "Change to end of line", Category: "Editing"},
			{Keys: "D", Action: "editor.deleteToEnd", Description: "Delete to end of line", Category: "Editing"},
			{Keys: "Y", Action: "editor.yankLine", Description: "Yank line", Category: "Editing"},
			{Keys: "J", Action: "editor.joinLines", Description: "Join lines", Category: "Editing"},
			{Keys: "g J", Action: "editor.joinLinesNoSpace", Description: "Join lines without space", Category: "Editing"},

			// Paste
			{Keys: "p", Action: "editor.pasteAfter", Description: "Paste after", Category: "Editing"},
			{Keys: "P", Action: "editor.pasteBefore", Description: "Paste before", Category: "Editing"},
			{Keys: "g p", Action: "editor.pasteAfterCursor", Description: "Paste after, cursor after", Category: "Editing"},
			{Keys: "g P", Action: "editor.pasteBeforeCursor", Description: "Paste before, cursor after", Category: "Editing"},

			// Undo/Redo
			{Keys: "u", Action: "editor.undo", Description: "Undo", Category: "History"},
			{Keys: "C-r", Action: "editor.redo", Description: "Redo", Category: "History"},
			{Keys: "U", Action: "editor.undoLine", Description: "Undo line changes", Category: "History"},
			{Keys: ".", Action: "editor.repeatLast", Description: "Repeat last change", Category: "History"},

			// Search
			{Keys: "/", Action: "search.forward", Description: "Search forward", Category: "Search"},
			{Keys: "?", Action: "search.backward", Description: "Search backward", Category: "Search"},
			{Keys: "n", Action: "search.next", Description: "Next search result", Category: "Search"},
			{Keys: "N", Action: "search.previous", Description: "Previous search result", Category: "Search"},
			{Keys: "*", Action: "search.wordUnderCursor", Description: "Search word under cursor", Category: "Search"},
			{Keys: "#", Action: "search.wordUnderCursorBackward", Description: "Search word backward", Category: "Search"},
			{Keys: "g *", Action: "search.partialWord", Description: "Search partial word", Category: "Search"},
			{Keys: "g #", Action: "search.partialWordBackward", Description: "Search partial word backward", Category: "Search"},

			// Marks
			{Keys: "m", Action: "mark.set", Description: "Set mark", Category: "Marks"},
			{Keys: "'", Action: "mark.gotoLine", Description: "Go to mark line", Category: "Marks"},
			{Keys: "`", Action: "mark.gotoExact", Description: "Go to mark exact", Category: "Marks"},
			{Keys: "' '", Action: "mark.gotoLastJump", Description: "Go to last jump", Category: "Marks"},
			{Keys: "` `", Action: "mark.gotoLastJumpExact", Description: "Go to last jump exact", Category: "Marks"},

			// Registers
			{Keys: "\"", Action: "register.select", Description: "Select register", Category: "Registers"},

			// Macros
			{Keys: "q", Action: "macro.toggleRecord", Description: "Toggle macro recording", Category: "Macros"},
			{Keys: "@", Action: "macro.play", Description: "Play macro", Category: "Macros"},
			{Keys: "@ @", Action: "macro.playLast", Description: "Play last macro", Category: "Macros"},

			// Folding
			{Keys: "z o", Action: "fold.open", Description: "Open fold", Category: "Folding"},
			{Keys: "z c", Action: "fold.close", Description: "Close fold", Category: "Folding"},
			{Keys: "z a", Action: "fold.toggle", Description: "Toggle fold", Category: "Folding"},
			{Keys: "z O", Action: "fold.openRecursive", Description: "Open folds recursively", Category: "Folding"},
			{Keys: "z C", Action: "fold.closeRecursive", Description: "Close folds recursively", Category: "Folding"},
			{Keys: "z M", Action: "fold.closeAll", Description: "Close all folds", Category: "Folding"},
			{Keys: "z R", Action: "fold.openAll", Description: "Open all folds", Category: "Folding"},

			// Go to definition
			{Keys: "g d", Action: "goto.definition", Description: "Go to definition", Category: "Navigation"},
			{Keys: "g D", Action: "goto.declaration", Description: "Go to declaration", Category: "Navigation"},
			{Keys: "g i", Action: "goto.implementation", Description: "Go to implementation", Category: "Navigation"},
			{Keys: "g r", Action: "goto.references", Description: "Go to references", Category: "Navigation"},
			{Keys: "K", Action: "hover.show", Description: "Show hover info", Category: "Navigation"},
		},
	}
}

// DefaultInsertKeymap returns default insert mode bindings.
func DefaultInsertKeymap() *Keymap {
	return &Keymap{
		Name:   "default-insert",
		Mode:   mode.ModeInsert,
		Source: "default",
		Bindings: []Binding{
			// Exit insert mode
			{Keys: "Esc", Action: "mode.normal", Description: "Return to normal mode", Category: "Mode"},
			{Keys: "C-c", Action: "mode.normal", Description: "Return to normal mode", Category: "Mode"},
			{Keys: "C-[", Action: "mode.normal", Description: "Return to normal mode", Category: "Mode"},

			// Insert mode editing
			{Keys: "C-h", Action: "editor.deleteCharBefore", Description: "Delete char before cursor", Category: "Editing"},
			{Keys: "C-w", Action: "editor.deleteWordBefore", Description: "Delete word before cursor", Category: "Editing"},
			{Keys: "C-u", Action: "editor.deleteToLineStart", Description: "Delete to line start", Category: "Editing"},
			{Keys: "C-t", Action: "editor.indentLine", Description: "Indent line", Category: "Editing"},
			{Keys: "C-d", Action: "editor.outdentLine", Description: "Outdent line", Category: "Editing"},

			// Completion
			{Keys: "C-n", Action: "completion.next", Description: "Next completion", Category: "Completion"},
			{Keys: "C-p", Action: "completion.previous", Description: "Previous completion", Category: "Completion"},
			{Keys: "<C-Space>", Action: "completion.trigger", Description: "Trigger completion", Category: "Completion"},
			{Keys: "<C-x><C-o>", Action: "completion.omni", Description: "Omni completion", Category: "Completion"},
			{Keys: "<C-x><C-f>", Action: "completion.file", Description: "File completion", Category: "Completion"},
			{Keys: "<C-x><C-l>", Action: "completion.line", Description: "Line completion", Category: "Completion"},

			// Special inserts
			{Keys: "C-r", Action: "insert.register", Description: "Insert from register", Category: "Insert"},
			{Keys: "C-a", Action: "insert.lastInserted", Description: "Insert last inserted text", Category: "Insert"},
			{Keys: "C-e", Action: "insert.charBelow", Description: "Insert char from below", Category: "Insert"},
			{Keys: "C-y", Action: "insert.charAbove", Description: "Insert char from above", Category: "Insert"},

			// Movement in insert mode
			{Keys: "C-o", Action: "mode.insertNormalOnce", Description: "Execute one normal command", Category: "Mode"},

			// Navigation (for convenience)
			{Keys: "Left", Action: "cursor.moveLeft", Description: "Move left", Category: "Navigation"},
			{Keys: "Right", Action: "cursor.moveRight", Description: "Move right", Category: "Navigation"},
			{Keys: "Up", Action: "cursor.moveUp", Description: "Move up", Category: "Navigation"},
			{Keys: "Down", Action: "cursor.moveDown", Description: "Move down", Category: "Navigation"},
			{Keys: "Home", Action: "cursor.moveLineStart", Description: "Move to line start", Category: "Navigation"},
			{Keys: "End", Action: "cursor.moveLineEnd", Description: "Move to line end", Category: "Navigation"},
		},
	}
}

// DefaultVisualKeymap returns default visual mode bindings.
func DefaultVisualKeymap() *Keymap {
	return &Keymap{
		Name:   "default-visual",
		Mode:   mode.ModeVisual,
		Source: "default",
		Bindings: []Binding{
			// Exit visual mode
			{Keys: "Esc", Action: "mode.normal", Description: "Return to normal mode", Category: "Mode"},
			{Keys: "C-c", Action: "mode.normal", Description: "Return to normal mode", Category: "Mode"},

			// Switch visual modes
			{Keys: "v", Action: "mode.visualToggle", Description: "Toggle visual mode", Category: "Mode"},
			{Keys: "V", Action: "mode.visualLine", Description: "Switch to visual line", Category: "Mode"},
			{Keys: "C-v", Action: "mode.visualBlock", Description: "Switch to visual block", Category: "Mode"},

			// Selection operations
			{Keys: "o", Action: "selection.swapAnchor", Description: "Swap selection anchor", Category: "Selection"},
			{Keys: "O", Action: "selection.swapCorner", Description: "Swap block corner", Category: "Selection"},
			{Keys: "g v", Action: "selection.reselect", Description: "Reselect last selection", Category: "Selection"},

			// Operators on selection
			{Keys: "d", Action: "editor.deleteSelection", Description: "Delete selection", Category: "Editing"},
			{Keys: "x", Action: "editor.deleteSelection", Description: "Delete selection", Category: "Editing"},
			{Keys: "c", Action: "editor.changeSelection", Description: "Change selection", Category: "Editing"},
			{Keys: "s", Action: "editor.changeSelection", Description: "Change selection", Category: "Editing"},
			{Keys: "y", Action: "editor.yankSelection", Description: "Yank selection", Category: "Editing"},
			{Keys: ">", Action: "editor.indentSelection", Description: "Indent selection", Category: "Editing"},
			{Keys: "<", Action: "editor.outdentSelection", Description: "Outdent selection", Category: "Editing"},
			{Keys: "=", Action: "editor.formatSelection", Description: "Format selection", Category: "Editing"},
			{Keys: "u", Action: "editor.lowercaseSelection", Description: "Lowercase selection", Category: "Editing"},
			{Keys: "U", Action: "editor.uppercaseSelection", Description: "Uppercase selection", Category: "Editing"},
			{Keys: "~", Action: "editor.toggleCaseSelection", Description: "Toggle case", Category: "Editing"},
			{Keys: "J", Action: "editor.joinSelection", Description: "Join selected lines", Category: "Editing"},
			{Keys: "g q", Action: "editor.formatTextSelection", Description: "Format text", Category: "Editing"},

			// Insert at selection
			{Keys: "I", Action: "mode.insertAtSelectionStart", Description: "Insert at selection start", Category: "Mode"},
			{Keys: "A", Action: "mode.insertAtSelectionEnd", Description: "Insert at selection end", Category: "Mode"},

			// Search selection
			{Keys: "*", Action: "search.selection", Description: "Search selection forward", Category: "Search"},
			{Keys: "#", Action: "search.selectionBackward", Description: "Search selection backward", Category: "Search"},

			// Replace
			{Keys: "r", Action: "editor.replaceSelection", Description: "Replace with character", Category: "Editing"},

			// Movement keys work the same as normal mode
			{Keys: "h", Action: "cursor.moveLeft", Description: "Extend left", Category: "Movement"},
			{Keys: "j", Action: "cursor.moveDown", Description: "Extend down", Category: "Movement"},
			{Keys: "k", Action: "cursor.moveUp", Description: "Extend up", Category: "Movement"},
			{Keys: "l", Action: "cursor.moveRight", Description: "Extend right", Category: "Movement"},
			{Keys: "w", Action: "cursor.wordForward", Description: "Extend to next word", Category: "Movement"},
			{Keys: "b", Action: "cursor.wordBackward", Description: "Extend to previous word", Category: "Movement"},
			{Keys: "e", Action: "cursor.wordEndForward", Description: "Extend to word end", Category: "Movement"},
			{Keys: "0", Action: "cursor.moveLineStart", Description: "Extend to line start", Category: "Movement"},
			{Keys: "$", Action: "cursor.moveLineEnd", Description: "Extend to line end", Category: "Movement"},
			{Keys: "^", Action: "cursor.firstNonBlank", Description: "Extend to first non-blank", Category: "Movement"},
			{Keys: "g g", Action: "cursor.moveFirstLine", Description: "Extend to document start", Category: "Movement"},
			{Keys: "G", Action: "cursor.moveLastLine", Description: "Extend to document end", Category: "Movement"},
			{Keys: "%", Action: "cursor.matchingBracket", Description: "Extend to matching bracket", Category: "Movement"},

			// Text objects
			{Keys: "i w", Action: "textobj.innerWord", Description: "Inner word", Category: "Text Objects"},
			{Keys: "a w", Action: "textobj.aWord", Description: "A word", Category: "Text Objects"},
			{Keys: "i W", Action: "textobj.innerWORD", Description: "Inner WORD", Category: "Text Objects"},
			{Keys: "a W", Action: "textobj.aWORD", Description: "A WORD", Category: "Text Objects"},
			{Keys: "i s", Action: "textobj.innerSentence", Description: "Inner sentence", Category: "Text Objects"},
			{Keys: "a s", Action: "textobj.aSentence", Description: "A sentence", Category: "Text Objects"},
			{Keys: "i p", Action: "textobj.innerParagraph", Description: "Inner paragraph", Category: "Text Objects"},
			{Keys: "a p", Action: "textobj.aParagraph", Description: "A paragraph", Category: "Text Objects"},
			{Keys: "i (", Action: "textobj.innerParen", Description: "Inner parentheses", Category: "Text Objects"},
			{Keys: "a (", Action: "textobj.aParen", Description: "A parentheses", Category: "Text Objects"},
			{Keys: "i )", Action: "textobj.innerParen", Description: "Inner parentheses", Category: "Text Objects"},
			{Keys: "a )", Action: "textobj.aParen", Description: "A parentheses", Category: "Text Objects"},
			{Keys: "i [", Action: "textobj.innerBracket", Description: "Inner brackets", Category: "Text Objects"},
			{Keys: "a [", Action: "textobj.aBracket", Description: "A brackets", Category: "Text Objects"},
			{Keys: "i ]", Action: "textobj.innerBracket", Description: "Inner brackets", Category: "Text Objects"},
			{Keys: "a ]", Action: "textobj.aBracket", Description: "A brackets", Category: "Text Objects"},
			{Keys: "i {", Action: "textobj.innerBrace", Description: "Inner braces", Category: "Text Objects"},
			{Keys: "a {", Action: "textobj.aBrace", Description: "A braces", Category: "Text Objects"},
			{Keys: "i }", Action: "textobj.innerBrace", Description: "Inner braces", Category: "Text Objects"},
			{Keys: "a }", Action: "textobj.aBrace", Description: "A braces", Category: "Text Objects"},
			{Keys: "i <", Action: "textobj.innerAngle", Description: "Inner angle brackets", Category: "Text Objects"},
			{Keys: "a <", Action: "textobj.aAngle", Description: "A angle brackets", Category: "Text Objects"},
			{Keys: "i >", Action: "textobj.innerAngle", Description: "Inner angle brackets", Category: "Text Objects"},
			{Keys: "a >", Action: "textobj.aAngle", Description: "A angle brackets", Category: "Text Objects"},
			{Keys: "i \"", Action: "textobj.innerDoubleQuote", Description: "Inner double quotes", Category: "Text Objects"},
			{Keys: "a \"", Action: "textobj.aDoubleQuote", Description: "A double quotes", Category: "Text Objects"},
			{Keys: "i '", Action: "textobj.innerSingleQuote", Description: "Inner single quotes", Category: "Text Objects"},
			{Keys: "a '", Action: "textobj.aSingleQuote", Description: "A single quotes", Category: "Text Objects"},
			{Keys: "i `", Action: "textobj.innerBacktick", Description: "Inner backticks", Category: "Text Objects"},
			{Keys: "a `", Action: "textobj.aBacktick", Description: "A backticks", Category: "Text Objects"},
			{Keys: "i t", Action: "textobj.innerTag", Description: "Inner tag", Category: "Text Objects"},
			{Keys: "a t", Action: "textobj.aTag", Description: "A tag", Category: "Text Objects"},
			{Keys: "i b", Action: "textobj.innerBlock", Description: "Inner block", Category: "Text Objects"},
			{Keys: "a b", Action: "textobj.aBlock", Description: "A block", Category: "Text Objects"},
			{Keys: "i B", Action: "textobj.innerBigBlock", Description: "Inner big block", Category: "Text Objects"},
			{Keys: "a B", Action: "textobj.aBigBlock", Description: "A big block", Category: "Text Objects"},
		},
	}
}

// DefaultCommandKeymap returns default command-line mode bindings.
func DefaultCommandKeymap() *Keymap {
	return &Keymap{
		Name:   "default-command",
		Mode:   mode.ModeCommand,
		Source: "default",
		Bindings: []Binding{
			// Exit
			{Keys: "Esc", Action: "mode.normal", Description: "Cancel and return to normal", Category: "Mode"},
			{Keys: "C-c", Action: "mode.normal", Description: "Cancel and return to normal", Category: "Mode"},
			{Keys: "C-[", Action: "mode.normal", Description: "Cancel and return to normal", Category: "Mode"},

			// Execute
			{Keys: "Enter", Action: "command.execute", Description: "Execute command", Category: "Command"},

			// Editing
			{Keys: "C-h", Action: "command.backspace", Description: "Delete char before cursor", Category: "Editing"},
			{Keys: "BS", Action: "command.backspace", Description: "Delete char before cursor", Category: "Editing"},
			{Keys: "C-w", Action: "command.deleteWord", Description: "Delete word before cursor", Category: "Editing"},
			{Keys: "C-u", Action: "command.clear", Description: "Clear command line", Category: "Editing"},

			// Navigation
			{Keys: "Left", Action: "command.left", Description: "Move left", Category: "Navigation"},
			{Keys: "Right", Action: "command.right", Description: "Move right", Category: "Navigation"},
			{Keys: "C-b", Action: "command.left", Description: "Move left", Category: "Navigation"},
			{Keys: "C-f", Action: "command.right", Description: "Move right", Category: "Navigation"},
			{Keys: "C-a", Action: "command.home", Description: "Move to start", Category: "Navigation"},
			{Keys: "C-e", Action: "command.end", Description: "Move to end", Category: "Navigation"},
			{Keys: "Home", Action: "command.home", Description: "Move to start", Category: "Navigation"},
			{Keys: "End", Action: "command.end", Description: "Move to end", Category: "Navigation"},

			// History
			{Keys: "Up", Action: "command.historyPrev", Description: "Previous history", Category: "History"},
			{Keys: "Down", Action: "command.historyNext", Description: "Next history", Category: "History"},
			{Keys: "C-p", Action: "command.historyPrev", Description: "Previous history", Category: "History"},
			{Keys: "C-n", Action: "command.historyNext", Description: "Next history", Category: "History"},

			// Completion
			{Keys: "Tab", Action: "command.complete", Description: "Complete command", Category: "Completion"},
			{Keys: "S-Tab", Action: "command.completePrev", Description: "Previous completion", Category: "Completion"},
			{Keys: "C-d", Action: "command.showCompletions", Description: "Show completions", Category: "Completion"},

			// Special
			{Keys: "C-r", Action: "command.insertRegister", Description: "Insert from register", Category: "Insert"},
		},
	}
}

// DefaultGlobalKeymap returns global bindings (all modes).
func DefaultGlobalKeymap() *Keymap {
	return &Keymap{
		Name:   "default-global",
		Mode:   "", // All modes
		Source: "default",
		Bindings: []Binding{
			// File operations
			{Keys: "C-s", Action: "file.save", Description: "Save file", Category: "File"},
			{Keys: "C-S-s", Action: "file.saveAs", Description: "Save file as", Category: "File"},
			{Keys: "C-o", Action: "file.open", Description: "Open file", When: "!modeInsert", Category: "File"},

			// Command palette
			{Keys: "C-S-p", Action: "palette.show", Description: "Show command palette", Category: "Palette"},
			{Keys: "C-p", Action: "picker.files", Description: "Show file picker", When: "!modeInsert", Category: "Palette"},
			{Keys: "C-S-o", Action: "picker.symbols", Description: "Show symbol picker", Category: "Palette"},
			{Keys: "C-g", Action: "picker.goto", Description: "Go to line", Category: "Palette"},

			// Window/buffer management
			{Keys: "<C-w>h", Action: "window.focusLeft", Description: "Focus left window", Category: "Window"},
			{Keys: "<C-w>j", Action: "window.focusDown", Description: "Focus down window", Category: "Window"},
			{Keys: "<C-w>k", Action: "window.focusUp", Description: "Focus up window", Category: "Window"},
			{Keys: "<C-w>l", Action: "window.focusRight", Description: "Focus right window", Category: "Window"},
			{Keys: "<C-w>v", Action: "window.splitVertical", Description: "Split vertical", Category: "Window"},
			{Keys: "<C-w>s", Action: "window.splitHorizontal", Description: "Split horizontal", Category: "Window"},
			{Keys: "<C-w>q", Action: "window.close", Description: "Close window", Category: "Window"},
			{Keys: "<C-w>o", Action: "window.closeOther", Description: "Close other windows", Category: "Window"},
			{Keys: "<C-w>=", Action: "window.equalize", Description: "Equal window sizes", Category: "Window"},
			{Keys: "<C-w>_", Action: "window.maximize", Description: "Maximize height", Category: "Window"},
			{Keys: "<C-w>|", Action: "window.maximizeWidth", Description: "Maximize width", Category: "Window"},
			{Keys: "<C-w>w", Action: "window.focusNext", Description: "Focus next window", Category: "Window"},
			{Keys: "<C-w>W", Action: "window.focusPrev", Description: "Focus previous window", Category: "Window"},

			// Tab management
			{Keys: "g t", Action: "tab.next", Description: "Next tab", Category: "Tabs"},
			{Keys: "g T", Action: "tab.prev", Description: "Previous tab", Category: "Tabs"},

			// Buffer management
			{Keys: "C-^", Action: "buffer.alternate", Description: "Switch to alternate buffer", Category: "Buffer"},

			// Quickfix
			{Keys: "] q", Action: "quickfix.next", Description: "Next quickfix item", Category: "Quickfix"},
			{Keys: "[ q", Action: "quickfix.prev", Description: "Previous quickfix item", Category: "Quickfix"},
			{Keys: "] Q", Action: "quickfix.last", Description: "Last quickfix item", Category: "Quickfix"},
			{Keys: "[ Q", Action: "quickfix.first", Description: "First quickfix item", Category: "Quickfix"},

			// Location list
			{Keys: "] l", Action: "loclist.next", Description: "Next location", Category: "Location List"},
			{Keys: "[ l", Action: "loclist.prev", Description: "Previous location", Category: "Location List"},

			// Diagnostics
			{Keys: "] d", Action: "diagnostic.next", Description: "Next diagnostic", Category: "Diagnostics"},
			{Keys: "[ d", Action: "diagnostic.prev", Description: "Previous diagnostic", Category: "Diagnostics"},
			{Keys: "C-.", Action: "diagnostic.showActions", Description: "Show code actions", Category: "Diagnostics"},

			// Git
			{Keys: "] c", Action: "git.nextChange", Description: "Next git change", Category: "Git"},
			{Keys: "[ c", Action: "git.prevChange", Description: "Previous git change", Category: "Git"},
		},
	}
}
