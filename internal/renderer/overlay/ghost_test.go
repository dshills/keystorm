package overlay

import (
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/renderer/core"
)

func TestNewGhostText(t *testing.T) {
	style := core.NewStyle(core.ColorFromRGB(128, 128, 128))
	pos := Position{Line: 5, Col: 10}

	gt := NewGhostText("ghost-1", pos, "hello world", style)

	if gt.ID() != "ghost-1" {
		t.Errorf("ID() = %q, want %q", gt.ID(), "ghost-1")
	}
	if gt.Type() != TypeGhostText {
		t.Errorf("Type() = %v, want %v", gt.Type(), TypeGhostText)
	}
	if gt.Priority() != PriorityNormal {
		t.Errorf("Priority() = %v, want %v", gt.Priority(), PriorityNormal)
	}
	if gt.Text() != "hello world" {
		t.Errorf("Text() = %q, want %q", gt.Text(), "hello world")
	}
	if gt.LineCount() != 1 {
		t.Errorf("LineCount() = %d, want 1", gt.LineCount())
	}
}

func TestNewGhostTextMultiLine(t *testing.T) {
	style := core.NewStyle(core.ColorFromRGB(128, 128, 128))
	pos := Position{Line: 5, Col: 0}
	lines := []string{"line one", "line two", "line three"}

	gt := NewGhostTextMultiLine("ghost-ml", pos, lines, style)

	if gt.LineCount() != 3 {
		t.Errorf("LineCount() = %d, want 3", gt.LineCount())
	}
	if gt.Text() != "line one\nline two\nline three" {
		t.Errorf("Text() = %q, want multi-line text", gt.Text())
	}

	rng := gt.Range()
	if rng.Start.Line != 5 {
		t.Errorf("Start.Line = %d, want 5", rng.Start.Line)
	}
	if rng.End.Line != 7 {
		t.Errorf("End.Line = %d, want 7", rng.End.Line)
	}
}

func TestNewGhostTextMultiLineEmpty(t *testing.T) {
	style := core.NewStyle(core.ColorFromRGB(128, 128, 128))
	pos := Position{Line: 0, Col: 0}

	gt := NewGhostTextMultiLine("ghost-empty", pos, nil, style)

	if gt.LineCount() != 1 {
		t.Errorf("LineCount() = %d, want 1 (empty default)", gt.LineCount())
	}
}

func TestGhostTextShowHide(t *testing.T) {
	gt := NewGhostText("ghost", Position{}, "test", core.DefaultStyle())

	if !gt.IsVisible() {
		t.Error("Ghost text should be visible by default")
	}

	gt.Hide()
	if gt.IsVisible() {
		t.Error("Ghost text should be hidden after Hide()")
	}

	gt.Show()
	if !gt.IsVisible() {
		t.Error("Ghost text should be visible after Show()")
	}
}

func TestGhostTextAccept(t *testing.T) {
	gt := NewGhostText("ghost", Position{}, "test", core.DefaultStyle())

	gt.Accept()

	if !gt.IsAccepted() {
		t.Error("IsAccepted() should be true after Accept()")
	}
	if gt.IsVisible() {
		t.Error("Ghost text should be hidden after Accept()")
	}
}

func TestGhostTextReject(t *testing.T) {
	gt := NewGhostText("ghost", Position{}, "test", core.DefaultStyle())

	gt.Reject()

	if gt.IsVisible() {
		t.Error("Ghost text should be hidden after Reject()")
	}
}

func TestGhostTextAcceptPartial(t *testing.T) {
	gt := NewGhostText("ghost", Position{Line: 0, Col: 5}, "hello world", core.DefaultStyle())

	// Accept first word
	accepted := gt.AcceptPartial()
	if accepted != "hello" {
		t.Errorf("First partial = %q, want %q", accepted, "hello")
	}

	// Accept space
	accepted = gt.AcceptPartial()
	if accepted != " " {
		t.Errorf("Second partial = %q, want %q", accepted, " ")
	}

	// Accept second word
	accepted = gt.AcceptPartial()
	if accepted != "world" {
		t.Errorf("Third partial = %q, want %q", accepted, "world")
	}

	// Need one more call to finalize (partial accepted now equals length)
	accepted = gt.AcceptPartial()
	if accepted != "" {
		t.Errorf("Final partial = %q, want empty", accepted)
	}

	// Should be fully accepted now
	if !gt.IsAccepted() {
		t.Error("Should be fully accepted after all words")
	}
}

func TestGhostTextAcceptPartialMultiLine(t *testing.T) {
	lines := []string{"first", "second"}
	gt := NewGhostTextMultiLine("ghost", Position{Line: 0, Col: 0}, lines, core.DefaultStyle())

	// Accept first word
	gt.AcceptPartial()

	// Accept remaining of first line (returns newline)
	accepted := gt.AcceptPartial()
	if accepted != "\n" {
		t.Errorf("Expected newline, got %q", accepted)
	}

	// Should now be on second line
	if gt.LineCount() != 1 {
		t.Errorf("LineCount() = %d, want 1 after accepting first line", gt.LineCount())
	}
}

func TestGhostTextOpacity(t *testing.T) {
	gt := NewGhostText("ghost", Position{}, "test", core.DefaultStyle())
	gt.SetFadeInDuration(100 * time.Millisecond)
	gt.SetAnimationEnabled(true)

	gt.Show()

	// Immediately after show, opacity should be low
	opacity := gt.Opacity()
	if opacity >= 1.0 {
		t.Errorf("Opacity immediately after show should be < 1.0, got %f", opacity)
	}

	// Wait for animation to complete
	time.Sleep(150 * time.Millisecond)

	opacity = gt.Opacity()
	if opacity != 1.0 {
		t.Errorf("Opacity after animation should be 1.0, got %f", opacity)
	}
}

func TestGhostTextOpacityDisabled(t *testing.T) {
	gt := NewGhostText("ghost", Position{}, "test", core.DefaultStyle())
	gt.SetAnimationEnabled(false)

	gt.Show()

	opacity := gt.Opacity()
	if opacity != 1.0 {
		t.Errorf("Opacity with animation disabled should be 1.0, got %f", opacity)
	}
}

func TestGhostTextUpdatePosition(t *testing.T) {
	gt := NewGhostText("ghost", Position{Line: 5, Col: 10}, "test", core.DefaultStyle())

	newPos := Position{Line: 10, Col: 20}
	gt.UpdatePosition(newPos)

	rng := gt.Range()
	if rng.Start != newPos {
		t.Errorf("Start = %v, want %v", rng.Start, newPos)
	}
}

func TestGhostTextUpdateText(t *testing.T) {
	gt := NewGhostText("ghost", Position{Line: 5, Col: 0}, "old", core.DefaultStyle())

	gt.UpdateText("new text\nwith lines")

	if gt.Text() != "new text\nwith lines" {
		t.Errorf("Text() = %q, want %q", gt.Text(), "new text\nwith lines")
	}
	if gt.LineCount() != 2 {
		t.Errorf("LineCount() = %d, want 2", gt.LineCount())
	}
}

func TestGhostTextSpansForLine(t *testing.T) {
	gt := NewGhostText("ghost", Position{Line: 5, Col: 10}, "completion", core.DefaultStyle())

	t.Run("correct line", func(t *testing.T) {
		spans := gt.SpansForLine(5)
		if len(spans) != 1 {
			t.Fatalf("len(spans) = %d, want 1", len(spans))
		}

		span := spans[0]
		if span.StartCol != 10 {
			t.Errorf("StartCol = %d, want 10", span.StartCol)
		}
		if span.Text != "completion" {
			t.Errorf("Text = %q, want %q", span.Text, "completion")
		}
		if !span.AfterContent {
			t.Error("AfterContent should be true for first line")
		}
	})

	t.Run("wrong line", func(t *testing.T) {
		spans := gt.SpansForLine(6)
		if len(spans) != 0 {
			t.Errorf("len(spans) = %d, want 0 for wrong line", len(spans))
		}
	})

	t.Run("hidden ghost text", func(t *testing.T) {
		gt.Hide()
		spans := gt.SpansForLine(5)
		if len(spans) != 0 {
			t.Errorf("len(spans) = %d, want 0 for hidden ghost text", len(spans))
		}
	})
}

func TestGhostTextSpansForLineMultiLine(t *testing.T) {
	lines := []string{"line one", "line two", "line three"}
	gt := NewGhostTextMultiLine("ghost", Position{Line: 5, Col: 10}, lines, core.DefaultStyle())

	t.Run("first line", func(t *testing.T) {
		spans := gt.SpansForLine(5)
		if len(spans) != 1 {
			t.Fatalf("len(spans) = %d, want 1", len(spans))
		}
		if spans[0].Text != "line one" {
			t.Errorf("Text = %q, want %q", spans[0].Text, "line one")
		}
		if !spans[0].AfterContent {
			t.Error("First line should have AfterContent=true")
		}
	})

	t.Run("subsequent lines", func(t *testing.T) {
		spans := gt.SpansForLine(6)
		if len(spans) != 1 {
			t.Fatalf("len(spans) = %d, want 1", len(spans))
		}
		if spans[0].Text != "line two" {
			t.Errorf("Text = %q, want %q", spans[0].Text, "line two")
		}
		if spans[0].StartCol != 0 {
			t.Errorf("StartCol = %d, want 0 for subsequent lines", spans[0].StartCol)
		}
		if spans[0].AfterContent {
			t.Error("Subsequent lines should have AfterContent=false")
		}
	})
}

func TestFindWordEnd(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 5},
		{"hello world", 5},
		{" hello", 1},  // Returns whitespace as a "word"
		{"  hello", 2}, // Multiple spaces
		{"\thello", 1}, // Tab
		{"hello\tworld", 5},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := findWordEnd(tt.input)
			if got != tt.want {
				t.Errorf("findWordEnd(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
