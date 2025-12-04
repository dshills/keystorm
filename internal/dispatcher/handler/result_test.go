package handler_test

import (
	"errors"
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/handler"
)

func TestResultStatus(t *testing.T) {
	tests := []struct {
		status   handler.ResultStatus
		expected string
	}{
		{handler.StatusOK, "ok"},
		{handler.StatusNoOp, "no-op"},
		{handler.StatusError, "error"},
		{handler.StatusAsync, "async"},
		{handler.StatusCancelled, "cancelled"},
		{handler.ResultStatus(99), "unknown"},
	}

	for _, tc := range tests {
		if tc.status.String() != tc.expected {
			t.Errorf("ResultStatus(%d).String() = %q, want %q", tc.status, tc.status.String(), tc.expected)
		}
	}
}

func TestSuccess(t *testing.T) {
	result := handler.Success()

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
	if result.Error != nil {
		t.Error("expected nil error")
	}
}

func TestSuccessWithMessage(t *testing.T) {
	result := handler.SuccessWithMessage("done")

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
	if result.Message != "done" {
		t.Errorf("expected message 'done', got %q", result.Message)
	}
}

func TestSuccessWithData(t *testing.T) {
	result := handler.SuccessWithData("key", "value")

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}
	if result.Data == nil {
		t.Fatal("expected Data to be set")
	}
	if result.Data["key"] != "value" {
		t.Errorf("expected Data['key'] = 'value', got %v", result.Data["key"])
	}
}

func TestNoOp(t *testing.T) {
	result := handler.NoOp()

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp, got %v", result.Status)
	}
}

func TestNoOpWithMessage(t *testing.T) {
	result := handler.NoOpWithMessage("nothing to do")

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp, got %v", result.Status)
	}
	if result.Message != "nothing to do" {
		t.Errorf("expected message 'nothing to do', got %q", result.Message)
	}
}

func TestErrorResult(t *testing.T) {
	err := errors.New("test error")
	result := handler.Error(err)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError, got %v", result.Status)
	}
	if result.Error != err {
		t.Errorf("expected error %v, got %v", err, result.Error)
	}
}

func TestErrorfResult(t *testing.T) {
	result := handler.Errorf("error: %d", 42)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError, got %v", result.Status)
	}
	if result.Error == nil {
		t.Error("expected non-nil error")
	}
	if result.Error.Error() != "error: 42" {
		t.Errorf("expected error 'error: 42', got %q", result.Error.Error())
	}
}

func TestAsync(t *testing.T) {
	result := handler.Async()

	if result.Status != handler.StatusAsync {
		t.Errorf("expected StatusAsync, got %v", result.Status)
	}
}

func TestAsyncWithMessage(t *testing.T) {
	result := handler.AsyncWithMessage("processing")

	if result.Status != handler.StatusAsync {
		t.Errorf("expected StatusAsync, got %v", result.Status)
	}
	if result.Message != "processing" {
		t.Errorf("expected message 'processing', got %q", result.Message)
	}
}

func TestCancelled(t *testing.T) {
	result := handler.Cancelled()

	if result.Status != handler.StatusCancelled {
		t.Errorf("expected StatusCancelled, got %v", result.Status)
	}
}

func TestCancelledWithMessage(t *testing.T) {
	result := handler.CancelledWithMessage("user cancelled")

	if result.Status != handler.StatusCancelled {
		t.Errorf("expected StatusCancelled, got %v", result.Status)
	}
	if result.Message != "user cancelled" {
		t.Errorf("expected message 'user cancelled', got %q", result.Message)
	}
}

func TestResultIsOK(t *testing.T) {
	okResult := handler.Success()
	if !okResult.IsOK() {
		t.Error("expected IsOK() to return true for Success")
	}

	errorResult := handler.Error(errors.New("error"))
	if errorResult.IsOK() {
		t.Error("expected IsOK() to return false for Error")
	}

	noOpResult := handler.NoOp()
	if noOpResult.IsOK() {
		t.Error("expected IsOK() to return false for NoOp")
	}
}

func TestResultIsError(t *testing.T) {
	okResult := handler.Success()
	if okResult.IsError() {
		t.Error("expected IsError() to return false for Success")
	}

	errorResult := handler.Error(errors.New("error"))
	if !errorResult.IsError() {
		t.Error("expected IsError() to return true for Error")
	}

	cancelledResult := handler.Cancelled()
	if cancelledResult.IsError() {
		t.Error("expected IsError() to return false for Cancelled")
	}
}

func TestResultWithMessage(t *testing.T) {
	result := handler.Success().WithMessage("done")

	if result.Message != "done" {
		t.Errorf("expected message 'done', got %q", result.Message)
	}
	if result.Status != handler.StatusOK {
		t.Error("WithMessage should not change status")
	}
}

func TestResultWithModeChange(t *testing.T) {
	result := handler.Success().WithModeChange("insert")

	if result.ModeChange != "insert" {
		t.Errorf("expected ModeChange 'insert', got %q", result.ModeChange)
	}
}

func TestResultWithViewUpdate(t *testing.T) {
	vu := handler.ViewUpdate{Redraw: true}
	result := handler.Success().WithViewUpdate(vu)

	if !result.ViewUpdate.Redraw {
		t.Error("expected ViewUpdate.Redraw to be true")
	}
}

func TestResultWithData(t *testing.T) {
	result := handler.Success().WithData("key", "value")

	if result.Data == nil {
		t.Fatal("expected Data to be initialized")
	}

	if result.Data["key"] != "value" {
		t.Errorf("expected Data['key'] = 'value', got %v", result.Data["key"])
	}
}

func TestResultWithDataMultiple(t *testing.T) {
	result := handler.Success().
		WithData("key1", "value1").
		WithData("key2", 42)

	if result.Data["key1"] != "value1" {
		t.Errorf("expected Data['key1'] = 'value1', got %v", result.Data["key1"])
	}
	if result.Data["key2"] != 42 {
		t.Errorf("expected Data['key2'] = 42, got %v", result.Data["key2"])
	}
}

func TestResultWithRedraw(t *testing.T) {
	result := handler.Success().WithRedraw()

	if !result.ViewUpdate.Redraw {
		t.Error("expected ViewUpdate.Redraw to be true")
	}
}

func TestResultWithRedrawLines(t *testing.T) {
	result := handler.Success().WithRedrawLines(1, 2, 3)

	if len(result.ViewUpdate.RedrawLines) != 3 {
		t.Errorf("expected 3 redraw lines, got %d", len(result.ViewUpdate.RedrawLines))
	}
	if result.ViewUpdate.RedrawLines[0] != 1 {
		t.Error("expected first redraw line to be 1")
	}
}

func TestResultWithScrollTo(t *testing.T) {
	result := handler.Success().WithScrollTo(10, 5, false)

	if result.ViewUpdate.ScrollTo == nil {
		t.Fatal("expected ScrollTo to be set")
	}
	if result.ViewUpdate.ScrollTo.Line != 10 {
		t.Errorf("expected Line 10, got %d", result.ViewUpdate.ScrollTo.Line)
	}
	if result.ViewUpdate.ScrollTo.Column != 5 {
		t.Errorf("expected Column 5, got %d", result.ViewUpdate.ScrollTo.Column)
	}
	if result.ViewUpdate.ScrollTo.Center {
		t.Error("expected Center to be false")
	}
}

func TestResultWithScrollToCenter(t *testing.T) {
	result := handler.Success().WithScrollTo(10, 0, true)

	if result.ViewUpdate.ScrollTo == nil {
		t.Fatal("expected ScrollTo to be set")
	}
	if !result.ViewUpdate.ScrollTo.Center {
		t.Error("expected Center to be true")
	}
}

func TestResultWithCenterLine(t *testing.T) {
	result := handler.Success().WithCenterLine(50)

	if result.ViewUpdate.CenterLine == nil {
		t.Fatal("expected CenterLine to be set")
	}
	if *result.ViewUpdate.CenterLine != 50 {
		t.Errorf("expected CenterLine 50, got %d", *result.ViewUpdate.CenterLine)
	}
}

func TestResultWithEdit(t *testing.T) {
	edit := handler.Edit{NewText: "hello"}
	result := handler.Success().WithEdit(edit)

	if len(result.Edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(result.Edits))
	}
	if result.Edits[0].NewText != "hello" {
		t.Errorf("expected edit text 'hello', got %q", result.Edits[0].NewText)
	}
}

func TestResultWithEdits(t *testing.T) {
	edits := []handler.Edit{
		{NewText: "a"},
		{NewText: "b"},
	}
	result := handler.Success().WithEdits(edits)

	if len(result.Edits) != 2 {
		t.Fatalf("expected 2 edits, got %d", len(result.Edits))
	}
}

func TestResultWithCursorDelta(t *testing.T) {
	delta := handler.CursorDelta{LinesDelta: 1, ColumnDelta: 5}
	result := handler.Success().WithCursorDelta(delta)

	if result.CursorDelta.LinesDelta != 1 {
		t.Errorf("expected LinesDelta 1, got %d", result.CursorDelta.LinesDelta)
	}
	if result.CursorDelta.ColumnDelta != 5 {
		t.Errorf("expected ColumnDelta 5, got %d", result.CursorDelta.ColumnDelta)
	}
}

func TestResultGetData(t *testing.T) {
	result := handler.Success().WithData("key", "value")

	val, ok := result.GetData("key")
	if !ok {
		t.Error("expected GetData to return true")
	}
	if val != "value" {
		t.Errorf("expected 'value', got %v", val)
	}

	_, ok = result.GetData("missing")
	if ok {
		t.Error("expected GetData to return false for missing key")
	}
}

func TestResultGetDataNilMap(t *testing.T) {
	result := handler.Success()

	_, ok := result.GetData("key")
	if ok {
		t.Error("expected GetData to return false for nil map")
	}
}

func TestResultGetDataString(t *testing.T) {
	result := handler.Success().
		WithData("str", "hello").
		WithData("num", 123)

	if result.GetDataString("str") != "hello" {
		t.Errorf("expected 'hello', got %q", result.GetDataString("str"))
	}

	if result.GetDataString("num") != "" {
		t.Errorf("expected empty string for non-string, got %q", result.GetDataString("num"))
	}

	if result.GetDataString("missing") != "" {
		t.Errorf("expected empty string for missing, got %q", result.GetDataString("missing"))
	}
}

func TestResultGetDataInt(t *testing.T) {
	result := handler.Success().
		WithData("int", 42).
		WithData("int64", int64(64)).
		WithData("float", 3.14).
		WithData("str", "not a number")

	if result.GetDataInt("int") != 42 {
		t.Errorf("expected 42, got %d", result.GetDataInt("int"))
	}

	if result.GetDataInt("int64") != 64 {
		t.Errorf("expected 64, got %d", result.GetDataInt("int64"))
	}

	if result.GetDataInt("float") != 3 {
		t.Errorf("expected 3 (truncated), got %d", result.GetDataInt("float"))
	}

	if result.GetDataInt("str") != 0 {
		t.Errorf("expected 0 for non-int, got %d", result.GetDataInt("str"))
	}
}

func TestResultGetDataBool(t *testing.T) {
	result := handler.Success().
		WithData("true", true).
		WithData("false", false).
		WithData("str", "true")

	if !result.GetDataBool("true") {
		t.Error("expected true")
	}

	if result.GetDataBool("false") {
		t.Error("expected false")
	}

	if result.GetDataBool("str") {
		t.Error("expected false for non-bool")
	}

	if result.GetDataBool("missing") {
		t.Error("expected false for missing")
	}
}
