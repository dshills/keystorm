package key

import "strings"

// Modifier represents keyboard modifier keys.
type Modifier uint8

const (
	// ModNone indicates no modifiers.
	ModNone Modifier = 0

	// ModShift indicates the Shift key.
	ModShift Modifier = 1 << iota

	// ModCtrl indicates the Control key.
	ModCtrl

	// ModAlt indicates the Alt key (Option on macOS).
	ModAlt

	// ModMeta indicates the Meta key (Cmd on macOS, Win on Windows).
	ModMeta
)

// Has returns true if m contains the specified modifier.
func (m Modifier) Has(mod Modifier) bool {
	return m&mod != 0
}

// HasShift returns true if Shift is pressed.
func (m Modifier) HasShift() bool {
	return m.Has(ModShift)
}

// HasCtrl returns true if Control is pressed.
func (m Modifier) HasCtrl() bool {
	return m.Has(ModCtrl)
}

// HasAlt returns true if Alt is pressed.
func (m Modifier) HasAlt() bool {
	return m.Has(ModAlt)
}

// HasMeta returns true if Meta is pressed.
func (m Modifier) HasMeta() bool {
	return m.Has(ModMeta)
}

// With returns a new Modifier with the specified modifier added.
func (m Modifier) With(mod Modifier) Modifier {
	return m | mod
}

// Without returns a new Modifier with the specified modifier removed.
func (m Modifier) Without(mod Modifier) Modifier {
	return m &^ mod
}

// IsEmpty returns true if no modifiers are set.
func (m Modifier) IsEmpty() bool {
	return m == ModNone
}

// String returns a human-readable representation like "Ctrl+Alt" or "C-A".
func (m Modifier) String() string {
	if m == ModNone {
		return ""
	}

	var parts []string
	if m.HasCtrl() {
		parts = append(parts, "Ctrl")
	}
	if m.HasAlt() {
		parts = append(parts, "Alt")
	}
	if m.HasShift() {
		parts = append(parts, "Shift")
	}
	if m.HasMeta() {
		parts = append(parts, "Meta")
	}
	return strings.Join(parts, "+")
}

// ShortString returns a compact representation like "C-A-S-M".
func (m Modifier) ShortString() string {
	if m == ModNone {
		return ""
	}

	var parts []string
	if m.HasCtrl() {
		parts = append(parts, "C")
	}
	if m.HasAlt() {
		parts = append(parts, "A")
	}
	if m.HasShift() {
		parts = append(parts, "S")
	}
	if m.HasMeta() {
		parts = append(parts, "M")
	}
	return strings.Join(parts, "-")
}

// modifierNameMap maps modifier names (lowercase) to Modifier values.
var modifierNameMap = map[string]Modifier{
	"ctrl":    ModCtrl,
	"control": ModCtrl,
	"c":       ModCtrl,
	"alt":     ModAlt,
	"a":       ModAlt,
	"option":  ModAlt,
	"opt":     ModAlt,
	"shift":   ModShift,
	"s":       ModShift,
	"meta":    ModMeta,
	"m":       ModMeta,
	"cmd":     ModMeta,
	"command": ModMeta,
	"win":     ModMeta,
	"super":   ModMeta,
	"d":       ModMeta, // Vim uses D for command/meta
}

// ModifierFromName returns the Modifier for a given name (case-insensitive).
// Returns ModNone if the name is not recognized.
func ModifierFromName(name string) Modifier {
	if m, ok := modifierNameMap[name]; ok {
		return m
	}
	return ModNone
}

// ParseModifiers parses a modifier string like "Ctrl+Alt" or "C-A".
// Returns the combined modifiers and any error.
func ParseModifiers(s string) Modifier {
	s = strings.ToLower(s)
	var result Modifier

	// Try splitting by common separators
	var parts []string
	if strings.Contains(s, "+") {
		parts = strings.Split(s, "+")
	} else if strings.Contains(s, "-") {
		parts = strings.Split(s, "-")
	} else {
		// Single modifier
		parts = []string{s}
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if mod := ModifierFromName(part); mod != ModNone {
			result = result.With(mod)
		}
	}

	return result
}
