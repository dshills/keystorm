package lsp

import (
	"testing"
	"time"
)

func TestNewDiagnosticsService(t *testing.T) {
	ds := NewDiagnosticsService(nil)
	if ds == nil {
		t.Fatal("NewDiagnosticsService returned nil")
	}

	if ds.minSeverity != DiagnosticSeverityHint {
		t.Errorf("Default minSeverity: got %d, want %d", ds.minSeverity, DiagnosticSeverityHint)
	}

	if ds.maxPerFile != 1000 {
		t.Errorf("Default maxPerFile: got %d, want 1000", ds.maxPerFile)
	}

	// With options
	ds = NewDiagnosticsService(nil,
		WithMinSeverity(DiagnosticSeverityWarning),
		WithMaxDiagnosticsPerFile(50),
		WithDiagnosticsDebounce(200*time.Millisecond),
	)

	if ds.minSeverity != DiagnosticSeverityWarning {
		t.Errorf("Custom minSeverity: got %d, want %d", ds.minSeverity, DiagnosticSeverityWarning)
	}

	if ds.maxPerFile != 50 {
		t.Errorf("Custom maxPerFile: got %d, want 50", ds.maxPerFile)
	}
}

func TestDiagnosticsService_HandleDiagnostics(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	uri := DocumentURI("file:///test/file.go")
	diagnostics := []Diagnostic{
		{
			Range:    Range{Start: Position{Line: 5, Character: 0}},
			Severity: DiagnosticSeverityError,
			Message:  "undefined: foo",
			Source:   "compiler",
		},
		{
			Range:    Range{Start: Position{Line: 10, Character: 0}},
			Severity: DiagnosticSeverityWarning,
			Message:  "unused variable",
			Source:   "compiler",
		},
		{
			Range:    Range{Start: Position{Line: 2, Character: 0}},
			Severity: DiagnosticSeverityHint,
			Message:  "consider renaming",
			Source:   "linter",
		},
	}

	// Simulate receiving diagnostics
	ds.handleDiagnostics(uri, diagnostics)

	// Check diagnostics were stored
	path := "/test/file.go"
	stored := ds.GetDiagnostics(path)
	if len(stored) != 3 {
		t.Fatalf("Expected 3 diagnostics, got %d", len(stored))
	}

	// Should be sorted by line
	if stored[0].Range.Start.Line != 2 {
		t.Errorf("First diagnostic should be at line 2, got %d", stored[0].Range.Start.Line)
	}
	if stored[1].Range.Start.Line != 5 {
		t.Errorf("Second diagnostic should be at line 5, got %d", stored[1].Range.Start.Line)
	}
}

func TestDiagnosticsService_FilterBySeverity(t *testing.T) {
	ds := NewDiagnosticsService(nil, WithMinSeverity(DiagnosticSeverityWarning))

	uri := DocumentURI("file:///test/file.go")
	diagnostics := []Diagnostic{
		{
			Range:    Range{Start: Position{Line: 5}},
			Severity: DiagnosticSeverityError,
			Message:  "error",
		},
		{
			Range:    Range{Start: Position{Line: 10}},
			Severity: DiagnosticSeverityWarning,
			Message:  "warning",
		},
		{
			Range:    Range{Start: Position{Line: 15}},
			Severity: DiagnosticSeverityHint,
			Message:  "hint - should be filtered",
		},
	}

	ds.handleDiagnostics(uri, diagnostics)

	// Hint should be filtered out
	stored := ds.GetDiagnostics("/test/file.go")
	if len(stored) != 2 {
		t.Fatalf("Expected 2 diagnostics (hint filtered), got %d", len(stored))
	}
}

func TestDiagnosticsService_FilterBySource(t *testing.T) {
	ds := NewDiagnosticsService(nil, WithEnabledSources([]string{"compiler"}))

	uri := DocumentURI("file:///test/file.go")
	diagnostics := []Diagnostic{
		{
			Range:    Range{Start: Position{Line: 5}},
			Severity: DiagnosticSeverityError,
			Message:  "from compiler",
			Source:   "compiler",
		},
		{
			Range:    Range{Start: Position{Line: 10}},
			Severity: DiagnosticSeverityWarning,
			Message:  "from linter - should be filtered",
			Source:   "linter",
		},
	}

	ds.handleDiagnostics(uri, diagnostics)

	stored := ds.GetDiagnostics("/test/file.go")
	if len(stored) != 1 {
		t.Fatalf("Expected 1 diagnostic (linter filtered), got %d", len(stored))
	}
	if stored[0].Source != "compiler" {
		t.Errorf("Expected compiler source, got %s", stored[0].Source)
	}
}

func TestDiagnosticsService_GetFileDiagnostics(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	uri := DocumentURI("file:///test/file.go")
	diagnostics := []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "e1"},
		{Severity: DiagnosticSeverityError, Message: "e2"},
		{Severity: DiagnosticSeverityWarning, Message: "w1"},
		{Severity: DiagnosticSeverityInformation, Message: "i1"},
		{Severity: DiagnosticSeverityHint, Message: "h1"},
	}

	ds.handleDiagnostics(uri, diagnostics)

	fd, ok := ds.GetFileDiagnostics("/test/file.go")
	if !ok {
		t.Fatal("GetFileDiagnostics returned false")
	}

	if fd.ErrorCount != 2 {
		t.Errorf("ErrorCount: got %d, want 2", fd.ErrorCount)
	}
	if fd.WarningCount != 1 {
		t.Errorf("WarningCount: got %d, want 1", fd.WarningCount)
	}
	if fd.InfoCount != 1 {
		t.Errorf("InfoCount: got %d, want 1", fd.InfoCount)
	}
	if fd.HintCount != 1 {
		t.Errorf("HintCount: got %d, want 1", fd.HintCount)
	}
}

func TestDiagnosticsService_GetDiagnosticsAtLine(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	uri := DocumentURI("file:///test/file.go")
	diagnostics := []Diagnostic{
		{Range: Range{Start: Position{Line: 5}, End: Position{Line: 5}}, Message: "line 5"},
		{Range: Range{Start: Position{Line: 10}, End: Position{Line: 12}}, Message: "lines 10-12"},
		{Range: Range{Start: Position{Line: 20}, End: Position{Line: 20}}, Message: "line 20"},
	}

	ds.handleDiagnostics(uri, diagnostics)

	// Get diagnostics at line 5
	atLine5 := ds.GetDiagnosticsAtLine("/test/file.go", 5)
	if len(atLine5) != 1 {
		t.Errorf("Expected 1 diagnostic at line 5, got %d", len(atLine5))
	}

	// Get diagnostics at line 11 (within range 10-12)
	atLine11 := ds.GetDiagnosticsAtLine("/test/file.go", 11)
	if len(atLine11) != 1 {
		t.Errorf("Expected 1 diagnostic at line 11, got %d", len(atLine11))
	}

	// Get diagnostics at line 15 (no diagnostics)
	atLine15 := ds.GetDiagnosticsAtLine("/test/file.go", 15)
	if len(atLine15) != 0 {
		t.Errorf("Expected 0 diagnostics at line 15, got %d", len(atLine15))
	}
}

func TestDiagnosticsService_GetDiagnosticsAtPosition(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	uri := DocumentURI("file:///test/file.go")
	diagnostics := []Diagnostic{
		{
			Range:   Range{Start: Position{Line: 5, Character: 10}, End: Position{Line: 5, Character: 20}},
			Message: "at position",
		},
	}

	ds.handleDiagnostics(uri, diagnostics)

	// Position within range
	atPos := ds.GetDiagnosticsAtPosition("/test/file.go", Position{Line: 5, Character: 15})
	if len(atPos) != 1 {
		t.Errorf("Expected 1 diagnostic at position, got %d", len(atPos))
	}

	// Position outside range
	outsidePos := ds.GetDiagnosticsAtPosition("/test/file.go", Position{Line: 5, Character: 5})
	if len(outsidePos) != 0 {
		t.Errorf("Expected 0 diagnostics outside range, got %d", len(outsidePos))
	}
}

func TestDiagnosticsService_GetErrors(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	uri := DocumentURI("file:///test/file.go")
	diagnostics := []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "error1"},
		{Severity: DiagnosticSeverityWarning, Message: "warning"},
		{Severity: DiagnosticSeverityError, Message: "error2"},
	}

	ds.handleDiagnostics(uri, diagnostics)

	errors := ds.GetErrors("/test/file.go")
	if len(errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errors))
	}
}

func TestDiagnosticsService_Summary(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	// Add diagnostics for multiple files
	ds.handleDiagnostics(DocumentURI("file:///test/a.go"), []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "e"},
		{Severity: DiagnosticSeverityWarning, Message: "w"},
	})
	ds.handleDiagnostics(DocumentURI("file:///test/b.go"), []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "e"},
		{Severity: DiagnosticSeverityInformation, Message: "i"},
	})
	ds.handleDiagnostics(DocumentURI("file:///test/c.go"), []Diagnostic{
		{Severity: DiagnosticSeverityWarning, Message: "w"},
	})

	summary := ds.Summary()

	if summary.TotalFiles != 3 {
		t.Errorf("TotalFiles: got %d, want 3", summary.TotalFiles)
	}
	if summary.TotalErrors != 2 {
		t.Errorf("TotalErrors: got %d, want 2", summary.TotalErrors)
	}
	if summary.TotalWarns != 2 {
		t.Errorf("TotalWarns: got %d, want 2", summary.TotalWarns)
	}
	if summary.FilesWithErr != 2 {
		t.Errorf("FilesWithErr: got %d, want 2", summary.FilesWithErr)
	}
}

func TestDiagnosticsService_Clear(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	ds.handleDiagnostics(DocumentURI("file:///test/a.go"), []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "e"},
	})

	if !ds.HasDiagnostics("/test/a.go") {
		t.Error("Expected diagnostics before clear")
	}

	ds.Clear()

	if ds.HasDiagnostics("/test/a.go") {
		t.Error("Expected no diagnostics after clear")
	}
}

func TestDiagnosticsService_ClearFile(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	ds.handleDiagnostics(DocumentURI("file:///test/a.go"), []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "e"},
	})
	ds.handleDiagnostics(DocumentURI("file:///test/b.go"), []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "e"},
	})

	ds.ClearFile("/test/a.go")

	if ds.HasDiagnostics("/test/a.go") {
		t.Error("Expected no diagnostics for a.go after clear")
	}
	if !ds.HasDiagnostics("/test/b.go") {
		t.Error("Expected diagnostics for b.go still present")
	}
}

func TestDiagnosticsService_HasErrors(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	// File with only warnings
	ds.handleDiagnostics(DocumentURI("file:///test/warnings.go"), []Diagnostic{
		{Severity: DiagnosticSeverityWarning, Message: "w"},
	})

	// File with errors
	ds.handleDiagnostics(DocumentURI("file:///test/errors.go"), []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "e"},
	})

	if ds.HasErrors("/test/warnings.go") {
		t.Error("warnings.go should not have errors")
	}
	if !ds.HasErrors("/test/errors.go") {
		t.Error("errors.go should have errors")
	}
}

func TestDiagnosticsService_FilesWithErrors(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	ds.handleDiagnostics(DocumentURI("file:///test/a.go"), []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "e"},
	})
	ds.handleDiagnostics(DocumentURI("file:///test/b.go"), []Diagnostic{
		{Severity: DiagnosticSeverityWarning, Message: "w"},
	})
	ds.handleDiagnostics(DocumentURI("file:///test/c.go"), []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "e"},
	})

	files := ds.FilesWithErrors()
	if len(files) != 2 {
		t.Errorf("Expected 2 files with errors, got %d", len(files))
	}
}

func TestDiagnosticsService_NextDiagnostic(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	uri := DocumentURI("file:///test/file.go")
	diagnostics := []Diagnostic{
		{Range: Range{Start: Position{Line: 5, Character: 0}}, Message: "d1"},
		{Range: Range{Start: Position{Line: 10, Character: 0}}, Message: "d2"},
		{Range: Range{Start: Position{Line: 15, Character: 0}}, Message: "d3"},
	}

	ds.handleDiagnostics(uri, diagnostics)

	// Next after line 0
	next := ds.NextDiagnostic("/test/file.go", Position{Line: 0, Character: 0}, false)
	if next == nil || next.Range.Start.Line != 5 {
		t.Errorf("Expected next at line 5")
	}

	// Next after line 10
	next = ds.NextDiagnostic("/test/file.go", Position{Line: 10, Character: 0}, false)
	if next == nil || next.Range.Start.Line != 15 {
		t.Errorf("Expected next at line 15")
	}

	// Next after line 15, no wrap
	next = ds.NextDiagnostic("/test/file.go", Position{Line: 15, Character: 0}, false)
	if next != nil {
		t.Error("Expected nil when no more diagnostics")
	}

	// Next after line 15, with wrap
	next = ds.NextDiagnostic("/test/file.go", Position{Line: 15, Character: 0}, true)
	if next == nil || next.Range.Start.Line != 5 {
		t.Error("Expected wrap to first diagnostic")
	}
}

func TestDiagnosticsService_PrevDiagnostic(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	uri := DocumentURI("file:///test/file.go")
	diagnostics := []Diagnostic{
		{Range: Range{Start: Position{Line: 5, Character: 0}}, Message: "d1"},
		{Range: Range{Start: Position{Line: 10, Character: 0}}, Message: "d2"},
		{Range: Range{Start: Position{Line: 15, Character: 0}}, Message: "d3"},
	}

	ds.handleDiagnostics(uri, diagnostics)

	// Prev before line 20
	prev := ds.PrevDiagnostic("/test/file.go", Position{Line: 20, Character: 0}, false)
	if prev == nil || prev.Range.Start.Line != 15 {
		t.Error("Expected prev at line 15")
	}

	// Prev before line 5, no wrap
	prev = ds.PrevDiagnostic("/test/file.go", Position{Line: 5, Character: 0}, false)
	if prev != nil {
		t.Error("Expected nil when no previous diagnostics")
	}

	// Prev before line 5, with wrap
	prev = ds.PrevDiagnostic("/test/file.go", Position{Line: 5, Character: 0}, true)
	if prev == nil || prev.Range.Start.Line != 15 {
		t.Error("Expected wrap to last diagnostic")
	}
}

func TestDiagnosticSeverityString(t *testing.T) {
	tests := []struct {
		severity DiagnosticSeverity
		want     string
	}{
		{DiagnosticSeverityError, "Error"},
		{DiagnosticSeverityWarning, "Warning"},
		{DiagnosticSeverityInformation, "Information"},
		{DiagnosticSeverityHint, "Hint"},
		{DiagnosticSeverity(99), "Unknown"},
	}

	for _, tt := range tests {
		got := DiagnosticSeverityString(tt.severity)
		if got != tt.want {
			t.Errorf("DiagnosticSeverityString(%d) = %q, want %q", tt.severity, got, tt.want)
		}
	}
}

func TestDiagnosticSeverityIcon(t *testing.T) {
	tests := []struct {
		severity DiagnosticSeverity
		want     string
	}{
		{DiagnosticSeverityError, "E"},
		{DiagnosticSeverityWarning, "W"},
		{DiagnosticSeverityInformation, "I"},
		{DiagnosticSeverityHint, "H"},
	}

	for _, tt := range tests {
		got := DiagnosticSeverityIcon(tt.severity)
		if got != tt.want {
			t.Errorf("DiagnosticSeverityIcon(%d) = %q, want %q", tt.severity, got, tt.want)
		}
	}
}

func TestFormatDiagnostic(t *testing.T) {
	d := Diagnostic{
		Severity: DiagnosticSeverityError,
		Source:   "compiler",
		Message:  "undefined: foo",
		Code:     "E001",
	}

	formatted := FormatDiagnostic(d)
	if formatted != "E [compiler] undefined: foo (E001)" {
		t.Errorf("FormatDiagnostic = %q", formatted)
	}

	// Without source
	d2 := Diagnostic{
		Severity: DiagnosticSeverityWarning,
		Message:  "unused variable",
	}
	formatted2 := FormatDiagnostic(d2)
	if formatted2 != "W unused variable" {
		t.Errorf("FormatDiagnostic without source = %q", formatted2)
	}
}

func TestFormatDiagnosticWithLocation(t *testing.T) {
	d := Diagnostic{
		Range:    Range{Start: Position{Line: 9, Character: 4}},
		Severity: DiagnosticSeverityError,
		Message:  "undefined: foo",
	}

	formatted := FormatDiagnosticWithLocation("/test/file.go", d)
	// Line/char are 0-indexed, should display as 1-indexed
	expected := "/test/file.go:10:5: E undefined: foo"
	if formatted != expected {
		t.Errorf("FormatDiagnosticWithLocation = %q, want %q", formatted, expected)
	}
}

func TestPositionInRange(t *testing.T) {
	rng := Range{
		Start: Position{Line: 5, Character: 10},
		End:   Position{Line: 5, Character: 20},
	}

	tests := []struct {
		pos  Position
		want bool
	}{
		{Position{Line: 5, Character: 15}, true},  // In range
		{Position{Line: 5, Character: 10}, true},  // At start
		{Position{Line: 5, Character: 20}, true},  // At end
		{Position{Line: 5, Character: 5}, false},  // Before start
		{Position{Line: 5, Character: 25}, false}, // After end
		{Position{Line: 4, Character: 15}, false}, // Wrong line
	}

	for _, tt := range tests {
		got := positionInRange(tt.pos, rng)
		if got != tt.want {
			t.Errorf("positionInRange(%v, %v) = %v, want %v", tt.pos, rng, got, tt.want)
		}
	}
}

func TestSortDiagnosticsBySeverity(t *testing.T) {
	diagnostics := []Diagnostic{
		{Severity: DiagnosticSeverityHint, Range: Range{Start: Position{Line: 1}}},
		{Severity: DiagnosticSeverityError, Range: Range{Start: Position{Line: 2}}},
		{Severity: DiagnosticSeverityWarning, Range: Range{Start: Position{Line: 3}}},
		{Severity: DiagnosticSeverityError, Range: Range{Start: Position{Line: 4}}},
	}

	sorted := SortDiagnosticsBySeverity(diagnostics)

	// Errors first (severity 1), then warnings (2), then hints (4)
	if sorted[0].Severity != DiagnosticSeverityError {
		t.Error("First should be error")
	}
	if sorted[1].Severity != DiagnosticSeverityError {
		t.Error("Second should be error")
	}
	if sorted[2].Severity != DiagnosticSeverityWarning {
		t.Error("Third should be warning")
	}
	if sorted[3].Severity != DiagnosticSeverityHint {
		t.Error("Fourth should be hint")
	}
}

func TestFilterDiagnosticsByPattern(t *testing.T) {
	diagnostics := []Diagnostic{
		{Message: "undefined: foo", Source: "compiler"},
		{Message: "unused variable bar", Source: "linter"},
		{Message: "type mismatch", Source: "compiler"},
	}

	// Filter by message
	filtered := FilterDiagnosticsByPattern(diagnostics, "undefined")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 match for 'undefined', got %d", len(filtered))
	}

	// Filter by source
	filtered = FilterDiagnosticsByPattern(diagnostics, "linter")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 match for 'linter', got %d", len(filtered))
	}

	// Case insensitive
	filtered = FilterDiagnosticsByPattern(diagnostics, "UNUSED")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 case-insensitive match, got %d", len(filtered))
	}
}

func TestDiagnosticsService_MaxPerFile(t *testing.T) {
	ds := NewDiagnosticsService(nil, WithMaxDiagnosticsPerFile(5))

	// Create 10 diagnostics
	var diagnostics []Diagnostic
	for i := 0; i < 10; i++ {
		diagnostics = append(diagnostics, Diagnostic{
			Range:    Range{Start: Position{Line: i}},
			Severity: DiagnosticSeverityWarning,
			Message:  "warning",
		})
	}

	ds.handleDiagnostics(DocumentURI("file:///test/file.go"), diagnostics)

	stored := ds.GetDiagnostics("/test/file.go")
	if len(stored) != 5 {
		t.Errorf("Expected max 5 diagnostics, got %d", len(stored))
	}
}

func TestDiagnosticsService_EmptyDiagnosticsClearFile(t *testing.T) {
	ds := NewDiagnosticsService(nil)

	// Add diagnostics
	ds.handleDiagnostics(DocumentURI("file:///test/file.go"), []Diagnostic{
		{Severity: DiagnosticSeverityError, Message: "e"},
	})

	if !ds.HasDiagnostics("/test/file.go") {
		t.Error("Expected diagnostics")
	}

	// Send empty diagnostics (clears the file)
	ds.handleDiagnostics(DocumentURI("file:///test/file.go"), []Diagnostic{})

	if ds.HasDiagnostics("/test/file.go") {
		t.Error("Expected no diagnostics after empty update")
	}
}
