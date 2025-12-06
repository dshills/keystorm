// Package layer provides configuration layer management for Keystorm.
//
// The layer package handles multiple configuration sources with priority-based
// merging. Higher priority layers override values from lower priority layers.
package layer

import (
	"time"
)

// Layer represents a single configuration layer.
type Layer struct {
	// Name identifies the layer (e.g., "user", "workspace", "default").
	Name string

	// Priority determines merge order (higher overrides lower).
	Priority int

	// Source indicates where this layer was loaded from.
	Source Source

	// Path is the file path (if loaded from file).
	Path string

	// Data holds the configuration values as a nested map.
	Data map[string]any

	// ModTime is when the source was last modified.
	ModTime time.Time

	// ReadOnly prevents modifications to this layer.
	ReadOnly bool
}

// NewLayer creates a new configuration layer.
func NewLayer(name string, source Source, priority int) *Layer {
	return &Layer{
		Name:     name,
		Source:   source,
		Priority: priority,
		Data:     make(map[string]any),
		ModTime:  time.Now(),
	}
}

// NewLayerWithData creates a new layer with initial data.
func NewLayerWithData(name string, source Source, priority int, data map[string]any) *Layer {
	return &Layer{
		Name:     name,
		Source:   source,
		Priority: priority,
		Data:     data,
		ModTime:  time.Now(),
	}
}

// Clone creates a deep copy of the layer.
func (l *Layer) Clone() *Layer {
	return &Layer{
		Name:     l.Name,
		Priority: l.Priority,
		Source:   l.Source,
		Path:     l.Path,
		Data:     cloneMap(l.Data),
		ModTime:  l.ModTime,
		ReadOnly: l.ReadOnly,
	}
}

// Source indicates where a configuration layer came from.
type Source uint8

const (
	// SourceBuiltin represents built-in default configuration.
	SourceBuiltin Source = iota
	// SourceUserGlobal represents user global config (~/.config/keystorm/).
	SourceUserGlobal
	// SourceWorkspace represents workspace/project config (.keystorm/).
	SourceWorkspace
	// SourceLanguage represents language-specific overrides.
	SourceLanguage
	// SourceEnv represents environment variables.
	SourceEnv
	// SourceArgs represents command-line arguments.
	SourceArgs
	// SourcePlugin represents plugin-provided configuration.
	SourcePlugin
	// SourceSession represents in-memory session overrides.
	SourceSession
)

// String returns a human-readable name for the source.
func (s Source) String() string {
	switch s {
	case SourceBuiltin:
		return "builtin"
	case SourceUserGlobal:
		return "user"
	case SourceWorkspace:
		return "workspace"
	case SourceLanguage:
		return "language"
	case SourceEnv:
		return "environment"
	case SourceArgs:
		return "arguments"
	case SourcePlugin:
		return "plugin"
	case SourceSession:
		return "session"
	default:
		return "unknown"
	}
}

// cloneMap creates a deep copy of a map.
func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	for key, val := range src {
		switch v := val.(type) {
		case map[string]any:
			dst[key] = cloneMap(v)
		case []any:
			dst[key] = cloneSlice(v)
		default:
			dst[key] = val
		}
	}

	return dst
}

// cloneSlice creates a deep copy of a slice.
func cloneSlice(src []any) []any {
	if src == nil {
		return nil
	}

	dst := make([]any, len(src))
	for i, val := range src {
		switch v := val.(type) {
		case map[string]any:
			dst[i] = cloneMap(v)
		case []any:
			dst[i] = cloneSlice(v)
		default:
			dst[i] = val
		}
	}

	return dst
}
