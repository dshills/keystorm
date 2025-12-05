package api

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// mockConfigProvider implements ConfigProvider for testing.
type mockConfigProvider struct {
	mu       sync.Mutex
	values   map[string]any
	watches  map[string]watchEntry
	nextID   int
	setError error // Optional error to return from Set
}

type watchEntry struct {
	pattern string
	handler func(key string, oldValue, newValue any)
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{
		values:  make(map[string]any),
		watches: make(map[string]watchEntry),
	}
}

func (m *mockConfigProvider) Get(key string) (any, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	val, ok := m.values[key]
	return val, ok
}

func (m *mockConfigProvider) Set(key string, value any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.setError != nil {
		return m.setError
	}

	oldValue := m.values[key]
	m.values[key] = value

	// Copy handlers to avoid holding lock during callback
	var handlers []func(key string, oldValue, newValue any)
	for _, entry := range m.watches {
		if m.matchPattern(entry.pattern, key) {
			handlers = append(handlers, entry.handler)
		}
	}

	// Release lock before calling handlers
	m.mu.Unlock()

	// Call handlers outside lock
	for _, h := range handlers {
		h(key, oldValue, value)
	}

	// Re-acquire lock (since we deferred Unlock)
	m.mu.Lock()

	return nil
}

func (m *mockConfigProvider) Watch(pattern string, handler func(key string, oldValue, newValue any)) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := fmt.Sprintf("watch-%d", m.nextID)
	m.watches[id] = watchEntry{
		pattern: pattern,
		handler: handler,
	}
	return id
}

func (m *mockConfigProvider) Unwatch(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.watches[id]
	if exists {
		delete(m.watches, id)
	}
	return exists
}

func (m *mockConfigProvider) Keys(pattern string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []string
	for key := range m.values {
		if m.matchPattern(pattern, key) {
			result = append(result, key)
		}
	}
	return result
}

// matchPattern does simple wildcard matching.
func (m *mockConfigProvider) matchPattern(pattern, key string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(key, prefix+".")
	}
	return pattern == key
}

func (m *mockConfigProvider) WatchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.watches)
}

func (m *mockConfigProvider) SetValue(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
}

func setupConfigTest(t *testing.T, cp *mockConfigProvider) (*lua.LState, *ConfigModule) {
	t.Helper()

	ctx := &Context{Config: cp}
	mod := NewConfigModule(ctx, "testplugin")

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	return L, mod
}

func TestConfigModuleName(t *testing.T) {
	ctx := &Context{}
	mod := NewConfigModule(ctx, "test")
	if mod.Name() != "config" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "config")
	}
}

func TestConfigModuleCapability(t *testing.T) {
	ctx := &Context{}
	mod := NewConfigModule(ctx, "test")
	if mod.RequiredCapability() != security.CapabilityConfig {
		t.Errorf("RequiredCapability() = %q, want %q", mod.RequiredCapability(), security.CapabilityConfig)
	}
}

func TestConfigGet(t *testing.T) {
	cp := newMockConfigProvider()
	cp.SetValue("editor.tabSize", 4)
	cp.SetValue("editor.theme", "dark")

	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		tab_size = _ks_config.get("editor.tabSize")
		theme = _ks_config.get("editor.theme")
		missing = _ks_config.get("nonexistent")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	tabSize := L.GetGlobal("tab_size")
	if tabSize.(lua.LNumber) != 4 {
		t.Errorf("tab_size = %v, want 4", tabSize)
	}

	theme := L.GetGlobal("theme")
	if theme.(lua.LString) != "dark" {
		t.Errorf("theme = %v, want 'dark'", theme)
	}

	missing := L.GetGlobal("missing")
	if missing != lua.LNil {
		t.Errorf("missing = %v, want nil", missing)
	}
}

func TestConfigGetNestedValues(t *testing.T) {
	cp := newMockConfigProvider()
	cp.SetValue("editor.font", map[string]any{
		"family": "Fira Code",
		"size":   14,
	})

	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		font = _ks_config.get("editor.font")
		family = font.family
		size = font.size
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	family := L.GetGlobal("family")
	if family.(lua.LString) != "Fira Code" {
		t.Errorf("family = %v, want 'Fira Code'", family)
	}

	size := L.GetGlobal("size")
	if size.(lua.LNumber) != 14 {
		t.Errorf("size = %v, want 14", size)
	}
}

func TestConfigSet(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	// Set a value in the plugin's namespace
	err := L.DoString(`
		result = _ks_config.set("plugins.testplugin.timeout", 5000)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LTrue {
		t.Error("set should return true on success")
	}

	// Verify the value was set
	val, ok := cp.Get("plugins.testplugin.timeout")
	if !ok {
		t.Fatal("value should have been set")
	}
	if val.(float64) != 5000 {
		t.Errorf("value = %v, want 5000", val)
	}
}

func TestConfigSetNestedValue(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		_ks_config.set("plugins.testplugin.settings", {
			enabled = true,
			level = 3
		})
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	val, ok := cp.Get("plugins.testplugin.settings")
	if !ok {
		t.Fatal("value should have been set")
	}

	settings := val.(map[string]any)
	if settings["enabled"] != true {
		t.Errorf("enabled = %v, want true", settings["enabled"])
	}
	if settings["level"] != float64(3) {
		t.Errorf("level = %v, want 3", settings["level"])
	}
}

func TestConfigSetOutsideNamespace(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	// Try to set a value outside the plugin's namespace
	err := L.DoString(`
		_ks_config.set("editor.tabSize", 8)
	`)
	if err == nil {
		t.Error("set outside plugin namespace should error")
	}
	if !strings.Contains(err.Error(), "outside plugin namespace") {
		t.Errorf("error should mention namespace restriction, got: %v", err)
	}
}

func TestConfigSetAnotherPluginNamespace(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	// Try to set a value in another plugin's namespace
	err := L.DoString(`
		_ks_config.set("plugins.otherplugin.setting", "value")
	`)
	if err == nil {
		t.Error("set in another plugin's namespace should error")
	}
}

func TestConfigHas(t *testing.T) {
	cp := newMockConfigProvider()
	cp.SetValue("editor.tabSize", 4)

	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		has_tab = _ks_config.has("editor.tabSize")
		has_missing = _ks_config.has("nonexistent")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	hasTab := L.GetGlobal("has_tab")
	if hasTab != lua.LTrue {
		t.Error("has should return true for existing key")
	}

	hasMissing := L.GetGlobal("has_missing")
	if hasMissing != lua.LFalse {
		t.Error("has should return false for missing key")
	}
}

func TestConfigKeys(t *testing.T) {
	cp := newMockConfigProvider()
	cp.SetValue("plugins.testplugin.setting1", "value1")
	cp.SetValue("plugins.testplugin.setting2", "value2")
	cp.SetValue("plugins.other.setting", "value")
	cp.SetValue("editor.tabSize", 4)

	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		keys = _ks_config.keys("plugins.testplugin.*")
		key_count = 0
		for _ in pairs(keys) do key_count = key_count + 1 end
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	keyCount := L.GetGlobal("key_count")
	if keyCount.(lua.LNumber) != 2 {
		t.Errorf("key_count = %v, want 2", keyCount)
	}
}

func TestConfigKeysDefaultNamespace(t *testing.T) {
	cp := newMockConfigProvider()
	cp.SetValue("plugins.testplugin.setting1", "value1")
	cp.SetValue("plugins.testplugin.setting2", "value2")

	L, _ := setupConfigTest(t, cp)

	// Without pattern, should default to plugin's namespace
	err := L.DoString(`
		keys = _ks_config.keys()
		key_count = 0
		for _ in pairs(keys) do key_count = key_count + 1 end
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	keyCount := L.GetGlobal("key_count")
	if keyCount.(lua.LNumber) != 2 {
		t.Errorf("key_count = %v, want 2", keyCount)
	}
}

func TestConfigWatch(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		changed_key = nil
		old_val = nil
		new_val = nil
		watch_id = _ks_config.watch("editor.*", function(key, old, new)
			changed_key = key
			old_val = old
			new_val = new
		end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Check watch ID was returned
	watchID := L.GetGlobal("watch_id")
	if watchID == lua.LNil {
		t.Fatal("watch ID should not be nil")
	}
	if _, ok := watchID.(lua.LString); !ok {
		t.Fatalf("watch ID should be a string, got %T", watchID)
	}

	// Verify watch was created
	if cp.WatchCount() != 1 {
		t.Errorf("watch count = %d, want 1", cp.WatchCount())
	}

	// Trigger a config change
	cp.Set("editor.tabSize", 8)

	// Give the handler time to execute
	time.Sleep(10 * time.Millisecond)

	changedKey := L.GetGlobal("changed_key")
	if changedKey.(lua.LString) != "editor.tabSize" {
		t.Errorf("changed_key = %v, want 'editor.tabSize'", changedKey)
	}

	newVal := L.GetGlobal("new_val")
	if newVal.(lua.LNumber) != 8 {
		t.Errorf("new_val = %v, want 8", newVal)
	}
}

func TestConfigWatchWithOldValue(t *testing.T) {
	cp := newMockConfigProvider()
	cp.SetValue("editor.tabSize", 4)

	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		old_val = nil
		new_val = nil
		_ks_config.watch("editor.tabSize", function(key, old, new)
			old_val = old
			new_val = new
		end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Change value
	cp.Set("editor.tabSize", 8)
	time.Sleep(10 * time.Millisecond)

	oldVal := L.GetGlobal("old_val")
	if oldVal.(lua.LNumber) != 4 {
		t.Errorf("old_val = %v, want 4", oldVal)
	}

	newVal := L.GetGlobal("new_val")
	if newVal.(lua.LNumber) != 8 {
		t.Errorf("new_val = %v, want 8", newVal)
	}
}

func TestConfigUnwatch(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		watch_id = _ks_config.watch("editor.*", function() end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if cp.WatchCount() != 1 {
		t.Fatalf("watch count = %d, want 1", cp.WatchCount())
	}

	// Unwatch
	err = L.DoString(`
		result = _ks_config.unwatch(watch_id)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LTrue {
		t.Error("unwatch should return true for existing watch")
	}

	if cp.WatchCount() != 0 {
		t.Errorf("watch count = %d, want 0", cp.WatchCount())
	}
}

func TestConfigUnwatchNotFound(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		result = _ks_config.unwatch("nonexistent")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LFalse {
		t.Error("unwatch should return false for nonexistent watch")
	}
}

func TestConfigNamespaceConstant(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		ns = _ks_config.namespace
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	ns := L.GetGlobal("ns")
	if ns.(lua.LString) != "testplugin" {
		t.Errorf("namespace = %v, want 'testplugin'", ns)
	}
}

func TestConfigGetEmptyKey(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		_ks_config.get("")
	`)
	if err == nil {
		t.Error("get with empty key should error")
	}
}

func TestConfigSetEmptyKey(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		_ks_config.set("", "value")
	`)
	if err == nil {
		t.Error("set with empty key should error")
	}
}

func TestConfigWatchEmptyPattern(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		_ks_config.watch("", function() end)
	`)
	if err == nil {
		t.Error("watch with empty pattern should error")
	}
}

func TestConfigNilProvider(t *testing.T) {
	ctx := &Context{Config: nil}
	mod := NewConfigModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// get should return nil
	err := L.DoString(`
		val = _ks_config.get("key")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}
	val := L.GetGlobal("val")
	if val != lua.LNil {
		t.Error("get should return nil when provider is nil")
	}

	// has should return false
	err = L.DoString(`
		has = _ks_config.has("key")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}
	has := L.GetGlobal("has")
	if has != lua.LFalse {
		t.Error("has should return false when provider is nil")
	}

	// keys should return empty table
	err = L.DoString(`
		keys = _ks_config.keys()
		count = 0
		for _ in pairs(keys) do count = count + 1 end
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}
	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 0 {
		t.Error("keys should return empty table when provider is nil")
	}

	// set should error
	err = L.DoString(`
		_ks_config.set("plugins.testplugin.key", "value")
	`)
	if err == nil {
		t.Error("set should error when provider is nil")
	}

	// watch should error
	err = L.DoString(`
		_ks_config.watch("*", function() end)
	`)
	if err == nil {
		t.Error("watch should error when provider is nil")
	}

	// unwatch should return false
	err = L.DoString(`
		result = _ks_config.unwatch("any")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}
	result := L.GetGlobal("result")
	if result != lua.LFalse {
		t.Error("unwatch should return false when provider is nil")
	}
}

func TestConfigCleanup(t *testing.T) {
	cp := newMockConfigProvider()
	L, mod := setupConfigTest(t, cp)

	// Create multiple watches
	err := L.DoString(`
		_ks_config.watch("editor.*", function() end)
		_ks_config.watch("plugins.*", function() end)
		_ks_config.watch("theme.*", function() end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	if cp.WatchCount() != 3 {
		t.Fatalf("watch count = %d, want 3", cp.WatchCount())
	}

	// Cleanup should unwatch all
	mod.Cleanup()

	if cp.WatchCount() != 0 {
		t.Errorf("watch count after cleanup = %d, want 0", cp.WatchCount())
	}
}

func TestConfigGetArrayValue(t *testing.T) {
	cp := newMockConfigProvider()
	cp.SetValue("editor.rulers", []any{80, 120, 160})

	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		rulers = _ks_config.get("editor.rulers")
		count = #rulers
		first = rulers[1]
		last = rulers[3]
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 3 {
		t.Errorf("count = %v, want 3", count)
	}

	first := L.GetGlobal("first")
	if first.(lua.LNumber) != 80 {
		t.Errorf("first = %v, want 80", first)
	}

	last := L.GetGlobal("last")
	if last.(lua.LNumber) != 160 {
		t.Errorf("last = %v, want 160", last)
	}
}

func TestConfigSetArrayValue(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		_ks_config.set("plugins.testplugin.tags", {"tag1", "tag2", "tag3"})
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	val, ok := cp.Get("plugins.testplugin.tags")
	if !ok {
		t.Fatal("value should have been set")
	}

	arr := val.([]any)
	if len(arr) != 3 {
		t.Errorf("array length = %d, want 3", len(arr))
	}
	if arr[0] != "tag1" {
		t.Errorf("arr[0] = %v, want 'tag1'", arr[0])
	}
}

func TestConfigMultipleWatches(t *testing.T) {
	cp := newMockConfigProvider()
	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		count1 = 0
		count2 = 0
		_ks_config.watch("editor.tabSize", function() count1 = count1 + 1 end)
		_ks_config.watch("editor.*", function() count2 = count2 + 1 end)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Trigger config change
	cp.Set("editor.tabSize", 4)
	time.Sleep(10 * time.Millisecond)

	count1 := L.GetGlobal("count1")
	count2 := L.GetGlobal("count2")

	// Both watches should fire
	if count1.(lua.LNumber) != 1 {
		t.Errorf("count1 = %v, want 1", count1)
	}
	if count2.(lua.LNumber) != 1 {
		t.Errorf("count2 = %v, want 1", count2)
	}
}

func TestConfigGetBoolValue(t *testing.T) {
	cp := newMockConfigProvider()
	cp.SetValue("editor.wordWrap", true)
	cp.SetValue("editor.lineNumbers", false)

	L, _ := setupConfigTest(t, cp)

	err := L.DoString(`
		wrap = _ks_config.get("editor.wordWrap")
		numbers = _ks_config.get("editor.lineNumbers")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	wrap := L.GetGlobal("wrap")
	if wrap != lua.LTrue {
		t.Errorf("wrap = %v, want true", wrap)
	}

	numbers := L.GetGlobal("numbers")
	if numbers != lua.LFalse {
		t.Errorf("numbers = %v, want false", numbers)
	}
}
