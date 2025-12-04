package macro

import (
	"testing"

	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

func TestHandler_Namespace(t *testing.T) {
	h := NewHandler()
	if h.Namespace() != "macro" {
		t.Errorf("expected namespace 'macro', got '%s'", h.Namespace())
	}
}

func TestHandler_CanHandle(t *testing.T) {
	h := NewHandler()

	validActions := []string{
		ActionStartRecord,
		ActionStopRecord,
		ActionPlay,
		ActionPlayLast,
		ActionEdit,
		ActionList,
		ActionClear,
		ActionClearAll,
	}

	for _, action := range validActions {
		if !h.CanHandle(action) {
			t.Errorf("expected CanHandle(%s) to return true", action)
		}
	}

	if h.CanHandle("invalid.action") {
		t.Error("expected CanHandle('invalid.action') to return false")
	}
}

func TestHandler_NoRecorder(t *testing.T) {
	h := NewHandler()
	ctx := execctx.New()

	action := input.Action{Name: ActionStartRecord}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for no recorder, got %v", result.Status)
	}
}

func TestHandler_StartRecord(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	action := input.Action{
		Name: ActionStartRecord,
		Args: input.ActionArgs{Register: 'a'},
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if !recorder.IsRecording() {
		t.Error("expected recording to be true")
	}

	if recorder.CurrentRegister() != 'a' {
		t.Errorf("expected register 'a', got '%c'", recorder.CurrentRegister())
	}
}

func TestHandler_StartRecordNoRegister(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	action := input.Action{Name: ActionStartRecord}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for no register, got %v", result.Status)
	}
}

func TestHandler_StartRecordInvalidRegister(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	action := input.Action{
		Name: ActionStartRecord,
		Args: input.ActionArgs{Register: '1'}, // Invalid
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError for invalid register, got %v", result.Status)
	}
}

func TestHandler_StartRecordAlreadyRecording(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	// Start first recording
	action := input.Action{
		Name: ActionStartRecord,
		Args: input.ActionArgs{Register: 'a'},
	}
	h.HandleAction(action, ctx)

	// Try to start another
	action2 := input.Action{
		Name: ActionStartRecord,
		Args: input.ActionArgs{Register: 'b'},
	}
	result := h.HandleAction(action2, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError when already recording, got %v", result.Status)
	}
}

func TestHandler_StopRecord(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	// Start recording
	h.HandleAction(input.Action{
		Name: ActionStartRecord,
		Args: input.ActionArgs{Register: 'a'},
	}, ctx)

	// Record an action
	recorder.RecordAction(RecordedAction{
		Name:  "cursor.moveRight",
		Count: 1,
	})

	// Stop recording
	result := h.HandleAction(input.Action{Name: ActionStopRecord}, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	if recorder.IsRecording() {
		t.Error("expected recording to be false")
	}

	// Check macro was saved
	macro := recorder.GetMacro('a')
	if macro == nil {
		t.Fatal("expected macro to be saved")
	}

	if len(macro.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(macro.Actions))
	}
}

func TestHandler_StopRecordNotRecording(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	result := h.HandleAction(input.Action{Name: ActionStopRecord}, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp when not recording, got %v", result.Status)
	}
}

func TestHandler_Play(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	// Create a macro
	macro := &Macro{
		Name: 'a',
		Actions: []RecordedAction{
			{Name: "cursor.moveRight", Count: 1},
			{Name: "cursor.moveDown", Count: 1},
		},
	}
	recorder.SetMacro('a', macro)

	// Play it
	action := input.Action{
		Name: ActionPlay,
		Args: input.ActionArgs{Register: 'a'},
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}

	actions, ok := result.GetData("macroActions")
	if !ok {
		t.Error("expected macroActions in result data")
	}

	if len(actions.([]input.Action)) != 2 {
		t.Errorf("expected 2 actions, got %d", len(actions.([]input.Action)))
	}
}

func TestHandler_PlayWithCount(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()
	ctx.Count = 3

	// Create a macro with 2 actions
	macro := &Macro{
		Name: 'a',
		Actions: []RecordedAction{
			{Name: "cursor.moveRight", Count: 1},
		},
	}
	recorder.SetMacro('a', macro)

	// Play it 3 times
	action := input.Action{
		Name: ActionPlay,
		Args: input.ActionArgs{Register: 'a'},
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	actions, _ := result.GetData("macroActions")
	if len(actions.([]input.Action)) != 3 {
		t.Errorf("expected 3 actions (1 action × 3 times), got %d", len(actions.([]input.Action)))
	}
}

func TestHandler_PlayEmpty(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	action := input.Action{
		Name: ActionPlay,
		Args: input.ActionArgs{Register: 'a'},
	}
	result := h.HandleAction(action, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for empty macro, got %v", result.Status)
	}
}

func TestHandler_PlayWhileRecording(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	// Start recording
	h.HandleAction(input.Action{
		Name: ActionStartRecord,
		Args: input.ActionArgs{Register: 'a'},
	}, ctx)

	// Try to play
	result := h.HandleAction(input.Action{
		Name: ActionPlay,
		Args: input.ActionArgs{Register: 'b'},
	}, ctx)

	if result.Status != handler.StatusError {
		t.Errorf("expected StatusError when playing while recording, got %v", result.Status)
	}
}

func TestHandler_PlayLast(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	// Create and play a macro
	macro := &Macro{
		Name:    'a',
		Actions: []RecordedAction{{Name: "test", Count: 1}},
	}
	recorder.SetMacro('a', macro)
	h.HandleAction(input.Action{
		Name: ActionPlay,
		Args: input.ActionArgs{Register: 'a'},
	}, ctx)

	// Play last
	result := h.HandleAction(input.Action{Name: ActionPlayLast}, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v: %v", result.Status, result.Error)
	}
}

func TestHandler_PlayLastNoMacro(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	result := h.HandleAction(input.Action{Name: ActionPlayLast}, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp when no macro played yet, got %v", result.Status)
	}
}

func TestHandler_List(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	// Create some macros
	recorder.SetMacro('a', &Macro{Name: 'a', Actions: []RecordedAction{{Name: "test"}}})
	recorder.SetMacro('b', &Macro{Name: 'b', Actions: []RecordedAction{{Name: "test"}, {Name: "test2"}}})

	result := h.HandleAction(input.Action{Name: ActionList}, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	registers, ok := result.GetData("registers")
	if !ok {
		t.Error("expected registers in result data")
	}

	if len(registers.([]rune)) != 2 {
		t.Errorf("expected 2 registers, got %d", len(registers.([]rune)))
	}
}

func TestHandler_ListEmpty(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	result := h.HandleAction(input.Action{Name: ActionList}, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for empty list, got %v", result.Status)
	}
}

func TestHandler_Clear(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	// Create a macro
	recorder.SetMacro('a', &Macro{Name: 'a', Actions: []RecordedAction{{Name: "test"}}})

	// Clear it
	result := h.HandleAction(input.Action{
		Name: ActionClear,
		Args: input.ActionArgs{Register: 'a'},
	}, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	if recorder.GetMacro('a') != nil {
		t.Error("expected macro to be cleared")
	}
}

func TestHandler_ClearAll(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	// Create some macros
	recorder.SetMacro('a', &Macro{Name: 'a', Actions: []RecordedAction{{Name: "test"}}})
	recorder.SetMacro('b', &Macro{Name: 'b', Actions: []RecordedAction{{Name: "test"}}})

	// Clear all
	result := h.HandleAction(input.Action{Name: ActionClearAll}, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	if len(recorder.ListMacros()) != 0 {
		t.Errorf("expected 0 macros after clear all, got %d", len(recorder.ListMacros()))
	}
}

func TestHandler_Edit(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	// Create a macro
	macro := &Macro{
		Name:    'a',
		Actions: []RecordedAction{{Name: "test"}},
	}
	recorder.SetMacro('a', macro)

	result := h.HandleAction(input.Action{
		Name: ActionEdit,
		Args: input.ActionArgs{Register: 'a'},
	}, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	returnedMacro, ok := result.GetData("macro")
	if !ok {
		t.Error("expected macro in result data")
	}

	if returnedMacro.(*Macro).Name != 'a' {
		t.Errorf("expected macro 'a', got '%c'", returnedMacro.(*Macro).Name)
	}
}

func TestHandler_EditEmpty(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)
	ctx := execctx.New()

	result := h.HandleAction(input.Action{
		Name: ActionEdit,
		Args: input.ActionArgs{Register: 'a'},
	}, ctx)

	if result.Status != handler.StatusNoOp {
		t.Errorf("expected StatusNoOp for empty macro, got %v", result.Status)
	}
}

func TestIsValidRegister(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'a', true},
		{'z', true},
		{'m', true},
		{'A', false},
		{'0', false},
		{'_', false},
		{' ', false},
	}

	for _, tt := range tests {
		got := isValidRegister(tt.r)
		if got != tt.want {
			t.Errorf("isValidRegister(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestDefaultMacroRecorder(t *testing.T) {
	r := NewDefaultMacroRecorder()

	// Initially not recording
	if r.IsRecording() {
		t.Error("expected not recording initially")
	}

	// Start recording
	r.StartRecording('a')
	if !r.IsRecording() {
		t.Error("expected recording after start")
	}
	if r.CurrentRegister() != 'a' {
		t.Errorf("expected register 'a', got '%c'", r.CurrentRegister())
	}

	// Record actions
	r.RecordAction(RecordedAction{Name: "action1"})
	r.RecordAction(RecordedAction{Name: "action2"})

	// Stop recording
	macro, _ := r.StopRecording()
	if r.IsRecording() {
		t.Error("expected not recording after stop")
	}
	if len(macro.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(macro.Actions))
	}

	// Get macro
	retrieved := r.GetMacro('a')
	if retrieved == nil {
		t.Fatal("expected macro to be saved")
	}
	if len(retrieved.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(retrieved.Actions))
	}

	// Last played
	r.SetLastPlayedRegister('a')
	if r.LastPlayedRegister() != 'a' {
		t.Errorf("expected last played 'a', got '%c'", r.LastPlayedRegister())
	}
}

func TestHandler_PlayWithCallback(t *testing.T) {
	recorder := NewDefaultMacroRecorder()
	h := NewHandlerWithRecorder(recorder)

	executed := 0
	h.PlayCallback = func(action input.Action) handler.Result {
		executed++
		return handler.Success()
	}

	ctx := execctx.New()

	// Create a macro
	macro := &Macro{
		Name: 'a',
		Actions: []RecordedAction{
			{Name: "test1"},
			{Name: "test2"},
		},
	}
	recorder.SetMacro('a', macro)

	// Play it
	result := h.HandleAction(input.Action{
		Name: ActionPlay,
		Args: input.ActionArgs{Register: 'a'},
	}, ctx)

	if result.Status != handler.StatusOK {
		t.Errorf("expected StatusOK, got %v", result.Status)
	}

	if executed != 2 {
		t.Errorf("expected 2 actions executed, got %d", executed)
	}
}
