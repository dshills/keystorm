package engine

import (
	"strings"
	"testing"

	"github.com/dshills/keystorm/internal/engine/history"
	"github.com/dshills/keystorm/internal/engine/tracking"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupLargeEngine(b *testing.B, lines int) *Engine {
	b.Helper()
	var sb strings.Builder
	line := strings.Repeat("x", 80) + "\n"
	for i := 0; i < lines; i++ {
		sb.WriteString(line)
	}
	return New(WithContent(sb.String()))
}

// ============================================================================
// Read Operation Benchmarks
// ============================================================================

func BenchmarkEngineText(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.Text()
	}
}

func BenchmarkEngineTextRange(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.TextRange(1000, 2000)
	}
}

func BenchmarkEngineLen(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.Len()
	}
}

func BenchmarkEngineLineCount(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.LineCount()
	}
}

func BenchmarkEngineLineText(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.LineText(5000)
	}
}

// ============================================================================
// Position Conversion Benchmarks
// ============================================================================

func BenchmarkEngineOffsetToPoint(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	mid := e.Len() / 2
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.OffsetToPoint(mid)
	}
}

func BenchmarkEnginePointToOffset(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	point := Point{Line: 5000, Column: 40}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.PointToOffset(point)
	}
}

// ============================================================================
// Write Operation Benchmarks
// ============================================================================

func BenchmarkEngineInsert(b *testing.B) {
	for i := 0; i < b.N; i++ {
		e := New()
		for j := 0; j < 1000; j++ {
			e.Insert(ByteOffset(j), "x")
		}
	}
}

func BenchmarkEngineInsertMiddle(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	mid := e.Len() / 2
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		e.Insert(mid, "x")
	}
}

func BenchmarkEngineDelete(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		e := New(WithContent(strings.Repeat("x", 10000)))
		b.StartTimer()

		for j := 0; j < 100; j++ {
			e.Delete(0, 10)
		}
	}
}

func BenchmarkEngineReplace(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		e := New(WithContent(strings.Repeat("x", 10000)))
		b.StartTimer()

		for j := 0; j < 100; j++ {
			e.Replace(500, 510, "yyyyyyyyyy")
		}
	}
}

func BenchmarkEngineApplyEdit(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	mid := e.Len() / 2
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		e.ApplyEdit(Edit{
			Range:   Range{Start: mid, End: mid + 10},
			NewText: "replacement",
		})
	}
}

func BenchmarkEngineApplyEdits(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		e := New(WithContent(strings.Repeat("x", 10000)))
		edits := []Edit{
			{Range: Range{Start: 9000, End: 9010}, NewText: "aaaa"},
			{Range: Range{Start: 5000, End: 5010}, NewText: "bbbb"},
			{Range: Range{Start: 1000, End: 1010}, NewText: "cccc"},
		}
		b.StartTimer()

		e.ApplyEdits(edits)
	}
}

// ============================================================================
// Undo/Redo Benchmarks
// ============================================================================

func BenchmarkEngineUndo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		e := New()
		for j := 0; j < 100; j++ {
			e.Insert(ByteOffset(j), "x")
		}
		b.StartTimer()

		for j := 0; j < 100; j++ {
			e.Undo()
		}
	}
}

func BenchmarkEngineRedo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		e := New()
		for j := 0; j < 100; j++ {
			e.Insert(ByteOffset(j), "x")
		}
		for j := 0; j < 100; j++ {
			e.Undo()
		}
		b.StartTimer()

		for j := 0; j < 100; j++ {
			e.Redo()
		}
	}
}

func BenchmarkEngineUndoGroup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		e := New()
		for j := 0; j < 10; j++ {
			e.BeginUndoGroup("group")
			for k := 0; k < 10; k++ {
				e.Insert(ByteOffset(j*10+k), "x")
			}
			e.EndUndoGroup()
		}
		b.StartTimer()

		for j := 0; j < 10; j++ {
			e.Undo()
		}
	}
}

// ============================================================================
// Command Execution Benchmarks
// ============================================================================

func BenchmarkEngineExecuteInsert(b *testing.B) {
	e := New()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		e.SetPrimaryCursor(ByteOffset(i))
		cmd := history.NewInsertCommand("x")
		e.Execute(cmd)
	}
}

// ============================================================================
// Cursor Benchmarks
// ============================================================================

func BenchmarkEngineAddCursor(b *testing.B) {
	e := New(WithContent(strings.Repeat("x", 10000)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		e.ClearSecondary()
		b.StartTimer()

		for j := 0; j < 100; j++ {
			e.AddCursor(ByteOffset(j * 100))
		}
	}
}

func BenchmarkEngineCursorsClone(b *testing.B) {
	e := New(WithContent(strings.Repeat("x", 10000)))
	for i := 0; i < 100; i++ {
		e.AddCursor(ByteOffset(i * 100))
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.Cursors()
	}
}

// ============================================================================
// Snapshot Benchmarks
// ============================================================================

func BenchmarkEngineCreateSnapshot(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		e.CreateSnapshot("snap")
	}
}

func BenchmarkEngineGetSnapshot(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	id := e.CreateSnapshot("snap")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		e.GetSnapshot(id)
	}
}

func BenchmarkEngineSnapshot(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.Snapshot()
	}
}

// ============================================================================
// Change Tracking Benchmarks
// ============================================================================

func BenchmarkEngineChangesSince(b *testing.B) {
	e := New()
	initialRev := e.RevisionID()

	for i := 0; i < 1000; i++ {
		e.Insert(ByteOffset(i), "x")
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.ChangesSince(initialRev)
	}
}

func BenchmarkEngineLatestChanges(b *testing.B) {
	e := New()

	for i := 0; i < 1000; i++ {
		e.Insert(ByteOffset(i), "x")
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = e.LatestChanges(100)
	}
}

// ============================================================================
// Diff Benchmarks
// ============================================================================

func BenchmarkEngineDiffSinceSnapshot(b *testing.B) {
	e := New(WithContent(strings.Repeat("line\n", 1000)))
	snapID := e.CreateSnapshot("before")

	// Make some changes
	for i := 0; i < 10; i++ {
		e.Replace(ByteOffset(i*50), ByteOffset(i*50+4), "modified")
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		e.DiffSinceSnapshot(snapID)
	}
}

func BenchmarkEngineComputeDiffSinceSnapshot(b *testing.B) {
	e := New(WithContent(strings.Repeat("line\n", 1000)))
	snapID := e.CreateSnapshot("before")

	// Make some changes
	for i := 0; i < 10; i++ {
		e.Replace(ByteOffset(i*50), ByteOffset(i*50+4), "modified")
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		e.ComputeDiffSinceSnapshot(snapID, DiffOptions{ContextLines: 3})
	}
}

// ============================================================================
// AI Context Benchmarks
// ============================================================================

func BenchmarkEngineGetAIContext(b *testing.B) {
	e := New(WithContent(strings.Repeat("line\n", 1000)))
	e.CreateSnapshot("before")
	initialRev := e.RevisionID()

	// Make some changes
	for i := 0; i < 100; i++ {
		e.Insert(ByteOffset(i), "x")
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		e.GetAIContext(tracking.AIContextOptions{
			SinceRevision:    initialRev,
			MaxChanges:       100,
			IncludeDiff:      true,
			DiffFromSnapshot: "before",
		})
	}
}

// ============================================================================
// Combined Workflow Benchmarks
// ============================================================================

func BenchmarkEngineTypicalEditWorkflow(b *testing.B) {
	// Simulates typical editing: insert, navigate, delete, undo
	for i := 0; i < b.N; i++ {
		e := New()

		// Type a line
		for j := 0; j < 80; j++ {
			e.Insert(ByteOffset(j), "x")
		}
		e.Insert(80, "\n")

		// Navigate and edit
		point := e.OffsetToPoint(40)
		offset := e.PointToOffset(point)
		e.Delete(offset, offset+5)

		// Undo
		e.Undo()
	}
}

func BenchmarkEngineAIContextWorkflow(b *testing.B) {
	// Simulates AI context generation workflow
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		e := New(WithContent(strings.Repeat("original line\n", 100)))
		snapID := e.CreateSnapshot("before_ai")
		initialRev := e.RevisionID()

		// Simulate AI making changes
		for j := 0; j < 10; j++ {
			e.Replace(ByteOffset(j*14), ByteOffset(j*14+8), "modified")
		}
		b.StartTimer()

		// Generate context for AI response
		_ = e.GetAIContext(tracking.AIContextOptions{
			SinceRevision: initialRev,
			MaxChanges:    100,
		})
		_, _ = e.DiffSinceSnapshot(snapID)
	}
}

// ============================================================================
// Memory Benchmarks
// ============================================================================

func BenchmarkEngineMemorySnapshots(b *testing.B) {
	e := setupLargeEngine(b, 10000)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create many snapshots - should share structure
		for j := 0; j < 100; j++ {
			e.CreateSnapshot("snap")
		}
	}
}

func BenchmarkEngineMemoryEdits(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e := New()
		for j := 0; j < 1000; j++ {
			e.Insert(ByteOffset(j), "x")
		}
	}
}
