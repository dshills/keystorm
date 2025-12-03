package highlight

import (
	"testing"
)

func TestTokenTypeString(t *testing.T) {
	tests := []struct {
		tokenType TokenType
		expected  string
	}{
		{TokenNone, "none"},
		{TokenComment, "comment"},
		{TokenCommentLine, "comment.line"},
		{TokenCommentBlock, "comment.block"},
		{TokenString, "string"},
		{TokenKeyword, "keyword"},
		{TokenKeywordControl, "keyword.control"},
		{TokenFunction, "function"},
		{TokenTypeName, "type"},
		{TokenTypeBuiltin, "type.builtin"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.tokenType.String(); got != tt.expected {
				t.Errorf("TokenType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTokenTypeCategories(t *testing.T) {
	tests := []struct {
		name      string
		tokenType TokenType
		isComment bool
		isString  bool
		isNumber  bool
		isKeyword bool
		isFunc    bool
		isType    bool
	}{
		{"comment", TokenComment, true, false, false, false, false, false},
		{"comment.line", TokenCommentLine, true, false, false, false, false, false},
		{"string", TokenString, false, true, false, false, false, false},
		{"string.escape", TokenStringEscape, false, true, false, false, false, false},
		{"number", TokenNumber, false, false, true, false, false, false},
		{"number.hex", TokenNumberHex, false, false, true, false, false, false},
		{"keyword", TokenKeyword, false, false, false, true, false, false},
		{"keyword.control", TokenKeywordControl, false, false, false, true, false, false},
		{"function", TokenFunction, false, false, false, false, true, false},
		{"function.builtin", TokenFunctionBuiltin, false, false, false, false, true, false},
		{"type", TokenTypeName, false, false, false, false, false, true},
		{"type.builtin", TokenTypeBuiltin, false, false, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tokenType.IsComment(); got != tt.isComment {
				t.Errorf("IsComment() = %v, want %v", got, tt.isComment)
			}
			if got := tt.tokenType.IsString(); got != tt.isString {
				t.Errorf("IsString() = %v, want %v", got, tt.isString)
			}
			if got := tt.tokenType.IsNumber(); got != tt.isNumber {
				t.Errorf("IsNumber() = %v, want %v", got, tt.isNumber)
			}
			if got := tt.tokenType.IsKeyword(); got != tt.isKeyword {
				t.Errorf("IsKeyword() = %v, want %v", got, tt.isKeyword)
			}
			if got := tt.tokenType.IsFunction(); got != tt.isFunc {
				t.Errorf("IsFunction() = %v, want %v", got, tt.isFunc)
			}
			if got := tt.tokenType.IsType(); got != tt.isType {
				t.Errorf("IsType() = %v, want %v", got, tt.isType)
			}
		})
	}
}

func TestToken(t *testing.T) {
	tok := Token{
		Type:     TokenKeyword,
		StartCol: 5,
		EndCol:   10,
		Text:     "func",
	}

	t.Run("Len", func(t *testing.T) {
		if got := tok.Len(); got != 5 {
			t.Errorf("Token.Len() = %v, want 5", got)
		}
	})

	t.Run("Contains", func(t *testing.T) {
		tests := []struct {
			col      uint32
			expected bool
		}{
			{4, false},
			{5, true},
			{7, true},
			{9, true},
			{10, false},
			{11, false},
		}

		for _, tt := range tests {
			if got := tok.Contains(tt.col); got != tt.expected {
				t.Errorf("Token.Contains(%d) = %v, want %v", tt.col, got, tt.expected)
			}
		}
	})
}

func TestTokenLine(t *testing.T) {
	tl := TokenLine{
		Line: 0,
		Tokens: []Token{
			{Type: TokenKeyword, StartCol: 0, EndCol: 4},
			{Type: TokenIdentifier, StartCol: 5, EndCol: 9},
			{Type: TokenPunctuation, StartCol: 9, EndCol: 10},
		},
	}

	tests := []struct {
		col       uint32
		wantType  TokenType
		wantFound bool
	}{
		{0, TokenKeyword, true},
		{2, TokenKeyword, true},
		{4, TokenNone, false}, // Between tokens
		{5, TokenIdentifier, true},
		{8, TokenIdentifier, true},
		{9, TokenPunctuation, true},
		{10, TokenNone, false},
		{100, TokenNone, false},
	}

	for _, tt := range tests {
		tok, found := tl.TokenAt(tt.col)
		if found != tt.wantFound {
			t.Errorf("TokenLine.TokenAt(%d) found = %v, want %v", tt.col, found, tt.wantFound)
		}
		if found && tok.Type != tt.wantType {
			t.Errorf("TokenLine.TokenAt(%d) type = %v, want %v", tt.col, tok.Type, tt.wantType)
		}
	}
}

func TestTokenTypeFromString(t *testing.T) {
	tests := []struct {
		scope    string
		expected TokenType
	}{
		{"comment", TokenComment},
		{"comment.line", TokenCommentLine},
		{"comment.block", TokenCommentBlock},
		{"keyword", TokenKeyword},
		{"keyword.control", TokenKeywordControl},
		{"string", TokenString},
		{"function", TokenFunction},
		{"type", TokenTypeName},
		{"type.builtin", TokenTypeBuiltin},
		{"nonexistent", TokenNone},
		{"comment.line.double-slash.go", TokenCommentLine}, // Should match parent
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			if got := TokenTypeFromString(tt.scope); got != tt.expected {
				t.Errorf("TokenTypeFromString(%q) = %v, want %v", tt.scope, got, tt.expected)
			}
		})
	}
}

func TestTokenTypeScope(t *testing.T) {
	tests := []struct {
		tokenType TokenType
		expected  string
	}{
		{TokenComment, "comment"},
		{TokenKeyword, "keyword"},
		{TokenString, "string"},
		{TokenFunction, "function"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.tokenType.Scope(); got != tt.expected {
				t.Errorf("TokenType.Scope() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLexerState(t *testing.T) {
	// Verify lexer states are distinct
	states := []LexerState{
		LexerStateNormal,
		LexerStateBlockComment,
		LexerStateBlockCommentDoc,
		LexerStateStringDouble,
		LexerStateStringSingle,
		LexerStateStringBacktick,
		LexerStateStringRaw,
		LexerStateStringHeredoc,
	}

	seen := make(map[LexerState]bool)
	for _, s := range states {
		if seen[s] {
			t.Errorf("Duplicate lexer state: %v", s)
		}
		seen[s] = true
	}
}
