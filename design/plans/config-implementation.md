# Keystorm Configuration System - Implementation Plan

## Comprehensive Design Document for `internal/config`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Requirements Analysis](#2-requirements-analysis)
3. [Architecture Overview](#3-architecture-overview)
4. [Package Structure](#4-package-structure)
5. [Core Types and Interfaces](#5-core-types-and-interfaces)
6. [Configuration Sources](#6-configuration-sources)
7. [Schema and Validation](#7-schema-and-validation)
8. [Settings Registry](#8-settings-registry)
9. [Watch and Live Reload](#9-watch-and-live-reload)
10. [Language and Filetype Settings](#10-language-and-filetype-settings)
11. [Plugin Configuration](#11-plugin-configuration)
12. [Integration with Other Modules](#12-integration-with-other-modules)
13. [Implementation Phases](#13-implementation-phases)
14. [Testing Strategy](#14-testing-strategy)
15. [Performance Considerations](#15-performance-considerations)

---

## 1. Executive Summary

The Configuration System is the central settings management layer for Keystorm. It handles loading, merging, validating, and providing access to all editor configuration including user preferences, keymaps, per-language settings, and plugin configurations. This is "where all chaos begins" - a well-designed config system prevents that chaos from spreading.

### Role in the Architecture

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   Input Engine  │─────▶│  Config System  │◀────▶│   File System   │
│   (keymaps)     │      │ (internal/      │      │   (JSON/TOML)   │
└─────────────────┘      │  config)        │      └─────────────────┘
                         └────────┬────────┘
                                  │
                    ┌─────────────┼─────────────┬─────────────┐
                    ▼             ▼             ▼             ▼
            ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐
            │  Renderer │  │  Plugin   │  │  Project  │  │    LSP    │
            │ (themes)  │  │  System   │  │  (ignore) │  │ (server   │
            │           │  │           │  │           │  │  settings)│
            └───────────┘  └───────────┘  └───────────┘  └───────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Layered configuration | User > Project > Language > Default allows flexible override hierarchy |
| TOML as primary format | Human-readable, supports comments, well-suited for config files |
| JSON Schema validation | Enables IDE support for config files, clear error messages |
| Type-safe access API | Prevents runtime errors from typos in setting names |
| Live reload with debounce | Config changes apply immediately without restart |
| Observer pattern | Components subscribe to setting changes they care about |
| Lazy loading | Only load sections when accessed to minimize startup time |

### Integration Points

The Configuration System connects to:
- **Input Engine**: Provides keymaps, mode settings, key timeouts
- **Renderer**: Provides theme, colors, font settings, UI preferences
- **Plugin System**: Provides plugin-specific configuration sections
- **Project Model**: Provides workspace settings, ignore patterns
- **LSP Client**: Provides language server paths, settings per server
- **Event Bus**: Publishes config change events
- **Dispatcher**: Provides action bindings, command settings

---

## 2. Requirements Analysis

### 2.1 From Architecture Specification

From `design/specs/architecture.md`:

> "Configuration System - Where all chaos begins. Config files, settings, overrides, per-language configs, keymaps, plugin settings."

> "Config Format: Declarative pipeline definitions (YAML/JSON/TOML/whatever). Strongly typed interfaces where possible."

### 2.2 Functional Requirements

1. **Configuration Sources**
   - Default built-in configuration
   - User global config (`~/.config/keystorm/config.toml`)
   - User settings (`~/.config/keystorm/settings.toml`)
   - User keymaps (`~/.config/keystorm/keymaps.toml`)
   - Project/workspace config (`.keystorm/config.toml`)
   - Language-specific config (per-language sections)
   - Plugin configuration sections
   - Environment variables (select settings)
   - Command-line arguments (overrides)

2. **Configuration Categories**
   - **Editor**: tabs, spaces, line numbers, word wrap, etc.
   - **Input**: key timeout, leader key, mode settings
   - **Keymaps**: key bindings per mode
   - **UI/Renderer**: theme, colors, fonts, statusline
   - **Project**: workspace settings, ignore patterns
   - **Languages**: per-language overrides (tab size, formatter)
   - **LSP**: language server configurations
   - **Plugins**: plugin-specific settings
   - **AI**: model settings, context limits (future)

3. **Configuration Formats**
   - TOML (primary, human-editable)
   - JSON (compatibility with VS Code settings)
   - Environment variables (for sensitive data)

4. **Type Safety**
   - Strongly typed settings with defaults
   - Schema validation with clear error messages
   - Enum validation for constrained values
   - Range validation for numeric values

5. **Live Reload**
   - Watch config files for changes
   - Debounce rapid changes
   - Validate before applying
   - Notify subscribers of changes
   - Rollback on invalid config

6. **Access Patterns**
   - Get single setting by path (`editor.tabSize`)
   - Get section as struct (`editor.*`)
   - Subscribe to setting changes
   - Override settings programmatically
   - Query effective value (resolved from layers)

### 2.3 Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Config load time | < 50ms for full config tree |
| Setting lookup | O(1) via path map |
| Memory footprint | < 5MB for config data |
| File watch latency | < 100ms debounce |
| Schema validation | < 10ms for full config |

---

## 3. Architecture Overview

### 3.1 Component Diagram

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                          Configuration System                                  │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                                │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐    │
│  │   Config         │  │   Schema         │  │      Settings            │    │
│  │   Loader         │  │   Validator      │  │      Registry            │    │
│  │  - TOML parser   │  │  - JSON Schema   │  │  - Type-safe access     │    │
│  │  - JSON parser   │  │  - Type checking │  │  - Path-based lookup    │    │
│  │  - Env vars      │  │  - Range checks  │  │  - Default values       │    │
│  └──────────────────┘  └──────────────────┘  └──────────────────────────┘    │
│                                                                                │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐    │
│  │   Layer          │  │   File           │  │      Change              │    │
│  │   Manager        │  │   Watcher        │  │      Notifier            │    │
│  │  - Precedence    │  │  - fsnotify      │  │  - Observers             │    │
│  │  - Merging       │  │  - Debounce      │  │  - Callbacks             │    │
│  │  - Resolution    │  │  - Reload        │  │  - Events                │    │
│  └──────────────────┘  └──────────────────┘  └──────────────────────────┘    │
│                                                                                │
│  ┌──────────────────────────────────────────────────────────────────────┐    │
│  │                     Configuration Sections                            │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │  Editor  │ │   Input  │ │  Keymap  │ │    UI    │ │  Project │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │ Language │ │   LSP    │ │  Plugin  │ │    AI    │ │  Debug   │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  └──────────────────────────────────────────────────────────────────────┘    │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Layer Hierarchy

Configuration is loaded and merged in layers with higher layers overriding lower:

```
┌─────────────────────────────┐
│  7. Command Line Arguments  │  ← Highest priority
├─────────────────────────────┤
│  6. Environment Variables   │
├─────────────────────────────┤
│  5. Plugin Settings         │
├─────────────────────────────┤
│  4. Project/Workspace       │  ← .keystorm/config.toml
├─────────────────────────────┤
│  3. User Keymaps            │  ← ~/.config/keystorm/keymaps.toml
├─────────────────────────────┤
│  2. User Settings           │  ← ~/.config/keystorm/settings.toml
├─────────────────────────────┤
│  1. Built-in Defaults       │  ← Lowest priority
└─────────────────────────────┘
```

### 3.3 Data Flow

```
Config File Change (detected by watcher)
       │
       ▼
┌──────────────────┐
│   Config Loader  │  Parse TOML/JSON to intermediate representation
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Schema Validator │  Validate against JSON Schema
└────────┬─────────┘
         │ (if valid)
         ▼
┌──────────────────┐
│  Layer Manager   │  Merge into appropriate layer
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Settings Registry│  Update resolved values, compute effective settings
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Change Notifier  │  Notify subscribers of changed settings
└────────┬─────────┘
         │
         ▼
Subscribers (Input, Renderer, Plugins, etc.)
```

---

## 4. Package Structure

```
internal/config/
├── doc.go                      # Package documentation
├── config.go                   # Main Config type and API
├── errors.go                   # Error types
│
├── loader/                     # Configuration loading
│   ├── loader.go               # Loader interface
│   ├── toml.go                 # TOML loader
│   ├── json.go                 # JSON loader
│   ├── env.go                  # Environment variable loader
│   ├── args.go                 # Command-line argument loader
│   └── loader_test.go
│
├── schema/                     # Schema and validation
│   ├── schema.go               # Schema types
│   ├── validator.go            # JSON Schema validator
│   ├── types.go                # Type definitions for validation
│   ├── errors.go               # Validation error types
│   └── schema_test.go
│
├── layer/                      # Layer management
│   ├── layer.go                # Layer type
│   ├── manager.go              # LayerManager
│   ├── merge.go                # Merge strategies
│   ├── precedence.go           # Precedence rules
│   └── layer_test.go
│
├── registry/                   # Settings registry
│   ├── registry.go             # SettingsRegistry
│   ├── setting.go              # Setting definition
│   ├── accessor.go             # Type-safe accessors
│   ├── path.go                 # Path parsing and navigation
│   └── registry_test.go
│
├── watcher/                    # File watching
│   ├── watcher.go              # ConfigWatcher
│   ├── debounce.go             # Change debouncing
│   └── watcher_test.go
│
├── notify/                     # Change notification
│   ├── notifier.go             # ChangeNotifier
│   ├── subscriber.go           # Subscriber interface
│   └── notify_test.go
│
├── sections/                   # Configuration sections
│   ├── editor.go               # EditorConfig
│   ├── input.go                # InputConfig
│   ├── keymap.go               # KeymapConfig
│   ├── ui.go                   # UIConfig
│   ├── project.go              # ProjectConfig
│   ├── language.go             # LanguageConfig
│   ├── lsp.go                  # LSPConfig
│   ├── plugin.go               # PluginConfig
│   ├── ai.go                   # AIConfig (future)
│   └── sections_test.go
│
├── defaults/                   # Default configurations
│   ├── defaults.go             # DefaultConfig factory
│   ├── editor.go               # Default editor settings
│   ├── keymaps.go              # Default keymaps
│   ├── themes.go               # Default themes
│   └── languages.go            # Default language settings
│
├── migration/                  # Config migration
│   ├── migrate.go              # Migration framework
│   ├── v1_to_v2.go             # Version migrations
│   └── migrate_test.go
│
└── integration.go              # ConfigSystem facade
    integration_test.go
```

### Rationale

- **loader/**: Isolates parsing logic for each format
- **schema/**: Centralizes validation with reusable validators
- **layer/**: Manages the complexity of multi-layer merging
- **registry/**: Provides the primary access API
- **sections/**: Type-safe structs for each config category
- **defaults/**: Embedded defaults separate from loading logic
- **migration/**: Future-proofs for config format evolution

---

## 5. Core Types and Interfaces

### 5.1 Config Interface

```go
// internal/config/config.go

// Config is the main configuration interface.
// It provides access to all editor settings with type safety.
type Config interface {
    // Access settings by path
    Get(path string) (any, error)
    GetString(path string) (string, error)
    GetInt(path string) (int, error)
    GetInt64(path string) (int64, error)
    GetFloat64(path string) (float64, error)
    GetBool(path string) (bool, error)
    GetStringSlice(path string) ([]string, error)
    GetDuration(path string) (time.Duration, error)

    // Access typed sections
    Editor() *EditorConfig
    Input() *InputConfig
    UI() *UIConfig
    Project() *ProjectConfig
    Language(lang string) *LanguageConfig
    LSP(server string) *LSPServerConfig
    Plugin(name string) map[string]any

    // Keymaps
    Keymaps() *KeymapConfig
    KeymapForMode(mode string) []KeyBinding

    // Modify settings (programmatic override)
    Set(path string, value any) error
    SetForSession(path string, value any) error // Doesn't persist

    // Watch for changes
    OnChange(path string, handler ChangeHandler) UnsubscribeFunc
    OnSectionChange(section string, handler SectionChangeHandler) UnsubscribeFunc

    // Reload from disk
    Reload() error

    // Persistence
    Save() error
    SaveUserSettings() error

    // Validation
    Validate() []ValidationError

    // Schema
    Schema() *Schema
    SchemaForSection(section string) *Schema
}

// ChangeHandler is called when a specific setting changes.
type ChangeHandler func(path string, oldValue, newValue any)

// SectionChangeHandler is called when any setting in a section changes.
type SectionChangeHandler func(section string, changes []SettingChange)

// UnsubscribeFunc removes a change subscription.
type UnsubscribeFunc func()

// SettingChange describes a single setting change.
type SettingChange struct {
    Path     string
    OldValue any
    NewValue any
}
```

### 5.2 Setting Definition

```go
// internal/config/registry/setting.go

// Setting defines a configuration setting with its metadata.
type Setting struct {
    // Path is the dot-separated path (e.g., "editor.tabSize")
    Path string

    // Type is the setting's data type
    Type SettingType

    // Default is the default value
    Default any

    // Description is human-readable documentation
    Description string

    // Scope defines where this setting can be set
    Scope SettingScope

    // Enum lists allowed values for enum types
    Enum []any

    // Minimum and Maximum for numeric types
    Minimum *float64
    Maximum *float64

    // Pattern for string validation (regex)
    Pattern string

    // Deprecated marks settings that should be migrated
    Deprecated bool
    DeprecatedMessage string
    ReplacedBy string

    // Tags for filtering/grouping
    Tags []string
}

// SettingType represents the data type of a setting.
type SettingType uint8

const (
    TypeString SettingType = iota
    TypeInt
    TypeFloat
    TypeBool
    TypeArray
    TypeObject
    TypeDuration
    TypeEnum
)

// SettingScope defines where a setting can be configured.
type SettingScope uint8

const (
    ScopeGlobal      SettingScope = 1 << iota // User global only
    ScopeWorkspace                            // Workspace level
    ScopeLanguage                             // Per-language override
    ScopeResource                             // Per-file (future)

    ScopeAll = ScopeGlobal | ScopeWorkspace | ScopeLanguage | ScopeResource
)
```

### 5.3 Layer Types

```go
// internal/config/layer/layer.go

// Layer represents a single configuration layer.
type Layer struct {
    // Name identifies the layer (e.g., "user", "workspace", "default")
    Name string

    // Priority determines merge order (higher overrides lower)
    Priority int

    // Source is where this layer was loaded from
    Source LayerSource

    // Path is the file path (if from file)
    Path string

    // Data holds the configuration values
    Data map[string]any

    // ModTime is when the source was last modified
    ModTime time.Time

    // ReadOnly prevents modifications
    ReadOnly bool
}

// LayerSource indicates where a layer came from.
type LayerSource uint8

const (
    SourceBuiltin    LayerSource = iota // Built-in defaults
    SourceUserGlobal                    // ~/.config/keystorm/
    SourceWorkspace                     // .keystorm/
    SourceEnv                           // Environment variables
    SourceArgs                          // Command-line arguments
    SourcePlugin                        // Plugin-provided
    SourceSession                       // In-memory session override
)

// LayerManager manages configuration layers and merging.
type LayerManager struct {
    mu     sync.RWMutex
    layers []*Layer // Sorted by priority
    merged map[string]any
}

// Merge combines all layers into a single resolved configuration.
func (m *LayerManager) Merge() map[string]any {
    m.mu.Lock()
    defer m.mu.Unlock()

    result := make(map[string]any)

    // Apply layers in priority order (lowest first, highest last)
    for _, layer := range m.layers {
        result = deepMerge(result, layer.Data)
    }

    m.merged = result
    return result
}

// GetEffectiveValue returns the resolved value for a path.
func (m *LayerManager) GetEffectiveValue(path string) (any, *Layer, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // Search layers from highest to lowest priority
    for i := len(m.layers) - 1; i >= 0; i-- {
        layer := m.layers[i]
        if val, ok := getByPath(layer.Data, path); ok {
            return val, layer, true
        }
    }

    return nil, nil, false
}
```

### 5.4 Validation Types

```go
// internal/config/schema/validator.go

// Validator validates configuration against a schema.
type Validator interface {
    // Validate checks the configuration and returns errors.
    Validate(config map[string]any) []ValidationError

    // ValidateSetting checks a single setting value.
    ValidateSetting(path string, value any) *ValidationError

    // Schema returns the JSON Schema.
    Schema() *Schema
}

// ValidationError describes a validation failure.
type ValidationError struct {
    Path    string
    Message string
    Value   any
    Code    ValidationErrorCode
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("%s: %s (value: %v)", e.Path, e.Message, e.Value)
}

// ValidationErrorCode categorizes validation errors.
type ValidationErrorCode uint8

const (
    ErrUnknownSetting ValidationErrorCode = iota
    ErrTypeMismatch
    ErrOutOfRange
    ErrInvalidEnum
    ErrPatternMismatch
    ErrRequiredMissing
    ErrDeprecated
)

// Schema represents the configuration schema.
type Schema struct {
    Version     string
    Properties  map[string]*PropertySchema
    Definitions map[string]*PropertySchema
}

// PropertySchema defines a single property in the schema.
type PropertySchema struct {
    Type        string
    Description string
    Default     any
    Enum        []any
    Minimum     *float64
    Maximum     *float64
    Pattern     string
    Items       *PropertySchema // For arrays
    Properties  map[string]*PropertySchema // For objects
    Required    []string
    Deprecated  bool
}
```

---

## 6. Configuration Sources

### 6.1 TOML Loader

```go
// internal/config/loader/toml.go

// TOMLLoader loads configuration from TOML files.
type TOMLLoader struct {
    fs vfs.VFS // File system abstraction
}

// NewTOMLLoader creates a new TOML loader.
func NewTOMLLoader(fs vfs.VFS) *TOMLLoader {
    return &TOMLLoader{fs: fs}
}

// Load parses a TOML file into a configuration map.
func (l *TOMLLoader) Load(path string) (map[string]any, error) {
    data, err := l.fs.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil // File doesn't exist, not an error
        }
        return nil, fmt.Errorf("reading config file: %w", err)
    }

    var config map[string]any
    if err := toml.Unmarshal(data, &config); err != nil {
        return nil, &ParseError{
            Path:    path,
            Message: err.Error(),
            Err:     err,
        }
    }

    return config, nil
}

// LoadWithIncludes handles @include directives.
func (l *TOMLLoader) LoadWithIncludes(path string, maxDepth int) (map[string]any, error) {
    if maxDepth <= 0 {
        return nil, fmt.Errorf("include depth exceeded")
    }

    config, err := l.Load(path)
    if err != nil {
        return nil, err
    }

    // Process includes
    if includes, ok := config["@include"].([]any); ok {
        baseDir := filepath.Dir(path)
        for _, inc := range includes {
            incPath := filepath.Join(baseDir, inc.(string))
            incConfig, err := l.LoadWithIncludes(incPath, maxDepth-1)
            if err != nil {
                return nil, fmt.Errorf("loading include %s: %w", incPath, err)
            }
            config = deepMerge(incConfig, config)
        }
        delete(config, "@include")
    }

    return config, nil
}
```

### 6.2 Environment Variable Loader

```go
// internal/config/loader/env.go

// EnvLoader loads configuration from environment variables.
type EnvLoader struct {
    prefix  string // Environment variable prefix (e.g., "KEYSTORM_")
    mapping map[string]string // Env var -> config path
}

// NewEnvLoader creates a new environment variable loader.
func NewEnvLoader(prefix string) *EnvLoader {
    return &EnvLoader{
        prefix: prefix,
        mapping: defaultEnvMapping(),
    }
}

// defaultEnvMapping returns the default environment variable mappings.
func defaultEnvMapping() map[string]string {
    return map[string]string{
        "KEYSTORM_LOG_LEVEL":     "logging.level",
        "KEYSTORM_THEME":         "ui.theme",
        "KEYSTORM_FONT_SIZE":     "ui.fontSize",
        "KEYSTORM_TAB_SIZE":      "editor.tabSize",
        "KEYSTORM_CONFIG_DIR":    "paths.configDir",
        "KEYSTORM_DATA_DIR":      "paths.dataDir",
        "KEYSTORM_CACHE_DIR":     "paths.cacheDir",
        // Sensitive settings
        "KEYSTORM_OPENAI_KEY":    "ai.openaiApiKey",
        "KEYSTORM_ANTHROPIC_KEY": "ai.anthropicApiKey",
    }
}

// Load reads environment variables and returns a config map.
func (l *EnvLoader) Load() (map[string]any, error) {
    config := make(map[string]any)

    for env, path := range l.mapping {
        if val := os.Getenv(env); val != "" {
            setByPath(config, path, l.parseValue(val))
        }
    }

    // Also scan for KEYSTORM_* variables not in mapping
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

// envToPath converts KEYSTORM_EDITOR_TAB_SIZE to editor.tabSize.
func (l *EnvLoader) envToPath(env string) string {
    // Remove prefix
    name := strings.TrimPrefix(env, l.prefix)

    // Convert to lowercase with dots
    parts := strings.Split(strings.ToLower(name), "_")

    // CamelCase within parts
    for i := 1; i < len(parts); i++ {
        if len(parts[i]) > 0 {
            // Keep first part lowercase, camelCase rest
            // tab_size -> tabSize
        }
    }

    return strings.Join(parts, ".")
}

// parseValue attempts to parse the string value into an appropriate type.
func (l *EnvLoader) parseValue(s string) any {
    // Try bool
    if s == "true" || s == "1" {
        return true
    }
    if s == "false" || s == "0" {
        return false
    }

    // Try int
    if i, err := strconv.ParseInt(s, 10, 64); err == nil {
        return i
    }

    // Try float
    if f, err := strconv.ParseFloat(s, 64); err == nil {
        return f
    }

    // Try duration
    if d, err := time.ParseDuration(s); err == nil {
        return d
    }

    // Try JSON array/object
    if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "{") {
        var v any
        if json.Unmarshal([]byte(s), &v) == nil {
            return v
        }
    }

    // Default to string
    return s
}
```

### 6.3 Command-Line Argument Loader

```go
// internal/config/loader/args.go

// ArgsLoader parses command-line arguments into configuration.
type ArgsLoader struct {
    args []string
}

// NewArgsLoader creates a loader for command-line arguments.
func NewArgsLoader(args []string) *ArgsLoader {
    return &ArgsLoader{args: args}
}

// Load parses --config.path=value style arguments.
func (l *ArgsLoader) Load() (map[string]any, error) {
    config := make(map[string]any)

    for _, arg := range l.args {
        // Support --config.path=value and --config.path value
        if !strings.HasPrefix(arg, "--") {
            continue
        }

        arg = strings.TrimPrefix(arg, "--")

        var path, value string
        if idx := strings.Index(arg, "="); idx >= 0 {
            path = arg[:idx]
            value = arg[idx+1:]
        } else {
            // Next arg is value (not implemented in this simple version)
            continue
        }

        // Only accept config.* arguments
        if !strings.HasPrefix(path, "config.") {
            continue
        }

        path = strings.TrimPrefix(path, "config.")
        setByPath(config, path, parseArgValue(value))
    }

    return config, nil
}
```

---

## 7. Schema and Validation

### 7.1 Schema Definition

```go
// internal/config/schema/schema.go

// LoadSchema loads the configuration schema from embedded resources.
func LoadSchema() (*Schema, error) {
    data, err := embeddedFS.ReadFile("schema/keystorm-config.schema.json")
    if err != nil {
        return nil, fmt.Errorf("loading embedded schema: %w", err)
    }

    var schema Schema
    if err := json.Unmarshal(data, &schema); err != nil {
        return nil, fmt.Errorf("parsing schema: %w", err)
    }

    return &schema, nil
}

// BuildSchema programmatically builds the schema from registered settings.
func BuildSchema(registry *SettingsRegistry) *Schema {
    schema := &Schema{
        Version:    "1.0.0",
        Properties: make(map[string]*PropertySchema),
    }

    for _, setting := range registry.All() {
        prop := &PropertySchema{
            Type:        setting.Type.String(),
            Description: setting.Description,
            Default:     setting.Default,
            Deprecated:  setting.Deprecated,
        }

        if setting.Enum != nil {
            prop.Enum = setting.Enum
        }
        if setting.Minimum != nil {
            prop.Minimum = setting.Minimum
        }
        if setting.Maximum != nil {
            prop.Maximum = setting.Maximum
        }
        if setting.Pattern != "" {
            prop.Pattern = setting.Pattern
        }

        schema.Properties[setting.Path] = prop
    }

    return schema
}
```

### 7.2 Validator Implementation

```go
// internal/config/schema/validator.go

// SchemaValidator validates configuration against the schema.
type SchemaValidator struct {
    schema   *Schema
    registry *SettingsRegistry
}

// NewSchemaValidator creates a new validator.
func NewSchemaValidator(schema *Schema, registry *SettingsRegistry) *SchemaValidator {
    return &SchemaValidator{
        schema:   schema,
        registry: registry,
    }
}

// Validate checks the entire configuration.
func (v *SchemaValidator) Validate(config map[string]any) []ValidationError {
    var errors []ValidationError

    // Flatten config to paths
    paths := flattenConfig(config)

    for path, value := range paths {
        if err := v.ValidateSetting(path, value); err != nil {
            errors = append(errors, *err)
        }
    }

    return errors
}

// ValidateSetting validates a single setting.
func (v *SchemaValidator) ValidateSetting(path string, value any) *ValidationError {
    // Get setting definition
    setting := v.registry.Get(path)
    if setting == nil {
        // Unknown setting - could be plugin setting, allow it
        if !isPluginPath(path) {
            return &ValidationError{
                Path:    path,
                Message: "unknown setting",
                Value:   value,
                Code:    ErrUnknownSetting,
            }
        }
        return nil
    }

    // Check deprecated
    if setting.Deprecated {
        return &ValidationError{
            Path:    path,
            Message: setting.DeprecatedMessage,
            Value:   value,
            Code:    ErrDeprecated,
        }
    }

    // Type check
    if err := v.validateType(setting, value); err != nil {
        return err
    }

    // Enum check
    if setting.Enum != nil {
        if !containsValue(setting.Enum, value) {
            return &ValidationError{
                Path:    path,
                Message: fmt.Sprintf("value must be one of: %v", setting.Enum),
                Value:   value,
                Code:    ErrInvalidEnum,
            }
        }
    }

    // Range check
    if setting.Type == TypeInt || setting.Type == TypeFloat {
        if err := v.validateRange(setting, value); err != nil {
            return err
        }
    }

    // Pattern check
    if setting.Pattern != "" && setting.Type == TypeString {
        if str, ok := value.(string); ok {
            re, err := regexp.Compile(setting.Pattern)
            if err == nil && !re.MatchString(str) {
                return &ValidationError{
                    Path:    path,
                    Message: fmt.Sprintf("value must match pattern: %s", setting.Pattern),
                    Value:   value,
                    Code:    ErrPatternMismatch,
                }
            }
        }
    }

    return nil
}

// validateType checks if the value matches the expected type.
func (v *SchemaValidator) validateType(setting *Setting, value any) *ValidationError {
    var ok bool

    switch setting.Type {
    case TypeString:
        _, ok = value.(string)
    case TypeInt:
        switch value.(type) {
        case int, int64, float64:
            ok = true
        }
    case TypeFloat:
        switch value.(type) {
        case float64, int, int64:
            ok = true
        }
    case TypeBool:
        _, ok = value.(bool)
    case TypeArray:
        _, ok = value.([]any)
    case TypeObject:
        _, ok = value.(map[string]any)
    case TypeDuration:
        switch v := value.(type) {
        case string:
            _, err := time.ParseDuration(v)
            ok = err == nil
        case time.Duration:
            ok = true
        }
    default:
        ok = true
    }

    if !ok {
        return &ValidationError{
            Path:    setting.Path,
            Message: fmt.Sprintf("expected %s, got %T", setting.Type, value),
            Value:   value,
            Code:    ErrTypeMismatch,
        }
    }

    return nil
}

// validateRange checks numeric ranges.
func (v *SchemaValidator) validateRange(setting *Setting, value any) *ValidationError {
    var num float64
    switch n := value.(type) {
    case int:
        num = float64(n)
    case int64:
        num = float64(n)
    case float64:
        num = n
    default:
        return nil
    }

    if setting.Minimum != nil && num < *setting.Minimum {
        return &ValidationError{
            Path:    setting.Path,
            Message: fmt.Sprintf("value must be >= %v", *setting.Minimum),
            Value:   value,
            Code:    ErrOutOfRange,
        }
    }

    if setting.Maximum != nil && num > *setting.Maximum {
        return &ValidationError{
            Path:    setting.Path,
            Message: fmt.Sprintf("value must be <= %v", *setting.Maximum),
            Value:   value,
            Code:    ErrOutOfRange,
        }
    }

    return nil
}
```

---

## 8. Settings Registry

### 8.1 Registry Implementation

```go
// internal/config/registry/registry.go

// SettingsRegistry manages all setting definitions and provides access.
type SettingsRegistry struct {
    mu       sync.RWMutex
    settings map[string]*Setting
    sections map[string][]*Setting // Section -> settings

    // Resolved values
    values   map[string]any
    layers   *LayerManager

    // Change notification
    notifier *ChangeNotifier
}

// NewSettingsRegistry creates a new registry with default settings.
func NewSettingsRegistry() *SettingsRegistry {
    r := &SettingsRegistry{
        settings: make(map[string]*Setting),
        sections: make(map[string][]*Setting),
        values:   make(map[string]any),
        notifier: NewChangeNotifier(),
    }

    // Register built-in settings
    r.registerBuiltinSettings()

    return r
}

// Register adds a setting definition.
func (r *SettingsRegistry) Register(setting *Setting) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.settings[setting.Path]; exists {
        return fmt.Errorf("setting already registered: %s", setting.Path)
    }

    r.settings[setting.Path] = setting

    // Add to section index
    section := getSection(setting.Path)
    r.sections[section] = append(r.sections[section], setting)

    // Set default value
    r.values[setting.Path] = setting.Default

    return nil
}

// Get retrieves a setting definition.
func (r *SettingsRegistry) Get(path string) *Setting {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.settings[path]
}

// GetValue retrieves the current effective value.
func (r *SettingsRegistry) GetValue(path string) (any, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    val, ok := r.values[path]
    return val, ok
}

// SetValue updates a value and notifies subscribers.
func (r *SettingsRegistry) SetValue(path string, value any) error {
    r.mu.Lock()

    oldValue := r.values[path]
    r.values[path] = value

    r.mu.Unlock()

    // Notify outside lock
    if oldValue != value {
        r.notifier.Notify(path, oldValue, value)
    }

    return nil
}

// All returns all registered settings.
func (r *SettingsRegistry) All() []*Setting {
    r.mu.RLock()
    defer r.mu.RUnlock()

    result := make([]*Setting, 0, len(r.settings))
    for _, s := range r.settings {
        result = append(result, s)
    }
    return result
}

// Section returns all settings in a section.
func (r *SettingsRegistry) Section(name string) []*Setting {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.sections[name]
}

// OnChange subscribes to changes for a specific path.
func (r *SettingsRegistry) OnChange(path string, handler ChangeHandler) UnsubscribeFunc {
    return r.notifier.Subscribe(path, handler)
}

// registerBuiltinSettings registers all built-in editor settings.
func (r *SettingsRegistry) registerBuiltinSettings() {
    // Editor settings
    r.Register(&Setting{
        Path:        "editor.tabSize",
        Type:        TypeInt,
        Default:     4,
        Description: "Number of spaces a tab is equal to",
        Scope:       ScopeAll,
        Minimum:     ptr(1.0),
        Maximum:     ptr(32.0),
        Tags:        []string{"editor", "whitespace"},
    })

    r.Register(&Setting{
        Path:        "editor.insertSpaces",
        Type:        TypeBool,
        Default:     true,
        Description: "Insert spaces when pressing Tab",
        Scope:       ScopeAll,
        Tags:        []string{"editor", "whitespace"},
    })

    r.Register(&Setting{
        Path:        "editor.lineNumbers",
        Type:        TypeEnum,
        Default:     "on",
        Description: "Controls line number display",
        Scope:       ScopeAll,
        Enum:        []any{"on", "off", "relative"},
        Tags:        []string{"editor", "display"},
    })

    r.Register(&Setting{
        Path:        "editor.wordWrap",
        Type:        TypeEnum,
        Default:     "off",
        Description: "Controls how lines should wrap",
        Scope:       ScopeAll,
        Enum:        []any{"off", "on", "wordWrapColumn", "bounded"},
        Tags:        []string{"editor", "display"},
    })

    r.Register(&Setting{
        Path:        "editor.wordWrapColumn",
        Type:        TypeInt,
        Default:     80,
        Description: "Column at which to wrap lines",
        Scope:       ScopeAll,
        Minimum:     ptr(20.0),
        Maximum:     ptr(500.0),
        Tags:        []string{"editor", "display"},
    })

    r.Register(&Setting{
        Path:        "editor.cursorStyle",
        Type:        TypeEnum,
        Default:     "block",
        Description: "Cursor style in normal mode",
        Scope:       ScopeGlobal,
        Enum:        []any{"block", "line", "underline"},
        Tags:        []string{"editor", "cursor"},
    })

    r.Register(&Setting{
        Path:        "editor.cursorBlink",
        Type:        TypeBool,
        Default:     true,
        Description: "Whether the cursor should blink",
        Scope:       ScopeGlobal,
        Tags:        []string{"editor", "cursor"},
    })

    r.Register(&Setting{
        Path:        "editor.scrolloff",
        Type:        TypeInt,
        Default:     5,
        Description: "Minimum lines to keep above/below cursor",
        Scope:       ScopeAll,
        Minimum:     ptr(0.0),
        Maximum:     ptr(100.0),
        Tags:        []string{"editor", "scroll"},
    })

    r.Register(&Setting{
        Path:        "editor.autoSave",
        Type:        TypeEnum,
        Default:     "off",
        Description: "Controls auto save behavior",
        Scope:       ScopeAll,
        Enum:        []any{"off", "afterDelay", "onFocusChange", "onWindowChange"},
        Tags:        []string{"editor", "save"},
    })

    r.Register(&Setting{
        Path:        "editor.autoSaveDelay",
        Type:        TypeInt,
        Default:     1000,
        Description: "Auto save delay in milliseconds",
        Scope:       ScopeAll,
        Minimum:     ptr(100.0),
        Tags:        []string{"editor", "save"},
    })

    // Input settings
    r.Register(&Setting{
        Path:        "input.keyTimeout",
        Type:        TypeDuration,
        Default:     "1s",
        Description: "Timeout for key sequences",
        Scope:       ScopeGlobal,
        Tags:        []string{"input", "vim"},
    })

    r.Register(&Setting{
        Path:        "input.leaderKey",
        Type:        TypeString,
        Default:     "\\",
        Description: "Leader key for custom mappings",
        Scope:       ScopeGlobal,
        Tags:        []string{"input", "vim"},
    })

    r.Register(&Setting{
        Path:        "input.escapeDelay",
        Type:        TypeDuration,
        Default:     "100ms",
        Description: "Delay before Escape is recognized",
        Scope:       ScopeGlobal,
        Tags:        []string{"input", "vim"},
    })

    // UI settings
    r.Register(&Setting{
        Path:        "ui.theme",
        Type:        TypeString,
        Default:     "gruvbox-dark",
        Description: "Color theme name",
        Scope:       ScopeGlobal,
        Tags:        []string{"ui", "theme"},
    })

    r.Register(&Setting{
        Path:        "ui.fontSize",
        Type:        TypeInt,
        Default:     14,
        Description: "Font size in points",
        Scope:       ScopeGlobal,
        Minimum:     ptr(6.0),
        Maximum:     ptr(72.0),
        Tags:        []string{"ui", "font"},
    })

    r.Register(&Setting{
        Path:        "ui.fontFamily",
        Type:        TypeString,
        Default:     "monospace",
        Description: "Font family name",
        Scope:       ScopeGlobal,
        Tags:        []string{"ui", "font"},
    })

    r.Register(&Setting{
        Path:        "ui.statusline.enabled",
        Type:        TypeBool,
        Default:     true,
        Description: "Show the status line",
        Scope:       ScopeGlobal,
        Tags:        []string{"ui", "statusline"},
    })

    r.Register(&Setting{
        Path:        "ui.statusline.showMode",
        Type:        TypeBool,
        Default:     true,
        Description: "Show current mode in status line",
        Scope:       ScopeGlobal,
        Tags:        []string{"ui", "statusline"},
    })

    // Project settings
    r.Register(&Setting{
        Path:        "project.exclude",
        Type:        TypeArray,
        Default:     []any{"**/.git", "**/node_modules", "**/.venv", "**/target"},
        Description: "Glob patterns to exclude from project",
        Scope:       ScopeWorkspace,
        Tags:        []string{"project", "search"},
    })

    r.Register(&Setting{
        Path:        "project.searchExclude",
        Type:        TypeArray,
        Default:     []any{"**/*.min.js", "**/*.map"},
        Description: "Additional excludes for search",
        Scope:       ScopeWorkspace,
        Tags:        []string{"project", "search"},
    })

    // LSP settings
    r.Register(&Setting{
        Path:        "lsp.enabled",
        Type:        TypeBool,
        Default:     true,
        Description: "Enable Language Server Protocol support",
        Scope:       ScopeGlobal,
        Tags:        []string{"lsp"},
    })

    r.Register(&Setting{
        Path:        "lsp.diagnostics.enabled",
        Type:        TypeBool,
        Default:     true,
        Description: "Show LSP diagnostics",
        Scope:       ScopeAll,
        Tags:        []string{"lsp", "diagnostics"},
    })

    r.Register(&Setting{
        Path:        "lsp.completion.autoTrigger",
        Type:        TypeBool,
        Default:     true,
        Description: "Automatically show completions",
        Scope:       ScopeAll,
        Tags:        []string{"lsp", "completion"},
    })

    // Logging/Debug settings
    r.Register(&Setting{
        Path:        "logging.level",
        Type:        TypeEnum,
        Default:     "info",
        Description: "Logging level",
        Scope:       ScopeGlobal,
        Enum:        []any{"trace", "debug", "info", "warn", "error"},
        Tags:        []string{"logging"},
    })

    r.Register(&Setting{
        Path:        "logging.file",
        Type:        TypeString,
        Default:     "",
        Description: "Log file path (empty for stderr)",
        Scope:       ScopeGlobal,
        Tags:        []string{"logging"},
    })
}

// Helper function
func ptr(f float64) *float64 {
    return &f
}
```

### 8.2 Type-Safe Accessors

```go
// internal/config/registry/accessor.go

// Accessor provides type-safe access to configuration values.
type Accessor struct {
    registry *SettingsRegistry
}

// NewAccessor creates a type-safe accessor.
func NewAccessor(registry *SettingsRegistry) *Accessor {
    return &Accessor{registry: registry}
}

// String returns a string setting.
func (a *Accessor) String(path string) string {
    val, ok := a.registry.GetValue(path)
    if !ok {
        if s := a.registry.Get(path); s != nil {
            if def, ok := s.Default.(string); ok {
                return def
            }
        }
        return ""
    }
    if str, ok := val.(string); ok {
        return str
    }
    return fmt.Sprintf("%v", val)
}

// Int returns an integer setting.
func (a *Accessor) Int(path string) int {
    val, ok := a.registry.GetValue(path)
    if !ok {
        if s := a.registry.Get(path); s != nil {
            switch def := s.Default.(type) {
            case int:
                return def
            case int64:
                return int(def)
            case float64:
                return int(def)
            }
        }
        return 0
    }
    switch v := val.(type) {
    case int:
        return v
    case int64:
        return int(v)
    case float64:
        return int(v)
    }
    return 0
}

// Int64 returns an int64 setting.
func (a *Accessor) Int64(path string) int64 {
    val, ok := a.registry.GetValue(path)
    if !ok {
        if s := a.registry.Get(path); s != nil {
            switch def := s.Default.(type) {
            case int:
                return int64(def)
            case int64:
                return def
            case float64:
                return int64(def)
            }
        }
        return 0
    }
    switch v := val.(type) {
    case int:
        return int64(v)
    case int64:
        return v
    case float64:
        return int64(v)
    }
    return 0
}

// Float64 returns a float64 setting.
func (a *Accessor) Float64(path string) float64 {
    val, ok := a.registry.GetValue(path)
    if !ok {
        if s := a.registry.Get(path); s != nil {
            switch def := s.Default.(type) {
            case int:
                return float64(def)
            case int64:
                return float64(def)
            case float64:
                return def
            }
        }
        return 0
    }
    switch v := val.(type) {
    case int:
        return float64(v)
    case int64:
        return float64(v)
    case float64:
        return v
    }
    return 0
}

// Bool returns a boolean setting.
func (a *Accessor) Bool(path string) bool {
    val, ok := a.registry.GetValue(path)
    if !ok {
        if s := a.registry.Get(path); s != nil {
            if def, ok := s.Default.(bool); ok {
                return def
            }
        }
        return false
    }
    if b, ok := val.(bool); ok {
        return b
    }
    return false
}

// Duration returns a duration setting.
func (a *Accessor) Duration(path string) time.Duration {
    val, ok := a.registry.GetValue(path)
    if !ok {
        if s := a.registry.Get(path); s != nil {
            switch def := s.Default.(type) {
            case string:
                if d, err := time.ParseDuration(def); err == nil {
                    return d
                }
            case time.Duration:
                return def
            }
        }
        return 0
    }
    switch v := val.(type) {
    case string:
        if d, err := time.ParseDuration(v); err == nil {
            return d
        }
    case time.Duration:
        return v
    }
    return 0
}

// StringSlice returns a string slice setting.
func (a *Accessor) StringSlice(path string) []string {
    val, ok := a.registry.GetValue(path)
    if !ok {
        if s := a.registry.Get(path); s != nil {
            if def, ok := s.Default.([]any); ok {
                return anySliceToStrings(def)
            }
        }
        return nil
    }
    switch v := val.(type) {
    case []string:
        return v
    case []any:
        return anySliceToStrings(v)
    }
    return nil
}

func anySliceToStrings(slice []any) []string {
    result := make([]string, len(slice))
    for i, v := range slice {
        result[i] = fmt.Sprintf("%v", v)
    }
    return result
}
```

---

## 9. Watch and Live Reload

### 9.1 Config Watcher

```go
// internal/config/watcher/watcher.go

// ConfigWatcher watches configuration files for changes.
type ConfigWatcher struct {
    watcher   *fsnotify.Watcher
    paths     map[string]bool // Watched paths
    debounce  time.Duration
    callbacks []func(path string)

    mu        sync.Mutex
    pending   map[string]time.Time
    debouncer *time.Timer

    done chan struct{}
}

// NewConfigWatcher creates a new configuration watcher.
func NewConfigWatcher(debounce time.Duration) (*ConfigWatcher, error) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, fmt.Errorf("creating fsnotify watcher: %w", err)
    }

    return &ConfigWatcher{
        watcher:  watcher,
        paths:    make(map[string]bool),
        debounce: debounce,
        pending:  make(map[string]time.Time),
        done:     make(chan struct{}),
    }, nil
}

// Watch adds a path to be watched.
func (w *ConfigWatcher) Watch(path string) error {
    w.mu.Lock()
    defer w.mu.Unlock()

    if w.paths[path] {
        return nil // Already watching
    }

    // Ensure directory exists
    dir := filepath.Dir(path)
    if _, err := os.Stat(dir); os.IsNotExist(err) {
        // Create directory so we can watch it
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("creating config directory: %w", err)
        }
    }

    // Watch the directory (fsnotify can't watch non-existent files)
    if err := w.watcher.Add(dir); err != nil {
        return fmt.Errorf("watching directory %s: %w", dir, err)
    }

    w.paths[path] = true
    return nil
}

// Unwatch removes a path from watching.
func (w *ConfigWatcher) Unwatch(path string) {
    w.mu.Lock()
    defer w.mu.Unlock()

    delete(w.paths, path)

    // Check if any other paths share this directory
    dir := filepath.Dir(path)
    for p := range w.paths {
        if filepath.Dir(p) == dir {
            return // Still need to watch this directory
        }
    }

    _ = w.watcher.Remove(dir)
}

// OnChange registers a callback for file changes.
func (w *ConfigWatcher) OnChange(callback func(path string)) {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.callbacks = append(w.callbacks, callback)
}

// Start begins watching for changes.
func (w *ConfigWatcher) Start() {
    go w.watchLoop()
}

// Stop stops the watcher.
func (w *ConfigWatcher) Stop() {
    close(w.done)
    w.watcher.Close()
}

// watchLoop processes file system events.
func (w *ConfigWatcher) watchLoop() {
    for {
        select {
        case event, ok := <-w.watcher.Events:
            if !ok {
                return
            }
            w.handleEvent(event)

        case err, ok := <-w.watcher.Errors:
            if !ok {
                return
            }
            // Log error but continue watching
            log.Printf("config watcher error: %v", err)

        case <-w.done:
            return
        }
    }
}

// handleEvent processes a single file system event.
func (w *ConfigWatcher) handleEvent(event fsnotify.Event) {
    // Only care about writes and creates
    if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
        return
    }

    w.mu.Lock()

    // Check if this is a watched file
    if !w.paths[event.Name] {
        w.mu.Unlock()
        return
    }

    // Record pending change
    w.pending[event.Name] = time.Now()

    // Reset debounce timer
    if w.debouncer != nil {
        w.debouncer.Stop()
    }

    w.debouncer = time.AfterFunc(w.debounce, func() {
        w.flushPending()
    })

    w.mu.Unlock()
}

// flushPending processes all pending changes.
func (w *ConfigWatcher) flushPending() {
    w.mu.Lock()
    pending := w.pending
    w.pending = make(map[string]time.Time)
    callbacks := w.callbacks
    w.mu.Unlock()

    for path := range pending {
        for _, cb := range callbacks {
            cb(path)
        }
    }
}
```

### 9.2 Live Reload Integration

```go
// internal/config/config.go (partial)

// setupWatcher sets up file watching for live reload.
func (c *ConfigManager) setupWatcher() error {
    watcher, err := NewConfigWatcher(100 * time.Millisecond)
    if err != nil {
        return err
    }

    c.watcher = watcher

    // Watch all config files
    for _, path := range c.configPaths() {
        if err := watcher.Watch(path); err != nil {
            log.Printf("warning: cannot watch %s: %v", path, err)
        }
    }

    // Handle changes
    watcher.OnChange(func(path string) {
        c.handleConfigChange(path)
    })

    watcher.Start()
    return nil
}

// handleConfigChange processes a configuration file change.
func (c *ConfigManager) handleConfigChange(path string) {
    log.Printf("config file changed: %s", path)

    // Load the changed file
    loader := c.loaderForPath(path)
    data, err := loader.Load(path)
    if err != nil {
        log.Printf("error loading changed config %s: %v", path, err)
        c.notifyError(path, err)
        return
    }

    // Validate
    errors := c.validator.Validate(data)
    if len(errors) > 0 {
        log.Printf("validation errors in %s:", path)
        for _, e := range errors {
            log.Printf("  %s", e)
        }
        c.notifyError(path, &ValidationErrors{Errors: errors})
        return
    }

    // Determine which layer this file belongs to
    layer := c.layerForPath(path)

    // Update the layer
    oldData := layer.Data
    layer.Data = data
    layer.ModTime = time.Now()

    // Re-merge all layers
    merged := c.layers.Merge()

    // Find what changed
    changes := diffConfigs(oldData, data)

    // Update registry values
    for _, change := range changes {
        c.registry.SetValue(change.Path, change.NewValue)
    }

    // Notify about section changes
    changedSections := make(map[string][]SettingChange)
    for _, change := range changes {
        section := getSection(change.Path)
        changedSections[section] = append(changedSections[section], change)
    }

    for section, sectionChanges := range changedSections {
        c.notifier.NotifySection(section, sectionChanges)
    }

    log.Printf("config reloaded: %d settings changed", len(changes))
}
```

---

## 10. Language and Filetype Settings

### 10.1 Language Configuration

```go
// internal/config/sections/language.go

// LanguageConfig holds per-language settings.
type LanguageConfig struct {
    // Identity
    ID          string   `toml:"id"`          // Language identifier (e.g., "go", "python")
    Extensions  []string `toml:"extensions"`  // File extensions
    Filenames   []string `toml:"filenames"`   // Exact filenames (e.g., "Makefile")

    // Editor overrides
    TabSize      *int    `toml:"tabSize"`
    InsertSpaces *bool   `toml:"insertSpaces"`
    WordWrap     *string `toml:"wordWrap"`

    // Formatting
    FormatOnSave        *bool   `toml:"formatOnSave"`
    DefaultFormatter    string  `toml:"defaultFormatter"`
    FormatOnPaste       *bool   `toml:"formatOnPaste"`

    // Comments
    LineCommentPrefix   string   `toml:"lineCommentPrefix"`
    BlockCommentStart   string   `toml:"blockCommentStart"`
    BlockCommentEnd     string   `toml:"blockCommentEnd"`

    // Indentation
    IndentationRules    *IndentationRules `toml:"indentationRules"`

    // Brackets
    Brackets            []BracketPair `toml:"brackets"`
    AutoClosingPairs    []BracketPair `toml:"autoClosingPairs"`

    // LSP
    LSPServer           string            `toml:"lspServer"`
    LSPSettings         map[string]any    `toml:"lspSettings"`
}

// IndentationRules configures auto-indentation.
type IndentationRules struct {
    IncreaseIndentPattern string `toml:"increaseIndentPattern"`
    DecreaseIndentPattern string `toml:"decreaseIndentPattern"`
    IndentNextLinePattern string `toml:"indentNextLinePattern"`
}

// BracketPair defines a bracket pair.
type BracketPair struct {
    Open  string `toml:"open"`
    Close string `toml:"close"`
}

// LanguageRegistry manages language configurations.
type LanguageRegistry struct {
    mu        sync.RWMutex
    languages map[string]*LanguageConfig
    byExt     map[string]string // Extension -> language ID
    byName    map[string]string // Filename -> language ID
}

// NewLanguageRegistry creates a new language registry.
func NewLanguageRegistry() *LanguageRegistry {
    r := &LanguageRegistry{
        languages: make(map[string]*LanguageConfig),
        byExt:     make(map[string]string),
        byName:    make(map[string]string),
    }

    // Register built-in languages
    r.registerBuiltinLanguages()

    return r
}

// Register adds a language configuration.
func (r *LanguageRegistry) Register(lang *LanguageConfig) {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.languages[lang.ID] = lang

    for _, ext := range lang.Extensions {
        r.byExt[ext] = lang.ID
    }

    for _, name := range lang.Filenames {
        r.byName[name] = lang.ID
    }
}

// Get retrieves a language configuration by ID.
func (r *LanguageRegistry) Get(id string) *LanguageConfig {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.languages[id]
}

// DetectLanguage determines the language for a file path.
func (r *LanguageRegistry) DetectLanguage(path string) string {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Check filename first
    name := filepath.Base(path)
    if lang, ok := r.byName[name]; ok {
        return lang
    }

    // Check extension
    ext := filepath.Ext(path)
    if ext != "" {
        ext = strings.TrimPrefix(ext, ".")
        if lang, ok := r.byExt[ext]; ok {
            return lang
        }
    }

    return "plaintext"
}

// EffectiveConfig returns settings for a language, merging with defaults.
func (r *LanguageRegistry) EffectiveConfig(langID string, defaults *EditorConfig) *EditorConfig {
    lang := r.Get(langID)
    if lang == nil {
        return defaults
    }

    // Clone defaults and apply overrides
    result := *defaults

    if lang.TabSize != nil {
        result.TabSize = *lang.TabSize
    }
    if lang.InsertSpaces != nil {
        result.InsertSpaces = *lang.InsertSpaces
    }
    if lang.WordWrap != nil {
        result.WordWrap = *lang.WordWrap
    }

    return &result
}

// registerBuiltinLanguages registers default language configurations.
func (r *LanguageRegistry) registerBuiltinLanguages() {
    r.Register(&LanguageConfig{
        ID:                "go",
        Extensions:        []string{"go"},
        TabSize:           ptr(4),
        InsertSpaces:      ptrBool(false), // Go uses tabs
        LineCommentPrefix: "//",
        BlockCommentStart: "/*",
        BlockCommentEnd:   "*/",
        Brackets: []BracketPair{
            {Open: "{", Close: "}"},
            {Open: "[", Close: "]"},
            {Open: "(", Close: ")"},
        },
        LSPServer: "gopls",
    })

    r.Register(&LanguageConfig{
        ID:                "python",
        Extensions:        []string{"py", "pyw", "pyi"},
        TabSize:           ptr(4),
        InsertSpaces:      ptrBool(true),
        LineCommentPrefix: "#",
        Brackets: []BracketPair{
            {Open: "{", Close: "}"},
            {Open: "[", Close: "]"},
            {Open: "(", Close: ")"},
        },
        LSPServer: "pyright",
    })

    r.Register(&LanguageConfig{
        ID:                "javascript",
        Extensions:        []string{"js", "mjs", "cjs"},
        TabSize:           ptr(2),
        InsertSpaces:      ptrBool(true),
        LineCommentPrefix: "//",
        BlockCommentStart: "/*",
        BlockCommentEnd:   "*/",
        Brackets: []BracketPair{
            {Open: "{", Close: "}"},
            {Open: "[", Close: "]"},
            {Open: "(", Close: ")"},
        },
        LSPServer: "typescript-language-server",
    })

    r.Register(&LanguageConfig{
        ID:                "typescript",
        Extensions:        []string{"ts", "tsx"},
        TabSize:           ptr(2),
        InsertSpaces:      ptrBool(true),
        LineCommentPrefix: "//",
        BlockCommentStart: "/*",
        BlockCommentEnd:   "*/",
        Brackets: []BracketPair{
            {Open: "{", Close: "}"},
            {Open: "[", Close: "]"},
            {Open: "(", Close: ")"},
            {Open: "<", Close: ">"},
        },
        LSPServer: "typescript-language-server",
    })

    r.Register(&LanguageConfig{
        ID:                "rust",
        Extensions:        []string{"rs"},
        TabSize:           ptr(4),
        InsertSpaces:      ptrBool(true),
        LineCommentPrefix: "//",
        BlockCommentStart: "/*",
        BlockCommentEnd:   "*/",
        Brackets: []BracketPair{
            {Open: "{", Close: "}"},
            {Open: "[", Close: "]"},
            {Open: "(", Close: ")"},
            {Open: "<", Close: ">"},
        },
        LSPServer: "rust-analyzer",
    })

    r.Register(&LanguageConfig{
        ID:         "makefile",
        Filenames:  []string{"Makefile", "makefile", "GNUmakefile"},
        TabSize:    ptr(4),
        InsertSpaces: ptrBool(false), // Make requires tabs
        LineCommentPrefix: "#",
    })

    r.Register(&LanguageConfig{
        ID:         "yaml",
        Extensions: []string{"yaml", "yml"},
        TabSize:    ptr(2),
        InsertSpaces: ptrBool(true),
        LineCommentPrefix: "#",
        LSPServer: "yaml-language-server",
    })

    r.Register(&LanguageConfig{
        ID:         "json",
        Extensions: []string{"json", "jsonc"},
        TabSize:    ptr(2),
        InsertSpaces: ptrBool(true),
    })

    r.Register(&LanguageConfig{
        ID:         "toml",
        Extensions: []string{"toml"},
        TabSize:    ptr(2),
        InsertSpaces: ptrBool(true),
        LineCommentPrefix: "#",
    })

    r.Register(&LanguageConfig{
        ID:         "markdown",
        Extensions: []string{"md", "markdown"},
        TabSize:    ptr(2),
        InsertSpaces: ptrBool(true),
        WordWrap:   ptrString("on"),
    })
}

func ptrBool(b bool) *bool    { return &b }
func ptrString(s string) *string { return &s }
func ptr(i int) *int          { return &i }
```

---

## 11. Plugin Configuration

### 11.1 Plugin Config Section

```go
// internal/config/sections/plugin.go

// PluginConfig holds configuration for a specific plugin.
type PluginConfig struct {
    // Enabled determines if the plugin should be loaded
    Enabled bool `toml:"enabled"`

    // Settings holds plugin-specific settings
    Settings map[string]any `toml:"settings"`
}

// PluginConfigManager manages plugin configurations.
type PluginConfigManager struct {
    mu       sync.RWMutex
    configs  map[string]*PluginConfig // Plugin name -> config
    schemas  map[string]*Schema       // Plugin name -> settings schema
    registry *SettingsRegistry
}

// NewPluginConfigManager creates a plugin configuration manager.
func NewPluginConfigManager(registry *SettingsRegistry) *PluginConfigManager {
    return &PluginConfigManager{
        configs:  make(map[string]*PluginConfig),
        schemas:  make(map[string]*Schema),
        registry: registry,
    }
}

// RegisterPluginSchema registers a plugin's configuration schema.
// This is called by the plugin system when loading a plugin.
func (m *PluginConfigManager) RegisterPluginSchema(pluginName string, schema *Schema) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.schemas[pluginName] = schema

    // Register settings with the main registry
    for path, prop := range schema.Properties {
        fullPath := "plugins." + pluginName + "." + path
        m.registry.Register(&Setting{
            Path:        fullPath,
            Type:        schemaTypeToSettingType(prop.Type),
            Default:     prop.Default,
            Description: prop.Description,
            Scope:       ScopeAll,
            Enum:        prop.Enum,
        })
    }
}

// GetPluginConfig returns configuration for a plugin.
func (m *PluginConfigManager) GetPluginConfig(pluginName string) *PluginConfig {
    m.mu.RLock()
    defer m.mu.RUnlock()

    if config, ok := m.configs[pluginName]; ok {
        return config
    }

    // Return default config
    return &PluginConfig{
        Enabled:  true,
        Settings: make(map[string]any),
    }
}

// SetPluginSetting sets a plugin setting.
func (m *PluginConfigManager) SetPluginSetting(pluginName, setting string, value any) error {
    fullPath := "plugins." + pluginName + "." + setting
    return m.registry.SetValue(fullPath, value)
}

// GetPluginSetting gets a plugin setting.
func (m *PluginConfigManager) GetPluginSetting(pluginName, setting string) (any, bool) {
    fullPath := "plugins." + pluginName + "." + setting
    return m.registry.GetValue(fullPath)
}

// OnPluginSettingChange subscribes to changes in a plugin's settings.
func (m *PluginConfigManager) OnPluginSettingChange(pluginName string, handler func(path string, oldValue, newValue any)) UnsubscribeFunc {
    prefix := "plugins." + pluginName + "."
    return m.registry.notifier.SubscribePrefix(prefix, handler)
}

// ValidatePluginConfig validates plugin configuration against schema.
func (m *PluginConfigManager) ValidatePluginConfig(pluginName string, config map[string]any) []ValidationError {
    m.mu.RLock()
    schema, ok := m.schemas[pluginName]
    m.mu.RUnlock()

    if !ok {
        return nil // No schema, no validation
    }

    validator := NewSchemaValidator(schema, nil)
    return validator.Validate(config)
}
```

---

## 12. Integration with Other Modules

### 12.1 Integration Interface

```go
// internal/config/integration.go

// ConfigSystem provides the unified configuration interface.
type ConfigSystem struct {
    // Core components
    registry  *SettingsRegistry
    layers    *LayerManager
    validator *SchemaValidator
    watcher   *ConfigWatcher
    notifier  *ChangeNotifier

    // Specialized managers
    languages *LanguageRegistry
    plugins   *PluginConfigManager
    keymaps   *KeymapManager

    // Configuration
    paths ConfigPaths
}

// ConfigPaths defines standard configuration file locations.
type ConfigPaths struct {
    ConfigDir    string // ~/.config/keystorm/
    DataDir      string // ~/.local/share/keystorm/
    CacheDir     string // ~/.cache/keystorm/
    UserConfig   string // ~/.config/keystorm/config.toml
    UserSettings string // ~/.config/keystorm/settings.toml
    UserKeymaps  string // ~/.config/keystorm/keymaps.toml
}

// NewConfigSystem creates and initializes the configuration system.
func NewConfigSystem(opts ...ConfigOption) (*ConfigSystem, error) {
    paths := defaultPaths()

    for _, opt := range opts {
        opt(&paths)
    }

    cs := &ConfigSystem{
        registry:  NewSettingsRegistry(),
        layers:    NewLayerManager(),
        notifier:  NewChangeNotifier(),
        languages: NewLanguageRegistry(),
        paths:     paths,
    }

    // Load default schema
    schema, err := LoadSchema()
    if err != nil {
        return nil, fmt.Errorf("loading schema: %w", err)
    }

    cs.validator = NewSchemaValidator(schema, cs.registry)
    cs.plugins = NewPluginConfigManager(cs.registry)
    cs.keymaps = NewKeymapManager()

    // Load configuration layers
    if err := cs.loadAllLayers(); err != nil {
        return nil, fmt.Errorf("loading configuration: %w", err)
    }

    // Setup file watcher
    if err := cs.setupWatcher(); err != nil {
        // Non-fatal, log and continue
        log.Printf("warning: config watcher failed: %v", err)
    }

    return cs, nil
}

// loadAllLayers loads all configuration layers in order.
func (cs *ConfigSystem) loadAllLayers() error {
    // 1. Built-in defaults
    defaults := DefaultConfig()
    cs.layers.Add(&Layer{
        Name:     "defaults",
        Priority: 0,
        Source:   SourceBuiltin,
        Data:     defaults,
        ReadOnly: true,
    })

    // 2. User global settings
    userSettings, _ := NewTOMLLoader(nil).Load(cs.paths.UserSettings)
    if userSettings != nil {
        cs.layers.Add(&Layer{
            Name:     "user-settings",
            Priority: 100,
            Source:   SourceUserGlobal,
            Path:     cs.paths.UserSettings,
            Data:     userSettings,
        })
    }

    // 3. User keymaps
    keymapData, _ := NewTOMLLoader(nil).Load(cs.paths.UserKeymaps)
    if keymapData != nil {
        cs.layers.Add(&Layer{
            Name:     "user-keymaps",
            Priority: 110,
            Source:   SourceUserGlobal,
            Path:     cs.paths.UserKeymaps,
            Data:     keymapData,
        })
    }

    // 4. Environment variables
    envData, _ := NewEnvLoader("KEYSTORM_").Load()
    if len(envData) > 0 {
        cs.layers.Add(&Layer{
            Name:     "environment",
            Priority: 300,
            Source:   SourceEnv,
            Data:     envData,
            ReadOnly: true,
        })
    }

    // Merge and apply to registry
    merged := cs.layers.Merge()
    cs.applyToRegistry(merged)

    return nil
}

// LoadWorkspace loads workspace-specific configuration.
func (cs *ConfigSystem) LoadWorkspace(workspacePath string) error {
    wsConfigPath := filepath.Join(workspacePath, ".keystorm", "config.toml")

    wsConfig, err := NewTOMLLoader(nil).Load(wsConfigPath)
    if err != nil {
        return nil // No workspace config is fine
    }

    // Validate
    errors := cs.validator.Validate(wsConfig)
    if len(errors) > 0 {
        return &ValidationErrors{Errors: errors}
    }

    // Add as layer
    cs.layers.Add(&Layer{
        Name:     "workspace",
        Priority: 200,
        Source:   SourceWorkspace,
        Path:     wsConfigPath,
        Data:     wsConfig,
    })

    // Re-merge and apply
    merged := cs.layers.Merge()
    cs.applyToRegistry(merged)

    // Watch workspace config
    if cs.watcher != nil {
        cs.watcher.Watch(wsConfigPath)
    }

    return nil
}

// applyToRegistry updates all registry values from merged config.
func (cs *ConfigSystem) applyToRegistry(merged map[string]any) {
    paths := flattenConfig(merged)
    for path, value := range paths {
        cs.registry.SetValue(path, value)
    }
}

// Config API methods

// Get returns a setting value by path.
func (cs *ConfigSystem) Get(path string) (any, error) {
    val, ok := cs.registry.GetValue(path)
    if !ok {
        return nil, fmt.Errorf("setting not found: %s", path)
    }
    return val, nil
}

// GetString returns a string setting.
func (cs *ConfigSystem) GetString(path string) string {
    return NewAccessor(cs.registry).String(path)
}

// GetInt returns an integer setting.
func (cs *ConfigSystem) GetInt(path string) int {
    return NewAccessor(cs.registry).Int(path)
}

// GetBool returns a boolean setting.
func (cs *ConfigSystem) GetBool(path string) bool {
    return NewAccessor(cs.registry).Bool(path)
}

// GetDuration returns a duration setting.
func (cs *ConfigSystem) GetDuration(path string) time.Duration {
    return NewAccessor(cs.registry).Duration(path)
}

// Set updates a setting value.
func (cs *ConfigSystem) Set(path string, value any) error {
    // Validate
    if err := cs.validator.ValidateSetting(path, value); err != nil {
        return err
    }

    return cs.registry.SetValue(path, value)
}

// OnChange subscribes to setting changes.
func (cs *ConfigSystem) OnChange(path string, handler ChangeHandler) UnsubscribeFunc {
    return cs.registry.OnChange(path, handler)
}

// OnSectionChange subscribes to any change in a section.
func (cs *ConfigSystem) OnSectionChange(section string, handler SectionChangeHandler) UnsubscribeFunc {
    return cs.notifier.SubscribeSection(section, handler)
}

// Editor returns the editor configuration.
func (cs *ConfigSystem) Editor() *EditorConfig {
    return &EditorConfig{
        TabSize:        cs.GetInt("editor.tabSize"),
        InsertSpaces:   cs.GetBool("editor.insertSpaces"),
        LineNumbers:    cs.GetString("editor.lineNumbers"),
        WordWrap:       cs.GetString("editor.wordWrap"),
        WordWrapColumn: cs.GetInt("editor.wordWrapColumn"),
        CursorStyle:    cs.GetString("editor.cursorStyle"),
        CursorBlink:    cs.GetBool("editor.cursorBlink"),
        Scrolloff:      cs.GetInt("editor.scrolloff"),
        AutoSave:       cs.GetString("editor.autoSave"),
        AutoSaveDelay:  cs.GetInt("editor.autoSaveDelay"),
    }
}

// Input returns the input configuration.
func (cs *ConfigSystem) Input() *InputConfig {
    return &InputConfig{
        KeyTimeout:  cs.GetDuration("input.keyTimeout"),
        LeaderKey:   cs.GetString("input.leaderKey"),
        EscapeDelay: cs.GetDuration("input.escapeDelay"),
    }
}

// UI returns the UI configuration.
func (cs *ConfigSystem) UI() *UIConfig {
    return &UIConfig{
        Theme:      cs.GetString("ui.theme"),
        FontSize:   cs.GetInt("ui.fontSize"),
        FontFamily: cs.GetString("ui.fontFamily"),
    }
}

// Language returns configuration for a specific language.
func (cs *ConfigSystem) Language(lang string) *LanguageConfig {
    return cs.languages.Get(lang)
}

// LanguageForFile returns configuration for a file.
func (cs *ConfigSystem) LanguageForFile(path string) *LanguageConfig {
    langID := cs.languages.DetectLanguage(path)
    return cs.languages.Get(langID)
}

// Plugin returns plugin configuration.
func (cs *ConfigSystem) Plugin(name string) map[string]any {
    config := cs.plugins.GetPluginConfig(name)
    return config.Settings
}

// Keymaps returns the keymap manager.
func (cs *ConfigSystem) Keymaps() *KeymapManager {
    return cs.keymaps
}

// Reload reloads all configuration from disk.
func (cs *ConfigSystem) Reload() error {
    return cs.loadAllLayers()
}

// Save persists user settings to disk.
func (cs *ConfigSystem) Save() error {
    // Extract user layer
    layer := cs.layers.GetByName("user-settings")
    if layer == nil {
        return nil
    }

    // Encode to TOML
    data, err := toml.Marshal(layer.Data)
    if err != nil {
        return fmt.Errorf("encoding config: %w", err)
    }

    // Write to file
    return os.WriteFile(layer.Path, data, 0644)
}

// Close shuts down the configuration system.
func (cs *ConfigSystem) Close() {
    if cs.watcher != nil {
        cs.watcher.Stop()
    }
}
```

### 12.2 Plugin API Integration

```go
// internal/config/api.go

// PluginConfigAPI implements the ks.config API for plugins.
type PluginConfigAPI struct {
    config     *ConfigSystem
    pluginName string
}

// NewPluginConfigAPI creates a config API for a plugin.
func NewPluginConfigAPI(config *ConfigSystem, pluginName string) *PluginConfigAPI {
    return &PluginConfigAPI{
        config:     config,
        pluginName: pluginName,
    }
}

// Get retrieves a plugin setting.
func (api *PluginConfigAPI) Get(key string) any {
    val, _ := api.config.plugins.GetPluginSetting(api.pluginName, key)
    return val
}

// Set updates a plugin setting.
func (api *PluginConfigAPI) Set(key string, value any) error {
    return api.config.plugins.SetPluginSetting(api.pluginName, key, value)
}

// OnChange subscribes to plugin setting changes.
func (api *PluginConfigAPI) OnChange(key string, handler func(oldValue, newValue any)) UnsubscribeFunc {
    return api.config.plugins.OnPluginSettingChange(api.pluginName, func(path string, old, new any) {
        // Extract key from path (plugins.name.key -> key)
        prefix := "plugins." + api.pluginName + "."
        if strings.HasPrefix(path, prefix) {
            actualKey := strings.TrimPrefix(path, prefix)
            if actualKey == key || key == "*" {
                handler(old, new)
            }
        }
    })
}

// GetEditorSetting retrieves a global editor setting (read-only for plugins).
func (api *PluginConfigAPI) GetEditorSetting(path string) any {
    val, _ := api.config.Get(path)
    return val
}
```

---

## 13. Implementation Phases

### Phase 1: Core Infrastructure

**Goal**: Establish foundational types and basic loading.

**Tasks**:
1. `registry/setting.go` - Setting definition type
2. `registry/registry.go` - SettingsRegistry with built-in settings
3. `registry/accessor.go` - Type-safe accessors
4. `loader/toml.go` - TOML loader
5. `errors.go` - Error types
6. Unit tests for core types

**Success Criteria**:
- Can register and retrieve settings
- Can load TOML config file
- Type-safe access works

### Phase 2: Layer Management

**Goal**: Implement configuration layering and merging.

**Tasks**:
1. `layer/layer.go` - Layer type
2. `layer/manager.go` - LayerManager
3. `layer/merge.go` - Deep merge logic
4. `layer/precedence.go` - Precedence rules
5. `loader/env.go` - Environment variable loader
6. Integration tests for merging

**Success Criteria**:
- Multiple layers merge correctly
- Higher priority overrides lower
- Environment variables work

### Phase 3: Schema and Validation

**Goal**: Implement configuration validation.

**Tasks**:
1. `schema/schema.go` - Schema types
2. `schema/validator.go` - Validation logic
3. `schema/types.go` - Type definitions
4. `schema/errors.go` - Validation errors
5. Embedded JSON schema file
6. Validation tests

**Success Criteria**:
- Invalid config produces clear errors
- Type mismatches detected
- Enum validation works
- Range validation works

### Phase 4: File Watching and Reload

**Goal**: Implement live configuration reload.

**Tasks**:
1. `watcher/watcher.go` - ConfigWatcher
2. `watcher/debounce.go` - Debouncing logic
3. `notify/notifier.go` - ChangeNotifier
4. `notify/subscriber.go` - Subscriber management
5. Integration with ConfigSystem
6. Live reload tests

**Success Criteria**:
- File changes detected
- Changes debounced properly
- Subscribers notified
- Invalid changes rejected

### Phase 5: Language Configuration

**Goal**: Implement per-language settings.

**Tasks**:
1. `sections/language.go` - LanguageConfig
2. Language detection by extension/filename
3. Language-specific setting overrides
4. Built-in language configurations
5. Integration with editor settings

**Success Criteria**:
- Go files use tabs
- Python files use 4 spaces
- Language detected from extension
- Overrides merge correctly

### Phase 6: Plugin Configuration

**Goal**: Support plugin settings.

**Tasks**:
1. `sections/plugin.go` - PluginConfig
2. Plugin schema registration
3. Plugin settings access API
4. Plugin setting change notifications
5. Integration with plugin system

**Success Criteria**:
- Plugins can register schemas
- Plugin settings validated
- Changes notify plugins
- Plugin API works

### Phase 7: Keymap Configuration

**Goal**: Load and manage keymaps.

**Tasks**:
1. `sections/keymap.go` - KeymapConfig
2. Keymap file loading
3. Mode-specific keymap access
4. User keymap overrides
5. Integration with input system

**Success Criteria**:
- Default keymaps load
- User keymaps override
- Mode-specific bindings work

### Phase 8: Integration and Polish

**Goal**: Full system integration.

**Tasks**:
1. `integration.go` - ConfigSystem facade
2. Complete typed section accessors
3. Config migration support
4. Performance optimization
5. Documentation
6. End-to-end tests

**Success Criteria**:
- All components integrated
- Clean public API
- < 50ms load time
- All tests passing

---

## 14. Testing Strategy

### 14.1 Unit Tests

```go
// registry/registry_test.go
func TestSettingsRegistry_Register(t *testing.T) {
    r := NewSettingsRegistry()

    err := r.Register(&Setting{
        Path:        "test.setting",
        Type:        TypeInt,
        Default:     42,
        Description: "Test setting",
    })
    require.NoError(t, err)

    s := r.Get("test.setting")
    require.NotNil(t, s)
    assert.Equal(t, 42, s.Default)
}

func TestSettingsRegistry_GetValue(t *testing.T) {
    r := NewSettingsRegistry()
    r.Register(&Setting{
        Path:    "editor.tabSize",
        Type:    TypeInt,
        Default: 4,
    })

    val, ok := r.GetValue("editor.tabSize")
    assert.True(t, ok)
    assert.Equal(t, 4, val)
}

// schema/validator_test.go
func TestValidator_TypeMismatch(t *testing.T) {
    validator := setupValidator(t)

    config := map[string]any{
        "editor": map[string]any{
            "tabSize": "not a number",
        },
    }

    errors := validator.Validate(config)
    require.Len(t, errors, 1)
    assert.Equal(t, ErrTypeMismatch, errors[0].Code)
}

func TestValidator_EnumValidation(t *testing.T) {
    validator := setupValidator(t)

    config := map[string]any{
        "editor": map[string]any{
            "lineNumbers": "invalid",
        },
    }

    errors := validator.Validate(config)
    require.Len(t, errors, 1)
    assert.Equal(t, ErrInvalidEnum, errors[0].Code)
}

// layer/manager_test.go
func TestLayerManager_Merge(t *testing.T) {
    m := NewLayerManager()

    m.Add(&Layer{
        Name:     "defaults",
        Priority: 0,
        Data: map[string]any{
            "editor": map[string]any{
                "tabSize": 4,
                "wordWrap": "off",
            },
        },
    })

    m.Add(&Layer{
        Name:     "user",
        Priority: 100,
        Data: map[string]any{
            "editor": map[string]any{
                "tabSize": 2, // Override
            },
        },
    })

    merged := m.Merge()
    editor := merged["editor"].(map[string]any)

    assert.Equal(t, 2, editor["tabSize"])     // Overridden
    assert.Equal(t, "off", editor["wordWrap"]) // From defaults
}
```

### 14.2 Integration Tests

```go
// integration_test.go
func TestConfigSystem_LoadAndAccess(t *testing.T) {
    // Create temp config dir
    tmpDir := t.TempDir()
    settingsPath := filepath.Join(tmpDir, "settings.toml")

    // Write test config
    err := os.WriteFile(settingsPath, []byte(`
[editor]
tabSize = 2
insertSpaces = true

[ui]
theme = "dracula"
`), 0644)
    require.NoError(t, err)

    // Create config system
    cs, err := NewConfigSystem(WithConfigDir(tmpDir))
    require.NoError(t, err)
    defer cs.Close()

    // Test access
    assert.Equal(t, 2, cs.GetInt("editor.tabSize"))
    assert.True(t, cs.GetBool("editor.insertSpaces"))
    assert.Equal(t, "dracula", cs.GetString("ui.theme"))

    // Test typed section
    editor := cs.Editor()
    assert.Equal(t, 2, editor.TabSize)
    assert.True(t, editor.InsertSpaces)
}

func TestConfigSystem_LiveReload(t *testing.T) {
    tmpDir := t.TempDir()
    settingsPath := filepath.Join(tmpDir, "settings.toml")

    // Initial config
    err := os.WriteFile(settingsPath, []byte(`
[editor]
tabSize = 4
`), 0644)
    require.NoError(t, err)

    cs, err := NewConfigSystem(WithConfigDir(tmpDir))
    require.NoError(t, err)
    defer cs.Close()

    // Subscribe to changes
    changed := make(chan bool, 1)
    cs.OnChange("editor.tabSize", func(path string, old, new any) {
        changed <- true
    })

    // Modify config
    time.Sleep(100 * time.Millisecond) // Let watcher start
    err = os.WriteFile(settingsPath, []byte(`
[editor]
tabSize = 2
`), 0644)
    require.NoError(t, err)

    // Wait for change notification
    select {
    case <-changed:
        assert.Equal(t, 2, cs.GetInt("editor.tabSize"))
    case <-time.After(2 * time.Second):
        t.Fatal("change notification timeout")
    }
}
```

### 14.3 Benchmark Tests

```go
// bench_test.go
func BenchmarkConfigGet(b *testing.B) {
    cs, _ := NewConfigSystem()
    defer cs.Close()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = cs.GetInt("editor.tabSize")
    }
}

func BenchmarkConfigLoad(b *testing.B) {
    tmpDir := b.TempDir()
    writeTestConfig(tmpDir)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cs, _ := NewConfigSystem(WithConfigDir(tmpDir))
        cs.Close()
    }
}

func BenchmarkLayerMerge(b *testing.B) {
    m := setupLayerManager()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = m.Merge()
    }
}
```

---

## 15. Performance Considerations

### 15.1 Caching Strategy

```go
// Cached accessor for hot paths
type CachedAccessor struct {
    registry *SettingsRegistry
    cache    sync.Map // path -> cachedValue
}

type cachedValue struct {
    value   any
    version int64
}

func (a *CachedAccessor) Int(path string) int {
    // Check cache
    if v, ok := a.cache.Load(path); ok {
        cv := v.(cachedValue)
        // Check if still valid
        if cv.version == a.registry.version {
            return cv.value.(int)
        }
    }

    // Cache miss, load from registry
    val := NewAccessor(a.registry).Int(path)
    a.cache.Store(path, cachedValue{
        value:   val,
        version: a.registry.version,
    })

    return val
}
```

### 15.2 Lazy Loading

```go
// Lazy section loading
type LazySection struct {
    loader func() any
    value  any
    loaded bool
    mu     sync.Mutex
}

func (s *LazySection) Get() any {
    s.mu.Lock()
    defer s.mu.Unlock()

    if !s.loaded {
        s.value = s.loader()
        s.loaded = true
    }

    return s.value
}
```

### 15.3 Performance Targets

| Operation | Target |
|-----------|--------|
| Setting lookup | < 100ns (cached), < 1μs (uncached) |
| Config load | < 50ms |
| Layer merge | < 10ms |
| Validation | < 10ms |
| File watch latency | < 100ms |
| Change notification | < 1ms |

---

## Dependencies

### Internal Dependencies

| Package | Purpose |
|---------|---------|
| `internal/event` | Event bus for config change events |
| `internal/project/vfs` | File system abstraction |

### External Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/BurntSushi/toml` | TOML parsing |
| `github.com/fsnotify/fsnotify` | File watching |

---

## Example Configuration Files

### ~/.config/keystorm/settings.toml

```toml
# Keystorm User Settings

[editor]
tabSize = 4
insertSpaces = true
lineNumbers = "relative"
wordWrap = "off"
cursorStyle = "block"
cursorBlink = true
scrolloff = 5

[input]
keyTimeout = "1s"
leaderKey = " "
escapeDelay = "50ms"

[ui]
theme = "gruvbox-dark"
fontSize = 14
fontFamily = "JetBrains Mono"

[ui.statusline]
enabled = true
showMode = true

[project]
exclude = ["**/.git", "**/node_modules", "**/.venv", "**/target", "**/dist"]

[lsp]
enabled = true

[lsp.diagnostics]
enabled = true

[logging]
level = "info"
```

### ~/.config/keystorm/keymaps.toml

```toml
# Custom Keymaps

[[keymaps]]
mode = "normal"
key = "<leader>w"
action = "file.save"
description = "Save file"

[[keymaps]]
mode = "normal"
key = "<leader>q"
action = "editor.quit"
description = "Quit"

[[keymaps]]
mode = "normal"
key = "<leader>ff"
action = "palette.findFiles"
description = "Find files"

[[keymaps]]
mode = "normal"
key = "<leader>fg"
action = "palette.liveGrep"
description = "Live grep"

[[keymaps]]
mode = "normal"
key = "gd"
action = "lsp.goToDefinition"
description = "Go to definition"

[[keymaps]]
mode = "normal"
key = "gr"
action = "lsp.findReferences"
description = "Find references"

[[keymaps]]
mode = "insert"
key = "jk"
action = "mode.normal"
description = "Exit insert mode"
```

### .keystorm/config.toml (Project)

```toml
# Project-specific settings

[editor]
tabSize = 2

[project]
exclude = ["**/vendor", "**/build"]
searchExclude = ["**/*.generated.go"]

[languages.go]
formatOnSave = true
```

---

## References

- [VS Code Settings Documentation](https://code.visualstudio.com/docs/getstarted/settings)
- [Neovim Configuration](https://neovim.io/doc/user/lua.html#lua-vim-options)
- [TOML Specification](https://toml.io/en/)
- [JSON Schema](https://json-schema.org/)
