package mouse

import (
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/input/key"
)

func TestButtonString(t *testing.T) {
	tests := []struct {
		button   Button
		expected string
	}{
		{ButtonNone, "none"},
		{ButtonLeft, "left"},
		{ButtonMiddle, "middle"},
		{ButtonRight, "right"},
		{ButtonScrollUp, "scroll-up"},
		{ButtonScrollDown, "scroll-down"},
		{ButtonScrollLeft, "scroll-left"},
		{ButtonScrollRight, "scroll-right"},
		{ButtonBack, "back"},
		{ButtonForward, "forward"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.button.String(); got != tt.expected {
				t.Errorf("Button.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestButtonIsScroll(t *testing.T) {
	scrollButtons := []Button{ButtonScrollUp, ButtonScrollDown, ButtonScrollLeft, ButtonScrollRight}
	nonScrollButtons := []Button{ButtonNone, ButtonLeft, ButtonMiddle, ButtonRight, ButtonBack, ButtonForward}

	for _, b := range scrollButtons {
		if !b.IsScroll() {
			t.Errorf("%s.IsScroll() = false, want true", b)
		}
	}

	for _, b := range nonScrollButtons {
		if b.IsScroll() {
			t.Errorf("%s.IsScroll() = true, want false", b)
		}
	}
}

func TestActionString(t *testing.T) {
	tests := []struct {
		action   Action
		expected string
	}{
		{ActionNone, "none"},
		{ActionPress, "press"},
		{ActionRelease, "release"},
		{ActionMove, "move"},
		{ActionDrag, "drag"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.action.String(); got != tt.expected {
				t.Errorf("Action.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPositionEqual(t *testing.T) {
	p1 := Position{X: 10, Y: 20}
	p2 := Position{X: 10, Y: 20}
	p3 := Position{X: 15, Y: 20}

	if !p1.Equal(p2) {
		t.Error("Equal positions not detected as equal")
	}

	if p1.Equal(p3) {
		t.Error("Different positions detected as equal")
	}
}

func TestPositionDistance(t *testing.T) {
	tests := []struct {
		p1, p2   Position
		expected int
	}{
		{Position{0, 0}, Position{0, 0}, 0},
		{Position{0, 0}, Position{3, 4}, 7},   // Manhattan distance
		{Position{5, 5}, Position{2, 1}, 7},   // 3 + 4
		{Position{-1, -1}, Position{1, 1}, 4}, // 2 + 2
	}

	for _, tt := range tests {
		got := tt.p1.Distance(tt.p2)
		if got != tt.expected {
			t.Errorf("Distance(%v, %v) = %d, want %d", tt.p1, tt.p2, got, tt.expected)
		}
	}
}

func TestClickTrackerSingleClick(t *testing.T) {
	tracker := newClickTracker(400*time.Millisecond, 4)

	pos := Position{X: 100, Y: 100}
	now := time.Now()

	count := tracker.recordClick(pos, now)
	if count != 1 {
		t.Errorf("First click count = %d, want 1", count)
	}
}

func TestClickTrackerDoubleClick(t *testing.T) {
	tracker := newClickTracker(400*time.Millisecond, 4)

	pos := Position{X: 100, Y: 100}
	now := time.Now()

	tracker.recordClick(pos, now)
	count := tracker.recordClick(pos, now.Add(100*time.Millisecond))

	if count != 2 {
		t.Errorf("Double click count = %d, want 2", count)
	}
}

func TestClickTrackerTripleClick(t *testing.T) {
	tracker := newClickTracker(400*time.Millisecond, 4)

	pos := Position{X: 100, Y: 100}
	now := time.Now()

	tracker.recordClick(pos, now)
	tracker.recordClick(pos, now.Add(100*time.Millisecond))
	count := tracker.recordClick(pos, now.Add(200*time.Millisecond))

	if count != 3 {
		t.Errorf("Triple click count = %d, want 3", count)
	}
}

func TestClickTrackerQuadClickWraps(t *testing.T) {
	tracker := newClickTracker(400*time.Millisecond, 4)

	pos := Position{X: 100, Y: 100}
	now := time.Now()

	tracker.recordClick(pos, now)
	tracker.recordClick(pos, now.Add(100*time.Millisecond))
	tracker.recordClick(pos, now.Add(200*time.Millisecond))
	count := tracker.recordClick(pos, now.Add(300*time.Millisecond))

	if count != 1 {
		t.Errorf("Quad click count = %d, want 1 (wrapped)", count)
	}
}

func TestClickTrackerTimeoutResets(t *testing.T) {
	tracker := newClickTracker(400*time.Millisecond, 4)

	pos := Position{X: 100, Y: 100}
	now := time.Now()

	tracker.recordClick(pos, now)
	// Wait longer than double-click timeout
	count := tracker.recordClick(pos, now.Add(500*time.Millisecond))

	if count != 1 {
		t.Errorf("Click after timeout = %d, want 1", count)
	}
}

func TestClickTrackerDistanceResets(t *testing.T) {
	tracker := newClickTracker(400*time.Millisecond, 4)

	now := time.Now()

	tracker.recordClick(Position{X: 100, Y: 100}, now)
	// Click far away
	count := tracker.recordClick(Position{X: 200, Y: 200}, now.Add(100*time.Millisecond))

	if count != 1 {
		t.Errorf("Click at different position = %d, want 1", count)
	}
}

func TestClickTrackerZeroTimestamp(t *testing.T) {
	tracker := newClickTracker(400*time.Millisecond, 4)

	pos := Position{X: 100, Y: 100}

	// Zero timestamp should be handled gracefully
	count := tracker.recordClick(pos, time.Time{})
	if count != 1 {
		t.Errorf("First click with zero timestamp = %d, want 1", count)
	}

	// Second click with valid timestamp should not count as double-click
	// because the first used fallback time.Now() which is far from this fixed time
	count = tracker.recordClick(pos, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if count != 1 {
		t.Errorf("Click after zero timestamp = %d, want 1 (new sequence)", count)
	}
}

func TestClickTrackerClockSkew(t *testing.T) {
	tracker := newClickTracker(400*time.Millisecond, 4)

	pos := Position{X: 100, Y: 100}
	now := time.Now()

	// First click
	tracker.recordClick(pos, now)

	// Second click with earlier timestamp (clock skew)
	count := tracker.recordClick(pos, now.Add(-100*time.Millisecond))
	if count != 1 {
		t.Errorf("Click with negative elapsed time = %d, want 1 (clock skew)", count)
	}
}

func TestDragTracker(t *testing.T) {
	tracker := newDragTracker()

	if tracker.isActive() {
		t.Error("New tracker should not be active")
	}

	// Start drag
	startPos := Position{X: 100, Y: 100}
	tracker.start(startPos, ButtonLeft)

	if !tracker.isActive() {
		t.Error("Tracker should be active after start")
	}

	if tracker.isSelecting() {
		t.Error("Tracker should not be selecting until marked")
	}

	if tracker.getButton() != ButtonLeft {
		t.Errorf("Button = %v, want ButtonLeft", tracker.getButton())
	}

	if tracker.getStartPos() != startPos {
		t.Errorf("Start position = %v, want %v", tracker.getStartPos(), startPos)
	}

	// Update position
	newPos := Position{X: 150, Y: 120}
	tracker.update(newPos)

	if tracker.getCurrentPos() != newPos {
		t.Errorf("Current position = %v, want %v", tracker.getCurrentPos(), newPos)
	}

	// Check delta
	delta := tracker.getDelta()
	if delta.X != 50 || delta.Y != 20 {
		t.Errorf("Delta = %v, want {50, 20}", delta)
	}

	// Start selection
	tracker.startSelection()
	if !tracker.isSelecting() {
		t.Error("Tracker should be selecting after startSelection")
	}

	// End drag
	tracker.end()
	if tracker.isActive() {
		t.Error("Tracker should not be active after end")
	}
}

func TestHandlerSingleClick(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	event := Event{
		Position:  Position{X: 100, Y: 50},
		Button:    ButtonLeft,
		Modifiers: key.ModNone,
		Action:    ActionPress,
		Timestamp: time.Now(),
	}

	action := handler.Handle(event)
	if action == nil {
		t.Fatal("Expected action for left click")
	}

	if action.Name != "cursor.setPosition" {
		t.Errorf("Action name = %q, want %q", action.Name, "cursor.setPosition")
	}

	x := action.Args.GetInt("x")
	y := action.Args.GetInt("y")
	if x != 100 || y != 50 {
		t.Errorf("Position = (%d, %d), want (100, 50)", x, y)
	}
}

func TestHandlerDoubleClick(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	now := time.Now()
	pos := Position{X: 100, Y: 50}

	// First click
	handler.Handle(Event{
		Position:  pos,
		Button:    ButtonLeft,
		Action:    ActionPress,
		Timestamp: now,
	})

	// Second click (double-click)
	action := handler.Handle(Event{
		Position:  pos,
		Button:    ButtonLeft,
		Action:    ActionPress,
		Timestamp: now.Add(100 * time.Millisecond),
	})

	if action == nil {
		t.Fatal("Expected action for double click")
	}

	if action.Name != "selection.word" {
		t.Errorf("Action name = %q, want %q", action.Name, "selection.word")
	}
}

func TestHandlerTripleClick(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	now := time.Now()
	pos := Position{X: 100, Y: 50}

	// First two clicks
	handler.Handle(Event{Position: pos, Button: ButtonLeft, Action: ActionPress, Timestamp: now})
	handler.Handle(Event{Position: pos, Button: ButtonLeft, Action: ActionPress, Timestamp: now.Add(100 * time.Millisecond)})

	// Third click (triple-click)
	action := handler.Handle(Event{
		Position:  pos,
		Button:    ButtonLeft,
		Action:    ActionPress,
		Timestamp: now.Add(200 * time.Millisecond),
	})

	if action == nil {
		t.Fatal("Expected action for triple click")
	}

	if action.Name != "selection.line" {
		t.Errorf("Action name = %q, want %q", action.Name, "selection.line")
	}
}

func TestHandlerShiftClick(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	event := Event{
		Position:  Position{X: 100, Y: 50},
		Button:    ButtonLeft,
		Modifiers: key.ModShift,
		Action:    ActionPress,
		Timestamp: time.Now(),
	}

	action := handler.Handle(event)
	if action == nil {
		t.Fatal("Expected action for shift+click")
	}

	if action.Name != "selection.extendTo" {
		t.Errorf("Action name = %q, want %q", action.Name, "selection.extendTo")
	}
}

func TestHandlerCtrlClick(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	event := Event{
		Position:  Position{X: 100, Y: 50},
		Button:    ButtonLeft,
		Modifiers: key.ModCtrl,
		Action:    ActionPress,
		Timestamp: time.Now(),
	}

	action := handler.Handle(event)
	if action == nil {
		t.Fatal("Expected action for ctrl+click")
	}

	if action.Name != "cursor.add" {
		t.Errorf("Action name = %q, want %q", action.Name, "cursor.add")
	}
}

func TestHandlerMiddleClick(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	event := Event{
		Position: Position{X: 100, Y: 50},
		Button:   ButtonMiddle,
		Action:   ActionPress,
	}

	action := handler.Handle(event)
	if action == nil {
		t.Fatal("Expected action for middle click")
	}

	if action.Name != "editor.pasteSelection" {
		t.Errorf("Action name = %q, want %q", action.Name, "editor.pasteSelection")
	}
}

func TestHandlerRightClick(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	event := Event{
		Position: Position{X: 100, Y: 50},
		Button:   ButtonRight,
		Action:   ActionPress,
	}

	action := handler.Handle(event)
	if action == nil {
		t.Fatal("Expected action for right click")
	}

	if action.Name != "contextMenu.show" {
		t.Errorf("Action name = %q, want %q", action.Name, "contextMenu.show")
	}
}

func TestHandlerScrollUp(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	event := Event{
		Position: Position{X: 100, Y: 50},
		Button:   ButtonScrollUp,
		Action:   ActionPress,
	}

	action := handler.Handle(event)
	if action == nil {
		t.Fatal("Expected action for scroll up")
	}

	if action.Name != "scroll.up" {
		t.Errorf("Action name = %q, want %q", action.Name, "scroll.up")
	}

	if action.Count != 3 { // Default scroll lines
		t.Errorf("Scroll count = %d, want 3", action.Count)
	}
}

func TestHandlerScrollWithShift(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	event := Event{
		Position:  Position{X: 100, Y: 50},
		Button:    ButtonScrollDown,
		Modifiers: key.ModShift,
		Action:    ActionPress,
	}

	action := handler.Handle(event)
	if action == nil {
		t.Fatal("Expected action for scroll with shift")
	}

	if action.Count != 1 { // Single line with shift
		t.Errorf("Scroll count = %d, want 1", action.Count)
	}
}

func TestHandlerCtrlScrollZoom(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	// Ctrl+scroll up = zoom in
	event := Event{
		Position:  Position{X: 100, Y: 50},
		Button:    ButtonScrollUp,
		Modifiers: key.ModCtrl,
		Action:    ActionPress,
	}

	action := handler.Handle(event)
	if action == nil {
		t.Fatal("Expected action for ctrl+scroll")
	}

	if action.Name != "view.zoomIn" {
		t.Errorf("Action name = %q, want %q", action.Name, "view.zoomIn")
	}

	// Ctrl+scroll down = zoom out
	event.Button = ButtonScrollDown
	action = handler.Handle(event)

	if action.Name != "view.zoomOut" {
		t.Errorf("Action name = %q, want %q", action.Name, "view.zoomOut")
	}
}

func TestHandlerDrag(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	now := time.Now()
	startPos := Position{X: 100, Y: 50}

	// Press to start drag
	handler.Handle(Event{
		Position:  startPos,
		Button:    ButtonLeft,
		Action:    ActionPress,
		Timestamp: now,
	})

	// First drag event starts selection
	action := handler.Handle(Event{
		Position: Position{X: 150, Y: 60},
		Button:   ButtonLeft,
		Action:   ActionDrag,
	})

	if action == nil {
		t.Fatal("Expected action for first drag")
	}

	if action.Name != "selection.start" {
		t.Errorf("First drag action = %q, want %q", action.Name, "selection.start")
	}

	// Second drag extends selection
	action = handler.Handle(Event{
		Position: Position{X: 200, Y: 70},
		Button:   ButtonLeft,
		Action:   ActionDrag,
	})

	if action == nil {
		t.Fatal("Expected action for continued drag")
	}

	if action.Name != "selection.extendTo" {
		t.Errorf("Continue drag action = %q, want %q", action.Name, "selection.extendTo")
	}
}

func TestHandlerBackForwardButtons(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	// Back button
	action := handler.Handle(Event{Button: ButtonBack, Action: ActionPress})
	if action == nil || action.Name != "navigation.back" {
		t.Errorf("Back button action = %v, want navigation.back", action)
	}

	// Forward button
	action = handler.Handle(Event{Button: ButtonForward, Action: ActionPress})
	if action == nil || action.Name != "navigation.forward" {
		t.Errorf("Forward button action = %v, want navigation.forward", action)
	}
}

func TestHandlerReset(t *testing.T) {
	handler := NewHandler(DefaultConfig())

	// Start a drag
	handler.Handle(Event{
		Position: Position{X: 100, Y: 50},
		Button:   ButtonLeft,
		Action:   ActionPress,
	})

	if !handler.IsDragging() {
		t.Error("Should be dragging after press")
	}

	handler.Reset()

	if handler.IsDragging() {
		t.Error("Should not be dragging after reset")
	}
}

func TestScrollDirectionString(t *testing.T) {
	tests := []struct {
		dir      ScrollDirection
		expected string
	}{
		{ScrollNone, "none"},
		{ScrollUp, "up"},
		{ScrollDown, "down"},
		{ScrollLeft, "left"},
		{ScrollRight, "right"},
	}

	for _, tt := range tests {
		if got := tt.dir.String(); got != tt.expected {
			t.Errorf("%v.String() = %q, want %q", tt.dir, got, tt.expected)
		}
	}
}

func TestParseScrollEvent(t *testing.T) {
	config := DefaultConfig()

	// Normal scroll
	event := Event{
		Position: Position{X: 100, Y: 50},
		Button:   ButtonScrollUp,
		Action:   ActionPress,
	}

	scroll := ParseScrollEvent(event, config)
	if scroll == nil {
		t.Fatal("Expected scroll event")
	}

	if scroll.Direction != ScrollUp {
		t.Errorf("Direction = %v, want ScrollUp", scroll.Direction)
	}

	if scroll.Lines != config.ScrollLines {
		t.Errorf("Lines = %d, want %d", scroll.Lines, config.ScrollLines)
	}

	if scroll.IsZoom {
		t.Error("Should not be zoom")
	}
}

func TestParseScrollEventZoom(t *testing.T) {
	config := DefaultConfig()

	event := Event{
		Position:  Position{X: 100, Y: 50},
		Button:    ButtonScrollUp,
		Modifiers: key.ModCtrl,
		Action:    ActionPress,
	}

	scroll := ParseScrollEvent(event, config)
	if scroll == nil {
		t.Fatal("Expected scroll event")
	}

	if !scroll.IsZoom {
		t.Error("Should be zoom")
	}

	if !scroll.ZoomIn {
		t.Error("Scroll up should be zoom in")
	}
}

func TestScrollEventToAction(t *testing.T) {
	// Scroll action
	scroll := &ScrollEvent{
		Direction: ScrollDown,
		Lines:     5,
	}

	action := scroll.ToAction()
	if action.Name != "scroll.down" {
		t.Errorf("Action name = %q, want %q", action.Name, "scroll.down")
	}
	if action.Count != 5 {
		t.Errorf("Count = %d, want 5", action.Count)
	}

	// Zoom action
	zoom := &ScrollEvent{
		IsZoom: true,
		ZoomIn: true,
	}

	action = zoom.ToAction()
	if action.Name != "view.zoomIn" {
		t.Errorf("Zoom action = %q, want %q", action.Name, "view.zoomIn")
	}
}

func TestClickTypeString(t *testing.T) {
	tests := []struct {
		ct       ClickType
		expected string
	}{
		{ClickSingle, "single"},
		{ClickDouble, "double"},
		{ClickTriple, "triple"},
		{ClickType(0), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.expected {
			t.Errorf("%v.String() = %q, want %q", tt.ct, got, tt.expected)
		}
	}
}

func TestDragState(t *testing.T) {
	tracker := newDragTracker()

	state := tracker.GetState()
	if state.Active {
		t.Error("Initial state should not be active")
	}

	tracker.start(Position{X: 10, Y: 20}, ButtonLeft)
	tracker.update(Position{X: 30, Y: 40})
	tracker.startSelection()

	state = tracker.GetState()
	if !state.Active {
		t.Error("State should be active")
	}
	if !state.Selecting {
		t.Error("State should be selecting")
	}
	if state.Button != ButtonLeft {
		t.Errorf("Button = %v, want ButtonLeft", state.Button)
	}
	if state.StartPos.X != 10 || state.StartPos.Y != 20 {
		t.Errorf("StartPos = %v, want {10, 20}", state.StartPos)
	}
	if state.CurrentPos.X != 30 || state.CurrentPos.Y != 40 {
		t.Errorf("CurrentPos = %v, want {30, 40}", state.CurrentPos)
	}
}

func TestHandlerDisabledFeatures(t *testing.T) {
	config := DefaultConfig()
	config.EnableMiddleClickPaste = false
	config.EnableContextMenu = false
	config.EnableDragSelection = false
	config.EnableZoom = false

	handler := NewHandler(config)

	// Middle click should not generate action
	action := handler.Handle(Event{Button: ButtonMiddle, Action: ActionPress})
	if action != nil {
		t.Error("Middle click should be disabled")
	}

	// Right click should not generate action
	action = handler.Handle(Event{Button: ButtonRight, Action: ActionPress})
	if action != nil {
		t.Error("Right click context menu should be disabled")
	}

	// Ctrl+scroll should not zoom
	action = handler.Handle(Event{
		Button:    ButtonScrollUp,
		Modifiers: key.ModCtrl,
		Action:    ActionPress,
	})
	if action != nil && action.Name == "view.zoomIn" {
		t.Error("Zoom should be disabled")
	}

	// Drag should not create selection
	handler.Handle(Event{Position: Position{X: 100, Y: 50}, Button: ButtonLeft, Action: ActionPress})
	action = handler.Handle(Event{Position: Position{X: 150, Y: 60}, Button: ButtonLeft, Action: ActionDrag})
	if action != nil {
		t.Error("Drag selection should be disabled")
	}
}

func TestScrollEventIsHorizontalVertical(t *testing.T) {
	vertical := &ScrollEvent{Direction: ScrollUp}
	if !vertical.IsVertical() {
		t.Error("ScrollUp should be vertical")
	}
	if vertical.IsHorizontal() {
		t.Error("ScrollUp should not be horizontal")
	}

	horizontal := &ScrollEvent{Direction: ScrollLeft}
	if !horizontal.IsHorizontal() {
		t.Error("ScrollLeft should be horizontal")
	}
	if horizontal.IsVertical() {
		t.Error("ScrollLeft should not be vertical")
	}
}

// Benchmarks

func BenchmarkHandlerClick(b *testing.B) {
	handler := NewHandler(DefaultConfig())
	event := Event{
		Position: Position{X: 100, Y: 50},
		Button:   ButtonLeft,
		Action:   ActionPress,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.Handle(event)
	}
}

func BenchmarkClickTrackerDoubleClick(b *testing.B) {
	tracker := newClickTracker(400*time.Millisecond, 4)
	pos := Position{X: 100, Y: 100}
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.recordClick(pos, now)
		tracker.recordClick(pos, now.Add(100*time.Millisecond))
		tracker.reset()
	}
}

func BenchmarkHandlerScroll(b *testing.B) {
	handler := NewHandler(DefaultConfig())
	event := Event{
		Button: ButtonScrollUp,
		Action: ActionPress,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.Handle(event)
	}
}
