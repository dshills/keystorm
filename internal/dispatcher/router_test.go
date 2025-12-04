package dispatcher_test

import (
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher"
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

func TestRouterRegisterNamespace(t *testing.T) {
	router := dispatcher.NewRouter()

	bnh := handler.NewBaseNamespaceHandler("cursor")
	router.RegisterNamespace("cursor", bnh)

	if !router.HasNamespace("cursor") {
		t.Error("expected HasNamespace('cursor') to return true")
	}
}

func TestRouterUnregisterNamespace(t *testing.T) {
	router := dispatcher.NewRouter()

	bnh := handler.NewBaseNamespaceHandler("cursor")
	router.RegisterNamespace("cursor", bnh)
	router.UnregisterNamespace("cursor")

	if router.HasNamespace("cursor") {
		t.Error("expected HasNamespace('cursor') to return false after unregister")
	}
}

func TestRouterRoute(t *testing.T) {
	router := dispatcher.NewRouter()

	bnh := handler.NewBaseNamespaceHandler("cursor")
	bnh.Register("cursor.moveDown", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})
	router.RegisterNamespace("cursor", bnh)

	h := router.Route("cursor.moveDown")
	if h == nil {
		t.Fatal("expected non-nil handler for 'cursor.moveDown'")
	}
}

func TestRouterRouteUnknown(t *testing.T) {
	router := dispatcher.NewRouter()

	h := router.Route("unknown.action")
	if h != nil {
		t.Error("expected nil handler for unknown namespace")
	}
}

func TestRouterRouteFallback(t *testing.T) {
	router := dispatcher.NewRouter()

	fallback := handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success().WithMessage("fallback")
	})
	router.SetFallback(fallback)

	h := router.Route("unknown.action")
	if h == nil {
		t.Fatal("expected fallback handler")
	}

	result := h.Handle(input.Action{Name: "unknown"}, execctx.New())
	if result.Message != "fallback" {
		t.Errorf("expected fallback message, got %q", result.Message)
	}
}

func TestRouterCanRoute(t *testing.T) {
	router := dispatcher.NewRouter()

	bnh := handler.NewBaseNamespaceHandler("cursor")
	bnh.Register("cursor.moveDown", func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})
	router.RegisterNamespace("cursor", bnh)

	if !router.CanRoute("cursor.moveDown") {
		t.Error("expected CanRoute('cursor.moveDown') to return true")
	}

	if router.CanRoute("cursor.unknown") {
		t.Error("expected CanRoute('cursor.unknown') to return false")
	}

	if router.CanRoute("editor.save") {
		t.Error("expected CanRoute('editor.save') to return false")
	}
}

func TestRouterCanRouteWithFallback(t *testing.T) {
	router := dispatcher.NewRouter()

	fallback := handler.NewHandlerFunc(func(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
		return handler.Success()
	})
	router.SetFallback(fallback)

	// With fallback, any action can be routed
	if !router.CanRoute("anything") {
		t.Error("expected CanRoute to return true with fallback")
	}
}

func TestRouterGetNamespaceHandler(t *testing.T) {
	router := dispatcher.NewRouter()

	bnh := handler.NewBaseNamespaceHandler("cursor")
	router.RegisterNamespace("cursor", bnh)

	h := router.GetNamespaceHandler("cursor")
	if h == nil {
		t.Error("expected non-nil namespace handler")
	}
	if h.Namespace() != "cursor" {
		t.Errorf("expected namespace 'cursor', got %q", h.Namespace())
	}

	h2 := router.GetNamespaceHandler("unknown")
	if h2 != nil {
		t.Error("expected nil for unknown namespace")
	}
}

func TestRouterNamespaces(t *testing.T) {
	router := dispatcher.NewRouter()

	router.RegisterNamespace("cursor", handler.NewBaseNamespaceHandler("cursor"))
	router.RegisterNamespace("editor", handler.NewBaseNamespaceHandler("editor"))
	router.RegisterNamespace("buffer", handler.NewBaseNamespaceHandler("buffer"))

	namespaces := router.Namespaces()
	if len(namespaces) != 3 {
		t.Errorf("expected 3 namespaces, got %d", len(namespaces))
	}

	// Check all are present (order not guaranteed)
	found := make(map[string]bool)
	for _, ns := range namespaces {
		found[ns] = true
	}

	for _, expected := range []string{"cursor", "editor", "buffer"} {
		if !found[expected] {
			t.Errorf("expected to find namespace %q", expected)
		}
	}
}

func TestExtractActionName(t *testing.T) {
	tests := []struct {
		fullName string
		expected string
	}{
		{"cursor.moveDown", "moveDown"},
		{"editor.file.save", "file.save"},
		{"simple", "simple"},
		{"", ""},
		{".leading", "leading"},
	}

	for _, tc := range tests {
		got := dispatcher.ExtractActionName(tc.fullName)
		if got != tc.expected {
			t.Errorf("ExtractActionName(%q) = %q, want %q", tc.fullName, got, tc.expected)
		}
	}
}

func TestBuildActionName(t *testing.T) {
	tests := []struct {
		namespace string
		action    string
		expected  string
	}{
		{"cursor", "moveDown", "cursor.moveDown"},
		{"", "save", "save"},
		{"editor", "", "editor."},
	}

	for _, tc := range tests {
		got := dispatcher.BuildActionName(tc.namespace, tc.action)
		if got != tc.expected {
			t.Errorf("BuildActionName(%q, %q) = %q, want %q", tc.namespace, tc.action, got, tc.expected)
		}
	}
}
