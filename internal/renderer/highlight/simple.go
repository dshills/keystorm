package highlight

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Rule defines a highlighting rule.
type Rule struct {
	// Pattern is the regex pattern to match.
	Pattern *regexp.Regexp

	// TokenType is the type to assign to matches.
	TokenType TokenType

	// Submatch is the submatch index to use (0 for whole match).
	Submatch int
}

// SimpleHighlighter is a simple regex-based syntax highlighter.
type SimpleHighlighter struct {
	language   string
	extensions []string
	rules      []Rule
	keywords   map[string]TokenType
	multiLine  map[string]multiLineRule
}

// multiLineRule defines rules for multi-line constructs.
type multiLineRule struct {
	start     string
	end       string
	tokenType TokenType
	state     LexerState
}

// NewSimpleHighlighter creates a new simple highlighter.
func NewSimpleHighlighter(language string, extensions []string) *SimpleHighlighter {
	return &SimpleHighlighter{
		language:   language,
		extensions: extensions,
		rules:      make([]Rule, 0),
		keywords:   make(map[string]TokenType),
		multiLine:  make(map[string]multiLineRule),
	}
}

// AddRule adds a highlighting rule.
func (h *SimpleHighlighter) AddRule(pattern string, tokenType TokenType) *SimpleHighlighter {
	re := regexp.MustCompile(pattern)
	h.rules = append(h.rules, Rule{
		Pattern:   re,
		TokenType: tokenType,
	})
	return h
}

// AddKeywords adds keywords with a specific token type.
func (h *SimpleHighlighter) AddKeywords(tokenType TokenType, keywords ...string) *SimpleHighlighter {
	for _, kw := range keywords {
		h.keywords[kw] = tokenType
	}
	return h
}

// AddMultiLine adds a multi-line construct rule.
func (h *SimpleHighlighter) AddMultiLine(start, end string, tokenType TokenType, state LexerState) *SimpleHighlighter {
	h.multiLine[start] = multiLineRule{
		start:     start,
		end:       end,
		tokenType: tokenType,
		state:     state,
	}
	return h
}

// Language returns the language name.
func (h *SimpleHighlighter) Language() string {
	return h.language
}

// FileExtensions returns the supported file extensions.
func (h *SimpleHighlighter) FileExtensions() []string {
	return h.extensions
}

// HighlightLine tokenizes a single line.
func (h *SimpleHighlighter) HighlightLine(line string, prevState LexerState) ([]Token, LexerState) {
	tokens := make([]Token, 0)
	state := prevState

	// Handle continuation of multi-line constructs
	if state != LexerStateNormal {
		endIdx, found := h.findMultiLineEnd(line, state)
		if found {
			tokens = append(tokens, Token{
				Type:     h.tokenTypeForState(state),
				StartCol: 0,
				EndCol:   uint32(endIdx),
			})
			line = line[endIdx:]
			state = LexerStateNormal
			if len(line) == 0 {
				return tokens, state
			}
			// Adjust subsequent token positions
			offset := uint32(endIdx)
			subTokens, newState := h.highlightNormal(line)
			for i := range subTokens {
				subTokens[i].StartCol += offset
				subTokens[i].EndCol += offset
			}
			tokens = append(tokens, subTokens...)
			return tokens, newState
		}
		// Entire line is part of multi-line construct
		return []Token{{
			Type:     h.tokenTypeForState(state),
			StartCol: 0,
			EndCol:   uint32(len(line)),
		}}, state
	}

	return h.highlightNormal(line)
}

// highlightNormal highlights a line in normal state.
func (h *SimpleHighlighter) highlightNormal(line string) ([]Token, LexerState) {
	tokens := make([]Token, 0)
	covered := make([]bool, len(line))
	state := LexerStateNormal

	// Check for multi-line construct starts
	for start, rule := range h.multiLine {
		idx := strings.Index(line, start)
		if idx >= 0 && !h.isCovered(covered, idx, idx+len(start)) {
			endIdx := strings.Index(line[idx+len(start):], rule.end)
			if endIdx >= 0 {
				// Entire construct is on this line
				endPos := idx + len(start) + endIdx + len(rule.end)
				tokens = append(tokens, Token{
					Type:     rule.tokenType,
					StartCol: uint32(idx),
					EndCol:   uint32(endPos),
				})
				h.markCovered(covered, idx, endPos)
			} else {
				// Construct continues to next line
				tokens = append(tokens, Token{
					Type:     rule.tokenType,
					StartCol: uint32(idx),
					EndCol:   uint32(len(line)),
				})
				h.markCovered(covered, idx, len(line))
				state = rule.state
			}
		}
	}

	// Apply regex rules
	for _, rule := range h.rules {
		matches := rule.Pattern.FindAllStringSubmatchIndex(line, -1)
		for _, match := range matches {
			start := match[0]
			end := match[1]
			if rule.Submatch > 0 && len(match) > rule.Submatch*2+1 {
				start = match[rule.Submatch*2]
				end = match[rule.Submatch*2+1]
			}
			if start >= 0 && end > start && !h.isCovered(covered, start, end) {
				tokens = append(tokens, Token{
					Type:     rule.TokenType,
					StartCol: uint32(start),
					EndCol:   uint32(end),
				})
				h.markCovered(covered, start, end)
			}
		}
	}

	// Find identifiers and check for keywords
	tokens = append(tokens, h.findIdentifiers(line, covered)...)

	// Sort tokens by position
	h.sortTokens(tokens)

	return tokens, state
}

// findMultiLineEnd finds the end of a multi-line construct.
func (h *SimpleHighlighter) findMultiLineEnd(line string, state LexerState) (int, bool) {
	for _, rule := range h.multiLine {
		if rule.state == state {
			idx := strings.Index(line, rule.end)
			if idx >= 0 {
				return idx + len(rule.end), true
			}
			return 0, false
		}
	}
	return 0, false
}

// tokenTypeForState returns the token type for a lexer state.
func (h *SimpleHighlighter) tokenTypeForState(state LexerState) TokenType {
	for _, rule := range h.multiLine {
		if rule.state == state {
			return rule.tokenType
		}
	}
	return TokenNone
}

// findIdentifiers finds identifiers in the line and checks for keywords.
func (h *SimpleHighlighter) findIdentifiers(line string, covered []bool) []Token {
	tokens := make([]Token, 0)

	i := 0
	for i < len(line) {
		// Skip covered regions
		if covered[i] {
			i++
			continue
		}

		// Check for identifier start
		r := rune(line[i])
		if unicode.IsLetter(r) || r == '_' {
			start := i
			for i < len(line) {
				r = rune(line[i])
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
					break
				}
				i++
			}
			end := i

			// Check if any part is covered
			partCovered := false
			for j := start; j < end; j++ {
				if covered[j] {
					partCovered = true
					break
				}
			}

			if !partCovered {
				word := line[start:end]
				tokenType := TokenIdentifier
				if kwType, ok := h.keywords[word]; ok {
					tokenType = kwType
				}
				tokens = append(tokens, Token{
					Type:     tokenType,
					StartCol: uint32(start),
					EndCol:   uint32(end),
				})
				h.markCovered(covered, start, end)
			}
		} else {
			i++
		}
	}

	return tokens
}

// isCovered checks if a range is already covered.
func (h *SimpleHighlighter) isCovered(covered []bool, start, end int) bool {
	if start < 0 || start >= len(covered) {
		return false
	}
	for i := start; i < end && i < len(covered); i++ {
		if covered[i] {
			return true
		}
	}
	return false
}

// markCovered marks a range as covered.
func (h *SimpleHighlighter) markCovered(covered []bool, start, end int) {
	if start < 0 {
		start = 0
	}
	for i := start; i < end && i < len(covered); i++ {
		covered[i] = true
	}
}

// sortTokens sorts tokens by start position.
func (h *SimpleHighlighter) sortTokens(tokens []Token) {
	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].StartCol < tokens[j].StartCol
	})
}

// GoHighlighter returns a highlighter for Go.
func GoHighlighter() *SimpleHighlighter {
	h := NewSimpleHighlighter("go", []string{".go"})

	// Multi-line constructs
	h.AddMultiLine("/*", "*/", TokenCommentBlock, LexerStateBlockComment)
	h.AddMultiLine("`", "`", TokenString, LexerStateStringBacktick)

	// Single-line patterns
	h.AddRule(`//.*$`, TokenCommentLine)
	h.AddRule(`"(?:[^"\\]|\\.)*"`, TokenString)
	h.AddRule(`'(?:[^'\\]|\\.)'`, TokenString)
	h.AddRule(`\b0[xX][0-9a-fA-F]+\b`, TokenNumberHex)
	h.AddRule(`\b0[oO][0-7]+\b`, TokenNumberOctal)
	h.AddRule(`\b0[bB][01]+\b`, TokenNumberBinary)
	h.AddRule(`\b\d+\.?\d*(?:[eE][+-]?\d+)?\b`, TokenNumber)

	// Keywords
	h.AddKeywords(TokenKeywordControl,
		"if", "else", "for", "range", "switch", "case", "default",
		"break", "continue", "return", "goto", "fallthrough", "select")
	h.AddKeywords(TokenKeywordDeclaration,
		"func", "var", "const", "type", "struct", "interface", "map", "chan")
	h.AddKeywords(TokenKeywordOther,
		"package", "import", "defer", "go")
	h.AddKeywords(TokenConstantLanguage,
		"true", "false", "nil", "iota")
	h.AddKeywords(TokenTypeBuiltin,
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128",
		"bool", "byte", "rune", "string", "error", "any")
	h.AddKeywords(TokenFunctionBuiltin,
		"make", "new", "len", "cap", "append", "copy", "delete",
		"close", "panic", "recover", "print", "println",
		"real", "imag", "complex", "min", "max", "clear")

	return h
}

// PythonHighlighter returns a highlighter for Python.
func PythonHighlighter() *SimpleHighlighter {
	h := NewSimpleHighlighter("python", []string{".py", ".pyw", ".pyi"})

	// Multi-line strings
	h.AddMultiLine(`"""`, `"""`, TokenString, LexerStateStringDouble)
	h.AddMultiLine(`'''`, `'''`, TokenString, LexerStateStringSingle)

	// Single-line patterns
	h.AddRule(`#.*$`, TokenCommentLine)
	h.AddRule(`"(?:[^"\\]|\\.)*"`, TokenString)
	h.AddRule(`'(?:[^'\\]|\\.)*'`, TokenString)
	h.AddRule(`\b0[xX][0-9a-fA-F]+\b`, TokenNumberHex)
	h.AddRule(`\b0[oO][0-7]+\b`, TokenNumberOctal)
	h.AddRule(`\b0[bB][01]+\b`, TokenNumberBinary)
	h.AddRule(`\b\d+\.?\d*(?:[eE][+-]?\d+)?j?\b`, TokenNumber)
	h.AddRule(`@\w+`, TokenMeta) // Decorators

	// Keywords
	h.AddKeywords(TokenKeywordControl,
		"if", "elif", "else", "for", "while", "break", "continue",
		"return", "try", "except", "finally", "raise", "with", "as",
		"match", "case")
	h.AddKeywords(TokenKeywordDeclaration,
		"def", "class", "lambda", "async", "await")
	h.AddKeywords(TokenKeywordOther,
		"import", "from", "global", "nonlocal", "pass", "yield",
		"assert", "del", "in", "is", "not", "and", "or")
	h.AddKeywords(TokenConstantLanguage,
		"True", "False", "None")
	h.AddKeywords(TokenTypeBuiltin,
		"int", "float", "str", "bool", "list", "dict", "set", "tuple",
		"bytes", "bytearray", "complex", "frozenset", "type", "object")
	h.AddKeywords(TokenFunctionBuiltin,
		"print", "len", "range", "enumerate", "zip", "map", "filter",
		"open", "input", "isinstance", "issubclass", "hasattr", "getattr",
		"setattr", "delattr", "callable", "iter", "next", "sorted", "reversed",
		"sum", "min", "max", "abs", "round", "pow", "divmod", "all", "any",
		"format", "repr", "str", "int", "float", "bool", "list", "dict", "set",
		"tuple", "bytes", "bytearray", "id", "hash", "dir", "vars", "locals",
		"globals", "type", "super", "property", "staticmethod", "classmethod")

	return h
}

// JavaScriptHighlighter returns a highlighter for JavaScript/TypeScript.
func JavaScriptHighlighter() *SimpleHighlighter {
	h := NewSimpleHighlighter("javascript", []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"})

	// Multi-line constructs
	h.AddMultiLine("/*", "*/", TokenCommentBlock, LexerStateBlockComment)
	h.AddMultiLine("`", "`", TokenStringInterpolated, LexerStateStringBacktick)

	// Single-line patterns
	h.AddRule(`//.*$`, TokenCommentLine)
	h.AddRule(`"(?:[^"\\]|\\.)*"`, TokenString)
	h.AddRule(`'(?:[^'\\]|\\.)*'`, TokenString)
	h.AddRule(`/(?:[^/\\]|\\.)+/[gimsuy]*`, TokenStringRegexp)
	h.AddRule(`\b0[xX][0-9a-fA-F]+\b`, TokenNumberHex)
	h.AddRule(`\b0[oO][0-7]+\b`, TokenNumberOctal)
	h.AddRule(`\b0[bB][01]+\b`, TokenNumberBinary)
	h.AddRule(`\b\d+\.?\d*(?:[eE][+-]?\d+)?\b`, TokenNumber)
	h.AddRule(`@\w+`, TokenMeta) // Decorators

	// Keywords
	h.AddKeywords(TokenKeywordControl,
		"if", "else", "for", "while", "do", "switch", "case", "default",
		"break", "continue", "return", "throw", "try", "catch", "finally")
	h.AddKeywords(TokenKeywordDeclaration,
		"function", "var", "let", "const", "class", "extends", "async", "await",
		"type", "interface", "enum", "namespace", "module", "declare")
	h.AddKeywords(TokenKeywordOther,
		"import", "export", "from", "as", "default", "new", "delete",
		"typeof", "instanceof", "in", "of", "this", "super", "static",
		"get", "set", "yield", "debugger", "with")
	h.AddKeywords(TokenConstantLanguage,
		"true", "false", "null", "undefined", "NaN", "Infinity")
	h.AddKeywords(TokenStorageModifier,
		"public", "private", "protected", "readonly", "abstract", "override")

	return h
}

// RustHighlighter returns a highlighter for Rust.
func RustHighlighter() *SimpleHighlighter {
	h := NewSimpleHighlighter("rust", []string{".rs"})

	// Multi-line constructs
	h.AddMultiLine("/*", "*/", TokenCommentBlock, LexerStateBlockComment)

	// Single-line patterns
	h.AddRule(`//.*$`, TokenCommentLine)
	h.AddRule(`"(?:[^"\\]|\\.)*"`, TokenString)
	h.AddRule(`'(?:[^'\\]|\\.)*'`, TokenString)
	h.AddRule(`r#*"[^"]*"#*`, TokenString)       // Raw strings
	h.AddRule(`b"(?:[^"\\]|\\.)*"`, TokenString) // Byte strings
	h.AddRule(`\b0[xX][0-9a-fA-F_]+\b`, TokenNumberHex)
	h.AddRule(`\b0[oO][0-7_]+\b`, TokenNumberOctal)
	h.AddRule(`\b0[bB][01_]+\b`, TokenNumberBinary)
	h.AddRule(`\b\d[\d_]*\.?[\d_]*(?:[eE][+-]?[\d_]+)?(?:f32|f64|i\d+|u\d+|isize|usize)?\b`, TokenNumber)
	h.AddRule(`#\[.*?\]`, TokenMeta)  // Attributes
	h.AddRule(`#!\[.*?\]`, TokenMeta) // Inner attributes

	// Keywords
	h.AddKeywords(TokenKeywordControl,
		"if", "else", "match", "for", "while", "loop", "break", "continue",
		"return", "yield")
	h.AddKeywords(TokenKeywordDeclaration,
		"fn", "let", "mut", "const", "static", "struct", "enum", "trait",
		"impl", "type", "mod", "macro_rules")
	h.AddKeywords(TokenKeywordOther,
		"use", "crate", "super", "self", "Self", "pub", "where", "as",
		"async", "await", "dyn", "move", "ref", "unsafe", "extern")
	h.AddKeywords(TokenConstantLanguage,
		"true", "false", "None", "Some", "Ok", "Err")
	h.AddKeywords(TokenTypeBuiltin,
		"i8", "i16", "i32", "i64", "i128", "isize",
		"u8", "u16", "u32", "u64", "u128", "usize",
		"f32", "f64", "bool", "char", "str", "String",
		"Vec", "Box", "Option", "Result")
	h.AddKeywords(TokenFunctionBuiltin,
		"println", "print", "format", "panic", "assert", "debug_assert",
		"todo", "unimplemented", "unreachable")

	return h
}

// MarkdownHighlighter returns a highlighter for Markdown.
func MarkdownHighlighter() *SimpleHighlighter {
	h := NewSimpleHighlighter("markdown", []string{".md", ".markdown"})

	// Patterns (order matters - more specific first)
	h.AddRule("^#{1,6}\\s+.*$", TokenMarkupHeading)
	h.AddRule("\\*\\*[^*]+\\*\\*", TokenMarkupBold)
	h.AddRule("__[^_]+__", TokenMarkupBold)
	h.AddRule("\\*[^*]+\\*", TokenMarkupItalic)
	h.AddRule("_[^_]+_", TokenMarkupItalic)
	h.AddRule("~~[^~]+~~", TokenMarkupStrike)
	h.AddRule("`[^`]+`", TokenMarkupCode)
	h.AddRule("^```.*$", TokenMarkupCode)
	h.AddRule("^>\\s+.*$", TokenMarkupQuote)
	h.AddRule("^\\s*[-*+]\\s+", TokenMarkupList)
	h.AddRule("^\\s*\\d+\\.\\s+", TokenMarkupList)
	h.AddRule("\\[([^\\]]+)\\]\\(([^)]+)\\)", TokenMarkupLink)

	return h
}

// RegisterBuiltinHighlighters registers all built-in highlighters.
func RegisterBuiltinHighlighters(r *Registry) {
	r.Register(GoHighlighter())
	r.Register(PythonHighlighter())
	r.Register(JavaScriptHighlighter())
	r.Register(RustHighlighter())
	r.Register(MarkdownHighlighter())
}
