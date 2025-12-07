// Package dap implements the Debug Adapter Protocol client.
package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Transport represents a DAP transport layer.
type Transport interface {
	// Send sends a message to the debug adapter.
	Send(msg *Message) error

	// Receive receives a message from the debug adapter.
	Receive() (*Message, error)

	// Close closes the transport.
	Close() error
}

// Message represents a DAP message with headers and content.
type Message struct {
	// ContentLength is the length of the content.
	ContentLength int

	// ContentType is the MIME type (optional).
	ContentType string

	// Content is the JSON content.
	Content json.RawMessage
}

// StdioTransport implements Transport over stdin/stdout of a subprocess.
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	reader *bufio.Reader
	mu     sync.Mutex
}

// NewStdioTransport creates a new stdio transport for a debug adapter process.
func NewStdioTransport(cmd *exec.Cmd) (*StdioTransport, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("start command: %w", err)
	}

	return &StdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		reader: bufio.NewReader(stdout),
	}, nil
}

// Send sends a message to the debug adapter.
func (t *StdioTransport) Send(msg *Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return writeMessage(t.stdin, msg)
}

// Receive receives a message from the debug adapter.
func (t *StdioTransport) Receive() (*Message, error) {
	return readMessage(t.reader)
}

// Close closes the transport and terminates the subprocess.
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stdin.Close()
	t.stdout.Close()

	if t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}

	return t.cmd.Wait()
}

// SocketTransport implements Transport over a TCP socket.
type SocketTransport struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
}

// NewSocketTransport creates a new socket transport.
func NewSocketTransport(address string) (*SocketTransport, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", address, err)
	}

	return &SocketTransport{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

// NewSocketTransportFromConn creates a socket transport from an existing connection.
func NewSocketTransportFromConn(conn net.Conn) *SocketTransport {
	return &SocketTransport{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}
}

// Send sends a message to the debug adapter.
func (t *SocketTransport) Send(msg *Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return writeMessage(t.conn, msg)
}

// Receive receives a message from the debug adapter.
func (t *SocketTransport) Receive() (*Message, error) {
	return readMessage(t.reader)
}

// Close closes the socket connection.
func (t *SocketTransport) Close() error {
	return t.conn.Close()
}

// writeMessage writes a DAP message to the writer.
func writeMessage(w io.Writer, msg *Message) error {
	// Build headers
	headers := fmt.Sprintf("Content-Length: %d\r\n", len(msg.Content))
	if msg.ContentType != "" {
		headers += fmt.Sprintf("Content-Type: %s\r\n", msg.ContentType)
	}
	headers += "\r\n"

	// Write headers
	if _, err := w.Write([]byte(headers)); err != nil {
		return fmt.Errorf("write headers: %w", err)
	}

	// Write content
	if _, err := w.Write(msg.Content); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	return nil
}

// MaxContentLength is the maximum allowed content length for DAP messages (10MB).
const MaxContentLength = 10 * 1024 * 1024

// readMessage reads a DAP message from the reader.
func readMessage(r *bufio.Reader) (*Message, error) {
	var contentLength int
	var contentType string

	// Read headers
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // End of headers
		}

		// Parse header
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header: %s", line)
		}

		switch strings.ToLower(parts[0]) {
		case "content-length":
			length, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid content-length: %w", err)
			}
			if length < 0 || length > MaxContentLength {
				return nil, fmt.Errorf("content-length %d exceeds maximum allowed %d", length, MaxContentLength)
			}
			contentLength = length
		case "content-type":
			contentType = parts[1]
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read content
	content := make([]byte, contentLength)
	if _, err := io.ReadFull(r, content); err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	return &Message{
		ContentLength: contentLength,
		ContentType:   contentType,
		Content:       content,
	}, nil
}

// RawTransport wraps any io.ReadWriteCloser as a Transport.
type RawTransport struct {
	rwc    io.ReadWriteCloser
	reader *bufio.Reader
	mu     sync.Mutex
}

// NewRawTransport creates a transport from any ReadWriteCloser.
func NewRawTransport(rwc io.ReadWriteCloser) *RawTransport {
	return &RawTransport{
		rwc:    rwc,
		reader: bufio.NewReader(rwc),
	}
}

// Send sends a message.
func (t *RawTransport) Send(msg *Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return writeMessage(t.rwc, msg)
}

// Receive receives a message.
func (t *RawTransport) Receive() (*Message, error) {
	return readMessage(t.reader)
}

// Close closes the underlying connection.
func (t *RawTransport) Close() error {
	return t.rwc.Close()
}
