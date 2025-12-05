package api

import (
	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// BufferModule implements the ks.buf API module.
type BufferModule struct {
	ctx *Context
}

// NewBufferModule creates a new buffer module.
func NewBufferModule(ctx *Context) *BufferModule {
	return &BufferModule{ctx: ctx}
}

// Name returns the module name.
func (m *BufferModule) Name() string {
	return "buf"
}

// RequiredCapability returns the capability required for this module.
func (m *BufferModule) RequiredCapability() security.Capability {
	return security.CapabilityBuffer
}

// Register registers the module into the Lua state.
func (m *BufferModule) Register(L *lua.LState) error {
	mod := L.NewTable()

	// Register all buffer functions
	L.SetField(mod, "text", L.NewFunction(m.text))
	L.SetField(mod, "text_range", L.NewFunction(m.textRange))
	L.SetField(mod, "line", L.NewFunction(m.line))
	L.SetField(mod, "line_count", L.NewFunction(m.lineCount))
	L.SetField(mod, "len", L.NewFunction(m.bufLen))
	L.SetField(mod, "insert", L.NewFunction(m.insert))
	L.SetField(mod, "delete", L.NewFunction(m.delete))
	L.SetField(mod, "replace", L.NewFunction(m.replace))
	L.SetField(mod, "undo", L.NewFunction(m.undo))
	L.SetField(mod, "redo", L.NewFunction(m.redo))
	L.SetField(mod, "path", L.NewFunction(m.path))
	L.SetField(mod, "modified", L.NewFunction(m.modified))

	L.SetGlobal("_ks_buf", mod)
	return nil
}

// text() -> string
// Returns the full buffer text.
func (m *BufferModule) text(L *lua.LState) int {
	if m.ctx.Buffer == nil {
		L.Push(lua.LString(""))
		return 1
	}

	L.Push(lua.LString(m.ctx.Buffer.Text()))
	return 1
}

// text_range(start, end) -> string
// Returns text in the given byte range.
func (m *BufferModule) textRange(L *lua.LState) int {
	start := L.CheckInt(1)
	end := L.CheckInt(2)

	if m.ctx.Buffer == nil {
		L.Push(lua.LString(""))
		return 1
	}

	text, err := m.ctx.Buffer.TextRange(start, end)
	if err != nil {
		L.RaiseError("text_range: %v", err)
		return 0
	}

	L.Push(lua.LString(text))
	return 1
}

// line(n) -> string
// Returns the text of a specific line (1-indexed).
func (m *BufferModule) line(L *lua.LState) int {
	lineNum := L.CheckInt(1)

	if m.ctx.Buffer == nil {
		L.Push(lua.LString(""))
		return 1
	}

	text, err := m.ctx.Buffer.Line(lineNum)
	if err != nil {
		L.RaiseError("line: %v", err)
		return 0
	}

	L.Push(lua.LString(text))
	return 1
}

// line_count() -> number
// Returns the total number of lines.
func (m *BufferModule) lineCount(L *lua.LState) int {
	if m.ctx.Buffer == nil {
		L.Push(lua.LNumber(0))
		return 1
	}

	L.Push(lua.LNumber(m.ctx.Buffer.LineCount()))
	return 1
}

// len() -> number
// Returns the buffer length in bytes.
func (m *BufferModule) bufLen(L *lua.LState) int {
	if m.ctx.Buffer == nil {
		L.Push(lua.LNumber(0))
		return 1
	}

	L.Push(lua.LNumber(m.ctx.Buffer.Len()))
	return 1
}

// insert(offset, text) -> end_offset
// Inserts text at the given byte offset.
func (m *BufferModule) insert(L *lua.LState) int {
	offset := L.CheckInt(1)
	text := L.CheckString(2)

	if offset < 0 {
		L.ArgError(1, "offset must be non-negative")
		return 0
	}

	if m.ctx.Buffer == nil {
		L.RaiseError("insert: no buffer available")
		return 0
	}

	endOffset, err := m.ctx.Buffer.Insert(offset, text)
	if err != nil {
		L.RaiseError("insert: %v", err)
		return 0
	}

	L.Push(lua.LNumber(endOffset))
	return 1
}

// delete(start, end) -> nil
// Deletes text in the given byte range.
func (m *BufferModule) delete(L *lua.LState) int {
	start := L.CheckInt(1)
	end := L.CheckInt(2)

	if start < 0 {
		L.ArgError(1, "start must be non-negative")
		return 0
	}
	if end < start {
		L.ArgError(2, "end must be >= start")
		return 0
	}

	if m.ctx.Buffer == nil {
		L.RaiseError("delete: no buffer available")
		return 0
	}

	if err := m.ctx.Buffer.Delete(start, end); err != nil {
		L.RaiseError("delete: %v", err)
		return 0
	}

	return 0
}

// replace(start, end, text) -> end_offset
// Replaces text in the given byte range.
func (m *BufferModule) replace(L *lua.LState) int {
	start := L.CheckInt(1)
	end := L.CheckInt(2)
	text := L.CheckString(3)

	if start < 0 {
		L.ArgError(1, "start must be non-negative")
		return 0
	}
	if end < start {
		L.ArgError(2, "end must be >= start")
		return 0
	}

	if m.ctx.Buffer == nil {
		L.RaiseError("replace: no buffer available")
		return 0
	}

	endOffset, err := m.ctx.Buffer.Replace(start, end, text)
	if err != nil {
		L.RaiseError("replace: %v", err)
		return 0
	}

	L.Push(lua.LNumber(endOffset))
	return 1
}

// undo() -> bool
// Undoes the last change.
func (m *BufferModule) undo(L *lua.LState) int {
	if m.ctx.Buffer == nil {
		L.Push(lua.LBool(false))
		return 1
	}

	L.Push(lua.LBool(m.ctx.Buffer.Undo()))
	return 1
}

// redo() -> bool
// Redoes the last undone change.
func (m *BufferModule) redo(L *lua.LState) int {
	if m.ctx.Buffer == nil {
		L.Push(lua.LBool(false))
		return 1
	}

	L.Push(lua.LBool(m.ctx.Buffer.Redo()))
	return 1
}

// path() -> string
// Returns the file path of the buffer.
func (m *BufferModule) path(L *lua.LState) int {
	if m.ctx.Buffer == nil {
		L.Push(lua.LString(""))
		return 1
	}

	L.Push(lua.LString(m.ctx.Buffer.Path()))
	return 1
}

// modified() -> bool
// Returns true if the buffer has unsaved changes.
func (m *BufferModule) modified(L *lua.LState) int {
	if m.ctx.Buffer == nil {
		L.Push(lua.LBool(false))
		return 1
	}

	L.Push(lua.LBool(m.ctx.Buffer.Modified()))
	return 1
}
