package api

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// ConfigProvider defines the interface for configuration operations.
//
// IMPORTANT: Thread Safety Requirement
// The ConfigProvider implementation MUST invoke watch handlers on the same
// goroutine that owns the Lua state (the plugin's main goroutine).
// gopher-lua's LState is not goroutine-safe, so callbacks cannot be invoked
// from arbitrary goroutines. The ConfigProvider should use a queue/channel
// mechanism to marshal callback invocations to the correct goroutine.
type ConfigProvider interface {
	// Get retrieves a configuration value by key.
	// Keys are hierarchical, separated by dots (e.g., "editor.tabSize").
	// Returns the value and true if found, or nil and false if not found.
	Get(key string) (any, bool)

	// Set sets a configuration value.
	// Only keys under the plugin's namespace can be set.
	// Returns an error if the key is outside the allowed namespace.
	Set(key string, value any) error

	// Watch registers a handler to be called when a config key changes.
	// The pattern can include wildcards (e.g., "myplugin.*").
	// Returns a subscription ID for unwatching.
	Watch(pattern string, handler func(key string, oldValue, newValue any)) string

	// Unwatch removes a config watch subscription.
	Unwatch(id string) bool

	// Keys returns all keys matching a pattern.
	// Pattern supports simple wildcards (e.g., "myplugin.*").
	Keys(pattern string) []string
}

// ConfigModule implements the ks.config API module.
type ConfigModule struct {
	ctx        *Context
	pluginName string
	L          *lua.LState

	// Track watches for cleanup
	mu         sync.Mutex
	watches    map[string]watchInfo
	handlerTbl *lua.LTable // Table storing handler functions to prevent GC
	handlerKey string      // Global key for handler table
	nextID     uint64      // Counter for generating watch IDs
}

// watchInfo tracks information about a config watch.
type watchInfo struct {
	pattern string
	watchID string // ID from the ConfigProvider
}

// NewConfigModule creates a new config module.
func NewConfigModule(ctx *Context, pluginName string) *ConfigModule {
	return &ConfigModule{
		ctx:        ctx,
		pluginName: pluginName,
		watches:    make(map[string]watchInfo),
		handlerKey: "_ks_config_handlers_" + pluginName,
	}
}

// Name returns the module name.
func (m *ConfigModule) Name() string {
	return "config"
}

// RequiredCapability returns the capability required for this module.
func (m *ConfigModule) RequiredCapability() security.Capability {
	return security.CapabilityConfig
}

// Register registers the module into the Lua state.
func (m *ConfigModule) Register(L *lua.LState) error {
	m.L = L

	// Create table to store handler functions (prevents GC)
	m.handlerTbl = L.NewTable()
	L.SetGlobal(m.handlerKey, m.handlerTbl)

	mod := L.NewTable()

	// Register config functions
	L.SetField(mod, "get", L.NewFunction(m.get))
	L.SetField(mod, "set", L.NewFunction(m.set))
	L.SetField(mod, "watch", L.NewFunction(m.watch))
	L.SetField(mod, "unwatch", L.NewFunction(m.unwatch))
	L.SetField(mod, "keys", L.NewFunction(m.keys))
	L.SetField(mod, "has", L.NewFunction(m.has))

	// Add namespace constant for convenience
	L.SetField(mod, "namespace", lua.LString(m.pluginName))

	L.SetGlobal("_ks_config", mod)
	return nil
}

// Cleanup releases all handler references and unwatches all config changes.
// This should be called when the plugin is unloaded.
func (m *ConfigModule) Cleanup() {
	// Collect watch IDs while holding the lock
	m.mu.Lock()
	watchIDs := make([]string, 0, len(m.watches))
	for _, info := range m.watches {
		watchIDs = append(watchIDs, info.watchID)
	}

	// Clear handler table
	if m.L != nil {
		m.L.SetGlobal(m.handlerKey, lua.LNil)
	}

	// Clear references to prevent use after cleanup
	m.L = nil
	m.handlerTbl = nil
	m.watches = make(map[string]watchInfo)
	m.mu.Unlock()

	// Unwatch all config watches outside the lock to avoid deadlock
	if m.ctx.Config != nil {
		for _, watchID := range watchIDs {
			m.ctx.Config.Unwatch(watchID)
		}
	}
}

// generateWatchID generates a unique watch ID for this plugin.
func (m *ConfigModule) generateWatchID() string {
	id := atomic.AddUint64(&m.nextID, 1)
	return fmt.Sprintf("%s_cfg_%d", m.pluginName, id)
}

// pluginNamespace returns the config namespace for this plugin.
func (m *ConfigModule) pluginNamespace() string {
	return "plugins." + m.pluginName
}

// isInPluginNamespace checks if a key is within the plugin's writable namespace.
func (m *ConfigModule) isInPluginNamespace(key string) bool {
	namespace := m.pluginNamespace()
	return key == namespace || strings.HasPrefix(key, namespace+".")
}

// get(key) -> value or nil
// Retrieves a configuration value.
func (m *ConfigModule) get(L *lua.LState) int {
	key := L.CheckString(1)

	if key == "" {
		L.ArgError(1, "key cannot be empty")
		return 0
	}

	if m.ctx.Config == nil {
		L.Push(lua.LNil)
		return 1
	}

	value, found := m.ctx.Config.Get(key)
	if !found {
		L.Push(lua.LNil)
		return 1
	}

	L.Push(m.anyToLValue(L, value))
	return 1
}

// set(key, value) -> bool
// Sets a configuration value. Only keys in the plugin's namespace can be set.
func (m *ConfigModule) set(L *lua.LState) int {
	key := L.CheckString(1)
	value := L.Get(2)

	if key == "" {
		L.ArgError(1, "key cannot be empty")
		return 0
	}

	if m.ctx.Config == nil {
		L.RaiseError("config.set: no config provider available")
		return 0
	}

	// Enforce namespace restriction
	// Plugins can only write to their own namespace: plugins.<pluginName>.*
	if !m.isInPluginNamespace(key) {
		L.RaiseError("config.set: key %q is outside plugin namespace %q", key, m.pluginNamespace())
		return 0
	}

	goValue := m.lvalueToAny(value)

	if err := m.ctx.Config.Set(key, goValue); err != nil {
		L.RaiseError("config.set: %v", err)
		return 0
	}

	L.Push(lua.LTrue)
	return 1
}

// watch(pattern, handler) -> watchID
// Watches for configuration changes matching a pattern.
func (m *ConfigModule) watch(L *lua.LState) int {
	pattern := L.CheckString(1)
	handler := L.CheckFunction(2)

	if pattern == "" {
		L.ArgError(1, "pattern cannot be empty")
		return 0
	}

	if m.ctx.Config == nil {
		L.RaiseError("config.watch: no config provider available")
		return 0
	}

	// Generate local watch ID
	localID := m.generateWatchID()

	// Create Go callback that calls the Lua handler
	callback := m.createCallback(localID)

	// Watch with the config provider first (before storing handler)
	providerWatchID := m.ctx.Config.Watch(pattern, callback)

	// Only store handler if provider returned a valid ID
	if providerWatchID == "" {
		L.RaiseError("config.watch: provider returned invalid watch ID")
		return 0
	}

	// Store handler in our table to prevent GC
	m.mu.Lock()
	if m.handlerTbl != nil {
		m.handlerTbl.RawSetString(localID, handler)
	}

	// Track watch for cleanup
	m.watches[localID] = watchInfo{
		pattern: pattern,
		watchID: providerWatchID,
	}
	m.mu.Unlock()

	L.Push(lua.LString(localID))
	return 1
}

// unwatch(watchID) -> bool
// Removes a config watch. Returns true if watch existed.
func (m *ConfigModule) unwatch(L *lua.LState) int {
	watchID := L.CheckString(1)

	if watchID == "" {
		L.ArgError(1, "watch ID cannot be empty")
		return 0
	}

	if m.ctx.Config == nil {
		L.Push(lua.LFalse)
		return 1
	}

	m.mu.Lock()
	info, exists := m.watches[watchID]
	if !exists {
		m.mu.Unlock()
		L.Push(lua.LFalse)
		return 1
	}

	// Remove from our tracking
	delete(m.watches, watchID)

	// Remove handler from table
	if m.handlerTbl != nil {
		m.handlerTbl.RawSetString(watchID, lua.LNil)
	}
	m.mu.Unlock()

	// Unwatch from provider
	m.ctx.Config.Unwatch(info.watchID)

	L.Push(lua.LTrue)
	return 1
}

// keys(pattern?) -> {keys}
// Returns all configuration keys matching a pattern.
// If no pattern is provided, returns keys in the plugin's namespace.
func (m *ConfigModule) keys(L *lua.LState) int {
	pattern := L.OptString(1, m.pluginNamespace()+".*")

	if m.ctx.Config == nil {
		L.Push(L.NewTable())
		return 1
	}

	keys := m.ctx.Config.Keys(pattern)

	tbl := L.NewTable()
	for i, key := range keys {
		tbl.RawSetInt(i+1, lua.LString(key))
	}

	L.Push(tbl)
	return 1
}

// has(key) -> bool
// Checks if a configuration key exists.
func (m *ConfigModule) has(L *lua.LState) int {
	key := L.CheckString(1)

	if key == "" {
		L.ArgError(1, "key cannot be empty")
		return 0
	}

	if m.ctx.Config == nil {
		L.Push(lua.LFalse)
		return 1
	}

	_, found := m.ctx.Config.Get(key)
	L.Push(lua.LBool(found))
	return 1
}

// createCallback creates a Go callback that invokes a Lua handler.
func (m *ConfigModule) createCallback(localID string) func(key string, oldValue, newValue any) {
	return func(key string, oldValue, newValue any) {
		m.mu.Lock()
		L := m.L
		handlerTbl := m.handlerTbl
		m.mu.Unlock()

		if L == nil || handlerTbl == nil {
			return // Plugin unloaded
		}

		// Get the handler function from our table
		handler := L.GetField(handlerTbl, localID)
		if handler.Type() != lua.LTFunction {
			return // Handler was removed
		}

		// Call the handler with key, old_value, new_value
		L.Push(handler)
		L.Push(lua.LString(key))
		L.Push(m.anyToLValue(L, oldValue))
		L.Push(m.anyToLValue(L, newValue))
		if err := L.PCall(3, 0, nil); err != nil {
			// Log error but don't propagate (config watchers shouldn't crash the system)
			_ = err
		}
	}
}

// anyToLValue converts a Go value to a Lua value.
func (m *ConfigModule) anyToLValue(L *lua.LState, v any) lua.LValue {
	switch val := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(val)
	case int:
		return lua.LNumber(val)
	case int64:
		return lua.LNumber(val)
	case float64:
		return lua.LNumber(val)
	case string:
		return lua.LString(val)
	case []any:
		tbl := L.NewTable()
		for i, item := range val {
			tbl.RawSetInt(i+1, m.anyToLValue(L, item))
		}
		return tbl
	case map[string]any:
		tbl := L.NewTable()
		for k, item := range val {
			tbl.RawSetString(k, m.anyToLValue(L, item))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprintf("%v", val))
	}
}

// lvalueToAny converts a Lua value to a Go value.
func (m *ConfigModule) lvalueToAny(v lua.LValue) any {
	if v == nil || v == lua.LNil {
		return nil
	}

	switch val := v.(type) {
	case lua.LBool:
		return bool(val)
	case lua.LNumber:
		return float64(val)
	case lua.LString:
		return string(val)
	case *lua.LTable:
		// Check if it's an array-like table with contiguous integer keys starting at 1
		isArray := true
		maxIdx := 0
		count := 0
		val.ForEach(func(k, _ lua.LValue) {
			count++
			if num, ok := k.(lua.LNumber); ok {
				idx := int(num)
				// Verify it's a positive integer (Lua arrays are 1-indexed)
				if float64(idx) != float64(num) || idx < 1 {
					isArray = false
					return
				}
				if idx > maxIdx {
					maxIdx = idx
				}
			} else {
				isArray = false
			}
		})

		// Only treat as array if keys are contiguous (count == maxIdx)
		if isArray && maxIdx > 0 && count == maxIdx {
			arr := make([]any, maxIdx)
			val.ForEach(func(k, v lua.LValue) {
				if num, ok := k.(lua.LNumber); ok {
					idx := int(num) - 1
					if idx >= 0 && idx < maxIdx {
						arr[idx] = m.lvalueToAny(v)
					}
				}
			})
			return arr
		}

		// Treat as map
		result := make(map[string]any)
		val.ForEach(func(k, v lua.LValue) {
			var keyStr string
			switch key := k.(type) {
			case lua.LString:
				keyStr = string(key)
			case lua.LNumber:
				keyStr = fmt.Sprintf("%v", float64(key))
			default:
				keyStr = k.String()
			}
			result[keyStr] = m.lvalueToAny(v)
		})
		return result
	default:
		return v.String()
	}
}
