package integration

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/dshills/keystorm/internal/event"
	"github.com/dshills/keystorm/internal/event/topic"
)

// IntegrationTopics defines event topics used by the integration layer.
var IntegrationTopics = struct {
	// Manager lifecycle events
	ManagerStarted  topic.Topic
	ManagerStopping topic.Topic
	ManagerStopped  topic.Topic

	// Workspace events
	WorkspaceChanged topic.Topic

	// Terminal events
	TerminalCreated topic.Topic
	TerminalClosed  topic.Topic
	TerminalOutput  topic.Topic
	TerminalTitle   topic.Topic

	// Git events
	GitStatusChanged  topic.Topic
	GitBranchChanged  topic.Topic
	GitCommitCreated  topic.Topic
	GitPullCompleted  topic.Topic
	GitPushCompleted  topic.Topic
	GitStashCreated   topic.Topic
	GitMergeCompleted topic.Topic

	// Debug events
	DebugSessionStarted topic.Topic
	DebugSessionStopped topic.Topic
	DebugBreakpointHit  topic.Topic
	DebugStepCompleted  topic.Topic
	DebugVariableChange topic.Topic
	DebugOutputReceived topic.Topic

	// Task events
	TaskStarted   topic.Topic
	TaskOutput    topic.Topic
	TaskCompleted topic.Topic
	TaskFailed    topic.Topic
	TaskCancelled topic.Topic
}{
	// Manager lifecycle events
	ManagerStarted:  "integration.started",
	ManagerStopping: "integration.stopping",
	ManagerStopped:  "integration.stopped",

	// Workspace events
	WorkspaceChanged: "integration.workspace.changed",

	// Terminal events
	TerminalCreated: "terminal.created",
	TerminalClosed:  "terminal.closed",
	TerminalOutput:  "terminal.output",
	TerminalTitle:   "terminal.title",

	// Git events
	GitStatusChanged:  "git.status.changed",
	GitBranchChanged:  "git.branch.changed",
	GitCommitCreated:  "git.commit.created",
	GitPullCompleted:  "git.pull.completed",
	GitPushCompleted:  "git.push.completed",
	GitStashCreated:   "git.stash.created",
	GitMergeCompleted: "git.merge.completed",

	// Debug events
	DebugSessionStarted: "debug.session.started",
	DebugSessionStopped: "debug.session.stopped",
	DebugBreakpointHit:  "debug.breakpoint.hit",
	DebugStepCompleted:  "debug.step.completed",
	DebugVariableChange: "debug.variable.change",
	DebugOutputReceived: "debug.output",

	// Task events
	TaskStarted:   "task.started",
	TaskOutput:    "task.output",
	TaskCompleted: "task.completed",
	TaskFailed:    "task.failed",
	TaskCancelled: "task.cancelled",
}

// EventBridge connects the typed event.Bus to the integration layer.
// It provides bidirectional event forwarding between the typed event system
// and the integration layer's EventPublisher interface.
type EventBridge struct {
	mu sync.RWMutex

	// The typed event bus
	bus event.Bus

	// The adapter that implements EventPublisher
	adapter *event.BusAdapter

	// Subscriber for receiving events from the typed bus
	subscriber *event.IntegrationSubscriber

	// The integration manager (optional, for receiving events)
	manager *Manager

	// State
	closed   atomic.Bool
	ctx      context.Context
	cancel   context.CancelFunc
	handlers map[topic.Topic]func(map[string]any)
}

// NewEventBridge creates a new bridge between the typed event bus and integration layer.
func NewEventBridge(bus event.Bus) *EventBridge {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventBridge{
		bus:        bus,
		adapter:    event.NewBusAdapter(bus, "integration"),
		subscriber: event.NewIntegrationSubscriber(bus),
		ctx:        ctx,
		cancel:     cancel,
		handlers:   make(map[topic.Topic]func(map[string]any)),
	}
}

// Publisher returns an EventPublisher that publishes to the typed bus.
// Use this to inject into the integration Manager.
func (b *EventBridge) Publisher() EventPublisher {
	return b.adapter
}

// Bus returns the underlying typed event bus.
func (b *EventBridge) Bus() event.Bus {
	return b.bus
}

// SetManager associates an integration manager with the bridge.
// This enables the bridge to forward events from the typed bus to the manager.
func (b *EventBridge) SetManager(m *Manager) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.manager = m
	// Update manager's event bus to use our adapter
	m.SetEventBus(b.adapter)
}

// Subscribe registers a handler for events on a specific topic.
// The handler receives events in the legacy map[string]any format.
func (b *EventBridge) Subscribe(t topic.Topic, handler func(map[string]any)) string {
	if b.closed.Load() {
		return ""
	}

	b.mu.Lock()
	b.handlers[t] = handler
	b.mu.Unlock()

	return b.subscriber.Subscribe(string(t), handler)
}

// SubscribeAll registers handlers for all integration topics.
// This is useful for monitoring all integration events.
func (b *EventBridge) SubscribeAll(handler func(eventType string, data map[string]any)) {
	// Subscribe to all integration topic categories
	patterns := []string{
		"integration.*",
		"terminal.*",
		"git.*",
		"debug.*",
		"task.*",
	}

	for _, pattern := range patterns {
		b.subscriber.Subscribe(pattern, func(data map[string]any) {
			// Extract event type from data if available
			eventType := ""
			if t, ok := data["topic"].(string); ok {
				eventType = t
			}
			handler(eventType, data)
		})
	}
}

// Unsubscribe removes a subscription by ID.
func (b *EventBridge) Unsubscribe(id string) bool {
	return b.subscriber.Unsubscribe(id)
}

// Publish publishes an event directly to the typed bus.
func (b *EventBridge) Publish(eventType string, data map[string]any) {
	b.adapter.Publish(eventType, data)
}

// PublishSync publishes an event synchronously.
func (b *EventBridge) PublishSync(eventType string, data map[string]any) error {
	return b.adapter.PublishSync(eventType, data)
}

// Close shuts down the bridge and releases resources.
func (b *EventBridge) Close() error {
	if b.closed.Swap(true) {
		return nil
	}

	b.cancel()

	b.mu.Lock()
	b.handlers = make(map[topic.Topic]func(map[string]any))
	b.mu.Unlock()

	_ = b.subscriber.Close()
	return b.adapter.Close()
}

// SubscriptionCount returns the number of active subscriptions.
func (b *EventBridge) SubscriptionCount() int {
	return b.subscriber.SubscriptionCount()
}

// AllIntegrationTopics returns all integration topic patterns for subscribing.
func AllIntegrationTopics() []topic.Topic {
	return []topic.Topic{
		IntegrationTopics.ManagerStarted,
		IntegrationTopics.ManagerStopping,
		IntegrationTopics.ManagerStopped,
		IntegrationTopics.WorkspaceChanged,
		IntegrationTopics.TerminalCreated,
		IntegrationTopics.TerminalClosed,
		IntegrationTopics.TerminalOutput,
		IntegrationTopics.TerminalTitle,
		IntegrationTopics.GitStatusChanged,
		IntegrationTopics.GitBranchChanged,
		IntegrationTopics.GitCommitCreated,
		IntegrationTopics.GitPullCompleted,
		IntegrationTopics.GitPushCompleted,
		IntegrationTopics.GitStashCreated,
		IntegrationTopics.GitMergeCompleted,
		IntegrationTopics.DebugSessionStarted,
		IntegrationTopics.DebugSessionStopped,
		IntegrationTopics.DebugBreakpointHit,
		IntegrationTopics.DebugStepCompleted,
		IntegrationTopics.DebugVariableChange,
		IntegrationTopics.DebugOutputReceived,
		IntegrationTopics.TaskStarted,
		IntegrationTopics.TaskOutput,
		IntegrationTopics.TaskCompleted,
		IntegrationTopics.TaskFailed,
		IntegrationTopics.TaskCancelled,
	}
}

// WildcardTopics returns wildcard patterns for integration events.
func WildcardTopics() []topic.Topic {
	return []topic.Topic{
		"integration.*",
		"terminal.*",
		"git.*",
		"debug.*",
		"task.*",
	}
}
