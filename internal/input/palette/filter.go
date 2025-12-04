package palette

import (
	"sort"
	"strings"
	"unicode"
)

// SearchResult represents a matched command with scoring information.
type SearchResult struct {
	// Command is the matched command.
	Command *Command

	// Score is the match score (higher is better).
	Score int

	// Matches contains the rune indices of matched characters in the title.
	Matches []int
}

// Filter handles command search and filtering logic.
type Filter struct {
	// MinScore is the minimum score for a match to be included.
	// Default is 0 (include all matches).
	MinScore int
}

// NewFilter creates a new filter with default settings.
func NewFilter() *Filter {
	return &Filter{
		MinScore: 0,
	}
}

// Search finds commands matching the query using fuzzy matching.
// Results are sorted by score (descending), with ties broken by title.
func (f *Filter) Search(commands []*Command, query string, limit int) []SearchResult {
	// Normalize query once at entry point
	query = strings.TrimSpace(strings.ToLower(query))

	if query == "" {
		// Return all commands (caller handles sorting by history)
		results := make([]SearchResult, 0, len(commands))
		for _, cmd := range commands {
			results = append(results, SearchResult{
				Command: cmd,
				Score:   0,
			})
		}
		if limit > 0 && len(results) > limit {
			results = results[:limit]
		}
		return results
	}

	queryRunes := []rune(query)
	results := make([]SearchResult, 0, len(commands))

	for _, cmd := range commands {
		score, matches := f.matchCommand(queryRunes, cmd)
		if score > f.MinScore {
			results = append(results, SearchResult{
				Command: cmd,
				Score:   score,
				Matches: matches,
			})
		}
	}

	// Sort by score descending, then by title for deterministic ordering
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Command.Title < results[j].Command.Title
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// matchCommand scores a command against the query.
// Returns score and matched character indices (rune indices).
func (f *Filter) matchCommand(queryRunes []rune, cmd *Command) (int, []int) {
	// Try matching against title first (higher weight)
	titleScore, titleMatches := f.fuzzyMatch(queryRunes, cmd.Title)
	if titleScore > 0 {
		// Boost title matches
		return titleScore + 50, titleMatches
	}

	// Try matching against ID
	idScore, idMatches := f.fuzzyMatch(queryRunes, cmd.ID)
	if idScore > 0 {
		return idScore + 25, idMatches
	}

	// Try matching against description
	descScore, descMatches := f.fuzzyMatch(queryRunes, cmd.Description)
	if descScore > 0 {
		return descScore, descMatches
	}

	// Try matching against category
	catScore, catMatches := f.fuzzyMatch(queryRunes, cmd.Category)
	if catScore > 0 {
		return catScore, catMatches
	}

	return 0, nil
}

// fuzzyMatch performs fuzzy string matching using runes for proper UTF-8 support.
// Returns score and match indices (rune indices, not byte indices).
func (f *Filter) fuzzyMatch(queryRunes []rune, text string) (int, []int) {
	if text == "" || len(queryRunes) == 0 {
		return 0, nil
	}

	// Convert text to runes for proper UTF-8 handling
	textRunes := []rune(strings.ToLower(text))
	originalRunes := []rune(text) // Keep original case for boundary detection

	matches := make([]int, 0, len(queryRunes))
	queryIdx := 0

	for i := 0; i < len(textRunes) && queryIdx < len(queryRunes); i++ {
		if textRunes[i] == queryRunes[queryIdx] {
			matches = append(matches, i)
			queryIdx++
		}
	}

	// All query characters must match
	if queryIdx != len(queryRunes) {
		return 0, nil
	}

	// Calculate score using rune-based operations
	score := f.calculateScore(queryRunes, originalRunes, textRunes, matches)
	return score, matches
}

// calculateScore computes a match score based on various factors.
// All indices and lengths are rune-based for proper UTF-8 support.
func (f *Filter) calculateScore(queryRunes, originalRunes, textRunes []rune, matches []int) int {
	if len(matches) == 0 {
		return 0
	}

	score := 100 // Base score for matching

	// Bonus for consecutive matches
	consecutiveBonus := 0
	for i := 1; i < len(matches); i++ {
		if matches[i] == matches[i-1]+1 {
			consecutiveBonus += 20
		}
	}
	score += consecutiveBonus

	// Bonus for matches at word boundaries
	wordBoundaryBonus := 0
	for _, idx := range matches {
		if f.isWordBoundaryRunes(originalRunes, idx) {
			wordBoundaryBonus += 15
		}
	}
	score += wordBoundaryBonus

	// Bonus for prefix match
	if matches[0] == 0 {
		score += 25
	}

	// Penalty for gaps between matches
	if len(matches) > 1 {
		totalGap := matches[len(matches)-1] - matches[0] - len(matches) + 1
		if totalGap > 0 {
			score -= totalGap * 2
		}
	}

	// Penalty for matches far from start
	if matches[0] > 0 {
		score -= matches[0]
	}

	// Bonus for shorter text (more specific match) - use rune count
	textLen := len(textRunes)
	if textLen < 20 {
		score += 20 - textLen
	}

	// Bonus for exact prefix match
	if len(textRunes) >= len(queryRunes) {
		isPrefix := true
		for i, qr := range queryRunes {
			if textRunes[i] != qr {
				isPrefix = false
				break
			}
		}
		if isPrefix {
			score += 50
		}
	}

	// Ensure minimum score of 1 for any match
	if score < 1 {
		score = 1
	}

	return score
}

// isWordBoundaryRunes checks if the rune at idx is at a word boundary.
// Uses rune slice for proper UTF-8 handling.
func (f *Filter) isWordBoundaryRunes(runes []rune, idx int) bool {
	if idx == 0 {
		return true
	}
	if idx >= len(runes) {
		return false
	}

	prevChar := runes[idx-1]
	currChar := runes[idx]

	// Word boundary conditions:
	// - After separator characters (including Unicode space)
	if unicode.IsSpace(prevChar) || unicode.IsPunct(prevChar) {
		return true
	}

	// - CamelCase boundary (lowercase followed by uppercase)
	if unicode.IsLower(prevChar) && unicode.IsUpper(currChar) {
		return true
	}

	return false
}

// FilterByCategory returns commands in the specified category.
func (f *Filter) FilterByCategory(commands []*Command, category string) []*Command {
	if category == "" {
		return commands
	}

	categoryLower := strings.ToLower(category)
	result := make([]*Command, 0, len(commands)/4)

	for _, cmd := range commands {
		if strings.ToLower(cmd.Category) == categoryLower {
			result = append(result, cmd)
		}
	}

	return result
}

// FilterBySource returns commands from the specified source.
func (f *Filter) FilterBySource(commands []*Command, source string) []*Command {
	if source == "" {
		return commands
	}

	result := make([]*Command, 0, len(commands)/4)

	for _, cmd := range commands {
		if cmd.Source == source {
			result = append(result, cmd)
		}
	}

	return result
}

// Categories returns all unique categories from the commands.
func Categories(commands []*Command) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, cmd := range commands {
		if cmd.Category != "" && !seen[cmd.Category] {
			seen[cmd.Category] = true
			result = append(result, cmd.Category)
		}
	}

	sort.Strings(result)
	return result
}
