package event

import (
	"context"
	"sync/atomic"

	"github.com/dshills/keystorm/internal/event/dispatch"
	"github.com/dshills/keystorm/internal/event/topic"
)

// Bus is the central event bus interface.
type Bus interface {
	// Publishing
	Publish(ctx context.Context, event any) error
	PublishSync(ctx context.Context, event any) error
	PublishAsync(ctx context.Context, event any) error

	// Subscription
	Subscribe(topicPattern topic.Topic, handler Handler, opts ...SubscriptionOption) (Subscription, error)
	SubscribeFunc(topicPattern topic.Topic, fn HandlerFunc, opts ...SubscriptionOption) (Subscription, error)
	Unsubscribe(sub Subscription) error

	// Lifecycle
	Start() error
	Stop(ctx context.Context) error
	Pause()
	Resume()

	// Status
	Stats() Stats
	IsRunning() bool
	IsPaused() bool
}

// bus is the default Bus implementation.
type bus struct {
	// Subscription management
	registry *Registry

	// Dispatchers
	syncDispatcher  *dispatch.SyncDispatcher
	asyncDispatcher *dispatch.AsyncDispatcher

	// State
	running atomic.Bool
	paused  atomic.Bool

	// Configuration
	config busConfig

	// Stats
	eventsPublished  atomic.Uint64
	eventsDelivered  atomic.Uint64
	eventsDropped    atomic.Uint64
	handlersExecuted atomic.Uint64
	handlerErrors    atomic.Uint64
	handlerPanics    atomic.Uint64
	totalDeliveryNs  atomic.Int64
}

// NewBus creates a new event bus with the given options.
func NewBus(opts ...BusOption) Bus {
	config := defaultBusConfig()
	for _, opt := range opts {
		opt(&config)
	}

	// Create panic handler wrapper for dispatch package.
	// dispatch.PanicHandler has signature: func(event any, panicValue any, stack []byte)
	// event.PanicHandler has signature: func(event any, handler Handler, recovered any)
	// We adapt the dispatch signature to call the event panic handler.
	dispatchPanicHandler := func(event any, panicValue any, _ []byte) {
		if config.panicHandler != nil {
			config.panicHandler(event, nil, panicValue)
		}
	}

	b := &bus{
		registry: NewRegistry(),
		config:   config,
	}

	b.syncDispatcher = dispatch.NewSyncDispatcher(
		dispatch.WithPanicHandler(dispatchPanicHandler),
	)

	b.asyncDispatcher = dispatch.NewAsyncDispatcher(
		dispatch.WithQueueSize(config.asyncQueueSize),
		dispatch.WithWorkerCount(config.asyncWorkerCount),
		dispatch.WithAsyncTimeout(config.defaultTimeout),
		dispatch.WithAsyncPanicHandler(dispatchPanicHandler),
	)

	return b
}

// Start starts the event bus.
func (b *bus) Start() error {
	if b.running.Load() {
		return ErrBusAlreadyRunning
	}
	if err := b.asyncDispatcher.Start(); err != nil {
		return err
	}
	b.running.Store(true)
	return nil
}

// Stop stops the event bus gracefully.
// It waits for all pending async events to be processed or until the context is cancelled.
func (b *bus) Stop(ctx context.Context) error {
	if !b.running.Swap(false) {
		return ErrBusNotRunning
	}
	return b.asyncDispatcher.Stop(ctx)
}

// Pause temporarily stops event delivery.
// Events can still be published but will not be delivered to handlers.
func (b *bus) Pause() {
	b.paused.Store(true)
}

// Resume restarts event delivery after a pause.
func (b *bus) Resume() {
	b.paused.Store(false)
}

// IsRunning returns true if the bus is running.
func (b *bus) IsRunning() bool {
	return b.running.Load()
}

// IsPaused returns true if the bus is paused.
func (b *bus) IsPaused() bool {
	return b.paused.Load()
}

// Publish sends an event using the default delivery mode (async).
func (b *bus) Publish(ctx context.Context, event any) error {
	return b.PublishAsync(ctx, event)
}

// PublishSync sends an event synchronously.
// The call blocks until all sync handlers complete.
func (b *bus) PublishSync(ctx context.Context, event any) error {
	if !b.running.Load() {
		return ErrBusNotRunning
	}
	if b.paused.Load() {
		return nil // Silently drop when paused
	}

	eventTopic := b.extractTopic(event)
	if eventTopic == "" {
		return ErrInvalidEvent
	}

	// Get matching subscriptions
	subs := b.registry.MatchActive(eventTopic)
	if len(subs) == 0 {
		return nil // No subscribers
	}

	// Update metrics
	b.eventsPublished.Add(1)

	// Dispatch to sync handlers
	for _, sub := range subs {
		if sub.Config().DeliveryMode != DeliverySync {
			continue
		}
		if !sub.ShouldDeliver(event) {
			continue
		}

		result := b.syncDispatcher.Dispatch(ctx, event, sub.Handler())
		b.handlersExecuted.Add(1)

		switch {
		case result.Panicked:
			b.handlerPanics.Add(1)
		case result.Error != nil:
			b.handlerErrors.Add(1)
		case result.Success:
			b.eventsDelivered.Add(1)
		}

		b.totalDeliveryNs.Add(result.Duration.Nanoseconds())

		// Handle one-time subscriptions
		if sub.Config().Once && result.Success {
			sub.Cancel()
			b.registry.Remove(sub.ID())
		}
	}

	return nil
}

// PublishAsync queues an event for asynchronous delivery.
func (b *bus) PublishAsync(ctx context.Context, event any) error {
	if !b.running.Load() {
		return ErrBusNotRunning
	}
	if b.paused.Load() {
		return nil // Silently drop when paused
	}

	eventTopic := b.extractTopic(event)
	if eventTopic == "" {
		return ErrInvalidEvent
	}

	subs := b.registry.MatchActive(eventTopic)
	if len(subs) == 0 {
		return nil // No subscribers
	}

	b.eventsPublished.Add(1)

	// Queue for async handlers
	for _, sub := range subs {
		if sub.Config().DeliveryMode != DeliveryAsync {
			continue
		}
		if !sub.ShouldDeliver(event) {
			continue
		}

		err := b.asyncDispatcher.Enqueue(ctx, event, sub.Handler())
		if err != nil {
			b.eventsDropped.Add(1)
			// Queue full - event dropped, but continue trying other handlers
		}
	}

	return nil
}

// Subscribe creates a new subscription for the given topic pattern.
// This method is safe to call concurrently.
func (b *bus) Subscribe(topicPattern topic.Topic, handler Handler, opts ...SubscriptionOption) (Subscription, error) {
	if handler == nil {
		return nil, ErrNilHandler
	}
	if topicPattern == "" {
		return nil, ErrInvalidTopic
	}

	sub := newSubscription(generateID(), topicPattern, handler, opts...)
	b.registry.Add(sub) // Registry is thread-safe

	return sub, nil
}

// SubscribeFunc is a convenience method for subscribing with a function handler.
func (b *bus) SubscribeFunc(topicPattern topic.Topic, fn HandlerFunc, opts ...SubscriptionOption) (Subscription, error) {
	return b.Subscribe(topicPattern, fn, opts...)
}

// Unsubscribe removes a subscription.
// This method is safe to call concurrently.
func (b *bus) Unsubscribe(sub Subscription) error {
	if sub == nil {
		return ErrInvalidSubscription
	}

	sub.Cancel()
	removed := b.registry.Remove(sub.ID()) // Registry is thread-safe

	if !removed {
		return ErrSubscriptionNotFound
	}

	return nil
}

// Stats returns current bus statistics.
func (b *bus) Stats() Stats {
	asyncStats := b.asyncDispatcher.Stats()
	syncStats := b.syncDispatcher.Stats()

	// Combine handler execution stats from both dispatchers
	handlersExecuted := b.handlersExecuted.Load() + asyncStats.Processed
	handlerErrors := b.handlerErrors.Load() + asyncStats.Failed
	handlerPanics := b.handlerPanics.Load() + asyncStats.Panicked + syncStats.Panicked

	totalDeliveryNs := b.totalDeliveryNs.Load() + int64(asyncStats.TotalDuration)
	var avgNs int64
	if handlersExecuted > 0 {
		avgNs = totalDeliveryNs / int64(handlersExecuted)
	}

	return Stats{
		EventsPublished:   b.eventsPublished.Load(),
		EventsDelivered:   b.eventsDelivered.Load() + asyncStats.Succeeded,
		EventsDropped:     b.eventsDropped.Load() + asyncStats.Dropped,
		HandlersExecuted:  handlersExecuted,
		HandlerErrors:     handlerErrors,
		HandlerPanics:     handlerPanics,
		AvgDeliveryTimeNs: avgNs,
		ActiveSubscribers: b.registry.CountActive(),
		QueueDepth:        asyncStats.QueueDepth,
	}
}

// extractTopic extracts the topic from an event.
func (b *bus) extractTopic(event any) topic.Topic {
	// First try TopicProvider interface
	if tp, ok := event.(TopicProvider); ok {
		return tp.EventTopic()
	}

	// Try Envelope
	if env, ok := event.(Envelope); ok {
		return env.Topic
	}

	// Cannot determine topic
	return ""
}
