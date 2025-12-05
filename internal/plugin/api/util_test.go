package api

import (
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

func setupUtilTest(t *testing.T) *lua.LState {
	t.Helper()

	mod := NewUtilModule()

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	return L
}

func TestUtilModuleName(t *testing.T) {
	mod := NewUtilModule()
	if mod.Name() != "util" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "util")
	}
}

func TestUtilModuleCapability(t *testing.T) {
	mod := NewUtilModule()
	// Util module requires no special capability
	if mod.RequiredCapability() != security.Capability("") {
		t.Errorf("RequiredCapability() = %q, want empty", mod.RequiredCapability())
	}
}

func TestUtilSplit(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		parts = _ks_util.split("a,b,c", ",")
		count = #parts
		first = parts[1]
		second = parts[2]
		third = parts[3]
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 3 {
		t.Errorf("split parts count = %v, want 3", count)
	}

	first := L.GetGlobal("first")
	if first.String() != "a" {
		t.Errorf("split[1] = %q, want 'a'", first.String())
	}

	second := L.GetGlobal("second")
	if second.String() != "b" {
		t.Errorf("split[2] = %q, want 'b'", second.String())
	}
}

func TestUtilTrim(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		result = _ks_util.trim("  hello world  ")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.String() != "hello world" {
		t.Errorf("trim() = %q, want 'hello world'", result.String())
	}
}

func TestUtilTrimLeft(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		result = _ks_util.trim_left("  hello  ")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.String() != "hello  " {
		t.Errorf("trim_left() = %q, want 'hello  '", result.String())
	}
}

func TestUtilTrimRight(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		result = _ks_util.trim_right("  hello  ")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.String() != "  hello" {
		t.Errorf("trim_right() = %q, want '  hello'", result.String())
	}
}

func TestUtilStartsWith(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		yes = _ks_util.starts_with("hello world", "hello")
		no = _ks_util.starts_with("hello world", "world")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	yes := L.GetGlobal("yes")
	if yes != lua.LTrue {
		t.Error("starts_with('hello world', 'hello') should be true")
	}

	no := L.GetGlobal("no")
	if no != lua.LFalse {
		t.Error("starts_with('hello world', 'world') should be false")
	}
}

func TestUtilEndsWith(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		yes = _ks_util.ends_with("hello world", "world")
		no = _ks_util.ends_with("hello world", "hello")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	yes := L.GetGlobal("yes")
	if yes != lua.LTrue {
		t.Error("ends_with('hello world', 'world') should be true")
	}

	no := L.GetGlobal("no")
	if no != lua.LFalse {
		t.Error("ends_with('hello world', 'hello') should be false")
	}
}

func TestUtilContains(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		yes = _ks_util.contains("hello world", "lo wo")
		no = _ks_util.contains("hello world", "foo")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	yes := L.GetGlobal("yes")
	if yes != lua.LTrue {
		t.Error("contains('hello world', 'lo wo') should be true")
	}

	no := L.GetGlobal("no")
	if no != lua.LFalse {
		t.Error("contains('hello world', 'foo') should be false")
	}
}

func TestUtilEscapePattern(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		result = _ks_util.escape_pattern("a.b*c?d")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	expected := "a%.b%*c%?d"
	if result.String() != expected {
		t.Errorf("escape_pattern() = %q, want %q", result.String(), expected)
	}
}

func TestUtilLines(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		lines = _ks_util.lines("line1\nline2\nline3")
		count = #lines
		first = lines[1]
		second = lines[2]
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 3 {
		t.Errorf("lines count = %v, want 3", count)
	}

	first := L.GetGlobal("first")
	if first.String() != "line1" {
		t.Errorf("lines[1] = %q, want 'line1'", first.String())
	}
}

func TestUtilLinesWithCRLF(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		lines = _ks_util.lines("line1\r\nline2\r\n")
		count = #lines
		first = lines[1]
		second = lines[2]
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 3 {
		t.Errorf("lines count = %v, want 3", count)
	}

	first := L.GetGlobal("first")
	if first.String() != "line1" {
		t.Errorf("lines[1] = %q, want 'line1'", first.String())
	}
}

func TestUtilJoin(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		result = _ks_util.join({"a", "b", "c"}, ",")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.String() != "a,b,c" {
		t.Errorf("join() = %q, want 'a,b,c'", result.String())
	}
}

func TestUtilJoinDefault(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		result = _ks_util.join({"a", "b", "c"})
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.String() != "abc" {
		t.Errorf("join() with default sep = %q, want 'abc'", result.String())
	}
}

func TestUtilKeys(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		tbl = {a = 1, b = 2, c = 3}
		keys = _ks_util.keys(tbl)
		count = #keys
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 3 {
		t.Errorf("keys count = %v, want 3", count)
	}
}

func TestUtilValues(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		tbl = {a = 1, b = 2, c = 3}
		values = _ks_util.values(tbl)
		count = #values
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 3 {
		t.Errorf("values count = %v, want 3", count)
	}
}

func TestUtilMerge(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		t1 = {a = 1, b = 2}
		t2 = {b = 3, c = 4}
		merged = _ks_util.merge(t1, t2)
		a_val = merged.a
		b_val = merged.b
		c_val = merged.c
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	aVal := L.GetGlobal("a_val")
	if aVal.(lua.LNumber) != 1 {
		t.Errorf("merged.a = %v, want 1", aVal)
	}

	bVal := L.GetGlobal("b_val")
	if bVal.(lua.LNumber) != 3 {
		t.Errorf("merged.b = %v, want 3 (overwritten)", bVal)
	}

	cVal := L.GetGlobal("c_val")
	if cVal.(lua.LNumber) != 4 {
		t.Errorf("merged.c = %v, want 4", cVal)
	}
}

func TestUtilIsEmpty(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		empty = _ks_util.is_empty({})
		not_empty = _ks_util.is_empty({a = 1})
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	empty := L.GetGlobal("empty")
	if empty != lua.LTrue {
		t.Error("is_empty({}) should be true")
	}

	notEmpty := L.GetGlobal("not_empty")
	if notEmpty != lua.LFalse {
		t.Error("is_empty({a = 1}) should be false")
	}
}

func TestUtilLen(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		len1 = _ks_util.len({})
		len2 = _ks_util.len({a = 1, b = 2})
		len3 = _ks_util.len({"x", "y", "z"})
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	len1 := L.GetGlobal("len1")
	if len1.(lua.LNumber) != 0 {
		t.Errorf("len({}) = %v, want 0", len1)
	}

	len2 := L.GetGlobal("len2")
	if len2.(lua.LNumber) != 2 {
		t.Errorf("len({a=1, b=2}) = %v, want 2", len2)
	}

	len3 := L.GetGlobal("len3")
	if len3.(lua.LNumber) != 3 {
		t.Errorf("len({'x', 'y', 'z'}) = %v, want 3", len3)
	}
}

func TestUtilSplitEmpty(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		parts = _ks_util.split("", ",")
		count = #parts
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 1 {
		t.Errorf("split('', ',') count = %v, want 1", count)
	}
}

func TestUtilEscapePatternAllSpecial(t *testing.T) {
	L := setupUtilTest(t)

	err := L.DoString(`
		result = _ks_util.escape_pattern("^$()%.[]*+-?")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	expected := "%^%$%(%)%%%.%[%]%*%+%-%?"
	if result.String() != expected {
		t.Errorf("escape_pattern() = %q, want %q", result.String(), expected)
	}
}
