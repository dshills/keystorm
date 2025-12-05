package hook

import (
	"fmt"
	"strings"
	"sync"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// PluginHost defines the interface for plugin host operations.
// This is used to avoid circular imports with the main plugin package.
type PluginHost interface {
	// Name returns the plugin name.
	Name() string

	// Call calls a global Lua function in the plugin.
	Call(fn string, args ...interface{}) ([]interface{}, error)

	// HasFunction returns true if the plugin has the named global function.
	HasFunction(name string) bool
}

// PluginManager defines the interface for plugin manager operations.
type PluginManager interface {
	// Get returns a plugin by name.
	Get(name string) PluginHost

	// Active returns all active plugins.
	Active() []PluginHost
}

// ActionHandler handles plugin actions by routing them to the appropriate plugin.
// It acts as a bridge between the dispatcher and the plugin system.
type ActionHandler struct {
	mu      sync.RWMutex
	manager PluginManager

	// handlers maps action names to plugin handlers
	handlers map[string]pluginActionHandler
}

// pluginActionHandler holds info about a registered plugin action handler.
type pluginActionHandler struct {
	pluginName   string
	functionName string
}

// NewActionHandler creates a new plugin action handler.
func NewActionHandler(manager PluginManager) *ActionHandler {
	return &ActionHandler{
		manager:  manager,
		handlers: make(map[string]pluginActionHandler),
	}
}

// RegisterAction registers a plugin action handler.
// actionName is the full action name (e.g., "myplugin.goToDefinition")
// pluginName is the name of the plugin that will handle the action
// functionName is the Lua function to call (optional, defaults to the action suffix)
func (h *ActionHandler) RegisterAction(actionName, pluginName, functionName string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.handlers[actionName] = pluginActionHandler{
		pluginName:   pluginName,
		functionName: functionName,
	}
}

// UnregisterAction removes a plugin action handler.
func (h *ActionHandler) UnregisterAction(actionName string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.handlers, actionName)
}

// UnregisterPlugin removes all action handlers for a plugin.
func (h *ActionHandler) UnregisterPlugin(pluginName string) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	count := 0
	for action, ph := range h.handlers {
		if ph.pluginName == pluginName {
			delete(h.handlers, action)
			count++
		}
	}
	return count
}

// Handle implements handler.Handler interface.
func (h *ActionHandler) Handle(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	h.mu.RLock()
	ph, ok := h.handlers[action.Name]
	h.mu.RUnlock()

	if !ok {
		return handler.Errorf("no plugin handler for action: %s", action.Name)
	}

	// Get the plugin
	plugin := h.manager.Get(ph.pluginName)
	if plugin == nil {
		return handler.Errorf("plugin %q not found", ph.pluginName)
	}

	// Determine function name
	fnName := ph.functionName
	if fnName == "" {
		// Extract the last part of the action name as the function name
		// e.g., "myplugin.goToDefinition" -> "goToDefinition"
		parts := strings.Split(action.Name, ".")
		if len(parts) > 0 {
			fnName = parts[len(parts)-1]
		}
	}

	// Check if function exists
	if !plugin.HasFunction(fnName) {
		return handler.Errorf("plugin %q has no function %q", ph.pluginName, fnName)
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
		return handler.Errorf("plugin %q error: %v", ph.pluginName, err)
	}

	// Process the result
	return h.processPluginResult(results)
}

// CanHandle implements handler.Handler interface.
func (h *ActionHandler) CanHandle(actionName string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, ok := h.handlers[actionName]
	return ok
}

// Priority implements handler.Handler interface.
func (h *ActionHandler) Priority() int {
	return 0
}

// processPluginResult converts plugin return values to a handler.Result.
func (h *ActionHandler) processPluginResult(results []interface{}) handler.Result {
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
		return h.processResultTable(v)

	default:
		return handler.Success()
	}
}

// processResultTable converts a Lua table result to a handler.Result.
func (h *ActionHandler) processResultTable(tbl map[string]interface{}) handler.Result {
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

// ListActions returns all registered action names.
func (h *ActionHandler) ListActions() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	actions := make([]string, 0, len(h.handlers))
	for action := range h.handlers {
		actions = append(actions, action)
	}
	return actions
}

// ListPluginActions returns all actions registered by a specific plugin.
func (h *ActionHandler) ListPluginActions(pluginName string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	actions := make([]string, 0)
	for action, ph := range h.handlers {
		if ph.pluginName == pluginName {
			actions = append(actions, action)
		}
	}
	return actions
}
