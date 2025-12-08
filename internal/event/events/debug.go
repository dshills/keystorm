package events

import (
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Debug event topics.
const (
	// TopicDebugSessionStarted is published when a debug session starts.
	TopicDebugSessionStarted topic.Topic = "debug.session.started"

	// TopicDebugSessionStopped is published when a debug session ends.
	TopicDebugSessionStopped topic.Topic = "debug.session.stopped"

	// TopicDebugSessionPaused is published when execution is paused.
	TopicDebugSessionPaused topic.Topic = "debug.session.paused"

	// TopicDebugSessionResumed is published when execution resumes.
	TopicDebugSessionResumed topic.Topic = "debug.session.resumed"

	// TopicDebugBreakpointHit is published when a breakpoint is triggered.
	TopicDebugBreakpointHit topic.Topic = "debug.breakpoint.hit"

	// TopicDebugBreakpointAdded is published when a breakpoint is set.
	TopicDebugBreakpointAdded topic.Topic = "debug.breakpoint.added"

	// TopicDebugBreakpointRemoved is published when a breakpoint is cleared.
	TopicDebugBreakpointRemoved topic.Topic = "debug.breakpoint.removed"

	// TopicDebugBreakpointChanged is published when a breakpoint is modified.
	TopicDebugBreakpointChanged topic.Topic = "debug.breakpoint.changed"

	// TopicDebugStepCompleted is published when a step operation finishes.
	TopicDebugStepCompleted topic.Topic = "debug.step.completed"

	// TopicDebugVariablesUpdated is published when variables change.
	TopicDebugVariablesUpdated topic.Topic = "debug.variables.updated"

	// TopicDebugCallStackUpdated is published when call stack changes.
	TopicDebugCallStackUpdated topic.Topic = "debug.callstack.updated"

	// TopicDebugOutputReceived is published when debug output is received.
	TopicDebugOutputReceived topic.Topic = "debug.output.received"

	// TopicDebugExceptionThrown is published when an exception occurs.
	TopicDebugExceptionThrown topic.Topic = "debug.exception.thrown"

	// TopicDebugThreadStarted is published when a new thread starts.
	TopicDebugThreadStarted topic.Topic = "debug.thread.started"

	// TopicDebugThreadExited is published when a thread exits.
	TopicDebugThreadExited topic.Topic = "debug.thread.exited"

	// TopicDebugModuleLoaded is published when a module is loaded.
	TopicDebugModuleLoaded topic.Topic = "debug.module.loaded"

	// TopicDebugWatchEvaluated is published when a watch expression is evaluated.
	TopicDebugWatchEvaluated topic.Topic = "debug.watch.evaluated"
)

// DebugStopReason describes why the debugger stopped.
type DebugStopReason string

// Debug stop reasons.
const (
	DebugStopReasonBreakpoint DebugStopReason = "breakpoint"
	DebugStopReasonStep       DebugStopReason = "step"
	DebugStopReasonPause      DebugStopReason = "pause"
	DebugStopReasonException  DebugStopReason = "exception"
	DebugStopReasonEntry      DebugStopReason = "entry"
	DebugStopReasonExit       DebugStopReason = "exit"
)

// DebugStepType describes the type of step operation.
type DebugStepType string

// Debug step types.
const (
	DebugStepOver DebugStepType = "over"
	DebugStepInto DebugStepType = "into"
	DebugStepOut  DebugStepType = "out"
)

// DebugBreakpointType describes the type of breakpoint.
type DebugBreakpointType string

// Breakpoint types.
const (
	BreakpointTypeLine      DebugBreakpointType = "line"
	BreakpointTypeFunction  DebugBreakpointType = "function"
	BreakpointTypeException DebugBreakpointType = "exception"
	BreakpointTypeData      DebugBreakpointType = "data"
	BreakpointTypeLog       DebugBreakpointType = "log"
)

// DebugBreakpoint represents a breakpoint.
type DebugBreakpoint struct {
	// ID is the breakpoint ID.
	ID string

	// Type is the breakpoint type.
	Type DebugBreakpointType

	// File is the source file.
	File string

	// Line is the line number.
	Line int

	// Column is the column number, if applicable.
	Column int

	// Condition is the conditional expression.
	Condition string

	// HitCondition specifies when to break (e.g., "> 10").
	HitCondition string

	// LogMessage is the message to log (for logpoints).
	LogMessage string

	// HitCount is the number of times this breakpoint was hit.
	HitCount int

	// IsEnabled indicates if the breakpoint is enabled.
	IsEnabled bool

	// IsVerified indicates if the breakpoint was verified by the debugger.
	IsVerified bool
}

// DebugVariable represents a variable in the debugger.
type DebugVariable struct {
	// Name is the variable name.
	Name string

	// Value is the display value.
	Value string

	// Type is the variable type.
	Type string

	// VariablesReference indicates if this has child variables.
	VariablesReference int

	// MemoryReference is the memory address, if applicable.
	MemoryReference string

	// EvaluateName is the expression to evaluate this variable.
	EvaluateName string

	// IsIndexed indicates if children are indexed.
	IsIndexed bool

	// ChildCount is the number of children.
	ChildCount int
}

// DebugStackFrame represents a stack frame.
type DebugStackFrame struct {
	// ID is the frame ID.
	ID int

	// Name is the function name.
	Name string

	// File is the source file.
	File string

	// Line is the line number.
	Line int

	// Column is the column number.
	Column int

	// ModuleName is the module/package name.
	ModuleName string

	// CanRestart indicates if this frame can be restarted.
	CanRestart bool
}

// DebugThread represents a thread in the debugger.
type DebugThread struct {
	// ID is the thread ID.
	ID int

	// Name is the thread name.
	Name string

	// IsStopped indicates if the thread is stopped.
	IsStopped bool

	// StopReason is why the thread stopped.
	StopReason DebugStopReason
}

// DebugSessionStarted is published when a debug session starts.
type DebugSessionStarted struct {
	// SessionID is the unique session identifier.
	SessionID string

	// Adapter is the debug adapter name (e.g., "delve", "debugpy").
	Adapter string

	// AdapterVersion is the adapter version.
	AdapterVersion string

	// TargetProcess is the target being debugged.
	TargetProcess string

	// WorkingDirectory is the working directory.
	WorkingDirectory string

	// Args are the program arguments.
	Args []string

	// IsAttach indicates if attached to a running process.
	IsAttach bool
}

// DebugSessionStopped is published when a debug session ends.
type DebugSessionStopped struct {
	// SessionID is the unique session identifier.
	SessionID string

	// ExitCode is the process exit code.
	ExitCode int

	// Reason explains why the session ended.
	Reason string

	// Duration is how long the session lasted.
	Duration time.Duration
}

// DebugSessionPaused is published when execution is paused.
type DebugSessionPaused struct {
	// SessionID is the unique session identifier.
	SessionID string

	// ThreadID is the thread that stopped.
	ThreadID int

	// Reason is why execution paused.
	Reason DebugStopReason

	// Description provides additional details.
	Description string

	// File is the current source file.
	File string

	// Line is the current line number.
	Line int

	// Column is the current column number.
	Column int

	// AllThreadsStopped indicates if all threads stopped.
	AllThreadsStopped bool
}

// DebugSessionResumed is published when execution resumes.
type DebugSessionResumed struct {
	// SessionID is the unique session identifier.
	SessionID string

	// ThreadID is the thread that resumed, or 0 for all.
	ThreadID int

	// AllThreadsContinued indicates if all threads continued.
	AllThreadsContinued bool
}

// DebugBreakpointHit is published when a breakpoint is triggered.
type DebugBreakpointHit struct {
	// SessionID is the unique session identifier.
	SessionID string

	// BreakpointID is the breakpoint that was hit.
	BreakpointID string

	// File is the source file.
	File string

	// Line is the line number.
	Line int

	// ThreadID is the thread that hit the breakpoint.
	ThreadID int

	// HitCount is the total hit count for this breakpoint.
	HitCount int
}

// DebugBreakpointAdded is published when a breakpoint is set.
type DebugBreakpointAdded struct {
	// SessionID is the unique session identifier.
	SessionID string

	// Breakpoint is the added breakpoint.
	Breakpoint DebugBreakpoint
}

// DebugBreakpointRemoved is published when a breakpoint is cleared.
type DebugBreakpointRemoved struct {
	// SessionID is the unique session identifier.
	SessionID string

	// BreakpointID is the removed breakpoint ID.
	BreakpointID string

	// File was the source file.
	File string

	// Line was the line number.
	Line int
}

// DebugBreakpointChanged is published when a breakpoint is modified.
type DebugBreakpointChanged struct {
	// SessionID is the unique session identifier.
	SessionID string

	// Breakpoint is the modified breakpoint.
	Breakpoint DebugBreakpoint

	// Changes describes what changed.
	Changes []string
}

// DebugStepCompleted is published when a step operation finishes.
type DebugStepCompleted struct {
	// SessionID is the unique session identifier.
	SessionID string

	// StepType is the type of step performed.
	StepType DebugStepType

	// File is the new source file.
	File string

	// Line is the new line number.
	Line int

	// FunctionName is the current function.
	FunctionName string
}

// DebugVariablesUpdated is published when variables change.
type DebugVariablesUpdated struct {
	// SessionID is the unique session identifier.
	SessionID string

	// Scope is the variable scope (e.g., "Locals", "Globals").
	Scope string

	// Variables are the updated variables.
	Variables []DebugVariable

	// FrameID is the stack frame ID.
	FrameID int
}

// DebugCallStackUpdated is published when call stack changes.
type DebugCallStackUpdated struct {
	// SessionID is the unique session identifier.
	SessionID string

	// ThreadID is the thread ID.
	ThreadID int

	// Frames are the stack frames.
	Frames []DebugStackFrame

	// TotalFrames is the total number of frames.
	TotalFrames int
}

// DebugOutputReceived is published when debug output is received.
type DebugOutputReceived struct {
	// SessionID is the unique session identifier.
	SessionID string

	// Category is the output category (e.g., "console", "stdout", "stderr").
	Category string

	// Output is the output text.
	Output string

	// File is the source file, if applicable.
	File string

	// Line is the line number, if applicable.
	Line int

	// Timestamp is when the output was received.
	Timestamp time.Time
}

// DebugExceptionThrown is published when an exception occurs.
type DebugExceptionThrown struct {
	// SessionID is the unique session identifier.
	SessionID string

	// ExceptionID identifies the exception.
	ExceptionID string

	// Description describes the exception.
	Description string

	// Details provides additional details.
	Details string

	// File is the source file where the exception occurred.
	File string

	// Line is the line number.
	Line int

	// Breakmode indicates the break behavior.
	Breakmode string

	// CanContinue indicates if execution can continue.
	CanContinue bool
}

// DebugThreadStarted is published when a new thread starts.
type DebugThreadStarted struct {
	// SessionID is the unique session identifier.
	SessionID string

	// Thread is the new thread.
	Thread DebugThread
}

// DebugThreadExited is published when a thread exits.
type DebugThreadExited struct {
	// SessionID is the unique session identifier.
	SessionID string

	// ThreadID is the exited thread ID.
	ThreadID int

	// ExitCode is the thread exit code.
	ExitCode int
}

// DebugModuleLoaded is published when a module is loaded.
type DebugModuleLoaded struct {
	// SessionID is the unique session identifier.
	SessionID string

	// ModuleID identifies the module.
	ModuleID string

	// Name is the module name.
	Name string

	// Path is the module path.
	Path string

	// Version is the module version.
	Version string

	// IsUserCode indicates if this is user code.
	IsUserCode bool
}

// DebugWatchEvaluated is published when a watch expression is evaluated.
type DebugWatchEvaluated struct {
	// SessionID is the unique session identifier.
	SessionID string

	// Expression is the evaluated expression.
	Expression string

	// Result is the evaluation result.
	Result DebugVariable

	// FrameID is the stack frame used for evaluation.
	FrameID int

	// Error is the error message if evaluation failed.
	Error string
}
