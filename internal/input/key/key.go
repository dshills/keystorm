package key

import (
	"fmt"
	"strings"
)

// Key represents a keyboard key.
// For character keys, use KeyRune and set the Rune field in KeyEvent.
type Key uint16

const (
	// KeyNone represents no key.
	KeyNone Key = iota

	// Special keys
	KeyEscape
	KeyEnter
	KeyTab
	KeyBackspace
	KeyDelete
	KeyInsert
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown

	// Arrow keys
	KeyUp
	KeyDown
	KeyLeft
	KeyRight

	// Function keys
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12

	// Other special keys
	KeySpace
	KeyPause
	KeyPrintScreen
	KeyScrollLock
	KeyNumLock
	KeyCapsLock

	// Keypad keys
	KeyKP0
	KeyKP1
	KeyKP2
	KeyKP3
	KeyKP4
	KeyKP5
	KeyKP6
	KeyKP7
	KeyKP8
	KeyKP9
	KeyKPAdd
	KeyKPSubtract
	KeyKPMultiply
	KeyKPDivide
	KeyKPDecimal
	KeyKPEnter

	// KeyRune is used for character keys (letters, numbers, punctuation).
	// The actual character is stored in KeyEvent.Rune.
	KeyRune
)

// String returns a human-readable name for the key.
func (k Key) String() string {
	switch k {
	case KeyNone:
		return "None"
	case KeyEscape:
		return "Escape"
	case KeyEnter:
		return "Enter"
	case KeyTab:
		return "Tab"
	case KeyBackspace:
		return "Backspace"
	case KeyDelete:
		return "Delete"
	case KeyInsert:
		return "Insert"
	case KeyHome:
		return "Home"
	case KeyEnd:
		return "End"
	case KeyPageUp:
		return "PageUp"
	case KeyPageDown:
		return "PageDown"
	case KeyUp:
		return "Up"
	case KeyDown:
		return "Down"
	case KeyLeft:
		return "Left"
	case KeyRight:
		return "Right"
	case KeyF1:
		return "F1"
	case KeyF2:
		return "F2"
	case KeyF3:
		return "F3"
	case KeyF4:
		return "F4"
	case KeyF5:
		return "F5"
	case KeyF6:
		return "F6"
	case KeyF7:
		return "F7"
	case KeyF8:
		return "F8"
	case KeyF9:
		return "F9"
	case KeyF10:
		return "F10"
	case KeyF11:
		return "F11"
	case KeyF12:
		return "F12"
	case KeySpace:
		return "Space"
	case KeyPause:
		return "Pause"
	case KeyPrintScreen:
		return "PrintScreen"
	case KeyScrollLock:
		return "ScrollLock"
	case KeyNumLock:
		return "NumLock"
	case KeyCapsLock:
		return "CapsLock"
	case KeyKP0:
		return "KP0"
	case KeyKP1:
		return "KP1"
	case KeyKP2:
		return "KP2"
	case KeyKP3:
		return "KP3"
	case KeyKP4:
		return "KP4"
	case KeyKP5:
		return "KP5"
	case KeyKP6:
		return "KP6"
	case KeyKP7:
		return "KP7"
	case KeyKP8:
		return "KP8"
	case KeyKP9:
		return "KP9"
	case KeyKPAdd:
		return "KP+"
	case KeyKPSubtract:
		return "KP-"
	case KeyKPMultiply:
		return "KP*"
	case KeyKPDivide:
		return "KP/"
	case KeyKPDecimal:
		return "KP."
	case KeyKPEnter:
		return "KPEnter"
	case KeyRune:
		return "Rune"
	default:
		return fmt.Sprintf("Key(%d)", k)
	}
}

// IsSpecial returns true if this is a special (non-character) key.
func (k Key) IsSpecial() bool {
	return k != KeyNone && k != KeyRune
}

// IsFunctionKey returns true if this is a function key (F1-F12).
func (k Key) IsFunctionKey() bool {
	return k >= KeyF1 && k <= KeyF12
}

// IsArrowKey returns true if this is an arrow key.
func (k Key) IsArrowKey() bool {
	return k >= KeyUp && k <= KeyRight
}

// IsNavigationKey returns true if this is a navigation key.
func (k Key) IsNavigationKey() bool {
	return k.IsArrowKey() || k == KeyHome || k == KeyEnd || k == KeyPageUp || k == KeyPageDown
}

// IsKeypadKey returns true if this is a keypad key.
func (k Key) IsKeypadKey() bool {
	return k >= KeyKP0 && k <= KeyKPEnter
}

// keyNameMap maps key names (lowercase) to Key values.
var keyNameMap = map[string]Key{
	"none":        KeyNone,
	"escape":      KeyEscape,
	"esc":         KeyEscape,
	"enter":       KeyEnter,
	"return":      KeyEnter,
	"cr":          KeyEnter,
	"tab":         KeyTab,
	"backspace":   KeyBackspace,
	"bs":          KeyBackspace,
	"delete":      KeyDelete,
	"del":         KeyDelete,
	"insert":      KeyInsert,
	"ins":         KeyInsert,
	"home":        KeyHome,
	"end":         KeyEnd,
	"pageup":      KeyPageUp,
	"pgup":        KeyPageUp,
	"pagedown":    KeyPageDown,
	"pgdn":        KeyPageDown,
	"up":          KeyUp,
	"down":        KeyDown,
	"left":        KeyLeft,
	"right":       KeyRight,
	"f1":          KeyF1,
	"f2":          KeyF2,
	"f3":          KeyF3,
	"f4":          KeyF4,
	"f5":          KeyF5,
	"f6":          KeyF6,
	"f7":          KeyF7,
	"f8":          KeyF8,
	"f9":          KeyF9,
	"f10":         KeyF10,
	"f11":         KeyF11,
	"f12":         KeyF12,
	"space":       KeySpace,
	"pause":       KeyPause,
	"printscreen": KeyPrintScreen,
	"scrolllock":  KeyScrollLock,
	"numlock":     KeyNumLock,
	"capslock":    KeyCapsLock,
}

// KeyFromName returns the Key for a given name (case-insensitive).
// Returns KeyNone if the name is not recognized.
func KeyFromName(name string) Key {
	name = strings.ToLower(strings.TrimSpace(name))
	if k, ok := keyNameMap[name]; ok {
		return k
	}
	return KeyNone
}
