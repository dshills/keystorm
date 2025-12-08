# Event Bus Guide

This guide covers best practices, performance tuning, and common patterns for using the Keystorm event bus.

## Best Practices

### 1. Choose the Right Delivery Mode

**Use synchronous delivery when:**
- The handler must complete before the publisher continues
- You need guaranteed ordering with other handlers
- The handler is fast (<1ms) and non-blocking
- UI updates that must complete before the next frame

```go
// Critical path - use sync
bus.SubscribeFunc(topic, handler, event.WithDeliveryMode(event.DeliverySync))
```

**Use asynchronous delivery when:**
- The handler does I/O operations (file, network)
- The handler can tolerate slight delays
- You want to avoid blocking the publisher
- For metrics, logging, and analytics

```go
// Non-critical - use async
bus.SubscribeFunc(topic, handler, event.WithDeliveryMode(event.DeliveryAsync))
```

### 2. Use Specific Topics Over Broad Wildcards

Wildcards are powerful but can lead to performance issues and unexpected behavior.

```go
// Prefer specific topics
bus.Subscribe(topic.Topic("buffer.content.inserted"), handler)

// Use wildcards judiciously
bus.Subscribe(topic.Topic("buffer.*"), handler)  // OK for small sets
bus.Subscribe(topic.Topic("**"), handler)         // Avoid - matches everything
```

### 3. Set Appropriate Priorities

Priorities ensure handlers execute in the correct order:

```go
// Renderer needs events first for UI consistency
bus.SubscribeFunc(topic, renderHandler,
    event.WithPriority(event.PriorityCritical))

// Metrics can run last
bus.SubscribeFunc(topic, metricsHandler,
    event.WithPriority(event.PriorityLow))
```

Priority guidelines:
- **Critical (100)**: Core engine, renderer - state-affecting handlers
- **High (75)**: LSP, dispatcher - important but not state-critical
- **Normal (50)**: Plugins, integrations - default for most handlers
- **Low (25)**: Metrics, logging - observers only

### 4. Handle Errors Gracefully

Always return errors from handlers - they're logged but don't stop other handlers:

```go
func handler(ctx context.Context, e any) error {
    if err := doSomething(); err != nil {
        // Return error for logging, but consider if you should
        // also emit an error event for recovery
        return fmt.Errorf("handler failed: %w", err)
    }
    return nil
}
```

### 5. Use Context for Cancellation

Respect context cancellation in long-running handlers:

```go
func handler(ctx context.Context, e any) error {
    for _, item := range largeSlice {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        process(item)
    }
    return nil
}
```

### 6. Avoid Handler Side Effects on Other Handlers

Handlers should not modify shared state that other handlers depend on without proper synchronization:

```go
// Bad - modifies shared state
var counter int
func handler1(ctx context.Context, e any) error {
    counter++  // Race condition!
    return nil
}

// Good - use synchronization or message passing
var counter atomic.Int64
func handler1(ctx context.Context, e any) error {
    counter.Add(1)
    return nil
}
```

## Performance Tuning

### Topic Matching Performance

The trie-based topic matcher provides O(k) matching where k is the number of topic segments:

| Pattern Count | Linear Search | Trie Match | Speedup |
|---------------|---------------|------------|---------|
| 5             | ~500ns        | ~150ns     | 3x      |
| 50            | ~5us          | ~170ns     | 30x     |
| 1000          | ~100us        | ~175ns     | 570x    |

**Recommendations:**
- Use the default trie matcher for best performance
- Avoid very long topic names (>10 segments)
- Pre-compile topic patterns where possible

### Subscription Management

```go
// Cache subscription IDs for later removal
subID, _ := bus.Subscribe(topic, handler)

// Unsubscribe when no longer needed
bus.Unsubscribe(subID)
```

### Batch Publishing

For high-frequency events, consider batching:

```go
// Instead of publishing every keystroke
for _, char := range input {
    bus.Publish(ctx, charEvent)  // High overhead
}

// Batch into single event
bus.Publish(ctx, inputBatchEvent)  // Lower overhead
```

### Memory Allocation

Reduce allocations in hot paths:

```go
// Reuse event objects where safe
type BufferEvent struct {
    Data     string
    BufferID string
}

func (e *BufferEvent) Reset() {
    e.Data = ""
    e.BufferID = ""
}

var eventPool = sync.Pool{
    New: func() any {
        return &BufferEvent{}
    },
}

func publishBufferEvent(data string, bufferID string) {
    evt := eventPool.Get().(*BufferEvent)
    evt.Data = data
    evt.BufferID = bufferID
    bus.Publish(ctx, evt)
    // Reset fields before returning to pool to prevent data leaks
    evt.Reset()
    eventPool.Put(evt)
}
```

> **Warning**: Always reset pooled objects before returning them to the pool
> to prevent accidental data retention or leaking sensitive information.

### Profiling Events

Use the built-in metrics for performance analysis:

```go
// Get bus statistics
stats := bus.Stats()
fmt.Printf("Subscriptions: %d\n", stats.SubscriptionCount)
fmt.Printf("Events published: %d\n", stats.PublishCount)
```

## Common Patterns

### Request-Response Pattern

When you need a response from a handler:

```go
// Create a response channel
type RequestEvent struct {
    Query    string
    Response chan<- string
}

// Publisher
respCh := make(chan string, 1)
bus.PublishSync(ctx, event.NewEvent(topic, RequestEvent{
    Query:    "lookup",
    Response: respCh,
}, "requester"))
result := <-respCh

// Handler - use safe type assertion
func handler(ctx context.Context, e any) error {
    req, ok := e.(RequestEvent)
    if !ok {
        return fmt.Errorf("unexpected event type: %T", e)
    }
    req.Response <- "result"
    return nil
}
```

### Event Sourcing

Build state from event history:

```go
type StateBuilder struct {
    state map[string]string
}

func (b *StateBuilder) HandleInsert(ctx context.Context, e any) error {
    evt, ok := e.(events.BufferContentInserted)
    if !ok {
        return fmt.Errorf("expected BufferContentInserted, got %T", e)
    }
    b.state[evt.BufferID] = evt.Text
    return nil
}

func (b *StateBuilder) HandleDelete(ctx context.Context, e any) error {
    evt, ok := e.(events.BufferContentDeleted)
    if !ok {
        return fmt.Errorf("expected BufferContentDeleted, got %T", e)
    }
    delete(b.state, evt.BufferID)
    return nil
}
```

### Fan-Out Pattern

Distribute events to multiple independent handlers:

```go
// Multiple handlers for same topic
bus.Subscribe(topic.Topic("file.saved"), notifyUser)
bus.Subscribe(topic.Topic("file.saved"), triggerBackup)
bus.Subscribe(topic.Topic("file.saved"), updateIndex)
// All handlers execute for each file.saved event
```

### Debouncing

Prevent event storms:

```go
type DebouncedHandler struct {
    timer    *time.Timer
    duration time.Duration
    fn       func()
    mu       sync.Mutex
}

func (d *DebouncedHandler) Handle(ctx context.Context, e any) error {
    d.mu.Lock()
    defer d.mu.Unlock()

    if d.timer != nil {
        // Stop returns false if the timer already fired.
        // We don't need to drain the channel since AfterFunc doesn't use one.
        d.timer.Stop()
    }
    d.timer = time.AfterFunc(d.duration, func() {
        // The callback runs outside the mutex, which is correct.
        // If you need to access shared state in fn, ensure fn handles
        // its own synchronization.
        d.fn()
    })
    return nil
}
```

> **Note**: The callback function (`fn`) executes outside the mutex lock. If `fn`
> accesses shared state, it must handle its own synchronization.

## Debugging

### Enable Debug Logging

```go
bus := event.NewBus(event.WithDebugLogging(true))
```

### Trace Event Flow

```go
// Add a tracing subscriber - use TopicProvider interface for type-safe access
bus.Subscribe(topic.Topic("**"), func(ctx context.Context, e any) error {
    env := event.ToEnvelope(e)
    log.Printf("Event: %s from %s", env.Topic, env.Metadata.Source)
    return nil
}, event.WithPriority(event.PriorityLow))
```

> **Warning**: Be careful when logging event payloads as they may contain
> sensitive data (credentials, tokens, user input). Only log metadata in
> production environments.

### Check Subscription State

```go
// List all subscriptions
for _, sub := range bus.Subscriptions() {
    fmt.Printf("Topic: %s, Priority: %d\n", sub.Topic, sub.Priority)
}
```
