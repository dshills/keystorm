// Package search provides handlers for search and replace operations.
//
// This package implements Vim-style search functionality including:
//   - Forward search (/)
//   - Backward search (?)
//   - Find next match (n)
//   - Find previous match (N)
//   - Word under cursor search (* and #)
//   - Search and replace (:s and :%s)
//
// The search handler supports regular expressions and maintains
// search history for repeat operations.
package search
