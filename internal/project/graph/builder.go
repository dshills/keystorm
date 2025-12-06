package graph

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Builder constructs a project graph from source files.
type Builder struct {
	graph   *MemGraph
	parsers map[string]LanguageParser
	workers int

	// Ignore patterns
	ignorePatterns []string
}

// LanguageParser extracts graph information from source files.
type LanguageParser interface {
	// Language returns the language ID (e.g., "go", "typescript").
	Language() string

	// FileExtensions returns supported file extensions.
	FileExtensions() []string

	// Parse extracts nodes and edges from a file.
	Parse(ctx context.Context, path string, content []byte) (*ParseResult, error)
}

// ParseResult contains extracted graph information from a file.
type ParseResult struct {
	Nodes []Node
	Edges []Edge
}

// NewBuilder creates a new graph builder.
func NewBuilder(workers int) *Builder {
	if workers <= 0 {
		workers = 4
	}
	b := &Builder{
		graph:   New(),
		parsers: make(map[string]LanguageParser),
		workers: workers,
		ignorePatterns: []string{
			"**/.git/**",
			"**/node_modules/**",
			"**/vendor/**",
			"**/__pycache__/**",
			"**/dist/**",
			"**/build/**",
		},
	}

	// Register built-in parsers
	b.RegisterParser(&GoParser{})
	b.RegisterParser(&GenericParser{})

	return b
}

// RegisterParser registers a language parser.
func (b *Builder) RegisterParser(parser LanguageParser) {
	for _, ext := range parser.FileExtensions() {
		b.parsers[ext] = parser
	}
}

// SetIgnorePatterns sets the ignore patterns for the builder.
func (b *Builder) SetIgnorePatterns(patterns []string) {
	b.ignorePatterns = patterns
}

// Build constructs the graph from the given root paths.
func (b *Builder) Build(ctx context.Context, roots ...string) (*MemGraph, error) {
	b.graph.Clear()

	// Collect all files
	var files []string
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			// Check for context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Skip ignored paths
			if b.shouldIgnore(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() {
				files = append(files, path)
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Process files in parallel
	type result struct {
		path   string
		result *ParseResult
		err    error
	}

	jobs := make(chan string, len(files))
	results := make(chan result, len(files))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < b.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				pr, err := b.parseFile(ctx, path)
				results <- result{path: path, result: pr, err: err}
			}
		}()
	}

	// Send jobs
	go func() {
		for _, file := range files {
			jobs <- file
		}
		close(jobs)
	}()

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for r := range results {
		if r.err != nil {
			continue // Skip files with errors
		}
		if r.result == nil {
			continue
		}

		// Add nodes
		for _, node := range r.result.Nodes {
			_ = b.graph.AddNode(node)
		}

		// Add edges (nodes must exist first)
		for _, edge := range r.result.Edges {
			_ = b.graph.AddEdge(edge)
		}
	}

	return b.graph, nil
}

// parseFile parses a single file and returns graph information.
func (b *Builder) parseFile(ctx context.Context, path string) (*ParseResult, error) {
	ext := filepath.Ext(path)
	parser, ok := b.parsers[ext]
	if !ok {
		parser = b.parsers[""] // Use generic parser
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return parser.Parse(ctx, path, content)
}

// shouldIgnore checks if a path should be ignored.
func (b *Builder) shouldIgnore(path string) bool {
	// Simple glob matching for common patterns
	for _, pattern := range b.ignorePatterns {
		if matchIgnorePattern(pattern, path) {
			return true
		}
	}
	return false
}

// matchIgnorePattern matches a path against an ignore pattern.
func matchIgnorePattern(pattern, path string) bool {
	// Normalize to forward slashes
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		// Extract the key segment (e.g., ".git" from "**/.git/**")
		parts := strings.Split(pattern, "**")
		for _, part := range parts {
			part = strings.Trim(part, "/")
			if part != "" && strings.Contains(path, "/"+part+"/") {
				return true
			}
			if part != "" && strings.HasSuffix(path, "/"+part) {
				return true
			}
		}
		return false
	}

	// Simple pattern matching
	matched, _ := filepath.Match(pattern, filepath.Base(path))
	return matched
}

// Graph returns the built graph.
func (b *Builder) Graph() *MemGraph {
	return b.graph
}

// GoParser parses Go source files.
type GoParser struct{}

// Language returns "go".
func (p *GoParser) Language() string {
	return "go"
}

// FileExtensions returns Go file extensions.
func (p *GoParser) FileExtensions() []string {
	return []string{".go"}
}

// Parse extracts graph information from a Go file.
func (p *GoParser) Parse(ctx context.Context, path string, content []byte) (*ParseResult, error) {
	result := &ParseResult{}

	// Create file node
	fileNode := NewFileNode(path)
	fileNode.Metadata.Size = int64(len(content))
	result.Nodes = append(result.Nodes, fileNode)

	// Extract package name
	pkgName := extractGoPackage(content)
	if pkgName != "" {
		// Create or reference package node
		pkgPath := filepath.Dir(path)
		pkgNode := NewPackageNode(pkgPath, pkgName, "go")
		result.Nodes = append(result.Nodes, pkgNode)

		// File belongs to package
		result.Edges = append(result.Edges, NewContainsEdge(pkgNode.ID, fileNode.ID))
	}

	// Extract imports
	imports := extractGoImports(content)
	for _, imp := range imports {
		// Create edge to imported package (external reference)
		importNode := Node{
			ID:       NodeID("import:" + imp),
			Type:     NodeTypePackage,
			Name:     imp,
			Language: "go",
			Metadata: NodeMeta{
				ModulePath: imp,
			},
		}
		result.Nodes = append(result.Nodes, importNode)
		result.Edges = append(result.Edges, NewImportEdge(fileNode.ID, importNode.ID, imp, nil))
	}

	// Detect test files
	if strings.HasSuffix(path, "_test.go") {
		// This is a test file - try to find the target
		targetPath := strings.TrimSuffix(path, "_test.go") + ".go"
		targetNode := NewFileNode(targetPath)
		result.Edges = append(result.Edges, NewTestsEdge(fileNode.ID, targetNode.ID))
	}

	return result, nil
}

// extractGoPackage extracts the package name from Go source.
func extractGoPackage(content []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	packageRegex := regexp.MustCompile(`^package\s+(\w+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "package ") {
			matches := packageRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				return matches[1]
			}
		}
		// Stop after finding package declaration or hitting imports
		if strings.HasPrefix(line, "import") {
			break
		}
	}
	return ""
}

// extractGoImports extracts import paths from Go source.
func extractGoImports(content []byte) []string {
	var imports []string
	scanner := bufio.NewScanner(bytes.NewReader(content))
	inImportBlock := false
	importRegex := regexp.MustCompile(`"([^"]+)"`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Single import
		if strings.HasPrefix(line, "import ") && !strings.Contains(line, "(") {
			matches := importRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				imports = append(imports, matches[1])
			}
			continue
		}

		// Start of import block
		if strings.HasPrefix(line, "import (") {
			inImportBlock = true
			continue
		}

		// End of import block
		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		// Inside import block
		if inImportBlock {
			matches := importRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				imports = append(imports, matches[1])
			}
		}

		// Stop parsing after imports (optimize for large files)
		if !inImportBlock && len(imports) > 0 && !strings.HasPrefix(line, "import") && !strings.HasPrefix(line, "//") && line != "" {
			break
		}
	}

	return imports
}

// GenericParser is a fallback parser that creates basic file nodes.
type GenericParser struct{}

// Language returns "generic".
func (p *GenericParser) Language() string {
	return "generic"
}

// FileExtensions returns an empty string to match all files.
func (p *GenericParser) FileExtensions() []string {
	return []string{""}
}

// Parse creates a basic file node.
func (p *GenericParser) Parse(ctx context.Context, path string, content []byte) (*ParseResult, error) {
	fileNode := NewFileNode(path)
	fileNode.Metadata.Size = int64(len(content))

	return &ParseResult{
		Nodes: []Node{fileNode},
	}, nil
}
