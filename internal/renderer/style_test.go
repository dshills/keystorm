package renderer

import (
	"testing"
)

func TestAttributeHas(t *testing.T) {
	attrs := AttrBold | AttrItalic

	if !attrs.Has(AttrBold) {
		t.Error("should have AttrBold")
	}
	if !attrs.Has(AttrItalic) {
		t.Error("should have AttrItalic")
	}
	if attrs.Has(AttrUnderline) {
		t.Error("should not have AttrUnderline")
	}
}

func TestAttributeWith(t *testing.T) {
	attrs := AttrBold
	attrs = attrs.With(AttrItalic)

	if !attrs.Has(AttrBold) || !attrs.Has(AttrItalic) {
		t.Error("With should add attribute")
	}
}

func TestAttributeWithout(t *testing.T) {
	attrs := AttrBold | AttrItalic
	attrs = attrs.Without(AttrBold)

	if attrs.Has(AttrBold) {
		t.Error("Without should remove attribute")
	}
	if !attrs.Has(AttrItalic) {
		t.Error("Without should not affect other attributes")
	}
}

func TestDefaultStyle(t *testing.T) {
	s := DefaultStyle()

	if !s.Foreground.IsDefault() {
		t.Error("default style foreground should be default")
	}
	if !s.Background.IsDefault() {
		t.Error("default style background should be default")
	}
	if s.Attributes != AttrNone {
		t.Error("default style attributes should be none")
	}
}

func TestNewStyle(t *testing.T) {
	fg := ColorRed
	s := NewStyle(fg)

	if !s.Foreground.Equals(fg) {
		t.Error("NewStyle should set foreground")
	}
	if !s.Background.IsDefault() {
		t.Error("NewStyle should have default background")
	}
}

func TestStyleWithForeground(t *testing.T) {
	s := DefaultStyle().WithForeground(ColorRed)
	if !s.Foreground.Equals(ColorRed) {
		t.Error("WithForeground should set foreground")
	}
}

func TestStyleWithBackground(t *testing.T) {
	s := DefaultStyle().WithBackground(ColorBlue)
	if !s.Background.Equals(ColorBlue) {
		t.Error("WithBackground should set background")
	}
}

func TestStyleWithAttributes(t *testing.T) {
	s := DefaultStyle().WithAttributes(AttrBold | AttrItalic)
	if s.Attributes != (AttrBold | AttrItalic) {
		t.Error("WithAttributes should set attributes")
	}
}

func TestStyleBold(t *testing.T) {
	s := DefaultStyle().Bold()
	if !s.Attributes.Has(AttrBold) {
		t.Error("Bold() should add bold attribute")
	}
}

func TestStyleDim(t *testing.T) {
	s := DefaultStyle().Dim()
	if !s.Attributes.Has(AttrDim) {
		t.Error("Dim() should add dim attribute")
	}
}

func TestStyleItalic(t *testing.T) {
	s := DefaultStyle().Italic()
	if !s.Attributes.Has(AttrItalic) {
		t.Error("Italic() should add italic attribute")
	}
}

func TestStyleUnderline(t *testing.T) {
	s := DefaultStyle().Underline()
	if !s.Attributes.Has(AttrUnderline) {
		t.Error("Underline() should add underline attribute")
	}
}

func TestStyleReverse(t *testing.T) {
	s := DefaultStyle().Reverse()
	if !s.Attributes.Has(AttrReverse) {
		t.Error("Reverse() should add reverse attribute")
	}
}

func TestStyleStrikethrough(t *testing.T) {
	s := DefaultStyle().Strikethrough()
	if !s.Attributes.Has(AttrStrikethrough) {
		t.Error("Strikethrough() should add strikethrough attribute")
	}
}

func TestStyleMerge(t *testing.T) {
	base := Style{
		Foreground: ColorRed,
		Background: ColorDefault,
		Attributes: AttrBold,
	}

	overlay := Style{
		Foreground: ColorDefault,
		Background: ColorBlue,
		Attributes: AttrItalic,
	}

	merged := base.Merge(overlay)

	// Foreground: base (overlay is default)
	if !merged.Foreground.Equals(ColorRed) {
		t.Error("merge should keep base foreground when overlay is default")
	}

	// Background: overlay (non-default)
	if !merged.Background.Equals(ColorBlue) {
		t.Error("merge should use overlay background when non-default")
	}

	// Attributes: OR'd together
	if !merged.Attributes.Has(AttrBold) || !merged.Attributes.Has(AttrItalic) {
		t.Error("merge should OR attributes")
	}
}

func TestStyleEquals(t *testing.T) {
	s1 := Style{
		Foreground: ColorRed,
		Background: ColorBlue,
		Attributes: AttrBold,
	}
	s2 := Style{
		Foreground: ColorRed,
		Background: ColorBlue,
		Attributes: AttrBold,
	}
	s3 := Style{
		Foreground: ColorGreen,
		Background: ColorBlue,
		Attributes: AttrBold,
	}

	if !s1.Equals(s2) {
		t.Error("identical styles should be equal")
	}
	if s1.Equals(s3) {
		t.Error("different styles should not be equal")
	}
}

func TestStyleIsDefault(t *testing.T) {
	if !DefaultStyle().IsDefault() {
		t.Error("DefaultStyle should be default")
	}

	s := DefaultStyle().WithForeground(ColorRed)
	if s.IsDefault() {
		t.Error("style with non-default foreground should not be default")
	}
}

func TestStyleInvert(t *testing.T) {
	s := Style{
		Foreground: ColorRed,
		Background: ColorBlue,
		Attributes: AttrBold,
	}

	inverted := s.Invert()

	if !inverted.Foreground.Equals(ColorBlue) {
		t.Error("invert should swap foreground to background")
	}
	if !inverted.Background.Equals(ColorRed) {
		t.Error("invert should swap background to foreground")
	}
	if !inverted.Attributes.Has(AttrBold) {
		t.Error("invert should preserve attributes")
	}
}

func TestStyleSpanLen(t *testing.T) {
	span := StyleSpan{StartCol: 5, EndCol: 15}
	if span.Len() != 10 {
		t.Errorf("expected len 10, got %d", span.Len())
	}
}

func TestStyleSpanContains(t *testing.T) {
	span := StyleSpan{StartCol: 5, EndCol: 15}

	if span.Contains(4) {
		t.Error("should not contain 4")
	}
	if !span.Contains(5) {
		t.Error("should contain 5")
	}
	if !span.Contains(10) {
		t.Error("should contain 10")
	}
	if span.Contains(15) {
		t.Error("should not contain 15 (exclusive)")
	}
}

func TestStyleSpanOverlaps(t *testing.T) {
	span1 := StyleSpan{StartCol: 5, EndCol: 15}
	span2 := StyleSpan{StartCol: 10, EndCol: 20}
	span3 := StyleSpan{StartCol: 15, EndCol: 25}
	span4 := StyleSpan{StartCol: 0, EndCol: 5}

	if !span1.Overlaps(span2) {
		t.Error("span1 and span2 should overlap")
	}
	if span1.Overlaps(span3) {
		t.Error("span1 and span3 should not overlap (adjacent)")
	}
	if span1.Overlaps(span4) {
		t.Error("span1 and span4 should not overlap (adjacent)")
	}
}

func TestStyleSpanIntersection(t *testing.T) {
	span1 := StyleSpan{StartCol: 5, EndCol: 15, Style: DefaultStyle().WithForeground(ColorRed)}
	span2 := StyleSpan{StartCol: 10, EndCol: 20, Style: DefaultStyle().WithBackground(ColorBlue)}

	intersection := span1.Intersection(span2)

	if intersection.StartCol != 10 || intersection.EndCol != 15 {
		t.Errorf("expected intersection [10,15), got [%d,%d)", intersection.StartCol, intersection.EndCol)
	}

	// Non-overlapping spans
	span3 := StyleSpan{StartCol: 20, EndCol: 30}
	empty := span1.Intersection(span3)
	if empty.Len() != 0 {
		t.Error("non-overlapping spans should have empty intersection")
	}
}
