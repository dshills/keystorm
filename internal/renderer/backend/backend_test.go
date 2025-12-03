package backend

import (
	"testing"

	"github.com/dshills/keystorm/internal/renderer/core"
)

func TestNullBackendInit(t *testing.T) {
	b := NewNullBackend(80, 24)
	if err := b.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	w, h := b.Size()
	if w != 80 || h != 24 {
		t.Errorf("expected size (80, 24), got (%d, %d)", w, h)
	}
}

func TestNullBackendSetGetCell(t *testing.T) {
	b := NewNullBackend(80, 24)
	b.Init()

	cell := core.NewStyledCell('X', core.DefaultStyle().WithForeground(core.ColorRed))
	b.SetCell(10, 5, cell)

	got := b.GetCell(10, 5)
	if !got.Equals(cell) {
		t.Errorf("cell mismatch: expected %+v, got %+v", cell, got)
	}

	// Out of bounds should be ignored/return empty
	b.SetCell(-1, 0, cell)
	b.SetCell(100, 0, cell)

	empty := b.GetCell(-1, 0)
	if !empty.Equals(core.EmptyCell()) {
		t.Error("out of bounds should return empty cell")
	}
}

func TestNullBackendFill(t *testing.T) {
	b := NewNullBackend(80, 24)
	b.Init()

	cell := core.NewCell('.')
	rect := core.NewScreenRect(5, 10, 10, 20)
	b.Fill(rect, cell)

	// Check inside rect
	got := b.GetCell(15, 7)
	if !got.Equals(cell) {
		t.Error("cell inside rect should be filled")
	}

	// Check outside rect
	got = b.GetCell(0, 0)
	if got.Equals(cell) {
		t.Error("cell outside rect should not be filled")
	}
}

func TestNullBackendClear(t *testing.T) {
	b := NewNullBackend(80, 24)
	b.Init()

	// Set some cells
	b.SetCell(10, 10, core.NewCell('X'))
	b.SetCell(20, 20, core.NewCell('Y'))

	b.Clear()

	// All cells should be empty
	got := b.GetCell(10, 10)
	if !got.Equals(core.EmptyCell()) {
		t.Error("clear should reset all cells")
	}
}

func TestNullBackendCursor(t *testing.T) {
	b := NewNullBackend(80, 24)
	b.Init()

	b.ShowCursor(15, 10)
	x, y, visible := b.CursorPosition()
	if x != 15 || y != 10 || !visible {
		t.Errorf("cursor position: expected (15, 10, true), got (%d, %d, %v)", x, y, visible)
	}

	b.HideCursor()
	_, _, visible = b.CursorPosition()
	if visible {
		t.Error("cursor should be hidden")
	}
}

func TestNullBackendCursorStyle(t *testing.T) {
	b := NewNullBackend(80, 24)
	b.Init()

	b.SetCursorStyle(CursorBar)
	if b.CursorStyleValue() != CursorBar {
		t.Error("cursor style should be bar")
	}

	b.SetCursorStyle(CursorUnderline)
	if b.CursorStyleValue() != CursorUnderline {
		t.Error("cursor style should be underline")
	}
}

func TestNullBackendResize(t *testing.T) {
	b := NewNullBackend(80, 24)
	b.Init()

	resizeCalled := false
	b.OnResize(func(w, h int) {
		resizeCalled = true
		if w != 100 || h != 40 {
			t.Errorf("resize callback: expected (100, 40), got (%d, %d)", w, h)
		}
	})

	b.Resize(100, 40)

	if !resizeCalled {
		t.Error("resize callback was not called")
	}

	w, h := b.Size()
	if w != 100 || h != 40 {
		t.Errorf("expected size (100, 40), got (%d, %d)", w, h)
	}
}

func TestNullBackendPostEvent(t *testing.T) {
	b := NewNullBackend(80, 24)
	b.Init()

	event := Event{
		Type: EventKey,
		Key:  KeyEnter,
	}
	b.PostEvent(event)

	// Should be able to poll the event back
	got := b.PollEvent()
	if got.Type != EventKey || got.Key != KeyEnter {
		t.Errorf("expected enter key event, got %+v", got)
	}
}

func TestNullBackendHasTrueColor(t *testing.T) {
	b := NewNullBackend(80, 24)
	if !b.HasTrueColor() {
		t.Error("null backend should report true color support")
	}
}

func TestModMaskHas(t *testing.T) {
	mod := ModShift | ModCtrl

	if !mod.Has(ModShift) {
		t.Error("should have shift")
	}
	if !mod.Has(ModCtrl) {
		t.Error("should have ctrl")
	}
	if mod.Has(ModAlt) {
		t.Error("should not have alt")
	}
}

func TestEventTypes(t *testing.T) {
	// Key event
	keyEvent := Event{Type: EventKey, Key: KeyEscape, Mod: ModShift}
	if keyEvent.Type != EventKey {
		t.Error("should be key event")
	}

	// Mouse event
	mouseEvent := Event{Type: EventMouse, MouseX: 10, MouseY: 20, MouseButton: MouseLeft}
	if mouseEvent.Type != EventMouse {
		t.Error("should be mouse event")
	}

	// Resize event
	resizeEvent := Event{Type: EventResize, Width: 100, Height: 50}
	if resizeEvent.Type != EventResize {
		t.Error("should be resize event")
	}

	// Paste event
	pasteEvent := Event{Type: EventPaste, PasteText: "hello"}
	if pasteEvent.Type != EventPaste {
		t.Error("should be paste event")
	}

	// Focus event
	focusEvent := Event{Type: EventFocus, Focused: true}
	if focusEvent.Type != EventFocus {
		t.Error("should be focus event")
	}
}
