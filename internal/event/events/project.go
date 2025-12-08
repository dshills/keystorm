package events

import (
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Project event topics.
const (
	// TopicProjectFileOpened is published when a file is opened.
	TopicProjectFileOpened topic.Topic = "project.file.opened"

	// TopicProjectFileClosed is published when a file is closed.
	TopicProjectFileClosed topic.Topic = "project.file.closed"

	// TopicProjectFileSaved is published when a file is saved.
	TopicProjectFileSaved topic.Topic = "project.file.saved"

	// TopicProjectFileChanged is published when a file changes externally.
	TopicProjectFileChanged topic.Topic = "project.file.changed"

	// TopicProjectFileDirtyChanged is published when dirty state changes.
	TopicProjectFileDirtyChanged topic.Topic = "project.file.dirty.changed"

	// TopicProjectFileRenamed is published when a file is renamed.
	TopicProjectFileRenamed topic.Topic = "project.file.renamed"

	// TopicProjectWorkspaceOpened is published when workspace is initialized.
	TopicProjectWorkspaceOpened topic.Topic = "project.workspace.opened"

	// TopicProjectWorkspaceClosed is published when workspace is shut down.
	TopicProjectWorkspaceClosed topic.Topic = "project.workspace.closed"

	// TopicProjectWorkspaceFolderAdded is published when a folder is added.
	TopicProjectWorkspaceFolderAdded topic.Topic = "project.workspace.folder.added"

	// TopicProjectWorkspaceFolderRemoved is published when a folder is removed.
	TopicProjectWorkspaceFolderRemoved topic.Topic = "project.workspace.folder.removed"

	// TopicProjectIndexStarted is published when project indexing begins.
	TopicProjectIndexStarted topic.Topic = "project.index.started"

	// TopicProjectIndexProgress is published with indexing progress.
	TopicProjectIndexProgress topic.Topic = "project.index.progress"

	// TopicProjectIndexCompleted is published when project indexing completes.
	TopicProjectIndexCompleted topic.Topic = "project.index.completed"

	// TopicProjectIndexChanged is published when the project index updates.
	TopicProjectIndexChanged topic.Topic = "project.index.changed"

	// TopicProjectSearchStarted is published when a project search begins.
	TopicProjectSearchStarted topic.Topic = "project.search.started"

	// TopicProjectSearchResult is published for each search result.
	TopicProjectSearchResult topic.Topic = "project.search.result"

	// TopicProjectSearchCompleted is published when a search completes.
	TopicProjectSearchCompleted topic.Topic = "project.search.completed"
)

// FileChangeAction represents the type of file change.
type FileChangeAction string

// File change actions.
const (
	FileActionCreated  FileChangeAction = "created"
	FileActionModified FileChangeAction = "modified"
	FileActionDeleted  FileChangeAction = "deleted"
	FileActionRenamed  FileChangeAction = "renamed"
)

// LineEnding represents the line ending style.
type LineEnding string

// Line ending styles.
const (
	LineEndingLF   LineEnding = "lf"   // Unix/Linux/macOS
	LineEndingCRLF LineEnding = "crlf" // Windows
	LineEndingCR   LineEnding = "cr"   // Old macOS
)

// ProjectFileOpened is published when a file is opened.
type ProjectFileOpened struct {
	// Path is the absolute path to the file.
	Path string

	// BufferID is the buffer associated with the file.
	BufferID string

	// LanguageID identifies the language for syntax highlighting.
	LanguageID string

	// Encoding is the file encoding (e.g., "utf-8").
	Encoding string

	// LineEnding is the detected line ending style.
	LineEnding LineEnding

	// Size is the file size in bytes.
	Size int64

	// IsReadOnly indicates if the file is read-only.
	IsReadOnly bool
}

// ProjectFileClosed is published when a file is closed.
type ProjectFileClosed struct {
	// Path is the absolute path to the file.
	Path string

	// BufferID is the buffer that was associated with the file.
	BufferID string

	// WasDirty indicates if there were unsaved changes.
	WasDirty bool
}

// ProjectFileSaved is published when a file is saved.
type ProjectFileSaved struct {
	// Path is the absolute path where the file was saved.
	Path string

	// BufferID is the buffer that was saved.
	BufferID string

	// DiskModTime is the modification time on disk after saving.
	DiskModTime time.Time

	// BytesWritten is the number of bytes written.
	BytesWritten int64

	// Encoding is the encoding used when saving.
	Encoding string
}

// ProjectFileChanged is published when a file changes externally.
type ProjectFileChanged struct {
	// Path is the absolute path to the file.
	Path string

	// Action is the type of change.
	Action FileChangeAction

	// DiskModTime is the new modification time on disk.
	DiskModTime time.Time

	// IsOpenInEditor indicates if the file is currently open.
	IsOpenInEditor bool

	// BufferID is the buffer if the file is open, empty otherwise.
	BufferID string
}

// ProjectFileDirtyChanged is published when dirty state changes.
type ProjectFileDirtyChanged struct {
	// Path is the absolute path to the file.
	Path string

	// BufferID is the buffer associated with the file.
	BufferID string

	// IsDirty indicates whether the file has unsaved changes.
	IsDirty bool
}

// ProjectFileRenamed is published when a file is renamed.
type ProjectFileRenamed struct {
	// OldPath is the previous path.
	OldPath string

	// NewPath is the new path.
	NewPath string

	// BufferID is the buffer associated with the file.
	BufferID string
}

// ProjectWorkspaceOpened is published when workspace is initialized.
type ProjectWorkspaceOpened struct {
	// Roots are the workspace root directories.
	Roots []string

	// Name is the workspace name.
	Name string

	// ConfigPath is the path to the workspace config file, if any.
	ConfigPath string
}

// ProjectWorkspaceClosed is published when workspace is shut down.
type ProjectWorkspaceClosed struct {
	// DirtyFiles lists files with unsaved changes.
	DirtyFiles []string

	// OpenFileCount is the number of files that were open.
	OpenFileCount int
}

// ProjectWorkspaceFolderAdded is published when a folder is added.
type ProjectWorkspaceFolderAdded struct {
	// Path is the absolute path to the added folder.
	Path string

	// Name is the folder name.
	Name string

	// Index is the position in the workspace folder list.
	Index int
}

// ProjectWorkspaceFolderRemoved is published when a folder is removed.
type ProjectWorkspaceFolderRemoved struct {
	// Path is the absolute path to the removed folder.
	Path string

	// Name is the folder name.
	Name string
}

// ProjectIndexStarted is published when project indexing begins.
type ProjectIndexStarted struct {
	// Roots are the directories being indexed.
	Roots []string

	// EstimatedFiles is the estimated number of files to index.
	EstimatedFiles int
}

// ProjectIndexProgress is published with indexing progress.
type ProjectIndexProgress struct {
	// FilesIndexed is the number of files indexed so far.
	FilesIndexed int

	// TotalFiles is the total number of files to index.
	TotalFiles int

	// CurrentFile is the file currently being indexed.
	CurrentFile string

	// PercentComplete is the completion percentage (0-100).
	PercentComplete float64
}

// ProjectIndexCompleted is published when project indexing completes.
type ProjectIndexCompleted struct {
	// FilesIndexed is the total number of files indexed.
	FilesIndexed int

	// Duration is how long indexing took.
	Duration time.Duration

	// Errors is the number of files that failed to index.
	Errors int
}

// ProjectIndexChanged is published when the project index updates.
type ProjectIndexChanged struct {
	// Action is the type of change.
	Action FileChangeAction

	// Paths are the affected file paths.
	Paths []string

	// SymbolCount is the number of symbols affected.
	SymbolCount int
}

// SearchMatch represents a single search match.
type SearchMatch struct {
	// Path is the file path.
	Path string

	// Line is the line number (1-based).
	Line int

	// Column is the column number (1-based).
	Column int

	// Text is the matching line text.
	Text string

	// MatchStart is the start offset of the match in Text.
	MatchStart int

	// MatchEnd is the end offset of the match in Text.
	MatchEnd int
}

// ProjectSearchStarted is published when a project search begins.
type ProjectSearchStarted struct {
	// SearchID is a unique identifier for this search.
	SearchID string

	// Query is the search query.
	Query string

	// IsRegex indicates if the query is a regular expression.
	IsRegex bool

	// IsCaseSensitive indicates if the search is case-sensitive.
	IsCaseSensitive bool

	// IncludePatterns are glob patterns for files to include.
	IncludePatterns []string

	// ExcludePatterns are glob patterns for files to exclude.
	ExcludePatterns []string
}

// ProjectSearchResult is published for each search result.
type ProjectSearchResult struct {
	// SearchID identifies the search this result belongs to.
	SearchID string

	// Match is the search match.
	Match SearchMatch

	// ResultIndex is the 0-based index of this result.
	ResultIndex int
}

// ProjectSearchCompleted is published when a search completes.
type ProjectSearchCompleted struct {
	// SearchID identifies the completed search.
	SearchID string

	// TotalMatches is the total number of matches found.
	TotalMatches int

	// FilesSearched is the number of files searched.
	FilesSearched int

	// Duration is how long the search took.
	Duration time.Duration

	// WasCancelled indicates if the search was cancelled.
	WasCancelled bool
}
