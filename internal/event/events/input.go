package events

import (
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Input event topics.
const (
	// TopicInputKeystroke is published when a key is pressed.
	TopicInputKeystroke topic.Topic = "input.keystroke"

	// TopicInputSequenceResolved is published when a key sequence matches an action.
	TopicInputSequenceResolved topic.Topic = "input.sequence.resolved"

	// TopicInputSequencePending is published when waiting for more keys.
	TopicInputSequencePending topic.Topic = "input.sequence.pending"

	// TopicInputSequenceAborted is published when a key sequence is aborted.
	TopicInputSequenceAborted topic.Topic = "input.sequence.aborted"

	// TopicInputModeChanged is published when the editor mode changes.
	TopicInputModeChanged topic.Topic = "input.mode.changed"

	// TopicInputMacroStarted is published when macro recording starts.
	TopicInputMacroStarted topic.Topic = "input.macro.started"

	// TopicInputMacroStopped is published when macro recording stops.
	TopicInputMacroStopped topic.Topic = "input.macro.stopped"

	// TopicInputMacroPlayed is published when a macro is played.
	TopicInputMacroPlayed topic.Topic = "input.macro.played"

	// TopicInputMouseClicked is published when a mouse button is clicked.
	TopicInputMouseClicked topic.Topic = "input.mouse.clicked"

	// TopicInputMouseDragged is published when a mouse drag occurs.
	TopicInputMouseDragged topic.Topic = "input.mouse.dragged"

	// TopicInputMouseScrolled is published when the mouse wheel scrolls.
	TopicInputMouseScrolled topic.Topic = "input.mouse.scrolled"
)

// Modifier represents keyboard modifiers.
type Modifier string

// Keyboard modifiers.
const (
	ModifierCtrl  Modifier = "ctrl"
	ModifierShift Modifier = "shift"
	ModifierAlt   Modifier = "alt"
	ModifierMeta  Modifier = "meta" // Cmd on macOS, Win on Windows
	ModifierSuper Modifier = "super"
)

// MouseButton represents a mouse button.
type MouseButton string

// Mouse buttons.
const (
	MouseButtonLeft    MouseButton = "left"
	MouseButtonRight   MouseButton = "right"
	MouseButtonMiddle  MouseButton = "middle"
	MouseButtonBack    MouseButton = "back"
	MouseButtonForward MouseButton = "forward"
)

// InputKeystroke is published when a key is pressed.
type InputKeystroke struct {
	// Key is the key that was pressed (e.g., "a", "Enter", "Escape").
	Key string

	// Modifiers are the active modifier keys.
	Modifiers []Modifier

	// Timestamp is when the key was pressed.
	Timestamp time.Time

	// Mode is the current editor mode.
	Mode string

	// IsRepeat indicates if this is a key repeat event.
	IsRepeat bool

	// Raw is the raw key code, if available.
	Raw int
}

// InputSequenceResolved is published when a key sequence matches an action.
type InputSequenceResolved struct {
	// Sequence is the complete key sequence (e.g., "dd", "ciw").
	Sequence string

	// Action is the action name that was matched.
	Action string

	// Mode is the editor mode when the sequence was resolved.
	Mode string

	// Count is the repeat count (e.g., "3dd" has count 3).
	Count int

	// Register is the register name, if specified (e.g., "\"add).
	Register string

	// Args contains additional arguments parsed from the sequence.
	Args map[string]any
}

// InputSequencePending is published when waiting for more keys.
type InputSequencePending struct {
	// PendingSequence is the partial key sequence so far.
	PendingSequence string

	// Mode is the current editor mode.
	Mode string

	// PossibleActions lists actions that could match.
	PossibleActions []string

	// Timeout is when the pending sequence will expire.
	Timeout time.Time
}

// InputSequenceAborted is published when a key sequence is aborted.
type InputSequenceAborted struct {
	// Sequence is the partial sequence that was aborted.
	Sequence string

	// Mode is the current editor mode.
	Mode string

	// Reason explains why the sequence was aborted.
	Reason string
}

// InputModeChanged is published when the editor mode changes.
type InputModeChanged struct {
	// PreviousMode is the mode before the change.
	PreviousMode string

	// CurrentMode is the new mode.
	CurrentMode string

	// Trigger describes what caused the mode change.
	Trigger string
}

// InputMacroStarted is published when macro recording starts.
type InputMacroStarted struct {
	// Register is the register where the macro will be stored.
	Register string

	// Mode is the editor mode when recording started.
	Mode string
}

// InputMacroStopped is published when macro recording stops.
type InputMacroStopped struct {
	// Register is the register where the macro was stored.
	Register string

	// KeysRecorded is the number of keys recorded.
	KeysRecorded int

	// Duration is how long the recording lasted.
	Duration time.Duration
}

// InputMacroPlayed is published when a macro is played.
type InputMacroPlayed struct {
	// Register is the register containing the macro.
	Register string

	// Count is how many times the macro was played.
	Count int

	// KeysExecuted is the total number of keys executed.
	KeysExecuted int

	// Duration is how long playback took.
	Duration time.Duration
}

// InputMouseClicked is published when a mouse button is clicked.
type InputMouseClicked struct {
	// Button is the mouse button that was clicked.
	Button MouseButton

	// Position is where the click occurred (buffer coordinates).
	Position Position

	// ScreenX is the x coordinate in screen/terminal cells.
	ScreenX int

	// ScreenY is the y coordinate in screen/terminal cells.
	ScreenY int

	// Modifiers are the active modifier keys.
	Modifiers []Modifier

	// ClickCount is 1 for single click, 2 for double, etc.
	ClickCount int

	// Timestamp is when the click occurred.
	Timestamp time.Time
}

// InputMouseDragged is published when a mouse drag occurs.
type InputMouseDragged struct {
	// Button is the mouse button being held.
	Button MouseButton

	// StartPosition is where the drag started.
	StartPosition Position

	// CurrentPosition is the current drag position.
	CurrentPosition Position

	// Modifiers are the active modifier keys.
	Modifiers []Modifier
}

// InputMouseScrolled is published when the mouse wheel scrolls.
type InputMouseScrolled struct {
	// DeltaX is the horizontal scroll amount.
	DeltaX int

	// DeltaY is the vertical scroll amount.
	DeltaY int

	// Position is the mouse position when scrolling.
	Position Position

	// Modifiers are the active modifier keys.
	Modifiers []Modifier
}
