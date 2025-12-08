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
	"github.com/dshills/keystorm/internal/input/key"
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
	plugins     *plugin.Manager
	integration *integration.Manager

	// Event subscriptions
	subscriptions *subscriptionManager

	// State
	running atomic.Bool
	done    chan struct{}

	// Shutdown synchronization
	shutdownOnce sync.Once

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
		opts: opts,
		done: make(chan struct{}),
	}

	// Use bootstrapper for component initialization with cleanup on failure
	b := newBootstrapper(app, opts)
	if err := b.bootstrap(); err != nil {
		return nil, err
	}

	// Wire event subscriptions after successful bootstrap
	if err := app.WireEventSubscriptions(); err != nil {
		b.cleanup()
		return nil, &InitError{Component: "event subscriptions", Err: err}
	}

	return app, nil
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

	// Wire dispatcher to active document
	app.WireDispatcher()

	// Set initial mode
	if err := app.modeManager.SetInitialMode("normal"); err != nil {
		// Non-fatal, continue without mode
		_ = err
	}

	// Load plugins
	if app.plugins != nil {
		ctx := context.Background()
		if err := app.plugins.LoadAll(ctx); err != nil {
			// Non-fatal, log warning
			_ = err
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

	// Start input polling goroutine
	inputEvents := app.startInputPolling()

	lastUpdate := time.Now()

	for app.running.Load() {
		select {
		case <-app.done:
			return nil

		case ev, ok := <-inputEvents:
			if !ok {
				// Input channel closed
				return nil
			}
			// Handle input event
			if err := app.handleBackendEvent(ev); err != nil {
				if err == ErrQuit {
					return nil
				}
				// Log error but continue
				_ = err
			}

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

	// Note: Cursor provider, mode display, file path, and modified state
	// would be set through a status line component or separate UI layer
	// in a full implementation. The renderer focuses on text content only.
}

// Shutdown initiates graceful shutdown.
// Safe to call multiple times.
func (app *Application) Shutdown() {
	app.shutdownOnce.Do(func() {
		// Signal event loop to stop
		close(app.done)

		// Perform cleanup if running
		if app.running.Load() {
			app.shutdown()
		}
	})
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
			_ = app.plugins.UnloadAll(ctx)
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

	// 5. Cleanup event subscriptions (before stopping event bus)
	// Subscriptions must be cleaned up while event bus is still running
	// to properly unsubscribe handlers.
	if app.subscriptions != nil {
		app.subscriptions.cleanup()
	}

	// 6. Close config
	if app.config != nil {
		app.config.Close()
	}

	// 7. Stop event bus
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

// Plugins returns the plugin manager (may be nil).
func (app *Application) Plugins() *plugin.Manager {
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
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return "init " + e.Component
	}
	return "init " + e.Component + ": " + e.Err.Error()
}

func (e *InitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// placeholderMode is a minimal mode implementation for bootstrapping.
type placeholderMode struct {
	name string
}

// Compile-time assertion that placeholderMode implements mode.Mode.
var _ mode.Mode = (*placeholderMode)(nil)

func (m *placeholderMode) Name() string        { return m.name }
func (m *placeholderMode) DisplayName() string { return m.name }
func (m *placeholderMode) CursorStyle() mode.CursorStyle {
	if m.name == "insert" {
		return mode.CursorBar
	}
	return mode.CursorBlock
}

func (m *placeholderMode) Enter(_ *mode.Context) error { return nil }
func (m *placeholderMode) Exit(_ *mode.Context) error  { return nil }

func (m *placeholderMode) HandleUnmapped(_ key.Event, _ *mode.Context) *mode.UnmappedResult {
	return nil
}
