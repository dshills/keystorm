package rope

// Chunk size constants control the granularity of text storage.
const (
	// MinChunkSize is the minimum bytes per chunk (except for the last chunk).
	MinChunkSize = 128

	// MaxChunkSize is the maximum bytes per chunk before splitting.
	MaxChunkSize = 256

	// TargetChunkSize is the preferred chunk size when building.
	TargetChunkSize = (MinChunkSize + MaxChunkSize) / 2
)

// Chunk represents a bounded string stored in leaf nodes.
// Chunks are immutable once created.
type Chunk struct {
	data    string      // The actual text (immutable)
	summary TextSummary // Precomputed metrics
}

// NewChunk creates a chunk from a string.
// Computes summary metrics eagerly.
func NewChunk(s string) Chunk {
	return Chunk{
		data:    s,
		summary: ComputeSummary(s),
	}
}

// String returns the chunk's text.
func (c Chunk) String() string {
	return c.data
}

// Summary returns the chunk's precomputed metrics.
func (c Chunk) Summary() TextSummary {
	return c.summary
}

// Len returns the byte length of the chunk.
func (c Chunk) Len() int {
	return len(c.data)
}

// IsEmpty returns true if the chunk contains no text.
func (c Chunk) IsEmpty() bool {
	return len(c.data) == 0
}

// Split splits a chunk at byte offset, returning two chunks.
// The offset must be at a valid UTF-8 boundary.
func (c Chunk) Split(offset int) (Chunk, Chunk) {
	if offset <= 0 {
		return Chunk{}, c
	}
	if offset >= len(c.data) {
		return c, Chunk{}
	}

	return NewChunk(c.data[:offset]), NewChunk(c.data[offset:])
}

// Append concatenates another chunk to this one, potentially returning
// multiple chunks if the result exceeds MaxChunkSize.
func (c Chunk) Append(other Chunk) []Chunk {
	if c.IsEmpty() {
		if other.IsEmpty() {
			return nil
		}
		return []Chunk{other}
	}
	if other.IsEmpty() {
		return []Chunk{c}
	}

	combined := c.data + other.data
	if len(combined) <= MaxChunkSize {
		return []Chunk{NewChunk(combined)}
	}

	// Need to split into multiple chunks
	return splitIntoChunks(combined)
}

// splitIntoChunks splits a string into chunks of appropriate size.
func splitIntoChunks(s string) []Chunk {
	if len(s) == 0 {
		return nil
	}
	if len(s) <= MaxChunkSize {
		return []Chunk{NewChunk(s)}
	}

	var chunks []Chunk
	remaining := s

	for len(remaining) > 0 {
		chunkSize := TargetChunkSize
		if len(remaining) <= MaxChunkSize {
			// Last chunk, take it all
			chunks = append(chunks, NewChunk(remaining))
			break
		}

		// Find a good split point (UTF-8 boundary)
		splitPoint := findUTF8Boundary(remaining, chunkSize)
		chunks = append(chunks, NewChunk(remaining[:splitPoint]))
		remaining = remaining[splitPoint:]
	}

	return chunks
}

// findUTF8Boundary finds a valid UTF-8 boundary near the target position.
// It prefers splitting after a newline if one exists nearby.
func findUTF8Boundary(s string, target int) int {
	if target >= len(s) {
		return len(s)
	}
	if target <= 0 {
		return 0
	}

	// Look for a newline near the target for a cleaner split
	searchStart := target - MinChunkSize/4
	if searchStart < 0 {
		searchStart = 0
	}
	searchEnd := target + MinChunkSize/4
	if searchEnd > len(s) {
		searchEnd = len(s)
	}

	// Prefer splitting after a newline
	for i := target; i < searchEnd; i++ {
		if s[i] == '\n' {
			return i + 1
		}
	}
	for i := target - 1; i >= searchStart; i-- {
		if s[i] == '\n' {
			return i + 1
		}
	}

	// No newline found, just ensure UTF-8 boundary
	// Move forward until we're at a valid boundary
	pos := target
	for pos < len(s) && !isUTF8Start(s[pos]) {
		pos++
	}

	// If we went too far forward, try backward
	if pos > target+4 || pos >= len(s) {
		pos = target
		for pos > 0 && !isUTF8Start(s[pos]) {
			pos--
		}
	}

	return pos
}

// isUTF8Start returns true if the byte is the start of a UTF-8 sequence.
func isUTF8Start(b byte) bool {
	// In UTF-8, continuation bytes start with 10xxxxxx (0x80-0xBF)
	// Start bytes are either ASCII (0x00-0x7F) or multi-byte starts (0xC0-0xFF)
	return b&0xC0 != 0x80
}

// ValidateUTF8 checks if a string is valid UTF-8 and returns the
// first invalid byte position, or -1 if valid.
func ValidateUTF8(s string) int {
	for i := 0; i < len(s); {
		if s[i] < 0x80 {
			// ASCII
			i++
			continue
		}

		// Multi-byte sequence
		var size int
		switch {
		case s[i]&0xE0 == 0xC0:
			size = 2
		case s[i]&0xF0 == 0xE0:
			size = 3
		case s[i]&0xF8 == 0xF0:
			size = 4
		default:
			return i
		}

		if i+size > len(s) {
			return i
		}

		// Check continuation bytes
		for j := 1; j < size; j++ {
			if s[i+j]&0xC0 != 0x80 {
				return i
			}
		}

		i += size
	}
	return -1
}
