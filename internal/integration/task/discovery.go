// Package task provides task discovery and execution for Keystorm.
package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// maxConcurrentDiscovery limits the number of concurrent file discoveries.
var maxConcurrentDiscovery = runtime.GOMAXPROCS(0) * 2

// TaskType identifies the type of task.
type TaskType string

const (
	// TaskTypeShell is a shell command task.
	TaskTypeShell TaskType = "shell"
	// TaskTypeProcess is a process-based task.
	TaskTypeProcess TaskType = "process"
	// TaskTypeNPM is an npm script task.
	TaskTypeNPM TaskType = "npm"
	// TaskTypeMake is a make target task.
	TaskTypeMake TaskType = "make"
	// TaskTypeGo is a go command task.
	TaskTypeGo TaskType = "go"
)

// TaskGroup categorizes tasks.
type TaskGroup string

const (
	// TaskGroupBuild contains build-related tasks.
	TaskGroupBuild TaskGroup = "build"
	// TaskGroupTest contains test-related tasks.
	TaskGroupTest TaskGroup = "test"
	// TaskGroupRun contains run/start tasks.
	TaskGroupRun TaskGroup = "run"
	// TaskGroupClean contains cleanup tasks.
	TaskGroupClean TaskGroup = "clean"
	// TaskGroupLint contains linting tasks.
	TaskGroupLint TaskGroup = "lint"
	// TaskGroupOther contains uncategorized tasks.
	TaskGroupOther TaskGroup = "other"
)

// Task represents a discovered task that can be executed.
type Task struct {
	// ID is a unique identifier for the task.
	ID string `json:"id"`

	// Name is the display name of the task.
	Name string `json:"name"`

	// Description is a human-readable description.
	Description string `json:"description,omitempty"`

	// Source identifies where this task was discovered from.
	Source string `json:"source"`

	// SourceFile is the file path where the task was found.
	SourceFile string `json:"sourceFile,omitempty"`

	// Type is the task type.
	Type TaskType `json:"type"`

	// Group is the task category.
	Group TaskGroup `json:"group"`

	// Command is the command to execute.
	Command string `json:"command"`

	// Args are the command arguments.
	Args []string `json:"args,omitempty"`

	// Cwd is the working directory for the task.
	Cwd string `json:"cwd,omitempty"`

	// Env are environment variables for the task.
	Env map[string]string `json:"env,omitempty"`

	// DependsOn lists task IDs this task depends on.
	DependsOn []string `json:"dependsOn,omitempty"`

	// ProblemMatcher is the problem matcher pattern name.
	ProblemMatcher string `json:"problemMatcher,omitempty"`

	// IsDefault indicates this is a default task for its group.
	IsDefault bool `json:"isDefault,omitempty"`

	// RunOptions contains execution options.
	RunOptions *RunOptions `json:"runOptions,omitempty"`
}

// RunOptions contains task execution options.
type RunOptions struct {
	// InstanceLimit is the max concurrent instances (0 = unlimited).
	InstanceLimit int `json:"instanceLimit,omitempty"`

	// RunOn specifies when to run: "default", "folderOpen", "save".
	RunOn string `json:"runOn,omitempty"`

	// Reevaluate inputs on rerun.
	ReevaluateOnRerun bool `json:"reevaluateOnRerun,omitempty"`
}

// Source is a task source that can discover tasks from files or directories.
type Source interface {
	// Name returns the source name (e.g., "makefile", "npm", "taskfile").
	Name() string

	// Patterns returns glob patterns for files this source handles.
	Patterns() []string

	// Priority returns the source priority (higher = more important).
	Priority() int

	// Discover finds tasks in the given file.
	Discover(ctx context.Context, path string) ([]*Task, error)
}

// DiscoveryOptions configures task discovery.
type DiscoveryOptions struct {
	// RootDir is the root directory to search from.
	RootDir string

	// MaxDepth is the maximum directory depth to search (0 = root only).
	MaxDepth int

	// ExcludeDirs are directory names to exclude.
	ExcludeDirs []string

	// Sources are the sources to use (nil = all registered).
	Sources []string

	// Timeout is the discovery timeout.
	Timeout time.Duration
}

// DefaultDiscoveryOptions returns sensible default options.
func DefaultDiscoveryOptions(rootDir string) DiscoveryOptions {
	return DiscoveryOptions{
		RootDir:  rootDir,
		MaxDepth: 3,
		ExcludeDirs: []string{
			"node_modules",
			".git",
			"vendor",
			".venv",
			"__pycache__",
			"dist",
			"build",
			".cache",
		},
		Timeout: 30 * time.Second,
	}
}

// Discovery manages task discovery from multiple sources.
type Discovery struct {
	mu      sync.RWMutex
	sources map[string]Source

	// Cache of discovered tasks by root directory.
	cache      map[string]*DiscoveryResult
	cacheMu    sync.RWMutex
	cacheTime  time.Duration
	lastUpdate map[string]time.Time
}

// DiscoveryResult contains the results of task discovery.
type DiscoveryResult struct {
	// Tasks is the list of discovered tasks.
	Tasks []*Task

	// BySource groups tasks by source name.
	BySource map[string][]*Task

	// ByGroup groups tasks by task group.
	ByGroup map[TaskGroup][]*Task

	// Errors contains any errors during discovery.
	Errors []DiscoveryError

	// Duration is how long discovery took.
	Duration time.Duration

	// Timestamp is when discovery completed.
	Timestamp time.Time
}

// DiscoveryError represents an error during discovery.
type DiscoveryError struct {
	Source string
	File   string
	Err    error
}

func (e DiscoveryError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s: %s: %v", e.Source, e.File, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Source, e.Err)
}

// NewDiscovery creates a new task discovery manager.
func NewDiscovery() *Discovery {
	return &Discovery{
		sources:    make(map[string]Source),
		cache:      make(map[string]*DiscoveryResult),
		cacheTime:  5 * time.Minute,
		lastUpdate: make(map[string]time.Time),
	}
}

// RegisterSource registers a task source.
func (d *Discovery) RegisterSource(source Source) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sources[source.Name()] = source
}

// UnregisterSource removes a task source.
func (d *Discovery) UnregisterSource(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.sources, name)
}

// GetSource returns a registered source by name.
func (d *Discovery) GetSource(name string) (Source, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	source, ok := d.sources[name]
	return source, ok
}

// Sources returns all registered source names.
func (d *Discovery) Sources() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	names := make([]string, 0, len(d.sources))
	for name := range d.sources {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SetCacheTime sets the cache duration.
func (d *Discovery) SetCacheTime(duration time.Duration) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	d.cacheTime = duration
}

// ClearCache clears the discovery cache.
func (d *Discovery) ClearCache() {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	d.cache = make(map[string]*DiscoveryResult)
	d.lastUpdate = make(map[string]time.Time)
}

// ClearCacheFor clears the cache for a specific root directory.
func (d *Discovery) ClearCacheFor(rootDir string) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	delete(d.cache, rootDir)
	delete(d.lastUpdate, rootDir)
}

// Discover finds tasks in the given root directory.
func (d *Discovery) Discover(ctx context.Context, opts DiscoveryOptions) (*DiscoveryResult, error) {
	// Check cache first
	if result := d.getCached(opts.RootDir); result != nil {
		return result, nil
	}

	// Apply timeout
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	start := time.Now()

	// Get sources to use
	sources := d.getSourcesFor(opts.Sources)
	if len(sources) == 0 {
		return &DiscoveryResult{
			Tasks:     []*Task{},
			BySource:  make(map[string][]*Task),
			ByGroup:   make(map[TaskGroup][]*Task),
			Duration:  time.Since(start),
			Timestamp: time.Now(),
		}, nil
	}

	// Build pattern to source mapping
	patternMap := make(map[string][]Source)
	for _, src := range sources {
		for _, pattern := range src.Patterns() {
			patternMap[pattern] = append(patternMap[pattern], src)
		}
	}

	// Find matching files
	files, err := d.findFiles(opts, patternMap)
	if err != nil {
		return nil, fmt.Errorf("find files: %w", err)
	}

	// Discover tasks from each file
	result := &DiscoveryResult{
		Tasks:    make([]*Task, 0),
		BySource: make(map[string][]*Task),
		ByGroup:  make(map[TaskGroup][]*Task),
		Errors:   make([]DiscoveryError, 0),
	}

	var wg sync.WaitGroup
	var resultMu sync.Mutex

	// Use semaphore to limit concurrent discoveries
	sem := make(chan struct{}, maxConcurrentDiscovery)

	for filePath, fileSources := range files {
		// Check context before spawning goroutine
		select {
		case <-ctx.Done():
			break
		default:
		}

		// Sort sources by priority
		sort.Slice(fileSources, func(i, j int) bool {
			return fileSources[i].Priority() > fileSources[j].Priority()
		})

		// Use highest priority source for this file
		// Capture loop variables for goroutine
		src := fileSources[0]
		file := filePath

		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			tasks, err := src.Discover(ctx, file)
			resultMu.Lock()
			defer resultMu.Unlock()

			if err != nil {
				result.Errors = append(result.Errors, DiscoveryError{
					Source: src.Name(),
					File:   file,
					Err:    err,
				})
				return
			}

			for _, task := range tasks {
				// Ensure task has an ID
				if task.ID == "" {
					task.ID = generateTaskID(opts.RootDir, src.Name(), file, task.Name)
				}

				// Set source info
				task.Source = src.Name()
				if task.SourceFile == "" {
					task.SourceFile = file
				}

				// Set default working directory
				if task.Cwd == "" {
					task.Cwd = filepath.Dir(file)
				}

				result.Tasks = append(result.Tasks, task)
				result.BySource[src.Name()] = append(result.BySource[src.Name()], task)
				result.ByGroup[task.Group] = append(result.ByGroup[task.Group], task)
			}
		}()
	}

	wg.Wait()

	// Sort tasks by name
	sort.Slice(result.Tasks, func(i, j int) bool {
		return result.Tasks[i].Name < result.Tasks[j].Name
	})

	result.Duration = time.Since(start)
	result.Timestamp = time.Now()

	// Cache the result
	d.setCache(opts.RootDir, result)

	return result, nil
}

// getSourcesFor returns the sources to use for discovery.
func (d *Discovery) getSourcesFor(names []string) []Source {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(names) == 0 {
		// Return all sources
		sources := make([]Source, 0, len(d.sources))
		for _, src := range d.sources {
			sources = append(sources, src)
		}
		return sources
	}

	// Return only named sources
	sources := make([]Source, 0, len(names))
	for _, name := range names {
		if src, ok := d.sources[name]; ok {
			sources = append(sources, src)
		}
	}
	return sources
}

// findFiles finds files matching the source patterns.
func (d *Discovery) findFiles(opts DiscoveryOptions, patternMap map[string][]Source) (map[string][]Source, error) {
	result := make(map[string][]Source)

	// Create exclude set
	excludeSet := make(map[string]bool)
	for _, dir := range opts.ExcludeDirs {
		excludeSet[dir] = true
	}

	// Walk the directory tree
	err := walkDir(opts.RootDir, opts.MaxDepth, excludeSet, func(path string, depth int) error {
		name := filepath.Base(path)
		for pattern, sources := range patternMap {
			matched, err := filepath.Match(pattern, name)
			if err != nil {
				continue
			}
			if matched {
				result[path] = append(result[path], sources...)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// walkDir walks a directory tree up to maxDepth.
func walkDir(root string, maxDepth int, excludeSet map[string]bool, fn func(path string, depth int) error) error {
	visited := make(map[string]bool)

	// Mark root directory as visited using its real path
	rootReal, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		// If we can't resolve the root, use cleaned path
		rootReal = filepath.Clean(root)
	}
	visited[rootReal] = true

	return walkDirRecursive(root, 0, maxDepth, excludeSet, visited, fn)
}

func walkDirRecursive(dir string, depth, maxDepth int, excludeSet, visited map[string]bool, fn func(path string, depth int) error) error {
	// Use os.ReadDir to include hidden directories (dot-prefixed)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		entryPath := filepath.Join(dir, name)

		if entry.IsDir() {
			// Skip excluded directories
			if excludeSet[name] {
				continue
			}

			// Recurse if within depth limit
			if depth < maxDepth {
				// Get real path to detect symlink cycles
				realPath, err := filepath.EvalSymlinks(entryPath)
				if err != nil {
					// If we can't resolve the symlink, skip it
					continue
				}

				// Detect cycles by checking if we've visited this path
				if visited[realPath] {
					continue
				}
				visited[realPath] = true

				if err := walkDirRecursive(entryPath, depth+1, maxDepth, excludeSet, visited, fn); err != nil {
					return err
				}
			}
		} else {
			// Process file
			if err := fn(entryPath, depth); err != nil {
				return err
			}
		}
	}

	return nil
}

// getCached returns a cached result if valid.
func (d *Discovery) getCached(rootDir string) *DiscoveryResult {
	d.cacheMu.RLock()
	defer d.cacheMu.RUnlock()

	result, ok := d.cache[rootDir]
	if !ok {
		return nil
	}

	lastUpdate, ok := d.lastUpdate[rootDir]
	if !ok {
		return nil
	}

	if time.Since(lastUpdate) > d.cacheTime {
		return nil
	}

	return result
}

// setCache stores a result in the cache.
func (d *Discovery) setCache(rootDir string, result *DiscoveryResult) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()

	d.cache[rootDir] = result
	d.lastUpdate[rootDir] = time.Now()
}

// generateTaskID generates a unique task ID.
func generateTaskID(rootDir, source, file, name string) string {
	// Use relative path from root to ensure uniqueness across directories
	relPath, err := filepath.Rel(rootDir, file)
	if err != nil {
		// Fall back to hash of full path if relative path fails
		h := sha256.Sum256([]byte(file))
		relPath = hex.EncodeToString(h[:8])
	}
	return fmt.Sprintf("%s:%s:%s", source, relPath, name)
}

// InferGroup infers the task group from the task name.
func InferGroup(name string) TaskGroup {
	// Common patterns for task groups
	buildPatterns := []string{"build", "compile", "package", "bundle", "webpack", "rollup", "esbuild"}
	testPatterns := []string{"test", "spec", "check", "verify", "coverage"}
	runPatterns := []string{"run", "start", "serve", "dev", "watch", "develop"}
	cleanPatterns := []string{"clean", "clear", "purge", "reset"}
	lintPatterns := []string{"lint", "format", "fmt", "prettier", "eslint", "golangci"}

	lowerName := strings.ToLower(name)

	for _, pattern := range buildPatterns {
		if strings.Contains(lowerName, pattern) {
			return TaskGroupBuild
		}
	}

	for _, pattern := range testPatterns {
		if strings.Contains(lowerName, pattern) {
			return TaskGroupTest
		}
	}

	for _, pattern := range runPatterns {
		if strings.Contains(lowerName, pattern) {
			return TaskGroupRun
		}
	}

	for _, pattern := range cleanPatterns {
		if strings.Contains(lowerName, pattern) {
			return TaskGroupClean
		}
	}

	for _, pattern := range lintPatterns {
		if strings.Contains(lowerName, pattern) {
			return TaskGroupLint
		}
	}

	return TaskGroupOther
}

// statFile returns file info, following symlinks.
func statFile(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
