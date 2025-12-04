package mouse

// dragTracker tracks mouse drag state.
type dragTracker struct {
	// active indicates a drag is in progress.
	active bool

	// selecting indicates the drag is creating a selection.
	selecting bool

	// button is the mouse button being held.
	button Button

	// startPos is where the drag started.
	startPos Position

	// currentPos is the current drag position.
	currentPos Position
}

// newDragTracker creates a new drag tracker.
func newDragTracker() *dragTracker {
	return &dragTracker{}
}

// start begins a new drag operation.
func (t *dragTracker) start(pos Position, button Button) {
	t.active = true
	t.selecting = false
	t.button = button
	t.startPos = pos
	t.currentPos = pos
}

// update updates the current drag position.
func (t *dragTracker) update(pos Position) {
	if t.active {
		t.currentPos = pos
	}
}

// end ends the current drag operation.
func (t *dragTracker) end() {
	t.active = false
	t.selecting = false
	t.button = ButtonNone
	t.startPos = Position{}
	t.currentPos = Position{}
}

// isActive returns true if a drag is in progress.
func (t *dragTracker) isActive() bool {
	return t.active
}

// isSelecting returns true if the drag is creating a selection.
func (t *dragTracker) isSelecting() bool {
	return t.selecting
}

// startSelection marks the drag as creating a selection.
func (t *dragTracker) startSelection() {
	if t.active {
		t.selecting = true
	}
}

// getButton returns the button being held during the drag.
func (t *dragTracker) getButton() Button {
	return t.button
}

// getStartPos returns the drag start position.
func (t *dragTracker) getStartPos() Position {
	return t.startPos
}

// getCurrentPos returns the current drag position.
func (t *dragTracker) getCurrentPos() Position {
	return t.currentPos
}

// getDelta returns the distance dragged from start.
func (t *dragTracker) getDelta() Position {
	return Position{
		X: t.currentPos.X - t.startPos.X,
		Y: t.currentPos.Y - t.startPos.Y,
	}
}

// DragState represents the current state of a drag operation.
type DragState struct {
	// Active indicates a drag is in progress.
	Active bool

	// Selecting indicates the drag is creating a selection.
	Selecting bool

	// Button is the mouse button being held.
	Button Button

	// StartPos is where the drag started.
	StartPos Position

	// CurrentPos is the current drag position.
	CurrentPos Position
}

// GetState returns the current drag state.
func (t *dragTracker) GetState() DragState {
	return DragState{
		Active:     t.active,
		Selecting:  t.selecting,
		Button:     t.button,
		StartPos:   t.startPos,
		CurrentPos: t.currentPos,
	}
}
