package graph

import "testing"

func TestNodeType_String(t *testing.T) {
	tests := []struct {
		nodeType NodeType
		want     string
	}{
		{NodeTypeFile, "file"},
		{NodeTypeDirectory, "directory"},
		{NodeTypeModule, "module"},
		{NodeTypePackage, "package"},
		{NodeTypeClass, "class"},
		{NodeTypeFunction, "function"},
		{NodeTypeTest, "test"},
		{NodeTypeConfig, "config"},
		{NodeTypeAPI, "api"},
		{NodeTypeSchema, "schema"},
		{NodeType(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.nodeType.String(); got != tt.want {
			t.Errorf("NodeType(%d).String() = %q, want %q", tt.nodeType, got, tt.want)
		}
	}
}

func TestNewFileNode(t *testing.T) {
	node := NewFileNode("/path/to/main.go")

	if node.Type != NodeTypeFile {
		t.Errorf("Type = %v, want NodeTypeFile", node.Type)
	}
	if node.Path != "/path/to/main.go" {
		t.Errorf("Path = %q, want /path/to/main.go", node.Path)
	}
	if node.Name != "main.go" {
		t.Errorf("Name = %q, want main.go", node.Name)
	}
	if node.Language != "go" {
		t.Errorf("Language = %q, want go", node.Language)
	}
	if node.ID == "" {
		t.Error("ID should not be empty")
	}
}

func TestNewDirectoryNode(t *testing.T) {
	node := NewDirectoryNode("/path/to/dir")

	if node.Type != NodeTypeDirectory {
		t.Errorf("Type = %v, want NodeTypeDirectory", node.Type)
	}
	if node.Path != "/path/to/dir" {
		t.Errorf("Path = %q, want /path/to/dir", node.Path)
	}
	if node.Name != "dir" {
		t.Errorf("Name = %q, want dir", node.Name)
	}
}

func TestNewModuleNode(t *testing.T) {
	node := NewModuleNode("github.com/user/repo", "v1.2.3")

	if node.Type != NodeTypeModule {
		t.Errorf("Type = %v, want NodeTypeModule", node.Type)
	}
	if node.Metadata.ModulePath != "github.com/user/repo" {
		t.Errorf("ModulePath = %q, want github.com/user/repo", node.Metadata.ModulePath)
	}
	if node.Metadata.Version != "v1.2.3" {
		t.Errorf("Version = %q, want v1.2.3", node.Metadata.Version)
	}
}

func TestNewPackageNode(t *testing.T) {
	node := NewPackageNode("/path/to/pkg", "mypkg", "go")

	if node.Type != NodeTypePackage {
		t.Errorf("Type = %v, want NodeTypePackage", node.Type)
	}
	if node.Name != "mypkg" {
		t.Errorf("Name = %q, want mypkg", node.Name)
	}
	if node.Language != "go" {
		t.Errorf("Language = %q, want go", node.Language)
	}
}

func TestNewFunctionNode(t *testing.T) {
	node := NewFunctionNode("/path/to/file.go", "MyFunc", 10, 25)

	if node.Type != NodeTypeFunction {
		t.Errorf("Type = %v, want NodeTypeFunction", node.Type)
	}
	if node.Name != "MyFunc" {
		t.Errorf("Name = %q, want MyFunc", node.Name)
	}
	if node.Metadata.StartLine != 10 {
		t.Errorf("StartLine = %d, want 10", node.Metadata.StartLine)
	}
	if node.Metadata.EndLine != 25 {
		t.Errorf("EndLine = %d, want 25", node.Metadata.EndLine)
	}
}

func TestNewTestNode(t *testing.T) {
	node := NewTestNode("/path/to/file_test.go", "/path/to/file.go")

	if node.Type != NodeTypeTest {
		t.Errorf("Type = %v, want NodeTypeTest", node.Type)
	}
	if node.Metadata.TestTarget != "/path/to/file.go" {
		t.Errorf("TestTarget = %q, want /path/to/file.go", node.Metadata.TestTarget)
	}
}

func TestNode_IsFileNode(t *testing.T) {
	fileNode := NewFileNode("/path/to/file.go")
	dirNode := NewDirectoryNode("/path/to/dir")

	if !fileNode.IsFileNode() {
		t.Error("file node should return true for IsFileNode")
	}
	if dirNode.IsFileNode() {
		t.Error("directory node should return false for IsFileNode")
	}
}

func TestNode_IsDirectoryNode(t *testing.T) {
	fileNode := NewFileNode("/path/to/file.go")
	dirNode := NewDirectoryNode("/path/to/dir")

	if fileNode.IsDirectoryNode() {
		t.Error("file node should return false for IsDirectoryNode")
	}
	if !dirNode.IsDirectoryNode() {
		t.Error("directory node should return true for IsDirectoryNode")
	}
}

func TestNode_IsCodeNode(t *testing.T) {
	tests := []struct {
		node Node
		want bool
	}{
		{NewFileNode("/file.go"), true},
		{NewFunctionNode("/file.go", "func", 1, 10), true},
		{NewDirectoryNode("/dir"), false},
		{NewModuleNode("module", "v1"), false},
	}

	for _, tt := range tests {
		if got := tt.node.IsCodeNode(); got != tt.want {
			t.Errorf("IsCodeNode() for %v = %v, want %v", tt.node.Type, got, tt.want)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/path/to/file.go", "go"},
		{"/path/to/file.ts", "typescript"},
		{"/path/to/file.tsx", "typescriptreact"},
		{"/path/to/file.js", "javascript"},
		{"/path/to/file.jsx", "javascriptreact"},
		{"/path/to/file.py", "python"},
		{"/path/to/file.rs", "rust"},
		{"/path/to/file.rb", "ruby"},
		{"/path/to/file.java", "java"},
		{"/path/to/file.c", "c"},
		{"/path/to/file.h", "c"},
		{"/path/to/file.cpp", "cpp"},
		{"/path/to/file.hpp", "cpp"},
		{"/path/to/file.cs", "csharp"},
		{"/path/to/file.swift", "swift"},
		{"/path/to/file.kt", "kotlin"},
		{"/path/to/file.scala", "scala"},
		{"/path/to/file.md", "markdown"},
		{"/path/to/file.json", "json"},
		{"/path/to/file.yaml", "yaml"},
		{"/path/to/file.yml", "yaml"},
		{"/path/to/file.toml", "toml"},
		{"/path/to/file.xml", "xml"},
		{"/path/to/file.html", "html"},
		{"/path/to/file.css", "css"},
		{"/path/to/file.scss", "scss"},
		{"/path/to/file.sql", "sql"},
		{"/path/to/file.sh", "shellscript"},
		{"/path/to/file.bash", "shellscript"},
		{"/path/to/file.unknown", "plaintext"},
		{"/path/to/file", "plaintext"},
	}

	for _, tt := range tests {
		got := detectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
