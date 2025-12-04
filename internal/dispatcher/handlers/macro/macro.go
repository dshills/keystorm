// Package macro provides handlers for macro recording and playback.
package macro

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
)

// Action names for macro operations.
const (
	ActionStartRecord = "macro.startRecord" // q{register} - start recording
	ActionStopRecord  = "macro.stopRecord"  // q - stop recording
	ActionPlay        = "macro.play"        // @{register} - play macro
	ActionPlayLast    = "macro.playLast"    // @@ - repeat last macro
	ActionEdit        = "macro.edit"        // Edit macro content
	ActionList        = "macro.list"        // List all macros
	ActionClear       = "macro.clear"       // Clear a macro
	ActionClearAll    = "macro.clearAll"    // Clear all macros
)

// RecordedAction represents a single action in a macro.
type RecordedAction struct {
	// Name is the action name.
	Name string
	// Args contains the action arguments.
	Args input.ActionArgs
	// Count is the repeat count.
	Count int
}

// Macro represents a recorded macro.
type Macro struct {
	// Actions is the sequence of recorded actions.
	Actions []RecordedAction
	// Name is the register name (a-z).
	Name rune
}

// MacroRecorder provides macro recording and playback functionality.
type MacroRecorder interface {
	// StartRecording begins recording to the specified register.
	StartRecording(register rune) error
	// StopRecording stops the current recording.
	StopRecording() (*Macro, error)
	// IsRecording returns true if currently recording.
	IsRecording() bool
	// CurrentRegister returns the register being recorded to.
	CurrentRegister() rune
	// RecordAction adds an action to the current recording.
	RecordAction(action RecordedAction) error
	// GetMacro returns the macro for a register.
	GetMacro(register rune) *Macro
	// SetMacro sets the macro for a register.
	SetMacro(register rune, macro *Macro)
	// ClearMacro clears the macro for a register.
	ClearMacro(register rune)
	// ClearAll clears all macros.
	ClearAll()
	// ListMacros returns all register names with macros.
	ListMacros() []rune
	// LastPlayedRegister returns the last played register.
	LastPlayedRegister() rune
	// SetLastPlayedRegister sets the last played register.
	SetLastPlayedRegister(register rune)
}

const macroRecorderKey = "_macro_recorder"

// Handler implements namespace-based macro handling.
type Handler struct {
	recorder MacroRecorder
	// PlayCallback is called for each action during playback.
	// If nil, actions are returned in result data for the dispatcher to execute.
	PlayCallback func(action input.Action) handler.Result
}

// NewHandler creates a new macro handler.
func NewHandler() *Handler {
	return &Handler{}
}

// NewHandlerWithRecorder creates a handler with a macro recorder.
func NewHandlerWithRecorder(r MacroRecorder) *Handler {
	return &Handler{recorder: r}
}

// Namespace returns the macro namespace.
func (h *Handler) Namespace() string {
	return "macro"
}

// CanHandle returns true if this handler can process the action.
func (h *Handler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionStartRecord, ActionStopRecord, ActionPlay, ActionPlayLast,
		ActionEdit, ActionList, ActionClear, ActionClearAll:
		return true
	}
	return false
}

// HandleAction processes a macro action.
func (h *Handler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	recorder := h.getRecorder(ctx)
	if recorder == nil {
		return handler.NoOpWithMessage("macro: no recorder")
	}

	switch action.Name {
	case ActionStartRecord:
		return h.startRecord(recorder, action)
	case ActionStopRecord:
		return h.stopRecord(recorder)
	case ActionPlay:
		return h.play(recorder, action, ctx)
	case ActionPlayLast:
		return h.playLast(recorder, ctx)
	case ActionEdit:
		return h.edit(recorder, action)
	case ActionList:
		return h.list(recorder)
	case ActionClear:
		return h.clear(recorder, action)
	case ActionClearAll:
		return h.clearAll(recorder)
	default:
		return handler.Errorf("unknown macro action: %s", action.Name)
	}
}

// getRecorder returns the macro recorder.
func (h *Handler) getRecorder(ctx *execctx.ExecutionContext) MacroRecorder {
	if h.recorder != nil {
		return h.recorder
	}
	if v, ok := ctx.GetData(macroRecorderKey); ok {
		if r, ok := v.(MacroRecorder); ok {
			return r
		}
	}
	return nil
}

// startRecord begins recording to a register.
func (h *Handler) startRecord(recorder MacroRecorder, action input.Action) handler.Result {
	if recorder.IsRecording() {
		return handler.Errorf("macro: already recording to register '%c'", recorder.CurrentRegister())
	}

	register := action.Args.Register
	if register == 0 {
		return handler.Errorf("macro: register required (q{a-z})")
	}

	if !isValidRegister(register) {
		return handler.Errorf("macro: invalid register '%c' (must be a-z)", register)
	}

	if err := recorder.StartRecording(register); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithMessage("macro: recording to @"+string(register)).
		WithData("recording", true).
		WithData("register", string(register))
}

// stopRecord stops the current recording.
func (h *Handler) stopRecord(recorder MacroRecorder) handler.Result {
	if !recorder.IsRecording() {
		return handler.NoOpWithMessage("macro: not recording")
	}

	register := recorder.CurrentRegister()
	macro, err := recorder.StopRecording()
	if err != nil {
		return handler.Error(err)
	}

	actionCount := 0
	if macro != nil {
		actionCount = len(macro.Actions)
	}

	return handler.Success().
		WithMessage("macro: recorded "+itoa(actionCount)+" actions to @"+string(register)).
		WithData("recording", false).
		WithData("register", string(register)).
		WithData("actionCount", actionCount)
}

// play plays a macro from a register.
func (h *Handler) play(recorder MacroRecorder, action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	if recorder.IsRecording() {
		return handler.Errorf("macro: cannot play while recording")
	}

	register := action.Args.Register
	if register == 0 {
		return handler.Errorf("macro: register required (@{a-z})")
	}

	if !isValidRegister(register) {
		return handler.Errorf("macro: invalid register '%c'", register)
	}

	macro := recorder.GetMacro(register)
	if macro == nil || len(macro.Actions) == 0 {
		return handler.NoOpWithMessage("macro: register @" + string(register) + " is empty")
	}

	recorder.SetLastPlayedRegister(register)

	count := ctx.GetCount()
	return h.executeMacro(macro, count)
}

// playLast plays the last played macro.
func (h *Handler) playLast(recorder MacroRecorder, ctx *execctx.ExecutionContext) handler.Result {
	if recorder.IsRecording() {
		return handler.Errorf("macro: cannot play while recording")
	}

	register := recorder.LastPlayedRegister()
	if register == 0 {
		return handler.NoOpWithMessage("macro: no macro played yet")
	}

	macro := recorder.GetMacro(register)
	if macro == nil || len(macro.Actions) == 0 {
		return handler.NoOpWithMessage("macro: register @" + string(register) + " is empty")
	}

	count := ctx.GetCount()
	return h.executeMacro(macro, count)
}

// executeMacro executes a macro the specified number of times.
func (h *Handler) executeMacro(macro *Macro, count int) handler.Result {
	if count <= 0 {
		count = 1
	}

	// Build list of actions to execute
	var actions []input.Action
	for i := 0; i < count; i++ {
		for _, recorded := range macro.Actions {
			action := input.Action{
				Name:   recorded.Name,
				Args:   recorded.Args,
				Count:  recorded.Count,
				Source: input.SourceMacro,
			}
			actions = append(actions, action)
		}
	}

	// If we have a callback, execute actions directly
	if h.PlayCallback != nil {
		for _, action := range actions {
			result := h.PlayCallback(action)
			if result.Status == handler.StatusError {
				return result
			}
		}
		return handler.Success().WithMessage("macro: executed " + itoa(len(actions)) + " actions")
	}

	// Otherwise return actions for dispatcher to execute
	return handler.Success().
		WithData("macroActions", actions).
		WithMessage("macro: queued " + itoa(len(actions)) + " actions")
}

// edit allows editing a macro's content.
func (h *Handler) edit(recorder MacroRecorder, action input.Action) handler.Result {
	register := action.Args.Register
	if register == 0 {
		return handler.Errorf("macro: register required")
	}

	if !isValidRegister(register) {
		return handler.Errorf("macro: invalid register '%c'", register)
	}

	macro := recorder.GetMacro(register)
	if macro == nil {
		return handler.NoOpWithMessage("macro: register @" + string(register) + " is empty")
	}

	// Return macro content for editing
	return handler.Success().
		WithData("macro", macro).
		WithData("register", string(register)).
		WithMessage("macro: editing @" + string(register))
}

// list returns all recorded macros.
func (h *Handler) list(recorder MacroRecorder) handler.Result {
	registers := recorder.ListMacros()

	if len(registers) == 0 {
		return handler.NoOpWithMessage("macro: no macros recorded")
	}

	var macros []string
	for _, r := range registers {
		macro := recorder.GetMacro(r)
		if macro != nil {
			macros = append(macros, "@"+string(r)+": "+itoa(len(macro.Actions))+" actions")
		}
	}

	return handler.Success().
		WithData("registers", registers).
		WithData("macros", macros).
		WithMessage("macro: " + itoa(len(registers)) + " macros recorded")
}

// clear clears a macro register.
func (h *Handler) clear(recorder MacroRecorder, action input.Action) handler.Result {
	register := action.Args.Register
	if register == 0 {
		return handler.Errorf("macro: register required")
	}

	if !isValidRegister(register) {
		return handler.Errorf("macro: invalid register '%c'", register)
	}

	recorder.ClearMacro(register)
	return handler.Success().WithMessage("macro: cleared @" + string(register))
}

// clearAll clears all macros.
func (h *Handler) clearAll(recorder MacroRecorder) handler.Result {
	recorder.ClearAll()
	return handler.Success().WithMessage("macro: cleared all macros")
}

// isValidRegister checks if a register is valid (a-z).
func isValidRegister(r rune) bool {
	return r >= 'a' && r <= 'z'
}

// itoa converts an int to string.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// DefaultMacroRecorder is a simple in-memory macro recorder.
type DefaultMacroRecorder struct {
	macros       map[rune]*Macro
	recording    bool
	current      rune
	currentMacro *Macro
	lastPlayed   rune
}

// NewDefaultMacroRecorder creates a new default macro recorder.
func NewDefaultMacroRecorder() *DefaultMacroRecorder {
	return &DefaultMacroRecorder{
		macros: make(map[rune]*Macro),
	}
}

// StartRecording begins recording to the specified register.
func (r *DefaultMacroRecorder) StartRecording(register rune) error {
	r.recording = true
	r.current = register
	r.currentMacro = &Macro{
		Name:    register,
		Actions: make([]RecordedAction, 0),
	}
	return nil
}

// StopRecording stops the current recording.
func (r *DefaultMacroRecorder) StopRecording() (*Macro, error) {
	if !r.recording {
		return nil, nil
	}

	macro := r.currentMacro
	r.macros[r.current] = macro
	r.recording = false
	r.current = 0
	r.currentMacro = nil

	return macro, nil
}

// IsRecording returns true if currently recording.
func (r *DefaultMacroRecorder) IsRecording() bool {
	return r.recording
}

// CurrentRegister returns the register being recorded to.
func (r *DefaultMacroRecorder) CurrentRegister() rune {
	return r.current
}

// RecordAction adds an action to the current recording.
func (r *DefaultMacroRecorder) RecordAction(action RecordedAction) error {
	if !r.recording || r.currentMacro == nil {
		return nil
	}
	r.currentMacro.Actions = append(r.currentMacro.Actions, action)
	return nil
}

// GetMacro returns the macro for a register.
func (r *DefaultMacroRecorder) GetMacro(register rune) *Macro {
	return r.macros[register]
}

// SetMacro sets the macro for a register.
func (r *DefaultMacroRecorder) SetMacro(register rune, macro *Macro) {
	r.macros[register] = macro
}

// ClearMacro clears the macro for a register.
func (r *DefaultMacroRecorder) ClearMacro(register rune) {
	delete(r.macros, register)
}

// ClearAll clears all macros.
func (r *DefaultMacroRecorder) ClearAll() {
	r.macros = make(map[rune]*Macro)
	r.lastPlayed = 0
}

// ListMacros returns all register names with macros.
func (r *DefaultMacroRecorder) ListMacros() []rune {
	var registers []rune
	for reg := range r.macros {
		registers = append(registers, reg)
	}
	return registers
}

// LastPlayedRegister returns the last played register.
func (r *DefaultMacroRecorder) LastPlayedRegister() rune {
	return r.lastPlayed
}

// SetLastPlayedRegister sets the last played register.
func (r *DefaultMacroRecorder) SetLastPlayedRegister(register rune) {
	r.lastPlayed = register
}
