package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoader(t *testing.T) {
	loader := NewLoader()
	if loader == nil {
		t.Fatal("NewLoader() returned nil")
	}

	paths := loader.Paths()
	if len(paths) == 0 {
		t.Error("NewLoader() should have default paths")
	}
}

func TestNewLoaderWithPaths(t *testing.T) {
	customPaths := []string{"/custom/path1", "/custom/path2"}
	loader := NewLoader(WithPaths(customPaths...))

	paths := loader.Paths()
	if len(paths) != 2 {
		t.Errorf("Paths() len = %d, want 2", len(paths))
	}
	if paths[0] != "/custom/path1" {
		t.Errorf("Paths()[0] = %q, want %q", paths[0], "/custom/path1")
	}
}

func TestLoaderAddPath(t *testing.T) {
	loader := NewLoader(WithPaths("/initial"))
	loader.AddPath("/added")

	paths := loader.Paths()
	if len(paths) != 2 {
		t.Errorf("Paths() len = %d, want 2", len(paths))
	}
	if paths[1] != "/added" {
		t.Errorf("Paths()[1] = %q, want %q", paths[1], "/added")
	}
}

func TestLoaderDiscoverEmpty(t *testing.T) {
	dir := t.TempDir()
	loader := NewLoader(WithPaths(dir))

	plugins, err := loader.Discover()
	if err != nil {
		t.Errorf("Discover() error = %v", err)
	}

	if len(plugins) != 0 {
		t.Errorf("Discover() found %d plugins in empty dir", len(plugins))
	}
}

func TestLoaderDiscoverDirectoryPlugin(t *testing.T) {
	dir := t.TempDir()

	// Create a plugin directory with manifest
	pluginDir := filepath.Join(dir, "my-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `{"name": "my-plugin", "version": "1.0.0"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("-- plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))
	plugins, err := loader.Discover()
	if err != nil {
		t.Errorf("Discover() error = %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("Discover() found %d plugins, want 1", len(plugins))
	}

	if plugins[0].Name != "my-plugin" {
		t.Errorf("Plugin name = %q, want %q", plugins[0].Name, "my-plugin")
	}
	if plugins[0].Manifest == nil {
		t.Error("Plugin manifest is nil")
	}
	if plugins[0].State != StateUnloaded {
		t.Errorf("Plugin state = %v, want %v", plugins[0].State, StateUnloaded)
	}
}

func TestLoaderDiscoverDirectoryPluginWithInitLua(t *testing.T) {
	dir := t.TempDir()

	// Create a plugin directory without manifest, just init.lua
	pluginDir := filepath.Join(dir, "simple-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("-- plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))
	plugins, err := loader.Discover()
	if err != nil {
		t.Errorf("Discover() error = %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("Discover() found %d plugins, want 1", len(plugins))
	}

	// Should use directory name as plugin name
	if plugins[0].Name != "simple-plugin" {
		t.Errorf("Plugin name = %q, want %q", plugins[0].Name, "simple-plugin")
	}
}

func TestLoaderDiscoverSingleFilePlugin(t *testing.T) {
	dir := t.TempDir()

	// Create a single-file plugin
	if err := os.WriteFile(filepath.Join(dir, "quick.lua"), []byte("-- plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))
	plugins, err := loader.Discover()
	if err != nil {
		t.Errorf("Discover() error = %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("Discover() found %d plugins, want 1", len(plugins))
	}

	// Should use filename without extension as name
	if plugins[0].Name != "quick" {
		t.Errorf("Plugin name = %q, want %q", plugins[0].Name, "quick")
	}
}

func TestLoaderDiscoverNoEntryPoint(t *testing.T) {
	dir := t.TempDir()

	// Create a directory without any Lua files
	pluginDir := filepath.Join(dir, "empty-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))
	plugins, err := loader.Discover()
	if err != nil {
		t.Errorf("Discover() error = %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("Discover() found %d plugins, want 1", len(plugins))
	}

	// Should have error
	if plugins[0].Error == nil {
		t.Error("Plugin should have error for no entry point")
	}
	if plugins[0].State != StateError {
		t.Errorf("Plugin state = %v, want %v", plugins[0].State, StateError)
	}
}

func TestLoaderDiscoverInvalidManifest(t *testing.T) {
	dir := t.TempDir()

	// Create a plugin with invalid manifest
	pluginDir := filepath.Join(dir, "bad-manifest")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))
	plugins, err := loader.Discover()
	if err != nil {
		t.Errorf("Discover() error = %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("Discover() found %d plugins, want 1", len(plugins))
	}

	if plugins[0].Error == nil {
		t.Error("Plugin should have error for invalid manifest")
	}
}

func TestLoaderFirstPathWins(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// Create same plugin in both directories
	for i, dir := range []string{dir1, dir2} {
		pluginDir := filepath.Join(dir, "dup-plugin")
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Use different versions to distinguish
		manifest := `{"name": "dup-plugin", "version": "` + string('1'+byte(i)) + `.0.0"}`
		if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("-- plugin"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// dir1 comes first
	loader := NewLoader(WithPaths(dir1, dir2))
	plugins, err := loader.Discover()
	if err != nil {
		t.Errorf("Discover() error = %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("Discover() found %d plugins, want 1", len(plugins))
	}

	// Should use first path's version
	if plugins[0].Manifest.Version != "1.0.0" {
		t.Errorf("Plugin version = %q, want %q (from first path)", plugins[0].Manifest.Version, "1.0.0")
	}
}

func TestLoaderGet(t *testing.T) {
	dir := t.TempDir()

	pluginDir := filepath.Join(dir, "get-test")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("-- plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))
	loader.Discover()

	// Get existing
	info, ok := loader.Get("get-test")
	if !ok {
		t.Error("Get(get-test) ok = false")
	}
	if info.Name != "get-test" {
		t.Errorf("Get().Name = %q, want %q", info.Name, "get-test")
	}

	// Get non-existent
	_, ok = loader.Get("nonexistent")
	if ok {
		t.Error("Get(nonexistent) ok = true, want false")
	}
}

func TestLoaderFindPlugin(t *testing.T) {
	dir := t.TempDir()

	// Create plugin
	pluginDir := filepath.Join(dir, "findme")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("-- plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))

	// Find without prior discovery
	info, err := loader.FindPlugin("findme")
	if err != nil {
		t.Errorf("FindPlugin() error = %v", err)
	}
	if info.Name != "findme" {
		t.Errorf("FindPlugin().Name = %q, want %q", info.Name, "findme")
	}

	// Find non-existent
	_, err = loader.FindPlugin("notfound")
	if err == nil {
		t.Error("FindPlugin(notfound) should return error")
	}
}

func TestLoaderFindSingleFilePlugin(t *testing.T) {
	dir := t.TempDir()

	// Create single-file plugin
	if err := os.WriteFile(filepath.Join(dir, "single.lua"), []byte("-- plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))

	info, err := loader.FindPlugin("single")
	if err != nil {
		t.Errorf("FindPlugin() error = %v", err)
	}
	if info.Name != "single" {
		t.Errorf("FindPlugin().Name = %q, want %q", info.Name, "single")
	}
}

func TestLoaderRefresh(t *testing.T) {
	dir := t.TempDir()
	loader := NewLoader(WithPaths(dir))

	// Initial discover - empty
	plugins, _ := loader.Discover()
	if len(plugins) != 0 {
		t.Errorf("Initial plugins = %d, want 0", len(plugins))
	}

	// Add a plugin
	if err := os.WriteFile(filepath.Join(dir, "new.lua"), []byte("-- plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	// Refresh
	plugins, _ = loader.Refresh()
	if len(plugins) != 1 {
		t.Errorf("After refresh plugins = %d, want 1", len(plugins))
	}
}

func TestLoaderValidatePlugin(t *testing.T) {
	dir := t.TempDir()

	// Create valid plugin
	pluginDir := filepath.Join(dir, "valid")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name": "valid", "version": "1.0.0"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()

	err := loader.ValidatePlugin(pluginDir)
	if err != nil {
		t.Errorf("ValidatePlugin() error = %v", err)
	}

	// Create invalid plugin (no entry point)
	invalidDir := filepath.Join(dir, "invalid")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatal(err)
	}

	err = loader.ValidatePlugin(invalidDir)
	if err == nil {
		t.Error("ValidatePlugin() should fail for invalid plugin")
	}
}

func TestLoaderLoadManifestOnly(t *testing.T) {
	dir := t.TempDir()

	pluginDir := filepath.Join(dir, "manifest-only")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name": "manifest-only", "version": "2.0.0"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "init.lua"), []byte("-- plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))

	m, err := loader.LoadManifestOnly("manifest-only")
	if err != nil {
		t.Errorf("LoadManifestOnly() error = %v", err)
	}
	if m.Version != "2.0.0" {
		t.Errorf("Manifest version = %q, want %q", m.Version, "2.0.0")
	}
}

func TestLoaderListNames(t *testing.T) {
	dir := t.TempDir()

	// Create multiple plugins
	for _, name := range []string{"aaa", "bbb", "ccc"} {
		if err := os.WriteFile(filepath.Join(dir, name+".lua"), []byte("-- plugin"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	loader := NewLoader(WithPaths(dir))
	loader.Discover()

	names := loader.ListNames()
	if len(names) != 3 {
		t.Errorf("ListNames() len = %d, want 3", len(names))
	}

	// Should be sorted
	if names[0] != "aaa" || names[1] != "bbb" || names[2] != "ccc" {
		t.Errorf("ListNames() = %v, want sorted [aaa bbb ccc]", names)
	}
}

func TestLoaderCount(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(dir, name+".lua"), []byte("-- plugin"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	loader := NewLoader(WithPaths(dir))
	loader.Discover()

	if loader.Count() != 2 {
		t.Errorf("Count() = %d, want 2", loader.Count())
	}
}

func TestLoaderHasErrors(t *testing.T) {
	dir := t.TempDir()

	// Create a plugin with error
	pluginDir := filepath.Join(dir, "error-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	// No entry point = error

	loader := NewLoader(WithPaths(dir))
	loader.Discover()

	if !loader.HasErrors() {
		t.Error("HasErrors() = false, want true")
	}
}

func TestLoaderErrors(t *testing.T) {
	dir := t.TempDir()

	// Create a plugin with error
	pluginDir := filepath.Join(dir, "error-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a valid plugin
	if err := os.WriteFile(filepath.Join(dir, "valid.lua"), []byte("-- plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))
	loader.Discover()

	errors := loader.Errors()
	if len(errors) != 1 {
		t.Errorf("Errors() len = %d, want 1", len(errors))
	}
	if errors[0].Name != "error-plugin" {
		t.Errorf("Error plugin name = %q, want %q", errors[0].Name, "error-plugin")
	}
}

func TestLoaderPluginsByState(t *testing.T) {
	dir := t.TempDir()

	// Create valid plugin
	if err := os.WriteFile(filepath.Join(dir, "valid.lua"), []byte("-- plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create plugin with error
	pluginDir := filepath.Join(dir, "error-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(WithPaths(dir))
	loader.Discover()

	unloaded := loader.PluginsByState(StateUnloaded)
	if len(unloaded) != 1 {
		t.Errorf("PluginsByState(Unloaded) len = %d, want 1", len(unloaded))
	}

	errored := loader.PluginsByState(StateError)
	if len(errored) != 1 {
		t.Errorf("PluginsByState(Error) len = %d, want 1", len(errored))
	}
}

func TestDefaultPluginPaths(t *testing.T) {
	paths := DefaultPluginPaths()

	// Should have at least the config path
	if len(paths) == 0 {
		t.Error("DefaultPluginPaths() returned empty slice")
	}

	// Paths should contain "plugins"
	for _, p := range paths {
		if filepath.Base(p) != "plugins" {
			t.Errorf("Path %q doesn't end with 'plugins'", p)
		}
	}
}
