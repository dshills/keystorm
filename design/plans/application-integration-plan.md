# Keystorm Application Integration Plan

## Executive Summary

This plan details how to bring all 10 core modules together into a runnable application. The codebase is architecturally complete with ~61k LOC across 368 files, but lacks the "glue" layer to assemble components into a running editor.

## Current State

### Completed Modules (All at 100%)
| Module | Purpose | LOC | Key Dependencies |
|--------|---------|-----|-----------------|
| engine | Text buffer with rope/piece tree | ~1000 | stdlib |
| renderer | Terminal display layer | ~2000 | tcell/v2 |
| input | Keystroke/mouse handling | ~1000 | stdlib |
| dispatcher | Action routing | ~2000 | input, engine, event |
| event | Pub/sub messaging bus | ~2000 | stdlib |
| config | Configuration system | ~5000 | toml, yaml |
| project | Workspace management | ~2000 | fsnotify |
| lsp | Language Server Protocol | ~3000 | stdlib |
| plugin | Lua plugin system | ~2000 | gopher-lua |
| integration | Terminal, Git, Debug, Tasks | ~2000 | stdlib |

### What's Missing
1. **Main entry point** (`cmd/keystorm/main.go`)
2. **Application struct** - Central coordinator
3. **Wiring layer** - Component creation and dependency injection
4. **Main event loop** - Input processing and rendering cycle
5. **Lifecycle management** - Startup, shutdown, signal handling
6. **Default configuration** - Built-in defaults

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         cmd/keystorm/main.go                        │
│  - Parse CLI args                                                   │
│  - Create Application                                               │
│  - Run main loop                                                    │
│  - Handle signals                                                   │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    internal/app/application.go                      │
│  Application struct {                                               │
│    eventBus    *event.Bus                                          │
│    config      *config.ConfigSystem                                │
│    engine      *engine.Engine                                      │
│    renderer    *renderer.Renderer                                  │
│    input       *input.Handler                                      │
│    dispatcher  *dispatcher.Dispatcher                              │
│    project     *project.Project                                    │
│    lsp         *lsp.Manager                                        │
│    plugins     *plugin.System                                      │
│    integration *integration.Manager                                │
│  }                                                                  │
└─────────────────────────────────────────────────────────────────────┘
                                    │
          ┌─────────────────────────┼─────────────────────────┐
          │                         │                         │
          ▼                         ▼                         ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ internal/app/   │    │ internal/app/   │    │ internal/app/   │
│   bootstrap.go  │    │   eventloop.go  │    │   lifecycle.go  │
│ - Wire deps     │    │ - Main loop     │    │ - Startup       │
│ - Create comps  │    │ - Process input │    │ - Shutdown      │
│ - Init order    │    │ - Render frame  │    │ - Signal handle │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

---

## Implementation Plan

### Phase 1: Application Foundation

Create the core application structure and CLI entry point.

#### 1.1 Create internal/app package

**File: `internal/app/app.go`**
```go
package app

// Application is the central coordinator for all Keystorm components
type Application struct {
    // Core components
    eventBus    *event.Bus
    config      *config.ConfigSystem

    // Editor components
    engine      *engine.Engine
    renderer    *renderer.Renderer
    inputMgr    *input.Manager
    dispatcher  *dispatcher.Dispatcher
    modes       *mode.Manager

    // Workspace components
    project     project.Project
    lsp         *lsp.Manager

    // Extension components
    plugins     *plugin.System
    integration *integration.Manager

    // State
    running     atomic.Bool
    done        chan struct{}

    // Document management
    documents   *DocumentManager
    activeDoc   *Document
}

// Options configures application creation
type Options struct {
    ConfigPath    string
    WorkspacePath string
    Files         []string
    Debug         bool
    LogLevel      string
}
```

**File: `internal/app/document.go`**
```go
// Document represents an open file with its engine and state
type Document struct {
    Path     string
    Engine   *engine.Engine
    Modified bool
    Language string
    LSPConn  *lsp.Connection
}

// DocumentManager tracks all open documents
type DocumentManager struct {
    documents map[string]*Document
    active    *Document
    mu        sync.RWMutex
}
```

#### 1.2 Create cmd/keystorm entry point

**File: `cmd/keystorm/main.go`**
```go
package main

import (
    "flag"
    "os"
    "os/signal"
    "syscall"

    "keystorm/internal/app"
)

func main() {
    opts := parseFlags()

    application, err := app.New(opts)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
        os.Exit(1)
    }

    // Handle signals
    signals := make(chan os.Signal, 1)
    signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-signals
        application.Shutdown()
    }()

    // Run the application
    if err := application.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

func parseFlags() *app.Options {
    opts := &app.Options{}
    flag.StringVar(&opts.ConfigPath, "config", "", "Path to config file")
    flag.StringVar(&opts.WorkspacePath, "workspace", ".", "Workspace directory")
    flag.BoolVar(&opts.Debug, "debug", false, "Enable debug mode")
    flag.StringVar(&opts.LogLevel, "log-level", "info", "Log level")
    flag.Parse()
    opts.Files = flag.Args()
    return opts
}
```

---

### Phase 2: Bootstrap and Wiring

Wire all components together with proper initialization order.

#### 2.1 Component initialization order

Components must be initialized in dependency order:

```
1. Event Bus       (no deps - messaging foundation)
2. Config System   (needs: event bus for change notifications)
3. Mode Manager    (needs: config for mode settings)
4. Engine          (needs: config for buffer settings)
5. Renderer        (needs: config, event bus)
6. Input Manager   (needs: config, mode manager)
7. Dispatcher      (needs: engine, modes, config, event bus)
8. Project         (needs: config, event bus)
9. LSP Manager     (needs: config, project, event bus)
10. Plugin System  (needs: all above for API exposure)
11. Integration    (needs: config, event bus, project)
```

**File: `internal/app/bootstrap.go`**
```go
package app

func (a *Application) bootstrap(opts *Options) error {
    var err error

    // 1. Event Bus - messaging foundation
    a.eventBus = event.NewBus(event.BusConfig{
        BufferSize:   1000,
        WorkerCount:  4,
        AsyncDefault: false,
    })
    if err := a.eventBus.Start(); err != nil {
        return fmt.Errorf("event bus start: %w", err)
    }

    // 2. Config System
    a.config, err = config.NewConfigSystem(
        config.WithConfigPath(opts.ConfigPath),
        config.WithEventBus(event.NewBusAdapter(a.eventBus)),
        config.WithDefaults(defaultConfig()),
    )
    if err != nil {
        return fmt.Errorf("config: %w", err)
    }

    // 3. Mode Manager
    a.modes = mode.NewManager()
    a.registerModes()

    // 4. Input Manager
    a.inputMgr = input.NewManager(
        input.WithConfig(a.config.Input()),
        input.WithModeManager(a.modes),
    )

    // 5. Renderer
    a.renderer, err = renderer.New(
        renderer.WithConfig(a.config.UI()),
        renderer.WithEventBus(a.eventBus),
    )
    if err != nil {
        return fmt.Errorf("renderer: %w", err)
    }

    // 6. Dispatcher
    a.dispatcher = dispatcher.New(
        dispatcher.WithEventBus(a.eventBus),
        dispatcher.WithConfig(a.config),
    )
    a.registerHandlers()

    // 7. Project/Workspace
    if opts.WorkspacePath != "" {
        a.project, err = project.Open(opts.WorkspacePath,
            project.WithEventBus(a.eventBus),
            project.WithConfig(a.config),
        )
        if err != nil {
            return fmt.Errorf("project: %w", err)
        }
    }

    // 8. LSP Manager
    if a.config.LSP().Enable {
        a.lsp = lsp.NewManager(
            lsp.WithConfig(a.config.LSP()),
            lsp.WithEventBus(a.eventBus),
        )
    }

    // 9. Plugin System
    a.plugins, err = plugin.NewSystem(
        plugin.WithConfig(a.config.Plugins()),
        plugin.WithEventBus(a.eventBus),
        plugin.WithProviders(a.createPluginProviders()),
    )
    if err != nil {
        return fmt.Errorf("plugins: %w", err)
    }

    // 10. Integration Manager
    a.integration = integration.NewManager(
        integration.WithConfig(a.config),
        integration.WithEventBus(a.eventBus),
    )

    // 11. Document Manager
    a.documents = NewDocumentManager(a)

    // Open initial files
    for _, file := range opts.Files {
        if err := a.documents.Open(file); err != nil {
            // Log warning but continue
            a.logWarning("Failed to open %s: %v", file, err)
        }
    }

    // Create scratch buffer if no files opened
    if a.documents.Count() == 0 {
        a.documents.CreateScratch()
    }

    return nil
}

func (a *Application) registerModes() {
    a.modes.Register(mode.Normal, vim.NewNormalMode(a.config.Vim()))
    a.modes.Register(mode.Insert, vim.NewInsertMode(a.config.Vim()))
    a.modes.Register(mode.Visual, vim.NewVisualMode(a.config.Vim()))
    a.modes.Register(mode.Command, vim.NewCommandMode(a.config.Vim()))
    a.modes.Register(mode.Replace, vim.NewReplaceMode(a.config.Vim()))
    a.modes.SetDefault(mode.Normal)
}

func (a *Application) registerHandlers() {
    // Register all dispatcher handlers
    a.dispatcher.RegisterNamespace("cursor", handlers.NewCursorHandler())
    a.dispatcher.RegisterNamespace("editor", handlers.NewEditorHandler())
    a.dispatcher.RegisterNamespace("file", handlers.NewFileHandler(a.documents, a.project))
    a.dispatcher.RegisterNamespace("view", handlers.NewViewHandler(a.renderer))
    a.dispatcher.RegisterNamespace("mode", handlers.NewModeHandler(a.modes))
    a.dispatcher.RegisterNamespace("window", handlers.NewWindowHandler(a.renderer))
    a.dispatcher.RegisterNamespace("search", handlers.NewSearchHandler())
    a.dispatcher.RegisterNamespace("completion", handlers.NewCompletionHandler(a.lsp))
    a.dispatcher.RegisterNamespace("macro", handlers.NewMacroHandler())
    a.dispatcher.RegisterNamespace("operator", handlers.NewOperatorHandler())
    a.dispatcher.RegisterNamespace("integration", handlers.NewIntegrationHandler(a.integration))
}
```

---

### Phase 3: Main Event Loop

Implement the core event loop that drives the editor.

#### 3.1 Event loop architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Main Event Loop                        │
│                                                             │
│  ┌─────────┐    ┌─────────────┐    ┌──────────────┐       │
│  │ Input   │───▶│ Dispatcher  │───▶│ Engine/State │       │
│  │ Events  │    │ (Actions)   │    │ (Updates)    │       │
│  └─────────┘    └─────────────┘    └──────────────┘       │
│       │                                    │               │
│       │         ┌─────────────┐           │               │
│       └────────▶│ Event Bus   │◀──────────┘               │
│                 │ (Pub/Sub)   │                           │
│                 └──────┬──────┘                           │
│                        │                                   │
│  ┌─────────────┬───────┼───────┬─────────────┐           │
│  ▼             ▼       ▼       ▼             ▼           │
│ LSP        Plugins  Renderer  Project   Integration      │
│                         │                                 │
│                         ▼                                 │
│                    ┌─────────┐                            │
│                    │ Screen  │                            │
│                    │ Output  │                            │
│                    └─────────┘                            │
└─────────────────────────────────────────────────────────────┘
```

**File: `internal/app/eventloop.go`**
```go
package app

import (
    "time"
)

const (
    targetFPS     = 60
    frameTime     = time.Second / targetFPS
    inputPollRate = time.Millisecond * 10
)

func (a *Application) Run() error {
    a.running.Store(true)
    defer a.Shutdown()

    // Start the renderer
    if err := a.renderer.Start(); err != nil {
        return fmt.Errorf("renderer start: %w", err)
    }

    // Initialize mode
    a.modes.Switch(mode.Normal)

    // Load plugins
    if err := a.plugins.LoadAll(); err != nil {
        a.logWarning("Some plugins failed to load: %v", err)
    }

    // Main event loop
    return a.eventLoop()
}

func (a *Application) eventLoop() error {
    frameTicker := time.NewTicker(frameTime)
    defer frameTicker.Stop()

    inputCh := a.renderer.Backend().Events()

    for a.running.Load() {
        select {
        case <-a.done:
            return nil

        case ev := <-inputCh:
            if err := a.handleInput(ev); err != nil {
                if err == ErrQuit {
                    return nil
                }
                a.logError("Input handling error: %v", err)
            }

        case <-frameTicker.C:
            a.render()
        }
    }

    return nil
}

func (a *Application) handleInput(ev interface{}) error {
    // Convert tcell event to input event
    inputEv := a.inputMgr.Translate(ev)
    if inputEv == nil {
        return nil
    }

    // Let mode manager process the event
    action, consumed := a.modes.Current().HandleInput(inputEv)
    if !consumed {
        return nil
    }

    // Check for quit action
    if action.ID == "quit" {
        return a.handleQuit()
    }

    // Build execution context
    ctx := a.buildExecutionContext()

    // Dispatch the action
    result := a.dispatcher.Dispatch(ctx, action)
    if result.Error != nil {
        a.showError(result.Error)
    }

    // Mark renderer dirty if state changed
    if result.Status == dispatcher.StatusOK {
        a.renderer.MarkDirty()
    }

    return nil
}

func (a *Application) buildExecutionContext() *execctx.Context {
    doc := a.documents.Active()
    return execctx.New(
        execctx.WithEngine(doc.Engine),
        execctx.WithCursors(doc.Engine.Cursors()),
        execctx.WithModeManager(a.modes),
        execctx.WithRenderer(a.renderer),
        execctx.WithHistory(doc.Engine.History()),
        execctx.WithConfig(a.config),
        execctx.WithProject(a.project),
        execctx.WithDocument(doc),
    )
}

func (a *Application) render() {
    doc := a.documents.Active()
    if doc == nil {
        return
    }

    // Update renderer with current state
    a.renderer.SetBuffer(doc.Engine)
    a.renderer.SetCursors(doc.Engine.Cursors())
    a.renderer.SetMode(a.modes.Current().Name())
    a.renderer.SetFilePath(doc.Path)
    a.renderer.SetModified(doc.Modified)

    // Render if dirty
    a.renderer.RenderIfDirty()
}

func (a *Application) handleQuit() error {
    // Check for unsaved changes
    dirty := a.documents.DirtyDocuments()
    if len(dirty) > 0 {
        // Show confirmation dialog
        // For now, we'll just warn
        a.showWarning("Unsaved changes in %d files", len(dirty))
        // Could prompt user here
    }
    return ErrQuit
}
```

---

### Phase 4: Lifecycle Management

Handle startup, shutdown, and signal processing.

**File: `internal/app/lifecycle.go`**
```go
package app

import (
    "context"
    "sync"
    "time"
)

var ErrQuit = errors.New("quit requested")

func New(opts *Options) (*Application, error) {
    a := &Application{
        done: make(chan struct{}),
    }

    if err := a.bootstrap(opts); err != nil {
        return nil, err
    }

    return a, nil
}

func (a *Application) Shutdown() {
    if !a.running.CompareAndSwap(true, false) {
        return // Already shutting down
    }

    close(a.done)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    var wg sync.WaitGroup

    // Shutdown in reverse initialization order

    // 1. Stop plugins
    wg.Add(1)
    go func() {
        defer wg.Done()
        a.plugins.Shutdown(ctx)
    }()

    // 2. Stop integration
    wg.Add(1)
    go func() {
        defer wg.Done()
        a.integration.Close()
    }()

    // 3. Stop LSP
    if a.lsp != nil {
        wg.Add(1)
        go func() {
            defer wg.Done()
            a.lsp.ShutdownAll(ctx)
        }()
    }

    // Wait for async shutdowns
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
    case <-ctx.Done():
        a.logWarning("Shutdown timed out, forcing exit")
    }

    // 4. Close project
    if a.project != nil {
        a.project.Close()
    }

    // 5. Stop renderer (restore terminal)
    a.renderer.Stop()

    // 6. Stop event bus
    a.eventBus.Stop()

    // 7. Save config if needed
    a.config.Save()
}
```

---

### Phase 5: Event Subscriptions

Wire up event subscriptions between components.

**File: `internal/app/subscriptions.go`**
```go
package app

func (a *Application) setupEventSubscriptions() {
    // Buffer changes -> LSP sync
    a.eventBus.Subscribe(
        "buffer.content.*",
        a.handleBufferChange,
        event.WithPriority(event.PriorityNormal),
    )

    // Buffer changes -> Renderer dirty
    a.eventBus.Subscribe(
        "buffer.content.*",
        func(ev event.Envelope) {
            a.renderer.MarkDirty()
        },
        event.WithPriority(event.PriorityLow),
    )

    // Config changes -> Update components
    a.eventBus.Subscribe(
        "config.changed.*",
        a.handleConfigChange,
        event.WithPriority(event.PriorityHigh),
    )

    // File events -> Project index update
    a.eventBus.Subscribe(
        "file.*",
        func(ev event.Envelope) {
            if a.project != nil {
                a.project.RefreshIndex()
            }
        },
        event.WithPriority(event.PriorityLow),
    )

    // Diagnostics -> Show in status/gutter
    a.eventBus.Subscribe(
        "lsp.diagnostics.*",
        a.handleDiagnostics,
        event.WithPriority(event.PriorityNormal),
    )

    // Mode changes -> Update status line
    a.eventBus.Subscribe(
        "mode.changed",
        func(ev event.Envelope) {
            a.renderer.SetMode(a.modes.Current().Name())
            a.renderer.MarkDirty()
        },
        event.WithPriority(event.PriorityNormal),
    )
}

func (a *Application) handleBufferChange(ev event.Envelope) {
    doc := a.documents.Active()
    if doc == nil || a.lsp == nil {
        return
    }

    // Sync document with LSP
    if doc.LSPConn != nil {
        doc.LSPConn.DidChange(doc.Engine.Text())
    }

    // Mark document as modified
    doc.Modified = true
}

func (a *Application) handleConfigChange(ev event.Envelope) {
    // Reload affected components
    // Theme changes
    if strings.HasPrefix(ev.Topic(), "config.changed.ui.theme") {
        a.renderer.ReloadTheme()
    }

    // Keymap changes
    if strings.HasPrefix(ev.Topic(), "config.changed.keymaps") {
        a.inputMgr.ReloadKeymaps()
    }
}

func (a *Application) handleDiagnostics(ev event.Envelope) {
    // Update gutter decorations
    // Update status line
    a.renderer.MarkDirty()
}
```

---

### Phase 6: Default Configuration

Provide sensible defaults for first-time users.

**File: `internal/app/defaults.go`**
```go
package app

func defaultConfig() map[string]interface{} {
    return map[string]interface{}{
        // Editor defaults
        "editor.tabSize":        4,
        "editor.insertSpaces":   true,
        "editor.lineEnding":     "lf",
        "editor.autoIndent":     true,
        "editor.trimWhitespace": true,

        // UI defaults
        "ui.theme":              "default",
        "ui.lineNumbers":        true,
        "ui.relativeLine":       false,
        "ui.showWhitespace":     false,
        "ui.cursorLine":         true,
        "ui.statusLine":         true,
        "ui.scrolloff":          5,

        // Vim defaults
        "vim.enable":            true,
        "vim.startInNormal":     true,
        "vim.smartCase":         true,
        "vim.ignoreCase":        true,
        "vim.incsearch":         true,
        "vim.hlsearch":          true,

        // Input defaults
        "input.leaderKey":       " ", // Space as leader
        "input.timeout":         1000, // ms for key sequences

        // Files defaults
        "files.encoding":        "utf-8",
        "files.autoSave":        false,
        "files.autoSaveDelay":   1000,

        // Search defaults
        "search.maxResults":     1000,
        "search.caseSensitive":  false,
        "search.regex":          false,

        // LSP defaults
        "lsp.enable":            true,
        "lsp.formatOnSave":      false,
        "lsp.diagnostics":       true,

        // Terminal defaults
        "terminal.shell":        "", // Use $SHELL
        "terminal.scrollback":   10000,

        // Logging
        "logging.level":         "info",
        "logging.file":          "",
    }
}

func defaultKeymaps() map[string]interface{} {
    return map[string]interface{}{
        // Normal mode
        "normal": map[string]string{
            "h":         "cursor.left",
            "j":         "cursor.down",
            "k":         "cursor.up",
            "l":         "cursor.right",
            "w":         "cursor.wordForward",
            "b":         "cursor.wordBackward",
            "e":         "cursor.wordEnd",
            "0":         "cursor.lineStart",
            "$":         "cursor.lineEnd",
            "^":         "cursor.firstNonBlank",
            "gg":        "cursor.documentStart",
            "G":         "cursor.documentEnd",
            "i":         "mode.insert",
            "I":         "mode.insertLineStart",
            "a":         "mode.append",
            "A":         "mode.appendLineEnd",
            "o":         "editor.insertLineBelow",
            "O":         "editor.insertLineAbove",
            "v":         "mode.visual",
            "V":         "mode.visualLine",
            "<C-v>":     "mode.visualBlock",
            "x":         "editor.deleteChar",
            "dd":        "editor.deleteLine",
            "yy":        "editor.yankLine",
            "p":         "editor.pasteAfter",
            "P":         "editor.pasteBefore",
            "u":         "editor.undo",
            "<C-r>":     "editor.redo",
            "/":         "search.forward",
            "?":         "search.backward",
            "n":         "search.next",
            "N":         "search.previous",
            ":":         "mode.command",
            "<Esc>":     "mode.normal",
            "<C-w>h":    "window.focusLeft",
            "<C-w>j":    "window.focusDown",
            "<C-w>k":    "window.focusUp",
            "<C-w>l":    "window.focusRight",
            "<C-w>s":    "window.splitHorizontal",
            "<C-w>v":    "window.splitVertical",
            "<C-w>c":    "window.close",
            "<leader>e": "file.explorer",
            "<leader>f": "file.find",
            "<leader>g": "search.grep",
            "<leader>b": "file.buffers",
        },
        // Insert mode
        "insert": map[string]string{
            "<Esc>":     "mode.normal",
            "<C-[>":     "mode.normal",
            "<BS>":      "editor.backspace",
            "<Del>":     "editor.delete",
            "<CR>":      "editor.newline",
            "<Tab>":     "editor.indent",
            "<S-Tab>":   "editor.unindent",
            "<C-w>":     "editor.deleteWord",
            "<C-u>":     "editor.deleteToLineStart",
            "<C-n>":     "completion.next",
            "<C-p>":     "completion.previous",
        },
        // Visual mode
        "visual": map[string]string{
            "<Esc>":     "mode.normal",
            "d":         "editor.delete",
            "y":         "editor.yank",
            "c":         "editor.change",
            ">":         "editor.indentSelection",
            "<":         "editor.unindentSelection",
        },
        // Command mode
        "command": map[string]string{
            "<Esc>":     "mode.normal",
            "<CR>":      "command.execute",
            "<Tab>":     "command.complete",
            "<C-c>":     "mode.normal",
        },
    }
}
```

---

### Phase 7: Testing and Validation

#### 7.1 Unit tests for app package

**File: `internal/app/app_test.go`**
```go
package app

func TestApplicationBootstrap(t *testing.T) {
    opts := &Options{
        WorkspacePath: t.TempDir(),
    }

    app, err := New(opts)
    require.NoError(t, err)
    require.NotNil(t, app)

    // Verify components initialized
    assert.NotNil(t, app.eventBus)
    assert.NotNil(t, app.config)
    assert.NotNil(t, app.dispatcher)
    assert.NotNil(t, app.modes)

    app.Shutdown()
}

func TestApplicationLifecycle(t *testing.T) {
    app, err := New(&Options{})
    require.NoError(t, err)

    // Start in background
    done := make(chan error)
    go func() {
        done <- app.Run()
    }()

    // Let it run briefly
    time.Sleep(100 * time.Millisecond)

    // Trigger shutdown
    app.Shutdown()

    // Verify clean exit
    err = <-done
    assert.NoError(t, err)
}
```

#### 7.2 Integration tests

**File: `internal/app/integration_test.go`**
```go
func TestFullEditingFlow(t *testing.T) {
    // Create app with test file
    tmpFile := filepath.Join(t.TempDir(), "test.txt")
    os.WriteFile(tmpFile, []byte("Hello World"), 0644)

    app, err := New(&Options{
        Files: []string{tmpFile},
    })
    require.NoError(t, err)
    defer app.Shutdown()

    // Simulate input sequence: i, type text, Esc, :wq
    // Verify file contents
}
```

---

## File Structure Summary

```
cmd/
└── keystorm/
    └── main.go              # CLI entry point

internal/
├── app/
│   ├── app.go               # Application struct and core types
│   ├── bootstrap.go         # Component creation and wiring
│   ├── document.go          # Document and DocumentManager
│   ├── eventloop.go         # Main event loop
│   ├── lifecycle.go         # Startup/shutdown management
│   ├── subscriptions.go     # Event bus subscriptions
│   ├── defaults.go          # Default configuration
│   ├── errors.go            # Application-specific errors
│   ├── app_test.go          # Unit tests
│   └── integration_test.go  # Integration tests
└── [existing modules...]
```

---

## Implementation Order

1. **Phase 1** (Foundation)
   - Create `internal/app/app.go` - Application struct
   - Create `internal/app/document.go` - Document management
   - Create `cmd/keystorm/main.go` - CLI entry point

2. **Phase 2** (Bootstrap)
   - Create `internal/app/bootstrap.go` - Component wiring
   - Create `internal/app/defaults.go` - Default config

3. **Phase 3** (Event Loop)
   - Create `internal/app/eventloop.go` - Main loop
   - Create `internal/app/lifecycle.go` - Lifecycle management

4. **Phase 4** (Integration)
   - Create `internal/app/subscriptions.go` - Event wiring
   - Integration testing

5. **Phase 5** (Polish)
   - Error handling refinement
   - Logging integration
   - Performance optimization

---

## Dependencies Between Modules

```
Event Bus ─────────────────────────────────────────────────────┐
    │                                                          │
    ├──▶ Config System ─────────────────────────────────────┐  │
    │         │                                              │  │
    │         ├──▶ Mode Manager                             │  │
    │         │         │                                    │  │
    │         │         └──▶ Input Manager                  │  │
    │         │                    │                         │  │
    │         ├──▶ Renderer ◀──────┘                        │  │
    │         │                                              │  │
    │         ├──▶ Engine ──────────────────────────────┐   │  │
    │         │                                          │   │  │
    │         └──▶ Dispatcher ◀─────────────────────────┘   │  │
    │                   │                                    │  │
    ├──▶ Project ◀──────┘                                   │  │
    │         │                                              │  │
    ├──▶ LSP Manager ◀──────────────────────────────────────┘  │
    │                                                          │
    ├──▶ Plugin System ◀───────────────────────────────────────┤
    │                                                          │
    └──▶ Integration Manager ◀─────────────────────────────────┘
```

---

## Risk Areas and Mitigations

| Risk | Mitigation |
|------|-----------|
| Thread safety between components | Use event bus for cross-component communication |
| Renderer blocking on slow operations | Async dispatch for long-running handlers |
| Plugin misbehavior | Sandbox with timeouts, capability restrictions |
| LSP server crashes | Automatic restart with backoff |
| Configuration conflicts | Layered config with clear precedence |
| Memory leaks from event subscriptions | Proper unsubscribe on shutdown |

---

## Success Criteria

1. Application starts and shows empty buffer
2. Can open, edit, and save files
3. Vim keybindings work correctly
4. Mode transitions work (Normal/Insert/Visual)
5. Undo/redo functions correctly
6. LSP provides completions for Go files
7. Config changes apply without restart
8. Clean shutdown with no goroutine leaks
9. Plugins can be loaded and activated
10. Integration components accessible (terminal, git)

---

## Estimated Effort

| Phase | Effort |
|-------|--------|
| Phase 1: Foundation | Medium |
| Phase 2: Bootstrap | Medium |
| Phase 3: Event Loop | Medium |
| Phase 4: Integration | Medium |
| Phase 5: Polish | Low |
| Testing | Medium |

---

## Next Steps

1. Review this plan and adjust based on feedback
2. Begin Phase 1 implementation
3. Iterate through phases with testing at each step
4. Continuous integration testing as features come online
