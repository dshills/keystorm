# Keystorm Rendering Layer - Implementation Plan

## Comprehensive Design Document for `internal/renderer`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Requirements Analysis](#2-requirements-analysis)
3. [Architecture Overview](#3-architecture-overview)
4. [Package Structure](#4-package-structure)
5. [Core Types and Interfaces](#5-core-types-and-interfaces)
6. [Line Layout System](#6-line-layout-system)
7. [Syntax Highlighting Integration](#7-syntax-highlighting-integration)
8. [Viewport and Scrolling](#8-viewport-and-scrolling)
9. [Dirty Region Tracking](#9-dirty-region-tracking)
10. [Terminal Backend](#10-terminal-backend)
11. [Cursor and Selection Rendering](#11-cursor-and-selection-rendering)
12. [AI Overlay Rendering](#12-ai-overlay-rendering)
13. [Implementation Phases](#13-implementation-phases)
14. [Testing Strategy](#14-testing-strategy)
15. [Performance Considerations](#15-performance-considerations)

---

## 1. Executive Summary

The rendering layer is responsible for transforming the internal buffer state into visual output on screen. This document outlines the design for Keystorm's rendering system, which must support:

- **Terminal-based rendering** (initial target) with future GUI extensibility
- **60+ FPS rendering** for smooth editing experience
- **Efficient dirty region updates** to minimize redraw work
- **Syntax highlighting** via Tree-sitter and LSP semantic tokens
- **AI integration overlays** (ghost text, diff previews, inline suggestions)
- **Multi-cursor visualization**
- **Large file support** (100k+ lines with virtualized rendering)

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Viewport-based rendering | Only render visible lines; essential for large file performance |
| Line cache with invalidation | Avoid re-computing layout for unchanged lines |
| Style spans (not per-char) | Memory efficient; matches how highlighting data is provided |
| Backend abstraction | Support terminal now, GUI later (tcell -> future OpenGL/Metal) |
| Double-buffered output | Prevent flickering; compute diff for minimal terminal writes |
| Event-driven updates | React to buffer changes, cursor moves, scroll events |

---

## 2. Requirements Analysis

### 2.1 From Architecture Specification

From `design/specs/architecture.md`, the rendering layer must handle:

- **Line layout**: Convert buffer content to visual lines
- **Syntax coloration**: Apply highlighting from multiple sources
- **60+ FPS rendering**: Smooth updates during typing and scrolling
- **Scrolling**: Horizontal and vertical scroll with smooth animation
- **Dirty region updates**: Incremental redraw for efficiency

### 2.2 Integration Points

The renderer integrates with:

| Component | Integration |
|-----------|-------------|
| `engine.Engine` | Read buffer content, line text, cursor positions |
| `engine.tracking` | Subscribe to change events for invalidation |
| `event` (future) | Receive buffer change, cursor move, scroll events |
| `lsp` (future) | Receive semantic tokens for highlighting |
| Tree-sitter (future) | Receive syntax highlighting spans |
| `config` (future) | Theme colors, font settings, tab width |

### 2.3 Functional Requirements

1. **Basic Text Display**
   - Render buffer lines within viewport
   - Handle variable-width characters (tabs, Unicode)
   - Line numbers with configurable width
   - Word wrap (optional, configurable)

2. **Cursor and Selection**
   - Multiple cursor rendering with distinct styling
   - Selection highlighting (primary and secondary)
   - Cursor blink animation
   - Block/line/underline cursor styles (mode-dependent)

3. **Syntax Highlighting**
   - Tree-sitter-based highlighting (fast, incremental)
   - LSP semantic tokens (enhanced highlighting)
   - Merged/prioritized style application
   - Bracket matching visualization

4. **AI Integration Overlays**
   - Ghost text for inline completions
   - Diff preview (additions in green, deletions in red)
   - Inline suggestions with accept/reject UI
   - Progress indicators for async AI operations

5. **Scrolling**
   - Smooth vertical scrolling
   - Horizontal scrolling for long lines
   - Scroll margins (keep cursor visible with context)
   - Minimap (optional, future)

6. **Performance**
   - Virtualized rendering for large files
   - Incremental updates (dirty regions only)
   - Frame rate limiting (cap at 60 FPS)
   - Background syntax highlighting

---

## 3. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Renderer (Facade)                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │   Viewport   │  │  LineCache   │  │    StyleResolver         │  │
│  │  Management  │  │              │  │  (highlight + semantic)  │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │   Scrolling  │  │ DirtyRegion  │  │     CursorRenderer       │  │
│  │   Manager    │  │   Tracker    │  │                          │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │  AIOverlay   │  │  LineLayout  │  │      GutterRenderer      │  │
│  │  Renderer    │  │   Engine     │  │   (line nums, signs)     │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                       Backend Abstraction                           │
├─────────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │   Terminal   │  │    Screen    │  │      Future: GUI         │  │
│  │   (tcell)    │  │    Buffer    │  │   (OpenGL/Metal/etc)     │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Data Flow

```
Buffer Change Event
       │
       ▼
┌──────────────────┐
│ DirtyRegion      │  Mark affected lines as dirty
│ Tracker          │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ LineCache        │  Invalidate cached line layouts
│                  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Render Request   │  Triggered by event loop (vsync/timer)
│                  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Viewport         │  Determine visible line range
│ Management       │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ LineLayout       │  Compute visual layout for dirty lines
│ Engine           │  (fetch from cache or compute fresh)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ StyleResolver    │  Apply syntax + semantic highlighting
│                  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Screen Buffer    │  Build cell grid with styled content
│                  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Backend          │  Output to terminal (diff with previous frame)
│                  │
└──────────────────┘
```

---

## 4. Package Structure

```
internal/renderer/
    doc.go                      # Package documentation

    # Core types
    types.go                    # Common types (Color, Style, Cell, etc.)
    options.go                  # Configuration options

    # Main renderer facade
    renderer.go                 # Renderer type (main API)
    view.go                     # View type (single editor view)

    # Layout system
    layout/
        line.go                 # LineLayout type
        wrap.go                 # Word wrap algorithms
        tab.go                  # Tab expansion
        unicode.go              # Unicode width handling
        cache.go                # LineCache for layout caching
        layout_test.go

    # Viewport and scrolling
    viewport/
        viewport.go             # Viewport type (visible region)
        scroll.go               # Scroll state and animation
        margins.go              # Scroll margins (cursor context)
        viewport_test.go

    # Dirty region tracking
    dirty/
        tracker.go              # DirtyTracker type
        region.go               # DirtyRegion representation
        dirty_test.go

    # Style and highlighting
    style/
        style.go                # Style type (fg, bg, attrs)
        span.go                 # StyleSpan (range + style)
        resolver.go             # StyleResolver (merge multiple sources)
        theme.go                # Theme definition
        style_test.go

    # Cursor and selection rendering
    cursor/
        cursor.go               # Cursor rendering logic
        selection.go            # Selection highlight rendering
        blink.go                # Cursor blink animation
        cursor_test.go

    # AI overlay rendering
    overlay/
        ghost.go                # Ghost text (inline completion preview)
        diff.go                 # Diff preview rendering
        progress.go             # Progress indicators
        overlay_test.go

    # Gutter (line numbers, signs, etc.)
    gutter/
        gutter.go               # Gutter renderer
        linenums.go             # Line number rendering
        signs.go                # Sign column (breakpoints, errors, etc.)
        fold.go                 # Fold markers
        gutter_test.go

    # Backend abstraction
    backend/
        backend.go              # Backend interface
        terminal.go             # Terminal backend (tcell)
        buffer.go               # Screen buffer (double-buffered cells)
        diff.go                 # Screen diff for minimal updates
        backend_test.go

    # Tests
    renderer_test.go
    bench_test.go
```

### Rationale

- **Separation of concerns**: Each sub-package handles one aspect
- **Testability**: Components can be tested independently
- **Extensibility**: Easy to add new overlay types, backends, etc.
- **Performance isolation**: Hot paths (layout, diff) are isolated for optimization

---

## 5. Core Types and Interfaces

### 5.1 Color and Style Types

```go
// internal/renderer/types.go

// Color represents a color value.
// Supports true color (RGB) and terminal palette colors.
type Color struct {
    R, G, B uint8
    Indexed bool   // If true, R is a palette index (0-255)
}

// Predefined colors
var (
    ColorDefault = Color{Indexed: true, R: 0}   // Terminal default
    ColorBlack   = Color{R: 0, G: 0, B: 0}
    ColorWhite   = Color{R: 255, G: 255, B: 255}
    // ... more colors
)

// ColorFromHex creates a color from a hex string like "#FF5500".
func ColorFromHex(hex string) (Color, error) {
    // Implementation
}

// Attribute represents text attributes (bold, italic, etc.).
type Attribute uint16

const (
    AttrNone      Attribute = 0
    AttrBold      Attribute = 1 << iota
    AttrDim
    AttrItalic
    AttrUnderline
    AttrBlink
    AttrReverse
    AttrStrikethrough
    AttrHidden
)

// Style represents the visual style of text.
type Style struct {
    Foreground Color
    Background Color
    Attributes Attribute
}

// DefaultStyle returns the default terminal style.
func DefaultStyle() Style {
    return Style{
        Foreground: ColorDefault,
        Background: ColorDefault,
        Attributes: AttrNone,
    }
}

// Merge combines two styles, with other taking precedence.
func (s Style) Merge(other Style) Style {
    result := s
    if other.Foreground != ColorDefault {
        result.Foreground = other.Foreground
    }
    if other.Background != ColorDefault {
        result.Background = other.Background
    }
    result.Attributes |= other.Attributes
    return result
}

// Cell represents a single terminal cell.
type Cell struct {
    Rune  rune
    Width int   // Display width (1 for most chars, 2 for wide CJK)
    Style Style
}

// EmptyCell returns an empty cell with default style.
func EmptyCell() Cell {
    return Cell{Rune: ' ', Width: 1, Style: DefaultStyle()}
}
```

### 5.2 Coordinate Types

```go
// internal/renderer/types.go (continued)

// ScreenPos represents a position on screen (0-indexed).
type ScreenPos struct {
    Row int // Screen row (0 = top)
    Col int // Screen column (0 = left)
}

// ScreenRect represents a rectangular region on screen.
type ScreenRect struct {
    Top    int
    Left   int
    Bottom int // Exclusive
    Right  int // Exclusive
}

// Width returns the width of the rectangle.
func (r ScreenRect) Width() int {
    return r.Right - r.Left
}

// Height returns the height of the rectangle.
func (r ScreenRect) Height() int {
    return r.Bottom - r.Top
}

// Contains returns true if pos is within the rectangle.
func (r ScreenRect) Contains(pos ScreenPos) bool {
    return pos.Row >= r.Top && pos.Row < r.Bottom &&
           pos.Col >= r.Left && pos.Col < r.Right
}

// Intersects returns true if two rectangles overlap.
func (r ScreenRect) Intersects(other ScreenRect) bool {
    return r.Left < other.Right && r.Right > other.Left &&
           r.Top < other.Bottom && r.Bottom > other.Top
}

// Intersection returns the overlapping region.
func (r ScreenRect) Intersection(other ScreenRect) ScreenRect {
    return ScreenRect{
        Top:    max(r.Top, other.Top),
        Left:   max(r.Left, other.Left),
        Bottom: min(r.Bottom, other.Bottom),
        Right:  min(r.Right, other.Right),
    }
}
```

### 5.3 Core Interfaces

```go
// internal/renderer/renderer.go

// BufferReader provides read access to buffer content.
// This interface abstracts the engine for rendering.
type BufferReader interface {
    // Content access
    LineText(line uint32) string
    LineCount() uint32
    Len() int64

    // Tab configuration
    TabWidth() int
}

// CursorProvider provides cursor/selection information.
type CursorProvider interface {
    // Primary cursor position (line, column)
    PrimaryCursor() (line uint32, col uint32)

    // All selections for rendering
    Selections() []Selection

    // Check if position is in a selection
    IsSelected(line uint32, col uint32) bool
}

// Selection represents a selection range for rendering.
type Selection struct {
    StartLine uint32
    StartCol  uint32
    EndLine   uint32
    EndCol    uint32
    IsPrimary bool
}

// HighlightProvider provides syntax highlighting information.
type HighlightProvider interface {
    // Get style spans for a line range.
    // Returns spans sorted by start position.
    HighlightsForLine(line uint32) []StyleSpan

    // Called when lines change to invalidate caches.
    InvalidateLines(startLine, endLine uint32)
}

// StyleSpan represents a styled range within a line.
type StyleSpan struct {
    StartCol uint32
    EndCol   uint32 // Exclusive
    Style    Style
}

// OverlayProvider provides AI overlay information.
type OverlayProvider interface {
    // Ghost text at cursor position
    GhostText() (text string, line uint32, col uint32, ok bool)

    // Diff overlay for preview
    DiffOverlay() *DiffOverlay

    // Progress indicator
    Progress() (message string, percent float64, ok bool)
}

// DiffOverlay represents a diff preview.
type DiffOverlay struct {
    Hunks []DiffHunk
}

// DiffHunk represents a single diff section.
type DiffHunk struct {
    StartLine   uint32
    DeletedText []string // Lines to show as deleted
    AddedText   []string // Lines to show as added
}
```

### 5.4 Renderer Interface

```go
// internal/renderer/renderer.go (continued)

// Renderer is the main rendering facade.
type Renderer struct {
    // Configuration
    opts     Options
    theme    *Theme

    // Screen state
    backend  Backend
    buffer   *ScreenBuffer
    width    int
    height   int

    // Content providers
    bufReader   BufferReader
    cursorProv  CursorProvider
    hlProvider  HighlightProvider
    overlayProv OverlayProvider

    // Sub-components
    viewport    *Viewport
    lineCache   *LineCache
    dirtyTrack  *DirtyTracker
    gutterRend  *GutterRenderer
    cursorRend  *CursorRenderer
    overlayRend *OverlayRenderer

    // State
    lastFrame   time.Time
    frameCount  uint64
    needsRedraw bool
}

// Options configures the renderer.
type Options struct {
    // Display
    ShowLineNumbers  bool
    LineNumberWidth  int // 0 = auto
    WordWrap         bool
    WrapAtColumn     int // 0 = window width

    // Scrolling
    ScrollMarginTop    int
    ScrollMarginBottom int
    ScrollMarginLeft   int
    ScrollMarginRight  int
    SmoothScroll       bool

    // Cursor
    CursorStyle      CursorStyle
    CursorBlink      bool
    CursorBlinkRate  time.Duration

    // Performance
    MaxFPS           int
    LazyHighlighting bool
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
    return Options{
        ShowLineNumbers:    true,
        LineNumberWidth:    0, // Auto
        WordWrap:           false,
        ScrollMarginTop:    5,
        ScrollMarginBottom: 5,
        ScrollMarginLeft:   10,
        ScrollMarginRight:  10,
        SmoothScroll:       true,
        CursorStyle:        CursorBlock,
        CursorBlink:        true,
        CursorBlinkRate:    500 * time.Millisecond,
        MaxFPS:             60,
        LazyHighlighting:   true,
    }
}

// CursorStyle defines cursor appearance.
type CursorStyle uint8

const (
    CursorBlock CursorStyle = iota
    CursorBar
    CursorUnderline
)
```

---

## 6. Line Layout System

The line layout system converts buffer lines into visual representations, handling tabs, variable-width characters, and word wrapping.

### 6.1 LineLayout Type

```go
// internal/renderer/layout/line.go

// LineLayout represents the visual layout of a single buffer line.
type LineLayout struct {
    // Source information
    BufferLine uint32 // The buffer line number

    // Visual representation
    Cells      []Cell   // Visual cells (after tab expansion, etc.)
    VisualCols []uint32 // Map visual column -> buffer column
    BufferCols []uint32 // Map buffer column -> visual column

    // Wrapping (if enabled)
    WrapPoints []int    // Visual columns where line wraps
    RowCount   int      // Number of visual rows (1 if no wrap)

    // Metadata
    Width      int      // Total visual width
    HasTabs    bool     // Contains tab characters
    HasWide    bool     // Contains wide (CJK) characters
}

// VisualColumn converts a buffer column to visual column.
func (l *LineLayout) VisualColumn(bufCol uint32) int {
    if int(bufCol) >= len(l.BufferCols) {
        // Beyond end of line
        if len(l.BufferCols) == 0 {
            return 0
        }
        // Extrapolate
        return int(l.BufferCols[len(l.BufferCols)-1]) + int(bufCol) - len(l.BufferCols) + 1
    }
    return int(l.BufferCols[bufCol])
}

// BufferColumn converts a visual column to buffer column.
func (l *LineLayout) BufferColumn(visCol int) uint32 {
    if visCol >= len(l.VisualCols) {
        if len(l.VisualCols) == 0 {
            return 0
        }
        // Beyond end of line
        return l.VisualCols[len(l.VisualCols)-1] + uint32(visCol-len(l.VisualCols)+1)
    }
    return l.VisualCols[visCol]
}

// VisualRow returns which visual row a visual column falls on.
// Returns 0 if no wrapping or column is on first row.
func (l *LineLayout) VisualRow(visCol int) int {
    if len(l.WrapPoints) == 0 {
        return 0
    }
    row := 0
    for _, wp := range l.WrapPoints {
        if visCol >= wp {
            row++
        } else {
            break
        }
    }
    return row
}
```

### 6.2 Layout Engine

```go
// internal/renderer/layout/line.go (continued)

// LayoutEngine computes line layouts.
type LayoutEngine struct {
    tabWidth   int
    wrapWidth  int  // 0 = no wrap
    wrapAtWord bool
}

// NewLayoutEngine creates a layout engine with the given settings.
func NewLayoutEngine(tabWidth int) *LayoutEngine {
    return &LayoutEngine{
        tabWidth:   tabWidth,
        wrapWidth:  0,
        wrapAtWord: true,
    }
}

// SetWrap configures word wrapping.
func (e *LayoutEngine) SetWrap(width int, atWord bool) {
    e.wrapWidth = width
    e.wrapAtWord = atWord
}

// Layout computes the visual layout for a line.
func (e *LayoutEngine) Layout(line string, bufferLine uint32) *LineLayout {
    layout := &LineLayout{
        BufferLine: bufferLine,
        Cells:      make([]Cell, 0, len(line)),
        VisualCols: make([]uint32, 0, len(line)),
        BufferCols: make([]uint32, 0, len(line)),
        RowCount:   1,
    }

    visCol := 0
    bufCol := uint32(0)

    for _, r := range line {
        startVisCol := visCol

        // Record buffer -> visual mapping
        for len(layout.BufferCols) <= int(bufCol) {
            layout.BufferCols = append(layout.BufferCols, uint32(visCol))
        }

        if r == '\t' {
            // Tab expansion
            layout.HasTabs = true
            tabStop := e.tabWidth - (visCol % e.tabWidth)
            for i := 0; i < tabStop; i++ {
                layout.Cells = append(layout.Cells, Cell{
                    Rune:  ' ',
                    Width: 1,
                    Style: DefaultStyle(),
                })
                layout.VisualCols = append(layout.VisualCols, bufCol)
                visCol++
            }
        } else {
            // Regular character
            width := runeWidth(r)
            if width == 2 {
                layout.HasWide = true
            }

            layout.Cells = append(layout.Cells, Cell{
                Rune:  r,
                Width: width,
                Style: DefaultStyle(),
            })

            // For wide characters, add a placeholder for the second cell
            if width == 2 {
                layout.Cells = append(layout.Cells, Cell{
                    Rune:  0, // Continuation marker
                    Width: 0,
                    Style: DefaultStyle(),
                })
            }

            for i := 0; i < width; i++ {
                layout.VisualCols = append(layout.VisualCols, bufCol)
                visCol++
            }
        }

        bufCol++

        // Check for word wrap
        if e.wrapWidth > 0 && visCol >= e.wrapWidth {
            wrapPoint := e.findWrapPoint(layout, startVisCol)
            layout.WrapPoints = append(layout.WrapPoints, wrapPoint)
            layout.RowCount++
        }
    }

    layout.Width = visCol
    return layout
}

// findWrapPoint finds the best point to wrap (at word boundary if possible).
func (e *LayoutEngine) findWrapPoint(layout *LineLayout, afterCol int) int {
    if !e.wrapAtWord {
        return afterCol
    }

    // Look backward for a space
    for i := afterCol; i > afterCol-20 && i > 0; i-- {
        if layout.Cells[i].Rune == ' ' {
            return i + 1
        }
    }

    // No good wrap point, wrap at column
    return afterCol
}

// runeWidth returns the display width of a rune.
func runeWidth(r rune) int {
    // East Asian Width calculation
    // Simplified version - should use unicode.EastAsianWidth for accuracy
    if r >= 0x1100 && (
        (r <= 0x115F) || // Hangul Jamo
        (r >= 0x2E80 && r <= 0x9FFF) || // CJK
        (r >= 0xAC00 && r <= 0xD7A3) || // Hangul Syllables
        (r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility
        (r >= 0xFE10 && r <= 0xFE1F) || // Vertical forms
        (r >= 0xFE30 && r <= 0xFE6F) || // CJK Compatibility Forms
        (r >= 0xFF00 && r <= 0xFF60) || // Fullwidth Forms
        (r >= 0xFFE0 && r <= 0xFFE6)) { // Fullwidth Forms
        return 2
    }
    if r < 32 || r == 0x7F {
        return 0 // Control characters
    }
    return 1
}
```

### 6.3 Line Cache

```go
// internal/renderer/layout/cache.go

// LineCache caches computed line layouts.
type LineCache struct {
    mu      sync.RWMutex
    entries map[uint32]*cacheEntry
    engine  *LayoutEngine
    maxSize int
}

type cacheEntry struct {
    layout     *LineLayout
    lineHash   uint64 // Hash of line content for validation
    lastAccess time.Time
}

// NewLineCache creates a new line cache.
func NewLineCache(engine *LayoutEngine, maxSize int) *LineCache {
    return &LineCache{
        entries: make(map[uint32]*cacheEntry),
        engine:  engine,
        maxSize: maxSize,
    }
}

// Get retrieves or computes the layout for a line.
func (c *LineCache) Get(line uint32, text string) *LineLayout {
    hash := hashLine(text)

    c.mu.RLock()
    entry, ok := c.entries[line]
    if ok && entry.lineHash == hash {
        entry.lastAccess = time.Now()
        c.mu.RUnlock()
        return entry.layout
    }
    c.mu.RUnlock()

    // Compute layout
    layout := c.engine.Layout(text, line)

    c.mu.Lock()
    defer c.mu.Unlock()

    c.entries[line] = &cacheEntry{
        layout:     layout,
        lineHash:   hash,
        lastAccess: time.Now(),
    }

    // Evict if too large
    if len(c.entries) > c.maxSize {
        c.evict()
    }

    return layout
}

// Invalidate marks a line as needing re-layout.
func (c *LineCache) Invalidate(line uint32) {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.entries, line)
}

// InvalidateRange marks a range of lines as needing re-layout.
func (c *LineCache) InvalidateRange(startLine, endLine uint32) {
    c.mu.Lock()
    defer c.mu.Unlock()
    for line := startLine; line <= endLine; line++ {
        delete(c.entries, line)
    }
}

// InvalidateAll clears the entire cache.
func (c *LineCache) InvalidateAll() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.entries = make(map[uint32]*cacheEntry)
}

// evict removes the least recently used entries.
func (c *LineCache) evict() {
    // Find entries to evict (keep newest half)
    entries := make([]*struct {
        line uint32
        time time.Time
    }, 0, len(c.entries))

    for line, entry := range c.entries {
        entries = append(entries, &struct {
            line uint32
            time time.Time
        }{line, entry.lastAccess})
    }

    // Sort by access time
    sort.Slice(entries, func(i, j int) bool {
        return entries[i].time.Before(entries[j].time)
    })

    // Remove oldest half
    toRemove := len(entries) / 2
    for i := 0; i < toRemove; i++ {
        delete(c.entries, entries[i].line)
    }
}

// hashLine computes a hash of line content.
func hashLine(s string) uint64 {
    h := fnv.New64a()
    h.Write([]byte(s))
    return h.Sum64()
}
```

---

## 7. Syntax Highlighting Integration

### 7.1 Style Resolver

```go
// internal/renderer/style/resolver.go

// Source identifies the origin of styling.
type Source uint8

const (
    SourceDefault Source = iota
    SourceSyntax         // Tree-sitter
    SourceSemantic       // LSP semantic tokens
    SourceDiagnostic     // Errors, warnings
    SourceSearch         // Search highlights
    SourceSelection      // Selection highlight
    SourceAI             // AI overlays
)

// Priority returns the priority of a source (higher = wins).
func (s Source) Priority() int {
    return int(s) // Higher source values have higher priority
}

// StyleResolver merges styles from multiple sources.
type StyleResolver struct {
    mu         sync.RWMutex
    theme      *Theme
    syntax     HighlightProvider
    semantic   HighlightProvider
    diagnostic DiagnosticProvider
}

// NewStyleResolver creates a style resolver.
func NewStyleResolver(theme *Theme) *StyleResolver {
    return &StyleResolver{
        theme: theme,
    }
}

// SetSyntaxProvider sets the syntax highlighting provider.
func (r *StyleResolver) SetSyntaxProvider(p HighlightProvider) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.syntax = p
}

// SetSemanticProvider sets the semantic highlighting provider.
func (r *StyleResolver) SetSemanticProvider(p HighlightProvider) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.semantic = p
}

// ResolveStyles computes final styles for a line.
// Returns style spans in visual column order.
func (r *StyleResolver) ResolveStyles(line uint32, layout *LineLayout) []StyleSpan {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Collect spans from all sources
    var allSpans []sourceSpan

    // Default style for entire line
    allSpans = append(allSpans, sourceSpan{
        span:   StyleSpan{StartCol: 0, EndCol: uint32(layout.Width), Style: r.theme.Default},
        source: SourceDefault,
    })

    // Syntax highlighting
    if r.syntax != nil {
        for _, span := range r.syntax.HighlightsForLine(line) {
            // Convert buffer columns to visual columns
            visStart := layout.VisualColumn(span.StartCol)
            visEnd := layout.VisualColumn(span.EndCol)
            allSpans = append(allSpans, sourceSpan{
                span:   StyleSpan{StartCol: uint32(visStart), EndCol: uint32(visEnd), Style: span.Style},
                source: SourceSyntax,
            })
        }
    }

    // Semantic tokens
    if r.semantic != nil {
        for _, span := range r.semantic.HighlightsForLine(line) {
            visStart := layout.VisualColumn(span.StartCol)
            visEnd := layout.VisualColumn(span.EndCol)
            allSpans = append(allSpans, sourceSpan{
                span:   StyleSpan{StartCol: uint32(visStart), EndCol: uint32(visEnd), Style: span.Style},
                source: SourceSemantic,
            })
        }
    }

    // Merge spans by priority
    return r.mergeSpans(allSpans, uint32(layout.Width))
}

type sourceSpan struct {
    span   StyleSpan
    source Source
}

// mergeSpans merges overlapping spans by priority.
func (r *StyleResolver) mergeSpans(spans []sourceSpan, width uint32) []StyleSpan {
    if len(spans) == 0 {
        return nil
    }

    // Build event list (span starts and ends)
    type event struct {
        col    uint32
        isEnd  bool
        span   sourceSpan
    }

    events := make([]event, 0, len(spans)*2)
    for _, ss := range spans {
        events = append(events,
            event{col: ss.span.StartCol, isEnd: false, span: ss},
            event{col: ss.span.EndCol, isEnd: true, span: ss})
    }

    // Sort by column, starts before ends at same column
    sort.Slice(events, func(i, j int) bool {
        if events[i].col != events[j].col {
            return events[i].col < events[j].col
        }
        return !events[i].isEnd && events[j].isEnd
    })

    // Sweep through events, maintaining active spans
    var result []StyleSpan
    var activeSpans []sourceSpan
    lastCol := uint32(0)

    for _, ev := range events {
        // Emit span for previous region
        if ev.col > lastCol && len(activeSpans) > 0 {
            style := r.computeStyle(activeSpans)
            result = append(result, StyleSpan{
                StartCol: lastCol,
                EndCol:   ev.col,
                Style:    style,
            })
        }

        // Update active spans
        if ev.isEnd {
            // Remove span
            for i, as := range activeSpans {
                if as.span.StartCol == ev.span.span.StartCol &&
                   as.span.EndCol == ev.span.span.EndCol {
                    activeSpans = append(activeSpans[:i], activeSpans[i+1:]...)
                    break
                }
            }
        } else {
            // Add span
            activeSpans = append(activeSpans, ev.span)
        }

        lastCol = ev.col
    }

    return result
}

// computeStyle merges active spans by priority.
func (r *StyleResolver) computeStyle(spans []sourceSpan) Style {
    // Sort by priority (highest first)
    sorted := make([]sourceSpan, len(spans))
    copy(sorted, spans)
    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].source.Priority() > sorted[j].source.Priority()
    })

    // Merge from lowest to highest priority
    result := DefaultStyle()
    for i := len(sorted) - 1; i >= 0; i-- {
        result = result.Merge(sorted[i].span.Style)
    }
    return result
}
```

### 7.2 Theme

```go
// internal/renderer/style/theme.go

// Theme defines colors for syntax highlighting.
type Theme struct {
    Name string

    // Base colors
    Default    Style
    Background Style
    LineNumber Style
    Selection  Style
    Cursor     Style

    // Syntax colors
    Keyword    Style
    String     Style
    Number     Style
    Comment    Style
    Function   Style
    Type       Style
    Variable   Style
    Constant   Style
    Operator   Style
    Punctuation Style

    // Diagnostic colors
    Error      Style
    Warning    Style
    Info       Style
    Hint       Style

    // AI overlay colors
    GhostText  Style
    DiffAdded  Style
    DiffDeleted Style

    // Special
    SearchMatch    Style
    CurrentLine    Style
    MatchingBracket Style
}

// DefaultTheme returns a basic theme.
func DefaultTheme() *Theme {
    return &Theme{
        Name: "Default",

        Default:    Style{Foreground: ColorFromRGB(212, 212, 212)},
        Background: Style{Background: ColorFromRGB(30, 30, 30)},
        LineNumber: Style{Foreground: ColorFromRGB(133, 133, 133)},
        Selection:  Style{Background: ColorFromRGB(68, 68, 68)},
        Cursor:     Style{Background: ColorFromRGB(255, 255, 255)},

        Keyword:    Style{Foreground: ColorFromRGB(197, 134, 192)},
        String:     Style{Foreground: ColorFromRGB(206, 145, 120)},
        Number:     Style{Foreground: ColorFromRGB(181, 206, 168)},
        Comment:    Style{Foreground: ColorFromRGB(106, 153, 85)},
        Function:   Style{Foreground: ColorFromRGB(220, 220, 170)},
        Type:       Style{Foreground: ColorFromRGB(78, 201, 176)},
        Variable:   Style{Foreground: ColorFromRGB(156, 220, 254)},
        Constant:   Style{Foreground: ColorFromRGB(100, 150, 200)},
        Operator:   Style{Foreground: ColorFromRGB(212, 212, 212)},
        Punctuation: Style{Foreground: ColorFromRGB(212, 212, 212)},

        Error:   Style{Foreground: ColorFromRGB(244, 71, 71), Attributes: AttrUnderline},
        Warning: Style{Foreground: ColorFromRGB(230, 180, 80), Attributes: AttrUnderline},
        Info:    Style{Foreground: ColorFromRGB(100, 180, 255)},
        Hint:    Style{Foreground: ColorFromRGB(150, 150, 150)},

        GhostText:  Style{Foreground: ColorFromRGB(128, 128, 128), Attributes: AttrItalic},
        DiffAdded:  Style{Background: ColorFromRGB(35, 80, 35)},
        DiffDeleted: Style{Background: ColorFromRGB(80, 35, 35)},

        SearchMatch:    Style{Background: ColorFromRGB(120, 100, 40)},
        CurrentLine:    Style{Background: ColorFromRGB(40, 40, 40)},
        MatchingBracket: Style{Background: ColorFromRGB(60, 60, 60), Attributes: AttrBold},
    }
}
```

---

## 8. Viewport and Scrolling

### 8.1 Viewport Type

```go
// internal/renderer/viewport/viewport.go

// Viewport represents the visible portion of the buffer.
type Viewport struct {
    // Position in buffer (first visible line)
    TopLine    uint32
    LeftColumn int

    // Size in screen cells
    Width  int
    Height int

    // Computed values
    BottomLine uint32 // Last visible line (computed from TopLine + Height)

    // Scroll margins (keep cursor this far from edges)
    MarginTop    int
    MarginBottom int
    MarginLeft   int
    MarginRight  int

    // Scroll animation state
    targetTopLine uint32
    scrollVel     float64
    animating     bool
}

// NewViewport creates a viewport with the given size.
func NewViewport(width, height int) *Viewport {
    return &Viewport{
        TopLine:      0,
        LeftColumn:   0,
        Width:        width,
        Height:       height,
        MarginTop:    5,
        MarginBottom: 5,
        MarginLeft:   10,
        MarginRight:  10,
    }
}

// Resize updates the viewport size.
func (v *Viewport) Resize(width, height int) {
    v.Width = width
    v.Height = height
    v.updateBottomLine()
}

// VisibleLineRange returns the range of visible buffer lines.
func (v *Viewport) VisibleLineRange() (start, end uint32) {
    return v.TopLine, v.BottomLine
}

// IsLineVisible returns true if the line is within the viewport.
func (v *Viewport) IsLineVisible(line uint32) bool {
    return line >= v.TopLine && line <= v.BottomLine
}

// LineToScreenRow converts a buffer line to a screen row.
// Returns -1 if the line is not visible.
func (v *Viewport) LineToScreenRow(line uint32) int {
    if line < v.TopLine || line > v.BottomLine {
        return -1
    }
    return int(line - v.TopLine)
}

// ScreenRowToLine converts a screen row to a buffer line.
func (v *Viewport) ScreenRowToLine(row int) uint32 {
    return v.TopLine + uint32(row)
}

// ColumnToScreenCol converts a buffer column to a screen column.
func (v *Viewport) ColumnToScreenCol(col int) int {
    return col - v.LeftColumn
}

// ScreenColToColumn converts a screen column to a buffer column.
func (v *Viewport) ScreenColToColumn(col int) int {
    return col + v.LeftColumn
}

// updateBottomLine recomputes the bottom visible line.
func (v *Viewport) updateBottomLine() {
    v.BottomLine = v.TopLine + uint32(v.Height) - 1
}
```

### 8.2 Scrolling

```go
// internal/renderer/viewport/scroll.go

// ScrollTo scrolls to show the given line at the top.
func (v *Viewport) ScrollTo(line uint32, smooth bool) {
    if smooth && v.animating {
        v.targetTopLine = line
        return
    }

    if smooth {
        v.targetTopLine = line
        v.animating = true
        v.scrollVel = 0
    } else {
        v.TopLine = line
        v.animating = false
        v.updateBottomLine()
    }
}

// ScrollBy scrolls by a delta number of lines.
func (v *Viewport) ScrollBy(delta int, smooth bool) {
    newTop := int(v.TopLine) + delta
    if newTop < 0 {
        newTop = 0
    }
    v.ScrollTo(uint32(newTop), smooth)
}

// ScrollToReveal scrolls minimally to reveal a position.
// Uses margins to keep context around the target.
func (v *Viewport) ScrollToReveal(line uint32, col int, smooth bool) {
    needScroll := false
    targetTop := v.TopLine
    targetLeft := v.LeftColumn

    // Vertical scroll
    if line < v.TopLine+uint32(v.MarginTop) {
        // Scroll up
        targetTop = line - uint32(v.MarginTop)
        if int(targetTop) < 0 {
            targetTop = 0
        }
        needScroll = true
    } else if line > v.BottomLine-uint32(v.MarginBottom) {
        // Scroll down
        targetTop = line - uint32(v.Height) + uint32(v.MarginBottom) + 1
        needScroll = true
    }

    // Horizontal scroll
    screenCol := col - v.LeftColumn
    if screenCol < v.MarginLeft {
        targetLeft = col - v.MarginLeft
        if targetLeft < 0 {
            targetLeft = 0
        }
        needScroll = true
    } else if screenCol > v.Width-v.MarginRight {
        targetLeft = col - v.Width + v.MarginRight
        needScroll = true
    }

    if needScroll {
        if smooth {
            v.targetTopLine = targetTop
            v.animating = true
        } else {
            v.TopLine = targetTop
            v.LeftColumn = targetLeft
            v.updateBottomLine()
        }
    }
}

// Update advances scroll animation by dt seconds.
// Returns true if the viewport moved.
func (v *Viewport) Update(dt float64) bool {
    if !v.animating {
        return false
    }

    // Simple easing animation
    diff := float64(int(v.targetTopLine) - int(v.TopLine))
    if math.Abs(diff) < 0.5 {
        v.TopLine = v.targetTopLine
        v.animating = false
        v.updateBottomLine()
        return true
    }

    // Smooth interpolation
    v.scrollVel = diff * 10 // Spring constant
    move := v.scrollVel * dt

    if math.Abs(move) > math.Abs(diff) {
        v.TopLine = v.targetTopLine
        v.animating = false
    } else {
        v.TopLine = uint32(int(v.TopLine) + int(move))
    }

    v.updateBottomLine()
    return true
}
```

---

## 9. Dirty Region Tracking

### 9.1 Dirty Tracker

```go
// internal/renderer/dirty/tracker.go

// DirtyTracker tracks which screen regions need redrawing.
type DirtyTracker struct {
    mu         sync.Mutex
    dirtyLines map[uint32]bool    // Buffer lines that need redraw
    fullRedraw bool               // If true, redraw everything
    regions    []DirtyRegion      // Specific dirty screen regions
}

// DirtyRegion represents a rectangular dirty area.
type DirtyRegion struct {
    StartLine uint32
    EndLine   uint32 // Exclusive
    StartCol  int
    EndCol    int    // Exclusive, -1 = end of line
}

// NewDirtyTracker creates a new dirty tracker.
func NewDirtyTracker() *DirtyTracker {
    return &DirtyTracker{
        dirtyLines: make(map[uint32]bool),
        fullRedraw: true, // Initial draw is full
    }
}

// MarkLine marks a single line as dirty.
func (t *DirtyTracker) MarkLine(line uint32) {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.dirtyLines[line] = true
}

// MarkLines marks a range of lines as dirty.
func (t *DirtyTracker) MarkLines(startLine, endLine uint32) {
    t.mu.Lock()
    defer t.mu.Unlock()
    for line := startLine; line <= endLine; line++ {
        t.dirtyLines[line] = true
    }
}

// MarkAll marks everything as dirty (full redraw).
func (t *DirtyTracker) MarkAll() {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.fullRedraw = true
}

// MarkRegion marks a specific screen region as dirty.
func (t *DirtyTracker) MarkRegion(region DirtyRegion) {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.regions = append(t.regions, region)
}

// IsDirty returns true if any region needs redrawing.
func (t *DirtyTracker) IsDirty() bool {
    t.mu.Lock()
    defer t.mu.Unlock()
    return t.fullRedraw || len(t.dirtyLines) > 0 || len(t.regions) > 0
}

// IsLineDirty returns true if a specific line needs redrawing.
func (t *DirtyTracker) IsLineDirty(line uint32) bool {
    t.mu.Lock()
    defer t.mu.Unlock()
    return t.fullRedraw || t.dirtyLines[line]
}

// NeedsFullRedraw returns true if a full redraw is needed.
func (t *DirtyTracker) NeedsFullRedraw() bool {
    t.mu.Lock()
    defer t.mu.Unlock()
    return t.fullRedraw
}

// GetDirtyLines returns all dirty lines and clears the tracker.
func (t *DirtyTracker) GetDirtyLines() (lines []uint32, fullRedraw bool) {
    t.mu.Lock()
    defer t.mu.Unlock()

    fullRedraw = t.fullRedraw

    if fullRedraw {
        t.fullRedraw = false
        t.dirtyLines = make(map[uint32]bool)
        t.regions = nil
        return nil, true
    }

    lines = make([]uint32, 0, len(t.dirtyLines))
    for line := range t.dirtyLines {
        lines = append(lines, line)
    }
    sort.Slice(lines, func(i, j int) bool { return lines[i] < lines[j] })

    t.dirtyLines = make(map[uint32]bool)
    t.regions = nil

    return lines, false
}

// Clear resets all dirty state.
func (t *DirtyTracker) Clear() {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.dirtyLines = make(map[uint32]bool)
    t.regions = nil
    t.fullRedraw = false
}
```

### 9.2 Change Event Handler

```go
// internal/renderer/dirty/tracker.go (continued)

// HandleBufferChange updates dirty state based on a buffer change.
func (t *DirtyTracker) HandleBufferChange(startLine, oldEndLine, newEndLine uint32) {
    t.mu.Lock()
    defer t.mu.Unlock()

    // Mark affected lines as dirty
    endLine := newEndLine
    if oldEndLine > newEndLine {
        endLine = oldEndLine
    }

    for line := startLine; line <= endLine; line++ {
        t.dirtyLines[line] = true
    }

    // If lines were inserted or deleted, all subsequent lines are dirty
    if oldEndLine != newEndLine {
        // This is expensive for large files - consider tracking line shifts instead
        // For now, mark as full redraw for simplicity
        t.fullRedraw = true
    }
}

// HandleCursorMove updates dirty state for cursor movement.
func (t *DirtyTracker) HandleCursorMove(oldLine, newLine uint32) {
    t.mu.Lock()
    defer t.mu.Unlock()

    // Mark old and new cursor lines as dirty
    t.dirtyLines[oldLine] = true
    t.dirtyLines[newLine] = true
}

// HandleScroll marks the entire viewport as dirty.
func (t *DirtyTracker) HandleScroll() {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.fullRedraw = true
}

// HandleResize marks everything as dirty due to terminal resize.
func (t *DirtyTracker) HandleResize() {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.fullRedraw = true
}
```

---

## 10. Terminal Backend

### 10.1 Backend Interface

```go
// internal/renderer/backend/backend.go

// Backend abstracts the output target (terminal, GUI, etc.).
type Backend interface {
    // Initialization
    Init() error
    Shutdown()

    // Screen size
    Size() (width, height int)
    OnResize(callback func(width, height int))

    // Drawing
    SetCell(x, y int, cell Cell)
    Clear()
    Show() // Flush changes to screen

    // Cursor
    ShowCursor(x, y int)
    HideCursor()
    SetCursorStyle(style CursorStyle)

    // Events
    PollEvent() Event
    PostEvent(event Event)
}

// Event represents an input event.
type Event interface {
    isEvent()
}

// KeyEvent represents a keyboard event.
type KeyEvent struct {
    Key  Key
    Rune rune
    Mod  ModMask
}

func (KeyEvent) isEvent() {}

// MouseEvent represents a mouse event.
type MouseEvent struct {
    X, Y    int
    Button  MouseButton
    Mod     ModMask
}

func (MouseEvent) isEvent() {}

// ResizeEvent represents a terminal resize.
type ResizeEvent struct {
    Width, Height int
}

func (ResizeEvent) isEvent() {}
```

### 10.2 Screen Buffer

```go
// internal/renderer/backend/buffer.go

// ScreenBuffer is a double-buffered cell grid.
type ScreenBuffer struct {
    width   int
    height  int
    front   [][]Cell // Currently displayed
    back    [][]Cell // Being built
}

// NewScreenBuffer creates a new screen buffer.
func NewScreenBuffer(width, height int) *ScreenBuffer {
    sb := &ScreenBuffer{
        width:  width,
        height: height,
    }
    sb.front = sb.allocGrid(width, height)
    sb.back = sb.allocGrid(width, height)
    return sb
}

func (sb *ScreenBuffer) allocGrid(width, height int) [][]Cell {
    grid := make([][]Cell, height)
    for i := range grid {
        grid[i] = make([]Cell, width)
        for j := range grid[i] {
            grid[i][j] = EmptyCell()
        }
    }
    return grid
}

// Resize resizes the buffer.
func (sb *ScreenBuffer) Resize(width, height int) {
    sb.width = width
    sb.height = height
    sb.front = sb.allocGrid(width, height)
    sb.back = sb.allocGrid(width, height)
}

// SetCell sets a cell in the back buffer.
func (sb *ScreenBuffer) SetCell(x, y int, cell Cell) {
    if x >= 0 && x < sb.width && y >= 0 && y < sb.height {
        sb.back[y][x] = cell
    }
}

// GetCell returns a cell from the back buffer.
func (sb *ScreenBuffer) GetCell(x, y int) Cell {
    if x >= 0 && x < sb.width && y >= 0 && y < sb.height {
        return sb.back[y][x]
    }
    return EmptyCell()
}

// Clear fills the back buffer with empty cells.
func (sb *ScreenBuffer) Clear() {
    for y := range sb.back {
        for x := range sb.back[y] {
            sb.back[y][x] = EmptyCell()
        }
    }
}

// ClearRow clears a single row.
func (sb *ScreenBuffer) ClearRow(y int) {
    if y >= 0 && y < sb.height {
        for x := range sb.back[y] {
            sb.back[y][x] = EmptyCell()
        }
    }
}

// Diff computes the difference between front and back buffers.
// Returns cells that need to be redrawn.
func (sb *ScreenBuffer) Diff() []CellUpdate {
    var updates []CellUpdate

    for y := 0; y < sb.height; y++ {
        for x := 0; x < sb.width; x++ {
            front := sb.front[y][x]
            back := sb.back[y][x]

            if !cellsEqual(front, back) {
                updates = append(updates, CellUpdate{
                    X:    x,
                    Y:    y,
                    Cell: back,
                })
            }
        }
    }

    return updates
}

// Swap swaps front and back buffers.
func (sb *ScreenBuffer) Swap() {
    sb.front, sb.back = sb.back, sb.front
}

// CellUpdate represents a single cell change.
type CellUpdate struct {
    X, Y int
    Cell Cell
}

func cellsEqual(a, b Cell) bool {
    return a.Rune == b.Rune &&
           a.Width == b.Width &&
           a.Style.Foreground == b.Style.Foreground &&
           a.Style.Background == b.Style.Background &&
           a.Style.Attributes == b.Style.Attributes
}
```

### 10.3 Terminal Implementation

```go
// internal/renderer/backend/terminal.go

// TerminalBackend implements Backend using tcell.
type TerminalBackend struct {
    screen      tcell.Screen
    resizeFunc  func(width, height int)
    eventChan   chan Event
}

// NewTerminalBackend creates a new terminal backend.
func NewTerminalBackend() (*TerminalBackend, error) {
    screen, err := tcell.NewScreen()
    if err != nil {
        return nil, err
    }

    return &TerminalBackend{
        screen:    screen,
        eventChan: make(chan Event, 100),
    }, nil
}

// Init initializes the terminal.
func (t *TerminalBackend) Init() error {
    if err := t.screen.Init(); err != nil {
        return err
    }
    t.screen.EnableMouse()
    t.screen.Clear()

    // Start event polling goroutine
    go t.pollEvents()

    return nil
}

// Shutdown releases terminal resources.
func (t *TerminalBackend) Shutdown() {
    t.screen.Fini()
}

// Size returns the terminal dimensions.
func (t *TerminalBackend) Size() (width, height int) {
    return t.screen.Size()
}

// OnResize sets the resize callback.
func (t *TerminalBackend) OnResize(callback func(width, height int)) {
    t.resizeFunc = callback
}

// SetCell sets a cell on screen.
func (t *TerminalBackend) SetCell(x, y int, cell Cell) {
    style := t.convertStyle(cell.Style)
    t.screen.SetContent(x, y, cell.Rune, nil, style)
}

// Clear clears the screen.
func (t *TerminalBackend) Clear() {
    t.screen.Clear()
}

// Show flushes changes to the terminal.
func (t *TerminalBackend) Show() {
    t.screen.Show()
}

// ShowCursor shows the cursor at position.
func (t *TerminalBackend) ShowCursor(x, y int) {
    t.screen.ShowCursor(x, y)
}

// HideCursor hides the cursor.
func (t *TerminalBackend) HideCursor() {
    t.screen.HideCursor()
}

// SetCursorStyle sets the cursor appearance.
func (t *TerminalBackend) SetCursorStyle(style CursorStyle) {
    // tcell cursor style mapping
    var tcellStyle tcell.CursorStyle
    switch style {
    case CursorBlock:
        tcellStyle = tcell.CursorStyleDefault
    case CursorBar:
        tcellStyle = tcell.CursorStyleSteadyBar
    case CursorUnderline:
        tcellStyle = tcell.CursorStyleSteadyUnderline
    }
    t.screen.SetCursorStyle(tcellStyle)
}

// PollEvent returns the next event.
func (t *TerminalBackend) PollEvent() Event {
    return <-t.eventChan
}

// PostEvent posts an event to the queue.
func (t *TerminalBackend) PostEvent(event Event) {
    select {
    case t.eventChan <- event:
    default:
        // Drop if queue is full
    }
}

// pollEvents polls tcell events and converts them.
func (t *TerminalBackend) pollEvents() {
    for {
        ev := t.screen.PollEvent()
        if ev == nil {
            return
        }

        switch e := ev.(type) {
        case *tcell.EventKey:
            t.eventChan <- t.convertKeyEvent(e)
        case *tcell.EventMouse:
            t.eventChan <- t.convertMouseEvent(e)
        case *tcell.EventResize:
            w, h := e.Size()
            if t.resizeFunc != nil {
                t.resizeFunc(w, h)
            }
            t.eventChan <- ResizeEvent{Width: w, Height: h}
        }
    }
}

// convertStyle converts our Style to tcell.Style.
func (t *TerminalBackend) convertStyle(s Style) tcell.Style {
    style := tcell.StyleDefault

    // Foreground
    if s.Foreground.Indexed {
        style = style.Foreground(tcell.PaletteColor(int(s.Foreground.R)))
    } else {
        style = style.Foreground(tcell.NewRGBColor(
            int32(s.Foreground.R),
            int32(s.Foreground.G),
            int32(s.Foreground.B)))
    }

    // Background
    if s.Background.Indexed {
        style = style.Background(tcell.PaletteColor(int(s.Background.R)))
    } else {
        style = style.Background(tcell.NewRGBColor(
            int32(s.Background.R),
            int32(s.Background.G),
            int32(s.Background.B)))
    }

    // Attributes
    if s.Attributes&AttrBold != 0 {
        style = style.Bold(true)
    }
    if s.Attributes&AttrItalic != 0 {
        style = style.Italic(true)
    }
    if s.Attributes&AttrUnderline != 0 {
        style = style.Underline(true)
    }
    if s.Attributes&AttrReverse != 0 {
        style = style.Reverse(true)
    }
    if s.Attributes&AttrDim != 0 {
        style = style.Dim(true)
    }
    if s.Attributes&AttrBlink != 0 {
        style = style.Blink(true)
    }
    if s.Attributes&AttrStrikethrough != 0 {
        style = style.StrikeThrough(true)
    }

    return style
}

// convertKeyEvent converts tcell key event to our format.
func (t *TerminalBackend) convertKeyEvent(e *tcell.EventKey) KeyEvent {
    // Key conversion implementation
    return KeyEvent{
        Key:  convertKey(e.Key()),
        Rune: e.Rune(),
        Mod:  convertMod(e.Modifiers()),
    }
}

// convertMouseEvent converts tcell mouse event to our format.
func (t *TerminalBackend) convertMouseEvent(e *tcell.EventMouse) MouseEvent {
    x, y := e.Position()
    return MouseEvent{
        X:      x,
        Y:      y,
        Button: convertButton(e.Buttons()),
        Mod:    convertMod(e.Modifiers()),
    }
}
```

---

## 11. Cursor and Selection Rendering

### 11.1 Cursor Renderer

```go
// internal/renderer/cursor/cursor.go

// CursorRenderer handles cursor display.
type CursorRenderer struct {
    style       CursorStyle
    blinkState  bool
    blinkTimer  *time.Timer
    blinkRate   time.Duration
    visible     bool
}

// NewCursorRenderer creates a cursor renderer.
func NewCursorRenderer(style CursorStyle, blinkRate time.Duration) *CursorRenderer {
    return &CursorRenderer{
        style:     style,
        blinkRate: blinkRate,
        visible:   true,
    }
}

// SetStyle changes the cursor style.
func (r *CursorRenderer) SetStyle(style CursorStyle) {
    r.style = style
}

// StartBlink starts the cursor blink animation.
func (r *CursorRenderer) StartBlink() {
    r.blinkState = true
    if r.blinkTimer != nil {
        r.blinkTimer.Stop()
    }
    r.blinkTimer = time.AfterFunc(r.blinkRate, r.toggleBlink)
}

// StopBlink stops the cursor blink and shows it.
func (r *CursorRenderer) StopBlink() {
    if r.blinkTimer != nil {
        r.blinkTimer.Stop()
        r.blinkTimer = nil
    }
    r.blinkState = true
}

// ResetBlink resets the blink timer (called on keypress).
func (r *CursorRenderer) ResetBlink() {
    r.blinkState = true
    if r.blinkTimer != nil {
        r.blinkTimer.Reset(r.blinkRate)
    }
}

func (r *CursorRenderer) toggleBlink() {
    r.blinkState = !r.blinkState
    r.blinkTimer = time.AfterFunc(r.blinkRate, r.toggleBlink)
}

// IsVisible returns true if cursor should be drawn this frame.
func (r *CursorRenderer) IsVisible() bool {
    return r.visible && r.blinkState
}

// RenderCursor renders the cursor at the given position.
func (r *CursorRenderer) RenderCursor(
    buf *ScreenBuffer,
    screenX, screenY int,
    charUnder Cell,
    theme *Theme,
) {
    if !r.IsVisible() {
        return
    }

    switch r.style {
    case CursorBlock:
        // Invert colors
        cell := charUnder
        cell.Style.Foreground, cell.Style.Background =
            cell.Style.Background, cell.Style.Foreground
        buf.SetCell(screenX, screenY, cell)

    case CursorBar:
        // Draw a vertical bar (left edge of cell)
        // Most terminals don't support partial cell rendering,
        // so we use a special character or attribute
        cell := charUnder
        cell.Style.Attributes |= AttrReverse
        buf.SetCell(screenX, screenY, cell)

    case CursorUnderline:
        // Underline the cell
        cell := charUnder
        cell.Style.Attributes |= AttrUnderline
        buf.SetCell(screenX, screenY, cell)
    }
}
```

### 11.2 Selection Renderer

```go
// internal/renderer/cursor/selection.go

// SelectionRenderer handles selection highlighting.
type SelectionRenderer struct {
    primaryStyle   Style
    secondaryStyle Style
}

// NewSelectionRenderer creates a selection renderer.
func NewSelectionRenderer(theme *Theme) *SelectionRenderer {
    return &SelectionRenderer{
        primaryStyle:   theme.Selection,
        secondaryStyle: theme.Selection.Merge(Style{Attributes: AttrDim}),
    }
}

// ApplySelection applies selection styling to cells in a row.
func (r *SelectionRenderer) ApplySelection(
    cells []Cell,
    line uint32,
    selections []Selection,
    layout *LineLayout,
) []Cell {
    result := make([]Cell, len(cells))
    copy(result, cells)

    for _, sel := range selections {
        // Check if selection intersects this line
        if line < sel.StartLine || line > sel.EndLine {
            continue
        }

        // Calculate column range for this line
        var startCol, endCol uint32
        if line == sel.StartLine {
            startCol = sel.StartCol
        } else {
            startCol = 0
        }
        if line == sel.EndLine {
            endCol = sel.EndCol
        } else {
            endCol = uint32(len(cells)) // End of line
        }

        // Convert buffer columns to visual columns
        visStart := layout.VisualColumn(startCol)
        visEnd := layout.VisualColumn(endCol)

        // Apply selection style
        style := r.primaryStyle
        if !sel.IsPrimary {
            style = r.secondaryStyle
        }

        for i := visStart; i < visEnd && i < len(result); i++ {
            result[i].Style = result[i].Style.Merge(style)
        }
    }

    return result
}
```

---

## 12. AI Overlay Rendering

### 12.1 Ghost Text Renderer

```go
// internal/renderer/overlay/ghost.go

// GhostTextRenderer renders inline completion previews.
type GhostTextRenderer struct {
    style Style
}

// NewGhostTextRenderer creates a ghost text renderer.
func NewGhostTextRenderer(theme *Theme) *GhostTextRenderer {
    return &GhostTextRenderer{
        style: theme.GhostText,
    }
}

// RenderGhostText inserts ghost text into a line's cells.
func (r *GhostTextRenderer) RenderGhostText(
    cells []Cell,
    ghostText string,
    insertCol int,
    layout *LineLayout,
) []Cell {
    if len(ghostText) == 0 {
        return cells
    }

    // Convert insert column to visual position
    visCol := layout.VisualColumn(uint32(insertCol))

    // Build ghost cells
    ghostCells := make([]Cell, 0, len(ghostText))
    for _, r := range ghostText {
        if r == '\n' {
            break // Only show first line of ghost text
        }
        width := runeWidth(r)
        ghostCells = append(ghostCells, Cell{
            Rune:  r,
            Width: width,
            Style: r.style,
        })
    }

    // Insert ghost cells at position
    result := make([]Cell, 0, len(cells)+len(ghostCells))
    result = append(result, cells[:visCol]...)
    result = append(result, ghostCells...)
    result = append(result, cells[visCol:]...)

    return result
}
```

### 12.2 Diff Preview Renderer

```go
// internal/renderer/overlay/diff.go

// DiffRenderer renders inline diff previews.
type DiffRenderer struct {
    addedStyle   Style
    deletedStyle Style
}

// NewDiffRenderer creates a diff renderer.
func NewDiffRenderer(theme *Theme) *DiffRenderer {
    return &DiffRenderer{
        addedStyle:   theme.DiffAdded,
        deletedStyle: theme.DiffDeleted,
    }
}

// DiffLineInfo contains diff information for a line.
type DiffLineInfo struct {
    Type        DiffLineType
    OriginalLine string // For deleted lines
    NewLine      string // For added lines
}

// DiffLineType indicates how a line is affected.
type DiffLineType uint8

const (
    DiffLineUnchanged DiffLineType = iota
    DiffLineDeleted
    DiffLineAdded
    DiffLineModified
)

// RenderDiffLine renders a line with diff styling.
func (r *DiffRenderer) RenderDiffLine(
    cells []Cell,
    info DiffLineInfo,
) []Cell {
    result := make([]Cell, len(cells))
    copy(result, cells)

    switch info.Type {
    case DiffLineAdded:
        for i := range result {
            result[i].Style = result[i].Style.Merge(r.addedStyle)
        }

    case DiffLineDeleted:
        for i := range result {
            result[i].Style = result[i].Style.Merge(r.deletedStyle)
        }

    case DiffLineModified:
        // Could show inline diff here
        // For now, just highlight the whole line
        for i := range result {
            result[i].Style = result[i].Style.Merge(r.addedStyle)
        }
    }

    return result
}

// RenderDeletedLines renders deleted lines that should be shown as overlay.
func (r *DiffRenderer) RenderDeletedLines(
    buf *ScreenBuffer,
    startRow int,
    deletedLines []string,
    width int,
    layoutEngine *LayoutEngine,
) int {
    rowsUsed := 0
    for _, line := range deletedLines {
        layout := layoutEngine.Layout(line, 0)

        // Build cells with deleted style
        cells := make([]Cell, width)
        for i := range cells {
            cells[i] = EmptyCell()
            cells[i].Style = r.deletedStyle
        }

        for i, cell := range layout.Cells {
            if i < width {
                cells[i] = cell
                cells[i].Style = cells[i].Style.Merge(r.deletedStyle)
            }
        }

        // Add "-" sign in gutter
        cells[0] = Cell{Rune: '-', Width: 1, Style: r.deletedStyle}

        // Write to buffer
        row := startRow + rowsUsed
        for x, cell := range cells {
            buf.SetCell(x, row, cell)
        }
        rowsUsed++
    }
    return rowsUsed
}
```

---

## 13. Implementation Phases

### Phase 1: Core Types and Backend

**Goal**: Establish foundational types and terminal abstraction.

**Tasks**:
1. `types.go` - Color, Style, Cell, ScreenPos, ScreenRect
2. `backend/backend.go` - Backend interface
3. `backend/terminal.go` - Terminal implementation with tcell
4. `backend/buffer.go` - Double-buffered screen buffer
5. Basic tests

**Success Criteria**:
- Can initialize terminal and draw colored text
- Resize handling works
- Key/mouse events are received

### Phase 2: Line Layout

**Goal**: Implement line layout engine with tab expansion and Unicode support.

**Tasks**:
1. `layout/line.go` - LineLayout type
2. `layout/tab.go` - Tab expansion
3. `layout/unicode.go` - Unicode width calculation
4. `layout/cache.go` - Line cache with LRU eviction
5. Comprehensive tests for edge cases

**Success Criteria**:
- Tabs expand correctly to configurable stops
- Wide CJK characters handled properly
- Cache hit rate > 90% for typical editing

### Phase 3: Viewport and Scrolling

**Goal**: Implement viewport management and smooth scrolling.

**Tasks**:
1. `viewport/viewport.go` - Viewport type
2. `viewport/scroll.go` - Scroll state and animation
3. `viewport/margins.go` - Scroll margins
4. Integration with line layout

**Success Criteria**:
- Can scroll to reveal cursor
- Smooth scroll animation (when enabled)
- Scroll margins keep cursor in context

### Phase 4: Basic Rendering

**Goal**: Render buffer content without highlighting.

**Tasks**:
1. `renderer.go` - Main Renderer type
2. `view.go` - View type for single editor view
3. `gutter/gutter.go` - Gutter renderer
4. `gutter/linenums.go` - Line numbers
5. Render loop with frame limiting

**Success Criteria**:
- Can display buffer content
- Line numbers show correctly
- 60 FPS for scrolling in 10k line files

### Phase 5: Dirty Region Tracking

**Goal**: Implement efficient incremental updates.

**Tasks**:
1. `dirty/tracker.go` - DirtyTracker type
2. `dirty/region.go` - DirtyRegion representation
3. Integration with buffer change events
4. Screen buffer diff for minimal terminal writes

**Success Criteria**:
- Single character edit only redraws one line
- Cursor movement only redraws cursor lines
- Full redraw only on scroll/resize

### Phase 6: Cursor and Selection

**Goal**: Render cursors and selections.

**Tasks**:
1. `cursor/cursor.go` - Cursor rendering
2. `cursor/selection.go` - Selection highlighting
3. `cursor/blink.go` - Cursor blink animation
4. Multi-cursor support

**Success Criteria**:
- Block/bar/underline cursor styles work
- Cursor blinks at configurable rate
- Multiple selections render correctly

### Phase 7: Style Resolution

**Goal**: Implement syntax highlighting integration.

**Tasks**:
1. `style/style.go` - Style type
2. `style/span.go` - StyleSpan for ranges
3. `style/resolver.go` - Merge multiple sources
4. `style/theme.go` - Theme definition
5. Integration hooks for future Tree-sitter/LSP

**Success Criteria**:
- Styles apply correctly to text
- Multiple overlapping styles merge properly
- Theme switching works

### Phase 8: AI Overlays

**Goal**: Render AI integration overlays.

**Tasks**:
1. `overlay/ghost.go` - Ghost text completion preview
2. `overlay/diff.go` - Diff preview rendering
3. `overlay/progress.go` - Progress indicators
4. Integration with overlay provider interface

**Success Criteria**:
- Ghost text appears at cursor in distinct style
- Diff additions/deletions are color-coded
- Progress indicator shows during AI operations

### Phase 9: Optimization and Polish

**Goal**: Optimize performance and handle edge cases.

**Tasks**:
1. Profile and optimize hot paths
2. Add word wrap support
3. Implement minimap (optional)
4. Fuzz testing for robustness
5. Documentation and examples

**Success Criteria**:
- 60 FPS for 100k line files
- No visual glitches in edge cases
- Memory usage scales linearly with viewport size (not file size)

---

## 14. Testing Strategy

### 14.1 Unit Tests

```go
// Example test for line layout
func TestLineLayoutTabExpansion(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        tabWidth int
        expected int // Expected visual width
    }{
        {"single tab at start", "\thello", 4, 9},
        {"tab in middle", "ab\tcd", 4, 6},
        {"multiple tabs", "\t\t", 4, 8},
        {"tab at tab stop", "1234\t", 4, 8},
        {"tab near tab stop", "123\t", 4, 4},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            engine := NewLayoutEngine(tt.tabWidth)
            layout := engine.Layout(tt.input, 0)
            if layout.Width != tt.expected {
                t.Errorf("got width %d, want %d", layout.Width, tt.expected)
            }
        })
    }
}

// Example test for style merging
func TestStyleMerge(t *testing.T) {
    base := Style{
        Foreground: ColorFromRGB(255, 0, 0),
        Background: ColorDefault,
        Attributes: AttrBold,
    }
    overlay := Style{
        Foreground: ColorDefault,
        Background: ColorFromRGB(0, 0, 255),
        Attributes: AttrItalic,
    }

    merged := base.Merge(overlay)

    // Foreground should stay from base (overlay is default)
    if merged.Foreground != base.Foreground {
        t.Error("foreground should be preserved from base")
    }
    // Background should come from overlay
    if merged.Background != overlay.Background {
        t.Error("background should come from overlay")
    }
    // Attributes should be combined
    if merged.Attributes != (AttrBold | AttrItalic) {
        t.Error("attributes should be combined")
    }
}
```

### 14.2 Integration Tests

```go
// Test rendering a simple buffer
func TestRenderSimpleBuffer(t *testing.T) {
    // Create mock buffer
    buf := &mockBuffer{
        lines: []string{
            "Hello, World!",
            "Line 2",
            "",
            "Line 4",
        },
    }

    // Create renderer with test backend
    backend := newTestBackend(80, 24)
    renderer := NewRenderer(backend, DefaultOptions())
    renderer.SetBuffer(buf)

    // Render
    renderer.Render()

    // Verify output
    row0 := backend.GetRow(0)
    if !strings.HasPrefix(row0, "Hello, World!") {
        t.Errorf("unexpected first row: %q", row0)
    }
}
```

### 14.3 Benchmark Tests

```go
func BenchmarkRenderFullScreen(b *testing.B) {
    // Create large buffer
    lines := make([]string, 10000)
    for i := range lines {
        lines[i] = strings.Repeat("x", 80)
    }
    buf := &mockBuffer{lines: lines}

    backend := newTestBackend(80, 24)
    renderer := NewRenderer(backend, DefaultOptions())
    renderer.SetBuffer(buf)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        renderer.Render()
    }
}

func BenchmarkRenderIncrementalUpdate(b *testing.B) {
    // Setup as above...
    renderer.Render() // Initial render

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Simulate cursor move
        renderer.OnCursorMove(100, 101)
        renderer.Render()
    }
}
```

---

## 15. Performance Considerations

### 15.1 Memory Efficiency

**Line Cache Strategy**:
- Cache only visible lines plus small buffer (viewport + 50 lines)
- LRU eviction when cache exceeds limit
- Invalidate on content change, not on scroll

**Cell Allocation**:
- Reuse cell slices where possible
- Use sync.Pool for frequently allocated objects
- Avoid string concatenation in hot paths

### 15.2 CPU Efficiency

**Hot Path Optimizations**:
- Pre-compute visual column mappings
- Use integer math for screen coordinates
- Batch terminal writes (don't call SetCell per character)

**Lazy Computation**:
- Only compute layout for visible lines
- Defer syntax highlighting for off-screen lines
- Skip style resolution for unchanged lines

### 15.3 Rendering Pipeline

**Double Buffering**:
- Build frame in back buffer
- Diff against front buffer
- Only send changed cells to terminal

**Frame Limiting**:
- Cap at 60 FPS (16.6ms per frame)
- Skip frames if previous render still in progress
- Coalesce rapid events (multiple edits in one frame)

### 15.4 Large File Handling

**Virtualized Rendering**:
- Only process visible lines
- Use rope's efficient line access
- Don't build full line layouts for entire file

**Incremental Highlighting**:
- Highlight visible region first
- Expand highlighting in background
- Cancel and restart on scroll

---

## Dependencies

External packages required:

| Package | Purpose |
|---------|---------|
| `github.com/gdamore/tcell/v2` | Terminal abstraction |
| `github.com/rivo/uniseg` | Unicode grapheme cluster handling |
| `github.com/mattn/go-runewidth` | East Asian Width calculation |

Future dependencies (for syntax highlighting):

| Package | Purpose |
|---------|---------|
| `github.com/smacker/go-tree-sitter` | Tree-sitter bindings |
| Grammar packages | Language-specific parsers |

---

## References

- [tcell documentation](https://github.com/gdamore/tcell)
- [Unicode Standard Annex #11: East Asian Width](https://unicode.org/reports/tr11/)
- [Tree-sitter](https://tree-sitter.github.io/tree-sitter/)
- [Zed Rendering Architecture](https://zed.dev/blog/zed-decoded)
- [VS Code TextMate Tokenization](https://code.visualstudio.com/api/language-extensions/syntax-highlight-guide)
