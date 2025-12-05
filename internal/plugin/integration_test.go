package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSystem(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	if sys == nil {
		t.Fatal("NewSystem returned nil")
	}

	if sys.IsInitialized() {
		t.Error("new system should not be initialized")
	}
}

func TestSystemInitialize(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	err := sys.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if !sys.IsInitialized() {
		t.Error("system should be initialized after Initialize()")
	}

	// Second init should fail
	err = sys.Initialize()
	if err != ErrAlreadyInitialized {
		t.Errorf("second Initialize should return ErrAlreadyInitialized, got: %v", err)
	}
}

func TestSystemShutdown(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	// Shutdown before init is ok
	err := sys.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown before init should not error: %v", err)
	}

	// Init and shutdown
	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	err = sys.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	if sys.IsInitialized() {
		t.Error("system should not be initialized after Shutdown()")
	}
}

func TestSystemManager(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	manager := sys.Manager()
	if manager == nil {
		t.Error("Manager should not be nil after initialization")
	}
}

func TestSystemRegistry(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	registry := sys.Registry()
	if registry == nil {
		t.Error("Registry should not be nil after initialization")
	}

	// Check that default modules are registered
	modules := registry.List()
	if len(modules) == 0 {
		t.Error("registry should have default modules")
	}
}

func TestSystemAPIContext(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	apiCtx := sys.APIContext()
	if apiCtx == nil {
		t.Error("APIContext should not be nil after initialization")
	}
}

func TestSystemDiscoverUninitialized(t *testing.T) {
	sys := NewSystem(DefaultSystemConfig())

	_, err := sys.Discover()
	if err != ErrNotInitialized {
		t.Errorf("Discover before init should return ErrNotInitialized, got: %v", err)
	}
}

func TestSystemLoadPluginUninitialized(t *testing.T) {
	sys := NewSystem(DefaultSystemConfig())

	_, err := sys.LoadPlugin(context.Background(), "test")
	if err != ErrNotInitialized {
		t.Errorf("LoadPlugin before init should return ErrNotInitialized, got: %v", err)
	}
}

func TestSystemPluginCount(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	// Before init
	if sys.PluginCount() != 0 {
		t.Error("PluginCount before init should be 0")
	}

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	// After init with no plugins
	if sys.PluginCount() != 0 {
		t.Error("PluginCount should be 0 with no plugins loaded")
	}

	if sys.ActivePluginCount() != 0 {
		t.Error("ActivePluginCount should be 0 with no plugins loaded")
	}
}

func TestSystemListPlugins(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	// Before init
	if sys.ListPlugins() != nil {
		t.Error("ListPlugins before init should return nil")
	}

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	// After init
	plugins := sys.ListPlugins()
	if plugins == nil {
		t.Error("ListPlugins after init should not return nil")
	}
	if len(plugins) != 0 {
		t.Error("ListPlugins should return empty slice with no plugins")
	}
}

func TestSystemGetPlugin(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	// Before init
	_, ok := sys.GetPlugin("test")
	if ok {
		t.Error("GetPlugin before init should return false")
	}

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	// Non-existent plugin
	_, ok = sys.GetPlugin("nonexistent")
	if ok {
		t.Error("GetPlugin for nonexistent plugin should return false")
	}
}

func TestSystemHasErrors(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	// Before init
	if sys.HasErrors() {
		t.Error("HasErrors before init should return false")
	}

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	// No plugins, no errors
	if sys.HasErrors() {
		t.Error("HasErrors should return false with no plugins")
	}
}

func TestSystemErrors(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	// Before init
	if sys.Errors() != nil {
		t.Error("Errors before init should return nil")
	}

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	// No errors
	errs := sys.Errors()
	if errs == nil {
		t.Error("Errors should not return nil after init")
	}
	if len(errs) != 0 {
		t.Error("Errors should be empty with no plugins")
	}
}

func TestSystemSubscribe(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	// Before init - should return no-op
	unsub := sys.Subscribe(func(e ManagerEvent) {})
	if unsub == nil {
		t.Error("Subscribe should return unsubscribe function even before init")
	}
	unsub() // Should not panic

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	// After init
	eventReceived := make(chan ManagerEvent, 1)
	unsub = sys.Subscribe(func(e ManagerEvent) {
		select {
		case eventReceived <- e:
		default:
		}
	})
	defer unsub()

	if unsub == nil {
		t.Error("Subscribe should return unsubscribe function")
	}
}

func TestSystemStats(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	// Before init
	stats := sys.Stats()
	if stats.Initialized {
		t.Error("Stats.Initialized should be false before init")
	}

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	// After init
	stats = sys.Stats()
	if !stats.Initialized {
		t.Error("Stats.Initialized should be true after init")
	}
	if stats.TotalPlugins != 0 {
		t.Error("Stats.TotalPlugins should be 0 with no plugins")
	}
	if stats.ActivePlugins != 0 {
		t.Error("Stats.ActivePlugins should be 0 with no plugins")
	}
	if stats.HasErrors {
		t.Error("Stats.HasErrors should be false with no plugins")
	}
	if len(stats.RegisteredModules) == 0 {
		t.Error("Stats should have registered modules")
	}
}

func TestSystemSetProvider(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	// Before init
	err := sys.SetProvider("buffer", nil)
	if err != ErrNotInitialized {
		t.Errorf("SetProvider before init should return ErrNotInitialized, got: %v", err)
	}

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	// Unknown provider type
	err = sys.SetProvider("unknown", nil)
	if err == nil {
		t.Error("SetProvider with unknown type should error")
	}

	// Invalid provider type (wrong interface)
	err = sys.SetProvider("buffer", "not a provider")
	if err == nil {
		t.Error("SetProvider with wrong type should error")
	}
}

func TestDefaultSystemConfig(t *testing.T) {
	config := DefaultSystemConfig()

	// Check manager config has defaults
	if len(config.ManagerConfig.PluginPaths) == 0 {
		t.Error("default config should have plugin paths")
	}
}

// Integration test with real plugins
func TestSystemLoadRealPlugin(t *testing.T) {
	// Create temp plugin directory
	tmpDir, err := os.MkdirTemp("", "plugin-system-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple test plugin
	pluginDir := filepath.Join(tmpDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	// Create manifest
	manifest := `{
		"name": "test-plugin",
		"version": "1.0.0",
		"main": "init.lua"
	}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create plugin code
	code := `
		local activated = false

		function setup(config)
			-- Setup called with config
		end

		function activate()
			activated = true
		end

		function deactivate()
			activated = false
		end

		function is_activated()
			return activated
		end
	`
	if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte(code), 0644); err != nil {
		t.Fatalf("failed to write plugin code: %v", err)
	}

	// Create system with custom plugin path
	config := DefaultSystemConfig()
	config.ManagerConfig.PluginPaths = []string{tmpDir}
	config.ManagerConfig.AutoActivate = false

	sys := NewSystem(config)
	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	// Discover plugins
	plugins, err := sys.Discover()
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name != "test-plugin" {
		t.Errorf("expected plugin name 'test-plugin', got %q", plugins[0].Name)
	}

	// Load plugin
	ctx := context.Background()
	host, err := sys.LoadPlugin(ctx, "test-plugin")
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}
	if host == nil {
		t.Fatal("LoadPlugin returned nil host")
	}

	// Check it's loaded but not active (autoactivate is off)
	if host.State() != StateLoaded {
		t.Errorf("expected state Loaded, got %v", host.State())
	}

	if sys.PluginCount() != 1 {
		t.Errorf("expected 1 plugin loaded, got %d", sys.PluginCount())
	}

	if sys.ActivePluginCount() != 0 {
		t.Errorf("expected 0 active plugins, got %d", sys.ActivePluginCount())
	}

	// Activate through manager
	if err := sys.Manager().Activate(ctx, "test-plugin"); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	if host.State() != StateActive {
		t.Errorf("expected state Active, got %v", host.State())
	}

	if sys.ActivePluginCount() != 1 {
		t.Errorf("expected 1 active plugin, got %d", sys.ActivePluginCount())
	}

	// Check stats
	stats := sys.Stats()
	if stats.TotalPlugins != 1 {
		t.Errorf("stats.TotalPlugins = %d, want 1", stats.TotalPlugins)
	}
	if stats.ActivePlugins != 1 {
		t.Errorf("stats.ActivePlugins = %d, want 1", stats.ActivePlugins)
	}
	if len(stats.PluginStats) != 1 {
		t.Errorf("len(stats.PluginStats) = %d, want 1", len(stats.PluginStats))
	}

	// Unload
	if err := sys.UnloadPlugin(ctx, "test-plugin"); err != nil {
		t.Fatalf("UnloadPlugin failed: %v", err)
	}

	if sys.PluginCount() != 0 {
		t.Errorf("expected 0 plugins after unload, got %d", sys.PluginCount())
	}
}

func TestSystemReloadPlugin(t *testing.T) {
	// Create temp plugin directory
	tmpDir, err := os.MkdirTemp("", "plugin-reload-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple test plugin
	pluginDir := filepath.Join(tmpDir, "reload-test")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := `{"name": "reload-test", "version": "1.0.0", "main": "init.lua"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	code := `version = "v1"`
	if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte(code), 0644); err != nil {
		t.Fatalf("failed to write plugin code: %v", err)
	}

	// Create system
	config := DefaultSystemConfig()
	config.ManagerConfig.PluginPaths = []string{tmpDir}
	config.ManagerConfig.AutoActivate = true

	sys := NewSystem(config)
	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	ctx := context.Background()

	// Load plugin
	host, err := sys.LoadPlugin(ctx, "reload-test")
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	// Check version
	v := host.GetGlobal("version")
	if v != "v1" {
		t.Errorf("expected version v1, got %v", v)
	}

	// Update plugin code
	code = `version = "v2"`
	if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte(code), 0644); err != nil {
		t.Fatalf("failed to update plugin code: %v", err)
	}

	// Reload plugin
	if err := sys.ReloadPlugin(ctx, "reload-test"); err != nil {
		t.Fatalf("ReloadPlugin failed: %v", err)
	}

	// Get new host and check version
	newHost, ok := sys.GetPlugin("reload-test")
	if !ok {
		t.Fatal("plugin not found after reload")
	}

	v = newHost.GetGlobal("version")
	if v != "v2" {
		t.Errorf("expected version v2 after reload, got %v", v)
	}
}

func TestSystemUnloadPluginUninitialized(t *testing.T) {
	sys := NewSystem(DefaultSystemConfig())

	err := sys.UnloadPlugin(context.Background(), "test")
	if err != ErrNotInitialized {
		t.Errorf("UnloadPlugin before init should return ErrNotInitialized, got: %v", err)
	}
}

func TestSystemReloadPluginUninitialized(t *testing.T) {
	sys := NewSystem(DefaultSystemConfig())

	err := sys.ReloadPlugin(context.Background(), "test")
	if err != ErrNotInitialized {
		t.Errorf("ReloadPlugin before init should return ErrNotInitialized, got: %v", err)
	}
}

func TestSystemLoadAllUninitialized(t *testing.T) {
	sys := NewSystem(DefaultSystemConfig())

	err := sys.LoadAll(context.Background())
	if err != ErrNotInitialized {
		t.Errorf("LoadAll before init should return ErrNotInitialized, got: %v", err)
	}
}

func TestSystemConcurrentAccess(t *testing.T) {
	config := DefaultSystemConfig()
	sys := NewSystem(config)

	if err := sys.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer sys.Shutdown(context.Background())

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			sys.IsInitialized()
			sys.PluginCount()
			sys.ActivePluginCount()
			sys.ListPlugins()
			sys.ListActivePlugins()
			sys.HasErrors()
			sys.Errors()
			sys.Stats()
			sys.GetPlugin("nonexistent")
			done <- true
		}()
	}

	// Wait with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("concurrent access test timed out")
		}
	}
}
