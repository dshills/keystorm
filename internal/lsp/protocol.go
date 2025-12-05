package lsp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// DocumentURI represents a URI as used in LSP.
// It is typically a file:// URI.
type DocumentURI string

// Position in a text document expressed as zero-based line and character offset.
// Character offset is measured in UTF-16 code units per the LSP specification.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range in a text document expressed as start and end positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location inside a resource.
type Location struct {
	URI   DocumentURI `json:"uri"`
	Range Range       `json:"range"`
}

// TextDocumentIdentifier identifies a text document.
type TextDocumentIdentifier struct {
	URI DocumentURI `json:"uri"`
}

// VersionedTextDocumentIdentifier identifies a specific version of a text document.
type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier
	Version int `json:"version"`
}

// TextDocumentItem is an item to transfer a text document from the client to the server.
type TextDocumentItem struct {
	URI        DocumentURI `json:"uri"`
	LanguageID string      `json:"languageId"`
	Version    int         `json:"version"`
	Text       string      `json:"text"`
}

// TextDocumentPositionParams is a parameter literal used in requests to pass
// a text document and a position inside that document.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// TextEdit represents a textual edit applicable to a text document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// TextDocumentContentChangeEvent describes a content change event.
type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength int    `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

// MarkupContent represents human readable text.
type MarkupContent struct {
	Kind  MarkupKind `json:"kind"`
	Value string     `json:"value"`
}

// MarkupKind describes the content type.
type MarkupKind string

const (
	MarkupKindPlainText MarkupKind = "plaintext"
	MarkupKindMarkdown  MarkupKind = "markdown"
)

// Command represents a reference to a command.
type Command struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

// WorkspaceFolder represents a workspace folder.
type WorkspaceFolder struct {
	URI  DocumentURI `json:"uri"`
	Name string      `json:"name"`
}

// WorkspaceEdit represents changes to many resources managed in the workspace.
type WorkspaceEdit struct {
	Changes         map[DocumentURI][]TextEdit `json:"changes,omitempty"`
	DocumentChanges []any                      `json:"documentChanges,omitempty"`
}

// --- Initialize ---

// InitializeParams are the parameters sent in an initialize request.
type InitializeParams struct {
	ProcessID             int                `json:"processId"`
	RootURI               DocumentURI        `json:"rootUri,omitempty"`
	RootPath              string             `json:"rootPath,omitempty"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	InitializationOptions any                `json:"initializationOptions,omitempty"`
	WorkspaceFolders      []WorkspaceFolder  `json:"workspaceFolders,omitempty"`
	Trace                 string             `json:"trace,omitempty"`
}

// InitializeResult is the result of the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities    `json:"capabilities"`
	ServerInfo   *InitializeServerInfo `json:"serverInfo,omitempty"`
}

// InitializeServerInfo contains information about the language server from initialization.
type InitializeServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// InitializedParams are the parameters sent in an initialized notification.
type InitializedParams struct{}

// --- Capabilities ---

// ClientCapabilities define capabilities the editor / tool provides on the client side.
type ClientCapabilities struct {
	Workspace    *WorkspaceClientCapabilities    `json:"workspace,omitempty"`
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Window       *WindowClientCapabilities       `json:"window,omitempty"`
	General      *GeneralClientCapabilities      `json:"general,omitempty"`
}

// WorkspaceClientCapabilities define capabilities the editor provides on the workspace.
type WorkspaceClientCapabilities struct {
	ApplyEdit              bool                                `json:"applyEdit,omitempty"`
	WorkspaceEdit          *WorkspaceEditClientCapabilities    `json:"workspaceEdit,omitempty"`
	DidChangeConfiguration *DidChangeConfigurationCapabilities `json:"didChangeConfiguration,omitempty"`
	DidChangeWatchedFiles  *DidChangeWatchedFilesCapabilities  `json:"didChangeWatchedFiles,omitempty"`
	Symbol                 *WorkspaceSymbolClientCapabilities  `json:"symbol,omitempty"`
	WorkspaceFolders       bool                                `json:"workspaceFolders,omitempty"`
	Configuration          bool                                `json:"configuration,omitempty"`
}

// WorkspaceEditClientCapabilities define capabilities for workspace edits.
type WorkspaceEditClientCapabilities struct {
	DocumentChanges bool `json:"documentChanges,omitempty"`
}

// DidChangeConfigurationCapabilities define capabilities for configuration changes.
type DidChangeConfigurationCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DidChangeWatchedFilesCapabilities define capabilities for file watching.
type DidChangeWatchedFilesCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// WorkspaceSymbolClientCapabilities define capabilities for workspace symbols.
type WorkspaceSymbolClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// TextDocumentClientCapabilities define capabilities for text documents.
type TextDocumentClientCapabilities struct {
	Synchronization    *TextDocumentSyncClientCapabilities   `json:"synchronization,omitempty"`
	Completion         *CompletionClientCapabilities         `json:"completion,omitempty"`
	Hover              *HoverClientCapabilities              `json:"hover,omitempty"`
	SignatureHelp      *SignatureHelpClientCapabilities      `json:"signatureHelp,omitempty"`
	Definition         *DefinitionClientCapabilities         `json:"definition,omitempty"`
	TypeDefinition     *TypeDefinitionClientCapabilities     `json:"typeDefinition,omitempty"`
	References         *ReferenceClientCapabilities          `json:"references,omitempty"`
	DocumentHighlight  *DocumentHighlightClientCapabilities  `json:"documentHighlight,omitempty"`
	DocumentSymbol     *DocumentSymbolClientCapabilities     `json:"documentSymbol,omitempty"`
	CodeAction         *CodeActionClientCapabilities         `json:"codeAction,omitempty"`
	Formatting         *FormattingClientCapabilities         `json:"formatting,omitempty"`
	RangeFormatting    *RangeFormattingClientCapabilities    `json:"rangeFormatting,omitempty"`
	Rename             *RenameClientCapabilities             `json:"rename,omitempty"`
	PublishDiagnostics *PublishDiagnosticsClientCapabilities `json:"publishDiagnostics,omitempty"`
}

// TextDocumentSyncClientCapabilities define capabilities for text document sync.
type TextDocumentSyncClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

// CompletionClientCapabilities define capabilities for completion.
type CompletionClientCapabilities struct {
	DynamicRegistration bool                        `json:"dynamicRegistration,omitempty"`
	CompletionItem      *CompletionItemCapabilities `json:"completionItem,omitempty"`
	ContextSupport      bool                        `json:"contextSupport,omitempty"`
}

// CompletionItemCapabilities define capabilities for completion items.
type CompletionItemCapabilities struct {
	SnippetSupport          bool         `json:"snippetSupport,omitempty"`
	CommitCharactersSupport bool         `json:"commitCharactersSupport,omitempty"`
	DocumentationFormat     []MarkupKind `json:"documentationFormat,omitempty"`
	DeprecatedSupport       bool         `json:"deprecatedSupport,omitempty"`
	PreselectSupport        bool         `json:"preselectSupport,omitempty"`
}

// HoverClientCapabilities define capabilities for hover.
type HoverClientCapabilities struct {
	DynamicRegistration bool         `json:"dynamicRegistration,omitempty"`
	ContentFormat       []MarkupKind `json:"contentFormat,omitempty"`
}

// SignatureHelpClientCapabilities define capabilities for signature help.
type SignatureHelpClientCapabilities struct {
	DynamicRegistration  bool                              `json:"dynamicRegistration,omitempty"`
	SignatureInformation *SignatureInformationCapabilities `json:"signatureInformation,omitempty"`
	ContextSupport       bool                              `json:"contextSupport,omitempty"`
}

// SignatureInformationCapabilities define capabilities for signature information.
type SignatureInformationCapabilities struct {
	DocumentationFormat []MarkupKind `json:"documentationFormat,omitempty"`
}

// DefinitionClientCapabilities define capabilities for definition.
type DefinitionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

// TypeDefinitionClientCapabilities define capabilities for type definition.
type TypeDefinitionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

// ReferenceClientCapabilities define capabilities for references.
type ReferenceClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DocumentHighlightClientCapabilities define capabilities for document highlight.
type DocumentHighlightClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DocumentSymbolClientCapabilities define capabilities for document symbols.
type DocumentSymbolClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// CodeActionClientCapabilities define capabilities for code actions.
type CodeActionClientCapabilities struct {
	DynamicRegistration      bool                      `json:"dynamicRegistration,omitempty"`
	CodeActionLiteralSupport *CodeActionLiteralSupport `json:"codeActionLiteralSupport,omitempty"`
}

// CodeActionLiteralSupport define code action literal support capabilities.
type CodeActionLiteralSupport struct {
	CodeActionKind *CodeActionKindSupport `json:"codeActionKind,omitempty"`
}

// CodeActionKindSupport define code action kind support.
type CodeActionKindSupport struct {
	ValueSet []CodeActionKind `json:"valueSet,omitempty"`
}

// FormattingClientCapabilities define capabilities for formatting.
type FormattingClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// RangeFormattingClientCapabilities define capabilities for range formatting.
type RangeFormattingClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// RenameClientCapabilities define capabilities for rename.
type RenameClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	PrepareSupport      bool `json:"prepareSupport,omitempty"`
}

// PublishDiagnosticsClientCapabilities define capabilities for diagnostics.
type PublishDiagnosticsClientCapabilities struct {
	RelatedInformation     bool                  `json:"relatedInformation,omitempty"`
	TagSupport             *DiagnosticTagSupport `json:"tagSupport,omitempty"`
	VersionSupport         bool                  `json:"versionSupport,omitempty"`
	CodeDescriptionSupport bool                  `json:"codeDescriptionSupport,omitempty"`
	DataSupport            bool                  `json:"dataSupport,omitempty"`
}

// DiagnosticTagSupport define diagnostic tag support.
type DiagnosticTagSupport struct {
	ValueSet []DiagnosticTag `json:"valueSet,omitempty"`
}

// WindowClientCapabilities define capabilities for the window.
type WindowClientCapabilities struct {
	WorkDoneProgress bool `json:"workDoneProgress,omitempty"`
}

// GeneralClientCapabilities define general client capabilities.
type GeneralClientCapabilities struct {
	StaleRequestSupport *StaleRequestSupport `json:"staleRequestSupport,omitempty"`
}

// StaleRequestSupport define stale request handling.
type StaleRequestSupport struct {
	Cancel                 bool     `json:"cancel,omitempty"`
	RetryOnContentModified []string `json:"retryOnContentModified,omitempty"`
}

// ServerCapabilities define capabilities provided by the server.
type ServerCapabilities struct {
	TextDocumentSync                any                          `json:"textDocumentSync,omitempty"`
	CompletionProvider              *CompletionOptions           `json:"completionProvider,omitempty"`
	HoverProvider                   any                          `json:"hoverProvider,omitempty"`
	SignatureHelpProvider           *SignatureHelpOptions        `json:"signatureHelpProvider,omitempty"`
	DefinitionProvider              any                          `json:"definitionProvider,omitempty"`
	TypeDefinitionProvider          any                          `json:"typeDefinitionProvider,omitempty"`
	ReferencesProvider              any                          `json:"referencesProvider,omitempty"`
	DocumentHighlightProvider       any                          `json:"documentHighlightProvider,omitempty"`
	DocumentSymbolProvider          any                          `json:"documentSymbolProvider,omitempty"`
	WorkspaceSymbolProvider         any                          `json:"workspaceSymbolProvider,omitempty"`
	CodeActionProvider              any                          `json:"codeActionProvider,omitempty"`
	DocumentFormattingProvider      any                          `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider any                          `json:"documentRangeFormattingProvider,omitempty"`
	RenameProvider                  any                          `json:"renameProvider,omitempty"`
	Workspace                       *ServerWorkspaceCapabilities `json:"workspace,omitempty"`
}

// ServerWorkspaceCapabilities define workspace capabilities from the server.
type ServerWorkspaceCapabilities struct {
	WorkspaceFolders *WorkspaceFoldersServerCapabilities `json:"workspaceFolders,omitempty"`
}

// WorkspaceFoldersServerCapabilities define workspace folder support.
type WorkspaceFoldersServerCapabilities struct {
	Supported           bool `json:"supported,omitempty"`
	ChangeNotifications any  `json:"changeNotifications,omitempty"`
}

// CompletionOptions define options for completion.
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
	WorkDoneProgress  bool     `json:"workDoneProgress,omitempty"`
}

// SignatureHelpOptions define options for signature help.
type SignatureHelpOptions struct {
	TriggerCharacters   []string `json:"triggerCharacters,omitempty"`
	RetriggerCharacters []string `json:"retriggerCharacters,omitempty"`
}

// --- Document Sync ---

// DidOpenTextDocumentParams are parameters for textDocument/didOpen.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidChangeTextDocumentParams are parameters for textDocument/didChange.
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// DidCloseTextDocumentParams are parameters for textDocument/didClose.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DidSaveTextDocumentParams are parameters for textDocument/didSave.
type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         string                 `json:"text,omitempty"`
}

// TextDocumentSyncKind defines how the server wants to sync.
type TextDocumentSyncKind int

const (
	TextDocumentSyncKindNone        TextDocumentSyncKind = 0
	TextDocumentSyncKindFull        TextDocumentSyncKind = 1
	TextDocumentSyncKindIncremental TextDocumentSyncKind = 2
)

// --- Completion ---

// CompletionParams are parameters for textDocument/completion.
type CompletionParams struct {
	TextDocumentPositionParams
	Context *CompletionContext `json:"context,omitempty"`
}

// CompletionContext contains additional information about the context.
type CompletionContext struct {
	TriggerKind      CompletionTriggerKind `json:"triggerKind"`
	TriggerCharacter string                `json:"triggerCharacter,omitempty"`
}

// CompletionTriggerKind defines how a completion was triggered.
type CompletionTriggerKind int

const (
	CompletionTriggerKindInvoked                         CompletionTriggerKind = 1
	CompletionTriggerKindTriggerCharacter                CompletionTriggerKind = 2
	CompletionTriggerKindTriggerForIncompleteCompletions CompletionTriggerKind = 3
)

// CompletionList represents a collection of completion items.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// CompletionItem represents a completion suggestion.
type CompletionItem struct {
	Label               string              `json:"label"`
	Kind                CompletionItemKind  `json:"kind,omitempty"`
	Tags                []CompletionItemTag `json:"tags,omitempty"`
	Detail              string              `json:"detail,omitempty"`
	Documentation       any                 `json:"documentation,omitempty"` // string or MarkupContent
	Deprecated          bool                `json:"deprecated,omitempty"`
	Preselect           bool                `json:"preselect,omitempty"`
	SortText            string              `json:"sortText,omitempty"`
	FilterText          string              `json:"filterText,omitempty"`
	InsertText          string              `json:"insertText,omitempty"`
	InsertTextFormat    InsertTextFormat    `json:"insertTextFormat,omitempty"`
	TextEdit            *TextEdit           `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit          `json:"additionalTextEdits,omitempty"`
	CommitCharacters    []string            `json:"commitCharacters,omitempty"`
	Command             *Command            `json:"command,omitempty"`
	Data                any                 `json:"data,omitempty"`
}

// CompletionItemKind represents the type of completion item.
type CompletionItemKind int

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

// CompletionItemTag represents a tag for completion items.
type CompletionItemTag int

const (
	CompletionItemTagDeprecated CompletionItemTag = 1
)

// InsertTextFormat defines the format of insert text.
type InsertTextFormat int

const (
	InsertTextFormatPlainText InsertTextFormat = 1
	InsertTextFormatSnippet   InsertTextFormat = 2
)

// --- Hover ---

// HoverParams are parameters for textDocument/hover.
type HoverParams struct {
	TextDocumentPositionParams
}

// Hover represents hover information.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// --- Diagnostics ---

// PublishDiagnosticsParams are parameters for textDocument/publishDiagnostics.
type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Version     int          `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Diagnostic represents a diagnostic (error, warning, info, hint).
type Diagnostic struct {
	Range              Range                          `json:"range"`
	Severity           DiagnosticSeverity             `json:"severity,omitempty"`
	Code               any                            `json:"code,omitempty"` // string or number
	CodeDescription    *CodeDescription               `json:"codeDescription,omitempty"`
	Source             string                         `json:"source,omitempty"`
	Message            string                         `json:"message"`
	Tags               []DiagnosticTag                `json:"tags,omitempty"`
	RelatedInformation []DiagnosticRelatedInformation `json:"relatedInformation,omitempty"`
	Data               any                            `json:"data,omitempty"`
}

// DiagnosticSeverity represents the severity of a diagnostic.
type DiagnosticSeverity int

const (
	DiagnosticSeverityError       DiagnosticSeverity = 1
	DiagnosticSeverityWarning     DiagnosticSeverity = 2
	DiagnosticSeverityInformation DiagnosticSeverity = 3
	DiagnosticSeverityHint        DiagnosticSeverity = 4
)

// DiagnosticTag represents additional metadata about a diagnostic.
type DiagnosticTag int

const (
	DiagnosticTagUnnecessary DiagnosticTag = 1
	DiagnosticTagDeprecated  DiagnosticTag = 2
)

// CodeDescription describes a code.
type CodeDescription struct {
	Href string `json:"href"`
}

// DiagnosticRelatedInformation represents related diagnostic information.
type DiagnosticRelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

// --- Code Action ---

// CodeActionParams are parameters for textDocument/codeAction.
type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

// CodeActionContext contains additional information for code action requests.
type CodeActionContext struct {
	Diagnostics []Diagnostic     `json:"diagnostics"`
	Only        []CodeActionKind `json:"only,omitempty"`
}

// CodeAction represents a code action.
type CodeAction struct {
	Title       string         `json:"title"`
	Kind        CodeActionKind `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	IsPreferred bool           `json:"isPreferred,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
	Command     *Command       `json:"command,omitempty"`
	Data        any            `json:"data,omitempty"`
}

// CodeActionKind represents the type of code action.
type CodeActionKind string

const (
	CodeActionKindQuickFix              CodeActionKind = "quickfix"
	CodeActionKindRefactor              CodeActionKind = "refactor"
	CodeActionKindRefactorExtract       CodeActionKind = "refactor.extract"
	CodeActionKindRefactorInline        CodeActionKind = "refactor.inline"
	CodeActionKindRefactorRewrite       CodeActionKind = "refactor.rewrite"
	CodeActionKindSource                CodeActionKind = "source"
	CodeActionKindSourceOrganizeImports CodeActionKind = "source.organizeImports"
	CodeActionKindSourceFixAll          CodeActionKind = "source.fixAll"
)

// --- Formatting ---

// DocumentFormattingParams are parameters for textDocument/formatting.
type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

// DocumentRangeFormattingParams are parameters for textDocument/rangeFormatting.
type DocumentRangeFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Options      FormattingOptions      `json:"options"`
}

// FormattingOptions describe options for formatting.
type FormattingOptions struct {
	TabSize                int  `json:"tabSize"`
	InsertSpaces           bool `json:"insertSpaces"`
	TrimTrailingWhitespace bool `json:"trimTrailingWhitespace,omitempty"`
	InsertFinalNewline     bool `json:"insertFinalNewline,omitempty"`
	TrimFinalNewlines      bool `json:"trimFinalNewlines,omitempty"`
}

// --- Rename ---

// RenameParams are parameters for textDocument/rename.
type RenameParams struct {
	TextDocumentPositionParams
	NewName string `json:"newName"`
}

// PrepareRenameParams are parameters for textDocument/prepareRename.
type PrepareRenameParams struct {
	TextDocumentPositionParams
}

// PrepareRenameResult is the result of a prepare rename request.
type PrepareRenameResult struct {
	Range       Range  `json:"range"`
	Placeholder string `json:"placeholder,omitempty"`
}

// --- References ---

// ReferenceParams are parameters for textDocument/references.
type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

// ReferenceContext contains additional information for reference requests.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// --- Signature Help ---

// SignatureHelpParams are parameters for textDocument/signatureHelp.
type SignatureHelpParams struct {
	TextDocumentPositionParams
	Context *SignatureHelpContext `json:"context,omitempty"`
}

// SignatureHelpContext contains additional information about signature help.
type SignatureHelpContext struct {
	TriggerKind         SignatureHelpTriggerKind `json:"triggerKind"`
	TriggerCharacter    string                   `json:"triggerCharacter,omitempty"`
	IsRetrigger         bool                     `json:"isRetrigger"`
	ActiveSignatureHelp *SignatureHelp           `json:"activeSignatureHelp,omitempty"`
}

// SignatureHelpTriggerKind defines how a signature was triggered.
type SignatureHelpTriggerKind int

const (
	SignatureHelpTriggerKindInvoked          SignatureHelpTriggerKind = 1
	SignatureHelpTriggerKindTriggerCharacter SignatureHelpTriggerKind = 2
	SignatureHelpTriggerKindContentChange    SignatureHelpTriggerKind = 3
)

// SignatureHelp represents signature help.
type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature,omitempty"`
	ActiveParameter int                    `json:"activeParameter,omitempty"`
}

// SignatureInformation represents a signature.
type SignatureInformation struct {
	Label           string                 `json:"label"`
	Documentation   any                    `json:"documentation,omitempty"` // string or MarkupContent
	Parameters      []ParameterInformation `json:"parameters,omitempty"`
	ActiveParameter int                    `json:"activeParameter,omitempty"`
}

// ParameterInformation represents a parameter.
type ParameterInformation struct {
	Label         any `json:"label"`                   // string or [int, int]
	Documentation any `json:"documentation,omitempty"` // string or MarkupContent
}

// --- Document Symbols ---

// DocumentSymbolParams are parameters for textDocument/documentSymbol.
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DocumentSymbol represents a symbol in a document.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Tags           []SymbolTag      `json:"tags,omitempty"`
	Deprecated     bool             `json:"deprecated,omitempty"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation represents information about a symbol.
type SymbolInformation struct {
	Name          string      `json:"name"`
	Kind          SymbolKind  `json:"kind"`
	Tags          []SymbolTag `json:"tags,omitempty"`
	Deprecated    bool        `json:"deprecated,omitempty"`
	Location      Location    `json:"location"`
	ContainerName string      `json:"containerName,omitempty"`
}

// SymbolKind represents the type of symbol.
type SymbolKind int

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

// SymbolTag represents additional metadata about a symbol.
type SymbolTag int

const (
	SymbolTagDeprecated SymbolTag = 1
)

// --- Workspace Symbols ---

// WorkspaceSymbolParams are parameters for workspace/symbol.
type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// --- Utility Functions ---

// FilePathToURI converts a file path to a DocumentURI.
func FilePathToURI(path string) DocumentURI {
	if path == "" {
		return ""
	}

	// Make path absolute
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}

	// Convert to forward slashes
	path = filepath.ToSlash(path)

	// On Windows, add extra slash for drive letter
	if runtime.GOOS == "windows" && len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}

	// URL encode the path
	u := &url.URL{
		Scheme: "file",
		Path:   path,
	}

	return DocumentURI(u.String())
}

// URIToFilePath converts a DocumentURI to a file path.
func URIToFilePath(uri DocumentURI) string {
	if uri == "" {
		return ""
	}

	u, err := url.Parse(string(uri))
	if err != nil {
		return string(uri)
	}

	if u.Scheme != "file" {
		return string(uri)
	}

	path := u.Path

	// On Windows, remove leading slash before drive letter
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}

	return filepath.FromSlash(path)
}

// ParseCompletionResult parses a completion response which may be a list or array.
func ParseCompletionResult(data json.RawMessage) (*CompletionList, error) {
	if len(data) == 0 {
		return &CompletionList{}, nil
	}

	// Try parsing as CompletionList first
	var list CompletionList
	if err := json.Unmarshal(data, &list); err == nil && (list.Items != nil || list.IsIncomplete) {
		return &list, nil
	}

	// Try parsing as array of CompletionItem
	var items []CompletionItem
	if err := json.Unmarshal(data, &items); err == nil {
		return &CompletionList{Items: items}, nil
	}

	return nil, fmt.Errorf("failed to parse completion result")
}

// ParseLocationResult parses a location response which may be a single location,
// array of locations, or array of location links.
func ParseLocationResult(data json.RawMessage) ([]Location, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// Try parsing as single Location
	var loc Location
	if err := json.Unmarshal(data, &loc); err == nil && loc.URI != "" {
		return []Location{loc}, nil
	}

	// Try parsing as array of Location
	var locs []Location
	if err := json.Unmarshal(data, &locs); err == nil {
		return locs, nil
	}

	return nil, fmt.Errorf("failed to parse location result")
}

// ExtractDocumentation extracts documentation string from various formats.
func ExtractDocumentation(doc any) string {
	if doc == nil {
		return ""
	}

	switch v := doc.(type) {
	case string:
		return v
	case map[string]any:
		if val, ok := v["value"].(string); ok {
			return val
		}
	case MarkupContent:
		return v.Value
	}

	// Try JSON conversion for complex types
	if data, err := json.Marshal(doc); err == nil {
		var mc MarkupContent
		if err := json.Unmarshal(data, &mc); err == nil {
			return mc.Value
		}
		var s string
		if err := json.Unmarshal(data, &s); err == nil {
			return s
		}
	}

	return fmt.Sprintf("%v", doc)
}

// GetTextDocumentSyncKind extracts the sync kind from server capabilities.
func GetTextDocumentSyncKind(caps ServerCapabilities) TextDocumentSyncKind {
	if caps.TextDocumentSync == nil {
		return TextDocumentSyncKindNone
	}

	// It can be a number or an object
	switch v := caps.TextDocumentSync.(type) {
	case float64:
		return TextDocumentSyncKind(int(v))
	case int:
		return TextDocumentSyncKind(v)
	case map[string]any:
		if change, ok := v["change"].(float64); ok {
			return TextDocumentSyncKind(int(change))
		}
	}

	return TextDocumentSyncKindFull
}

// HasCapability checks if a capability is enabled (can be bool or object).
func HasCapability(cap any) bool {
	if cap == nil {
		return false
	}
	switch v := cap.(type) {
	case bool:
		return v
	case map[string]any:
		return true // Object means enabled with options
	default:
		return true
	}
}

// DefaultClientCapabilities returns reasonable default client capabilities.
func DefaultClientCapabilities() ClientCapabilities {
	return ClientCapabilities{
		Workspace: &WorkspaceClientCapabilities{
			ApplyEdit:        true,
			WorkspaceFolders: true,
			Configuration:    true,
			WorkspaceEdit: &WorkspaceEditClientCapabilities{
				DocumentChanges: true,
			},
		},
		TextDocument: &TextDocumentClientCapabilities{
			Synchronization: &TextDocumentSyncClientCapabilities{
				DidSave:           true,
				WillSave:          false,
				WillSaveWaitUntil: false,
			},
			Completion: &CompletionClientCapabilities{
				CompletionItem: &CompletionItemCapabilities{
					SnippetSupport:          true,
					DocumentationFormat:     []MarkupKind{MarkupKindMarkdown, MarkupKindPlainText},
					DeprecatedSupport:       true,
					PreselectSupport:        true,
					CommitCharactersSupport: true,
				},
				ContextSupport: true,
			},
			Hover: &HoverClientCapabilities{
				ContentFormat: []MarkupKind{MarkupKindMarkdown, MarkupKindPlainText},
			},
			SignatureHelp: &SignatureHelpClientCapabilities{
				SignatureInformation: &SignatureInformationCapabilities{
					DocumentationFormat: []MarkupKind{MarkupKindMarkdown, MarkupKindPlainText},
				},
				ContextSupport: true,
			},
			Definition:        &DefinitionClientCapabilities{LinkSupport: true},
			TypeDefinition:    &TypeDefinitionClientCapabilities{LinkSupport: true},
			References:        &ReferenceClientCapabilities{},
			DocumentHighlight: &DocumentHighlightClientCapabilities{},
			DocumentSymbol:    &DocumentSymbolClientCapabilities{},
			CodeAction: &CodeActionClientCapabilities{
				CodeActionLiteralSupport: &CodeActionLiteralSupport{
					CodeActionKind: &CodeActionKindSupport{
						ValueSet: []CodeActionKind{
							CodeActionKindQuickFix,
							CodeActionKindRefactor,
							CodeActionKindRefactorExtract,
							CodeActionKindRefactorInline,
							CodeActionKindRefactorRewrite,
							CodeActionKindSource,
							CodeActionKindSourceOrganizeImports,
						},
					},
				},
			},
			Formatting:      &FormattingClientCapabilities{},
			RangeFormatting: &RangeFormattingClientCapabilities{},
			Rename:          &RenameClientCapabilities{PrepareSupport: true},
			PublishDiagnostics: &PublishDiagnosticsClientCapabilities{
				RelatedInformation: true,
				TagSupport: &DiagnosticTagSupport{
					ValueSet: []DiagnosticTag{DiagnosticTagUnnecessary, DiagnosticTagDeprecated},
				},
				VersionSupport:         true,
				CodeDescriptionSupport: true,
				DataSupport:            true,
			},
		},
		Window: &WindowClientCapabilities{
			WorkDoneProgress: true,
		},
	}
}

// DetectLanguageID returns the LSP language ID for a file path.
func DetectLanguageID(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".py":
		return "python"
	case ".rb":
		return "ruby"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".php":
		return "php"
	case ".lua":
		return "lua"
	case ".sh", ".bash":
		return "shellscript"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".xml":
		return "xml"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss":
		return "scss"
	case ".less":
		return "less"
	case ".md", ".markdown":
		return "markdown"
	case ".sql":
		return "sql"
	case ".dockerfile":
		return "dockerfile"
	case ".proto":
		return "protobuf"
	case ".zig":
		return "zig"
	case ".nim":
		return "nim"
	case ".ex", ".exs":
		return "elixir"
	case ".erl", ".hrl":
		return "erlang"
	case ".hs":
		return "haskell"
	case ".ml", ".mli":
		return "ocaml"
	case ".fs", ".fsi", ".fsx":
		return "fsharp"
	case ".clj", ".cljs", ".cljc":
		return "clojure"
	case ".v":
		return "v"
	case ".d":
		return "d"
	default:
		// Check filename for special cases
		base := strings.ToLower(filepath.Base(path))
		switch base {
		case "dockerfile":
			return "dockerfile"
		case "makefile", "gnumakefile":
			return "makefile"
		case "cmakelists.txt":
			return "cmake"
		}
		return "plaintext"
	}
}
