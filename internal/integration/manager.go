package integration

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dshills/keystorm/internal/config"
	"github.com/dshills/keystorm/internal/integration/process"
)

// EventPublisher defines the interface for publishing integration events.
//
// Event types published by the integration layer:
//   - terminal.created, terminal.closed, terminal.output
//   - git.status.changed, git.branch.changed, git.commit.created
//   - debug.session.started, debug.session.stopped, debug.breakpoint.hit
//   - task.started, task.output, task.completed
type EventPublisher interface {
	// Publish sends an event to subscribers.
	Publish(eventType string, data map[string]any)
}

// Manager is the central facade for all integration features.
//
// It provides a unified API and manages component lifecycles for:
//   - Terminal emulation
//   - Git operations
//   - Debugger integration
//   - Task runner
//
// Manager is safe for concurrent use.
type Manager struct {
	mu sync.RWMutex

	// Core components (will be added in later phases)
	// terminal  *terminal.Manager
	// git       *git.Manager
	// debug     *debug.Manager
	// task      *task.Manager

	// Process supervisor for child process management
	supervisor *process.Supervisor

	// Configuration
	workspaceRoot   string
	configSystem    *config.ConfigSystem
	shutdownTimeout time.Duration

	// Event publishing
	eventBus EventPublisher

	// Lifecycle
	closed   atomic.Bool
	shutdown chan struct{}

	// Metrics
	startTime time.Time
}

// ManagerConfig configures the integration manager.
type ManagerConfig struct {
	// WorkspaceRoot is the root directory for git and task operations.
	// Required for most integration features.
	WorkspaceRoot string

	// ConfigSystem provides configuration access.
	// Optional - if nil, default configurations are used.
	ConfigSystem *config.ConfigSystem

	// EventBus for publishing integration events.
	// Optional - if nil, events are not published.
	EventBus EventPublisher

	// MaxProcesses limits concurrent child processes (0 = unlimited).
	MaxProcesses int

	// ShutdownTimeout is the graceful shutdown timeout for processes.
	// Default is 5 seconds.
	ShutdownTimeout time.Duration
}

// ManagerOption configures a Manager instance.
type ManagerOption func(*managerOptions)

type managerOptions struct {
	workspaceRoot   string
	configSystem    *config.ConfigSystem
	eventBus        EventPublisher
	maxProcesses    int
	shutdownTimeout time.Duration
}

// WithWorkspaceRoot sets the workspace root directory.
func WithWorkspaceRoot(root string) ManagerOption {
	return func(o *managerOptions) {
		o.workspaceRoot = root
	}
}

// WithConfigSystem sets the configuration system.
func WithConfigSystem(cs *config.ConfigSystem) ManagerOption {
	return func(o *managerOptions) {
		o.configSystem = cs
	}
}

// WithEventBus sets the event publisher.
func WithEventBus(eb EventPublisher) ManagerOption {
	return func(o *managerOptions) {
		o.eventBus = eb
	}
}

// WithMaxProcesses sets the maximum concurrent processes.
func WithMaxProcesses(max int) ManagerOption {
	return func(o *managerOptions) {
		o.maxProcesses = max
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout.
func WithShutdownTimeout(timeout time.Duration) ManagerOption {
	return func(o *managerOptions) {
		o.shutdownTimeout = timeout
	}
}

// NewManager creates a new integration manager.
//
// The manager coordinates all integration components and manages their
// lifecycles. Call Close() when done to clean up resources.
func NewManager(opts ...ManagerOption) (*Manager, error) {
	// Apply options with defaults
	options := &managerOptions{
		shutdownTimeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(options)
	}

	// Create process supervisor
	supervisorOpts := []process.SupervisorOption{}
	if options.maxProcesses > 0 {
		supervisorOpts = append(supervisorOpts, process.WithMaxProcesses(options.maxProcesses))
	}
	supervisor := process.NewSupervisor(supervisorOpts...)

	m := &Manager{
		supervisor:      supervisor,
		workspaceRoot:   options.workspaceRoot,
		configSystem:    options.configSystem,
		shutdownTimeout: options.shutdownTimeout,
		eventBus:        options.eventBus,
		shutdown:        make(chan struct{}),
		startTime:       time.Now(),
	}

	// Publish manager started event
	m.publishEvent("integration.started", map[string]any{
		"workspace": m.workspaceRoot,
	})

	return m, nil
}

// NewManagerWithConfig creates a manager from a ManagerConfig struct.
//
// This is an alternative to NewManager with functional options.
func NewManagerWithConfig(cfg ManagerConfig) (*Manager, error) {
	opts := []ManagerOption{}

	if cfg.WorkspaceRoot != "" {
		opts = append(opts, WithWorkspaceRoot(cfg.WorkspaceRoot))
	}
	if cfg.ConfigSystem != nil {
		opts = append(opts, WithConfigSystem(cfg.ConfigSystem))
	}
	if cfg.EventBus != nil {
		opts = append(opts, WithEventBus(cfg.EventBus))
	}
	if cfg.MaxProcesses > 0 {
		opts = append(opts, WithMaxProcesses(cfg.MaxProcesses))
	}
	if cfg.ShutdownTimeout > 0 {
		opts = append(opts, WithShutdownTimeout(cfg.ShutdownTimeout))
	}

	return NewManager(opts...)
}

// Close shuts down all integration components and releases resources.
//
// Close sends SIGTERM to all child processes and waits for them to exit.
// Processes that don't exit within the shutdown timeout are killed.
//
// It is safe to call Close multiple times.
func (m *Manager) Close() error {
	if m.closed.Swap(true) {
		return nil // Already closed
	}

	close(m.shutdown)

	// Publish shutdown event
	m.publishEvent("integration.stopping", nil)

	// Shutdown supervisor with configured timeout
	m.supervisor.Shutdown(m.shutdownTimeout)

	// Publish stopped event
	m.publishEvent("integration.stopped", map[string]any{
		"uptime": time.Since(m.startTime).String(),
	})

	return nil
}

// CloseWithTimeout shuts down with a custom timeout.
func (m *Manager) CloseWithTimeout(timeout time.Duration) error {
	if m.closed.Swap(true) {
		return nil // Already closed
	}

	close(m.shutdown)

	m.publishEvent("integration.stopping", nil)
	m.supervisor.Shutdown(timeout)
	m.publishEvent("integration.stopped", map[string]any{
		"uptime": time.Since(m.startTime).String(),
	})

	return nil
}

// IsClosed returns true if the manager has been closed.
func (m *Manager) IsClosed() bool {
	return m.closed.Load()
}

// ShutdownChan returns a channel that is closed when shutdown begins.
func (m *Manager) ShutdownChan() <-chan struct{} {
	return m.shutdown
}

// Supervisor returns the process supervisor.
//
// The supervisor manages child processes for terminals, debuggers, and tasks.
// Use this for advanced process management needs.
func (m *Manager) Supervisor() *process.Supervisor {
	return m.supervisor
}

// WorkspaceRoot returns the configured workspace root.
func (m *Manager) WorkspaceRoot() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.workspaceRoot
}

// SetWorkspaceRoot updates the workspace root.
//
// This may be called when the user opens a different workspace.
func (m *Manager) SetWorkspaceRoot(root string) {
	m.mu.Lock()
	m.workspaceRoot = root
	m.mu.Unlock()

	m.publishEvent("integration.workspace.changed", map[string]any{
		"workspace": root,
	})
}

// Config returns the configuration system.
// May return nil if no config system was provided.
func (m *Manager) Config() *config.ConfigSystem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configSystem
}

// SetConfig updates the configuration system.
func (m *Manager) SetConfig(cs *config.ConfigSystem) {
	m.mu.Lock()
	m.configSystem = cs
	m.mu.Unlock()
}

// EventBus returns the event publisher.
// May return nil if no event bus was provided.
func (m *Manager) EventBus() EventPublisher {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.eventBus
}

// SetEventBus updates the event publisher.
func (m *Manager) SetEventBus(eb EventPublisher) {
	m.mu.Lock()
	m.eventBus = eb
	m.mu.Unlock()
}

// Uptime returns how long the manager has been running.
func (m *Manager) Uptime() time.Duration {
	return time.Since(m.startTime)
}

// Health returns the health status of integration components.
func (m *Manager) Health() HealthStatus {
	status := HealthStatus{
		Status:        StatusHealthy,
		Uptime:        m.Uptime(),
		ProcessCount:  m.supervisor.Count(),
		Components:    make(map[string]ComponentHealth),
		WorkspaceRoot: m.WorkspaceRoot(),
		Configured:    m.configSystem != nil,
		EventsEnabled: m.eventBus != nil,
	}

	// Check supervisor health
	if m.supervisor.IsShuttingDown() {
		status.Status = StatusDegraded
		status.Components["supervisor"] = ComponentHealth{
			Status:  StatusDegraded,
			Message: "shutting down",
		}
	} else {
		status.Components["supervisor"] = ComponentHealth{
			Status:  StatusHealthy,
			Message: fmt.Sprintf("%d processes", m.supervisor.Count()),
		}
	}

	// Add component health checks as they are implemented
	// status.Components["terminal"] = m.terminal.Health()
	// status.Components["git"] = m.git.Health()
	// status.Components["debug"] = m.debug.Health()
	// status.Components["task"] = m.task.Health()

	return status
}

// publishEvent publishes an event if an event bus is configured.
// It creates a copy of the data map to avoid mutating the caller's map.
func (m *Manager) publishEvent(eventType string, data map[string]any) {
	m.mu.RLock()
	eb := m.eventBus
	m.mu.RUnlock()

	if eb != nil {
		// Create a copy to avoid mutating caller's map
		eventData := make(map[string]any, len(data)+1)
		for k, v := range data {
			eventData[k] = v
		}
		eventData["timestamp"] = time.Now().UnixMilli()

		// Publish with panic recovery to prevent event bus issues from crashing the manager
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but don't crash - event publishing errors shouldn't affect manager
				}
			}()
			eb.Publish(eventType, eventData)
		}()
	}
}

// WaitForShutdown blocks until the manager is closed.
//
// This is useful for main functions that need to wait for cleanup:
//
//	manager, _ := integration.NewManager(...)
//	go func() {
//	    <-sigChan
//	    manager.Close()
//	}()
//	manager.WaitForShutdown()
func (m *Manager) WaitForShutdown() {
	<-m.shutdown
	m.supervisor.Wait()
}

// WaitForShutdownContext blocks until the manager is closed or context is cancelled.
func (m *Manager) WaitForShutdownContext(ctx context.Context) error {
	select {
	case <-m.shutdown:
		m.supervisor.Wait()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// HealthStatus represents the health of the integration layer.
type HealthStatus struct {
	// Status is the overall health status.
	Status Status

	// Uptime is how long the manager has been running.
	Uptime time.Duration

	// ProcessCount is the number of active child processes.
	ProcessCount int

	// Components contains health status for each component.
	Components map[string]ComponentHealth

	// WorkspaceRoot is the configured workspace.
	WorkspaceRoot string

	// Configured indicates if a config system is set.
	Configured bool

	// EventsEnabled indicates if event publishing is enabled.
	EventsEnabled bool
}

// ComponentHealth represents the health of a single component.
type ComponentHealth struct {
	// Status is the component's health status.
	Status Status

	// Message provides additional details.
	Message string

	// LastError is the most recent error, if any.
	LastError string

	// Metadata contains component-specific information.
	Metadata map[string]any
}

// Status represents a health status level.
type Status int

const (
	// StatusHealthy indicates the component is fully operational.
	StatusHealthy Status = iota

	// StatusDegraded indicates the component is operational but with issues.
	StatusDegraded

	// StatusUnhealthy indicates the component is not operational.
	StatusUnhealthy
)

// String returns a human-readable status name.
func (s Status) String() string {
	switch s {
	case StatusHealthy:
		return "healthy"
	case StatusDegraded:
		return "degraded"
	case StatusUnhealthy:
		return "unhealthy"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}
