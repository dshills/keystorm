// Package app provides the main application structure and coordination
// for the Keystorm editor. It wires together all core modules and manages
// the application lifecycle.
package app

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dshills/keystorm/internal/config"
	"github.com/dshills/keystorm/internal/dispatcher"
	"github.com/dshills/keystorm/internal/event"
	"github.com/dshills/keystorm/internal/input/mode"
	"github.com/dshills/keystorm/internal/integration"
	"github.com/dshills/keystorm/internal/lsp"
	"github.com/dshills/keystorm/internal/plugin"
	"github.com/dshills/keystorm/internal/project"
	"github.com/dshills/keystorm/internal/renderer"
	"github.com/dshills/keystorm/internal/renderer/backend"
)

// Application is the central coordinator for all Keystorm components.
// It manages component lifecycles, wiring, and the main event loop.
type Application struct {
	mu sync.RWMutex

	// Core infrastructure
	eventBus event.Bus
	config   *config.Config

	// Editor components
	renderer    *renderer.Renderer
	backend     backend.Backend
	modeManager *mode.Manager
	dispatcher  *dispatcher.Dispatcher

	// Document management
	documents *DocumentManager

	// Workspace components
	project project.Project
	lsp     *lsp.Manager

	// Extension components
	plugins     *plugin.System
	integration *integration.Manager

	// State
	running atomic.Bool
	done    chan struct{}

	// Options
	opts Options
}

// Options configures the application.
type Options struct {
	// ConfigPath is the path to the configuration file.
	ConfigPath string

	// WorkspacePath is the workspace/project directory.
	WorkspacePath string

	// Files are files to open on startup.
	Files []string

	// Debug enables debug mode with extra logging.
	Debug bool

	// LogLevel sets the logging verbosity.
	LogLevel string

	// ReadOnly opens files in read-only mode.
	ReadOnly bool
}

// New creates a new Application with the given options.
func New(opts Options) (*Application, error) {
	app := &Application{
		opts:      opts,
		done:      make(chan struct{}),
		documents: NewDocumentManager(),
	}

	if err := app.bootstrap(); err != nil {
		return nil, err
	}

	return app, nil
}

// bootstrap initializes all components in dependency order.
func (app *Application) bootstrap() error {
	var err error

	// 1. Event Bus - messaging foundation
	app.eventBus = event.NewBus()
	if err := app.eventBus.Start(); err != nil {
		return &InitError{Component: "event bus", Err: err}
	}

	// 2. Config System
	configOpts := []config.Option{
		config.WithWatcher(true),
		config.WithSchemaValidation(true),
	}
	if app.opts.WorkspacePath != "" {
		configOpts = append(configOpts, config.WithProjectConfigDir(app.opts.WorkspacePath))
	}
	app.config = config.New(configOpts...)
	if err := app.config.Load(context.Background()); err != nil {
		// Config load errors are non-fatal - use defaults
		// Log warning in production
	}

	// 3. Mode Manager
	app.modeManager = mode.NewManager()
	app.registerModes()

	// 4. Dispatcher
	dispatcherConfig := dispatcher.DefaultConfig()
	dispatcherConfig.RecoverFromPanic = true
	app.dispatcher = dispatcher.New(dispatcherConfig)
	app.dispatcher.SetModeManager(app.modeManager)

	// 5. Project (if workspace specified)
	if app.opts.WorkspacePath != "" {
		proj := project.New(project.WithConfig(project.DefaultConfig()))
		if err := proj.Open(context.Background(), app.opts.WorkspacePath); err != nil {
			// Project open errors are non-fatal
			// Log warning in production
		} else {
			app.project = proj
		}
	}

	// 6. LSP Manager
	app.lsp = lsp.NewManager(
		lsp.WithRequestTimeout(10*time.Second),
		lsp.WithSupervision(lsp.DefaultSupervisorConfig()),
	)

	// Register default language servers
	for lang, cfg := range lsp.AutoDetectServers() {
		app.lsp.RegisterServer(lang, cfg)
	}

	// Set workspace folders if project is open
	if app.project != nil {
		folders := lsp.DetectWorkspaceFolders(app.project.Root())
		app.lsp.SetWorkspaceFolders(folders)
	}

	// 7. Plugin System
	app.plugins, err = plugin.NewSystem()
	if err != nil {
		// Plugin system errors are non-fatal
		// Log warning in production
		app.plugins = nil
	}

	// 8. Integration Manager
	integrationOpts := []integration.ManagerOption{
		integration.WithShutdownTimeout(5 * time.Second),
	}
	if app.opts.WorkspacePath != "" {
		integrationOpts = append(integrationOpts, integration.WithWorkspaceRoot(app.opts.WorkspacePath))
	}
	app.integration, err = integration.NewManager(integrationOpts...)
	if err != nil {
		// Integration errors are non-fatal
		// Log warning in production
		app.integration = nil
	}

	// 9. Open initial files
	for _, file := range app.opts.Files {
		if _, err := app.documents.Open(file); err != nil {
			// File open errors are non-fatal for startup
			// Log warning in production
		}
	}

	// Create scratch buffer if no files opened
	if app.documents.Count() == 0 {
		app.documents.CreateScratch()
	}

	return nil
}

// registerModes registers the default editing modes.
func (app *Application) registerModes() {
	// Register placeholder modes - real modes from vim package would be registered here
	// This allows the application to be tested without full vim implementation
	app.modeManager.Register(&placeholderMode{name: "normal"})
	app.modeManager.Register(&placeholderMode{name: "insert"})
	app.modeManager.Register(&placeholderMode{name: "visual"})
	app.modeManager.Register(&placeholderMode{name: "command"})
}

// SetBackend sets the terminal backend.
// Must be called before Run().
func (app *Application) SetBackend(b backend.Backend) error {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.running.Load() {
		return ErrAlreadyRunning
	}

	app.backend = b
	return nil
}

// Run starts the application main loop.
// Blocks until shutdown is requested.
func (app *Application) Run() error {
	if !app.running.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	defer app.running.Store(false)

	// Initialize backend if set
	if app.backend != nil {
		if err := app.backend.Init(); err != nil {
			return &InitError{Component: "backend", Err: err}
		}
		defer app.backend.Shutdown()

		// Create renderer with backend
		app.renderer = renderer.New(app.backend, renderer.DefaultOptions())
	}

	// Set initial mode
	if err := app.modeManager.SetInitialMode("normal"); err != nil {
		// Non-fatal, continue without mode
	}

	// Load plugins
	if app.plugins != nil {
		if err := app.plugins.LoadAll(); err != nil {
			// Non-fatal, log warning
		}
	}

	// Run main event loop
	return app.eventLoop()
}

// eventLoop is the main application loop.
func (app *Application) eventLoop() error {
	if app.backend == nil {
		// No backend - wait for shutdown
		<-app.done
		return nil
	}

	const (
		targetFPS = 60
		frameTime = time.Second / targetFPS
	)

	frameTicker := time.NewTicker(frameTime)
	defer frameTicker.Stop()

	lastUpdate := time.Now()

	for app.running.Load() {
		select {
		case <-app.done:
			return nil

		case <-frameTicker.C:
			// Calculate delta time
			now := time.Now()
			dt := now.Sub(lastUpdate).Seconds()
			lastUpdate = now

			// Update and render
			if app.renderer != nil {
				app.updateRenderer()
				app.renderer.Update(dt)
				app.renderer.Render()
			}

		default:
			// Poll for input events (non-blocking would be better)
			// For now, we skip this to avoid blocking
			// A real implementation would use a separate goroutine for input
		}
	}

	return nil
}

// updateRenderer updates renderer state from current document.
func (app *Application) updateRenderer() {
	doc := app.documents.Active()
	if doc == nil || app.renderer == nil {
		return
	}

	// Set buffer for rendering
	app.renderer.SetBuffer(doc.Engine)

	// Set cursor provider (would need an adapter)
	// app.renderer.SetCursorProvider(cursorAdapter)
}

// Shutdown initiates graceful shutdown.
func (app *Application) Shutdown() {
	if !app.running.Load() {
		return
	}

	close(app.done)
	app.shutdown()
}

// shutdown performs cleanup in reverse initialization order.
func (app *Application) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// 1. Stop plugins
	if app.plugins != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.plugins.Shutdown()
		}()
	}

	// 2. Stop integration
	if app.integration != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.integration.Close()
		}()
	}

	// 3. Stop LSP
	if app.lsp != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.lsp.Shutdown(ctx)
		}()
	}

	// Wait for async shutdowns with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		// Timeout - continue with cleanup
	}

	// 4. Close project
	if app.project != nil {
		app.project.Close(ctx)
	}

	// 5. Close config
	if app.config != nil {
		app.config.Close()
	}

	// 6. Stop event bus
	if app.eventBus != nil {
		app.eventBus.Stop(ctx)
	}
}

// IsRunning returns true if the application is running.
func (app *Application) IsRunning() bool {
	return app.running.Load()
}

// EventBus returns the event bus.
func (app *Application) EventBus() event.Bus {
	return app.eventBus
}

// Config returns the configuration system.
func (app *Application) Config() *config.Config {
	return app.config
}

// Renderer returns the renderer.
func (app *Application) Renderer() *renderer.Renderer {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.renderer
}

// ModeManager returns the mode manager.
func (app *Application) ModeManager() *mode.Manager {
	return app.modeManager
}

// Dispatcher returns the dispatcher.
func (app *Application) Dispatcher() *dispatcher.Dispatcher {
	return app.dispatcher
}

// Documents returns the document manager.
func (app *Application) Documents() *DocumentManager {
	return app.documents
}

// Project returns the project (may be nil).
func (app *Application) Project() project.Project {
	return app.project
}

// LSP returns the LSP manager.
func (app *Application) LSP() *lsp.Manager {
	return app.lsp
}

// Plugins returns the plugin system (may be nil).
func (app *Application) Plugins() *plugin.System {
	return app.plugins
}

// Integration returns the integration manager (may be nil).
func (app *Application) Integration() *integration.Manager {
	return app.integration
}

// ActiveDocument returns the active document (may be nil).
func (app *Application) ActiveDocument() *Document {
	return app.documents.Active()
}

// InitError represents an initialization error.
type InitError struct {
	Component string
	Err       error
}

func (e *InitError) Error() string {
	return "init " + e.Component + ": " + e.Err.Error()
}

func (e *InitError) Unwrap() error {
	return e.Err
}

// placeholderMode is a minimal mode implementation for bootstrapping.
type placeholderMode struct {
	name string
}

func (m *placeholderMode) Name() string { return m.name }

func (m *placeholderMode) Enter(_ *mode.Context) error { return nil }

func (m *placeholderMode) Exit(_ *mode.Context) error { return nil }

func (m *placeholderMode) HandleKey(_ *mode.KeyEvent) (mode.Result, bool) {
	return mode.Result{}, false
}
