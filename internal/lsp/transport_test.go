package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// mockPipe creates a bidirectional pipe for testing.
type mockPipe struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func newMockPipe() *mockPipe {
	r, w := io.Pipe()
	return &mockPipe{reader: r, writer: w}
}

func (p *mockPipe) Close() error {
	p.reader.Close()
	p.writer.Close()
	return nil
}

func TestTransport_SendNotification(t *testing.T) {
	// Create pipes
	clientToServer := newMockPipe()
	serverToClient := newMockPipe()

	transport := NewTransport(serverToClient.reader, clientToServer.writer, nil)
	defer transport.Close()

	// Read what the transport sends
	var received []byte
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Read header
		header := make([]byte, 100)
		n, _ := clientToServer.reader.Read(header)
		received = append(received, header[:n]...)
		// Read remaining
		more := make([]byte, 1024)
		n, _ = clientToServer.reader.Read(more)
		received = append(received, more[:n]...)
	}()

	// Send notification
	ctx := context.Background()
	params := map[string]string{"message": "hello"}
	err := transport.Notify(ctx, "test/notification", params)
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	// Close writer to unblock reader
	clientToServer.writer.Close()
	wg.Wait()

	// Verify format
	str := string(received)
	if len(str) == 0 {
		t.Fatal("No data received")
	}

	// Should contain Content-Length header
	if !contains(str, "Content-Length:") {
		t.Errorf("Missing Content-Length header in: %s", str)
	}

	// Should contain JSON body
	if !contains(str, `"jsonrpc":"2.0"`) {
		t.Errorf("Missing jsonrpc field in: %s", str)
	}
	if !contains(str, `"method":"test/notification"`) {
		t.Errorf("Missing method field in: %s", str)
	}
}

func TestTransport_Call(t *testing.T) {
	// Create pipes for bidirectional communication
	clientToServer := newMockPipe()
	serverToClient := newMockPipe()

	transport := NewTransport(serverToClient.reader, clientToServer.writer, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start the transport's read loop
	transport.Start(ctx)

	// Mock server that reads request and sends response
	go func() {
		// Read the request
		header := make([]byte, 256)
		n, err := clientToServer.reader.Read(header)
		if err != nil {
			return
		}

		// Parse content length
		headerStr := string(header[:n])
		var contentLength int
		fmt.Sscanf(headerStr, "Content-Length: %d", &contentLength)

		// Find where body starts (after \r\n\r\n)
		bodyStart := 0
		for i := 0; i < len(headerStr)-3; i++ {
			if headerStr[i:i+4] == "\r\n\r\n" {
				bodyStart = i + 4
				break
			}
		}

		// Read the body
		body := headerStr[bodyStart:]
		if len(body) < contentLength {
			remaining := make([]byte, contentLength-len(body))
			clientToServer.reader.Read(remaining)
			body += string(remaining)
		}

		// Parse request
		var req Request
		json.Unmarshal([]byte(body), &req)

		// Send response
		result := map[string]string{"status": "ok"}
		resultBytes, _ := json.Marshal(result)
		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultBytes,
		}
		respBytes, _ := json.Marshal(resp)
		respHeader := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(respBytes))
		serverToClient.writer.Write([]byte(respHeader))
		serverToClient.writer.Write(respBytes)
	}()

	// Make a call
	var result map[string]string
	err := transport.Call(ctx, "test/method", nil, &result)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status=ok, got %v", result)
	}

	transport.Close()
}

func TestTransport_CallWithError(t *testing.T) {
	clientToServer := newMockPipe()
	serverToClient := newMockPipe()

	transport := NewTransport(serverToClient.reader, clientToServer.writer, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport.Start(ctx)

	// Mock server response channel to ensure proper sync
	serverDone := make(chan struct{})

	// Mock server that returns an error
	go func() {
		defer close(serverDone)
		// Read request - use buffered reader pattern
		buf := make([]byte, 4096)
		n, err := clientToServer.reader.Read(buf)
		if err != nil || n == 0 {
			return
		}
		data := string(buf[:n])

		// Parse content length
		var contentLength int
		fmt.Sscanf(data, "Content-Length: %d", &contentLength)

		// Find body start
		bodyStart := 0
		for i := 0; i < len(data)-3; i++ {
			if data[i:i+4] == "\r\n\r\n" {
				bodyStart = i + 4
				break
			}
		}

		// Read remaining body if needed
		body := data[bodyStart:]
		for len(body) < contentLength {
			more := make([]byte, contentLength-len(body))
			m, err := clientToServer.reader.Read(more)
			if err != nil {
				return
			}
			body += string(more[:m])
		}

		var req Request
		if err := json.Unmarshal([]byte(body[:contentLength]), &req); err != nil {
			return
		}

		// Send error response
		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    CodeMethodNotFound,
				Message: "method not found",
			},
		}
		respBytes, _ := json.Marshal(resp)
		respHeader := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(respBytes))
		serverToClient.writer.Write([]byte(respHeader))
		serverToClient.writer.Write(respBytes)
	}()

	// Make a call that will fail
	var result any
	err := transport.Call(ctx, "unknown/method", nil, &result)

	// Wait for server to finish
	<-serverDone

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("Expected *RPCError, got %T: %v", err, err)
	}

	if rpcErr.Code != CodeMethodNotFound {
		t.Errorf("Expected code %d, got %d", CodeMethodNotFound, rpcErr.Code)
	}

	transport.Close()
}

func TestTransport_Notification(t *testing.T) {
	clientToServer := newMockPipe()
	serverToClient := newMockPipe()

	transport := NewTransport(serverToClient.reader, clientToServer.writer, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Set up notification handler
	received := make(chan string, 1)
	transport.OnNotification("test/notify", func(method string, params json.RawMessage) {
		var p struct {
			Message string `json:"message"`
		}
		json.Unmarshal(params, &p)
		received <- p.Message
	})

	transport.Start(ctx)

	// Send a notification from "server"
	go func() {
		notif := map[string]any{
			"jsonrpc": "2.0",
			"method":  "test/notify",
			"params":  map[string]string{"message": "hello from server"},
		}
		notifBytes, _ := json.Marshal(notif)
		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(notifBytes))
		serverToClient.writer.Write([]byte(header))
		serverToClient.writer.Write(notifBytes)
	}()

	// Wait for notification
	select {
	case msg := <-received:
		if msg != "hello from server" {
			t.Errorf("Expected 'hello from server', got %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for notification")
	}

	transport.Close()
}

func TestTransport_CallTimeout(t *testing.T) {
	clientToServer := newMockPipe()
	serverToClient := newMockPipe()

	transport := NewTransport(serverToClient.reader, clientToServer.writer, nil)

	// Start transport with background context
	bgCtx := context.Background()
	transport.Start(bgCtx)

	// Use a short timeout context for the call
	ctx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
	defer cancel()

	// Read the request but don't respond - this goroutine keeps reading
	// to let the write complete, but never sends a response
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := clientToServer.reader.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	var result any
	err := transport.Call(ctx, "slow/method", nil, &result)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}

	// Close pipes to unblock any readers
	clientToServer.Close()
	serverToClient.Close()
	transport.Close()
}

func TestTransport_Close(t *testing.T) {
	clientToServer := newMockPipe()
	serverToClient := newMockPipe()

	transport := NewTransport(serverToClient.reader, clientToServer.writer, clientToServer)

	ctx := context.Background()
	transport.Start(ctx)

	// Close transport
	if err := transport.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Further calls should fail
	err := transport.Notify(ctx, "test", nil)
	if err != ErrShutdown {
		t.Errorf("Expected ErrShutdown after close, got %v", err)
	}

	// Double close should be safe
	if err := transport.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestTransport_IsClosed(t *testing.T) {
	clientToServer := newMockPipe()
	serverToClient := newMockPipe()

	transport := NewTransport(serverToClient.reader, clientToServer.writer, nil)

	if transport.IsClosed() {
		t.Error("Transport should not be closed initially")
	}

	transport.Close()

	if !transport.IsClosed() {
		t.Error("Transport should be closed after Close()")
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
