package macro

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/input/key"
)

// Helper to create test events
func makeEvent(r rune) key.Event {
	return key.Event{
		Key:       key.KeyRune,
		Rune:      r,
		Modifiers: key.ModNone,
		Timestamp: time.Now(),
	}
}

func makeSpecialEvent(k key.Key, mods key.Modifier) key.Event {
	return key.Event{
		Key:       k,
		Modifiers: mods,
		Timestamp: time.Now(),
	}
}

// ==================== Register Tests ====================

func TestIsValidRegister(t *testing.T) {
	tests := []struct {
		input rune
		want  bool
	}{
		{'a', true},
		{'z', true},
		{'m', true},
		{'0', true},
		{'9', true},
		{'5', true},
		{'A', false}, // uppercase not valid (but can be normalized)
		{'Z', false},
		{'!', false},
		{' ', false},
		{0, false},
	}

	for _, tt := range tests {
		got := IsValidRegister(tt.input)
		if got != tt.want {
			t.Errorf("IsValidRegister(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeRegister(t *testing.T) {
	tests := []struct {
		input rune
		want  rune
	}{
		{'a', 'a'},
		{'z', 'z'},
		{'A', 'a'},
		{'Z', 'z'},
		{'0', '0'},
		{'9', '9'},
		{'!', 0},
		{' ', 0},
	}

	for _, tt := range tests {
		got := NormalizeRegister(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeRegister(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsAppendRegister(t *testing.T) {
	tests := []struct {
		input rune
		want  bool
	}{
		{'A', true},
		{'Z', true},
		{'M', true},
		{'a', false},
		{'z', false},
		{'0', false},
	}

	for _, tt := range tests {
		got := IsAppendRegister(tt.input)
		if got != tt.want {
			t.Errorf("IsAppendRegister(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestAllRegisters(t *testing.T) {
	letters := AllLetterRegisters()
	if len(letters) != 26 {
		t.Errorf("AllLetterRegisters() returned %d registers, want 26", len(letters))
	}

	digits := AllDigitRegisters()
	if len(digits) != 10 {
		t.Errorf("AllDigitRegisters() returned %d registers, want 10", len(digits))
	}

	all := AllRegisters()
	if len(all) != 36 {
		t.Errorf("AllRegisters() returned %d registers, want 36", len(all))
	}
}

// ==================== Recorder Tests ====================

func TestRecorderBasic(t *testing.T) {
	r := NewRecorder()

	// Initially not recording
	if r.IsRecording() {
		t.Error("new recorder should not be recording")
	}

	// Start recording
	if err := r.StartRecording('a'); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	if !r.IsRecording() {
		t.Error("should be recording after StartRecording")
	}

	if r.CurrentRegister() != 'a' {
		t.Errorf("CurrentRegister() = %q, want 'a'", r.CurrentRegister())
	}

	// Record some events
	events := []key.Event{
		makeEvent('h'),
		makeEvent('e'),
		makeEvent('l'),
		makeEvent('l'),
		makeEvent('o'),
	}
	for _, e := range events {
		r.Record(e)
	}

	if r.CurrentRecordingLength() != 5 {
		t.Errorf("CurrentRecordingLength() = %d, want 5", r.CurrentRecordingLength())
	}

	// Stop recording
	recorded := r.StopRecording()
	if len(recorded) != 5 {
		t.Errorf("StopRecording returned %d events, want 5", len(recorded))
	}

	if r.IsRecording() {
		t.Error("should not be recording after StopRecording")
	}

	// Verify macro is saved
	saved := r.Get('a')
	if len(saved) != 5 {
		t.Errorf("Get('a') returned %d events, want 5", len(saved))
	}

	if !r.HasMacro('a') {
		t.Error("HasMacro('a') should return true")
	}
}

func TestRecorderInvalidRegister(t *testing.T) {
	r := NewRecorder()

	err := r.StartRecording('!')
	if err == nil {
		t.Error("StartRecording with invalid register should fail")
	}

	err = r.Set('!', []key.Event{makeEvent('x')})
	if err == nil {
		t.Error("Set with invalid register should fail")
	}
}

func TestRecorderAlreadyRecording(t *testing.T) {
	r := NewRecorder()

	if err := r.StartRecording('a'); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	err := r.StartRecording('b')
	if err == nil {
		t.Error("StartRecording while already recording should fail")
	}

	r.StopRecording()
}

func TestRecorderAppend(t *testing.T) {
	r := NewRecorder()

	// Set initial macro
	initial := []key.Event{makeEvent('a'), makeEvent('b')}
	r.Set('x', initial)

	// Append more events
	additional := []key.Event{makeEvent('c'), makeEvent('d')}
	r.Append('x', additional)

	// Verify
	result := r.Get('x')
	if len(result) != 4 {
		t.Errorf("Get('x') returned %d events, want 4", len(result))
	}
}

func TestRecorderClear(t *testing.T) {
	r := NewRecorder()

	r.Set('a', []key.Event{makeEvent('x')})
	r.Set('b', []key.Event{makeEvent('y')})

	// Clear single register
	r.Clear('a')
	if r.HasMacro('a') {
		t.Error("HasMacro('a') should return false after Clear")
	}
	if !r.HasMacro('b') {
		t.Error("HasMacro('b') should still return true")
	}

	// Clear all
	r.ClearAll()
	if r.HasMacro('b') {
		t.Error("HasMacro('b') should return false after ClearAll")
	}
}

func TestRecorderListRegisters(t *testing.T) {
	r := NewRecorder()

	r.Set('a', []key.Event{makeEvent('x')})
	r.Set('m', []key.Event{makeEvent('y')})
	r.Set('5', []key.Event{makeEvent('z')})

	list := r.ListRegisters()
	if len(list) != 3 {
		t.Errorf("ListRegisters() returned %d registers, want 3", len(list))
	}
}

func TestRecorderConcurrent(t *testing.T) {
	r := NewRecorder()

	// Pre-populate some registers
	for i := 'a'; i <= 'e'; i++ {
		r.Set(i, []key.Event{makeEvent(i)})
	}

	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				r.Get('a')
				r.HasMacro('b')
				r.ListRegisters()
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reg := rune('f' + idx)
			for j := 0; j < 100; j++ {
				r.Set(reg, []key.Event{makeEvent('x')})
			}
		}(i)
	}

	wg.Wait()
}

// ==================== Player Tests ====================

func TestPlayerBasic(t *testing.T) {
	r := NewRecorder()
	p := NewPlayer(r)

	// Set up a macro
	events := []key.Event{
		makeEvent('a'),
		makeEvent('b'),
		makeEvent('c'),
	}
	r.Set('x', events)

	// Play the macro
	var played []key.Event
	var mu sync.Mutex

	err := p.Play('x', 1, func(e key.Event) {
		mu.Lock()
		played = append(played, e)
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("Play failed: %v", err)
	}

	if len(played) != 3 {
		t.Errorf("handler called %d times, want 3", len(played))
	}

	// Verify events match
	for i, e := range played {
		if e.Rune != events[i].Rune {
			t.Errorf("played[%d].Rune = %q, want %q", i, e.Rune, events[i].Rune)
		}
	}
}

func TestPlayerCount(t *testing.T) {
	r := NewRecorder()
	p := NewPlayer(r)

	events := []key.Event{makeEvent('x')}
	r.Set('a', events)

	var count int
	var mu sync.Mutex

	err := p.Play('a', 5, func(e key.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("Play failed: %v", err)
	}

	if count != 5 {
		t.Errorf("handler called %d times, want 5", count)
	}
}

func TestPlayerEmptyRegister(t *testing.T) {
	r := NewRecorder()
	p := NewPlayer(r)

	err := p.Play('z', 1, func(e key.Event) {})
	if err == nil {
		t.Error("Play with empty register should fail")
	}
}

func TestPlayerInvalidRegister(t *testing.T) {
	r := NewRecorder()
	p := NewPlayer(r)

	err := p.Play('!', 1, func(e key.Event) {})
	if err == nil {
		t.Error("Play with invalid register should fail")
	}
}

func TestPlayerNilHandler(t *testing.T) {
	r := NewRecorder()
	p := NewPlayer(r)

	r.Set('a', []key.Event{makeEvent('x')})

	err := p.Play('a', 1, nil)
	if err == nil {
		t.Error("Play with nil handler should fail")
	}
}

func TestPlayerLastPlayed(t *testing.T) {
	r := NewRecorder()
	p := NewPlayer(r)

	r.Set('a', []key.Event{makeEvent('x')})
	r.Set('b', []key.Event{makeEvent('y')})

	// Play register 'a'
	p.Play('a', 1, func(e key.Event) {})

	if r.LastPlayed() != 'a' {
		t.Errorf("LastPlayed() = %q, want 'a'", r.LastPlayed())
	}

	// Play register 'b'
	p.Play('b', 1, func(e key.Event) {})

	if r.LastPlayed() != 'b' {
		t.Errorf("LastPlayed() = %q, want 'b'", r.LastPlayed())
	}

	// PlayLast should replay 'b'
	var count int
	p.PlayLast(1, func(e key.Event) {
		count++
	})

	if count != 1 {
		t.Errorf("PlayLast handler called %d times, want 1", count)
	}
}

func TestPlayerAsync(t *testing.T) {
	r := NewRecorder()
	p := NewPlayer(r)

	events := []key.Event{
		makeEvent('a'),
		makeEvent('b'),
		makeEvent('c'),
	}
	r.Set('x', events)

	done := make(chan struct{})
	var played []key.Event
	var mu sync.Mutex

	err := p.PlayAsync('x', 1, func(e key.Event) {
		mu.Lock()
		played = append(played, e)
		mu.Unlock()
	}, done)

	if err != nil {
		t.Fatalf("PlayAsync failed: %v", err)
	}

	// Wait for completion
	select {
	case <-done:
		// OK
	case <-time.After(time.Second):
		t.Fatal("PlayAsync did not complete")
	}

	mu.Lock()
	if len(played) != 3 {
		t.Errorf("handler called %d times, want 3", len(played))
	}
	mu.Unlock()
}

func TestPlayerCancel(t *testing.T) {
	r := NewRecorder()
	p := NewPlayer(r)

	// Large macro to ensure we can cancel mid-playback
	events := make([]key.Event, 1000)
	for i := range events {
		events[i] = makeEvent('x')
	}
	r.Set('a', events)

	done := make(chan struct{})
	var count int
	var mu sync.Mutex

	err := p.PlayAsync('a', 100, func(e key.Event) {
		mu.Lock()
		count++
		mu.Unlock()
		time.Sleep(time.Microsecond) // Slow down playback
	}, done)

	if err != nil {
		t.Fatalf("PlayAsync failed: %v", err)
	}

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel
	p.Cancel()

	// Wait for completion
	select {
	case <-done:
		// OK
	case <-time.After(time.Second):
		t.Fatal("PlayAsync did not complete after cancel")
	}

	mu.Lock()
	if count >= 100*1000 {
		t.Errorf("playback should have been cancelled, but all events played")
	}
	mu.Unlock()
}

func TestPlayerWithContext(t *testing.T) {
	r := NewRecorder()
	p := NewPlayer(r)

	events := make([]key.Event, 100)
	for i := range events {
		events[i] = makeEvent('x')
	}
	r.Set('a', events)

	ctx, cancel := context.WithCancel(context.Background())

	var count int
	var mu sync.Mutex

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := p.PlayWithContext(ctx, 'a', 100, func(e key.Event) {
		mu.Lock()
		count++
		mu.Unlock()
		time.Sleep(time.Microsecond)
	})

	// Should have been cancelled
	if err != context.Canceled {
		t.Logf("PlayWithContext returned: %v (count=%d)", err, count)
	}

	mu.Lock()
	if count >= 100*100 {
		t.Error("playback should have been cancelled")
	}
	mu.Unlock()
}

// ==================== Persistence Tests ====================

func TestPersistenceSaveLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "macros.json")

	// Create recorder with macros
	r1 := NewRecorder()
	r1.Set('a', []key.Event{
		makeEvent('h'),
		makeEvent('e'),
		makeEvent('l'),
		makeEvent('l'),
		makeEvent('o'),
	})
	r1.Set('b', []key.Event{
		makeSpecialEvent(key.KeyEscape, key.ModNone),
		makeEvent('d'),
		makeEvent('d'),
	})
	r1.SetLastPlayed('a')

	// Save
	if err := Save(r1, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved file does not exist: %v", err)
	}

	// Load into new recorder
	r2 := NewRecorder()
	if err := Load(r2, path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify macros loaded
	if !r2.HasMacro('a') {
		t.Error("macro 'a' not loaded")
	}
	if !r2.HasMacro('b') {
		t.Error("macro 'b' not loaded")
	}

	// Verify content
	events := r2.Get('a')
	if len(events) != 5 {
		t.Errorf("Get('a') returned %d events, want 5", len(events))
	}

	// Verify last played
	if r2.LastPlayed() != 'a' {
		t.Errorf("LastPlayed() = %q, want 'a'", r2.LastPlayed())
	}
}

func TestPersistenceLoadNonexistent(t *testing.T) {
	r := NewRecorder()

	// Loading non-existent file should succeed (no-op)
	err := Load(r, "/nonexistent/path/macros.json")
	if err != nil {
		t.Errorf("Load non-existent file should not error: %v", err)
	}
}

func TestPersistenceExportImport(t *testing.T) {
	r1 := NewRecorder()
	r1.Set('a', []key.Event{makeEvent('x'), makeEvent('y')})
	r1.Set('b', []key.Event{makeEvent('z')})

	// Export
	data, err := Export(r1)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import into new recorder (replace mode)
	r2 := NewRecorder()
	r2.Set('b', []key.Event{makeEvent('q')}) // Pre-existing

	err = Import(r2, data, false) // replace mode
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify 'a' imported
	if !r2.HasMacro('a') {
		t.Error("macro 'a' not imported")
	}

	// Verify 'b' replaced
	events := r2.Get('b')
	if len(events) != 1 || events[0].Rune != 'z' {
		t.Error("macro 'b' not replaced correctly")
	}
}

func TestPersistenceImportMerge(t *testing.T) {
	r1 := NewRecorder()
	r1.Set('a', []key.Event{makeEvent('x')})
	r1.Set('b', []key.Event{makeEvent('y')})

	data, err := Export(r1)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import with merge
	r2 := NewRecorder()
	r2.Set('b', []key.Event{makeEvent('q')}) // Pre-existing 'q'

	err = Import(r2, data, true) // merge mode
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify 'a' imported
	if !r2.HasMacro('a') {
		t.Error("macro 'a' not imported")
	}

	// Verify 'b' kept original (not replaced in merge mode)
	events := r2.Get('b')
	if len(events) != 1 || events[0].Rune != 'q' {
		t.Error("macro 'b' should not be replaced in merge mode")
	}
}

// ==================== Register Info Tests ====================

func TestGetRegisterInfo(t *testing.T) {
	r := NewRecorder()
	r.Set('a', []key.Event{makeEvent('x'), makeEvent('y')})

	info := GetRegisterInfo(r, 'a')
	if info.Name != 'a' {
		t.Errorf("info.Name = %q, want 'a'", info.Name)
	}
	if info.EventCount != 2 {
		t.Errorf("info.EventCount = %d, want 2", info.EventCount)
	}
	if info.IsEmpty {
		t.Error("info.IsEmpty should be false")
	}

	// Empty register
	info = GetRegisterInfo(r, 'z')
	if !info.IsEmpty {
		t.Error("info.IsEmpty should be true for empty register")
	}
}

func TestGetAllRegisterInfo(t *testing.T) {
	r := NewRecorder()
	r.Set('a', []key.Event{makeEvent('x')})
	r.Set('b', []key.Event{makeEvent('y'), makeEvent('z')})

	infos := GetAllRegisterInfo(r)
	if len(infos) != 2 {
		t.Errorf("GetAllRegisterInfo returned %d entries, want 2", len(infos))
	}
}

// ==================== Edge Cases ====================

func TestRecorderStopWithoutStart(t *testing.T) {
	r := NewRecorder()

	// Should not panic
	result := r.StopRecording()
	if result != nil {
		t.Error("StopRecording without start should return nil")
	}
}

func TestRecorderRecordWithoutStart(t *testing.T) {
	r := NewRecorder()

	// Should not panic
	r.Record(makeEvent('x'))

	// No macro should be saved
	if r.HasMacro('a') {
		t.Error("no macro should be saved when not recording")
	}
}

func TestRecorderGetReturnsEmptySlice(t *testing.T) {
	r := NewRecorder()

	// Get on empty register should return empty slice, not nil
	events := r.Get('a')
	if events == nil {
		t.Error("Get should return empty slice, not nil")
	}
	if len(events) != 0 {
		t.Errorf("Get on empty register returned %d events, want 0", len(events))
	}
}

func TestPlayerMinimumCount(t *testing.T) {
	r := NewRecorder()
	p := NewPlayer(r)

	r.Set('a', []key.Event{makeEvent('x')})

	var count int
	// Count of 0 should be treated as 1
	p.Play('a', 0, func(e key.Event) {
		count++
	})

	if count != 1 {
		t.Errorf("count=0 should be treated as 1, got %d plays", count)
	}

	count = 0
	// Negative count should also be treated as 1
	p.Play('a', -5, func(e key.Event) {
		count++
	})

	if count != 1 {
		t.Errorf("count=-5 should be treated as 1, got %d plays", count)
	}
}

func TestRecorderEmptyMacroNotSaved(t *testing.T) {
	r := NewRecorder()

	r.StartRecording('a')
	// Record nothing
	r.StopRecording()

	if r.HasMacro('a') {
		t.Error("empty macro should not be saved")
	}
}
