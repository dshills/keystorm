package fuzzy

import "unicode"

// Scorer calculates match scores.
type Scorer interface {
	// Score calculates a match score based on various factors.
	// Higher scores indicate better matches.
	//
	// Parameters:
	//   - queryRunes: the normalized query runes
	//   - originalRunes: original text runes (preserves case)
	//   - textRunes: normalized text runes (lowercase if case-insensitive)
	//   - matches: rune indices of matched characters in text
	Score(queryRunes, originalRunes, textRunes []rune, matches []int) int
}

// DefaultScorer implements a comprehensive scoring algorithm.
type DefaultScorer struct{}

// Score implements the Scorer interface.
func (s DefaultScorer) Score(queryRunes, originalRunes, textRunes []rune, matches []int) int {
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
		if isWordBoundary(originalRunes, idx) {
			wordBoundaryBonus += 15
		}
	}
	score += wordBoundaryBonus

	// Bonus for prefix match (first match at position 0)
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

	// Bonus for shorter text (more specific match)
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

// isWordBoundary checks if the rune at idx is at a word boundary.
func isWordBoundary(runes []rune, idx int) bool {
	if idx == 0 {
		return true
	}
	if idx >= len(runes) {
		return false
	}

	prevChar := runes[idx-1]
	currChar := runes[idx]

	// Word boundary conditions:
	// - After separator characters (including Unicode space/punct)
	if unicode.IsSpace(prevChar) || unicode.IsPunct(prevChar) {
		return true
	}

	// - CamelCase boundary (lowercase followed by uppercase)
	if unicode.IsLower(prevChar) && unicode.IsUpper(currChar) {
		return true
	}

	// - snake_case/kebab-case boundary (handled by punct check above)
	// - Path separator (handled by punct check above)

	return false
}

// WeightedScorer allows customizing scoring weights.
type WeightedScorer struct {
	// BaseScore is the starting score for any match.
	BaseScore int

	// ConsecutiveBonus is added for each consecutive character match.
	ConsecutiveBonus int

	// WordBoundaryBonus is added for matches at word boundaries.
	WordBoundaryBonus int

	// PrefixBonus is added when the first match is at position 0.
	PrefixBonus int

	// ExactPrefixBonus is added when query matches the start of text exactly.
	ExactPrefixBonus int

	// GapPenalty is subtracted for each gap character between matches.
	GapPenalty int

	// LeadingPenalty is subtracted for each character before first match.
	LeadingPenalty int

	// LengthBonus is added for shorter texts (up to this threshold).
	LengthBonusThreshold int
}

// DefaultWeights returns the default scoring weights.
func DefaultWeights() WeightedScorer {
	return WeightedScorer{
		BaseScore:            100,
		ConsecutiveBonus:     20,
		WordBoundaryBonus:    15,
		PrefixBonus:          25,
		ExactPrefixBonus:     50,
		GapPenalty:           2,
		LeadingPenalty:       1,
		LengthBonusThreshold: 20,
	}
}

// Score implements the Scorer interface with configurable weights.
func (s WeightedScorer) Score(queryRunes, originalRunes, textRunes []rune, matches []int) int {
	if len(matches) == 0 {
		return 0
	}

	score := s.BaseScore

	// Consecutive matches bonus
	for i := 1; i < len(matches); i++ {
		if matches[i] == matches[i-1]+1 {
			score += s.ConsecutiveBonus
		}
	}

	// Word boundary bonus
	for _, idx := range matches {
		if isWordBoundary(originalRunes, idx) {
			score += s.WordBoundaryBonus
		}
	}

	// Prefix bonus
	if matches[0] == 0 {
		score += s.PrefixBonus
	}

	// Gap penalty
	if len(matches) > 1 {
		totalGap := matches[len(matches)-1] - matches[0] - len(matches) + 1
		if totalGap > 0 {
			score -= totalGap * s.GapPenalty
		}
	}

	// Leading penalty
	if matches[0] > 0 {
		score -= matches[0] * s.LeadingPenalty
	}

	// Length bonus
	textLen := len(textRunes)
	if textLen < s.LengthBonusThreshold {
		score += s.LengthBonusThreshold - textLen
	}

	// Exact prefix bonus
	if len(textRunes) >= len(queryRunes) {
		isPrefix := true
		for i, qr := range queryRunes {
			if textRunes[i] != qr {
				isPrefix = false
				break
			}
		}
		if isPrefix {
			score += s.ExactPrefixBonus
		}
	}

	if score < 1 {
		score = 1
	}

	return score
}

// FilePathScorer is optimized for file path matching.
// It gives extra weight to filename matches over directory matches.
type FilePathScorer struct {
	base WeightedScorer
}

// NewFilePathScorer creates a scorer optimized for file paths.
func NewFilePathScorer() FilePathScorer {
	return FilePathScorer{
		base: WeightedScorer{
			BaseScore:            100,
			ConsecutiveBonus:     25, // Higher for paths
			WordBoundaryBonus:    20, // Path separators matter
			PrefixBonus:          15, // Less important for paths
			ExactPrefixBonus:     30,
			GapPenalty:           3,
			LeadingPenalty:       1,
			LengthBonusThreshold: 30, // Paths are longer
		},
	}
}

// Score implements the Scorer interface for file paths.
func (s FilePathScorer) Score(queryRunes, originalRunes, textRunes []rune, matches []int) int {
	baseScore := s.base.Score(queryRunes, originalRunes, textRunes, matches)

	// Find the last path separator
	lastSep := -1
	for i := len(originalRunes) - 1; i >= 0; i-- {
		if originalRunes[i] == '/' || originalRunes[i] == '\\' {
			lastSep = i
			break
		}
	}

	// Bonus if matches are in the filename (after last separator)
	if lastSep >= 0 && len(matches) > 0 {
		filenameMatches := 0
		for _, idx := range matches {
			if idx > lastSep {
				filenameMatches++
			}
		}
		// Bonus proportional to filename matches
		baseScore += filenameMatches * 10
	}

	return baseScore
}
