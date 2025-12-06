// Package config provides plugin configuration management.
package config

import (
	"strings"
	"sync"

	"github.com/dshills/keystorm/internal/config/notify"
	"github.com/dshills/keystorm/internal/config/schema"
)

// PluginConfig holds configuration for a single plugin.
type PluginConfig struct {
	// Name is the unique plugin identifier.
	Name string

	// Enabled controls whether the plugin is active.
	Enabled bool

	// Settings contains plugin-specific settings.
	Settings map[string]any
}

// PluginManager manages plugin configuration and schemas.
//
// Thread Safety:
// PluginManager is safe for concurrent use. All public methods acquire
// appropriate locks before accessing internal state. The manager uses
// its own mutex (mu) independent of Config's mutex to avoid deadlocks.
//
// When calling Config methods (like Get), PluginManager only uses
// read-only accessors that acquire Config's read lock, which is safe
// to nest with PluginManager's lock.
type PluginManager struct {
	mu sync.RWMutex

	// config is the parent Config for accessing/storing settings.
	// Only read-only methods (Get) should be called to avoid lock inversion.
	config *Config

	// notifier for change notifications
	notifier *notify.Notifier

	// schemas maps plugin names to their JSON schemas
	schemas map[string]*schema.Schema

	// plugins maps plugin names to their current configuration
	plugins map[string]*PluginConfig
}

// NewPluginManager creates a new PluginManager.
func NewPluginManager(config *Config, notifier *notify.Notifier) *PluginManager {
	return &PluginManager{
		config:   config,
		notifier: notifier,
		schemas:  make(map[string]*schema.Schema),
		plugins:  make(map[string]*PluginConfig),
	}
}

// RegisterPlugin registers a plugin with its configuration schema.
// If schemaJSON is nil, no validation will be performed for the plugin's settings.
func (m *PluginManager) RegisterPlugin(name string, schemaJSON []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if schemaJSON != nil {
		s, err := schema.Parse(schemaJSON)
		if err != nil {
			return err
		}
		m.schemas[name] = s
	}

	// Initialize plugin config with defaults
	m.plugins[name] = &PluginConfig{
		Name:     name,
		Enabled:  true,
		Settings: make(map[string]any),
	}

	return nil
}

// UnregisterPlugin removes a plugin registration.
func (m *PluginManager) UnregisterPlugin(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.schemas, name)
	delete(m.plugins, name)
}

// IsRegistered returns true if the plugin is registered.
func (m *PluginManager) IsRegistered(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.plugins[name]
	return ok
}

// GetPluginConfig returns a copy of the plugin's configuration.
func (m *PluginManager) GetPluginConfig(name string) (*PluginConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pc, ok := m.plugins[name]
	if !ok {
		return nil, false
	}

	// Return a copy to maintain snapshot guarantee
	copy := &PluginConfig{
		Name:     pc.Name,
		Enabled:  pc.Enabled,
		Settings: make(map[string]any, len(pc.Settings)),
	}
	for k, v := range pc.Settings {
		copy.Settings[k] = v
	}
	return copy, true
}

// GetSetting returns a plugin setting value.
func (m *PluginManager) GetSetting(pluginName, key string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pc, ok := m.plugins[pluginName]
	if !ok {
		return nil, false
	}

	v, ok := pc.Settings[key]
	return v, ok
}

// SetSetting sets a plugin setting value.
func (m *PluginManager) SetSetting(pluginName, key string, value any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pc, ok := m.plugins[pluginName]
	if !ok {
		return ErrSettingNotFound
	}

	// Validate against schema if present
	if s, hasSchema := m.schemas[pluginName]; hasSchema {
		testSettings := make(map[string]any, len(pc.Settings)+1)
		for k, v := range pc.Settings {
			testSettings[k] = v
		}
		testSettings[key] = value

		validator := schema.NewValidator(s)
		if err := validator.Validate(testSettings); err != nil {
			return err
		}
	}

	oldValue := pc.Settings[key]
	pc.Settings[key] = value

	// Notify change (outside lock would be better, but simpler this way)
	path := "plugins." + pluginName + ".settings." + key
	if m.notifier != nil {
		m.notifier.NotifySet(path, oldValue, value, "plugin")
	}

	return nil
}

// SetEnabled enables or disables a plugin.
func (m *PluginManager) SetEnabled(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pc, ok := m.plugins[name]
	if !ok {
		return ErrSettingNotFound
	}

	oldEnabled := pc.Enabled
	pc.Enabled = enabled

	path := "plugins." + name + ".enabled"
	if m.notifier != nil {
		m.notifier.NotifySet(path, oldEnabled, enabled, "plugin")
	}

	return nil
}

// IsEnabled returns whether a plugin is enabled.
func (m *PluginManager) IsEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pc, ok := m.plugins[name]
	if !ok {
		return false
	}
	return pc.Enabled
}

// ValidateSettings validates plugin settings against its schema.
// Returns nil if valid or no schema is registered.
func (m *PluginManager) ValidateSettings(name string, settings map[string]any) []*ValidationError {
	m.mu.RLock()
	s := m.schemas[name]
	m.mu.RUnlock()

	if s == nil {
		return nil
	}

	validator := schema.NewValidator(s)
	err := validator.Validate(settings)
	if err == nil {
		return nil
	}

	schemaErrors, ok := err.(*schema.ValidationErrors)
	if !ok {
		return []*ValidationError{{
			Path:    "plugins." + name,
			Message: err.Error(),
		}}
	}

	var validationErrors []*ValidationError
	for _, se := range schemaErrors.Errors {
		validationErrors = append(validationErrors, &ValidationError{
			Path:    "plugins." + name + ".settings." + se.Path,
			Message: se.Message,
			Value:   se.Value,
		})
	}
	return validationErrors
}

// ListPlugins returns names of all registered plugins.
func (m *PluginManager) ListPlugins() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.plugins))
	for name := range m.plugins {
		names = append(names, name)
	}
	return names
}

// LoadFromConfig loads plugin configurations from the config system.
// This should be called after Config.Load() to initialize plugin settings
// from stored configuration.
func (m *PluginManager) LoadFromConfig() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read plugins config from parent Config
	pluginsValue, ok := m.config.Get("plugins")
	if !ok {
		return
	}

	pluginsMap, ok := pluginsValue.(map[string]any)
	if !ok {
		return
	}

	for name, pluginValue := range pluginsMap {
		pluginMap, ok := pluginValue.(map[string]any)
		if !ok {
			continue
		}

		pc, exists := m.plugins[name]
		if !exists {
			// Create new plugin config for unregistered plugins
			pc = &PluginConfig{
				Name:     name,
				Enabled:  true,
				Settings: make(map[string]any),
			}
			m.plugins[name] = pc
		}

		// Load enabled state
		if enabled, ok := pluginMap["enabled"].(bool); ok {
			pc.Enabled = enabled
		}

		// Load settings
		if settings, ok := pluginMap["settings"].(map[string]any); ok {
			for k, v := range settings {
				pc.Settings[k] = v
			}
		}
	}
}

// SubscribePlugin subscribes to changes for a specific plugin's settings.
func (m *PluginManager) SubscribePlugin(pluginName string, observer notify.Observer) *notify.Subscription {
	path := "plugins." + pluginName
	return m.notifier.SubscribePath(path, observer)
}

// SubscribeAllPlugins subscribes to changes for all plugin settings.
func (m *PluginManager) SubscribeAllPlugins(observer notify.Observer) *notify.Subscription {
	return m.notifier.SubscribePath("plugins", observer)
}

// GetSchema returns the schema for a plugin, if registered.
func (m *PluginManager) GetSchema(name string) (*schema.Schema, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.schemas[name]
	return s, ok
}

// UpdateSettings updates multiple settings for a plugin at once.
func (m *PluginManager) UpdateSettings(pluginName string, settings map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pc, ok := m.plugins[pluginName]
	if !ok {
		return ErrSettingNotFound
	}

	// Validate against schema if present
	if s, hasSchema := m.schemas[pluginName]; hasSchema {
		mergedSettings := make(map[string]any, len(pc.Settings)+len(settings))
		for k, v := range pc.Settings {
			mergedSettings[k] = v
		}
		for k, v := range settings {
			mergedSettings[k] = v
		}

		validator := schema.NewValidator(s)
		if err := validator.Validate(mergedSettings); err != nil {
			return err
		}
	}

	// Apply changes and notify
	for key, value := range settings {
		oldValue := pc.Settings[key]
		pc.Settings[key] = value

		path := "plugins." + pluginName + ".settings." + key
		if m.notifier != nil {
			m.notifier.NotifySet(path, oldValue, value, "plugin")
		}
	}

	return nil
}

// GetSettingString returns a string setting with a default value.
func (m *PluginManager) GetSettingString(pluginName, key, defaultValue string) string {
	v, ok := m.GetSetting(pluginName, key)
	if !ok {
		return defaultValue
	}
	s, ok := v.(string)
	if !ok {
		return defaultValue
	}
	return s
}

// GetSettingInt returns an int setting with a default value.
func (m *PluginManager) GetSettingInt(pluginName, key string, defaultValue int) int {
	v, ok := m.GetSetting(pluginName, key)
	if !ok {
		return defaultValue
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return defaultValue
	}
}

// GetSettingBool returns a bool setting with a default value.
func (m *PluginManager) GetSettingBool(pluginName, key string, defaultValue bool) bool {
	v, ok := m.GetSetting(pluginName, key)
	if !ok {
		return defaultValue
	}
	b, ok := v.(bool)
	if !ok {
		return defaultValue
	}
	return b
}

// MatchesPluginPath returns true if the path is for a plugin setting.
func MatchesPluginPath(path string) bool {
	return strings.HasPrefix(path, "plugins.")
}

// ParsePluginPath extracts plugin name and setting key from a path.
// Returns empty strings if path is not a valid plugin setting path.
func ParsePluginPath(path string) (pluginName, settingKey string) {
	if !strings.HasPrefix(path, "plugins.") {
		return "", ""
	}

	rest := strings.TrimPrefix(path, "plugins.")
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) == 0 {
		return "", ""
	}

	pluginName = parts[0]
	if len(parts) > 1 && strings.HasPrefix(parts[1], "settings.") {
		settingKey = strings.TrimPrefix(parts[1], "settings.")
	}

	return pluginName, settingKey
}
