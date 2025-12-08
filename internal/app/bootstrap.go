package app

import (
	"context"
	"time"

	"github.com/dshills/keystorm/internal/config"
	"github.com/dshills/keystorm/internal/dispatcher"
	"github.com/dshills/keystorm/internal/event"
	"github.com/dshills/keystorm/internal/input/mode"
	"github.com/dshills/keystorm/internal/integration"
	"github.com/dshills/keystorm/internal/lsp"
	"github.com/dshills/keystorm/internal/plugin"
	"github.com/dshills/keystorm/internal/project"
)

// bootstrapper handles component initialization with proper cleanup on failure.
type bootstrapper struct {
	app       *Application
	opts      Options
	initOrder []string
}

// newBootstrapper creates a new bootstrapper for the application.
func newBootstrapper(app *Application, opts Options) *bootstrapper {
	return &bootstrapper{
		app:       app,
		opts:      opts,
		initOrder: make([]string, 0, 10),
	}
}

// bootstrap initializes all components in dependency order.
// On failure, it cleans up already-initialized components.
func (b *bootstrapper) bootstrap() error {
	var err error

	// 1. Event Bus - messaging foundation
	if err = b.initEventBus(); err != nil {
		b.cleanup()
		return err
	}

	// 2. Config System
	if err = b.initConfig(); err != nil {
		b.cleanup()
		return err
	}

	// 3. Mode Manager
	if err = b.initModeManager(); err != nil {
		b.cleanup()
		return err
	}

	// 4. Dispatcher
	if err = b.initDispatcher(); err != nil {
		b.cleanup()
		return err
	}

	// 5. Project (if workspace specified)
	if err = b.initProject(); err != nil {
		b.cleanup()
		return err
	}

	// 6. LSP Manager
	if err = b.initLSP(); err != nil {
		b.cleanup()
		return err
	}

	// 7. Plugin System
	if err = b.initPlugins(); err != nil {
		b.cleanup()
		return err
	}

	// 8. Integration Manager
	if err = b.initIntegration(); err != nil {
		b.cleanup()
		return err
	}

	// 9. Open initial files and setup documents
	if err = b.initDocuments(); err != nil {
		b.cleanup()
		return err
	}

	return nil
}

// initEventBus initializes the event bus.
func (b *bootstrapper) initEventBus() error {
	b.app.eventBus = event.NewBus()
	if err := b.app.eventBus.Start(); err != nil {
		return &InitError{Component: "event bus", Err: err}
	}
	b.initOrder = append(b.initOrder, "eventBus")
	return nil
}

// initConfig initializes the configuration system.
func (b *bootstrapper) initConfig() error {
	configOpts := []config.Option{
		config.WithWatcher(true),
		config.WithSchemaValidation(true),
	}

	if b.opts.ConfigPath != "" {
		// ConfigPath specifies user config directory
		configOpts = append(configOpts, config.WithUserConfigDir(b.opts.ConfigPath))
	}

	if b.opts.WorkspacePath != "" {
		configOpts = append(configOpts, config.WithProjectConfigDir(b.opts.WorkspacePath))
	}

	b.app.config = config.New(configOpts...)

	// Load configuration - errors are non-fatal, use defaults
	if err := b.app.config.Load(context.Background()); err != nil {
		// Log warning in production but continue with defaults
		_ = err
	}

	b.initOrder = append(b.initOrder, "config")
	return nil
}

// initModeManager initializes the mode manager with default modes.
func (b *bootstrapper) initModeManager() error {
	b.app.modeManager = mode.NewManager()

	// Register default editing modes
	b.registerModes()

	b.initOrder = append(b.initOrder, "modeManager")
	return nil
}

// registerModes registers the default editing modes.
func (b *bootstrapper) registerModes() {
	// Register placeholder modes - real modes from vim package would be registered here
	// This allows the application to be tested without full vim implementation
	b.app.modeManager.Register(&placeholderMode{name: "normal"})
	b.app.modeManager.Register(&placeholderMode{name: "insert"})
	b.app.modeManager.Register(&placeholderMode{name: "visual"})
	b.app.modeManager.Register(&placeholderMode{name: "command"})
	b.app.modeManager.Register(&placeholderMode{name: "replace"})
}

// initDispatcher initializes the dispatcher with handlers.
func (b *bootstrapper) initDispatcher() error {
	dispatcherConfig := dispatcher.DefaultConfig()
	dispatcherConfig.RecoverFromPanic = true
	dispatcherConfig.EnableMetrics = b.opts.Debug

	b.app.dispatcher = dispatcher.New(dispatcherConfig)

	// Note: ModeManager wiring requires an adapter to bridge the interface types.
	// The mode.Manager returns mode.Mode from Current(), but execctx expects
	// execctx.ModeInterface. This will be addressed in Phase 3 (handler integration).
	// For now, dispatcher is initialized without mode manager wiring.
	// TODO: Create mode manager adapter in Phase 3

	// Register core handlers
	b.registerHandlers()

	b.initOrder = append(b.initOrder, "dispatcher")
	return nil
}

// registerHandlers registers all dispatcher handlers.
func (b *bootstrapper) registerHandlers() {
	// Register all standard handlers with the dispatcher
	RegisterHandlers(b.app.dispatcher)
}

// initProject initializes the project/workspace manager.
func (b *bootstrapper) initProject() error {
	if b.opts.WorkspacePath == "" {
		// No workspace specified - skip project initialization
		return nil
	}

	proj := project.New(project.WithConfig(project.DefaultConfig()))
	if err := proj.Open(context.Background(), b.opts.WorkspacePath); err != nil {
		// Project open errors are non-fatal - continue without project
		_ = err
		return nil
	}

	b.app.project = proj
	b.initOrder = append(b.initOrder, "project")
	return nil
}

// initLSP initializes the LSP manager.
func (b *bootstrapper) initLSP() error {
	b.app.lsp = lsp.NewManager(
		lsp.WithRequestTimeout(10*time.Second),
		lsp.WithSupervision(lsp.DefaultSupervisorConfig()),
	)

	// Register default language servers based on detection
	for lang, cfg := range lsp.AutoDetectServers() {
		b.app.lsp.RegisterServer(lang, cfg)
	}

	// Set workspace folders if project is open
	if b.app.project != nil {
		folders := lsp.DetectWorkspaceFolders(b.app.project.Root())
		b.app.lsp.SetWorkspaceFolders(folders)
	}

	b.initOrder = append(b.initOrder, "lsp")
	return nil
}

// initPlugins initializes the plugin manager.
func (b *bootstrapper) initPlugins() error {
	b.app.plugins = plugin.NewManager(plugin.DefaultManagerConfig())
	b.initOrder = append(b.initOrder, "plugins")
	return nil
}

// initIntegration initializes the integration manager.
func (b *bootstrapper) initIntegration() error {
	integrationOpts := []integration.ManagerOption{
		integration.WithShutdownTimeout(5 * time.Second),
	}

	if b.opts.WorkspacePath != "" {
		integrationOpts = append(integrationOpts, integration.WithWorkspaceRoot(b.opts.WorkspacePath))
	}

	mgr, err := integration.NewManager(integrationOpts...)
	if err != nil {
		// Integration errors are non-fatal - continue without integration
		_ = err
		return nil
	}

	b.app.integration = mgr
	b.initOrder = append(b.initOrder, "integration")
	return nil
}

// initDocuments initializes the document manager and opens initial files.
func (b *bootstrapper) initDocuments() error {
	b.app.documents = NewDocumentManager()

	// Open initial files
	for _, file := range b.opts.Files {
		if _, err := b.app.documents.Open(file); err != nil {
			// File open errors are non-fatal for startup
			_ = err
		}
	}

	// Create scratch buffer if no files opened
	if b.app.documents.Count() == 0 {
		b.app.documents.CreateScratch()
	}

	b.initOrder = append(b.initOrder, "documents")
	return nil
}

// cleanup performs cleanup in reverse initialization order.
// Called when bootstrap fails partway through.
func (b *bootstrapper) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Cleanup in reverse order
	for i := len(b.initOrder) - 1; i >= 0; i-- {
		component := b.initOrder[i]
		b.cleanupComponent(ctx, component)
	}
}

// cleanupComponent cleans up a single component.
func (b *bootstrapper) cleanupComponent(ctx context.Context, component string) {
	switch component {
	case "eventBus":
		if b.app.eventBus != nil {
			b.app.eventBus.Stop(ctx)
			b.app.eventBus = nil
		}
	case "config":
		if b.app.config != nil {
			b.app.config.Close()
			b.app.config = nil
		}
	case "modeManager":
		b.app.modeManager = nil
	case "dispatcher":
		b.app.dispatcher = nil
	case "project":
		if b.app.project != nil {
			b.app.project.Close(ctx)
			b.app.project = nil
		}
	case "lsp":
		if b.app.lsp != nil {
			b.app.lsp.Shutdown(ctx)
			b.app.lsp = nil
		}
	case "plugins":
		if b.app.plugins != nil {
			_ = b.app.plugins.UnloadAll(ctx)
			b.app.plugins = nil
		}
	case "integration":
		if b.app.integration != nil {
			b.app.integration.Close()
			b.app.integration = nil
		}
	case "documents":
		b.app.documents = nil
	}
}

// WireEventSubscriptions sets up event subscriptions between components.
// Called after bootstrap completes successfully.
// Prerequisites: eventBus must be initialized and started.
func (app *Application) WireEventSubscriptions() error {
	if app.eventBus == nil {
		return nil
	}

	// Create and initialize subscription manager
	app.subscriptions = newSubscriptionManager(app)
	if err := app.subscriptions.setupSubscriptions(); err != nil {
		return &InitError{Component: "subscriptions", Err: err}
	}

	return nil
}

// WireDispatcher connects the dispatcher to active document.
func (app *Application) WireDispatcher() {
	if app.dispatcher == nil {
		return
	}

	doc := app.documents.Active()
	if doc == nil {
		return
	}

	// Note: Engine and cursor wiring requires adapters to bridge interface types.
	// The engine.Engine has methods with different signatures than what
	// execctx.EngineInterface expects (e.g., Delete returns error vs EditResult).
	// This will be addressed in Phase 3 (handler integration).
	// TODO: Create engine/cursor adapters in Phase 3
	_ = doc // Suppress unused warning
}

// SwitchDocument changes the active document and re-wires the dispatcher.
func (app *Application) SwitchDocument(doc *Document) {
	if doc == nil {
		return
	}

	app.documents.SetActive(doc)
	app.WireDispatcher()
}
