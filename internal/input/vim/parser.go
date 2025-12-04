package vim

import (
	"github.com/dshills/keystorm/internal/input/key"
)

// ParseStatus indicates the result of parsing a key event.
type ParseStatus uint8

const (
	// StatusPending indicates more input is needed.
	StatusPending ParseStatus = iota

	// StatusComplete indicates a complete command was parsed.
	StatusComplete

	// StatusInvalid indicates the sequence is invalid.
	StatusInvalid

	// StatusPassthrough indicates the key should be passed through.
	StatusPassthrough
)

// String returns a string representation of the status.
func (s ParseStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusComplete:
		return "complete"
	case StatusInvalid:
		return "invalid"
	case StatusPassthrough:
		return "passthrough"
	default:
		return "unknown"
	}
}

// ParseState represents the current state of the parser.
type ParseState uint8

const (
	// StateInitial is waiting for initial input.
	StateInitial ParseState = iota

	// StateCount is accumulating a count prefix.
	StateCount

	// StateRegister is waiting for a register name after ".
	StateRegister

	// StateOperator has received an operator, waiting for motion/text-object.
	StateOperator

	// StateOperatorCount is accumulating count after operator.
	StateOperatorCount

	// StateGPrefix has received 'g', waiting for second key.
	StateGPrefix

	// StateTextObjectPrefix has received 'i' or 'a', waiting for text object.
	StateTextObjectPrefix

	// StateCharSearch has received f/F/t/T, waiting for character.
	StateCharSearch

	// StateMarkSet has received 'm', waiting for mark name.
	StateMarkSet

	// StateMarkGoto has received '`' or "'", waiting for mark name.
	StateMarkGoto
)

// String returns a string representation of the state.
func (s ParseState) String() string {
	switch s {
	case StateInitial:
		return "initial"
	case StateCount:
		return "count"
	case StateRegister:
		return "register"
	case StateOperator:
		return "operator"
	case StateOperatorCount:
		return "operatorCount"
	case StateGPrefix:
		return "gPrefix"
	case StateTextObjectPrefix:
		return "textObjectPrefix"
	case StateCharSearch:
		return "charSearch"
	case StateMarkSet:
		return "markSet"
	case StateMarkGoto:
		return "markGoto"
	default:
		return "unknown"
	}
}

// Command represents a parsed Vim command.
type Command struct {
	// Count is the repeat count (0 means 1).
	Count int

	// Register is the target register (0 means default).
	Register rune

	// Operator is the operator, if any.
	Operator *Operator

	// Motion is the motion, if any.
	Motion *Motion

	// TextObject is the text object, if any.
	TextObject *TextObject

	// TextObjectPrefix is 'i' (inner) or 'a' (around).
	TextObjectPrefix TextObjectPrefix

	// CharArg is the character argument for f/F/t/T.
	CharArg rune

	// Linewise indicates line-wise operation (dd, yy, etc.).
	Linewise bool

	// Action is the action name to dispatch.
	Action string

	// Args holds additional arguments for the action.
	Args map[string]any
}

// NewCommand creates a new empty command.
func NewCommand() *Command {
	return &Command{
		Args: make(map[string]any),
	}
}

// GetCount returns the effective count (1 if none specified).
func (c *Command) GetCount() int {
	if c.Count <= 0 {
		return 1
	}
	return c.Count
}

// ParseResult contains the result of parsing a key event.
type ParseResult struct {
	// Status indicates the parse result.
	Status ParseStatus

	// Command is the parsed command (if Status == StatusComplete).
	Command *Command

	// PendingDisplay is a string showing pending keys (for status line).
	PendingDisplay string
}

// Parser parses Vim-style key sequences into commands.
type Parser struct {
	// Current parser state
	state ParseState

	// Accumulated state
	count1        CountState       // Pre-operator count
	count2        CountState       // Post-operator count
	register      rune             // Selected register
	operator      *Operator        // Pending operator
	textObjPrefix TextObjectPrefix // 'i' or 'a' for text objects
	charSearch    rune             // f/F/t/T waiting for char

	// Key accumulator for display
	pendingKeys []rune
}

// NewParser creates a new Vim command parser.
func NewParser() *Parser {
	return &Parser{
		state:       StateInitial,
		pendingKeys: make([]rune, 0, 8),
	}
}

// Reset clears all parser state.
func (p *Parser) Reset() {
	p.state = StateInitial
	p.count1.Reset()
	p.count2.Reset()
	p.register = 0
	p.operator = nil
	p.textObjPrefix = PrefixNone
	p.charSearch = 0
	p.pendingKeys = p.pendingKeys[:0]
}

// State returns the current parser state.
func (p *Parser) State() ParseState {
	return p.state
}

// PendingKeys returns the pending key display string.
func (p *Parser) PendingKeys() string {
	return string(p.pendingKeys)
}

// Parse processes a key event and returns the result.
func (p *Parser) Parse(event key.Event) ParseResult {
	// Only handle unmodified rune events (except Escape)
	if event.Key == key.KeyEscape {
		p.Reset()
		return ParseResult{Status: StatusPassthrough}
	}

	// For non-rune events, pass through
	if !event.IsRune() {
		return ParseResult{Status: StatusPassthrough}
	}

	// Modified keys (Ctrl, Alt, Meta) pass through
	if event.IsModified() {
		return ParseResult{Status: StatusPassthrough}
	}

	r := event.Rune
	p.pendingKeys = append(p.pendingKeys, r)

	switch p.state {
	case StateInitial:
		return p.parseInitial(r)

	case StateCount:
		return p.parseCount(r)

	case StateRegister:
		return p.parseRegister(r)

	case StateOperator:
		return p.parseOperator(r)

	case StateOperatorCount:
		return p.parseOperatorCount(r)

	case StateGPrefix:
		return p.parseGPrefix(r)

	case StateTextObjectPrefix:
		return p.parseTextObjectPrefix(r)

	case StateCharSearch:
		return p.parseCharSearch(r)

	case StateMarkSet:
		return p.parseMarkSet(r)

	case StateMarkGoto:
		return p.parseMarkGoto(r)

	default:
		p.Reset()
		return ParseResult{Status: StatusInvalid}
	}
}

// parseInitial handles input in the initial state.
func (p *Parser) parseInitial(r rune) ParseResult {
	// Count prefix (1-9, not 0 which is a motion)
	if IsCountStart(r) {
		p.state = StateCount
		p.count1.AccumulateDigit(r)
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Register prefix
	if r == '"' {
		p.state = StateRegister
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// 'g' prefix for g-commands
	if r == 'g' {
		p.state = StateGPrefix
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Operator
	if op := GetOperator(r); op != nil {
		p.operator = op
		p.state = StateOperator
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// f/F/t/T character search (check before motion since these are in motion map)
	if IsCharSearchMotion(r) {
		p.charSearch = r
		p.state = StateCharSearch
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Motion (single key)
	if m := GetMotion(r); m != nil {
		return p.completeMotion(m)
	}

	// Mark set 'm'
	if r == 'm' {
		p.state = StateMarkSet
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Mark goto ' or `
	if r == '\'' || r == '`' {
		p.state = StateMarkGoto
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Unknown key - pass through
	p.Reset()
	return ParseResult{Status: StatusPassthrough}
}

// parseCount handles input during count accumulation.
func (p *Parser) parseCount(r rune) ParseResult {
	// Continue accumulating digits
	if IsCountDigit(r) {
		p.count1.AccumulateDigit(r)
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Count complete, check what follows

	// Register prefix
	if r == '"' {
		p.state = StateRegister
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// 'g' prefix
	if r == 'g' {
		p.state = StateGPrefix
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Operator
	if op := GetOperator(r); op != nil {
		p.operator = op
		p.state = StateOperator
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// f/F/t/T character search (check before motion since these are in motion map)
	if IsCharSearchMotion(r) {
		p.charSearch = r
		p.state = StateCharSearch
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Motion
	if m := GetMotion(r); m != nil {
		return p.completeMotion(m)
	}

	// Invalid
	p.Reset()
	return ParseResult{Status: StatusInvalid}
}

// parseRegister handles input after ".
func (p *Parser) parseRegister(r rune) ParseResult {
	if !IsValidRegister(r) {
		p.Reset()
		return ParseResult{Status: StatusInvalid}
	}

	p.register = r

	// Continue parsing from initial-like state
	p.state = StateInitial
	return ParseResult{
		Status:         StatusPending,
		PendingDisplay: p.PendingKeys(),
	}
}

// parseOperator handles input after an operator key.
func (p *Parser) parseOperator(r rune) ParseResult {
	// Count after operator
	if IsCountStart(r) {
		p.state = StateOperatorCount
		p.count2.AccumulateDigit(r)
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Same operator key = line-wise (dd, yy, cc)
	if p.operator.Key == r {
		return p.completeLinewise()
	}

	// 'g' prefix for g-motions
	if r == 'g' {
		p.state = StateGPrefix
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Text object prefix 'i' or 'a'
	if IsTextObjectPrefix(r) {
		p.textObjPrefix = GetTextObjectPrefix(r)
		p.state = StateTextObjectPrefix
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// f/F/t/T character search (check before motion since these are in motion map)
	if IsCharSearchMotion(r) {
		p.charSearch = r
		p.state = StateCharSearch
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Motion
	if m := GetMotion(r); m != nil {
		return p.completeOperatorMotion(m)
	}

	// Invalid
	p.Reset()
	return ParseResult{Status: StatusInvalid}
}

// parseOperatorCount handles count after operator.
func (p *Parser) parseOperatorCount(r rune) ParseResult {
	// Continue accumulating digits
	if IsCountDigit(r) {
		p.count2.AccumulateDigit(r)
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Count complete, continue parsing motion/text-object

	// 'g' prefix
	if r == 'g' {
		p.state = StateGPrefix
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Text object prefix
	if IsTextObjectPrefix(r) {
		p.textObjPrefix = GetTextObjectPrefix(r)
		p.state = StateTextObjectPrefix
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// f/F/t/T character search (check before motion since these are in motion map)
	if IsCharSearchMotion(r) {
		p.charSearch = r
		p.state = StateCharSearch
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Motion
	if m := GetMotion(r); m != nil {
		return p.completeOperatorMotion(m)
	}

	// Invalid
	p.Reset()
	return ParseResult{Status: StatusInvalid}
}

// parseGPrefix handles input after 'g'.
func (p *Parser) parseGPrefix(r rune) ParseResult {
	// g-motions
	if m := GetGMotion(r); m != nil {
		if p.operator != nil {
			return p.completeOperatorMotion(m)
		}
		return p.completeMotion(m)
	}

	// g-operators (gu, gU, g~)
	if op := GetGOperator(r); op != nil {
		if p.operator != nil {
			// Can't have operator + g-operator
			p.Reset()
			return ParseResult{Status: StatusInvalid}
		}
		p.operator = op
		p.state = StateOperator
		return ParseResult{
			Status:         StatusPending,
			PendingDisplay: p.PendingKeys(),
		}
	}

	// Invalid g-command
	p.Reset()
	return ParseResult{Status: StatusInvalid}
}

// parseTextObjectPrefix handles input after 'i' or 'a'.
func (p *Parser) parseTextObjectPrefix(r rune) ParseResult {
	textObj := GetTextObject(r)
	if textObj == nil {
		p.Reset()
		return ParseResult{Status: StatusInvalid}
	}

	return p.completeTextObject(textObj)
}

// parseCharSearch handles input after f/F/t/T.
func (p *Parser) parseCharSearch(r rune) ParseResult {
	motion := GetMotion(p.charSearch)
	if motion == nil {
		p.Reset()
		return ParseResult{Status: StatusInvalid}
	}

	cmd := p.buildBaseCommand()
	cmd.Motion = motion
	cmd.CharArg = r

	if p.operator != nil {
		cmd.Operator = p.operator
		cmd.Action = p.operator.Action
	} else {
		cmd.Action = motion.Action
	}

	cmd.Args["char"] = string(r)

	p.Reset()
	return ParseResult{
		Status:  StatusComplete,
		Command: cmd,
	}
}

// parseMarkSet handles input after 'm'.
func (p *Parser) parseMarkSet(r rune) ParseResult {
	// Mark name must be a-z, A-Z, or 0-9
	if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
		p.Reset()
		return ParseResult{Status: StatusInvalid}
	}

	cmd := p.buildBaseCommand()
	cmd.Action = "mark.set"
	cmd.Args["mark"] = string(r)

	p.Reset()
	return ParseResult{
		Status:  StatusComplete,
		Command: cmd,
	}
}

// parseMarkGoto handles input after ' or `.
func (p *Parser) parseMarkGoto(r rune) ParseResult {
	// Mark name must be valid
	if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '\'' || r == '`' || r == '.' || r == '<' || r == '>') {
		p.Reset()
		return ParseResult{Status: StatusInvalid}
	}

	cmd := p.buildBaseCommand()
	cmd.Action = "mark.goto"
	cmd.Args["mark"] = string(r)

	p.Reset()
	return ParseResult{
		Status:  StatusComplete,
		Command: cmd,
	}
}

// completeMotion builds a complete motion command.
func (p *Parser) completeMotion(m *Motion) ParseResult {
	cmd := p.buildBaseCommand()
	cmd.Motion = m
	cmd.Action = m.Action

	p.Reset()
	return ParseResult{
		Status:  StatusComplete,
		Command: cmd,
	}
}

// completeOperatorMotion builds a complete operator+motion command.
func (p *Parser) completeOperatorMotion(m *Motion) ParseResult {
	cmd := p.buildBaseCommand()
	cmd.Operator = p.operator
	cmd.Motion = m
	cmd.Action = p.operator.Action

	cmd.Args["motion"] = m.Name
	cmd.Args["inclusive"] = m.Inclusive
	cmd.Args["linewise"] = m.Type == MotionLinewise

	p.Reset()
	return ParseResult{
		Status:  StatusComplete,
		Command: cmd,
	}
}

// completeTextObject builds a complete operator+text-object command.
func (p *Parser) completeTextObject(textObj *TextObject) ParseResult {
	cmd := p.buildBaseCommand()
	cmd.Operator = p.operator
	cmd.TextObject = textObj
	cmd.TextObjectPrefix = p.textObjPrefix

	if p.operator != nil {
		cmd.Action = p.operator.Action
	} else {
		// Text object without operator (in visual mode, selects the text)
		if p.textObjPrefix == PrefixInner {
			cmd.Action = textObj.InnerAction
		} else {
			cmd.Action = textObj.AroundAction
		}
	}

	cmd.Args["textObject"] = textObj.Name
	cmd.Args["inner"] = p.textObjPrefix == PrefixInner

	p.Reset()
	return ParseResult{
		Status:  StatusComplete,
		Command: cmd,
	}
}

// completeLinewise builds a complete line-wise operator command (dd, yy, etc.).
func (p *Parser) completeLinewise() ParseResult {
	cmd := p.buildBaseCommand()
	cmd.Operator = p.operator
	cmd.Linewise = true
	cmd.Action = p.operator.LinewiseAction

	p.Reset()
	return ParseResult{
		Status:  StatusComplete,
		Command: cmd,
	}
}

// buildBaseCommand creates a Command with common fields set.
func (p *Parser) buildBaseCommand() *Command {
	cmd := NewCommand()

	// Combine counts: pre-operator * post-operator
	cmd.Count = CombineCounts(p.count1.Get(), p.count2.Get())
	if cmd.Count == 1 && !p.count1.Active && !p.count2.Active {
		cmd.Count = 0 // No explicit count
	}

	cmd.Register = p.register

	return cmd
}
