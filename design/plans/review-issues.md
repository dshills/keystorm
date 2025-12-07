# Integration Layer Review Issues

Identified during pre-commit review on 2025-12-07.

---

## Critical Issues

### 1. PTY Terminal Startup Bug
**File:** `internal/integration/terminal/pty.go`
**Category:** Bug

`newPTYTerminal` calls `pty.Start(cmd)` but no `exec.Cmd` is constructed or passed into the function. This will not compile and the terminal process will never be started.

**Suggestion:** Construct the `exec.Cmd` from `TerminalOptions` (`opts.Shell`, `opts.Args`) and any required environment/working directory before calling `pty.Start(cmd)`. Ensure you check and return errors from creating pipes and starting the process.

---

### 2. Plaintext Credential Storage
**File:** `internal/integration/git/auth.go`
**Category:** Security

`authManager` stores credentials (Username, Password, SSHKey) in plaintext in memory and caches them in `credentialCache` without any protection or expiration guarantees. `getHTTPAuth` returns `&http.BasicAuth` with plaintext password. This increases the risk of leaking secrets (in memory dumps, logs, or when publishing events).

**Suggestion:** Avoid storing raw passwords/keys in long-lived in-memory caches. Use OS credential helpers, encrypted storage, or ephemeral tokens where possible. Limit lifetime of cached credentials, zero memory after use when feasible, and avoid publishing secrets in events/logs. Secure the `credentialCache` initialization with proper zero value creation and consider using a dedicated secrets manager abstraction.

---

## High Severity Issues

### 3. PTY Read Loop Resource Leak
**File:** `internal/integration/terminal/pty.go`
**Category:** Bug

`ptyTerminal.readLoop` returns on any read error but does not close the PTY, mark the terminal as done, set exit code, or publish terminal closed events. This causes resource leaks and the rest of the system may never learn the terminal exited.

**Suggestion:** On read error, set `t.exitCode` appropriately, close `t.pty`, close the done channel (selectively), and publish a `terminal.closed` event. Also log errors for diagnostics.

---

### 4. Ignored Pipe Errors in Session Launch
**File:** `internal/integration/debug/session.go`
**Category:** Bug

Many functions ignore errors returned by important calls: e.g., in `session.Launch` the results of `cmd.StdinPipe()`, `cmd.StdoutPipe()` (and potential `cmd.StderrPipe()`) are ignored and not checked for errors.

**Suggestion:** Always check and handle errors returned by io pipe creation, `exec.Command()`, `cmd.Start()`, and other system calls. If an error occurs, perform appropriate cleanup (kill process if started) and return a meaningful error.

---

### 5. SSH Key Passphrase Handling
**File:** `internal/integration/git/auth.go`
**Category:** Bug

`authManager.getSSHAuth` attempts to use `ssh.NewPublicKeysFromFile(keyPath, "")` for private keys without any handling for encrypted keys (passphrase), and silently falls through if the key can't be loaded. Encrypted keys or other key formats will fail unexpectedly.

**Suggestion:** Handle passphrases or prompt the user via a secure API when keys are encrypted. Consider supporting ssh-agent, various key formats, and explicit error messages. Do not assume empty passphrase works for all keys.

---

### 6. Stdout/Stderr Stream Merging Issue
**File:** `internal/integration/task/executor.go`
**Category:** Bug

`executor.processOutput` uses `io.MultiReader(run.process.Stdout, run.process.Stderr)` and a single Scanner to read output. `io.MultiReader` concatenates streams sequentially (stdout then stderr), which will not preserve interleaving timestamps/order and may block waiting for first stream EOF before reading the second.

**Suggestion:** Read stdout and stderr concurrently in separate goroutines and merge lines in a channel if ordering is important, or tag emitted events with stream type (stdout/stderr). This preserves ordering semantics and avoids blocking on one stream.

---

### 7. Event Data Sanitization
**File:** Multiple integration files
**Category:** Security

Multiple places publish integration events with free-form maps (`map[string]any`) that include strings derived from external sources (terminal output, task output, commit messages). If any event consumers log or render these without sanitization, it can lead to injection or leaking of secrets (e.g., terminal output containing passwords).

**Suggestion:** Sanitize or redact sensitive values before publishing events. Define a schema for events and avoid including untrusted data unless necessary. Provide a clear security policy for event consumers.

---

## Medium Severity Issues

### 8. Scanner Buffer Size Limit
**File:** `internal/integration/task/executor.go`
**Category:** Bug

`bufio.Scanner` used in `processOutput` uses its default max token size (~64KiB). Long output lines (e.g., minified js or very long compiler errors) can cause scanner failures.

**Suggestion:** Use `bufio.Reader` with `ReadLine` or create a Scanner with a larger buffer via `Scanner.Buffer` to handle longer lines safely. Also check `scanner.Err()` after scanning.

---

### 9. DAP Client Pending Request Leak
**File:** `internal/integration/debug/dap_client.go`
**Category:** Bug

`sendRequest` in `dapClient` creates a pending response channel and deletes it in a defer. If the transport fails and `readLoop` exits (or the connection is closed), pending requests may never receive a response and callers will block until their timeout — the pending entries are removed only after `sendRequest` returns, but `readLoop` does not notify waiting callers of transport failure.

**Suggestion:** On `transport.Receive()` error, `readLoop` should close/notify all pending response channels with an error or close them so waiting `sendRequest` callers can unblock immediately. Consider a dedicated error channel and closing behavior to avoid leaking pending requests.

---

### 10. DAP Event Handler Goroutine Storm
**File:** `internal/integration/debug/dap_client.go`
**Category:** Bug

`dapClient.readLoop` pushes responses into pending channels without verifying channel capacity or existence after acquiring the lock. Handlers invoked from `handleEvent` are launched as goroutines without bounded concurrency which may lead to large numbers of goroutines on event storms.

**Suggestion:** When dispatching responses or events, handle the case where no recipient exists (log or drop). For event handlers, consider using a worker pool or bounded goroutine approach and handle panics inside handlers to avoid unexpected crashes.

---

### 11. Debug Session Stderr Not Captured
**File:** `internal/integration/debug/session.go`
**Category:** Bug

`session.Launch` starts a debug adapter process but doesn't capture or forward stderr. It also doesn't guarantee child process cleanup if subsequent initialization steps fail (e.g., `client.Initialize` returns error but `cmd.Process.Kill` is called only in one error path).

**Suggestion:** Wire `cmd.Stderr` (e.g., to a buffer or to logs) and ensure cleanup happens for all error paths (use defer with a started flag or context). Check errors from `Std{in,out,err}Pipe` calls and handle them.

---

### 12. Incorrect Diff Hunk Line Counts
**File:** `internal/integration/git/repository.go`
**Category:** Bug

`repository.generateFileDiff` computes `OldLines` and `NewLines` using `FromLine + len(hunk.Lines)` which results in incorrect hunk line counts (`OldLines` and `NewLines` should represent number of lines, not end line numbers).

**Suggestion:** Compute `OldLines = len(hunk.LinesOld)` (or the correct number of lines in the old hunk) and `NewLines` accordingly. Use the diff library's hunk metadata carefully and validate the fields' semantics.

---

### 13. Missing Old File Handling in Diff
**File:** `internal/integration/git/repository.go`
**Category:** Bug

`generateFileDiff` swallows errors from `r.getOldContent` returning `object.ErrFileNotFound` and then continues. The branch treats other errors returned from `getOldContent` as fatal but doesn't handle the missing old file case consistently when detecting added/renamed files.

**Suggestion:** Handle `ErrFileNotFound` explicitly by treating `oldContent` as empty (file added). Ensure all relevant error cases are logged or returned as appropriate; avoid swallowing unexpected errors silently.

---

### 14. Uninitialized Maps May Panic
**File:** Multiple integration files
**Category:** Best Practice

Many caches/maps (e.g., `authManager.credentialCache`, `Supervisor.processes`, `executor.matchers`) are used but initialization is not shown. Using nil maps will panic on writes. Several components rely on maps and channels being non-nil without explicit initialization.

**Suggestion:** Initialize maps (`make(map[...])`) and channels in constructors (`NewXYZ`) and document ownership. Add defensive nil checks where appropriate.

---

### 15. Git Provider Workspace Context
**File:** `internal/integration/git/provider.go`
**Category:** Best Practice

`provider GitProvider.Status` uses `gm.Discover(".")` (current working directory) rather than the execution/workspace context (like `ctx.WorkspaceRoot` used elsewhere). This can return unexpected repositories when invoked from the editor with a different workspace root.

**Suggestion:** Accept a context or workspace root parameter (or use manager-level workspace selection) instead of assuming `"."`. Propagate workspace information from the dispatcher/execution context to provider handlers.

---

### 16. Git Status Cache TTL
**File:** `internal/integration/git/repository.go`
**Category:** Performance

`repository.Status` uses a caching TTL of 100ms which may be too small to meaningfully reduce expensive filesystem/git operations, yet high enough to introduce complexity. Conversely, frequent invalidation may cause unnecessary Git work.

**Suggestion:** Parameterize the cache duration and tie invalidation to repository events (fsnotify or hooks) when possible. Allow tuning via configuration and benchmark the trade-offs.

---

### 17. Task Executor Background Semantics
**File:** `internal/integration/task/executor.go`
**Category:** Bug

`task.executor.Run` launches the process via `supervisor.Start` and then immediately sets `run.state = TaskStateRunning` and returns. If `opts.Background` is false, `waitForCompletion` is started in a goroutine but there is no guarantee the caller will wait; also, when `opts.Background` is true the code doesn't wait at all; this behavior may be surprising to callers.

**Suggestion:** Document semantics clearly for Background option. If `Background==false` the function should either block until completion or explicitly return a `TaskRun` that the caller can `Wait()` on. Ensure `Wait()` uses the underlying process Done channel so callers can coordinate.

---

## Low Severity / Informational Issues

### 18. Makefile Parser Limitations
**File:** `internal/integration/task/makefile.go`
**Category:** Style

Makefile target regex in `makefileSource.Parse` is simplistic and will miss many valid Make targets (targets with dots, slashes, non-ASCII, or targets containing '%' or pattern rules). It also attempts to find description only in the immediate previous line as a comment.

**Suggestion:** Document that the Makefile parser is heuristic and consider using a more robust parser (or conservative heuristics). Expand regex to allow common target names, handle multi-line comments, and skip rule recipe lines.

---

### 19. Problem Matcher Regex Panic
**File:** `internal/integration/task/matcher.go`
**Category:** Best Practice

Problem matchers compile regexes (`regexp.MustCompile`) or `regexp.Compile` implicitly in `newProblemMatcher` without handling pattern compile errors; if config is user-provided, `MustCompile` can panic.

**Suggestion:** Use `regexp.Compile` and return errors to callers for user-provided patterns. Validate matcher config on registration rather than allowing panics at runtime.

---

### 20. Shell Integration URL Format
**File:** `internal/integration/terminal/shell.go`
**Category:** Info

`ShellIntegration bashIntegrationScript` builds an OSC 7 `file://` URL using `printf '\033]7;file://%s%s\033\\' "$HOSTNAME" "$PWD"` which concatenates hostname and path without a slash between them; parsing may fail or produce non-standard file URLs. `parseFileURL` must handle this format explicitly.

**Suggestion:** Construct file URLs as `file://<hostname>/<path>` (ensure leading slash) or use only the path if hostname isn't necessary. Update `parseFileURL` to robustly parse the exact emitted sequence, or change the script to emit a simpler, unambiguous format.

---

## Summary

| Severity | Count |
|----------|-------|
| Critical | 2 |
| High | 5 |
| Medium | 10 |
| Low/Info | 3 |
| **Total** | **20** |

These issues should be addressed in a follow-up effort focused on hardening the integration layer.
