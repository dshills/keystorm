package event

import (
	"testing"
)

func TestFilterBySource(t *testing.T) {
	filter := FilterBySource("test-source")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "matching source via envelope",
			event: Envelope{
				Topic:    "test.event",
				Metadata: Metadata{Source: "test-source"},
			},
			want: true,
		},
		{
			name: "non-matching source via envelope",
			event: Envelope{
				Topic:    "test.event",
				Metadata: Metadata{Source: "other-source"},
			},
			want: false,
		},
		{
			name:  "matching source via typed event",
			event: NewEvent("test.event", "payload", "test-source"),
			want:  true,
		},
		{
			name:  "non-matching source via typed event",
			event: NewEvent("test.event", "payload", "other-source"),
			want:  false,
		},
		{
			name:  "unknown event type",
			event: "just a string",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterBySource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterBySourcePrefix(t *testing.T) {
	filter := FilterBySourcePrefix("plugin.")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "matching prefix",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "plugin.custom"},
			},
			want: true,
		},
		{
			name: "non-matching prefix",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "core.engine"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterBySourcePrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterBySources(t *testing.T) {
	filter := FilterBySources("buffer", "cursor", "input")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "first allowed source",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "buffer"},
			},
			want: true,
		},
		{
			name: "second allowed source",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "cursor"},
			},
			want: true,
		},
		{
			name: "disallowed source",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "other"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterBySources() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterExcludeSource(t *testing.T) {
	filter := FilterExcludeSource("metrics")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "excluded source",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "metrics"},
			},
			want: false,
		},
		{
			name: "allowed source",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "buffer"},
			},
			want: true,
		},
		{
			name:  "unknown event type",
			event: "just a string",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterExcludeSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterByTopic(t *testing.T) {
	filter := FilterByTopic("buffer.**")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name:  "matching topic multi-segment",
			event: Envelope{Topic: "buffer.content.inserted"},
			want:  true,
		},
		{
			name:  "matching topic single-segment",
			event: Envelope{Topic: "buffer.saved"},
			want:  true,
		},
		{
			name:  "non-matching topic",
			event: Envelope{Topic: "cursor.moved"},
			want:  false,
		},
		{
			name:  "typed event matching",
			event: NewEvent[string]("buffer.saved", "data", "test"),
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterByTopic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterByTopicPrefix(t *testing.T) {
	filter := FilterByTopicPrefix("lsp.")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name:  "matching prefix",
			event: Envelope{Topic: "lsp.server.initialized"},
			want:  true,
		},
		{
			name:  "non-matching prefix",
			event: Envelope{Topic: "buffer.changed"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterByTopicPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterExcludeTopic(t *testing.T) {
	filter := FilterExcludeTopic("debug.**")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name:  "excluded topic multi-segment",
			event: Envelope{Topic: "debug.session.started"},
			want:  false,
		},
		{
			name:  "excluded topic single-segment",
			event: Envelope{Topic: "debug.started"},
			want:  false,
		},
		{
			name:  "allowed topic",
			event: Envelope{Topic: "buffer.changed"},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterExcludeTopic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterByCorrelation(t *testing.T) {
	filter := FilterByCorrelation("corr-123")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "matching correlation",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{CorrelationID: "corr-123"},
			},
			want: true,
		},
		{
			name: "non-matching correlation",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{CorrelationID: "other"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterByCorrelation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterByCausation(t *testing.T) {
	filter := FilterByCausation("cause-456")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "matching causation",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{CausationID: "cause-456"},
			},
			want: true,
		},
		{
			name: "non-matching causation",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{CausationID: "other"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterByCausation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterPayload(t *testing.T) {
	type TestPayload struct {
		Value int
	}

	filter := FilterPayload(func(p TestPayload) bool {
		return p.Value > 10
	})

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name:  "typed event passing filter",
			event: NewEvent("test", TestPayload{Value: 20}, "source"),
			want:  true,
		},
		{
			name:  "typed event failing filter",
			event: NewEvent("test", TestPayload{Value: 5}, "source"),
			want:  false,
		},
		{
			name: "envelope with typed payload passing",
			event: Envelope{
				Topic:   "test",
				Payload: TestPayload{Value: 15},
			},
			want: true,
		},
		{
			name:  "wrong payload type",
			event: NewEvent("test", "string payload", "source"),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterPayload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterAnd(t *testing.T) {
	filter := FilterAnd(
		FilterBySource("test"),
		FilterByTopicPrefix("buffer."),
	)

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "both conditions pass",
			event: Envelope{
				Topic:    "buffer.changed",
				Metadata: Metadata{Source: "test"},
			},
			want: true,
		},
		{
			name: "only source passes",
			event: Envelope{
				Topic:    "cursor.moved",
				Metadata: Metadata{Source: "test"},
			},
			want: false,
		},
		{
			name: "only topic passes",
			event: Envelope{
				Topic:    "buffer.changed",
				Metadata: Metadata{Source: "other"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterAnd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterOr(t *testing.T) {
	filter := FilterOr(
		FilterBySource("buffer"),
		FilterBySource("cursor"),
	)

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "first condition passes",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "buffer"},
			},
			want: true,
		},
		{
			name: "second condition passes",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "cursor"},
			},
			want: true,
		},
		{
			name: "neither passes",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "other"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterOr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterNot(t *testing.T) {
	filter := FilterNot(FilterBySource("excluded"))

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "excluded source",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "excluded"},
			},
			want: false,
		},
		{
			name: "other source",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Source: "allowed"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterNot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterAll(t *testing.T) {
	filter := FilterAll()

	events := []any{
		"string",
		123,
		Envelope{Topic: "test"},
		NewEvent("test", "data", "source"),
	}

	for i, event := range events {
		if !filter(event) {
			t.Errorf("FilterAll() for event %d = false, want true", i)
		}
	}
}

func TestFilterNone(t *testing.T) {
	filter := FilterNone()

	events := []any{
		"string",
		123,
		Envelope{Topic: "test"},
		NewEvent("test", "data", "source"),
	}

	for i, event := range events {
		if filter(event) {
			t.Errorf("FilterNone() for event %d = true, want false", i)
		}
	}
}

func TestFilterByVersion(t *testing.T) {
	filter := FilterByVersion(2)

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "matching version",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Version: 2},
			},
			want: true,
		},
		{
			name: "non-matching version",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Version: 1},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterByVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterByMinVersion(t *testing.T) {
	filter := FilterByMinVersion(2)

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "version equal to min",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Version: 2},
			},
			want: true,
		},
		{
			name: "version above min",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Version: 3},
			},
			want: true,
		},
		{
			name: "version below min",
			event: Envelope{
				Topic:    "test",
				Metadata: Metadata{Version: 1},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterByMinVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterByBufferID(t *testing.T) {
	filter := FilterByBufferID("buffer-123")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "matching BufferID in map",
			event: Envelope{
				Topic:   "test",
				Payload: map[string]any{"BufferID": "buffer-123"},
			},
			want: true,
		},
		{
			name: "matching buffer_id in map",
			event: Envelope{
				Topic:   "test",
				Payload: map[string]any{"buffer_id": "buffer-123"},
			},
			want: true,
		},
		{
			name: "non-matching BufferID",
			event: Envelope{
				Topic:   "test",
				Payload: map[string]any{"BufferID": "other"},
			},
			want: false,
		},
		{
			name: "no BufferID field",
			event: Envelope{
				Topic:   "test",
				Payload: map[string]any{"key": "value"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterByBufferID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterByURI(t *testing.T) {
	filter := FilterByURI("file:///path/to/file.go")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "matching URI in map",
			event: Envelope{
				Topic:   "test",
				Payload: map[string]any{"URI": "file:///path/to/file.go"},
			},
			want: true,
		},
		{
			name: "matching uri in map",
			event: Envelope{
				Topic:   "test",
				Payload: map[string]any{"uri": "file:///path/to/file.go"},
			},
			want: true,
		},
		{
			name: "non-matching URI",
			event: Envelope{
				Topic:   "test",
				Payload: map[string]any{"URI": "file:///other/file.go"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterByURI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterBySessionID(t *testing.T) {
	filter := FilterBySessionID("session-789")

	tests := []struct {
		name  string
		event any
		want  bool
	}{
		{
			name: "matching SessionID in map",
			event: Envelope{
				Topic:   "test",
				Payload: map[string]any{"SessionID": "session-789"},
			},
			want: true,
		},
		{
			name: "non-matching SessionID",
			event: Envelope{
				Topic:   "test",
				Payload: map[string]any{"SessionID": "other"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filter(tt.event); got != tt.want {
				t.Errorf("FilterBySessionID() = %v, want %v", got, tt.want)
			}
		})
	}
}
