package mouse

import (
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/input"
	"github.com/dshills/keystorm/internal/input/key"
)

// Button represents a mouse button.
type Button uint8

const (
	// ButtonNone indicates no button.
	ButtonNone Button = iota
	// ButtonLeft is the primary (left) mouse button.
	ButtonLeft
	// ButtonMiddle is the middle mouse button (scroll wheel click).
	ButtonMiddle
	// ButtonRight is the secondary (right) mouse button.
	ButtonRight
	// ButtonScrollUp indicates scroll wheel up.
	ButtonScrollUp
	// ButtonScrollDown indicates scroll wheel down.
	ButtonScrollDown
	// ButtonScrollLeft indicates horizontal scroll left.
	ButtonScrollLeft
	// ButtonScrollRight indicates horizontal scroll right.
	ButtonScrollRight
	// ButtonBack is the back navigation button (mouse button 4).
	ButtonBack
	// ButtonForward is the forward navigation button (mouse button 5).
	ButtonForward
)

// String returns a string representation of the button.
func (b Button) String() string {
	switch b {
	case ButtonLeft:
		return "left"
	case ButtonMiddle:
		return "middle"
	case ButtonRight:
		return "right"
	case ButtonScrollUp:
		return "scroll-up"
	case ButtonScrollDown:
		return "scroll-down"
	case ButtonScrollLeft:
		return "scroll-left"
	case ButtonScrollRight:
		return "scroll-right"
	case ButtonBack:
		return "back"
	case ButtonForward:
		return "forward"
	default:
		return "none"
	}
}

// IsScroll returns true if this is a scroll button.
func (b Button) IsScroll() bool {
	return b == ButtonScrollUp || b == ButtonScrollDown ||
		b == ButtonScrollLeft || b == ButtonScrollRight
}

// Action represents the type of mouse action.
type Action uint8

const (
	// ActionNone indicates no action.
	ActionNone Action = iota
	// ActionPress indicates a button press.
	ActionPress
	// ActionRelease indicates a button release.
	ActionRelease
	// ActionMove indicates mouse movement (no button held).
	ActionMove
	// ActionDrag indicates mouse movement with a button held.
	ActionDrag
)

// String returns a string representation of the action.
func (a Action) String() string {
	switch a {
	case ActionPress:
		return "press"
	case ActionRelease:
		return "release"
	case ActionMove:
		return "move"
	case ActionDrag:
		return "drag"
	default:
		return "none"
	}
}

// Position represents a screen coordinate.
type Position struct {
	X int
	Y int
}

// Equal returns true if two positions are equal.
func (p Position) Equal(other Position) bool {
	return p.X == other.X && p.Y == other.Y
}

// Distance returns the Manhattan distance (|dx| + |dy|) between two positions.
// Manhattan distance is used for click proximity detection as it's computationally
// efficient and provides a reasonable approximation for UI purposes.
func (p Position) Distance(other Position) int {
	dx := p.X - other.X
	if dx < 0 {
		dx = -dx
	}
	dy := p.Y - other.Y
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

// Event represents a mouse input event.
type Event struct {
	// Position is the screen coordinates.
	Position Position

	// Button is the mouse button involved.
	Button Button

	// Modifiers are any keyboard modifiers held during the event.
	Modifiers key.Modifier

	// Action is the type of mouse action.
	Action Action

	// Timestamp is when the event occurred.
	Timestamp time.Time
}

// Config configures mouse handler behavior.
type Config struct {
	// DoubleClickTime is the maximum time between clicks for a double-click.
	DoubleClickTime time.Duration

	// DoubleClickDistance is the maximum distance between clicks for a double-click.
	DoubleClickDistance int

	// ScrollLines is the number of lines to scroll per wheel tick.
	ScrollLines int

	// ScrollLinesShift is the number of lines when Shift is held.
	ScrollLinesShift int

	// EnableDragSelection enables selection via drag.
	EnableDragSelection bool

	// EnableMiddleClickPaste enables middle-click paste.
	EnableMiddleClickPaste bool

	// EnableContextMenu enables right-click context menu.
	EnableContextMenu bool

	// EnableZoom enables Ctrl+scroll zoom.
	EnableZoom bool
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		DoubleClickTime:        400 * time.Millisecond,
		DoubleClickDistance:    4,
		ScrollLines:            3,
		ScrollLinesShift:       1,
		EnableDragSelection:    true,
		EnableMiddleClickPaste: true,
		EnableContextMenu:      true,
		EnableZoom:             true,
	}
}

// Handler processes mouse events and generates editor actions.
type Handler struct {
	mu     sync.Mutex
	config Config

	// Click tracking
	click *clickTracker

	// Drag tracking
	drag *dragTracker
}

// NewHandler creates a new mouse handler with the given configuration.
func NewHandler(config Config) *Handler {
	return &Handler{
		config: config,
		click:  newClickTracker(config.DoubleClickTime, config.DoubleClickDistance),
		drag:   newDragTracker(),
	}
}

// Handle processes a mouse event and returns an action (or nil).
func (h *Handler) Handle(event Event) *input.Action {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch event.Action {
	case ActionPress:
		return h.handlePress(event)
	case ActionRelease:
		return h.handleRelease(event)
	case ActionMove:
		return h.handleMove(event)
	case ActionDrag:
		return h.handleDrag(event)
	}

	return nil
}

// handlePress handles mouse button press events.
func (h *Handler) handlePress(event Event) *input.Action {
	// Handle scroll buttons
	if event.Button.IsScroll() {
		return h.handleScroll(event)
	}

	switch event.Button {
	case ButtonLeft:
		return h.handleLeftPress(event)

	case ButtonMiddle:
		if h.config.EnableMiddleClickPaste {
			return &input.Action{
				Name:   "editor.pasteSelection",
				Source: input.SourceMouse,
				Args: input.ActionArgs{
					Extra: map[string]interface{}{
						"x": event.Position.X,
						"y": event.Position.Y,
					},
				},
			}
		}

	case ButtonRight:
		if h.config.EnableContextMenu {
			return &input.Action{
				Name:   "contextMenu.show",
				Source: input.SourceMouse,
				Args: input.ActionArgs{
					Extra: map[string]interface{}{
						"x": event.Position.X,
						"y": event.Position.Y,
					},
				},
			}
		}

	case ButtonBack:
		return &input.Action{
			Name:   "navigation.back",
			Source: input.SourceMouse,
		}

	case ButtonForward:
		return &input.Action{
			Name:   "navigation.forward",
			Source: input.SourceMouse,
		}
	}

	return nil
}

// handleLeftPress handles left mouse button press.
func (h *Handler) handleLeftPress(event Event) *input.Action {
	// Track click count for double/triple click detection
	clickCount := h.click.recordClick(event.Position, event.Timestamp)

	// Start drag tracking
	h.drag.start(event.Position, event.Button)

	// Generate action based on click count
	switch clickCount {
	case 1:
		// Single click - position cursor
		// Check for Shift+click to extend selection
		if event.Modifiers.HasShift() {
			return &input.Action{
				Name:   "selection.extendTo",
				Source: input.SourceMouse,
				Args: input.ActionArgs{
					Extra: map[string]interface{}{
						"x": event.Position.X,
						"y": event.Position.Y,
					},
				},
			}
		}
		// Check for Ctrl+click to add cursor
		if event.Modifiers.HasCtrl() || event.Modifiers.HasMeta() {
			return &input.Action{
				Name:   "cursor.add",
				Source: input.SourceMouse,
				Args: input.ActionArgs{
					Extra: map[string]interface{}{
						"x": event.Position.X,
						"y": event.Position.Y,
					},
				},
			}
		}
		return &input.Action{
			Name:   "cursor.setPosition",
			Source: input.SourceMouse,
			Args: input.ActionArgs{
				Extra: map[string]interface{}{
					"x": event.Position.X,
					"y": event.Position.Y,
				},
			},
		}

	case 2:
		// Double click - select word
		return &input.Action{
			Name:   "selection.word",
			Source: input.SourceMouse,
			Args: input.ActionArgs{
				Extra: map[string]interface{}{
					"x": event.Position.X,
					"y": event.Position.Y,
				},
			},
		}

	case 3:
		// Triple click - select line
		return &input.Action{
			Name:   "selection.line",
			Source: input.SourceMouse,
			Args: input.ActionArgs{
				Extra: map[string]interface{}{
					"x": event.Position.X,
					"y": event.Position.Y,
				},
			},
		}
	}

	return nil
}

// handleRelease handles mouse button release events.
// Design note: Actions are generated on press, not release, following common
// editor conventions. Selection is finalized during drag, so release only
// cleans up tracking state. If release-time actions are needed in the future,
// this method can be extended without breaking existing behavior.
//
//nolint:unparam // result always nil by design; return kept for future extensibility
func (h *Handler) handleRelease(_ Event) *input.Action {
	// End drag tracking
	wasSelecting := h.drag.isSelecting()
	h.drag.end()

	// If we were dragging to select, the selection is already made
	// No additional action needed on release
	if wasSelecting {
		return nil
	}

	return nil
}

// handleMove handles mouse movement (no button held).
func (h *Handler) handleMove(event Event) *input.Action {
	// Hover effects could be handled here
	// For now, movement without button doesn't generate actions
	return nil
}

// handleDrag handles mouse drag (movement with button held).
func (h *Handler) handleDrag(event Event) *input.Action {
	if !h.config.EnableDragSelection {
		return nil
	}

	// Only handle left button drag
	if h.drag.button != ButtonLeft {
		return nil
	}

	// Mark as selecting (first drag after press)
	if !h.drag.selecting {
		h.drag.selecting = true
		// Start selection at drag start position
		return &input.Action{
			Name:   "selection.start",
			Source: input.SourceMouse,
			Args: input.ActionArgs{
				Extra: map[string]interface{}{
					"x": h.drag.startPos.X,
					"y": h.drag.startPos.Y,
				},
			},
		}
	}

	// Extend selection to current position
	return &input.Action{
		Name:   "selection.extendTo",
		Source: input.SourceMouse,
		Args: input.ActionArgs{
			Extra: map[string]interface{}{
				"x": event.Position.X,
				"y": event.Position.Y,
			},
		},
	}
}

// handleScroll handles scroll wheel events.
func (h *Handler) handleScroll(event Event) *input.Action {
	// Check for zoom (Ctrl+scroll)
	if h.config.EnableZoom && (event.Modifiers.HasCtrl() || event.Modifiers.HasMeta()) {
		switch event.Button {
		case ButtonScrollUp:
			return &input.Action{
				Name:   "view.zoomIn",
				Source: input.SourceMouse,
			}
		case ButtonScrollDown:
			return &input.Action{
				Name:   "view.zoomOut",
				Source: input.SourceMouse,
			}
		}
		return nil
	}

	// Determine scroll amount
	lines := h.config.ScrollLines
	if event.Modifiers.HasShift() {
		lines = h.config.ScrollLinesShift
	}

	switch event.Button {
	case ButtonScrollUp:
		return &input.Action{
			Name:   "scroll.up",
			Source: input.SourceMouse,
			Count:  lines,
		}
	case ButtonScrollDown:
		return &input.Action{
			Name:   "scroll.down",
			Source: input.SourceMouse,
			Count:  lines,
		}
	case ButtonScrollLeft:
		return &input.Action{
			Name:   "scroll.left",
			Source: input.SourceMouse,
			Count:  lines,
		}
	case ButtonScrollRight:
		return &input.Action{
			Name:   "scroll.right",
			Source: input.SourceMouse,
			Count:  lines,
		}
	}

	return nil
}

// Reset clears all handler state.
func (h *Handler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.click.reset()
	h.drag.end()
}

// IsDragging returns true if a drag operation is in progress.
func (h *Handler) IsDragging() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.drag.active
}

// IsSelecting returns true if a selection drag is in progress.
func (h *Handler) IsSelecting() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.drag.selecting
}

// DragStart returns the starting position of the current drag (if any).
func (h *Handler) DragStart() (Position, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.drag.active {
		return Position{}, false
	}
	return h.drag.startPos, true
}
