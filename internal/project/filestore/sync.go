package filestore

import (
	"context"
	"sync"
	"time"
)

// SyncManager monitors file store for external changes and manages
// automatic synchronization between editor buffers and disk.
type SyncManager struct {
	mu       sync.Mutex
	store    *FileStore
	interval time.Duration
	running  bool
	stop     chan struct{}
	done     chan struct{}

	// Event handlers
	onExternalChange []func(doc *Document)
	onConflict       []func(doc *Document)
}

// NewSyncManager creates a new synchronization manager.
func NewSyncManager(store *FileStore, checkInterval time.Duration) *SyncManager {
	if checkInterval <= 0 {
		checkInterval = 2 * time.Second
	}
	return &SyncManager{
		store:    store,
		interval: checkInterval,
	}
}

// Start begins monitoring for external changes.
func (sm *SyncManager) Start() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.running {
		return
	}

	sm.running = true
	sm.stop = make(chan struct{})
	sm.done = make(chan struct{})

	go sm.monitorLoop()
}

// Stop stops monitoring for external changes.
func (sm *SyncManager) Stop() {
	sm.mu.Lock()
	if !sm.running {
		sm.mu.Unlock()
		return
	}
	sm.running = false
	close(sm.stop)
	sm.mu.Unlock()

	// Wait for the monitor loop to finish
	<-sm.done
}

// IsRunning returns true if the sync manager is running.
func (sm *SyncManager) IsRunning() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.running
}

// CheckNow performs an immediate check for external changes.
func (sm *SyncManager) CheckNow(ctx context.Context) {
	sm.checkExternalChanges(ctx)
}

// monitorLoop periodically checks for external file changes.
func (sm *SyncManager) monitorLoop() {
	defer close(sm.done)

	ticker := time.NewTicker(sm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.stop:
			return
		case <-ticker.C:
			sm.checkExternalChanges(context.Background())
		}
	}
}

// checkExternalChanges checks all open documents for external modifications.
func (sm *SyncManager) checkExternalChanges(ctx context.Context) {
	changed := sm.store.CheckExternalChanges()

	// Copy handler slices to avoid races during iteration
	sm.mu.Lock()
	conflictHandlers := make([]func(doc *Document), len(sm.onConflict))
	copy(conflictHandlers, sm.onConflict)
	externalHandlers := make([]func(doc *Document), len(sm.onExternalChange))
	copy(externalHandlers, sm.onExternalChange)
	sm.mu.Unlock()

	for _, doc := range changed {
		// Determine if there's a conflict (dirty document with external changes)
		if doc.IsDirty() {
			// Conflict: document has both local and external changes
			for _, handler := range conflictHandlers {
				handler(doc)
			}
		} else {
			// No conflict: just external changes
			for _, handler := range externalHandlers {
				handler(doc)
			}
		}
	}
}

// OnExternalChange registers a handler called when an external change is detected.
// This is called when a clean document has been modified externally.
func (sm *SyncManager) OnExternalChange(handler func(doc *Document)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onExternalChange = append(sm.onExternalChange, handler)
}

// OnConflict registers a handler called when a conflict is detected.
// A conflict occurs when a dirty document has also been modified externally.
func (sm *SyncManager) OnConflict(handler func(doc *Document)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onConflict = append(sm.onConflict, handler)
}

// AutoReloadPolicy determines how external changes should be handled.
type AutoReloadPolicy int

const (
	// AutoReloadNever never auto-reloads files.
	AutoReloadNever AutoReloadPolicy = iota

	// AutoReloadClean auto-reloads only clean (non-dirty) files.
	AutoReloadClean

	// AutoReloadPrompt prompts the user for all external changes.
	AutoReloadPrompt
)

// AutoReloadSyncManager extends SyncManager with automatic reload behavior.
type AutoReloadSyncManager struct {
	*SyncManager
	policy AutoReloadPolicy
}

// NewAutoReloadSyncManager creates a sync manager with auto-reload support.
func NewAutoReloadSyncManager(store *FileStore, checkInterval time.Duration, policy AutoReloadPolicy) *AutoReloadSyncManager {
	sm := &AutoReloadSyncManager{
		SyncManager: NewSyncManager(store, checkInterval),
		policy:      policy,
	}

	// Register handlers based on policy
	if policy == AutoReloadClean {
		sm.OnExternalChange(func(doc *Document) {
			// Auto-reload clean documents
			_ = store.Reload(context.Background(), doc.Path, false)
		})
	}

	return sm
}

// SetPolicy updates the auto-reload policy.
func (sm *AutoReloadSyncManager) SetPolicy(policy AutoReloadPolicy) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.policy = policy
}

// GetPolicy returns the current auto-reload policy.
func (sm *AutoReloadSyncManager) GetPolicy() AutoReloadPolicy {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.policy
}

// ConflictResolution represents how to resolve a conflict.
type ConflictResolution int

const (
	// ResolveKeepLocal keeps the local changes and ignores external changes.
	ResolveKeepLocal ConflictResolution = iota

	// ResolveUseExternal discards local changes and uses the external version.
	ResolveUseExternal

	// ResolveMerge attempts to merge local and external changes (future).
	ResolveMerge
)

// ResolveConflict resolves a conflict for a specific document.
func (sm *AutoReloadSyncManager) ResolveConflict(ctx context.Context, path string, resolution ConflictResolution) error {
	switch resolution {
	case ResolveKeepLocal:
		// Do nothing - keep local changes
		// The next save will overwrite the external changes
		return nil

	case ResolveUseExternal:
		// Force reload to use external changes
		return sm.store.Reload(ctx, path, true)

	case ResolveMerge:
		// Not implemented - would require diff/merge logic
		return sm.store.Reload(ctx, path, true)

	default:
		return nil
	}
}

// BatchSave saves multiple documents atomically (best effort).
// Returns the paths of documents that failed to save.
type BatchSaveResult struct {
	Saved  []string
	Failed map[string]error
}

// SaveAll saves all dirty documents.
func (sm *AutoReloadSyncManager) SaveAll(ctx context.Context) BatchSaveResult {
	result := BatchSaveResult{
		Failed: make(map[string]error),
	}

	dirty := sm.store.DirtyDocuments()
	for _, doc := range dirty {
		if err := sm.store.Save(ctx, doc.Path); err != nil {
			result.Failed[doc.Path] = err
		} else {
			result.Saved = append(result.Saved, doc.Path)
		}
	}

	return result
}

// SavePaths saves specific documents.
func (sm *AutoReloadSyncManager) SavePaths(ctx context.Context, paths []string) BatchSaveResult {
	result := BatchSaveResult{
		Failed: make(map[string]error),
	}

	for _, path := range paths {
		if err := sm.store.Save(ctx, path); err != nil {
			result.Failed[path] = err
		} else {
			result.Saved = append(result.Saved, path)
		}
	}

	return result
}

// WatchDocument sets up tracking for a specific document.
// This is useful when you want to monitor a specific file more closely.
type DocumentWatcher struct {
	doc        *Document
	lastCheck  time.Time
	checkCount int64
}

// NewDocumentWatcher creates a watcher for a specific document.
func NewDocumentWatcher(doc *Document) *DocumentWatcher {
	return &DocumentWatcher{
		doc:       doc,
		lastCheck: time.Now(),
	}
}

// Check checks if the document has external changes since last check.
func (dw *DocumentWatcher) Check(currentDiskModTime time.Time) bool {
	dw.lastCheck = time.Now()
	dw.checkCount++
	return dw.doc.HasExternalChanges(currentDiskModTime)
}

// Stats returns watcher statistics.
func (dw *DocumentWatcher) Stats() (lastCheck time.Time, checkCount int64) {
	return dw.lastCheck, dw.checkCount
}
