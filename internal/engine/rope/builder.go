package rope

import (
	"io"
	"strings"
)

// Builder provides efficient incremental construction of a rope.
// It buffers writes and builds the rope structure when Build() is called.
type Builder struct {
	chunks   []Chunk
	buffer   strings.Builder
	totalLen int
}

// NewBuilder creates a new rope builder.
func NewBuilder() *Builder {
	return &Builder{
		chunks: make([]Chunk, 0, 64),
	}
}

// WriteString appends a string to the builder.
func (b *Builder) WriteString(s string) {
	if len(s) == 0 {
		return
	}

	b.totalLen += len(s)
	b.buffer.WriteString(s)

	// Flush to chunks if buffer is large enough
	if b.buffer.Len() >= MaxChunkSize*2 {
		b.flushBuffer()
	}
}

// Write implements io.Writer.
func (b *Builder) Write(p []byte) (n int, err error) {
	b.WriteString(string(p))
	return len(p), nil
}

// WriteByte appends a single byte.
func (b *Builder) WriteByte(c byte) error {
	b.totalLen++
	return b.buffer.WriteByte(c)
}

// WriteRune appends a single rune.
func (b *Builder) WriteRune(r rune) (int, error) {
	n, err := b.buffer.WriteRune(r)
	b.totalLen += n
	return n, err
}

// flushBuffer converts the buffer contents to chunks.
func (b *Builder) flushBuffer() {
	if b.buffer.Len() == 0 {
		return
	}

	s := b.buffer.String()
	b.buffer.Reset()

	newChunks := splitIntoChunks(s)
	b.chunks = append(b.chunks, newChunks...)
}

// Len returns the total number of bytes written.
func (b *Builder) Len() int {
	return b.totalLen
}

// Reset clears the builder for reuse.
func (b *Builder) Reset() {
	b.chunks = b.chunks[:0]
	b.buffer.Reset()
	b.totalLen = 0
}

// Build creates the rope from accumulated data.
// After calling Build, the builder is reset.
func (b *Builder) Build() Rope {
	// Flush any remaining buffer
	b.flushBuffer()

	if len(b.chunks) == 0 {
		b.Reset()
		return New()
	}

	chunks := b.chunks
	b.Reset()

	return buildFromChunks(chunks)
}

// String returns the accumulated text as a string.
// This is primarily for debugging; prefer Build() for creating ropes.
func (b *Builder) String() string {
	var sb strings.Builder
	sb.Grow(b.totalLen)

	for _, chunk := range b.chunks {
		sb.WriteString(chunk.String())
	}
	sb.WriteString(b.buffer.String())

	return sb.String()
}

// ReadFrom implements io.ReaderFrom for efficient reading.
func (b *Builder) ReadFrom(r io.Reader) (int64, error) {
	buf := make([]byte, 64*1024) // 64KB buffer
	var total int64

	for {
		n, err := r.Read(buf)
		if n > 0 {
			b.WriteString(string(buf[:n]))
			total += int64(n)
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

// FromLines creates a rope from a slice of lines.
// Each line will have a newline appended except the last.
func FromLines(lines []string) Rope {
	if len(lines) == 0 {
		return New()
	}

	var builder Builder
	for i, line := range lines {
		builder.WriteString(line)
		if i < len(lines)-1 {
			builder.WriteByte('\n')
		}
	}

	return builder.Build()
}

// FromChunks creates a rope directly from chunks.
// This is useful when you have pre-chunked data.
func FromChunks(chunks []Chunk) Rope {
	if len(chunks) == 0 {
		return New()
	}
	return buildFromChunks(chunks)
}

// Join concatenates multiple ropes with a separator.
func Join(ropes []Rope, sep string) Rope {
	if len(ropes) == 0 {
		return New()
	}
	if len(ropes) == 1 {
		return ropes[0]
	}

	result := ropes[0]
	sepRope := FromString(sep)

	for i := 1; i < len(ropes); i++ {
		if sep != "" {
			result = result.Concat(sepRope)
		}
		result = result.Concat(ropes[i])
	}

	return result
}

// Repeat creates a rope by repeating a string n times.
func Repeat(s string, n int) Rope {
	if n <= 0 || len(s) == 0 {
		return New()
	}

	// For small repetitions, just use string repeat
	totalLen := len(s) * n
	if totalLen <= MaxChunkSize*4 {
		return FromString(strings.Repeat(s, n))
	}

	// For larger repetitions, build efficiently
	var builder Builder
	for i := 0; i < n; i++ {
		builder.WriteString(s)
	}
	return builder.Build()
}
