package plugin

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Manager manages the lifecycle of all plugins.
// It handles discovery, loading, activation, and event dispatching.
type Manager struct {
	mu sync.RWMutex

	// Loader for plugin discovery
	loader *Loader

	// Loaded plugins by name
	plugins map[string]*Host

	// Plugin load order (for deterministic iteration)
	loadOrder []string

	// Event handlers (protected by mu)
	eventHandlers []EventHandler

	// Configuration
	config ManagerConfig
}

// ManagerConfig configures the plugin manager.
type ManagerConfig struct {
	// PluginPaths are directories to search for plugins
	PluginPaths []string

	// AutoActivate plugins on load
	AutoActivate bool

	// ParallelLoad allows loading plugins in parallel (reserved for future use)
	ParallelLoad bool

	// MaxParallel is the maximum number of parallel load operations (reserved for future use)
	MaxParallel int
}

// DefaultManagerConfig returns sensible default configuration.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		PluginPaths:  DefaultPluginPaths(),
		AutoActivate: true,
		ParallelLoad: false,
		MaxParallel:  4,
	}
}

// EventHandler handles plugin manager events.
// Handlers must be non-blocking and should not call back into the Manager
// to avoid deadlocks. Panics in handlers are recovered.
type EventHandler func(event ManagerEvent)

// ManagerEvent represents a plugin manager event.
type ManagerEvent struct {
	Type   ManagerEventType
	Plugin string
	Error  error
}

// ManagerEventType is the type of manager event.
type ManagerEventType int

const (
	// EventPluginLoaded is emitted when a plugin is loaded.
	EventPluginLoaded ManagerEventType = iota
	// EventPluginUnloaded is emitted when a plugin is unloaded.
	EventPluginUnloaded
	// EventPluginActivated is emitted when a plugin is activated.
	EventPluginActivated
	// EventPluginDeactivated is emitted when a plugin is deactivated.
	EventPluginDeactivated
	// EventPluginReloaded is emitted when a plugin is reloaded.
	EventPluginReloaded
	// EventPluginError is emitted when a plugin encounters an error.
	EventPluginError
)

// String returns a string representation of the event type.
func (t ManagerEventType) String() string {
	switch t {
	case EventPluginLoaded:
		return "loaded"
	case EventPluginUnloaded:
		return "unloaded"
	case EventPluginActivated:
		return "activated"
	case EventPluginDeactivated:
		return "deactivated"
	case EventPluginReloaded:
		return "reloaded"
	case EventPluginError:
		return "error"
	default:
		return "unknown"
	}
}

// NewManager creates a new plugin manager.
func NewManager(config ManagerConfig) *Manager {
	return &Manager{
		loader:    NewLoader(WithPaths(config.PluginPaths...)),
		plugins:   make(map[string]*Host),
		loadOrder: make([]string, 0),
		config:    config,
	}
}

// Discover searches for available plugins.
func (m *Manager) Discover() ([]*PluginInfo, error) {
	return m.loader.Discover()
}

// Load loads a plugin by name.
// If the plugin is already loaded, returns ErrAlreadyLoaded.
func (m *Manager) Load(ctx context.Context, name string) (*Host, error) {
	// Check if already loaded (quick check under lock)
	m.mu.Lock()
	if _, exists := m.plugins[name]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("plugin %q: %w", name, ErrAlreadyLoaded)
	}
	m.mu.Unlock()

	// Find the plugin (no lock needed for loader)
	info, err := m.loader.FindPlugin(name)
	if err != nil {
		return nil, err
	}

	// Create host (no lock needed)
	host, err := NewHost(info.Manifest)
	if err != nil {
		return nil, err
	}

	// Load the plugin (potentially long operation, no lock)
	if err := host.Load(ctx); err != nil {
		return nil, fmt.Errorf("failed to load plugin %q: %w", name, err)
	}

	// Register the plugin (brief lock)
	m.mu.Lock()
	// Double-check - another goroutine might have loaded it
	if _, exists := m.plugins[name]; exists {
		m.mu.Unlock()
		host.Unload(ctx) // Clean up
		return nil, fmt.Errorf("plugin %q: %w", name, ErrAlreadyLoaded)
	}
	m.plugins[name] = host
	m.loadOrder = append(m.loadOrder, name)
	m.mu.Unlock()

	// Emit event (outside lock)
	m.emitEvent(ManagerEvent{Type: EventPluginLoaded, Plugin: name})

	// Auto-activate if configured (outside lock)
	if m.config.AutoActivate {
		if err := host.Activate(ctx); err != nil {
			m.emitEvent(ManagerEvent{Type: EventPluginError, Plugin: name, Error: err})
			// Continue - plugin is loaded but not activated
		} else {
			m.emitEvent(ManagerEvent{Type: EventPluginActivated, Plugin: name})
		}
	}

	return host, nil
}

// LoadAll loads all discovered plugins.
func (m *Manager) LoadAll(ctx context.Context) error {
	plugins, err := m.loader.Discover()
	if err != nil {
		return err
	}

	var loadErrors []error
	for _, info := range plugins {
		if _, err := m.Load(ctx, info.Name); err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("%s: %w", info.Name, err))
		}
	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load %d plugins: %w", len(loadErrors), errors.Join(loadErrors...))
	}
	return nil
}

// Unload unloads a plugin by name.
func (m *Manager) Unload(ctx context.Context, name string) error {
	// Get and remove the plugin from registry (brief lock)
	m.mu.Lock()
	host, exists := m.plugins[name]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("plugin %q: %w", name, ErrPluginNotFound)
	}
	delete(m.plugins, name)
	m.removeFromLoadOrder(name)
	m.mu.Unlock()

	// Deactivate if active (outside lock)
	if host.State() == StateActive {
		if err := host.Deactivate(ctx); err != nil {
			m.emitEvent(ManagerEvent{Type: EventPluginError, Plugin: name, Error: err})
		} else {
			m.emitEvent(ManagerEvent{Type: EventPluginDeactivated, Plugin: name})
		}
	}

	// Unload (potentially long operation, outside lock)
	if err := host.Unload(ctx); err != nil {
		return fmt.Errorf("failed to unload plugin %q: %w", name, err)
	}

	m.emitEvent(ManagerEvent{Type: EventPluginUnloaded, Plugin: name})
	return nil
}

// UnloadAll unloads all plugins in reverse load order.
func (m *Manager) UnloadAll(ctx context.Context) error {
	// Get names in reverse load order (brief lock)
	m.mu.RLock()
	names := make([]string, len(m.loadOrder))
	for i, name := range m.loadOrder {
		names[len(m.loadOrder)-1-i] = name
	}
	m.mu.RUnlock()

	var unloadErrors []error
	for _, name := range names {
		if err := m.Unload(ctx, name); err != nil {
			unloadErrors = append(unloadErrors, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(unloadErrors) > 0 {
		return fmt.Errorf("failed to unload %d plugins: %w", len(unloadErrors), errors.Join(unloadErrors...))
	}
	return nil
}

// Get returns a plugin by name.
func (m *Manager) Get(name string) (*Host, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	host, exists := m.plugins[name]
	return host, exists
}

// List returns all loaded plugins.
func (m *Manager) List() []*Host {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Host, 0, len(m.loadOrder))
	for _, name := range m.loadOrder {
		if host, exists := m.plugins[name]; exists {
			result = append(result, host)
		}
	}
	return result
}

// ListActive returns all active plugins.
func (m *Manager) ListActive() []*Host {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Host, 0)
	for _, name := range m.loadOrder {
		if host, exists := m.plugins[name]; exists {
			if host.State() == StateActive {
				result = append(result, host)
			}
		}
	}
	return result
}

// ListByState returns plugins in a specific state.
func (m *Manager) ListByState(state State) []*Host {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Host, 0)
	for _, name := range m.loadOrder {
		if host, exists := m.plugins[name]; exists {
			if host.State() == state {
				result = append(result, host)
			}
		}
	}
	return result
}

// Activate activates a loaded plugin.
func (m *Manager) Activate(ctx context.Context, name string) error {
	// Get the host (brief lock)
	m.mu.RLock()
	host, exists := m.plugins[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %q: %w", name, ErrPluginNotFound)
	}

	// Activate (potentially long operation, outside lock)
	if err := host.Activate(ctx); err != nil {
		m.emitEvent(ManagerEvent{Type: EventPluginError, Plugin: name, Error: err})
		return err
	}

	m.emitEvent(ManagerEvent{Type: EventPluginActivated, Plugin: name})
	return nil
}

// ActivateAll activates all loaded plugins.
func (m *Manager) ActivateAll(ctx context.Context) error {
	// Get names (brief lock)
	m.mu.RLock()
	names := make([]string, len(m.loadOrder))
	copy(names, m.loadOrder)
	m.mu.RUnlock()

	var activateErrors []error
	for _, name := range names {
		if err := m.Activate(ctx, name); err != nil {
			activateErrors = append(activateErrors, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(activateErrors) > 0 {
		return fmt.Errorf("failed to activate %d plugins: %w", len(activateErrors), errors.Join(activateErrors...))
	}
	return nil
}

// Deactivate deactivates an active plugin.
func (m *Manager) Deactivate(ctx context.Context, name string) error {
	// Get the host (brief lock)
	m.mu.RLock()
	host, exists := m.plugins[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %q: %w", name, ErrPluginNotFound)
	}

	// Deactivate (potentially long operation, outside lock)
	if err := host.Deactivate(ctx); err != nil {
		m.emitEvent(ManagerEvent{Type: EventPluginError, Plugin: name, Error: err})
		return err
	}

	m.emitEvent(ManagerEvent{Type: EventPluginDeactivated, Plugin: name})
	return nil
}

// DeactivateAll deactivates all plugins in reverse load order.
func (m *Manager) DeactivateAll(ctx context.Context) error {
	// Get names in reverse load order (brief lock)
	m.mu.RLock()
	names := make([]string, len(m.loadOrder))
	for i, name := range m.loadOrder {
		names[len(m.loadOrder)-1-i] = name
	}
	m.mu.RUnlock()

	var deactivateErrors []error
	for _, name := range names {
		if err := m.Deactivate(ctx, name); err != nil {
			deactivateErrors = append(deactivateErrors, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(deactivateErrors) > 0 {
		return fmt.Errorf("failed to deactivate %d plugins: %w", len(deactivateErrors), errors.Join(deactivateErrors...))
	}
	return nil
}

// Reload reloads a plugin (unload + load).
func (m *Manager) Reload(ctx context.Context, name string) error {
	// Check if plugin exists and get its state (brief lock)
	m.mu.RLock()
	host, exists := m.plugins[name]
	if !exists {
		m.mu.RUnlock()
		return fmt.Errorf("plugin %q: %w", name, ErrPluginNotFound)
	}
	wasActive := host.State() == StateActive
	m.mu.RUnlock()

	// Unload (outside lock)
	if err := m.Unload(ctx, name); err != nil {
		return fmt.Errorf("reload unload failed: %w", err)
	}

	// Refresh discovery to pick up any changes (outside lock)
	if _, err := m.loader.Refresh(); err != nil {
		return fmt.Errorf("reload refresh failed: %w", err)
	}

	// Load again (outside lock)
	newHost, err := m.Load(ctx, name)
	if err != nil {
		return fmt.Errorf("reload load failed: %w", err)
	}

	// Restore active state if it was active and auto-activate is off
	if wasActive && !m.config.AutoActivate {
		if err := newHost.Activate(ctx); err != nil {
			m.emitEvent(ManagerEvent{Type: EventPluginError, Plugin: name, Error: err})
		}
	}

	m.emitEvent(ManagerEvent{Type: EventPluginReloaded, Plugin: name})
	return nil
}

// Subscribe adds an event handler.
// Returns an unsubscribe function to remove the handler.
func (m *Manager) Subscribe(handler EventHandler) func() {
	if handler == nil {
		return func() {} // No-op for nil handlers
	}

	m.mu.Lock()
	m.eventHandlers = append(m.eventHandlers, handler)
	index := len(m.eventHandlers) - 1
	m.mu.Unlock()

	// Return unsubscribe function
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		// Set to nil instead of removing to avoid index shifting issues
		if index < len(m.eventHandlers) {
			m.eventHandlers[index] = nil
		}
	}
}

// Count returns the number of loaded plugins.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}

// CountActive returns the number of active plugins.
func (m *Manager) CountActive() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, host := range m.plugins {
		if host.State() == StateActive {
			count++
		}
	}
	return count
}

// HasErrors returns true if any plugin is in an error state.
func (m *Manager) HasErrors() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, host := range m.plugins {
		if host.State() == StateError {
			return true
		}
	}
	return false
}

// Errors returns all plugins in error state with their errors.
func (m *Manager) Errors() map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	errs := make(map[string]error)
	for name, host := range m.plugins {
		if host.State() == StateError && host.Error() != nil {
			errs[name] = host.Error()
		}
	}
	return errs
}

// Loader returns the underlying loader for advanced operations.
func (m *Manager) Loader() *Loader {
	return m.loader
}

// emitEvent sends an event to all handlers.
// Handlers are called outside any locks and panics are recovered.
func (m *Manager) emitEvent(event ManagerEvent) {
	// Copy handlers under lock
	m.mu.RLock()
	handlers := make([]EventHandler, len(m.eventHandlers))
	copy(handlers, m.eventHandlers)
	m.mu.RUnlock()

	// Call handlers outside lock with panic recovery
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		func() {
			defer func() {
				recover() // Ignore panics from handlers
			}()
			handler(event)
		}()
	}
}

// removeFromLoadOrder removes a name from the load order slice.
// Must be called with mu held.
func (m *Manager) removeFromLoadOrder(name string) {
	for i, n := range m.loadOrder {
		if n == name {
			m.loadOrder = append(m.loadOrder[:i], m.loadOrder[i+1:]...)
			return
		}
	}
}
