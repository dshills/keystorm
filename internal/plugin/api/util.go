package api

import (
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// UtilModule implements the ks.util API module.
// This provides utility functions for string and table manipulation.
type UtilModule struct{}

// NewUtilModule creates a new util module.
func NewUtilModule() *UtilModule {
	return &UtilModule{}
}

// Name returns the module name.
func (m *UtilModule) Name() string {
	return "util"
}

// RequiredCapability returns the capability required for this module.
// Utility functions require no special capability.
func (m *UtilModule) RequiredCapability() security.Capability {
	return "" // No special capability required
}

// Register registers the module into the Lua state.
func (m *UtilModule) Register(L *lua.LState) error {
	mod := L.NewTable()

	// String utilities
	L.SetField(mod, "split", L.NewFunction(m.split))
	L.SetField(mod, "trim", L.NewFunction(m.trim))
	L.SetField(mod, "trim_left", L.NewFunction(m.trimLeft))
	L.SetField(mod, "trim_right", L.NewFunction(m.trimRight))
	L.SetField(mod, "starts_with", L.NewFunction(m.startsWith))
	L.SetField(mod, "ends_with", L.NewFunction(m.endsWith))
	L.SetField(mod, "contains", L.NewFunction(m.contains))
	L.SetField(mod, "escape_pattern", L.NewFunction(m.escapePattern))
	L.SetField(mod, "lines", L.NewFunction(m.lines))
	L.SetField(mod, "join", L.NewFunction(m.join))

	// Table utilities
	L.SetField(mod, "keys", L.NewFunction(m.keys))
	L.SetField(mod, "values", L.NewFunction(m.values))
	L.SetField(mod, "merge", L.NewFunction(m.merge))
	L.SetField(mod, "is_empty", L.NewFunction(m.isEmpty))
	L.SetField(mod, "len", L.NewFunction(m.tableLen))

	L.SetGlobal("_ks_util", mod)
	return nil
}

// split(str, sep) -> {parts}
// Splits a string by separator.
func (m *UtilModule) split(L *lua.LState) int {
	str := L.CheckString(1)
	sep := L.CheckString(2)

	parts := strings.Split(str, sep)
	tbl := L.NewTable()
	for i, part := range parts {
		tbl.RawSetInt(i+1, lua.LString(part))
	}

	L.Push(tbl)
	return 1
}

// trim(str) -> string
// Trims whitespace from both ends of a string.
func (m *UtilModule) trim(L *lua.LState) int {
	str := L.CheckString(1)
	L.Push(lua.LString(strings.TrimSpace(str)))
	return 1
}

// trim_left(str) -> string
// Trims whitespace from the left side of a string.
func (m *UtilModule) trimLeft(L *lua.LState) int {
	str := L.CheckString(1)
	L.Push(lua.LString(strings.TrimLeft(str, " \t\n\r")))
	return 1
}

// trim_right(str) -> string
// Trims whitespace from the right side of a string.
func (m *UtilModule) trimRight(L *lua.LState) int {
	str := L.CheckString(1)
	L.Push(lua.LString(strings.TrimRight(str, " \t\n\r")))
	return 1
}

// starts_with(str, prefix) -> bool
// Checks if a string starts with a prefix.
func (m *UtilModule) startsWith(L *lua.LState) int {
	str := L.CheckString(1)
	prefix := L.CheckString(2)
	L.Push(lua.LBool(strings.HasPrefix(str, prefix)))
	return 1
}

// ends_with(str, suffix) -> bool
// Checks if a string ends with a suffix.
func (m *UtilModule) endsWith(L *lua.LState) int {
	str := L.CheckString(1)
	suffix := L.CheckString(2)
	L.Push(lua.LBool(strings.HasSuffix(str, suffix)))
	return 1
}

// contains(str, substr) -> bool
// Checks if a string contains a substring.
func (m *UtilModule) contains(L *lua.LState) int {
	str := L.CheckString(1)
	substr := L.CheckString(2)
	L.Push(lua.LBool(strings.Contains(str, substr)))
	return 1
}

// escape_pattern(str) -> string
// Escapes special characters for use in Lua patterns.
func (m *UtilModule) escapePattern(L *lua.LState) int {
	str := L.CheckString(1)

	// Lua pattern special characters: ^$()%.[]*+-?
	// IMPORTANT: % must be escaped first to avoid double-escaping
	escaped := strings.ReplaceAll(str, "%", "%%")

	otherSpecialChars := []string{"^", "$", "(", ")", ".", "[", "]", "*", "+", "-", "?"}
	for _, ch := range otherSpecialChars {
		escaped = strings.ReplaceAll(escaped, ch, "%"+ch)
	}

	L.Push(lua.LString(escaped))
	return 1
}

// lines(str) -> {lines}
// Splits a string into lines.
func (m *UtilModule) lines(L *lua.LState) int {
	str := L.CheckString(1)

	// Handle both \n and \r\n line endings
	normalized := strings.ReplaceAll(str, "\r\n", "\n")
	parts := strings.Split(normalized, "\n")

	tbl := L.NewTable()
	for i, part := range parts {
		tbl.RawSetInt(i+1, lua.LString(part))
	}

	L.Push(tbl)
	return 1
}

// join(tbl, sep) -> string
// Joins table elements with a separator.
func (m *UtilModule) join(L *lua.LState) int {
	tbl := L.CheckTable(1)
	sep := L.OptString(2, "")

	var parts []string
	tbl.ForEach(func(key, value lua.LValue) {
		if str, ok := value.(lua.LString); ok {
			parts = append(parts, string(str))
		} else {
			parts = append(parts, value.String())
		}
	})

	L.Push(lua.LString(strings.Join(parts, sep)))
	return 1
}

// keys(tbl) -> {keys}
// Returns the keys of a table.
func (m *UtilModule) keys(L *lua.LState) int {
	tbl := L.CheckTable(1)

	result := L.NewTable()
	i := 1
	tbl.ForEach(func(key, _ lua.LValue) {
		result.RawSetInt(i, key)
		i++
	})

	L.Push(result)
	return 1
}

// values(tbl) -> {values}
// Returns the values of a table.
func (m *UtilModule) values(L *lua.LState) int {
	tbl := L.CheckTable(1)

	result := L.NewTable()
	i := 1
	tbl.ForEach(func(_, value lua.LValue) {
		result.RawSetInt(i, value)
		i++
	})

	L.Push(result)
	return 1
}

// merge(tbl1, tbl2, ...) -> merged
// Merges multiple tables into one. Later values override earlier ones.
func (m *UtilModule) merge(L *lua.LState) int {
	result := L.NewTable()

	// Merge all arguments
	for i := 1; i <= L.GetTop(); i++ {
		tbl := L.CheckTable(i)
		tbl.ForEach(func(key, value lua.LValue) {
			L.RawSet(result, key, value)
		})
	}

	L.Push(result)
	return 1
}

// is_empty(tbl) -> bool
// Checks if a table is empty.
func (m *UtilModule) isEmpty(L *lua.LState) int {
	tbl := L.CheckTable(1)

	empty := true
	tbl.ForEach(func(_, _ lua.LValue) {
		empty = false
	})

	L.Push(lua.LBool(empty))
	return 1
}

// len(tbl) -> number
// Returns the length of a table (number of elements).
func (m *UtilModule) tableLen(L *lua.LState) int {
	tbl := L.CheckTable(1)

	count := 0
	tbl.ForEach(func(_, _ lua.LValue) {
		count++
	})

	L.Push(lua.LNumber(count))
	return 1
}
