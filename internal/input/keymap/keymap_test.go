package keymap

import (
	"strings"
	"testing"

	"github.com/dshills/keystorm/internal/input/key"
	"github.com/dshills/keystorm/internal/input/mode"
)

func TestNewKeymap(t *testing.T) {
	km := NewKeymap("test")

	if km.Name != "test" {
		t.Errorf("Name = %q, want %q", km.Name, "test")
	}
	if len(km.Bindings) != 0 {
		t.Errorf("Bindings should be empty, got %d", len(km.Bindings))
	}
}

func TestKeymapBuilders(t *testing.T) {
	km := NewKeymap("test").
		ForMode("normal").
		ForFileType("go").
		WithPriority(10).
		WithSource("test-source").
		Add("j", "cursor.down").
		Add("k", "cursor.up")

	if km.Mode != "normal" {
		t.Errorf("Mode = %q, want %q", km.Mode, "normal")
	}
	if km.FileType != "go" {
		t.Errorf("FileType = %q, want %q", km.FileType, "go")
	}
	if km.Priority != 10 {
		t.Errorf("Priority = %d, want %d", km.Priority, 10)
	}
	if km.Source != "test-source" {
		t.Errorf("Source = %q, want %q", km.Source, "test-source")
	}
	if len(km.Bindings) != 2 {
		t.Errorf("len(Bindings) = %d, want %d", len(km.Bindings), 2)
	}
}

func TestKeymapValidate(t *testing.T) {
	tests := []struct {
		name    string
		keymap  *Keymap
		wantErr bool
	}{
		{
			name: "valid keymap",
			keymap: &Keymap{
				Bindings: []Binding{
					{Keys: "j", Action: "cursor.down"},
					{Keys: "k", Action: "cursor.up"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty keys",
			keymap: &Keymap{
				Bindings: []Binding{
					{Keys: "", Action: "cursor.down"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty action",
			keymap: &Keymap{
				Bindings: []Binding{
					{Keys: "j", Action: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid key sequence",
			keymap: &Keymap{
				Bindings: []Binding{
					{Keys: "<invalid>", Action: "cursor.down"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.keymap.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestKeymapParse(t *testing.T) {
	km := &Keymap{
		Name: "test",
		Bindings: []Binding{
			{Keys: "j", Action: "cursor.down"},
			{Keys: "g g", Action: "cursor.documentStart"},
		},
	}

	parsed, err := km.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(parsed.ParsedBindings) != 2 {
		t.Errorf("len(ParsedBindings) = %d, want %d", len(parsed.ParsedBindings), 2)
	}

	// Check first binding
	if len(parsed.ParsedBindings[0].Sequence.Events) != 1 {
		t.Errorf("first binding events = %d, want 1", len(parsed.ParsedBindings[0].Sequence.Events))
	}

	// Check second binding
	if len(parsed.ParsedBindings[1].Sequence.Events) != 2 {
		t.Errorf("second binding events = %d, want 2", len(parsed.ParsedBindings[1].Sequence.Events))
	}
}

func TestKeymapClone(t *testing.T) {
	km := NewKeymap("original").
		ForMode("normal").
		Add("j", "cursor.down")

	clone := km.Clone()

	// Modify original
	km.Name = "modified"
	km.Add("k", "cursor.up")

	// Clone should be unchanged
	if clone.Name != "original" {
		t.Errorf("clone.Name = %q, want %q", clone.Name, "original")
	}
	if len(clone.Bindings) != 1 {
		t.Errorf("clone.Bindings = %d, want 1", len(clone.Bindings))
	}
}

func TestNewBinding(t *testing.T) {
	b := NewBinding("j", "cursor.down").
		WithDescription("Move down").
		WithPriority(5).
		WithCategory("Movement").
		WithWhen("editorTextFocus").
		WithArgs(map[string]any{"count": 1})

	if b.Keys != "j" {
		t.Errorf("Keys = %q, want %q", b.Keys, "j")
	}
	if b.Action != "cursor.down" {
		t.Errorf("Action = %q, want %q", b.Action, "cursor.down")
	}
	if b.Description != "Move down" {
		t.Errorf("Description = %q, want %q", b.Description, "Move down")
	}
	if b.Priority != 5 {
		t.Errorf("Priority = %d, want %d", b.Priority, 5)
	}
	if b.Category != "Movement" {
		t.Errorf("Category = %q, want %q", b.Category, "Movement")
	}
	if b.When != "editorTextFocus" {
		t.Errorf("When = %q, want %q", b.When, "editorTextFocus")
	}
	if b.Args["count"] != 1 {
		t.Errorf("Args[count] = %v, want 1", b.Args["count"])
	}
}

func TestParsedBindingMatch(t *testing.T) {
	seq, _ := key.ParseSequence("g g")
	pb := &ParsedBinding{
		Binding:  Binding{Keys: "g g", Action: "cursor.documentStart"},
		Sequence: seq,
	}

	// Should match same sequence
	sameSeq, _ := key.ParseSequence("g g")
	if !pb.Match(sameSeq) {
		t.Error("Should match same sequence")
	}

	// Should not match different sequence
	diffSeq, _ := key.ParseSequence("g j")
	if pb.Match(diffSeq) {
		t.Error("Should not match different sequence")
	}
}

func TestParsedBindingIsPrefix(t *testing.T) {
	seq, _ := key.ParseSequence("g g")
	pb := &ParsedBinding{
		Binding:  Binding{Keys: "g g", Action: "cursor.documentStart"},
		Sequence: seq,
	}

	// Single "g" should be a prefix
	prefixSeq, _ := key.ParseSequence("g")
	if !pb.IsPrefix(prefixSeq) {
		t.Error("'g' should be prefix of 'g g'")
	}

	// "g g" should be a prefix (exact match is a prefix)
	if !pb.IsPrefix(seq) {
		t.Error("'g g' should be prefix of 'g g'")
	}

	// "g j" should not be a prefix
	notPrefix, _ := key.ParseSequence("g j")
	if pb.IsPrefix(notPrefix) {
		t.Error("'g j' should not be prefix of 'g g'")
	}
}

func TestBindingMatchScore(t *testing.T) {
	km1 := &Keymap{Name: "mode-specific", Mode: "normal", Priority: 0}
	km2 := &Keymap{Name: "global", Mode: "", Priority: 0}
	km3 := &Keymap{Name: "filetype", Mode: "normal", FileType: "go", Priority: 0}

	bm1 := BindingMatch{ParsedBinding: &ParsedBinding{}, Keymap: km1}
	bm2 := BindingMatch{ParsedBinding: &ParsedBinding{}, Keymap: km2}
	bm3 := BindingMatch{ParsedBinding: &ParsedBinding{}, Keymap: km3}

	bm1.CalculateScore()
	bm2.CalculateScore()
	bm3.CalculateScore()

	// Mode-specific should beat global
	if !bm1.Less(bm2) {
		t.Error("Mode-specific should beat global")
	}

	// Filetype-specific should beat mode-only
	if !bm3.Less(bm1) {
		t.Error("Filetype-specific should beat mode-only")
	}
}

func TestGroupByCategory(t *testing.T) {
	bindings := []Binding{
		{Keys: "h", Action: "cursor.left", Category: "Movement"},
		{Keys: "j", Action: "cursor.down", Category: "Movement"},
		{Keys: "i", Action: "mode.insert", Category: "Mode"},
		{Keys: "x", Action: "editor.delete", Category: ""},
	}

	groups := GroupByCategory(bindings)

	if len(groups) != 3 {
		t.Errorf("len(groups) = %d, want 3", len(groups))
	}

	// Check Movement category
	found := false
	for _, g := range groups {
		if g.Name == "Movement" {
			found = true
			if len(g.Bindings) != 2 {
				t.Errorf("Movement bindings = %d, want 2", len(g.Bindings))
			}
		}
	}
	if !found {
		t.Error("Movement category not found")
	}

	// Check Other category (for empty category)
	found = false
	for _, g := range groups {
		if g.Name == "Other" {
			found = true
			if len(g.Bindings) != 1 {
				t.Errorf("Other bindings = %d, want 1", len(g.Bindings))
			}
		}
	}
	if !found {
		t.Error("Other category not found")
	}
}

func TestRegistryBasic(t *testing.T) {
	reg := NewRegistry()

	km := NewKeymap("test").
		ForMode("normal").
		Add("j", "cursor.down").
		Add("k", "cursor.up")

	if err := reg.Register(km); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Check keymap is registered
	got := reg.Get("test")
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Name != "test" {
		t.Errorf("Get().Name = %q, want %q", got.Name, "test")
	}

	// Unregister
	reg.Unregister("test")
	if reg.Get("test") != nil {
		t.Error("Get() should return nil after Unregister")
	}
}

func TestRegistryLookup(t *testing.T) {
	reg := NewRegistry()

	km := NewKeymap("normal").
		ForMode("normal").
		Add("j", "cursor.down").
		Add("g g", "cursor.documentStart")

	if err := reg.Register(km); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	ctx := &LookupContext{Mode: "normal"}

	// Single key lookup
	seq, _ := key.ParseSequence("j")
	binding := reg.Lookup(seq, ctx)
	if binding == nil {
		t.Fatal("Lookup('j') returned nil")
	}
	if binding.Action != "cursor.down" {
		t.Errorf("Lookup('j').Action = %q, want %q", binding.Action, "cursor.down")
	}

	// Multi-key lookup
	seq, _ = key.ParseSequence("g g")
	binding = reg.Lookup(seq, ctx)
	if binding == nil {
		t.Fatal("Lookup('g g') returned nil")
	}
	if binding.Action != "cursor.documentStart" {
		t.Errorf("Lookup('g g').Action = %q, want %q", binding.Action, "cursor.documentStart")
	}

	// Non-existent lookup
	seq, _ = key.ParseSequence("x")
	binding = reg.Lookup(seq, ctx)
	if binding != nil {
		t.Error("Lookup('x') should return nil")
	}
}

func TestRegistryHasPrefix(t *testing.T) {
	reg := NewRegistry()

	km := NewKeymap("normal").
		ForMode("normal").
		Add("g g", "cursor.documentStart").
		Add("g j", "cursor.down")

	if err := reg.Register(km); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	ctx := &LookupContext{Mode: "normal"}

	// "g" should be a prefix
	seq, _ := key.ParseSequence("g")
	if !reg.HasPrefix(seq, ctx) {
		t.Error("HasPrefix('g') should return true")
	}

	// "j" should not be a prefix
	seq, _ = key.ParseSequence("j")
	if reg.HasPrefix(seq, ctx) {
		t.Error("HasPrefix('j') should return false")
	}
}

func TestRegistryModeSpecific(t *testing.T) {
	reg := NewRegistry()

	// Normal mode binding
	normalKm := NewKeymap("normal").
		ForMode("normal").
		Add("j", "cursor.down")

	// Global binding
	globalKm := NewKeymap("global").
		Add("C-s", "file.save")

	if err := reg.Register(normalKm); err != nil {
		t.Fatalf("Register(normal) error = %v", err)
	}
	if err := reg.Register(globalKm); err != nil {
		t.Fatalf("Register(global) error = %v", err)
	}

	// Normal mode should find mode-specific binding
	ctx := &LookupContext{Mode: "normal"}
	seq, _ := key.ParseSequence("j")
	binding := reg.Lookup(seq, ctx)
	if binding == nil {
		t.Fatal("Should find 'j' in normal mode")
	}

	// Insert mode should not find normal mode binding
	ctx = &LookupContext{Mode: "insert"}
	binding = reg.Lookup(seq, ctx)
	if binding != nil {
		t.Error("Should not find 'j' in insert mode")
	}

	// Global binding should work in any mode
	seq, _ = key.ParseSequence("C-s")
	ctx = &LookupContext{Mode: "insert"}
	binding = reg.Lookup(seq, ctx)
	if binding == nil {
		t.Fatal("Should find global 'C-s' in insert mode")
	}
}

func TestRegistryConditions(t *testing.T) {
	reg := NewRegistry()

	km := NewKeymap("test").
		ForMode("normal").
		AddBinding(Binding{
			Keys:   "C-s",
			Action: "file.save",
			When:   "editorTextFocus",
		}).
		AddBinding(Binding{
			Keys:   "C-o",
			Action: "file.open",
			When:   "!editorReadonly",
		})

	if err := reg.Register(km); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// With editorTextFocus = true
	ctx := &LookupContext{
		Mode:       "normal",
		Conditions: map[string]bool{"editorTextFocus": true},
	}
	seq, _ := key.ParseSequence("C-s")
	binding := reg.Lookup(seq, ctx)
	if binding == nil {
		t.Error("Should find 'C-s' with editorTextFocus = true")
	}

	// With editorTextFocus = false
	ctx.Conditions["editorTextFocus"] = false
	binding = reg.Lookup(seq, ctx)
	if binding != nil {
		t.Error("Should not find 'C-s' with editorTextFocus = false")
	}

	// Check NOT condition
	ctx.Conditions["editorReadonly"] = false
	seq, _ = key.ParseSequence("C-o")
	binding = reg.Lookup(seq, ctx)
	if binding == nil {
		t.Error("Should find 'C-o' with editorReadonly = false")
	}

	ctx.Conditions["editorReadonly"] = true
	binding = reg.Lookup(seq, ctx)
	if binding != nil {
		t.Error("Should not find 'C-o' with editorReadonly = true")
	}
}

func TestRegistryFileType(t *testing.T) {
	reg := NewRegistry()

	// Go-specific binding
	goKm := NewKeymap("go-keymap").
		ForMode("normal").
		ForFileType("go").
		Add("g f", "go.format")

	// General binding
	generalKm := NewKeymap("general").
		ForMode("normal").
		Add("g f", "editor.format")

	if err := reg.Register(goKm); err != nil {
		t.Fatalf("Register(go) error = %v", err)
	}
	if err := reg.Register(generalKm); err != nil {
		t.Fatalf("Register(general) error = %v", err)
	}

	seq, _ := key.ParseSequence("g f")

	// Go file should use Go-specific binding
	ctx := &LookupContext{Mode: "normal", FileType: "go"}
	binding := reg.Lookup(seq, ctx)
	if binding == nil {
		t.Fatal("Should find binding for Go file")
	}
	if binding.Action != "go.format" {
		t.Errorf("Go file action = %q, want %q", binding.Action, "go.format")
	}

	// Python file should use general binding
	ctx.FileType = "python"
	binding = reg.Lookup(seq, ctx)
	if binding == nil {
		t.Fatal("Should find binding for Python file")
	}
	if binding.Action != "editor.format" {
		t.Errorf("Python file action = %q, want %q", binding.Action, "editor.format")
	}
}

func TestRegistryAllBindings(t *testing.T) {
	reg := NewRegistry()

	km := NewKeymap("test").
		ForMode("normal").
		Add("j", "cursor.down").
		Add("k", "cursor.up")

	if err := reg.Register(km); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	bindings := reg.AllBindings("normal")
	if len(bindings) != 2 {
		t.Errorf("AllBindings() = %d, want 2", len(bindings))
	}
}

func TestDefaultConditionEvaluator(t *testing.T) {
	eval := &DefaultConditionEvaluator{}

	tests := []struct {
		condition string
		ctx       *LookupContext
		want      bool
	}{
		{
			condition: "",
			ctx:       NewLookupContext(),
			want:      true,
		},
		{
			condition: "editorTextFocus",
			ctx: &LookupContext{
				Conditions: map[string]bool{"editorTextFocus": true},
			},
			want: true,
		},
		{
			condition: "editorTextFocus",
			ctx: &LookupContext{
				Conditions: map[string]bool{"editorTextFocus": false},
			},
			want: false,
		},
		{
			condition: "!editorReadonly",
			ctx: &LookupContext{
				Conditions: map[string]bool{"editorReadonly": false},
			},
			want: true,
		},
		{
			condition: "!editorReadonly",
			ctx: &LookupContext{
				Conditions: map[string]bool{"editorReadonly": true},
			},
			want: false,
		},
		{
			condition: "editorTextFocus && !editorReadonly",
			ctx: &LookupContext{
				Conditions: map[string]bool{
					"editorTextFocus": true,
					"editorReadonly":  false,
				},
			},
			want: true,
		},
		{
			condition: "editorTextFocus && !editorReadonly",
			ctx: &LookupContext{
				Conditions: map[string]bool{
					"editorTextFocus": true,
					"editorReadonly":  true,
				},
			},
			want: false,
		},
		{
			condition: "modeFoo || modeBar",
			ctx: &LookupContext{
				Conditions: map[string]bool{
					"modeFoo": false,
					"modeBar": true,
				},
			},
			want: true,
		},
		{
			condition: "resourceLangId == go",
			ctx: &LookupContext{
				Variables: map[string]string{
					"resourceLangId": "go",
				},
			},
			want: true,
		},
		{
			condition: "resourceLangId == python",
			ctx: &LookupContext{
				Variables: map[string]string{
					"resourceLangId": "go",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			got := eval.Evaluate(tt.condition, tt.ctx)
			if got != tt.want {
				t.Errorf("Evaluate(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	reg := NewRegistry()

	if err := LoadDefaults(reg); err != nil {
		t.Fatalf("LoadDefaults() error = %v", err)
	}

	// Check that keymaps are loaded
	keymaps := reg.Keymaps()
	if len(keymaps) < 4 {
		t.Errorf("Expected at least 4 keymaps, got %d", len(keymaps))
	}

	// Check normal mode bindings
	ctx := &LookupContext{Mode: mode.ModeNormal}
	seq, _ := key.ParseSequence("j")
	binding := reg.Lookup(seq, ctx)
	if binding == nil {
		t.Fatal("Should find 'j' binding in normal mode")
	}
	if binding.Action != "cursor.moveDown" {
		t.Errorf("'j' action = %q, want %q", binding.Action, "cursor.moveDown")
	}

	// Check multi-key binding
	seq, _ = key.ParseSequence("g g")
	binding = reg.Lookup(seq, ctx)
	if binding == nil {
		t.Fatal("Should find 'g g' binding in normal mode")
	}
	if binding.Action != "cursor.moveFirstLine" {
		t.Errorf("'g g' action = %q, want %q", binding.Action, "cursor.moveFirstLine")
	}

	// Check insert mode bindings
	ctx.Mode = mode.ModeInsert
	seq, _ = key.ParseSequence("Esc")
	binding = reg.Lookup(seq, ctx)
	if binding == nil {
		t.Fatal("Should find 'Esc' binding in insert mode")
	}
	if binding.Action != "mode.normal" {
		t.Errorf("'Esc' action = %q, want %q", binding.Action, "mode.normal")
	}

	// Check global binding
	seq, _ = key.ParseSequence("C-s")
	binding = reg.Lookup(seq, ctx)
	if binding == nil {
		t.Error("Should find 'C-s' global binding")
	}
}

func TestPrefixTree(t *testing.T) {
	tree := NewPrefixTree()

	km := &Keymap{Name: "test", Mode: "normal"}

	// Insert some bindings
	seq1, _ := key.ParseSequence("g g")
	pb1 := &ParsedBinding{Binding: Binding{Keys: "g g", Action: "action1"}, Sequence: seq1}
	tree.Insert(seq1, "normal", pb1, km)

	seq2, _ := key.ParseSequence("g j")
	pb2 := &ParsedBinding{Binding: Binding{Keys: "g j", Action: "action2"}, Sequence: seq2}
	tree.Insert(seq2, "normal", pb2, km)

	seq3, _ := key.ParseSequence("j")
	pb3 := &ParsedBinding{Binding: Binding{Keys: "j", Action: "action3"}, Sequence: seq3}
	tree.Insert(seq3, "normal", pb3, km)

	// Lookup exact matches
	entries := tree.Lookup(seq1, "normal")
	if len(entries) != 1 {
		t.Errorf("Lookup('g g') = %d entries, want 1", len(entries))
	}
	if entries[0].Binding.Action != "action1" {
		t.Errorf("Lookup('g g').Action = %q, want %q", entries[0].Binding.Action, "action1")
	}

	// Check prefix
	prefixSeq, _ := key.ParseSequence("g")
	if !tree.HasPrefix(prefixSeq, "normal") {
		t.Error("HasPrefix('g') should be true")
	}

	// Remove and verify
	tree.Remove(seq1, "normal", km)
	entries = tree.Lookup(seq1, "normal")
	if len(entries) != 0 {
		t.Errorf("After Remove, Lookup('g g') = %d entries, want 0", len(entries))
	}

	// "g" should still be prefix due to "g j"
	if !tree.HasPrefix(prefixSeq, "normal") {
		t.Error("HasPrefix('g') should still be true after removing 'g g'")
	}
}

func TestLoaderJSON(t *testing.T) {
	jsonData := `{
		"name": "test-keymap",
		"mode": "normal",
		"fileType": "go",
		"priority": 10,
		"source": "test",
		"bindings": [
			{
				"keys": "j",
				"action": "cursor.down",
				"description": "Move down",
				"category": "Movement"
			},
			{
				"keys": "g g",
				"action": "cursor.documentStart",
				"when": "editorTextFocus"
			}
		]
	}`

	loader := NewLoader()
	km, err := loader.LoadReader(strings.NewReader(jsonData))
	if err != nil {
		t.Fatalf("LoadReader() error = %v", err)
	}

	if km.Name != "test-keymap" {
		t.Errorf("Name = %q, want %q", km.Name, "test-keymap")
	}
	if km.Mode != "normal" {
		t.Errorf("Mode = %q, want %q", km.Mode, "normal")
	}
	if km.FileType != "go" {
		t.Errorf("FileType = %q, want %q", km.FileType, "go")
	}
	if km.Priority != 10 {
		t.Errorf("Priority = %d, want %d", km.Priority, 10)
	}
	if len(km.Bindings) != 2 {
		t.Errorf("len(Bindings) = %d, want 2", len(km.Bindings))
	}

	// Check first binding
	b := km.Bindings[0]
	if b.Keys != "j" {
		t.Errorf("Bindings[0].Keys = %q, want %q", b.Keys, "j")
	}
	if b.Description != "Move down" {
		t.Errorf("Bindings[0].Description = %q, want %q", b.Description, "Move down")
	}
	if b.Category != "Movement" {
		t.Errorf("Bindings[0].Category = %q, want %q", b.Category, "Movement")
	}

	// Check second binding
	b = km.Bindings[1]
	if b.When != "editorTextFocus" {
		t.Errorf("Bindings[1].When = %q, want %q", b.When, "editorTextFocus")
	}
}

func TestKeymapMarshalJSON(t *testing.T) {
	km := NewKeymap("test").
		ForMode("normal").
		WithPriority(5).
		AddBinding(Binding{
			Keys:        "j",
			Action:      "cursor.down",
			Description: "Move down",
		})

	data, err := km.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	// Unmarshal and check
	var km2 Keymap
	if err := km2.UnmarshalJSON(data); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if km2.Name != "test" {
		t.Errorf("Name = %q, want %q", km2.Name, "test")
	}
	if km2.Mode != "normal" {
		t.Errorf("Mode = %q, want %q", km2.Mode, "normal")
	}
	if km2.Priority != 5 {
		t.Errorf("Priority = %d, want 5", km2.Priority)
	}
	if len(km2.Bindings) != 1 {
		t.Errorf("len(Bindings) = %d, want 1", len(km2.Bindings))
	}
}

func TestDefaultKeymapCoverage(t *testing.T) {
	// Ensure all default keymaps can be parsed
	keymaps := []*Keymap{
		DefaultNormalKeymap(),
		DefaultInsertKeymap(),
		DefaultVisualKeymap(),
		DefaultCommandKeymap(),
		DefaultGlobalKeymap(),
	}

	for _, km := range keymaps {
		t.Run(km.Name, func(t *testing.T) {
			parsed, err := km.Parse()
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}

			// Verify all bindings parsed
			if len(parsed.ParsedBindings) != len(km.Bindings) {
				t.Errorf("ParsedBindings = %d, want %d", len(parsed.ParsedBindings), len(km.Bindings))
			}
		})
	}
}
