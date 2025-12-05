package lsp

import (
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/plugin/api"
)

func TestNewProvider(t *testing.T) {
	client := NewClient()
	provider := NewProvider(client)

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}

	if provider.client != client {
		t.Error("expected provider to wrap client")
	}

	if provider.timeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", provider.timeout)
	}
}

func TestNewProviderNilClientPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil client")
		}
	}()
	NewProvider(nil)
}

func TestNewProviderWithOptions(t *testing.T) {
	client := NewClient()
	provider := NewProvider(client,
		WithProviderTimeout(30*time.Second),
	)

	if provider.timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", provider.timeout)
	}
}

func TestProviderSetDocumentContent(t *testing.T) {
	client := NewClient()
	provider := NewProvider(client)

	content := "package main\n\nfunc main() {}\n"
	provider.SetDocumentContent("/test/file.go", content)

	got := provider.getContent("/test/file.go")
	if got != content {
		t.Errorf("expected content %q, got %q", content, got)
	}
}

func TestProviderClearDocumentContent(t *testing.T) {
	client := NewClient()
	provider := NewProvider(client)

	provider.SetDocumentContent("/test/file.go", "content")
	provider.ClearDocumentContent("/test/file.go")

	got := provider.getContent("/test/file.go")
	if got != "" {
		t.Errorf("expected empty content after clear, got %q", got)
	}
}

func TestProviderIsAvailable(t *testing.T) {
	client := NewClient()
	provider := NewProvider(client)

	// Without starting client, nothing should be available
	if provider.IsAvailable("/test/file.go") {
		t.Error("expected IsAvailable to return false for non-started client")
	}
}

func TestProviderExtractPrefix(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		offset   int
		expected string
	}{
		{
			name:     "simple word",
			content:  "hello world",
			offset:   5,
			expected: "hello",
		},
		{
			name:     "partial word",
			content:  "fmt.Print",
			offset:   9,
			expected: "Print",
		},
		{
			name:     "empty at start",
			content:  "hello",
			offset:   0,
			expected: "",
		},
		{
			name:     "after dot",
			content:  "obj.",
			offset:   4,
			expected: "",
		},
		{
			name:     "with underscore",
			content:  "my_var",
			offset:   6,
			expected: "my_var",
		},
		{
			name:     "with numbers",
			content:  "var123",
			offset:   6,
			expected: "var123",
		},
		{
			name:     "empty content",
			content:  "",
			offset:   0,
			expected: "",
		},
		{
			name:     "offset beyond content",
			content:  "short",
			offset:   100,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerExtractPrefix(tt.content, tt.offset)
			if got != tt.expected {
				t.Errorf("providerExtractPrefix(%q, %d) = %q, want %q",
					tt.content, tt.offset, got, tt.expected)
			}
		})
	}
}

func TestProviderIsWordChar(t *testing.T) {
	tests := []struct {
		char     rune
		expected bool
	}{
		{'a', true},
		{'z', true},
		{'A', true},
		{'Z', true},
		{'0', true},
		{'9', true},
		{'_', true},
		{'.', false},
		{' ', false},
		{'(', false},
		{'-', false},
	}

	for _, tt := range tests {
		got := providerIsWordChar(tt.char)
		if got != tt.expected {
			t.Errorf("providerIsWordChar(%q) = %v, want %v", tt.char, got, tt.expected)
		}
	}
}

func TestProviderConvertCompletionItem(t *testing.T) {
	lspItem := CompletionItem{
		Label:         "println",
		Kind:          CompletionItemKindFunction,
		Detail:        "func(a ...any)",
		Documentation: "Println writes to stdout",
		InsertText:    "println($0)",
		SortText:      "0001",
	}

	apiItem := providerConvertCompletionItem(lspItem)

	if apiItem.Label != "println" {
		t.Errorf("expected label 'println', got %q", apiItem.Label)
	}

	if apiItem.Kind != api.CompletionKindFunction {
		t.Errorf("expected kind Function, got %v", apiItem.Kind)
	}

	if apiItem.Detail != "func(a ...any)" {
		t.Errorf("expected detail 'func(a ...any)', got %q", apiItem.Detail)
	}

	if apiItem.Documentation != "Println writes to stdout" {
		t.Errorf("expected documentation, got %q", apiItem.Documentation)
	}
}

func TestProviderExtractDocumentation(t *testing.T) {
	tests := []struct {
		name     string
		doc      any
		expected string
	}{
		{
			name:     "string",
			doc:      "Simple doc",
			expected: "Simple doc",
		},
		{
			name:     "MarkupContent",
			doc:      MarkupContent{Kind: "markdown", Value: "# Heading"},
			expected: "# Heading",
		},
		{
			name:     "MarkupContent pointer",
			doc:      &MarkupContent{Kind: "plaintext", Value: "Plain text"},
			expected: "Plain text",
		},
		{
			name:     "map with value",
			doc:      map[string]any{"value": "Map value"},
			expected: "Map value",
		},
		{
			name:     "nil",
			doc:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerExtractDocumentation(tt.doc)
			if got != tt.expected {
				t.Errorf("providerExtractDocumentation() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestProviderConvertRange(t *testing.T) {
	lspRange := Range{
		Start: Position{Line: 10, Character: 5},
		End:   Position{Line: 10, Character: 15},
	}

	apiRange := providerConvertRange(lspRange, "")

	if apiRange.StartLine != 10 {
		t.Errorf("expected start line 10, got %d", apiRange.StartLine)
	}
	if apiRange.StartColumn != 5 {
		t.Errorf("expected start column 5, got %d", apiRange.StartColumn)
	}
	if apiRange.EndLine != 10 {
		t.Errorf("expected end line 10, got %d", apiRange.EndLine)
	}
	if apiRange.EndColumn != 15 {
		t.Errorf("expected end column 15, got %d", apiRange.EndColumn)
	}
}

func TestProviderConvertAPIRangeToLSP(t *testing.T) {
	apiRange := api.Range{
		StartLine:   5,
		StartColumn: 10,
		EndLine:     5,
		EndColumn:   20,
	}

	lspRange := providerConvertAPIRangeToLSP(apiRange, "")

	if lspRange.Start.Line != 5 {
		t.Errorf("expected start line 5, got %d", lspRange.Start.Line)
	}
	if lspRange.Start.Character != 10 {
		t.Errorf("expected start character 10, got %d", lspRange.Start.Character)
	}
	if lspRange.End.Line != 5 {
		t.Errorf("expected end line 5, got %d", lspRange.End.Line)
	}
	if lspRange.End.Character != 20 {
		t.Errorf("expected end character 20, got %d", lspRange.End.Character)
	}
}

func TestProviderConvertLocation(t *testing.T) {
	lspLoc := Location{
		URI: "file:///home/user/project/main.go",
		Range: Range{
			Start: Position{Line: 5, Character: 0},
			End:   Position{Line: 5, Character: 10},
		},
	}

	apiLoc := providerConvertLocation(lspLoc, "")

	if apiLoc.Path != "/home/user/project/main.go" {
		t.Errorf("expected path '/home/user/project/main.go', got %q", apiLoc.Path)
	}

	if apiLoc.Range.StartLine != 5 {
		t.Errorf("expected start line 5, got %d", apiLoc.Range.StartLine)
	}
}

func TestProviderConvertDiagnostic(t *testing.T) {
	lspDiag := Diagnostic{
		Range: Range{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 15},
		},
		Severity: DiagnosticSeverityError,
		Code:     "E001",
		Source:   "gopls",
		Message:  "undefined: foo",
	}

	apiDiag := providerConvertDiagnostic(lspDiag, "")

	if apiDiag.Severity != api.DiagnosticSeverityError {
		t.Errorf("expected severity Error, got %v", apiDiag.Severity)
	}

	if apiDiag.Message != "undefined: foo" {
		t.Errorf("expected message 'undefined: foo', got %q", apiDiag.Message)
	}

	if apiDiag.Source != "gopls" {
		t.Errorf("expected source 'gopls', got %q", apiDiag.Source)
	}

	if apiDiag.Code != "E001" {
		t.Errorf("expected code 'E001', got %q", apiDiag.Code)
	}
}

func TestProviderConvertDiagnosticNumericCode(t *testing.T) {
	// Test float64 code (common from JSON)
	lspDiag := Diagnostic{
		Range: Range{
			Start: Position{Line: 1, Character: 0},
			End:   Position{Line: 1, Character: 10},
		},
		Severity: DiagnosticSeverityWarning,
		Code:     float64(123),
		Source:   "linter",
		Message:  "some warning",
	}

	apiDiag := providerConvertDiagnostic(lspDiag, "")

	if apiDiag.Code != "123" {
		t.Errorf("expected code '123' for float64 code, got %q", apiDiag.Code)
	}

	// Test int code
	lspDiag.Code = 456
	apiDiag = providerConvertDiagnostic(lspDiag, "")

	if apiDiag.Code != "456" {
		t.Errorf("expected code '456' for int code, got %q", apiDiag.Code)
	}
}

func TestProviderConvertHover(t *testing.T) {
	rng := Range{
		Start: Position{Line: 5, Character: 10},
		End:   Position{Line: 5, Character: 20},
	}
	lspHover := &Hover{
		Contents: MarkupContent{Kind: "plaintext", Value: "func main()"},
		Range:    &rng,
	}

	apiHover := providerConvertHover(lspHover, "")

	if apiHover == nil {
		t.Fatal("expected non-nil hover")
	}

	if apiHover.Contents != "func main()" {
		t.Errorf("expected contents 'func main()', got %q", apiHover.Contents)
	}

	if apiHover.Range == nil {
		t.Error("expected non-nil range")
	}
}

func TestProviderConvertHoverNil(t *testing.T) {
	apiHover := providerConvertHover(nil, "")
	if apiHover != nil {
		t.Error("expected nil hover for nil input")
	}
}

func TestProviderExtractHoverContents(t *testing.T) {
	tests := []struct {
		name     string
		contents any
		expected string
	}{
		{
			name:     "string",
			contents: "Simple content",
			expected: "Simple content",
		},
		{
			name:     "MarkupContent",
			contents: MarkupContent{Kind: "markdown", Value: "**Bold**"},
			expected: "**Bold**",
		},
		{
			name:     "map",
			contents: map[string]any{"value": "Mapped content"},
			expected: "Mapped content",
		},
		{
			name:     "array of strings",
			contents: []any{"Part1", "Part2"},
			expected: "Part1\n\nPart2",
		},
		{
			name:     "nil",
			contents: nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerExtractHoverContents(tt.contents)
			if got != tt.expected {
				t.Errorf("providerExtractHoverContents() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestProviderConvertTextEdit(t *testing.T) {
	lspEdit := TextEdit{
		Range: Range{
			Start: Position{Line: 5, Character: 0},
			End:   Position{Line: 5, Character: 5},
		},
		NewText: "hello",
	}

	apiEdit := providerConvertTextEdit(lspEdit, "")

	if apiEdit.NewText != "hello" {
		t.Errorf("expected new text 'hello', got %q", apiEdit.NewText)
	}

	if apiEdit.Range.StartLine != 5 {
		t.Errorf("expected start line 5, got %d", apiEdit.Range.StartLine)
	}
}

func TestProviderConvertCodeAction(t *testing.T) {
	lspAction := CodeAction{
		Title: "Extract variable",
		Kind:  CodeActionKindRefactorExtract,
		Command: &Command{
			Title:   "Extract",
			Command: "extract.variable",
		},
	}

	apiAction := providerConvertCodeAction(lspAction, "")

	if apiAction.Title != "Extract variable" {
		t.Errorf("expected title 'Extract variable', got %q", apiAction.Title)
	}

	if apiAction.Kind != api.CodeActionKindRefactorExtract {
		t.Errorf("expected kind refactor.extract, got %v", apiAction.Kind)
	}

	if apiAction.Command != "extract.variable" {
		t.Errorf("expected command 'extract.variable', got %q", apiAction.Command)
	}
}

func TestProviderJoinStrings(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		sep      string
		expected string
	}{
		{
			name:     "empty",
			parts:    []string{},
			sep:      ", ",
			expected: "",
		},
		{
			name:     "single",
			parts:    []string{"one"},
			sep:      ", ",
			expected: "one",
		},
		{
			name:     "multiple",
			parts:    []string{"one", "two", "three"},
			sep:      ", ",
			expected: "one, two, three",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerJoinStrings(tt.parts, tt.sep)
			if got != tt.expected {
				t.Errorf("providerJoinStrings() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseTextEditFromMap(t *testing.T) {
	m := map[string]any{
		"newText": "replaced",
		"range": map[string]any{
			"start": map[string]any{
				"line":      float64(10),
				"character": float64(5),
			},
			"end": map[string]any{
				"line":      float64(10),
				"character": float64(15),
			},
		},
	}

	edit := parseTextEditFromMap(m)

	if edit.NewText != "replaced" {
		t.Errorf("expected new text 'replaced', got %q", edit.NewText)
	}

	if edit.Range.Start.Line != 10 {
		t.Errorf("expected start line 10, got %d", edit.Range.Start.Line)
	}

	if edit.Range.Start.Character != 5 {
		t.Errorf("expected start character 5, got %d", edit.Range.Start.Character)
	}
}

func TestParsePositionFromMap(t *testing.T) {
	m := map[string]any{
		"line":      float64(42),
		"character": float64(13),
	}

	pos := parsePositionFromMap(m)

	if pos.Line != 42 {
		t.Errorf("expected line 42, got %d", pos.Line)
	}

	if pos.Character != 13 {
		t.Errorf("expected character 13, got %d", pos.Character)
	}
}

func TestProviderImplementsInterface(t *testing.T) {
	// This test ensures the Provider type implements api.LSPProvider
	var _ api.LSPProvider = (*Provider)(nil)
}

func TestProviderContentCacheConcurrency(t *testing.T) {
	client := NewClient()
	provider := NewProvider(client)

	// Test concurrent access to content cache
	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			provider.SetDocumentContent("/file1.go", "content1")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			provider.SetDocumentContent("/file2.go", "content2")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = provider.getContent("/file1.go")
			_ = provider.getContent("/file2.go")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			provider.ClearDocumentContent("/file1.go")
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}
}
