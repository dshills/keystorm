package highlight

import (
	"testing"

	"github.com/dshills/keystorm/internal/renderer/core"
)

func TestDefaultTheme(t *testing.T) {
	theme := DefaultTheme()

	if theme.Name != "Default Dark" {
		t.Errorf("DefaultTheme().Name = %q, want %q", theme.Name, "Default Dark")
	}

	// Verify colors are set
	if theme.Background == core.ColorDefault {
		t.Error("DefaultTheme().Background should not be default")
	}
	if theme.Foreground == core.ColorDefault {
		t.Error("DefaultTheme().Foreground should not be default")
	}

	// Verify token styles exist
	if len(theme.TokenStyles) == 0 {
		t.Error("DefaultTheme() should have token styles")
	}

	// Check a few key token styles exist
	tokensToCheck := []TokenType{
		TokenComment,
		TokenString,
		TokenKeyword,
		TokenFunction,
		TokenTypeName,
	}

	for _, tt := range tokensToCheck {
		if _, ok := theme.TokenStyles[tt]; !ok {
			t.Errorf("DefaultTheme() missing style for %v", tt)
		}
	}
}

func TestThemeStyleForToken(t *testing.T) {
	theme := DefaultTheme()

	// Existing token type
	style := theme.StyleForToken(TokenComment)
	if style.Foreground == core.ColorDefault {
		t.Error("StyleForToken(TokenComment) should return a styled foreground")
	}

	// Non-existing token type should fall back to foreground
	style = theme.StyleForToken(TokenEditorWhitespace)
	if style.Foreground != theme.Foreground {
		t.Error("StyleForToken for missing token should return theme foreground")
	}
}

func TestThemeStyleForScope(t *testing.T) {
	theme := DefaultTheme()

	// Direct scope match
	style := theme.StyleForScope("comment")
	if style.Foreground == core.ColorDefault {
		t.Error("StyleForScope('comment') should return styled foreground")
	}

	// Hierarchical scope should match parent
	style = theme.StyleForScope("comment.line")
	if style.Foreground == core.ColorDefault {
		t.Error("StyleForScope('comment.line') should return styled foreground")
	}

	// Non-matching scope falls back to foreground
	style = theme.StyleForScope("nonexistent.scope.here")
	if style.Foreground != theme.Foreground {
		t.Error("StyleForScope for unknown scope should return theme foreground")
	}
}

func TestBuiltInThemes(t *testing.T) {
	themes := []*Theme{
		DefaultTheme(),
		MonokaiTheme(),
		DraculaTheme(),
		SolarizedDarkTheme(),
		LightTheme(),
	}

	for _, theme := range themes {
		t.Run(theme.Name, func(t *testing.T) {
			if theme.Name == "" {
				t.Error("Theme name should not be empty")
			}
			// Light theme might use default colors, others should have explicit background
			if theme.Background == core.ColorDefault && theme.Name != "Light" {
				t.Logf("Theme %q uses default background color", theme.Name)
			}
			if len(theme.TokenStyles) == 0 {
				t.Error("Theme should have token styles")
			}
		})
	}
}

func TestThemeRegistry(t *testing.T) {
	registry := NewThemeRegistry()

	t.Run("built-in themes registered", func(t *testing.T) {
		names := registry.Names()
		if len(names) < 5 {
			t.Errorf("Expected at least 5 built-in themes, got %d", len(names))
		}

		expectedThemes := []string{
			"Default Dark",
			"Monokai",
			"Dracula",
			"Solarized Dark",
			"Light",
		}

		for _, name := range expectedThemes {
			theme, ok := registry.Get(name)
			if !ok {
				t.Errorf("Expected theme %q to be registered", name)
			}
			if theme.Name != name {
				t.Errorf("Theme.Name = %q, want %q", theme.Name, name)
			}
		}
	})

	t.Run("current theme", func(t *testing.T) {
		current := registry.Current()
		if current == nil {
			t.Fatal("Current() should not return nil")
		}
		if current.Name != "Default Dark" {
			t.Errorf("Default current theme should be 'Default Dark', got %q", current.Name)
		}
	})

	t.Run("set current", func(t *testing.T) {
		ok := registry.SetCurrent("Monokai")
		if !ok {
			t.Error("SetCurrent('Monokai') should succeed")
		}
		if registry.Current().Name != "Monokai" {
			t.Error("Current theme should be Monokai after SetCurrent")
		}

		ok = registry.SetCurrent("NonExistent")
		if ok {
			t.Error("SetCurrent('NonExistent') should fail")
		}
		if registry.Current().Name != "Monokai" {
			t.Error("Current should remain Monokai after failed SetCurrent")
		}
	})

	t.Run("register custom theme", func(t *testing.T) {
		custom := &Theme{
			Name:        "Custom",
			Background:  core.ColorFromRGB(0, 0, 0),
			Foreground:  core.ColorFromRGB(255, 255, 255),
			TokenStyles: make(map[TokenType]core.Style),
		}

		registry.Register(custom)

		got, ok := registry.Get("Custom")
		if !ok {
			t.Error("Custom theme should be retrievable after registration")
		}
		if got.Name != "Custom" {
			t.Errorf("Got theme name %q, want 'Custom'", got.Name)
		}
	})
}

func TestThemeColors(t *testing.T) {
	// Verify that theme colors are distinguishable
	theme := MonokaiTheme()

	// Comments should be different from keywords
	commentStyle := theme.StyleForToken(TokenComment)
	keywordStyle := theme.StyleForToken(TokenKeyword)

	if commentStyle.Foreground == keywordStyle.Foreground {
		t.Error("Comment and keyword colors should be different")
	}

	// Strings should be different from functions
	stringStyle := theme.StyleForToken(TokenString)
	functionStyle := theme.StyleForToken(TokenFunction)

	if stringStyle.Foreground == functionStyle.Foreground {
		t.Error("String and function colors should be different")
	}
}

func TestThemeStyleAttributes(t *testing.T) {
	theme := DefaultTheme()

	// Comments should be italic
	commentStyle := theme.StyleForToken(TokenComment)
	if !commentStyle.Attributes.Has(core.AttrItalic) {
		t.Error("Comment style should be italic")
	}

	// Headings should be bold in markdown
	headingStyle := theme.StyleForToken(TokenMarkupHeading)
	if !headingStyle.Attributes.Has(core.AttrBold) {
		t.Error("Heading style should be bold")
	}
}
