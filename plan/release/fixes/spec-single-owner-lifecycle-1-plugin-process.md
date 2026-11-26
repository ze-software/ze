# Spec: single-owner-lifecycle-1-plugin-process

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | A plugin that exits and is not restarted keeps answering for its name while nothing runs it, a peer whose OPEN check belongs to a plugin that is down can be accepted with no check (RFC 9234), a restarting plugin can be sent a runtime call in the middle of its handshake, and a plugin restarted during the stop is torn down again: shipped-daemon lifecycle defects an operator meets in production. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

Design pass of 2026-11-19 refreshed every claim at its producer after six
commits (aa6ebb8d15, 325ec5d0d6, 751f7c82a9, f145b152e1, 4a0c0f468a,
bc532d7502). Both decisions below were approved by Thomas on 2026-11-24 and
follow owner decision D1 of the parent ("D1 - yes you are right"): each defect
is closed at its existing choke point, with no owner object, no generation
counter and no source gate.

| ID | Question | Approved decision |
|----|----------|-------------------|
| D-A | Where does "this peer's OPEN must be checked by plugin P" live once P's process has ended and its registrations are released? | Approved (2026-11-24): keep the per-peer OPEN-policy capability declarations of a plugin that validates OPEN in the capability injector while the plugin stays configured, and record the plugin's name in a small "OPEN policy owed" record on the server. `PluginsWithPerPeerOpenPolicy` names a plugin when it runs or when the record holds it, so ze still advertises the Role capability (RFC 9234 Section 4.1) and rejects incoming peer OPENs fail-closed with Role Mismatch (RFC 9234 Section 4.2, AC-11) until a running generation replaces its declarations or a reload removes the plugin. |
| D-B | Does a config reload start a configured plugin that is down? | Approved (2026-11-24): every reload, with or without a config difference, restarts each configured plugin that holds a stop record and is not removed by the reload through `restartPluginReplacement` (the path `request plugin restart` uses, with its bounds, cleanup wait, late-cleanup refusal, admission and post-startup delivery). A plugin that fails to start refuses the reload as a whole, matching auto-load. A reload never lifts a respawn-limit disable: only manual `ze request plugin restart <name>` does (AC-3). |

## Task

Child 1 of `plan/release/fixes/spec-single-owner-lifecycle.md`, which holds the
decision record (owner decisions D1, D2, D3 of 2026-11-11) and the routing of
every journal row to a child. This child owns the plugin-process lifecycle.

Principle, from the parent: a plugin generation already has an identity (the
process pointer) and an owner (the supervising goroutine with its run-once
cleanup). Each defect below is a path that goes round them.

What the code already does today, so this spec does not build it: every path
that leaves a configured plugin down records why (`RecordStopped`) and raises
`doctor-plugin-stopped`; `ze request plugin restart <name>` starts a down
plugin again (via `restartPluginReplacementReenabled` / `RespawnReenabled`) and
is the fix doctor names; `restartReplacementWithin` skips `RecordStopped` on
pre-spawn `transaction.ErrRestartRefused` refusals (including `ErrDaemonStopping`);
`collectReloadStopProcesses` / `stopCollectedProcessesMode` / `finishRemovalScope`
/ `unconfigureProcessless` already collect and unconfigure removed down plugins on
reload acceptance (preserving them on rejection); `writeBatchWithDeadline` and
`MuxConn.CallBatchRPC` already check `ctx.Err()` before writing; and the process
registration is an atomic pointer.

The items this spec fixes:

1. A generation that ends with no successor (failure policy `ignore`, an
   undeclared or unknown policy, a restart refused before it stopped anything)
   removes injected capabilities directly (for `ignore`/unspecified/unknown),
   but keeps its dead process in the manager slot and keeps its registry row,
   commands, families, pipe aliases, answer shapes and loaded mark. Its
   commands and families stay published for a plugin nothing runs, and a reload
   whose difference touches its config roots fails its transaction against a
   dead connection.
2. The set of plugins whose answer a peer's OPEN needs is built from live
   slots and injected capabilities. After an `ignore` exit (which currently
   calls `RemovePluginCapabilities`), after a failed restart, and in the window
   of every restart, that set is empty for a peer whose role is configured, and
   the OPEN is accepted with no role check. This item lands before the retire
   replaces those arms, so every down path stays fail-closed.
3. A reload does not start a configured plugin that is down.
4. `Respawn` publishes the replacement before its handshake has run, and
   runtime senders pick processes without reading their stage.
5. The engine event bus calls a handler that was unsubscribed after the
   dispatch took its list.
6. A plugin that exits between the stop signal (`SignalShutdownRequested`) and
   the stop drain (`BeginDrain`) is started again, then torn down; and once
   `BeginDrain` runs after the config fence, reload and startup-phase admission
   stay open during the drain until `Server.Stop` runs `stopTransaction` and
   sets `stopEntered`.
7. A forked plugin whose program exits before it connects back holds its
   start for the 30-second connect-back wait.
8. Direct `Conn` regression coverage is still owed for the ended-context
   batch-write guard already landed in `pkg/plugin/rpc/conn.go`
   (`writeBatchWithDeadline`) and `pkg/plugin/rpc/mux.go` (`CallBatchRPC`), so
   the 2026-11-13 journal row can be proven and closed.

A sixth item of the first design, the firewall with `backend vpp` waiting for
VPP inside its config stage, left this spec on 2026-11-11 for
`plan/release/fixes/spec-firewall-vpp-apply-before-connect.md`. Two rows left
on 2026-11-19 ("Rows moved to another spec").

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/hub-engine.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/doors/forwarding.md`, `ai/doors/system-plugins.md`, `ai/doors/core.md`, `ai/doors/le-tooling.md`
  → Constraint: every plugin runs internal or forked; a lifecycle fix is proven in both modes (`ai/rules/testing.md`, "Lifecycle Regressions").
  → Constraint: one plugin failure stops the startup tier, by owner ruling; this spec does not make a tier survivable.
  → Constraint: the engine never enumerates plugins. The OPEN-policy record of D-A is keyed by what a plugin declared (a per-peer capability and a validate-open callback), never by a plugin name written in engine code.
  → Constraint: `pkg/` is a published contract. AC-6 tightens the `ze.EventBus` `Subscribe` contract on purpose, with its page.
  → Constraint: the external plugins and their modules are fixed while the daemon runs, so a reload never adds or removes an external plugin block.
- [ ] `docs/architecture/api/process-protocol.md`, "Failure-Policy Declaration (Stage 1)" through "Who stops the plugin server" (read 2026-11-19)
  → Decision: `ignore` means "the engine logs and carries on without it", and a plugin past its restart bound is "the `ignore` outcome". Both end in one down state here.
  → Constraint: the page names four down paths and one fix, `ze request plugin restart <name>`, whose order "lifts a respawn-limit disable". D-B keeps that the only thing that lifts it.
  → Constraint: "A stop ends every restart. From the start of the stop drain (`BeginDrain`)". AC-8 moves the start to the stop signal and rewrites that sentence.
  → Constraint: every restart waits at most `restartCleanupGrace` for the stopped process's cleanup, and "each later restart of the name is refused until that cleanup has run". Any start this spec adds keeps that refusal (AC-12).
  → Constraint: "Historical error events remain in the report ring" when a plugin's report source is cleared, so the crash report survives the retire (A-9).
  → Constraint: whole-daemon shutdown closes connections without `bye`; a live removal sends `bye removed`. Unchanged.
- [ ] `docs/architecture/doctor-and-health-checks.md`, "The runtime phase" (read 2026-11-19)
  → Constraint: a runtime check reads the daemon's plugin snapshot. `ddos-flowspec-stopped-record` reads "configured and stopped" from it, and a retired plugin stays configured, so that check keeps its answer.
- [ ] `docs/guide/health-checks.md` (read whole, 2026-11-19)
  → Constraint: the `doctor-plugin-stopped` rows (the doctor table and the report-bus table) state the fix command; the `plugins` row of "Registered Components" still reads "Plugin crashes and disabled-by-respawn-limit", which is stale against `health.Register` in the process package.
- [ ] `ai/rules/testing.md`, "Lifecycle Regressions"
  → Constraint: every scenario of that table the change reaches is proven by a `.ci`, in both modes; unreached scenarios are named with the reason.
- [ ] `docs/contributing/ze-go-style.md` (before the first Go edit)
  → Constraint: read in full by the implementing agent; the pre-write hook refuses a Go edit until then.

Not read by this design pass, each read by the phase that edits its subject, which records a conflict with this spec before coding: `docs/architecture/plugin/plugin-system.md`, `ai/digests/plugin-transport.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/core-design.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/api/ipc_protocol.md`, `docs/architecture/api/capability-contract.md`, `docs/architecture/plugin/component-boundaries.md`. The process-protocol section "Panic containment" was not located by this pass (A-7).

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc9234.txt`, Sections 4.1 and 4.2 (read 2026-11-19)
  → Constraint: Section 4.1: "If the BGP Role is locally configured, the eBGP speaker MUST advertise the BGP Role Capability in the BGP OPEN message." So the OPEN ze sends to a role-configured peer carries the capability while its plugin is down (D-A).
  → Constraint: Section 4.2: "If the Roles do not correspond, the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11)." A peer whose roles ze cannot confirm is refused with that pair, which `OpenValidationUnavailableError` already sends (owner decision, 2026-08-30).
- [ ] `rfc/short/rfc9234.md` - the phase that adds the AC-11 tests reads it and tags them through `/ze-rfc`

**Key insights:**
- The process pointer is the generation. Comparing the manager's process for a name with the process in hand is the currency test.
- Three paths already retire a process the same way (a failed startup, a removal, a failed restart): slot removed by pointer comparison, process stopped and joined, name-keyed registrations released, loaded mark cleared. The `ignore` exit is the path that skips it.
- `RemoveProcess` clears the stop record, so a stop is recorded after the slot is removed, as `restartReplacementWithin` already does.
- The stop has two ordered points: `stopUnderWay()` is true from `SignalShutdownRequested` (before the config fence), which ends crash-triggered respawns and mid-stop retires across the fence, drain, and teardown; `BeginDrain` runs right after `awaitReloadWorker` and `fenceCommitReloads` complete, sealing post-fence reload and startup-phase admission so already-admitted commit reloads finish cleanly while nothing starts during the drain. The server context ends much later, in `Server.Stop`, after both.
- `restartPluginProcess` is the only caller of `ProcessManager.Respawn`, so one read there governs every restart.

## Current Behavior (MANDATORY)

**Source files read:** each at its producing function (originally 2026-11-19, re-verified against HEAD on 2026-11-26).
- [ ] `internal/component/plugin/server/failure_policy.go` - `superviseProcess` clears the stop record (`ClearStopped`), serves the generation, then calls `applyFailurePolicy`. That function returns at once on `s.stopping`, on an ended server context, for a pending removal, and when the manager holds another process under the name. It raises the crash report, then: `restart` calls `restartPluginAfterExit`; `fatal` stops the daemon; `ignore`, unspecified and unknown call `s.capInjector.RemovePluginCapabilities(name)` and `RecordStopped` with cause `StopPolicyExit`, leaving the manager slot, registry row, families, pipe aliases, answer shapes and loaded mark intact. `stopDaemonForPlugin` reads `stopUnderWay()`.
- [ ] `internal/component/plugin/server/dispatch.go` - `cleanupProcess` runs once per process: the commands and pending requests of the process, its subscriptions, and by name its cache consumer, its installed routes, its report source and the cleanup hooks.
- [ ] `internal/component/plugin/server/rpc_register.go` - `RegisterProcessCleanup` appends to the hook list `runProcessCleanupHooks` walks with the process name.
- [ ] `internal/component/plugin/process/stopped.go` - `StopCause` has `StopRespawnLimit`, `StopPolicyExit`, `StopRestartFailed`, `StopHandshakeFailed` (and the refused zero value). `recordStoppedLocked` raises `doctor-plugin-stopped` with the fix `ze request plugin restart <name>`. `ClearStopped` and `RemoveProcess` clear it. `Reenable` (and atomic `RespawnReenabled` in `manager.go`) forgets the respawn history for the operator's restart.
- [ ] `internal/component/plugin/process/register.go` - the `plugins` health row probes `HealthProbeDown(CodeDoctorPluginStopped)`.
- [ ] `internal/component/plugin/server/restart.go` - `awaitReplacedCleanup` waits `restartCleanupGrace` (3 s) and, on expiry, records the old process in `lateCleanups` and fails the restart. `lateCleanupRecord.owed` forgets a name once its cleanup ran. `releasePluginRegistrations` waits for (or runs) the runtime cleanup, then releases by name: the registry row, injected capabilities, families, pipe aliases, answer shapes. `rollbackReplacement` takes the slot, stops, waits at most the stop bound, releases and clears the loaded mark.
- [ ] `internal/component/plugin/server/reload_tx.go` - `restartPlugin` has three callers (the `CodeBroken` rollback, `request plugin restart`, and the failure policy through `restartPluginAfterExit`), each under a `restartControllerSet` reservation. `restartPluginProcess` refuses a name that owes a late cleanup, calls `Respawn` (or `RespawnReenabled` when `reenable` is true), wraps pre-spawn refusals (`respawnRefused`, including `ErrDaemonStopping`) with `transaction.ErrRestartRefused`, awaits the old cleanup, releases the old registrations, holds peer declarations and runtime restoration, runs the handshake, reconciles declarations and delivers post-startup. `restartReplacementWithin` already skips `RecordStopped` on `transaction.ErrRestartRefused` (preserving any existing stop record, including on `ErrDaemonStopping`) and records `StopRestartFailed` only for failures after spawn.
- [ ] `internal/component/plugin/server/register_plugin_restart.go` - `handlePluginRestart` accepts any configured name (`ConfiguredNames`), running or not, takes the transaction lock, and calls `restartPluginReplacementReenabled` (which passes `reenable = true` to `ProcessManager.RespawnReenabled`, atomically lifting any respawn-limit disable only after pre-spawn admission passes).
- [ ] `internal/component/plugin/process/manager.go`, `manager_shutdown.go` - `Respawn` and `RespawnReenabled` refuse when respawns were refused, when shutdown owns the manager, when the name is disabled or unknown, or at a bound (recording `StopRespawnLimit`). They remove the old process from the slot, drop the lock, stop and join it, then `startAdmitted` publishes the replacement before any handshake. `RemoveProcess` leaves the name configured; `UnconfigureProcess` drops a processless name from `configs`, `disabled`, respawn history, and `stopped`. `AllProcesses` and `GetProcess` read no stage. `ProcessServing` answers the process that runs a registration, whatever its stage.
- [ ] `internal/component/plugin/process/process.go` - `registration` is an `atomic.Pointer`. `startExternal` starts `monitorCmd`, which closes `reaped` at the child's exit, then waits in `waiter.Wait` on the 30-second connect-back timeout alone.
- [ ] `internal/component/plugin/server/startup_removal.go` - `stopRemovedProcess`, `takeAndStopProcess` (slot removed only when it still holds this process), `releaseStoppedProcess`, plus `recordUnconfigureOnAccept`, `finishRemovalScope`, and `unconfigureProcessless`, which already unconfigure finalized running removals and removed configured-down (`processless`) plugins only when the reload scope is accepted (`accepted == true`), discarding `pendingUnconfigure` on rejection so down plugins stay configured and reported down.
- [ ] `internal/component/plugin/server/startup.go` - `runPluginStartup` builds boot's set from config-path owners, explicit blocks, unclaimed families, event types and send types, with dependencies. `runPluginPhaseWithin` refuses a plugin that owes a late cleanup and refuses when `stopEntered` is set, which `Server.Stop` sets (after the drain, not at `BeginDrain`).
- [ ] `internal/component/plugin/server/reload.go` - `reloadConfig` runs the compensation and removal-recovery retries, prepares the config, and returns "config reload: no changes" on an empty diff. With a diff it calls `collectReloadStopProcesses`, auto-loads for added/changed/removed paths, builds the affected set from `AllProcesses` and each registration's config roots, and runs the transaction, whose bridge reads a missing or closed connection as a failed verify. `txLock.closeAdmission()` is currently called only in `Server.Stop` (`stopTransaction`), after `BeginDrain` and the drain round.
- [ ] `internal/component/plugin/server/reload_dataplane_writer.go`, `internal/component/plugin/server/startup_autoload.go` - `collectReloadStopProcesses` and `collectNonFIBRemovedProcesses` already collect both running and configured-down plugins whose roots left (plus displaced FIB writers and orphaned dependencies), and `stopCollectedProcessesMode` already records `processless` configured names via `recordUnconfigureOnAccept(scope, name)` so `finishRemovalScope` unconfigures them on reload acceptance. `selectReloadAutoLoadWithReasons` skips names that are loaded, configured, or hold a slot; `startAutoLoaded` refuses the whole reload when a start fails.
- [ ] `internal/component/plugin/server/server.go` - `SignalShutdownRequested` closes `shutdownRequested` once; `stopUnderWay` reads it. `PluginsWithPerPeerOpenPolicy` intersects the live slots whose registration wants validate-open with the per-peer capabilities the injector holds for the peer's selectors; its comment rests on the release removing capabilities and registration together. `CallFilterUpdate` sends through `ProcessServing`, `CallDoctorCheck` through `GetProcess`, and `DoctorCheckPlugins` walks every slot; none reads a stage.
- [ ] `internal/component/plugin/registration.go` - `CapabilityInjector.planLocked` refuses a code already held for a peer, with no exemption for the plugin that holds it. `ReplacePluginCapabilities` replaces a named owner's whole declaration in one publication.
- [ ] `internal/component/plugin/server/source_policy.go` - `CaptureSourcePolicyOwners` takes each owner through `GetProcess` and captures its connection, with no stage read.
- [ ] `internal/component/plugin/server/resolver_cache.go` - `ResolverCacheOwners` walks every slot though its comment says "running plugins"; `CallResolverCacheClear` sends through `GetProcess`.
- [ ] `internal/component/plugin/server/drain.go` - `BeginDrain` calls `SignalShutdownRequested` and `RefuseRespawns`, but does not seal reload or startup-phase admission (`txLock.closeAdmission` / post-fence drain guard).
- [ ] `cmd/ze/hub/main.go` - the stop sequence starts with `apiServer.SignalShutdownRequested()`, then the force-signal channel, the announce, `close(reloadCh)` + `awaitReloadWorker`, `fenceCommitReloads` (which lets already-admitted commit reloads and confirmed-commit reverts finish during the config fence), and then `BeginDrain` after the config fence completes.
- [ ] `internal/component/plugin/server/doctor_snapshot.go` - `pluginDoctorState` reports `Stop`, `Down`, `Disabled`, `CleanupOwed` and the process state per configured name.
- [ ] `internal/component/plugin/doctor/checks_runtime.go` - `pluginStoppedRow` writes the stop message, the time and, when a cleanup is owed, why the restart is refused. The comment on `checkPluginStopped` already records that an accepted reload unconfigures a removed down plugin via `UnconfigureProcess` and a rejected reload preserves it.
- [ ] `internal/component/plugin/server/engine_event.go` - `dispatch` copies the handlers from a map and calls each; its comment states an unregistered handler "IS still called once more". `unregister` deletes the entry and waits for nothing.
- [ ] `pkg/ze/eventbus.go` - the `Subscribe` comment promises only that the returned function "removes the handler".
- [ ] `internal/component/bgp/server/validate.go` - `broadcastStampedValidateOpen` takes the policy plugins the reactor resolved, asks every slot through `askOpenValidators`, which reads no stage, and refuses the OPEN through `unansweredOpenValidation` when a policy plugin did not answer.
- [ ] `internal/component/bgp/server/events.go` - `addOpenRemovalObservers` reads the registration of the processes subscribed to the peer's state, not the manager's slots, and its comment says why it stays off the manager's lock. A process holds subscriptions only from Stage 5.
- [ ] `internal/component/bgp/reactor/policy_reload.go` - `policyReplayOwner` reads a bound process's registration through `GetProcess` to find a claimant and sends nothing to it.
- [ ] `pkg/plugin/rpc/conn.go`, `pkg/plugin/rpc/mux.go` - `writeFrame`, `writeBatchWithDeadline` (`conn.go:661-664`), and `MuxConn.CallBatchRPC` (`mux.go:330`) already return `ctx.Err()` before writing; `mux_batch_test.go` tests `MuxConn.CallBatchRPC` with a canceled context, while `conn_test.go` still lacks direct ended-context regression tests for `writeBatchWithDeadline` / `Conn.CallBatchRPC`.
- [ ] `internal/component/bgp/plugins/role/register.go` - `bgp-role` declares no failure policy, so its exit takes the `ignore` path. It owns the `bgp` config root.

**Behavior to preserve:**
- A `CodeBroken` restart that is refused leaves the running plugin serving, with every registration.
- `fatal` stops the daemon. A removal owns its own recovery. A startup failure rolls back as today.
- Daemon stop releases nothing by name and sends no `bye`.
- The restart bounds (`RespawnLimit` in `RespawnWindow`, `MaxTotalRespawns`), their counters, and the rule that only the operator's restart lifts a disable.
- `ze request plugin restart <name>` starts a down plugin and stays the fix `doctor-plugin-stopped` names.
- An exit during the stop still raises its crash report.
- A handler may register and unregister from inside a dispatch without deadlock.
- An OPEN whose per-peer policy plugin did not answer is refused with Role Mismatch.
- `addOpenRemovalObservers` stays off the process manager's lock.
- `test/plugin/drain-respawn-refused.ci`, `test/plugin/drain-plugin-failure-stop.ci` and `test/plugin/doctor-plugin-stopped.ci` keep their expectations.

**Behavior to change:**
- The eight items of the Task, as the acceptance criteria state them.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Generation supervisor | `internal/component/plugin/server/failure_policy.go`, `superviseProcess`, `applyFailurePolicy` | One goroutine per generation: serves it, then acts on its end; `ignore`/unspecified/unknown currently calls `capInjector.RemovePluginCapabilities` and `RecordStopped` without retiring the slot or other registrations | Extends: replaces those direct `RemovePluginCapabilities` arms with the shared retire path (skipping it once `stopUnderWay()` is true), followed by `RecordStopped` |
| Retire of a stopped process | `startup_removal.go` `takeAndStopProcess`, `releaseStoppedProcess`; `restart.go` `rollbackReplacement` | Slot removed by pointer comparison, process stopped and joined within its stop bound, name-keyed registrations released, loaded mark cleared. Used for a failed startup, a removal and a failed restart | Extends: the `ignore` outcomes and pre-spawn refused exits run the same steps. No second release list is written, and no release moves into the cleanup body |
| Run-once generation cleanup | `process.go` `RunRuntimeCleanupOnce`, `WaitRuntimeCleanup`; `dispatch.go` `cleanupProcess` | Runs the server's cleanup body once per process and lets a caller wait for it | Does not change: the retire runs after it, as every other caller of the release does |
| Stop record | `process/stopped.go` `RecordStopped`, `ClearStopped`, `Stopped`, `RestartCommand`; `process/manager.go` `RespawnReenabled`, `UnconfigureProcess` | Records each configured plugin that is down with its cause, raises `doctor-plugin-stopped` with the fix, clears at running and on accepted removal (`UnconfigureProcess`) | Extends: it is the set a reload restarts (D-B), read through one new accessor that lists the recorded names. The retire records after the slot removal |
| Operator restart path | `register_plugin_restart.go` `handlePluginRestart`; `reload_tx.go` `restartPluginReplacementReenabled`, `restartPluginReplacement`, `restartReplacementWithin`, `restartPluginProcess`; `restart_controller.go` `restartControllerSet` | `handlePluginRestart` restarts a configured plugin via `restartPluginReplacementReenabled` (`RespawnReenabled`) under one controller per name with bounds on every wait; `restartReplacementWithin` already skips `RecordStopped` on pre-spawn `transaction.ErrRestartRefused` refusals (including `ErrDaemonStopping`) | Extends: the reload calls `restartPluginReplacement` (`reenable = false`) for each down plugin, and `restartPluginProcess` checks `stopUnderWay()` returning `ErrRestartRefused` + `ErrDaemonStopping` (which `restartReplacementWithin` already handles without writing a stop record). No second start path |
| Late-cleanup record | `restart.go` `lateCleanupRecord`, `awaitReplacedCleanup`; read in `restartPluginProcess`, `runPluginPhaseWithin`, `pluginDoctorState` | Refuses a start of a name whose stopped predecessor has not run its cleanup, and tells doctor | Applies unchanged: the reload restart passes through `restartPluginProcess`, so it is refused the same way (AC-12). Its shape is the model for the OPEN-policy record |
| The stop fact and post-fence drain seal | `server.go` `SignalShutdownRequested`, `stopUnderWay`; `drain.go` `BeginDrain`; `reload.go` `txLock.closeAdmission`; `process/manager.go` `RefuseRespawns` | `stopUnderWay` is true from the stop signal (before `awaitReloadWorker` and `fenceCommitReloads`); `BeginDrain` runs after the config fence and calls `RefuseRespawns`; `txLock.closeAdmission()` and `stopEntered` run only later in `Server.Stop` | Extends: `applyFailurePolicy` (skip retire) and `restartPluginProcess` (refuse respawns) read `stopUnderWay()` so no plugin crashes respawn or retire during the config fence, drain, or teardown (AC-2, AC-8). `BeginDrain` seals post-fence reload and startup-phase admission (`txLock.closeAdmission()` / post-fence drain guard checked by `reloadConfig` and `runPluginPhaseWithin`), so already-admitted commit reloads, queued SIGHUP reloads and confirmed-commit reverts still finish during the config fence (`signals.md`, "Stop sequence") while nothing starts once the drain begins (AC-15) |
| Per-peer OPEN policy set | `server.go` `PluginsWithPerPeerOpenPolicy`; `plugin/registration.go` `CapabilityInjector`; `bgp/server/validate.go` `unansweredOpenValidation` | Names the plugins that owe an answer for a peer, from live slots and injected per-peer capabilities; an unanswered one refuses the OPEN | Extends (D-A): the producer also reads the owed record, and the declarations of an owed plugin stay injected. The refusal itself is unchanged |
| Whole-declaration replacement | `plugin/registration.go` `ReplacePluginCapabilities` | Replaces a named owner's capability declaration in one publication, used by the config transaction | Extends: a replacement's Stage 3 declaration takes the place of the kept one through it, since `planLocked` refuses a code its own predecessor still holds (A-5) |
| Reload removal collection and acceptance-scoped unconfigure | `reload_dataplane_writer.go` `collectReloadStopProcesses`, `collectNonFIBRemovedProcesses`; `startup_autoload.go` `rootRemovalStopsOwner`, `stopCollectedProcessesMode`; `startup_removal.go` `recordUnconfigureOnAccept`, `finishRemovalScope`, `unconfigureProcessless` | Already collects both running and configured-down plugins whose roots left (plus displaced FIB writers and orphaned dependencies), schedules `processless` names on the reload acceptance scope, and calls `UnconfigureProcess` only on reload acceptance (`accepted == true`), preserving stop records if the reload is rejected | Extends: `reloadConfig` excludes `collectReloadStopProcesses` from its down-plugin restart set, and `finishRemovalScope` / `unconfigureProcessless` (plus non-provisional final stops) also clear the OPEN-policy owed record and kept OPEN-policy capability declarations only on reload acceptance (`accepted == true`), preserving them when a reload is rejected |
| Reload auto-load | `reload_dataplane_writer.go` `selectReloadAutoLoadWithReasons`, `autoLoadForReload`; `startup_autoload.go` `startAutoLoaded` | Starts owners of added, changed and writer-judged config paths; a failed start refuses the reload | Does not apply to a down plugin: a plugin that already ran is restarted by the restart path. Its refusal rule is the precedent AC-3 follows |
| Respawn limiter | `ProcessManager.Respawn`, `RespawnReenabled`, fields `disabled`, `respawnTimes`, `totalRespawns` | Bounds restarts and disables the name at a bound; `RespawnReenabled` (called only by `handlePluginRestart`) lifts the disable atomically after pre-spawn admission checks pass | Unchanged. Reload calls `restartPluginReplacement` (`Respawn`, `reenable = false`) and never lifts a disable |
| Stage | `Process.Stage`, `Process.Running` | Atomic handshake stage per process | Extends: one manager accessor returns only running processes, and runtime senders use it |
| Doctor snapshot | `doctor_snapshot.go` `pluginDoctorState`; `plugin/doctor/checks_runtime.go` `checkPluginStopped`, `pluginStoppedRow` | Reports each down plugin with cause, exit status and the fix | Extends: the row also says that peers wait on the plugin's OPEN check when the owed record holds it |
| Pre-write ended-context batch check | `pkg/plugin/rpc/conn.go` `writeBatchWithDeadline`; `pkg/plugin/rpc/mux.go` `CallBatchRPC` | Already checks `ctx.Err()` before writing a batch frame; `mux_batch_test.go` tests `MuxConn.CallBatchRPC` under a canceled context | Does not change production code: this spec adds the missing direct `Conn` regression tests in `pkg/plugin/rpc/conn_test.go` (AC-14) and closes the 2026-11-13 journal row once proven |
| Cleanup-hook list | `rpc_register.go` `RegisterProcessCleanup`, `runProcessCleanupHooks` | Other packages register a by-name cleanup run inside the cleanup body | Does not apply: nothing new is released from the cleanup body |

## Design documents

Derived from `./le spec design documents spec plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` (39 design pages, 29 decision rows).

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | Every lifecycle change (retire, reload restart, kept OPEN policy, stop fence, running-only accessors, reap-aware connect-back) works identically on internal and forked transports and is proven by tests in both modes; the tightened `pkg/ze.EventBus` `Subscribe` contract is documented on purpose with its page (`docs/architecture/plugin/component-boundaries.md`). |
| `docs/architecture/api/process-protocol.md` | Filter Insertion | the `// Design:` page of `failure_policy.go`, `restart.go`, `startup_removal.go`, `startup.go`, `reload.go`, `server.go`, `source_policy.go`, `engine_event.go`, `drain.go`, `manager.go`, `process.go`; a page this spec modifies | "A stop ends every restart. From the start of the stop drain (`BeginDrain`), the manager refuses each respawn with `respawn refused: ze is stopping`, so a plugin that exits during the drain or teardown stays down: the daemon is going away and a new generation would only be torn down again." | departs | AC-8 moves the start of respawn refusal from `BeginDrain` to the stop signal (`SignalShutdownRequested` / `stopUnderWay`), closing the config-fence window before the drain where a plugin exit could still trigger a restart, while AC-15 seals post-fence reload and startup-phase admission in `BeginDrain` so already-admitted commit reloads finish during the config fence; phase 7 updates this sentence on the page. |
| `docs/architecture/api/process-protocol.md` | Filter Insertion | the `// Design:` page of `failure_policy.go`, `reload.go`; a page this spec modifies | "One value answers two questions, because the engine starts a plugin again for exactly one reason. `restart` is also the plugin's statement that it MAY be started again, and the other two are its statement that it must not be." | departs | Under owner-approved D-B (2026-11-24, AC-3), a config reload restarts any configured down plugin (including `ignore`, unspecified, and `respawn false`) through `restartPluginReplacement`, while `ignore` still prevents automatic respawn on exit and `fatal` still stops the daemon; phase 4 updates the failure-policy table and prose to distinguish automatic respawn on exit from reload and operator restart. |
| `docs/architecture/api/process-protocol.md` | Panic containment | the `// Design:` page of `failure_policy.go`, `reload.go`; a page this spec modifies | "No boundary restarts a plugin: the engine releases it as it releases any plugin that exited, and a reload starts it again." | agrees | AC-1 retires an ended generation by releasing its name-keyed registrations and slot, and AC-3 (D-B) has a reload start a configured down plugin again, making this existing statement true. |
| `docs/architecture/config/transaction-protocol.md` | Phase 3: Rollback (conditional) | the `// Design:` page of `reload_tx.go`; anchored to `failure_policy.go`, `startup_removal.go`, `startup.go`, `reload.go`, `eventbus.go` | "the restart was refused before it could stop the old process (the respawn limit, a disabled plugin, a stopping daemon, no such plugin), without claiming whether that process still runs; the restart failed after the old process stopped, so it is not running" | agrees | AC-1 keeps a live generation's registrations when a `CodeBroken` restart is refused before stopping it, retires only an ended generation whose post-exit restart was refused before `Respawn` stopped anything, and AC-15 honors the shutdown-refusal contract in `Shutdown while a transaction runs`. |
| `docs/architecture/doctor-and-health-checks.md` | Plugin lifecycle checks | the `// Design:` page of `doctor_snapshot.go`, `checks_runtime.go`, `stopped.go`; anchored to plugin server and process files; a page this spec modifies | "Every path that leaves a configured plugin not running records it with the process manager (`ProcessManager.RecordStopped`): its failure policy (`ignore`, or none declared) after an exit, the respawn limit, a failed restart, and a startup handshake that did not reach running." | agrees | AC-4, AC-5, and AC-12 preserve every down path, `RecordStopped`, `doctor-plugin-stopped`, and `ze request plugin restart <name>` as the fix; reload removals of down plugins already unconfigure them only on reload acceptance inside `finishRemovalScope` / `unconfigureProcessless` (preserving them on rejection), and AC-11 extends that acceptance-scoped unconfigure to clear the owed OPEN-policy record and kept declarations as well while adding one sentence to the `plugin-stopped` row when peers wait on a down plugin's OPEN check, which phase 2 documents on this page. |
| `docs/architecture/exabgp-bridge.md` | Script exit, respawn, and route ownership | anchored to `internal/component/plugin/server/failure_policy.go` | "The wrapper declares `failure-policy: restart` at Stage 1 so an endpoint without `respawn false;` is restarted by the process manager and re-declares its virtual session capabilities when it reconnects." | agrees | `failure-policy: restart` keeps respawning on exit until daemon stop begins (AC-8), while `respawn false;` takes the `ignore` outcome, retires cleanly (AC-1), and restarts on reload (AC-3). |
| `docs/architecture/firewall/firewall-irr.md` | A crash starts the plugin again | anchored to `internal/component/plugin/server/failure_policy.go` | "It is bounded at 5 restarts in 60 seconds and 20 over the life of the daemon, which is what stops an answer that panics every time from cycling: past the bound the plugin is disabled and `show health` reports doctor-plugin-stopped." | agrees | Respawn bounds, permanent disable, and `doctor-plugin-stopped` reporting stay unchanged (AC-1, AC-4, AC-5), and AC-3 never lifts a respawn-limit disable on reload. |
| `docs/architecture/hub-architecture.md` | SSH session model factory | anchored to `failure_policy.go`, `startup.go`, `server.go`, `manager.go`, `process.go`; a page this spec modifies | "From the start of the drain the stop is terminal: a plugin that exits is not started again." | departs | AC-8 makes plugin-exit respawn refusal start at the stop signal (`SignalShutdownRequested`) rather than at the start of the drain, closing the config-fence window against crash-triggered respawns while already-admitted commit reloads finish before `BeginDrain` seals reload and startup-phase admission (AC-15); phase 7 updates this sentence on the page. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering `internal/component/plugin/*`, `pkg/plugin/rpc/conn_test.go`, `pkg/ze/eventbus.go` | "The engine never enumerates plugins." | agrees | The OPEN-policy owed record (AC-11, D-A) is derived solely from Stage 1/3 declarations (`WantsValidateOpen` and per-peer capabilities) with no plugin name in engine code; startup tier failure remains fatal to the tier; no second start path or engine-op path is added. |
| `docs/architecture/behavior/peer-lifecycle.md` | TL;DR | anchored to `reload_tx.go`, `source_policy.go` | - | silent | Documents BGP peer FSM run loops and source-policy reload coordination; says nothing about plugin process retirement, down-plugin reload restart, stop fencing, or stage gating of `CaptureSourcePolicyOwners` (AC-7 skips only handshaking replacements before Stage 5). |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `reload_tx.go`, `reload.go` | - | silent | Documents dependency-ordered operation planning during config apply; says nothing about restarting down plugins before a reload diff or refusing reloads once `BeginDrain` seals post-fence admission. |
| `docs/architecture/core-design.md` | 18. Config Transaction Protocol | the `// Design:` page of `validate.go`, `eventbus.go`; anchored to `reload_tx.go` | "Config changes (SIGHUP, CLI commit, API) use a bus-based transaction protocol with verify, apply, and rollback phases. The engine orchestrates; plugins participate via bus events." | agrees | Reloads keep the bus-based verify/apply/rollback transaction; restarting configured down plugins before computing the affected set (AC-3, D-B) ensures the owner of modified roots participates rather than failing verify on a dead connection, and AC-6 tightens engine-event unsubscribe without changing bus event semantics. |
| `docs/architecture/config/syntax.md` | Overview | anchored to `internal/component/plugin/server/startup_removal.go` | - | silent | Documents Ze configuration syntax; this spec adds, removes, or changes no configuration syntax or leaf. |
| `docs/architecture/api/commands.md` | Overview | the `// Design:` page of `startup.go`; anchored to `server.go`, `manager.go` | - | silent | Documents CLI/API command paths, selectors, and pipe syntax; this spec adds or changes no command syntax (retiring a down plugin unregisters its commands and pipe aliases through the existing release path). |
| `docs/architecture/api/ipc_protocol.md` | RPC Response (Either Direction) | anchored to `startup.go`, `process.go` | "Batched event delivery (`ze-plugin-callback:deliver-batch`) on external TLS connections runs through `MuxConn.CallBatchRPC`, which formats frames into a pooled buffer via `rpc.WriteBatchFrame` without intermediate `map[string]any` marshaling." | agrees | `writeBatchWithDeadline` and `MuxConn.CallBatchRPC` already check `ctx.Err()` before writing without changing batch wire framing or splitting; AC-14 adds direct `Conn` regression tests in `pkg/plugin/rpc/conn_test.go`. |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `internal/component/plugin/server/startup.go` | - | silent | Documents CLI and web completion producers; says nothing about plugin process retirement, post-fence drain gating, or `runPluginPhaseWithin`. |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 4: an attack-response table is cleared at the next start | anchored to `internal/component/plugin/server/startup.go` | "`runPluginPhase` starts all engines before their handshakes, then completes each dependency tier before the next." | agrees | AC-15 only refuses entering a new start phase once `BeginDrain` seals post-fence admission after the config fence; tiered startup ordering and `rootRemovalStopsOwner` behavior for plugins with remaining config roots stay unchanged. |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `startup.go`, `process.go` | - | silent | Documents the IKEv2 engine and its kernel/dataplane interactions; says nothing about generic plugin process retirement, down-plugin reload restarts, or connect-back reap watching. |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Decisions | anchored to `internal/component/plugin/server/startup.go` | - | silent | Documents the PPPoE client plugin and its kernel lifecycle; says nothing about generic plugin server startup phase gating or retirement. |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `startup.go`, `reload.go`, `source_policy.go`, `manager.go`, `process.go` | "The server captures the selected declared owners as immutable process/connection handles." | agrees | AC-7 restricts `CaptureSourcePolicyOwners` to running processes so a handshaking replacement is never captured before Stage 5, while keeping immutable handle capture and all registration/dependency rules intact. |
| `docs/architecture/behavior/signals.md` | Stop sequence | anchored to `internal/component/plugin/server/reload.go` | "A stop signal, and every daemon stop command, reaches `run` in `cmd/ze/hub/main.go` through `waitLoop`, and the hub runs three phases: the config fence (a running commit reload finishes), the stop drain (`drain` RPC to every plugin that declared a drain budget, deadline at most 60 s from the start of the stop), and teardown, where BGP sends Cease 6/2 to each live session." | agrees | AC-8 refuses crash-triggered respawns from the stop signal (`SignalShutdownRequested` / `stopUnderWay`), while AC-15 seals reload and startup-phase admission in `BeginDrain` after `awaitReloadWorker` and `fenceCommitReloads` complete, so already-admitted commit reloads, queued SIGHUP reloads, and confirmed-commit reverts still finish during the config fence exactly as this sequence requires. |
| `docs/architecture/api/architecture.md` | Message Bus and Participant Wiring | anchored to `server.go`, `process.go`, `validate.go` | - | silent | Documents message bus participant wiring and cache control; says nothing about retiring down plugin processes, keeping per-peer OPEN-policy declarations while down, or gating runtime senders on `StageRunning`. |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `internal/component/plugin/server/server.go` | - | silent | Documents YANG schema architecture, loading, and validation; this spec changes no YANG schema or config validation rules. |
| `docs/architecture/overview.md` | 2. System Architecture | anchored to `internal/component/plugin/server/server.go` | - | silent | High-level system overview; says nothing about internal plugin process retirement, stop fencing, or stage-gated senders. |
| `docs/architecture/testing/runner-architecture.md` | Layers | anchored to `internal/component/plugin/server/server.go` | - | silent | Documents the functional test runner layers and suite wrappers; this spec does not modify the test runner. |
| `docs/architecture/update-cache.md` | Cache Consumer Model | anchored to `server.go`, `process.go` | - | silent | Documents the BGP UPDATE cache consumer model and FIFO acknowledgements; this spec does not modify cache consumer tracking or cleanup. |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `internal/component/plugin/server/source_policy.go` | - | silent | Documents RIB storage pools, selection, and source-policy handlers inside `bgp-rib`/`bgp-rs`; says nothing about stage gating of `CaptureSourcePolicyOwners` on the plugin server. |
| `docs/architecture/resolve.md` | Clearing a resolver cache | the `// Design:` page of `resolver_cache.go`; anchored to `resolver_cache.go` | "the handler asks each plugin that declared the cache through the `ze-plugin-callback:resolver-cache-clear` callback (`pluginserver.Server.ResolverCacheOwners`, `CallResolverCacheClear`)." | agrees | AC-7 restricts `ResolverCacheOwners` and `CallResolverCacheClear` to running processes so a handshaking replacement is not queried before Stage 5, keeping the callback protocol and clear semantics unchanged. |
| `docs/architecture/iface/management.md` | Two layers, not one flat list | anchored to `internal/component/plugin/server/engine_event.go` | - | silent | Documents interface netlink management and the link-event queue; it only notes that `EmitEngineEvent` dispatches synchronously on the caller's goroutine, which AC-6 preserves. |
| `docs/architecture/plugin/component-boundaries.md` | The Bus was deleted, not kept beside the stream system | anchored to `engine_event.go`; a page this spec modifies | "The Bus was absorbed into the stream system behind a public `ze.EventBus` interface backed by `Server.Emit` and `Server.Subscribe`." | agrees | Keeps the `ze.EventBus` interface and `Subscribe` signature unchanged; AC-6 tightens post-`unsubscribe` delivery so a retired handler in an in-progress snapshot is skipped before invocation, and phase 6 documents that guarantee on this page. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `internal/component/plugin/process/manager.go` | - | silent | Records the decision to keep the push/registry metrics model for plugins; this spec changes no metric names, labels, or collection model. |
| `docs/architecture/api/capability-contract.md` | Design Decisions | anchored to `process.go`; a page this spec modifies | "3. **Respawn**: Re-confirm capability on every spawn" | agrees | Every restarted generation still re-declares its capabilities at Stage 3 (`ReplacePluginCapabilities`, A-5); under approved D-A (AC-11), a down OPEN-policy validator's per-peer OPEN-policy capabilities stay injected until a running replacement confirms its own declarations or an accepted reload removes the plugin, and phase 2 documents that on this page. |
| `docs/architecture/config/environment.md` | Overview | anchored to `internal/component/plugin/process/process.go` | - | silent | Documents environment variable registration and shell-safe inheritance; this spec adds or changes no environment variable or child env passing. |
| `docs/architecture/plugin/show-enrichers.md` | Constraints | anchored to `internal/component/plugin/process/process.go` | - | silent | Documents show-enricher proxy lifecycle and connection-scoped failure recording; says nothing about retiring `ignore` exits, stop fencing, or connect-back reap watching. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `internal/component/plugin/process/process.go` | - | silent | Documents `.ci` functional test file syntax and stderr relay rules; this spec does not alter `.ci` syntax or stderr relaying. |
| `docs/architecture/edge-cases/as4.md` | Overview | anchored to `internal/component/bgp/server/validate.go` | - | silent | Documents 4-byte ASN capability negotiation and `AS_PATH` reconstruction; says nothing about OPEN-validator stage gating or down-validator refusal. |
| `docs/architecture/meta/role.md` | Absence | anchored to `internal/component/bgp/server/validate.go` | "RFC 9234 Section 4.2 requires the connection to be rejected when the Roles do not correspond. An unreachable validator leaves ze unable to say that they do, so it rejects rather than assume (owner decision, 2026-08-30)." | agrees | AC-11 (D-A, approved 2026-11-24) extends this exact fail-closed refusal (NOTIFICATION 2/11) to cover retired, failed-restart, disabled, and handshaking policy plugins. |
| `ai/doors/bgp-engine.md` | Boundaries | the door covering `internal/component/bgp/server/validate.go` | "Never name a plugin package from the reactor." | agrees | `askOpenValidators` uses the generic running-only process accessor and `PluginsWithPerPeerOpenPolicy` reads the server's declaration-derived owed record, keeping BGP engine code free of plugin names. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering `pkg/ze/eventbus.go` | "**One reload path.** SIGHUP and a managed push both reach `doReload` (`cmd/ze/hub/main_reload.go`)." | agrees | Down-plugin reload restart (AC-3) and post-fence drain refusal (AC-15) sit in `Server.reloadConfig` on the single reload path; AC-8 reads the hub's existing `SignalShutdownRequested` call and AC-15 seals admission in `BeginDrain` after `fenceCommitReloads`, without adding a second stop or reload path. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/plugin/*.ci` and `test/reload/*.ci` | "**A bare `go test` omits the feature tags**, so plugins do not register and unrelated tests fail." | agrees | All package test commands in the TDD plan use `./le test unit pkg path ...`, and all nine new `.ci` functional tests follow the `/ze-test` draft-and-promote workflow under `test/draft/`. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 on `process-protocol.md` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no engine operation and does not alter plugin identity authorization or filter profiles. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 on `process-protocol.md` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec does not change queue overflow or drop behavior; `writeBatchWithDeadline` and `CallBatchRPC` already return `ctx.Err()` before writing a batch whose delivery context has already ended, and AC-14 pins that behavior with direct `Conn` regression tests. |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 on `exabgp-bridge.md` | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | This spec treats ExaBGP endpoints identically to every other bus endpoint and touches no bridge config section or mode. |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-26 on `exabgp-bridge.md` | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | This spec does not alter `FormatBase64`, `WriteBatchFrame` wire grammar, or the ExaBGP wrapper's rendering path. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Orthogonal to plugin process lifecycle; this spec does not alter VPP/kernel commit backend checks or host-side exemptions. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Orthogonal to plugin process lifecycle; this spec does not alter data-plane mismatch validation verdicts. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Orthogonal to plugin process lifecycle; this spec touches neither the VPP firewall verifier nor YANG backend annotations. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Orthogonal to plugin process lifecycle; this spec touches no data-plane selector or backend naming. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Orthogonal to plugin process lifecycle; this spec touches no BGP egress next-hop rewriting or export filter chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Orthogonal to plugin process lifecycle; this spec touches no `policychain` or export-chain authoring forms. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Orthogonal to plugin process lifecycle; this spec touches no built-in policy chain steps or `FilterInsertion` semantics. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Orthogonal to plugin process lifecycle; this spec touches no export-chain folding or `wireOverride` provenance checks. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Orthogonal to plugin process lifecycle; this spec touches no next-hop IPv4/IPv6 validation or import-chain rules. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Orthogonal to plugin process lifecycle; this spec touches no forwarded vs originated next-hop handling. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Orthogonal to plugin process lifecycle; this spec does not touch the implied-filter seam or `\| implied` output. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `core-design.md` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec does not add any service-config start refusal to `ze start`, and AC-3 (D-B) follows the reload rule directly: a reload whose configured down plugin fails to start stays refused as a whole while running services keep serving and `ze doctor` reports the down plugin with its fix. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `core-design.md` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Orthogonal to plugin process lifecycle; this spec touches neither VPP/kernel data-plane mismatch detection nor start-time feature exclusion. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 on `commands.md` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec adds or changes no command path or pipe operator. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 on `commands.md` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec adds or changes no pipe override or answer format; retiring a down plugin releases its declared pipe aliases cleanly. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 on `commands.md` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec adds or changes no pipe operation semantics. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 on `plugin-system.md` | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | This spec touches only live daemon process supervision, reload, and shutdown; offline query-mode declaration parsing is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `architecture.md` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Orthogonal to plugin process lifecycle; this spec does not touch `\| implied` config formatting. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `architecture.md` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | This spec preserves bus participant naming and default message registrations. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `architecture.md` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Orthogonal to plugin process lifecycle; this spec does not alter `send > to` filter compilation. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `architecture.md` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Orthogonal to plugin process lifecycle; this spec does not alter `receive > from` filter compilation. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `architecture.md` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Orthogonal to plugin process lifecycle; this spec does not alter send/receive allow-lists or bus enforcement. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `architecture.md` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Orthogonal to plugin process lifecycle; this spec does not alter participant bus filter compilation or conflict checks. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `architecture.md` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Orthogonal to plugin process lifecycle; this spec adds no bus arbitration between participants. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `architecture.md` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Orthogonal to plugin process lifecycle; this spec does not touch effective bus-filter display. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A plugin process ends: its connection closes or its in-process engine returns. No data format; the trigger is the return of `handleSingleProcessCommandsRPC`.
- `ze signal reload`, SIGHUP, a managed push or a commit reaches `reloadConfig` with the candidate tree.
- SIGTERM, SIGINT or `request shutdown` reaches the hub's stop sequence, whose first act is `SignalShutdownRequested`.
- A peer OPEN reaches `broadcastStampedValidateOpen` with the policy plugins of that peer.
- An engine event reaches the engine subscribers' `dispatch`.
- A forked plugin's program exits before it dials back.

### Transformation Path
1. Generation end: `superviseProcess` stops the drain caller; the cleanup body has run. `applyFailurePolicy` raises the crash report and reads the policy.
2. Once `stopUnderWay()` is true (`SignalShutdownRequested`), `applyFailurePolicy` goes no further than the crash report and the policy's log line: it retires nothing, and `restartPluginProcess` refuses any respawn with `transaction.ErrRestartRefused` wrapping `process.ErrDaemonStopping` (which `restartReplacementWithin` already handles without writing a stop record).
3. Otherwise, `restart` calls the restart. For every outcome after which the manager still holds this dead process under the name (`ignore`, unspecified, unknown — replacing their direct `RemovePluginCapabilities` calls — and a restart refused before `Respawn` stopped anything), the supervisor retires it: the retire steps of the Existing Mechanisms table, then `RecordStopped` with the cause.
4. Inside the name-keyed release, on a path that leaves the plugin configured: when the registration wants validate-open, its capability declarations stay in the injector and its name enters the owed record. On a non-provisional removal (such as stopping a rolled-back candidate auto-load) or on reload acceptance (`finishRemovalScope`), everything is released and the owed record and kept declarations are cleared.
5. A generation that reaches running clears the stop record, as today, and the owed record; its Stage 3 declaration replaced the kept one.
6. Reload: once `BeginDrain` has sealed post-fence admission (after the hub's config fence `awaitReloadWorker` and `fenceCommitReloads`), `reloadConfig` and `runPluginPhaseWithin` refuse with `transaction.ErrShutdown`; during the config fence before `BeginDrain`, already-admitted commit reloads, queued SIGHUP reloads, and confirmed-commit reverts are not aborted by `stopUnderWay()`. Otherwise, after the diff and `collectReloadStopProcesses` are computed and before the no-change return, `reloadConfig` restarts each name that holds a stop record and is not in the removal collection, in dependency order, through `restartPluginReplacement` (`reenable = false`). A failure refuses the reload before anything of the candidate is applied. The affected set is built afterwards, so a restarted plugin verifies and applies the change.
7. When the reload acceptance scope is accepted (`finishRemovalScope` with `accepted == true`), finalized removals and `unconfigureProcessless` drop the configured entry and stop record (as today) and clear the plugin's OPEN-policy owed record and kept OPEN-policy capability declarations; when the reload is rejected, `pendingUnconfigure` is discarded and both the stop record and the owed OPEN-policy record remain intact.
8. OPEN validation: the producer returns running validators and owed names for the peer's selectors. `askOpenValidators` asks running processes only. A name nobody answered refuses the OPEN.
9. The other runtime senders take their process from the running-only accessor.
10. Event dispatch checks each subscription's retired mark immediately before calling it.
11. `startExternal` ends its wait when the child is reaped and fails the start with the exit status.
12. `writeBatchWithDeadline` (`pkg/plugin/rpc/conn.go`) and `MuxConn.CallBatchRPC` (`pkg/plugin/rpc/mux.go`) already return `ctx.Err()` before writing; this spec adds the missing direct `Conn` regression tests in `pkg/plugin/rpc/conn_test.go`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | No message added or changed. A plugin in its handshake stops receiving `validate-open`, `filter-update`, `doctor-check`, `resolver-cache-clear` and `source-policy`, which the handshake could not answer. Ended-context batch writes already return `ctx.Err()` without writing | Read `askOpenValidators`, `CallFilterUpdate`, `CallDoctorCheck`, `CallResolverCacheClear`, `CaptureSourcePolicyOwners`, `writeBatchWithDeadline`, `CallBatchRPC` |
| Engine ↔ BGP peer | The Role Mismatch NOTIFICATION for an unanswered policy, which exists today, is now also sent while the policy plugin is down or restarting. No new message | Read `OpenValidationUnavailableError.NotifyCodes` |
| Hub ↔ plugin server | No new call. `SignalShutdownRequested` starts AC-2/AC-8 respawn/retire suppression, and `BeginDrain` (after `awaitReloadWorker` and `fenceCommitReloads`) seals post-fence reload and startup-phase admission (AC-15) | Read `cmd/ze/hub/main.go`, `drain.go` |

### Changed-Wire Census

No wire changes. Source-supported reason: the edits are in-memory ordering and
storage inside the engine. No field of the public RPC types, no method in the
plugin engine or callback YANG modules, no JSON event and no config leaf is
added, removed or renamed. The NOTIFICATION sent for an unanswered OPEN policy
is the one `OpenValidationUnavailableError` already produces. The doctor row
of a down plugin gains a sentence in its message text, which no reader parses.
The public `ze.EventBus` `Subscribe` keeps its signature; its contract is
tightened (AC-6) and documented.

**Search evidence:** scope `internal/component/plugin`, `internal/component/bgp/server`, `pkg/ze`, `cmd/ze/hub`; every symbol named in "Files to Modify" was read at its producer on 2026-11-26 and none encodes or decodes a format. Positive control: `validate.go` `openMessageToRPC` is the encoder of the `validate-open` input, and this spec does not edit it.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| none | writer | none: no encoder is edited | the read above | preserved |
| none | reader | none: no decoder is edited | the read above | preserved |

### Integration Points
- `Server.applyFailurePolicy` - reads `stopUnderWay`; replaces `ignore`/unspecified/unknown's direct `RemovePluginCapabilities` with the shared retire path (for any generation that ended with no successor).
- `Server.releasePluginRegistrations` - gains the kept-declaration branch and the owed record write.
- `Server.finishRemovalScope`, `Server.unconfigureProcessless` (and non-provisional final stops in `startup_removal.go`) - clear the owed record and kept OPEN-policy capability declarations on reload acceptance (preserving them on reload rejection).
- `Server.PluginsWithPerPeerOpenPolicy` - also reads the owed record.
- `Server.BeginDrain` - seals post-fence reload and startup-phase admission (`txLock.closeAdmission()` / post-fence drain guard) after the config fence completes.
- `Server.reloadConfig` - restarts configured down plugins not in `collectReloadStopProcesses`; refuses once `BeginDrain` sealed post-fence admission.
- `Server.restartPluginProcess` - reads `stopUnderWay` and returns `transaction.ErrRestartRefused` wrapping `process.ErrDaemonStopping` (skipping `RecordStopped` via existing `restartReplacementWithin`).
- `Server.runPluginPhaseWithin` - refuses once `BeginDrain` sealed post-fence admission (beside `stopEntered`).
- `ProcessManager` - gains the running-only accessor and the accessor listing recorded stops.
- `Process.startExternal` - the wait also ends on the reaped channel.
- The engine event subscribers - the handler entry gains a retired mark.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The retire, the reload restart and the stop refusal each reuse the function that already does the job for another caller |
| No unintended coupling (components stay isolated) | Yes | No new import. The owed record lives in the plugin server beside the producer that reads it |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms table: no new owner object, counter, gate or start path |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No encoding path is touched |
| Registration over hardcoding, outbound | Yes | No command, family or handler is added |
| Registration over hardcoding, inbound | Yes | No list learns a plugin name. The owed record is derived from two declarations (`WantsValidateOpen`, a per-peer capability); the reload set is the stop record; the removal set is `registry.ConfigRootsMap` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The retire always runs after the generation's cleanup completed | `applyFailurePolicy` is called only by `superviseProcess`, after `handleSingleProcessCommandsRPC`, which defers the cleanup, returned (read 2026-11-19) | The release would wait on a cleanup that cannot run, or strip a successor | `TestIgnoredExitRetiresGeneration` with a cleanup hook that records its order | unvalidated |
| A-2 | Every change of the running config passes `reloadConfig` | The hub door's "One reload path" for SIGHUP and the managed push; commit reloads close in the hub's stop sequence. Not read: `create bgp peer` | A config change could commit while a plugin that owns its roots is down, unverified | Phase 4 reads every caller of the transaction coordinator; a path outside `reloadConfig` gets the same restart-or-refuse step and a test | unvalidated |
| A-3 | No caller unsubscribes an engine event while holding a lock its handler takes, in a way that needs the handler to have finished | Not read: there are about 150 `Subscribe` call sites | AC-6's permitted survivor (an invocation already running) reaches state the caller just replaced | Phase 6 census of every unsubscribe call site whose handler reads state the caller replaces; each gets the ddos-local shape (a retired mark under the subscriber's own lock) or is shown safe | unvalidated |
| A-4 | The 2026-09-17 early exit of `firewall-domain` and `firewall-irr` no longer happens, because those plugins run in the daemon and 32fa1ed795 (2026-09-24, `sdk.HostOwnsSignals`) stopped a host signal from reaching in-process plugins | The commit postdates the row; the mode the guest ran was not established | The row's cause stays unidentified; AC-8 still closes the window | AC-10: phase 7 establishes the mode the guest test runs the two plugins in, then reproduces | unvalidated |
| A-5 | A replacement's Stage 3 declaration can take the place of the kept declaration of its own name | `planLocked` refuses a code already held for a peer with no exemption for the holder, so `AddPluginCapabilities` would refuse it; `ReplacePluginCapabilities` replaces a named owner's whole declaration (both read 2026-11-19). Not read: which of the two Stage 3 calls | The replacement fails Stage 3 against its own predecessor | Phase 2 reads the Stage 3 call site and routes it through the replacement; `TestReplacementReplacesKeptOpenPolicyDeclarations` | unvalidated |
| A-6 | A plugin restarted after a retire is marked loaded again | `releaseStoppedProcess` clears the mark and `restartPluginProcess` was not seen to set it; the mark's readers are the auto-load selection and a dependency check in `startup.go` | A restarted plugin reads as not loaded to a dependent's check. The same holds today after a failed restart followed by `request plugin restart` | Phase 3 reads the three readers; if the mark stays clear, the restart sets it when the replacement reaches running; `TestRestartOfRetiredPluginMarksItLoaded` | unvalidated |
| A-7 | The process-protocol page says, in "Panic containment", that "a reload starts it again" | The 2026-11-11 reading and the 2026-11-19 review. This pass read the page from "Failure-Policy Declaration" to "Who stops the plugin server" and did not meet the sentence | The page edit of phase 4 has one sentence less to correct | Phase 4 searches the page; AC-3 makes the sentence true wherever it is | unvalidated |
| A-8 | The failure-policy test fixture can declare a per-peer capability and answer `validate-open`, internal and forked | The fixture behind `test/plugin/failure-policy-restart.ci` (found by its log line `FIXTURE: failure-restart generation 1 is exiting`); not read | AC-11's `.ci` needs a fixture of its own | Phase 1 reads the fixture | unvalidated |
| A-9 | The crash report of a retired generation survives the retire | `RemoveProcess` and `cleanupProcess` clear the plugin's report source; the process-protocol page says "Historical error events remain in the report ring" | The retire would erase the crash an operator needs | `TestIgnoredExitRetiresGeneration` asserts the event is still listed | unvalidated |

A-5 and A-6 of the first design moved with the firewall vpp item, and its A-7 and A-8 are answered by the code read above (the doctor row carries its fix; `Respawn` restarts explicit and internal plugins from the manager's configs). The ids above are this design's.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The retire runs during the stop and unregisters families or capabilities while sessions still close | A changed Cease or a late encode error in `test/plugin/shutdown-is-prompt.ci` | The retire is skipped once `stopUnderWay` is true (AC-2); `TestExitAfterStopBeganRetiresNothing` pins the fence window, where the server context is still live |
| R-2 | A peer whose role is configured establishes with no role check while its plugin is down or restarting | A session Established in `test/plugin/role-open-refused-validator-down.ci` | AC-11 lands before AC-1. The producer keeps reading non-running state and is never moved to the running-only accessor |
| R-3 | A down plugin that cannot start blocks every reload | `doctor-config-reload-refused` on each reload | Intended, and the same answer boot gives (the tier fails). The refusal names the plugin and both ways out: `ze request plugin restart <name>` once the cause is fixed, or a config that no longer needs the plugin |
| R-4 | A dependent of a retired plugin keeps running against a provider that is gone | A dependent's errors after an `ignore` exit | Unchanged from today (the provider was already dead). See Known Limitations |
| R-5 | The reload restarts a plugin the candidate removes | A removed plugin starts during the reload that removes it | The restart set excludes the removal collection, computed from the candidate tree with the predicate the removal itself uses |
| R-6 | A `restart` plugin that crashes during a long config fence stays down for the rest of the stop | The drain round reports it absent | Intended: the stop is terminal, and the drain already tolerates an absent caller |
| R-7 | A stopped predecessor whose cleanup is late lets a reload start a successor that the cleanup then strips | A restarted plugin loses its routes seconds after a reload | AC-12: the reload restart is refused by the late-cleanup record, as every restart is |
| R-8 | The stop-fence change collides with the drain spec's tests | `test/plugin/drain-respawn-refused.ci` red | Its expected log line is `ErrDaemonStopping`'s text, which the earlier refusal also produces; both drain tests are in the re-run list |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A running plugin loses its registrations (a retire of the wrong process), a peer is refused that should establish or established that should be refused, a reload is refused that should apply, or a stop hangs. All daemon-visible; no wire format is at risk |
| How is it reverted? | Each phase is one commit and reverts alone. No config or stored state changes |
| Who else touches this path? | `plan/release/fixes/spec-daemon-drain-before-stop.md` (ready) lists the failure-policy, manager, process, server and hub files. Its `BeginDrain` and `RefuseRespawns` are in the tree. The one real conflict is the fence window: this spec makes the stop fact, not the drain, the start of "no restart, no release, no retire", and the drain spec's remaining phases are written against that order. Its comment in `applyFailurePolicy` ("genuine exits during it retain their crash report and existing policy handling") stays true for the crash report and the policy's log line. `plan/release/fixes/spec-single-owner-reload-reconcile.md` (design) works on reload reconciliation; D-B keeps the restart of a down plugin here, because it is what makes the retire safe, and that spec designs around a reload that already restarts such a plugin. `plan/release/fixes/spec-single-owner-lifecycle-3-bgp-plugins.md` reuses the supervisor "as is" and leaves any change of the path to this child. No spec blocks another, so Depends stays `-` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A plugin with failure policy `ignore` exits, then `ze signal reload` | → | the retire in `applyFailurePolicy`, the restart of down plugins in `reloadConfig` | `test/plugin/failure-policy-ignore-reload.ci` |
| The same, plugin forked | → | same | `test/plugin/failure-policy-ignore-reload-external.ci` |
| A peer whose OPEN policy plugin has exited sends its OPEN | → | the owed record read by `PluginsWithPerPeerOpenPolicy` | `test/plugin/role-open-refused-validator-down.ci` |
| The same, plugin forked | → | same | `test/plugin/role-open-refused-validator-down-external.ci` |
| SIGTERM, then a `restart` plugin exits before the drain | → | `stopUnderWay` read in `restartPluginProcess` and `applyFailurePolicy` | `test/plugin/stop-fence-respawn-refused.ci` |
| A forked plugin whose program exits at once | → | the reaped channel in `startExternal` | `test/plugin/plugin-exits-before-connect-back.ci` |
| A peer OPEN while a validator is in its handshake | → | running-only accessor in `askOpenValidators` | `TestAskOpenValidatorsSkipsHandshakingProcess` (seam: a `.ci` cannot hold a process inside its handshake) |
| An engine event after `unsubscribe` | → | retired mark in `dispatch` | `TestEngineEventUnsubscribedHandlerSkippedInSnapshot` (engine-internal bus, no operator entry point) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A running generation ends, the stop is not under way, and the name is left with no successor: policy `ignore`, unspecified or unknown, or policy `restart` whose restart was refused before `Respawn` stopped anything | Once `applyFailurePolicy` returns, the direct `RemovePluginCapabilities` arms are replaced by the shared retire path: the registry row, families, pipe aliases and answer shapes held under the name are released exactly once, and so are its injected capabilities unless AC-11 keeps them; a command the plugin declared answers as an unknown command | A `CodeBroken` restart that is refused keeps every registration, because that generation has not ended. A restart that failed after `Respawn` already left no slot, by `rollbackReplacement` |
| AC-2 | A generation ends after the stop signal, in the fence, the drain or the teardown | Its crash report is raised and its policy's log line is written, as today, and the daemon exits 0 | Every name-keyed registration, the slot and the loaded mark stay until the process exits, because a release during the stop could change what closing sessions send. The guard reads `stopUnderWay`, since the server context is still live in the fence and the drain |
| AC-3 | A reload, with or without a config difference, while a configured plugin holds a stop record and the reload does not remove it | The reload restarts the plugin through `restartPluginReplacement` (`reenable = false`): it completes the five stages, receives the running config, answers its commands, takes part in the reload's verify and apply, and its stop record clears; the reload reports success | A plugin the reload removes stays down and, once the reload scope is accepted (`finishRemovalScope` / `unconfigureProcessless`), its configured entry, stop record and owed record clear (a rejected reload preserves all three). A plugin that does not start refuses the reload with nothing applied, keeps a stop record naming the new cause, and the refusal names it with `ze request plugin restart <name>`. A respawn-limit disable is not lifted, because only the operator's `ze request plugin restart <name>` (`RespawnReenabled`) lifts it. A plugin with policy `fatal` is never in this state |
| AC-4 | The same generation end as AC-1 | The manager slot and the loaded mark of the name are free, and the stop record stands with its cause and exit status because it is written after the slot removal; `ze request plugin restart <name>` then starts the plugin, with no process to stop first | The slot of a replacement another path already stored stays, by pointer comparison. The restart counters stay, because the bound is over the daemon's life. The name stays configured |
| AC-5 | `ze doctor` while a plugin is down as in AC-4, and again after AC-3's reload or the restart command | The first run reports the plugin by name with its cause, its exit status and the fix `ze request plugin restart <name>`, and the `plugins` health row reads down; the second run reports the row healthy | The `doctor-plugin-crashed` event stays listed until it ages out, because it records that an exit happened |
| AC-6 | `unsubscribe` returned for an engine-event subscription, then an event is emitted, or a dispatch that already took its list reaches that subscription | The other subscribers receive the event, and the retired subscription is skipped at a check made immediately before each call | An invocation of that handler already running when `unsubscribe` returned finishes, because waiting for it deadlocks a handler that unsubscribes itself. A call whose check passed just before `unsubscribe` returned also runs, because the check and the call are not one atomic step; a subscriber that replaces state its handler reads guards it itself (A-3) |
| AC-7 | A runtime sender asks for a process while a replacement is between `Respawn` and the end of its handshake: the all-slot walks (`askOpenValidators`, `DoctorCheckPlugins`, `ResolverCacheOwners`) and the by-name senders (`CallFilterUpdate`, `CallDoctorCheck`, `CallResolverCacheClear`, `CaptureSourcePolicyOwners`) | The sender reaches only a process at the running stage, a by-name sender answers that the plugin is unavailable, the replacement completes its handshake undisturbed, and a `-race` test over the concurrent read and write of the registration is clean | An OPEN that needs a validator still in its handshake is refused as unanswered and the peer retries. `PluginsWithPerPeerOpenPolicy` keeps reading non-running state, because it must name a validator that is not running. `addOpenRemovalObservers` and `policyReplayOwner` are unchanged, because the first reads subscribers, which exist only from Stage 5, and the second sends nothing |
| AC-8 | The stop signal arrived (`SignalShutdownRequested`), then a plugin with policy `restart` exits, a rollback asks for a restart, or an operator types `request plugin restart` | Each is refused at `restartPluginProcess` (reading `stopUnderWay`) with `transaction.ErrRestartRefused` wrapping `respawn refused: ze is stopping`; `restartReplacementWithin` already skips `RecordStopped` on `ErrRestartRefused`; the stop completes with exit 0 | A restart whose `Respawn` call had already returned before the signal completes its handshake, because the stop sequence then closes it like any running plugin |
| AC-10 | The 2026-09-17 early exit of `firewall-domain` and `firewall-irr` at a guest stop | The spec first records which mode the guest test runs the two plugins in. If internal, the row is a candidate for "already fixed by 32fa1ed795" and is marked fixed only when the guest reproduction shows no early exit; if forked, the reproduced cause is recorded with its log lines | The row stays open when the reproduction does not run or does not decide, because a fix claimed without it is unproven |
| AC-11 | A peer whose config attaches per-peer OPEN policy sends its OPEN while no running generation of the policy plugin answers: the plugin was retired, its restart failed, it is past its bound, or its replacement is still in its handshake | The OPEN ze sends carries the plugin's declared capability, the peer's OPEN is refused with the Role Mismatch NOTIFICATION (code 2, subcode 11), the log names the peer and the plugin, and the plugin's doctor row says peers wait on it; once a generation runs, the same peer establishes. Lands no later than AC-1 | A peer the plugin declared nothing for establishes as before. A plugin an accepted reload removed from the config owes nothing (`finishRemovalScope` / `unconfigureProcessless` clears its owed record and kept declarations on reload acceptance, preserving them if the reload is rejected). Capabilities of a plugin that validates no OPEN are released at the retire, because its sessions would otherwise run with a capability nothing serves |
| AC-12 | A reload or a restart command names a plugin whose stopped predecessor has not finished its runtime cleanup | The plugin is not started, the reload or command is refused with the late-cleanup reason, and `ze doctor` shows the plugin down with the cleanup it still owes | Once that cleanup has run, the next reload or restart starts it, because the record forgets a name whose cleanup ran |
| AC-13 | A forked plugin's program exits before it connects back | Its start fails at once with an error naming the exit status, and the stop record of the plugin carries that status | A child that is still alive waits the full connect-back bound (30 s), because a slow start is not a failure |
| AC-14 | `writeBatchWithDeadline` (`pkg/plugin/rpc/conn.go`, guard already landed alongside `MuxConn.CallBatchRPC`) is called with a context that has ended | Direct `Conn` unit tests in `pkg/plugin/rpc/conn_test.go` prove it returns the context's error before writing (matching `writeFrame`) so the plugin receives nothing, and delivers and acknowledges a batch under a live context | A batch whose write began before the context ended completes, because the bytes are already on the connection |
| AC-15 | The hub's config fence has finished and `BeginDrain` has sealed post-fence admission, then a reload, commit, or startup phase reaches the plugin server; conversely, an already-admitted commit reload, in-flight SIGHUP reload, or confirmed-commit revert runs inside the config fence (after `SignalShutdownRequested` and before `BeginDrain`) | Once `BeginDrain` runs, `reloadConfig` and `runPluginPhaseWithin` refuse with `transaction.ErrShutdown` and start or restart no plugin; during the config fence before `BeginDrain`, an already-admitted reload or revert is not aborted by `stopUnderWay` and finishes (or is stood down at `reloadShutdownGrace` by `fenceCommitReloads`/`awaitReloadWorker`), as `signals.md` specifies | A transaction already in flight when `BeginDrain` or `Server.Stop` arrives is stood down by `fenceCommitReloads` / `stopTransaction` as today. Inside the config fence, `restartPluginProcess` still refuses plugin-exit respawns on `stopUnderWay` (AC-8), so no crashed plugin is respawned while stopping |

AC-9 (the firewall vpp backend at boot) moved on 2026-11-11 to `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md`; the ids are not renumbered.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a plugin that crashes, then reloads to bring it back | exit -> retire -> stop record -> `ze signal reload` -> restart path -> five stages | `test/plugin/failure-policy-ignore-reload.ci`, `test/plugin/failure-policy-ignore-reload-external.ci` |
| 2 | Asks `ze doctor` why a feature is missing, then runs the fix it names | retire -> `doctor-plugin-stopped` -> `ze request plugin restart <name>` | `test/plugin/doctor-plugin-stopped.ci` (exists, re-run against the retire) |
| 3 | Runs a peer with a configured role while the role plugin is down | OPEN -> policy set from the owed record -> Role Mismatch -> restart -> Established | `test/plugin/role-open-refused-validator-down.ci` and its external twin |
| 4 | Stops the daemon while a plugin is unstable | signal -> stop fact -> refusal -> fence -> drain -> teardown | `test/plugin/stop-fence-respawn-refused.ci` |
| 5 | Configures an external plugin whose program fails at once | fork -> exit -> reaped -> start fails with the status | `test/plugin/plugin-exits-before-connect-back.ci` |

## 🧪 TDD Test Plan

Tests that exist and already prove a part are named, not rewritten: `TestStoppedRecordRaisesAndClears`, `TestPluginStoppedEveryDownPath`, `TestPluginStoppedRowWhileCleanupOwed`, `TestRespawnRefusedWhileDraining`, `TestRestartPluginRestartsAPluginThatDeclaredNothing`, `test/plugin/doctor-plugin-stopped.ci`, `test/plugin/plugin-restart-external-running.ci`. Before writing a row below, the phase searches its package for a test that already asserts it and names that test instead.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIgnoredExitRetiresGeneration` | `internal/component/plugin/server/failure_policy_test.go` | AC-1, AC-4, A-1, A-9: after an `ignore` exit the registry row, families, aliases and shapes are gone, the slot and loaded mark are free, the crash event is listed | |
| `TestRetireRecordsStopAfterSlotRemoval` | `internal/component/plugin/server/failure_policy_test.go` | AC-4: the stop record and its bus warning stand after the retire | |
| `TestRefusedRestartAfterExitRetiresGeneration` | `internal/component/plugin/server/failure_policy_test.go` | AC-1 for a restart refused by the limit or a disable | |
| `TestRetireLeavesReplacementSlot` | `internal/component/plugin/server/failure_policy_test.go` | AC-4 survivor | |
| `TestBrokenRestartRefusedKeepsRegistrations` | `internal/component/plugin/server/restart_refused_test.go` | AC-1 survivor | |
| `TestRestartOfRetiredPluginMarksItLoaded` | `internal/component/plugin/server/failure_policy_test.go` | A-6 | |
| `TestExitAfterStopBeganRetiresNothing` | `internal/component/plugin/server/failure_policy_test.go` | AC-2: with the stop signalled and the server context live, no release and no retire, and the crash report is raised | |
| `TestRestartRefusedOnceStopBegan` | `internal/component/plugin/server/restart_refused_test.go` | AC-8: `ErrDaemonStopping` after `SignalShutdownRequested` with no `BeginDrain`, and no stop record | |
| `TestPluginPhaseRefusedOnceDrainBegan` | `internal/component/plugin/server/startup_test.go` | AC-15 at `runPluginPhaseWithin` once `BeginDrain` sealed post-fence admission | |
| `TestReloadAfterDrainBeganRefused` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-15 at `reloadConfig` once `BeginDrain` sealed post-fence admission | |
| `TestFencedReloadCompletesAfterStopSignalBeforeDrain` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-15: an admitted reload after `SignalShutdownRequested` and before `BeginDrain` finishes through the config fence | |
| `TestReloadRestartsDownPlugin` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-3 with an empty diff | |
| `TestReloadWithDiffRestartsDownPluginBeforeVerify` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-3: the restarted plugin receives the verify of the change | |
| `TestReloadRefusedWhenDownPluginDoesNotStart` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-3 survivor: nothing applied, the record names the new cause | |
| `TestReloadKeepsRespawnLimitDisable` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-3 survivor: the refusal names the restart command | |
| `TestReloadRemovingDownPluginClearsItsRecordsOnAcceptOnly` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-3 survivor, AC-11 survivor, R-5: accepted reload clears stop record and owed OPEN-policy record; rejected reload preserves both | |
| `TestReloadAllRunningEmptyDiffStartsNothing` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-3: the no-change reload still does nothing | |
| `TestReloadDoesNotStartWhileCleanupOwed` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-12 | |
| `TestOpenPolicyOwedAfterValidatorRetired` | `internal/component/plugin/server/open_policy_owed_test.go` | AC-11: the producer names the retired validator and its capability is still injected for the peer | |
| `TestOpenPolicyOwedDuringRestartWindow` | `internal/component/plugin/server/open_policy_owed_test.go` | AC-11 between the release of the old generation and Stage 3 of the new | |
| `TestOpenPolicyClearedWhenValidatorRuns` | `internal/component/plugin/server/open_policy_owed_test.go` | AC-11: running clears the record | |
| `TestOpenPolicyClearedWhenReloadRemovesPlugin` | `internal/component/plugin/server/open_policy_owed_test.go` | AC-11 survivor | |
| `TestNonValidatorCapabilitiesReleasedAtRetire` | `internal/component/plugin/server/open_policy_owed_test.go` | AC-11 survivor, AC-1 | |
| `TestReplacementReplacesKeptOpenPolicyDeclarations` | `internal/component/plugin/server/open_policy_owed_test.go` | A-5 | |
| `TestOpenRefusedWhileValidatorDown` | `internal/component/bgp/server/rfc9234_validate_test.go` | AC-11: the refusal carries code 2, subcode 11. RFC 9234 tagged, with its discrimination record | |
| `TestAskOpenValidatorsSkipsHandshakingProcess` | `internal/component/bgp/server/rfc9234_validate_test.go` | AC-7 | |
| `TestRunningProcessesOmitsHandshakingReplacement` | `internal/component/plugin/process/manager_test.go` | AC-7 | |
| `TestRegistrationConcurrentReadWrite` | `internal/component/plugin/process/process_test.go` | AC-7, under `-race`; no such test exists today | |
| `TestByNameSendersRefuseHandshakingProcess` | `internal/component/plugin/server/server_test.go` | AC-7: one table row per by-name sender | |
| `TestEngineEventUnsubscribedHandlerSkippedInSnapshot` | `internal/component/plugin/server/engine_event_test.go` | AC-6: two handlers that unsubscribe each other, exactly one is called, because the dispatch order comes from a map | |
| `TestEngineEventSelfUnsubscribeReturns` | `internal/component/plugin/server/engine_event_test.go` | AC-6: a handler that unsubscribes itself returns | |
| `TestEngineEventOtherSubscribersStillCalled` | `internal/component/plugin/server/engine_event_test.go` | AC-6 positive half | |
| `TestStartExternalFailsWhenChildReaped` | `internal/component/plugin/process/process_test.go` | AC-13 | |
| `TestStartExternalWaitsForLiveChild` | `internal/component/plugin/process/process_test.go` | AC-13 survivor, with the connect-back bound shortened through the package's test seam | |
| `TestWriteBatchRefusesEndedContext` | `pkg/plugin/rpc/conn_test.go` | AC-14 | |
| `TestWriteBatchDeliversUnderLiveContext` | `pkg/plugin/rpc/conn_test.go` | AC-14 positive half | |

### Boundary Tests (numeric inputs)
N-A: the change takes no numeric input. The restart bounds and the connect-back bound are unchanged.

### Functional Tests
Each new `.ci` is drafted under `test/draft/<suite>/` and promoted (`/ze-test`).

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `failure-policy-ignore-reload` | `test/plugin/failure-policy-ignore-reload.ci` | An internal plugin declaring `ignore` exits; its command answers unknown; doctor names it; `ze signal reload` starts it; its command answers | |
| `failure-policy-ignore-reload-external` | `test/plugin/failure-policy-ignore-reload-external.ci` | The same with a forked plugin under an explicit block | |
| `failure-policy-restart-bound-reload` | `test/plugin/failure-policy-restart-bound-reload.ci` | A plugin past its restart bound is down; a reload is refused and names `ze request plugin restart`; that command starts it | |
| `failure-policy-respawn-false-reload` | `test/plugin/failure-policy-respawn-false-reload.ci` | `respawn false` on a `restart` plugin: it stays down after its exit and a reload starts it | |
| `role-open-refused-validator-down` | `test/plugin/role-open-refused-validator-down.ci` | A peer with per-peer OPEN policy is refused with NOTIFICATION 2/11 while the policy plugin is down, ze's OPEN still carries the capability, and the peer establishes after the restart | |
| `role-open-refused-validator-down-external` | `test/plugin/role-open-refused-validator-down-external.ci` | The same with the policy plugin forked | |
| `stop-fence-respawn-refused` | `test/plugin/stop-fence-respawn-refused.ci` | A `restart` plugin exits after SIGTERM and before the drain: the refusal line is logged, no retire or release line is, exit 0 | |
| `reload-after-stop-refused` | `test/reload/reload-after-stop-refused.ci` | A reload sent after the stop signal, while a drain holds the stop open, starts nothing and ze exits 0 | |
| `plugin-exits-before-connect-back` | `test/plugin/plugin-exits-before-connect-back.ci` | An external plugin whose program exits at once: ze answers at once, and doctor names the exit status | |

Existing tests re-run because their contract is reached (`ai/rules/testing.md`, "The Affected Population"): `test/plugin/doctor-plugin-stopped.ci`, `test/plugin/plugin-restart-external-running.ci`, `test/plugin/plugin-restart-external-ignores-stop.ci`, `test/plugin/failure-policy-restart.ci`, `test/plugin/failure-policy-fatal.ci`, `test/plugin/failure-policy-fatal-live-peer.ci`, `test/plugin/failure-policy-disagreement.ci`, `test/plugin/drain-respawn-refused.ci`, `test/plugin/drain-plugin-failure-stop.ci`, `test/plugin/shutdown-is-prompt.ci`, `test/reload/tx-protocol-external-plugin.ci`, `test/reload/reload-plugin-only-no-change.ci`.

Commands: `./le test unit pkg path ./internal/component/plugin/server/`, `./le test unit pkg path ./internal/component/plugin/process/`, `./le test unit pkg path ./internal/component/plugin/doctor/`, `./le test unit pkg path ./internal/component/bgp/server/`, `./le test unit pkg path ./pkg/plugin/rpc/`, `./le test functional plugin`, `./le test functional reload`, and for the tagged test `./le rfc record` then `./le rfc check`. The main thread runs them, one at a time.

### Lifecycle scenarios (`ai/rules/testing.md`, "Lifecycle Regressions")
| Scenario | Reached? | Proof or reason |
|----------|----------|-----------------|
| Process ends after it declared: `restart`, `ignore`, `fatal` | Yes | `test/plugin/failure-policy-restart.ci`, `test/plugin/failure-policy-ignore-reload.ci` and its external twin, `test/plugin/failure-policy-fatal.ci` |
| A plugin fails a startup stage after it declared | Yes, through the shared release | `TestNonValidatorCapabilitiesReleasedAtRetire` and the existing startup-failure tests of the server package; the outcome is unchanged for a plugin that validates no OPEN |
| A restarting plugin passes its restart bound | Yes | `test/plugin/failure-policy-restart-bound-reload.ci` |
| The `respawn` leaf disagrees | `false` on a `restart` plugin: yes, `test/plugin/failure-policy-respawn-false-reload.ci`. `true` on a non-restart plugin: not reached, the Stage 1 refusal is untouched (`test/plugin/failure-policy-disagreement.ci` re-run) |
| A plugin exits during the stop drain or teardown | Yes | `test/plugin/drain-respawn-refused.ci`, `test/plugin/stop-fence-respawn-refused.ci` |
| A signal during the stop | The fence state is reached | `test/plugin/stop-fence-respawn-refused.ci`; the drain and teardown states keep `test/plugin/drain-plugin-failure-stop.ci` |
| A reload or commit after shutdown began | Yes | `test/reload/reload-after-stop-refused.ci`, `TestReloadAfterDrainBeganRefused`, `TestFencedReloadCompletesAfterStopSignalBeforeDrain` |
| A plugin crashes during verify or apply, misses its deadline, or reports `broken` | Yes, through `restartPlugin` | `test/reload/tx-protocol-external-plugin.ci`, `TestBrokenRestartRefusedKeepsRegistrations` |
| A reload's context ends during a handshake, or a sibling fails at the barrier | Yes, through the rollback release | Existing reload-rollback tests of the server package, re-run; outcome unchanged |
| A plugin dispatches another plugin's command at startup | Not reached | No dispatch path is edited |

Seams: AC-7's interleaving (a sender between `Respawn` and Stage 5), AC-11's restart window, AC-12's late cleanup and A-1's cleanup order are Go tests with a controlled seam, because a `.ci` cannot hold a process inside its handshake or its cleanup.

### Interop Tests (Scope: protocol)
N-A: no new wire behavior. The NOTIFICATION of AC-11 is the one ze already sends for an unanswered OPEN policy, and what a peer does with a Role Mismatch is unchanged, so no FRR or BIRD source was read for this design.

## Files to Modify
- `internal/component/plugin/server/failure_policy.go` - replace `ignore`/unspecified/unknown's direct `RemovePluginCapabilities` with the shared retire path and the `stopUnderWay` guard in `applyFailurePolicy`; its comments
- `internal/component/plugin/server/restart.go` - the kept-declaration branch of `releasePluginRegistrations`, the OPEN-policy owed record beside `lateCleanupRecord`; comments on the release and on `rollbackReplacement`, which now also serves an ended generation
- `internal/component/plugin/server/reload_tx.go` - `restartPluginProcess` reads `stopUnderWay` and returns `transaction.ErrRestartRefused` wrapping `process.ErrDaemonStopping` (`restartReplacementWithin` already skips `RecordStopped` on `ErrRestartRefused`)
- `internal/component/plugin/server/startup_removal.go` - `finishRemovalScope` / `unconfigureProcessless` (and non-provisional final stops) clear the owed OPEN-policy record and kept declarations on reload acceptance (preserving them on rejection)
- `internal/component/plugin/server/startup.go` - `runPluginPhaseWithin` refuses once `BeginDrain` sealed post-fence admission (beside `stopEntered`); the loaded mark of A-6; the Stage 3 capability call of A-5
- `internal/component/plugin/server/reload.go` - the restart of down plugins (excluding `collectReloadStopProcesses`) and refusal once `BeginDrain` sealed post-fence admission
- `internal/component/plugin/server/server.go` - `PluginsWithPerPeerOpenPolicy` reads the owed record and its comment is rewritten; `CallFilterUpdate`, `CallDoctorCheck`, `DoctorCheckPlugins` use the running-only accessor
- `internal/component/plugin/server/source_policy.go`, `internal/component/plugin/server/resolver_cache.go` - `CaptureSourcePolicyOwners`, `ResolverCacheOwners` and `CallResolverCacheClear` use the running-only accessor
- `internal/component/plugin/server/doctor_snapshot.go`, `internal/component/plugin/doctor/checks_runtime.go` - the down row says peers wait on the plugin's OPEN check
- `internal/component/plugin/server/engine_event.go` - retired mark; the "IS still called once more" comment is replaced
- `internal/component/plugin/server/drain.go` - `BeginDrain` seals post-fence reload and startup-phase admission (`txLock.closeAdmission()` / post-fence drain guard) and updates its comment
- `internal/component/plugin/process/manager.go` - running-only accessor; comments on `Respawn` and `RefuseRespawns`
- `internal/component/plugin/process/stopped.go` - the accessor listing recorded stops
- `internal/component/plugin/process/process.go` - `startExternal` ends its wait on the reaped channel
- `internal/component/bgp/server/validate.go` - `askOpenValidators` uses the accessor
- `pkg/plugin/rpc/conn_test.go` - direct `Conn` regression tests for AC-14 (`TestWriteBatchRefusesEndedContext`, `TestWriteBatchDeliversUnderLiveContext`)
- `pkg/ze/eventbus.go` - the `Subscribe` comment states the tightened contract
- `docs/architecture/api/process-protocol.md` - the failure-policy table (what "may be restarted" means against a reload and the operator's command), the down-path paragraph (retire, reload restart, kept OPEN policy), "A stop ends every restart", the engine-event unsubscribe contract, and the sentence of A-7
- `docs/architecture/hub-architecture.md` - update "From the start of the drain the stop is terminal" to state that plugin-exit respawns are refused from the stop signal (`SignalShutdownRequested`) while fenced commit reloads finish before the drain
- `docs/architecture/api/capability-contract.md` - a down plugin's kept OPEN-policy declarations
- `docs/architecture/plugin/component-boundaries.md` - the `Subscribe` contract
- `docs/architecture/doctor-and-health-checks.md` - the down row's added sentence, where the page describes plugin state
- `docs/guide/health-checks.md` - the `doctor-plugin-stopped` doctor row, the stale `plugins` row of "Registered Components", the report-bus row
- `docs/guide/plugins.md` - what an `ignore` exit leaves and the two ways back
- `rfc/short/rfc9234.md` - the test tag of AC-11, through `/ze-rfc`
- `plan/journal/rollback-forgets-partial-apply.md`, `plan/journal/plugin-startup-barrier-deadlock.md`, `plan/journal/false-synchronization-claim.md`, `plan/journal/late-write-lands-on-the-successor.md`, `plan/journal/stop-leaves-the-work-running.md`, `plan/journal/guard-added-to-one-half-of-a-pair.md`, `plan/journal/wait-outlives-the-process-it-waits-on.md` - each owned row marked as its phase lands

Read and left unchanged: `internal/component/bgp/server/events.go` and `internal/component/bgp/reactor/policy_reload.go` (AC-7 survivors), `internal/component/plugin/registration.go` (the replacement function exists), `cmd/ze/hub/main.go` (`SignalShutdownRequested` and `BeginDrain` already bracket the config fence), `internal/component/plugin/server/dispatch.go` and `internal/component/plugin/server/rpc_register.go` (nothing new is released from the cleanup body), `internal/component/plugin/server/reload_dataplane_writer.go` and `internal/component/plugin/server/startup_autoload.go` (`collectReloadStopProcesses` and `stopCollectedProcessesMode` already collect and schedule `processless` removed down plugins for acceptance-scoped unconfiguration), and `pkg/plugin/rpc/conn.go` / `pkg/plugin/rpc/mux.go` (`writeBatchWithDeadline` and `CallBatchRPC` already check `ctx.Err()` before writing).

## Files to Create
- `internal/component/plugin/server/reload_down_plugin_test.go`
- `internal/component/plugin/server/open_policy_owed_test.go`
- `test/plugin/failure-policy-ignore-reload.ci`
- `test/plugin/failure-policy-ignore-reload-external.ci`
- `test/plugin/failure-policy-restart-bound-reload.ci`
- `test/plugin/failure-policy-respawn-false-reload.ci`
- `test/plugin/role-open-refused-validator-down.ci`
- `test/plugin/role-open-refused-validator-down-external.ci`
- `test/plugin/stop-fence-respawn-refused.ci`
- `test/plugin/plugin-exits-before-connect-back.ci`
- `test/reload/reload-after-stop-refused.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No RPC and no leaf is added |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command; `ze signal reload` and `ze request plugin restart` exist |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC; the lifecycle `.ci` tests are in the TDD plan |
| Pipe completeness | N-A | No output path |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | Yes | "A configured plugin is down": `doctor-plugin-stopped`, raised by `recordStoppedLocked`, fix `ze request plugin restart <name>`, runtime phase (exists). "Peers are refused because their OPEN policy plugin is down": the same row, with the added sentence, since the cause and the fix are the plugin's. "A reload was refused because a down plugin did not start": `doctor-config-reload-refused` through `raiseReloadRefused`, with the restart command as its fix. "A forked plugin exited before connecting back": the stop record's exit status in the `doctor-plugin-stopped` row |
| Prometheus counters/metrics | No | `ze_plugin_status` and `ze_plugin_restarts_total` keep their meaning |
| BGP family surface | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or scope changed? | No | A defect fix; `./le feature report` is read in phase 3 to confirm no feature page claims the old behavior |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md` (three rows), `docs/guide/plugins.md` |
| 7 | Wire format changed? | No | Census |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, the sections named in Files to Modify; `docs/plugin-development/handlers.md` if it states the unsubscribe contract (searched in phase 6) |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9234.md` and its `docs/features/rfc-status.md` row: the Section 4.2 refusal is newly proven for a policy plugin that is down |
| 10 | Test infrastructure changed? | Yes | The failure fixture gains an `ignore` mode and a validator mode (A-8); `docs/functional-tests.md` if it lists fixture modes (searched in phase 1) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `ai/digests/plugin-transport.md` and `ai/digests/hub-engine.md` if they describe a generation's end, `Respawn` or the start of the respawn refusal (read in phases 3 and 7) |
| 13 | Route metadata keys? | No | - |
| 14 | Prometheus counters? | No | - |
| 15 | Registered plugin, event type, command or inventory changed? | No | - |
| 16 | Changed source file referenced by doc source anchors? | Yes | Derived per phase with `./le spec citation anchors spec plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` |
| 17 | Existing docs show examples for this area? | Yes | `docs/guide/plugins.md` failure-policy examples checked against the new end state |

## Design presentation

### What changes

This spec closes the remaining plugin-process lifecycle defects at their existing choke points, with no new owner object, generation counter, or source gate. A plugin generation that exits without a successor (`ignore`, unspecified, unknown, or a restart refused before `Respawn` stopped anything) is retired through the same release steps startup rollback and removal already use (replacing `ignore`/unspecified/unknown's direct `RemovePluginCapabilities` calls), leaving its slot and loaded mark clear while keeping its stop record and `doctor-plugin-stopped` finding. For a plugin that holds per-peer OPEN policy (such as `bgp-role`), its per-peer capability declarations stay injected and an "OPEN policy owed" record keeps incoming peer OPENs failing closed with Role Mismatch (RFC 9234 Sections 4.1 and 4.2) while the plugin is down or restarting, until a running replacement takes over or an accepted reload removes it (`finishRemovalScope` / `unconfigureProcessless`). Every config reload restarts configured down plugins not removed by the candidate through `restartPluginReplacement` (`reenable = false`, refusing the reload if restart fails, without lifting respawn-limit disables). Runtime senders read only running processes so a handshaking replacement never receives runtime RPCs; engine-event dispatch checks a retired mark before invoking each handler in its snapshot; `SignalShutdownRequested` (`stopUnderWay`) immediately ends crash-triggered respawns and mid-stop retires across the config fence, drain, and teardown, while `BeginDrain` seals post-fence reload and startup-phase admission so already-admitted commit reloads, queued SIGHUP reloads, and confirmed-commit reverts still finish during the config fence; forked plugins that exit before TLS connect-back fail immediately with their exit status instead of waiting 30 s; and direct `Conn` regression tests pin the ended-context check already present in `writeBatchWithDeadline`.

### Before and after

1. **A plugin with failure policy `ignore` exits, then the operator runs its command and reloads:**
   - Before: `applyFailurePolicy` removes injected capabilities directly, but leaves the dead process in the manager slot with its commands, families, pipe aliases, and answer shapes published; calling its command fails against a closed connection, a reload touching its config roots fails verify against the dead connection, and `ze signal reload` with no diff answers `config reload: no changes` leaving the plugin down.
   - After: `applyFailurePolicy` retires the dead process through the shared release path and writes the stop record after slot removal; calling its command answers unknown command, `ze doctor` reports `doctor-plugin-stopped` with `ze request plugin restart <name>`, and `ze signal reload` (with or without a diff) restarts the plugin through `restartPluginReplacement`, verifies and applies any changed roots against it, and clears the stop record.

2. **A peer with a configured BGP Role sends OPEN while `bgp-role` is down or restarting:**
   - Before: `PluginsWithPerPeerOpenPolicy` returns empty after an `ignore` exit (which currently removes capabilities), after a failed restart, or during a restart window, so ze sends OPEN without the Role capability and accepts the peer's OPEN with no role check.
   - After: ze still sends the Role capability in its OPEN, refuses the peer's OPEN with NOTIFICATION Role Mismatch (code 2, subcode 11), logs the peer and plugin name, and adds a sentence to the `doctor-plugin-stopped` row saying peers wait on the plugin's OPEN check; once a generation reaches running (or an accepted reload removes it), the owed record clears.

3. **A plugin past its respawn limit during reload vs manual restart:**
   - Before: a reload skips the down plugin or fails verify against its dead slot.
   - After: `ze signal reload` refuses the reload naming the plugin and `ze request plugin restart <name>` without lifting the respawn-limit disable; running `ze request plugin restart <name>` (`RespawnReenabled`) lifts the disable and starts the plugin.

4. **SIGTERM arrives, then a `restart` plugin exits during the config fence, or a reload arrives during the fence vs after `BeginDrain`:**
   - Before: `RefuseRespawns` has not fired during the config fence, so a `restart` plugin that exits before `BeginDrain` is respawned and torn down again; and after `BeginDrain`, `reloadConfig` / `runPluginPhaseWithin` admission stays open during the drain until `Server.Stop`.
   - After: `restartPluginProcess` and `applyFailurePolicy` read `stopUnderWay()` (`SignalShutdownRequested`), so respawns fail immediately with `ErrRestartRefused` + `respawn refused: ze is stopping` (with no stop record written) and no retire strips closing sessions' capabilities; already-admitted commit reloads, queued SIGHUP reloads, and confirmed-commit reverts still finish cleanly inside the config fence; and once `BeginDrain` runs after the fence, reload and startup-phase admission are sealed with `transaction.ErrShutdown`, and ze exits 0.

5. **An external plugin binary exits immediately before TLS connect-back:**
   - Before: `startExternal` waits out the full 30-second connect-back timeout before failing.
   - After: `startExternal` watches the `reaped` channel alongside `waiter.Wait`, failing immediately with the child's exit status and recording that status in `doctor-plugin-stopped`.

6. **Runtime senders during a replacement's handshake and unsubscribed engine-event handlers:**
   - Before: `askOpenValidators`, `CallFilterUpdate`, `CallDoctorCheck`, `DoctorCheckPlugins`, `CaptureSourcePolicyOwners`, `ResolverCacheOwners`, and `CallResolverCacheClear` can reach a replacement between `Respawn` and Stage 5; an engine-event handler unsubscribed during dispatch is still invoked once more from the snapshot.
   - After: all runtime senders read only `StageRunning` processes (by-name senders report unavailable while handshaking), and engine-event dispatch checks each subscription's retired mark immediately before invoking it.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Release every capability of a down OPEN-policy validator and keep a copied list of peer selectors (D-A alternative) | Ze would send OPEN without the Role capability to a role-configured peer, violating RFC 9234 Section 4.1, and the copied list duplicates declarations and goes stale. |
| Keep every capability of every down plugin (D-A alternative) | Ze would advertise Graceful Restart or address families on sessions with no backing plugin. |
| Add a Stage 1 wire field naming the config path that implies OPEN policy (D-A alternative) | Introduces a new wire field for a fact already expressed by existing `WantsValidateOpen` and per-peer capability declarations. |
| Defer restarting down plugins on reload to `spec-single-owner-reload-reconcile` (D-B alternative) | Once AC-1 retires a down plugin's slot, a reload modifying its config roots would commit a change its owner never verified. |
| Start down plugins during reload through a second auto-load path (D-B alternative) | Re-implements boot's needed-set computation and bypasses `restartControllerSet`, bounds, and late-cleanup refusal. |
| Let config reload lift a respawn-limit disable (D-B alternative) | A managed config push or commit revert would restart a crash loop no operator asked to re-enable. |
| Move the five name-keyed releases into `cleanupProcess` | `cleanupProcess` also runs for a replaced generation and for a late cleanup, where a by-name release would strip the live successor's registrations. |
| Key the stop guard on `s.ctx.Err()` instead of `stopUnderWay()` | `s.ctx` ends only in `Server.Stop`, after both the config fence and the stop drain, leaving both windows unprotected. |
| Refuse `reloadConfig` and `runPluginPhaseWithin` unconditionally on `stopUnderWay()` | `SignalShutdownRequested` runs before `awaitReloadWorker` and `fenceCommitReloads`, so gating reloads on `stopUnderWay()` would abort an already-admitted commit reload, queued SIGHUP reload, or confirmed-commit revert during the config fence, contradicting `signals.md` ("Stop sequence"). Sealing post-fence admission in `BeginDrain` preserves the config fence while still blocking reloads during the drain and teardown. |
| Make engine-event `unsubscribe` block until in-flight handlers finish | Deadlocks any handler that unsubscribes itself and any caller unsubscribing while holding a lock the handler takes. |
| Add individual stage checks at each of the eight runtime sender sites | Eight places to drift or forget instead of one running-only manager accessor. |

### Documents departed from

1. **`docs/architecture/api/process-protocol.md`**, section `Filter Insertion` (under **Failure-Policy Declaration (Stage 1)**):
   - Current statement: `"A stop ends every restart. From the start of the stop drain (\`BeginDrain\`), the manager refuses each respawn with \`respawn refused: ze is stopping\`, so a plugin that exits during the drain or teardown stays down: the daemon is going away and a new generation would only be torn down again."`
   - After this work: Respawn refusal begins at the stop signal (`SignalShutdownRequested` / `stopUnderWay`), covering the config fence as well as the stop drain and teardown; `BeginDrain` retains `RefuseRespawns` as the manager's own guard and seals post-fence reload/phase admission. Updated in phase 7.
2. **`docs/architecture/api/process-protocol.md`**, section `Filter Insertion` (under **Failure-Policy Declaration (Stage 1)**):
   - Current statement: `"One value answers two questions, because the engine starts a plugin again for exactly one reason. \`restart\` is also the plugin's statement that it MAY be started again, and the other two are its statement that it must not be."` (and the `ignore` table row `"May the plugin be restarted? No"`).
   - After this work (approved by Thomas on 2026-11-24 as D-B): `failure-policy` governs automatic recovery when a generation exits (`restart` auto-restarts on exit, `ignore` retires and leaves the plugin down, `fatal` stops ze); an explicit config reload (for a configured down plugin not disabled by the respawn limit) and `ze request plugin restart <name>` both start a configured down plugin again through `restartPluginReplacement` / `restartPluginReplacementReenabled`. Updated in phase 4.
3. **`docs/architecture/hub-architecture.md`**, section `SSH session model factory`:
   - Current statement: `"From the start of the drain the stop is terminal: a plugin that exits is not started again."`
   - After this work: From the stop signal (`SignalShutdownRequested`), a plugin that exits during the config fence, the drain, or teardown is not started again; already-admitted commit reloads finish during the config fence, and once `BeginDrain` runs no reload or startup phase starts. Updated in phase 7.

### Choices for Thomas

none

## Implementation Steps

Each phase is one commit, lands when its scoped tests pass, and edits its pages in the same commit. No phase runs a whole-tree check; evidence is the scoped commands in the TDD plan, run by the main thread.

1. **Phase: Wiring (MANDATORY FIRST)** -- the failing end-to-end tests
   - Inputs: the failure fixture (A-8), the plugin-system page and the plugin-transport digest.
   - Tests: `test/plugin/failure-policy-ignore-reload.ci` and `test/plugin/role-open-refused-validator-down.ci`, drafted.
   - Verify: the first is red at "reload starts it". The second is red today for a failed restart or `ignore` exit (the session establishes); the log is pasted.
2. **Phase: OPEN policy survives its plugin** -- AC-11
   - Constraint: the producer never moves to the running-only accessor. The declarations are kept only on a path that leaves the plugin configured; reload removals clear them only inside `finishRemovalScope` / `unconfigureProcessless` on reload acceptance (`accepted == true`), preserving them on reload rejection.
   - Tests: the six owed-record unit rows, `TestOpenRefusedWhileValidatorDown` with its RFC tag and record, both `.ci` files.
   - Files: the release, the producer, `startup_removal.go` (`finishRemovalScope` / `unconfigureProcessless`), the Stage 3 capability call, the doctor snapshot and row; the process-protocol, capability-contract and doctor pages; `rfc/short/rfc9234.md`.
   - Verify: A-5 confirmed or broken.
3. **Phase: an ended generation is retired** -- AC-1, AC-2, AC-4, AC-5
   - Constraint: order is cleanup complete, slot removed by pointer comparison, process stopped and joined within its stop bound, registrations released (replacing `ignore`/unspecified/unknown's direct `RemovePluginCapabilities` calls), loaded mark cleared, stop recorded. Nothing of it runs once `stopUnderWay` is true.
   - Tests: the seven retire and stop-guard unit rows; `test/plugin/doctor-plugin-stopped.ci` re-run.
   - Files: the failure policy, the restart file's comments; the process-protocol and guide pages.
   - Verify: A-1, A-6 and A-9 confirmed or broken. Journal row rollback 2026-08-21 is marked fixed once phase 4 lands.
4. **Phase: a reload restarts a down plugin** -- AC-3, AC-12
   - Inputs: A-2 and A-7.
   - Tests: the seven reload unit rows (`TestReloadRemovingDownPluginClearsItsRecordsOnAcceptOnly` verifying both accepted and rejected scopes); the four reload `.ci` files; the wiring test of phase 1 goes green and is promoted.
   - Files: `reload.go`, the stop-record accessor in `stopped.go`; the process-protocol page.
5. **Phase: runtime senders see running processes only** -- AC-7
   - Tests: the manager, process, OPEN-validation and by-name sender rows.
   - Files: the accessor and its callers.
   - Verify: journal rows barrier 2026-09-08 and false-synchronization 2026-10-03 marked fixed.
6. **Phase: engine event unsubscribe** -- AC-6
   - Inputs: the census of A-3, recorded in Design Insights.
   - Tests: the three engine-event rows.
   - Files: the engine event subscribers, `pkg/ze/eventbus.go`, the process-protocol and component-boundaries pages, and each subscriber the census finds unsafe.
   - Verify: journal row late-write 2026-09-09 marked fixed.
7. **Phase: the stop fact and post-fence drain seal** -- AC-8, AC-15, AC-10
   - Inputs: the mode the guest runs the firewall plugins in, then the QEMU-guest reproduction, with the log kept in the commit body.
   - Tests: `TestRestartRefusedOnceStopBegan`, `TestPluginPhaseRefusedOnceDrainBegan`, `TestReloadAfterDrainBeganRefused`, `TestFencedReloadCompletesAfterStopSignalBeforeDrain`, `test/plugin/stop-fence-respawn-refused.ci`, `test/reload/reload-after-stop-refused.ci`, the two drain `.ci` re-runs.
   - Files: `reload_tx.go` (`restartPluginProcess`), `drain.go` (`BeginDrain`), `startup.go` (`runPluginPhaseWithin`), `reload.go` (`reloadConfig`); the process-protocol page ("A stop ends every restart") and `docs/architecture/hub-architecture.md` ("From the start of the drain the stop is terminal").
   - Verify: journal row stop 2026-09-17 marked fixed only when AC-10's reproduction decided; otherwise it stays open with the attempt added.
8. **Phase: the reap-aware connect-back guard and batch-write regression tests** -- AC-13, AC-14
   - Tests: the two `startExternal` rows, the two direct `Conn` batch-write rows in `pkg/plugin/rpc/conn_test.go`, `test/plugin/plugin-exits-before-connect-back.ci`.
   - Files: `internal/component/plugin/process/process.go`, `pkg/plugin/rpc/conn_test.go` (`conn.go` and `mux.go` already carry the `ctx.Err()` check).
   - Verify: journal rows wait-outlives 2026-11-13 and guard-half-pair 2026-11-13 marked fixed.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named symbol |
| Feature completeness | Each user story passes in internal and forked mode |
| Correctness | The retire compares pointers and never runs once the stop is under way; the stop is recorded after the slot removal; the reload set excludes the removal collection; the owed record is cleared only by a running generation or an accepted reload removal (preserving it on reload rejection) |
| Naming | Log lines keep `plugin` as the key; every new line about a down plugin carries `doctor-plugin-stopped` |
| Data flow | No engine or config code names a plugin |
| Operator errors and doctor codes | Each refusal (reload, restart, OPEN) names the plugin, why, and `ze request plugin restart <name>` where that is the fix |
| Rule: stale-comments | The comments on `releasePluginRegistrations`, `rollbackReplacement`, `applyFailurePolicy`, `PluginsWithPerPeerOpenPolicy`, `checkPluginStopped`, `ResolverCacheOwners`, `dispatch`, `BeginDrain`, `RefuseRespawns` and `Respawn` describe the new behavior |
| Rule: goroutine-lifecycle | No goroutine is added |
| Rule: no-layering | No second start path, no second release list |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Nine new `.ci` files promoted | `ls` of each path; none left under `test/draft/` |
| The name-keyed releases have one function | A grep of the server package for the alias, shape and capability release calls shows `releasePluginRegistrations` only |
| Every caller that sends to a process it looked up is classified | `gopls references` on `AllProcesses`, `GetProcess` and `ProcessServing`, each hit classified in the commit body of phase 5 as "sends: uses the running accessor", "reads non-running state on purpose" or "lifecycle owner" |
| `Respawn` has one caller | `gopls references` on `ProcessManager.Respawn` shows `restartPluginProcess` and tests |
| Journal rows true | A grep of each owned row's date in its class file |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-closed OPEN policy | No window in which a role-configured peer's check is skipped: retire, failed restart, restart window, bound passed |
| Resource exhaustion | A reload cannot restart a crash-looping plugin without bound: it never lifts the respawn limit, and each generation it starts counts against the bounds |
| Name reuse | A name released by the retire can be taken only by the same configured plugin's successor |
| Unverified config | No reload commits a change to config roots owned by a plugin that is down |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 broken | Stop. Report the path to the main thread; the retire stays out until that path waits for the cleanup |
| A-2 broken | The path found gets the restart-or-refuse step in phase 4, with its test |
| A-3 census finds an unsafe subscriber | Fix that subscriber in phase 6 |
| A-5 broken | The injector gains the replacement of a plugin's own kept declaration in phase 2 |
| Test fails on behavior mismatch | Re-read the producer in Current Behavior |
| 3 fix attempts failed | STOP. Report all three. Ask the user |

## Design Insights
- Six commits after the first design had already built the stop record, the doctor row, the operator restart, the restart controller and the atomic registration. What was left of defect 1 is narrower than the journal row says: the dead process and its name-keyed registrations.
- The fail-open of RFC 9234 does not wait for the retire. It exists today after a failed restart, where `rollbackReplacement` removes the slot and the capabilities, and in the window of every restart, where the replacement holds an empty registration until Stage 1.
- `RemoveProcess` leaves the name configured, so `Respawn` starts a retired plugin with no process to stop. That is why the reload can use the restart path instead of the start path.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/rollback-forgets-partial-apply.md` | 2026-08-21 | unrestarted exit keeps by-name registrations, loaded mark, slot |
| `plan/journal/plugin-startup-barrier-deadlock.md` | 2026-09-08 | replacement published before its handshake; senders read no stage |
| `plan/journal/false-synchronization-claim.md` | 2026-10-03 | process registration: made atomic by 4a0c0f468a, the `-race` test is still owed (AC-7) |
| `plan/journal/late-write-lands-on-the-successor.md` | 2026-09-09 | engine event dispatch calls a retired handler |
| `plan/journal/stop-leaves-the-work-running.md` | 2026-09-17 | plugins restarted between the stop signal and the drain |
| `plan/journal/guard-added-to-one-half-of-a-pair.md` | 2026-11-13 | `writeBatchWithDeadline` (`conn.go:661-664`) and `CallBatchRPC` (`mux.go:330`) already check `ctx.Err()` on HEAD; direct `Conn` regression tests in `conn_test.go` are still owed (AC-14) |
| `plan/journal/wait-outlives-the-process-it-waits-on.md` | 2026-11-13 | `startExternal` waits out the 30 s connect-back of a child that already exited (`waiter.Wait` does not watch `reaped`) |

## Rows moved to another spec (2026-11-19)
| Class file | Date | Surface (short) | Why not here | Home |
|------------|------|-----------------|--------------|------|
| `plan/journal/exit-cleanup-runs-after-its-transport-closed.md` | 2026-11-13 | in-process plugin's exit withdrawal refused after `Run` stops dispatch | It needs a design of its own: either the SDK orders exit cleanup before dispatch stops, or the engine withdraws what a generation announced. Its symptom is a FlowSpec rule left at the peers, the subject of the receiving spec | `plan/release/fixes/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md` |
| `plan/journal/guard-keyed-on-a-field-its-consumer-ignores.md` | 2026-11-14 | state-key grant and doctor snapshot keyed on the process name, so `use ddos-flowspec` under another name cannot reach its record | A question of which name identifies a plugin block, which the receiving spec redesigns (an endpoint's name against the registry name it runs) | `spec-explicit-plugin-and-attach-config` (closed; fixed in `9779726169`) |

## Rows considered and not owned
| Class file | Date | Surface (short) | Why not this root cause | Home |
|------------|------|-----------------|-------------------------|------|
| `plan/journal/plugin-startup-barrier-deadlock.md` | 2026-08-10 | firewall vpp backend waits for VPP in the config stage | A forwarding design decision about when "config applied" is true for a data-plane backend. The first design's AC-9, phase 6 and assumptions A-5 and A-6 went with it | `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md` |

## Rows found already fixed
| Class file | Date | Fixed by | Evidence | Action |
|------------|------|----------|----------|--------|
| `plan/journal/plugin-startup-barrier-deadlock.md` | 2026-08-19 | bde0241cf4 (2026-08-20), "fix(bgp): a borrowed plugin server is stopped only by its owner" | `test/plugin/shutdown-is-prompt.ci` exists; the process-protocol page, "Who stops the plugin server", states the rule. The `.ci` was not run by the author of this spec | The parent's journal pass marks the row fixed; this spec owns no work for it |
| `plan/journal/false-synchronization-claim.md` | 2026-10-03 | 4a0c0f468a (2026-11-17) | `Process.registration` is an `atomic.Pointer` (read 2026-11-19). No `-race` test of the concurrent read and write exists in the process package | The row stays in the owned table until phase 5 adds `TestRegistrationConcurrentReadWrite` and marks it fixed |
| `plan/journal/guard-added-to-one-half-of-a-pair.md` | 2026-11-13 | already landed on HEAD | `writeBatchWithDeadline` (`pkg/plugin/rpc/conn.go:661-664`) and `MuxConn.CallBatchRPC` (`pkg/plugin/rpc/mux.go:330`) both check `ctx.Err()` before writing (verified 2026-11-26); `mux_batch_test.go` covers `MuxConn`, while direct `Conn` regression tests are still owed | The row stays in the owned table until phase 8 adds `TestWriteBatchRefusesEndedContext` and `TestWriteBatchDeliversUnderLiveContext` in `pkg/plugin/rpc/conn_test.go` and marks it fixed |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Retire an ended generation with the steps three paths already run | Move the five name-keyed releases into the cleanup body, the first design | The cleanup also runs for a generation `Respawn` is replacing and for a late cleanup, where a release by name strips a successor; the retire steps already exist and wait for the cleanup first |
| Retire at all, rather than leave the dead process in its slot | Keep the slot, as today | A dead process answers for the name in the registry, the families and the reload's affected set, where it fails every transaction over its roots with a closed-connection error. Retired, every down path ends in one state (a failed restart and a failed handshake already leave no slot), and the reload answers by name with the fix |
| Record the stop after the slot removal | Record first | `RemoveProcess` clears the stop record and its bus warning; `restartReplacementWithin` already records after its rollback |
| D-A: keep the OPEN-policy declarations and a name record | The three alternatives in the decisions table | RFC 9234 Sections 4.1 and 4.2; one source for the peer selectors |
| D-B: a reload restarts down plugins through the restart path, and a failed start refuses it | The three alternatives in the decisions table | One start path; no config commits without its owner; the auto-load precedent |
| A down plugin an accepted reload removes loses its stop record and owed OPEN-policy record | Clear on stop before acceptance (loses records on rejected reload) or keep records after removal | `startup_removal.go` (`finishRemovalScope` / `unconfigureProcessless`) already unconfigures removed down plugins only on reload acceptance (`accepted == true`), preserving stop records on rejection; extending that same acceptance-scoped step clears the owed OPEN-policy record and kept declarations without losing fail-closed protection if the reload fails |
| Skip retire and refuse respawns on `stopUnderWay`; seal reload and phase admission in `BeginDrain` | Key everything on `s.ctx.Err()` (too late: after drain) or refuse reloads on `stopUnderWay` (too early: aborts fenced commit reloads) | `SignalShutdownRequested` runs before `awaitReloadWorker` and `fenceCommitReloads`, so gating retire and crash-triggered respawns on `stopUnderWay()` closes the fence + drain + teardown window without aborting already-admitted commit reloads, queued SIGHUP reloads, or confirmed-commit reverts; `BeginDrain` runs right after the config fence and seals post-fence reload and startup-phase admission |
| Read the stop fact at `restartPluginProcess` | A second hub call that refuses respawns early, the first design | The hub already calls `SignalShutdownRequested` first; `restartPluginProcess` is the one caller of `Respawn` |
| Retired mark checked before each call | `unsubscribe` waits for a running handler | A wait deadlocks a self-unsubscribing handler and a caller holding the handler's lock |
| One running-only accessor | A stage check at each sender | One place to forget instead of eight |

## Known Limitations
- Dependents of a plugin that went down keep running, as today. Stopping or restarting them is reload reconciliation, homed in `plan/release/fixes/spec-single-owner-reload-reconcile.md`.
- An engine-event handler already running when it is unsubscribed finishes (AC-6). Subscribers own that case.
- A session that was Established when its policy plugin went down stays up; its route filters answer through the filter's declared on-error, as today.

## RFC Documentation (Scope: protocol)

The code AC-11 touches carries the RFC 9234 Section 4.2 quote already above `unansweredOpenValidation` and `OpenValidationUnavailableError.NotifyCodes`. The kept declaration carries the Section 4.1 quote where the release keeps it.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 9234 Section 4.1 | A peer with a configured role: ze's OPEN carries the Role capability, also while the role plugin is down | None: the capability follows the configured role | `test/plugin/role-open-refused-validator-down.ci` asserts the capability in ze's OPEN | N-A |
| RFC 9234 Section 4.2 | A peer with a configured role whose roles ze cannot confirm is refused with NOTIFICATION code 2, subcode 11 | A peer with no role configured is not held to any OPEN policy and establishes | `TestOpenRefusedWhileValidatorDown`, both `role-open-refused-validator-down` files | `TestOpenPolicyOwedAfterValidatorRetired` asserts a peer the plugin declared nothing for is not named |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated (AC-9 moved)
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests and functional suites named in the TDD plan. GitHub CI verifies the pushed commit; a red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
