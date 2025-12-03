package tracking

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dshills/keystorm/internal/engine/rope"
)

// Errors returned by snapshot operations.
var (
	ErrSnapshotNotFound = errors.New("snapshot not found")
)

// SnapshotID uniquely identifies a named snapshot.
type SnapshotID uint64

// snapshotIDCounter generates unique snapshot IDs.
var snapshotIDCounter uint64

// NewSnapshotID generates a new unique snapshot ID.
func NewSnapshotID() SnapshotID {
	return SnapshotID(atomic.AddUint64(&snapshotIDCounter, 1))
}

// Snapshot represents a named checkpoint of buffer state.
// Snapshots are immutable and can be safely shared across goroutines.
type Snapshot struct {
	// ID uniquely identifies this snapshot.
	ID SnapshotID

	// Name is the human-readable name for this snapshot.
	// Common names include "before_ai_edit", "checkpoint_1", etc.
	Name string

	// Timestamp when this snapshot was created.
	Timestamp time.Time

	// Revision is the buffer revision at the time of snapshot.
	Revision RevisionID

	// rope is the immutable rope snapshot.
	// Since ropes are immutable, this is O(1) to create.
	rope rope.Rope
}

// NewSnapshot creates a new snapshot with the given parameters.
func NewSnapshot(name string, rp rope.Rope, revision RevisionID) *Snapshot {
	return &Snapshot{
		ID:        NewSnapshotID(),
		Name:      name,
		Timestamp: time.Now(),
		Revision:  revision,
		rope:      rp,
	}
}

// Rope returns the rope snapshot.
func (s *Snapshot) Rope() rope.Rope {
	return s.rope
}

// Text returns the full text at this snapshot.
// Use sparingly for large buffers.
func (s *Snapshot) Text() string {
	return s.rope.String()
}

// Len returns the byte length at this snapshot.
func (s *Snapshot) Len() int64 {
	return int64(s.rope.Len())
}

// LineCount returns the number of lines at this snapshot.
func (s *Snapshot) LineCount() uint32 {
	return s.rope.LineCount()
}

// Age returns how long ago this snapshot was created.
func (s *Snapshot) Age() time.Duration {
	return time.Since(s.Timestamp)
}

// SnapshotManager manages named snapshots.
// All operations are thread-safe.
type SnapshotManager struct {
	mu        sync.RWMutex
	snapshots map[SnapshotID]*Snapshot
	byName    map[string]*Snapshot
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		snapshots: make(map[SnapshotID]*Snapshot),
		byName:    make(map[string]*Snapshot),
	}
}

// Create creates a new named snapshot.
// If a snapshot with the same name exists, it is replaced.
func (sm *SnapshotManager) Create(name string, rp rope.Rope, revision RevisionID) SnapshotID {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Remove existing snapshot with same name
	if existing, ok := sm.byName[name]; ok {
		delete(sm.snapshots, existing.ID)
	}

	snap := NewSnapshot(name, rp, revision)

	sm.snapshots[snap.ID] = snap
	if name != "" {
		sm.byName[name] = snap
	}

	return snap.ID
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

// Delete removes a snapshot by ID.
func (sm *SnapshotManager) Delete(id SnapshotID) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if snap, ok := sm.snapshots[id]; ok {
		if snap.Name != "" {
			delete(sm.byName, snap.Name)
		}
		delete(sm.snapshots, id)
	}
}

// DeleteByName removes a snapshot by name.
func (sm *SnapshotManager) DeleteByName(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if snap, ok := sm.byName[name]; ok {
		delete(sm.snapshots, snap.ID)
		delete(sm.byName, name)
	}
}

// List returns all snapshots, sorted by timestamp (oldest first).
func (sm *SnapshotManager) List() []*Snapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	snapshots := make([]*Snapshot, 0, len(sm.snapshots))
	for _, snap := range sm.snapshots {
		snapshots = append(snapshots, snap)
	}

	// Sort by timestamp (oldest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.Before(snapshots[j].Timestamp)
	})

	return snapshots
}

// Count returns the number of snapshots.
func (sm *SnapshotManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.snapshots)
}

// Clear removes all snapshots.
func (sm *SnapshotManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.snapshots = make(map[SnapshotID]*Snapshot)
	sm.byName = make(map[string]*Snapshot)
}

// Names returns all snapshot names.
func (sm *SnapshotManager) Names() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	names := make([]string, 0, len(sm.byName))
	for name := range sm.byName {
		names = append(names, name)
	}
	return names
}

// Prune removes snapshots older than the given duration.
// Returns the number of snapshots removed.
func (sm *SnapshotManager) Prune(maxAge time.Duration) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	var removed int

	for id, snap := range sm.snapshots {
		if snap.Timestamp.Before(cutoff) {
			if snap.Name != "" {
				delete(sm.byName, snap.Name)
			}
			delete(sm.snapshots, id)
			removed++
		}
	}

	return removed
}

// PruneKeepN removes oldest snapshots, keeping only the N most recent.
// Returns the number of snapshots removed.
func (sm *SnapshotManager) PruneKeepN(n int) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.snapshots) <= n {
		return 0
	}

	// Get all snapshots sorted by timestamp
	snapshots := make([]*Snapshot, 0, len(sm.snapshots))
	for _, snap := range sm.snapshots {
		snapshots = append(snapshots, snap)
	}

	// Sort by timestamp (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})

	// Remove oldest
	removed := 0
	for i := n; i < len(snapshots); i++ {
		snap := snapshots[i]
		if snap.Name != "" {
			delete(sm.byName, snap.Name)
		}
		delete(sm.snapshots, snap.ID)
		removed++
	}

	return removed
}
