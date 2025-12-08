package lua

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	lua "github.com/yuin/gopher-lua"
)

// ErrExecutorClosed is returned when attempting to use a closed executor.
var ErrExecutorClosed = errors.New("lua executor is closed")

// LuaCall represents a Lua operation to be executed.
type LuaCall struct {
	// Fn is the function to execute on the Lua state.
	// It receives the LState and should perform all Lua operations.
	Fn func(L *lua.LState) error

	// Result channel receives the result of the operation.
	// The channel is closed after the result is sent.
	Result chan error
}

// Executor serializes all Lua operations through a single goroutine.
//
// gopher-lua's LState is NOT goroutine-safe. All LState operations must occur
// on a single goroutine. The Executor provides a channel-based mechanism to
// marshal Lua operations from multiple goroutines to a single worker goroutine.
//
// Usage:
//
//	exec := NewExecutor(L)
//	go exec.Run(ctx)
//	defer exec.Close()
//
//	// From any goroutine:
//	err := exec.Execute(ctx, func(L *lua.LState) error {
//	    L.Push(handler)
//	    L.Push(args)
//	    return L.PCall(1, 0, nil)
//	})
type Executor struct {
	L      *lua.LState
	queue  chan *LuaCall
	closed atomic.Bool
	done   chan struct{}

	// closeOnce ensures Close is only called once
	closeOnce sync.Once
}

// NewExecutor creates a new Executor for the given Lua state.
// The queue size determines how many operations can be buffered.
func NewExecutor(L *lua.LState, queueSize int) *Executor {
	if queueSize <= 0 {
		queueSize = 100
	}
	return &Executor{
		L:     L,
		queue: make(chan *LuaCall, queueSize),
		done:  make(chan struct{}),
	}
}

// Run processes Lua operations from the queue.
// This method blocks until the context is cancelled or Close is called.
// MUST be called from the goroutine that owns the Lua state.
func (e *Executor) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			e.drainQueue(ctx.Err())
			return
		case <-e.done:
			e.drainQueue(ErrExecutorClosed)
			return
		case call, ok := <-e.queue:
			if !ok {
				return
			}
			// Execute the Lua operation
			err := e.executeCall(call)
			// Send result (non-blocking since buffer is 1)
			select {
			case call.Result <- err:
			default:
			}
			close(call.Result)
		}
	}
}

// executeCall runs a single Lua operation with panic recovery.
func (e *Executor) executeCall(call *LuaCall) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			default:
				err = errors.New("lua panic")
			}
		}
	}()
	return call.Fn(e.L)
}

// drainQueue drains remaining calls from the queue with the given error.
func (e *Executor) drainQueue(err error) {
	for {
		select {
		case call, ok := <-e.queue:
			if !ok {
				return
			}
			select {
			case call.Result <- err:
			default:
			}
			close(call.Result)
		default:
			return
		}
	}
}

// Execute runs a Lua operation synchronously.
// The operation is queued and executed on the executor's goroutine.
// This method blocks until the operation completes or the context is cancelled.
//
// The fn function receives the LState and should perform all necessary Lua
// operations. The function is executed on the executor's goroutine, ensuring
// thread-safe access to the LState.
func (e *Executor) Execute(ctx context.Context, fn func(L *lua.LState) error) error {
	if e.closed.Load() {
		return ErrExecutorClosed
	}

	call := &LuaCall{
		Fn:     fn,
		Result: make(chan error, 1),
	}

	// Try to enqueue the call
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return ErrExecutorClosed
	case e.queue <- call:
		// Call enqueued, wait for result
	}

	// Wait for result
	select {
	case <-ctx.Done():
		// Context cancelled while waiting for result
		// The call is already queued and will be processed, but we don't wait
		return ctx.Err()
	case err, ok := <-call.Result:
		if !ok {
			return ErrExecutorClosed
		}
		return err
	}
}

// ExecuteAsync queues a Lua operation without waiting for completion.
// This is useful for fire-and-forget operations like event handlers.
// Returns ErrExecutorClosed if the executor is closed.
func (e *Executor) ExecuteAsync(fn func(L *lua.LState) error) error {
	if e.closed.Load() {
		return ErrExecutorClosed
	}

	call := &LuaCall{
		Fn:     fn,
		Result: make(chan error, 1),
	}

	select {
	case <-e.done:
		return ErrExecutorClosed
	case e.queue <- call:
		// Ignore result - fire and forget
		go func() {
			<-call.Result // Drain result to prevent goroutine leak
		}()
		return nil
	default:
		// Queue is full - drop the call
		return errors.New("lua executor queue full")
	}
}

// Close stops the executor and prevents new operations.
// In-flight operations will complete with ErrExecutorClosed.
func (e *Executor) Close() {
	e.closeOnce.Do(func() {
		e.closed.Store(true)
		close(e.done)
	})
}

// IsClosed returns true if the executor has been closed.
func (e *Executor) IsClosed() bool {
	return e.closed.Load()
}
