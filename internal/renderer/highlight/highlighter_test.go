package highlight

import (
	"testing"

	"github.com/dshills/keystorm/internal/renderer/core"
)

func TestNewProvider(t *testing.T) {
	t.Run("with nil theme", func(t *testing.T) {
		p := NewProvider(nil, 0)
		if p.theme == nil {
			t.Error("Provider should have default theme when nil passed")
		}
		if p.theme.Name != "Default Dark" {
			t.Errorf("Default theme name = %q, want 'Default Dark'", p.theme.Name)
		}
	})

	t.Run("with custom theme", func(t *testing.T) {
		theme := MonokaiTheme()
		p := NewProvider(theme, 100)
		if p.theme != theme {
			t.Error("Provider should use provided theme")
		}
	})

	t.Run("with zero cache size", func(t *testing.T) {
		p := NewProvider(nil, 0)
		if p.maxCacheSize != 1000 {
			t.Errorf("Default cache size = %d, want 1000", p.maxCacheSize)
		}
	})

	t.Run("with custom cache size", func(t *testing.T) {
		p := NewProvider(nil, 500)
		if p.maxCacheSize != 500 {
			t.Errorf("Cache size = %d, want 500", p.maxCacheSize)
		}
	})
}

func TestProviderSetHighlighter(t *testing.T) {
	p := NewProvider(nil, 100)
	h := GoHighlighter()

	p.SetHighlighter(h)

	if p.highlighter != h {
		t.Error("SetHighlighter should set the highlighter")
	}
}

func TestProviderSetTheme(t *testing.T) {
	p := NewProvider(nil, 100)
	theme := DraculaTheme()

	p.SetTheme(theme)

	if p.Theme() != theme {
		t.Error("SetTheme should update the theme")
	}
}

func TestProviderSetLineGetter(t *testing.T) {
	p := NewProvider(nil, 100)

	lines := []string{
		"package main",
		"",
		"func main() {",
		"}",
	}

	p.SetLineGetter(func(line uint32) string {
		if int(line) < len(lines) {
			return lines[line]
		}
		return ""
	})

	p.SetHighlighter(GoHighlighter())

	// Should now be able to get highlights
	spans := p.HighlightsForLine(0)
	if len(spans) == 0 {
		t.Error("HighlightsForLine should return spans for Go code")
	}
}

func TestProviderHighlightsForLine(t *testing.T) {
	lines := []string{
		"package main",
		"",
		"// comment",
		"func main() {",
		`	fmt.Println("hello")`,
		"}",
	}

	p := NewProvider(nil, 100)
	p.SetHighlighter(GoHighlighter())
	p.SetLineGetter(func(line uint32) string {
		if int(line) < len(lines) {
			return lines[line]
		}
		return ""
	})

	t.Run("package declaration", func(t *testing.T) {
		spans := p.HighlightsForLine(0)
		if len(spans) == 0 {
			t.Error("Should have spans for package declaration")
		}
		// Should have 'package' keyword
		foundKeyword := false
		for _, span := range spans {
			if span.StartCol == 0 && span.EndCol == 7 {
				foundKeyword = true
				break
			}
		}
		if !foundKeyword {
			t.Error("Should highlight 'package' keyword")
		}
	})

	t.Run("empty line", func(t *testing.T) {
		spans := p.HighlightsForLine(1)
		if len(spans) != 0 {
			t.Error("Empty line should have no spans")
		}
	})

	t.Run("comment line", func(t *testing.T) {
		spans := p.HighlightsForLine(2)
		if len(spans) == 0 {
			t.Error("Comment line should have spans")
		}
	})

	t.Run("no highlighter", func(t *testing.T) {
		p2 := NewProvider(nil, 100)
		p2.SetLineGetter(func(line uint32) string { return "test" })
		spans := p2.HighlightsForLine(0)
		if spans != nil {
			t.Error("Should return nil when no highlighter set")
		}
	})

	t.Run("no line getter", func(t *testing.T) {
		p2 := NewProvider(nil, 100)
		p2.SetHighlighter(GoHighlighter())
		spans := p2.HighlightsForLine(0)
		if spans != nil {
			t.Error("Should return nil when no line getter set")
		}
	})
}

func TestProviderInvalidateLines(t *testing.T) {
	lines := []string{
		"func a() {}",
		"func b() {}",
		"func c() {}",
		"func d() {}",
	}

	p := NewProvider(nil, 100)
	p.SetHighlighter(GoHighlighter())
	p.SetLineGetter(func(line uint32) string {
		if int(line) < len(lines) {
			return lines[line]
		}
		return ""
	})

	// Prime the cache
	for i := range lines {
		p.HighlightsForLine(uint32(i))
	}

	// Invalidate lines 1-2
	p.InvalidateLines(1, 2)

	// Cache should be cleared for those lines
	p.mu.RLock()
	if _, ok := p.lineCache[0]; !ok {
		t.Error("Line 0 should still be cached")
	}
	if _, ok := p.lineCache[1]; ok {
		t.Error("Line 1 should be invalidated")
	}
	if _, ok := p.lineCache[2]; ok {
		t.Error("Line 2 should be invalidated")
	}
	if _, ok := p.lineCache[3]; ok {
		t.Error("Line 3 should be invalidated (continuation)")
	}
	p.mu.RUnlock()
}

func TestProviderInvalidateAll(t *testing.T) {
	lines := []string{
		"func a() {}",
		"func b() {}",
	}

	p := NewProvider(nil, 100)
	p.SetHighlighter(GoHighlighter())
	p.SetLineGetter(func(line uint32) string {
		if int(line) < len(lines) {
			return lines[line]
		}
		return ""
	})

	// Prime the cache
	for i := range lines {
		p.HighlightsForLine(uint32(i))
	}

	p.InvalidateAll()

	p.mu.RLock()
	if len(p.lineCache) != 0 {
		t.Error("InvalidateAll should clear all cache")
	}
	p.mu.RUnlock()
}

func TestProviderCaching(t *testing.T) {
	lines := []string{
		"func test() {}",
	}

	p := NewProvider(nil, 100)
	h := GoHighlighter()
	p.SetHighlighter(h)
	p.SetLineGetter(func(line uint32) string {
		if int(line) < len(lines) {
			return lines[line]
		}
		return ""
	})

	// First call should compute and cache
	spans1 := p.HighlightsForLine(0)

	// Second call should return same result from cache
	spans2 := p.HighlightsForLine(0)

	// Results should be equivalent
	if len(spans1) != len(spans2) {
		t.Errorf("Cached result differs: got %d spans, want %d", len(spans2), len(spans1))
	}

	// Verify cache is populated
	p.mu.RLock()
	if _, ok := p.lineCache[0]; !ok {
		t.Error("Cache should be populated after first call")
	}
	p.mu.RUnlock()
}

func TestProviderStyleSpans(t *testing.T) {
	lines := []string{
		`"hello world"`,
	}

	p := NewProvider(nil, 100)
	p.SetHighlighter(GoHighlighter())
	p.SetLineGetter(func(line uint32) string {
		if int(line) < len(lines) {
			return lines[line]
		}
		return ""
	})

	spans := p.HighlightsForLine(0)
	if len(spans) == 0 {
		t.Fatal("Should have spans for string literal")
	}

	// Verify span structure
	for _, span := range spans {
		if span.EndCol <= span.StartCol {
			t.Error("Span EndCol should be greater than StartCol")
		}
		if span.Style.Foreground == core.ColorDefault {
			t.Error("Span should have styled foreground")
		}
	}
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()

	if r.byLanguage == nil {
		t.Error("Registry should have initialized byLanguage map")
	}
	if r.byExtension == nil {
		t.Error("Registry should have initialized byExtension map")
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	h := GoHighlighter()

	r.Register(h)

	// Check by language
	got, ok := r.GetByLanguage("go")
	if !ok {
		t.Error("Should find highlighter by language")
	}
	if got != h {
		t.Error("Should return the registered highlighter")
	}

	// Check by extension
	got, ok = r.GetByExtension(".go")
	if !ok {
		t.Error("Should find highlighter by extension")
	}
	if got != h {
		t.Error("Should return the registered highlighter")
	}

	// Without leading dot
	_, ok = r.GetByExtension("go")
	if !ok {
		t.Error("Should find highlighter by extension without dot")
	}
}

func TestRegistryGetByLanguage(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinHighlighters(r)

	tests := []struct {
		language string
		found    bool
	}{
		{"go", true},
		{"python", true},
		{"javascript", true},
		{"rust", true},
		{"markdown", true},
		{"cobol", false},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			_, ok := r.GetByLanguage(tt.language)
			if ok != tt.found {
				t.Errorf("GetByLanguage(%q) found = %v, want %v", tt.language, ok, tt.found)
			}
		})
	}
}

func TestRegistryGetByExtension(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinHighlighters(r)

	tests := []struct {
		ext   string
		lang  string
		found bool
	}{
		{".go", "go", true},
		{".py", "python", true},
		{".js", "javascript", true},
		{".ts", "javascript", true},
		{".tsx", "javascript", true},
		{".rs", "rust", true},
		{".md", "markdown", true},
		{".cbl", "", false},
		{"", "", false}, // Empty extension should return false
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			h, ok := r.GetByExtension(tt.ext)
			if ok != tt.found {
				t.Errorf("GetByExtension(%q) found = %v, want %v", tt.ext, ok, tt.found)
			}
			if ok && h.Language() != tt.lang {
				t.Errorf("GetByExtension(%q) language = %q, want %q", tt.ext, h.Language(), tt.lang)
			}
		})
	}
}

func TestRegistryLanguages(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinHighlighters(r)

	langs := r.Languages()
	if len(langs) != 5 {
		t.Errorf("Expected 5 languages, got %d", len(langs))
	}

	expected := map[string]bool{
		"go":         true,
		"python":     true,
		"javascript": true,
		"rust":       true,
		"markdown":   true,
	}

	for _, lang := range langs {
		if !expected[lang] {
			t.Errorf("Unexpected language: %q", lang)
		}
	}
}

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()

	if r == nil {
		t.Error("DefaultRegistry should not return nil")
	}

	// Default registry is empty (highlighters added separately)
	langs := r.Languages()
	if len(langs) != 0 {
		t.Error("DefaultRegistry should start empty")
	}
}
