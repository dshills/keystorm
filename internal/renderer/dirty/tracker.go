package dirty

import (
	"sync"
)

// ChangeType represents the type of buffer change.
type ChangeType uint8

const (
	// ChangeInsert indicates text was inserted.
	ChangeInsert ChangeType = iota

	// ChangeDelete indicates text was deleted.
	ChangeDelete

	// ChangeReplace indicates text was replaced.
	ChangeReplace

	// ChangeStyle indicates only styling changed (no content change).
	ChangeStyle

	// ChangeCursor indicates cursor position changed.
	ChangeCursor

	// ChangeSelection indicates selection changed.
	ChangeSelection

	// ChangeScroll indicates viewport scrolled.
	ChangeScroll

	// ChangeResize indicates screen was resized.
	ChangeResize

	// ChangeOverlay indicates overlay content changed.
	ChangeOverlay
)

// String returns the string representation of the change type.
func (ct ChangeType) String() string {
	switch ct {
	case ChangeInsert:
		return "insert"
	case ChangeDelete:
		return "delete"
	case ChangeReplace:
		return "replace"
	case ChangeStyle:
		return "style"
	case ChangeCursor:
		return "cursor"
	case ChangeSelection:
		return "selection"
	case ChangeScroll:
		return "scroll"
	case ChangeResize:
		return "resize"
	case ChangeOverlay:
		return "overlay"
	default:
		return "unknown"
	}
}

// Change represents a single change event.
type Change struct {
	Type   ChangeType
	Region Region
}

// Tracker tracks dirty regions and coalesces them for efficient rendering.
type Tracker struct {
	mu sync.RWMutex

	// regions contains the current dirty regions.
	regions []Region

	// fullRedraw indicates the entire screen needs redrawing.
	fullRedraw bool

	// maxRegions is the maximum number of regions before forcing full redraw.
	maxRegions int

	// screenHeight is the current screen height (for optimization).
	screenHeight uint32

	// screenWidth is the current screen width.
	screenWidth uint32

	// coalesceThreshold is the percentage of screen that triggers full redraw.
	coalesceThreshold float64
}

// NewTracker creates a new dirty region tracker.
// Negative dimensions are treated as zero.
func NewTracker(screenWidth, screenHeight int) *Tracker {
	// Clamp negative values to 0
	if screenWidth < 0 {
		screenWidth = 0
	}
	if screenHeight < 0 {
		screenHeight = 0
	}
	return &Tracker{
		regions:           make([]Region, 0, 16),
		maxRegions:        32,
		screenHeight:      uint32(screenHeight),
		screenWidth:       uint32(screenWidth),
		coalesceThreshold: 0.5, // 50% of screen = full redraw
	}
}

// SetScreenSize updates the screen dimensions.
// Negative dimensions are treated as zero.
func (t *Tracker) SetScreenSize(width, height int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Clamp negative values to 0
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	t.screenWidth = uint32(width)
	t.screenHeight = uint32(height)
	t.fullRedraw = true
}

// MarkFullRedraw marks the entire screen as needing redraw.
func (t *Tracker) MarkFullRedraw() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.fullRedraw = true
	t.regions = t.regions[:0]
}

// MarkLine marks a single line as dirty.
func (t *Tracker) MarkLine(line uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.fullRedraw {
		return
	}

	t.addRegion(NewSingleLine(line))
}

// MarkLines marks a range of lines as dirty.
func (t *Tracker) MarkLines(startLine, endLine uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.fullRedraw {
		return
	}

	t.addRegion(NewLineRegion(startLine, endLine))
}

// MarkRegion marks a rectangular region as dirty.
func (t *Tracker) MarkRegion(region Region) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.fullRedraw {
		return
	}

	t.addRegion(region)
}

// MarkChange marks a region dirty based on a change event.
func (t *Tracker) MarkChange(change Change) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.fullRedraw {
		return
	}

	switch change.Type {
	case ChangeResize, ChangeScroll:
		// These require full redraw
		t.fullRedraw = true
		t.regions = t.regions[:0]
	case ChangeInsert, ChangeDelete, ChangeReplace:
		// Content changes may affect lines below (line count change)
		region := change.Region
		if change.Type == ChangeInsert || change.Type == ChangeDelete {
			// Extend to end of screen for line-affecting changes
			// Guard against screenHeight == 0 to prevent underflow
			if t.screenHeight > 0 {
				region.EndLine = t.screenHeight - 1
			} else {
				return
			}
		}
		t.addRegion(region)
	default:
		t.addRegion(change.Region)
	}
}

// addRegion adds a region and coalesces with existing regions.
func (t *Tracker) addRegion(region Region) {
	if region.IsEmpty() {
		return
	}

	// Handle zero screen height
	if t.screenHeight == 0 {
		return
	}

	// Clamp to screen bounds
	if region.StartLine >= t.screenHeight {
		region.StartLine = t.screenHeight - 1
	}
	if region.EndLine >= t.screenHeight {
		region.EndLine = t.screenHeight - 1
	}

	// Re-validate after clamping
	if region.IsEmpty() {
		return
	}

	// Try to merge with existing regions
	for i := 0; i < len(t.regions); i++ {
		if merged, ok := t.regions[i].Merge(region); ok {
			t.regions[i] = merged
			t.coalesceRegions()
			return
		}
	}

	// Add as new region
	t.regions = append(t.regions, region)

	// Check if we have too many regions
	if len(t.regions) > t.maxRegions {
		t.coalesceRegions()
	}

	// Check if dirty area exceeds threshold
	if t.dirtyAreaRatio() > t.coalesceThreshold {
		t.fullRedraw = true
		t.regions = t.regions[:0]
	}
}

// coalesceRegions merges overlapping or adjacent regions.
func (t *Tracker) coalesceRegions() {
	if len(t.regions) <= 1 {
		return
	}

	// Simple O(n²) merge - acceptable for small region counts
	changed := true
	for changed {
		changed = false
		for i := 0; i < len(t.regions); i++ {
			for j := i + 1; j < len(t.regions); j++ {
				if merged, ok := t.regions[i].Merge(t.regions[j]); ok {
					t.regions[i] = merged
					// Remove region j
					t.regions = append(t.regions[:j], t.regions[j+1:]...)
					changed = true
					break
				}
			}
			if changed {
				break
			}
		}
	}
}

// dirtyAreaRatio returns the ratio of dirty area to total screen area.
func (t *Tracker) dirtyAreaRatio() float64 {
	if t.screenHeight == 0 || t.screenWidth == 0 {
		return 0
	}

	// Use float64 for all arithmetic to avoid integer overflow
	totalArea := float64(t.screenWidth) * float64(t.screenHeight)
	dirtyArea := float64(0)

	for _, r := range t.regions {
		lineCount := float64(r.EndLine-r.StartLine) + 1
		if r.FullWidth {
			dirtyArea += lineCount * float64(t.screenWidth)
		} else {
			colCount := float64(r.EndCol - r.StartCol)
			dirtyArea += lineCount * colCount
		}
	}

	return dirtyArea / totalArea
}

// IsDirty returns true if any region is marked dirty.
func (t *Tracker) IsDirty() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.fullRedraw || len(t.regions) > 0
}

// NeedsFullRedraw returns true if a full redraw is needed.
func (t *Tracker) NeedsFullRedraw() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.fullRedraw
}

// DirtyRegions returns a copy of the current dirty regions.
// If full redraw is needed, returns a single region covering the screen.
func (t *Tracker) DirtyRegions() []Region {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.fullRedraw {
		if t.screenHeight == 0 {
			return []Region{}
		}
		return []Region{NewLineRegion(0, t.screenHeight-1)}
	}

	result := make([]Region, len(t.regions))
	copy(result, t.regions)
	return result
}

// DirtyLines returns a list of dirty line numbers.
// Useful for line-based rendering.
func (t *Tracker) DirtyLines() []uint32 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.fullRedraw {
		lines := make([]uint32, t.screenHeight)
		for i := uint32(0); i < t.screenHeight; i++ {
			lines[i] = i
		}
		return lines
	}

	// Use a map to deduplicate lines
	lineSet := make(map[uint32]struct{})
	for _, r := range t.regions {
		for line := r.StartLine; line <= r.EndLine; line++ {
			lineSet[line] = struct{}{}
		}
	}

	lines := make([]uint32, 0, len(lineSet))
	for line := range lineSet {
		lines = append(lines, line)
	}

	// Sort lines for sequential access
	sortLines(lines)
	return lines
}

// IsLineDirty returns true if the given line needs redrawing.
func (t *Tracker) IsLineDirty(line uint32) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.fullRedraw {
		return true
	}

	for _, r := range t.regions {
		if r.ContainsLine(line) {
			return true
		}
	}

	return false
}

// IsRegionDirty returns true if any part of the region needs redrawing.
func (t *Tracker) IsRegionDirty(region Region) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.fullRedraw {
		return true
	}

	for _, r := range t.regions {
		if r.Overlaps(region) {
			return true
		}
	}

	return false
}

// Clear clears all dirty regions.
func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.regions = t.regions[:0]
	t.fullRedraw = false
}

// RegionCount returns the number of dirty regions.
func (t *Tracker) RegionCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.fullRedraw {
		return 1
	}
	return len(t.regions)
}

// SetMaxRegions sets the maximum number of regions before forcing full redraw.
// Values less than 1 are clamped to 1.
func (t *Tracker) SetMaxRegions(maxRegs int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if maxRegs < 1 {
		maxRegs = 1
	}
	t.maxRegions = maxRegs
}

// SetCoalesceThreshold sets the dirty area threshold for triggering full redraw.
func (t *Tracker) SetCoalesceThreshold(threshold float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if threshold < 0 {
		threshold = 0
	}
	if threshold > 1 {
		threshold = 1
	}
	t.coalesceThreshold = threshold
}

// Stats returns statistics about the tracker state.
func (t *Tracker) Stats() TrackerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return TrackerStats{
		RegionCount:   len(t.regions),
		FullRedraw:    t.fullRedraw,
		DirtyRatio:    t.dirtyAreaRatio(),
		ScreenWidth:   t.screenWidth,
		ScreenHeight:  t.screenHeight,
		MaxRegions:    t.maxRegions,
		CoalThreshold: t.coalesceThreshold,
	}
}

// TrackerStats contains statistics about the tracker state.
type TrackerStats struct {
	RegionCount   int
	FullRedraw    bool
	DirtyRatio    float64
	ScreenWidth   uint32
	ScreenHeight  uint32
	MaxRegions    int
	CoalThreshold float64
}

// sortLines sorts a slice of line numbers in ascending order.
func sortLines(lines []uint32) {
	// Simple insertion sort - fast for small slices
	for i := 1; i < len(lines); i++ {
		key := lines[i]
		j := i - 1
		for j >= 0 && lines[j] > key {
			lines[j+1] = lines[j]
			j--
		}
		lines[j+1] = key
	}
}
