package dap

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// Client is a DAP client that communicates with a debug adapter.
type Client struct {
	transport Transport
	seq       int64
	pending   map[int]*pendingRequest
	pendingMu sync.RWMutex
	handlers  eventHandlers
	handlerMu sync.RWMutex
	done      chan struct{}
	closeOnce sync.Once
	err       error
	errMu     sync.RWMutex
}

// pendingRequest tracks a pending request awaiting response.
type pendingRequest struct {
	done      chan struct{}
	closeOnce sync.Once
	response  *Response
	err       error
}

// close safely closes the done channel.
func (p *pendingRequest) close() {
	p.closeOnce.Do(func() {
		close(p.done)
	})
}

// eventHandlers stores event handler functions.
type eventHandlers struct {
	onInitialized    func()
	onStopped        func(StoppedEventBody)
	onContinued      func(ContinuedEventBody)
	onExited         func(ExitedEventBody)
	onTerminated     func(TerminatedEventBody)
	onThread         func(ThreadEventBody)
	onOutput         func(OutputEventBody)
	onBreakpoint     func(BreakpointEventBody)
	onModule         func(ModuleEventBody)
	onLoadedSource   func(LoadedSourceEventBody)
	onProcess        func(ProcessEventBody)
	onCapabilities   func(CapabilitiesEventBody)
	onProgressStart  func(ProgressStartEventBody)
	onProgressUpdate func(ProgressUpdateEventBody)
	onProgressEnd    func(ProgressEndEventBody)
	onAny            func(Event)
}

// NewClient creates a new DAP client with the given transport.
func NewClient(transport Transport) *Client {
	c := &Client{
		transport: transport,
		pending:   make(map[int]*pendingRequest),
		done:      make(chan struct{}),
	}
	go c.receiveLoop()
	return c
}

// Close closes the client and underlying transport.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		close(c.done)
	})
	return c.transport.Close()
}

// Error returns any error that occurred during receive.
func (c *Client) Error() error {
	c.errMu.RLock()
	defer c.errMu.RUnlock()
	return c.err
}

// receiveLoop continuously receives messages from the transport.
func (c *Client) receiveLoop() {
	for {
		msg, err := c.transport.Receive()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-c.done:
				return
			default:
			}

			c.errMu.Lock()
			c.err = err
			c.errMu.Unlock()

			// Cancel all pending requests
			c.pendingMu.Lock()
			for _, req := range c.pending {
				req.err = err
				req.close()
			}
			c.pending = make(map[int]*pendingRequest)
			c.pendingMu.Unlock()
			return
		}

		// Check if we're shutting down
		select {
		case <-c.done:
			return
		default:
		}

		c.handleMessage(msg)
	}
}

// handleMessage dispatches a received message.
func (c *Client) handleMessage(msg *Message) {
	var base ProtocolMessage
	if err := json.Unmarshal(msg.Content, &base); err != nil {
		return
	}

	switch base.Type {
	case "response":
		c.handleResponse(msg.Content)
	case "event":
		c.handleEvent(msg.Content)
	}
}

// handleResponse processes a response message.
func (c *Client) handleResponse(content []byte) {
	var resp Response
	if err := json.Unmarshal(content, &resp); err != nil {
		return
	}

	c.pendingMu.Lock()
	req, ok := c.pending[resp.RequestSeq]
	if ok {
		delete(c.pending, resp.RequestSeq)
	}
	c.pendingMu.Unlock()

	if ok {
		req.response = &resp
		req.close()
	}
}

// handleEvent processes an event message.
func (c *Client) handleEvent(content []byte) {
	var evt Event
	if err := json.Unmarshal(content, &evt); err != nil {
		return
	}

	c.handlerMu.RLock()
	handlers := c.handlers
	c.handlerMu.RUnlock()

	// Call specific handler
	switch evt.Event {
	case "initialized":
		if handlers.onInitialized != nil {
			handlers.onInitialized()
		}
	case "stopped":
		if handlers.onStopped != nil {
			var body StoppedEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onStopped(body)
			}
		}
	case "continued":
		if handlers.onContinued != nil {
			var body ContinuedEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onContinued(body)
			}
		}
	case "exited":
		if handlers.onExited != nil {
			var body ExitedEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onExited(body)
			}
		}
	case "terminated":
		if handlers.onTerminated != nil {
			var body TerminatedEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onTerminated(body)
			}
		}
	case "thread":
		if handlers.onThread != nil {
			var body ThreadEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onThread(body)
			}
		}
	case "output":
		if handlers.onOutput != nil {
			var body OutputEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onOutput(body)
			}
		}
	case "breakpoint":
		if handlers.onBreakpoint != nil {
			var body BreakpointEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onBreakpoint(body)
			}
		}
	case "module":
		if handlers.onModule != nil {
			var body ModuleEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onModule(body)
			}
		}
	case "loadedSource":
		if handlers.onLoadedSource != nil {
			var body LoadedSourceEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onLoadedSource(body)
			}
		}
	case "process":
		if handlers.onProcess != nil {
			var body ProcessEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onProcess(body)
			}
		}
	case "capabilities":
		if handlers.onCapabilities != nil {
			var body CapabilitiesEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onCapabilities(body)
			}
		}
	case "progressStart":
		if handlers.onProgressStart != nil {
			var body ProgressStartEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onProgressStart(body)
			}
		}
	case "progressUpdate":
		if handlers.onProgressUpdate != nil {
			var body ProgressUpdateEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onProgressUpdate(body)
			}
		}
	case "progressEnd":
		if handlers.onProgressEnd != nil {
			var body ProgressEndEventBody
			if err := json.Unmarshal(evt.Body, &body); err == nil {
				handlers.onProgressEnd(body)
			}
		}
	}

	// Always call onAny if set
	if handlers.onAny != nil {
		handlers.onAny(evt)
	}
}

// sendRequest sends a request and waits for the response.
func (c *Client) sendRequest(ctx context.Context, command string, args interface{}) (*Response, error) {
	seq := int(atomic.AddInt64(&c.seq, 1))

	var argsJSON json.RawMessage
	if args != nil {
		var err error
		argsJSON, err = json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("marshal arguments: %w", err)
		}
	}

	req := Request{
		ProtocolMessage: ProtocolMessage{
			Seq:  seq,
			Type: "request",
		},
		Command:   command,
		Arguments: argsJSON,
	}

	content, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	pending := &pendingRequest{
		done: make(chan struct{}),
	}

	c.pendingMu.Lock()
	c.pending[seq] = pending
	c.pendingMu.Unlock()

	msg := &Message{
		ContentLength: len(content),
		Content:       content,
	}

	if err := c.transport.Send(msg); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, seq)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("send request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, seq)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-pending.done:
		if pending.err != nil {
			return nil, pending.err
		}
		return pending.response, nil
	}
}

// Event handler setters

// OnInitialized sets the handler for the initialized event.
func (c *Client) OnInitialized(handler func()) {
	c.handlerMu.Lock()
	c.handlers.onInitialized = handler
	c.handlerMu.Unlock()
}

// OnStopped sets the handler for the stopped event.
func (c *Client) OnStopped(handler func(StoppedEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onStopped = handler
	c.handlerMu.Unlock()
}

// OnContinued sets the handler for the continued event.
func (c *Client) OnContinued(handler func(ContinuedEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onContinued = handler
	c.handlerMu.Unlock()
}

// OnExited sets the handler for the exited event.
func (c *Client) OnExited(handler func(ExitedEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onExited = handler
	c.handlerMu.Unlock()
}

// OnTerminated sets the handler for the terminated event.
func (c *Client) OnTerminated(handler func(TerminatedEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onTerminated = handler
	c.handlerMu.Unlock()
}

// OnThread sets the handler for the thread event.
func (c *Client) OnThread(handler func(ThreadEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onThread = handler
	c.handlerMu.Unlock()
}

// OnOutput sets the handler for the output event.
func (c *Client) OnOutput(handler func(OutputEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onOutput = handler
	c.handlerMu.Unlock()
}

// OnBreakpoint sets the handler for the breakpoint event.
func (c *Client) OnBreakpoint(handler func(BreakpointEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onBreakpoint = handler
	c.handlerMu.Unlock()
}

// OnModule sets the handler for the module event.
func (c *Client) OnModule(handler func(ModuleEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onModule = handler
	c.handlerMu.Unlock()
}

// OnLoadedSource sets the handler for the loadedSource event.
func (c *Client) OnLoadedSource(handler func(LoadedSourceEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onLoadedSource = handler
	c.handlerMu.Unlock()
}

// OnProcess sets the handler for the process event.
func (c *Client) OnProcess(handler func(ProcessEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onProcess = handler
	c.handlerMu.Unlock()
}

// OnCapabilities sets the handler for the capabilities event.
func (c *Client) OnCapabilities(handler func(CapabilitiesEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onCapabilities = handler
	c.handlerMu.Unlock()
}

// OnProgressStart sets the handler for the progressStart event.
func (c *Client) OnProgressStart(handler func(ProgressStartEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onProgressStart = handler
	c.handlerMu.Unlock()
}

// OnProgressUpdate sets the handler for the progressUpdate event.
func (c *Client) OnProgressUpdate(handler func(ProgressUpdateEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onProgressUpdate = handler
	c.handlerMu.Unlock()
}

// OnProgressEnd sets the handler for the progressEnd event.
func (c *Client) OnProgressEnd(handler func(ProgressEndEventBody)) {
	c.handlerMu.Lock()
	c.handlers.onProgressEnd = handler
	c.handlerMu.Unlock()
}

// OnAnyEvent sets a handler for all events.
func (c *Client) OnAnyEvent(handler func(Event)) {
	c.handlerMu.Lock()
	c.handlers.onAny = handler
	c.handlerMu.Unlock()
}

// DAP Request Methods

// Initialize sends the initialize request.
func (c *Client) Initialize(ctx context.Context, args InitializeRequestArguments) (*Capabilities, error) {
	resp, err := c.sendRequest(ctx, "initialize", args)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("initialize failed: %s", resp.Message)
	}

	var caps Capabilities
	if err := json.Unmarshal(resp.Body, &caps); err != nil {
		return nil, fmt.Errorf("unmarshal capabilities: %w", err)
	}

	return &caps, nil
}

// ConfigurationDone sends the configurationDone request.
func (c *Client) ConfigurationDone(ctx context.Context) error {
	resp, err := c.sendRequest(ctx, "configurationDone", nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("configurationDone failed: %s", resp.Message)
	}

	return nil
}

// Launch sends the launch request.
func (c *Client) Launch(ctx context.Context, args interface{}) error {
	resp, err := c.sendRequest(ctx, "launch", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("launch failed: %s", resp.Message)
	}

	return nil
}

// Attach sends the attach request.
func (c *Client) Attach(ctx context.Context, args interface{}) error {
	resp, err := c.sendRequest(ctx, "attach", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("attach failed: %s", resp.Message)
	}

	return nil
}

// Disconnect sends the disconnect request.
func (c *Client) Disconnect(ctx context.Context, args DisconnectArguments) error {
	resp, err := c.sendRequest(ctx, "disconnect", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("disconnect failed: %s", resp.Message)
	}

	return nil
}

// Terminate sends the terminate request.
func (c *Client) Terminate(ctx context.Context, args TerminateArguments) error {
	resp, err := c.sendRequest(ctx, "terminate", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("terminate failed: %s", resp.Message)
	}

	return nil
}

// SetBreakpoints sends the setBreakpoints request.
func (c *Client) SetBreakpoints(ctx context.Context, args SetBreakpointsArguments) ([]Breakpoint, error) {
	resp, err := c.sendRequest(ctx, "setBreakpoints", args)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("setBreakpoints failed: %s", resp.Message)
	}

	var body SetBreakpointsResponseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("unmarshal breakpoints: %w", err)
	}

	return body.Breakpoints, nil
}

// SetFunctionBreakpoints sends the setFunctionBreakpoints request.
func (c *Client) SetFunctionBreakpoints(ctx context.Context, args SetFunctionBreakpointsArguments) ([]Breakpoint, error) {
	resp, err := c.sendRequest(ctx, "setFunctionBreakpoints", args)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("setFunctionBreakpoints failed: %s", resp.Message)
	}

	var body SetBreakpointsResponseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("unmarshal breakpoints: %w", err)
	}

	return body.Breakpoints, nil
}

// SetExceptionBreakpoints sends the setExceptionBreakpoints request.
func (c *Client) SetExceptionBreakpoints(ctx context.Context, args SetExceptionBreakpointsArguments) error {
	resp, err := c.sendRequest(ctx, "setExceptionBreakpoints", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("setExceptionBreakpoints failed: %s", resp.Message)
	}

	return nil
}

// Continue sends the continue request.
func (c *Client) Continue(ctx context.Context, args ContinueArguments) (*ContinueResponseBody, error) {
	resp, err := c.sendRequest(ctx, "continue", args)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("continue failed: %s", resp.Message)
	}

	var body ContinueResponseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("unmarshal continue response: %w", err)
	}

	return &body, nil
}

// Next sends the next (step over) request.
func (c *Client) Next(ctx context.Context, args NextArguments) error {
	resp, err := c.sendRequest(ctx, "next", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("next failed: %s", resp.Message)
	}

	return nil
}

// StepIn sends the stepIn request.
func (c *Client) StepIn(ctx context.Context, args StepInArguments) error {
	resp, err := c.sendRequest(ctx, "stepIn", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("stepIn failed: %s", resp.Message)
	}

	return nil
}

// StepOut sends the stepOut request.
func (c *Client) StepOut(ctx context.Context, args StepOutArguments) error {
	resp, err := c.sendRequest(ctx, "stepOut", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("stepOut failed: %s", resp.Message)
	}

	return nil
}

// Pause sends the pause request.
func (c *Client) Pause(ctx context.Context, args PauseArguments) error {
	resp, err := c.sendRequest(ctx, "pause", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("pause failed: %s", resp.Message)
	}

	return nil
}

// Threads sends the threads request.
func (c *Client) Threads(ctx context.Context) ([]Thread, error) {
	resp, err := c.sendRequest(ctx, "threads", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("threads failed: %s", resp.Message)
	}

	var body ThreadsResponseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("unmarshal threads: %w", err)
	}

	return body.Threads, nil
}

// StackTrace sends the stackTrace request.
func (c *Client) StackTrace(ctx context.Context, args StackTraceArguments) (*StackTraceResponseBody, error) {
	resp, err := c.sendRequest(ctx, "stackTrace", args)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("stackTrace failed: %s", resp.Message)
	}

	var body StackTraceResponseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("unmarshal stackTrace: %w", err)
	}

	return &body, nil
}

// Scopes sends the scopes request.
func (c *Client) Scopes(ctx context.Context, args ScopesArguments) ([]Scope, error) {
	resp, err := c.sendRequest(ctx, "scopes", args)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("scopes failed: %s", resp.Message)
	}

	var body ScopesResponseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("unmarshal scopes: %w", err)
	}

	return body.Scopes, nil
}

// Variables sends the variables request.
func (c *Client) Variables(ctx context.Context, args VariablesArguments) ([]Variable, error) {
	resp, err := c.sendRequest(ctx, "variables", args)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("variables failed: %s", resp.Message)
	}

	var body VariablesResponseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("unmarshal variables: %w", err)
	}

	return body.Variables, nil
}

// SetVariable sends the setVariable request.
func (c *Client) SetVariable(ctx context.Context, args SetVariableArguments) (*SetVariableResponseBody, error) {
	resp, err := c.sendRequest(ctx, "setVariable", args)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("setVariable failed: %s", resp.Message)
	}

	var body SetVariableResponseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("unmarshal setVariable: %w", err)
	}

	return &body, nil
}

// Evaluate sends the evaluate request.
func (c *Client) Evaluate(ctx context.Context, args EvaluateArguments) (*EvaluateResponseBody, error) {
	resp, err := c.sendRequest(ctx, "evaluate", args)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("evaluate failed: %s", resp.Message)
	}

	var body EvaluateResponseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("unmarshal evaluate: %w", err)
	}

	return &body, nil
}

// Source sends the source request.
func (c *Client) Source(ctx context.Context, args SourceArguments) (*SourceResponseBody, error) {
	resp, err := c.sendRequest(ctx, "source", args)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("source failed: %s", resp.Message)
	}

	var body SourceResponseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("unmarshal source: %w", err)
	}

	return &body, nil
}
