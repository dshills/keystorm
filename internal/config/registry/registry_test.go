package registry

import (
	"testing"
)

func TestRegistry_Register(t *testing.T) {
	r := New()

	err := r.Register(Setting{
		Path:        "editor.tabSize",
		Type:        TypeInt,
		Default:     4,
		Description: "Tab size",
		Scope:       ScopeAll,
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Duplicate should fail
	err = r.Register(Setting{
		Path: "editor.tabSize",
		Type: TypeInt,
	})
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_MustRegister_Panics(t *testing.T) {
	r := New()

	// First registration should succeed
	r.MustRegister(Setting{
		Path: "test.setting",
		Type: TypeString,
	})

	// Second registration should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate MustRegister")
		}
	}()

	r.MustRegister(Setting{
		Path: "test.setting",
		Type: TypeString,
	})
}

func TestRegistry_Get(t *testing.T) {
	r := New()
	r.MustRegister(Setting{
		Path:    "test.setting",
		Type:    TypeString,
		Default: "default",
	})

	// Existing setting
	s := r.Get("test.setting")
	if s == nil {
		t.Fatal("expected to find setting")
	}
	if s.Default != "default" {
		t.Errorf("Default = %v, want 'default'", s.Default)
	}

	// Non-existing setting
	s = r.Get("nonexistent")
	if s != nil {
		t.Error("expected nil for non-existing setting")
	}
}

func TestRegistry_Has(t *testing.T) {
	r := New()
	r.MustRegister(Setting{
		Path: "test.setting",
		Type: TypeString,
	})

	if !r.Has("test.setting") {
		t.Error("expected Has to return true for existing setting")
	}

	if r.Has("nonexistent") {
		t.Error("expected Has to return false for non-existing setting")
	}
}

func TestRegistry_All(t *testing.T) {
	r := New()
	r.MustRegister(Setting{Path: "b.setting", Type: TypeString})
	r.MustRegister(Setting{Path: "a.setting", Type: TypeString})
	r.MustRegister(Setting{Path: "c.setting", Type: TypeString})

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 settings, got %d", len(all))
	}

	// Should be sorted by path
	if all[0].Path != "a.setting" {
		t.Error("expected sorted by path")
	}
}

func TestRegistry_Section(t *testing.T) {
	r := New()
	r.MustRegister(Setting{Path: "editor.tabSize", Type: TypeInt})
	r.MustRegister(Setting{Path: "editor.insertSpaces", Type: TypeBool})
	r.MustRegister(Setting{Path: "ui.theme", Type: TypeString})

	editor := r.Section("editor")
	if len(editor) != 2 {
		t.Errorf("expected 2 editor settings, got %d", len(editor))
	}

	ui := r.Section("ui")
	if len(ui) != 1 {
		t.Errorf("expected 1 ui setting, got %d", len(ui))
	}

	empty := r.Section("nonexistent")
	if len(empty) != 0 {
		t.Errorf("expected 0 settings for nonexistent section, got %d", len(empty))
	}
}

func TestRegistry_Sections(t *testing.T) {
	r := New()
	r.MustRegister(Setting{Path: "editor.tabSize", Type: TypeInt})
	r.MustRegister(Setting{Path: "ui.theme", Type: TypeString})
	r.MustRegister(Setting{Path: "files.encoding", Type: TypeString})

	sections := r.Sections()
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}

	// Should be sorted
	expected := []string{"editor", "files", "ui"}
	for i, s := range expected {
		if sections[i] != s {
			t.Errorf("sections[%d] = %s, want %s", i, sections[i], s)
		}
	}
}

func TestRegistry_Search(t *testing.T) {
	r := New()
	r.MustRegister(Setting{
		Path:        "editor.tabSize",
		Type:        TypeInt,
		Description: "Number of spaces for a tab",
		Tags:        []string{"formatting"},
	})
	r.MustRegister(Setting{
		Path:        "editor.insertSpaces",
		Type:        TypeBool,
		Description: "Use spaces instead of tabs",
		Tags:        []string{"formatting"},
	})
	r.MustRegister(Setting{
		Path:        "ui.theme",
		Type:        TypeString,
		Description: "Color theme",
	})

	// Search by path
	results := r.Search("tab")
	if len(results) != 2 {
		t.Errorf("search 'tab': expected 2, got %d", len(results))
	}

	// Search by description
	results = r.Search("spaces")
	if len(results) != 2 {
		t.Errorf("search 'spaces': expected 2, got %d", len(results))
	}

	// Search by tag
	results = r.Search("formatting")
	if len(results) != 2 {
		t.Errorf("search 'formatting': expected 2, got %d", len(results))
	}

	// Search no match
	results = r.Search("nonexistent")
	if len(results) != 0 {
		t.Errorf("search 'nonexistent': expected 0, got %d", len(results))
	}
}

func TestRegistry_ByTag(t *testing.T) {
	r := New()
	r.MustRegister(Setting{
		Path: "editor.tabSize",
		Tags: []string{"formatting", "editor"},
	})
	r.MustRegister(Setting{
		Path: "editor.insertSpaces",
		Tags: []string{"formatting", "editor"},
	})
	r.MustRegister(Setting{
		Path: "ui.theme",
		Tags: []string{"appearance"},
	})

	formatting := r.ByTag("formatting")
	if len(formatting) != 2 {
		t.Errorf("expected 2 formatting settings, got %d", len(formatting))
	}

	appearance := r.ByTag("appearance")
	if len(appearance) != 1 {
		t.Errorf("expected 1 appearance setting, got %d", len(appearance))
	}
}

func TestRegistry_Deprecated(t *testing.T) {
	r := New()
	r.MustRegister(Setting{
		Path:       "old.setting",
		Deprecated: true,
	})
	r.MustRegister(Setting{
		Path:       "new.setting",
		Deprecated: false,
	})

	deprecated := r.Deprecated()
	if len(deprecated) != 1 {
		t.Errorf("expected 1 deprecated setting, got %d", len(deprecated))
	}
	if deprecated[0].Path != "old.setting" {
		t.Errorf("expected old.setting, got %s", deprecated[0].Path)
	}
}

func TestRegistry_Default(t *testing.T) {
	r := New()
	r.MustRegister(Setting{
		Path:    "test.setting",
		Default: "default_value",
	})

	d := r.Default("test.setting")
	if d != "default_value" {
		t.Errorf("Default = %v, want 'default_value'", d)
	}

	d = r.Default("nonexistent")
	if d != nil {
		t.Errorf("Default for nonexistent = %v, want nil", d)
	}
}

func TestRegistry_Defaults(t *testing.T) {
	r := New()
	r.MustRegister(Setting{Path: "a", Default: 1})
	r.MustRegister(Setting{Path: "b", Default: "two"})
	r.MustRegister(Setting{Path: "c", Default: nil}) // No default

	defaults := r.Defaults()
	if len(defaults) != 2 {
		t.Errorf("expected 2 defaults, got %d", len(defaults))
	}
	if defaults["a"] != 1 {
		t.Errorf("defaults[a] = %v, want 1", defaults["a"])
	}
	if defaults["b"] != "two" {
		t.Errorf("defaults[b] = %v, want 'two'", defaults["b"])
	}
}

func TestRegistry_Validate(t *testing.T) {
	r := New()
	r.MustRegister(Setting{
		Path:    "test.int",
		Type:    TypeInt,
		Minimum: MinValue(1),
		Maximum: MaxValue(10),
	})

	// Valid
	if err := r.Validate("test.int", 5); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	// Invalid (out of range)
	if err := r.Validate("test.int", 100); err == nil {
		t.Error("expected error for out-of-range value")
	}

	// Unknown setting (allowed by default)
	if err := r.Validate("unknown.setting", "anything"); err != nil {
		t.Errorf("expected unknown settings to be allowed, got error: %v", err)
	}
}

func TestNewWithDefaults(t *testing.T) {
	r := NewWithDefaults()

	// Should have built-in settings
	if !r.Has("editor.tabSize") {
		t.Error("expected editor.tabSize to be registered")
	}
	if !r.Has("ui.theme") {
		t.Error("expected ui.theme to be registered")
	}

	// Check a default value
	tabSize := r.Default("editor.tabSize")
	if tabSize != 4 {
		t.Errorf("default tabSize = %v, want 4", tabSize)
	}
}
