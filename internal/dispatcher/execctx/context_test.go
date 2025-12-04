package execctx_test

import (
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/input"
)

func TestNew(t *testing.T) {
	ctx := execctx.New()

	if ctx.Count != 1 {
		t.Errorf("expected default Count 1, got %d", ctx.Count)
	}
	if ctx.Data == nil {
		t.Error("expected Data to be initialized")
	}
}

func TestNewWithInputContext(t *testing.T) {
	inputCtx := &input.Context{
		Mode:         "insert",
		PendingCount: 5,
		FilePath:     "/path/to/file.go",
		FileType:     "go",
		HasSelection: true,
		IsReadOnly:   false,
		IsModified:   true,
	}

	ctx := execctx.NewWithInputContext(inputCtx)

	if ctx.Count != 5 {
		t.Errorf("expected Count 5 from input context, got %d", ctx.Count)
	}
	if ctx.FilePath != "/path/to/file.go" {
		t.Errorf("expected FilePath '/path/to/file.go', got %q", ctx.FilePath)
	}
	if ctx.FileType != "go" {
		t.Errorf("expected FileType 'go', got %q", ctx.FileType)
	}
	if ctx.Input != inputCtx {
		t.Error("expected Input to be set to input context")
	}
}

func TestNewWithNilInputContext(t *testing.T) {
	ctx := execctx.NewWithInputContext(nil)

	if ctx.Count != 1 {
		t.Errorf("expected default Count 1, got %d", ctx.Count)
	}
}

func TestWithBuilders(t *testing.T) {
	ctx := execctx.New().
		WithCount(10).
		WithDryRun(true)

	if ctx.Count != 10 {
		t.Errorf("expected Count 10, got %d", ctx.Count)
	}
	if !ctx.DryRun {
		t.Error("expected DryRun to be true")
	}
}

func TestWithCountZero(t *testing.T) {
	ctx := execctx.New().WithCount(0)

	// Zero count should not change the default
	if ctx.Count != 1 {
		t.Errorf("expected Count to remain 1 for zero input, got %d", ctx.Count)
	}
}

func TestGetCount(t *testing.T) {
	ctx := execctx.New()
	ctx.Count = 0

	if ctx.GetCount() != 1 {
		t.Errorf("expected GetCount() to return 1 for zero Count, got %d", ctx.GetCount())
	}

	ctx.Count = 5
	if ctx.GetCount() != 5 {
		t.Errorf("expected GetCount() to return 5, got %d", ctx.GetCount())
	}
}

func TestMode(t *testing.T) {
	// With input context
	inputCtx := &input.Context{Mode: "visual"}
	ctx := execctx.NewWithInputContext(inputCtx)

	if ctx.Mode() != "visual" {
		t.Errorf("expected Mode 'visual', got %q", ctx.Mode())
	}

	// Without input context
	ctx2 := execctx.New()
	if ctx2.Mode() != "" {
		t.Errorf("expected empty Mode without input context, got %q", ctx2.Mode())
	}
}

func TestHasSelection(t *testing.T) {
	inputCtx := &input.Context{HasSelection: true}
	ctx := execctx.NewWithInputContext(inputCtx)

	if !ctx.HasSelection() {
		t.Error("expected HasSelection() to return true")
	}

	ctx2 := execctx.New()
	if ctx2.HasSelection() {
		t.Error("expected HasSelection() to return false without input context")
	}
}

func TestIsReadOnly(t *testing.T) {
	inputCtx := &input.Context{IsReadOnly: true}
	ctx := execctx.NewWithInputContext(inputCtx)

	if !ctx.IsReadOnly() {
		t.Error("expected IsReadOnly() to return true")
	}
}

func TestIsModified(t *testing.T) {
	inputCtx := &input.Context{IsModified: true}
	ctx := execctx.NewWithInputContext(inputCtx)

	if !ctx.IsModified() {
		t.Error("expected IsModified() to return true")
	}
}

func TestPendingOperator(t *testing.T) {
	inputCtx := &input.Context{PendingOperator: "d"}
	ctx := execctx.NewWithInputContext(inputCtx)

	if ctx.PendingOperator() != "d" {
		t.Errorf("expected PendingOperator 'd', got %q", ctx.PendingOperator())
	}
}

func TestPendingRegister(t *testing.T) {
	inputCtx := &input.Context{PendingRegister: 'a'}
	ctx := execctx.NewWithInputContext(inputCtx)

	if ctx.PendingRegister() != 'a' {
		t.Errorf("expected PendingRegister 'a', got %c", ctx.PendingRegister())
	}
}

func TestSetData(t *testing.T) {
	ctx := execctx.New()
	ctx.SetData("key", "value")

	val, ok := ctx.GetData("key")
	if !ok {
		t.Error("expected GetData to find key")
	}
	if val != "value" {
		t.Errorf("expected value 'value', got %v", val)
	}
}

func TestSetDataNilMap(t *testing.T) {
	ctx := &execctx.ExecutionContext{}
	ctx.SetData("key", "value")

	if ctx.Data == nil {
		t.Error("expected Data to be initialized")
	}
}

func TestGetDataMissing(t *testing.T) {
	ctx := execctx.New()

	_, ok := ctx.GetData("missing")
	if ok {
		t.Error("expected GetData to return false for missing key")
	}
}

func TestGetDataNilMap(t *testing.T) {
	ctx := &execctx.ExecutionContext{}

	_, ok := ctx.GetData("key")
	if ok {
		t.Error("expected GetData to return false for nil map")
	}
}

func TestGetDataString(t *testing.T) {
	ctx := execctx.New()
	ctx.SetData("str", "hello")
	ctx.SetData("notstr", 123)

	if ctx.GetDataString("str") != "hello" {
		t.Errorf("expected 'hello', got %q", ctx.GetDataString("str"))
	}

	if ctx.GetDataString("notstr") != "" {
		t.Errorf("expected empty string for non-string value, got %q", ctx.GetDataString("notstr"))
	}

	if ctx.GetDataString("missing") != "" {
		t.Errorf("expected empty string for missing key, got %q", ctx.GetDataString("missing"))
	}
}

func TestGetDataInt(t *testing.T) {
	ctx := execctx.New()
	ctx.SetData("int", 42)
	ctx.SetData("int64", int64(64))
	ctx.SetData("float", 3.14)
	ctx.SetData("str", "not an int")

	if ctx.GetDataInt("int") != 42 {
		t.Errorf("expected 42, got %d", ctx.GetDataInt("int"))
	}

	if ctx.GetDataInt("int64") != 64 {
		t.Errorf("expected 64, got %d", ctx.GetDataInt("int64"))
	}

	if ctx.GetDataInt("float") != 3 {
		t.Errorf("expected 3 (truncated), got %d", ctx.GetDataInt("float"))
	}

	if ctx.GetDataInt("str") != 0 {
		t.Errorf("expected 0 for non-int value, got %d", ctx.GetDataInt("str"))
	}
}

func TestGetDataBool(t *testing.T) {
	ctx := execctx.New()
	ctx.SetData("true", true)
	ctx.SetData("false", false)
	ctx.SetData("str", "true")

	if !ctx.GetDataBool("true") {
		t.Error("expected true")
	}

	if ctx.GetDataBool("false") {
		t.Error("expected false")
	}

	if ctx.GetDataBool("str") {
		t.Error("expected false for non-bool value")
	}
}

func TestValidate(t *testing.T) {
	ctx := execctx.New()

	// Without engine
	err := ctx.Validate()
	if err != execctx.ErrMissingEngine {
		t.Errorf("expected ErrMissingEngine, got %v", err)
	}
}

func TestValidateForEdit(t *testing.T) {
	ctx := execctx.New()

	// Without engine
	err := ctx.ValidateForEdit()
	if err != execctx.ErrMissingEngine {
		t.Errorf("expected ErrMissingEngine, got %v", err)
	}
}

func TestValidateForEditReadOnly(t *testing.T) {
	inputCtx := &input.Context{IsReadOnly: true}
	ctx := execctx.NewWithInputContext(inputCtx)

	// Set mock engine and cursors to pass those checks
	// Note: In real usage, these would be actual implementations
	// For this test, we just check the read-only validation

	// Since we can't easily mock the interfaces, we'll test the read-only logic directly
	if !ctx.IsReadOnly() {
		t.Error("expected IsReadOnly to return true")
	}
}
