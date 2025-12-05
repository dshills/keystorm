package api

import (
	"sync"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// LSPProvider defines the interface for LSP operations.
//
// IMPORTANT: Thread Safety Requirement
// The LSPProvider implementation MUST handle concurrent access safely.
// Methods may be called from the plugin's goroutine which owns the Lua state.
// If LSP operations are asynchronous, the provider should ensure callbacks
// are invoked on the appropriate goroutine.
type LSPProvider interface {
	// Completions returns completion items at the given position.
	// Position is byte offset in the buffer.
	// Returns a list of completion items or an error.
	Completions(bufferPath string, offset int) ([]CompletionItem, error)

	// Diagnostics returns diagnostics for the given file.
	// Returns a list of diagnostic items or an error.
	Diagnostics(bufferPath string) ([]Diagnostic, error)

	// Definition returns the definition location for the symbol at the given position.
	// Returns the location or nil if no definition found.
	Definition(bufferPath string, offset int) (*Location, error)

	// References returns all references to the symbol at the given position.
	// includeDeclaration controls whether to include the declaration itself.
	References(bufferPath string, offset int, includeDeclaration bool) ([]Location, error)

	// Hover returns hover information for the symbol at the given position.
	// Returns hover content or nil if no hover available.
	Hover(bufferPath string, offset int) (*HoverInfo, error)

	// SignatureHelp returns signature help for the function at the given position.
	SignatureHelp(bufferPath string, offset int) (*SignatureInfo, error)

	// Format formats the document or selection.
	// If startOffset and endOffset are -1, formats the entire document.
	Format(bufferPath string, startOffset, endOffset int) ([]TextEdit, error)

	// CodeActions returns available code actions at the given range.
	CodeActions(bufferPath string, startOffset, endOffset int, diagnostics []Diagnostic) ([]CodeAction, error)

	// Rename renames the symbol at the given position.
	Rename(bufferPath string, offset int, newName string) ([]TextEdit, error)

	// IsAvailable returns true if an LSP server is available for the given file.
	IsAvailable(bufferPath string) bool
}

// CompletionItem represents a completion suggestion.
type CompletionItem struct {
	Label         string
	Kind          CompletionKind
	Detail        string
	Documentation string
	InsertText    string
	SortText      string
}

// CompletionKind represents the type of completion item.
type CompletionKind int

const (
	CompletionKindText CompletionKind = iota + 1
	CompletionKindMethod
	CompletionKindFunction
	CompletionKindConstructor
	CompletionKindField
	CompletionKindVariable
	CompletionKindClass
	CompletionKindInterface
	CompletionKindModule
	CompletionKindProperty
	CompletionKindUnit
	CompletionKindValue
	CompletionKindEnum
	CompletionKindKeyword
	CompletionKindSnippet
	CompletionKindColor
	CompletionKindFile
	CompletionKindReference
	CompletionKindFolder
	CompletionKindEnumMember
	CompletionKindConstant
	CompletionKindStruct
	CompletionKindEvent
	CompletionKindOperator
	CompletionKindTypeParameter
)

// Diagnostic represents a diagnostic message (error, warning, etc.).
type Diagnostic struct {
	Range       Range
	Severity    DiagnosticSeverity
	Code        string
	Source      string
	Message     string
	RelatedInfo []DiagnosticRelatedInfo
}

// DiagnosticSeverity represents the severity of a diagnostic.
type DiagnosticSeverity int

const (
	DiagnosticSeverityError DiagnosticSeverity = iota + 1
	DiagnosticSeverityWarning
	DiagnosticSeverityInformation
	DiagnosticSeverityHint
)

// DiagnosticRelatedInfo provides additional information about a diagnostic.
type DiagnosticRelatedInfo struct {
	Location Location
	Message  string
}

// Location represents a location in a document.
type Location struct {
	Path  string
	Range Range
}

// Range represents a range in a document.
type Range struct {
	StartLine   int
	StartColumn int
	EndLine     int
	EndColumn   int
}

// HoverInfo represents hover information.
type HoverInfo struct {
	Contents string
	Range    *Range
}

// SignatureInfo represents signature help information.
type SignatureInfo struct {
	Signatures      []SignatureInformation
	ActiveSignature int
	ActiveParameter int
}

// SignatureInformation represents a function signature.
type SignatureInformation struct {
	Label         string
	Documentation string
	Parameters    []ParameterInfo
}

// ParameterInfo represents a parameter in a signature.
type ParameterInfo struct {
	Label         string
	Documentation string
}

// TextEdit represents a text edit operation.
type TextEdit struct {
	Range   Range
	NewText string
}

// CodeAction represents a code action (quick fix, refactoring, etc.).
type CodeAction struct {
	Title       string
	Kind        CodeActionKind
	Diagnostics []Diagnostic
	Edits       []TextEdit
	Command     string
}

// CodeActionKind represents the type of code action.
type CodeActionKind string

const (
	CodeActionKindQuickFix        CodeActionKind = "quickfix"
	CodeActionKindRefactor        CodeActionKind = "refactor"
	CodeActionKindRefactorExtract CodeActionKind = "refactor.extract"
	CodeActionKindRefactorInline  CodeActionKind = "refactor.inline"
	CodeActionKindRefactorRewrite CodeActionKind = "refactor.rewrite"
	CodeActionKindSource          CodeActionKind = "source"
	CodeActionKindSourceOrganize  CodeActionKind = "source.organizeImports"
	CodeActionKindSourceFixAll    CodeActionKind = "source.fixAll"
)

// LSPModule implements the ks.lsp API module.
type LSPModule struct {
	ctx        *Context
	pluginName string
	L          *lua.LState

	mu sync.Mutex
}

// NewLSPModule creates a new LSP module.
func NewLSPModule(ctx *Context, pluginName string) *LSPModule {
	return &LSPModule{
		ctx:        ctx,
		pluginName: pluginName,
	}
}

// Name returns the module name.
func (m *LSPModule) Name() string {
	return "lsp"
}

// RequiredCapability returns the capability required for this module.
func (m *LSPModule) RequiredCapability() security.Capability {
	return security.CapabilityLSP
}

// Register registers the module into the Lua state.
func (m *LSPModule) Register(L *lua.LState) error {
	m.L = L

	mod := L.NewTable()

	// Register LSP functions
	L.SetField(mod, "completions", L.NewFunction(m.completions))
	L.SetField(mod, "diagnostics", L.NewFunction(m.diagnostics))
	L.SetField(mod, "definition", L.NewFunction(m.definition))
	L.SetField(mod, "references", L.NewFunction(m.references))
	L.SetField(mod, "hover", L.NewFunction(m.hover))
	L.SetField(mod, "signature_help", L.NewFunction(m.signatureHelp))
	L.SetField(mod, "format", L.NewFunction(m.format))
	L.SetField(mod, "code_actions", L.NewFunction(m.codeActions))
	L.SetField(mod, "rename", L.NewFunction(m.rename))
	L.SetField(mod, "is_available", L.NewFunction(m.isAvailable))

	// Add completion kind constants
	kinds := L.NewTable()
	L.SetField(kinds, "TEXT", lua.LNumber(CompletionKindText))
	L.SetField(kinds, "METHOD", lua.LNumber(CompletionKindMethod))
	L.SetField(kinds, "FUNCTION", lua.LNumber(CompletionKindFunction))
	L.SetField(kinds, "CONSTRUCTOR", lua.LNumber(CompletionKindConstructor))
	L.SetField(kinds, "FIELD", lua.LNumber(CompletionKindField))
	L.SetField(kinds, "VARIABLE", lua.LNumber(CompletionKindVariable))
	L.SetField(kinds, "CLASS", lua.LNumber(CompletionKindClass))
	L.SetField(kinds, "INTERFACE", lua.LNumber(CompletionKindInterface))
	L.SetField(kinds, "MODULE", lua.LNumber(CompletionKindModule))
	L.SetField(kinds, "PROPERTY", lua.LNumber(CompletionKindProperty))
	L.SetField(kinds, "KEYWORD", lua.LNumber(CompletionKindKeyword))
	L.SetField(kinds, "SNIPPET", lua.LNumber(CompletionKindSnippet))
	L.SetField(kinds, "CONSTANT", lua.LNumber(CompletionKindConstant))
	L.SetField(kinds, "STRUCT", lua.LNumber(CompletionKindStruct))
	L.SetField(mod, "completion_kind", kinds)

	// Add diagnostic severity constants
	severity := L.NewTable()
	L.SetField(severity, "ERROR", lua.LNumber(DiagnosticSeverityError))
	L.SetField(severity, "WARNING", lua.LNumber(DiagnosticSeverityWarning))
	L.SetField(severity, "INFORMATION", lua.LNumber(DiagnosticSeverityInformation))
	L.SetField(severity, "HINT", lua.LNumber(DiagnosticSeverityHint))
	L.SetField(mod, "severity", severity)

	// Add code action kind constants
	actionKinds := L.NewTable()
	L.SetField(actionKinds, "QUICKFIX", lua.LString(CodeActionKindQuickFix))
	L.SetField(actionKinds, "REFACTOR", lua.LString(CodeActionKindRefactor))
	L.SetField(actionKinds, "REFACTOR_EXTRACT", lua.LString(CodeActionKindRefactorExtract))
	L.SetField(actionKinds, "REFACTOR_INLINE", lua.LString(CodeActionKindRefactorInline))
	L.SetField(actionKinds, "SOURCE", lua.LString(CodeActionKindSource))
	L.SetField(actionKinds, "SOURCE_ORGANIZE", lua.LString(CodeActionKindSourceOrganize))
	L.SetField(actionKinds, "SOURCE_FIX_ALL", lua.LString(CodeActionKindSourceFixAll))
	L.SetField(mod, "action_kind", actionKinds)

	L.SetGlobal("_ks_lsp", mod)
	return nil
}

// Cleanup releases resources.
func (m *LSPModule) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L = nil
}

// completions(path?, offset?) -> {items} or nil
// Returns completion items at the current or given position.
func (m *LSPModule) completions(L *lua.LState) int {
	path := L.OptString(1, "")
	offset := L.OptInt(2, -1)

	if m.ctx.LSP == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Use current buffer path if not specified
	if path == "" {
		if m.ctx.Buffer != nil {
			path = m.ctx.Buffer.Path()
		}
		if path == "" {
			L.Push(lua.LNil)
			return 1
		}
	}

	// Use current cursor offset if not specified
	if offset < 0 {
		if m.ctx.Cursor != nil {
			offset = m.ctx.Cursor.Get()
		} else {
			offset = 0
		}
	}

	items, err := m.ctx.LSP.Completions(path, offset)
	if err != nil {
		L.Push(lua.LNil)
		return 1
	}

	tbl := L.NewTable()
	for i, item := range items {
		itemTbl := L.NewTable()
		L.SetField(itemTbl, "label", lua.LString(item.Label))
		L.SetField(itemTbl, "kind", lua.LNumber(item.Kind))
		L.SetField(itemTbl, "detail", lua.LString(item.Detail))
		L.SetField(itemTbl, "documentation", lua.LString(item.Documentation))
		L.SetField(itemTbl, "insert_text", lua.LString(item.InsertText))
		L.SetField(itemTbl, "sort_text", lua.LString(item.SortText))
		tbl.RawSetInt(i+1, itemTbl)
	}

	L.Push(tbl)
	return 1
}

// diagnostics(path?) -> {diagnostics} or nil
// Returns diagnostics for the current or given file.
func (m *LSPModule) diagnostics(L *lua.LState) int {
	path := L.OptString(1, "")

	if m.ctx.LSP == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Use current buffer path if not specified
	if path == "" {
		if m.ctx.Buffer != nil {
			path = m.ctx.Buffer.Path()
		}
		if path == "" {
			L.Push(lua.LNil)
			return 1
		}
	}

	diags, err := m.ctx.LSP.Diagnostics(path)
	if err != nil {
		L.Push(lua.LNil)
		return 1
	}

	tbl := L.NewTable()
	for i, diag := range diags {
		diagTbl := m.diagnosticToTable(L, diag)
		tbl.RawSetInt(i+1, diagTbl)
	}

	L.Push(tbl)
	return 1
}

// definition(path?, offset?) -> location or nil
// Returns the definition location for the symbol at the given position.
func (m *LSPModule) definition(L *lua.LState) int {
	path := L.OptString(1, "")
	offset := L.OptInt(2, -1)

	if m.ctx.LSP == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Use current buffer path if not specified
	if path == "" {
		if m.ctx.Buffer != nil {
			path = m.ctx.Buffer.Path()
		}
		if path == "" {
			L.Push(lua.LNil)
			return 1
		}
	}

	// Use current cursor offset if not specified
	if offset < 0 {
		if m.ctx.Cursor != nil {
			offset = m.ctx.Cursor.Get()
		} else {
			offset = 0
		}
	}

	loc, err := m.ctx.LSP.Definition(path, offset)
	if err != nil || loc == nil {
		L.Push(lua.LNil)
		return 1
	}

	L.Push(m.locationToTable(L, *loc))
	return 1
}

// references(path?, offset?, include_declaration?) -> {locations} or nil
// Returns all references to the symbol at the given position.
func (m *LSPModule) references(L *lua.LState) int {
	path := L.OptString(1, "")
	offset := L.OptInt(2, -1)
	includeDecl := L.OptBool(3, true)

	if m.ctx.LSP == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Use current buffer path if not specified
	if path == "" {
		if m.ctx.Buffer != nil {
			path = m.ctx.Buffer.Path()
		}
		if path == "" {
			L.Push(lua.LNil)
			return 1
		}
	}

	// Use current cursor offset if not specified
	if offset < 0 {
		if m.ctx.Cursor != nil {
			offset = m.ctx.Cursor.Get()
		} else {
			offset = 0
		}
	}

	locs, err := m.ctx.LSP.References(path, offset, includeDecl)
	if err != nil {
		L.Push(lua.LNil)
		return 1
	}

	tbl := L.NewTable()
	for i, loc := range locs {
		tbl.RawSetInt(i+1, m.locationToTable(L, loc))
	}

	L.Push(tbl)
	return 1
}

// hover(path?, offset?) -> hover_info or nil
// Returns hover information for the symbol at the given position.
func (m *LSPModule) hover(L *lua.LState) int {
	path := L.OptString(1, "")
	offset := L.OptInt(2, -1)

	if m.ctx.LSP == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Use current buffer path if not specified
	if path == "" {
		if m.ctx.Buffer != nil {
			path = m.ctx.Buffer.Path()
		}
		if path == "" {
			L.Push(lua.LNil)
			return 1
		}
	}

	// Use current cursor offset if not specified
	if offset < 0 {
		if m.ctx.Cursor != nil {
			offset = m.ctx.Cursor.Get()
		} else {
			offset = 0
		}
	}

	info, err := m.ctx.LSP.Hover(path, offset)
	if err != nil || info == nil {
		L.Push(lua.LNil)
		return 1
	}

	tbl := L.NewTable()
	L.SetField(tbl, "contents", lua.LString(info.Contents))
	if info.Range != nil {
		L.SetField(tbl, "range", m.rangeToTable(L, *info.Range))
	}

	L.Push(tbl)
	return 1
}

// signature_help(path?, offset?) -> signature_info or nil
// Returns signature help for the function at the given position.
func (m *LSPModule) signatureHelp(L *lua.LState) int {
	path := L.OptString(1, "")
	offset := L.OptInt(2, -1)

	if m.ctx.LSP == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Use current buffer path if not specified
	if path == "" {
		if m.ctx.Buffer != nil {
			path = m.ctx.Buffer.Path()
		}
		if path == "" {
			L.Push(lua.LNil)
			return 1
		}
	}

	// Use current cursor offset if not specified
	if offset < 0 {
		if m.ctx.Cursor != nil {
			offset = m.ctx.Cursor.Get()
		} else {
			offset = 0
		}
	}

	info, err := m.ctx.LSP.SignatureHelp(path, offset)
	if err != nil || info == nil {
		L.Push(lua.LNil)
		return 1
	}

	tbl := L.NewTable()
	L.SetField(tbl, "active_signature", lua.LNumber(info.ActiveSignature+1)) // Convert to 1-indexed
	L.SetField(tbl, "active_parameter", lua.LNumber(info.ActiveParameter+1)) // Convert to 1-indexed

	sigsTbl := L.NewTable()
	for i, sig := range info.Signatures {
		sigTbl := L.NewTable()
		L.SetField(sigTbl, "label", lua.LString(sig.Label))
		L.SetField(sigTbl, "documentation", lua.LString(sig.Documentation))

		paramsTbl := L.NewTable()
		for j, param := range sig.Parameters {
			paramTbl := L.NewTable()
			L.SetField(paramTbl, "label", lua.LString(param.Label))
			L.SetField(paramTbl, "documentation", lua.LString(param.Documentation))
			paramsTbl.RawSetInt(j+1, paramTbl)
		}
		L.SetField(sigTbl, "parameters", paramsTbl)
		sigsTbl.RawSetInt(i+1, sigTbl)
	}
	L.SetField(tbl, "signatures", sigsTbl)

	L.Push(tbl)
	return 1
}

// format(path?, start_offset?, end_offset?) -> {edits} or nil
// Formats the document or selection.
func (m *LSPModule) format(L *lua.LState) int {
	path := L.OptString(1, "")
	startOffset := L.OptInt(2, -1)
	endOffset := L.OptInt(3, -1)

	if m.ctx.LSP == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Use current buffer path if not specified
	if path == "" {
		if m.ctx.Buffer != nil {
			path = m.ctx.Buffer.Path()
		}
		if path == "" {
			L.Push(lua.LNil)
			return 1
		}
	}

	edits, err := m.ctx.LSP.Format(path, startOffset, endOffset)
	if err != nil {
		L.Push(lua.LNil)
		return 1
	}

	tbl := L.NewTable()
	for i, edit := range edits {
		tbl.RawSetInt(i+1, m.textEditToTable(L, edit))
	}

	L.Push(tbl)
	return 1
}

// code_actions(path?, start_offset?, end_offset?, diagnostics?) -> {actions} or nil
// Returns available code actions at the given range.
func (m *LSPModule) codeActions(L *lua.LState) int {
	path := L.OptString(1, "")
	startOffset := L.OptInt(2, -1)
	endOffset := L.OptInt(3, -1)

	if m.ctx.LSP == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Use current buffer path if not specified
	if path == "" {
		if m.ctx.Buffer != nil {
			path = m.ctx.Buffer.Path()
		}
		if path == "" {
			L.Push(lua.LNil)
			return 1
		}
	}

	// Use current selection if offsets not specified
	if startOffset < 0 || endOffset < 0 {
		if m.ctx.Cursor != nil {
			selStart, selEnd := m.ctx.Cursor.Selection()
			if selStart >= 0 && selEnd >= 0 {
				startOffset = selStart
				endOffset = selEnd
			} else {
				startOffset = m.ctx.Cursor.Get()
				endOffset = startOffset
			}
		} else {
			startOffset = 0
			endOffset = 0
		}
	}

	// Parse diagnostics from optional fourth argument
	var diags []Diagnostic
	if L.GetTop() >= 4 {
		diagsTable := L.OptTable(4, nil)
		if diagsTable != nil {
			diags = m.tableToDiagnostics(L, diagsTable)
		}
	}

	actions, err := m.ctx.LSP.CodeActions(path, startOffset, endOffset, diags)
	if err != nil {
		L.Push(lua.LNil)
		return 1
	}

	tbl := L.NewTable()
	for i, action := range actions {
		actionTbl := L.NewTable()
		L.SetField(actionTbl, "title", lua.LString(action.Title))
		L.SetField(actionTbl, "kind", lua.LString(action.Kind))
		L.SetField(actionTbl, "command", lua.LString(action.Command))

		editsTbl := L.NewTable()
		for j, edit := range action.Edits {
			editsTbl.RawSetInt(j+1, m.textEditToTable(L, edit))
		}
		L.SetField(actionTbl, "edits", editsTbl)

		tbl.RawSetInt(i+1, actionTbl)
	}

	L.Push(tbl)
	return 1
}

// rename(path?, offset?, new_name) -> {edits} or nil
// Renames the symbol at the given position.
func (m *LSPModule) rename(L *lua.LState) int {
	path := L.OptString(1, "")
	offset := L.OptInt(2, -1)
	newName := L.CheckString(3)

	if newName == "" {
		L.ArgError(3, "new name cannot be empty")
		return 0
	}

	if m.ctx.LSP == nil {
		L.Push(lua.LNil)
		return 1
	}

	// Use current buffer path if not specified
	if path == "" {
		if m.ctx.Buffer != nil {
			path = m.ctx.Buffer.Path()
		}
		if path == "" {
			L.Push(lua.LNil)
			return 1
		}
	}

	// Use current cursor offset if not specified
	if offset < 0 {
		if m.ctx.Cursor != nil {
			offset = m.ctx.Cursor.Get()
		} else {
			offset = 0
		}
	}

	edits, err := m.ctx.LSP.Rename(path, offset, newName)
	if err != nil {
		L.Push(lua.LNil)
		return 1
	}

	tbl := L.NewTable()
	for i, edit := range edits {
		tbl.RawSetInt(i+1, m.textEditToTable(L, edit))
	}

	L.Push(tbl)
	return 1
}

// is_available(path?) -> bool
// Returns true if an LSP server is available for the given file.
func (m *LSPModule) isAvailable(L *lua.LState) int {
	path := L.OptString(1, "")

	if m.ctx.LSP == nil {
		L.Push(lua.LFalse)
		return 1
	}

	// Use current buffer path if not specified
	if path == "" {
		if m.ctx.Buffer != nil {
			path = m.ctx.Buffer.Path()
		}
		if path == "" {
			L.Push(lua.LFalse)
			return 1
		}
	}

	L.Push(lua.LBool(m.ctx.LSP.IsAvailable(path)))
	return 1
}

// Helper functions for converting Go types to Lua tables

func (m *LSPModule) rangeToTable(L *lua.LState, r Range) *lua.LTable {
	tbl := L.NewTable()
	// Use 1-indexed lines and columns for Lua
	L.SetField(tbl, "start_line", lua.LNumber(r.StartLine+1))
	L.SetField(tbl, "start_column", lua.LNumber(r.StartColumn+1))
	L.SetField(tbl, "end_line", lua.LNumber(r.EndLine+1))
	L.SetField(tbl, "end_column", lua.LNumber(r.EndColumn+1))
	return tbl
}

func (m *LSPModule) locationToTable(L *lua.LState, loc Location) *lua.LTable {
	tbl := L.NewTable()
	L.SetField(tbl, "path", lua.LString(loc.Path))
	L.SetField(tbl, "range", m.rangeToTable(L, loc.Range))
	return tbl
}

func (m *LSPModule) diagnosticToTable(L *lua.LState, diag Diagnostic) *lua.LTable {
	tbl := L.NewTable()
	L.SetField(tbl, "range", m.rangeToTable(L, diag.Range))
	L.SetField(tbl, "severity", lua.LNumber(diag.Severity))
	L.SetField(tbl, "code", lua.LString(diag.Code))
	L.SetField(tbl, "source", lua.LString(diag.Source))
	L.SetField(tbl, "message", lua.LString(diag.Message))

	if len(diag.RelatedInfo) > 0 {
		relatedTbl := L.NewTable()
		for i, info := range diag.RelatedInfo {
			infoTbl := L.NewTable()
			L.SetField(infoTbl, "location", m.locationToTable(L, info.Location))
			L.SetField(infoTbl, "message", lua.LString(info.Message))
			relatedTbl.RawSetInt(i+1, infoTbl)
		}
		L.SetField(tbl, "related_info", relatedTbl)
	}

	return tbl
}

func (m *LSPModule) textEditToTable(L *lua.LState, edit TextEdit) *lua.LTable {
	tbl := L.NewTable()
	L.SetField(tbl, "range", m.rangeToTable(L, edit.Range))
	L.SetField(tbl, "new_text", lua.LString(edit.NewText))
	return tbl
}

func (m *LSPModule) tableToDiagnostics(L *lua.LState, tbl *lua.LTable) []Diagnostic {
	var diags []Diagnostic
	tbl.ForEach(func(_, value lua.LValue) {
		if diagTbl, ok := value.(*lua.LTable); ok {
			diag := Diagnostic{
				Message: getTableString(L, diagTbl, "message"),
				Code:    getTableString(L, diagTbl, "code"),
				Source:  getTableString(L, diagTbl, "source"),
			}

			if sevVal := L.GetField(diagTbl, "severity"); sevVal != lua.LNil {
				if sev, ok := sevVal.(lua.LNumber); ok {
					diag.Severity = DiagnosticSeverity(sev)
				}
			}

			if rangeVal := L.GetField(diagTbl, "range"); rangeVal != lua.LNil {
				if rangeTbl, ok := rangeVal.(*lua.LTable); ok {
					diag.Range = m.tableToRange(L, rangeTbl)
				}
			}

			diags = append(diags, diag)
		}
	})
	return diags
}

func (m *LSPModule) tableToRange(L *lua.LState, tbl *lua.LTable) Range {
	// Convert from 1-indexed Lua to 0-indexed Go
	return Range{
		StartLine:   int(getTableNumber(L, tbl, "start_line")) - 1,
		StartColumn: int(getTableNumber(L, tbl, "start_column")) - 1,
		EndLine:     int(getTableNumber(L, tbl, "end_line")) - 1,
		EndColumn:   int(getTableNumber(L, tbl, "end_column")) - 1,
	}
}

// Note: getTableString and getTableNumber are defined in keymap.go and ui.go respectively
