package hook_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/dispatcher/hook"
	"github.com/dshills/keystorm/internal/input"
)

var errValidationFailed = errors.New("validation failed")

// TestPreDispatchFunc verifies PreDispatchFunc adapter works.
func TestPreDispatchFunc(t *testing.T) {
	called := false
	h := hook.NewPreDispatchFunc("test-pre", 100, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		called = true
		return true
	})

	if h.Name() != "test-pre" {
		t.Errorf("expected name 'test-pre', got %q", h.Name())
	}
	if h.Priority() != 100 {
		t.Errorf("expected priority 100, got %d", h.Priority())
	}

	action := &input.Action{Name: "test"}
	ctx := execctx.New()

	result := h.PreDispatch(action, ctx)
	if !called {
		t.Error("expected PreDispatch to be called")
	}
	if !result {
		t.Error("expected PreDispatch to return true")
	}
}

// TestPostDispatchFunc verifies PostDispatchFunc adapter works.
func TestPostDispatchFunc(t *testing.T) {
	called := false
	h := hook.NewPostDispatchFunc("test-post", 200, func(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
		called = true
	})

	if h.Name() != "test-post" {
		t.Errorf("expected name 'test-post', got %q", h.Name())
	}
	if h.Priority() != 200 {
		t.Errorf("expected priority 200, got %d", h.Priority())
	}

	action := &input.Action{Name: "test"}
	ctx := execctx.New()
	result := handler.Success()

	h.PostDispatch(action, ctx, &result)
	if !called {
		t.Error("expected PostDispatch to be called")
	}
}

// TestManagerPriorityOrdering verifies hooks run in priority order.
func TestManagerPriorityOrdering(t *testing.T) {
	m := hook.NewManager()

	var order []string

	// Register in non-priority order
	m.RegisterPre(hook.NewPreDispatchFunc("low", 10, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		order = append(order, "low")
		return true
	}))
	m.RegisterPre(hook.NewPreDispatchFunc("high", 100, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		order = append(order, "high")
		return true
	}))
	m.RegisterPre(hook.NewPreDispatchFunc("mid", 50, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		order = append(order, "mid")
		return true
	}))

	action := &input.Action{Name: "test"}
	ctx := execctx.New()
	m.RunPreDispatch(action, ctx)

	// High priority should run first
	expected := []string{"high", "mid", "low"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d hooks, got %d", len(expected), len(order))
	}
	for i, name := range expected {
		if order[i] != name {
			t.Errorf("position %d: expected %q, got %q", i, name, order[i])
		}
	}
}

// TestManagerPostHookOrdering verifies post-hooks run in reverse priority order.
func TestManagerPostHookOrdering(t *testing.T) {
	m := hook.NewManager()

	var order []string

	m.RegisterPost(hook.NewPostDispatchFunc("high", 100, func(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
		order = append(order, "high")
	}))
	m.RegisterPost(hook.NewPostDispatchFunc("low", 10, func(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
		order = append(order, "low")
	}))
	m.RegisterPost(hook.NewPostDispatchFunc("mid", 50, func(action *input.Action, ctx *execctx.ExecutionContext, result *handler.Result) {
		order = append(order, "mid")
	}))

	action := &input.Action{Name: "test"}
	ctx := execctx.New()
	result := handler.Success()
	m.RunPostDispatch(action, ctx, &result)

	// Low priority should run first for post-hooks (so high can see final result)
	expected := []string{"low", "mid", "high"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d hooks, got %d", len(expected), len(order))
	}
	for i, name := range expected {
		if order[i] != name {
			t.Errorf("position %d: expected %q, got %q", i, name, order[i])
		}
	}
}

// TestManagerCancel verifies hook cancellation.
func TestManagerCancel(t *testing.T) {
	m := hook.NewManager()

	secondCalled := false

	m.RegisterPre(hook.NewPreDispatchFunc("canceller", 100, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		return false // Cancel
	}))
	m.RegisterPre(hook.NewPreDispatchFunc("second", 50, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		secondCalled = true
		return true
	}))

	action := &input.Action{Name: "test"}
	ctx := execctx.New()
	result := m.RunPreDispatch(action, ctx)

	if result {
		t.Error("expected RunPreDispatch to return false when cancelled")
	}
	if secondCalled {
		t.Error("second hook should not be called after cancellation")
	}
}

// TestManagerUnregister verifies hook removal.
func TestManagerUnregister(t *testing.T) {
	m := hook.NewManager()

	m.RegisterPre(hook.NewPreDispatchFunc("hook1", 100, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		return true
	}))
	m.RegisterPre(hook.NewPreDispatchFunc("hook2", 50, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		return true
	}))

	if m.PreHookCount() != 2 {
		t.Fatalf("expected 2 hooks, got %d", m.PreHookCount())
	}

	// Unregister by name
	removed := m.UnregisterPre("hook1")
	if !removed {
		t.Error("expected hook1 to be removed")
	}

	if m.PreHookCount() != 1 {
		t.Errorf("expected 1 hook after removal, got %d", m.PreHookCount())
	}

	names := m.PreHookNames()
	if len(names) != 1 || names[0] != "hook2" {
		t.Errorf("expected hook2 to remain, got %v", names)
	}

	// Try to remove non-existent
	removed = m.UnregisterPre("nonexistent")
	if removed {
		t.Error("should not remove non-existent hook")
	}
}

// TestManagerReplaceDuplicate verifies duplicate names replace existing.
func TestManagerReplaceDuplicate(t *testing.T) {
	m := hook.NewManager()

	callCount := 0

	m.RegisterPre(hook.NewPreDispatchFunc("test", 100, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		callCount++
		return true
	}))

	// Register again with same name
	m.RegisterPre(hook.NewPreDispatchFunc("test", 200, func(action *input.Action, ctx *execctx.ExecutionContext) bool {
		callCount += 10
		return true
	}))

	if m.PreHookCount() != 1 {
		t.Errorf("expected 1 hook (replaced), got %d", m.PreHookCount())
	}

	action := &input.Action{Name: "test"}
	ctx := execctx.New()
	m.RunPreDispatch(action, ctx)

	if callCount != 10 {
		t.Errorf("expected replacement hook to be called, got count %d", callCount)
	}
}

// TestManagerClear verifies clearing all hooks.
func TestManagerClear(t *testing.T) {
	m := hook.NewManager()

	m.RegisterPre(hook.NewPreDispatchFunc("pre1", 100, nil))
	m.RegisterPre(hook.NewPreDispatchFunc("pre2", 50, nil))
	m.RegisterPost(hook.NewPostDispatchFunc("post1", 100, nil))

	m.Clear()

	if m.PreHookCount() != 0 {
		t.Errorf("expected 0 pre-hooks after clear, got %d", m.PreHookCount())
	}
	if m.PostHookCount() != 0 {
		t.Errorf("expected 0 post-hooks after clear, got %d", m.PostHookCount())
	}
}

// TestCountLimitHook verifies count limiting.
func TestCountLimitHook(t *testing.T) {
	h := hook.NewCountLimitHook(1000)

	if h.Name() != "count-limit" {
		t.Errorf("expected name 'count-limit', got %q", h.Name())
	}

	action := &input.Action{Name: "test"}
	ctx := execctx.New()
	ctx.Count = 5000

	h.PreDispatch(action, ctx)

	if ctx.Count != 1000 {
		t.Errorf("expected count limited to 1000, got %d", ctx.Count)
	}

	// Count below limit should not change
	ctx.Count = 500
	h.PreDispatch(action, ctx)

	if ctx.Count != 500 {
		t.Errorf("expected count unchanged at 500, got %d", ctx.Count)
	}
}

// TestRepeatHook verifies action capture for repeat.
func TestRepeatHook(t *testing.T) {
	h := hook.NewRepeatHook()

	if h.Name() != "repeat" {
		t.Errorf("expected name 'repeat', got %q", h.Name())
	}

	// Initially no action
	action, initialCount := h.LastAction()
	if action != nil || initialCount != 0 {
		t.Error("expected no action initially")
	}

	// Capture a repeatable action
	editAction := &input.Action{Name: "editor.insertText"}
	ctx := execctx.New()
	ctx.Count = 3
	result := handler.Success()

	h.PostDispatch(editAction, ctx, &result)

	captured, capturedCount := h.LastAction()
	if captured == nil {
		t.Fatal("expected action to be captured")
	}
	if captured.Name != "editor.insertText" {
		t.Errorf("expected captured action 'editor.insertText', got %q", captured.Name)
	}
	if capturedCount != 3 {
		t.Errorf("expected captured count 3, got %d", capturedCount)
	}

	// Non-repeatable action should not be captured
	cursorAction := &input.Action{Name: "cursor.moveDown"}
	h.PostDispatch(cursorAction, ctx, &result)

	captured, _ = h.LastAction()
	if captured.Name != "editor.insertText" {
		t.Error("cursor action should not replace edit action")
	}

	// Failed action should not be captured
	failedAction := &input.Action{Name: "editor.delete"}
	failedResult := handler.Errorf("failed")
	h.PostDispatch(failedAction, ctx, &failedResult)

	captured, _ = h.LastAction()
	if captured.Name != "editor.insertText" {
		t.Error("failed action should not be captured")
	}

	// Clear
	h.Clear()
	captured, clearCount := h.LastAction()
	if captured != nil || clearCount != 0 {
		t.Error("expected no action and zero count after clear")
	}
}

// TestAIContextHook verifies change tracking.
func TestAIContextHook(t *testing.T) {
	h := hook.NewAIContextHook(10) // Max 10 changes

	if h.Name() != "ai-context" {
		t.Errorf("expected name 'ai-context', got %q", h.Name())
	}

	// No changes initially
	if len(h.Changes()) != 0 {
		t.Error("expected no changes initially")
	}

	// Action with edits
	action := &input.Action{Name: "editor.insertText"}
	ctx := execctx.New()
	ctx.FilePath = "/test/file.go"
	result := handler.Success().WithEdits([]handler.Edit{
		{
			OldText: "",
			NewText: "hello",
		},
	})

	h.PostDispatch(action, ctx, &result)

	changes := h.Changes()
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	if changes[0].Action != "editor.insertText" {
		t.Errorf("expected action 'editor.insertText', got %q", changes[0].Action)
	}
	if changes[0].FilePath != "/test/file.go" {
		t.Errorf("expected file path '/test/file.go', got %q", changes[0].FilePath)
	}
	if changes[0].NewText != "hello" {
		t.Errorf("expected new text 'hello', got %q", changes[0].NewText)
	}

	// Test max size limiting
	for i := 0; i < 15; i++ {
		r := handler.Success().WithEdits([]handler.Edit{
			{NewText: "x"},
		})
		h.PostDispatch(action, ctx, &r)
	}

	if len(h.Changes()) != 10 {
		t.Errorf("expected max 10 changes, got %d", len(h.Changes()))
	}

	// Test recent changes
	recent := h.RecentChanges(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent changes, got %d", len(recent))
	}

	// Clear
	h.Clear()
	if len(h.Changes()) != 0 {
		t.Error("expected no changes after clear")
	}
}

// TestReadOnlyHook verifies read-only protection.
func TestReadOnlyHook(t *testing.T) {
	h := hook.NewReadOnlyHook()

	if h.Name() != "read-only" {
		t.Errorf("expected name 'read-only', got %q", h.Name())
	}

	// Non-read-only should allow all
	action := &input.Action{Name: "editor.delete"}
	ctx := execctx.New()
	ctx.Input = &input.Context{IsReadOnly: false}

	if !h.PreDispatch(action, ctx) {
		t.Error("expected non-read-only to allow editing")
	}

	// Read-only should block editing
	ctx.Input = &input.Context{IsReadOnly: true}

	if h.PreDispatch(action, ctx) {
		t.Error("expected read-only to block editing")
	}

	// Read-only should allow cursor movement
	cursorAction := &input.Action{Name: "cursor.moveDown"}
	if !h.PreDispatch(cursorAction, ctx) {
		t.Error("expected read-only to allow cursor movement")
	}

	// Read-only should allow mode switching to non-edit modes
	modeAction := &input.Action{Name: "mode.normal"}
	if !h.PreDispatch(modeAction, ctx) {
		t.Error("expected read-only to allow mode.normal")
	}
}

// TestTimingHook verifies timing measurement.
func TestTimingHook(t *testing.T) {
	var recordedAction string
	var recordedDuration time.Duration

	h := hook.NewTimingHook(func(action string, duration time.Duration) {
		recordedAction = action
		recordedDuration = duration
	})

	if h.Name() != "timing" {
		t.Errorf("expected name 'timing', got %q", h.Name())
	}

	action := &input.Action{Name: "test.action"}
	ctx := execctx.New()
	result := handler.Success()

	h.PreDispatch(action, ctx)
	time.Sleep(10 * time.Millisecond)
	h.PostDispatch(action, ctx, &result)

	if recordedAction != "test.action" {
		t.Errorf("expected recorded action 'test.action', got %q", recordedAction)
	}
	if recordedDuration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", recordedDuration)
	}
}

// TestValidationHook verifies custom validation.
func TestValidationHook(t *testing.T) {
	h := hook.NewValidationHook("test-validator", 800, func(action *input.Action, ctx *execctx.ExecutionContext) error {
		if action.Name == "blocked" {
			return errValidationFailed
		}
		return nil
	})

	if h.Name() != "test-validator" {
		t.Errorf("expected name 'test-validator', got %q", h.Name())
	}

	ctx := execctx.New()

	// Valid action
	validAction := &input.Action{Name: "allowed"}
	if !h.PreDispatch(validAction, ctx) {
		t.Error("expected allowed action to pass")
	}

	// Blocked action
	blockedAction := &input.Action{Name: "blocked"}
	if h.PreDispatch(blockedAction, ctx) {
		t.Error("expected blocked action to fail")
	}
}

// TestLoggingHook verifies logging.
func TestLoggingHook(t *testing.T) {
	var logged []string

	h := hook.NewLoggingHook("test-logger", 1000, func(format string, args ...interface{}) {
		logged = append(logged, format)
	})

	if h.Name() != "test-logger" {
		t.Errorf("expected name 'test-logger', got %q", h.Name())
	}

	action := &input.Action{Name: "test"}
	ctx := execctx.New()
	result := handler.Success()

	h.PreDispatch(action, ctx)
	h.PostDispatch(action, ctx, &result)

	if len(logged) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(logged))
	}
}

// TestActionFilterHook verifies action filtering.
func TestActionFilterHook(t *testing.T) {
	h := hook.NewActionFilterHook("test-filter", 500, func(action *input.Action, ctx *execctx.ExecutionContext) (bool, string) {
		if action.Name == "dangerous" {
			return false, "dangerous action blocked"
		}
		return true, ""
	})

	ctx := execctx.New()

	// Safe action
	safeAction := &input.Action{Name: "safe"}
	if !h.PreDispatch(safeAction, ctx) {
		t.Error("expected safe action to be allowed")
	}

	// Dangerous action
	dangerousAction := &input.Action{Name: "dangerous", Args: input.ActionArgs{}}
	if h.PreDispatch(dangerousAction, ctx) {
		t.Error("expected dangerous action to be blocked")
	}

	// Check reason stored
	if dangerousAction.Args.Extra == nil {
		t.Error("expected Extra to be set with reason")
	} else if dangerousAction.Args.Extra["filter_reason"] != "dangerous action blocked" {
		t.Errorf("unexpected reason: %v", dangerousAction.Args.Extra["filter_reason"])
	}
}

// TestHookPriorityConstants verifies priority constant values.
func TestHookPriorityConstants(t *testing.T) {
	// Verify ordering makes sense
	if hook.PriorityAudit <= hook.PriorityCountLimit {
		t.Error("PriorityAudit should be > PriorityCountLimit")
	}
	if hook.PriorityCountLimit <= hook.PriorityValidation {
		t.Error("PriorityCountLimit should be > PriorityValidation")
	}
	if hook.PriorityValidation <= hook.PriorityRepeat {
		t.Error("PriorityValidation should be > PriorityRepeat")
	}
	if hook.PriorityRepeat <= hook.PriorityAIContext {
		t.Error("PriorityRepeat should be > PriorityAIContext")
	}
}
