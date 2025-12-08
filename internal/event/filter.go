package event

import (
	"strings"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Common filter predicates for event subscription.

// FilterBySource creates a filter that only allows events from the specified source.
func FilterBySource(source string) FilterFunc {
	return func(event any) bool {
		if mp, ok := event.(MetadataProvider); ok {
			return mp.EventMetadata().Source == source
		}
		if env, ok := event.(Envelope); ok {
			return env.Metadata.Source == source
		}
		return false
	}
}

// FilterBySourcePrefix creates a filter that only allows events from sources starting with prefix.
func FilterBySourcePrefix(prefix string) FilterFunc {
	return func(event any) bool {
		var source string
		if mp, ok := event.(MetadataProvider); ok {
			source = mp.EventMetadata().Source
		} else if env, ok := event.(Envelope); ok {
			source = env.Metadata.Source
		}
		return source != "" && strings.HasPrefix(source, prefix)
	}
}

// FilterBySources creates a filter that only allows events from one of the specified sources.
func FilterBySources(sources ...string) FilterFunc {
	sourceSet := make(map[string]bool, len(sources))
	for _, s := range sources {
		sourceSet[s] = true
	}
	return func(event any) bool {
		var source string
		if mp, ok := event.(MetadataProvider); ok {
			source = mp.EventMetadata().Source
		} else if env, ok := event.(Envelope); ok {
			source = env.Metadata.Source
		}
		return sourceSet[source]
	}
}

// FilterExcludeSource creates a filter that excludes events from the specified source.
func FilterExcludeSource(source string) FilterFunc {
	return func(event any) bool {
		if mp, ok := event.(MetadataProvider); ok {
			return mp.EventMetadata().Source != source
		}
		if env, ok := event.(Envelope); ok {
			return env.Metadata.Source != source
		}
		return true
	}
}

// FilterByTopic creates a filter that only allows events matching the topic pattern.
// This is useful when subscribing to a wildcard but wanting finer-grained control.
func FilterByTopic(pattern topic.Topic) FilterFunc {
	return func(event any) bool {
		var eventTopic topic.Topic
		if tp, ok := event.(TopicProvider); ok {
			eventTopic = tp.EventTopic()
		} else if env, ok := event.(Envelope); ok {
			eventTopic = env.Topic
		}
		return eventTopic != "" && eventTopic.Matches(pattern)
	}
}

// FilterByTopicPrefix creates a filter for events with topics starting with prefix.
func FilterByTopicPrefix(prefix string) FilterFunc {
	return func(event any) bool {
		var eventTopic topic.Topic
		if tp, ok := event.(TopicProvider); ok {
			eventTopic = tp.EventTopic()
		} else if env, ok := event.(Envelope); ok {
			eventTopic = env.Topic
		}
		return eventTopic != "" && strings.HasPrefix(string(eventTopic), prefix)
	}
}

// FilterExcludeTopic creates a filter that excludes events matching the topic pattern.
func FilterExcludeTopic(pattern topic.Topic) FilterFunc {
	return func(event any) bool {
		var eventTopic topic.Topic
		if tp, ok := event.(TopicProvider); ok {
			eventTopic = tp.EventTopic()
		} else if env, ok := event.(Envelope); ok {
			eventTopic = env.Topic
		}
		if eventTopic == "" {
			return true
		}
		return !eventTopic.Matches(pattern)
	}
}

// FilterByCorrelation creates a filter that only allows events with the specified correlation ID.
func FilterByCorrelation(correlationID string) FilterFunc {
	return func(event any) bool {
		if mp, ok := event.(MetadataProvider); ok {
			return mp.EventMetadata().CorrelationID == correlationID
		}
		if env, ok := event.(Envelope); ok {
			return env.Metadata.CorrelationID == correlationID
		}
		return false
	}
}

// FilterByCausation creates a filter that only allows events with the specified causation ID.
func FilterByCausation(causationID string) FilterFunc {
	return func(event any) bool {
		if mp, ok := event.(MetadataProvider); ok {
			return mp.EventMetadata().CausationID == causationID
		}
		if env, ok := event.(Envelope); ok {
			return env.Metadata.CausationID == causationID
		}
		return false
	}
}

// FilterPayload creates a filter based on the payload.
// The predicate receives the payload and returns true if the event should be delivered.
func FilterPayload[T any](predicate func(payload T) bool) FilterFunc {
	return func(event any) bool {
		// Try typed Event[T]
		if e, ok := event.(Event[T]); ok {
			return predicate(e.Payload)
		}
		// Try Envelope with typed payload
		if env, ok := event.(Envelope); ok {
			if payload, ok := env.Payload.(T); ok {
				return predicate(payload)
			}
		}
		// Try direct payload
		if payload, ok := event.(T); ok {
			return predicate(payload)
		}
		return false
	}
}

// FilterAnd combines multiple filters with AND logic.
// All filters must pass for the event to be delivered.
func FilterAnd(filters ...FilterFunc) FilterFunc {
	return func(event any) bool {
		for _, f := range filters {
			if !f(event) {
				return false
			}
		}
		return true
	}
}

// FilterOr combines multiple filters with OR logic.
// At least one filter must pass for the event to be delivered.
func FilterOr(filters ...FilterFunc) FilterFunc {
	return func(event any) bool {
		for _, f := range filters {
			if f(event) {
				return true
			}
		}
		return false
	}
}

// FilterNot negates a filter.
func FilterNot(filter FilterFunc) FilterFunc {
	return func(event any) bool {
		return !filter(event)
	}
}

// FilterAll allows all events (no filtering).
func FilterAll() FilterFunc {
	return func(event any) bool {
		return true
	}
}

// FilterNone blocks all events.
func FilterNone() FilterFunc {
	return func(event any) bool {
		return false
	}
}

// FilterByVersion creates a filter for events with specific schema versions.
func FilterByVersion(version int) FilterFunc {
	return func(event any) bool {
		if mp, ok := event.(MetadataProvider); ok {
			return mp.EventMetadata().Version == version
		}
		if env, ok := event.(Envelope); ok {
			return env.Metadata.Version == version
		}
		return false
	}
}

// FilterByMinVersion creates a filter for events with at least the specified schema version.
func FilterByMinVersion(minVersion int) FilterFunc {
	return func(event any) bool {
		if mp, ok := event.(MetadataProvider); ok {
			return mp.EventMetadata().Version >= minVersion
		}
		if env, ok := event.(Envelope); ok {
			return env.Metadata.Version >= minVersion
		}
		return false
	}
}

// FilterByBufferID creates a filter for events related to a specific buffer.
// This checks for a BufferID field in the payload (common in buffer/cursor events).
func FilterByBufferID(bufferID string) FilterFunc {
	return func(event any) bool {
		// Try to extract BufferID from common payload types
		switch e := event.(type) {
		case Envelope:
			return checkBufferID(e.Payload, bufferID)
		default:
			return checkBufferID(event, bufferID)
		}
	}
}

// checkBufferID checks if the payload has a BufferID field matching the target.
func checkBufferID(payload any, targetID string) bool {
	// Check for map[string]any with BufferID key
	if m, ok := payload.(map[string]any); ok {
		if id, ok := m["BufferID"].(string); ok {
			return id == targetID
		}
		if id, ok := m["buffer_id"].(string); ok {
			return id == targetID
		}
	}

	// Check for struct with BufferID field via interface
	type bufferIDer interface {
		GetBufferID() string
	}
	if b, ok := payload.(bufferIDer); ok {
		return b.GetBufferID() == targetID
	}

	return false
}

// FilterByURI creates a filter for events related to a specific file URI.
// This checks for a URI field in the payload (common in LSP/project events).
func FilterByURI(uri string) FilterFunc {
	return func(event any) bool {
		switch e := event.(type) {
		case Envelope:
			return checkURI(e.Payload, uri)
		default:
			return checkURI(event, uri)
		}
	}
}

// checkURI checks if the payload has a URI field matching the target.
func checkURI(payload any, targetURI string) bool {
	// Check for map[string]any with URI key
	if m, ok := payload.(map[string]any); ok {
		if u, ok := m["URI"].(string); ok {
			return u == targetURI
		}
		if u, ok := m["uri"].(string); ok {
			return u == targetURI
		}
	}

	// Check for struct with URI field via interface
	type uriGetter interface {
		GetURI() string
	}
	if g, ok := payload.(uriGetter); ok {
		return g.GetURI() == targetURI
	}

	return false
}

// FilterBySessionID creates a filter for events related to a specific session.
// This checks for a SessionID field in the payload (common in debug events).
func FilterBySessionID(sessionID string) FilterFunc {
	return func(event any) bool {
		switch e := event.(type) {
		case Envelope:
			return checkSessionID(e.Payload, sessionID)
		default:
			return checkSessionID(event, sessionID)
		}
	}
}

// checkSessionID checks if the payload has a SessionID field matching the target.
func checkSessionID(payload any, targetID string) bool {
	// Check for map[string]any with SessionID key
	if m, ok := payload.(map[string]any); ok {
		if id, ok := m["SessionID"].(string); ok {
			return id == targetID
		}
		if id, ok := m["session_id"].(string); ok {
			return id == targetID
		}
	}

	// Check for struct with SessionID field via interface
	type sessionIDer interface {
		GetSessionID() string
	}
	if s, ok := payload.(sessionIDer); ok {
		return s.GetSessionID() == targetID
	}

	return false
}
