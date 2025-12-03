package input

// Direction represents a directional command.
type Direction uint8

const (
	// DirNone indicates no direction.
	DirNone Direction = iota
	// DirUp indicates upward direction.
	DirUp
	// DirDown indicates downward direction.
	DirDown
	// DirLeft indicates leftward direction.
	DirLeft
	// DirRight indicates rightward direction.
	DirRight
	// DirForward indicates forward direction (context-dependent).
	DirForward
	// DirBackward indicates backward direction (context-dependent).
	DirBackward
)

// String returns a string representation of the direction.
func (d Direction) String() string {
	switch d {
	case DirUp:
		return "up"
	case DirDown:
		return "down"
	case DirLeft:
		return "left"
	case DirRight:
		return "right"
	case DirForward:
		return "forward"
	case DirBackward:
		return "backward"
	default:
		return "none"
	}
}

// ActionSource indicates the origin of an action.
type ActionSource uint8

const (
	// SourceKeyboard indicates the action originated from keyboard input.
	SourceKeyboard ActionSource = iota
	// SourceMouse indicates the action originated from mouse input.
	SourceMouse
	// SourcePalette indicates the action originated from the command palette.
	SourcePalette
	// SourceMacro indicates the action originated from macro playback.
	SourceMacro
	// SourcePlugin indicates the action originated from a plugin.
	SourcePlugin
	// SourceAPI indicates the action originated from an API call.
	SourceAPI
)

// String returns a string representation of the action source.
func (s ActionSource) String() string {
	switch s {
	case SourceKeyboard:
		return "keyboard"
	case SourceMouse:
		return "mouse"
	case SourcePalette:
		return "palette"
	case SourceMacro:
		return "macro"
	case SourcePlugin:
		return "plugin"
	case SourceAPI:
		return "api"
	default:
		return "unknown"
	}
}

// Motion represents a cursor motion for operator commands.
type Motion struct {
	// Name is the motion identifier (e.g., "word", "line", "paragraph").
	Name string

	// Direction indicates the motion direction.
	Direction Direction

	// Inclusive indicates whether the motion includes the end position.
	Inclusive bool

	// Count is the repeat count for the motion.
	Count int
}

// TextObject represents a text object for operator commands.
type TextObject struct {
	// Name is the text object identifier (e.g., "word", "sentence", "paragraph").
	Name string

	// Inner indicates whether to select inner (true) or around (false).
	Inner bool

	// Delimiter for delimited text objects (e.g., '"', '(', '{').
	Delimiter rune
}

// ActionArgs holds arguments for an action.
type ActionArgs struct {
	// Motion for operator commands.
	Motion *Motion

	// TextObject for text object commands.
	TextObject *TextObject

	// Register for yank/paste operations (a-z, 0-9, ", +, *, etc.).
	Register rune

	// Text for insert/replace operations.
	Text string

	// Direction for directional commands.
	Direction Direction

	// SearchPattern for search commands.
	SearchPattern string

	// Extra holds additional key-value pairs for extensibility.
	Extra map[string]interface{}
}

// Get retrieves a value from Extra with type assertion.
func (a ActionArgs) Get(key string) (interface{}, bool) {
	if a.Extra == nil {
		return nil, false
	}
	v, ok := a.Extra[key]
	return v, ok
}

// GetString retrieves a string value from Extra.
func (a ActionArgs) GetString(key string) string {
	if v, ok := a.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt retrieves an int value from Extra.
func (a ActionArgs) GetInt(key string) int {
	if v, ok := a.Get(key); ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

// GetBool retrieves a bool value from Extra.
func (a ActionArgs) GetBool(key string) bool {
	if v, ok := a.Get(key); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// Action represents a command to be executed by the dispatcher.
type Action struct {
	// Name is the command identifier (e.g., "editor.save", "cursor.moveDown").
	Name string

	// Args contains command-specific arguments.
	Args ActionArgs

	// Source indicates where this action originated.
	Source ActionSource

	// Count is the repeat count (from Vim-style count prefix).
	Count int
}

// WithCount returns a copy of the action with the specified count.
func (a Action) WithCount(count int) Action {
	a.Count = count
	return a
}

// WithRegister returns a copy of the action with the specified register.
func (a Action) WithRegister(register rune) Action {
	a.Args.Register = register
	return a
}

// WithMotion returns a copy of the action with the specified motion.
func (a Action) WithMotion(motion *Motion) Action {
	a.Args.Motion = motion
	return a
}

// WithTextObject returns a copy of the action with the specified text object.
func (a Action) WithTextObject(textObj *TextObject) Action {
	a.Args.TextObject = textObj
	return a
}
