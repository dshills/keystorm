package search

import (
	"errors"
	"testing"
)

func TestMatchMode_String(t *testing.T) {
	tests := []struct {
		mode MatchMode
		want string
	}{
		{MatchFuzzy, "fuzzy"},
		{MatchExact, "exact"},
		{MatchPrefix, "prefix"},
		{MatchContains, "contains"},
		{MatchGlob, "glob"},
		{MatchRegex, "regex"},
		{MatchMode(100), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("MatchMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestDefaultFileSearchOptions(t *testing.T) {
	opts := DefaultFileSearchOptions()

	if opts.MaxResults != 100 {
		t.Errorf("MaxResults = %d, want 100", opts.MaxResults)
	}
	if opts.MatchMode != MatchFuzzy {
		t.Errorf("MatchMode = %v, want MatchFuzzy", opts.MatchMode)
	}
	if opts.IncludeDirs {
		t.Error("IncludeDirs should be false by default")
	}
	if opts.CaseSensitive {
		t.Error("CaseSensitive should be false by default")
	}
	if !opts.BoostRecent {
		t.Error("BoostRecent should be true by default")
	}
}

func TestDefaultContentSearchOptions(t *testing.T) {
	opts := DefaultContentSearchOptions()

	if opts.MaxResults != 1000 {
		t.Errorf("MaxResults = %d, want 1000", opts.MaxResults)
	}
	if opts.CaseSensitive {
		t.Error("CaseSensitive should be false by default")
	}
	if opts.WholeWord {
		t.Error("WholeWord should be false by default")
	}
	if opts.UseRegex {
		t.Error("UseRegex should be false by default")
	}
	if opts.ContextLines != 2 {
		t.Errorf("ContextLines = %d, want 2", opts.ContextLines)
	}
	if opts.MaxFileSize != 10*1024*1024 {
		t.Errorf("MaxFileSize = %d, want 10MB", opts.MaxFileSize)
	}
}

func TestCompileQuery_Simple(t *testing.T) {
	opts := DefaultContentSearchOptions()
	re, err := CompileQuery("hello", opts)
	if err != nil {
		t.Fatalf("CompileQuery error = %v", err)
	}

	// Should match case-insensitively
	if !re.MatchString("Hello") {
		t.Error("Should match 'Hello' case-insensitively")
	}
	if !re.MatchString("HELLO") {
		t.Error("Should match 'HELLO' case-insensitively")
	}
	if !re.MatchString("say hello world") {
		t.Error("Should match 'say hello world'")
	}
}

func TestCompileQuery_CaseSensitive(t *testing.T) {
	opts := DefaultContentSearchOptions()
	opts.CaseSensitive = true

	re, err := CompileQuery("hello", opts)
	if err != nil {
		t.Fatalf("CompileQuery error = %v", err)
	}

	if !re.MatchString("hello") {
		t.Error("Should match 'hello'")
	}
	if re.MatchString("Hello") {
		t.Error("Should not match 'Hello' with case sensitivity")
	}
}

func TestCompileQuery_WholeWord(t *testing.T) {
	opts := DefaultContentSearchOptions()
	opts.WholeWord = true

	re, err := CompileQuery("hello", opts)
	if err != nil {
		t.Fatalf("CompileQuery error = %v", err)
	}

	if !re.MatchString("say hello world") {
		t.Error("Should match 'say hello world'")
	}
	if re.MatchString("helloworld") {
		t.Error("Should not match 'helloworld' with whole word")
	}
}

func TestCompileQuery_Regex(t *testing.T) {
	opts := DefaultContentSearchOptions()
	opts.UseRegex = true

	re, err := CompileQuery("hel+o", opts)
	if err != nil {
		t.Fatalf("CompileQuery error = %v", err)
	}

	if !re.MatchString("hello") {
		t.Error("Should match 'hello'")
	}
	if !re.MatchString("helllo") {
		t.Error("Should match 'helllo'")
	}
	if re.MatchString("heo") {
		t.Error("Should not match 'heo'")
	}
}

func TestCompileQuery_InvalidRegex(t *testing.T) {
	opts := DefaultContentSearchOptions()
	opts.UseRegex = true

	_, err := CompileQuery("[invalid", opts)
	if !errors.Is(err, ErrInvalidQuery) {
		t.Errorf("CompileQuery error = %v, want ErrInvalidQuery (wrapped)", err)
	}
}

func TestCompileQuery_EscapesSpecialChars(t *testing.T) {
	opts := DefaultContentSearchOptions()

	// Without regex mode, special characters should be escaped
	re, err := CompileQuery("file.go", opts)
	if err != nil {
		t.Fatalf("CompileQuery error = %v", err)
	}

	if !re.MatchString("file.go") {
		t.Error("Should match 'file.go'")
	}
	if re.MatchString("filexgo") {
		t.Error("Should not match 'filexgo' (dot should be literal)")
	}
}
