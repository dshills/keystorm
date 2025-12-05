package hook

import (
	"errors"
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/input"
)

// mockPluginHost implements PluginHost for testing.
type mockPluginHost struct {
	name      string
	functions map[string]func(args ...interface{}) ([]interface{}, error)
}

func newMockPluginHost(name string) *mockPluginHost {
	return &mockPluginHost{
		name:      name,
		functions: make(map[string]func(args ...interface{}) ([]interface{}, error)),
	}
}

func (m *mockPluginHost) Name() string {
	return m.name
}

func (m *mockPluginHost) Call(fn string, args ...interface{}) ([]interface{}, error) {
	if handler, ok := m.functions[fn]; ok {
		return handler(args...)
	}
	return nil, errors.New("function not found")
}

func (m *mockPluginHost) HasFunction(name string) bool {
	_, ok := m.functions[name]
	return ok
}

func (m *mockPluginHost) AddFunction(name string, handler func(args ...interface{}) ([]interface{}, error)) {
	m.functions[name] = handler
}

// mockPluginManager implements PluginManager for testing.
type mockPluginManager struct {
	plugins map[string]PluginHost
}

func newMockPluginManager() *mockPluginManager {
	return &mockPluginManager{
		plugins: make(map[string]PluginHost),
	}
}

func (m *mockPluginManager) Get(name string) PluginHost {
	return m.plugins[name]
}

func (m *mockPluginManager) Active() []PluginHost {
	result := make([]PluginHost, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result
}

func (m *mockPluginManager) AddPlugin(p PluginHost) {
	m.plugins[p.Name()] = p
}

func TestActionHandlerRegisterAction(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	handler.RegisterAction("myplugin.doSomething", "myplugin", "doSomething")

	if !handler.CanHandle("myplugin.doSomething") {
		t.Error("handler should be able to handle registered action")
	}
}

func TestActionHandlerUnregisterAction(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	handler.RegisterAction("myplugin.doSomething", "myplugin", "doSomething")
	handler.UnregisterAction("myplugin.doSomething")

	if handler.CanHandle("myplugin.doSomething") {
		t.Error("handler should not be able to handle unregistered action")
	}
}

func TestActionHandlerUnregisterPlugin(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	handler.RegisterAction("myplugin.action1", "myplugin", "action1")
	handler.RegisterAction("myplugin.action2", "myplugin", "action2")
	handler.RegisterAction("otherplugin.action1", "otherplugin", "action1")

	count := handler.UnregisterPlugin("myplugin")
	if count != 2 {
		t.Errorf("UnregisterPlugin returned %d, want 2", count)
	}

	if handler.CanHandle("myplugin.action1") {
		t.Error("myplugin.action1 should have been unregistered")
	}
	if handler.CanHandle("myplugin.action2") {
		t.Error("myplugin.action2 should have been unregistered")
	}
	if !handler.CanHandle("otherplugin.action1") {
		t.Error("otherplugin.action1 should still be registered")
	}
}

func TestActionHandlerCanHandle(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	handler.RegisterAction("myplugin.doSomething", "myplugin", "doSomething")

	if !handler.CanHandle("myplugin.doSomething") {
		t.Error("CanHandle should return true for registered action")
	}
	if handler.CanHandle("myplugin.notRegistered") {
		t.Error("CanHandle should return false for unregistered action")
	}
}

func TestActionHandlerPriority(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	if handler.Priority() != 0 {
		t.Errorf("Priority() = %d, want 0", handler.Priority())
	}
}

func TestActionHandlerHandle(t *testing.T) {
	manager := newMockPluginManager()
	plugin := newMockPluginHost("myplugin")

	called := false
	plugin.AddFunction("doSomething", func(args ...interface{}) ([]interface{}, error) {
		called = true
		return nil, nil
	})

	manager.AddPlugin(plugin)
	handler := NewActionHandler(manager)
	handler.RegisterAction("myplugin.doSomething", "myplugin", "doSomething")

	action := input.Action{
		Name:  "myplugin.doSomething",
		Count: 1,
	}
	ctx := &execctx.ExecutionContext{}

	result := handler.Handle(action, ctx)
	if result.Error != nil {
		t.Errorf("Handle error = %v", result.Error)
	}
	if !called {
		t.Error("plugin function should have been called")
	}
}

func TestActionHandlerHandleWithArgs(t *testing.T) {
	manager := newMockPluginManager()
	plugin := newMockPluginHost("myplugin")

	var receivedArgs map[string]interface{}
	plugin.AddFunction("greet", func(args ...interface{}) ([]interface{}, error) {
		if len(args) > 0 {
			if m, ok := args[0].(map[string]interface{}); ok {
				receivedArgs = m
			}
		}
		return nil, nil
	})

	manager.AddPlugin(plugin)
	handler := NewActionHandler(manager)
	handler.RegisterAction("myplugin.greet", "myplugin", "greet")

	action := input.Action{
		Name:  "myplugin.greet",
		Count: 5,
		Args: input.ActionArgs{
			Extra: map[string]interface{}{
				"name": "World",
			},
		},
	}
	ctx := &execctx.ExecutionContext{}

	result := handler.Handle(action, ctx)
	if result.Error != nil {
		t.Errorf("Handle error = %v", result.Error)
	}

	if receivedArgs == nil {
		t.Fatal("receivedArgs should not be nil")
	}
	if receivedArgs["action"] != "myplugin.greet" {
		t.Errorf("receivedArgs[action] = %v, want 'myplugin.greet'", receivedArgs["action"])
	}
	if receivedArgs["count"] != 5 {
		t.Errorf("receivedArgs[count] = %v, want 5", receivedArgs["count"])
	}
	if receivedArgs["name"] != "World" {
		t.Errorf("receivedArgs[name] = %v, want 'World'", receivedArgs["name"])
	}
}

func TestActionHandlerHandleNotRegistered(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	action := input.Action{Name: "unregistered.action"}
	ctx := &execctx.ExecutionContext{}

	result := handler.Handle(action, ctx)
	if result.Error == nil {
		t.Error("Handle should return error for unregistered action")
	}
}

func TestActionHandlerHandlePluginNotFound(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)
	handler.RegisterAction("myplugin.doSomething", "myplugin", "doSomething")

	action := input.Action{Name: "myplugin.doSomething"}
	ctx := &execctx.ExecutionContext{}

	result := handler.Handle(action, ctx)
	if result.Error == nil {
		t.Error("Handle should return error when plugin not found")
	}
}

func TestActionHandlerHandleFunctionNotFound(t *testing.T) {
	manager := newMockPluginManager()
	plugin := newMockPluginHost("myplugin")
	manager.AddPlugin(plugin)

	handler := NewActionHandler(manager)
	handler.RegisterAction("myplugin.doSomething", "myplugin", "doSomething")

	action := input.Action{Name: "myplugin.doSomething"}
	ctx := &execctx.ExecutionContext{}

	result := handler.Handle(action, ctx)
	if result.Error == nil {
		t.Error("Handle should return error when function not found")
	}
}

func TestActionHandlerHandlePluginError(t *testing.T) {
	manager := newMockPluginManager()
	plugin := newMockPluginHost("myplugin")
	plugin.AddFunction("failing", func(args ...interface{}) ([]interface{}, error) {
		return nil, errors.New("plugin error")
	})
	manager.AddPlugin(plugin)

	handler := NewActionHandler(manager)
	handler.RegisterAction("myplugin.failing", "myplugin", "failing")

	action := input.Action{Name: "myplugin.failing"}
	ctx := &execctx.ExecutionContext{}

	result := handler.Handle(action, ctx)
	if result.Error == nil {
		t.Error("Handle should return error when plugin returns error")
	}
}

func TestActionHandlerHandleDefaultFunctionName(t *testing.T) {
	manager := newMockPluginManager()
	plugin := newMockPluginHost("myplugin")

	called := false
	plugin.AddFunction("customAction", func(args ...interface{}) ([]interface{}, error) {
		called = true
		return nil, nil
	})
	manager.AddPlugin(plugin)

	handler := NewActionHandler(manager)
	// Register with empty function name - should use last part of action name
	handler.RegisterAction("myplugin.customAction", "myplugin", "")

	action := input.Action{Name: "myplugin.customAction"}
	ctx := &execctx.ExecutionContext{}

	result := handler.Handle(action, ctx)
	if result.Error != nil {
		t.Errorf("Handle error = %v", result.Error)
	}
	if !called {
		t.Error("plugin function should have been called")
	}
}

func TestActionHandlerListActions(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	handler.RegisterAction("myplugin.action1", "myplugin", "action1")
	handler.RegisterAction("myplugin.action2", "myplugin", "action2")

	actions := handler.ListActions()
	if len(actions) != 2 {
		t.Errorf("ListActions returned %d actions, want 2", len(actions))
	}
}

func TestActionHandlerListPluginActions(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	handler.RegisterAction("myplugin.action1", "myplugin", "action1")
	handler.RegisterAction("myplugin.action2", "myplugin", "action2")
	handler.RegisterAction("otherplugin.action1", "otherplugin", "action1")

	actions := handler.ListPluginActions("myplugin")
	if len(actions) != 2 {
		t.Errorf("ListPluginActions returned %d actions, want 2", len(actions))
	}

	actions = handler.ListPluginActions("otherplugin")
	if len(actions) != 1 {
		t.Errorf("ListPluginActions returned %d actions, want 1", len(actions))
	}
}

func TestProcessPluginResultNil(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult(nil)
	if result.Error != nil {
		t.Errorf("processPluginResult(nil) should return OK, got error: %v", result.Error)
	}
}

func TestProcessPluginResultEmpty(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{})
	if result.Error != nil {
		t.Errorf("processPluginResult([]) should return OK, got error: %v", result.Error)
	}
}

func TestProcessPluginResultBoolTrue(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{true})
	if result.Error != nil {
		t.Errorf("processPluginResult([true]) should return OK, got error: %v", result.Error)
	}
}

func TestProcessPluginResultBoolFalse(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{false})
	if result.Error == nil {
		t.Error("processPluginResult([false]) should return error")
	}
}

func TestProcessPluginResultBoolFalseWithMessage(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{false, "custom error"})
	if result.Error == nil {
		t.Error("processPluginResult([false, msg]) should return error")
	}
	if result.Error.Error() != "custom error" {
		t.Errorf("error message = %q, want 'custom error'", result.Error.Error())
	}
}

func TestProcessPluginResultStringError(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{"error message"})
	if result.Error == nil {
		t.Error("processPluginResult([string]) should return error")
	}
	if result.Error.Error() != "error message" {
		t.Errorf("error message = %q, want 'error message'", result.Error.Error())
	}
}

func TestProcessPluginResultEmptyString(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{""})
	if result.Error != nil {
		t.Errorf("processPluginResult(['']) should return OK, got error: %v", result.Error)
	}
}

func TestProcessPluginResultTableWithError(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{
		map[string]interface{}{
			"error": "table error",
		},
	})
	if result.Error == nil {
		t.Error("processPluginResult with error field should return error")
	}
}

func TestProcessPluginResultTableWithStatusFalse(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{
		map[string]interface{}{
			"status": false,
		},
	})
	if result.Error == nil {
		t.Error("processPluginResult with status=false should return error")
	}
}

func TestProcessPluginResultTableWithStatusError(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{
		map[string]interface{}{
			"status":  "error",
			"message": "status error message",
		},
	})
	if result.Error == nil {
		t.Error("processPluginResult with status=error should return error")
	}
	if result.Error.Error() != "status error message" {
		t.Errorf("error message = %q, want 'status error message'", result.Error.Error())
	}
}

func TestProcessPluginResultTableWithMessage(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{
		map[string]interface{}{
			"message": "info message",
		},
	})
	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	if result.Message != "info message" {
		t.Errorf("result.Message = %q, want 'info message'", result.Message)
	}
}

func TestProcessPluginResultTableWithModeChange(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewActionHandler(manager)

	result := handler.processPluginResult([]interface{}{
		map[string]interface{}{
			"mode_change": "insert",
		},
	})
	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	if result.ModeChange != "insert" {
		t.Errorf("result.ModeChange = %q, want 'insert'", result.ModeChange)
	}
}
