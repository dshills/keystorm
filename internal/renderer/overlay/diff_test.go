package overlay

import (
	"testing"
)

func TestDiffOperationString(t *testing.T) {
	tests := []struct {
		op   DiffOperation
		want string
	}{
		{DiffOpEqual, "equal"},
		{DiffOpInsert, "insert"},
		{DiffOpDelete, "delete"},
		{DiffOpReplace, "replace"},
		{DiffOperation(255), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.op.String(); got != tt.want {
				t.Errorf("DiffOperation.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewDiffPreview(t *testing.T) {
	config := DefaultConfig()
	hunks := []DiffHunk{
		{
			Operation: DiffOpInsert,
			OldRange:  Range{Start: Position{Line: 5, Col: 0}, End: Position{Line: 5, Col: 0}},
			NewRange:  Range{Start: Position{Line: 5, Col: 0}, End: Position{Line: 7, Col: 0}},
			NewLines:  []string{"new line 1", "new line 2"},
		},
	}

	dp := NewDiffPreview("diff-1", hunks, config)

	if dp.ID() != "diff-1" {
		t.Errorf("ID() = %q, want %q", dp.ID(), "diff-1")
	}
	if dp.Type() != TypeDiffAdd {
		t.Errorf("Type() = %v, want %v", dp.Type(), TypeDiffAdd)
	}
	if dp.HunkCount() != 1 {
		t.Errorf("HunkCount() = %d, want 1", dp.HunkCount())
	}
}

func TestNewDiffPreviewSimple(t *testing.T) {
	config := DefaultConfig()
	oldLines := []string{"old line 1", "old line 2"}
	newLines := []string{"new line 1", "new line 2", "new line 3"}

	dp := NewDiffPreviewSimple("diff-simple", 5, oldLines, newLines, config)

	if dp.HunkCount() == 0 {
		t.Error("Should have at least one hunk")
	}
}

func TestDiffPreviewCounts(t *testing.T) {
	config := DefaultConfig()
	hunks := []DiffHunk{
		{
			Operation: DiffOpInsert,
			NewLines:  []string{"new 1", "new 2"},
		},
		{
			Operation: DiffOpDelete,
			OldLines:  []string{"old 1"},
		},
		{
			Operation: DiffOpReplace,
			OldLines:  []string{"old 2", "old 3"},
			NewLines:  []string{"new 3"},
		},
	}

	dp := NewDiffPreview("diff", hunks, config)

	// Additions: 2 (insert) + 1 (replace) = 3
	if got := dp.AdditionCount(); got != 3 {
		t.Errorf("AdditionCount() = %d, want 3", got)
	}

	// Deletions: 1 (delete) + 2 (replace) = 3
	if got := dp.DeletionCount(); got != 3 {
		t.Errorf("DeletionCount() = %d, want 3", got)
	}
}

func TestDiffPreviewCollapsed(t *testing.T) {
	dp := NewDiffPreview("diff", nil, DefaultConfig())

	if dp.IsCollapsed() {
		t.Error("Should not be collapsed by default")
	}

	dp.SetCollapsed(true)
	if !dp.IsCollapsed() {
		t.Error("Should be collapsed after SetCollapsed(true)")
	}
}

func TestDiffPreviewAcceptReject(t *testing.T) {
	t.Run("accept", func(t *testing.T) {
		dp := NewDiffPreview("diff", nil, DefaultConfig())
		dp.Accept()

		if !dp.IsAccepted() {
			t.Error("IsAccepted() should be true after Accept()")
		}
		if dp.IsVisible() {
			t.Error("Should not be visible after Accept()")
		}
	})

	t.Run("reject", func(t *testing.T) {
		dp := NewDiffPreview("diff", nil, DefaultConfig())
		dp.Reject()

		if !dp.IsRejected() {
			t.Error("IsRejected() should be true after Reject()")
		}
		if dp.IsVisible() {
			t.Error("Should not be visible after Reject()")
		}
	})
}

func TestDiffPreviewAcceptHunk(t *testing.T) {
	config := DefaultConfig()
	hunks := []DiffHunk{
		{Operation: DiffOpInsert, NewLines: []string{"line 1"}},
		{Operation: DiffOpInsert, NewLines: []string{"line 2"}},
		{Operation: DiffOpInsert, NewLines: []string{"line 3"}},
	}

	dp := NewDiffPreview("diff", hunks, config)

	// Accept first hunk
	if !dp.AcceptHunk(0) {
		t.Error("AcceptHunk(0) should succeed")
	}
	if dp.HunkCount() != 2 {
		t.Errorf("HunkCount() = %d, want 2 after accepting one hunk", dp.HunkCount())
	}

	// Invalid index
	if dp.AcceptHunk(10) {
		t.Error("AcceptHunk(10) should fail for invalid index")
	}

	// Accept remaining hunks
	dp.AcceptHunk(0)
	dp.AcceptHunk(0)

	if !dp.IsAccepted() {
		t.Error("Should be accepted when all hunks are accepted")
	}
}

func TestDiffPreviewRejectHunk(t *testing.T) {
	config := DefaultConfig()
	hunks := []DiffHunk{
		{Operation: DiffOpInsert, NewLines: []string{"line 1"}},
		{Operation: DiffOpInsert, NewLines: []string{"line 2"}},
	}

	dp := NewDiffPreview("diff", hunks, config)

	dp.RejectHunk(0)
	if dp.HunkCount() != 1 {
		t.Errorf("HunkCount() = %d, want 1", dp.HunkCount())
	}

	dp.RejectHunk(0)
	if !dp.IsRejected() {
		t.Error("Should be rejected when all hunks are rejected")
	}
}

func TestDiffPreviewSpansForLine(t *testing.T) {
	config := DefaultConfig()

	t.Run("insert", func(t *testing.T) {
		hunks := []DiffHunk{
			{
				Operation: DiffOpInsert,
				OldRange:  Range{Start: Position{Line: 5, Col: 0}, End: Position{Line: 5, Col: 0}},
				NewLines:  []string{"new line"},
			},
		}
		dp := NewDiffPreview("diff", hunks, config)

		spans := dp.SpansForLine(5)
		if len(spans) == 0 {
			t.Error("Should have spans for insertion line")
		}
	})

	t.Run("delete", func(t *testing.T) {
		hunks := []DiffHunk{
			{
				Operation: DiffOpDelete,
				OldRange:  Range{Start: Position{Line: 5, Col: 0}, End: Position{Line: 6, Col: 0}},
				OldLines:  []string{"deleted line"},
			},
		}
		dp := NewDiffPreview("diff", hunks, config)

		spans := dp.SpansForLine(5)
		if len(spans) == 0 {
			t.Error("Should have spans for deletion line")
		}
		if !spans[0].ReplaceContent {
			t.Error("Delete span should have ReplaceContent=true")
		}
	})

	t.Run("hidden", func(t *testing.T) {
		dp := NewDiffPreview("diff", nil, config)
		dp.SetVisible(false)

		spans := dp.SpansForLine(5)
		if len(spans) != 0 {
			t.Error("Should have no spans when hidden")
		}
	})

	t.Run("accepted", func(t *testing.T) {
		dp := NewDiffPreview("diff", nil, config)
		dp.Accept()

		spans := dp.SpansForLine(5)
		if len(spans) != 0 {
			t.Error("Should have no spans when accepted")
		}
	})
}

func TestDiffPreviewCollapsedSpans(t *testing.T) {
	config := DefaultConfig()
	hunks := []DiffHunk{
		{
			Operation: DiffOpInsert,
			OldRange:  Range{Start: Position{Line: 5, Col: 0}, End: Position{Line: 5, Col: 0}},
			NewLines:  []string{"line 1", "line 2"},
		},
	}

	dp := NewDiffPreview("diff", hunks, config)
	dp.SetCollapsed(true)

	spans := dp.SpansForLine(5)
	if len(spans) != 1 {
		t.Fatalf("len(spans) = %d, want 1 for collapsed summary", len(spans))
	}

	// Summary should contain change counts
	if spans[0].Text == "" {
		t.Error("Collapsed summary should have text")
	}
}

func TestComputeSimpleDiff(t *testing.T) {
	t.Run("empty both", func(t *testing.T) {
		hunks := computeSimpleDiff(0, nil, nil)
		if len(hunks) != 0 {
			t.Errorf("len(hunks) = %d, want 0", len(hunks))
		}
	})

	t.Run("all insertions", func(t *testing.T) {
		hunks := computeSimpleDiff(5, nil, []string{"a", "b"})
		if len(hunks) != 1 {
			t.Fatalf("len(hunks) = %d, want 1", len(hunks))
		}
		if hunks[0].Operation != DiffOpInsert {
			t.Errorf("Operation = %v, want DiffOpInsert", hunks[0].Operation)
		}
	})

	t.Run("all deletions", func(t *testing.T) {
		hunks := computeSimpleDiff(5, []string{"a", "b"}, nil)
		if len(hunks) != 1 {
			t.Fatalf("len(hunks) = %d, want 1", len(hunks))
		}
		if hunks[0].Operation != DiffOpDelete {
			t.Errorf("Operation = %v, want DiffOpDelete", hunks[0].Operation)
		}
	})

	t.Run("replacement", func(t *testing.T) {
		hunks := computeSimpleDiff(5, []string{"old"}, []string{"new"})
		if len(hunks) != 1 {
			t.Fatalf("len(hunks) = %d, want 1", len(hunks))
		}
		if hunks[0].Operation != DiffOpReplace {
			t.Errorf("Operation = %v, want DiffOpReplace", hunks[0].Operation)
		}
	})

	t.Run("equal lines", func(t *testing.T) {
		hunks := computeSimpleDiff(5, []string{"same"}, []string{"same"})
		if len(hunks) != 0 {
			t.Errorf("len(hunks) = %d, want 0 for equal lines", len(hunks))
		}
	})

	t.Run("mixed changes", func(t *testing.T) {
		old := []string{"a", "b", "c"}
		new := []string{"a", "x", "c"}
		hunks := computeSimpleDiff(0, old, new)

		// Should detect change in middle
		if len(hunks) == 0 {
			t.Error("Should have hunks for mixed changes")
		}
	})
}

func TestCalculateDiffRange(t *testing.T) {
	t.Run("empty hunks", func(t *testing.T) {
		rng := calculateDiffRange(nil)
		if !rng.IsEmpty() {
			t.Error("Empty hunks should return empty range")
		}
	})

	t.Run("single hunk", func(t *testing.T) {
		hunks := []DiffHunk{
			{OldRange: Range{Start: Position{Line: 5, Col: 0}, End: Position{Line: 10, Col: 0}}},
		}
		rng := calculateDiffRange(hunks)
		if rng.Start.Line != 5 || rng.End.Line != 10 {
			t.Errorf("Range = %v, want lines 5-10", rng)
		}
	})

	t.Run("multiple hunks", func(t *testing.T) {
		hunks := []DiffHunk{
			{OldRange: Range{Start: Position{Line: 10, Col: 0}, End: Position{Line: 15, Col: 0}}},
			{OldRange: Range{Start: Position{Line: 5, Col: 0}, End: Position{Line: 8, Col: 0}}},
			{OldRange: Range{Start: Position{Line: 20, Col: 0}, End: Position{Line: 25, Col: 0}}},
		}
		rng := calculateDiffRange(hunks)
		if rng.Start.Line != 5 {
			t.Errorf("Start.Line = %d, want 5", rng.Start.Line)
		}
		if rng.End.Line != 25 {
			t.Errorf("End.Line = %d, want 25", rng.End.Line)
		}
	})
}

func TestFormatDiffSummary(t *testing.T) {
	tests := []struct {
		adds int
		dels int
		want string
	}{
		{0, 0, "[no changes]"},
		{5, 0, "[+5 lines]"},
		{0, 3, "[-3 lines]"},
		{5, 3, "[+5 -3 lines]"},
	}

	for _, tt := range tests {
		got := formatDiffSummary(tt.adds, tt.dels)
		if got != tt.want {
			t.Errorf("formatDiffSummary(%d, %d) = %q, want %q", tt.adds, tt.dels, got, tt.want)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{12345, "12345"},
		{-1, "-1"},
		{-42, "-42"},
	}

	for _, tt := range tests {
		got := itoa(tt.n)
		if got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestNewInlineDiff(t *testing.T) {
	config := DefaultConfig()
	segments := []InlineDiffSegment{
		{Operation: DiffOpDelete, StartCol: 5, EndCol: 10},
		{Operation: DiffOpInsert, StartCol: 10, EndCol: 15, Text: "hello"},
	}

	id := NewInlineDiff("inline-1", 5, segments, config)

	if id.ID() != "inline-1" {
		t.Errorf("ID() = %q, want %q", id.ID(), "inline-1")
	}
	if id.Type() != TypeDiffModify {
		t.Errorf("Type() = %v, want %v", id.Type(), TypeDiffModify)
	}
}

func TestInlineDiffSpansForLine(t *testing.T) {
	config := DefaultConfig()
	segments := []InlineDiffSegment{
		{Operation: DiffOpDelete, StartCol: 5, EndCol: 10},
		{Operation: DiffOpInsert, StartCol: 10, EndCol: 15, Text: "hello"},
		{Operation: DiffOpEqual, StartCol: 15, EndCol: 20}, // Should be skipped
	}

	id := NewInlineDiff("inline", 5, segments, config)

	t.Run("correct line", func(t *testing.T) {
		spans := id.SpansForLine(5)
		if len(spans) != 2 {
			t.Fatalf("len(spans) = %d, want 2 (excluding equal)", len(spans))
		}

		// First span should be delete
		if spans[0].ReplaceContent != true {
			t.Error("Delete span should have ReplaceContent=true")
		}

		// Second span should be insert
		if spans[1].AfterContent != true {
			t.Error("Insert span should have AfterContent=true")
		}
	})

	t.Run("wrong line", func(t *testing.T) {
		spans := id.SpansForLine(6)
		if len(spans) != 0 {
			t.Errorf("len(spans) = %d, want 0 for wrong line", len(spans))
		}
	})

	t.Run("hidden", func(t *testing.T) {
		id.SetVisible(false)
		spans := id.SpansForLine(5)
		if len(spans) != 0 {
			t.Errorf("len(spans) = %d, want 0 for hidden", len(spans))
		}
	})
}
