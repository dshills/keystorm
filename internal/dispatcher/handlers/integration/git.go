package integration

import (
	"github.com/dshills/keystorm/internal/dispatcher/execctx"
	"github.com/dshills/keystorm/internal/dispatcher/handler"
	"github.com/dshills/keystorm/internal/input"
	"github.com/dshills/keystorm/internal/integration/git"
)

// Git action names.
const (
	ActionGitStatus   = "git.status"   // Get repository status
	ActionGitBranch   = "git.branch"   // Get current branch
	ActionGitBranches = "git.branches" // List all branches
	ActionGitCheckout = "git.checkout" // Switch branches
	ActionGitCommit   = "git.commit"   // Create a commit
	ActionGitAdd      = "git.add"      // Stage files
	ActionGitDiff     = "git.diff"     // Get diff output
	ActionGitLog      = "git.log"      // Get commit history
	ActionGitPull     = "git.pull"     // Pull from remote
	ActionGitPush     = "git.push"     // Push to remote
	ActionGitStash    = "git.stash"    // Stash changes
	ActionGitBlame    = "git.blame"    // Show file blame
)

// GitManager provides git operations.
// This is typically satisfied by *git.Repository.
type GitManager interface {
	// Status returns the current repository status.
	Status() (*git.Status, error)

	// CurrentBranch returns the current branch name.
	CurrentBranch() (string, error)

	// Branches lists all branches.
	Branches() ([]*git.Reference, error)

	// Checkout switches to a branch.
	Checkout(branch string) error

	// Commit creates a commit with the given message.
	Commit(message string, opts git.CommitOptions) (*git.Commit, error)

	// Add stages files for commit.
	Add(paths ...string) error

	// AddAll stages all changes.
	AddAll() error

	// Diff returns the diff for staged or unstaged changes.
	Diff(staged bool) (string, error)

	// DiffFile returns the diff for a specific file.
	DiffFile(path string, staged bool) (string, error)

	// Log returns commit history.
	Log(n int) ([]*git.Commit, error)

	// Pull pulls from remote.
	Pull() error

	// Push pushes to remote.
	Push() error

	// Stash stashes changes.
	Stash(message string) error

	// StashPop pops the most recent stash.
	StashPop() error

	// Blame returns blame information for a file.
	Blame(path string) ([]git.BlameLine, error)
}

const gitManagerKey = "_git_manager"

// GitHandler handles git-related actions.
type GitHandler struct {
	manager GitManager
}

// NewGitHandler creates a new git handler.
func NewGitHandler() *GitHandler {
	return &GitHandler{}
}

// NewGitHandlerWithManager creates a handler with a git manager.
func NewGitHandlerWithManager(manager GitManager) *GitHandler {
	return &GitHandler{manager: manager}
}

// Namespace returns the git namespace.
func (h *GitHandler) Namespace() string {
	return "git"
}

// CanHandle returns true if this handler can process the action.
func (h *GitHandler) CanHandle(actionName string) bool {
	switch actionName {
	case ActionGitStatus, ActionGitBranch, ActionGitBranches, ActionGitCheckout,
		ActionGitCommit, ActionGitAdd, ActionGitDiff, ActionGitLog,
		ActionGitPull, ActionGitPush, ActionGitStash, ActionGitBlame:
		return true
	}
	return false
}

// HandleAction processes a git action.
func (h *GitHandler) HandleAction(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	switch action.Name {
	case ActionGitStatus:
		return h.status(ctx)
	case ActionGitBranch:
		return h.branch(ctx)
	case ActionGitBranches:
		return h.branches(ctx)
	case ActionGitCheckout:
		return h.checkout(action, ctx)
	case ActionGitCommit:
		return h.commit(action, ctx)
	case ActionGitAdd:
		return h.add(action, ctx)
	case ActionGitDiff:
		return h.diff(action, ctx)
	case ActionGitLog:
		return h.log(action, ctx)
	case ActionGitPull:
		return h.pull(ctx)
	case ActionGitPush:
		return h.push(ctx)
	case ActionGitStash:
		return h.stash(action, ctx)
	case ActionGitBlame:
		return h.blame(action, ctx)
	default:
		return handler.Errorf("unknown git action: %s", action.Name)
	}
}

// getManager returns the git manager from handler or context.
func (h *GitHandler) getManager(ctx *execctx.ExecutionContext) GitManager {
	if h.manager != nil {
		return h.manager
	}
	if v, ok := ctx.GetData(gitManagerKey); ok {
		if gm, ok := v.(GitManager); ok {
			return gm
		}
	}
	return nil
}

func (h *GitHandler) status(ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.status: no git manager available")
	}

	status, err := gm.Status()
	if err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("branch", status.Branch).
		WithData("ahead", status.Ahead).
		WithData("behind", status.Behind).
		WithData("staged", status.Staged).
		WithData("unstaged", status.Unstaged).
		WithData("untracked", status.Untracked).
		WithData("conflicts", status.Conflicts).
		WithData("clean", !status.HasChanges()).
		WithMessage(formatStatusMessage(status))
}

func (h *GitHandler) branch(ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.branch: no git manager available")
	}

	branch, err := gm.CurrentBranch()
	if err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("branch", branch).
		WithMessage("On branch " + branch)
}

func (h *GitHandler) branches(ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.branches: no git manager available")
	}

	refs, err := gm.Branches()
	if err != nil {
		return handler.Error(err)
	}

	names := make([]string, len(refs))
	for i, ref := range refs {
		names[i] = ref.ShortName
	}

	return handler.Success().
		WithData("branches", names).
		WithMessage(formatBranchList(names))
}

func (h *GitHandler) checkout(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.checkout: no git manager available")
	}

	branch := action.Args.GetString("branch")
	if branch == "" {
		return handler.Errorf("git.checkout: branch required")
	}

	if err := gm.Checkout(branch); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithMessage("Switched to branch " + branch).
		WithRedraw()
}

func (h *GitHandler) commit(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.commit: no git manager available")
	}

	message := action.Args.GetString("message")
	if message == "" {
		return handler.Errorf("git.commit: message required")
	}

	opts := git.CommitOptions{
		Amend:      action.Args.GetBool("amend"),
		AllowEmpty: action.Args.GetBool("allowEmpty"),
		SignOff:    action.Args.GetBool("signoff"),
	}

	commit, err := gm.Commit(message, opts)
	if err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("hash", commit.Hash).
		WithData("shortHash", commit.ShortHash).
		WithMessage("[" + commit.ShortHash + "] " + message)
}

func (h *GitHandler) add(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.add: no git manager available")
	}

	// Check for --all flag
	if action.Args.GetBool("all") {
		if err := gm.AddAll(); err != nil {
			return handler.Error(err)
		}
		return handler.Success().WithMessage("Staged all changes")
	}

	// Get paths to add - try single path first
	var paths []string
	if path := action.Args.GetString("path"); path != "" {
		paths = []string{path}
	} else if pathsVal, ok := action.Args.Get("paths"); ok {
		if ps, ok := pathsVal.([]string); ok {
			paths = ps
		}
	}

	if len(paths) == 0 {
		return handler.Errorf("git.add: paths required")
	}

	if err := gm.Add(paths...); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("staged", paths).
		WithMessage("Staged " + itoa(len(paths)) + " file(s)")
}

func (h *GitHandler) diff(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.diff: no git manager available")
	}

	staged := action.Args.GetBool("staged")
	path := action.Args.GetString("path")

	var diff string
	var err error

	if path != "" {
		diff, err = gm.DiffFile(path, staged)
	} else {
		diff, err = gm.Diff(staged)
	}

	if err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("diff", diff).
		WithData("staged", staged).
		WithMessage(diff)
}

func (h *GitHandler) log(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.log: no git manager available")
	}

	n := action.Args.GetInt("count")
	if n <= 0 {
		n = 10
	}

	commits, err := gm.Log(n)
	if err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("commits", commits).
		WithMessage(formatCommitLog(commits))
}

func (h *GitHandler) pull(ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.pull: no git manager available")
	}

	if err := gm.Pull(); err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithMessage("Pulled from remote").
		WithRedraw()
}

func (h *GitHandler) push(ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.push: no git manager available")
	}

	if err := gm.Push(); err != nil {
		return handler.Error(err)
	}

	return handler.Success().WithMessage("Pushed to remote")
}

func (h *GitHandler) stash(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.stash: no git manager available")
	}

	// Check for pop operation
	if action.Args.GetBool("pop") {
		if err := gm.StashPop(); err != nil {
			return handler.Error(err)
		}
		return handler.Success().
			WithMessage("Popped stash").
			WithRedraw()
	}

	message := action.Args.GetString("message")
	if err := gm.Stash(message); err != nil {
		return handler.Error(err)
	}

	return handler.Success().WithMessage("Stashed changes")
}

func (h *GitHandler) blame(action input.Action, ctx *execctx.ExecutionContext) handler.Result {
	gm := h.getManager(ctx)
	if gm == nil {
		return handler.Errorf("git.blame: no git manager available")
	}

	path := action.Args.GetString("path")
	if path == "" {
		path = ctx.FilePath
	}
	if path == "" {
		return handler.Errorf("git.blame: path required")
	}

	lines, err := gm.Blame(path)
	if err != nil {
		return handler.Error(err)
	}

	return handler.Success().
		WithData("blame", lines).
		WithData("path", path)
}

// Helper functions

func formatStatusMessage(status *git.Status) string {
	if !status.HasChanges() {
		return "On branch " + status.Branch + " (clean)"
	}

	msg := "On branch " + status.Branch
	if status.Ahead > 0 {
		msg += " [ahead " + itoa(status.Ahead) + "]"
	}
	if status.Behind > 0 {
		msg += " [behind " + itoa(status.Behind) + "]"
	}
	if len(status.Staged) > 0 {
		msg += "\n" + itoa(len(status.Staged)) + " staged"
	}
	if len(status.Unstaged) > 0 {
		msg += "\n" + itoa(len(status.Unstaged)) + " modified"
	}
	if len(status.Untracked) > 0 {
		msg += "\n" + itoa(len(status.Untracked)) + " untracked"
	}
	if len(status.Conflicts) > 0 {
		msg += "\n" + itoa(len(status.Conflicts)) + " conflicts"
	}
	return msg
}

func formatBranchList(names []string) string {
	msg := ""
	for _, name := range names {
		msg += "  " + name + "\n"
	}
	return msg
}

func formatCommitLog(commits []*git.Commit) string {
	msg := ""
	for _, c := range commits {
		msg += c.ShortHash + " " + truncate(c.Message, 60) + "\n"
	}
	return msg
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

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
