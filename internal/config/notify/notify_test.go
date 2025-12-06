package notify

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	n := New()
	if n == nil {
		t.Fatal("New() returned nil")
	}
	defer n.Close()
}

func TestNew_WithAsync(t *testing.T) {
	n := New(WithAsync(100))
	if n == nil {
		t.Fatal("New() returned nil")
	}
	if !n.async {
		t.Error("expected async = true")
	}
	defer n.Close()
}

func TestChangeType_String(t *testing.T) {
	tests := []struct {
		ct   ChangeType
		want string
	}{
		{ChangeSet, "set"},
		{ChangeDelete, "delete"},
		{ChangeReload, "reload"},
		{ChangeType(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestNotifier_Subscribe(t *testing.T) {
	n := New()
	defer n.Close()

	var received atomic.Bool

	sub := n.Subscribe(func(change Change) {
		received.Store(true)
	})

	n.Notify(Change{Path: "test", Type: ChangeSet})

	if !received.Load() {
		t.Error("observer did not receive notification")
	}

	// Unsubscribe
	sub.Unsubscribe()

	received.Store(false)
	n.Notify(Change{Path: "test2", Type: ChangeSet})

	if received.Load() {
		t.Error("unsubscribed observer received notification")
	}
}

func TestNotifier_SubscribePath(t *testing.T) {
	n := New()
	defer n.Close()

	var editorChanges, uiChanges atomic.Int32

	n.SubscribePath("editor", func(change Change) {
		editorChanges.Add(1)
	})
	n.SubscribePath("ui", func(change Change) {
		uiChanges.Add(1)
	})

	// Send editor.tabSize change
	n.NotifySet("editor.tabSize", 4, 2, "user")
	// Send ui.theme change
	n.NotifySet("ui.theme", "light", "dark", "user")
	// Send editor exact match
	n.NotifySet("editor", nil, map[string]any{}, "user")

	if editorChanges.Load() != 2 {
		t.Errorf("editor observer received %d changes, want 2", editorChanges.Load())
	}
	if uiChanges.Load() != 1 {
		t.Errorf("ui observer received %d changes, want 1", uiChanges.Load())
	}
}

func TestNotifier_NotifySet(t *testing.T) {
	n := New()
	defer n.Close()

	var receivedChange Change

	n.Subscribe(func(change Change) {
		receivedChange = change
	})

	n.NotifySet("editor.tabSize", 4, 2, "user")

	if receivedChange.Path != "editor.tabSize" {
		t.Errorf("Path = %q, want 'editor.tabSize'", receivedChange.Path)
	}
	if receivedChange.Type != ChangeSet {
		t.Errorf("Type = %v, want ChangeSet", receivedChange.Type)
	}
	if receivedChange.OldValue != 4 {
		t.Errorf("OldValue = %v, want 4", receivedChange.OldValue)
	}
	if receivedChange.NewValue != 2 {
		t.Errorf("NewValue = %v, want 2", receivedChange.NewValue)
	}
	if receivedChange.Source != "user" {
		t.Errorf("Source = %q, want 'user'", receivedChange.Source)
	}
}

func TestNotifier_NotifyDelete(t *testing.T) {
	n := New()
	defer n.Close()

	var receivedChange Change

	n.Subscribe(func(change Change) {
		receivedChange = change
	})

	n.NotifyDelete("editor.tabSize", 4, "user")

	if receivedChange.Type != ChangeDelete {
		t.Errorf("Type = %v, want ChangeDelete", receivedChange.Type)
	}
	if receivedChange.OldValue != 4 {
		t.Errorf("OldValue = %v, want 4", receivedChange.OldValue)
	}
}

func TestNotifier_NotifyReload(t *testing.T) {
	n := New()
	defer n.Close()

	var globalReceived, pathReceived atomic.Bool

	n.Subscribe(func(change Change) {
		if change.Type == ChangeReload {
			globalReceived.Store(true)
		}
	})
	n.SubscribePath("editor", func(change Change) {
		if change.Type == ChangeReload {
			pathReceived.Store(true)
		}
	})

	n.NotifyReload("file")

	if !globalReceived.Load() {
		t.Error("global observer did not receive reload")
	}
	if !pathReceived.Load() {
		t.Error("path observer did not receive reload")
	}
}

func TestNotifier_Async(t *testing.T) {
	n := New(WithAsync(100))
	defer n.Close()

	var received atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	n.Subscribe(func(change Change) {
		received.Store(true)
		wg.Done()
	})

	n.Notify(Change{Path: "test", Type: ChangeSet})

	// Wait for async delivery
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if !received.Load() {
			t.Error("async observer did not receive notification")
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for async notification")
	}
}

func TestNotifier_MultipleObservers(t *testing.T) {
	n := New()
	defer n.Close()

	var count1, count2, count3 atomic.Int32

	n.Subscribe(func(change Change) {
		count1.Add(1)
	})
	n.Subscribe(func(change Change) {
		count2.Add(1)
	})
	n.SubscribePath("editor", func(change Change) {
		count3.Add(1)
	})

	n.NotifySet("editor.tabSize", nil, 4, "test")

	if count1.Load() != 1 {
		t.Error("global observer 1 did not receive notification")
	}
	if count2.Load() != 1 {
		t.Error("global observer 2 did not receive notification")
	}
	if count3.Load() != 1 {
		t.Error("path observer did not receive notification")
	}
}

func TestSubscription_Unsubscribe(t *testing.T) {
	n := New()
	defer n.Close()

	var count atomic.Int32

	sub := n.Subscribe(func(change Change) {
		count.Add(1)
	})

	n.Notify(Change{Path: "test", Type: ChangeSet})
	if count.Load() != 1 {
		t.Error("observer should receive first notification")
	}

	sub.Unsubscribe()

	n.Notify(Change{Path: "test", Type: ChangeSet})
	if count.Load() != 1 {
		t.Error("unsubscribed observer should not receive second notification")
	}

	// Unsubscribe again should be safe
	sub.Unsubscribe()
}

func TestBatch_Basic(t *testing.T) {
	n := New()
	defer n.Close()

	var changes []Change
	var mu sync.Mutex

	n.Subscribe(func(change Change) {
		mu.Lock()
		changes = append(changes, change)
		mu.Unlock()
	})

	batch := n.NewBatch()
	batch.Set("editor.tabSize", nil, 4, "test")
	batch.Set("editor.insertSpaces", nil, true, "test")
	batch.Add(Change{Path: "ui.theme", Type: ChangeSet, NewValue: "dark"})

	if batch.Len() != 3 {
		t.Errorf("Len() = %d, want 3", batch.Len())
	}

	// Changes not sent yet
	mu.Lock()
	if len(changes) != 0 {
		t.Error("changes sent before Commit()")
	}
	mu.Unlock()

	batch.Commit()

	mu.Lock()
	if len(changes) != 3 {
		t.Errorf("received %d changes after Commit(), want 3", len(changes))
	}
	mu.Unlock()

	// Batch should be empty after commit
	if batch.Len() != 0 {
		t.Errorf("Len() = %d after Commit(), want 0", batch.Len())
	}
}

func TestBatch_Discard(t *testing.T) {
	n := New()
	defer n.Close()

	var count atomic.Int32

	n.Subscribe(func(change Change) {
		count.Add(1)
	})

	batch := n.NewBatch()
	batch.Set("test", nil, 1, "test")
	batch.Set("test2", nil, 2, "test")

	batch.Discard()

	if batch.Len() != 0 {
		t.Errorf("Len() = %d after Discard(), want 0", batch.Len())
	}

	if count.Load() != 0 {
		t.Error("observer received notification after Discard()")
	}
}

func TestIsParentPath(t *testing.T) {
	tests := []struct {
		parent string
		child  string
		want   bool
	}{
		{"editor", "editor.tabSize", true},
		{"editor", "editor.font.family", true},
		{"", "editor", true},
		{"editor", "editor", false},
		{"editor", "ui", false},
		{"editor", "editorConfig", false},
		{"editor.font", "editor.fontFamily", false},
		{"editor.font", "editor.font.size", true},
	}

	for _, tt := range tests {
		got := isParentPath(tt.parent, tt.child)
		if got != tt.want {
			t.Errorf("isParentPath(%q, %q) = %v, want %v", tt.parent, tt.child, got, tt.want)
		}
	}
}

func TestNotifier_ConcurrentAccess(t *testing.T) {
	n := New()
	defer n.Close()

	var count atomic.Int32

	// Subscribe concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.Subscribe(func(change Change) {
				count.Add(1)
			})
		}()
	}
	wg.Wait()

	// Notify concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n.NotifySet("test", nil, i, "test")
		}(i)
	}
	wg.Wait()

	// Each of 10 observers should receive 10 notifications
	expected := int32(100)
	if count.Load() != expected {
		t.Errorf("count = %d, want %d", count.Load(), expected)
	}
}

func TestNotifier_CloseIdempotent(t *testing.T) {
	n := New()

	// Close should be safe to call multiple times
	n.Close()
	n.Close()
	n.Close()

	// Notify after close should not panic
	n.Notify(Change{Path: "test", Type: ChangeSet})
}

func TestNotifier_CloseIdempotentAsync(t *testing.T) {
	n := New(WithAsync(100))

	// Close should be safe to call multiple times
	n.Close()
	n.Close()
	n.Close()

	// Notify after close should not panic or block
	n.Notify(Change{Path: "test", Type: ChangeSet})
}
