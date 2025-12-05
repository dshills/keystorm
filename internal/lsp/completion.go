package lsp

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// CompletionService provides high-level completion functionality with
// filtering, sorting, caching, and trigger character detection.
type CompletionService struct {
	mu      sync.RWMutex
	manager *Manager

	// Configuration
	maxResults      int
	cacheTimeout    time.Duration
	prefetchOnType  bool
	cacheCleanupCap int // Only cleanup when cache exceeds this size

	// Caching - keyed by path+line+character (position), not prefix
	cache map[cacheKey]*cachedCompletion

	// Trigger character tracking
	triggerChars map[string][]string // languageID -> trigger chars
}

// cacheKey identifies a cached completion result by position (not prefix).
type cacheKey struct {
	path      string
	line      int
	character int
	language  string
}

// cachedCompletion stores a cached completion result.
type cachedCompletion struct {
	list      *CompletionList
	timestamp time.Time
	prefix    string // The prefix used when this was cached
}

// CompletionServiceOption configures the completion service.
type CompletionServiceOption func(*CompletionService)

// WithMaxResults sets the maximum number of results to return.
func WithMaxResults(n int) CompletionServiceOption {
	return func(cs *CompletionService) {
		cs.maxResults = n
	}
}

// WithCacheTimeout sets how long to cache completion results.
func WithCacheTimeout(d time.Duration) CompletionServiceOption {
	return func(cs *CompletionService) {
		cs.cacheTimeout = d
	}
}

// WithPrefetchOnType enables prefetching completions while typing.
func WithPrefetchOnType(enabled bool) CompletionServiceOption {
	return func(cs *CompletionService) {
		cs.prefetchOnType = enabled
	}
}

// NewCompletionService creates a new completion service.
func NewCompletionService(mgr *Manager, opts ...CompletionServiceOption) *CompletionService {
	cs := &CompletionService{
		manager:         mgr,
		maxResults:      100,
		cacheTimeout:    5 * time.Second,
		prefetchOnType:  true,
		cacheCleanupCap: 100,
		cache:           make(map[cacheKey]*cachedCompletion),
		triggerChars:    make(map[string][]string),
	}

	for _, opt := range opts {
		opt(cs)
	}

	return cs
}

// Complete returns completion items at the given position.
func (cs *CompletionService) Complete(ctx context.Context, path string, pos Position, prefix string) (*CompletionResult, error) {
	if cs.manager == nil {
		return nil, ErrServerNotReady
	}

	// Get server for file
	server, err := cs.manager.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}

	languageID := DetectLanguageID(path)

	// Check cache - key by position, not prefix
	key := cacheKey{
		path:      path,
		line:      pos.Line,
		character: pos.Character,
		language:  languageID,
	}

	if cached := cs.checkCache(key, prefix); cached != nil {
		return cs.processResults(cached, prefix), nil
	}

	// Request completions from server
	list, err := server.Completion(ctx, path, pos)
	if err != nil {
		return nil, err
	}

	// Cache the result
	cs.storeCache(key, list, prefix)

	return cs.processResults(list, prefix), nil
}

// CompleteWithTrigger handles completion triggered by a specific character.
func (cs *CompletionService) CompleteWithTrigger(ctx context.Context, path string, pos Position, triggerChar string) (*CompletionResult, error) {
	if cs.manager == nil {
		return nil, ErrServerNotReady
	}

	server, err := cs.manager.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}

	list, err := server.CompletionWithTrigger(ctx, path, pos, triggerChar)
	if err != nil {
		return nil, err
	}

	return cs.processResults(list, ""), nil
}

// ResolveItem resolves additional details for a completion item.
func (cs *CompletionService) ResolveItem(ctx context.Context, path string, item CompletionItem) (*CompletionItem, error) {
	if cs.manager == nil {
		return nil, ErrServerNotReady
	}

	server, err := cs.manager.ServerForFile(ctx, path)
	if err != nil {
		return nil, err
	}

	return server.ResolveCompletionItem(ctx, item)
}

// IsTriggerCharacter returns true if the character triggers completion.
func (cs *CompletionService) IsTriggerCharacter(ctx context.Context, path string, char string) bool {
	if cs.manager == nil {
		return false
	}

	languageID := DetectLanguageID(path)

	cs.mu.RLock()
	chars, ok := cs.triggerChars[languageID]
	cs.mu.RUnlock()

	if ok {
		for _, c := range chars {
			if c == char {
				return true
			}
		}
		return false
	}

	// Fetch from server
	server, err := cs.manager.ServerForFile(ctx, path)
	if err != nil {
		return false
	}

	triggerChars := server.CompletionTriggerCharacters()

	cs.mu.Lock()
	cs.triggerChars[languageID] = triggerChars
	cs.mu.Unlock()

	for _, c := range triggerChars {
		if c == char {
			return true
		}
	}

	return false
}

// GetTriggerCharacters returns the trigger characters for a language.
func (cs *CompletionService) GetTriggerCharacters(ctx context.Context, path string) []string {
	if cs.manager == nil {
		return nil
	}

	languageID := DetectLanguageID(path)

	cs.mu.RLock()
	chars, ok := cs.triggerChars[languageID]
	cs.mu.RUnlock()

	if ok {
		return chars
	}

	server, err := cs.manager.ServerForFile(ctx, path)
	if err != nil {
		return nil
	}

	triggerChars := server.CompletionTriggerCharacters()

	cs.mu.Lock()
	cs.triggerChars[languageID] = triggerChars
	cs.mu.Unlock()

	return triggerChars
}

// InvalidateCache clears the completion cache for a file.
func (cs *CompletionService) InvalidateCache(path string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for key := range cs.cache {
		if key.path == path {
			delete(cs.cache, key)
		}
	}
}

// ClearCache clears all cached completions.
func (cs *CompletionService) ClearCache() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.cache = make(map[cacheKey]*cachedCompletion)
}

// checkCache returns a cached result if still valid.
// The cache is compatible if:
// - Both prefixes are empty (invoked completion reuse), OR
// - Cached prefix is non-empty AND new prefix starts with cached prefix
// This prevents empty-prefix cache entries from matching typed prefixes.
func (cs *CompletionService) checkCache(key cacheKey, prefix string) *CompletionList {
	// Fast path: try read lock first
	cs.mu.RLock()
	cached, ok := cs.cache[key]
	if !ok {
		cs.mu.RUnlock()
		return nil
	}

	// Check if expired
	if time.Since(cached.timestamp) > cs.cacheTimeout {
		cs.mu.RUnlock()
		// Upgrade to write lock to delete
		cs.mu.Lock()
		// Re-check in case another goroutine already deleted
		if c, exists := cs.cache[key]; exists && time.Since(c.timestamp) > cs.cacheTimeout {
			delete(cs.cache, key)
		}
		cs.mu.Unlock()
		return nil
	}

	// Check prefix compatibility:
	// - Empty cached prefix only matches empty request prefix
	// - Non-empty cached prefix matches if request prefix starts with it
	prefixLower := strings.ToLower(prefix)
	cachedPrefixLower := strings.ToLower(cached.prefix)

	compatible := false
	if cachedPrefixLower == "" {
		// Empty cached prefix only reusable for empty request prefix
		compatible = (prefixLower == "")
	} else {
		// Non-empty cached prefix: request must start with it
		compatible = strings.HasPrefix(prefixLower, cachedPrefixLower)
	}

	if !compatible {
		cs.mu.RUnlock()
		return nil
	}

	list := cached.list
	cs.mu.RUnlock()
	return list
}

// storeCache stores a completion result in the cache.
func (cs *CompletionService) storeCache(key cacheKey, list *CompletionList, prefix string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Only cleanup when cache exceeds threshold (avoids O(n) on every store)
	if len(cs.cache) > cs.cacheCleanupCap {
		now := time.Now()
		for k, v := range cs.cache {
			if now.Sub(v.timestamp) > cs.cacheTimeout {
				delete(cs.cache, k)
			}
		}
	}

	cs.cache[key] = &cachedCompletion{
		list:      list,
		timestamp: time.Now(),
		prefix:    prefix,
	}
}

// processResults filters and sorts completion results.
func (cs *CompletionService) processResults(list *CompletionList, prefix string) *CompletionResult {
	if list == nil || len(list.Items) == 0 {
		return &CompletionResult{
			Items:                nil,
			IsIncomplete:         false,
			ServerIsIncomplete:   false,
			WasTruncatedByFilter: false,
		}
	}

	serverTotal := len(list.Items)
	items := list.Items

	// Filter by prefix
	if prefix != "" {
		items = FilterCompletions(items, prefix)
	}
	filteredCount := len(items)

	// Sort items
	items = SortCompletions(items, prefix)

	// Limit results
	truncatedByLimit := false
	if cs.maxResults > 0 && len(items) > cs.maxResults {
		items = items[:cs.maxResults]
		truncatedByLimit = true
	}

	return &CompletionResult{
		Items:                items,
		IsIncomplete:         list.IsIncomplete || truncatedByLimit,
		ServerIsIncomplete:   list.IsIncomplete,
		WasTruncatedByFilter: filteredCount < serverTotal,
		WasTruncatedByLimit:  truncatedByLimit,
		ServerTotalCount:     serverTotal,
		FilteredCount:        filteredCount,
	}
}

// CompletionResult is an enhanced completion result with metadata.
type CompletionResult struct {
	Items []CompletionItem

	// IsIncomplete is true if results are incomplete (from server or truncation)
	IsIncomplete bool

	// ServerIsIncomplete indicates the server reported incomplete results
	ServerIsIncomplete bool

	// WasTruncatedByFilter indicates filtering reduced results
	WasTruncatedByFilter bool

	// WasTruncatedByLimit indicates maxResults limit was applied
	WasTruncatedByLimit bool

	// ServerTotalCount is the number of items returned by the server
	ServerTotalCount int

	// FilteredCount is the number of items after filtering (before limit)
	FilteredCount int
}

// TotalCount returns the server's total count for backwards compatibility.
func (r *CompletionResult) TotalCount() int {
	return r.ServerTotalCount
}

// FilterCompletions filters completion items by prefix using fuzzy matching.
// FuzzyMatch handles case-insensitive matching internally.
func FilterCompletions(items []CompletionItem, prefix string) []CompletionItem {
	if prefix == "" {
		return items
	}

	var filtered []CompletionItem

	for _, item := range items {
		// Use FilterText if available, otherwise Label
		text := item.FilterText
		if text == "" {
			text = item.Label
		}

		if FuzzyMatch(text, prefix) {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// SortCompletions sorts completion items for optimal presentation.
func SortCompletions(items []CompletionItem, prefix string) []CompletionItem {
	if len(items) <= 1 {
		return items
	}

	prefixLower := strings.ToLower(prefix)

	// Create a copy to avoid mutating the original
	sorted := make([]CompletionItem, len(items))
	copy(sorted, items)

	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]

		// 1. Preselected items first
		if a.Preselect != b.Preselect {
			return a.Preselect
		}

		// 2. Exact prefix matches first
		if prefixLower != "" {
			aPrefix := strings.HasPrefix(strings.ToLower(a.Label), prefixLower)
			bPrefix := strings.HasPrefix(strings.ToLower(b.Label), prefixLower)
			if aPrefix != bPrefix {
				return aPrefix
			}
		}

		// 3. By kind priority (methods/functions over keywords/text)
		aPriority := completionKindPriority(a.Kind)
		bPriority := completionKindPriority(b.Kind)
		if aPriority != bPriority {
			return aPriority < bPriority
		}

		// 4. By sort text (case-insensitive alphabetically)
		sortA := a.SortText
		if sortA == "" {
			sortA = a.Label
		}
		sortB := b.SortText
		if sortB == "" {
			sortB = b.Label
		}
		return strings.ToLower(sortA) < strings.ToLower(sortB)
	})

	return sorted
}

// completionKindPriority returns priority for sorting (lower = higher priority).
func completionKindPriority(k CompletionItemKind) int {
	switch k {
	case CompletionItemKindMethod, CompletionItemKindFunction:
		return 1
	case CompletionItemKindField, CompletionItemKindVariable:
		return 2
	case CompletionItemKindClass, CompletionItemKindStruct, CompletionItemKindInterface:
		return 3
	case CompletionItemKindConstant:
		return 4
	case CompletionItemKindKeyword:
		return 5
	case CompletionItemKindSnippet:
		return 6
	case CompletionItemKindText:
		return 10
	default:
		return 7
	}
}

// FuzzyMatch returns true if text matches the pattern using fuzzy matching.
// Both text and pattern should be lowercase for case-insensitive matching.
func FuzzyMatch(text, pattern string) bool {
	if pattern == "" {
		return true
	}

	textLower := strings.ToLower(text)
	patternLower := strings.ToLower(pattern)

	// First check for substring match
	if strings.Contains(textLower, patternLower) {
		return true
	}

	// Then check for fuzzy character matching using runes
	textRunes := []rune(textLower)
	patternRunes := []rune(patternLower)

	ti := 0
	for pi := 0; pi < len(patternRunes) && ti < len(textRunes); pi++ {
		for ti < len(textRunes) && textRunes[ti] != patternRunes[pi] {
			ti++
		}
		if ti >= len(textRunes) {
			return false
		}
		ti++
	}

	return true
}

// FuzzyScore returns a score indicating how well text matches the pattern.
// Higher scores indicate better matches.
func FuzzyScore(text, pattern string) int {
	if pattern == "" {
		return 0
	}

	textLower := strings.ToLower(text)
	patternLower := strings.ToLower(pattern)

	score := 0

	// Exact match gets highest score
	if textLower == patternLower {
		return 1000
	}

	// Prefix match gets high score
	if strings.HasPrefix(textLower, patternLower) {
		score += 500
	}

	// Contains gets medium score
	if strings.Contains(textLower, patternLower) {
		score += 200
	}

	// Check for camelCase/snake_case boundary matches
	if matchesBoundaries(text, pattern) {
		score += 300
	}

	// Consecutive character matches get bonus (using runes)
	textRunes := []rune(textLower)
	patternRunes := []rune(patternLower)
	consecutiveBonus := 0
	ti := 0
	for pi := 0; pi < len(patternRunes) && ti < len(textRunes); pi++ {
		for ti < len(textRunes) && textRunes[ti] != patternRunes[pi] {
			ti++
			consecutiveBonus = 0
		}
		if ti < len(textRunes) {
			score += 10 + consecutiveBonus
			consecutiveBonus += 5
			ti++
		}
	}

	// Penalty for length difference (in runes)
	lenDiff := len(textRunes) - len(patternRunes)
	if lenDiff > 0 {
		score -= lenDiff * 2
	}

	return score
}

// matchesBoundaries checks if pattern matches word boundaries in text.
func matchesBoundaries(text, pattern string) bool {
	if pattern == "" {
		return true
	}

	boundaries := extractBoundaries(text)
	if len(boundaries) == 0 {
		return false
	}

	patternRunes := []rune(strings.ToLower(pattern))
	pi := 0
	for _, b := range boundaries {
		if pi < len(patternRunes) && unicode.ToLower(b) == patternRunes[pi] {
			pi++
		}
	}

	return pi == len(patternRunes)
}

// extractBoundaries extracts word boundary runes from text.
func extractBoundaries(text string) []rune {
	if text == "" {
		return nil
	}

	runes := []rune(text)
	var boundaries []rune
	boundaries = append(boundaries, runes[0])

	for i := 1; i < len(runes); i++ {
		c := runes[i]
		prev := runes[i-1]

		// Skip underscore
		if c == '_' {
			continue
		}
		// After underscore is a boundary
		if prev == '_' {
			boundaries = append(boundaries, c)
			continue
		}

		// CamelCase boundary: lowercase followed by uppercase
		if unicode.IsUpper(c) && unicode.IsLower(prev) {
			boundaries = append(boundaries, c)
		}
	}

	return boundaries
}

// CompletionItemKindString returns a human-readable name for a completion item kind.
func CompletionItemKindString(kind CompletionItemKind) string {
	switch kind {
	case CompletionItemKindText:
		return "Text"
	case CompletionItemKindMethod:
		return "Method"
	case CompletionItemKindFunction:
		return "Function"
	case CompletionItemKindConstructor:
		return "Constructor"
	case CompletionItemKindField:
		return "Field"
	case CompletionItemKindVariable:
		return "Variable"
	case CompletionItemKindClass:
		return "Class"
	case CompletionItemKindInterface:
		return "Interface"
	case CompletionItemKindModule:
		return "Module"
	case CompletionItemKindProperty:
		return "Property"
	case CompletionItemKindUnit:
		return "Unit"
	case CompletionItemKindValue:
		return "Value"
	case CompletionItemKindEnum:
		return "Enum"
	case CompletionItemKindKeyword:
		return "Keyword"
	case CompletionItemKindSnippet:
		return "Snippet"
	case CompletionItemKindColor:
		return "Color"
	case CompletionItemKindFile:
		return "File"
	case CompletionItemKindReference:
		return "Reference"
	case CompletionItemKindFolder:
		return "Folder"
	case CompletionItemKindEnumMember:
		return "EnumMember"
	case CompletionItemKindConstant:
		return "Constant"
	case CompletionItemKindStruct:
		return "Struct"
	case CompletionItemKindEvent:
		return "Event"
	case CompletionItemKindOperator:
		return "Operator"
	case CompletionItemKindTypeParameter:
		return "TypeParameter"
	default:
		return "Unknown"
	}
}

// CompletionItemKindIcon returns a single character icon for a completion item kind.
func CompletionItemKindIcon(kind CompletionItemKind) string {
	switch kind {
	case CompletionItemKindText:
		return "T"
	case CompletionItemKindMethod:
		return "m"
	case CompletionItemKindFunction:
		return "f"
	case CompletionItemKindConstructor:
		return "c"
	case CompletionItemKindField:
		return "F"
	case CompletionItemKindVariable:
		return "v"
	case CompletionItemKindClass:
		return "C"
	case CompletionItemKindInterface:
		return "I"
	case CompletionItemKindModule:
		return "M"
	case CompletionItemKindProperty:
		return "p"
	case CompletionItemKindUnit:
		return "U"
	case CompletionItemKindValue:
		return "V"
	case CompletionItemKindEnum:
		return "E"
	case CompletionItemKindKeyword:
		return "k"
	case CompletionItemKindSnippet:
		return "s"
	case CompletionItemKindColor:
		return "#"
	case CompletionItemKindFile:
		return "f"
	case CompletionItemKindReference:
		return "r"
	case CompletionItemKindFolder:
		return "D"
	case CompletionItemKindEnumMember:
		return "e"
	case CompletionItemKindConstant:
		return "K"
	case CompletionItemKindStruct:
		return "S"
	case CompletionItemKindEvent:
		return "E"
	case CompletionItemKindOperator:
		return "o"
	case CompletionItemKindTypeParameter:
		return "t"
	default:
		return "?"
	}
}

// GetInsertText returns the text to insert for a completion item.
func GetInsertText(item CompletionItem) string {
	// Prefer TextEdit if available
	if item.TextEdit != nil {
		return item.TextEdit.NewText
	}

	// Fall back to InsertText
	if item.InsertText != "" {
		return item.InsertText
	}

	// Finally use Label
	return item.Label
}

// IsSnippet returns true if the completion item uses snippet syntax.
func IsSnippet(item CompletionItem) bool {
	return item.InsertTextFormat == InsertTextFormatSnippet
}

// ExpandSnippet expands snippet placeholders to plain text.
// This is a minimal implementation that handles common cases:
// - $N tabstops are removed
// - ${N:default} placeholders use the default value
//
// Limitations (by design for simplicity):
// - Does not handle escaped dollar signs ($$)
// - Does not handle nested placeholders like ${1:${2:default}}
// - Does not handle choice syntax ${1|one,two,three|}
// - Does not handle variables like $TM_FILENAME
// For full snippet support, consider using a dedicated snippet library.
func ExpandSnippet(snippet string) string {
	var result strings.Builder
	runes := []rune(snippet)
	i := 0

	for i < len(runes) {
		if runes[i] == '$' {
			if i+1 < len(runes) {
				if runes[i+1] == '{' {
					// Handle ${...}
					end := -1
					for j := i + 2; j < len(runes); j++ {
						if runes[j] == '}' {
							end = j
							break
						}
					}
					if end != -1 {
						// Extract content between ${ and }
						content := string(runes[i+2 : end])
						if colonIdx := strings.Index(content, ":"); colonIdx != -1 {
							// Has default value
							result.WriteString(content[colonIdx+1:])
						}
						i = end + 1
						continue
					}
				} else if runes[i+1] >= '0' && runes[i+1] <= '9' {
					// Handle $N - skip the $ and all following digits
					i += 2
					for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
						i++
					}
					continue
				}
			}
		}
		result.WriteRune(runes[i])
		i++
	}
	return result.String()
}
