package dap

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"
)

// mockTransport implements Transport for testing.
type mockTransport struct {
	mu        sync.Mutex
	sendQueue []*Message
	recvQueue []*Message
	recvChan  chan *Message
	closed    bool
	sendErr   error
	recvErr   error
	onSend    func(*Message)
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		recvChan: make(chan *Message, 10),
	}
}

func (t *mockTransport) Send(msg *Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return io.ErrClosedPipe
	}
	if t.sendErr != nil {
		return t.sendErr
	}

	t.sendQueue = append(t.sendQueue, msg)
	if t.onSend != nil {
		t.onSend(msg)
	}
	return nil
}

func (t *mockTransport) Receive() (*Message, error) {
	if t.recvErr != nil {
		return nil, t.recvErr
	}

	msg, ok := <-t.recvChan
	if !ok {
		return nil, io.EOF
	}
	return msg, nil
}

func (t *mockTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.closed {
		t.closed = true
		close(t.recvChan)
	}
	return nil
}

func (t *mockTransport) queueResponse(resp *Message) {
	t.recvChan <- resp
}

func (t *mockTransport) getSentMessages() []*Message {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]*Message{}, t.sendQueue...)
}

func TestClientSendRequest(t *testing.T) {
	mt := newMockTransport()

	// Set up auto-response
	mt.onSend = func(msg *Message) {
		var req Request
		json.Unmarshal(msg.Content, &req)

		resp := Response{
			ProtocolMessage: ProtocolMessage{
				Seq:  1,
				Type: "response",
			},
			RequestSeq: req.Seq,
			Success:    true,
			Command:    req.Command,
			Body:       json.RawMessage(`{}`),
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&Message{
			ContentLength: len(content),
			Content:       content,
		})
	}

	client := NewClient(mt)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := client.ConfigurationDone(ctx)
	if err != nil {
		t.Fatalf("configurationDone: %v", err)
	}

	msgs := mt.getSentMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(msgs))
	}

	var req Request
	if err := json.Unmarshal(msgs[0].Content, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	if req.Command != "configurationDone" {
		t.Errorf("expected command 'configurationDone', got %s", req.Command)
	}

	if req.Type != "request" {
		t.Errorf("expected type 'request', got %s", req.Type)
	}
}

func TestClientInitialize(t *testing.T) {
	mt := newMockTransport()

	// Set up auto-response with capabilities
	mt.onSend = func(msg *Message) {
		var req Request
		json.Unmarshal(msg.Content, &req)

		caps := Capabilities{
			SupportsConfigurationDoneRequest: true,
			SupportsFunctionBreakpoints:      true,
			SupportsConditionalBreakpoints:   true,
		}
		body, _ := json.Marshal(caps)

		resp := Response{
			ProtocolMessage: ProtocolMessage{
				Seq:  1,
				Type: "response",
			},
			RequestSeq: req.Seq,
			Success:    true,
			Command:    req.Command,
			Body:       body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&Message{
			ContentLength: len(content),
			Content:       content,
		})
	}

	client := NewClient(mt)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	args := InitializeRequestArguments{
		ClientID:        "test",
		ClientName:      "Test Client",
		AdapterID:       "go",
		LinesStartAt1:   true,
		ColumnsStartAt1: true,
		PathFormat:      "path",
	}

	caps, err := client.Initialize(ctx, args)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	if !caps.SupportsConfigurationDoneRequest {
		t.Error("expected SupportsConfigurationDoneRequest true")
	}

	if !caps.SupportsFunctionBreakpoints {
		t.Error("expected SupportsFunctionBreakpoints true")
	}
}

func TestClientSetBreakpoints(t *testing.T) {
	mt := newMockTransport()

	// Set up auto-response
	mt.onSend = func(msg *Message) {
		var req Request
		json.Unmarshal(msg.Content, &req)

		bps := []Breakpoint{
			{ID: 1, Verified: true, Line: 10},
			{ID: 2, Verified: true, Line: 20},
		}
		body, _ := json.Marshal(SetBreakpointsResponseBody{Breakpoints: bps})

		resp := Response{
			ProtocolMessage: ProtocolMessage{
				Seq:  1,
				Type: "response",
			},
			RequestSeq: req.Seq,
			Success:    true,
			Command:    req.Command,
			Body:       body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&Message{
			ContentLength: len(content),
			Content:       content,
		})
	}

	client := NewClient(mt)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	args := SetBreakpointsArguments{
		Source: Source{Path: "/path/to/file.go"},
		Breakpoints: []SourceBreakpoint{
			{Line: 10},
			{Line: 20},
		},
	}

	bps, err := client.SetBreakpoints(ctx, args)
	if err != nil {
		t.Fatalf("setBreakpoints: %v", err)
	}

	if len(bps) != 2 {
		t.Fatalf("expected 2 breakpoints, got %d", len(bps))
	}

	if bps[0].Line != 10 {
		t.Errorf("expected first breakpoint at line 10, got %d", bps[0].Line)
	}

	if bps[1].Line != 20 {
		t.Errorf("expected second breakpoint at line 20, got %d", bps[1].Line)
	}
}

func TestClientRequestFailure(t *testing.T) {
	mt := newMockTransport()

	// Set up failure response
	mt.onSend = func(msg *Message) {
		var req Request
		json.Unmarshal(msg.Content, &req)

		resp := Response{
			ProtocolMessage: ProtocolMessage{
				Seq:  1,
				Type: "response",
			},
			RequestSeq: req.Seq,
			Success:    false,
			Command:    req.Command,
			Message:    "command not supported",
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&Message{
			ContentLength: len(content),
			Content:       content,
		})
	}

	client := NewClient(mt)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := client.ConfigurationDone(ctx)
	if err == nil {
		t.Fatal("expected error for failed request")
	}

	if err.Error() != "configurationDone failed: command not supported" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClientContextCancellation(t *testing.T) {
	mt := newMockTransport()
	// Don't set up auto-response - let it hang

	client := NewClient(mt)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.ConfigurationDone(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestClientEventHandlers(t *testing.T) {
	mt := newMockTransport()
	client := NewClient(mt)
	defer client.Close()

	// Track events
	var (
		initializedCalled bool
		stoppedBody       StoppedEventBody
		outputBody        OutputEventBody
	)

	client.OnInitialized(func() {
		initializedCalled = true
	})

	client.OnStopped(func(body StoppedEventBody) {
		stoppedBody = body
	})

	client.OnOutput(func(body OutputEventBody) {
		outputBody = body
	})

	// Send initialized event
	initEvt := Event{
		ProtocolMessage: ProtocolMessage{Seq: 1, Type: "event"},
		Event:           "initialized",
	}
	content, _ := json.Marshal(initEvt)
	mt.queueResponse(&Message{ContentLength: len(content), Content: content})

	// Send stopped event
	stoppedEvtBody, _ := json.Marshal(StoppedEventBody{
		Reason:   "breakpoint",
		ThreadID: 1,
	})
	stoppedEvt := Event{
		ProtocolMessage: ProtocolMessage{Seq: 2, Type: "event"},
		Event:           "stopped",
		Body:            stoppedEvtBody,
	}
	content, _ = json.Marshal(stoppedEvt)
	mt.queueResponse(&Message{ContentLength: len(content), Content: content})

	// Send output event
	outputEvtBody, _ := json.Marshal(OutputEventBody{
		Category: "stdout",
		Output:   "Hello, World!",
	})
	outputEvt := Event{
		ProtocolMessage: ProtocolMessage{Seq: 3, Type: "event"},
		Event:           "output",
		Body:            outputEvtBody,
	}
	content, _ = json.Marshal(outputEvt)
	mt.queueResponse(&Message{ContentLength: len(content), Content: content})

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	if !initializedCalled {
		t.Error("expected initialized event to be called")
	}

	if stoppedBody.Reason != "breakpoint" {
		t.Errorf("expected stopped reason 'breakpoint', got '%s'", stoppedBody.Reason)
	}

	if stoppedBody.ThreadID != 1 {
		t.Errorf("expected stopped threadID 1, got %d", stoppedBody.ThreadID)
	}

	if outputBody.Category != "stdout" {
		t.Errorf("expected output category 'stdout', got '%s'", outputBody.Category)
	}

	if outputBody.Output != "Hello, World!" {
		t.Errorf("expected output 'Hello, World!', got '%s'", outputBody.Output)
	}
}

func TestClientOnAnyEvent(t *testing.T) {
	mt := newMockTransport()
	client := NewClient(mt)
	defer client.Close()

	var events []Event
	client.OnAnyEvent(func(evt Event) {
		events = append(events, evt)
	})

	// Send multiple events
	for i, name := range []string{"initialized", "stopped", "continued"} {
		evt := Event{
			ProtocolMessage: ProtocolMessage{Seq: i + 1, Type: "event"},
			Event:           name,
		}
		content, _ := json.Marshal(evt)
		mt.queueResponse(&Message{ContentLength: len(content), Content: content})
	}

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	if events[0].Event != "initialized" {
		t.Errorf("expected first event 'initialized', got '%s'", events[0].Event)
	}

	if events[1].Event != "stopped" {
		t.Errorf("expected second event 'stopped', got '%s'", events[1].Event)
	}

	if events[2].Event != "continued" {
		t.Errorf("expected third event 'continued', got '%s'", events[2].Event)
	}
}

func TestClientThreads(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *Message) {
		var req Request
		json.Unmarshal(msg.Content, &req)

		threads := []Thread{
			{ID: 1, Name: "main"},
			{ID: 2, Name: "worker-1"},
		}
		body, _ := json.Marshal(ThreadsResponseBody{Threads: threads})

		resp := Response{
			ProtocolMessage: ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&Message{ContentLength: len(content), Content: content})
	}

	client := NewClient(mt)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	threads, err := client.Threads(ctx)
	if err != nil {
		t.Fatalf("threads: %v", err)
	}

	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}

	if threads[0].Name != "main" {
		t.Errorf("expected first thread 'main', got '%s'", threads[0].Name)
	}
}

func TestClientStackTrace(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *Message) {
		var req Request
		json.Unmarshal(msg.Content, &req)

		frames := []StackFrame{
			{
				ID:   1000,
				Name: "main.main",
				Source: &Source{
					Name: "main.go",
					Path: "/path/to/main.go",
				},
				Line:   42,
				Column: 1,
			},
		}
		body, _ := json.Marshal(StackTraceResponseBody{
			StackFrames: frames,
			TotalFrames: 1,
		})

		resp := Response{
			ProtocolMessage: ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&Message{ContentLength: len(content), Content: content})
	}

	client := NewClient(mt)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	args := StackTraceArguments{
		ThreadID:   1,
		StartFrame: 0,
		Levels:     20,
	}

	result, err := client.StackTrace(ctx, args)
	if err != nil {
		t.Fatalf("stackTrace: %v", err)
	}

	if len(result.StackFrames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(result.StackFrames))
	}

	frame := result.StackFrames[0]
	if frame.Name != "main.main" {
		t.Errorf("expected frame name 'main.main', got '%s'", frame.Name)
	}

	if frame.Line != 42 {
		t.Errorf("expected frame line 42, got %d", frame.Line)
	}

	if result.TotalFrames != 1 {
		t.Errorf("expected totalFrames 1, got %d", result.TotalFrames)
	}
}

func TestClientEvaluate(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *Message) {
		var req Request
		json.Unmarshal(msg.Content, &req)

		body, _ := json.Marshal(EvaluateResponseBody{
			Result:             "42",
			Type:               "int",
			VariablesReference: 0,
		})

		resp := Response{
			ProtocolMessage: ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&Message{ContentLength: len(content), Content: content})
	}

	client := NewClient(mt)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	args := EvaluateArguments{
		Expression: "x + y",
		FrameID:    1000,
		Context:    "watch",
	}

	result, err := client.Evaluate(ctx, args)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if result.Result != "42" {
		t.Errorf("expected result '42', got '%s'", result.Result)
	}

	if result.Type != "int" {
		t.Errorf("expected type 'int', got '%s'", result.Type)
	}
}

func TestClientSequenceNumbers(t *testing.T) {
	mt := newMockTransport()

	var seqs []int
	mt.onSend = func(msg *Message) {
		var req Request
		json.Unmarshal(msg.Content, &req)
		seqs = append(seqs, req.Seq)

		resp := Response{
			ProtocolMessage: ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            json.RawMessage(`{}`),
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&Message{ContentLength: len(content), Content: content})
	}

	client := NewClient(mt)
	defer client.Close()

	ctx := context.Background()

	// Send multiple requests
	for i := 0; i < 5; i++ {
		client.ConfigurationDone(ctx)
	}

	// Verify sequence numbers are monotonically increasing
	for i := 1; i < len(seqs); i++ {
		if seqs[i] <= seqs[i-1] {
			t.Errorf("sequence numbers not increasing: %v", seqs)
			break
		}
	}
}
