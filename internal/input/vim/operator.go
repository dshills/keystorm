package vim

// Operator represents a Vim operator command.
// Operators are commands that perform an action on a range of text
// defined by a motion or text object.
type Operator struct {
	// Name is the operator identifier (e.g., "delete", "change", "yank").
	Name string

	// Key is the primary key that triggers this operator (e.g., 'd', 'c', 'y').
	Key rune

	// Action is the action name to dispatch (e.g., "editor.delete").
	Action string

	// LinewiseAction is the action for line-wise operation (e.g., "dd").
	LinewiseAction string

	// ChangesText indicates if this operator modifies the buffer.
	ChangesText bool

	// EntersInsert indicates if this operator enters insert mode after.
	EntersInsert bool
}

// Standard Vim operators.
var (
	// OpDelete deletes text.
	OpDelete = Operator{
		Name:           "delete",
		Key:            'd',
		Action:         "editor.delete",
		LinewiseAction: "editor.deleteLine",
		ChangesText:    true,
		EntersInsert:   false,
	}

	// OpChange deletes text and enters insert mode.
	OpChange = Operator{
		Name:           "change",
		Key:            'c',
		Action:         "editor.change",
		LinewiseAction: "editor.changeLine",
		ChangesText:    true,
		EntersInsert:   true,
	}

	// OpYank copies text to a register.
	OpYank = Operator{
		Name:           "yank",
		Key:            'y',
		Action:         "editor.yank",
		LinewiseAction: "editor.yankLine",
		ChangesText:    false,
		EntersInsert:   false,
	}

	// OpIndentRight shifts text right.
	OpIndentRight = Operator{
		Name:           "indentRight",
		Key:            '>',
		Action:         "editor.indentRight",
		LinewiseAction: "editor.indentLineRight",
		ChangesText:    true,
		EntersInsert:   false,
	}

	// OpIndentLeft shifts text left.
	OpIndentLeft = Operator{
		Name:           "indentLeft",
		Key:            '<',
		Action:         "editor.indentLeft",
		LinewiseAction: "editor.indentLineLeft",
		ChangesText:    true,
		EntersInsert:   false,
	}

	// OpFormat formats text.
	OpFormat = Operator{
		Name:           "format",
		Key:            '=',
		Action:         "editor.format",
		LinewiseAction: "editor.formatLine",
		ChangesText:    true,
		EntersInsert:   false,
	}

	// OpToLower converts text to lowercase.
	OpToLower = Operator{
		Name:           "toLower",
		Key:            'u',
		Action:         "editor.toLower",
		LinewiseAction: "editor.lineTolower",
		ChangesText:    true,
		EntersInsert:   false,
	}

	// OpToUpper converts text to uppercase.
	OpToUpper = Operator{
		Name:           "toUpper",
		Key:            'U',
		Action:         "editor.toUpper",
		LinewiseAction: "editor.lineToUpper",
		ChangesText:    true,
		EntersInsert:   false,
	}

	// OpToggleCase toggles case.
	OpToggleCase = Operator{
		Name:           "toggleCase",
		Key:            '~',
		Action:         "editor.toggleCase",
		LinewiseAction: "editor.lineToggleCase",
		ChangesText:    true,
		EntersInsert:   false,
	}
)

// operators maps operator keys to their definitions.
var operators = map[rune]*Operator{
	'd': &OpDelete,
	'c': &OpChange,
	'y': &OpYank,
	'>': &OpIndentRight,
	'<': &OpIndentLeft,
	'=': &OpFormat,
	// Note: g~ for toggle case, gu for lowercase, gU for uppercase
	// are handled as g-prefixed operators
}

// gOperators maps g-prefixed operator keys to their definitions.
var gOperators = map[rune]*Operator{
	'~': &OpToggleCase,
	'u': &OpToLower,
	'U': &OpToUpper,
}

// GetOperator returns the operator for the given key.
// Returns nil if the key is not an operator.
func GetOperator(key rune) *Operator {
	return operators[key]
}

// GetGOperator returns the g-prefixed operator for the given key.
// Returns nil if the key is not a g-prefixed operator.
func GetGOperator(key rune) *Operator {
	return gOperators[key]
}

// IsOperator returns true if the key is an operator.
func IsOperator(key rune) bool {
	_, ok := operators[key]
	return ok
}

// IsGOperator returns true if the key is a g-prefixed operator.
func IsGOperator(key rune) bool {
	_, ok := gOperators[key]
	return ok
}

// OperatorKeys returns all operator key characters.
func OperatorKeys() []rune {
	keys := make([]rune, 0, len(operators))
	for k := range operators {
		keys = append(keys, k)
	}
	return keys
}
