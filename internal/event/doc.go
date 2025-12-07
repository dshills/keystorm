// Package event provides the Event & Messaging Bus for Keystorm.
//
// The event bus is the editor's "nervous system" - a central communication backbone
// enabling decoupled, event-driven architecture across all editor components. It
// facilitates communication between modules (engine, renderer, input, plugins, LSP,
// integrations) without direct dependencies.
//
// # Event Types
//
// Events use hierarchical topics with dot notation:
//
//	buffer.content.inserted
//	cursor.moved
//	config.changed
//	plugin.vim-surround.activated
//
// # Wildcards
//
// Subscriptions support wildcard patterns:
//
//	buffer.*     - matches buffer.content, buffer.cleared, etc.
//	buffer.**    - matches buffer.content.inserted, buffer.content.deleted, etc.
//	*.changed    - matches config.changed, cursor.changed, etc.
//
// # Delivery Modes
//
// Events can be delivered synchronously or asynchronously:
//
//   - Sync: Handler executes in publisher's goroutine (for critical paths)
//   - Async: Handler executes in worker pool (for non-blocking operations)
//
// # Priority
//
// Handlers execute in priority order:
//
//   - Critical: Renderer, core engine (first)
//   - High: LSP, dispatcher
//   - Normal: Plugins, integrations (default)
//   - Low: Metrics, logging (last)
//
// # Basic Usage
//
//	// Create and start the bus
//	bus := event.NewBus()
//	bus.Start()
//	defer bus.Stop(context.Background())
//
//	// Subscribe to events
//	bus.Subscribe(event.Topic("buffer.*"), handler, event.WithPriority(event.PriorityCritical))
//
//	// Publish events
//	evt := event.NewEvent(event.Topic("buffer.content.inserted"), payload, "engine")
//	bus.Publish(ctx, evt)
//
// # Type-Safe Events
//
// Use generics for type-safe event handling:
//
//	type BufferInserted struct {
//	    BufferID string
//	    Text     string
//	}
//
//	evt := event.NewEvent[BufferInserted](topic, BufferInserted{...}, "engine")
package event
