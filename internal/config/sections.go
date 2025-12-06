package config

// Section accessor methods return snapshot structs. Mutating the returned
// struct does not modify the underlying configuration. Use Config.Set()
// to update configuration values.

// EditorConfig provides type-safe access to editor settings.
type EditorConfig struct {
	// TabSize is the number of spaces a tab is equal to.
	TabSize int

	// InsertSpaces inserts spaces when pressing Tab.
	InsertSpaces bool

	// WordWrap controls how lines should wrap ("off", "on", "wordWrapColumn", "bounded").
	WordWrap string

	// WordWrapColumn is the column at which to wrap lines when WordWrap is "wordWrapColumn".
	WordWrapColumn int

	// LineNumbers controls the display of line numbers ("off", "on", "relative", "interval").
	LineNumbers string

	// CursorStyle controls the cursor style ("block", "line", "underline").
	CursorStyle string

	// CursorBlinking controls the cursor animation style.
	CursorBlinking string

	// ScrollBeyondLastLine allows scrolling beyond the last line.
	ScrollBeyondLastLine bool

	// ScrollOff is the minimum number of lines to keep above/below cursor.
	ScrollOff int

	// AutoIndent controls auto-indentation behavior ("none", "keep", "brackets", "full").
	AutoIndent string

	// TrimAutoWhitespace removes trailing auto-inserted whitespace.
	TrimAutoWhitespace bool

	// DetectIndentation automatically detects indentation settings from file.
	DetectIndentation bool

	// FormatOnSave formats the file when saving.
	FormatOnSave bool
}

// UIConfig provides type-safe access to UI settings.
type UIConfig struct {
	// Theme is the color theme name.
	Theme string

	// FontSize is the font size in pixels.
	FontSize int

	// FontFamily is the font family for the editor.
	FontFamily string

	// LineHeight is the line height multiplier.
	LineHeight float64

	// ShowStatusBar shows the status bar at the bottom.
	ShowStatusBar bool

	// ShowTabBar shows the tab bar at the top.
	ShowTabBar bool

	// ShowMinimap shows the minimap on the side.
	ShowMinimap bool
}

// VimConfig provides type-safe access to Vim mode settings.
type VimConfig struct {
	// Enabled enables Vim mode.
	Enabled bool

	// StartInInsertMode starts in insert mode instead of normal mode.
	StartInInsertMode bool

	// RelativeLineNumbers shows relative line numbers.
	RelativeLineNumbers bool
}

// InputConfig provides type-safe access to input settings.
type InputConfig struct {
	// KeyTimeout is the timeout for multi-key sequences.
	KeyTimeout string

	// LeaderKey is the leader key for custom mappings.
	LeaderKey string

	// DefaultMode is the default input mode when opening files.
	DefaultMode string
}

// FilesConfig provides type-safe access to file settings.
type FilesConfig struct {
	// Encoding is the default file encoding.
	Encoding string

	// EOL is the default end-of-line character ("auto", "lf", "crlf").
	EOL string

	// TrimTrailingWhitespace trims trailing whitespace when saving.
	TrimTrailingWhitespace bool

	// InsertFinalNewline inserts a final newline at end of file when saving.
	InsertFinalNewline bool

	// AutoSave controls auto-save behavior ("off", "afterDelay", "onFocusChange", "onWindowChange").
	AutoSave string

	// AutoSaveDelay is the auto-save delay in milliseconds.
	AutoSaveDelay int

	// Exclude is a list of glob patterns for files to exclude.
	Exclude []string

	// WatcherExclude is a list of glob patterns for files to exclude from watching.
	WatcherExclude []string
}

// SearchConfig provides type-safe access to search settings.
type SearchConfig struct {
	// CaseSensitive enables case-sensitive search.
	CaseSensitive bool

	// WholeWord matches whole words only.
	WholeWord bool

	// Regex enables regex search.
	Regex bool

	// MaxResults is the maximum number of search results.
	MaxResults int
}

// AIConfig provides type-safe access to AI settings.
type AIConfig struct {
	// Enabled enables AI features.
	Enabled bool

	// Provider is the AI provider ("anthropic", "openai", etc.).
	Provider string

	// Model is the AI model to use.
	Model string

	// MaxTokens is the maximum number of tokens for AI responses.
	MaxTokens int

	// Temperature is the AI temperature setting.
	Temperature float64
}

// LoggingConfig provides type-safe access to logging settings.
type LoggingConfig struct {
	// Level is the logging verbosity level ("debug", "info", "warn", "error").
	Level string

	// Format is the log format ("text", "json").
	Format string

	// File is the log file path (empty for no file logging).
	File string
}

// Editor returns type-safe access to editor settings.
func (c *Config) Editor() EditorConfig {
	return EditorConfig{
		TabSize:              c.getIntOr("editor.tabSize", 4),
		InsertSpaces:         c.getBoolOr("editor.insertSpaces", true),
		WordWrap:             c.getStringOr("editor.wordWrap", "off"),
		WordWrapColumn:       c.getIntOr("editor.wordWrapColumn", 80),
		LineNumbers:          c.getStringOr("editor.lineNumbers", "on"),
		CursorStyle:          c.getStringOr("editor.cursorStyle", "block"),
		CursorBlinking:       c.getStringOr("editor.cursorBlinking", "blink"),
		ScrollBeyondLastLine: c.getBoolOr("editor.scrollBeyondLastLine", true),
		ScrollOff:            c.getIntOr("editor.scrollOff", 5),
		AutoIndent:           c.getStringOr("editor.autoIndent", "full"),
		TrimAutoWhitespace:   c.getBoolOr("editor.trimAutoWhitespace", true),
		DetectIndentation:    c.getBoolOr("editor.detectIndentation", true),
		FormatOnSave:         c.getBoolOr("editor.formatOnSave", false),
	}
}

// UI returns type-safe access to UI settings.
func (c *Config) UI() UIConfig {
	return UIConfig{
		Theme:         c.getStringOr("ui.theme", "dark"),
		FontSize:      c.getIntOr("ui.fontSize", 14),
		FontFamily:    c.getStringOr("ui.fontFamily", "monospace"),
		LineHeight:    c.getFloatOr("ui.lineHeight", 1.5),
		ShowStatusBar: c.getBoolOr("ui.showStatusBar", true),
		ShowTabBar:    c.getBoolOr("ui.showTabBar", true),
		ShowMinimap:   c.getBoolOr("ui.showMinimap", true),
	}
}

// Vim returns type-safe access to Vim mode settings.
func (c *Config) Vim() VimConfig {
	return VimConfig{
		Enabled:             c.getBoolOr("vim.enabled", true),
		StartInInsertMode:   c.getBoolOr("vim.startInInsertMode", false),
		RelativeLineNumbers: c.getBoolOr("vim.relativeLineNumbers", false),
	}
}

// Input returns type-safe access to input settings.
func (c *Config) Input() InputConfig {
	return InputConfig{
		KeyTimeout:  c.getStringOr("input.keyTimeout", "500ms"),
		LeaderKey:   c.getStringOr("input.leaderKey", "<Space>"),
		DefaultMode: c.getStringOr("input.defaultMode", "normal"),
	}
}

// Files returns type-safe access to file settings.
func (c *Config) Files() FilesConfig {
	return FilesConfig{
		Encoding:               c.getStringOr("files.encoding", "utf-8"),
		EOL:                    c.getStringOr("files.eol", "lf"),
		TrimTrailingWhitespace: c.getBoolOr("files.trimTrailingWhitespace", false),
		InsertFinalNewline:     c.getBoolOr("files.insertFinalNewline", true),
		AutoSave:               c.getStringOr("files.autoSave", "off"),
		AutoSaveDelay:          c.getIntOr("files.autoSaveDelay", 1000),
		Exclude:                c.getStringSliceOr("files.exclude", []string{".git", "node_modules", ".DS_Store"}),
		WatcherExclude:         c.getStringSliceOr("files.watcherExclude", []string{".git", "node_modules"}),
	}
}

// Search returns type-safe access to search settings.
func (c *Config) Search() SearchConfig {
	return SearchConfig{
		CaseSensitive: c.getBoolOr("search.caseSensitive", false),
		WholeWord:     c.getBoolOr("search.wholeWord", false),
		Regex:         c.getBoolOr("search.regex", false),
		MaxResults:    c.getIntOr("search.maxResults", 1000),
	}
}

// AI returns type-safe access to AI settings.
func (c *Config) AI() AIConfig {
	return AIConfig{
		Enabled:     c.getBoolOr("ai.enabled", true),
		Provider:    c.getStringOr("ai.provider", "anthropic"),
		Model:       c.getStringOr("ai.model", "claude-sonnet-4-20250514"),
		MaxTokens:   c.getIntOr("ai.maxTokens", 4096),
		Temperature: c.getFloatOr("ai.temperature", 0.7),
	}
}

// Logging returns type-safe access to logging settings.
func (c *Config) Logging() LoggingConfig {
	return LoggingConfig{
		Level:  c.getStringOr("logging.level", "info"),
		Format: c.getStringOr("logging.format", "text"),
		File:   c.getStringOr("logging.file", ""),
	}
}

// Helper methods for getting values with defaults.
// These methods only return the default for ErrSettingNotFound.
// Type errors are logged and return the default to avoid breaking callers,
// but indicate a configuration problem that should be fixed.

func (c *Config) getStringOr(path string, defaultValue string) string {
	v, err := c.GetString(path)
	if err != nil {
		if err != ErrSettingNotFound {
			// Record type/parse errors - these indicate config problems
			c.recordConfigError(path, err)
		}
		return defaultValue
	}
	return v
}

func (c *Config) getIntOr(path string, defaultValue int) int {
	v, err := c.GetInt(path)
	if err != nil {
		if err != ErrSettingNotFound {
			c.recordConfigError(path, err)
		}
		return defaultValue
	}
	return v
}

func (c *Config) getBoolOr(path string, defaultValue bool) bool {
	v, err := c.GetBool(path)
	if err != nil {
		if err != ErrSettingNotFound {
			c.recordConfigError(path, err)
		}
		return defaultValue
	}
	return v
}

func (c *Config) getFloatOr(path string, defaultValue float64) float64 {
	v, err := c.GetFloat(path)
	if err != nil {
		if err != ErrSettingNotFound {
			c.recordConfigError(path, err)
		}
		return defaultValue
	}
	return v
}

func (c *Config) getStringSliceOr(path string, defaultValue []string) []string {
	v, err := c.GetStringSlice(path)
	if err != nil {
		if err != ErrSettingNotFound {
			c.recordConfigError(path, err)
		}
		// Return a copy of the default to prevent mutation
		result := make([]string, len(defaultValue))
		copy(result, defaultValue)
		return result
	}
	// Return a copy of the result to enforce snapshot guarantee
	result := make([]string, len(v))
	copy(result, v)
	return result
}

// recordConfigError stores configuration errors for later retrieval.
// Only the first error for each path is recorded to preserve the original cause.
// This helps identify misconfiguration without breaking callers.
func (c *Config) recordConfigError(path string, err error) {
	// Store errors for later retrieval via ConfigErrors()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.configErrors == nil {
		c.configErrors = make(map[string]error)
	}
	// Only store the first error for each path to preserve original cause
	if _, exists := c.configErrors[path]; !exists {
		c.configErrors[path] = err
	}
}

// ConfigErrors returns any configuration errors encountered during access.
// This allows callers to check for misconfigurations after loading.
func (c *Config) ConfigErrors() map[string]error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.configErrors == nil {
		return nil
	}
	// Return a copy to prevent mutation
	result := make(map[string]error, len(c.configErrors))
	for k, v := range c.configErrors {
		result[k] = v
	}
	return result
}

// ClearConfigErrors clears any stored configuration errors.
func (c *Config) ClearConfigErrors() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configErrors = nil
}
