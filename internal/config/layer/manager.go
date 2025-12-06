package layer

import (
	"fmt"
	"sort"
	"sync"
)

// Manager manages configuration layers and provides merged access.
type Manager struct {
	mu     sync.RWMutex
	layers []*Layer       // Sorted by priority (ascending)
	merged map[string]any // Cached merged result
	dirty  bool           // Whether merged cache needs refresh
}

// NewManager creates a new layer manager.
func NewManager() *Manager {
	return &Manager{
		layers: make([]*Layer, 0),
		merged: make(map[string]any),
		dirty:  true,
	}
}

// AddLayer adds a layer to the manager.
// Layers are automatically sorted by priority.
func (m *Manager) AddLayer(layer *Layer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.layers = append(m.layers, layer)
	m.sortLayers()
	m.dirty = true
}

// RemoveLayer removes a layer by name.
// Returns true if the layer was found and removed.
func (m *Manager) RemoveLayer(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, layer := range m.layers {
		if layer.Name == name {
			m.layers = append(m.layers[:i], m.layers[i+1:]...)
			m.dirty = true
			return true
		}
	}
	return false
}

// GetLayer returns a layer by name.
func (m *Manager) GetLayer(name string) *Layer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, layer := range m.layers {
		if layer.Name == name {
			return layer
		}
	}
	return nil
}

// GetLayerBySource returns the first layer with the given source.
func (m *Manager) GetLayerBySource(source Source) *Layer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, layer := range m.layers {
		if layer.Source == source {
			return layer
		}
	}
	return nil
}

// Layers returns a copy of all layers sorted by priority.
func (m *Manager) Layers() []*Layer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Layer, len(m.layers))
	copy(result, m.layers)
	return result
}

// LayerCount returns the number of layers.
func (m *Manager) LayerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.layers)
}

// Merge combines all layers into a single configuration map.
// Results are cached until a layer is added, removed, or updated.
func (m *Manager) Merge() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.dirty && m.merged != nil {
		return cloneMap(m.merged)
	}

	result := make(map[string]any)

	// Apply layers in priority order (lowest first, highest last)
	for _, layer := range m.layers {
		result = DeepMerge(result, layer.Data)
	}

	m.merged = result
	m.dirty = false

	return cloneMap(result)
}

// mergedData returns the cached merged data (internal use only).
// This refreshes the cache if dirty but returns the internal reference.
func (m *Manager) mergedData() map[string]any {
	if m.dirty || m.merged == nil {
		result := make(map[string]any)
		for _, layer := range m.layers {
			result = DeepMerge(result, layer.Data)
		}
		m.merged = result
		m.dirty = false
	}

	return m.merged
}

// Get returns the effective value for a setting path.
// Returns the value, the layer it came from, and whether it was found.
func (m *Manager) Get(path string) (any, *Layer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Search layers from highest to lowest priority
	for i := len(m.layers) - 1; i >= 0; i-- {
		layer := m.layers[i]
		if val, ok := GetByPath(layer.Data, path); ok {
			return val, layer, true
		}
	}

	return nil, nil, false
}

// GetEffectiveValue returns the merged value for a setting path.
func (m *Manager) GetEffectiveValue(path string) (any, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	merged := m.mergedData()
	return GetByPath(merged, path)
}

// Set sets a value in a specific layer.
// Returns an error if the layer is not found or is read-only.
func (m *Manager) Set(layerName, path string, value any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	layer := m.findLayer(layerName)
	if layer == nil {
		return fmt.Errorf("layer not found: %s", layerName)
	}

	if layer.ReadOnly {
		return fmt.Errorf("layer is read-only: %s", layerName)
	}

	if layer.Data == nil {
		layer.Data = make(map[string]any)
	}

	SetByPath(layer.Data, path, value)
	m.dirty = true
	return nil
}

// SetInSession sets a value in the session layer.
// Creates the session layer if it doesn't exist.
func (m *Manager) SetInSession(path string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find or create session layer
	var session *Layer
	for _, layer := range m.layers {
		if layer.Source == SourceSession {
			session = layer
			break
		}
	}

	if session == nil {
		session = NewLayer("session", SourceSession, PrioritySession)
		m.layers = append(m.layers, session)
		m.sortLayers()
	}

	if session.Data == nil {
		session.Data = make(map[string]any)
	}

	SetByPath(session.Data, path, value)
	m.dirty = true
}

// Delete removes a value from a specific layer.
func (m *Manager) Delete(layerName, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	layer := m.findLayer(layerName)
	if layer == nil {
		return fmt.Errorf("layer not found: %s", layerName)
	}

	if layer.ReadOnly {
		return fmt.Errorf("layer is read-only: %s", layerName)
	}

	if DeleteByPath(layer.Data, path) {
		m.dirty = true
	}
	return nil
}

// UpdateLayer updates a layer's data entirely.
// This replaces the layer's data and marks the manager as dirty.
func (m *Manager) UpdateLayer(name string, data map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	layer := m.findLayer(name)
	if layer == nil {
		return fmt.Errorf("layer not found: %s", name)
	}

	if layer.ReadOnly {
		return fmt.Errorf("layer is read-only: %s", name)
	}

	layer.Data = cloneMap(data)
	m.dirty = true
	return nil
}

// GetLayerValue returns a value from a specific layer.
func (m *Manager) GetLayerValue(layerName, path string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	layer := m.findLayer(layerName)
	if layer == nil {
		return nil, false
	}

	return GetByPath(layer.Data, path)
}

// WhichLayer returns the name of the layer that provides a value.
func (m *Manager) WhichLayer(path string) string {
	_, layer, found := m.Get(path)
	if !found {
		return ""
	}
	return layer.Name
}

// Invalidate marks the merged cache as dirty.
// Call this after modifying layer data directly.
func (m *Manager) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirty = true
}

// sortLayers sorts layers by priority (ascending).
func (m *Manager) sortLayers() {
	sort.Slice(m.layers, func(i, j int) bool {
		return m.layers[i].Priority < m.layers[j].Priority
	})
}

// findLayer finds a layer by name (must be called with lock held).
func (m *Manager) findLayer(name string) *Layer {
	for _, layer := range m.layers {
		if layer.Name == name {
			return layer
		}
	}
	return nil
}

// Clear removes all layers and releases memory.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.layers = nil
	m.merged = nil
	m.dirty = true
}
