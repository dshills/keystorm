package renderer

import (
	"testing"
)

func TestColorFromRGB(t *testing.T) {
	c := ColorFromRGB(255, 128, 64)
	if c.R != 255 || c.G != 128 || c.B != 64 {
		t.Errorf("expected RGB(255,128,64), got RGB(%d,%d,%d)", c.R, c.G, c.B)
	}
	if c.Indexed {
		t.Error("expected non-indexed color")
	}
}

func TestColorFromIndex(t *testing.T) {
	c := ColorFromIndex(42)
	if c.R != 42 {
		t.Errorf("expected index 42, got %d", c.R)
	}
	if !c.Indexed {
		t.Error("expected indexed color")
	}
}

func TestColorFromHex(t *testing.T) {
	tests := []struct {
		hex     string
		r, g, b uint8
		wantErr bool
	}{
		{"#FF0000", 255, 0, 0, false},
		{"FF0000", 255, 0, 0, false},
		{"#00FF00", 0, 255, 0, false},
		{"#0000FF", 0, 0, 255, false},
		{"#ABC", 170, 187, 204, false},
		{"ABC", 170, 187, 204, false},
		{"#FFFFFF", 255, 255, 255, false},
		{"#000000", 0, 0, 0, false},
		{"invalid", 0, 0, 0, true},
		{"#GG0000", 0, 0, 0, true},
		{"#12345", 0, 0, 0, true},
	}

	for _, tt := range tests {
		c, err := ColorFromHex(tt.hex)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ColorFromHex(%q): expected error", tt.hex)
			}
			continue
		}
		if err != nil {
			t.Errorf("ColorFromHex(%q): unexpected error: %v", tt.hex, err)
			continue
		}
		if c.R != tt.r || c.G != tt.g || c.B != tt.b {
			t.Errorf("ColorFromHex(%q): expected RGB(%d,%d,%d), got RGB(%d,%d,%d)",
				tt.hex, tt.r, tt.g, tt.b, c.R, c.G, c.B)
		}
	}
}

func TestColorIsDefault(t *testing.T) {
	if !ColorDefault.IsDefault() {
		t.Error("ColorDefault should be default")
	}

	c := ColorFromRGB(0, 0, 0)
	if c.IsDefault() {
		t.Error("RGB(0,0,0) should not be default")
	}

	c = ColorFromIndex(0)
	if c.IsDefault() {
		t.Error("Index(0) should not be default")
	}

	// Default colors should be equal to each other
	if !ColorDefault.Equals(ColorDefault) {
		t.Error("ColorDefault should equal itself")
	}

	// Default should not equal non-default
	if ColorDefault.Equals(ColorBlack) {
		t.Error("ColorDefault should not equal ColorBlack")
	}
}

func TestColorEquals(t *testing.T) {
	c1 := ColorFromRGB(255, 128, 64)
	c2 := ColorFromRGB(255, 128, 64)
	c3 := ColorFromRGB(255, 128, 65)
	c4 := ColorFromIndex(255)

	if !c1.Equals(c2) {
		t.Error("identical RGB colors should be equal")
	}
	if c1.Equals(c3) {
		t.Error("different RGB colors should not be equal")
	}
	if c1.Equals(c4) {
		t.Error("RGB and indexed colors should not be equal")
	}
}

func TestColorString(t *testing.T) {
	tests := []struct {
		color Color
		want  string
	}{
		{ColorDefault, "default"},
		{ColorFromIndex(42), "idx(42)"},
		{ColorFromRGB(255, 128, 64), "#FF8040"},
	}

	for _, tt := range tests {
		got := tt.color.String()
		if got != tt.want {
			t.Errorf("Color.String(): expected %q, got %q", tt.want, got)
		}
	}
}

func TestColorToHex(t *testing.T) {
	c := ColorFromRGB(255, 128, 64)
	want := "#FF8040"
	got := c.ToHex()
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	// Indexed colors return empty string
	c = ColorFromIndex(42)
	if c.ToHex() != "" {
		t.Error("indexed color should return empty hex")
	}
}

func TestColorLighten(t *testing.T) {
	c := ColorFromRGB(100, 100, 100)
	lighter := c.Lighten(0.5)

	if lighter.R <= c.R || lighter.G <= c.G || lighter.B <= c.B {
		t.Error("lightened color should have higher RGB values")
	}

	// Indexed colors are unchanged
	indexed := ColorFromIndex(42)
	lightIndexed := indexed.Lighten(0.5)
	if !lightIndexed.Equals(indexed) {
		t.Error("indexed colors should not change when lightened")
	}
}

func TestColorDarken(t *testing.T) {
	c := ColorFromRGB(200, 200, 200)
	darker := c.Darken(0.5)

	if darker.R >= c.R || darker.G >= c.G || darker.B >= c.B {
		t.Error("darkened color should have lower RGB values")
	}

	// Indexed colors are unchanged
	indexed := ColorFromIndex(42)
	darkIndexed := indexed.Darken(0.5)
	if !darkIndexed.Equals(indexed) {
		t.Error("indexed colors should not change when darkened")
	}
}

func TestColorBlend(t *testing.T) {
	c1 := ColorFromRGB(0, 0, 0)
	c2 := ColorFromRGB(255, 255, 255)

	// Blend at 0.0 should return c1
	blend0 := c1.Blend(c2, 0.0)
	if blend0.R != 0 || blend0.G != 0 || blend0.B != 0 {
		t.Error("blend(0.0) should return first color")
	}

	// Blend at 1.0 should return c2
	blend1 := c1.Blend(c2, 1.0)
	if blend1.R != 255 || blend1.G != 255 || blend1.B != 255 {
		t.Error("blend(1.0) should return second color")
	}

	// Blend at 0.5 should be middle
	blend5 := c1.Blend(c2, 0.5)
	if blend5.R < 120 || blend5.R > 135 {
		t.Errorf("blend(0.5) R should be ~127, got %d", blend5.R)
	}
}
