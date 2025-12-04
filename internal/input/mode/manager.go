package mode

import (
	"fmt"
	"sync"
)

// Manager manages editor modes and coordinates mode transitions.
type Manager struct {
	mu sync.RWMutex

	// modes holds all registered modes by name.
	modes map[string]Mode

	// current is the active mode.
	current Mode

	// previous is the mode before the current one.
	previous Mode

	// modeStack allows pushing/popping modes (e.g., for operator-pending).
	modeStack []Mode

	// callbacks are notified on mode changes.
	callbacks []ModeChangeCallback

	// context is reused for mode transitions.
	context *Context
}

// ModeChangeCallback is called when the mode changes.
type ModeChangeCallback func(from, to Mode)

// NewManager creates a new mode manager.
func NewManager() *Manager {
	return &Manager{
		modes:     make(map[string]Mode),
		modeStack: make([]Mode, 0, 4),
		context:   NewContext(),
	}
}

// Register adds a mode to the manager.
// If a mode with the same name exists, it is replaced.
func (m *Manager) Register(mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modes[mode.Name()] = mode
}

// Unregister removes a mode from the manager.
// Returns an error if trying to unregister the current mode.
func (m *Manager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current != nil && m.current.Name() == name {
		return fmt.Errorf("cannot unregister current mode: %s", name)
	}

	delete(m.modes, name)
	return nil
}

// Get returns a mode by name, or nil if not found.
func (m *Manager) Get(name string) Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.modes[name]
}

// Current returns the current mode.
// Returns nil if no mode is set.
func (m *Manager) Current() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// CurrentName returns the name of the current mode.
// Returns empty string if no mode is set.
func (m *Manager) CurrentName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.current == nil {
		return ""
	}
	return m.current.Name()
}

// Previous returns the previous mode.
// Returns nil if there is no previous mode.
func (m *Manager) Previous() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.previous
}

// Switch changes to a different mode.
// Calls Exit() on the current mode and Enter() on the new mode.
func (m *Manager) Switch(name string) error {
	return m.SwitchWithContext(name, nil)
}

// SwitchWithContext changes to a different mode with additional context.
func (m *Manager) SwitchWithContext(name string, ctx *Context) error {
	m.mu.Lock()

	newMode, ok := m.modes[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown mode: %s", name)
	}

	oldMode, callbacks, err := m.switchToLocked(newMode, ctx)
	m.mu.Unlock()

	if err != nil {
		return err
	}

	// Notify callbacks outside of lock
	for _, cb := range callbacks {
		if cb != nil {
			cb(oldMode, newMode)
		}
	}

	return nil
}

// switchToLocked performs the mode switch (must hold lock).
// Returns the old mode and callbacks to notify.
func (m *Manager) switchToLocked(newMode Mode, ctx *Context) (Mode, []ModeChangeCallback, error) {
	if ctx == nil {
		ctx = m.context
	}

	oldMode := m.current

	// Exit current mode
	if oldMode != nil {
		ctx.NextMode = newMode.Name()
		if err := oldMode.Exit(ctx); err != nil {
			return nil, nil, fmt.Errorf("exit %s: %w", oldMode.Name(), err)
		}
	}

	// Enter new mode
	if oldMode != nil {
		ctx.PreviousMode = oldMode.Name()
	} else {
		ctx.PreviousMode = ""
	}
	ctx.NextMode = ""

	if err := newMode.Enter(ctx); err != nil {
		return nil, nil, fmt.Errorf("enter %s: %w", newMode.Name(), err)
	}

	// Update state
	m.previous = oldMode
	m.current = newMode

	// Copy callbacks to call outside of lock
	callbacks := make([]ModeChangeCallback, len(m.callbacks))
	copy(callbacks, m.callbacks)

	return oldMode, callbacks, nil
}

// Push saves the current mode and switches to a new one.
// Use Pop to restore the previous mode.
func (m *Manager) Push(name string) error {
	return m.PushWithContext(name, nil)
}

// PushWithContext saves the current mode and switches to a new one with context.
func (m *Manager) PushWithContext(name string, ctx *Context) error {
	m.mu.Lock()

	newMode, ok := m.modes[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown mode: %s", name)
	}

	if m.current != nil {
		m.modeStack = append(m.modeStack, m.current)
	}

	oldMode, callbacks, err := m.switchToLocked(newMode, ctx)
	m.mu.Unlock()

	if err != nil {
		return err
	}

	// Notify callbacks outside of lock
	for _, cb := range callbacks {
		if cb != nil {
			cb(oldMode, newMode)
		}
	}

	return nil
}

// Pop restores the previously pushed mode.
// Returns an error if the mode stack is empty.
func (m *Manager) Pop() error {
	return m.PopWithContext(nil)
}

// PopWithContext restores the previously pushed mode with context.
func (m *Manager) PopWithContext(ctx *Context) error {
	m.mu.Lock()

	if len(m.modeStack) == 0 {
		m.mu.Unlock()
		return fmt.Errorf("mode stack is empty")
	}

	// Pop from stack
	previousMode := m.modeStack[len(m.modeStack)-1]
	m.modeStack = m.modeStack[:len(m.modeStack)-1]

	oldMode, callbacks, err := m.switchToLocked(previousMode, ctx)
	m.mu.Unlock()

	if err != nil {
		return err
	}

	// Notify callbacks outside of lock
	for _, cb := range callbacks {
		if cb != nil {
			cb(oldMode, previousMode)
		}
	}

	return nil
}

// StackDepth returns the number of modes on the stack.
func (m *Manager) StackDepth() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.modeStack)
}

// OnChange registers a callback for mode changes.
// Returns a function to unregister the callback.
func (m *Manager) OnChange(callback ModeChangeCallback) func() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callbacks = append(m.callbacks, callback)
	index := len(m.callbacks) - 1

	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		// Remove callback by setting to nil (preserves indices)
		if index < len(m.callbacks) {
			m.callbacks[index] = nil
		}
	}
}

// Modes returns the names of all registered modes.
func (m *Manager) Modes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.modes))
	for name := range m.modes {
		names = append(names, name)
	}
	return names
}

// SetInitialMode sets the initial mode without triggering exit/enter.
// Should only be called once during initialization.
func (m *Manager) SetInitialMode(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mode, ok := m.modes[name]
	if !ok {
		return fmt.Errorf("unknown mode: %s", name)
	}

	m.current = mode

	// Call Enter for initial setup
	ctx := m.context
	ctx.PreviousMode = ""
	return mode.Enter(ctx)
}

// IsMode returns true if the current mode matches the given name.
func (m *Manager) IsMode(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current != nil && m.current.Name() == name
}

// IsAnyMode returns true if the current mode matches any of the given names.
func (m *Manager) IsAnyMode(names ...string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current == nil {
		return false
	}

	currentName := m.current.Name()
	for _, name := range names {
		if currentName == name {
			return true
		}
	}
	return false
}
