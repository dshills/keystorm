package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dshills/keystorm/internal/config/layer"
	"github.com/dshills/keystorm/internal/config/loader"
	"github.com/dshills/keystorm/internal/config/notify"
	"github.com/dshills/keystorm/internal/config/schema"
	"github.com/dshills/keystorm/internal/config/watcher"
)

// Config provides unified access to the Keystorm configuration system.
// It manages configuration loading, validation, live reloading, and change notification.
type Config struct {
	mu sync.RWMutex

	// Layer manager for merged configuration
	layers *layer.Manager

	// Schema validator
	validator *schema.Validator

	// File watcher for live reload
	watcher *watcher.Watcher

	// Change notifier
	notifier *notify.Notifier

	// Plugin manager for plugin configuration
	plugins *PluginManager

	// Keymap manager for keymap configuration
	keymaps *KeymapManager

	// Configuration paths
	userConfigDir    string
	projectConfigDir string

	// Options
	enableWatcher bool
	enableSchema  bool

	// configErrors stores errors encountered during configuration access.
	// This allows detection of type mismatches and other config problems.
	configErrors map[string]error
}

// Option configures a Config instance.
type Option func(*Config)

// WithUserConfigDir sets the user configuration directory.
func WithUserConfigDir(dir string) Option {
	return func(c *Config) {
		c.userConfigDir = dir
	}
}

// WithProjectConfigDir sets the project configuration directory.
func WithProjectConfigDir(dir string) Option {
	return func(c *Config) {
		c.projectConfigDir = dir
	}
}

// WithWatcher enables file watching for live reload.
func WithWatcher(enable bool) Option {
	return func(c *Config) {
		c.enableWatcher = enable
	}
}

// WithSchemaValidation enables schema validation.
func WithSchemaValidation(enable bool) Option {
	return func(c *Config) {
		c.enableSchema = enable
	}
}

// New creates a new Config instance with the given options.
func New(opts ...Option) *Config {
	c := &Config{
		layers:        layer.NewManager(),
		notifier:      notify.New(),
		enableWatcher: true,
		enableSchema:  true,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Set default paths
	if c.userConfigDir == "" {
		c.userConfigDir = defaultUserConfigDir()
	}

	// Initialize schema validator
	if c.enableSchema {
		if s, err := schema.LoadEmbedded(); err == nil {
			c.validator = schema.NewValidator(s)
		}
	}

	// Initialize file watcher
	if c.enableWatcher {
		c.watcher = watcher.New()
		c.watcher.OnChange(c.handleFileChange)
	}

	// Initialize plugin manager.
	// This is safe because:
	// 1. NewPluginManager only stores references, it doesn't invoke callbacks
	// 2. The notifier is already initialized above
	// 3. PluginManager methods that need Config use thread-safe accessors (c.Get)
	c.plugins = NewPluginManager(c, c.notifier)

	// Initialize keymap manager.
	// Similar to plugin manager, NewKeymapManager only stores references.
	c.keymaps = NewKeymapManager(c, c.notifier)

	return c
}

// Load loads configuration from all sources.
func (c *Config) Load(_ context.Context) error {
	c.mu.Lock()

	// Load defaults layer
	if err := c.loadDefaults(); err != nil {
		c.mu.Unlock()
		return err
	}

	// Load user settings
	if err := c.loadUserSettings(); err != nil && !os.IsNotExist(err) {
		c.mu.Unlock()
		return err
	}

	// Load user keymaps
	if err := c.loadUserKeymaps(); err != nil && !os.IsNotExist(err) {
		c.mu.Unlock()
		return err
	}

	// Load project settings
	if c.projectConfigDir != "" {
		if err := c.loadProjectSettings(); err != nil && !os.IsNotExist(err) {
			c.mu.Unlock()
			return err
		}
	}

	// Load environment variables
	if err := c.loadEnvironment(); err != nil {
		c.mu.Unlock()
		return err
	}

	// Release lock before starting watcher to avoid deadlock
	// (watcher callbacks acquire the same lock)
	w := c.watcher
	plugins := c.plugins
	keymaps := c.keymaps
	c.mu.Unlock()

	// Load plugin configuration from the config layers.
	// This is called outside the Config lock because:
	// 1. LoadFromConfig has its own internal locking (PluginManager.mu)
	// 2. It calls c.Get() which acquires c.mu.RLock, avoiding deadlock
	// 3. The plugin manager reference is stable after initialization
	if plugins != nil {
		plugins.LoadFromConfig()
	}

	// Load keymap configuration.
	// 1. Load default keymaps first (lowest priority)
	// 2. Then load user keymaps from config (higher priority, override defaults)
	if keymaps != nil {
		if err := keymaps.LoadDefaults(); err != nil {
			return fmt.Errorf("loading default keymaps: %w", err)
		}
		if err := keymaps.LoadFromConfig(); err != nil {
			return fmt.Errorf("loading user keymaps: %w", err)
		}
	}

	// Start file watcher outside the lock
	if w != nil {
		w.Start()
	}

	return nil
}

// Close shuts down the configuration system.
func (c *Config) Close() {
	if c.watcher != nil {
		c.watcher.Stop()
	}
	if c.notifier != nil {
		c.notifier.Close()
	}
}

// Get returns the value at the given path from the merged configuration.
func (c *Config) Get(path string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	merged := c.layers.Merge()
	return getPath(merged, path)
}

// GetString returns a string value at the given path.
func (c *Config) GetString(path string) (string, error) {
	v, ok := c.Get(path)
	if !ok {
		return "", ErrSettingNotFound
	}
	s, ok := v.(string)
	if !ok {
		return "", &TypeError{Path: path, Expected: "string", Actual: typeName(v)}
	}
	return s, nil
}

// GetInt returns an integer value at the given path.
func (c *Config) GetInt(path string) (int, error) {
	v, ok := c.Get(path)
	if !ok {
		return 0, ErrSettingNotFound
	}
	switch val := v.(type) {
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case float64:
		return int(val), nil
	default:
		return 0, &TypeError{Path: path, Expected: "int", Actual: typeName(v)}
	}
}

// GetBool returns a boolean value at the given path.
func (c *Config) GetBool(path string) (bool, error) {
	v, ok := c.Get(path)
	if !ok {
		return false, ErrSettingNotFound
	}
	b, ok := v.(bool)
	if !ok {
		return false, &TypeError{Path: path, Expected: "bool", Actual: typeName(v)}
	}
	return b, nil
}

// GetFloat returns a float64 value at the given path.
func (c *Config) GetFloat(path string) (float64, error) {
	v, ok := c.Get(path)
	if !ok {
		return 0, ErrSettingNotFound
	}
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, &TypeError{Path: path, Expected: "float64", Actual: typeName(v)}
	}
}

// GetStringSlice returns a string slice at the given path.
func (c *Config) GetStringSlice(path string) ([]string, error) {
	v, ok := c.Get(path)
	if !ok {
		return nil, ErrSettingNotFound
	}

	switch val := v.(type) {
	case []string:
		return val, nil
	case []any:
		result := make([]string, len(val))
		for i, item := range val {
			s, ok := item.(string)
			if !ok {
				return nil, &TypeError{Path: path, Expected: "[]string", Actual: typeName(v)}
			}
			result[i] = s
		}
		return result, nil
	default:
		return nil, &TypeError{Path: path, Expected: "[]string", Actual: typeName(v)}
	}
}

// Set sets a value at the given path in the user settings layer.
func (c *Config) Set(path string, value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Validate against schema
	if c.validator != nil {
		if err := c.validator.ValidatePath(path, value); err != nil {
			return err
		}
	}

	// Set in user settings layer
	userLayer := c.layers.GetLayer("user-settings")
	if userLayer == nil {
		return ErrLayerNotFound
	}

	if userLayer.Data == nil {
		userLayer.Data = make(map[string]any)
	}

	// Get old merged value for notification (effective value before change)
	oldMerged := c.layers.Merge()
	oldValue, _ := getPath(oldMerged, path)

	if err := setPath(userLayer.Data, path, value); err != nil {
		return err
	}

	// Mark layers as dirty so merge is refreshed
	c.layers.Invalidate()

	// Get new merged value for notification (effective value after change)
	newMerged := c.layers.Merge()
	newValue, _ := getPath(newMerged, path)

	// Notify observers with effective merged values
	c.notifier.NotifySet(path, oldValue, newValue, "user")

	return nil
}

// Subscribe registers an observer for all configuration changes.
func (c *Config) Subscribe(observer notify.Observer) *notify.Subscription {
	return c.notifier.Subscribe(observer)
}

// SubscribePath registers an observer for changes to a specific path.
func (c *Config) SubscribePath(path string, observer notify.Observer) *notify.Subscription {
	return c.notifier.SubscribePath(path, observer)
}

// Merged returns the fully merged configuration.
func (c *Config) Merged() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.layers.Merge()
}

// loadDefaults loads the default configuration layer.
func (c *Config) loadDefaults() error {
	defaults := defaultConfig()
	l := layer.NewLayerWithData("defaults", layer.SourceBuiltin, layer.PriorityBuiltin, defaults)
	c.layers.AddLayer(l)
	return nil
}

// loadUserSettings loads user settings from the config directory.
func (c *Config) loadUserSettings() error {
	settingsPath := filepath.Join(c.userConfigDir, "settings.toml")

	tomlLoader := loader.NewTOMLLoader(settingsPath)
	data, err := tomlLoader.Load()
	if err != nil {
		return err
	}
	if data == nil {
		return os.ErrNotExist
	}

	l := layer.NewLayerWithData("user-settings", layer.SourceUserGlobal, layer.PriorityUserGlobal, data)
	c.layers.AddLayer(l)

	// Watch for changes
	if c.watcher != nil {
		_ = c.watcher.Watch(settingsPath)
	}

	return nil
}

// loadUserKeymaps loads user keymaps from the config directory.
func (c *Config) loadUserKeymaps() error {
	keymapsPath := filepath.Join(c.userConfigDir, "keymaps.toml")

	tomlLoader := loader.NewTOMLLoader(keymapsPath)
	data, err := tomlLoader.Load()
	if err != nil {
		return err
	}
	if data == nil {
		return os.ErrNotExist
	}

	l := layer.NewLayerWithData("user-keymaps", layer.SourceUserGlobal, layer.PriorityUserKeymaps, data)
	c.layers.AddLayer(l)

	// Watch for changes
	if c.watcher != nil {
		_ = c.watcher.Watch(keymapsPath)
	}

	return nil
}

// loadProjectSettings loads project-specific settings.
func (c *Config) loadProjectSettings() error {
	settingsPath := filepath.Join(c.projectConfigDir, "config.toml")

	tomlLoader := loader.NewTOMLLoader(settingsPath)
	data, err := tomlLoader.Load()
	if err != nil {
		return err
	}
	if data == nil {
		return os.ErrNotExist
	}

	l := layer.NewLayerWithData("project", layer.SourceWorkspace, layer.PriorityWorkspace, data)
	c.layers.AddLayer(l)

	// Watch for changes
	if c.watcher != nil {
		_ = c.watcher.Watch(settingsPath)
	}

	return nil
}

// loadEnvironment loads configuration from environment variables.
func (c *Config) loadEnvironment() error {
	envLoader := loader.NewEnvLoader("KEYSTORM")
	data, err := envLoader.Load()
	if err != nil {
		return err
	}

	if len(data) > 0 {
		l := layer.NewLayerWithData("environment", layer.SourceEnv, layer.PriorityEnv, data)
		c.layers.AddLayer(l)
	}

	return nil
}

// handleFileChange handles file change events from the watcher.
func (c *Config) handleFileChange(event watcher.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Determine which layer to update based on the path
	base := filepath.Base(event.Path)
	eventDir := filepath.Clean(filepath.Dir(event.Path))
	userDir := filepath.Clean(c.userConfigDir)

	var layerName string
	var source layer.Source
	var priority int

	switch base {
	case "settings.toml":
		if eventDir == userDir {
			layerName = "user-settings"
			source = layer.SourceUserGlobal
			priority = layer.PriorityUserGlobal
		} else {
			layerName = "project"
			source = layer.SourceWorkspace
			priority = layer.PriorityWorkspace
		}
	case "keymaps.toml":
		layerName = "user-keymaps"
		source = layer.SourceUserGlobal
		priority = layer.PriorityUserKeymaps
	case "config.toml":
		layerName = "project"
		source = layer.SourceWorkspace
		priority = layer.PriorityWorkspace
	default:
		return
	}

	// Handle remove events by removing the layer
	if event.Op == watcher.OpRemove {
		c.layers.RemoveLayer(layerName)
		c.notifier.NotifyReload(event.Path)
		return
	}

	// For create/write events, reload the file
	tomlLoader := loader.NewTOMLLoader(event.Path)
	data, err := tomlLoader.Load()
	if err != nil || data == nil {
		return
	}

	// Remove old layer and add new one
	c.layers.RemoveLayer(layerName)
	l := layer.NewLayerWithData(layerName, source, priority, data)
	c.layers.AddLayer(l)

	// Notify reload
	c.notifier.NotifyReload(event.Path)
}

// defaultUserConfigDir returns the default user configuration directory.
func defaultUserConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "keystorm")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "keystorm")
}

// defaultConfig returns the default configuration values.
func defaultConfig() map[string]any {
	return map[string]any{
		"editor": map[string]any{
			"tabSize":      4,
			"insertSpaces": true,
			"wordWrap":     "off",
			"lineNumbers":  true,
			"autoSave":     false,
			"formatOnSave": false,
		},
		"ui": map[string]any{
			"theme":       "dark",
			"fontSize":    14,
			"fontFamily":  "monospace",
			"showMinimap": true,
		},
		"vim": map[string]any{
			"enabled":             true,
			"startInInsertMode":   false,
			"relativeLineNumbers": false,
		},
		"files": map[string]any{
			"exclude":        []string{".git", "node_modules", ".DS_Store"},
			"watcherExclude": []string{".git", "node_modules"},
			"encoding":       "utf-8",
			"eol":            "lf",
		},
		"search": map[string]any{
			"caseSensitive": false,
			"wholeWord":     false,
			"regex":         false,
			"maxResults":    1000,
		},
		"ai": map[string]any{
			"enabled":     true,
			"provider":    "anthropic",
			"model":       "claude-sonnet-4-20250514",
			"maxTokens":   4096,
			"temperature": 0.7,
		},
		"logging": map[string]any{
			"level":  "info",
			"format": "text",
		},
	}
}

// getPath retrieves a value from a nested map using a dot-separated path.
func getPath(m map[string]any, path string) (any, bool) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return nil, false
	}

	current := any(m)
	for _, part := range parts {
		if cm, ok := current.(map[string]any); ok {
			current, ok = cm[part]
			if !ok {
				return nil, false
			}
		} else {
			return nil, false
		}
	}

	return current, true
}

// setPath sets a value in a nested map using a dot-separated path.
func setPath(m map[string]any, path string, value any) error {
	parts := splitPath(path)
	if len(parts) == 0 {
		return ErrInvalidPath
	}

	current := m
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			// Create nested map
			next = make(map[string]any)
			current[part] = next
		}
		nextMap, ok := next.(map[string]any)
		if !ok {
			return ErrInvalidPath
		}
		current = nextMap
	}

	current[parts[len(parts)-1]] = value
	return nil
}

// splitPath splits a dot-separated path into parts.
func splitPath(path string) []string {
	if path == "" {
		return nil
	}

	var parts []string
	current := ""
	for _, c := range path {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// typeName returns the type name for error messages.
func typeName(v any) string {
	if v == nil {
		return "nil"
	}
	switch v.(type) {
	case string:
		return "string"
	case int, int64:
		return "int"
	case float64:
		return "float64"
	case bool:
		return "bool"
	case []string:
		return "[]string"
	case []any:
		return "[]any"
	case map[string]any:
		return "map"
	default:
		return "unknown"
	}
}

// Plugins returns the plugin manager for plugin configuration.
// The returned PluginManager is thread-safe and can be used concurrently.
func (c *Config) Plugins() *PluginManager {
	c.mu.RLock()
	pm := c.plugins
	c.mu.RUnlock()
	return pm
}

// Plugin returns the configuration for a specific plugin.
// Returns nil if the plugin is not registered.
// The returned PluginConfig is a snapshot; mutations do not affect the config.
func (c *Config) Plugin(name string) *PluginConfig {
	c.mu.RLock()
	pm := c.plugins
	c.mu.RUnlock()

	if pm == nil {
		return nil
	}
	pc, ok := pm.GetPluginConfig(name)
	if !ok {
		return nil
	}
	return pc
}

// Keymaps returns the keymap manager for keymap configuration.
// The returned KeymapManager is thread-safe and can be used concurrently.
func (c *Config) Keymaps() *KeymapManager {
	c.mu.RLock()
	keymaps := c.keymaps
	c.mu.RUnlock()
	return keymaps
}
