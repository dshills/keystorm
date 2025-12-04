package dispatcher_test

import (
	"sort"
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher"
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	registry := dispatcher.NewRegistry()

	h := handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})

	registry.Register("test.action", h)

	got := registry.Get("test.action")
	if got == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestRegistryGetMissing(t *testing.T) {
	registry := dispatcher.NewRegistry()

	got := registry.Get("missing")
	if got != nil {
		t.Error("expected nil for missing action")
	}
}

func TestRegistryHas(t *testing.T) {
	registry := dispatcher.NewRegistry()

	h := handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})
	registry.Register("test", h)

	if !registry.Has("test") {
		t.Error("expected Has('test') to return true")
	}

	if registry.Has("missing") {
		t.Error("expected Has('missing') to return false")
	}
}

func TestRegistryUnregister(t *testing.T) {
	registry := dispatcher.NewRegistry()

	h := handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})
	registry.Register("test", h)
	registry.Unregister("test")

	if registry.Has("test") {
		t.Error("expected Has('test') to return false after unregister")
	}
}

func TestRegistryUnregisterHandler(t *testing.T) {
	registry := dispatcher.NewRegistry()

	h1 := handler.NewHandlerFuncWithPriority(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithMessage("h1")
	}, 10)

	h2 := handler.NewHandlerFuncWithPriority(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithMessage("h2")
	}, 20)

	registry.Register("test", h1)
	registry.Register("test", h2)

	// h2 should be first (higher priority)
	all := registry.GetAll("test")
	if len(all) != 2 {
		t.Fatalf("expected 2 handlers, got %d", len(all))
	}

	// Unregister h2
	registry.UnregisterHandler("test", h2)

	all = registry.GetAll("test")
	if len(all) != 1 {
		t.Fatalf("expected 1 handler after unregister, got %d", len(all))
	}
}

func TestRegistryPriority(t *testing.T) {
	registry := dispatcher.NewRegistry()

	low := handler.NewHandlerFuncWithPriority(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithMessage("low")
	}, 10)

	high := handler.NewHandlerFuncWithPriority(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithMessage("high")
	}, 100)

	// Register low first, then high
	registry.Register("test", low)
	registry.Register("test", high)

	// Get should return highest priority
	got := registry.Get("test")
	result := got.Handle(input.Action{Name: "test"}, execctx.New())

	if result.Message != "high" {
		t.Errorf("expected high priority handler, got %q", result.Message)
	}
}

func TestRegistryGetAll(t *testing.T) {
	registry := dispatcher.NewRegistry()

	h1 := handler.NewHandlerFuncWithPriority(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}, 10)

	h2 := handler.NewHandlerFuncWithPriority(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}, 20)

	h3 := handler.NewHandlerFuncWithPriority(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}, 15)

	registry.Register("test", h1)
	registry.Register("test", h2)
	registry.Register("test", h3)

	all := registry.GetAll("test")
	if len(all) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(all))
	}

	// Should be sorted by priority (descending)
	priorities := make([]int, len(all))
	for i, h := range all {
		priorities[i] = h.Priority()
	}

	if !sort.SliceIsSorted(priorities, func(i, j int) bool {
		return priorities[i] > priorities[j]
	}) {
		t.Errorf("expected handlers sorted by priority descending, got %v", priorities)
	}
}

func TestRegistryList(t *testing.T) {
	registry := dispatcher.NewRegistry()

	registry.Register("zzz", handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}))
	registry.Register("aaa", handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}))
	registry.Register("mmm", handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}))

	list := registry.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(list))
	}

	// List should be sorted
	if !sort.StringsAreSorted(list) {
		t.Errorf("expected sorted list, got %v", list)
	}
}

func TestRegistryCount(t *testing.T) {
	registry := dispatcher.NewRegistry()

	if registry.Count() != 0 {
		t.Error("expected count 0 for empty registry")
	}

	registry.Register("a", handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}))
	registry.Register("b", handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}))

	if registry.Count() != 2 {
		t.Errorf("expected count 2, got %d", registry.Count())
	}
}

func TestRegistryClear(t *testing.T) {
	registry := dispatcher.NewRegistry()

	registry.Register("a", handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}))
	registry.Register("b", handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	}))

	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", registry.Count())
	}
}
