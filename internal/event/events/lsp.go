package events

import "github.com/dshills/keystorm/internal/event/topic"

// LSP event topics.
const (
	// TopicLSPServerInitialized is published when an LSP server is ready.
	TopicLSPServerInitialized topic.Topic = "lsp.server.initialized"

	// TopicLSPServerShutdown is published when an LSP server closes.
	TopicLSPServerShutdown topic.Topic = "lsp.server.shutdown"

	// TopicLSPServerError is published on LSP server errors.
	TopicLSPServerError topic.Topic = "lsp.server.error"

	// TopicLSPServerRestarted is published when an LSP server is restarted.
	TopicLSPServerRestarted topic.Topic = "lsp.server.restarted"

	// TopicLSPDiagnosticsPublished is published when diagnostics are received.
	TopicLSPDiagnosticsPublished topic.Topic = "lsp.diagnostics.published"

	// TopicLSPDiagnosticsCleared is published when diagnostics are cleared.
	TopicLSPDiagnosticsCleared topic.Topic = "lsp.diagnostics.cleared"

	// TopicLSPCompletionAvailable is published when completions are ready.
	TopicLSPCompletionAvailable topic.Topic = "lsp.completion.available"

	// TopicLSPCompletionResolved is published when a completion is resolved.
	TopicLSPCompletionResolved topic.Topic = "lsp.completion.resolved"

	// TopicLSPHoverAvailable is published when hover info is ready.
	TopicLSPHoverAvailable topic.Topic = "lsp.hover.available"

	// TopicLSPSignatureAvailable is published when signature help is ready.
	TopicLSPSignatureAvailable topic.Topic = "lsp.signature.available"

	// TopicLSPDefinitionFound is published when definition location is found.
	TopicLSPDefinitionFound topic.Topic = "lsp.definition.found"

	// TopicLSPReferencesFound is published when references are found.
	TopicLSPReferencesFound topic.Topic = "lsp.references.found"

	// TopicLSPSymbolsFound is published when document symbols are found.
	TopicLSPSymbolsFound topic.Topic = "lsp.symbols.found"

	// TopicLSPSemanticTokensUpdated is published when semantic tokens are available.
	TopicLSPSemanticTokensUpdated topic.Topic = "lsp.semantic.tokens.updated"

	// TopicLSPCodeActionsAvailable is published when code actions are ready.
	TopicLSPCodeActionsAvailable topic.Topic = "lsp.code.actions.available"

	// TopicLSPCodeActionApplied is published when a code action is applied.
	TopicLSPCodeActionApplied topic.Topic = "lsp.code.action.applied"

	// TopicLSPFormatApplied is published when formatting is applied.
	TopicLSPFormatApplied topic.Topic = "lsp.format.applied"

	// TopicLSPRenameApplied is published when a rename is applied.
	TopicLSPRenameApplied topic.Topic = "lsp.rename.applied"

	// TopicLSPProgressStarted is published when an LSP progress starts.
	TopicLSPProgressStarted topic.Topic = "lsp.progress.started"

	// TopicLSPProgressUpdated is published when LSP progress updates.
	TopicLSPProgressUpdated topic.Topic = "lsp.progress.updated"

	// TopicLSPProgressEnded is published when LSP progress ends.
	TopicLSPProgressEnded topic.Topic = "lsp.progress.ended"
)

// DiagnosticSeverity represents the severity of a diagnostic.
type DiagnosticSeverity int

// Diagnostic severities (matching LSP specification).
const (
	DiagnosticSeverityError       DiagnosticSeverity = 1
	DiagnosticSeverityWarning     DiagnosticSeverity = 2
	DiagnosticSeverityInformation DiagnosticSeverity = 3
	DiagnosticSeverityHint        DiagnosticSeverity = 4
)

// CompletionItemKind represents the kind of completion item.
type CompletionItemKind int

// Completion item kinds (matching LSP specification).
const (
	CompletionItemKindText          CompletionItemKind = 1
	CompletionItemKindMethod        CompletionItemKind = 2
	CompletionItemKindFunction      CompletionItemKind = 3
	CompletionItemKindConstructor   CompletionItemKind = 4
	CompletionItemKindField         CompletionItemKind = 5
	CompletionItemKindVariable      CompletionItemKind = 6
	CompletionItemKindClass         CompletionItemKind = 7
	CompletionItemKindInterface     CompletionItemKind = 8
	CompletionItemKindModule        CompletionItemKind = 9
	CompletionItemKindProperty      CompletionItemKind = 10
	CompletionItemKindUnit          CompletionItemKind = 11
	CompletionItemKindValue         CompletionItemKind = 12
	CompletionItemKindEnum          CompletionItemKind = 13
	CompletionItemKindKeyword       CompletionItemKind = 14
	CompletionItemKindSnippet       CompletionItemKind = 15
	CompletionItemKindColor         CompletionItemKind = 16
	CompletionItemKindFile          CompletionItemKind = 17
	CompletionItemKindReference     CompletionItemKind = 18
	CompletionItemKindFolder        CompletionItemKind = 19
	CompletionItemKindEnumMember    CompletionItemKind = 20
	CompletionItemKindConstant      CompletionItemKind = 21
	CompletionItemKindStruct        CompletionItemKind = 22
	CompletionItemKindEvent         CompletionItemKind = 23
	CompletionItemKindOperator      CompletionItemKind = 24
	CompletionItemKindTypeParameter CompletionItemKind = 25
)

// SymbolKind represents the kind of a symbol.
type SymbolKind int

// Symbol kinds (matching LSP specification).
const (
	SymbolKindFile          SymbolKind = 1
	SymbolKindModule        SymbolKind = 2
	SymbolKindNamespace     SymbolKind = 3
	SymbolKindPackage       SymbolKind = 4
	SymbolKindClass         SymbolKind = 5
	SymbolKindMethod        SymbolKind = 6
	SymbolKindProperty      SymbolKind = 7
	SymbolKindField         SymbolKind = 8
	SymbolKindConstructor   SymbolKind = 9
	SymbolKindEnum          SymbolKind = 10
	SymbolKindInterface     SymbolKind = 11
	SymbolKindFunction      SymbolKind = 12
	SymbolKindVariable      SymbolKind = 13
	SymbolKindConstant      SymbolKind = 14
	SymbolKindString        SymbolKind = 15
	SymbolKindNumber        SymbolKind = 16
	SymbolKindBoolean       SymbolKind = 17
	SymbolKindArray         SymbolKind = 18
	SymbolKindObject        SymbolKind = 19
	SymbolKindKey           SymbolKind = 20
	SymbolKindNull          SymbolKind = 21
	SymbolKindEnumMember    SymbolKind = 22
	SymbolKindStruct        SymbolKind = 23
	SymbolKindEvent         SymbolKind = 24
	SymbolKindOperator      SymbolKind = 25
	SymbolKindTypeParameter SymbolKind = 26
)

// Diagnostic represents an LSP diagnostic.
type Diagnostic struct {
	// Range is the range of the diagnostic.
	Range Range

	// Severity indicates the severity level.
	Severity DiagnosticSeverity

	// Code is the diagnostic code.
	Code string

	// Source identifies the source of the diagnostic.
	Source string

	// Message describes the diagnostic.
	Message string

	// RelatedInformation provides additional context.
	RelatedInformation []DiagnosticRelatedInfo

	// Tags provide additional metadata.
	Tags []int
}

// DiagnosticRelatedInfo provides additional context for a diagnostic.
type DiagnosticRelatedInfo struct {
	// Location is where the related information is.
	Location Location

	// Message describes the related information.
	Message string
}

// Location represents a location in a document.
type Location struct {
	// URI is the document URI.
	URI string

	// Range is the range within the document.
	Range Range
}

// CompletionItem represents a completion suggestion.
type CompletionItem struct {
	// Label is the display text.
	Label string

	// Kind categorizes the completion.
	Kind CompletionItemKind

	// Detail provides additional information.
	Detail string

	// Documentation describes the completion.
	Documentation string

	// InsertText is the text to insert.
	InsertText string

	// InsertTextFormat indicates the format (1=PlainText, 2=Snippet).
	InsertTextFormat int

	// FilterText is used for filtering.
	FilterText string

	// SortText is used for sorting.
	SortText string

	// Preselect indicates if this should be preselected.
	Preselect bool

	// TextEdit is the edit to apply.
	TextEdit *TextEdit

	// AdditionalTextEdits are edits to apply after insertion.
	AdditionalTextEdits []TextEdit
}

// TextEdit represents a text edit operation.
type TextEdit struct {
	// Range is the range to replace.
	Range Range

	// NewText is the replacement text.
	NewText string
}

// DocumentSymbol represents a symbol in a document.
type DocumentSymbol struct {
	// Name is the symbol name.
	Name string

	// Detail provides additional detail.
	Detail string

	// Kind is the symbol kind.
	Kind SymbolKind

	// Range is the full range of the symbol.
	Range Range

	// SelectionRange is the range to select when navigating.
	SelectionRange Range

	// Children are nested symbols.
	Children []DocumentSymbol
}

// SemanticToken represents a semantic token.
type SemanticToken struct {
	// Line is the line number (0-based).
	Line int

	// StartChar is the start character (0-based).
	StartChar int

	// Length is the token length.
	Length int

	// TokenType is the semantic token type.
	TokenType int

	// Modifiers are the semantic token modifiers.
	Modifiers int
}

// CodeAction represents a code action.
type CodeAction struct {
	// Title is the display title.
	Title string

	// Kind categorizes the action (e.g., "quickfix", "refactor").
	Kind string

	// Diagnostics are the diagnostics this action addresses.
	Diagnostics []Diagnostic

	// IsPreferred indicates if this is the preferred action.
	IsPreferred bool

	// Edit is the workspace edit to apply.
	Edit *WorkspaceEdit
}

// WorkspaceEdit represents edits to multiple documents.
type WorkspaceEdit struct {
	// Changes maps document URIs to text edits.
	Changes map[string][]TextEdit
}

// LSPServerInitialized is published when an LSP server is ready.
type LSPServerInitialized struct {
	// LanguageID identifies the language.
	LanguageID string

	// ServerName is the server name.
	ServerName string

	// ServerVersion is the server version.
	ServerVersion string

	// Capabilities describes server capabilities.
	Capabilities map[string]any
}

// LSPServerShutdown is published when an LSP server closes.
type LSPServerShutdown struct {
	// LanguageID identifies the language.
	LanguageID string

	// Reason explains why the server shut down.
	Reason string

	// WasGraceful indicates if shutdown was graceful.
	WasGraceful bool
}

// LSPServerError is published on LSP server errors.
type LSPServerError struct {
	// LanguageID identifies the language.
	LanguageID string

	// ErrorMessage describes the error.
	ErrorMessage string

	// Code is the error code.
	Code int

	// IsFatal indicates if the error is fatal.
	IsFatal bool
}

// LSPServerRestarted is published when an LSP server is restarted.
type LSPServerRestarted struct {
	// LanguageID identifies the language.
	LanguageID string

	// Reason explains why the server was restarted.
	Reason string

	// RestartCount is the number of restarts.
	RestartCount int
}

// LSPDiagnosticsPublished is published when diagnostics are received.
type LSPDiagnosticsPublished struct {
	// URI is the document URI.
	URI string

	// LanguageID identifies the language.
	LanguageID string

	// Diagnostics are the published diagnostics.
	Diagnostics []Diagnostic

	// Version is the document version.
	Version int
}

// LSPDiagnosticsCleared is published when diagnostics are cleared.
type LSPDiagnosticsCleared struct {
	// URI is the document URI.
	URI string

	// LanguageID identifies the language.
	LanguageID string
}

// LSPCompletionAvailable is published when completions are ready.
type LSPCompletionAvailable struct {
	// URI is the document URI.
	URI string

	// Position is the completion position.
	Position Position

	// Items are the completion items.
	Items []CompletionItem

	// IsIncomplete indicates if the list is incomplete.
	IsIncomplete bool

	// TriggerCharacter is the character that triggered completion.
	TriggerCharacter string
}

// LSPCompletionResolved is published when a completion is resolved.
type LSPCompletionResolved struct {
	// URI is the document URI.
	URI string

	// Item is the resolved completion item.
	Item CompletionItem
}

// LSPHoverAvailable is published when hover info is ready.
type LSPHoverAvailable struct {
	// URI is the document URI.
	URI string

	// Position is the hover position.
	Position Position

	// Contents is the hover content (markdown).
	Contents string

	// Range is the range the hover applies to.
	Range *Range
}

// LSPSignatureAvailable is published when signature help is ready.
type LSPSignatureAvailable struct {
	// URI is the document URI.
	URI string

	// Position is the cursor position.
	Position Position

	// Signatures are the available signatures.
	Signatures []SignatureInfo

	// ActiveSignature is the active signature index.
	ActiveSignature int

	// ActiveParameter is the active parameter index.
	ActiveParameter int
}

// SignatureInfo represents a function signature.
type SignatureInfo struct {
	// Label is the signature label.
	Label string

	// Documentation describes the signature.
	Documentation string

	// Parameters are the parameter descriptions.
	Parameters []ParameterInfo
}

// ParameterInfo represents a parameter in a signature.
type ParameterInfo struct {
	// Label is the parameter label.
	Label string

	// Documentation describes the parameter.
	Documentation string
}

// LSPDefinitionFound is published when definition location is found.
type LSPDefinitionFound struct {
	// URI is the document URI where lookup was performed.
	URI string

	// Position is the position of the symbol.
	Position Position

	// Locations are the definition locations.
	Locations []Location
}

// LSPReferencesFound is published when references are found.
type LSPReferencesFound struct {
	// URI is the document URI.
	URI string

	// Position is the position of the symbol.
	Position Position

	// Locations are the reference locations.
	Locations []Location

	// IncludeDeclaration indicates if declaration was included.
	IncludeDeclaration bool
}

// LSPSymbolsFound is published when document symbols are found.
type LSPSymbolsFound struct {
	// URI is the document URI.
	URI string

	// Symbols are the document symbols.
	Symbols []DocumentSymbol
}

// LSPSemanticTokensUpdated is published when semantic tokens are available.
type LSPSemanticTokensUpdated struct {
	// URI is the document URI.
	URI string

	// Tokens are the semantic tokens.
	Tokens []SemanticToken

	// ResultID identifies this result for incremental updates.
	ResultID string
}

// LSPCodeActionsAvailable is published when code actions are ready.
type LSPCodeActionsAvailable struct {
	// URI is the document URI.
	URI string

	// Range is the range for which actions were requested.
	Range Range

	// Actions are the available code actions.
	Actions []CodeAction
}

// LSPCodeActionApplied is published when a code action is applied.
type LSPCodeActionApplied struct {
	// URI is the document URI.
	URI string

	// ActionTitle is the title of the applied action.
	ActionTitle string

	// ActionKind is the kind of action applied.
	ActionKind string

	// FilesModified is the number of files modified.
	FilesModified int
}

// LSPFormatApplied is published when formatting is applied.
type LSPFormatApplied struct {
	// URI is the document URI.
	URI string

	// EditsApplied is the number of edits applied.
	EditsApplied int

	// Range is the formatted range, nil for full document.
	Range *Range
}

// LSPRenameApplied is published when a rename is applied.
type LSPRenameApplied struct {
	// OldName is the original name.
	OldName string

	// NewName is the new name.
	NewName string

	// FilesModified is the number of files modified.
	FilesModified int

	// OccurrencesRenamed is the number of occurrences renamed.
	OccurrencesRenamed int
}

// LSPProgressStarted is published when an LSP progress starts.
type LSPProgressStarted struct {
	// Token identifies the progress.
	Token string

	// Title is the progress title.
	Title string

	// Message is the initial message.
	Message string

	// Cancellable indicates if progress can be cancelled.
	Cancellable bool
}

// LSPProgressUpdated is published when LSP progress updates.
type LSPProgressUpdated struct {
	// Token identifies the progress.
	Token string

	// Message is the current message.
	Message string

	// Percentage is the completion percentage (0-100).
	Percentage int
}

// LSPProgressEnded is published when LSP progress ends.
type LSPProgressEnded struct {
	// Token identifies the progress.
	Token string

	// Message is the final message.
	Message string
}
