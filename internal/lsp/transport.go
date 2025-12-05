package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Transport handles JSON-RPC 2.0 communication over stdio.
// It implements the LSP base protocol with Content-Length headers.
type Transport struct {
	reader *bufio.Reader
	writer io.Writer
	closer io.Closer

	mu       sync.Mutex
	nextID   atomic.Int64
	pending  map[int64]chan *Response
	handlers map[string]NotificationHandler

	closed atomic.Bool
	done   chan struct{}
}

// NotificationHandler handles incoming notifications from the server.
type NotificationHandler func(method string, params json.RawMessage)

// Request represents a JSON-RPC request.
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Response represents a JSON-RPC response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// notification is used to parse incoming notifications.
type notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// NewTransport creates a new transport over the given connection.
// The conn must support reading and writing (typically stdin/stdout pipes).
func NewTransport(r io.Reader, w io.Writer, c io.Closer) *Transport {
	t := &Transport{
		reader:   bufio.NewReaderSize(r, 64*1024),
		writer:   w,
		closer:   c,
		pending:  make(map[int64]chan *Response),
		handlers: make(map[string]NotificationHandler),
		done:     make(chan struct{}),
	}
	return t
}

// Start begins reading messages from the connection.
// This should be called in a goroutine.
func (t *Transport) Start(ctx context.Context) {
	go t.readLoop(ctx)
}

// Close closes the transport and releases resources.
func (t *Transport) Close() error {
	if t.closed.Swap(true) {
		return nil // Already closed
	}

	close(t.done)

	// Cancel all pending requests by clearing the map.
	// We don't close the channels to avoid race conditions with handleResponse.
	// Callers waiting on pending channels will receive from t.done instead.
	t.mu.Lock()
	t.pending = make(map[int64]chan *Response)
	t.mu.Unlock()

	if t.closer != nil {
		return t.closer.Close()
	}
	return nil
}

// Call sends a request and waits for a response.
func (t *Transport) Call(ctx context.Context, method string, params any, result any) error {
	if t.closed.Load() {
		return ErrShutdown
	}

	id := t.nextID.Add(1)
	ch := make(chan *Response, 1)

	t.mu.Lock()
	t.pending[id] = ch
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
	}()

	// Send request
	req := &Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := t.send(req); err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.done:
		return ErrShutdown
	case resp, ok := <-ch:
		if !ok {
			return ErrShutdown
		}
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("unmarshal result: %w", err)
			}
		}
		return nil
	}
}

// Notify sends a notification (no response expected).
func (t *Transport) Notify(ctx context.Context, method string, params any) error {
	if t.closed.Load() {
		return ErrShutdown
	}

	req := &Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	return t.send(req)
}

// OnNotification registers a handler for server notifications.
func (t *Transport) OnNotification(method string, handler NotificationHandler) {
	t.mu.Lock()
	t.handlers[method] = handler
	t.mu.Unlock()
}

// send writes a message with LSP content-length header.
func (t *Transport) send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, err := io.WriteString(t.writer, header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := t.writer.Write(data); err != nil {
		return fmt.Errorf("write body: %w", err)
	}

	return nil
}

// readLoop reads messages from the connection.
func (t *Transport) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.done:
			return
		default:
		}

		msg, err := t.readMessage()
		if err != nil {
			if t.closed.Load() {
				return
			}
			if err == io.EOF || err == io.ErrClosedPipe {
				return
			}
			// Log error but continue reading
			continue
		}

		t.dispatch(msg)
	}
}

// readMessage reads a single LSP message.
func (t *Transport) readMessage() (json.RawMessage, error) {
	// Read headers
	var contentLength int
	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // End of headers
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				length, err := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err == nil {
					contentLength = length
				}
			}
		}
		// Ignore Content-Type and other headers
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read body
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(t.reader, body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return body, nil
}

// dispatch routes a message to the appropriate handler.
func (t *Transport) dispatch(data json.RawMessage) {
	// Try to determine message type by checking for "id" field
	var probe struct {
		ID     *int64          `json:"id"`
		Method string          `json:"method"`
		Error  *RPCError       `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return
	}

	// If it has an ID and either result or error, it's a response
	if probe.ID != nil && (probe.Result != nil || probe.Error != nil) {
		var resp Response
		if err := json.Unmarshal(data, &resp); err != nil {
			return
		}
		t.handleResponse(&resp)
		return
	}

	// Otherwise, it's a notification (or request from server)
	if probe.Method != "" {
		var notif notification
		if err := json.Unmarshal(data, &notif); err != nil {
			return
		}
		t.handleNotification(&notif)
	}
}

// handleResponse routes a response to its waiting caller.
func (t *Transport) handleResponse(resp *Response) {
	// Check if closed before attempting to send
	if t.closed.Load() {
		return
	}

	t.mu.Lock()
	ch, ok := t.pending[resp.ID]
	if ok {
		// Remove from pending while holding lock to prevent races
		delete(t.pending, resp.ID)
	}
	t.mu.Unlock()

	if ok {
		select {
		case ch <- resp:
		default:
			// Channel full, drop response
		}
	}
}

// handleNotification routes a notification to its handler.
func (t *Transport) handleNotification(notif *notification) {
	t.mu.Lock()
	handler, ok := t.handlers[notif.Method]
	if !ok {
		// Check for wildcard handler
		handler, ok = t.handlers["*"]
	}
	t.mu.Unlock()

	if ok && handler != nil {
		// Run handler in goroutine to avoid blocking read loop
		go handler(notif.Method, notif.Params)
	}
}

// IsClosed returns true if the transport has been closed.
func (t *Transport) IsClosed() bool {
	return t.closed.Load()
}
