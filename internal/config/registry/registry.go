package registry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry maintains all known settings definitions and provides
// type-safe access to setting values.
type Registry struct {
	mu       sync.RWMutex
	settings map[string]*Setting
	sections map[string][]*Setting // Settings grouped by section
}

// New creates a new settings registry.
func New() *Registry {
	return &Registry{
		settings: make(map[string]*Setting),
		sections: make(map[string][]*Setting),
	}
}

// NewWithDefaults creates a registry with built-in default settings.
func NewWithDefaults() *Registry {
	r := New()
	r.RegisterDefaults()
	return r
}

// Register adds a setting definition to the registry.
// Returns an error if a setting with the same path already exists.
func (r *Registry) Register(setting Setting) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.settings[setting.Path]; exists {
		return fmt.Errorf("%w: %s", ErrSettingAlreadyRegistered, setting.Path)
	}

	s := &setting // Copy to heap
	r.settings[setting.Path] = s

	// Add to section index
	section := extractSection(setting.Path)
	r.sections[section] = append(r.sections[section], s)

	return nil
}

// MustRegister registers a setting and panics on error.
// Useful for registering built-in settings at init time.
func (r *Registry) MustRegister(setting Setting) {
	if err := r.Register(setting); err != nil {
		panic(err)
	}
}

// Get returns the setting definition for the given path.
// Returns nil if the setting is not registered.
func (r *Registry) Get(path string) *Setting {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.settings[path]
}

// Has checks if a setting is registered.
func (r *Registry) Has(path string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.settings[path]
	return exists
}

// All returns all registered settings sorted by path.
func (r *Registry) All() []*Setting {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Setting, 0, len(r.settings))
	for _, s := range r.settings {
		result = append(result, s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})

	return result
}

// Section returns all settings in a given section (e.g., "editor").
func (r *Registry) Section(name string) []*Setting {
	r.mu.RLock()
	defer r.mu.RUnlock()

	settings := r.sections[name]
	result := make([]*Setting, len(settings))
	copy(result, settings)
	return result
}

// Sections returns all section names.
func (r *Registry) Sections() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.sections))
	for section := range r.sections {
		result = append(result, section)
	}
	sort.Strings(result)
	return result
}

// Search finds settings matching a query string.
// Searches path, description, and tags.
func (r *Registry) Search(query string) []*Setting {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	var result []*Setting

	for _, s := range r.settings {
		if matchesSetting(s, query) {
			result = append(result, s)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})

	return result
}

// ByTag returns all settings with the given tag.
func (r *Registry) ByTag(tag string) []*Setting {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Setting
	for _, s := range r.settings {
		for _, t := range s.Tags {
			if t == tag {
				result = append(result, s)
				break
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})

	return result
}

// Deprecated returns all deprecated settings.
func (r *Registry) Deprecated() []*Setting {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Setting
	for _, s := range r.settings {
		if s.Deprecated {
			result = append(result, s)
		}
	}
	return result
}

// Default returns the default value for a setting.
// Returns nil if the setting is not registered.
func (r *Registry) Default(path string) any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if s, ok := r.settings[path]; ok {
		return s.Default
	}
	return nil
}

// Defaults returns a map of all default values.
func (r *Registry) Defaults() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]any, len(r.settings))
	for path, s := range r.settings {
		if s.Default != nil {
			result[path] = s.Default
		}
	}
	return result
}

// Validate checks if a value is valid for a setting.
func (r *Registry) Validate(path string, value any) error {
	r.mu.RLock()
	s, ok := r.settings[path]
	r.mu.RUnlock()

	if !ok {
		// Unknown setting - could be a plugin setting
		// We allow unknown settings but log a warning
		return nil
	}

	return s.Validate(value)
}

// extractSection extracts the top-level section from a path.
func extractSection(path string) string {
	parts := strings.SplitN(path, ".", 2)
	return parts[0]
}

// matchesSetting checks if a setting matches a search query.
func matchesSetting(s *Setting, query string) bool {
	// Match path
	if strings.Contains(strings.ToLower(s.Path), query) {
		return true
	}

	// Match description
	if strings.Contains(strings.ToLower(s.Description), query) {
		return true
	}

	// Match tags
	for _, tag := range s.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}

	return false
}

// ErrSettingAlreadyRegistered is returned when attempting to register a duplicate setting.
var ErrSettingAlreadyRegistered = fmt.Errorf("setting already registered")

// RegisterDefaults registers all built-in Keystorm settings.
func (r *Registry) RegisterDefaults() {
	// Editor settings
	r.MustRegister(Setting{
		Path:        "editor.tabSize",
		Type:        TypeInt,
		Default:     4,
		Description: "The number of spaces a tab is equal to",
		Scope:       ScopeAll,
		Minimum:     MinValue(1),
		Maximum:     MaxValue(16),
		Tags:        []string{"editor", "formatting"},
	})

	r.MustRegister(Setting{
		Path:        "editor.insertSpaces",
		Type:        TypeBool,
		Default:     true,
		Description: "Insert spaces when pressing Tab",
		Scope:       ScopeAll,
		Tags:        []string{"editor", "formatting"},
	})

	r.MustRegister(Setting{
		Path:        "editor.wordWrap",
		Type:        TypeEnum,
		Default:     "off",
		Description: "Controls how lines should wrap",
		Scope:       ScopeAll,
		Enum:        []any{"off", "on", "wordWrapColumn", "bounded"},
		Tags:        []string{"editor", "display"},
	})

	r.MustRegister(Setting{
		Path:        "editor.wordWrapColumn",
		Type:        TypeInt,
		Default:     80,
		Description: "Column at which to wrap lines when wordWrap is 'wordWrapColumn'",
		Scope:       ScopeAll,
		Minimum:     MinValue(1),
		Maximum:     MaxValue(500),
		Tags:        []string{"editor", "display"},
	})

	r.MustRegister(Setting{
		Path:        "editor.lineNumbers",
		Type:        TypeEnum,
		Default:     "on",
		Description: "Controls the display of line numbers",
		Scope:       ScopeAll,
		Enum:        []any{"off", "on", "relative", "interval"},
		Tags:        []string{"editor", "display"},
	})

	r.MustRegister(Setting{
		Path:        "editor.cursorStyle",
		Type:        TypeEnum,
		Default:     "block",
		Description: "Controls the cursor style",
		Scope:       ScopeAll,
		Enum:        []any{"block", "line", "underline"},
		Tags:        []string{"editor", "display"},
	})

	r.MustRegister(Setting{
		Path:        "editor.cursorBlinking",
		Type:        TypeEnum,
		Default:     "blink",
		Description: "Controls the cursor animation style",
		Scope:       ScopeAll,
		Enum:        []any{"blink", "smooth", "phase", "expand", "solid"},
		Tags:        []string{"editor", "display"},
	})

	r.MustRegister(Setting{
		Path:        "editor.scrollBeyondLastLine",
		Type:        TypeBool,
		Default:     true,
		Description: "Allows scrolling beyond the last line",
		Scope:       ScopeAll,
		Tags:        []string{"editor", "scrolling"},
	})

	r.MustRegister(Setting{
		Path:        "editor.scrollOff",
		Type:        TypeInt,
		Default:     5,
		Description: "Minimum number of lines to keep above/below cursor",
		Scope:       ScopeAll,
		Minimum:     MinValue(0),
		Maximum:     MaxValue(100),
		Tags:        []string{"editor", "scrolling"},
	})

	r.MustRegister(Setting{
		Path:        "editor.autoIndent",
		Type:        TypeEnum,
		Default:     "full",
		Description: "Controls auto-indentation behavior",
		Scope:       ScopeAll,
		Enum:        []any{"none", "keep", "brackets", "full"},
		Tags:        []string{"editor", "formatting"},
	})

	r.MustRegister(Setting{
		Path:        "editor.trimAutoWhitespace",
		Type:        TypeBool,
		Default:     true,
		Description: "Remove trailing auto-inserted whitespace",
		Scope:       ScopeAll,
		Tags:        []string{"editor", "formatting"},
	})

	r.MustRegister(Setting{
		Path:        "editor.detectIndentation",
		Type:        TypeBool,
		Default:     true,
		Description: "Automatically detect indentation settings from file",
		Scope:       ScopeAll,
		Tags:        []string{"editor", "formatting"},
	})

	// Input settings
	r.MustRegister(Setting{
		Path:        "input.keyTimeout",
		Type:        TypeDuration,
		Default:     "500ms",
		Description: "Timeout for multi-key sequences",
		Scope:       ScopeGlobal,
		Tags:        []string{"input", "vim"},
	})

	r.MustRegister(Setting{
		Path:        "input.leaderKey",
		Type:        TypeString,
		Default:     "<Space>",
		Description: "The leader key for custom mappings",
		Scope:       ScopeGlobal,
		Tags:        []string{"input", "vim"},
	})

	r.MustRegister(Setting{
		Path:        "input.defaultMode",
		Type:        TypeEnum,
		Default:     "normal",
		Description: "The default input mode when opening files",
		Scope:       ScopeGlobal,
		Enum:        []any{"normal", "insert"},
		Tags:        []string{"input", "vim"},
	})

	// UI settings
	r.MustRegister(Setting{
		Path:        "ui.theme",
		Type:        TypeString,
		Default:     "dark",
		Description: "Color theme name",
		Scope:       ScopeGlobal,
		Tags:        []string{"ui", "theme"},
	})

	r.MustRegister(Setting{
		Path:        "ui.fontSize",
		Type:        TypeInt,
		Default:     14,
		Description: "Font size in pixels",
		Scope:       ScopeGlobal,
		Minimum:     MinValue(6),
		Maximum:     MaxValue(72),
		Tags:        []string{"ui", "font"},
	})

	r.MustRegister(Setting{
		Path:        "ui.fontFamily",
		Type:        TypeString,
		Default:     "monospace",
		Description: "Font family for the editor",
		Scope:       ScopeGlobal,
		Tags:        []string{"ui", "font"},
	})

	r.MustRegister(Setting{
		Path:        "ui.lineHeight",
		Type:        TypeFloat,
		Default:     1.5,
		Description: "Line height multiplier",
		Scope:       ScopeGlobal,
		Minimum:     MinValue(1.0),
		Maximum:     MaxValue(3.0),
		Tags:        []string{"ui", "font"},
	})

	r.MustRegister(Setting{
		Path:        "ui.showStatusBar",
		Type:        TypeBool,
		Default:     true,
		Description: "Show the status bar at the bottom",
		Scope:       ScopeGlobal,
		Tags:        []string{"ui", "statusbar"},
	})

	r.MustRegister(Setting{
		Path:        "ui.showTabBar",
		Type:        TypeBool,
		Default:     true,
		Description: "Show the tab bar at the top",
		Scope:       ScopeGlobal,
		Tags:        []string{"ui", "tabs"},
	})

	// Files settings
	r.MustRegister(Setting{
		Path:        "files.encoding",
		Type:        TypeString,
		Default:     "utf-8",
		Description: "Default file encoding",
		Scope:       ScopeAll,
		Tags:        []string{"files"},
	})

	r.MustRegister(Setting{
		Path:        "files.eol",
		Type:        TypeEnum,
		Default:     "auto",
		Description: "Default end-of-line character",
		Scope:       ScopeAll,
		Enum:        []any{"auto", "lf", "crlf"},
		Tags:        []string{"files"},
	})

	r.MustRegister(Setting{
		Path:        "files.trimTrailingWhitespace",
		Type:        TypeBool,
		Default:     false,
		Description: "Trim trailing whitespace when saving",
		Scope:       ScopeAll,
		Tags:        []string{"files", "formatting"},
	})

	r.MustRegister(Setting{
		Path:        "files.insertFinalNewline",
		Type:        TypeBool,
		Default:     true,
		Description: "Insert a final newline at end of file when saving",
		Scope:       ScopeAll,
		Tags:        []string{"files", "formatting"},
	})

	r.MustRegister(Setting{
		Path:        "files.autoSave",
		Type:        TypeEnum,
		Default:     "off",
		Description: "Controls auto-save behavior",
		Scope:       ScopeGlobal,
		Enum:        []any{"off", "afterDelay", "onFocusChange", "onWindowChange"},
		Tags:        []string{"files"},
	})

	r.MustRegister(Setting{
		Path:        "files.autoSaveDelay",
		Type:        TypeInt,
		Default:     1000,
		Description: "Auto-save delay in milliseconds",
		Scope:       ScopeGlobal,
		Minimum:     MinValue(100),
		Maximum:     MaxValue(60000),
		Tags:        []string{"files"},
	})

	// Logging settings
	r.MustRegister(Setting{
		Path:        "logging.level",
		Type:        TypeEnum,
		Default:     "info",
		Description: "Logging verbosity level",
		Scope:       ScopeGlobal,
		Enum:        []any{"debug", "info", "warn", "error"},
		Tags:        []string{"logging"},
	})

	r.MustRegister(Setting{
		Path:        "logging.file",
		Type:        TypeString,
		Default:     "",
		Description: "Log file path (empty for no file logging)",
		Scope:       ScopeGlobal,
		Tags:        []string{"logging"},
	})
}
