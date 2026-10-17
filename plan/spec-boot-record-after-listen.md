# Spec: boot-record-after-listen

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | - |

## Task

Under fsync load ze stalls before its first listener (plan/journal/startup-wait-expires-under-load.md,
rows 2026-10-09 and 2026-10-17). Owner decision (Thomas, 2026-10-17), three commits:

- A. Record the boot config in the store AFTER the listeners start, not before. The owner lock,
  the SSH host key write and `recoverFileCommit` stay before serving. Every config-change path
  waits for the record (bounded) or is refused with a user-facing error. A failed record is
  reported. A crash between serving and the record is repaired by the next start.
- B. The functional-test DUT uses the in-memory store (`storage.CreateMemory`); tests that
  exercise the disk store keep it through an explicit per-test option.
- C. Trim redundant fsyncs in the storage write path (staging-folder sync in `installBytes`,
  the folder sync of an existing lock in `lockStoreFile`, the `file/active` mirror sync), each
  verified at its producer and against docs/architecture/storage-backends.md.

## Required Reading

### Architecture Docs
- [ ] docs/architecture/storage-backends.md, "Durable publication", "Version pointers and recovery"
  -> Constraint: a durable version precedes its pointer; object before entry.
- [ ] docs/architecture/hub-architecture.md, explicit-file commit authority
- [ ] ai/rules/config.md: env-only is allowed for a bootstrap value read before config parses.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/config_source.go` - initializeConfigSource records the boot config; promote, recover, runtime commit
- [ ] `cmd/ze/hub/main.go` - startup order: recoverFileCommit, clear stale candidate, load, record, listeners, ready file
- [ ] `internal/component/config/storage/source.go` - configStore binding every consumer borrows
- [ ] `internal/component/config/storage/pointer.go` - candidate, promote, active pointer API
- [ ] `internal/component/bgp/config/register.go` - reactor reads candidate then active at start
- [ ] `cmd/ze/ze_core_start.go` - openExplicitStore creates an absent store
- [ ] `internal/component/config/storage/memory.go` - CreateMemory

**Behavior to preserve:** owner lock, host key and recoverFileCommit before serving; ordering of
durable writes; every config commit semantics after the record.

**Behavior to change:** the record moves after the listeners (A); DUT store (B); syncs (C).

- `cmd/ze/hub/main.go` runYANGConfig calls `initializeConfigSource` (config_source.go) after
  config load and kernelcap, before any listener. In file mode it writes candidate + promotes
  (object, entry, candidate pointer, rollback, active pointer, mirror, candidate removal), each
  durable write syncing file and directories.
- Readers of the recorded version during startup: `bgp/config/register.go` (ReadCandidateConfig,
  then ReadActiveConfig) when the reactor starts.
- Writers: SSH editor (`cli/editor_commands.go`, `cli/editor.go`), web editor (`web/editor.go`),
  `commitRuntimeConfig`, `request reload`/SIGHUP (`main_reload.go`), managed (`managed.go`),
  confirm window, data restore. All reach pointer.go through the bound `configStore`.
- `openExplicitStore` (`cmd/ze/ze_core_start.go`) creates an absent store with `storage.Create`.
- Functional-test DUTs written as `exec=ze ... -` with a stdin block are NOT storeless: the
  runner rewrites them into `ze start <WorkDir>/stdin-<name>.conf` (`internal/test/runner/parsing.go`,
  the `daemonCfgIdx` branch; the `stdinRouteDaemonConfig` branch in `runner_exec.go`), so they open a live
  store through openExplicitStore and pay the boot record. test/plugin/filter-irr.ci is one.
  -> Decision: an earlier handoff said otherwise; corrected 2026-10-17.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`ze start <file>` / `ze <file>` -> openExplicitStore -> BindConfigSource -> hub.Run.

### Transformation Path
A: runYANGConfig arms a boot record on the bound handle (`storage.DeferBootRecord`) where it
used to record synchronously; after `writeReadyFile` the record runs once in a lifecycle
goroutine against an ungated view of the same handle. pointer.go functions consult the gate.

### Boundaries Crossed
hub -> config/storage (gate lives on `configStore`, the handle every consumer borrows).

### Integration Points
`configStore` (source.go); pointer.go `WriteCandidateVersion*`, `PromoteCandidate`,
`ClearCandidate`, `EnsureActiveVersion`, `ReadActiveConfig`, `ReadCandidateConfig`.

### Architectural Verification
- [ ] No consumer holds an unbound handle (only hub calls BindConfigSource; every consumer borrows it).

## Risks & Assumptions

### Assumptions
- The boot bytes ARE the accepted config for this run; making them durable is the only work the
  record adds. So a read while the record is pending is answered from those bytes.

### Risks
- A writer reaching the store through a handle that is not the bound one bypasses the gate.

## Blast Radius
Every daemon start; every config commit in the first moments after start.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start <file>` boot, then an SSH/web/API commit | → | `storage.DeferBootRecord`, `BootRecord.Run`, pointer.go gate | `TestBootRecordWriteWaitsThenLands`, `test/plugin/explicit-file-restart.ci` |
| `ze` zetest DUT with `ze.test.storage=memory` | → | `openExplicitStore` selector | `TestStoreSelector*` |

## Acceptance Criteria

| AC | Statement |
|----|-----------|
| AC-1 | A daemon started from an explicit file serves before its boot config is durable in the store |
| AC-2 | A config write (candidate, promote, clear, ensure) arriving before the record completes waits for it, up to 30s, then fails with an error naming the cause and the retry |
| AC-3 | A read of the bound config name while the record is pending answers the boot bytes |
| AC-4 | A failed record is logged at error level with the fix, and every later config write returns that failure |
| AC-5 | The record lands: after it completes, the active pointer names the boot bytes |
| AC-6 | (B) A zetest DUT given `ze.test.storage=memory` and no existing store runs on CreateMemory; a shipped build has no such path |
| AC-7 | (B) Every disk-store .ci test keeps the disk store (listed below) |
| AC-8 | (C) Each trimmed sync carries a comment why, docs updated, ordering unchanged |

## End-to-End User Stories
An operator starts ze on a busy disk; SSH answers at once; a commit typed in the first second
waits for the record and lands.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBootRecordPendingReadAnswersBootBytes` | `internal/component/config/storage/boot_record_test.go` | AC-3 | |
| `TestBootRecordWriteWaitsThenLands` | same | AC-2, AC-5 | |
| `TestBootRecordWaitBound` | same | AC-2 bound | |
| `TestBootRecordFailureRefusesWrites` | same | AC-4 | |
| `TestStoreSelector*` | `cmd/ze` | AC-6 | |

### Boundary Tests (numeric inputs)
- Wait bound: write released at bound minus epsilon proceeds; past the bound refuses.

### Functional Tests
- Existing `test/plugin/explicit-file-restart.ci`, `drain-restart-kind.ci` keep disk (B).

### Interop Tests (Scope: protocol)
Not protocol.

## Files to Modify
- cmd/ze/hub/main.go, cmd/ze/hub/config_source.go
- internal/component/config/storage/source.go, pointer.go
- cmd/ze/ze_core_start.go (B); internal/test/runner (B)
- internal/component/config/storage/tree.go, ownership.go (C)
- docs/architecture/storage-backends.md, docs/architecture/hub-architecture.md, docs/functional-tests.md

## Files to Create
- internal/component/config/storage/boot_record.go, boot_record_test.go
- cmd/ze/register_store_zetest.go (B)

### Integration Checklist
- [ ] gate consulted by every pointer.go entry point

### Documentation Update Checklist (BLOCKING)
- [ ] storage-backends.md "Version pointers and recovery": the boot record and its gate
- [ ] journal rows updated

## Implementation Steps
1. A: boot_record.go + tests; wire hub; docs; commit.
2. B: selector, runner default, per-test disk option, list; commit.
3. C: trims with sync-count test; commit.

### Critical Review Checklist
- [ ] early commit semantics, failure reporting, shutdown waits for the record

### Deliverables Checklist
- [ ] three commits

### Security Review Checklist
- [ ] memory selector unreachable in a shipped build

### Failure Routing
Red .ci after B: check whether it needs the disk option before touching code.

## Design Insights
Reads need not wait: the boot bytes are the accepted config before they are durable.

## Key Design Decisions
- Early writes WAIT (bounded 30s) rather than refuse at once: the record normally takes
  milliseconds, so an immediate refusal would make every early commit (and every test that
  commits right after ready) flaky; past the bound the operator gets a refusal naming the cause.
- The gate lives in the storage handle, not at each hub entry point: seven writers in four
  packages reach pointer.go, and only the handle is common to all of them.
- A failed record does not stop a running daemon: it keeps routing on the config it loaded,
  and refuses config changes with the record's error until restarted.

## Known Limitations
- Raw-key readers (`ze data cat`, backup) see the store as it is while the record is pending.
- The gate lives in the daemon's handle. Read-only readers in another process that open the
  store themselves (`withRuntimeStore` in `config/cli/config_data.go`, `cmd_history.go`,
  `cmd_diff.go`) read the previous active configuration, or none on a first start, until the
  record lands.
- The record starts before the plugin wait, not after the listeners: SSH binds during plugin
  startup with BGP; web, LG, MCP and managed bind after WaitForStartupComplete. It runs beside
  the rest of startup, so no listener waits for it.

## RFC Documentation (Scope: protocol)
Not protocol.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] design decided by the owner 2026-10-17

### Goal Gates (MUST pass)
- [ ] stall measured before/after under a concurrent fsync writer
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] /ze-close
