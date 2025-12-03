package key

import (
	"testing"
)

func TestKeyString(t *testing.T) {
	tests := []struct {
		key  Key
		want string
	}{
		{KeyNone, "None"},
		{KeyEscape, "Escape"},
		{KeyEnter, "Enter"},
		{KeyTab, "Tab"},
		{KeyBackspace, "Backspace"},
		{KeyDelete, "Delete"},
		{KeyUp, "Up"},
		{KeyDown, "Down"},
		{KeyLeft, "Left"},
		{KeyRight, "Right"},
		{KeyF1, "F1"},
		{KeyF12, "F12"},
		{KeySpace, "Space"},
		{KeyRune, "Rune"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.key.String(); got != tt.want {
				t.Errorf("Key.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKeyIsSpecial(t *testing.T) {
	tests := []struct {
		key  Key
		want bool
	}{
		{KeyNone, false},
		{KeyRune, false},
		{KeyEscape, true},
		{KeyEnter, true},
		{KeyF1, true},
		{KeyUp, true},
	}

	for _, tt := range tests {
		t.Run(tt.key.String(), func(t *testing.T) {
			if got := tt.key.IsSpecial(); got != tt.want {
				t.Errorf("Key.IsSpecial() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyIsFunctionKey(t *testing.T) {
	tests := []struct {
		key  Key
		want bool
	}{
		{KeyF1, true},
		{KeyF6, true},
		{KeyF12, true},
		{KeyEscape, false},
		{KeyEnter, false},
		{KeyRune, false},
	}

	for _, tt := range tests {
		t.Run(tt.key.String(), func(t *testing.T) {
			if got := tt.key.IsFunctionKey(); got != tt.want {
				t.Errorf("Key.IsFunctionKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyIsArrowKey(t *testing.T) {
	tests := []struct {
		key  Key
		want bool
	}{
		{KeyUp, true},
		{KeyDown, true},
		{KeyLeft, true},
		{KeyRight, true},
		{KeyEscape, false},
		{KeyEnter, false},
		{KeyHome, false},
	}

	for _, tt := range tests {
		t.Run(tt.key.String(), func(t *testing.T) {
			if got := tt.key.IsArrowKey(); got != tt.want {
				t.Errorf("Key.IsArrowKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyIsNavigationKey(t *testing.T) {
	tests := []struct {
		key  Key
		want bool
	}{
		{KeyUp, true},
		{KeyDown, true},
		{KeyLeft, true},
		{KeyRight, true},
		{KeyHome, true},
		{KeyEnd, true},
		{KeyPageUp, true},
		{KeyPageDown, true},
		{KeyEscape, false},
		{KeyEnter, false},
	}

	for _, tt := range tests {
		t.Run(tt.key.String(), func(t *testing.T) {
			if got := tt.key.IsNavigationKey(); got != tt.want {
				t.Errorf("Key.IsNavigationKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyFromName(t *testing.T) {
	tests := []struct {
		name string
		want Key
	}{
		{"escape", KeyEscape},
		{"esc", KeyEscape},
		{"enter", KeyEnter},
		{"return", KeyEnter},
		{"cr", KeyEnter},
		{"tab", KeyTab},
		{"backspace", KeyBackspace},
		{"bs", KeyBackspace},
		{"delete", KeyDelete},
		{"del", KeyDelete},
		{"up", KeyUp},
		{"down", KeyDown},
		{"left", KeyLeft},
		{"right", KeyRight},
		{"f1", KeyF1},
		{"f12", KeyF12},
		{"space", KeySpace},
		{"pageup", KeyPageUp},
		{"pgup", KeyPageUp},
		{"pagedown", KeyPageDown},
		{"pgdn", KeyPageDown},
		{"unknown", KeyNone},
		{"", KeyNone},
		// Case-insensitive tests
		{"ESCAPE", KeyEscape},
		{"Escape", KeyEscape},
		{"F1", KeyF1},
		{"  space  ", KeySpace}, // With whitespace
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KeyFromName(tt.name); got != tt.want {
				t.Errorf("KeyFromName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
