// Package git provides Git integration for the Keystorm editor.
//
// This package implements core Git operations including repository management,
// working tree operations (stage, unstage, discard), commit creation, and
// status tracking with caching for performance.
//
// # Architecture
//
// The package is organized around these core types:
//
//   - Manager: Entry point for Git operations, discovers and opens repositories
//   - Repository: Represents a Git repository with all operations
//   - Status: Working tree status with staged, unstaged, and untracked files
//
// # Usage
//
// Create a manager and discover/open repositories:
//
//	mgr := git.NewManager(git.ManagerConfig{
//	    EventBus: eventBus,
//	})
//
//	// Discover repository from any path within it
//	repo, err := mgr.Discover("/path/to/project/src")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get working tree status
//	status, err := repo.Status()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Branch: %s\n", status.Branch)
//	fmt.Printf("Modified: %d files\n", len(status.Unstaged))
//
// # Working Tree Operations
//
// Stage and commit changes:
//
//	// Stage specific files
//	err = repo.Stage("file1.go", "file2.go")
//
//	// Stage all changes
//	err = repo.StageAll()
//
//	// Create commit
//	commit, err := repo.Commit("Add new feature", git.CommitOptions{})
//
//	// Unstage files
//	err = repo.Unstage("file1.go")
//
//	// Discard changes
//	err = repo.Discard("file1.go")
//
// # Status Caching
//
// Status queries are cached for performance. The cache is automatically
// invalidated when operations modify the working tree:
//
//	// First call queries Git
//	status1, _ := repo.Status()
//
//	// Subsequent calls use cache (within TTL)
//	status2, _ := repo.Status()
//
//	// Staging invalidates cache
//	repo.Stage("file.go")
//
//	// Next status call queries Git again
//	status3, _ := repo.Status()
//
// # Events
//
// The package publishes events through the EventBus:
//
//   - git.status.changed: Working tree status changed
//   - git.commit.created: New commit created
//   - git.branch.changed: Current branch changed
//
// # Thread Safety
//
// All operations are thread-safe. The package uses sync.RWMutex to protect
// shared state and prevent race conditions during concurrent access.
package git
