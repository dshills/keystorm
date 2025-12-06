package layer

import (
	"testing"
)

func TestManager_AddLayer(t *testing.T) {
	m := NewManager()

	m.AddLayer(NewLayer("default", SourceBuiltin, PriorityBuiltin))
	m.AddLayer(NewLayer("user", SourceUserGlobal, PriorityUserGlobal))
	m.AddLayer(NewLayer("workspace", SourceWorkspace, PriorityWorkspace))

	if m.LayerCount() != 3 {
		t.Errorf("LayerCount() = %d, want 3", m.LayerCount())
	}

	// Verify sorted by priority
	layers := m.Layers()
	if layers[0].Name != "default" {
		t.Error("first layer should be 'default' (lowest priority)")
	}
	if layers[1].Name != "user" {
		t.Error("second layer should be 'user'")
	}
	if layers[2].Name != "workspace" {
		t.Error("third layer should be 'workspace' (highest priority)")
	}
}

func TestManager_RemoveLayer(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayer("test1", SourceBuiltin, PriorityBuiltin))
	m.AddLayer(NewLayer("test2", SourceUserGlobal, PriorityUserGlobal))

	// Remove existing
	if !m.RemoveLayer("test1") {
		t.Error("RemoveLayer should return true for existing layer")
	}
	if m.LayerCount() != 1 {
		t.Errorf("LayerCount() = %d, want 1", m.LayerCount())
	}

	// Remove non-existing
	if m.RemoveLayer("nonexistent") {
		t.Error("RemoveLayer should return false for non-existing layer")
	}
}

func TestManager_GetLayer(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayer("test", SourceBuiltin, PriorityBuiltin))

	layer := m.GetLayer("test")
	if layer == nil {
		t.Fatal("GetLayer should return the layer")
	}
	if layer.Name != "test" {
		t.Errorf("layer.Name = %q, want 'test'", layer.Name)
	}

	// Non-existing
	if m.GetLayer("nonexistent") != nil {
		t.Error("GetLayer should return nil for non-existing layer")
	}
}

func TestManager_GetLayerBySource(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayer("user", SourceUserGlobal, PriorityUserGlobal))
	m.AddLayer(NewLayer("workspace", SourceWorkspace, PriorityWorkspace))

	layer := m.GetLayerBySource(SourceUserGlobal)
	if layer == nil {
		t.Fatal("GetLayerBySource should return the layer")
	}
	if layer.Name != "user" {
		t.Errorf("layer.Name = %q, want 'user'", layer.Name)
	}

	// Non-existing source
	if m.GetLayerBySource(SourceEnv) != nil {
		t.Error("GetLayerBySource should return nil for non-existing source")
	}
}

func TestManager_Merge(t *testing.T) {
	m := NewManager()

	// Add defaults (lowest priority)
	defaults := NewLayerWithData("defaults", SourceBuiltin, PriorityBuiltin, map[string]any{
		"editor": map[string]any{
			"tabSize":      4,
			"insertSpaces": true,
		},
		"ui": map[string]any{
			"theme": "dark",
		},
	})
	m.AddLayer(defaults)

	// Add user settings (higher priority)
	user := NewLayerWithData("user", SourceUserGlobal, PriorityUserGlobal, map[string]any{
		"editor": map[string]any{
			"tabSize": 2, // Override default
		},
	})
	m.AddLayer(user)

	merged := m.Merge()

	// tabSize should be from user layer
	if val, _ := GetByPath(merged, "editor.tabSize"); val != 2 {
		t.Errorf("editor.tabSize = %v, want 2", val)
	}

	// insertSpaces should be from defaults
	if val, _ := GetByPath(merged, "editor.insertSpaces"); val != true {
		t.Errorf("editor.insertSpaces = %v, want true", val)
	}

	// theme should be from defaults
	if val, _ := GetByPath(merged, "ui.theme"); val != "dark" {
		t.Errorf("ui.theme = %v, want 'dark'", val)
	}
}

func TestManager_Merge_Caching(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayerWithData("test", SourceBuiltin, PriorityBuiltin, map[string]any{
		"value": 1,
	}))

	// First merge
	merged1 := m.Merge()
	merged2 := m.Merge()

	// Both should have same values
	if merged1["value"] != merged2["value"] {
		t.Error("cached merge should return same values")
	}

	// Modify the returned map - should not affect cache due to cloning
	merged1["value"] = 999
	merged3 := m.Merge()
	if merged3["value"] != 1 {
		t.Error("modifying returned merge should not affect cache")
	}
}

func TestManager_Get(t *testing.T) {
	m := NewManager()

	defaults := NewLayerWithData("defaults", SourceBuiltin, PriorityBuiltin, map[string]any{
		"editor": map[string]any{
			"tabSize": 4,
		},
	})
	user := NewLayerWithData("user", SourceUserGlobal, PriorityUserGlobal, map[string]any{
		"editor": map[string]any{
			"tabSize": 2,
		},
	})

	m.AddLayer(defaults)
	m.AddLayer(user)

	// Should get from highest priority layer
	val, layer, found := m.Get("editor.tabSize")
	if !found {
		t.Fatal("expected to find editor.tabSize")
	}
	if val != 2 {
		t.Errorf("value = %v, want 2", val)
	}
	if layer.Name != "user" {
		t.Errorf("layer = %q, want 'user'", layer.Name)
	}

	// Non-existent path
	_, _, found = m.Get("nonexistent")
	if found {
		t.Error("expected found = false for non-existent path")
	}
}

func TestManager_Set(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayer("user", SourceUserGlobal, PriorityUserGlobal))

	err := m.Set("user", "editor.tabSize", 4)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, found := m.GetEffectiveValue("editor.tabSize")
	if !found {
		t.Fatal("expected to find editor.tabSize")
	}
	if val != 4 {
		t.Errorf("value = %v, want 4", val)
	}
}

func TestManager_Set_ReadOnly(t *testing.T) {
	m := NewManager()
	layer := NewLayer("readonly", SourceBuiltin, PriorityBuiltin)
	layer.ReadOnly = true
	m.AddLayer(layer)

	err := m.Set("readonly", "editor.tabSize", 4)
	if err == nil {
		t.Error("expected error when setting read-only layer")
	}
}

func TestManager_Set_NonExistent(t *testing.T) {
	m := NewManager()

	err := m.Set("nonexistent", "editor.tabSize", 4)
	if err == nil {
		t.Error("expected error for non-existent layer")
	}
}

func TestManager_SetInSession(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayerWithData("defaults", SourceBuiltin, PriorityBuiltin, map[string]any{
		"editor": map[string]any{
			"tabSize": 4,
		},
	}))

	// Set in session - should create session layer
	m.SetInSession("editor.tabSize", 2)

	// Session should override defaults
	val, found := m.GetEffectiveValue("editor.tabSize")
	if !found {
		t.Fatal("expected to find editor.tabSize")
	}
	if val != 2 {
		t.Errorf("value = %v, want 2", val)
	}

	// Verify session layer was created
	session := m.GetLayerBySource(SourceSession)
	if session == nil {
		t.Error("session layer should have been created")
	}
}

func TestManager_Delete(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayerWithData("user", SourceUserGlobal, PriorityUserGlobal, map[string]any{
		"editor": map[string]any{
			"tabSize": 4,
		},
	}))

	err := m.Delete("user", "editor.tabSize")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, found := m.GetLayerValue("user", "editor.tabSize")
	if found {
		t.Error("editor.tabSize should be deleted")
	}
}

func TestManager_UpdateLayer(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayerWithData("user", SourceUserGlobal, PriorityUserGlobal, map[string]any{
		"old": "data",
	}))

	newData := map[string]any{
		"new": "data",
	}

	err := m.UpdateLayer("user", newData)
	if err != nil {
		t.Fatalf("UpdateLayer failed: %v", err)
	}

	// Old data should be gone
	_, found := m.GetLayerValue("user", "old")
	if found {
		t.Error("old data should be replaced")
	}

	// New data should exist
	val, found := m.GetLayerValue("user", "new")
	if !found || val != "data" {
		t.Error("new data should exist")
	}
}

func TestManager_WhichLayer(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayerWithData("defaults", SourceBuiltin, PriorityBuiltin, map[string]any{
		"editor": map[string]any{
			"tabSize": 4,
		},
	}))
	m.AddLayer(NewLayerWithData("user", SourceUserGlobal, PriorityUserGlobal, map[string]any{
		"editor": map[string]any{
			"tabSize": 2,
		},
	}))

	layer := m.WhichLayer("editor.tabSize")
	if layer != "user" {
		t.Errorf("WhichLayer = %q, want 'user'", layer)
	}

	layer = m.WhichLayer("nonexistent")
	if layer != "" {
		t.Errorf("WhichLayer for nonexistent = %q, want ''", layer)
	}
}

func TestManager_Invalidate(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayerWithData("test", SourceBuiltin, PriorityBuiltin, map[string]any{
		"value": 1,
	}))

	// Trigger cache
	m.Merge()

	// Directly modify layer data (not recommended but possible)
	layer := m.GetLayer("test")
	layer.Data["value"] = 2

	// Without invalidate, cache is stale
	m.Invalidate()

	// Now merge should reflect new value
	merged := m.Merge()
	if merged["value"] != 2 {
		t.Errorf("value = %v after invalidate, want 2", merged["value"])
	}
}

func TestManager_Clear(t *testing.T) {
	m := NewManager()
	m.AddLayer(NewLayer("test", SourceBuiltin, PriorityBuiltin))

	m.Clear()

	if m.LayerCount() != 0 {
		t.Errorf("LayerCount() = %d after Clear, want 0", m.LayerCount())
	}
}

func TestManager_PriorityOrder(t *testing.T) {
	m := NewManager()

	// Add layers in random order
	m.AddLayer(NewLayerWithData("session", SourceSession, PrioritySession, map[string]any{"value": "session"}))
	m.AddLayer(NewLayerWithData("defaults", SourceBuiltin, PriorityBuiltin, map[string]any{"value": "defaults"}))
	m.AddLayer(NewLayerWithData("user", SourceUserGlobal, PriorityUserGlobal, map[string]any{"value": "user"}))
	m.AddLayer(NewLayerWithData("env", SourceEnv, PriorityEnv, map[string]any{"value": "env"}))

	// Session has highest priority
	val, _ := m.GetEffectiveValue("value")
	if val != "session" {
		t.Errorf("value = %v, want 'session' (highest priority)", val)
	}

	// Remove session
	m.RemoveLayer("session")
	val, _ = m.GetEffectiveValue("value")
	if val != "env" {
		t.Errorf("value = %v, want 'env' (next highest priority)", val)
	}
}
