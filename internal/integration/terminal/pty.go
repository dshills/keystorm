package terminal

import (
	"io"
	"os"
	"os/exec"
)

// PTY represents a pseudo-terminal.
type PTY interface {
	// File returns the PTY file descriptor.
	File() *os.File

	// Read reads from the PTY.
	Read(p []byte) (n int, err error)

	// Write writes to the PTY.
	Write(p []byte) (n int, err error)

	// Resize changes the PTY size.
	Resize(cols, rows uint16) error

	// Close closes the PTY.
	Close() error
}

// StartPTY starts a command with a PTY.
// Returns the PTY and the started command's process.
func StartPTY(cmd *exec.Cmd, cols, rows uint16) (PTY, error) {
	return startPTY(cmd, cols, rows)
}

// ptyWrapper wraps a file as a PTY interface.
type ptyWrapper struct {
	file *os.File
}

func (p *ptyWrapper) File() *os.File {
	return p.file
}

func (p *ptyWrapper) Read(buf []byte) (int, error) {
	return p.file.Read(buf)
}

func (p *ptyWrapper) Write(data []byte) (int, error) {
	return p.file.Write(data)
}

func (p *ptyWrapper) Close() error {
	return p.file.Close()
}

// History stores scrollback history lines.
type History struct {
	lines    []*Line
	maxLines int
}

// NewHistory creates a new history buffer.
func NewHistory(maxLines int) *History {
	if maxLines <= 0 {
		maxLines = 10000
	}
	return &History{
		lines:    make([]*Line, 0, maxLines),
		maxLines: maxLines,
	}
}

// Add adds a line to history.
func (h *History) Add(line *Line) {
	// Create a copy of the line
	newLine := &Line{
		Cells:   make([]Cell, len(line.Cells)),
		Wrapped: line.Wrapped,
	}
	copy(newLine.Cells, line.Cells)

	h.lines = append(h.lines, newLine)

	// Trim if exceeds max
	if len(h.lines) > h.maxLines {
		h.lines = h.lines[len(h.lines)-h.maxLines:]
	}
}

// Line returns a line from history (0 = oldest).
func (h *History) Line(index int) *Line {
	if index < 0 || index >= len(h.lines) {
		return nil
	}
	return h.lines[index]
}

// Len returns the number of lines in history.
func (h *History) Len() int {
	return len(h.lines)
}

// Clear clears the history.
func (h *History) Clear() {
	h.lines = h.lines[:0]
}

// GetText returns all history as text.
func (h *History) GetText() string {
	var result []rune
	for i, line := range h.lines {
		for _, cell := range line.Cells {
			result = append(result, cell.Rune)
		}
		if i < len(h.lines)-1 && !line.Wrapped {
			result = append(result, '\n')
		}
	}
	return string(result)
}

// outputReader wraps PTY output to parse ANSI sequences.
type outputReader struct {
	pty    PTY
	parser *Parser
	buf    []byte
}

// newOutputReader creates a new output reader.
func newOutputReader(pty PTY, parser *Parser) *outputReader {
	return &outputReader{
		pty:    pty,
		parser: parser,
		buf:    make([]byte, 4096),
	}
}

// ReadAndParse reads from PTY and parses into screen.
// Returns bytes read and error.
func (r *outputReader) ReadAndParse() (int, error) {
	n, err := r.pty.Read(r.buf)
	if n > 0 {
		r.parser.Parse(r.buf[:n])
	}
	return n, err
}

// inputWriter wraps PTY input.
type inputWriter struct {
	pty PTY
}

// newInputWriter creates a new input writer.
func newInputWriter(pty PTY) *inputWriter {
	return &inputWriter{pty: pty}
}

// Write writes data to the PTY.
func (w *inputWriter) Write(p []byte) (int, error) {
	return w.pty.Write(p)
}

// WriteString writes a string to the PTY.
func (w *inputWriter) WriteString(s string) (int, error) {
	return w.pty.Write([]byte(s))
}

// Ensure interfaces are implemented.
var (
	_ io.Reader = (*outputReader)(nil)
	_ io.Writer = (*inputWriter)(nil)
)

// Read implements io.Reader.
func (r *outputReader) Read(p []byte) (int, error) {
	return r.pty.Read(p)
}
