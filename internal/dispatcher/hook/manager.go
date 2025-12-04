package hook

import (
	"sort"
	"sync"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// Manager manages dispatch hooks with priority-based ordering.
type Manager struct {
	mu        sync.RWMutex
	preHooks  []PreDispatchHook
	postHooks []PostDispatchHook
}

// NewManager creates a new hook manager.
func NewManager() *Manager {
	return &Manager{
		preHooks:  make([]PreDispatchHook, 0),
		postHooks: make([]PostDispatchHook, 0),
	}
}

// RegisterPre adds a pre-dispatch hook.
// Hooks are sorted by priority (higher runs first).
func (m *Manager) RegisterPre(h PreDispatchHook) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate by name
	for i, existing := range m.preHooks {
		if existing.Name() == h.Name() {
			// Replace existing
			m.preHooks[i] = h
			m.sortPreHooks()
			return
		}
	}

	m.preHooks = append(m.preHooks, h)
	m.sortPreHooks()
}

// RegisterPost adds a post-dispatch hook.
// Hooks are sorted by priority (higher runs last for post-hooks).
func (m *Manager) RegisterPost(h PostDispatchHook) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate by name
	for i, existing := range m.postHooks {
		if existing.Name() == h.Name() {
			// Replace existing
			m.postHooks[i] = h
			m.sortPostHooks()
			return
		}
	}

	m.postHooks = append(m.postHooks, h)
	m.sortPostHooks()
}

// Register adds a hook that implements both interfaces.
func (m *Manager) Register(h Hook) {
	if pre, ok := h.(PreDispatchHook); ok {
		m.RegisterPre(pre)
	}
	if post, ok := h.(PostDispatchHook); ok {
		m.RegisterPost(post)
	}
}

// UnregisterPre removes a pre-dispatch hook by name.
func (m *Manager) UnregisterPre(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, h := range m.preHooks {
		if h.Name() == name {
			m.preHooks = append(m.preHooks[:i], m.preHooks[i+1:]...)
			return true
		}
	}
	return false
}

// UnregisterPost removes a post-dispatch hook by name.
func (m *Manager) UnregisterPost(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, h := range m.postHooks {
		if h.Name() == name {
			m.postHooks = append(m.postHooks[:i], m.postHooks[i+1:]...)
			return true
		}
	}
	return false
}

// Unregister removes a hook by name from both pre and post lists.
func (m *Manager) Unregister(name string) bool {
	pre := m.UnregisterPre(name)
	post := m.UnregisterPost(name)
	return pre || post
}

// RunPreDispatch runs all pre-dispatch hooks in priority order.
// Returns false if any hook cancels the action.
func (m *Manager) RunPreDispatch(action *input.Action, ctx *execctx.ExecutionContext) bool {
	m.mu.RLock()
	hooks := make([]PreDispatchHook, len(m.preHooks))
	copy(hooks, m.preHooks)
	m.mu.RUnlock()

	for _, h := range hooks {
		if !h.PreDispatch(action, ctx) {
			return false
		}
	}
	return true
}

// RunPostDispatch runs all post-dispatch hooks from lowest to highest priority.
// This ordering allows higher priority hooks to see the final/modified results.
func (m *Manager) RunPostDispatch(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
	m.mu.RLock()
	hooks := make([]PostDispatchHook, len(m.postHooks))
	copy(hooks, m.postHooks)
	m.mu.RUnlock()

	for _, h := range hooks {
		h.PostDispatch(action, ctx, result)
	}
}

// PreHookCount returns the number of registered pre-dispatch hooks.
func (m *Manager) PreHookCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.preHooks)
}

// PostHookCount returns the number of registered post-dispatch hooks.
func (m *Manager) PostHookCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.postHooks)
}

// PreHookNames returns the names of all pre-dispatch hooks in order.
func (m *Manager) PreHookNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, len(m.preHooks))
	for i, h := range m.preHooks {
		names[i] = h.Name()
	}
	return names
}

// PostHookNames returns the names of all post-dispatch hooks in order.
func (m *Manager) PostHookNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, len(m.postHooks))
	for i, h := range m.postHooks {
		names[i] = h.Name()
	}
	return names
}

// sortPreHooks sorts pre-hooks by priority descending (higher first).
func (m *Manager) sortPreHooks() {
	sort.Slice(m.preHooks, func(i, j int) bool {
		return m.preHooks[i].Priority() > m.preHooks[j].Priority()
	})
}

// sortPostHooks sorts post-hooks by priority ascending (lower first, higher last).
// This allows higher priority hooks to see/modify results after lower ones.
func (m *Manager) sortPostHooks() {
	sort.Slice(m.postHooks, func(i, j int) bool {
		return m.postHooks[i].Priority() < m.postHooks[j].Priority()
	})
}

// Clear removes all hooks.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preHooks = m.preHooks[:0]
	m.postHooks = m.postHooks[:0]
}
