package key

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Parse errors
var (
	ErrEmptySpec        = errors.New("empty key specification")
	ErrInvalidSpec      = errors.New("invalid key specification")
	ErrUnmatchedBracket = errors.New("unmatched bracket in key specification")
)

// Parse parses a key specification string into a KeyEvent.
//
// Supported formats:
//   - Single character: "a", "A", "1", "@"
//   - Special keys: "Enter", "Escape", "Tab", "Backspace", "Space"
//   - With modifiers: "Ctrl+S", "Alt+F4", "Ctrl+Shift+P"
//   - Vim-style: "<C-s>", "<A-f>", "<C-S-p>", "<CR>", "<Esc>"
//   - Vim aliases: "<Enter>" -> Enter, "<Return>" -> Enter, "<BS>" -> Backspace
func Parse(spec string) (Event, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return Event{}, ErrEmptySpec
	}

	// Check for Vim-style <...> notation
	if strings.HasPrefix(spec, "<") && strings.HasSuffix(spec, ">") {
		return parseVimStyle(spec[1 : len(spec)-1])
	}

	// Check for modifier+key format (Ctrl+S, Alt+F4)
	if strings.Contains(spec, "+") {
		return parseModifierStyle(spec)
	}

	// Single character or key name
	return parseSingle(spec)
}

// parseVimStyle parses Vim-style notation like "C-s", "A-F4", "CR", "Esc"
func parseVimStyle(inner string) (Event, error) {
	if inner == "" {
		return Event{}, ErrInvalidSpec
	}

	inner = strings.TrimSpace(inner)

	// Split by hyphen to get modifiers and key
	parts := strings.Split(inner, "-")

	var mods Modifier
	var keyPart string

	if len(parts) == 1 {
		// No modifiers, just key name
		keyPart = parts[0]
	} else {
		// Last part is the key, rest are modifiers
		keyPart = parts[len(parts)-1]
		for _, p := range parts[:len(parts)-1] {
			p = strings.TrimSpace(p)
			p = strings.ToLower(p)
			switch p {
			case "c":
				mods = mods.With(ModCtrl)
			case "a":
				mods = mods.With(ModAlt)
			case "s":
				mods = mods.With(ModShift)
			case "m", "d": // D is Vim's notation for Command/Meta
				mods = mods.With(ModMeta)
			default:
				return Event{}, fmt.Errorf("%w: unknown modifier %q", ErrInvalidSpec, p)
			}
		}
	}

	// Parse the key part
	return parseKeyWithModifiers(keyPart, mods)
}

// parseModifierStyle parses "Ctrl+S" style notation
func parseModifierStyle(spec string) (Event, error) {
	parts := strings.Split(spec, "+")
	if len(parts) < 2 {
		return Event{}, ErrInvalidSpec
	}

	var mods Modifier

	// All but the last part are modifiers
	for _, p := range parts[:len(parts)-1] {
		p = strings.TrimSpace(p)
		mod := ModifierFromName(strings.ToLower(p))
		if mod == ModNone {
			return Event{}, fmt.Errorf("%w: unknown modifier %q", ErrInvalidSpec, p)
		}
		mods = mods.With(mod)
	}

	// Last part is the key
	keyPart := strings.TrimSpace(parts[len(parts)-1])
	return parseKeyWithModifiers(keyPart, mods)
}

// parseSingle parses a single character or key name
func parseSingle(spec string) (Event, error) {
	// Check for special key names first
	lowerSpec := strings.ToLower(spec)
	if key := KeyFromName(lowerSpec); key != KeyNone {
		return NewSpecialEvent(key, ModNone), nil
	}

	// Single character
	runes := []rune(spec)
	if len(runes) == 1 {
		r := runes[0]
		var mods Modifier
		// Uppercase letters have implicit Shift
		if unicode.IsUpper(r) {
			mods = ModShift
		}
		return NewRuneEvent(r, mods), nil
	}

	return Event{}, fmt.Errorf("%w: %q", ErrInvalidSpec, spec)
}

// parseKeyWithModifiers parses a key part with already-known modifiers
func parseKeyWithModifiers(keyPart string, mods Modifier) (Event, error) {
	keyPart = strings.TrimSpace(keyPart)
	if keyPart == "" {
		return Event{}, ErrInvalidSpec
	}

	// Check for special key names
	lowerKey := strings.ToLower(keyPart)

	// Common Vim aliases
	switch lowerKey {
	case "cr", "return", "enter":
		return NewSpecialEvent(KeyEnter, mods), nil
	case "esc", "escape":
		return NewSpecialEvent(KeyEscape, mods), nil
	case "tab":
		return NewSpecialEvent(KeyTab, mods), nil
	case "bs", "backspace":
		return NewSpecialEvent(KeyBackspace, mods), nil
	case "del", "delete":
		return NewSpecialEvent(KeyDelete, mods), nil
	case "ins", "insert":
		return NewSpecialEvent(KeyInsert, mods), nil
	case "space":
		return NewRuneEvent(' ', mods), nil
	case "lt":
		return NewRuneEvent('<', mods), nil
	case "gt":
		return NewRuneEvent('>', mods), nil
	case "bar":
		return NewRuneEvent('|', mods), nil
	case "bslash":
		return NewRuneEvent('\\', mods), nil
	case "up":
		return NewSpecialEvent(KeyUp, mods), nil
	case "down":
		return NewSpecialEvent(KeyDown, mods), nil
	case "left":
		return NewSpecialEvent(KeyLeft, mods), nil
	case "right":
		return NewSpecialEvent(KeyRight, mods), nil
	case "home":
		return NewSpecialEvent(KeyHome, mods), nil
	case "end":
		return NewSpecialEvent(KeyEnd, mods), nil
	case "pageup", "pgup":
		return NewSpecialEvent(KeyPageUp, mods), nil
	case "pagedown", "pgdn":
		return NewSpecialEvent(KeyPageDown, mods), nil
	case "f1":
		return NewSpecialEvent(KeyF1, mods), nil
	case "f2":
		return NewSpecialEvent(KeyF2, mods), nil
	case "f3":
		return NewSpecialEvent(KeyF3, mods), nil
	case "f4":
		return NewSpecialEvent(KeyF4, mods), nil
	case "f5":
		return NewSpecialEvent(KeyF5, mods), nil
	case "f6":
		return NewSpecialEvent(KeyF6, mods), nil
	case "f7":
		return NewSpecialEvent(KeyF7, mods), nil
	case "f8":
		return NewSpecialEvent(KeyF8, mods), nil
	case "f9":
		return NewSpecialEvent(KeyF9, mods), nil
	case "f10":
		return NewSpecialEvent(KeyF10, mods), nil
	case "f11":
		return NewSpecialEvent(KeyF11, mods), nil
	case "f12":
		return NewSpecialEvent(KeyF12, mods), nil
	}

	// Check for other special keys
	if key := KeyFromName(lowerKey); key != KeyNone {
		return NewSpecialEvent(key, mods), nil
	}

	// Single character
	runes := []rune(keyPart)
	if len(runes) == 1 {
		r := runes[0]
		// For Ctrl combinations, use lowercase
		if mods.HasCtrl() {
			r = unicode.ToLower(r)
		}
		return NewRuneEvent(r, mods), nil
	}

	return Event{}, fmt.Errorf("%w: unknown key %q", ErrInvalidSpec, keyPart)
}

// MustParse parses a key specification and panics on error.
// Use only for known-valid specs in initialization code.
func MustParse(spec string) Event {
	event, err := Parse(spec)
	if err != nil {
		panic("invalid key specification: " + spec + ": " + err.Error())
	}
	return event
}

// FormatSpec formats a key event as a specification string.
// This produces a canonical form that can be parsed back.
func FormatSpec(event Event) string {
	return event.VimString()
}

// NormalizeSpec parses and re-formats a key specification to its canonical form.
func NormalizeSpec(spec string) (string, error) {
	event, err := Parse(spec)
	if err != nil {
		return "", err
	}
	return FormatSpec(event), nil
}
