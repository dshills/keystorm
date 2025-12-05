package lua

import (
	"fmt"
	"reflect"

	lua "github.com/yuin/gopher-lua"
)

// Bridge provides utilities for Go-Lua interoperability.
type Bridge struct {
	L *lua.LState
}

// NewBridge creates a new Bridge for the given Lua state.
func NewBridge(L *lua.LState) *Bridge {
	return &Bridge{L: L}
}

// ToGoValue converts a Lua value to a Go value.
func (b *Bridge) ToGoValue(lv lua.LValue) interface{} {
	return b.toGoValueWithVisited(lv, make(map[*lua.LTable]bool))
}

// toGoValueWithVisited converts a Lua value to a Go value, tracking visited tables.
func (b *Bridge) toGoValueWithVisited(lv lua.LValue, visited map[*lua.LTable]bool) interface{} {
	if lv == nil {
		return nil
	}

	switch v := lv.(type) {
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		// Check if it's an integer
		f := float64(v)
		if f == float64(int64(f)) {
			return int64(f)
		}
		return f
	case lua.LString:
		return string(v)
	case *lua.LTable:
		// Check for circular reference
		if visited[v] {
			return nil // Break circular reference
		}
		visited[v] = true
		return b.tableToGoWithVisited(v, visited)
	case *lua.LNilType:
		return nil
	case *lua.LFunction:
		// Functions can't be directly converted
		return nil
	case *lua.LUserData:
		return v.Value
	default:
		return nil
	}
}

// tableToGo converts a Lua table to either a Go map or slice.
func (b *Bridge) tableToGo(t *lua.LTable) interface{} {
	return b.tableToGoWithVisited(t, make(map[*lua.LTable]bool))
}

// tableToGoWithVisited converts a Lua table with circular reference tracking.
func (b *Bridge) tableToGoWithVisited(t *lua.LTable, visited map[*lua.LTable]bool) interface{} {
	// Check if it's an array (sequential integer keys starting at 1)
	isArray := true
	maxN := 0
	t.ForEach(func(k, _ lua.LValue) {
		if kn, ok := k.(lua.LNumber); ok {
			n := int(kn)
			if float64(n) == float64(kn) && n > 0 {
				if n > maxN {
					maxN = n
				}
				return
			}
		}
		isArray = false
	})

	// Verify it's a contiguous array
	if isArray && maxN > 0 {
		count := 0
		t.ForEach(func(_, _ lua.LValue) {
			count++
		})
		if count != maxN {
			isArray = false
		}
	}

	if isArray && maxN > 0 {
		// Convert to slice
		arr := make([]interface{}, maxN)
		for i := 1; i <= maxN; i++ {
			arr[i-1] = b.toGoValueWithVisited(t.RawGetInt(i), visited)
		}
		return arr
	}

	// Convert to map
	m := make(map[string]interface{})
	t.ForEach(func(k, v lua.LValue) {
		var key string
		switch kv := k.(type) {
		case lua.LString:
			key = string(kv)
		case lua.LNumber:
			key = fmt.Sprintf("%v", float64(kv))
		default:
			key = k.String()
		}
		m[key] = b.toGoValueWithVisited(v, visited)
	})
	return m
}

// ToLuaValue converts a Go value to a Lua value.
func (b *Bridge) ToLuaValue(v interface{}) lua.LValue {
	if v == nil {
		return lua.LNil
	}

	switch val := v.(type) {
	case bool:
		return lua.LBool(val)
	case int:
		return lua.LNumber(val)
	case int8:
		return lua.LNumber(val)
	case int16:
		return lua.LNumber(val)
	case int32:
		return lua.LNumber(val)
	case int64:
		return lua.LNumber(val)
	case uint:
		return lua.LNumber(val)
	case uint8:
		return lua.LNumber(val)
	case uint16:
		return lua.LNumber(val)
	case uint32:
		return lua.LNumber(val)
	case uint64:
		return lua.LNumber(val)
	case float32:
		return lua.LNumber(val)
	case float64:
		return lua.LNumber(val)
	case string:
		return lua.LString(val)
	case []byte:
		return lua.LString(val)
	case []interface{}:
		return b.sliceToTable(val)
	case []string:
		return b.stringSliceToTable(val)
	case []int:
		return b.intSliceToTable(val)
	case map[string]interface{}:
		return b.mapToTable(val)
	case map[string]string:
		return b.stringMapToTable(val)
	case lua.LValue:
		return val
	default:
		// Try reflection for other types
		return b.reflectToLua(v)
	}
}

// sliceToTable converts a Go slice to a Lua table (array).
func (b *Bridge) sliceToTable(s []interface{}) *lua.LTable {
	t := b.L.NewTable()
	for i, v := range s {
		t.RawSetInt(i+1, b.ToLuaValue(v))
	}
	return t
}

// stringSliceToTable converts a string slice to a Lua table.
func (b *Bridge) stringSliceToTable(s []string) *lua.LTable {
	t := b.L.NewTable()
	for i, v := range s {
		t.RawSetInt(i+1, lua.LString(v))
	}
	return t
}

// intSliceToTable converts an int slice to a Lua table.
func (b *Bridge) intSliceToTable(s []int) *lua.LTable {
	t := b.L.NewTable()
	for i, v := range s {
		t.RawSetInt(i+1, lua.LNumber(v))
	}
	return t
}

// mapToTable converts a Go map to a Lua table.
func (b *Bridge) mapToTable(m map[string]interface{}) *lua.LTable {
	t := b.L.NewTable()
	for k, v := range m {
		t.RawSetString(k, b.ToLuaValue(v))
	}
	return t
}

// stringMapToTable converts a string map to a Lua table.
func (b *Bridge) stringMapToTable(m map[string]string) *lua.LTable {
	t := b.L.NewTable()
	for k, v := range m {
		t.RawSetString(k, lua.LString(v))
	}
	return t
}

// reflectToLua uses reflection to convert arbitrary Go values.
func (b *Bridge) reflectToLua(v interface{}) lua.LValue {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return lua.LNil
	}

	switch rv.Kind() {
	case reflect.Ptr:
		if rv.IsNil() {
			return lua.LNil
		}
		return b.reflectToLua(rv.Elem().Interface())

	case reflect.Slice, reflect.Array:
		t := b.L.NewTable()
		for i := 0; i < rv.Len(); i++ {
			t.RawSetInt(i+1, b.ToLuaValue(rv.Index(i).Interface()))
		}
		return t

	case reflect.Map:
		t := b.L.NewTable()
		for _, key := range rv.MapKeys() {
			k := b.ToLuaValue(key.Interface())
			v := b.ToLuaValue(rv.MapIndex(key).Interface())
			t.RawSet(k, v)
		}
		return t

	case reflect.Struct:
		return b.structToTable(rv)

	default:
		// For unsupported types, return as userdata
		ud := b.L.NewUserData()
		ud.Value = v
		return ud
	}
}

// structToTable converts a Go struct to a Lua table.
func (b *Bridge) structToTable(rv reflect.Value) *lua.LTable {
	t := b.L.NewTable()
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue // Skip unexported fields
		}

		// Use json tag if available, otherwise field name
		name := field.Name
		if tag := field.Tag.Get("json"); tag != "" && tag != "-" {
			// Parse json tag (handle ",omitempty" etc.)
			for j := 0; j < len(tag); j++ {
				if tag[j] == ',' {
					tag = tag[:j]
					break
				}
			}
			if tag != "" {
				name = tag
			}
		}

		t.RawSetString(name, b.ToLuaValue(rv.Field(i).Interface()))
	}

	return t
}

// GetTableField gets a field from a Lua table.
func (b *Bridge) GetTableField(t *lua.LTable, key string) lua.LValue {
	return t.RawGetString(key)
}

// GetTableString gets a string field from a Lua table.
func (b *Bridge) GetTableString(t *lua.LTable, key string) (string, bool) {
	v := t.RawGetString(key)
	if s, ok := v.(lua.LString); ok {
		return string(s), true
	}
	return "", false
}

// GetTableInt gets an int field from a Lua table.
func (b *Bridge) GetTableInt(t *lua.LTable, key string) (int, bool) {
	v := t.RawGetString(key)
	if n, ok := v.(lua.LNumber); ok {
		return int(n), true
	}
	return 0, false
}

// GetTableBool gets a bool field from a Lua table.
func (b *Bridge) GetTableBool(t *lua.LTable, key string) (bool, bool) {
	v := t.RawGetString(key)
	if b, ok := v.(lua.LBool); ok {
		return bool(b), true
	}
	return false, false
}

// GetTableFunc gets a function field from a Lua table.
func (b *Bridge) GetTableFunc(t *lua.LTable, key string) (*lua.LFunction, bool) {
	v := t.RawGetString(key)
	if f, ok := v.(*lua.LFunction); ok {
		return f, true
	}
	return nil, false
}

// GetTableTable gets a table field from a Lua table.
func (b *Bridge) GetTableTable(t *lua.LTable, key string) (*lua.LTable, bool) {
	v := t.RawGetString(key)
	if t, ok := v.(*lua.LTable); ok {
		return t, true
	}
	return nil, false
}

// CallFunc calls a Lua function with Go arguments and returns Go values.
func (b *Bridge) CallFunc(fn *lua.LFunction, args ...interface{}) ([]interface{}, error) {
	// Record stack top before pushing anything
	stackTop := b.L.GetTop()

	// Push function
	b.L.Push(fn)

	// Push arguments
	for _, arg := range args {
		b.L.Push(b.ToLuaValue(arg))
	}

	// Call
	if err := b.L.PCall(len(args), lua.MultRet, nil); err != nil {
		return nil, err
	}

	// Collect results (only the new values added after the call)
	nRet := b.L.GetTop() - stackTop
	if nRet <= 0 {
		return nil, nil
	}
	results := make([]interface{}, nRet)
	for i := 0; i < nRet; i++ {
		results[i] = b.ToGoValue(b.L.Get(stackTop + i + 1))
	}
	b.L.Pop(nRet)

	return results, nil
}

// WrapGoFunc wraps a Go function for use in Lua.
// The Go function should take and return interface{} values.
func (b *Bridge) WrapGoFunc(fn func(args []interface{}) (interface{}, error)) lua.LGFunction {
	return func(L *lua.LState) int {
		// Collect arguments
		nArgs := L.GetTop()
		args := make([]interface{}, nArgs)
		for i := 1; i <= nArgs; i++ {
			args[i-1] = b.ToGoValue(L.Get(i))
		}

		// Call Go function
		result, err := fn(args)
		if err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}

		// Return result
		if result == nil {
			return 0
		}
		L.Push(b.ToLuaValue(result))
		return 1
	}
}

// NewTable creates a new empty Lua table.
func (b *Bridge) NewTable() *lua.LTable {
	return b.L.NewTable()
}

// SetTableField sets a field in a Lua table.
func (b *Bridge) SetTableField(t *lua.LTable, key string, value interface{}) {
	t.RawSetString(key, b.ToLuaValue(value))
}
