package api

import (
	"errors"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// mockBufferProvider implements BufferProvider for testing.
type mockBufferProvider struct {
	text     string
	path     string
	modified bool
}

func (m *mockBufferProvider) Text() string { return m.text }
func (m *mockBufferProvider) TextRange(start, end int) (string, error) {
	if start < 0 || end > len(m.text) || start > end {
		return "", errors.New("invalid range")
	}
	return m.text[start:end], nil
}
func (m *mockBufferProvider) Line(lineNum int) (string, error) {
	lines := strings.Split(m.text, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return "", errors.New("invalid line number")
	}
	return lines[lineNum-1], nil
}
func (m *mockBufferProvider) LineCount() int {
	if m.text == "" {
		return 0
	}
	return strings.Count(m.text, "\n") + 1
}
func (m *mockBufferProvider) Len() int { return len(m.text) }
func (m *mockBufferProvider) Insert(offset int, text string) (int, error) {
	if offset < 0 || offset > len(m.text) {
		return 0, errors.New("invalid offset")
	}
	m.text = m.text[:offset] + text + m.text[offset:]
	m.modified = true
	return offset + len(text), nil
}
func (m *mockBufferProvider) Delete(start, end int) error {
	if start < 0 || end > len(m.text) || start > end {
		return errors.New("invalid range")
	}
	m.text = m.text[:start] + m.text[end:]
	m.modified = true
	return nil
}
func (m *mockBufferProvider) Replace(start, end int, text string) (int, error) {
	if start < 0 || end > len(m.text) || start > end {
		return 0, errors.New("invalid range")
	}
	m.text = m.text[:start] + text + m.text[end:]
	m.modified = true
	return start + len(text), nil
}
func (m *mockBufferProvider) Undo() bool {
	// Simple mock: just return true
	return true
}
func (m *mockBufferProvider) Redo() bool {
	return true
}
func (m *mockBufferProvider) Path() string   { return m.path }
func (m *mockBufferProvider) Modified() bool { return m.modified }

func setupBufferTest(t *testing.T, buf *mockBufferProvider) (*lua.LState, *BufferModule) {
	t.Helper()

	ctx := &Context{Buffer: buf}
	mod := NewBufferModule(ctx)

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	return L, mod
}

func TestBufferModuleName(t *testing.T) {
	mod := NewBufferModule(&Context{})
	if mod.Name() != "buf" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "buf")
	}
}

func TestBufferModuleCapability(t *testing.T) {
	mod := NewBufferModule(&Context{})
	if mod.RequiredCapability() != security.CapabilityBuffer {
		t.Errorf("RequiredCapability() = %q, want %q", mod.RequiredCapability(), security.CapabilityBuffer)
	}
}

func TestBufferText(t *testing.T) {
	buf := &mockBufferProvider{text: "hello world"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.text()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.String() != "hello world" {
		t.Errorf("text() = %q, want %q", result.String(), "hello world")
	}
}

func TestBufferTextRange(t *testing.T) {
	buf := &mockBufferProvider{text: "hello world"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.text_range(0, 5)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.String() != "hello" {
		t.Errorf("text_range(0, 5) = %q, want %q", result.String(), "hello")
	}
}

func TestBufferLine(t *testing.T) {
	buf := &mockBufferProvider{text: "line1\nline2\nline3"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.line(2)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.String() != "line2" {
		t.Errorf("line(2) = %q, want %q", result.String(), "line2")
	}
}

func TestBufferLineCount(t *testing.T) {
	buf := &mockBufferProvider{text: "line1\nline2\nline3"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.line_count()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.(lua.LNumber) != 3 {
		t.Errorf("line_count() = %v, want 3", result)
	}
}

func TestBufferLen(t *testing.T) {
	buf := &mockBufferProvider{text: "hello"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.len()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.(lua.LNumber) != 5 {
		t.Errorf("len() = %v, want 5", result)
	}
}

func TestBufferInsert(t *testing.T) {
	buf := &mockBufferProvider{text: "hello"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.insert(5, " world")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if buf.text != "hello world" {
		t.Errorf("buffer text = %q, want %q", buf.text, "hello world")
	}

	result := L.GetGlobal("result")
	if result.(lua.LNumber) != 11 {
		t.Errorf("insert returned %v, want 11", result)
	}
}

func TestBufferDelete(t *testing.T) {
	buf := &mockBufferProvider{text: "hello world"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		_ks_buf.delete(5, 11)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if buf.text != "hello" {
		t.Errorf("buffer text = %q, want %q", buf.text, "hello")
	}
}

func TestBufferReplace(t *testing.T) {
	buf := &mockBufferProvider{text: "hello world"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.replace(6, 11, "lua")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if buf.text != "hello lua" {
		t.Errorf("buffer text = %q, want %q", buf.text, "hello lua")
	}
}

func TestBufferUndo(t *testing.T) {
	buf := &mockBufferProvider{text: "hello"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.undo()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LTrue {
		t.Errorf("undo() = %v, want true", result)
	}
}

func TestBufferRedo(t *testing.T) {
	buf := &mockBufferProvider{text: "hello"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.redo()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LTrue {
		t.Errorf("redo() = %v, want true", result)
	}
}

func TestBufferPath(t *testing.T) {
	buf := &mockBufferProvider{text: "", path: "/test/file.txt"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.path()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result.String() != "/test/file.txt" {
		t.Errorf("path() = %q, want %q", result.String(), "/test/file.txt")
	}
}

func TestBufferModified(t *testing.T) {
	buf := &mockBufferProvider{text: "", modified: true}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		result = _ks_buf.modified()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LTrue {
		t.Errorf("modified() = %v, want true", result)
	}
}

func TestBufferNilContext(t *testing.T) {
	// Test with nil buffer provider
	ctx := &Context{Buffer: nil}
	mod := NewBufferModule(ctx)

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// Should not panic, should return empty/default values
	err := L.DoString(`
		assert(_ks_buf.text() == "", "text should be empty")
		assert(_ks_buf.line_count() == 0, "line_count should be 0")
		assert(_ks_buf.len() == 0, "len should be 0")
		assert(_ks_buf.modified() == false, "modified should be false")
		assert(_ks_buf.path() == "", "path should be empty")
	`)
	if err != nil {
		t.Errorf("DoString with nil buffer error = %v", err)
	}
}

func TestBufferInsertNegativeOffset(t *testing.T) {
	buf := &mockBufferProvider{text: "hello"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		_ks_buf.insert(-1, "x")
	`)
	if err == nil {
		t.Error("insert with negative offset should error")
	}
}

func TestBufferDeleteInvalidRange(t *testing.T) {
	buf := &mockBufferProvider{text: "hello"}
	L, _ := setupBufferTest(t, buf)

	err := L.DoString(`
		_ks_buf.delete(5, 3) -- end < start
	`)
	if err == nil {
		t.Error("delete with invalid range should error")
	}
}
