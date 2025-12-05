package plugin

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func createTestPluginDir(t *testing.T, dir, luaCode string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Use basename as plugin name in manifest
	name := filepath.Base(dir)

	// Write manifest
	manifest := `{
		"name": "` + name + `",
		"version": "1.0.0",
		"displayName": "Test Plugin",
		"main": "init.lua"
	}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Write Lua file
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(luaCode), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestNewManager(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewManager(config)

	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.loader == nil {
		t.Error("Manager.loader is nil")
	}
	if m.plugins == nil {
		t.Error("Manager.plugins is nil")
	}
}

func TestManagerDiscover(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "test-plugin"), "-- test plugin")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)

	plugins, err := m.Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(plugins) != 1 {
		t.Errorf("Discover() returned %d plugins, want 1", len(plugins))
	}
	if plugins[0].Name != "test-plugin" {
		t.Errorf("Plugin name = %q, want %q", plugins[0].Name, "test-plugin")
	}
}

func TestManagerLoad(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "test-plugin"), "-- loaded")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)
	m.Discover()

	ctx := context.Background()
	host, err := m.Load(ctx, "test-plugin")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if host == nil {
		t.Fatal("Load() returned nil host")
	}
	if host.State() != StateLoaded {
		t.Errorf("Host state = %v, want %v", host.State(), StateLoaded)
	}
	if m.Count() != 1 {
		t.Errorf("Count() = %d, want 1", m.Count())
	}
}

func TestManagerLoadAutoActivate(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "test-plugin"), "-- auto activate")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: true,
	}
	m := NewManager(config)
	m.Discover()

	ctx := context.Background()
	host, err := m.Load(ctx, "test-plugin")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if host.State() != StateActive {
		t.Errorf("Host state = %v, want %v (auto-activated)", host.State(), StateActive)
	}
}

func TestManagerLoadAlreadyLoaded(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "test-plugin"), "-- test")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)
	m.Discover()

	ctx := context.Background()
	_, err := m.Load(ctx, "test-plugin")
	if err != nil {
		t.Fatalf("First Load() error = %v", err)
	}

	_, err = m.Load(ctx, "test-plugin")
	if err == nil {
		t.Error("Second Load() should return error")
	}
}

func TestManagerLoadNotFound(t *testing.T) {
	config := ManagerConfig{
		PluginPaths:  []string{t.TempDir()},
		AutoActivate: false,
	}
	m := NewManager(config)

	ctx := context.Background()
	_, err := m.Load(ctx, "nonexistent")
	if err == nil {
		t.Error("Load() with nonexistent plugin should return error")
	}
}

func TestManagerUnload(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "test-plugin"), "-- unload test")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)
	m.Discover()

	ctx := context.Background()
	m.Load(ctx, "test-plugin")

	err := m.Unload(ctx, "test-plugin")
	if err != nil {
		t.Fatalf("Unload() error = %v", err)
	}

	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0", m.Count())
	}
}

func TestManagerUnloadNotFound(t *testing.T) {
	config := ManagerConfig{
		PluginPaths:  []string{t.TempDir()},
		AutoActivate: false,
	}
	m := NewManager(config)

	ctx := context.Background()
	err := m.Unload(ctx, "nonexistent")
	if err == nil {
		t.Error("Unload() with nonexistent plugin should return error")
	}
}

func TestManagerLoadAll(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-a"), "-- plugin a")
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-b"), "-- plugin b")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)

	ctx := context.Background()
	err := m.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if m.Count() != 2 {
		t.Errorf("Count() = %d, want 2", m.Count())
	}
}

func TestManagerUnloadAll(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-a"), "-- plugin a")
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-b"), "-- plugin b")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)

	ctx := context.Background()
	m.LoadAll(ctx)

	err := m.UnloadAll(ctx)
	if err != nil {
		t.Fatalf("UnloadAll() error = %v", err)
	}

	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0", m.Count())
	}
}

func TestManagerGet(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "test-plugin"), "-- get test")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)
	m.Discover()

	ctx := context.Background()
	m.Load(ctx, "test-plugin")

	host, ok := m.Get("test-plugin")
	if !ok {
		t.Error("Get() ok = false, want true")
	}
	if host == nil {
		t.Error("Get() returned nil host")
	}

	_, ok = m.Get("nonexistent")
	if ok {
		t.Error("Get() with nonexistent should return ok = false")
	}
}

func TestManagerList(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-a"), "-- plugin a")
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-b"), "-- plugin b")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)

	ctx := context.Background()
	m.LoadAll(ctx)

	list := m.List()
	if len(list) != 2 {
		t.Errorf("List() returned %d items, want 2", len(list))
	}
}

func TestManagerListActive(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-a"), "-- plugin a")
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-b"), "-- plugin b")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)

	ctx := context.Background()
	m.LoadAll(ctx)

	// None active yet
	active := m.ListActive()
	if len(active) != 0 {
		t.Errorf("ListActive() returned %d items, want 0", len(active))
	}

	// Activate one
	m.Activate(ctx, "plugin-a")
	active = m.ListActive()
	if len(active) != 1 {
		t.Errorf("ListActive() returned %d items, want 1", len(active))
	}
}

func TestManagerActivateDeactivate(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "test-plugin"), "-- activate test")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)
	m.Discover()

	ctx := context.Background()
	m.Load(ctx, "test-plugin")

	// Activate
	err := m.Activate(ctx, "test-plugin")
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	host, _ := m.Get("test-plugin")
	if host.State() != StateActive {
		t.Errorf("State = %v, want %v", host.State(), StateActive)
	}

	// Deactivate
	err = m.Deactivate(ctx, "test-plugin")
	if err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}

	if host.State() != StateLoaded {
		t.Errorf("State = %v, want %v", host.State(), StateLoaded)
	}
}

func TestManagerActivateAll(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-a"), "-- plugin a")
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-b"), "-- plugin b")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)

	ctx := context.Background()
	m.LoadAll(ctx)

	err := m.ActivateAll(ctx)
	if err != nil {
		t.Fatalf("ActivateAll() error = %v", err)
	}

	if m.CountActive() != 2 {
		t.Errorf("CountActive() = %d, want 2", m.CountActive())
	}
}

func TestManagerDeactivateAll(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-a"), "-- plugin a")
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-b"), "-- plugin b")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: true,
	}
	m := NewManager(config)

	ctx := context.Background()
	m.LoadAll(ctx)

	err := m.DeactivateAll(ctx)
	if err != nil {
		t.Fatalf("DeactivateAll() error = %v", err)
	}

	if m.CountActive() != 0 {
		t.Errorf("CountActive() = %d, want 0", m.CountActive())
	}
}

func TestManagerReload(t *testing.T) {
	pluginsDir := t.TempDir()
	pluginDir := createTestPluginDir(t, filepath.Join(pluginsDir, "test-plugin"), "answer = 42")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: true,
	}
	m := NewManager(config)
	m.Discover()

	ctx := context.Background()
	m.Load(ctx, "test-plugin")

	// Modify the plugin
	os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("answer = 100"), 0644)

	// Reload
	err := m.Reload(ctx, "test-plugin")
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	host, _ := m.Get("test-plugin")
	if host == nil {
		t.Fatal("Plugin not found after reload")
	}

	// Check the new value
	answer := host.GetGlobal("answer")
	if answer != int64(100) {
		t.Errorf("answer = %v, want 100", answer)
	}
}

func TestManagerSubscribe(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "test-plugin"), "-- events test")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)
	m.Discover()

	var mu sync.Mutex
	events := make([]ManagerEvent, 0)
	m.Subscribe(func(event ManagerEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	ctx := context.Background()
	m.Load(ctx, "test-plugin")
	m.Activate(ctx, "test-plugin")
	m.Deactivate(ctx, "test-plugin")
	m.Unload(ctx, "test-plugin")

	// Give time for events to be processed
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	expectedTypes := []ManagerEventType{
		EventPluginLoaded,
		EventPluginActivated,
		EventPluginDeactivated,
		EventPluginUnloaded,
	}

	if len(events) != len(expectedTypes) {
		t.Errorf("Got %d events, want %d", len(events), len(expectedTypes))
	}

	for i, expected := range expectedTypes {
		if i < len(events) && events[i].Type != expected {
			t.Errorf("Event %d type = %v, want %v", i, events[i].Type, expected)
		}
	}
}

func TestManagerEventTypeString(t *testing.T) {
	tests := []struct {
		eventType ManagerEventType
		expected  string
	}{
		{EventPluginLoaded, "loaded"},
		{EventPluginUnloaded, "unloaded"},
		{EventPluginActivated, "activated"},
		{EventPluginDeactivated, "deactivated"},
		{EventPluginReloaded, "reloaded"},
		{EventPluginError, "error"},
		{ManagerEventType(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.eventType.String(); got != tt.expected {
			t.Errorf("ManagerEventType(%d).String() = %q, want %q", tt.eventType, got, tt.expected)
		}
	}
}

func TestManagerCount(t *testing.T) {
	pluginsDir := t.TempDir()
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-a"), "-- plugin a")
	createTestPluginDir(t, filepath.Join(pluginsDir, "plugin-b"), "-- plugin b")

	config := ManagerConfig{
		PluginPaths:  []string{pluginsDir},
		AutoActivate: false,
	}
	m := NewManager(config)

	if m.Count() != 0 {
		t.Errorf("Initial Count() = %d, want 0", m.Count())
	}

	ctx := context.Background()
	m.LoadAll(ctx)

	if m.Count() != 2 {
		t.Errorf("Count() after LoadAll = %d, want 2", m.Count())
	}
}

func TestManagerLoader(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewManager(config)

	loader := m.Loader()
	if loader == nil {
		t.Error("Loader() returned nil")
	}
}
