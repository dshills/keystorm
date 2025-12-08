// Package event provides the Event & Messaging Bus for Keystorm.
//
// The event bus is the editor's "nervous system" - a central communication backbone
// enabling decoupled, event-driven architecture across all editor components. It
// facilitates communication between modules (engine, renderer, input, plugins, LSP,
// integrations) without direct dependencies.
//
// # Architecture
//
// The event system consists of several interconnected components:
//
//	                    ┌──────────────────────────────────────────┐
//	                    │               Event Bus                   │
//	                    │  - Subscriber registry                    │
//	                    │  - Topic matching (trie-based)            │
//	                    │  - Sync/Async dispatch                    │
//	                    └──────────────────────────────────────────┘
//	                                      │
//	          ┌───────────────────────────┼───────────────────────────┐
//	          ▼                           ▼                           ▼
//	┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
//	│    Registry     │         │     Filter      │         │   Publisher     │
//	│  - Subscription │         │  - Topic-based  │         │  - BusAdapter   │
//	│    management   │         │  - Source-based │         │  - Integration  │
//	└─────────────────┘         │  - Payload      │         │    bridge       │
//	                            └─────────────────┘         └─────────────────┘
//
// # Event Topics
//
// Events use hierarchical topics with dot notation:
//
//	buffer.content.inserted    - Text was inserted into a buffer
//	cursor.moved               - Cursor position changed
//	config.changed             - Configuration was modified
//	plugin.vim-surround.activated - Plugin-specific event
//
// # Wildcard Patterns
//
// Subscriptions support wildcard patterns for flexible matching:
//
//	buffer.*     - matches buffer.content, buffer.cleared (single segment)
//	buffer.**    - matches buffer.content.inserted, buffer.a.b.c (multi-segment)
//	*.changed    - matches config.changed, cursor.changed (prefix wildcard)
//
// # Delivery Modes
//
// Events can be delivered synchronously or asynchronously:
//
//   - Sync: Handler executes in publisher's goroutine (for critical paths)
//   - Async: Handler executes in worker pool (for non-blocking operations)
//
// Choose synchronous delivery for:
//   - UI updates that must complete before next frame
//   - State changes that other handlers depend on
//   - Low-latency requirements
//
// Choose asynchronous delivery for:
//   - Plugin notifications
//   - Metrics collection
//   - Non-critical logging
//
// # Priority Ordering
//
// Handlers execute in priority order for deterministic behavior:
//
//   - Critical (100): Renderer, core engine - executes first
//   - High (75): LSP, dispatcher
//   - Normal (50): Plugins, integrations - default priority
//   - Low (25): Metrics, logging - executes last
//
// # Basic Usage
//
//	// Create and start the bus
//	bus := event.NewBus()
//	if err := bus.Start(); err != nil {
//	    log.Fatal(err)
//	}
//	defer bus.Stop(context.Background())
//
//	// Subscribe to events with options
//	subID, err := bus.Subscribe(
//	    event.Topic("buffer.*"),
//	    handler,
//	    event.WithPriority(event.PriorityCritical),
//	    event.WithDeliveryMode(event.DeliverySync),
//	)
//
//	// Publish events
//	evt := event.NewEvent(event.Topic("buffer.content.inserted"), payload, "engine")
//	bus.Publish(ctx, evt)
//
//	// Synchronous publish with error handling
//	if err := bus.PublishSync(ctx, evt); err != nil {
//	    log.Printf("publish failed: %v", err)
//	}
//
// # Type-Safe Events
//
// Use generics for compile-time type safety:
//
//	// Define strongly-typed event
//	type BufferInserted struct {
//	    BufferID string
//	    Position Position
//	    Text     string
//	}
//
//	// Create typed event
//	evt := event.NewEvent(topic, BufferInserted{
//	    BufferID: "buf-123",
//	    Position: Position{Line: 10, Column: 5},
//	    Text:     "hello",
//	}, "engine")
//
//	// Type-safe handler with TypedSubscriber
//	subscriber := event.NewTypedSubscriber[BufferInserted](bus)
//	subscriber.Subscribe(topic, func(ctx context.Context, evt BufferInserted) error {
//	    fmt.Printf("Text inserted: %s at %d:%d\n", evt.Text, evt.Position.Line, evt.Position.Column)
//	    return nil
//	})
//
// # Filtering
//
// Use filters to conditionally process events:
//
//	// Topic-based filter
//	filter := event.NewTopicFilter("buffer.**")
//
//	// Source-based filter
//	filter := event.NewSourceFilter("engine", "lsp")
//
//	// Composite filters
//	filter := event.AndFilter(topicFilter, sourceFilter)
//	filter := event.OrFilter(filter1, filter2)
//
// # Integration Layer Bridge
//
// The BusAdapter connects the typed event bus to the integration layer:
//
//	adapter := event.NewBusAdapter(bus, "integration")
//
//	// Publish from integration layer (map[string]any format)
//	adapter.Publish("git.status.changed", map[string]any{
//	    "branch": "main",
//	    "clean":  true,
//	})
//
// # Performance Considerations
//
//   - Use async delivery for non-critical handlers to avoid blocking publishers
//   - Subscribe to specific topics rather than broad wildcards when possible
//   - Use filters to reduce unnecessary handler invocations
//   - The trie-based topic matcher provides O(k) matching where k is segments
//   - Sync dispatch adds ~500ns overhead; async adds ~2-5us for goroutine spawn
//
// # Thread Safety
//
// The Bus and all public types are safe for concurrent use. Subscriptions can
// be added/removed while events are being published. However, individual handlers
// must manage their own thread safety.
//
// # Subpackages
//
//   - events: Strongly-typed event payload definitions
//   - topic: Topic types and trie-based pattern matching
//   - dispatch: Synchronous and asynchronous dispatch implementations
package event
