package debug

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBreakpointManager_AddLineBreakpoint(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp, err := mgr.AddLineBreakpoint("/path/to/file.go", 42)
	if err != nil {
		t.Fatalf("AddLineBreakpoint failed: %v", err)
	}

	if bp.Path != "/path/to/file.go" {
		t.Errorf("expected path /path/to/file.go, got %s", bp.Path)
	}
	if bp.Line != 42 {
		t.Errorf("expected line 42, got %d", bp.Line)
	}
	if bp.Type != BreakpointTypeLine {
		t.Errorf("expected type Line, got %v", bp.Type)
	}
	if !bp.Enabled {
		t.Error("expected breakpoint to be enabled")
	}
}

func TestBreakpointManager_AddConditionalBreakpoint(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp, err := mgr.AddConditionalBreakpoint("/path/to/file.go", 10, "x > 5")
	if err != nil {
		t.Fatalf("AddConditionalBreakpoint failed: %v", err)
	}

	if bp.Condition != "x > 5" {
		t.Errorf("expected condition 'x > 5', got %s", bp.Condition)
	}
	if bp.Type != BreakpointTypeConditional {
		t.Errorf("expected type Conditional, got %v", bp.Type)
	}
}

func TestBreakpointManager_AddLogPoint(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp, err := mgr.AddLogPoint("/path/to/file.go", 20, "Value: {x}")
	if err != nil {
		t.Fatalf("AddLogPoint failed: %v", err)
	}

	if bp.LogMessage != "Value: {x}" {
		t.Errorf("expected log message 'Value: {x}', got %s", bp.LogMessage)
	}
	if bp.Type != BreakpointTypeLogPoint {
		t.Errorf("expected type LogPoint, got %v", bp.Type)
	}
}

func TestBreakpointManager_AddFunctionBreakpoint(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp, err := mgr.AddFunctionBreakpoint("main.handler", "")
	if err != nil {
		t.Fatalf("AddFunctionBreakpoint failed: %v", err)
	}

	if bp.FunctionName != "main.handler" {
		t.Errorf("expected function name 'main.handler', got %s", bp.FunctionName)
	}
	if bp.Type != BreakpointTypeFunction {
		t.Errorf("expected type Function, got %v", bp.Type)
	}
}

func TestBreakpointManager_RemoveBreakpoint(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp, _ := mgr.AddLineBreakpoint("/path/to/file.go", 42)

	err := mgr.RemoveBreakpoint(bp.ID)
	if err != nil {
		t.Fatalf("RemoveBreakpoint failed: %v", err)
	}

	_, ok := mgr.GetBreakpoint(bp.ID)
	if ok {
		t.Error("expected breakpoint to be removed")
	}
}

func TestBreakpointManager_RemoveNonexistent(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	err := mgr.RemoveBreakpoint(999)
	if err == nil {
		t.Error("expected error removing nonexistent breakpoint")
	}
}

func TestBreakpointManager_ToggleBreakpoint(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	// Toggle on - creates new breakpoint
	bp, created, err := mgr.ToggleBreakpoint("/path/to/file.go", 42)
	if err != nil {
		t.Fatalf("ToggleBreakpoint failed: %v", err)
	}
	if !created {
		t.Error("expected breakpoint to be created")
	}
	if bp == nil {
		t.Fatal("expected breakpoint to be returned")
	}

	// Toggle off - removes breakpoint
	bp2, created, err := mgr.ToggleBreakpoint("/path/to/file.go", 42)
	if err != nil {
		t.Fatalf("ToggleBreakpoint failed: %v", err)
	}
	if created {
		t.Error("expected breakpoint to be removed, not created")
	}
	if bp2 == nil {
		t.Error("expected removed breakpoint to be returned")
	}
}

func TestBreakpointManager_EnableDisable(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp, _ := mgr.AddLineBreakpoint("/path/to/file.go", 42)

	// Disable
	err := mgr.SetEnabled(bp.ID, false)
	if err != nil {
		t.Fatalf("SetEnabled(false) failed: %v", err)
	}
	if bp.Enabled {
		t.Error("expected breakpoint to be disabled")
	}

	// Enable
	err = mgr.SetEnabled(bp.ID, true)
	if err != nil {
		t.Fatalf("SetEnabled(true) failed: %v", err)
	}
	if !bp.Enabled {
		t.Error("expected breakpoint to be enabled")
	}
}

func TestBreakpointManager_SetCondition(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp, _ := mgr.AddLineBreakpoint("/path/to/file.go", 42)

	err := mgr.SetCondition(bp.ID, "i > 10")
	if err != nil {
		t.Fatalf("SetCondition failed: %v", err)
	}
	if bp.Condition != "i > 10" {
		t.Errorf("expected condition 'i > 10', got %s", bp.Condition)
	}
	if bp.Type != BreakpointTypeConditional {
		t.Errorf("expected type Conditional after setting condition, got %v", bp.Type)
	}
}

func TestBreakpointManager_SetHitCondition(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp, _ := mgr.AddLineBreakpoint("/path/to/file.go", 42)

	err := mgr.SetHitCondition(bp.ID, ">= 5")
	if err != nil {
		t.Fatalf("SetHitCondition failed: %v", err)
	}
	if bp.HitCondition != ">= 5" {
		t.Errorf("expected hit condition '>= 5', got %s", bp.HitCondition)
	}
}

func TestBreakpointManager_GetBreakpointsForPath(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	mgr.AddLineBreakpoint("/path/to/file1.go", 10)
	mgr.AddLineBreakpoint("/path/to/file1.go", 20)
	mgr.AddLineBreakpoint("/path/to/file2.go", 30)

	bps := mgr.GetBreakpointsForPath("/path/to/file1.go")
	if len(bps) != 2 {
		t.Errorf("expected 2 breakpoints for file1.go, got %d", len(bps))
	}

	bps = mgr.GetBreakpointsForPath("/path/to/file2.go")
	if len(bps) != 1 {
		t.Errorf("expected 1 breakpoint for file2.go, got %d", len(bps))
	}

	bps = mgr.GetBreakpointsForPath("/path/to/nonexistent.go")
	if len(bps) != 0 {
		t.Errorf("expected 0 breakpoints for nonexistent file, got %d", len(bps))
	}
}

func TestBreakpointManager_GetAllBreakpoints(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	mgr.AddLineBreakpoint("/path/to/file1.go", 10)
	mgr.AddLineBreakpoint("/path/to/file2.go", 20)
	mgr.AddFunctionBreakpoint("main.handler", "")

	bps := mgr.GetAllBreakpoints()
	if len(bps) != 3 {
		t.Errorf("expected 3 breakpoints, got %d", len(bps))
	}
}

func TestBreakpointManager_ClearAll(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	mgr.AddLineBreakpoint("/path/to/file1.go", 10)
	mgr.AddLineBreakpoint("/path/to/file2.go", 20)
	mgr.AddFunctionBreakpoint("main.handler", "")

	mgr.ClearAll()

	bps := mgr.GetAllBreakpoints()
	if len(bps) != 0 {
		t.Errorf("expected 0 breakpoints after clear, got %d", len(bps))
	}
}

func TestBreakpointManager_ClearForPath(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	mgr.AddLineBreakpoint("/path/to/file1.go", 10)
	mgr.AddLineBreakpoint("/path/to/file1.go", 20)
	mgr.AddLineBreakpoint("/path/to/file2.go", 30)

	mgr.ClearForPath("/path/to/file1.go")

	bps := mgr.GetBreakpointsForPath("/path/to/file1.go")
	if len(bps) != 0 {
		t.Errorf("expected 0 breakpoints for file1.go after clear, got %d", len(bps))
	}

	bps = mgr.GetBreakpointsForPath("/path/to/file2.go")
	if len(bps) != 1 {
		t.Errorf("expected 1 breakpoint for file2.go, got %d", len(bps))
	}
}

func TestBreakpointManager_Persistence(t *testing.T) {
	tempDir := t.TempDir()
	persistPath := filepath.Join(tempDir, "breakpoints.json")

	// Create manager and add breakpoints
	mgr := NewBreakpointManager(nil)
	mgr.SetPersistPath(persistPath)

	mgr.AddLineBreakpoint("/path/to/file1.go", 10)
	mgr.AddConditionalBreakpoint("/path/to/file2.go", 20, "x > 5")
	mgr.AddFunctionBreakpoint("main.handler", "")

	// Save
	err := mgr.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(persistPath); os.IsNotExist(err) {
		t.Fatal("persistence file not created")
	}

	// Create new manager and load
	mgr2 := NewBreakpointManager(nil)
	mgr2.SetPersistPath(persistPath)

	err = mgr2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	bps := mgr2.GetAllBreakpoints()
	if len(bps) != 3 {
		t.Errorf("expected 3 breakpoints after load, got %d", len(bps))
	}
}

func TestBreakpointManager_LoadNonexistent(t *testing.T) {
	mgr := NewBreakpointManager(nil)
	mgr.SetPersistPath("/nonexistent/path/breakpoints.json")

	err := mgr.Load()
	if err != nil {
		t.Errorf("Load should succeed silently for nonexistent file: %v", err)
	}
}

func TestBreakpointManager_IncrementHitCount(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp, _ := mgr.AddLineBreakpoint("/path/to/file.go", 42)

	mgr.IncrementHitCount(bp.ID)
	if bp.HitCount != 1 {
		t.Errorf("expected hit count 1, got %d", bp.HitCount)
	}

	mgr.IncrementHitCount(bp.ID)
	mgr.IncrementHitCount(bp.ID)
	if bp.HitCount != 3 {
		t.Errorf("expected hit count 3, got %d", bp.HitCount)
	}
}

func TestBreakpointManager_GetPathsWithBreakpoints(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	mgr.AddLineBreakpoint("/path/to/file1.go", 10)
	mgr.AddLineBreakpoint("/path/to/file1.go", 20)
	mgr.AddLineBreakpoint("/path/to/file2.go", 30)

	paths := mgr.GetPathsWithBreakpoints()
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
}

func TestBreakpointTypes(t *testing.T) {
	tests := []struct {
		bpType   BreakpointType
		expected string
	}{
		{BreakpointTypeLine, "line"},
		{BreakpointTypeConditional, "conditional"},
		{BreakpointTypeLogPoint, "logpoint"},
		{BreakpointTypeFunction, "function"},
		{BreakpointTypeData, "data"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.bpType.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.bpType.String())
			}
		})
	}
}

func TestBreakpoint_JSONSerialization(t *testing.T) {
	bp := &Breakpoint{
		ID:           1,
		Type:         BreakpointTypeConditional,
		Path:         "/path/to/file.go",
		Line:         42,
		Condition:    "x > 5",
		HitCondition: ">= 3",
		Enabled:      true,
		Verified:     true,
		HitCount:     5,
	}

	data, err := json.Marshal(bp)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var bp2 Breakpoint
	err = json.Unmarshal(data, &bp2)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if bp2.ID != bp.ID {
		t.Errorf("ID mismatch: expected %d, got %d", bp.ID, bp2.ID)
	}
	if bp2.Type != bp.Type {
		t.Errorf("Type mismatch: expected %v, got %v", bp.Type, bp2.Type)
	}
	if bp2.Condition != bp.Condition {
		t.Errorf("Condition mismatch: expected %s, got %s", bp.Condition, bp2.Condition)
	}
}

func TestBreakpointManager_SyncToSession_NoSession(t *testing.T) {
	mgr := NewBreakpointManager(nil)
	mgr.AddLineBreakpoint("/path/to/file.go", 42)

	err := mgr.SyncToSession(context.Background())
	if err == nil {
		t.Error("expected error when session is nil")
	}
}

func TestBreakpointManager_HasBreakpointAt(t *testing.T) {
	mgr := NewBreakpointManager(nil)
	mgr.AddLineBreakpoint("/path/to/file.go", 42)

	if !mgr.HasBreakpointAt("/path/to/file.go", 42) {
		t.Error("expected HasBreakpointAt to return true")
	}

	if mgr.HasBreakpointAt("/path/to/file.go", 100) {
		t.Error("expected HasBreakpointAt to return false")
	}
}

func TestBreakpointManager_GetBreakpointAt(t *testing.T) {
	mgr := NewBreakpointManager(nil)
	bp, _ := mgr.AddLineBreakpoint("/path/to/file.go", 42)

	found, ok := mgr.GetBreakpointAt("/path/to/file.go", 42)
	if !ok {
		t.Error("expected GetBreakpointAt to find breakpoint")
	}
	if found.ID != bp.ID {
		t.Error("returned breakpoint ID mismatch")
	}

	_, ok = mgr.GetBreakpointAt("/path/to/file.go", 100)
	if ok {
		t.Error("expected GetBreakpointAt to return false for nonexistent")
	}
}

func TestBreakpointManager_ResetHitCounts(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp1, _ := mgr.AddLineBreakpoint("/path/to/file.go", 10)
	bp2, _ := mgr.AddLineBreakpoint("/path/to/file.go", 20)

	mgr.IncrementHitCount(bp1.ID)
	mgr.IncrementHitCount(bp1.ID)
	mgr.IncrementHitCount(bp2.ID)

	mgr.ResetHitCounts()

	if bp1.HitCount != 0 {
		t.Error("bp1 hit count should be 0")
	}
	if bp2.HitCount != 0 {
		t.Error("bp2 hit count should be 0")
	}
}

func TestBreakpointManager_GetFunctionBreakpoints(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	mgr.AddFunctionBreakpoint("func1", "")
	mgr.AddFunctionBreakpoint("func2", "x > 0")

	bps := mgr.GetFunctionBreakpoints()
	if len(bps) != 2 {
		t.Errorf("expected 2 function breakpoints, got %d", len(bps))
	}
}

func TestBreakpointManager_GetDataBreakpoints(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	mgr.AddDataBreakpoint("data1", "write", "")
	mgr.AddDataBreakpoint("data2", "readWrite", "x > 0")

	bps := mgr.GetDataBreakpoints()
	if len(bps) != 2 {
		t.Errorf("expected 2 data breakpoints, got %d", len(bps))
	}
}

func TestBreakpointManager_SetLogMessage(t *testing.T) {
	mgr := NewBreakpointManager(nil)

	bp, _ := mgr.AddLineBreakpoint("/path/to/file.go", 42)

	err := mgr.SetLogMessage(bp.ID, "Value: {x}")
	if err != nil {
		t.Fatalf("SetLogMessage failed: %v", err)
	}
	if bp.LogMessage != "Value: {x}" {
		t.Error("log message not set")
	}
	if bp.Type != BreakpointTypeLogPoint {
		t.Error("type should be LogPoint after setting log message")
	}
}
