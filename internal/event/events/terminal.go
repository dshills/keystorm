package events

import (
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Terminal event topics.
const (
	// TopicTerminalCreated is published when a terminal session starts.
	TopicTerminalCreated topic.Topic = "terminal.created"

	// TopicTerminalClosed is published when a terminal session ends.
	TopicTerminalClosed topic.Topic = "terminal.closed"

	// TopicTerminalOutput is published when terminal output is available.
	TopicTerminalOutput topic.Topic = "terminal.output"

	// TopicTerminalInput is published when input is sent to terminal.
	TopicTerminalInput topic.Topic = "terminal.input"

	// TopicTerminalExited is published when terminal process exits.
	TopicTerminalExited topic.Topic = "terminal.exited"

	// TopicTerminalResized is published when terminal is resized.
	TopicTerminalResized topic.Topic = "terminal.resized"

	// TopicTerminalTitleChanged is published when terminal title changes.
	TopicTerminalTitleChanged topic.Topic = "terminal.title.changed"

	// TopicTerminalCwdChanged is published when terminal working directory changes.
	TopicTerminalCwdChanged topic.Topic = "terminal.cwd.changed"

	// TopicTerminalBell is published when terminal bell rings.
	TopicTerminalBell topic.Topic = "terminal.bell"

	// TopicTerminalFocused is published when terminal gains focus.
	TopicTerminalFocused topic.Topic = "terminal.focused"

	// TopicTerminalBlurred is published when terminal loses focus.
	TopicTerminalBlurred topic.Topic = "terminal.blurred"
)

// TerminalCreated is published when a terminal session starts.
type TerminalCreated struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string

	// Shell is the shell being used (e.g., "bash", "zsh").
	Shell string

	// Cwd is the initial working directory.
	Cwd string

	// Rows is the number of rows.
	Rows int

	// Cols is the number of columns.
	Cols int

	// Profile is the terminal profile name.
	Profile string

	// Env contains custom environment variables.
	Env map[string]string
}

// TerminalClosed is published when a terminal session ends.
type TerminalClosed struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string

	// Reason explains why the terminal was closed.
	Reason string
}

// TerminalOutput is published when terminal output is available.
type TerminalOutput struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string

	// Output is the output text.
	Output string

	// Timestamp is when the output was received.
	Timestamp time.Time

	// IsStderr indicates if output is from stderr.
	IsStderr bool
}

// TerminalInput is published when input is sent to terminal.
type TerminalInput struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string

	// Input is the input text.
	Input string

	// Timestamp is when the input was sent.
	Timestamp time.Time
}

// TerminalExited is published when terminal process exits.
type TerminalExited struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string

	// ExitCode is the process exit code.
	ExitCode int

	// Signal is the signal that terminated the process, if any.
	Signal string

	// Duration is how long the terminal session lasted.
	Duration time.Duration
}

// TerminalResized is published when terminal is resized.
type TerminalResized struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string

	// OldRows was the previous number of rows.
	OldRows int

	// OldCols was the previous number of columns.
	OldCols int

	// NewRows is the new number of rows.
	NewRows int

	// NewCols is the new number of columns.
	NewCols int
}

// TerminalTitleChanged is published when terminal title changes.
type TerminalTitleChanged struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string

	// OldTitle was the previous title.
	OldTitle string

	// NewTitle is the new title.
	NewTitle string
}

// TerminalCwdChanged is published when terminal working directory changes.
type TerminalCwdChanged struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string

	// OldCwd was the previous working directory.
	OldCwd string

	// NewCwd is the new working directory.
	NewCwd string
}

// TerminalBell is published when terminal bell rings.
type TerminalBell struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string

	// Timestamp is when the bell rang.
	Timestamp time.Time
}

// TerminalFocused is published when terminal gains focus.
type TerminalFocused struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string
}

// TerminalBlurred is published when terminal loses focus.
type TerminalBlurred struct {
	// TerminalID is the unique terminal identifier.
	TerminalID string
}
