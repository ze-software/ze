# Spec: plugin-restart-bounded-and-configuration-true

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P1 |
| Release reason | `request plugin restart` is the fix line of five `ze doctor` rows. A restart that resurrects a plugin the configuration removed, stalls every reader of the process manager for up to 30 s, or leaves a plugin down after a refusal is a shipped-daemon lifecycle defect an operator meets in production (`plan/README.md`, `release/fixes/`) |
| RFC gaps | - |
| Depends | Found by `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md`, which needs nothing from this spec. Shares files with `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` (`Respawn`, `restartPlugin`, `applyFailurePolicy`, `collectProcessesForRemovedConfigPaths`, the stop records): whichever lands second is designed against the other's code. Item 1's first half (`spec-fib-dummy` AC-17, `UnconfigureProcess` at `finishRemovalScope`) landed in `6da64fc95a` and closed in `4cbf5dbf47`; this spec keeps the second half (no process when roots go), designed against that landed code |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-11-25 |

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
| 1 | "Configured" never shrinks. A plugin a committed reload removed is still offered and accepted by the restart and respawned from its old config. A plugin with no process when its roots go keeps its `doctor-plugin-stopped` row. Split on 2026-11-24: the first half, a plugin with a process that an accepted reload stopped and that stays in `ConfiguredNames`, is owned by `spec-fib-dummy` (its AC-17). This spec keeps the second half, a plugin with no process when its roots go (AC-1) | `ProcessManager.configs`, `ConfiguredNames`, `Respawn`; `completePluginRestart`, `pluginRestartRefusal`; `stopCollectedProcesses` |
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

Item 1 is split since 2026-11-24. `spec-fib-dummy`
drops the name at `finishRemovalScope` for a plugin whose stop an accepted
reload made final (its AC-17: the engine-added writer it stops, and the owner
of a removed config root), because after its AC-5 one `request plugin
restart` would start the kernel writer beside `fib-dummy`. After that fix,
completion no longer offers such a name and the restart refuses it. What
stays here is the name the scope never holds: a plugin with no process when
its roots go, which `stopCollectedProcesses` skips, so its name stays
configured and its `doctor-plugin-stopped` row stays raised. This half is
designed against spec-fib-dummy's `finishRemovalScope` once that lands, and
extends it rather than adding a second place where a name leaves.

Item 3 cannot simply narrow the lock. `applyFailurePolicy` of the old
generation blocks in `GetProcess` until the slot is swapped, and only then
sees it was replaced and returns. Its correctness rests on that.

This is a skeleton: idea capture. No design is written here.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/le-tooling.md` (read 2026-11-25)
  → Constraint: every plugin runs internal or forked; every lifecycle fix reached in both modes must be proven in both (`ai/rules/testing.md`, "Lifecycle Regressions")
  → Constraint: engine and config code never enumerate plugin names; removal and cleanup-owed tracking derive from registrations and process pointers
  → Constraint: external `bus > endpoint` declarations and their schemas are fixed while the daemon runs (`RefuseChangedDeclarations`)
- [ ] `docs/architecture/api/process-protocol.md` - "Failure-Policy Declaration (Stage 1)", "Shutdown", "A failure during a reload means the plugin failed", "Design decisions" (read 2026-11-25)
  → Constraint: a removal owns its own recovery and remains provisional until the whole reload is accepted; one plugin failure stops the startup tier (owner ruling)
  → Constraint: each configured name has one restart controller (`restartControllerSet`) through process replacement, outgoing cleanup, startup, and result recording; `Respawn` withdraws the outgoing generation before `Stop` and retains physical ownership through its actual join
  → Constraint: an operator or rollback restart holds `txLock` and bounds every wait; a failure-policy restart holds no `txLock` and runs its replacement's handshake under the daemon lifetime unless the design changes that contract
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Plugin state a runtime check reads", "Plugin lifecycle checks", `doctor-plugin-stopped`, `doctor-plugin-restart-overlap` (read 2026-11-25)
  → Constraint: `plugin-stopped` declares `doctor-plugin-stopped` at the runtime phase, so `recent-events` deduplicates it in `show doctor`, while `ze show warnings` and the daemon log display the report-bus warning directly; both surfaces must agree on the fix
  → Constraint: `UnconfigureProcess` drops a finalized removal's name from `ConfiguredNames`, respawn history, disable state, stop record, and status gauge label, and is a no-op while a live process holds the slot
- [ ] `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` (status `design`, read 2026-11-25)
  → Constraint: owner decision D1 there: no new owner object, no generation counter; the process pointer is the generation
  → Constraint: owner decision D-B there (2026-11-24): reloads will restart configured down plugins from `pm.stopped`, so leaked `pm.stopped` entries from refused reload auto-loads (item 10) or removed down plugins (item 1) must not remain behind

**Key insights:**
- The process pointer is the generation identity. `Respawn` already withdraws `old` from `pm.processes[name]` under `pm.mu` before calling `old.Stop()` (landed in `bc532d7502`), so `applyFailurePolicy` returns when `GetProcess(name) != proc` without holding `pm.mu` across join or connect-back.
- `Server.restartControllers` (`restart_controller.go`, landed in `d73a860caf`) already serializes operator restarts and failure-policy restarts per plugin name, but does not serialize failure-policy restarts against `reloadConfig` (`txLock` and shared `s.coordinator`), and `lateCleanupRecord` still stores only one `*process.Process` per name.
- `cleanupProcess` runs once per generation and releases name-keyed state (`withdrawPluginRoutes`, cleanup hooks), so a replacement must not register before every stopped predecessor's cleanup ran.
- `spec-fib-dummy` (`6da64fc95a`) added `ProcessManager.UnconfigureProcess` at `finishRemovalScope` and `rollbackStartupProcess`, but `s.removals` requires a non-nil `*process.Process`, and `startup.go` still writes `RecordStopped` after `rollbackStartupProcess` unconfigured a failed reload auto-load.
- During a BGP daemon boot, `infraSetup` opens and wires the SSH listener inside `bgp` Stage 2 (`OnConfigure`), before Phase 1 Step (d) starts `superviseProcess` and before Phases 2–3 run, making the boot-window restart reachable in production.

## Current Behavior (MANDATORY)

**Source files verified directly at their producing functions (2026-11-25):**
- [ ] `internal/component/plugin/process/manager.go` - `UnconfigureProcess` (lines 660-674, added by `6da64fc95a`) drops `pm.configs`, `disabled`, `respawnTimes`, `totalRespawns`, `stopped` + bus warning, and status gauge label when `pm.processes[name] == nil`. `Respawn` (lines 829-963, updated by `bc532d7502`) holds `pm.mu` ONLY for admission/bookkeeping and withdrawing `old` from `pm.processes[name]`, then unlocks `pm.mu` before `old.Stop()`, `old.Wait(joinBound)`, and `startAdmitted` (30s connect-back); however, `pm.respawnMu` (lines 114, 830-831) is still a manager-wide lock held across the entire join and connect-back, serializing unrelated plugin names. Both respawn-limit branches (lines 885-907) still call `clearProcessCallbacks(name)` and `deletePluginStatusLabel(name)` without stopping `proc` or synchronizing with an in-flight `SetStage` reader (item 5)
- [ ] `internal/component/plugin/process/metric_hooks.go` - `noteStageChange` (lines 28-32) loads `p.metricHooks` and calls `hooks.stageChange(stage)` (`m.status.With(name).Set(...)`) with no synchronization against `clearMetricHooks` (`Store(nil)`) + `deletePluginStatusLabel`, so an in-flight `SetStage` re-creates `ze_plugin_status{plugin=name}` after deletion (item 5)
- [ ] `internal/component/plugin/process/stopped.go` - `Reenable` (lines 93-99) clears `disabled`, `respawnTimes` and `totalRespawns`. `recordStoppedLocked` (lines 112-130) always raises `doctor-plugin-stopped` on the report bus and logs WARN with `StoppedMessage` and `fix = RestartCommand(name)`, carrying no cleanup-owed state or last-resort remedy (item 6)
- [ ] `internal/component/plugin/server/register_plugin_restart.go` - `completePluginRestart` (lines 53-83) offers every `ConfiguredNames` entry; `pluginRestartRefusal` (lines 249-256) runs at line 136 before `txLock.tryAcquire()` at line 145; `handlePluginRestart` calls `pm.Reenable(name)` at line 164 BEFORE `restartPluginReplacement`, which can still refuse on `restartControllers.tryAcquire`, `lateCleanups.owed`, or `Respawn` pre-spawn checks (`ErrDaemonStopping`, `ErrProcessNotFound`, not started) (item 7). It has no guard against running during the boot window before `startupComplete` (item 9)
- [ ] `internal/component/plugin/server/restart_controller.go` - `restartControllerSet` (`tryAcquire`, `wait`, `release`, lines 15-81, added by `d73a860caf`) serializes operator/rollback restarts and failure-policy restarts per plugin name (part of item 4 already fixed)
- [ ] `internal/component/plugin/server/reload_tx.go` - `restartPluginReplacement` (lines 1076-1084) uses `restartControllers.tryAcquire`; `restartPluginAfterExit` (lines 1092-1108) uses `restartControllers.wait` and rechecks `pm.GetProcess(name) != outgoing`, still taking NO `txLock` (races `reloadConfig` on `s.coordinator`, config tree, and removals; item 4). `restartReplacementWithin` (lines 1119-1131) calls `pm.RecordStopped(StopRestartFailed)` on every error including pre-spawn refusals
- [ ] `internal/component/plugin/server/restart.go` - `restartCleanupGrace` is `txShutdownGrace` (3 s, line 38). `lateCleanupRecord` (lines 160-194) still uses `map[string]*process.Process`; `owe` overwrites earlier processes and `owed` deletes the entry once the latest finishes (item 4). `runUnbarrieredStartupHandshake` (lines 256-267) globally saves, nils, and restores `s.coordinator`, clobbering concurrent tier handshakes during boot or reload auto-load (items 4 and 9)
- [ ] `internal/component/plugin/server/reload_dataplane_writer.go` & `internal/component/plugin/server/reload.go` - `collectReloadStopProcesses` (called at `reload.go:464, 544, 607`) calls `collectNonFIBRemovedProcesses` (`:360-391`), `judgeReloadWriters` (`:207-356`), and `collectOrphanedDependencies` (`startup_autoload.go:601-663`); all three skip any plugin with `pm.GetProcess(name) == nil` (item 1)
- [ ] `internal/component/plugin/server/startup_removal.go` - `s.removals` is `map[*process.Process]*pluginRemoval`; `finishRemovalScope` (lines 285-314) calls `pm.UnconfigureProcess` only for stopped `*process.Process` entries in `s.removals` (item 1). When `reload.go:596` calls `autoStopPluginNames` for rolled-back candidate auto-loads while `s.txLock.acceptance != nil`, `notifyPluginRemoval` inserts them into `s.removals` under `scope`; `finishRemovalScope(scope, false)` then spawns background `recoverPluginRemoval` -> `restoreRemovedPlugin` (lines 396-445) -> `stopOrphanedDependencies`, which stops pre-existing unconfigured dependencies (item 2). `rollbackStartupProcess` (lines 135-146) calls `pm.UnconfigureProcess` for below-running reload auto-load failures
- [ ] `internal/component/plugin/server/startup_autoload.go` - `stopCollectedProcesses` (lines 493-546) skips `GetProcess == nil`. `collectOrphanedDependencies` (lines 601-663) spares a candidate only on `hasConfiguredPlugin` or another live dependent, ignoring pre-existing provenance or independent config/family/data-plane need (item 2)
- [ ] `internal/component/plugin/server/startup.go` - on failed reload auto-load below `StageRunning`, lines 667 and 759 unconditionally call `pm.RecordStopped(StopHandshakeFailed)` AFTER `rollbackStartupProcess` already called `UnconfigureProcess`, leaking `pm.stopped` and the `doctor-plugin-stopped` bus warning; on `runProcessGenerationReady` failure at `StageRunning` (lines 705-712), `rollbackReplacement` never calls `UnconfigureProcess` and line 709 calls `RecordStopped`, leaking both `pm.configs` and `pm.stopped` (item 10). Step (a) stores `s.procManager` at line 583; Step (d) spawns `superviseProcess` at lines 690-697, which calls `pm.ClearStopped` at `failure_policy.go:44` (item 9)
- [ ] `cmd/ze/hub/infra_setup.go`, `cmd/ze/hub/service_ssh.go`, `internal/component/bgp/plugin/register.go` - during Phase 1 Stage 2 (`bgp` `OnConfigure`), `infraSetup` opens the SSH listener and wires its executor to `apiServer.Dispatcher()` before Phase 1 Step (d) and before Phases 2–3 run, making `request plugin restart` reachable during the boot window (item 9)
- [ ] `internal/component/plugin/server/dispatch.go`, `dispatch_route.go`, `internal/core/rib/locrib/manager.go`, `internal/core/rib/store/store_bart.go` - `cleanupProcess` runs `withdrawPluginRoutes` (serial per-route `rib.Remove`, which calls `fam.String()`, acquires the shard lock, and executes BART `trie.Insert` even before `trie.Delete` on empty groups) plus 4 generation cleanup hooks (`sourceGenerationCleanup` has three sequential 35s-bounded steps) and 3 name cleanup hooks inside the 3 s `restartCleanupGrace` window; no benchmark exists for `withdrawPluginRoutes` or `RIB.Remove` (item 8)
- [ ] `internal/component/plugin/doctor/checks_runtime.go` - `pluginStoppedRow` (lines 85-102) adds the cleanup-owed explanation and `daemonRestartLastResort()` to `Message` while keeping `Fix: process.RestartCommand(name)` (pinned by `checks_runtime_test.go:127-129`); `recent-events` suppresses the bus warning in `show doctor` via `ownedCodes`, so the discrepancy appears in `ze show warnings` and daemon logs (item 6)
- [ ] `internal/component/plugin/server/reload_test.go` - `TestReloadFailureKeepsPreExistingDependency` (lines 586-667) checks `pm.GetProcess(depName)` immediately after `ReloadConfig` returns without waiting for `recoverPluginRemoval`'s background goroutine (`s.wg`), which is why it passed alone and failed under `-race` full-package load (item 2)

**Behavior to preserve:**
- The refusals `handlePluginRestart` makes today, and the restart of a running plugin
- `applyFailurePolicy` ignores a generation the name no longer holds
- The cleanup-owed refusal of a restart and of a startup phase
- A plugin that a failed or rolled-back reload started is stopped, and one the reload removed is recovered when the reload is rejected
- Counters (`restarts`, `delivered`, `failed`, `slow`) survive a disable. Only the status gauge is deleted
- `UnconfigureProcess` on finalized removals with a process (`spec-fib-dummy` AC-17) continues to drop the name only when the reload is accepted, never when a deferred reload scope is rejected

**Behavior to change:**
- Items 1 (second half), 2, 3 (residual `respawnMu` / proof tests), 4 (remaining two halves), 5, 6, 7, 8, 9, and 10, as the design and acceptance criteria state them

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Removal scope and unconfigure | `internal/component/plugin/server/reload_tx.go` `reloadAcceptance.complete`; `internal/component/plugin/server/startup_removal.go` `finishRemovalScope`; `internal/component/plugin/process/manager.go` `UnconfigureProcess` | Finalizes stopped `*process.Process` removals on acceptance (calling `UnconfigureProcess`) and schedules `recoverPluginRemoval` on rejection | Extends (items 1, 2, 10): carries process-less removals through the same acceptance scope to `UnconfigureProcess`, and keeps rolled-back candidate auto-loads out of committed-removal recovery |
| Process manager slots, configs and respawn | `internal/component/plugin/process/manager.go` `ProcessManager.configs`, `processes`, `respawnMu`, `Respawn`, `RemoveProcess`, `UnconfigureProcess`, `ConfiguredNames` | Holds configured plugins and live slots; `Respawn` already releases `pm.mu` across join/connect-back (`bc532d7502`) while holding global `pm.respawnMu` | Extends (items 1, 3, 5, 7): narrows/verifies respawn serialization, synchronizes status-gauge deletion, and moves operator re-enable inside the admitted respawn path |
| Restart controller set | `internal/component/plugin/server/restart_controller.go` `restartControllerSet` (`tryAcquire`, `wait`, `release`) | Serializes operator/rollback restarts and failure-policy restarts per plugin name (`d73a860caf`) | Preserved and extended (items 4, 7): keeps per-name restart serialization and adds reload/coordinator exclusion for failure-policy restarts |
| Generation identity | `GetProcess(name) != proc` in `applyFailurePolicy` and `restartPluginAfterExit` | Tells a superseded generation from the live one | Preserved (items 3, 4): `Respawn` withdraws `old` before `Stop`, keeping this check intact |
| Cleanup-owed record | `internal/component/plugin/server/restart.go` `lateCleanupRecord`, `awaitReplacedCleanup` | Refuses a restart or startup phase while one recorded stopped process owes cleanup | Extends (items 4, 8): tracks all stopped processes owing cleanup per name rather than overwriting |
| Transaction lock and startup coordinator | `txLock` in `handlePluginRestart`, `reloadConfig`, `recoverPluginRemoval`; `s.coordinator` in `startup.go` and `restart.go` `runUnbarrieredStartupHandshake` | Serializes operator restarts, reloads and removal recovery; `s.coordinator` synchronizes tier barriers | Extends (items 4, 9): removes the global `s.coordinator = nil` clobber on unbarriered handshakes and guards boot-window restarts |
| Stop record, doctor row and bus warning | `internal/component/plugin/process/stopped.go` `PluginStop`, `recordStoppedLocked`, `RestartCommand`; `internal/component/plugin/doctor/checks_runtime.go` `pluginStoppedRow` | Records down plugins, raises `doctor-plugin-stopped` on the bus, and renders the runtime doctor row | Extends (items 6, 10): shares one declaration of the cleanup-owed message/fix across bus warning and doctor row, and avoids leaking stop records on refused reload auto-loads |
| Metric hook set | `internal/component/plugin/process/metric_hooks.go` `processMetricHooks`, `noteStageChange`, `clearMetricHooks` | Atomic callback pointer swapped by the manager, unsynchronized with `deletePluginStatusLabel` | Extends (item 5): synchronizes status gauge updates against label deletion on disable |
| Orphaned-dependency stop | `internal/component/plugin/server/startup_autoload.go` `collectOrphanedDependencies`, `stopOrphanedDependencies` | Stops dependency-only plugins left without a live dependent | Extends (item 2): distinguishes rolled-back candidate auto-loads from committed removals and spares pre-existing or independently needed dependencies |
| Route withdrawal and cleanup | `internal/component/plugin/server/dispatch.go` `cleanupProcess`; `dispatch_route.go` `withdrawPluginRoutes`; `internal/core/rib/locrib/manager.go` `RIB.Remove`; `internal/core/rib/store/store_bart.go` `Store.Modify`, `Store.Delete` | Withdraws plugin routes serially inside `restartCleanupGrace` (3 s); measured 3.192 s for 1M routes | Extends (item 8): adds `Store.ModifyOrDelete` and chunked per-shard `RIB.RemoveBatch` so 1M-route withdrawal finishes well inside the 3 s bound |
| Neighbour: plugin-process lifecycle | `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` (status `design`, D-A/D-B approved 2026-11-24) | Owns retiring `ignore` exits, OPEN-policy owed record, reload restarting down plugins (D-B), running-only accessor, stop-signal fence | Coordinate, same files: item 1's `finishRemovalScope` removal and item 10's stop-record cleanup keep D-B's `pm.stopped` population clean |
| Neighbour: first half of item 1 | `spec-fib-dummy` (closed `4cbf5dbf47`, landed `6da64fc95a`), AC-17 | Dropped finalized reload removals with a process via `UnconfigureProcess` at `finishRemovalScope` | Extends (item 1, second half): process-less removals leave through the same `finishRemovalScope` -> `UnconfigureProcess` path |
| Neighbour: reload reconciliation | `plan/release/fixes/spec-single-owner-reload-reconcile.md` (status `design`) | Owns general reload/rollback reconciliation | Coordinate: items 1, 2, 10 own plugin-process removal and auto-load stop bookkeeping |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | A feature registers from `init()` in its own `register.go` and is discovered. | agrees | No plugin name, command, or feature enum is hardcoded in engine, config, or RIB code; removals derive from `registry.ConfigRootsMap`, `registry.ProgramsFIB`, and `AutoLoadReasonFor` |
| `ai/doors/plugin-infra.md` | Boundaries | door covering `internal/component/plugin/` | A compiled-in and an external plugin differ only in how they reach the bus. | agrees | Per-name respawn locking, `txLock`-serialized failure-policy restarts, `lateCleanupRecord`, and boot-window refusal treat internal and forked plugins identically |
| `ai/doors/core.md` | Where it sits | door covering `internal/core/rib/` | It imports neither `internal/component/` nor `internal/plugins/`. | agrees | `Store.ModifyOrDelete` and `RIB.RemoveBatch` stay pure core libraries in `internal/core/rib/` with no upward imports |
| `ai/doors/le-tooling.md` | Boundaries | door covering `test/plugin/*.ci` | Run one package through `./le test unit pkg path <dir>/ [run <regex>] [vet]`, which takes the tags from the toolchain. | agrees | All new `.ci` files draft under `test/draft/plugin/` and promote to `test/plugin/`, and package runs use `./le test unit pkg path` |
| `docs/architecture/api/process-protocol.md` | 5-Stage Startup Protocol | `// Design:` and anchors of `manager.go`, `metric_hooks.go`, `restart.go`, `failure_policy.go`, `startup.go`, `startup_removal.go`, `startup_autoload.go`, `stopped.go`, `register_plugin_restart.go`, `reload_tx.go`, `reload_dataplane_writer.go`, `dispatch_route.go`; modified by this spec | A restart the failure policy makes holds no lock: its replacement's handshake runs as long as the daemon does, so a slow replacement still comes up, and only its rollback wait is bounded. | departs | D1 changes failure-policy restarts (`restartPluginAfterExit`) to acquire `txLock` before `restartControllers` and bound the replacement handshake with `boundedRestartHandshake` (5 stage windows plus accepted `apply-budget` and `started-handler` window), preventing reload/config-tree races and wedged replacements; this page is updated in Phase 4 |
| `docs/architecture/plugin/plugin-system.md` | Startup phases and callbacks | `// Design:` of `reload_dataplane_writer.go`; anchors of `manager.go`, `startup.go`, `startup_autoload.go`; modified by this spec | Unbarriered ad-hoc and failure-policy startup retain their caller's lifetime as their only bound. | departs | D1 moves failure-policy startup onto `boundedRestartHandshake` under `txLock` (ad-hoc sessions keep their caller's lifetime); this page is updated in Phase 4 |
| `docs/architecture/doctor-and-health-checks.md` | Plugin lifecycle checks | `// Design:` and anchors of `stopped.go`, `register_plugin_restart.go`, `checks_runtime.go`, `manager.go`, `restart.go`, `failure_policy.go`, `startup.go`, `startup_removal.go`, `startup_autoload.go`, `reload_dataplane_writer.go`; modified by this spec | A plugin that was already down (a failed start left it configured with no process) is not collected by the removal at all, so it keeps its record and this check keeps reporting it, with a restart as its fix. | departs | Item 1 collects configured process-less plugins whose roots or FIB selection leave and unconfigures them at `finishRemovalScope(scope, true)`, clearing `pm.configs`, `pm.stopped`, and `doctor-plugin-stopped`; this page is updated in Phase 2 |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `internal/component/plugin/process/manager.go` | The Prometheus implementation should continue to use the existing push-model `metrics.Registry` path. | agrees | Item 5 keeps `ze_plugin_status` on the push-model `metrics.Registry` path and only synchronizes gauge deletion against concurrent `SetStage` via leaf mutex `statusMu` |
| `docs/architecture/cli/command-completion.md` | The three completion paths are distinct | anchored to `register_plugin_restart.go`, `startup.go` | (`request plugin restart` offers every configured plugin, saying which run). | agrees | `completePluginRestart` keeps reading `pm.ConfiguredNames()`; once Item 1 drops removed down plugins at `finishRemovalScope`, completion matches the running configuration |
| `docs/architecture/config/transaction-protocol.md` | Completion | `// Design:` of `reload_tx.go`; anchors of `failure_policy.go`, `startup.go`, `startup_removal.go` | The server scopes acceptance to the whole reload through `DeferReloadAcceptance`, which returns `func(bool) error`. | agrees | Item 1 attaches process-less removals to the same `reloadAcceptance` scope (`s.unconfigureOnAccept[scope]`), finalizing them only when accepted |
| `docs/architecture/forked-route-install.md` | Disconnect withdraws, and that conflicts with graceful restart | `// Design:` of `dispatch_route.go`; anchored to `locrib/manager.go` | The engine tracks the routes each plugin installed and withdraws them in `cleanupProcess`, so a crashed forked plugin leaves no stale kernel route. | agrees | Item 8 keeps tracking and withdrawal in `cleanupProcess` -> `withdrawPluginRoutes`, replacing per-route `RIB.Remove` calls with chunked `RIB.RemoveBatch` |
| `docs/architecture/plugin/rib-storage-design.md` | Generic `Store[T]` and bestPrev Consolidation | `// Design:` and anchors of `store_bart.go` | The `maprib` build tag degrades `Store[T]` to map-only storage for both backends; `FamilyRIB` retains its two build-tagged files (`familyrib_bart.go`, `familyrib_map.go`) wrapping the corresponding `Store[T]` variant. | agrees | Item 8 adds `Store.ModifyOrDelete` to both `store_bart.go` and `store_map.go` so the `-tags maprib` build stays in parity |
| `docs/architecture/rib/unified-locrib.md` | Constraints | `// Design:` of `locrib/manager.go` and `locrib/metrics.go` | **`OnChange` subscribers run synchronously under the locrib write lock, so they must be cheap.** | agrees | `RIB.RemoveBatch` keeps synchronous `OnChange` dispatch under `sh.mu` and chunks removals to 1,024 prefixes per shard lock hold |
| `docs/architecture/api/commands.md` | Process Lifecycle Commands | `// Design:` of `startup.go`; anchored to `manager.go` | - | silent | Covers command registry, pipe grammar, and SSH reboot/restart mapping; silent on plugin restart locking, boot-window refusal, and failed reload auto-load stop records |
| `docs/architecture/hub-architecture.md` | SSH session model factory | anchored to `manager.go`, `failure_policy.go`, `startup.go` | - | silent | Covers hub startup sequence, stop drain, and stop-signal table; silent on `Respawn` locking, failure-policy `txLock`, process-less unconfigure, and `handlePluginRestart` boot refusal |
| `docs/architecture/behavior/peer-lifecycle.md` | In-place import invalidation (RFC 8212 D-2) | anchored to `reload_tx.go` | - | silent | Covers BGP peer FSM and session lifecycle; silent on plugin restart lock ordering and cleanup-owed records |
| `docs/architecture/config/apply-ordering.md` | Phases 2 and 5, for BGP | anchored to `reload_tx.go` | - | silent | Covers cross-root config apply ordering; silent on plugin restart exclusion and `lateCleanupRecord` |
| `docs/architecture/core-design.md` | 1. System Architecture | anchored to `reload_tx.go`, `dispatch_route.go` | - | silent | Covers component layering and core boundaries; silent on plugin restart locking, failure-policy handshake bounds, and Loc-RIB batch removal internals |
| `docs/architecture/exabgp-bridge.md` | Script exit, respawn, and route ownership | anchored to `failure_policy.go` | - | silent | Covers ExaBGP wrapper exit and `on-exit keep` route ownership; silent on `superviseProcess` stop-record guard and failure-policy `txLock` serialization |
| `docs/architecture/firewall/firewall-irr.md` | A crash starts the plugin again | anchored to `failure_policy.go` | It is bounded at 5 restarts in 60 seconds and 20 over the life of the daemon, which is what stops an answer that panics every time from cycling: past the bound the plugin is disabled and `show health` reports doctor-plugin-stopped. | agrees | Both respawn limits and `doctor-plugin-stopped` reporting are preserved; Item 7 strengthens the bound by ensuring refused operator restarts never prematurely clear `disabled` or counters |
| `docs/architecture/api/ipc_protocol.md` | Plugin-Provided Commands | anchored to `startup.go` | - | silent | Covers IPC wire framing and answer protocol; silent on `StartupCoordinator` parameter passing and reload auto-load stop records |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 3: teardown belongs to the firewall engine, gated by config | anchored to `startup.go`, `startup_autoload.go` | - | silent | Covers firewall table ownership and flush-on-stop rules; silent on process-less reload unconfigure and orphan dependency auto-load reasons |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `startup.go`, `startup_autoload.go` | - | silent | Covers IKEv2 engine config and auto-load discovery; silent on plugin restart and failed reload auto-load stop records |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Scripted kernel tests | anchored to `startup.go` | - | silent | Covers PPPoE client config delivery and kernel tests; silent on plugin restart and reload auto-load stop records |
| `docs/architecture/config/syntax.md` | What a reader receives | anchored to `startup_removal.go` | - | silent | Covers YANG config syntax and running-map loading; silent on `finishRemovalScope` process-less unconfigure and rolled-back candidate stops |
| `docs/architecture/ospf/bfd-client.md` | Decisions | anchored to `startup_autoload.go` | - | silent | Covers OSPF BFD client auto-load; silent on orphan dependency cleanup and process-less removal |
| `docs/architecture/plugin/feature-gates.md` | Naming a plugin the build left out | anchored to `startup_autoload.go` | - | silent | Covers compile-out build tags and gated data-plane writers; silent on process-less reload unconfigure and orphan dependency cleanup |
| `docs/architecture/static-routes.md` | A main-table route reaches the FIB through the Loc-RIB | anchored to `startup_autoload.go`, `reload_dataplane_writer.go` | - | silent | Covers static route data-plane writer selection; silent on process-less down-plugin unconfigure and `AutoLoadReason` orphan guard |
| `docs/architecture/rib/forward-handle.md` | The decisions | `// Design:` and anchors of `locrib/manager.go` | - | silent | Covers `Change.Forward` zero-copy wire handles on `InsertForward`; `Remove` and `RemoveBatch` emit nil `Forward` handles as before |
| `docs/architecture/isis/isis-9-spf-rib.md` | Decision: ECMP needed a path-group expansion in shared code | anchored to `locrib/manager.go` | - | silent | Covers IS-IS SPF and ECMP sibling members; `RIB.RemoveBatch` preserves identical best-path and ECMP fallback notifications |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter | agrees | No engine operation or plugin identity authorization rule is added or changed |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included | agrees | `request plugin restart <name>` keeps its existing command path and adds no flags or presentation words |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed." | agrees | No pipe operator or command answer format is added or changed |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does | agrees | No pipe operation is added or changed |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration | agrees | Query-mode schema discovery and offline config parsing are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now. | agrees | Kernel-vs-VPP config validation is untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each. | agrees | Cross-plane mismatch refusal is untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation. | agrees | VPP firewall verification and YANG annotations are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it". | agrees | Data-plane selector vocabulary is untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it." | agrees | BGP export policy next-hop rewriting is untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs"). | agrees | Policy next-hop authoring forms are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)"). | agrees | BGP export chain compilation is untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self"). | agrees | Export chain next-hop folding and raw overrides are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load"). | agrees | Explicit next-hop policy validation is untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes"). | agrees | Forwarded vs originated next-hop policy is untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?". | agrees | Implied filter publication is untouched |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist | agrees | ExaBGP wrapper endpoints use the standard external plugin restart lifecycle |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator `ze request plugin restart <name>` (via CLI, SSH, REST, gRPC, MCP, web) reaching `handlePluginRestart` (`internal/component/plugin/server/register_plugin_restart.go`) and `completePluginRestart`.
- Plugin process exit under failure policy `restart` reaching `superviseProcess` -> `applyFailurePolicy` -> `restartPluginAfterExit` (`failure_policy.go`, `reload_tx.go`).
- Config reload (`ze signal reload`, SIGHUP, commit, managed push) reaching `reloadConfig` (`reload.go`), `collectReloadStopProcesses`, `stopCollectedProcesses`, `autoStopPluginNames`, and `finishRemovalScope` (`startup_removal.go`).
- Large-table plugin disconnect/restart reaching `cleanupProcess` -> `withdrawPluginRoutes` -> `locrib.RIB.RemoveBatch` (`dispatch_route.go`, `locrib/manager.go`, `store/store_bart.go`).
- Operator diagnostics `ze show doctor` (`pluginStoppedRow`, `doctor/checks_runtime.go`) and `ze show warnings` / daemon log (`recordStoppedLocked`, `process/stopped.go`).

### Transformation Path
1. **Operator restart (`handlePluginRestart`):** refuses if arg count != 1, server/pm nil, `!s.startupComplete()` (item 9, naming `ze show doctor` and when to retry), or self-restart. Acquires `txLock.tryAcquire()`. Under `txLock`, checks `pluginRestartRefusal(pm, name)` and `removalPendingFor(name)` (item 7), records `wasRunning`, and calls `restartPluginReplacementReenabled(name)` without prematurely calling `pm.Reenable(name)`.
2. **Restart controller and process respawn (`restartPluginReplacement`, `restartPluginProcess`, `ProcessManager.respawn`):** acquires per-name `restartControllers.tryAcquire`, checks `s.lateCleanups.owed(name)` (all uncleaned processes in `procs[name]` slice; item 4b), and calls `pm.RespawnReenabled(name)` (operator) or `pm.Respawn(name)` (rollback/failure-policy). Inside `pm.respawn(name, reenable)`, per-name `respawning[name]` wait channel serializes same-name respawns while allowing unrelated plugin names to respawn concurrently (item 3). Under `pm.mu`, all pre-spawn refusals (`ctx == nil`, `respawnRefused`, `shutdown`, `stopActive`, `cfg == nil`) run BEFORE `reenable` clears `disabled`, `respawnTimes`, and `totalRespawns` (item 7). On respawn-limit refusal, `clearProcessCallbacks` clears `metricHooks` and `deletePluginStatusLabel` deletes `ze_plugin_status{plugin=name}` under leaf lock `statusMu`, serialized with `wireMetrics`'s `stageChange` re-checking `p.metricHooks.Load() == hooks` (item 5).
3. **Failure-policy restart (`applyFailurePolicy` -> `restartPluginAfterExit`):** acquires `s.txLock.acquire(s.ctx)` FIRST, then rechecks `s.stopping`, `s.removalPendingFor(name)`, and `pm.GetProcess(name) != outgoing`, then acquires `s.restartControllers.wait(s.ctx, name)` (global lock order `txLock` -> `restartControllers`), and runs `restartPluginWithin(name, s.boundedRestartHandshake)` (item 4a). Releases `restartController` before `txLock.release()`, so any deferred peer-declaration admission/refusal runs after `txLock` is released with zero inversion.
4. **Unbarriered handshake (`runUnbarrieredStartupHandshake`):** passes `coord = nil` explicitly to `handleProcessStartupRPCWithin(ctx, proc, nil)` instead of mutating `s.coordinator` (items 4, 9). Tiered startup passes `newCoord` explicitly.
5. **Cleanup and route withdrawal (`awaitReplacedCleanup` -> `cleanupProcess` -> `withdrawPluginRoutes`):** `withdrawPluginRoutes` converts the plugin's installed `routeKey` set into a batch and calls `locrib.RIB.RemoveBatch`, which groups by `(fam, shardIdx)` and removes in 1,024-route chunks using `Store.ModifyOrDelete` (one `Get` + direct `Delete` when empty) and cached shard label strings, finishing 1M routes well inside `restartCleanupGrace` (3 s) (item 8). If cleanup still expires, `s.lateCleanups.owe(old)` appends `old` to `procs[name]` (deduplicated by pointer; item 4b) and `restartReplacementWithin` records `PluginStop{Cause: StopRestartFailed, CleanupOwed: true}`.
6. **Shared stop warning and doctor row (`process/stopped.go`, `doctor/checks_runtime.go`):** `process.StoppedMessage` and `process.StoppedFix` are the single declaration read by both `recordStoppedLocked` (bus warning and WARN log) and `pluginStoppedRow` (`ze show doctor`). When `CleanupOwed` is true, both emit the refusal + `privilege.RestartRemedy()` last-resort explanation in the message with `RestartCommand(name)` as the clean command fix. When `lateCleanups.owed(name)` later observes all owed cleanups finished, it clears `CleanupOwed` on the stop record and refreshes the bus warning (item 6).
7. **Reload removal of process-less down plugins (`collectReloadStopProcesses`, `stopCollectedProcesses`, `finishRemovalScope`):** `collectNonFIBRemovedProcesses`, `judgeReloadWriters`, and `collectOrphanedDependencies` inspect configured down plugins (`GetProcess == nil` with matching `RegistrationFor` / config) alongside running processes. `stopCollectedProcesses` records process-less removals on `s.unconfigureOnAccept[scope]` (when `scope != nil`) or calls `pm.UnconfigureProcess(name)` immediately (when `scope == nil`). On `finishRemovalScope(scope, true)`, each recorded name is passed to `pm.UnconfigureProcess(name)` and `s.unmarkPluginLoaded(name)`; on `finishRemovalScope(scope, false)`, the pending set is discarded so the down plugin stays configured and reported down (item 1).
8. **Rolled-back candidate auto-loads (`autoStopPluginNames`, `runPluginPhaseWithin`, `rollbackNonRunningStartupProcesses`):** `autoStopPluginNames` invokes direct final stop (`provisional = false`) so rolled-back candidate plugins never attach to `s.txLock.acceptance` / `s.removals` and never trigger background `recoverPluginRemoval` -> `restoreRemovedPlugin` -> `stopOrphanedDependencies`; `collectOrphanedDependencies` also spares any candidate whose `AutoLoadReasonFor` is recorded and not `AutoLoadReasonDependency` (item 2). On failed candidate reload auto-load at any stage (including `runProcessGenerationReady`), `pm.UnconfigureProcess` drops the name and `pm.RecordStopped` is skipped so neither `pm.configs` nor `pm.stopped` / `doctor-plugin-stopped` leaks (item 10).
9. **Boot Step (d) guard (`startup.go`, `failure_policy.go`):** `superviseProcess` calls `pm.ClearStopped` only when `pm.GetProcess(proc.Name()) == proc`, and Step (d) skips `superviseProcess`/`runProcessGenerationReady` if `pm.GetProcess(proc.Name()) != proc` (item 9 defense in depth).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator CLI / SSH / API -> `plugin/server` | `ze request plugin restart <name>` and tab completion via `handlePluginRestart` / `completePluginRestart`; adds boot-window refusal and removes stale configured names on finalized reload removal | Read `register_plugin_restart.go` |
| `plugin/server` -> `plugin/process` | `Respawn` / `RespawnReenabled`, `UnconfigureProcess`, `RecordStopped` (with `CleanupOwed`), `ClearStoppedCleanupOwed` | Read `manager.go`, `stopped.go`, `reload_tx.go`, `startup_removal.go` |
| `plugin/process` -> `core/report` & `plugin/doctor` | One shared `StoppedMessage`/`StoppedFix` declaration feeds both `report.RaiseWarningFix` (bus / `ze show warnings`) and `pluginStoppedRow` (`ze show doctor`) | Read `stopped.go`, `checks_runtime.go`, `checks_recent_events.go` |
| `plugin/server` -> `core/rib/locrib` -> `core/rib/store` | `withdrawPluginRoutes` calls new `RIB.RemoveBatch`, which uses `Store.ModifyOrDelete`; keeps `core` leaf direction (`server` -> `core`) | Read `dispatch_route.go`, `locrib/manager.go`, `store_bart.go` |

### Changed-Wire Census

No wire changes. Source-supported reason: every change is internal daemon bookkeeping, lock ordering, in-memory Loc-RIB batch removal, and operator/doctor text. No RPC method name, no JSON tag or field in `pkg/plugin/rpc`, no YANG leaf or RPC signature, no JSON event payload, and no external plugin protocol message is added, removed, or renamed.

**Search evidence:** scope `internal/component/plugin/process`, `internal/component/plugin/server`, `internal/component/plugin/doctor`, `internal/core/rib/locrib`, `internal/core/rib/store`, `pkg/plugin/rpc`; terms `DeclareRegistration`, `ByeInput`, `ConfigVerify`, `RouteInstall`, `RouteRemove`, `json:"`; positive control: `pkg/plugin/rpc/types.go` (`ByeInput`, `RouteInstallInput`, `RouteRemoveInput`) and `internal/component/plugin/server/dispatch_route.go` (`opRouteInstall`, `opRouteRemove`) are the wire encoders/decoders and none of their wire types or fields are modified by this spec.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| none | writer | none: no RPC/JSON/YANG encoder is edited | verified against `pkg/plugin/rpc/types.go` and `dispatch_route.go` (`opRouteInstall`, `opRouteRemove`) | preserved |
| none | reader | none: no RPC/JSON/YANG decoder is edited | verified against `pkg/plugin/rpc/types.go` and `register_plugin_restart.go` (`handlePluginRestart` YANG args unchanged) | preserved |

### Integration Points
- `ProcessManager.UnconfigureProcess` (`manager.go`) - called by `finishRemovalScope` for process-less removals (item 1) and by `startup.go` on `runProcessGenerationReady` reload auto-load failures (item 10).
- `ProcessManager.respawn` / `RespawnReenabled` (`manager.go`) - per-name `respawning` map (item 3), `statusMu`-synchronized `deletePluginStatusLabel` (item 5), and post-refusal re-enable (item 7).
- `ProcessManager.RecordStopped` / `ClearStoppedCleanupOwed` (`stopped.go`) - carries `PluginStop.CleanupOwed`, shares `StoppedMessage`/`StoppedFix` with `doctor/checks_runtime.go` (item 6).
- `Server.handlePluginRestart` (`register_plugin_restart.go`) - refuses during boot (`!startupComplete()`), checks `pluginRestartRefusal` under `txLock`, delegates re-enable to `restartPluginReplacementReenabled` (items 7, 9).
- `Server.restartPluginAfterExit` (`reload_tx.go`) - acquires `txLock.acquire(s.ctx)` then `restartControllers.wait`, checks `removalPendingFor` and `GetProcess`, uses `boundedRestartHandshake` (item 4).
- `lateCleanupRecord` (`restart.go`) - per-name slice `map[string][]*process.Process` and bus-warning refresh on clear (items 4, 6).
- `Server.runUnbarrieredStartupHandshake` / `handleProcessStartupRPCWithin` (`restart.go`, `startup.go`) - explicit `coord *plugin.StartupCoordinator` parameter replacing global `s.coordinator = nil` (items 4, 9).
- `Server.finishRemovalScope`, `notifyPluginRemoval`, `rollbackStartupProcess`, `stopCollectedProcesses`, `autoStopPluginNames`, `collectNonFIBRemovedProcesses`, `judgeReloadWriters`, `collectOrphanedDependencies` (`startup_removal.go`, `startup_autoload.go`, `reload_dataplane_writer.go`) - process-less removal finalization and direct non-provisional stop of rolled-back candidate auto-loads (items 1, 2, 10).
- `Store.ModifyOrDelete` (`store_bart.go`), `RIB.RemoveBatch` (`locrib/manager.go`), `recordRemoveBatch` (`locrib/metrics.go`), `withdrawPluginRoutes` (`dispatch_route.go`) - fast chunked route withdrawal (item 8).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Process-less removals finalize through `finishRemovalScope` -> `UnconfigureProcess`; operator re-enable passes through `restartPluginReplacement` -> `Respawn`; bus and doctor share `process.StoppedMessage`/`StoppedFix`; route withdrawal stays on `locrib.RIB` |
| No unintended coupling (components stay isolated) | Yes | No new cross-component imports: `process` imports `internal/core/privilege` (core leaf); `server` already imports `internal/core/rib/locrib`; `locrib` stays in `internal/core` with no upward imports |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `finishRemovalScope`, `Respawn`, `restartControllerSet`, `lateCleanupRecord`, `StoppedMessage`, and `locrib.RIB`; removes duplicate cleanup-owed prose from `checks_runtime.go` and removes `lifetimeRestartHandshake` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `Store.ModifyOrDelete` reduces BART copies/traversals by half; cached shard label strings eliminate 2M `strconv.Itoa` allocations per 1M-route withdrawal |
| Registration over hardcoding, outbound | Yes | No plugin name, command, or feature enum is hardcoded; removal and orphan checks derive from `registry.ConfigRootsMap`, `registry.ProgramsFIB`, `AutoLoadReasonFor`, and process pointers |
| Registration over hardcoding, inbound | Yes | No new command, config leaf, or diagnostic code is introduced; existing `ze-plugin:system-plugin-restart` and `doctor-plugin-stopped` registrations stay unchanged |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Item 2 follows the path `recoverPluginRemoval` -> `restoreRemovedPlugin` -> `stopOrphanedDependencies` -> `collectOrphanedDependencies` when a rolled-back candidate auto-load enters `s.removals`, and is not a test artefact | Source reading of `reload.go:596`, `startup_removal.go:202-346, 396-445`, `startup_autoload.go:601-663`, and `reload_test.go:586-667` (test never waits for `s.wg`) | A second path also stops pre-existing dependencies | Wait on `s.wg` in `TestReloadFailureKeepsPreExistingDependency` (reproduced 100% red at `reload_test.go:669` on 2026-11-25) | confirmed |
| A-2 | A boot-window `request plugin restart` is reachable in production because `infraSetup` opens and wires SSH during `bgp` Stage 2 (`OnConfigure`), before Phase 1 Step (d) and Phases 2–3 | Source reading of `bgp/plugin/register.go:319,365`, `cmd/ze/hub/infra_setup.go:225,322`, `service_ssh.go:152,181`, `startup.go:583,690-713`, `failure_policy.go:44`, `restart.go:256-267` | Non-BGP daemons still only open SSH after `WaitForStartupComplete`; Tier 0 internal plugins on `DirectBridge` can still call `dispatch-command` before Step (d) | `TestRestartRefusedDuringBootWindow` driving `handlePluginRestart` before `startupComplete` (passes in `295f10ed26`) | confirmed |
| A-3 | The 3 s `restartCleanupGrace` bound is exceeded by serial per-route `withdrawPluginRoutes` on a 1M-route table | Measured on 2026-11-25 (`CGO_ENABLED=0`, 32-core host, zero contention): before = 100k 279 ms, 500k 1.608 s, 1M 3.192 s (2.568 s nometa); after `ModifyOrDelete` + chunked `RemoveBatch` = 100k 190 ms, 500k 742 ms, 1M 1.806 s (1.434 s nometa) | `ModifyOrDelete` + chunked `RemoveBatch` does not bring 1M routes below 3 s, in which case `restartCleanupGrace` must also be sized | Re-measured 100k/500k/1M routes with `BenchmarkWithdrawPluginRoutes` after Phase 6 (`5a0006fb97`): 1M routes dropped from 3.192 s to 1.806 s (well inside 3 s) | confirmed |
| A-4 | `Respawn`'s primary lock-across-join defect (item 3) is already fixed by `bc532d7502`; only global `pm.respawnMu` (serializing unrelated plugin names) and explicit AC-3 tests remain | Source reading of `manager.go:829-963` and `manager_shutdown.go:82-114` | Another reader or path still blocks on `pm.mu` or `pm.respawnMu` during connect-back | `TestRespawnDoesNotBlockReadersOrUnrelatedRespawn` and `TestRespawnInDaemonExitNoOverlap` (pass in `ed3e6f7ad5`) | confirmed |
| A-5 | `TestPluginStoppedRowWhileCleanupOwed` (`checks_runtime_test.go:127-129`) and `TestRestartCommandFailsWhenOldCleanupNeverEnds` (`plugin_restart_guard_test.go:288-290`) pin `Fix == RestartCommand(name)` while putting the cleanup-owed refusal and daemon-restart last resort in `Message`/`Why` | Direct reading of `checks_runtime.go:85-102` and `register_plugin_restart.go:218-225`; owner confirmed D2 at Design gate (2026-11-25) | Bus warning and doctor row still diverge if `StoppedMessage` does not carry the shared cleanup-owed text | `TestStoppedBusWarningMatchesDoctorRowWhileCleanupOwed` | confirmed (owner D2) |
| A-6 | No production plugin relies on an unbounded failure-policy handshake without declaring `apply-budget` or `stage-timeout` | `restartStartupHandshakeBound` already gives 5 × `stage-timeout` (25 s default) plus declared `apply-budget` (up to 600 s) plus `started-handler` window; owner confirmed D1 at Design gate (2026-11-25) | A slow plugin without `apply-budget` hits the 25 s bound on crash-restart and must declare `apply-budget` | Existing plugin suite (`./le test functional plugin`) and `TestFailurePolicyRestartSerializesWithReload` | confirmed (owner D1) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Lock inversion between `txLock` and `restartControllers` when `restartPluginAfterExit` acquires `txLock` and defers peer-declaration admission/refusal | Hang or `-race` failure in failure-policy restart with changed filter insertions | Strict global lock order `txLock` -> `restartControllers`: `restartPluginAfterExit` releases `restartController` BEFORE calling `txLock.release()`, so deferred `admit`/`refuse` run after `txLock` is free. Tested under `-race`. |
| R-2 | `txLock.acquire(s.ctx)` in `restartPluginAfterExit` holds `txLock` during a slow replacement, making a concurrent `reloadConfig` return "config transaction in flight" | Reload returns busy during crash-restart | `boundedRestartHandshake` bounds the hold identically to `request plugin restart`, and `txLock.acquire` aborts immediately on daemon stop (`s.ctx`). |
| R-3 | Process-less removal drops a down plugin whose reload is later rejected or compensated, or drops a down FIB plugin the candidate still selects | Down plugin missing from `ConfiguredNames` or `doctor-plugin-stopped` after a rejected reload | Process-less removals only call `UnconfigureProcess` inside `finishRemovalScope(scope, true)` on accepted reloads; rejected scopes discard `s.unconfigureOnAccept[scope]`. |
| R-4 | `RIB.RemoveBatch` holds a shard lock too long, stalling concurrent BGP/sysrib lookups | Lookup latency spike during massive withdrawal | Process at most 1,024 prefixes per shard lock hold (< 1 ms per hold), releasing `sh.mu` between chunks. |
| R-5 | `statusMu` lock inversion with `pm.mu` or `p.stageMu` | Deadlock or `-race` report in `Respawn` + concurrent `SetStage` | `statusMu` is strictly a leaf lock: inside `statusMu` only `p.metricHooks.Load()` and `pmetrics.status` are touched; never acquires `pm.mu` or `p.stageMu`. |
| R-6 | Direct final stop of rolled-back auto-loads (`provisional = false`) fails `SendBye("removed")` | Timed-out rollback leaves process running | On `SendBye` error/timeout with `scope == nil`, `recoverPluginRemoval` still retries stopping it, and `collectOrphanedDependencies` spares pre-existing dependencies. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Plugin restarts, reload removal/recovery, failure-policy recovery, Prometheus status gauge metrics, doctor/warning output, and Loc-RIB route withdrawal |
| How is it reverted? | Single commit revert per phase (each phase lands as one self-contained commit) |
| Who else touches this path? | `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` (`design`, D-A/D-B approved 2026-11-24) and `plan/release/fixes/spec-single-owner-reload-reconcile.md` (`design`) share `manager.go`, `stopped.go`, `reload_tx.go`, `startup_autoload.go`, and `startup_removal.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze request plugin restart <name>` after reload removed down plugin X | → | `collectReloadStopProcesses` -> `finishRemovalScope` -> `UnconfigureProcess` -> `pluginRestartRefusal` | `test/plugin/plugin-restart-removed-down.ci` |
| `ze request plugin restart <name>` on forked plugin while replacement waits for connect-back | → | `Respawn` per-name `respawning` + released `pm.mu` -> `PluginDoctorSnapshot` | `test/plugin/plugin-restart-external-nonblocking.ci` |
| `ze request plugin restart <name>` refused (cleanup owed / stopping) on disabled plugin | → | `handlePluginRestart` -> `restartPluginReplacementReenabled` -> `pm.RespawnReenabled` | `test/plugin/plugin-restart-refused-keeps-disable.ci` |
| `ze request plugin restart <name>` during boot window | → | `handlePluginRestart` `!startupComplete()` guard | `TestRestartRefusedDuringBootWindow` (seam: `.ci` cannot reliably catch the sub-second window between `bgp` Stage 2 SSH bind and Phase 1 Step (d)) |
| Refused reload auto-loading a plugin whose handshake fails | → | `runPluginPhaseWithin` -> `unconfigureFailedReloadStart` skipping `RecordStopped` | `test/plugin/reload-failed-autoload-no-stopped-row.ci` |
| Forked plugin disconnect withdrawing large route table | → | `cleanupProcess` -> `withdrawPluginRoutes` -> `locrib.RIB.RemoveBatch` -> `Store.ModifyOrDelete` | `TestWithdrawPluginRoutesLargeTableWithinGrace` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A committed reload removes all config roots of (or displaces the FIB selection of) plugin X while X is already down and holds no process | Completion offers only the names still configured (not X). `ze request plugin restart X` answers that X is not configured. `ze show doctor` and `ze show warnings` report no `doctor-plugin-stopped` row or warning for X | A plugin stopped by a failed, rolled-back, or rejected deferred reload stays configured and keeps its stop record, because the committed configuration still names it. A removed plugin that was running is already covered by `spec-fib-dummy` AC-17 (`6da64fc95a`) |
| AC-2 | A rejected reload auto-loaded a dependent plugin whose dependency already ran before the reload | First, `TestReloadFailureKeepsPreExistingDependency` waiting on `s.wg` reproduces the failure deterministically before the fix. After the fix, the pre-existing dependency stays running and configured after all background recovery settles, and any non-dependency `AutoLoadReason` plugin is spared by `collectOrphanedDependencies` | A dependency that the rejected reload itself auto-loaded is stopped and unconfigured, because nothing in the committed configuration needs it |
| AC-3 | A restart of a forked plugin while its replacement waits on connect-back, and a restart of an in-daemon plugin whose engine reads `pm` on exit | `PluginDoctorSnapshot`, `GetProcess`, `AllProcesses`, `ConfiguredNames`, and `Respawn` of an unrelated plugin name all answer within a bound well below the 30 s connect-back wait. The old generation's `applyFailurePolicy` sees `GetProcess(name) != old` and returns without acting. An in-daemon engine calling `pm.GetProcess`/`AllProcesses`/`ConfiguredNames` on exit joins within `StopJoinBound` and raises no `doctor-plugin-restart-overlap` | A second `Respawn` of the *same* plugin name waits on that name's `respawning` slot, because two spawns cannot race one name; during join/connect-back `GetProcess(name)` returns `nil` (non-blocking) because `old` was already withdrawn |
| AC-4 | A failure-policy restart races an operator restart or a config reload, or two stopped processes of one name owe runtime cleanup | Failure-policy restart acquires `txLock` before `restartControllers`, runs under `boundedRestartHandshake`, never clobbers `s.coordinator`, and skips restart if `removalPendingFor(name)` or superseded. `lateCleanupRecord.owed(name)` remains true while ANY stopped process recorded via `owe` still has not completed its cleanup, and clears only once every recorded process has finished | An operator restart arriving while another restart or reload holds `txLock`/controller is answered with its refusal, because two generations cannot take one name |
| AC-5 | The respawn limit disables a plugin while a `SetStage` transition is in flight | `ze_plugin_status{plugin=name}` is deleted and stays deleted regardless of goroutine scheduling between `SetStage` and `Respawn` | The `ze_plugin_restarts_total`, `ze_plugin_events_delivered_total`, `failed`, and `slow` counters survive, because deleting counters mid-lifetime breaks Prometheus `rate()`/`increase()` |
| AC-6 | A restart fails because a stopped process still owes its runtime cleanup | Both the `doctor-plugin-stopped` report-bus warning (`ze show warnings`, daemon log) and `ze show doctor`'s `plugin-stopped` row come from one shared declaration in `process/stopped.go`, carry the cleanup-owed refusal explanation and `privilege.RestartRemedy()` last resort in their message, and name `ze request plugin restart <name>` as `Fix`. Once the owed cleanup finishes, the bus warning refreshes to drop the cleanup-owed text while the plugin remains down | `ze show doctor` still deduplicates the bus warning via `ownedCodes` so one fault renders one row; `Fix` stays the bare restart command with no trailing prose (owner decision D2) |
| AC-7 | `ze request plugin restart <name>` is refused after operator invocation (controller busy, cleanup owed, daemon stopping, unconfigured under `txLock`, manager not started) | The plugin's respawn-limit disable, `respawnTimes`, `totalRespawns`, and existing `StopRespawnLimit` record remain intact. A restart that passes all pre-spawn refusals lifts the disable and counters atomically under `pm.mu` and starts the plugin | A restart that fails *after* spawning the replacement records `StopRestartFailed`, because the old disable was already lifted to run that attempt |
| AC-8 | A plugin holding a large route table (up to 1,000,000 installed routes) is restarted | `withdrawPluginRoutes` via `RIB.RemoveBatch` and `Store.ModifyOrDelete` completes well inside the 3 s `restartCleanupGrace` (measured and recorded before and after), so a single `ze request plugin restart <name>` succeeds without triggering `errReplacedCleanupLate` | A restart whose old process holds a genuinely hung engine call still expires at `restartCleanupGrace` (3 s) and reports the cleanup-owed failure, because that call ends only when ze restarts |
| AC-9 | `ze request plugin restart <name>` arrives during the boot window (before `startupComplete`, including once `bgp` Stage 2 opens SSH) | `handlePluginRestart` refuses immediately with an operator error stating startup is still in progress and naming `ze show doctor` then the restart once boot completes. Once boot completes, no cleanup is owed and `request plugin restart <name>` succeeds. Defense in depth: unbarriered handshakes never touch `s.coordinator`, `superviseProcess` clears stopped state only if `pm.GetProcess(name) == proc`, and Step (d) skips replaced processes | Boot-time failure-policy restarts after Phase 1 Step (d) remain allowed under D1's `txLock` + per-handshake `coord = nil` without disturbing Phase 2/3 tier barriers |
| AC-10 | A reload auto-loads a plugin whose startup fails (below `StageRunning` or during `runProcessGenerationReady` at `StageRunning`) and the reload is refused | `ze show doctor` reports the refused reload (`doctor-config-reload-refused`) and NO `doctor-plugin-stopped` row or bus warning for that plugin; `plugins` health stays healthy; `ze request plugin restart <name>` answers that it is not configured | A plugin the running configuration already names (boot or removal recovery), whose handshake failed, keeps its configured entry and `doctor-plugin-stopped` row, because the running configuration still requires it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Removes a config section whose plugin was already stopped, commits reload, then runs `ze show doctor` and `ze request plugin restart <name>` | `reloadConfig` -> `collectReloadStopProcesses` -> `stopCollectedProcesses` -> `finishRemovalScope` -> `UnconfigureProcess` -> `pluginRestartRefusal` / `checkPluginStopped` | `test/plugin/plugin-restart-removed-down.ci` |
| 2 | Runs `ze show doctor` while a forked plugin is restarting and stuck waiting on connect-back | `handlePluginRestart` -> `Respawn` (unlocked `pm.mu`, per-name `respawning`) -> concurrent `PluginDoctorSnapshot` | `test/plugin/plugin-restart-external-nonblocking.ci` |
| 3 | Runs `ze request plugin restart <name>` on a respawn-disabled plugin while cleanup is owed or daemon is stopping | `handlePluginRestart` -> `restartPluginReplacementReenabled` -> pre-spawn refusal preserves `disabled` and counters | `test/plugin/plugin-restart-refused-keeps-disable.ci` |
| 4 | Reloads a config that auto-loads a failing plugin, then checks `ze show doctor`, `ze show warnings`, `ze show health`, and `ze request plugin restart <name>` | `autoLoadForReload` -> `runPluginPhaseWithin` (`unconfigureFailedReloadStart`) -> skips `RecordStopped`, calls `UnconfigureProcess` | `test/plugin/reload-failed-autoload-no-stopped-row.ci` |
| 5 | Inspects `ze show warnings` and `ze show doctor` after a restart expired `restartCleanupGrace` | `awaitReplacedCleanup` -> `lateCleanups.owe` -> `RecordStopped(CleanupOwed: true)` -> shared `process.StoppedMessage`/`StoppedFix` | `TestStoppedBusWarningMatchesDoctorRowWhileCleanupOwed` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReloadRemovingDownNonFIBPluginUnconfiguresOnAccept` | `internal/component/plugin/server/startup_removal_test.go` | AC-1 (non-FIB down plugin removed on accepted reload, cleared from `ConfiguredNames` and `Stopped`) | pass (`87ca99cd5b`) |
| `TestReloadRemovingDownFIBPluginUnconfiguresOnAccept` | `internal/component/plugin/server/startup_removal_test.go` | AC-1 (`ProgramsFIB` down plugin displaced/removed on accepted reload) | pass (`87ca99cd5b`) |
| `TestRejectedReloadKeepsDownPluginConfiguredAndStopped` | `internal/component/plugin/server/startup_removal_test.go` | AC-1 survivor (rejected deferred reload preserves down plugin's `pm.configs` and `pm.stopped`) | pass (`87ca99cd5b`) |
| `TestReloadFailureKeepsPreExistingDependency` (strengthened with `s.wg.Wait()`) | `internal/component/plugin/server/reload_test.go` | AC-2 deterministic repro and fix | red -> pass (`87ca99cd5b`) |
| `TestReloadFailureStopsReloadAutoLoadedDependencyOnly` | `internal/component/plugin/server/reload_test.go` | AC-2 survivor + `AutoLoadReason` guard | pass (`87ca99cd5b`) |
| `TestRespawnDoesNotBlockReadersOrUnrelatedRespawn` | `internal/component/plugin/process/manager_respawn_retirement_test.go` | AC-3 (`GetProcess`, `AllProcesses`, `ConfiguredNames`, and `Respawn("other")` answer promptly during 30 s connect-back) | pass (`ed3e6f7ad5`) |
| `TestRespawnInDaemonExitNoOverlap` | `internal/component/plugin/process/manager_respawn_retirement_test.go` | AC-3 (in-daemon engine calling `pm.GetProcess`/`AllProcesses` on exit raises no `doctor-plugin-restart-overlap`) | pass (`ed3e6f7ad5`) |
| `TestFailurePolicyRestartSerializesWithReloadAndBoundsHandshake` | `internal/component/plugin/server/failure_policy_test.go` | AC-4 (`txLock` ordering, `removalPendingFor`, `boundedRestartHandshake`, no deadlock with deferred peer-declaration change) | pass (`295f10ed26`) |
| `TestUnbarrieredHandshakeDoesNotClobberTierCoordinator` | `internal/component/plugin/server/restart_test.go` | AC-4, AC-9 (concurrent tier barrier intact while unbarriered restart runs) | pass (`295f10ed26`) |
| `TestLateCleanupRecordTracksMultipleProcessesPerName` | `internal/component/plugin/server/restart_test.go` | AC-4 (`owed` stays true when P2 finishes before P1; clears only when both finish) | pass (`295f10ed26`) |
| `TestRespawnLimitDeleteStatusLabelSurvivesConcurrentSetStage` | `internal/component/plugin/process/manager_metrics_test.go` | AC-5 (under `-race`, status gauge label stays deleted while counters survive) | pass (`ed3e6f7ad5`) |
| `TestStoppedBusWarningMatchesDoctorRowWhileCleanupOwed` | `internal/component/plugin/doctor/checks_runtime_test.go` | AC-6 (shared declaration in `process/stopped.go` drives both bus warning and doctor row; refresh clears cleanup-owed text when cleanup finishes) | pass (`d15a406540`) |
| `TestRefusedRestartPreservesDisableAndRespawnCounters` | `internal/component/plugin/server/plugin_restart_guard_test.go` | AC-7 (cleanup-owed, controller-busy, and daemon-stopping refusals keep `Disabled`, `respawnTimes`, `totalRespawns`, and `StopRespawnLimit`; admitted restart lifts them) | pass (`ed3e6f7ad5`) |
| `TestModifyOrDeleteSinglePass` | `internal/core/rib/store/store_bart_test.go` | AC-8 (`Store.ModifyOrDelete` keeps or deletes accurately) | pass (`5a0006fb97`) |
| `TestRIBRemoveBatchMatchesSerialRemove` | `internal/core/rib/locrib/locrib_test.go` | AC-8 (`RemoveBatch` preserves best-path fallback, `OnChange` notifications, revision, and metrics) | pass (`5a0006fb97`) |
| `TestWithdrawPluginRoutesLargeTableWithinGrace` & `BenchmarkWithdrawPluginRoutes` | `internal/component/plugin/server/dispatch_route_test.go` | AC-8 (large-table withdrawal completes well under 3 s `restartCleanupGrace`, with recorded before/after benchmark numbers) | pass (`5a0006fb97`; 1M 3.192 s -> 1.806 s) |
| `TestRestartRefusedDuringBootWindow` | `internal/component/plugin/server/plugin_restart_guard_test.go` | AC-9 (refused before `startupComplete`, succeeds after; no owed cleanup) | pass (`295f10ed26`) |
| `TestSuperviseStaleProcessDoesNotClearStopped` | `internal/component/plugin/server/failure_policy_test.go` | AC-9 defense in depth (`superviseProcess(old)` does not wipe down record when `GetProcess(name) != old`) | pass (`295f10ed26`) |
| `TestFailedReloadAutoLoadLeavesNoStoppedRecordOrConfig` | `internal/component/plugin/server/startup_removal_test.go` | AC-10 (both below-`StageRunning` and `runProcessGenerationReady` failures leave no `pm.configs`, `pm.stopped`, or `doctor-plugin-stopped` bus warning; boot failure still records it) | pass (`87ca99cd5b`) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A: no numeric operator input | | | | |

### Functional Tests
Each new `.ci` is drafted under `test/draft/plugin/` and promoted (`/ze-test`).

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-restart-removed-down` | `test/plugin/plugin-restart-removed-down.ci` | A configured internal plugin goes down, a committed reload removes its section, `ze show doctor` shows no stopped row and `ze request plugin restart <name>` refuses it as unconfigured | red -> pass (80/80 stress-repro, promoted) |
| `plugin-restart-external-nonblocking` | `test/plugin/plugin-restart-external-nonblocking.ci` | Restarting a forked plugin whose replacement stalls connect-back does not block `ze show doctor` or restarting another plugin | pass (80/80 stress-repro, promoted) |
| `plugin-restart-refused-keeps-disable` | `test/plugin/plugin-restart-refused-keeps-disable.ci` | A respawn-disabled plugin (internal and forked) whose restart is refused while cleanup is owed keeps its disabled state until an admitted restart runs | red -> pass (80/80 stress-repro, promoted) |
| `reload-failed-autoload-no-stopped-row` | `test/plugin/reload-failed-autoload-no-stopped-row.ci` | A refused reload whose auto-loaded plugin fails its handshake leaves `ze show doctor` with `doctor-config-reload-refused` only (no `doctor-plugin-stopped`, `plugins` health up, restart refused as unconfigured) | red -> pass (80/80 stress-repro, promoted) |

Existing tests re-run because their contract is reached (`ai/rules/testing.md`, "The Affected Population"): `test/plugin/plugin-restart-external-running.ci`, `test/plugin/plugin-restart-external-ignores-stop.ci`, `test/plugin/doctor-plugin-stopped.ci`, `test/plugin/failure-policy-restart.ci`, `test/plugin/failure-policy-fatal.ci`, `test/plugin/drain-respawn-refused.ci`, `test/plugin/shutdown-is-prompt.ci`, `test/reload/tx-protocol-external-plugin.ci`.

### Lifecycle scenarios (`ai/rules/testing.md`, "Lifecycle Regressions")
| Scenario | Reached? | Proof or reason |
|----------|----------|-----------------|
| Process ends after it declared (`restart`, `ignore`, `fatal`) | Yes (`restart` takes `txLock` + `boundedRestartHandshake`) | `test/plugin/failure-policy-restart.ci`, `TestFailurePolicyRestartSerializesWithReloadAndBoundsHandshake`, `test/plugin/failure-policy-fatal.ci` |
| A plugin fails a startup stage after it declared | Yes (reload auto-load failure vs boot failure) | `test/plugin/reload-failed-autoload-no-stopped-row.ci`, `TestFailedReloadAutoLoadLeavesNoStoppedRecordOrConfig` |
| A restarting plugin passes its restart bound | Yes (status gauge label + disable preserved on refusal) | `TestRespawnLimitDeleteStatusLabelSurvivesConcurrentSetStage`, `test/plugin/plugin-restart-refused-keeps-disable.ci` |
| The `respawn` leaf disagrees | Not reached | Stage 1 refusal (`refuseRespawnDisagreement`) is untouched |
| A plugin exits during the stop drain or teardown | Yes (pre-spawn refusal preserves disable) | `test/plugin/drain-respawn-refused.ci`, `TestRefusedRestartPreservesDisableAndRespawnCounters` |
| A signal arrives during the stop | Not reached | Hub stop signal table is untouched |
| A reload or commit arrives after shutdown began | Not reached | Shutdown gate in hub is untouched |
| A plugin crashes during verify/apply, misses deadline, or reports `broken` | Yes (`restartPlugin` rollback path) | `test/reload/tx-protocol-external-plugin.ci` |
| A reload's context ends during handshake, or a sibling fails at barrier | Yes (rolled-back candidate auto-load stop + `coord` isolation) | `TestReloadFailureKeepsPreExistingDependency`, `TestFailedReloadAutoLoadLeavesNoStoppedRecordOrConfig` |
| A plugin dispatches another plugin's command at startup | Yes (boot-window restart refusal) | `TestRestartRefusedDuringBootWindow` (seam: `.ci` cannot hold the boot window reliably) |

### Interop Tests (Scope: protocol)
N-A: no wire-visible change.

## Files to Modify
- `internal/component/plugin/process/manager.go` - items 1, 3, 5, 7 (per-name `respawning`, leaf `statusMu`, `RespawnReenabled` / `respawn` post-refusal re-enable, `ConfigFor` helper if needed)
- `internal/component/plugin/process/metric_hooks.go` - item 5 (synchronize `stageChange` under `statusMu` with `metricHooks.Load() == hooks`)
- `internal/component/plugin/process/stopped.go` - items 6, 7 (`PluginStop.CleanupOwed`, shared `StoppedMessage` / `StoppedFix` using `privilege.RestartRemedy`, `ClearStoppedCleanupOwed`)
- `internal/component/plugin/server/register_plugin_restart.go` - items 7, 9 (boot-window `!startupComplete()` refusal, `pluginRestartRefusal` under `txLock`, remove premature `pm.Reenable`, call `restartPluginReplacementReenabled`)
- `internal/component/plugin/server/reload_tx.go` - items 4, 6, 7 (`restartPluginAfterExit` acquires `txLock` + `boundedRestartHandshake`, reenable flag on `restartPluginReplacement`/`restartPluginProcess`, preserve stop record on pre-spawn refusal, set `CleanupOwed` on `errReplacedCleanupLate`)
- `internal/component/plugin/server/restart.go` - items 4, 6, 9 (`lateCleanupRecord` multi-process slice per name + bus refresh on clear; `runUnbarrieredStartupHandshake` passes `nil` coord; remove `lifetimeRestartHandshake`)
- `internal/component/plugin/server/failure_policy.go` - item 9 (`superviseProcess` clears stop record only when `pm.GetProcess(proc.Name()) == proc`)
- `internal/component/plugin/server/startup.go` - items 4, 9, 10 (explicit `coord` parameter on startup handshake helpers; Step (d) skips replaced processes; gate `RecordStopped` and call `UnconfigureProcess` on failed candidate reload auto-loads)
- `internal/component/plugin/server/startup_removal.go` - items 1, 2, 10 (`s.unconfigureOnAccept[scope]` finalized in `finishRemovalScope`; `provisional` parameter on `notifyPluginRemoval`/`rollbackStartupProcess`; `unconfigureFailedReloadStart`)
- `internal/component/plugin/server/startup_autoload.go` - items 1, 2 (record process-less removals in `stopCollectedProcesses`; direct final stop in `autoStopPluginNames`; process-less down plugins and `AutoLoadReason` guard in `collectOrphanedDependencies`)
- `internal/component/plugin/server/reload_dataplane_writer.go` - item 1 (include configured process-less plugins in `collectNonFIBRemovedProcesses` and `judgeReloadWriters`)
- `internal/component/plugin/server/dispatch_route.go` - item 8 (`withdrawPluginRoutes` uses `locrib.RIB.RemoveBatch`)
- `internal/component/plugin/doctor/checks_runtime.go` - item 6 (`pluginStoppedRow` calls shared `process.StoppedMessage` / `process.StoppedFix`; update stale comment on process-less removed plugins)
- `internal/core/rib/store/store_bart.go`, `internal/core/rib/store/store_map.go` - item 8 (`Store.ModifyOrDelete` for both default BART and `-tags maprib` builds)
- `internal/core/rib/locrib/manager.go` - item 8 (`Remove` uses `ModifyOrDelete` + single `shardIdx`; chunked per-shard `RemoveBatch`)
- `internal/core/rib/locrib/metrics.go` - item 8 (cached shard label strings + `recordRemoveBatch`)
- `docs/architecture/api/process-protocol.md` - items 4, 6, 7, 9 (update failure-policy restart lock + handshake bound, cleanup-owed bus warning, re-enable after refusals, boot-window refusal)
- `docs/architecture/plugin/plugin-system.md` - item 4 (update failure-policy startup bound sentence under "Startup phases and callbacks")
- `docs/architecture/doctor-and-health-checks.md` - items 1, 6, 10 (update plugin-stopped for process-less removed plugins, failed reload auto-loads, and shared cleanup-owed bus/doctor declaration)
- `plan/journal/retained-entry-still-answers-lookups.md`, `plan/journal/reconciler-claims-state-another-owner-installed.md`, `plan/journal/lock-held-across-a-join.md`, `plan/journal/two-owners-share-one-name.md`, `plan/journal/lifetime-ends-before-its-last-reader.md`, `plan/journal/fix-command-refused-in-the-state-it-reports.md`, `plan/journal/record-written-before-the-operation-succeeds.md`, `plan/journal/bound-too-small-for-its-own-burst.md`, `plan/journal/new-caller-assumes-the-old-callers-timing.md` - mark owned rows fixed as each phase lands

## Files to Create
- `test/plugin/plugin-restart-removed-down.ci`
- `test/plugin/plugin-restart-external-nonblocking.ci`
- `test/plugin/plugin-restart-refused-keeps-disable.ci`
- `test/plugin/reload-failed-autoload-no-stopped-row.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No new RPC or config leaf; `ze-system-cmd.yang` `request plugin restart` unchanged |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | `request plugin restart <name>` keeps its syntax; gains boot-window refusal and stops offering removed down plugins |
| CLI grammar (keyword before value) | N-A | No new command or keyword |
| Editor autocomplete | Yes | `completePluginRestart` (`register_plugin_restart.go`) automatically stops offering removed down plugins once `UnconfigureProcess` drops them from `ConfiguredNames` |
| Functional test for new RPC/API | Yes | Four `.ci` tests under `test/plugin/` covering items 1, 3, 7, 10 through `ze cli` |
| Pipe completeness | N-A | No new output path |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | Yes | Existing `doctor-plugin-stopped` (`process/stopped.go`, `doctor/checks_runtime.go`, runtime phase) and `doctor-config-reload-refused`; no new code needed; bus warning and doctor row now share one cleanup-owed message/fix declaration, and boot-window refusal returns an `opererr.Error` |
| Prometheus counters/metrics | Yes | Existing `ze_plugin_status` gauge (`process/manager.go`, `metric_hooks.go`) synchronized on disable; `ze_plugin_restarts_total`, `ze_plugin_events_delivered_total`, and `ze_locrib_shard_removes_total` / `ze_locrib_shard_depth` preserved |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP family, capability, or attribute touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Lifecycle defect fixes only; no feature level changes |
| 2 | Config syntax changed? | No | No config syntax change |
| 3 | CLI command added/changed? | No | `request plugin restart <name>` syntax unchanged; `docs/guide/command-reference.md` checked for any claim about boot-window restarts |
| 4 | API/RPC added/changed? | No | No RPC signature or answer schema changed |
| 5 | Plugin added/changed? | No | Infrastructure fixes only |
| 6 | Has a user guide page? | No | `docs/guide/plugins.md` and `docs/guide/health-checks.md` describe general restart/doctor behavior without the internal lock/cleanup-owed details; checked in Phase 4 |
| 7 | Wire format changed? | No | Verified in Changed-Wire Census |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` ("Failure-Policy Declaration (Stage 1)": failure-policy restart takes `txLock` + `boundedRestartHandshake`, shared cleanup-owed warning, re-enable after refusal, boot-window refusal) |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement changed |
| 10 | Test infrastructure changed? | No | Reuses existing `test/plugin/` runner and fixtures |
| 11 | Affects daemon comparison? | No | Internal lifecycle hardening |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/architecture/doctor-and-health-checks.md`; `docs/architecture/rib/unified-locrib.md` checked for `Remove`/`RemoveBatch` anchor in Phase 6 |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | Existing metric names and labels unchanged |
| 15 | Registered plugin, event type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: modified pages are `docs/architecture/api/process-protocol.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/doctor-and-health-checks.md`. Declared `// Design:` pages unaffected by this change: `docs/architecture/api/commands.md` (no command/pipe change), `docs/architecture/config/transaction-protocol.md` (`DeferReloadAcceptance` contract preserved), `docs/architecture/forked-route-install.md` (`withdrawPluginRoutes` contract preserved), `docs/architecture/plugin/rib-storage-design.md` (`Store` parity preserved across BART and `maprib`), `docs/architecture/rib/unified-locrib.md` (`OnChange` contract preserved), `docs/architecture/rib/forward-handle.md` (`Change.Forward` untouched) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No syntax or example changes |

## Design presentation

### What changes

This spec fixes the remaining ten lifecycle defects around `ze request plugin restart`, failure-policy restarts, reload removals, and large-table plugin cleanup. A plugin that was already down when a committed reload removed its configuration now leaves the configured set cleanly (no stale tab completion, no restart acceptance, and no lingering `doctor-plugin-stopped` row), and a refused reload whose auto-loaded plugin failed startup no longer leaks a `doctor-plugin-stopped` warning or `plugins` health failure. Rejected reloads stop only their own rolled-back auto-loads without touching pre-existing dependencies. `ProcessManager.Respawn` serializes per plugin name instead of globally across the 30 s connect-back wait. Failure-policy restarts serialize with reloads through `txLock` under `boundedRestartHandshake` (departing from `process-protocol.md`'s old unbounded unlocked rule), unbarriered handshakes stop clobbering `s.coordinator`, and `lateCleanupRecord` tracks every stopped process owing cleanup per name. Respawn-limit disables synchronize `ze_plugin_status` deletion against concurrent `SetStage` calls, operator restarts lift disable/counters only after all pre-spawn refusals pass, and restarts during the boot window are refused cleanly until startup completes. The bus warning (`ze show warnings`) and `ze show doctor` share one cleanup-owed message and fix declaration in `process/stopped.go` and refresh when cleanup finishes. Finally, `Store.ModifyOrDelete` and chunked per-shard `locrib.RIB.RemoveBatch` bring 1M-route withdrawal well inside the 3 s `restartCleanupGrace` bound (down from 3.192 s measured).

### Before and after

1. **Removing a section whose plugin is already down (Item 1):**
   - *Before:* Commit reload removing `fib-dummy` while it has no process → `ze show doctor` still prints `ERROR [doctor-plugin-stopped] configured plugin fib-dummy is not running`, tab completion still offers `fib-dummy`, and `ze request plugin restart fib-dummy` respawns it from stale config.
   - *After:* On accepted reload, `finishRemovalScope` unconfigures `fib-dummy`; `ze show doctor` is clean, tab completion omits `fib-dummy`, and `ze request plugin restart fib-dummy` answers `plugin fib-dummy is not a configured plugin`.
2. **Refused reload whose auto-load failed handshake (Item 10):**
   - *Before:* Refused reload logs `doctor-config-reload-refused` AND leaves `doctor-plugin-stopped` active on the report bus, turning `ze show health` (`plugins`) DOWN and showing `fix: ze request plugin restart <name>` in `ze show warnings` for a plugin that is not configured.
   - *After:* Only `doctor-config-reload-refused` is reported; `plugins` health stays up and `ze show warnings` carries no `doctor-plugin-stopped` warning for the rejected candidate.
3. **Cleanup-owed warning in `ze show warnings` vs `ze show doctor` (Item 6):**
   - *Before:* `ze show doctor` explains that restarts are refused until cleanup finishes and names the daemon-restart last resort, while `ze show warnings` prints only the bare failure and `fix: ze request plugin restart <name>` without the refusal or last resort.
   - *After:* Both surfaces render from one shared declaration in `process/stopped.go`, carrying the same cleanup-owed refusal explanation and `privilege.RestartRemedy()` last resort in the message and `ze request plugin restart <name>` as `Fix`; when the cleanup finishes, the bus warning refreshes automatically.
4. **Refused restart on a respawn-disabled plugin (Item 7):**
   - *Before:* `ze request plugin restart <name>` fails because cleanup is owed or the daemon is stopping, yet `pm.Reenable(name)` already cleared `disabled` and all respawn counters.
   - *After:* Refused restarts leave `disabled`, `respawnTimes`, `totalRespawns`, and `StopRespawnLimit` unchanged; only an admitted restart clears them.
5. **Restart during the boot window (Item 9):**
   - *Before:* `ze request plugin restart <name>` sent over SSH right after `bgp` Stage 2 races Phase 1 Step (d), can erase a failed restart's stop record, fail startup, or nil `s.coordinator` mid-tier.
   - *After:* `ze request plugin restart <name>` answers immediately that plugin startup is still in progress and names `ze show doctor` then the restart once startup completes.
6. **Restarting a plugin holding 1,000,000 routes (Item 8):**
   - *Before:* `withdrawPluginRoutes` takes 3.192 s (exceeding the 3 s `restartCleanupGrace`), failing the restart with `errReplacedCleanupLate`.
   - *After:* `Store.ModifyOrDelete` + chunked per-shard `RIB.RemoveBatch` completes well inside 3 s, so one `ze request plugin restart <name>` succeeds.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep failure-policy restarts off `txLock` with unbounded `lifetimeRestartHandshake` and add ad-hoc checks in `reloadConfig` | A concurrent reload can still swap `SetConfigTree` mid-handshake (leaving the replacement on stale config) and a hung replacement stays wedged forever with no `doctor-plugin-stopped` row; `boundedRestartHandshake` already honors declared `apply-budget` up to 600 s |
| Change `Fix` itself when cleanup is owed (to `ze show doctor` or `privilege.RestartRemedy()`) | Breaks two pinned tests (`TestPluginStoppedRowWhileCleanupOwed`, `TestRestartCommandFailsWhenOldCleanupNeverEnds`), puts prose on the `fix:` line, or drops the restart command once cleanup clears |
| Widen `restartCleanupGrace` (e.g., to 15–30 s) without optimizing `locrib.RIB.Remove` | Stalls `txLock` and operator restarts up to 10× longer on a truly hung engine RPC while keeping the 4× BART trie traversal overhead and 2M label allocations |
| Allow `request plugin restart` during boot by waiting for Step (d) | Blocks `txLock` during boot, creates tier-dependency hazards if a Tier 0 provider stops mid-boot, and still requires eliminating the `s.coordinator` clobber |
| Unconfigure process-less removals inside `stopCollectedProcesses` directly | Drops a down plugin's configured entry and stop record even when an outer deferred reload scope (`DeferReloadAcceptance`) is later rejected |

### Documents departed from

- `docs/architecture/api/process-protocol.md`, section `5-Stage Startup Protocol` ("Failure-Policy Declaration (Stage 1)"):
  - *Before:* `A restart the failure policy makes holds no lock: its replacement's handshake runs as long as the daemon does, so a slow replacement still comes up, and only its rollback wait is bounded.`
  - *After:* A failure-policy restart acquires the config transaction lock (`txLock.acquire`) before its per-name restart controller so it cannot race a config reload or removal, and bounds its replacement's handshake with `boundedRestartHandshake` (five stage windows plus any accepted `apply-budget` and `started-handler` window, the same bound `request plugin restart` uses). A replacement that does not reach running within that bound is rolled back and recorded stopped (`doctor-plugin-stopped`), releasing `txLock`.
- `docs/architecture/plugin/plugin-system.md`, section `Startup phases and callbacks`:
  - *Before:* `Unbarriered ad-hoc and failure-policy startup retain their caller's lifetime as their only bound.`
  - *After:* Unbarriered ad-hoc sessions retain their caller's lifetime as their only bound; failure-policy restarts use `boundedRestartHandshake` under `txLock`, honoring any accepted `apply-budget` and `started-handler` window.
- `docs/architecture/doctor-and-health-checks.md`, section `Plugin lifecycle checks`:
  - *Before:* `A plugin that was already down (a failed start left it configured with no process) is not collected by the removal at all, so it keeps its record and this check keeps reporting it, with a restart as its fix.`
  - *After:* A plugin that was already down when a reload removes its config roots or displaces its FIB selection is recorded on the reload acceptance scope and dropped by `ProcessManager.UnconfigureProcess` when the reload is accepted, clearing its configured entry, stop record, and `doctor-plugin-stopped` row; a rejected reload preserves them.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Item 3 scope (`pm.respawnMu` after `bc532d7502`) | (a) Narrow `pm.respawnMu` to per-name + add AC-3 tests + close journal row; (b) Keep global `pm.respawnMu` | Decided 2026-11-25: (a) narrow to per-name + add AC-3 tests |
| C-2 | Item 4 failure-policy restart exclusion (D1) | (a) Take `txLock.acquire` + `boundedRestartHandshake` (departing from `process-protocol.md` line 1209); (b) Keep unlocked/unbounded | Decided 2026-11-25: (a) `txLock.acquire` + `boundedRestartHandshake` |
| C-3 | Item 6 cleanup-owed fix surface (D2) | (a) One shared producer in `process/stopped.go`, keep `Fix` as `RestartCommand(name)`, put refusal + last resort in `Message` on both bus and doctor, refresh bus on clear; (b) Change `Fix` itself | Decided 2026-11-25: (a) shared producer with clean `Fix` + shared `Message` + bus refresh |
| C-4 | Item 8 large-table cleanup (D3) | (a) Optimize `Store.ModifyOrDelete` + chunked `RIB.RemoveBatch`, keep 3 s `restartCleanupGrace`; (b) Widen `restartCleanupGrace` | Decided 2026-11-25: (a) optimize `locrib` + keep 3 s bound |

## Implementation Steps

Each phase lands as one self-contained commit with its scoped tests and doc/journal updates.

1. **Phase: Wiring & Deterministic Repros (MANDATORY FIRST)** -- draft failing `.ci` tests and deterministic repros
   - Tests: `test/draft/plugin/plugin-restart-removed-down.ci`, `test/draft/plugin/reload-failed-autoload-no-stopped-row.ci`, `test/draft/plugin/plugin-restart-refused-keeps-disable.ci`, `test/draft/plugin/plugin-restart-external-nonblocking.ci`; add `s.wg.Wait()` repro in `TestReloadFailureKeepsPreExistingDependency`.
   - Verify: new tests fail against current code before fixes land.
2. **Phase: Reload removals, pre-existing dependencies & failed auto-loads (Items 1, 2, 10)** -- AC-1, AC-2, AC-10
   - Files: `startup_removal.go`, `startup_autoload.go`, `reload_dataplane_writer.go`, `startup.go`, `docs/architecture/doctor-and-health-checks.md`, `plan/journal/retained-entry-still-answers-lookups.md`, `plan/journal/reconciler-claims-state-another-owner-installed.md`.
   - Tests: AC-1, AC-2, AC-10 unit tests + promote `plugin-restart-removed-down.ci` and `reload-failed-autoload-no-stopped-row.ci`.
3. **Phase: Per-name respawn lock, synchronized status gauge & atomic re-enable (Items 3, 5, 7)** -- AC-3, AC-5, AC-7
   - Files: `process/manager.go`, `process/metric_hooks.go`, `server/register_plugin_restart.go`, `server/reload_tx.go`, `plan/journal/lock-held-across-a-join.md`, `plan/journal/lifetime-ends-before-its-last-reader.md`, `plan/journal/record-written-before-the-operation-succeeds.md`.
   - Tests: AC-3, AC-5, AC-7 unit tests + promote `plugin-restart-external-nonblocking.ci` and `plugin-restart-refused-keeps-disable.ci`.
4. **Phase: Failure-policy `txLock`, multi-process `lateCleanupRecord`, coordinator isolation & boot guard (Items 4, 9)** -- AC-4, AC-9
   - Files: `server/reload_tx.go`, `server/restart.go`, `server/startup.go`, `server/failure_policy.go`, `server/register_plugin_restart.go`, `docs/architecture/api/process-protocol.md`, `plan/journal/two-owners-share-one-name.md`, `plan/journal/new-caller-assumes-the-old-callers-timing.md`.
   - Tests: AC-4, AC-9 unit tests + re-run existing `test/plugin/` lifecycle `.ci` suite.
5. **Phase: Shared cleanup-owed bus warning and doctor row (Item 6)** -- AC-6
   - Files: `process/stopped.go`, `server/reload_tx.go`, `server/restart.go`, `doctor/checks_runtime.go`, `docs/architecture/api/process-protocol.md`, `docs/architecture/doctor-and-health-checks.md`, `plan/journal/fix-command-refused-in-the-state-it-reports.md`.
   - Tests: `TestStoppedBusWarningMatchesDoctorRowWhileCleanupOwed` + existing `TestPluginStoppedRowWhileCleanupOwed` and `TestRestartCommandFailsWhenOldCleanupNeverEnds`.
6. **Phase: Fast large-table route withdrawal (Item 8)** -- AC-8
   - Files: `internal/core/rib/store/store_bart.go`, `internal/core/rib/locrib/manager.go`, `internal/core/rib/locrib/metrics.go`, `internal/component/plugin/server/dispatch_route.go`, `plan/journal/bound-too-small-for-its-own-burst.md`.
   - Tests: `TestModifyOrDeleteSinglePass`, `TestRIBRemoveBatchMatchesSerialRemove`, `TestWithdrawPluginRoutesLargeTableWithinGrace`, `BenchmarkWithdrawPluginRoutes` (record before/after 100k/500k/1M numbers in spec and commit body).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has an implementation at file:line and a passing test |
| Feature completeness | Every user story passes through real CLI/daemon entry points (internal and forked) |
| Correctness | Global lock order `txLock` -> `restartControllers` -> `pm.mu` -> `statusMu` (leaf) has zero inversions; `applyFailurePolicy` generation test holds; rejected reloads never drop down plugins |
| Naming | No new diagnostic codes or metric names; existing `doctor-plugin-stopped` and `ze_plugin_status` kept intact |
| Data flow | No plugin names hardcoded in engine/config/RIB code; `internal/core/rib` adds no upward imports |
| Operator errors and doctor codes | Boot-window refusal and all restart refusals return `opererr.Error` with a concrete fix; bus warning and doctor row derive from one shared `process/stopped.go` producer |
| Rule: stale-comments | Update comments on `checks_runtime.go` (`checkPluginStopped`), `metric_hooks.go` (`clearMetricHooks`), `restart.go` (`lateCleanupRecord`, `runUnbarrieredStartupHandshake`), `reload_tx.go` (`restartPluginAfterExit`), and `manager.go` (`Respawn`, `ConfiguredNames`) |
| Rule: goroutine-lifecycle | Zero new goroutines added |
| Rule: no-layering | Remove `lifetimeRestartHandshake` and global `s.coordinator` nil-and-restore rather than keeping both paths |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Four new `.ci` files promoted | `ls` of all four paths under `test/plugin/`; none left under `test/draft/` |
| Shared cleanup-owed declaration has one owner | `grep` shows cleanup-owed message/last-resort text only in `process/stopped.go`, called by `recordStoppedLocked` and `pluginStoppedRow` |
| `s.coordinator` never nilled by unbarriered handshakes | `grep s.coordinator internal/component/plugin/server/restart.go` returns zero matches |
| 1M-route withdrawal fits well inside 3 s | `BenchmarkWithdrawPluginRoutes` / `TestWithdrawPluginRoutesLargeTableWithinGrace` output pasted in spec and commit |
| All 10 owned journal rows updated | `grep` of all 9 journal class files confirms every owned 2026-11-12 row marked `fixed:` with commit SHA |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Respawn-limit bypass | A refused `request plugin restart` must never reset `disabled`, `respawnTimes`, or `totalRespawns`, preventing a crash loop from bypassing limits via forced refusals |
| Stale registration resurrection | A removed down plugin or failed reload auto-load must never remain restartable via `request plugin restart` |
| Lock starvation / DoS | `Respawn` per-name locking keeps one slow/stuck forked plugin from blocking `ze doctor` or restarts of other plugins; `RIB.RemoveBatch` chunks per-shard lock holds to 1,024 routes |

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
- Commit `bc532d7502` already released `pm.mu` before `old.Stop()`, `old.Wait`, and `startAdmitted`, withdrawing `old` from `pm.processes` first. Only global `pm.respawnMu` and explicit AC-3 tests remained of item 3.
- Commit `d73a860caf` added per-name `restartControllerSet`, closing the direct operator-vs-failure-policy race of item 4, while leaving `lateCleanupRecord` 1-per-name and `restartPluginAfterExit` unserialized against `reloadConfig` and clobbering `s.coordinator`.
- Commit `6da64fc95a` (`spec-fib-dummy` AC-17) added `UnconfigureProcess` at `finishRemovalScope` and `rollbackStartupProcess`, closing the running-plugin half of item 1, while process-less down plugins are skipped by `s.removals` and `startup.go` still calls `RecordStopped` after unconfiguring failed reload auto-loads (item 10).
- Measuring `withdrawPluginRoutes` on 1,000,000 routes gave 3.192 s (exceeding `restartCleanupGrace` 3 s) due to four BART traversals per empty-group delete (`Modify` + `Delete`), duplicate prefix hashing, and 2M per-route `strconv.Itoa`/Prometheus label lookups.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `retained-entry-still-answers-lookups.md` | 2026-11-12 | item 1, second half: a plugin with no process when its roots go keeps its name and its `doctor-plugin-stopped` row. The first half (a removed plugin with a process stays in `ConfiguredNames`) landed in `6da64fc95a` (`spec-fib-dummy` AC-17); this spec's closure marks the row fixed |
| `reconciler-claims-state-another-owner-installed.md` | 2026-11-12 | item 2: rejected reload's recovery may stop a pre-existing dependency |
| `lock-held-across-a-join.md` | 2026-11-12 | item 3: `Respawn` holds lock across join and connect-back (`pm.mu` fixed by `bc532d7502`; this spec narrows `pm.respawnMu` to per-name, adds AC-3 tests, and closes the row) |
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
| Track process-less reload removals in `s.unconfigureOnAccept[scope]` finalized by `finishRemovalScope(scope, true)` | Unconfigure inside `stopCollectedProcesses` directly | Preserves provisional reload semantics: a rejected deferred reload scope keeps the down plugin configured and reported down |
| Stop rolled-back candidate auto-loads directly (`provisional = false`) and guard `collectOrphanedDependencies` with `AutoLoadReasonFor` | Only patch `collectOrphanedDependencies` | Rolled-back candidates were never committed; keeping them out of `s.removals` eliminates the spurious background `recoverPluginRemoval` goroutine completely |
| Replace manager-wide `pm.respawnMu` with per-name `respawning map[string]chan struct{}` under `pm.mu` | Keep manager-wide `pm.respawnMu` | Matches `restartControllerSet` and prevents a 30 s connect-back wait on one plugin from blocking respawns of unrelated plugins |
| Failure-policy restart acquires `txLock.acquire(s.ctx)` before `restartControllers` and uses `boundedRestartHandshake` (D1) | Keep unlocked/unbounded `lifetimeRestartHandshake` | Prevents config-tree/removal races and wedged replacements; honors declared `apply-budget` up to 600 s; single lock order `txLock` -> `restartControllers` avoids deadlock with deferred peer-declaration admission |
| Pass `coord *plugin.StartupCoordinator` explicitly per handshake | Keep global `s.coordinator` nil-and-restore | Removes shared mutable state and fixes concurrent tier barrier clobbering across both reloads and boot |
| Synchronize `stageChange` and `deletePluginStatusLabel` with leaf mutex `statusMu` + `p.metricHooks.Load() == hooks` | Use `pm.mu` or `SeriesCustody` | Leaf lock cannot invert with `pm.mu`/`stageMu` and guarantees linearizable deletion while leaving counters intact |
| One shared `StoppedMessage`/`StoppedFix` in `process/stopped.go` carrying `PluginStop.CleanupOwed`, keeping `Fix` as `RestartCommand(name)` + bus refresh on clear (D2) | Change `Fix` itself to `ze show doctor` or `privilege.RestartRemedy()` | Keeps both surfaces identical from one declaration while preserving clean command-only `Fix` lines and existing tests |
| Defer operator re-enable into `pm.RespawnReenabled` under `pm.mu` after all pre-spawn refusals | Roll back `Reenable` on error in `handlePluginRestart` | Atomic with spawn admission; zero race window and no saved-counter restore logic needed |
| Add `Store.ModifyOrDelete` and chunked per-shard `RIB.RemoveBatch` while keeping 3 s `restartCleanupGrace` (D3) | Widen `restartCleanupGrace` to 15–30 s | Fixes the measured 4× BART traversal and 2M label-lookup bottleneck without weakening fast detection of genuinely hung engine RPCs |
| Refuse `request plugin restart` while `!s.startupComplete()`, plus defense-in-depth guards in `superviseProcess` and Step (d) | Wait for Step (d) during boot | Eliminates all three boot-window bugs cleanly without holding `txLock` during boot |

## Known Limitations
- Retiring an `ignore`-policy exit and making reloads restart configured down plugins remain owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` (D-A, D-B).
- `sourceGenerationCleanup` hooks inside `cleanupProcess` keep their own 35 s internal cancellation timeouts for hung BGP peer sessions; if a BGP plugin holds a hung peer session beyond 3 s during a restart, `restartCleanupGrace` still reports the cleanup-owed refusal as intended.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] No code snippets
- [ ] Current Behavior, Existing Mechanisms, Design documents, Data Flow, Changed-Wire Census and Design presentation sections completed
- [ ] AC-N rows carry testable positive assertions and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist and Documentation Update Checklist answered Yes/No/N-A

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: `./le test unit pkg path ./internal/component/plugin/process/`, `./le test unit pkg path ./internal/component/plugin/server/`, `./le test unit pkg path ./internal/component/plugin/doctor/`, `./le test unit pkg path ./internal/core/rib/store/`, `./le test unit pkg path ./internal/core/rib/locrib/`, and `./le test functional plugin` / `./le test functional reload` on the tests this spec names. No agent runs any `./le verify` command
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [x] Tests written (all 19 unit tests + 4 `.ci` functional tests)
- [x] Tests FAIL (AC-1 `plugin-restart-removed-down` exit 0 vs 1; AC-2 `TestReloadFailureKeepsPreExistingDependency` `reload_test.go:669`; AC-7 `plugin-restart-refused-keeps-disable` `StopRespawnLimit` overwritten; AC-10 `reload-failed-autoload-no-stopped-row` leaked `doctor-plugin-stopped`)
- [x] Tests PASS (all 5 packages + all 12 functional `.ci` tests green; 80/80 stress-repro under 64 burners)
- [x] Functional `.ci` tests for end-to-end behavior, internal and forked (promoted to `test/plugin/`)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-plugin-restart-bounded-and-configuration-true.md` only, in the same `./le commit create` script
