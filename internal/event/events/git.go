package events

import (
	"time"

	"github.com/dshills/keystorm/internal/event/topic"
)

// Git event topics.
const (
	// TopicGitStatusChanged is published when repository status changes.
	TopicGitStatusChanged topic.Topic = "git.status.changed"

	// TopicGitBranchChanged is published when the current branch changes.
	TopicGitBranchChanged topic.Topic = "git.branch.changed"

	// TopicGitCommitCreated is published when a commit is made.
	TopicGitCommitCreated topic.Topic = "git.commit.created"

	// TopicGitConflictDetected is published when merge conflicts are found.
	TopicGitConflictDetected topic.Topic = "git.conflict.detected"

	// TopicGitConflictResolved is published when conflicts are resolved.
	TopicGitConflictResolved topic.Topic = "git.conflict.resolved"

	// TopicGitStashCreated is published when a stash is created.
	TopicGitStashCreated topic.Topic = "git.stash.created"

	// TopicGitStashApplied is published when a stash is applied.
	TopicGitStashApplied topic.Topic = "git.stash.applied"

	// TopicGitStashDropped is published when a stash is dropped.
	TopicGitStashDropped topic.Topic = "git.stash.dropped"

	// TopicGitFetchCompleted is published when a fetch completes.
	TopicGitFetchCompleted topic.Topic = "git.fetch.completed"

	// TopicGitPullCompleted is published when a pull completes.
	TopicGitPullCompleted topic.Topic = "git.pull.completed"

	// TopicGitPushCompleted is published when a push completes.
	TopicGitPushCompleted topic.Topic = "git.push.completed"

	// TopicGitMergeCompleted is published when a merge completes.
	TopicGitMergeCompleted topic.Topic = "git.merge.completed"

	// TopicGitRebaseCompleted is published when a rebase completes.
	TopicGitRebaseCompleted topic.Topic = "git.rebase.completed"

	// TopicGitTagCreated is published when a tag is created.
	TopicGitTagCreated topic.Topic = "git.tag.created"

	// TopicGitRemoteAdded is published when a remote is added.
	TopicGitRemoteAdded topic.Topic = "git.remote.added"

	// TopicGitRemoteRemoved is published when a remote is removed.
	TopicGitRemoteRemoved topic.Topic = "git.remote.removed"

	// TopicGitOperationStarted is published when a git operation starts.
	TopicGitOperationStarted topic.Topic = "git.operation.started"

	// TopicGitOperationProgress is published with git operation progress.
	TopicGitOperationProgress topic.Topic = "git.operation.progress"

	// TopicGitOperationFailed is published when a git operation fails.
	TopicGitOperationFailed topic.Topic = "git.operation.failed"
)

// GitFileStatus represents the status of a file in git.
type GitFileStatus string

// Git file statuses.
const (
	GitStatusUntracked GitFileStatus = "untracked"
	GitStatusModified  GitFileStatus = "modified"
	GitStatusAdded     GitFileStatus = "added"
	GitStatusDeleted   GitFileStatus = "deleted"
	GitStatusRenamed   GitFileStatus = "renamed"
	GitStatusCopied    GitFileStatus = "copied"
	GitStatusIgnored   GitFileStatus = "ignored"
	GitStatusConflict  GitFileStatus = "conflict"
)

// GitFileChange represents a changed file in git.
type GitFileChange struct {
	// Path is the file path.
	Path string

	// Status is the file status.
	Status GitFileStatus

	// OldPath is the previous path for renamed files.
	OldPath string

	// Staged indicates if the change is staged.
	Staged bool
}

// GitStatusChanged is published when repository status changes.
type GitStatusChanged struct {
	// Root is the repository root directory.
	Root string

	// Branch is the current branch name.
	Branch string

	// Ahead is the number of commits ahead of remote.
	Ahead int

	// Behind is the number of commits behind remote.
	Behind int

	// Staged lists staged files.
	Staged []GitFileChange

	// Unstaged lists unstaged changes.
	Unstaged []GitFileChange

	// Untracked lists untracked files.
	Untracked []string

	// Conflicted lists files with conflicts.
	Conflicted []string

	// IsClean indicates if working tree is clean.
	IsClean bool
}

// GitBranchChanged is published when the current branch changes.
type GitBranchChanged struct {
	// Root is the repository root directory.
	Root string

	// OldBranch was the previous branch (empty if detached).
	OldBranch string

	// NewBranch is the new branch (empty if detached).
	NewBranch string

	// IsDetached indicates if HEAD is detached.
	IsDetached bool

	// HeadCommit is the HEAD commit hash.
	HeadCommit string
}

// GitCommitCreated is published when a commit is made.
type GitCommitCreated struct {
	// Root is the repository root directory.
	Root string

	// CommitHash is the full commit hash.
	CommitHash string

	// ShortHash is the abbreviated commit hash.
	ShortHash string

	// Message is the commit message.
	Message string

	// Author is the commit author.
	Author string

	// AuthorEmail is the author's email.
	AuthorEmail string

	// Timestamp is when the commit was created.
	Timestamp time.Time

	// FilesChanged is the number of files changed.
	FilesChanged int

	// Insertions is the number of lines added.
	Insertions int

	// Deletions is the number of lines deleted.
	Deletions int
}

// GitConflictDetected is published when merge conflicts are found.
type GitConflictDetected struct {
	// Root is the repository root directory.
	Root string

	// Files lists files with conflicts.
	Files []string

	// Operation is the operation that caused conflicts (merge, rebase, etc.).
	Operation string

	// Source is the source branch/commit.
	Source string

	// Target is the target branch.
	Target string
}

// GitConflictResolved is published when conflicts are resolved.
type GitConflictResolved struct {
	// Root is the repository root directory.
	Root string

	// File is the resolved file.
	File string

	// Resolution describes how it was resolved.
	Resolution string
}

// GitStashCreated is published when a stash is created.
type GitStashCreated struct {
	// Root is the repository root directory.
	Root string

	// StashRef is the stash reference (e.g., "stash@{0}").
	StashRef string

	// Message is the stash message.
	Message string

	// FilesStashed is the number of files stashed.
	FilesStashed int
}

// GitStashApplied is published when a stash is applied.
type GitStashApplied struct {
	// Root is the repository root directory.
	Root string

	// StashRef is the stash reference.
	StashRef string

	// Dropped indicates if the stash was dropped after applying.
	Dropped bool

	// HasConflicts indicates if applying caused conflicts.
	HasConflicts bool
}

// GitStashDropped is published when a stash is dropped.
type GitStashDropped struct {
	// Root is the repository root directory.
	Root string

	// StashRef is the dropped stash reference.
	StashRef string
}

// GitFetchCompleted is published when a fetch completes.
type GitFetchCompleted struct {
	// Root is the repository root directory.
	Root string

	// Remote is the remote that was fetched.
	Remote string

	// NewCommits is the number of new commits fetched.
	NewCommits int

	// UpdatedRefs lists updated references.
	UpdatedRefs []string
}

// GitPullCompleted is published when a pull completes.
type GitPullCompleted struct {
	// Root is the repository root directory.
	Root string

	// Remote is the remote pulled from.
	Remote string

	// Branch is the branch pulled.
	Branch string

	// CommitsBehind was the number of commits behind before pull.
	CommitsBehind int

	// FastForward indicates if it was a fast-forward.
	FastForward bool

	// HasConflicts indicates if pull caused conflicts.
	HasConflicts bool
}

// GitPushCompleted is published when a push completes.
type GitPushCompleted struct {
	// Root is the repository root directory.
	Root string

	// Remote is the remote pushed to.
	Remote string

	// Branch is the branch pushed.
	Branch string

	// CommitsPushed is the number of commits pushed.
	CommitsPushed int

	// ForcePush indicates if it was a force push.
	ForcePush bool
}

// GitMergeCompleted is published when a merge completes.
type GitMergeCompleted struct {
	// Root is the repository root directory.
	Root string

	// Source is the merged branch.
	Source string

	// Target is the target branch.
	Target string

	// MergeCommit is the merge commit hash.
	MergeCommit string

	// FastForward indicates if it was a fast-forward merge.
	FastForward bool
}

// GitRebaseCompleted is published when a rebase completes.
type GitRebaseCompleted struct {
	// Root is the repository root directory.
	Root string

	// Branch is the rebased branch.
	Branch string

	// Onto is the branch rebased onto.
	Onto string

	// CommitsRebased is the number of commits rebased.
	CommitsRebased int

	// WasAborted indicates if the rebase was aborted.
	WasAborted bool
}

// GitTagCreated is published when a tag is created.
type GitTagCreated struct {
	// Root is the repository root directory.
	Root string

	// TagName is the tag name.
	TagName string

	// CommitHash is the tagged commit.
	CommitHash string

	// Message is the tag message (for annotated tags).
	Message string

	// IsAnnotated indicates if it's an annotated tag.
	IsAnnotated bool
}

// GitRemoteAdded is published when a remote is added.
type GitRemoteAdded struct {
	// Root is the repository root directory.
	Root string

	// Name is the remote name.
	Name string

	// URL is the remote URL.
	URL string
}

// GitRemoteRemoved is published when a remote is removed.
type GitRemoteRemoved struct {
	// Root is the repository root directory.
	Root string

	// Name is the removed remote name.
	Name string
}

// GitOperationStarted is published when a git operation starts.
type GitOperationStarted struct {
	// Root is the repository root directory.
	Root string

	// Operation is the operation name (e.g., "fetch", "pull", "push").
	Operation string

	// Remote is the remote involved, if any.
	Remote string
}

// GitOperationProgress is published with git operation progress.
type GitOperationProgress struct {
	// Root is the repository root directory.
	Root string

	// Operation is the operation name.
	Operation string

	// Phase describes the current phase.
	Phase string

	// Current is the current progress value.
	Current int

	// Total is the total progress value.
	Total int

	// Message is a progress message.
	Message string
}

// GitOperationFailed is published when a git operation fails.
type GitOperationFailed struct {
	// Root is the repository root directory.
	Root string

	// Operation is the failed operation name.
	Operation string

	// ErrorMessage describes the failure.
	ErrorMessage string

	// ExitCode is the git exit code.
	ExitCode int
}
