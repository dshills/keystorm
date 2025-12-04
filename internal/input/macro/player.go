package macro

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/dshills/keystorm/internal/input/key"
)

// EventHandler is a callback function that processes replayed key events.
type EventHandler func(event key.Event)

// Player replays recorded macros.
type Player struct {
	recorder *Recorder
	mu       sync.Mutex
	playing  atomic.Bool
	cancel   context.CancelFunc
}

// NewPlayer creates a new macro player that uses the given recorder for macro storage.
func NewPlayer(recorder *Recorder) *Player {
	return &Player{
		recorder: recorder,
	}
}

// Play replays a macro from the specified register.
// The count parameter specifies how many times to replay the macro (minimum 1).
// The handler is called for each key event in the macro.
// Returns an error if the register is empty or invalid.
// Playback runs synchronously - use PlayAsync for non-blocking playback.
func (p *Player) Play(register rune, count int, handler EventHandler) error {
	if !IsValidRegister(register) {
		return fmt.Errorf("invalid register: %c", register)
	}

	events := p.recorder.Get(register)
	if len(events) == 0 {
		return fmt.Errorf("empty register: %c", register)
	}

	if count < 1 {
		count = 1
	}

	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	// Set up cancellation
	ctx, cancel := context.WithCancel(context.Background())

	p.mu.Lock()
	if p.playing.Load() {
		p.mu.Unlock()
		cancel()
		return fmt.Errorf("already playing a macro")
	}
	p.cancel = cancel
	p.playing.Store(true)
	p.mu.Unlock()

	defer func() {
		cancel() // Always release context resources
		p.playing.Store(false)
		p.mu.Lock()
		p.cancel = nil
		p.mu.Unlock()
	}()

	// Play the macro count times
	for i := 0; i < count; i++ {
		for _, event := range events {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				handler(event)
			}
		}
	}

	// Track last played register only after successful playback
	p.recorder.SetLastPlayed(register)

	return nil
}

// PlayAsync replays a macro asynchronously.
// Returns immediately and plays the macro in a goroutine.
// The done channel is closed when playback completes (can be nil if not needed).
// Any error during setup is returned immediately; playback errors are ignored.
func (p *Player) PlayAsync(register rune, count int, handler EventHandler, done chan<- struct{}) error {
	if !IsValidRegister(register) {
		return fmt.Errorf("invalid register: %c", register)
	}

	events := p.recorder.Get(register)
	if len(events) == 0 {
		return fmt.Errorf("empty register: %c", register)
	}

	if count < 1 {
		count = 1
	}

	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	// Set up cancellation
	ctx, cancel := context.WithCancel(context.Background())

	p.mu.Lock()
	if p.playing.Load() {
		p.mu.Unlock()
		cancel()
		return fmt.Errorf("already playing a macro")
	}
	p.cancel = cancel
	p.playing.Store(true)
	p.mu.Unlock()

	go func() {
		defer func() {
			cancel() // Always release context resources
			p.playing.Store(false)
			p.mu.Lock()
			p.cancel = nil
			p.mu.Unlock()
			if done != nil {
				close(done)
			}
		}()

		for i := 0; i < count; i++ {
			for _, event := range events {
				select {
				case <-ctx.Done():
					return
				default:
					handler(event)
				}
			}
		}

		// Track last played register only after successful playback
		p.recorder.SetLastPlayed(register)
	}()

	return nil
}

// PlayLast replays the last played macro.
// Equivalent to @@ in Vim.
func (p *Player) PlayLast(count int, handler EventHandler) error {
	register := p.recorder.LastPlayed()
	if register == 0 {
		return fmt.Errorf("no macro has been played")
	}
	return p.Play(register, count, handler)
}

// IsPlaying returns true if a macro is currently being played.
func (p *Player) IsPlaying() bool {
	return p.playing.Load()
}

// Cancel stops the currently playing macro.
// Safe to call even if no macro is playing.
func (p *Player) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
	}
}

// PlayWithContext plays a macro with an external context for cancellation.
// This allows integration with application-level cancellation.
func (p *Player) PlayWithContext(ctx context.Context, register rune, count int, handler EventHandler) error {
	if !IsValidRegister(register) {
		return fmt.Errorf("invalid register: %c", register)
	}

	events := p.recorder.Get(register)
	if len(events) == 0 {
		return fmt.Errorf("empty register: %c", register)
	}

	if count < 1 {
		count = 1
	}

	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	// Create a child context for internal cancellation
	childCtx, cancel := context.WithCancel(ctx)

	p.mu.Lock()
	if p.playing.Load() {
		p.mu.Unlock()
		cancel()
		return fmt.Errorf("already playing a macro")
	}
	p.cancel = cancel
	p.playing.Store(true)
	p.mu.Unlock()

	defer func() {
		cancel() // Always release context resources
		p.playing.Store(false)
		p.mu.Lock()
		p.cancel = nil
		p.mu.Unlock()
	}()

	// Play the macro count times
	for i := 0; i < count; i++ {
		for _, event := range events {
			select {
			case <-childCtx.Done():
				return childCtx.Err()
			default:
				handler(event)
			}
		}
	}

	// Track last played register only after successful playback
	p.recorder.SetLastPlayed(register)

	return nil
}
