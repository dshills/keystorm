package api

import (
	"errors"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// mockCursorProvider implements CursorProvider for testing.
type mockCursorProvider struct {
	primary   int
	cursors   []int
	selStart  int
	selEnd    int
	lineNum   int
	columnNum int
	maxOffset int
}

func newMockCursorProvider() *mockCursorProvider {
	return &mockCursorProvider{
		primary:   0,
		cursors:   []int{0},
		selStart:  -1,
		selEnd:    -1,
		lineNum:   1,
		columnNum: 1,
		maxOffset: 100,
	}
}

func (m *mockCursorProvider) Get() int { return m.primary }
func (m *mockCursorProvider) GetAll() []int {
	result := make([]int, len(m.cursors))
	copy(result, m.cursors)
	return result
}
func (m *mockCursorProvider) Set(offset int) error {
	if offset < 0 || offset > m.maxOffset {
		return errors.New("offset out of range")
	}
	m.primary = offset
	m.cursors[0] = offset
	return nil
}
func (m *mockCursorProvider) Add(offset int) error {
	if offset < 0 || offset > m.maxOffset {
		return errors.New("offset out of range")
	}
	m.cursors = append(m.cursors, offset)
	return nil
}
func (m *mockCursorProvider) Clear() {
	m.cursors = []int{m.primary}
}
func (m *mockCursorProvider) Selection() (start, end int) {
	return m.selStart, m.selEnd
}
func (m *mockCursorProvider) SetSelection(start, end int) error {
	if start < 0 || end < 0 {
		return errors.New("invalid selection")
	}
	m.selStart = start
	m.selEnd = end
	return nil
}
func (m *mockCursorProvider) Count() int {
	return len(m.cursors)
}
func (m *mockCursorProvider) Line() int {
	return m.lineNum
}
func (m *mockCursorProvider) Column() int {
	return m.columnNum
}

func setupCursorTest(t *testing.T, cursor *mockCursorProvider) (*lua.LState, *CursorModule) {
	t.Helper()

	ctx := &Context{Cursor: cursor}
	mod := NewCursorModule(ctx)

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	return L, mod
}

func TestCursorModuleName(t *testing.T) {
	mod := NewCursorModule(&Context{})
	if mod.Name() != "cursor" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "cursor")
	}
}

func TestCursorModuleCapability(t *testing.T) {
	mod := NewCursorModule(&Context{})
	if mod.RequiredCapability() != security.CapabilityCursor {
		t.Errorf("RequiredCapability() = %q, want %q", mod.RequiredCapability(), security.CapabilityCursor)
	}
}

func TestCursorGet(t *testing.T) {
	cursor := newMockCursorProvider()
	cursor.primary = 42
	cursor.cursors = []int{42}
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		result = _ks_cursor.get()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.(lua.LNumber) != 42 {
		t.Errorf("get() = %v, want 42", result)
	}
}

func TestCursorGetAll(t *testing.T) {
	cursor := newMockCursorProvider()
	cursor.cursors = []int{10, 20, 30}
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		result = _ks_cursor.get_all()
		count = #result
		first = result[1]
		second = result[2]
		third = result[3]
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 3 {
		t.Errorf("get_all() length = %v, want 3", count)
	}

	first := L.GetGlobal("first")
	if first.(lua.LNumber) != 10 {
		t.Errorf("get_all()[1] = %v, want 10", first)
	}
}

func TestCursorSet(t *testing.T) {
	cursor := newMockCursorProvider()
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		_ks_cursor.set(50)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if cursor.primary != 50 {
		t.Errorf("cursor.primary = %d, want 50", cursor.primary)
	}
}

func TestCursorSetNegative(t *testing.T) {
	cursor := newMockCursorProvider()
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		_ks_cursor.set(-5)
	`)
	if err == nil {
		t.Error("set with negative offset should error")
	}
}

func TestCursorAdd(t *testing.T) {
	cursor := newMockCursorProvider()
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		_ks_cursor.add(25)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if len(cursor.cursors) != 2 {
		t.Errorf("cursor count = %d, want 2", len(cursor.cursors))
	}
	if cursor.cursors[1] != 25 {
		t.Errorf("added cursor = %d, want 25", cursor.cursors[1])
	}
}

func TestCursorClear(t *testing.T) {
	cursor := newMockCursorProvider()
	cursor.primary = 10
	cursor.cursors = []int{10, 20, 30}
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		_ks_cursor.clear()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if len(cursor.cursors) != 1 {
		t.Errorf("cursor count after clear = %d, want 1", len(cursor.cursors))
	}
}

func TestCursorSelection(t *testing.T) {
	cursor := newMockCursorProvider()
	cursor.selStart = 10
	cursor.selEnd = 20
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		sel = _ks_cursor.selection()
		sel_start = sel.start
		sel_end = sel["end"]
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	selStart := L.GetGlobal("sel_start")
	if selStart.(lua.LNumber) != 10 {
		t.Errorf("selection.start = %v, want 10", selStart)
	}

	selEnd := L.GetGlobal("sel_end")
	if selEnd.(lua.LNumber) != 20 {
		t.Errorf("selection.end = %v, want 20", selEnd)
	}
}

func TestCursorSelectionNone(t *testing.T) {
	cursor := newMockCursorProvider()
	// selStart and selEnd default to -1 (no selection)
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		sel = _ks_cursor.selection()
		is_nil = sel == nil
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	isNil := L.GetGlobal("is_nil")
	if isNil != lua.LTrue {
		t.Error("selection() should return nil when no selection")
	}
}

func TestCursorSetSelection(t *testing.T) {
	cursor := newMockCursorProvider()
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		_ks_cursor.set_selection(5, 15)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if cursor.selStart != 5 || cursor.selEnd != 15 {
		t.Errorf("selection = (%d, %d), want (5, 15)", cursor.selStart, cursor.selEnd)
	}
}

func TestCursorCount(t *testing.T) {
	cursor := newMockCursorProvider()
	cursor.cursors = []int{10, 20, 30}
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		result = _ks_cursor.count()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.(lua.LNumber) != 3 {
		t.Errorf("count() = %v, want 3", result)
	}
}

func TestCursorLine(t *testing.T) {
	cursor := newMockCursorProvider()
	cursor.lineNum = 5
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		result = _ks_cursor.line()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.(lua.LNumber) != 5 {
		t.Errorf("line() = %v, want 5", result)
	}
}

func TestCursorColumn(t *testing.T) {
	cursor := newMockCursorProvider()
	cursor.columnNum = 10
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		result = _ks_cursor.column()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.(lua.LNumber) != 10 {
		t.Errorf("column() = %v, want 10", result)
	}
}

func TestCursorMove(t *testing.T) {
	cursor := newMockCursorProvider()
	cursor.primary = 10
	cursor.cursors = []int{10}
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		_ks_cursor.move(5)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if cursor.primary != 15 {
		t.Errorf("cursor after move(5) = %d, want 15", cursor.primary)
	}
}

func TestCursorMoveNegative(t *testing.T) {
	cursor := newMockCursorProvider()
	cursor.primary = 10
	cursor.cursors = []int{10}
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		_ks_cursor.move(-5)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if cursor.primary != 5 {
		t.Errorf("cursor after move(-5) = %d, want 5", cursor.primary)
	}
}

func TestCursorMoveClampToZero(t *testing.T) {
	cursor := newMockCursorProvider()
	cursor.primary = 5
	cursor.cursors = []int{5}
	L, _ := setupCursorTest(t, cursor)

	err := L.DoString(`
		_ks_cursor.move(-100)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if cursor.primary != 0 {
		t.Errorf("cursor after move(-100) = %d, want 0 (clamped)", cursor.primary)
	}
}

func TestCursorNilContext(t *testing.T) {
	ctx := &Context{Cursor: nil}
	mod := NewCursorModule(ctx)

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// Should not panic, should return default values
	err := L.DoString(`
		assert(_ks_cursor.get() == 0, "get should be 0")
		assert(_ks_cursor.count() == 0, "count should be 0")
		assert(_ks_cursor.line() == 1, "line should be 1")
		assert(_ks_cursor.column() == 1, "column should be 1")
		assert(_ks_cursor.selection() == nil, "selection should be nil")
	`)
	if err != nil {
		t.Errorf("DoString with nil cursor error = %v", err)
	}
}

func TestLineColToOffset(t *testing.T) {
	buf := &mockBufferProvider{text: "line1\nline2\nline3"}

	tests := []struct {
		line, col int
		expected  int
	}{
		{1, 1, 0},   // Start of line 1
		{1, 3, 2},   // 3rd char of line 1
		{2, 1, 6},   // Start of line 2
		{2, 3, 8},   // 3rd char of line 2
		{3, 1, 12},  // Start of line 3
		{99, 1, 17}, // Past end should return buffer length
	}

	for _, tt := range tests {
		got := lineColToOffset(buf, tt.line, tt.col)
		if got != tt.expected {
			t.Errorf("lineColToOffset(%d, %d) = %d, want %d", tt.line, tt.col, got, tt.expected)
		}
	}
}

func TestLineColToOffsetUTF8(t *testing.T) {
	// Test with multi-byte UTF-8 characters
	// "日本語" is 3 characters, each 3 bytes = 9 bytes total
	buf := &mockBufferProvider{text: "日本語\nabc"}

	tests := []struct {
		name      string
		line, col int
		expected  int
	}{
		{"line 1, col 1 (first char)", 1, 1, 0},
		{"line 1, col 2 (second char)", 1, 2, 3},   // byte offset 3
		{"line 1, col 3 (third char)", 1, 3, 6},    // byte offset 6
		{"line 1, col 4 (past line end)", 1, 4, 9}, // byte offset 9 (end of 日本語)
		{"line 2, col 1", 2, 1, 10},                // after newline
		{"line 2, col 2", 2, 2, 11},                // 'b'
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lineColToOffset(buf, tt.line, tt.col)
			if got != tt.expected {
				t.Errorf("lineColToOffset(%d, %d) = %d, want %d", tt.line, tt.col, got, tt.expected)
			}
		})
	}
}

func TestLineColToOffsetEdgeCases(t *testing.T) {
	// Test edge cases
	tests := []struct {
		name      string
		text      string
		line, col int
		expected  int
	}{
		{"empty buffer", "", 1, 1, 0},
		{"line 0 clamped to 1", "abc", 0, 1, 0},
		{"col 0 clamped to 1", "abc", 1, 0, 0},
		{"negative line", "abc", -1, 1, 0},
		{"col past line end", "abc", 1, 10, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &mockBufferProvider{text: tt.text}
			got := lineColToOffset(buf, tt.line, tt.col)
			if got != tt.expected {
				t.Errorf("lineColToOffset(%d, %d) = %d, want %d", tt.line, tt.col, got, tt.expected)
			}
		})
	}
}
