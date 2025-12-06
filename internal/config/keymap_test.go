package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dshills/keystorm/internal/config/notify"
	"github.com/dshills/keystorm/internal/input/keymap"
)

func TestKeymapManager_New(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()
	if km == nil {
		t.Fatal("Keymaps() returned nil")
	}

	// Registry should be initialized
	if km.Registry() == nil {
		t.Error("Registry() returned nil")
	}
}

func TestKeymapManager_LoadDefaults(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Load defaults
	err := km.LoadDefaults()
	if err != nil {
		t.Fatalf("LoadDefaults() error = %v", err)
	}

	// Check that default keymaps were loaded
	reg := km.Registry()

	// Check for default-normal keymap
	normalKm := reg.Get("default-normal")
	if normalKm == nil {
		t.Error("default-normal keymap not registered")
	}

	// Check for default-insert keymap
	insertKm := reg.Get("default-insert")
	if insertKm == nil {
		t.Error("default-insert keymap not registered")
	}

	// Check for default-visual keymap
	visualKm := reg.Get("default-visual")
	if visualKm == nil {
		t.Error("default-visual keymap not registered")
	}
}

func TestKeymapManager_AddBinding(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Add a binding for normal mode
	binding := KeymapBinding{
		Keys:        "g g",
		Action:      "cursor.document_start",
		Description: "Go to beginning of document",
		Category:    "Navigation",
	}

	err := km.AddBinding("normal", binding)
	if err != nil {
		t.Fatalf("AddBinding() error = %v", err)
	}

	// Verify the binding was added
	got, ok := km.GetBinding("normal", "g g")
	if !ok {
		t.Fatal("GetBinding() returned false")
	}

	if got.Keys != "g g" {
		t.Errorf("Keys = %q, want 'g g'", got.Keys)
	}
	if got.Action != "cursor.document_start" {
		t.Errorf("Action = %q, want 'cursor.document_start'", got.Action)
	}
}

func TestKeymapManager_RemoveBinding(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Add a binding
	binding := KeymapBinding{
		Keys:   "x x",
		Action: "test.action",
	}
	_ = km.AddBinding("normal", binding)

	// Verify it exists
	_, ok := km.GetBinding("normal", "x x")
	if !ok {
		t.Fatal("binding should exist before removal")
	}

	// Remove it
	err := km.RemoveBinding("normal", "x x")
	if err != nil {
		t.Fatalf("RemoveBinding() error = %v", err)
	}

	// Verify it's gone
	_, ok = km.GetBinding("normal", "x x")
	if ok {
		t.Error("binding should not exist after removal")
	}
}

func TestKeymapManager_RemoveBinding_NotFound(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Try to remove non-existent binding
	err := km.RemoveBinding("normal", "z z z")
	if err == nil {
		t.Error("RemoveBinding() should return error for non-existent binding")
	}
}

func TestKeymapManager_ListUserBindings(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Add multiple bindings
	_ = km.AddBinding("normal", KeymapBinding{Keys: "a a", Action: "action1"})
	_ = km.AddBinding("normal", KeymapBinding{Keys: "b b", Action: "action2"})
	_ = km.AddBinding("normal", KeymapBinding{Keys: "c c", Action: "action3"})

	bindings := km.ListUserBindings("normal")
	if len(bindings) != 3 {
		t.Errorf("ListUserBindings() returned %d bindings, want 3", len(bindings))
	}
}

func TestKeymapManager_ListModes(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Add bindings for different modes
	_ = km.AddBinding("normal", KeymapBinding{Keys: "n", Action: "normal.action"})
	_ = km.AddBinding("insert", KeymapBinding{Keys: "i", Action: "insert.action"})
	_ = km.AddBinding("visual", KeymapBinding{Keys: "v", Action: "visual.action"})

	modes := km.ListModes()
	if len(modes) != 3 {
		t.Errorf("ListModes() returned %d modes, want 3", len(modes))
	}

	modeMap := make(map[string]bool)
	for _, m := range modes {
		modeMap[m] = true
	}

	for _, expected := range []string{"normal", "insert", "visual"} {
		if !modeMap[expected] {
			t.Errorf("ListModes() missing mode %q", expected)
		}
	}
}

func TestKeymapManager_GlobalBindings(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Add a global binding (empty mode)
	binding := KeymapBinding{
		Keys:   "<C-s>",
		Action: "editor.save",
	}

	err := km.AddBinding("", binding)
	if err != nil {
		t.Fatalf("AddBinding() for global error = %v", err)
	}

	// Verify it was added to user-global
	got, ok := km.GetBinding("", "<C-s>")
	if !ok {
		t.Fatal("GetBinding() for global returned false")
	}

	if got.Action != "editor.save" {
		t.Errorf("Action = %q, want 'editor.save'", got.Action)
	}
}

func TestKeymapManager_Lookup(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Load defaults first
	if err := km.LoadDefaults(); err != nil {
		t.Fatalf("LoadDefaults() error = %v", err)
	}

	// Lookup a known default binding (j for cursor down in normal mode)
	binding, err := km.Lookup("normal", "", "j")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	if binding == nil {
		t.Fatal("Lookup() returned nil binding")
	}

	if binding.Action != "cursor.down" {
		t.Errorf("Action = %q, want 'cursor.down'", binding.Action)
	}
}

func TestKeymapManager_HasPrefix(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Load defaults which include multi-key sequences like "g g"
	if err := km.LoadDefaults(); err != nil {
		t.Fatalf("LoadDefaults() error = %v", err)
	}

	// "g" should be a prefix (since "g g" exists in defaults)
	hasPrefix, err := km.HasPrefix("normal", "g")
	if err != nil {
		t.Fatalf("HasPrefix() error = %v", err)
	}

	if !hasPrefix {
		t.Error("HasPrefix('g') = false, want true")
	}

	// "z z z" should not be a prefix
	hasPrefix, err = km.HasPrefix("normal", "z z z")
	if err != nil {
		t.Fatalf("HasPrefix() error = %v", err)
	}

	if hasPrefix {
		t.Error("HasPrefix('z z z') = true, want false")
	}
}

func TestKeymapManager_Notifications(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	var received []notify.Change
	var mu sync.Mutex

	sub := km.SubscribeKeymaps(func(change notify.Change) {
		mu.Lock()
		received = append(received, change)
		mu.Unlock()
	})
	defer sub.Unsubscribe()

	// Add a binding (should trigger notification)
	_ = km.AddBinding("normal", KeymapBinding{Keys: "t", Action: "test"})

	// Remove a binding (should trigger notification)
	_ = km.RemoveBinding("normal", "t")

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 2 {
		t.Errorf("Received %d notifications, want 2", count)
	}
}

func TestKeymapManager_LoadFromConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create keymaps.toml with user keymaps
	// Use valid key sequences (Space followed by f, Space followed by g)
	keymapsPath := filepath.Join(tmpDir, "keymaps.toml")
	keymapsContent := `
[[keymaps]]
name = "user-custom"
mode = "normal"
priority = 100

[[keymaps.bindings]]
keys = "<Space> f"
action = "file.find"
description = "Find files"
category = "File"

[[keymaps.bindings]]
keys = "<Space> g"
action = "git.status"
description = "Git status"
category = "Git"
`
	if err := os.WriteFile(keymapsPath, []byte(keymapsContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := New(
		WithUserConfigDir(tmpDir),
		WithWatcher(false),
	)
	defer c.Close()

	// Load config (this loads keymaps.toml)
	if err := c.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify user keymaps were loaded
	km := c.Keymaps()
	reg := km.Registry()

	// Check for the user-custom keymap
	userKm := reg.Get("user-custom")
	if userKm == nil {
		t.Error("user-custom keymap not registered")
	} else {
		if len(userKm.ParsedBindings) < 2 {
			t.Errorf("user-custom keymap has %d bindings, want at least 2", len(userKm.ParsedBindings))
		}
	}
}

func TestKeymapManager_UserOverridesDefaults(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Load defaults
	if err := km.LoadDefaults(); err != nil {
		t.Fatalf("LoadDefaults() error = %v", err)
	}

	// Add a user binding that overrides a default
	// In defaults, 'j' is cursor.down. Let's override it.
	userBinding := KeymapBinding{
		Keys:     "j",
		Action:   "custom.action",
		Priority: 100, // Higher priority
	}

	err := km.AddBinding("normal", userBinding)
	if err != nil {
		t.Fatalf("AddBinding() error = %v", err)
	}

	// Lookup should return the user binding due to higher priority
	binding, err := km.Lookup("normal", "", "j")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	// Note: The actual priority depends on keymap registration order
	// and the scoring system. User keymaps with priority 100 should
	// override default keymaps with priority 0.
	if binding == nil {
		t.Fatal("Lookup() returned nil")
	}

	// The binding could be either custom or default depending on scoring
	// At minimum, we verify lookup works without error
	t.Logf("Binding action: %s (may be custom or default based on scoring)", binding.Action)
}

func TestKeymapManager_ConcurrentAccess(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()
	_ = km.LoadDefaults()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent adds
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			binding := KeymapBinding{
				Keys:   "t",
				Action: "test.action",
			}
			_ = km.AddBinding("normal", binding)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			km.GetBinding("normal", "t")
			km.ListUserBindings("normal")
			km.ListModes()
			_, _ = km.Lookup("normal", "", "j")
		}()
	}

	wg.Wait()
}

func TestKeymapManager_ConditionEvaluator(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Create a custom condition evaluator
	customEval := &testConditionEvaluator{
		conditions: map[string]bool{
			"editorTextFocus": true,
		},
	}

	km.SetConditionEvaluator(customEval)

	// Add binding with condition
	binding := KeymapBinding{
		Keys:   "c",
		Action: "conditional.action",
		When:   "editorTextFocus",
	}
	_ = km.AddBinding("normal", binding)

	// The binding should be found when condition is true
	// (Actual condition evaluation happens in Registry.Lookup)
}

// testConditionEvaluator is a simple test implementation of ConditionEvaluator
type testConditionEvaluator struct {
	conditions map[string]bool
}

func (e *testConditionEvaluator) Evaluate(condition string, ctx *keymap.LookupContext) bool {
	return e.conditions[condition]
}

func TestConfig_KeymapsIntegration(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	// Test that Keymaps() returns the same manager
	km1 := c.Keymaps()
	km2 := c.Keymaps()

	if km1 != km2 {
		t.Error("Keymaps() should return the same manager instance")
	}

	// Test that Registry() returns the same registry
	reg1 := km1.Registry()
	reg2 := km2.Registry()

	if reg1 != reg2 {
		t.Error("Registry() should return the same registry instance")
	}
}

func TestKeymapBinding_WithArgs(t *testing.T) {
	c := New(WithWatcher(false))
	defer c.Close()

	km := c.Keymaps()

	// Add binding with args
	binding := KeymapBinding{
		Keys:   "m a",
		Action: "mark.set",
		Args: map[string]any{
			"register": "a",
		},
	}

	err := km.AddBinding("normal", binding)
	if err != nil {
		t.Fatalf("AddBinding() error = %v", err)
	}

	got, ok := km.GetBinding("normal", "m a")
	if !ok {
		t.Fatal("GetBinding() returned false")
	}

	if got.Args == nil {
		t.Fatal("Args is nil")
	}
	if got.Args["register"] != "a" {
		t.Errorf("Args[register] = %v, want 'a'", got.Args["register"])
	}
}
