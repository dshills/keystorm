package workspace

import (
	"encoding/json"
	"os"
	stdpath "path"
	"path/filepath"
	"strings"
)

// Config holds workspace-level configuration.
type Config struct {
	// ExcludePatterns are glob patterns to exclude from indexing/watching.
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`

	// SearchExcludePatterns are additional excludes for search only.
	SearchExcludePatterns []string `json:"search_exclude_patterns,omitempty"`

	// WatcherExcludePatterns are additional excludes for file watching.
	WatcherExcludePatterns []string `json:"watcher_exclude_patterns,omitempty"`

	// MaxFileSize is the maximum file size to index (bytes).
	// Files larger than this will be skipped during indexing.
	MaxFileSize int64 `json:"max_file_size,omitempty"`

	// IndexingConcurrency is the number of parallel indexing workers.
	IndexingConcurrency int `json:"indexing_concurrency,omitempty"`

	// FileAssociations maps file patterns to language IDs.
	FileAssociations map[string]string `json:"file_associations,omitempty"`

	// EditorSettings holds editor-specific settings.
	EditorSettings EditorSettings `json:"editor,omitempty"`
}

// EditorSettings holds editor-specific configuration.
type EditorSettings struct {
	// TabSize is the number of spaces per tab.
	TabSize int `json:"tab_size,omitempty"`

	// InsertSpaces determines whether to use spaces instead of tabs.
	InsertSpaces bool `json:"insert_spaces,omitempty"`

	// TrimTrailingWhitespace removes trailing whitespace on save.
	TrimTrailingWhitespace bool `json:"trim_trailing_whitespace,omitempty"`

	// InsertFinalNewline ensures files end with a newline.
	InsertFinalNewline bool `json:"insert_final_newline,omitempty"`

	// DefaultEncoding is the default character encoding.
	DefaultEncoding string `json:"default_encoding,omitempty"`

	// DefaultLineEnding is the default line ending style.
	DefaultLineEnding string `json:"default_line_ending,omitempty"`
}

// DefaultConfig returns the default workspace configuration.
func DefaultConfig() *Config {
	return &Config{
		ExcludePatterns: []string{
			"**/.git/**",
			"**/node_modules/**",
			"**/.DS_Store",
			"**/vendor/**",
			"**/__pycache__/**",
			"**/.venv/**",
			"**/dist/**",
			"**/build/**",
			"**/.idea/**",
			"**/.vscode/**",
			"**/target/**",
		},
		SearchExcludePatterns: []string{
			"**/*.min.js",
			"**/*.min.css",
			"**/package-lock.json",
			"**/yarn.lock",
			"**/go.sum",
		},
		WatcherExcludePatterns: []string{
			"**/*.log",
			"**/tmp/**",
		},
		MaxFileSize:         10 * 1024 * 1024, // 10 MB
		IndexingConcurrency: 4,
		FileAssociations: map[string]string{
			"*.go":         "go",
			"*.mod":        "gomod",
			"*.sum":        "gosum",
			"*.ts":         "typescript",
			"*.tsx":        "typescriptreact",
			"*.js":         "javascript",
			"*.jsx":        "javascriptreact",
			"*.py":         "python",
			"*.rs":         "rust",
			"*.java":       "java",
			"*.c":          "c",
			"*.cpp":        "cpp",
			"*.h":          "c",
			"*.hpp":        "cpp",
			"*.cs":         "csharp",
			"*.rb":         "ruby",
			"*.php":        "php",
			"*.swift":      "swift",
			"*.kt":         "kotlin",
			"*.scala":      "scala",
			"*.md":         "markdown",
			"*.json":       "json",
			"*.yaml":       "yaml",
			"*.yml":        "yaml",
			"*.toml":       "toml",
			"*.xml":        "xml",
			"*.html":       "html",
			"*.css":        "css",
			"*.scss":       "scss",
			"*.less":       "less",
			"*.sql":        "sql",
			"*.sh":         "shellscript",
			"*.bash":       "shellscript",
			"*.zsh":        "shellscript",
			"Makefile":     "makefile",
			"Dockerfile":   "dockerfile",
			"*.dockerfile": "dockerfile",
		},
		EditorSettings: EditorSettings{
			TabSize:                4,
			InsertSpaces:           true,
			TrimTrailingWhitespace: true,
			InsertFinalNewline:     true,
			DefaultEncoding:        "utf-8",
			DefaultLineEnding:      "lf",
		},
	}
}

// ConfigFileName is the name of the workspace configuration file.
const ConfigFileName = ".keystorm/workspace.json"

// LoadConfig loads configuration from a workspace folder.
// It looks for .keystorm/workspace.json in the folder.
func LoadConfig(folderPath string) (*Config, error) {
	configPath := filepath.Join(folderPath, ConfigFileName)
	return LoadConfigFromFile(configPath)
}

// LoadConfigFromFile loads configuration from a specific file path.
func LoadConfigFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig saves configuration to a workspace folder.
func SaveConfig(folderPath string, config *Config) error {
	configPath := filepath.Join(folderPath, ConfigFileName)
	return SaveConfigToFile(configPath, config)
}

// SaveConfigToFile saves configuration to a specific file path.
func SaveConfigToFile(path string, config *Config) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// MergeConfig merges two configurations, with override taking precedence.
// Returns a copy of base with override values applied.
func MergeConfig(base, override *Config) *Config {
	// Handle nil inputs
	if base == nil && override == nil {
		return DefaultConfig()
	}
	if base == nil {
		copy := *override
		return &copy
	}
	if override == nil {
		copy := *base
		return &copy
	}

	result := *base

	// Merge exclude patterns (combine lists)
	if len(override.ExcludePatterns) > 0 {
		result.ExcludePatterns = mergeStringSlices(base.ExcludePatterns, override.ExcludePatterns)
	}
	if len(override.SearchExcludePatterns) > 0 {
		result.SearchExcludePatterns = mergeStringSlices(base.SearchExcludePatterns, override.SearchExcludePatterns)
	}
	if len(override.WatcherExcludePatterns) > 0 {
		result.WatcherExcludePatterns = mergeStringSlices(base.WatcherExcludePatterns, override.WatcherExcludePatterns)
	}

	// Override scalar values if set
	if override.MaxFileSize > 0 {
		result.MaxFileSize = override.MaxFileSize
	}
	if override.IndexingConcurrency > 0 {
		result.IndexingConcurrency = override.IndexingConcurrency
	}

	// Merge file associations
	if len(override.FileAssociations) > 0 {
		result.FileAssociations = mergeStringMaps(base.FileAssociations, override.FileAssociations)
	}

	// Override editor settings
	if override.EditorSettings.TabSize > 0 {
		result.EditorSettings.TabSize = override.EditorSettings.TabSize
	}
	if override.EditorSettings.DefaultEncoding != "" {
		result.EditorSettings.DefaultEncoding = override.EditorSettings.DefaultEncoding
	}
	if override.EditorSettings.DefaultLineEnding != "" {
		result.EditorSettings.DefaultLineEnding = override.EditorSettings.DefaultLineEnding
	}
	// Booleans are tricky - always take override
	result.EditorSettings.InsertSpaces = override.EditorSettings.InsertSpaces
	result.EditorSettings.TrimTrailingWhitespace = override.EditorSettings.TrimTrailingWhitespace
	result.EditorSettings.InsertFinalNewline = override.EditorSettings.InsertFinalNewline

	return &result
}

// mergeStringSlices combines two string slices, removing duplicates.
func mergeStringSlices(base, override []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(base)+len(override))

	for _, s := range base {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range override {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

// mergeStringMaps combines two string maps, with override taking precedence.
func mergeStringMaps(base, override map[string]string) map[string]string {
	result := make(map[string]string, len(base)+len(override))

	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}

	return result
}

// IsExcluded checks if a path should be excluded based on config patterns.
func (c *Config) IsExcluded(path string) bool {
	return matchesAnyPattern(path, c.ExcludePatterns)
}

// IsSearchExcluded checks if a path should be excluded from search.
func (c *Config) IsSearchExcluded(path string) bool {
	return matchesAnyPattern(path, c.ExcludePatterns) ||
		matchesAnyPattern(path, c.SearchExcludePatterns)
}

// IsWatcherExcluded checks if a path should be excluded from watching.
func (c *Config) IsWatcherExcluded(path string) bool {
	return matchesAnyPattern(path, c.ExcludePatterns) ||
		matchesAnyPattern(path, c.WatcherExcludePatterns)
}

// GetLanguageID returns the language ID for a file path based on file associations.
func (c *Config) GetLanguageID(path string) string {
	name := filepath.Base(path)
	ext := filepath.Ext(path)

	// Check exact filename match first
	if lang, ok := c.FileAssociations[name]; ok {
		return lang
	}

	// Check extension match
	if ext != "" {
		if lang, ok := c.FileAssociations["*"+ext]; ok {
			return lang
		}
	}

	return ""
}

// matchesAnyPattern checks if path matches any of the glob patterns.
func matchesAnyPattern(path string, patterns []string) bool {
	// Normalize path to forward slashes for matching
	path = filepath.ToSlash(path)

	for _, pattern := range patterns {
		if matchGlobPattern(pattern, path) {
			return true
		}
	}
	return false
}

// matchGlobPattern matches a path against a glob pattern.
// Supports ** for matching any number of directories.
// Both pattern and path should be normalized to forward slashes before calling.
func matchGlobPattern(pattern, path string) bool {
	// Normalize both pattern and path to forward slashes
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Remove leading slash from path for consistent matching
	normalizedPath := strings.TrimPrefix(path, "/")

	// Handle ** patterns
	if pattern == "**" {
		return true
	}

	// Check if pattern contains **
	if idx := findDoublestar(pattern); idx >= 0 {
		// Split pattern around first **
		prefix := pattern[:idx]
		suffix := pattern[idx+2:]

		// Remove leading slash from suffix
		suffix = strings.TrimPrefix(suffix, "/")

		// Normalize prefix (remove leading/trailing slashes)
		prefix = trimSlashes(prefix)

		if prefix == "" {
			// Pattern like "**/.git/**" or "**/vendor/**"
			// Check if suffix contains another **
			if suffixIdx := findDoublestar(suffix); suffixIdx >= 0 {
				// Get the middle segment between ** markers
				middle := suffix[:suffixIdx]
				middle = trimSlashes(middle)
				if middle != "" {
					// Check if path contains the middle segment
					return containsSegment(normalizedPath, middle)
				}
				return true
			}
			// Pattern like "**/vendor" - check if path ends with or contains segment
			if suffix == "" {
				return true
			}
			return containsSegment(normalizedPath, suffix)
		}

		// Pattern like "src/**/test"
		// Check if normalized path has the prefix
		if !strings.HasPrefix(normalizedPath, prefix) {
			return false
		}
		// Also ensure it's at a path boundary
		if len(normalizedPath) > len(prefix) && normalizedPath[len(prefix)] != '/' {
			return false
		}

		if suffix == "" {
			return true
		}
		// Recursively check the remaining pattern
		return matchGlobPattern("**/"+suffix, normalizedPath)
	}

	// Standard glob matching using path.Match (not filepath.Match)
	// since we've normalized to forward slashes
	baseName := normalizedPath
	if idx := strings.LastIndex(normalizedPath, "/"); idx >= 0 {
		baseName = normalizedPath[idx+1:]
	}

	// Try matching against base name first
	if matched, _ := stdpath.Match(pattern, baseName); matched {
		return true
	}
	// Also try matching the full normalized path
	if matched, _ := stdpath.Match(pattern, normalizedPath); matched {
		return true
	}
	return false
}

// findDoublestar returns the index of ** in pattern, or -1 if not found.
func findDoublestar(pattern string) int {
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] == '*' && pattern[i+1] == '*' {
			return i
		}
	}
	return -1
}

// trimSlashes removes leading and trailing slashes.
func trimSlashes(s string) string {
	return strings.Trim(s, "/")
}

// containsSegment checks if path contains a directory segment.
// Handles patterns like ".git" in "**/.git/**".
func containsSegment(path, segment string) bool {
	// Remove any slashes for simple segment matching
	segment = trimSlashes(segment)

	// Check for exact segment match
	parts := splitPath(path)
	for _, part := range parts {
		// Try exact match
		if part == segment {
			return true
		}
		// Try glob match using path.Match for forward-slash paths
		if matched, _ := stdpath.Match(segment, part); matched {
			return true
		}
	}
	return false
}

// splitPath splits a path into segments, filtering empty parts.
func splitPath(path string) []string {
	// Normalize to forward slashes
	path = filepath.ToSlash(path)

	// Split and filter empty segments
	parts := strings.Split(path, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
