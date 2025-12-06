// Package config provides keymap configuration management.
package config

import (
	"fmt"
	"sync"

	"github.com/dshills/keystorm/internal/config/notify"
	"github.com/dshills/keystorm/internal/input/key"
	"github.com/dshills/keystorm/internal/input/keymap"
)

// KeymapBinding represents a single key binding from config.
type KeymapBinding struct {
	// Keys is the key sequence that triggers this binding.
	Keys string

	// Action is the command to execute.
	Action string

	// Args are fixed arguments for the action.
	Args map[string]any

	// When is a condition expression that must be true for this binding.
	When string

	// Description provides documentation for the binding.
	Description string

	// Priority determines precedence when multiple bindings match.
	Priority int

	// Category groups bindings for display purposes.
	Category string
}

// KeymapEntry represents a keymap definition from config.
type KeymapEntry struct {
	// Name is the keymap identifier.
	Name string

	// Mode is the mode this keymap applies to.
	Mode string

	// FileType restricts this keymap to a specific file type.
	FileType string

	// Priority determines precedence when multiple keymaps match.
	Priority int

	// Bindings are the key-to-action mappings.
	Bindings []KeymapBinding
}

// KeymapManager manages keymap configuration and integrates with the keymap registry.
//
// Thread Safety:
// KeymapManager is safe for concurrent use. All public methods acquire
// appropriate locks before accessing internal state.
type KeymapManager struct {
	mu sync.RWMutex

	// config is the parent Config for accessing keymap settings.
	config *Config

	// notifier for change notifications.
	notifier *notify.Notifier

	// registry is the keymap registry for efficient lookup.
	registry *keymap.Registry

	// userKeymaps stores user-defined keymaps loaded from config.
	userKeymaps map[string]*KeymapEntry
}

// NewKeymapManager creates a new KeymapManager.
func NewKeymapManager(config *Config, notifier *notify.Notifier) *KeymapManager {
	return &KeymapManager{
		config:      config,
		notifier:    notifier,
		registry:    keymap.NewRegistry(),
		userKeymaps: make(map[string]*KeymapEntry),
	}
}

// Registry returns the underlying keymap registry.
// The registry can be used for efficient key lookup during editing.
func (m *KeymapManager) Registry() *keymap.Registry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registry
}

// LoadDefaults loads the default keymaps into the registry.
func (m *KeymapManager) LoadDefaults() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	defaults := []*keymap.Keymap{
		keymap.DefaultNormalKeymap(),
		keymap.DefaultInsertKeymap(),
		keymap.DefaultVisualKeymap(),
		keymap.DefaultCommandKeymap(),
		keymap.DefaultGlobalKeymap(),
	}

	for _, km := range defaults {
		if err := m.registry.Register(km); err != nil {
			return fmt.Errorf("registering default keymap %q: %w", km.Name, err)
		}
	}

	return nil
}

// LoadFromConfig loads keymap configurations from the config system.
// This merges user keymaps from keymaps.toml into the registry.
func (m *KeymapManager) LoadFromConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read keymaps from the config
	keymapsValue, ok := m.config.Get("keymaps")
	if !ok {
		return nil
	}

	keymapsSlice, ok := keymapsValue.([]any)
	if !ok {
		// Try as a map with named keymaps
		keymapsMap, ok := keymapsValue.(map[string]any)
		if !ok {
			return nil
		}
		return m.loadKeymapsFromMap(keymapsMap)
	}

	return m.loadKeymapsFromSlice(keymapsSlice)
}

// loadKeymapsFromSlice loads keymaps from a TOML array.
func (m *KeymapManager) loadKeymapsFromSlice(keymaps []any) error {
	var lastErr error
	for _, kmValue := range keymaps {
		kmMap, ok := kmValue.(map[string]any)
		if !ok {
			continue
		}

		entry, err := m.parseKeymapEntry(kmMap)
		if err != nil {
			lastErr = err
			continue
		}

		if err := m.registerEntry(entry); err != nil {
			lastErr = err
			continue
		}

		m.userKeymaps[entry.Name] = entry
	}
	return lastErr
}

// loadKeymapsFromMap loads keymaps from a TOML table.
func (m *KeymapManager) loadKeymapsFromMap(keymaps map[string]any) error {
	for name, kmValue := range keymaps {
		kmMap, ok := kmValue.(map[string]any)
		if !ok {
			continue
		}

		entry, err := m.parseKeymapEntry(kmMap)
		if err != nil {
			continue
		}
		if entry.Name == "" {
			entry.Name = name
		}

		if err := m.registerEntry(entry); err != nil {
			continue
		}

		m.userKeymaps[entry.Name] = entry
	}
	return nil
}

// parseKeymapEntry parses a keymap entry from a config map.
func (m *KeymapManager) parseKeymapEntry(data map[string]any) (*KeymapEntry, error) {
	entry := &KeymapEntry{
		Bindings: make([]KeymapBinding, 0),
	}

	if name, ok := data["name"].(string); ok {
		entry.Name = name
	}
	if mode, ok := data["mode"].(string); ok {
		entry.Mode = mode
	}
	if fileType, ok := data["fileType"].(string); ok {
		entry.FileType = fileType
	}
	if priority, ok := data["priority"].(int64); ok {
		entry.Priority = int(priority)
	} else if priority, ok := data["priority"].(int); ok {
		entry.Priority = priority
	}

	// Parse bindings
	if bindings, ok := data["bindings"].([]any); ok {
		for _, bValue := range bindings {
			bMap, ok := bValue.(map[string]any)
			if !ok {
				continue
			}

			binding := m.parseBinding(bMap)
			entry.Bindings = append(entry.Bindings, binding)
		}
	}

	if entry.Name == "" {
		entry.Name = fmt.Sprintf("user-%s", entry.Mode)
		if entry.Mode == "" {
			entry.Name = "user-global"
		}
	}

	return entry, nil
}

// parseBinding parses a single binding from config.
func (m *KeymapManager) parseBinding(data map[string]any) KeymapBinding {
	binding := KeymapBinding{}

	if keys, ok := data["keys"].(string); ok {
		binding.Keys = keys
	}
	if action, ok := data["action"].(string); ok {
		binding.Action = action
	}
	if args, ok := data["args"].(map[string]any); ok {
		binding.Args = args
	}
	if when, ok := data["when"].(string); ok {
		binding.When = when
	}
	if desc, ok := data["description"].(string); ok {
		binding.Description = desc
	}
	if priority, ok := data["priority"].(int64); ok {
		binding.Priority = int(priority)
	} else if priority, ok := data["priority"].(int); ok {
		binding.Priority = priority
	}
	if category, ok := data["category"].(string); ok {
		binding.Category = category
	}

	return binding
}

// registerEntry converts a KeymapEntry to a keymap.Keymap and registers it.
func (m *KeymapManager) registerEntry(entry *KeymapEntry) error {
	km := &keymap.Keymap{
		Name:     entry.Name,
		Mode:     entry.Mode,
		FileType: entry.FileType,
		Priority: entry.Priority,
		Source:   "user",
		Bindings: make([]keymap.Binding, 0, len(entry.Bindings)),
	}

	for _, b := range entry.Bindings {
		km.Bindings = append(km.Bindings, keymap.Binding{
			Keys:        b.Keys,
			Action:      b.Action,
			Args:        b.Args,
			When:        b.When,
			Description: b.Description,
			Priority:    b.Priority,
			Category:    b.Category,
		})
	}

	return m.registry.Register(km)
}

// AddBinding adds a user binding to a mode's keymap.
// This is the primary method for user keymap customization.
func (m *KeymapManager) AddBinding(mode string, binding KeymapBinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keymapName := "user-" + mode
	if mode == "" {
		keymapName = "user-global"
	}

	// Get or create user keymap for this mode
	entry, ok := m.userKeymaps[keymapName]
	if !ok {
		entry = &KeymapEntry{
			Name:     keymapName,
			Mode:     mode,
			Priority: 100, // User keymaps have high priority
			Bindings: make([]KeymapBinding, 0),
		}
		m.userKeymaps[keymapName] = entry
	}

	entry.Bindings = append(entry.Bindings, binding)

	// Re-register the keymap
	if err := m.registerEntry(entry); err != nil {
		return err
	}

	// Notify change
	path := "keymaps." + keymapName
	if m.notifier != nil {
		m.notifier.NotifySet(path, nil, binding, "user")
	}

	return nil
}

// RemoveBinding removes a user binding by key sequence.
func (m *KeymapManager) RemoveBinding(mode, keys string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keymapName := "user-" + mode
	if mode == "" {
		keymapName = "user-global"
	}

	entry, ok := m.userKeymaps[keymapName]
	if !ok {
		return ErrSettingNotFound
	}

	// Find and remove the binding
	found := false
	newBindings := make([]KeymapBinding, 0, len(entry.Bindings))
	for _, b := range entry.Bindings {
		if b.Keys == keys {
			found = true
			continue
		}
		newBindings = append(newBindings, b)
	}

	if !found {
		return ErrSettingNotFound
	}

	entry.Bindings = newBindings

	// Re-register the keymap
	if err := m.registerEntry(entry); err != nil {
		return err
	}

	// Notify change
	path := "keymaps." + keymapName
	if m.notifier != nil {
		m.notifier.NotifyDelete(path, keys, "user")
	}

	return nil
}

// GetBinding returns a binding by mode and key sequence.
func (m *KeymapManager) GetBinding(mode, keys string) (*KeymapBinding, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keymapName := "user-" + mode
	if mode == "" {
		keymapName = "user-global"
	}

	entry, ok := m.userKeymaps[keymapName]
	if !ok {
		return nil, false
	}

	for _, b := range entry.Bindings {
		if b.Keys == keys {
			// Return a copy
			copy := b
			return &copy, true
		}
	}

	return nil, false
}

// ListUserBindings returns all user-defined bindings for a mode.
func (m *KeymapManager) ListUserBindings(mode string) []KeymapBinding {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keymapName := "user-" + mode
	if mode == "" {
		keymapName = "user-global"
	}

	entry, ok := m.userKeymaps[keymapName]
	if !ok {
		return nil
	}

	// Return copies
	result := make([]KeymapBinding, len(entry.Bindings))
	copy(result, entry.Bindings)
	return result
}

// ListModes returns all modes that have user-defined keymaps.
func (m *KeymapManager) ListModes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	modes := make([]string, 0, len(m.userKeymaps))
	for _, entry := range m.userKeymaps {
		if entry.Mode != "" {
			modes = append(modes, entry.Mode)
		}
	}
	return modes
}

// SubscribeKeymaps subscribes to keymap changes.
func (m *KeymapManager) SubscribeKeymaps(observer notify.Observer) *notify.Subscription {
	return m.notifier.SubscribePath("keymaps", observer)
}

// SetConditionEvaluator sets the condition evaluator for the registry.
func (m *KeymapManager) SetConditionEvaluator(eval keymap.ConditionEvaluator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registry.SetConditionEvaluator(eval)
}

// Lookup finds the best matching binding for a key sequence.
func (m *KeymapManager) Lookup(mode, fileType string, keys string) (*keymap.Binding, error) {
	m.mu.RLock()
	reg := m.registry
	m.mu.RUnlock()

	// Parse the key sequence
	seq, err := key.ParseSequence(keys)
	if err != nil {
		return nil, err
	}

	ctx := keymap.NewLookupContext()
	ctx.Mode = mode
	ctx.FileType = fileType

	binding := reg.Lookup(seq, ctx)
	return binding, nil
}

// HasPrefix checks if any binding starts with the given key sequence.
func (m *KeymapManager) HasPrefix(mode, keys string) (bool, error) {
	m.mu.RLock()
	reg := m.registry
	m.mu.RUnlock()

	seq, err := key.ParseSequence(keys)
	if err != nil {
		return false, err
	}

	ctx := keymap.NewLookupContext()
	ctx.Mode = mode

	return reg.HasPrefix(seq, ctx), nil
}
