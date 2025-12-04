package fuzzy

import (
	"container/heap"
	"context"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// AsyncMatcher provides async fuzzy matching for large item sets.
// It uses worker pools to parallelize matching across multiple CPU cores.
type AsyncMatcher struct {
	matcher    *Matcher
	numWorkers int
}

// NewAsyncMatcher creates an async matcher with the given base matcher.
// If numWorkers is 0, it defaults to runtime.NumCPU().
// Panics if matcher is nil.
func NewAsyncMatcher(matcher *Matcher, numWorkers int) *AsyncMatcher {
	if matcher == nil {
		panic("fuzzy: NewAsyncMatcher called with nil matcher")
	}
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	return &AsyncMatcher{
		matcher:    matcher,
		numWorkers: numWorkers,
	}
}

// MatchAsync performs fuzzy matching asynchronously.
// Returns a channel that receives results as they are found.
//
// IMPORTANT: The caller MUST either:
//   - Drain the results channel completely, OR
//   - Call the returned cancel function to release resources
//
// Failure to do either may cause goroutine leaks.
// Results are sent in score order (highest first).
func (m *AsyncMatcher) MatchAsync(ctx context.Context, query string, items []Item, limit int) (<-chan Result, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	results := make(chan Result, 100)

	go func() {
		defer close(results)
		m.matchParallel(ctx, query, items, limit, results)
	}()

	return results, cancel
}

// MatchParallel performs parallel matching and returns all results.
// This is useful when you need all results at once but want parallel processing.
// Uses a top-k heap per worker for efficient memory usage with large item sets.
func (m *AsyncMatcher) MatchParallel(ctx context.Context, query string, items []Item, limit int) []Result {
	// Normalize query
	if !m.matcher.options.CaseSensitive {
		query = strings.ToLower(query)
	}
	query = strings.TrimSpace(query)

	if query == "" {
		return m.matcher.emptyQueryResults(items, limit)
	}

	queryRunes := []rune(query)

	// Calculate adaptive chunk size based on items and workers
	chunkSize := (len(items) + m.numWorkers - 1) / m.numWorkers
	minChunkSize := 50 // Lower minimum for better parallelism
	if len(items) < 1000 {
		minChunkSize = 10 // Even smaller for small datasets
	}
	if chunkSize < minChunkSize {
		chunkSize = minChunkSize
	}

	var wg sync.WaitGroup
	resultChan := make(chan []Result, m.numWorkers)

	// Determine per-worker limit for top-k optimization
	// Each worker keeps at most 2x the limit to allow for merging
	workerLimit := limit
	if workerLimit > 0 {
		workerLimit = limit * 2
	}

	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}

		wg.Add(1)
		go func(chunk []Item) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			// Use top-k heap if limit is specified
			var chunkResults []Result
			if workerLimit > 0 {
				chunkResults = m.matchChunkTopK(ctx, queryRunes, chunk, workerLimit)
			} else {
				chunkResults = m.matchChunkAll(ctx, queryRunes, chunk)
			}

			select {
			case resultChan <- chunkResults:
			case <-ctx.Done():
			}
		}(items[i:end])
	}

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results from workers
	var allResults []Result
	for chunk := range resultChan {
		allResults = append(allResults, chunk...)
	}

	// Sort by score descending, then by text
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].Score != allResults[j].Score {
			return allResults[i].Score > allResults[j].Score
		}
		return allResults[i].Item.Text < allResults[j].Item.Text
	})

	// Apply limit
	if limit > 0 && len(allResults) > limit {
		allResults = allResults[:limit]
	}

	return allResults
}

// matchChunkTopK matches items in a chunk and keeps only top-k results.
func (m *AsyncMatcher) matchChunkTopK(ctx context.Context, queryRunes []rune, chunk []Item, k int) []Result {
	h := &resultHeap{}
	heap.Init(h)

	for _, item := range chunk {
		select {
		case <-ctx.Done():
			return h.toSlice()
		default:
		}

		score, matches := m.matcher.matchItem(queryRunes, item.Text)
		if score > m.matcher.options.MinScore {
			if h.Len() < k {
				heap.Push(h, Result{
					Item:    item,
					Score:   score,
					Matches: matches,
				})
			} else if score > (*h)[0].Score {
				// Replace minimum if new score is better
				(*h)[0] = Result{
					Item:    item,
					Score:   score,
					Matches: matches,
				}
				heap.Fix(h, 0)
			}
		}
	}

	return h.toSlice()
}

// matchChunkAll matches all items in a chunk (no limit).
func (m *AsyncMatcher) matchChunkAll(ctx context.Context, queryRunes []rune, chunk []Item) []Result {
	results := make([]Result, 0, len(chunk)/4)

	for _, item := range chunk {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		score, matches := m.matcher.matchItem(queryRunes, item.Text)
		if score > m.matcher.options.MinScore {
			results = append(results, Result{
				Item:    item,
				Score:   score,
				Matches: matches,
			})
		}
	}

	return results
}

// matchParallel performs the parallel matching and sends results to channel.
func (m *AsyncMatcher) matchParallel(ctx context.Context, query string, items []Item, limit int, results chan<- Result) {
	// Normalize query
	if !m.matcher.options.CaseSensitive {
		query = strings.ToLower(query)
	}
	query = strings.TrimSpace(query)

	if query == "" {
		for i, item := range items {
			if limit > 0 && i >= limit {
				break
			}
			select {
			case results <- Result{Item: item, Score: 0}:
			case <-ctx.Done():
				return
			}
		}
		return
	}

	queryRunes := []rune(query)

	// Collect results using top-k optimization
	collected := m.collectResultsTopK(ctx, queryRunes, items, limit)

	// Sort and send results
	sort.Slice(collected, func(i, j int) bool {
		if collected[i].Score != collected[j].Score {
			return collected[i].Score > collected[j].Score
		}
		return collected[i].Item.Text < collected[j].Item.Text
	})

	sent := 0
	for _, r := range collected {
		if limit > 0 && sent >= limit {
			break
		}
		select {
		case results <- r:
			sent++
		case <-ctx.Done():
			return
		}
	}
}

// collectResultsTopK gathers results from parallel workers using top-k heaps.
func (m *AsyncMatcher) collectResultsTopK(ctx context.Context, queryRunes []rune, items []Item, limit int) []Result {
	// Calculate adaptive chunk size
	chunkSize := (len(items) + m.numWorkers - 1) / m.numWorkers
	minChunkSize := 50
	if len(items) < 1000 {
		minChunkSize = 10
	}
	if chunkSize < minChunkSize {
		chunkSize = minChunkSize
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var collected []Result

	// Per-worker limit
	workerLimit := limit
	if workerLimit > 0 {
		workerLimit = limit * 2
	}

	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}

		wg.Add(1)
		go func(chunk []Item) {
			defer wg.Done()

			var localResults []Result
			if workerLimit > 0 {
				localResults = m.matchChunkTopK(ctx, queryRunes, chunk, workerLimit)
			} else {
				localResults = m.matchChunkAll(ctx, queryRunes, chunk)
			}

			mu.Lock()
			collected = append(collected, localResults...)
			mu.Unlock()
		}(items[i:end])
	}

	wg.Wait()
	return collected
}

// resultHeap is a min-heap of Results by score (for top-k selection).
type resultHeap []Result

func (h resultHeap) Len() int           { return len(h) }
func (h resultHeap) Less(i, j int) bool { return h[i].Score < h[j].Score } // Min-heap by score
func (h resultHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *resultHeap) Push(x any) {
	*h = append(*h, x.(Result)) //nolint:errcheck // heap.Interface requires any; we only push Result
}

func (h *resultHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *resultHeap) toSlice() []Result {
	result := make([]Result, len(*h))
	copy(result, *h)
	return result
}

// StreamingMatcher provides incremental results as the user types.
// It maintains state across queries and can cancel previous searches.
type StreamingMatcher struct {
	matcher   *AsyncMatcher
	cancel    context.CancelFunc
	mu        sync.Mutex
	lastQuery string
}

// NewStreamingMatcher creates a streaming matcher.
// Panics if matcher is nil.
func NewStreamingMatcher(matcher *Matcher) *StreamingMatcher {
	if matcher == nil {
		panic("fuzzy: NewStreamingMatcher called with nil matcher")
	}
	return &StreamingMatcher{
		matcher: NewAsyncMatcher(matcher, 0),
	}
}

// Search starts a new search, canceling any previous search.
// Returns a channel that receives results.
// Uses context.Background() internally; use SearchWithContext for custom context.
func (m *StreamingMatcher) Search(query string, items []Item, limit int) <-chan Result {
	return m.SearchWithContext(context.Background(), query, items, limit)
}

// SearchWithContext starts a new search with a custom context.
// The provided context is used in addition to internal cancellation.
// Canceling any previous search before starting the new one.
func (m *StreamingMatcher) SearchWithContext(ctx context.Context, query string, items []Item, limit int) <-chan Result {
	m.mu.Lock()

	// Cancel previous search
	if m.cancel != nil {
		m.cancel()
	}

	m.lastQuery = query
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	results, _ := m.matcher.MatchAsync(ctx, query, items, limit)
	return results
}

// Cancel stops the current search.
func (m *StreamingMatcher) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

// LastQuery returns the most recent query string.
func (m *StreamingMatcher) LastQuery() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastQuery
}
