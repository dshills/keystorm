package lua

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestNewExecutor(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	exec := NewExecutor(L, 10)
	if exec == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if exec.L != L {
		t.Error("Executor has wrong LState")
	}
	if exec.IsClosed() {
		t.Error("New executor should not be closed")
	}
}

func TestNewExecutorDefaultQueueSize(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	exec := NewExecutor(L, 0)
	if exec == nil {
		t.Fatal("NewExecutor returned nil")
	}
	// Queue size of 0 should default to 100
	if cap(exec.queue) != 100 {
		t.Errorf("Expected default queue size 100, got %d", cap(exec.queue))
	}
}

func TestExecutorExecute(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	exec := NewExecutor(L, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start executor in background
	go exec.Run(ctx)
	defer exec.Close()

	// Execute a simple operation
	var executed bool
	err := exec.Execute(ctx, func(L *lua.LState) error {
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !executed {
		t.Error("Lua operation was not executed")
	}
}

func TestExecutorExecuteMultiple(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	exec := NewExecutor(L, 100)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start executor in background
	go exec.Run(ctx)
	defer exec.Close()

	// Execute multiple operations in sequence
	var counter int32
	for i := 0; i < 10; i++ {
		err := exec.Execute(ctx, func(L *lua.LState) error {
			atomic.AddInt32(&counter, 1)
			return nil
		})
		if err != nil {
			t.Fatalf("Execute %d returned error: %v", i, err)
		}
	}

	if counter != 10 {
		t.Errorf("Expected counter to be 10, got %d", counter)
	}
}

func TestExecutorExecuteAsync(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	exec := NewExecutor(L, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start executor in background
	go exec.Run(ctx)
	defer exec.Close()

	// Execute async operation
	var executed int32
	err := exec.ExecuteAsync(func(L *lua.LState) error {
		atomic.StoreInt32(&executed, 1)
		return nil
	})

	if err != nil {
		t.Fatalf("ExecuteAsync returned error: %v", err)
	}

	// Wait for async execution
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&executed) != 1 {
		t.Error("Async operation was not executed")
	}
}

func TestExecutorClose(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	exec := NewExecutor(L, 10)
	ctx := context.Background()

	// Start executor in background
	go exec.Run(ctx)

	// Close the executor
	exec.Close()

	if !exec.IsClosed() {
		t.Error("Executor should be closed")
	}

	// Execute should return error after close
	err := exec.Execute(ctx, func(L *lua.LState) error {
		return nil
	})

	if err != ErrExecutorClosed {
		t.Errorf("Expected ErrExecutorClosed, got %v", err)
	}

	// ExecuteAsync should also return error
	err = exec.ExecuteAsync(func(L *lua.LState) error {
		return nil
	})

	if err != ErrExecutorClosed {
		t.Errorf("Expected ErrExecutorClosed from ExecuteAsync, got %v", err)
	}
}

func TestExecutorCloseIdempotent(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	exec := NewExecutor(L, 10)

	// Close multiple times should not panic
	exec.Close()
	exec.Close()
	exec.Close()

	if !exec.IsClosed() {
		t.Error("Executor should be closed")
	}
}

func TestExecutorConcurrentAccess(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	exec := NewExecutor(L, 100)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start executor in background
	go exec.Run(ctx)
	defer exec.Close()

	// Multiple goroutines submitting work
	var wg sync.WaitGroup
	var counter int32
	numGoroutines := 10
	opsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				err := exec.Execute(ctx, func(L *lua.LState) error {
					// This should be safe because all ops run on executor's goroutine
					atomic.AddInt32(&counter, 1)
					return nil
				})
				if err != nil {
					t.Errorf("Execute error: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	expected := int32(numGoroutines * opsPerGoroutine)
	if counter != expected {
		t.Errorf("Expected counter to be %d, got %d", expected, counter)
	}
}

func TestExecutorContextCancellation(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	exec := NewExecutor(L, 10)
	ctx, cancel := context.WithCancel(context.Background())

	// Start executor in background
	go exec.Run(ctx)
	defer exec.Close()

	// Execute a successful operation first
	err := exec.Execute(ctx, func(L *lua.LState) error {
		return nil
	})
	if err != nil {
		t.Fatalf("First execute failed: %v", err)
	}

	// Cancel context
	cancel()

	// Give executor time to notice cancellation
	time.Sleep(50 * time.Millisecond)

	// Execute should fail with context error
	err = exec.Execute(ctx, func(L *lua.LState) error {
		return nil
	})
	if err == nil {
		t.Error("Expected error after context cancellation")
	}
}

func TestExecutorPanicRecovery(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	exec := NewExecutor(L, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start executor in background
	go exec.Run(ctx)
	defer exec.Close()

	// Execute operation that panics
	err := exec.Execute(ctx, func(L *lua.LState) error {
		panic("test panic")
	})

	if err == nil {
		t.Fatal("Expected error from panic")
	}

	// Executor should still be functional after panic recovery
	var executed bool
	err = exec.Execute(ctx, func(L *lua.LState) error {
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("Execute after panic failed: %v", err)
	}
	if !executed {
		t.Error("Operation after panic was not executed")
	}
}
