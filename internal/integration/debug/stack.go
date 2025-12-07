package debug

import (
	"context"
	"fmt"
	"sync"

	"github.com/dshills/keystorm/internal/integration/debug/dap"
)

// StackFrame represents a stack frame with enhanced information.
type StackFrame struct {
	// ID is the unique frame identifier.
	ID int

	// Name is the function name.
	Name string

	// Source is the source file information.
	Source *dap.Source

	// Line is the current line in the source.
	Line int

	// Column is the current column in the source.
	Column int

	// EndLine is the end line of the frame range.
	EndLine int

	// EndColumn is the end column of the frame range.
	EndColumn int

	// CanRestart indicates if execution can be restarted from this frame.
	CanRestart bool

	// InstructionPointerReference is the memory address of the instruction pointer.
	InstructionPointerReference string

	// ModuleID is the module this frame belongs to.
	ModuleID interface{}

	// PresentationHint is a hint for how to present the frame.
	// Values: "normal", "label", "subtle"
	PresentationHint string

	// Scopes are the variable scopes for this frame.
	Scopes []*VariableScope

	// IsCurrentFrame indicates if this is the current execution frame.
	IsCurrentFrame bool
}

// CallStack represents the call stack for a thread.
type CallStack struct {
	// ThreadID is the thread this call stack belongs to.
	ThreadID int

	// ThreadName is the name of the thread.
	ThreadName string

	// Frames are the stack frames in order (top of stack first).
	Frames []*StackFrame

	// TotalFrames is the total number of frames (may be more than Frames length).
	TotalFrames int

	// CurrentFrameIndex is the index of the currently selected frame.
	CurrentFrameIndex int
}

// HasSource returns true if the frame has source information.
func (f *StackFrame) HasSource() bool {
	return f.Source != nil && f.Source.Path != ""
}

// SourcePath returns the source file path, or empty string if unavailable.
func (f *StackFrame) SourcePath() string {
	if f.Source == nil {
		return ""
	}
	return f.Source.Path
}

// SourceName returns the source file name, or empty string if unavailable.
func (f *StackFrame) SourceName() string {
	if f.Source == nil {
		return ""
	}
	return f.Source.Name
}

// FormatLocation returns a formatted location string like "file.go:42".
func (f *StackFrame) FormatLocation() string {
	if f.Source == nil || f.Source.Name == "" {
		return fmt.Sprintf("<unknown>:%d", f.Line)
	}
	return fmt.Sprintf("%s:%d", f.Source.Name, f.Line)
}

// CurrentFrame returns the currently selected frame.
func (c *CallStack) CurrentFrame() *StackFrame {
	if c.CurrentFrameIndex < 0 || c.CurrentFrameIndex >= len(c.Frames) {
		return nil
	}
	return c.Frames[c.CurrentFrameIndex]
}

// IsAtTop returns true if the current frame is at the top of the stack.
func (c *CallStack) IsAtTop() bool {
	return c.CurrentFrameIndex == 0
}

// IsAtBottom returns true if the current frame is at the bottom of the loaded stack.
func (c *CallStack) IsAtBottom() bool {
	return c.CurrentFrameIndex == len(c.Frames)-1
}

// StackNavigator provides call stack navigation capabilities.
type StackNavigator struct {
	session *Session
	mu      sync.RWMutex

	// Current call stacks by thread ID
	stacks map[int]*CallStack

	// Currently selected thread
	currentThreadID int

	// Maximum frames to fetch per request
	maxFramesPerRequest int

	// Variable inspector for fetching scopes
	variableInspector *VariableInspector
}

// NewStackNavigator creates a new stack navigator.
func NewStackNavigator(session *Session) *StackNavigator {
	return &StackNavigator{
		session:             session,
		stacks:              make(map[int]*CallStack),
		maxFramesPerRequest: 20,
	}
}

// SetVariableInspector sets the variable inspector for fetching scopes.
func (n *StackNavigator) SetVariableInspector(vi *VariableInspector) {
	n.mu.Lock()
	n.variableInspector = vi
	n.mu.Unlock()
}

// GetCallStack retrieves the call stack for a thread.
func (n *StackNavigator) GetCallStack(ctx context.Context, threadID int) (*CallStack, error) {
	frames, totalFrames, err := n.session.GetStackTrace(ctx, threadID, 0, n.maxFramesPerRequest)
	if err != nil {
		return nil, fmt.Errorf("get stack trace: %w", err)
	}

	// Get thread name
	threads, _ := n.session.GetThreads(ctx)
	threadName := fmt.Sprintf("Thread %d", threadID)
	for _, t := range threads {
		if t.ID == threadID {
			threadName = t.Name
			break
		}
	}

	stack := &CallStack{
		ThreadID:          threadID,
		ThreadName:        threadName,
		Frames:            make([]*StackFrame, len(frames)),
		TotalFrames:       totalFrames,
		CurrentFrameIndex: 0,
	}

	for i, f := range frames {
		stack.Frames[i] = n.dapFrameToStackFrame(f)
		if i == 0 {
			stack.Frames[i].IsCurrentFrame = true
		}
	}

	n.mu.Lock()
	n.stacks[threadID] = stack
	n.currentThreadID = threadID
	n.mu.Unlock()

	return stack, nil
}

// dapFrameToStackFrame converts a DAP StackFrame to our StackFrame type.
func (n *StackNavigator) dapFrameToStackFrame(f dap.StackFrame) *StackFrame {
	return &StackFrame{
		ID:                          f.ID,
		Name:                        f.Name,
		Source:                      f.Source,
		Line:                        f.Line,
		Column:                      f.Column,
		EndLine:                     f.EndLine,
		EndColumn:                   f.EndColumn,
		CanRestart:                  f.CanRestart,
		InstructionPointerReference: f.InstructionPointerReference,
		ModuleID:                    f.ModuleID,
		PresentationHint:            f.PresentationHint,
	}
}

// FetchMoreFrames loads additional frames for a call stack.
func (n *StackNavigator) FetchMoreFrames(ctx context.Context, threadID int) error {
	n.mu.RLock()
	stack, ok := n.stacks[threadID]
	n.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no call stack for thread %d", threadID)
	}

	if len(stack.Frames) >= stack.TotalFrames {
		return nil // Already have all frames
	}

	startFrame := len(stack.Frames)
	frames, totalFrames, err := n.session.GetStackTrace(ctx, threadID, startFrame, n.maxFramesPerRequest)
	if err != nil {
		return fmt.Errorf("get more frames: %w", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	stack.TotalFrames = totalFrames
	for _, f := range frames {
		stack.Frames = append(stack.Frames, n.dapFrameToStackFrame(f))
	}

	return nil
}

// SelectFrame selects a frame in the call stack.
func (n *StackNavigator) SelectFrame(threadID int, frameIndex int) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	stack, ok := n.stacks[threadID]
	if !ok {
		return fmt.Errorf("no call stack for thread %d", threadID)
	}

	if frameIndex < 0 || frameIndex >= len(stack.Frames) {
		return fmt.Errorf("frame index %d out of range [0, %d)", frameIndex, len(stack.Frames))
	}

	// Update current frame flags
	if stack.CurrentFrameIndex < len(stack.Frames) {
		stack.Frames[stack.CurrentFrameIndex].IsCurrentFrame = false
	}
	stack.Frames[frameIndex].IsCurrentFrame = true
	stack.CurrentFrameIndex = frameIndex

	return nil
}

// SelectFrameUp moves up (towards caller) in the call stack.
func (n *StackNavigator) SelectFrameUp(threadID int) error {
	n.mu.RLock()
	stack, ok := n.stacks[threadID]
	if !ok {
		n.mu.RUnlock()
		return fmt.Errorf("no call stack for thread %d", threadID)
	}
	currentIndex := stack.CurrentFrameIndex
	n.mu.RUnlock()

	if currentIndex >= len(stack.Frames)-1 {
		return fmt.Errorf("already at bottom of stack")
	}

	return n.SelectFrame(threadID, currentIndex+1)
}

// SelectFrameDown moves down (towards callee) in the call stack.
func (n *StackNavigator) SelectFrameDown(threadID int) error {
	n.mu.RLock()
	stack, ok := n.stacks[threadID]
	if !ok {
		n.mu.RUnlock()
		return fmt.Errorf("no call stack for thread %d", threadID)
	}
	currentIndex := stack.CurrentFrameIndex
	n.mu.RUnlock()

	if currentIndex <= 0 {
		return fmt.Errorf("already at top of stack")
	}

	return n.SelectFrame(threadID, currentIndex-1)
}

// GetCurrentFrame returns the currently selected frame for a thread.
func (n *StackNavigator) GetCurrentFrame(threadID int) (*StackFrame, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	stack, ok := n.stacks[threadID]
	if !ok {
		return nil, fmt.Errorf("no call stack for thread %d", threadID)
	}

	return stack.CurrentFrame(), nil
}

// GetFrameScopes fetches the scopes for a frame.
func (n *StackNavigator) GetFrameScopes(ctx context.Context, frame *StackFrame) ([]*VariableScope, error) {
	n.mu.RLock()
	vi := n.variableInspector
	n.mu.RUnlock()

	if vi == nil {
		return nil, fmt.Errorf("variable inspector not set")
	}

	scopes, err := vi.GetScopes(ctx, frame.ID)
	if err != nil {
		return nil, err
	}

	frame.Scopes = scopes
	return scopes, nil
}

// GetAllStacks returns all loaded call stacks.
func (n *StackNavigator) GetAllStacks() map[int]*CallStack {
	n.mu.RLock()
	defer n.mu.RUnlock()

	result := make(map[int]*CallStack, len(n.stacks))
	for k, v := range n.stacks {
		result[k] = v
	}
	return result
}

// GetCurrentThreadID returns the currently selected thread ID.
func (n *StackNavigator) GetCurrentThreadID() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.currentThreadID
}

// SetCurrentThread sets the currently selected thread.
func (n *StackNavigator) SetCurrentThread(threadID int) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, ok := n.stacks[threadID]; !ok {
		return fmt.Errorf("no call stack for thread %d", threadID)
	}

	n.currentThreadID = threadID
	return nil
}

// ClearStacks clears all cached call stacks.
func (n *StackNavigator) ClearStacks() {
	n.mu.Lock()
	n.stacks = make(map[int]*CallStack)
	n.currentThreadID = 0
	n.mu.Unlock()
}

// RestartFrame restarts execution from a specific frame (if supported).
func (n *StackNavigator) RestartFrame(ctx context.Context, frameID int) error {
	caps := n.session.Capabilities()
	if caps == nil || !caps.SupportsRestartFrame {
		return fmt.Errorf("restart frame not supported")
	}

	return n.session.client.RestartFrame(ctx, dap.RestartFrameArguments{
		FrameID: frameID,
	})
}

// StepInTargets returns possible step-in targets for a frame (if supported).
func (n *StackNavigator) StepInTargets(ctx context.Context, frameID int) ([]StepInTarget, error) {
	caps := n.session.Capabilities()
	if caps == nil || !caps.SupportsStepInTargetsRequest {
		return nil, fmt.Errorf("step-in targets not supported")
	}

	args := dap.StepInTargetsArguments{
		FrameID: frameID,
	}

	targets, err := n.session.client.StepInTargets(ctx, args)
	if err != nil {
		return nil, err
	}

	result := make([]StepInTarget, len(targets))
	for i, t := range targets {
		result[i] = StepInTarget{
			ID:    t.ID,
			Label: t.Label,
		}
	}

	return result, nil
}

// StepInTarget represents a possible step-in target.
type StepInTarget struct {
	// ID is the unique identifier for this target.
	ID int

	// Label is the display name for this target.
	Label string
}

// GotoTargets returns possible goto targets for a source location (if supported).
func (n *StackNavigator) GotoTargets(ctx context.Context, source dap.Source, line int) ([]GotoTarget, error) {
	caps := n.session.Capabilities()
	if caps == nil || !caps.SupportsGotoTargetsRequest {
		return nil, fmt.Errorf("goto targets not supported")
	}

	args := dap.GotoTargetsArguments{
		Source: source,
		Line:   line,
	}

	targets, err := n.session.client.GotoTargets(ctx, args)
	if err != nil {
		return nil, err
	}

	result := make([]GotoTarget, len(targets))
	for i, t := range targets {
		result[i] = GotoTarget{
			ID:        t.ID,
			Label:     t.Label,
			Line:      t.Line,
			Column:    t.Column,
			EndLine:   t.EndLine,
			EndColumn: t.EndColumn,
		}
	}

	return result, nil
}

// GotoTarget represents a possible goto target.
type GotoTarget struct {
	// ID is the unique identifier for this target.
	ID int

	// Label is the display name for this target.
	Label string

	// Line is the line of the target.
	Line int

	// Column is the column of the target.
	Column int

	// EndLine is the optional end line.
	EndLine int

	// EndColumn is the optional end column.
	EndColumn int
}

// Goto moves execution to a target (if supported).
func (n *StackNavigator) Goto(ctx context.Context, threadID int, targetID int) error {
	caps := n.session.Capabilities()
	if caps == nil || !caps.SupportsGotoTargetsRequest {
		return fmt.Errorf("goto not supported")
	}

	args := dap.GotoArguments{
		ThreadID: threadID,
		TargetID: targetID,
	}

	return n.session.client.Goto(ctx, args)
}

// FormatStackTrace returns a formatted string representation of the call stack.
func (n *StackNavigator) FormatStackTrace(threadID int) string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	stack, ok := n.stacks[threadID]
	if !ok {
		return ""
	}

	var result string
	for i, frame := range stack.Frames {
		marker := "  "
		if i == stack.CurrentFrameIndex {
			marker = "> "
		}
		result += fmt.Sprintf("%s#%d %s at %s\n", marker, i, frame.Name, frame.FormatLocation())
	}

	if len(stack.Frames) < stack.TotalFrames {
		result += fmt.Sprintf("  ... (%d more frames)\n", stack.TotalFrames-len(stack.Frames))
	}

	return result
}

// SetMaxFramesPerRequest sets the maximum frames to fetch per request.
func (n *StackNavigator) SetMaxFramesPerRequest(max int) {
	n.mu.Lock()
	n.maxFramesPerRequest = max
	n.mu.Unlock()
}
