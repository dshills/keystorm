// Package task provides task runner integration for Keystorm.
//
// This package enables automatic discovery and execution of build tasks,
// test commands, and other development workflows from various sources
// (Makefiles, package.json scripts, VS Code tasks, etc.).
//
// # Architecture
//
// The task system consists of discovery and execution components:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                    Task Discoverer                               │
//	│  - Scans workspace for task sources                             │
//	│  - Parses Makefile, package.json, tasks.json                    │
//	│  - Returns unified task definitions                              │
//	└─────────────────────────────────────────────────────────────────┘
//	                              │
//	                              ▼
//	┌─────────────────────────────────────────────────────────────────┐
//	│                    Task Executor                                 │
//	│  - Runs tasks as child processes                                │
//	│  - Streams output in real-time                                  │
//	│  - Applies problem matchers to detect errors                    │
//	└─────────────────────────────────────────────────────────────────┘
//
// # Task Sources
//
// Tasks are discovered from multiple sources:
//
//   - Makefile: GNU Make targets
//   - package.json: npm/yarn scripts
//   - tasks.json: VS Code task definitions
//   - go.mod: Go module commands (build, test, etc.)
//   - Custom: User-defined task files
//
// # Task Execution
//
// Tasks run as managed child processes with:
//
//   - Real-time stdout/stderr streaming
//   - Working directory configuration
//   - Environment variable support
//   - Cancellation support
//   - Exit code tracking
//
// # Problem Matchers
//
// Problem matchers parse task output to extract errors and warnings:
//
//	// Define a problem matcher
//	matcher := task.ProblemMatcher{
//	    Pattern: `^(.+):(\d+):(\d+): (error|warning): (.+)$`,
//	    File:    1,
//	    Line:    2,
//	    Column:  3,
//	    Severity: 4,
//	    Message: 5,
//	}
//
// Built-in matchers are provided for common tools (Go, TypeScript, ESLint, etc.).
//
// # Usage
//
// Discover and run tasks:
//
//	// Create discoverer with sources
//	discoverer := task.NewDiscoverer(
//	    task.WithSource(sources.NewMakefileSource()),
//	    task.WithSource(sources.NewPackageJSONSource()),
//	)
//
//	// Discover tasks in workspace
//	tasks, err := discoverer.DiscoverTasks(ctx, "/path/to/workspace")
//
//	// Create executor
//	executor := task.NewExecutor()
//
//	// Run a task
//	execution, err := executor.Execute(ctx, tasks[0])
//
//	// Stream output
//	for line := range execution.OutputChan() {
//	    fmt.Println(line.Content)
//	}
//
//	// Check result
//	if execution.ExitCode != 0 {
//	    // Handle failure
//	}
//
// # Execution States
//
// Task executions transition through states:
//
//   - Pending: Task is queued
//   - Running: Task is executing
//   - Completed: Task finished successfully
//   - Failed: Task finished with non-zero exit
//   - Cancelled: Task was cancelled
//
// # Subpackages
//
//   - sources: Task source implementations (Makefile, package.json, etc.)
package task
