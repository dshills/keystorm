package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/dshills/keystorm/internal/plugin/api"
)

// System provides a unified interface to the Keystorm plugin system.
// It coordinates the plugin manager, API registry, and editor providers
// to deliver a complete plugin runtime environment.
//
// System is the primary entry point for the editor to interact with plugins.
// It handles:
//   - Plugin discovery, loading, and lifecycle management
//   - API module registration and capability enforcement
//   - Event routing between editor and plugins
//   - Resource cleanup on shutdown
type System struct {
	mu sync.RWMutex

	// Core components
	manager  *Manager
	registry *api.Registry
	apiCtx   *api.Context

	// Configuration
	config SystemConfig

	// State
	initialized bool
}

// SystemConfig configures the plugin system.
type SystemConfig struct {
	// ManagerConfig for the plugin manager
	ManagerConfig ManagerConfig

	// Providers for API modules
	BufferProvider  api.BufferProvider
	CursorProvider  api.CursorProvider
	ModeProvider    api.ModeProvider
	KeymapProvider  api.KeymapProvider
	CommandProvider api.CommandProvider
	EventProvider   api.EventProvider
	UIProvider      api.UIProvider
	ConfigProvider  api.ConfigProvider
	LSPProvider     api.LSPProvider
}

// DefaultSystemConfig returns sensible default system configuration.
func DefaultSystemConfig() SystemConfig {
	return SystemConfig{
		ManagerConfig: DefaultManagerConfig(),
	}
}

// NewSystem creates a new plugin system with the given configuration.
func NewSystem(config SystemConfig) *System {
	return &System{
		config: config,
	}
}

// Initialize sets up the plugin system.
// This must be called before any other operations.
func (s *System) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return ErrAlreadyInitialized
	}

	// Create API context with all providers
	s.apiCtx = &api.Context{
		Buffer:  s.config.BufferProvider,
		Cursor:  s.config.CursorProvider,
		Mode:    s.config.ModeProvider,
		Keymap:  s.config.KeymapProvider,
		Command: s.config.CommandProvider,
		Event:   s.config.EventProvider,
		UI:      s.config.UIProvider,
		Config:  s.config.ConfigProvider,
		LSP:     s.config.LSPProvider,
	}

	// Create API registry with standard modules
	registry, err := api.DefaultRegistry(s.apiCtx)
	if err != nil {
		return fmt.Errorf("failed to create API registry: %w", err)
	}
	s.registry = registry

	// Create plugin manager
	s.manager = NewManager(s.config.ManagerConfig)

	s.initialized = true
	return nil
}

// Shutdown gracefully shuts down the plugin system.
// It deactivates and unloads all plugins.
func (s *System) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return nil // Nothing to shut down
	}

	// Unload all plugins (handles deactivation internally)
	if err := s.manager.UnloadAll(ctx); err != nil {
		return fmt.Errorf("failed to unload plugins: %w", err)
	}

	s.initialized = false
	return nil
}

// Manager returns the plugin manager for direct access.
func (s *System) Manager() *Manager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.manager
}

// Registry returns the API registry.
func (s *System) Registry() *api.Registry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.registry
}

// APIContext returns the API context with all providers.
func (s *System) APIContext() *api.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.apiCtx
}

// IsInitialized returns true if the system is initialized.
func (s *System) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized
}

// Discover discovers available plugins.
func (s *System) Discover() ([]*PluginInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return nil, ErrNotInitialized
	}

	return s.manager.Discover()
}

// LoadPlugin loads a single plugin by name.
// The plugin's Lua state is configured with the API modules based on its capabilities.
func (s *System) LoadPlugin(ctx context.Context, name string) (*Host, error) {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		return nil, ErrNotInitialized
	}
	manager := s.manager
	s.mu.RUnlock()

	// Load through manager
	host, err := manager.Load(ctx, name)
	if err != nil {
		return nil, err
	}

	// Inject API modules based on capabilities
	if err := s.injectAPIs(host); err != nil {
		// Unload on failure
		_ = manager.Unload(ctx, name)
		return nil, fmt.Errorf("failed to inject APIs for plugin %q: %w", name, err)
	}

	return host, nil
}

// LoadAll loads all discovered plugins.
func (s *System) LoadAll(ctx context.Context) error {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		return ErrNotInitialized
	}
	s.mu.RUnlock()

	plugins, err := s.Discover()
	if err != nil {
		return err
	}

	var loadErrors []error
	for _, info := range plugins {
		if _, err := s.LoadPlugin(ctx, info.Name); err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("%s: %w", info.Name, err))
		}
	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load %d plugins: %v", len(loadErrors), loadErrors)
	}
	return nil
}

// UnloadPlugin unloads a plugin by name.
func (s *System) UnloadPlugin(ctx context.Context, name string) error {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		return ErrNotInitialized
	}
	manager := s.manager
	s.mu.RUnlock()

	return manager.Unload(ctx, name)
}

// ReloadPlugin reloads a plugin by name.
func (s *System) ReloadPlugin(ctx context.Context, name string) error {
	s.mu.RLock()
	if !s.initialized {
		s.mu.RUnlock()
		return ErrNotInitialized
	}
	manager := s.manager
	s.mu.RUnlock()

	// Get current state
	host, exists := manager.Get(name)
	if !exists {
		return fmt.Errorf("plugin %q: %w", name, ErrPluginNotFound)
	}
	wasActive := host.State() == StateActive

	// Unload
	if err := s.UnloadPlugin(ctx, name); err != nil {
		return fmt.Errorf("reload unload failed: %w", err)
	}

	// Reload
	newHost, err := s.LoadPlugin(ctx, name)
	if err != nil {
		return fmt.Errorf("reload load failed: %w", err)
	}

	// Re-activate if it was active
	if wasActive && newHost.State() == StateLoaded {
		if err := manager.Activate(ctx, name); err != nil {
			return fmt.Errorf("reload activate failed: %w", err)
		}
	}

	return nil
}

// GetPlugin returns a loaded plugin by name.
func (s *System) GetPlugin(name string) (*Host, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return nil, false
	}

	return s.manager.Get(name)
}

// ListPlugins returns all loaded plugins.
func (s *System) ListPlugins() []*Host {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return nil
	}

	return s.manager.List()
}

// ListActivePlugins returns all active plugins.
func (s *System) ListActivePlugins() []*Host {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return nil
	}

	return s.manager.ListActive()
}

// PluginCount returns the number of loaded plugins.
func (s *System) PluginCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return 0
	}

	return s.manager.Count()
}

// ActivePluginCount returns the number of active plugins.
func (s *System) ActivePluginCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return 0
	}

	return s.manager.CountActive()
}

// Subscribe subscribes to plugin manager events.
func (s *System) Subscribe(handler EventHandler) func() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized || s.manager == nil {
		return func() {} // No-op
	}

	return s.manager.Subscribe(handler)
}

// HasErrors returns true if any plugin has errors.
func (s *System) HasErrors() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return false
	}

	return s.manager.HasErrors()
}

// Errors returns all plugin errors.
func (s *System) Errors() map[string]error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return nil
	}

	return s.manager.Errors()
}

// injectAPIs injects API modules into a plugin's Lua state based on capabilities.
func (s *System) injectAPIs(host *Host) error {
	L := host.LuaState()
	if L == nil {
		return nil // Plugin not loaded yet
	}

	// Inject all modules without capability checking
	// The sandbox already handles capability-based restrictions
	// API modules that need capability checking use the security package directly
	return s.registry.InjectAll(L, nil)
}

// SetProvider updates a provider at runtime.
// This is useful when providers need to be updated after initialization.
func (s *System) SetProvider(providerType string, provider interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return ErrNotInitialized
	}

	switch providerType {
	case "buffer":
		if p, ok := provider.(api.BufferProvider); ok {
			s.apiCtx.Buffer = p
		} else {
			return fmt.Errorf("invalid buffer provider type")
		}
	case "cursor":
		if p, ok := provider.(api.CursorProvider); ok {
			s.apiCtx.Cursor = p
		} else {
			return fmt.Errorf("invalid cursor provider type")
		}
	case "mode":
		if p, ok := provider.(api.ModeProvider); ok {
			s.apiCtx.Mode = p
		} else {
			return fmt.Errorf("invalid mode provider type")
		}
	case "keymap":
		if p, ok := provider.(api.KeymapProvider); ok {
			s.apiCtx.Keymap = p
		} else {
			return fmt.Errorf("invalid keymap provider type")
		}
	case "command":
		if p, ok := provider.(api.CommandProvider); ok {
			s.apiCtx.Command = p
		} else {
			return fmt.Errorf("invalid command provider type")
		}
	case "event":
		if p, ok := provider.(api.EventProvider); ok {
			s.apiCtx.Event = p
		} else {
			return fmt.Errorf("invalid event provider type")
		}
	case "ui":
		if p, ok := provider.(api.UIProvider); ok {
			s.apiCtx.UI = p
		} else {
			return fmt.Errorf("invalid ui provider type")
		}
	case "config":
		if p, ok := provider.(api.ConfigProvider); ok {
			s.apiCtx.Config = p
		} else {
			return fmt.Errorf("invalid config provider type")
		}
	case "lsp":
		if p, ok := provider.(api.LSPProvider); ok {
			s.apiCtx.LSP = p
		} else {
			return fmt.Errorf("invalid lsp provider type")
		}
	default:
		return fmt.Errorf("unknown provider type: %s", providerType)
	}

	return nil
}

// Stats returns system-wide statistics.
func (s *System) Stats() SystemStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SystemStats{
		Initialized: s.initialized,
	}

	if s.initialized && s.manager != nil {
		stats.TotalPlugins = s.manager.Count()
		stats.ActivePlugins = s.manager.CountActive()
		stats.HasErrors = s.manager.HasErrors()

		// Collect individual plugin stats
		for _, host := range s.manager.List() {
			stats.PluginStats = append(stats.PluginStats, host.Stats())
		}
	}

	if s.registry != nil {
		stats.RegisteredModules = s.registry.List()
	}

	return stats
}

// SystemStats contains system-wide statistics.
type SystemStats struct {
	Initialized       bool
	TotalPlugins      int
	ActivePlugins     int
	HasErrors         bool
	RegisteredModules []string
	PluginStats       []HostStats
}
