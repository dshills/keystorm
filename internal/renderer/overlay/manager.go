package overlay

import (
	"sort"
	"sync"

	"github.com/dshills/keystorm/internal/renderer/core"
)

// Manager manages all overlays and composites them for rendering.
type Manager struct {
	mu sync.RWMutex

	// overlays contains all registered overlays, keyed by ID.
	overlays map[string]Overlay

	// sortedIDs contains overlay IDs sorted by priority.
	sortedIDs []string

	// needsSort indicates the sortedIDs needs re-sorting.
	needsSort bool

	// config holds the overlay configuration.
	config Config

	// activeGhostText is the currently active ghost text overlay.
	activeGhostText *GhostText

	// activeDiff is the currently active diff preview.
	activeDiff *DiffPreview
}

// NewManager creates a new overlay manager.
func NewManager(config Config) *Manager {
	return &Manager{
		overlays:  make(map[string]Overlay),
		sortedIDs: make([]string, 0),
		config:    config,
	}
}

// Config returns the current configuration.
func (m *Manager) Config() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SetConfig updates the configuration.
func (m *Manager) SetConfig(config Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// Add adds an overlay to the manager.
func (m *Manager) Add(overlay Overlay) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := overlay.ID()
	m.overlays[id] = overlay
	m.sortedIDs = append(m.sortedIDs, id)
	m.needsSort = true

	// Track special overlays
	if gt, ok := overlay.(*GhostText); ok {
		m.activeGhostText = gt
	}
	if dp, ok := overlay.(*DiffPreview); ok {
		m.activeDiff = dp
	}
}

// Remove removes an overlay by ID.
func (m *Manager) Remove(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	overlay, ok := m.overlays[id]
	if !ok {
		return false
	}

	delete(m.overlays, id)

	// Remove from sorted list
	for i, sid := range m.sortedIDs {
		if sid == id {
			m.sortedIDs = append(m.sortedIDs[:i], m.sortedIDs[i+1:]...)
			break
		}
	}

	// Clear special overlay references
	if gt, ok := overlay.(*GhostText); ok && m.activeGhostText == gt {
		m.activeGhostText = nil
	}
	if dp, ok := overlay.(*DiffPreview); ok && m.activeDiff == dp {
		m.activeDiff = nil
	}

	return true
}

// Get returns an overlay by ID.
func (m *Manager) Get(id string) (Overlay, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	overlay, ok := m.overlays[id]
	return overlay, ok
}

// Clear removes all overlays.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.overlays = make(map[string]Overlay)
	m.sortedIDs = make([]string, 0)
	m.activeGhostText = nil
	m.activeDiff = nil
}

// ClearType removes all overlays of a specific type.
func (m *Manager) ClearType(typ Type) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var toRemove []string
	for id, overlay := range m.overlays {
		if overlay.Type() == typ {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		delete(m.overlays, id)
	}

	// Rebuild sorted list
	m.sortedIDs = make([]string, 0, len(m.overlays))
	for id := range m.overlays {
		m.sortedIDs = append(m.sortedIDs, id)
	}
	m.needsSort = true

	// Clear special overlay references if removed
	if m.activeGhostText != nil {
		if _, ok := m.overlays[m.activeGhostText.ID()]; !ok {
			m.activeGhostText = nil
		}
	}
	if m.activeDiff != nil {
		if _, ok := m.overlays[m.activeDiff.ID()]; !ok {
			m.activeDiff = nil
		}
	}
}

// Count returns the number of overlays.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.overlays)
}

// OverlaysOnLine returns all overlays that affect the given line.
func (m *Manager) OverlaysOnLine(line uint32) []Overlay {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Overlay
	for _, overlay := range m.overlays {
		if overlay.IsVisible() && overlay.Range().ContainsLine(line) {
			result = append(result, overlay)
		}
	}

	// Sort by priority
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority() < result[j].Priority()
	})

	return result
}

// SpansForLine returns all overlay spans for a line, sorted by priority.
func (m *Manager) SpansForLine(line uint32) []Span {
	m.mu.Lock()
	m.ensureSorted()
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	var spans []Span
	for _, id := range m.sortedIDs {
		overlay := m.overlays[id]
		if !overlay.IsVisible() {
			continue
		}
		if !overlay.Range().ContainsLine(line) {
			continue
		}

		// Check if overlay type is enabled
		if !m.isTypeEnabled(overlay.Type()) {
			continue
		}

		overlaySpans := overlay.SpansForLine(line)
		spans = append(spans, overlaySpans...)
	}

	return spans
}

// isTypeEnabled checks if an overlay type is enabled in config.
func (m *Manager) isTypeEnabled(typ Type) bool {
	switch typ {
	case TypeGhostText:
		return m.config.ShowGhostText
	case TypeDiffAdd, TypeDiffDelete, TypeDiffModify:
		return m.config.ShowDiffPreview
	case TypeDiagnostic:
		return m.config.ShowDiagnostics
	default:
		return true
	}
}

// ensureSorted ensures the sortedIDs list is sorted by priority.
func (m *Manager) ensureSorted() {
	if !m.needsSort {
		return
	}

	sort.Slice(m.sortedIDs, func(i, j int) bool {
		oi := m.overlays[m.sortedIDs[i]]
		oj := m.overlays[m.sortedIDs[j]]
		return oi.Priority() < oj.Priority()
	})

	m.needsSort = false
}

// ActiveGhostText returns the currently active ghost text, if any.
func (m *Manager) ActiveGhostText() *GhostText {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeGhostText
}

// ActiveDiff returns the currently active diff preview, if any.
func (m *Manager) ActiveDiff() *DiffPreview {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeDiff
}

// SetGhostText sets or replaces the active ghost text.
func (m *Manager) SetGhostText(gt *GhostText) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove existing ghost text
	if m.activeGhostText != nil {
		m.removeOverlayLocked(m.activeGhostText.ID())
	}

	if gt != nil {
		m.overlays[gt.ID()] = gt
		m.sortedIDs = append(m.sortedIDs, gt.ID())
		m.needsSort = true
	}
	m.activeGhostText = gt
}

// ClearGhostText removes any active ghost text.
func (m *Manager) ClearGhostText() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeGhostText != nil {
		m.removeOverlayLocked(m.activeGhostText.ID())
		m.activeGhostText = nil
	}
}

// SetDiffPreview sets or replaces the active diff preview.
func (m *Manager) SetDiffPreview(dp *DiffPreview) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove existing diff preview
	if m.activeDiff != nil {
		m.removeOverlayLocked(m.activeDiff.ID())
	}

	if dp != nil {
		m.overlays[dp.ID()] = dp
		m.sortedIDs = append(m.sortedIDs, dp.ID())
		m.needsSort = true
	}
	m.activeDiff = dp
}

// ClearDiffPreview removes any active diff preview.
func (m *Manager) ClearDiffPreview() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeDiff != nil {
		m.removeOverlayLocked(m.activeDiff.ID())
		m.activeDiff = nil
	}
}

// removeOverlayLocked removes an overlay (must hold write lock).
func (m *Manager) removeOverlayLocked(id string) {
	delete(m.overlays, id)
	for i, sid := range m.sortedIDs {
		if sid == id {
			m.sortedIDs = append(m.sortedIDs[:i], m.sortedIDs[i+1:]...)
			break
		}
	}
}

// AcceptGhostText accepts the active ghost text.
// Returns the text that was accepted, or empty string if none.
func (m *Manager) AcceptGhostText() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeGhostText == nil || !m.activeGhostText.IsVisible() {
		return ""
	}

	text := m.activeGhostText.Text()
	m.activeGhostText.Accept()
	m.removeOverlayLocked(m.activeGhostText.ID())
	m.activeGhostText = nil

	return text
}

// AcceptGhostTextPartial accepts part of the active ghost text.
// Returns the text that was accepted.
func (m *Manager) AcceptGhostTextPartial() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeGhostText == nil || !m.activeGhostText.IsVisible() {
		return ""
	}

	accepted := m.activeGhostText.AcceptPartial()
	if m.activeGhostText.IsAccepted() {
		m.removeOverlayLocked(m.activeGhostText.ID())
		m.activeGhostText = nil
	}

	return accepted
}

// RejectGhostText rejects the active ghost text.
func (m *Manager) RejectGhostText() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeGhostText != nil {
		m.activeGhostText.Reject()
		m.removeOverlayLocked(m.activeGhostText.ID())
		m.activeGhostText = nil
	}
}

// AcceptDiff accepts the active diff preview.
// Returns true if a diff was accepted.
func (m *Manager) AcceptDiff() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeDiff == nil || !m.activeDiff.IsVisible() {
		return false
	}

	m.activeDiff.Accept()
	m.removeOverlayLocked(m.activeDiff.ID())
	m.activeDiff = nil

	return true
}

// RejectDiff rejects the active diff preview.
func (m *Manager) RejectDiff() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeDiff != nil {
		m.activeDiff.Reject()
		m.removeOverlayLocked(m.activeDiff.ID())
		m.activeDiff = nil
	}
}

// Compositor composites overlay spans onto base content.
type Compositor struct {
	config Config
}

// NewCompositor creates a new compositor.
func NewCompositor(config Config) *Compositor {
	return &Compositor{config: config}
}

// SetConfig updates the compositor configuration.
func (c *Compositor) SetConfig(config Config) {
	c.config = config
}

// CompositeCell applies an overlay span to a base cell.
func (c *Compositor) CompositeCell(base core.Cell, span Span, col uint32) core.Cell {
	// Check if column is within span
	if col < span.StartCol {
		return base
	}
	if span.EndCol > 0 && col >= span.EndCol {
		return base
	}

	result := base

	// Apply overlay style
	result.Style = MergeStyles(base.Style, span.Style)

	// Replace content if specified
	if span.ReplaceContent && span.Text != "" {
		runeIdx := int(col - span.StartCol)
		runes := []rune(span.Text)
		if runeIdx >= 0 && runeIdx < len(runes) {
			r := runes[runeIdx]
			result.Rune = r
			result.Width = core.RuneWidth(r)
		}
	}

	return result
}

// CompositeLine composites all overlay spans onto a line of cells.
func (c *Compositor) CompositeLine(baseCells []core.Cell, spans []Span) []core.Cell {
	if len(spans) == 0 {
		return baseCells
	}

	// Make a copy to avoid modifying original
	result := make([]core.Cell, len(baseCells))
	copy(result, baseCells)

	// Collect after-content spans
	var afterSpans []Span

	// Apply overlay spans
	for _, span := range spans {
		if span.AfterContent {
			afterSpans = append(afterSpans, span)
			continue
		}

		// Apply to existing cells
		for col := span.StartCol; col < uint32(len(result)); col++ {
			if span.EndCol > 0 && col >= span.EndCol {
				break
			}
			result[col] = c.CompositeCell(result[col], span, col)
		}
	}

	// Append after-content spans
	for _, span := range afterSpans {
		if span.Text == "" {
			continue
		}
		for _, r := range span.Text {
			result = append(result, core.Cell{
				Rune:  r,
				Width: core.RuneWidth(r),
				Style: span.Style,
			})
		}
	}

	return result
}

// OverlayLine represents a line with overlay information.
type OverlayLine struct {
	// Line is the line number.
	Line uint32

	// Cells contains the composited cells.
	Cells []core.Cell

	// HasGhostText indicates if this line has ghost text.
	HasGhostText bool

	// HasDiff indicates if this line has diff overlay.
	HasDiff bool

	// InsertedLines contains any lines to insert after this line.
	InsertedLines [][]core.Cell
}
