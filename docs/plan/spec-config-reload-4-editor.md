# Spec: config-reload-4-editor

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/config/editor/model_commands.go` — cmdCommit() flow
4. `internal/config/editor/editor.go` — Editor.Save()

**Parent spec:** `spec-reload-lifecycle-tests.md` (umbrella)
**Depends on:** `spec-config-reload-3-sighup.md` (SIGHUP reload must work)

## Task

Wire the editor's `commit` command to trigger a config reload after saving. Currently `cmdCommit()` validates YANG and calls `editor.Save()` — the running daemon is never notified. After this change, commit will save then trigger the reload pipeline (YANG validate → plugin verify → plugin apply with diff).

**Approach:** Editor saves to disk, then sends a reload signal/command to the running daemon. This reuses the SIGHUP pipeline from sub-spec 3 entirely.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — editor/daemon separation

### Source Files (MUST read)
- [ ] `internal/config/editor/model_commands.go` — cmdCommit() at line 426, cmdCommitConfirm()
- [ ] `internal/config/editor/editor.go` — Editor struct, Save(), WorkingContent()
- [ ] `internal/config/editor/model.go` — Model struct, how editor connects to validator

**Key insights:**
- Editor runs as a separate process (`ze config edit`) or in-process
- Editor does not have direct access to the Server or reactor
- The simplest approach: after Save(), send SIGHUP to the daemon process
- Alternative: use socket-based command dispatch (like `ze bgp daemon reload`)
- PID file or socket path needed for editor → daemon communication

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/config/editor/model_commands.go` — cmdCommit() validates YANG, calls editor.Save(), returns success
- [ ] `internal/config/editor/editor.go` — Save() writes to file, clears dirty flag, no daemon notification

**Behavior to preserve:**
- YANG validation before save unchanged
- File save mechanics unchanged
- Editor can still work without a running daemon (standalone validation mode)

**Behavior to change:**
- After successful save, attempt to notify the running daemon of config change
- If daemon notification fails (daemon not running), log warning but don't fail the commit
- Commit output includes reload result (success/failure)

## Data Flow (MANDATORY)

### Entry Point
- User types `commit` in editor → `cmdCommit()` in model_commands.go

### Transformation Path
1. YANG validate working content (existing)
2. If validation fails → return error (existing)
3. Call `editor.Save()` to write config to disk (existing)
4. **New:** Attempt to notify daemon of config change
5. If daemon is running → trigger reload (via SIGHUP or command)
6. Reload goes through coordinator pipeline (sub-spec 3)
7. Return reload result to user (or warning if daemon not running)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Editor → Daemon | SIGHUP signal or socket command | [ ] |
| Daemon → Coordinator | ReloadFromDisk() (sub-spec 3) | [ ] |

### Integration Points
- `cmdCommit()` in model_commands.go — add post-save notification
- PID file (from spec-signal-command) or socket path — locate running daemon
- `syscall.Kill(pid, syscall.SIGHUP)` — signal-based notification
- OR `ze bgp daemon reload` — command-based notification via CLI dispatch

### Architectural Verification
- [ ] No bypassed layers (editor notifies daemon, daemon runs coordinator)
- [ ] No unintended coupling (editor only sends signal, doesn't import server/reactor)
- [ ] No duplicated functionality (reuses SIGHUP pipeline from sub-spec 3)
- [ ] Zero-copy preserved where applicable (N/A)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCommitTriggersReload` | `internal/config/editor/model_commands_test.go` | After save, reload notification attempted | |
| `TestCommitReloadFailsGracefully` | `internal/config/editor/model_commands_test.go` | Daemon not running → commit succeeds with warning | |
| `TestCommitValidationFailsNoReload` | `internal/config/editor/model_commands_test.go` | YANG validation fails → no save, no reload | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `editor-commit-reload` | `test/reload/editor-commit.ci` | Edit config in editor, commit, verify daemon applies changes | |

## Files to Modify
- `internal/config/editor/model_commands.go` — add post-save reload notification to cmdCommit()
- `internal/config/editor/editor.go` — add optional ReloadNotifier interface field

## Files to Create
- `test/reload/editor-commit.ci` — functional test for editor commit → reload

## Implementation Steps

### Step 1: Design notification mechanism
Decide between SIGHUP (needs PID file) vs socket command (needs socket path). SIGHUP is simpler and already works via sub-spec 3. Socket command is more robust but requires connection setup.

### Step 2: Write commit reload tests
Test that commit attempts notification after save. Test graceful handling when daemon is not running.

### Step 3: Add ReloadNotifier to Editor
Add an optional interface field to Editor or Model that can notify the daemon. When nil (standalone mode), skip notification.

### Step 4: Implement notification in cmdCommit
After successful `editor.Save()`, call ReloadNotifier if set. Report result to user.

### Step 5: Write functional test
Create .ci test that starts daemon, edits config, commits, verifies changes applied.

### Step 6: Verify
Run `make lint && make test && make functional` — all tests pass.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Post-save daemon notification | | | |
| Graceful handling when daemon not running | | | |
| Commit output includes reload result | | | |
| ReloadNotifier interface | | | |
| Functional test: editor-commit | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestCommitTriggersReload | | | |
| TestCommitReloadFailsGracefully | | | |
| TestCommitValidationFailsNoReload | | | |
| editor-commit.ci | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/config/editor/model_commands.go` | | |
| `internal/config/editor/editor.go` | | |
| `test/reload/editor-commit.ci` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Design
- [x] No premature abstraction (simple notification, not a full commit protocol)
- [x] No speculative features (no verify-before-save — that's a future enhancement)
- [x] Single responsibility (editor saves, daemon reloads, coordinator orchestrates)
- [x] Explicit behavior (reload result reported to user)
- [x] Minimal coupling (editor sends signal, doesn't import server/reactor)
- [x] Next-developer test (follows existing editor command patterns)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes
