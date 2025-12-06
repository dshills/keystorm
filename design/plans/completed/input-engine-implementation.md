# Keystorm Input Engine - Implementation Plan

## Comprehensive Design Document for `internal/input`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Requirements Analysis](#2-requirements-analysis)
3. [Architecture Overview](#3-architecture-overview)
4. [Package Structure](#4-package-structure)
5. [Core Types and Interfaces](#5-core-types-and-interfaces)
6. [Key Event System](#6-key-event-system)
7. [Mode System](#7-mode-system)
8. [Keymap and Bindings](#8-keymap-and-bindings)
9. [Command Palette](#9-command-palette)
10. [Fuzzy Search](#10-fuzzy-search)
11. [Mouse Handling](#11-mouse-handling)
12. [Input Recording and Macros](#12-input-recording-and-macros)
13. [Implementation Phases](#13-implementation-phases)
14. [Testing Strategy](#14-testing-strategy)
15. [Performance Considerations](#15-performance-considerations)

---

## 1. Executive Summary

The Input Engine transforms raw user input (keystrokes, mouse actions, commands) into structured editor commands. This is the nerve center connecting human interaction to editor behavior. Keystorm adopts a **Vim-style modal editing** paradigm while supporting **modeless operation** for accessibility.

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Modal editing (Vim-like) | Power users expect modes; enables single-key commands |
| Mode escape hatch | Allow modeless operation for beginners via config |
| Key sequence parsing | Support multi-key chords (e.g., `g` `g`, `d` `i` `w`) |
| Layered keymaps | User > Filetype > Mode > Default precedence |
| Context-aware dispatch | Same key can mean different things based on context |
| Command palette | Discoverable access to all commands |
| Fuzzy matching | Fast navigation via fuzzy search |
| Macro recording | Record and replay key sequences |
| Plugin hooks | Extensions can intercept/modify input handling |

### Integration Points

The Input Engine connects to:
- **Renderer**: Receives raw key/mouse events from backend
- **Dispatcher**: Sends parsed actions for execution
- **Config**: Loads keymaps, mode settings, user preferences
- **Event Bus**: Publishes input events for plugins/extensions

---

## 2. Requirements Analysis

### 2.1 From Architecture Specification

From `design/specs/architecture.md`:

> "Keystrokes, mouse actions, shortcuts, command palettes, fuzzy search—everything that turns human fidgeting into editor commands. Vim is mode-based. VS Code is not. This difference cascades everywhere."

The Command/Action Dispatcher:

> "Once you press a key or pick a command, someone must decide: 'What does that mean right now?' This dispatcher matches input → action, often with context: mode, selection, active file type, extension overrides, etc."

### 2.2 Functional Requirements

1. **Keystroke Handling**
   - Raw key events from terminal backend
   - Modifier keys (Ctrl, Alt, Shift, Meta/Super)
   - Special keys (Enter, Escape, Tab, Arrow keys, F1-F12)
   - Unicode character input
   - Dead keys and compose sequences

2. **Modal Editing (Vim-style)**
   - Normal mode: Navigation and commands
   - Insert mode: Text entry
   - Visual mode: Selection (char, line, block)
   - Command-line mode: Ex commands
   - Operator-pending mode: Awaiting motion/text object
   - Custom modes (plugin-defined)

3. **Key Sequences**
   - Multi-key commands (`d` `d` = delete line)
   - Counts (`5` `j` = move down 5 lines)
   - Operators + Motions (`d` `w` = delete word)
   - Text objects (`c` `i` `"` = change inside quotes)
   - Timeout for ambiguous sequences

4. **Keymaps and Bindings**
   - Default bindings per mode
   - User overrides
   - Filetype-specific bindings
   - Plugin-registered bindings
   - Binding inheritance and precedence

5. **Command Palette**
   - Searchable list of all commands
   - Fuzzy matching
   - Recent commands history
   - Keyboard navigation
   - Command parameters/arguments

6. **Fuzzy Search**
   - File picker (Ctrl+P style)
   - Symbol search
   - Buffer switcher
   - Generic fuzzy list interface

7. **Mouse Handling**
   - Click to position cursor
   - Drag to select
   - Double-click word select
   - Triple-click line select
   - Scroll wheel
   - Middle-click paste
   - Right-click context menu

8. **Macros**
   - Record key sequences
   - Replay macros
   - Named macro registers
   - Macro persistence

### 2.3 Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Input latency | < 5ms from keypress to action dispatch |
| Key sequence timeout | Configurable, default 1000ms |
| Fuzzy search latency | < 50ms for 10k items |
| Memory overhead | < 1MB for keymap storage |

---

## 3. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Input Engine                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │   KeyEvent   │  │   Mode       │  │      KeySequence         │  │
│  │   Parser     │  │   Manager    │  │        Parser            │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │   Keymap     │  │   Command    │  │      Fuzzy               │  │
│  │   Registry   │  │   Palette    │  │      Matcher             │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │   Mouse      │  │   Macro      │  │      Context             │  │
│  │   Handler    │  │   Recorder   │  │      Provider            │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                       Action Dispatcher                              │
└─────────────────────────────────────────────────────────────────────┘
```

### Data Flow

```
Raw Event (from backend)
       │
       ▼
┌──────────────────┐
│ KeyEvent Parser  │  Normalize to internal representation
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Mode Manager     │  Determine current mode context
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ KeySequence      │  Accumulate multi-key sequences
│ Parser           │  Handle counts, operators, motions
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Keymap Registry  │  Look up binding for sequence + mode + context
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Action           │  Create action with parsed arguments
│ Builder          │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Dispatcher       │  Execute action, update editor state
└──────────────────┘
```

---

## 4. Package Structure

```
internal/input/
    doc.go                      # Package documentation

    # Core types
    types.go                    # Key, Modifier, Event types
    action.go                   # Action type and arguments

    # Main input handler
    handler.go                  # InputHandler (main entry point)
    context.go                  # InputContext (current state)

    # Key event processing
    key/
        key.go                  # Key type and parsing
        event.go                # KeyEvent structure
        modifier.go             # Modifier handling
        sequence.go             # KeySequence type
        parser.go               # Parse raw events to KeyEvent
        key_test.go

    # Mode system
    mode/
        mode.go                 # Mode interface and types
        manager.go              # ModeManager
        normal.go               # Normal mode behavior
        insert.go               # Insert mode behavior
        visual.go               # Visual mode behavior
        command.go              # Command-line mode
        operator.go             # Operator-pending mode
        mode_test.go

    # Keymap system
    keymap/
        keymap.go               # Keymap type
        binding.go              # KeyBinding type
        registry.go             # KeymapRegistry
        loader.go               # Load keymaps from config
        default.go              # Default keymaps
        keymap_test.go

    # Vim-style parsing
    vim/
        parser.go               # VimParser for sequences
        count.go                # Count prefix handling
        operator.go             # Operator definitions
        motion.go               # Motion definitions
        textobj.go              # Text object definitions
        register.go             # Register system (", a-z, etc.)
        vim_test.go

    # Command palette
    palette/
        palette.go              # CommandPalette type
        command.go              # Command registration
        filter.go               # Command filtering/search
        history.go              # Command history
        ui.go                   # UI rendering helpers
        palette_test.go

    # Fuzzy search
    fuzzy/
        matcher.go              # Fuzzy matching algorithm
        scorer.go               # Match scoring
        cache.go                # Result caching
        async.go                # Async matching for large sets
        fuzzy_test.go

    # Mouse handling
    mouse/
        mouse.go                # MouseHandler
        click.go                # Click detection
        drag.go                 # Drag handling
        scroll.go               # Scroll handling
        gesture.go              # Gesture recognition
        mouse_test.go

    # Macro system
    macro/
        recorder.go             # MacroRecorder
        player.go               # MacroPlayer
        register.go             # MacroRegister storage
        persistence.go          # Save/load macros
        macro_test.go

    # Tests
    handler_test.go
    integration_test.go
    bench_test.go
```

### Rationale

- **Separation of concerns**: Each aspect of input handling is isolated
- **Vim-specific logic isolated**: `vim/` package can be swapped for other paradigms
- **Testability**: Components can be tested independently
- **Extensibility**: New modes, commands, and handlers can be added easily

---

## 5. Core Types and Interfaces

### 5.1 Key and Modifier Types

```go
// internal/input/types.go

// Key represents a keyboard key.
type Key uint16

const (
    KeyNone Key = iota

    // Special keys
    KeyEscape
    KeyEnter
    KeyTab
    KeyBackspace
    KeyDelete
    KeyInsert
    KeyHome
    KeyEnd
    KeyPageUp
    KeyPageDown
    KeyUp
    KeyDown
    KeyLeft
    KeyRight

    // Function keys
    KeyF1
    KeyF2
    KeyF3
    KeyF4
    KeyF5
    KeyF6
    KeyF7
    KeyF8
    KeyF9
    KeyF10
    KeyF11
    KeyF12

    // Special
    KeySpace
    KeyPause

    // For character keys, use KeyRune with Rune field
    KeyRune
)

// Modifier represents keyboard modifiers.
type Modifier uint8

const (
    ModNone  Modifier = 0
    ModShift Modifier = 1 << iota
    ModCtrl
    ModAlt
    ModMeta // Cmd on macOS, Win on Windows
)

// String returns a human-readable modifier string.
func (m Modifier) String() string {
    var parts []string
    if m&ModCtrl != 0 {
        parts = append(parts, "Ctrl")
    }
    if m&ModAlt != 0 {
        parts = append(parts, "Alt")
    }
    if m&ModShift != 0 {
        parts = append(parts, "Shift")
    }
    if m&ModMeta != 0 {
        parts = append(parts, "Meta")
    }
    return strings.Join(parts, "+")
}
```

### 5.2 Key Event

```go
// internal/input/key/event.go

// KeyEvent represents a single key press.
type KeyEvent struct {
    Key       Key
    Rune      rune     // For KeyRune events
    Modifiers Modifier
    Timestamp time.Time
}

// IsChar returns true if this is a character key (printable).
func (e KeyEvent) IsChar() bool {
    return e.Key == KeyRune && e.Rune != 0
}

// IsModified returns true if any modifier is pressed (excluding Shift for chars).
func (e KeyEvent) IsModified() bool {
    if e.IsChar() {
        return e.Modifiers&(ModCtrl|ModAlt|ModMeta) != 0
    }
    return e.Modifiers != ModNone
}

// String returns a canonical string representation (e.g., "Ctrl+S", "g").
func (e KeyEvent) String() string {
    var parts []string

    if e.Modifiers&ModCtrl != 0 {
        parts = append(parts, "C")
    }
    if e.Modifiers&ModAlt != 0 {
        parts = append(parts, "A")
    }
    if e.Modifiers&ModMeta != 0 {
        parts = append(parts, "M")
    }
    if e.Modifiers&ModShift != 0 && !e.IsChar() {
        parts = append(parts, "S")
    }

    var keyName string
    switch e.Key {
    case KeyRune:
        if e.Rune == ' ' {
            keyName = "Space"
        } else {
            keyName = string(e.Rune)
        }
    case KeyEscape:
        keyName = "Esc"
    case KeyEnter:
        keyName = "Enter"
    case KeyTab:
        keyName = "Tab"
    case KeyBackspace:
        keyName = "BS"
    case KeyDelete:
        keyName = "Del"
    case KeyUp:
        keyName = "Up"
    case KeyDown:
        keyName = "Down"
    case KeyLeft:
        keyName = "Left"
    case KeyRight:
        keyName = "Right"
    // ... other keys
    default:
        keyName = fmt.Sprintf("Key(%d)", e.Key)
    }

    parts = append(parts, keyName)
    return strings.Join(parts, "-")
}

// Matches checks if this event matches a key specification string.
func (e KeyEvent) Matches(spec string) bool {
    parsed, err := ParseKeySpec(spec)
    if err != nil {
        return false
    }
    return e.Key == parsed.Key &&
           e.Rune == parsed.Rune &&
           e.Modifiers == parsed.Modifiers
}

// ParseKeySpec parses a key specification like "Ctrl+S" or "<C-s>".
func ParseKeySpec(spec string) (KeyEvent, error) {
    // Support both "Ctrl+S" and Vim-style "<C-s>" notation
    // Implementation parses the string and returns KeyEvent
    // ...
}
```

### 5.3 Action Type

```go
// internal/input/action.go

// Action represents a command to be executed.
type Action struct {
    // Name is the command identifier (e.g., "editor.save", "cursor.moveDown")
    Name string

    // Args contains command-specific arguments
    Args ActionArgs

    // Source indicates where this action originated
    Source ActionSource

    // Repeat count (from Vim-style count prefix)
    Count int
}

// ActionArgs holds command arguments.
type ActionArgs struct {
    // Motion for operator commands
    Motion *Motion

    // TextObject for text object commands
    TextObject *TextObject

    // Register for yank/paste
    Register rune

    // Text for insert/replace operations
    Text string

    // Direction for directional commands
    Direction Direction

    // Generic key-value pairs for extensibility
    Extra map[string]interface{}
}

// ActionSource indicates the origin of an action.
type ActionSource uint8

const (
    SourceKeyboard ActionSource = iota
    SourceMouse
    SourcePalette
    SourceMacro
    SourcePlugin
    SourceAPI
)

// Direction for directional commands.
type Direction uint8

const (
    DirNone Direction = iota
    DirUp
    DirDown
    DirLeft
    DirRight
    DirForward
    DirBackward
)
```

### 5.4 Input Handler Interface

```go
// internal/input/handler.go

// InputHandler is the main entry point for input processing.
type InputHandler struct {
    modeManager   *mode.Manager
    keymapReg     *keymap.Registry
    vimParser     *vim.Parser
    commandPalette *palette.Palette
    macroRecorder *macro.Recorder
    mouseHandler  *mouse.Handler

    // Configuration
    config        Config

    // Current state
    context       *InputContext
    keySequence   []KeyEvent
    seqTimeout    *time.Timer

    // Output channel for actions
    actionChan    chan Action

    // Event hooks for plugins
    hooks         []InputHook
}

// Config configures the input handler.
type Config struct {
    // Mode settings
    DefaultMode     string
    EnableModes     bool // false = modeless editing

    // Sequence handling
    SequenceTimeout time.Duration
    ShowPendingKeys bool

    // Mouse
    EnableMouse     bool
    DoubleClickTime time.Duration

    // Clipboard
    UseSystemClipboard bool
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
    return Config{
        DefaultMode:        "normal",
        EnableModes:        true,
        SequenceTimeout:    1000 * time.Millisecond,
        ShowPendingKeys:    true,
        EnableMouse:        true,
        DoubleClickTime:    400 * time.Millisecond,
        UseSystemClipboard: true,
    }
}

// NewInputHandler creates a new input handler.
func NewInputHandler(config Config) *InputHandler {
    h := &InputHandler{
        config:     config,
        actionChan: make(chan Action, 100),
        context:    NewInputContext(),
    }

    h.modeManager = mode.NewManager(config.DefaultMode)
    h.keymapReg = keymap.NewRegistry()
    h.vimParser = vim.NewParser()
    h.commandPalette = palette.New()
    h.macroRecorder = macro.NewRecorder()
    h.mouseHandler = mouse.NewHandler(config.DoubleClickTime)

    // Load default keymaps
    h.keymapReg.LoadDefaults()

    return h
}

// HandleKeyEvent processes a key event.
func (h *InputHandler) HandleKeyEvent(event KeyEvent) {
    // Record for macro if recording
    if h.macroRecorder.IsRecording() {
        h.macroRecorder.Record(event)
    }

    // Run pre-hooks
    for _, hook := range h.hooks {
        if hook.PreKeyEvent(&event) {
            return // Hook consumed the event
        }
    }

    // Add to sequence
    h.keySequence = append(h.keySequence, event)
    h.resetSequenceTimeout()

    // Try to resolve the sequence
    h.resolveSequence()
}

// HandleMouseEvent processes a mouse event.
func (h *InputHandler) HandleMouseEvent(event MouseEvent) {
    action := h.mouseHandler.Handle(event, h.context)
    if action != nil {
        h.dispatchAction(*action)
    }
}

// Actions returns the channel for receiving actions.
func (h *InputHandler) Actions() <-chan Action {
    return h.actionChan
}
```

---

## 6. Key Event System

### 6.1 Key Sequence Handling

```go
// internal/input/key/sequence.go

// KeySequence represents a series of key events.
type KeySequence struct {
    Events []KeyEvent
}

// String returns the sequence as a string (e.g., "g g" or "d i w").
func (s KeySequence) String() string {
    parts := make([]string, len(s.Events))
    for i, e := range s.Events {
        parts[i] = e.String()
    }
    return strings.Join(parts, " ")
}

// Matches checks if this sequence matches a pattern.
func (s KeySequence) Matches(pattern string) bool {
    // Pattern format: "g g", "d <motion>", "C-x C-s"
    // ...
}

// HasPrefix checks if the sequence starts with another sequence.
func (s KeySequence) HasPrefix(prefix KeySequence) bool {
    if len(prefix.Events) > len(s.Events) {
        return false
    }
    for i, e := range prefix.Events {
        if !eventsEqual(e, s.Events[i]) {
            return false
        }
    }
    return true
}

// SequenceParser parses key sequences into actions.
type SequenceParser struct {
    currentMode  string
    pending      []KeyEvent
    timeout      time.Duration
    lastKeyTime  time.Time
}

// ParseResult indicates the outcome of parsing.
type ParseResult struct {
    Status   ParseStatus
    Action   *Action
    Consumed int // Number of events consumed
}

// ParseStatus indicates parsing state.
type ParseStatus uint8

const (
    // ParseComplete: Sequence resolved to an action
    ParseComplete ParseStatus = iota
    // ParsePending: Need more keys to determine action
    ParsePending
    // ParseInvalid: Sequence doesn't match any binding
    ParseInvalid
    // ParseTimeout: Sequence timed out
    ParseTimeout
)
```

### 6.2 Sequence Resolution

```go
// internal/input/handler.go (continued)

// resolveSequence attempts to resolve the current key sequence to an action.
func (h *InputHandler) resolveSequence() {
    currentMode := h.modeManager.Current()
    seq := KeySequence{Events: h.keySequence}

    // First, check for exact match
    if binding := h.keymapReg.Lookup(seq, currentMode.Name(), h.context); binding != nil {
        action := h.buildAction(binding, seq)
        h.clearSequence()
        h.dispatchAction(action)
        return
    }

    // Check for prefix match (more keys might complete a binding)
    if h.keymapReg.HasPrefix(seq, currentMode.Name(), h.context) {
        // Wait for more keys
        return
    }

    // For Vim modes, try Vim-style parsing
    if h.config.EnableModes && currentMode.Name() == "normal" {
        result := h.vimParser.Parse(seq, h.context)
        switch result.Status {
        case ParseComplete:
            h.keySequence = h.keySequence[result.Consumed:]
            h.dispatchAction(*result.Action)
            if len(h.keySequence) > 0 {
                h.resolveSequence() // Recursively process remaining
            }
            return
        case ParsePending:
            return // Wait for more keys
        }
    }

    // No match found - handle based on mode
    if currentMode.Name() == "insert" {
        // In insert mode, unmatched keys are typed as text
        for _, event := range h.keySequence {
            if event.IsChar() && !event.IsModified() {
                h.dispatchAction(Action{
                    Name: "editor.insertChar",
                    Args: ActionArgs{Text: string(event.Rune)},
                })
            }
        }
        h.clearSequence()
    } else {
        // In other modes, invalid sequence
        h.clearSequence()
        // Optionally notify user of invalid key sequence
    }
}

// resetSequenceTimeout resets the key sequence timeout.
func (h *InputHandler) resetSequenceTimeout() {
    if h.seqTimeout != nil {
        h.seqTimeout.Stop()
    }

    h.seqTimeout = time.AfterFunc(h.config.SequenceTimeout, func() {
        h.handleSequenceTimeout()
    })
}

// handleSequenceTimeout handles when key sequence times out.
func (h *InputHandler) handleSequenceTimeout() {
    if len(h.keySequence) == 0 {
        return
    }

    // Try to execute partial match or clear
    h.clearSequence()
}

// clearSequence clears the pending key sequence.
func (h *InputHandler) clearSequence() {
    h.keySequence = h.keySequence[:0]
    if h.seqTimeout != nil {
        h.seqTimeout.Stop()
        h.seqTimeout = nil
    }
}
```

---

## 7. Mode System

### 7.1 Mode Interface

```go
// internal/input/mode/mode.go

// Mode defines editor mode behavior.
type Mode interface {
    // Name returns the mode identifier.
    Name() string

    // DisplayName returns a human-readable name.
    DisplayName() string

    // CursorStyle returns the cursor style for this mode.
    CursorStyle() CursorStyle

    // Enter is called when entering this mode.
    Enter(ctx *ModeContext) error

    // Exit is called when leaving this mode.
    Exit(ctx *ModeContext) error

    // HandleUnmapped handles keys that have no binding.
    HandleUnmapped(event KeyEvent, ctx *ModeContext) *Action
}

// ModeContext provides context to mode handlers.
type ModeContext struct {
    PreviousMode string
    Editor       EditorState
    Selection    *Selection
    Register     rune
}

// CursorStyle defines cursor appearance.
type CursorStyle uint8

const (
    CursorBlock CursorStyle = iota
    CursorBar
    CursorUnderline
)

// EditorState provides read-only access to editor state.
type EditorState interface {
    CursorPosition() (line, col uint32)
    HasSelection() bool
    CurrentLine() string
    LineCount() uint32
    FilePath() string
    FileType() string
    IsModified() bool
}
```

### 7.2 Mode Manager

```go
// internal/input/mode/manager.go

// Manager manages editor modes.
type Manager struct {
    modes       map[string]Mode
    current     Mode
    previous    Mode
    modeStack   []Mode
    onChange    []func(from, to Mode)
}

// NewManager creates a mode manager with default modes.
func NewManager(initialMode string) *Manager {
    m := &Manager{
        modes: make(map[string]Mode),
    }

    // Register default modes
    m.Register(&NormalMode{})
    m.Register(&InsertMode{})
    m.Register(&VisualMode{selectMode: SelectChar})
    m.Register(&VisualLineMode{})
    m.Register(&VisualBlockMode{})
    m.Register(&CommandMode{})
    m.Register(&OperatorPendingMode{})

    // Set initial mode
    if mode, ok := m.modes[initialMode]; ok {
        m.current = mode
    } else {
        m.current = m.modes["normal"]
    }

    return m
}

// Current returns the current mode.
func (m *Manager) Current() Mode {
    return m.current
}

// Switch changes to a different mode.
func (m *Manager) Switch(name string, ctx *ModeContext) error {
    newMode, ok := m.modes[name]
    if !ok {
        return fmt.Errorf("unknown mode: %s", name)
    }

    if m.current != nil {
        if err := m.current.Exit(ctx); err != nil {
            return err
        }
        ctx.PreviousMode = m.current.Name()
    }

    m.previous = m.current
    m.current = newMode

    if err := m.current.Enter(ctx); err != nil {
        return err
    }

    // Notify listeners
    for _, fn := range m.onChange {
        fn(m.previous, m.current)
    }

    return nil
}

// Push pushes a mode onto the stack (for temporary modes).
func (m *Manager) Push(name string, ctx *ModeContext) error {
    m.modeStack = append(m.modeStack, m.current)
    return m.Switch(name, ctx)
}

// Pop returns to the previous stacked mode.
func (m *Manager) Pop(ctx *ModeContext) error {
    if len(m.modeStack) == 0 {
        return m.Switch("normal", ctx)
    }
    prev := m.modeStack[len(m.modeStack)-1]
    m.modeStack = m.modeStack[:len(m.modeStack)-1]
    return m.Switch(prev.Name(), ctx)
}

// OnChange registers a callback for mode changes.
func (m *Manager) OnChange(fn func(from, to Mode)) {
    m.onChange = append(m.onChange, fn)
}

// Register adds a mode to the manager.
func (m *Manager) Register(mode Mode) {
    m.modes[mode.Name()] = mode
}
```

### 7.3 Normal Mode

```go
// internal/input/mode/normal.go

// NormalMode handles normal mode behavior.
type NormalMode struct{}

func (m *NormalMode) Name() string        { return "normal" }
func (m *NormalMode) DisplayName() string { return "NORMAL" }
func (m *NormalMode) CursorStyle() CursorStyle { return CursorBlock }

func (m *NormalMode) Enter(ctx *ModeContext) error {
    // Clear any selection when entering normal mode
    // Position cursor at start of selection if there was one
    return nil
}

func (m *NormalMode) Exit(ctx *ModeContext) error {
    return nil
}

func (m *NormalMode) HandleUnmapped(event KeyEvent, ctx *ModeContext) *Action {
    // In normal mode, unmapped keys do nothing
    return nil
}
```

### 7.4 Insert Mode

```go
// internal/input/mode/insert.go

// InsertMode handles insert mode behavior.
type InsertMode struct{}

func (m *InsertMode) Name() string        { return "insert" }
func (m *InsertMode) DisplayName() string { return "INSERT" }
func (m *InsertMode) CursorStyle() CursorStyle { return CursorBar }

func (m *InsertMode) Enter(ctx *ModeContext) error {
    return nil
}

func (m *InsertMode) Exit(ctx *ModeContext) error {
    // Move cursor back one character (Vim behavior)
    return nil
}

func (m *InsertMode) HandleUnmapped(event KeyEvent, ctx *ModeContext) *Action {
    // In insert mode, unmapped printable characters are inserted
    if event.IsChar() && !event.IsModified() {
        return &Action{
            Name:   "editor.insertChar",
            Args:   ActionArgs{Text: string(event.Rune)},
            Source: SourceKeyboard,
        }
    }
    return nil
}
```

### 7.5 Visual Mode

```go
// internal/input/mode/visual.go

// SelectMode indicates the type of visual selection.
type SelectMode uint8

const (
    SelectChar SelectMode = iota
    SelectLine
    SelectBlock
)

// VisualMode handles visual (selection) mode.
type VisualMode struct {
    selectMode SelectMode
}

func (m *VisualMode) Name() string {
    switch m.selectMode {
    case SelectLine:
        return "visual-line"
    case SelectBlock:
        return "visual-block"
    default:
        return "visual"
    }
}

func (m *VisualMode) DisplayName() string {
    switch m.selectMode {
    case SelectLine:
        return "V-LINE"
    case SelectBlock:
        return "V-BLOCK"
    default:
        return "VISUAL"
    }
}

func (m *VisualMode) CursorStyle() CursorStyle { return CursorBlock }

func (m *VisualMode) Enter(ctx *ModeContext) error {
    // Start selection at current cursor position
    return nil
}

func (m *VisualMode) Exit(ctx *ModeContext) error {
    // Selection is cleared when exiting visual mode
    // unless an operator was applied
    return nil
}

func (m *VisualMode) HandleUnmapped(event KeyEvent, ctx *ModeContext) *Action {
    return nil
}
```

---

## 8. Keymap and Bindings

### 8.1 Keymap Types

```go
// internal/input/keymap/keymap.go

// Keymap holds key bindings for a mode or context.
type Keymap struct {
    Name     string
    Mode     string // Empty means all modes
    FileType string // Empty means all file types
    Bindings []Binding
}

// Binding represents a single key-to-action mapping.
type Binding struct {
    // Keys is the key sequence that triggers this binding.
    // Format: "g g", "d i w", "C-x C-s"
    Keys string

    // Action is the command to execute.
    Action string

    // Args are fixed arguments for the action.
    Args map[string]interface{}

    // When is a condition that must be true for this binding.
    // Format: "editorTextFocus && !editorReadonly"
    When string

    // Description for command palette / help.
    Description string

    // Priority for conflict resolution (higher wins).
    Priority int
}

// ParsedBinding is a binding with parsed key sequence.
type ParsedBinding struct {
    Binding
    Sequence KeySequence
}
```

### 8.2 Keymap Registry

```go
// internal/input/keymap/registry.go

// Registry manages all keymaps and binding lookups.
type Registry struct {
    mu        sync.RWMutex
    keymaps   map[string]*Keymap    // By name
    modeBindings map[string][]ParsedBinding // Mode -> bindings
    prefixTree   *PrefixTree
}

// NewRegistry creates a new keymap registry.
func NewRegistry() *Registry {
    return &Registry{
        keymaps:      make(map[string]*Keymap),
        modeBindings: make(map[string][]ParsedBinding),
        prefixTree:   NewPrefixTree(),
    }
}

// Register adds a keymap to the registry.
func (r *Registry) Register(km *Keymap) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.keymaps[km.Name] = km

    // Parse and index all bindings
    for _, b := range km.Bindings {
        seq, err := ParseKeySequence(b.Keys)
        if err != nil {
            return fmt.Errorf("invalid key sequence %q: %w", b.Keys, err)
        }

        parsed := ParsedBinding{
            Binding:  b,
            Sequence: seq,
        }

        mode := km.Mode
        if mode == "" {
            mode = "*" // Global bindings
        }

        r.modeBindings[mode] = append(r.modeBindings[mode], parsed)
        r.prefixTree.Insert(seq, mode, &parsed)
    }

    return nil
}

// Lookup finds a binding for a key sequence in the current context.
func (r *Registry) Lookup(seq KeySequence, mode string, ctx *InputContext) *Binding {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Check mode-specific bindings first, then global
    for _, m := range []string{mode, "*"} {
        if binding := r.prefixTree.Lookup(seq, m); binding != nil {
            // Check condition
            if binding.When == "" || r.evaluateCondition(binding.When, ctx) {
                return &binding.Binding
            }
        }
    }

    return nil
}

// HasPrefix checks if any binding starts with this sequence.
func (r *Registry) HasPrefix(seq KeySequence, mode string, ctx *InputContext) bool {
    r.mu.RLock()
    defer r.mu.RUnlock()

    for _, m := range []string{mode, "*"} {
        if r.prefixTree.HasPrefix(seq, m) {
            return true
        }
    }

    return false
}

// evaluateCondition evaluates a when condition.
func (r *Registry) evaluateCondition(when string, ctx *InputContext) bool {
    // Simple expression evaluator for conditions like:
    // "editorTextFocus && !editorReadonly"
    // "resourceLangId == go"
    // Implementation uses a basic expression parser
    return true // Simplified
}

// LoadDefaults loads the default keymaps.
func (r *Registry) LoadDefaults() {
    r.Register(DefaultNormalKeymap())
    r.Register(DefaultInsertKeymap())
    r.Register(DefaultVisualKeymap())
    r.Register(DefaultCommandKeymap())
    r.Register(DefaultGlobalKeymap())
}
```

### 8.3 Default Keymaps

```go
// internal/input/keymap/default.go

// DefaultNormalKeymap returns default normal mode bindings.
func DefaultNormalKeymap() *Keymap {
    return &Keymap{
        Name: "default-normal",
        Mode: "normal",
        Bindings: []Binding{
            // Movement
            {Keys: "h", Action: "cursor.left", Description: "Move left"},
            {Keys: "j", Action: "cursor.down", Description: "Move down"},
            {Keys: "k", Action: "cursor.up", Description: "Move up"},
            {Keys: "l", Action: "cursor.right", Description: "Move right"},
            {Keys: "w", Action: "cursor.wordForward", Description: "Move to next word"},
            {Keys: "b", Action: "cursor.wordBackward", Description: "Move to previous word"},
            {Keys: "e", Action: "cursor.wordEnd", Description: "Move to end of word"},
            {Keys: "0", Action: "cursor.lineStart", Description: "Move to line start"},
            {Keys: "$", Action: "cursor.lineEnd", Description: "Move to line end"},
            {Keys: "^", Action: "cursor.firstNonBlank", Description: "Move to first non-blank"},
            {Keys: "g g", Action: "cursor.documentStart", Description: "Go to document start"},
            {Keys: "G", Action: "cursor.documentEnd", Description: "Go to document end"},
            {Keys: "C-d", Action: "scroll.halfPageDown", Description: "Scroll half page down"},
            {Keys: "C-u", Action: "scroll.halfPageUp", Description: "Scroll half page up"},
            {Keys: "C-f", Action: "scroll.pageDown", Description: "Scroll page down"},
            {Keys: "C-b", Action: "scroll.pageUp", Description: "Scroll page up"},

            // Mode switching
            {Keys: "i", Action: "mode.insert", Description: "Enter insert mode"},
            {Keys: "I", Action: "mode.insertLineStart", Description: "Insert at line start"},
            {Keys: "a", Action: "mode.append", Description: "Append after cursor"},
            {Keys: "A", Action: "mode.appendLineEnd", Description: "Append at line end"},
            {Keys: "o", Action: "mode.openBelow", Description: "Open line below"},
            {Keys: "O", Action: "mode.openAbove", Description: "Open line above"},
            {Keys: "v", Action: "mode.visual", Description: "Enter visual mode"},
            {Keys: "V", Action: "mode.visualLine", Description: "Enter visual line mode"},
            {Keys: "C-v", Action: "mode.visualBlock", Description: "Enter visual block mode"},
            {Keys: ":", Action: "mode.command", Description: "Enter command mode"},

            // Operators (require motion or text object)
            {Keys: "d", Action: "operator.delete", Description: "Delete"},
            {Keys: "c", Action: "operator.change", Description: "Change"},
            {Keys: "y", Action: "operator.yank", Description: "Yank (copy)"},
            {Keys: ">", Action: "operator.indent", Description: "Indent"},
            {Keys: "<", Action: "operator.outdent", Description: "Outdent"},
            {Keys: "g u", Action: "operator.lowercase", Description: "Lowercase"},
            {Keys: "g U", Action: "operator.uppercase", Description: "Uppercase"},

            // Quick actions (doubled operator = line operation)
            {Keys: "d d", Action: "editor.deleteLine", Description: "Delete line"},
            {Keys: "y y", Action: "editor.yankLine", Description: "Yank line"},
            {Keys: "c c", Action: "editor.changeLine", Description: "Change line"},
            {Keys: "> >", Action: "editor.indentLine", Description: "Indent line"},
            {Keys: "< <", Action: "editor.outdentLine", Description: "Outdent line"},

            // Other
            {Keys: "x", Action: "editor.deleteChar", Description: "Delete character"},
            {Keys: "r", Action: "editor.replaceChar", Description: "Replace character"},
            {Keys: "p", Action: "editor.pasteAfter", Description: "Paste after"},
            {Keys: "P", Action: "editor.pasteBefore", Description: "Paste before"},
            {Keys: "u", Action: "editor.undo", Description: "Undo"},
            {Keys: "C-r", Action: "editor.redo", Description: "Redo"},
            {Keys: ".", Action: "editor.repeatLast", Description: "Repeat last change"},
            {Keys: "/", Action: "search.forward", Description: "Search forward"},
            {Keys: "?", Action: "search.backward", Description: "Search backward"},
            {Keys: "n", Action: "search.next", Description: "Next search result"},
            {Keys: "N", Action: "search.previous", Description: "Previous search result"},
            {Keys: "*", Action: "search.wordUnderCursor", Description: "Search word under cursor"},
            {Keys: "z z", Action: "view.centerCursor", Description: "Center cursor on screen"},
        },
    }
}

// DefaultInsertKeymap returns default insert mode bindings.
func DefaultInsertKeymap() *Keymap {
    return &Keymap{
        Name: "default-insert",
        Mode: "insert",
        Bindings: []Binding{
            {Keys: "Esc", Action: "mode.normal", Description: "Return to normal mode"},
            {Keys: "C-c", Action: "mode.normal", Description: "Return to normal mode"},
            {Keys: "C-[", Action: "mode.normal", Description: "Return to normal mode"},

            // Insert mode editing
            {Keys: "C-h", Action: "editor.deleteCharBefore", Description: "Delete char before cursor"},
            {Keys: "C-w", Action: "editor.deleteWordBefore", Description: "Delete word before cursor"},
            {Keys: "C-u", Action: "editor.deleteToLineStart", Description: "Delete to line start"},

            // Completion
            {Keys: "C-n", Action: "completion.next", Description: "Next completion"},
            {Keys: "C-p", Action: "completion.previous", Description: "Previous completion"},
            {Keys: "C-Space", Action: "completion.trigger", Description: "Trigger completion"},

            // Movement in insert mode
            {Keys: "C-o", Action: "mode.insertNormalOnce", Description: "Execute one normal command"},
        },
    }
}

// DefaultGlobalKeymap returns global bindings (all modes).
func DefaultGlobalKeymap() *Keymap {
    return &Keymap{
        Name: "default-global",
        Mode: "", // All modes
        Bindings: []Binding{
            // File operations
            {Keys: "C-s", Action: "file.save", Description: "Save file"},
            {Keys: "C-S-s", Action: "file.saveAs", Description: "Save file as"},

            // Command palette
            {Keys: "C-S-p", Action: "palette.show", Description: "Show command palette"},
            {Keys: "C-p", Action: "picker.files", Description: "Show file picker"},

            // Window/buffer
            {Keys: "C-w h", Action: "window.focusLeft", Description: "Focus left window"},
            {Keys: "C-w j", Action: "window.focusDown", Description: "Focus down window"},
            {Keys: "C-w k", Action: "window.focusUp", Description: "Focus up window"},
            {Keys: "C-w l", Action: "window.focusRight", Description: "Focus right window"},
            {Keys: "C-w v", Action: "window.splitVertical", Description: "Split vertical"},
            {Keys: "C-w s", Action: "window.splitHorizontal", Description: "Split horizontal"},
            {Keys: "C-w q", Action: "window.close", Description: "Close window"},
        },
    }
}
```

---

## 9. Command Palette

### 9.1 Palette Types

```go
// internal/input/palette/palette.go

// Palette provides searchable access to all editor commands.
type Palette struct {
    mu        sync.RWMutex
    commands  map[string]*Command
    history   *CommandHistory
    fuzzy     *fuzzy.Matcher
}

// Command represents a registered command.
type Command struct {
    // ID is the unique command identifier.
    ID string

    // Title is the display name.
    Title string

    // Description provides additional context.
    Description string

    // Category for grouping in the palette.
    Category string

    // Keybinding shows the shortcut (for display).
    Keybinding string

    // Handler executes the command.
    Handler CommandHandler

    // Args defines required/optional arguments.
    Args []CommandArg

    // When condition for availability.
    When string

    // Source indicates where the command was registered.
    Source string // "core", "plugin:<name>", etc.
}

// CommandArg defines a command argument.
type CommandArg struct {
    Name        string
    Type        ArgType
    Required    bool
    Default     interface{}
    Description string
    Options     []string // For enum types
}

// ArgType defines argument types.
type ArgType uint8

const (
    ArgString ArgType = iota
    ArgNumber
    ArgBoolean
    ArgFile
    ArgEnum
)

// CommandHandler executes a command.
type CommandHandler func(args map[string]interface{}) error

// New creates a new command palette.
func New() *Palette {
    return &Palette{
        commands: make(map[string]*Command),
        history:  NewCommandHistory(100),
        fuzzy:    fuzzy.NewMatcher(),
    }
}

// Register adds a command to the palette.
func (p *Palette) Register(cmd *Command) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.commands[cmd.ID] = cmd
}

// Unregister removes a command.
func (p *Palette) Unregister(id string) {
    p.mu.Lock()
    defer p.mu.Unlock()
    delete(p.commands, id)
}

// Search returns commands matching the query.
func (p *Palette) Search(query string, limit int) []*Command {
    p.mu.RLock()
    defer p.mu.RUnlock()

    if query == "" {
        // Return recent commands first
        return p.recentCommands(limit)
    }

    // Build searchable items
    items := make([]fuzzy.Item, 0, len(p.commands))
    for _, cmd := range p.commands {
        items = append(items, fuzzy.Item{
            Text: cmd.Title,
            Data: cmd,
        })
    }

    // Fuzzy match
    results := p.fuzzy.Match(query, items, limit)

    commands := make([]*Command, len(results))
    for i, r := range results {
        commands[i] = r.Data.(*Command)
    }
    return commands
}

// Execute runs a command by ID.
func (p *Palette) Execute(id string, args map[string]interface{}) error {
    p.mu.RLock()
    cmd, ok := p.commands[id]
    p.mu.RUnlock()

    if !ok {
        return fmt.Errorf("unknown command: %s", id)
    }

    // Record in history
    p.history.Add(id)

    // Execute
    return cmd.Handler(args)
}
```

### 9.2 Command History

```go
// internal/input/palette/history.go

// CommandHistory tracks recently used commands.
type CommandHistory struct {
    mu       sync.Mutex
    items    []string
    maxItems int
}

// NewCommandHistory creates a command history.
func NewCommandHistory(maxItems int) *CommandHistory {
    return &CommandHistory{
        items:    make([]string, 0, maxItems),
        maxItems: maxItems,
    }
}

// Add records a command execution.
func (h *CommandHistory) Add(id string) {
    h.mu.Lock()
    defer h.mu.Unlock()

    // Remove if already present
    for i, item := range h.items {
        if item == id {
            h.items = append(h.items[:i], h.items[i+1:]...)
            break
        }
    }

    // Add to front
    h.items = append([]string{id}, h.items...)

    // Trim to max size
    if len(h.items) > h.maxItems {
        h.items = h.items[:h.maxItems]
    }
}

// Recent returns recent command IDs.
func (h *CommandHistory) Recent(limit int) []string {
    h.mu.Lock()
    defer h.mu.Unlock()

    if limit > len(h.items) {
        limit = len(h.items)
    }
    result := make([]string, limit)
    copy(result, h.items[:limit])
    return result
}
```

---

## 10. Fuzzy Search

### 10.1 Fuzzy Matcher

```go
// internal/input/fuzzy/matcher.go

// Matcher performs fuzzy string matching.
type Matcher struct {
    cache     *MatchCache
    scorer    Scorer
}

// Item represents a searchable item.
type Item struct {
    Text string
    Data interface{}
}

// Result represents a match result.
type Result struct {
    Item     Item
    Score    int
    Matches  []int // Indices of matched characters
}

// NewMatcher creates a new fuzzy matcher.
func NewMatcher() *Matcher {
    return &Matcher{
        cache:  NewMatchCache(1000),
        scorer: DefaultScorer{},
    }
}

// Match finds items matching the query.
func (m *Matcher) Match(query string, items []Item, limit int) []Result {
    if query == "" {
        // Return first `limit` items with score 0
        results := make([]Result, 0, limit)
        for i := 0; i < len(items) && i < limit; i++ {
            results = append(results, Result{Item: items[i], Score: 0})
        }
        return results
    }

    query = strings.ToLower(query)

    // Check cache
    if cached := m.cache.Get(query); cached != nil {
        return m.filterAndLimit(cached, limit)
    }

    // Match all items
    results := make([]Result, 0, len(items))
    for _, item := range items {
        score, matches := m.matchItem(query, item.Text)
        if score > 0 {
            results = append(results, Result{
                Item:    item,
                Score:   score,
                Matches: matches,
            })
        }
    }

    // Sort by score (descending)
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })

    // Cache results
    m.cache.Set(query, results)

    return m.filterAndLimit(results, limit)
}

// matchItem scores a single item against the query.
func (m *Matcher) matchItem(query, text string) (int, []int) {
    text = strings.ToLower(text)

    // Find matching character positions
    matches := make([]int, 0, len(query))
    queryIdx := 0
    for i := 0; i < len(text) && queryIdx < len(query); i++ {
        if text[i] == query[queryIdx] {
            matches = append(matches, i)
            queryIdx++
        }
    }

    // All query characters must match
    if queryIdx != len(query) {
        return 0, nil
    }

    // Calculate score
    score := m.scorer.Score(query, text, matches)
    return score, matches
}

func (m *Matcher) filterAndLimit(results []Result, limit int) []Result {
    if limit <= 0 || limit > len(results) {
        return results
    }
    return results[:limit]
}
```

### 10.2 Scoring Algorithm

```go
// internal/input/fuzzy/scorer.go

// Scorer calculates match scores.
type Scorer interface {
    Score(query, text string, matches []int) int
}

// DefaultScorer implements a scoring algorithm.
type DefaultScorer struct{}

func (s DefaultScorer) Score(query, text string, matches []int) int {
    if len(matches) == 0 {
        return 0
    }

    score := 0

    // Base score: all characters matched
    score += 100

    // Bonus for consecutive matches
    for i := 1; i < len(matches); i++ {
        if matches[i] == matches[i-1]+1 {
            score += 20 // Consecutive bonus
        }
    }

    // Bonus for matches at word boundaries
    for _, idx := range matches {
        if idx == 0 || text[idx-1] == '/' || text[idx-1] == '_' ||
           text[idx-1] == '-' || text[idx-1] == ' ' {
            score += 15 // Word boundary bonus
        }
        // Bonus for uppercase following lowercase (camelCase)
        if idx > 0 && unicode.IsLower(rune(text[idx-1])) && unicode.IsUpper(rune(text[idx])) {
            score += 15
        }
    }

    // Bonus for prefix match
    if matches[0] == 0 {
        score += 25
    }

    // Penalty for long gaps between matches
    totalGap := matches[len(matches)-1] - matches[0] - len(matches) + 1
    score -= totalGap * 2

    // Penalty for matches at end of string
    endDistance := len(text) - matches[len(matches)-1]
    score -= endDistance

    // Bonus for shorter text (more specific match)
    if len(text) < 20 {
        score += 20 - len(text)
    }

    return score
}
```

---

## 11. Mouse Handling

### 11.1 Mouse Handler

```go
// internal/input/mouse/mouse.go

// MouseEvent represents a mouse input event.
type MouseEvent struct {
    X, Y       int
    Button     MouseButton
    Modifiers  Modifier
    Action     MouseAction
    Timestamp  time.Time
}

// MouseButton identifies mouse buttons.
type MouseButton uint8

const (
    ButtonNone MouseButton = iota
    ButtonLeft
    ButtonMiddle
    ButtonRight
    ButtonScrollUp
    ButtonScrollDown
    ButtonScrollLeft
    ButtonScrollRight
)

// MouseAction identifies mouse actions.
type MouseAction uint8

const (
    MousePress MouseAction = iota
    MouseRelease
    MouseMove
    MouseDrag
)

// Handler processes mouse events.
type Handler struct {
    doubleClickTime time.Duration
    lastClick       *clickInfo
    dragStart       *dragInfo
}

type clickInfo struct {
    pos       Position
    button    MouseButton
    time      time.Time
    count     int
}

type dragInfo struct {
    startPos  Position
    button    MouseButton
    selecting bool
}

// Position represents a screen position.
type Position struct {
    X, Y int
}

// NewHandler creates a mouse handler.
func NewHandler(doubleClickTime time.Duration) *Handler {
    return &Handler{
        doubleClickTime: doubleClickTime,
    }
}

// Handle processes a mouse event and returns an action.
func (h *Handler) Handle(event MouseEvent, ctx *InputContext) *Action {
    switch event.Action {
    case MousePress:
        return h.handlePress(event, ctx)
    case MouseRelease:
        return h.handleRelease(event, ctx)
    case MouseMove:
        return h.handleMove(event, ctx)
    case MouseDrag:
        return h.handleDrag(event, ctx)
    }
    return nil
}

// handlePress handles mouse button press.
func (h *Handler) handlePress(event MouseEvent, ctx *InputContext) *Action {
    pos := Position{X: event.X, Y: event.Y}

    switch event.Button {
    case ButtonLeft:
        // Check for double/triple click
        clickCount := h.detectClickCount(pos, event.Button, event.Timestamp)
        h.lastClick = &clickInfo{
            pos:    pos,
            button: event.Button,
            time:   event.Timestamp,
            count:  clickCount,
        }

        // Start potential drag
        h.dragStart = &dragInfo{
            startPos:  pos,
            button:    event.Button,
            selecting: false,
        }

        switch clickCount {
        case 1:
            return &Action{
                Name: "cursor.setPosition",
                Args: ActionArgs{
                    Extra: map[string]interface{}{
                        "x": event.X,
                        "y": event.Y,
                    },
                },
                Source: SourceMouse,
            }
        case 2:
            return &Action{
                Name:   "selection.word",
                Source: SourceMouse,
            }
        case 3:
            return &Action{
                Name:   "selection.line",
                Source: SourceMouse,
            }
        }

    case ButtonMiddle:
        return &Action{
            Name:   "editor.pasteSelection",
            Source: SourceMouse,
        }

    case ButtonRight:
        return &Action{
            Name: "contextMenu.show",
            Args: ActionArgs{
                Extra: map[string]interface{}{
                    "x": event.X,
                    "y": event.Y,
                },
            },
            Source: SourceMouse,
        }

    case ButtonScrollUp:
        count := 3 // Default scroll lines
        if event.Modifiers&ModShift != 0 {
            count = 1
        }
        if event.Modifiers&ModCtrl != 0 {
            // Ctrl+Scroll = zoom
            return &Action{
                Name:   "view.zoomIn",
                Source: SourceMouse,
            }
        }
        return &Action{
            Name:  "scroll.up",
            Count: count,
            Source: SourceMouse,
        }

    case ButtonScrollDown:
        count := 3
        if event.Modifiers&ModShift != 0 {
            count = 1
        }
        if event.Modifiers&ModCtrl != 0 {
            return &Action{
                Name:   "view.zoomOut",
                Source: SourceMouse,
            }
        }
        return &Action{
            Name:  "scroll.down",
            Count: count,
            Source: SourceMouse,
        }
    }

    return nil
}

// handleDrag handles mouse drag (button held while moving).
func (h *Handler) handleDrag(event MouseEvent, ctx *InputContext) *Action {
    if h.dragStart == nil {
        return nil
    }

    if !h.dragStart.selecting {
        h.dragStart.selecting = true
        // Start selection at drag start position
    }

    return &Action{
        Name: "selection.extendTo",
        Args: ActionArgs{
            Extra: map[string]interface{}{
                "x": event.X,
                "y": event.Y,
            },
        },
        Source: SourceMouse,
    }
}

// detectClickCount determines if this is a double/triple click.
func (h *Handler) detectClickCount(pos Position, button MouseButton, timestamp time.Time) int {
    if h.lastClick == nil {
        return 1
    }

    // Check if same position and button, within time threshold
    if h.lastClick.button != button {
        return 1
    }
    if timestamp.Sub(h.lastClick.time) > h.doubleClickTime {
        return 1
    }
    if abs(pos.X-h.lastClick.pos.X) > 2 || abs(pos.Y-h.lastClick.pos.Y) > 2 {
        return 1
    }

    // Increment click count (max 3)
    count := h.lastClick.count + 1
    if count > 3 {
        count = 1
    }
    return count
}
```

---

## 12. Input Recording and Macros

### 12.1 Macro Recorder

```go
// internal/input/macro/recorder.go

// Recorder records key sequences for macro playback.
type Recorder struct {
    mu         sync.Mutex
    recording  bool
    register   rune
    events     []KeyEvent
    registers  map[rune][]KeyEvent
}

// NewRecorder creates a macro recorder.
func NewRecorder() *Recorder {
    return &Recorder{
        registers: make(map[rune][]KeyEvent),
    }
}

// StartRecording begins recording to a register.
func (r *Recorder) StartRecording(register rune) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if r.recording {
        return fmt.Errorf("already recording")
    }

    r.recording = true
    r.register = register
    r.events = nil
    return nil
}

// StopRecording ends recording and saves the macro.
func (r *Recorder) StopRecording() []KeyEvent {
    r.mu.Lock()
    defer r.mu.Unlock()

    if !r.recording {
        return nil
    }

    r.recording = false
    r.registers[r.register] = r.events
    result := r.events
    r.events = nil
    return result
}

// IsRecording returns true if currently recording.
func (r *Recorder) IsRecording() bool {
    r.mu.Lock()
    defer r.mu.Unlock()
    return r.recording
}

// Record adds a key event to the current recording.
func (r *Recorder) Record(event KeyEvent) {
    r.mu.Lock()
    defer r.mu.Unlock()

    if r.recording {
        r.events = append(r.events, event)
    }
}

// Get retrieves a recorded macro.
func (r *Recorder) Get(register rune) []KeyEvent {
    r.mu.Lock()
    defer r.mu.Unlock()
    return r.registers[register]
}

// Append adds to an existing macro.
func (r *Recorder) Append(register rune, events []KeyEvent) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.registers[register] = append(r.registers[register], events...)
}
```

### 12.2 Macro Player

```go
// internal/input/macro/player.go

// Player replays recorded macros.
type Player struct {
    recorder   *Recorder
    handler    *InputHandler
    playing    bool
    count      int
    cancelChan chan struct{}
}

// NewPlayer creates a macro player.
func NewPlayer(recorder *Recorder, handler *InputHandler) *Player {
    return &Player{
        recorder: recorder,
        handler:  handler,
    }
}

// Play replays a macro from a register.
func (p *Player) Play(register rune, count int) error {
    events := p.recorder.Get(register)
    if len(events) == 0 {
        return fmt.Errorf("empty register: %c", register)
    }

    p.playing = true
    p.count = count
    p.cancelChan = make(chan struct{})

    go func() {
        defer func() { p.playing = false }()

        for i := 0; i < count; i++ {
            for _, event := range events {
                select {
                case <-p.cancelChan:
                    return
                default:
                    p.handler.HandleKeyEvent(event)
                }
            }
        }
    }()

    return nil
}

// IsPlaying returns true if a macro is currently playing.
func (p *Player) IsPlaying() bool {
    return p.playing
}

// Cancel stops macro playback.
func (p *Player) Cancel() {
    if p.playing && p.cancelChan != nil {
        close(p.cancelChan)
    }
}
```

---

## 13. Implementation Phases

### Phase 1: Core Types and Key Events

**Goal**: Establish foundational types and key event parsing.

**Tasks**:
1. `types.go` - Key, Modifier, Action types
2. `key/key.go` - Key type definitions
3. `key/event.go` - KeyEvent structure
4. `key/parser.go` - Parse terminal key events
5. `key/sequence.go` - KeySequence type
6. Basic tests for key parsing

**Success Criteria**:
- Can parse raw terminal events to KeyEvent
- Key specifications parse correctly ("Ctrl+S", "<C-s>", "g g")
- Key sequence comparison works

### Phase 2: Mode System

**Goal**: Implement modal editing infrastructure.

**Tasks**:
1. `mode/mode.go` - Mode interface
2. `mode/manager.go` - ModeManager
3. `mode/normal.go` - Normal mode
4. `mode/insert.go` - Insert mode
5. `mode/visual.go` - Visual modes
6. `mode/command.go` - Command-line mode
7. Tests for mode transitions

**Success Criteria**:
- Mode switching works correctly
- Mode-specific cursor styles
- Enter/exit hooks fire properly

### Phase 3: Keymap System

**Goal**: Implement key binding lookup and management.

**Tasks**:
1. `keymap/keymap.go` - Keymap type
2. `keymap/binding.go` - Binding definitions
3. `keymap/registry.go` - Registry with prefix tree
4. `keymap/loader.go` - Load keymaps from config
5. `keymap/default.go` - Default keymaps
6. Tests for binding lookup

**Success Criteria**:
- Bindings resolve correctly for mode + sequence
- Prefix detection for multi-key sequences
- Priority and condition evaluation works

### Phase 4: Basic Input Handler

**Goal**: Wire together key parsing, modes, and keymaps.

**Tasks**:
1. `handler.go` - InputHandler main logic
2. `context.go` - InputContext for state
3. Sequence timeout handling
4. Action dispatch
5. Integration tests

**Success Criteria**:
- Key presses map to actions
- Multi-key sequences work
- Sequence timeout clears pending keys

### Phase 5: Vim-Style Parsing

**Goal**: Implement Vim operators, motions, and text objects.

**Tasks**:
1. `vim/parser.go` - VimParser
2. `vim/count.go` - Count prefix handling
3. `vim/operator.go` - Operator definitions (d, c, y, etc.)
4. `vim/motion.go` - Motion definitions (w, e, b, etc.)
5. `vim/textobj.go` - Text objects (iw, a", etc.)
6. `vim/register.go` - Register system
7. Comprehensive tests

**Success Criteria**:
- `5j` moves down 5 lines
- `diw` deletes inner word
- `"ayw` yanks word to register a
- Operators + motions compose correctly

### Phase 6: Command Palette

**Goal**: Implement searchable command interface.

**Tasks**:
1. `palette/palette.go` - Palette type
2. `palette/command.go` - Command registration
3. `palette/filter.go` - Search/filter logic
4. `palette/history.go` - Command history
5. Integration with input handler

**Success Criteria**:
- Commands searchable by name
- Recent commands appear first
- Command execution works

### Phase 7: Fuzzy Search

**Goal**: Implement general-purpose fuzzy matching.

**Tasks**:
1. `fuzzy/matcher.go` - Fuzzy matching algorithm
2. `fuzzy/scorer.go` - Match scoring
3. `fuzzy/cache.go` - Result caching
4. `fuzzy/async.go` - Async matching for large sets
5. Benchmarks

**Success Criteria**:
- Fuzzy matching < 50ms for 10k items
- Scoring favors prefix/word boundary matches
- Cache improves repeated queries

### Phase 8: Mouse Handling

**Goal**: Implement comprehensive mouse support.

**Tasks**:
1. `mouse/mouse.go` - MouseHandler
2. `mouse/click.go` - Click/double-click detection
3. `mouse/drag.go` - Drag handling
4. `mouse/scroll.go` - Scroll wheel
5. Integration with input handler

**Success Criteria**:
- Click positions cursor
- Double-click selects word
- Drag creates selection
- Scroll works with modifiers

### Phase 9: Macro System

**Goal**: Implement macro recording and playback.

**Tasks**:
1. `macro/recorder.go` - MacroRecorder
2. `macro/player.go` - MacroPlayer
3. `macro/register.go` - Register storage
4. `macro/persistence.go` - Save/load macros
5. Integration tests

**Success Criteria**:
- Record keystrokes to registers
- Playback with count
- Persist macros across sessions

### Phase 10: Polish and Integration

**Goal**: Optimize, polish, and integrate with editor.

**Tasks**:
1. Performance profiling and optimization
2. Error handling and edge cases
3. Input hook system for plugins
4. Documentation
5. Integration with dispatcher and renderer

**Success Criteria**:
- Input latency < 5ms
- No dropped key events
- Hooks work for plugins
- Full integration with editor

---

## 14. Testing Strategy

### 14.1 Unit Tests

```go
// Example: Key event parsing
func TestKeyEventParsing(t *testing.T) {
    tests := []struct {
        spec     string
        expected KeyEvent
    }{
        {"a", KeyEvent{Key: KeyRune, Rune: 'a'}},
        {"A", KeyEvent{Key: KeyRune, Rune: 'A', Modifiers: ModShift}},
        {"Ctrl+s", KeyEvent{Key: KeyRune, Rune: 's', Modifiers: ModCtrl}},
        {"C-s", KeyEvent{Key: KeyRune, Rune: 's', Modifiers: ModCtrl}},
        {"<C-S-a>", KeyEvent{Key: KeyRune, Rune: 'a', Modifiers: ModCtrl | ModShift}},
        {"Esc", KeyEvent{Key: KeyEscape}},
        {"<CR>", KeyEvent{Key: KeyEnter}},
    }

    for _, tt := range tests {
        t.Run(tt.spec, func(t *testing.T) {
            got, err := ParseKeySpec(tt.spec)
            if err != nil {
                t.Fatalf("ParseKeySpec(%q): %v", tt.spec, err)
            }
            if got.Key != tt.expected.Key || got.Rune != tt.expected.Rune ||
               got.Modifiers != tt.expected.Modifiers {
                t.Errorf("got %+v, want %+v", got, tt.expected)
            }
        })
    }
}

// Example: Mode transitions
func TestModeTransitions(t *testing.T) {
    mgr := mode.NewManager("normal")

    // Start in normal
    if mgr.Current().Name() != "normal" {
        t.Errorf("expected normal mode, got %s", mgr.Current().Name())
    }

    // Switch to insert
    ctx := &mode.ModeContext{}
    mgr.Switch("insert", ctx)
    if mgr.Current().Name() != "insert" {
        t.Errorf("expected insert mode, got %s", mgr.Current().Name())
    }

    // Previous should be normal
    if ctx.PreviousMode != "normal" {
        t.Errorf("expected previous mode normal, got %s", ctx.PreviousMode)
    }
}

// Example: Keymap lookup
func TestKeymapLookup(t *testing.T) {
    reg := keymap.NewRegistry()
    reg.LoadDefaults()

    // Test single key
    seq := KeySequence{Events: []KeyEvent{{Key: KeyRune, Rune: 'j'}}}
    binding := reg.Lookup(seq, "normal", &InputContext{})
    if binding == nil {
        t.Fatal("expected binding for 'j'")
    }
    if binding.Action != "cursor.down" {
        t.Errorf("expected cursor.down, got %s", binding.Action)
    }

    // Test sequence
    seq = KeySequence{Events: []KeyEvent{
        {Key: KeyRune, Rune: 'g'},
        {Key: KeyRune, Rune: 'g'},
    }}
    binding = reg.Lookup(seq, "normal", &InputContext{})
    if binding == nil {
        t.Fatal("expected binding for 'g g'")
    }
    if binding.Action != "cursor.documentStart" {
        t.Errorf("expected cursor.documentStart, got %s", binding.Action)
    }
}
```

### 14.2 Integration Tests

```go
// Test full input -> action flow
func TestInputToAction(t *testing.T) {
    handler := NewInputHandler(DefaultConfig())

    // Simulate typing "dd" in normal mode
    handler.HandleKeyEvent(KeyEvent{Key: KeyRune, Rune: 'd'})
    handler.HandleKeyEvent(KeyEvent{Key: KeyRune, Rune: 'd'})

    select {
    case action := <-handler.Actions():
        if action.Name != "editor.deleteLine" {
            t.Errorf("expected editor.deleteLine, got %s", action.Name)
        }
    case <-time.After(100 * time.Millisecond):
        t.Fatal("timeout waiting for action")
    }
}

// Test mode switching
func TestModeSwitch(t *testing.T) {
    handler := NewInputHandler(DefaultConfig())

    // Press 'i' to enter insert mode
    handler.HandleKeyEvent(KeyEvent{Key: KeyRune, Rune: 'i'})

    select {
    case action := <-handler.Actions():
        if action.Name != "mode.insert" {
            t.Errorf("expected mode.insert, got %s", action.Name)
        }
    case <-time.After(100 * time.Millisecond):
        t.Fatal("timeout waiting for action")
    }

    // Verify mode changed
    if handler.CurrentMode() != "insert" {
        t.Errorf("expected insert mode, got %s", handler.CurrentMode())
    }
}
```

### 14.3 Benchmark Tests

```go
func BenchmarkKeyLookup(b *testing.B) {
    reg := keymap.NewRegistry()
    reg.LoadDefaults()
    seq := KeySequence{Events: []KeyEvent{{Key: KeyRune, Rune: 'j'}}}
    ctx := &InputContext{}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        reg.Lookup(seq, "normal", ctx)
    }
}

func BenchmarkFuzzyMatch(b *testing.B) {
    matcher := fuzzy.NewMatcher()

    // Generate 10k items
    items := make([]fuzzy.Item, 10000)
    for i := range items {
        items[i] = fuzzy.Item{Text: fmt.Sprintf("file%d.go", i)}
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        matcher.Match("file123", items, 10)
    }
}
```

---

## 15. Performance Considerations

### 15.1 Key Event Latency

**Goal**: < 5ms from raw event to action dispatch.

**Optimizations**:
- Pre-parse all keymap bindings at startup
- Use radix/prefix tree for O(k) lookup (k = key sequence length)
- Avoid allocations in hot path
- Use sync.Pool for temporary objects

### 15.2 Fuzzy Search Performance

**Goal**: < 50ms for 10k items.

**Optimizations**:
- Pre-compute lowercase versions of items
- Use byte-level comparison (avoid rune iteration)
- Cache results for repeated queries
- Use goroutines for parallel matching of large sets
- Limit initial scan with early termination

### 15.3 Memory Usage

**Goal**: < 1MB for keymap storage.

**Strategies**:
- Intern common strings (action names, key specs)
- Share bindings between similar modes
- Lazy loading of rarely-used keymaps
- Evict command palette search cache on memory pressure

### 15.4 Sequence Handling

**Optimizations**:
- Fixed-size array for short sequences (< 4 keys)
- Timer pooling for sequence timeouts
- Early exit when no prefix matches

---

## Dependencies

External packages required:

| Package | Purpose |
|---------|---------|
| (none for core) | Input engine is pure Go |

Internal dependencies:

| Package | Purpose |
|---------|---------|
| `internal/renderer/backend` | Raw key/mouse events |
| `internal/dispatcher` | Action execution |
| `internal/config` | Keymap loading, user settings |
| `internal/event` | Event publishing for plugins |

---

## References

- [Vim Documentation](https://vimdoc.sourceforge.net/)
- [Neovim API](https://neovim.io/doc/user/api.html)
- [VS Code Keybindings](https://code.visualstudio.com/docs/getstarted/keybindings)
- [Kakoune Modal Editing](https://kakoune.org/why-kakoune/why-kakoune.html)
- [fzf Fuzzy Matching](https://github.com/junegunn/fzf)
