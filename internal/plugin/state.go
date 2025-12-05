package plugin

// State represents the lifecycle state of a plugin.
type State int

// Plugin states.
const (
	// StateUnloaded - Plugin is not loaded.
	StateUnloaded State = iota

	// StateLoaded - Plugin code is loaded but not activated.
	StateLoaded

	// StateActivating - Plugin is being activated.
	StateActivating

	// StateActive - Plugin is active and running.
	StateActive

	// StateDeactivating - Plugin is being deactivated.
	StateDeactivating

	// StateError - Plugin encountered an error.
	StateError
)

// String returns a string representation of the state.
func (s State) String() string {
	switch s {
	case StateUnloaded:
		return "unloaded"
	case StateLoaded:
		return "loaded"
	case StateActivating:
		return "activating"
	case StateActive:
		return "active"
	case StateDeactivating:
		return "deactivating"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// IsUsable returns true if the plugin can be used (loaded or active).
func (s State) IsUsable() bool {
	return s == StateLoaded || s == StateActive
}
