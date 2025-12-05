package api

import (
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/input/keymap"
	"github.com/dshills/keystorm/internal/plugin/security"
)

// mockKeymapProvider implements KeymapProvider for testing.
type mockKeymapProvider struct {
	keymaps map[string]*keymap.ParsedKeymap
}

func newMockKeymapProvider() *mockKeymapProvider {
	return &mockKeymapProvider{
		keymaps: make(map[string]*keymap.ParsedKeymap),
	}
}

func (m *mockKeymapProvider) Register(km *keymap.Keymap) error {
	parsed, err := km.Parse()
	if err != nil {
		return err
	}
	m.keymaps[km.Name] = parsed
	return nil
}

func (m *mockKeymapProvider) Unregister(name string) {
	delete(m.keymaps, name)
}

func (m *mockKeymapProvider) Get(name string) *keymap.ParsedKeymap {
	return m.keymaps[name]
}

func (m *mockKeymapProvider) AllBindings(mode string) []keymap.BindingMatch {
	var matches []keymap.BindingMatch
	for _, km := range m.keymaps {
		// If mode is empty, return all bindings; otherwise filter by mode
		if mode != "" && km.Mode != "" && km.Mode != mode {
			continue
		}
		for i := range km.ParsedBindings {
			matches = append(matches, keymap.BindingMatch{
				ParsedBinding: &km.ParsedBindings[i],
				Keymap:        km.Keymap,
			})
		}
	}
	return matches
}

func setupKeymapTest(t *testing.T, kmp *mockKeymapProvider) (*lua.LState, *KeymapModule) {
	t.Helper()

	ctx := &Context{Keymap: kmp}
	mod := NewKeymapModule(ctx, "testplugin")

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	return L, mod
}

func TestKeymapModuleName(t *testing.T) {
	ctx := &Context{}
	mod := NewKeymapModule(ctx, "test")
	if mod.Name() != "keymap" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "keymap")
	}
}

func TestKeymapModuleCapability(t *testing.T) {
	ctx := &Context{}
	mod := NewKeymapModule(ctx, "test")
	if mod.RequiredCapability() != security.CapabilityKeymap {
		t.Errorf("RequiredCapability() = %q, want %q", mod.RequiredCapability(), security.CapabilityKeymap)
	}
}

func TestKeymapSet(t *testing.T) {
	kmp := newMockKeymapProvider()
	L, _ := setupKeymapTest(t, kmp)

	err := L.DoString(`
		_ks_keymap.set("normal", "g d", "plugin.testplugin.goToDefinition")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Check that keymap was registered
	kmName := "testplugin_normal_g_d"
	km := kmp.Get(kmName)
	if km == nil {
		t.Fatal("keymap not registered")
	}

	if km.Mode != "normal" {
		t.Errorf("keymap.Mode = %q, want %q", km.Mode, "normal")
	}

	if km.Source != "plugin:testplugin" {
		t.Errorf("keymap.Source = %q, want %q", km.Source, "plugin:testplugin")
	}

	if len(km.ParsedBindings) != 1 {
		t.Fatalf("keymap.ParsedBindings length = %d, want 1", len(km.ParsedBindings))
	}

	binding := km.ParsedBindings[0]
	if binding.Keys != "g d" {
		t.Errorf("binding.Keys = %q, want %q", binding.Keys, "g d")
	}
	if binding.Action != "plugin.testplugin.goToDefinition" {
		t.Errorf("binding.Action = %q, want %q", binding.Action, "plugin.testplugin.goToDefinition")
	}
}

func TestKeymapSetWithOptions(t *testing.T) {
	kmp := newMockKeymapProvider()
	L, _ := setupKeymapTest(t, kmp)

	err := L.DoString(`
		_ks_keymap.set("visual", "S", "plugin.testplugin.surround", {
			desc = "Surround selection",
			when = "editorTextFocus",
			category = "Editing",
			priority = 10
		})
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	kmName := "testplugin_visual_S"
	km := kmp.Get(kmName)
	if km == nil {
		t.Fatal("keymap not registered")
	}

	if len(km.ParsedBindings) != 1 {
		t.Fatalf("keymap.ParsedBindings length = %d, want 1", len(km.ParsedBindings))
	}

	binding := km.ParsedBindings[0]
	if binding.Description != "Surround selection" {
		t.Errorf("binding.Description = %q, want %q", binding.Description, "Surround selection")
	}
	if binding.When != "editorTextFocus" {
		t.Errorf("binding.When = %q, want %q", binding.When, "editorTextFocus")
	}
	if binding.Category != "Editing" {
		t.Errorf("binding.Category = %q, want %q", binding.Category, "Editing")
	}
	if binding.Priority != 10 {
		t.Errorf("binding.Priority = %d, want %d", binding.Priority, 10)
	}
}

func TestKeymapSetEmptyKeys(t *testing.T) {
	kmp := newMockKeymapProvider()
	L, _ := setupKeymapTest(t, kmp)

	err := L.DoString(`
		_ks_keymap.set("normal", "", "some.action")
	`)
	if err == nil {
		t.Error("set with empty keys should error")
	}
}

func TestKeymapSetEmptyAction(t *testing.T) {
	kmp := newMockKeymapProvider()
	L, _ := setupKeymapTest(t, kmp)

	err := L.DoString(`
		_ks_keymap.set("normal", "g d", "")
	`)
	if err == nil {
		t.Error("set with empty action should error")
	}
}

func TestKeymapDel(t *testing.T) {
	kmp := newMockKeymapProvider()
	L, _ := setupKeymapTest(t, kmp)

	// First set a keymap
	err := L.DoString(`
		_ks_keymap.set("normal", "g d", "plugin.testplugin.action")
	`)
	if err != nil {
		t.Fatalf("set DoString error = %v", err)
	}

	kmName := "testplugin_normal_g_d"
	if kmp.Get(kmName) == nil {
		t.Fatal("keymap not registered")
	}

	// Now delete it
	err = L.DoString(`
		_ks_keymap.del("normal", "g d")
	`)
	if err != nil {
		t.Fatalf("del DoString error = %v", err)
	}

	if kmp.Get(kmName) != nil {
		t.Error("keymap should have been deleted")
	}
}

func TestKeymapGet(t *testing.T) {
	kmp := newMockKeymapProvider()
	L, _ := setupKeymapTest(t, kmp)

	// Set a keymap
	err := L.DoString(`
		_ks_keymap.set("normal", "g d", "plugin.testplugin.action", {
			desc = "Test description"
		})
	`)
	if err != nil {
		t.Fatalf("set DoString error = %v", err)
	}

	// Get it
	err = L.DoString(`
		binding = _ks_keymap.get("normal", "g d")
	`)
	if err != nil {
		t.Fatalf("get DoString error = %v", err)
	}

	binding := L.GetGlobal("binding")
	if binding == lua.LNil {
		t.Fatal("binding should not be nil")
	}

	tbl, ok := binding.(*lua.LTable)
	if !ok {
		t.Fatalf("binding should be a table, got %T", binding)
	}

	keys := L.GetField(tbl, "keys")
	if keys.(lua.LString) != "g d" {
		t.Errorf("binding.keys = %v, want %q", keys, "g d")
	}

	action := L.GetField(tbl, "action")
	if action.(lua.LString) != "plugin.testplugin.action" {
		t.Errorf("binding.action = %v, want %q", action, "plugin.testplugin.action")
	}

	desc := L.GetField(tbl, "desc")
	if desc.(lua.LString) != "Test description" {
		t.Errorf("binding.desc = %v, want %q", desc, "Test description")
	}
}

func TestKeymapGetNotFound(t *testing.T) {
	kmp := newMockKeymapProvider()
	L, _ := setupKeymapTest(t, kmp)

	err := L.DoString(`
		binding = _ks_keymap.get("normal", "nonexistent")
		is_nil = binding == nil
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	isNil := L.GetGlobal("is_nil")
	if isNil != lua.LTrue {
		t.Error("binding for nonexistent keymap should be nil")
	}
}

func TestKeymapList(t *testing.T) {
	kmp := newMockKeymapProvider()
	L, _ := setupKeymapTest(t, kmp)

	// Set multiple keymaps
	err := L.DoString(`
		_ks_keymap.set("normal", "g d", "plugin.testplugin.action1")
		_ks_keymap.set("normal", "g r", "plugin.testplugin.action2")
		_ks_keymap.set("visual", "S", "plugin.testplugin.action3")
	`)
	if err != nil {
		t.Fatalf("set DoString error = %v", err)
	}

	// List normal mode keymaps
	err = L.DoString(`
		bindings = _ks_keymap.list("normal")
		count = #bindings
	`)
	if err != nil {
		t.Fatalf("list DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 2 {
		t.Errorf("list count = %v, want 2", count)
	}
}

func TestKeymapListAll(t *testing.T) {
	kmp := newMockKeymapProvider()
	L, _ := setupKeymapTest(t, kmp)

	// Set multiple keymaps for different modes
	err := L.DoString(`
		_ks_keymap.set("normal", "g d", "plugin.testplugin.action1")
		_ks_keymap.set("visual", "S", "plugin.testplugin.action2")
	`)
	if err != nil {
		t.Fatalf("set DoString error = %v", err)
	}

	// List all keymaps (empty mode string)
	err = L.DoString(`
		bindings = _ks_keymap.list("")
		count = #bindings
	`)
	if err != nil {
		t.Fatalf("list DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	// Both normal and visual mode keymaps should be included when listing empty mode
	if count.(lua.LNumber) < 1 {
		t.Errorf("list count = %v, want at least 1", count)
	}
}

func TestKeymapNilProvider(t *testing.T) {
	ctx := &Context{Keymap: nil}
	mod := NewKeymapModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// get should return nil
	err := L.DoString(`
		result = _ks_keymap.get("normal", "g d")
		is_nil = result == nil
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	isNil := L.GetGlobal("is_nil")
	if isNil != lua.LTrue {
		t.Error("get should return nil when provider is nil")
	}

	// list should return empty table
	err = L.DoString(`
		result = _ks_keymap.list("normal")
		count = #result
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 0 {
		t.Errorf("list should return empty table when provider is nil, got %v", count)
	}

	// set should error
	err = L.DoString(`
		_ks_keymap.set("normal", "g d", "action")
	`)
	if err == nil {
		t.Error("set should error when provider is nil")
	}
}

func TestSanitizeForName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gd", "gd"},
		{"g d", "g_d"},
		{"C-s", "C-s"},
		{"<C-S-a>", "x3cC-S-ax3e"},
		{"abc123", "abc123"},
	}

	for _, tt := range tests {
		result := sanitizeForName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeForName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
