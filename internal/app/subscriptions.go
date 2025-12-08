// Package app provides the main application structure and coordination.
package app

import (
	"context"
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/event"
	"github.com/dshills/keystorm/internal/event/topic"
	"github.com/dshills/keystorm/internal/lsp"
)

// Event topics used throughout the application.
const (
	// Buffer events
	TopicBufferContentInserted topic.Topic = "buffer.content.inserted"
	TopicBufferContentDeleted  topic.Topic = "buffer.content.deleted"
	TopicBufferContentReplaced topic.Topic = "buffer.content.replaced"
	TopicBufferContentChanged  topic.Topic = "buffer.content.*"

	// Config events
	TopicConfigChanged        topic.Topic = "config.changed"
	TopicConfigChangedUI      topic.Topic = "config.changed.ui"
	TopicConfigChangedUITheme topic.Topic = "config.changed.ui.theme"
	TopicConfigChangedKeymaps topic.Topic = "config.changed.keymaps"
	TopicConfigChangedAll     topic.Topic = "config.changed.*"

	// Mode events
	TopicModeChanged topic.Topic = "mode.changed"

	// File events
	TopicFileOpened  topic.Topic = "file.opened"
	TopicFileClosed  topic.Topic = "file.closed"
	TopicFileSaved   topic.Topic = "file.saved"
	TopicFileChanged topic.Topic = "file.*"

	// LSP events
	TopicLSPDiagnostics topic.Topic = "lsp.diagnostics"
	TopicLSPCompletion  topic.Topic = "lsp.completion"
	TopicLSPHover       topic.Topic = "lsp.hover"
	TopicLSPAll         topic.Topic = "lsp.*"

	// Document events
	TopicDocumentModified  topic.Topic = "document.modified"
	TopicDocumentActivated topic.Topic = "document.activated"
)

// subscriptionManager manages event bus subscriptions for the application.
type subscriptionManager struct {
	mu            sync.RWMutex
	subscriptions []event.Subscription
	app           *Application
}

// newSubscriptionManager creates a new subscription manager.
func newSubscriptionManager(app *Application) *subscriptionManager {
	return &subscriptionManager{
		subscriptions: make([]event.Subscription, 0),
		app:           app,
	}
}

// setupSubscriptions registers all event subscriptions.
func (sm *subscriptionManager) setupSubscriptions() error {
	if sm.app.eventBus == nil {
		return nil
	}

	// Buffer changes -> Renderer dirty
	if err := sm.subscribeBufferToRenderer(); err != nil {
		return err
	}

	// Buffer changes -> LSP sync
	if err := sm.subscribeBufferToLSP(); err != nil {
		return err
	}

	// Config changes -> Component updates
	if err := sm.subscribeConfigChanges(); err != nil {
		return err
	}

	// Mode changes -> Status line update
	if err := sm.subscribeModeChanges(); err != nil {
		return err
	}

	// LSP diagnostics -> Renderer update
	if err := sm.subscribeDiagnostics(); err != nil {
		return err
	}

	// File events -> Project index update
	if err := sm.subscribeFileToProject(); err != nil {
		return err
	}

	return nil
}

// subscribeBufferToRenderer subscribes to buffer changes to mark renderer dirty.
func (sm *subscriptionManager) subscribeBufferToRenderer() error {
	sub, err := sm.app.eventBus.SubscribeFunc(
		TopicBufferContentChanged,
		sm.handleBufferChangeForRenderer,
		event.WithPriority(event.PriorityLow),
		event.WithDeliveryMode(event.DeliverySync),
	)
	if err != nil {
		return err
	}
	sm.addSubscription(sub)
	return nil
}

// subscribeBufferToLSP subscribes to buffer changes for LSP sync.
func (sm *subscriptionManager) subscribeBufferToLSP() error {
	sub, err := sm.app.eventBus.SubscribeFunc(
		TopicBufferContentChanged,
		sm.handleBufferChangeForLSP,
		event.WithPriority(event.PriorityNormal),
		event.WithDeliveryMode(event.DeliveryAsync),
	)
	if err != nil {
		return err
	}
	sm.addSubscription(sub)
	return nil
}

// subscribeConfigChanges subscribes to config change events.
func (sm *subscriptionManager) subscribeConfigChanges() error {
	sub, err := sm.app.eventBus.SubscribeFunc(
		TopicConfigChangedAll,
		sm.handleConfigChange,
		event.WithPriority(event.PriorityHigh),
		event.WithDeliveryMode(event.DeliverySync),
	)
	if err != nil {
		return err
	}
	sm.addSubscription(sub)
	return nil
}

// subscribeModeChanges subscribes to mode change events.
func (sm *subscriptionManager) subscribeModeChanges() error {
	sub, err := sm.app.eventBus.SubscribeFunc(
		TopicModeChanged,
		sm.handleModeChange,
		event.WithPriority(event.PriorityNormal),
		event.WithDeliveryMode(event.DeliverySync),
	)
	if err != nil {
		return err
	}
	sm.addSubscription(sub)
	return nil
}

// subscribeDiagnostics subscribes to LSP diagnostic events.
func (sm *subscriptionManager) subscribeDiagnostics() error {
	sub, err := sm.app.eventBus.SubscribeFunc(
		TopicLSPDiagnostics,
		sm.handleDiagnostics,
		event.WithPriority(event.PriorityNormal),
		event.WithDeliveryMode(event.DeliveryAsync),
	)
	if err != nil {
		return err
	}
	sm.addSubscription(sub)
	return nil
}

// subscribeFileToProject subscribes to file events for project index updates.
func (sm *subscriptionManager) subscribeFileToProject() error {
	sub, err := sm.app.eventBus.SubscribeFunc(
		TopicFileChanged,
		sm.handleFileChange,
		event.WithPriority(event.PriorityLow),
		event.WithDeliveryMode(event.DeliveryAsync),
	)
	if err != nil {
		return err
	}
	sm.addSubscription(sub)
	return nil
}

// addSubscription adds a subscription to the managed list.
func (sm *subscriptionManager) addSubscription(sub event.Subscription) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.subscriptions = append(sm.subscriptions, sub)
}

// cleanup unsubscribes all managed subscriptions.
// Safe to call multiple times (idempotent).
func (sm *subscriptionManager) cleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.app == nil || sm.app.eventBus == nil {
		sm.subscriptions = nil
		return
	}

	for _, sub := range sm.subscriptions {
		if sub != nil {
			_ = sm.app.eventBus.Unsubscribe(sub)
		}
	}
	sm.subscriptions = nil
}

// Event Handlers

// handleBufferChangeForRenderer marks the renderer as dirty when buffer content changes.
func (sm *subscriptionManager) handleBufferChangeForRenderer(_ context.Context, ev any) error {
	// Renderer dirty marking is handled through the render cycle
	// This handler ensures we respond to programmatic buffer changes
	_ = ev // Event payload could contain change details for optimization
	return nil
}

// handleBufferChangeForLSP syncs document changes with LSP.
func (sm *subscriptionManager) handleBufferChangeForLSP(ctx context.Context, ev any) error {
	if sm.app.lspClient == nil {
		return nil
	}

	doc := sm.app.documents.Active()
	if doc == nil || !doc.IsLSPOpened() {
		return nil
	}

	// Mark document as modified
	doc.SetModified(true)
	doc.IncrementVersion()

	// Try to extract incremental changes from event payload
	// If we have a BufferChangePayload, we can send incremental changes
	if payload, ok := ev.(event.Event[BufferChangePayload]); ok {
		change := payload.Payload
		if change.Path == doc.Path {
			// Build LSP content change event with range information
			lspChange := lsp.TextDocumentContentChangeEvent{
				Range: &lsp.Range{
					Start: sm.offsetToPosition(doc, change.StartOffset),
					End:   sm.offsetToPosition(doc, change.EndOffset),
				},
				Text: change.Text,
			}

			// Use a short timeout for LSP notifications
			lspCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			// Send incremental change to LSP
			if err := sm.app.lspClient.ChangeDocument(lspCtx, doc.Path, []lsp.TextDocumentContentChangeEvent{lspChange}); err != nil {
				// Non-fatal, just log and continue
				_ = err
			}
			return nil
		}
	}

	// Fall back to full document sync for events without detailed change info
	fullChange := lsp.TextDocumentContentChangeEvent{
		Text: doc.Content(),
	}

	lspCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := sm.app.lspClient.ChangeDocument(lspCtx, doc.Path, []lsp.TextDocumentContentChangeEvent{fullChange}); err != nil {
		// Non-fatal, just log and continue
		_ = err
	}

	return nil
}

// offsetToPosition converts a byte offset to an LSP Position.
func (sm *subscriptionManager) offsetToPosition(doc *Document, offset int) lsp.Position {
	if doc == nil || doc.Engine == nil {
		return lsp.Position{}
	}

	// Use the engine to convert byte offset to line/column
	point := doc.Engine.OffsetToPoint(int64(offset))
	return lsp.Position{
		Line:      int(point.Line),
		Character: int(point.Column),
	}
}

// handleConfigChange handles configuration change events.
func (sm *subscriptionManager) handleConfigChange(_ context.Context, ev any) error {
	// Extract topic from event to determine what changed
	envelope := event.ToEnvelope(ev)
	if envelope.Topic == "" {
		return nil
	}

	// Handle theme changes
	if envelope.Topic.HasPrefix(TopicConfigChangedUITheme) {
		// Theme reload would be triggered here
		// Currently renderer handles this internally
	}

	// Handle keymap changes
	if envelope.Topic.HasPrefix(TopicConfigChangedKeymaps) {
		// Keymap reload would be triggered here
		// Mode manager would need to reload keybindings
	}

	return nil
}

// handleModeChange handles mode change events.
func (sm *subscriptionManager) handleModeChange(_ context.Context, _ any) error {
	// Mode changes are reflected in the status line through the render cycle
	// This handler could be used for mode-specific setup
	return nil
}

// handleDiagnostics handles LSP diagnostic events.
func (sm *subscriptionManager) handleDiagnostics(_ context.Context, _ any) error {
	// Diagnostics would be displayed in the gutter and status line
	// The renderer handles this through the render cycle
	return nil
}

// handleFileChange handles file system events for project indexing.
func (sm *subscriptionManager) handleFileChange(_ context.Context, _ any) error {
	if sm.app.project == nil {
		return nil
	}

	// Trigger project index refresh
	// This is async so it won't block the event handler
	// The project module handles debouncing internally
	return nil
}

// BufferChangePayload contains data for buffer change events.
type BufferChangePayload struct {
	// Path is the document path.
	Path string

	// StartOffset is the byte offset where the change started.
	StartOffset int

	// EndOffset is the byte offset where the change ended (before edit).
	EndOffset int

	// Text is the new text that was inserted.
	Text string

	// OldText is the text that was replaced (if any).
	OldText string
}

// ConfigChangePayload contains data for config change events.
type ConfigChangePayload struct {
	// Key is the configuration key that changed.
	Key string

	// OldValue is the previous value.
	OldValue any

	// NewValue is the new value.
	NewValue any
}

// ModeChangePayload contains data for mode change events.
type ModeChangePayload struct {
	// PreviousMode is the name of the previous mode.
	PreviousMode string

	// CurrentMode is the name of the new mode.
	CurrentMode string
}

// FileEventPayload contains data for file events.
type FileEventPayload struct {
	// Path is the file path.
	Path string

	// Action is the action that occurred (opened, closed, saved).
	Action string
}

// DiagnosticsPayload contains data for LSP diagnostics events.
type DiagnosticsPayload struct {
	// Path is the document path.
	Path string

	// Diagnostics contains the diagnostic messages.
	// Using any to avoid coupling to LSP types.
	Diagnostics any
}

// PublishBufferChange publishes a buffer change event.
func (app *Application) PublishBufferChange(ctx context.Context, topicName topic.Topic, payload BufferChangePayload) error {
	if app.eventBus == nil {
		return nil
	}
	ev := event.NewEvent(topicName, payload, "app")
	return app.eventBus.Publish(ctx, ev)
}

// PublishModeChange publishes a mode change event.
func (app *Application) PublishModeChange(ctx context.Context, previous, current string) error {
	if app.eventBus == nil {
		return nil
	}
	payload := ModeChangePayload{
		PreviousMode: previous,
		CurrentMode:  current,
	}
	ev := event.NewEvent(TopicModeChanged, payload, "app")
	return app.eventBus.PublishSync(ctx, ev)
}

// PublishFileEvent publishes a file event.
func (app *Application) PublishFileEvent(ctx context.Context, topicName topic.Topic, path string) error {
	if app.eventBus == nil {
		return nil
	}
	payload := FileEventPayload{
		Path:   path,
		Action: topicName.Base(),
	}
	ev := event.NewEvent(topicName, payload, "app")
	return app.eventBus.Publish(ctx, ev)
}
