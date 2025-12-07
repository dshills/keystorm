package api

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"github.com/dshills/keystorm/internal/plugin/security"
)

// IntegrationModule implements the ks.integration API module.
// It provides plugin access to the integration layer (git, debug, tasks).
type IntegrationModule struct {
	ctx *Context
}

// NewIntegrationModule creates a new integration module.
func NewIntegrationModule(ctx *Context) *IntegrationModule {
	return &IntegrationModule{
		ctx: ctx,
	}
}

// Name returns the module name.
func (m *IntegrationModule) Name() string {
	return "integration"
}

// RequiredCapability returns the capability required for this module.
func (m *IntegrationModule) RequiredCapability() security.Capability {
	return security.CapabilityIntegration
}

// Register registers the module into the Lua state.
func (m *IntegrationModule) Register(L *lua.LState) error {
	mod := L.NewTable()

	// Core integration functions
	L.SetField(mod, "workspace_root", L.NewFunction(m.workspaceRoot))
	L.SetField(mod, "health", L.NewFunction(m.health))

	// Git submodule
	git := L.NewTable()
	L.SetField(git, "status", L.NewFunction(m.gitStatus))
	L.SetField(git, "branch", L.NewFunction(m.gitBranch))
	L.SetField(git, "branches", L.NewFunction(m.gitBranches))
	L.SetField(git, "add", L.NewFunction(m.gitAdd))
	L.SetField(git, "commit", L.NewFunction(m.gitCommit))
	L.SetField(git, "diff", L.NewFunction(m.gitDiff))
	L.SetField(mod, "git", git)

	// Debug submodule
	debug := L.NewTable()
	L.SetField(debug, "start", L.NewFunction(m.debugStart))
	L.SetField(debug, "stop", L.NewFunction(m.debugStop))
	L.SetField(debug, "sessions", L.NewFunction(m.debugSessions))
	L.SetField(debug, "set_breakpoint", L.NewFunction(m.debugSetBreakpoint))
	L.SetField(debug, "remove_breakpoint", L.NewFunction(m.debugRemoveBreakpoint))
	L.SetField(debug, "continue", L.NewFunction(m.debugContinue))
	L.SetField(debug, "step_over", L.NewFunction(m.debugStepOver))
	L.SetField(debug, "step_into", L.NewFunction(m.debugStepInto))
	L.SetField(debug, "step_out", L.NewFunction(m.debugStepOut))
	L.SetField(debug, "variables", L.NewFunction(m.debugVariables))
	L.SetField(mod, "debug", debug)

	// Task submodule
	task := L.NewTable()
	L.SetField(task, "list", L.NewFunction(m.taskList))
	L.SetField(task, "run", L.NewFunction(m.taskRun))
	L.SetField(task, "stop", L.NewFunction(m.taskStop))
	L.SetField(task, "status", L.NewFunction(m.taskStatus))
	L.SetField(task, "output", L.NewFunction(m.taskOutput))
	L.SetField(mod, "task", task)

	L.SetGlobal("_ks_integration", mod)
	return nil
}

// workspace_root() -> string
func (m *IntegrationModule) workspaceRoot(L *lua.LState) int {
	if m.ctx.Integration == nil {
		L.Push(lua.LString(""))
		return 1
	}
	L.Push(lua.LString(m.ctx.Integration.WorkspaceRoot()))
	return 1
}

// health() -> table
func (m *IntegrationModule) health(L *lua.LState) int {
	if m.ctx.Integration == nil {
		L.RaiseError("integration provider not available")
		return 0
	}

	health := m.ctx.Integration.Health()
	tbl := L.NewTable()
	L.SetField(tbl, "status", lua.LString(health.Status))
	L.SetField(tbl, "uptime", lua.LNumber(health.Uptime))
	L.SetField(tbl, "process_count", lua.LNumber(health.ProcessCount))
	L.SetField(tbl, "workspace_root", lua.LString(health.WorkspaceRoot))

	components := L.NewTable()
	for name, status := range health.Components {
		L.SetField(components, name, lua.LString(status))
	}
	L.SetField(tbl, "components", components)

	L.Push(tbl)
	return 1
}

// Git functions

// git.status() -> table
func (m *IntegrationModule) gitStatus(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Git() == nil {
		L.RaiseError("git provider not available")
		return 0
	}

	status, err := m.ctx.Integration.Git().Status()
	if err != nil {
		L.RaiseError("git status: %s", err.Error())
		return 0
	}

	tbl := L.NewTable()
	L.SetField(tbl, "branch", lua.LString(status.Branch))
	L.SetField(tbl, "ahead", lua.LNumber(status.Ahead))
	L.SetField(tbl, "behind", lua.LNumber(status.Behind))
	L.SetField(tbl, "has_conflicts", lua.LBool(status.HasConflicts))
	L.SetField(tbl, "is_clean", lua.LBool(status.IsClean))

	staged := L.NewTable()
	for i, f := range status.Staged {
		staged.RawSetInt(i+1, lua.LString(f))
	}
	L.SetField(tbl, "staged", staged)

	modified := L.NewTable()
	for i, f := range status.Modified {
		modified.RawSetInt(i+1, lua.LString(f))
	}
	L.SetField(tbl, "modified", modified)

	untracked := L.NewTable()
	for i, f := range status.Untracked {
		untracked.RawSetInt(i+1, lua.LString(f))
	}
	L.SetField(tbl, "untracked", untracked)

	L.Push(tbl)
	return 1
}

// git.branch() -> string
func (m *IntegrationModule) gitBranch(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Git() == nil {
		L.RaiseError("git provider not available")
		return 0
	}

	branch, err := m.ctx.Integration.Git().Branch()
	if err != nil {
		L.RaiseError("git branch: %s", err.Error())
		return 0
	}

	L.Push(lua.LString(branch))
	return 1
}

// git.branches() -> table
func (m *IntegrationModule) gitBranches(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Git() == nil {
		L.RaiseError("git provider not available")
		return 0
	}

	branches, err := m.ctx.Integration.Git().Branches()
	if err != nil {
		L.RaiseError("git branches: %s", err.Error())
		return 0
	}

	tbl := L.NewTable()
	for i, b := range branches {
		tbl.RawSetInt(i+1, lua.LString(b))
	}

	L.Push(tbl)
	return 1
}

// git.add(paths) -> nil
func (m *IntegrationModule) gitAdd(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Git() == nil {
		L.RaiseError("git provider not available")
		return 0
	}

	pathsTable := L.CheckTable(1)
	var paths []string
	pathsTable.ForEach(func(_, v lua.LValue) {
		if s, ok := v.(lua.LString); ok {
			paths = append(paths, string(s))
		}
	})

	if err := m.ctx.Integration.Git().Add(paths); err != nil {
		L.RaiseError("git add: %s", err.Error())
		return 0
	}

	return 0
}

// git.commit(message) -> nil
func (m *IntegrationModule) gitCommit(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Git() == nil {
		L.RaiseError("git provider not available")
		return 0
	}

	message := L.CheckString(1)
	if message == "" {
		L.ArgError(1, "commit message cannot be empty")
		return 0
	}

	if err := m.ctx.Integration.Git().Commit(message); err != nil {
		L.RaiseError("git commit: %s", err.Error())
		return 0
	}

	return 0
}

// git.diff(staged?) -> string
func (m *IntegrationModule) gitDiff(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Git() == nil {
		L.RaiseError("git provider not available")
		return 0
	}

	staged := L.OptBool(1, false)

	diff, err := m.ctx.Integration.Git().Diff(staged)
	if err != nil {
		L.RaiseError("git diff: %s", err.Error())
		return 0
	}

	L.Push(lua.LString(diff))
	return 1
}

// Debug functions

// debug.start(config) -> session_id
func (m *IntegrationModule) debugStart(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Debug() == nil {
		L.RaiseError("debug provider not available")
		return 0
	}

	configTable := L.CheckTable(1)
	config := DebugConfig{
		Adapter:     getStringField(L, configTable, "adapter"),
		Program:     getStringField(L, configTable, "program"),
		Cwd:         getStringField(L, configTable, "cwd"),
		StopOnEntry: getBoolField(L, configTable, "stop_on_entry"),
	}

	// Parse args
	if argsLV := L.GetField(configTable, "args"); argsLV.Type() == lua.LTTable {
		argsTable := argsLV.(*lua.LTable)
		argsTable.ForEach(func(_, v lua.LValue) {
			if s, ok := v.(lua.LString); ok {
				config.Args = append(config.Args, string(s))
			}
		})
	}

	// Parse env
	if envLV := L.GetField(configTable, "env"); envLV.Type() == lua.LTTable {
		config.Env = make(map[string]string)
		envTable := envLV.(*lua.LTable)
		envTable.ForEach(func(k, v lua.LValue) {
			if ks, ok := k.(lua.LString); ok {
				if vs, ok := v.(lua.LString); ok {
					config.Env[string(ks)] = string(vs)
				}
			}
		})
	}

	sessionID, err := m.ctx.Integration.Debug().Start(config)
	if err != nil {
		L.RaiseError("debug start: %s", err.Error())
		return 0
	}

	L.Push(lua.LString(sessionID))
	return 1
}

// debug.stop(session_id) -> nil
func (m *IntegrationModule) debugStop(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Debug() == nil {
		L.RaiseError("debug provider not available")
		return 0
	}

	sessionID := L.CheckString(1)
	if err := m.ctx.Integration.Debug().Stop(sessionID); err != nil {
		L.RaiseError("debug stop: %s", err.Error())
		return 0
	}

	return 0
}

// debug.sessions() -> table
func (m *IntegrationModule) debugSessions(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Debug() == nil {
		L.RaiseError("debug provider not available")
		return 0
	}

	sessions := m.ctx.Integration.Debug().Sessions()
	tbl := L.NewTable()

	for i, s := range sessions {
		session := L.NewTable()
		L.SetField(session, "id", lua.LString(s.ID))
		L.SetField(session, "adapter", lua.LString(s.Adapter))
		L.SetField(session, "program", lua.LString(s.Program))
		L.SetField(session, "state", lua.LString(s.State))
		tbl.RawSetInt(i+1, session)
	}

	L.Push(tbl)
	return 1
}

// debug.set_breakpoint(file, line) -> breakpoint_id
func (m *IntegrationModule) debugSetBreakpoint(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Debug() == nil {
		L.RaiseError("debug provider not available")
		return 0
	}

	file := L.CheckString(1)
	line := L.CheckInt(2)

	id, err := m.ctx.Integration.Debug().SetBreakpoint(file, line)
	if err != nil {
		L.RaiseError("debug set_breakpoint: %s", err.Error())
		return 0
	}

	L.Push(lua.LString(id))
	return 1
}

// debug.remove_breakpoint(id) -> nil
func (m *IntegrationModule) debugRemoveBreakpoint(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Debug() == nil {
		L.RaiseError("debug provider not available")
		return 0
	}

	id := L.CheckString(1)
	if err := m.ctx.Integration.Debug().RemoveBreakpoint(id); err != nil {
		L.RaiseError("debug remove_breakpoint: %s", err.Error())
		return 0
	}

	return 0
}

// debug.continue(session_id) -> nil
func (m *IntegrationModule) debugContinue(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Debug() == nil {
		L.RaiseError("debug provider not available")
		return 0
	}

	sessionID := L.CheckString(1)
	if err := m.ctx.Integration.Debug().Continue(sessionID); err != nil {
		L.RaiseError("debug continue: %s", err.Error())
		return 0
	}

	return 0
}

// debug.step_over(session_id) -> nil
func (m *IntegrationModule) debugStepOver(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Debug() == nil {
		L.RaiseError("debug provider not available")
		return 0
	}

	sessionID := L.CheckString(1)
	if err := m.ctx.Integration.Debug().StepOver(sessionID); err != nil {
		L.RaiseError("debug step_over: %s", err.Error())
		return 0
	}

	return 0
}

// debug.step_into(session_id) -> nil
func (m *IntegrationModule) debugStepInto(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Debug() == nil {
		L.RaiseError("debug provider not available")
		return 0
	}

	sessionID := L.CheckString(1)
	if err := m.ctx.Integration.Debug().StepInto(sessionID); err != nil {
		L.RaiseError("debug step_into: %s", err.Error())
		return 0
	}

	return 0
}

// debug.step_out(session_id) -> nil
func (m *IntegrationModule) debugStepOut(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Debug() == nil {
		L.RaiseError("debug provider not available")
		return 0
	}

	sessionID := L.CheckString(1)
	if err := m.ctx.Integration.Debug().StepOut(sessionID); err != nil {
		L.RaiseError("debug step_out: %s", err.Error())
		return 0
	}

	return 0
}

// debug.variables(session_id) -> table
func (m *IntegrationModule) debugVariables(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Debug() == nil {
		L.RaiseError("debug provider not available")
		return 0
	}

	sessionID := L.CheckString(1)
	vars, err := m.ctx.Integration.Debug().Variables(sessionID)
	if err != nil {
		L.RaiseError("debug variables: %s", err.Error())
		return 0
	}

	tbl := L.NewTable()
	for i, v := range vars {
		varTbl := L.NewTable()
		L.SetField(varTbl, "name", lua.LString(v.Name))
		L.SetField(varTbl, "value", lua.LString(v.Value))
		L.SetField(varTbl, "type", lua.LString(v.Type))
		tbl.RawSetInt(i+1, varTbl)
	}

	L.Push(tbl)
	return 1
}

// Task functions

// task.list() -> table
func (m *IntegrationModule) taskList(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Task() == nil {
		L.RaiseError("task provider not available")
		return 0
	}

	tasks, err := m.ctx.Integration.Task().List()
	if err != nil {
		L.RaiseError("task list: %s", err.Error())
		return 0
	}

	tbl := L.NewTable()
	for i, t := range tasks {
		taskTbl := L.NewTable()
		L.SetField(taskTbl, "name", lua.LString(t.Name))
		L.SetField(taskTbl, "source", lua.LString(t.Source))
		L.SetField(taskTbl, "description", lua.LString(t.Description))
		L.SetField(taskTbl, "command", lua.LString(t.Command))
		tbl.RawSetInt(i+1, taskTbl)
	}

	L.Push(tbl)
	return 1
}

// task.run(name) -> task_id
func (m *IntegrationModule) taskRun(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Task() == nil {
		L.RaiseError("task provider not available")
		return 0
	}

	name := L.CheckString(1)
	taskID, err := m.ctx.Integration.Task().Run(name)
	if err != nil {
		L.RaiseError("task run: %s", err.Error())
		return 0
	}

	L.Push(lua.LString(taskID))
	return 1
}

// task.stop(task_id) -> nil
func (m *IntegrationModule) taskStop(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Task() == nil {
		L.RaiseError("task provider not available")
		return 0
	}

	taskID := L.CheckString(1)
	if err := m.ctx.Integration.Task().Stop(taskID); err != nil {
		L.RaiseError("task stop: %s", err.Error())
		return 0
	}

	return 0
}

// task.status(task_id) -> table
func (m *IntegrationModule) taskStatus(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Task() == nil {
		L.RaiseError("task provider not available")
		return 0
	}

	taskID := L.CheckString(1)
	status, err := m.ctx.Integration.Task().Status(taskID)
	if err != nil {
		L.RaiseError("task status: %s", err.Error())
		return 0
	}

	tbl := L.NewTable()
	L.SetField(tbl, "id", lua.LString(status.ID))
	L.SetField(tbl, "name", lua.LString(status.Name))
	L.SetField(tbl, "state", lua.LString(status.State))
	L.SetField(tbl, "exit_code", lua.LNumber(status.ExitCode))
	L.SetField(tbl, "start_time", lua.LNumber(status.StartTime))
	L.SetField(tbl, "end_time", lua.LNumber(status.EndTime))

	L.Push(tbl)
	return 1
}

// task.output(task_id) -> string
func (m *IntegrationModule) taskOutput(L *lua.LState) int {
	if m.ctx.Integration == nil || m.ctx.Integration.Task() == nil {
		L.RaiseError("task provider not available")
		return 0
	}

	taskID := L.CheckString(1)
	output, err := m.ctx.Integration.Task().Output(taskID)
	if err != nil {
		L.RaiseError("task output: %s", err.Error())
		return 0
	}

	L.Push(lua.LString(output))
	return 1
}

// Helper functions

func getStringField(L *lua.LState, tbl *lua.LTable, key string) string {
	lv := L.GetField(tbl, key)
	if s, ok := lv.(lua.LString); ok {
		return string(s)
	}
	return ""
}

func getBoolField(L *lua.LState, tbl *lua.LTable, key string) bool {
	lv := L.GetField(tbl, key)
	if b, ok := lv.(lua.LBool); ok {
		return bool(b)
	}
	return false
}

// Ensure IntegrationModule implements Module interface
var _ Module = (*IntegrationModule)(nil)

// Suppress unused fmt import warning
var _ = fmt.Sprintf
