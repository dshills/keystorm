package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/input"
)

// mockEngine implements execctx.EngineInterface for testing.
type mockEngine struct {
	text string
}

func newMockEngine(text string) *mockEngine {
	return &mockEngine{text: text}
}

func (e *mockEngine) Insert(offset buffer.ByteOffset, text string) (buffer.EditResult, error) {
	return buffer.EditResult{}, nil
}

func (e *mockEngine) Delete(start, end buffer.ByteOffset) (buffer.EditResult, error) {
	return buffer.EditResult{}, nil
}

func (e *mockEngine) Replace(start, end buffer.ByteOffset, text string) (buffer.EditResult, error) {
	return buffer.EditResult{}, nil
}

func (e *mockEngine) Text() string { return e.text }

func (e *mockEngine) TextRange(start, end buffer.ByteOffset) string {
	return e.text[start:end]
}

func (e *mockEngine) LineText(line uint32) string { return "" }

func (e *mockEngine) Len() buffer.ByteOffset {
	return buffer.ByteOffset(len(e.text))
}

func (e *mockEngine) LineCount() uint32                        { return 1 }
func (e *mockEngine) LineStartOffset(uint32) buffer.ByteOffset { return 0 }
func (e *mockEngine) LineEndOffset(uint32) buffer.ByteOffset   { return e.Len() }
func (e *mockEngine) LineLen(uint32) uint32                    { return uint32(len(e.text)) }

func (e *mockEngine) OffsetToPoint(offset buffer.ByteOffset) buffer.Point {
	return buffer.Point{Line: 0, Column: uint32(offset)}
}

func (e *mockEngine) PointToOffset(point buffer.Point) buffer.ByteOffset {
	return buffer.ByteOffset(point.Column)
}

func (e *mockEngine) Snapshot() execctx.EngineReader { return e }
func (e *mockEngine) RevisionID() buffer.RevisionID  { return 0 }

// mockFileManager implements FileManager for testing.
type mockFileManager struct {
	files    map[string]string
	readonly map[string]bool
}

func newMockFileManager() *mockFileManager {
	return &mockFileManager{
		files:    make(map[string]string),
		readonly: make(map[string]bool),
	}
}

func (m *mockFileManager) OpenFile(path string) (string, error) {
	content, ok := m.files[path]
	if !ok {
		return "", os.ErrNotExist
	}
	return content, nil
}

func (m *mockFileManager) SaveFile(path string, content string) error {
	if m.readonly[path] {
		return os.ErrPermission
	}
	m.files[path] = content
	return nil
}

func (m *mockFileManager) CreateFile(path string) error {
	m.files[path] = ""
	return nil
}

func (m *mockFileManager) FileExists(path string) bool {
	_, ok := m.files[path]
	return ok
}

func (m *mockFileManager) IsReadOnly(path string) bool {
	return m.readonly[path]
}

// mockBufferManager implements BufferManager for testing.
type mockBufferManager struct {
	buffers  []string
	current  int
	modified map[int]bool
}

func newMockBufferManager() *mockBufferManager {
	return &mockBufferManager{
		buffers:  []string{"buffer1.txt"},
		current:  0,
		modified: make(map[int]bool),
	}
}

func (m *mockBufferManager) CurrentBuffer() int   { return m.current }
func (m *mockBufferManager) BufferCount() int     { return len(m.buffers) }
func (m *mockBufferManager) BufferList() []string { return m.buffers }

func (m *mockBufferManager) SwitchBuffer(index int) error {
	if index < 0 || index >= len(m.buffers) {
		return os.ErrNotExist
	}
	m.current = index
	return nil
}

func (m *mockBufferManager) CloseBuffer(index int) error {
	if index < 0 || index >= len(m.buffers) {
		return os.ErrNotExist
	}
	m.buffers = append(m.buffers[:index], m.buffers[index+1:]...)
	if m.current >= len(m.buffers) && m.current > 0 {
		m.current--
	}
	return nil
}

func (m *mockBufferManager) NewBuffer() (int, error) {
	m.buffers = append(m.buffers, "untitled")
	return len(m.buffers) - 1, nil
}

func (m *mockBufferManager) BufferModified(index int) bool {
	return m.modified[index]
}

func TestHandler_Namespace(t *testing.T) {
	h := NewHandler()
	if h.Namespace() != "file" {
		t.Errorf("expected namespace 'file', got '%s'", h.Namespace())
	}
}

func TestHandler_CanHandle(t *testing.T) {
	h := NewHandler()

	validActions := []string{
		ActionSave,
		ActionSaveAs,
		ActionSaveAll,
		ActionOpen,
		ActionReload,
		ActionClose,
		ActionCloseAll,
		ActionNew,
		ActionNextBuffer,
		ActionPrevBuffer,
		ActionListBuffer,
	}

	for _, action := range validActions {
		if !h.CanHandle(action) {
			t.Errorf("expected CanHandle(%s) to return true", action)
		}
	}

	if h.CanHandle("invalid.action") {
		t.Error("expected CanHandle('invalid.action') to return false")
	}
}

func TestHandler_Save(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	h := NewHandler()
	engine := newMockEngine("hello world")

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.FilePath = testFile

	action := input.Action{Name: ActionSave}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Verify file was written
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("failed to read saved file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected content 'hello world', got '%s'", string(content))
	}
}

func TestHandler_SaveWithFileManager(t *testing.T) {
	fm := newMockFileManager()
	h := NewHandlerWithManagers(fm, nil)
	engine := newMockEngine("test content")

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.FilePath = "/test/file.txt"

	action := input.Action{Name: ActionSave}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Verify content was saved via file manager
	saved, ok := fm.files["/test/file.txt"]
	if !ok {
		t.Error("file was not saved to file manager")
	}
	if saved != "test content" {
		t.Errorf("expected 'test content', got '%s'", saved)
	}
}

func TestHandler_SaveNoPath(t *testing.T) {
	h := NewHandler()
	engine := newMockEngine("content")

	ctx := execctx.New()
	ctx.Engine = engine
	// No FilePath set

	action := input.Action{Name: ActionSave}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for no path, got %v", result.Status)
	}
}

func TestHandler_SaveAs(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "saveas.txt")

	h := NewHandler()
	engine := newMockEngine("save as content")

	ctx := execctx.New()
	ctx.Engine = engine

	action := input.Action{
		Name: ActionSaveAs,
		Args: input.ActionArgs{
			Extra: map[string]interface{}{"path": testFile},
		},
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Verify file was written
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("failed to read saved file: %v", err)
	}
	if string(content) != "save as content" {
		t.Errorf("expected 'save as content', got '%s'", string(content))
	}
}

func TestHandler_Open(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "open.txt")
	err := os.WriteFile(testFile, []byte("file content"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := NewHandler()
	ctx := execctx.New()

	action := input.Action{
		Name: ActionOpen,
		Args: input.ActionArgs{
			Extra: map[string]interface{}{"path": testFile},
		},
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Check content was returned in result data
	content, ok := result.GetData("content")
	if !ok {
		t.Error("expected content in result data")
	}
	if content.(string) != "file content" {
		t.Errorf("expected 'file content', got '%s'", content.(string))
	}
}

func TestHandler_OpenWithFileManager(t *testing.T) {
	fm := newMockFileManager()
	fm.files["/test/file.txt"] = "managed content"

	h := NewHandlerWithManagers(fm, nil)
	ctx := execctx.New()

	action := input.Action{
		Name: ActionOpen,
		Args: input.ActionArgs{
			Extra: map[string]interface{}{"path": "/test/file.txt"},
		},
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	content, _ := result.GetData("content")
	if content.(string) != "managed content" {
		t.Errorf("expected 'managed content', got '%s'", content.(string))
	}
}

func TestHandler_OpenNotFound(t *testing.T) {
	h := NewHandler()
	ctx := execctx.New()

	action := input.Action{
		Name: ActionOpen,
		Args: input.ActionArgs{
			Extra: map[string]interface{}{"path": "/nonexistent/file.txt"},
		},
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for non-existent file, got %v", result.Status)
	}
}

func TestHandler_Reload(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "reload.txt")
	err := os.WriteFile(testFile, []byte("original content"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := NewHandler()
	engine := newMockEngine("modified content")

	ctx := execctx.New()
	ctx.Engine = engine
	ctx.FilePath = testFile

	action := input.Action{Name: ActionReload}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	// Check original content was returned
	content, ok := result.GetData("content")
	if !ok {
		t.Error("expected content in result data")
	}
	if content.(string) != "original content" {
		t.Errorf("expected 'original content', got '%s'", content.(string))
	}
}

func TestHandler_Close(t *testing.T) {
	bm := newMockBufferManager()
	h := NewHandlerWithManagers(nil, bm)
	ctx := execctx.New()

	action := input.Action{Name: ActionClose}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if bm.BufferCount() != 0 {
		t.Errorf("expected 0 buffers after close, got %d", bm.BufferCount())
	}
}

func TestHandler_CloseModified(t *testing.T) {
	bm := newMockBufferManager()
	bm.modified[0] = true

	h := NewHandlerWithManagers(nil, bm)
	ctx := execctx.New()

	action := input.Action{Name: ActionClose}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for modified buffer, got %v", result.Status)
	}

	// Buffer should still exist
	if bm.BufferCount() != 1 {
		t.Errorf("expected 1 buffer after failed close, got %d", bm.BufferCount())
	}
}

func TestHandler_New(t *testing.T) {
	bm := newMockBufferManager()
	h := NewHandlerWithManagers(nil, bm)
	ctx := execctx.New()

	action := input.Action{Name: ActionNew}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if bm.BufferCount() != 2 {
		t.Errorf("expected 2 buffers after new, got %d", bm.BufferCount())
	}

	// Should switch to new buffer
	if bm.CurrentBuffer() != 1 {
		t.Errorf("expected current buffer 1, got %d", bm.CurrentBuffer())
	}
}

func TestHandler_NextBuffer(t *testing.T) {
	bm := newMockBufferManager()
	bm.buffers = []string{"a.txt", "b.txt", "c.txt"}

	h := NewHandlerWithManagers(nil, bm)
	ctx := execctx.New()

	action := input.Action{Name: ActionNextBuffer}

	// First next
	result := h.HandleAction(action, ctx)
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
	if bm.CurrentBuffer() != 1 {
		t.Errorf("expected buffer 1, got %d", bm.CurrentBuffer())
	}

	// Second next
	h.HandleAction(action, ctx)
	if bm.CurrentBuffer() != 2 {
		t.Errorf("expected buffer 2, got %d", bm.CurrentBuffer())
	}

	// Third next should wrap to 0
	h.HandleAction(action, ctx)
	if bm.CurrentBuffer() != 0 {
		t.Errorf("expected buffer 0 (wrapped), got %d", bm.CurrentBuffer())
	}
}

func TestHandler_PrevBuffer(t *testing.T) {
	bm := newMockBufferManager()
	bm.buffers = []string{"a.txt", "b.txt", "c.txt"}
	bm.current = 0

	h := NewHandlerWithManagers(nil, bm)
	ctx := execctx.New()

	action := input.Action{Name: ActionPrevBuffer}

	// First prev should wrap to last
	result := h.HandleAction(action, ctx)
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
	if bm.CurrentBuffer() != 2 {
		t.Errorf("expected buffer 2 (wrapped), got %d", bm.CurrentBuffer())
	}
}

func TestHandler_ListBuffers(t *testing.T) {
	bm := newMockBufferManager()
	bm.buffers = []string{"a.txt", "b.txt", "c.txt"}
	bm.current = 1

	h := NewHandlerWithManagers(nil, bm)
	ctx := execctx.New()

	action := input.Action{Name: ActionListBuffer}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	buffers, ok := result.GetData("buffers")
	if !ok {
		t.Error("expected buffers in result data")
	}
	if len(buffers.([]string)) != 3 {
		t.Errorf("expected 3 buffers, got %d", len(buffers.([]string)))
	}

	current, ok := result.GetData("current")
	if !ok {
		t.Error("expected current in result data")
	}
	if current.(int) != 1 {
		t.Errorf("expected current 1, got %d", current.(int))
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{-1, "-1"},
		{-123, "-123"},
	}

	for _, tt := range tests {
		got := itoa(tt.n)
		if got != tt.want {
			t.Errorf("itoa(%d) = %s, want %s", tt.n, got, tt.want)
		}
	}
}
