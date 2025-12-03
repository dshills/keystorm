package key

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Event represents a single key press event.
type Event struct {
	// Key identifies the key pressed.
	Key Key

	// Rune is the character for KeyRune events.
	Rune rune

	// Modifiers contains the active modifier keys.
	Modifiers Modifier

	// Timestamp is when the event occurred.
	Timestamp time.Time
}

// NewEvent creates a key event with the current timestamp.
func NewEvent(key Key, r rune, mods Modifier) Event {
	return Event{
		Key:       key,
		Rune:      r,
		Modifiers: mods,
		Timestamp: time.Now(),
	}
}

// NewRuneEvent creates a key event for a character.
func NewRuneEvent(r rune, mods Modifier) Event {
	return Event{
		Key:       KeyRune,
		Rune:      r,
		Modifiers: mods,
		Timestamp: time.Now(),
	}
}

// NewSpecialEvent creates a key event for a special key.
func NewSpecialEvent(key Key, mods Modifier) Event {
	return Event{
		Key:       key,
		Modifiers: mods,
		Timestamp: time.Now(),
	}
}

// IsRune returns true if this is a character key event.
func (e Event) IsRune() bool {
	return e.Key == KeyRune && e.Rune != 0
}

// IsChar returns true if this is a printable character.
func (e Event) IsChar() bool {
	return e.IsRune() && unicode.IsPrint(e.Rune)
}

// IsModified returns true if any modifier is pressed.
// For character events, Shift alone is not considered modified
// (since Shift changes the character itself).
func (e Event) IsModified() bool {
	if e.IsRune() {
		// For characters, Shift is part of the character
		return e.Modifiers&(ModCtrl|ModAlt|ModMeta) != 0
	}
	return e.Modifiers != ModNone
}

// IsSpecial returns true if this is a special (non-character) key.
func (e Event) IsSpecial() bool {
	return e.Key.IsSpecial()
}

// String returns a canonical string representation.
// Examples: "a", "A", "Ctrl+S", "C-s", "Enter", "<C-S-p>"
func (e Event) String() string {
	var parts []string

	// Add modifiers
	if e.Modifiers.HasCtrl() {
		parts = append(parts, "C")
	}
	if e.Modifiers.HasAlt() {
		parts = append(parts, "A")
	}
	if e.Modifiers.HasMeta() {
		parts = append(parts, "M")
	}
	// Only show Shift for non-character keys
	if e.Modifiers.HasShift() && !e.IsRune() {
		parts = append(parts, "S")
	}

	// Add key name
	var keyName string
	switch e.Key {
	case KeyRune:
		if e.Rune == ' ' {
			keyName = "Space"
		} else {
			keyName = string(e.Rune)
		}
	case KeyEscape:
		keyName = "Esc"
	case KeyEnter:
		keyName = "Enter"
	case KeyTab:
		keyName = "Tab"
	case KeyBackspace:
		keyName = "BS"
	case KeyDelete:
		keyName = "Del"
	case KeyInsert:
		keyName = "Ins"
	case KeyHome:
		keyName = "Home"
	case KeyEnd:
		keyName = "End"
	case KeyPageUp:
		keyName = "PgUp"
	case KeyPageDown:
		keyName = "PgDn"
	case KeyUp:
		keyName = "Up"
	case KeyDown:
		keyName = "Down"
	case KeyLeft:
		keyName = "Left"
	case KeyRight:
		keyName = "Right"
	case KeySpace:
		keyName = "Space"
	default:
		keyName = e.Key.String()
	}

	parts = append(parts, keyName)

	// Join with hyphen for consistency with Vim notation
	return strings.Join(parts, "-")
}

// VimString returns a Vim-style string representation.
// Examples: "<Esc>", "<C-s>", "<C-S-p>", "<CR>", "a", "A"
func (e Event) VimString() string {
	// Simple characters without modifiers (except Shift)
	if e.IsRune() && !e.IsModified() {
		if e.Rune == ' ' {
			return "<Space>"
		}
		return string(e.Rune)
	}

	// Build Vim-style <...> notation
	var parts []string

	if e.Modifiers.HasCtrl() {
		parts = append(parts, "C")
	}
	if e.Modifiers.HasAlt() {
		parts = append(parts, "A")
	}
	if e.Modifiers.HasMeta() {
		parts = append(parts, "D") // Vim uses D for command/meta
	}
	if e.Modifiers.HasShift() && !e.IsRune() {
		parts = append(parts, "S")
	}

	// Key name
	var keyName string
	switch e.Key {
	case KeyRune:
		keyName = strings.ToLower(string(e.Rune))
	case KeyEscape:
		keyName = "Esc"
	case KeyEnter:
		keyName = "CR"
	case KeyTab:
		keyName = "Tab"
	case KeyBackspace:
		keyName = "BS"
	case KeyDelete:
		keyName = "Del"
	case KeySpace:
		keyName = "Space"
	case KeyUp:
		keyName = "Up"
	case KeyDown:
		keyName = "Down"
	case KeyLeft:
		keyName = "Left"
	case KeyRight:
		keyName = "Right"
	case KeyHome:
		keyName = "Home"
	case KeyEnd:
		keyName = "End"
	case KeyPageUp:
		keyName = "PageUp"
	case KeyPageDown:
		keyName = "PageDown"
	default:
		keyName = e.Key.String()
	}

	parts = append(parts, keyName)

	return "<" + strings.Join(parts, "-") + ">"
}

// Equals returns true if two events represent the same key press.
// Timestamps are not compared.
func (e Event) Equals(other Event) bool {
	return e.Key == other.Key &&
		e.Rune == other.Rune &&
		e.Modifiers == other.Modifiers
}

// Matches checks if this event matches a key specification string.
func (e Event) Matches(spec string) bool {
	parsed, err := Parse(spec)
	if err != nil {
		return false
	}
	return e.Equals(parsed)
}

// IsEscape returns true if this is the Escape key (with no modifiers).
func (e Event) IsEscape() bool {
	return e.Key == KeyEscape && e.Modifiers == ModNone
}

// IsEnter returns true if this is the Enter key (with no modifiers).
func (e Event) IsEnter() bool {
	return e.Key == KeyEnter && e.Modifiers == ModNone
}

// IsBackspace returns true if this is Backspace (with no modifiers).
func (e Event) IsBackspace() bool {
	return e.Key == KeyBackspace && e.Modifiers == ModNone
}

// IsTab returns true if this is Tab (with no modifiers).
func (e Event) IsTab() bool {
	return e.Key == KeyTab && e.Modifiers == ModNone
}

// Clone returns a copy of the event.
func (e Event) Clone() Event {
	return Event{
		Key:       e.Key,
		Rune:      e.Rune,
		Modifiers: e.Modifiers,
		Timestamp: e.Timestamp,
	}
}

// WithModifier returns a copy with the specified modifier added.
func (e Event) WithModifier(mod Modifier) Event {
	clone := e.Clone()
	clone.Modifiers = clone.Modifiers.With(mod)
	return clone
}

// GoString implements fmt.GoStringer for debugging.
func (e Event) GoString() string {
	return fmt.Sprintf("Event{Key: %s, Rune: %q, Modifiers: %s}",
		e.Key.String(), e.Rune, e.Modifiers.String())
}
