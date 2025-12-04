package window

import (
	"errors"
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// mockWindowManager implements WindowManager for testing.
type mockWindowManager struct {
	windows       int
	current       int
	lastDirection Direction
	lastResize    [2]int
	rotated       bool
	maximized     bool
}

func newMockWindowManager() *mockWindowManager {
	return &mockWindowManager{
		windows: 1,
		current: 0,
	}
}

func (m *mockWindowManager) SplitHorizontal() error {
	m.windows++
	return nil
}

func (m *mockWindowManager) SplitVertical() error {
	m.windows++
	return nil
}

func (m *mockWindowManager) Focus(dir Direction) error {
	m.lastDirection = dir
	return nil
}

func (m *mockWindowManager) FocusNext() error {
	m.current = (m.current + 1) % m.windows
	return nil
}

func (m *mockWindowManager) FocusPrev() error {
	m.current--
	if m.current < 0 {
		m.current = m.windows - 1
	}
	return nil
}

func (m *mockWindowManager) FocusIndex(index int) error {
	if index < 0 || index >= m.windows {
		return errors.New("invalid index")
	}
	m.current = index
	return nil
}

func (m *mockWindowManager) Close() error {
	if m.windows <= 1 {
		return errors.New("cannot close last window")
	}
	m.windows--
	if m.current >= m.windows {
		m.current = m.windows - 1
	}
	return nil
}

func (m *mockWindowManager) CloseOthers() error {
	m.windows = 1
	m.current = 0
	return nil
}

func (m *mockWindowManager) Swap() error {
	return nil
}

func (m *mockWindowManager) Resize(deltaWidth, deltaHeight int) error {
	m.lastResize = [2]int{deltaWidth, deltaHeight}
	return nil
}

func (m *mockWindowManager) Equalize() error {
	return nil
}

func (m *mockWindowManager) Maximize() error {
	m.maximized = true
	return nil
}

func (m *mockWindowManager) MaximizeWidth() error {
	return nil
}

func (m *mockWindowManager) WindowCount() int {
	return m.windows
}

func (m *mockWindowManager) CurrentWindow() int {
	return m.current
}

func (m *mockWindowManager) Rotate(forward bool) error {
	m.rotated = true
	return nil
}

func TestHandler_Namespace(t *testing.T) {
	h := NewHandler()
	if h.Namespace() != "window" {
		t.Errorf("expected namespace 'window', got '%s'", h.Namespace())
	}
}

func TestHandler_CanHandle(t *testing.T) {
	h := NewHandler()

	validActions := []string{
		ActionSplitHorizontal,
		ActionSplitVertical,
		ActionFocusLeft,
		ActionFocusDown,
		ActionFocusUp,
		ActionFocusRight,
		ActionFocusNext,
		ActionFocusPrev,
		ActionFocusTop,
		ActionFocusBottom,
		ActionClose,
		ActionCloseOther,
		ActionSwap,
		ActionIncreaseHeight,
		ActionDecreaseHeight,
		ActionIncreaseWidth,
		ActionDecreaseWidth,
		ActionEqualize,
		ActionMaximize,
		ActionMaximizeWidth,
		ActionRotateDown,
		ActionRotateUp,
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

func TestHandler_NoWindowManager(t *testing.T) {
	h := NewHandler()
	ctx := execctx.New()

	action := input.Action{Name: ActionSplitHorizontal}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for no window manager, got %v", result.Status)
	}
}

func TestHandler_SplitHorizontal(t *testing.T) {
	wm := newMockWindowManager()
	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionSplitHorizontal}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if wm.WindowCount() != 2 {
		t.Errorf("expected 2 windows after split, got %d", wm.WindowCount())
	}
}

func TestHandler_SplitVertical(t *testing.T) {
	wm := newMockWindowManager()
	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionSplitVertical}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if wm.WindowCount() != 2 {
		t.Errorf("expected 2 windows after split, got %d", wm.WindowCount())
	}
}

func TestHandler_Focus(t *testing.T) {
	tests := []struct {
		action    string
		direction Direction
	}{
		{ActionFocusLeft, DirLeft},
		{ActionFocusRight, DirRight},
		{ActionFocusUp, DirUp},
		{ActionFocusDown, DirDown},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			wm := newMockWindowManager()
			h := NewHandlerWithManager(wm)
			ctx := execctx.New()

			action := input.Action{Name: tt.action}
			result := h.HandleAction(action, ctx)

			if result.Status != handler.StatusOK {
				t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
			}

			if wm.lastDirection != tt.direction {
				t.Errorf("expected direction %v, got %v", tt.direction, wm.lastDirection)
			}
		})
	}
}

func TestHandler_FocusNext(t *testing.T) {
	wm := newMockWindowManager()
	wm.windows = 3

	h := NewHandlerWithManager(wm)
	ctx := execctx.New()
	ctx.Count = 2

	action := input.Action{Name: ActionFocusNext}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	if wm.CurrentWindow() != 2 {
		t.Errorf("expected window 2, got %d", wm.CurrentWindow())
	}
}

func TestHandler_FocusPrev(t *testing.T) {
	wm := newMockWindowManager()
	wm.windows = 3
	wm.current = 0

	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionFocusPrev}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	// Should wrap to last window
	if wm.CurrentWindow() != 2 {
		t.Errorf("expected window 2 (wrapped), got %d", wm.CurrentWindow())
	}
}

func TestHandler_FocusTop(t *testing.T) {
	wm := newMockWindowManager()
	wm.windows = 3
	wm.current = 2

	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionFocusTop}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	if wm.CurrentWindow() != 0 {
		t.Errorf("expected window 0, got %d", wm.CurrentWindow())
	}
}

func TestHandler_FocusBottom(t *testing.T) {
	wm := newMockWindowManager()
	wm.windows = 3
	wm.current = 0

	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionFocusBottom}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	if wm.CurrentWindow() != 2 {
		t.Errorf("expected window 2, got %d", wm.CurrentWindow())
	}
}

func TestHandler_Close(t *testing.T) {
	wm := newMockWindowManager()
	wm.windows = 2

	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionClose}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if wm.WindowCount() != 1 {
		t.Errorf("expected 1 window after close, got %d", wm.WindowCount())
	}
}

func TestHandler_CloseLastWindow(t *testing.T) {
	wm := newMockWindowManager()
	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionClose}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for closing last window, got %v", result.Status)
	}

	if wm.WindowCount() != 1 {
		t.Errorf("expected 1 window still, got %d", wm.WindowCount())
	}
}

func TestHandler_CloseOthers(t *testing.T) {
	wm := newMockWindowManager()
	wm.windows = 5
	wm.current = 2

	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionCloseOther}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	if wm.WindowCount() != 1 {
		t.Errorf("expected 1 window after closeOthers, got %d", wm.WindowCount())
	}
}

func TestHandler_Swap(t *testing.T) {
	wm := newMockWindowManager()
	wm.windows = 2

	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionSwap}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
}

func TestHandler_SwapSingleWindow(t *testing.T) {
	wm := newMockWindowManager()
	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionSwap}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for single window, got %v", result.Status)
	}
}

func TestHandler_Resize(t *testing.T) {
	tests := []struct {
		action         string
		expectedWidth  int
		expectedHeight int
	}{
		{ActionIncreaseHeight, 0, 1},
		{ActionDecreaseHeight, 0, -1},
		{ActionIncreaseWidth, 1, 0},
		{ActionDecreaseWidth, -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			wm := newMockWindowManager()
			h := NewHandlerWithManager(wm)
			ctx := execctx.New()

			action := input.Action{Name: tt.action}
			result := h.HandleAction(action, ctx)

			if result.Status != handler.StatusOK {
				t.Errorf("expected StatusOK, got %v", result.Status)
			}

			if wm.lastResize[0] != tt.expectedWidth || wm.lastResize[1] != tt.expectedHeight {
				t.Errorf("expected resize [%d,%d], got [%d,%d]",
					tt.expectedWidth, tt.expectedHeight, wm.lastResize[0], wm.lastResize[1])
			}
		})
	}
}

func TestHandler_ResizeWithCount(t *testing.T) {
	wm := newMockWindowManager()
	h := NewHandlerWithManager(wm)
	ctx := execctx.New()
	ctx.Count = 5

	action := input.Action{Name: ActionIncreaseHeight}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	if wm.lastResize[1] != 5 {
		t.Errorf("expected height delta 5, got %d", wm.lastResize[1])
	}
}

func TestHandler_Equalize(t *testing.T) {
	wm := newMockWindowManager()
	wm.windows = 3

	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionEqualize}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
}

func TestHandler_Maximize(t *testing.T) {
	wm := newMockWindowManager()
	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionMaximize}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	if !wm.maximized {
		t.Error("expected window to be maximized")
	}
}

func TestHandler_Rotate(t *testing.T) {
	wm := newMockWindowManager()
	wm.windows = 3

	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionRotateDown}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	if !wm.rotated {
		t.Error("expected windows to be rotated")
	}
}

func TestHandler_RotateSingleWindow(t *testing.T) {
	wm := newMockWindowManager()
	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionRotateDown}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for single window, got %v", result.Status)
	}
}

func TestHandler_MoveToTab(t *testing.T) {
	wm := newMockWindowManager()
	h := NewHandlerWithManager(wm)
	ctx := execctx.New()

	action := input.Action{Name: ActionMoveToTab}
	result := h.HandleAction(action, ctx)

	// Should return NoOp as it's not implemented
	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for unimplemented action, got %v", result.Status)
	}
}
