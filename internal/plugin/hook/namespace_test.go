package hook

import (
	"errors"
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/input"
)

func TestPluginNamespaceHandlerNamespace(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewPluginNamespaceHandler(manager)

	if handler.Namespace() != "plugin" {
		t.Errorf("Namespace() = %q, want 'plugin'", handler.Namespace())
	}
}

func TestPluginNamespaceHandlerCanHandle(t *testing.T) {
	manager := newMockPluginManager()
	plugin := newMockPluginHost("myplugin")
	plugin.AddFunction("doSomething", func(args ...interface{}) ([]interface{}, error) {
		return nil, nil
	})
	manager.AddPlugin(plugin)

	handler := NewPluginNamespaceHandler(manager)

	tests := []struct {
		action   string
		expected bool
	}{
		{"plugin.myplugin.doSomething", true},
		{"plugin.myplugin.notExists", false},
		{"plugin.notexists.doSomething", false},
		{"plugin.myplugin", false},       // Missing function name
		{"plugin.", false},               // Missing plugin and function
		{"notplugin.myplugin.do", false}, // Wrong namespace
		{"core.something", false},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			result := handler.CanHandle(tt.action)
			if result != tt.expected {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.action, result, tt.expected)
			}
		})
	}
}

func TestPluginNamespaceHandlerHandleAction(t *testing.T) {
	manager := newMockPluginManager()
	plugin := newMockPluginHost("myplugin")

	called := false
	plugin.AddFunction("doSomething", func(args ...interface{}) ([]interface{}, error) {
		called = true
		return nil, nil
	})
	manager.AddPlugin(plugin)

	handler := NewPluginNamespaceHandler(manager)

	action := input.Action{
		Name:  "plugin.myplugin.doSomething",
		Count: 1,
	}
	ctx := &execctx.ExecutionContext{}

	result := handler.HandleAction(action, ctx)
	if result.Error != nil {
		t.Errorf("HandleAction error = %v", result.Error)
	}
	if !called {
		t.Error("plugin function should have been called")
	}
}

func TestPluginNamespaceHandlerHandleActionWithArgs(t *testing.T) {
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

	handler := NewPluginNamespaceHandler(manager)

	action := input.Action{
		Name:  "plugin.myplugin.greet",
		Count: 3,
		Args: input.ActionArgs{
			Extra: map[string]interface{}{
				"name": "World",
			},
		},
	}
	ctx := &execctx.ExecutionContext{}

	result := handler.HandleAction(action, ctx)
	if result.Error != nil {
		t.Errorf("HandleAction error = %v", result.Error)
	}

	if receivedArgs == nil {
		t.Fatal("receivedArgs should not be nil")
	}
	if receivedArgs["action"] != "plugin.myplugin.greet" {
		t.Errorf("receivedArgs[action] = %v, want 'plugin.myplugin.greet'", receivedArgs["action"])
	}
	if receivedArgs["count"] != 3 {
		t.Errorf("receivedArgs[count] = %v, want 3", receivedArgs["count"])
	}
	if receivedArgs["name"] != "World" {
		t.Errorf("receivedArgs[name] = %v, want 'World'", receivedArgs["name"])
	}
}

func TestPluginNamespaceHandlerHandleActionInvalidFormat(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewPluginNamespaceHandler(manager)

	tests := []struct {
		name   string
		action string
	}{
		{"missing function", "plugin.myplugin"},
		{"wrong prefix", "notplugin.myplugin.do"},
		{"empty plugin name", "plugin..doSomething"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := input.Action{Name: tt.action}
			ctx := &execctx.ExecutionContext{}

			result := handler.HandleAction(action, ctx)
			if result.Error == nil {
				t.Errorf("HandleAction(%q) should return error", tt.action)
			}
		})
	}
}

func TestPluginNamespaceHandlerHandleActionPluginNotFound(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewPluginNamespaceHandler(manager)

	action := input.Action{Name: "plugin.notexists.doSomething"}
	ctx := &execctx.ExecutionContext{}

	result := handler.HandleAction(action, ctx)
	if result.Error == nil {
		t.Error("HandleAction should return error when plugin not found")
	}
}

func TestPluginNamespaceHandlerHandleActionFunctionNotFound(t *testing.T) {
	manager := newMockPluginManager()
	plugin := newMockPluginHost("myplugin")
	manager.AddPlugin(plugin)

	handler := NewPluginNamespaceHandler(manager)

	action := input.Action{Name: "plugin.myplugin.notExists"}
	ctx := &execctx.ExecutionContext{}

	result := handler.HandleAction(action, ctx)
	if result.Error == nil {
		t.Error("HandleAction should return error when function not found")
	}
}

func TestPluginNamespaceHandlerHandleActionPluginError(t *testing.T) {
	manager := newMockPluginManager()
	plugin := newMockPluginHost("myplugin")
	plugin.AddFunction("failing", func(args ...interface{}) ([]interface{}, error) {
		return nil, errors.New("plugin error")
	})
	manager.AddPlugin(plugin)

	handler := NewPluginNamespaceHandler(manager)

	action := input.Action{Name: "plugin.myplugin.failing"}
	ctx := &execctx.ExecutionContext{}

	result := handler.HandleAction(action, ctx)
	if result.Error == nil {
		t.Error("HandleAction should return error when plugin returns error")
	}
}

func TestPluginNamespaceHandlerNestedFunctionName(t *testing.T) {
	manager := newMockPluginManager()
	plugin := newMockPluginHost("myplugin")

	called := false
	// Function name with dots is converted to underscores
	plugin.AddFunction("surround_word", func(args ...interface{}) ([]interface{}, error) {
		called = true
		return nil, nil
	})
	manager.AddPlugin(plugin)

	handler := NewPluginNamespaceHandler(manager)

	// Action with nested function name: surround.word -> surround_word
	action := input.Action{Name: "plugin.myplugin.surround.word"}
	ctx := &execctx.ExecutionContext{}

	result := handler.HandleAction(action, ctx)
	if result.Error != nil {
		t.Errorf("HandleAction error = %v", result.Error)
	}
	if !called {
		t.Error("plugin function should have been called")
	}
}

func TestParseActionName(t *testing.T) {
	manager := newMockPluginManager()
	handler := NewPluginNamespaceHandler(manager)

	tests := []struct {
		name       string
		action     string
		wantPlugin string
		wantFn     string
		wantErr    bool
	}{
		{
			name:       "simple",
			action:     "plugin.myplugin.doSomething",
			wantPlugin: "myplugin",
			wantFn:     "doSomething",
			wantErr:    false,
		},
		{
			name:       "nested function",
			action:     "plugin.myplugin.surround.word",
			wantPlugin: "myplugin",
			wantFn:     "surround_word",
			wantErr:    false,
		},
		{
			name:       "deeply nested",
			action:     "plugin.vim.text.objects.word",
			wantPlugin: "vim",
			wantFn:     "text_objects_word",
			wantErr:    false,
		},
		{
			name:    "missing prefix",
			action:  "notplugin.myplugin.do",
			wantErr: true,
		},
		{
			name:    "missing function",
			action:  "plugin.myplugin",
			wantErr: true,
		},
		{
			name:    "empty plugin name",
			action:  "plugin..doSomething",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginName, fnName, err := handler.parseActionName(tt.action)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseActionName(%q) should return error", tt.action)
				}
				return
			}

			if err != nil {
				t.Errorf("parseActionName(%q) error = %v", tt.action, err)
				return
			}

			if pluginName != tt.wantPlugin {
				t.Errorf("parseActionName(%q) pluginName = %q, want %q", tt.action, pluginName, tt.wantPlugin)
			}
			if fnName != tt.wantFn {
				t.Errorf("parseActionName(%q) fnName = %q, want %q", tt.action, fnName, tt.wantFn)
			}
		})
	}
}

func TestProcessPluginResultInNamespace(t *testing.T) {
	// Test processPluginResult function (package-level)
	tests := []struct {
		name     string
		results  []interface{}
		wantErr  bool
		wantMsg  string
		wantMode string
	}{
		{
			name:    "nil results",
			results: nil,
			wantErr: false,
		},
		{
			name:    "empty results",
			results: []interface{}{},
			wantErr: false,
		},
		{
			name:    "nil first",
			results: []interface{}{nil},
			wantErr: false,
		},
		{
			name:    "true",
			results: []interface{}{true},
			wantErr: false,
		},
		{
			name:    "false",
			results: []interface{}{false},
			wantErr: true,
		},
		{
			name:    "false with message",
			results: []interface{}{false, "error msg"},
			wantErr: true,
			wantMsg: "error msg",
		},
		{
			name:    "error string",
			results: []interface{}{"error occurred"},
			wantErr: true,
			wantMsg: "error occurred",
		},
		{
			name:    "empty string",
			results: []interface{}{""},
			wantErr: false,
		},
		{
			name: "table with message",
			results: []interface{}{
				map[string]interface{}{
					"message": "success message",
				},
			},
			wantErr: false,
			wantMsg: "success message",
		},
		{
			name: "table with mode change",
			results: []interface{}{
				map[string]interface{}{
					"mode_change": "visual",
				},
			},
			wantErr:  false,
			wantMode: "visual",
		},
		{
			name: "table with error",
			results: []interface{}{
				map[string]interface{}{
					"error": "table error",
				},
			},
			wantErr: true,
			wantMsg: "table error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processPluginResult(tt.results)

			if tt.wantErr {
				if result.Error == nil {
					t.Error("expected error but got nil")
				}
				if tt.wantMsg != "" && result.Error.Error() != tt.wantMsg {
					t.Errorf("error message = %q, want %q", result.Error.Error(), tt.wantMsg)
				}
				return
			}

			if result.Error != nil {
				t.Errorf("unexpected error: %v", result.Error)
			}

			if tt.wantMsg != "" && result.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", result.Message, tt.wantMsg)
			}

			if tt.wantMode != "" && result.ModeChange != tt.wantMode {
				t.Errorf("mode_change = %q, want %q", result.ModeChange, tt.wantMode)
			}
		})
	}
}

func TestProcessResultTable(t *testing.T) {
	tests := []struct {
		name     string
		table    map[string]interface{}
		wantErr  bool
		wantMsg  string
		wantMode string
	}{
		{
			name:    "empty table",
			table:   map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "error field",
			table: map[string]interface{}{
				"error": "table error",
			},
			wantErr: true,
			wantMsg: "table error",
		},
		{
			name: "empty error field",
			table: map[string]interface{}{
				"error": "",
			},
			wantErr: false,
		},
		{
			name: "status false",
			table: map[string]interface{}{
				"status": false,
			},
			wantErr: true,
		},
		{
			name: "status true",
			table: map[string]interface{}{
				"status": true,
			},
			wantErr: false,
		},
		{
			name: "status error string",
			table: map[string]interface{}{
				"status":  "error",
				"message": "status error",
			},
			wantErr: true,
			wantMsg: "status error",
		},
		{
			name: "status failed string",
			table: map[string]interface{}{
				"status": "failed",
			},
			wantErr: true,
		},
		{
			name: "message only",
			table: map[string]interface{}{
				"message": "info message",
			},
			wantErr: false,
			wantMsg: "info message",
		},
		{
			name: "mode change only",
			table: map[string]interface{}{
				"mode_change": "insert",
			},
			wantErr:  false,
			wantMode: "insert",
		},
		{
			name: "message and mode change",
			table: map[string]interface{}{
				"message":     "switched mode",
				"mode_change": "normal",
			},
			wantErr:  false,
			wantMsg:  "switched mode",
			wantMode: "normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processResultTable(tt.table)

			if tt.wantErr {
				if result.Error == nil {
					t.Error("expected error but got nil")
				}
				if tt.wantMsg != "" && result.Error.Error() != tt.wantMsg {
					t.Errorf("error message = %q, want %q", result.Error.Error(), tt.wantMsg)
				}
				return
			}

			if result.Error != nil {
				t.Errorf("unexpected error: %v", result.Error)
			}

			if tt.wantMsg != "" && result.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", result.Message, tt.wantMsg)
			}

			if tt.wantMode != "" && result.ModeChange != tt.wantMode {
				t.Errorf("mode_change = %q, want %q", result.ModeChange, tt.wantMode)
			}
		})
	}
}
