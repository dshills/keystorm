package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dshills/keystorm/internal/config/notify"
)

func TestPluginManager_RegisterPlugin(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	if pm == nil {
		t.Fatal("Plugins() returned nil")
	}

	// Register a plugin without schema
	err := pm.RegisterPlugin("test-plugin", nil)
	if err != nil {
		t.Fatalf("RegisterPlugin() error = %v", err)
	}

	if !pm.IsRegistered("test-plugin") {
		t.Error("IsRegistered() = false, want true")
	}

	// Check default state
	if !pm.IsEnabled("test-plugin") {
		t.Error("IsEnabled() = false, want true")
	}
}

func TestPluginManager_RegisterPluginWithSchema(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()

	// Valid JSON schema
	schemaJSON := []byte(`{
		"type": "object",
		"properties": {
			"setting1": {"type": "string"},
			"setting2": {"type": "integer"}
		}
	}`)

	err := pm.RegisterPlugin("schema-plugin", schemaJSON)
	if err != nil {
		t.Fatalf("RegisterPlugin() error = %v", err)
	}

	if !pm.IsRegistered("schema-plugin") {
		t.Error("IsRegistered() = false, want true")
	}

	s, ok := pm.GetSchema("schema-plugin")
	if !ok {
		t.Error("GetSchema() = false, want true")
	}
	if s == nil {
		t.Error("GetSchema() returned nil schema")
	}
}

func TestPluginManager_RegisterPluginInvalidSchema(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()

	// Invalid JSON
	invalidSchema := []byte(`{invalid json}`)

	err := pm.RegisterPlugin("bad-plugin", invalidSchema)
	if err == nil {
		t.Error("RegisterPlugin() should return error for invalid schema")
	}
}

func TestPluginManager_UnregisterPlugin(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()

	_ = pm.RegisterPlugin("temp-plugin", nil)
	if !pm.IsRegistered("temp-plugin") {
		t.Fatal("Plugin not registered")
	}

	pm.UnregisterPlugin("temp-plugin")

	if pm.IsRegistered("temp-plugin") {
		t.Error("IsRegistered() = true, want false after unregister")
	}
}

func TestPluginManager_GetSetSetting(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("test-plugin", nil)

	// Set a setting
	err := pm.SetSetting("test-plugin", "key1", "value1")
	if err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}

	// Get the setting
	v, ok := pm.GetSetting("test-plugin", "key1")
	if !ok {
		t.Fatal("GetSetting() returned false")
	}
	if v != "value1" {
		t.Errorf("GetSetting() = %v, want 'value1'", v)
	}

	// Get non-existent setting
	_, ok = pm.GetSetting("test-plugin", "nonexistent")
	if ok {
		t.Error("GetSetting() returned true for non-existent key")
	}
}

func TestPluginManager_GetSettingTyped(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("test-plugin", nil)

	// Set various types
	_ = pm.SetSetting("test-plugin", "strKey", "hello")
	_ = pm.SetSetting("test-plugin", "intKey", 42)
	_ = pm.SetSetting("test-plugin", "boolKey", true)

	// Test typed getters
	strVal := pm.GetSettingString("test-plugin", "strKey", "default")
	if strVal != "hello" {
		t.Errorf("GetSettingString() = %q, want 'hello'", strVal)
	}

	intVal := pm.GetSettingInt("test-plugin", "intKey", 0)
	if intVal != 42 {
		t.Errorf("GetSettingInt() = %d, want 42", intVal)
	}

	boolVal := pm.GetSettingBool("test-plugin", "boolKey", false)
	if !boolVal {
		t.Error("GetSettingBool() = false, want true")
	}

	// Test defaults for missing keys
	defaultStr := pm.GetSettingString("test-plugin", "missing", "default")
	if defaultStr != "default" {
		t.Errorf("GetSettingString() default = %q, want 'default'", defaultStr)
	}
}

func TestPluginManager_SetEnabled(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("test-plugin", nil)

	// Disable plugin
	err := pm.SetEnabled("test-plugin", false)
	if err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	if pm.IsEnabled("test-plugin") {
		t.Error("IsEnabled() = true, want false")
	}

	// Re-enable
	err = pm.SetEnabled("test-plugin", true)
	if err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	if !pm.IsEnabled("test-plugin") {
		t.Error("IsEnabled() = false, want true")
	}
}

func TestPluginManager_SetEnabled_NotRegistered(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()

	err := pm.SetEnabled("nonexistent", true)
	if err == nil {
		t.Error("SetEnabled() should return error for non-existent plugin")
	}
}

func TestPluginManager_GetPluginConfig(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("test-plugin", nil)
	_ = pm.SetSetting("test-plugin", "key1", "value1")
	_ = pm.SetEnabled("test-plugin", false)

	pc, ok := pm.GetPluginConfig("test-plugin")
	if !ok {
		t.Fatal("GetPluginConfig() returned false")
	}

	if pc.Name != "test-plugin" {
		t.Errorf("Name = %q, want 'test-plugin'", pc.Name)
	}
	if pc.Enabled {
		t.Error("Enabled = true, want false")
	}
	if pc.Settings["key1"] != "value1" {
		t.Errorf("Settings[key1] = %v, want 'value1'", pc.Settings["key1"])
	}
}

func TestPluginManager_GetPluginConfig_SnapshotGuarantee(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("test-plugin", nil)
	_ = pm.SetSetting("test-plugin", "key1", "value1")

	// Get config
	pc1, _ := pm.GetPluginConfig("test-plugin")

	// Mutate returned config
	pc1.Settings["key1"] = "mutated"
	pc1.Enabled = false

	// Get config again
	pc2, _ := pm.GetPluginConfig("test-plugin")

	// Mutation should not affect the original
	if pc2.Settings["key1"] != "value1" {
		t.Error("Mutation affected underlying config")
	}
	if !pc2.Enabled {
		t.Error("Mutation affected underlying config enabled state")
	}
}

func TestPluginManager_ListPlugins(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("plugin-a", nil)
	_ = pm.RegisterPlugin("plugin-b", nil)
	_ = pm.RegisterPlugin("plugin-c", nil)

	plugins := pm.ListPlugins()
	if len(plugins) != 3 {
		t.Errorf("ListPlugins() count = %d, want 3", len(plugins))
	}

	// Check all plugins are present
	found := make(map[string]bool)
	for _, p := range plugins {
		found[p] = true
	}
	for _, expected := range []string{"plugin-a", "plugin-b", "plugin-c"} {
		if !found[expected] {
			t.Errorf("ListPlugins() missing %q", expected)
		}
	}
}

func TestPluginManager_ValidateSettings(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()

	schemaJSON := []byte(`{
		"type": "object",
		"properties": {
			"port": {"type": "integer", "minimum": 1, "maximum": 65535}
		},
		"required": ["port"]
	}`)

	_ = pm.RegisterPlugin("validated-plugin", schemaJSON)

	// Valid settings
	validSettings := map[string]any{"port": 8080}
	errors := pm.ValidateSettings("validated-plugin", validSettings)
	if len(errors) > 0 {
		t.Errorf("ValidateSettings() returned %d errors for valid settings", len(errors))
	}

	// Invalid settings - port out of range
	invalidSettings := map[string]any{"port": 100000}
	errors = pm.ValidateSettings("validated-plugin", invalidSettings)
	if len(errors) == 0 {
		t.Error("ValidateSettings() should return errors for invalid settings")
	}
}

func TestPluginManager_ValidateSettings_NoSchema(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("no-schema-plugin", nil)

	// Any settings should be valid without schema
	settings := map[string]any{"anything": "goes"}
	errors := pm.ValidateSettings("no-schema-plugin", settings)
	if len(errors) > 0 {
		t.Error("ValidateSettings() should return nil for plugin without schema")
	}
}

func TestPluginManager_UpdateSettings(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("test-plugin", nil)

	// Update multiple settings at once
	settings := map[string]any{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}
	err := pm.UpdateSettings("test-plugin", settings)
	if err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	// Verify all settings were applied
	v1, _ := pm.GetSetting("test-plugin", "key1")
	if v1 != "value1" {
		t.Errorf("key1 = %v, want 'value1'", v1)
	}
	v2, _ := pm.GetSetting("test-plugin", "key2")
	if v2 != 42 {
		t.Errorf("key2 = %v, want 42", v2)
	}
	v3, _ := pm.GetSetting("test-plugin", "key3")
	if v3 != true {
		t.Errorf("key3 = %v, want true", v3)
	}
}

func TestPluginManager_Notifications(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("test-plugin", nil)

	var received []notify.Change
	var mu sync.Mutex

	sub := pm.SubscribePlugin("test-plugin", func(change notify.Change) {
		mu.Lock()
		received = append(received, change)
		mu.Unlock()
	})
	defer sub.Unsubscribe()

	// Make changes
	_ = pm.SetSetting("test-plugin", "key1", "value1")
	_ = pm.SetEnabled("test-plugin", false)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 2 {
		t.Errorf("Received %d notifications, want 2", count)
	}
}

func TestPluginManager_LoadFromConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config file with plugin settings
	settingsPath := filepath.Join(tmpDir, "settings.toml")
	settingsContent := `
[plugins]
[plugins.my-plugin]
enabled = false
[plugins.my-plugin.settings]
option1 = "custom-value"
option2 = 123
`
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
	)
	defer c.Close()

	// Register the plugin before loading
	pm := c.Plugins()
	_ = pm.RegisterPlugin("my-plugin", nil)

	// Load config
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check plugin settings were loaded
	if pm.IsEnabled("my-plugin") {
		t.Error("IsEnabled() = true, want false")
	}

	v, _ := pm.GetSetting("my-plugin", "option1")
	if v != "custom-value" {
		t.Errorf("option1 = %v, want 'custom-value'", v)
	}
}

func TestConfig_Plugin(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("test-plugin", nil)
	_ = pm.SetSetting("test-plugin", "key", "value")

	// Use Config.Plugin() accessor
	pc := c.Plugin("test-plugin")
	if pc == nil {
		t.Fatal("Plugin() returned nil")
	}

	if pc.Name != "test-plugin" {
		t.Errorf("Name = %q, want 'test-plugin'", pc.Name)
	}
	if pc.Settings["key"] != "value" {
		t.Errorf("Settings[key] = %v, want 'value'", pc.Settings["key"])
	}

	// Non-existent plugin
	pc = c.Plugin("nonexistent")
	if pc != nil {
		t.Error("Plugin() should return nil for non-existent plugin")
	}
}

func TestMatchesPluginPath(t *testing.T) {
	tests := []struct {
		path  string
		match bool
	}{
		{"plugins.myplugin", true},
		{"plugins.myplugin.settings.key", true},
		{"plugins", false}, // exact "plugins" doesn't have the dot
		{"editor.tabSize", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := MatchesPluginPath(tt.path); got != tt.match {
			t.Errorf("MatchesPluginPath(%q) = %v, want %v", tt.path, got, tt.match)
		}
	}
}

func TestParsePluginPath(t *testing.T) {
	tests := []struct {
		path       string
		pluginName string
		settingKey string
	}{
		{"plugins.myplugin.settings.key", "myplugin", "key"},
		{"plugins.myplugin.settings.nested.key", "myplugin", "nested.key"},
		{"plugins.myplugin.enabled", "myplugin", ""},
		{"plugins.myplugin", "myplugin", ""},
		{"editor.tabSize", "", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		pluginName, settingKey := ParsePluginPath(tt.path)
		if pluginName != tt.pluginName || settingKey != tt.settingKey {
			t.Errorf("ParsePluginPath(%q) = (%q, %q), want (%q, %q)",
				tt.path, pluginName, settingKey, tt.pluginName, tt.settingKey)
		}
	}
}

func TestPluginManager_ConcurrentAccess(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	pm := c.Plugins()
	_ = pm.RegisterPlugin("concurrent-plugin", nil)

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent writes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = pm.SetSetting("concurrent-plugin", "key", i)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pm.GetSetting("concurrent-plugin", "key")
			pm.IsEnabled("concurrent-plugin")
			pm.GetPluginConfig("concurrent-plugin")
		}()
	}

	wg.Wait()
}
