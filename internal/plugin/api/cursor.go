package api

import (
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// CursorModule implements the ks.cursor API module.
type CursorModule struct {
	ctx *Context
}

// NewCursorModule creates a new cursor module.
func NewCursorModule(ctx *Context) *CursorModule {
	return &CursorModule{ctx: ctx}
}

// Name returns the module name.
func (m *CursorModule) Name() string {
	return "cursor"
}

// RequiredCapability returns the capability required for this module.
func (m *CursorModule) RequiredCapability() security.Capability {
	return security.CapabilityCursor
}

// Register registers the module into the Lua state.
func (m *CursorModule) Register(L *lua.LState) error {
	mod := L.NewTable()

	// Register all cursor functions
	L.SetField(mod, "get", L.NewFunction(m.get))
	L.SetField(mod, "get_all", L.NewFunction(m.getAll))
	L.SetField(mod, "set", L.NewFunction(m.set))
	L.SetField(mod, "add", L.NewFunction(m.add))
	L.SetField(mod, "clear", L.NewFunction(m.clear))
	L.SetField(mod, "selection", L.NewFunction(m.selection))
	L.SetField(mod, "set_selection", L.NewFunction(m.setSelection))
	L.SetField(mod, "count", L.NewFunction(m.count))
	L.SetField(mod, "line", L.NewFunction(m.line))
	L.SetField(mod, "column", L.NewFunction(m.column))
	L.SetField(mod, "move", L.NewFunction(m.move))
	L.SetField(mod, "move_to_line", L.NewFunction(m.moveToLine))

	L.SetGlobal("_ks_cursor", mod)
	return nil
}

// get() -> offset
// Returns the primary cursor offset.
func (m *CursorModule) get(L *lua.LState) int {
	if m.ctx.Cursor == nil {
		L.Push(lua.LNumber(0))
		return 1
	}

	L.Push(lua.LNumber(m.ctx.Cursor.Get()))
	return 1
}

// get_all() -> {offsets}
// Returns all cursor offsets (for multi-cursor).
func (m *CursorModule) getAll(L *lua.LState) int {
	if m.ctx.Cursor == nil {
		L.Push(L.NewTable())
		return 1
	}

	offsets := m.ctx.Cursor.GetAll()
	tbl := L.NewTable()
	for i, offset := range offsets {
		tbl.RawSetInt(i+1, lua.LNumber(offset))
	}

	L.Push(tbl)
	return 1
}

// set(offset) -> nil
// Sets the primary cursor position.
func (m *CursorModule) set(L *lua.LState) int {
	offset := L.CheckInt(1)

	if offset < 0 {
		L.ArgError(1, "offset must be non-negative")
		return 0
	}

	if m.ctx.Cursor == nil {
		L.RaiseError("set: no cursor available")
		return 0
	}

	if err := m.ctx.Cursor.Set(offset); err != nil {
		L.RaiseError("set: %v", err)
		return 0
	}

	return 0
}

// add(offset) -> nil
// Adds a secondary cursor at the given offset.
func (m *CursorModule) add(L *lua.LState) int {
	offset := L.CheckInt(1)

	if offset < 0 {
		L.ArgError(1, "offset must be non-negative")
		return 0
	}

	if m.ctx.Cursor == nil {
		L.RaiseError("add: no cursor available")
		return 0
	}

	if err := m.ctx.Cursor.Add(offset); err != nil {
		L.RaiseError("add: %v", err)
		return 0
	}

	return 0
}

// clear() -> nil
// Clears all secondary cursors.
func (m *CursorModule) clear(L *lua.LState) int {
	if m.ctx.Cursor == nil {
		return 0
	}

	m.ctx.Cursor.Clear()
	return 0
}

// selection() -> {start, end} or nil
// Returns the selection range, or nil if no selection.
func (m *CursorModule) selection(L *lua.LState) int {
	if m.ctx.Cursor == nil {
		L.Push(lua.LNil)
		return 1
	}

	start, end := m.ctx.Cursor.Selection()
	if start < 0 || end < 0 {
		L.Push(lua.LNil)
		return 1
	}

	tbl := L.NewTable()
	L.SetField(tbl, "start", lua.LNumber(start))
	L.SetField(tbl, "end", lua.LNumber(end))
	L.Push(tbl)
	return 1
}

// set_selection(start, end) -> nil
// Sets the selection range.
func (m *CursorModule) setSelection(L *lua.LState) int {
	start := L.CheckInt(1)
	end := L.CheckInt(2)

	if start < 0 {
		L.ArgError(1, "start must be non-negative")
		return 0
	}
	if end < 0 {
		L.ArgError(2, "end must be non-negative")
		return 0
	}

	if m.ctx.Cursor == nil {
		L.RaiseError("set_selection: no cursor available")
		return 0
	}

	if err := m.ctx.Cursor.SetSelection(start, end); err != nil {
		L.RaiseError("set_selection: %v", err)
		return 0
	}

	return 0
}

// count() -> number
// Returns the number of cursors.
func (m *CursorModule) count(L *lua.LState) int {
	if m.ctx.Cursor == nil {
		L.Push(lua.LNumber(0))
		return 1
	}

	L.Push(lua.LNumber(m.ctx.Cursor.Count()))
	return 1
}

// line() -> number
// Returns the current line number (1-indexed).
func (m *CursorModule) line(L *lua.LState) int {
	if m.ctx.Cursor == nil {
		L.Push(lua.LNumber(1))
		return 1
	}

	L.Push(lua.LNumber(m.ctx.Cursor.Line()))
	return 1
}

// column() -> number
// Returns the current column number (1-indexed).
func (m *CursorModule) column(L *lua.LState) int {
	if m.ctx.Cursor == nil {
		L.Push(lua.LNumber(1))
		return 1
	}

	L.Push(lua.LNumber(m.ctx.Cursor.Column()))
	return 1
}

// move(delta) -> nil
// Moves the cursor by the given delta.
func (m *CursorModule) move(L *lua.LState) int {
	delta := L.CheckInt(1)

	if m.ctx.Cursor == nil {
		L.RaiseError("move: no cursor available")
		return 0
	}

	// Calculate new position
	current := m.ctx.Cursor.Get()
	newPos := current + delta
	if newPos < 0 {
		newPos = 0
	}

	if err := m.ctx.Cursor.Set(newPos); err != nil {
		L.RaiseError("move: %v", err)
		return 0
	}

	return 0
}

// move_to_line(line, col?) -> nil
// Moves the cursor to a specific line and optional column.
func (m *CursorModule) moveToLine(L *lua.LState) int {
	lineNum := L.CheckInt(1)
	col := L.OptInt(2, 1) // Default to column 1

	if lineNum < 1 {
		L.ArgError(1, "line must be >= 1")
		return 0
	}
	if col < 1 {
		L.ArgError(2, "column must be >= 1")
		return 0
	}

	if m.ctx.Cursor == nil || m.ctx.Buffer == nil {
		L.RaiseError("move_to_line: no cursor/buffer available")
		return 0
	}

	// Calculate byte offset from line and column
	// This requires buffer context to convert line/col to byte offset
	offset := lineColToOffset(m.ctx.Buffer, lineNum, col)

	if err := m.ctx.Cursor.Set(offset); err != nil {
		L.RaiseError("move_to_line: %v", err)
		return 0
	}

	return 0
}

// lineColToOffset converts line and column (1-indexed) to byte offset.
// Columns are counted in Unicode codepoints (runes), not bytes.
func lineColToOffset(buf BufferProvider, line, col int) int {
	if buf == nil {
		return 0
	}

	text := buf.Text()
	if len(text) == 0 {
		return 0
	}

	// Normalize CRLF to LF for consistent line handling
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	// Clamp line to valid range (1-indexed)
	if line < 1 {
		line = 1
	}
	if line > len(lines) {
		return len(text)
	}

	// Calculate byte offset of the start of the target line
	// We need to account for the original text (may have CRLF)
	byteOffset := 0
	for i := 0; i < line-1; i++ {
		byteOffset += len(lines[i]) + 1 // +1 for newline
	}

	// Adjust for CRLF: we need to use original text offsets
	// Recalculate by scanning original text
	byteOffset = 0
	currentLine := 1
	for i := 0; i < len(text) && currentLine < line; i++ {
		if text[i] == '\n' {
			currentLine++
		}
		byteOffset++
	}

	// Now add column offset (col is 1-indexed, rune-based)
	targetLine := lines[line-1]
	runes := []rune(targetLine)

	// Clamp column to valid range
	colIndex := col - 1 // Convert to 0-indexed
	if colIndex < 0 {
		colIndex = 0
	}
	if colIndex > len(runes) {
		colIndex = len(runes)
	}

	// Calculate byte offset of the column within the line
	colByteOffset := len(string(runes[:colIndex]))

	return byteOffset + colByteOffset
}
