# Keystorm Integration Layer - Implementation Plan

## Comprehensive Design Document for `internal/integration`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Requirements Analysis](#2-requirements-analysis)
3. [Architecture Overview](#3-architecture-overview)
4. [Package Structure](#4-package-structure)
5. [Core Types and Interfaces](#5-core-types-and-interfaces)
6. [Terminal Integration](#6-terminal-integration)
7. [Git Integration](#7-git-integration)
8. [Debugger Integration](#8-debugger-integration)
9. [Task Runner Integration](#9-task-runner-integration)
10. [Integration with Editor](#10-integration-with-editor)
11. [Implementation Phases](#11-implementation-phases)
12. [Testing Strategy](#12-testing-strategy)
13. [Performance Considerations](#13-performance-considerations)

---

## 1. Executive Summary

The Integration Layer transforms Keystorm from "just an editor" into a "mini IDE" by providing seamless integration with external tools: Terminal, Git, Debugger, and Task Runner. This layer abstracts the complexity of interacting with these tools while exposing clean, consistent APIs to the rest of the editor.

### Role in the Architecture

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   Dispatcher    │─────▶│  Integration    │◀────▶│  External       │
│  (git, debug,   │      │     Layer       │      │  Tools          │
│   task, term)   │      │  (internal/int) │      │  (git, dap,     │
└─────────────────┘      └────────┬────────┘      │   shell, make)  │
                                  │               └─────────────────┘
                    ┌─────────────┼─────────────┐
                    ▼             ▼             ▼
            ┌───────────┐  ┌───────────┐  ┌───────────┐
            │  Plugin   │  │  Renderer │  │   Event   │
            │   API     │  │ (output)  │  │    Bus    │
            └───────────┘  └───────────┘  └───────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| PTY-based terminal | Full terminal emulation with ANSI escape codes, shell job control |
| libgit2/go-git for Git | Native Git operations without spawning processes for common ops |
| DAP (Debug Adapter Protocol) | Standard protocol for debugger integration across languages |
| Task definitions in TOML | Consistent with project config, supports Make/npm/go as backends |
| Async operations with channels | Non-blocking UI, proper cancellation support |
| Process supervision | Lifecycle management, resource cleanup, signal handling |

### Integration Points

The Integration Layer connects to:
- **Dispatcher**: Receives integration actions (git.commit, debug.step, task.run, terminal.send)
- **Renderer**: Provides output for terminal, debug console, task output panels
- **Event Bus**: Publishes integration events (git.status.changed, debug.breakpoint.hit)
- **Config System**: Reads terminal, git, debugger, and task settings
- **Plugin API**: Implements integration provider interfaces for plugins
- **Project**: Workspace root for git operations, task file discovery

---

## 2. Requirements Analysis

### 2.1 From Architecture Specification

From `design/specs/architecture.md`:

> "9. Integration Layer (Terminal, Git, Debugger, Tasks)
> location: internal/integration
> This is where 'just an editor' becomes 'mini IDE.'
> VS Code: integrated terminal, debugger protocol, Git interface, task runner.
> Vim: can do all of this, but typically via plugins and duct tape."

### 2.2 Functional Requirements

#### 2.2.1 Terminal Integration

1. **Terminal Emulation**
   - Full PTY-based terminal with ANSI escape code support
   - Multiple terminal instances (tabs)
   - Configurable shell (bash, zsh, fish, powershell)
   - Copy/paste, scrollback buffer
   - Shell integration (working directory tracking)

2. **Terminal Features**
   - Send keystrokes and text to terminal
   - Read terminal output (for AI analysis)
   - Resize handling
   - Terminal profiles (dev, production, custom)

#### 2.2.2 Git Integration

1. **Repository Operations**
   - Initialize, clone repositories
   - Detect repository state
   - Multi-root workspace support

2. **Working Tree Operations**
   - Stage/unstage files (add, reset)
   - Discard changes
   - Stash/unstash

3. **Commit Operations**
   - Create commits with messages
   - Amend last commit
   - View commit history

4. **Branch Operations**
   - Create, switch, delete branches
   - Merge branches
   - Rebase (interactive future)

5. **Remote Operations**
   - Fetch, pull, push
   - Remote management
   - Authentication (SSH, HTTPS, credentials)

6. **Diff and Blame**
   - File diffs (staged, unstaged, between commits)
   - Line-by-line blame information
   - Conflict detection and markers

#### 2.2.3 Debugger Integration

1. **Session Management**
   - Start/stop debug sessions
   - Attach to running processes
   - Multiple debug configurations

2. **Execution Control**
   - Continue, pause, step over/into/out
   - Run to cursor
   - Restart session

3. **Breakpoints**
   - Line breakpoints
   - Conditional breakpoints
   - Function breakpoints
   - Exception breakpoints
   - Breakpoint persistence

4. **State Inspection**
   - Variable inspection (locals, globals, watches)
   - Call stack navigation
   - Evaluate expressions
   - Memory inspection (future)

5. **Debug Adapters**
   - Go (delve)
   - Node.js
   - Python (debugpy)
   - Generic DAP support

#### 2.2.4 Task Runner Integration

1. **Task Discovery**
   - Detect tasks from Makefile, package.json, Taskfile, go.mod
   - Custom task definitions in .keystorm/tasks.toml
   - Task inheritance and composition

2. **Task Execution**
   - Run tasks with arguments
   - Environment variable support
   - Working directory control
   - Input/output capture

3. **Task Management**
   - List available tasks
   - Task history
   - Kill running tasks
   - Task dependencies

4. **Problem Matching**
   - Parse compiler/linter output
   - Extract errors, warnings, locations
   - Navigate to problems

### 2.3 Non-Functional Requirements

1. **Performance**
   - Terminal latency < 10ms for keystrokes
   - Git status < 100ms for typical repos
   - Debug stepping < 50ms perceived latency
   - Task start < 200ms

2. **Reliability**
   - Process cleanup on exit
   - Connection recovery for debuggers
   - Graceful handling of tool failures
   - Resource limits (terminal scrollback, output buffers)

3. **Security**
   - Credential handling for Git
   - Environment isolation for tasks
   - Debug session authentication

### 2.4 Existing Configuration

From `internal/config/sections.go`:

```go
// TerminalConfig provides type-safe access to integrated terminal settings.
type TerminalConfig struct {
    Shell       string  // Shell executable path
    FontSize    int     // Terminal font size
    FontFamily  string  // Terminal font family
    CursorStyle string  // Terminal cursor style ("block", "line", "underline")
    Scrollback  int     // Number of scrollback lines
}
```

This will be extended with additional integration-specific settings.

---

## 3. Architecture Overview

### 3.1 Component Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Integration Layer                               │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │                      Integration Manager                         ││
│  │  - Component lifecycle                                           ││
│  │  - Unified API facade                                            ││
│  │  - Configuration management                                      ││
│  └─────────────────────────────────────────────────────────────────┘│
│                              │                                       │
│           ┌──────────────────┼──────────────────┐                   │
│           ▼                  ▼                  ▼                   │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐        │
│  │    Terminal     │ │      Git        │ │    Debugger     │        │
│  │  ┌───────────┐  │ │  ┌───────────┐  │ │  ┌───────────┐  │        │
│  │  │ PTY Mgr   │  │ │  │Repository │  │ │  │ DAP Client│  │        │
│  │  │ Shell     │  │ │  │ Workdir   │  │ │  │ Breakpts  │  │        │
│  │  │ Output    │  │ │  │ Remote    │  │ │  │ Variables │  │        │
│  │  └───────────┘  │ │  └───────────┘  │ │  └───────────┘  │        │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘        │
│           │                  │                  │                   │
│           └──────────────────┼──────────────────┘                   │
│                              ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │                        Task Runner                               ││
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐     ││
│  │  │ Discovery │  │ Executor  │  │  Output   │  │  Problem  │     ││
│  │  │           │  │           │  │  Parser   │  │  Matcher  │     ││
│  │  └───────────┘  └───────────┘  └───────────┘  └───────────┘     ││
│  └─────────────────────────────────────────────────────────────────┘│
│                              │                                       │
│  ┌───────────────────────────┴───────────────────────────┐          │
│  │                    Process Supervisor                  │          │
│  │  - Child process management                           │          │
│  │  - Signal handling                                    │          │
│  │  - Resource cleanup                                   │          │
│  └────────────────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 Data Flow

**Terminal Input Flow:**
```
1. User types in terminal panel
2. Renderer captures keystroke
3. Dispatcher invokes terminal.input action
4. Terminal Manager writes to PTY stdin
5. Shell processes input
6. Shell writes output to PTY stdout
7. Terminal Manager reads output
8. ANSI parser updates terminal state
9. Renderer updates terminal display
10. Event bus emits terminal.output event
```

**Git Commit Flow:**
```
1. User invokes git.commit action
2. Integration Manager routes to Git component
3. Git component validates staged changes
4. Creates commit with message
5. Updates repository state
6. Event bus emits git.commit.created event
7. Status bar updates branch/commit info
```

**Debug Session Flow:**
```
1. User sets breakpoint via gutter click
2. Dispatcher invokes debug.breakpoint.add
3. Debugger Manager stores breakpoint
4. User starts debug session
5. DAP client connects to debug adapter
6. Breakpoints sent to adapter
7. Execution hits breakpoint
8. DAP sends stopped event
9. Event bus emits debug.stopped
10. UI updates: highlights line, shows variables
```

**Task Execution Flow:**
```
1. User selects task from picker
2. Dispatcher invokes task.run action
3. Task Runner locates task definition
4. Process Supervisor spawns task process
5. Output captured and streamed
6. Problem matcher parses output
7. Diagnostics sent to diagnostics store
8. Task completes, exit code captured
9. Event bus emits task.completed event
```

---

## 4. Package Structure

```
internal/integration/
├── doc.go                  # Package documentation
├── errors.go               # Error types
├── manager.go              # Integration Manager facade
│
├── terminal/               # Terminal emulation
│   ├── terminal.go         # Terminal interface and factory
│   ├── pty.go              # PTY management (Unix)
│   ├── pty_windows.go      # ConPTY for Windows
│   ├── ansi.go             # ANSI escape code parser
│   ├── screen.go           # Terminal screen buffer
│   ├── history.go          # Scrollback history
│   ├── shell.go            # Shell integration
│   └── profile.go          # Terminal profiles
│
├── git/                    # Git integration
│   ├── git.go              # Git interface and types
│   ├── repository.go       # Repository operations
│   ├── worktree.go         # Working tree operations
│   ├── commit.go           # Commit operations
│   ├── branch.go           # Branch operations
│   ├── remote.go           # Remote operations
│   ├── diff.go             # Diff generation
│   ├── blame.go            # Blame information
│   ├── status.go           # Status tracking
│   └── auth.go             # Authentication handling
│
├── debug/                  # Debugger integration
│   ├── debug.go            # Debugger interface
│   ├── session.go          # Debug session management
│   ├── dap/                # Debug Adapter Protocol
│   │   ├── client.go       # DAP client
│   │   ├── protocol.go     # DAP message types
│   │   ├── transport.go    # DAP transport (stdio, socket)
│   │   └── events.go       # DAP event handling
│   ├── breakpoint.go       # Breakpoint management
│   ├── variable.go         # Variable inspection
│   ├── stack.go            # Call stack
│   ├── config.go           # Launch configurations
│   └── adapters/           # Debug adapter configs
│       ├── delve.go        # Go debugger
│       ├── node.go         # Node.js debugger
│       └── python.go       # Python debugger
│
├── task/                   # Task runner
│   ├── task.go             # Task interface and types
│   ├── discovery.go        # Task source discovery
│   ├── sources/            # Task definition sources
│   │   ├── makefile.go     # Makefile parser
│   │   ├── package.go      # package.json parser
│   │   ├── taskfile.go     # Taskfile.yml parser
│   │   ├── keystorm.go     # .keystorm/tasks.toml parser
│   │   └── gomod.go        # go.mod targets
│   ├── executor.go         # Task execution
│   ├── output.go           # Output capture
│   ├── problem.go          # Problem matchers
│   └── variable.go         # Variable substitution
│
├── process/                # Process management
│   ├── supervisor.go       # Process supervisor
│   ├── process.go          # Process wrapper
│   ├── signal.go           # Signal handling
│   └── cleanup.go          # Resource cleanup
│
└── provider.go             # Plugin API provider implementation
```

---

## 5. Core Types and Interfaces

### 5.1 Integration Manager

```go
// Manager is the central facade for all integration features.
// It provides a unified API and manages component lifecycles.
type Manager struct {
    mu sync.RWMutex

    terminal  *terminal.Manager
    git       *git.Manager
    debug     *debug.Manager
    task      *task.Manager
    process   *process.Supervisor

    config    *config.ConfigSystem
    eventBus  EventPublisher

    closed    atomic.Bool
}

// ManagerConfig configures the integration manager.
type ManagerConfig struct {
    // WorkspaceRoot is the root directory for git and task operations.
    WorkspaceRoot string

    // TerminalConfig provides terminal settings.
    TerminalConfig *config.TerminalConfig

    // EventBus for publishing integration events.
    EventBus EventPublisher

    // ConfigSystem for reading configuration.
    ConfigSystem *config.ConfigSystem
}

// NewManager creates a new integration manager.
func NewManager(cfg ManagerConfig) (*Manager, error)

// Terminal returns the terminal manager.
func (m *Manager) Terminal() *terminal.Manager

// Git returns the git manager.
func (m *Manager) Git() *git.Manager

// Debug returns the debug manager.
func (m *Manager) Debug() *debug.Manager

// Task returns the task manager.
func (m *Manager) Task() *task.Manager

// Close shuts down all integration components.
func (m *Manager) Close() error
```

### 5.2 Event Publisher Interface

```go
// EventPublisher defines the interface for publishing integration events.
type EventPublisher interface {
    // Publish sends an event to subscribers.
    Publish(eventType string, data map[string]any)
}

// Common event types:
// - terminal.created, terminal.closed, terminal.output
// - git.status.changed, git.branch.changed, git.commit.created
// - debug.session.started, debug.session.stopped, debug.breakpoint.hit
// - task.started, task.output, task.completed
```

### 5.3 Terminal Types

```go
// Terminal represents a terminal instance.
type Terminal interface {
    // ID returns the terminal's unique identifier.
    ID() string

    // Name returns the terminal's display name.
    Name() string

    // Write sends input to the terminal.
    Write(data []byte) (int, error)

    // Read reads output from the terminal.
    // Returns immediately with available data.
    Read(buf []byte) (int, error)

    // Resize changes the terminal size.
    Resize(cols, rows int) error

    // Screen returns the current screen state.
    Screen() *Screen

    // History returns the scrollback buffer.
    History() *History

    // Close terminates the terminal.
    Close() error

    // Done returns a channel that closes when terminal exits.
    Done() <-chan struct{}

    // ExitCode returns the exit code after terminal closes.
    ExitCode() int
}

// Screen represents the terminal screen buffer.
type Screen struct {
    Width, Height int
    Cells         [][]Cell
    CursorX       int
    CursorY       int
    CursorVisible bool
}

// Cell represents a single character cell.
type Cell struct {
    Rune       rune
    Foreground Color
    Background Color
    Attributes CellAttributes // Bold, italic, underline, etc.
}

// TerminalManager manages terminal instances.
type TerminalManager interface {
    // Create creates a new terminal.
    Create(opts TerminalOptions) (Terminal, error)

    // Get returns a terminal by ID.
    Get(id string) (Terminal, bool)

    // List returns all terminals.
    List() []Terminal

    // Close closes a terminal by ID.
    Close(id string) error

    // CloseAll closes all terminals.
    CloseAll() error
}

// TerminalOptions configures a new terminal.
type TerminalOptions struct {
    Name       string            // Display name
    Shell      string            // Shell executable (default from config)
    Args       []string          // Shell arguments
    Env        map[string]string // Environment variables
    WorkDir    string            // Working directory
    Cols, Rows int               // Initial size
    Profile    string            // Terminal profile name
}
```

### 5.4 Git Types

```go
// Repository represents a git repository.
type Repository interface {
    // Path returns the repository root path.
    Path() string

    // Head returns the current HEAD reference.
    Head() (*Reference, error)

    // Status returns the working tree status.
    Status() (*Status, error)

    // Stage stages files for commit.
    Stage(paths ...string) error

    // Unstage unstages files.
    Unstage(paths ...string) error

    // Discard discards changes to files.
    Discard(paths ...string) error

    // Commit creates a new commit.
    Commit(message string, opts CommitOptions) (*Commit, error)

    // Branches returns all branches.
    Branches() ([]*Branch, error)

    // Checkout switches to a branch or commit.
    Checkout(ref string, opts CheckoutOptions) error

    // Diff returns diff for the given paths.
    Diff(opts DiffOptions) (*Diff, error)

    // Blame returns blame information for a file.
    Blame(path string) (*BlameResult, error)

    // Log returns commit history.
    Log(opts LogOptions) ([]*Commit, error)

    // Fetch fetches from remotes.
    Fetch(opts FetchOptions) error

    // Pull pulls from remote.
    Pull(opts PullOptions) error

    // Push pushes to remote.
    Push(opts PushOptions) error

    // Stash stashes changes.
    Stash(message string) error

    // StashPop pops the last stash.
    StashPop() error
}

// Status represents working tree status.
type Status struct {
    Branch    string
    Ahead     int
    Behind    int
    Staged    []FileStatus
    Unstaged  []FileStatus
    Untracked []string
    Conflicts []string
}

// FileStatus represents the status of a file.
type FileStatus struct {
    Path       string
    OldPath    string // For renames
    Status     StatusCode // Added, Modified, Deleted, Renamed, etc.
    Staged     bool
}

// StatusCode represents file status.
type StatusCode int

const (
    StatusUnmodified StatusCode = iota
    StatusModified
    StatusAdded
    StatusDeleted
    StatusRenamed
    StatusCopied
    StatusUntracked
    StatusIgnored
    StatusConflict
)

// GitManager manages git operations.
type GitManager interface {
    // Open opens a repository at the given path.
    Open(path string) (Repository, error)

    // Discover finds the repository containing the path.
    Discover(path string) (Repository, error)

    // Init initializes a new repository.
    Init(path string) (Repository, error)

    // Clone clones a repository.
    Clone(url, path string, opts CloneOptions) (Repository, error)

    // IsRepository checks if path is a repository.
    IsRepository(path string) bool
}
```

### 5.5 Debugger Types

```go
// Session represents a debug session.
type Session interface {
    // ID returns the session's unique identifier.
    ID() string

    // State returns the current session state.
    State() SessionState

    // Continue resumes execution.
    Continue() error

    // Pause pauses execution.
    Pause() error

    // StepOver steps over the current statement.
    StepOver() error

    // StepInto steps into the current statement.
    StepInto() error

    // StepOut steps out of the current function.
    StepOut() error

    // Restart restarts the session.
    Restart() error

    // Stop terminates the session.
    Stop() error

    // Threads returns all threads.
    Threads() ([]*Thread, error)

    // StackTrace returns the call stack for a thread.
    StackTrace(threadID int) ([]*StackFrame, error)

    // Scopes returns variable scopes for a frame.
    Scopes(frameID int) ([]*Scope, error)

    // Variables returns variables in a scope.
    Variables(variablesReference int) ([]*Variable, error)

    // Evaluate evaluates an expression.
    Evaluate(expression string, frameID int) (*Variable, error)

    // SetBreakpoints sets breakpoints in a file.
    SetBreakpoints(path string, breakpoints []SourceBreakpoint) ([]*Breakpoint, error)

    // Events returns a channel of debug events.
    Events() <-chan DebugEvent
}

// SessionState represents the debug session state.
type SessionState int

const (
    SessionStateInitializing SessionState = iota
    SessionStateRunning
    SessionStateStopped
    SessionStateTerminated
)

// Thread represents a debug thread.
type Thread struct {
    ID   int
    Name string
}

// StackFrame represents a call stack frame.
type StackFrame struct {
    ID     int
    Name   string
    Source *Source
    Line   int
    Column int
}

// Variable represents a debugger variable.
type Variable struct {
    Name               string
    Value              string
    Type               string
    VariablesReference int    // For expanding complex types
    IndexedVariables   int    // For arrays
    NamedVariables     int    // For objects
}

// Breakpoint represents a breakpoint.
type Breakpoint struct {
    ID        int
    Verified  bool
    Line      int
    Column    int
    Source    *Source
    Message   string // Error message if not verified
}

// SourceBreakpoint represents a breakpoint request.
type SourceBreakpoint struct {
    Line         int
    Column       int
    Condition    string
    HitCondition string
    LogMessage   string
}

// DebugEvent represents a debug event.
type DebugEvent struct {
    Type string // stopped, continued, output, terminated, etc.
    Data any
}

// DebugManager manages debug sessions.
type DebugManager interface {
    // Launch starts a new debug session.
    Launch(config LaunchConfig) (Session, error)

    // Attach attaches to a running process.
    Attach(config AttachConfig) (Session, error)

    // Sessions returns all active sessions.
    Sessions() []Session

    // GetSession returns a session by ID.
    GetSession(id string) (Session, bool)

    // Configurations returns available launch configs.
    Configurations() []LaunchConfig

    // AddConfiguration adds a launch configuration.
    AddConfiguration(config LaunchConfig) error
}

// LaunchConfig represents a debug launch configuration.
type LaunchConfig struct {
    Name        string
    Type        string            // "go", "node", "python", etc.
    Request     string            // "launch" or "attach"
    Program     string            // Program to debug
    Args        []string          // Program arguments
    Env         map[string]string // Environment variables
    Cwd         string            // Working directory
    StopOnEntry bool

    // Adapter-specific options
    Options map[string]any
}
```

### 5.6 Task Types

```go
// Task represents a runnable task.
type Task struct {
    Name        string
    Description string
    Source      string            // "makefile", "package.json", "keystorm", etc.
    Command     string
    Args        []string
    Env         map[string]string
    WorkDir     string
    Group       string            // "build", "test", "run", etc.
    DependsOn   []string          // Task dependencies
    ProblemMatcher string          // Problem matcher name
}

// TaskRun represents a running task instance.
type TaskRun interface {
    // ID returns the run's unique identifier.
    ID() string

    // Task returns the task definition.
    Task() *Task

    // State returns the current run state.
    State() TaskState

    // Output returns captured output.
    Output() io.Reader

    // Wait waits for the task to complete.
    Wait() error

    // Kill terminates the task.
    Kill() error

    // ExitCode returns the exit code (-1 if still running).
    ExitCode() int
}

// TaskState represents task run state.
type TaskState int

const (
    TaskStatePending TaskState = iota
    TaskStateRunning
    TaskStateSucceeded
    TaskStateFailed
    TaskStateCancelled
)

// Problem represents a parsed problem from task output.
type Problem struct {
    File     string
    Line     int
    Column   int
    EndLine  int
    EndColumn int
    Severity ProblemSeverity
    Message  string
    Code     string
    Source   string // Task that produced it
}

// ProblemSeverity represents problem severity.
type ProblemSeverity int

const (
    ProblemSeverityError ProblemSeverity = iota
    ProblemSeverityWarning
    ProblemSeverityInfo
)

// TaskManager manages tasks.
type TaskManager interface {
    // Discover finds all available tasks.
    Discover(workspaceRoot string) ([]*Task, error)

    // Run executes a task.
    Run(task *Task, opts RunOptions) (TaskRun, error)

    // RunByName runs a task by name.
    RunByName(name string, opts RunOptions) (TaskRun, error)

    // Active returns currently running tasks.
    Active() []TaskRun

    // History returns recent task runs.
    History(limit int) []TaskRun

    // Problems returns problems from the last run of a task.
    Problems(taskName string) []*Problem

    // AllProblems returns all current problems.
    AllProblems() []*Problem

    // RegisterMatcher registers a problem matcher.
    RegisterMatcher(name string, matcher *ProblemMatcher) error
}

// RunOptions configures task execution.
type RunOptions struct {
    Args       []string          // Additional arguments
    Env        map[string]string // Additional environment
    WorkDir    string            // Override working directory
    Background bool              // Run without waiting
}

// ProblemMatcher defines how to parse problems from output.
type ProblemMatcher struct {
    Pattern     string           // Regex pattern
    File        int              // Capture group for file
    Line        int              // Capture group for line
    Column      int              // Capture group for column
    Message     int              // Capture group for message
    Severity    int              // Capture group for severity (optional)
    Code        int              // Capture group for code (optional)
    Loop        bool             // Match multiple times per line
    Multiline   bool             // Pattern spans multiple lines
}
```

### 5.7 Process Supervisor

```go
// Supervisor manages child processes.
type Supervisor struct {
    mu        sync.RWMutex
    processes map[string]*Process
    shutdown  chan struct{}
}

// Process represents a managed child process.
type Process struct {
    ID       string
    Cmd      *exec.Cmd
    Stdin    io.WriteCloser
    Stdout   io.ReadCloser
    Stderr   io.ReadCloser
    Started  time.Time
    Done     chan struct{}
    ExitCode int
}

// NewSupervisor creates a new process supervisor.
func NewSupervisor() *Supervisor

// Start starts a new managed process.
func (s *Supervisor) Start(name string, cmd *exec.Cmd) (*Process, error)

// Get returns a process by ID.
func (s *Supervisor) Get(id string) (*Process, bool)

// Kill kills a process by ID.
func (s *Supervisor) Kill(id string) error

// KillAll kills all managed processes.
func (s *Supervisor) KillAll()

// Shutdown initiates graceful shutdown of all processes.
func (s *Supervisor) Shutdown(timeout time.Duration)
```

---

## 6. Terminal Integration

### 6.1 PTY Management

```go
// ptyTerminal implements Terminal using PTY.
type ptyTerminal struct {
    id      string
    name    string
    pty     *os.File
    cmd     *exec.Cmd
    screen  *Screen
    history *History
    parser  *ansiParser

    mu      sync.RWMutex
    done    chan struct{}
    exitCode int

    eventBus EventPublisher
}

// newPTYTerminal creates a PTY-based terminal.
func newPTYTerminal(opts TerminalOptions, eventBus EventPublisher) (*ptyTerminal, error) {
    // Create PTY
    ptmx, err := pty.Start(cmd)
    if err != nil {
        return nil, fmt.Errorf("pty start: %w", err)
    }

    t := &ptyTerminal{
        id:       uuid.New().String(),
        name:     opts.Name,
        pty:      ptmx,
        cmd:      cmd,
        screen:   NewScreen(opts.Cols, opts.Rows),
        history:  NewHistory(opts.Scrollback),
        parser:   newAnsiParser(),
        done:     make(chan struct{}),
        eventBus: eventBus,
    }

    // Start output reader
    go t.readLoop()

    // Wait for process exit
    go t.waitLoop()

    return t, nil
}

// readLoop reads from PTY and updates screen.
func (t *ptyTerminal) readLoop() {
    buf := make([]byte, 4096)
    for {
        n, err := t.pty.Read(buf)
        if err != nil {
            return
        }

        t.mu.Lock()
        t.parser.Parse(buf[:n], t.screen, t.history)
        t.mu.Unlock()

        // Emit output event
        if t.eventBus != nil {
            t.eventBus.Publish("terminal.output", map[string]any{
                "id":   t.id,
                "data": string(buf[:n]),
            })
        }
    }
}
```

### 6.2 ANSI Parser

```go
// ansiParser parses ANSI escape sequences.
type ansiParser struct {
    state   parserState
    params  []int
    private rune
}

// parserState tracks parser state machine.
type parserState int

const (
    stateGround parserState = iota
    stateEscape
    stateCSI
    stateOSC
    stateDCS
)

// Parse processes input bytes and updates screen.
func (p *ansiParser) Parse(data []byte, screen *Screen, history *History) {
    for _, b := range data {
        p.parseByte(b, screen, history)
    }
}

// parseByte handles a single byte.
func (p *ansiParser) parseByte(b byte, screen *Screen, history *History) {
    switch p.state {
    case stateGround:
        if b == 0x1b { // ESC
            p.state = stateEscape
            return
        }
        if b >= 0x20 && b < 0x7f {
            screen.WriteRune(rune(b))
        } else {
            p.handleControlChar(b, screen, history)
        }

    case stateEscape:
        switch b {
        case '[':
            p.state = stateCSI
            p.params = p.params[:0]
        case ']':
            p.state = stateOSC
        case 'P':
            p.state = stateDCS
        default:
            p.handleEscapeSequence(b, screen)
            p.state = stateGround
        }

    case stateCSI:
        p.parseCSI(b, screen, history)

    // ... other states
    }
}

// handleCSICommand handles CSI command sequences.
func (p *ansiParser) handleCSICommand(cmd rune, screen *Screen, history *History) {
    switch cmd {
    case 'A': // Cursor Up
        screen.MoveCursorUp(p.paramOr(0, 1))
    case 'B': // Cursor Down
        screen.MoveCursorDown(p.paramOr(0, 1))
    case 'C': // Cursor Forward
        screen.MoveCursorRight(p.paramOr(0, 1))
    case 'D': // Cursor Back
        screen.MoveCursorLeft(p.paramOr(0, 1))
    case 'H', 'f': // Cursor Position
        row := p.paramOr(0, 1) - 1
        col := p.paramOr(1, 1) - 1
        screen.SetCursor(col, row)
    case 'J': // Erase Display
        screen.EraseDisplay(p.paramOr(0, 0))
    case 'K': // Erase Line
        screen.EraseLine(p.paramOr(0, 0))
    case 'm': // SGR (Select Graphic Rendition)
        screen.SetAttributes(p.params)
    case 'S': // Scroll Up
        history.Push(screen.ScrollUp(p.paramOr(0, 1)))
    case 'T': // Scroll Down
        screen.ScrollDown(p.paramOr(0, 1))
    // ... many more CSI commands
    }
}
```

### 6.3 Shell Integration

```go
// ShellIntegration provides shell integration features.
type ShellIntegration struct {
    terminal Terminal
    shell    string

    cwd      string
    prompt   string
    command  string

    mu sync.RWMutex
}

// Initialize sets up shell integration.
func (si *ShellIntegration) Initialize() error {
    // Detect shell type and install integration script
    switch si.shell {
    case "bash":
        return si.initBash()
    case "zsh":
        return si.initZsh()
    case "fish":
        return si.initFish()
    default:
        return nil // No integration for unknown shells
    }
}

// bashIntegrationScript is injected into bash.
const bashIntegrationScript = `
__keystorm_prompt_command() {
    local exit_code=$?
    printf '\033]7;file://%s%s\033\\' "$HOSTNAME" "$PWD"
    printf '\033]1337;CurrentCmd=%s\033\\' "$BASH_COMMAND"
    return $exit_code
}
PROMPT_COMMAND="__keystorm_prompt_command${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
`

// handleOSC processes OSC escape sequences for shell integration.
func (si *ShellIntegration) handleOSC(cmd int, data string) {
    switch cmd {
    case 7: // Working directory
        si.mu.Lock()
        si.cwd = parseFileURL(data)
        si.mu.Unlock()
    case 1337: // iTerm2-style commands
        parts := strings.SplitN(data, "=", 2)
        if len(parts) == 2 {
            switch parts[0] {
            case "CurrentCmd":
                si.mu.Lock()
                si.command = parts[1]
                si.mu.Unlock()
            }
        }
    }
}

// CurrentDirectory returns the shell's current directory.
func (si *ShellIntegration) CurrentDirectory() string {
    si.mu.RLock()
    defer si.mu.RUnlock()
    return si.cwd
}
```

---

## 7. Git Integration

### 7.1 Repository Implementation

```go
// repository implements Repository using go-git.
type repository struct {
    path string
    repo *git.Repository

    mu        sync.RWMutex
    statusMu  sync.RWMutex
    cached    *Status
    cacheTime time.Time

    eventBus EventPublisher
}

// Open opens an existing repository.
func (m *gitManager) Open(path string) (Repository, error) {
    repo, err := git.PlainOpen(path)
    if err != nil {
        return nil, fmt.Errorf("open repository: %w", err)
    }

    return &repository{
        path:     path,
        repo:     repo,
        eventBus: m.eventBus,
    }, nil
}

// Status returns the working tree status.
func (r *repository) Status() (*Status, error) {
    r.statusMu.Lock()
    defer r.statusMu.Unlock()

    // Return cached status if recent (< 100ms)
    if r.cached != nil && time.Since(r.cacheTime) < 100*time.Millisecond {
        return r.cached, nil
    }

    worktree, err := r.repo.Worktree()
    if err != nil {
        return nil, fmt.Errorf("get worktree: %w", err)
    }

    gitStatus, err := worktree.Status()
    if err != nil {
        return nil, fmt.Errorf("get status: %w", err)
    }

    status := &Status{
        Staged:    make([]FileStatus, 0),
        Unstaged:  make([]FileStatus, 0),
        Untracked: make([]string, 0),
    }

    // Get branch info
    head, err := r.repo.Head()
    if err == nil {
        status.Branch = head.Name().Short()
        // Calculate ahead/behind
        status.Ahead, status.Behind = r.calculateAheadBehind(head)
    }

    // Process file statuses
    for path, fileStatus := range gitStatus {
        if fileStatus.Staging != git.Unmodified {
            status.Staged = append(status.Staged, FileStatus{
                Path:   path,
                Status: convertStatus(fileStatus.Staging),
                Staged: true,
            })
        }

        if fileStatus.Worktree != git.Unmodified {
            if fileStatus.Worktree == git.Untracked {
                status.Untracked = append(status.Untracked, path)
            } else {
                status.Unstaged = append(status.Unstaged, FileStatus{
                    Path:   path,
                    Status: convertStatus(fileStatus.Worktree),
                    Staged: false,
                })
            }
        }
    }

    r.cached = status
    r.cacheTime = time.Now()

    return status, nil
}

// Commit creates a new commit.
func (r *repository) Commit(message string, opts CommitOptions) (*Commit, error) {
    r.mu.Lock()
    defer r.mu.Unlock()

    worktree, err := r.repo.Worktree()
    if err != nil {
        return nil, fmt.Errorf("get worktree: %w", err)
    }

    commitOpts := &git.CommitOptions{
        Author: &object.Signature{
            Name:  opts.AuthorName,
            Email: opts.AuthorEmail,
            When:  time.Now(),
        },
    }

    if opts.Amend {
        commitOpts.Amend = true
    }

    hash, err := worktree.Commit(message, commitOpts)
    if err != nil {
        return nil, fmt.Errorf("commit: %w", err)
    }

    // Get commit object
    commitObj, err := r.repo.CommitObject(hash)
    if err != nil {
        return nil, fmt.Errorf("get commit: %w", err)
    }

    commit := &Commit{
        Hash:    hash.String(),
        Message: commitObj.Message,
        Author:  commitObj.Author.Name,
        Email:   commitObj.Author.Email,
        Time:    commitObj.Author.When,
    }

    // Invalidate status cache
    r.statusMu.Lock()
    r.cached = nil
    r.statusMu.Unlock()

    // Emit event
    if r.eventBus != nil {
        r.eventBus.Publish("git.commit.created", map[string]any{
            "hash":    commit.Hash,
            "message": commit.Message,
        })
    }

    return commit, nil
}
```

### 7.2 Diff Generation

```go
// Diff returns diff for the given options.
func (r *repository) Diff(opts DiffOptions) (*Diff, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    worktree, err := r.repo.Worktree()
    if err != nil {
        return nil, fmt.Errorf("get worktree: %w", err)
    }

    var diff *Diff

    switch {
    case opts.Cached:
        // Staged changes (index vs HEAD)
        diff, err = r.stagedDiff()
    case opts.Commit != "":
        // Specific commit
        diff, err = r.commitDiff(opts.Commit)
    case opts.From != "" && opts.To != "":
        // Between two commits
        diff, err = r.rangeDiff(opts.From, opts.To)
    default:
        // Working tree changes (worktree vs index)
        diff, err = r.worktreeDiff(worktree, opts.Paths)
    }

    return diff, err
}

// worktreeDiff generates diff for unstaged changes.
func (r *repository) worktreeDiff(worktree *git.Worktree, paths []string) (*Diff, error) {
    status, err := worktree.Status()
    if err != nil {
        return nil, err
    }

    diff := &Diff{
        Files: make([]*FileDiff, 0),
    }

    for path, fileStatus := range status {
        if fileStatus.Worktree == git.Unmodified {
            continue
        }

        if len(paths) > 0 && !containsPath(paths, path) {
            continue
        }

        fileDiff, err := r.generateFileDiff(path, false)
        if err != nil {
            continue // Skip files we can't diff
        }

        diff.Files = append(diff.Files, fileDiff)
    }

    return diff, nil
}

// generateFileDiff creates a diff for a single file.
func (r *repository) generateFileDiff(path string, staged bool) (*FileDiff, error) {
    // Get old content (from index or HEAD)
    oldContent, err := r.getOldContent(path, staged)
    if err != nil && !errors.Is(err, object.ErrFileNotFound) {
        return nil, err
    }

    // Get new content
    newContent, err := r.getNewContent(path, staged)
    if err != nil {
        return nil, err
    }

    // Generate unified diff
    edits := myers.ComputeEdits(span.URIFromPath(path), string(oldContent), string(newContent))
    unified := gotextdiff.ToUnified(path, path, string(oldContent), edits)

    fileDiff := &FileDiff{
        Path:    path,
        Status:  detectDiffStatus(oldContent, newContent),
        Hunks:   make([]*Hunk, 0),
    }

    // Parse hunks from unified diff
    for _, hunk := range unified.Hunks {
        fileDiff.Hunks = append(fileDiff.Hunks, &Hunk{
            OldStart: hunk.FromLine,
            OldLines: hunk.FromLine + len(hunk.Lines),
            NewStart: hunk.ToLine,
            NewLines: hunk.ToLine + len(hunk.Lines),
            Lines:    convertDiffLines(hunk.Lines),
        })
    }

    return fileDiff, nil
}
```

### 7.3 Authentication

```go
// authManager handles git authentication.
type authManager struct {
    credentialCache map[string]*credentials
    mu              sync.RWMutex
}

// credentials stores authentication info.
type credentials struct {
    Username string
    Password string
    SSHKey   []byte
    ExpiresAt time.Time
}

// GetAuth returns authentication for a remote URL.
func (am *authManager) GetAuth(url string) (transport.AuthMethod, error) {
    // Parse URL to determine auth type
    parsed, err := giturls.Parse(url)
    if err != nil {
        return nil, err
    }

    switch parsed.Scheme {
    case "ssh", "git":
        return am.getSSHAuth(url)
    case "http", "https":
        return am.getHTTPAuth(url)
    default:
        return nil, nil
    }
}

// getSSHAuth returns SSH authentication.
func (am *authManager) getSSHAuth(url string) (transport.AuthMethod, error) {
    // Try SSH agent first
    if auth, err := ssh.NewSSHAgentAuth("git"); err == nil {
        return auth, nil
    }

    // Fall back to default key
    home, _ := os.UserHomeDir()
    keyPath := filepath.Join(home, ".ssh", "id_rsa")

    if _, err := os.Stat(keyPath); err == nil {
        return ssh.NewPublicKeysFromFile("git", keyPath, "")
    }

    // Try ed25519
    keyPath = filepath.Join(home, ".ssh", "id_ed25519")
    if _, err := os.Stat(keyPath); err == nil {
        return ssh.NewPublicKeysFromFile("git", keyPath, "")
    }

    return nil, fmt.Errorf("no SSH key found")
}

// getHTTPAuth returns HTTP authentication.
func (am *authManager) getHTTPAuth(url string) (transport.AuthMethod, error) {
    // Check cache
    am.mu.RLock()
    if cred, ok := am.credentialCache[url]; ok {
        if time.Now().Before(cred.ExpiresAt) {
            am.mu.RUnlock()
            return &http.BasicAuth{
                Username: cred.Username,
                Password: cred.Password,
            }, nil
        }
    }
    am.mu.RUnlock()

    // Try git credential helper
    cred, err := am.fromCredentialHelper(url)
    if err == nil {
        am.mu.Lock()
        am.credentialCache[url] = cred
        am.mu.Unlock()

        return &http.BasicAuth{
            Username: cred.Username,
            Password: cred.Password,
        }, nil
    }

    return nil, fmt.Errorf("no credentials found for %s", url)
}
```

---

## 8. Debugger Integration

### 8.1 DAP Client

```go
// dapClient implements the Debug Adapter Protocol client.
type dapClient struct {
    transport DAPTransport

    mu         sync.Mutex
    seq        int
    pending    map[int]chan *dap.Response
    handlers   map[string][]EventHandler

    capabilities *dap.Capabilities
}

// DAPTransport abstracts the DAP transport layer.
type DAPTransport interface {
    Send(message dap.Message) error
    Receive() (dap.Message, error)
    Close() error
}

// NewDAPClient creates a new DAP client.
func NewDAPClient(transport DAPTransport) *dapClient {
    c := &dapClient{
        transport: transport,
        pending:   make(map[int]chan *dap.Response),
        handlers:  make(map[string][]EventHandler),
    }

    go c.readLoop()

    return c
}

// Initialize sends the initialize request.
func (c *dapClient) Initialize(args dap.InitializeRequestArguments) (*dap.Capabilities, error) {
    resp, err := c.sendRequest("initialize", args)
    if err != nil {
        return nil, err
    }

    var caps dap.Capabilities
    if err := json.Unmarshal(resp.Body, &caps); err != nil {
        return nil, err
    }

    c.capabilities = &caps
    return &caps, nil
}

// sendRequest sends a request and waits for response.
func (c *dapClient) sendRequest(command string, args any) (*dap.Response, error) {
    c.mu.Lock()
    seq := c.seq
    c.seq++

    respChan := make(chan *dap.Response, 1)
    c.pending[seq] = respChan
    c.mu.Unlock()

    defer func() {
        c.mu.Lock()
        delete(c.pending, seq)
        c.mu.Unlock()
    }()

    req := &dap.Request{
        ProtocolMessage: dap.ProtocolMessage{
            Seq:  seq,
            Type: "request",
        },
        Command:   command,
        Arguments: args,
    }

    if err := c.transport.Send(req); err != nil {
        return nil, err
    }

    select {
    case resp := <-respChan:
        if !resp.Success {
            return nil, fmt.Errorf("DAP error: %s", resp.Message)
        }
        return resp, nil
    case <-time.After(30 * time.Second):
        return nil, fmt.Errorf("DAP request timeout: %s", command)
    }
}

// readLoop reads messages from transport.
func (c *dapClient) readLoop() {
    for {
        msg, err := c.transport.Receive()
        if err != nil {
            return
        }

        switch m := msg.(type) {
        case *dap.Response:
            c.mu.Lock()
            if ch, ok := c.pending[m.RequestSeq]; ok {
                ch <- m
            }
            c.mu.Unlock()

        case *dap.Event:
            c.handleEvent(m)
        }
    }
}

// handleEvent dispatches events to handlers.
func (c *dapClient) handleEvent(event *dap.Event) {
    c.mu.Lock()
    handlers := c.handlers[event.Event]
    c.mu.Unlock()

    for _, h := range handlers {
        go h(event)
    }
}
```

### 8.2 Debug Session

```go
// session implements Session.
type session struct {
    id       string
    client   *dapClient
    config   LaunchConfig
    state    SessionState

    threads     []*Thread
    breakpoints map[string][]*Breakpoint // path -> breakpoints

    mu       sync.RWMutex
    events   chan DebugEvent
    done     chan struct{}

    eventBus EventPublisher
}

// Launch starts a debug session.
func (m *debugManager) Launch(config LaunchConfig) (Session, error) {
    // Find debug adapter for type
    adapter, err := m.findAdapter(config.Type)
    if err != nil {
        return nil, err
    }

    // Start adapter process
    cmd := exec.Command(adapter.Command, adapter.Args...)
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()

    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("start adapter: %w", err)
    }

    // Create transport
    transport := NewStdioTransport(stdout, stdin)
    client := NewDAPClient(transport)

    // Initialize
    caps, err := client.Initialize(dap.InitializeRequestArguments{
        AdapterID:                    config.Type,
        LinesStartAt1:                true,
        ColumnsStartAt1:              true,
        SupportsVariableType:         true,
        SupportsVariablePaging:       true,
        SupportsRunInTerminalRequest: true,
    })
    if err != nil {
        cmd.Process.Kill()
        return nil, fmt.Errorf("initialize: %w", err)
    }

    s := &session{
        id:          uuid.New().String(),
        client:      client,
        config:      config,
        state:       SessionStateInitializing,
        breakpoints: make(map[string][]*Breakpoint),
        events:      make(chan DebugEvent, 100),
        done:        make(chan struct{}),
        eventBus:    m.eventBus,
    }

    // Register event handlers
    client.OnEvent("stopped", s.handleStopped)
    client.OnEvent("continued", s.handleContinued)
    client.OnEvent("terminated", s.handleTerminated)
    client.OnEvent("output", s.handleOutput)
    client.OnEvent("breakpoint", s.handleBreakpoint)

    // Send launch request
    launchArgs := map[string]any{
        "program":     config.Program,
        "args":        config.Args,
        "cwd":         config.Cwd,
        "env":         config.Env,
        "stopOnEntry": config.StopOnEntry,
    }

    // Merge adapter-specific options
    for k, v := range config.Options {
        launchArgs[k] = v
    }

    if _, err := client.sendRequest("launch", launchArgs); err != nil {
        return nil, fmt.Errorf("launch: %w", err)
    }

    // Send configuration done
    if caps.SupportsConfigurationDoneRequest {
        if _, err := client.sendRequest("configurationDone", nil); err != nil {
            return nil, fmt.Errorf("configurationDone: %w", err)
        }
    }

    s.state = SessionStateRunning

    if s.eventBus != nil {
        s.eventBus.Publish("debug.session.started", map[string]any{
            "id":     s.id,
            "config": config.Name,
        })
    }

    return s, nil
}

// handleStopped handles stopped events.
func (s *session) handleStopped(event *dap.Event) {
    var body dap.StoppedEventBody
    json.Unmarshal(event.Body, &body)

    s.mu.Lock()
    s.state = SessionStateStopped
    s.mu.Unlock()

    s.events <- DebugEvent{
        Type: "stopped",
        Data: map[string]any{
            "reason":      body.Reason,
            "threadId":    body.ThreadId,
            "allStopped":  body.AllThreadsStopped,
            "description": body.Description,
        },
    }

    if s.eventBus != nil {
        s.eventBus.Publish("debug.stopped", map[string]any{
            "sessionId": s.id,
            "reason":    body.Reason,
        })
    }
}

// SetBreakpoints sets breakpoints for a file.
func (s *session) SetBreakpoints(path string, breakpoints []SourceBreakpoint) ([]*Breakpoint, error) {
    args := dap.SetBreakpointsArguments{
        Source: dap.Source{
            Path: path,
        },
        Breakpoints: make([]dap.SourceBreakpoint, len(breakpoints)),
    }

    for i, bp := range breakpoints {
        args.Breakpoints[i] = dap.SourceBreakpoint{
            Line:         bp.Line,
            Column:       bp.Column,
            Condition:    bp.Condition,
            HitCondition: bp.HitCondition,
            LogMessage:   bp.LogMessage,
        }
    }

    resp, err := s.client.sendRequest("setBreakpoints", args)
    if err != nil {
        return nil, err
    }

    var body dap.SetBreakpointsResponseBody
    json.Unmarshal(resp.Body, &body)

    result := make([]*Breakpoint, len(body.Breakpoints))
    for i, bp := range body.Breakpoints {
        result[i] = &Breakpoint{
            ID:       bp.Id,
            Verified: bp.Verified,
            Line:     bp.Line,
            Column:   bp.Column,
            Message:  bp.Message,
            Source: &Source{
                Path: path,
            },
        }
    }

    s.mu.Lock()
    s.breakpoints[path] = result
    s.mu.Unlock()

    return result, nil
}
```

### 8.3 Debug Adapter Configuration

```go
// AdapterConfig defines a debug adapter.
type AdapterConfig struct {
    Type    string   // "go", "node", "python"
    Command string   // Adapter executable
    Args    []string // Adapter arguments
    Runtime string   // Runtime to use
}

// DefaultAdapters provides default debug adapter configurations.
var DefaultAdapters = map[string]AdapterConfig{
    "go": {
        Type:    "go",
        Command: "dlv",
        Args:    []string{"dap"},
    },
    "node": {
        Type:    "node",
        Command: "node",
        Args:    []string{"--inspect-brk=0"},
    },
    "python": {
        Type:    "python",
        Command: "python",
        Args:    []string{"-m", "debugpy.adapter"},
    },
}

// findAdapter locates the debug adapter for a type.
func (m *debugManager) findAdapter(adapterType string) (*AdapterConfig, error) {
    // Check user configuration first
    if adapter := m.config.DebugAdapters[adapterType]; adapter != nil {
        return adapter, nil
    }

    // Fall back to defaults
    if adapter, ok := DefaultAdapters[adapterType]; ok {
        // Verify command exists
        if _, err := exec.LookPath(adapter.Command); err != nil {
            return nil, fmt.Errorf("adapter %s not found: %s", adapterType, adapter.Command)
        }
        return &adapter, nil
    }

    return nil, fmt.Errorf("unknown adapter type: %s", adapterType)
}
```

---

## 9. Task Runner Integration

### 9.1 Task Discovery

```go
// discoverer finds tasks from various sources.
type discoverer struct {
    sources []TaskSource
}

// TaskSource defines a task definition source.
type TaskSource interface {
    // Name returns the source name.
    Name() string

    // Detect checks if this source exists in a directory.
    Detect(dir string) bool

    // Parse parses tasks from the source.
    Parse(dir string) ([]*Task, error)
}

// NewDiscoverer creates a task discoverer with all sources.
func NewDiscoverer() *discoverer {
    return &discoverer{
        sources: []TaskSource{
            &makefileSource{},
            &packageJsonSource{},
            &taskfileSource{},
            &keystormSource{},
            &goModSource{},
        },
    }
}

// Discover finds all tasks in a directory.
func (d *discoverer) Discover(dir string) ([]*Task, error) {
    var allTasks []*Task

    for _, source := range d.sources {
        if source.Detect(dir) {
            tasks, err := source.Parse(dir)
            if err != nil {
                continue // Log and continue
            }
            allTasks = append(allTasks, tasks...)
        }
    }

    return allTasks, nil
}
```

### 9.2 Makefile Parser

```go
// makefileSource parses Makefile targets.
type makefileSource struct{}

func (s *makefileSource) Name() string { return "makefile" }

func (s *makefileSource) Detect(dir string) bool {
    for _, name := range []string{"Makefile", "makefile", "GNUmakefile"} {
        if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
            return true
        }
    }
    return false
}

func (s *makefileSource) Parse(dir string) ([]*Task, error) {
    // Find Makefile
    var makefile string
    for _, name := range []string{"Makefile", "makefile", "GNUmakefile"} {
        path := filepath.Join(dir, name)
        if _, err := os.Stat(path); err == nil {
            makefile = path
            break
        }
    }

    content, err := os.ReadFile(makefile)
    if err != nil {
        return nil, err
    }

    var tasks []*Task

    // Parse targets
    // Regex for target: target: dependencies
    targetRe := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_-]*)\s*:`)
    // Regex for .PHONY targets
    phonyRe := regexp.MustCompile(`^\.PHONY\s*:\s*(.+)$`)

    phonyTargets := make(map[string]bool)

    lines := strings.Split(string(content), "\n")
    for i, line := range lines {
        // Track PHONY targets
        if matches := phonyRe.FindStringSubmatch(line); matches != nil {
            for _, t := range strings.Fields(matches[1]) {
                phonyTargets[t] = true
            }
            continue
        }

        // Find targets
        if matches := targetRe.FindStringSubmatch(line); matches != nil {
            target := matches[1]

            // Skip internal targets (starting with . or _)
            if strings.HasPrefix(target, ".") || strings.HasPrefix(target, "_") {
                continue
            }

            // Look for description in comment above
            description := ""
            if i > 0 && strings.HasPrefix(lines[i-1], "#") {
                description = strings.TrimPrefix(lines[i-1], "#")
                description = strings.TrimSpace(description)
            }

            tasks = append(tasks, &Task{
                Name:        target,
                Description: description,
                Source:      "makefile",
                Command:     "make",
                Args:        []string{target},
                WorkDir:     dir,
                Group:       guessGroup(target),
            })
        }
    }

    return tasks, nil
}
```

### 9.3 Task Executor

```go
// executor runs tasks.
type executor struct {
    supervisor *process.Supervisor
    matchers   map[string]*ProblemMatcher

    mu      sync.RWMutex
    active  map[string]*taskRun
    history []*taskRun

    eventBus EventPublisher
}

// taskRun implements TaskRun.
type taskRun struct {
    id       string
    task     *Task
    state    TaskState
    process  *process.Process
    output   *bytes.Buffer
    problems []*Problem
    matcher  *problemMatcher

    mu       sync.RWMutex
    exitCode int
}

// Run executes a task.
func (e *executor) Run(task *Task, opts RunOptions) (TaskRun, error) {
    // Build command
    cmd := exec.Command(task.Command, append(task.Args, opts.Args...)...)

    // Set working directory
    workDir := task.WorkDir
    if opts.WorkDir != "" {
        workDir = opts.WorkDir
    }
    cmd.Dir = workDir

    // Set environment
    cmd.Env = os.Environ()
    for k, v := range task.Env {
        cmd.Env = append(cmd.Env, k+"="+v)
    }
    for k, v := range opts.Env {
        cmd.Env = append(cmd.Env, k+"="+v)
    }

    // Create run
    run := &taskRun{
        id:       uuid.New().String(),
        task:     task,
        state:    TaskStatePending,
        output:   new(bytes.Buffer),
        exitCode: -1,
    }

    // Set up problem matcher
    if task.ProblemMatcher != "" {
        if matcher, ok := e.matchers[task.ProblemMatcher]; ok {
            run.matcher = newProblemMatcher(matcher)
        }
    }

    // Start process
    proc, err := e.supervisor.Start(task.Name, cmd)
    if err != nil {
        return nil, fmt.Errorf("start task: %w", err)
    }

    run.process = proc
    run.state = TaskStateRunning

    // Track active run
    e.mu.Lock()
    e.active[run.id] = run
    e.mu.Unlock()

    // Emit start event
    if e.eventBus != nil {
        e.eventBus.Publish("task.started", map[string]any{
            "id":   run.id,
            "task": task.Name,
        })
    }

    // Start output processing
    go e.processOutput(run)

    // Wait for completion in background
    if !opts.Background {
        go e.waitForCompletion(run)
    }

    return run, nil
}

// processOutput reads and processes task output.
func (e *executor) processOutput(run *taskRun) {
    scanner := bufio.NewScanner(io.MultiReader(run.process.Stdout, run.process.Stderr))

    for scanner.Scan() {
        line := scanner.Text()

        run.mu.Lock()
        run.output.WriteString(line + "\n")

        // Run problem matcher
        if run.matcher != nil {
            if problem := run.matcher.Match(line); problem != nil {
                problem.Source = run.task.Name
                run.problems = append(run.problems, problem)
            }
        }
        run.mu.Unlock()

        // Emit output event
        if e.eventBus != nil {
            e.eventBus.Publish("task.output", map[string]any{
                "id":   run.id,
                "line": line,
            })
        }
    }
}

// waitForCompletion waits for task to finish.
func (e *executor) waitForCompletion(run *taskRun) {
    <-run.process.Done

    run.mu.Lock()
    run.exitCode = run.process.ExitCode
    if run.exitCode == 0 {
        run.state = TaskStateSucceeded
    } else {
        run.state = TaskStateFailed
    }
    run.mu.Unlock()

    // Move to history
    e.mu.Lock()
    delete(e.active, run.id)
    e.history = append([]*taskRun{run}, e.history...)
    if len(e.history) > 100 {
        e.history = e.history[:100]
    }
    e.mu.Unlock()

    // Emit completion event
    if e.eventBus != nil {
        e.eventBus.Publish("task.completed", map[string]any{
            "id":       run.id,
            "task":     run.task.Name,
            "exitCode": run.exitCode,
            "success":  run.exitCode == 0,
        })
    }
}
```

### 9.4 Problem Matcher

```go
// problemMatcher matches problems in output.
type problemMatcher struct {
    pattern  *regexp.Regexp
    config   *ProblemMatcher

    // Multiline state
    lines []string
}

// newProblemMatcher creates a problem matcher.
func newProblemMatcher(config *ProblemMatcher) *problemMatcher {
    pattern := regexp.MustCompile(config.Pattern)
    return &problemMatcher{
        pattern: pattern,
        config:  config,
    }
}

// Match tries to match a line against the pattern.
func (pm *problemMatcher) Match(line string) *Problem {
    if pm.config.Multiline {
        return pm.matchMultiline(line)
    }

    matches := pm.pattern.FindStringSubmatch(line)
    if matches == nil {
        return nil
    }

    return pm.extractProblem(matches)
}

// extractProblem extracts a problem from regex matches.
func (pm *problemMatcher) extractProblem(matches []string) *Problem {
    problem := &Problem{}

    if pm.config.File > 0 && pm.config.File < len(matches) {
        problem.File = matches[pm.config.File]
    }

    if pm.config.Line > 0 && pm.config.Line < len(matches) {
        problem.Line, _ = strconv.Atoi(matches[pm.config.Line])
    }

    if pm.config.Column > 0 && pm.config.Column < len(matches) {
        problem.Column, _ = strconv.Atoi(matches[pm.config.Column])
    }

    if pm.config.Message > 0 && pm.config.Message < len(matches) {
        problem.Message = matches[pm.config.Message]
    }

    if pm.config.Severity > 0 && pm.config.Severity < len(matches) {
        problem.Severity = parseSeverity(matches[pm.config.Severity])
    } else {
        problem.Severity = ProblemSeverityError
    }

    if pm.config.Code > 0 && pm.config.Code < len(matches) {
        problem.Code = matches[pm.config.Code]
    }

    return problem
}

// DefaultMatchers provides common problem matchers.
var DefaultMatchers = map[string]*ProblemMatcher{
    "go": {
        Pattern: `^(.+):(\d+):(\d+):\s+(.+)$`,
        File:    1,
        Line:    2,
        Column:  3,
        Message: 4,
    },
    "gcc": {
        Pattern: `^(.+):(\d+):(\d+):\s+(warning|error):\s+(.+)$`,
        File:     1,
        Line:     2,
        Column:   3,
        Severity: 4,
        Message:  5,
    },
    "typescript": {
        Pattern: `^(.+)\((\d+),(\d+)\):\s+(error|warning)\s+(\w+):\s+(.+)$`,
        File:     1,
        Line:     2,
        Column:   3,
        Severity: 4,
        Code:     5,
        Message:  6,
    },
    "eslint": {
        Pattern: `^\s*(\d+):(\d+)\s+(error|warning)\s+(.+?)\s+(\S+)$`,
        Line:     1,
        Column:   2,
        Severity: 3,
        Message:  4,
        Code:     5,
    },
}
```

---

## 10. Integration with Editor

### 10.1 Configuration Extensions

Add to `internal/config/sections.go`:

```go
// IntegrationConfig provides type-safe access to integration settings.
type IntegrationConfig struct {
    // Terminal settings (existing TerminalConfig expanded)
    Terminal TerminalIntegrationConfig

    // Git settings
    Git GitIntegrationConfig

    // Debug settings
    Debug DebugIntegrationConfig

    // Task settings
    Task TaskIntegrationConfig
}

// GitIntegrationConfig provides git-specific settings.
type GitIntegrationConfig struct {
    // Enabled enables git integration.
    Enabled bool

    // AutoFetch enables automatic background fetch.
    AutoFetch bool

    // AutoFetchInterval is the auto-fetch interval in seconds.
    AutoFetchInterval int

    // ShowStatusInStatusBar shows git status in status bar.
    ShowStatusInStatusBar bool

    // ShowGutterDecorations shows git gutter decorations.
    ShowGutterDecorations bool

    // DefaultRemote is the default remote for push/pull.
    DefaultRemote string
}

// DebugIntegrationConfig provides debug-specific settings.
type DebugIntegrationConfig struct {
    // Enabled enables debug integration.
    Enabled bool

    // ConfirmOnExit prompts before closing debug session.
    ConfirmOnExit bool

    // OpenDebugConsoleOnStart opens debug console when starting.
    OpenDebugConsoleOnStart bool

    // Adapters are custom debug adapter configurations.
    Adapters map[string]AdapterSettings
}

// TaskIntegrationConfig provides task-specific settings.
type TaskIntegrationConfig struct {
    // Enabled enables task integration.
    Enabled bool

    // AutoDetect enables automatic task detection.
    AutoDetect bool

    // Shell is the shell for running tasks.
    Shell string

    // ProblemMatchers are custom problem matchers.
    ProblemMatchers map[string]ProblemMatcherSettings
}
```

### 10.2 Plugin API Provider

```go
// Provider implements plugin API integration interfaces.
type Provider struct {
    manager *Manager
}

// NewProvider creates an integration provider.
func NewProvider(manager *Manager) *Provider {
    return &Provider{manager: manager}
}

// TerminalProvider implements api.TerminalProvider.
type TerminalProvider struct {
    manager *terminal.Manager
}

func (p *TerminalProvider) Create(opts api.TerminalOptions) (api.Terminal, error) {
    term, err := p.manager.Create(terminal.TerminalOptions{
        Name:    opts.Name,
        Shell:   opts.Shell,
        Env:     opts.Env,
        WorkDir: opts.WorkDir,
    })
    if err != nil {
        return nil, err
    }
    return &terminalAdapter{term: term}, nil
}

// GitProvider implements api.GitProvider.
type GitProvider struct {
    manager *git.Manager
}

func (p *GitProvider) Status() (*api.GitStatus, error) {
    repo, err := p.manager.Discover(".")
    if err != nil {
        return nil, err
    }

    status, err := repo.Status()
    if err != nil {
        return nil, err
    }

    return &api.GitStatus{
        Branch:    status.Branch,
        Ahead:     status.Ahead,
        Behind:    status.Behind,
        Modified:  len(status.Unstaged),
        Staged:    len(status.Staged),
        Untracked: len(status.Untracked),
    }, nil
}

// DebugProvider implements api.DebugProvider.
type DebugProvider struct {
    manager *debug.Manager
}

func (p *DebugProvider) Launch(config api.LaunchConfig) (api.DebugSession, error) {
    session, err := p.manager.Launch(debug.LaunchConfig{
        Name:        config.Name,
        Type:        config.Type,
        Program:     config.Program,
        Args:        config.Args,
        Env:         config.Env,
        StopOnEntry: config.StopOnEntry,
    })
    if err != nil {
        return nil, err
    }
    return &sessionAdapter{session: session}, nil
}

// TaskProvider implements api.TaskProvider.
type TaskProvider struct {
    manager *task.Manager
}

func (p *TaskProvider) List() ([]api.Task, error) {
    tasks, err := p.manager.Discover(".")
    if err != nil {
        return nil, err
    }

    result := make([]api.Task, len(tasks))
    for i, t := range tasks {
        result[i] = api.Task{
            Name:        t.Name,
            Description: t.Description,
            Source:      t.Source,
            Group:       t.Group,
        }
    }
    return result, nil
}

func (p *TaskProvider) Run(name string, args ...string) (api.TaskRun, error) {
    run, err := p.manager.RunByName(name, task.RunOptions{Args: args})
    if err != nil {
        return nil, err
    }
    return &taskRunAdapter{run: run}, nil
}
```

### 10.3 Dispatcher Handlers

```go
// RegisterHandlers registers integration action handlers with the dispatcher.
func RegisterHandlers(d *dispatcher.Dispatcher, m *Manager) {
    // Terminal handlers
    d.Register("terminal.create", createTerminalHandler(m.Terminal()))
    d.Register("terminal.close", closeTerminalHandler(m.Terminal()))
    d.Register("terminal.send", sendTerminalHandler(m.Terminal()))
    d.Register("terminal.paste", pasteTerminalHandler(m.Terminal()))

    // Git handlers
    d.Register("git.status", gitStatusHandler(m.Git()))
    d.Register("git.stage", gitStageHandler(m.Git()))
    d.Register("git.unstage", gitUnstageHandler(m.Git()))
    d.Register("git.commit", gitCommitHandler(m.Git()))
    d.Register("git.push", gitPushHandler(m.Git()))
    d.Register("git.pull", gitPullHandler(m.Git()))
    d.Register("git.checkout", gitCheckoutHandler(m.Git()))
    d.Register("git.diff", gitDiffHandler(m.Git()))
    d.Register("git.blame", gitBlameHandler(m.Git()))

    // Debug handlers
    d.Register("debug.start", debugStartHandler(m.Debug()))
    d.Register("debug.stop", debugStopHandler(m.Debug()))
    d.Register("debug.continue", debugContinueHandler(m.Debug()))
    d.Register("debug.pause", debugPauseHandler(m.Debug()))
    d.Register("debug.stepOver", debugStepOverHandler(m.Debug()))
    d.Register("debug.stepInto", debugStepIntoHandler(m.Debug()))
    d.Register("debug.stepOut", debugStepOutHandler(m.Debug()))
    d.Register("debug.breakpoint.add", addBreakpointHandler(m.Debug()))
    d.Register("debug.breakpoint.remove", removeBreakpointHandler(m.Debug()))
    d.Register("debug.breakpoint.toggle", toggleBreakpointHandler(m.Debug()))

    // Task handlers
    d.Register("task.run", runTaskHandler(m.Task()))
    d.Register("task.stop", stopTaskHandler(m.Task()))
    d.Register("task.list", listTasksHandler(m.Task()))
}

// gitCommitHandler handles git.commit action.
func gitCommitHandler(gm *git.Manager) handler.Handler {
    return func(ctx *execctx.ExecutionContext) handler.Result {
        message := ctx.Args.String("message")
        if message == "" {
            return handler.Error(fmt.Errorf("commit message required"))
        }

        repo, err := gm.Discover(ctx.WorkspaceRoot)
        if err != nil {
            return handler.Error(err)
        }

        commit, err := repo.Commit(message, git.CommitOptions{})
        if err != nil {
            return handler.Error(err)
        }

        return handler.Success().
            WithMessage(fmt.Sprintf("Created commit %s", commit.Hash[:8]))
    }
}

// debugStartHandler handles debug.start action.
func debugStartHandler(dm *debug.Manager) handler.Handler {
    return func(ctx *execctx.ExecutionContext) handler.Result {
        configName := ctx.Args.String("config")

        configs := dm.Configurations()
        var config debug.LaunchConfig

        if configName != "" {
            for _, c := range configs {
                if c.Name == configName {
                    config = c
                    break
                }
            }
            if config.Name == "" {
                return handler.Error(fmt.Errorf("config not found: %s", configName))
            }
        } else if len(configs) > 0 {
            config = configs[0]
        } else {
            return handler.Error(fmt.Errorf("no debug configurations found"))
        }

        session, err := dm.Launch(config)
        if err != nil {
            return handler.Error(err)
        }

        return handler.Success().
            WithData("sessionId", session.ID()).
            WithMessage(fmt.Sprintf("Started debug session: %s", config.Name))
    }
}
```

---

## 11. Implementation Phases

### Phase 1: Core Infrastructure (Foundation)

**Goals:** Process supervision and manager scaffolding.

**Tasks:**
1. Create package structure and documentation
2. Implement process supervisor (`process/supervisor.go`)
   - Process lifecycle management
   - Signal handling
   - Resource cleanup
3. Implement integration manager (`manager.go`)
   - Component lifecycle
   - Configuration integration
   - Event publishing

**Deliverables:**
- `doc.go`, `errors.go`
- `manager.go`, `manager_test.go`
- `process/supervisor.go`, `process/process.go`
- `process/supervisor_test.go`

**Test Criteria:**
- Process supervisor starts/stops processes
- Signal forwarding works
- Resource cleanup on shutdown

### Phase 2: Terminal Integration

**Goals:** Full PTY-based terminal emulation.

**Tasks:**
1. Implement PTY management (`terminal/pty.go`)
   - Unix PTY creation
   - Windows ConPTY support
2. Implement ANSI parser (`terminal/ansi.go`)
   - CSI sequence handling
   - SGR (color/style) support
   - Screen buffer management
3. Implement terminal manager (`terminal/terminal.go`)
   - Multiple terminal instances
   - Resize handling
   - Shell integration
4. Implement screen buffer (`terminal/screen.go`)
   - Cell grid management
   - Cursor tracking
   - Scrollback history

**Deliverables:**
- `terminal/*.go` with tests
- Platform-specific PTY implementations

**Test Criteria:**
- Terminal spawns shell correctly
- ANSI sequences render properly
- Resize works
- Multiple terminals work

### Phase 3: Git Integration - Core

**Goals:** Basic git operations.

**Tasks:**
1. Implement repository management (`git/repository.go`)
   - Open/discover repositories
   - Status tracking
2. Implement working tree operations (`git/worktree.go`)
   - Stage/unstage
   - Discard changes
3. Implement commit operations (`git/commit.go`)
   - Create commits
   - Amend support
4. Implement status caching
   - Efficient status queries
   - Cache invalidation

**Deliverables:**
- `git/git.go`, `git/repository.go`
- `git/worktree.go`, `git/commit.go`
- `git/status.go`

**Test Criteria:**
- Status returns correct info
- Stage/unstage works
- Commits created successfully

### Phase 4: Git Integration - Advanced

**Goals:** Branch, remote, and diff operations.

**Tasks:**
1. Implement branch operations (`git/branch.go`)
   - Create/delete/switch branches
   - Branch listing
2. Implement remote operations (`git/remote.go`)
   - Fetch/pull/push
   - Remote management
3. Implement diff generation (`git/diff.go`)
   - Staged/unstaged diffs
   - Commit diffs
4. Implement blame (`git/blame.go`)
5. Implement authentication (`git/auth.go`)
   - SSH key support
   - Credential helpers

**Deliverables:**
- `git/branch.go`, `git/remote.go`
- `git/diff.go`, `git/blame.go`
- `git/auth.go`

**Test Criteria:**
- Branch operations work
- Push/pull with auth works
- Diffs generated correctly

### Phase 5: Debugger Integration - DAP Client

**Goals:** Debug Adapter Protocol client.

**Tasks:**
1. Implement DAP transport (`debug/dap/transport.go`)
   - Stdio transport
   - Socket transport
2. Implement DAP protocol (`debug/dap/protocol.go`)
   - Message types
   - Request/response handling
3. Implement DAP client (`debug/dap/client.go`)
   - Request sending
   - Event handling
4. Implement session management (`debug/session.go`)
   - Session lifecycle
   - State tracking

**Deliverables:**
- `debug/dap/*.go` with tests
- `debug/session.go`

**Test Criteria:**
- DAP protocol messages correct
- Session connects to adapter
- Events received properly

### Phase 6: Debugger Integration - Features

**Goals:** Full debugging features.

**Tasks:**
1. Implement breakpoint management (`debug/breakpoint.go`)
   - Line/conditional breakpoints
   - Breakpoint persistence
2. Implement variable inspection (`debug/variable.go`)
   - Scope navigation
   - Variable expansion
3. Implement call stack (`debug/stack.go`)
4. Implement adapter configurations (`debug/adapters/`)
   - Go (delve)
   - Node.js
   - Python

**Deliverables:**
- `debug/breakpoint.go`, `debug/variable.go`
- `debug/stack.go`
- `debug/adapters/*.go`

**Test Criteria:**
- Breakpoints hit correctly
- Variables inspectable
- Stack navigation works

### Phase 7: Task Runner - Discovery

**Goals:** Task discovery from multiple sources.

**Tasks:**
1. Implement task discovery framework (`task/discovery.go`)
2. Implement Makefile parser (`task/sources/makefile.go`)
3. Implement package.json parser (`task/sources/package.go`)
4. Implement Taskfile parser (`task/sources/taskfile.go`)
5. Implement keystorm tasks parser (`task/sources/keystorm.go`)

**Deliverables:**
- `task/discovery.go`
- `task/sources/*.go` with tests

**Test Criteria:**
- Tasks discovered from all sources
- Task metadata correct
- Discovery fast enough

### Phase 8: Task Runner - Execution

**Goals:** Task execution and problem matching.

**Tasks:**
1. Implement task executor (`task/executor.go`)
   - Process management
   - Environment handling
2. Implement output processing (`task/output.go`)
   - Stream handling
   - Event emission
3. Implement problem matchers (`task/problem.go`)
   - Pattern matching
   - Problem extraction
4. Implement variable substitution (`task/variable.go`)

**Deliverables:**
- `task/executor.go`, `task/output.go`
- `task/problem.go`, `task/variable.go`

**Test Criteria:**
- Tasks execute correctly
- Output captured
- Problems matched

### Phase 9: Editor Integration

**Goals:** Full integration with Keystorm editor.

**Tasks:**
1. Implement configuration extensions
   - Integration settings
   - Adapter configurations
2. Implement plugin API providers (`provider.go`)
   - All provider interfaces
3. Register dispatcher handlers
   - All integration actions
4. Event bus integration
   - Event publishing
   - Event handling

**Deliverables:**
- Configuration extensions
- `provider.go`
- Dispatcher handlers

**Test Criteria:**
- Actions work end-to-end
- Events flow correctly
- Plugins can access features

### Phase 10: Testing and Polish

**Goals:** Comprehensive testing and documentation.

**Tasks:**
1. Integration tests with real tools
   - Real git repositories
   - Real debug sessions
   - Real task execution
2. Performance optimization
   - Status caching
   - Event debouncing
3. Documentation
   - API documentation
   - User guide
4. Edge case handling
   - Error recovery
   - Resource cleanup

**Deliverables:**
- Integration tests
- Performance benchmarks
- Documentation

---

## 12. Testing Strategy

### 12.1 Unit Tests

Each component should have thorough unit tests:

```go
// terminal_test.go
func TestScreen_WriteRune(t *testing.T) {
    screen := NewScreen(80, 24)
    screen.WriteRune('A')

    if screen.Cells[0][0].Rune != 'A' {
        t.Error("expected 'A' at position 0,0")
    }
}

func TestAnsiParser_CursorMovement(t *testing.T) {
    screen := NewScreen(80, 24)
    parser := newAnsiParser()

    parser.Parse([]byte("\x1b[10;20H"), screen, nil)

    if screen.CursorX != 19 || screen.CursorY != 9 {
        t.Errorf("expected cursor at 19,9, got %d,%d", screen.CursorX, screen.CursorY)
    }
}
```

### 12.2 Mock Implementations

```go
// testing/mock_git.go
type MockRepository struct {
    StatusFunc    func() (*Status, error)
    CommitFunc    func(string, CommitOptions) (*Commit, error)
    StageFunc     func(...string) error
}

func (m *MockRepository) Status() (*Status, error) {
    if m.StatusFunc != nil {
        return m.StatusFunc()
    }
    return &Status{Branch: "main"}, nil
}

// testing/mock_dap.go
type MockDAPTransport struct {
    SendFunc    func(dap.Message) error
    ReceiveFunc func() (dap.Message, error)
    messages    chan dap.Message
}
```

### 12.3 Integration Tests

```go
// integration_test.go
func TestGit_RealRepository(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Create temp directory with git repo
    dir := t.TempDir()
    cmd := exec.Command("git", "init", dir)
    if err := cmd.Run(); err != nil {
        t.Fatalf("git init: %v", err)
    }

    manager := git.NewManager(nil)
    repo, err := manager.Open(dir)
    if err != nil {
        t.Fatalf("open: %v", err)
    }

    status, err := repo.Status()
    if err != nil {
        t.Fatalf("status: %v", err)
    }

    if status.Branch != "main" && status.Branch != "master" {
        t.Errorf("unexpected branch: %s", status.Branch)
    }
}
```

### 12.4 Test Matrix

| Test Type | Coverage Target |
|-----------|----------------|
| Unit | 80%+ line coverage |
| Integration | Core workflows |
| Mock | Protocol compliance |
| Benchmark | Performance regression |
| Fuzzing | ANSI parser robustness |

---

## 13. Performance Considerations

### 13.1 Latency Targets

| Operation | Target | Approach |
|-----------|--------|----------|
| Terminal keystroke | <10ms | Direct PTY write |
| Git status | <100ms | Caching, incremental |
| Debug step | <50ms | Async protocol |
| Task start | <200ms | Process pooling |

### 13.2 Optimization Strategies

1. **Caching**
   - Git status caching with invalidation
   - Task discovery caching
   - Breakpoint state caching

2. **Async Operations**
   - Non-blocking PTY reads
   - Background git fetch
   - Async debug protocol

3. **Resource Management**
   - Terminal scrollback limits
   - Output buffer limits
   - Process limits

4. **Debouncing**
   - Git status queries
   - File change events
   - Problem matcher updates

### 13.3 Benchmarks

```go
func BenchmarkTerminal_Write(b *testing.B) {
    term, _ := NewPTYTerminal(TerminalOptions{
        Shell: "/bin/sh",
        Cols:  80,
        Rows:  24,
    }, nil)
    defer term.Close()

    data := []byte("hello world\n")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        term.Write(data)
    }
}

func BenchmarkGit_Status(b *testing.B) {
    manager := git.NewManager(nil)
    repo, _ := manager.Open(".")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        repo.Status()
    }
}

func BenchmarkProblemMatcher_Match(b *testing.B) {
    matcher := newProblemMatcher(DefaultMatchers["go"])
    line := "/path/to/file.go:42:10: undefined: foo"

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        matcher.Match(line)
    }
}
```

---

## Appendix A: Default Task Configurations

```toml
# .keystorm/tasks.toml

[tasks.build]
description = "Build the project"
command = "go"
args = ["build", "./..."]
group = "build"
problem_matcher = "go"

[tasks.test]
description = "Run tests"
command = "go"
args = ["test", "-v", "./..."]
group = "test"
problem_matcher = "go"

[tasks.lint]
description = "Run linter"
command = "golangci-lint"
args = ["run"]
group = "build"

[tasks.run]
description = "Run the application"
command = "go"
args = ["run", "."]
group = "run"
```

---

## Appendix B: Debug Launch Configurations

```toml
# .keystorm/launch.toml

[[configurations]]
name = "Debug Main"
type = "go"
request = "launch"
program = "${workspaceFolder}"
args = []

[[configurations]]
name = "Debug Tests"
type = "go"
request = "launch"
mode = "test"
program = "${workspaceFolder}"

[[configurations]]
name = "Attach to Process"
type = "go"
request = "attach"
processId = "${command:pickProcess}"
```

---

## Appendix C: Dependencies

```go
// go.mod additions
require (
    github.com/creack/pty v1.1.21          // PTY for Unix
    github.com/go-git/go-git/v5 v5.11.0    // Git operations
    github.com/google/go-dap v0.11.0       // DAP types
    github.com/fsnotify/fsnotify v1.7.0    // File watching (for tasks)
    github.com/pelletier/go-toml/v2 v2.1.1 // Task config parsing
    gopkg.in/yaml.v3 v3.0.1                // Taskfile parsing
)
```

---

## Summary

This implementation plan provides a comprehensive roadmap for building the Integration Layer in Keystorm. The 10-phase approach ensures incremental delivery while maintaining code quality. Key architectural decisions prioritize:

1. **Clean abstraction** - Unified Manager facade hides tool complexity
2. **Multi-tool support** - Terminal, Git, Debugger, Task Runner as separate components
3. **Protocol compliance** - DAP for debuggers, proper PTY for terminals
4. **Reliability** - Process supervision, crash recovery, resource cleanup
5. **Performance** - Caching, async operations, debouncing
6. **Extensibility** - Plugin API integration, custom configurations

The Integration Layer transforms Keystorm from a text editor into a full development environment, providing the "mini IDE" capabilities described in the architecture specification.
