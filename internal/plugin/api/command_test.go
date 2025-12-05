package api

import (
	"errors"
	"sync"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/input/palette"
	"github.com/dshills/keystorm/internal/plugin/security"
)

// mockCommandProvider implements CommandProvider for testing.
type mockCommandProvider struct {
	mu       sync.RWMutex
	commands map[string]*palette.Command
}

func newMockCommandProvider() *mockCommandProvider {
	return &mockCommandProvider{
		commands: make(map[string]*palette.Command),
	}
}

func (m *mockCommandProvider) Register(cmd *palette.Command) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cmd == nil || cmd.ID == "" {
		return errors.New("invalid command")
	}
	m.commands[cmd.ID] = cmd
	return nil
}

func (m *mockCommandProvider) Unregister(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.commands[id]
	if exists {
		delete(m.commands, id)
	}
	return exists
}

func (m *mockCommandProvider) UnregisterBySource(source string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, cmd := range m.commands {
		if cmd.Source == source {
			delete(m.commands, id)
			count++
		}
	}
	return count
}

func (m *mockCommandProvider) Get(id string) *palette.Command {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.commands[id]
}

func (m *mockCommandProvider) Has(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.commands[id]
	return exists
}

func (m *mockCommandProvider) Execute(id string, args map[string]any) error {
	m.mu.RLock()
	cmd, exists := m.commands[id]
	m.mu.RUnlock()

	if !exists {
		return errors.New("command not found")
	}
	if cmd.Handler == nil {
		return errors.New("command has no handler")
	}
	return cmd.Handler(args)
}

func (m *mockCommandProvider) All() []*palette.Command {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*palette.Command, 0, len(m.commands))
	for _, cmd := range m.commands {
		result = append(result, cmd)
	}
	return result
}

func setupCommandTest(t *testing.T, cp *mockCommandProvider) (*lua.LState, *CommandModule) {
	t.Helper()

	ctx := &Context{Command: cp}
	mod := NewCommandModule(ctx, "testplugin")

	L := lua.NewState()
	t.Cleanup(func() {
		mod.Cleanup()
		L.Close()
	})

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	return L, mod
}

func TestCommandModuleName(t *testing.T) {
	ctx := &Context{}
	mod := NewCommandModule(ctx, "test")
	if mod.Name() != "command" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "command")
	}
}

func TestCommandModuleCapability(t *testing.T) {
	ctx := &Context{}
	mod := NewCommandModule(ctx, "test")
	if mod.RequiredCapability() != security.CapabilityCommand {
		t.Errorf("RequiredCapability() = %q, want %q", mod.RequiredCapability(), security.CapabilityCommand)
	}
}

func TestCommandRegister(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	err := L.DoString(`
		_ks_command.register({
			id = "testplugin.sayHello",
			title = "Say Hello",
			description = "Says hello to the user",
			category = "Test",
			handler = function(args)
				-- do nothing
			end
		})
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Check command was registered
	cmd := cp.Get("testplugin.sayHello")
	if cmd == nil {
		t.Fatal("command not registered")
	}

	if cmd.ID != "testplugin.sayHello" {
		t.Errorf("cmd.ID = %q, want %q", cmd.ID, "testplugin.sayHello")
	}
	if cmd.Title != "Say Hello" {
		t.Errorf("cmd.Title = %q, want %q", cmd.Title, "Say Hello")
	}
	if cmd.Description != "Says hello to the user" {
		t.Errorf("cmd.Description = %q, want %q", cmd.Description, "Says hello to the user")
	}
	if cmd.Category != "Test" {
		t.Errorf("cmd.Category = %q, want %q", cmd.Category, "Test")
	}
	if cmd.Source != "plugin:testplugin" {
		t.Errorf("cmd.Source = %q, want %q", cmd.Source, "plugin:testplugin")
	}
	if cmd.Handler == nil {
		t.Error("cmd.Handler should not be nil")
	}
}

func TestCommandRegisterMissingID(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	err := L.DoString(`
		_ks_command.register({
			title = "Test",
			handler = function() end
		})
	`)
	if err == nil {
		t.Error("register without id should error")
	}
}

func TestCommandRegisterMissingTitle(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	err := L.DoString(`
		_ks_command.register({
			id = "test.cmd",
			handler = function() end
		})
	`)
	if err == nil {
		t.Error("register without title should error")
	}
}

func TestCommandRegisterMissingHandler(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	err := L.DoString(`
		_ks_command.register({
			id = "test.cmd",
			title = "Test"
		})
	`)
	if err == nil {
		t.Error("register without handler should error")
	}
}

func TestCommandUnregister(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	// Register a command
	err := L.DoString(`
		_ks_command.register({
			id = "testplugin.toDelete",
			title = "To Delete",
			handler = function() end
		})
	`)
	if err != nil {
		t.Fatalf("register DoString error = %v", err)
	}

	if cp.Get("testplugin.toDelete") == nil {
		t.Fatal("command not registered")
	}

	// Unregister it
	err = L.DoString(`
		result = _ks_command.unregister("testplugin.toDelete")
	`)
	if err != nil {
		t.Fatalf("unregister DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LTrue {
		t.Error("unregister should return true for existing command")
	}

	if cp.Get("testplugin.toDelete") != nil {
		t.Error("command should have been unregistered")
	}
}

func TestCommandUnregisterNotFound(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	err := L.DoString(`
		result = _ks_command.unregister("nonexistent")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LFalse {
		t.Error("unregister should return false for nonexistent command")
	}
}

func TestCommandUnregisterOtherPluginCommand(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	// Register a command from another plugin directly
	cp.Register(&palette.Command{
		ID:     "otherplugin.cmd",
		Title:  "Other",
		Source: "plugin:otherplugin",
	})

	// Try to unregister it (should fail since it's from another plugin)
	err := L.DoString(`
		result = _ks_command.unregister("otherplugin.cmd")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	result := L.GetGlobal("result")
	if result != lua.LFalse {
		t.Error("unregister should return false for other plugin's command")
	}

	// Command should still exist
	if cp.Get("otherplugin.cmd") == nil {
		t.Error("other plugin's command should not have been deleted")
	}
}

func TestCommandList(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	// Register multiple commands
	err := L.DoString(`
		_ks_command.register({
			id = "testplugin.cmd1",
			title = "Command 1",
			handler = function() end
		})
		_ks_command.register({
			id = "testplugin.cmd2",
			title = "Command 2",
			handler = function() end
		})
	`)
	if err != nil {
		t.Fatalf("register DoString error = %v", err)
	}

	// Add a command from another plugin
	cp.Register(&palette.Command{
		ID:     "otherplugin.cmd",
		Title:  "Other",
		Source: "plugin:otherplugin",
	})

	// List commands
	err = L.DoString(`
		cmds = _ks_command.list()
		count = #cmds
	`)
	if err != nil {
		t.Fatalf("list DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	// Should only list this plugin's commands, not other plugin's
	if count.(lua.LNumber) != 2 {
		t.Errorf("list count = %v, want 2", count)
	}
}

func TestCommandExecute(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	// Register a command that sets a global variable
	err := L.DoString(`
		executed = false
		_ks_command.register({
			id = "testplugin.setFlag",
			title = "Set Flag",
			handler = function(args)
				executed = true
			end
		})
	`)
	if err != nil {
		t.Fatalf("register DoString error = %v", err)
	}

	// Execute the command
	err = L.DoString(`
		_ks_command.execute("testplugin.setFlag")
	`)
	if err != nil {
		t.Fatalf("execute DoString error = %v", err)
	}

	// Check flag was set
	executed := L.GetGlobal("executed")
	if executed != lua.LTrue {
		t.Error("handler should have been called")
	}
}

func TestCommandExecuteWithArgs(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	// Register a command that captures args
	err := L.DoString(`
		received_name = nil
		_ks_command.register({
			id = "testplugin.greet",
			title = "Greet",
			handler = function(args)
				received_name = args.name
			end
		})
	`)
	if err != nil {
		t.Fatalf("register DoString error = %v", err)
	}

	// Execute with args
	err = L.DoString(`
		_ks_command.execute("testplugin.greet", { name = "World" })
	`)
	if err != nil {
		t.Fatalf("execute DoString error = %v", err)
	}

	// Check arg was received
	receivedName := L.GetGlobal("received_name")
	if receivedName.(lua.LString) != "World" {
		t.Errorf("received_name = %v, want 'World'", receivedName)
	}
}

func TestCommandExecuteNotFound(t *testing.T) {
	cp := newMockCommandProvider()
	L, _ := setupCommandTest(t, cp)

	err := L.DoString(`
		_ks_command.execute("nonexistent")
	`)
	if err == nil {
		t.Error("execute nonexistent command should error")
	}
}

func TestCommandNilProvider(t *testing.T) {
	ctx := &Context{Command: nil}
	mod := NewCommandModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// list should return empty table
	err := L.DoString(`
		result = _ks_command.list()
		count = #result
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	count := L.GetGlobal("count")
	if count.(lua.LNumber) != 0 {
		t.Errorf("list should return empty table when provider is nil, got %v", count)
	}

	// register should error
	err = L.DoString(`
		_ks_command.register({
			id = "test",
			title = "Test",
			handler = function() end
		})
	`)
	if err == nil {
		t.Error("register should error when provider is nil")
	}
}

func TestCommandCleanup(t *testing.T) {
	cp := newMockCommandProvider()
	ctx := &Context{Command: cp}
	mod := NewCommandModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// Register a command
	err := L.DoString(`
		_ks_command.register({
			id = "testplugin.cmd",
			title = "Test",
			handler = function() end
		})
	`)
	if err != nil {
		t.Fatalf("register DoString error = %v", err)
	}

	if cp.Get("testplugin.cmd") == nil {
		t.Fatal("command not registered")
	}

	// Cleanup
	mod.Cleanup()

	// Command should be removed
	if cp.Get("testplugin.cmd") != nil {
		t.Error("command should have been removed during cleanup")
	}
}

func TestMapToTableConversion(t *testing.T) {
	ctx := &Context{}
	mod := NewCommandModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	// Test various value types
	testMap := map[string]any{
		"string": "hello",
		"number": 42.5,
		"bool":   true,
		"nil":    nil,
		"array":  []any{1, 2, 3},
		"nested": map[string]any{"key": "value"},
	}

	tbl := mod.mapToTable(L, testMap)

	// Check string
	if L.GetField(tbl, "string").(lua.LString) != "hello" {
		t.Error("string conversion failed")
	}

	// Check number
	if L.GetField(tbl, "number").(lua.LNumber) != 42.5 {
		t.Error("number conversion failed")
	}

	// Check bool
	if L.GetField(tbl, "bool") != lua.LTrue {
		t.Error("bool conversion failed")
	}

	// Check nil
	if L.GetField(tbl, "nil") != lua.LNil {
		t.Error("nil conversion failed")
	}
}

func TestLValueToAnyConversion(t *testing.T) {
	ctx := &Context{}
	mod := NewCommandModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	// Test various Lua value types
	tests := []struct {
		name     string
		luaValue lua.LValue
		expected any
	}{
		{"nil", lua.LNil, nil},
		{"true", lua.LTrue, true},
		{"false", lua.LFalse, false},
		{"number", lua.LNumber(42), float64(42)},
		{"string", lua.LString("hello"), "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mod.lvalueToAny(tt.luaValue)
			if result != tt.expected {
				t.Errorf("lvalueToAny(%v) = %v, want %v", tt.luaValue, result, tt.expected)
			}
		})
	}
}
