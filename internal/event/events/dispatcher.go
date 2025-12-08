package events

import (
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Dispatcher event topics.
const (
	// TopicDispatcherActionDispatched is published when an action is sent to handler.
	TopicDispatcherActionDispatched topic.Topic = "dispatcher.action.dispatched"

	// TopicDispatcherActionExecuted is published when handler completes.
	TopicDispatcherActionExecuted topic.Topic = "dispatcher.action.executed"

	// TopicDispatcherActionFailed is published when handler raises error.
	TopicDispatcherActionFailed topic.Topic = "dispatcher.action.failed"

	// TopicDispatcherActionCancelled is published when action is cancelled.
	TopicDispatcherActionCancelled topic.Topic = "dispatcher.action.cancelled"

	// TopicDispatcherModeChanged is published when mode is switched by handler.
	TopicDispatcherModeChanged topic.Topic = "dispatcher.mode.changed"

	// TopicDispatcherViewUpdateRequested is published when view needs update.
	TopicDispatcherViewUpdateRequested topic.Topic = "dispatcher.view.update.requested"

	// TopicDispatcherUndoRedoPerformed is published when undo/redo is performed.
	TopicDispatcherUndoRedoPerformed topic.Topic = "dispatcher.undo.redo.performed"

	// TopicDispatcherRepeatRequested is published when action repeat is requested.
	TopicDispatcherRepeatRequested topic.Topic = "dispatcher.repeat.requested"

	// TopicDispatcherActionRegistered is published when an action is registered.
	TopicDispatcherActionRegistered topic.Topic = "dispatcher.action.registered"

	// TopicDispatcherActionUnregistered is published when an action is unregistered.
	TopicDispatcherActionUnregistered topic.Topic = "dispatcher.action.unregistered"

	// TopicDispatcherQueueUpdated is published when the action queue changes.
	TopicDispatcherQueueUpdated topic.Topic = "dispatcher.queue.updated"
)

// ActionExecutionStatus represents the status of action execution.
type ActionExecutionStatus string

// Action execution statuses.
const (
	ActionStatusSuccess   ActionExecutionStatus = "success"
	ActionStatusError     ActionExecutionStatus = "error"
	ActionStatusCancelled ActionExecutionStatus = "cancelled"
	ActionStatusSkipped   ActionExecutionStatus = "skipped"
)

// ActionContext contains the context in which an action was executed.
type ActionContext struct {
	// Mode is the editor mode.
	Mode string

	// BufferID is the active buffer.
	BufferID string

	// FilePath is the active file path.
	FilePath string

	// CursorPosition is the cursor position.
	CursorPosition Position

	// HasSelection indicates if there's a selection.
	HasSelection bool

	// VisualMode indicates the visual selection mode.
	VisualMode string
}

// DispatcherActionDispatched is published when an action is sent to handler.
type DispatcherActionDispatched struct {
	// ActionID is a unique identifier for this action execution.
	ActionID string

	// ActionName is the action name.
	ActionName string

	// Count is the repeat count.
	Count int

	// Args contains action arguments.
	Args map[string]any

	// Context is the execution context.
	Context ActionContext

	// Timestamp is when the action was dispatched.
	Timestamp time.Time

	// Source describes where the action came from.
	Source string
}

// DispatcherActionExecuted is published when handler completes.
type DispatcherActionExecuted struct {
	// ActionID is the unique action execution identifier.
	ActionID string

	// ActionName is the action name.
	ActionName string

	// Duration is how long execution took.
	Duration time.Duration

	// Status is the execution status.
	Status ActionExecutionStatus

	// Context was the execution context.
	Context ActionContext

	// Result contains any result data.
	Result map[string]any

	// ModifiedBuffers lists buffers that were modified.
	ModifiedBuffers []string
}

// DispatcherActionFailed is published when handler raises error.
type DispatcherActionFailed struct {
	// ActionID is the unique action execution identifier.
	ActionID string

	// ActionName is the action name.
	ActionName string

	// ErrorMessage describes the error.
	ErrorMessage string

	// ErrorCode is the error code, if applicable.
	ErrorCode string

	// Duration is how long execution took before failing.
	Duration time.Duration

	// Context was the execution context.
	Context ActionContext

	// CanRetry indicates if the action can be retried.
	CanRetry bool
}

// DispatcherActionCancelled is published when action is cancelled.
type DispatcherActionCancelled struct {
	// ActionID is the unique action execution identifier.
	ActionID string

	// ActionName is the action name.
	ActionName string

	// Reason explains why the action was cancelled.
	Reason string

	// Duration is how long the action ran before cancellation.
	Duration time.Duration

	// Context was the execution context.
	Context ActionContext
}

// DispatcherModeChanged is published when mode is switched by handler.
type DispatcherModeChanged struct {
	// NewMode is the new mode.
	NewMode string

	// PreviousMode was the previous mode.
	PreviousMode string

	// Trigger is what caused the mode change.
	Trigger string

	// ActionID is the action that caused the change, if any.
	ActionID string

	// BufferID is the active buffer.
	BufferID string
}

// DispatcherViewUpdateRequested is published when view needs update.
type DispatcherViewUpdateRequested struct {
	// BufferID is the buffer to update.
	BufferID string

	// Redraw indicates if a full redraw is needed.
	Redraw bool

	// ScrollTo is the position to scroll to.
	ScrollTo *Position

	// CenterLine is the line to center on.
	CenterLine *int

	// RevealCursor indicates if cursor should be revealed.
	RevealCursor bool

	// HighlightRanges are ranges to highlight.
	HighlightRanges []Range

	// ActionID is the action that requested the update.
	ActionID string
}

// DispatcherUndoRedoPerformed is published when undo/redo is performed.
type DispatcherUndoRedoPerformed struct {
	// BufferID is the buffer where undo/redo was performed.
	BufferID string

	// IsUndo is true for undo, false for redo.
	IsUndo bool

	// ChangesReverted is the number of changes reverted.
	ChangesReverted int

	// NewRevisionID is the revision after undo/redo.
	NewRevisionID string

	// OldRevisionID was the revision before undo/redo.
	OldRevisionID string

	// CursorPosition is the cursor position after undo/redo.
	CursorPosition Position

	// ActionID is the action that performed undo/redo.
	ActionID string
}

// DispatcherRepeatRequested is published when action repeat is requested.
type DispatcherRepeatRequested struct {
	// ActionName is the action to repeat.
	ActionName string

	// Count is the repeat count.
	Count int

	// OriginalCount was the original count of the action.
	OriginalCount int

	// Args are the original arguments.
	Args map[string]any
}

// DispatcherActionRegistered is published when an action is registered.
type DispatcherActionRegistered struct {
	// ActionName is the registered action name.
	ActionName string

	// Description describes the action.
	Description string

	// Source identifies who registered the action.
	Source string

	// DefaultBinding is the default key binding.
	DefaultBinding string

	// Modes lists the modes where the action is available.
	Modes []string

	// IsOverride indicates if this overrides an existing action.
	IsOverride bool
}

// DispatcherActionUnregistered is published when an action is unregistered.
type DispatcherActionUnregistered struct {
	// ActionName is the unregistered action name.
	ActionName string

	// Source identifies who unregistered the action.
	Source string
}

// DispatcherQueueUpdated is published when the action queue changes.
type DispatcherQueueUpdated struct {
	// QueuedCount is the number of actions waiting.
	QueuedCount int

	// CurrentAction is the currently executing action.
	CurrentAction string

	// QueuedActions lists queued action names.
	QueuedActions []string
}
