# Code Review Remediation Plan

**Date:** 2025-12-08
**Review Source:** `review-results/review-report-20251208-100733.md`
**Total Issues Reported:** 828 (23 Critical, 140 High, 438 Medium, 204 Low, 23 Informational)

## Issue Analysis Summary

After thorough analysis, many reported issues are:
- **False positives** (syntax errors that don't exist - code compiles cleanly)
- **Duplicate reports** of the same underlying issue
- **Low consensus** (only 1/3 providers flagged them)

### False Positives Identified

The following "Critical" issues are **FALSE POSITIVES** (code compiles and works correctly):

| Issue # | File | Claim | Reality |
|---------|------|-------|---------|
| 2 | layer/merge.go:30 | cloneMap/cloneSlice undefined | Functions defined in layer/layer.go:117-149 |
| 11 | engine/errors.go:32 | Extra closing parenthesis | File has proper syntax |
| 14 | event/dispatch/errors.go:16 | Syntax error | File has proper syntax |
| 15 | integration/errors.go:24 | Syntax error | File has proper syntax |
| 16 | integration/git/errors.go:40 | Syntax error | File has proper syntax |
| 17 | plugin/errors.go:34 | Extra closing parenthesis | File has proper syntax |

---

## Valid Issues Requiring Action

### Priority 1: Security Issues (CRITICAL)

#### 1.1 Lua VM Thread Safety (Issue #23, #79, #80)
**Files:** `internal/plugin/api/command.go:236`, `internal/plugin/api/config.go:238`, `internal/plugin/api/event.go:196`
**Problem:** gopher-lua LState is accessed from multiple goroutines without synchronization. LState is not goroutine-safe.
**Risk:** Race conditions, crashes, undefined behavior
**Remediation:**
1. Create a Lua execution queue/channel for each plugin
2. All Lua calls must be serialized through this single-goroutine worker
3. Use message passing for async operations
4. Add integration tests for concurrent Lua access

#### 1.2 Lua Sandbox OS Module Leak (Issue #18)
**File:** `internal/plugin/lua/sandbox.go:126`
**Problem:** When granting CapabilityShell, the full gopher-lua `os` module is loaded, then only some functions are overridden. Other dangerous functions like `os.remove`, `os.rename`, `os.exit` remain accessible.
**Risk:** Sandbox escape, file system manipulation
**Remediation:**
1. Build a completely custom `os` table from scratch
2. Explicitly whitelist only safe functions
3. Never use `originalRequire` for security-sensitive modules
4. Add tests to verify dangerous functions are blocked

#### 1.3 Task Command Injection (Issue #10)
**File:** `internal/dispatcher/handlers/integration/task.go:139`
**Problem:** Task commands may be derived from user input and executed without sanitization.
**Risk:** Arbitrary code execution
**Remediation:**
1. Implement command whitelist/validation
2. Add sandboxing for task execution
3. Escape or quote shell arguments properly
4. Document security model for tasks

#### 1.4 Git Credential Helper Injection (Issue #73)
**File:** `internal/integration/git/auth.go:40`
**Problem:** `h.Helper` is passed directly to `git credential` command without validation.
**Risk:** Command injection
**Remediation:**
1. Validate/whitelist allowed credential helper values
2. Ensure helper is treated as literal argument
3. Use direct helper execution instead of string argument

#### 1.5 Sensitive Credential Handling (Issue #22)
**File:** `internal/integration/git/auth.go`
**Problem:** Passwords/credentials stored in memory as plain strings without protection.
**Risk:** Credential exposure in logs, stack traces, memory dumps
**Remediation:**
1. Use secure string type that clears on GC
2. Integrate with OS keychain where possible
3. Never log credential values
4. Zero-out sensitive memory when done

### Priority 2: Race Conditions (HIGH)

#### 2.1 AsyncDispatcher Stop/Enqueue Race (Issue #13)
**File:** `internal/event/dispatch/async.go:62`
**Problem:** Stop() closes the queue channel while Enqueue() may concurrently send to it.
**Risk:** Panic (send on closed channel)
**Remediation:**
1. Add coordination between Stop and Enqueue
2. Use running flag check before channel operations
3. Drain queue before closing
4. Add concurrent tests for Stop/Enqueue

#### 2.2 Line Cache Stats Race (Issue #19)
**File:** `internal/renderer/linecache/cache.go:105`
**Problem:** `hits`, `misses`, `evictions` counters are `uint64` but accessed with regular increment under mutex, while reads use RLock.
**Risk:** Data race, incorrect statistics
**Remediation:**
1. Change counters to `atomic.Uint64`
2. Use `Add(1)` for increments
3. Use `Load()` for reads
4. Remove mutex protection for these fields

#### 2.3 Sandbox Capabilities Map Race (Issue #82)
**File:** `internal/plugin/lua/sandbox.go:148`
**Problem:** capabilities map accessed without synchronization in Grant, Revoke, HasCapability.
**Risk:** Data race, inconsistent security checks
**Remediation:**
1. Add sync.RWMutex for capabilities map
2. Acquire locks in all accessor methods
3. Or document single-threaded access requirement

#### 2.4 Macro Recorder Not Thread-Safe (Issue #41)
**File:** `internal/dispatcher/handlers/macro/macro.go:236`
**Problem:** DefaultMacroRecorder has no synchronization around fields.
**Risk:** Data races in concurrent access
**Remediation:**
1. Add sync.RWMutex to DefaultMacroRecorder
2. Protect all field access
3. Or document single-goroutine usage requirement

### Priority 3: Code Duplication (MEDIUM)

#### 3.1 Map Path Utilities Duplication (Issues #1, #3, #4, #5, #6, #8)
**Files:** Multiple files in config package
**Problem:** `DeepMerge`, `GetByPath`, `SetByPath`, `Clone`, etc. duplicated across:
- `layer/merge.go`
- `loader/env.go`
- `loader/toml.go`
- `config/migration.go`
- `config/registry/accessor.go`

**Remediation:**
1. Create `internal/config/maputil` package
2. Consolidate all path-based map operations
3. Ensure consistent deep-clone behavior
4. Update all imports

#### 3.2 isWordChar Duplication (Issues #33, #35, #36)
**Files:** `handlers/completion/completion.go`, `handlers/editor/delete.go`, `handlers/editor/yank.go`
**Problem:** ASCII-only word character detection duplicated and inconsistent with `unicode` package usage elsewhere.
**Remediation:**
1. Create shared `internal/util/unicode.go` with `IsWordChar` using `unicode.IsLetter`, `unicode.IsDigit`
2. Replace all ASCII-based implementations
3. Ensure consistent behavior across codebase

#### 3.3 Glob Matching Duplication (Issues #98, #99)
**Files:** `project/watcher/ignore.go`, `project/workspace/config.go`
**Problem:** Complex custom glob matching reimplemented in multiple places.
**Remediation:**
1. Adopt `github.com/gobwas/glob` or similar library
2. Remove custom implementations
3. Improve test coverage for edge cases

### Priority 4: Performance Issues (MEDIUM)

#### 4.1 Myers Diff Memory Usage (Issue #12)
**File:** `internal/engine/tracking/diff.go:132`
**Problem:** Full V vector copied each iteration, O(n+m) memory per iteration for large inputs.
**Remediation:**
1. Add input size limits
2. Implement memory-efficient variant
3. Fall back to heuristic diff for very large inputs
4. Return clear error when limits exceeded

#### 4.2 Bubble Sort in Latency Stats (Issue #66)
**File:** `internal/input/metrics.go:236`
**Problem:** O(n²) bubble sort for 1000 element slice.
**Remediation:**
1. Replace with `sort.Slice` (O(n log n))

#### 4.3 Rope Reverse Iterator Inefficiency (Issue #53)
**File:** `internal/engine/rope/iter.go:218`
**Problem:** ReverseRuneIterator uses O(log n) per byte/rune operations.
**Remediation:**
1. Implement specialized reverse cursor with amortized O(1) backward movement
2. Use ReverseChunkIterator for better locality

#### 4.4 Position Converter Recreation (Issue #78)
**File:** `internal/lsp/position.go:256`
**Problem:** PositionConverter recreated and line index rebuilt on every conversion call.
**Remediation:**
1. Cache PositionConverter per document in lsp.Provider
2. Update only on document changes

### Priority 5: Input Validation (MEDIUM)

#### 5.1 MaxFPS Division by Zero (Issue #102)
**File:** `internal/renderer/renderer.go:64`
**Problem:** No validation for zero MaxFPS causing division panic.
**Remediation:**
1. Validate MaxFPS >= 1 in New()
2. Clamp to reasonable default

#### 5.2 Viewport Size Validation (Issues #105, #106)
**File:** `internal/renderer/viewport/viewport.go:40, 84`
**Problem:** No validation for negative/zero width/height, causes underflow.
**Remediation:**
1. Validate dimensions in NewViewport and Resize
2. Guard uint32 conversion in bottomLine()

#### 5.3 ReDoS in Regex Operations (Issues #89, #97)
**Files:** `project/index/contentindex.go:124`, `project/search/search.go:210`
**Problem:** User-supplied regex compiled/executed without safeguards.
**Remediation:**
1. Add pattern length/complexity limits
2. Use timeout/cancellable context for regex ops
3. Pre-check for exponential backtracking patterns

### Priority 6: Best Practices (LOW)

#### 6.1 Atomic File Save (Issue #25)
**File:** `internal/app/lifecycle.go:11`
**Remediation:** Write to temp file, fsync, then rename

#### 6.2 Hook Pointer Storage (Issue #20)
**File:** `internal/input/hooks.go:62`
**Remediation:** Store heap-allocated pointers or copies instead of slice element addresses

#### 6.3 Binding Pointer Return (Issue #21)
**File:** `internal/input/keymap/registry.go:170`
**Remediation:** Return copy of Binding, not pointer to local slice element

---

## Implementation Phases

### Phase 1: Critical Security (Immediate)
- [ ] 1.1 Lua VM thread safety
- [ ] 1.2 Lua sandbox OS module
- [ ] 1.3 Task command sanitization
- [ ] 1.4 Git credential validation
- [ ] 1.5 Secure credential handling

### Phase 2: Race Conditions (Week 1)
- [ ] 2.1 AsyncDispatcher race
- [ ] 2.2 Line cache stats atomics
- [ ] 2.3 Sandbox capabilities mutex
- [ ] 2.4 Macro recorder mutex

### Phase 3: Code Quality (Week 2)
- [ ] 3.1 Map utility consolidation
- [ ] 3.2 Unicode word detection
- [ ] 3.3 Glob library adoption

### Phase 4: Performance (Week 3)
- [ ] 4.1 Diff limits
- [ ] 4.2 Sort algorithm
- [ ] 4.3 Rope iterator
- [ ] 4.4 Position converter caching

### Phase 5: Validation & Polish (Week 4)
- [ ] 5.1-5.3 Input validation
- [ ] 6.1-6.3 Best practices fixes

---

## Notes

- Issues with only 33% provider consensus require verification before fixing
- Many "critical" syntax errors are false positives - code compiles successfully
- Focus on security issues first as they have the highest risk
- Add tests for each fix to prevent regression
