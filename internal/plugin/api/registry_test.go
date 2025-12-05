package api

import (
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// mockModule is a simple test module.
type mockModule struct {
	name       string
	capability security.Capability
	registered bool
}

func (m *mockModule) Name() string                            { return m.name }
func (m *mockModule) RequiredCapability() security.Capability { return m.capability }
func (m *mockModule) Register(L *lua.LState) error {
	m.registered = true
	mod := L.NewTable()
	L.SetField(mod, "test", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString("mock"))
		return 1
	}))
	L.SetGlobal("_ks_"+m.name, mod)
	return nil
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.modules == nil {
		t.Error("modules map is nil")
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()

	mod := &mockModule{name: "test"}
	err := r.Register(mod)
	if err != nil {
		t.Errorf("Register error = %v", err)
	}

	// Duplicate registration should fail
	err = r.Register(mod)
	if err == nil {
		t.Error("duplicate Register should return error")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	mod := &mockModule{name: "test"}
	r.Register(mod)

	got, ok := r.Get("test")
	if !ok {
		t.Error("Get returned ok = false")
	}
	if got != mod {
		t.Error("Get returned wrong module")
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("Get for nonexistent should return ok = false")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockModule{name: "mod1"})
	r.Register(&mockModule{name: "mod2"})

	names := r.List()
	if len(names) != 2 {
		t.Errorf("List returned %d items, want 2", len(names))
	}
}

func TestRegistryInjectAll(t *testing.T) {
	r := NewRegistry()
	mod1 := &mockModule{name: "test1", capability: ""}
	mod2 := &mockModule{name: "test2", capability: security.CapabilityBuffer}
	r.Register(mod1)
	r.Register(mod2)

	L := lua.NewState()
	defer L.Close()

	// Checker without buffer capability
	checker := security.NewPermissionChecker("test")

	err := r.InjectAll(L, checker)
	if err != nil {
		t.Errorf("InjectAll error = %v", err)
	}

	// mod1 should be registered (no capability required)
	if !mod1.registered {
		t.Error("mod1 should be registered")
	}

	// mod2 should NOT be registered (requires buffer capability)
	if mod2.registered {
		t.Error("mod2 should not be registered without buffer capability")
	}
}

func TestRegistryInjectAllWithCapability(t *testing.T) {
	r := NewRegistry()
	mod := &mockModule{name: "test", capability: security.CapabilityBuffer}
	r.Register(mod)

	L := lua.NewState()
	defer L.Close()

	// Checker with buffer capability
	checker := security.NewPermissionChecker("test")
	checker.Grant(security.CapabilityBuffer)

	err := r.InjectAll(L, checker)
	if err != nil {
		t.Errorf("InjectAll error = %v", err)
	}

	if !mod.registered {
		t.Error("mod should be registered with buffer capability")
	}
}

func TestRegistryInject(t *testing.T) {
	r := NewRegistry()
	mod1 := &mockModule{name: "mod1", capability: ""}
	mod2 := &mockModule{name: "mod2", capability: ""}
	r.Register(mod1)
	r.Register(mod2)

	L := lua.NewState()
	defer L.Close()

	checker := security.NewPermissionChecker("test")

	// Inject only mod1
	err := r.Inject(L, checker, "mod1")
	if err != nil {
		t.Errorf("Inject error = %v", err)
	}

	if !mod1.registered {
		t.Error("mod1 should be registered")
	}
	if mod2.registered {
		t.Error("mod2 should not be registered")
	}
}

func TestRegistryInjectNonexistent(t *testing.T) {
	r := NewRegistry()
	L := lua.NewState()
	defer L.Close()

	checker := security.NewPermissionChecker("test")

	err := r.Inject(L, checker, "nonexistent")
	if err == nil {
		t.Error("Inject nonexistent should return error")
	}
}

func TestRegistryInjectWithoutCapability(t *testing.T) {
	r := NewRegistry()
	mod := &mockModule{name: "test", capability: security.CapabilityBuffer}
	r.Register(mod)

	L := lua.NewState()
	defer L.Close()

	// Checker without buffer capability
	checker := security.NewPermissionChecker("test")

	err := r.Inject(L, checker, "test")
	if err == nil {
		t.Error("Inject without capability should return error")
	}
}

func TestInstallKSLoader(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	// Set up a mock module
	mod := L.NewTable()
	L.SetField(mod, "foo", lua.LString("bar"))
	L.SetGlobal("_ks_buf", mod)

	err := installKSLoader(L)
	if err != nil {
		t.Errorf("installKSLoader error = %v", err)
	}

	// Verify require("ks") works
	err = L.DoString(`
		local ks = require("ks")
		assert(ks.buf.foo == "bar", "ks.buf.foo should be 'bar'")
		assert(ks.version == "1.0.0", "ks.version should be '1.0.0'")
		assert(ks.api_version == 1, "ks.api_version should be 1")
	`)
	if err != nil {
		t.Errorf("Lua verification error = %v", err)
	}

	// Verify internal global was cleaned up
	val := L.GetGlobal("_ks_buf")
	if val != lua.LNil {
		t.Error("_ks_buf should be nil after installKSLoader")
	}
}

func TestDefaultRegistry(t *testing.T) {
	ctx := &Context{}
	r, err := DefaultRegistry(ctx)
	if err != nil {
		t.Fatalf("DefaultRegistry error = %v", err)
	}

	if r == nil {
		t.Fatal("DefaultRegistry returned nil")
	}

	// Check that standard modules are registered
	expectedModules := []string{"buf", "cursor", "mode", "util"}
	for _, name := range expectedModules {
		if _, ok := r.Get(name); !ok {
			t.Errorf("module %q not registered", name)
		}
	}
}
