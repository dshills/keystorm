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

	// MaxSize is the maximum log file size in MB.
	MaxSize int

	// MaxBackups is the maximum number of log backups.
	MaxBackups int
}

// TerminalConfig provides type-safe access to integrated terminal settings.
type TerminalConfig struct {
	// Shell is the shell executable path.
	Shell string

	// FontSize is the terminal font size.
	FontSize int

	// FontFamily is the terminal font family.
	FontFamily string

	// CursorStyle is the terminal cursor style ("block", "line", "underline").
	CursorStyle string

	// Scrollback is the number of scrollback lines.
	Scrollback int
}

// LSPConfig provides type-safe access to Language Server Protocol settings.
type LSPConfig struct {
	// Enabled enables LSP features.
	Enabled bool

	// DiagnosticsDelay is the delay before showing diagnostics in milliseconds.
	DiagnosticsDelay int

	// CompletionTriggerCharacters are characters that trigger completion.
	CompletionTriggerCharacters []string

	// SignatureHelpTriggerCharacters are characters that trigger signature help.
	SignatureHelpTriggerCharacters []string
}

// PathsConfig provides type-safe access to path settings.
type PathsConfig struct {
	// ConfigDir is the configuration directory path.
	ConfigDir string

	// DataDir is the data directory path.
	DataDir string

	// CacheDir is the cache directory path.
	CacheDir string

	// PluginDir is the plugin directory path.
	PluginDir string
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
		Level:      c.getStringOr("logging.level", "info"),
		Format:     c.getStringOr("logging.format", "text"),
		File:       c.getStringOr("logging.file", ""),
		MaxSize:    c.getIntOr("logging.maxSize", 10),
		MaxBackups: c.getIntOr("logging.maxBackups", 5),
	}
}

// Terminal returns type-safe access to integrated terminal settings.
func (c *Config) Terminal() TerminalConfig {
	return TerminalConfig{
		Shell:       c.getStringOr("terminal.shell", ""),
		FontSize:    c.getIntOr("terminal.fontSize", 14),
		FontFamily:  c.getStringOr("terminal.fontFamily", "monospace"),
		CursorStyle: c.getStringOr("terminal.cursorStyle", "block"),
		Scrollback:  c.getIntOr("terminal.scrollback", 10000),
	}
}

// LSP returns type-safe access to Language Server Protocol settings.
func (c *Config) LSP() LSPConfig {
	return LSPConfig{
		Enabled:                        c.getBoolOr("lsp.enabled", true),
		DiagnosticsDelay:               c.getIntOr("lsp.diagnosticsDelay", 500),
		CompletionTriggerCharacters:    c.getStringSliceOr("lsp.completionTriggerCharacters", []string{".", ":", "<"}),
		SignatureHelpTriggerCharacters: c.getStringSliceOr("lsp.signatureHelpTriggerCharacters", []string{"(", ","}),
	}
}

// Paths returns type-safe access to path settings.
func (c *Config) Paths() PathsConfig {
	return PathsConfig{
		ConfigDir: c.getStringOr("paths.configDir", ""),
		DataDir:   c.getStringOr("paths.dataDir", ""),
		CacheDir:  c.getStringOr("paths.cacheDir", ""),
		PluginDir: c.getStringOr("paths.pluginDir", ""),
	}
}

// IntegrationSettings provides type-safe access to integration layer settings.
type IntegrationSettings struct {
	// Enabled controls whether the integration layer is active.
	Enabled bool

	// WorkspaceRoot is the root directory for the workspace.
	WorkspaceRoot string

	// MaxProcesses limits concurrent managed processes.
	MaxProcesses int

	// ShutdownTimeoutSeconds is how long to wait for graceful process shutdown.
	ShutdownTimeoutSeconds int

	// Git provides git integration settings.
	Git GitSettings

	// Debug provides debugger integration settings.
	Debug DebugSettings

	// Task provides task runner settings.
	Task TaskSettings

	// TerminalSettings provides terminal settings.
	Terminal TerminalSettings
}

// GitSettings provides settings for git integration.
type GitSettings struct {
	// Enabled controls whether git integration is active.
	Enabled bool

	// AutoFetch enables periodic fetching from remotes.
	AutoFetch bool

	// AutoFetchInterval is the interval in seconds between auto-fetches.
	AutoFetchInterval int

	// ShowInlineBlame shows blame annotations inline.
	ShowInlineBlame bool

	// ConfirmCommit requires confirmation before committing.
	ConfirmCommit bool

	// SignCommits enables GPG signing of commits.
	SignCommits bool

	// DefaultRemote is the default remote for push/pull operations.
	DefaultRemote string
}

// DebugSettings provides settings for debugger integration.
type DebugSettings struct {
	// Enabled controls whether debug integration is active.
	Enabled bool

	// DefaultAdapter is the default debug adapter to use.
	DefaultAdapter string

	// AutoAttachBreakpoints automatically sets breakpoints from saved state.
	AutoAttachBreakpoints bool

	// ShowInlineValues shows variable values inline during debugging.
	ShowInlineValues bool

	// StopOnEntry stops at entry point when starting debug session.
	StopOnEntry bool

	// Timeout is the debug session timeout in seconds.
	Timeout int

	// Adapters provides per-adapter configurations.
	Adapters DebugAdaptersSettings
}

// DebugAdaptersSettings provides per-adapter debug settings.
type DebugAdaptersSettings struct {
	// Delve provides Go debugger (Delve) settings.
	Delve DelveAdapterSettings

	// Node provides Node.js debugger settings.
	Node NodeAdapterSettings

	// Python provides Python debugger settings.
	Python PythonAdapterSettings
}

// DelveAdapterSettings provides Delve-specific debug settings.
type DelveAdapterSettings struct {
	// Path is the path to the dlv executable.
	Path string

	// BuildFlags are additional build flags for dlv.
	BuildFlags string

	// Args are additional arguments to pass to dlv.
	Args []string
}

// NodeAdapterSettings provides Node.js debugger settings.
type NodeAdapterSettings struct {
	// Path is the path to the node executable.
	Path string

	// InspectPort is the debug port for Node.js inspector.
	InspectPort int

	// SourceMaps enables source map support.
	SourceMaps bool
}

// PythonAdapterSettings provides Python debugger settings.
type PythonAdapterSettings struct {
	// Path is the path to the python executable.
	Path string

	// DebuggerPath is the path to debugpy or pdb.
	DebuggerPath string

	// JustMyCode limits debugging to user code only.
	JustMyCode bool
}

// TaskSettings provides task runner settings.
type TaskSettings struct {
	// Enabled controls whether task integration is active.
	Enabled bool

	// AutoDetect enables automatic detection of task files.
	AutoDetect bool

	// DefaultShell is the default shell for running tasks.
	DefaultShell string

	// MaxConcurrent limits concurrent task execution.
	MaxConcurrent int

	// OutputBufferSize is the size of the output buffer per task.
	OutputBufferSize int

	// Sources configures task discovery from different sources.
	Sources TaskSourcesSettings
}

// TaskSourcesSettings configures task discovery sources.
type TaskSourcesSettings struct {
	// Makefile enables discovery of targets from Makefile.
	Makefile bool

	// PackageJSON enables discovery of scripts from package.json.
	PackageJSON bool

	// TasksJSON enables discovery from .vscode/tasks.json.
	TasksJSON bool

	// Custom enables discovery from custom task definitions.
	Custom bool

	// CustomPath is the path to custom task definitions file.
	CustomPath string
}

// TerminalSettings provides terminal settings.
type TerminalSettings struct {
	// Enabled controls whether terminal integration is active.
	Enabled bool

	// DefaultShell is the default shell to spawn.
	DefaultShell string

	// ShellArgs are arguments to pass to the shell.
	ShellArgs []string

	// ScrollbackLines is the number of scrollback lines to keep.
	ScrollbackLines int

	// CopyOnSelect enables copy-on-select behavior.
	CopyOnSelect bool

	// CursorStyle is the cursor style (block, underline, bar).
	CursorStyle string

	// FontSize is the terminal font size.
	FontSize int
}

// Integration returns type-safe access to integration layer settings.
func (c *Config) Integration() IntegrationSettings {
	return IntegrationSettings{
		Enabled:                c.getBoolOr("integration.enabled", true),
		WorkspaceRoot:          c.getStringOr("integration.workspaceRoot", ""),
		MaxProcesses:           c.getIntOr("integration.maxProcesses", 10),
		ShutdownTimeoutSeconds: c.getIntOr("integration.shutdownTimeout", 30),
		Git:                    c.gitSettings(),
		Debug:                  c.debugSettings(),
		Task:                   c.taskSettings(),
		Terminal:               c.terminalSettings(),
	}
}

func (c *Config) gitSettings() GitSettings {
	return GitSettings{
		Enabled:           c.getBoolOr("integration.git.enabled", true),
		AutoFetch:         c.getBoolOr("integration.git.autoFetch", false),
		AutoFetchInterval: c.getIntOr("integration.git.autoFetchInterval", 300),
		ShowInlineBlame:   c.getBoolOr("integration.git.showInlineBlame", false),
		ConfirmCommit:     c.getBoolOr("integration.git.confirmCommit", true),
		SignCommits:       c.getBoolOr("integration.git.signCommits", false),
		DefaultRemote:     c.getStringOr("integration.git.defaultRemote", "origin"),
	}
}

func (c *Config) debugSettings() DebugSettings {
	return DebugSettings{
		Enabled:               c.getBoolOr("integration.debug.enabled", true),
		DefaultAdapter:        c.getStringOr("integration.debug.defaultAdapter", ""),
		AutoAttachBreakpoints: c.getBoolOr("integration.debug.autoAttachBreakpoints", true),
		ShowInlineValues:      c.getBoolOr("integration.debug.showInlineValues", true),
		StopOnEntry:           c.getBoolOr("integration.debug.stopOnEntry", false),
		Timeout:               c.getIntOr("integration.debug.timeout", 30),
		Adapters:              c.debugAdaptersSettings(),
	}
}

func (c *Config) debugAdaptersSettings() DebugAdaptersSettings {
	return DebugAdaptersSettings{
		Delve:  c.delveAdapterSettings(),
		Node:   c.nodeAdapterSettings(),
		Python: c.pythonAdapterSettings(),
	}
}

func (c *Config) delveAdapterSettings() DelveAdapterSettings {
	return DelveAdapterSettings{
		Path:       c.getStringOr("integration.debug.adapters.delve.path", "dlv"),
		BuildFlags: c.getStringOr("integration.debug.adapters.delve.buildFlags", ""),
		Args:       c.getStringSliceOr("integration.debug.adapters.delve.args", nil),
	}
}

func (c *Config) nodeAdapterSettings() NodeAdapterSettings {
	return NodeAdapterSettings{
		Path:        c.getStringOr("integration.debug.adapters.node.path", "node"),
		InspectPort: c.getIntOr("integration.debug.adapters.node.inspectPort", 9229),
		SourceMaps:  c.getBoolOr("integration.debug.adapters.node.sourceMaps", true),
	}
}

func (c *Config) pythonAdapterSettings() PythonAdapterSettings {
	return PythonAdapterSettings{
		Path:         c.getStringOr("integration.debug.adapters.python.path", "python3"),
		DebuggerPath: c.getStringOr("integration.debug.adapters.python.debuggerPath", ""),
		JustMyCode:   c.getBoolOr("integration.debug.adapters.python.justMyCode", true),
	}
}

func (c *Config) taskSettings() TaskSettings {
	return TaskSettings{
		Enabled:          c.getBoolOr("integration.task.enabled", true),
		AutoDetect:       c.getBoolOr("integration.task.autoDetect", true),
		DefaultShell:     c.getStringOr("integration.task.defaultShell", ""),
		MaxConcurrent:    c.getIntOr("integration.task.maxConcurrent", 5),
		OutputBufferSize: c.getIntOr("integration.task.outputBufferSize", 65536),
		Sources:          c.taskSourcesSettings(),
	}
}

func (c *Config) taskSourcesSettings() TaskSourcesSettings {
	return TaskSourcesSettings{
		Makefile:    c.getBoolOr("integration.task.sources.makefile", true),
		PackageJSON: c.getBoolOr("integration.task.sources.packageJson", true),
		TasksJSON:   c.getBoolOr("integration.task.sources.tasksJson", true),
		Custom:      c.getBoolOr("integration.task.sources.custom", false),
		CustomPath:  c.getStringOr("integration.task.sources.customPath", ".keystorm/tasks.json"),
	}
}

func (c *Config) terminalSettings() TerminalSettings {
	return TerminalSettings{
		Enabled:         c.getBoolOr("integration.terminal.enabled", true),
		DefaultShell:    c.getStringOr("integration.terminal.defaultShell", ""),
		ShellArgs:       c.getStringSliceOr("integration.terminal.shellArgs", nil),
		ScrollbackLines: c.getIntOr("integration.terminal.scrollbackLines", 10000),
		CopyOnSelect:    c.getBoolOr("integration.terminal.copyOnSelect", true),
		CursorStyle:     c.getStringOr("integration.terminal.cursorStyle", "block"),
		FontSize:        c.getIntOr("integration.terminal.fontSize", 14),
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
