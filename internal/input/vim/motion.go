package vim

// MotionType categorizes motions by their behavior.
type MotionType uint8

const (
	// MotionCharwise moves character by character.
	MotionCharwise MotionType = iota

	// MotionLinewise operates on whole lines.
	MotionLinewise

	// MotionBlockwise operates on rectangular blocks.
	MotionBlockwise
)

// Motion represents a Vim motion command.
// Motions define how the cursor moves and what range an operator affects.
type Motion struct {
	// Name is the motion identifier (e.g., "word", "end", "line").
	Name string

	// Keys is the key sequence that triggers this motion.
	Keys string

	// Action is the action name to dispatch (e.g., "cursor.wordForward").
	Action string

	// Type indicates the motion type (charwise, linewise, blockwise).
	Type MotionType

	// Inclusive indicates if the motion includes the character under cursor.
	// e.g., 'e' is inclusive, 'w' is exclusive.
	Inclusive bool

	// Repeatable indicates if this motion can be repeated with count.
	Repeatable bool
}

// Standard Vim motions.
var (
	// Character motions
	MotionLeft = Motion{
		Name:       "left",
		Keys:       "h",
		Action:     "cursor.left",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	MotionRight = Motion{
		Name:       "right",
		Keys:       "l",
		Action:     "cursor.right",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	MotionUp = Motion{
		Name:       "up",
		Keys:       "k",
		Action:     "cursor.up",
		Type:       MotionLinewise,
		Inclusive:  false,
		Repeatable: true,
	}

	MotionDown = Motion{
		Name:       "down",
		Keys:       "j",
		Action:     "cursor.down",
		Type:       MotionLinewise,
		Inclusive:  false,
		Repeatable: true,
	}

	// Word motions
	MotionWordForward = Motion{
		Name:       "wordForward",
		Keys:       "w",
		Action:     "cursor.wordForward",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	MotionWordBackward = Motion{
		Name:       "wordBackward",
		Keys:       "b",
		Action:     "cursor.wordBackward",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	MotionWordEnd = Motion{
		Name:       "wordEnd",
		Keys:       "e",
		Action:     "cursor.wordEnd",
		Type:       MotionCharwise,
		Inclusive:  true,
		Repeatable: true,
	}

	// WORD motions (whitespace-delimited)
	MotionWORDForward = Motion{
		Name:       "WORDForward",
		Keys:       "W",
		Action:     "cursor.WORDForward",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	MotionWORDBackward = Motion{
		Name:       "WORDBackward",
		Keys:       "B",
		Action:     "cursor.WORDBackward",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	MotionWORDEnd = Motion{
		Name:       "WORDEnd",
		Keys:       "E",
		Action:     "cursor.WORDEnd",
		Type:       MotionCharwise,
		Inclusive:  true,
		Repeatable: true,
	}

	// Line motions
	MotionLineStart = Motion{
		Name:       "lineStart",
		Keys:       "0",
		Action:     "cursor.lineStart",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: false,
	}

	MotionFirstNonBlank = Motion{
		Name:       "firstNonBlank",
		Keys:       "^",
		Action:     "cursor.firstNonBlank",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: false,
	}

	MotionLineEnd = Motion{
		Name:       "lineEnd",
		Keys:       "$",
		Action:     "cursor.lineEnd",
		Type:       MotionCharwise,
		Inclusive:  true,
		Repeatable: false,
	}

	// Screen line motions (for wrapped lines)
	MotionScreenLineStart = Motion{
		Name:       "screenLineStart",
		Keys:       "g0",
		Action:     "cursor.screenLineStart",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: false,
	}

	MotionScreenLineEnd = Motion{
		Name:       "screenLineEnd",
		Keys:       "g$",
		Action:     "cursor.screenLineEnd",
		Type:       MotionCharwise,
		Inclusive:  true,
		Repeatable: false,
	}

	// Document motions
	MotionDocumentStart = Motion{
		Name:       "documentStart",
		Keys:       "gg",
		Action:     "cursor.documentStart",
		Type:       MotionLinewise,
		Inclusive:  false,
		Repeatable: false,
	}

	MotionDocumentEnd = Motion{
		Name:       "documentEnd",
		Keys:       "G",
		Action:     "cursor.documentEnd",
		Type:       MotionLinewise,
		Inclusive:  true,
		Repeatable: false,
	}

	// Search motions
	MotionFindChar = Motion{
		Name:       "findChar",
		Keys:       "f",
		Action:     "cursor.findChar",
		Type:       MotionCharwise,
		Inclusive:  true,
		Repeatable: true,
	}

	MotionFindCharBack = Motion{
		Name:       "findCharBack",
		Keys:       "F",
		Action:     "cursor.findCharBack",
		Type:       MotionCharwise,
		Inclusive:  true,
		Repeatable: true,
	}

	MotionTillChar = Motion{
		Name:       "tillChar",
		Keys:       "t",
		Action:     "cursor.tillChar",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	MotionTillCharBack = Motion{
		Name:       "tillCharBack",
		Keys:       "T",
		Action:     "cursor.tillCharBack",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	// Paragraph motions
	MotionParagraphForward = Motion{
		Name:       "paragraphForward",
		Keys:       "}",
		Action:     "cursor.paragraphForward",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	MotionParagraphBackward = Motion{
		Name:       "paragraphBackward",
		Keys:       "{",
		Action:     "cursor.paragraphBackward",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	// Sentence motions
	MotionSentenceForward = Motion{
		Name:       "sentenceForward",
		Keys:       ")",
		Action:     "cursor.sentenceForward",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	MotionSentenceBackward = Motion{
		Name:       "sentenceBackward",
		Keys:       "(",
		Action:     "cursor.sentenceBackward",
		Type:       MotionCharwise,
		Inclusive:  false,
		Repeatable: true,
	}

	// Match motions
	MotionMatchPair = Motion{
		Name:       "matchPair",
		Keys:       "%",
		Action:     "cursor.matchPair",
		Type:       MotionCharwise,
		Inclusive:  true,
		Repeatable: false,
	}
)

// motions maps single-key motion keys to their definitions.
var motions = map[rune]*Motion{
	'h': &MotionLeft,
	'l': &MotionRight,
	'k': &MotionUp,
	'j': &MotionDown,
	'w': &MotionWordForward,
	'b': &MotionWordBackward,
	'e': &MotionWordEnd,
	'W': &MotionWORDForward,
	'B': &MotionWORDBackward,
	'E': &MotionWORDEnd,
	'0': &MotionLineStart,
	'^': &MotionFirstNonBlank,
	'$': &MotionLineEnd,
	'G': &MotionDocumentEnd,
	'f': &MotionFindChar,
	'F': &MotionFindCharBack,
	't': &MotionTillChar,
	'T': &MotionTillCharBack,
	'}': &MotionParagraphForward,
	'{': &MotionParagraphBackward,
	')': &MotionSentenceForward,
	'(': &MotionSentenceBackward,
	'%': &MotionMatchPair,
}

// gMotions maps g-prefixed motion keys to their definitions.
var gMotions = map[rune]*Motion{
	'g': &MotionDocumentStart, // gg
	'0': &MotionScreenLineStart,
	'$': &MotionScreenLineEnd,
}

// charSearchMotions are motions that require a character argument.
var charSearchMotions = map[rune]bool{
	'f': true,
	'F': true,
	't': true,
	'T': true,
}

// GetMotion returns the motion for the given key.
// Returns nil if the key is not a motion.
func GetMotion(key rune) *Motion {
	return motions[key]
}

// GetGMotion returns the g-prefixed motion for the given key.
// Returns nil if the key is not a g-prefixed motion.
func GetGMotion(key rune) *Motion {
	return gMotions[key]
}

// IsMotion returns true if the key is a motion.
func IsMotion(key rune) bool {
	_, ok := motions[key]
	return ok
}

// IsGMotion returns true if the key is a g-prefixed motion.
func IsGMotion(key rune) bool {
	_, ok := gMotions[key]
	return ok
}

// IsCharSearchMotion returns true if the motion requires a character argument.
func IsCharSearchMotion(key rune) bool {
	return charSearchMotions[key]
}

// MotionKeys returns all motion key characters.
func MotionKeys() []rune {
	keys := make([]rune, 0, len(motions))
	for k := range motions {
		keys = append(keys, k)
	}
	return keys
}
