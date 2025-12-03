package key

import (
	"testing"
)

func TestModifierHas(t *testing.T) {
	tests := []struct {
		mod    Modifier
		check  Modifier
		expect bool
	}{
		{ModNone, ModCtrl, false},
		{ModCtrl, ModCtrl, true},
		{ModCtrl | ModAlt, ModCtrl, true},
		{ModCtrl | ModAlt, ModAlt, true},
		{ModCtrl | ModAlt, ModShift, false},
		{ModCtrl | ModAlt | ModShift | ModMeta, ModMeta, true},
	}

	for _, tt := range tests {
		if got := tt.mod.Has(tt.check); got != tt.expect {
			t.Errorf("Modifier(%d).Has(%d) = %v, want %v", tt.mod, tt.check, got, tt.expect)
		}
	}
}

func TestModifierWith(t *testing.T) {
	mod := ModNone
	mod = mod.With(ModCtrl)
	if !mod.HasCtrl() {
		t.Error("With(ModCtrl) should set Ctrl")
	}

	mod = mod.With(ModAlt)
	if !mod.HasCtrl() || !mod.HasAlt() {
		t.Error("With(ModAlt) should keep Ctrl and add Alt")
	}
}

func TestModifierWithout(t *testing.T) {
	mod := ModCtrl | ModAlt | ModShift
	mod = mod.Without(ModAlt)
	if mod.HasAlt() {
		t.Error("Without(ModAlt) should remove Alt")
	}
	if !mod.HasCtrl() || !mod.HasShift() {
		t.Error("Without(ModAlt) should keep Ctrl and Shift")
	}
}

func TestModifierString(t *testing.T) {
	tests := []struct {
		mod  Modifier
		want string
	}{
		{ModNone, ""},
		{ModCtrl, "Ctrl"},
		{ModAlt, "Alt"},
		{ModShift, "Shift"},
		{ModMeta, "Meta"},
		{ModCtrl | ModAlt, "Ctrl+Alt"},
		{ModCtrl | ModShift, "Ctrl+Shift"},
		{ModCtrl | ModAlt | ModShift | ModMeta, "Ctrl+Alt+Shift+Meta"},
	}

	for _, tt := range tests {
		if got := tt.mod.String(); got != tt.want {
			t.Errorf("Modifier(%d).String() = %q, want %q", tt.mod, got, tt.want)
		}
	}
}

func TestModifierShortString(t *testing.T) {
	tests := []struct {
		mod  Modifier
		want string
	}{
		{ModNone, ""},
		{ModCtrl, "C"},
		{ModAlt, "A"},
		{ModShift, "S"},
		{ModMeta, "M"},
		{ModCtrl | ModAlt, "C-A"},
		{ModCtrl | ModAlt | ModShift | ModMeta, "C-A-S-M"},
	}

	for _, tt := range tests {
		if got := tt.mod.ShortString(); got != tt.want {
			t.Errorf("Modifier(%d).ShortString() = %q, want %q", tt.mod, got, tt.want)
		}
	}
}

func TestModifierFromName(t *testing.T) {
	tests := []struct {
		name string
		want Modifier
	}{
		{"ctrl", ModCtrl},
		{"control", ModCtrl},
		{"c", ModCtrl},
		{"alt", ModAlt},
		{"a", ModAlt},
		{"option", ModAlt},
		{"shift", ModShift},
		{"s", ModShift},
		{"meta", ModMeta},
		{"m", ModMeta},
		{"cmd", ModMeta},
		{"command", ModMeta},
		{"d", ModMeta},
		{"unknown", ModNone},
		{"", ModNone},
	}

	for _, tt := range tests {
		if got := ModifierFromName(tt.name); got != tt.want {
			t.Errorf("ModifierFromName(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestParseModifiers(t *testing.T) {
	tests := []struct {
		input string
		want  Modifier
	}{
		{"ctrl", ModCtrl},
		{"Ctrl+Alt", ModCtrl | ModAlt},
		{"C-A", ModCtrl | ModAlt},
		{"ctrl+alt+shift", ModCtrl | ModAlt | ModShift},
		{"C-A-S-M", ModCtrl | ModAlt | ModShift | ModMeta},
		{"", ModNone},
	}

	for _, tt := range tests {
		if got := ParseModifiers(tt.input); got != tt.want {
			t.Errorf("ParseModifiers(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
