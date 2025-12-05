// Package filestore provides open document management for the project module.
//
// FileStore tracks all currently open documents, their state (clean/dirty),
// and provides synchronization between editor buffers and disk files.
package filestore

import (
	"bytes"
	"errors"
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/project/vfs"
)

// ErrInvalidEditRange is returned when ApplyEdit receives invalid offsets.
var ErrInvalidEditRange = errors.New("invalid edit range")

// Document represents an open file in the editor.
// It tracks the file's state, content, and metadata.
type Document struct {
	mu sync.RWMutex

	// Path is the absolute path to the file.
	Path string

	// Version is incremented on each edit.
	// Used for tracking changes and LSP synchronization.
	Version int64

	// Content is the current document content.
	// This may differ from disk if the document is dirty.
	Content []byte

	// OriginalContent is the content when the file was opened or last saved.
	// Used to detect if the document has changed.
	OriginalContent []byte

	// Encoding is the detected or specified character encoding.
	Encoding vfs.Encoding

	// LineEnding is the detected or specified line ending style.
	LineEnding vfs.LineEnding

	// OpenedAt is when the document was opened.
	OpenedAt time.Time

	// ModifiedAt is when the document was last modified in the editor.
	ModifiedAt time.Time

	// DiskModTime is the file's modification time on disk.
	// Used to detect external changes.
	DiskModTime time.Time

	// ReadOnly indicates the document should not be saved.
	ReadOnly bool

	// LanguageID is the language identifier (e.g., "go", "python").
	// Used for LSP and syntax highlighting.
	LanguageID string

	// closed indicates the document has been closed.
	closed bool
}

// NewDocument creates a new Document from file content.
func NewDocument(path string, content []byte, diskModTime time.Time) *Document {
	// Detect encoding info
	info := vfs.DetectEncodingInfo(content)

	// Strip BOM if present
	cleanContent, _ := vfs.StripBOM(content)

	// Make a copy of the content to avoid external modification
	contentCopy := make([]byte, len(cleanContent))
	copy(contentCopy, cleanContent)

	originalCopy := make([]byte, len(cleanContent))
	copy(originalCopy, cleanContent)

	return &Document{
		Path:            path,
		Version:         1,
		Content:         contentCopy,
		OriginalContent: originalCopy,
		Encoding:        info.Encoding,
		LineEnding:      info.LineEnding,
		OpenedAt:        time.Now(),
		ModifiedAt:      time.Now(),
		DiskModTime:     diskModTime,
		LanguageID:      detectLanguageID(path),
	}
}

// IsDirty returns true if the document has unsaved changes.
func (d *Document) IsDirty() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return !bytes.Equal(d.Content, d.OriginalContent)
}

// IsClosed returns true if the document has been closed.
func (d *Document) IsClosed() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.closed
}

// GetContent returns a copy of the document content.
func (d *Document) GetContent() []byte {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]byte, len(d.Content))
	copy(result, d.Content)
	return result
}

// SetContent updates the document content and increments the version.
func (d *Document) SetContent(content []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.Content = make([]byte, len(content))
	copy(d.Content, content)
	d.Version++
	d.ModifiedAt = time.Now()
}

// ApplyEdit applies an incremental edit to the document.
// startOffset and endOffset are byte offsets in the current content.
// Returns ErrInvalidEditRange if offsets are out of bounds.
func (d *Document) ApplyEdit(startOffset, endOffset int, newText []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Validate bounds
	if startOffset < 0 || endOffset < 0 ||
		startOffset > len(d.Content) || endOffset > len(d.Content) ||
		startOffset > endOffset {
		return ErrInvalidEditRange
	}

	// Build new content: prefix + newText + suffix
	prefix := d.Content[:startOffset]
	suffix := d.Content[endOffset:]

	newContent := make([]byte, len(prefix)+len(newText)+len(suffix))
	copy(newContent, prefix)
	copy(newContent[len(prefix):], newText)
	copy(newContent[len(prefix)+len(newText):], suffix)

	d.Content = newContent
	d.Version++
	d.ModifiedAt = time.Now()
	return nil
}

// MarkSaved updates the document state after saving.
func (d *Document) MarkSaved(diskModTime time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.OriginalContent = make([]byte, len(d.Content))
	copy(d.OriginalContent, d.Content)
	d.DiskModTime = diskModTime
}

// MarkClosed marks the document as closed.
func (d *Document) MarkClosed() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
}

// HasExternalChanges checks if the file has been modified externally.
func (d *Document) HasExternalChanges(currentDiskModTime time.Time) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return !currentDiskModTime.Equal(d.DiskModTime)
}

// Reload updates the document with new content from disk.
// Returns true if the content actually changed.
func (d *Document) Reload(content []byte, diskModTime time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Strip BOM if present
	cleanContent, encoding := vfs.StripBOM(content)

	// Check if content actually changed
	changed := false
	if len(cleanContent) != len(d.Content) {
		changed = true
	} else {
		for i := range cleanContent {
			if cleanContent[i] != d.Content[i] {
				changed = true
				break
			}
		}
	}

	if changed {
		d.Content = make([]byte, len(cleanContent))
		copy(d.Content, cleanContent)
		d.OriginalContent = make([]byte, len(cleanContent))
		copy(d.OriginalContent, cleanContent)
		d.Version++
		d.ModifiedAt = time.Now()

		// Update encoding if BOM was present
		if encoding != vfs.EncodingUTF8 {
			d.Encoding = encoding
		}
	}

	d.DiskModTime = diskModTime
	return changed
}

// GetVersion returns the current document version.
func (d *Document) GetVersion() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Version
}

// LineCount returns the number of lines in the document.
func (d *Document) LineCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return vfs.CountLines(d.Content)
}

// Size returns the size of the document content in bytes.
func (d *Document) Size() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.Content)
}

// ContentForSave returns the content prepared for saving to disk.
// This includes re-adding BOM if needed and normalizing line endings.
// The returned slice is a copy safe for use after the lock is released.
func (d *Document) ContentForSave() []byte {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Make a copy of the content to avoid races after lock release
	content := make([]byte, len(d.Content))
	copy(content, d.Content)

	// Normalize line endings if specified
	if d.LineEnding != vfs.LineEndingMixed {
		content = vfs.NormalizeLineEndings(content, d.LineEnding)
	}

	// Add BOM if needed
	content = vfs.AddBOM(content, d.Encoding)

	return content
}

// detectLanguageID returns the language identifier based on file extension.
func detectLanguageID(path string) string {
	ext := ""
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			ext = path[i:]
			break
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}

	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".jsx":
		return "javascriptreact"
	case ".tsx":
		return "typescriptreact"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss":
		return "scss"
	case ".less":
		return "less"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".xml":
		return "xml"
	case ".md", ".markdown":
		return "markdown"
	case ".sql":
		return "sql"
	case ".sh", ".bash":
		return "shellscript"
	case ".ps1":
		return "powershell"
	case ".dockerfile":
		return "dockerfile"
	case ".lua":
		return "lua"
	case ".r", ".R":
		return "r"
	case ".jl":
		return "julia"
	case ".ex", ".exs":
		return "elixir"
	case ".erl":
		return "erlang"
	case ".hs":
		return "haskell"
	case ".ml", ".mli":
		return "ocaml"
	case ".fs", ".fsi", ".fsx":
		return "fsharp"
	case ".clj", ".cljs", ".cljc":
		return "clojure"
	case ".vim":
		return "vim"
	case ".toml":
		return "toml"
	case ".ini", ".cfg":
		return "ini"
	case ".proto":
		return "protobuf"
	case ".graphql", ".gql":
		return "graphql"
	default:
		return "plaintext"
	}
}
