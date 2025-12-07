package task

import (
	"strings"
	"testing"
	"time"
)

func TestOutputStream_String(t *testing.T) {
	tests := []struct {
		stream OutputStream
		want   string
	}{
		{OutputStreamStdout, "stdout"},
		{OutputStreamStderr, "stderr"},
		{OutputStream(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.stream.String()
		if got != tt.want {
			t.Errorf("OutputStream(%d).String() = %q, want %q", tt.stream, got, tt.want)
		}
	}
}

func TestNewOutputProcessor(t *testing.T) {
	// Test default buffer size
	p := NewOutputProcessor(0)
	if p == nil {
		t.Fatal("NewOutputProcessor returned nil")
	}
	if p.bufferSize != 64*1024 {
		t.Errorf("default bufferSize = %d, want %d", p.bufferSize, 64*1024)
	}

	// Test custom buffer size
	p = NewOutputProcessor(1024)
	if p.bufferSize != 1024 {
		t.Errorf("custom bufferSize = %d, want 1024", p.bufferSize)
	}
}

func TestOutputProcessor_Process(t *testing.T) {
	p := NewOutputProcessor(1024)

	input := strings.NewReader("line1\nline2\nline3")
	var received []OutputLine

	p.Process(input, OutputStreamStdout, func(line OutputLine) {
		received = append(received, line)
	})

	if len(received) != 3 {
		t.Fatalf("got %d lines, want 3", len(received))
	}

	expected := []string{"line1", "line2", "line3"}
	for i, want := range expected {
		if received[i].Content != want {
			t.Errorf("line[%d].Content = %q, want %q", i, received[i].Content, want)
		}
		if received[i].Stream != OutputStreamStdout {
			t.Errorf("line[%d].Stream = %v, want stdout", i, received[i].Stream)
		}
		if received[i].LineNumber != i+1 {
			t.Errorf("line[%d].LineNumber = %d, want %d", i, received[i].LineNumber, i+1)
		}
	}
}

func TestOutputProcessor_Lines(t *testing.T) {
	p := NewOutputProcessor(1024)

	input := strings.NewReader("line1\nline2\nline3")
	p.Process(input, OutputStreamStdout, nil)

	lines := p.Lines()
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}

	// Verify it's a copy
	lines[0].Content = "modified"
	original := p.Lines()
	if original[0].Content == "modified" {
		t.Error("Lines() should return a copy, not the original slice")
	}
}

func TestOutputProcessor_StdoutStderrLines(t *testing.T) {
	p := NewOutputProcessor(1024)

	// Process stdout
	p.Process(strings.NewReader("stdout1\nstdout2"), OutputStreamStdout, nil)
	// Process stderr
	p.Process(strings.NewReader("stderr1"), OutputStreamStderr, nil)

	stdout := p.StdoutLines()
	if len(stdout) != 2 {
		t.Errorf("got %d stdout lines, want 2", len(stdout))
	}

	stderr := p.StderrLines()
	if len(stderr) != 1 {
		t.Errorf("got %d stderr lines, want 1", len(stderr))
	}
}

func TestOutputProcessor_LineCount(t *testing.T) {
	p := NewOutputProcessor(1024)

	p.Process(strings.NewReader("line1\nline2\nline3"), OutputStreamStdout, nil)

	if count := p.LineCount(); count != 3 {
		t.Errorf("LineCount() = %d, want 3", count)
	}
}

func TestOutputProcessor_LastLines(t *testing.T) {
	p := NewOutputProcessor(1024)

	p.Process(strings.NewReader("line1\nline2\nline3\nline4\nline5"), OutputStreamStdout, nil)

	tests := []struct {
		n    int
		want int
	}{
		{0, 0},
		{2, 2},
		{3, 3},
		{10, 5}, // More than available
	}

	for _, tt := range tests {
		got := p.LastLines(tt.n)
		if len(got) != tt.want {
			t.Errorf("LastLines(%d) returned %d lines, want %d", tt.n, len(got), tt.want)
		}
	}

	// Verify content of last 2 lines
	last2 := p.LastLines(2)
	if last2[0].Content != "line4" || last2[1].Content != "line5" {
		t.Errorf("LastLines(2) = [%q, %q], want [line4, line5]", last2[0].Content, last2[1].Content)
	}
}

func TestOutputProcessor_Content(t *testing.T) {
	p := NewOutputProcessor(1024)

	p.Process(strings.NewReader("line1\nline2\nline3"), OutputStreamStdout, nil)

	content := p.Content()
	want := "line1\nline2\nline3"
	if content != want {
		t.Errorf("Content() = %q, want %q", content, want)
	}
}

func TestOutputProcessor_StdoutStderrContent(t *testing.T) {
	p := NewOutputProcessor(1024)

	p.Process(strings.NewReader("stdout1\nstdout2"), OutputStreamStdout, nil)
	p.Process(strings.NewReader("stderr1"), OutputStreamStderr, nil)

	stdout := p.StdoutContent()
	if stdout != "stdout1\nstdout2" {
		t.Errorf("StdoutContent() = %q, want %q", stdout, "stdout1\nstdout2")
	}

	stderr := p.StderrContent()
	if stderr != "stderr1" {
		t.Errorf("StderrContent() = %q, want %q", stderr, "stderr1")
	}
}

func TestOutputProcessor_Clear(t *testing.T) {
	p := NewOutputProcessor(1024)

	p.Process(strings.NewReader("line1\nline2"), OutputStreamStdout, nil)

	if p.LineCount() != 2 {
		t.Fatalf("expected 2 lines before clear")
	}

	p.Clear()

	if p.LineCount() != 0 {
		t.Errorf("LineCount after Clear() = %d, want 0", p.LineCount())
	}
	if len(p.Lines()) != 0 {
		t.Errorf("Lines() after Clear() returned %d lines, want 0", len(p.Lines()))
	}
}

func TestOutputProcessor_ProcessAsync(t *testing.T) {
	p := NewOutputProcessor(1024)

	input := strings.NewReader("line1\nline2")
	errCh := p.ProcessAsync(input, OutputStreamStdout, nil)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ProcessAsync error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ProcessAsync didn't complete in time")
	}

	if p.LineCount() != 2 {
		t.Errorf("LineCount() = %d, want 2", p.LineCount())
	}
}

func TestOutputProcessor_ProcessError(t *testing.T) {
	p := NewOutputProcessor(1024)

	// Process returns nil error on successful completion
	err := p.Process(strings.NewReader("line1\nline2"), OutputStreamStdout, nil)
	if err != nil {
		t.Errorf("Process returned unexpected error: %v", err)
	}

	if p.LineCount() != 2 {
		t.Errorf("LineCount() = %d, want 2", p.LineCount())
	}
}

func TestNewOutputBuffer(t *testing.T) {
	// Test default capacity
	b := NewOutputBuffer(0)
	if b.capacity != 1000 {
		t.Errorf("default capacity = %d, want 1000", b.capacity)
	}

	// Test custom capacity
	b = NewOutputBuffer(100)
	if b.capacity != 100 {
		t.Errorf("custom capacity = %d, want 100", b.capacity)
	}
}

func TestOutputBuffer_Add(t *testing.T) {
	b := NewOutputBuffer(3)

	// Add lines
	for i := 0; i < 5; i++ {
		b.Add(OutputLine{Content: string(rune('a' + i))})
	}

	// Should only have last 3 due to ring buffer
	if b.Count() != 3 {
		t.Errorf("Count() = %d, want 3", b.Count())
	}

	lines := b.Lines()
	expected := []string{"c", "d", "e"}
	for i, want := range expected {
		if lines[i].Content != want {
			t.Errorf("lines[%d].Content = %q, want %q", i, lines[i].Content, want)
		}
	}
}

func TestOutputBuffer_Clear(t *testing.T) {
	b := NewOutputBuffer(10)

	b.Add(OutputLine{Content: "test"})
	b.Add(OutputLine{Content: "test2"})

	if b.Count() != 2 {
		t.Fatalf("Count before Clear = %d, want 2", b.Count())
	}

	b.Clear()

	if b.Count() != 0 {
		t.Errorf("Count after Clear = %d, want 0", b.Count())
	}
}

func TestOutputLine_Fields(t *testing.T) {
	now := time.Now()
	line := OutputLine{
		Content:    "test content",
		Stream:     OutputStreamStderr,
		Timestamp:  now,
		LineNumber: 42,
	}

	if line.Content != "test content" {
		t.Errorf("Content = %q, want %q", line.Content, "test content")
	}
	if line.Stream != OutputStreamStderr {
		t.Errorf("Stream = %v, want stderr", line.Stream)
	}
	if line.Timestamp != now {
		t.Errorf("Timestamp mismatch")
	}
	if line.LineNumber != 42 {
		t.Errorf("LineNumber = %d, want 42", line.LineNumber)
	}
}
