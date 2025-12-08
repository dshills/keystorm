package events

import (
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Task event topics.
const (
	// TopicTaskDiscovered is published when tasks are scanned.
	TopicTaskDiscovered topic.Topic = "task.discovered"

	// TopicTaskStarted is published when task execution begins.
	TopicTaskStarted topic.Topic = "task.started"

	// TopicTaskOutput is published when task produces output.
	TopicTaskOutput topic.Topic = "task.output"

	// TopicTaskCompleted is published when task execution finishes.
	TopicTaskCompleted topic.Topic = "task.completed"

	// TopicTaskProblemFound is published when a problem matcher detects an issue.
	TopicTaskProblemFound topic.Topic = "task.problem.found"

	// TopicTaskCancelled is published when a task is cancelled.
	TopicTaskCancelled topic.Topic = "task.cancelled"

	// TopicTaskFailed is published when a task fails.
	TopicTaskFailed topic.Topic = "task.failed"

	// TopicTaskQueueUpdated is published when the task queue changes.
	TopicTaskQueueUpdated topic.Topic = "task.queue.updated"

	// TopicTaskDependencyResolved is published when a dependency is resolved.
	TopicTaskDependencyResolved topic.Topic = "task.dependency.resolved"
)

// TaskSource indicates where a task definition came from.
type TaskSource string

// Task sources.
const (
	TaskSourceMakefile    TaskSource = "makefile"
	TaskSourcePackageJSON TaskSource = "package.json"
	TaskSourceTaskfile    TaskSource = "taskfile"
	TaskSourceGoMod       TaskSource = "go.mod"
	TaskSourceKeystorm    TaskSource = "keystorm"
	TaskSourceCustom      TaskSource = "custom"
)

// TaskProblemSeverity indicates the severity of a task problem.
type TaskProblemSeverity string

// Task problem severities.
const (
	TaskProblemError   TaskProblemSeverity = "error"
	TaskProblemWarning TaskProblemSeverity = "warning"
	TaskProblemInfo    TaskProblemSeverity = "info"
)

// TaskDefinition represents a task definition.
type TaskDefinition struct {
	// Name is the task name.
	Name string

	// Description describes the task.
	Description string

	// Source is where the task was defined.
	Source TaskSource

	// Command is the command to run.
	Command string

	// Args are the command arguments.
	Args []string

	// Cwd is the working directory.
	Cwd string

	// Env contains environment variables.
	Env map[string]string

	// DependsOn lists task dependencies.
	DependsOn []string

	// Group categorizes the task.
	Group string

	// IsDefault indicates if this is the default task.
	IsDefault bool

	// ProblemMatcher is the problem matcher pattern.
	ProblemMatcher string
}

// TaskProblem represents a problem found during task execution.
type TaskProblem struct {
	// File is the source file.
	File string

	// Line is the line number.
	Line int

	// Column is the column number.
	Column int

	// EndLine is the end line for range problems.
	EndLine int

	// EndColumn is the end column for range problems.
	EndColumn int

	// Message describes the problem.
	Message string

	// Severity indicates the problem severity.
	Severity TaskProblemSeverity

	// Code is the error/warning code.
	Code string

	// Source identifies what produced this problem.
	Source string
}

// TaskDiscovered is published when tasks are scanned.
type TaskDiscovered struct {
	// Tasks are the discovered task definitions.
	Tasks []TaskDefinition

	// TaskCount is the number of tasks discovered.
	TaskCount int

	// Sources lists where tasks were found.
	Sources []TaskSource

	// Duration is how long discovery took.
	Duration time.Duration
}

// TaskStarted is published when task execution begins.
type TaskStarted struct {
	// TaskID is a unique identifier for this execution.
	TaskID string

	// TaskName is the task name.
	TaskName string

	// Source is where the task was defined.
	Source TaskSource

	// Command is the command being run.
	Command string

	// Args are the command arguments.
	Args []string

	// Cwd is the working directory.
	Cwd string

	// StartTime is when execution started.
	StartTime time.Time

	// ParentTaskID is the parent task ID for dependency chains.
	ParentTaskID string
}

// TaskOutput is published when task produces output.
type TaskOutput struct {
	// TaskID is the unique task execution identifier.
	TaskID string

	// TaskName is the task name.
	TaskName string

	// Output is the output text.
	Output string

	// IsStderr indicates if output is from stderr.
	IsStderr bool

	// Timestamp is when the output was produced.
	Timestamp time.Time

	// LineNumber is the line number in the task output.
	LineNumber int
}

// TaskCompleted is published when task execution finishes.
type TaskCompleted struct {
	// TaskID is the unique task execution identifier.
	TaskID string

	// TaskName is the task name.
	TaskName string

	// ExitCode is the process exit code.
	ExitCode int

	// Duration is how long the task ran.
	Duration time.Duration

	// StartTime is when execution started.
	StartTime time.Time

	// EndTime is when execution ended.
	EndTime time.Time

	// ProblemsFound is the number of problems detected.
	ProblemsFound int

	// OutputLines is the total number of output lines.
	OutputLines int
}

// TaskProblemFound is published when a problem matcher detects an issue.
type TaskProblemFound struct {
	// TaskID is the unique task execution identifier.
	TaskID string

	// TaskName is the task name.
	TaskName string

	// Problem is the detected problem.
	Problem TaskProblem

	// RawOutput is the original output line.
	RawOutput string

	// MatcherName identifies the problem matcher.
	MatcherName string
}

// TaskCancelled is published when a task is cancelled.
type TaskCancelled struct {
	// TaskID is the unique task execution identifier.
	TaskID string

	// TaskName is the task name.
	TaskName string

	// Reason explains why the task was cancelled.
	Reason string

	// Duration is how long the task ran before cancellation.
	Duration time.Duration

	// WasKilled indicates if the process was killed.
	WasKilled bool
}

// TaskFailed is published when a task fails.
type TaskFailed struct {
	// TaskID is the unique task execution identifier.
	TaskID string

	// TaskName is the task name.
	TaskName string

	// ExitCode is the non-zero exit code.
	ExitCode int

	// ErrorMessage describes the failure.
	ErrorMessage string

	// Duration is how long the task ran.
	Duration time.Duration

	// ProblemsFound is the number of problems detected.
	ProblemsFound int
}

// TaskQueueUpdated is published when the task queue changes.
type TaskQueueUpdated struct {
	// QueuedCount is the number of tasks waiting.
	QueuedCount int

	// RunningCount is the number of tasks running.
	RunningCount int

	// QueuedTasks lists queued task names.
	QueuedTasks []string

	// RunningTasks lists running task names.
	RunningTasks []string
}

// TaskDependencyResolved is published when a dependency is resolved.
type TaskDependencyResolved struct {
	// TaskID is the dependent task execution identifier.
	TaskID string

	// TaskName is the dependent task name.
	TaskName string

	// DependencyName is the resolved dependency name.
	DependencyName string

	// DependencyTaskID is the dependency's execution ID.
	DependencyTaskID string

	// Success indicates if the dependency succeeded.
	Success bool

	// RemainingDependencies is the count of remaining dependencies.
	RemainingDependencies int
}
