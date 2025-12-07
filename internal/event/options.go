package event

import "time"

// BusOption configures an event Bus.
type BusOption func(*busConfig)

// busConfig contains configuration for the event bus.
type busConfig struct {
	// asyncQueueSize is the size of the async event queue.
	asyncQueueSize int

	// asyncWorkerCount is the number of async worker goroutines.
	asyncWorkerCount int

	// defaultTimeout is the default handler execution timeout.
	defaultTimeout time.Duration

	// panicHandler is called when a handler panics.
	panicHandler PanicHandler

	// metricsEnabled controls whether metrics are collected.
	metricsEnabled bool
}

// defaultBusConfig returns sensible default configuration.
func defaultBusConfig() busConfig {
	return busConfig{
		asyncQueueSize:   10000,
		asyncWorkerCount: 10,
		defaultTimeout:   5 * time.Second,
		panicHandler:     DefaultPanicHandler,
		metricsEnabled:   true,
	}
}

// WithAsyncQueueSize sets the async event queue size.
func WithAsyncQueueSize(size int) BusOption {
	return func(c *busConfig) {
		if size > 0 {
			c.asyncQueueSize = size
		}
	}
}

// WithAsyncWorkerCount sets the number of async worker goroutines.
func WithAsyncWorkerCount(count int) BusOption {
	return func(c *busConfig) {
		if count > 0 {
			c.asyncWorkerCount = count
		}
	}
}

// WithDefaultTimeout sets the default handler execution timeout.
func WithDefaultTimeout(timeout time.Duration) BusOption {
	return func(c *busConfig) {
		c.defaultTimeout = timeout
	}
}

// WithBusPanicHandler sets the panic handler for the bus.
func WithBusPanicHandler(h PanicHandler) BusOption {
	return func(c *busConfig) {
		if h != nil {
			c.panicHandler = h
		}
	}
}

// WithMetrics enables or disables metrics collection.
func WithMetrics(enabled bool) BusOption {
	return func(c *busConfig) {
		c.metricsEnabled = enabled
	}
}
