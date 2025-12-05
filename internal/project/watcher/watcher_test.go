package watcher

import (
	"testing"
	"time"
)

func TestOp_String(t *testing.T) {
	tests := []struct {
		op   Op
		want string
	}{
		{OpCreate, "CREATE"},
		{OpWrite, "WRITE"},
		{OpRemove, "REMOVE"},
		{OpRename, "RENAME"},
		{OpChmod, "CHMOD"},
		{Op(0), "UNKNOWN"},
		{Op(100), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("Op(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestOp_Has(t *testing.T) {
	tests := []struct {
		op     Op
		check  Op
		expect bool
	}{
		{OpCreate, OpCreate, true},
		{OpCreate, OpWrite, false},
		{OpCreate | OpWrite, OpCreate, true},
		{OpCreate | OpWrite, OpWrite, true},
		{OpCreate | OpWrite, OpRemove, false},
		{OpCreate | OpWrite | OpRemove, OpRemove, true},
	}

	for _, tt := range tests {
		if got := tt.op.Has(tt.check); got != tt.expect {
			t.Errorf("Op(%d).Has(%d) = %v, want %v", tt.op, tt.check, got, tt.expect)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.DebounceDelay != 100*time.Millisecond {
		t.Errorf("DebounceDelay = %v, want %v", config.DebounceDelay, 100*time.Millisecond)
	}

	if config.BufferSize != 100 {
		t.Errorf("BufferSize = %d, want %d", config.BufferSize, 100)
	}

	if config.IgnoreHidden != false {
		t.Error("IgnoreHidden should default to false")
	}

	if config.MaxWatches != 0 {
		t.Errorf("MaxWatches = %d, want 0", config.MaxWatches)
	}
}

func TestWatcherOptions(t *testing.T) {
	config := DefaultConfig()

	WithDebounceDelay(500 * time.Millisecond)(&config)
	if config.DebounceDelay != 500*time.Millisecond {
		t.Errorf("DebounceDelay = %v, want %v", config.DebounceDelay, 500*time.Millisecond)
	}

	WithBufferSize(200)(&config)
	if config.BufferSize != 200 {
		t.Errorf("BufferSize = %d, want %d", config.BufferSize, 200)
	}

	WithIgnorePatterns([]string{"*.log", "node_modules/"})(&config)
	if len(config.IgnorePatterns) != 2 {
		t.Errorf("IgnorePatterns count = %d, want 2", len(config.IgnorePatterns))
	}

	WithIgnoreHidden(true)(&config)
	if !config.IgnoreHidden {
		t.Error("IgnoreHidden should be true")
	}

	WithFollowSymlinks(true)(&config)
	if !config.FollowSymlinks {
		t.Error("FollowSymlinks should be true")
	}

	WithMaxWatches(1000)(&config)
	if config.MaxWatches != 1000 {
		t.Errorf("MaxWatches = %d, want 1000", config.MaxWatches)
	}

	filter := func(e Event) bool { return true }
	WithEventFilter(filter)(&config)
	if config.EventFilter == nil {
		t.Error("EventFilter should not be nil")
	}
}

func TestEventDispatcher(t *testing.T) {
	dispatcher := NewEventDispatcher()

	var receivedEvent Event
	var receivedError error

	dispatcher.OnEvent(func(e Event) {
		receivedEvent = e
	})

	dispatcher.OnError(func(err error) {
		receivedError = err
	})

	// Test event dispatch
	event := Event{
		Path:      "/test/file.txt",
		Op:        OpWrite,
		Timestamp: time.Now(),
	}
	dispatcher.Dispatch(event)

	if receivedEvent.Path != event.Path {
		t.Errorf("received path = %q, want %q", receivedEvent.Path, event.Path)
	}

	// Test error dispatch
	testErr := ErrWatcherClosed
	dispatcher.DispatchError(testErr)

	if receivedError != testErr {
		t.Errorf("received error = %v, want %v", receivedError, testErr)
	}
}

func TestEvent_Fields(t *testing.T) {
	now := time.Now()
	event := Event{
		Path:      "/test/file.txt",
		Op:        OpCreate | OpWrite,
		Timestamp: now,
	}

	if event.Path != "/test/file.txt" {
		t.Errorf("Path = %q, want %q", event.Path, "/test/file.txt")
	}

	if !event.Op.Has(OpCreate) {
		t.Error("Op should have OpCreate")
	}

	if !event.Op.Has(OpWrite) {
		t.Error("Op should have OpWrite")
	}

	if !event.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", event.Timestamp, now)
	}
}
