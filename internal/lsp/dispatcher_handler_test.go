package lsp

import (
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/input"
)

func TestNewHandler(t *testing.T) {
	h := NewHandler()

	if h == nil {
		t.Fatal("expected non-nil handler")
	}

	if h.Namespace() != "lsp" {
		t.Errorf("expected namespace 'lsp', got %q", h.Namespace())
	}

	if h.requestTimeout != 5*time.Second {
		t.Errorf("expected default timeout 5s, got %v", h.requestTimeout)
	}
}

func TestNewHandlerWithOptions(t *testing.T) {
	client := NewClient()
	h := NewHandler(
		WithLSPClient(client),
		WithHandlerTimeout(10*time.Second),
	)

	if h.client != client {
		t.Error("expected client to be set")
	}

	if h.requestTimeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", h.requestTimeout)
	}
}

func TestHandlerSetClient(t *testing.T) {
	h := NewHandler()

	if h.client != nil {
		t.Error("expected nil client initially")
	}

	client := NewClient()
	h.SetClient(client)

	if h.client != client {
		t.Error("expected client to be set")
	}
}

func TestHandlerNamespace(t *testing.T) {
	h := NewHandler()

	if h.Namespace() != "lsp" {
		t.Errorf("expected namespace 'lsp', got %q", h.Namespace())
	}
}

func TestHandlerCanHandle(t *testing.T) {
	h := NewHandler()

	tests := []struct {
		action string
		want   bool
	}{
		{ActionGotoDefinition, true},
		{ActionGotoTypeDefinition, true},
		{ActionGotoImplementation, true},
		{ActionFindReferences, true},
		{ActionHover, true},
		{ActionCompletion, true},
		{ActionSignatureHelp, true},
		{ActionDocumentSymbols, true},
		{ActionWorkspaceSymbols, true},
		{ActionCodeAction, true},
		{ActionApplyCodeEdit, true},
		{ActionFormat, true},
		{ActionFormatRange, true},
		{ActionFormatOnType, true},
		{ActionOrganizeImports, true},
		{ActionRename, true},
		{ActionPrepareRename, true},
		{ActionExtractVariable, true},
		{ActionExtractFunction, true},
		{ActionNextDiagnostic, true},
		{ActionPrevDiagnostic, true},
		{ActionShowDiagnostic, true},
		{ActionRestartServer, true},
		{ActionServerStatus, true},
		{"lsp.unknownAction", false},
		{"other.action", false},
		{"", false},
	}

	for _, tt := range tests {
		got := h.CanHandle(tt.action)
		if got != tt.want {
			t.Errorf("CanHandle(%q) = %v, want %v", tt.action, got, tt.want)
		}
	}
}

func TestHandlerHandleActionWithoutClient(t *testing.T) {
	h := NewHandler()

	// All actions should return error when no client is set
	actions := []string{
		ActionGotoDefinition,
		ActionHover,
		ActionCompletion,
		ActionFormat,
	}

	for _, actionName := range actions {
		result := h.HandleAction(input.Action{Name: actionName}, &execctx.ExecutionContext{})
		if !result.IsError() {
			t.Errorf("HandleAction(%s) expected error without client", actionName)
		}
	}
}

func TestHandlerHandleActionUnknown(t *testing.T) {
	h := NewHandler()

	result := h.HandleAction(input.Action{Name: "lsp.unknown"}, &execctx.ExecutionContext{})
	if !result.IsError() {
		t.Error("expected error for unknown action")
	}
}

func TestListActions(t *testing.T) {
	actions := ListActions()

	if len(actions) == 0 {
		t.Error("expected non-empty action list")
	}

	// Check that all expected actions are present
	expected := map[string]bool{
		ActionGotoDefinition:     true,
		ActionGotoTypeDefinition: true,
		ActionHover:              true,
		ActionCompletion:         true,
		ActionFormat:             true,
		ActionRename:             true,
	}

	for _, action := range actions {
		delete(expected, action)
	}

	if len(expected) > 0 {
		t.Errorf("missing actions: %v", expected)
	}
}

func TestPositionToPoint(t *testing.T) {
	tests := []struct {
		pos      Position
		wantLine uint32
		wantCol  uint32
	}{
		{Position{Line: 0, Character: 0}, 0, 0},
		{Position{Line: 10, Character: 5}, 10, 5},
		{Position{Line: 100, Character: 50}, 100, 50},
	}

	for _, tt := range tests {
		point := positionToPoint(tt.pos)
		if point.Line != tt.wantLine {
			t.Errorf("positionToPoint(%+v).Line = %d, want %d", tt.pos, point.Line, tt.wantLine)
		}
		if point.Column != tt.wantCol {
			t.Errorf("positionToPoint(%+v).Column = %d, want %d", tt.pos, point.Column, tt.wantCol)
		}
	}
}

func TestHandlerEnsureClient(t *testing.T) {
	h := NewHandler()

	// Without client
	err := h.ensureClient()
	if err == nil {
		t.Error("expected error without client")
	}
	if err != ErrNotStarted {
		t.Errorf("expected ErrNotStarted, got %v", err)
	}

	// With client
	h.SetClient(NewClient())
	err = h.ensureClient()
	if err != nil {
		t.Errorf("unexpected error with client: %v", err)
	}
}

func TestHandlerGetContext(t *testing.T) {
	h := NewHandler(WithHandlerTimeout(5 * time.Second))

	ctx, cancel := h.getContext()
	defer cancel()

	if ctx == nil {
		t.Error("expected non-nil context")
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("expected context to have deadline")
	}

	remaining := time.Until(deadline)
	if remaining < 4*time.Second || remaining > 6*time.Second {
		t.Errorf("expected deadline ~5s from now, got %v", remaining)
	}
}

func TestHandlerGetPositionFromContextNilCursors(t *testing.T) {
	h := NewHandler()

	pos := h.getPositionFromContext(&execctx.ExecutionContext{
		Cursors: nil,
	})

	if pos.Line != 0 || pos.Character != 0 {
		t.Errorf("expected zero position for nil cursors, got %+v", pos)
	}
}

func TestHandlerGetFilePath(t *testing.T) {
	h := NewHandler()

	ctx := &execctx.ExecutionContext{
		FilePath: "/test/file.go",
	}

	path := h.getFilePath(ctx)
	if path != "/test/file.go" {
		t.Errorf("expected '/test/file.go', got %q", path)
	}
}

func TestActionConstants(t *testing.T) {
	// Ensure action constants have correct namespace prefix
	actions := []string{
		ActionGotoDefinition,
		ActionGotoTypeDefinition,
		ActionGotoImplementation,
		ActionFindReferences,
		ActionHover,
		ActionCompletion,
		ActionSignatureHelp,
		ActionDocumentSymbols,
		ActionWorkspaceSymbols,
		ActionCodeAction,
		ActionApplyCodeEdit,
		ActionFormat,
		ActionFormatRange,
		ActionFormatOnType,
		ActionOrganizeImports,
		ActionRename,
		ActionPrepareRename,
		ActionExtractVariable,
		ActionExtractFunction,
		ActionNextDiagnostic,
		ActionPrevDiagnostic,
		ActionShowDiagnostic,
		ActionRestartServer,
		ActionServerStatus,
	}

	for _, action := range actions {
		if len(action) < 5 || action[:4] != "lsp." {
			t.Errorf("action %q should start with 'lsp.'", action)
		}
	}
}
