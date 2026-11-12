# Spec: plugin-restart-bounded-and-configuration-true

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | `request plugin restart` is the fix line of five `ze doctor` rows. A restart that resurrects a plugin the configuration removed, stalls every reader of the process manager for up to 30 s, or leaves a plugin down after a refusal is a shipped-daemon lifecycle defect an operator meets in production (`plan/README.md`, `release/fixes/`) |
| RFC gaps | - |
| Depends | Found by `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md`, which needs nothing from this spec. Shares files with `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` (`Respawn`, `restartPlugin`, `applyFailurePolicy`, the stop records): whichever lands second is designed against the other's code |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Goal, in the owner's terms:** an operator can restart any configured plugin,
running or not, and gets an answer within a bound. The restart does not stall
the daemon, and it does not bring back what the configuration removed.

`request plugin restart <name>` was changed on 2026-11-12 to restart a running
plugin (909da87de2, ce86ac4a64, ae5d1d84fd, 0becf0044f), because five
`ze doctor` rows prescribe a restart for a plugin that runs. Two independent
reviews then found lifecycle defects around it. A hardening change landing the
same day fixes the worst of them: the restart refuses its own caller and a
pending removal, and it bounds the waits for the old process's cleanup and the
replacement's handshake. When a wait expires, the restart fails cleanly and
writes a "cleanup owed" record (`lateCleanupRecord`), which refuses later
restarts until that cleanup has run. The process's metric callbacks became one
atomically swapped set (`metric_hooks.go`). This spec owns what the reviews
found and the hardening did not fix. Each item is one journal row
(Owned journal rows):

| # | Defect | Where |
|---|--------|-------|
| 1 | "Configured" never shrinks. A plugin a committed reload removed is still offered and accepted by the restart and respawned from its old config. A plugin with no process when its roots go keeps its `doctor-plugin-stopped` row | `ProcessManager.configs`, `ConfiguredNames`, `Respawn`; `completePluginRestart`, `pluginRestartRefusal`; `stopCollectedProcesses` |
| 2 | Seen under full-package load, not reproduced on demand: a rejected reload's recovery may stop a dependency that ran before the reload | `restoreRemovedPlugin` -> `stopOrphanedDependencies` -> `collectOrphanedDependencies` |
| 3 | `Respawn` holds the manager's write lock across the old process's join and the replacement's start, connect-back included | `ProcessManager.Respawn`, `Process.StopJoinBound`, the 30 s connect-back wait in `process.go` |
| 4 | A failure-policy restart runs without the transaction lock, and the cleanup-owed record keeps one process per name | `applyFailurePolicy` -> `restartPlugin`; `lateCleanupRecord.owe`, `owed` |
| 5 | The respawn-limit path deletes the status label while the process stays live | `Respawn`, `clearProcessCallbacks`, `deletePluginStatusLabel`, `clearMetricHooks` |
| 6 | The bus warning names a fix that is refused while a cleanup is owed | `recordStoppedLocked`, `RestartCommand`; `pluginStoppedRow` |
| 7 | A refused restart still lifts the respawn-limit disable | `handlePluginRestart`, `ProcessManager.Reenable` |
| 8 | The 3 s cleanup bound may be too short for a plugin with a large route table. Unmeasured | `awaitReplacedCleanup`, `restartCleanupGrace`, `withdrawPluginRoutes` |
| 9 | Unverified: a restart during the boot window, before step (d) starts the runtime handler | `awaitReplacedCleanup`, `runPluginPhaseWithin` |
| 10 | Same family as item 1: a plugin that a refused reload auto-loaded, and whose handshake failed, keeps its stop record. The record is written after the rollback removed its process, and the reload's stop skips a name with no process | `runPluginPhaseWithin` (`RecordStopped` with `StopHandshakeFailed`); `startAutoLoaded` -> `autoStopPluginNames` -> `stopCollectedProcesses` |

For item 1, a fix was tried in `stopCollectedProcesses` and withdrawn: that
function also stops the plugins a failed or rolled-back reload started, and
those were never removed from the configuration. The finding is that the name
leaves where a reload's removal becomes final
(`reloadAcceptance.complete(true)` -> `finishRemovalScope`), taken from the
names the committed-removal sites record in the scope, a name with no process
included. Which shape that takes is design.

Item 3 cannot simply narrow the lock. `applyFailurePolicy` of the old
generation blocks in `GetProcess` until the slot is swapped, and only then
sees it was replaced and returns. Its correctness rests on that.

This is a skeleton: idea capture. No design is written here.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - "Failure-Policy Declaration (Stage 1)", "Shutdown", "A failure during a reload means the plugin failed"
  → Constraint: a removal owns its own recovery; one plugin failure stops the startup tier (owner ruling)
- [ ] `docs/architecture/doctor-and-health-checks.md` - the `plugins` row, `doctor-plugin-stopped`, `doctor-plugin-restart-overlap`
- [ ] `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` - its defect 2 (a replacement published before its handshake) and its retire step change the same `Respawn` and supervisor paths
  → Constraint: owner decision D1 there: no new owner object, no generation counter; the process pointer is the generation

**Key insights:**
- The process pointer is the generation identity. `applyFailurePolicy` returns when `GetProcess(name) != proc`.
- `cleanupProcess` runs once per generation and releases some state by name (`withdrawPluginRoutes` among them), so a replacement must not register before its predecessor's cleanup ran. `lateCleanupRecord` enforces that across restarts.

## Current Behavior (MANDATORY)

**Source files read (working tree, with the hardening uncommitted):**
- [ ] `internal/component/plugin/process/manager.go` - the `configs` comment: "Nothing drops an entry, so a plugin a reload removed stays listed". `ConfiguredNames` lists `pm.configs`. `GetProcess` takes the read lock. `Respawn` takes `pm.mu.Lock` for its whole body, finds the config in `pm.configs`, and on the limit paths calls `clearProcessCallbacks` and `deletePluginStatusLabel` without stopping the process. Otherwise it stops the old process and joins it for `StopJoinBound` (`recordRestartOverlap` on expiry), then calls `StartWithContext` on the replacement and stores it, all under the lock. `RemoveProcess` clears the stop record
- [ ] `internal/component/plugin/process/process.go` - the forked start waits for the TLS connect-back with `context.WithTimeout(p.ctx, 30*time.Second)`. `StopJoinBound` is the exit grace plus `respawnJoinMargin`: 2 s for a forked plugin on Linux, 1 s in the daemon
- [ ] `internal/component/plugin/process/metric_hooks.go` - `clearMetricHooks` stores nil. Its comment says "A reader that loaded the set before this call may still make one last call through it"
- [ ] `internal/component/plugin/process/stopped.go` - `Reenable` deletes `disabled`, `respawnTimes` and `totalRespawns` for the name. `recordStoppedLocked` raises `doctor-plugin-stopped` with `RestartCommand(name)` as its fix, always. `PluginStop` holds Cause, Detail, Policy, ExitStatus and Time
- [ ] `internal/component/plugin/process/doctor_records.go` - `recordRestartOverlap` raises `doctor-plugin-restart-overlap`
- [ ] `internal/component/plugin/server/register_plugin_restart.go` - `completePluginRestart` offers every `ConfiguredNames` entry. `pluginRestartRefusal` accepts any of them. `handlePluginRestart` refuses a wrong count, no server, no manager, an unconfigured name, its own caller, a held `txLock` and a pending removal, then calls `pm.Reenable(name)`, then `restartPlugin`
- [ ] `internal/component/plugin/server/reload_tx.go` - `restartPlugin` records `StopRestartFailed` on any error. `restartPluginProcess` refuses when `lateCleanups.owed`, then calls `Respawn`, `awaitReplacedCleanup` (on expiry it rolls the replacement back), `releasePluginRegistrations` and `restartHandshake`. `reloadAcceptance.complete` calls `finishRemovalScope`
- [ ] `internal/component/plugin/server/restart.go` - `restartCleanupGrace` is `txShutdownGrace` (3 s). `awaitReplacedCleanup` returns at once below `StageRunning`, and otherwise waits and calls `lateCleanups.owe(old)` on expiry. `lateCleanupRecord` is a map from name to one process. `owe` overwrites. `owed` probes that one process and forgets it once its cleanup ran
- [ ] `internal/component/plugin/server/failure_policy.go` - `applyFailurePolicy` returns when `GetProcess(name) != proc`, and for `restart` calls `restartPluginAfterExit` (`reload_tx.go`), which takes no `txLock` and lets the replacement's handshake run as long as the daemon does
- [ ] `internal/component/plugin/server/startup_removal.go` - `finishRemovalScope` deletes the accepted, stopped removals of the scope and sends the rest to `recoverPluginRemoval`, whose goroutine waits for the attempt and the scope, takes `txLock` and runs `restoreRemovedPlugin`. That function calls `stopOrphanedDependencies` when the process's roots are gone from the tree and no block configures it
- [ ] `internal/component/plugin/server/startup_autoload.go` - `stopCollectedProcesses` skips a name whose `GetProcess` is nil. `collectOrphanedDependencies` adds a dependency that is not configured, has a process, and has no live dependent outside the set. It does not ask whether that dependency predates the reload
- [ ] `internal/component/plugin/server/startup.go` - `runPluginPhaseWithin` refuses a phase for a name with a cleanup owed. Step (d), after the last tier, starts `superviseProcess` for every process of `allProcesses` at `StageRunning` or above
- [ ] `internal/component/plugin/server/dispatch_route.go` - `withdrawPluginRoutes` calls `rib.Remove` once per installed route
- [ ] `internal/component/plugin/doctor/checks_runtime.go` - when `cleanupOwed`, `pluginStoppedRow` says the restart is refused until the cleanup has finished, and names the daemon restart as the last resort (`daemonRestartLastResort`)
- [ ] `internal/component/plugin/server/startup.go` - on a failed handshake, `runPluginPhaseWithin` and `rollbackNonRunningStartupProcesses` call `RecordStopped` with `StopHandshakeFailed` after `rollbackStartupProcess` has removed the process. `startup_autoload.go` `startAutoLoaded` then calls `autoStopPluginNames`, whose `stopCollectedProcesses` skips a name whose `GetProcess` is nil (item 10)
- [ ] `internal/component/plugin/server/reload_test.go` - `TestReloadFailureKeepsPreExistingDependency`: a pre-existing, unconfigured dependency added with `AddProcess`, and a dependent auto-loaded by the reload that another plugin rejects. It failed in two full-package `-race` runs with all feature tags on 2026-11-12 (`tmp/session/2026-11-12-005413/scratch/job-harden1-d1f3dd49.log`, `job-round3-e71f6e7a.log`) and passed 200 of 200 runs alone

**Behavior to preserve:**
- The refusals `handlePluginRestart` makes today, and the restart of a running plugin
- `applyFailurePolicy` ignores a generation the name no longer holds
- The cleanup-owed refusal of a restart and of a startup phase
- A plugin that a failed or rolled-back reload started is stopped, and one the reload removed is recovered when the reload is rejected
- Counters (`restarts`, `delivered`) survive a disable. Only the status gauge is deleted

**Behavior to change:**
- Items 1 to 9 of the Task, as the acceptance criteria state them

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Removal scope | `internal/component/plugin/server/reload_tx.go` `reloadAcceptance.complete`; `internal/component/plugin/server/startup_removal.go` `finishRemovalScope` | Makes a reload's removals final when the reload is accepted, and recovers them when it is rejected | Extends (item 1): the point where a removed name stops being configured |
| Process manager slots and configs | `internal/component/plugin/process/manager.go` `ProcessManager.configs`, `processes`, `Respawn`, `RemoveProcess`, `ConfiguredNames` | Holds every name it was ever given, and one process slot per name | Extends (items 1, 3, 5) |
| Generation identity | `GetProcess(name) != proc` in `applyFailurePolicy` | Tells a superseded generation from the live one | Preserved (item 3): any narrowing of the lock keeps this answer |
| Cleanup-owed record | `internal/component/plugin/server/restart.go` `lateCleanupRecord`, `awaitReplacedCleanup` | Refuses a restart or a phase while a stopped process of the name owes its cleanup | Extends (items 4, 8, 9) |
| Transaction lock | `txLock` in `handlePluginRestart`, `recoverPluginRemoval` | Serializes operator restarts, reloads and removal recovery | Extends (item 4): the failure-policy restart reaches the same exclusion, by a shape the design chooses |
| Stop record and bus warning | `internal/component/plugin/process/stopped.go` `PluginStop`, `recordStoppedLocked`, `RestartCommand`; `internal/component/plugin/doctor/checks_runtime.go` `pluginStoppedRow` | One record per down plugin; the bus and the doctor row share its message | Extends (item 6): one declaration of the fix, read by both surfaces |
| Metric hook set | `internal/component/plugin/process/metric_hooks.go` | One atomic set of callbacks, swapped by the manager | Extends (item 5) |
| Orphaned-dependency stop | `internal/component/plugin/server/startup_autoload.go` `collectOrphanedDependencies`, `stopOrphanedDependencies` | Stops dependencies left without a dependent | Extends (item 2), once reproduced |
| Neighbour: plugin-process lifecycle | `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` (status `ready`) | Owns the replacement published before its handshake, the unrestarted exit and the stop-signal refusal, over the same `Respawn` and supervisor | Does not apply as an owner: its design is approved and owns other rows. Coordinate, same files |
| Neighbour: reload reconciliation | `plan/release/fixes/spec-single-owner-reload-reconcile.md` (status `design`) | Owns reload and rollback reconciliation in general | Does not apply as an owner of items 1 and 2: they are plugin-process removal state, not a participant's applied state. Coordinate |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Operator `request plugin restart <name>`; a plugin exit under failure policy `restart`; a reload removal or its rejection]

### Transformation Path
1. [Written at design]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [Written at design] | | No |

### Changed-Wire Census

**Search evidence:** [Written at design. Expected: no wire change]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [disposition] |

### Integration Points
- [Written at design]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Item 2 follows the path `collectOrphanedDependencies` takes, and is not a test artefact | Source reading of `collectOrphanedDependencies`; two reds in full-package `-race` runs with all feature tags on 2026-11-12 ("failed reload must keep pre-existing dependency"), 200 of 200 green alone | The fix lands where the reproduction shows the real cause | An on-demand reproduction that drives the path deterministically | unvalidated |
| A-2 | Step (d) of `runPluginPhaseWithin` runs the old process's cleanup after a boot-window restart | `allProcesses` holds the old pointer, and its stage stays at or above running | A cleanup owed forever, which refuses every later restart | Reproduce or refute (item 9) | unvalidated |
| A-3 | The 3 s cleanup bound can be too short | `withdrawPluginRoutes` removes one route per call | Item 8 closes as measured and sufficient | Measure the cleanup on a large table | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Narrowing `Respawn`'s lock breaks the generation test of `applyFailurePolicy` | A superseded generation applies its policy to the replacement | The design states how the old supervisor still learns it was replaced |
| R-2 | Dropping a removed name also drops one a rejected reload restores | A restored plugin refused by the restart command | Only accepted, final removals drop a name (the withdrawn `stopCollectedProcesses` attempt) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Plugin restarts, reload removal and recovery, and every reader of the process manager |
| How is it reverted? | Single commit revert per item |
| Who else touches this path? | `spec-single-owner-lifecycle-1-plugin-process`, `spec-single-owner-reload-reconcile` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `request plugin restart <name>` through `ze cli` | → | `handlePluginRestart` -> `restartPlugin` | [named at design; `test/plugin/plugin-restart-external-running.ci` exists for the running case] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A committed reload removes plugin X, with X running, and again with X already down and holding no process | Completion offers the names still configured. `request plugin restart X` answers that X is not configured. `ze doctor` reports no stopped row for X | A plugin stopped by a failed or rolled-back reload stays configured and restartable, because the configuration still names it |
| AC-2 | Item 2 | First, the failure is reproduced on demand (it fails under full-package load and passes alone), and the reproduction is recorded. Then it is fixed: a dependency that ran before a rejected reload runs after its recovery | A dependency that the rejected reload itself started is stopped, because nothing configured needs it |
| AC-3 | A restart of a forked plugin while its replacement waits for connect-back | The doctor snapshot and every `GetProcess`, `AllProcesses` and `ConfiguredNames` caller answer within a bound that does not include the connect-back wait. The old generation's failure policy still sees it was replaced. An in-daemon engine that reads the manager on its exit raises no `doctor-plugin-restart-overlap` | A reader that needs the replaced slot itself waits for the swap, because the generation test depends on it |
| AC-4 | A failure-policy restart and an operator restart of the same plugin at once | One restart runs and the other is refused or follows it. The cleanup-owed refusal holds while any stopped process of the name still owes its cleanup | The second request is answered with its refusal, because two generations cannot take one name |
| AC-5 | The respawn limit disables a live plugin | The status gauge label stays deleted whatever stage transition was in flight | The `restarts` and `delivered` counters stay, as the existing comment requires |
| AC-6 | A restart failed because a cleanup is owed | The bus warning and the doctor row name the same fix, from one declaration | None: both surfaces say the same thing |
| AC-7 | A restart refused after the operator's order (cleanup owed, daemon stopping) | The respawn-limit disable and the respawn counters are as they were before the command. A restart that runs lifts them | None: lifting them belongs to a restart that runs |
| AC-8 | Restart of a plugin holding a large route table | The cleanup time is measured and recorded. The restart answers within its bound and succeeds with one command | A restart whose old process holds a call that never returns fails with the cleanup-owed answer, because that call ends only when ze restarts |
| AC-10 | A reload auto-loads a plugin whose handshake fails, and the reload is refused | `ze doctor` reports the refused reload and no `doctor-plugin-stopped` row for that plugin, and the restart command answers that it is not configured | A plugin the running configuration names, whose handshake failed, keeps its row, because the configuration still runs it |
| AC-9 | `request plugin restart` during the boot window | First, reproduced or refuted. The restart answers within its bound, and once boot completes no cleanup is owed for the name | The restart may be refused during boot with a message naming when to retry, if the design chooses that |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | `internal/component/plugin/process/`, `internal/component/plugin/server/` | AC-1 to AC-9 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A: no numeric operator input | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | `test/plugin/` | AC-1, AC-3, AC-7 through `ze cli`, internal and forked (`ai/rules/testing.md`, "Lifecycle Regressions") | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible change.

## Files to Modify
- `internal/component/plugin/process/manager.go` - items 1, 3, 5
- `internal/component/plugin/process/stopped.go` - items 6, 7
- `internal/component/plugin/server/register_plugin_restart.go` - items 1, 7
- `internal/component/plugin/server/restart.go` - items 4, 8, 9
- `internal/component/plugin/server/failure_policy.go` - item 4
- `internal/component/plugin/server/startup_removal.go` - items 1, 2
- `internal/component/plugin/server/startup_autoload.go` - item 2
- [completed at design]

## Files to Create
- [named at design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| CLI commands/flags | | |
| Functional test for new RPC/API | | |
| Doctor check for runtime dependencies | | |
| Doctor code per problem class | | |
| Prometheus counters/metrics | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 12 | Internal architecture changed? | | `docs/architecture/api/process-protocol.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [written at design]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, or a recorded refutation for AC-2 and AC-9 |
| Correctness | The generation test of `applyFailurePolicy` holds under any lock change |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `retained-entry-still-answers-lookups.md` | 2026-11-12 | item 1: `ConfiguredNames` keeps a removed plugin |
| `reconciler-claims-state-another-owner-installed.md` | 2026-11-12 | item 2: rejected reload's recovery may stop a pre-existing dependency |
| `lock-held-across-a-join.md` | 2026-11-12 | item 3: `Respawn` holds `pm.mu` across join and connect-back |
| `two-owners-share-one-name.md` | 2026-11-12 | item 4: failure-policy restart without `txLock`; one cleanup-owed process per name |
| `lifetime-ends-before-its-last-reader.md` | 2026-11-12 | item 5: respawn-limit path deletes the label of a live process |
| `fix-command-refused-in-the-state-it-reports.md` | 2026-11-12 | item 6: bus warning names a refused fix |
| `record-written-before-the-operation-succeeds.md` | 2026-11-12 | item 7: `Reenable` before the restart's refusals |
| `bound-too-small-for-its-own-burst.md` | 2026-11-12 | item 8: 3 s cleanup bound against a large route table |
| `new-caller-assumes-the-old-callers-timing.md` | 2026-11-12 | item 9: restart in the boot window |
| `retained-entry-still-answers-lookups.md` | 2026-11-12 | item 10, same family as item 1: a refused reload's failed auto-load keeps its stop record |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [Written at design]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated, or refuted with recorded evidence where the AC asks for it
- [ ] Targeted tests pass: `./le test unit pkg path ./internal/component/plugin/process/`, `./le test unit pkg path ./internal/component/plugin/server/`, and `./le test functional plugin` on the restart and reload tests this spec names. No agent runs any `./le verify` command
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior, internal and forked

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-plugin-restart-bounded-and-configuration-true.md` only, in the same `./le commit create` script
