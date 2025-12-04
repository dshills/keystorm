// Package window provides handlers for window and split operations.
package window

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for window operations.
const (
	// Split operations
	ActionSplitHorizontal = "window.splitHorizontal" // Ctrl+W s, :sp
	ActionSplitVertical   = "window.splitVertical"   // Ctrl+W v, :vsp

	// Navigation
	ActionFocusLeft   = "window.focusLeft"   // Ctrl+W h
	ActionFocusDown   = "window.focusDown"   // Ctrl+W j
	ActionFocusUp     = "window.focusUp"     // Ctrl+W k
	ActionFocusRight  = "window.focusRight"  // Ctrl+W l
	ActionFocusNext   = "window.focusNext"   // Ctrl+W w
	ActionFocusPrev   = "window.focusPrev"   // Ctrl+W W
	ActionFocusTop    = "window.focusTop"    // Ctrl+W t
	ActionFocusBottom = "window.focusBottom" // Ctrl+W b

	// Window management
	ActionClose      = "window.close"      // Ctrl+W c, :close
	ActionCloseOther = "window.closeOther" // Ctrl+W o, :only
	ActionSwap       = "window.swap"       // Ctrl+W x

	// Resizing
	ActionIncreaseHeight = "window.increaseHeight" // Ctrl+W +
	ActionDecreaseHeight = "window.decreaseHeight" // Ctrl+W -
	ActionIncreaseWidth  = "window.increaseWidth"  // Ctrl+W >
	ActionDecreaseWidth  = "window.decreaseWidth"  // Ctrl+W <
	ActionEqualize       = "window.equalize"       // Ctrl+W =
	ActionMaximize       = "window.maximize"       // Ctrl+W _
	ActionMaximizeWidth  = "window.maximizeWidth"  // Ctrl+W |

	// Layout
	ActionRotateDown = "window.rotateDown" // Ctrl+W r
	ActionRotateUp   = "window.rotateUp"   // Ctrl+W R
	ActionMoveToTab  = "window.moveToTab"  // Ctrl+W T
)

// Direction represents window navigation direction.
type Direction int

const (
	DirLeft Direction = iota
	DirRight
	DirUp
	DirDown
)

// WindowManager provides window management operations.
// This interface is implemented by the window/layout system.
type WindowManager interface {
	// SplitHorizontal creates a horizontal split.
	SplitHorizontal() error
	// SplitVertical creates a vertical split.
	SplitVertical() error
	// Focus moves focus in the specified direction.
	Focus(dir Direction) error
	// FocusNext moves focus to the next window.
	FocusNext() error
	// FocusPrev moves focus to the previous window.
	FocusPrev() error
	// FocusIndex moves focus to a specific window index.
	FocusIndex(index int) error
	// Close closes the current window.
	Close() error
	// CloseOthers closes all windows except the current one.
	CloseOthers() error
	// Swap swaps with the next window.
	Swap() error
	// Resize resizes the current window.
	Resize(deltaWidth, deltaHeight int) error
	// Equalize equalizes all window sizes.
	Equalize() error
	// Maximize maximizes the current window.
	Maximize() error
	// MaximizeWidth maximizes the current window width.
	MaximizeWidth() error
	// WindowCount returns the number of windows.
	WindowCount() int
	// CurrentWindow returns the current window index.
	CurrentWindow() int
	// Rotate rotates window positions.
	Rotate(forward bool) error
}

const windowManagerKey = "_window_manager"

// Handler implements namespace-based window handling.
type Handler struct {
	windowManager WindowManager
}

// NewHandler creates a new window handler.
func NewHandler() *Handler {
	return &Handler{}
}

// NewHandlerWithManager creates a handler with a window manager.
func NewHandlerWithManager(wm WindowManager) *Handler {
	return &Handler{windowManager: wm}
}

// Namespace returns the window namespace.
func (h *Handler) Namespace() string {
	return "window"
}

// CanHandle returns true if this handler can process the action.
func (h *Handler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionSplitHorizontal, ActionSplitVertical,
		ActionFocusLeft, ActionFocusDown, ActionFocusUp, ActionFocusRight,
		ActionFocusNext, ActionFocusPrev, ActionFocusTop, ActionFocusBottom,
		ActionClose, ActionCloseOther, ActionSwap,
		ActionIncreaseHeight, ActionDecreaseHeight, ActionIncreaseWidth, ActionDecreaseWidth,
		ActionEqualize, ActionMaximize, ActionMaximizeWidth,
		ActionRotateDown, ActionRotateUp, ActionMoveToTab:
		return true
	}
	return false
}

// HandleAction processes a window action.
func (h *Handler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	wm := h.getWindowManager(ctx)
	if wm == nil {
		return handler.NoOpWithMessage("window: no window manager")
	}

	count := ctx.GetCount()

	switch action.Name {
	case ActionSplitHorizontal:
		return h.splitHorizontal(wm)
	case ActionSplitVertical:
		return h.splitVertical(wm)
	case ActionFocusLeft:
		return h.focus(wm, DirLeft)
	case ActionFocusDown:
		return h.focus(wm, DirDown)
	case ActionFocusUp:
		return h.focus(wm, DirUp)
	case ActionFocusRight:
		return h.focus(wm, DirRight)
	case ActionFocusNext:
		return h.focusNext(wm, count)
	case ActionFocusPrev:
		return h.focusPrev(wm, count)
	case ActionFocusTop:
		return h.focusIndex(wm, 0)
	case ActionFocusBottom:
		return h.focusIndex(wm, wm.WindowCount()-1)
	case ActionClose:
		return h.close(wm)
	case ActionCloseOther:
		return h.closeOthers(wm)
	case ActionSwap:
		return h.swap(wm)
	case ActionIncreaseHeight:
		return h.resize(wm, 0, count)
	case ActionDecreaseHeight:
		return h.resize(wm, 0, -count)
	case ActionIncreaseWidth:
		return h.resize(wm, count, 0)
	case ActionDecreaseWidth:
		return h.resize(wm, -count, 0)
	case ActionEqualize:
		return h.equalize(wm)
	case ActionMaximize:
		return h.maximize(wm)
	case ActionMaximizeWidth:
		return h.maximizeWidth(wm)
	case ActionRotateDown:
		return h.rotate(wm, true)
	case ActionRotateUp:
		return h.rotate(wm, false)
	case ActionMoveToTab:
		return handler.NoOpWithMessage("window.moveToTab: not implemented")
	default:
		return handler.Errorf("unknown window action: %s", action.Name)
	}
}

// getWindowManager returns the window manager from handler or context.
func (h *Handler) getWindowManager(ctx *execctx.ExecutionContext) WindowManager {
	if h.windowManager != nil {
		return h.windowManager
	}
	if v, ok := ctx.GetData(windowManagerKey); ok {
		if wm, ok := v.(WindowManager); ok {
			return wm
		}
	}
	return nil
}

// splitHorizontal creates a horizontal split.
func (h *Handler) splitHorizontal(wm WindowManager) handler.Result {
	if err := wm.SplitHorizontal(); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// splitVertical creates a vertical split.
func (h *Handler) splitVertical(wm WindowManager) handler.Result {
	if err := wm.SplitVertical(); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// focus moves focus in a direction.
func (h *Handler) focus(wm WindowManager, dir Direction) handler.Result {
	if err := wm.Focus(dir); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// focusNext moves focus to the next window.
func (h *Handler) focusNext(wm WindowManager, count int) handler.Result {
	for i := 0; i < count; i++ {
		if err := wm.FocusNext(); err != nil {
			return handler.Error(err)
		}
	}
	return handler.Success().WithRedraw()
}

// focusPrev moves focus to the previous window.
func (h *Handler) focusPrev(wm WindowManager, count int) handler.Result {
	for i := 0; i < count; i++ {
		if err := wm.FocusPrev(); err != nil {
			return handler.Error(err)
		}
	}
	return handler.Success().WithRedraw()
}

// focusIndex moves focus to a specific window.
func (h *Handler) focusIndex(wm WindowManager, index int) handler.Result {
	if index < 0 || index >= wm.WindowCount() {
		return handler.NoOp()
	}
	if err := wm.FocusIndex(index); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// close closes the current window.
func (h *Handler) close(wm WindowManager) handler.Result {
	if wm.WindowCount() <= 1 {
		return handler.NoOpWithMessage("window: cannot close last window")
	}
	if err := wm.Close(); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// closeOthers closes all other windows.
func (h *Handler) closeOthers(wm WindowManager) handler.Result {
	if err := wm.CloseOthers(); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// swap swaps with the next window.
func (h *Handler) swap(wm WindowManager) handler.Result {
	if wm.WindowCount() <= 1 {
		return handler.NoOp()
	}
	if err := wm.Swap(); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// resize resizes the current window.
func (h *Handler) resize(wm WindowManager, deltaWidth, deltaHeight int) handler.Result {
	if err := wm.Resize(deltaWidth, deltaHeight); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// equalize equalizes all window sizes.
func (h *Handler) equalize(wm WindowManager) handler.Result {
	if err := wm.Equalize(); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// maximize maximizes the current window.
func (h *Handler) maximize(wm WindowManager) handler.Result {
	if err := wm.Maximize(); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// maximizeWidth maximizes the current window width.
func (h *Handler) maximizeWidth(wm WindowManager) handler.Result {
	if err := wm.MaximizeWidth(); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}

// rotate rotates window positions.
func (h *Handler) rotate(wm WindowManager, forward bool) handler.Result {
	if wm.WindowCount() <= 1 {
		return handler.NoOp()
	}
	if err := wm.Rotate(forward); err != nil {
		return handler.Error(err)
	}
	return handler.Success().WithRedraw()
}
