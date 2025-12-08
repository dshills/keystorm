package api

import (
	"fmt"
	"sync"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// Module represents a Lua API module that can be registered with the plugin system.
type Module interface {
	// Name returns the module name (e.g., "buf", "cursor", "mode").
	Name() string

	// RequiredCapability returns the capability required to use this module.
	// Returns empty string if no capability is required.
	RequiredCapability() security.Capability

	// Register registers the module functions into the Lua state.
	// The module should register itself under _ks_<name> global.
	Register(L *lua.LState) error
}

// Registry manages API modules and their registration.
type Registry struct {
	mu      sync.RWMutex
	modules map[string]Module
}

// NewRegistry creates a new API registry.
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]Module),
	}
}

// Register adds a module to the registry.
func (r *Registry) Register(mod Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[mod.Name()]; exists {
		return fmt.Errorf("module %q already registered", mod.Name())
	}

	r.modules[mod.Name()] = mod
	return nil
}

// Get returns a module by name.
func (r *Registry) Get(name string) (Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mod, ok := r.modules[name]
	return mod, ok
}

// List returns all registered module names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.modules))
	for name := range r.modules {
		names = append(names, name)
	}
	return names
}

// InjectAll registers all modules into the Lua state, checking capabilities.
// If checker is nil, only modules with no required capability will be injected.
func (r *Registry) InjectAll(L *lua.LState, checker *security.PermissionChecker) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, mod := range r.modules {
		// Check if plugin has required capability
		reqCap := mod.RequiredCapability()
		if reqCap != "" {
			// If no checker provided, skip modules that require capabilities
			if checker == nil || !checker.HasCapability(reqCap) {
				continue
			}
		}

		if err := mod.Register(L); err != nil {
			return fmt.Errorf("failed to register module %q: %w", name, err)
		}
	}

	// Install the ks module loader
	if err := installKSLoader(L); err != nil {
		return fmt.Errorf("failed to install ks loader: %w", err)
	}

	return nil
}

// Inject registers specific modules into the Lua state.
// Unlike InjectAll, this returns an error if a module requires a capability
// that the checker doesn't have (or if checker is nil and capability is required).
func (r *Registry) Inject(L *lua.LState, checker *security.PermissionChecker, moduleNames ...string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range moduleNames {
		mod, ok := r.modules[name]
		if !ok {
			return fmt.Errorf("module %q not found", name)
		}

		// Check capability
		reqCap := mod.RequiredCapability()
		if reqCap != "" {
			if checker == nil {
				return fmt.Errorf("plugin lacks capability %q for module %q (no permission checker)", reqCap, name)
			}
			if !checker.HasCapability(reqCap) {
				return fmt.Errorf("plugin lacks capability %q for module %q", reqCap, name)
			}
		}

		if err := mod.Register(L); err != nil {
			return fmt.Errorf("failed to register module %q: %w", name, err)
		}
	}

	return nil
}

// installKSLoader installs the ks module that aggregates all API modules.
// Plugins use: local ks = require("ks")
func installKSLoader(L *lua.LState) error {
	// Create the ks module table
	ksModule := L.NewTable()

	// Collect all _ks_* globals into the ks table.
	// Only modules that were successfully registered (based on capability checks) will have
	// their _ks_* global set, so this effectively respects capability restrictions.
	moduleNames := []string{"buf", "cursor", "mode", "util", "keymap", "command", "event", "config", "ui", "lsp", "project", "integration"}
	for _, name := range moduleNames {
		globalName := "_ks_" + name
		val := L.GetGlobal(globalName)
		if val != lua.LNil {
			L.SetField(ksModule, name, val)
			// Clean up internal global
			L.SetGlobal(globalName, lua.LNil)
		}
	}

	// Add version info
	L.SetField(ksModule, "version", lua.LString("1.0.0"))
	L.SetField(ksModule, "api_version", lua.LNumber(1))

	// Register as preloaded module so require("ks") works
	L.PreloadModule("ks", func(L *lua.LState) int {
		L.Push(ksModule)
		return 1
	})

	return nil
}

// DefaultRegistry creates a registry with all standard modules registered.
// Returns an error if any module registration fails (which should never happen
// with standard modules unless there's a programming error).
func DefaultRegistry(ctx *Context) (*Registry, error) {
	r := NewRegistry()

	// Register core modules
	modules := []Module{
		NewBufferModule(ctx),
		NewCursorModule(ctx),
		NewModeModule(ctx),
		NewUtilModule(),
	}

	for _, mod := range modules {
		if err := r.Register(mod); err != nil {
			return nil, fmt.Errorf("failed to register module %q: %w", mod.Name(), err)
		}
	}

	return r, nil
}

// Context provides access to editor state for API modules.
// This is passed to modules during construction so they can access
// the editor's buffer, cursor, mode, etc.
type Context struct {
	// Buffer provides buffer operations.
	// This is typically the engine or a buffer interface.
	Buffer BufferProvider

	// Cursor provides cursor operations.
	Cursor CursorProvider

	// Mode provides mode operations.
	Mode ModeProvider

	// Keymap provides keymap operations.
	Keymap KeymapProvider

	// Command provides command/palette operations.
	Command CommandProvider

	// Event provides event bus operations.
	Event EventProvider

	// UI provides UI operations (notifications, statusline, etc.).
	UI UIProvider

	// Config provides configuration operations.
	Config ConfigProvider

	// LSP provides language server protocol operations (completions, diagnostics, etc.).
	LSP LSPProvider

	// Integration provides access to integration layer features (git, debug, tasks).
	Integration IntegrationProvider

	// LuaExecutor provides thread-safe execution of Lua callbacks.
	// All Lua callback invocations from providers (Event, Command, Config) must
	// go through this executor to ensure thread safety.
	//
	// gopher-lua's LState is NOT goroutine-safe. The executor serializes all
	// Lua operations through a single worker goroutine.
	LuaExecutor LuaExecutorProvider
}

// LuaExecutorProvider defines the interface for thread-safe Lua execution.
// This allows callbacks from arbitrary goroutines to safely execute Lua code.
type LuaExecutorProvider interface {
	// ExecuteAsync queues a Lua operation for execution without waiting.
	// The function fn will be called on the Lua state's owning goroutine.
	// Returns an error if the executor is closed or the queue is full.
	ExecuteAsync(fn func(L interface{}) error) error
}

// BufferProvider defines the interface for buffer operations.
type BufferProvider interface {
	// Text returns the full buffer text.
	Text() string

	// TextRange returns text in the given byte range.
	TextRange(start, end int) (string, error)

	// Line returns the text of a specific line (1-indexed).
	Line(lineNum int) (string, error)

	// LineCount returns the total number of lines.
	LineCount() int

	// Len returns the buffer length in bytes.
	Len() int

	// Insert inserts text at the given byte offset.
	// Returns the end offset after insertion.
	Insert(offset int, text string) (int, error)

	// Delete deletes text in the given byte range.
	Delete(start, end int) error

	// Replace replaces text in the given byte range.
	// Returns the end offset after replacement.
	Replace(start, end int, text string) (int, error)

	// Undo undoes the last change.
	Undo() bool

	// Redo redoes the last undone change.
	Redo() bool

	// Path returns the file path of the buffer.
	Path() string

	// Modified returns true if the buffer has unsaved changes.
	Modified() bool
}

// CursorProvider defines the interface for cursor operations.
type CursorProvider interface {
	// Get returns the primary cursor offset.
	Get() int

	// GetAll returns all cursor offsets (for multi-cursor).
	GetAll() []int

	// Set sets the primary cursor position.
	Set(offset int) error

	// Add adds a secondary cursor.
	Add(offset int) error

	// Clear clears all secondary cursors.
	Clear()

	// Selection returns the selection range, or (-1, -1) if no selection.
	Selection() (start, end int)

	// SetSelection sets the selection range.
	SetSelection(start, end int) error

	// Count returns the number of cursors.
	Count() int

	// Line returns the current line number (1-indexed).
	Line() int

	// Column returns the current column number (1-indexed).
	Column() int
}

// ModeProvider defines the interface for mode operations.
type ModeProvider interface {
	// Current returns the current mode name.
	Current() string

	// Switch switches to a different mode.
	Switch(mode string) error

	// Is checks if currently in the given mode.
	Is(mode string) bool
}

// IntegrationProvider defines the interface for integration layer operations.
// It provides access to git, debugger, task runner, and terminal features.
type IntegrationProvider interface {
	// WorkspaceRoot returns the workspace root directory.
	WorkspaceRoot() string

	// Health returns the integration layer health status.
	Health() IntegrationHealth

	// Git returns the git provider, or nil if not available.
	Git() GitProvider

	// Debug returns the debug provider, or nil if not available.
	Debug() DebugProvider

	// Task returns the task provider, or nil if not available.
	Task() TaskProvider
}

// IntegrationHealth represents the health status of the integration layer.
type IntegrationHealth struct {
	Status        string            // "healthy", "degraded", "unhealthy"
	Uptime        int64             // Uptime in milliseconds
	ProcessCount  int               // Number of active child processes
	WorkspaceRoot string            // Configured workspace root
	Components    map[string]string // Component name -> status
}

// GitProvider defines the interface for git operations.
type GitProvider interface {
	// Status returns the current git status.
	Status() (GitStatus, error)

	// Branch returns the current branch name.
	Branch() (string, error)

	// Branches lists all branches.
	Branches() ([]string, error)

	// Commit creates a commit with the given message.
	Commit(message string) error

	// Add stages files for commit.
	Add(paths []string) error

	// Diff returns the diff for staged or unstaged changes.
	Diff(staged bool) (string, error)
}

// GitStatus represents the current git repository status.
type GitStatus struct {
	Branch       string
	Ahead        int
	Behind       int
	Staged       []string
	Modified     []string
	Untracked    []string
	HasConflicts bool
	IsClean      bool
}

// DebugProvider defines the interface for debugger operations.
type DebugProvider interface {
	// Start starts a debug session with the given configuration.
	Start(config DebugConfig) (string, error)

	// Stop stops the debug session with the given ID.
	Stop(sessionID string) error

	// Sessions lists active debug sessions.
	Sessions() []DebugSession

	// SetBreakpoint sets a breakpoint at the given location.
	SetBreakpoint(file string, line int) (string, error)

	// RemoveBreakpoint removes a breakpoint by ID.
	RemoveBreakpoint(id string) error

	// Continue continues execution in the session.
	Continue(sessionID string) error

	// StepOver steps over the current line.
	StepOver(sessionID string) error

	// StepInto steps into the current function call.
	StepInto(sessionID string) error

	// StepOut steps out of the current function.
	StepOut(sessionID string) error

	// Variables returns variables in the current scope.
	Variables(sessionID string) ([]DebugVariable, error)
}

// DebugConfig configures a debug session.
type DebugConfig struct {
	Adapter     string            // Debug adapter name (e.g., "delve", "node")
	Program     string            // Program or script to debug
	Args        []string          // Arguments to pass to the program
	Env         map[string]string // Environment variables
	Cwd         string            // Working directory
	StopOnEntry bool              // Stop at program entry
}

// DebugSession represents an active debug session.
type DebugSession struct {
	ID      string
	Adapter string
	Program string
	State   string // "running", "paused", "stopped"
}

// DebugVariable represents a variable in the debugger.
type DebugVariable struct {
	Name  string
	Value string
	Type  string
}

// TaskProvider defines the interface for task runner operations.
type TaskProvider interface {
	// List returns all available tasks.
	List() ([]TaskInfo, error)

	// Run starts a task by name.
	Run(name string) (string, error)

	// Stop stops a running task by ID.
	Stop(taskID string) error

	// Status returns the status of a task.
	Status(taskID string) (TaskStatus, error)

	// Output returns the output of a task.
	Output(taskID string) (string, error)
}

// TaskInfo describes an available task.
type TaskInfo struct {
	Name        string
	Source      string // Source of the task (e.g., "Makefile", "package.json")
	Description string
	Command     string
}

// TaskStatus represents the current status of a task.
type TaskStatus struct {
	ID        string
	Name      string
	State     string // "pending", "running", "completed", "failed"
	ExitCode  int
	StartTime int64 // Unix timestamp in milliseconds
	EndTime   int64 // Unix timestamp in milliseconds, 0 if still running
}
