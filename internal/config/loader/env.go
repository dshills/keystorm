package loader

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"
)

// EnvLoader loads configuration from environment variables.
type EnvLoader struct {
	prefix  string            // Environment variable prefix (e.g., "KEYSTORM_")
	mapping map[string]string // Env var -> config path
}

// NewEnvLoader creates a new environment variable loader.
// The prefix should include the trailing underscore (e.g., "KEYSTORM_").
func NewEnvLoader(prefix string) *EnvLoader {
	return &EnvLoader{
		prefix:  prefix,
		mapping: defaultEnvMapping(),
	}
}

// NewEnvLoaderWithMapping creates a loader with custom environment variable mappings.
func NewEnvLoaderWithMapping(prefix string, mapping map[string]string) *EnvLoader {
	return &EnvLoader{
		prefix:  prefix,
		mapping: mapping,
	}
}

// defaultEnvMapping returns the default environment variable mappings.
func defaultEnvMapping() map[string]string {
	return map[string]string{
		"KEYSTORM_LOG_LEVEL":     "logging.level",
		"KEYSTORM_THEME":         "ui.theme",
		"KEYSTORM_FONT_SIZE":     "ui.fontSize",
		"KEYSTORM_TAB_SIZE":      "editor.tabSize",
		"KEYSTORM_INSERT_SPACES": "editor.insertSpaces",
		"KEYSTORM_WORD_WRAP":     "editor.wordWrap",
		"KEYSTORM_CONFIG_DIR":    "paths.configDir",
		"KEYSTORM_DATA_DIR":      "paths.dataDir",
		"KEYSTORM_CACHE_DIR":     "paths.cacheDir",
		// Sensitive settings
		"KEYSTORM_OPENAI_KEY":    "ai.openaiApiKey",
		"KEYSTORM_ANTHROPIC_KEY": "ai.anthropicApiKey",
	}
}

// Load reads environment variables and returns a configuration map.
// Note: Empty string values are treated as valid values, not as unset.
func (l *EnvLoader) Load() (map[string]any, error) {
	config := make(map[string]any)

	// First, load explicitly mapped variables
	for env, path := range l.mapping {
		if val, ok := os.LookupEnv(env); ok {
			setByPath(config, path, l.parseValue(val))
		}
	}

	// Then, scan for additional prefixed variables not in mapping
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, l.prefix) {
			continue
		}

		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		name := parts[0]
		value := parts[1]

		// Skip if already mapped
		if _, ok := l.mapping[name]; ok {
			continue
		}

		// Convert KEYSTORM_EDITOR_TAB_SIZE to editor.tabSize
		path := l.envToPath(name)
		setByPath(config, path, l.parseValue(value))
	}

	return config, nil
}

// AddMapping adds a custom environment variable mapping.
func (l *EnvLoader) AddMapping(envVar, configPath string) {
	if l.mapping == nil {
		l.mapping = make(map[string]string)
	}
	l.mapping[envVar] = configPath
}

// RemoveMapping removes an environment variable mapping.
func (l *EnvLoader) RemoveMapping(envVar string) {
	delete(l.mapping, envVar)
}

// envToPath converts KEYSTORM_EDITOR_TAB_SIZE to editor.tabSize.
func (l *EnvLoader) envToPath(env string) string {
	// Remove prefix
	name := strings.TrimPrefix(env, l.prefix)

	// Split by underscore
	parts := strings.Split(name, "_")
	if len(parts) == 0 {
		return strings.ToLower(name)
	}

	// Convert to camelCase path
	// First part is section (lowercase)
	// Subsequent parts form the setting name in camelCase
	result := make([]string, 0, 2)

	// First part is the section
	if len(parts) > 0 {
		result = append(result, strings.ToLower(parts[0]))
	}

	// Remaining parts form the setting name
	if len(parts) > 1 {
		settingParts := parts[1:]
		settingName := strings.ToLower(settingParts[0])
		for i := 1; i < len(settingParts); i++ {
			part := settingParts[i]
			if len(part) > 0 {
				settingName += strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
			}
		}
		result = append(result, settingName)
	}

	return strings.Join(result, ".")
}

// parseValue attempts to parse the string value into an appropriate type.
func (l *EnvLoader) parseValue(s string) any {
	// Empty string
	if s == "" {
		return s
	}

	// Try bool
	lower := strings.ToLower(s)
	if lower == "true" || lower == "yes" || lower == "on" || s == "1" {
		return true
	}
	if lower == "false" || lower == "no" || lower == "off" || s == "0" {
		return false
	}

	// Try int
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}

	// Try float (only if it contains a decimal point to avoid misinterpreting ints)
	if strings.Contains(s, ".") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}

	// Try duration
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	// Try JSON array/object
	if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "{") {
		var v any
		if err := json.Unmarshal([]byte(s), &v); err == nil {
			return v
		}
	}

	// Default to string
	return s
}

// setByPath sets a value in a nested map using a dot-separated path.
func setByPath(data map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := data

	// Navigate/create intermediate maps
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			// Create intermediate map
			next := make(map[string]any)
			current[part] = next
			current = next
		}
	}

	// Set the final value
	if len(parts) > 0 {
		current[parts[len(parts)-1]] = value
	}
}

// GetEnvOrDefault returns the environment variable value or a default.
func GetEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// ExpandEnvInString expands environment variables in a string.
// Supports both $VAR and ${VAR} syntax.
func ExpandEnvInString(s string) string {
	return os.ExpandEnv(s)
}
