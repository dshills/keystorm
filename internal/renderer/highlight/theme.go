package highlight

import (
	"github.com/dshills/keystorm/internal/renderer/core"
)

// Theme defines colors and styles for syntax highlighting.
type Theme struct {
	// Name is the display name of the theme.
	Name string

	// Background is the editor background color.
	Background core.Color

	// Foreground is the default text color.
	Foreground core.Color

	// Selection is the selection highlight color.
	Selection core.Color

	// Cursor is the cursor color.
	Cursor core.Color

	// LineHighlight is the current line highlight color.
	LineHighlight core.Color

	// TokenStyles maps token types to their styles.
	TokenStyles map[TokenType]core.Style

	// ScopeStyles maps scope strings to styles (for custom scopes).
	ScopeStyles map[string]core.Style
}

// StyleForToken returns the style for a given token type.
func (t *Theme) StyleForToken(tokenType TokenType) core.Style {
	if style, ok := t.TokenStyles[tokenType]; ok {
		return style
	}
	// Fall back to default style
	return core.Style{
		Foreground: t.Foreground,
		Background: core.ColorDefault,
	}
}

// StyleForScope returns the style for a given scope string.
func (t *Theme) StyleForScope(scope string) core.Style {
	// Check exact match first
	if style, ok := t.ScopeStyles[scope]; ok {
		return style
	}

	// Try token type mapping
	if tokenType := TokenTypeFromString(scope); tokenType != TokenNone {
		if style, ok := t.TokenStyles[tokenType]; ok {
			return style
		}
	}

	// Check parent scopes
	for len(scope) > 0 {
		if style, ok := t.ScopeStyles[scope]; ok {
			return style
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

	// Fall back to default
	return core.Style{
		Foreground: t.Foreground,
		Background: core.ColorDefault,
	}
}

// DefaultTheme returns a sensible default dark theme.
func DefaultTheme() *Theme {
	return &Theme{
		Name:          "Default Dark",
		Background:    core.ColorFromRGB(30, 30, 30),
		Foreground:    core.ColorFromRGB(212, 212, 212),
		Selection:     core.ColorFromRGB(64, 64, 128),
		Cursor:        core.ColorFromRGB(255, 255, 255),
		LineHighlight: core.ColorFromRGB(40, 40, 40),
		TokenStyles:   defaultDarkTokenStyles(),
		ScopeStyles:   make(map[string]core.Style),
	}
}

// MonokaiTheme returns a Monokai-inspired theme.
func MonokaiTheme() *Theme {
	return &Theme{
		Name:          "Monokai",
		Background:    core.ColorFromRGB(39, 40, 34),
		Foreground:    core.ColorFromRGB(248, 248, 242),
		Selection:     core.ColorFromRGB(73, 72, 62),
		Cursor:        core.ColorFromRGB(248, 248, 240),
		LineHighlight: core.ColorFromRGB(62, 61, 50),
		TokenStyles:   monokaiTokenStyles(),
		ScopeStyles:   make(map[string]core.Style),
	}
}

// DraculaTheme returns a Dracula-inspired theme.
func DraculaTheme() *Theme {
	return &Theme{
		Name:          "Dracula",
		Background:    core.ColorFromRGB(40, 42, 54),
		Foreground:    core.ColorFromRGB(248, 248, 242),
		Selection:     core.ColorFromRGB(68, 71, 90),
		Cursor:        core.ColorFromRGB(248, 248, 242),
		LineHighlight: core.ColorFromRGB(68, 71, 90),
		TokenStyles:   draculaTokenStyles(),
		ScopeStyles:   make(map[string]core.Style),
	}
}

// SolarizedDarkTheme returns a Solarized Dark theme.
func SolarizedDarkTheme() *Theme {
	return &Theme{
		Name:          "Solarized Dark",
		Background:    core.ColorFromRGB(0, 43, 54),
		Foreground:    core.ColorFromRGB(131, 148, 150),
		Selection:     core.ColorFromRGB(7, 54, 66),
		Cursor:        core.ColorFromRGB(131, 148, 150),
		LineHighlight: core.ColorFromRGB(7, 54, 66),
		TokenStyles:   solarizedDarkTokenStyles(),
		ScopeStyles:   make(map[string]core.Style),
	}
}

// LightTheme returns a light theme.
func LightTheme() *Theme {
	return &Theme{
		Name:          "Light",
		Background:    core.ColorFromRGB(255, 255, 255),
		Foreground:    core.ColorFromRGB(0, 0, 0),
		Selection:     core.ColorFromRGB(173, 214, 255),
		Cursor:        core.ColorFromRGB(0, 0, 0),
		LineHighlight: core.ColorFromRGB(245, 245, 245),
		TokenStyles:   lightTokenStyles(),
		ScopeStyles:   make(map[string]core.Style),
	}
}

// defaultDarkTokenStyles returns default dark theme token styles.
func defaultDarkTokenStyles() map[TokenType]core.Style {
	// Colors
	comment := core.ColorFromRGB(106, 153, 85)   // Green
	keyword := core.ColorFromRGB(86, 156, 214)   // Blue
	str := core.ColorFromRGB(206, 145, 120)      // Orange
	number := core.ColorFromRGB(181, 206, 168)   // Light green
	function := core.ColorFromRGB(220, 220, 170) // Yellow
	typ := core.ColorFromRGB(78, 201, 176)       // Teal
	variable := core.ColorFromRGB(156, 220, 254) // Light blue
	operator := core.ColorFromRGB(212, 212, 212) // White
	invalid := core.ColorFromRGB(244, 71, 71)    // Red

	return map[TokenType]core.Style{
		// Comments
		TokenComment:      core.NewStyle(comment).Italic(),
		TokenCommentLine:  core.NewStyle(comment).Italic(),
		TokenCommentBlock: core.NewStyle(comment).Italic(),
		TokenCommentDoc:   core.NewStyle(comment).Italic(),

		// Strings
		TokenString:             core.NewStyle(str),
		TokenStringQuoted:       core.NewStyle(str),
		TokenStringInterpolated: core.NewStyle(str),
		TokenStringRegexp:       core.NewStyle(str),
		TokenStringEscape:       core.NewStyle(core.ColorFromRGB(215, 186, 125)),

		// Numbers
		TokenNumber:        core.NewStyle(number),
		TokenNumberInteger: core.NewStyle(number),
		TokenNumberFloat:   core.NewStyle(number),
		TokenNumberHex:     core.NewStyle(number),
		TokenNumberOctal:   core.NewStyle(number),
		TokenNumberBinary:  core.NewStyle(number),

		// Keywords
		TokenKeyword:            core.NewStyle(keyword),
		TokenKeywordControl:     core.NewStyle(keyword),
		TokenKeywordOperator:    core.NewStyle(keyword),
		TokenKeywordOther:       core.NewStyle(keyword),
		TokenKeywordDeclaration: core.NewStyle(keyword),

		// Operators
		TokenOperator:             core.NewStyle(operator),
		TokenOperatorAssignment:   core.NewStyle(operator),
		TokenOperatorComparison:   core.NewStyle(operator),
		TokenOperatorArithmetic:   core.NewStyle(operator),
		TokenOperatorLogical:      core.NewStyle(operator),
		TokenPunctuation:          core.NewStyle(operator),
		TokenPunctuationBracket:   core.NewStyle(operator),
		TokenPunctuationDelimiter: core.NewStyle(operator),

		// Identifiers
		TokenIdentifier:        core.NewStyle(variable),
		TokenVariable:          core.NewStyle(variable),
		TokenVariableParameter: core.NewStyle(variable),
		TokenVariableOther:     core.NewStyle(variable),
		TokenConstant:          core.NewStyle(core.ColorFromRGB(79, 193, 255)),
		TokenConstantLanguage:  core.NewStyle(keyword),

		// Functions
		TokenFunction:            core.NewStyle(function),
		TokenFunctionDeclaration: core.NewStyle(function),
		TokenFunctionCall:        core.NewStyle(function),
		TokenFunctionMethod:      core.NewStyle(function),
		TokenFunctionBuiltin:     core.NewStyle(function),

		// Types
		TokenTypeName:      core.NewStyle(typ),
		TokenTypeBuiltin:   core.NewStyle(typ),
		TokenTypeClass:     core.NewStyle(typ),
		TokenTypeInterface: core.NewStyle(typ),
		TokenTypeStruct:    core.NewStyle(typ),
		TokenTypeEnum:      core.NewStyle(typ),
		TokenTypeParameter: core.NewStyle(typ),

		// Storage
		TokenStorage:         core.NewStyle(keyword),
		TokenStorageType:     core.NewStyle(keyword),
		TokenStorageModifier: core.NewStyle(keyword),

		// Invalid
		TokenInvalid:           core.NewStyle(invalid),
		TokenInvalidDeprecated: core.NewStyle(invalid).Strikethrough(),
		TokenInvalidIllegal:    core.NewStyle(invalid).Bold(),

		// Markup
		TokenMarkupHeading: core.NewStyle(keyword).Bold(),
		TokenMarkupBold:    core.DefaultStyle().Bold(),
		TokenMarkupItalic:  core.DefaultStyle().Italic(),
		TokenMarkupCode:    core.NewStyle(str),
		TokenMarkupLink:    core.NewStyle(typ).Underline(),
	}
}

// monokaiTokenStyles returns Monokai theme token styles.
func monokaiTokenStyles() map[TokenType]core.Style {
	pink := core.ColorFromRGB(249, 38, 114)
	green := core.ColorFromRGB(166, 226, 46)
	orange := core.ColorFromRGB(253, 151, 31)
	yellow := core.ColorFromRGB(230, 219, 116)
	blue := core.ColorFromRGB(102, 217, 239)
	purple := core.ColorFromRGB(174, 129, 255)
	comment := core.ColorFromRGB(117, 113, 94)
	white := core.ColorFromRGB(248, 248, 242)

	return map[TokenType]core.Style{
		// Comments
		TokenComment:      core.NewStyle(comment),
		TokenCommentLine:  core.NewStyle(comment),
		TokenCommentBlock: core.NewStyle(comment),
		TokenCommentDoc:   core.NewStyle(comment),

		// Strings
		TokenString:             core.NewStyle(yellow),
		TokenStringQuoted:       core.NewStyle(yellow),
		TokenStringInterpolated: core.NewStyle(yellow),
		TokenStringRegexp:       core.NewStyle(yellow),
		TokenStringEscape:       core.NewStyle(purple),

		// Numbers
		TokenNumber:        core.NewStyle(purple),
		TokenNumberInteger: core.NewStyle(purple),
		TokenNumberFloat:   core.NewStyle(purple),
		TokenNumberHex:     core.NewStyle(purple),
		TokenNumberOctal:   core.NewStyle(purple),
		TokenNumberBinary:  core.NewStyle(purple),

		// Keywords
		TokenKeyword:            core.NewStyle(pink),
		TokenKeywordControl:     core.NewStyle(pink),
		TokenKeywordOperator:    core.NewStyle(pink),
		TokenKeywordOther:       core.NewStyle(pink),
		TokenKeywordDeclaration: core.NewStyle(blue).Italic(),

		// Operators
		TokenOperator:             core.NewStyle(pink),
		TokenOperatorAssignment:   core.NewStyle(pink),
		TokenOperatorComparison:   core.NewStyle(pink),
		TokenOperatorArithmetic:   core.NewStyle(pink),
		TokenOperatorLogical:      core.NewStyle(pink),
		TokenPunctuation:          core.NewStyle(white),
		TokenPunctuationBracket:   core.NewStyle(white),
		TokenPunctuationDelimiter: core.NewStyle(white),

		// Identifiers
		TokenIdentifier:        core.NewStyle(white),
		TokenVariable:          core.NewStyle(white),
		TokenVariableParameter: core.NewStyle(orange).Italic(),
		TokenVariableOther:     core.NewStyle(white),
		TokenConstant:          core.NewStyle(purple),
		TokenConstantLanguage:  core.NewStyle(purple),

		// Functions
		TokenFunction:            core.NewStyle(green),
		TokenFunctionDeclaration: core.NewStyle(green),
		TokenFunctionCall:        core.NewStyle(green),
		TokenFunctionMethod:      core.NewStyle(green),
		TokenFunctionBuiltin:     core.NewStyle(blue),

		// Types
		TokenTypeName:      core.NewStyle(blue).Italic(),
		TokenTypeBuiltin:   core.NewStyle(blue).Italic(),
		TokenTypeClass:     core.NewStyle(green).Underline(),
		TokenTypeInterface: core.NewStyle(blue).Italic(),
		TokenTypeStruct:    core.NewStyle(green),
		TokenTypeEnum:      core.NewStyle(green),
		TokenTypeParameter: core.NewStyle(orange).Italic(),

		// Storage
		TokenStorage:         core.NewStyle(pink),
		TokenStorageType:     core.NewStyle(blue).Italic(),
		TokenStorageModifier: core.NewStyle(pink),

		// Invalid
		TokenInvalid:           core.NewStyle(core.ColorFromRGB(249, 38, 114)).WithBackground(core.ColorFromRGB(80, 20, 40)),
		TokenInvalidDeprecated: core.NewStyle(comment).Strikethrough(),
		TokenInvalidIllegal:    core.NewStyle(core.ColorFromRGB(249, 38, 114)).Bold(),
	}
}

// draculaTokenStyles returns Dracula theme token styles.
func draculaTokenStyles() map[TokenType]core.Style {
	pink := core.ColorFromRGB(255, 121, 198)
	green := core.ColorFromRGB(80, 250, 123)
	orange := core.ColorFromRGB(255, 184, 108)
	yellow := core.ColorFromRGB(241, 250, 140)
	purple := core.ColorFromRGB(189, 147, 249)
	cyan := core.ColorFromRGB(139, 233, 253)
	red := core.ColorFromRGB(255, 85, 85)
	comment := core.ColorFromRGB(98, 114, 164)
	white := core.ColorFromRGB(248, 248, 242)

	return map[TokenType]core.Style{
		TokenComment:      core.NewStyle(comment),
		TokenCommentLine:  core.NewStyle(comment),
		TokenCommentBlock: core.NewStyle(comment),
		TokenCommentDoc:   core.NewStyle(comment),

		TokenString:       core.NewStyle(yellow),
		TokenStringEscape: core.NewStyle(pink),

		TokenNumber: core.NewStyle(purple),

		TokenKeyword:            core.NewStyle(pink),
		TokenKeywordControl:     core.NewStyle(pink),
		TokenKeywordDeclaration: core.NewStyle(pink),

		TokenOperator:    core.NewStyle(pink),
		TokenPunctuation: core.NewStyle(white),

		TokenIdentifier:        core.NewStyle(white),
		TokenVariable:          core.NewStyle(white),
		TokenVariableParameter: core.NewStyle(orange).Italic(),
		TokenConstant:          core.NewStyle(purple),
		TokenConstantLanguage:  core.NewStyle(purple),

		TokenFunction:            core.NewStyle(green),
		TokenFunctionDeclaration: core.NewStyle(green),
		TokenFunctionCall:        core.NewStyle(green),
		TokenFunctionBuiltin:     core.NewStyle(cyan),

		TokenTypeName:    core.NewStyle(cyan).Italic(),
		TokenTypeBuiltin: core.NewStyle(cyan).Italic(),
		TokenTypeClass:   core.NewStyle(cyan),

		TokenStorage:         core.NewStyle(pink),
		TokenStorageModifier: core.NewStyle(pink),

		TokenInvalid:        core.NewStyle(red),
		TokenInvalidIllegal: core.NewStyle(red).Bold(),
	}
}

// solarizedDarkTokenStyles returns Solarized Dark theme token styles.
func solarizedDarkTokenStyles() map[TokenType]core.Style {
	base03 := core.ColorFromRGB(0, 43, 54)
	base01 := core.ColorFromRGB(88, 110, 117)
	base0 := core.ColorFromRGB(131, 148, 150)
	yellow := core.ColorFromRGB(181, 137, 0)
	orange := core.ColorFromRGB(203, 75, 22)
	red := core.ColorFromRGB(220, 50, 47)
	magenta := core.ColorFromRGB(211, 54, 130)
	violet := core.ColorFromRGB(108, 113, 196)
	blue := core.ColorFromRGB(38, 139, 210)
	cyan := core.ColorFromRGB(42, 161, 152)
	green := core.ColorFromRGB(133, 153, 0)

	_ = base03 // Used for background
	_ = base0  // Used for foreground

	return map[TokenType]core.Style{
		TokenComment:      core.NewStyle(base01).Italic(),
		TokenCommentLine:  core.NewStyle(base01).Italic(),
		TokenCommentBlock: core.NewStyle(base01).Italic(),
		TokenCommentDoc:   core.NewStyle(base01).Italic(),

		TokenString:       core.NewStyle(cyan),
		TokenStringEscape: core.NewStyle(orange),

		TokenNumber: core.NewStyle(magenta),

		TokenKeyword:            core.NewStyle(green),
		TokenKeywordControl:     core.NewStyle(green),
		TokenKeywordDeclaration: core.NewStyle(green),

		TokenOperator:    core.NewStyle(green),
		TokenPunctuation: core.NewStyle(base01),

		TokenIdentifier:        core.NewStyle(blue),
		TokenVariable:          core.NewStyle(blue),
		TokenVariableParameter: core.NewStyle(blue),
		TokenConstant:          core.NewStyle(violet),
		TokenConstantLanguage:  core.NewStyle(violet),

		TokenFunction:            core.NewStyle(blue),
		TokenFunctionDeclaration: core.NewStyle(blue),
		TokenFunctionCall:        core.NewStyle(blue),
		TokenFunctionBuiltin:     core.NewStyle(blue),

		TokenTypeName:    core.NewStyle(yellow),
		TokenTypeBuiltin: core.NewStyle(yellow),
		TokenTypeClass:   core.NewStyle(yellow),

		TokenStorage:         core.NewStyle(green),
		TokenStorageModifier: core.NewStyle(orange),

		TokenInvalid:        core.NewStyle(red),
		TokenInvalidIllegal: core.NewStyle(red).Bold(),
	}
}

// lightTokenStyles returns light theme token styles.
func lightTokenStyles() map[TokenType]core.Style {
	comment := core.ColorFromRGB(0, 128, 0)    // Green
	keyword := core.ColorFromRGB(0, 0, 255)    // Blue
	str := core.ColorFromRGB(163, 21, 21)      // Dark red
	number := core.ColorFromRGB(9, 134, 88)    // Teal
	function := core.ColorFromRGB(121, 94, 38) // Brown
	typ := core.ColorFromRGB(38, 127, 153)     // Cyan
	variable := core.ColorFromRGB(0, 16, 128)  // Dark blue
	operator := core.ColorFromRGB(0, 0, 0)     // Black
	invalid := core.ColorFromRGB(205, 49, 49)  // Red

	return map[TokenType]core.Style{
		TokenComment:      core.NewStyle(comment).Italic(),
		TokenCommentLine:  core.NewStyle(comment).Italic(),
		TokenCommentBlock: core.NewStyle(comment).Italic(),
		TokenCommentDoc:   core.NewStyle(comment).Italic(),

		TokenString:       core.NewStyle(str),
		TokenStringEscape: core.NewStyle(core.ColorFromRGB(205, 49, 49)),

		TokenNumber: core.NewStyle(number),

		TokenKeyword:            core.NewStyle(keyword),
		TokenKeywordControl:     core.NewStyle(keyword),
		TokenKeywordDeclaration: core.NewStyle(keyword),

		TokenOperator:    core.NewStyle(operator),
		TokenPunctuation: core.NewStyle(operator),

		TokenIdentifier:        core.NewStyle(variable),
		TokenVariable:          core.NewStyle(variable),
		TokenVariableParameter: core.NewStyle(variable),
		TokenConstant:          core.NewStyle(core.ColorFromRGB(0, 112, 193)),
		TokenConstantLanguage:  core.NewStyle(keyword),

		TokenFunction:            core.NewStyle(function),
		TokenFunctionDeclaration: core.NewStyle(function),
		TokenFunctionCall:        core.NewStyle(function),
		TokenFunctionBuiltin:     core.NewStyle(function),

		TokenTypeName:    core.NewStyle(typ),
		TokenTypeBuiltin: core.NewStyle(typ),
		TokenTypeClass:   core.NewStyle(typ),

		TokenStorage:         core.NewStyle(keyword),
		TokenStorageModifier: core.NewStyle(keyword),

		TokenInvalid:        core.NewStyle(invalid),
		TokenInvalidIllegal: core.NewStyle(invalid).Bold(),
	}
}

// ThemeRegistry holds available themes.
type ThemeRegistry struct {
	themes  map[string]*Theme
	current *Theme
}

// NewThemeRegistry creates a new theme registry with built-in themes.
func NewThemeRegistry() *ThemeRegistry {
	r := &ThemeRegistry{
		themes: make(map[string]*Theme),
	}

	// Register built-in themes
	r.Register(DefaultTheme())
	r.Register(MonokaiTheme())
	r.Register(DraculaTheme())
	r.Register(SolarizedDarkTheme())
	r.Register(LightTheme())

	// Set default
	r.current = r.themes["Default Dark"]

	return r
}

// Register adds a theme to the registry.
func (r *ThemeRegistry) Register(theme *Theme) {
	r.themes[theme.Name] = theme
}

// Get returns a theme by name.
func (r *ThemeRegistry) Get(name string) (*Theme, bool) {
	t, ok := r.themes[name]
	return t, ok
}

// Current returns the current theme.
func (r *ThemeRegistry) Current() *Theme {
	return r.current
}

// SetCurrent sets the current theme by name.
func (r *ThemeRegistry) SetCurrent(name string) bool {
	if t, ok := r.themes[name]; ok {
		r.current = t
		return true
	}
	return false
}

// Names returns all registered theme names.
func (r *ThemeRegistry) Names() []string {
	names := make([]string, 0, len(r.themes))
	for name := range r.themes {
		names = append(names, name)
	}
	return names
}
