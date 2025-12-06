package graph

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewBuilder(t *testing.T) {
	b := NewBuilder(0) // Should default to 4
	if b == nil {
		t.Fatal("NewBuilder returned nil")
	}
	if b.workers != 4 {
		t.Errorf("workers = %d, want 4 (default)", b.workers)
	}

	b2 := NewBuilder(8)
	if b2.workers != 8 {
		t.Errorf("workers = %d, want 8", b2.workers)
	}
}

func TestBuilder_RegisterParser(t *testing.T) {
	b := NewBuilder(1)

	// GoParser should be registered by default
	if _, ok := b.parsers[".go"]; !ok {
		t.Error("GoParser should be registered for .go extension")
	}

	// GenericParser should be registered as fallback
	if _, ok := b.parsers[""]; !ok {
		t.Error("GenericParser should be registered as fallback")
	}
}

func TestBuilder_SetIgnorePatterns(t *testing.T) {
	b := NewBuilder(1)
	patterns := []string{"*.log", "*.tmp"}
	b.SetIgnorePatterns(patterns)

	if len(b.ignorePatterns) != 2 {
		t.Errorf("ignorePatterns len = %d, want 2", len(b.ignorePatterns))
	}
}

func TestBuilder_Build_EmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	b := NewBuilder(2)
	g, err := b.Build(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if g.NodeCount() != 0 {
		t.Errorf("NodeCount() = %d, want 0", g.NodeCount())
	}
}

func TestBuilder_Build_WithGoFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a Go file
	goContent := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello")
	os.Exit(0)
}
`
	goFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write Go file: %v", err)
	}

	b := NewBuilder(2)
	g, err := b.Build(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Should have file node, package node, and import nodes
	if g.NodeCount() < 3 {
		t.Errorf("NodeCount() = %d, want >= 3", g.NodeCount())
	}

	// Check file node exists
	fileNodes := g.FindNodesByType(NodeTypeFile)
	if len(fileNodes) != 1 {
		t.Errorf("File nodes = %d, want 1", len(fileNodes))
	}

	// Check package node exists
	pkgNodes := g.FindNodesByType(NodeTypePackage)
	if len(pkgNodes) < 1 {
		t.Errorf("Package nodes = %d, want >= 1", len(pkgNodes))
	}
}

func TestBuilder_Build_IgnoresPatterns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a vendor directory with a Go file
	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}

	vendorFile := filepath.Join(vendorDir, "lib.go")
	if err := os.WriteFile(vendorFile, []byte("package lib"), 0644); err != nil {
		t.Fatalf("Failed to write vendor file: %v", err)
	}

	// Create a non-vendor Go file
	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write main file: %v", err)
	}

	b := NewBuilder(2)
	g, err := b.Build(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Only main.go should be parsed (vendor is ignored)
	fileNodes := g.FindNodesByType(NodeTypeFile)
	for _, node := range fileNodes {
		if filepath.Base(node.Path) == "lib.go" {
			t.Error("vendor/lib.go should be ignored")
		}
	}
}

func TestBuilder_Build_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some files
	for i := 0; i < 10; i++ {
		file := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".go")
		if err := os.WriteFile(file, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	b := NewBuilder(2)
	_, err = b.Build(ctx, tmpDir)
	if err != context.Canceled {
		t.Errorf("Build() error = %v, want context.Canceled", err)
	}
}

func TestGoParser_Language(t *testing.T) {
	p := &GoParser{}
	if got := p.Language(); got != "go" {
		t.Errorf("Language() = %q, want go", got)
	}
}

func TestGoParser_FileExtensions(t *testing.T) {
	p := &GoParser{}
	exts := p.FileExtensions()
	if len(exts) != 1 || exts[0] != ".go" {
		t.Errorf("FileExtensions() = %v, want [.go]", exts)
	}
}

func TestGoParser_Parse(t *testing.T) {
	p := &GoParser{}

	content := `package mypackage

import (
	"fmt"
	"strings"
)

func Hello() {
	fmt.Println(strings.ToUpper("hello"))
}
`

	result, err := p.Parse(context.Background(), "/path/to/file.go", []byte(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should have file node, package node, and 2 import nodes
	if len(result.Nodes) < 4 {
		t.Errorf("Nodes len = %d, want >= 4", len(result.Nodes))
	}

	// Should have edges for contains and imports
	if len(result.Edges) < 3 {
		t.Errorf("Edges len = %d, want >= 3", len(result.Edges))
	}
}

func TestGoParser_Parse_TestFile(t *testing.T) {
	p := &GoParser{}

	content := `package mypackage

import "testing"

func TestHello(t *testing.T) {
	// test
}
`

	result, err := p.Parse(context.Background(), "/path/to/file_test.go", []byte(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should have a tests edge
	hasTestEdge := false
	for _, edge := range result.Edges {
		if edge.Type == EdgeTypeTests {
			hasTestEdge = true
			break
		}
	}
	if !hasTestEdge {
		t.Error("Test file should create a tests edge")
	}
}

func TestGoParser_Parse_SingleImport(t *testing.T) {
	p := &GoParser{}

	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`

	result, err := p.Parse(context.Background(), "/path/to/main.go", []byte(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should have import edge for fmt
	hasImportEdge := false
	for _, edge := range result.Edges {
		if edge.Type == EdgeTypeImports && edge.Metadata.ImportPath == "fmt" {
			hasImportEdge = true
			break
		}
	}
	if !hasImportEdge {
		t.Error("Should have import edge for fmt")
	}
}

func TestGenericParser_Language(t *testing.T) {
	p := &GenericParser{}
	if got := p.Language(); got != "generic" {
		t.Errorf("Language() = %q, want generic", got)
	}
}

func TestGenericParser_FileExtensions(t *testing.T) {
	p := &GenericParser{}
	exts := p.FileExtensions()
	if len(exts) != 1 || exts[0] != "" {
		t.Errorf("FileExtensions() = %v, want [\"\"]", exts)
	}
}

func TestGenericParser_Parse(t *testing.T) {
	p := &GenericParser{}

	content := "Some generic content"
	result, err := p.Parse(context.Background(), "/path/to/file.txt", []byte(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Nodes) != 1 {
		t.Errorf("Nodes len = %d, want 1", len(result.Nodes))
	}

	if result.Nodes[0].Type != NodeTypeFile {
		t.Errorf("Node type = %v, want NodeTypeFile", result.Nodes[0].Type)
	}

	if result.Nodes[0].Metadata.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", result.Nodes[0].Metadata.Size, len(content))
	}
}

func TestMatchIgnorePattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/.git/**", "/project/.git/config", true},
		{"**/.git/**", "/project/src/main.go", false},
		{"**/node_modules/**", "/project/node_modules/pkg/index.js", true},
		{"**/vendor/**", "/project/vendor/lib/lib.go", true},
		{"**/vendor/**", "/project/src/vendor.go", false},
		{"*.log", "debug.log", true},
		{"*.log", "main.go", false},
	}

	for _, tt := range tests {
		got := matchIgnorePattern(tt.pattern, tt.path)
		if got != tt.want {
			t.Errorf("matchIgnorePattern(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestExtractGoPackage(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"package main\n\nfunc main() {}", "main"},
		{"package mypackage\n\nimport \"fmt\"", "mypackage"},
		{"// Copyright notice\n\npackage util", "util"},
		{"", ""},
		{"import \"fmt\"", ""},
	}

	for _, tt := range tests {
		got := extractGoPackage([]byte(tt.content))
		if got != tt.want {
			t.Errorf("extractGoPackage(%q) = %q, want %q", tt.content, got, tt.want)
		}
	}
}

func TestExtractGoImports(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "single import",
			content: `package main

import "fmt"`,
			want: []string{"fmt"},
		},
		{
			name: "import block",
			content: `package main

import (
	"fmt"
	"os"
)`,
			want: []string{"fmt", "os"},
		},
		{
			name: "named imports",
			content: `package main

import (
	f "fmt"
	_ "os"
)`,
			want: []string{"fmt", "os"},
		},
		{
			name: "no imports",
			content: `package main

func main() {}`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractGoImports([]byte(tt.content))
			if len(got) != len(tt.want) {
				t.Errorf("extractGoImports() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i, imp := range got {
				if imp != tt.want[i] {
					t.Errorf("extractGoImports()[%d] = %q, want %q", i, imp, tt.want[i])
				}
			}
		})
	}
}
