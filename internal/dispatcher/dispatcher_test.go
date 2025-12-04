package dispatcher_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/dispatcher"
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

func TestNewWithDefaults(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}

	if d.Registry() == nil {
		t.Error("expected non-nil registry")
	}

	if d.Router() == nil {
		t.Error("expected non-nil router")
	}

	// Metrics should be nil by default
	if d.Metrics() != nil {
		t.Error("expected nil metrics by default")
	}
}

func TestNewWithMetrics(t *testing.T) {
	config := dispatcher.DefaultConfig().WithMetrics()
	d := dispatcher.New(config)

	if d.Metrics() == nil {
		t.Error("expected non-nil metrics when enabled")
	}
}

func TestDispatchNoHandler(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	result := d.Dispatch(input.Action{Name: "unknown.action"})

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for unknown action, got %v", result.Status)
	}
}

func TestRegisterHandler(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	called := false
	d.RegisterHandlerFunc("test.action", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		called = true
		return handler.Success()
	})

	result := d.Dispatch(input.Action{Name: "test.action"})

	if !called {
		t.Error("expected handler to be called")
	}
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
}

func TestRegisterNamespace(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	bnh := handler.NewBaseNamespaceHandler("cursor")
	bnh.Register("cursor.moveDown", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithMessage("moved down")
	})

	d.RegisterNamespace("cursor", bnh)

	result := d.Dispatch(input.Action{Name: "cursor.moveDown"})

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
	if result.Message != "moved down" {
		t.Errorf("expected message 'moved down', got %q", result.Message)
	}
}

func TestUnregisterHandler(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	d.RegisterHandlerFunc("test.action", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	// Should work
	result := d.Dispatch(input.Action{Name: "test.action"})
	if result.Status != handler.StatusOK {
		t.Error("expected handler to work before unregister")
	}

	// Unregister
	d.UnregisterHandler("test.action")

	// Should fail now
	result = d.Dispatch(input.Action{Name: "test.action"})
	if result.Status != handler.StatusError {
		t.Error("expected error after unregister")
	}
}

func TestPreDispatchHook(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	hookCalled := false
	d.RegisterPreHook(dispatcher.PreDispatchFunc(func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		hookCalled = true
		return true
	}))

	d.RegisterHandlerFunc("test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	d.Dispatch(input.Action{Name: "test"})

	if !hookCalled {
		t.Error("expected pre-dispatch hook to be called")
	}
}

func TestPreDispatchHookCancel(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	d.RegisterPreHook(dispatcher.PreDispatchFunc(func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		return false // Cancel
	}))

	handlerCalled := false
	d.RegisterHandlerFunc("test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		handlerCalled = true
		return handler.Success()
	})

	result := d.Dispatch(input.Action{Name: "test"})

	if handlerCalled {
		t.Error("expected handler NOT to be called when hook cancels")
	}
	if result.Status != handler.StatusCancelled {
		t.Errorf("expected StatusCancelled, got %v", result.Status)
	}
}

func TestPostDispatchHook(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	hookCalled := false
	var capturedResult handler.Result
	d.RegisterPostHook(dispatcher.PostDispatchFunc(func(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
		hookCalled = true
		capturedResult = *result
	}))

	d.RegisterHandlerFunc("test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithMessage("done")
	})

	d.Dispatch(input.Action{Name: "test"})

	if !hookCalled {
		t.Error("expected post-dispatch hook to be called")
	}
	if capturedResult.Message != "done" {
		t.Errorf("expected captured message 'done', got %q", capturedResult.Message)
	}
}

func TestDispatchWithCount(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	var capturedCount int
	d.RegisterHandlerFunc("test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		capturedCount = ctx.Count
		return handler.Success()
	})

	d.Dispatch(input.Action{Name: "test", Count: 5})

	if capturedCount != 5 {
		t.Errorf("expected count 5, got %d", capturedCount)
	}
}

func TestDispatchWithContext(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	var capturedMode string
	d.RegisterHandlerFunc("test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		capturedMode = ctx.Mode()
		return handler.Success()
	})

	inputCtx := &input.Context{Mode: "visual"}
	d.DispatchWithContext(input.Action{Name: "test"}, inputCtx)

	if capturedMode != "visual" {
		t.Errorf("expected mode 'visual', got %q", capturedMode)
	}
}

func TestPanicRecovery(t *testing.T) {
	config := dispatcher.DefaultConfig().WithPanicRecovery(true)
	d := dispatcher.New(config)

	d.RegisterHandlerFunc("panic", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		panic("test panic")
	})

	result := d.Dispatch(input.Action{Name: "panic"})

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError after panic, got %v", result.Status)
	}
}

func TestNoPanicRecovery(t *testing.T) {
	config := dispatcher.DefaultConfig().WithPanicRecovery(false)
	d := dispatcher.New(config)

	d.RegisterHandlerFunc("panic", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		panic("test panic")
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate when recovery is disabled")
		}
	}()

	d.Dispatch(input.Action{Name: "panic"})
}

func TestAsyncDispatch(t *testing.T) {
	config := dispatcher.DefaultConfig().WithAsyncDispatch(10)
	d := dispatcher.New(config)

	var callCount int32
	d.RegisterHandlerFunc("async.test", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		atomic.AddInt32(&callCount, 1)
		return handler.Success()
	})

	d.Start()
	defer d.Stop()

	// Send action via channel
	d.Actions() <- input.Action{Name: "async.test"}

	// Wait for result
	select {
	case result := <-d.Results():
		if result.Status != handler.StatusOK {
			t.Errorf("expected StatusOK, got %v", result.Status)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for async result")
	}

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected handler called once, got %d", callCount)
	}
}

func TestMetricsRecording(t *testing.T) {
	config := dispatcher.DefaultConfig().WithMetrics()
	d := dispatcher.New(config)

	d.RegisterHandlerFunc("test.action", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	// Dispatch a few times
	d.Dispatch(input.Action{Name: "test.action"})
	d.Dispatch(input.Action{Name: "test.action"})
	d.Dispatch(input.Action{Name: "test.action"})

	metrics := d.Metrics()
	if metrics.TotalDispatches() != 3 {
		t.Errorf("expected 3 total dispatches, got %d", metrics.TotalDispatches())
	}

	actionStats := metrics.ActionStats("test.action")
	if actionStats == nil {
		t.Fatal("expected action stats for 'test.action'")
	}
	if actionStats.DispatchCount != 3 {
		t.Errorf("expected dispatch count 3, got %d", actionStats.DispatchCount)
	}
}

func TestRouterPrecedence(t *testing.T) {
	d := dispatcher.NewWithDefaults()

	// Register both namespace handler and exact handler
	bnh := handler.NewBaseNamespaceHandler("test")
	bnh.Register("test.action", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithMessage("namespace")
	})
	d.RegisterNamespace("test", bnh)

	d.RegisterHandlerFunc("test.action", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithMessage("exact")
	})

	// Router (namespace) should take precedence
	result := d.Dispatch(input.Action{Name: "test.action"})

	if result.Message != "namespace" {
		t.Errorf("expected namespace handler to take precedence, got message %q", result.Message)
	}
}
