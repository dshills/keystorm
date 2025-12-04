package vim

import (
	"testing"

	"github.com/dshills/keystorm/internal/input/key"
)

// Helper to create a rune key event
func runeEvent(r rune) key.Event {
	return key.NewRuneEvent(r, key.ModNone)
}

// Helper to parse a sequence of characters
func parseSequence(p *Parser, s string) ParseResult {
	var result ParseResult
	for _, r := range s {
		result = p.Parse(runeEvent(r))
	}
	return result
}

func TestParserMotions(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAction string
		wantCount  int
	}{
		{"simple h", "h", "cursor.left", 0},
		{"simple j", "j", "cursor.down", 0},
		{"simple k", "k", "cursor.up", 0},
		{"simple l", "l", "cursor.right", 0},
		{"simple w", "w", "cursor.wordForward", 0},
		{"simple b", "b", "cursor.wordBackward", 0},
		{"simple e", "e", "cursor.wordEnd", 0},
		{"simple 0", "0", "cursor.lineStart", 0},
		{"simple $", "$", "cursor.lineEnd", 0},
		{"simple G", "G", "cursor.documentEnd", 0},
		{"gg", "gg", "cursor.documentStart", 0},
		{"5j", "5j", "cursor.down", 5},
		{"10w", "10w", "cursor.wordForward", 10},
		{"3b", "3b", "cursor.wordBackward", 3},
		{"25G", "25G", "cursor.documentEnd", 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			result := parseSequence(p, tt.input)

			if result.Status != StatusComplete {
				t.Fatalf("expected StatusComplete, got %v", result.Status)
			}
			if result.Command == nil {
				t.Fatal("expected command, got nil")
			}
			if result.Command.Action != tt.wantAction {
				t.Errorf("expected action %q, got %q", tt.wantAction, result.Command.Action)
			}
			if result.Command.Count != tt.wantCount {
				t.Errorf("expected count %d, got %d", tt.wantCount, result.Command.Count)
			}
		})
	}
}

func TestParserOperatorMotion(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAction string
		wantMotion string
		wantCount  int
	}{
		{"dw", "dw", "editor.delete", "wordForward", 0},
		{"cw", "cw", "editor.change", "wordForward", 0},
		{"yw", "yw", "editor.yank", "wordForward", 0},
		{"d3w", "d3w", "editor.delete", "wordForward", 3},
		{"3dw", "3dw", "editor.delete", "wordForward", 3},
		{"2d3w", "2d3w", "editor.delete", "wordForward", 6},
		{"dj", "dj", "editor.delete", "down", 0},
		{"y$", "y$", "editor.yank", "lineEnd", 0},
		{"d0", "d0", "editor.delete", "lineStart", 0},
		{"dG", "dG", "editor.delete", "documentEnd", 0},
		{"dgg", "dgg", "editor.delete", "documentStart", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			result := parseSequence(p, tt.input)

			if result.Status != StatusComplete {
				t.Fatalf("expected StatusComplete, got %v", result.Status)
			}
			if result.Command == nil {
				t.Fatal("expected command, got nil")
			}
			if result.Command.Action != tt.wantAction {
				t.Errorf("expected action %q, got %q", tt.wantAction, result.Command.Action)
			}
			if result.Command.Motion == nil {
				t.Fatal("expected motion, got nil")
			}
			if result.Command.Motion.Name != tt.wantMotion {
				t.Errorf("expected motion %q, got %q", tt.wantMotion, result.Command.Motion.Name)
			}
			if result.Command.Count != tt.wantCount {
				t.Errorf("expected count %d, got %d", tt.wantCount, result.Command.Count)
			}
		})
	}
}

func TestParserLinewise(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAction string
		wantCount  int
	}{
		{"dd", "dd", "editor.deleteLine", 0},
		{"yy", "yy", "editor.yankLine", 0},
		{"cc", "cc", "editor.changeLine", 0},
		{"5dd", "5dd", "editor.deleteLine", 5},
		{"3yy", "3yy", "editor.yankLine", 3},
		{">>", ">>", "editor.indentLineRight", 0},
		{"<<", "<<", "editor.indentLineLeft", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			result := parseSequence(p, tt.input)

			if result.Status != StatusComplete {
				t.Fatalf("expected StatusComplete, got %v", result.Status)
			}
			if result.Command == nil {
				t.Fatal("expected command, got nil")
			}
			if result.Command.Action != tt.wantAction {
				t.Errorf("expected action %q, got %q", tt.wantAction, result.Command.Action)
			}
			if !result.Command.Linewise {
				t.Error("expected linewise to be true")
			}
			if result.Command.Count != tt.wantCount {
				t.Errorf("expected count %d, got %d", tt.wantCount, result.Command.Count)
			}
		})
	}
}

func TestParserTextObjects(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAction string
		wantObj    string
		wantInner  bool
	}{
		{"diw", "diw", "editor.delete", "word", true},
		{"daw", "daw", "editor.delete", "word", false},
		{"ciw", "ciw", "editor.change", "word", true},
		{"yiw", "yiw", "editor.yank", "word", true},
		{"di\"", "di\"", "editor.delete", "doubleQuote", true},
		{"da\"", "da\"", "editor.delete", "doubleQuote", false},
		{"di(", "di(", "editor.delete", "paren", true},
		{"da)", "da)", "editor.delete", "paren", false},
		{"di{", "di{", "editor.delete", "brace", true},
		{"da}", "da}", "editor.delete", "brace", false},
		{"dip", "dip", "editor.delete", "paragraph", true},
		{"dap", "dap", "editor.delete", "paragraph", false},
		{"dis", "dis", "editor.delete", "sentence", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			result := parseSequence(p, tt.input)

			if result.Status != StatusComplete {
				t.Fatalf("expected StatusComplete, got %v", result.Status)
			}
			if result.Command == nil {
				t.Fatal("expected command, got nil")
			}
			if result.Command.Action != tt.wantAction {
				t.Errorf("expected action %q, got %q", tt.wantAction, result.Command.Action)
			}
			if result.Command.TextObject == nil {
				t.Fatal("expected text object, got nil")
			}
			if result.Command.TextObject.Name != tt.wantObj {
				t.Errorf("expected text object %q, got %q", tt.wantObj, result.Command.TextObject.Name)
			}
			inner := result.Command.TextObjectPrefix == PrefixInner
			if inner != tt.wantInner {
				t.Errorf("expected inner=%v, got %v", tt.wantInner, inner)
			}
		})
	}
}

func TestParserRegisters(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantRegister rune
		wantAction   string
	}{
		{"\"ayw", "\"ayw", 'a', "editor.yank"},
		{"\"bdd", "\"bdd", 'b', "editor.deleteLine"},
		{"\"0p", "\"0p", '0', ""}, // 'p' is not handled in normal parsing
		{"\"_dd", "\"_dd", '_', "editor.deleteLine"},
		{"\"+yy", "\"+yy", '+', "editor.yankLine"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			result := parseSequence(p, tt.input)

			if result.Status != StatusComplete && result.Status != StatusPassthrough {
				t.Fatalf("expected StatusComplete or StatusPassthrough, got %v", result.Status)
			}
			if result.Command == nil && result.Status == StatusComplete {
				t.Fatal("expected command, got nil")
			}
			if result.Command != nil {
				if result.Command.Register != tt.wantRegister {
					t.Errorf("expected register %c, got %c", tt.wantRegister, result.Command.Register)
				}
				if tt.wantAction != "" && result.Command.Action != tt.wantAction {
					t.Errorf("expected action %q, got %q", tt.wantAction, result.Command.Action)
				}
			}
		})
	}
}

func TestParserCharSearch(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAction string
		wantChar   rune
	}{
		{"fa", "fa", "cursor.findChar", 'a'},
		{"Fa", "Fa", "cursor.findCharBack", 'a'},
		{"tx", "tx", "cursor.tillChar", 'x'},
		{"T.", "T.", "cursor.tillCharBack", '.'},
		{"3fa", "3fa", "cursor.findChar", 'a'},
		{"dfa", "dfa", "editor.delete", 'a'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			result := parseSequence(p, tt.input)

			if result.Status != StatusComplete {
				t.Fatalf("expected StatusComplete, got %v", result.Status)
			}
			if result.Command == nil {
				t.Fatal("expected command, got nil")
			}
			if result.Command.Action != tt.wantAction {
				t.Errorf("expected action %q, got %q", tt.wantAction, result.Command.Action)
			}
			if result.Command.CharArg != tt.wantChar {
				t.Errorf("expected char %c, got %c", tt.wantChar, result.Command.CharArg)
			}
		})
	}
}

func TestParserMarks(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAction string
		wantMark   string
	}{
		{"ma", "ma", "mark.set", "a"},
		{"mA", "mA", "mark.set", "A"},
		{"m0", "m0", "mark.set", "0"},
		{"'a", "'a", "mark.goto", "a"},
		{"`a", "`a", "mark.goto", "a"},
		{"'.", "'.", "mark.goto", "."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			result := parseSequence(p, tt.input)

			if result.Status != StatusComplete {
				t.Fatalf("expected StatusComplete, got %v", result.Status)
			}
			if result.Command == nil {
				t.Fatal("expected command, got nil")
			}
			if result.Command.Action != tt.wantAction {
				t.Errorf("expected action %q, got %q", tt.wantAction, result.Command.Action)
			}
			if mark, ok := result.Command.Args["mark"].(string); !ok || mark != tt.wantMark {
				t.Errorf("expected mark %q, got %q", tt.wantMark, mark)
			}
		})
	}
}

func TestParserPendingState(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		state  ParseState
		status ParseStatus
	}{
		{"d", "d", StateOperator, StatusPending},
		{"y", "y", StateOperator, StatusPending},
		{"c", "c", StateOperator, StatusPending},
		{"g", "g", StateGPrefix, StatusPending},
		{"di", "di", StateTextObjectPrefix, StatusPending},
		{"3", "3", StateCount, StatusPending},
		{"3d", "3d", StateOperator, StatusPending},
		{"\"", "\"", StateRegister, StatusPending},
		{"f", "f", StateCharSearch, StatusPending},
		{"m", "m", StateMarkSet, StatusPending},
		{"'", "'", StateMarkGoto, StatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			result := parseSequence(p, tt.input)

			if result.Status != tt.status {
				t.Errorf("expected status %v, got %v", tt.status, result.Status)
			}
			if p.State() != tt.state {
				t.Errorf("expected state %v, got %v", tt.state, p.State())
			}
		})
	}
}

func TestParserReset(t *testing.T) {
	p := NewParser()

	// Build up some state
	p.Parse(runeEvent('3'))
	p.Parse(runeEvent('"'))
	p.Parse(runeEvent('a'))
	p.Parse(runeEvent('d'))

	// Reset
	p.Reset()

	if p.state != StateInitial {
		t.Errorf("expected StateInitial after reset, got %v", p.state)
	}
	if p.count1.Active {
		t.Error("expected count1 to be inactive after reset")
	}
	if p.register != 0 {
		t.Error("expected register to be 0 after reset")
	}
	if p.operator != nil {
		t.Error("expected operator to be nil after reset")
	}
	if len(p.pendingKeys) != 0 {
		t.Error("expected pendingKeys to be empty after reset")
	}
}

func TestParserEscapeReset(t *testing.T) {
	p := NewParser()

	// Build up some state
	p.Parse(runeEvent('d'))
	p.Parse(runeEvent('i'))

	// Escape should reset
	result := p.Parse(key.NewSpecialEvent(key.KeyEscape, key.ModNone))

	if result.Status != StatusPassthrough {
		t.Errorf("expected StatusPassthrough for escape, got %v", result.Status)
	}
	if p.state != StateInitial {
		t.Errorf("expected StateInitial after escape, got %v", p.state)
	}
}

func TestCountState(t *testing.T) {
	t.Run("accumulate digits", func(t *testing.T) {
		cs := NewCountState()
		cs.AccumulateDigit('1')
		cs.AccumulateDigit('2')
		cs.AccumulateDigit('3')

		if cs.Value != 123 {
			t.Errorf("expected 123, got %d", cs.Value)
		}
		if cs.Get() != 123 {
			t.Errorf("expected Get() = 123, got %d", cs.Get())
		}
	})

	t.Run("zero not start", func(t *testing.T) {
		cs := NewCountState()
		if cs.AccumulateDigit('0') {
			t.Error("expected '0' to be rejected at start")
		}
	})

	t.Run("zero after start", func(t *testing.T) {
		cs := NewCountState()
		cs.AccumulateDigit('1')
		if !cs.AccumulateDigit('0') {
			t.Error("expected '0' to be accepted after start")
		}
		if cs.Value != 10 {
			t.Errorf("expected 10, got %d", cs.Value)
		}
	})

	t.Run("default count", func(t *testing.T) {
		cs := NewCountState()
		if cs.Get() != 1 {
			t.Errorf("expected default Get() = 1, got %d", cs.Get())
		}
	})

	t.Run("multiply", func(t *testing.T) {
		cs := NewCountState()
		cs.AccumulateDigit('3')
		if cs.Multiply(4) != 12 {
			t.Errorf("expected Multiply(4) = 12, got %d", cs.Multiply(4))
		}
	})
}

func TestOperatorLookup(t *testing.T) {
	tests := []struct {
		key      rune
		wantOp   bool
		wantName string
	}{
		{'d', true, "delete"},
		{'c', true, "change"},
		{'y', true, "yank"},
		{'>', true, "indentRight"},
		{'<', true, "indentLeft"},
		{'x', false, ""},
		{'i', false, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			op := GetOperator(tt.key)
			if tt.wantOp {
				if op == nil {
					t.Fatal("expected operator, got nil")
				}
				if op.Name != tt.wantName {
					t.Errorf("expected name %q, got %q", tt.wantName, op.Name)
				}
			} else {
				if op != nil {
					t.Errorf("expected nil, got operator %q", op.Name)
				}
			}
		})
	}
}

func TestMotionLookup(t *testing.T) {
	tests := []struct {
		key        rune
		wantMotion bool
		wantName   string
	}{
		{'h', true, "left"},
		{'j', true, "down"},
		{'k', true, "up"},
		{'l', true, "right"},
		{'w', true, "wordForward"},
		{'b', true, "wordBackward"},
		{'e', true, "wordEnd"},
		{'x', false, ""},
		{'d', false, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			m := GetMotion(tt.key)
			if tt.wantMotion {
				if m == nil {
					t.Fatal("expected motion, got nil")
				}
				if m.Name != tt.wantName {
					t.Errorf("expected name %q, got %q", tt.wantName, m.Name)
				}
			} else {
				if m != nil {
					t.Errorf("expected nil, got motion %q", m.Name)
				}
			}
		})
	}
}

func TestTextObjectLookup(t *testing.T) {
	tests := []struct {
		key     rune
		wantObj bool
		wantKey rune
	}{
		{'w', true, 'w'},
		{'W', true, 'W'},
		{'s', true, 's'},
		{'p', true, 'p'},
		{'"', true, '"'},
		{'(', true, '('},
		{')', true, ')'},
		{'x', false, 0},
	}

	for _, tt := range tests {
		name := string(tt.key)
		if tt.key == '"' {
			name = "dquote"
		}
		t.Run(name, func(t *testing.T) {
			obj := GetTextObject(tt.key)
			if tt.wantObj {
				if obj == nil {
					t.Fatal("expected text object, got nil")
				}
				if obj.Key != tt.wantKey {
					t.Errorf("expected key %c, got %c", tt.wantKey, obj.Key)
				}
			} else {
				if obj != nil {
					t.Errorf("expected nil, got text object %q", obj.Name)
				}
			}
		})
	}
}

func TestRegisterStore(t *testing.T) {
	t.Run("set and get", func(t *testing.T) {
		rs := NewRegisterStore()
		rs.Set('a', "hello", false, false)

		content, linewise, blockwise := rs.Get('a')
		if content != "hello" {
			t.Errorf("expected 'hello', got %q", content)
		}
		if linewise || blockwise {
			t.Error("expected not linewise/blockwise")
		}
	})

	t.Run("uppercase append", func(t *testing.T) {
		rs := NewRegisterStore()
		rs.Set('a', "hello", false, false)
		rs.Set('A', " world", false, false)

		content, _, _ := rs.Get('a')
		if content != "hello world" {
			t.Errorf("expected 'hello world', got %q", content)
		}
	})

	t.Run("black hole", func(t *testing.T) {
		rs := NewRegisterStore()
		rs.Set('_', "should be discarded", false, false)

		content, _, _ := rs.Get('_')
		if content != "" {
			t.Errorf("expected empty for black hole, got %q", content)
		}
	})

	t.Run("yank to register 0", func(t *testing.T) {
		rs := NewRegisterStore()
		rs.SetYank("yanked text", false, false)

		content, _, _ := rs.Get('0')
		if content != "yanked text" {
			t.Errorf("expected 'yanked text' in 0, got %q", content)
		}

		content, _, _ = rs.Get('"')
		if content != "yanked text" {
			t.Errorf("expected 'yanked text' in unnamed, got %q", content)
		}
	})

	t.Run("delete rotation", func(t *testing.T) {
		rs := NewRegisterStore()

		// First delete goes to "1
		rs.SetDelete("first", false, false, false)
		content, _, _ := rs.Get('1')
		if content != "first" {
			t.Errorf("expected 'first' in 1, got %q", content)
		}

		// Second delete rotates: "1 -> "2, new goes to "1
		rs.SetDelete("second", false, false, false)
		content, _, _ = rs.Get('1')
		if content != "second" {
			t.Errorf("expected 'second' in 1, got %q", content)
		}
		content, _, _ = rs.Get('2')
		if content != "first" {
			t.Errorf("expected 'first' in 2, got %q", content)
		}
	})

	t.Run("small delete", func(t *testing.T) {
		rs := NewRegisterStore()
		rs.SetDelete("small", false, false, true)

		content, _, _ := rs.Get('-')
		if content != "small" {
			t.Errorf("expected 'small' in -, got %q", content)
		}
	})
}

func TestIsValidRegister(t *testing.T) {
	valid := []rune{'"', 'a', 'z', 'A', 'Z', '0', '9', '-', '_', '.', '%', '#', ':', '/', '=', '+', '*'}
	invalid := []rune{'!', '@', '$', '^', '&', ' '}

	for _, r := range valid {
		if !IsValidRegister(r) {
			t.Errorf("expected %c to be valid register", r)
		}
	}

	for _, r := range invalid {
		if IsValidRegister(r) {
			t.Errorf("expected %c to be invalid register", r)
		}
	}
}
