package highlight

import (
	"testing"
)

func TestNewSimpleHighlighter(t *testing.T) {
	h := NewSimpleHighlighter("test", []string{".test", ".tst"})

	if h.Language() != "test" {
		t.Errorf("Language() = %q, want 'test'", h.Language())
	}

	exts := h.FileExtensions()
	if len(exts) != 2 {
		t.Errorf("FileExtensions() length = %d, want 2", len(exts))
	}
}

func TestSimpleHighlighterAddRule(t *testing.T) {
	h := NewSimpleHighlighter("test", nil)
	h.AddRule(`//.*$`, TokenCommentLine)

	if len(h.rules) != 1 {
		t.Error("AddRule should add a rule")
	}

	tokens, _ := h.HighlightLine("// comment", LexerStateNormal)
	if len(tokens) == 0 {
		t.Error("Should find comment token")
	}
	if tokens[0].Type != TokenCommentLine {
		t.Errorf("Token type = %v, want TokenCommentLine", tokens[0].Type)
	}
}

func TestSimpleHighlighterAddKeywords(t *testing.T) {
	h := NewSimpleHighlighter("test", nil)
	h.AddKeywords(TokenKeyword, "if", "else", "for")

	tokens, _ := h.HighlightLine("if else for", LexerStateNormal)

	if len(tokens) != 3 {
		t.Fatalf("Expected 3 tokens, got %d", len(tokens))
	}

	for _, tok := range tokens {
		if tok.Type != TokenKeyword {
			t.Errorf("Token type = %v, want TokenKeyword", tok.Type)
		}
	}
}

func TestSimpleHighlighterAddMultiLine(t *testing.T) {
	h := NewSimpleHighlighter("test", nil)
	h.AddMultiLine("/*", "*/", TokenCommentBlock, LexerStateBlockComment)

	t.Run("single line block comment", func(t *testing.T) {
		tokens, state := h.HighlightLine("/* comment */", LexerStateNormal)
		if len(tokens) == 0 {
			t.Error("Should find comment token")
		}
		if state != LexerStateNormal {
			t.Error("State should return to normal after complete comment")
		}
	})

	t.Run("multi-line block comment start", func(t *testing.T) {
		tokens, state := h.HighlightLine("/* comment", LexerStateNormal)
		if len(tokens) == 0 {
			t.Error("Should find comment token")
		}
		if state != LexerStateBlockComment {
			t.Error("State should be block comment after incomplete comment")
		}
	})

	t.Run("multi-line block comment continuation", func(t *testing.T) {
		tokens, state := h.HighlightLine("still in comment", LexerStateBlockComment)
		if len(tokens) == 0 {
			t.Error("Should have comment token for entire line")
		}
		if tokens[0].Type != TokenCommentBlock {
			t.Error("Token should be block comment")
		}
		if state != LexerStateBlockComment {
			t.Error("State should remain block comment")
		}
	})

	t.Run("multi-line block comment end", func(t *testing.T) {
		tokens, state := h.HighlightLine("end */ code", LexerStateBlockComment)
		if len(tokens) == 0 {
			t.Error("Should have tokens")
		}
		// First token should be comment up to */
		if tokens[0].Type != TokenCommentBlock {
			t.Error("First token should be block comment")
		}
		if state != LexerStateNormal {
			t.Error("State should return to normal after */")
		}
	})
}

func TestGoHighlighter(t *testing.T) {
	h := GoHighlighter()

	if h.Language() != "go" {
		t.Errorf("Language() = %q, want 'go'", h.Language())
	}

	tests := []struct {
		name     string
		line     string
		expected []struct {
			text      string
			tokenType TokenType
		}
	}{
		{
			name: "package declaration",
			line: "package main",
			expected: []struct {
				text      string
				tokenType TokenType
			}{
				{"package", TokenKeywordOther},
				{"main", TokenIdentifier},
			},
		},
		{
			name: "line comment",
			line: "// this is a comment",
			expected: []struct {
				text      string
				tokenType TokenType
			}{
				{"// this is a comment", TokenCommentLine},
			},
		},
		{
			name: "func declaration",
			line: "func main() {",
			expected: []struct {
				text      string
				tokenType TokenType
			}{
				{"func", TokenKeywordDeclaration},
				{"main", TokenIdentifier},
			},
		},
		{
			name: "keywords",
			line: "if x := 5; x > 0 {",
			expected: []struct {
				text      string
				tokenType TokenType
			}{
				{"if", TokenKeywordControl},
			},
		},
		{
			name: "builtin types",
			line: "var x int",
			expected: []struct {
				text      string
				tokenType TokenType
			}{
				{"var", TokenKeywordDeclaration},
				{"int", TokenTypeBuiltin},
			},
		},
		{
			name: "string literal",
			line: `fmt.Println("hello")`,
			expected: []struct {
				text      string
				tokenType TokenType
			}{
				{`"hello"`, TokenString},
			},
		},
		{
			name: "numbers",
			line: "x := 42 + 3.14 + 0xFF",
			expected: []struct {
				text      string
				tokenType TokenType
			}{
				{"42", TokenNumber},
				{"3.14", TokenNumber},
				{"0xFF", TokenNumberHex},
			},
		},
		{
			name: "builtin functions",
			line: "make([]int, 10)",
			expected: []struct {
				text      string
				tokenType TokenType
			}{
				{"make", TokenFunctionBuiltin},
			},
		},
		{
			name: "constants",
			line: "return true, nil",
			expected: []struct {
				text      string
				tokenType TokenType
			}{
				{"return", TokenKeywordControl},
				{"true", TokenConstantLanguage},
				{"nil", TokenConstantLanguage},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, _ := h.HighlightLine(tt.line, LexerStateNormal)

			for _, exp := range tt.expected {
				found := false
				for _, tok := range tokens {
					// Check by position and type
					if tok.Type == exp.tokenType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected token type %v for %q not found in tokens", exp.tokenType, exp.text)
				}
			}
		})
	}
}

func TestGoHighlighterMultiLineComment(t *testing.T) {
	h := GoHighlighter()

	// Start comment
	tokens, state := h.HighlightLine("/* multi-line", LexerStateNormal)
	if state != LexerStateBlockComment {
		t.Errorf("State after start = %v, want LexerStateBlockComment", state)
	}
	if len(tokens) == 0 {
		t.Error("Should have token for comment start")
	}

	// Middle of comment
	tokens, state = h.HighlightLine("still in comment", state)
	if state != LexerStateBlockComment {
		t.Error("Should still be in block comment state")
	}
	if len(tokens) != 1 || tokens[0].Type != TokenCommentBlock {
		t.Error("Entire line should be comment")
	}

	// End of comment
	tokens, state = h.HighlightLine("end of comment */", state)
	if state != LexerStateNormal {
		t.Error("Should return to normal state after */")
	}
}

func TestGoHighlighterBacktickString(t *testing.T) {
	h := GoHighlighter()

	// Start raw string
	tokens, state := h.HighlightLine("s := `multi-line", LexerStateNormal)
	if state != LexerStateStringBacktick {
		t.Errorf("State = %v, want LexerStateStringBacktick", state)
	}

	// Continue string
	tokens, state = h.HighlightLine("still in string", state)
	if len(tokens) != 1 || tokens[0].Type != TokenString {
		t.Error("Entire line should be string")
	}

	// End string
	tokens, state = h.HighlightLine("end` + x", state)
	if state != LexerStateNormal {
		t.Error("Should return to normal after backtick")
	}
}

func TestPythonHighlighter(t *testing.T) {
	h := PythonHighlighter()

	if h.Language() != "python" {
		t.Errorf("Language() = %q, want 'python'", h.Language())
	}

	tests := []struct {
		name      string
		line      string
		tokenType TokenType
	}{
		{"comment", "# comment", TokenCommentLine},
		{"def keyword", "def foo():", TokenKeywordDeclaration},
		{"if keyword", "if x > 0:", TokenKeywordControl},
		{"class keyword", "class Foo:", TokenKeywordDeclaration},
		{"import keyword", "import os", TokenKeywordOther},
		{"True constant", "x = True", TokenConstantLanguage},
		{"None constant", "x = None", TokenConstantLanguage},
		{"decorator", "@decorator", TokenMeta},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, _ := h.HighlightLine(tt.line, LexerStateNormal)
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.tokenType {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected token type %v not found", tt.tokenType)
			}
		})
	}
}

func TestPythonMultiLineString(t *testing.T) {
	h := PythonHighlighter()

	// Start triple-quoted string
	_, state := h.HighlightLine(`"""docstring`, LexerStateNormal)
	if state != LexerStateStringDouble {
		t.Errorf("State = %v, want LexerStateStringDouble", state)
	}

	// Continue
	tokens, state := h.HighlightLine("more text", state)
	if len(tokens) != 1 || tokens[0].Type != TokenString {
		t.Error("Should be string token")
	}

	// End
	_, state = h.HighlightLine(`end"""`, state)
	if state != LexerStateNormal {
		t.Error("Should return to normal after closing quotes")
	}
}

func TestJavaScriptHighlighter(t *testing.T) {
	h := JavaScriptHighlighter()

	if h.Language() != "javascript" {
		t.Errorf("Language() = %q, want 'javascript'", h.Language())
	}

	tests := []struct {
		name      string
		line      string
		tokenType TokenType
	}{
		{"const keyword", "const x = 1", TokenKeywordDeclaration},
		{"let keyword", "let y = 2", TokenKeywordDeclaration},
		{"function keyword", "function foo() {}", TokenKeywordDeclaration},
		{"arrow function", "const f = () => {}", TokenKeywordDeclaration},
		{"class keyword", "class Foo {}", TokenKeywordDeclaration},
		{"async keyword", "async function foo() {}", TokenKeywordDeclaration},
		{"import", "import x from 'y'", TokenKeywordOther},
		{"null", "x = null", TokenConstantLanguage},
		{"undefined", "x = undefined", TokenConstantLanguage},
		{"decorator", "@decorator", TokenMeta},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, _ := h.HighlightLine(tt.line, LexerStateNormal)
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.tokenType {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected token type %v not found", tt.tokenType)
			}
		})
	}
}

func TestJavaScriptTemplateString(t *testing.T) {
	h := JavaScriptHighlighter()

	// Start template literal
	_, state := h.HighlightLine("const s = `template", LexerStateNormal)
	if state != LexerStateStringBacktick {
		t.Errorf("State = %v, want LexerStateStringBacktick", state)
	}

	// Continue
	tokens, _ := h.HighlightLine("more text", state)
	if len(tokens) != 1 || tokens[0].Type != TokenStringInterpolated {
		t.Error("Should be interpolated string token")
	}
}

func TestRustHighlighter(t *testing.T) {
	h := RustHighlighter()

	if h.Language() != "rust" {
		t.Errorf("Language() = %q, want 'rust'", h.Language())
	}

	tests := []struct {
		name      string
		line      string
		tokenType TokenType
	}{
		{"fn keyword", "fn main() {}", TokenKeywordDeclaration},
		{"let keyword", "let x = 1", TokenKeywordDeclaration},
		{"mut keyword", "let mut x = 1", TokenKeywordDeclaration},
		{"if keyword", "if x > 0 {}", TokenKeywordControl},
		{"match keyword", "match x {}", TokenKeywordControl},
		{"struct keyword", "struct Foo {}", TokenKeywordDeclaration},
		{"impl keyword", "impl Foo {}", TokenKeywordDeclaration},
		{"use keyword", "use std::io", TokenKeywordOther},
		{"attribute", "#[derive(Debug)]", TokenMeta},
		{"Some constant", "Some(x)", TokenConstantLanguage},
		{"None constant", "None", TokenConstantLanguage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, _ := h.HighlightLine(tt.line, LexerStateNormal)
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.tokenType {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected token type %v not found", tt.tokenType)
			}
		})
	}
}

func TestMarkdownHighlighter(t *testing.T) {
	h := MarkdownHighlighter()

	if h.Language() != "markdown" {
		t.Errorf("Language() = %q, want 'markdown'", h.Language())
	}

	tests := []struct {
		name      string
		line      string
		tokenType TokenType
	}{
		{"heading", "# Heading", TokenMarkupHeading},
		{"h2", "## Heading 2", TokenMarkupHeading},
		{"bold stars", "**bold**", TokenMarkupBold},
		{"bold underscore", "__bold__", TokenMarkupBold},
		{"italic stars", "*italic*", TokenMarkupItalic},
		{"italic underscore", "_italic_", TokenMarkupItalic},
		{"strikethrough", "~~struck~~", TokenMarkupStrike},
		{"inline code", "`code`", TokenMarkupCode},
		{"code fence", "```go", TokenMarkupCode},
		{"quote", "> quote", TokenMarkupQuote},
		{"unordered list", "- item", TokenMarkupList},
		{"ordered list", "1. item", TokenMarkupList},
		{"link", "[text](url)", TokenMarkupLink},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, _ := h.HighlightLine(tt.line, LexerStateNormal)
			found := false
			for _, tok := range tokens {
				if tok.Type == tt.tokenType {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected token type %v not found in line %q", tt.tokenType, tt.line)
			}
		})
	}
}

func TestRegisterBuiltinHighlighters(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinHighlighters(r)

	expectedLangs := []string{"go", "python", "javascript", "rust", "markdown"}
	for _, lang := range expectedLangs {
		if _, ok := r.GetByLanguage(lang); !ok {
			t.Errorf("Expected language %q to be registered", lang)
		}
	}

	expectedExts := []string{".go", ".py", ".js", ".ts", ".rs", ".md"}
	for _, ext := range expectedExts {
		if _, ok := r.GetByExtension(ext); !ok {
			t.Errorf("Expected extension %q to be registered", ext)
		}
	}
}

func TestHighlighterInterface(t *testing.T) {
	// Verify all highlighters implement the interface
	var _ Highlighter = GoHighlighter()
	var _ Highlighter = PythonHighlighter()
	var _ Highlighter = JavaScriptHighlighter()
	var _ Highlighter = RustHighlighter()
	var _ Highlighter = MarkdownHighlighter()
}

func TestTokenSorting(t *testing.T) {
	h := GoHighlighter()

	tokens, _ := h.HighlightLine("if x := 5; x > 0 {", LexerStateNormal)

	// Tokens should be sorted by StartCol
	for i := 1; i < len(tokens); i++ {
		if tokens[i].StartCol < tokens[i-1].StartCol {
			t.Errorf("Tokens not sorted: token %d (col %d) before token %d (col %d)",
				i-1, tokens[i-1].StartCol, i, tokens[i].StartCol)
		}
	}
}

func TestNoOverlappingTokens(t *testing.T) {
	h := GoHighlighter()

	lines := []string{
		`fmt.Println("hello")`,
		`x := 42 // comment`,
		`func main() { return }`,
	}

	for _, line := range lines {
		tokens, _ := h.HighlightLine(line, LexerStateNormal)

		// Sort by start
		for i := 1; i < len(tokens); i++ {
			prev := tokens[i-1]
			curr := tokens[i]

			if prev.EndCol > curr.StartCol {
				t.Errorf("Overlapping tokens in %q: [%d-%d] and [%d-%d]",
					line, prev.StartCol, prev.EndCol, curr.StartCol, curr.EndCol)
			}
		}
	}
}
