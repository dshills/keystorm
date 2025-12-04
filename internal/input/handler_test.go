package input

import (
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/input/key"
	"github.com/dshills/keystorm/internal/input/keymap"
	"github.com/dshills/keystorm/internal/input/mode"
)

func TestNewHandler(t *testing.T) {
	config := DefaultConfig()
	h := NewHandler(config)
	defer h.Close()

	if h == nil {
		t.Fatal("NewHandler returned nil")
	}

	if h.CurrentMode() != mode.ModeNormal {
		t.Errorf("expected initial mode %q, got %q", mode.ModeNormal, h.CurrentMode())
	}

	if h.IsClosed() {
		t.Error("handler should not be closed after creation")
	}
}

func TestHandlerDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.DefaultMode != mode.ModeNormal {
		t.Errorf("expected default mode %q, got %q", mode.ModeNormal, config.DefaultMode)
	}

	if !config.EnableModes {
		t.Error("expected EnableModes to be true by default")
	}

	if config.SequenceTimeout != 1000*time.Millisecond {
		t.Errorf("expected sequence timeout 1000ms, got %v", config.SequenceTimeout)
	}
}

func TestHandlerSwitchMode(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	// Switch to insert mode
	if err := h.SwitchMode(mode.ModeInsert); err != nil {
		t.Fatalf("failed to switch to insert mode: %v", err)
	}

	if h.CurrentMode() != mode.ModeInsert {
		t.Errorf("expected mode %q, got %q", mode.ModeInsert, h.CurrentMode())
	}

	// Switch to visual mode
	if err := h.SwitchMode(mode.ModeVisual); err != nil {
		t.Fatalf("failed to switch to visual mode: %v", err)
	}

	if h.CurrentMode() != mode.ModeVisual {
		t.Errorf("expected mode %q, got %q", mode.ModeVisual, h.CurrentMode())
	}

	// Switch back to normal mode
	if err := h.SwitchMode(mode.ModeNormal); err != nil {
		t.Fatalf("failed to switch to normal mode: %v", err)
	}

	if h.CurrentMode() != mode.ModeNormal {
		t.Errorf("expected mode %q, got %q", mode.ModeNormal, h.CurrentMode())
	}
}

func TestHandlerSwitchModeInvalid(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	err := h.SwitchMode("nonexistent")
	if err == nil {
		t.Error("expected error when switching to nonexistent mode")
	}
}

func TestHandlerPendingKeys(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	// Initially no pending keys
	if h.PendingKeys() != "" {
		t.Errorf("expected no pending keys, got %q", h.PendingKeys())
	}

	// Send a key that might be part of a sequence
	h.HandleKeyEvent(key.NewRuneEvent('g', key.ModNone))

	// Should have pending key
	pending := h.PendingKeys()
	if pending == "" {
		t.Error("expected pending keys after 'g'")
	}
}

func TestHandlerSetCount(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	h.SetCount(5)

	ctx := h.Context()
	if ctx.PendingCount != 5 {
		t.Errorf("expected pending count 5, got %d", ctx.PendingCount)
	}

	if ctx.GetCount() != 5 {
		t.Errorf("expected GetCount() to return 5, got %d", ctx.GetCount())
	}
}

func TestHandlerSetRegister(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	h.SetRegister('a')

	ctx := h.Context()
	if ctx.PendingRegister != 'a' {
		t.Errorf("expected pending register 'a', got %c", ctx.PendingRegister)
	}
}

func TestHandlerSetOperator(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	h.SetOperator("delete")

	ctx := h.Context()
	if ctx.PendingOperator != "delete" {
		t.Errorf("expected pending operator 'delete', got %q", ctx.PendingOperator)
	}
}

func TestHandlerContext(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	ctx := h.Context()
	if ctx == nil {
		t.Fatal("Context() returned nil")
	}

	// Verify it's a copy
	ctx.Mode = "modified"
	if h.CurrentMode() == "modified" {
		t.Error("Context() should return a copy, not the original")
	}
}

func TestHandlerModeManager(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	mm := h.ModeManager()
	if mm == nil {
		t.Fatal("ModeManager() returned nil")
	}

	// Should have all default modes registered
	modes := []string{
		mode.ModeNormal,
		mode.ModeInsert,
		mode.ModeVisual,
		mode.ModeVisualLine,
		mode.ModeVisualBlock,
		mode.ModeCommand,
		mode.ModeOperatorPending,
		mode.ModeReplace,
	}

	for _, modeName := range modes {
		if mm.Get(modeName) == nil {
			t.Errorf("expected mode %q to be registered", modeName)
		}
	}
}

func TestHandlerKeymapRegistry(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	kr := h.KeymapRegistry()
	if kr == nil {
		t.Fatal("KeymapRegistry() returned nil")
	}
}

func TestHandlerActions(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	actions := h.Actions()
	if actions == nil {
		t.Fatal("Actions() returned nil")
	}
}

func TestHandlerClose(t *testing.T) {
	h := NewHandler(DefaultConfig())

	if h.IsClosed() {
		t.Error("handler should not be closed initially")
	}

	h.Close()

	if !h.IsClosed() {
		t.Error("handler should be closed after Close()")
	}

	// Calling Close again should be safe
	h.Close()

	if !h.IsClosed() {
		t.Error("handler should still be closed")
	}
}

func TestHandlerWithBinding(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	// Register a test keymap with a binding using a unique key (Q is unused)
	km := keymap.NewKeymap("test").
		ForMode(mode.ModeNormal).
		WithPriority(100). // Higher priority than defaults
		Add("Q", "test.action")

	if err := h.KeymapRegistry().Register(km); err != nil {
		t.Fatalf("failed to register keymap: %v", err)
	}

	// Send the key
	h.HandleKeyEvent(key.NewRuneEvent('Q', key.ModNone))

	// Check that an action was dispatched
	select {
	case action := <-h.Actions():
		if action.Name != "test.action" {
			t.Errorf("expected action 'test.action', got %q", action.Name)
		}
		if action.Source != SourceKeyboard {
			t.Errorf("expected source SourceKeyboard, got %v", action.Source)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected action to be dispatched")
	}
}

func TestHandlerWithSequence(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	// Register a two-key sequence using unique keys
	km := keymap.NewKeymap("test").
		ForMode(mode.ModeNormal).
		WithPriority(100). // Higher priority than defaults
		Add("Z Z", "test.sequence")

	if err := h.KeymapRegistry().Register(km); err != nil {
		t.Fatalf("failed to register keymap: %v", err)
	}

	// Send first key
	h.HandleKeyEvent(key.NewRuneEvent('Z', key.ModNone))

	// Should not dispatch yet (waiting for more keys)
	select {
	case action := <-h.Actions():
		t.Errorf("unexpected action dispatched: %v", action)
	case <-time.After(50 * time.Millisecond):
		// Expected - no action yet
	}

	// Send second key
	h.HandleKeyEvent(key.NewRuneEvent('Z', key.ModNone))

	// Now should dispatch
	select {
	case action := <-h.Actions():
		if action.Name != "test.sequence" {
			t.Errorf("expected action 'test.sequence', got %q", action.Name)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected action to be dispatched after sequence")
	}
}

func TestHandlerWithCount(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	// Register a binding
	km := keymap.NewKeymap("test").
		ForMode(mode.ModeNormal).
		Add("j", "cursor.down")

	if err := h.KeymapRegistry().Register(km); err != nil {
		t.Fatalf("failed to register keymap: %v", err)
	}

	// Set count and send key
	h.SetCount(5)
	h.HandleKeyEvent(key.NewRuneEvent('j', key.ModNone))

	// Check that action has count
	select {
	case action := <-h.Actions():
		if action.Count != 5 {
			t.Errorf("expected count 5, got %d", action.Count)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected action to be dispatched")
	}
}

func TestHandlerWithRegister(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	// Register a binding
	km := keymap.NewKeymap("test").
		ForMode(mode.ModeNormal).
		Add("y", "yank")

	if err := h.KeymapRegistry().Register(km); err != nil {
		t.Fatalf("failed to register keymap: %v", err)
	}

	// Set register and send key
	h.SetRegister('a')
	h.HandleKeyEvent(key.NewRuneEvent('y', key.ModNone))

	// Check that action has register
	select {
	case action := <-h.Actions():
		if action.Args.Register != 'a' {
			t.Errorf("expected register 'a', got %c", action.Args.Register)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected action to be dispatched")
	}
}

// TestHook is a test hook for verifying hook behavior.
type testHook struct {
	preKeyEventCalled  bool
	postKeyEventCalled bool
	preActionCalled    bool
	consumeKeyEvent    bool
	consumeAction      bool
}

func (h *testHook) PreKeyEvent(event *key.Event, ctx *Context) bool {
	h.preKeyEventCalled = true
	return h.consumeKeyEvent
}

func (h *testHook) PostKeyEvent(event *key.Event, action *Action, ctx *Context) {
	h.postKeyEventCalled = true
}

func (h *testHook) PreAction(action *Action, ctx *Context) bool {
	h.preActionCalled = true
	return h.consumeAction
}

func TestHandlerAddRemoveHook(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	// Register a binding
	km := keymap.NewKeymap("test").
		ForMode(mode.ModeNormal).
		Add("x", "test")

	if err := h.KeymapRegistry().Register(km); err != nil {
		t.Fatalf("failed to register keymap: %v", err)
	}

	hook := &testHook{}
	h.AddHook(hook)

	// Send a key
	h.HandleKeyEvent(key.NewRuneEvent('x', key.ModNone))

	if !hook.preKeyEventCalled {
		t.Error("PreKeyEvent should have been called")
	}
	if !hook.postKeyEventCalled {
		t.Error("PostKeyEvent should have been called")
	}
	if !hook.preActionCalled {
		t.Error("PreAction should have been called")
	}

	// Drain the action
	select {
	case <-h.Actions():
	case <-time.After(100 * time.Millisecond):
	}

	// Reset and remove hook
	hook.preKeyEventCalled = false
	hook.postKeyEventCalled = false
	hook.preActionCalled = false
	h.RemoveHook(hook)

	// Send another key
	h.HandleKeyEvent(key.NewRuneEvent('x', key.ModNone))

	if hook.preKeyEventCalled {
		t.Error("PreKeyEvent should not be called after hook removal")
	}
}

func TestHandlerHookConsumeKeyEvent(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	// Register a binding
	km := keymap.NewKeymap("test").
		ForMode(mode.ModeNormal).
		Add("x", "test")

	if err := h.KeymapRegistry().Register(km); err != nil {
		t.Fatalf("failed to register keymap: %v", err)
	}

	hook := &testHook{consumeKeyEvent: true}
	h.AddHook(hook)

	// Send a key
	h.HandleKeyEvent(key.NewRuneEvent('x', key.ModNone))

	// Should not dispatch action (key was consumed)
	select {
	case action := <-h.Actions():
		t.Errorf("unexpected action: %v (key should have been consumed)", action)
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

func TestHandlerHookConsumeAction(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	// Register a binding
	km := keymap.NewKeymap("test").
		ForMode(mode.ModeNormal).
		Add("x", "test")

	if err := h.KeymapRegistry().Register(km); err != nil {
		t.Fatalf("failed to register keymap: %v", err)
	}

	hook := &testHook{consumeAction: true}
	h.AddHook(hook)

	// Send a key
	h.HandleKeyEvent(key.NewRuneEvent('x', key.ModNone))

	// Should not dispatch action (action was consumed by hook)
	select {
	case action := <-h.Actions():
		t.Errorf("unexpected action: %v (action should have been consumed)", action)
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

func TestHandlerSequenceTimeout(t *testing.T) {
	// Use a short timeout for testing
	config := DefaultConfig()
	config.SequenceTimeout = 50 * time.Millisecond
	h := NewHandler(config)
	defer h.Close()

	// Register a two-key sequence
	km := keymap.NewKeymap("test").
		ForMode(mode.ModeNormal).
		Add("g g", "goto.top")

	if err := h.KeymapRegistry().Register(km); err != nil {
		t.Fatalf("failed to register keymap: %v", err)
	}

	// Send first key
	h.HandleKeyEvent(key.NewRuneEvent('g', key.ModNone))

	// Verify we have pending keys
	if h.PendingKeys() == "" {
		t.Error("expected pending keys after first key")
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Pending keys should be cleared
	if h.PendingKeys() != "" {
		t.Errorf("expected pending keys to be cleared after timeout, got %q", h.PendingKeys())
	}
}

func TestHandlerClosedIgnoresEvents(t *testing.T) {
	h := NewHandler(DefaultConfig())

	// Register a binding
	km := keymap.NewKeymap("test").
		ForMode(mode.ModeNormal).
		Add("x", "test")

	if err := h.KeymapRegistry().Register(km); err != nil {
		t.Fatalf("failed to register keymap: %v", err)
	}

	// Close the handler
	h.Close()

	// Send a key - should not panic
	h.HandleKeyEvent(key.NewRuneEvent('x', key.ModNone))

	// Actions channel should be closed
	_, ok := <-h.Actions()
	if ok {
		t.Error("expected actions channel to be closed")
	}
}

// mockEditorState implements EditorStateProvider for testing.
type mockEditorState struct {
	mode         string
	fileType     string
	filePath     string
	hasSelection bool
	isModified   bool
	isReadOnly   bool
	line, col    uint32
}

func (m *mockEditorState) Mode() string                       { return m.mode }
func (m *mockEditorState) FileType() string                   { return m.fileType }
func (m *mockEditorState) FilePath() string                   { return m.filePath }
func (m *mockEditorState) HasSelection() bool                 { return m.hasSelection }
func (m *mockEditorState) IsModified() bool                   { return m.isModified }
func (m *mockEditorState) IsReadOnly() bool                   { return m.isReadOnly }
func (m *mockEditorState) CursorPosition() (line, col uint32) { return m.line, m.col }

func TestHandlerUpdateContext(t *testing.T) {
	h := NewHandler(DefaultConfig())
	defer h.Close()

	editor := &mockEditorState{
		mode:         mode.ModeInsert,
		fileType:     "go",
		filePath:     "/path/to/file.go",
		hasSelection: true,
		isModified:   true,
		isReadOnly:   false,
		line:         10,
		col:          5,
	}

	h.UpdateContext(editor)

	ctx := h.Context()
	if ctx.Mode != mode.ModeInsert {
		t.Errorf("expected mode %q, got %q", mode.ModeInsert, ctx.Mode)
	}
	if ctx.FileType != "go" {
		t.Errorf("expected file type 'go', got %q", ctx.FileType)
	}
	if ctx.FilePath != "/path/to/file.go" {
		t.Errorf("expected file path '/path/to/file.go', got %q", ctx.FilePath)
	}
	if !ctx.HasSelection {
		t.Error("expected HasSelection to be true")
	}
	if !ctx.IsModified {
		t.Error("expected IsModified to be true")
	}
	if ctx.IsReadOnly {
		t.Error("expected IsReadOnly to be false")
	}
	if ctx.LineNumber != 10 {
		t.Errorf("expected line 10, got %d", ctx.LineNumber)
	}
	if ctx.ColumnNumber != 5 {
		t.Errorf("expected column 5, got %d", ctx.ColumnNumber)
	}
}
