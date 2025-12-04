package vim

import (
	"sync"
	"unicode"
)

// RegisterType categorizes registers by their behavior.
type RegisterType uint8

const (
	// RegisterNamed is a named register (a-z, A-Z).
	RegisterNamed RegisterType = iota

	// RegisterNumbered is a numbered register (0-9).
	RegisterNumbered

	// RegisterUnnamed is the default register (").
	RegisterUnnamed

	// RegisterSmallDelete is the small delete register (-).
	RegisterSmallDelete

	// RegisterBlackHole is the black hole register (_).
	RegisterBlackHole

	// RegisterLastInserted is the last inserted text register (.).
	RegisterLastInserted

	// RegisterFileName is the current file name register (%).
	RegisterFileName

	// RegisterAlternate is the alternate file name register (#).
	RegisterAlternate

	// RegisterCommand is the last command register (:).
	RegisterCommand

	// RegisterSearch is the last search pattern register (/).
	RegisterSearch

	// RegisterExpression is the expression register (=).
	RegisterExpression

	// RegisterClipboard is the system clipboard register (+).
	RegisterClipboard

	// RegisterSelection is the primary selection register (*).
	RegisterSelection

	// RegisterLastYank is the yank register (0).
	RegisterLastYank
)

// Register represents a named storage location for text.
type Register struct {
	// Name is the register character.
	Name rune

	// Type categorizes the register.
	Type RegisterType

	// Content holds the register's text content.
	Content string

	// Linewise indicates if the content is line-oriented.
	Linewise bool

	// Blockwise indicates if the content is block-oriented.
	Blockwise bool

	// ReadOnly indicates if the register is read-only.
	ReadOnly bool
}

// RegisterStore manages all registers.
type RegisterStore struct {
	mu        sync.RWMutex
	registers map[rune]*Register

	// lastYankRegister tracks the last yank for register 0.
	lastYankRegister *Register //nolint:unused // for future register 0 support

	// numberedRegisters are 1-9, rotating delete history.
	numberedRegisters [9]*Register

	// clipboard provides system clipboard access.
	clipboard ClipboardProvider
}

// ClipboardProvider abstracts system clipboard access.
type ClipboardProvider interface {
	// Get returns the current clipboard content.
	Get() (string, error)

	// Set sets the clipboard content.
	Set(content string) error
}

// NewRegisterStore creates a new register store.
func NewRegisterStore() *RegisterStore {
	rs := &RegisterStore{
		registers: make(map[rune]*Register),
	}
	rs.initializeRegisters()
	return rs
}

// SetClipboard sets the clipboard provider for system clipboard integration.
func (rs *RegisterStore) SetClipboard(clipboard ClipboardProvider) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.clipboard = clipboard
}

// initializeRegisters sets up the default registers.
func (rs *RegisterStore) initializeRegisters() {
	// Unnamed register
	rs.registers['"'] = &Register{Name: '"', Type: RegisterUnnamed}

	// Named registers (a-z)
	for r := 'a'; r <= 'z'; r++ {
		rs.registers[r] = &Register{Name: r, Type: RegisterNamed}
	}

	// Numbered registers (0-9)
	rs.registers['0'] = &Register{Name: '0', Type: RegisterLastYank}
	for i := 1; i <= 9; i++ {
		r := rune('0' + i)
		rs.registers[r] = &Register{Name: r, Type: RegisterNumbered}
		rs.numberedRegisters[i-1] = rs.registers[r]
	}

	// Special registers
	rs.registers['-'] = &Register{Name: '-', Type: RegisterSmallDelete}
	rs.registers['_'] = &Register{Name: '_', Type: RegisterBlackHole}
	rs.registers['.'] = &Register{Name: '.', Type: RegisterLastInserted, ReadOnly: true}
	rs.registers['%'] = &Register{Name: '%', Type: RegisterFileName, ReadOnly: true}
	rs.registers['#'] = &Register{Name: '#', Type: RegisterAlternate, ReadOnly: true}
	rs.registers[':'] = &Register{Name: ':', Type: RegisterCommand, ReadOnly: true}
	rs.registers['/'] = &Register{Name: '/', Type: RegisterSearch, ReadOnly: true}
	rs.registers['+'] = &Register{Name: '+', Type: RegisterClipboard}
	rs.registers['*'] = &Register{Name: '*', Type: RegisterSelection}
}

// Get returns the content of a register.
// Returns content, linewise, blockwise.
func (rs *RegisterStore) Get(name rune) (string, bool, bool) {
	// Handle uppercase named registers (same content as lowercase)
	if unicode.IsUpper(name) {
		name = unicode.ToLower(name)
	}

	// Handle clipboard registers - capture provider outside lock
	if name == '+' || name == '*' {
		rs.mu.RLock()
		clipboard := rs.clipboard
		rs.mu.RUnlock()

		if clipboard != nil {
			content, err := clipboard.Get()
			if err != nil {
				return "", false, false
			}
			return content, false, false
		}
	}

	rs.mu.RLock()
	defer rs.mu.RUnlock()

	reg, ok := rs.registers[name]
	if !ok {
		return "", false, false
	}
	return reg.Content, reg.Linewise, reg.Blockwise
}

// Set stores content in a register.
func (rs *RegisterStore) Set(name rune, content string, linewise, blockwise bool) {
	// Black hole register discards everything
	if name == '_' {
		return
	}

	// Handle clipboard registers - capture provider outside lock
	if name == '+' || name == '*' {
		rs.mu.RLock()
		clipboard := rs.clipboard
		rs.mu.RUnlock()

		if clipboard != nil {
			_ = clipboard.Set(content)
			return
		}
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Handle uppercase named registers (append mode)
	appendMode := false
	if unicode.IsUpper(name) {
		name = unicode.ToLower(name)
		appendMode = true
	}

	reg, ok := rs.registers[name]
	if !ok {
		// Unknown register
		return
	}

	if reg.ReadOnly {
		return
	}

	if appendMode && reg.Type == RegisterNamed {
		if reg.Linewise {
			reg.Content += "\n" + content
		} else {
			reg.Content += content
		}
	} else {
		reg.Content = content
		reg.Linewise = linewise
		reg.Blockwise = blockwise
	}
}

// SetYank stores a yank operation in register 0 and the unnamed register.
func (rs *RegisterStore) SetYank(content string, linewise, blockwise bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Store in register 0
	if reg, ok := rs.registers['0']; ok {
		reg.Content = content
		reg.Linewise = linewise
		reg.Blockwise = blockwise
	}

	// Also store in unnamed register
	if reg, ok := rs.registers['"']; ok {
		reg.Content = content
		reg.Linewise = linewise
		reg.Blockwise = blockwise
	}
}

// SetDelete stores a delete operation, rotating numbered registers.
func (rs *RegisterStore) SetDelete(content string, linewise, blockwise bool, small bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Small deletes go to the - register
	if small {
		if reg, ok := rs.registers['-']; ok {
			reg.Content = content
			reg.Linewise = linewise
			reg.Blockwise = blockwise
		}
		// Also store in unnamed register
		if reg, ok := rs.registers['"']; ok {
			reg.Content = content
			reg.Linewise = linewise
			reg.Blockwise = blockwise
		}
		return
	}

	// Rotate numbered registers (9 <- 8 <- ... <- 1)
	for i := 8; i > 0; i-- {
		rs.numberedRegisters[i].Content = rs.numberedRegisters[i-1].Content
		rs.numberedRegisters[i].Linewise = rs.numberedRegisters[i-1].Linewise
		rs.numberedRegisters[i].Blockwise = rs.numberedRegisters[i-1].Blockwise
	}

	// Store new delete in register 1
	rs.numberedRegisters[0].Content = content
	rs.numberedRegisters[0].Linewise = linewise
	rs.numberedRegisters[0].Blockwise = blockwise

	// Also store in unnamed register
	if reg, ok := rs.registers['"']; ok {
		reg.Content = content
		reg.Linewise = linewise
		reg.Blockwise = blockwise
	}
}

// SetLastInserted updates the last inserted text register.
func (rs *RegisterStore) SetLastInserted(content string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if reg, ok := rs.registers['.']; ok {
		reg.Content = content
		reg.Linewise = false
		reg.Blockwise = false
	}
}

// SetFileName updates the filename register.
func (rs *RegisterStore) SetFileName(filename string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if reg, ok := rs.registers['%']; ok {
		reg.Content = filename
	}
}

// SetAlternateFileName updates the alternate filename register.
func (rs *RegisterStore) SetAlternateFileName(filename string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if reg, ok := rs.registers['#']; ok {
		reg.Content = filename
	}
}

// SetLastCommand updates the last command register.
func (rs *RegisterStore) SetLastCommand(cmd string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if reg, ok := rs.registers[':']; ok {
		reg.Content = cmd
	}
}

// SetLastSearch updates the last search pattern register.
func (rs *RegisterStore) SetLastSearch(pattern string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if reg, ok := rs.registers['/']; ok {
		reg.Content = pattern
	}
}

// GetRegisterType returns the type of register for a given name.
func GetRegisterType(name rune) RegisterType {
	switch {
	case name == '"':
		return RegisterUnnamed
	case name >= 'a' && name <= 'z':
		return RegisterNamed
	case name >= 'A' && name <= 'Z':
		return RegisterNamed
	case name == '0':
		return RegisterLastYank
	case name >= '1' && name <= '9':
		return RegisterNumbered
	case name == '-':
		return RegisterSmallDelete
	case name == '_':
		return RegisterBlackHole
	case name == '.':
		return RegisterLastInserted
	case name == '%':
		return RegisterFileName
	case name == '#':
		return RegisterAlternate
	case name == ':':
		return RegisterCommand
	case name == '/':
		return RegisterSearch
	case name == '=':
		return RegisterExpression
	case name == '+':
		return RegisterClipboard
	case name == '*':
		return RegisterSelection
	default:
		return RegisterUnnamed
	}
}

// IsValidRegister returns true if the register name is valid.
func IsValidRegister(name rune) bool {
	switch {
	case name == '"':
		return true
	case name >= 'a' && name <= 'z':
		return true
	case name >= 'A' && name <= 'Z':
		return true
	case name >= '0' && name <= '9':
		return true
	case name == '-', name == '_', name == '.':
		return true
	case name == '%', name == '#', name == ':':
		return true
	case name == '/', name == '=':
		return true
	case name == '+', name == '*':
		return true
	default:
		return false
	}
}
