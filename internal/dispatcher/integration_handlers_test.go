package dispatcher

import (
	"testing"

	"github.com/dshills/keystorm/internal/input"
)

func TestSystem_IntegrationHandlers(t *testing.T) {
	sys := NewSystemWithDefaults()
	defer sys.Stop()

	// Verify integration handlers are registered
	t.Run("git handler registered", func(t *testing.T) {
		if sys.GitHandler() == nil {
			t.Error("GitHandler is nil")
		}
		if !sys.CanHandle("git.status") {
			t.Error("git.status action should be handleable")
		}
	})

	t.Run("task handler registered", func(t *testing.T) {
		if sys.TaskHandler() == nil {
			t.Error("TaskHandler is nil")
		}
		if !sys.CanHandle("task.list") {
			t.Error("task.list action should be handleable")
		}
	})

	t.Run("debug handler registered", func(t *testing.T) {
		if sys.DebugHandler() == nil {
			t.Error("DebugHandler is nil")
		}
		if !sys.CanHandle("debug.start") {
			t.Error("debug.start action should be handleable")
		}
	})
}

func TestSystem_IntegrationNamespaces(t *testing.T) {
	sys := NewSystemWithDefaults()
	defer sys.Stop()

	namespaces := sys.ListNamespaces()

	// Check that integration namespaces are present
	expectedNamespaces := []string{"git", "task", "debug"}
	for _, ns := range expectedNamespaces {
		found := false
		for _, registered := range namespaces {
			if registered == ns {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("namespace %q not found in registered namespaces", ns)
		}
	}
}

func TestSystem_GitHandler_NoManager(t *testing.T) {
	sys := NewSystemWithDefaults()
	defer sys.Stop()

	// Dispatch a git action without a manager - should fail gracefully
	action := input.Action{Name: "git.status"}
	result := sys.Dispatch(action)

	// Should return an error since no manager is set
	if result.Status != 2 { // StatusError
		// The handler should report no manager available
		if result.Message == "" && result.Error == nil {
			t.Error("Expected error message when no git manager is set")
		}
	}
}

func TestSystem_TaskHandler_NoManager(t *testing.T) {
	sys := NewSystemWithDefaults()
	defer sys.Stop()

	// Dispatch a task action without a manager - should fail gracefully
	action := input.Action{Name: "task.list"}
	result := sys.Dispatch(action)

	// Should return an error since no manager is set
	if result.Status != 2 { // StatusError
		if result.Message == "" && result.Error == nil {
			t.Error("Expected error message when no task manager is set")
		}
	}
}

func TestSystem_DebugHandler_NoManager(t *testing.T) {
	sys := NewSystemWithDefaults()
	defer sys.Stop()

	// Dispatch a debug action without a manager - should fail gracefully
	action := input.Action{Name: "debug.sessions"}
	result := sys.Dispatch(action)

	// Should return an error since no manager is set
	if result.Status != 2 { // StatusError
		if result.Message == "" && result.Error == nil {
			t.Error("Expected error message when no debug manager is set")
		}
	}
}

func TestSystem_EventPublisher(t *testing.T) {
	sys := NewSystemWithDefaults()
	defer sys.Stop()

	// Initially nil
	if sys.EventPublisher() != nil {
		t.Error("EventPublisher should be nil initially")
	}

	// Create a mock publisher
	mock := &mockEventPublisher{}
	sys.SetEventPublisher(mock)

	if sys.EventPublisher() != mock {
		t.Error("EventPublisher was not set correctly")
	}

	// Publish an event
	sys.PublishEvent("test.event", map[string]any{"key": "value"})

	if len(mock.events) != 1 {
		t.Errorf("got %d events, want 1", len(mock.events))
	}
	if mock.events[0].eventType != "test.event" {
		t.Errorf("event type = %q, want test.event", mock.events[0].eventType)
	}
}

func TestSystem_PublishEvent_NilPublisher(t *testing.T) {
	sys := NewSystemWithDefaults()
	defer sys.Stop()

	// Should not panic when publishing without a publisher
	sys.PublishEvent("test.event", map[string]any{"key": "value"})
}

// mockEventPublisher records published events for testing.
type mockEventPublisher struct {
	events []struct {
		eventType string
		data      map[string]any
	}
}

func (m *mockEventPublisher) Publish(eventType string, data map[string]any) {
	m.events = append(m.events, struct {
		eventType string
		data      map[string]any
	}{eventType, data})
}

func TestSystem_Stats_IncludesIntegrationHandlers(t *testing.T) {
	sys := NewSystemWithDefaults()
	defer sys.Stop()

	stats := sys.Stats()

	// Should have at least 14 namespaces (cursor, motion, editor, mode, operator,
	// search, view, file, window, completion, macro, git, task, debug)
	if stats.NamespaceCount < 14 {
		t.Errorf("NamespaceCount = %d, want at least 14", stats.NamespaceCount)
	}
}
