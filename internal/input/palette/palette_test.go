package palette

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCommandArgValidation(t *testing.T) {
	tests := []struct {
		name    string
		arg     CommandArg
		value   any
		wantErr bool
	}{
		{
			name:    "string valid",
			arg:     CommandArg{Name: "text", Type: ArgString},
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "string invalid",
			arg:     CommandArg{Name: "text", Type: ArgString},
			value:   123,
			wantErr: true,
		},
		{
			name:    "number int",
			arg:     CommandArg{Name: "count", Type: ArgNumber},
			value:   42,
			wantErr: false,
		},
		{
			name:    "number float",
			arg:     CommandArg{Name: "count", Type: ArgNumber},
			value:   3.14,
			wantErr: false,
		},
		{
			name:    "number invalid",
			arg:     CommandArg{Name: "count", Type: ArgNumber},
			value:   "not a number",
			wantErr: true,
		},
		{
			name:    "boolean valid",
			arg:     CommandArg{Name: "enabled", Type: ArgBoolean},
			value:   true,
			wantErr: false,
		},
		{
			name:    "boolean invalid",
			arg:     CommandArg{Name: "enabled", Type: ArgBoolean},
			value:   "true",
			wantErr: true,
		},
		{
			name:    "enum valid",
			arg:     CommandArg{Name: "mode", Type: ArgEnum, Options: []string{"read", "write"}},
			value:   "read",
			wantErr: false,
		},
		{
			name:    "enum invalid",
			arg:     CommandArg{Name: "mode", Type: ArgEnum, Options: []string{"read", "write"}},
			value:   "delete",
			wantErr: true,
		},
		{
			name:    "required missing",
			arg:     CommandArg{Name: "req", Type: ArgString, Required: true},
			value:   nil,
			wantErr: true,
		},
		{
			name:    "optional missing",
			arg:     CommandArg{Name: "opt", Type: ArgString, Required: false},
			value:   nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.arg.Validate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommandExecution(t *testing.T) {
	executed := false

	cmd := &Command{
		ID:    "test.cmd",
		Title: "Test Command",
		Handler: func(args map[string]any) error {
			executed = true
			return nil
		},
	}

	err := cmd.Execute(nil)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if !executed {
		t.Error("handler was not executed")
	}
}

func TestCommandWithArgs(t *testing.T) {
	var receivedLine int

	cmd := &Command{
		ID:    "editor.goToLine",
		Title: "Go to Line",
		Args: []CommandArg{
			{Name: "line", Type: ArgNumber, Required: true},
		},
		Handler: func(args map[string]any) error {
			receivedLine = args["line"].(int)
			return nil
		},
	}

	// Missing required arg
	err := cmd.Execute(nil)
	if err == nil {
		t.Error("expected error for missing required arg")
	}

	// Valid arg
	err = cmd.Execute(map[string]any{"line": 42})
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if receivedLine != 42 {
		t.Errorf("expected line 42, got %d", receivedLine)
	}
}

func TestCommandDefaults(t *testing.T) {
	var receivedCount int

	cmd := &Command{
		ID:    "test.count",
		Title: "Count",
		Args: []CommandArg{
			{Name: "count", Type: ArgNumber, Default: 10},
		},
		Handler: func(args map[string]any) error {
			receivedCount = args["count"].(int)
			return nil
		},
	}

	err := cmd.Execute(nil)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if receivedCount != 10 {
		t.Errorf("expected count 10, got %d", receivedCount)
	}
}

func TestHistoryBasic(t *testing.T) {
	h := NewHistory(5)

	h.Add("cmd1")
	h.Add("cmd2")
	h.Add("cmd3")

	recent := h.Recent(10)
	if len(recent) != 3 {
		t.Errorf("expected 3 items, got %d", len(recent))
	}
	if recent[0] != "cmd3" {
		t.Errorf("expected cmd3 first, got %s", recent[0])
	}
}

func TestHistoryMRU(t *testing.T) {
	h := NewHistory(5)

	h.Add("cmd1")
	h.Add("cmd2")
	h.Add("cmd1") // Re-add cmd1

	recent := h.Recent(10)
	if len(recent) != 2 {
		t.Errorf("expected 2 items, got %d", len(recent))
	}
	if recent[0] != "cmd1" {
		t.Errorf("expected cmd1 first (MRU), got %s", recent[0])
	}
}

func TestHistoryMaxSize(t *testing.T) {
	h := NewHistory(3)

	h.Add("cmd1")
	h.Add("cmd2")
	h.Add("cmd3")
	h.Add("cmd4") // Should push out cmd1

	if h.Len() != 3 {
		t.Errorf("expected 3 items, got %d", h.Len())
	}
	if h.Contains("cmd1") {
		t.Error("cmd1 should have been evicted")
	}
}

func TestHistoryPosition(t *testing.T) {
	h := NewHistory(10)

	h.Add("cmd1")
	h.Add("cmd2")
	h.Add("cmd3")

	if pos := h.Position("cmd3"); pos != 0 {
		t.Errorf("expected position 0, got %d", pos)
	}
	if pos := h.Position("cmd1"); pos != 2 {
		t.Errorf("expected position 2, got %d", pos)
	}
	if pos := h.Position("unknown"); pos != -1 {
		t.Errorf("expected position -1, got %d", pos)
	}
}

func TestFilterFuzzyMatch(t *testing.T) {
	f := NewFilter()

	commands := []*Command{
		{ID: "file.save", Title: "Save File"},
		{ID: "file.saveAs", Title: "Save File As"},
		{ID: "file.open", Title: "Open File"},
		{ID: "edit.copy", Title: "Copy"},
		{ID: "edit.paste", Title: "Paste"},
	}

	tests := []struct {
		query       string
		wantFirst   string
		wantMatches int
	}{
		{"save", "file.save", 2},  // Matches "Save File" and "Save File As"
		{"sf", "file.save", 2},    // Fuzzy: S_ave F_ile
		{"open", "file.open", 1},  // Exact match
		{"cop", "edit.copy", 1},   // Prefix match
		{"xyz", "", 0},            // No match
		{"file", "file.save", 3},  // Matches all file.* commands
		{"sfa", "file.saveAs", 1}, // Matches "Save File As"
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := f.Search(commands, tt.query, 10)
			if len(results) != tt.wantMatches {
				t.Errorf("query %q: got %d matches, want %d", tt.query, len(results), tt.wantMatches)
			}
			if tt.wantMatches > 0 && results[0].Command.ID != tt.wantFirst {
				t.Errorf("query %q: got first %s, want %s", tt.query, results[0].Command.ID, tt.wantFirst)
			}
		})
	}
}

func TestFilterWordBoundary(t *testing.T) {
	f := NewFilter()

	commands := []*Command{
		{ID: "saveFile", Title: "saveFile"},
		{ID: "safe", Title: "safe"},
	}

	// "sf" should prefer "saveFile" due to word boundary match
	results := f.Search(commands, "sf", 10)
	if len(results) == 0 {
		t.Fatal("expected matches")
	}
	// Both should match, but saveFile should score higher (camelCase boundary)
	if results[0].Command.ID != "saveFile" {
		t.Errorf("expected saveFile first, got %s", results[0].Command.ID)
	}
}

func TestPaletteRegister(t *testing.T) {
	p := New()

	cmd := &Command{
		ID:      "test.cmd",
		Title:   "Test Command",
		Handler: func(args map[string]any) error { return nil },
	}

	err := p.Register(cmd)
	if err != nil {
		t.Errorf("Register() error = %v", err)
	}

	if !p.Has("test.cmd") {
		t.Error("command not registered")
	}

	if p.Count() != 1 {
		t.Errorf("expected count 1, got %d", p.Count())
	}
}

func TestPaletteRegisterValidation(t *testing.T) {
	p := New()

	// Nil command
	if err := p.Register(nil); err == nil {
		t.Error("expected error for nil command")
	}

	// Empty ID
	if err := p.Register(&Command{Title: "Test"}); err == nil {
		t.Error("expected error for empty ID")
	}

	// Empty title
	if err := p.Register(&Command{ID: "test"}); err == nil {
		t.Error("expected error for empty title")
	}
}

func TestPaletteUnregister(t *testing.T) {
	p := New()
	p.Register(&Command{ID: "test.cmd", Title: "Test"})

	if !p.Unregister("test.cmd") {
		t.Error("Unregister() returned false")
	}

	if p.Has("test.cmd") {
		t.Error("command still exists after unregister")
	}

	if p.Unregister("test.cmd") {
		t.Error("Unregister() returned true for non-existent command")
	}
}

func TestPaletteSearch(t *testing.T) {
	p := New()

	p.Register(&Command{ID: "file.save", Title: "Save File", Category: "File"})
	p.Register(&Command{ID: "file.open", Title: "Open File", Category: "File"})
	p.Register(&Command{ID: "edit.copy", Title: "Copy", Category: "Edit"})

	// Empty query returns all
	results := p.Search("", 10)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Fuzzy search
	results = p.Search("save", 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Command.ID != "file.save" {
		t.Errorf("expected file.save, got %s", results[0].Command.ID)
	}
}

func TestPaletteSearchWithHistory(t *testing.T) {
	p := New()

	p.Register(&Command{ID: "file.save", Title: "Save File"})
	p.Register(&Command{ID: "file.open", Title: "Open File"})

	// Execute open to add to history
	p.Execute("file.open", nil)

	// Empty search should show recent first
	results := p.Search("", 10)
	if results[0].Command.ID != "file.open" {
		t.Errorf("expected file.open first (recent), got %s", results[0].Command.ID)
	}

	// Search should also boost recent
	results = p.Search("file", 10)
	if results[0].Command.ID != "file.open" {
		t.Errorf("expected file.open first (recent + match), got %s", results[0].Command.ID)
	}
}

func TestPaletteExecute(t *testing.T) {
	p := New()
	executed := false

	p.Register(&Command{
		ID:      "test.cmd",
		Title:   "Test",
		Handler: func(args map[string]any) error { executed = true; return nil },
	})

	err := p.Execute("test.cmd", nil)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if !executed {
		t.Error("command was not executed")
	}

	// Unknown command
	err = p.Execute("unknown", nil)
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestPaletteExecuteAddsToHistory(t *testing.T) {
	p := New()

	p.Register(&Command{
		ID:      "test.cmd",
		Title:   "Test",
		Handler: func(args map[string]any) error { return nil },
	})

	p.Execute("test.cmd", nil)

	recent := p.RecentCommands(10)
	if len(recent) != 1 || recent[0] != "test.cmd" {
		t.Errorf("expected test.cmd in history, got %v", recent)
	}
}

func TestPaletteCategories(t *testing.T) {
	p := New()

	p.Register(&Command{ID: "file.save", Title: "Save", Category: "File"})
	p.Register(&Command{ID: "file.open", Title: "Open", Category: "File"})
	p.Register(&Command{ID: "edit.copy", Title: "Copy", Category: "Edit"})

	cats := p.Categories()
	if len(cats) != 2 {
		t.Errorf("expected 2 categories, got %d", len(cats))
	}
}

func TestPaletteCommandsByCategory(t *testing.T) {
	p := New()

	p.Register(&Command{ID: "file.save", Title: "Save", Category: "File"})
	p.Register(&Command{ID: "file.open", Title: "Open", Category: "File"})
	p.Register(&Command{ID: "edit.copy", Title: "Copy", Category: "Edit"})

	fileCmds := p.CommandsByCategory("File")
	if len(fileCmds) != 2 {
		t.Errorf("expected 2 File commands, got %d", len(fileCmds))
	}
}

func TestPaletteUnregisterBySource(t *testing.T) {
	p := New()

	p.Register(&Command{ID: "core.cmd", Title: "Core", Source: "core"})
	p.Register(&Command{ID: "plugin.cmd", Title: "Plugin", Source: "plugin:git"})
	p.Register(&Command{ID: "plugin.cmd2", Title: "Plugin2", Source: "plugin:git"})

	count := p.UnregisterBySource("plugin:git")
	if count != 2 {
		t.Errorf("expected 2 unregistered, got %d", count)
	}

	if p.Has("plugin.cmd") || p.Has("plugin.cmd2") {
		t.Error("plugin commands still exist")
	}

	if !p.Has("core.cmd") {
		t.Error("core command was removed")
	}
}

func TestPaletteOnChange(t *testing.T) {
	p := New()
	changes := 0

	p.OnChange(func() {
		changes++
	})

	p.Register(&Command{ID: "test", Title: "Test"})
	if changes != 1 {
		t.Errorf("expected 1 change, got %d", changes)
	}

	p.Unregister("test")
	if changes != 2 {
		t.Errorf("expected 2 changes, got %d", changes)
	}
}

func TestPaletteConcurrency(t *testing.T) {
	p := New()
	var wg sync.WaitGroup
	var ops int64

	// Register commands concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p.Register(&Command{
				ID:    "cmd" + string(rune('0'+i%10)),
				Title: "Command",
			})
			atomic.AddInt64(&ops, 1)
		}(i)
	}

	// Search concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Search("cmd", 10)
			atomic.AddInt64(&ops, 1)
		}()
	}

	wg.Wait()

	if ops != 200 {
		t.Errorf("expected 200 ops, got %d", ops)
	}
}

func TestPaletteClear(t *testing.T) {
	p := New()

	p.Register(&Command{ID: "test1", Title: "Test1"})
	p.Register(&Command{ID: "test2", Title: "Test2"})
	p.Execute("test1", nil)

	p.Clear()

	if p.Count() != 0 {
		t.Errorf("expected 0 commands, got %d", p.Count())
	}

	if p.History().Len() != 0 {
		t.Errorf("expected 0 history, got %d", p.History().Len())
	}
}

func TestCommandHandlerError(t *testing.T) {
	p := New()

	expectedErr := errors.New("handler error")
	p.Register(&Command{
		ID:    "test.err",
		Title: "Error",
		Handler: func(args map[string]any) error {
			return expectedErr
		},
	})

	err := p.Execute("test.err", nil)
	if err != expectedErr {
		t.Errorf("expected handler error, got %v", err)
	}
}

func TestArgTypeString(t *testing.T) {
	tests := []struct {
		argType ArgType
		want    string
	}{
		{ArgString, "string"},
		{ArgNumber, "number"},
		{ArgBoolean, "boolean"},
		{ArgFile, "file"},
		{ArgEnum, "enum"},
		{ArgType(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.argType.String(); got != tt.want {
			t.Errorf("ArgType(%d).String() = %s, want %s", tt.argType, got, tt.want)
		}
	}
}

func TestFilterLimit(t *testing.T) {
	f := NewFilter()

	commands := make([]*Command, 100)
	for i := 0; i < 100; i++ {
		commands[i] = &Command{
			ID:    "cmd",
			Title: "Command",
		}
	}

	results := f.Search(commands, "cmd", 10)
	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}
}

func TestFilterEmptyQuery(t *testing.T) {
	f := NewFilter()

	commands := []*Command{
		{ID: "cmd1", Title: "First"},
		{ID: "cmd2", Title: "Second"},
	}

	results := f.Search(commands, "", 10)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestFilterByCategory(t *testing.T) {
	f := NewFilter()

	commands := []*Command{
		{ID: "file.save", Title: "Save", Category: "File"},
		{ID: "edit.copy", Title: "Copy", Category: "Edit"},
	}

	filtered := f.FilterByCategory(commands, "File")
	if len(filtered) != 1 {
		t.Errorf("expected 1 result, got %d", len(filtered))
	}
}

func TestFilterBySource(t *testing.T) {
	f := NewFilter()

	commands := []*Command{
		{ID: "core.cmd", Title: "Core", Source: "core"},
		{ID: "plugin.cmd", Title: "Plugin", Source: "plugin"},
	}

	filtered := f.FilterBySource(commands, "core")
	if len(filtered) != 1 {
		t.Errorf("expected 1 result, got %d", len(filtered))
	}
}

func BenchmarkPaletteSearch(b *testing.B) {
	p := New()

	// Register 1000 commands
	for i := 0; i < 1000; i++ {
		p.Register(&Command{
			ID:          "command" + string(rune('0'+i%10)),
			Title:       "Some Command Title",
			Description: "This is a description of the command",
			Category:    "Category",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Search("cmd", 10)
	}
}

func BenchmarkFilterFuzzyMatch(b *testing.B) {
	f := NewFilter()

	commands := make([]*Command, 1000)
	for i := 0; i < 1000; i++ {
		commands[i] = &Command{
			ID:          "command" + string(rune('0'+i%10)),
			Title:       "Some Command Title",
			Description: "Description",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Search(commands, "sct", 10)
	}
}
