package key

import (
	"testing"
)

func TestNewRuneEvent(t *testing.T) {
	e := NewRuneEvent('a', ModNone)
	if e.Key != KeyRune {
		t.Errorf("NewRuneEvent key = %v, want KeyRune", e.Key)
	}
	if e.Rune != 'a' {
		t.Errorf("NewRuneEvent rune = %q, want 'a'", e.Rune)
	}
	if e.Modifiers != ModNone {
		t.Errorf("NewRuneEvent modifiers = %v, want ModNone", e.Modifiers)
	}
}

func TestNewSpecialEvent(t *testing.T) {
	e := NewSpecialEvent(KeyEscape, ModNone)
	if e.Key != KeyEscape {
		t.Errorf("NewSpecialEvent key = %v, want KeyEscape", e.Key)
	}
	if e.Rune != 0 {
		t.Errorf("NewSpecialEvent rune = %q, want 0", e.Rune)
	}
}

func TestEventIsRune(t *testing.T) {
	tests := []struct {
		event Event
		want  bool
	}{
		{NewRuneEvent('a', ModNone), true},
		{NewRuneEvent('A', ModShift), true},
		{NewSpecialEvent(KeyEscape, ModNone), false},
		{NewSpecialEvent(KeyEnter, ModNone), false},
		{Event{Key: KeyRune, Rune: 0}, false}, // Zero rune
	}

	for _, tt := range tests {
		if got := tt.event.IsRune(); got != tt.want {
			t.Errorf("Event.IsRune() = %v, want %v for %+v", got, tt.want, tt.event)
		}
	}
}

func TestEventIsChar(t *testing.T) {
	tests := []struct {
		event Event
		want  bool
	}{
		{NewRuneEvent('a', ModNone), true},
		{NewRuneEvent(' ', ModNone), true},
		{NewRuneEvent('\n', ModNone), false}, // Not printable
		{NewSpecialEvent(KeyEscape, ModNone), false},
	}

	for _, tt := range tests {
		if got := tt.event.IsChar(); got != tt.want {
			t.Errorf("Event.IsChar() = %v, want %v for %+v", got, tt.want, tt.event)
		}
	}
}

func TestEventIsModified(t *testing.T) {
	tests := []struct {
		event Event
		want  bool
	}{
		{NewRuneEvent('a', ModNone), false},
		{NewRuneEvent('A', ModShift), false}, // Shift alone doesn't count for runes
		{NewRuneEvent('a', ModCtrl), true},
		{NewRuneEvent('a', ModAlt), true},
		{NewSpecialEvent(KeyEscape, ModNone), false},
		{NewSpecialEvent(KeyEscape, ModShift), true}, // Shift counts for special keys
		{NewSpecialEvent(KeyEnter, ModCtrl), true},
	}

	for _, tt := range tests {
		if got := tt.event.IsModified(); got != tt.want {
			t.Errorf("Event.IsModified() = %v, want %v for %+v", got, tt.want, tt.event)
		}
	}
}

func TestEventEquals(t *testing.T) {
	tests := []struct {
		a, b Event
		want bool
	}{
		{NewRuneEvent('a', ModNone), NewRuneEvent('a', ModNone), true},
		{NewRuneEvent('a', ModNone), NewRuneEvent('b', ModNone), false},
		{NewRuneEvent('a', ModNone), NewRuneEvent('a', ModCtrl), false},
		{NewSpecialEvent(KeyEscape, ModNone), NewSpecialEvent(KeyEscape, ModNone), true},
		{NewSpecialEvent(KeyEscape, ModNone), NewSpecialEvent(KeyEnter, ModNone), false},
	}

	for _, tt := range tests {
		if got := tt.a.Equals(tt.b); got != tt.want {
			t.Errorf("%+v.Equals(%+v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestEventString(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{NewRuneEvent('a', ModNone), "a"},
		{NewRuneEvent('A', ModShift), "A"}, // Shift implicit for uppercase
		{NewRuneEvent('s', ModCtrl), "C-s"},
		{NewRuneEvent('f', ModCtrl|ModAlt), "C-A-f"},
		{NewSpecialEvent(KeyEscape, ModNone), "Esc"},
		{NewSpecialEvent(KeyEnter, ModNone), "Enter"},
		{NewSpecialEvent(KeyEnter, ModCtrl), "C-Enter"},
		{NewRuneEvent(' ', ModNone), "Space"},
	}

	for _, tt := range tests {
		if got := tt.event.String(); got != tt.want {
			t.Errorf("Event.String() = %q, want %q for %+v", got, tt.want, tt.event)
		}
	}
}

func TestEventVimString(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{NewRuneEvent('a', ModNone), "a"},
		{NewRuneEvent('A', ModShift), "A"},
		{NewRuneEvent('s', ModCtrl), "<C-s>"},
		{NewRuneEvent('f', ModCtrl|ModAlt), "<C-A-f>"},
		{NewSpecialEvent(KeyEscape, ModNone), "<Esc>"},
		{NewSpecialEvent(KeyEnter, ModNone), "<CR>"},
		{NewSpecialEvent(KeyEnter, ModCtrl), "<C-CR>"},
		{NewRuneEvent(' ', ModNone), "<Space>"},
	}

	for _, tt := range tests {
		if got := tt.event.VimString(); got != tt.want {
			t.Errorf("Event.VimString() = %q, want %q for %+v", got, tt.want, tt.event)
		}
	}
}

func TestEventMatches(t *testing.T) {
	tests := []struct {
		event Event
		spec  string
		want  bool
	}{
		{NewRuneEvent('a', ModNone), "a", true},
		{NewRuneEvent('s', ModCtrl), "Ctrl+s", true},
		{NewRuneEvent('s', ModCtrl), "<C-s>", true},
		{NewSpecialEvent(KeyEscape, ModNone), "Escape", true},
		{NewSpecialEvent(KeyEscape, ModNone), "<Esc>", true},
		{NewSpecialEvent(KeyEnter, ModNone), "Enter", true},
		{NewSpecialEvent(KeyEnter, ModNone), "<CR>", true},
		{NewRuneEvent('a', ModNone), "b", false},
		{NewRuneEvent('a', ModNone), "Ctrl+a", false},
	}

	for _, tt := range tests {
		if got := tt.event.Matches(tt.spec); got != tt.want {
			t.Errorf("Event.Matches(%q) = %v, want %v for %+v", tt.spec, got, tt.want, tt.event)
		}
	}
}

func TestEventIsEscape(t *testing.T) {
	if !NewSpecialEvent(KeyEscape, ModNone).IsEscape() {
		t.Error("KeyEscape without modifiers should be Escape")
	}
	if NewSpecialEvent(KeyEscape, ModCtrl).IsEscape() {
		t.Error("KeyEscape with Ctrl should not be plain Escape")
	}
	if NewSpecialEvent(KeyEnter, ModNone).IsEscape() {
		t.Error("KeyEnter should not be Escape")
	}
}

func TestEventIsEnter(t *testing.T) {
	if !NewSpecialEvent(KeyEnter, ModNone).IsEnter() {
		t.Error("KeyEnter without modifiers should be Enter")
	}
	if NewSpecialEvent(KeyEnter, ModCtrl).IsEnter() {
		t.Error("KeyEnter with Ctrl should not be plain Enter")
	}
}

func TestEventWithModifier(t *testing.T) {
	e := NewRuneEvent('a', ModNone)
	e2 := e.WithModifier(ModCtrl)

	if e.Modifiers != ModNone {
		t.Error("Original event should not be modified")
	}
	if !e2.Modifiers.HasCtrl() {
		t.Error("New event should have Ctrl")
	}
}
