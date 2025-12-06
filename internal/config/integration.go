package config

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dshills/keystorm/internal/config/notify"
)

// ErrSystemClosed is returned when operations are attempted on a closed ConfigSystem.
var ErrSystemClosed = errors.New("config system is closed")

// ConfigSystem provides a high-level facade for the configuration system.
// It wraps Config with additional functionality for system integration.
//
// Thread Safety:
// ConfigSystem is safe for concurrent use. All methods are thread-safe.
type ConfigSystem struct {
	mu     sync.RWMutex
	config *Config
	closed atomic.Bool

	// Metrics for performance monitoring (protected by mu)
	loadTime     time.Duration
	lastReloadAt time.Time
}

// SystemOption configures a ConfigSystem instance.
type SystemOption func(*systemOptions)

type systemOptions struct {
	userConfigDir    string
	projectConfigDir string
	enableWatcher    bool
	enableSchema     bool
}

// WithSystemUserConfigDir sets the user configuration directory.
func WithSystemUserConfigDir(dir string) SystemOption {
	return func(o *systemOptions) {
		o.userConfigDir = dir
	}
}

// WithSystemProjectConfigDir sets the project configuration directory.
func WithSystemProjectConfigDir(dir string) SystemOption {
	return func(o *systemOptions) {
		o.projectConfigDir = dir
	}
}

// WithSystemWatcher enables or disables file watching.
func WithSystemWatcher(enable bool) SystemOption {
	return func(o *systemOptions) {
		o.enableWatcher = enable
	}
}

// WithSystemSchemaValidation enables or disables schema validation.
func WithSystemSchemaValidation(enable bool) SystemOption {
	return func(o *systemOptions) {
		o.enableSchema = enable
	}
}

// NewConfigSystem creates and initializes a new ConfigSystem.
// It loads configuration from all sources and starts file watching if enabled.
func NewConfigSystem(ctx context.Context, opts ...SystemOption) (*ConfigSystem, error) {
	// Apply options
	options := &systemOptions{
		enableWatcher: true,
		enableSchema:  true,
	}
	for _, opt := range opts {
		opt(options)
	}

	// Build Config options
	var configOpts []Option
	if options.userConfigDir != "" {
		configOpts = append(configOpts, WithUserConfigDir(options.userConfigDir))
	}
	if options.projectConfigDir != "" {
		configOpts = append(configOpts, WithProjectConfigDir(options.projectConfigDir))
	}
	configOpts = append(configOpts, WithWatcher(options.enableWatcher))
	configOpts = append(configOpts, WithSchemaValidation(options.enableSchema))

	// Create config
	cfg := New(configOpts...)

	// Load configuration
	start := time.Now()
	if err := cfg.Load(ctx); err != nil {
		cfg.Close()
		return nil, fmt.Errorf("loading configuration: %w", err)
	}
	loadTime := time.Since(start)
	now := time.Now()

	sys := &ConfigSystem{
		config: cfg,
	}
	// Initialize timing fields under the mutex for memory safety
	sys.mu.Lock()
	sys.loadTime = loadTime
	sys.lastReloadAt = now
	sys.mu.Unlock()

	return sys, nil
}

// Close shuts down the configuration system and releases resources.
// It is safe to call Close multiple times.
func (s *ConfigSystem) Close() {
	if s.closed.Swap(true) {
		return // Already closed
	}
	if s.config != nil {
		s.config.Close()
	}
}

// Config returns the underlying Config instance for advanced usage.
// The caller must not call Close() on the returned Config or modify its lifecycle.
// Returns nil if the ConfigSystem has been closed.
func (s *ConfigSystem) Config() *Config {
	if s.closed.Load() {
		return nil
	}
	return s.config
}

// LoadTime returns the duration of the initial configuration load.
func (s *ConfigSystem) LoadTime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadTime
}

// LastReloadAt returns the time of the last configuration reload.
func (s *ConfigSystem) LastReloadAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastReloadAt
}

// Reload reloads configuration from all sources.
// Returns ErrSystemClosed if the system has been closed.
func (s *ConfigSystem) Reload(ctx context.Context) error {
	if s.closed.Load() {
		return ErrSystemClosed
	}
	start := time.Now()
	if err := s.config.Load(ctx); err != nil {
		return fmt.Errorf("reloading configuration: %w", err)
	}

	s.mu.Lock()
	s.loadTime = time.Since(start)
	s.lastReloadAt = time.Now()
	s.mu.Unlock()

	return nil
}

// Get returns a value at the given path.
func (s *ConfigSystem) Get(path string) (any, bool) {
	return s.config.Get(path)
}

// GetString returns a string value at the given path.
func (s *ConfigSystem) GetString(path string) (string, error) {
	return s.config.GetString(path)
}

// GetInt returns an integer value at the given path.
func (s *ConfigSystem) GetInt(path string) (int, error) {
	return s.config.GetInt(path)
}

// GetBool returns a boolean value at the given path.
func (s *ConfigSystem) GetBool(path string) (bool, error) {
	return s.config.GetBool(path)
}

// GetFloat returns a float64 value at the given path.
func (s *ConfigSystem) GetFloat(path string) (float64, error) {
	return s.config.GetFloat(path)
}

// GetStringSlice returns a string slice at the given path.
func (s *ConfigSystem) GetStringSlice(path string) ([]string, error) {
	return s.config.GetStringSlice(path)
}

// Set sets a value at the given path in the user settings layer.
// Returns ErrSystemClosed if the system has been closed.
func (s *ConfigSystem) Set(path string, value any) error {
	if s.closed.Load() {
		return ErrSystemClosed
	}
	return s.config.Set(path, value)
}

// Subscribe registers an observer for all configuration changes.
// Returns nil if the system has been closed.
func (s *ConfigSystem) Subscribe(observer notify.Observer) *notify.Subscription {
	if s.closed.Load() {
		return nil
	}
	return s.config.Subscribe(observer)
}

// SubscribePath registers an observer for changes to a specific path.
// Returns nil if the system has been closed.
func (s *ConfigSystem) SubscribePath(path string, observer notify.Observer) *notify.Subscription {
	if s.closed.Load() {
		return nil
	}
	return s.config.SubscribePath(path, observer)
}

// Merged returns the fully merged configuration.
func (s *ConfigSystem) Merged() map[string]any {
	return s.config.Merged()
}

// Editor returns type-safe access to editor settings.
func (s *ConfigSystem) Editor() EditorConfig {
	return s.config.Editor()
}

// UI returns type-safe access to UI settings.
func (s *ConfigSystem) UI() UIConfig {
	return s.config.UI()
}

// Vim returns type-safe access to Vim mode settings.
func (s *ConfigSystem) Vim() VimConfig {
	return s.config.Vim()
}

// Input returns type-safe access to input settings.
func (s *ConfigSystem) Input() InputConfig {
	return s.config.Input()
}

// Files returns type-safe access to file settings.
func (s *ConfigSystem) Files() FilesConfig {
	return s.config.Files()
}

// Search returns type-safe access to search settings.
func (s *ConfigSystem) Search() SearchConfig {
	return s.config.Search()
}

// AI returns type-safe access to AI settings.
func (s *ConfigSystem) AI() AIConfig {
	return s.config.AI()
}

// Logging returns type-safe access to logging settings.
func (s *ConfigSystem) Logging() LoggingConfig {
	return s.config.Logging()
}

// Terminal returns type-safe access to integrated terminal settings.
func (s *ConfigSystem) Terminal() TerminalConfig {
	return s.config.Terminal()
}

// LSP returns type-safe access to Language Server Protocol settings.
func (s *ConfigSystem) LSP() LSPConfig {
	return s.config.LSP()
}

// Paths returns type-safe access to path settings.
func (s *ConfigSystem) Paths() PathsConfig {
	return s.config.Paths()
}

// Plugins returns the plugin manager.
func (s *ConfigSystem) Plugins() *PluginManager {
	return s.config.Plugins()
}

// Plugin returns configuration for a specific plugin.
func (s *ConfigSystem) Plugin(name string) *PluginConfig {
	return s.config.Plugin(name)
}

// Keymaps returns the keymap manager.
func (s *ConfigSystem) Keymaps() *KeymapManager {
	return s.config.Keymaps()
}

// ConfigErrors returns any configuration errors encountered during access.
func (s *ConfigSystem) ConfigErrors() map[string]error {
	return s.config.ConfigErrors()
}

// ClearConfigErrors clears any stored configuration errors.
func (s *ConfigSystem) ClearConfigErrors() {
	s.config.ClearConfigErrors()
}

// Health returns the health status of the configuration system.
func (s *ConfigSystem) Health() SystemHealth {
	errors := s.config.ConfigErrors()
	status := HealthOK
	if len(errors) > 0 {
		status = HealthDegraded
	}

	s.mu.RLock()
	loadTime := s.loadTime
	lastReloadAt := s.lastReloadAt
	s.mu.RUnlock()

	// Return a copy of errors to prevent external mutation
	errorsCopy := make(map[string]error, len(errors))
	for k, v := range errors {
		errorsCopy[k] = v
	}

	return SystemHealth{
		Status:       status,
		LoadTime:     loadTime,
		LastReloadAt: lastReloadAt,
		ErrorCount:   len(errors),
		Errors:       errorsCopy,
	}
}

// SystemHealth represents the health status of the configuration system.
type SystemHealth struct {
	// Status is the overall health status.
	Status HealthStatus

	// LoadTime is the duration of the last configuration load.
	LoadTime time.Duration

	// LastReloadAt is the time of the last configuration reload.
	LastReloadAt time.Time

	// ErrorCount is the number of configuration errors.
	ErrorCount int

	// Errors contains the configuration errors by path.
	Errors map[string]error
}

// HealthStatus represents the health status of a component.
type HealthStatus int

const (
	// HealthOK indicates the system is healthy.
	HealthOK HealthStatus = iota
	// HealthDegraded indicates the system has non-critical issues.
	HealthDegraded
	// HealthUnhealthy indicates the system has critical issues.
	HealthUnhealthy
)

// String returns a human-readable status string.
func (s HealthStatus) String() string {
	switch s {
	case HealthOK:
		return "ok"
	case HealthDegraded:
		return "degraded"
	case HealthUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}
