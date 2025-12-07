package debug

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/dshills/keystorm/internal/integration/debug/dap"
)

// mockTransport implements dap.Transport for testing.
type mockTransport struct {
	mu        sync.Mutex
	sendQueue []*dap.Message
	recvChan  chan *dap.Message
	closed    bool
	onSend    func(*dap.Message)
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		recvChan: make(chan *dap.Message, 10),
	}
}

func (t *mockTransport) Send(msg *dap.Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return io.ErrClosedPipe
	}

	t.sendQueue = append(t.sendQueue, msg)
	if t.onSend != nil {
		t.onSend(msg)
	}
	return nil
}

func (t *mockTransport) Receive() (*dap.Message, error) {
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

func (t *mockTransport) queueResponse(resp *dap.Message) {
	t.recvChan <- resp
}

func TestSessionState(t *testing.T) {
	mt := newMockTransport()
	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	// Initial state should be connected
	if session.State() != StateConnected {
		t.Errorf("expected initial state Connected, got %v", session.State())
	}
}

func TestSessionStateString(t *testing.T) {
	tests := []struct {
		state    SessionState
		expected string
	}{
		{StateInitializing, "initializing"},
		{StateConnected, "connected"},
		{StateConfiguring, "configuring"},
		{StateRunning, "running"},
		{StateStopped, "stopped"},
		{StateTerminated, "terminated"},
		{StateDisconnected, "disconnected"},
		{SessionState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("SessionState(%d).String() = %s, want %s", tt.state, got, tt.expected)
		}
	}
}

func TestSessionInitialize(t *testing.T) {
	mt := newMockTransport()

	// Auto-respond to initialize
	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		if req.Command == "initialize" {
			caps := dap.Capabilities{
				SupportsConfigurationDoneRequest: true,
			}
			body, _ := json.Marshal(caps)

			resp := dap.Response{
				ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
				RequestSeq:      req.Seq,
				Success:         true,
				Command:         req.Command,
				Body:            body,
			}

			content, _ := json.Marshal(resp)
			mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
		}
	}

	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	config := DefaultSessionConfig()
	err := session.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// State should be configuring after initialize
	if session.State() != StateConfiguring {
		t.Errorf("expected state Configuring, got %v", session.State())
	}

	// Capabilities should be set
	caps := session.Capabilities()
	if caps == nil {
		t.Fatal("expected capabilities to be set")
	}

	if !caps.SupportsConfigurationDoneRequest {
		t.Error("expected SupportsConfigurationDoneRequest true")
	}
}

func TestSessionConfigurationDone(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		resp := dap.Response{
			ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            json.RawMessage(`{}`),
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
	}

	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := session.ConfigurationDone(ctx)
	if err != nil {
		t.Fatalf("configurationDone: %v", err)
	}

	if session.State() != StateRunning {
		t.Errorf("expected state Running, got %v", session.State())
	}
}

func TestSessionSetBreakpoints(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		bps := []dap.Breakpoint{
			{ID: 1, Verified: true, Line: 10},
			{ID: 2, Verified: true, Line: 20},
		}
		body, _ := json.Marshal(dap.SetBreakpointsResponseBody{Breakpoints: bps})

		resp := dap.Response{
			ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
	}

	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	bps, err := session.SetBreakpoints(ctx, "/path/to/file.go", []int{10, 20})
	if err != nil {
		t.Fatalf("setBreakpoints: %v", err)
	}

	if len(bps) != 2 {
		t.Fatalf("expected 2 breakpoints, got %d", len(bps))
	}

	// Verify internal state
	stored := session.GetBreakpoints("/path/to/file.go")
	if len(stored) != 2 {
		t.Errorf("expected 2 stored breakpoints, got %d", len(stored))
	}
}

func TestSessionClearBreakpoints(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		body, _ := json.Marshal(dap.SetBreakpointsResponseBody{Breakpoints: []dap.Breakpoint{}})

		resp := dap.Response{
			ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
	}

	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := session.ClearBreakpoints(ctx, "/path/to/file.go")
	if err != nil {
		t.Fatalf("clearBreakpoints: %v", err)
	}

	stored := session.GetBreakpoints("/path/to/file.go")
	if len(stored) != 0 {
		t.Errorf("expected 0 stored breakpoints, got %d", len(stored))
	}
}

func TestSessionHandlers(t *testing.T) {
	mt := newMockTransport()
	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	var (
		stateChanges  []SessionState
		stoppedReason string
		stoppedThread int
		outputText    string
	)

	handlers := SessionHandlers{
		OnStateChanged: func(old, new SessionState) {
			stateChanges = append(stateChanges, new)
		},
		OnStopped: func(reason string, threadID int, allStopped bool) {
			stoppedReason = reason
			stoppedThread = threadID
		},
		OnOutput: func(category, output string) {
			outputText = output
		},
	}

	session.SetHandlers(handlers)

	// Simulate stopped event
	stoppedBody, _ := json.Marshal(dap.StoppedEventBody{
		Reason:   "breakpoint",
		ThreadID: 1,
	})
	stoppedEvt := dap.Event{
		ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "event"},
		Event:           "stopped",
		Body:            stoppedBody,
	}
	content, _ := json.Marshal(stoppedEvt)
	mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})

	// Simulate output event
	outputBody, _ := json.Marshal(dap.OutputEventBody{
		Category: "stdout",
		Output:   "Hello from debuggee",
	})
	outputEvt := dap.Event{
		ProtocolMessage: dap.ProtocolMessage{Seq: 2, Type: "event"},
		Event:           "output",
		Body:            outputBody,
	}
	content, _ = json.Marshal(outputEvt)
	mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	if stoppedReason != "breakpoint" {
		t.Errorf("expected stopped reason 'breakpoint', got '%s'", stoppedReason)
	}

	if stoppedThread != 1 {
		t.Errorf("expected stopped thread 1, got %d", stoppedThread)
	}

	if outputText != "Hello from debuggee" {
		t.Errorf("expected output 'Hello from debuggee', got '%s'", outputText)
	}

	// State should be stopped
	if session.State() != StateStopped {
		t.Errorf("expected state Stopped, got %v", session.State())
	}

	// Current thread should be set
	if session.CurrentThread() != 1 {
		t.Errorf("expected current thread 1, got %d", session.CurrentThread())
	}
}

func TestSessionStepping(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		resp := dap.Response{
			ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            json.RawMessage(`{}`),
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
	}

	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Test Next (step over)
	if err := session.Next(ctx, 1); err != nil {
		t.Errorf("next: %v", err)
	}

	// Test StepIn
	if err := session.StepIn(ctx, 1); err != nil {
		t.Errorf("stepIn: %v", err)
	}

	// Test StepOut
	if err := session.StepOut(ctx, 1); err != nil {
		t.Errorf("stepOut: %v", err)
	}
}

func TestSessionContinue(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		body, _ := json.Marshal(dap.ContinueResponseBody{AllThreadsContinued: true})

		resp := dap.Response{
			ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
	}

	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := session.Continue(ctx, 1); err != nil {
		t.Fatalf("continue: %v", err)
	}

	if session.State() != StateRunning {
		t.Errorf("expected state Running, got %v", session.State())
	}
}

func TestSessionGetStackTrace(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		frames := []dap.StackFrame{
			{ID: 1000, Name: "main.main", Line: 42},
			{ID: 1001, Name: "runtime.main", Line: 250},
		}
		body, _ := json.Marshal(dap.StackTraceResponseBody{
			StackFrames: frames,
			TotalFrames: 2,
		})

		resp := dap.Response{
			ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
	}

	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	frames, total, err := session.GetStackTrace(ctx, 1, 0, 20)
	if err != nil {
		t.Fatalf("getStackTrace: %v", err)
	}

	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}

	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}

	if frames[0].Name != "main.main" {
		t.Errorf("expected first frame 'main.main', got '%s'", frames[0].Name)
	}
}

func TestSessionGetVariables(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		vars := []dap.Variable{
			{Name: "x", Value: "42", Type: "int"},
			{Name: "y", Value: "hello", Type: "string"},
		}
		body, _ := json.Marshal(dap.VariablesResponseBody{Variables: vars})

		resp := dap.Response{
			ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
	}

	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	vars, err := session.GetVariables(ctx, 1)
	if err != nil {
		t.Fatalf("getVariables: %v", err)
	}

	if len(vars) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(vars))
	}

	if vars[0].Name != "x" || vars[0].Value != "42" {
		t.Errorf("unexpected first variable: %+v", vars[0])
	}
}

func TestSessionEvaluate(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		body, _ := json.Marshal(dap.EvaluateResponseBody{
			Result: "42",
			Type:   "int",
		})

		resp := dap.Response{
			ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
	}

	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := session.Evaluate(ctx, "x + y", 1000, "watch")
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if result.Result != "42" {
		t.Errorf("expected result '42', got '%s'", result.Result)
	}
}

func TestSessionDisconnect(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		resp := dap.Response{
			ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            json.RawMessage(`{}`),
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
	}

	client := dap.NewClient(mt)
	session := NewSession(client)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := session.Disconnect(ctx, true); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	if session.State() != StateDisconnected {
		t.Errorf("expected state Disconnected, got %v", session.State())
	}
}

func TestDefaultSessionConfig(t *testing.T) {
	config := DefaultSessionConfig()

	if config.AdapterID != "generic" {
		t.Errorf("expected AdapterID 'generic', got '%s'", config.AdapterID)
	}

	if config.ClientID != "keystorm" {
		t.Errorf("expected ClientID 'keystorm', got '%s'", config.ClientID)
	}

	if config.ClientName != "Keystorm Editor" {
		t.Errorf("expected ClientName 'Keystorm Editor', got '%s'", config.ClientName)
	}

	if !config.LinesStartAt1 {
		t.Error("expected LinesStartAt1 true")
	}

	if !config.ColumnsStartAt1 {
		t.Error("expected ColumnsStartAt1 true")
	}

	if config.PathFormat != "path" {
		t.Errorf("expected PathFormat 'path', got '%s'", config.PathFormat)
	}
}

func TestSessionThreads(t *testing.T) {
	mt := newMockTransport()

	mt.onSend = func(msg *dap.Message) {
		var req dap.Request
		json.Unmarshal(msg.Content, &req)

		threads := []dap.Thread{
			{ID: 1, Name: "main"},
			{ID: 2, Name: "worker"},
		}
		body, _ := json.Marshal(dap.ThreadsResponseBody{Threads: threads})

		resp := dap.Response{
			ProtocolMessage: dap.ProtocolMessage{Seq: 1, Type: "response"},
			RequestSeq:      req.Seq,
			Success:         true,
			Command:         req.Command,
			Body:            body,
		}

		content, _ := json.Marshal(resp)
		mt.queueResponse(&dap.Message{ContentLength: len(content), Content: content})
	}

	client := dap.NewClient(mt)
	session := NewSession(client)
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	threads, err := session.GetThreads(ctx)
	if err != nil {
		t.Fatalf("getThreads: %v", err)
	}

	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}

	// Verify internal state is updated
	stored := session.Threads()
	if len(stored) != 2 {
		t.Errorf("expected 2 stored threads, got %d", len(stored))
	}
}
