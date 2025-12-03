package renderer

import (
	"testing"
)

func TestNewScreenPos(t *testing.T) {
	p := NewScreenPos(10, 20)
	if p.Row != 10 || p.Col != 20 {
		t.Errorf("expected (10, 20), got (%d, %d)", p.Row, p.Col)
	}
}

func TestScreenPosAdd(t *testing.T) {
	p := NewScreenPos(5, 10)
	p2 := p.Add(3, 7)
	if p2.Row != 8 || p2.Col != 17 {
		t.Errorf("expected (8, 17), got (%d, %d)", p2.Row, p2.Col)
	}

	// Original should be unchanged
	if p.Row != 5 || p.Col != 10 {
		t.Error("Add should not modify original")
	}
}

func TestScreenPosEquals(t *testing.T) {
	p1 := NewScreenPos(5, 10)
	p2 := NewScreenPos(5, 10)
	p3 := NewScreenPos(5, 11)

	if !p1.Equals(p2) {
		t.Error("identical positions should be equal")
	}
	if p1.Equals(p3) {
		t.Error("different positions should not be equal")
	}
}

func TestScreenPosBefore(t *testing.T) {
	tests := []struct {
		p1, p2 ScreenPos
		before bool
	}{
		{NewScreenPos(0, 0), NewScreenPos(0, 1), true},
		{NewScreenPos(0, 0), NewScreenPos(1, 0), true},
		{NewScreenPos(1, 5), NewScreenPos(2, 0), true},
		{NewScreenPos(1, 0), NewScreenPos(0, 5), false},
		{NewScreenPos(5, 5), NewScreenPos(5, 5), false},
	}

	for _, tt := range tests {
		got := tt.p1.Before(tt.p2)
		if got != tt.before {
			t.Errorf("(%d,%d).Before(%d,%d): expected %v, got %v",
				tt.p1.Row, tt.p1.Col, tt.p2.Row, tt.p2.Col, tt.before, got)
		}
	}
}

func TestNewScreenRect(t *testing.T) {
	r := NewScreenRect(1, 2, 10, 20)
	if r.Top != 1 || r.Left != 2 || r.Bottom != 10 || r.Right != 20 {
		t.Errorf("unexpected rect: %+v", r)
	}
}

func TestRectFromSize(t *testing.T) {
	r := RectFromSize(5, 10, 20, 30)
	if r.Top != 5 || r.Left != 10 || r.Bottom != 25 || r.Right != 40 {
		t.Errorf("unexpected rect: %+v", r)
	}
}

func TestScreenRectWidth(t *testing.T) {
	r := NewScreenRect(0, 5, 10, 25)
	if r.Width() != 20 {
		t.Errorf("expected width 20, got %d", r.Width())
	}

	// Empty rect
	r = NewScreenRect(0, 10, 10, 5)
	if r.Width() != 0 {
		t.Errorf("expected width 0 for invalid rect, got %d", r.Width())
	}
}

func TestScreenRectHeight(t *testing.T) {
	r := NewScreenRect(5, 0, 15, 10)
	if r.Height() != 10 {
		t.Errorf("expected height 10, got %d", r.Height())
	}

	// Empty rect
	r = NewScreenRect(10, 0, 5, 10)
	if r.Height() != 0 {
		t.Errorf("expected height 0 for invalid rect, got %d", r.Height())
	}
}

func TestScreenRectSize(t *testing.T) {
	r := NewScreenRect(0, 0, 25, 80)
	w, h := r.Size()
	if w != 80 || h != 25 {
		t.Errorf("expected (80, 25), got (%d, %d)", w, h)
	}
}

func TestScreenRectIsEmpty(t *testing.T) {
	empty1 := NewScreenRect(0, 0, 0, 10)
	empty2 := NewScreenRect(0, 0, 10, 0)
	empty3 := NewScreenRect(10, 10, 5, 5)
	nonEmpty := NewScreenRect(0, 0, 10, 10)

	if !empty1.IsEmpty() {
		t.Error("zero-height rect should be empty")
	}
	if !empty2.IsEmpty() {
		t.Error("zero-width rect should be empty")
	}
	if !empty3.IsEmpty() {
		t.Error("inverted rect should be empty")
	}
	if nonEmpty.IsEmpty() {
		t.Error("10x10 rect should not be empty")
	}
}

func TestScreenRectContains(t *testing.T) {
	r := NewScreenRect(5, 10, 15, 30)

	tests := []struct {
		pos      ScreenPos
		contains bool
	}{
		{NewScreenPos(5, 10), true},   // Top-left corner
		{NewScreenPos(14, 29), true},  // Bottom-right (inclusive)
		{NewScreenPos(10, 20), true},  // Middle
		{NewScreenPos(4, 10), false},  // Above
		{NewScreenPos(15, 10), false}, // Below (exclusive)
		{NewScreenPos(5, 9), false},   // Left
		{NewScreenPos(5, 30), false},  // Right (exclusive)
	}

	for _, tt := range tests {
		got := r.Contains(tt.pos)
		if got != tt.contains {
			t.Errorf("Contains(%d,%d): expected %v, got %v",
				tt.pos.Row, tt.pos.Col, tt.contains, got)
		}
	}
}

func TestScreenRectContainsRect(t *testing.T) {
	outer := NewScreenRect(0, 0, 100, 100)
	inner := NewScreenRect(10, 10, 50, 50)
	partial := NewScreenRect(50, 50, 150, 150)

	if !outer.ContainsRect(inner) {
		t.Error("outer should contain inner")
	}
	if outer.ContainsRect(partial) {
		t.Error("outer should not contain partial")
	}
	if !outer.ContainsRect(outer) {
		t.Error("rect should contain itself")
	}
}

func TestScreenRectIntersects(t *testing.T) {
	r1 := NewScreenRect(0, 0, 10, 10)
	r2 := NewScreenRect(5, 5, 15, 15)
	r3 := NewScreenRect(10, 10, 20, 20)
	r4 := NewScreenRect(20, 20, 30, 30)

	if !r1.Intersects(r2) {
		t.Error("r1 and r2 should intersect")
	}
	if r1.Intersects(r3) {
		t.Error("r1 and r3 should not intersect (adjacent)")
	}
	if r1.Intersects(r4) {
		t.Error("r1 and r4 should not intersect")
	}
}

func TestScreenRectIntersection(t *testing.T) {
	r1 := NewScreenRect(0, 0, 10, 10)
	r2 := NewScreenRect(5, 5, 15, 15)

	inter := r1.Intersection(r2)
	expected := NewScreenRect(5, 5, 10, 10)
	if !inter.Equals(expected) {
		t.Errorf("expected %+v, got %+v", expected, inter)
	}

	// Non-intersecting
	r3 := NewScreenRect(20, 20, 30, 30)
	empty := r1.Intersection(r3)
	if !empty.IsEmpty() {
		t.Error("intersection of non-overlapping rects should be empty")
	}
}

func TestScreenRectUnion(t *testing.T) {
	r1 := NewScreenRect(0, 0, 10, 10)
	r2 := NewScreenRect(5, 5, 15, 15)

	union := r1.Union(r2)
	expected := NewScreenRect(0, 0, 15, 15)
	if !union.Equals(expected) {
		t.Errorf("expected %+v, got %+v", expected, union)
	}

	// Union with empty rect
	empty := ScreenRect{}
	if !r1.Union(empty).Equals(r1) {
		t.Error("union with empty should return original")
	}
	if !empty.Union(r1).Equals(r1) {
		t.Error("empty union with rect should return rect")
	}
}

func TestScreenRectInset(t *testing.T) {
	r := NewScreenRect(0, 0, 100, 100)
	inset := r.Inset(10, 20, 30, 40)

	if inset.Top != 10 || inset.Left != 40 || inset.Bottom != 70 || inset.Right != 80 {
		t.Errorf("unexpected inset: %+v", inset)
	}
}

func TestScreenRectExpand(t *testing.T) {
	r := NewScreenRect(10, 10, 50, 50)
	expanded := r.Expand(5, 10, 15, 20)

	if expanded.Top != 5 || expanded.Left != -10 || expanded.Bottom != 65 || expanded.Right != 60 {
		t.Errorf("unexpected expansion: %+v", expanded)
	}
}

func TestScreenRectCorners(t *testing.T) {
	r := NewScreenRect(5, 10, 25, 50)

	tl := r.TopLeft()
	if tl.Row != 5 || tl.Col != 10 {
		t.Errorf("TopLeft: expected (5,10), got (%d,%d)", tl.Row, tl.Col)
	}

	tr := r.TopRight()
	if tr.Row != 5 || tr.Col != 50 {
		t.Errorf("TopRight: expected (5,50), got (%d,%d)", tr.Row, tr.Col)
	}

	bl := r.BottomLeft()
	if bl.Row != 25 || bl.Col != 10 {
		t.Errorf("BottomLeft: expected (25,10), got (%d,%d)", bl.Row, bl.Col)
	}

	br := r.BottomRight()
	if br.Row != 25 || br.Col != 50 {
		t.Errorf("BottomRight: expected (25,50), got (%d,%d)", br.Row, br.Col)
	}
}

func TestScreenRectClamp(t *testing.T) {
	r := NewScreenRect(10, 20, 30, 50)

	tests := []struct {
		input, expected ScreenPos
	}{
		{NewScreenPos(20, 30), NewScreenPos(20, 30)}, // Inside
		{NewScreenPos(5, 25), NewScreenPos(10, 25)},  // Above
		{NewScreenPos(35, 25), NewScreenPos(29, 25)}, // Below
		{NewScreenPos(15, 10), NewScreenPos(15, 20)}, // Left
		{NewScreenPos(15, 60), NewScreenPos(15, 49)}, // Right
		{NewScreenPos(0, 0), NewScreenPos(10, 20)},   // Top-left outside
		{NewScreenPos(50, 60), NewScreenPos(29, 49)}, // Bottom-right outside
	}

	for _, tt := range tests {
		got := r.Clamp(tt.input)
		if !got.Equals(tt.expected) {
			t.Errorf("Clamp(%d,%d): expected (%d,%d), got (%d,%d)",
				tt.input.Row, tt.input.Col,
				tt.expected.Row, tt.expected.Col,
				got.Row, got.Col)
		}
	}
}

func TestScreenRectEquals(t *testing.T) {
	r1 := NewScreenRect(1, 2, 3, 4)
	r2 := NewScreenRect(1, 2, 3, 4)
	r3 := NewScreenRect(1, 2, 3, 5)

	if !r1.Equals(r2) {
		t.Error("identical rects should be equal")
	}
	if r1.Equals(r3) {
		t.Error("different rects should not be equal")
	}
}
