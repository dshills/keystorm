package lsp

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// SupervisorState represents the state of a supervised server.
type SupervisorState int

const (
	// SupervisorStateIdle means the supervisor is not monitoring.
	SupervisorStateIdle SupervisorState = iota
	// SupervisorStateRunning means the server is running normally.
	SupervisorStateRunning
	// SupervisorStateRestarting means the server crashed and is being restarted.
	SupervisorStateRestarting
	// SupervisorStateFailed means the server has exceeded max restart attempts.
	SupervisorStateFailed
	// SupervisorStateStopped means the supervisor was explicitly stopped.
	SupervisorStateStopped
)

// String returns a human-readable state name.
func (s SupervisorState) String() string {
	switch s {
	case SupervisorStateIdle:
		return "idle"
	case SupervisorStateRunning:
		return "running"
	case SupervisorStateRestarting:
		return "restarting"
	case SupervisorStateFailed:
		return "failed"
	case SupervisorStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// SupervisorConfig configures the server supervisor.
type SupervisorConfig struct {
	// MaxRestarts is the maximum number of restart attempts before giving up.
	// Default: 5
	MaxRestarts int

	// InitialBackoff is the initial backoff duration after a crash.
	// Default: 1 second
	InitialBackoff time.Duration

	// MaxBackoff is the maximum backoff duration.
	// Default: 60 seconds
	MaxBackoff time.Duration

	// BackoffMultiplier is the multiplier applied to backoff after each failure.
	// Default: 2.0
	BackoffMultiplier float64

	// ResetWindow is the time after which the restart count resets if the server
	// has been running successfully.
	// Default: 5 minutes
	ResetWindow time.Duration
}

// DefaultSupervisorConfig returns the default supervisor configuration.
func DefaultSupervisorConfig() SupervisorConfig {
	return SupervisorConfig{
		MaxRestarts:       5,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 2.0,
		ResetWindow:       5 * time.Minute,
	}
}

// SupervisorEvent represents an event from the supervisor.
type SupervisorEvent struct {
	Type       SupervisorEventType
	LanguageID string
	Error      error
	Attempt    int
	NextRetry  time.Duration
}

// SupervisorEventType identifies the type of supervisor event.
type SupervisorEventType int

const (
	// SupervisorEventCrash indicates the server crashed.
	SupervisorEventCrash SupervisorEventType = iota
	// SupervisorEventRestarting indicates a restart attempt is starting.
	SupervisorEventRestarting
	// SupervisorEventRecovered indicates the server has recovered.
	SupervisorEventRecovered
	// SupervisorEventFailed indicates the server has permanently failed.
	SupervisorEventFailed
)

// String returns a human-readable event type name.
func (t SupervisorEventType) String() string {
	switch t {
	case SupervisorEventCrash:
		return "crash"
	case SupervisorEventRestarting:
		return "restarting"
	case SupervisorEventRecovered:
		return "recovered"
	case SupervisorEventFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Supervisor monitors a language server and handles crash recovery.
// It automatically restarts crashed servers with exponential backoff
// and re-syncs open documents after recovery.
//
// Thread Safety: Supervisor is safe for concurrent use. The state field
// uses atomic operations for lock-free reads. Other fields are protected
// by mu (server management) or documentsMu (document tracking).
type Supervisor struct {
	mu sync.Mutex

	config     SupervisorConfig
	languageID string

	// Server management (protected by mu)
	server       *Server
	serverConfig ServerConfig
	folders      []WorkspaceFolder

	// State tracking (state uses atomic, others protected by mu)
	state        atomic.Int32
	restartCount int
	lastStart    time.Time

	// Document state for recovery (protected by documentsMu)
	documents   map[DocumentURI]documentState
	documentsMu sync.RWMutex
	diagHandler func(uri DocumentURI, diagnostics []Diagnostic)

	// Lifecycle
	ctx       context.Context
	cancel    context.CancelFunc
	eventCh   chan SupervisorEvent
	closed    atomic.Bool
	closeOnce sync.Once
}

// documentState captures the state of a document for recovery.
type documentState struct {
	URI        DocumentURI
	LanguageID string
	Content    string
}

// NewSupervisor creates a new server supervisor.
func NewSupervisor(serverConfig ServerConfig, languageID string, config SupervisorConfig) *Supervisor {
	s := &Supervisor{
		config:       config,
		languageID:   languageID,
		serverConfig: serverConfig,
		documents:    make(map[DocumentURI]documentState),
		eventCh:      make(chan SupervisorEvent, 16),
	}
	s.state.Store(int32(SupervisorStateIdle))
	return s
}

// Start begins supervision and starts the server.
func (s *Supervisor) Start(ctx context.Context, folders []WorkspaceFolder) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if SupervisorState(s.state.Load()) != SupervisorStateIdle {
		return ErrServerAlreadyRunning
	}

	s.folders = folders
	s.ctx, s.cancel = context.WithCancel(ctx)

	if err := s.startServerLocked(); err != nil {
		s.state.Store(int32(SupervisorStateFailed))
		return err
	}

	s.state.Store(int32(SupervisorStateRunning))

	// Start monitoring
	go s.monitor()

	return nil
}

// startServerLocked starts the server (must hold mu lock).
func (s *Supervisor) startServerLocked() error {
	server := NewServer(s.serverConfig, s.languageID)

	// Set up diagnostics forwarding
	if s.diagHandler != nil {
		server.OnDiagnostics(s.diagHandler)
	}

	if err := server.Start(s.ctx, s.folders); err != nil {
		return err
	}

	s.server = server
	s.lastStart = time.Now()

	return nil
}

// monitor watches for server crashes and handles restarts.
// This is the main supervision loop that runs in its own goroutine.
func (s *Supervisor) monitor() {
	for {
		// Get current server
		s.mu.Lock()
		server := s.server
		s.mu.Unlock()

		if server == nil {
			return
		}

		// Wait for exit or cancellation
		select {
		case <-s.ctx.Done():
			return
		case exitErr := <-server.ExitChannel():
			// Server exited - handle crash with retry loop
			if !s.handleCrashWithRetry(exitErr) {
				// Permanently failed or stopped, exit monitor
				return
			}
			// Successfully recovered, continue monitoring
		}
	}
}

// handleCrashWithRetry handles a server crash with retry logic.
// Returns true if server recovered, false if permanently failed or stopped.
func (s *Supervisor) handleCrashWithRetry(initialErr error) bool {
	exitErr := initialErr

	for {
		s.mu.Lock()

		// Check if we were explicitly stopped
		if SupervisorState(s.state.Load()) == SupervisorStateStopped {
			s.mu.Unlock()
			return false
		}

		// Check if server ran long enough to reset counters
		if time.Since(s.lastStart) > s.config.ResetWindow {
			s.restartCount = 0
		}

		s.restartCount++

		// Emit crash event
		s.emitEventLocked(SupervisorEvent{
			Type:       SupervisorEventCrash,
			LanguageID: s.languageID,
			Error:      exitErr,
			Attempt:    s.restartCount,
		})

		// Check if we've exceeded max restarts
		if s.restartCount > s.config.MaxRestarts {
			s.state.Store(int32(SupervisorStateFailed))
			s.emitEventLocked(SupervisorEvent{
				Type:       SupervisorEventFailed,
				LanguageID: s.languageID,
				Error:      exitErr,
				Attempt:    s.restartCount,
			})
			s.mu.Unlock()
			return false
		}

		// Calculate backoff delay
		delay := CalculateBackoff(
			s.restartCount,
			s.config.InitialBackoff,
			s.config.MaxBackoff,
			s.config.BackoffMultiplier,
		)

		// Update state to restarting
		s.state.Store(int32(SupervisorStateRestarting))
		s.emitEventLocked(SupervisorEvent{
			Type:       SupervisorEventRestarting,
			LanguageID: s.languageID,
			Attempt:    s.restartCount,
			NextRetry:  delay,
		})

		s.mu.Unlock()

		// Wait with backoff (without holding lock)
		select {
		case <-s.ctx.Done():
			return false
		case <-time.After(delay):
		}

		// Re-acquire lock for restart attempt
		s.mu.Lock()

		// Check if we were stopped during backoff
		if SupervisorState(s.state.Load()) == SupervisorStateStopped {
			s.mu.Unlock()
			return false
		}

		// Attempt restart
		err := s.startServerLocked()
		if err != nil {
			// Restart failed, continue retry loop with new error
			exitErr = err
			s.mu.Unlock()
			continue
		}

		// Re-sync documents
		s.resyncDocumentsLocked()

		s.state.Store(int32(SupervisorStateRunning))
		s.emitEventLocked(SupervisorEvent{
			Type:       SupervisorEventRecovered,
			LanguageID: s.languageID,
			Attempt:    s.restartCount,
		})

		s.mu.Unlock()
		return true
	}
}

// resyncDocumentsLocked re-opens all documents on the recovered server.
// Must hold mu lock.
func (s *Supervisor) resyncDocumentsLocked() {
	if s.server == nil {
		return
	}

	s.documentsMu.RLock()
	docs := make([]documentState, 0, len(s.documents))
	for _, doc := range s.documents {
		docs = append(docs, doc)
	}
	s.documentsMu.RUnlock()

	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	for _, doc := range docs {
		_ = s.server.OpenDocument(ctx, URIToFilePath(doc.URI), doc.LanguageID, doc.Content)
	}
}

// emitEventLocked sends an event to listeners (must hold mu or be safe to call).
// Events are dropped if channel is full or closed.
func (s *Supervisor) emitEventLocked(event SupervisorEvent) {
	if s.closed.Load() {
		return
	}
	select {
	case s.eventCh <- event:
	default:
		// Channel full, drop event
	}
}

// Stop stops the supervisor and the server.
// ctx must be non-nil; if nil, context.Background() will be used.
func (s *Supervisor) Stop(ctx context.Context) error {
	// Handle nil context defensively
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	state := SupervisorState(s.state.Load())
	if state == SupervisorStateStopped || state == SupervisorStateIdle {
		s.mu.Unlock()
		return nil
	}

	s.state.Store(int32(SupervisorStateStopped))
	server := s.server
	s.server = nil
	s.mu.Unlock()

	// Cancel context to stop monitor
	if s.cancel != nil {
		s.cancel()
	}

	// Close event channel (once)
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		close(s.eventCh)
	})

	// Shutdown server
	if server != nil {
		return server.Shutdown(ctx)
	}

	return nil
}

// State returns the current supervisor state.
func (s *Supervisor) State() SupervisorState {
	return SupervisorState(s.state.Load())
}

// Server returns the current server instance (may be nil during restart).
func (s *Supervisor) Server() *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.server
}

// RestartCount returns the number of restart attempts since the last reset.
func (s *Supervisor) RestartCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restartCount
}

// Events returns the event channel for monitoring supervisor events.
// The channel is closed when the supervisor is stopped.
func (s *Supervisor) Events() <-chan SupervisorEvent {
	return s.eventCh
}

// OnDiagnostics sets a handler for diagnostics notifications.
func (s *Supervisor) OnDiagnostics(handler func(uri DocumentURI, diagnostics []Diagnostic)) {
	s.mu.Lock()
	s.diagHandler = handler
	if s.server != nil {
		s.server.OnDiagnostics(handler)
	}
	s.mu.Unlock()
}

// --- Document State Tracking ---

// TrackDocument records a document's state for recovery.
func (s *Supervisor) TrackDocument(uri DocumentURI, languageID, content string) {
	s.documentsMu.Lock()
	s.documents[uri] = documentState{
		URI:        uri,
		LanguageID: languageID,
		Content:    content,
	}
	s.documentsMu.Unlock()
}

// UpdateDocumentContent updates a tracked document's content.
func (s *Supervisor) UpdateDocumentContent(uri DocumentURI, content string) {
	s.documentsMu.Lock()
	if doc, exists := s.documents[uri]; exists {
		doc.Content = content
		s.documents[uri] = doc
	}
	s.documentsMu.Unlock()
}

// UntrackDocument removes a document from tracking.
func (s *Supervisor) UntrackDocument(uri DocumentURI) {
	s.documentsMu.Lock()
	delete(s.documents, uri)
	s.documentsMu.Unlock()
}

// TrackedDocuments returns the URIs of all tracked documents.
func (s *Supervisor) TrackedDocuments() []DocumentURI {
	s.documentsMu.RLock()
	defer s.documentsMu.RUnlock()

	uris := make([]DocumentURI, 0, len(s.documents))
	for uri := range s.documents {
		uris = append(uris, uri)
	}
	return uris
}

// --- Forwarded Server Methods ---

// OpenDocument opens a document and tracks it for recovery.
func (s *Supervisor) OpenDocument(ctx context.Context, path, languageID, content string) error {
	s.mu.Lock()
	server := s.server
	s.mu.Unlock()

	if server == nil {
		return ErrServerNotReady
	}

	uri := FilePathToURI(path)
	s.TrackDocument(uri, languageID, content)

	return server.OpenDocument(ctx, path, languageID, content)
}

// CloseDocument closes a document and removes it from tracking.
func (s *Supervisor) CloseDocument(ctx context.Context, path string) error {
	s.mu.Lock()
	server := s.server
	s.mu.Unlock()

	if server == nil {
		return ErrServerNotReady
	}

	uri := FilePathToURI(path)
	s.UntrackDocument(uri)

	return server.CloseDocument(ctx, path)
}

// ChangeDocument sends document changes and updates tracking.
func (s *Supervisor) ChangeDocument(ctx context.Context, path string, changes []TextDocumentContentChangeEvent) error {
	s.mu.Lock()
	server := s.server
	s.mu.Unlock()

	if server == nil {
		return ErrServerNotReady
	}

	// Update tracked content (for full sync)
	uri := FilePathToURI(path)
	for _, change := range changes {
		if change.Range == nil {
			s.UpdateDocumentContent(uri, change.Text)
		}
	}

	return server.ChangeDocument(ctx, path, changes)
}

// IsReady returns true if the server is ready to accept requests.
func (s *Supervisor) IsReady() bool {
	state := SupervisorState(s.state.Load())
	if state != SupervisorStateRunning {
		return false
	}

	s.mu.Lock()
	server := s.server
	s.mu.Unlock()

	return server != nil && server.Status() == ServerStatusReady
}

// LanguageID returns the language this supervisor handles.
func (s *Supervisor) LanguageID() string {
	return s.languageID
}

// --- Statistics ---

// SupervisorStats provides statistics about the supervisor.
type SupervisorStats struct {
	State          SupervisorState
	RestartCount   int
	LastStartTime  time.Time
	CurrentBackoff time.Duration
	TrackedDocs    int
}

// Stats returns current supervisor statistics.
func (s *Supervisor) Stats() SupervisorStats {
	s.mu.Lock()
	restartCount := s.restartCount
	lastStart := s.lastStart
	s.mu.Unlock()

	s.documentsMu.RLock()
	docCount := len(s.documents)
	s.documentsMu.RUnlock()

	// Calculate current backoff based on restart count
	currentBackoff := CalculateBackoff(
		restartCount,
		s.config.InitialBackoff,
		s.config.MaxBackoff,
		s.config.BackoffMultiplier,
	)

	return SupervisorStats{
		State:          SupervisorState(s.state.Load()),
		RestartCount:   restartCount,
		LastStartTime:  lastStart,
		CurrentBackoff: currentBackoff,
		TrackedDocs:    docCount,
	}
}

// CalculateBackoff calculates the backoff duration for a given attempt.
// attempt=0 or attempt=1 returns initial, subsequent attempts use exponential growth.
func CalculateBackoff(attempt int, initial, max time.Duration, multiplier float64) time.Duration {
	if attempt <= 1 {
		return initial
	}

	delay := float64(initial) * math.Pow(multiplier, float64(attempt-1))
	if delay > float64(max) {
		return max
	}
	return time.Duration(delay)
}
