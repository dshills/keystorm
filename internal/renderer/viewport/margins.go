package viewport

// MarginConfig holds scroll margin configuration.
type MarginConfig struct {
	Top    int // Lines to keep above cursor
	Bottom int // Lines to keep below cursor
	Left   int // Columns to keep left of cursor
	Right  int // Columns to keep right of cursor
}

// DefaultMargins returns sensible default margins.
func DefaultMargins() MarginConfig {
	return MarginConfig{
		Top:    5,
		Bottom: 5,
		Left:   10,
		Right:  10,
	}
}

// CompactMargins returns smaller margins for compact views.
func CompactMargins() MarginConfig {
	return MarginConfig{
		Top:    2,
		Bottom: 2,
		Left:   5,
		Right:  5,
	}
}

// NoMargins returns zero margins (cursor can go to edge).
func NoMargins() MarginConfig {
	return MarginConfig{
		Top:    0,
		Bottom: 0,
		Left:   0,
		Right:  0,
	}
}

// SetMarginsFromConfig sets margins from a MarginConfig.
func (v *Viewport) SetMarginsFromConfig(config MarginConfig) {
	v.SetMargins(config.Top, config.Bottom, config.Left, config.Right)
}

// GetMarginConfig returns the current margins as a MarginConfig.
func (v *Viewport) GetMarginConfig() MarginConfig {
	top, bottom, left, right := v.Margins()
	return MarginConfig{
		Top:    top,
		Bottom: bottom,
		Left:   left,
		Right:  right,
	}
}

// maxMarginRatio limits margins to 1/3 of viewport dimension to ensure
// there's always usable space in the center.
const maxMarginRatio = 3

// EffectiveMargins returns margins adjusted for viewport size.
// Ensures margins don't exceed 1/3 of the viewport dimensions.
func (v *Viewport) EffectiveMargins() MarginConfig {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.clampMargins(MarginConfig{
		Top:    v.marginTop,
		Bottom: v.marginBottom,
		Left:   v.marginLeft,
		Right:  v.marginRight,
	})
}

// clampMargins applies viewport size constraints to margins (internal, no lock).
func (v *Viewport) clampMargins(config MarginConfig) MarginConfig {
	// Clamp vertical margins
	maxVertical := v.height / maxMarginRatio
	if config.Top > maxVertical {
		config.Top = maxVertical
	}
	if config.Bottom > maxVertical {
		config.Bottom = maxVertical
	}

	// Clamp horizontal margins
	maxHorizontal := v.width / maxMarginRatio
	if config.Left > maxHorizontal {
		config.Left = maxHorizontal
	}
	if config.Right > maxHorizontal {
		config.Right = maxHorizontal
	}

	return config
}

// CursorZone represents where the cursor is relative to margins.
type CursorZone uint8

const (
	ZoneCenter       CursorZone = iota // Cursor is in comfortable zone
	ZoneTopMargin                      // Cursor is in top margin
	ZoneBottomMargin                   // Cursor is in bottom margin
	ZoneLeftMargin                     // Cursor is in left margin
	ZoneRightMargin                    // Cursor is in right margin
	ZoneAbove                          // Cursor is above viewport
	ZoneBelow                          // Cursor is below viewport
	ZoneLeft                           // Cursor is left of viewport
	ZoneRight                          // Cursor is right of viewport
)

// CursorZones returns the zones where the cursor currently falls.
// Returns both vertical and horizontal zones.
func (v *Viewport) CursorZones(line uint32, col int) (vertical, horizontal CursorZone) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	margins := v.effectiveMargins()

	// Vertical zone
	if line < v.topLine {
		vertical = ZoneAbove
	} else if line > v.bottomLine() {
		vertical = ZoneBelow
	} else {
		screenRow := int(line - v.topLine)
		if screenRow < margins.Top {
			vertical = ZoneTopMargin
		} else if screenRow >= v.height-margins.Bottom {
			vertical = ZoneBottomMargin
		} else {
			vertical = ZoneCenter
		}
	}

	// Horizontal zone
	screenCol := col - v.leftColumn
	if screenCol < 0 {
		horizontal = ZoneLeft
	} else if screenCol >= v.width {
		horizontal = ZoneRight
	} else if screenCol < margins.Left {
		horizontal = ZoneLeftMargin
	} else if screenCol >= v.width-margins.Right {
		horizontal = ZoneRightMargin
	} else {
		horizontal = ZoneCenter
	}

	return
}

// effectiveMargins returns clamped margins (internal, no lock).
func (v *Viewport) effectiveMargins() MarginConfig {
	return v.clampMargins(MarginConfig{
		Top:    v.marginTop,
		Bottom: v.marginBottom,
		Left:   v.marginLeft,
		Right:  v.marginRight,
	})
}

// NeedsScrollForCursor returns true if scrolling is needed to keep cursor comfortable.
func (v *Viewport) NeedsScrollForCursor(line uint32, col int) bool {
	vZone, hZone := v.CursorZones(line, col)

	// Any zone other than center needs scroll
	return vZone != ZoneCenter || hZone != ZoneCenter
}

// VisibleContentArea returns the area of the viewport inside all margins.
// This is the "comfortable" zone where the cursor can be without triggering scroll.
type ContentArea struct {
	StartLine   uint32 // Inclusive
	EndLine     uint32 // Inclusive
	StartColumn int    // Inclusive
	EndColumn   int    // Exclusive
}

// VisibleContentArea returns the visible content area accounting for margins.
func (v *Viewport) VisibleContentArea() ContentArea {
	v.mu.RLock()
	defer v.mu.RUnlock()

	margins := v.effectiveMargins()

	// Check for degenerate case where margins exceed viewport
	if margins.Top+margins.Bottom >= v.height {
		// No usable vertical space - return minimal area at top
		return ContentArea{
			StartLine:   v.topLine,
			EndLine:     v.topLine,
			StartColumn: v.leftColumn,
			EndColumn:   v.leftColumn,
		}
	}

	startLine := v.topLine + uint32(margins.Top)
	endLine := v.bottomLine()
	if uint32(margins.Bottom) < endLine-v.topLine {
		endLine -= uint32(margins.Bottom)
	} else {
		endLine = startLine
	}

	// Ensure endLine >= startLine
	if endLine < startLine {
		endLine = startLine
	}

	startCol := v.leftColumn + margins.Left
	endCol := v.leftColumn + v.width - margins.Right
	if endCol < startCol {
		endCol = startCol
	}

	return ContentArea{
		StartLine:   startLine,
		EndLine:     endLine,
		StartColumn: startCol,
		EndColumn:   endCol,
	}
}
