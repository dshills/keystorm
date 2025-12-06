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
// # ConfigSystem Facade
//
// For most use cases, use ConfigSystem which provides a high-level facade:
//
//	sys, err := config.NewConfigSystem(ctx,
//	    config.WithSystemUserConfigDir("/path/to/config"),
//	    config.WithSystemWatcher(true),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sys.Close()
//
//	// Access typed sections
//	editor := sys.Editor()
//	fmt.Println(editor.TabSize)
//
//	// Check system health
//	health := sys.Health()
//	if health.Status != config.HealthOK {
//	    log.Warn("config issues:", health.Errors)
//	}
//
// # Typed Section Accessors
//
// The package provides type-safe accessors for all configuration sections:
//
//	// Editor settings
//	editor := cfg.Editor()
//	tabSize := editor.TabSize
//	insertSpaces := editor.InsertSpaces
//
//	// UI settings
//	ui := cfg.UI()
//	theme := ui.Theme
//	fontSize := ui.FontSize
//
//	// Vim mode settings
//	vim := cfg.Vim()
//	enabled := vim.Enabled
//
//	// Input settings
//	input := cfg.Input()
//	leaderKey := input.LeaderKey
//
//	// File settings
//	files := cfg.Files()
//	encoding := files.Encoding
//
//	// Search settings
//	search := cfg.Search()
//	maxResults := search.MaxResults
//
//	// AI settings
//	ai := cfg.AI()
//	provider := ai.Provider
//
//	// Logging settings
//	logging := cfg.Logging()
//	level := logging.Level
//
//	// Terminal settings
//	terminal := cfg.Terminal()
//	shell := terminal.Shell
//
//	// LSP settings
//	lsp := cfg.LSP()
//	enabled := lsp.Enabled
//
//	// Path settings
//	paths := cfg.Paths()
//	configDir := paths.ConfigDir
//
// # Plugin Configuration
//
// Plugin configuration is managed via PluginManager:
//
//	plugins := cfg.Plugins()
//
//	// Get plugin config
//	pc, ok := plugins.GetPluginConfig("my-plugin")
//	if ok {
//	    fmt.Println(pc.Enabled)
//	}
//
//	// Subscribe to plugin changes
//	sub := plugins.SubscribePlugin("my-plugin", func(change notify.Change) {
//	    // Handle change
//	})
//	defer sub.Unsubscribe()
//
// # Keymap Configuration
//
// Keymaps are managed via KeymapManager:
//
//	keymaps := cfg.Keymaps()
//
//	// Load default keymaps
//	keymaps.LoadDefaults()
//
//	// Add user binding
//	keymaps.AddBinding("normal", config.KeymapBinding{
//	    Keys:   "g g",
//	    Action: "cursor.document_start",
//	})
//
//	// Lookup binding
//	binding, err := keymaps.Lookup("normal", "", "g g")
//
// # Configuration Migration
//
// The package supports migrating configuration between versions:
//
//	m := config.DefaultMigrator()
//
//	// Check if migration is needed
//	if m.NeedsMigration(data) {
//	    migrated, results, err := m.Migrate(data)
//	    if err != nil {
//	        log.Error("migration failed:", err)
//	    }
//	}
//
// # Change Notifications
//
// Subscribe to configuration changes:
//
//	// Subscribe to all changes
//	sub := cfg.Subscribe(func(change notify.Change) {
//	    fmt.Printf("Changed: %s\n", change.Path)
//	})
//	defer sub.Unsubscribe()
//
//	// Subscribe to specific path
//	sub := cfg.SubscribePath("editor", func(change notify.Change) {
//	    // Handle editor changes
//	})
//
// # Error Handling
//
// The package defines several error types:
//
//   - ErrSettingNotFound: Setting path doesn't exist
//   - ErrTypeMismatch: Value type doesn't match expected type
//   - ErrValidationFailed: Value fails schema validation
//   - ErrFileNotFound: Configuration file doesn't exist
//   - ErrLayerNotFound: Configuration layer doesn't exist
//   - ErrReadOnly: Attempted to modify a read-only layer
//
// # Thread Safety
//
// All exported types (Config, ConfigSystem, PluginManager, KeymapManager)
// are safe for concurrent use. Internal locking ensures data consistency.
//
// # Performance
//
// The configuration system is optimized for read-heavy workloads:
//   - Merged configuration is cached and invalidated on changes
//   - File watching uses debouncing to avoid excessive reloads
//   - Type-safe accessors return snapshot copies to prevent races
//   - Initial load time is typically under 50ms
package config
