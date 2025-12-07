package dap

import (
	"encoding/json"
	"testing"
)

func TestRequestMarshal(t *testing.T) {
	req := Request{
		ProtocolMessage: ProtocolMessage{
			Seq:  1,
			Type: "request",
		},
		Command:   "initialize",
		Arguments: json.RawMessage(`{"adapterID": "go"}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["seq"].(float64) != 1 {
		t.Errorf("expected seq 1, got %v", parsed["seq"])
	}

	if parsed["type"] != "request" {
		t.Errorf("expected type 'request', got %v", parsed["type"])
	}

	if parsed["command"] != "initialize" {
		t.Errorf("expected command 'initialize', got %v", parsed["command"])
	}
}

func TestResponseMarshal(t *testing.T) {
	resp := Response{
		ProtocolMessage: ProtocolMessage{
			Seq:  2,
			Type: "response",
		},
		RequestSeq: 1,
		Success:    true,
		Command:    "initialize",
		Body:       json.RawMessage(`{"supportsConfigurationDoneRequest": true}`),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["request_seq"].(float64) != 1 {
		t.Errorf("expected request_seq 1, got %v", parsed["request_seq"])
	}

	if parsed["success"] != true {
		t.Errorf("expected success true, got %v", parsed["success"])
	}
}

func TestEventMarshal(t *testing.T) {
	evt := Event{
		ProtocolMessage: ProtocolMessage{
			Seq:  3,
			Type: "event",
		},
		Event: "stopped",
		Body:  json.RawMessage(`{"reason": "breakpoint", "threadId": 1}`),
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["event"] != "stopped" {
		t.Errorf("expected event 'stopped', got %v", parsed["event"])
	}
}

func TestCapabilitiesUnmarshal(t *testing.T) {
	data := `{
		"supportsConfigurationDoneRequest": true,
		"supportsFunctionBreakpoints": true,
		"supportsConditionalBreakpoints": true,
		"supportsEvaluateForHovers": true,
		"supportsStepBack": false
	}`

	var caps Capabilities
	if err := json.Unmarshal([]byte(data), &caps); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !caps.SupportsConfigurationDoneRequest {
		t.Error("expected SupportsConfigurationDoneRequest true")
	}

	if !caps.SupportsFunctionBreakpoints {
		t.Error("expected SupportsFunctionBreakpoints true")
	}

	if !caps.SupportsConditionalBreakpoints {
		t.Error("expected SupportsConditionalBreakpoints true")
	}

	if !caps.SupportsEvaluateForHovers {
		t.Error("expected SupportsEvaluateForHovers true")
	}

	if caps.SupportsStepBack {
		t.Error("expected SupportsStepBack false")
	}
}

func TestStoppedEventBodyUnmarshal(t *testing.T) {
	data := `{
		"reason": "breakpoint",
		"threadId": 1,
		"allThreadsStopped": true,
		"hitBreakpointIds": [1, 2]
	}`

	var body StoppedEventBody
	if err := json.Unmarshal([]byte(data), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if body.Reason != "breakpoint" {
		t.Errorf("expected reason 'breakpoint', got %s", body.Reason)
	}

	if body.ThreadID != 1 {
		t.Errorf("expected threadId 1, got %d", body.ThreadID)
	}

	if !body.AllThreadsStopped {
		t.Error("expected allThreadsStopped true")
	}

	if len(body.HitBreakpointIds) != 2 {
		t.Errorf("expected 2 hitBreakpointIds, got %d", len(body.HitBreakpointIds))
	}
}

func TestStackFrameUnmarshal(t *testing.T) {
	data := `{
		"id": 1000,
		"name": "main.main",
		"source": {
			"name": "main.go",
			"path": "/home/user/project/main.go"
		},
		"line": 42,
		"column": 1
	}`

	var frame StackFrame
	if err := json.Unmarshal([]byte(data), &frame); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if frame.ID != 1000 {
		t.Errorf("expected id 1000, got %d", frame.ID)
	}

	if frame.Name != "main.main" {
		t.Errorf("expected name 'main.main', got %s", frame.Name)
	}

	if frame.Source == nil {
		t.Fatal("expected source to be non-nil")
	}

	if frame.Source.Path != "/home/user/project/main.go" {
		t.Errorf("unexpected source path: %s", frame.Source.Path)
	}

	if frame.Line != 42 {
		t.Errorf("expected line 42, got %d", frame.Line)
	}
}

func TestVariableUnmarshal(t *testing.T) {
	data := `{
		"name": "x",
		"value": "42",
		"type": "int",
		"variablesReference": 0
	}`

	var v Variable
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if v.Name != "x" {
		t.Errorf("expected name 'x', got %s", v.Name)
	}

	if v.Value != "42" {
		t.Errorf("expected value '42', got %s", v.Value)
	}

	if v.Type != "int" {
		t.Errorf("expected type 'int', got %s", v.Type)
	}

	if v.VariablesReference != 0 {
		t.Errorf("expected variablesReference 0, got %d", v.VariablesReference)
	}
}

func TestBreakpointMarshal(t *testing.T) {
	bp := Breakpoint{
		ID:       1,
		Verified: true,
		Line:     10,
		Source: &Source{
			Path: "/path/to/file.go",
		},
	}

	data, err := json.Marshal(bp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["id"].(float64) != 1 {
		t.Errorf("expected id 1, got %v", parsed["id"])
	}

	if parsed["verified"] != true {
		t.Errorf("expected verified true, got %v", parsed["verified"])
	}

	if parsed["line"].(float64) != 10 {
		t.Errorf("expected line 10, got %v", parsed["line"])
	}
}

func TestSourceBreakpointMarshal(t *testing.T) {
	bp := SourceBreakpoint{
		Line:         25,
		Column:       5,
		Condition:    "x > 10",
		HitCondition: "3",
		LogMessage:   "x = {x}",
	}

	data, err := json.Marshal(bp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["line"].(float64) != 25 {
		t.Errorf("expected line 25, got %v", parsed["line"])
	}

	if parsed["condition"] != "x > 10" {
		t.Errorf("expected condition 'x > 10', got %v", parsed["condition"])
	}

	if parsed["logMessage"] != "x = {x}" {
		t.Errorf("expected logMessage 'x = {x}', got %v", parsed["logMessage"])
	}
}

func TestOutputEventBodyUnmarshal(t *testing.T) {
	data := `{
		"category": "stdout",
		"output": "Hello, World!\n",
		"source": {
			"path": "/path/to/file.go"
		},
		"line": 10
	}`

	var body OutputEventBody
	if err := json.Unmarshal([]byte(data), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if body.Category != "stdout" {
		t.Errorf("expected category 'stdout', got %s", body.Category)
	}

	if body.Output != "Hello, World!\n" {
		t.Errorf("expected output 'Hello, World!\\n', got %s", body.Output)
	}

	if body.Source == nil {
		t.Fatal("expected source to be non-nil")
	}

	if body.Line != 10 {
		t.Errorf("expected line 10, got %d", body.Line)
	}
}

func TestInitializeRequestArgumentsMarshal(t *testing.T) {
	args := InitializeRequestArguments{
		ClientID:        "vscode",
		ClientName:      "Visual Studio Code",
		AdapterID:       "go",
		LinesStartAt1:   true,
		ColumnsStartAt1: true,
		PathFormat:      "path",
	}

	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["clientID"] != "vscode" {
		t.Errorf("expected clientID 'vscode', got %v", parsed["clientID"])
	}

	if parsed["adapterID"] != "go" {
		t.Errorf("expected adapterID 'go', got %v", parsed["adapterID"])
	}

	if parsed["linesStartAt1"] != true {
		t.Errorf("expected linesStartAt1 true, got %v", parsed["linesStartAt1"])
	}

	if parsed["pathFormat"] != "path" {
		t.Errorf("expected pathFormat 'path', got %v", parsed["pathFormat"])
	}
}
