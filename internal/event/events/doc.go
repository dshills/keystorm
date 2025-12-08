// Package events defines strongly-typed event payloads for the Keystorm event bus.
//
// Each event type has a corresponding topic constant and payload struct. Events are
// grouped by their source module:
//
//   - Buffer events: text insertions, deletions, revisions
//   - Cursor events: cursor movement, selection changes, multi-cursor
//   - Input events: keystrokes, mode changes, macros
//   - Config events: setting changes, keymap updates
//   - Project events: file operations, workspace lifecycle
//   - Plugin events: plugin lifecycle, errors
//   - LSP events: server lifecycle, diagnostics, completions
//   - Integration events: terminal, git, debugger, task runner
//   - Dispatcher events: action execution lifecycle
//   - Renderer events: frame rendering, scrolling, resizing
//
// # Usage
//
// Events are typically created using the event.NewEvent function:
//
//	import (
//	    "github.com/dshills/keystorm/internal/event"
//	    "github.com/dshills/keystorm/internal/event/events"
//	)
//
//	// Create and publish a buffer insert event
//	evt := event.NewEvent(events.TopicBufferContentInserted,
//	    events.BufferContentInserted{
//	        BufferID: "buf-123",
//	        Position: events.Position{Line: 10, Column: 5},
//	        Text:     "hello",
//	    },
//	    "engine",
//	)
//	bus.PublishSync(ctx, evt)
//
// # Topic Naming Convention
//
// Topics follow a hierarchical dot-notation:
//
//	<module>.<entity>.<action>
//
// Examples:
//   - buffer.content.inserted
//   - cursor.moved
//   - lsp.diagnostics.published
//   - git.status.changed
//
// # Wildcard Subscriptions
//
// Subscribers can use wildcards to match multiple topics:
//   - "*" matches exactly one segment: "buffer.*" matches "buffer.cleared"
//   - "**" matches zero or more segments: "buffer.**" matches "buffer.content.inserted"
package events
