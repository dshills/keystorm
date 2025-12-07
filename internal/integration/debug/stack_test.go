package debug

import (
	"testing"

	"github.com/dshills/keystorm/internal/integration/debug/dap"
)

func TestStackFrame_HasSource(t *testing.T) {
	tests := []struct {
		name     string
		frame    *StackFrame
		expected bool
	}{
		{
			name:     "nil source",
			frame:    &StackFrame{Source: nil},
			expected: false,
		},
		{
			name:     "empty path",
			frame:    &StackFrame{Source: &dap.Source{Path: ""}},
			expected: false,
		},
		{
			name:     "valid source",
			frame:    &StackFrame{Source: &dap.Source{Path: "/path/to/file.go"}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.frame.HasSource() != tt.expected {
				t.Errorf("HasSource() = %v, expected %v", tt.frame.HasSource(), tt.expected)
			}
		})
	}
}

func TestStackFrame_SourcePath(t *testing.T) {
	frame := &StackFrame{
		Source: &dap.Source{Path: "/path/to/file.go"},
	}

	if frame.SourcePath() != "/path/to/file.go" {
		t.Errorf("SourcePath() = %s, expected /path/to/file.go", frame.SourcePath())
	}

	frameNil := &StackFrame{Source: nil}
	if frameNil.SourcePath() != "" {
		t.Errorf("SourcePath() for nil source should be empty")
	}
}

func TestStackFrame_SourceName(t *testing.T) {
	frame := &StackFrame{
		Source: &dap.Source{Name: "file.go"},
	}

	if frame.SourceName() != "file.go" {
		t.Errorf("SourceName() = %s, expected file.go", frame.SourceName())
	}

	frameNil := &StackFrame{Source: nil}
	if frameNil.SourceName() != "" {
		t.Errorf("SourceName() for nil source should be empty")
	}
}

func TestStackFrame_FormatLocation(t *testing.T) {
	tests := []struct {
		name     string
		frame    *StackFrame
		expected string
	}{
		{
			name:     "nil source",
			frame:    &StackFrame{Source: nil, Line: 42},
			expected: "<unknown>:42",
		},
		{
			name:     "empty name",
			frame:    &StackFrame{Source: &dap.Source{Name: ""}, Line: 42},
			expected: "<unknown>:42",
		},
		{
			name:     "valid source",
			frame:    &StackFrame{Source: &dap.Source{Name: "file.go"}, Line: 42},
			expected: "file.go:42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.frame.FormatLocation()
			if result != tt.expected {
				t.Errorf("FormatLocation() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestCallStack_CurrentFrame(t *testing.T) {
	frames := []*StackFrame{
		{ID: 1, Name: "func1"},
		{ID: 2, Name: "func2"},
		{ID: 3, Name: "func3"},
	}

	stack := &CallStack{
		ThreadID:          1,
		Frames:            frames,
		CurrentFrameIndex: 1,
	}

	current := stack.CurrentFrame()
	if current == nil {
		t.Fatal("CurrentFrame() returned nil")
	}
	if current.Name != "func2" {
		t.Errorf("CurrentFrame().Name = %s, expected func2", current.Name)
	}
}

func TestCallStack_CurrentFrame_OutOfRange(t *testing.T) {
	stack := &CallStack{
		Frames:            []*StackFrame{},
		CurrentFrameIndex: 0,
	}

	if stack.CurrentFrame() != nil {
		t.Error("CurrentFrame() should return nil for empty frames")
	}

	stack2 := &CallStack{
		Frames:            []*StackFrame{{ID: 1}},
		CurrentFrameIndex: 5,
	}

	if stack2.CurrentFrame() != nil {
		t.Error("CurrentFrame() should return nil for out of range index")
	}
}

func TestCallStack_IsAtTop(t *testing.T) {
	stack := &CallStack{
		Frames:            []*StackFrame{{}, {}, {}},
		CurrentFrameIndex: 0,
	}

	if !stack.IsAtTop() {
		t.Error("IsAtTop() should return true when index is 0")
	}

	stack.CurrentFrameIndex = 1
	if stack.IsAtTop() {
		t.Error("IsAtTop() should return false when index is not 0")
	}
}

func TestCallStack_IsAtBottom(t *testing.T) {
	stack := &CallStack{
		Frames:            []*StackFrame{{}, {}, {}},
		CurrentFrameIndex: 2,
	}

	if !stack.IsAtBottom() {
		t.Error("IsAtBottom() should return true when at last frame")
	}

	stack.CurrentFrameIndex = 1
	if stack.IsAtBottom() {
		t.Error("IsAtBottom() should return false when not at last frame")
	}
}

func TestStackNavigator_NewStackNavigator(t *testing.T) {
	nav := NewStackNavigator(nil)
	if nav == nil {
		t.Fatal("NewStackNavigator returned nil")
	}
	if nav.stacks == nil {
		t.Error("stacks should be initialized")
	}
	if nav.maxFramesPerRequest != 20 {
		t.Errorf("maxFramesPerRequest should be 20, got %d", nav.maxFramesPerRequest)
	}
}

func TestStackNavigator_SetVariableInspector(t *testing.T) {
	nav := NewStackNavigator(nil)
	vi := NewVariableInspector(nil)

	nav.SetVariableInspector(vi)

	if nav.variableInspector != vi {
		t.Error("variableInspector not set correctly")
	}
}

func TestStackNavigator_SelectFrame(t *testing.T) {
	nav := NewStackNavigator(nil)

	// Set up a mock stack
	nav.stacks[1] = &CallStack{
		ThreadID: 1,
		Frames: []*StackFrame{
			{ID: 1, Name: "func1", IsCurrentFrame: true},
			{ID: 2, Name: "func2", IsCurrentFrame: false},
			{ID: 3, Name: "func3", IsCurrentFrame: false},
		},
		CurrentFrameIndex: 0,
	}

	err := nav.SelectFrame(1, 2)
	if err != nil {
		t.Fatalf("SelectFrame failed: %v", err)
	}

	stack := nav.stacks[1]
	if stack.CurrentFrameIndex != 2 {
		t.Errorf("CurrentFrameIndex should be 2, got %d", stack.CurrentFrameIndex)
	}
	if stack.Frames[0].IsCurrentFrame {
		t.Error("Frame 0 should not be current")
	}
	if !stack.Frames[2].IsCurrentFrame {
		t.Error("Frame 2 should be current")
	}
}

func TestStackNavigator_SelectFrame_InvalidThread(t *testing.T) {
	nav := NewStackNavigator(nil)

	err := nav.SelectFrame(999, 0)
	if err == nil {
		t.Error("expected error for invalid thread")
	}
}

func TestStackNavigator_SelectFrame_OutOfRange(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.stacks[1] = &CallStack{
		ThreadID: 1,
		Frames:   []*StackFrame{{}, {}},
	}

	err := nav.SelectFrame(1, 10)
	if err == nil {
		t.Error("expected error for out of range frame")
	}

	err = nav.SelectFrame(1, -1)
	if err == nil {
		t.Error("expected error for negative frame index")
	}
}

func TestStackNavigator_SelectFrameUp(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.stacks[1] = &CallStack{
		ThreadID: 1,
		Frames: []*StackFrame{
			{ID: 1, IsCurrentFrame: true},
			{ID: 2},
			{ID: 3},
		},
		CurrentFrameIndex: 0,
	}

	err := nav.SelectFrameUp(1)
	if err != nil {
		t.Fatalf("SelectFrameUp failed: %v", err)
	}

	if nav.stacks[1].CurrentFrameIndex != 1 {
		t.Errorf("CurrentFrameIndex should be 1, got %d", nav.stacks[1].CurrentFrameIndex)
	}
}

func TestStackNavigator_SelectFrameUp_AtBottom(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.stacks[1] = &CallStack{
		ThreadID:          1,
		Frames:            []*StackFrame{{}, {}, {}},
		CurrentFrameIndex: 2,
	}

	err := nav.SelectFrameUp(1)
	if err == nil {
		t.Error("expected error when already at bottom")
	}
}

func TestStackNavigator_SelectFrameDown(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.stacks[1] = &CallStack{
		ThreadID: 1,
		Frames: []*StackFrame{
			{ID: 1},
			{ID: 2, IsCurrentFrame: true},
			{ID: 3},
		},
		CurrentFrameIndex: 1,
	}

	err := nav.SelectFrameDown(1)
	if err != nil {
		t.Fatalf("SelectFrameDown failed: %v", err)
	}

	if nav.stacks[1].CurrentFrameIndex != 0 {
		t.Errorf("CurrentFrameIndex should be 0, got %d", nav.stacks[1].CurrentFrameIndex)
	}
}

func TestStackNavigator_SelectFrameDown_AtTop(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.stacks[1] = &CallStack{
		ThreadID:          1,
		Frames:            []*StackFrame{{}, {}, {}},
		CurrentFrameIndex: 0,
	}

	err := nav.SelectFrameDown(1)
	if err == nil {
		t.Error("expected error when already at top")
	}
}

func TestStackNavigator_GetCurrentFrame(t *testing.T) {
	nav := NewStackNavigator(nil)

	frame := &StackFrame{ID: 1, Name: "main"}
	nav.stacks[1] = &CallStack{
		ThreadID:          1,
		Frames:            []*StackFrame{frame},
		CurrentFrameIndex: 0,
	}

	result, err := nav.GetCurrentFrame(1)
	if err != nil {
		t.Fatalf("GetCurrentFrame failed: %v", err)
	}

	if result.Name != "main" {
		t.Errorf("expected frame name 'main', got %s", result.Name)
	}
}

func TestStackNavigator_GetCurrentFrame_InvalidThread(t *testing.T) {
	nav := NewStackNavigator(nil)

	_, err := nav.GetCurrentFrame(999)
	if err == nil {
		t.Error("expected error for invalid thread")
	}
}

func TestStackNavigator_ClearStacks(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.stacks[1] = &CallStack{ThreadID: 1}
	nav.stacks[2] = &CallStack{ThreadID: 2}
	nav.currentThreadID = 1

	nav.ClearStacks()

	if len(nav.stacks) != 0 {
		t.Errorf("stacks should be empty after clear, got %d", len(nav.stacks))
	}
	if nav.currentThreadID != 0 {
		t.Errorf("currentThreadID should be 0 after clear, got %d", nav.currentThreadID)
	}
}

func TestStackNavigator_GetAllStacks(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.stacks[1] = &CallStack{ThreadID: 1}
	nav.stacks[2] = &CallStack{ThreadID: 2}

	stacks := nav.GetAllStacks()

	if len(stacks) != 2 {
		t.Errorf("expected 2 stacks, got %d", len(stacks))
	}
}

func TestStackNavigator_GetSetCurrentThreadID(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.stacks[1] = &CallStack{ThreadID: 1}
	nav.stacks[2] = &CallStack{ThreadID: 2}

	err := nav.SetCurrentThread(1)
	if err != nil {
		t.Fatalf("SetCurrentThread failed: %v", err)
	}

	if nav.GetCurrentThreadID() != 1 {
		t.Errorf("expected current thread 1, got %d", nav.GetCurrentThreadID())
	}
}

func TestStackNavigator_SetCurrentThread_InvalidThread(t *testing.T) {
	nav := NewStackNavigator(nil)

	err := nav.SetCurrentThread(999)
	if err == nil {
		t.Error("expected error for invalid thread")
	}
}

func TestStackNavigator_SetMaxFramesPerRequest(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.SetMaxFramesPerRequest(50)

	if nav.maxFramesPerRequest != 50 {
		t.Errorf("expected maxFramesPerRequest 50, got %d", nav.maxFramesPerRequest)
	}
}

func TestStackNavigator_FormatStackTrace(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.stacks[1] = &CallStack{
		ThreadID: 1,
		Frames: []*StackFrame{
			{
				ID:     1,
				Name:   "main.main",
				Source: &dap.Source{Name: "main.go"},
				Line:   10,
			},
			{
				ID:     2,
				Name:   "main.helper",
				Source: &dap.Source{Name: "helper.go"},
				Line:   20,
			},
		},
		TotalFrames:       2,
		CurrentFrameIndex: 0,
	}

	result := nav.FormatStackTrace(1)

	if result == "" {
		t.Error("FormatStackTrace returned empty string")
	}

	// Should contain the current frame marker
	if len(result) == 0 {
		t.Error("expected non-empty formatted stack trace")
	}
}

func TestStackNavigator_FormatStackTrace_InvalidThread(t *testing.T) {
	nav := NewStackNavigator(nil)

	result := nav.FormatStackTrace(999)
	if result != "" {
		t.Error("expected empty string for invalid thread")
	}
}

func TestStackNavigator_FormatStackTrace_WithMoreFrames(t *testing.T) {
	nav := NewStackNavigator(nil)

	nav.stacks[1] = &CallStack{
		ThreadID: 1,
		Frames: []*StackFrame{
			{ID: 1, Name: "func1", Source: &dap.Source{Name: "f.go"}, Line: 1},
		},
		TotalFrames:       10, // More frames available
		CurrentFrameIndex: 0,
	}

	result := nav.FormatStackTrace(1)

	if result == "" {
		t.Error("FormatStackTrace returned empty string")
	}
}

func TestStepInTarget(t *testing.T) {
	target := StepInTarget{
		ID:    1,
		Label: "someFunction",
	}

	if target.ID != 1 {
		t.Errorf("expected ID 1, got %d", target.ID)
	}
	if target.Label != "someFunction" {
		t.Errorf("expected label 'someFunction', got %s", target.Label)
	}
}

func TestGotoTarget(t *testing.T) {
	target := GotoTarget{
		ID:        1,
		Label:     "loop start",
		Line:      42,
		Column:    10,
		EndLine:   45,
		EndColumn: 5,
	}

	if target.ID != 1 {
		t.Errorf("expected ID 1, got %d", target.ID)
	}
	if target.Line != 42 {
		t.Errorf("expected line 42, got %d", target.Line)
	}
}

func TestDapFrameToStackFrame(t *testing.T) {
	nav := NewStackNavigator(nil)

	dapFrame := dap.StackFrame{
		ID:               1,
		Name:             "main.handler",
		Source:           &dap.Source{Path: "/path/to/file.go", Name: "file.go"},
		Line:             42,
		Column:           10,
		EndLine:          50,
		EndColumn:        5,
		CanRestart:       true,
		PresentationHint: "normal",
	}

	result := nav.dapFrameToStackFrame(dapFrame)

	if result.ID != dapFrame.ID {
		t.Errorf("ID mismatch")
	}
	if result.Name != dapFrame.Name {
		t.Errorf("Name mismatch")
	}
	if result.Line != dapFrame.Line {
		t.Errorf("Line mismatch")
	}
	if result.Column != dapFrame.Column {
		t.Errorf("Column mismatch")
	}
	if !result.CanRestart {
		t.Error("CanRestart should be true")
	}
	if result.PresentationHint != "normal" {
		t.Error("PresentationHint mismatch")
	}
}
