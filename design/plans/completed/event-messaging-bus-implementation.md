# Keystorm Event & Messaging Bus - Implementation Plan

## Comprehensive Design Document for `internal/event`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Requirements Analysis](#2-requirements-analysis)
3. [Architecture Overview](#3-architecture-overview)
4. [Package Structure](#4-package-structure)
5. [Core Types and Interfaces](#5-core-types-and-interfaces)
6. [Event Type System](#6-event-type-system)
7. [Bus Implementation](#7-bus-implementation)
8. [Subscription Management](#8-subscription-management)
9. [Delivery Semantics](#9-delivery-semantics)
10. [Thread Safety and Concurrency](#10-thread-safety-and-concurrency)
11. [Integration Patterns](#11-integration-patterns)
12. [Implementation Phases](#12-implementation-phases)
13. [Testing Strategy](#13-testing-strategy)
14. [Performance Considerations](#14-performance-considerations)

---

## 1. Executive Summary

The Event & Messaging Bus is Keystorm's "nervous system" - the central communication backbone enabling decoupled, event-driven architecture across all editor components. It facilitates communication between modules (engine, renderer, input, plugins, LSP, integrations) without direct dependencies, enabling a responsive, extensible, and AI-native editor.

### Role in the Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Event Bus                                        │
│                         (internal/event)                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│    Publishers                          Subscribers                            │
│    ─────────                          ───────────                            │
│    ┌─────────┐                        ┌─────────┐                            │
│    │ Engine  │──────┐    ┌────────────│Renderer │                            │
│    └─────────┘      │    │            └─────────┘                            │
│    ┌─────────┐      │    │            ┌─────────┐                            │
│    │  Input  │──────┼────┼────────────│  Plugin │                            │
│    └─────────┘      │    │            └─────────┘                            │
│    ┌─────────┐      ▼    ▼            ┌─────────┐                            │
│    │ Config  │───▶[  BUS  ]◀──────────│   LSP   │                            │
│    └─────────┘      ▲    ▲            └─────────┘                            │
│    ┌─────────┐      │    │            ┌─────────┐                            │
│    │ Project │──────┼────┼────────────│Dispatch │                            │
│    └─────────┘      │    │            └─────────┘                            │
│    ┌─────────┐      │    │            ┌─────────┐                            │
│    │Integrat.│──────┘    └────────────│   AI    │                            │
│    └─────────┘                        └─────────┘                            │
│                                                                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Hierarchical event types | Dot-notation (`buffer.content.inserted`) enables wildcard subscriptions |
| Generic typed events | Go generics ensure type safety without `interface{}` chaos |
| Sync + async delivery | Critical paths (cursor, buffer) sync; non-critical (metrics) async |
| Copy-on-read handlers | Prevents subscription changes during dispatch from causing races |
| Bounded async queues | Prevents slow subscribers from blocking publishers |
| Panic recovery | Handler panics isolated; don't crash the editor |
| Priority subscriptions | Critical handlers (renderer) execute before plugins |
| Context propagation | Cancellation and deadlines flow through event chain |

### Integration Points

The Event Bus connects to:
- **Engine**: Buffer changes, cursor movements, undo/redo state
- **Renderer**: Redraw requests, viewport changes, highlight updates
- **Input**: Keystroke events, mode changes, macro state
- **Dispatcher**: Action execution lifecycle, view update requests
- **Config**: Setting changes, keymap updates, reload events
- **Project**: File operations, workspace events, index changes
- **Plugin**: Plugin lifecycle, custom plugin events
- **LSP**: Server lifecycle, diagnostics, semantic tokens
- **Integration**: Terminal, Git, debugger, task runner events

---

## 2. Requirements Analysis

### 2.1 From Architecture Specification

From `design/specs/architecture.md`:

> "Event & Messaging Bus - The editor's nervous system. Keeps components informed about buffer changes, config changes, window events, extension events, etc."

### 2.2 Functional Requirements

1. **Event Publishing**
   - Publish events with typed payloads
   - Support hierarchical event types (dot-notation)
   - Provide both sync and async publishing modes
   - Include event metadata (timestamp, source, correlation ID)

2. **Event Subscription**
   - Subscribe to specific event types
   - Subscribe with wildcard patterns (`buffer.*`, `*.changed`)
   - Support subscription priorities (critical, normal, low)
   - One-time subscriptions (auto-unsubscribe after first event)
   - Subscribe with filters (predicate functions)

3. **Event Delivery**
   - Synchronous delivery for critical paths
   - Asynchronous delivery with bounded queues
   - Configurable delivery guarantees (at-least-once, at-most-once)
   - Handler timeout support
   - Panic recovery in handlers

4. **Event Types**
   - Strongly typed event payloads per event type
   - Versioned event schemas for backward compatibility
   - Standard metadata (timestamp, source module, correlation ID)

5. **Lifecycle Management**
   - Graceful shutdown with pending event drain
   - Pause/resume event delivery
   - Subscription cleanup on module unload

### 2.3 Event Categories by Module

Based on analysis of existing modules:

| Module | Published Events | Subscribed Events |
|--------|------------------|-------------------|
| Engine | buffer.*, cursor.*, history.* | config.tab.*, lsp.document.* |
| Renderer | renderer.frame.*, renderer.scroll.* | buffer.*, cursor.*, config.renderer.*, lsp.semantic.* |
| Input | input.keystroke, input.mode.*, input.macro.* | config.input.*, dispatcher.action.* |
| Dispatcher | dispatcher.action.* | input.sequence.*, plugin.action.* |
| Config | config.changed, config.section.* | config.watcher.* |
| Project | project.file.*, project.workspace.* | filestore.*, lsp.didOpen |
| Plugin | plugin.loaded, plugin.activated, plugin.* | dispatcher.*, buffer.*, project.* |
| LSP | lsp.server.*, lsp.diagnostics.*, lsp.completion.* | project.file.*, buffer.* |
| Integration | terminal.*, git.*, debug.*, task.* | project.file.*, config.* |

### 2.4 Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Sync event dispatch | < 10μs for single subscriber |
| Async event enqueue | < 1μs |
| Subscription lookup | O(1) for exact match, O(k) for wildcard (k = pattern segments) |
| Memory per subscriber | < 1KB overhead |
| Max concurrent handlers | Configurable, default 100 |
| Handler timeout | Configurable, default 5s for async |
| Event throughput | > 100,000 events/sec |

---

## 3. Architecture Overview

### 3.1 Component Diagram

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              Event Bus System                                  │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                                │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐    │
│  │   Event Types    │  │   Topic Router   │  │    Subscription          │    │
│  │  - Hierarchical  │  │  - Exact match   │  │    Registry              │    │
│  │  - Generic<T>    │  │  - Wildcard      │  │  - Priority ordered      │    │
│  │  - Metadata      │  │  - Pattern trie  │  │  - Thread-safe           │    │
│  └──────────────────┘  └──────────────────┘  └──────────────────────────┘    │
│                                                                                │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐    │
│  │   Sync           │  │   Async          │  │    Handler               │    │
│  │   Dispatcher     │  │   Dispatcher     │  │    Executor              │    │
│  │  - Blocking      │  │  - Queue-based   │  │  - Timeout               │    │
│  │  - Sequential    │  │  - Worker pool   │  │  - Panic recovery        │    │
│  │  - Same goroutine│  │  - Bounded       │  │  - Context               │    │
│  └──────────────────┘  └──────────────────┘  └──────────────────────────┘    │
│                                                                                │
│  ┌──────────────────────────────────────────────────────────────────────┐    │
│  │                        Event Categories                               │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │  buffer  │ │  cursor  │ │  input   │ │  config  │ │ project  │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │  plugin  │ │   lsp    │ │ terminal │ │   git    │ │  debug   │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  └──────────────────────────────────────────────────────────────────────┘    │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Event Flow

```
Publisher                     Event Bus                    Subscribers
─────────                     ─────────                    ───────────

┌─────────┐                                               ┌─────────┐
│ Engine  │                                               │Renderer │
└────┬────┘                                               └────▲────┘
     │                                                         │
     │ Publish(buffer.content.inserted, {...})                 │
     │                                                         │
     ▼                                                         │
┌─────────────────────────────────────────────────────────────┐│
│                       Event Bus                             ││
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     ││
│  │ Create Event│───▶│ Route Topic │───▶│Find Handlers│     ││
│  │ - Add meta  │    │ - Exact     │    │ - Wildcards │     ││
│  │ - Timestamp │    │ - Wildcard  │    │ - Priority  │     ││
│  └─────────────┘    └─────────────┘    └──────┬──────┘     ││
│                                               │             ││
│  ┌────────────────────────────────────────────▼──────────┐ ││
│  │                    Dispatch                            │ ││
│  │  ┌──────────────┐          ┌──────────────┐           │ ││
│  │  │ Sync Path    │          │ Async Path   │           │ ││
│  │  │ (blocking)   │          │ (enqueue)    │           │ ││
│  │  └───────┬──────┘          └──────┬───────┘           │ ││
│  └──────────│────────────────────────│───────────────────┘ ││
│             │                        │                      ││
└─────────────│────────────────────────│──────────────────────┘│
              │                        │                       │
              ▼                        ▼                       │
         ┌─────────┐            ┌─────────────┐               │
         │ Execute │            │ Worker Pool │───────────────┘
         │ Handler │            │ (async)     │
         └─────────┘            └─────────────┘
```

### 3.3 Subscription Matching

```
Event: "buffer.content.inserted"

Matching subscriptions:
  ✓ "buffer.content.inserted"   (exact match)
  ✓ "buffer.content.*"          (wildcard segment)
  ✓ "buffer.*"                  (wildcard subtree)
  ✓ "*"                         (global wildcard)
  ✗ "buffer.content.deleted"    (different event)
  ✗ "cursor.*"                  (different category)

Priority ordering:
  1. Priority.Critical (renderer, engine internal)
  2. Priority.High (dispatcher, LSP)
  3. Priority.Normal (plugins, integrations)
  4. Priority.Low (metrics, logging)
```

---

## 4. Package Structure

```
internal/event/
├── doc.go                    # Package documentation
│
├── # Core types
├── event.go                  # Event type and metadata
├── event_test.go
├── types.go                  # Type definitions (Priority, DeliveryMode, etc.)
├── errors.go                 # Error types
│
├── # Bus implementation
├── bus.go                    # Main Bus type and interface
├── bus_test.go
├── options.go                # Bus configuration options
│
├── # Subscription management
├── subscription.go           # Subscription type and interface
├── subscription_test.go
├── registry.go               # Subscription registry with priority ordering
├── registry_test.go
│
├── # Topic routing
├── topic/
│   ├── topic.go              # Topic type (hierarchical)
│   ├── topic_test.go
│   ├── matcher.go            # Wildcard pattern matching
│   ├── matcher_test.go
│   ├── trie.go               # Trie for efficient pattern lookup
│   └── trie_test.go
│
├── # Dispatching
├── dispatch/
│   ├── dispatcher.go         # Dispatcher interface
│   ├── sync.go               # Synchronous dispatcher
│   ├── sync_test.go
│   ├── async.go              # Asynchronous dispatcher with worker pool
│   ├── async_test.go
│   ├── executor.go           # Handler execution with timeout/recovery
│   └── executor_test.go
│
├── # Typed events (generated or hand-written)
├── events/
│   ├── buffer.go             # Buffer event types
│   ├── cursor.go             # Cursor event types
│   ├── input.go              # Input event types
│   ├── dispatcher.go         # Dispatcher event types
│   ├── config.go             # Config event types
│   ├── project.go            # Project/file event types
│   ├── plugin.go             # Plugin event types
│   ├── lsp.go                # LSP event types
│   ├── terminal.go           # Terminal event types
│   ├── git.go                # Git event types
│   ├── debug.go              # Debugger event types
│   ├── task.go               # Task runner event types
│   └── renderer.go           # Renderer event types
│
├── # Utilities
├── filter.go                 # Event filter predicates
├── filter_test.go
├── metrics.go                # Event bus metrics/observability
└── metrics_test.go
```

### Rationale

- **Separation of concerns**: Topic routing, dispatching, and subscription management are independent
- **Typed events package**: Pre-defined event types ensure consistency across modules
- **Dual dispatchers**: Sync for critical path, async for non-blocking operations
- **Trie-based routing**: Efficient wildcard pattern matching at scale

---

## 5. Core Types and Interfaces

### 5.1 Event Interface and Types

```go
// internal/event/event.go

// Event represents an event in the system.
// Events are immutable once created.
type Event[T any] struct {
    // Type is the hierarchical event type (e.g., "buffer.content.inserted").
    Type Topic

    // Payload contains the event-specific data.
    Payload T

    // Metadata contains standard event information.
    Metadata Metadata
}

// Metadata contains standard information attached to every event.
type Metadata struct {
    // ID is a unique identifier for this event instance.
    ID string

    // Timestamp is when the event was created.
    Timestamp time.Time

    // Source identifies the module that published the event.
    Source string

    // CorrelationID links related events (e.g., request/response).
    CorrelationID string

    // CausationID links to the event that caused this one.
    CausationID string

    // Version is the schema version of the payload.
    Version int
}

// NewEvent creates a new event with the given type and payload.
func NewEvent[T any](eventType Topic, payload T, source string) Event[T] {
    return Event[T]{
        Type:    eventType,
        Payload: payload,
        Metadata: Metadata{
            ID:        generateID(),
            Timestamp: time.Now(),
            Source:    source,
            Version:   1,
        },
    }
}

// WithCorrelation returns a copy of the event with a correlation ID set.
func (e Event[T]) WithCorrelation(correlationID string) Event[T] {
    e.Metadata.CorrelationID = correlationID
    return e
}

// WithCausation returns a copy of the event with a causation ID set.
func (e Event[T]) WithCausation(causationID string) Event[T] {
    e.Metadata.CausationID = causationID
    return e
}
```

### 5.2 Topic Type

```go
// internal/event/topic/topic.go

// Topic represents a hierarchical event type using dot notation.
// Examples: "buffer.content.inserted", "config.changed", "plugin.vim-surround.activated"
type Topic string

// String returns the topic as a string.
func (t Topic) String() string {
    return string(t)
}

// Segments returns the topic split by dots.
func (t Topic) Segments() []string {
    return strings.Split(string(t), ".")
}

// Parent returns the parent topic.
// "buffer.content.inserted" -> "buffer.content"
func (t Topic) Parent() Topic {
    segments := t.Segments()
    if len(segments) <= 1 {
        return ""
    }
    return Topic(strings.Join(segments[:len(segments)-1], "."))
}

// Child returns a child topic with the given segment.
// "buffer".Child("content") -> "buffer.content"
func (t Topic) Child(segment string) Topic {
    if t == "" {
        return Topic(segment)
    }
    return Topic(string(t) + "." + segment)
}

// IsWildcard returns true if the topic contains wildcards.
func (t Topic) IsWildcard() bool {
    return strings.Contains(string(t), "*")
}

// Matches returns true if this topic matches the pattern.
// Patterns support:
//   - "*" matches any single segment
//   - "**" matches zero or more segments
func (t Topic) Matches(pattern Topic) bool {
    return matchPattern(t.Segments(), pattern.Segments())
}

// Common topic constants
const (
    WildcardSingle = "*"  // Matches any single segment
    WildcardMulti  = "**" // Matches zero or more segments
)
```

### 5.3 Bus Interface

```go
// internal/event/bus.go

// Bus is the central event bus interface.
type Bus interface {
    // Publishing
    Publish(ctx context.Context, event any) error
    PublishSync(ctx context.Context, event any) error
    PublishAsync(ctx context.Context, event any) error

    // Subscription
    Subscribe(topic Topic, handler Handler, opts ...SubscribeOption) (Subscription, error)
    SubscribeFunc(topic Topic, fn HandlerFunc, opts ...SubscribeOption) (Subscription, error)
    Unsubscribe(sub Subscription) error

    // Lifecycle
    Start() error
    Stop(ctx context.Context) error
    Pause()
    Resume()

    // Status
    Stats() Stats
    IsRunning() bool
}

// Handler is the interface for event handlers.
type Handler interface {
    // Handle processes an event.
    // The event parameter is type-erased; handlers should type-assert.
    Handle(ctx context.Context, event any) error
}

// HandlerFunc is a function adapter for Handler.
type HandlerFunc func(ctx context.Context, event any) error

func (f HandlerFunc) Handle(ctx context.Context, event any) error {
    return f(ctx, event)
}

// TypedHandler provides type-safe event handling using generics.
type TypedHandler[T any] interface {
    Handle(ctx context.Context, event Event[T]) error
}

// TypedHandlerFunc is a function adapter for TypedHandler.
type TypedHandlerFunc[T any] func(ctx context.Context, event Event[T]) error

func (f TypedHandlerFunc[T]) Handle(ctx context.Context, event Event[T]) error {
    return f(ctx, event)
}

// AsHandler converts a TypedHandler to a generic Handler.
func AsHandler[T any](h TypedHandler[T]) Handler {
    return HandlerFunc(func(ctx context.Context, event any) error {
        if e, ok := event.(Event[T]); ok {
            return h.Handle(ctx, e)
        }
        // Type mismatch - skip silently or return error based on config
        return nil
    })
}

// Stats contains event bus statistics.
type Stats struct {
    EventsPublished   uint64
    EventsDelivered   uint64
    EventsDropped     uint64
    HandlersExecuted  uint64
    HandlerErrors     uint64
    HandlerPanics     uint64
    AvgDeliveryTimeNs int64
    ActiveSubscribers int
    QueueDepth        int
}
```

### 5.4 Subscription Interface

```go
// internal/event/subscription.go

// Subscription represents an active subscription to events.
type Subscription interface {
    // ID returns the unique subscription identifier.
    ID() string

    // Topic returns the subscribed topic pattern.
    Topic() Topic

    // IsActive returns true if the subscription is active.
    IsActive() bool

    // Pause temporarily stops event delivery.
    Pause()

    // Resume restarts event delivery after pause.
    Resume()

    // Cancel permanently cancels the subscription.
    Cancel()
}

// SubscribeOption configures a subscription.
type SubscribeOption func(*subscriptionConfig)

type subscriptionConfig struct {
    priority     Priority
    deliveryMode DeliveryMode
    filter       FilterFunc
    once         bool
    timeout      time.Duration
    name         string // For debugging/metrics
}

// WithPriority sets the handler priority.
func WithPriority(p Priority) SubscribeOption {
    return func(c *subscriptionConfig) {
        c.priority = p
    }
}

// WithDeliveryMode sets sync or async delivery.
func WithDeliveryMode(m DeliveryMode) SubscribeOption {
    return func(c *subscriptionConfig) {
        c.deliveryMode = m
    }
}

// WithFilter adds a predicate filter.
func WithFilter(f FilterFunc) SubscribeOption {
    return func(c *subscriptionConfig) {
        c.filter = f
    }
}

// Once creates a one-time subscription.
func Once() SubscribeOption {
    return func(c *subscriptionConfig) {
        c.once = true
    }
}

// WithTimeout sets the handler execution timeout.
func WithTimeout(d time.Duration) SubscribeOption {
    return func(c *subscriptionConfig) {
        c.timeout = d
    }
}

// WithName sets a name for debugging.
func WithName(name string) SubscribeOption {
    return func(c *subscriptionConfig) {
        c.name = name
    }
}
```

### 5.5 Types and Constants

```go
// internal/event/types.go

// Priority determines handler execution order.
type Priority int

const (
    PriorityCritical Priority = 0   // Renderer, core engine (first)
    PriorityHigh     Priority = 100 // LSP, dispatcher
    PriorityNormal   Priority = 200 // Plugins, integrations (default)
    PriorityLow      Priority = 300 // Metrics, logging (last)
)

// DeliveryMode specifies how events are delivered to handlers.
type DeliveryMode int

const (
    // DeliverySync executes the handler synchronously in the publisher's goroutine.
    // Use for critical paths where latency matters (buffer changes, cursor moves).
    DeliverySync DeliveryMode = iota

    // DeliveryAsync queues the event for asynchronous delivery.
    // Use for non-critical handlers (metrics, plugins, integrations).
    DeliveryAsync
)

// FilterFunc is a predicate for filtering events.
type FilterFunc func(event any) bool
```

---

## 6. Event Type System

### 6.1 Buffer Events

```go
// internal/event/events/buffer.go

// BufferContentInserted is emitted when text is inserted into a buffer.
type BufferContentInserted struct {
    BufferID  string
    Position  Position
    Text      string
    NewRange  Range
    RevisionID string
}

// BufferContentDeleted is emitted when text is deleted from a buffer.
type BufferContentDeleted struct {
    BufferID   string
    Range      Range
    DeletedText string
    RevisionID  string
}

// BufferContentReplaced is emitted when text is replaced in a buffer.
type BufferContentReplaced struct {
    BufferID    string
    OldRange    Range
    NewRange    Range
    OldText     string
    NewText     string
    RevisionID  string
}

// BufferRevisionChanged is emitted when a new revision is created.
type BufferRevisionChanged struct {
    BufferID      string
    RevisionID    string
    PreviousID    string
    ChangeCount   int
}

// BufferSnapshotCreated is emitted when a named snapshot is created.
type BufferSnapshotCreated struct {
    BufferID   string
    SnapshotID string
    Name       string
    RevisionID string
}

// BufferCleared is emitted when a buffer is cleared.
type BufferCleared struct {
    BufferID string
}

// BufferReadOnlyChanged is emitted when read-only state changes.
type BufferReadOnlyChanged struct {
    BufferID   string
    IsReadOnly bool
}

// Topics for buffer events
const (
    TopicBufferContentInserted  Topic = "buffer.content.inserted"
    TopicBufferContentDeleted   Topic = "buffer.content.deleted"
    TopicBufferContentReplaced  Topic = "buffer.content.replaced"
    TopicBufferRevisionChanged  Topic = "buffer.revision.changed"
    TopicBufferSnapshotCreated  Topic = "buffer.snapshot.created"
    TopicBufferCleared          Topic = "buffer.cleared"
    TopicBufferReadOnlyChanged  Topic = "buffer.readonly.changed"
)

// Position represents a position in a buffer.
type Position struct {
    Line   int
    Column int
    Offset int
}

// Range represents a range in a buffer.
type Range struct {
    Start Position
    End   Position
}
```

### 6.2 Cursor Events

```go
// internal/event/events/cursor.go

// CursorMoved is emitted when the primary cursor moves.
type CursorMoved struct {
    BufferID  string
    Position  Position
    Selection *Selection // nil if no selection
}

// CursorAdded is emitted when a secondary cursor is added.
type CursorAdded struct {
    BufferID  string
    CursorID  string
    Position  Position
    Selection *Selection
}

// CursorRemoved is emitted when a secondary cursor is removed.
type CursorRemoved struct {
    BufferID string
    CursorID string
}

// CursorSelectionChanged is emitted when selection changes.
type CursorSelectionChanged struct {
    BufferID  string
    Selection Selection
}

// Selection represents a text selection.
type Selection struct {
    Anchor Position
    Head   Position
}

// Topics for cursor events
const (
    TopicCursorMoved             Topic = "cursor.moved"
    TopicCursorAdded             Topic = "cursor.added"
    TopicCursorRemoved           Topic = "cursor.removed"
    TopicCursorSelectionChanged  Topic = "cursor.selection.changed"
)
```

### 6.3 Input Events

```go
// internal/event/events/input.go

// InputKeystroke is emitted when a key is pressed.
type InputKeystroke struct {
    Key       string
    Modifiers []string // ctrl, shift, alt, meta
    Timestamp time.Time
    Mode      string
}

// InputSequenceResolved is emitted when a key sequence matches an action.
type InputSequenceResolved struct {
    Sequence string
    Action   string
    Mode     string
}

// InputSequencePending is emitted when waiting for more keys.
type InputSequencePending struct {
    PendingSequence string
    Mode            string
}

// InputModeChanged is emitted when the editor mode changes.
type InputModeChanged struct {
    PreviousMode string
    CurrentMode  string
}

// InputMacroStarted is emitted when macro recording starts.
type InputMacroStarted struct {
    Register string
}

// InputMacroStopped is emitted when macro recording stops.
type InputMacroStopped struct {
    Register    string
    KeysRecorded int
}

// Topics for input events
const (
    TopicInputKeystroke        Topic = "input.keystroke"
    TopicInputSequenceResolved Topic = "input.sequence.resolved"
    TopicInputSequencePending  Topic = "input.sequence.pending"
    TopicInputModeChanged      Topic = "input.mode.changed"
    TopicInputMacroStarted     Topic = "input.macro.started"
    TopicInputMacroStopped     Topic = "input.macro.stopped"
)
```

### 6.4 Config Events

```go
// internal/event/events/config.go

// ConfigChanged is emitted when a setting changes.
type ConfigChanged struct {
    Path     string      // e.g., "editor.tabSize"
    OldValue any
    NewValue any
    Source   string      // "user", "project", "default"
}

// ConfigSectionReloaded is emitted when a config section is reloaded.
type ConfigSectionReloaded struct {
    Section string
    Source  string
}

// ConfigKeymapUpdated is emitted when keymaps change.
type ConfigKeymapUpdated struct {
    Mode    string
    Changes []KeymapChange
}

// KeymapChange represents a single keymap modification.
type KeymapChange struct {
    Keys   string
    Action string
    Added  bool // true if added, false if removed
}

// Topics for config events
const (
    TopicConfigChanged         Topic = "config.changed"
    TopicConfigSectionReloaded Topic = "config.section.reloaded"
    TopicConfigKeymapUpdated   Topic = "config.keymap.updated"
)
```

### 6.5 Project Events

```go
// internal/event/events/project.go

// ProjectFileOpened is emitted when a file is opened.
type ProjectFileOpened struct {
    Path       string
    LanguageID string
    Encoding   string
    LineEnding string
}

// ProjectFileClosed is emitted when a file is closed.
type ProjectFileClosed struct {
    Path string
}

// ProjectFileSaved is emitted when a file is saved.
type ProjectFileSaved struct {
    Path        string
    DiskModTime time.Time
}

// ProjectFileChanged is emitted when a file changes externally.
type ProjectFileChanged struct {
    Path   string
    Action FileChangeAction // Created, Modified, Deleted, Renamed
}

// FileChangeAction represents the type of file change.
type FileChangeAction string

const (
    FileCreated  FileChangeAction = "created"
    FileModified FileChangeAction = "modified"
    FileDeleted  FileChangeAction = "deleted"
    FileRenamed  FileChangeAction = "renamed"
)

// ProjectFileDirtyChanged is emitted when dirty state changes.
type ProjectFileDirtyChanged struct {
    Path    string
    IsDirty bool
}

// ProjectWorkspaceOpened is emitted when workspace is initialized.
type ProjectWorkspaceOpened struct {
    Roots []string
}

// ProjectWorkspaceClosed is emitted when workspace is shut down.
type ProjectWorkspaceClosed struct{}

// ProjectIndexChanged is emitted when the project index updates.
type ProjectIndexChanged struct {
    ChangeType FileChangeAction
    Paths      []string
}

// Topics for project events
const (
    TopicProjectFileOpened       Topic = "project.file.opened"
    TopicProjectFileClosed       Topic = "project.file.closed"
    TopicProjectFileSaved        Topic = "project.file.saved"
    TopicProjectFileChanged      Topic = "project.file.changed"
    TopicProjectFileDirtyChanged Topic = "project.file.dirty.changed"
    TopicProjectWorkspaceOpened  Topic = "project.workspace.opened"
    TopicProjectWorkspaceClosed  Topic = "project.workspace.closed"
    TopicProjectIndexChanged     Topic = "project.index.changed"
)
```

### 6.6 Plugin Events

```go
// internal/event/events/plugin.go

// PluginLoaded is emitted when a plugin is loaded from disk.
type PluginLoaded struct {
    PluginName string
    Path       string
    Version    string
}

// PluginUnloaded is emitted when a plugin is unloaded.
type PluginUnloaded struct {
    PluginName string
}

// PluginActivated is emitted when a plugin is activated.
type PluginActivated struct {
    PluginName   string
    Capabilities []string
}

// PluginDeactivated is emitted when a plugin is deactivated.
type PluginDeactivated struct {
    PluginName string
    Reason     string
}

// PluginReloaded is emitted when a plugin is hot-reloaded.
type PluginReloaded struct {
    PluginName string
    Changes    []string
}

// PluginError is emitted when a plugin encounters an error.
type PluginError struct {
    PluginName   string
    ErrorMessage string
    Severity     string // "warning", "error", "fatal"
}

// PluginActionRegistered is emitted when a plugin registers an action.
type PluginActionRegistered struct {
    PluginName string
    ActionName string
}

// Topics for plugin events
const (
    TopicPluginLoaded           Topic = "plugin.loaded"
    TopicPluginUnloaded         Topic = "plugin.unloaded"
    TopicPluginActivated        Topic = "plugin.activated"
    TopicPluginDeactivated      Topic = "plugin.deactivated"
    TopicPluginReloaded         Topic = "plugin.reloaded"
    TopicPluginError            Topic = "plugin.error"
    TopicPluginActionRegistered Topic = "plugin.action.registered"
)
```

### 6.7 LSP Events

```go
// internal/event/events/lsp.go

// LSPServerInitialized is emitted when an LSP server is ready.
type LSPServerInitialized struct {
    LanguageID   string
    Capabilities map[string]any
}

// LSPServerShutdown is emitted when an LSP server closes.
type LSPServerShutdown struct {
    LanguageID string
}

// LSPDiagnosticsPublished is emitted when diagnostics are received.
type LSPDiagnosticsPublished struct {
    URI         string
    Diagnostics []Diagnostic
}

// Diagnostic represents an LSP diagnostic.
type Diagnostic struct {
    Range    Range
    Severity int    // 1=Error, 2=Warning, 3=Info, 4=Hint
    Code     string
    Source   string
    Message  string
}

// LSPCompletionAvailable is emitted when completions are ready.
type LSPCompletionAvailable struct {
    URI      string
    Position Position
    Items    []CompletionItem
}

// CompletionItem represents a completion suggestion.
type CompletionItem struct {
    Label      string
    Kind       int
    Detail     string
    InsertText string
}

// LSPSemanticTokensUpdated is emitted when semantic tokens are available.
type LSPSemanticTokensUpdated struct {
    URI    string
    Tokens []SemanticToken
}

// SemanticToken represents a semantic token.
type SemanticToken struct {
    Line      int
    StartChar int
    Length    int
    TokenType int
    Modifiers int
}

// LSPServerError is emitted on LSP server errors.
type LSPServerError struct {
    LanguageID   string
    ErrorMessage string
}

// Topics for LSP events
const (
    TopicLSPServerInitialized      Topic = "lsp.server.initialized"
    TopicLSPServerShutdown         Topic = "lsp.server.shutdown"
    TopicLSPDiagnosticsPublished   Topic = "lsp.diagnostics.published"
    TopicLSPCompletionAvailable    Topic = "lsp.completion.available"
    TopicLSPSemanticTokensUpdated  Topic = "lsp.semanticTokens.updated"
    TopicLSPServerError            Topic = "lsp.server.error"
)
```

### 6.8 Integration Events

```go
// internal/event/events/terminal.go

// TerminalCreated is emitted when a terminal session starts.
type TerminalCreated struct {
    TerminalID string
    Shell      string
    Cwd        string
}

// TerminalClosed is emitted when a terminal session ends.
type TerminalClosed struct {
    TerminalID string
}

// TerminalOutput is emitted when terminal output is available.
type TerminalOutput struct {
    TerminalID string
    Output     string
    Timestamp  time.Time
}

// TerminalExited is emitted when terminal process exits.
type TerminalExited struct {
    TerminalID string
    ExitCode   int
}

// Topics for terminal events
const (
    TopicTerminalCreated Topic = "terminal.created"
    TopicTerminalClosed  Topic = "terminal.closed"
    TopicTerminalOutput  Topic = "terminal.output"
    TopicTerminalExited  Topic = "terminal.exited"
)

// internal/event/events/git.go

// GitStatusChanged is emitted when repository status changes.
type GitStatusChanged struct {
    Root      string
    Staged    []string
    Unstaged  []string
    Untracked []string
}

// GitBranchChanged is emitted when the current branch changes.
type GitBranchChanged struct {
    Root      string
    OldBranch string
    NewBranch string
}

// GitCommitCreated is emitted when a commit is made.
type GitCommitCreated struct {
    Root       string
    CommitHash string
    Message    string
}

// GitConflictDetected is emitted when merge conflicts are found.
type GitConflictDetected struct {
    Root  string
    Files []string
}

// Topics for git events
const (
    TopicGitStatusChanged    Topic = "git.status.changed"
    TopicGitBranchChanged    Topic = "git.branch.changed"
    TopicGitCommitCreated    Topic = "git.commit.created"
    TopicGitConflictDetected Topic = "git.conflict.detected"
)

// internal/event/events/debug.go

// DebugSessionStarted is emitted when a debug session starts.
type DebugSessionStarted struct {
    SessionID     string
    Adapter       string
    TargetProcess string
}

// DebugSessionStopped is emitted when a debug session ends.
type DebugSessionStopped struct {
    SessionID string
    ExitCode  int
    Reason    string
}

// DebugBreakpointHit is emitted when a breakpoint is triggered.
type DebugBreakpointHit struct {
    SessionID string
    File      string
    Line      int
    Reason    string
}

// DebugBreakpointAdded is emitted when a breakpoint is set.
type DebugBreakpointAdded struct {
    SessionID string
    File      string
    Line      int
}

// DebugBreakpointRemoved is emitted when a breakpoint is cleared.
type DebugBreakpointRemoved struct {
    SessionID string
    File      string
    Line      int
}

// DebugStepCompleted is emitted when a step operation finishes.
type DebugStepCompleted struct {
    SessionID string
    File      string
    Line      int
    Reason    string
}

// Topics for debug events
const (
    TopicDebugSessionStarted    Topic = "debug.session.started"
    TopicDebugSessionStopped    Topic = "debug.session.stopped"
    TopicDebugBreakpointHit     Topic = "debug.breakpoint.hit"
    TopicDebugBreakpointAdded   Topic = "debug.breakpoint.added"
    TopicDebugBreakpointRemoved Topic = "debug.breakpoint.removed"
    TopicDebugStepCompleted     Topic = "debug.step.completed"
)

// internal/event/events/task.go

// TaskDiscovered is emitted when tasks are scanned.
type TaskDiscovered struct {
    TaskCount int
    Sources   []string // make, package.json, Taskfile, etc.
}

// TaskStarted is emitted when task execution begins.
type TaskStarted struct {
    TaskID    string
    TaskName  string
    StartTime time.Time
}

// TaskOutput is emitted when task produces output.
type TaskOutput struct {
    TaskID    string
    Output    string
    Timestamp time.Time
}

// TaskCompleted is emitted when task execution finishes.
type TaskCompleted struct {
    TaskID   string
    ExitCode int
    Duration time.Duration
}

// TaskProblemFound is emitted when a problem matcher detects an issue.
type TaskProblemFound struct {
    TaskID   string
    File     string
    Line     int
    Message  string
    Severity string // "error", "warning", "info"
}

// TaskCancelled is emitted when a task is cancelled.
type TaskCancelled struct {
    TaskID string
    Reason string
}

// Topics for task events
const (
    TopicTaskDiscovered   Topic = "task.discovered"
    TopicTaskStarted      Topic = "task.started"
    TopicTaskOutput       Topic = "task.output"
    TopicTaskCompleted    Topic = "task.completed"
    TopicTaskProblemFound Topic = "task.problem.found"
    TopicTaskCancelled    Topic = "task.cancelled"
)
```

### 6.9 Dispatcher Events

```go
// internal/event/events/dispatcher.go

// DispatcherActionDispatched is emitted when an action is sent to handler.
type DispatcherActionDispatched struct {
    ActionName string
    Count      int
    Args       map[string]any
}

// DispatcherActionExecuted is emitted when handler completes.
type DispatcherActionExecuted struct {
    ActionName string
    Duration   time.Duration
    Status     string // "success", "error", "cancelled"
}

// DispatcherActionFailed is emitted when handler raises error.
type DispatcherActionFailed struct {
    ActionName   string
    ErrorMessage string
}

// DispatcherModeChanged is emitted when mode is switched by handler.
type DispatcherModeChanged struct {
    NewMode      string
    PreviousMode string
}

// DispatcherViewUpdateRequested is emitted when view needs update.
type DispatcherViewUpdateRequested struct {
    Redraw     bool
    ScrollTo   *Position
    CenterLine *int
}

// Topics for dispatcher events
const (
    TopicDispatcherActionDispatched      Topic = "dispatcher.action.dispatched"
    TopicDispatcherActionExecuted        Topic = "dispatcher.action.executed"
    TopicDispatcherActionFailed          Topic = "dispatcher.action.failed"
    TopicDispatcherModeChanged           Topic = "dispatcher.mode.changed"
    TopicDispatcherViewUpdateRequested   Topic = "dispatcher.view.update.requested"
)
```

### 6.10 Renderer Events

```go
// internal/event/events/renderer.go

// RendererFrameRendered is emitted after a frame is rendered.
type RendererFrameRendered struct {
    FrameCount int
    FPS        float64
    DeltaMs    float64
}

// RendererRedrawNeeded is emitted when display needs update.
type RendererRedrawNeeded struct {
    FullRedraw bool
    LineRanges []LineRange
}

// LineRange represents a range of lines.
type LineRange struct {
    Start int
    End   int
}

// RendererResizeHandled is emitted when window is resized.
type RendererResizeHandled struct {
    Width       int
    Height      int
    GutterWidth int
}

// RendererScrollChanged is emitted when viewport scrolls.
type RendererScrollChanged struct {
    TopLine    int
    LeftColumn int
    Smooth     bool
}

// RendererHighlightInvalidated is emitted when syntax highlighting is stale.
type RendererHighlightInvalidated struct {
    LineRange LineRange
}

// Topics for renderer events
const (
    TopicRendererFrameRendered        Topic = "renderer.frame.rendered"
    TopicRendererRedrawNeeded         Topic = "renderer.redraw.needed"
    TopicRendererResizeHandled        Topic = "renderer.resize.handled"
    TopicRendererScrollChanged        Topic = "renderer.scroll.changed"
    TopicRendererHighlightInvalidated Topic = "renderer.highlight.invalidated"
)
```

---

## 7. Bus Implementation

### 7.1 Default Bus Implementation

```go
// internal/event/bus.go

// bus is the default Bus implementation.
type bus struct {
    mu sync.RWMutex

    // Subscription management
    registry *Registry

    // Dispatchers
    syncDispatcher  *dispatch.SyncDispatcher
    asyncDispatcher *dispatch.AsyncDispatcher

    // Topic routing
    topicMatcher *topic.Matcher

    // State
    running atomic.Bool
    paused  atomic.Bool

    // Configuration
    opts busOptions

    // Metrics
    stats Stats
}

type busOptions struct {
    asyncQueueSize    int
    asyncWorkerCount  int
    defaultTimeout    time.Duration
    panicHandler      func(event any, handler Handler, err any)
    metricsEnabled    bool
}

// DefaultBusOptions returns sensible defaults.
func DefaultBusOptions() busOptions {
    return busOptions{
        asyncQueueSize:   10000,
        asyncWorkerCount: 10,
        defaultTimeout:   5 * time.Second,
        panicHandler:     defaultPanicHandler,
        metricsEnabled:   true,
    }
}

// NewBus creates a new event bus.
func NewBus(opts ...BusOption) *bus {
    options := DefaultBusOptions()
    for _, opt := range opts {
        opt(&options)
    }

    b := &bus{
        registry:     NewRegistry(),
        topicMatcher: topic.NewMatcher(),
        opts:         options,
    }

    b.syncDispatcher = dispatch.NewSyncDispatcher(
        dispatch.WithPanicHandler(options.panicHandler),
    )

    b.asyncDispatcher = dispatch.NewAsyncDispatcher(
        dispatch.WithQueueSize(options.asyncQueueSize),
        dispatch.WithWorkerCount(options.asyncWorkerCount),
        dispatch.WithTimeout(options.defaultTimeout),
        dispatch.WithPanicHandler(options.panicHandler),
    )

    return b
}

// Start starts the event bus.
func (b *bus) Start() error {
    if b.running.Swap(true) {
        return ErrAlreadyRunning
    }
    return b.asyncDispatcher.Start()
}

// Stop stops the event bus gracefully.
func (b *bus) Stop(ctx context.Context) error {
    if !b.running.Swap(false) {
        return ErrNotRunning
    }
    return b.asyncDispatcher.Stop(ctx)
}

// Pause temporarily stops event delivery.
func (b *bus) Pause() {
    b.paused.Store(true)
}

// Resume restarts event delivery after pause.
func (b *bus) Resume() {
    b.paused.Store(false)
}
```

### 7.2 Publishing

```go
// internal/event/bus.go (continued)

// Publish sends an event using default delivery mode.
// Default is async for non-critical events.
func (b *bus) Publish(ctx context.Context, event any) error {
    return b.PublishAsync(ctx, event)
}

// PublishSync sends an event synchronously.
// The call blocks until all sync handlers complete.
func (b *bus) PublishSync(ctx context.Context, event any) error {
    if !b.running.Load() {
        return ErrNotRunning
    }
    if b.paused.Load() {
        return nil // Silently drop when paused
    }

    topicName := b.extractTopic(event)
    if topicName == "" {
        return ErrInvalidEvent
    }

    // Get matching subscriptions
    subs := b.findSubscriptions(topicName)
    if len(subs) == 0 {
        return nil // No subscribers
    }

    // Update metrics
    atomic.AddUint64(&b.stats.EventsPublished, 1)

    // Dispatch to sync handlers
    for _, sub := range subs {
        if sub.config.deliveryMode != DeliverySync {
            continue
        }
        if sub.config.filter != nil && !sub.config.filter(event) {
            continue
        }

        if err := b.syncDispatcher.Dispatch(ctx, event, sub.handler); err != nil {
            atomic.AddUint64(&b.stats.HandlerErrors, 1)
            // Continue to other handlers
        }
        atomic.AddUint64(&b.stats.HandlersExecuted, 1)

        // Handle one-time subscriptions
        if sub.config.once {
            b.Unsubscribe(sub)
        }
    }

    return nil
}

// PublishAsync queues an event for asynchronous delivery.
func (b *bus) PublishAsync(ctx context.Context, event any) error {
    if !b.running.Load() {
        return ErrNotRunning
    }
    if b.paused.Load() {
        return nil
    }

    topicName := b.extractTopic(event)
    if topicName == "" {
        return ErrInvalidEvent
    }

    subs := b.findSubscriptions(topicName)
    if len(subs) == 0 {
        return nil
    }

    atomic.AddUint64(&b.stats.EventsPublished, 1)

    // Queue for async handlers
    for _, sub := range subs {
        if sub.config.deliveryMode != DeliveryAsync {
            continue
        }
        if sub.config.filter != nil && !sub.config.filter(event) {
            continue
        }

        if err := b.asyncDispatcher.Enqueue(ctx, event, sub); err != nil {
            atomic.AddUint64(&b.stats.EventsDropped, 1)
            // Queue full - event dropped
        }
    }

    return nil
}

// extractTopic extracts the topic from an event.
func (b *bus) extractTopic(event any) Topic {
    // Use reflection to find the Topic field or use a type switch
    switch e := event.(type) {
    case interface{ EventTopic() Topic }:
        return e.EventTopic()
    default:
        // Fallback: use type name as topic
        t := reflect.TypeOf(event)
        if t.Kind() == reflect.Ptr {
            t = t.Elem()
        }
        return Topic(toSnakeCase(t.Name()))
    }
}

// findSubscriptions returns all matching subscriptions, sorted by priority.
func (b *bus) findSubscriptions(topicName Topic) []*subscription {
    b.mu.RLock()
    defer b.mu.RUnlock()

    // Get exact matches and wildcard matches
    matches := b.topicMatcher.Match(topicName)

    // Get subscriptions for each matched pattern
    var subs []*subscription
    for _, pattern := range matches {
        subs = append(subs, b.registry.Get(pattern)...)
    }

    // Sort by priority
    sort.SliceStable(subs, func(i, j int) bool {
        return subs[i].config.priority < subs[j].config.priority
    })

    return subs
}
```

### 7.3 Subscription

```go
// internal/event/bus.go (continued)

// Subscribe creates a new subscription.
func (b *bus) Subscribe(topicPattern Topic, handler Handler, opts ...SubscribeOption) (Subscription, error) {
    config := subscriptionConfig{
        priority:     PriorityNormal,
        deliveryMode: DeliveryAsync, // Default to async
        timeout:      b.opts.defaultTimeout,
    }
    for _, opt := range opts {
        opt(&config)
    }

    sub := &subscription{
        id:      generateID(),
        topic:   topicPattern,
        handler: handler,
        config:  config,
        active:  atomic.Bool{},
    }
    sub.active.Store(true)

    b.mu.Lock()
    defer b.mu.Unlock()

    // Register with topic matcher for pattern matching
    b.topicMatcher.Add(topicPattern)

    // Add to registry
    b.registry.Add(topicPattern, sub)

    return sub, nil
}

// SubscribeFunc is a convenience method for function handlers.
func (b *bus) SubscribeFunc(topicPattern Topic, fn HandlerFunc, opts ...SubscribeOption) (Subscription, error) {
    return b.Subscribe(topicPattern, fn, opts...)
}

// Unsubscribe removes a subscription.
func (b *bus) Unsubscribe(sub Subscription) error {
    s, ok := sub.(*subscription)
    if !ok {
        return ErrInvalidSubscription
    }

    s.active.Store(false)

    b.mu.Lock()
    defer b.mu.Unlock()

    b.registry.Remove(s.topic, s)

    // Clean up topic matcher if no more subscriptions
    if b.registry.Count(s.topic) == 0 {
        b.topicMatcher.Remove(s.topic)
    }

    return nil
}
```

---

## 8. Subscription Management

### 8.1 Registry Implementation

```go
// internal/event/registry.go

// Registry manages subscriptions organized by topic pattern.
type Registry struct {
    mu   sync.RWMutex
    subs map[Topic][]*subscription
}

// NewRegistry creates a new subscription registry.
func NewRegistry() *Registry {
    return &Registry{
        subs: make(map[Topic][]*subscription),
    }
}

// Add adds a subscription for a topic pattern.
func (r *Registry) Add(topicPattern Topic, sub *subscription) {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.subs[topicPattern] = append(r.subs[topicPattern], sub)
}

// Remove removes a subscription.
func (r *Registry) Remove(topicPattern Topic, sub *subscription) {
    r.mu.Lock()
    defer r.mu.Unlock()

    subs := r.subs[topicPattern]
    for i, s := range subs {
        if s.id == sub.id {
            // Remove while preserving order
            r.subs[topicPattern] = append(subs[:i], subs[i+1:]...)
            break
        }
    }
}

// Get returns all subscriptions for a topic pattern.
// Returns a copy to prevent modification during iteration.
func (r *Registry) Get(topicPattern Topic) []*subscription {
    r.mu.RLock()
    defer r.mu.RUnlock()

    subs := r.subs[topicPattern]
    if len(subs) == 0 {
        return nil
    }

    // Return copy to prevent races
    result := make([]*subscription, len(subs))
    copy(result, subs)
    return result
}

// Count returns the number of subscriptions for a topic pattern.
func (r *Registry) Count(topicPattern Topic) int {
    r.mu.RLock()
    defer r.mu.RUnlock()

    return len(r.subs[topicPattern])
}

// All returns all subscriptions.
func (r *Registry) All() []*subscription {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var all []*subscription
    for _, subs := range r.subs {
        all = append(all, subs...)
    }
    return all
}
```

### 8.2 Subscription Type

```go
// internal/event/subscription.go

// subscription is the internal subscription implementation.
type subscription struct {
    id      string
    topic   Topic
    handler Handler
    config  subscriptionConfig
    active  atomic.Bool
    paused  atomic.Bool
}

// ID returns the subscription ID.
func (s *subscription) ID() string {
    return s.id
}

// Topic returns the subscribed topic pattern.
func (s *subscription) Topic() Topic {
    return s.topic
}

// IsActive returns true if the subscription is active.
func (s *subscription) IsActive() bool {
    return s.active.Load()
}

// Pause temporarily stops event delivery.
func (s *subscription) Pause() {
    s.paused.Store(true)
}

// Resume restarts event delivery.
func (s *subscription) Resume() {
    s.paused.Store(false)
}

// Cancel permanently cancels the subscription.
func (s *subscription) Cancel() {
    s.active.Store(false)
}
```

---

## 9. Delivery Semantics

### 9.1 Sync Dispatcher

```go
// internal/event/dispatch/sync.go

// SyncDispatcher executes handlers synchronously.
type SyncDispatcher struct {
    panicHandler func(event any, handler Handler, err any)
}

// NewSyncDispatcher creates a new sync dispatcher.
func NewSyncDispatcher(opts ...DispatcherOption) *SyncDispatcher {
    d := &SyncDispatcher{
        panicHandler: defaultPanicHandler,
    }
    for _, opt := range opts {
        opt(d)
    }
    return d
}

// Dispatch executes a handler synchronously.
func (d *SyncDispatcher) Dispatch(ctx context.Context, event any, handler Handler) (err error) {
    // Panic recovery
    defer func() {
        if r := recover(); r != nil {
            if d.panicHandler != nil {
                d.panicHandler(event, handler, r)
            }
            err = fmt.Errorf("handler panic: %v", r)
        }
    }()

    // Check context
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    return handler.Handle(ctx, event)
}
```

### 9.2 Async Dispatcher

```go
// internal/event/dispatch/async.go

// AsyncDispatcher executes handlers asynchronously using a worker pool.
type AsyncDispatcher struct {
    mu sync.Mutex

    // Configuration
    queueSize   int
    workerCount int
    timeout     time.Duration

    // State
    queue   chan asyncTask
    workers []*worker
    running atomic.Bool

    // Handlers
    panicHandler func(event any, handler Handler, err any)
}

type asyncTask struct {
    ctx     context.Context
    event   any
    sub     *subscription
    enqueue time.Time
}

// NewAsyncDispatcher creates a new async dispatcher.
func NewAsyncDispatcher(opts ...AsyncOption) *AsyncDispatcher {
    d := &AsyncDispatcher{
        queueSize:    10000,
        workerCount:  10,
        timeout:      5 * time.Second,
        panicHandler: defaultPanicHandler,
    }
    for _, opt := range opts {
        opt(d)
    }
    return d
}

// Start starts the worker pool.
func (d *AsyncDispatcher) Start() error {
    d.mu.Lock()
    defer d.mu.Unlock()

    if d.running.Load() {
        return ErrAlreadyRunning
    }

    d.queue = make(chan asyncTask, d.queueSize)
    d.workers = make([]*worker, d.workerCount)

    for i := 0; i < d.workerCount; i++ {
        w := &worker{
            id:           i,
            queue:        d.queue,
            timeout:      d.timeout,
            panicHandler: d.panicHandler,
        }
        d.workers[i] = w
        go w.run()
    }

    d.running.Store(true)
    return nil
}

// Stop stops the worker pool gracefully.
func (d *AsyncDispatcher) Stop(ctx context.Context) error {
    if !d.running.Swap(false) {
        return ErrNotRunning
    }

    close(d.queue)

    // Wait for workers to drain
    done := make(chan struct{})
    go func() {
        for _, w := range d.workers {
            w.wait()
        }
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// Enqueue adds a task to the queue.
func (d *AsyncDispatcher) Enqueue(ctx context.Context, event any, sub *subscription) error {
    if !d.running.Load() {
        return ErrNotRunning
    }

    task := asyncTask{
        ctx:     ctx,
        event:   event,
        sub:     sub,
        enqueue: time.Now(),
    }

    select {
    case d.queue <- task:
        return nil
    default:
        return ErrQueueFull
    }
}

// worker processes tasks from the queue.
type worker struct {
    id           int
    queue        <-chan asyncTask
    timeout      time.Duration
    panicHandler func(event any, handler Handler, err any)
    done         sync.WaitGroup
}

func (w *worker) run() {
    w.done.Add(1)
    defer w.done.Done()

    for task := range w.queue {
        w.execute(task)
    }
}

func (w *worker) execute(task asyncTask) {
    // Panic recovery
    defer func() {
        if r := recover(); r != nil {
            if w.panicHandler != nil {
                w.panicHandler(task.event, task.sub.handler, r)
            }
        }
    }()

    // Check if subscription is still active
    if !task.sub.IsActive() || task.sub.paused.Load() {
        return
    }

    // Create timeout context
    ctx, cancel := context.WithTimeout(task.ctx, w.timeout)
    defer cancel()

    // Execute handler
    _ = task.sub.handler.Handle(ctx, task.event)
}

func (w *worker) wait() {
    w.done.Wait()
}
```

---

## 10. Thread Safety and Concurrency

### 10.1 Topic Matcher with Trie

```go
// internal/event/topic/matcher.go

// Matcher provides efficient topic pattern matching.
type Matcher struct {
    mu   sync.RWMutex
    root *trieNode
}

type trieNode struct {
    children map[string]*trieNode
    patterns []Topic // Patterns that end at this node
}

// NewMatcher creates a new topic matcher.
func NewMatcher() *Matcher {
    return &Matcher{
        root: &trieNode{
            children: make(map[string]*trieNode),
        },
    }
}

// Add adds a pattern to the matcher.
func (m *Matcher) Add(pattern Topic) {
    m.mu.Lock()
    defer m.mu.Unlock()

    segments := pattern.Segments()
    node := m.root

    for _, seg := range segments {
        if node.children[seg] == nil {
            node.children[seg] = &trieNode{
                children: make(map[string]*trieNode),
            }
        }
        node = node.children[seg]
    }

    node.patterns = append(node.patterns, pattern)
}

// Remove removes a pattern from the matcher.
func (m *Matcher) Remove(pattern Topic) {
    m.mu.Lock()
    defer m.mu.Unlock()

    segments := pattern.Segments()
    node := m.root

    for _, seg := range segments {
        if node.children[seg] == nil {
            return
        }
        node = node.children[seg]
    }

    // Remove pattern from leaf
    for i, p := range node.patterns {
        if p == pattern {
            node.patterns = append(node.patterns[:i], node.patterns[i+1:]...)
            break
        }
    }
}

// Match returns all patterns that match the given topic.
func (m *Matcher) Match(eventTopic Topic) []Topic {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var matches []Topic
    segments := eventTopic.Segments()

    m.matchRecursive(m.root, segments, 0, &matches)

    return matches
}

func (m *Matcher) matchRecursive(node *trieNode, segments []string, depth int, matches *[]Topic) {
    if node == nil {
        return
    }

    // If we've consumed all segments, collect patterns at this node
    if depth == len(segments) {
        *matches = append(*matches, node.patterns...)
        // Also check for ** wildcard that matches zero segments
        if child := node.children[WildcardMulti]; child != nil {
            *matches = append(*matches, child.patterns...)
        }
        return
    }

    segment := segments[depth]

    // Exact match
    if child := node.children[segment]; child != nil {
        m.matchRecursive(child, segments, depth+1, matches)
    }

    // Single wildcard (*) matches any one segment
    if child := node.children[WildcardSingle]; child != nil {
        m.matchRecursive(child, segments, depth+1, matches)
    }

    // Multi wildcard (**) matches zero or more segments
    if child := node.children[WildcardMulti]; child != nil {
        // Try matching 0, 1, 2, ... remaining segments
        for i := depth; i <= len(segments); i++ {
            m.matchRecursive(child, segments, i, matches)
        }
    }
}
```

### 10.2 Concurrent Access Patterns

```go
// internal/event/bus.go (thread safety patterns)

// Thread-safe stats access
func (b *bus) Stats() Stats {
    return Stats{
        EventsPublished:   atomic.LoadUint64(&b.stats.EventsPublished),
        EventsDelivered:   atomic.LoadUint64(&b.stats.EventsDelivered),
        EventsDropped:     atomic.LoadUint64(&b.stats.EventsDropped),
        HandlersExecuted:  atomic.LoadUint64(&b.stats.HandlersExecuted),
        HandlerErrors:     atomic.LoadUint64(&b.stats.HandlerErrors),
        HandlerPanics:     atomic.LoadUint64(&b.stats.HandlerPanics),
        AvgDeliveryTimeNs: atomic.LoadInt64(&b.stats.AvgDeliveryTimeNs),
        ActiveSubscribers: b.registry.TotalCount(),
        QueueDepth:        b.asyncDispatcher.QueueDepth(),
    }
}

// Copy-on-read for handler iteration
// (Implemented in findSubscriptions - returns a copy of subscriptions)

// Atomic state management
func (b *bus) IsRunning() bool {
    return b.running.Load()
}
```

---

## 11. Integration Patterns

### 11.1 Publisher Helper

```go
// internal/event/publisher.go

// Publisher is a helper for modules to publish events.
type Publisher struct {
    bus    Bus
    source string
}

// NewPublisher creates a new publisher for a module.
func NewPublisher(bus Bus, source string) *Publisher {
    return &Publisher{
        bus:    bus,
        source: source,
    }
}

// Publish publishes an event asynchronously.
func (p *Publisher) Publish(ctx context.Context, topic Topic, payload any) error {
    event := Event[any]{
        Type:    topic,
        Payload: payload,
        Metadata: Metadata{
            ID:        generateID(),
            Timestamp: time.Now(),
            Source:    p.source,
            Version:   1,
        },
    }
    return p.bus.Publish(ctx, event)
}

// PublishSync publishes an event synchronously.
func (p *Publisher) PublishSync(ctx context.Context, topic Topic, payload any) error {
    event := Event[any]{
        Type:    topic,
        Payload: payload,
        Metadata: Metadata{
            ID:        generateID(),
            Timestamp: time.Now(),
            Source:    p.source,
            Version:   1,
        },
    }
    return p.bus.PublishSync(ctx, event)
}
```

### 11.2 Subscriber Helper

```go
// internal/event/subscriber.go

// Subscriber is a helper for modules to subscribe to events.
type Subscriber struct {
    bus           Bus
    subscriptions []Subscription
    mu            sync.Mutex
}

// NewSubscriber creates a new subscriber helper.
func NewSubscriber(bus Bus) *Subscriber {
    return &Subscriber{
        bus: bus,
    }
}

// On subscribes to a topic with a typed handler.
func On[T any](s *Subscriber, topic Topic, handler func(ctx context.Context, event Event[T]) error, opts ...SubscribeOption) error {
    sub, err := s.bus.Subscribe(topic, AsHandler(TypedHandlerFunc[T](handler)), opts...)
    if err != nil {
        return err
    }

    s.mu.Lock()
    s.subscriptions = append(s.subscriptions, sub)
    s.mu.Unlock()

    return nil
}

// Close unsubscribes from all subscriptions.
func (s *Subscriber) Close() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    for _, sub := range s.subscriptions {
        _ = s.bus.Unsubscribe(sub)
    }
    s.subscriptions = nil
    return nil
}
```

### 11.3 Module Integration Example

```go
// Example: Engine integration with event bus

// internal/engine/engine.go

type Engine struct {
    buffer    *Buffer
    publisher *event.Publisher
    // ...
}

func NewEngine(bus event.Bus) *Engine {
    return &Engine{
        publisher: event.NewPublisher(bus, "engine"),
    }
}

// Insert inserts text and publishes an event.
func (e *Engine) Insert(ctx context.Context, pos Position, text string) error {
    // Perform the insert
    result, err := e.buffer.Insert(pos, text)
    if err != nil {
        return err
    }

    // Publish event synchronously (critical path)
    return e.publisher.PublishSync(ctx, events.TopicBufferContentInserted, events.BufferContentInserted{
        BufferID:   e.buffer.ID(),
        Position:   pos,
        Text:       text,
        NewRange:   result.NewRange,
        RevisionID: result.RevisionID,
    })
}
```

### 11.4 EventPublisher Adapter (For Integration Layer)

```go
// internal/event/adapter.go

// EventPublisherAdapter adapts Bus to the EventPublisher interface
// used by the integration layer.
type EventPublisherAdapter struct {
    bus    Bus
    source string
}

// NewEventPublisherAdapter creates an adapter for the integration layer.
func NewEventPublisherAdapter(bus Bus, source string) *EventPublisherAdapter {
    return &EventPublisherAdapter{
        bus:    bus,
        source: source,
    }
}

// Publish implements the EventPublisher interface from integration layer.
func (a *EventPublisherAdapter) Publish(eventType string, data map[string]any) {
    ctx := context.Background()
    event := Event[map[string]any]{
        Type:    Topic(eventType),
        Payload: data,
        Metadata: Metadata{
            ID:        generateID(),
            Timestamp: time.Now(),
            Source:    a.source,
            Version:   1,
        },
    }
    _ = a.bus.PublishAsync(ctx, event)
}
```

---

## 12. Implementation Phases

### Phase 1: Core Event System (Foundation)
**Files:** `event.go`, `types.go`, `errors.go`, `topic/topic.go`, `topic/matcher.go`

1. Implement `Event[T]` generic type with metadata
2. Implement `Topic` type with hierarchical operations
3. Implement basic wildcard pattern matching
4. Implement error types
5. Write comprehensive tests for topic matching

**Deliverables:**
- Type-safe event representation
- Topic parsing and matching
- Wildcard support (`*`, `**`)

### Phase 2: Subscription Management
**Files:** `subscription.go`, `registry.go`, `registry_test.go`

1. Implement `subscription` type with lifecycle
2. Implement `Registry` with thread-safe operations
3. Implement subscription options (priority, filter, once)
4. Write tests for subscription management

**Deliverables:**
- Subscription lifecycle (active, paused, cancelled)
- Priority-ordered subscription retrieval
- Filter predicates

### Phase 3: Synchronous Dispatcher
**Files:** `dispatch/dispatcher.go`, `dispatch/sync.go`, `dispatch/executor.go`

1. Implement `SyncDispatcher` with panic recovery
2. Implement handler execution with context support
3. Write tests for sync dispatch

**Deliverables:**
- Blocking event delivery
- Panic isolation
- Context cancellation support

### Phase 4: Asynchronous Dispatcher
**Files:** `dispatch/async.go`, `dispatch/async_test.go`

1. Implement worker pool with bounded queue
2. Implement graceful shutdown with drain
3. Implement handler timeout
4. Write tests including edge cases (queue full, timeout)

**Deliverables:**
- Non-blocking event delivery
- Configurable worker pool
- Queue overflow handling

### Phase 5: Bus Implementation
**Files:** `bus.go`, `bus_test.go`, `options.go`

1. Implement main `bus` type combining all components
2. Implement `Publish`, `PublishSync`, `PublishAsync`
3. Implement `Subscribe`, `Unsubscribe`
4. Implement lifecycle methods (`Start`, `Stop`, `Pause`, `Resume`)
5. Write integration tests

**Deliverables:**
- Complete Bus interface implementation
- Configurable options
- Stats/metrics collection

### Phase 6: Topic Routing Optimization
**Files:** `topic/trie.go`, `topic/trie_test.go`

1. Implement trie-based pattern storage
2. Optimize wildcard matching algorithm
3. Benchmark against linear search
4. Write performance tests

**Deliverables:**
- O(k) pattern matching (k = topic segments)
- Efficient memory usage

### Phase 7: Typed Events
**Files:** `events/*.go`

1. Define all buffer events
2. Define all cursor events
3. Define all input events
4. Define all config events
5. Define all project events
6. Define all plugin events
7. Define all LSP events
8. Define all integration events (terminal, git, debug, task)
9. Define all dispatcher events
10. Define all renderer events

**Deliverables:**
- Comprehensive event type definitions
- Topic constants for each event type
- Documentation for each event

### Phase 8: Integration Helpers
**Files:** `publisher.go`, `subscriber.go`, `adapter.go`, `filter.go`

1. Implement `Publisher` helper
2. Implement `Subscriber` helper with typed handlers
3. Implement `EventPublisherAdapter` for integration layer
4. Implement common filter predicates

**Deliverables:**
- Easy-to-use publishing API
- Type-safe subscription helpers
- Backward compatibility with integration layer

### Phase 9: Metrics and Observability
**Files:** `metrics.go`, `metrics_test.go`

1. Implement event count metrics
2. Implement latency tracking
3. Implement queue depth monitoring
4. Implement handler error tracking

**Deliverables:**
- Runtime statistics
- Performance monitoring
- Debugging support

### Phase 10: Documentation and Polish
**Files:** `doc.go`, README updates

1. Write package documentation
2. Add usage examples
3. Document best practices
4. Performance tuning guide

**Deliverables:**
- Comprehensive documentation
- Example code
- Integration guide

---

## 13. Testing Strategy

### 13.1 Unit Tests

```go
// internal/event/bus_test.go

func TestBus_PublishSync(t *testing.T) {
    bus := NewBus()
    bus.Start()
    defer bus.Stop(context.Background())

    received := make(chan events.BufferContentInserted, 1)

    _, err := bus.Subscribe(events.TopicBufferContentInserted,
        HandlerFunc(func(ctx context.Context, e any) error {
            if evt, ok := e.(Event[events.BufferContentInserted]); ok {
                received <- evt.Payload
            }
            return nil
        }),
        WithDeliveryMode(DeliverySync),
    )
    require.NoError(t, err)

    event := NewEvent(events.TopicBufferContentInserted,
        events.BufferContentInserted{
            BufferID: "test",
            Text:     "hello",
        },
        "test",
    )

    err = bus.PublishSync(context.Background(), event)
    require.NoError(t, err)

    select {
    case evt := <-received:
        assert.Equal(t, "test", evt.BufferID)
        assert.Equal(t, "hello", evt.Text)
    case <-time.After(time.Second):
        t.Fatal("timeout waiting for event")
    }
}

func TestBus_WildcardSubscription(t *testing.T) {
    bus := NewBus()
    bus.Start()
    defer bus.Stop(context.Background())

    received := atomic.Int32{}

    // Subscribe to buffer.*
    _, err := bus.Subscribe(Topic("buffer.*"),
        HandlerFunc(func(ctx context.Context, e any) error {
            received.Add(1)
            return nil
        }),
        WithDeliveryMode(DeliverySync),
    )
    require.NoError(t, err)

    // Publish different buffer events
    bus.PublishSync(context.Background(), NewEvent(
        Topic("buffer.content.inserted"), struct{}{}, "test"))
    bus.PublishSync(context.Background(), NewEvent(
        Topic("buffer.content.deleted"), struct{}{}, "test"))
    bus.PublishSync(context.Background(), NewEvent(
        Topic("cursor.moved"), struct{}{}, "test")) // Should not match

    assert.Equal(t, int32(2), received.Load())
}

func TestBus_Priority(t *testing.T) {
    bus := NewBus()
    bus.Start()
    defer bus.Stop(context.Background())

    order := make([]string, 0, 3)
    var mu sync.Mutex

    // Subscribe with different priorities (out of order)
    bus.Subscribe(Topic("test"),
        HandlerFunc(func(ctx context.Context, e any) error {
            mu.Lock()
            order = append(order, "normal")
            mu.Unlock()
            return nil
        }),
        WithPriority(PriorityNormal),
        WithDeliveryMode(DeliverySync),
    )

    bus.Subscribe(Topic("test"),
        HandlerFunc(func(ctx context.Context, e any) error {
            mu.Lock()
            order = append(order, "critical")
            mu.Unlock()
            return nil
        }),
        WithPriority(PriorityCritical),
        WithDeliveryMode(DeliverySync),
    )

    bus.Subscribe(Topic("test"),
        HandlerFunc(func(ctx context.Context, e any) error {
            mu.Lock()
            order = append(order, "low")
            mu.Unlock()
            return nil
        }),
        WithPriority(PriorityLow),
        WithDeliveryMode(DeliverySync),
    )

    bus.PublishSync(context.Background(), NewEvent(Topic("test"), struct{}{}, "test"))

    assert.Equal(t, []string{"critical", "normal", "low"}, order)
}
```

### 13.2 Topic Matcher Tests

```go
// internal/event/topic/matcher_test.go

func TestMatcher_ExactMatch(t *testing.T) {
    m := NewMatcher()
    m.Add(Topic("buffer.content.inserted"))

    matches := m.Match(Topic("buffer.content.inserted"))
    assert.Len(t, matches, 1)
    assert.Equal(t, Topic("buffer.content.inserted"), matches[0])

    // No match
    matches = m.Match(Topic("buffer.content.deleted"))
    assert.Len(t, matches, 0)
}

func TestMatcher_SingleWildcard(t *testing.T) {
    m := NewMatcher()
    m.Add(Topic("buffer.*.inserted"))

    // Matches
    matches := m.Match(Topic("buffer.content.inserted"))
    assert.Len(t, matches, 1)

    // Different segment - still matches
    matches = m.Match(Topic("buffer.text.inserted"))
    assert.Len(t, matches, 1)

    // Different last segment - no match
    matches = m.Match(Topic("buffer.content.deleted"))
    assert.Len(t, matches, 0)
}

func TestMatcher_MultiWildcard(t *testing.T) {
    m := NewMatcher()
    m.Add(Topic("buffer.**"))

    // Matches various depths
    matches := m.Match(Topic("buffer.content"))
    assert.Len(t, matches, 1)

    matches = m.Match(Topic("buffer.content.inserted"))
    assert.Len(t, matches, 1)

    matches = m.Match(Topic("buffer.content.text.inserted"))
    assert.Len(t, matches, 1)

    // Different prefix - no match
    matches = m.Match(Topic("cursor.moved"))
    assert.Len(t, matches, 0)
}
```

### 13.3 Async Dispatcher Tests

```go
// internal/event/dispatch/async_test.go

func TestAsyncDispatcher_QueueFull(t *testing.T) {
    d := NewAsyncDispatcher(
        WithQueueSize(2),
        WithWorkerCount(0), // No workers - queue fills up
    )
    d.Start()
    defer d.Stop(context.Background())

    sub := &subscription{
        id:      "test",
        handler: HandlerFunc(func(ctx context.Context, e any) error { return nil }),
        config:  subscriptionConfig{deliveryMode: DeliveryAsync},
    }
    sub.active.Store(true)

    // Fill queue
    err := d.Enqueue(context.Background(), struct{}{}, sub)
    assert.NoError(t, err)
    err = d.Enqueue(context.Background(), struct{}{}, sub)
    assert.NoError(t, err)

    // Queue full
    err = d.Enqueue(context.Background(), struct{}{}, sub)
    assert.ErrorIs(t, err, ErrQueueFull)
}

func TestAsyncDispatcher_GracefulShutdown(t *testing.T) {
    d := NewAsyncDispatcher(
        WithQueueSize(100),
        WithWorkerCount(2),
    )
    d.Start()

    executed := atomic.Int32{}
    sub := &subscription{
        id: "test",
        handler: HandlerFunc(func(ctx context.Context, e any) error {
            time.Sleep(10 * time.Millisecond)
            executed.Add(1)
            return nil
        }),
        config: subscriptionConfig{deliveryMode: DeliveryAsync},
    }
    sub.active.Store(true)

    // Enqueue several tasks
    for i := 0; i < 10; i++ {
        d.Enqueue(context.Background(), struct{}{}, sub)
    }

    // Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := d.Stop(ctx)
    assert.NoError(t, err)
    assert.Equal(t, int32(10), executed.Load())
}
```

### 13.4 Integration Tests

```go
// internal/event/integration_test.go

func TestEventBus_EngineRendererFlow(t *testing.T) {
    // Simulate the buffer change -> renderer redraw flow
    bus := NewBus()
    bus.Start()
    defer bus.Stop(context.Background())

    rendererCalled := make(chan struct{})

    // Renderer subscribes to buffer changes (critical priority, sync)
    bus.Subscribe(Topic("buffer.*"),
        HandlerFunc(func(ctx context.Context, e any) error {
            close(rendererCalled)
            return nil
        }),
        WithPriority(PriorityCritical),
        WithDeliveryMode(DeliverySync),
    )

    // Engine publishes buffer change
    publisher := NewPublisher(bus, "engine")
    err := publisher.PublishSync(context.Background(),
        events.TopicBufferContentInserted,
        events.BufferContentInserted{
            BufferID: "test",
            Text:     "hello",
        },
    )
    require.NoError(t, err)

    // Renderer should have been called synchronously
    select {
    case <-rendererCalled:
        // Success
    default:
        t.Fatal("renderer was not called synchronously")
    }
}

func TestEventBus_PluginAsyncDelivery(t *testing.T) {
    bus := NewBus()
    bus.Start()
    defer bus.Stop(context.Background())

    pluginCalled := make(chan struct{})

    // Plugin subscribes to buffer changes (normal priority, async)
    bus.Subscribe(Topic("buffer.*"),
        HandlerFunc(func(ctx context.Context, e any) error {
            close(pluginCalled)
            return nil
        }),
        WithPriority(PriorityNormal),
        WithDeliveryMode(DeliveryAsync),
    )

    // Engine publishes buffer change
    publisher := NewPublisher(bus, "engine")
    err := publisher.Publish(context.Background(),
        events.TopicBufferContentInserted,
        events.BufferContentInserted{
            BufferID: "test",
            Text:     "hello",
        },
    )
    require.NoError(t, err)

    // Plugin should be called asynchronously
    select {
    case <-pluginCalled:
        // Success
    case <-time.After(time.Second):
        t.Fatal("plugin was not called")
    }
}
```

### 13.5 Benchmark Tests

```go
// internal/event/bench_test.go

func BenchmarkBus_PublishSync(b *testing.B) {
    bus := NewBus()
    bus.Start()
    defer bus.Stop(context.Background())

    bus.Subscribe(Topic("test"),
        HandlerFunc(func(ctx context.Context, e any) error { return nil }),
        WithDeliveryMode(DeliverySync),
    )

    event := NewEvent(Topic("test"), struct{}{}, "bench")
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        bus.PublishSync(ctx, event)
    }
}

func BenchmarkBus_PublishAsync(b *testing.B) {
    bus := NewBus()
    bus.Start()
    defer bus.Stop(context.Background())

    bus.Subscribe(Topic("test"),
        HandlerFunc(func(ctx context.Context, e any) error { return nil }),
        WithDeliveryMode(DeliveryAsync),
    )

    event := NewEvent(Topic("test"), struct{}{}, "bench")
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        bus.PublishAsync(ctx, event)
    }
}

func BenchmarkMatcher_Match(b *testing.B) {
    m := NewMatcher()

    // Add patterns
    patterns := []Topic{
        "buffer.content.inserted",
        "buffer.content.deleted",
        "buffer.*",
        "cursor.*",
        "config.**",
    }
    for _, p := range patterns {
        m.Add(p)
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        m.Match(Topic("buffer.content.inserted"))
    }
}

func BenchmarkBus_ManySubscribers(b *testing.B) {
    bus := NewBus()
    bus.Start()
    defer bus.Stop(context.Background())

    // Add many subscribers
    for i := 0; i < 100; i++ {
        bus.Subscribe(Topic("test"),
            HandlerFunc(func(ctx context.Context, e any) error { return nil }),
            WithDeliveryMode(DeliverySync),
        )
    }

    event := NewEvent(Topic("test"), struct{}{}, "bench")
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        bus.PublishSync(ctx, event)
    }
}
```

---

## 14. Performance Considerations

### 14.1 Critical Path Optimization

The buffer change → renderer redraw path is latency-critical:

```go
// Optimizations for sync delivery:

1. **Pre-allocated handler slice**: Copy subscriptions to pre-allocated slice
   to avoid allocation during dispatch.

2. **Inline type assertion**: Use type switch instead of reflection where
   possible.

3. **Zero-allocation events**: For high-frequency events, consider object
   pooling.

4. **Lock-free stats**: Use atomic operations for metrics to avoid lock
   contention.
```

### 14.2 Memory Management

```go
// Object pooling for high-frequency events
var eventPool = sync.Pool{
    New: func() any {
        return &Event[any]{}
    },
}

// Use for hot paths:
func getPooledEvent() *Event[any] {
    return eventPool.Get().(*Event[any])
}

func putPooledEvent(e *Event[any]) {
    e.Payload = nil
    eventPool.Put(e)
}
```

### 14.3 Queue Sizing Guidelines

| Use Case | Queue Size | Worker Count |
|----------|------------|--------------|
| Development | 1,000 | 4 |
| Production (small) | 10,000 | 10 |
| Production (large) | 100,000 | 50 |
| High-throughput | 1,000,000 | 100 |

### 14.4 Backpressure Handling

```go
// Options for handling queue overflow:

1. **Drop newest**: Current implementation - drops if queue is full
2. **Drop oldest**: Remove oldest event to make room
3. **Block**: Block publisher until space available (not recommended for UI)
4. **Overflow buffer**: Secondary buffer for burst handling
```

### 14.5 Performance Targets

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| Sync dispatch latency | < 10μs | Benchmark single handler |
| Async enqueue latency | < 1μs | Benchmark queue insert |
| Pattern match latency | < 100ns | Benchmark trie lookup |
| Memory per subscription | < 1KB | Profile heap usage |
| Throughput | > 100K events/sec | Benchmark sustained load |

---

## Summary

The Event & Messaging Bus provides Keystorm with a robust, performant, and type-safe event-driven communication system. Key features include:

1. **Type-safe events** using Go generics
2. **Hierarchical topics** with wildcard pattern matching
3. **Dual delivery modes** (sync for critical paths, async for plugins)
4. **Priority-ordered handlers** (renderer > LSP > plugins > metrics)
5. **Thread-safe operations** with minimal lock contention
6. **Panic isolation** to prevent handler failures from crashing the editor
7. **Graceful lifecycle management** with drain on shutdown
8. **Comprehensive metrics** for monitoring and debugging

The implementation follows Keystorm's design principles of being "AI-agnostic but rock solid" while enabling the decoupled, event-driven architecture needed for an AI-native editor.
