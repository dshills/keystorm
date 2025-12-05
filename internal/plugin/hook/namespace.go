package hook

import (
	"fmt"
	"strings"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// PluginNamespaceHandler routes actions in the "plugin" namespace to plugins.
// It handles actions in the format: plugin.<pluginname>.<action>
// For example: plugin.vim-surround.surround
type PluginNamespaceHandler struct {
	manager PluginManager
}

// NewPluginNamespaceHandler creates a new plugin namespace handler.
func NewPluginNamespaceHandler(manager PluginManager) *PluginNamespaceHandler {
	return &PluginNamespaceHandler{
		manager: manager,
	}
}

// HandleAction implements handler.NamespaceHandler interface.
func (h *PluginNamespaceHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	// Parse the action name: plugin.<pluginname>.<action>
	pluginName, fnName, err := h.parseActionName(action.Name)
	if err != nil {
		return handler.Errorf("invalid plugin action: %v", err)
	}

	// Get the plugin
	plugin := h.manager.Get(pluginName)
	if plugin == nil {
		return handler.Errorf("plugin %q not found or not active", pluginName)
	}

	// Check if function exists
	if !plugin.HasFunction(fnName) {
		return handler.Errorf("plugin %q has no function %q", pluginName, fnName)
	}

	// Build argument map for the plugin
	args := map[string]interface{}{
		"action": action.Name,
		"count":  action.Count,
	}

	// Merge action args from Extra map
	if action.Args.Extra != nil {
		for key, val := range action.Args.Extra {
			args[key] = val
		}
	}

	// Call the plugin function
	results, err := plugin.Call(fnName, args)
	if err != nil {
		return handler.Errorf("plugin %q error: %v", pluginName, err)
	}

	// Process the result
	return processPluginResult(results)
}

// CanHandle implements handler.NamespaceHandler interface.
func (h *PluginNamespaceHandler) CanHandle(actionName string) bool {
	// Must start with "plugin."
	if !strings.HasPrefix(actionName, "plugin.") {
		return false
	}

	// Must have at least two more parts: plugin.<name>.<action>
	parts := strings.SplitN(actionName, ".", 3)
	if len(parts) < 3 {
		return false
	}

	// Check if plugin exists and has the function
	pluginName := parts[1]
	fnName := parts[2]

	plugin := h.manager.Get(pluginName)
	if plugin == nil {
		return false
	}

	return plugin.HasFunction(fnName)
}

// Namespace implements handler.NamespaceHandler interface.
func (h *PluginNamespaceHandler) Namespace() string {
	return "plugin"
}

// parseActionName extracts plugin name and function name from action.
// Action format: plugin.<pluginname>.<functionname>
// Also supports: plugin.<pluginname>.<sub>.<functionname> (nested namespaces)
func (h *PluginNamespaceHandler) parseActionName(actionName string) (pluginName, fnName string, err error) {
	// Remove "plugin." prefix
	if !strings.HasPrefix(actionName, "plugin.") {
		return "", "", fmt.Errorf("action %q does not start with 'plugin.'", actionName)
	}

	rest := strings.TrimPrefix(actionName, "plugin.")

	// Split into parts
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("action %q missing function name after plugin name", actionName)
	}

	pluginName = parts[0]
	fnName = parts[1]

	if pluginName == "" {
		return "", "", fmt.Errorf("action %q has empty plugin name", actionName)
	}
	if fnName == "" {
		return "", "", fmt.Errorf("action %q has empty function name", actionName)
	}

	// Convert dotted function name to underscore-separated for Lua
	// e.g., "surround.word" -> "surround_word"
	fnName = strings.ReplaceAll(fnName, ".", "_")

	return pluginName, fnName, nil
}

// processPluginResult converts plugin return values to a handler.Result.
func processPluginResult(results []interface{}) handler.Result {
	if len(results) == 0 {
		return handler.Success()
	}

	// First result can be:
	// - nil: success
	// - string: error message
	// - bool: false = error, true = success
	// - table: { status, message, mode_change, ... }
	first := results[0]
	if first == nil {
		return handler.Success()
	}

	switch v := first.(type) {
	case bool:
		if v {
			return handler.Success()
		}
		if len(results) > 1 {
			if msg, ok := results[1].(string); ok {
				return handler.Error(fmt.Errorf("%s", msg))
			}
		}
		return handler.Error(fmt.Errorf("plugin returned failure"))

	case string:
		// Non-empty string is treated as an error
		if v != "" {
			return handler.Error(fmt.Errorf("%s", v))
		}
		return handler.Success()

	case map[string]interface{}:
		return processResultTable(v)

	default:
		return handler.Success()
	}
}

// processResultTable converts a Lua table result to a handler.Result.
func processResultTable(tbl map[string]interface{}) handler.Result {
	result := handler.Success()

	// Check for error
	if errVal, ok := tbl["error"]; ok {
		if errStr, ok := errVal.(string); ok && errStr != "" {
			return handler.Error(fmt.Errorf("%s", errStr))
		}
	}

	// Check for status
	if status, ok := tbl["status"]; ok {
		switch v := status.(type) {
		case bool:
			if !v {
				return handler.Error(fmt.Errorf("plugin returned failure"))
			}
		case string:
			if v == "error" || v == "failed" {
				if msg, ok := tbl["message"].(string); ok {
					return handler.Error(fmt.Errorf("%s", msg))
				}
				return handler.Error(fmt.Errorf("plugin returned failure"))
			}
		}
	}

	// Check for message
	if msg, ok := tbl["message"].(string); ok {
		result.Message = msg
	}

	// Check for mode change
	if mode, ok := tbl["mode_change"].(string); ok {
		result.ModeChange = mode
	}

	return result
}
