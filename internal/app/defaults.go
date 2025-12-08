package app

// DefaultEditorConfig returns the default editor configuration.
func DefaultEditorConfig() map[string]any {
	return map[string]any{
		// Tab and indentation
		"tabSize":        4,
		"insertSpaces":   true,
		"autoIndent":     true,
		"trimWhitespace": true,

		// Line endings
		"lineEnding": "lf",

		// Display
		"wordWrap":       false,
		"wordWrapColumn": 80,

		// Editing behavior
		"autoCloseBrackets": true,
		"autoCloseQuotes":   true,
		"autoSurround":      true,

		// Clipboard
		"useSystemClipboard": true,
	}
}

// DefaultUIConfig returns the default UI configuration.
func DefaultUIConfig() map[string]any {
	return map[string]any{
		// Theme
		"theme":      "default",
		"colorTheme": "dark",

		// Line numbers
		"lineNumbers":  true,
		"relativeLine": false,
		"signColumn":   true,
		"foldColumn":   false,

		// Whitespace
		"showWhitespace": false,
		"showIndent":     false,

		// Cursor
		"cursorLine":   true,
		"cursorColumn": false,
		"cursorBlink":  true,
		"cursorStyle":  "block", // block, line, underline

		// Status line
		"statusLine":       true,
		"statusLineHeight": 1,

		// Tab line
		"tabLine":       true,
		"tabLineHeight": 1,

		// Scrolling
		"scrolloff":       5,
		"sidescrolloff":   10,
		"smoothScrolling": false,

		// Minimap
		"minimap":      false,
		"minimapWidth": 80,

		// Font
		"fontSize":   14,
		"fontFamily": "monospace",
		"lineHeight": 1.4,
	}
}

// DefaultVimConfig returns the default Vim emulation configuration.
func DefaultVimConfig() map[string]any {
	return map[string]any{
		// Enable/disable vim mode
		"enable":        true,
		"startInNormal": true,

		// Search
		"smartCase":  true,
		"ignoreCase": true,
		"incsearch":  true,
		"hlsearch":   true,
		"wrapscan":   true,

		// Matching
		"showMatch": true,
		"matchTime": 5, // tenths of a second

		// Bells
		"visualBell": false,
		"errorbells": false,

		// History
		"history": 1000,

		// Undo
		"undoLevels": 1000,
		"undoFile":   true,

		// Clipboard
		"clipboard": "unnamed", // unnamed, unnamedplus, ""

		// Timeouts
		"timeout":     true,
		"timeoutLen":  1000, // ms for mapped sequences
		"ttimeout":    true,
		"ttimeoutLen": 50, // ms for key codes

		// Misc
		"hidden":    true, // allow hidden buffers
		"autoread":  true, // auto-reload changed files
		"backspace": "indent,eol,start",
	}
}

// DefaultInputConfig returns the default input configuration.
func DefaultInputConfig() map[string]any {
	return map[string]any{
		// Leader key
		"leaderKey":   " ", // Space as leader
		"localLeader": "\\",

		// Key sequence timeout
		"timeout": 1000, // ms

		// Mouse
		"mouseEnabled":     true,
		"mouseScrollLines": 3,

		// Repeat
		"repeatDelay": 500, // ms before repeat starts
		"repeatRate":  50,  // ms between repeats
	}
}

// DefaultFilesConfig returns the default file handling configuration.
func DefaultFilesConfig() map[string]any {
	return map[string]any{
		// Encoding
		"encoding":    "utf-8",
		"bomHandling": "remove",

		// Auto-save
		"autoSave":      false,
		"autoSaveDelay": 1000, // ms

		// Backup
		"backup":    false,
		"backupDir": "",

		// Swap
		"swapFile":   true,
		"swapDir":    "",
		"updateTime": 300, // ms

		// File watching
		"watchFiles":   true,
		"watchExclude": []string{".git", "node_modules", "vendor"},

		// Recent files
		"recentFilesMax": 100,

		// File associations
		"associations": map[string]string{},
	}
}

// DefaultSearchConfig returns the default search configuration.
func DefaultSearchConfig() map[string]any {
	return map[string]any{
		// Limits
		"maxResults":  1000,
		"maxFileSize": 10 * 1024 * 1024, // 10MB

		// Behavior
		"caseSensitive": false,
		"regex":         false,
		"wholeWord":     false,
		"preserveCase":  false,

		// Excludes
		"exclude": []string{
			".git",
			"node_modules",
			"vendor",
			"*.min.js",
			"*.min.css",
		},

		// Include
		"include": []string{},

		// Ripgrep integration
		"useRipgrep": true,
	}
}

// DefaultLSPConfig returns the default LSP configuration.
func DefaultLSPConfig() map[string]any {
	return map[string]any{
		// Enable/disable
		"enable": true,

		// Features
		"diagnostics":     true,
		"hover":           true,
		"completion":      true,
		"signatureHelp":   true,
		"definition":      true,
		"references":      true,
		"documentSymbol":  true,
		"workspaceSymbol": true,
		"codeAction":      true,
		"codeLens":        true,
		"formatting":      true,
		"rename":          true,
		"inlayHints":      false,
		"semanticTokens":  true,

		// Behavior
		"formatOnSave":       false,
		"organizeImports":    false,
		"autoComplete":       true,
		"completeUnimported": true,

		// Timeouts
		"initTimeout":    30000, // ms
		"requestTimeout": 10000, // ms

		// Diagnostics
		"diagnosticDelay": 500, // ms debounce

		// Server configs (per-language)
		"servers": map[string]any{},
	}
}

// DefaultTerminalConfig returns the default terminal configuration.
func DefaultTerminalConfig() map[string]any {
	return map[string]any{
		// Shell
		"shell":     "", // Use $SHELL or default
		"shellArgs": []string{},

		// Size
		"scrollback": 10000,

		// Rendering
		"fontSize":   14,
		"fontFamily": "monospace",
		"lineHeight": 1.2,

		// Cursor
		"cursorStyle": "block",
		"cursorBlink": true,

		// Bell
		"bellStyle": "none", // none, sound, visual

		// Integration
		"copyOnSelect":     true,
		"rightClickPastes": true,

		// Environment
		"env": map[string]string{},
	}
}

// DefaultGitConfig returns the default Git integration configuration.
func DefaultGitConfig() map[string]any {
	return map[string]any{
		// Enable/disable
		"enable": true,

		// Features
		"showGutter":      true,
		"showBlame":       false,
		"showBranchName":  true,
		"showAheadBehind": true,

		// Auto-fetch
		"autoFetch":         true,
		"autoFetchInterval": 300, // seconds

		// Decorations
		"decorateFilenames": true,

		// Diff
		"diffAlgorithm": "histogram", // myers, minimal, patience, histogram
	}
}

// DefaultLoggingConfig returns the default logging configuration.
func DefaultLoggingConfig() map[string]any {
	return map[string]any{
		// Log level
		"level": "info", // debug, info, warn, error

		// Output
		"file":       "", // empty = stderr
		"maxSize":    10, // MB
		"maxBackups": 3,
		"maxAge":     7, // days
		"compress":   true,

		// Format
		"format": "text", // text, json
	}
}

// DefaultPluginConfig returns the default plugin configuration.
func DefaultPluginConfig() map[string]any {
	return map[string]any{
		// Enable/disable
		"enable": true,

		// Directories
		"directories": []string{
			"~/.config/keystorm/plugins",
			"~/.local/share/keystorm/plugins",
		},

		// Security
		"allowNetwork":    false,
		"allowFileSystem": true,
		"allowSubprocess": false,
		"sandboxed":       true,

		// Timeouts
		"loadTimeout":    5000,  // ms
		"executeTimeout": 10000, // ms
	}
}

// DefaultConfig returns all default configurations combined.
func DefaultConfig() map[string]any {
	return map[string]any{
		"editor":   DefaultEditorConfig(),
		"ui":       DefaultUIConfig(),
		"vim":      DefaultVimConfig(),
		"input":    DefaultInputConfig(),
		"files":    DefaultFilesConfig(),
		"search":   DefaultSearchConfig(),
		"lsp":      DefaultLSPConfig(),
		"terminal": DefaultTerminalConfig(),
		"git":      DefaultGitConfig(),
		"logging":  DefaultLoggingConfig(),
		"plugins":  DefaultPluginConfig(),
	}
}

// DefaultNormalModeKeymaps returns the default keymaps for normal mode.
func DefaultNormalModeKeymaps() map[string]string {
	return map[string]string{
		// Movement
		"h":  "cursor.left",
		"j":  "cursor.down",
		"k":  "cursor.up",
		"l":  "cursor.right",
		"w":  "cursor.wordForward",
		"W":  "cursor.WORDForward",
		"b":  "cursor.wordBackward",
		"B":  "cursor.WORDBackward",
		"e":  "cursor.wordEnd",
		"E":  "cursor.WORDEnd",
		"0":  "cursor.lineStart",
		"$":  "cursor.lineEnd",
		"^":  "cursor.firstNonBlank",
		"gg": "cursor.documentStart",
		"G":  "cursor.documentEnd",
		"{":  "cursor.paragraphBackward",
		"}":  "cursor.paragraphForward",
		"%":  "cursor.matchingBracket",
		"f":  "cursor.findForward",
		"F":  "cursor.findBackward",
		"t":  "cursor.tillForward",
		"T":  "cursor.tillBackward",
		";":  "cursor.repeatFind",
		",":  "cursor.repeatFindReverse",

		// Mode changes
		"i":     "mode.insert",
		"I":     "mode.insertLineStart",
		"a":     "mode.append",
		"A":     "mode.appendLineEnd",
		"o":     "editor.insertLineBelow",
		"O":     "editor.insertLineAbove",
		"v":     "mode.visual",
		"V":     "mode.visualLine",
		"<C-v>": "mode.visualBlock",
		"R":     "mode.replace",

		// Editing
		"x":  "editor.deleteChar",
		"X":  "editor.deleteCharBefore",
		"dd": "editor.deleteLine",
		"D":  "editor.deleteToEnd",
		"cc": "editor.changeLine",
		"C":  "editor.changeToEnd",
		"s":  "editor.substitute",
		"S":  "editor.substituteLine",
		"r":  "editor.replaceChar",
		"J":  "editor.joinLines",
		"~":  "editor.toggleCase",
		".":  "editor.repeatLast",

		// Yank/paste
		"yy": "editor.yankLine",
		"Y":  "editor.yankLine",
		"p":  "editor.pasteAfter",
		"P":  "editor.pasteBefore",

		// Undo/redo
		"u":     "editor.undo",
		"<C-r>": "editor.redo",

		// Search
		"/": "search.forward",
		"?": "search.backward",
		"n": "search.next",
		"N": "search.previous",
		"*": "search.wordUnderCursor",
		"#": "search.wordUnderCursorBackward",

		// Command mode
		":": "mode.command",

		// Escape
		"<Esc>": "mode.normal",

		// Window management
		"<C-w>h": "window.focusLeft",
		"<C-w>j": "window.focusDown",
		"<C-w>k": "window.focusUp",
		"<C-w>l": "window.focusRight",
		"<C-w>s": "window.splitHorizontal",
		"<C-w>v": "window.splitVertical",
		"<C-w>c": "window.close",
		"<C-w>o": "window.closeOthers",
		"<C-w>=": "window.equalSize",
		"<C-w>_": "window.maximizeHeight",
		"<C-w>|": "window.maximizeWidth",

		// Buffer navigation
		"<C-^>": "buffer.alternate",
		"gd":    "lsp.definition",
		"gD":    "lsp.declaration",
		"gr":    "lsp.references",
		"gi":    "lsp.implementation",
		"K":     "lsp.hover",

		// Leader key shortcuts (space)
		"<leader>e": "file.explorer",
		"<leader>f": "file.find",
		"<leader>g": "search.grep",
		"<leader>b": "buffer.list",
		"<leader>w": "file.save",
		"<leader>q": "app.quit",
	}
}

// DefaultInsertModeKeymaps returns the default keymaps for insert mode.
func DefaultInsertModeKeymaps() map[string]string {
	return map[string]string{
		"<Esc>":      "mode.normal",
		"<C-[>":      "mode.normal",
		"<C-c>":      "mode.normal",
		"<BS>":       "editor.backspace",
		"<Del>":      "editor.delete",
		"<CR>":       "editor.newline",
		"<Tab>":      "editor.indent",
		"<S-Tab>":    "editor.unindent",
		"<C-w>":      "editor.deleteWord",
		"<C-u>":      "editor.deleteToLineStart",
		"<C-h>":      "editor.backspace",
		"<C-n>":      "completion.next",
		"<C-p>":      "completion.previous",
		"<C-y>":      "completion.accept",
		"<C-e>":      "completion.cancel",
		"<C-x><C-o>": "completion.trigger",
	}
}

// DefaultVisualModeKeymaps returns the default keymaps for visual mode.
func DefaultVisualModeKeymaps() map[string]string {
	return map[string]string{
		"<Esc>": "mode.normal",
		"<C-[>": "mode.normal",
		"d":     "editor.delete",
		"x":     "editor.delete",
		"y":     "editor.yank",
		"c":     "editor.change",
		"s":     "editor.change",
		">":     "editor.indentSelection",
		"<":     "editor.unindentSelection",
		"~":     "editor.toggleCaseSelection",
		"u":     "editor.lowercaseSelection",
		"U":     "editor.uppercaseSelection",
		"J":     "editor.joinSelection",
		"gq":    "editor.formatSelection",
		"r":     "editor.replaceSelection",
		"o":     "selection.swapAnchor",
		"v":     "mode.normal",
		"V":     "mode.visualLine",
		"<C-v>": "mode.visualBlock",
	}
}

// DefaultCommandModeKeymaps returns the default keymaps for command mode.
func DefaultCommandModeKeymaps() map[string]string {
	return map[string]string{
		"<Esc>":   "mode.normal",
		"<C-[>":   "mode.normal",
		"<C-c>":   "mode.normal",
		"<CR>":    "command.execute",
		"<Tab>":   "command.complete",
		"<S-Tab>": "command.completePrev",
		"<C-n>":   "command.historyNext",
		"<C-p>":   "command.historyPrev",
		"<Up>":    "command.historyPrev",
		"<Down>":  "command.historyNext",
		"<C-a>":   "command.cursorStart",
		"<C-e>":   "command.cursorEnd",
		"<C-b>":   "command.cursorLeft",
		"<C-f>":   "command.cursorRight",
		"<C-w>":   "command.deleteWord",
		"<C-u>":   "command.deleteLine",
		"<BS>":    "command.backspace",
		"<Del>":   "command.delete",
	}
}

// DefaultKeymaps returns all default keymaps.
func DefaultKeymaps() map[string]map[string]string {
	return map[string]map[string]string{
		"normal":  DefaultNormalModeKeymaps(),
		"insert":  DefaultInsertModeKeymaps(),
		"visual":  DefaultVisualModeKeymaps(),
		"command": DefaultCommandModeKeymaps(),
	}
}
