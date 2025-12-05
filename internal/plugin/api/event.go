package api

import (
	"fmt"
	"sync"
	"sync/atomic"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// EventProvider defines the interface for event bus operations.
//
// IMPORTANT: Thread Safety Requirement
// The EventProvider implementation MUST invoke subscription handlers on the
// same goroutine that owns the Lua state (the plugin's main goroutine).
// gopher-lua's LState is not goroutine-safe, so callbacks cannot be invoked
// from arbitrary goroutines. The EventProvider should use a queue/channel
// mechanism to marshal callback invocations to the correct goroutine.
type EventProvider interface {
	// Subscribe adds an event handler for the given event type.
	// Returns a subscription ID that can be used to unsubscribe.
	//
	// The handler MUST be invoked on the goroutine that owns the Lua state.
	Subscribe(eventType string, handler func(data map[string]any)) string

	// Unsubscribe removes a subscription by ID.
	// Returns true if the subscription existed.
	Unsubscribe(id string) bool

	// Emit publishes an event to all subscribers.
	// Handlers should be invoked on their respective owning goroutines.
	Emit(eventType string, data map[string]any)
}

// EventModule implements the ks.event API module.
type EventModule struct {
	ctx        *Context
	pluginName string
	L          *lua.LState

	// Track subscriptions for cleanup
	mu            sync.Mutex
	subscriptions map[string]subscriptionInfo
	handlerTbl    *lua.LTable // Table storing handler functions to prevent GC
	handlerKey    string      // Global key for handler table
	nextID        uint64      // Counter for generating subscription IDs
}

// subscriptionInfo tracks information about a subscription.
type subscriptionInfo struct {
	eventType string
	subID     string // ID from the EventProvider
}

// NewEventModule creates a new event module.
func NewEventModule(ctx *Context, pluginName string) *EventModule {
	return &EventModule{
		ctx:           ctx,
		pluginName:    pluginName,
		subscriptions: make(map[string]subscriptionInfo),
		handlerKey:    "_ks_event_handlers_" + pluginName,
	}
}

// Name returns the module name.
func (m *EventModule) Name() string {
	return "event"
}

// RequiredCapability returns the capability required for this module.
func (m *EventModule) RequiredCapability() security.Capability {
	return security.CapabilityEvent
}

// Register registers the module into the Lua state.
func (m *EventModule) Register(L *lua.LState) error {
	m.L = L

	// Create table to store handler functions (prevents GC)
	m.handlerTbl = L.NewTable()
	L.SetGlobal(m.handlerKey, m.handlerTbl)

	mod := L.NewTable()

	// Register event functions
	L.SetField(mod, "on", L.NewFunction(m.on))
	L.SetField(mod, "off", L.NewFunction(m.off))
	L.SetField(mod, "once", L.NewFunction(m.once))
	L.SetField(mod, "emit", L.NewFunction(m.emit))

	L.SetGlobal("_ks_event", mod)
	return nil
}

// Cleanup releases all handler references and unsubscribes from all events.
// This should be called when the plugin is unloaded.
func (m *EventModule) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Unsubscribe from all events
	if m.ctx.Event != nil {
		for _, info := range m.subscriptions {
			m.ctx.Event.Unsubscribe(info.subID)
		}
	}

	// Clear handler table
	if m.L != nil {
		m.L.SetGlobal(m.handlerKey, lua.LNil)
	}

	// Clear references to prevent use after cleanup
	m.L = nil
	m.handlerTbl = nil
	m.subscriptions = make(map[string]subscriptionInfo)
}

// generateSubID generates a unique subscription ID for this plugin.
func (m *EventModule) generateSubID() string {
	id := atomic.AddUint64(&m.nextID, 1)
	return fmt.Sprintf("%s_%d", m.pluginName, id)
}

// on(eventType, handler) -> subscriptionID
// Subscribes to an event type. Handler receives event data as a table.
func (m *EventModule) on(L *lua.LState) int {
	eventType := L.CheckString(1)
	handler := L.CheckFunction(2)

	if eventType == "" {
		L.ArgError(1, "event type cannot be empty")
		return 0
	}

	if m.ctx.Event == nil {
		L.RaiseError("on: no event provider available")
		return 0
	}

	// Generate local subscription ID
	localID := m.generateSubID()

	// Store handler in our table to prevent GC
	m.mu.Lock()
	if m.handlerTbl != nil {
		m.handlerTbl.RawSetString(localID, handler)
	}
	m.mu.Unlock()

	// Create Go callback that calls the Lua handler
	callback := m.createCallback(localID)

	// Subscribe with the event provider
	providerSubID := m.ctx.Event.Subscribe(eventType, callback)

	// Track subscription for cleanup
	m.mu.Lock()
	m.subscriptions[localID] = subscriptionInfo{
		eventType: eventType,
		subID:     providerSubID,
	}
	m.mu.Unlock()

	L.Push(lua.LString(localID))
	return 1
}

// off(subscriptionID) -> bool
// Unsubscribes from an event. Returns true if subscription existed.
func (m *EventModule) off(L *lua.LState) int {
	subID := L.CheckString(1)

	if subID == "" {
		L.ArgError(1, "subscription ID cannot be empty")
		return 0
	}

	if m.ctx.Event == nil {
		L.Push(lua.LFalse)
		return 1
	}

	m.mu.Lock()
	info, exists := m.subscriptions[subID]
	if !exists {
		m.mu.Unlock()
		L.Push(lua.LFalse)
		return 1
	}

	// Remove from our tracking
	delete(m.subscriptions, subID)

	// Remove handler from table
	if m.handlerTbl != nil {
		m.handlerTbl.RawSetString(subID, lua.LNil)
	}
	m.mu.Unlock()

	// Unsubscribe from provider
	m.ctx.Event.Unsubscribe(info.subID)

	L.Push(lua.LTrue)
	return 1
}

// once(eventType, handler) -> subscriptionID
// Subscribes to an event type for a single occurrence.
// Handler is automatically unsubscribed after first call.
func (m *EventModule) once(L *lua.LState) int {
	eventType := L.CheckString(1)
	handler := L.CheckFunction(2)

	if eventType == "" {
		L.ArgError(1, "event type cannot be empty")
		return 0
	}

	if m.ctx.Event == nil {
		L.RaiseError("once: no event provider available")
		return 0
	}

	// Generate local subscription ID
	localID := m.generateSubID()

	// Store handler in our table to prevent GC
	m.mu.Lock()
	if m.handlerTbl != nil {
		m.handlerTbl.RawSetString(localID, handler)
	}
	m.mu.Unlock()

	// Create Go callback that calls the Lua handler and then unsubscribes
	callback := m.createOnceCallback(localID)

	// Subscribe with the event provider
	providerSubID := m.ctx.Event.Subscribe(eventType, callback)

	// Track subscription for cleanup
	m.mu.Lock()
	m.subscriptions[localID] = subscriptionInfo{
		eventType: eventType,
		subID:     providerSubID,
	}
	m.mu.Unlock()

	L.Push(lua.LString(localID))
	return 1
}

// emit(eventType, data?) -> nil
// Emits a plugin event. Event type is prefixed with "plugin.<pluginname>."
func (m *EventModule) emit(L *lua.LState) int {
	eventType := L.CheckString(1)

	if eventType == "" {
		L.ArgError(1, "event type cannot be empty")
		return 0
	}

	if m.ctx.Event == nil {
		L.RaiseError("emit: no event provider available")
		return 0
	}

	// Prefix event type with plugin namespace
	fullEventType := "plugin." + m.pluginName + "." + eventType

	// Parse optional data table
	var data map[string]any
	if L.GetTop() >= 2 {
		dataTable := L.OptTable(2, nil)
		if dataTable != nil {
			data = m.tableToMap(L, dataTable)
		}
	}

	if data == nil {
		data = make(map[string]any)
	}

	// Add source information
	data["source"] = "plugin:" + m.pluginName
	data["event_type"] = fullEventType

	// Emit the event
	m.ctx.Event.Emit(fullEventType, data)

	return 0
}

// createCallback creates a Go callback that invokes a Lua handler.
func (m *EventModule) createCallback(localID string) func(data map[string]any) {
	return func(data map[string]any) {
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

		// Convert data to Lua table
		dataTable := m.mapToTable(L, data)

		// Call the handler
		L.Push(handler)
		L.Push(dataTable)
		if err := L.PCall(1, 0, nil); err != nil {
			// Log error but don't propagate (event handlers shouldn't crash the system)
			// In a production system, this would go to a logger
			_ = err
		}
	}
}

// createOnceCallback creates a callback that unsubscribes after first call.
func (m *EventModule) createOnceCallback(localID string) func(data map[string]any) {
	called := false
	var callMu sync.Mutex

	// Create the base callback once, not on each invocation
	baseCallback := m.createCallback(localID)

	return func(data map[string]any) {
		callMu.Lock()
		if called {
			callMu.Unlock()
			return
		}
		called = true
		callMu.Unlock()

		// Call the handler
		baseCallback(data)

		// Then clean up the subscription
		m.mu.Lock()
		info, exists := m.subscriptions[localID]
		if exists {
			delete(m.subscriptions, localID)
			if m.handlerTbl != nil {
				m.handlerTbl.RawSetString(localID, lua.LNil)
			}
		}
		eventProvider := m.ctx.Event
		m.mu.Unlock()

		// Unsubscribe from provider
		if exists && eventProvider != nil {
			eventProvider.Unsubscribe(info.subID)
		}
	}
}

// mapToTable converts a Go map to a Lua table.
func (m *EventModule) mapToTable(L *lua.LState, data map[string]any) *lua.LTable {
	if data == nil {
		return L.NewTable()
	}

	tbl := L.NewTable()
	for k, v := range data {
		tbl.RawSetString(k, m.anyToLValue(L, v))
	}
	return tbl
}

// tableToMap converts a Lua table to a Go map.
func (m *EventModule) tableToMap(L *lua.LState, tbl *lua.LTable) map[string]any {
	result := make(map[string]any)
	tbl.ForEach(func(key, value lua.LValue) {
		if keyStr, ok := key.(lua.LString); ok {
			result[string(keyStr)] = m.lvalueToAny(value)
		}
	})
	return result
}

// anyToLValue converts a Go value to a Lua value.
func (m *EventModule) anyToLValue(L *lua.LState, v any) lua.LValue {
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
		return m.mapToTable(L, val)
	default:
		return lua.LString(fmt.Sprintf("%v", val))
	}
}

// lvalueToAny converts a Lua value to a Go value.
func (m *EventModule) lvalueToAny(v lua.LValue) any {
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
		// Check if it's an array-like table
		isArray := true
		maxIdx := 0
		val.ForEach(func(k, _ lua.LValue) {
			if num, ok := k.(lua.LNumber); ok {
				idx := int(num)
				if idx > maxIdx {
					maxIdx = idx
				}
			} else {
				isArray = false
			}
		})

		if isArray && maxIdx > 0 {
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
