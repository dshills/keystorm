package event_test

import (
	"context"
	"fmt"
	"time"

	"github.com/dshills/keystorm/internal/event"
	"github.com/dshills/keystorm/internal/event/events"
	"github.com/dshills/keystorm/internal/event/topic"
)

// Example_basicUsage demonstrates basic event bus operations.
func Example_basicUsage() {
	// Create and start the event bus
	bus := event.NewBus()
	if err := bus.Start(); err != nil {
		fmt.Printf("Failed to start bus: %v\n", err)
		return
	}
	defer bus.Stop(context.Background())

	// Subscribe to buffer events
	_, err := bus.SubscribeFunc(
		topic.Topic("buffer.content.inserted"),
		func(ctx context.Context, e any) error {
			fmt.Println("Buffer content inserted")
			return nil
		},
		event.WithDeliveryMode(event.DeliverySync),
	)
	if err != nil {
		fmt.Printf("Subscribe failed: %v\n", err)
		return
	}

	// Publish an event
	evt := event.NewEvent(
		topic.Topic("buffer.content.inserted"),
		events.BufferContentInserted{
			BufferID: "buf-123",
			Position: events.Position{Line: 1, Column: 0},
			Text:     "Hello, World!",
		},
		"engine",
	)

	if err := bus.PublishSync(context.Background(), evt); err != nil {
		fmt.Printf("Publish failed: %v\n", err)
		return
	}

	// Output: Buffer content inserted
}

// Example_wildcardSubscription shows how to use wildcard patterns.
func Example_wildcardSubscription() {
	bus := event.NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	// Subscribe to all buffer events using wildcard
	_, _ = bus.SubscribeFunc(
		topic.Topic("buffer.*"),
		func(ctx context.Context, e any) error {
			// Extract topic from the event
			if tp, ok := e.(event.TopicProvider); ok {
				fmt.Printf("Buffer event: %s\n", tp.EventTopic())
			}
			return nil
		},
		event.WithDeliveryMode(event.DeliverySync),
	)

	// These will match
	bus.PublishSync(context.Background(), event.NewEvent(
		topic.Topic("buffer.cleared"), struct{}{}, "engine"))
	bus.PublishSync(context.Background(), event.NewEvent(
		topic.Topic("buffer.saved"), struct{}{}, "engine"))

	// This won't match (more than one segment after buffer)
	bus.PublishSync(context.Background(), event.NewEvent(
		topic.Topic("buffer.content.inserted"), struct{}{}, "engine"))

	// Output:
	// Buffer event: buffer.cleared
	// Buffer event: buffer.saved
}

// Example_priorityHandling demonstrates handler priority ordering.
func Example_priorityHandling() {
	bus := event.NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	testTopic := topic.Topic("test.priority")

	// Subscribe with different priorities (in random order)
	_, _ = bus.SubscribeFunc(testTopic, func(ctx context.Context, e any) error {
		fmt.Println("Low priority handler")
		return nil
	}, event.WithPriority(event.PriorityLow), event.WithDeliveryMode(event.DeliverySync))

	_, _ = bus.SubscribeFunc(testTopic, func(ctx context.Context, e any) error {
		fmt.Println("Critical priority handler")
		return nil
	}, event.WithPriority(event.PriorityCritical), event.WithDeliveryMode(event.DeliverySync))

	_, _ = bus.SubscribeFunc(testTopic, func(ctx context.Context, e any) error {
		fmt.Println("Normal priority handler")
		return nil
	}, event.WithPriority(event.PriorityNormal), event.WithDeliveryMode(event.DeliverySync))

	// Publish - handlers execute in priority order
	bus.PublishSync(context.Background(), event.NewEvent(testTopic, struct{}{}, "test"))

	// Output:
	// Critical priority handler
	// Normal priority handler
	// Low priority handler
}

// Example_sourceFiltering demonstrates filtering events by source.
func Example_sourceFiltering() {
	bus := event.NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	// Create a filter that only allows events from "engine" source
	filter := event.FilterBySource("engine")

	// Subscribe with filter
	_, _ = bus.SubscribeFunc(
		topic.Topic("buffer.*"),
		func(ctx context.Context, e any) error {
			fmt.Println("Received event from engine")
			return nil
		},
		event.WithFilter(filter),
		event.WithDeliveryMode(event.DeliverySync),
	)

	// This will be delivered (source is "engine")
	bus.PublishSync(context.Background(), event.NewEvent(
		topic.Topic("buffer.cleared"), struct{}{}, "engine"))

	// This will be filtered out (source is "plugin")
	bus.PublishSync(context.Background(), event.NewEvent(
		topic.Topic("buffer.cleared"), struct{}{}, "plugin"))

	// Output: Received event from engine
}

// Example_integrationBridge shows how to bridge with the integration layer.
func Example_integrationBridge() {
	bus := event.NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	// Create adapter for integration layer
	adapter := event.NewBusAdapter(bus, "integration")
	defer adapter.Close()

	// Subscribe to git events
	_, _ = bus.SubscribeFunc(
		topic.Topic("git.status.changed"),
		func(ctx context.Context, e any) error {
			fmt.Println("Git status changed")
			return nil
		},
		event.WithDeliveryMode(event.DeliverySync),
	)

	// Integration layer publishes using map[string]any format
	// Use PublishSync for synchronous delivery
	adapter.PublishSync("git.status.changed", map[string]any{
		"branch": "main",
		"clean":  true,
	})

	// Output: Git status changed
}

// Example_asyncDelivery demonstrates asynchronous event delivery.
func Example_asyncDelivery() {
	bus := event.NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	done := make(chan struct{})

	// Subscribe with async delivery
	_, _ = bus.SubscribeFunc(
		topic.Topic("async.test"),
		func(ctx context.Context, e any) error {
			fmt.Println("Async handler executed")
			close(done)
			return nil
		},
		event.WithDeliveryMode(event.DeliveryAsync),
	)

	// Publish (returns immediately, handler runs in worker pool)
	bus.Publish(context.Background(), event.NewEvent(
		topic.Topic("async.test"), struct{}{}, "test"))

	// Wait for async handler
	select {
	case <-done:
	case <-time.After(time.Second):
		fmt.Println("Timeout")
	}

	// Output: Async handler executed
}

// Example_multipleSourcesFilter shows filtering by multiple sources.
func Example_multipleSourcesFilter() {
	bus := event.NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	// Create a filter that allows events from "engine" or "renderer"
	filter := event.FilterBySources("engine", "renderer")

	count := 0
	_, _ = bus.SubscribeFunc(
		topic.Topic("test.*"),
		func(ctx context.Context, e any) error {
			count++
			return nil
		},
		event.WithFilter(filter),
		event.WithDeliveryMode(event.DeliverySync),
	)

	// These will pass the filter
	bus.PublishSync(context.Background(), event.NewEvent(
		topic.Topic("test.event"), struct{}{}, "engine"))
	bus.PublishSync(context.Background(), event.NewEvent(
		topic.Topic("test.event"), struct{}{}, "renderer"))

	// This will be filtered out
	bus.PublishSync(context.Background(), event.NewEvent(
		topic.Topic("test.event"), struct{}{}, "plugin"))

	fmt.Printf("Received %d events\n", count)

	// Output: Received 2 events
}

// Example_envelopeHandling demonstrates handling type-erased events.
func Example_envelopeHandling() {
	bus := event.NewBus()
	bus.Start()
	defer bus.Stop(context.Background())

	_, _ = bus.SubscribeFunc(
		topic.Topic("user.action"),
		func(ctx context.Context, e any) error {
			// Convert to envelope for type-erased access
			env := event.ToEnvelope(e)
			fmt.Printf("Event from %s on topic %s\n", env.Metadata.Source, env.Topic)
			return nil
		},
		event.WithDeliveryMode(event.DeliverySync),
	)

	bus.PublishSync(context.Background(), event.NewEvent(
		topic.Topic("user.action"),
		map[string]string{"action": "click"},
		"ui",
	))

	// Output: Event from ui on topic user.action
}
