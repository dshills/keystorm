package key

import (
	"errors"
	"testing"
)

func TestParseSingleCharacter(t *testing.T) {
	tests := []struct {
		spec     string
		wantRune rune
		wantMod  Modifier
	}{
		{"a", 'a', ModNone},
		{"A", 'A', ModShift},
		{"1", '1', ModNone},
		{"@", '@', ModNone},
	}

	for _, tt := range tests {
		event, err := Parse(tt.spec)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", tt.spec, err)
			continue
		}
		if event.Key != KeyRune {
			t.Errorf("Parse(%q) key = %v, want KeyRune", tt.spec, event.Key)
		}
		if event.Rune != tt.wantRune {
			t.Errorf("Parse(%q) rune = %q, want %q", tt.spec, event.Rune, tt.wantRune)
		}
		if event.Modifiers != tt.wantMod {
			t.Errorf("Parse(%q) modifiers = %v, want %v", tt.spec, event.Modifiers, tt.wantMod)
		}
	}
}

func TestParseSpecialKeys(t *testing.T) {
	tests := []struct {
		spec    string
		wantKey Key
	}{
		{"Enter", KeyEnter},
		{"enter", KeyEnter},
		{"Escape", KeyEscape},
		{"escape", KeyEscape},
		{"Tab", KeyTab},
		{"Backspace", KeyBackspace},
		{"Space", KeySpace},
		{"Delete", KeyDelete},
		{"Up", KeyUp},
		{"Down", KeyDown},
		{"Left", KeyLeft},
		{"Right", KeyRight},
		{"Home", KeyHome},
		{"End", KeyEnd},
		{"PageUp", KeyPageUp},
		{"PageDown", KeyPageDown},
		{"F1", KeyF1},
		{"F12", KeyF12},
	}

	for _, tt := range tests {
		event, err := Parse(tt.spec)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", tt.spec, err)
			continue
		}
		if event.Key != tt.wantKey {
			t.Errorf("Parse(%q) key = %v, want %v", tt.spec, event.Key, tt.wantKey)
		}
	}
}

func TestParseModifierStyle(t *testing.T) {
	tests := []struct {
		spec     string
		wantKey  Key
		wantRune rune
		wantMod  Modifier
	}{
		{"Ctrl+s", KeyRune, 's', ModCtrl},
		{"Ctrl+S", KeyRune, 's', ModCtrl}, // Ctrl makes lowercase
		{"Alt+f", KeyRune, 'f', ModAlt},
		{"Ctrl+Alt+x", KeyRune, 'x', ModCtrl | ModAlt},
		{"Ctrl+Shift+p", KeyRune, 'p', ModCtrl | ModShift},
		{"Ctrl+Enter", KeyEnter, 0, ModCtrl},
		{"Alt+F4", KeyF4, 0, ModAlt},
	}

	for _, tt := range tests {
		event, err := Parse(tt.spec)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", tt.spec, err)
			continue
		}
		if event.Key != tt.wantKey {
			t.Errorf("Parse(%q) key = %v, want %v", tt.spec, event.Key, tt.wantKey)
		}
		if tt.wantKey == KeyRune && event.Rune != tt.wantRune {
			t.Errorf("Parse(%q) rune = %q, want %q", tt.spec, event.Rune, tt.wantRune)
		}
		if event.Modifiers != tt.wantMod {
			t.Errorf("Parse(%q) modifiers = %v, want %v", tt.spec, event.Modifiers, tt.wantMod)
		}
	}
}

func TestParseVimStyle(t *testing.T) {
	tests := []struct {
		spec     string
		wantKey  Key
		wantRune rune
		wantMod  Modifier
	}{
		{"<C-s>", KeyRune, 's', ModCtrl},
		{"<A-f>", KeyRune, 'f', ModAlt},
		{"<C-A-x>", KeyRune, 'x', ModCtrl | ModAlt},
		{"<C-S-p>", KeyRune, 'p', ModCtrl | ModShift},
		{"<M-a>", KeyRune, 'a', ModMeta},
		{"<D-s>", KeyRune, 's', ModMeta}, // D is Vim's meta/command
		{"<CR>", KeyEnter, 0, ModNone},
		{"<Esc>", KeyEscape, 0, ModNone},
		{"<Tab>", KeyTab, 0, ModNone},
		{"<BS>", KeyBackspace, 0, ModNone},
		{"<Del>", KeyDelete, 0, ModNone},
		{"<Space>", KeyRune, ' ', ModNone},
		{"<Up>", KeyUp, 0, ModNone},
		{"<Down>", KeyDown, 0, ModNone},
		{"<Left>", KeyLeft, 0, ModNone},
		{"<Right>", KeyRight, 0, ModNone},
		{"<Home>", KeyHome, 0, ModNone},
		{"<End>", KeyEnd, 0, ModNone},
		{"<PageUp>", KeyPageUp, 0, ModNone},
		{"<PageDown>", KeyPageDown, 0, ModNone},
		{"<F1>", KeyF1, 0, ModNone},
		{"<C-CR>", KeyEnter, 0, ModCtrl},
		{"<C-Tab>", KeyTab, 0, ModCtrl},
	}

	for _, tt := range tests {
		event, err := Parse(tt.spec)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", tt.spec, err)
			continue
		}
		if event.Key != tt.wantKey {
			t.Errorf("Parse(%q) key = %v, want %v", tt.spec, event.Key, tt.wantKey)
		}
		if tt.wantKey == KeyRune && event.Rune != tt.wantRune {
			t.Errorf("Parse(%q) rune = %q, want %q", tt.spec, event.Rune, tt.wantRune)
		}
		if event.Modifiers != tt.wantMod {
			t.Errorf("Parse(%q) modifiers = %v, want %v", tt.spec, event.Modifiers, tt.wantMod)
		}
	}
}

func TestParseVimAliases(t *testing.T) {
	// Test Vim-specific aliases
	tests := []struct {
		spec     string
		wantKey  Key
		wantRune rune
	}{
		{"<Return>", KeyEnter, 0},
		{"<Enter>", KeyEnter, 0},
		{"<lt>", KeyRune, '<'},
		{"<gt>", KeyRune, '>'},
		{"<Bar>", KeyRune, '|'},
		{"<Bslash>", KeyRune, '\\'},
	}

	for _, tt := range tests {
		event, err := Parse(tt.spec)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", tt.spec, err)
			continue
		}
		if event.Key != tt.wantKey {
			t.Errorf("Parse(%q) key = %v, want %v", tt.spec, event.Key, tt.wantKey)
		}
		if tt.wantKey == KeyRune && event.Rune != tt.wantRune {
			t.Errorf("Parse(%q) rune = %q, want %q", tt.spec, event.Rune, tt.wantRune)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		spec    string
		wantErr error
	}{
		{"", ErrEmptySpec},
		{"  ", ErrEmptySpec},
		{"<>", ErrInvalidSpec},
		{"<C->", ErrInvalidSpec},
		{"<X-a>", ErrInvalidSpec}, // Unknown modifier
		{"Ctrl+", ErrInvalidSpec},
		{"Unknown+a", ErrInvalidSpec},
		{"unknownkey", ErrInvalidSpec},
	}

	for _, tt := range tests {
		_, err := Parse(tt.spec)
		if err == nil {
			t.Errorf("Parse(%q) expected error", tt.spec)
			continue
		}
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("Parse(%q) error = %v, want %v", tt.spec, err, tt.wantErr)
		}
	}
}

func TestMustParse(t *testing.T) {
	// Valid spec should not panic
	event := MustParse("Ctrl+s")
	if event.Key != KeyRune || event.Rune != 's' {
		t.Error("MustParse valid spec failed")
	}

	// Invalid spec should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParse should panic on invalid spec")
		}
	}()
	MustParse("")
}

func TestFormatSpec(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{NewRuneEvent('a', ModNone), "a"},
		{NewRuneEvent('s', ModCtrl), "<C-s>"},
		{NewSpecialEvent(KeyEscape, ModNone), "<Esc>"},
		{NewSpecialEvent(KeyEnter, ModNone), "<CR>"},
	}

	for _, tt := range tests {
		if got := FormatSpec(tt.event); got != tt.want {
			t.Errorf("FormatSpec(%+v) = %q, want %q", tt.event, got, tt.want)
		}
	}
}

func TestNormalizeSpec(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Ctrl+s", "<C-s>"},
		{"<C-s>", "<C-s>"},
		{"Enter", "<CR>"},
		{"<CR>", "<CR>"},
		{"a", "a"},
	}

	for _, tt := range tests {
		got, err := NormalizeSpec(tt.input)
		if err != nil {
			t.Errorf("NormalizeSpec(%q) error = %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("NormalizeSpec(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseSequence(t *testing.T) {
	tests := []struct {
		input   string
		wantLen int
		wantVim string
	}{
		{"", 0, ""},
		{"g g", 2, "gg"},
		{"d i w", 3, "diw"},
		{"gg", 2, "gg"},
		{"diw", 3, "diw"},
		{"<C-x><C-s>", 2, "<C-x><C-s>"},
		{"<C-x> <C-s>", 2, "<C-x><C-s>"},
		{"dd", 2, "dd"},
	}

	for _, tt := range tests {
		seq, err := ParseSequence(tt.input)
		if err != nil {
			t.Errorf("ParseSequence(%q) error = %v", tt.input, err)
			continue
		}
		if seq.Len() != tt.wantLen {
			t.Errorf("ParseSequence(%q) len = %d, want %d", tt.input, seq.Len(), tt.wantLen)
		}
		if got := seq.VimString(); got != tt.wantVim {
			t.Errorf("ParseSequence(%q).VimString() = %q, want %q", tt.input, got, tt.wantVim)
		}
	}
}

func TestParseSequenceWithUnmatchedBracket(t *testing.T) {
	// Unmatched < is treated as literal character
	seq, err := ParseSequence("a<b")
	if err != nil {
		t.Errorf("ParseSequence(\"a<b\") error = %v", err)
		return
	}
	if seq.Len() != 3 {
		t.Errorf("ParseSequence(\"a<b\") len = %d, want 3", seq.Len())
	}
}

func TestMustParseSequence(t *testing.T) {
	// Valid sequence should not panic
	seq := MustParseSequence("g g")
	if seq.Len() != 2 {
		t.Error("MustParseSequence valid sequence failed")
	}

	// Invalid sequence should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParseSequence should panic on invalid sequence")
		}
	}()
	MustParseSequence("<C->")
}

func TestParseRoundTrip(t *testing.T) {
	// Parse -> Format -> Parse should give same result
	specs := []string{
		"a", "A", "Ctrl+s", "<C-s>", "<Esc>", "<CR>",
		"<C-A-x>", "<F1>", "<Space>", "<Up>",
	}

	for _, spec := range specs {
		event1, err := Parse(spec)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", spec, err)
			continue
		}

		formatted := FormatSpec(event1)
		event2, err := Parse(formatted)
		if err != nil {
			t.Errorf("Parse(FormatSpec(%q)) error = %v", spec, err)
			continue
		}

		if !event1.Equals(event2) {
			t.Errorf("Round trip failed for %q: %+v != %+v", spec, event1, event2)
		}
	}
}
