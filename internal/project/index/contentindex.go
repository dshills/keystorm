package index

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// ContentIndex provides full-text content indexing and search.
// It uses an inverted index for fast keyword lookup.
type ContentIndex struct {
	mu sync.RWMutex

	// Inverted index: term -> document IDs
	termIndex map[string]map[string][]int // term -> path -> line numbers

	// Document store: path -> content metadata
	documents map[string]*DocumentMeta

	// Configuration
	config ContentIndexConfig
}

// DocumentMeta holds metadata about an indexed document.
type DocumentMeta struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	LineCount int    `json:"line_count"`
	WordCount int    `json:"word_count"`
	Hash      string `json:"hash,omitempty"` // Content hash for change detection
}

// ContentIndexConfig configures the content index.
type ContentIndexConfig struct {
	// MinTermLength is the minimum term length to index
	MinTermLength int

	// MaxTermLength is the maximum term length to index
	MaxTermLength int

	// StopWords are common words to exclude from indexing
	StopWords map[string]bool

	// CaseSensitive determines if indexing is case-sensitive
	CaseSensitive bool
}

// DefaultContentIndexConfig returns the default configuration.
func DefaultContentIndexConfig() ContentIndexConfig {
	return ContentIndexConfig{
		MinTermLength: 2,
		MaxTermLength: 100,
		StopWords:     defaultStopWords(),
		CaseSensitive: false,
	}
}

// NewContentIndex creates a new content index.
func NewContentIndex(config ContentIndexConfig) *ContentIndex {
	return &ContentIndex{
		termIndex: make(map[string]map[string][]int),
		documents: make(map[string]*DocumentMeta),
		config:    config,
	}
}

// IndexDocument indexes a document's content.
func (ci *ContentIndex) IndexDocument(path string, content []byte) error {
	// Build term index outside the lock for better concurrency
	lines := splitIntoLines(content)
	wordCount := 0

	// Build local term -> line numbers mapping
	localTerms := make(map[string][]int)
	for lineNum, line := range lines {
		terms := ci.tokenize(line)
		wordCount += len(terms)

		for _, term := range terms {
			if ci.shouldIndexTerm(term) {
				normalizedTerm := ci.normalizeTerm(term)
				localTerms[normalizedTerm] = append(localTerms[normalizedTerm], lineNum+1) // 1-based
			}
		}
	}

	// Build document metadata
	docMeta := &DocumentMeta{
		Path:      path,
		Size:      int64(len(content)),
		LineCount: len(lines),
		WordCount: wordCount,
	}

	// Now acquire lock and merge into global index
	ci.mu.Lock()
	defer ci.mu.Unlock()

	// Remove existing index for this document
	ci.removeDocumentLocked(path)

	// Merge local terms into global index
	for term, lineNums := range localTerms {
		if ci.termIndex[term] == nil {
			ci.termIndex[term] = make(map[string][]int)
		}
		ci.termIndex[term][path] = lineNums
	}

	// Store document metadata
	ci.documents[path] = docMeta

	return nil
}

// RemoveDocument removes a document from the index.
func (ci *ContentIndex) RemoveDocument(path string) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	ci.removeDocumentLocked(path)
	return nil
}

// removeDocumentLocked removes a document (caller must hold lock).
func (ci *ContentIndex) removeDocumentLocked(path string) {
	// Remove from term index
	for term, docs := range ci.termIndex {
		delete(docs, path)
		if len(docs) == 0 {
			delete(ci.termIndex, term)
		}
	}

	// Remove document metadata
	delete(ci.documents, path)
}

// Search searches for terms in the index.
func (ci *ContentIndex) Search(ctx context.Context, query string, opts ContentSearchOptions) ([]ContentSearchResult, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if query == "" {
		return nil, ErrInvalidQuery
	}

	// Tokenize query
	queryTerms := ci.tokenize(query)
	if len(queryTerms) == 0 {
		return nil, nil
	}

	// Find documents matching all terms
	var matchingPaths []string
	if opts.MatchAll {
		matchingPaths = ci.findMatchingAll(queryTerms)
	} else {
		matchingPaths = ci.findMatchingAny(queryTerms)
	}

	// Build results
	var results []ContentSearchResult
	for _, path := range matchingPaths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Apply filters
		if !ci.matchesFilters(path, opts) {
			continue
		}

		// Get line numbers for matching terms
		lineNumbers := make(map[int]bool)
		for _, term := range queryTerms {
			normalizedTerm := ci.normalizeTerm(term)
			if docs, ok := ci.termIndex[normalizedTerm]; ok {
				if lines, ok := docs[path]; ok {
					for _, line := range lines {
						lineNumbers[line] = true
					}
				}
			}
		}

		// Convert to slice and sort
		lines := make([]int, 0, len(lineNumbers))
		for line := range lineNumbers {
			lines = append(lines, line)
		}
		sort.Ints(lines)

		doc := ci.documents[path]
		results = append(results, ContentSearchResult{
			Path:        path,
			LineNumbers: lines,
			Score:       ci.scoreMatch(path, queryTerms),
			DocumentMeta: DocumentMeta{
				Path:      doc.Path,
				Size:      doc.Size,
				LineCount: doc.LineCount,
				WordCount: doc.WordCount,
			},
		})

		if opts.MaxResults > 0 && len(results) >= opts.MaxResults {
			break
		}
	}

	return results, nil
}

// SearchRegex searches using a regular expression.
// Pattern length is limited to prevent resource exhaustion during compilation.
func (ci *ContentIndex) SearchRegex(ctx context.Context, pattern string, opts ContentSearchOptions) ([]ContentSearchResult, error) {
	// Validate pattern length to prevent resource exhaustion
	if len(pattern) > MaxRegexPatternLength {
		return nil, ErrPatternTooLong
	}

	ci.mu.RLock()
	defer ci.mu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	var results []ContentSearchResult

	for path := range ci.documents {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Apply filters
		if !ci.matchesFilters(path, opts) {
			continue
		}

		// Find matching terms
		var matchingLines []int
		for term, docs := range ci.termIndex {
			if re.MatchString(term) {
				if lines, ok := docs[path]; ok {
					matchingLines = append(matchingLines, lines...)
				}
			}
		}

		if len(matchingLines) > 0 {
			doc := ci.documents[path]
			results = append(results, ContentSearchResult{
				Path:        path,
				LineNumbers: dedupInts(matchingLines),
				Score:       float64(len(matchingLines)),
				DocumentMeta: DocumentMeta{
					Path:      doc.Path,
					Size:      doc.Size,
					LineCount: doc.LineCount,
					WordCount: doc.WordCount,
				},
			})
		}

		if opts.MaxResults > 0 && len(results) >= opts.MaxResults {
			break
		}
	}

	return results, nil
}

// HasDocument checks if a document is indexed.
func (ci *ContentIndex) HasDocument(path string) bool {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	_, ok := ci.documents[path]
	return ok
}

// GetDocument returns document metadata.
func (ci *ContentIndex) GetDocument(path string) (DocumentMeta, bool) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	doc, ok := ci.documents[path]
	if !ok {
		return DocumentMeta{}, false
	}
	return *doc, true
}

// DocumentCount returns the number of indexed documents.
func (ci *ContentIndex) DocumentCount() int {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	return len(ci.documents)
}

// TermCount returns the number of unique terms in the index.
func (ci *ContentIndex) TermCount() int {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	return len(ci.termIndex)
}

// Clear removes all indexed content.
func (ci *ContentIndex) Clear() {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	ci.termIndex = make(map[string]map[string][]int)
	ci.documents = make(map[string]*DocumentMeta)
}

// Save persists the index to a writer.
func (ci *ContentIndex) Save(w io.Writer) error {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	data := struct {
		TermIndex map[string]map[string][]int `json:"term_index"`
		Documents map[string]*DocumentMeta    `json:"documents"`
	}{
		TermIndex: ci.termIndex,
		Documents: ci.documents,
	}

	encoder := json.NewEncoder(w)
	return encoder.Encode(data)
}

// Load restores the index from a reader.
func (ci *ContentIndex) Load(r io.Reader) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	var data struct {
		TermIndex map[string]map[string][]int `json:"term_index"`
		Documents map[string]*DocumentMeta    `json:"documents"`
	}

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&data); err != nil {
		return err
	}

	ci.termIndex = data.TermIndex
	ci.documents = data.Documents

	if ci.termIndex == nil {
		ci.termIndex = make(map[string]map[string][]int)
	}
	if ci.documents == nil {
		ci.documents = make(map[string]*DocumentMeta)
	}

	return nil
}

// tokenize splits text into terms.
func (ci *ContentIndex) tokenize(text string) []string {
	var terms []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				terms = append(terms, current.String())
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		terms = append(terms, current.String())
	}

	return terms
}

// shouldIndexTerm checks if a term should be indexed.
func (ci *ContentIndex) shouldIndexTerm(term string) bool {
	if len(term) < ci.config.MinTermLength {
		return false
	}
	if ci.config.MaxTermLength > 0 && len(term) > ci.config.MaxTermLength {
		return false
	}

	normalizedTerm := ci.normalizeTerm(term)
	if ci.config.StopWords[normalizedTerm] {
		return false
	}

	return true
}

// normalizeTerm normalizes a term for indexing.
func (ci *ContentIndex) normalizeTerm(term string) string {
	if ci.config.CaseSensitive {
		return term
	}
	return strings.ToLower(term)
}

// findMatchingAll finds documents containing all terms.
func (ci *ContentIndex) findMatchingAll(terms []string) []string {
	if len(terms) == 0 {
		return nil
	}

	// Start with documents matching the first term
	firstTerm := ci.normalizeTerm(terms[0])
	docs, ok := ci.termIndex[firstTerm]
	if !ok {
		return nil
	}

	candidates := make(map[string]bool)
	for path := range docs {
		candidates[path] = true
	}

	// Filter by remaining terms
	for i := 1; i < len(terms); i++ {
		term := ci.normalizeTerm(terms[i])
		docs, ok := ci.termIndex[term]
		if !ok {
			return nil
		}

		// Keep only candidates that have this term
		for path := range candidates {
			if _, found := docs[path]; !found {
				delete(candidates, path)
			}
		}
	}

	result := make([]string, 0, len(candidates))
	for path := range candidates {
		result = append(result, path)
	}
	return result
}

// findMatchingAny finds documents containing any of the terms.
func (ci *ContentIndex) findMatchingAny(terms []string) []string {
	found := make(map[string]bool)

	for _, term := range terms {
		normalizedTerm := ci.normalizeTerm(term)
		if docs, ok := ci.termIndex[normalizedTerm]; ok {
			for path := range docs {
				found[path] = true
			}
		}
	}

	result := make([]string, 0, len(found))
	for path := range found {
		result = append(result, path)
	}
	return result
}

// scoreMatch calculates a relevance score for a match.
func (ci *ContentIndex) scoreMatch(path string, queryTerms []string) float64 {
	doc, ok := ci.documents[path]
	if !ok {
		return 0
	}

	// Score based on:
	// 1. Number of matching terms
	// 2. Term frequency (how many times terms appear)
	// 3. Document length (shorter docs score higher for same matches)

	matchingTerms := 0
	totalOccurrences := 0

	for _, term := range queryTerms {
		normalizedTerm := ci.normalizeTerm(term)
		if docs, ok := ci.termIndex[normalizedTerm]; ok {
			if lines, ok := docs[path]; ok {
				matchingTerms++
				totalOccurrences += len(lines)
			}
		}
	}

	if matchingTerms == 0 {
		return 0
	}

	// Term coverage: what fraction of query terms were found
	termCoverage := float64(matchingTerms) / float64(len(queryTerms))

	// Normalize by document length
	lengthNorm := 1.0
	if doc.WordCount > 0 {
		lengthNorm = 1.0 / (1.0 + float64(doc.WordCount)/1000.0)
	}

	return termCoverage * float64(totalOccurrences) * lengthNorm * 100
}

// matchesFilters checks if a path matches the search filters.
func (ci *ContentIndex) matchesFilters(path string, opts ContentSearchOptions) bool {
	// Check include patterns
	if len(opts.IncludePaths) > 0 {
		matched := false
		for _, pattern := range opts.IncludePaths {
			if strings.Contains(path, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check exclude patterns
	for _, pattern := range opts.ExcludePaths {
		if strings.Contains(path, pattern) {
			return false
		}
	}

	// Check file types
	if len(opts.FileTypes) > 0 {
		matched := false
		for _, ft := range opts.FileTypes {
			if strings.HasSuffix(path, ft) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// ContentSearchOptions configures content search.
type ContentSearchOptions struct {
	// MatchAll requires all terms to be present
	MatchAll bool

	// MaxResults limits the number of results
	MaxResults int

	// IncludePaths filters results to paths containing these substrings
	IncludePaths []string

	// ExcludePaths excludes results containing these substrings
	ExcludePaths []string

	// FileTypes filters by file extension
	FileTypes []string
}

// ContentSearchResult represents a content search match.
type ContentSearchResult struct {
	Path         string
	LineNumbers  []int
	Score        float64
	DocumentMeta DocumentMeta
}

// splitIntoLines splits content into lines.
func splitIntoLines(content []byte) []string {
	if len(content) == 0 {
		return []string{}
	}

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines
}

// dedupInts removes duplicates from a slice of ints.
func dedupInts(nums []int) []int {
	seen := make(map[int]bool)
	result := make([]int, 0, len(nums))

	for _, n := range nums {
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}

	return result
}

// defaultStopWords returns common stop words.
func defaultStopWords() map[string]bool {
	words := []string{
		"a", "an", "and", "are", "as", "at", "be", "by", "for",
		"from", "has", "he", "in", "is", "it", "its", "of", "on",
		"that", "the", "to", "was", "were", "will", "with",
		// Common programming keywords (we still want to index these, so commented out)
		// "if", "else", "for", "while", "return", "func", "function",
	}

	stopWords := make(map[string]bool)
	for _, w := range words {
		stopWords[w] = true
	}
	return stopWords
}
