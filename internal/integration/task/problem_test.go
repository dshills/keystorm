package task

import (
	"testing"
)

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  ProblemSeverity
	}{
		{"error", ProblemSeverityError},
		{"Error", ProblemSeverityError},
		{"ERROR", ProblemSeverityError},
		{"fatal", ProblemSeverityError},
		{"Fatal", ProblemSeverityError},
		{"FATAL", ProblemSeverityError},
		{"warning", ProblemSeverityWarning},
		{"Warning", ProblemSeverityWarning},
		{"WARNING", ProblemSeverityWarning},
		{"warn", ProblemSeverityWarning},
		{"Warn", ProblemSeverityWarning},
		{"WARN", ProblemSeverityWarning},
		{"info", ProblemSeverityInfo},
		{"Info", ProblemSeverityInfo},
		{"INFO", ProblemSeverityInfo},
		{"note", ProblemSeverityInfo},
		{"Note", ProblemSeverityInfo},
		{"NOTE", ProblemSeverityInfo},
		{"unknown", ProblemSeverityError}, // default
	}

	for _, tt := range tests {
		got := parseSeverity(tt.input)
		if got != tt.want {
			t.Errorf("parseSeverity(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNewProblemMatcher(t *testing.T) {
	pm := NewProblemMatcher()
	if pm == nil {
		t.Fatal("NewProblemMatcher returned nil")
	}

	// Should have built-in matchers registered
	matchers := pm.ListMatchers()
	if len(matchers) == 0 {
		t.Error("no built-in matchers registered")
	}

	// Check for specific built-in matchers
	expectedMatchers := []string{"$gcc", "$go", "$tsc", "$generic"}
	for _, name := range expectedMatchers {
		if m := pm.GetMatcher(name); m == nil {
			t.Errorf("expected built-in matcher %q not found", name)
		}
	}
}

func TestProblemMatcher_Register(t *testing.T) {
	pm := NewProblemMatcher()

	def := ProblemMatcherDefinition{
		Name:  "custom",
		Owner: "test",
		Patterns: []ProblemPattern{
			{
				Pattern:         `^ERROR:\s*(.+)$`,
				Message:         1,
				DefaultSeverity: ProblemSeverityError,
			},
		},
	}

	err := pm.Register(def)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	matcher := pm.GetMatcher("custom")
	if matcher == nil {
		t.Error("registered matcher not found")
	}
}

func TestProblemMatcher_RegisterInvalidPattern(t *testing.T) {
	pm := NewProblemMatcher()

	def := ProblemMatcherDefinition{
		Name:  "invalid",
		Owner: "test",
		Patterns: []ProblemPattern{
			{
				Pattern: `[invalid`, // Invalid regex
			},
		},
	}

	err := pm.Register(def)
	if err == nil {
		t.Error("expected error for invalid pattern")
	}
}

func TestProblemMatcher_Unregister(t *testing.T) {
	pm := NewProblemMatcher()

	def := ProblemMatcherDefinition{
		Name:  "toremove",
		Owner: "test",
		Patterns: []ProblemPattern{
			{Pattern: `test`},
		},
	}

	_ = pm.Register(def)
	if pm.GetMatcher("toremove") == nil {
		t.Fatal("matcher not registered")
	}

	pm.Unregister("toremove")
	if pm.GetMatcher("toremove") != nil {
		t.Error("matcher still exists after unregister")
	}
}

func TestCompiledMatcher_MatchGCC(t *testing.T) {
	pm := NewProblemMatcher()
	matcher := pm.GetMatcher("$gcc")
	if matcher == nil {
		t.Fatal("$gcc matcher not found")
	}

	tests := []struct {
		line    string
		wantOK  bool
		file    string
		lineNum int
		col     int
		sev     ProblemSeverity
		msg     string
	}{
		{
			line:    "main.c:10:5: error: expected ';' before '}' token",
			wantOK:  true,
			file:    "main.c",
			lineNum: 10,
			col:     5,
			sev:     ProblemSeverityError,
			msg:     "expected ';' before '}' token",
		},
		{
			line:    "utils.c:20:3: warning: unused variable 'x'",
			wantOK:  true,
			file:    "utils.c",
			lineNum: 20,
			col:     3,
			sev:     ProblemSeverityWarning,
			msg:     "unused variable 'x'",
		},
		{
			line:    "header.h:5: note: declared here",
			wantOK:  true,
			file:    "header.h",
			lineNum: 5,
			sev:     ProblemSeverityInfo,
			msg:     "declared here",
		},
		{
			line:   "not a problem line",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			problem, ok := matcher.Match(tt.line)
			if ok != tt.wantOK {
				t.Errorf("Match() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if !ok {
				return
			}

			if problem.File != tt.file {
				t.Errorf("File = %q, want %q", problem.File, tt.file)
			}
			if problem.Line != tt.lineNum {
				t.Errorf("Line = %d, want %d", problem.Line, tt.lineNum)
			}
			if tt.col > 0 && problem.Column != tt.col {
				t.Errorf("Column = %d, want %d", problem.Column, tt.col)
			}
			if problem.Severity != tt.sev {
				t.Errorf("Severity = %q, want %q", problem.Severity, tt.sev)
			}
			if problem.Message != tt.msg {
				t.Errorf("Message = %q, want %q", problem.Message, tt.msg)
			}
		})
	}
}

func TestCompiledMatcher_MatchGo(t *testing.T) {
	pm := NewProblemMatcher()
	matcher := pm.GetMatcher("$go")
	if matcher == nil {
		t.Fatal("$go matcher not found")
	}

	tests := []struct {
		line    string
		wantOK  bool
		file    string
		lineNum int
		col     int
		msg     string
	}{
		{
			line:    "main.go:15:10: undefined: someFunc",
			wantOK:  true,
			file:    "main.go",
			lineNum: 15,
			col:     10,
			msg:     "undefined: someFunc",
		},
		{
			line:    "pkg/util.go:30: syntax error",
			wantOK:  true,
			file:    "pkg/util.go",
			lineNum: 30,
			msg:     "syntax error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			problem, ok := matcher.Match(tt.line)
			if ok != tt.wantOK {
				t.Errorf("Match() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if !ok {
				return
			}

			if problem.File != tt.file {
				t.Errorf("File = %q, want %q", problem.File, tt.file)
			}
			if problem.Line != tt.lineNum {
				t.Errorf("Line = %d, want %d", problem.Line, tt.lineNum)
			}
			if tt.col > 0 && problem.Column != tt.col {
				t.Errorf("Column = %d, want %d", problem.Column, tt.col)
			}
			if problem.Message != tt.msg {
				t.Errorf("Message = %q, want %q", problem.Message, tt.msg)
			}
		})
	}
}

func TestCompiledMatcher_MatchTSC(t *testing.T) {
	pm := NewProblemMatcher()
	matcher := pm.GetMatcher("$tsc")
	if matcher == nil {
		t.Fatal("$tsc matcher not found")
	}

	line := "src/index.ts(42,15): error TS2339: Property 'foo' does not exist on type 'Bar'"
	problem, ok := matcher.Match(line)
	if !ok {
		t.Fatal("expected match")
	}

	if problem.File != "src/index.ts" {
		t.Errorf("File = %q, want %q", problem.File, "src/index.ts")
	}
	if problem.Line != 42 {
		t.Errorf("Line = %d, want 42", problem.Line)
	}
	if problem.Column != 15 {
		t.Errorf("Column = %d, want 15", problem.Column)
	}
	if problem.Severity != ProblemSeverityError {
		t.Errorf("Severity = %q, want error", problem.Severity)
	}
	if problem.Code != "TS2339" {
		t.Errorf("Code = %q, want TS2339", problem.Code)
	}
}

func TestProblemMatcher_MatchLine(t *testing.T) {
	pm := NewProblemMatcher()

	// Test matching with any registered matcher
	// Use a line that matches the $go pattern specifically
	line := "test.go:10:5: undefined: something"
	problem, matcherName, ok := pm.MatchLine(line)

	if !ok {
		t.Fatal("expected match")
	}

	if matcherName == "" {
		t.Error("matcher name should not be empty")
	}

	// The file should be extracted correctly
	// Note: Different matchers may match and extract differently
	if problem.File == "" {
		t.Error("File should not be empty")
	}

	// Just verify that some line number was extracted
	if problem.Line <= 0 {
		t.Errorf("Line = %d, should be positive", problem.Line)
	}

	// Test non-matching line
	_, _, ok = pm.MatchLine("this is not an error line")
	if ok {
		t.Error("expected no match for non-error line")
	}
}

func TestProblem_Fields(t *testing.T) {
	p := Problem{
		File:      "test.go",
		Line:      10,
		Column:    5,
		EndLine:   12,
		EndColumn: 15,
		Severity:  ProblemSeverityWarning,
		Code:      "W001",
		Message:   "test warning",
		Source:    "test-tool",
	}

	if p.File != "test.go" {
		t.Errorf("File = %q", p.File)
	}
	if p.Line != 10 {
		t.Errorf("Line = %d", p.Line)
	}
	if p.Column != 5 {
		t.Errorf("Column = %d", p.Column)
	}
	if p.EndLine != 12 {
		t.Errorf("EndLine = %d", p.EndLine)
	}
	if p.EndColumn != 15 {
		t.Errorf("EndColumn = %d", p.EndColumn)
	}
	if p.Severity != ProblemSeverityWarning {
		t.Errorf("Severity = %q", p.Severity)
	}
	if p.Code != "W001" {
		t.Errorf("Code = %q", p.Code)
	}
	if p.Message != "test warning" {
		t.Errorf("Message = %q", p.Message)
	}
	if p.Source != "test-tool" {
		t.Errorf("Source = %q", p.Source)
	}
}

func TestProblemMatcherDefinition_FileLocation(t *testing.T) {
	pm := NewProblemMatcher()

	def := ProblemMatcherDefinition{
		Name:  "absolute",
		Owner: "test",
		Patterns: []ProblemPattern{
			{
				Pattern: `^(.+):(\d+):\s*(.+)$`,
				File:    1,
				Line:    2,
				Message: 3,
			},
		},
		FileLocation: "absolute",
	}

	if err := pm.Register(def); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	matcher := pm.GetMatcher("absolute")
	if matcher == nil {
		t.Fatal("matcher not found")
	}

	if matcher.def.FileLocation != "absolute" {
		t.Errorf("FileLocation = %q, want absolute", matcher.def.FileLocation)
	}
}
