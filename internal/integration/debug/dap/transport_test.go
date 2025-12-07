package dap

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestWriteMessage(t *testing.T) {
	var buf bytes.Buffer
	content := json.RawMessage(`{"test": "value"}`)

	msg := &Message{
		ContentLength: len(content),
		Content:       content,
	}

	if err := writeMessage(&buf, msg); err != nil {
		t.Fatalf("write message: %v", err)
	}

	result := buf.String()
	if !strings.HasPrefix(result, "Content-Length: 17\r\n\r\n") {
		t.Errorf("unexpected header: %q", result)
	}

	if !strings.HasSuffix(result, `{"test": "value"}`) {
		t.Errorf("unexpected content: %q", result)
	}
}

func TestWriteMessageWithContentType(t *testing.T) {
	var buf bytes.Buffer
	content := json.RawMessage(`{}`)

	msg := &Message{
		ContentLength: len(content),
		ContentType:   "application/json",
		Content:       content,
	}

	if err := writeMessage(&buf, msg); err != nil {
		t.Fatalf("write message: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, "Content-Type: application/json\r\n") {
		t.Errorf("missing Content-Type header: %q", result)
	}
}

func TestReadMessage(t *testing.T) {
	input := "Content-Length: 17\r\n\r\n{\"test\": \"value\"}"
	reader := strings.NewReader(input)
	bufReader := bufio.NewReader(reader)

	msg, err := readMessage(bufReader)
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	if msg.ContentLength != 17 {
		t.Errorf("expected ContentLength 17, got %d", msg.ContentLength)
	}

	var parsed map[string]string
	if err := json.Unmarshal(msg.Content, &parsed); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}

	if parsed["test"] != "value" {
		t.Errorf("expected 'value', got '%s'", parsed["test"])
	}
}

func TestReadMessageWithContentType(t *testing.T) {
	input := "Content-Length: 2\r\nContent-Type: application/json\r\n\r\n{}"
	reader := strings.NewReader(input)
	bufReader := bufio.NewReader(reader)

	msg, err := readMessage(bufReader)
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	if msg.ContentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", msg.ContentType)
	}
}

func TestReadMessageMissingContentLength(t *testing.T) {
	input := "Content-Type: application/json\r\n\r\n{}"
	reader := strings.NewReader(input)
	bufReader := bufio.NewReader(reader)

	_, err := readMessage(bufReader)
	if err == nil {
		t.Error("expected error for missing Content-Length")
	}
}

func TestReadMessageInvalidHeader(t *testing.T) {
	input := "InvalidHeader\r\n\r\n"
	reader := strings.NewReader(input)
	bufReader := bufio.NewReader(reader)

	_, err := readMessage(bufReader)
	if err == nil {
		t.Error("expected error for invalid header")
	}
}

func TestRoundTrip(t *testing.T) {
	content := json.RawMessage(`{"seq": 1, "type": "request", "command": "initialize"}`)

	original := &Message{
		ContentLength: len(content),
		Content:       content,
	}

	// Write to buffer
	var buf bytes.Buffer
	if err := writeMessage(&buf, original); err != nil {
		t.Fatalf("write message: %v", err)
	}

	// Read back
	bufReader := bufio.NewReader(&buf)
	result, err := readMessage(bufReader)
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	if result.ContentLength != original.ContentLength {
		t.Errorf("ContentLength mismatch: expected %d, got %d", original.ContentLength, result.ContentLength)
	}

	if !bytes.Equal(result.Content, original.Content) {
		t.Errorf("Content mismatch: expected %s, got %s", original.Content, result.Content)
	}
}

func TestSocketTransport(t *testing.T) {
	// Create a listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	// Server goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		server := NewSocketTransportFromConn(conn)

		// Read message
		msg, err := server.Receive()
		if err != nil {
			t.Errorf("server receive: %v", err)
			return
		}

		// Echo back
		if err := server.Send(msg); err != nil {
			t.Errorf("server send: %v", err)
			return
		}
	}()

	// Client
	transport, err := NewSocketTransport(listener.Addr().String())
	if err != nil {
		t.Fatalf("create transport: %v", err)
	}
	defer transport.Close()

	content := json.RawMessage(`{"test": "echo"}`)
	msg := &Message{
		ContentLength: len(content),
		Content:       content,
	}

	// Send
	if err := transport.Send(msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Receive echo
	result, err := transport.Receive()
	if err != nil {
		t.Fatalf("receive: %v", err)
	}

	if !bytes.Equal(result.Content, content) {
		t.Errorf("echo mismatch: expected %s, got %s", content, result.Content)
	}

	<-done
}

func TestRawTransport(t *testing.T) {
	// Create two pipes for bidirectional communication
	// Client writes to pw1, server reads from pr1
	// Server writes to pw2, client reads from pr2
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()

	defer pr1.Close()
	defer pw1.Close()
	defer pr2.Close()
	defer pw2.Close()

	// Client transport: writes to pw1, reads from pr2
	clientRWC := &pipeRWC{r: pr2, w: pw1}
	clientTransport := NewRawTransport(clientRWC)

	// Server transport: reads from pr1, writes to pw2
	serverRWC := &pipeRWC{r: pr1, w: pw2}
	serverTransport := NewRawTransport(serverRWC)

	// Server goroutine: echo back received message
	done := make(chan struct{})
	go func() {
		defer close(done)
		msg, err := serverTransport.Receive()
		if err != nil {
			t.Errorf("server receive: %v", err)
			return
		}
		if err := serverTransport.Send(msg); err != nil {
			t.Errorf("server send: %v", err)
			return
		}
	}()

	// Client: send message
	content := json.RawMessage(`{"hello": "world"}`)
	msg := &Message{
		ContentLength: len(content),
		Content:       content,
	}

	if err := clientTransport.Send(msg); err != nil {
		t.Fatalf("client send: %v", err)
	}

	// Client: receive echo with timeout
	resultChan := make(chan *Message)
	errChan := make(chan error)
	go func() {
		result, err := clientTransport.Receive()
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	timer := time.NewTimer(time.Second)
	select {
	case result := <-resultChan:
		if result.ContentLength != 18 {
			t.Errorf("expected ContentLength 18, got %d", result.ContentLength)
		}
		if !bytes.Equal(result.Content, content) {
			t.Errorf("content mismatch: expected %s, got %s", content, result.Content)
		}
	case err := <-errChan:
		t.Fatalf("receive error: %v", err)
	case <-timer.C:
		t.Fatal("timeout waiting for message")
	}

	<-done
}

// pipeRWC wraps separate read and write ends of a pipe as io.ReadWriteCloser
type pipeRWC struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (p *pipeRWC) Read(data []byte) (int, error) {
	return p.r.Read(data)
}

func (p *pipeRWC) Write(data []byte) (int, error) {
	return p.w.Write(data)
}

func (p *pipeRWC) Close() error {
	p.r.Close()
	return p.w.Close()
}
