package lsp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/dshills/keystorm/internal/plugin/api"
)

// Provider implements api.LSPProvider interface using the LSP Client.
// It acts as an adapter between the internal LSP implementation and
// the plugin API, handling type conversions and context management.
//
// Provider is safe for concurrent use.
type Provider struct {
	mu     sync.RWMutex
	client *Client

	// Document content cache for position conversion
	// Maps file path to content
	contentCache map[string]string

	// Request timeout for LSP operations
	timeout time.Duration
}

// ProviderOption configures the Provider.
type ProviderOption func(*Provider)

// WithProviderTimeout sets the request timeout.
func WithProviderTimeout(d time.Duration) ProviderOption {
	return func(p *Provider) {
		p.timeout = d
	}
}

// NewProvider creates a new LSP provider wrapping the given client.
// Panics if client is nil.
func NewProvider(client *Client, opts ...ProviderOption) *Provider {
	if client == nil {
		panic("lsp: NewProvider called with nil client")
	}

	p := &Provider{
		client:       client,
		contentCache: make(map[string]string),
		timeout:      10 * time.Second,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// SetDocumentContent updates the cached content for a document.
// This is needed for accurate position/offset conversions.
func (p *Provider) SetDocumentContent(path, content string) {
	p.mu.Lock()
	p.contentCache[path] = content
	p.mu.Unlock()
}

// ClearDocumentContent removes cached content for a document.
func (p *Provider) ClearDocumentContent(path string) {
	p.mu.Lock()
	delete(p.contentCache, path)
	p.mu.Unlock()
}

// getContent returns cached content for a document.
func (p *Provider) getContent(path string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.contentCache[path]
}

// context returns a context with the configured timeout.
func (p *Provider) context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), p.timeout)
}

// --- LSPProvider Interface Implementation ---

// Completions returns completion items at the given position.
func (p *Provider) Completions(bufferPath string, offset int) ([]api.CompletionItem, error) {
	ctx, cancel := p.context()
	defer cancel()

	content := p.getContent(bufferPath)
	pos := ByteOffsetToLSPPosition(content, offset)

	// Extract prefix for filtering (simple heuristic: word characters before cursor)
	prefix := providerExtractPrefix(content, offset)

	result, err := p.client.Complete(ctx, bufferPath, pos, prefix)
	if err != nil {
		return nil, err
	}

	if result == nil || len(result.Items) == 0 {
		return nil, nil
	}

	items := make([]api.CompletionItem, len(result.Items))
	for i, item := range result.Items {
		items[i] = providerConvertCompletionItem(item)
	}

	return items, nil
}

// Diagnostics returns diagnostics for the given file.
func (p *Provider) Diagnostics(bufferPath string) ([]api.Diagnostic, error) {
	diags := p.client.Diagnostics(bufferPath)
	if len(diags) == 0 {
		return nil, nil
	}

	content := p.getContent(bufferPath)
	result := make([]api.Diagnostic, len(diags))
	for i, diag := range diags {
		result[i] = providerConvertDiagnostic(diag, content)
	}

	return result, nil
}

// Definition returns the definition location for the symbol at the given position.
func (p *Provider) Definition(bufferPath string, offset int) (*api.Location, error) {
	ctx, cancel := p.context()
	defer cancel()

	content := p.getContent(bufferPath)
	pos := ByteOffsetToLSPPosition(content, offset)

	result, err := p.client.GoToDefinition(ctx, bufferPath, pos)
	if err != nil {
		return nil, err
	}

	if result == nil || len(result.Locations) == 0 {
		return nil, nil
	}

	// Return the first location
	loc := result.Locations[0]
	targetPath := URIToFilePath(loc.URI)
	targetContent := p.getContent(targetPath)
	apiLoc := providerConvertLocation(loc, targetContent)
	return &apiLoc, nil
}

// References returns all references to the symbol at the given position.
func (p *Provider) References(bufferPath string, offset int, includeDeclaration bool) ([]api.Location, error) {
	ctx, cancel := p.context()
	defer cancel()

	content := p.getContent(bufferPath)
	pos := ByteOffsetToLSPPosition(content, offset)

	result, err := p.client.FindReferences(ctx, bufferPath, pos)
	if err != nil {
		return nil, err
	}

	if result == nil || len(result.Locations) == 0 {
		return nil, nil
	}

	locations := make([]api.Location, len(result.Locations))
	for i, loc := range result.Locations {
		targetPath := URIToFilePath(loc.URI)
		targetContent := p.getContent(targetPath)
		locations[i] = providerConvertLocation(loc, targetContent)
	}

	return locations, nil
}

// Hover returns hover information for the symbol at the given position.
func (p *Provider) Hover(bufferPath string, offset int) (*api.HoverInfo, error) {
	ctx, cancel := p.context()
	defer cancel()

	content := p.getContent(bufferPath)
	pos := ByteOffsetToLSPPosition(content, offset)

	hover, err := p.client.Hover(ctx, bufferPath, pos)
	if err != nil {
		return nil, err
	}

	if hover == nil {
		return nil, nil
	}

	return providerConvertHover(hover, content), nil
}

// SignatureHelp returns signature help for the function at the given position.
func (p *Provider) SignatureHelp(bufferPath string, offset int) (*api.SignatureInfo, error) {
	ctx, cancel := p.context()
	defer cancel()

	content := p.getContent(bufferPath)
	pos := ByteOffsetToLSPPosition(content, offset)

	result, err := p.client.SignatureHelp(ctx, bufferPath, pos)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	return providerConvertSignatureHelp(result), nil
}

// Format formats the document or selection.
func (p *Provider) Format(bufferPath string, startOffset, endOffset int) ([]api.TextEdit, error) {
	ctx, cancel := p.context()
	defer cancel()

	content := p.getContent(bufferPath)

	var result *FormatResult
	var err error

	if startOffset < 0 || endOffset < 0 {
		// Format entire document
		result, err = p.client.Format(ctx, bufferPath)
	} else {
		// Format range
		rng := Range{
			Start: ByteOffsetToLSPPosition(content, startOffset),
			End:   ByteOffsetToLSPPosition(content, endOffset),
		}
		result, err = p.client.FormatRange(ctx, bufferPath, rng)
	}

	if err != nil {
		return nil, err
	}

	if result == nil || len(result.Edits) == 0 {
		return nil, nil
	}

	edits := make([]api.TextEdit, len(result.Edits))
	for i, edit := range result.Edits {
		edits[i] = providerConvertTextEdit(edit, content)
	}

	return edits, nil
}

// CodeActions returns available code actions at the given range.
func (p *Provider) CodeActions(bufferPath string, startOffset, endOffset int, diagnostics []api.Diagnostic) ([]api.CodeAction, error) {
	ctx, cancel := p.context()
	defer cancel()

	content := p.getContent(bufferPath)

	rng := Range{
		Start: ByteOffsetToLSPPosition(content, startOffset),
		End:   ByteOffsetToLSPPosition(content, endOffset),
	}

	// Convert API diagnostics to LSP diagnostics
	lspDiags := make([]Diagnostic, len(diagnostics))
	for i, diag := range diagnostics {
		lspDiags[i] = providerConvertAPIDiagnosticToLSP(diag, content)
	}

	result, err := p.client.CodeActions(ctx, bufferPath, rng, lspDiags)
	if err != nil {
		return nil, err
	}

	if result == nil || len(result.All) == 0 {
		return nil, nil
	}

	actions := make([]api.CodeAction, len(result.All))
	for i, action := range result.All {
		actions[i] = providerConvertCodeAction(action, content)
	}

	return actions, nil
}

// Rename renames the symbol at the given position.
func (p *Provider) Rename(bufferPath string, offset int, newName string) ([]api.TextEdit, error) {
	ctx, cancel := p.context()
	defer cancel()

	content := p.getContent(bufferPath)
	pos := ByteOffsetToLSPPosition(content, offset)

	result, err := p.client.Rename(ctx, bufferPath, pos, newName)
	if err != nil {
		return nil, err
	}

	if result == nil || result.Edit == nil {
		return nil, nil
	}

	// Collect all edits from the workspace edit
	var edits []api.TextEdit

	// Handle document changes (map format)
	for uri, changes := range result.Edit.Changes {
		path := URIToFilePath(uri)
		docContent := p.getContent(path)
		for _, change := range changes {
			edits = append(edits, providerConvertTextEdit(change, docContent))
		}
	}

	// Handle document edits (newer LSP format) - DocumentChanges is []any
	for _, docEditAny := range result.Edit.DocumentChanges {
		// Try to extract as a map and parse
		if docEditMap, ok := docEditAny.(map[string]any); ok {
			// Get the text document URI
			if textDoc, ok := docEditMap["textDocument"].(map[string]any); ok {
				if uriVal, ok := textDoc["uri"].(string); ok {
					path := URIToFilePath(DocumentURI(uriVal))
					docContent := p.getContent(path)

					// Get edits array
					if editsArr, ok := docEditMap["edits"].([]any); ok {
						for _, editAny := range editsArr {
							if editMap, ok := editAny.(map[string]any); ok {
								edit := parseTextEditFromMap(editMap)
								edits = append(edits, providerConvertTextEdit(edit, docContent))
							}
						}
					}
				}
			}
		}
	}

	return edits, nil
}

// IsAvailable returns true if an LSP server is available for the given file.
func (p *Provider) IsAvailable(bufferPath string) bool {
	return p.client.IsAvailable(bufferPath)
}

// --- Type Conversion Helpers ---

// providerExtractPrefix extracts the word prefix before the cursor for completion filtering.
// The offset is a byte offset into the content string.
func providerExtractPrefix(content string, offset int) string {
	if content == "" || offset <= 0 || offset > len(content) {
		return ""
	}

	// Convert to runes for proper UTF-8 handling
	runes := []rune(content)

	// Convert byte offset to rune offset
	runeOffset := 0
	byteCount := 0
	for i, r := range runes {
		if byteCount >= offset {
			runeOffset = i
			break
		}
		byteCount += len(string(r))
		if byteCount >= offset {
			runeOffset = i + 1
			break
		}
	}
	if byteCount < offset {
		runeOffset = len(runes)
	}

	// Find start of word
	start := runeOffset
	for start > 0 {
		r := runes[start-1]
		if !providerIsWordChar(r) {
			break
		}
		start--
	}

	if start >= runeOffset {
		return ""
	}

	return string(runes[start:runeOffset])
}

// providerIsWordChar returns true if the rune is a word character.
func providerIsWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// providerConvertCompletionItem converts an LSP CompletionItem to an API CompletionItem.
func providerConvertCompletionItem(item CompletionItem) api.CompletionItem {
	return api.CompletionItem{
		Label:         item.Label,
		Kind:          api.CompletionKind(item.Kind),
		Detail:        item.Detail,
		Documentation: providerExtractDocumentation(item.Documentation),
		InsertText:    GetInsertText(item),
		SortText:      item.SortText,
	}
}

// providerExtractDocumentation extracts text from MarkupContent or string.
func providerExtractDocumentation(doc any) string {
	if doc == nil {
		return ""
	}
	switch v := doc.(type) {
	case string:
		return v
	case MarkupContent:
		return v.Value
	case *MarkupContent:
		if v != nil {
			return v.Value
		}
	case map[string]any:
		if val, ok := v["value"].(string); ok {
			return val
		}
	}
	return ""
}

// providerConvertDiagnostic converts an LSP Diagnostic to an API Diagnostic.
func providerConvertDiagnostic(diag Diagnostic, content string) api.Diagnostic {
	apiDiag := api.Diagnostic{
		Severity: api.DiagnosticSeverity(diag.Severity),
		Message:  diag.Message,
		Source:   diag.Source,
	}

	// Convert code to string
	if diag.Code != nil {
		switch v := diag.Code.(type) {
		case string:
			apiDiag.Code = v
		case float64:
			apiDiag.Code = fmt.Sprintf("%g", v)
		case int:
			apiDiag.Code = fmt.Sprintf("%d", v)
		case int64:
			apiDiag.Code = fmt.Sprintf("%d", v)
		}
	}

	// Convert range
	apiDiag.Range = providerConvertRange(diag.Range, content)

	// Convert related info
	if len(diag.RelatedInformation) > 0 {
		apiDiag.RelatedInfo = make([]api.DiagnosticRelatedInfo, len(diag.RelatedInformation))
		for i, info := range diag.RelatedInformation {
			apiDiag.RelatedInfo[i] = api.DiagnosticRelatedInfo{
				Location: providerConvertLocation(info.Location, content),
				Message:  info.Message,
			}
		}
	}

	return apiDiag
}

// providerConvertAPIDiagnosticToLSP converts an API Diagnostic to an LSP Diagnostic.
func providerConvertAPIDiagnosticToLSP(diag api.Diagnostic, content string) Diagnostic {
	return Diagnostic{
		Range:    providerConvertAPIRangeToLSP(diag.Range, content),
		Severity: DiagnosticSeverity(diag.Severity),
		Code:     diag.Code,
		Source:   diag.Source,
		Message:  diag.Message,
	}
}

// providerConvertLocation converts an LSP Location to an API Location.
func providerConvertLocation(loc Location, content string) api.Location {
	return api.Location{
		Path:  URIToFilePath(loc.URI),
		Range: providerConvertRange(loc.Range, content),
	}
}

// providerConvertRange converts an LSP Range to an API Range.
func providerConvertRange(rng Range, _ string) api.Range {
	return api.Range{
		StartLine:   int(rng.Start.Line),
		StartColumn: int(rng.Start.Character),
		EndLine:     int(rng.End.Line),
		EndColumn:   int(rng.End.Character),
	}
}

// providerConvertAPIRangeToLSP converts an API Range to an LSP Range.
func providerConvertAPIRangeToLSP(rng api.Range, _ string) Range {
	return Range{
		Start: Position{
			Line:      rng.StartLine,
			Character: rng.StartColumn,
		},
		End: Position{
			Line:      rng.EndLine,
			Character: rng.EndColumn,
		},
	}
}

// providerConvertHover converts an LSP Hover to an API HoverInfo.
func providerConvertHover(hover *Hover, content string) *api.HoverInfo {
	if hover == nil {
		return nil
	}

	info := &api.HoverInfo{
		Contents: providerExtractHoverContents(hover.Contents),
	}

	if hover.Range != nil {
		rng := providerConvertRange(*hover.Range, content)
		info.Range = &rng
	}

	return info
}

// providerExtractHoverContents extracts text from hover contents.
func providerExtractHoverContents(contents any) string {
	if contents == nil {
		return ""
	}
	switch v := contents.(type) {
	case string:
		return v
	case MarkupContent:
		return v.Value
	case *MarkupContent:
		if v != nil {
			return v.Value
		}
	case map[string]any:
		if val, ok := v["value"].(string); ok {
			return val
		}
	case []any:
		// Multiple contents, concatenate
		var parts []string
		for _, item := range v {
			switch it := item.(type) {
			case string:
				parts = append(parts, it)
			case MarkupContent:
				parts = append(parts, it.Value)
			case *MarkupContent:
				if it != nil {
					parts = append(parts, it.Value)
				}
			case map[string]any:
				if val, ok := it["value"].(string); ok {
					parts = append(parts, val)
				}
			}
		}
		return providerJoinStrings(parts, "\n\n")
	}
	return ""
}

// providerJoinStrings joins strings with a separator.
func providerJoinStrings(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

// providerConvertSignatureHelp converts an LSP SignatureHelpResult to an API SignatureInfo.
func providerConvertSignatureHelp(result *SignatureHelpResult) *api.SignatureInfo {
	if result == nil || result.Help == nil {
		return nil
	}

	sh := result.Help
	info := &api.SignatureInfo{
		ActiveSignature: sh.ActiveSignature,
		ActiveParameter: sh.ActiveParameter,
		Signatures:      make([]api.SignatureInformation, len(sh.Signatures)),
	}

	for i, sig := range sh.Signatures {
		info.Signatures[i] = api.SignatureInformation{
			Label:         sig.Label,
			Documentation: providerExtractDocumentation(sig.Documentation),
			Parameters:    make([]api.ParameterInfo, len(sig.Parameters)),
		}

		for j, param := range sig.Parameters {
			info.Signatures[i].Parameters[j] = api.ParameterInfo{
				Label:         providerExtractParamLabel(param.Label),
				Documentation: providerExtractDocumentation(param.Documentation),
			}
		}
	}

	return info
}

// providerExtractParamLabel extracts the label string from a parameter label.
func providerExtractParamLabel(label any) string {
	switch v := label.(type) {
	case string:
		return v
	case []any:
		// [start, end] offsets into signature label
		if len(v) >= 2 {
			// Return empty string, the caller should handle offset-based labels
			return ""
		}
	}
	return ""
}

// providerConvertTextEdit converts an LSP TextEdit to an API TextEdit.
func providerConvertTextEdit(edit TextEdit, content string) api.TextEdit {
	return api.TextEdit{
		Range:   providerConvertRange(edit.Range, content),
		NewText: edit.NewText,
	}
}

// providerConvertCodeAction converts an LSP CodeAction to an API CodeAction.
func providerConvertCodeAction(action CodeAction, content string) api.CodeAction {
	apiAction := api.CodeAction{
		Title: action.Title,
		Kind:  api.CodeActionKind(action.Kind),
	}

	// Convert command if present
	if action.Command != nil {
		apiAction.Command = action.Command.Command
	}

	// Convert workspace edit if present
	if action.Edit != nil {
		var edits []api.TextEdit

		// Handle document changes (map format)
		for uri, changes := range action.Edit.Changes {
			path := URIToFilePath(uri)
			// Note: We may not have content for all files
			_ = path // Used for future content lookup if needed
			for _, change := range changes {
				edits = append(edits, providerConvertTextEdit(change, content))
			}
		}

		// Handle document edits (newer LSP format) - DocumentChanges is []any
		for _, docEditAny := range action.Edit.DocumentChanges {
			if docEditMap, ok := docEditAny.(map[string]any); ok {
				if editsArr, ok := docEditMap["edits"].([]any); ok {
					for _, editAny := range editsArr {
						if editMap, ok := editAny.(map[string]any); ok {
							edit := parseTextEditFromMap(editMap)
							edits = append(edits, providerConvertTextEdit(edit, content))
						}
					}
				}
			}
		}

		apiAction.Edits = edits
	}

	return apiAction
}

// parseTextEditFromMap parses a TextEdit from a map[string]any.
func parseTextEditFromMap(m map[string]any) TextEdit {
	edit := TextEdit{}

	if newText, ok := m["newText"].(string); ok {
		edit.NewText = newText
	}

	if rangeMap, ok := m["range"].(map[string]any); ok {
		edit.Range = parseRangeFromMap(rangeMap)
	}

	return edit
}

// parseRangeFromMap parses a Range from a map[string]any.
func parseRangeFromMap(m map[string]any) Range {
	rng := Range{}

	if startMap, ok := m["start"].(map[string]any); ok {
		rng.Start = parsePositionFromMap(startMap)
	}

	if endMap, ok := m["end"].(map[string]any); ok {
		rng.End = parsePositionFromMap(endMap)
	}

	return rng
}

// parsePositionFromMap parses a Position from a map[string]any.
func parsePositionFromMap(m map[string]any) Position {
	pos := Position{}

	if line, ok := m["line"].(float64); ok {
		pos.Line = int(line)
	}

	if char, ok := m["character"].(float64); ok {
		pos.Character = int(char)
	}

	return pos
}

// Verify Provider implements api.LSPProvider interface.
var _ api.LSPProvider = (*Provider)(nil)
