package input

import (
	"testing"

	"github.com/dshills/keystorm/internal/input/key"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext()

	if ctx == nil {
		t.Fatal("NewContext returned nil")
	}

	if ctx.Mode != "normal" {
		t.Errorf("expected default mode 'normal', got %q", ctx.Mode)
	}

	if ctx.Conditions == nil {
		t.Error("expected Conditions map to be initialized")
	}

	if ctx.Variables == nil {
		t.Error("expected Variables map to be initialized")
	}
}

func TestContextClone(t *testing.T) {
	ctx := NewContext()
	ctx.Mode = "insert"
	ctx.FileType = "go"
	ctx.FilePath = "/path/to/file.go"
	ctx.HasSelection = true
	ctx.IsModified = true
	ctx.IsReadOnly = false
	ctx.LineNumber = 10
	ctx.ColumnNumber = 5
	ctx.PendingOperator = "delete"
	ctx.PendingCount = 3
	ctx.PendingRegister = 'a'
	ctx.SetCondition("editorTextFocus", true)
	ctx.SetVariable("resourceLangId", "go")
	ctx.AppendToSequence(key.NewRuneEvent('d', key.ModNone))

	clone := ctx.Clone()

	// Verify all fields are copied
	if clone.Mode != ctx.Mode {
		t.Errorf("expected mode %q, got %q", ctx.Mode, clone.Mode)
	}
	if clone.FileType != ctx.FileType {
		t.Errorf("expected file type %q, got %q", ctx.FileType, clone.FileType)
	}
	if clone.FilePath != ctx.FilePath {
		t.Errorf("expected file path %q, got %q", ctx.FilePath, clone.FilePath)
	}
	if clone.HasSelection != ctx.HasSelection {
		t.Errorf("expected HasSelection %v, got %v", ctx.HasSelection, clone.HasSelection)
	}
	if clone.IsModified != ctx.IsModified {
		t.Errorf("expected IsModified %v, got %v", ctx.IsModified, clone.IsModified)
	}
	if clone.IsReadOnly != ctx.IsReadOnly {
		t.Errorf("expected IsReadOnly %v, got %v", ctx.IsReadOnly, clone.IsReadOnly)
	}
	if clone.LineNumber != ctx.LineNumber {
		t.Errorf("expected line %d, got %d", ctx.LineNumber, clone.LineNumber)
	}
	if clone.ColumnNumber != ctx.ColumnNumber {
		t.Errorf("expected column %d, got %d", ctx.ColumnNumber, clone.ColumnNumber)
	}
	if clone.PendingOperator != ctx.PendingOperator {
		t.Errorf("expected pending operator %q, got %q", ctx.PendingOperator, clone.PendingOperator)
	}
	if clone.PendingCount != ctx.PendingCount {
		t.Errorf("expected pending count %d, got %d", ctx.PendingCount, clone.PendingCount)
	}
	if clone.PendingRegister != ctx.PendingRegister {
		t.Errorf("expected pending register %c, got %c", ctx.PendingRegister, clone.PendingRegister)
	}

	// Verify conditions are deeply copied
	if !clone.GetCondition("editorTextFocus") {
		t.Error("expected editorTextFocus condition to be copied")
	}
	ctx.SetCondition("editorTextFocus", false)
	if !clone.GetCondition("editorTextFocus") {
		t.Error("modifying original should not affect clone")
	}

	// Verify variables are deeply copied
	if clone.GetVariable("resourceLangId") != "go" {
		t.Error("expected resourceLangId variable to be copied")
	}
	ctx.SetVariable("resourceLangId", "python")
	if clone.GetVariable("resourceLangId") != "go" {
		t.Error("modifying original should not affect clone")
	}

	// Verify sequence is deeply copied
	if clone.PendingSequence == nil {
		t.Error("expected PendingSequence to be copied")
	}
	if clone.PendingSequence.Len() != 1 {
		t.Errorf("expected sequence length 1, got %d", clone.PendingSequence.Len())
	}
}

func TestContextConditions(t *testing.T) {
	ctx := NewContext()

	// Get non-existent condition
	if ctx.GetCondition("nonexistent") {
		t.Error("expected false for non-existent condition")
	}

	// Set and get condition
	ctx.SetCondition("editorTextFocus", true)
	if !ctx.GetCondition("editorTextFocus") {
		t.Error("expected editorTextFocus to be true")
	}

	// Override condition
	ctx.SetCondition("editorTextFocus", false)
	if ctx.GetCondition("editorTextFocus") {
		t.Error("expected editorTextFocus to be false after override")
	}
}

func TestContextVariables(t *testing.T) {
	ctx := NewContext()

	// Get non-existent variable
	if ctx.GetVariable("nonexistent") != "" {
		t.Error("expected empty string for non-existent variable")
	}

	// Set and get variable
	ctx.SetVariable("resourceLangId", "go")
	if ctx.GetVariable("resourceLangId") != "go" {
		t.Errorf("expected 'go', got %q", ctx.GetVariable("resourceLangId"))
	}

	// Override variable
	ctx.SetVariable("resourceLangId", "python")
	if ctx.GetVariable("resourceLangId") != "python" {
		t.Errorf("expected 'python', got %q", ctx.GetVariable("resourceLangId"))
	}
}

func TestContextClearPending(t *testing.T) {
	ctx := NewContext()
	ctx.PendingOperator = "delete"
	ctx.PendingCount = 5
	ctx.PendingRegister = 'a'
	ctx.AppendToSequence(key.NewRuneEvent('d', key.ModNone))

	ctx.ClearPending()

	if ctx.PendingOperator != "" {
		t.Errorf("expected empty pending operator, got %q", ctx.PendingOperator)
	}
	if ctx.PendingCount != 0 {
		t.Errorf("expected pending count 0, got %d", ctx.PendingCount)
	}
	if ctx.PendingRegister != 0 {
		t.Errorf("expected pending register 0, got %c", ctx.PendingRegister)
	}
	if ctx.PendingSequence != nil {
		t.Error("expected nil pending sequence")
	}
}

func TestContextHasPendingOperator(t *testing.T) {
	ctx := NewContext()

	if ctx.HasPendingOperator() {
		t.Error("expected no pending operator initially")
	}

	ctx.PendingOperator = "delete"
	if !ctx.HasPendingOperator() {
		t.Error("expected pending operator after setting")
	}
}

func TestContextHasPendingCount(t *testing.T) {
	ctx := NewContext()

	if ctx.HasPendingCount() {
		t.Error("expected no pending count initially")
	}

	ctx.PendingCount = 5
	if !ctx.HasPendingCount() {
		t.Error("expected pending count after setting")
	}

	ctx.PendingCount = 0
	if ctx.HasPendingCount() {
		t.Error("expected no pending count when set to 0")
	}

	ctx.PendingCount = -1
	if ctx.HasPendingCount() {
		t.Error("expected no pending count when negative")
	}
}

func TestContextGetCount(t *testing.T) {
	ctx := NewContext()

	// Default is 1
	if ctx.GetCount() != 1 {
		t.Errorf("expected default count 1, got %d", ctx.GetCount())
	}

	// Zero returns 1
	ctx.PendingCount = 0
	if ctx.GetCount() != 1 {
		t.Errorf("expected count 1 for zero, got %d", ctx.GetCount())
	}

	// Negative returns 1
	ctx.PendingCount = -5
	if ctx.GetCount() != 1 {
		t.Errorf("expected count 1 for negative, got %d", ctx.GetCount())
	}

	// Positive returns actual value
	ctx.PendingCount = 5
	if ctx.GetCount() != 5 {
		t.Errorf("expected count 5, got %d", ctx.GetCount())
	}
}

func TestContextAccumulateCount(t *testing.T) {
	ctx := NewContext()

	// Accumulate digits
	ctx.AccumulateCount(1)
	if ctx.PendingCount != 1 {
		t.Errorf("expected count 1, got %d", ctx.PendingCount)
	}

	ctx.AccumulateCount(2)
	if ctx.PendingCount != 12 {
		t.Errorf("expected count 12, got %d", ctx.PendingCount)
	}

	ctx.AccumulateCount(3)
	if ctx.PendingCount != 123 {
		t.Errorf("expected count 123, got %d", ctx.PendingCount)
	}

	// Invalid digits are ignored
	ctx.AccumulateCount(-1)
	if ctx.PendingCount != 123 {
		t.Errorf("expected count 123 after invalid digit, got %d", ctx.PendingCount)
	}

	ctx.AccumulateCount(10)
	if ctx.PendingCount != 123 {
		t.Errorf("expected count 123 after invalid digit, got %d", ctx.PendingCount)
	}
}

func TestContextAppendToSequence(t *testing.T) {
	ctx := NewContext()

	if ctx.PendingSequence != nil {
		t.Error("expected nil sequence initially")
	}

	// First append creates the sequence
	ctx.AppendToSequence(key.NewRuneEvent('d', key.ModNone))
	if ctx.PendingSequence == nil {
		t.Fatal("expected sequence to be created")
	}
	if ctx.PendingSequence.Len() != 1 {
		t.Errorf("expected sequence length 1, got %d", ctx.PendingSequence.Len())
	}

	// Second append adds to existing sequence
	ctx.AppendToSequence(key.NewRuneEvent('w', key.ModNone))
	if ctx.PendingSequence.Len() != 2 {
		t.Errorf("expected sequence length 2, got %d", ctx.PendingSequence.Len())
	}
}

func TestContextClearSequence(t *testing.T) {
	ctx := NewContext()
	ctx.AppendToSequence(key.NewRuneEvent('d', key.ModNone))

	ctx.ClearSequence()

	if ctx.PendingSequence != nil {
		t.Error("expected nil sequence after clear")
	}
}

func TestContextUpdateFromEditor(t *testing.T) {
	ctx := NewContext()

	editor := &mockEditorState{
		mode:         "insert",
		fileType:     "go",
		filePath:     "/path/to/file.go",
		hasSelection: true,
		isModified:   true,
		isReadOnly:   true,
		line:         10,
		col:          5,
	}

	ctx.UpdateFromEditor(editor)

	if ctx.Mode != "insert" {
		t.Errorf("expected mode 'insert', got %q", ctx.Mode)
	}
	if ctx.FileType != "go" {
		t.Errorf("expected file type 'go', got %q", ctx.FileType)
	}
	if ctx.FilePath != "/path/to/file.go" {
		t.Errorf("expected file path '/path/to/file.go', got %q", ctx.FilePath)
	}
	if !ctx.HasSelection {
		t.Error("expected HasSelection to be true")
	}
	if !ctx.IsModified {
		t.Error("expected IsModified to be true")
	}
	if !ctx.IsReadOnly {
		t.Error("expected IsReadOnly to be true")
	}
	if ctx.LineNumber != 10 {
		t.Errorf("expected line 10, got %d", ctx.LineNumber)
	}
	if ctx.ColumnNumber != 5 {
		t.Errorf("expected column 5, got %d", ctx.ColumnNumber)
	}

	// Check standard conditions
	if !ctx.GetCondition("editorTextFocus") {
		t.Error("expected editorTextFocus condition")
	}
	if !ctx.GetCondition("editorReadonly") {
		t.Error("expected editorReadonly condition")
	}
	if !ctx.GetCondition("editorHasSelection") {
		t.Error("expected editorHasSelection condition")
	}

	// Check standard variables
	if ctx.GetVariable("resourceLangId") != "go" {
		t.Errorf("expected resourceLangId 'go', got %q", ctx.GetVariable("resourceLangId"))
	}
}

func TestContextUpdateFromEditorNil(t *testing.T) {
	ctx := NewContext()
	ctx.Mode = "insert"

	// Should not panic and should not modify context
	ctx.UpdateFromEditor(nil)

	if ctx.Mode != "insert" {
		t.Errorf("expected mode 'insert' unchanged, got %q", ctx.Mode)
	}
}

func TestContextConditionsWithNilMap(t *testing.T) {
	ctx := &Context{} // Create without initialization

	// GetCondition should handle nil map
	if ctx.GetCondition("test") {
		t.Error("expected false for nil map")
	}

	// SetCondition should initialize map
	ctx.SetCondition("test", true)
	if ctx.Conditions == nil {
		t.Error("expected Conditions to be initialized")
	}
	if !ctx.GetCondition("test") {
		t.Error("expected test condition to be true")
	}
}

func TestContextVariablesWithNilMap(t *testing.T) {
	ctx := &Context{} // Create without initialization

	// GetVariable should handle nil map
	if ctx.GetVariable("test") != "" {
		t.Error("expected empty string for nil map")
	}

	// SetVariable should initialize map
	ctx.SetVariable("test", "value")
	if ctx.Variables == nil {
		t.Error("expected Variables to be initialized")
	}
	if ctx.GetVariable("test") != "value" {
		t.Errorf("expected 'value', got %q", ctx.GetVariable("test"))
	}
}
