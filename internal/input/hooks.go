package input

import (
	"sort"
	"sync"

	"github.com/dshills/keystorm/internal/input/key"
)

// HookPriority defines the execution order for hooks.
// Lower values execute first.
type HookPriority int

const (
	// HookPriorityHighest runs before all other hooks.
	HookPriorityHighest HookPriority = -1000
	// HookPriorityHigh runs early in the hook chain.
	HookPriorityHigh HookPriority = -100
	// HookPriorityNormal is the default priority.
	HookPriorityNormal HookPriority = 0
	// HookPriorityLow runs late in the hook chain.
	HookPriorityLow HookPriority = 100
	// HookPriorityLowest runs after all other hooks.
	HookPriorityLowest HookPriority = 1000
)

// HookID uniquely identifies a registered hook.
type HookID uint64

// HookRegistration holds metadata about a registered hook.
type HookRegistration struct {
	ID       HookID
	Name     string
	Priority HookPriority
	Hook     Hook
}

// HookManager manages input hooks with support for priorities and named registration.
type HookManager struct {
	mu      sync.RWMutex
	hooks   []HookRegistration
	nextID  HookID
	sorted  bool
	byID    map[HookID]*HookRegistration
	byName  map[string]*HookRegistration
	enabled bool
}

// NewHookManager creates a new hook manager.
func NewHookManager() *HookManager {
	return &HookManager{
		hooks:   make([]HookRegistration, 0),
		byID:    make(map[HookID]*HookRegistration),
		byName:  make(map[string]*HookRegistration),
		enabled: true,
	}
}

// Register adds a hook with default priority and auto-generated name.
func (m *HookManager) Register(hook Hook) HookID {
	return m.RegisterWithOptions(hook, "", HookPriorityNormal)
}

// RegisterWithPriority adds a hook with specified priority.
func (m *HookManager) RegisterWithPriority(hook Hook, priority HookPriority) HookID {
	return m.RegisterWithOptions(hook, "", priority)
}

// RegisterNamed adds a hook with a name for later reference.
func (m *HookManager) RegisterNamed(hook Hook, name string) HookID {
	return m.RegisterWithOptions(hook, name, HookPriorityNormal)
}

// RegisterWithOptions adds a hook with all options specified.
func (m *HookManager) RegisterWithOptions(hook Hook, name string, priority HookPriority) HookID {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate unique ID
	m.nextID++
	id := m.nextID

	reg := HookRegistration{
		ID:       id,
		Name:     name,
		Priority: priority,
		Hook:     hook,
	}

	m.hooks = append(m.hooks, reg)
	m.byID[id] = &m.hooks[len(m.hooks)-1]

	if name != "" {
		m.byName[name] = &m.hooks[len(m.hooks)-1]
	}

	m.sorted = false
	return id
}

// Unregister removes a hook by ID.
func (m *HookManager) Unregister(id HookID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	reg, ok := m.byID[id]
	if !ok {
		return false
	}

	// Remove from maps
	delete(m.byID, id)
	if reg.Name != "" {
		delete(m.byName, reg.Name)
	}

	// Remove from slice
	for i := range m.hooks {
		if m.hooks[i].ID == id {
			m.hooks = append(m.hooks[:i], m.hooks[i+1:]...)
			break
		}
	}

	return true
}

// UnregisterByName removes a hook by name.
func (m *HookManager) UnregisterByName(name string) bool {
	m.mu.Lock()
	reg, ok := m.byName[name]
	if !ok {
		m.mu.Unlock()
		return false
	}
	id := reg.ID
	m.mu.Unlock()

	return m.Unregister(id)
}

// Get returns a hook registration by ID.
func (m *HookManager) Get(id HookID) *HookRegistration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byID[id]
}

// GetByName returns a hook registration by name.
func (m *HookManager) GetByName(name string) *HookRegistration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byName[name]
}

// SetEnabled enables or disables all hooks.
func (m *HookManager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

// IsEnabled returns whether hooks are enabled.
func (m *HookManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// Count returns the number of registered hooks.
func (m *HookManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.hooks)
}

// List returns all hook registrations.
func (m *HookManager) List() []HookRegistration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]HookRegistration, len(m.hooks))
	copy(result, m.hooks)
	return result
}

// ensureSorted sorts hooks by priority if needed.
func (m *HookManager) ensureSorted() {
	if m.sorted {
		return
	}

	sort.SliceStable(m.hooks, func(i, j int) bool {
		return m.hooks[i].Priority < m.hooks[j].Priority
	})
	m.sorted = true
}

// RunPreKeyEvent runs all PreKeyEvent hooks in priority order.
// Returns true if any hook consumed the event.
func (m *HookManager) RunPreKeyEvent(event *key.Event, ctx *Context) bool {
	m.mu.Lock()
	if !m.enabled || len(m.hooks) == 0 {
		m.mu.Unlock()
		return false
	}

	m.ensureSorted()

	// Copy hooks for iteration outside lock
	hooks := make([]Hook, len(m.hooks))
	for i := range m.hooks {
		hooks[i] = m.hooks[i].Hook
	}
	m.mu.Unlock()

	// Run hooks
	for _, hook := range hooks {
		if hook.PreKeyEvent(event, ctx) {
			return true
		}
	}
	return false
}

// RunPostKeyEvent runs all PostKeyEvent hooks in priority order.
func (m *HookManager) RunPostKeyEvent(event *key.Event, action *Action, ctx *Context) {
	m.mu.Lock()
	if !m.enabled || len(m.hooks) == 0 {
		m.mu.Unlock()
		return
	}

	m.ensureSorted()

	// Copy hooks for iteration outside lock
	hooks := make([]Hook, len(m.hooks))
	for i := range m.hooks {
		hooks[i] = m.hooks[i].Hook
	}
	m.mu.Unlock()

	// Run hooks
	for _, hook := range hooks {
		hook.PostKeyEvent(event, action, ctx)
	}
}

// RunPreAction runs all PreAction hooks in priority order.
// Returns true if any hook consumed the action.
func (m *HookManager) RunPreAction(action *Action, ctx *Context) bool {
	m.mu.Lock()
	if !m.enabled || len(m.hooks) == 0 {
		m.mu.Unlock()
		return false
	}

	m.ensureSorted()

	// Copy hooks for iteration outside lock
	hooks := make([]Hook, len(m.hooks))
	for i := range m.hooks {
		hooks[i] = m.hooks[i].Hook
	}
	m.mu.Unlock()

	// Run hooks
	for _, hook := range hooks {
		if hook.PreAction(action, ctx) {
			return true
		}
	}
	return false
}

// Clear removes all hooks.
func (m *HookManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hooks = make([]HookRegistration, 0)
	m.byID = make(map[HookID]*HookRegistration)
	m.byName = make(map[string]*HookRegistration)
	m.sorted = true
}

// BaseHook provides a default implementation of the Hook interface.
// Embed this in custom hooks to only implement the methods you need.
type BaseHook struct{}

// PreKeyEvent is a no-op that does not consume events.
func (BaseHook) PreKeyEvent(*key.Event, *Context) bool {
	return false
}

// PostKeyEvent is a no-op.
func (BaseHook) PostKeyEvent(*key.Event, *Action, *Context) {}

// PreAction is a no-op that does not consume actions.
func (BaseHook) PreAction(*Action, *Context) bool {
	return false
}

// FuncHook wraps functions into a Hook interface implementation.
type FuncHook struct {
	PreKeyEventFunc  func(*key.Event, *Context) bool
	PostKeyEventFunc func(*key.Event, *Action, *Context)
	PreActionFunc    func(*Action, *Context) bool
}

// PreKeyEvent calls the PreKeyEventFunc if set.
func (h FuncHook) PreKeyEvent(event *key.Event, ctx *Context) bool {
	if h.PreKeyEventFunc != nil {
		return h.PreKeyEventFunc(event, ctx)
	}
	return false
}

// PostKeyEvent calls the PostKeyEventFunc if set.
func (h FuncHook) PostKeyEvent(event *key.Event, action *Action, ctx *Context) {
	if h.PostKeyEventFunc != nil {
		h.PostKeyEventFunc(event, action, ctx)
	}
}

// PreAction calls the PreActionFunc if set.
func (h FuncHook) PreAction(action *Action, ctx *Context) bool {
	if h.PreActionFunc != nil {
		return h.PreActionFunc(action, ctx)
	}
	return false
}

// LoggingHook logs all input events and actions.
// Useful for debugging and development.
type LoggingHook struct {
	BaseHook
	Logger func(format string, args ...interface{})
}

// PreKeyEvent logs the key event.
func (h LoggingHook) PreKeyEvent(event *key.Event, ctx *Context) bool {
	if h.Logger != nil {
		h.Logger("[input] key event: %s (mode=%s)", event.String(), ctx.Mode)
	}
	return false
}

// PostKeyEvent logs the resulting action.
func (h LoggingHook) PostKeyEvent(event *key.Event, action *Action, ctx *Context) {
	if h.Logger != nil {
		if action != nil {
			h.Logger("[input] -> action: %s (count=%d)", action.Name, action.Count)
		}
	}
}

// PreAction logs action dispatch.
func (h LoggingHook) PreAction(action *Action, ctx *Context) bool {
	if h.Logger != nil {
		h.Logger("[input] dispatching: %s", action.Name)
	}
	return false
}

// FilterHook filters events or actions based on predicates.
type FilterHook struct {
	BaseHook

	// KeyEventFilter returns true to block/consume a key event.
	KeyEventFilter func(*key.Event, *Context) bool

	// ActionFilter returns true to block/consume an action.
	ActionFilter func(*Action, *Context) bool
}

// PreKeyEvent applies the key event filter.
func (h FilterHook) PreKeyEvent(event *key.Event, ctx *Context) bool {
	if h.KeyEventFilter != nil {
		return h.KeyEventFilter(event, ctx)
	}
	return false
}

// PreAction applies the action filter.
func (h FilterHook) PreAction(action *Action, ctx *Context) bool {
	if h.ActionFilter != nil {
		return h.ActionFilter(action, ctx)
	}
	return false
}
