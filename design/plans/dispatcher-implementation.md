# Keystorm Command/Action Dispatcher - Implementation Plan

## Comprehensive Design Document for `internal/dispatcher`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Requirements Analysis](#2-requirements-analysis)
3. [Architecture Overview](#3-architecture-overview)
4. [Package Structure](#4-package-structure)
5. [Core Types and Interfaces](#5-core-types-and-interfaces)
6. [Handler Registry System](#6-handler-registry-system)
7. [Action Routing](#7-action-routing)
8. [Handler Implementations](#8-handler-implementations)
9. [State Management](#9-state-management)
10. [Error Handling and Recovery](#10-error-handling-and-recovery)
11. [Hook System](#11-hook-system)
12. [Implementation Phases](#12-implementation-phases)
13. [Testing Strategy](#13-testing-strategy)
14. [Performance Considerations](#14-performance-considerations)

---

## 1. Executive Summary

The Command/Action Dispatcher is the central routing hub that connects user input to editor operations. It receives **Actions** from the Input Engine and executes them against the appropriate editor subsystems (Engine, Mode Manager, Renderer, LSP, etc.).

### Role in the Architecture

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   Input Engine  │─────▶│   Dispatcher    │─────▶│  Text Engine    │
│  (internal/     │      │  (internal/     │      │  (internal/     │
│   input)        │      │   dispatcher)   │      │   engine)       │
└─────────────────┘      └────────┬────────┘      └─────────────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    ▼             ▼             ▼
            ┌───────────┐  ┌───────────┐  ┌───────────┐
            │   Mode    │  │  Renderer │  │    LSP    │
            │  Manager  │  │           │  │           │
            └───────────┘  └───────────┘  └───────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Handler Registry Pattern | Enables extensibility; plugins can register custom action handlers |
| Action Namespace Routing | Fast O(1) lookup via prefix map (`cursor.*`, `editor.*`, `mode.*`) |
| Stateless Handlers | Handlers are pure functions; state lives in subsystems (Engine, Cursors) |
| Pre/Post Hook System | Enables plugins, audit trails, and AI context building |
| Result Feedback | Handlers return results for status display and error handling |
| Async Support | Long-running operations (search, LSP) don't block the main loop |

### Integration Points

The Dispatcher connects to:
- **Input Engine**: Receives `Action` from `InputHandler.Actions()` channel
- **Text Engine**: Executes buffer operations (insert, delete, etc.)
- **Cursor Manager**: Updates cursor/selection positions
- **Mode Manager**: Handles mode transitions
- **History**: Records operations for undo/redo
- **Renderer**: Triggers view updates (scroll, center cursor)
- **LSP Client**: Forwards LSP-related actions
- **Event Bus**: Publishes action events for plugins/extensions
- **AI Orchestration**: Routes AI-related actions to workflow system

---

## 2. Requirements Analysis

### 2.1 From Architecture Specification

From `design/specs/architecture.md`:

> "Once you press a key or pick a command, someone must decide: 'What does that mean right now?' This dispatcher matches input → action, often with context: mode, selection, active file type, extension overrides, etc."

### 2.2 Functional Requirements

1. **Action Routing**
   - Route actions by name to appropriate handlers
   - Support namespaced actions (`cursor.moveDown`, `editor.save`)
   - Handle unknown actions gracefully

2. **Handler Management**
   - Register/unregister action handlers
   - Support handler priority for overrides
   - Enable plugin-registered handlers

3. **Context Awareness**
   - Pass context to handlers (mode, selection, file type)
   - Support conditional execution based on state
   - Handle repeat counts (`5j` = move down 5 times)

4. **State Coordination**
   - Update cursors after text operations
   - Maintain mode state
   - Track modifications for undo grouping

5. **Error Handling**
   - Catch and report handler errors
   - Prevent partial state corruption
   - Support recovery strategies

6. **Extensibility**
   - Hook system for pre/post action processing
   - Plugin handler registration
   - Custom action namespaces

### 2.3 Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Action dispatch latency | < 1ms for simple actions |
| Handler lookup | O(1) via namespace prefix |
| Memory overhead | < 100KB for handler registry |
| Thread safety | Safe for concurrent action dispatch |

---

## 3. Architecture Overview

### 3.1 Component Diagram

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              Dispatcher                                       │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐   │
│  │   Handler        │  │   Action         │  │      Hook                │   │
│  │   Registry       │  │   Router         │  │      Manager             │   │
│  └──────────────────┘  └──────────────────┘  └──────────────────────────┘   │
│                                                                              │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐   │
│  │   Context        │  │   Result         │  │      Error               │   │
│  │   Builder        │  │   Handler        │  │      Handler             │   │
│  └──────────────────┘  └──────────────────┘  └──────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                        Handler Implementations                        │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │   │
│  │  │ Cursor   │ │ Editor   │ │  Mode    │ │ Search   │ │  View    │   │   │
│  │  │ Handlers │ │ Handlers │ │ Handlers │ │ Handlers │ │ Handlers │   │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │   │
│  │  │  File    │ │ Window   │ │ Macro    │ │   LSP    │ │   AI     │   │   │
│  │  │ Handlers │ │ Handlers │ │ Handlers │ │ Handlers │ │ Handlers │   │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│                          External Dependencies                                │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │  Engine  │ │ Cursors  │ │  Mode    │ │ History  │ │ Renderer │          │
│  │          │ │          │ │ Manager  │ │          │ │          │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Data Flow

```
Action (from Input)
       │
       ▼
┌──────────────────┐
│ Pre-Dispatch     │  Run pre-hooks (audit, AI context, plugin intercept)
│ Hooks            │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Action Router    │  Extract namespace, look up handler
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Context Builder  │  Build execution context from editor state
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Handler          │  Execute handler with action + context
│ Execution        │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Result           │  Process handler result (cursor update, error)
│ Processing       │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Post-Dispatch    │  Run post-hooks (audit trail, state sync)
│ Hooks            │
└────────┬─────────┘
         │
         ▼
Result (status, errors)
```

---

## 4. Package Structure

```
internal/dispatcher/
    doc.go                      # Package documentation

    # Core dispatcher
    dispatcher.go               # Main Dispatcher type and dispatch loop
    config.go                   # Configuration options
    errors.go                   # Error types

    # Handler system
    handler/
        handler.go              # Handler interface and types
        registry.go             # HandlerRegistry
        router.go               # ActionRouter (namespace-based routing)
        result.go               # HandlerResult type
        handler_test.go

    # Execution context
    context/
        context.go              # ExecutionContext type
        builder.go              # ContextBuilder
        context_test.go

    # Hook system
    hook/
        hook.go                 # Hook interface
        manager.go              # HookManager
        builtin.go              # Built-in hooks (audit, repeat)
        hook_test.go

    # Handler implementations by namespace
    handlers/
        cursor/
            cursor.go           # Cursor movement handlers
            motion.go           # Motion handlers (word, line, etc.)
            cursor_test.go

        editor/
            editor.go           # Text editing handlers
            insert.go           # Insert/append handlers
            delete.go           # Delete handlers
            change.go           # Change handlers
            yank.go             # Yank/paste handlers
            indent.go           # Indent/outdent handlers
            editor_test.go

        mode/
            mode.go             # Mode switching handlers
            mode_test.go

        search/
            search.go           # Search handlers
            replace.go          # Search and replace
            search_test.go

        view/
            view.go             # View/scroll handlers
            scroll.go           # Scroll operations
            view_test.go

        file/
            file.go             # File operations (save, open)
            buffer.go           # Buffer management
            file_test.go

        window/
            window.go           # Window/split handlers
            window_test.go

        completion/
            completion.go       # Completion handlers
            completion_test.go

        macro/
            macro.go            # Macro handlers
            macro_test.go

        lsp/
            lsp.go              # LSP action handlers
            lsp_test.go

        ai/
            ai.go               # AI workflow action handlers
            ai_test.go

        palette/
            palette.go          # Command palette handlers
            palette_test.go

        operator/
            operator.go         # Operator handlers (delete, change, yank)
            operator_test.go

    # Integration
    integration.go              # DispatcherSystem (unified interface)

    # Tests
    dispatcher_test.go
    integration_test.go
    bench_test.go
```

### Rationale

- **Handler isolation**: Each namespace in its own subpackage for maintainability
- **Clear interfaces**: Handler and Hook interfaces enable extensibility
- **Testability**: Each handler package can be tested in isolation
- **Separation of concerns**: Core dispatcher logic separate from handlers

---

## 5. Core Types and Interfaces

### 5.1 Handler Interface

```go
// internal/dispatcher/handler/handler.go

// Handler processes a specific action or set of actions.
type Handler interface {
    // Handle executes the action and returns a result.
    Handle(action input.Action, ctx *ExecutionContext) Result

    // CanHandle returns true if this handler can process the action.
    CanHandle(actionName string) bool

    // Priority returns the handler priority (higher = checked first).
    Priority() int
}

// HandlerFunc is a function adapter for Handler interface.
type HandlerFunc func(action input.Action, ctx *ExecutionContext) Result

func (f HandlerFunc) Handle(action input.Action, ctx *ExecutionContext) Result {
    return f(action, ctx)
}

func (f HandlerFunc) CanHandle(actionName string) bool {
    return true // Caller must ensure correct routing
}

func (f HandlerFunc) Priority() int {
    return 0
}

// SimpleHandler wraps a function with explicit action name.
type SimpleHandler struct {
    ActionName string
    Fn         func(action input.Action, ctx *ExecutionContext) Result
    Prio       int
}

func (h *SimpleHandler) Handle(action input.Action, ctx *ExecutionContext) Result {
    return h.Fn(action, ctx)
}

func (h *SimpleHandler) CanHandle(actionName string) bool {
    return actionName == h.ActionName
}

func (h *SimpleHandler) Priority() int {
    return h.Prio
}
```

### 5.2 Handler Result

```go
// internal/dispatcher/handler/result.go

// Result represents the outcome of handling an action.
type Result struct {
    // Status indicates the result status.
    Status ResultStatus

    // Error contains any error that occurred.
    Error error

    // Message is an optional status message for display.
    Message string

    // Edits contains text edits that were applied.
    Edits []Edit

    // CursorDelta indicates how the cursor moved.
    CursorDelta CursorDelta

    // ModeChange indicates a mode transition.
    ModeChange string

    // ViewUpdate indicates required view updates.
    ViewUpdate ViewUpdate

    // Data holds handler-specific return data.
    Data map[string]interface{}
}

// ResultStatus indicates the outcome of an action.
type ResultStatus uint8

const (
    // StatusOK indicates successful execution.
    StatusOK ResultStatus = iota
    // StatusNoOp indicates the action had no effect.
    StatusNoOp
    // StatusError indicates an error occurred.
    StatusError
    // StatusAsync indicates the operation is running asynchronously.
    StatusAsync
    // StatusCancelled indicates the operation was cancelled.
    StatusCancelled
)

// Edit represents a text edit for result tracking.
type Edit struct {
    Range   Range
    NewText string
    OldText string
}

// CursorDelta describes cursor position change.
type CursorDelta struct {
    BytesDelta  int64  // Change in byte offset
    LinesDelta  int    // Change in line number
    ColumnDelta int    // Change in column
}

// ViewUpdate describes required view updates.
type ViewUpdate struct {
    ScrollTo    *ScrollTarget
    CenterLine  *uint32
    Redraw      bool
    RedrawLines []uint32
}

// ScrollTarget specifies a scroll destination.
type ScrollTarget struct {
    Line   uint32
    Column uint32
    Center bool
}

// Success creates a successful result.
func Success() Result {
    return Result{Status: StatusOK}
}

// SuccessWithMessage creates a successful result with a message.
func SuccessWithMessage(msg string) Result {
    return Result{Status: StatusOK, Message: msg}
}

// NoOp creates a no-operation result.
func NoOp() Result {
    return Result{Status: StatusNoOp}
}

// Error creates an error result.
func Error(err error) Result {
    return Result{Status: StatusError, Error: err}
}

// Errorf creates an error result with a formatted message.
func Errorf(format string, args ...interface{}) Result {
    return Result{
        Status: StatusError,
        Error:  fmt.Errorf(format, args...),
    }
}

// WithModeChange adds a mode change to the result.
func (r Result) WithModeChange(mode string) Result {
    r.ModeChange = mode
    return r
}

// WithViewUpdate adds a view update to the result.
func (r Result) WithViewUpdate(vu ViewUpdate) Result {
    r.ViewUpdate = vu
    return r
}
```

### 5.3 Execution Context

```go
// internal/dispatcher/context/context.go

// ExecutionContext provides context for action execution.
type ExecutionContext struct {
    // Engine provides access to the text buffer.
    Engine EngineInterface

    // Cursors provides access to cursor/selection state.
    Cursors CursorManagerInterface

    // ModeManager provides mode state.
    ModeManager ModeManagerInterface

    // History provides undo/redo.
    History HistoryInterface

    // Renderer provides view operations.
    Renderer RendererInterface

    // Input provides the input context.
    Input *input.Context

    // Buffer metadata.
    FilePath string
    FileType string

    // Execution options.
    Count int  // Repeat count (1 if not specified)
    DryRun bool // If true, don't apply changes (for preview)
}

// EngineInterface abstracts the text engine.
type EngineInterface interface {
    // Text operations
    Insert(offset buffer.ByteOffset, text string) (buffer.EditResult, error)
    Delete(start, end buffer.ByteOffset) (buffer.EditResult, error)
    Replace(start, end buffer.ByteOffset, text string) (buffer.EditResult, error)

    // Read operations
    Text() string
    TextRange(start, end buffer.ByteOffset) string
    LineText(line uint32) string
    Len() buffer.ByteOffset
    LineCount() uint32

    // Position conversion
    OffsetToPoint(offset buffer.ByteOffset) buffer.Point
    PointToOffset(point buffer.Point) buffer.ByteOffset

    // Snapshotting
    Snapshot() EngineReader
    RevisionID() RevisionID
}

// CursorManagerInterface abstracts cursor management.
type CursorManagerInterface interface {
    // Primary cursor
    Primary() cursor.Selection
    SetPrimary(sel cursor.Selection)

    // Multi-cursor
    All() []cursor.Selection
    Add(sel cursor.Selection)
    Clear()
    Count() int

    // Transformations
    TransformAfterEdit(edit buffer.Edit)
}

// ModeManagerInterface abstracts mode management.
type ModeManagerInterface interface {
    Current() mode.Mode
    Switch(name string) error
    Push(name string) error
    Pop() error
}

// HistoryInterface abstracts undo/redo.
type HistoryInterface interface {
    BeginGroup(name string)
    EndGroup()
    Push(cmd history.Command)
    Undo() error
    Redo() error
    CanUndo() bool
    CanRedo() bool
}

// RendererInterface abstracts rendering.
type RendererInterface interface {
    ScrollTo(line, col uint32)
    CenterOnLine(line uint32)
    Redraw()
    RedrawLines(lines []uint32)
}

// NewExecutionContext creates a new execution context.
func NewExecutionContext(
    engine EngineInterface,
    cursors CursorManagerInterface,
    modeManager ModeManagerInterface,
    history HistoryInterface,
    renderer RendererInterface,
    inputCtx *input.Context,
) *ExecutionContext {
    count := 1
    if inputCtx != nil && inputCtx.PendingCount > 0 {
        count = inputCtx.PendingCount
    }

    return &ExecutionContext{
        Engine:      engine,
        Cursors:     cursors,
        ModeManager: modeManager,
        History:     history,
        Renderer:    renderer,
        Input:       inputCtx,
        Count:       count,
    }
}
```

### 5.4 Dispatcher Type

```go
// internal/dispatcher/dispatcher.go

// Dispatcher routes actions to handlers and coordinates execution.
type Dispatcher struct {
    mu sync.RWMutex

    // Core components
    registry    *handler.Registry
    router      *handler.Router
    hookManager *hook.Manager

    // Editor subsystems
    engine      EngineInterface
    cursors     CursorManagerInterface
    modeManager ModeManagerInterface
    history     HistoryInterface
    renderer    RendererInterface

    // Configuration
    config Config

    // Metrics
    metrics *Metrics

    // Action channel for async dispatch
    actionChan chan input.Action

    // Result channel for async results
    resultChan chan handler.Result

    // Shutdown
    done chan struct{}
}

// Config configures the dispatcher.
type Config struct {
    // AsyncDispatch enables asynchronous action dispatch.
    AsyncDispatch bool

    // ActionBufferSize is the size of the action channel buffer.
    ActionBufferSize int

    // EnableMetrics enables dispatch metrics collection.
    EnableMetrics bool

    // RecoverFromPanic enables panic recovery in handlers.
    RecoverFromPanic bool

    // DefaultTimeout is the default handler timeout.
    DefaultTimeout time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
    return Config{
        AsyncDispatch:    false,
        ActionBufferSize: 100,
        EnableMetrics:    true,
        RecoverFromPanic: true,
        DefaultTimeout:   5 * time.Second,
    }
}

// New creates a new dispatcher.
func New(config Config) *Dispatcher {
    d := &Dispatcher{
        registry:    handler.NewRegistry(),
        router:      handler.NewRouter(),
        hookManager: hook.NewManager(),
        config:      config,
        done:        make(chan struct{}),
    }

    if config.AsyncDispatch {
        d.actionChan = make(chan input.Action, config.ActionBufferSize)
        d.resultChan = make(chan handler.Result, config.ActionBufferSize)
    }

    if config.EnableMetrics {
        d.metrics = NewMetrics()
    }

    // Register built-in handlers
    d.registerBuiltinHandlers()

    return d
}

// SetEngine sets the text engine.
func (d *Dispatcher) SetEngine(engine EngineInterface) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.engine = engine
}

// SetCursors sets the cursor manager.
func (d *Dispatcher) SetCursors(cursors CursorManagerInterface) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.cursors = cursors
}

// SetModeManager sets the mode manager.
func (d *Dispatcher) SetModeManager(modeManager ModeManagerInterface) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.modeManager = modeManager
}

// SetHistory sets the history/undo manager.
func (d *Dispatcher) SetHistory(history HistoryInterface) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.history = history
}

// SetRenderer sets the renderer.
func (d *Dispatcher) SetRenderer(renderer RendererInterface) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.renderer = renderer
}

// Dispatch executes an action synchronously.
func (d *Dispatcher) Dispatch(action input.Action) handler.Result {
    return d.dispatchInternal(action, nil)
}

// DispatchWithContext executes an action with explicit input context.
func (d *Dispatcher) DispatchWithContext(action input.Action, inputCtx *input.Context) handler.Result {
    return d.dispatchInternal(action, inputCtx)
}

// dispatchInternal is the core dispatch logic.
func (d *Dispatcher) dispatchInternal(action input.Action, inputCtx *input.Context) handler.Result {
    startTime := time.Now()

    // Build execution context
    ctx := d.buildContext(inputCtx)

    // Apply repeat count from action
    if action.Count > 0 {
        ctx.Count = action.Count
    }

    // Run pre-dispatch hooks
    if !d.hookManager.RunPreDispatch(&action, ctx) {
        return handler.Result{Status: handler.StatusCancelled, Message: "cancelled by hook"}
    }

    // Find handler
    h := d.router.Route(action.Name)
    if h == nil {
        return handler.Errorf("no handler for action: %s", action.Name)
    }

    // Execute handler with panic recovery
    var result handler.Result
    if d.config.RecoverFromPanic {
        result = d.executeWithRecovery(h, action, ctx)
    } else {
        result = h.Handle(action, ctx)
    }

    // Process result
    d.processResult(action, result, ctx)

    // Run post-dispatch hooks
    d.hookManager.RunPostDispatch(&action, ctx, &result)

    // Record metrics
    if d.metrics != nil {
        d.metrics.RecordDispatch(action.Name, time.Since(startTime), result.Status)
    }

    return result
}

// executeWithRecovery executes a handler with panic recovery.
func (d *Dispatcher) executeWithRecovery(h handler.Handler, action input.Action, ctx *ExecutionContext) (result handler.Result) {
    defer func() {
        if r := recover(); r != nil {
            result = handler.Errorf("handler panic: %v", r)
        }
    }()
    return h.Handle(action, ctx)
}

// buildContext builds an execution context.
func (d *Dispatcher) buildContext(inputCtx *input.Context) *ExecutionContext {
    d.mu.RLock()
    defer d.mu.RUnlock()

    return context.NewExecutionContext(
        d.engine,
        d.cursors,
        d.modeManager,
        d.history,
        d.renderer,
        inputCtx,
    )
}

// processResult processes a handler result.
func (d *Dispatcher) processResult(action input.Action, result handler.Result, ctx *ExecutionContext) {
    // Handle mode change
    if result.ModeChange != "" && ctx.ModeManager != nil {
        _ = ctx.ModeManager.Switch(result.ModeChange)
    }

    // Handle view updates
    if result.ViewUpdate.Redraw && ctx.Renderer != nil {
        ctx.Renderer.Redraw()
    } else if len(result.ViewUpdate.RedrawLines) > 0 && ctx.Renderer != nil {
        ctx.Renderer.RedrawLines(result.ViewUpdate.RedrawLines)
    }

    if result.ViewUpdate.ScrollTo != nil && ctx.Renderer != nil {
        st := result.ViewUpdate.ScrollTo
        if st.Center {
            ctx.Renderer.CenterOnLine(st.Line)
        } else {
            ctx.Renderer.ScrollTo(st.Line, st.Column)
        }
    }
}

// RegisterHandler registers a handler for an action pattern.
func (d *Dispatcher) RegisterHandler(pattern string, h handler.Handler) {
    d.router.Register(pattern, h)
}

// RegisterHook registers a dispatch hook.
func (d *Dispatcher) RegisterHook(h hook.Hook) {
    d.hookManager.Register(h)
}

// registerBuiltinHandlers registers all built-in handlers.
func (d *Dispatcher) registerBuiltinHandlers() {
    // Cursor handlers
    d.router.RegisterNamespace("cursor", handlers.NewCursorHandlers())

    // Editor handlers
    d.router.RegisterNamespace("editor", handlers.NewEditorHandlers())

    // Mode handlers
    d.router.RegisterNamespace("mode", handlers.NewModeHandlers())

    // Operator handlers
    d.router.RegisterNamespace("operator", handlers.NewOperatorHandlers())

    // Search handlers
    d.router.RegisterNamespace("search", handlers.NewSearchHandlers())

    // View handlers
    d.router.RegisterNamespace("view", handlers.NewViewHandlers())
    d.router.RegisterNamespace("scroll", handlers.NewScrollHandlers())

    // File handlers
    d.router.RegisterNamespace("file", handlers.NewFileHandlers())

    // Window handlers
    d.router.RegisterNamespace("window", handlers.NewWindowHandlers())

    // Macro handlers
    d.router.RegisterNamespace("macro", handlers.NewMacroHandlers())

    // Completion handlers
    d.router.RegisterNamespace("completion", handlers.NewCompletionHandlers())

    // Palette handlers
    d.router.RegisterNamespace("palette", handlers.NewPaletteHandlers())

    // Selection handlers
    d.router.RegisterNamespace("selection", handlers.NewSelectionHandlers())

    // Context menu handlers
    d.router.RegisterNamespace("contextMenu", handlers.NewContextMenuHandlers())
}

// Start starts the async dispatch loop (if enabled).
func (d *Dispatcher) Start() {
    if !d.config.AsyncDispatch {
        return
    }

    go d.dispatchLoop()
}

// Stop stops the async dispatch loop.
func (d *Dispatcher) Stop() {
    close(d.done)
}

// dispatchLoop processes actions asynchronously.
func (d *Dispatcher) dispatchLoop() {
    for {
        select {
        case action := <-d.actionChan:
            result := d.Dispatch(action)
            select {
            case d.resultChan <- result:
            default:
                // Result channel full, drop result
            }
        case <-d.done:
            return
        }
    }
}

// Actions returns the action channel for async dispatch.
func (d *Dispatcher) Actions() chan<- input.Action {
    return d.actionChan
}

// Results returns the result channel for async dispatch.
func (d *Dispatcher) Results() <-chan handler.Result {
    return d.resultChan
}
```

---

## 6. Handler Registry System

### 6.1 Handler Registry

```go
// internal/dispatcher/handler/registry.go

// Registry manages handler registration.
type Registry struct {
    mu       sync.RWMutex
    handlers map[string][]Handler // action name -> handlers (sorted by priority)
}

// NewRegistry creates a new handler registry.
func NewRegistry() *Registry {
    return &Registry{
        handlers: make(map[string][]Handler),
    }
}

// Register adds a handler for an action name.
func (r *Registry) Register(actionName string, h Handler) {
    r.mu.Lock()
    defer r.mu.Unlock()

    handlers := r.handlers[actionName]
    handlers = append(handlers, h)

    // Sort by priority (descending)
    sort.Slice(handlers, func(i, j int) bool {
        return handlers[i].Priority() > handlers[j].Priority()
    })

    r.handlers[actionName] = handlers
}

// Unregister removes a handler.
func (r *Registry) Unregister(actionName string, h Handler) {
    r.mu.Lock()
    defer r.mu.Unlock()

    handlers := r.handlers[actionName]
    for i, existing := range handlers {
        if existing == h {
            r.handlers[actionName] = append(handlers[:i], handlers[i+1:]...)
            break
        }
    }
}

// Get returns the highest priority handler for an action.
func (r *Registry) Get(actionName string) Handler {
    r.mu.RLock()
    defer r.mu.RUnlock()

    handlers := r.handlers[actionName]
    if len(handlers) == 0 {
        return nil
    }
    return handlers[0]
}

// GetAll returns all handlers for an action.
func (r *Registry) GetAll(actionName string) []Handler {
    r.mu.RLock()
    defer r.mu.RUnlock()

    handlers := r.handlers[actionName]
    result := make([]Handler, len(handlers))
    copy(result, handlers)
    return result
}

// List returns all registered action names.
func (r *Registry) List() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()

    names := make([]string, 0, len(r.handlers))
    for name := range r.handlers {
        names = append(names, name)
    }
    sort.Strings(names)
    return names
}
```

### 6.2 Action Router

```go
// internal/dispatcher/handler/router.go

// Router routes actions to handlers using namespace prefixes.
type Router struct {
    mu sync.RWMutex

    // Exact match handlers
    exact map[string]Handler

    // Namespace handlers (e.g., "cursor" handles "cursor.*")
    namespaces map[string]NamespaceHandler

    // Fallback handler
    fallback Handler
}

// NamespaceHandler handles all actions within a namespace.
type NamespaceHandler interface {
    // HandleAction handles an action within this namespace.
    HandleAction(action input.Action, ctx *ExecutionContext) Result

    // CanHandle returns true if this handler can process the action.
    CanHandle(actionName string) bool

    // Namespace returns the namespace prefix.
    Namespace() string
}

// NewRouter creates a new action router.
func NewRouter() *Router {
    return &Router{
        exact:      make(map[string]Handler),
        namespaces: make(map[string]NamespaceHandler),
    }
}

// Register registers an exact action handler.
func (r *Router) Register(actionName string, h Handler) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.exact[actionName] = h
}

// RegisterNamespace registers a namespace handler.
func (r *Router) RegisterNamespace(namespace string, h NamespaceHandler) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.namespaces[namespace] = h
}

// SetFallback sets the fallback handler for unmatched actions.
func (r *Router) SetFallback(h Handler) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.fallback = h
}

// Route finds the appropriate handler for an action.
func (r *Router) Route(actionName string) Handler {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Check exact match first
    if h, ok := r.exact[actionName]; ok {
        return h
    }

    // Extract namespace prefix
    namespace := extractNamespace(actionName)
    if namespace != "" {
        if h, ok := r.namespaces[namespace]; ok {
            if h.CanHandle(actionName) {
                return &namespaceAdapter{h: h}
            }
        }
    }

    // Fallback
    return r.fallback
}

// extractNamespace extracts the namespace from "namespace.action" format.
func extractNamespace(actionName string) string {
    idx := strings.Index(actionName, ".")
    if idx < 0 {
        return ""
    }
    return actionName[:idx]
}

// namespaceAdapter adapts NamespaceHandler to Handler interface.
type namespaceAdapter struct {
    h NamespaceHandler
}

func (a *namespaceAdapter) Handle(action input.Action, ctx *ExecutionContext) Result {
    return a.h.HandleAction(action, ctx)
}

func (a *namespaceAdapter) CanHandle(actionName string) bool {
    return a.h.CanHandle(actionName)
}

func (a *namespaceAdapter) Priority() int {
    return 0
}
```

---

## 7. Action Routing

### 7.1 Action Namespaces

Standard action namespaces and their responsibilities:

| Namespace | Responsibility | Example Actions |
|-----------|---------------|-----------------|
| `cursor` | Cursor movement | `cursor.left`, `cursor.up`, `cursor.wordForward` |
| `editor` | Text editing | `editor.insertChar`, `editor.deleteLine`, `editor.undo` |
| `mode` | Mode transitions | `mode.normal`, `mode.insert`, `mode.visual` |
| `operator` | Vim operators | `operator.delete`, `operator.change`, `operator.yank` |
| `search` | Search operations | `search.forward`, `search.next`, `search.replace` |
| `view` | View manipulation | `view.centerCursor`, `view.zoomIn` |
| `scroll` | Scrolling | `scroll.up`, `scroll.halfPageDown` |
| `file` | File operations | `file.save`, `file.open`, `file.close` |
| `window` | Window/splits | `window.splitVertical`, `window.focusLeft` |
| `buffer` | Buffer management | `buffer.next`, `buffer.previous`, `buffer.close` |
| `selection` | Selection operations | `selection.word`, `selection.line`, `selection.all` |
| `macro` | Macro recording | `macro.record`, `macro.play`, `macro.stop` |
| `completion` | Auto-completion | `completion.trigger`, `completion.next`, `completion.accept` |
| `palette` | Command palette | `palette.show`, `palette.execute` |
| `lsp` | LSP operations | `lsp.goToDefinition`, `lsp.findReferences` |
| `ai` | AI workflows | `ai.explain`, `ai.refactor`, `ai.generateTests` |
| `contextMenu` | Context menu | `contextMenu.show` |

### 7.2 Action Name Conventions

```
<namespace>.<action>[.<modifier>]

Examples:
- cursor.moveDown
- cursor.wordForward
- editor.insertChar
- editor.deleteToLineEnd
- mode.insert
- mode.visual.line
- mode.visual.block
- operator.delete
- search.forward
- file.save
- file.save.as
```

---

## 8. Handler Implementations

### 8.1 Cursor Handlers

```go
// internal/dispatcher/handlers/cursor/cursor.go

// Handlers implements NamespaceHandler for cursor operations.
type Handlers struct {
    actions map[string]handlerFn
}

type handlerFn func(action input.Action, ctx *context.ExecutionContext) handler.Result

// NewCursorHandlers creates cursor handlers.
func NewCursorHandlers() *Handlers {
    h := &Handlers{
        actions: make(map[string]handlerFn),
    }

    // Basic movement
    h.actions["cursor.left"] = h.moveLeft
    h.actions["cursor.right"] = h.moveRight
    h.actions["cursor.up"] = h.moveUp
    h.actions["cursor.down"] = h.moveDown

    // Word movement
    h.actions["cursor.wordForward"] = h.wordForward
    h.actions["cursor.wordBackward"] = h.wordBackward
    h.actions["cursor.wordEnd"] = h.wordEnd

    // Line movement
    h.actions["cursor.lineStart"] = h.lineStart
    h.actions["cursor.lineEnd"] = h.lineEnd
    h.actions["cursor.firstNonBlank"] = h.firstNonBlank

    // Document movement
    h.actions["cursor.documentStart"] = h.documentStart
    h.actions["cursor.documentEnd"] = h.documentEnd
    h.actions["cursor.goToLine"] = h.goToLine

    // Position setting
    h.actions["cursor.setPosition"] = h.setPosition

    return h
}

func (h *Handlers) Namespace() string {
    return "cursor"
}

func (h *Handlers) CanHandle(actionName string) bool {
    _, ok := h.actions[actionName]
    return ok
}

func (h *Handlers) HandleAction(action input.Action, ctx *context.ExecutionContext) handler.Result {
    fn, ok := h.actions[action.Name]
    if !ok {
        return handler.Errorf("unknown cursor action: %s", action.Name)
    }
    return fn(action, ctx)
}

// Movement handlers

func (h *Handlers) moveLeft(action input.Action, ctx *context.ExecutionContext) handler.Result {
    count := ctx.Count
    sel := ctx.Cursors.Primary()
    offset := sel.Head

    // Calculate new position
    for i := 0; i < count && offset > 0; i++ {
        offset--
        // Skip over UTF-8 continuation bytes
        text := ctx.Engine.TextRange(0, offset+1)
        for offset > 0 && !utf8.RuneStart(text[offset]) {
            offset--
        }
    }

    // Update cursor
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))

    return handler.Success()
}

func (h *Handlers) moveRight(action input.Action, ctx *context.ExecutionContext) handler.Result {
    count := ctx.Count
    sel := ctx.Cursors.Primary()
    offset := sel.Head
    maxOffset := ctx.Engine.Len()

    // Calculate new position
    text := ctx.Engine.Text()
    for i := 0; i < count && offset < maxOffset; i++ {
        _, size := utf8.DecodeRuneInString(text[offset:])
        offset += buffer.ByteOffset(size)
    }

    // Update cursor
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))

    return handler.Success()
}

func (h *Handlers) moveUp(action input.Action, ctx *context.ExecutionContext) handler.Result {
    count := ctx.Count
    sel := ctx.Cursors.Primary()
    point := ctx.Engine.OffsetToPoint(sel.Head)

    // Calculate new line
    if point.Line >= uint32(count) {
        point.Line -= uint32(count)
    } else {
        point.Line = 0
    }

    // Clamp column to line length
    lineLen := ctx.Engine.LineLen(point.Line)
    if point.Column > lineLen {
        point.Column = lineLen
    }

    // Convert back to offset
    offset := ctx.Engine.PointToOffset(point)
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))

    return handler.Success()
}

func (h *Handlers) moveDown(action input.Action, ctx *context.ExecutionContext) handler.Result {
    count := ctx.Count
    sel := ctx.Cursors.Primary()
    point := ctx.Engine.OffsetToPoint(sel.Head)
    maxLine := ctx.Engine.LineCount() - 1

    // Calculate new line
    point.Line += uint32(count)
    if point.Line > maxLine {
        point.Line = maxLine
    }

    // Clamp column to line length
    lineLen := ctx.Engine.LineLen(point.Line)
    if point.Column > lineLen {
        point.Column = lineLen
    }

    // Convert back to offset
    offset := ctx.Engine.PointToOffset(point)
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))

    return handler.Success()
}

func (h *Handlers) wordForward(action input.Action, ctx *context.ExecutionContext) handler.Result {
    count := ctx.Count
    sel := ctx.Cursors.Primary()
    offset := sel.Head
    text := ctx.Engine.Text()
    maxOffset := buffer.ByteOffset(len(text))

    for i := 0; i < count && offset < maxOffset; i++ {
        // Skip current word
        for offset < maxOffset && isWordChar(text, offset) {
            offset = nextRune(text, offset)
        }
        // Skip whitespace
        for offset < maxOffset && !isWordChar(text, offset) {
            offset = nextRune(text, offset)
        }
    }

    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))
    return handler.Success()
}

func (h *Handlers) documentStart(action input.Action, ctx *context.ExecutionContext) handler.Result {
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(0))
    return handler.Success()
}

func (h *Handlers) documentEnd(action input.Action, ctx *context.ExecutionContext) handler.Result {
    offset := ctx.Engine.Len()
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))
    return handler.Success()
}

func (h *Handlers) goToLine(action input.Action, ctx *context.ExecutionContext) handler.Result {
    line := uint32(action.Args.GetInt("line"))
    if line == 0 {
        line = uint32(ctx.Count)
    }

    // Clamp to valid range
    maxLine := ctx.Engine.LineCount()
    if line > maxLine {
        line = maxLine
    }
    if line > 0 {
        line-- // Convert to 0-based
    }

    offset := ctx.Engine.LineStartOffset(line)
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))

    return handler.Success().WithViewUpdate(handler.ViewUpdate{
        CenterLine: &line,
    })
}

func (h *Handlers) setPosition(action input.Action, ctx *context.ExecutionContext) handler.Result {
    x := action.Args.GetInt("x")
    y := action.Args.GetInt("y")

    // Convert screen coordinates to buffer position
    // This would integrate with the renderer to map screen -> buffer
    offset := ctx.Engine.PointToOffset(buffer.Point{
        Line:   uint32(y),
        Column: uint32(x),
    })

    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))
    return handler.Success()
}

// Helper functions

func isWordChar(text string, offset buffer.ByteOffset) bool {
    if int(offset) >= len(text) {
        return false
    }
    r, _ := utf8.DecodeRuneInString(text[offset:])
    return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func nextRune(text string, offset buffer.ByteOffset) buffer.ByteOffset {
    _, size := utf8.DecodeRuneInString(text[offset:])
    return offset + buffer.ByteOffset(size)
}
```

### 8.2 Editor Handlers

```go
// internal/dispatcher/handlers/editor/editor.go

// Handlers implements NamespaceHandler for editor operations.
type Handlers struct {
    actions map[string]handlerFn
}

type handlerFn func(action input.Action, ctx *context.ExecutionContext) handler.Result

// NewEditorHandlers creates editor handlers.
func NewEditorHandlers() *Handlers {
    h := &Handlers{
        actions: make(map[string]handlerFn),
    }

    // Insert operations
    h.actions["editor.insertChar"] = h.insertChar
    h.actions["editor.insertText"] = h.insertText
    h.actions["editor.insertNewline"] = h.insertNewline

    // Delete operations
    h.actions["editor.deleteChar"] = h.deleteChar
    h.actions["editor.deleteCharBefore"] = h.deleteCharBefore
    h.actions["editor.deleteLine"] = h.deleteLine
    h.actions["editor.deleteToLineEnd"] = h.deleteToLineEnd
    h.actions["editor.deleteToLineStart"] = h.deleteToLineStart
    h.actions["editor.deleteWordBefore"] = h.deleteWordBefore
    h.actions["editor.deleteWordAfter"] = h.deleteWordAfter

    // Line operations
    h.actions["editor.yankLine"] = h.yankLine
    h.actions["editor.changeLine"] = h.changeLine
    h.actions["editor.indentLine"] = h.indentLine
    h.actions["editor.outdentLine"] = h.outdentLine
    h.actions["editor.joinLines"] = h.joinLines
    h.actions["editor.duplicateLine"] = h.duplicateLine

    // Clipboard operations
    h.actions["editor.pasteAfter"] = h.pasteAfter
    h.actions["editor.pasteBefore"] = h.pasteBefore
    h.actions["editor.pasteSelection"] = h.pasteSelection

    // Replace operations
    h.actions["editor.replaceChar"] = h.replaceChar

    // Undo/redo
    h.actions["editor.undo"] = h.undo
    h.actions["editor.redo"] = h.redo

    // Repeat
    h.actions["editor.repeatLast"] = h.repeatLast

    return h
}

func (h *Handlers) Namespace() string {
    return "editor"
}

func (h *Handlers) CanHandle(actionName string) bool {
    _, ok := h.actions[actionName]
    return ok
}

func (h *Handlers) HandleAction(action input.Action, ctx *context.ExecutionContext) handler.Result {
    fn, ok := h.actions[action.Name]
    if !ok {
        return handler.Errorf("unknown editor action: %s", action.Name)
    }
    return fn(action, ctx)
}

// Insert operations

func (h *Handlers) insertChar(action input.Action, ctx *context.ExecutionContext) handler.Result {
    text := action.Args.Text
    if text == "" {
        return handler.NoOp()
    }

    // Begin undo group for character insertions
    ctx.History.BeginGroup("insert")
    defer ctx.History.EndGroup()

    // Get all selections and apply in reverse order
    selections := ctx.Cursors.All()
    for i := len(selections) - 1; i >= 0; i-- {
        sel := selections[i]

        // Delete selection if not empty
        if !sel.IsEmpty() {
            _, err := ctx.Engine.Delete(sel.Start(), sel.End())
            if err != nil {
                return handler.Error(err)
            }
        }

        // Insert text
        result, err := ctx.Engine.Insert(sel.Start(), text)
        if err != nil {
            return handler.Error(err)
        }

        // Update cursor to end of inserted text
        ctx.Cursors.TransformAfterEdit(buffer.Edit{
            Range:   sel.Range(),
            NewText: text,
        })
    }

    return handler.Success()
}

func (h *Handlers) insertNewline(action input.Action, ctx *context.ExecutionContext) handler.Result {
    action.Args.Text = "\n"
    return h.insertChar(action, ctx)
}

func (h *Handlers) deleteLine(action input.Action, ctx *context.ExecutionContext) handler.Result {
    count := ctx.Count
    sel := ctx.Cursors.Primary()
    point := ctx.Engine.OffsetToPoint(sel.Head)

    // Calculate range of lines to delete
    startLine := point.Line
    endLine := startLine + uint32(count)
    if endLine > ctx.Engine.LineCount() {
        endLine = ctx.Engine.LineCount()
    }

    start := ctx.Engine.LineStartOffset(startLine)
    var end buffer.ByteOffset
    if endLine >= ctx.Engine.LineCount() {
        end = ctx.Engine.Len()
    } else {
        end = ctx.Engine.LineStartOffset(endLine)
    }

    // Store deleted text for yank register
    deletedText := ctx.Engine.TextRange(start, end)

    // Delete
    _, err := ctx.Engine.Delete(start, end)
    if err != nil {
        return handler.Error(err)
    }

    // Position cursor at start of next line (or end of buffer)
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(start))

    return handler.SuccessWithMessage(fmt.Sprintf("%d lines deleted", count)).
        WithData("yanked", deletedText)
}

func (h *Handlers) undo(action input.Action, ctx *context.ExecutionContext) handler.Result {
    count := ctx.Count
    for i := 0; i < count; i++ {
        if !ctx.History.CanUndo() {
            if i == 0 {
                return handler.SuccessWithMessage("Already at oldest change")
            }
            break
        }
        if err := ctx.History.Undo(); err != nil {
            return handler.Error(err)
        }
    }
    return handler.Success()
}

func (h *Handlers) redo(action input.Action, ctx *context.ExecutionContext) handler.Result {
    count := ctx.Count
    for i := 0; i < count; i++ {
        if !ctx.History.CanRedo() {
            if i == 0 {
                return handler.SuccessWithMessage("Already at newest change")
            }
            break
        }
        if err := ctx.History.Redo(); err != nil {
            return handler.Error(err)
        }
    }
    return handler.Success()
}

// ... additional editor handlers
```

### 8.3 Mode Handlers

```go
// internal/dispatcher/handlers/mode/mode.go

// Handlers implements NamespaceHandler for mode operations.
type Handlers struct {
    actions map[string]handlerFn
}

type handlerFn func(action input.Action, ctx *context.ExecutionContext) handler.Result

// NewModeHandlers creates mode handlers.
func NewModeHandlers() *Handlers {
    h := &Handlers{
        actions: make(map[string]handlerFn),
    }

    // Mode switching
    h.actions["mode.normal"] = h.switchToNormal
    h.actions["mode.insert"] = h.switchToInsert
    h.actions["mode.insertLineStart"] = h.insertLineStart
    h.actions["mode.append"] = h.append
    h.actions["mode.appendLineEnd"] = h.appendLineEnd
    h.actions["mode.openBelow"] = h.openBelow
    h.actions["mode.openAbove"] = h.openAbove
    h.actions["mode.visual"] = h.switchToVisual
    h.actions["mode.visualLine"] = h.switchToVisualLine
    h.actions["mode.visualBlock"] = h.switchToVisualBlock
    h.actions["mode.command"] = h.switchToCommand
    h.actions["mode.replace"] = h.switchToReplace

    return h
}

func (h *Handlers) Namespace() string {
    return "mode"
}

func (h *Handlers) CanHandle(actionName string) bool {
    _, ok := h.actions[actionName]
    return ok
}

func (h *Handlers) HandleAction(action input.Action, ctx *context.ExecutionContext) handler.Result {
    fn, ok := h.actions[action.Name]
    if !ok {
        return handler.Errorf("unknown mode action: %s", action.Name)
    }
    return fn(action, ctx)
}

func (h *Handlers) switchToNormal(action input.Action, ctx *context.ExecutionContext) handler.Result {
    if err := ctx.ModeManager.Switch("normal"); err != nil {
        return handler.Error(err)
    }

    // Clear pending state
    if ctx.Input != nil {
        ctx.Input.ClearPending()
    }

    // Collapse selection to cursor
    sel := ctx.Cursors.Primary()
    if !sel.IsEmpty() {
        ctx.Cursors.SetPrimary(sel.CollapseToStart())
    }

    return handler.Success().WithModeChange("normal")
}

func (h *Handlers) switchToInsert(action input.Action, ctx *context.ExecutionContext) handler.Result {
    if err := ctx.ModeManager.Switch("insert"); err != nil {
        return handler.Error(err)
    }
    return handler.Success().WithModeChange("insert")
}

func (h *Handlers) insertLineStart(action input.Action, ctx *context.ExecutionContext) handler.Result {
    // Move to first non-blank character, then enter insert mode
    sel := ctx.Cursors.Primary()
    point := ctx.Engine.OffsetToPoint(sel.Head)
    lineStart := ctx.Engine.LineStartOffset(point.Line)
    lineText := ctx.Engine.LineText(point.Line)

    // Find first non-blank
    offset := lineStart
    for i, r := range lineText {
        if !unicode.IsSpace(r) {
            offset = lineStart + buffer.ByteOffset(i)
            break
        }
    }

    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))

    if err := ctx.ModeManager.Switch("insert"); err != nil {
        return handler.Error(err)
    }
    return handler.Success().WithModeChange("insert")
}

func (h *Handlers) append(action input.Action, ctx *context.ExecutionContext) handler.Result {
    // Move cursor right by one, then enter insert mode
    sel := ctx.Cursors.Primary()
    offset := sel.Head

    // Move right unless at end of buffer
    text := ctx.Engine.Text()
    if int(offset) < len(text) {
        _, size := utf8.DecodeRuneInString(text[offset:])
        offset += buffer.ByteOffset(size)
    }

    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))

    if err := ctx.ModeManager.Switch("insert"); err != nil {
        return handler.Error(err)
    }
    return handler.Success().WithModeChange("insert")
}

func (h *Handlers) appendLineEnd(action input.Action, ctx *context.ExecutionContext) handler.Result {
    // Move to end of line, then enter insert mode
    sel := ctx.Cursors.Primary()
    point := ctx.Engine.OffsetToPoint(sel.Head)
    offset := ctx.Engine.LineEndOffset(point.Line)

    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(offset))

    if err := ctx.ModeManager.Switch("insert"); err != nil {
        return handler.Error(err)
    }
    return handler.Success().WithModeChange("insert")
}

func (h *Handlers) openBelow(action input.Action, ctx *context.ExecutionContext) handler.Result {
    // Insert newline below current line, move cursor there, enter insert mode
    sel := ctx.Cursors.Primary()
    point := ctx.Engine.OffsetToPoint(sel.Head)
    lineEnd := ctx.Engine.LineEndOffset(point.Line)

    // Insert newline
    result, err := ctx.Engine.Insert(lineEnd, "\n")
    if err != nil {
        return handler.Error(err)
    }

    // Move cursor to new line
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(lineEnd + 1))

    if err := ctx.ModeManager.Switch("insert"); err != nil {
        return handler.Error(err)
    }
    return handler.Success().WithModeChange("insert")
}

func (h *Handlers) openAbove(action input.Action, ctx *context.ExecutionContext) handler.Result {
    // Insert newline above current line, move cursor there, enter insert mode
    sel := ctx.Cursors.Primary()
    point := ctx.Engine.OffsetToPoint(sel.Head)
    lineStart := ctx.Engine.LineStartOffset(point.Line)

    // Insert newline before current line
    _, err := ctx.Engine.Insert(lineStart, "\n")
    if err != nil {
        return handler.Error(err)
    }

    // Move cursor to new line (which is now at lineStart)
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(lineStart))

    if err := ctx.ModeManager.Switch("insert"); err != nil {
        return handler.Error(err)
    }
    return handler.Success().WithModeChange("insert")
}

func (h *Handlers) switchToVisual(action input.Action, ctx *context.ExecutionContext) handler.Result {
    if err := ctx.ModeManager.Switch("visual"); err != nil {
        return handler.Error(err)
    }
    // Start selection at current cursor position
    sel := ctx.Cursors.Primary()
    ctx.Cursors.SetPrimary(cursor.NewSelection(sel.Head, sel.Head))

    return handler.Success().WithModeChange("visual")
}

func (h *Handlers) switchToVisualLine(action input.Action, ctx *context.ExecutionContext) handler.Result {
    if err := ctx.ModeManager.Switch("visual-line"); err != nil {
        return handler.Error(err)
    }
    // Select entire current line
    sel := ctx.Cursors.Primary()
    point := ctx.Engine.OffsetToPoint(sel.Head)
    lineStart := ctx.Engine.LineStartOffset(point.Line)
    lineEnd := ctx.Engine.LineEndOffset(point.Line) + 1 // Include newline

    ctx.Cursors.SetPrimary(cursor.NewSelection(lineStart, lineEnd))

    return handler.Success().WithModeChange("visual-line")
}

func (h *Handlers) switchToCommand(action input.Action, ctx *context.ExecutionContext) handler.Result {
    if err := ctx.ModeManager.Switch("command"); err != nil {
        return handler.Error(err)
    }
    return handler.Success().WithModeChange("command")
}
```

### 8.4 Operator Handlers

```go
// internal/dispatcher/handlers/operator/operator.go

// Handlers implements NamespaceHandler for operator operations.
// Operators are Vim-style commands that require a motion or text object.
type Handlers struct {
    actions map[string]handlerFn

    // Pending operator state
    pendingOperator string
}

type handlerFn func(action input.Action, ctx *context.ExecutionContext) handler.Result

// NewOperatorHandlers creates operator handlers.
func NewOperatorHandlers() *Handlers {
    h := &Handlers{
        actions: make(map[string]handlerFn),
    }

    // Operators
    h.actions["operator.delete"] = h.delete
    h.actions["operator.change"] = h.change
    h.actions["operator.yank"] = h.yank
    h.actions["operator.indent"] = h.indent
    h.actions["operator.outdent"] = h.outdent
    h.actions["operator.lowercase"] = h.lowercase
    h.actions["operator.uppercase"] = h.uppercase
    h.actions["operator.toggleCase"] = h.toggleCase
    h.actions["operator.format"] = h.format

    return h
}

func (h *Handlers) Namespace() string {
    return "operator"
}

func (h *Handlers) CanHandle(actionName string) bool {
    _, ok := h.actions[actionName]
    return ok
}

func (h *Handlers) HandleAction(action input.Action, ctx *context.ExecutionContext) handler.Result {
    fn, ok := h.actions[action.Name]
    if !ok {
        return handler.Errorf("unknown operator action: %s", action.Name)
    }
    return fn(action, ctx)
}

func (h *Handlers) delete(action input.Action, ctx *context.ExecutionContext) handler.Result {
    // Get the range to delete from motion or text object
    deleteRange, err := h.resolveRange(action, ctx)
    if err != nil {
        return handler.Error(err)
    }

    // Store deleted text for yank register
    deletedText := ctx.Engine.TextRange(deleteRange.Start, deleteRange.End)

    // Delete
    _, err = ctx.Engine.Delete(deleteRange.Start, deleteRange.End)
    if err != nil {
        return handler.Error(err)
    }

    // Position cursor at start of deleted region
    ctx.Cursors.SetPrimary(cursor.NewCursorSelection(deleteRange.Start))

    return handler.Success().WithData("yanked", deletedText)
}

func (h *Handlers) change(action input.Action, ctx *context.ExecutionContext) handler.Result {
    // Delete like operator.delete, then enter insert mode
    result := h.delete(action, ctx)
    if result.Status != handler.StatusOK {
        return result
    }

    // Enter insert mode
    if err := ctx.ModeManager.Switch("insert"); err != nil {
        return handler.Error(err)
    }

    return result.WithModeChange("insert")
}

func (h *Handlers) yank(action input.Action, ctx *context.ExecutionContext) handler.Result {
    // Get the range to yank
    yankRange, err := h.resolveRange(action, ctx)
    if err != nil {
        return handler.Error(err)
    }

    // Copy text to register
    text := ctx.Engine.TextRange(yankRange.Start, yankRange.End)

    // Store in register (implementation depends on clipboard integration)
    register := action.Args.Register
    if register == 0 {
        register = '"' // Default register
    }

    // TODO: Store in register system

    return handler.SuccessWithMessage(fmt.Sprintf("Yanked %d bytes", len(text))).
        WithData("yanked", text).
        WithData("register", string(register))
}

func (h *Handlers) indent(action input.Action, ctx *context.ExecutionContext) handler.Result {
    // Get the range to indent
    indentRange, err := h.resolveRange(action, ctx)
    if err != nil {
        return handler.Error(err)
    }

    // Get lines in range
    startPoint := ctx.Engine.OffsetToPoint(indentRange.Start)
    endPoint := ctx.Engine.OffsetToPoint(indentRange.End)

    // Indent each line
    for line := startPoint.Line; line <= endPoint.Line; line++ {
        lineStart := ctx.Engine.LineStartOffset(line)
        _, err := ctx.Engine.Insert(lineStart, "\t")
        if err != nil {
            return handler.Error(err)
        }
    }

    lineCount := endPoint.Line - startPoint.Line + 1
    return handler.SuccessWithMessage(fmt.Sprintf("%d lines indented", lineCount))
}

// resolveRange determines the range for an operator based on motion or selection.
func (h *Handlers) resolveRange(action input.Action, ctx *context.ExecutionContext) (buffer.Range, error) {
    // If there's a visual selection, use that
    sel := ctx.Cursors.Primary()
    if !sel.IsEmpty() {
        return sel.Range(), nil
    }

    // If a motion was provided, calculate range from cursor to motion target
    if action.Args.Motion != nil {
        return h.calculateMotionRange(action.Args.Motion, ctx)
    }

    // If a text object was provided, calculate range for text object
    if action.Args.TextObject != nil {
        return h.calculateTextObjectRange(action.Args.TextObject, ctx)
    }

    return buffer.Range{}, fmt.Errorf("operator requires motion or selection")
}

func (h *Handlers) calculateMotionRange(motion *input.Motion, ctx *context.ExecutionContext) (buffer.Range, error) {
    sel := ctx.Cursors.Primary()
    start := sel.Head

    // Execute motion to find end position
    // This is simplified - real implementation would use motion handlers
    switch motion.Name {
    case "word":
        end := h.findWordEnd(ctx.Engine.Text(), start, motion.Count)
        if motion.Direction == input.DirBackward {
            return buffer.Range{Start: end, End: start}, nil
        }
        return buffer.Range{Start: start, End: end}, nil
    case "line":
        point := ctx.Engine.OffsetToPoint(start)
        lineStart := ctx.Engine.LineStartOffset(point.Line)
        lineEnd := ctx.Engine.LineEndOffset(point.Line) + 1
        return buffer.Range{Start: lineStart, End: lineEnd}, nil
    default:
        return buffer.Range{}, fmt.Errorf("unknown motion: %s", motion.Name)
    }
}

func (h *Handlers) calculateTextObjectRange(textObj *input.TextObject, ctx *context.ExecutionContext) (buffer.Range, error) {
    sel := ctx.Cursors.Primary()
    offset := sel.Head
    text := ctx.Engine.Text()

    switch textObj.Name {
    case "word":
        return h.findWordBounds(text, offset, textObj.Inner)
    case "quote":
        return h.findQuoteBounds(text, offset, textObj.Delimiter, textObj.Inner)
    case "paren", "bracket", "brace":
        return h.findBracketBounds(text, offset, textObj.Delimiter, textObj.Inner)
    default:
        return buffer.Range{}, fmt.Errorf("unknown text object: %s", textObj.Name)
    }
}

// Helper functions for motion/text object calculation
func (h *Handlers) findWordEnd(text string, start buffer.ByteOffset, count int) buffer.ByteOffset {
    // Implementation of word end finding
    offset := start
    for i := 0; i < count; i++ {
        // Skip word characters
        for int(offset) < len(text) && isWordChar(text, offset) {
            offset = nextRune(text, offset)
        }
        // Skip non-word characters
        for int(offset) < len(text) && !isWordChar(text, offset) && text[offset] != '\n' {
            offset = nextRune(text, offset)
        }
    }
    return offset
}

func (h *Handlers) findWordBounds(text string, offset buffer.ByteOffset, inner bool) (buffer.Range, error) {
    // Find word boundaries around offset
    if int(offset) >= len(text) {
        return buffer.Range{}, fmt.Errorf("offset out of bounds")
    }

    // Find start of word
    start := offset
    for start > 0 && isWordChar(text, start-1) {
        start--
    }

    // Find end of word
    end := offset
    for int(end) < len(text) && isWordChar(text, end) {
        end = nextRune(text, end)
    }

    if !inner {
        // Include trailing whitespace for "around word"
        for int(end) < len(text) && unicode.IsSpace(rune(text[end])) && text[end] != '\n' {
            end++
        }
    }

    return buffer.Range{Start: start, End: end}, nil
}

func (h *Handlers) findQuoteBounds(text string, offset buffer.ByteOffset, quote rune, inner bool) (buffer.Range, error) {
    // Find matching quotes around offset
    // Implementation searches backward and forward for quote characters
    // ...
    return buffer.Range{}, nil // Simplified
}

func (h *Handlers) findBracketBounds(text string, offset buffer.ByteOffset, bracket rune, inner bool) (buffer.Range, error) {
    // Find matching brackets around offset
    // Implementation uses bracket matching with nesting awareness
    // ...
    return buffer.Range{}, nil // Simplified
}
```

---

## 9. State Management

### 9.1 Dispatcher State

The Dispatcher itself is stateless regarding editor content. All state lives in:
- **Engine**: Text buffer state
- **Cursors**: Cursor/selection positions
- **ModeManager**: Current mode
- **History**: Undo/redo stack
- **Input Context**: Pending operators, counts, registers

### 9.2 Execution Context Lifecycle

```
1. Action received
2. Build ExecutionContext from current state
3. Execute handler
4. Handler modifies state through interfaces
5. Process result (mode changes, view updates)
6. Context discarded (stateless)
```

### 9.3 Cursor State After Edits

```go
// After any edit, cursors must be updated
func (d *Dispatcher) processResult(action input.Action, result handler.Result, ctx *ExecutionContext) {
    // Update cursors based on edits
    for _, edit := range result.Edits {
        ctx.Cursors.TransformAfterEdit(edit)
    }

    // Handle other result processing...
}
```

---

## 10. Error Handling and Recovery

### 10.1 Error Types

```go
// internal/dispatcher/errors.go

// DispatchError wraps errors with dispatch context.
type DispatchError struct {
    Action  string
    Handler string
    Err     error
}

func (e *DispatchError) Error() string {
    return fmt.Sprintf("dispatch error [action=%s, handler=%s]: %v",
        e.Action, e.Handler, e.Err)
}

func (e *DispatchError) Unwrap() error {
    return e.Err
}

// ErrNoHandler indicates no handler found for action.
var ErrNoHandler = errors.New("no handler for action")

// ErrHandlerPanic indicates a handler panicked.
type ErrHandlerPanic struct {
    Action string
    Panic  interface{}
    Stack  []byte
}

func (e *ErrHandlerPanic) Error() string {
    return fmt.Sprintf("handler panic [action=%s]: %v\n%s",
        e.Action, e.Panic, string(e.Stack))
}

// ErrInvalidContext indicates context is missing required fields.
var ErrInvalidContext = errors.New("invalid execution context")

// ErrActionCancelled indicates action was cancelled by a hook.
var ErrActionCancelled = errors.New("action cancelled")
```

### 10.2 Panic Recovery

```go
func (d *Dispatcher) executeWithRecovery(h handler.Handler, action input.Action, ctx *ExecutionContext) (result handler.Result) {
    defer func() {
        if r := recover(); r != nil {
            stack := make([]byte, 4096)
            n := runtime.Stack(stack, false)

            err := &ErrHandlerPanic{
                Action: action.Name,
                Panic:  r,
                Stack:  stack[:n],
            }

            result = handler.Error(err)

            // Log the panic
            if d.config.EnableMetrics {
                d.metrics.RecordPanic(action.Name)
            }
        }
    }()

    return h.Handle(action, ctx)
}
```

### 10.3 Transaction Rollback

For compound operations that may fail partway:

```go
func (h *Handlers) atomicMultiEdit(action input.Action, ctx *context.ExecutionContext) handler.Result {
    // Create snapshot before changes
    snapshot := ctx.Engine.Snapshot()
    snapshotID := ctx.Engine.RevisionID()

    // Begin history group
    ctx.History.BeginGroup("multi-edit")

    // Attempt edits
    for _, edit := range edits {
        if err := applyEdit(edit, ctx); err != nil {
            // Rollback to snapshot
            ctx.History.EndGroup()
            // Undo to snapshot (implementation specific)
            for ctx.Engine.RevisionID() != snapshotID {
                ctx.History.Undo()
            }
            return handler.Error(err)
        }
    }

    ctx.History.EndGroup()
    return handler.Success()
}
```

---

## 11. Hook System

### 11.1 Hook Interface

```go
// internal/dispatcher/hook/hook.go

// Hook allows interception of action dispatch.
type Hook interface {
    // Name returns the hook identifier.
    Name() string

    // Priority returns the hook priority (higher = runs first).
    Priority() int
}

// PreDispatchHook runs before action dispatch.
type PreDispatchHook interface {
    Hook

    // PreDispatch runs before handler execution.
    // Return false to cancel the action.
    PreDispatch(action *input.Action, ctx *context.ExecutionContext) bool
}

// PostDispatchHook runs after action dispatch.
type PostDispatchHook interface {
    Hook

    // PostDispatch runs after handler execution.
    PostDispatch(action *input.Action, ctx *context.ExecutionContext, result *handler.Result)
}
```

### 11.2 Hook Manager

```go
// internal/dispatcher/hook/manager.go

// Manager manages dispatch hooks.
type Manager struct {
    mu        sync.RWMutex
    preHooks  []PreDispatchHook
    postHooks []PostDispatchHook
}

// NewManager creates a new hook manager.
func NewManager() *Manager {
    return &Manager{}
}

// Register adds a hook.
func (m *Manager) Register(h Hook) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if pre, ok := h.(PreDispatchHook); ok {
        m.preHooks = append(m.preHooks, pre)
        sort.Slice(m.preHooks, func(i, j int) bool {
            return m.preHooks[i].Priority() > m.preHooks[j].Priority()
        })
    }

    if post, ok := h.(PostDispatchHook); ok {
        m.postHooks = append(m.postHooks, post)
        sort.Slice(m.postHooks, func(i, j int) bool {
            return m.postHooks[i].Priority() > m.postHooks[j].Priority()
        })
    }
}

// Unregister removes a hook by name.
func (m *Manager) Unregister(name string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Remove from preHooks
    for i, h := range m.preHooks {
        if h.Name() == name {
            m.preHooks = append(m.preHooks[:i], m.preHooks[i+1:]...)
            break
        }
    }

    // Remove from postHooks
    for i, h := range m.postHooks {
        if h.Name() == name {
            m.postHooks = append(m.postHooks[:i], m.postHooks[i+1:]...)
            break
        }
    }
}

// RunPreDispatch runs all pre-dispatch hooks.
// Returns false if any hook cancels the action.
func (m *Manager) RunPreDispatch(action *input.Action, ctx *context.ExecutionContext) bool {
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, h := range m.preHooks {
        if !h.PreDispatch(action, ctx) {
            return false
        }
    }
    return true
}

// RunPostDispatch runs all post-dispatch hooks.
func (m *Manager) RunPostDispatch(action *input.Action, ctx *context.ExecutionContext, result *handler.Result) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, h := range m.postHooks {
        h.PostDispatch(action, ctx, result)
    }
}
```

### 11.3 Built-in Hooks

```go
// internal/dispatcher/hook/builtin.go

// AuditHook logs all dispatched actions.
type AuditHook struct {
    logger Logger
}

func (h *AuditHook) Name() string     { return "audit" }
func (h *AuditHook) Priority() int    { return 1000 } // Run first

func (h *AuditHook) PreDispatch(action *input.Action, ctx *context.ExecutionContext) bool {
    h.logger.Debug("dispatch", "action", action.Name, "count", ctx.Count)
    return true
}

func (h *AuditHook) PostDispatch(action *input.Action, ctx *context.ExecutionContext, result *handler.Result) {
    if result.Status == handler.StatusError {
        h.logger.Error("dispatch failed", "action", action.Name, "error", result.Error)
    }
}

// RepeatHook captures the last action for "." repeat.
type RepeatHook struct {
    lastAction *input.Action
    lastCount  int
}

func (h *RepeatHook) Name() string  { return "repeat" }
func (h *RepeatHook) Priority() int { return 500 }

func (h *RepeatHook) PostDispatch(action *input.Action, ctx *context.ExecutionContext, result *handler.Result) {
    // Only capture successful editing actions
    if result.Status == handler.StatusOK && isRepeatable(action.Name) {
        h.lastAction = action
        h.lastCount = ctx.Count
    }
}

func (h *RepeatHook) GetLastAction() (*input.Action, int) {
    return h.lastAction, h.lastCount
}

func isRepeatable(actionName string) bool {
    // Editing actions are repeatable
    return strings.HasPrefix(actionName, "editor.") ||
        strings.HasPrefix(actionName, "operator.")
}

// AIContextHook builds context for AI integration.
type AIContextHook struct {
    tracker ChangeTracker
}

func (h *AIContextHook) Name() string  { return "ai-context" }
func (h *AIContextHook) Priority() int { return 100 }

func (h *AIContextHook) PostDispatch(action *input.Action, ctx *context.ExecutionContext, result *handler.Result) {
    // Track edits for AI context building
    if len(result.Edits) > 0 {
        for _, edit := range result.Edits {
            h.tracker.RecordEdit(edit)
        }
    }
}
```

---

## 12. Implementation Phases

### Phase 1: Core Infrastructure

**Goal**: Establish foundational types and dispatcher skeleton.

**Tasks**:
1. `handler/handler.go` - Handler interface
2. `handler/result.go` - Result type
3. `context/context.go` - ExecutionContext type
4. `dispatcher.go` - Basic Dispatcher structure
5. `config.go` - Configuration
6. `errors.go` - Error types
7. Unit tests for core types

**Success Criteria**:
- Handler interface defined
- Result type handles all cases
- ExecutionContext builds correctly
- Basic dispatch loop works

### Phase 2: Handler Registry and Routing

**Goal**: Implement handler registration and routing.

**Tasks**:
1. `handler/registry.go` - Handler registry
2. `handler/router.go` - Namespace-based router
3. Router tests with mock handlers
4. Integration with Dispatcher

**Success Criteria**:
- Handlers register correctly
- Namespace routing works (O(1) lookup)
- Priority-based handler selection
- Unknown action handling

### Phase 3: Cursor Handlers

**Goal**: Implement all cursor movement operations.

**Tasks**:
1. `handlers/cursor/cursor.go` - Basic movement (h,j,k,l)
2. `handlers/cursor/motion.go` - Word, line, paragraph motions
3. Comprehensive tests for all movements
4. Multi-cursor support

**Success Criteria**:
- All Vim cursor motions work
- Repeat counts respected (5j)
- UTF-8 character handling correct
- Multi-cursor movements synchronized

### Phase 4: Editor Handlers

**Goal**: Implement text editing operations.

**Tasks**:
1. `handlers/editor/insert.go` - Insert operations
2. `handlers/editor/delete.go` - Delete operations
3. `handlers/editor/yank.go` - Yank/paste operations
4. `handlers/editor/indent.go` - Indent/outdent
5. Integration with undo/redo

**Success Criteria**:
- All basic edits work
- Multi-cursor edits correct
- Undo/redo integration
- Cursor positioning after edits

### Phase 5: Mode and Operator Handlers

**Goal**: Implement mode transitions and Vim operators.

**Tasks**:
1. `handlers/mode/mode.go` - Mode switching
2. `handlers/operator/operator.go` - Operator framework
3. Motion/text object resolution
4. Operator + motion composition

**Success Criteria**:
- Mode transitions work correctly
- Operators compose with motions (dw, ci")
- Visual selection operations
- Mode-specific cursor styles

### Phase 6: Hook System

**Goal**: Implement extensible hook system.

**Tasks**:
1. `hook/hook.go` - Hook interfaces
2. `hook/manager.go` - Hook manager
3. `hook/builtin.go` - Built-in hooks (audit, repeat, AI)
4. Hook ordering and cancellation

**Success Criteria**:
- Hooks run in priority order
- Pre-hooks can cancel actions
- Post-hooks receive results
- Built-in hooks work correctly

### Phase 7: Additional Handler Namespaces

**Goal**: Implement remaining handler namespaces.

**Tasks**:
1. `handlers/search/` - Search and replace
2. `handlers/view/` - View/scroll operations
3. `handlers/file/` - File operations
4. `handlers/window/` - Window/split operations
5. `handlers/completion/` - Completion handling
6. `handlers/macro/` - Macro operations

**Success Criteria**:
- All standard actions have handlers
- Integration with respective subsystems
- Error handling for each namespace

### Phase 8: Integration and Polish

**Goal**: Full integration with editor systems.

**Tasks**:
1. `integration.go` - DispatcherSystem facade
2. Integration with InputHandler
3. Performance optimization
4. Metrics and monitoring
5. Documentation

**Success Criteria**:
- End-to-end action flow works
- Dispatch latency < 1ms
- Comprehensive documentation
- All tests passing

---

## 13. Testing Strategy

### 13.1 Unit Tests

```go
// Example: Handler registration and lookup
func TestHandlerRegistry(t *testing.T) {
    reg := handler.NewRegistry()

    // Register handlers
    h1 := &mockHandler{name: "h1", priority: 10}
    h2 := &mockHandler{name: "h2", priority: 20}

    reg.Register("cursor.moveDown", h1)
    reg.Register("cursor.moveDown", h2)

    // Higher priority should be returned
    got := reg.Get("cursor.moveDown")
    if got != h2 {
        t.Errorf("expected h2 (higher priority), got %v", got)
    }
}

// Example: Cursor movement
func TestCursorMoveDown(t *testing.T) {
    engine := newMockEngine("line1\nline2\nline3")
    cursors := cursor.NewCursorSet(cursor.NewCursorSelection(0))
    ctx := &context.ExecutionContext{
        Engine:  engine,
        Cursors: cursors,
        Count:   1,
    }

    handler := cursor.NewCursorHandlers()
    action := input.Action{Name: "cursor.down"}

    result := handler.HandleAction(action, ctx)

    if result.Status != handler.StatusOK {
        t.Fatalf("expected OK, got %v", result.Status)
    }

    // Cursor should be on line 2
    sel := cursors.Primary()
    point := engine.OffsetToPoint(sel.Head)
    if point.Line != 1 {
        t.Errorf("expected line 1, got %d", point.Line)
    }
}

// Example: Operator with motion
func TestDeleteWord(t *testing.T) {
    engine := newMockEngine("hello world")
    cursors := cursor.NewCursorSet(cursor.NewCursorSelection(0))
    ctx := &context.ExecutionContext{
        Engine:  engine,
        Cursors: cursors,
        Count:   1,
    }

    handler := operator.NewOperatorHandlers()
    action := input.Action{
        Name: "operator.delete",
        Args: input.ActionArgs{
            Motion: &input.Motion{
                Name:      "word",
                Direction: input.DirForward,
                Count:     1,
            },
        },
    }

    result := handler.HandleAction(action, ctx)

    if result.Status != handler.StatusOK {
        t.Fatalf("expected OK, got %v", result.Status)
    }

    // "hello " should be deleted, leaving "world"
    if engine.Text() != "world" {
        t.Errorf("expected 'world', got %q", engine.Text())
    }
}
```

### 13.2 Integration Tests

```go
func TestFullDispatchCycle(t *testing.T) {
    // Create dispatcher with all handlers
    dispatcher := New(DefaultConfig())

    // Set up mock subsystems
    engine := newMockEngine("hello world")
    cursors := cursor.NewCursorSet(cursor.NewCursorSelection(0))
    modeManager := mode.NewManager("normal")

    dispatcher.SetEngine(engine)
    dispatcher.SetCursors(cursors)
    dispatcher.SetModeManager(modeManager)

    // Dispatch a sequence of actions
    actions := []input.Action{
        {Name: "cursor.wordForward"},  // Move to "world"
        {Name: "mode.insert"},         // Enter insert mode
        {Name: "editor.insertChar", Args: input.ActionArgs{Text: "X"}}, // Insert X
        {Name: "mode.normal"},         // Back to normal
    }

    for _, action := range actions {
        result := dispatcher.Dispatch(action)
        if result.Status == handler.StatusError {
            t.Fatalf("action %s failed: %v", action.Name, result.Error)
        }
    }

    // Verify final state
    if engine.Text() != "hello Xworld" {
        t.Errorf("expected 'hello Xworld', got %q", engine.Text())
    }
}
```

### 13.3 Benchmark Tests

```go
func BenchmarkDispatchSimpleAction(b *testing.B) {
    dispatcher := New(DefaultConfig())
    dispatcher.SetEngine(newMockEngine("hello world"))
    dispatcher.SetCursors(cursor.NewCursorSet(cursor.NewCursorSelection(0)))

    action := input.Action{Name: "cursor.right"}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        dispatcher.Dispatch(action)
    }
}

func BenchmarkRouterLookup(b *testing.B) {
    router := handler.NewRouter()
    router.RegisterNamespace("cursor", cursor.NewCursorHandlers())
    router.RegisterNamespace("editor", editor.NewEditorHandlers())
    router.RegisterNamespace("mode", mode.NewModeHandlers())

    actions := []string{
        "cursor.moveDown",
        "editor.insertChar",
        "mode.insert",
        "cursor.wordForward",
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        router.Route(actions[i%len(actions)])
    }
}
```

---

## 14. Performance Considerations

### 14.1 Dispatch Latency

**Target**: < 1ms for simple actions

**Strategies**:
- O(1) handler lookup via namespace map
- No allocations in hot path
- Minimal locking (RWMutex for handler registry)
- Inline simple handlers

### 14.2 Memory Efficiency

**Strategies**:
- Reuse ExecutionContext where possible (sync.Pool)
- Avoid copying large strings in Result
- Lazy evaluation of result data

### 14.3 Concurrency

**Design**:
- Dispatcher itself is thread-safe
- Handlers assume exclusive access during execution
- Async dispatch via channel for background operations

### 14.4 Hot Path Optimization

```go
// Optimized dispatch for known-simple actions
func (d *Dispatcher) dispatchFast(action input.Action) handler.Result {
    // Skip hooks and context building for simple actions
    switch action.Name {
    case "cursor.left", "cursor.right", "cursor.up", "cursor.down":
        return d.cursorHandlers.HandleActionFast(action, d.engine, d.cursors)
    default:
        return d.dispatchInternal(action, nil)
    }
}
```

---

## Dependencies

### Internal Dependencies

| Package | Purpose |
|---------|---------|
| `internal/input` | Action, Context types |
| `internal/engine` | Buffer operations |
| `internal/engine/buffer` | Position types |
| `internal/engine/cursor` | Cursor types |
| `internal/input/mode` | Mode types and manager |
| `internal/engine/history` | Undo/redo |
| `internal/renderer` | View operations |
| `internal/event` | Event publishing |

### External Dependencies

None required for core dispatcher.

---

## References

- [Vim Documentation - Operators](https://vimdoc.sourceforge.net/htmldoc/motion.html#operator)
- [VS Code Commands API](https://code.visualstudio.com/api/extension-guides/command)
- [Kakoune Selection Model](https://kakoune.org/why-kakoune/why-kakoune.html)
- [xi-editor Plugin Protocol](https://xi-editor.io/docs/plugin.html)
