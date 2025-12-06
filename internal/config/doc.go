// Package config provides the configuration system for Keystorm.
//
// The config package manages loading, merging, validating, and providing
// access to all editor settings including user preferences, keymaps,
// per-language settings, and plugin configurations.
//
// # Architecture
//
// Configuration is organized in layers with higher layers overriding lower:
//
//	┌─────────────────────────────┐
//	│  7. Command Line Arguments  │  ← Highest priority
//	├─────────────────────────────┤
//	│  6. Environment Variables   │
//	├─────────────────────────────┤
//	│  5. Plugin Settings         │
//	├─────────────────────────────┤
//	│  4. Project/Workspace       │  ← .keystorm/config.toml
//	├─────────────────────────────┤
//	│  3. User Keymaps            │  ← ~/.config/keystorm/keymaps.toml
//	├─────────────────────────────┤
//	│  2. User Settings           │  ← ~/.config/keystorm/settings.toml
//	├─────────────────────────────┤
//	│  1. Built-in Defaults       │  ← Lowest priority
//	└─────────────────────────────┘
//
// # Sub-packages
//
//   - loader: Configuration file loading (TOML, JSON, environment variables)
//   - registry: Type-safe settings registry with definitions and accessors
//   - layer: Layer management and merging strategies
//   - schema: JSON Schema validation
//   - watcher: File watching for live reload
//   - notify: Change notification and observer pattern
//
// # Basic Usage
//
// Load configuration from default paths:
//
//	cfg, err := config.Load()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access typed settings
//	tabSize := cfg.GetInt("editor.tabSize")
//	theme := cfg.GetString("ui.theme")
//
//	// Access typed sections
//	editor := cfg.Editor()
//	fmt.Println(editor.TabSize)
//
// # Type-Safe Access
//
// The registry provides type-safe accessors to prevent runtime errors:
//
//	// Using generic accessor
//	tabSize, err := cfg.GetInt("editor.tabSize")
//	if err != nil {
//	    // Handle error (wrong type or unknown setting)
//	}
//
//	// Using typed section
//	editor := cfg.Editor()
//	tabSize := editor.TabSize // Compile-time type safety
//
// # Configuration Files
//
// Keystorm uses TOML as the primary configuration format:
//
//	# ~/.config/keystorm/settings.toml
//	[editor]
//	tabSize = 4
//	insertSpaces = true
//	wordWrap = "on"
//
//	[ui]
//	theme = "dark"
//	fontSize = 14
//
// # Error Handling
//
// The package defines several error types:
//
//   - ErrSettingNotFound: Setting path doesn't exist
//   - ErrTypeMismatch: Value type doesn't match expected type
//   - ErrValidationFailed: Value fails schema validation
//   - ErrParseError: Configuration file parsing failed
//   - ErrFileNotFound: Configuration file doesn't exist
package config
