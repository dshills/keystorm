package index

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Common incremental indexer errors.
var (
	ErrIndexerStopped = errors.New("indexer is stopped")
	ErrIndexerBusy    = errors.New("indexer is busy")
)

// IncrementalIndexer manages background indexing with incremental updates.
type IncrementalIndexer struct {
	mu sync.RWMutex

	// Indexes
	fileIndex    Index
	contentIndex *ContentIndex

	// Configuration
	config IncrementalConfig

	// Worker pool
	workers int
	jobs    chan indexJob
	wg      sync.WaitGroup

	// State
	status   atomic.Int32 // IndexStatus
	progress IndexProgress

	// Control
	ctx    context.Context
	cancel context.CancelFunc

	// Event handlers
	handlers []func(IndexEvent)
}

// IndexStatus indicates the indexer state.
type IndexStatus int32

const (
	// IndexStatusIdle means the indexer is idle.
	IndexStatusIdle IndexStatus = iota
	// IndexStatusIndexing means the indexer is actively indexing.
	IndexStatusIndexing
	// IndexStatusError means the indexer encountered an error.
	IndexStatusError
	// IndexStatusStopped means the indexer is stopped.
	IndexStatusStopped
)

// String returns the string representation of the status.
func (s IndexStatus) String() string {
	switch s {
	case IndexStatusIdle:
		return "idle"
	case IndexStatusIndexing:
		return "indexing"
	case IndexStatusError:
		return "error"
	case IndexStatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// IndexProgress tracks indexing progress.
type IndexProgress struct {
	mu sync.RWMutex

	TotalFiles     int
	IndexedFiles   int
	ErrorFiles     int
	BytesProcessed int64
	StartTime      time.Time
	LastUpdateTime time.Time
	CurrentFile    string
}

// Copy returns a copy of the progress.
func (p *IndexProgress) Copy() IndexProgress {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return IndexProgress{
		TotalFiles:     p.TotalFiles,
		IndexedFiles:   p.IndexedFiles,
		ErrorFiles:     p.ErrorFiles,
		BytesProcessed: p.BytesProcessed,
		StartTime:      p.StartTime,
		LastUpdateTime: p.LastUpdateTime,
		CurrentFile:    p.CurrentFile,
	}
}

// PercentComplete returns the completion percentage.
func (p *IndexProgress) PercentComplete() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.TotalFiles == 0 {
		return 0
	}
	return float64(p.IndexedFiles) / float64(p.TotalFiles) * 100
}

// IncrementalConfig configures the incremental indexer.
type IncrementalConfig struct {
	// Workers is the number of parallel indexing workers
	Workers int

	// MaxFileSize is the maximum file size to index (bytes)
	MaxFileSize int64

	// ExcludePatterns are glob patterns to exclude
	ExcludePatterns []string

	// IncludePatterns are glob patterns to include (empty = all)
	IncludePatterns []string

	// IndexContent enables content indexing
	IndexContent bool

	// BatchSize is the number of files to process before emitting progress
	BatchSize int
}

// DefaultIncrementalConfig returns the default configuration.
func DefaultIncrementalConfig() IncrementalConfig {
	return IncrementalConfig{
		Workers:      4,
		MaxFileSize:  10 * 1024 * 1024, // 10MB
		IndexContent: true,
		BatchSize:    100,
		ExcludePatterns: []string{
			"**/.git/**",
			"**/node_modules/**",
			"**/vendor/**",
			"**/__pycache__/**",
			"**/dist/**",
			"**/build/**",
		},
	}
}

// indexJob represents a file to be indexed.
type indexJob struct {
	path string
	info os.FileInfo
}

// IndexEvent represents an indexing event.
type IndexEvent struct {
	Type     IndexEventType
	Path     string
	Error    error
	Progress IndexProgress
}

// IndexEventType indicates the type of indexing event.
type IndexEventType int

const (
	// IndexEventStarted is emitted when indexing starts.
	IndexEventStarted IndexEventType = iota
	// IndexEventProgress is emitted periodically during indexing.
	IndexEventProgress
	// IndexEventFileIndexed is emitted when a file is indexed.
	IndexEventFileIndexed
	// IndexEventFileError is emitted when a file fails to index.
	IndexEventFileError
	// IndexEventCompleted is emitted when indexing completes.
	IndexEventCompleted
	// IndexEventError is emitted when indexing fails.
	IndexEventError
)

// NewIncrementalIndexer creates a new incremental indexer.
func NewIncrementalIndexer(fileIndex Index, contentIndex *ContentIndex, config IncrementalConfig) *IncrementalIndexer {
	if config.Workers <= 0 {
		config.Workers = 4
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())

	ii := &IncrementalIndexer{
		fileIndex:    fileIndex,
		contentIndex: contentIndex,
		config:       config,
		workers:      config.Workers,
		jobs:         make(chan indexJob, config.Workers*10),
		ctx:          ctx,
		cancel:       cancel,
	}

	return ii
}

// Start begins background indexing for the given roots.
func (ii *IncrementalIndexer) Start(ctx context.Context, roots ...string) error {
	if !ii.status.CompareAndSwap(int32(IndexStatusIdle), int32(IndexStatusIndexing)) {
		current := IndexStatus(ii.status.Load())
		if current == IndexStatusStopped {
			return ErrIndexerStopped
		}
		return ErrIndexerBusy
	}

	// Reset progress
	ii.progress = IndexProgress{
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
	}

	// Emit start event
	ii.emitEvent(IndexEvent{
		Type:     IndexEventStarted,
		Progress: ii.progress.Copy(),
	})

	// Start workers
	for i := 0; i < ii.workers; i++ {
		ii.wg.Add(1)
		go ii.worker()
	}

	// Collect and process files
	go func() {
		defer func() {
			close(ii.jobs)
			ii.wg.Wait()

			// Set final status
			if IndexStatus(ii.status.Load()) == IndexStatusIndexing {
				ii.status.Store(int32(IndexStatusIdle))
			}

			// Emit completion event
			ii.emitEvent(IndexEvent{
				Type:     IndexEventCompleted,
				Progress: ii.progress.Copy(),
			})
		}()

		// Collect files from all roots
		for _, root := range roots {
			select {
			case <-ctx.Done():
				return
			case <-ii.ctx.Done():
				return
			default:
			}

			err := ii.collectFiles(ctx, root)
			if err != nil && err != context.Canceled {
				ii.emitEvent(IndexEvent{
					Type:  IndexEventError,
					Error: err,
				})
			}
		}
	}()

	return nil
}

// collectFiles walks a directory and queues files for indexing.
func (ii *IncrementalIndexer) collectFiles(ctx context.Context, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ii.ctx.Done():
			return ii.ctx.Err()
		default:
		}

		// Skip directories (but continue walking)
		if info.IsDir() {
			// Check if directory should be excluded
			if ii.shouldExclude(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check exclude patterns
		if ii.shouldExclude(path) {
			return nil
		}

		// Check file size
		if ii.config.MaxFileSize > 0 && info.Size() > ii.config.MaxFileSize {
			return nil
		}

		// Increment total count
		ii.progress.mu.Lock()
		ii.progress.TotalFiles++
		ii.progress.mu.Unlock()

		// Queue for indexing
		select {
		case ii.jobs <- indexJob{path: path, info: info}:
		case <-ctx.Done():
			return ctx.Err()
		case <-ii.ctx.Done():
			return ii.ctx.Err()
		}

		return nil
	})
}

// worker processes indexing jobs.
func (ii *IncrementalIndexer) worker() {
	defer ii.wg.Done()

	for job := range ii.jobs {
		select {
		case <-ii.ctx.Done():
			return
		default:
		}

		err := ii.indexFile(job.path, job.info)

		// Update progress
		ii.progress.mu.Lock()
		ii.progress.CurrentFile = job.path
		ii.progress.LastUpdateTime = time.Now()
		if err != nil {
			ii.progress.ErrorFiles++
			ii.emitEvent(IndexEvent{
				Type:  IndexEventFileError,
				Path:  job.path,
				Error: err,
			})
		} else {
			ii.progress.IndexedFiles++
			ii.progress.BytesProcessed += job.info.Size()
		}
		indexed := ii.progress.IndexedFiles
		ii.progress.mu.Unlock()

		// Emit progress event every batch
		if indexed%ii.config.BatchSize == 0 {
			ii.emitEvent(IndexEvent{
				Type:     IndexEventProgress,
				Progress: ii.progress.Copy(),
			})
		}
	}
}

// indexFile indexes a single file.
func (ii *IncrementalIndexer) indexFile(path string, info os.FileInfo) error {
	// Add to file index
	fileInfo := FileInfo{
		Path:    path,
		Name:    info.Name(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
		Mode:    info.Mode(),
	}

	if err := ii.fileIndex.Add(path, fileInfo); err != nil && err != ErrAlreadyExists {
		return err
	}

	// Index content if enabled
	if ii.config.IndexContent && ii.contentIndex != nil {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if err := ii.contentIndex.IndexDocument(path, content); err != nil {
			return err
		}
	}

	return nil
}

// shouldExclude checks if a path should be excluded.
func (ii *IncrementalIndexer) shouldExclude(path string) bool {
	// Normalize path
	path = filepath.ToSlash(path)

	for _, pattern := range ii.config.ExcludePatterns {
		if matchExcludePattern(pattern, path) {
			return true
		}
	}

	return false
}

// ProcessChange handles a file change event.
func (ii *IncrementalIndexer) ProcessChange(event FileChangeEvent) error {
	ii.mu.Lock()
	defer ii.mu.Unlock()

	switch event.Type {
	case FileChangeCreated, FileChangeModified:
		// Re-index the file
		info, err := os.Stat(event.Path)
		if err != nil {
			return err
		}

		return ii.indexFile(event.Path, info)

	case FileChangeDeleted:
		// Remove from indexes
		if err := ii.fileIndex.Remove(event.Path); err != nil && err != ErrNotFound {
			return err
		}

		if ii.contentIndex != nil {
			if err := ii.contentIndex.RemoveDocument(event.Path); err != nil {
				return err
			}
		}

	case FileChangeRenamed:
		// Remove old path
		if err := ii.fileIndex.Remove(event.OldPath); err != nil && err != ErrNotFound {
			return err
		}
		if ii.contentIndex != nil {
			_ = ii.contentIndex.RemoveDocument(event.OldPath)
		}

		// Index new path
		info, err := os.Stat(event.Path)
		if err != nil {
			return err
		}

		return ii.indexFile(event.Path, info)
	}

	return nil
}

// Rebuild forces a full reindex.
func (ii *IncrementalIndexer) Rebuild(ctx context.Context, roots ...string) error {
	// Clear existing indexes
	ii.fileIndex.Clear()
	if ii.contentIndex != nil {
		ii.contentIndex.Clear()
	}

	// Start fresh indexing
	return ii.Start(ctx, roots...)
}

// Stop stops the indexer.
func (ii *IncrementalIndexer) Stop() {
	ii.cancel()
	ii.status.Store(int32(IndexStatusStopped))
	ii.wg.Wait()
}

// Status returns the current indexer status.
func (ii *IncrementalIndexer) Status() IndexStatus {
	return IndexStatus(ii.status.Load())
}

// Progress returns the current indexing progress.
func (ii *IncrementalIndexer) Progress() IndexProgress {
	return ii.progress.Copy()
}

// OnEvent registers an event handler.
func (ii *IncrementalIndexer) OnEvent(handler func(IndexEvent)) {
	ii.mu.Lock()
	defer ii.mu.Unlock()

	ii.handlers = append(ii.handlers, handler)
}

// emitEvent sends an event to all handlers.
func (ii *IncrementalIndexer) emitEvent(event IndexEvent) {
	ii.mu.RLock()
	handlers := make([]func(IndexEvent), len(ii.handlers))
	copy(handlers, ii.handlers)
	ii.mu.RUnlock()

	for _, h := range handlers {
		h(event)
	}
}

// Save persists both indexes.
func (ii *IncrementalIndexer) Save(fileWriter, contentWriter io.Writer) error {
	if err := ii.fileIndex.Save(fileWriter); err != nil {
		return err
	}

	if ii.contentIndex != nil && contentWriter != nil {
		if err := ii.contentIndex.Save(contentWriter); err != nil {
			return err
		}
	}

	return nil
}

// Load restores both indexes.
func (ii *IncrementalIndexer) Load(fileReader, contentReader io.Reader) error {
	if err := ii.fileIndex.Load(fileReader); err != nil {
		return err
	}

	if ii.contentIndex != nil && contentReader != nil {
		if err := ii.contentIndex.Load(contentReader); err != nil {
			return err
		}
	}

	return nil
}

// FileChangeEvent represents a file system change.
type FileChangeEvent struct {
	Type      FileChangeType
	Path      string
	OldPath   string // For renames
	Timestamp time.Time
}

// FileChangeType indicates the type of file change.
type FileChangeType int

const (
	// FileChangeCreated indicates a file was created.
	FileChangeCreated FileChangeType = iota
	// FileChangeModified indicates a file was modified.
	FileChangeModified
	// FileChangeDeleted indicates a file was deleted.
	FileChangeDeleted
	// FileChangeRenamed indicates a file was renamed.
	FileChangeRenamed
)

// matchExcludePattern matches a path against an exclude pattern.
func matchExcludePattern(pattern, path string) bool {
	// Normalize to forward slashes
	pattern = filepath.ToSlash(pattern)

	// Handle ** patterns
	if containsDoublestar(pattern) {
		parts := splitOnDoublestar(pattern)
		for _, part := range parts {
			part = trimSlashes(part)
			if part != "" && containsPath(path, part) {
				return true
			}
		}
		return false
	}

	// Simple pattern matching
	matched, _ := filepath.Match(pattern, filepath.Base(path))
	return matched
}

// containsDoublestar checks if pattern contains **
func containsDoublestar(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '*' && s[i+1] == '*' {
			return true
		}
	}
	return false
}

// splitOnDoublestar splits a string on **
func splitOnDoublestar(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '*' && s[i+1] == '*' {
			parts = append(parts, s[start:i])
			start = i + 2
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// trimSlashes removes leading and trailing slashes
func trimSlashes(s string) string {
	for len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// containsPath checks if path contains the segment
func containsPath(path, segment string) bool {
	// Check for /segment/ or /segment at end or segment/ at start
	if segment == "" {
		return false
	}
	pathWithSlashes := "/" + path + "/"
	segmentWithSlashes := "/" + segment + "/"
	for i := 0; i <= len(pathWithSlashes)-len(segmentWithSlashes); i++ {
		if pathWithSlashes[i:i+len(segmentWithSlashes)] == segmentWithSlashes {
			return true
		}
	}
	return false
}
