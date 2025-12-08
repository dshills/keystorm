package integration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dshills/keystorm/internal/integration/debug"
	"github.com/dshills/keystorm/internal/integration/git"
	"github.com/dshills/keystorm/internal/integration/task"
	"github.com/dshills/keystorm/internal/plugin/api"
)

// Provider implements api.IntegrationProvider by wrapping the integration Manager
// and providing access to Git, Debug, and Task subsystems.
//
// This provider bridges the plugin API with the integration layer, allowing
// Lua plugins to access integration features through a standardized interface.
type Provider struct {
	mu sync.RWMutex

	manager   *Manager
	gitRepo   *git.Repository
	debugSess *debugSessionManager
	taskDisc  *task.Discovery
	taskExec  *task.Executor
	workspace string
}

// ProviderOption configures a Provider.
type ProviderOption func(*Provider)

// WithGitRepository sets the git repository for git operations.
func WithGitRepository(repo *git.Repository) ProviderOption {
	return func(p *Provider) {
		p.gitRepo = repo
	}
}

// WithDebugSession sets up debug session management.
func WithDebugSession() ProviderOption {
	return func(p *Provider) {
		p.debugSess = newDebugSessionManager()
	}
}

// WithTaskDiscovery sets the task discovery for task operations.
func WithTaskDiscovery(disc *task.Discovery) ProviderOption {
	return func(p *Provider) {
		p.taskDisc = disc
	}
}

// WithTaskExecutor sets the task executor for task operations.
func WithTaskExecutor(exec *task.Executor) ProviderOption {
	return func(p *Provider) {
		p.taskExec = exec
	}
}

// WithWorkspace sets the workspace root for the provider.
func WithWorkspace(workspace string) ProviderOption {
	return func(p *Provider) {
		p.workspace = workspace
	}
}

// NewProvider creates a new integration provider.
func NewProvider(manager *Manager, opts ...ProviderOption) *Provider {
	p := &Provider{
		manager: manager,
	}

	for _, opt := range opts {
		opt(p)
	}

	// Use manager's workspace if not explicitly set
	if p.workspace == "" && manager != nil {
		p.workspace = manager.WorkspaceRoot()
	}

	return p
}

// WorkspaceRoot returns the workspace root directory.
func (p *Provider) WorkspaceRoot() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.workspace
}

// Health returns the integration layer health status.
func (p *Provider) Health() api.IntegrationHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.manager == nil {
		return api.IntegrationHealth{
			Status:     "unavailable",
			Components: make(map[string]string),
		}
	}

	health := p.manager.Health()

	// Convert internal HealthStatus to api.IntegrationHealth
	components := make(map[string]string)
	for name, ch := range health.Components {
		components[name] = ch.Status.String()
	}

	return api.IntegrationHealth{
		Status:        health.Status.String(),
		Uptime:        health.Uptime.Milliseconds(),
		ProcessCount:  health.ProcessCount,
		WorkspaceRoot: health.WorkspaceRoot,
		Components:    components,
	}
}

// Git returns the git provider, or nil if not available.
func (p *Provider) Git() api.GitProvider {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.gitRepo == nil {
		return nil
	}

	return &gitProviderAdapter{repo: p.gitRepo}
}

// Debug returns the debug provider, or nil if not available.
func (p *Provider) Debug() api.DebugProvider {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.debugSess == nil {
		return nil
	}

	return &debugProviderAdapter{mgr: p.debugSess}
}

// Task returns the task provider, or nil if not available.
func (p *Provider) Task() api.TaskProvider {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.taskDisc == nil || p.taskExec == nil {
		return nil
	}

	return &taskProviderAdapter{
		disc:      p.taskDisc,
		exec:      p.taskExec,
		workspace: p.workspace,
	}
}

// SetGitRepository updates the git repository.
func (p *Provider) SetGitRepository(repo *git.Repository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gitRepo = repo
}

// SetTaskDiscovery updates the task discovery.
func (p *Provider) SetTaskDiscovery(disc *task.Discovery) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.taskDisc = disc
}

// SetTaskExecutor updates the task executor.
func (p *Provider) SetTaskExecutor(exec *task.Executor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.taskExec = exec
}

// SetWorkspace updates the workspace root.
func (p *Provider) SetWorkspace(workspace string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.workspace = workspace
}

// gitProviderAdapter adapts git.Repository to api.GitProvider.
type gitProviderAdapter struct {
	repo *git.Repository
}

func (g *gitProviderAdapter) Status() (api.GitStatus, error) {
	status, err := g.repo.Status()
	if err != nil {
		return api.GitStatus{}, err
	}

	// Convert []FileStatus to []string for Staged and Modified
	staged := make([]string, len(status.Staged))
	for i, fs := range status.Staged {
		staged[i] = fs.Path
	}

	modified := make([]string, len(status.Unstaged))
	for i, fs := range status.Unstaged {
		modified[i] = fs.Path
	}

	return api.GitStatus{
		Branch:       status.Branch,
		Ahead:        status.Ahead,
		Behind:       status.Behind,
		Staged:       staged,
		Modified:     modified,
		Untracked:    status.Untracked,
		HasConflicts: status.HasConflicts(),
		IsClean:      !status.HasChanges(),
	}, nil
}

func (g *gitProviderAdapter) Branch() (string, error) {
	return g.repo.CurrentBranch()
}

func (g *gitProviderAdapter) Branches() ([]string, error) {
	branches, err := g.repo.ListBranches()
	if err != nil {
		return nil, err
	}

	names := make([]string, len(branches))
	for i, branch := range branches {
		names[i] = branch.Name
	}
	return names, nil
}

func (g *gitProviderAdapter) Commit(message string) error {
	_, err := g.repo.Commit(message, git.CommitOptions{})
	return err
}

func (g *gitProviderAdapter) Add(paths []string) error {
	return g.repo.Stage(paths...)
}

func (g *gitProviderAdapter) Diff(staged bool) (string, error) {
	return g.repo.DiffRaw(git.DiffOptions{Staged: staged})
}

// debugSessionManager manages debug sessions for the provider.
// This fills the gap where the debug package doesn't have a Manager type.
type debugSessionManager struct {
	mu              sync.RWMutex
	sessions        map[string]*debug.Session
	breakpointMgr   *debug.BreakpointManager
	nextID          atomic.Uint64
	sessionConfigs  map[string]api.DebugConfig
	sessionPrograms map[string]string
	sessionAdapters map[string]string
}

func newDebugSessionManager() *debugSessionManager {
	return &debugSessionManager{
		sessions:        make(map[string]*debug.Session),
		breakpointMgr:   debug.NewBreakpointManager(nil),
		sessionConfigs:  make(map[string]api.DebugConfig),
		sessionPrograms: make(map[string]string),
		sessionAdapters: make(map[string]string),
	}
}

func (m *debugSessionManager) generateID() string {
	return fmt.Sprintf("session-%d", m.nextID.Add(1))
}

func (m *debugSessionManager) addSession(id string, session *debug.Session, config api.DebugConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = session
	m.sessionConfigs[id] = config
	m.sessionPrograms[id] = config.Program
	m.sessionAdapters[id] = config.Adapter
	// Update breakpoint manager with session
	m.breakpointMgr = debug.NewBreakpointManager(session)
}

func (m *debugSessionManager) getSession(id string) (*debug.Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *debugSessionManager) listSessions() []sessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]sessionInfo, 0, len(m.sessions))
	for id, sess := range m.sessions {
		result = append(result, sessionInfo{
			id:      id,
			session: sess,
			adapter: m.sessionAdapters[id],
			program: m.sessionPrograms[id],
		})
	}
	return result
}

func (m *debugSessionManager) removeSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	delete(m.sessionConfigs, id)
	delete(m.sessionPrograms, id)
	delete(m.sessionAdapters, id)
}

type sessionInfo struct {
	id      string
	session *debug.Session
	adapter string
	program string
}

// debugProviderAdapter adapts debugSessionManager to api.DebugProvider.
type debugProviderAdapter struct {
	mgr *debugSessionManager
}

func (d *debugProviderAdapter) Start(config api.DebugConfig) (string, error) {
	// Create a new session ID
	id := d.mgr.generateID()

	// Note: Actual session creation requires setting up DAP transport
	// which involves creating a client, connecting to the adapter, etc.
	// For now, we create a placeholder that can be expanded later.
	// This follows the pattern where the debug layer needs proper setup.

	// Store the config for the session
	d.mgr.mu.Lock()
	d.mgr.sessionConfigs[id] = config
	d.mgr.sessionPrograms[id] = config.Program
	d.mgr.sessionAdapters[id] = config.Adapter
	d.mgr.mu.Unlock()

	return id, nil
}

func (d *debugProviderAdapter) Stop(sessionID string) error {
	session, ok := d.mgr.getSession(sessionID)
	if !ok {
		// Check if it's just a placeholder
		d.mgr.mu.RLock()
		_, hasConfig := d.mgr.sessionConfigs[sessionID]
		d.mgr.mu.RUnlock()

		if hasConfig {
			d.mgr.removeSession(sessionID)
			return nil
		}
		return ErrSessionNotFound
	}

	// Disconnect and terminate the session
	if err := session.Disconnect(context.Background(), true); err != nil {
		return err
	}

	d.mgr.removeSession(sessionID)
	return nil
}

func (d *debugProviderAdapter) Sessions() []api.DebugSession {
	sessions := d.mgr.listSessions()
	result := make([]api.DebugSession, 0, len(sessions))

	// Also include sessions that only have configs (pending sessions)
	d.mgr.mu.RLock()
	configIDs := make(map[string]bool)
	for id := range d.mgr.sessionConfigs {
		configIDs[id] = true
	}
	d.mgr.mu.RUnlock()

	seen := make(map[string]bool)
	for _, s := range sessions {
		seen[s.id] = true
		result = append(result, api.DebugSession{
			ID:      s.id,
			Adapter: s.adapter,
			Program: s.program,
			State:   s.session.State().String(),
		})
	}

	// Add pending sessions
	d.mgr.mu.RLock()
	for id := range configIDs {
		if !seen[id] {
			result = append(result, api.DebugSession{
				ID:      id,
				Adapter: d.mgr.sessionAdapters[id],
				Program: d.mgr.sessionPrograms[id],
				State:   "pending",
			})
		}
	}
	d.mgr.mu.RUnlock()

	return result
}

func (d *debugProviderAdapter) SetBreakpoint(file string, line int) (string, error) {
	bp, err := d.mgr.breakpointMgr.AddLineBreakpoint(file, line)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", bp.ID), nil
}

func (d *debugProviderAdapter) RemoveBreakpoint(id string) error {
	// Parse the ID as int
	var bpID int
	_, err := fmt.Sscanf(id, "%d", &bpID)
	if err != nil {
		return fmt.Errorf("invalid breakpoint ID: %s", id)
	}
	return d.mgr.breakpointMgr.RemoveBreakpoint(bpID)
}

func (d *debugProviderAdapter) Continue(sessionID string) error {
	session, ok := d.mgr.getSession(sessionID)
	if !ok {
		return ErrSessionNotFound
	}
	// Use current thread from session
	threadID := session.CurrentThread()
	return session.Continue(context.Background(), threadID)
}

func (d *debugProviderAdapter) StepOver(sessionID string) error {
	session, ok := d.mgr.getSession(sessionID)
	if !ok {
		return ErrSessionNotFound
	}
	threadID := session.CurrentThread()
	return session.Next(context.Background(), threadID)
}

func (d *debugProviderAdapter) StepInto(sessionID string) error {
	session, ok := d.mgr.getSession(sessionID)
	if !ok {
		return ErrSessionNotFound
	}
	threadID := session.CurrentThread()
	return session.StepIn(context.Background(), threadID)
}

func (d *debugProviderAdapter) StepOut(sessionID string) error {
	session, ok := d.mgr.getSession(sessionID)
	if !ok {
		return ErrSessionNotFound
	}
	threadID := session.CurrentThread()
	return session.StepOut(context.Background(), threadID)
}

func (d *debugProviderAdapter) Variables(sessionID string) ([]api.DebugVariable, error) {
	session, ok := d.mgr.getSession(sessionID)
	if !ok {
		return nil, ErrSessionNotFound
	}

	// Get stack trace to find the current frame's variables reference
	threadID := session.CurrentThread()
	frames, _, err := session.GetStackTrace(context.Background(), threadID, 0, 1)
	if err != nil {
		return nil, err
	}

	if len(frames) == 0 {
		return []api.DebugVariable{}, nil
	}

	// Get scopes for the current frame
	scopes, err := session.GetScopes(context.Background(), frames[0].ID)
	if err != nil {
		return nil, err
	}

	// Collect variables from all scopes
	var result []api.DebugVariable
	for _, scope := range scopes {
		vars, err := session.GetVariables(context.Background(), scope.VariablesReference)
		if err != nil {
			continue // Skip scopes that fail
		}
		for _, v := range vars {
			result = append(result, api.DebugVariable{
				Name:  v.Name,
				Value: v.Value,
				Type:  v.Type,
			})
		}
	}
	return result, nil
}

// taskProviderAdapter adapts task.Discovery and task.Executor to api.TaskProvider.
type taskProviderAdapter struct {
	disc      *task.Discovery
	exec      *task.Executor
	workspace string
}

func (t *taskProviderAdapter) List() ([]api.TaskInfo, error) {
	opts := task.DefaultDiscoveryOptions(t.workspace)
	result, err := t.disc.Discover(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	tasks := make([]api.TaskInfo, len(result.Tasks))
	for i, tk := range result.Tasks {
		tasks[i] = api.TaskInfo{
			Name:        tk.Name,
			Source:      tk.Source,
			Description: tk.Description,
			Command:     tk.Command,
		}
	}
	return tasks, nil
}

func (t *taskProviderAdapter) Run(name string) (string, error) {
	// Find the task by name
	opts := task.DefaultDiscoveryOptions(t.workspace)
	result, err := t.disc.Discover(context.Background(), opts)
	if err != nil {
		return "", err
	}

	var found *task.Task
	for _, tk := range result.Tasks {
		if tk.Name == name {
			found = tk
			break
		}
	}

	if found == nil {
		return "", ErrTaskNotFound
	}

	exec, err := t.exec.Execute(context.Background(), found)
	if err != nil {
		return "", err
	}

	return exec.ID, nil
}

func (t *taskProviderAdapter) Stop(taskID string) error {
	return t.exec.CancelExecution(taskID)
}

func (t *taskProviderAdapter) Status(taskID string) (api.TaskStatus, error) {
	exec, ok := t.exec.GetExecution(taskID)
	if !ok {
		return api.TaskStatus{}, ErrTaskNotFound
	}

	return api.TaskStatus{
		ID:        exec.ID,
		Name:      exec.Task.Name,
		State:     string(exec.State),
		ExitCode:  exec.ExitCode,
		StartTime: exec.StartTime.UnixMilli(),
	}, nil
}

func (t *taskProviderAdapter) Output(taskID string) (string, error) {
	exec, ok := t.exec.GetExecution(taskID)
	if !ok {
		return "", ErrTaskNotFound
	}

	// Get output lines and convert to string
	lines := exec.Output()
	var builder strings.Builder
	for _, line := range lines {
		builder.WriteString(line.Content)
		builder.WriteString("\n")
	}
	return builder.String(), nil
}

// EventBusAdapter adapts the new typed event bus for the Provider.
// It allows integration layer to publish events through the typed event system.
type EventBusAdapter struct {
	eventBus EventPublisher
}

// NewEventBusAdapter creates an adapter that bridges to the legacy EventPublisher.
func NewEventBusAdapter(eventBus EventPublisher) *EventBusAdapter {
	return &EventBusAdapter{eventBus: eventBus}
}

// Publish publishes an event through the event bus.
func (a *EventBusAdapter) Publish(eventType string, data map[string]any) {
	if a.eventBus != nil {
		// Add timestamp if not present
		if _, ok := data["timestamp"]; !ok {
			data["timestamp"] = time.Now().UnixMilli()
		}
		a.eventBus.Publish(eventType, data)
	}
}
