package lsp

import (
	"testing"
	"time"
)

func TestDefaultSupervisorConfig(t *testing.T) {
	config := DefaultSupervisorConfig()

	if config.MaxRestarts != 5 {
		t.Errorf("expected MaxRestarts 5, got %d", config.MaxRestarts)
	}

	if config.InitialBackoff != 1*time.Second {
		t.Errorf("expected InitialBackoff 1s, got %v", config.InitialBackoff)
	}

	if config.MaxBackoff != 60*time.Second {
		t.Errorf("expected MaxBackoff 60s, got %v", config.MaxBackoff)
	}

	if config.BackoffMultiplier != 2.0 {
		t.Errorf("expected BackoffMultiplier 2.0, got %v", config.BackoffMultiplier)
	}

	if config.ResetWindow != 5*time.Minute {
		t.Errorf("expected ResetWindow 5m, got %v", config.ResetWindow)
	}
}

func TestNewSupervisor(t *testing.T) {
	config := ServerConfig{
		Command: "test-server",
	}
	supervisorConfig := DefaultSupervisorConfig()

	supervisor := NewSupervisor(config, "go", supervisorConfig)

	if supervisor == nil {
		t.Fatal("expected non-nil supervisor")
	}

	if supervisor.LanguageID() != "go" {
		t.Errorf("expected language ID 'go', got %q", supervisor.LanguageID())
	}

	if supervisor.State() != SupervisorStateIdle {
		t.Errorf("expected state Idle, got %v", supervisor.State())
	}

	if supervisor.RestartCount() != 0 {
		t.Errorf("expected restart count 0, got %d", supervisor.RestartCount())
	}
}

func TestSupervisorState_String(t *testing.T) {
	tests := []struct {
		state    SupervisorState
		expected string
	}{
		{SupervisorStateIdle, "idle"},
		{SupervisorStateRunning, "running"},
		{SupervisorStateRestarting, "restarting"},
		{SupervisorStateFailed, "failed"},
		{SupervisorStateStopped, "stopped"},
		{SupervisorState(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.expected {
			t.Errorf("SupervisorState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestSupervisorEventType_String(t *testing.T) {
	tests := []struct {
		eventType SupervisorEventType
		expected  string
	}{
		{SupervisorEventCrash, "crash"},
		{SupervisorEventRestarting, "restarting"},
		{SupervisorEventRecovered, "recovered"},
		{SupervisorEventFailed, "failed"},
		{SupervisorEventType(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.eventType.String()
		if got != tt.expected {
			t.Errorf("SupervisorEventType(%d).String() = %q, want %q", tt.eventType, got, tt.expected)
		}
	}
}

func TestCalculateBackoff(t *testing.T) {
	initial := 1 * time.Second
	max := 60 * time.Second
	multiplier := 2.0

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 32 * time.Second},
		{7, 60 * time.Second}, // Capped at max
		{10, 60 * time.Second},
	}

	for _, tt := range tests {
		got := CalculateBackoff(tt.attempt, initial, max, multiplier)
		if got != tt.expected {
			t.Errorf("CalculateBackoff(%d, %v, %v, %v) = %v, want %v",
				tt.attempt, initial, max, multiplier, got, tt.expected)
		}
	}
}

func TestSupervisorDocumentTracking(t *testing.T) {
	config := ServerConfig{
		Command: "test-server",
	}
	supervisorConfig := DefaultSupervisorConfig()
	supervisor := NewSupervisor(config, "go", supervisorConfig)

	uri := DocumentURI("file:///test/file.go")

	// Track document
	supervisor.TrackDocument(uri, "go", "package main")

	docs := supervisor.TrackedDocuments()
	if len(docs) != 1 {
		t.Errorf("expected 1 tracked document, got %d", len(docs))
	}

	if docs[0] != uri {
		t.Errorf("expected URI %q, got %q", uri, docs[0])
	}

	// Update document content
	supervisor.UpdateDocumentContent(uri, "package main\n\nfunc main() {}")

	// Untrack document
	supervisor.UntrackDocument(uri)

	docs = supervisor.TrackedDocuments()
	if len(docs) != 0 {
		t.Errorf("expected 0 tracked documents after untrack, got %d", len(docs))
	}
}

func TestSupervisorStats(t *testing.T) {
	config := ServerConfig{
		Command: "test-server",
	}
	supervisorConfig := DefaultSupervisorConfig()
	supervisor := NewSupervisor(config, "go", supervisorConfig)

	// Track some documents
	supervisor.TrackDocument("file:///a.go", "go", "package a")
	supervisor.TrackDocument("file:///b.go", "go", "package b")

	stats := supervisor.Stats()

	if stats.State != SupervisorStateIdle {
		t.Errorf("expected state Idle, got %v", stats.State)
	}

	if stats.RestartCount != 0 {
		t.Errorf("expected restart count 0, got %d", stats.RestartCount)
	}

	if stats.TrackedDocs != 2 {
		t.Errorf("expected 2 tracked docs, got %d", stats.TrackedDocs)
	}
}

func TestSupervisorIsReadyBeforeStart(t *testing.T) {
	config := ServerConfig{
		Command: "test-server",
	}
	supervisorConfig := DefaultSupervisorConfig()
	supervisor := NewSupervisor(config, "go", supervisorConfig)

	if supervisor.IsReady() {
		t.Error("expected IsReady to return false before start")
	}
}

func TestSupervisorEventsChannel(t *testing.T) {
	config := ServerConfig{
		Command: "test-server",
	}
	supervisorConfig := DefaultSupervisorConfig()
	supervisor := NewSupervisor(config, "go", supervisorConfig)

	events := supervisor.Events()
	if events == nil {
		t.Error("expected non-nil events channel")
	}
}

func TestSupervisorDiagnosticsHandler(t *testing.T) {
	config := ServerConfig{
		Command: "test-server",
	}
	supervisorConfig := DefaultSupervisorConfig()
	supervisor := NewSupervisor(config, "go", supervisorConfig)

	var called bool
	supervisor.OnDiagnostics(func(uri DocumentURI, diagnostics []Diagnostic) {
		called = true
	})

	// Handler is set but won't be called without a running server
	if called {
		t.Error("handler should not be called before server starts")
	}
}

func TestSupervisorStopBeforeStart(t *testing.T) {
	config := ServerConfig{
		Command: "test-server",
	}
	supervisorConfig := DefaultSupervisorConfig()
	supervisor := NewSupervisor(config, "go", supervisorConfig)

	// Stop should be safe to call before Start
	err := supervisor.Stop(nil)
	if err != nil {
		t.Errorf("Stop on idle supervisor should not return error: %v", err)
	}
}

func TestManagerWithSupervision(t *testing.T) {
	manager := NewManager(
		WithSupervision(DefaultSupervisorConfig()),
	)

	if !manager.IsSupervised() {
		t.Error("expected manager to be in supervised mode")
	}
}

func TestManagerWithoutSupervision(t *testing.T) {
	manager := NewManager()

	if manager.IsSupervised() {
		t.Error("expected manager to not be in supervised mode by default")
	}
}

func TestManagerSupervisorStatsNoSupervisor(t *testing.T) {
	manager := NewManager()

	_, exists := manager.SupervisorStats("go")
	if exists {
		t.Error("expected no supervisor stats for non-existent language")
	}
}
