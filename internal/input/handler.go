package input

import (
	"sync"
	"time"

	"github.com/dshills/keystorm/internal/input/key"
	"github.com/dshills/keystorm/internal/input/keymap"
	"github.com/dshills/keystorm/internal/input/mode"
)

// Config configures the input handler.
type Config struct {
	// DefaultMode is the initial mode (default: "normal").
	DefaultMode string

	// EnableModes enables modal editing (default: true).
	// When false, the editor operates in a modeless fashion.
	EnableModes bool

	// SequenceTimeout is how long to wait for multi-key sequences.
	// Default: 1000ms
	SequenceTimeout time.Duration

	// ShowPendingKeys shows pending keys in the status bar.
	ShowPendingKeys bool

	// EnableMouse enables mouse input handling.
	EnableMouse bool

	// DoubleClickTime is the maximum time between clicks for double-click.
	// Default: 400ms
	DoubleClickTime time.Duration

	// UseSystemClipboard uses the system clipboard for yank/paste.
	UseSystemClipboard bool
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		DefaultMode:        mode.ModeNormal,
		EnableModes:        true,
		SequenceTimeout:    1000 * time.Millisecond,
		ShowPendingKeys:    true,
		EnableMouse:        true,
		DoubleClickTime:    400 * time.Millisecond,
		UseSystemClipboard: true,
	}
}

// Handler is the main entry point for input processing.
// It coordinates key events, modes, keymaps, and action dispatch.
type Handler struct {
	mu sync.RWMutex

	// Configuration
	config Config

	// Mode manager
	modeManager *mode.Manager

	// Keymap registry
	keymapRegistry *keymap.Registry

	// Current context
	context *Context

	// Sequence timeout timer
	seqTimer *time.Timer

	// Action output channel
	actionChan chan Action

	// Hooks for input interception
	hooks []Hook

	// Closed flag
	closed bool
}

// Hook allows interception and modification of input handling.
type Hook interface {
	// PreKeyEvent is called before processing a key event.
	// Return true to consume the event (stop further processing).
	PreKeyEvent(event *key.Event, ctx *Context) bool

	// PostKeyEvent is called after processing a key event.
	PostKeyEvent(event *key.Event, action *Action, ctx *Context)

	// PreAction is called before dispatching an action.
	// Return true to consume the action.
	PreAction(action *Action, ctx *Context) bool
}

// NewHandler creates a new input handler.
func NewHandler(config Config) *Handler {
	h := &Handler{
		config:         config,
		context:        NewContext(),
		actionChan:     make(chan Action, 100),
		hooks:          make([]Hook, 0),
		modeManager:    mode.NewManager(),
		keymapRegistry: keymap.NewRegistry(),
	}

	// Register default modes
	h.registerDefaultModes()

	// Load default keymaps
	if err := keymap.LoadDefaults(h.keymapRegistry); err != nil {
		// Log error but continue - keymaps can be loaded later
	}

	// Set initial mode
	h.context.Mode = config.DefaultMode
	if err := h.modeManager.SetInitialMode(config.DefaultMode); err != nil {
		// Fall back to normal mode
		h.context.Mode = mode.ModeNormal
		_ = h.modeManager.SetInitialMode(mode.ModeNormal)
	}

	return h
}

// registerDefaultModes registers the built-in modes.
func (h *Handler) registerDefaultModes() {
	h.modeManager.Register(mode.NewNormalMode())
	h.modeManager.Register(mode.NewInsertMode())
	h.modeManager.Register(mode.NewVisualMode())
	h.modeManager.Register(mode.NewVisualLineMode())
	h.modeManager.Register(mode.NewVisualBlockMode())
	h.modeManager.Register(mode.NewCommandMode())
	h.modeManager.Register(mode.NewOperatorPendingMode())
	h.modeManager.Register(mode.NewReplaceMode())
}

// HandleKeyEvent processes a key event.
func (h *Handler) HandleKeyEvent(event key.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}

	// Run pre-hooks
	for _, hook := range h.hooks {
		if hook.PreKeyEvent(&event, h.context) {
			return // Hook consumed the event
		}
	}

	// Add to pending sequence
	h.context.AppendToSequence(event)

	// Reset sequence timeout
	h.resetSequenceTimeout()

	// Try to resolve the sequence
	action := h.resolveSequence()

	// Run post-hooks
	for _, hook := range h.hooks {
		hook.PostKeyEvent(&event, action, h.context)
	}
}

// resolveSequence attempts to resolve the pending key sequence to an action.
func (h *Handler) resolveSequence() *Action {
	if h.context.PendingSequence == nil || h.context.PendingSequence.Len() == 0 {
		return nil
	}

	currentMode := h.modeManager.Current()
	if currentMode == nil {
		h.clearSequence()
		return nil
	}

	// Build lookup context
	lookupCtx := h.buildLookupContext()

	// Check for exact binding match
	binding := h.keymapRegistry.Lookup(h.context.PendingSequence, lookupCtx)
	if binding != nil {
		action := h.buildAction(binding)
		h.clearSequence()
		h.dispatchAction(action)
		return &action
	}

	// Check if this sequence could be a prefix of a longer binding
	if h.keymapRegistry.HasPrefix(h.context.PendingSequence, lookupCtx) {
		// Wait for more keys
		return nil
	}

	// No match found - handle based on mode
	action := h.handleUnmatchedSequence(currentMode)
	h.clearSequence()
	return action
}

// buildLookupContext creates a keymap lookup context from the input context.
func (h *Handler) buildLookupContext() *keymap.LookupContext {
	ctx := keymap.NewLookupContext()
	ctx.Mode = h.context.Mode
	ctx.FileType = h.context.FileType

	// Copy conditions
	for k, v := range h.context.Conditions {
		ctx.Conditions[k] = v
	}

	// Copy variables
	for k, v := range h.context.Variables {
		ctx.Variables[k] = v
	}

	return ctx
}

// buildAction creates an action from a binding.
func (h *Handler) buildAction(binding *keymap.Binding) Action {
	action := Action{
		Name:   binding.Action,
		Source: SourceKeyboard,
		Count:  h.context.GetCount(),
	}

	// Copy binding args
	if binding.Args != nil {
		action.Args.Extra = make(map[string]interface{}, len(binding.Args))
		for k, v := range binding.Args {
			action.Args.Extra[k] = v
		}
	}

	// Apply pending register if set
	if h.context.PendingRegister != 0 {
		action.Args.Register = h.context.PendingRegister
	}

	return action
}

// handleUnmatchedSequence handles key sequences that don't match any binding.
func (h *Handler) handleUnmatchedSequence(currentMode mode.Mode) *Action {
	if h.context.PendingSequence == nil || h.context.PendingSequence.Len() == 0 {
		return nil
	}

	// Create mode context for HandleUnmapped
	modeCtx := &mode.Context{}
	if prev := h.modeManager.Previous(); prev != nil {
		modeCtx.PreviousMode = prev.Name()
	}

	// Process each event through the mode's HandleUnmapped
	for _, event := range h.context.PendingSequence.Events {
		result := currentMode.HandleUnmapped(event, modeCtx)
		if result == nil || !result.Consumed {
			continue
		}

		if result.Action != nil {
			// Convert mode action to input action
			inputAction := Action{
				Name:   result.Action.Name,
				Source: SourceKeyboard,
				Count:  h.context.GetCount(),
			}

			// Copy args from mode action
			if result.Action.Args != nil {
				inputAction.Args.Extra = make(map[string]interface{}, len(result.Action.Args))
				for k, v := range result.Action.Args {
					inputAction.Args.Extra[k] = v
				}
			}

			// Use InsertText if set
			if result.InsertText != "" {
				inputAction.Args.Text = result.InsertText
			}

			h.dispatchAction(inputAction)
			return &inputAction
		}
	}

	return nil
}

// dispatchAction sends an action to the output channel.
func (h *Handler) dispatchAction(action Action) {
	// Run pre-action hooks
	for _, hook := range h.hooks {
		if hook.PreAction(&action, h.context) {
			return // Hook consumed the action
		}
	}

	// Clear pending state after action
	h.context.PendingCount = 0
	h.context.PendingRegister = 0
	h.context.PendingOperator = ""

	// Non-blocking send with overflow protection
	select {
	case h.actionChan <- action:
	default:
		// Channel full - drop oldest and try again
		select {
		case <-h.actionChan:
		default:
		}
		select {
		case h.actionChan <- action:
		default:
		}
	}
}

// clearSequence clears the pending key sequence and stops the timer.
func (h *Handler) clearSequence() {
	h.context.ClearSequence()
	h.stopSequenceTimeout()
}

// resetSequenceTimeout resets the sequence timeout timer.
func (h *Handler) resetSequenceTimeout() {
	h.stopSequenceTimeout()

	if h.config.SequenceTimeout > 0 {
		h.seqTimer = time.AfterFunc(h.config.SequenceTimeout, func() {
			h.handleSequenceTimeout()
		})
	}
}

// stopSequenceTimeout stops the sequence timeout timer.
func (h *Handler) stopSequenceTimeout() {
	if h.seqTimer != nil {
		h.seqTimer.Stop()
		h.seqTimer = nil
	}
}

// handleSequenceTimeout is called when the sequence timeout fires.
func (h *Handler) handleSequenceTimeout() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed || h.context.PendingSequence == nil {
		return
	}

	// Try to resolve with what we have, or clear
	h.resolveSequence()

	// Clear any remaining sequence
	h.clearSequence()
}

// Actions returns the channel for receiving dispatched actions.
func (h *Handler) Actions() <-chan Action {
	return h.actionChan
}

// ModeManager returns the mode manager.
func (h *Handler) ModeManager() *mode.Manager {
	return h.modeManager
}

// KeymapRegistry returns the keymap registry.
func (h *Handler) KeymapRegistry() *keymap.Registry {
	return h.keymapRegistry
}

// Context returns a copy of the current context.
func (h *Handler) Context() *Context {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.context.Clone()
}

// CurrentMode returns the name of the current mode.
func (h *Handler) CurrentMode() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.context.Mode
}

// SwitchMode changes to a different mode.
func (h *Handler) SwitchMode(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.modeManager.Switch(name); err != nil {
		return err
	}

	h.context.Mode = name
	h.context.ClearPending()
	return nil
}

// PendingKeys returns the pending key sequence as a string.
func (h *Handler) PendingKeys() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.context.PendingSequence == nil {
		return ""
	}
	return h.context.PendingSequence.String()
}

// SetCount sets the count prefix for the next command.
func (h *Handler) SetCount(count int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.context.PendingCount = count
}

// SetRegister sets the register for the next command.
func (h *Handler) SetRegister(register rune) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.context.PendingRegister = register
}

// SetOperator sets the pending operator.
func (h *Handler) SetOperator(operator string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.context.PendingOperator = operator
}

// UpdateContext updates the context from an editor state provider.
func (h *Handler) UpdateContext(editor EditorStateProvider) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.context.UpdateFromEditor(editor)
}

// AddHook adds an input hook.
func (h *Handler) AddHook(hook Hook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hooks = append(h.hooks, hook)
}

// RemoveHook removes an input hook.
func (h *Handler) RemoveHook(hook Hook) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, hk := range h.hooks {
		if hk == hook {
			h.hooks = append(h.hooks[:i], h.hooks[i+1:]...)
			return
		}
	}
}

// Close shuts down the handler and closes the action channel.
func (h *Handler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}

	h.closed = true
	h.stopSequenceTimeout()
	close(h.actionChan)
}

// IsClosed returns true if the handler has been closed.
func (h *Handler) IsClosed() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.closed
}
