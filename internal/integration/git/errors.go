package git

import "errors"

// Error types for git operations.
var (
	// ErrNotRepository indicates the path is not a git repository.
	ErrNotRepository = errors.New("not a git repository")

	// ErrRepositoryNotFound indicates no repository was found.
	ErrRepositoryNotFound = errors.New("repository not found")

	// ErrNoHead indicates the repository has no HEAD (empty repository).
	ErrNoHead = errors.New("repository has no HEAD")

	// ErrNothingToCommit indicates there are no staged changes to commit.
	ErrNothingToCommit = errors.New("nothing to commit")

	// ErrPathNotFound indicates the specified path was not found.
	ErrPathNotFound = errors.New("path not found")

	// ErrConflict indicates a merge conflict exists.
	ErrConflict = errors.New("merge conflict")

	// ErrDetachedHead indicates the repository is in detached HEAD state.
	ErrDetachedHead = errors.New("detached HEAD state")

	// ErrBranchExists indicates the branch already exists.
	ErrBranchExists = errors.New("branch already exists")

	// ErrBranchNotFound indicates the branch was not found.
	ErrBranchNotFound = errors.New("branch not found")

	// ErrRemoteNotFound indicates the remote was not found.
	ErrRemoteNotFound = errors.New("remote not found")

	// ErrAuthenticationFailed indicates authentication failed.
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrDirtyWorkingTree indicates uncommitted changes exist.
	ErrDirtyWorkingTree = errors.New("dirty working tree")

	// ErrManagerClosed indicates the manager has been closed.
	ErrManagerClosed = errors.New("manager closed")
)
