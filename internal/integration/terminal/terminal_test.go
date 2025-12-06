package terminal

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewHistory(t *testing.T) {
	h := NewHistory(100)

	if h.Len() != 0 {
		t.Errorf("expected empty history, got %d lines", h.Len())
	}
}

func TestHistoryAdd(t *testing.T) {
	h := NewHistory(100)

	line := &Line{
		Cells: []Cell{{Rune: 'H'}, {Rune: 'i'}},
	}
	h.Add(line)

	if h.Len() != 1 {
		t.Errorf("expected 1 line, got %d", h.Len())
	}

	retrieved := h.Line(0)
	if retrieved.Cells[0].Rune != 'H' {
		t.Error("history line not preserved")
	}
}

func TestHistoryMaxLines(t *testing.T) {
	h := NewHistory(5)

	// Add more lines than max
	for i := 0; i < 10; i++ {
		h.Add(&Line{Cells: []Cell{{Rune: rune('A' + i)}}})
	}

	if h.Len() != 5 {
		t.Errorf("expected 5 lines (max), got %d", h.Len())
	}

	// Should have the last 5 lines (F, G, H, I, J)
	first := h.Line(0)
	if first.Cells[0].Rune != 'F' {
		t.Errorf("expected 'F' as oldest, got %c", first.Cells[0].Rune)
	}
}

func TestHistoryClear(t *testing.T) {
	h := NewHistory(100)

	h.Add(&Line{Cells: []Cell{{Rune: 'A'}}})
	h.Add(&Line{Cells: []Cell{{Rune: 'B'}}})
	h.Clear()

	if h.Len() != 0 {
		t.Errorf("expected empty after clear, got %d", h.Len())
	}
}

func TestHistoryGetText(t *testing.T) {
	h := NewHistory(100)

	h.Add(&Line{Cells: []Cell{{Rune: 'H'}, {Rune: 'e'}, {Rune: 'l'}, {Rune: 'l'}, {Rune: 'o'}}})
	h.Add(&Line{Cells: []Cell{{Rune: 'W'}, {Rune: 'o'}, {Rune: 'r'}, {Rune: 'l'}, {Rune: 'd'}}})

	text := h.GetText()
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "World") {
		t.Errorf("expected 'Hello\\nWorld', got '%s'", text)
	}
}

func TestHistoryLineOutOfBounds(t *testing.T) {
	h := NewHistory(100)

	h.Add(&Line{Cells: []Cell{{Rune: 'A'}}})

	if h.Line(-1) != nil {
		t.Error("expected nil for negative index")
	}
	if h.Line(1) != nil {
		t.Error("expected nil for out of bounds index")
	}
}

// mockEventPublisher implements EventPublisher for testing.
type mockEventPublisher struct {
	mu     sync.Mutex
	events []mockEvent
}

type mockEvent struct {
	eventType string
	data      map[string]any
}

func (m *mockEventPublisher) Publish(eventType string, data map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, mockEvent{eventType, data})
}

func (m *mockEventPublisher) getEvents() []mockEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mockEvent{}, m.events...)
}

func TestNewManager(t *testing.T) {
	m := NewManager(ManagerConfig{})

	if m.Count() != 0 {
		t.Errorf("expected 0 terminals, got %d", m.Count())
	}
}

func TestManagerConfigDefaults(t *testing.T) {
	m := NewManager(ManagerConfig{
		DefaultCols: 120,
		DefaultRows: 40,
	})

	if m.defaultCols != 120 {
		t.Errorf("expected defaultCols 120, got %d", m.defaultCols)
	}
	if m.defaultRows != 40 {
		t.Errorf("expected defaultRows 40, got %d", m.defaultRows)
	}
}

func TestManagerCreateTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal creation test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{
		Name: "test-term",
	})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	if term.ID() == "" {
		t.Error("expected non-empty terminal ID")
	}
	if term.Name() != "test-term" {
		t.Errorf("expected name 'test-term', got '%s'", term.Name())
	}
	if !term.IsRunning() {
		t.Error("terminal should be running")
	}
	if m.Count() != 1 {
		t.Errorf("expected 1 terminal, got %d", m.Count())
	}
}

func TestManagerGetTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	got, ok := m.Get(term.ID())
	if !ok {
		t.Error("expected to find terminal")
	}
	if got.ID() != term.ID() {
		t.Error("retrieved wrong terminal")
	}

	_, ok = m.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent terminal")
	}
}

func TestManagerListTerminals(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term1, err := m.Create(Options{Name: "term1"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term1.Close()

	term2, err := m.Create(Options{Name: "term2"})
	if err != nil {
		t.Skipf("skipping: failed to create second terminal: %v", err)
	}
	defer term2.Close()

	list := m.List()
	if len(list) != 2 {
		t.Errorf("expected 2 terminals, got %d", len(list))
	}
}

func TestManagerCloseTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}

	err = m.Close(term.ID())
	if err != nil {
		t.Fatalf("failed to close terminal: %v", err)
	}

	<-term.Done() // Wait for close

	if m.Count() != 0 {
		t.Errorf("expected 0 terminals after close, got %d", m.Count())
	}
}

func TestManagerCloseNonexistent(t *testing.T) {
	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	err := m.Close("nonexistent")
	if err != ErrTerminalNotFound {
		t.Errorf("expected ErrTerminalNotFound, got %v", err)
	}
}

func TestManagerCloseAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term1, err := m.Create(Options{Name: "term1"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}

	term2, err := m.Create(Options{Name: "term2"})
	if err != nil {
		t.Skipf("skipping: failed to create second terminal: %v", err)
	}

	m.CloseAll()

	<-term1.Done()
	<-term2.Done()

	if m.Count() != 0 {
		t.Errorf("expected 0 terminals after close all, got %d", m.Count())
	}
}

func TestManagerShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}

	m.Shutdown(5 * time.Second)

	<-term.Done()

	// Should not be able to create after shutdown
	_, err = m.Create(Options{Name: "new"})
	if err != ErrManagerClosed {
		t.Errorf("expected ErrManagerClosed, got %v", err)
	}
}

func TestManagerEventPublishing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	pub := &mockEventPublisher{}
	m := NewManager(ManagerConfig{
		EventBus: pub,
	})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}

	// Should have a created event
	events := pub.getEvents()
	found := false
	for _, e := range events {
		if e.eventType == "terminal.created" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected terminal.created event")
	}

	term.Close()
	<-term.Done()

	// Should have a closed event
	events = pub.getEvents()
	found = false
	for _, e := range events {
		if e.eventType == "terminal.closed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected terminal.closed event")
	}
}

func TestTerminalSetName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "original"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	term.SetName("updated")

	if term.Name() != "updated" {
		t.Errorf("expected name 'updated', got '%s'", term.Name())
	}
}

func TestTerminalWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	// Write something to the terminal
	n, err := term.WriteString("echo hello\n")
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if n != 11 {
		t.Errorf("expected 11 bytes written, got %d", n)
	}
}

func TestTerminalWriteClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	term.Close()
	<-term.Done()

	_, err = term.Write([]byte("test"))
	if err != ErrTerminalClosed {
		t.Errorf("expected ErrTerminalClosed, got %v", err)
	}
}

func TestTerminalResize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{
		Name: "test",
		Cols: 80,
		Rows: 24,
	})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	err = term.Resize(120, 40)
	if err != nil {
		t.Fatalf("failed to resize: %v", err)
	}

	screen := term.Screen()
	if screen.Width() != 120 || screen.Height() != 40 {
		t.Errorf("expected 120x40, got %dx%d", screen.Width(), screen.Height())
	}
}

func TestTerminalResizeInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	err = term.Resize(0, 0)
	if err != ErrInvalidSize {
		t.Errorf("expected ErrInvalidSize, got %v", err)
	}

	err = term.Resize(-1, 24)
	if err != ErrInvalidSize {
		t.Errorf("expected ErrInvalidSize, got %v", err)
	}
}

func TestTerminalResizeClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	term.Close()
	<-term.Done()

	err = term.Resize(100, 50)
	if err != ErrTerminalClosed {
		t.Errorf("expected ErrTerminalClosed, got %v", err)
	}
}

func TestTerminalScreen(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{
		Name: "test",
		Cols: 80,
		Rows: 24,
	})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	screen := term.Screen()
	if screen == nil {
		t.Fatal("expected non-nil screen")
	}
	if screen.Width() != 80 {
		t.Errorf("expected width 80, got %d", screen.Width())
	}
}

func TestTerminalHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{
		Name:       "test",
		Scrollback: 5000,
	})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	history := term.History()
	if history == nil {
		t.Fatal("expected non-nil history")
	}
}

func TestTerminalExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}

	// Exit code should be -1 while running
	if term.ExitCode() != -1 {
		t.Errorf("expected exit code -1 while running, got %d", term.ExitCode())
	}

	term.Close()
	<-term.Done()

	// After close, exit code may be set (though value depends on how shell was killed)
}

func TestTerminalPID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	pid := term.PID()
	if pid <= 0 {
		t.Errorf("expected positive PID, got %d", pid)
	}
}

func TestTerminalDone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}

	done := term.Done()

	// Channel should be open while running
	select {
	case <-done:
		t.Error("done channel should not be closed while running")
	default:
		// Good
	}

	term.Close()
	<-done // Should unblock
}

func TestTerminalOutputCallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	var mu sync.Mutex
	var output []byte

	term, err := m.Create(Options{
		Name: "test",
		OnOutput: func(data []byte) {
			mu.Lock()
			output = append(output, data...)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	// Wait a bit for shell startup output
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	hasOutput := len(output) > 0
	mu.Unlock()

	// Shell usually produces some output on startup
	if !hasOutput {
		t.Log("no output received (may be normal for some shells)")
	}
}

func TestTerminalTitleCallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	var mu sync.Mutex
	var titles []string

	term, err := m.Create(Options{
		Name: "test",
		OnTitle: func(title string) {
			mu.Lock()
			titles = append(titles, title)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	// Some shells set title on startup, but not all
	// This is more of an integration test
}

func TestTerminalCloseCallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	closeCalled := make(chan struct{})

	term, err := m.Create(Options{
		Name: "test",
		OnClose: func() {
			close(closeCalled)
		},
	})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}

	term.Close()

	select {
	case <-closeCalled:
		// Good
	case <-time.After(5 * time.Second):
		t.Error("close callback not called within timeout")
	}
}

func TestTerminalWorkingDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{
		Name:    "test",
		WorkDir: "/tmp",
	})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	// Initial working directory should be set
	cwd := term.WorkingDirectory()
	if cwd != "/tmp" {
		t.Logf("initial cwd: %s (may differ based on shell)", cwd)
	}
}

func TestTerminalDoubleClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	term, err := m.Create(Options{Name: "test"})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}

	// First close
	err = term.Close()
	if err != nil {
		t.Fatalf("first close failed: %v", err)
	}

	// Second close should be idempotent
	err = term.Close()
	if err != nil {
		t.Fatalf("second close should succeed: %v", err)
	}
}

func TestShellNotFound(t *testing.T) {
	m := NewManager(ManagerConfig{})
	defer m.Shutdown(5 * time.Second)

	_, err := m.Create(Options{
		Name:  "test",
		Shell: "/nonexistent/shell",
	})

	if err == nil {
		t.Error("expected error for nonexistent shell")
	}
}

func TestOptionsDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping terminal test in short mode")
	}

	m := NewManager(ManagerConfig{
		DefaultCols:  100,
		DefaultRows:  50,
		Scrollback:   5000,
		DefaultShell: "/bin/sh",
	})
	defer m.Shutdown(5 * time.Second)

	// Create with no options - should use manager defaults
	term, err := m.Create(Options{})
	if err != nil {
		t.Skipf("skipping: failed to create terminal (may not have PTY): %v", err)
	}
	defer term.Close()

	screen := term.Screen()
	if screen.Width() != 100 || screen.Height() != 50 {
		t.Errorf("expected 100x50, got %dx%d", screen.Width(), screen.Height())
	}
}
