package tracking

import (
	"sync"

	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/rope"
)

// DefaultMaxChanges is the default maximum number of changes to track.
const DefaultMaxChanges = 10000

// DefaultMaxRevisions is the default maximum number of revisions to store.
const DefaultMaxRevisions = 100

// TrackerOption configures a Tracker.
type TrackerOption func(*Tracker)

// WithMaxChanges sets the maximum number of changes to track.
// IMPORTANT: This option must only be used during Tracker creation via NewTracker.
// Applying it to an existing Tracker with recorded changes will discard those changes.
func WithMaxChanges(maxChanges int) TrackerOption {
	return func(t *Tracker) {
		t.maxChanges = maxChanges
		t.changes = make([]trackedChange, maxChanges)
	}
}

// WithMaxRevisions sets the maximum number of revisions to store.
func WithMaxRevisions(maxRevisions int) TrackerOption {
	return func(t *Tracker) {
		t.revisions = newRevisionStore(maxRevisions)
	}
}

// Tracker records changes for AI context queries.
// It maintains a bounded history of changes and supports named snapshots.
// All operations are thread-safe.
type Tracker struct {
	mu sync.RWMutex

	// Recent changes in a ring buffer
	changes    []trackedChange
	head       int // Index of oldest entry
	count      int // Number of entries
	maxChanges int

	// Revision snapshots for efficient diffs
	revisions *revisionStore

	// Named snapshots
	snapshots *SnapshotManager
}

// NewTracker creates a new change tracker with default settings.
func NewTracker(opts ...TrackerOption) *Tracker {
	t := &Tracker{
		maxChanges: DefaultMaxChanges,
		changes:    make([]trackedChange, DefaultMaxChanges),
		revisions:  newRevisionStore(DefaultMaxRevisions),
		snapshots:  NewSnapshotManager(),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// RecordChange records a single change.
// The ropeSnapshot should be the rope BEFORE the change was applied.
func (t *Tracker) RecordChange(rev RevisionID, change Change, ropeSnapshot rope.Rope) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.recordChangeLocked(rev, change)
	t.storeRevisionLocked(rev, ropeSnapshot)
}

// RecordChanges records multiple changes atomically.
// The ropeSnapshot should be the rope BEFORE any changes were applied.
func (t *Tracker) RecordChanges(rev RevisionID, changes []Change, ropeSnapshot rope.Rope) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, change := range changes {
		t.recordChangeLocked(rev, change)
	}
	t.storeRevisionLocked(rev, ropeSnapshot)
}

// recordChangeLocked adds a change to the ring buffer (must hold lock).
func (t *Tracker) recordChangeLocked(rev RevisionID, change Change) {
	idx := (t.head + t.count) % t.maxChanges
	if t.count < t.maxChanges {
		t.count++
	} else {
		// Ring buffer is full, advance head
		t.head = (t.head + 1) % t.maxChanges
	}

	t.changes[idx] = trackedChange{
		revision: rev,
		change:   change,
	}
}

// storeRevisionLocked stores a revision snapshot (must hold lock).
func (t *Tracker) storeRevisionLocked(rev RevisionID, ropeSnapshot rope.Rope) {
	t.revisions.Add(NewRevision(rev, ropeSnapshot))
}

// ChangesSince returns all changes since a revision.
// Returns changes in chronological order.
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

// ChangesSinceWithLimit returns up to limit changes since a revision.
func (t *Tracker) ChangesSinceWithLimit(rev RevisionID, limit int) []Change {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []Change
	for i := 0; i < t.count && len(result) < limit; i++ {
		idx := (t.head + i) % t.maxChanges
		tc := t.changes[idx]
		if tc.revision > rev {
			result = append(result, tc.change)
		}
	}

	return result
}

// ChangesBetween returns changes between two revisions (exclusive start, inclusive end).
func (t *Tracker) ChangesBetween(startRev, endRev RevisionID) []Change {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []Change
	for i := 0; i < t.count; i++ {
		idx := (t.head + i) % t.maxChanges
		tc := t.changes[idx]
		if tc.revision > startRev && tc.revision <= endRev {
			result = append(result, tc.change)
		}
	}

	return result
}

// LatestChanges returns the most recent N changes.
func (t *Tracker) LatestChanges(n int) []Change {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if n > t.count {
		n = t.count
	}

	result := make([]Change, n)
	for i := 0; i < n; i++ {
		// Start from the most recent
		idx := (t.head + t.count - 1 - i) % t.maxChanges
		if idx < 0 {
			idx += t.maxChanges
		}
		result[n-1-i] = t.changes[idx].change // Reverse to get chronological order
	}

	return result
}

// ChangeCount returns the number of tracked changes.
func (t *Tracker) ChangeCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.count
}

// Snapshot Operations

// CreateSnapshot creates a named snapshot of the current state.
func (t *Tracker) CreateSnapshot(name string, currentRope rope.Rope, rev RevisionID) SnapshotID {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snapshots.Create(name, currentRope, rev)
}

// GetSnapshot retrieves a snapshot by ID.
func (t *Tracker) GetSnapshot(id SnapshotID) (*Snapshot, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snap, ok := t.snapshots.Get(id)
	if !ok {
		return nil, ErrSnapshotNotFound
	}
	return snap, nil
}

// GetSnapshotByName retrieves a snapshot by name.
func (t *Tracker) GetSnapshotByName(name string) (*Snapshot, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snap, ok := t.snapshots.GetByName(name)
	if !ok {
		return nil, ErrSnapshotNotFound
	}
	return snap, nil
}

// DeleteSnapshot removes a snapshot.
func (t *Tracker) DeleteSnapshot(id SnapshotID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.snapshots.Delete(id)
}

// DeleteSnapshotByName removes a snapshot by name.
func (t *Tracker) DeleteSnapshotByName(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.snapshots.DeleteByName(name)
}

// ListSnapshots returns all snapshots.
func (t *Tracker) ListSnapshots() []*Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.snapshots.List()
}

// SnapshotCount returns the number of snapshots.
func (t *Tracker) SnapshotCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.snapshots.Count()
}

// Diff Operations

// DiffSinceSnapshot returns changes since a snapshot.
// Returns the change history rather than computing a diff.
func (t *Tracker) DiffSinceSnapshot(id SnapshotID) ([]Change, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snap, ok := t.snapshots.Get(id)
	if !ok {
		return nil, ErrSnapshotNotFound
	}

	return t.changesSinceLocked(snap.Revision), nil
}

// ComputeDiffSinceSnapshot computes a line-level diff from a snapshot to current state.
func (t *Tracker) ComputeDiffSinceSnapshot(id SnapshotID, currentRope rope.Rope, opts DiffOptions) (DiffResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snap, ok := t.snapshots.Get(id)
	if !ok {
		return DiffResult{}, ErrSnapshotNotFound
	}

	return ComputeLineDiff(snap.Rope(), currentRope, opts), nil
}

// ComputeDiffBetweenSnapshots computes a line-level diff between two snapshots.
func (t *Tracker) ComputeDiffBetweenSnapshots(fromID, toID SnapshotID, opts DiffOptions) (DiffResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	fromSnap, ok := t.snapshots.Get(fromID)
	if !ok {
		return DiffResult{}, ErrSnapshotNotFound
	}

	toSnap, ok := t.snapshots.Get(toID)
	if !ok {
		return DiffResult{}, ErrSnapshotNotFound
	}

	return ComputeLineDiff(fromSnap.Rope(), toSnap.Rope(), opts), nil
}

// GetSnapshotText returns the full text from a snapshot.
func (t *Tracker) GetSnapshotText(id SnapshotID) (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snap, ok := t.snapshots.Get(id)
	if !ok {
		return "", ErrSnapshotNotFound
	}

	return snap.Text(), nil
}

// Revision Operations

// GetRevision retrieves a stored revision.
func (t *Tracker) GetRevision(id RevisionID) (*Revision, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.revisions.Get(id)
}

// RevisionCount returns the number of stored revisions.
func (t *Tracker) RevisionCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.revisions.Len()
}

// Internal helpers

// changesSinceLocked returns changes since a revision (must hold lock).
func (t *Tracker) changesSinceLocked(rev RevisionID) []Change {
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

// Clear removes all tracked changes, revisions, and snapshots.
func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.head = 0
	t.count = 0
	t.revisions.Clear()
	t.snapshots.Clear()
}

// ChangeSet Operations

// BuildChangeSet creates a ChangeSet from changes since a revision.
func (t *Tracker) BuildChangeSet(sinceRev RevisionID) *ChangeSet {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cs := NewChangeSet(sinceRev)

	for i := 0; i < t.count; i++ {
		idx := (t.head + i) % t.maxChanges
		tc := t.changes[idx]
		if tc.revision > sinceRev {
			cs.Add(tc.change)
		}
	}

	return cs
}

// BuildChangeSetBetween creates a ChangeSet for changes between two revisions.
func (t *Tracker) BuildChangeSetBetween(startRev, endRev RevisionID) *ChangeSet {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cs := NewChangeSet(startRev)

	for i := 0; i < t.count; i++ {
		idx := (t.head + i) % t.maxChanges
		tc := t.changes[idx]
		if tc.revision > startRev && tc.revision <= endRev {
			cs.Add(tc.change)
		}
	}

	return cs
}

// AI Context Helpers

// GetAIContext returns a summary of recent changes suitable for AI context.
// It includes the change summary and optionally line-level diffs.
func (t *Tracker) GetAIContext(currentRope rope.Rope, opts AIContextOptions) AIContext {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ctx := AIContext{}

	// Get recent changes
	changes := t.changesSinceLocked(opts.SinceRevision)
	if opts.MaxChanges > 0 && len(changes) > opts.MaxChanges {
		changes = changes[len(changes)-opts.MaxChanges:]
	}
	ctx.Changes = changes

	// Build change set summary
	cs := &ChangeSet{
		StartRevision: opts.SinceRevision,
	}
	for _, c := range changes {
		cs.Add(c)
	}
	ctx.Summary = cs.Summary()

	// Compute line diff if requested
	if opts.IncludeDiff {
		if snap, ok := t.snapshots.GetByName(opts.DiffFromSnapshot); ok {
			ctx.Diff = ComputeLineDiff(snap.Rope(), currentRope, opts.DiffOptions)
			ctx.HasDiff = true
		}
	}

	return ctx
}

// AIContextOptions configures AI context generation.
type AIContextOptions struct {
	// SinceRevision limits changes to those after this revision.
	SinceRevision RevisionID

	// MaxChanges limits the number of changes returned.
	// 0 means no limit.
	MaxChanges int

	// IncludeDiff includes a line-level diff if true.
	IncludeDiff bool

	// DiffFromSnapshot is the snapshot name to diff from.
	// Only used if IncludeDiff is true.
	DiffFromSnapshot string

	// DiffOptions configures the diff computation.
	DiffOptions DiffOptions
}

// AIContext contains information suitable for AI context.
type AIContext struct {
	// Changes is the list of recent changes.
	Changes []Change

	// Summary is a human-readable summary of the changes.
	Summary string

	// HasDiff indicates if Diff is populated.
	HasDiff bool

	// Diff is the line-level diff if requested.
	Diff DiffResult
}

// ChangeObserver defines the interface for change tracking integration.
// This can be implemented by buffer types to enable automatic tracking.
type ChangeObserver interface {
	// OnChange is called when a change occurs.
	OnChange(rev RevisionID, change Change, beforeRope rope.Rope)
}

// TrackerObserver wraps a Tracker to implement ChangeObserver.
type TrackerObserver struct {
	tracker *Tracker
}

// NewTrackerObserver creates a new observer that records changes to a tracker.
func NewTrackerObserver(t *Tracker) *TrackerObserver {
	return &TrackerObserver{tracker: t}
}

// OnChange implements ChangeObserver.
func (to *TrackerObserver) OnChange(rev RevisionID, change Change, beforeRope rope.Rope) {
	to.tracker.RecordChange(rev, change, beforeRope)
}

// FromBufferEdit converts a buffer edit result to a Change.
func FromBufferEdit(result buffer.EditResult, newText string, revID RevisionID) Change {
	var changeType ChangeType
	if result.OldRange.IsEmpty() {
		changeType = ChangeInsert
	} else if newText == "" {
		changeType = ChangeDelete
	} else {
		changeType = ChangeReplace
	}

	return Change{
		Type:       changeType,
		Range:      result.OldRange,
		NewRange:   result.NewRange,
		OldText:    result.OldText,
		NewText:    newText,
		RevisionID: revID,
	}
}
