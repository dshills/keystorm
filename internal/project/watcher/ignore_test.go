package watcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnorePatterns_AddPattern(t *testing.T) {
	ip := NewIgnorePatterns()

	// Add valid patterns
	if err := ip.AddPattern("*.log"); err != nil {
		t.Errorf("AddPattern(*.log) error = %v", err)
	}

	if err := ip.AddPattern("node_modules/"); err != nil {
		t.Errorf("AddPattern(node_modules/) error = %v", err)
	}

	if ip.Count() != 2 {
		t.Errorf("Count() = %d, want 2", ip.Count())
	}

	// Empty patterns should be skipped
	if err := ip.AddPattern(""); err != nil {
		t.Errorf("AddPattern('') error = %v", err)
	}
	if err := ip.AddPattern("#"); err != nil {
		t.Errorf("AddPattern('#') error = %v", err)
	}

	// Comments should be skipped
	if err := ip.AddPattern("# this is a comment"); err != nil {
		t.Errorf("AddPattern(comment) error = %v", err)
	}

	if ip.Count() != 2 {
		t.Errorf("Count() = %d after skipped patterns, want 2", ip.Count())
	}
}

func TestIgnorePatterns_Match_SimplePatterns(t *testing.T) {
	ip := NewIgnorePatterns()
	_ = ip.AddPattern("*.log")
	_ = ip.AddPattern("*.tmp")

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{"test.log", false, true},
		{"path/to/debug.log", false, true},
		{"file.tmp", false, true},
		{"test.txt", false, false},
		{"log.txt", false, false},
	}

	for _, tt := range tests {
		if got := ip.Match(tt.path, tt.isDir); got != tt.ignored {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.path, tt.isDir, got, tt.ignored)
		}
	}
}

func TestIgnorePatterns_Match_DirectoryPatterns(t *testing.T) {
	ip := NewIgnorePatterns()
	_ = ip.AddPattern("build/")
	_ = ip.AddPattern("node_modules/")

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{"build", true, true},
		{"build/output.js", false, false}, // Pattern is dir-only, but this matches basename
		{"node_modules", true, true},
		{"src/node_modules", true, true}, // Non-rooted patterns match anywhere
		{"build.txt", false, false},      // Not a directory
	}

	for _, tt := range tests {
		if got := ip.Match(tt.path, tt.isDir); got != tt.ignored {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.path, tt.isDir, got, tt.ignored)
		}
	}
}

func TestIgnorePatterns_Match_RootedPatterns(t *testing.T) {
	ip := NewIgnorePatterns()
	_ = ip.AddPattern("/build")

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{"build", true, true},
		{"build", false, true},
		{"src/build", true, false}, // Rooted patterns only match at root
	}

	for _, tt := range tests {
		if got := ip.Match(tt.path, tt.isDir); got != tt.ignored {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.path, tt.isDir, got, tt.ignored)
		}
	}
}

func TestIgnorePatterns_Match_NegationPatterns(t *testing.T) {
	ip := NewIgnorePatterns()
	_ = ip.AddPattern("*.log")
	_ = ip.AddPattern("!important.log")

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{"debug.log", false, true},
		{"important.log", false, false}, // Negated
		{"error.log", false, true},
	}

	for _, tt := range tests {
		if got := ip.Match(tt.path, tt.isDir); got != tt.ignored {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.path, tt.isDir, got, tt.ignored)
		}
	}
}

func TestIgnorePatterns_Match_DoubleGlob(t *testing.T) {
	ip := NewIgnorePatterns()
	_ = ip.AddPattern("**/node_modules/**")
	_ = ip.AddPattern("**/test/**")

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{"node_modules/package.json", false, true},
		{"src/node_modules/lodash", true, true},
		{"deep/path/node_modules/pkg", true, true},
		{"test/unit", true, true},
		{"src/test/integration", true, true},
		{"testing", true, false}, // "test" pattern should not match "testing"
	}

	for _, tt := range tests {
		if got := ip.Match(tt.path, tt.isDir); got != tt.ignored {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.path, tt.isDir, got, tt.ignored)
		}
	}
}

func TestIgnorePatterns_MatchRelative(t *testing.T) {
	ip := NewIgnorePatterns()
	_ = ip.AddPattern("/build")
	_ = ip.AddPattern("*.log")

	basePath := "/project"

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{"/project/build", true, true},
		{"/project/src/build", true, false}, // Rooted pattern
		{"/project/debug.log", false, true},
	}

	for _, tt := range tests {
		if got := ip.MatchRelative(tt.path, basePath, tt.isDir); got != tt.ignored {
			t.Errorf("MatchRelative(%q, %q, %v) = %v, want %v", tt.path, basePath, tt.isDir, got, tt.ignored)
		}
	}
}

func TestIgnorePatterns_AddFromFile(t *testing.T) {
	// Create a temporary gitignore file
	tmpDir := t.TempDir()
	gitignorePath := filepath.Join(tmpDir, ".gitignore")

	content := `# Comment line
*.log
node_modules/
!important.log
build/

# Another comment
*.tmp
`
	if err := os.WriteFile(gitignorePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ip := NewIgnorePatterns()
	if err := ip.AddFromFile(gitignorePath); err != nil {
		t.Fatalf("AddFromFile error = %v", err)
	}

	// Should have 5 patterns (comments and empty lines excluded)
	if ip.Count() != 5 {
		t.Errorf("Count() = %d, want 5", ip.Count())
	}

	// Verify patterns work
	if !ip.Match("test.log", false) {
		t.Error("*.log pattern should match test.log")
	}
	if ip.Match("important.log", false) {
		t.Error("important.log should be negated")
	}
}

func TestIgnorePatterns_AddFromFile_NotExists(t *testing.T) {
	ip := NewIgnorePatterns()
	err := ip.AddFromFile("/nonexistent/.gitignore")
	if err == nil {
		t.Error("AddFromFile should error for nonexistent file")
	}
}

func TestIgnorePatterns_Clear(t *testing.T) {
	ip := NewIgnorePatterns()
	_ = ip.AddPattern("*.log")
	_ = ip.AddPattern("*.tmp")

	if ip.Count() != 2 {
		t.Errorf("Count() = %d before clear, want 2", ip.Count())
	}

	ip.Clear()

	if ip.Count() != 0 {
		t.Errorf("Count() = %d after clear, want 0", ip.Count())
	}
}

func TestIgnorePatterns_Patterns(t *testing.T) {
	ip := NewIgnorePatterns()
	_ = ip.AddPattern("*.log")
	_ = ip.AddPattern("!important.log")
	_ = ip.AddPattern("build/")

	patterns := ip.Patterns()

	if len(patterns) != 3 {
		t.Errorf("Patterns() length = %d, want 3", len(patterns))
	}

	expected := []string{"*.log", "!important.log", "build/"}
	for i, p := range patterns {
		if p != expected[i] {
			t.Errorf("patterns[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestNewDefaultIgnorePatterns(t *testing.T) {
	ip := NewDefaultIgnorePatterns()

	if ip.Count() == 0 {
		t.Error("Default patterns should not be empty")
	}

	// Test some default patterns work
	if !ip.Match(".git", true) {
		t.Error(".git should be ignored by default")
	}
	if !ip.Match("node_modules", true) {
		t.Error("node_modules should be ignored by default")
	}
	if !ip.Match("test.log", false) {
		t.Error("*.log should be ignored by default")
	}
}

func TestIgnorePatterns_ConcurrentAccess(t *testing.T) {
	ip := NewIgnorePatterns()

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = ip.AddPattern("*.log")
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = ip.Match("test.log", false)
			_ = ip.Count()
		}
		done <- true
	}()

	<-done
	<-done
}

func TestIgnorePatterns_ComplexPatterns(t *testing.T) {
	ip := NewIgnorePatterns()
	_ = ip.AddPattern("*.min.js")
	_ = ip.AddPattern("*.bundle.*")
	_ = ip.AddPattern("[Bb]uild/")

	tests := []struct {
		path    string
		isDir   bool
		ignored bool
	}{
		{"app.min.js", false, true},
		{"vendor.bundle.js", false, true},
		{"vendor.bundle.css", false, true},
		{"app.js", false, false},
		{"Build", true, true},
		{"build", true, true},
	}

	for _, tt := range tests {
		if got := ip.Match(tt.path, tt.isDir); got != tt.ignored {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.path, tt.isDir, got, tt.ignored)
		}
	}
}
