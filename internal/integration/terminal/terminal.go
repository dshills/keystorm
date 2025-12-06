package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Terminal represents a terminal instance.
type Terminal struct {
	id   string
	name string

	pty     PTY
	cmd     *exec.Cmd
	screen  *Screen
	history *History
	parser  *Parser

	mu       sync.RWMutex
	done     chan struct{}
	exitCode atomic.Int32
	closed   atomic.Bool

	// Callbacks
	onOutput func(data []byte)
	onTitle  func(title string)
	onClose  func()

	// Shell integration
	cwd     string
	cwdLock sync.RWMutex
}

// Options configures a new terminal.
type Options struct {
	// Name is a human-readable name for the terminal.
	Name string

	// Shell is the shell executable (defaults to $SHELL or /bin/sh).
	Shell string

	// Args are additional arguments to pass to the shell.
	Args []string

	// Env are additional environment variables.
	Env []string

	// WorkDir is the working directory for the shell.
	WorkDir string

	// Cols is the number of columns (default 80).
	Cols int

	// Rows is the number of rows (default 24).
	Rows int

	// Scrollback is the number of scrollback lines (default 10000).
	Scrollback int

	// OnOutput is called when output is received.
	OnOutput func(data []byte)

	// OnTitle is called when the terminal title changes.
	OnTitle func(title string)

	// OnClose is called when the terminal closes.
	OnClose func()
}

// newTerminal creates a new terminal with the given options.
func newTerminal(opts Options) (*Terminal, error) {
	// Set defaults
	if opts.Shell == "" {
		opts.Shell = os.Getenv("SHELL")
		if opts.Shell == "" {
			opts.Shell = "/bin/sh"
		}
	}
	if opts.Cols <= 0 {
		opts.Cols = 80
	}
	if opts.Rows <= 0 {
		opts.Rows = 24
	}
	if opts.Scrollback <= 0 {
		opts.Scrollback = 10000
	}
	if opts.Name == "" {
		opts.Name = "terminal"
	}

	// Verify shell exists
	if _, err := exec.LookPath(opts.Shell); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrShellNotFound, opts.Shell)
	}

	// Create command
	args := append([]string{"-l"}, opts.Args...) // Login shell by default
	cmd := exec.Command(opts.Shell, args...)
	cmd.Dir = opts.WorkDir

	// Set environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, opts.Env...)
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	// Start PTY
	pty, err := StartPTY(cmd, uint16(opts.Cols), uint16(opts.Rows))
	if err != nil {
		return nil, fmt.Errorf("start PTY: %w", err)
	}

	// Create screen and parser
	screen := NewScreen(opts.Cols, opts.Rows)
	history := NewHistory(opts.Scrollback)
	parser := NewParser(screen)

	t := &Terminal{
		id:       uuid.New().String(),
		name:     opts.Name,
		pty:      pty,
		cmd:      cmd,
		screen:   screen,
		history:  history,
		parser:   parser,
		done:     make(chan struct{}),
		onOutput: opts.OnOutput,
		onTitle:  opts.OnTitle,
		onClose:  opts.OnClose,
		cwd:      opts.WorkDir,
	}

	t.exitCode.Store(-1)

	// Set up parser callbacks
	parser.SetTitleCallback(func(title string) {
		if t.onTitle != nil {
			t.onTitle(title)
		}
	})

	parser.SetOSCCallback(func(cmd int, data string) {
		// Handle shell integration OSC sequences
		if cmd == 7 {
			// Working directory change
			t.cwdLock.Lock()
			t.cwd = data
			t.cwdLock.Unlock()
		}
	})

	// Start reading output
	go t.readLoop()

	return t, nil
}

// ID returns the terminal's unique identifier.
func (t *Terminal) ID() string {
	return t.id
}

// Name returns the terminal's display name.
func (t *Terminal) Name() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.name
}

// SetName updates the terminal's display name.
func (t *Terminal) SetName(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.name = name
}

// Write sends input to the terminal.
func (t *Terminal) Write(data []byte) (int, error) {
	if t.closed.Load() {
		return 0, ErrTerminalClosed
	}
	return t.pty.Write(data)
}

// WriteString sends a string to the terminal.
func (t *Terminal) WriteString(s string) (int, error) {
	return t.Write([]byte(s))
}

// Screen returns the current screen state.
func (t *Terminal) Screen() *Screen {
	return t.screen
}

// History returns the scrollback history.
func (t *Terminal) History() *History {
	return t.history
}

// Resize changes the terminal size.
func (t *Terminal) Resize(cols, rows int) error {
	if t.closed.Load() {
		return ErrTerminalClosed
	}

	if cols < 1 || rows < 1 {
		return ErrInvalidSize
	}

	if err := t.pty.Resize(uint16(cols), uint16(rows)); err != nil {
		return fmt.Errorf("resize PTY: %w", err)
	}

	t.screen.Resize(cols, rows)
	return nil
}

// Close terminates the terminal.
func (t *Terminal) Close() error {
	if t.closed.Swap(true) {
		return nil // Already closed
	}

	// Kill the process
	if t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}

	// Close the PTY
	t.pty.Close()

	// Wait for read loop to finish
	<-t.done

	// Call close callback
	if t.onClose != nil {
		t.onClose()
	}

	return nil
}

// Done returns a channel that is closed when the terminal exits.
func (t *Terminal) Done() <-chan struct{} {
	return t.done
}

// ExitCode returns the exit code after the terminal closes.
// Returns -1 if still running.
func (t *Terminal) ExitCode() int {
	return int(t.exitCode.Load())
}

// IsRunning returns true if the terminal is still running.
func (t *Terminal) IsRunning() bool {
	return !t.closed.Load()
}

// WorkingDirectory returns the terminal's current working directory.
func (t *Terminal) WorkingDirectory() string {
	t.cwdLock.RLock()
	defer t.cwdLock.RUnlock()
	return t.cwd
}

// PID returns the shell process ID.
func (t *Terminal) PID() int {
	if t.cmd.Process == nil {
		return -1
	}
	return t.cmd.Process.Pid
}

// readLoop reads output from the PTY and updates the screen.
func (t *Terminal) readLoop() {
	defer close(t.done)

	buf := make([]byte, 4096)
	for {
		n, err := t.pty.Read(buf)
		if n > 0 {
			data := buf[:n]

			// Parse ANSI sequences and update screen
			t.parser.Parse(data)

			// Call output callback
			if t.onOutput != nil {
				t.onOutput(data)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			// Check if terminal was closed
			if t.closed.Load() {
				break
			}
			// Other error - continue trying
			continue
		}
	}

	// Wait for process to exit
	if t.cmd.Process != nil {
		state, _ := t.cmd.Process.Wait()
		if state != nil {
			t.exitCode.Store(int32(state.ExitCode()))
		}
	}
}

// Manager manages multiple terminal instances.
type Manager struct {
	mu        sync.RWMutex
	terminals map[string]*Terminal

	// Configuration
	defaultShell string
	defaultCols  int
	defaultRows  int
	scrollback   int

	// Callbacks
	eventBus EventPublisher

	// Lifecycle
	closed atomic.Bool
}

// EventPublisher publishes terminal events.
type EventPublisher interface {
	Publish(eventType string, data map[string]any)
}

// ManagerConfig configures a terminal manager.
type ManagerConfig struct {
	// DefaultShell is the default shell (defaults to $SHELL).
	DefaultShell string

	// DefaultCols is the default terminal width.
	DefaultCols int

	// DefaultRows is the default terminal height.
	DefaultRows int

	// Scrollback is the default scrollback lines.
	Scrollback int

	// EventBus for publishing terminal events.
	EventBus EventPublisher
}

// NewManager creates a new terminal manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.DefaultShell == "" {
		cfg.DefaultShell = os.Getenv("SHELL")
		if cfg.DefaultShell == "" {
			cfg.DefaultShell = "/bin/sh"
		}
	}
	if cfg.DefaultCols <= 0 {
		cfg.DefaultCols = 80
	}
	if cfg.DefaultRows <= 0 {
		cfg.DefaultRows = 24
	}
	if cfg.Scrollback <= 0 {
		cfg.Scrollback = 10000
	}

	return &Manager{
		terminals:    make(map[string]*Terminal),
		defaultShell: cfg.DefaultShell,
		defaultCols:  cfg.DefaultCols,
		defaultRows:  cfg.DefaultRows,
		scrollback:   cfg.Scrollback,
		eventBus:     cfg.EventBus,
	}
}

// Create creates a new terminal.
func (m *Manager) Create(opts Options) (*Terminal, error) {
	if m.closed.Load() {
		return nil, ErrManagerClosed
	}

	// Apply defaults
	if opts.Shell == "" {
		opts.Shell = m.defaultShell
	}
	if opts.Cols <= 0 {
		opts.Cols = m.defaultCols
	}
	if opts.Rows <= 0 {
		opts.Rows = m.defaultRows
	}
	if opts.Scrollback <= 0 {
		opts.Scrollback = m.scrollback
	}

	// Create terminal
	term, err := newTerminal(opts)
	if err != nil {
		return nil, err
	}

	// Track terminal
	m.mu.Lock()
	m.terminals[term.id] = term
	m.mu.Unlock()

	// Set up close callback to remove from tracking
	originalOnClose := term.onClose
	term.onClose = func() {
		m.mu.Lock()
		delete(m.terminals, term.id)
		m.mu.Unlock()

		// Publish event
		m.publishEvent("terminal.closed", map[string]any{
			"id":       term.id,
			"name":     term.name,
			"exitCode": term.ExitCode(),
		})

		if originalOnClose != nil {
			originalOnClose()
		}
	}

	// Publish event
	m.publishEvent("terminal.created", map[string]any{
		"id":   term.id,
		"name": term.name,
	})

	return term, nil
}

// Get returns a terminal by ID.
func (m *Manager) Get(id string) (*Terminal, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	term, ok := m.terminals[id]
	return term, ok
}

// List returns all terminals.
func (m *Manager) List() []*Terminal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Terminal, 0, len(m.terminals))
	for _, term := range m.terminals {
		result = append(result, term)
	}
	return result
}

// Count returns the number of terminals.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.terminals)
}

// Close closes a terminal by ID.
func (m *Manager) Close(id string) error {
	term, ok := m.Get(id)
	if !ok {
		return ErrTerminalNotFound
	}
	return term.Close()
}

// CloseAll closes all terminals.
func (m *Manager) CloseAll() error {
	terminals := m.List()
	for _, term := range terminals {
		term.Close()
	}
	return nil
}

// Shutdown shuts down the manager and all terminals.
func (m *Manager) Shutdown(timeout time.Duration) {
	if m.closed.Swap(true) {
		return
	}

	// Get all terminals
	terminals := m.List()

	if len(terminals) == 0 {
		return
	}

	// Close all terminals
	for _, term := range terminals {
		term.Close()
	}

	// Wait for all terminals to close
	done := make(chan struct{})
	go func() {
		for _, term := range terminals {
			<-term.Done()
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		// Force kill any remaining
		for _, term := range terminals {
			if term.cmd.Process != nil {
				term.cmd.Process.Kill()
			}
		}
	}
}

// publishEvent publishes an event if an event bus is configured.
func (m *Manager) publishEvent(eventType string, data map[string]any) {
	if m.eventBus != nil {
		if data == nil {
			data = make(map[string]any)
		}
		data["timestamp"] = time.Now().UnixMilli()
		m.eventBus.Publish(eventType, data)
	}
}
