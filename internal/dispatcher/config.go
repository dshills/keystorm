package dispatcher

import "time"

// Config holds dispatcher configuration options.
type Config struct {
	// AsyncDispatch enables asynchronous action dispatch via channels.
	AsyncDispatch bool

	// ActionBufferSize is the buffer size for the async action channel.
	// Only used when AsyncDispatch is true.
	ActionBufferSize int

	// EnableMetrics enables dispatch timing and statistics collection.
	EnableMetrics bool

	// RecoverFromPanic wraps handler execution in panic recovery.
	RecoverFromPanic bool

	// DefaultTimeout is the default timeout for handler execution.
	// Zero means no timeout.
	DefaultTimeout time.Duration

	// MaxRepeatCount limits the maximum repeat count for actions.
	// Zero means no limit.
	MaxRepeatCount int
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		AsyncDispatch:    false,
		ActionBufferSize: 100,
		EnableMetrics:    false,
		RecoverFromPanic: true,
		DefaultTimeout:   0,
		MaxRepeatCount:   10000,
	}
}

// WithAsyncDispatch returns a copy of the config with async dispatch enabled.
func (c Config) WithAsyncDispatch(bufferSize int) Config {
	c.AsyncDispatch = true
	if bufferSize > 0 {
		c.ActionBufferSize = bufferSize
	}
	return c
}

// WithMetrics returns a copy of the config with metrics enabled.
func (c Config) WithMetrics() Config {
	c.EnableMetrics = true
	return c
}

// WithPanicRecovery returns a copy of the config with panic recovery set.
func (c Config) WithPanicRecovery(recover bool) Config {
	c.RecoverFromPanic = recover
	return c
}

// WithTimeout returns a copy of the config with the default timeout set.
func (c Config) WithTimeout(timeout time.Duration) Config {
	c.DefaultTimeout = timeout
	return c
}

// WithMaxRepeatCount returns a copy of the config with the max repeat count set.
func (c Config) WithMaxRepeatCount(max int) Config {
	c.MaxRepeatCount = max
	return c
}
