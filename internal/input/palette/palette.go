package palette

import (
	"fmt"
	"sort"
	"sync"
)

// Palette provides searchable access to editor commands.
type Palette struct {
	mu       sync.RWMutex
	commands map[string]*Command
	history  *History
	filter   *Filter

	// onChange callbacks are called when commands are added/removed.
	onChange []func()
}

// New creates a new command palette.
func New() *Palette {
	return &Palette{
		commands: make(map[string]*Command),
		history:  NewHistory(100),
		filter:   NewFilter(),
	}
}

// NewWithHistory creates a palette with a custom history size.
func NewWithHistory(historySize int) *Palette {
	return &Palette{
		commands: make(map[string]*Command),
		history:  NewHistory(historySize),
		filter:   NewFilter(),
	}
}

// Register adds a command to the palette.
// If a command with the same ID exists, it is replaced.
func (p *Palette) Register(cmd *Command) error {
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}
	if cmd.ID == "" {
		return fmt.Errorf("command ID cannot be empty")
	}
	if cmd.Title == "" {
		return fmt.Errorf("command title cannot be empty")
	}

	p.mu.Lock()
	p.commands[cmd.ID] = cmd
	p.mu.Unlock()

	p.notifyChange()
	return nil
}

// RegisterAll adds multiple commands to the palette.
func (p *Palette) RegisterAll(commands []*Command) error {
	for _, cmd := range commands {
		if err := p.Register(cmd); err != nil {
			return err
		}
	}
	return nil
}

// Unregister removes a command from the palette.
func (p *Palette) Unregister(id string) bool {
	p.mu.Lock()
	_, exists := p.commands[id]
	if exists {
		delete(p.commands, id)
	}
	p.mu.Unlock()

	if exists {
		p.notifyChange()
	}
	return exists
}

// UnregisterBySource removes all commands from a specific source.
func (p *Palette) UnregisterBySource(source string) int {
	p.mu.Lock()
	count := 0
	for id, cmd := range p.commands {
		if cmd.Source == source {
			delete(p.commands, id)
			count++
		}
	}
	p.mu.Unlock()

	if count > 0 {
		p.notifyChange()
	}
	return count
}

// Get retrieves a command by ID.
func (p *Palette) Get(id string) *Command {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.commands[id]
}

// Has checks if a command exists.
func (p *Palette) Has(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, exists := p.commands[id]
	return exists
}

// All returns all registered commands.
func (p *Palette) All() []*Command {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*Command, 0, len(p.commands))
	for _, cmd := range p.commands {
		result = append(result, cmd)
	}

	// Sort by title for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Title < result[j].Title
	})

	return result
}

// Count returns the number of registered commands.
func (p *Palette) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.commands)
}

// Search finds commands matching the query.
// Results are sorted by relevance, with recent commands prioritized.
func (p *Palette) Search(query string, limit int) []SearchResult {
	p.mu.RLock()
	commands := make([]*Command, 0, len(p.commands))
	for _, cmd := range p.commands {
		commands = append(commands, cmd)
	}
	p.mu.RUnlock()

	if query == "" {
		// Return recent commands first, then alphabetical
		return p.recentCommands(commands, limit)
	}

	// Perform fuzzy search with limit for early cutoff optimization
	// We request more results than needed since history boost may reorder
	searchLimit := limit
	if searchLimit > 0 {
		searchLimit = limit * 2 // Get extra results for history reordering
		if searchLimit < 50 {
			searchLimit = 50 // Minimum to ensure good coverage
		}
	}
	results := p.filter.Search(commands, query, searchLimit)

	// Boost recent commands
	for i := range results {
		pos := p.history.Position(results[i].Command.ID)
		if pos >= 0 {
			// Recent commands get a bonus (more recent = higher bonus)
			results[i].Score += (100 - pos)
		}
	}

	// Re-sort after history boost with deterministic tie-breaker
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Command.Title < results[j].Command.Title
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// recentCommands returns commands sorted by recency, then alphabetically.
func (p *Palette) recentCommands(commands []*Command, limit int) []SearchResult {
	results := make([]SearchResult, 0, len(commands))

	for _, cmd := range commands {
		pos := p.history.Position(cmd.ID)
		score := 0
		if pos >= 0 {
			// Recent commands get positive score
			score = 1000 - pos
		}
		results = append(results, SearchResult{
			Command: cmd,
			Score:   score,
		})
	}

	// Sort: by score desc (recent first), then by title
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Command.Title < results[j].Command.Title
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// Execute runs a command by ID with the given arguments.
// History is only updated after successful execution.
func (p *Palette) Execute(id string, args map[string]any) error {
	p.mu.RLock()
	cmd, exists := p.commands[id]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("unknown command: %s", id)
	}

	// Execute the command
	err := cmd.Execute(args)

	// Only record in history after successful execution
	if err == nil {
		p.history.Add(id)
	}

	return err
}

// ExecuteWithValidation runs a command after validating arguments.
// This is the same as Execute but makes the validation step explicit.
func (p *Palette) ExecuteWithValidation(id string, args map[string]any) error {
	return p.Execute(id, args)
}

// History returns the command history.
func (p *Palette) History() *History {
	return p.history
}

// RecentCommands returns IDs of recently executed commands.
func (p *Palette) RecentCommands(limit int) []string {
	return p.history.Recent(limit)
}

// Categories returns all unique command categories.
func (p *Palette) Categories() []string {
	p.mu.RLock()
	commands := make([]*Command, 0, len(p.commands))
	for _, cmd := range p.commands {
		commands = append(commands, cmd)
	}
	p.mu.RUnlock()

	return Categories(commands)
}

// CommandsByCategory returns commands in the specified category.
func (p *Palette) CommandsByCategory(category string) []*Command {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*Command, 0)
	for _, cmd := range p.commands {
		if cmd.Category == category {
			result = append(result, cmd)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Title < result[j].Title
	})

	return result
}

// OnChange registers a callback for command list changes.
// Callbacks are invoked after command registration/unregistration.
// Note: Callbacks should not call OnChange or modify the callback list.
// Callbacks may safely read/execute commands but should avoid
// registering/unregistering commands to prevent infinite loops.
func (p *Palette) OnChange(fn func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onChange = append(p.onChange, fn)
}

// notifyChange calls all registered change callbacks.
// Callbacks are invoked without holding locks to prevent deadlocks.
func (p *Palette) notifyChange() {
	p.mu.RLock()
	callbacks := make([]func(), len(p.onChange))
	copy(callbacks, p.onChange)
	p.mu.RUnlock()

	for _, fn := range callbacks {
		fn()
	}
}

// SetFilter sets a custom filter for searching.
func (p *Palette) SetFilter(filter *Filter) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.filter = filter
}

// Clear removes all commands and clears history.
func (p *Palette) Clear() {
	p.mu.Lock()
	p.commands = make(map[string]*Command)
	p.mu.Unlock()

	p.history.Clear()
	p.notifyChange()
}
