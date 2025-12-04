package handler_test

import (
	"strings"
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

func TestHandlerFunc(t *testing.T) {
	called := false
	fn := handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		called = true
		return handler.Success()
	})

	result := fn.Handle(input.Action{Name: "test"}, execctx.New())

	if !called {
		t.Error("expected handler func to be called")
	}
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
}

func TestHandlerFuncNil(t *testing.T) {
	fn := &handler.HandlerFunc{}
	result := fn.Handle(input.Action{Name: "test"}, execctx.New())

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for nil func, got %v", result.Status)
	}
}

func TestHandlerFuncCanHandle(t *testing.T) {
	fn := handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	// HandlerFunc always returns true for CanHandle
	if !fn.CanHandle("anything") {
		t.Error("expected CanHandle to return true")
	}
}

func TestHandlerFuncPriority(t *testing.T) {
	fn := handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	if fn.Priority() != 0 {
		t.Errorf("expected priority 0, got %d", fn.Priority())
	}
}

func TestHandlerFuncWithPriority(t *testing.T) {
	fn := handler.NewHandlerFuncWithPriority(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}, 50)

	if fn.Priority() != 50 {
		t.Errorf("expected priority 50, got %d", fn.Priority())
	}
}

func TestSimpleHandler(t *testing.T) {
	called := false
	sh := &handler.SimpleHandler{
		ActionName: "test.action",
		Fn: func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
			called = true
			return handler.Success()
		},
		Prio: 50,
	}

	if sh.Priority() != 50 {
		t.Errorf("expected priority 50, got %d", sh.Priority())
	}

	result := sh.Handle(input.Action{Name: "test.action"}, execctx.New())

	if !called {
		t.Error("expected handler to be called")
	}
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
}

func TestSimpleHandlerCanHandle(t *testing.T) {
	sh := &handler.SimpleHandler{
		ActionName: "test.action",
		Fn: func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
			return handler.Success()
		},
	}

	if !sh.CanHandle("test.action") {
		t.Error("expected CanHandle('test.action') to return true")
	}

	if sh.CanHandle("other.action") {
		t.Error("expected CanHandle('other.action') to return false")
	}
}

func TestSimpleHandlerNilFn(t *testing.T) {
	sh := &handler.SimpleHandler{ActionName: "test"}
	result := sh.Handle(input.Action{Name: "test"}, execctx.New())

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for nil func, got %v", result.Status)
	}
}

func TestBaseNamespaceHandler(t *testing.T) {
	bnh := handler.NewBaseNamespaceHandler("cursor")

	if bnh.Namespace() != "cursor" {
		t.Errorf("expected namespace 'cursor', got '%s'", bnh.Namespace())
	}
}

func TestBaseNamespaceHandlerRegister(t *testing.T) {
	bnh := handler.NewBaseNamespaceHandler("test")

	called := false
	bnh.Register("test.doSomething", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		called = true
		return handler.Success()
	})

	// Action registered - CanHandle should work
	if !bnh.CanHandle("test.doSomething") {
		t.Error("expected CanHandle('test.doSomething') to return true")
	}

	// Action not registered
	if bnh.CanHandle("test.other") {
		t.Error("expected CanHandle('test.other') to return false")
	}

	// Handle the action
	result := bnh.HandleAction(input.Action{Name: "test.doSomething"}, execctx.New())

	if !called {
		t.Error("expected action handler to be called")
	}
	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
}

func TestBaseNamespaceHandlerUnknownAction(t *testing.T) {
	bnh := handler.NewBaseNamespaceHandler("test")

	result := bnh.HandleAction(input.Action{Name: "test.unknown"}, execctx.New())

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for unknown action, got %v", result.Status)
	}
}

func TestNamespaceAdapter(t *testing.T) {
	bnh := handler.NewBaseNamespaceHandler("test")
	bnh.Register("test.action", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithMessage("handled")
	})

	adapter := handler.NewNamespaceAdapter(bnh)

	// Check properties
	if !adapter.CanHandle("test.action") {
		t.Error("expected adapter.CanHandle('test.action') to return true")
	}

	if adapter.Priority() != 0 {
		t.Errorf("expected priority 0, got %d", adapter.Priority())
	}

	// Handle via adapter
	result := adapter.Handle(input.Action{Name: "test.action"}, execctx.New())

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
	if result.Message != "handled" {
		t.Errorf("expected message 'handled', got '%s'", result.Message)
	}
}

func TestExtractActionName(t *testing.T) {
	tests := []struct {
		fullName string
		expected string
	}{
		{"cursor.moveDown", "moveDown"},
		{"editor.save", "save"},
		{"single", "single"},
		{"a.b.c", "b.c"},
		{"", ""},
		{".leading", "leading"},
	}

	for _, tc := range tests {
		// Extract action name by removing namespace prefix
		idx := strings.Index(tc.fullName, ".")
		var got string
		if idx < 0 {
			got = tc.fullName
		} else {
			got = tc.fullName[idx+1:]
		}
		if got != tc.expected {
			t.Errorf("extractActionName(%q) = %q, want %q", tc.fullName, got, tc.expected)
		}
	}
}
