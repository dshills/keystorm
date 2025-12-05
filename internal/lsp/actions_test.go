package lsp

import (
	"testing"
)

func TestNewActionsService(t *testing.T) {
	as := NewActionsService(nil)
	if as == nil {
		t.Fatal("NewActionsService returned nil")
	}

	if as.formatOnSave {
		t.Error("formatOnSave should default to false")
	}

	if as.codeActionCacheAge != 10 {
		t.Errorf("codeActionCacheAge: got %d, want 10", as.codeActionCacheAge)
	}
}

func TestActionsServiceOptions(t *testing.T) {
	opts := FormattingOptions{
		TabSize:      2,
		InsertSpaces: true,
	}

	as := NewActionsService(nil,
		WithFormatOnSave(true),
		WithFormatOnType(true),
		WithFormattingOptions(opts),
		WithFormatExcludes([]string{"*.min.js", "vendor/*"}),
		WithCodeActionKinds([]CodeActionKind{CodeActionKindQuickFix}),
		WithRenameConfirmation(false),
		WithCodeActionCacheAge(30),
	)

	if !as.formatOnSave {
		t.Error("formatOnSave should be true")
	}

	if !as.formatOnType {
		t.Error("formatOnType should be true")
	}

	if as.formatOptions.TabSize != 2 {
		t.Errorf("TabSize: got %d, want 2", as.formatOptions.TabSize)
	}

	if !as.formatOptions.InsertSpaces {
		t.Error("InsertSpaces should be true")
	}

	if len(as.formatExcludes) != 2 {
		t.Errorf("formatExcludes: got %d items, want 2", len(as.formatExcludes))
	}

	if len(as.codeActionKinds) != 1 {
		t.Errorf("codeActionKinds: got %d items, want 1", len(as.codeActionKinds))
	}

	if as.renameConfirmation {
		t.Error("renameConfirmation should be false")
	}

	if as.codeActionCacheAge != 30 {
		t.Errorf("codeActionCacheAge: got %d, want 30", as.codeActionCacheAge)
	}
}

func TestDefaultFormattingOptions(t *testing.T) {
	opts := DefaultFormattingOptions()

	if opts.TabSize != 4 {
		t.Errorf("TabSize: got %d, want 4", opts.TabSize)
	}

	if opts.InsertSpaces {
		t.Error("InsertSpaces should be false (tabs)")
	}

	if !opts.TrimTrailingWhitespace {
		t.Error("TrimTrailingWhitespace should be true")
	}

	if !opts.InsertFinalNewline {
		t.Error("InsertFinalNewline should be true")
	}

	if !opts.TrimFinalNewlines {
		t.Error("TrimFinalNewlines should be true")
	}
}

func TestCategorizeActions(t *testing.T) {
	as := NewActionsService(nil)

	actions := []CodeAction{
		{Title: "Fix typo", Kind: CodeActionKindQuickFix},
		{Title: "Extract method", Kind: CodeActionKindRefactorExtract},
		{Title: "Inline variable", Kind: CodeActionKindRefactorInline},
		{Title: "Organize imports", Kind: CodeActionKindSourceOrganizeImports},
		{Title: "Unknown action", Kind: ""},
	}

	result := as.categorizeActions(actions)

	if result.TotalCount != 5 {
		t.Errorf("TotalCount: got %d, want 5", result.TotalCount)
	}

	if len(result.QuickFixes) != 1 {
		t.Errorf("QuickFixes: got %d, want 1", len(result.QuickFixes))
	}

	if len(result.Refactors) != 2 {
		t.Errorf("Refactors: got %d, want 2", len(result.Refactors))
	}

	if len(result.SourceFixes) != 1 {
		t.Errorf("SourceFixes: got %d, want 1", len(result.SourceFixes))
	}

	if len(result.OtherActions) != 1 {
		t.Errorf("OtherActions: got %d, want 1", len(result.OtherActions))
	}
}

func TestFilterActionsByKind(t *testing.T) {
	kinds := []CodeActionKind{CodeActionKindQuickFix, CodeActionKindRefactor}
	as := NewActionsService(nil,
		WithCodeActionKinds(kinds),
	)

	actions := []CodeAction{
		{Title: "Fix typo", Kind: CodeActionKindQuickFix},
		{Title: "Extract method", Kind: CodeActionKindRefactorExtract},
		{Title: "Organize imports", Kind: CodeActionKindSourceOrganizeImports},
	}

	filtered := as.filterActionsByKindWith(actions, kinds)

	if len(filtered) != 2 {
		t.Errorf("Filtered: got %d, want 2", len(filtered))
	}

	// Should include quickfix and refactor.extract (prefix match)
	hasQuickFix := false
	hasRefactor := false
	for _, a := range filtered {
		if a.Kind == CodeActionKindQuickFix {
			hasQuickFix = true
		}
		if a.Kind == CodeActionKindRefactorExtract {
			hasRefactor = true
		}
	}

	if !hasQuickFix {
		t.Error("Should include quickfix action")
	}
	if !hasRefactor {
		t.Error("Should include refactor action")
	}
}

func TestIsExcludedFromFormatting(t *testing.T) {
	as := NewActionsService(nil,
		WithFormatExcludes([]string{"*.min.js", "*.generated.go", "vendor/*"}),
	)

	tests := []struct {
		path string
		want bool
	}{
		{"/project/src/app.js", false},
		{"/project/dist/app.min.js", true},
		{"/project/generated.go", false}, // not matching *.generated.go
		{"/project/types.generated.go", true},
		{"/project/main.go", false},
	}

	for _, tt := range tests {
		// Call with lock held as required by the method
		as.mu.RLock()
		got := as.isExcludedFromFormattingLocked(tt.path)
		as.mu.RUnlock()
		if got != tt.want {
			t.Errorf("isExcludedFromFormattingLocked(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestShouldFormatOnSave(t *testing.T) {
	as := NewActionsService(nil,
		WithFormatOnSave(true),
		WithFormatExcludes([]string{"*.min.js"}),
	)

	if !as.ShouldFormatOnSave("/project/main.go") {
		t.Error("Should format main.go on save")
	}

	if as.ShouldFormatOnSave("/project/app.min.js") {
		t.Error("Should NOT format app.min.js on save")
	}

	// Disable format on save
	as.SetFormatOnSave(false)

	if as.ShouldFormatOnSave("/project/main.go") {
		t.Error("Should NOT format when formatOnSave is disabled")
	}
}

func TestSetFormattingOptions(t *testing.T) {
	as := NewActionsService(nil)

	opts := FormattingOptions{
		TabSize:      8,
		InsertSpaces: true,
	}

	as.SetFormattingOptions(opts)
	got := as.GetFormattingOptions()

	if got.TabSize != 8 {
		t.Errorf("TabSize: got %d, want 8", got.TabSize)
	}

	if !got.InsertSpaces {
		t.Error("InsertSpaces should be true")
	}
}

func TestCodeActionKindString(t *testing.T) {
	tests := []struct {
		kind CodeActionKind
		want string
	}{
		{CodeActionKindQuickFix, "Quick Fix"},
		{CodeActionKindRefactor, "Refactor"},
		{CodeActionKindRefactorExtract, "Extract"},
		{CodeActionKindRefactorInline, "Inline"},
		{CodeActionKindRefactorRewrite, "Rewrite"},
		{CodeActionKindSource, "Source"},
		{CodeActionKindSourceOrganizeImports, "Organize Imports"},
		{"custom.action", "custom.action"},
		{"", "Action"},
	}

	for _, tt := range tests {
		got := CodeActionKindString(tt.kind)
		if got != tt.want {
			t.Errorf("CodeActionKindString(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestFormatCodeAction(t *testing.T) {
	action := CodeAction{
		Title: "Fix typo",
		Kind:  CodeActionKindQuickFix,
	}

	got := FormatCodeAction(action)
	want := "[Quick Fix] Fix typo"
	if got != want {
		t.Errorf("FormatCodeAction: got %q, want %q", got, want)
	}

	// Preferred action
	action.IsPreferred = true
	got = FormatCodeAction(action)
	want = "[Quick Fix] Fix typo (preferred)"
	if got != want {
		t.Errorf("FormatCodeAction (preferred): got %q, want %q", got, want)
	}
}

func TestSortCodeActions(t *testing.T) {
	actions := []CodeAction{
		{Title: "Refactor", Kind: CodeActionKindRefactor, IsPreferred: false},
		{Title: "Source", Kind: CodeActionKindSource, IsPreferred: false},
		{Title: "Quick Fix", Kind: CodeActionKindQuickFix, IsPreferred: false},
		{Title: "Preferred Fix", Kind: CodeActionKindQuickFix, IsPreferred: true},
	}

	SortCodeActions(actions)

	// Preferred should be first
	if actions[0].Title != "Preferred Fix" {
		t.Errorf("First action should be preferred, got %q", actions[0].Title)
	}

	// Then quickfix
	if actions[1].Kind != CodeActionKindQuickFix {
		t.Errorf("Second action should be quickfix, got %q", actions[1].Kind)
	}

	// Then refactor
	if actions[2].Kind != CodeActionKindRefactor {
		t.Errorf("Third action should be refactor, got %q", actions[2].Kind)
	}

	// Then source
	if actions[3].Kind != CodeActionKindSource {
		t.Errorf("Fourth action should be source, got %q", actions[3].Kind)
	}
}

func TestGroupCodeActionsByKind(t *testing.T) {
	actions := []CodeAction{
		{Title: "Fix 1", Kind: CodeActionKindQuickFix},
		{Title: "Fix 2", Kind: CodeActionKindQuickFix},
		{Title: "Extract", Kind: CodeActionKindRefactorExtract},
		{Title: "Organize", Kind: CodeActionKindSourceOrganizeImports},
	}

	grouped := GroupCodeActionsByKind(actions)

	if len(grouped[CodeActionKindQuickFix]) != 2 {
		t.Errorf("QuickFix group: got %d, want 2", len(grouped[CodeActionKindQuickFix]))
	}

	// refactor.extract should be grouped under "refactor"
	if len(grouped[CodeActionKindRefactor]) != 1 {
		t.Errorf("Refactor group: got %d, want 1", len(grouped[CodeActionKindRefactor]))
	}

	// source.organizeImports should be grouped under "source"
	if len(grouped[CodeActionKindSource]) != 1 {
		t.Errorf("Source group: got %d, want 1", len(grouped[CodeActionKindSource]))
	}
}

func TestFormatTextEdit(t *testing.T) {
	// Single line edit
	edit1 := TextEdit{
		Range:   Range{Start: Position{Line: 5, Character: 0}, End: Position{Line: 5, Character: 10}},
		NewText: "new text",
	}
	got := FormatTextEdit(edit1)
	if got != `Line 6: "new text"` {
		t.Errorf("FormatTextEdit (single line): got %q", got)
	}

	// Multi-line edit
	edit2 := TextEdit{
		Range:   Range{Start: Position{Line: 5, Character: 0}, End: Position{Line: 10, Character: 0}},
		NewText: "replacement",
	}
	got = FormatTextEdit(edit2)
	if got != `Lines 6-11: "replacement"` {
		t.Errorf("FormatTextEdit (multi-line): got %q", got)
	}
}

func TestCountWorkspaceEditChanges(t *testing.T) {
	// Nil edit
	if CountWorkspaceEditChanges(nil) != 0 {
		t.Error("Nil edit should return 0")
	}

	// Edit with changes
	edit := &WorkspaceEdit{
		Changes: map[DocumentURI][]TextEdit{
			"file:///a.go": {{NewText: "a"}, {NewText: "b"}},
			"file:///b.go": {{NewText: "c"}},
		},
	}

	if CountWorkspaceEditChanges(edit) != 3 {
		t.Errorf("CountWorkspaceEditChanges: got %d, want 3", CountWorkspaceEditChanges(edit))
	}
}

func TestGetWorkspaceEditFiles(t *testing.T) {
	// Nil edit
	files := GetWorkspaceEditFiles(nil)
	if files != nil {
		t.Error("Nil edit should return nil")
	}

	// Edit with changes
	edit := &WorkspaceEdit{
		Changes: map[DocumentURI][]TextEdit{
			"file:///b.go": {{NewText: "b"}},
			"file:///a.go": {{NewText: "a"}},
		},
	}

	files = GetWorkspaceEditFiles(edit)
	if len(files) != 2 {
		t.Errorf("GetWorkspaceEditFiles: got %d files, want 2", len(files))
	}

	// Should be sorted
	if files[0] != "/a.go" {
		t.Errorf("First file should be /a.go, got %q", files[0])
	}
}

func TestCacheInvalidation(t *testing.T) {
	as := NewActionsService(nil)

	// Manually add cache entries
	as.codeActionCache[actionCacheKey{path: "/test/a.go", startLine: 1}] = &actionCacheEntry{}
	as.codeActionCache[actionCacheKey{path: "/test/a.go", startLine: 2}] = &actionCacheEntry{}
	as.codeActionCache[actionCacheKey{path: "/test/b.go", startLine: 1}] = &actionCacheEntry{}

	as.InvalidateCodeActionCache("/test/a.go")

	if len(as.codeActionCache) != 1 {
		t.Errorf("After InvalidateCodeActionCache: got %d entries, want 1", len(as.codeActionCache))
	}

	// Clear all
	as.codeActionCache[actionCacheKey{path: "/test/c.go"}] = &actionCacheEntry{}
	as.ClearCodeActionCache()

	if len(as.codeActionCache) != 0 {
		t.Errorf("After ClearCodeActionCache: got %d entries, want 0", len(as.codeActionCache))
	}
}

func TestSignatureHelpState(t *testing.T) {
	as := NewActionsService(nil)

	// Initially no active signature
	if as.GetActiveSignature() != nil {
		t.Error("Should have no active signature initially")
	}

	// Clear should not panic
	as.ClearSignatureHelp()

	if as.GetActiveSignature() != nil {
		t.Error("Should still have no active signature after clear")
	}
}

func TestBuildSignatureResult(t *testing.T) {
	as := NewActionsService(nil)

	help := &SignatureHelp{
		Signatures: []SignatureInformation{
			{
				Label:           "func(a int, b string) bool",
				Documentation:   "Test function",
				ActiveParameter: 1,
				Parameters: []ParameterInformation{
					{Label: "a int", Documentation: "First param"},
					{Label: "b string", Documentation: "Second param"},
				},
			},
		},
		ActiveSignature: 0,
		ActiveParameter: 1,
	}

	result := as.buildSignatureResult(help)

	if !result.HasActiveSignature {
		t.Error("Should have active signature")
	}

	if len(result.Signatures) != 1 {
		t.Errorf("Signatures: got %d, want 1", len(result.Signatures))
	}

	sig := result.Signatures[0]
	if sig.Label != "func(a int, b string) bool" {
		t.Errorf("Label: got %q", sig.Label)
	}

	if sig.Documentation != "Test function" {
		t.Errorf("Documentation: got %q", sig.Documentation)
	}

	if len(sig.Parameters) != 2 {
		t.Errorf("Parameters: got %d, want 2", len(sig.Parameters))
	}

	// Second parameter should be active
	if !sig.Parameters[1].IsActive {
		t.Error("Second parameter should be active")
	}

	if sig.ActiveParameter == nil {
		t.Error("ActiveParameter should not be nil")
	}
}

func TestExtractDocumentation(t *testing.T) {
	tests := []struct {
		doc  any
		want string
	}{
		{nil, ""},
		{"plain string", "plain string"},
		{map[string]any{"kind": "markdown", "value": "# Header"}, "# Header"},
		{map[string]any{"kind": "plaintext"}, ""},
		{123, ""},
	}

	for i, tt := range tests {
		got := extractDocumentation(tt.doc)
		if got != tt.want {
			t.Errorf("test %d: extractDocumentation = %q, want %q", i, got, tt.want)
		}
	}
}

func TestExtractParameterLabel(t *testing.T) {
	tests := []struct {
		label any
		want  string
	}{
		{nil, ""},
		{"param string", "param string"},
		{[]any{0, 5}, ""}, // range format - not supported yet
		{123, ""},
	}

	for i, tt := range tests {
		got := extractParameterLabel(tt.label)
		if got != tt.want {
			t.Errorf("test %d: extractParameterLabel = %q, want %q", i, got, tt.want)
		}
	}
}

func TestNeedsRenameConfirmation(t *testing.T) {
	// Default is true
	as := NewActionsService(nil)
	if !as.NeedsRenameConfirmation() {
		t.Error("Default should require rename confirmation")
	}

	// With confirmation disabled
	as = NewActionsService(nil, WithRenameConfirmation(false))
	if as.NeedsRenameConfirmation() {
		t.Error("Should not require confirmation when disabled")
	}
}
