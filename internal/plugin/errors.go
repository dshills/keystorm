package plugin

import "errors"

// Plugin system errors.
var (
	// ErrPluginNotFound is returned when a plugin cannot be located.
	ErrPluginNotFound = errors.New("plugin not found")

	// ErrNoEntryPoint is returned when a plugin has no valid entry point.
	ErrNoEntryPoint = errors.New("plugin has no entry point (init.lua or plugin.lua)")

	// ErrNilManifest is returned when a nil manifest is provided.
	ErrNilManifest = errors.New("manifest is nil")

	// ErrAlreadyLoaded is returned when attempting to load an already loaded plugin.
	ErrAlreadyLoaded = errors.New("plugin is already loaded")

	// ErrNotLoaded is returned when attempting to use an unloaded plugin.
	ErrNotLoaded = errors.New("plugin is not loaded")

	// ErrPluginDisabled is returned when attempting to use a disabled plugin.
	ErrPluginDisabled = errors.New("plugin is disabled")

	// ErrDependencyNotFound is returned when a required dependency is missing.
	ErrDependencyNotFound = errors.New("plugin dependency not found")

	// ErrCyclicDependency is returned when plugins have circular dependencies.
	ErrCyclicDependency = errors.New("cyclic plugin dependency detected")

	// ErrCapabilityDenied is returned when a plugin lacks required capability.
	ErrCapabilityDenied = errors.New("capability denied")

	// ErrInvalidPlugin is returned when plugin validation fails.
	ErrInvalidPlugin = errors.New("invalid plugin")
)
