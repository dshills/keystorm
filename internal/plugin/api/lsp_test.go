package api

import (
	"errors"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// mockLSPProvider implements LSPProvider for testing.
type mockLSPProvider struct {
	completions   []CompletionItem
	diagnostics   []Diagnostic
	definition    *Location
	references    []Location
	hover         *HoverInfo
	signatureHelp *SignatureInfo
	format        []TextEdit
	codeActions   []CodeAction
	rename        []TextEdit
	isAvailable   bool
	err           error
}

func newMockLSPProvider() *mockLSPProvider {
	return &mockLSPProvider{
		isAvailable: true,
	}
}

func (m *mockLSPProvider) Completions(bufferPath string, offset int) ([]CompletionItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.completions, nil
}

func (m *mockLSPProvider) Diagnostics(bufferPath string) ([]Diagnostic, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.diagnostics, nil
}

func (m *mockLSPProvider) Definition(bufferPath string, offset int) (*Location, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.definition, nil
}

func (m *mockLSPProvider) References(bufferPath string, offset int, includeDeclaration bool) ([]Location, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.references, nil
}

func (m *mockLSPProvider) Hover(bufferPath string, offset int) (*HoverInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.hover, nil
}

func (m *mockLSPProvider) SignatureHelp(bufferPath string, offset int) (*SignatureInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.signatureHelp, nil
}

func (m *mockLSPProvider) Format(bufferPath string, startOffset, endOffset int) ([]TextEdit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.format, nil
}

func (m *mockLSPProvider) CodeActions(bufferPath string, startOffset, endOffset int, diagnostics []Diagnostic) ([]CodeAction, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.codeActions, nil
}

func (m *mockLSPProvider) Rename(bufferPath string, offset int, newName string) ([]TextEdit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.rename, nil
}

func (m *mockLSPProvider) IsAvailable(bufferPath string) bool {
	return m.isAvailable
}

// mockBufferProviderForLSP implements BufferProvider for LSP tests.
type mockBufferProviderForLSP struct {
	path string
}

func (m *mockBufferProviderForLSP) Text() string                                     { return "" }
func (m *mockBufferProviderForLSP) TextRange(start, end int) (string, error)         { return "", nil }
func (m *mockBufferProviderForLSP) Line(lineNum int) (string, error)                 { return "", nil }
func (m *mockBufferProviderForLSP) LineCount() int                                   { return 0 }
func (m *mockBufferProviderForLSP) Len() int                                         { return 0 }
func (m *mockBufferProviderForLSP) Insert(offset int, text string) (int, error)      { return 0, nil }
func (m *mockBufferProviderForLSP) Delete(start, end int) error                      { return nil }
func (m *mockBufferProviderForLSP) Replace(start, end int, text string) (int, error) { return 0, nil }
func (m *mockBufferProviderForLSP) Undo() bool                                       { return false }
func (m *mockBufferProviderForLSP) Redo() bool                                       { return false }
func (m *mockBufferProviderForLSP) Path() string                                     { return m.path }
func (m *mockBufferProviderForLSP) Modified() bool                                   { return false }

// mockCursorProviderForLSP implements CursorProvider for LSP tests.
type mockCursorProviderForLSP struct {
	offset   int
	selStart int
	selEnd   int
}

func (m *mockCursorProviderForLSP) Get() int                    { return m.offset }
func (m *mockCursorProviderForLSP) GetAll() []int               { return []int{m.offset} }
func (m *mockCursorProviderForLSP) Set(offset int) error        { m.offset = offset; return nil }
func (m *mockCursorProviderForLSP) Add(offset int) error        { return nil }
func (m *mockCursorProviderForLSP) Clear()                      {}
func (m *mockCursorProviderForLSP) Selection() (start, end int) { return m.selStart, m.selEnd }
func (m *mockCursorProviderForLSP) SetSelection(start, end int) error {
	m.selStart = start
	m.selEnd = end
	return nil
}
func (m *mockCursorProviderForLSP) Count() int  { return 1 }
func (m *mockCursorProviderForLSP) Line() int   { return 1 }
func (m *mockCursorProviderForLSP) Column() int { return 1 }

func setupLSPTest(t *testing.T, lsp *mockLSPProvider) (*lua.LState, *LSPModule) {
	t.Helper()

	ctx := &Context{
		LSP:    lsp,
		Buffer: &mockBufferProviderForLSP{path: "/test/file.go"},
		Cursor: &mockCursorProviderForLSP{offset: 100, selStart: -1, selEnd: -1},
	}
	mod := NewLSPModule(ctx, "testplugin")

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	return L, mod
}

func TestLSPModuleName(t *testing.T) {
	ctx := &Context{}
	mod := NewLSPModule(ctx, "test")
	if mod.Name() != "lsp" {
		t.Errorf("Name() = %q, want %q", mod.Name(), "lsp")
	}
}

func TestLSPModuleCapability(t *testing.T) {
	ctx := &Context{}
	mod := NewLSPModule(ctx, "test")
	if mod.RequiredCapability() != security.CapabilityLSP {
		t.Errorf("RequiredCapability() = %q, want %q", mod.RequiredCapability(), security.CapabilityLSP)
	}
}

func TestLSPCompletions(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.completions = []CompletionItem{
		{
			Label:         "println",
			Kind:          CompletionKindFunction,
			Detail:        "func println(a ...interface{})",
			Documentation: "Prints to stdout",
			InsertText:    "println($1)",
			SortText:      "0001",
		},
		{
			Label:  "print",
			Kind:   CompletionKindFunction,
			Detail: "func print(a ...interface{})",
		},
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		items = _ks_lsp.completions("/test/file.go", 50)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	items := L.GetGlobal("items")
	if items == lua.LNil {
		t.Fatal("completions should not return nil")
	}

	tbl := items.(*lua.LTable)
	if tbl.Len() != 2 {
		t.Errorf("completion count = %d, want 2", tbl.Len())
	}

	// Check first item
	first := tbl.RawGetInt(1).(*lua.LTable)
	if L.GetField(first, "label").(lua.LString) != "println" {
		t.Error("first item label should be 'println'")
	}
	if L.GetField(first, "kind").(lua.LNumber) != lua.LNumber(CompletionKindFunction) {
		t.Error("first item kind should be FUNCTION")
	}
}

func TestLSPCompletionsWithDefaults(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.completions = []CompletionItem{{Label: "test"}}

	L, _ := setupLSPTest(t, lsp)

	// Call without arguments - should use current buffer/cursor
	err := L.DoString(`
		items = _ks_lsp.completions()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	items := L.GetGlobal("items")
	if items == lua.LNil {
		t.Fatal("completions with defaults should return items")
	}
}

func TestLSPCompletionsNilProvider(t *testing.T) {
	ctx := &Context{LSP: nil}
	mod := NewLSPModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	err := L.DoString(`
		items = _ks_lsp.completions("/test/file.go", 50)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	items := L.GetGlobal("items")
	if items != lua.LNil {
		t.Error("completions should return nil when provider is nil")
	}
}

func TestLSPDiagnostics(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.diagnostics = []Diagnostic{
		{
			Range: Range{
				StartLine:   10,
				StartColumn: 5,
				EndLine:     10,
				EndColumn:   20,
			},
			Severity: DiagnosticSeverityError,
			Code:     "E001",
			Source:   "go",
			Message:  "undefined: foo",
		},
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		diags = _ks_lsp.diagnostics("/test/file.go")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	diags := L.GetGlobal("diags")
	if diags == lua.LNil {
		t.Fatal("diagnostics should not return nil")
	}

	tbl := diags.(*lua.LTable)
	if tbl.Len() != 1 {
		t.Errorf("diagnostic count = %d, want 1", tbl.Len())
	}

	// Check first diagnostic
	first := tbl.RawGetInt(1).(*lua.LTable)
	if L.GetField(first, "message").(lua.LString) != "undefined: foo" {
		t.Error("diagnostic message mismatch")
	}
	if L.GetField(first, "severity").(lua.LNumber) != lua.LNumber(DiagnosticSeverityError) {
		t.Error("diagnostic severity should be ERROR")
	}

	// Check range (should be 1-indexed in Lua)
	rangeTbl := L.GetField(first, "range").(*lua.LTable)
	if L.GetField(rangeTbl, "start_line").(lua.LNumber) != 11 { // 10 + 1 for 1-indexing
		t.Error("diagnostic range start_line should be 11 (1-indexed)")
	}
}

func TestLSPDefinition(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.definition = &Location{
		Path: "/test/other.go",
		Range: Range{
			StartLine:   50,
			StartColumn: 10,
			EndLine:     50,
			EndColumn:   20,
		},
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		loc = _ks_lsp.definition("/test/file.go", 100)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	loc := L.GetGlobal("loc")
	if loc == lua.LNil {
		t.Fatal("definition should not return nil")
	}

	locTbl := loc.(*lua.LTable)
	if L.GetField(locTbl, "path").(lua.LString) != "/test/other.go" {
		t.Error("definition path mismatch")
	}
}

func TestLSPDefinitionNotFound(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.definition = nil // No definition found

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		loc = _ks_lsp.definition("/test/file.go", 100)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	loc := L.GetGlobal("loc")
	if loc != lua.LNil {
		t.Error("definition should return nil when not found")
	}
}

func TestLSPReferences(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.references = []Location{
		{Path: "/test/file1.go", Range: Range{StartLine: 10, StartColumn: 5, EndLine: 10, EndColumn: 15}},
		{Path: "/test/file2.go", Range: Range{StartLine: 20, StartColumn: 10, EndLine: 20, EndColumn: 20}},
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		refs = _ks_lsp.references("/test/file.go", 100, true)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	refs := L.GetGlobal("refs")
	if refs == lua.LNil {
		t.Fatal("references should not return nil")
	}

	tbl := refs.(*lua.LTable)
	if tbl.Len() != 2 {
		t.Errorf("reference count = %d, want 2", tbl.Len())
	}
}

func TestLSPHover(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.hover = &HoverInfo{
		Contents: "func Println(a ...interface{}) (n int, err error)\n\nPrints to stdout with newline",
		Range: &Range{
			StartLine:   10,
			StartColumn: 5,
			EndLine:     10,
			EndColumn:   12,
		},
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		info = _ks_lsp.hover("/test/file.go", 100)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	info := L.GetGlobal("info")
	if info == lua.LNil {
		t.Fatal("hover should not return nil")
	}

	tbl := info.(*lua.LTable)
	contents := L.GetField(tbl, "contents").(lua.LString)
	if contents == "" {
		t.Error("hover contents should not be empty")
	}

	// Check range exists
	rangeTbl := L.GetField(tbl, "range")
	if rangeTbl == lua.LNil {
		t.Error("hover range should not be nil")
	}
}

func TestLSPHoverNoRange(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.hover = &HoverInfo{
		Contents: "Some documentation",
		Range:    nil,
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		info = _ks_lsp.hover()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	info := L.GetGlobal("info")
	if info == lua.LNil {
		t.Fatal("hover should not return nil")
	}

	tbl := info.(*lua.LTable)
	rangeTbl := L.GetField(tbl, "range")
	if rangeTbl != lua.LNil {
		t.Error("hover range should be nil when not provided")
	}
}

func TestLSPSignatureHelp(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.signatureHelp = &SignatureInfo{
		ActiveSignature: 0,
		ActiveParameter: 1,
		Signatures: []SignatureInformation{
			{
				Label:         "func Println(a ...interface{}) (n int, err error)",
				Documentation: "Prints to stdout",
				Parameters: []ParameterInfo{
					{Label: "a", Documentation: "values to print"},
				},
			},
		},
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		sig = _ks_lsp.signature_help()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	sig := L.GetGlobal("sig")
	if sig == lua.LNil {
		t.Fatal("signature_help should not return nil")
	}

	tbl := sig.(*lua.LTable)
	// Check 1-indexed values
	activeSig := L.GetField(tbl, "active_signature").(lua.LNumber)
	if activeSig != 1 { // 0 + 1 for 1-indexing
		t.Errorf("active_signature = %v, want 1", activeSig)
	}
	activeParam := L.GetField(tbl, "active_parameter").(lua.LNumber)
	if activeParam != 2 { // 1 + 1 for 1-indexing
		t.Errorf("active_parameter = %v, want 2", activeParam)
	}

	sigs := L.GetField(tbl, "signatures").(*lua.LTable)
	if sigs.Len() != 1 {
		t.Errorf("signatures count = %d, want 1", sigs.Len())
	}
}

func TestLSPFormat(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.format = []TextEdit{
		{
			Range:   Range{StartLine: 5, StartColumn: 0, EndLine: 5, EndColumn: 10},
			NewText: "    formatted",
		},
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		edits = _ks_lsp.format("/test/file.go")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	edits := L.GetGlobal("edits")
	if edits == lua.LNil {
		t.Fatal("format should not return nil")
	}

	tbl := edits.(*lua.LTable)
	if tbl.Len() != 1 {
		t.Errorf("edit count = %d, want 1", tbl.Len())
	}

	edit := tbl.RawGetInt(1).(*lua.LTable)
	newText := L.GetField(edit, "new_text").(lua.LString)
	if newText != "    formatted" {
		t.Errorf("new_text = %q, want '    formatted'", newText)
	}
}

func TestLSPFormatWithRange(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.format = []TextEdit{}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		edits = _ks_lsp.format("/test/file.go", 10, 50)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	edits := L.GetGlobal("edits")
	if edits == lua.LNil {
		t.Fatal("format should not return nil")
	}
}

func TestLSPCodeActions(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.codeActions = []CodeAction{
		{
			Title:   "Organize imports",
			Kind:    CodeActionKindSourceOrganize,
			Command: "organize_imports",
			Edits: []TextEdit{
				{Range: Range{StartLine: 1, StartColumn: 0, EndLine: 5, EndColumn: 0}, NewText: "import (\n\t\"fmt\"\n)\n"},
			},
		},
		{
			Title: "Extract variable",
			Kind:  CodeActionKindRefactorExtract,
		},
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		actions = _ks_lsp.code_actions("/test/file.go", 10, 50)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	actions := L.GetGlobal("actions")
	if actions == lua.LNil {
		t.Fatal("code_actions should not return nil")
	}

	tbl := actions.(*lua.LTable)
	if tbl.Len() != 2 {
		t.Errorf("action count = %d, want 2", tbl.Len())
	}

	first := tbl.RawGetInt(1).(*lua.LTable)
	if L.GetField(first, "title").(lua.LString) != "Organize imports" {
		t.Error("first action title mismatch")
	}
	if L.GetField(first, "kind").(lua.LString) != lua.LString(CodeActionKindSourceOrganize) {
		t.Error("first action kind mismatch")
	}
}

func TestLSPCodeActionsWithSelection(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.codeActions = []CodeAction{{Title: "Test"}}

	ctx := &Context{
		LSP:    lsp,
		Buffer: &mockBufferProviderForLSP{path: "/test/file.go"},
		Cursor: &mockCursorProviderForLSP{offset: 100, selStart: 50, selEnd: 150},
	}
	mod := NewLSPModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// Call without range arguments - should use selection
	err := L.DoString(`
		actions = _ks_lsp.code_actions()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	actions := L.GetGlobal("actions")
	if actions == lua.LNil {
		t.Fatal("code_actions with selection should return actions")
	}
}

func TestLSPRename(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.rename = []TextEdit{
		{Range: Range{StartLine: 10, StartColumn: 5, EndLine: 10, EndColumn: 10}, NewText: "newName"},
		{Range: Range{StartLine: 20, StartColumn: 8, EndLine: 20, EndColumn: 13}, NewText: "newName"},
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		edits = _ks_lsp.rename("/test/file.go", 100, "newName")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	edits := L.GetGlobal("edits")
	if edits == lua.LNil {
		t.Fatal("rename should not return nil")
	}

	tbl := edits.(*lua.LTable)
	if tbl.Len() != 2 {
		t.Errorf("edit count = %d, want 2", tbl.Len())
	}
}

func TestLSPRenameEmptyName(t *testing.T) {
	lsp := newMockLSPProvider()
	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		edits = _ks_lsp.rename("/test/file.go", 100, "")
	`)
	if err == nil {
		t.Error("rename with empty name should error")
	}
}

func TestLSPIsAvailable(t *testing.T) {
	tests := []struct {
		name       string
		available  bool
		wantResult bool
	}{
		{"available", true, true},
		{"not available", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lsp := newMockLSPProvider()
			lsp.isAvailable = tt.available

			L, _ := setupLSPTest(t, lsp)

			err := L.DoString(`
				available = _ks_lsp.is_available("/test/file.go")
			`)
			if err != nil {
				t.Fatalf("DoString error = %v", err)
			}

			available := L.GetGlobal("available")
			if bool(available.(lua.LBool)) != tt.wantResult {
				t.Errorf("is_available = %v, want %v", available, tt.wantResult)
			}
		})
	}
}

func TestLSPIsAvailableNilProvider(t *testing.T) {
	ctx := &Context{LSP: nil}
	mod := NewLSPModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	err := L.DoString(`
		available = _ks_lsp.is_available("/test/file.go")
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	available := L.GetGlobal("available")
	if available != lua.LFalse {
		t.Error("is_available should return false when provider is nil")
	}
}

func TestLSPConstants(t *testing.T) {
	lsp := newMockLSPProvider()
	L, _ := setupLSPTest(t, lsp)

	// Test completion kind constants
	err := L.DoString(`
		assert_kind = _ks_lsp.completion_kind.FUNCTION == 3
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	assertKind := L.GetGlobal("assert_kind")
	if assertKind != lua.LTrue {
		t.Error("completion_kind.FUNCTION should be 3")
	}

	// Test severity constants
	err = L.DoString(`
		assert_sev = _ks_lsp.severity.ERROR == 1
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	assertSev := L.GetGlobal("assert_sev")
	if assertSev != lua.LTrue {
		t.Error("severity.ERROR should be 1")
	}

	// Test action kind constants
	err = L.DoString(`
		assert_action = _ks_lsp.action_kind.QUICKFIX == "quickfix"
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	assertAction := L.GetGlobal("assert_action")
	if assertAction != lua.LTrue {
		t.Error("action_kind.QUICKFIX should be 'quickfix'")
	}
}

func TestLSPErrorHandling(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.err = errors.New("LSP server error")

	L, _ := setupLSPTest(t, lsp)

	// All methods should return nil on error, not crash
	methods := []string{
		`result = _ks_lsp.completions("/test/file.go", 50)`,
		`result = _ks_lsp.diagnostics("/test/file.go")`,
		`result = _ks_lsp.definition("/test/file.go", 50)`,
		`result = _ks_lsp.references("/test/file.go", 50, true)`,
		`result = _ks_lsp.hover("/test/file.go", 50)`,
		`result = _ks_lsp.signature_help("/test/file.go", 50)`,
		`result = _ks_lsp.format("/test/file.go")`,
		`result = _ks_lsp.code_actions("/test/file.go", 10, 50)`,
		`result = _ks_lsp.rename("/test/file.go", 50, "newName")`,
	}

	for _, method := range methods {
		err := L.DoString(method)
		if err != nil {
			t.Errorf("method should not error on LSP failure: %s - %v", method, err)
		}

		result := L.GetGlobal("result")
		if result != lua.LNil {
			t.Errorf("method should return nil on LSP error: %s", method)
		}
	}
}

func TestLSPCleanup(t *testing.T) {
	lsp := newMockLSPProvider()
	_, mod := setupLSPTest(t, lsp)

	// Cleanup should not panic
	mod.Cleanup()

	// After cleanup, L should be nil
	if mod.L != nil {
		t.Error("L should be nil after cleanup")
	}
}

func TestLSPDiagnosticRelatedInfo(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.diagnostics = []Diagnostic{
		{
			Range:    Range{StartLine: 10, StartColumn: 5, EndLine: 10, EndColumn: 20},
			Severity: DiagnosticSeverityError,
			Message:  "undefined: foo",
			RelatedInfo: []DiagnosticRelatedInfo{
				{
					Location: Location{
						Path:  "/test/other.go",
						Range: Range{StartLine: 5, StartColumn: 0, EndLine: 5, EndColumn: 10},
					},
					Message: "defined here",
				},
			},
		},
	}

	L, _ := setupLSPTest(t, lsp)

	err := L.DoString(`
		diags = _ks_lsp.diagnostics()
		related = diags[1].related_info
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	related := L.GetGlobal("related")
	if related == lua.LNil {
		t.Fatal("related_info should not be nil")
	}

	tbl := related.(*lua.LTable)
	if tbl.Len() != 1 {
		t.Errorf("related_info count = %d, want 1", tbl.Len())
	}

	first := tbl.RawGetInt(1).(*lua.LTable)
	msg := L.GetField(first, "message").(lua.LString)
	if msg != "defined here" {
		t.Errorf("related message = %q, want 'defined here'", msg)
	}
}

func TestLSPTableToDiagnostics(t *testing.T) {
	lsp := newMockLSPProvider()
	lsp.codeActions = []CodeAction{{Title: "Test"}}

	L, mod := setupLSPTest(t, lsp)

	// Create a diagnostics table in Lua and pass it to code_actions
	err := L.DoString(`
		diags = {
			{
				message = "test error",
				severity = 1,
				code = "E001",
				source = "test",
				range = {
					start_line = 11,
					start_column = 6,
					end_line = 11,
					end_column = 21
				}
			}
		}
		actions = _ks_lsp.code_actions("/test/file.go", 10, 50, diags)
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	// Verify the tableToRange conversion (1-indexed Lua to 0-indexed Go)
	diagTbl := L.GetGlobal("diags").(*lua.LTable).RawGetInt(1).(*lua.LTable)
	rangeTbl := L.GetField(diagTbl, "range").(*lua.LTable)

	goRange := mod.tableToRange(L, rangeTbl)
	if goRange.StartLine != 10 { // 11 - 1
		t.Errorf("StartLine = %d, want 10", goRange.StartLine)
	}
	if goRange.StartColumn != 5 { // 6 - 1
		t.Errorf("StartColumn = %d, want 5", goRange.StartColumn)
	}
}

func TestLSPNoBufferPath(t *testing.T) {
	lsp := newMockLSPProvider()
	ctx := &Context{
		LSP:    lsp,
		Buffer: &mockBufferProviderForLSP{path: ""}, // Empty path
		Cursor: &mockCursorProviderForLSP{offset: 100, selStart: -1, selEnd: -1},
	}
	mod := NewLSPModule(ctx, "testplugin")

	L := lua.NewState()
	defer L.Close()

	if err := mod.Register(L); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// Without explicit path and empty buffer path, should return nil
	err := L.DoString(`
		items = _ks_lsp.completions()
	`)
	if err != nil {
		t.Fatalf("DoString error = %v", err)
	}

	items := L.GetGlobal("items")
	if items != lua.LNil {
		t.Error("completions should return nil when no path available")
	}
}
