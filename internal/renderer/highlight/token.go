// Package highlight provides syntax highlighting for the renderer.
package highlight

// TokenType represents the semantic type of a token.
type TokenType uint16

// Token types for syntax highlighting.
// These follow TextMate/VS Code scope naming conventions at a high level.
const (
	TokenNone TokenType = iota

	// Comments
	TokenComment
	TokenCommentLine
	TokenCommentBlock
	TokenCommentDoc

	// Strings
	TokenString
	TokenStringQuoted
	TokenStringInterpolated
	TokenStringRegexp
	TokenStringEscape

	// Numbers
	TokenNumber
	TokenNumberInteger
	TokenNumberFloat
	TokenNumberHex
	TokenNumberOctal
	TokenNumberBinary

	// Keywords
	TokenKeyword
	TokenKeywordControl     // if, else, for, while, switch, case, return, break, continue
	TokenKeywordOperator    // new, delete, typeof, instanceof
	TokenKeywordOther       // package, import, export, from
	TokenKeywordDeclaration // var, let, const, func, type, struct, interface

	// Operators and punctuation
	TokenOperator
	TokenOperatorAssignment
	TokenOperatorComparison
	TokenOperatorArithmetic
	TokenOperatorLogical
	TokenPunctuation
	TokenPunctuationBracket
	TokenPunctuationDelimiter

	// Identifiers
	TokenIdentifier
	TokenVariable
	TokenVariableParameter
	TokenVariableOther
	TokenConstant
	TokenConstantLanguage // true, false, nil, null

	// Functions
	TokenFunction
	TokenFunctionDeclaration
	TokenFunctionCall
	TokenFunctionMethod
	TokenFunctionBuiltin

	// Types
	TokenTypeName
	TokenTypeBuiltin   // int, string, bool, etc.
	TokenTypeClass     // class names
	TokenTypeInterface // interface names
	TokenTypeStruct    // struct names
	TokenTypeEnum      // enum names
	TokenTypeParameter // generic type parameters

	// Storage
	TokenStorage
	TokenStorageType     // class, struct, enum, interface
	TokenStorageModifier // public, private, static, const

	// Support
	TokenSupport
	TokenSupportFunction
	TokenSupportClass
	TokenSupportType
	TokenSupportConstant
	TokenSupportVariable

	// Markup (for markdown, HTML, etc.)
	TokenMarkup
	TokenMarkupHeading
	TokenMarkupBold
	TokenMarkupItalic
	TokenMarkupUnderline
	TokenMarkupStrike
	TokenMarkupQuote
	TokenMarkupList
	TokenMarkupLink
	TokenMarkupCode
	TokenMarkupRaw

	// Invalid/Error
	TokenInvalid
	TokenInvalidDeprecated
	TokenInvalidIllegal

	// Special
	TokenMeta      // Meta information (e.g., preprocessor)
	TokenTag       // HTML/XML tags
	TokenAttribute // HTML/XML attributes
	TokenNamespace // Namespace identifiers
	TokenLabel     // Labels (goto targets, etc.)

	// Editor-specific (not for syntax, for UI hints)
	TokenEditorWhitespace
	TokenEditorIndentGuide
	TokenEditorLineNumber
	TokenEditorSelection
	TokenEditorCursor

	// Sentinel for iteration
	tokenTypeCount
)

// String returns the string representation of a token type.
func (t TokenType) String() string {
	if int(t) < len(tokenTypeNames) {
		return tokenTypeNames[t]
	}
	return "unknown"
}

// IsComment returns true if this is a comment token.
func (t TokenType) IsComment() bool {
	return t >= TokenComment && t <= TokenCommentDoc
}

// IsString returns true if this is a string token.
func (t TokenType) IsString() bool {
	return t >= TokenString && t <= TokenStringEscape
}

// IsNumber returns true if this is a number token.
func (t TokenType) IsNumber() bool {
	return t >= TokenNumber && t <= TokenNumberBinary
}

// IsKeyword returns true if this is a keyword token.
func (t TokenType) IsKeyword() bool {
	return t >= TokenKeyword && t <= TokenKeywordDeclaration
}

// IsOperator returns true if this is an operator or punctuation token.
func (t TokenType) IsOperator() bool {
	return t >= TokenOperator && t <= TokenPunctuationDelimiter
}

// IsIdentifier returns true if this is an identifier-like token.
func (t TokenType) IsIdentifier() bool {
	return t >= TokenIdentifier && t <= TokenConstantLanguage
}

// IsFunction returns true if this is a function-related token.
func (t TokenType) IsFunction() bool {
	return t >= TokenFunction && t <= TokenFunctionBuiltin
}

// IsType returns true if this is a type-related token.
func (t TokenType) IsType() bool {
	return t >= TokenTypeName && t <= TokenTypeParameter
}

// Token represents a highlighted token in source code.
type Token struct {
	// Type is the semantic type of the token.
	Type TokenType

	// StartCol is the starting column (0-indexed, buffer coordinates).
	StartCol uint32

	// EndCol is the ending column (exclusive).
	EndCol uint32

	// Text is the actual text of the token (optional, for debugging).
	Text string
}

// Len returns the length of the token.
func (t Token) Len() uint32 {
	return t.EndCol - t.StartCol
}

// Contains returns true if the column is within the token.
func (t Token) Contains(col uint32) bool {
	return col >= t.StartCol && col < t.EndCol
}

// TokenLine represents all tokens on a single line.
type TokenLine struct {
	// Line is the line number (0-indexed).
	Line uint32

	// Tokens are the tokens on this line, sorted by StartCol.
	Tokens []Token

	// State is the lexer state at the end of this line.
	// Used for multi-line constructs like block comments and strings.
	State LexerState
}

// LexerState represents the lexer's state for continuation across lines.
type LexerState uint32

// Common lexer states.
const (
	LexerStateNormal LexerState = iota
	LexerStateBlockComment
	LexerStateBlockCommentDoc
	LexerStateStringDouble
	LexerStateStringSingle
	LexerStateStringBacktick
	LexerStateStringRaw
	LexerStateStringHeredoc
)

// TokenAt returns the token at the given column, if any.
func (tl TokenLine) TokenAt(col uint32) (Token, bool) {
	for _, tok := range tl.Tokens {
		if tok.Contains(col) {
			return tok, true
		}
		if tok.StartCol > col {
			break // Tokens are sorted, no need to continue
		}
	}
	return Token{}, false
}

// TokenTypeFromString converts a scope string to a TokenType.
// Supports TextMate-style scope names like "comment.line", "keyword.control".
func TokenTypeFromString(scope string) TokenType {
	if t, ok := scopeToToken[scope]; ok {
		return t
	}
	// Try prefix matching for hierarchical scopes
	for len(scope) > 0 {
		if t, ok := scopeToToken[scope]; ok {
			return t
		}
		// Remove last segment
		for i := len(scope) - 1; i >= 0; i-- {
			if scope[i] == '.' {
				scope = scope[:i]
				break
			}
			if i == 0 {
				scope = ""
			}
		}
	}
	return TokenNone
}

// Scope returns the TextMate-style scope name for this token type.
func (t TokenType) Scope() string {
	if int(t) < len(tokenTypeScopes) {
		return tokenTypeScopes[t]
	}
	return ""
}

// tokenTypeNames maps token types to their string names.
var tokenTypeNames = []string{
	TokenNone: "none",

	TokenComment:      "comment",
	TokenCommentLine:  "comment.line",
	TokenCommentBlock: "comment.block",
	TokenCommentDoc:   "comment.block.documentation",

	TokenString:             "string",
	TokenStringQuoted:       "string.quoted",
	TokenStringInterpolated: "string.interpolated",
	TokenStringRegexp:       "string.regexp",
	TokenStringEscape:       "string.escape",

	TokenNumber:        "number",
	TokenNumberInteger: "number.integer",
	TokenNumberFloat:   "number.float",
	TokenNumberHex:     "number.hex",
	TokenNumberOctal:   "number.octal",
	TokenNumberBinary:  "number.binary",

	TokenKeyword:            "keyword",
	TokenKeywordControl:     "keyword.control",
	TokenKeywordOperator:    "keyword.operator",
	TokenKeywordOther:       "keyword.other",
	TokenKeywordDeclaration: "keyword.declaration",

	TokenOperator:             "operator",
	TokenOperatorAssignment:   "operator.assignment",
	TokenOperatorComparison:   "operator.comparison",
	TokenOperatorArithmetic:   "operator.arithmetic",
	TokenOperatorLogical:      "operator.logical",
	TokenPunctuation:          "punctuation",
	TokenPunctuationBracket:   "punctuation.bracket",
	TokenPunctuationDelimiter: "punctuation.delimiter",

	TokenIdentifier:        "identifier",
	TokenVariable:          "variable",
	TokenVariableParameter: "variable.parameter",
	TokenVariableOther:     "variable.other",
	TokenConstant:          "constant",
	TokenConstantLanguage:  "constant.language",

	TokenFunction:            "function",
	TokenFunctionDeclaration: "function.declaration",
	TokenFunctionCall:        "function.call",
	TokenFunctionMethod:      "function.method",
	TokenFunctionBuiltin:     "function.builtin",

	TokenTypeName:      "type",
	TokenTypeBuiltin:   "type.builtin",
	TokenTypeClass:     "type.class",
	TokenTypeInterface: "type.interface",
	TokenTypeStruct:    "type.struct",
	TokenTypeEnum:      "type.enum",
	TokenTypeParameter: "type.parameter",

	TokenStorage:         "storage",
	TokenStorageType:     "storage.type",
	TokenStorageModifier: "storage.modifier",

	TokenSupport:         "support",
	TokenSupportFunction: "support.function",
	TokenSupportClass:    "support.class",
	TokenSupportType:     "support.type",
	TokenSupportConstant: "support.constant",
	TokenSupportVariable: "support.variable",

	TokenMarkup:          "markup",
	TokenMarkupHeading:   "markup.heading",
	TokenMarkupBold:      "markup.bold",
	TokenMarkupItalic:    "markup.italic",
	TokenMarkupUnderline: "markup.underline",
	TokenMarkupStrike:    "markup.strike",
	TokenMarkupQuote:     "markup.quote",
	TokenMarkupList:      "markup.list",
	TokenMarkupLink:      "markup.link",
	TokenMarkupCode:      "markup.code",
	TokenMarkupRaw:       "markup.raw",

	TokenInvalid:           "invalid",
	TokenInvalidDeprecated: "invalid.deprecated",
	TokenInvalidIllegal:    "invalid.illegal",

	TokenMeta:      "meta",
	TokenTag:       "tag",
	TokenAttribute: "attribute",
	TokenNamespace: "namespace",
	TokenLabel:     "label",

	TokenEditorWhitespace:  "editor.whitespace",
	TokenEditorIndentGuide: "editor.indent-guide",
	TokenEditorLineNumber:  "editor.line-number",
	TokenEditorSelection:   "editor.selection",
	TokenEditorCursor:      "editor.cursor",
}

// tokenTypeScopes maps token types to TextMate scope names.
var tokenTypeScopes = tokenTypeNames // Same as names for now

// scopeToToken maps TextMate scope strings to token types.
var scopeToToken = func() map[string]TokenType {
	m := make(map[string]TokenType, len(tokenTypeNames))
	for i, name := range tokenTypeNames {
		if name != "" {
			m[name] = TokenType(i)
		}
	}
	return m
}()
