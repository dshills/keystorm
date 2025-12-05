package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	plua "github.com/dshills/keystorm/internal/plugin/lua"
	glua "github.com/yuin/gopher-lua"
)

func createTestPlugin(t *testing.T, name string, luaCode string) *Manifest {
	t.Helper()
	dir := t.TempDir()

	// Write Lua file
	luaPath := filepath.Join(dir, "init.lua")
	if err := os.WriteFile(luaPath, []byte(luaCode), 0644); err != nil {
		t.Fatal(err)
	}

	return &Manifest{
		Name:    name,
		Version: "1.0.0",
		Main:    "init.lua",
		path:    dir,
	}
}

func TestNewHost(t *testing.T) {
	manifest := &Manifest{
		Name:    "test",
		Version: "1.0.0",
	}

	host, err := NewHost(manifest)
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}

	if host.Name() != "test" {
		t.Errorf("Name() = %q, want %q", host.Name(), "test")
	}
	if host.Manifest() != manifest {
		t.Error("Manifest() returned wrong manifest")
	}
	if host.State() != StateUnloaded {
		t.Errorf("State() = %v, want %v", host.State(), StateUnloaded)
	}
}

func TestNewHostNilManifest(t *testing.T) {
	_, err := NewHost(nil)
	if err != ErrNilManifest {
		t.Errorf("NewHost(nil) error = %v, want ErrNilManifest", err)
	}
}

func TestNewHostWithOptions(t *testing.T) {
	manifest := &Manifest{Name: "test", Version: "1.0.0"}

	host, err := NewHost(manifest,
		WithHostMemoryLimit(5*1024*1024),
		WithHostExecutionTimeout(2*time.Second),
		WithHostConfig(map[string]interface{}{"key": "value"}),
	)
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}

	config := host.Config()
	if config["key"] != "value" {
		t.Errorf("Config[key] = %v, want 'value'", config["key"])
	}
}

func TestHostLoadUnload(t *testing.T) {
	manifest := createTestPlugin(t, "test", `-- simple plugin`)
	host, _ := NewHost(manifest)

	ctx := context.Background()

	// Load
	if err := host.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if host.State() != StateLoaded {
		t.Errorf("State() after Load = %v, want %v", host.State(), StateLoaded)
	}

	// Unload
	if err := host.Unload(ctx); err != nil {
		t.Fatalf("Unload() error = %v", err)
	}
	if host.State() != StateUnloaded {
		t.Errorf("State() after Unload = %v, want %v", host.State(), StateUnloaded)
	}
}

func TestHostLoadAlreadyLoaded(t *testing.T) {
	manifest := createTestPlugin(t, "test", `-- simple plugin`)
	host, _ := NewHost(manifest)
	ctx := context.Background()

	host.Load(ctx)

	err := host.Load(ctx)
	if err != ErrAlreadyLoaded {
		t.Errorf("Load() on loaded host error = %v, want ErrAlreadyLoaded", err)
	}
}

func TestHostActivateDeactivate(t *testing.T) {
	manifest := createTestPlugin(t, "test", `
		activated = false
		deactivated = false

		function activate()
			activated = true
		end

		function deactivate()
			deactivated = true
		end
	`)

	host, _ := NewHost(manifest)
	ctx := context.Background()

	// Load first
	if err := host.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Activate
	if err := host.Activate(ctx); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if host.State() != StateActive {
		t.Errorf("State() after Activate = %v, want %v", host.State(), StateActive)
	}

	// Check activate was called
	activated := host.GetGlobal("activated")
	if activated != true {
		t.Error("activate() function was not called")
	}

	// Deactivate
	if err := host.Deactivate(ctx); err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}
	if host.State() != StateLoaded {
		t.Errorf("State() after Deactivate = %v, want %v", host.State(), StateLoaded)
	}

	// Check deactivate was called
	deactivated := host.GetGlobal("deactivated")
	if deactivated != true {
		t.Error("deactivate() function was not called")
	}
}

func TestHostActivateNotLoaded(t *testing.T) {
	manifest := &Manifest{Name: "test", Version: "1.0.0"}
	host, _ := NewHost(manifest)
	ctx := context.Background()

	err := host.Activate(ctx)
	if err != ErrNotLoaded {
		t.Errorf("Activate() on unloaded host error = %v, want ErrNotLoaded", err)
	}
}

func TestHostSetup(t *testing.T) {
	manifest := createTestPlugin(t, "test", `
		received_config = nil

		function setup(config)
			received_config = config
		end
	`)
	manifest.ConfigSchema = map[string]ConfigProperty{
		"option": {Default: "default_value"},
	}

	host, _ := NewHost(manifest)
	ctx := context.Background()

	host.Load(ctx)
	host.Activate(ctx)

	// Check setup was called with config
	config := host.GetGlobal("received_config")
	if config == nil {
		t.Error("setup() was not called or received nil config")
	}
}

func TestHostReload(t *testing.T) {
	manifest := createTestPlugin(t, "test", `counter = 1`)
	host, _ := NewHost(manifest)
	ctx := context.Background()

	host.Load(ctx)
	host.Activate(ctx)

	// Modify counter
	host.SetGlobal("counter", 100)

	// Reload
	if err := host.Reload(ctx); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	// Counter should be reset to 1
	counter := host.GetGlobal("counter")
	if counter != int64(1) {
		t.Errorf("counter after Reload = %v, want 1", counter)
	}

	// Should be active again
	if host.State() != StateActive {
		t.Errorf("State() after Reload = %v, want %v", host.State(), StateActive)
	}
}

func TestHostCall(t *testing.T) {
	manifest := createTestPlugin(t, "test", `
		function add(a, b)
			return a + b
		end
	`)

	host, _ := NewHost(manifest)
	ctx := context.Background()
	host.Load(ctx)

	results, err := host.Call("add", 2, 3)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Call() returned %d results, want 1", len(results))
	}

	if results[0] != int64(5) {
		t.Errorf("add(2, 3) = %v, want 5", results[0])
	}
}

func TestHostCallNotLoaded(t *testing.T) {
	manifest := &Manifest{Name: "test", Version: "1.0.0"}
	host, _ := NewHost(manifest)

	_, err := host.Call("any")
	if err != ErrNotLoaded {
		t.Errorf("Call() on unloaded host error = %v, want ErrNotLoaded", err)
	}
}

func TestHostHasFunction(t *testing.T) {
	manifest := createTestPlugin(t, "test", `
		function exists() end
	`)

	host, _ := NewHost(manifest)
	ctx := context.Background()
	host.Load(ctx)

	if !host.HasFunction("exists") {
		t.Error("HasFunction(exists) = false, want true")
	}
	if host.HasFunction("notexists") {
		t.Error("HasFunction(notexists) = true, want false")
	}
}

func TestHostGetSetGlobal(t *testing.T) {
	manifest := createTestPlugin(t, "test", `x = 42`)
	host, _ := NewHost(manifest)
	ctx := context.Background()
	host.Load(ctx)

	// Get
	x := host.GetGlobal("x")
	if x != int64(42) {
		t.Errorf("GetGlobal(x) = %v, want 42", x)
	}

	// Set
	host.SetGlobal("x", 100)
	x = host.GetGlobal("x")
	if x != int64(100) {
		t.Errorf("GetGlobal(x) after Set = %v, want 100", x)
	}
}

func TestHostRegisterFunc(t *testing.T) {
	manifest := createTestPlugin(t, "test", ``)
	host, _ := NewHost(manifest)
	ctx := context.Background()
	host.Load(ctx)

	// Register a function
	host.RegisterFunc("triple", func(L *glua.LState) int {
		n := L.CheckNumber(1)
		L.Push(glua.LNumber(float64(n) * 3))
		return 1
	})

	// Call it
	results, err := host.Call("triple", 7)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	if results[0] != int64(21) {
		t.Errorf("triple(7) = %v, want 21", results[0])
	}
}

func TestHostRegisterModule(t *testing.T) {
	// Create plugin with empty init.lua since module isn't registered yet
	manifest := createTestPlugin(t, "test", `-- empty init`)
	host, _ := NewHost(manifest)
	ctx := context.Background()

	// Load the plugin first
	if err := host.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Register the module after loading
	host.RegisterModule("mymod", map[string]glua.LGFunction{
		"greet": func(L *glua.LState) int {
			name := L.CheckString(1)
			L.Push(glua.LString("Hello, " + name + "!"))
			return 1
		},
	})

	// Now execute code that uses the module
	err := host.DoString(`result = mymod.greet("World")`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}

	result := host.GetGlobal("result")
	if result != "Hello, World!" {
		t.Errorf("result = %v, want 'Hello, World!'", result)
	}
}

func TestHostDoString(t *testing.T) {
	manifest := createTestPlugin(t, "test", ``)
	host, _ := NewHost(manifest)
	ctx := context.Background()
	host.Load(ctx)

	err := host.DoString(`answer = 42`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}

	answer := host.GetGlobal("answer")
	if answer != int64(42) {
		t.Errorf("answer = %v, want 42", answer)
	}
}

func TestHostDoStringNotLoaded(t *testing.T) {
	manifest := &Manifest{Name: "test", Version: "1.0.0"}
	host, _ := NewHost(manifest)

	err := host.DoString(`x = 1`)
	if err != ErrNotLoaded {
		t.Errorf("DoString() on unloaded host error = %v, want ErrNotLoaded", err)
	}
}

func TestHostDoFile(t *testing.T) {
	manifest := createTestPlugin(t, "test", ``)
	host, _ := NewHost(manifest)
	ctx := context.Background()
	host.Load(ctx)

	// Create a Lua file to execute
	extraFile := filepath.Join(manifest.Path(), "extra.lua")
	if err := os.WriteFile(extraFile, []byte(`extra_loaded = true`), 0644); err != nil {
		t.Fatal(err)
	}

	err := host.DoFile("extra.lua")
	if err != nil {
		t.Fatalf("DoFile() error = %v", err)
	}

	loaded := host.GetGlobal("extra_loaded")
	if loaded != true {
		t.Error("extra.lua was not executed")
	}
}

func TestHostConfig(t *testing.T) {
	manifest := &Manifest{
		Name:    "test",
		Version: "1.0.0",
		ConfigSchema: map[string]ConfigProperty{
			"setting": {Default: "default"},
		},
	}

	host, _ := NewHost(manifest)

	// Default should be applied
	config := host.Config()
	if config["setting"] != "default" {
		t.Errorf("config[setting] = %v, want 'default'", config["setting"])
	}

	// Set new value
	host.SetConfig("setting", "custom")
	config = host.Config()
	if config["setting"] != "custom" {
		t.Errorf("config[setting] after SetConfig = %v, want 'custom'", config["setting"])
	}

	// Config should return a copy
	config["setting"] = "modified"
	config2 := host.Config()
	if config2["setting"] != "custom" {
		t.Error("Config() did not return a copy")
	}
}

func TestHostTracking(t *testing.T) {
	manifest := &Manifest{Name: "test", Version: "1.0.0"}
	host, _ := NewHost(manifest)

	// Track commands
	host.TrackCommand("cmd1")
	host.TrackCommand("cmd2")
	if len(host.TrackedCommands()) != 2 {
		t.Errorf("TrackedCommands() len = %d, want 2", len(host.TrackedCommands()))
	}

	// Track keymaps
	host.TrackKeymap("km1")
	if len(host.TrackedKeymaps()) != 1 {
		t.Errorf("TrackedKeymaps() len = %d, want 1", len(host.TrackedKeymaps()))
	}

	// Track subscriptions
	host.TrackSubscription("sub1")
	if len(host.TrackedSubscriptions()) != 1 {
		t.Errorf("TrackedSubscriptions() len = %d, want 1", len(host.TrackedSubscriptions()))
	}
}

func TestHostStats(t *testing.T) {
	manifest := &Manifest{Name: "test", Version: "1.0.0"}
	host, _ := NewHost(manifest)

	host.TrackCommand("cmd1")
	host.TrackKeymap("km1")
	host.TrackSubscription("sub1")

	stats := host.Stats()
	if stats.Name != "test" {
		t.Errorf("Stats.Name = %q, want %q", stats.Name, "test")
	}
	if stats.State != StateUnloaded {
		t.Errorf("Stats.State = %v, want %v", stats.State, StateUnloaded)
	}
	if stats.Commands != 1 {
		t.Errorf("Stats.Commands = %d, want 1", stats.Commands)
	}
	if stats.Keymaps != 1 {
		t.Errorf("Stats.Keymaps = %d, want 1", stats.Keymaps)
	}
	if stats.Subscriptions != 1 {
		t.Errorf("Stats.Subscriptions = %d, want 1", stats.Subscriptions)
	}
}

func TestHostError(t *testing.T) {
	dir := t.TempDir()
	luaPath := filepath.Join(dir, "init.lua")
	if err := os.WriteFile(luaPath, []byte(`invalid lua code !!!`), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := &Manifest{
		Name:    "test",
		Version: "1.0.0",
		Main:    "init.lua",
		path:    dir,
	}

	host, _ := NewHost(manifest)
	ctx := context.Background()

	err := host.Load(ctx)
	if err == nil {
		t.Error("Load() with invalid Lua should return error")
	}

	if host.State() != StateError {
		t.Errorf("State() = %v, want %v", host.State(), StateError)
	}

	if host.Error() == nil {
		t.Error("Error() should not be nil after load failure")
	}
}

func TestHostLuaState(t *testing.T) {
	manifest := createTestPlugin(t, "test", ``)
	host, _ := NewHost(manifest)
	ctx := context.Background()
	host.Load(ctx)

	L := host.LuaState()
	if L == nil {
		t.Error("LuaState() returned nil after load")
	}
}

func TestHostBridge(t *testing.T) {
	manifest := createTestPlugin(t, "test", ``)
	host, _ := NewHost(manifest)
	ctx := context.Background()
	host.Load(ctx)

	bridge := host.Bridge()
	if bridge == nil {
		t.Error("Bridge() returned nil after load")
	}
}

func TestHostCapabilities(t *testing.T) {
	manifest := createTestPlugin(t, "test", ``)
	manifest.Capabilities = []plua.Capability{plua.CapabilityFileRead}

	host, _ := NewHost(manifest)
	ctx := context.Background()
	host.Load(ctx)

	// Capability should be granted
	L := host.LuaState()
	if L == nil {
		t.Fatal("LuaState() returned nil")
	}

	// io module should be available due to FileRead capability
	io := L.GetGlobal("io")
	if io == glua.LNil {
		t.Error("io module should be available with FileRead capability")
	}
}
