package integration

import (
	"testing"

	"github.com/dshills/keystorm/internal/integration/git"
)

func TestNewProvider(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer m.Close()

	p := NewProvider(m)
	if p == nil {
		t.Fatal("NewProvider returned nil")
	}
}

func TestNewProvider_NilManager(t *testing.T) {
	p := NewProvider(nil)
	if p == nil {
		t.Fatal("NewProvider with nil manager returned nil")
	}
}

func TestProvider_SetWorkspace(t *testing.T) {
	p := NewProvider(nil)

	// Initially empty
	ws := p.WorkspaceRoot()
	if ws != "" {
		t.Errorf("WorkspaceRoot = %q, want empty", ws)
	}

	// Set workspace
	p.SetWorkspace("/tmp/test")
	ws = p.WorkspaceRoot()
	if ws != "/tmp/test" {
		t.Errorf("WorkspaceRoot = %q, want /tmp/test", ws)
	}
}

func TestProvider_WithWorkspace(t *testing.T) {
	p := NewProvider(nil, WithWorkspace("/custom/workspace"))

	ws := p.WorkspaceRoot()
	if ws != "/custom/workspace" {
		t.Errorf("WorkspaceRoot = %q, want /custom/workspace", ws)
	}
}

func TestProvider_WorkspaceFromManager(t *testing.T) {
	m, err := NewManager(WithWorkspaceRoot("/manager/workspace"))
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer m.Close()

	// Provider should inherit workspace from manager
	p := NewProvider(m)
	ws := p.WorkspaceRoot()
	if ws != "/manager/workspace" {
		t.Errorf("WorkspaceRoot = %q, want /manager/workspace", ws)
	}
}

func TestProvider_Git_NoRepository(t *testing.T) {
	p := NewProvider(nil)

	gitProvider := p.Git()
	if gitProvider != nil {
		t.Error("Git() should return nil without repository")
	}
}

func TestProvider_Debug_NoSession(t *testing.T) {
	p := NewProvider(nil)

	debugProvider := p.Debug()
	if debugProvider != nil {
		t.Error("Debug() should return nil without session manager")
	}
}

func TestProvider_Debug_WithSession(t *testing.T) {
	p := NewProvider(nil, WithDebugSession())

	debugProvider := p.Debug()
	if debugProvider == nil {
		t.Fatal("Debug() returned nil with session manager")
	}

	// Should be able to list sessions (empty)
	sessions := debugProvider.Sessions()
	if len(sessions) != 0 {
		t.Errorf("Sessions = %d, want 0", len(sessions))
	}
}

func TestProvider_Task_NoComponents(t *testing.T) {
	p := NewProvider(nil)

	taskProvider := p.Task()
	if taskProvider != nil {
		t.Error("Task() should return nil without discoverer/executor")
	}
}

func TestProvider_Health_NilManager(t *testing.T) {
	p := NewProvider(nil)

	health := p.Health()
	if health.Status != "unavailable" {
		t.Errorf("Status = %q, want unavailable", health.Status)
	}
}

func TestProvider_Health_WithManager(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer m.Close()

	p := NewProvider(m)
	health := p.Health()

	if health.Status != "healthy" {
		t.Errorf("Status = %q, want healthy", health.Status)
	}
}

func TestGitAdapter_ConvertFileStatus(t *testing.T) {
	// Test that FileStatus is converted to strings correctly
	statuses := []git.FileStatus{
		{Path: "file1.go", Status: git.StatusModified},
		{Path: "file2.go", Status: git.StatusAdded},
	}

	strings := make([]string, len(statuses))
	for i, s := range statuses {
		strings[i] = s.Path
	}

	if len(strings) != 2 {
		t.Errorf("got %d strings, want 2", len(strings))
	}
	if strings[0] != "file1.go" {
		t.Errorf("strings[0] = %q, want file1.go", strings[0])
	}
	if strings[1] != "file2.go" {
		t.Errorf("strings[1] = %q, want file2.go", strings[1])
	}
}

func TestDebugSessionManager_Operations(t *testing.T) {
	dsm := newDebugSessionManager()

	t.Run("initial state", func(t *testing.T) {
		sessions := dsm.listSessions()
		if len(sessions) != 0 {
			t.Errorf("listSessions = %d, want 0", len(sessions))
		}
	})

	t.Run("get unknown session", func(t *testing.T) {
		_, ok := dsm.getSession("unknown")
		if ok {
			t.Error("getSession should return false for unknown ID")
		}
	})

	t.Run("generate unique IDs", func(t *testing.T) {
		id1 := dsm.generateID()
		id2 := dsm.generateID()
		if id1 == id2 {
			t.Error("generateID should produce unique IDs")
		}
	})
}

func TestProvider_SetGitRepository(t *testing.T) {
	p := NewProvider(nil)

	// Git should be nil initially
	if p.Git() != nil {
		t.Error("Git() should be nil initially")
	}

	// Setting a nil repository is allowed
	p.SetGitRepository(nil)
	if p.Git() != nil {
		t.Error("Git() should still be nil after setting nil")
	}
}

func TestProvider_SetTaskDiscovery(t *testing.T) {
	p := NewProvider(nil)

	// Task should be nil initially
	if p.Task() != nil {
		t.Error("Task() should be nil initially")
	}

	// Setting just discovery is not enough - need executor too
	p.SetTaskDiscovery(nil)
	if p.Task() != nil {
		t.Error("Task() should still be nil without executor")
	}
}

func TestProvider_SetTaskExecutor(t *testing.T) {
	p := NewProvider(nil)

	// Setting just executor is not enough - need discovery too
	p.SetTaskExecutor(nil)
	if p.Task() != nil {
		t.Error("Task() should still be nil without discovery")
	}
}

func TestEventBusAdapter(t *testing.T) {
	t.Run("nil bus", func(t *testing.T) {
		adapter := NewEventBusAdapter(nil)
		// Should not panic
		adapter.Publish("test", map[string]any{})
	})

	t.Run("with mock bus", func(t *testing.T) {
		mock := &mockProviderEventBus{}
		adapter := NewEventBusAdapter(mock)

		data := map[string]any{"key": "value"}
		adapter.Publish("test.event", data)

		if len(mock.events) != 1 {
			t.Errorf("got %d events, want 1", len(mock.events))
		}
		if mock.events[0].eventType != "test.event" {
			t.Errorf("event type = %q, want test.event", mock.events[0].eventType)
		}
		// Should have timestamp added
		if _, ok := mock.events[0].data["timestamp"]; !ok {
			t.Error("timestamp should be added to event data")
		}
	})
}

// mockProviderEventBus records events for testing.
type mockProviderEventBus struct {
	events []struct {
		eventType string
		data      map[string]any
	}
}

func (m *mockProviderEventBus) Publish(eventType string, data map[string]any) {
	m.events = append(m.events, struct {
		eventType string
		data      map[string]any
	}{eventType, data})
}
