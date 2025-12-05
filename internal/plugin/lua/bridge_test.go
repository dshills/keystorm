package lua

import (
	"reflect"
	"testing"

	glua "github.com/yuin/gopher-lua"
)

func TestNewBridge(t *testing.T) {
	L := glua.NewState()
	defer L.Close()

	bridge := NewBridge(L)
	if bridge == nil {
		t.Error("NewBridge() returned nil")
	}
	if bridge.L != L {
		t.Error("NewBridge() has wrong LState")
	}
}

func TestBridgeToGoValue(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	tests := []struct {
		name     string
		input    glua.LValue
		expected interface{}
	}{
		{"nil", glua.LNil, nil},
		{"true", glua.LTrue, true},
		{"false", glua.LFalse, false},
		{"integer", glua.LNumber(42), int64(42)},
		{"float", glua.LNumber(3.14), 3.14},
		{"string", glua.LString("hello"), "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bridge.ToGoValue(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ToGoValue(%v) = %v (%T), want %v (%T)",
					tt.input, result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestBridgeToGoValueTable(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	// Test array table
	t.Run("array", func(t *testing.T) {
		tbl := L.NewTable()
		tbl.RawSetInt(1, glua.LString("a"))
		tbl.RawSetInt(2, glua.LString("b"))
		tbl.RawSetInt(3, glua.LString("c"))

		result := bridge.ToGoValue(tbl)
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("Expected []interface{}, got %T", result)
		}
		if len(arr) != 3 {
			t.Errorf("Array length = %d, want 3", len(arr))
		}
		if arr[0] != "a" || arr[1] != "b" || arr[2] != "c" {
			t.Errorf("Array = %v, want [a b c]", arr)
		}
	})

	// Test map table
	t.Run("map", func(t *testing.T) {
		tbl := L.NewTable()
		tbl.RawSetString("name", glua.LString("test"))
		tbl.RawSetString("count", glua.LNumber(42))

		result := bridge.ToGoValue(tbl)
		m, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected map[string]interface{}, got %T", result)
		}
		if m["name"] != "test" {
			t.Errorf("map[name] = %v, want 'test'", m["name"])
		}
		if m["count"] != int64(42) {
			t.Errorf("map[count] = %v, want 42", m["count"])
		}
	})
}

func TestBridgeToLuaValue(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	tests := []struct {
		name  string
		input interface{}
		check func(glua.LValue) bool
	}{
		{"nil", nil, func(v glua.LValue) bool { return v == glua.LNil }},
		{"true", true, func(v glua.LValue) bool { return v == glua.LTrue }},
		{"false", false, func(v glua.LValue) bool { return v == glua.LFalse }},
		{"int", 42, func(v glua.LValue) bool {
			n, ok := v.(glua.LNumber)
			return ok && float64(n) == 42
		}},
		{"int64", int64(42), func(v glua.LValue) bool {
			n, ok := v.(glua.LNumber)
			return ok && float64(n) == 42
		}},
		{"float64", 3.14, func(v glua.LValue) bool {
			n, ok := v.(glua.LNumber)
			return ok && float64(n) == 3.14
		}},
		{"string", "hello", func(v glua.LValue) bool {
			s, ok := v.(glua.LString)
			return ok && string(s) == "hello"
		}},
		{"bytes", []byte("world"), func(v glua.LValue) bool {
			s, ok := v.(glua.LString)
			return ok && string(s) == "world"
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bridge.ToLuaValue(tt.input)
			if !tt.check(result) {
				t.Errorf("ToLuaValue(%v) = %v (%T), check failed",
					tt.input, result, result)
			}
		})
	}
}

func TestBridgeToLuaValueSlice(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	// Test []interface{}
	t.Run("interface slice", func(t *testing.T) {
		input := []interface{}{"a", 1, true}
		result := bridge.ToLuaValue(input)

		tbl, ok := result.(*glua.LTable)
		if !ok {
			t.Fatalf("Expected *LTable, got %T", result)
		}

		if tbl.RawGetInt(1).(glua.LString) != "a" {
			t.Error("tbl[1] != 'a'")
		}
	})

	// Test []string
	t.Run("string slice", func(t *testing.T) {
		input := []string{"x", "y", "z"}
		result := bridge.ToLuaValue(input)

		tbl, ok := result.(*glua.LTable)
		if !ok {
			t.Fatalf("Expected *LTable, got %T", result)
		}

		if tbl.RawGetInt(1).(glua.LString) != "x" {
			t.Error("tbl[1] != 'x'")
		}
	})

	// Test []int
	t.Run("int slice", func(t *testing.T) {
		input := []int{1, 2, 3}
		result := bridge.ToLuaValue(input)

		tbl, ok := result.(*glua.LTable)
		if !ok {
			t.Fatalf("Expected *LTable, got %T", result)
		}

		if float64(tbl.RawGetInt(1).(glua.LNumber)) != 1 {
			t.Error("tbl[1] != 1")
		}
	})
}

func TestBridgeToLuaValueMap(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	// Test map[string]interface{}
	t.Run("interface map", func(t *testing.T) {
		input := map[string]interface{}{
			"name":  "test",
			"count": 42,
		}
		result := bridge.ToLuaValue(input)

		tbl, ok := result.(*glua.LTable)
		if !ok {
			t.Fatalf("Expected *LTable, got %T", result)
		}

		if string(tbl.RawGetString("name").(glua.LString)) != "test" {
			t.Error("tbl.name != 'test'")
		}
	})

	// Test map[string]string
	t.Run("string map", func(t *testing.T) {
		input := map[string]string{
			"key": "value",
		}
		result := bridge.ToLuaValue(input)

		tbl, ok := result.(*glua.LTable)
		if !ok {
			t.Fatalf("Expected *LTable, got %T", result)
		}

		if string(tbl.RawGetString("key").(glua.LString)) != "value" {
			t.Error("tbl.key != 'value'")
		}
	})
}

func TestBridgeToLuaValueStruct(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	type TestStruct struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	input := TestStruct{Name: "hello", Count: 42}
	result := bridge.ToLuaValue(input)

	tbl, ok := result.(*glua.LTable)
	if !ok {
		t.Fatalf("Expected *LTable, got %T", result)
	}

	nameVal := tbl.RawGetString("name")
	if string(nameVal.(glua.LString)) != "hello" {
		t.Errorf("tbl.name = %v, want 'hello'", nameVal)
	}

	countVal := tbl.RawGetString("count")
	if float64(countVal.(glua.LNumber)) != 42 {
		t.Errorf("tbl.count = %v, want 42", countVal)
	}
}

func TestBridgeGetTableField(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	tbl := L.NewTable()
	tbl.RawSetString("foo", glua.LString("bar"))

	result := bridge.GetTableField(tbl, "foo")
	if string(result.(glua.LString)) != "bar" {
		t.Errorf("GetTableField(foo) = %v, want 'bar'", result)
	}
}

func TestBridgeGetTableString(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	tbl := L.NewTable()
	tbl.RawSetString("name", glua.LString("test"))

	s, ok := bridge.GetTableString(tbl, "name")
	if !ok {
		t.Error("GetTableString() ok = false")
	}
	if s != "test" {
		t.Errorf("GetTableString(name) = %v, want 'test'", s)
	}

	// Non-existent key
	_, ok = bridge.GetTableString(tbl, "nonexistent")
	if ok {
		t.Error("GetTableString(nonexistent) ok = true, want false")
	}
}

func TestBridgeGetTableInt(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	tbl := L.NewTable()
	tbl.RawSetString("count", glua.LNumber(42))

	n, ok := bridge.GetTableInt(tbl, "count")
	if !ok {
		t.Error("GetTableInt() ok = false")
	}
	if n != 42 {
		t.Errorf("GetTableInt(count) = %d, want 42", n)
	}
}

func TestBridgeGetTableBool(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	tbl := L.NewTable()
	tbl.RawSetString("enabled", glua.LTrue)

	b, ok := bridge.GetTableBool(tbl, "enabled")
	if !ok {
		t.Error("GetTableBool() ok = false")
	}
	if !b {
		t.Error("GetTableBool(enabled) = false, want true")
	}
}

func TestBridgeCallFunc(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	glua.OpenBase(L)
	bridge := NewBridge(L)

	// Define a function
	err := L.DoString(`
		function add(a, b)
			return a + b
		end
	`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}

	fn := L.GetGlobal("add").(*glua.LFunction)
	results, err := bridge.CallFunc(fn, 2, 3)
	if err != nil {
		t.Errorf("CallFunc() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("CallFunc() returned %d results, want 1", len(results))
	}

	if results[0] != int64(5) {
		t.Errorf("add(2, 3) = %v, want 5", results[0])
	}
}

func TestBridgeWrapGoFunc(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	glua.OpenBase(L)
	bridge := NewBridge(L)

	// Create a wrapped Go function
	wrapped := bridge.WrapGoFunc(func(args []interface{}) (interface{}, error) {
		sum := 0.0
		for _, arg := range args {
			switch v := arg.(type) {
			case int64:
				sum += float64(v)
			case float64:
				sum += v
			}
		}
		return sum, nil
	})

	// Register it
	L.SetGlobal("sum", L.NewFunction(wrapped))

	// Call it from Lua
	err := L.DoString(`result = sum(1, 2, 3)`)
	if err != nil {
		t.Errorf("DoString() error = %v", err)
	}

	result := L.GetGlobal("result")
	if float64(result.(glua.LNumber)) != 6 {
		t.Errorf("sum(1, 2, 3) = %v, want 6", result)
	}
}

func TestBridgeNewTable(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	tbl := bridge.NewTable()
	if tbl == nil {
		t.Error("NewTable() returned nil")
	}
}

func TestBridgeSetTableField(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	tbl := bridge.NewTable()
	bridge.SetTableField(tbl, "key", "value")

	result := tbl.RawGetString("key")
	if string(result.(glua.LString)) != "value" {
		t.Errorf("SetTableField() set wrong value: %v", result)
	}
}

func TestBridgeRoundTrip(t *testing.T) {
	L := glua.NewState()
	defer L.Close()
	bridge := NewBridge(L)

	// Test that converting Go -> Lua -> Go preserves values
	original := map[string]interface{}{
		"name":    "test",
		"count":   42,
		"enabled": true,
		"items":   []interface{}{"a", "b", "c"},
	}

	luaVal := bridge.ToLuaValue(original)
	goVal := bridge.ToGoValue(luaVal)

	converted, ok := goVal.(map[string]interface{})
	if !ok {
		t.Fatalf("Round trip failed: got %T", goVal)
	}

	if converted["name"] != "test" {
		t.Errorf("name = %v, want 'test'", converted["name"])
	}
	// Numbers come back as int64 for integers
	if converted["count"] != int64(42) {
		t.Errorf("count = %v (%T), want 42", converted["count"], converted["count"])
	}
	if converted["enabled"] != true {
		t.Errorf("enabled = %v, want true", converted["enabled"])
	}
}
