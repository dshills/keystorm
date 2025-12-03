package tracking

import (
	"time"

	"github.com/dshills/keystorm/internal/engine/buffer"
	"github.com/dshills/keystorm/internal/engine/rope"
)

// RevisionID is an alias to buffer.RevisionID for convenience.
// It uniquely identifies a buffer state at a point in time.
type RevisionID = buffer.RevisionID

// Revision captures a buffer state at a point in time.
// It stores a reference to the immutable rope, enabling cheap
// storage through structural sharing.
type Revision struct {
	// ID uniquely identifies this revision.
	ID RevisionID

	// Timestamp when this revision was created.
	Timestamp time.Time

	// rope is the snapshot of the rope at this revision.
	// Since ropes are immutable, this is a cheap reference.
	rope rope.Rope
}

// NewRevision creates a new revision with the given ID and rope snapshot.
func NewRevision(id RevisionID, rp rope.Rope) *Revision {
	return &Revision{
		ID:        id,
		Timestamp: time.Now(),
		rope:      rp,
	}
}

// Rope returns the rope snapshot for this revision.
func (r *Revision) Rope() rope.Rope {
	return r.rope
}

// Text returns the full text content at this revision.
// Use sparingly for large buffers.
func (r *Revision) Text() string {
	return r.rope.String()
}

// Len returns the byte length at this revision.
func (r *Revision) Len() int64 {
	return int64(r.rope.Len())
}

// LineCount returns the number of lines at this revision.
func (r *Revision) LineCount() uint32 {
	return r.rope.LineCount()
}

// revisionStore manages a bounded collection of revisions.
// It uses a map for fast lookup while maintaining a bounded size.
type revisionStore struct {
	revisions  map[RevisionID]*Revision
	maxEntries int
	oldestID   RevisionID
}

// newRevisionStore creates a new revision store with the given capacity.
func newRevisionStore(maxEntries int) *revisionStore {
	if maxEntries <= 0 {
		maxEntries = 100
	}
	return &revisionStore{
		revisions:  make(map[RevisionID]*Revision),
		maxEntries: maxEntries,
	}
}

// Add stores a revision, evicting old entries if necessary.
func (rs *revisionStore) Add(rev *Revision) {
	rs.revisions[rev.ID] = rev

	// Track oldest for cleanup
	if rs.oldestID == 0 || rev.ID < rs.oldestID {
		rs.oldestID = rev.ID
	}

	// Cleanup if over capacity
	rs.cleanup()
}

// Get retrieves a revision by ID.
func (rs *revisionStore) Get(id RevisionID) (*Revision, bool) {
	rev, ok := rs.revisions[id]
	return rev, ok
}

// Delete removes a revision by ID.
func (rs *revisionStore) Delete(id RevisionID) {
	delete(rs.revisions, id)
}

// cleanup removes oldest revisions to stay within capacity.
func (rs *revisionStore) cleanup() {
	if len(rs.revisions) <= rs.maxEntries {
		return
	}

	// Find and remove oldest entries
	for len(rs.revisions) > rs.maxEntries {
		// Find the actual oldest
		var oldest RevisionID
		for id := range rs.revisions {
			if oldest == 0 || id < oldest {
				oldest = id
			}
		}
		if oldest == 0 {
			break
		}
		delete(rs.revisions, oldest)
	}

	// Update oldest ID
	rs.oldestID = 0
	for id := range rs.revisions {
		if rs.oldestID == 0 || id < rs.oldestID {
			rs.oldestID = id
		}
	}
}

// Len returns the number of stored revisions.
func (rs *revisionStore) Len() int {
	return len(rs.revisions)
}

// Clear removes all revisions.
func (rs *revisionStore) Clear() {
	rs.revisions = make(map[RevisionID]*Revision)
	rs.oldestID = 0
}
