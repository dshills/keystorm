package graph

import (
	"path/filepath"
	"time"
)

// NodeID uniquely identifies a node in the graph.
type NodeID string

// NodeType indicates the kind of node.
type NodeType int

const (
	// NodeTypeFile represents a source file.
	NodeTypeFile NodeType = iota
	// NodeTypeDirectory represents a directory.
	NodeTypeDirectory
	// NodeTypeModule represents a module (Go module, npm package, etc.).
	NodeTypeModule
	// NodeTypePackage represents a package (Go package, Python module, etc.).
	NodeTypePackage
	// NodeTypeClass represents a class definition.
	NodeTypeClass
	// NodeTypeFunction represents a function or method.
	NodeTypeFunction
	// NodeTypeTest represents a test file or test function.
	NodeTypeTest
	// NodeTypeConfig represents a configuration file.
	NodeTypeConfig
	// NodeTypeAPI represents an API endpoint definition.
	NodeTypeAPI
	// NodeTypeSchema represents a database schema, protobuf, etc.
	NodeTypeSchema
)

// String returns the string representation of a NodeType.
func (t NodeType) String() string {
	switch t {
	case NodeTypeFile:
		return "file"
	case NodeTypeDirectory:
		return "directory"
	case NodeTypeModule:
		return "module"
	case NodeTypePackage:
		return "package"
	case NodeTypeClass:
		return "class"
	case NodeTypeFunction:
		return "function"
	case NodeTypeTest:
		return "test"
	case NodeTypeConfig:
		return "config"
	case NodeTypeAPI:
		return "api"
	case NodeTypeSchema:
		return "schema"
	default:
		return "unknown"
	}
}

// Node represents an entity in the project graph.
type Node struct {
	// ID uniquely identifies this node.
	ID NodeID `json:"id"`
	// Type indicates the kind of node.
	Type NodeType `json:"type"`
	// Path is the file path (for file-based nodes).
	Path string `json:"path,omitempty"`
	// Name is the display name for the node.
	Name string `json:"name"`
	// Language is the programming language (e.g., "go", "typescript").
	Language string `json:"language,omitempty"`
	// Metadata holds type-specific metadata.
	Metadata NodeMeta `json:"metadata,omitempty"`
}

// NodeMeta holds type-specific node metadata.
type NodeMeta struct {
	// For modules/packages
	ModulePath string `json:"module_path,omitempty"` // e.g., "github.com/user/repo"
	Version    string `json:"version,omitempty"`

	// For files
	Size    int64     `json:"size,omitempty"`
	ModTime time.Time `json:"mod_time,omitempty"`

	// For functions/classes
	Signature string `json:"signature,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`

	// For tests
	TestTarget string `json:"test_target,omitempty"` // Path to tested file

	// Custom metadata
	Extra map[string]any `json:"extra,omitempty"`
}

// NewFileNode creates a new file node.
func NewFileNode(path string) Node {
	return Node{
		ID:       NodeID("file:" + path),
		Type:     NodeTypeFile,
		Path:     path,
		Name:     filepath.Base(path),
		Language: detectLanguage(path),
	}
}

// NewDirectoryNode creates a new directory node.
func NewDirectoryNode(path string) Node {
	return Node{
		ID:   NodeID("dir:" + path),
		Type: NodeTypeDirectory,
		Path: path,
		Name: filepath.Base(path),
	}
}

// NewModuleNode creates a new module node.
func NewModuleNode(modulePath, version string) Node {
	return Node{
		ID:   NodeID("module:" + modulePath),
		Type: NodeTypeModule,
		Name: modulePath,
		Metadata: NodeMeta{
			ModulePath: modulePath,
			Version:    version,
		},
	}
}

// NewPackageNode creates a new package node.
func NewPackageNode(path, name, language string) Node {
	return Node{
		ID:       NodeID("pkg:" + path),
		Type:     NodeTypePackage,
		Path:     path,
		Name:     name,
		Language: language,
	}
}

// NewFunctionNode creates a new function node.
func NewFunctionNode(path, name string, startLine, endLine int) Node {
	return Node{
		ID:       NodeID("func:" + path + ":" + name),
		Type:     NodeTypeFunction,
		Path:     path,
		Name:     name,
		Language: detectLanguage(path),
		Metadata: NodeMeta{
			StartLine: startLine,
			EndLine:   endLine,
		},
	}
}

// NewTestNode creates a new test node.
func NewTestNode(path, targetPath string) Node {
	return Node{
		ID:       NodeID("test:" + path),
		Type:     NodeTypeTest,
		Path:     path,
		Name:     filepath.Base(path),
		Language: detectLanguage(path),
		Metadata: NodeMeta{
			TestTarget: targetPath,
		},
	}
}

// IsFileNode returns true if this is a file node.
func (n Node) IsFileNode() bool {
	return n.Type == NodeTypeFile
}

// IsDirectoryNode returns true if this is a directory node.
func (n Node) IsDirectoryNode() bool {
	return n.Type == NodeTypeDirectory
}

// IsCodeNode returns true if this node represents code (file, function, class).
func (n Node) IsCodeNode() bool {
	switch n.Type {
	case NodeTypeFile, NodeTypeFunction, NodeTypeClass:
		return true
	default:
		return false
	}
}

// detectLanguage detects the language from a file path.
func detectLanguage(path string) string {
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
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
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
	case ".scss":
		return "scss"
	case ".sql":
		return "sql"
	case ".sh", ".bash":
		return "shellscript"
	default:
		return "plaintext"
	}
}
