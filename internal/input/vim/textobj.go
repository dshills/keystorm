package vim

// TextObject represents a Vim text object.
// Text objects select regions of text based on structure rather than motion.
type TextObject struct {
	// Name is the text object identifier (e.g., "word", "sentence", "paragraph").
	Name string

	// Key is the key that identifies this text object type.
	Key rune

	// Action is the action name to dispatch (e.g., "select.innerWord").
	InnerAction string

	// AroundAction is the action for the "around" variant.
	AroundAction string

	// RequiresDelimiter indicates if this text object needs a delimiter character.
	// e.g., i", i', i(, i{
	RequiresDelimiter bool
}

// Standard Vim text objects.
var (
	// Word text objects
	TextObjWord = TextObject{
		Name:              "word",
		Key:               'w',
		InnerAction:       "select.innerWord",
		AroundAction:      "select.aroundWord",
		RequiresDelimiter: false,
	}

	TextObjWORD = TextObject{
		Name:              "WORD",
		Key:               'W',
		InnerAction:       "select.innerWORD",
		AroundAction:      "select.aroundWORD",
		RequiresDelimiter: false,
	}

	// Sentence text object
	TextObjSentence = TextObject{
		Name:              "sentence",
		Key:               's',
		InnerAction:       "select.innerSentence",
		AroundAction:      "select.aroundSentence",
		RequiresDelimiter: false,
	}

	// Paragraph text object
	TextObjParagraph = TextObject{
		Name:              "paragraph",
		Key:               'p',
		InnerAction:       "select.innerParagraph",
		AroundAction:      "select.aroundParagraph",
		RequiresDelimiter: false,
	}

	// Block/tag text objects
	TextObjBlock = TextObject{
		Name:              "block",
		Key:               'b',
		InnerAction:       "select.innerBlock",
		AroundAction:      "select.aroundBlock",
		RequiresDelimiter: false,
	}

	TextObjBigBlock = TextObject{
		Name:              "bigBlock",
		Key:               'B',
		InnerAction:       "select.innerBigBlock",
		AroundAction:      "select.aroundBigBlock",
		RequiresDelimiter: false,
	}

	TextObjTag = TextObject{
		Name:              "tag",
		Key:               't',
		InnerAction:       "select.innerTag",
		AroundAction:      "select.aroundTag",
		RequiresDelimiter: false,
	}

	// Delimiter-based text objects
	TextObjParen = TextObject{
		Name:              "paren",
		Key:               '(',
		InnerAction:       "select.innerParen",
		AroundAction:      "select.aroundParen",
		RequiresDelimiter: false,
	}

	TextObjParenClose = TextObject{
		Name:              "paren",
		Key:               ')',
		InnerAction:       "select.innerParen",
		AroundAction:      "select.aroundParen",
		RequiresDelimiter: false,
	}

	TextObjBracket = TextObject{
		Name:              "bracket",
		Key:               '[',
		InnerAction:       "select.innerBracket",
		AroundAction:      "select.aroundBracket",
		RequiresDelimiter: false,
	}

	TextObjBracketClose = TextObject{
		Name:              "bracket",
		Key:               ']',
		InnerAction:       "select.innerBracket",
		AroundAction:      "select.aroundBracket",
		RequiresDelimiter: false,
	}

	TextObjBrace = TextObject{
		Name:              "brace",
		Key:               '{',
		InnerAction:       "select.innerBrace",
		AroundAction:      "select.aroundBrace",
		RequiresDelimiter: false,
	}

	TextObjBraceClose = TextObject{
		Name:              "brace",
		Key:               '}',
		InnerAction:       "select.innerBrace",
		AroundAction:      "select.aroundBrace",
		RequiresDelimiter: false,
	}

	TextObjAngle = TextObject{
		Name:              "angle",
		Key:               '<',
		InnerAction:       "select.innerAngle",
		AroundAction:      "select.aroundAngle",
		RequiresDelimiter: false,
	}

	TextObjAngleClose = TextObject{
		Name:              "angle",
		Key:               '>',
		InnerAction:       "select.innerAngle",
		AroundAction:      "select.aroundAngle",
		RequiresDelimiter: false,
	}

	// Quote text objects
	TextObjDoubleQuote = TextObject{
		Name:              "doubleQuote",
		Key:               '"',
		InnerAction:       "select.innerDoubleQuote",
		AroundAction:      "select.aroundDoubleQuote",
		RequiresDelimiter: false,
	}

	TextObjSingleQuote = TextObject{
		Name:              "singleQuote",
		Key:               '\'',
		InnerAction:       "select.innerSingleQuote",
		AroundAction:      "select.aroundSingleQuote",
		RequiresDelimiter: false,
	}

	TextObjBacktick = TextObject{
		Name:              "backtick",
		Key:               '`',
		InnerAction:       "select.innerBacktick",
		AroundAction:      "select.aroundBacktick",
		RequiresDelimiter: false,
	}
)

// textObjects maps text object keys to their definitions.
var textObjects = map[rune]*TextObject{
	'w':  &TextObjWord,
	'W':  &TextObjWORD,
	's':  &TextObjSentence,
	'p':  &TextObjParagraph,
	'b':  &TextObjBlock,
	'B':  &TextObjBigBlock,
	't':  &TextObjTag,
	'(':  &TextObjParen,
	')':  &TextObjParenClose,
	'[':  &TextObjBracket,
	']':  &TextObjBracketClose,
	'{':  &TextObjBrace,
	'}':  &TextObjBraceClose,
	'<':  &TextObjAngle,
	'>':  &TextObjAngleClose,
	'"':  &TextObjDoubleQuote,
	'\'': &TextObjSingleQuote,
	'`':  &TextObjBacktick,
}

// GetTextObject returns the text object for the given key.
// Returns nil if the key is not a text object.
func GetTextObject(key rune) *TextObject {
	return textObjects[key]
}

// IsTextObject returns true if the key is a text object.
func IsTextObject(key rune) bool {
	_, ok := textObjects[key]
	return ok
}

// TextObjectKeys returns all text object key characters.
func TextObjectKeys() []rune {
	keys := make([]rune, 0, len(textObjects))
	for k := range textObjects {
		keys = append(keys, k)
	}
	return keys
}

// TextObjectPrefix represents the prefix for text object selection.
type TextObjectPrefix uint8

const (
	// PrefixNone indicates no text object prefix.
	PrefixNone TextObjectPrefix = iota

	// PrefixInner indicates "inner" selection (i).
	PrefixInner

	// PrefixAround indicates "around" selection (a).
	PrefixAround
)

// String returns a string representation of the prefix.
func (p TextObjectPrefix) String() string {
	switch p {
	case PrefixInner:
		return "inner"
	case PrefixAround:
		return "around"
	default:
		return "none"
	}
}

// IsTextObjectPrefix returns true if the key is 'i' or 'a'.
func IsTextObjectPrefix(key rune) bool {
	return key == 'i' || key == 'a'
}

// GetTextObjectPrefix returns the prefix type for the key.
func GetTextObjectPrefix(key rune) TextObjectPrefix {
	switch key {
	case 'i':
		return PrefixInner
	case 'a':
		return PrefixAround
	default:
		return PrefixNone
	}
}
