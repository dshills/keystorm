# Keystorm Core Text Editor Engine - Implementation Plan

## Comprehensive Design Document for `internal/engine`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Research Findings](#2-research-findings)
3. [Package Structure](#3-package-structure)
4. [Core Types and Interfaces](#4-core-types-and-interfaces)
5. [Rope Implementation Details](#5-rope-implementation-details)
6. [Buffer API Design](#6-buffer-api-design)
7. [Cursor and Selection Management](#7-cursor-and-selection-management)
8. [Undo/Redo System Design](#8-undoredo-system-design)
9. [Change Tracking and Snapshotting](#9-change-tracking-and-snapshotting)
10. [Implementation Phases](#10-implementation-phases)
11. [Key Algorithms](#11-key-algorithms)
12. [Testing Strategy](#12-testing-strategy)
13. [Performance Considerations](#13-performance-considerations)

---

## 1. Executive Summary

This document outlines the design and implementation plan for Keystorm's core text editor engine. The engine will use a **B+ tree rope** (similar to Zed's SumTree) as the primary data structure, providing:

- O(log n) insertion, deletion, and access operations
- Efficient line/column indexing via aggregated metrics
- Copy-on-write semantics for cheap snapshots
- Thread-safe concurrent access for AI integration
- Built-in support for "what changed since X?" queries

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| B+ tree rope over binary tree rope | Better cache locality, fewer pointer indirections, superior worst-case performance |
| Persistent/immutable data structure | Enables cheap snapshots, thread-safe concurrent access, trivial undo |
| Metrics-based tree (SumTree pattern) | O(log n) line/column lookups without separate index structure |
| Chunk-based leaves (128-512 bytes) | Balance between tree depth and per-node overhead |
| Command pattern for undo/redo | Clean separation, composable operations, easy AI integration |

---

## 2. Research Findings

### 2.1 Rope Data Structure Fundamentals

Based on research from Wikipedia, Zed's blog, and xi-editor documentation:

**Classic Binary Tree Rope:**
- Each leaf holds a string segment
- Internal nodes store weight (left subtree character count)
- O(log n) for insert, delete, index operations
- O(1) concatenation (create new root)

**B+ Tree Rope (Recommended for Keystorm):**
- Higher branching factor (8-32 children per node)
- All data in leaves, internal nodes store metadata/summaries
- Better cache locality due to fewer levels
- Uniform node sizes simplify memory allocation

**Key Insight from Zed's SumTree:**
> "The SumTree is a thread-safe, snapshot-friendly, copy-on-write B+ tree where each leaf node contains multiple items and a Summary for each Item, and internal nodes contain a Summary of the items in its subtree."

### 2.2 Line Indexing Strategies

From xi-editor's metrics documentation:

**Monoid-based Metrics:**
- Define metrics as monoid homomorphisms (associative operations with identity)
- Store aggregated metrics in internal nodes
- Examples: byte count, UTF-16 code units, line count, longest line

**Multi-dimensional Seeking:**
- Seek on one dimension (e.g., byte offset) while computing others (e.g., line/column)
- Enables O(log n) conversion between coordinate systems

### 2.3 Undo/Redo Models

**Command Pattern (Recommended):**
- Encapsulate each edit as a command object
- Commands know how to execute and reverse themselves
- Maintain undo/redo stacks
- Supports compound commands for complex operations

**Persistent Data Structure Approach:**
- With immutable ropes, "undo" is just keeping a reference to the previous version
- Structural sharing minimizes memory overhead
- Trivially thread-safe

**Hybrid Approach (Recommended for Keystorm):**
- Use persistent rope for efficient snapshots
- Track edits as commands for fine-grained undo/redo
- Commands reference rope versions for "before/after" state

### 2.4 Multi-Cursor Implementation

**Why Ropes Excel:**
- Gap buffers degrade with non-local edits (common with multi-cursor)
- Ropes maintain O(log n) for any edit pattern
- Structural sharing means multi-cursor edits don't multiply memory usage

**Cursor Representation:**
- Store cursors as byte offsets (most stable representation)
- Update cursor positions after edits using transformation functions
- Support selection as cursor pair (anchor, head)

### 2.5 Change Tracking for AI Context

**Revision-based Tracking:**
- Each edit creates a new revision ID
- Store delta between revisions
- Enable "what changed between revision X and Y?" queries

**Snapshot-based Approach:**
- Cheap snapshots via copy-on-write rope
- Diff snapshots to generate change sets
- Label snapshots with semantic names ("before AI suggestion", "checkpoint 1")

---

## 3. Package Structure

```
internal/engine/
    doc.go                  # Package documentation

    # Core rope implementation
    rope/
        rope.go             # Main Rope type and operations
        node.go             # Internal and leaf node types
        chunk.go            # Leaf chunk type (bounded string)
        metrics.go          # Metric types (TextSummary, Point, etc.)
        cursor.go           # Tree cursor for traversal
        iter.go             # Iterator implementations (chars, lines, chunks)
        builder.go          # Efficient rope construction
        rope_test.go
        bench_test.go

    # Buffer abstraction over rope
    buffer/
        buffer.go           # Buffer type (rope + metadata)
        position.go         # Position types (byte offset, line/col, UTF-16)
        range.go            # Range types for selections
        edit.go             # Edit operations (insert, delete, replace)
        buffer_test.go

    # Cursor and selection management
    cursor/
        cursor.go           # Single cursor type
        selection.go        # Selection type (anchor + head)
        cursors.go          # Multi-cursor collection
        transform.go        # Cursor transformation after edits
        cursor_test.go

    # Undo/redo system
    history/
        operation.go        # Edit operation types
        command.go          # Command interface and implementations
        stack.go            # Undo/redo stack
        group.go            # Command grouping for compound edits
        history_test.go

    # Change tracking for AI
    tracking/
        revision.go         # Revision ID and metadata
        snapshot.go         # Named snapshots
        delta.go            # Change delta representation
        diff.go             # Diff computation between revisions
        tracking_test.go

    # Public API
    engine.go               # Main Engine type (facade)
    options.go              # Configuration options
    errors.go               # Error types
    engine_test.go
```

### Rationale

- **Separation of concerns**: Each sub-package has a single responsibility
- **Internal packages**: `rope/`, `buffer/`, etc. are implementation details
- **Facade pattern**: `Engine` type provides unified API
- **Testability**: Each package can be tested in isolation

---

## 4. Core Types and Interfaces

### 4.1 Position and Range Types

```go
// internal/engine/buffer/position.go

// ByteOffset represents an absolute byte position in the buffer.
// This is the canonical internal representation.
type ByteOffset uint64

// Point represents a line/column position.
// Line and Column are both 0-indexed.
type Point struct {
    Line   uint32
    Column uint32
}

// PointUTF16 represents a position in UTF-16 code units.
// Required for LSP compatibility.
type PointUTF16 struct {
    Line      uint32
    ColumnU16 uint32
}

// Range represents a contiguous region of text.
type Range struct {
    Start ByteOffset
    End   ByteOffset
}

// PointRange represents a range using line/column positions.
type PointRange struct {
    Start Point
    End   Point
}
```

### 4.2 Core Interfaces

```go
// internal/engine/engine.go

// Reader provides read-only access to buffer contents.
type Reader interface {
    // Content access
    Text() string                           // Full text (use sparingly)
    TextRange(start, end ByteOffset) string // Text in range
    RuneAt(offset ByteOffset) (rune, error) // Single rune
    LineText(line uint32) string            // Single line text

    // Metrics
    Len() ByteOffset                        // Total byte length
    LineCount() uint32                      // Number of lines

    // Position conversion
    OffsetToPoint(offset ByteOffset) Point
    PointToOffset(point Point) ByteOffset
    OffsetToPointUTF16(offset ByteOffset) PointUTF16
    PointUTF16ToOffset(point PointUTF16) ByteOffset

    // Line operations
    LineStartOffset(line uint32) ByteOffset
    LineEndOffset(line uint32) ByteOffset   // Excludes newline
    LineLen(line uint32) uint32             // Byte length

    // Iteration
    Lines() LineIterator
    Chunks() ChunkIterator
    Runes() RuneIterator
}

// Writer provides write operations on the buffer.
type Writer interface {
    // Basic edits (return new cursor positions)
    Insert(offset ByteOffset, text string) (ByteOffset, error)
    Delete(start, end ByteOffset) error
    Replace(start, end ByteOffset, text string) (ByteOffset, error)

    // Batch edits (for multi-cursor)
    ApplyEdits(edits []Edit) error
}

// Buffer combines read and write capabilities.
type Buffer interface {
    Reader
    Writer

    // Snapshot for concurrent access
    Snapshot() Reader

    // Revision tracking
    RevisionID() RevisionID
}

// UndoManager handles undo/redo operations.
type UndoManager interface {
    Undo() error
    Redo() error
    CanUndo() bool
    CanRedo() bool

    // Grouping for compound edits
    BeginGroup(name string)
    EndGroup()

    // History inspection
    UndoStack() []OperationInfo
    RedoStack() []OperationInfo
}

// ChangeTracker tracks changes for AI context.
type ChangeTracker interface {
    // Snapshots
    CreateSnapshot(name string) SnapshotID
    GetSnapshot(id SnapshotID) (Reader, error)
    DeleteSnapshot(id SnapshotID)

    // Diffing
    DiffSince(id SnapshotID) ([]Change, error)
    DiffBetween(from, to SnapshotID) ([]Change, error)

    // Revision queries
    ChangesSince(rev RevisionID) ([]Change, error)
}

// Engine is the main facade combining all capabilities.
type Engine interface {
    Buffer
    UndoManager
    ChangeTracker

    // Cursor management
    Cursors() CursorManager

    // Configuration
    SetTabWidth(width int)
    SetLineEnding(ending LineEnding)
}
```

### 4.3 Edit and Change Types

```go
// internal/engine/buffer/edit.go

// Edit represents a single edit operation.
type Edit struct {
    Range   Range  // Range to replace (empty for pure insert)
    NewText string // Replacement text (empty for pure delete)
}

// EditResult contains information about a completed edit.
type EditResult struct {
    OldRange    Range      // Original range
    NewRange    Range      // Range of inserted text
    OldText     string     // Deleted text (for undo)
    RevisionID  RevisionID // New revision after edit
}
```

```go
// internal/engine/tracking/delta.go

// Change represents a single change for AI context.
type Change struct {
    Type     ChangeType // Insert, Delete, Replace
    Range    Range      // Position in old text
    NewRange Range      // Position in new text
    OldText  string     // What was there before
    NewText  string     // What is there now
}

// ChangeType categorizes changes.
type ChangeType uint8

const (
    ChangeInsert ChangeType = iota
    ChangeDelete
    ChangeReplace
)
```

---

## 5. Rope Implementation Details

### 5.1 Node Structure

```go
// internal/engine/rope/node.go

// Node represents a node in the rope B+ tree.
// Uses a discriminated union pattern for Go.
type Node struct {
    // Shared fields
    height  uint8      // 0 for leaves, >0 for internal
    summary TextSummary // Aggregated metrics for subtree

    // Internal node fields (height > 0)
    children []*Node   // Child nodes (max branchingFactor)
    childSummaries []TextSummary // Per-child summaries for seeking

    // Leaf node fields (height == 0)
    chunks []Chunk     // Text chunks in this leaf
}

// Constants for tree structure
const (
    // Branching factor: balance between tree depth and node size
    // Higher = shallower tree, larger nodes
    // 16 is a good balance for modern CPUs (cache line friendly)
    minChildren = 8
    maxChildren = 16 // 2 * minChildren

    // Chunk size: balance between granularity and overhead
    // 128-512 bytes is typical; 256 is a good default
    minChunkSize = 128
    maxChunkSize = 256
)

// IsLeaf returns true if this is a leaf node.
func (n *Node) IsLeaf() bool {
    return n.height == 0
}

// Len returns the byte length of text in this subtree.
func (n *Node) Len() ByteOffset {
    return n.summary.Bytes
}
```

### 5.2 Metrics/Summary Types

```go
// internal/engine/rope/metrics.go

// TextSummary holds aggregated metrics for a text span.
// This is the "summary" type for our SumTree.
type TextSummary struct {
    // Primary metrics
    Bytes       ByteOffset // UTF-8 byte count
    UTF16Units  uint64     // UTF-16 code unit count (for LSP)
    Lines       uint32     // Number of newlines

    // For line length tracking (useful for horizontal scrolling)
    LongestLine     uint32 // Byte length of longest line
    FirstLineLen    uint32 // Length of first line (for concatenation)
    LastLineLen     uint32 // Length of last line (for concatenation)

    // Flags for optimization
    Flags TextFlags
}

// TextFlags indicate text properties for fast paths.
type TextFlags uint8

const (
    FlagASCII       TextFlags = 1 << iota // All ASCII (fast path for many operations)
    FlagHasNewlines                        // Contains newlines
    FlagHasTabs                            // Contains tabs
)

// Add combines two summaries (monoid operation).
// This is called when concatenating rope sections.
func (s TextSummary) Add(other TextSummary) TextSummary {
    result := TextSummary{
        Bytes:      s.Bytes + other.Bytes,
        UTF16Units: s.UTF16Units + other.UTF16Units,
        Lines:      s.Lines + other.Lines,
        Flags:      s.Flags & other.Flags, // AND for flags (all must have property)
    }

    // Update line length tracking
    if other.Lines > 0 {
        // Other has newlines, so longest line could be from either
        result.LongestLine = max(s.LongestLine, other.LongestLine)
        result.FirstLineLen = s.FirstLineLen
        result.LastLineLen = other.LastLineLen
    } else {
        // Other has no newlines, extends last line of s
        combined := s.LastLineLen + other.LastLineLen
        result.LongestLine = max(s.LongestLine, combined)
        result.FirstLineLen = s.FirstLineLen
        if s.Lines == 0 {
            result.FirstLineLen = combined
        }
        result.LastLineLen = combined
    }

    return result
}

// Zero returns the identity element for the summary monoid.
func (TextSummary) Zero() TextSummary {
    return TextSummary{Flags: FlagASCII} // Empty string is "ASCII"
}
```

### 5.3 Chunk Type

```go
// internal/engine/rope/chunk.go

// Chunk represents a bounded string stored in leaf nodes.
// Chunks are immutable once created.
type Chunk struct {
    data    string      // The actual text (immutable)
    summary TextSummary // Precomputed metrics
}

// NewChunk creates a chunk from a string.
// Computes summary metrics eagerly.
func NewChunk(s string) Chunk {
    return Chunk{
        data:    s,
        summary: computeSummary(s),
    }
}

// computeSummary calculates metrics for a string.
func computeSummary(s string) TextSummary {
    var sum TextSummary
    sum.Bytes = ByteOffset(len(s))

    isASCII := true
    var lineLen uint32

    for _, r := range s {
        // UTF-16 code units
        if r <= 0xFFFF {
            sum.UTF16Units++
        } else {
            sum.UTF16Units += 2 // Surrogate pair
        }

        // ASCII check
        if r > 127 {
            isASCII = false
        }

        // Line counting
        if r == '\n' {
            sum.Lines++
            sum.LongestLine = max(sum.LongestLine, lineLen)
            if sum.Lines == 1 {
                sum.FirstLineLen = lineLen
            }
            lineLen = 0
            sum.Flags |= FlagHasNewlines
        } else {
            lineLen += uint32(utf8.RuneLen(r))
            if r == '\t' {
                sum.Flags |= FlagHasTabs
            }
        }
    }

    sum.LastLineLen = lineLen
    if sum.Lines == 0 {
        sum.FirstLineLen = lineLen
        sum.LongestLine = lineLen
    }

    if isASCII {
        sum.Flags |= FlagASCII
    }

    return sum
}

// String returns the chunk's text.
func (c Chunk) String() string {
    return c.data
}

// Summary returns the chunk's precomputed metrics.
func (c Chunk) Summary() TextSummary {
    return c.summary
}

// Split splits a chunk at byte offset, returning two chunks.
func (c Chunk) Split(offset uint32) (Chunk, Chunk) {
    return NewChunk(c.data[:offset]), NewChunk(c.data[offset:])
}
```

### 5.4 Rope Type

```go
// internal/engine/rope/rope.go

// Rope is an immutable rope data structure for efficient text storage.
// Operations return new Rope values; the original is never modified.
type Rope struct {
    root *Node
}

// New creates an empty rope.
func New() Rope {
    return Rope{root: newLeafNode()}
}

// FromString creates a rope from a string.
func FromString(s string) Rope {
    if len(s) == 0 {
        return New()
    }

    // Split into chunks and build tree bottom-up
    chunks := splitIntoChunks(s)
    return buildFromChunks(chunks)
}

// FromReader creates a rope from an io.Reader.
func FromReader(r io.Reader) (Rope, error) {
    var builder Builder
    buf := make([]byte, 64*1024) // 64KB read buffer

    for {
        n, err := r.Read(buf)
        if n > 0 {
            builder.WriteString(string(buf[:n]))
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            return Rope{}, err
        }
    }

    return builder.Build(), nil
}

// Len returns the total byte length.
func (r Rope) Len() ByteOffset {
    if r.root == nil {
        return 0
    }
    return r.root.summary.Bytes
}

// LineCount returns the number of lines (newlines + 1).
func (r Rope) LineCount() uint32 {
    if r.root == nil {
        return 1
    }
    return r.root.summary.Lines + 1
}

// String returns the full text as a string.
// Use sparingly for large ropes.
func (r Rope) String() string {
    if r.root == nil {
        return ""
    }

    var sb strings.Builder
    sb.Grow(int(r.Len()))

    r.root.appendTo(&sb)
    return sb.String()
}
```

### 5.5 Core Rope Operations

```go
// internal/engine/rope/rope.go (continued)

// Insert inserts text at the given byte offset.
// Returns a new rope; original is unchanged.
func (r Rope) Insert(offset ByteOffset, text string) Rope {
    if len(text) == 0 {
        return r
    }
    if r.root == nil || offset == 0 {
        return FromString(text).Concat(r)
    }
    if offset >= r.Len() {
        return r.Concat(FromString(text))
    }

    // Split at offset, insert in middle
    left, right := r.Split(offset)
    return left.Concat(FromString(text)).Concat(right)
}

// Delete removes text in the given range.
// Returns a new rope; original is unchanged.
func (r Rope) Delete(start, end ByteOffset) Rope {
    if start >= end || r.root == nil {
        return r
    }

    // Clamp to valid range
    end = min(end, r.Len())
    start = min(start, end)

    // Split around the deleted region
    left, temp := r.Split(start)
    _, right := temp.Split(end - start)

    return left.Concat(right)
}

// Replace replaces text in the given range with new text.
func (r Rope) Replace(start, end ByteOffset, text string) Rope {
    return r.Delete(start, end).Insert(start, text)
}

// Slice returns a sub-rope for the given range.
func (r Rope) Slice(start, end ByteOffset) Rope {
    if r.root == nil || start >= end {
        return New()
    }

    end = min(end, r.Len())
    start = min(start, end)

    _, temp := r.Split(start)
    result, _ := temp.Split(end - start)
    return result
}

// Split splits the rope at offset, returning two ropes.
func (r Rope) Split(offset ByteOffset) (Rope, Rope) {
    if r.root == nil || offset == 0 {
        return New(), r
    }
    if offset >= r.Len() {
        return r, New()
    }

    leftRoot, rightRoot := r.root.split(offset)
    return Rope{leftRoot}, Rope{rightRoot}
}

// Concat concatenates two ropes.
// This is O(log n) due to tree rebalancing.
func (r Rope) Concat(other Rope) Rope {
    if r.root == nil {
        return other
    }
    if other.root == nil {
        return r
    }

    newRoot := concat(r.root, other.root)
    return Rope{newRoot}
}
```

### 5.6 Tree Cursor for Traversal

```go
// internal/engine/rope/cursor.go

// Cursor enables efficient traversal of the rope.
// It maintains a path from root to current position.
type Cursor struct {
    rope      Rope
    path      []cursorFrame // Stack of (node, childIndex) pairs
    offset    ByteOffset    // Current byte offset
    lineCol   Point         // Current line/column (cached)
    chunk     *Chunk        // Current chunk
    chunkOff  uint32        // Offset within current chunk
}

type cursorFrame struct {
    node       *Node
    childIndex int
    offset     ByteOffset // Offset at start of this node
}

// NewCursor creates a cursor at the start of the rope.
func NewCursor(r Rope) *Cursor {
    c := &Cursor{rope: r}
    c.seekStart()
    return c
}

// SeekByte moves the cursor to a byte offset.
// Returns false if offset is out of range.
func (c *Cursor) SeekByte(offset ByteOffset) bool {
    if offset > c.rope.Len() {
        return false
    }

    c.path = c.path[:0]
    c.offset = 0
    c.lineCol = Point{}

    node := c.rope.root
    if node == nil {
        return offset == 0
    }

    // Descend from root, tracking position
    for !node.IsLeaf() {
        c.path = append(c.path, cursorFrame{node: node, offset: c.offset})

        // Find child containing offset
        childOffset := ByteOffset(0)
        for i, childSum := range node.childSummaries {
            if childOffset+childSum.Bytes > offset {
                c.path[len(c.path)-1].childIndex = i
                c.offset = childOffset
                c.lineCol.Line += childSum.Lines // Update line tracking
                node = node.children[i]
                break
            }
            childOffset += childSum.Bytes
        }
    }

    // Find chunk within leaf
    chunkOffset := c.offset
    for i, chunk := range node.chunks {
        if chunkOffset+ByteOffset(len(chunk.data)) > offset {
            c.chunk = &node.chunks[i]
            c.chunkOff = uint32(offset - chunkOffset)
            c.offset = offset
            return true
        }
        chunkOffset += ByteOffset(len(chunk.data))
    }

    return false
}

// SeekLine moves the cursor to the start of a line.
func (c *Cursor) SeekLine(line uint32) bool {
    if line == 0 {
        return c.SeekByte(0)
    }

    // Similar to SeekByte but using line count as the dimension
    node := c.rope.root
    if node == nil || line > node.summary.Lines {
        return false
    }

    c.path = c.path[:0]
    c.offset = 0
    c.lineCol = Point{}
    targetLines := line

    for !node.IsLeaf() {
        c.path = append(c.path, cursorFrame{node: node, offset: c.offset})

        for i, childSum := range node.childSummaries {
            if childSum.Lines >= targetLines {
                c.path[len(c.path)-1].childIndex = i
                node = node.children[i]
                break
            }
            targetLines -= childSum.Lines
            c.offset += childSum.Bytes
            c.lineCol.Line += childSum.Lines
        }
    }

    // Find position within leaf chunks
    for i, chunk := range node.chunks {
        if chunk.summary.Lines >= targetLines {
            // Find newline within this chunk
            pos := findNthNewline(chunk.data, targetLines)
            c.chunk = &node.chunks[i]
            c.chunkOff = uint32(pos + 1)
            c.offset += ByteOffset(pos + 1)
            c.lineCol.Line = line
            c.lineCol.Column = 0
            return true
        }
        targetLines -= chunk.summary.Lines
        c.offset += ByteOffset(len(chunk.data))
        c.lineCol.Line += chunk.summary.Lines
    }

    return false
}

// Byte returns the current byte offset.
func (c *Cursor) Byte() ByteOffset {
    return c.offset
}

// Point returns the current line/column position.
func (c *Cursor) Point() Point {
    return c.lineCol
}

// Rune returns the rune at the current position.
func (c *Cursor) Rune() (rune, int) {
    if c.chunk == nil {
        return 0, 0
    }
    return utf8.DecodeRuneInString(c.chunk.data[c.chunkOff:])
}

// Next advances the cursor by one byte.
func (c *Cursor) Next() bool {
    // Implementation handles chunk/node boundaries
    // ...
}
```

---

## 6. Buffer API Design

### 6.1 Buffer Type

```go
// internal/engine/buffer/buffer.go

// Buffer wraps a Rope with additional editor functionality.
// It provides the primary interface for text manipulation.
type Buffer struct {
    mu          sync.RWMutex
    rope        rope.Rope
    revisionID  RevisionID
    lineEnding  LineEnding
    tabWidth    int

    // Change tracking
    tracker     *tracking.Tracker

    // Observers for change notifications
    observers   []ChangeObserver
}

// LineEnding specifies the line ending style.
type LineEnding uint8

const (
    LineEndingLF   LineEnding = iota // Unix: \n
    LineEndingCRLF                   // Windows: \r\n
    LineEndingCR                     // Old Mac: \r
)

// NewBuffer creates a new empty buffer.
func NewBuffer(opts ...Option) *Buffer {
    b := &Buffer{
        rope:       rope.New(),
        revisionID: NewRevisionID(),
        lineEnding: LineEndingLF,
        tabWidth:   4,
        tracker:    tracking.NewTracker(),
    }

    for _, opt := range opts {
        opt(b)
    }

    return b
}

// NewBufferFromString creates a buffer with initial content.
func NewBufferFromString(s string, opts ...Option) *Buffer {
    b := NewBuffer(opts...)
    b.rope = rope.FromString(s)
    return b
}

// NewBufferFromReader creates a buffer from an io.Reader.
func NewBufferFromReader(r io.Reader, opts ...Option) (*Buffer, error) {
    rp, err := rope.FromReader(r)
    if err != nil {
        return nil, err
    }

    b := NewBuffer(opts...)
    b.rope = rp
    return b, nil
}
```

### 6.2 Read Operations

```go
// internal/engine/buffer/buffer.go (continued)

// Text returns the full buffer content as a string.
// For large buffers, prefer using TextRange or iterators.
func (b *Buffer) Text() string {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return b.rope.String()
}

// TextRange returns text in the given byte range.
func (b *Buffer) TextRange(start, end ByteOffset) string {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return b.rope.Slice(start, end).String()
}

// Len returns the total byte length of the buffer.
func (b *Buffer) Len() ByteOffset {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return b.rope.Len()
}

// LineCount returns the number of lines.
func (b *Buffer) LineCount() uint32 {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return b.rope.LineCount()
}

// LineText returns the text of a specific line (without newline).
func (b *Buffer) LineText(line uint32) string {
    b.mu.RLock()
    defer b.mu.RUnlock()

    start := b.lineStartOffsetLocked(line)
    end := b.lineEndOffsetLocked(line)
    return b.rope.Slice(start, end).String()
}

// OffsetToPoint converts a byte offset to line/column.
func (b *Buffer) OffsetToPoint(offset ByteOffset) Point {
    b.mu.RLock()
    defer b.mu.RUnlock()

    cursor := rope.NewCursor(b.rope)
    cursor.SeekByte(offset)
    return cursor.Point()
}

// PointToOffset converts line/column to byte offset.
func (b *Buffer) PointToOffset(point Point) ByteOffset {
    b.mu.RLock()
    defer b.mu.RUnlock()

    cursor := rope.NewCursor(b.rope)
    cursor.SeekLine(point.Line)

    // Advance by column
    for col := uint32(0); col < point.Column; {
        r, size := cursor.Rune()
        if r == '\n' || size == 0 {
            break
        }
        col += uint32(size)
        cursor.Next()
    }

    return cursor.Byte()
}

// Snapshot returns a read-only snapshot of the current buffer state.
// Safe for concurrent access from other goroutines.
func (b *Buffer) Snapshot() Reader {
    b.mu.RLock()
    defer b.mu.RUnlock()

    return &bufferSnapshot{
        rope:       b.rope, // Ropes are immutable, safe to share
        revisionID: b.revisionID,
    }
}
```

### 6.3 Write Operations

```go
// internal/engine/buffer/buffer.go (continued)

// Insert inserts text at the given offset.
// Returns the end position of the inserted text.
func (b *Buffer) Insert(offset ByteOffset, text string) (ByteOffset, error) {
    b.mu.Lock()
    defer b.mu.Unlock()

    if offset > b.rope.Len() {
        return 0, ErrOffsetOutOfRange
    }

    // Normalize line endings
    text = b.normalizeLineEndings(text)

    oldRope := b.rope
    b.rope = b.rope.Insert(offset, text)
    b.revisionID = NewRevisionID()

    // Track change
    change := tracking.Change{
        Type:     tracking.ChangeInsert,
        Range:    Range{Start: offset, End: offset},
        NewRange: Range{Start: offset, End: offset + ByteOffset(len(text))},
        NewText:  text,
    }
    b.tracker.RecordChange(b.revisionID, change, oldRope)

    // Notify observers
    b.notifyObservers(change)

    return offset + ByteOffset(len(text)), nil
}

// Delete removes text in the given range.
func (b *Buffer) Delete(start, end ByteOffset) error {
    b.mu.Lock()
    defer b.mu.Unlock()

    if start > end || end > b.rope.Len() {
        return ErrRangeInvalid
    }

    oldText := b.rope.Slice(start, end).String()
    oldRope := b.rope
    b.rope = b.rope.Delete(start, end)
    b.revisionID = NewRevisionID()

    // Track change
    change := tracking.Change{
        Type:     tracking.ChangeDelete,
        Range:    Range{Start: start, End: end},
        NewRange: Range{Start: start, End: start},
        OldText:  oldText,
    }
    b.tracker.RecordChange(b.revisionID, change, oldRope)

    // Notify observers
    b.notifyObservers(change)

    return nil
}

// Replace replaces text in the given range with new text.
func (b *Buffer) Replace(start, end ByteOffset, text string) (ByteOffset, error) {
    b.mu.Lock()
    defer b.mu.Unlock()

    if start > end || end > b.rope.Len() {
        return 0, ErrRangeInvalid
    }

    text = b.normalizeLineEndings(text)
    oldText := b.rope.Slice(start, end).String()
    oldRope := b.rope
    b.rope = b.rope.Replace(start, end, text)
    b.revisionID = NewRevisionID()

    newEnd := start + ByteOffset(len(text))

    // Track change
    change := tracking.Change{
        Type:     tracking.ChangeReplace,
        Range:    Range{Start: start, End: end},
        NewRange: Range{Start: start, End: newEnd},
        OldText:  oldText,
        NewText:  text,
    }
    b.tracker.RecordChange(b.revisionID, change, oldRope)

    // Notify observers
    b.notifyObservers(change)

    return newEnd, nil
}

// ApplyEdits applies multiple edits atomically.
// Edits must be in reverse order (highest offset first) to maintain validity.
func (b *Buffer) ApplyEdits(edits []Edit) error {
    if len(edits) == 0 {
        return nil
    }

    b.mu.Lock()
    defer b.mu.Unlock()

    // Validate edits are in reverse order and non-overlapping
    for i := 1; i < len(edits); i++ {
        if edits[i].Range.End > edits[i-1].Range.Start {
            return ErrEditsOverlap
        }
    }

    oldRope := b.rope
    var changes []tracking.Change

    // Apply edits in reverse order
    for _, edit := range edits {
        oldText := b.rope.Slice(edit.Range.Start, edit.Range.End).String()
        text := b.normalizeLineEndings(edit.NewText)
        b.rope = b.rope.Replace(edit.Range.Start, edit.Range.End, text)

        newEnd := edit.Range.Start + ByteOffset(len(text))
        changes = append(changes, tracking.Change{
            Type:     tracking.ChangeReplace,
            Range:    edit.Range,
            NewRange: Range{Start: edit.Range.Start, End: newEnd},
            OldText:  oldText,
            NewText:  text,
        })
    }

    b.revisionID = NewRevisionID()
    b.tracker.RecordChanges(b.revisionID, changes, oldRope)

    // Notify observers
    for _, change := range changes {
        b.notifyObservers(change)
    }

    return nil
}
```

---

## 7. Cursor and Selection Management

### 7.1 Single Cursor Type

```go
// internal/engine/cursor/cursor.go

// Cursor represents an insertion point in the buffer.
type Cursor struct {
    offset ByteOffset // Primary position (byte offset)
}

// NewCursor creates a cursor at the given offset.
func NewCursor(offset ByteOffset) Cursor {
    return Cursor{offset: offset}
}

// Offset returns the cursor's byte offset.
func (c Cursor) Offset() ByteOffset {
    return c.offset
}

// MoveTo moves the cursor to a new offset.
func (c Cursor) MoveTo(offset ByteOffset) Cursor {
    return Cursor{offset: offset}
}
```

### 7.2 Selection Type

```go
// internal/engine/cursor/selection.go

// Selection represents a range of selected text.
// Anchor is where the selection started; Head is the current cursor position.
// When Anchor == Head, this represents a cursor with no selection.
type Selection struct {
    Anchor ByteOffset
    Head   ByteOffset
}

// NewSelection creates a selection from anchor to head.
func NewSelection(anchor, head ByteOffset) Selection {
    return Selection{Anchor: anchor, Head: head}
}

// NewCursorSelection creates a selection representing just a cursor.
func NewCursorSelection(offset ByteOffset) Selection {
    return Selection{Anchor: offset, Head: offset}
}

// IsEmpty returns true if the selection has no extent (just a cursor).
func (s Selection) IsEmpty() bool {
    return s.Anchor == s.Head
}

// Range returns the selection as a range (always Start <= End).
func (s Selection) Range() Range {
    if s.Anchor <= s.Head {
        return Range{Start: s.Anchor, End: s.Head}
    }
    return Range{Start: s.Head, End: s.Anchor}
}

// Start returns the lower bound of the selection.
func (s Selection) Start() ByteOffset {
    return min(s.Anchor, s.Head)
}

// End returns the upper bound of the selection.
func (s Selection) End() ByteOffset {
    return max(s.Anchor, s.Head)
}

// Cursor returns the head position.
func (s Selection) Cursor() ByteOffset {
    return s.Head
}

// IsForward returns true if the selection extends forward (head >= anchor).
func (s Selection) IsForward() bool {
    return s.Head >= s.Anchor
}

// Extend extends the selection to include the given offset.
func (s Selection) Extend(offset ByteOffset) Selection {
    return Selection{Anchor: s.Anchor, Head: offset}
}

// Collapse collapses the selection to a cursor at the head.
func (s Selection) Collapse() Selection {
    return Selection{Anchor: s.Head, Head: s.Head}
}

// CollapseToStart collapses the selection to its start.
func (s Selection) CollapseToStart() Selection {
    start := s.Start()
    return Selection{Anchor: start, Head: start}
}

// CollapseToEnd collapses the selection to its end.
func (s Selection) CollapseToEnd() Selection {
    end := s.End()
    return Selection{Anchor: end, Head: end}
}
```

### 7.3 Multi-Cursor Manager

```go
// internal/engine/cursor/cursors.go

// CursorSet manages multiple cursors/selections.
// Cursors are kept sorted by position and non-overlapping.
type CursorSet struct {
    selections []Selection
}

// NewCursorSet creates a cursor set with a single cursor.
func NewCursorSet(initial Selection) *CursorSet {
    return &CursorSet{
        selections: []Selection{initial},
    }
}

// Primary returns the primary (first) selection.
func (cs *CursorSet) Primary() Selection {
    if len(cs.selections) == 0 {
        return Selection{}
    }
    return cs.selections[0]
}

// All returns all selections.
func (cs *CursorSet) All() []Selection {
    return cs.selections
}

// Count returns the number of cursors/selections.
func (cs *CursorSet) Count() int {
    return len(cs.selections)
}

// Add adds a new selection, merging with overlapping ones.
func (cs *CursorSet) Add(sel Selection) {
    cs.selections = append(cs.selections, sel)
    cs.normalize()
}

// SetPrimary sets the primary selection, keeping others.
func (cs *CursorSet) SetPrimary(sel Selection) {
    if len(cs.selections) == 0 {
        cs.selections = []Selection{sel}
    } else {
        cs.selections[0] = sel
    }
    cs.normalize()
}

// Clear removes all selections except primary.
func (cs *CursorSet) Clear() {
    if len(cs.selections) > 1 {
        cs.selections = cs.selections[:1]
    }
}

// normalize sorts selections and merges overlapping ones.
func (cs *CursorSet) normalize() {
    if len(cs.selections) <= 1 {
        return
    }

    // Sort by start position
    sort.Slice(cs.selections, func(i, j int) bool {
        return cs.selections[i].Start() < cs.selections[j].Start()
    })

    // Merge overlapping
    merged := cs.selections[:1]
    for _, sel := range cs.selections[1:] {
        last := &merged[len(merged)-1]
        if sel.Start() <= last.End() {
            // Overlapping: extend last selection
            if sel.End() > last.End() {
                last.Head = sel.End()
                if !last.IsForward() {
                    last.Anchor = sel.End()
                }
            }
        } else {
            merged = append(merged, sel)
        }
    }
    cs.selections = merged
}
```

### 7.4 Cursor Transformation After Edits

```go
// internal/engine/cursor/transform.go

// TransformOffset updates an offset after an edit.
// Returns the new offset position.
func TransformOffset(offset ByteOffset, edit Edit) ByteOffset {
    // Edit is entirely before offset: adjust by delta
    if edit.Range.End <= offset {
        oldLen := edit.Range.End - edit.Range.Start
        newLen := ByteOffset(len(edit.NewText))
        return offset - oldLen + newLen
    }

    // Edit starts at or after offset: no change
    if edit.Range.Start >= offset {
        return offset
    }

    // Edit spans offset: move to end of new text
    return edit.Range.Start + ByteOffset(len(edit.NewText))
}

// TransformSelection updates a selection after an edit.
func TransformSelection(sel Selection, edit Edit) Selection {
    return Selection{
        Anchor: TransformOffset(sel.Anchor, edit),
        Head:   TransformOffset(sel.Head, edit),
    }
}

// TransformCursorSet updates all selections after an edit.
func TransformCursorSet(cs *CursorSet, edit Edit) {
    for i := range cs.selections {
        cs.selections[i] = TransformSelection(cs.selections[i], edit)
    }
    cs.normalize()
}

// TransformCursorSetMulti updates selections after multiple edits.
// Edits should be in application order (will be reversed internally).
func TransformCursorSetMulti(cs *CursorSet, edits []Edit) {
    // Process edits in reverse order to maintain offset validity
    for i := len(edits) - 1; i >= 0; i-- {
        TransformCursorSet(cs, edits[i])
    }
}
```

---

## 8. Undo/Redo System Design

### 8.1 Operation Types

```go
// internal/engine/history/operation.go

// Operation represents a single undoable edit.
type Operation struct {
    // Edit data
    Range      Range      // Range that was modified
    OldText    string     // Text that was replaced
    NewText    string     // Text that was inserted

    // Cursor state for restore
    CursorsBefore []Selection
    CursorsAfter  []Selection

    // Metadata
    Timestamp  time.Time
    RevisionID RevisionID
}

// OperationInfo provides read-only info about an operation.
type OperationInfo struct {
    Description string
    Timestamp   time.Time
    BytesDelta  int // Positive for insertions, negative for deletions
}

// Invert returns an operation that undoes this one.
func (op *Operation) Invert() *Operation {
    return &Operation{
        Range:         Range{Start: op.Range.Start, End: op.Range.Start + ByteOffset(len(op.NewText))},
        OldText:       op.NewText,
        NewText:       op.OldText,
        CursorsBefore: op.CursorsAfter,
        CursorsAfter:  op.CursorsBefore,
        Timestamp:     time.Now(),
    }
}
```

### 8.2 Command Pattern

```go
// internal/engine/history/command.go

// Command represents a composable edit action.
type Command interface {
    Execute(buffer *Buffer, cursors *CursorSet) error
    Undo(buffer *Buffer, cursors *CursorSet) error
    Description() string
}

// InsertCommand inserts text at cursors.
type InsertCommand struct {
    Text      string
    operations []Operation // Filled during execute
}

func (c *InsertCommand) Execute(buffer *Buffer, cursors *CursorSet) error {
    c.operations = nil

    // Get selections in reverse order (highest offset first)
    sels := cursors.All()
    reversed := make([]Selection, len(sels))
    for i, sel := range sels {
        reversed[len(sels)-1-i] = sel
    }

    for _, sel := range reversed {
        op := Operation{
            Range:   sel.Range(),
            OldText: buffer.TextRange(sel.Start(), sel.End()),
            NewText: c.Text,
            CursorsBefore: []Selection{sel},
        }

        // Apply edit
        newEnd, err := buffer.Replace(sel.Start(), sel.End(), c.Text)
        if err != nil {
            return err
        }

        op.CursorsAfter = []Selection{NewCursorSelection(newEnd)}
        c.operations = append(c.operations, op)
    }

    // Update cursor positions
    cursors.transformAfterInserts(c.operations)

    return nil
}

func (c *InsertCommand) Undo(buffer *Buffer, cursors *CursorSet) error {
    // Apply inverse operations in reverse order
    for i := len(c.operations) - 1; i >= 0; i-- {
        inv := c.operations[i].Invert()
        _, err := buffer.Replace(inv.Range.Start, inv.Range.End, inv.NewText)
        if err != nil {
            return err
        }
    }

    // Restore cursor positions
    if len(c.operations) > 0 {
        cursors.SetFrom(c.operations[0].CursorsBefore)
    }

    return nil
}

func (c *InsertCommand) Description() string {
    if len(c.Text) == 1 {
        return "Type character"
    }
    return fmt.Sprintf("Insert %d characters", len(c.Text))
}

// DeleteCommand deletes selections or characters.
type DeleteCommand struct {
    Direction  DeleteDirection // Forward or backward
    operations []Operation
}

type DeleteDirection int

const (
    DeleteBackward DeleteDirection = iota // Backspace
    DeleteForward                          // Delete key
)

// Similar implementation to InsertCommand...

// CompoundCommand groups multiple commands as one undo unit.
type CompoundCommand struct {
    Name     string
    Commands []Command
}

func (c *CompoundCommand) Execute(buffer *Buffer, cursors *CursorSet) error {
    for _, cmd := range c.Commands {
        if err := cmd.Execute(buffer, cursors); err != nil {
            return err
        }
    }
    return nil
}

func (c *CompoundCommand) Undo(buffer *Buffer, cursors *CursorSet) error {
    for i := len(c.Commands) - 1; i >= 0; i-- {
        if err := c.Commands[i].Undo(buffer, cursors); err != nil {
            return err
        }
    }
    return nil
}

func (c *CompoundCommand) Description() string {
    return c.Name
}
```

### 8.3 History Stack

```go
// internal/engine/history/stack.go

// History manages undo/redo state.
type History struct {
    undoStack []*undoEntry
    redoStack []*undoEntry

    // Grouping state
    grouping    bool
    groupName   string
    groupCmds   []Command

    // Configuration
    maxEntries int
}

type undoEntry struct {
    command   Command
    timestamp time.Time
}

// NewHistory creates a new history manager.
func NewHistory(maxEntries int) *History {
    if maxEntries <= 0 {
        maxEntries = 1000 // Default
    }
    return &History{
        maxEntries: maxEntries,
    }
}

// Push adds a command to the undo stack.
// Clears the redo stack.
func (h *History) Push(cmd Command) {
    if h.grouping {
        h.groupCmds = append(h.groupCmds, cmd)
        return
    }

    h.undoStack = append(h.undoStack, &undoEntry{
        command:   cmd,
        timestamp: time.Now(),
    })

    // Clear redo stack
    h.redoStack = nil

    // Enforce max entries
    if len(h.undoStack) > h.maxEntries {
        h.undoStack = h.undoStack[len(h.undoStack)-h.maxEntries:]
    }
}

// BeginGroup starts a command group.
func (h *History) BeginGroup(name string) {
    if h.grouping {
        return // Already grouping, ignore nested
    }
    h.grouping = true
    h.groupName = name
    h.groupCmds = nil
}

// EndGroup finishes a command group.
func (h *History) EndGroup() {
    if !h.grouping {
        return
    }
    h.grouping = false

    if len(h.groupCmds) == 0 {
        return
    }

    // Create compound command
    compound := &CompoundCommand{
        Name:     h.groupName,
        Commands: h.groupCmds,
    }

    h.undoStack = append(h.undoStack, &undoEntry{
        command:   compound,
        timestamp: time.Now(),
    })

    h.redoStack = nil
    h.groupCmds = nil
}

// Undo undoes the last command.
func (h *History) Undo(buffer *Buffer, cursors *CursorSet) error {
    if len(h.undoStack) == 0 {
        return ErrNothingToUndo
    }

    entry := h.undoStack[len(h.undoStack)-1]
    h.undoStack = h.undoStack[:len(h.undoStack)-1]

    if err := entry.command.Undo(buffer, cursors); err != nil {
        return err
    }

    h.redoStack = append(h.redoStack, entry)
    return nil
}

// Redo redoes the last undone command.
func (h *History) Redo(buffer *Buffer, cursors *CursorSet) error {
    if len(h.redoStack) == 0 {
        return ErrNothingToRedo
    }

    entry := h.redoStack[len(h.redoStack)-1]
    h.redoStack = h.redoStack[:len(h.redoStack)-1]

    if err := entry.command.Execute(buffer, cursors); err != nil {
        return err
    }

    h.undoStack = append(h.undoStack, entry)
    return nil
}

// CanUndo returns true if undo is available.
func (h *History) CanUndo() bool {
    return len(h.undoStack) > 0
}

// CanRedo returns true if redo is available.
func (h *History) CanRedo() bool {
    return len(h.redoStack) > 0
}

// UndoInfo returns info about available undo operations.
func (h *History) UndoInfo() []OperationInfo {
    result := make([]OperationInfo, len(h.undoStack))
    for i, entry := range h.undoStack {
        result[i] = OperationInfo{
            Description: entry.command.Description(),
            Timestamp:   entry.timestamp,
        }
    }
    return result
}
```

---

## 9. Change Tracking and Snapshotting

### 9.1 Revision System

```go
// internal/engine/tracking/revision.go

// RevisionID uniquely identifies a buffer state.
type RevisionID uint64

var revisionCounter uint64

// NewRevisionID generates a new unique revision ID.
func NewRevisionID() RevisionID {
    return RevisionID(atomic.AddUint64(&revisionCounter, 1))
}

// Revision captures a buffer state at a point in time.
type Revision struct {
    ID        RevisionID
    Timestamp time.Time
    rope      rope.Rope // Snapshot of the rope (immutable, cheap to store)
}
```

### 9.2 Snapshot System

```go
// internal/engine/tracking/snapshot.go

// SnapshotID identifies a named snapshot.
type SnapshotID uint64

// Snapshot represents a named checkpoint of buffer state.
type Snapshot struct {
    ID        SnapshotID
    Name      string
    Timestamp time.Time
    Revision  RevisionID
    rope      rope.Rope
}

// SnapshotManager manages named snapshots.
type SnapshotManager struct {
    mu        sync.RWMutex
    snapshots map[SnapshotID]*Snapshot
    byName    map[string]*Snapshot
    nextID    SnapshotID
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager() *SnapshotManager {
    return &SnapshotManager{
        snapshots: make(map[SnapshotID]*Snapshot),
        byName:    make(map[string]*Snapshot),
        nextID:    1,
    }
}

// Create creates a new named snapshot.
func (sm *SnapshotManager) Create(name string, rope rope.Rope, rev RevisionID) SnapshotID {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    // Remove existing snapshot with same name
    if existing, ok := sm.byName[name]; ok {
        delete(sm.snapshots, existing.ID)
    }

    id := sm.nextID
    sm.nextID++

    snap := &Snapshot{
        ID:        id,
        Name:      name,
        Timestamp: time.Now(),
        Revision:  rev,
        rope:      rope, // Rope is immutable, safe to share
    }

    sm.snapshots[id] = snap
    if name != "" {
        sm.byName[name] = snap
    }

    return id
}

// Get retrieves a snapshot by ID.
func (sm *SnapshotManager) Get(id SnapshotID) (*Snapshot, bool) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    snap, ok := sm.snapshots[id]
    return snap, ok
}

// GetByName retrieves a snapshot by name.
func (sm *SnapshotManager) GetByName(name string) (*Snapshot, bool) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    snap, ok := sm.byName[name]
    return snap, ok
}

// Delete removes a snapshot.
func (sm *SnapshotManager) Delete(id SnapshotID) {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    if snap, ok := sm.snapshots[id]; ok {
        delete(sm.byName, snap.Name)
        delete(sm.snapshots, id)
    }
}
```

### 9.3 Change Tracker

```go
// internal/engine/tracking/tracker.go

// Tracker records changes for AI context queries.
type Tracker struct {
    mu sync.RWMutex

    // Recent changes, kept in a ring buffer
    changes   []trackedChange
    head      int
    count     int
    maxChanges int

    // Revision to rope mapping for efficient diffs
    revisions map[RevisionID]*Revision

    // Snapshots
    snapshots *SnapshotManager
}

type trackedChange struct {
    revision RevisionID
    change   Change
}

// NewTracker creates a new change tracker.
func NewTracker() *Tracker {
    return &Tracker{
        maxChanges: 10000,
        changes:    make([]trackedChange, 10000),
        revisions:  make(map[RevisionID]*Revision),
        snapshots:  NewSnapshotManager(),
    }
}

// RecordChange records a single change.
func (t *Tracker) RecordChange(rev RevisionID, change Change, ropeSnapshot rope.Rope) {
    t.mu.Lock()
    defer t.mu.Unlock()

    // Store change in ring buffer
    idx := (t.head + t.count) % t.maxChanges
    if t.count < t.maxChanges {
        t.count++
    } else {
        t.head = (t.head + 1) % t.maxChanges
    }

    t.changes[idx] = trackedChange{
        revision: rev,
        change:   change,
    }

    // Store revision snapshot
    t.revisions[rev] = &Revision{
        ID:        rev,
        Timestamp: time.Now(),
        rope:      ropeSnapshot,
    }

    // Cleanup old revisions (keep last N)
    t.cleanupOldRevisions()
}

// RecordChanges records multiple changes atomically.
func (t *Tracker) RecordChanges(rev RevisionID, changes []Change, ropeSnapshot rope.Rope) {
    t.mu.Lock()
    defer t.mu.Unlock()

    for _, change := range changes {
        idx := (t.head + t.count) % t.maxChanges
        if t.count < t.maxChanges {
            t.count++
        } else {
            t.head = (t.head + 1) % t.maxChanges
        }

        t.changes[idx] = trackedChange{
            revision: rev,
            change:   change,
        }
    }

    t.revisions[rev] = &Revision{
        ID:        rev,
        Timestamp: time.Now(),
        rope:      ropeSnapshot,
    }

    t.cleanupOldRevisions()
}

// ChangesSince returns all changes since a revision.
func (t *Tracker) ChangesSince(rev RevisionID) []Change {
    t.mu.RLock()
    defer t.mu.RUnlock()

    var result []Change
    for i := 0; i < t.count; i++ {
        idx := (t.head + i) % t.maxChanges
        tc := t.changes[idx]
        if tc.revision > rev {
            result = append(result, tc.change)
        }
    }

    return result
}

// CreateSnapshot creates a named snapshot of current state.
func (t *Tracker) CreateSnapshot(name string, currentRope rope.Rope, rev RevisionID) SnapshotID {
    return t.snapshots.Create(name, currentRope, rev)
}

// DiffSinceSnapshot returns changes since a snapshot.
func (t *Tracker) DiffSinceSnapshot(id SnapshotID, currentRope rope.Rope) ([]Change, error) {
    snap, ok := t.snapshots.Get(id)
    if !ok {
        return nil, ErrSnapshotNotFound
    }

    return t.ChangesSince(snap.Revision), nil
}

// GetSnapshotText returns the text from a snapshot.
func (t *Tracker) GetSnapshotText(id SnapshotID) (string, error) {
    snap, ok := t.snapshots.Get(id)
    if !ok {
        return "", ErrSnapshotNotFound
    }

    return snap.rope.String(), nil
}
```

### 9.4 Diff Computation

```go
// internal/engine/tracking/diff.go

// DiffOptions configures diff computation.
type DiffOptions struct {
    ContextLines int  // Lines of context around changes
    IgnoreCase   bool // Case-insensitive comparison
    IgnoreSpace  bool // Ignore whitespace differences
}

// LineDiff represents a line-based diff.
type LineDiff struct {
    OldStart   int      // Starting line in old text
    OldCount   int      // Number of lines in old text
    NewStart   int      // Starting line in new text
    NewCount   int      // Number of lines in new text
    OldLines   []string // Old lines
    NewLines   []string // New lines
}

// ComputeLineDiff computes line-based diff between two ropes.
// Uses Myers diff algorithm for optimal results.
func ComputeLineDiff(old, new rope.Rope, opts DiffOptions) []LineDiff {
    oldLines := toLines(old)
    newLines := toLines(new)

    // Myers diff algorithm implementation
    return myersDiff(oldLines, newLines, opts)
}

// toLines extracts lines from a rope efficiently.
func toLines(r rope.Rope) []string {
    var lines []string
    iter := r.Lines()
    for iter.Next() {
        lines = append(lines, iter.Text())
    }
    return lines
}

// myersDiff implements the Myers diff algorithm.
// Returns a minimal edit script as LineDiff slices.
func myersDiff(oldLines, newLines []string, opts DiffOptions) []LineDiff {
    // Implementation of Myers' O(ND) diff algorithm
    // See: https://neil.fraser.name/software/diff_match_patch/myers.pdf

    n := len(oldLines)
    m := len(newLines)
    max := n + m

    v := make(map[int]int)
    trace := make([]map[int]int, 0)

    // Forward pass
    for d := 0; d <= max; d++ {
        vCopy := make(map[int]int)
        for k, val := range v {
            vCopy[k] = val
        }
        trace = append(trace, vCopy)

        for k := -d; k <= d; k += 2 {
            var x int
            if k == -d || (k != d && v[k-1] < v[k+1]) {
                x = v[k+1]
            } else {
                x = v[k-1] + 1
            }

            y := x - k

            // Extend diagonal
            for x < n && y < m && linesEqual(oldLines[x], newLines[y], opts) {
                x++
                y++
            }

            v[k] = x

            if x >= n && y >= m {
                // Solution found, backtrack to build diff
                return backtrack(trace, oldLines, newLines, opts)
            }
        }
    }

    return nil
}

func linesEqual(a, b string, opts DiffOptions) bool {
    if opts.IgnoreCase {
        a = strings.ToLower(a)
        b = strings.ToLower(b)
    }
    if opts.IgnoreSpace {
        a = strings.TrimSpace(a)
        b = strings.TrimSpace(b)
    }
    return a == b
}

func backtrack(trace []map[int]int, oldLines, newLines []string, opts DiffOptions) []LineDiff {
    // Backtrack through the trace to build the diff
    // Implementation omitted for brevity
    return nil
}
```

---

## 10. Implementation Phases

### Phase 1: Core Rope

**Goal**: Implement the foundational rope data structure with all core operations.

**Tasks**:
1. `rope/chunk.go` - Chunk type with summary computation
2. `rope/metrics.go` - TextSummary type and monoid operations
3. `rope/node.go` - Node type (leaf and internal)
4. `rope/rope.go` - Main Rope type with:
   - `FromString`, `FromReader`
   - `Len`, `LineCount`, `String`
   - `Insert`, `Delete`, `Replace`
   - `Split`, `Concat`, `Slice`
5. `rope/cursor.go` - Tree cursor for traversal
6. `rope/iter.go` - Iterators (lines, chunks, runes)
7. `rope/builder.go` - Efficient bulk construction
8. Comprehensive tests and benchmarks

**Success Criteria**:
- All operations pass correctness tests
- O(log n) performance for insert/delete/access
- Memory usage comparable to string for small texts
- 10MB+ file handling without issues

### Phase 2: Buffer Layer

**Goal**: Build the Buffer abstraction over the rope.

**Tasks**:
1. `buffer/position.go` - Position types (ByteOffset, Point, PointUTF16)
2. `buffer/range.go` - Range types
3. `buffer/edit.go` - Edit type
4. `buffer/buffer.go` - Buffer implementation:
   - Read operations with locking
   - Write operations with change tracking hooks
   - Snapshot support
   - Line ending normalization
5. Integration tests

**Success Criteria**:
- Thread-safe concurrent access
- Correct line/column <-> byte offset conversion
- UTF-16 coordinate support for LSP

### Phase 3: Cursor System

**Goal**: Implement cursor and selection management.

**Tasks**:
1. `cursor/cursor.go` - Single cursor type
2. `cursor/selection.go` - Selection type
3. `cursor/cursors.go` - Multi-cursor CursorSet
4. `cursor/transform.go` - Cursor transformation after edits
5. Tests for edge cases (overlapping selections, etc.)

**Success Criteria**:
- Selections merge correctly when overlapping
- Cursor positions update correctly after edits
- Multi-cursor edits work efficiently

### Phase 4: Undo/Redo System

**Goal**: Implement full undo/redo with command pattern.

**Tasks**:
1. `history/operation.go` - Operation type
2. `history/command.go` - Command interface and implementations:
   - InsertCommand
   - DeleteCommand
   - ReplaceCommand
   - CompoundCommand
3. `history/stack.go` - History stack with grouping
4. `history/group.go` - Command grouping utilities
5. Integration with Buffer

**Success Criteria**:
- Undo/redo preserves cursor positions
- Command grouping works for compound edits
- Memory usage bounded by max entries

### Phase 5: Change Tracking

**Goal**: Implement AI-friendly change tracking and snapshots.

**Tasks**:
1. `tracking/revision.go` - Revision ID system
2. `tracking/snapshot.go` - Named snapshot management
3. `tracking/delta.go` - Change representation
4. `tracking/diff.go` - Diff computation (Myers algorithm)
5. `tracking/tracker.go` - Main Tracker type
6. Integration with Buffer

**Success Criteria**:
- Cheap snapshot creation (O(1))
- "What changed since X?" queries work correctly
- Line-level diff generation for AI context

### Phase 6: Engine Facade

**Goal**: Create the unified Engine API.

**Tasks**:
1. `engine.go` - Engine type combining all components
2. `options.go` - Configuration options
3. `errors.go` - Error types
4. Public API documentation
5. End-to-end integration tests
6. Performance benchmarks

**Success Criteria**:
- Clean, intuitive public API
- All features accessible through Engine
- Comprehensive documentation

### Phase 7: Optimization and Polish

**Goal**: Optimize hot paths and ensure production readiness.

**Tasks**:
1. Profile and optimize rope operations
2. Add newline index cache (like Zed's u128 optimization)
3. Lazy metrics computation where beneficial
4. Memory pool for node allocation
5. Fuzz testing for robustness
6. Documentation and examples

**Success Criteria**:
- Performance meets or exceeds existing editors
- No memory leaks
- Robust against malformed input

---

## 11. Key Algorithms

### 11.1 Rope Split Algorithm

```
Algorithm: Split(rope, offset)
Input: rope (Rope), offset (ByteOffset)
Output: (leftRope, rightRope)

1. If offset == 0: return (Empty, rope)
2. If offset >= rope.Len(): return (rope, Empty)

3. leftNodes, rightNodes = [], []
4. node = rope.root
5. remainingOffset = offset

6. While node is not a leaf:
   a. For each child in node.children:
      - If child.summary.Bytes >= remainingOffset:
        * Recursively split child at remainingOffset
        * Add left part to leftNodes
        * Add right part to rightNodes
        * Break
      - Else:
        * Add entire child to leftNodes
        * remainingOffset -= child.summary.Bytes
   b. node = next child to process

7. Split leaf node chunks at remainingOffset:
   a. Find chunk containing offset
   b. Split chunk into leftChunk, rightChunk
   c. Build leaf nodes from chunks

8. Build left and right trees from collected nodes
9. Rebalance if necessary
10. Return (Rope{leftRoot}, Rope{rightRoot})
```

### 11.2 Rope Concatenation with Balancing

```
Algorithm: Concat(rope1, rope2)
Input: rope1 (Rope), rope2 (Rope)
Output: concatenated Rope

1. If rope1 is empty: return rope2
2. If rope2 is empty: return rope1

3. // Try to merge at leaf level if possible
   If rope1.root.height == rope2.root.height:
     newRoot = createInternalNode(rope1.root, rope2.root)

   Else if rope1.root.height > rope2.root.height:
     // Descend right spine of rope1
     newRoot = concatIntoRight(rope1.root, rope2.root)

   Else:
     // Descend left spine of rope2
     newRoot = concatIntoLeft(rope1.root, rope2.root)

4. // Rebalance if tree too deep
   If needsRebalance(newRoot):
     newRoot = rebalance(newRoot)

5. Return Rope{newRoot}

Function rebalance(node):
  // Collect all leaf chunks
  chunks = collectChunks(node)
  // Rebuild balanced tree bottom-up
  return buildBalancedTree(chunks)
```

### 11.3 Line-to-Offset Seeking

```
Algorithm: SeekLine(cursor, targetLine)
Input: cursor (Cursor), targetLine (uint32)
Output: bool (success)

1. If targetLine == 0:
   cursor.offset = 0
   return true

2. node = cursor.rope.root
3. If node is nil or targetLine > node.summary.Lines:
   return false

4. remainingLines = targetLine
5. currentOffset = 0
6. path = []

7. While node is not a leaf:
   a. Push (node, currentOffset) to path
   b. For each (i, childSummary) in node.childSummaries:
      - If childSummary.Lines >= remainingLines:
        * node = node.children[i]
        * break
      - Else:
        * remainingLines -= childSummary.Lines
        * currentOffset += childSummary.Bytes

8. // Find newline within leaf chunks
   For each chunk in node.chunks:
     If chunk.summary.Lines >= remainingLines:
       // Scan chunk for nth newline
       pos = findNthNewline(chunk.data, remainingLines)
       cursor.offset = currentOffset + pos + 1
       cursor.lineCol = Point{Line: targetLine, Column: 0}
       return true
     Else:
       remainingLines -= chunk.summary.Lines
       currentOffset += chunk.Len()

9. return false
```

### 11.4 Multi-Cursor Edit Application

```
Algorithm: ApplyMultiCursorEdit(buffer, cursors, text)
Input: buffer (Buffer), cursors (CursorSet), text (string)
Output: error

1. // Get selections sorted by offset (descending)
   selections = cursors.All()
   Sort selections by Start() descending

2. // Validate no overlaps
   For i = 0 to len(selections)-2:
     If selections[i].Start() < selections[i+1].End():
       return ErrSelectionsOverlap

3. // Apply edits in reverse order (preserves offsets)
   edits = []
   For each sel in selections:
     edit = Edit{
       Range: sel.Range(),
       NewText: text,
     }
     edits = append(edits, edit)

4. // Batch apply edits
   buffer.ApplyEdits(edits)

5. // Transform cursor positions
   For each edit in edits (forward order):
     cursors.Transform(edit)

6. return nil
```

### 11.5 Undo with Cursor Restoration

```
Algorithm: Undo(history, buffer, cursors)
Input: history (History), buffer (Buffer), cursors (CursorSet)
Output: error

1. If history.undoStack is empty:
   return ErrNothingToUndo

2. entry = history.undoStack.Pop()

3. // Execute undo
   error = entry.command.Undo(buffer, cursors)
   If error != nil:
     // Restore entry to stack and return error
     history.undoStack.Push(entry)
     return error

4. // Move to redo stack
   history.redoStack.Push(entry)

5. return nil
```

---

## 12. Testing Strategy

### 12.1 Unit Tests

```go
// Example test structure for rope operations
func TestRopeInsert(t *testing.T) {
    tests := []struct {
        name     string
        initial  string
        offset   ByteOffset
        text     string
        expected string
    }{
        {"insert at start", "world", 0, "hello ", "hello world"},
        {"insert at end", "hello", 5, " world", "hello world"},
        {"insert in middle", "helloworld", 5, " ", "hello world"},
        {"insert into empty", "", 0, "hello", "hello"},
        {"insert empty string", "hello", 3, "", "hello"},
        {"insert unicode", "hello", 5, " \u4e16\u754c", "hello \u4e16\u754c"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            r := rope.FromString(tt.initial)
            r = r.Insert(tt.offset, tt.text)
            if got := r.String(); got != tt.expected {
                t.Errorf("got %q, want %q", got, tt.expected)
            }
        })
    }
}

// Property-based tests using quick check
func TestRopeProperties(t *testing.T) {
    // Property: Insert then delete returns original
    f := func(s string, offset int, insert string) bool {
        if len(s) == 0 {
            offset = 0
        } else {
            offset = offset % (len(s) + 1)
        }

        r := rope.FromString(s)
        r = r.Insert(ByteOffset(offset), insert)
        r = r.Delete(ByteOffset(offset), ByteOffset(offset+len(insert)))
        return r.String() == s
    }

    if err := quick.Check(f, nil); err != nil {
        t.Error(err)
    }
}
```

### 12.2 Benchmark Tests

```go
func BenchmarkRopeInsertRandom(b *testing.B) {
    sizes := []int{1000, 10000, 100000, 1000000}

    for _, size := range sizes {
        b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
            data := generateText(size)
            r := rope.FromString(data)

            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                offset := ByteOffset(rand.Intn(int(r.Len())))
                r = r.Insert(offset, "x")
            }
        })
    }
}

func BenchmarkRopeLineAccess(b *testing.B) {
    data := generateTextWithLines(100000, 80) // 100k lines, ~80 chars each
    r := rope.FromString(data)
    lineCount := r.LineCount()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        line := uint32(rand.Intn(int(lineCount)))
        cursor := rope.NewCursor(r)
        cursor.SeekLine(line)
    }
}
```

### 12.3 Fuzz Tests

```go
func FuzzRopeOperations(f *testing.F) {
    // Seed corpus
    f.Add([]byte("hello world"), uint64(5), []byte(" there"))
    f.Add([]byte(""), uint64(0), []byte("start"))

    f.Fuzz(func(t *testing.T, initial []byte, offset uint64, insert []byte) {
        // Ensure valid UTF-8
        initialStr := string(bytes.ToValidUTF8(initial, []byte("?")))
        insertStr := string(bytes.ToValidUTF8(insert, []byte("?")))

        r := rope.FromString(initialStr)
        if uint64(r.Len()) > 0 {
            offset = offset % uint64(r.Len()+1)
        } else {
            offset = 0
        }

        // Operations should not panic
        r = r.Insert(ByteOffset(offset), insertStr)
        _ = r.String()
        _ = r.Len()
        _ = r.LineCount()
    })
}
```

---

## 13. Performance Considerations

### 13.1 Memory Efficiency

**Structural Sharing**:
- Immutable rope nodes enable automatic structural sharing
- Snapshots cost O(1) memory (just a pointer)
- Undo history shares unchanged tree portions

**Node Sizing**:
- Internal nodes: ~256-512 bytes (cache-line friendly)
- Leaf chunks: 128-512 bytes (balance granularity vs overhead)
- Target: 90%+ memory utilization

**Garbage Collection**:
- Use sync.Pool for frequently allocated small objects
- Consider arena allocation for bulk operations
- Profile and monitor GC pressure

### 13.2 CPU Efficiency

**Hot Path Optimizations**:
- Newline index cache (u128 bitmask per chunk)
- ASCII fast path (avoid UTF-8 decoding when possible)
- SIMD-friendly loops for character counting

**Avoiding Allocations**:
- Reuse cursors where possible
- Pre-allocate buffers for string building
- Use string interning for common strings

**Branch Prediction**:
- Order conditionals by likelihood
- Use branchless code for hot paths

### 13.3 Concurrency

**Read-Write Locking**:
- Use sync.RWMutex for buffer access
- Readers don't block each other
- Writers get exclusive access

**Copy-on-Write**:
- Immutable ropes enable lock-free reads
- Snapshots are inherently thread-safe
- AI context building can run concurrently

**Background Processing**:
- Syntax highlighting on snapshot
- Change tracking asynchronous
- Diff computation off main thread

---

## References

- [Wikipedia: Rope data structure](https://en.wikipedia.org/wiki/Rope_(data_structure))
- [Zed Blog: Rope & SumTree](https://zed.dev/blog/zed-decoded-rope-sumtree)
- [Zed Blog: Rope Optimizations](https://zed.dev/blog/zed-decoded-rope-optimizations-part-1)
- [Xi Editor: Rope Science - Metrics](https://xi-editor.io/docs/rope_science_02.html)
- [Xi Editor: CRDT Details](https://xi-editor.io/docs/crdt-details.html)
- [Ropey Design Document](https://github.com/cessen/ropey/blob/master/design/design.md)
- [Boehm: Ropes Paper (1995)](https://www.cs.tufts.edu/comp/150FP/archive/hans-boehm/ropes.pdf)
- [Myers Diff Algorithm](https://neil.fraser.name/software/diff_match_patch/myers.pdf)
