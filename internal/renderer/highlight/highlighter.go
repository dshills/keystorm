package highlight

import (
	"sync"

	"github.com/dshills/keystorm/internal/renderer/core"
)

// Highlighter defines the interface for syntax highlighters.
type Highlighter interface {
	// HighlightLine tokenizes a single line and returns the tokens.
	// prevState is the lexer state from the previous line (for multi-line constructs).
	// Returns the tokens and the state at the end of the line.
	HighlightLine(line string, prevState LexerState) ([]Token, LexerState)

	// Language returns the language this highlighter supports.
	Language() string

	// FileExtensions returns the file extensions this highlighter handles.
	FileExtensions() []string
}

// Provider implements the renderer's HighlightProvider interface.
// It bridges the highlight package with the renderer.
type Provider struct {
	mu sync.RWMutex

	// highlighter is the active highlighter
	highlighter Highlighter

	// theme is the active color theme
	theme *Theme

	// lineCache caches highlighted lines
	lineCache map[uint32]*cachedLine

	// stateCache caches lexer states at line boundaries
	stateCache map[uint32]LexerState

	// maxCacheSize limits the cache size
	maxCacheSize int

	// lineGetter retrieves line content by line number
	lineGetter func(line uint32) string
}

// cachedLine holds cached highlighting for a line.
type cachedLine struct {
	text   string  // Original text (for cache validation)
	tokens []Token // Highlighted tokens
	state  LexerState
}

// NewProvider creates a new highlight provider.
func NewProvider(theme *Theme, maxCache int) *Provider {
	if theme == nil {
		theme = DefaultTheme()
	}
	if maxCache <= 0 {
		maxCache = 1000
	}
	return &Provider{
		theme:        theme,
		lineCache:    make(map[uint32]*cachedLine),
		stateCache:   make(map[uint32]LexerState),
		maxCacheSize: maxCache,
	}
}

// SetHighlighter sets the active highlighter.
func (p *Provider) SetHighlighter(h Highlighter) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.highlighter = h
	p.clearCache()
}

// SetTheme sets the active theme.
func (p *Provider) SetTheme(theme *Theme) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.theme = theme
}

// Theme returns the current theme.
func (p *Provider) Theme() *Theme {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.theme
}

// SetLineGetter sets the function to retrieve line content.
func (p *Provider) SetLineGetter(getter func(line uint32) string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lineGetter = getter
}

// HighlightsForLine returns style spans for the given line.
// Implements the renderer.HighlightProvider interface.
func (p *Provider) HighlightsForLine(line uint32) []core.StyleSpan {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.highlighter == nil || p.lineGetter == nil {
		return nil
	}

	text := p.lineGetter(line)
	tokens := p.getTokensForLine(line, text)

	if len(tokens) == 0 {
		return nil
	}

	// Convert tokens to style spans
	spans := make([]core.StyleSpan, 0, len(tokens))
	for _, tok := range tokens {
		style := p.theme.StyleForToken(tok.Type)
		spans = append(spans, core.StyleSpan{
			StartCol: tok.StartCol,
			EndCol:   tok.EndCol,
			Style:    style,
		})
	}

	return spans
}

// InvalidateLines invalidates cached highlighting for a range.
func (p *Provider) InvalidateLines(startLine, endLine uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Invalidate all lines from startLine onwards
	// because a change can affect subsequent lines' state
	// We collect lines to delete to avoid infinite loop on sparse caches
	toDelete := make([]uint32, 0)
	for line := range p.lineCache {
		if line >= startLine {
			toDelete = append(toDelete, line)
		}
	}
	for _, line := range toDelete {
		delete(p.lineCache, line)
		delete(p.stateCache, line)
	}
}

// InvalidateAll clears all cached highlighting.
func (p *Provider) InvalidateAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearCache()
}

// getTokensForLine returns tokens for a line, using cache if available.
func (p *Provider) getTokensForLine(line uint32, text string) []Token {
	// Check cache
	if cached, ok := p.lineCache[line]; ok && cached.text == text {
		return cached.tokens
	}

	// Get previous state
	prevState := LexerStateNormal
	if line > 0 {
		if state, ok := p.stateCache[line-1]; ok {
			prevState = state
		} else {
			// Need to compute state for previous lines
			prevState = p.computeStateUpTo(line - 1)
		}
	}

	// Highlight the line
	tokens, endState := p.highlighter.HighlightLine(text, prevState)

	// Cache the result
	p.cacheResult(line, text, tokens, endState)

	return tokens
}

// computeStateUpTo computes and caches lexer state up to the given line.
func (p *Provider) computeStateUpTo(targetLine uint32) LexerState {
	// Find the last cached state before targetLine
	var startLine uint32
	state := LexerStateNormal

	for line := targetLine; line > 0; line-- {
		if s, ok := p.stateCache[line-1]; ok {
			startLine = line
			state = s
			break
		}
	}

	// Compute states from startLine to targetLine
	for line := startLine; line <= targetLine; line++ {
		text := p.lineGetter(line)
		_, state = p.highlighter.HighlightLine(text, state)
		p.stateCache[line] = state
	}

	return state
}

// cacheResult caches a highlighting result.
func (p *Provider) cacheResult(line uint32, text string, tokens []Token, state LexerState) {
	// Evict if cache is too large
	if len(p.lineCache) >= p.maxCacheSize {
		p.evictCache()
	}

	p.lineCache[line] = &cachedLine{
		text:   text,
		tokens: tokens,
		state:  state,
	}
	p.stateCache[line] = state
}

// evictCache removes some entries from the cache.
func (p *Provider) evictCache() {
	// Simple strategy: remove ~25% of entries
	toRemove := len(p.lineCache) / 4
	if toRemove < 10 {
		toRemove = 10
	}

	removed := 0
	for line := range p.lineCache {
		delete(p.lineCache, line)
		delete(p.stateCache, line)
		removed++
		if removed >= toRemove {
			break
		}
	}
}

// clearCache clears all cached data.
func (p *Provider) clearCache() {
	p.lineCache = make(map[uint32]*cachedLine)
	p.stateCache = make(map[uint32]LexerState)
}

// Registry manages available highlighters.
type Registry struct {
	mu sync.RWMutex

	// byLanguage maps language names to highlighters
	byLanguage map[string]Highlighter

	// byExtension maps file extensions to highlighters
	byExtension map[string]Highlighter
}

// NewRegistry creates a new highlighter registry.
func NewRegistry() *Registry {
	return &Registry{
		byLanguage:  make(map[string]Highlighter),
		byExtension: make(map[string]Highlighter),
	}
}

// Register adds a highlighter to the registry.
func (r *Registry) Register(h Highlighter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.byLanguage[h.Language()] = h
	for _, ext := range h.FileExtensions() {
		r.byExtension[ext] = h
	}
}

// GetByLanguage returns a highlighter for the given language.
func (r *Registry) GetByLanguage(language string) (Highlighter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.byLanguage[language]
	return h, ok
}

// GetByExtension returns a highlighter for the given file extension.
func (r *Registry) GetByExtension(ext string) (Highlighter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Handle empty extension
	if ext == "" {
		return nil, false
	}

	// Normalize extension (ensure it starts with .)
	if ext[0] != '.' {
		ext = "." + ext
	}

	h, ok := r.byExtension[ext]
	return h, ok
}

// Languages returns all registered language names.
func (r *Registry) Languages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	langs := make([]string, 0, len(r.byLanguage))
	for lang := range r.byLanguage {
		langs = append(langs, lang)
	}
	return langs
}

// DefaultRegistry returns a registry with built-in highlighters.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	// Built-in highlighters will be registered here
	// For now, we'll add them in simple.go
	return r
}
