# Keystorm Plugin System - Implementation Plan

## Comprehensive Design Document for `internal/plugin`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Requirements Analysis](#2-requirements-analysis)
3. [Architecture Overview](#3-architecture-overview)
4. [Package Structure](#4-package-structure)
5. [Core Types and Interfaces](#5-core-types-and-interfaces)
6. [Lua Runtime Integration](#6-lua-runtime-integration)
7. [API Surface Design](#7-api-surface-design)
8. [Plugin Lifecycle](#8-plugin-lifecycle)
9. [Security and Sandboxing](#9-security-and-sandboxing)
10. [Event System Integration](#10-event-system-integration)
11. [Hook System](#11-hook-system)
12. [Implementation Phases](#12-implementation-phases)
13. [Testing Strategy](#13-testing-strategy)
14. [Performance Considerations](#14-performance-considerations)

---

## 1. Executive Summary

The Plugin System is Keystorm's extensibility layer, enabling users and third-party developers to extend editor functionality through Lua scripts. Using **gopher-lua** as the embedded runtime, plugins can register commands, create keybindings, hook into editor events, and extend the UI.

### Role in the Architecture

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   Input Engine  │─────▶│   Dispatcher    │◀────▶│  Plugin System  │
│  (keybindings)  │      │  (actions)      │      │  (Lua scripts)  │
└─────────────────┘      └────────┬────────┘      └────────┬────────┘
                                  │                        │
                    ┌─────────────┼─────────────┐          │
                    ▼             ▼             ▼          ▼
            ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌─────────┐
            │   Text    │  │    Mode   │  │  Renderer │  │  Event  │
            │  Engine   │  │  Manager  │  │           │  │   Bus   │
            └───────────┘  └───────────┘  └───────────┘  └─────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Lua via gopher-lua | Lightweight, embeddable, proven in editors (Neovim, Redis), safe sandboxing |
| In-process execution | Low latency for keybindings and real-time features |
| API abstraction layer | Plugins use stable API; internal changes don't break plugins |
| Event-driven model | Plugins subscribe to events; loose coupling with core |
| Capability-based security | Plugins declare required capabilities; user grants permissions |
| Hot-reload support | Edit plugin → reload without restarting editor |

### Integration Points

The Plugin System connects to:
- **Dispatcher**: Registers action handlers for plugin commands
- **Input/Keymap**: Registers custom keybindings
- **Command Palette**: Registers commands for palette discovery
- **Text Engine**: Read/write buffer content via safe API
- **Mode Manager**: Access and modify modes
- **Renderer**: Custom UI overlays, status line contributions
- **Event Bus**: Subscribe to editor events (buffer change, save, mode change)
- **LSP Client**: Access LSP capabilities (completions, diagnostics)
- **Config System**: Read/write plugin configuration

---

## 2. Requirements Analysis

### 2.1 From Architecture Specification

From `design/specs/architecture.md`:

> "The editor's mutation chamber. Both Vim and VS Code delegate huge swaths of capability to plugins: language support, themes, UI elements, macros, AI assistants, you name it."

> "Extension Types: New tools (MCP, CLI, HTTP), New workflows (pipelines/graphs), New UI surfaces/panels, Custom context collectors"

### 2.2 Functional Requirements

1. **Plugin Loading**
   - Load plugins from user directory (`~/.config/keystorm/plugins/`)
   - Load plugins from project directory (`.keystorm/plugins/`)
   - Support single-file plugins (`plugin.lua`)
   - Support directory plugins (`plugin/init.lua` + modules)

2. **Plugin API**
   - Buffer manipulation (read, insert, delete, replace)
   - Cursor operations (get, set, add multi-cursor)
   - Mode access (get current, switch modes)
   - Keymap registration (mode-specific bindings)
   - Command registration (palette commands)
   - UI contributions (status line, overlays)
   - Event subscription (buffer events, lifecycle events)
   - Configuration access (read/write settings)

3. **Plugin Lifecycle**
   - `setup()` called on load with config table
   - `activate()` called when plugin becomes active
   - `deactivate()` called before unload
   - Hot-reload without editor restart

4. **Security Model**
   - Sandboxed Lua environment (no `io`, `os.execute` by default)
   - Capability-based permissions (filesystem, network, shell)
   - User confirmation for sensitive operations
   - Resource limits (memory, execution time)

### 2.3 Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Plugin load time | < 50ms per plugin |
| API call latency | < 100μs for simple operations |
| Memory per plugin | < 10MB default limit |
| Hot-reload time | < 200ms |
| Concurrent plugins | Support 50+ active plugins |

---

## 3. Architecture Overview

### 3.1 High-Level Component Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│                           Plugin System                                   │
├──────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐          │
│  │  Plugin Manager │  │   Lua Runtime   │  │  API Registry   │          │
│  │  - load/unload  │  │  - gopher-lua   │  │  - module map   │          │
│  │  - lifecycle    │  │  - sandboxing   │  │  - versioning   │          │
│  │  - hot-reload   │  │  - state mgmt   │  │  - validation   │          │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘          │
│           │                    │                    │                    │
│  ┌────────▼────────────────────▼────────────────────▼────────┐          │
│  │                     Plugin Host                            │          │
│  │  - per-plugin Lua state                                    │          │
│  │  - API injection                                           │          │
│  │  - event routing                                           │          │
│  │  - error isolation                                         │          │
│  └────────────────────────────────────────────────────────────┘          │
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                      API Modules                                  │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐    │   │
│  │  │ ks.buf  │ │ks.cursor│ │ ks.mode │ │ks.keymap│ │ks.command│   │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘    │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐    │   │
│  │  │ ks.ui   │ │ks.event │ │ks.config│ │ ks.lsp  │ │ ks.util │    │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘    │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Plugin-to-Editor Data Flow

```
Plugin Lua Script
       │
       ▼ (Lua call)
┌──────────────────┐
│  API Bridge      │  ← Go function registered in Lua
│  (ks.buf.insert) │
└────────┬─────────┘
         │ (Go call with validation)
         ▼
┌──────────────────┐
│  Plugin Host     │  ← Permission check, rate limiting
│  (validation)    │
└────────┬─────────┘
         │ (Safe call)
         ▼
┌──────────────────┐
│  Editor Core     │  ← Engine, Dispatcher, etc.
│  (engine.Insert) │
└──────────────────┘
```

### 3.3 Event Flow (Editor → Plugin)

```
Editor Event (e.g., BufferChanged)
         │
         ▼
┌──────────────────┐
│  Event Bus       │
└────────┬─────────┘
         │ (broadcast)
         ▼
┌──────────────────┐
│  Plugin Host     │  ← Filter by subscription
└────────┬─────────┘
         │ (dispatch to subscribers)
         ▼
┌──────────────────┐
│  Plugin Lua      │  ← Async callback execution
│  (on_buf_change) │
└──────────────────┘
```

---

## 4. Package Structure

```
internal/plugin/
├── doc.go                   # Package documentation
├── manager.go               # Plugin manager (load/unload/lifecycle)
├── manager_test.go
├── host.go                  # Per-plugin Lua state host
├── host_test.go
├── loader.go                # Plugin discovery and loading
├── loader_test.go
├── manifest.go              # Plugin manifest parsing (plugin.json)
├── manifest_test.go
├── errors.go                # Error types
│
├── lua/                     # Lua runtime integration
│   ├── state.go             # Lua state management
│   ├── state_test.go
│   ├── sandbox.go           # Sandboxing and security
│   ├── sandbox_test.go
│   ├── bridge.go            # Go-Lua bridge utilities
│   └── bridge_test.go
│
├── api/                     # API modules exposed to Lua
│   ├── registry.go          # API module registry
│   ├── registry_test.go
│   ├── buffer.go            # ks.buf module
│   ├── buffer_test.go
│   ├── cursor.go            # ks.cursor module
│   ├── cursor_test.go
│   ├── mode.go              # ks.mode module
│   ├── mode_test.go
│   ├── keymap.go            # ks.keymap module
│   ├── keymap_test.go
│   ├── command.go           # ks.command module
│   ├── command_test.go
│   ├── ui.go                # ks.ui module
│   ├── ui_test.go
│   ├── event.go             # ks.event module
│   ├── event_test.go
│   ├── config.go            # ks.config module
│   ├── config_test.go
│   ├── lsp.go               # ks.lsp module
│   ├── lsp_test.go
│   ├── util.go              # ks.util module (string, table helpers)
│   └── util_test.go
│
├── security/                # Security subsystem
│   ├── capabilities.go      # Capability definitions
│   ├── permissions.go       # Permission checking
│   ├── permissions_test.go
│   └── limits.go            # Resource limits
│
├── hook/                    # Plugin hooks into dispatcher
│   ├── handler.go           # Plugin-based action handler
│   ├── handler_test.go
│   ├── namespace.go         # Plugin namespace handler
│   └── namespace_test.go
│
└── integration.go           # System facade
    integration_test.go
```

---

## 5. Core Types and Interfaces

### 5.1 Plugin Manifest

```go
// Manifest describes a plugin's metadata and requirements.
type Manifest struct {
    // Identity
    Name        string   `json:"name"`        // Unique identifier (e.g., "vim-surround")
    Version     string   `json:"version"`     // Semver (e.g., "1.2.0")
    DisplayName string   `json:"displayName"` // Human-readable name
    Description string   `json:"description"` // Short description
    Author      string   `json:"author"`      // Author name or org
    License     string   `json:"license"`     // SPDX license identifier
    Homepage    string   `json:"homepage"`    // URL to plugin homepage
    Repository  string   `json:"repository"`  // Git repository URL

    // Entry point
    Main string `json:"main"` // Relative path to main Lua file (default: "init.lua")

    // Requirements
    MinEditorVersion string   `json:"minEditorVersion"` // Minimum Keystorm version
    Dependencies     []string `json:"dependencies"`     // Required plugins

    // Capabilities requested
    Capabilities []Capability `json:"capabilities"`

    // Contributions
    Commands    []CommandContribution    `json:"commands"`
    Keybindings []KeybindingContribution `json:"keybindings"`
    Menus       []MenuContribution       `json:"menus"`

    // Configuration schema
    ConfigSchema map[string]ConfigProperty `json:"configSchema"`
}

// Capability represents a permission the plugin requests.
type Capability string

const (
    CapabilityFileRead    Capability = "filesystem.read"
    CapabilityFileWrite   Capability = "filesystem.write"
    CapabilityNetwork     Capability = "network"
    CapabilityShell       Capability = "shell"
    CapabilityClipboard   Capability = "clipboard"
    CapabilityProcess     Capability = "process.spawn"
    CapabilityUnsafe      Capability = "unsafe" // Full Lua stdlib access
)

// CommandContribution declares a command the plugin provides.
type CommandContribution struct {
    ID          string `json:"id"`          // Command ID (e.g., "myplugin.doThing")
    Title       string `json:"title"`       // Display title
    Description string `json:"description"` // Long description
    Category    string `json:"category"`    // Command category
}

// KeybindingContribution declares default keybindings.
type KeybindingContribution struct {
    Keys    string `json:"keys"`    // Key sequence (e.g., "ctrl+shift+p")
    Command string `json:"command"` // Command to invoke
    When    string `json:"when"`    // Condition expression
    Mode    string `json:"mode"`    // Vim mode (normal, insert, visual)
}

// ConfigProperty describes a configuration option.
type ConfigProperty struct {
    Type        string      `json:"type"`        // string, number, boolean, array, object
    Default     interface{} `json:"default"`     // Default value
    Description string      `json:"description"` // Property description
    Enum        []string    `json:"enum"`        // Allowed values for enum types
}
```

### 5.2 Plugin Interface

```go
// Plugin represents a loaded plugin instance.
type Plugin interface {
    // Identity
    Name() string
    Version() string
    Manifest() *Manifest

    // State
    State() PluginState
    Error() error

    // Lifecycle
    Activate(ctx context.Context) error
    Deactivate(ctx context.Context) error
    Reload(ctx context.Context) error

    // API access
    Call(fn string, args ...interface{}) (interface{}, error)

    // Event handling
    HandleEvent(event Event) error
}

// PluginState represents the plugin lifecycle state.
type PluginState uint8

const (
    StateUnloaded PluginState = iota
    StateLoaded
    StateActivating
    StateActive
    StateDeactivating
    StateError
)
```

### 5.3 Plugin Manager Interface

```go
// Manager manages the plugin lifecycle.
type Manager interface {
    // Discovery
    Discover() ([]PluginInfo, error)

    // Loading
    Load(name string) (Plugin, error)
    LoadAll() error
    Unload(name string) error
    UnloadAll() error

    // Access
    Get(name string) (Plugin, bool)
    List() []Plugin
    ListActive() []Plugin

    // Lifecycle
    Activate(name string) error
    ActivateAll() error
    Deactivate(name string) error
    DeactivateAll() error
    Reload(name string) error

    // Events
    Subscribe(handler EventHandler)
    Unsubscribe(handler EventHandler)
}

// PluginInfo contains discovery information about a plugin.
type PluginInfo struct {
    Name     string
    Path     string
    Manifest *Manifest
    State    PluginState
    Error    error
}
```

### 5.4 Plugin Host Interface

```go
// Host manages a single plugin's Lua state and API access.
type Host interface {
    // Initialization
    Initialize(manifest *Manifest, apiModules []APIModule) error

    // Execution
    DoFile(path string) error
    DoString(code string) error
    Call(fn string, args ...lua.LValue) ([]lua.LValue, error)

    // State access
    GetGlobal(name string) lua.LValue
    SetGlobal(name string, value lua.LValue)

    // Lifecycle
    Close() error

    // Resource management
    SetMemoryLimit(bytes int64)
    SetExecutionTimeout(d time.Duration)
    MemoryUsage() int64
}
```

---

## 6. Lua Runtime Integration

### 6.1 State Management

```go
// luaState wraps gopher-lua with additional features.
type luaState struct {
    L *lua.LState

    // Configuration
    memoryLimit      int64
    executionTimeout time.Duration

    // Tracking
    memoryUsed int64
    startTime  time.Time

    // Sandbox
    sandbox *Sandbox
}

// NewLuaState creates a new sandboxed Lua state.
func NewLuaState(opts ...StateOption) (*luaState, error) {
    L := lua.NewState(lua.Options{
        SkipOpenLibs: true, // We'll open selectively
    })

    state := &luaState{
        L:                L,
        memoryLimit:      DefaultMemoryLimit,
        executionTimeout: DefaultTimeout,
    }

    for _, opt := range opts {
        opt(state)
    }

    // Open safe base libraries
    lua.OpenBase(L)
    lua.OpenTable(L)
    lua.OpenString(L)
    lua.OpenMath(L)
    lua.OpenCoroutine(L)
    // Note: io, os, debug are NOT opened by default

    // Install sandbox hooks
    state.sandbox = NewSandbox(L)
    state.sandbox.Install()

    return state, nil
}
```

### 6.2 Sandbox Implementation

```go
// Sandbox restricts Lua execution to safe operations.
type Sandbox struct {
    L            *lua.LState
    capabilities map[Capability]bool

    // Hooks
    instructionLimit int64
    instructionCount int64
}

// Install sets up the sandbox restrictions.
func (s *Sandbox) Install() {
    // Remove dangerous globals
    for _, name := range []string{"dofile", "loadfile", "load", "loadstring"} {
        s.L.SetGlobal(name, lua.LNil)
    }

    // Install instruction count hook (prevent infinite loops)
    s.L.SetHook(func(L *lua.LState, ar *lua.Debug) {
        s.instructionCount++
        if s.instructionLimit > 0 && s.instructionCount > s.instructionLimit {
            L.RaiseError("execution limit exceeded")
        }
    }, lua.MaskCount, 10000) // Check every 10k instructions

    // Install memory tracking if needed
    // gopher-lua doesn't have native memory limits, but we can track allocations
}

// Grant enables a capability.
func (s *Sandbox) Grant(cap Capability) {
    s.capabilities[cap] = true

    // Inject corresponding modules
    switch cap {
    case CapabilityFileRead:
        s.injectFileReadAPI()
    case CapabilityFileWrite:
        s.injectFileWriteAPI()
    case CapabilityNetwork:
        s.injectNetworkAPI()
    case CapabilityShell:
        s.injectShellAPI()
    }
}
```

### 6.3 Go-Lua Bridge

```go
// bridge provides utilities for Go-Lua interop.
type bridge struct {
    L *lua.LState
}

// RegisterFunc registers a Go function as a Lua function.
func (b *bridge) RegisterFunc(name string, fn func(*lua.LState) int) {
    b.L.SetGlobal(name, b.L.NewFunction(fn))
}

// RegisterModule registers a Go module as a Lua module.
func (b *bridge) RegisterModule(name string, funcs map[string]lua.LGFunction) {
    mod := b.L.SetFuncs(b.L.NewTable(), funcs)
    b.L.SetGlobal(name, mod)
}

// ToGoValue converts a Lua value to a Go value.
func (b *bridge) ToGoValue(lv lua.LValue) interface{} {
    switch v := lv.(type) {
    case lua.LBool:
        return bool(v)
    case lua.LNumber:
        return float64(v)
    case lua.LString:
        return string(v)
    case *lua.LTable:
        return b.tableToMap(v)
    case *lua.LNilType:
        return nil
    default:
        return nil
    }
}

// ToLuaValue converts a Go value to a Lua value.
func (b *bridge) ToLuaValue(v interface{}) lua.LValue {
    switch val := v.(type) {
    case nil:
        return lua.LNil
    case bool:
        return lua.LBool(val)
    case int:
        return lua.LNumber(val)
    case int64:
        return lua.LNumber(val)
    case float64:
        return lua.LNumber(val)
    case string:
        return lua.LString(val)
    case []interface{}:
        return b.sliceToTable(val)
    case map[string]interface{}:
        return b.mapToTable(val)
    default:
        return lua.LNil
    }
}
```

---

## 7. API Surface Design

### 7.1 Module Structure

All API modules are exposed under the `ks` namespace:

```lua
-- Example plugin using the API
local ks = require("ks")

-- Buffer operations
local text = ks.buf.text()
local line = ks.buf.line(1)
ks.buf.insert(0, "Hello, ")
ks.buf.delete(0, 7)

-- Cursor operations
local pos = ks.cursor.get()
ks.cursor.set(100)
ks.cursor.add(200) -- Multi-cursor

-- Mode operations
local mode = ks.mode.current()
ks.mode.switch("insert")

-- Keybindings
ks.keymap.set("normal", "gd", "myplugin.goToDefinition")

-- Commands
ks.command.register({
    id = "myplugin.goToDefinition",
    title = "Go to Definition",
    handler = function()
        -- Implementation
    end
})

-- Events
ks.event.on("BufWrite", function(ev)
    print("Saved: " .. ev.path)
end)

-- Configuration
local setting = ks.config.get("myplugin.timeout")
ks.config.set("myplugin.timeout", 5000)

-- UI
ks.ui.notify("Operation complete", "info")
ks.ui.statusline.set("left", "MyPlugin: Ready")
```

### 7.2 Buffer API (`ks.buf`)

```go
// bufferAPI implements the ks.buf module.
type bufferAPI struct {
    engine execctx.EngineInterface
    host   *Host
}

func (api *bufferAPI) Register(L *lua.LState) {
    mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
        "text":       api.text,       // () -> string
        "text_range": api.textRange,  // (start, end) -> string
        "line":       api.line,       // (lineNum) -> string
        "line_count": api.lineCount,  // () -> number
        "len":        api.len,        // () -> number
        "insert":     api.insert,     // (offset, text) -> end_offset
        "delete":     api.delete,     // (start, end) -> nil
        "replace":    api.replace,    // (start, end, text) -> end_offset
        "undo":       api.undo,       // () -> bool
        "redo":       api.redo,       // () -> bool
        "path":       api.path,       // () -> string (file path)
        "modified":   api.modified,   // () -> bool
    })
    L.SetGlobal("_ks_buf", mod)
}

// insert implements ks.buf.insert(offset, text)
func (api *bufferAPI) insert(L *lua.LState) int {
    offset := L.CheckInt64(1)
    text := L.CheckString(2)

    // Validate
    if offset < 0 {
        L.ArgError(1, "offset must be non-negative")
        return 0
    }

    // Execute
    endOffset, err := api.engine.Insert(buffer.ByteOffset(offset), text)
    if err != nil {
        L.RaiseError("insert failed: %v", err)
        return 0
    }

    L.Push(lua.LNumber(endOffset))
    return 1
}
```

### 7.3 Cursor API (`ks.cursor`)

```go
// cursorAPI implements the ks.cursor module.
type cursorAPI struct {
    cursors execctx.CursorManagerInterface
    host    *Host
}

func (api *cursorAPI) Register(L *lua.LState) {
    mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
        "get":           api.get,           // () -> offset
        "get_all":       api.getAll,        // () -> {offset, ...}
        "set":           api.set,           // (offset) -> nil
        "add":           api.add,           // (offset) -> nil
        "clear":         api.clear,         // () -> nil (clear secondary cursors)
        "selection":     api.selection,     // () -> {start, end} or nil
        "set_selection": api.setSelection,  // (start, end) -> nil
        "count":         api.count,         // () -> number
        "line":          api.line,          // () -> line number
        "column":        api.column,        // () -> column number
        "move":          api.move,          // (delta) -> nil
        "move_to_line":  api.moveToLine,    // (line, col?) -> nil
    })
    L.SetGlobal("_ks_cursor", mod)
}
```

### 7.4 Keymap API (`ks.keymap`)

```go
// keymapAPI implements the ks.keymap module.
type keymapAPI struct {
    registry *keymap.Registry
    host     *Host
}

func (api *keymapAPI) Register(L *lua.LState) {
    mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
        "set":    api.set,    // (mode, keys, action, opts?) -> nil
        "del":    api.del,    // (mode, keys) -> nil
        "get":    api.get,    // (mode, keys) -> action or nil
        "list":   api.list,   // (mode?) -> {bindings...}
    })
    L.SetGlobal("_ks_keymap", mod)
}

// set implements ks.keymap.set(mode, keys, action, opts?)
func (api *keymapAPI) set(L *lua.LState) int {
    mode := L.CheckString(1)
    keys := L.CheckString(2)
    action := L.CheckString(3)

    // Optional options table
    var opts keymapOpts
    if L.GetTop() >= 4 {
        optsTable := L.CheckTable(4)
        opts = api.parseOpts(optsTable)
    }

    // Create binding
    binding := keymap.Binding{
        Keys:        keys,
        Action:      action,
        Description: opts.desc,
        When:        opts.when,
    }

    // Create keymap for this plugin
    km := keymap.NewKeymap(api.host.pluginName + "_" + mode).
        ForMode(mode).
        WithSource("plugin:" + api.host.pluginName).
        AddBinding(binding)

    // Register with system
    if err := api.registry.Register(km); err != nil {
        L.RaiseError("keymap.set failed: %v", err)
    }

    return 0
}
```

### 7.5 Command API (`ks.command`)

```go
// commandAPI implements the ks.command module.
type commandAPI struct {
    palette *palette.Palette
    host    *Host
}

func (api *commandAPI) Register(L *lua.LState) {
    mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
        "register":   api.register,   // (opts) -> nil
        "unregister": api.unregister, // (id) -> nil
        "execute":    api.execute,    // (id, args?) -> result
        "list":       api.list,       // () -> {commands...}
    })
    L.SetGlobal("_ks_command", mod)
}

// register implements ks.command.register(opts)
func (api *commandAPI) register(L *lua.LState) int {
    opts := L.CheckTable(1)

    id := getStringField(L, opts, "id")
    title := getStringField(L, opts, "title")
    handler := L.GetField(opts, "handler")

    if handler.Type() != lua.LTFunction {
        L.ArgError(1, "handler must be a function")
        return 0
    }

    // Wrap Lua handler in Go handler
    goHandler := func(args map[string]any) error {
        L.Push(handler)
        L.Push(api.mapToTable(args))
        if err := L.PCall(1, 0, nil); err != nil {
            return fmt.Errorf("command handler error: %w", err)
        }
        return nil
    }

    cmd := &palette.Command{
        ID:      id,
        Title:   title,
        Handler: goHandler,
        Source:  "plugin:" + api.host.pluginName,
    }

    api.palette.Register(cmd)
    return 0
}
```

### 7.6 Event API (`ks.event`)

```go
// eventAPI implements the ks.event module.
type eventAPI struct {
    bus  *event.Bus
    host *Host

    // Track subscriptions for cleanup
    subscriptions map[string]event.SubscriptionID
}

func (api *eventAPI) Register(L *lua.LState) {
    mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
        "on":       api.on,       // (event, handler) -> subscription_id
        "off":      api.off,      // (subscription_id) -> nil
        "once":     api.once,     // (event, handler) -> subscription_id
        "emit":     api.emit,     // (event, data) -> nil (for plugin events)
    })
    L.SetGlobal("_ks_event", mod)
}

// on implements ks.event.on(eventName, handler)
func (api *eventAPI) on(L *lua.LState) int {
    eventName := L.CheckString(1)
    handler := L.CheckFunction(2)

    // Store handler reference to prevent GC
    handlerRef := L.Ref(lua.LTFunction)

    // Create Go callback
    callback := func(ev event.Event) {
        L.RawGetI(lua.LTFunction, handlerRef)
        L.Push(api.eventToTable(ev))
        if err := L.PCall(1, 0, nil); err != nil {
            api.host.logError("event handler error: %v", err)
        }
    }

    // Subscribe
    subID := api.bus.Subscribe(eventName, callback)
    api.subscriptions[string(subID)] = subID

    L.Push(lua.LString(subID))
    return 1
}
```

### 7.7 UI API (`ks.ui`)

```go
// uiAPI implements the ks.ui module.
type uiAPI struct {
    renderer execctx.RendererInterface
    host     *Host
}

func (api *uiAPI) Register(L *lua.LState) {
    mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
        "notify":       api.notify,       // (msg, level?) -> nil
        "input":        api.input,        // (prompt, default?) -> string or nil
        "select":       api.selectMenu,   // (items, opts?) -> selected or nil
        "confirm":      api.confirm,      // (msg) -> bool
    })

    // Statusline sub-module
    statusline := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
        "set":   api.statuslineSet,   // (position, text) -> nil
        "clear": api.statuslineClear, // (position) -> nil
    })
    L.SetField(mod, "statusline", statusline)

    // Overlay sub-module
    overlay := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
        "create": api.overlayCreate, // (opts) -> overlay_id
        "update": api.overlayUpdate, // (id, opts) -> nil
        "close":  api.overlayClose,  // (id) -> nil
    })
    L.SetField(mod, "overlay", overlay)

    L.SetGlobal("_ks_ui", mod)
}
```

### 7.8 Complete API Module Init

```lua
-- ks.lua - API initialization loaded into each plugin
local ks = {}

-- Load internal modules
ks.buf = _ks_buf
ks.cursor = _ks_cursor
ks.mode = _ks_mode
ks.keymap = _ks_keymap
ks.command = _ks_command
ks.event = _ks_event
ks.config = _ks_config
ks.ui = _ks_ui
ks.lsp = _ks_lsp
ks.util = _ks_util

-- Version info
ks.version = "1.0.0"
ks.api_version = 1

-- Clean up internal globals
_ks_buf = nil
_ks_cursor = nil
_ks_mode = nil
_ks_keymap = nil
_ks_command = nil
_ks_event = nil
_ks_config = nil
_ks_ui = nil
_ks_lsp = nil
_ks_util = nil

return ks
```

---

## 8. Plugin Lifecycle

### 8.1 Discovery and Loading

```go
// Loader discovers and loads plugins.
type Loader struct {
    paths []string // Search paths for plugins
}

// Discover finds all plugins in search paths.
func (l *Loader) Discover() ([]PluginInfo, error) {
    var plugins []PluginInfo

    for _, basePath := range l.paths {
        entries, err := os.ReadDir(basePath)
        if err != nil {
            continue // Skip missing directories
        }

        for _, entry := range entries {
            if !entry.IsDir() {
                continue
            }

            pluginPath := filepath.Join(basePath, entry.Name())
            info, err := l.inspectPlugin(pluginPath)
            if err != nil {
                plugins = append(plugins, PluginInfo{
                    Name:  entry.Name(),
                    Path:  pluginPath,
                    State: StateError,
                    Error: err,
                })
                continue
            }

            plugins = append(plugins, info)
        }
    }

    return plugins, nil
}

// inspectPlugin reads the plugin manifest and validates structure.
func (l *Loader) inspectPlugin(path string) (PluginInfo, error) {
    manifestPath := filepath.Join(path, "plugin.json")

    data, err := os.ReadFile(manifestPath)
    if err != nil {
        // Try plugin.lua as single-file plugin
        luaPath := filepath.Join(path, "init.lua")
        if _, err := os.Stat(luaPath); err == nil {
            return PluginInfo{
                Name:     filepath.Base(path),
                Path:     path,
                Manifest: &Manifest{Name: filepath.Base(path), Main: "init.lua"},
                State:    StateUnloaded,
            }, nil
        }
        return PluginInfo{}, fmt.Errorf("no plugin.json or init.lua found")
    }

    var manifest Manifest
    if err := json.Unmarshal(data, &manifest); err != nil {
        return PluginInfo{}, fmt.Errorf("invalid manifest: %w", err)
    }

    if err := manifest.Validate(); err != nil {
        return PluginInfo{}, fmt.Errorf("manifest validation failed: %w", err)
    }

    return PluginInfo{
        Name:     manifest.Name,
        Path:     path,
        Manifest: &manifest,
        State:    StateUnloaded,
    }, nil
}
```

### 8.2 Plugin Activation Flow

```go
// loadedPlugin represents a plugin with an active Lua state.
type loadedPlugin struct {
    info     PluginInfo
    host     *Host
    state    PluginState
    err      error
    config   map[string]interface{}

    // Cleanup tracking
    commands      []string
    keymaps       []string
    subscriptions []event.SubscriptionID
}

// Activate brings the plugin to active state.
func (p *loadedPlugin) Activate(ctx context.Context) error {
    if p.state != StateLoaded {
        return fmt.Errorf("plugin must be loaded before activation")
    }

    p.state = StateActivating

    // Call setup() with config
    if err := p.callSetup(ctx); err != nil {
        p.state = StateError
        p.err = err
        return err
    }

    // Call activate()
    if err := p.callActivate(ctx); err != nil {
        p.state = StateError
        p.err = err
        return err
    }

    p.state = StateActive
    return nil
}

func (p *loadedPlugin) callSetup(ctx context.Context) error {
    // Check if setup function exists
    L := p.host.LuaState()
    L.GetGlobal("setup")
    if L.Get(-1) == lua.LNil {
        L.Pop(1)
        return nil // setup is optional
    }
    L.Pop(1)

    // Call setup(config)
    configTable := p.host.bridge.ToLuaValue(p.config)
    _, err := p.host.Call("setup", configTable)
    return err
}

func (p *loadedPlugin) callActivate(ctx context.Context) error {
    L := p.host.LuaState()
    L.GetGlobal("activate")
    if L.Get(-1) == lua.LNil {
        L.Pop(1)
        return nil // activate is optional
    }
    L.Pop(1)

    _, err := p.host.Call("activate")
    return err
}
```

### 8.3 Hot Reload

```go
// Reload reloads a plugin without stopping the editor.
func (p *loadedPlugin) Reload(ctx context.Context) error {
    // Deactivate current state
    if p.state == StateActive {
        if err := p.Deactivate(ctx); err != nil {
            return fmt.Errorf("deactivation failed: %w", err)
        }
    }

    // Close old Lua state
    p.host.Close()

    // Create new host
    host, err := NewHost(p.info.Path, p.info.Manifest)
    if err != nil {
        p.state = StateError
        p.err = err
        return err
    }
    p.host = host
    p.state = StateLoaded

    // Re-activate
    return p.Activate(ctx)
}
```

---

## 9. Security and Sandboxing

### 9.1 Capability System

```go
// Permission represents a granted or denied capability.
type Permission struct {
    Capability Capability
    Granted    bool
    GrantedBy  string // "user", "manifest", "default"
    Reason     string
}

// PermissionManager handles capability requests.
type PermissionManager struct {
    defaults  map[Capability]bool
    granted   map[string]map[Capability]Permission // plugin -> capability -> permission
    callbacks map[Capability]PermissionCallback
}

// RequestPermission asks for a capability.
func (pm *PermissionManager) RequestPermission(
    pluginName string,
    cap Capability,
    reason string,
) (bool, error) {
    // Check if already decided
    if perms, ok := pm.granted[pluginName]; ok {
        if perm, ok := perms[cap]; ok {
            return perm.Granted, nil
        }
    }

    // Check default
    if granted, ok := pm.defaults[cap]; ok {
        pm.recordPermission(pluginName, cap, granted, "default", "")
        return granted, nil
    }

    // Prompt user (async)
    if callback, ok := pm.callbacks[cap]; ok {
        granted := callback(pluginName, cap, reason)
        pm.recordPermission(pluginName, cap, granted, "user", reason)
        return granted, nil
    }

    // Deny by default
    pm.recordPermission(pluginName, cap, false, "default", "no callback")
    return false, nil
}
```

### 9.2 Resource Limits

```go
// ResourceLimits defines plugin resource constraints.
type ResourceLimits struct {
    MaxMemory        int64         // Maximum memory in bytes
    MaxExecutionTime time.Duration // Maximum execution time per call
    MaxInstructions  int64         // Maximum Lua instructions per call
    MaxOpenFiles     int           // Maximum open file handles
    MaxNetConns      int           // Maximum network connections
}

// DefaultLimits returns sensible defaults.
func DefaultLimits() ResourceLimits {
    return ResourceLimits{
        MaxMemory:        10 * 1024 * 1024, // 10 MB
        MaxExecutionTime: 5 * time.Second,
        MaxInstructions:  10_000_000,
        MaxOpenFiles:     10,
        MaxNetConns:      5,
    }
}

// ResourceMonitor tracks resource usage.
type ResourceMonitor struct {
    limits  ResourceLimits
    current struct {
        memory      int64
        openFiles   int
        connections int
    }
    mu sync.Mutex
}

// CheckMemory returns an error if memory limit exceeded.
func (rm *ResourceMonitor) CheckMemory(allocating int64) error {
    rm.mu.Lock()
    defer rm.mu.Unlock()

    if rm.current.memory+allocating > rm.limits.MaxMemory {
        return ErrMemoryLimitExceeded
    }
    return nil
}
```

---

## 10. Event System Integration

### 10.1 Event Types

```go
// Event represents an editor event.
type Event struct {
    Type      EventType
    Timestamp time.Time
    Data      map[string]interface{}
}

// EventType identifies the event category.
type EventType string

const (
    // Buffer events
    EventBufCreate    EventType = "BufCreate"
    EventBufRead      EventType = "BufRead"
    EventBufWrite     EventType = "BufWrite"
    EventBufWritePre  EventType = "BufWritePre"
    EventBufWritePost EventType = "BufWritePost"
    EventBufDelete    EventType = "BufDelete"
    EventBufModified  EventType = "BufModified"
    EventTextChanged  EventType = "TextChanged"

    // Cursor events
    EventCursorMoved     EventType = "CursorMoved"
    EventCursorMovedI    EventType = "CursorMovedI"
    EventSelectionChanged EventType = "SelectionChanged"

    // Mode events
    EventModeChanged    EventType = "ModeChanged"
    EventInsertEnter    EventType = "InsertEnter"
    EventInsertLeave    EventType = "InsertLeave"
    EventVisualEnter    EventType = "VisualEnter"
    EventVisualLeave    EventType = "VisualLeave"

    // Window events
    EventWinNew    EventType = "WinNew"
    EventWinClosed EventType = "WinClosed"
    EventWinEnter  EventType = "WinEnter"
    EventWinLeave  EventType = "WinLeave"

    // LSP events
    EventLspAttach     EventType = "LspAttach"
    EventLspDetach     EventType = "LspDetach"
    EventDiagnostics   EventType = "Diagnostics"

    // Plugin lifecycle
    EventPluginLoaded    EventType = "PluginLoaded"
    EventPluginUnloaded  EventType = "PluginUnloaded"
    EventPluginError     EventType = "PluginError"
)
```

### 10.2 Event Bus Integration

```go
// pluginEventBridge connects the event bus to plugins.
type pluginEventBridge struct {
    bus     *event.Bus
    manager *Manager
}

// Setup registers event forwarding.
func (b *pluginEventBridge) Setup() {
    // Forward all events to interested plugins
    b.bus.SubscribeAll(func(ev event.Event) {
        for _, plugin := range b.manager.ListActive() {
            // Check if plugin subscribed to this event
            if plugin.HasSubscription(ev.Type) {
                go func(p Plugin) {
                    if err := p.HandleEvent(ev); err != nil {
                        b.manager.logPluginError(p.Name(), err)
                    }
                }(plugin)
            }
        }
    })
}
```

---

## 11. Hook System

### 11.1 Plugin Hook Handler

```go
// PluginHook enables plugins to hook into the dispatch system.
type PluginHook struct {
    name     string
    priority int
    manager  *Manager
}

// NewPluginHook creates a hook for plugin dispatch integration.
func NewPluginHook(manager *Manager) *PluginHook {
    return &PluginHook{
        name:     "plugin-hook",
        priority: 200, // Plugin priority range
        manager:  manager,
    }
}

func (h *PluginHook) Name() string     { return h.name }
func (h *PluginHook) Priority() int    { return h.priority }

// PreDispatch allows plugins to intercept/modify actions.
func (h *PluginHook) PreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
    // Check if any plugin wants to handle this action
    for _, plugin := range h.manager.ListActive() {
        if result, handled := plugin.InterceptAction(action, ctx); handled {
            if !result {
                return false // Plugin cancelled the action
            }
        }
    }
    return true
}

// PostDispatch notifies plugins of completed actions.
func (h *PluginHook) PostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
    // Notify plugins of action completion
    event := Event{
        Type:      EventType("ActionComplete"),
        Timestamp: time.Now(),
        Data: map[string]interface{}{
            "action": action.Name,
            "status": result.Status.String(),
        },
    }

    for _, plugin := range h.manager.ListActive() {
        _ = plugin.HandleEvent(event)
    }
}
```

### 11.2 Plugin Namespace Handler

```go
// PluginNamespaceHandler routes plugin.* actions to plugins.
type PluginNamespaceHandler struct {
    manager *Manager
}

func (h *PluginNamespaceHandler) Namespace() string { return "plugin" }

func (h *PluginNamespaceHandler) CanHandle(actionName string) bool {
    // Format: plugin.<pluginname>.<action>
    parts := strings.SplitN(actionName, ".", 3)
    if len(parts) < 3 {
        return false
    }
    pluginName := parts[1]
    _, ok := h.manager.Get(pluginName)
    return ok
}

func (h *PluginNamespaceHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
    parts := strings.SplitN(action.Name, ".", 3)
    if len(parts) < 3 {
        return handler.Errorf("invalid plugin action format: %s", action.Name)
    }

    pluginName := parts[1]
    pluginAction := parts[2]

    plugin, ok := h.manager.Get(pluginName)
    if !ok {
        return handler.Errorf("plugin not found: %s", pluginName)
    }

    // Call the plugin's handler
    result, err := plugin.Call("handle_action", map[string]interface{}{
        "action": pluginAction,
        "args":   action.Args.Extra,
        "count":  action.Count,
    })

    if err != nil {
        return handler.Error(err)
    }

    return h.convertResult(result)
}
```

---

## 12. Implementation Phases

### Phase 1: Core Infrastructure (Week 1)

**Goal**: Establish the foundational plugin loading and Lua runtime.

**Tasks**:
1. Implement `lua/state.go` - Lua state management with gopher-lua
2. Implement `lua/sandbox.go` - Basic sandboxing (remove dangerous globals)
3. Implement `lua/bridge.go` - Go-Lua type conversion utilities
4. Implement `manifest.go` - Manifest parsing and validation
5. Implement `loader.go` - Plugin discovery from filesystem
6. Implement `host.go` - Per-plugin host with Lua state

**Deliverables**:
- Can load and execute a simple Lua script
- Manifest parsing works
- Plugin discovery finds plugins in ~/.config/keystorm/plugins/

**Tests**:
- Unit tests for Lua state creation
- Unit tests for sandbox restrictions
- Unit tests for type conversion
- Integration test: load and run trivial plugin

**Success Criteria**:
- Load a plugin that prints "Hello from Lua"
- Sandbox prevents `os.execute()` and `io.open()`

---

### Phase 2: Plugin Manager (Week 2)

**Goal**: Full plugin lifecycle management.

**Tasks**:
1. Implement `manager.go` - Plugin manager with load/unload/activate
2. Implement `security/capabilities.go` - Capability definitions
3. Implement `security/permissions.go` - Permission checking
4. Implement `security/limits.go` - Resource limits
5. Implement `errors.go` - Error types
6. Add hot-reload support

**Deliverables**:
- Manager can load/unload/activate/deactivate plugins
- Capability system works
- Resource limits enforced

**Tests**:
- Unit tests for manager operations
- Unit tests for permission system
- Integration test: full lifecycle

**Success Criteria**:
- Load, activate, deactivate, reload a plugin
- Plugin requesting `filesystem.write` prompts user
- Memory limit terminates runaway plugin

---

### Phase 3: Core API Modules (Week 3)

**Goal**: Implement essential API modules.

**Tasks**:
1. Implement `api/registry.go` - API module registration
2. Implement `api/buffer.go` - ks.buf module
3. Implement `api/cursor.go` - ks.cursor module
4. Implement `api/mode.go` - ks.mode module
5. Implement `api/util.go` - ks.util module

**Deliverables**:
- Plugins can read/write buffer content
- Plugins can manipulate cursors
- Plugins can query/change modes

**Tests**:
- Unit tests for each API function
- Integration test: plugin that inserts text at cursor

**Success Criteria**:
- Plugin can read buffer, insert text, move cursor
- Plugin can switch modes

---

### Phase 4: Keymap and Command APIs (Week 4)

**Goal**: Plugins can register keybindings and commands.

**Tasks**:
1. Implement `api/keymap.go` - ks.keymap module
2. Implement `api/command.go` - ks.command module
3. Implement `hook/handler.go` - Plugin action handler
4. Implement `hook/namespace.go` - Plugin namespace handler
5. Register with dispatcher system

**Deliverables**:
- Plugins can register keybindings
- Plugins can register palette commands
- Plugin actions are dispatchable

**Tests**:
- Unit tests for keymap registration
- Unit tests for command registration
- Integration test: keybinding triggers plugin action

**Success Criteria**:
- Plugin registers `gd` -> `myplugin.goToDefinition`
- Command appears in palette
- Pressing `gd` executes plugin code

---

### Phase 5: Event and UI APIs (Week 5)

**Goal**: Plugins can subscribe to events and contribute UI.

**Tasks**:
1. Implement `api/event.go` - ks.event module
2. Implement `api/ui.go` - ks.ui module
3. Integrate with event bus
4. Add statusline contribution support
5. Add notification support

**Deliverables**:
- Plugins receive editor events
- Plugins can show notifications
- Plugins can contribute to statusline

**Tests**:
- Unit tests for event subscription
- Unit tests for UI functions
- Integration test: plugin reacts to BufWrite

**Success Criteria**:
- Plugin runs code on every save
- Plugin shows notification
- Plugin updates statusline segment

---

### Phase 6: Configuration API (Week 6)

**Goal**: Plugins can read/write configuration.

**Tasks**:
1. Implement `api/config.go` - ks.config module
2. Integrate with config system
3. Support plugin-specific config sections
4. Add config change notifications

**Deliverables**:
- Plugins can read editor settings
- Plugins have their own config namespace
- Config changes trigger plugin callbacks

**Tests**:
- Unit tests for config read/write
- Integration test: plugin responds to config change

**Success Criteria**:
- Plugin reads `myplugin.timeout` setting
- Changing setting triggers plugin callback

---

### Phase 7: LSP API (Week 7)

**Goal**: Plugins can access LSP capabilities.

**Tasks**:
1. Implement `api/lsp.go` - ks.lsp module
2. Expose completion, diagnostics, go-to-definition
3. Support LSP request forwarding
4. Add LSP event subscriptions

**Deliverables**:
- Plugins can query LSP for completions
- Plugins can access diagnostics
- Plugins can trigger go-to-definition

**Tests**:
- Unit tests for LSP API
- Integration test: plugin uses LSP completion

**Success Criteria**:
- Plugin fetches completions at cursor
- Plugin reads diagnostics for current file

---

### Phase 8: Integration and Polish (Week 8)

**Goal**: System integration, documentation, performance.

**Tasks**:
1. Implement `integration.go` - System facade
2. Write package documentation (`doc.go`)
3. Create example plugins
4. Performance optimization
5. Error handling improvements

**Deliverables**:
- Complete system integration
- Comprehensive documentation
- Example plugins: surround, comment, format-on-save
- All tests passing

**Tests**:
- End-to-end integration tests
- Performance benchmarks
- Stress tests (many plugins)

**Success Criteria**:
- Example plugins work correctly
- Plugin load time < 50ms
- API call latency < 100μs
- System handles 50+ plugins

---

## 13. Testing Strategy

### 13.1 Unit Testing

Each module has corresponding `*_test.go` files:

```go
// lua/state_test.go
func TestNewLuaState(t *testing.T) {
    state, err := NewLuaState()
    require.NoError(t, err)
    defer state.Close()

    // Verify base libraries loaded
    assert.NotNil(t, state.GetGlobal("table"))
    assert.NotNil(t, state.GetGlobal("string"))

    // Verify dangerous functions removed
    assert.Equal(t, lua.LNil, state.GetGlobal("dofile"))
    assert.Equal(t, lua.LNil, state.GetGlobal("loadfile"))
}

func TestSandboxExecutionLimit(t *testing.T) {
    state, _ := NewLuaState(WithInstructionLimit(1000))
    defer state.Close()

    // Infinite loop should be terminated
    err := state.DoString("while true do end")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "execution limit")
}
```

### 13.2 Integration Testing

```go
// integration_test.go
func TestPluginLifecycle(t *testing.T) {
    // Create test plugin
    pluginDir := t.TempDir()
    writeTestPlugin(pluginDir, `
        local ks = require("ks")

        local activated = false

        function setup(config)
            assert(config.test_value == 42)
        end

        function activate()
            activated = true
            ks.command.register({
                id = "test.greet",
                title = "Test Greet",
                handler = function()
                    ks.ui.notify("Hello!", "info")
                end
            })
        end

        function deactivate()
            activated = false
        end
    `)

    // Create manager
    manager := NewManager(WithPluginPaths(pluginDir))

    // Load
    plugin, err := manager.Load("testplugin")
    require.NoError(t, err)
    assert.Equal(t, StateLoaded, plugin.State())

    // Activate with config
    err = manager.Activate("testplugin")
    require.NoError(t, err)
    assert.Equal(t, StateActive, plugin.State())

    // Verify command registered
    cmd := palette.Get("test.greet")
    assert.NotNil(t, cmd)

    // Deactivate
    err = manager.Deactivate("testplugin")
    require.NoError(t, err)
    assert.Equal(t, StateLoaded, plugin.State())
}
```

### 13.3 Example Plugin Tests

```go
// examples/surround_test.go
func TestSurroundPlugin(t *testing.T) {
    // Set up editor with test buffer
    system := setupTestSystem(t)
    system.SetContent("hello world")
    system.SetCursor(0) // Position at 'h'

    // Load surround plugin
    err := system.PluginManager().Load("surround")
    require.NoError(t, err)

    // Select "hello" and surround with quotes
    system.SetSelection(0, 5)
    result := system.Dispatch(input.Action{Name: "plugin.surround.add", Args: input.ActionArgs{
        Extra: map[string]interface{}{"char": "\""},
    }})

    assert.Equal(t, handler.StatusOK, result.Status)
    assert.Equal(t, "\"hello\" world", system.Text())
}
```

---

## 14. Performance Considerations

### 14.1 Lua State Pooling

```go
// statePool reuses Lua states for short-lived operations.
type statePool struct {
    pool sync.Pool
}

func newStatePool() *statePool {
    return &statePool{
        pool: sync.Pool{
            New: func() interface{} {
                state, _ := NewLuaState()
                return state
            },
        },
    }
}

func (p *statePool) Get() *luaState {
    return p.pool.Get().(*luaState)
}

func (p *statePool) Put(state *luaState) {
    state.Reset() // Clear globals, reset state
    p.pool.Put(state)
}
```

### 14.2 API Call Batching

```go
// BatchedBufferAPI batches buffer operations for efficiency.
type BatchedBufferAPI struct {
    base    *bufferAPI
    pending []bufferOp
    mu      sync.Mutex
}

func (api *BatchedBufferAPI) Flush() error {
    api.mu.Lock()
    defer api.mu.Unlock()

    if len(api.pending) == 0 {
        return nil
    }

    // Apply all pending ops as single transaction
    api.base.engine.BeginUndoGroup("plugin-batch")
    defer api.base.engine.EndUndoGroup()

    for _, op := range api.pending {
        if err := op.Apply(api.base.engine); err != nil {
            return err
        }
    }

    api.pending = api.pending[:0]
    return nil
}
```

### 14.3 Event Debouncing

```go
// DebouncedEventHandler prevents event storms from overwhelming plugins.
type DebouncedEventHandler struct {
    handler  func(Event)
    debounce time.Duration
    timer    *time.Timer
    lastEvent Event
    mu       sync.Mutex
}

func (d *DebouncedEventHandler) Handle(ev Event) {
    d.mu.Lock()
    defer d.mu.Unlock()

    d.lastEvent = ev

    if d.timer != nil {
        d.timer.Stop()
    }

    d.timer = time.AfterFunc(d.debounce, func() {
        d.mu.Lock()
        ev := d.lastEvent
        d.mu.Unlock()
        d.handler(ev)
    })
}
```

### 14.4 Performance Targets

| Operation | Target | Measurement |
|-----------|--------|-------------|
| Plugin load | < 50ms | From discovery to StateLoaded |
| Plugin activate | < 100ms | From StateLoaded to StateActive |
| API call (simple) | < 100μs | ks.buf.text() |
| API call (edit) | < 1ms | ks.buf.insert() |
| Event dispatch | < 500μs | Event → all plugin handlers called |
| Hot reload | < 200ms | Full reload cycle |

---

## Appendix A: Example Plugin Structure

```
~/.config/keystorm/plugins/
└── vim-surround/
    ├── plugin.json          # Manifest
    ├── init.lua             # Entry point
    ├── lua/
    │   ├── surround.lua     # Main module
    │   └── utils.lua        # Utilities
    └── README.md            # Documentation
```

### plugin.json

```json
{
    "name": "vim-surround",
    "version": "1.0.0",
    "displayName": "Vim Surround",
    "description": "Add, change, delete surrounding pairs",
    "author": "Keystorm Community",
    "license": "MIT",
    "main": "init.lua",
    "minEditorVersion": "0.1.0",
    "capabilities": [],
    "commands": [
        {
            "id": "surround.add",
            "title": "Add Surrounding",
            "description": "Surround selection with pair"
        },
        {
            "id": "surround.change",
            "title": "Change Surrounding",
            "description": "Change surrounding pair"
        },
        {
            "id": "surround.delete",
            "title": "Delete Surrounding",
            "description": "Delete surrounding pair"
        }
    ],
    "keybindings": [
        {"keys": "ys", "command": "surround.add", "mode": "normal"},
        {"keys": "cs", "command": "surround.change", "mode": "normal"},
        {"keys": "ds", "command": "surround.delete", "mode": "normal"},
        {"keys": "S", "command": "surround.add", "mode": "visual"}
    ],
    "configSchema": {
        "pairs": {
            "type": "object",
            "default": {
                "(": {"open": "(", "close": ")"},
                "[": {"open": "[", "close": "]"},
                "{": {"open": "{", "close": "}"},
                "\"": {"open": "\"", "close": "\""},
                "'": {"open": "'", "close": "'"}
            },
            "description": "Surrounding pair definitions"
        }
    }
}
```

### init.lua

```lua
local ks = require("ks")
local surround = require("lua.surround")

local M = {}
local config = {}

function M.setup(opts)
    config = vim.tbl_deep_extend("force", {
        pairs = {
            ["("] = {open = "(", close = ")"},
            ["["] = {open = "[", close = "]"},
            ["{"] = {open = "{", close = "}"},
            ['"'] = {open = '"', close = '"'},
            ["'"] = {open = "'", close = "'"},
        }
    }, opts or {})

    surround.setup(config)
end

function M.activate()
    -- Register commands
    ks.command.register({
        id = "surround.add",
        title = "Add Surrounding",
        handler = function(args)
            local char = args.char or ks.ui.input("Surround with: ")
            if char then
                surround.add(char)
            end
        end
    })

    ks.command.register({
        id = "surround.change",
        title = "Change Surrounding",
        handler = function(args)
            local from = args.from or ks.ui.input("Change from: ")
            local to = args.to or ks.ui.input("Change to: ")
            if from and to then
                surround.change(from, to)
            end
        end
    })

    ks.command.register({
        id = "surround.delete",
        title = "Delete Surrounding",
        handler = function(args)
            local char = args.char or ks.ui.input("Delete surrounding: ")
            if char then
                surround.delete(char)
            end
        end
    })
end

function M.deactivate()
    -- Cleanup handled automatically
end

-- Export module functions
setup = M.setup
activate = M.activate
deactivate = M.deactivate

return M
```

---

## Appendix B: API Quick Reference

### ks.buf
- `text()` → string
- `text_range(start, end)` → string
- `line(n)` → string
- `line_count()` → number
- `len()` → number
- `insert(offset, text)` → end_offset
- `delete(start, end)` → nil
- `replace(start, end, text)` → end_offset
- `undo()` → bool
- `redo()` → bool
- `path()` → string
- `modified()` → bool

### ks.cursor
- `get()` → offset
- `get_all()` → {offsets}
- `set(offset)` → nil
- `add(offset)` → nil
- `clear()` → nil
- `selection()` → {start, end} or nil
- `set_selection(start, end)` → nil
- `count()` → number
- `line()` → number
- `column()` → number

### ks.mode
- `current()` → string
- `switch(mode)` → nil
- `is(mode)` → bool

### ks.keymap
- `set(mode, keys, action, opts?)` → nil
- `del(mode, keys)` → nil
- `get(mode, keys)` → action or nil
- `list(mode?)` → {bindings}

### ks.command
- `register(opts)` → nil
- `unregister(id)` → nil
- `execute(id, args?)` → result
- `list()` → {commands}

### ks.event
- `on(event, handler)` → subscription_id
- `off(subscription_id)` → nil
- `once(event, handler)` → subscription_id
- `emit(event, data)` → nil

### ks.config
- `get(key)` → value
- `set(key, value)` → nil
- `on_change(key, handler)` → subscription_id

### ks.ui
- `notify(msg, level?)` → nil
- `input(prompt, default?)` → string or nil
- `select(items, opts?)` → selected or nil
- `confirm(msg)` → bool
- `statusline.set(position, text)` → nil
- `statusline.clear(position)` → nil

### ks.lsp
- `completions(offset?)` → {items}
- `diagnostics(path?)` → {diagnostics}
- `definition(offset?)` → {locations}
- `references(offset?)` → {locations}
- `hover(offset?)` → {contents}

### ks.util
- `split(str, sep)` → {parts}
- `trim(str)` → string
- `starts_with(str, prefix)` → bool
- `ends_with(str, suffix)` → bool
- `escape_pattern(str)` → string
