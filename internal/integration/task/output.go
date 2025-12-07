package task

import (
	"bufio"
	"io"
	"sync"
	"time"
)

// OutputStream identifies the source stream.
type OutputStream int

const (
	// OutputStreamStdout is standard output.
	OutputStreamStdout OutputStream = iota
	// OutputStreamStderr is standard error.
	OutputStreamStderr
)

// String returns the stream name.
func (s OutputStream) String() string {
	switch s {
	case OutputStreamStdout:
		return "stdout"
	case OutputStreamStderr:
		return "stderr"
	default:
		return "unknown"
	}
}

// OutputLine represents a single line of output.
type OutputLine struct {
	// Content is the line content (without newline).
	Content string

	// Stream identifies the source (stdout or stderr).
	Stream OutputStream

	// Timestamp is when the line was received.
	Timestamp time.Time

	// LineNumber is the sequential line number (1-based).
	LineNumber int
}

// OutputProcessor handles output stream processing.
type OutputProcessor struct {
	// lines stores all output lines.
	lines []OutputLine

	// bufferSize is the maximum buffer size for reading.
	bufferSize int

	// lineCount tracks the total line count.
	lineCount int

	// mu protects the lines slice.
	mu sync.RWMutex
}

// NewOutputProcessor creates a new output processor.
func NewOutputProcessor(bufferSize int) *OutputProcessor {
	if bufferSize <= 0 {
		bufferSize = 64 * 1024 // 64KB default
	}

	return &OutputProcessor{
		lines:      make([]OutputLine, 0, 256),
		bufferSize: bufferSize,
	}
}

// Process reads from a reader and processes each line.
// The callback is called for each line as it's received.
// Returns any error from the scanner (e.g., token too long).
func (p *OutputProcessor) Process(r io.Reader, stream OutputStream, callback func(OutputLine)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, p.bufferSize), p.bufferSize)

	for scanner.Scan() {
		p.mu.Lock()
		p.lineCount++
		lineNum := p.lineCount

		line := OutputLine{
			Content:    scanner.Text(),
			Stream:     stream,
			Timestamp:  time.Now(),
			LineNumber: lineNum,
		}

		p.lines = append(p.lines, line)
		p.mu.Unlock()

		if callback != nil {
			callback(line)
		}
	}

	return scanner.Err()
}

// ProcessAsync starts processing in a goroutine and returns immediately.
// The returned channel receives the scanner error (if any) and is then closed.
func (p *OutputProcessor) ProcessAsync(r io.Reader, stream OutputStream, callback func(OutputLine)) <-chan error {
	done := make(chan error, 1)
	go func() {
		err := p.Process(r, stream, callback)
		if err != nil {
			done <- err
		}
		close(done)
	}()
	return done
}

// Lines returns all captured output lines.
func (p *OutputProcessor) Lines() []OutputLine {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]OutputLine, len(p.lines))
	copy(result, p.lines)
	return result
}

// StdoutLines returns only stdout lines.
func (p *OutputProcessor) StdoutLines() []OutputLine {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []OutputLine
	for _, line := range p.lines {
		if line.Stream == OutputStreamStdout {
			result = append(result, line)
		}
	}
	return result
}

// StderrLines returns only stderr lines.
func (p *OutputProcessor) StderrLines() []OutputLine {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []OutputLine
	for _, line := range p.lines {
		if line.Stream == OutputStreamStderr {
			result = append(result, line)
		}
	}
	return result
}

// LineCount returns the total number of lines processed.
func (p *OutputProcessor) LineCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lineCount
}

// LastLines returns the last n lines.
func (p *OutputProcessor) LastLines(n int) []OutputLine {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if n <= 0 || len(p.lines) == 0 {
		return nil
	}

	if n >= len(p.lines) {
		result := make([]OutputLine, len(p.lines))
		copy(result, p.lines)
		return result
	}

	start := len(p.lines) - n
	result := make([]OutputLine, n)
	copy(result, p.lines[start:])
	return result
}

// Content returns all output as a single string.
func (p *OutputProcessor) Content() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.lines) == 0 {
		return ""
	}

	// Calculate total size
	size := 0
	for _, line := range p.lines {
		size += len(line.Content) + 1 // +1 for newline
	}

	// Build string
	result := make([]byte, 0, size)
	for i, line := range p.lines {
		result = append(result, line.Content...)
		if i < len(p.lines)-1 {
			result = append(result, '\n')
		}
	}
	return string(result)
}

// StdoutContent returns only stdout as a string.
func (p *OutputProcessor) StdoutContent() string {
	lines := p.StdoutLines()
	if len(lines) == 0 {
		return ""
	}

	size := 0
	for _, line := range lines {
		size += len(line.Content) + 1
	}

	result := make([]byte, 0, size)
	for i, line := range lines {
		result = append(result, line.Content...)
		if i < len(lines)-1 {
			result = append(result, '\n')
		}
	}
	return string(result)
}

// StderrContent returns only stderr as a string.
func (p *OutputProcessor) StderrContent() string {
	lines := p.StderrLines()
	if len(lines) == 0 {
		return ""
	}

	size := 0
	for _, line := range lines {
		size += len(line.Content) + 1
	}

	result := make([]byte, 0, size)
	for i, line := range lines {
		result = append(result, line.Content...)
		if i < len(lines)-1 {
			result = append(result, '\n')
		}
	}
	return string(result)
}

// Clear removes all captured lines.
func (p *OutputProcessor) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lines = p.lines[:0]
	p.lineCount = 0
}

// OutputBuffer provides a ring buffer for limited output storage.
type OutputBuffer struct {
	lines    []OutputLine
	capacity int
	head     int
	count    int
	mu       sync.RWMutex
}

// NewOutputBuffer creates a new ring buffer with the given capacity.
func NewOutputBuffer(capacity int) *OutputBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &OutputBuffer{
		lines:    make([]OutputLine, capacity),
		capacity: capacity,
	}
}

// Add adds a line to the buffer.
func (b *OutputBuffer) Add(line OutputLine) {
	b.mu.Lock()
	defer b.mu.Unlock()

	idx := (b.head + b.count) % b.capacity
	b.lines[idx] = line

	if b.count < b.capacity {
		b.count++
	} else {
		b.head = (b.head + 1) % b.capacity
	}
}

// Lines returns all lines in order.
func (b *OutputBuffer) Lines() []OutputLine {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]OutputLine, b.count)
	for i := 0; i < b.count; i++ {
		idx := (b.head + i) % b.capacity
		result[i] = b.lines[idx]
	}
	return result
}

// Count returns the number of lines in the buffer.
func (b *OutputBuffer) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// Clear empties the buffer.
func (b *OutputBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.head = 0
	b.count = 0
}
