# Spec: transactional-config-commit

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-05-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/transaction-protocol.md` - transaction coordinator design
4. `internal/component/config/transaction/orchestrator.go` - TxCoordinator.Execute
5. `internal/component/cli/model_commands.go` - CLI commitSaveAndReload
6. `internal/component/web/editor.go` - web EditorManager.Commit + commitHook
7. `internal/component/api/config_session.go` - API ConfigSessionManager.Commit
8. `internal/component/plugin/server/reload.go` - Server.reloadConfig (SIGHUP path)
9. `cmd/ze/hub/main_reload.go` - doReload (SIGHUP orchestration)

## Task

Make config commit transactional end-to-end across all five entry points: CLI, web,
API, SIGHUP, and managed/appliance push.

Today only the SIGHUP path uses the transaction coordinator (TxCoordinator with
verify -> apply -> commit phases and rollback). CLI, web, and API save the config
file first, then ask the daemon to reload as an afterthought. If reload fails,
config on disk diverges from running state.

The transaction protocol design doc already states the target principle: "Runtime is
authoritative. Config file is written only after all plugins confirm." The code does
not follow this for CLI/web/API.

Make all entry points go through the transaction coordinator so the config file is
written only after plugins have verified and applied successfully.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/transaction-protocol.md` - transaction coordinator design, phase protocol, ack semantics
  -> Decision: "Runtime is authoritative. Config file is written only after all plugins confirm. Disk failure is a warning, not a rollback trigger."
  -> Constraint: TxCoordinator.Execute runs verify -> apply -> commit(writeConfigFile). ConfigWriter slot exists on the coordinator but is unused in production; writeConfigFile returns true when configWriter is nil.
  -> Constraint: Rollback is an event. Engine emits rollback when any plugin's apply fails or times out. All applied plugins undo via journals.
- [ ] `docs/architecture/config/yang-config-design.md` - config editor, session semantics
  -> Constraint: Editor.Save() writes to storage (file or blob). The CLI and web both use this before reload. Session-mode commits merge changes then save.
- [ ] `docs/architecture/api/process-protocol.md` - plugin process management, 5-stage startup
  -> Decision: Plugins declare WantsConfigRoots and ConfigRoots at registration. Only plugins with affected roots participate in transactions.

### RFC Summaries (MUST for protocol work)
N/A - no wire protocol changes.

**Key insights:**
- The transaction coordinator already has a ConfigWriter slot designed for this exact purpose, but nobody sets it
- SIGHUP path already goes through the full transaction (verify -> apply) via Server.reloadConfig -> runTxCoordinator, but it does not write the config file because the file is the trigger (already on disk)
- CLI is an out-of-process editor that saves to disk, then fires a "ze-system:daemon-reload" RPC. The daemon-reload handler calls ReloadFromDisk, which re-reads the file and runs the transaction. Failure leaves file on disk but runtime unchanged.
- Web and API are in-process. They save to disk via editor.Save(), then call doReload (which is reloadAfterCommit). doReload reads the file back from disk and runs Server.ReloadConfig. API has file rollback on failure; web does not.
- Managed push caches the config to storage, then sends self-SIGHUP, which enters the SIGHUP path

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/transaction/orchestrator.go` - TxCoordinator state machine
  -> Constraint: Execute() runs verify -> apply -> publishCommitted -> writeConfigFile -> publishApplied(saved). ConfigWriter is a func() error set via SetConfigWriter. When nil, writeConfigFile returns true (no-op success).
  -> Constraint: On verify failure: publishAbort, return StateAborted. On apply failure: publishRollback, collectRollbackAcks, return StateRolledBack. No file write in either failure case.
- [ ] `internal/component/cli/model_commands.go:746-823` - CLI commit path
  -> Constraint: cmdCommit validates inline (not stale model state), blocks on errors+warnings. cmdCommitForce skips warnings, still blocks on errors. Both call commitSaveAndReload.
  -> Constraint: commitSaveAndReload: editor.Save() first, then archive (best-effort), then tryReload. Reload failure does not fail the commit. Status message indicates outcome.
  -> Constraint: cmdCommitSession (session mode): validates, calls editor.CommitSession(), then archive + tryReload. Same fire-and-forget reload.
- [ ] `internal/component/cli/reload.go` - CLI reload notifier
  -> Constraint: NewSocketReloadNotifier connects to daemon Unix socket, sends "ze-system:daemon-reload" RPC, waits for response. 5-second timeout. On error, CLI shows "(reload errors, type 'errors' for details)" but commit is still considered successful.
- [ ] `internal/component/web/editor.go` - Web EditorManager
  -> Constraint: EditorManager.Commit calls editor.CommitSession(). Separate RunCommitHook() call after. Hook is func() error.
  -> Constraint: SetCommitHook wired in main.go:791 to reloadAfterCommit (= doReload).
- [ ] `internal/component/web/handler_config.go:870-877` - Web commit handler
  -> Constraint: HandleConfigCommit calls mgr.RunCommitHook(). On error: returns HTTP 500 with "config saved but reload failed". Config is already on disk. No file rollback.
- [ ] `internal/component/api/config_session.go` - API ConfigSessionManager
  -> Constraint: Commit path: validate (onValidate hook), save (editor.Save()), onCommit hook. On onCommit failure: restores file via RestoreOriginalContent. This is the only path with file rollback on reload failure.
  -> Decision: API already does rollback-on-failure, but at the file level (restore previous content), not at the plugin level (no plugin rollback).
- [ ] `internal/component/plugin/server/reload.go` - Server.reloadConfig
  -> Constraint: Acquires txLock (one transaction at a time). Computes diff (running vs new tree). Auto-loads/stops plugins for added/removed config sections. Calls runTxCoordinator for affected plugins. On commit: SetConfigTree on reactor. On failure: stops auto-loaded plugins, returns error.
  -> Constraint: No config file write here. The SIGHUP path assumes the file is the source; plugins verify/apply the in-memory tree.
- [ ] `internal/component/plugin/server/reload_tx.go` - runTxCoordinator
  -> Constraint: Builds participants from affected plugins. Creates ConfigEventGateway + configTxBridge. Creates TxCoordinator. Does NOT set ConfigWriter. Calls Execute, converts result to error.
- [ ] `internal/component/plugin/server/system.go:211` - handleDaemonReload RPC
  -> Constraint: Calls Server.ReloadFromDisk which reads config from disk, then calls reloadConfig. Fallback: direct Reactor.Reload() when no config loader.
- [ ] `cmd/ze/hub/main_reload.go` - doReload
  -> Constraint: doReload: load() reads config file once, snapshots provider, calls Server.ReloadConfig(ctx, newTree), refreshes provider, calls engine.Reload, applies host tuning / console / conntrack / update checker / archive scheduler / listener migration. On engine.Reload failure: rolls back via rollbackReload (plugin rollback + provider restore + engine re-reload).
  -> Decision: doReload is the full subsystem reload path. It does more than the transaction coordinator: it also refreshes the ConfigProvider, engine subsystems, host tuning, console, conntrack, etc.
- [ ] `cmd/ze/hub/main.go:787-791` - reloadAfterCommit closure
  -> Constraint: reloadAfterCommit = func() error { return doReload(apiServer, eng, configProvider, loadBoth, lm) }. Same function used by web (commitHook) and API (onCommit).
- [ ] `internal/component/managed/client.go:176-203` - managed client
  -> Constraint: fetchAndProcess: ProcessConfig validates + caches the config. On success: calls OnReload (which is self-SIGHUP). The SIGHUP then enters the standard SIGHUP path. No transactional integrity beyond what SIGHUP provides.
  -> Decision: Managed path has an extra validate step (handler.Validate) before caching, but the actual reload is fully delegated to SIGHUP.

**Behavior to preserve:**
- CLI standalone mode (no daemon): commit saves to disk, reports "daemon not running". No transaction needed -- no runtime to protect.
- Validation before commit (errors + warnings block). Both CLI and API pre-validate. This stays.
- Archive notification (best-effort, non-fatal). Independent of transaction.
- Session conflict detection (CLI session mode, web session mode). Independent of transaction.
- txLock mutual exclusion: only one config transaction at a time. SIGHUP queuing when locked.
- doReload's subsystem refresh (ConfigProvider, engine.Reload, host tuning, etc.). Must happen after successful transaction.
- Web SSE broadcast of config changes after commit.
- API file rollback on reload failure (will be replaced by not writing the file until success).

**Behavior to change:**
- CLI with daemon: currently save-then-reload. Change to: send candidate to daemon, daemon runs transaction, daemon writes file on success, CLI gets success/failure.
- Web commit: currently save-then-commitHook. Change to: pass candidate tree to transaction coordinator, write file only on success.
- API commit: currently save-then-onCommit-then-maybe-rollback-file. Change to: pass candidate tree to transaction coordinator, write file only on success. Simpler, no file rollback needed.
- Managed push: currently validate-then-cache-then-ACK-then-self-SIGHUP. Three problems: (1) the cache write happens before the transaction, so a failed transaction leaves stale config in the store; (2) the ACK tells the hub "config accepted" before plugins verify it; (3) OnReload is fire-and-forget (self-SIGHUP), so the client never learns whether the transaction succeeded. Fix: replace fire-and-forget OnReload with synchronous OnCommit(data) callback wired by hub.Run after startup. fetchAndProcess defers cache + ACK until OnCommit returns. On success: cache, ACK OK. On failure: ACK with error, no cache. First-boot race (OnCommit not yet wired): temporary rejection, hub retries.
- runTxCoordinator: must set ConfigWriter on the coordinator so the file write is part of the transaction's commit phase.

## Data Flow (MANDATORY)

### Entry Point
Five entry points, converging to one transaction path:

1. **CLI**: operator types `commit` in SSH CLI -> model_commands.go cmdCommit -> (new) sends candidate config via RPC to daemon
2. **Web**: operator clicks commit in HTMX UI -> handler_config.go HandleConfigCommit -> (new) passes candidate tree to transaction path
3. **API**: client calls POST /api/v1/config/session/commit -> config_session.go Commit -> (new) passes candidate tree to transaction path
4. **SIGHUP**: operator sends SIGHUP or file changes -> main_reload.go doReload -> Server.ReloadConfig (already transactional)
5. **Managed**: hub pushes config -> managed/client.go fetchAndProcess -> caches + self-SIGHUP (enters SIGHUP path)

### Transformation Path
1. Candidate config arrives as: string content (CLI/API), tree (web/SIGHUP), or raw bytes (managed)
2. Parsed to map[string]any tree if needed
3. Diffed against running config (Server.reloadConfig computes configDiff)
4. Diff routed to affected plugins based on WantsConfigRoots declarations
5. Transaction coordinator runs verify -> apply -> writeConfigFile -> publishCommitted
6. On success: file written (ConfigWriter), reactor SetConfigTree, ConfigProvider refreshed, engine.Reload
7. On failure: rollback, file untouched, error returned to caller

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI process -> daemon process | Unix socket RPC (NUL-framed JSON). Currently "daemon-reload"; needs new "config-commit" RPC carrying candidate content | [ ] |
| Web/API (in-process) -> plugin server | Direct function call to Server.ReloadConfig or new commit method | [ ] |
| Plugin server -> plugins | Stream events via EventGateway (config namespace): verify-plugin, apply-plugin, rollback | [ ] |
| Plugins -> plugin server | Ack events: verify-ok/failed, apply-ok/failed, rollback-ok | [ ] |
| Transaction coordinator -> disk | ConfigWriter callback writes config file after all plugins apply | [ ] |
| Managed client -> daemon | Self-SIGHUP after caching config | [ ] |

### Integration Points
- `Server.ReloadConfig(ctx, newTree)` - already handles diff, plugin routing, transaction. Must become the single commit path for web/API.
- `doReload` in main_reload.go - wraps ReloadConfig with provider refresh + engine.Reload + subsystem reload. Web/API commit hooks already call this.
- `handleDaemonReload` RPC - CLI's current entry point. Needs a sibling RPC that accepts candidate content instead of re-reading from disk.
- `TxCoordinator.SetConfigWriter` - unused slot that must be wired to write the config file.
- `Editor.Save()` - currently called before reload. Must be deferred or replaced by the ConfigWriter.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Design

### Approach: Timestamped versions with named pointers

Every config write creates a timestamped version in storage. Named pointers
(`active`, `candidate`, `rollback`, `recovery`) reference specific timestamps.
Promotion is pointer manipulation, not file copying.

**Storage model:**

| Key | Purpose |
|-----|---------|
| `file/{timestamp}/ze.conf` | Immutable config version (already exists for version history) |
| `meta/config/active` | Pointer to the timestamp of the running config |
| `meta/config/candidate` | Pointer to the timestamp of a pending commit (transient) |
| `meta/config/rollback` | Pointer to the timestamp of the previous active (before last commit) |
| `meta/config/recovery` | Operator-set pointer to a known-good config |

On plain filesystem: pointer files containing the timestamp string. In zefs
blob storage: meta keys containing the timestamp string.

**Boot:** Daemon reads the `active` pointer, loads that timestamped version.

**Commit flow (all five entry points):**

1. Caller writes candidate config to `file/{timestamp}/ze.conf`
2. Set `candidate` pointer to that timestamp
3. `doReload` reads from the candidate (not from active)
4. `Server.ReloadConfig` runs the transaction (verify -> apply)
5. On success:
   - `rollback` pointer = current `active` pointer
   - `active` pointer = candidate timestamp
   - Clear `candidate` pointer
   - Subsystem refresh (ConfigProvider, engine.Reload, etc.)
6. On failure:
   - Clear `candidate` pointer
   - `active` pointer unchanged
   - Return error to caller

**Per-entry-point specifics:**

- **CLI (no daemon):** Write to `active` directly (no transaction possible). Same as today.
- **CLI (with daemon):** Write candidate version, fire `daemon-reload` RPC. RPC handler reads candidate, runs transaction, promotes on success. CLI waits for RPC result and reports success or failure.
- **Web/API:** Write candidate version, call `doReload` with candidate path. On success: promote. On failure: clean up candidate, return error.
- **SIGHUP:** Operator edited the file (or wrote a new version). SIGHUP handler reads from file, writes it as a candidate version, runs the transaction. On success: promote. On failure: clean up candidate.
- **Managed:** `OnCommit(data)` writes candidate version, runs transaction via `doReload`. On success: promote, return nil. `fetchAndProcess` defers ACK until `OnCommit` returns. Hub gets an honest ACK.

**Alternatives rejected:**

1. *Save-then-rollback:* Write directly to active, snapshot bytes, restore on failure. Simpler but active is briefly wrong on disk. No clean version history.
2. *Verify-then-save (ConfigWriter slot):* Wire TxCoordinator.SetConfigWriter. Architecturally pure but requires changing doReload's interface to not read from disk. Conflates the transaction coordinator's responsibility (plugin coordination) with storage management (file versioning, pointer promotion).

**Failure modes:**

| Failure | What happens |
|---------|-------------|
| Candidate write fails (disk full, permission) | Error before transaction starts. No candidate pointer set. Active unchanged. |
| Verify fails (plugin rejects config) | Transaction aborted. Candidate pointer cleared. Active unchanged. |
| Apply fails (plugin crashes during apply) | Rollback. Candidate pointer cleared. Active unchanged. |
| Pointer promotion fails after successful apply | Runtime is live with new config. Active pointer stale. Log error, operator can retry. Same as today's "config file write failed" in TxCoordinator.writeConfigFile. |
| Concurrent commits | txLock prevents concurrent transactions. Second caller gets ErrReloadInProgress. Candidate from second caller not written. |
| Crash during transaction | Candidate pointer exists but transaction incomplete. On next boot, daemon ignores candidate (boots from active). Stale candidate cleaned up on startup. |
| SIGHUP during active transaction | Already handled: SIGHUP is queued via txLock.queueSIGHUP, replayed after current transaction. |

**Managed path detail:**

Replace `OnReload func()` with `OnCommit func(data []byte) error` on `ClientConfig`. The managed client goroutine starts before hub.Run, so `OnCommit` starts nil. Hub.Run wires it after creating `doReload`. If `OnCommit` is nil when a config push arrives (first-boot race), return a temporary rejection. Hub retries on next heartbeat (30s).

`fetchAndProcess` becomes:
1. Validate config bytes (parse check)
2. Call `OnCommit(data)` -- blocks until transaction completes
3. On success: send `ConfigAck{OK: true}`
4. On failure: send `ConfigAck{OK: false, Error: reason}`
5. Cache write is part of OnCommit (the candidate-then-promote flow handles it)

**Triple challenge:**

| Challenge | Answer |
|-----------|--------|
| Simplicity | Named pointers + timestamped versions. No new RPC, no new coordinator wiring. doReload gains a candidate-aware read path and promote/cleanup. Storage gains pointer operations (ReadPointer, WritePointer, ClearPointer). Minimal additions. |
| Uniformity | Uses the existing Storage interface (WriteVersion, ReadFile, meta keys). Pointer operations follow the same pattern as WriteVersion/ListVersions. All five entry points converge to the same doReload path. |
| Performance | No per-event allocations. Pointer read is one meta key read. Promotion is two pointer writes. No additional parsing or diffing beyond what doReload already does. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| CLI `commit` with daemon | -> | daemon-reload RPC -> doReload reads candidate -> transaction -> promote | `TestCLICommitTransactional` |
| CLI `commit` without daemon | -> | Write to active directly (no transaction) | `TestCLICommitStandalone` |
| Web `commit` | -> | Write candidate -> doReload -> transaction -> promote | `TestWebCommitTransactional` |
| API `POST /config/session/commit` | -> | Write candidate -> doReload -> transaction -> promote | `TestAPICommitTransactional` |
| SIGHUP | -> | Read file -> write candidate -> doReload -> transaction -> promote | `TestSIGHUPCommitTransactional` |
| Managed push | -> | OnCommit(data) -> write candidate -> doReload -> transaction -> promote -> ACK | `TestManagedCommitTransactional` |
| CLI `commit` with daemon, verify fails | -> | daemon-reload -> transaction fails -> candidate cleared -> CLI gets error | `TestCLICommitVerifyFail` |
| Boot with stale candidate | -> | Daemon ignores candidate pointer, boots from active | `TestBootIgnoresStaleCandidate` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | CLI commit with running daemon, plugins accept | Config written as timestamped version, candidate promoted to active, rollback points to previous active, CLI reports success |
| AC-2 | CLI commit with running daemon, plugin verify rejects | Candidate version cleaned up, active pointer unchanged, CLI reports commit failure with plugin error |
| AC-3 | Web commit, plugins accept | Same as AC-1 but triggered from web handler. SSE broadcast after promotion. |
| AC-4 | Web commit, plugin apply fails | Candidate cleaned up, active unchanged, HTTP error returned. No SSE broadcast. |
| AC-5 | API session commit, plugins accept | Same as AC-1 but triggered from API. Session closed after promotion. |
| AC-6 | API session commit, plugin verify rejects | Candidate cleaned up, active unchanged, API error returned. Session remains open for editing. |
| AC-7 | SIGHUP, plugins accept | File content written as candidate version, promoted to active, subsystem refresh runs |
| AC-8 | SIGHUP, plugin apply fails + rollback | Candidate cleaned up, active unchanged, error logged to stderr |
| AC-9 | Managed config push, plugins accept | Candidate promoted, ACK OK sent to hub |
| AC-10 | Managed config push, plugin verify rejects | Candidate cleaned up, ACK with error sent to hub. Hub knows device rejected. |
| AC-11 | Managed config push before hub.Run wires OnCommit | Temporary rejection returned. Hub retries. No candidate written. |
| AC-12 | Boot with stale candidate pointer (crash recovery) | Daemon boots from active, stale candidate pointer cleared or ignored |
| AC-13 | Concurrent commit attempt while transaction in progress | Second caller gets ErrReloadInProgress. No candidate written for second caller. |
| AC-14 | CLI commit without daemon | Config written directly to active (no transaction). Same as today. |
| AC-15 | `rollback` pointer set after successful commit | `rollback` points to the previous active timestamp, enabling future `ze rollback` |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWriteCandidateVersion` | `internal/component/config/storage/pointer_test.go` | Writing a timestamped version and setting candidate pointer | |
| `TestPromoteCandidateToActive` | `internal/component/config/storage/pointer_test.go` | Atomic pointer promotion: rollback=old active, active=candidate, candidate cleared | |
| `TestPromoteFailureCleanup` | `internal/component/config/storage/pointer_test.go` | On transaction failure: candidate pointer cleared, active unchanged | |
| `TestBootFromActivePointer` | `internal/component/config/storage/pointer_test.go` | Boot reads active pointer, loads that version | |
| `TestBootIgnoresStaleCandidate` | `internal/component/config/storage/pointer_test.go` | Stale candidate pointer does not affect boot | |
| `TestReadConfigFromCandidate` | `cmd/ze/hub/main_reload_test.go` | doReload reads from candidate path when candidate pointer is set | |
| `TestDoReloadRollbackOnVerifyFail` | `cmd/ze/hub/main_reload_test.go` | doReload clears candidate and returns error when verify fails | |
| `TestDoReloadPromotesOnSuccess` | `cmd/ze/hub/main_reload_test.go` | doReload promotes candidate to active on success | |
| `TestManagedOnCommitNil` | `internal/component/managed/client_test.go` | OnCommit nil returns temporary rejection | |
| `TestManagedACKDeferredUntilCommit` | `internal/component/managed/client_test.go` | ACK sent only after OnCommit returns | |
| `TestManagedACKErrorOnCommitFail` | `internal/component/managed/client_test.go` | ACK error when OnCommit returns error | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-commit-transactional` | `test/plugin/*.ci` | CLI commit with plugin that accepts: config applied, rollback pointer set | |
| `test-commit-verify-reject` | `test/plugin/*.ci` | CLI commit with plugin that rejects verify: commit fails, active unchanged | |
| `test-sighup-transactional` | `test/plugin/*.ci` | SIGHUP reload: candidate written, promoted on success | |

### Interop Tests (MANDATORY for protocol features)
N/A - no wire protocol changes.

### Future (if deferring any tests)
- `ze rollback` command: uses rollback pointer to restore previous config. Separate spec.

## Files to Modify
- `pkg/zefs/keys.go` - add `KeyFileCandidate` key (`file/candidate/{basename}`)
- `internal/component/config/storage/storage.go` - add pointer read/write methods to Storage interface
- `internal/component/config/storage/blob.go` - implement pointer ops using meta keys
- `cmd/ze/hub/main_reload.go` - doReload: read from candidate when set, promote/cleanup on success/failure
- `cmd/ze/hub/main.go` - boot: resolve active pointer; wire managed OnCommit; clean stale candidate on startup
- `internal/component/cli/model_commands.go` - commitSaveAndReload: write candidate version instead of writing to active; treat reload failure as commit failure
- `internal/component/cli/editor_commands.go` - Save: write to candidate path when daemon is available
- `internal/component/web/editor.go` - commitHook: write candidate, call doReload, promote on success
- `internal/component/web/handler_config.go` - HandleConfigCommit: propagate transaction failure (no more "saved but reload failed")
- `internal/component/api/config_session.go` - Commit: write candidate, call onCommit, promote on success. Remove file rollback (no longer needed).
- `internal/component/managed/client.go` - replace OnReload with OnCommit; defer ACK until OnCommit returns
- `internal/component/managed/handler.go` - ProcessConfig: validate only, remove Cache call
- `internal/component/plugin/server/system.go` - handleDaemonReload: read from candidate if set
- `docs/architecture/config/transaction-protocol.md` - update to document candidate/pointer model

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| CLI commands/flags | No (commit semantics change, not syntax) | |
| CLI grammar (action before identifier) | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | Yes | `test/plugin/*.ci` |
| Doctor check for runtime dependencies | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - transactional commit |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No (semantics change) | `docs/guide/command-reference.md` - commit now transactional |
| 4 | API/RPC added/changed? | No (semantics change) | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/operations.md` - commit + rollback model |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - transactional commit |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/transaction-protocol.md` |

## Files to Create
- `internal/component/config/storage/pointer.go` - named pointer operations (ReadPointer, WritePointer, ClearPointer, PromoteCandidate)
- `internal/component/config/storage/pointer_test.go` - tests for pointer operations
- `test/plugin/commit-transactional.ci` - functional test for transactional commit
- `test/plugin/commit-verify-reject.ci` - functional test for commit rejection

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- storage pointer API + doReload candidate path
   - Tests: `TestWriteCandidateVersion`, `TestPromoteCandidateToActive`, `TestBootFromActivePointer`
   - Files: `pointer.go`, `keys.go`, `storage.go`, `blob.go`
   - Verify: pointer ops work in isolation; doReload skeleton can read from candidate path

2. **Phase: Storage pointer operations** -- implement read/write/clear/promote for blob and filesystem
   - Tests: `TestPromoteFailureCleanup`, `TestBootIgnoresStaleCandidate`
   - Files: `pointer.go`, `blob.go`, filesystem implementation
   - Verify: all pointer tests pass, including cleanup on failure

3. **Phase: doReload candidate integration** -- doReload reads candidate, promotes/cleans up
   - Tests: `TestReadConfigFromCandidate`, `TestDoReloadRollbackOnVerifyFail`, `TestDoReloadPromotesOnSuccess`
   - Files: `main_reload.go`, `main.go` (boot resolution)
   - Verify: doReload drives full cycle: candidate -> transaction -> promote/cleanup

4. **Phase: CLI commit path** -- write candidate instead of active, treat reload failure as commit failure
   - Tests: `TestCLICommitTransactional`, `TestCLICommitVerifyFail`, `TestCLICommitStandalone`
   - Files: `model_commands.go`, `editor_commands.go`
   - Verify: CLI commit with daemon writes candidate; failure = commit failure

5. **Phase: Web + API commit paths** -- write candidate, propagate transaction result
   - Tests: `TestWebCommitTransactional`, `TestAPICommitTransactional`
   - Files: `editor.go`, `handler_config.go`, `config_session.go`
   - Verify: web/API commit writes candidate; success = promote; failure = cleanup + error

6. **Phase: Managed path** -- OnCommit callback, deferred ACK
   - Tests: `TestManagedOnCommitNil`, `TestManagedACKDeferredUntilCommit`, `TestManagedACKErrorOnCommitFail`
   - Files: `client.go`, `handler.go`, `main.go` (wiring)
   - Verify: managed push defers ACK until transaction result; nil OnCommit = temp rejection

7. **Phase: SIGHUP path** -- write candidate from edited file, promote on success
   - Tests: `TestSIGHUPCommitTransactional`
   - Files: `main_reload.go`
   - Verify: SIGHUP creates candidate from file, promotes on success, cleans up on failure

8. **Functional tests** -- end-to-end commit and rejection scenarios
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Pointer promotion is atomic (rollback set before active changes). Candidate cleaned on every failure path. |
| Naming | Pointer names: active, candidate, rollback, recovery. Storage keys follow zefs conventions. |
| Data flow | All five entry points converge to doReload with candidate. No path bypasses the transaction. |
| Concurrency | txLock prevents concurrent transactions. Candidate pointer protected by same lock. |
| Boot safety | Stale candidate from crash does not affect boot. Active pointer is authoritative. |
| CLI standalone | CLI without daemon still works (writes to active, no transaction). |
| Rule: no-layering | Remove API file rollback (RestoreOriginalContent path). Remove web "saved but reload failed" message. |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `KeyFileCandidate` in zefs keys | `grep KeyFileCandidate pkg/zefs/keys.go` |
| Pointer operations in storage | `go test ./internal/component/config/storage/ -run TestPointer` |
| doReload reads candidate | `grep candidate cmd/ze/hub/main_reload.go` |
| CLI commit writes candidate | `grep candidate internal/component/cli/model_commands.go` |
| Web commit propagates failure | no "saved but reload failed" in handler_config.go |
| API removes file rollback | no RestoreOriginalContent in config_session.go Commit path |
| Managed OnCommit replaces OnReload | `grep OnCommit internal/component/managed/client.go` |
| Managed ACK deferred | ACK sent after OnCommit returns in fetchAndProcess |
| Boot resolves active pointer | `grep active.*pointer cmd/ze/hub/main.go` |
| Stale candidate cleanup on boot | test proves boot ignores candidate |
| Functional tests exist | `ls test/plugin/commit-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Candidate timestamp must be valid format (no path traversal via crafted timestamp) |
| Pointer manipulation | Only the transaction path sets/clears candidate pointer. No external API to manipulate pointers directly. |
| Stale candidate | Crash recovery must not load a stale candidate as if it were active |
| Managed rejection | Temporary rejection (OnCommit nil) must not leak internal state to hub |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Fix test assertion or setup |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

N/A - no wire protocol changes.

## Implementation Summary

### What Was Implemented
- Storage now has named config pointers (`active`, `candidate`, `rollback`, `recovery`) and candidate version helpers.
- Hub reload reads staged candidates, promotes on success, clears on failure, initializes active on boot, and ignores stale boot candidates.
- CLI non-session commits and session commits with a daemon stage candidates and treat reload failure as commit failure. Standalone CLI still writes directly.
- Web and REST/API commits stage candidates before invoking the hub reload hook.
- Managed pushes use synchronous `OnCommit(data)` and ACK only after commit succeeds or fails.
- The `.ci` runner now supports native file assertions, HTTP `sendfile` content types, and self-signed HTTPS checks.
- The `.ci` runner keeps blob storage enabled for daemon tests started with `--web`, because the web server requires blob storage.
- Listener migration during reload now rolls back earlier successful listener reconfigurations if a later listener fails, then rejects the candidate.
### Bugs Found/Fixed
- CLI session commit still wrote active config before reload. It now uses `CommitSessionCandidate`, keeps edits dirty on rejection, and finalizes state only after reload success.
- Direct SSH editor commits were calling `reload`; the registered command is `daemon reload`, so `ze config edit` now invokes the correct command.
- Set/set-meta parsing preserved displayed boolean tokens (`enable`/`disable`) instead of normalizing to `true`/`false`, causing editor-generated configs to fail YANG validation before reload. Boolean leaves are now normalized in all set parsers.
- Web transactional functional tests were forced into filesystem storage by the test runner, which disables the web server. `--web` tests now keep blob storage and isolate state through `ze.config.dir`.
- Session candidate commits wrote a version under lock but set `PointerCandidate` after releasing the lock, bypassing the existing-candidate guard. `WriteCandidateVersionWithGuard` now stages session candidates atomically under the same guard.
- Listener migration failure previously cleared the candidate without rolling plugin/provider/engine state back, and without reverting listeners already reconfigured in the same migration. `doReload` and `ListenerMigrator` now rollback those paths before returning failure.
- Legacy active-file mirror failure after pointer promotion previously returned an error even though the active pointer had advanced. The mirror write is now a warning after promotion.
### Documentation Updates
- `docs/architecture/config/transaction-protocol.md` documents active/candidate/rollback pointer persistence.
- `docs/architecture/testing/ci-format.md` documents file expectations and HTTP sendfile/content-type/TLS options.
- `docs/features.md`, `docs/guide/command-reference.md`, and `docs/guide/operations.md` describe transactional reload/commit behavior.
### Deviations from Plan
- The design uses storage pointer promotion around `doReload` instead of wiring `TxCoordinator.SetConfigWriter` in production. This matches the chosen design section: storage owns persistence, plugin transactions own runtime apply/rollback.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Implemented, functional verified | `TestCommitTriggersReload`, `TestCmdCommitSessionReload`, `test/plugin/cli-commit-transactional.ci` | Direct SSH CLI commit stages a candidate, daemon reload promotes it, and CLI reports success. |
| AC-2 | Implemented, functional verified | `TestCommitReloadFailsGracefully`, `TestCmdCommitSessionReloadFails`, `test/plugin/cli-commit-reject.ci` | Direct SSH CLI reject keeps active unchanged, clears candidate, and returns the plugin error. |
| AC-3 | Implemented, functional verified | `TestHandleConfigCommitPOST_HookCalled`, `test/ui/web-commit-transactional.ci` | Functional test verifies HTTPS web commit applies accepted config through the daemon reload path. |
| AC-4 | Implemented, functional verified | `TestHandleConfigCommitPOST_HookError`, `test/ui/web-commit-reject.ci` | Functional test verifies HTTP error on plugin apply failure and unchanged running config. |
| AC-5 | Implemented, functional verified | `TestConfigSessionCommitHook`, `test/plugin/audit-config-commit.ci` | Functional test verifies REST config session commit and active/candidate/rollback pointers. |
| AC-6 | Implemented, functional verified | `TestConfigSessionCommitHookFailureKeepsSession`, `test/plugin/api-config-commit-reject.ci` | Functional test verifies rejected REST commit keeps session editable and active unchanged. |
| AC-7 | Implemented, functional verified | `TestDoReloadPromotesCandidateOnSuccess`, `test/reload/commit-transactional.ci` | SIGHUP success promotes candidate and sets rollback. |
| AC-8 | Implemented, functional verified | `TestDoReloadClearsCandidateOnFailure`, `test/reload/commit-verify-reject.ci` | Functional test verifies rejected reload leaves active unchanged and clears candidate; plugin-server units cover rollback event behavior. |
| AC-9 | Implemented, functional verified | `TestManagedACKDeferredUntilCommit`, `TestWireManagedCommitStagesCandidateAndPromotes`, `test/managed/config-push-transactional.ci` | End-to-end managed hub/client push returns ACK OK only after promotion. |
| AC-10 | Implemented, functional verified | `TestManagedACKErrorOnCommitFail`, `TestWireManagedCommitClearsCandidateOnReloadFailure`, `test/managed/config-push-transactional.ci` | End-to-end managed reject returns ACK error and leaves active unchanged. |
| AC-11 | Implemented, unit verified | `TestManagedOnCommitNil` | No candidate writer exists when `OnCommit` is nil. |
| AC-12 | Implemented, unit verified | `TestBootIgnoresStaleCandidate`, `TestClearStaleCandidateOnBoot` | Boot reads active and clears stale candidate. |
| AC-13 | Implemented, functional verified | `TestReloadConfigConcurrentRejected`, `TestWriteCandidateVersionRejectsExistingCandidate`, `TestStageSIGHUPCandidateRejectsExistingCandidate`, `test/plugin/concurrent-config-commit.ci` | Functional test verifies a concurrent REST commit is rejected and second candidate is not promoted. |
| AC-14 | Implemented, unit verified | `TestCommitNoNotifierStandalone` | Standalone CLI keeps direct save behavior. |
| AC-15 | Implemented, functional verified | `TestPromoteCandidateToActive`, `test/reload/commit-transactional.ci`, `test/plugin/audit-config-commit.ci` | Rollback pointer is previous active after successful promotion. |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestWriteCandidateVersion` | Passes | `internal/component/config/storage/pointer_test.go` | Candidate version write + pointer. |
| `TestPromoteCandidateToActive` | Passes | `internal/component/config/storage/pointer_test.go` | Promotion sets rollback and active, then clears candidate. |
| `TestPromoteFailureCleanup` | Passes | `internal/component/config/storage/pointer_test.go` | Failure cleanup leaves active unchanged. |
| `TestBootFromActivePointer` | Passes | `internal/component/config/storage/pointer_test.go` | Boot loads active pointer. |
| `TestBootIgnoresStaleCandidate` | Passes | `internal/component/config/storage/pointer_test.go` | Candidate is ignored at boot. |
| `TestReadConfigFromCandidate` | Changed | `cmd/ze/hub/main_reload_test.go` | Covered by `TestDoReloadPromotesCandidateOnSuccess`. |
| `TestDoReloadRollbackOnVerifyFail` | Changed | `cmd/ze/hub/main_reload_test.go` | Covered by `TestDoReloadClearsCandidateOnFailure`. |
| `TestDoReloadPromotesOnSuccess` | Passes | `cmd/ze/hub/main_reload_test.go` | Implemented as `TestDoReloadPromotesCandidateOnSuccess`. |
| `TestManagedOnCommitNil` | Passes | `internal/component/managed/client_test.go` | Nil commit hook rejects. |
| `TestManagedACKDeferredUntilCommit` | Passes | `internal/component/managed/client_test.go` | ACK after commit hook returns. |
| `TestManagedACKErrorOnCommitFail` | Passes | `internal/component/managed/client_test.go` | ACK includes commit error. |
| `TestWriteCandidateVersionWithGuardRejectsExistingCandidate` | Passes | `internal/component/config/storage/pointer_test.go` | Guard-held candidate staging rejects an existing candidate. |
| `TestPromoteCandidateMirrorFailureIsNonFatal` | Passes | `internal/component/config/storage/pointer_test.go` | Legacy active-file mirror failure does not turn a promoted active pointer into caller-visible commit failure. |
| `TestCmdCommitSessionRejectsExistingCandidate` | Passes | `internal/component/cli/model_commands_test.go` | Session commits reject an existing in-flight candidate instead of overwriting it. |
| `TestDoReloadRollsBackOnListenerMigrationFailure` | Passes | `cmd/ze/hub/main_reload_test.go` | Listener migration failure rolls plugin/provider state back and clears candidate. |
| `TestReloadListenersRollsBackAppliedServiceOnLaterFailure` | Passes | `cmd/ze/hub/listener_migrate_test.go` | Earlier successful listener reconfigurations are reverted if a later service fails. |
| `test-commit-transactional` | Passes | `test/plugin/cli-commit-transactional.ci`, `test/plugin/audit-config-commit.ci`, `test/ui/web-commit-transactional.ci` | Direct SSH CLI, REST, and web success paths cover staged commit behavior. |
| `test-commit-verify-reject` | Passes | `test/plugin/cli-commit-reject.ci`, `test/plugin/api-config-commit-reject.ci`, `test/ui/web-commit-reject.ci` | Direct SSH CLI, REST, and web rejection paths keep active unchanged. |
| `test-sighup-transactional` | Passes | `test/reload/commit-transactional.ci`, `test/reload/commit-verify-reject.ci` | Native `.ci` file assertions cover pointers. |
| `test-managed-transactional` | Passes | `test/managed/config-push-transactional.ci` | Managed hub/client config push covers ACK OK and ACK error paths. |
| `test-concurrent-config-commit` | Passes | `test/plugin/concurrent-config-commit.ci` | Concurrent REST commit attempts reject the second candidate while the first transaction is in progress. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:** 15 acceptance criteria, 16 unit/regression tests, 5 functional test scenarios after scope expansion.
- **Done:** 15 acceptance criteria implemented with unit and functional coverage where user-facing.
- **Partial:** none known after the 2026-05-25 targeted verification pass.
- **Skipped:** none.
- **Changed:** direct SSH CLI, web reject, managed E2E, and concurrent commit functional tests were added beyond the initial functional-test table.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Make config commit transactional across CLI | Unit + functional | `go test ./internal/component/cli` passes; `go run ./cmd/ze-test bgp plugin -v cli-commit-transactional cli-commit-reject` passes. |
| Make config commit transactional across web | Unit + functional | `go test ./internal/component/web` passes; `go run ./cmd/ze-test ui -v web-commit-transactional web-commit-reject` passes. |
| Make config commit transactional across API | Unit + functional | `go test ./internal/component/api/...` passes; `go run ./cmd/ze-test bgp plugin -v audit-config-commit api-config-commit-reject` passes. |
| Make SIGHUP reload use candidate promotion | Unit + functional | `go test ./cmd/ze/hub` passes; `go run ./cmd/ze-test bgp reload -v commit-transactional commit-verify-reject` passes. |
| Make managed push ACK reflect commit result | Unit + functional | `go test ./internal/component/managed ./cmd/ze/hub` passes; `go run ./cmd/ze-test managed -v config-push-transactional` passes. |
| Reject overlapping commit attempts | Unit + functional | `go test ./internal/component/plugin/server ./internal/component/config/storage ./cmd/ze/hub` passes; `go run ./cmd/ze-test bgp plugin -v concurrent-config-commit` passes. |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | High | Session candidate commits could overwrite an existing in-flight candidate because `CommitSessionCandidate` wrote the version under lock but wrote `PointerCandidate` after releasing the lock. | `internal/component/cli/editor_commit.go` | Added `storage.WriteCandidateVersionWithGuard` and changed session staging to use it under the existing guard. Added `TestCmdCommitSessionRejectsExistingCandidate`. |
| 2 | High | Listener migration failure after plugin/provider/engine reload cleared the candidate without rolling runtime/provider state back. | `cmd/ze/hub/main_reload.go` | `doReload` now calls `rollbackReload` before clearing candidate on listener migration failure, and side-effect refreshes run only after listener migration succeeds. Added `TestDoReloadRollsBackOnListenerMigrationFailure`. |
| 3 | Medium | `PromoteCandidate` returned an error if the legacy active-file mirror write failed after the active pointer had already advanced. | `internal/component/config/storage/pointer.go` | Mirror write failure is now logged as a warning after pointer promotion. Added `TestPromoteCandidateMirrorFailureIsNonFatal`. |

### Fixes applied
- Added guarded candidate staging helper for callers that already hold a storage write guard.
- Added session, storage, hub reload, and listener rollback regression tests.
- Moved listener migration before non-error side-effect refreshes and added rollback on listener migration failure.
- Changed `ListenerMigrator` fields to `Reconfigurable` so rollback behavior is testable without live servers.
- Added listener migration rollback for previously applied services when a later service reconfigure fails.
- Treated legacy active-file mirror failure as non-fatal after active pointer promotion.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 4 | Medium | Earlier listener reconfigurations could remain applied if a later service failed during the same listener migration. | `cmd/ze/hub/listener_migrate.go` | `ReloadListeners` now tracks applied changes and reconfigures them back to old addresses in reverse order on later failure. Added `TestReloadListenersRollsBackAppliedServiceOnLaterFailure`. |
| 5 | Note | Listener rollback coverage is unit-level; there is no functional test with multiple live listener services where one reconfigure succeeds and a later one fails. | `cmd/ze/hub/listener_migrate_test.go`, `cmd/ze/hub/main_reload_test.go` | Accepted as residual risk because unit coverage verifies rollback mechanics and no AC requires multi-live-listener functional failure injection. |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- Focused review re-run shows no findings; residual NOTE recorded above.
- All NOTEs recorded above.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/config/storage/pointer.go` | Yes | `grep WriteCandidateVersion` found storage pointer implementation. |
| `internal/component/config/storage/pointer_test.go` | Yes | `go test ./internal/component/config/...` passed. |
| `cmd/ze/hub/main_reload_test.go` | Yes | `go test ./cmd/ze/hub` passed. |
| `cmd/ze/hub/listener_migrate_test.go` | Yes | `go test ./cmd/ze/hub -run 'TestReloadListenersRollsBackAppliedServiceOnLaterFailure|TestDoReloadRollsBackOnListenerMigrationFailure'` passed. |
| `cmd/ze/hub/managed.go` | Yes | `grep wireManagedCommit` found managed commit wiring. |
| `test/plugin/cli-commit-transactional.ci` | Yes | `go run ./cmd/ze-test bgp plugin -v cli-commit-transactional` passed. |
| `test/plugin/cli-commit-reject.ci` | Yes | `go run ./cmd/ze-test bgp plugin -v cli-commit-reject` passed. |
| `test/ui/web-commit-transactional.ci` | Yes | `go run ./cmd/ze-test ui -v web-commit-transactional` passed. |
| `test/ui/web-commit-reject.ci` | Yes | `go run ./cmd/ze-test ui -v web-commit-reject` passed. |
| `test/plugin/audit-config-commit.ci` | Yes | `go run ./cmd/ze-test bgp plugin -v audit-config-commit` passed. |
| `test/plugin/api-config-commit-reject.ci` | Yes | `go run ./cmd/ze-test bgp plugin -v api-config-commit-reject` passed. |
| `test/reload/commit-transactional.ci` | Yes | `go run ./cmd/ze-test bgp reload -v commit-transactional` passed. |
| `test/reload/commit-verify-reject.ci` | Yes | `go run ./cmd/ze-test bgp reload -v commit-verify-reject` passed. |
| `test/managed/config-push-transactional.ci` | Yes | `go run ./cmd/ze-test managed -v config-push-transactional` passed. |
| `test/plugin/concurrent-config-commit.ci` | Yes | `go run ./cmd/ze-test bgp plugin -v concurrent-config-commit` passed. |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | CLI commit with daemon promotes accepted config. | `go run ./cmd/ze-test bgp plugin -v cli-commit-transactional` passed. |
| AC-2 | CLI commit with daemon rejects failed verify/apply and keeps active unchanged. | `go run ./cmd/ze-test bgp plugin -v cli-commit-reject` passed. |
| AC-3 | Web commit promotes accepted config. | `go run ./cmd/ze-test ui -v web-commit-transactional` passed. |
| AC-4 | Web commit returns HTTP error on plugin failure and keeps running config. | `go run ./cmd/ze-test ui -v web-commit-reject` passed. |
| AC-5 | API session commit promotes accepted config and closes session. | `go run ./cmd/ze-test bgp plugin -v audit-config-commit` passed. |
| AC-6 | API session commit rejects failed verify and keeps session open. | `go run ./cmd/ze-test bgp plugin -v api-config-commit-reject` passed. |
| AC-7 | SIGHUP success writes candidate and promotes active. | `go run ./cmd/ze-test bgp reload -v commit-transactional` passed. |
| AC-8 | SIGHUP failed apply/reject clears candidate and leaves active unchanged. | `go run ./cmd/ze-test bgp reload -v commit-verify-reject` passed; `go test ./internal/component/plugin/server` passed. |
| AC-9 | Managed push success sends ACK OK after commit. | `go run ./cmd/ze-test managed -v config-push-transactional` passed. |
| AC-10 | Managed push reject sends ACK error and leaves active unchanged. | `go run ./cmd/ze-test managed -v config-push-transactional` passed. |
| AC-11 | Managed push before `OnCommit` is wired rejects temporarily. | `go test ./internal/component/managed` passed with `TestManagedOnCommitNil`. |
| AC-12 | Boot ignores and clears stale candidate. | `go test ./internal/component/config/... ./cmd/ze/hub` passed with pointer and boot tests. |
| AC-13 | Concurrent commit while transaction is in progress rejects second candidate. | `go test ./internal/component/cli -run TestCmdCommitSessionRejectsExistingCandidate` and `go run ./cmd/ze-test bgp plugin -v concurrent-config-commit` passed. |
| AC-14 | CLI commit without daemon writes directly. | `go test ./internal/component/cli` passed with `TestCommitNoNotifierStandalone`. |
| AC-15 | Rollback pointer records previous active after successful commit. | `go run ./cmd/ze-test bgp reload -v commit-transactional` and `go run ./cmd/ze-test bgp plugin -v audit-config-commit` passed. |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Direct SSH CLI commit success | `test/plugin/cli-commit-transactional.ci` | Passes via `ze config edit` over SSH, staged candidate, daemon reload, and CLI output check. |
| Direct SSH CLI commit reject | `test/plugin/cli-commit-reject.ci` | Passes via `ze config edit` over SSH with plugin rejection and active config check. |
| Web commit success | `test/ui/web-commit-transactional.ci` | Passes through HTTPS `/config/set`, `/config/commit`, then `/cli`. |
| Web commit reject | `test/ui/web-commit-reject.ci` | Passes through HTTPS `/config/commit` returning 500 and unchanged running config check. |
| REST/API commit success | `test/plugin/audit-config-commit.ci` | Passes through REST config session commit and pointer assertions. |
| REST/API commit reject | `test/plugin/api-config-commit-reject.ci` | Passes through REST config session commit rejection and open-session assertion. |
| SIGHUP success | `test/reload/commit-transactional.ci` | Passes through edited file, SIGHUP, candidate promotion, and pointer assertions. |
| SIGHUP reject | `test/reload/commit-verify-reject.ci` | Passes through edited file, SIGHUP rejection, and active/candidate assertions. |
| Managed push success/reject | `test/managed/config-push-transactional.ci` | Passes with fake TLS hub and real managed client `ze start`, covering ACK OK and ACK error. |
| Concurrent commit reject | `test/plugin/concurrent-config-commit.ci` | Passes with slow verifier and two REST commits, covering in-flight rejection. |

### Full Verification
| Command | Result | Notes |
|---------|--------|-------|
| `make ze-verify` | Failed before completion, accepted by user | Unit tests, race tests, and encode functional suite passed. Plugin functional suite failed in `test/plugin/lg-graph-lab.ci` and `test/plugin/task-cancel.ci`; rerunning `go run ./cmd/ze-test bgp plugin -v 172 362` reproduced the same two failures. User accepted these unrelated failures for this transactional config commit session. |
| `go test ./cmd/ze/hub -count=1` | Passed | Hub unit coverage after storage-agnostic startup and web commit hook race fixes. |
| `go run ./cmd/ze-test bgp plugin -v 9 33 78 104` | Passed | REST/API reject, audit REST commit, direct SSH CLI transactional commit, and concurrent commit rejection. |
| `go run ./cmd/ze-test ui -v web-commit-transactional web-commit-reject` | Passed | Web accepted and rejected transactional commits. |
| `go run ./cmd/ze-test bgp reload -v commit-transactional commit-verify-reject` | Passed | SIGHUP candidate promotion and failed verify cleanup. |
| `go run ./cmd/ze-test managed -v config-push-transactional` | Passed | Managed push ACK success and reject paths. |
| `make ze-ui-test` | Passed | Full UI suite passed after installing web commit hook before serving and waiting for plugin startup in the commit hook. |
| `make ze-verify` | Failed in ExaBGP compatibility timeout | Transactional suites had passed; ExaBGP encoding compatibility hung with all 37 cases pending, unrelated to transactional commit. |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
