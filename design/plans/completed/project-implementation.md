# Keystorm File/Project Model - Implementation Plan

## Comprehensive Design Document for `internal/project`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Requirements Analysis](#2-requirements-analysis)
3. [Architecture Overview](#3-architecture-overview)
4. [Package Structure](#4-package-structure)
5. [Core Types and Interfaces](#5-core-types-and-interfaces)
6. [File System Abstraction](#6-file-system-abstraction)
7. [Project Graph](#7-project-graph)
8. [File Watching](#8-file-watching)
9. [Indexing and Search](#9-indexing-and-search)
10. [Workspace Management](#10-workspace-management)
11. [Integration with Editor](#11-integration-with-editor)
12. [Implementation Phases](#12-implementation-phases)
13. [Testing Strategy](#13-testing-strategy)
14. [Performance Considerations](#14-performance-considerations)

---

## 1. Executive Summary

The File/Project Model provides Keystorm with workspace awareness, file management, and project intelligence. Unlike simple editors that treat files individually, Keystorm models the project as a graph of interconnected nodes (files, modules, services, tests, APIs) with edges representing relationships (imports, calls, ownership).

### Role in the Architecture

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   Dispatcher    │─────▶│  Project Model  │◀────▶│  File System    │
│   (file ops)    │      │ (internal/proj) │      │  (OS/VFS)       │
└─────────────────┘      └────────┬────────┘      └─────────────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    ▼             ▼             ▼
            ┌───────────┐  ┌───────────┐  ┌───────────┐
            │    LSP    │  │  Context  │  │   Event   │
            │ Workspace │  │  Engine   │  │    Bus    │
            └───────────┘  └───────────┘  └───────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Graph-based model | AI context requires understanding relationships, not just file contents |
| fsnotify for watching | Cross-platform, well-maintained, Go-native file watching |
| Concurrent indexing | Large projects need parallel processing for reasonable startup times |
| Virtual FS abstraction | Enables testing, remote files, and in-memory operations |
| Incremental updates | Only reindex changed files, not entire project on every change |
| Language-agnostic core | Specific language intelligence comes from LSP, not project model |

### Integration Points

The Project Model connects to:
- **Dispatcher**: Receives file/project actions (open, save, search, goto-file)
- **LSP**: Provides workspace folders, file watchers, document URIs
- **Context Engine**: Supplies project graph for AI prompt building
- **Engine**: Notifies about buffer ↔ file synchronization
- **Event Bus**: Publishes file/project events (change, add, delete)
- **Config System**: Reads workspace/project settings
- **Plugin API**: Implements `api.ProjectProvider` interface

---

## 2. Requirements Analysis

### 2.1 From Architecture Specification

From `design/specs/architecture.md`:

> "Editors need to keep track of:
> - open files
> - file watchers
> - folders/workspaces
> - indexing
> - search"

> "Instead of 'just files in a folder', model the project as a graph:
> - Nodes: files, modules, services, tests, APIs, DB schemas, configs
> - Edges: imports, calls, ownership, 'this test targets that code', etc."

> "This graph becomes the main prompt substrate for the AI layer."

### 2.2 Functional Requirements

1. **Workspace Management**
   - Support single-folder and multi-root workspaces
   - Track workspace configuration (.keystorm/, .vscode/ compatibility)
   - Handle workspace open/close lifecycle

2. **File Operations**
   - Open files from disk to buffer
   - Save buffers to disk
   - Create, delete, rename files and directories
   - Handle encoding detection and conversion

3. **File Watching**
   - Detect external file changes
   - Debounce rapid changes
   - Support ignore patterns (.gitignore, .keystormignore)
   - Handle watch limit exhaustion gracefully

4. **Indexing**
   - Build file index for fast lookup
   - Index file contents for search
   - Extract structural information (symbols, imports)
   - Background incremental updates

5. **Search**
   - Fast file name/path search (fuzzy matching)
   - Full-text content search (ripgrep-like)
   - Symbol search (leveraging LSP when available)
   - Filter by file type, path pattern

6. **Project Graph**
   - Track file relationships (imports, references)
   - Identify module/package boundaries
   - Map tests to implementation files
   - Support "find related files" queries

### 2.3 Non-Functional Requirements

1. **Performance**
   - Sub-100ms file open for typical files
   - Sub-200ms fuzzy file search on 100k file projects
   - Background indexing (no UI blocking)
   - Minimal memory footprint for large projects

2. **Reliability**
   - Handle file system errors gracefully
   - Recover from watch failures
   - Persist index for fast startup

3. **Scalability**
   - Support projects with 100k+ files
   - Handle deeply nested directory structures
   - Work with network file systems (slower, less reliable)

### 2.4 Plugin API Interface

```go
// ProjectProvider defines the interface for project operations.
// Plugins use this to access workspace and file information.
type ProjectProvider interface {
    // Workspace
    WorkspaceRoot() string
    WorkspaceFolders() []string

    // File operations
    OpenFile(path string) ([]byte, error)
    SaveFile(path string, content []byte) error
    FileExists(path string) bool
    ListDirectory(path string) ([]FileInfo, error)

    // Search
    FindFiles(pattern string, limit int) ([]string, error)
    SearchContent(query string, opts SearchOptions) ([]SearchResult, error)

    // Project graph
    RelatedFiles(path string) ([]RelatedFile, error)
    FileImports(path string) ([]string, error)
    FileImportedBy(path string) ([]string, error)

    // Events
    OnFileChange(handler func(event FileChangeEvent))
}
```

---

## 3. Architecture Overview

### 3.1 Component Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Project Model                                 │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │                      Workspace Manager                          ││
│  │  - Multi-root workspace support                                 ││
│  │  - Configuration loading                                        ││
│  │  - Workspace lifecycle                                          ││
│  └─────────────────────────────────────────────────────────────────┘│
│                              │                                       │
│           ┌──────────────────┼──────────────────┐                   │
│           ▼                  ▼                  ▼                   │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐             │
│  │   FileStore │    │  Watcher    │    │   Indexer   │             │
│  │  - Open docs│    │  - fsnotify │    │  - Content  │             │
│  │  - Dirty    │    │  - Debounce │    │  - Symbols  │             │
│  │  - Encoding │    │  - Ignore   │    │  - Graph    │             │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘             │
│         │                  │                  │                     │
│  ┌──────┴──────────────────┴──────────────────┴──────┐             │
│  │                  Virtual File System              │             │
│  │  - OS file system                                 │             │
│  │  - In-memory (for testing)                        │             │
│  │  - Remote (future: SSH, cloud)                    │             │
│  └───────────────────────────────────────────────────┘             │
│                              │                                       │
│  ┌───────────────────────────┴───────────────────────┐             │
│  │                   Project Graph                    │             │
│  │  - Nodes: files, modules, packages                │             │
│  │  - Edges: imports, tests, ownership               │             │
│  │  - Queries: related, dependents, dependencies     │             │
│  └───────────────────────────────────────────────────┘             │
│                              │                                       │
│  ┌───────────────────────────┴───────────────────────┐             │
│  │                     Search Engine                  │             │
│  │  - Fuzzy file matching                            │             │
│  │  - Full-text search                               │             │
│  │  - Symbol search (via LSP)                        │             │
│  └───────────────────────────────────────────────────┘             │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 Data Flow

**File Open Flow:**
```
1. User invokes file.open action
2. Dispatcher calls Project.OpenFile(path)
3. FileStore checks if already open
4. If not, VFS reads file from disk
5. Encoding is detected/converted
6. Engine creates buffer with content
7. FileStore tracks document state
8. Watcher is set up for external changes
9. Event bus emits file.opened event
```

**External Change Detection Flow:**
```
1. fsnotify detects file change
2. Watcher debounces rapid changes
3. Watcher checks ignore patterns
4. If file is open in editor:
   a. Compare with buffer content
   b. If different, emit file.externalChange event
   c. UI prompts user: reload or keep
5. Indexer updates index in background
6. Project graph updates relationships
```

**Search Flow:**
```
1. User types in file finder
2. Dispatcher calls Project.FindFiles(query)
3. Search engine applies fuzzy matching to file index
4. Results ranked by score, recency, frequency
5. Top N results returned
6. User selects file → file.open action
```

---

## 4. Package Structure

```
internal/project/
├── doc.go                  # Package documentation
├── errors.go               # Error types
├── project.go              # Main Project type and interface
│
├── workspace/              # Workspace management
│   ├── workspace.go        # Workspace type
│   ├── config.go           # Workspace configuration
│   └── multiroot.go        # Multi-root workspace support
│
├── vfs/                    # Virtual file system abstraction
│   ├── vfs.go              # VFS interface
│   ├── os.go               # OS file system implementation
│   ├── memory.go           # In-memory implementation (testing)
│   └── encoding.go         # Encoding detection/conversion
│
├── watcher/                # File watching
│   ├── watcher.go          # Watcher interface and factory
│   ├── fsnotify.go         # fsnotify-based implementation
│   ├── debounce.go         # Change debouncing
│   └── ignore.go           # Ignore pattern matching
│
├── index/                  # File indexing
│   ├── index.go            # Index interface
│   ├── fileindex.go        # File path/name index
│   ├── contentindex.go     # Full-text content index
│   └── persist.go          # Index persistence
│
├── graph/                  # Project graph
│   ├── graph.go            # Graph interface and types
│   ├── node.go             # Node types (file, module, etc.)
│   ├── edge.go             # Edge types (import, test, etc.)
│   ├── builder.go          # Graph construction
│   └── query.go            # Graph queries
│
├── search/                 # Search functionality
│   ├── search.go           # Search interface
│   ├── fuzzy.go            # Fuzzy file matching
│   ├── content.go          # Full-text content search
│   └── ranking.go          # Result ranking
│
├── filestore/              # Open file tracking
│   ├── store.go            # FileStore type
│   ├── document.go         # Open document state
│   └── sync.go             # Buffer ↔ disk sync
│
└── provider.go             # Plugin API implementation
```

---

## 5. Core Types and Interfaces

### 5.1 Project Interface

```go
// Project is the main interface for workspace and file operations.
// It provides a unified API for file management, search, and project intelligence.
type Project interface {
    // Lifecycle
    Open(ctx context.Context, roots ...string) error
    Close(ctx context.Context) error

    // Workspace info
    Root() string
    Roots() []string
    IsInWorkspace(path string) bool

    // File operations
    OpenFile(ctx context.Context, path string) (*Document, error)
    SaveFile(ctx context.Context, path string, content []byte) error
    CloseFile(ctx context.Context, path string) error
    CreateFile(ctx context.Context, path string, content []byte) error
    DeleteFile(ctx context.Context, path string) error
    RenameFile(ctx context.Context, oldPath, newPath string) error

    // Directory operations
    CreateDirectory(ctx context.Context, path string) error
    DeleteDirectory(ctx context.Context, path string, recursive bool) error
    ListDirectory(ctx context.Context, path string) ([]FileInfo, error)

    // Search
    FindFiles(ctx context.Context, query string, opts FindOptions) ([]FileMatch, error)
    SearchContent(ctx context.Context, query string, opts SearchOptions) ([]ContentMatch, error)

    // Project graph
    Graph() Graph
    RelatedFiles(ctx context.Context, path string) ([]RelatedFile, error)

    // Open documents
    OpenDocuments() []*Document
    GetDocument(path string) (*Document, bool)
    IsDirty(path string) bool

    // Events
    OnFileChange(handler func(FileChangeEvent))
    OnWorkspaceChange(handler func(WorkspaceChangeEvent))

    // Status
    IndexStatus() IndexStatus
    WatcherStatus() WatcherStatus
}
```

### 5.2 Workspace Types

```go
// Workspace represents a collection of folders being edited.
type Workspace struct {
    mu     sync.RWMutex
    roots  []WorkspaceFolder
    config *WorkspaceConfig
}

// WorkspaceFolder represents a single folder in the workspace.
type WorkspaceFolder struct {
    // URI is the folder path as a URI (file://)
    URI string
    // Path is the local file system path
    Path string
    // Name is the display name for the folder
    Name string
}

// WorkspaceConfig holds workspace-level configuration.
type WorkspaceConfig struct {
    // ExcludePatterns are glob patterns to exclude from indexing/watching
    ExcludePatterns []string
    // SearchExcludePatterns are additional excludes for search only
    SearchExcludePatterns []string
    // WatcherExcludePatterns are additional excludes for file watching
    WatcherExcludePatterns []string
    // MaxFileSize is the maximum file size to index (bytes)
    MaxFileSize int64
    // IndexingConcurrency is the number of parallel indexing workers
    IndexingConcurrency int
}
```

### 5.3 Document Types

```go
// Document represents an open file with its current state.
type Document struct {
    mu sync.RWMutex

    // Identity
    Path       string
    URI        string

    // Content state
    Content    []byte
    Encoding   Encoding
    LineEnding LineEnding

    // Sync state
    DiskVersion    int64     // Modification time when last synced
    BufferVersion  int       // Buffer version number
    Dirty          bool      // Has unsaved changes
    LastSyncTime   time.Time

    // Metadata
    Language   string    // Detected or configured language ID
    Size       int64     // File size in bytes
    ReadOnly   bool      // File is read-only
}

// Encoding represents a character encoding.
type Encoding string

const (
    EncodingUTF8    Encoding = "utf-8"
    EncodingUTF16LE Encoding = "utf-16le"
    EncodingUTF16BE Encoding = "utf-16be"
    EncodingLatin1  Encoding = "iso-8859-1"
    // ... more encodings
)

// LineEnding represents the line ending style.
type LineEnding string

const (
    LineEndingLF   LineEnding = "lf"   // Unix: \n
    LineEndingCRLF LineEnding = "crlf" // Windows: \r\n
    LineEndingCR   LineEnding = "cr"   // Old Mac: \r
)
```

### 5.4 File Info Types

```go
// FileInfo represents information about a file or directory.
type FileInfo struct {
    Path       string
    Name       string
    Size       int64
    ModTime    time.Time
    IsDir      bool
    IsSymlink  bool
    Mode       os.FileMode
}

// FileChangeEvent represents a file system change.
type FileChangeEvent struct {
    Type      FileChangeType
    Path      string
    OldPath   string  // For renames
    Timestamp time.Time
}

// FileChangeType indicates the type of file change.
type FileChangeType int

const (
    FileChangeCreated FileChangeType = iota
    FileChangeModified
    FileChangeDeleted
    FileChangeRenamed
)
```

---

## 6. File System Abstraction

### 6.1 VFS Interface

```go
// VFS is a virtual file system abstraction.
// It allows swapping the underlying file system for testing or remote access.
type VFS interface {
    // Read operations
    Open(path string) (io.ReadCloser, error)
    ReadFile(path string) ([]byte, error)
    Stat(path string) (FileInfo, error)
    ReadDir(path string) ([]FileInfo, error)

    // Write operations
    WriteFile(path string, data []byte, perm os.FileMode) error
    Create(path string) (io.WriteCloser, error)
    Mkdir(path string, perm os.FileMode) error
    MkdirAll(path string, perm os.FileMode) error
    Remove(path string) error
    RemoveAll(path string) error
    Rename(oldPath, newPath string) error

    // Path operations
    Abs(path string) (string, error)
    Rel(basePath, targetPath string) (string, error)
    Join(elem ...string) string
    Dir(path string) string
    Base(path string) string
    Ext(path string) string

    // Queries
    Exists(path string) bool
    IsDir(path string) bool
    Glob(pattern string) ([]string, error)
    Walk(root string, fn WalkFunc) error
}

// WalkFunc is the function type for VFS.Walk.
type WalkFunc func(path string, info FileInfo, err error) error
```

### 6.2 OS File System Implementation

```go
// OSFS implements VFS using the operating system's file system.
type OSFS struct {
    // root restricts operations to a subtree (optional)
    root string
}

// NewOSFS creates a new OS file system.
func NewOSFS(root string) *OSFS {
    return &OSFS{root: root}
}
```

### 6.3 Memory File System Implementation

```go
// MemFS implements VFS using an in-memory file system.
// Used primarily for testing.
type MemFS struct {
    mu    sync.RWMutex
    files map[string]*memFile
    dirs  map[string]bool
}

type memFile struct {
    content []byte
    mode    os.FileMode
    modTime time.Time
}

// NewMemFS creates a new in-memory file system.
func NewMemFS() *MemFS {
    return &MemFS{
        files: make(map[string]*memFile),
        dirs:  make(map[string]bool),
    }
}
```

### 6.4 Encoding Detection

```go
// DetectEncoding attempts to detect the encoding of file content.
func DetectEncoding(content []byte) Encoding {
    // Check for BOM
    if bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}) {
        return EncodingUTF8
    }
    if bytes.HasPrefix(content, []byte{0xFF, 0xFE}) {
        return EncodingUTF16LE
    }
    if bytes.HasPrefix(content, []byte{0xFE, 0xFF}) {
        return EncodingUTF16BE
    }

    // Try UTF-8 validation
    if utf8.Valid(content) {
        return EncodingUTF8
    }

    // Fall back to Latin-1 (accepts all bytes)
    return EncodingLatin1
}

// DetectLineEnding detects the dominant line ending in content.
func DetectLineEnding(content []byte) LineEnding {
    crlf := bytes.Count(content, []byte{'\r', '\n'})
    lf := bytes.Count(content, []byte{'\n'}) - crlf
    cr := bytes.Count(content, []byte{'\r'}) - crlf

    if crlf >= lf && crlf >= cr {
        return LineEndingCRLF
    }
    if cr > lf {
        return LineEndingCR
    }
    return LineEndingLF
}
```

---

## 7. Project Graph

### 7.1 Graph Interface

```go
// Graph represents the project's structural relationships.
type Graph interface {
    // Node operations
    AddNode(node Node) error
    RemoveNode(id NodeID) error
    GetNode(id NodeID) (Node, bool)
    NodeCount() int

    // Edge operations
    AddEdge(edge Edge) error
    RemoveEdge(from, to NodeID, edgeType EdgeType) error
    GetEdges(from NodeID) []Edge
    GetReverseEdges(to NodeID) []Edge
    EdgeCount() int

    // Queries
    Dependencies(id NodeID) []Node
    Dependents(id NodeID) []Node
    RelatedNodes(id NodeID, maxDegree int) []Node
    FindPath(from, to NodeID) []Node

    // Batch operations
    Clear()
    Rebuild(ctx context.Context) error

    // Serialization
    Save(w io.Writer) error
    Load(r io.Reader) error
}
```

### 7.2 Node Types

```go
// NodeID uniquely identifies a node in the graph.
type NodeID string

// NodeType indicates the kind of node.
type NodeType int

const (
    NodeTypeFile NodeType = iota
    NodeTypeDirectory
    NodeTypeModule      // Go module, npm package, etc.
    NodeTypePackage     // Go package, Python module, etc.
    NodeTypeClass
    NodeTypeFunction
    NodeTypeTest
    NodeTypeConfig
    NodeTypeAPI         // API endpoint definition
    NodeTypeSchema      // Database schema, protobuf, etc.
)

// Node represents an entity in the project graph.
type Node struct {
    ID         NodeID
    Type       NodeType
    Path       string    // File path (for file-based nodes)
    Name       string    // Display name
    Language   string    // Programming language
    Metadata   NodeMeta  // Type-specific metadata
}

// NodeMeta holds type-specific node metadata.
type NodeMeta struct {
    // For modules/packages
    ModulePath string   // e.g., "github.com/user/repo"
    Version    string

    // For files
    Size       int64
    ModTime    time.Time

    // For functions/classes
    Signature  string
    StartLine  int
    EndLine    int

    // For tests
    TestTarget string   // Path to tested file

    // Custom metadata
    Extra      map[string]any
}
```

### 7.3 Edge Types

```go
// EdgeType indicates the relationship type.
type EdgeType int

const (
    EdgeTypeImports EdgeType = iota  // File A imports file B
    EdgeTypeExports                   // Module exports symbol
    EdgeTypeCalls                     // Function A calls function B
    EdgeTypeExtends                   // Class A extends class B
    EdgeTypeImplements                // Class implements interface
    EdgeTypeTests                     // Test file tests implementation
    EdgeTypeContains                  // Directory contains file
    EdgeTypeDependsOn                 // Module depends on module
    EdgeTypeReferences                // Generic reference
)

// Edge represents a relationship between nodes.
type Edge struct {
    From     NodeID
    To       NodeID
    Type     EdgeType
    Weight   float64   // Relationship strength (for ranking)
    Metadata EdgeMeta
}

// EdgeMeta holds edge-specific metadata.
type EdgeMeta struct {
    // For imports
    ImportPath string
    Symbols    []string  // Imported symbols

    // For calls
    CallSites  []Location

    // Custom metadata
    Extra      map[string]any
}
```

### 7.4 Graph Builder

```go
// GraphBuilder constructs the project graph from source files.
type GraphBuilder struct {
    graph      Graph
    vfs        VFS
    parsers    map[string]LanguageParser
    workers    int
}

// LanguageParser extracts graph information from source files.
type LanguageParser interface {
    // Language returns the language ID (e.g., "go", "typescript")
    Language() string

    // FileExtensions returns supported file extensions
    FileExtensions() []string

    // Parse extracts nodes and edges from a file
    Parse(ctx context.Context, path string, content []byte) (*ParseResult, error)
}

// ParseResult contains extracted graph information.
type ParseResult struct {
    Nodes []Node
    Edges []Edge
}

// Built-in parsers
type GoParser struct{}        // Parses Go imports, packages
type TypeScriptParser struct{} // Parses TS/JS imports, exports
type PythonParser struct{}     // Parses Python imports
type GenericParser struct{}    // Fallback: file nodes only
```

---

## 8. File Watching

### 8.1 Watcher Interface

```go
// Watcher monitors file system changes.
type Watcher interface {
    // Watch starts watching a path (file or directory).
    Watch(path string) error

    // Unwatch stops watching a path.
    Unwatch(path string) error

    // Events returns the channel of file change events.
    Events() <-chan WatchEvent

    // Errors returns the channel of watcher errors.
    Errors() <-chan error

    // Close stops the watcher and releases resources.
    Close() error

    // Stats returns watcher statistics.
    Stats() WatcherStats
}

// WatchEvent represents a file system change event.
type WatchEvent struct {
    Path      string
    Op        WatchOp
    Timestamp time.Time
}

// WatchOp represents the type of file system operation.
type WatchOp int

const (
    WatchOpCreate WatchOp = 1 << iota
    WatchOpWrite
    WatchOpRemove
    WatchOpRename
    WatchOpChmod
)

// WatcherStats provides watcher status information.
type WatcherStats struct {
    WatchedPaths  int
    PendingEvents int
    Errors        int
    LastError     error
}
```

### 8.2 Debouncing

```go
// DebouncedWatcher wraps a Watcher with event debouncing.
// Multiple rapid changes to the same file are coalesced into one event.
type DebouncedWatcher struct {
    inner    Watcher
    delay    time.Duration
    events   chan WatchEvent
    pending  map[string]*pendingEvent
    mu       sync.Mutex
}

type pendingEvent struct {
    event WatchEvent
    timer *time.Timer
}

// NewDebouncedWatcher creates a debounced watcher.
// Events are delayed by the specified duration and coalesced.
func NewDebouncedWatcher(inner Watcher, delay time.Duration) *DebouncedWatcher {
    dw := &DebouncedWatcher{
        inner:   inner,
        delay:   delay,
        events:  make(chan WatchEvent, 100),
        pending: make(map[string]*pendingEvent),
    }
    go dw.loop()
    return dw
}
```

### 8.3 Ignore Patterns

```go
// IgnorePatterns manages file/directory ignore rules.
type IgnorePatterns struct {
    patterns []ignorePattern
}

type ignorePattern struct {
    pattern  string
    glob     glob.Glob
    negation bool  // Pattern starts with !
    dirOnly  bool  // Pattern ends with /
}

// NewIgnorePatterns creates an ignore pattern matcher.
func NewIgnorePatterns() *IgnorePatterns {
    return &IgnorePatterns{}
}

// AddPattern adds an ignore pattern (gitignore syntax).
func (ip *IgnorePatterns) AddPattern(pattern string) error

// AddFromFile loads patterns from a file (e.g., .gitignore).
func (ip *IgnorePatterns) AddFromFile(path string) error

// Match returns true if the path should be ignored.
func (ip *IgnorePatterns) Match(path string, isDir bool) bool

// Default patterns loaded for all projects
var DefaultIgnorePatterns = []string{
    ".git/",
    "node_modules/",
    "__pycache__/",
    "*.pyc",
    ".DS_Store",
    "Thumbs.db",
    "*.swp",
    "*.swo",
    "*~",
}
```

---

## 9. Indexing and Search

### 9.1 Index Interface

```go
// Index provides fast file lookup.
type Index interface {
    // Add adds a file to the index.
    Add(path string, info FileInfo) error

    // Remove removes a file from the index.
    Remove(path string) error

    // Update updates an existing file in the index.
    Update(path string, info FileInfo) error

    // Get retrieves file info by path.
    Get(path string) (FileInfo, bool)

    // Count returns the number of indexed files.
    Count() int

    // All returns all indexed paths.
    All() []string

    // Query searches the index.
    Query(q IndexQuery) ([]IndexResult, error)

    // Clear removes all entries.
    Clear()

    // Persistence
    Save(w io.Writer) error
    Load(r io.Reader) error
}

// IndexQuery defines search parameters.
type IndexQuery struct {
    Pattern     string      // Search pattern
    MatchType   MatchType   // How to match
    FileTypes   []string    // Filter by extension
    MaxResults  int         // Limit results
    IncludeDirs bool        // Include directories
}

// MatchType indicates how to match search patterns.
type MatchType int

const (
    MatchExact MatchType = iota
    MatchPrefix
    MatchSuffix
    MatchContains
    MatchFuzzy
    MatchGlob
    MatchRegex
)

// IndexResult represents a search match.
type IndexResult struct {
    Path   string
    Info   FileInfo
    Score  float64  // Match quality (for ranking)
}
```

### 9.2 Fuzzy File Matching

```go
// FuzzyMatcher implements fuzzy string matching for file search.
type FuzzyMatcher struct {
    caseSensitive bool
    maxDistance   int
}

// Match scores how well pattern matches target.
// Returns 0 for no match, higher scores for better matches.
func (fm *FuzzyMatcher) Match(pattern, target string) float64 {
    // Implementation uses a combination of:
    // - Subsequence matching (characters in order)
    // - Consecutive character bonus
    // - Start-of-word bonus
    // - Path component matching
}

// Example scoring:
// Pattern: "main"
// "main.go"           -> 1.0 (exact match at start)
// "domain.go"         -> 0.7 (subsequence match)
// "my_main_file.go"   -> 0.9 (word boundary match)
// "something.go"      -> 0.0 (no match)
```

### 9.3 Content Search

```go
// ContentSearcher provides full-text search.
type ContentSearcher interface {
    // Search performs a content search.
    Search(ctx context.Context, query string, opts SearchOptions) ([]ContentMatch, error)

    // IndexFile indexes a file's content.
    IndexFile(path string, content []byte) error

    // RemoveFile removes a file from the index.
    RemoveFile(path string) error

    // Clear removes all indexed content.
    Clear()
}

// SearchOptions configures content search.
type SearchOptions struct {
    // Query options
    CaseSensitive bool
    WholeWord     bool
    UseRegex      bool

    // Scope
    IncludePaths  []string  // Glob patterns to include
    ExcludePaths  []string  // Glob patterns to exclude
    FileTypes     []string  // Extensions to search

    // Limits
    MaxResults    int
    MaxFileSize   int64

    // Context
    ContextLines  int  // Lines of context around matches
}

// ContentMatch represents a search result.
type ContentMatch struct {
    Path        string
    Line        int
    Column      int
    Text        string      // Matching line
    Context     []string    // Surrounding lines
    Highlights  []Range     // Match positions in text
}
```

### 9.4 Incremental Indexing

```go
// IncrementalIndexer manages background indexing.
type IncrementalIndexer struct {
    fileIndex     Index
    contentIndex  ContentSearcher
    graph         Graph
    vfs           VFS

    // Worker pool
    workers       int
    queue         chan indexJob

    // State
    status        IndexStatus
    progress      IndexProgress
}

// IndexStatus indicates the indexer state.
type IndexStatus int

const (
    IndexStatusIdle IndexStatus = iota
    IndexStatusIndexing
    IndexStatusError
)

// IndexProgress tracks indexing progress.
type IndexProgress struct {
    TotalFiles     int
    IndexedFiles   int
    ErrorFiles     int
    BytesProcessed int64
    StartTime      time.Time
    EstimatedDone  time.Time
}

// Start begins background indexing.
func (ii *IncrementalIndexer) Start(ctx context.Context, roots []string) error

// ProcessChange handles a file change event.
func (ii *IncrementalIndexer) ProcessChange(event FileChangeEvent) error

// Rebuild forces a full reindex.
func (ii *IncrementalIndexer) Rebuild(ctx context.Context) error
```

---

## 10. Workspace Management

### 10.1 Workspace Lifecycle

```go
// WorkspaceManager handles workspace operations.
type WorkspaceManager struct {
    mu         sync.RWMutex
    workspace  *Workspace
    vfs        VFS
    watcher    Watcher
    indexer    *IncrementalIndexer
    graph      Graph
    fileStore  *FileStore

    // Event handlers
    handlers   []func(WorkspaceChangeEvent)
}

// Open opens a workspace with the given roots.
func (wm *WorkspaceManager) Open(ctx context.Context, roots ...string) error {
    // 1. Validate roots exist
    // 2. Load workspace configuration
    // 3. Initialize file store
    // 4. Start file watcher
    // 5. Begin background indexing
    // 6. Build initial graph
    // 7. Emit workspace.opened event
}

// Close closes the workspace.
func (wm *WorkspaceManager) Close(ctx context.Context) error {
    // 1. Save dirty files (with user prompt)
    // 2. Stop file watcher
    // 3. Persist indexes
    // 4. Clear state
    // 5. Emit workspace.closed event
}

// AddFolder adds a folder to a multi-root workspace.
func (wm *WorkspaceManager) AddFolder(ctx context.Context, path string) error

// RemoveFolder removes a folder from a multi-root workspace.
func (wm *WorkspaceManager) RemoveFolder(ctx context.Context, path string) error
```

### 10.2 Configuration Loading

```go
// LoadConfig loads workspace configuration from various sources.
func LoadConfig(root string, vfs VFS) (*WorkspaceConfig, error) {
    config := DefaultWorkspaceConfig()

    // Load in priority order (later overrides earlier):
    // 1. Default configuration
    // 2. .keystorm/settings.json
    // 3. .vscode/settings.json (compatibility)
    // 4. Environment variables

    return config, nil
}

// DefaultWorkspaceConfig returns sensible defaults.
func DefaultWorkspaceConfig() *WorkspaceConfig {
    return &WorkspaceConfig{
        ExcludePatterns: []string{
            "**/node_modules/**",
            "**/.git/**",
            "**/vendor/**",
            "**/__pycache__/**",
            "**/dist/**",
            "**/build/**",
        },
        MaxFileSize:          10 * 1024 * 1024, // 10MB
        IndexingConcurrency:  runtime.NumCPU(),
    }
}
```

---

## 11. Integration with Editor

### 11.1 Event Bus Integration

```go
// Event types emitted by Project
const (
    EventFileOpened       = "project.file.opened"
    EventFileClosed       = "project.file.closed"
    EventFileSaved        = "project.file.saved"
    EventFileChanged      = "project.file.changed"      // External change
    EventFileCreated      = "project.file.created"
    EventFileDeleted      = "project.file.deleted"
    EventFileRenamed      = "project.file.renamed"
    EventWorkspaceOpened  = "project.workspace.opened"
    EventWorkspaceClosed  = "project.workspace.closed"
    EventIndexingStarted  = "project.indexing.started"
    EventIndexingProgress = "project.indexing.progress"
    EventIndexingComplete = "project.indexing.complete"
)
```

### 11.2 LSP Integration

```go
// LSPWorkspaceProvider implements LSP workspace folder interface.
type LSPWorkspaceProvider struct {
    project Project
}

// WorkspaceFolders returns folders for LSP initialization.
func (p *LSPWorkspaceProvider) WorkspaceFolders() []lsp.WorkspaceFolder {
    roots := p.project.Roots()
    folders := make([]lsp.WorkspaceFolder, len(roots))
    for i, root := range roots {
        folders[i] = lsp.WorkspaceFolder{
            URI:  "file://" + root,
            Name: filepath.Base(root),
        }
    }
    return folders
}

// FileWatcher provides file watching for LSP.
func (p *LSPWorkspaceProvider) WatchFiles(patterns []string, handler func(lsp.FileEvent)) {
    // Bridge project watcher to LSP file events
}
```

### 11.3 Dispatcher Integration

```go
// Handler provides project operations as dispatcher actions.
type Handler struct {
    project Project
    actions map[string]ActionFunc
}

// Action names
const (
    ActionOpenFile       = "project.openFile"
    ActionSaveFile       = "project.saveFile"
    ActionCloseFile      = "project.closeFile"
    ActionNewFile        = "project.newFile"
    ActionDeleteFile     = "project.deleteFile"
    ActionRenameFile     = "project.renameFile"
    ActionFindFiles      = "project.findFiles"
    ActionSearchContent  = "project.searchContent"
    ActionRelatedFiles   = "project.relatedFiles"
    ActionGotoFile       = "project.gotoFile"
)
```

### 11.4 Plugin API Implementation

```go
// Provider implements api.ProjectProvider for plugins.
type Provider struct {
    project Project
}

func (p *Provider) WorkspaceRoot() string {
    return p.project.Root()
}

func (p *Provider) WorkspaceFolders() []string {
    return p.project.Roots()
}

func (p *Provider) OpenFile(path string) ([]byte, error) {
    doc, err := p.project.OpenFile(context.Background(), path)
    if err != nil {
        return nil, err
    }
    return doc.Content, nil
}

// ... implement remaining ProjectProvider methods
```

---

## 12. Implementation Phases

### Phase 1: Core Infrastructure (Foundation)
**Goal**: Basic file operations and VFS abstraction

**Files to create**:
- `doc.go` - Package documentation
- `errors.go` - Error types
- `vfs/vfs.go` - VFS interface
- `vfs/os.go` - OS file system implementation
- `vfs/memory.go` - In-memory implementation
- `vfs/encoding.go` - Encoding detection

**Tests**:
- VFS interface tests (run against both OS and memory implementations)
- Encoding detection tests
- Line ending detection tests

**Deliverables**:
- Complete VFS abstraction
- File read/write operations
- Encoding handling

### Phase 2: Document Management
**Goal**: Track open documents and their state

**Files to create**:
- `filestore/store.go` - FileStore type
- `filestore/document.go` - Document type
- `filestore/sync.go` - Buffer ↔ disk synchronization

**Tests**:
- Document open/close lifecycle
- Dirty state tracking
- Concurrent access

**Deliverables**:
- Open document tracking
- Dirty state management
- Save/reload operations

### Phase 3: File Watching
**Goal**: Detect external file system changes

**Files to create**:
- `watcher/watcher.go` - Watcher interface
- `watcher/fsnotify.go` - fsnotify implementation
- `watcher/debounce.go` - Event debouncing
- `watcher/ignore.go` - Ignore patterns

**Tests**:
- Watch/unwatch operations
- Debouncing behavior
- Ignore pattern matching

**Deliverables**:
- File watching with debouncing
- Gitignore-style pattern support
- External change detection

### Phase 4: File Indexing
**Goal**: Fast file lookup and path matching

**Files to create**:
- `index/index.go` - Index interface
- `index/fileindex.go` - File path/name index
- `index/persist.go` - Index persistence

**Tests**:
- Index add/remove/update
- Query with different match types
- Persistence round-trip

**Deliverables**:
- File index with persistence
- Multiple match types (exact, fuzzy, glob)

### Phase 5: Search Engine
**Goal**: Fuzzy file search and content search

**Files to create**:
- `search/search.go` - Search interface
- `search/fuzzy.go` - Fuzzy file matching
- `search/content.go` - Content search
- `search/ranking.go` - Result ranking

**Tests**:
- Fuzzy matching accuracy
- Content search with regex
- Result ranking

**Deliverables**:
- Fast fuzzy file finder
- Full-text content search
- Ranked results

### Phase 6: Workspace Management
**Goal**: Multi-root workspace support

**Files to create**:
- `workspace/workspace.go` - Workspace type
- `workspace/config.go` - Configuration loading
- `workspace/multiroot.go` - Multi-root support

**Tests**:
- Single and multi-root workspaces
- Configuration loading priority
- Folder add/remove

**Deliverables**:
- Multi-root workspace support
- Configuration system
- Workspace lifecycle

### Phase 7: Project Graph
**Goal**: Build file relationship graph

**Files to create**:
- `graph/graph.go` - Graph interface
- `graph/node.go` - Node types
- `graph/edge.go` - Edge types
- `graph/builder.go` - Graph construction
- `graph/query.go` - Graph queries

**Tests**:
- Graph operations
- Relationship queries
- Language-specific parsing

**Deliverables**:
- Project graph structure
- Import relationship tracking
- Related files queries

### Phase 8: Incremental Indexing
**Goal**: Background indexing with incremental updates

**Files to create**:
- `index/contentindex.go` - Content index
- `index/incremental.go` - Incremental indexer

**Tests**:
- Background indexing
- Incremental updates
- Large project handling

**Deliverables**:
- Background content indexing
- Incremental updates on file changes
- Progress reporting

### Phase 9: Project Interface
**Goal**: Unified Project API

**Files to create**:
- `project.go` - Main Project type

**Tests**:
- Full API coverage
- Integration tests

**Deliverables**:
- Complete Project interface
- All components wired together

### Phase 10: Integration
**Goal**: Connect to dispatcher and plugin system

**Files to create**:
- `handler.go` - Dispatcher action handler
- `provider.go` - Plugin API implementation

**Tests**:
- Dispatcher action handling
- Plugin API compliance

**Deliverables**:
- Dispatcher integration
- Plugin system integration
- Event bus integration

---

## 13. Testing Strategy

### 13.1 Unit Tests

Each component should have comprehensive unit tests:

```go
// vfs/os_test.go
func TestOSFS_ReadFile(t *testing.T)
func TestOSFS_WriteFile(t *testing.T)
func TestOSFS_Walk(t *testing.T)

// watcher/debounce_test.go
func TestDebounce_CoalescesRapidChanges(t *testing.T)
func TestDebounce_DelaysEvents(t *testing.T)

// search/fuzzy_test.go
func TestFuzzyMatch_ExactMatch(t *testing.T)
func TestFuzzyMatch_SubsequenceMatch(t *testing.T)
func TestFuzzyMatch_WordBoundaryBonus(t *testing.T)
```

### 13.2 Integration Tests

```go
// project_integration_test.go
func TestProject_OpenWorkspace(t *testing.T)
func TestProject_FileLifecycle(t *testing.T)
func TestProject_SearchIntegration(t *testing.T)
func TestProject_ExternalChangeDetection(t *testing.T)
```

### 13.3 Benchmark Tests

```go
// search/fuzzy_bench_test.go
func BenchmarkFuzzyMatch(b *testing.B)
func BenchmarkFindFiles_10kFiles(b *testing.B)
func BenchmarkFindFiles_100kFiles(b *testing.B)

// index/fileindex_bench_test.go
func BenchmarkIndex_Add(b *testing.B)
func BenchmarkIndex_Query(b *testing.B)
```

### 13.4 Test Fixtures

```go
// testing/fixtures.go
// CreateTestWorkspace creates a temporary workspace for testing.
func CreateTestWorkspace(t *testing.T, files map[string]string) string

// CreateLargeWorkspace creates a workspace with many files for perf testing.
func CreateLargeWorkspace(t *testing.T, fileCount int) string
```

---

## 14. Performance Considerations

### 14.1 Memory Management

- **Lazy loading**: Don't load file contents until needed
- **Content eviction**: Evict unused file contents from memory
- **Index compression**: Compress persistent index format
- **Streaming**: Stream large file operations

### 14.2 Concurrency

- **Parallel indexing**: Use worker pool for initial indexing
- **Lock granularity**: Fine-grained locks per document
- **Read-write locks**: RWMutex for read-heavy operations
- **Channel-based events**: Non-blocking event delivery

### 14.3 I/O Optimization

- **Batch file operations**: Group multiple reads/writes
- **Memory-mapped files**: Consider mmap for large files
- **Watch batching**: Coalesce watch setup operations
- **Index persistence**: Incremental saves, not full rewrites

### 14.4 Performance Targets

| Operation | Target | Notes |
|-----------|--------|-------|
| File open | < 100ms | For files under 1MB |
| Fuzzy search | < 200ms | 100k file index |
| Content search | < 2s | Full project, regex |
| Initial indexing | < 30s | 100k file project |
| Incremental update | < 100ms | Single file change |
| Workspace open | < 5s | With index loading |

---

## Appendix A: File Type Detection

```go
// LanguageID returns the language identifier for a file.
func LanguageID(path string) string {
    ext := filepath.Ext(path)
    switch ext {
    case ".go":
        return "go"
    case ".ts":
        return "typescript"
    case ".tsx":
        return "typescriptreact"
    case ".js":
        return "javascript"
    case ".jsx":
        return "javascriptreact"
    case ".py":
        return "python"
    case ".rs":
        return "rust"
    case ".rb":
        return "ruby"
    case ".java":
        return "java"
    case ".c", ".h":
        return "c"
    case ".cpp", ".cc", ".cxx", ".hpp":
        return "cpp"
    case ".cs":
        return "csharp"
    case ".md":
        return "markdown"
    case ".json":
        return "json"
    case ".yaml", ".yml":
        return "yaml"
    case ".toml":
        return "toml"
    case ".xml":
        return "xml"
    case ".html", ".htm":
        return "html"
    case ".css":
        return "css"
    case ".sql":
        return "sql"
    case ".sh", ".bash":
        return "shellscript"
    default:
        return "plaintext"
    }
}
```

---

## Appendix B: Default Ignore Patterns

```go
var DefaultExcludePatterns = []string{
    // Version control
    "**/.git/**",
    "**/.svn/**",
    "**/.hg/**",

    // Dependencies
    "**/node_modules/**",
    "**/vendor/**",
    "**/.venv/**",
    "**/venv/**",
    "**/__pycache__/**",

    // Build outputs
    "**/dist/**",
    "**/build/**",
    "**/out/**",
    "**/target/**",
    "**/bin/**",
    "**/obj/**",

    // IDE/Editor
    "**/.idea/**",
    "**/.vscode/**",
    "**/.vs/**",
    "**/*.swp",
    "**/*.swo",
    "**/*~",

    // OS
    "**/.DS_Store",
    "**/Thumbs.db",

    // Logs and temp
    "**/*.log",
    "**/tmp/**",
    "**/temp/**",
}
```

---

## Appendix C: Dependencies

### Required Go Packages

```go
import (
    // Standard library
    "context"
    "io"
    "os"
    "path/filepath"
    "sync"
    "time"

    // File watching
    "github.com/fsnotify/fsnotify"

    // Glob patterns
    "github.com/gobwas/glob"

    // Encoding detection (optional)
    "golang.org/x/text/encoding"
    "golang.org/x/text/encoding/unicode"
)
```

---

## Summary

The File/Project Model implementation provides:

1. **Virtual File System**: Abstraction for testing and extensibility
2. **Document Management**: Track open files with dirty state
3. **File Watching**: Detect external changes with debouncing
4. **Indexing**: Fast file lookup with persistence
5. **Search**: Fuzzy file search and content search
6. **Workspace**: Multi-root workspace support
7. **Project Graph**: File relationship tracking for AI context
8. **Integration**: Dispatcher actions and plugin API

The 10-phase implementation plan progresses from foundational components to integration, with each phase building on previous work and producing testable deliverables.
