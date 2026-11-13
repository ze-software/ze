# Spec: single-owner-lifecycle-1-plugin-process

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | A plugin that exits and is not restarted stays down until the daemon restarts, a restarting plugin can be sent a runtime call in the middle of its handshake, and a plugin restarted during the stop is torn down again: shipped-daemon lifecycle defects an operator meets in production. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 1 of `plan/release/fixes/spec-single-owner-lifecycle.md`, which holds the
decision record (owner decisions D1, D2, D3 of 2026-11-11) and the routing of
every journal row to a child. This child owns the plugin-process lifecycle.

Principle, from the parent: a plugin generation already has an identity (the
process pointer) and an owner (the supervising goroutine with its run-once
cleanup). Each defect below is a path that goes round them. Each fix closes that
path at its choke point. No new owner object, no generation counter, no source
gate (owner decision D1: "D1 - yes you are right").

Five items: four defects, which between them own the rows of "Owned journal
rows" (defect 2 owns two), and one row found already fixed:

1. A generation that ends with no successor (failure policy `ignore`, an
   undeclared policy, a respawn refused, failed or over its bound) keeps its
   name-keyed registrations, its loaded mark and its manager slot, so nothing
   can start that plugin again while the daemon runs. A reload does not start
   it either, because a reload with no config difference returns before it
   starts anything.
2. `Respawn` publishes the replacement process in the manager before its
   handshake has run, and runtime senders pick processes without reading their
   stage, so a replacement can be sent a runtime call while it is still in the
   handshake. The registration those senders read is an unsynchronized field.
3. The engine event bus calls a handler that was unsubscribed after the dispatch
   took its list.
4. A plugin that exits between the stop signal and the stop drain is started
   again, then torn down.
5. One row is already fixed and only its journal state is stale.

A sixth item, the firewall with `backend vpp` waiting for VPP inside its config
stage, left this spec on 2026-11-11: it is a forwarding design decision, homed
in `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md` ("Rows
considered and not owned").

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/hub-engine.md`, `ai/doors/bgp-engine.md`, `ai/doors/forwarding.md`, `ai/doors/system-plugins.md`, `ai/doors/core.md`, `ai/doors/le-tooling.md`
  → Constraint: every plugin runs internal or forked; a lifecycle fix is proven in both modes (`ai/rules/testing.md`, "Lifecycle Regressions").
  → Constraint: one plugin failure stops the startup tier, by owner ruling; this spec does not make a tier survivable.
  → Constraint: the report bus is process-local (`ai/doors/forwarding.md`), so the down warning is raised by the engine, never by the plugin.
  → Constraint: the engine never names a plugin.
- [ ] `docs/architecture/api/process-protocol.md`, sections "Failure-Policy Declaration (Stage 1)", "Shutdown", "The stop drain", "Who stops the plugin server", "Panic containment"
  → Decision: `ignore` means "the engine logs and carries on without it"; a plugin past its restart bound is "the `ignore` outcome". This spec gives both the same end state.
  → Constraint: "A stop ends every restart. From the start of the stop drain (`BeginDrain`)". This spec moves the start of the refusal earlier and rewrites that sentence.
  → Constraint: "Panic containment" says "a reload starts it again". That sentence is false today (Current Behavior) and becomes true with AC-3.
  → Constraint: whole-daemon shutdown closes connections without `bye`; a live removal sends `bye removed` and is acknowledged. Unchanged.
- [ ] `docs/architecture/api/process-protocol.md`, "A failure during a reload means the plugin failed"
  → Constraint: the removal and reload-recovery paths own a failed removal's restart; this spec does not change them.
- [ ] `ai/rules/testing.md`, "Lifecycle Regressions"
  → Constraint: every scenario of that table the change reaches is proven by a `.ci`, in both modes; unreached scenarios are named with the reason (TDD plan, "Lifecycle scenarios").
- [ ] `docs/contributing/ze-go-style.md` (before the first Go edit)
  → Constraint: read in full by the implementing agent; the pre-write hook refuses a Go edit until then.

Not read by the author of this spec: `docs/architecture/plugin/plugin-system.md` and `ai/digests/plugin-transport.md`. The implementing agent reads both before phase 1 and records a conflict with this spec before coding.

### RFC Summaries (Scope: protocol)
N-A: Scope is plugin. One RFC consequence is in scope as a risk, R-3 (RFC 9234 OPEN policy while its plugin restarts).

**Key insights:**
- The process pointer is the generation. Comparing the manager's process for a name with the process in hand is the currency test.
- `cleanupProcess` runs once per generation through `RunRuntimeCleanupOnce`, and it already releases some things by name (cache consumer, installed routes, cleanup hooks).
- A replacement's handshake starts only after its predecessor's cleanup finished (A-1). That ordering is what makes a by-name release in the cleanup safe.
- A reload with an empty diff starts nothing, and auto-load reads only added roots.
- The stop signal and `BeginDrain` are separated by the reload fence.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/failure_policy.go` - `superviseProcess` is "the whole life of one plugin process": drain caller, `handleSingleProcessCommandsRPC`, then `applyFailurePolicy`. `applyFailurePolicy` returns at once when the server context ended, when a removal owns the process, or when the manager holds another process under the name. It reports `plugin-crash`, then restarts (`restart`), stops the daemon (`fatal`), or only logs (`ignore`, unspecified, unknown). A failed restart only logs "so it stays down".
- [ ] `internal/component/plugin/server/dispatch.go` - `handleSingleProcessCommandsRPC` defers `cleanupProcess`. `cleanupProcess` runs under `RunRuntimeCleanupOnce`: the command registry and pending requests by process pointer, subscriptions by pointer, then by name the cache consumer, `withdrawPluginRoutes` and `runProcessCleanupHooks`.
- [ ] `internal/component/plugin/server/restart.go` - `releasePluginRegistrations` waits on `WaitRuntimeCleanup` for a process that reached running (or runs `cleanupProcess` itself for one that did not), then releases by name: the `PluginRegistry` row, injected capabilities, families, pipe aliases, answer shapes. `restartHandshake` runs the five stages with no tier barrier, then starts `superviseProcess`.
- [ ] `internal/component/plugin/server/reload_tx.go` - `restartPlugin`: `Respawn`, then `releasePluginRegistrations` on the old process, then `restartHandshake` on the new one. A refusal (`respawnRefused`) returns before anything is released. Its two callers are `applyFailurePolicy` and the `CodeBroken` rollback (`restartPluginFn`).
- [ ] `internal/component/plugin/server/startup_removal.go` - `stopRemovedProcess` removes the manager slot when it still holds this process, stops and waits for the process, calls `releasePluginRegistrations`, then `unmarkPluginLoaded`. `rollbackStartupProcess` reaches it directly for a process below running, and through an acknowledged `bye` otherwise.
- [ ] `internal/component/plugin/server/startup.go` - `runPluginPhaseWithin` marks every plugin loaded, runs each tier's handshake, rolls back a process below running, and starts `superviseProcess` for each running one after the last tier. `unmarkPluginLoaded` clears the mark.
- [ ] `internal/component/plugin/server/reload.go` - `reloadConfig` returns "config reload: no changes" when the diff is empty, before any plugin start. With a diff it calls `autoLoadForNewConfigPaths` with the added keys only.
- [ ] `internal/component/plugin/server/startup_autoload.go` - `selectAutoLoad` selects owners of the added paths, skips a name with an explicit plugin block (`hasConfiguredPlugin`) and a name the manager still holds. Other selection sites also skip a name that is marked loaded.
- [ ] `internal/component/plugin/process/manager.go` - `Respawn` holds the manager lock, refuses when stopping, disabled, unknown or over a bound, stops and joins the old process, starts the new one and stores it under the name before returning. `RefuseRespawns` sets a flag nothing clears. `ReportCrash` raises `plugin-crash`. `plugin-down` is raised only at a respawn bound and cleared only by a successful `Respawn`. `AllProcesses` returns every slot whatever its stage.
- [ ] `internal/component/plugin/process/process.go` - `registration` is a plain pointer field, set by `NewProcess` to an empty registration, written by `SetRegistration` and read by `Registration` with no lock. `stage` is atomic. `RunRuntimeCleanupOnce` and `WaitRuntimeCleanup` are a once and a channel closed after the cleanup body returns.
- [ ] `internal/component/plugin/process/register.go` - registers the `plugins` health row over the `plugin-down` code.
- [ ] `internal/component/plugin/server/rpc_register.go` - `RegisterProcessCleanup` appends to a package list that `runProcessCleanupHooks` walks with the process name.
- [ ] `internal/component/plugin/server/engine_event.go` - `dispatch` copies the handlers under the read lock, releases it and calls each. Its comment states an unregistered handler "IS still called once more". `unregister` deletes the map entry and waits for nothing.
- [ ] `internal/component/plugin/server/drain.go` - `BeginDrain` calls `RefuseRespawns` and snapshots the drain callers.
- [ ] `cmd/ze/hub/main.go` - after the stop signal: the force-signal channel, `announceSignal`, the reload worker is closed and awaited, the confirm window stopped, commit reloads closed, and only then `BeginDrain`.
- [ ] `internal/component/bgp/server/validate.go` - `askOpenValidators` walks every manager slot, reads `WantsValidateOpen` from the registration and the connection, and sends `validate-open`. It reads no stage. A plugin not asked stays in `pending`, and `unansweredOpenValidation` then refuses the OPEN.
- [ ] `internal/component/bgp/server/events.go` - `addOpenRemovalObservers` and the delivery-order builder read the registration of manager slots; neither reads the stage at the lines read.
- [ ] `internal/component/plugin/server/server.go` - `DoctorCheckPlugins` and `PluginsWithPerPeerOpenPolicy` read the registration of every manager slot; neither reads the stage at the lines read.
- [ ] `internal/component/plugin/server/source_policy.go` - reads the registration of a named slot.
- [ ] `internal/component/plugin/server/resolver_cache.go` - `ResolverCacheOwners` reads the registration of every slot.
- [ ] `internal/component/bgp/reactor/policy_reload.go` - reads the registration of a named slot to find a replay claimant.

**Behavior to preserve:**
- A `CodeBroken` restart that is refused leaves the running plugin serving, with every registration.
- `fatal` stops the daemon. A removal owns its own recovery. A startup failure rolls back as today.
- Daemon stop releases nothing by name and sends no `bye`.
- The restart bounds (`RespawnLimit` in `RespawnWindow`, `MaxTotalRespawns`) and their counters.
- A handler may register and unregister from inside a dispatch without deadlock.
- An OPEN whose per-peer policy plugin did not answer is refused.
- `test/plugin/drain-respawn-refused.ci` and `test/plugin/drain-plugin-failure-stop.ci` keep their expectations.

**Behavior to change:**
- The four defects of the Task, as the acceptance criteria state them.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Run-once generation cleanup | `internal/component/plugin/process/process.go`, `RunRuntimeCleanupOnce`, `WaitRuntimeCleanup` | Runs the server's cleanup body once per process and lets a caller wait for it | Extends: the by-name release of `releasePluginRegistrations` moves into that body, so every generation end runs it |
| Generation supervisor | `internal/component/plugin/server/failure_policy.go`, `superviseProcess`, `applyFailurePolicy` | One goroutine per generation: serves it, then acts on its end | Extends: the "no successor" outcomes retire the slot and the loaded mark there, after the cleanup returned |
| Cleanup-hook list | `internal/component/plugin/server/rpc_register.go`, `RegisterProcessCleanup`, `runProcessCleanupHooks` | Other packages register a by-name cleanup run inside `cleanupProcess` | Does not apply: the five releases already live in the server package and its imports, so no hook is needed; the list stays for packages the server cannot import |
| Process pointer as identity | `ProcessManager.GetProcess`, compared in `applyFailurePolicy`, `stopRemovedProcess`, `stopDaemonOnRemovalFailure` | "Is this still the live generation of the name" | Extends: the retire step uses the same comparison, so it never removes a replacement's slot |
| Stop flags | `ProcessManager` fields `stopping` and `respawnRefused`, `RefuseRespawns`; `Server.BeginDrain` | `respawnRefused` makes `Respawn` answer `ErrDaemonStopping` from the drain on | Extends: the hub calls the refusal as the first action after the stop signal; `BeginDrain` keeps its call, which is then a no-op |
| Respawn limiter | `ProcessManager.Respawn`, fields `disabled`, `respawnTimes`, `totalRespawns` | Bounds restarts, raises `plugin-down` at a bound | Extends: a bound reached after an exit is one of the "no successor" outcomes. The counters are unchanged |
| Stage | `Process.Stage`, `Process.WaitForStage` | Atomic handshake stage per process | Extends: one manager accessor returns only running processes, and runtime senders use it |
| Crash and down reports | `ProcessManager.ReportCrash`, `plugin-down` in `Respawn`, the health row in the process package's `init` | `plugin-crash` on every unexpected exit; `plugin-down` only at a respawn bound | Extends: `plugin-down` is raised for every generation that ends with no successor and cleared when a generation of that name reaches running |
| Reload auto-load | `selectAutoLoad`, `startAutoLoaded` in the server's auto-load file | Starts owners of added config roots | Extends: a reload also starts every plugin the candidate tree needs that holds no slot and no loaded mark, through the same `startAutoLoaded` |
## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A plugin process ends: its connection closes or its in-process engine returns. No data format; the trigger is the return of `handleSingleProcessCommandsRPC`.
- `ze signal reload` or SIGHUP reaches `reloadConfig` with the candidate tree.
- SIGTERM, SIGINT or `request shutdown` reaches the hub's stop sequence.
- A peer OPEN reaches `askOpenValidators`.
- An engine event reaches the engine subscribers' `dispatch`.

### Transformation Path
1. Generation end: `superviseProcess` stops the drain caller; `handleSingleProcessCommandsRPC` has returned, so `cleanupProcess` ran. New: unless the server context ended, the cleanup body also runs the five by-name releases.
2. `applyFailurePolicy` reads the policy. `restart`: `restartPlugin`. New: every outcome that leaves the name with no live generation (ignore, unspecified, unknown, respawn refused for a reason other than the stop, respawn failed) calls one retire step: remove the slot if it still holds this process, clear the loaded mark, raise `plugin-down` with the cause.
3. Reload: after the removal-recovery retries and before the empty-diff return, `reloadConfig` computes the plugins the candidate tree needs that hold neither slot nor loaded mark, and starts them through `startAutoLoaded`. A reload whose only work is that start still runs the handshake and config delivery of `runPluginPhaseWithin`.
4. A started generation that reaches running clears `plugin-down` for its name.
5. Stop: the hub calls the respawn refusal first, then the existing sequence.
6. OPEN validation and the other runtime senders take their process list from the running-only accessor.
7. Event dispatch checks each subscription's retired mark immediately before calling it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | No message added or changed. A plugin in its handshake stops receiving `validate-open`, which the handshake refused anyway | Read `askOpenValidators`; the handshake refusal is the 2026-09-08 journal row, not re-read |
| Hub ↔ plugin server | One new call from the stop sequence into the server, beside `BeginDrain` | Read the hub's stop sequence |
### Changed-Wire Census

No wire changes. Source-supported reason: the edits are in-memory ordering and
storage. The process registration becomes an atomic pointer inside the engine;
the five releases move between two engine functions; the manager gains an
accessor and two report calls; the hub adds one in-process call. No field of
the public RPC types, no method in the plugin engine or callback YANG modules,
no JSON event and no config leaf is added, removed or renamed. The public
EventBus `Subscribe` keeps its signature; its contract is tightened (AC-6) and
documented. The `plugin-down` report code exists today and gains raise sites
only. Evidence for the claim: every symbol in "Files to Modify" was read at its
producer, and none of them encodes or decodes.

### Integration Points
- `Server.cleanupProcess` - gains the by-name releases, guarded by the server context.
- `Server.applyFailurePolicy` - gains the retire step.
- `Server.reloadConfig`, `startAutoLoaded` - gain the "needed but not loaded" start.
- `ProcessManager` - gains the running-only accessor and the down report pair beside `ReportCrash`.
- The hub stop sequence - gains the early refusal.
- The engine event subscribers - the handler entry gains a retired mark.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every release stays in the generation's own cleanup; the reload start reuses `runPluginPhaseWithin` |
| No unintended coupling (components stay isolated) | Yes | No new import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms table: no new owner object, counter or gate |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No encoding path is touched |
| Registration over hardcoding, outbound | Yes | No command, family or handler is added |
| Registration over hardcoding, inbound | Yes | No list learns a plugin name; the reload start derives its set from `registry.PluginsForConfigPaths` and the explicit plugin blocks, the sources boot uses |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No path lets a replacement process register a name before its predecessor's run-once cleanup completed | Read at the three sites that store a process under a name: `Respawn` (its only caller `restartPlugin` calls `releasePluginRegistrations`, which waits, before `restartHandshake`), `startConfigs` (reached through `runPluginPhaseWithin`, whose selection sites skip a name with a slot or a loaded mark; `stopRemovedProcess` clears the mark last, after the release), `AddProcess` (tests only) | A by-name release in the old cleanup would delete the replacement's registrations | `TestReplacementHandshakeWaitsForPredecessorCleanup`: a cleanup hook blocks the old generation's cleanup; the test asserts the replacement has sent no Stage 1 while it blocks, then that its registrations survive the release | unvalidated |
| A-2 | The retire step always runs after the generation's cleanup completed | `applyFailurePolicy` is called only by `superviseProcess`, after `handleSingleProcessCommandsRPC` returned | A reload could start a successor while the old cleanup still runs | Same test as A-1, driven through the ignore path | unvalidated |
| A-3 | No caller unsubscribes an engine event while holding a lock its handler takes, in a way that needs the handler to have finished | Not read: there are about 150 `Subscribe` call sites | AC-6's permitted survivor (an invocation already running) reaches state the caller just replaced | Phase 4 census of every unsubscribe call site whose handler reads state the caller replaces; each gets the ddos-local shape (a retired mark under the subscriber's own lock) or is shown safe | unvalidated |
| A-4 | The plugins that exit early on a guest stop do so because their process receives the stop signal itself | `firewall-domain` and `firewall-irr` declare `FailureRestart` and build their context from `sdk.SignalContext`, which returns a signal-bound context unless the host owns signals; the mode the guest ran is unknown | The window closes anyway (AC-8), but the row's cause stays unidentified | Phase 5 reproduction in the QEMU guest with `test/plugin/firewall-domain-group-update.ci` and `test/plugin/firewall-domain-group-clear.ci`, reading the daemon log between the signal and the drain | unvalidated |
| A-7 | `ze doctor` shows a `plugin-down` warning with a fix command | The process package registers a health row; how a row carries its fix was not read | The doctor row of the Integration Checklist needs a doctor check registered from the process package | Phase 2 reads `docs/architecture/doctor-and-health-checks.md` and the health probe | unvalidated |
| A-8 | A plugin with an explicit `plugin { }` block that is down is started by the reload step of AC-3 through the same phase boot uses | Not read: how boot turns explicit blocks into plugin configs | AC-3 covers auto-loaded plugins only | Phase 3 reads the boot selection of explicit blocks; `test/plugin/failure-policy-ignore-reload-external.ci` proves it | unvalidated |

A-5 and A-6 moved with the firewall vpp item to `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md`; the remaining ids are not renumbered.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The by-name release runs at daemon stop and unregisters families or capabilities while sessions still close | A changed Cease or a late encode error in `test/plugin/shutdown-is-prompt.ci` | The release is skipped once the server context ended (AC-2); a unit test pins the guard |
| R-2 | A plugin started by a reload after passing its restart bound crashes again and is never restarted | `plugin-down` returns right after the reload | Intended: the bound is over the daemon's life (the page's own words). The warning names it |
| R-3 | During a validator's restart window its name leaves the per-peer policy set, so an OPEN passes with no role check | `PluginsWithPerPeerOpenPolicy` builds its set from the registration of live slots; the rest of that function was not read | Phase 3 reads it in full. If the set can lose a restarting validator, the fix is in that producer: the set comes from what the peer's config attaches, and `TestOpenRefusedWhileValidatorRestarts` proves the OPEN is refused |
| R-4 | A dependent of a retired plugin keeps running against a provider that is gone | A dependent's errors after an `ignore` exit | Unchanged from today (the provider was already dead). Not widened here; see Known Limitations |
| R-5 | The reload start runs for a plugin the operator removed from the candidate | A removed plugin restarts on reload | The set is computed from the candidate tree, never from the previous slot list |
| R-6 | The early respawn refusal makes a `restart` plugin that crashes during a long config fence stay down for the rest of the stop | The drain round reports it failed | Intended: the stop is terminal; the drain already tolerates an absent caller |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A plugin loses its commands, families or capabilities while it runs (A-1 broken), a reload starts a plugin twice, or a stop hangs. All daemon-visible; no wire format is at risk |
| How is it reverted? | Each phase is one commit and reverts alone. No config or stored state changes |
| Who else touches this path? | `plan/release/fixes/spec-daemon-drain-before-stop.md` (ready, phase 3/6) lists the failure-policy, manager, process, server and hub files this spec edits. Its drain caller, `BeginDrain` and `RefuseRespawns` are in the tree today; its remaining phases are the protocol drains. This child lands first for the shared files: its phase 5 moves the refusal that the drain's `.ci` tests assert, so landing it before the BGP, OSPF and IS-IS drain phases means those are written against the final stop order. Neither spec blocks the other, so Depends stays `-`. The sibling `plan/release/fixes/spec-single-owner-reload-reconcile.md` works on reload. Decided 2026-11-11 by the main thread of the parent spec: the AC-3 step (a reload starts a needed plugin that is down) lives in this spec, because it is what makes freeing the slot useful to an operator and the process-protocol page already promises it. The sibling is still awaiting its own design pass and reads this spec when it gets one, so it designs around a reload that already starts such a plugin. That is an ordering of design, not a dependency of this spec, so Depends stays `-` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A plugin process with `failure-policy ignore` exits, then `ze signal reload` | → | `applyFailurePolicy` retire step, `reloadConfig` start of needed plugins | `test/plugin/failure-policy-ignore-reload.ci` |
| The same, plugin forked | → | same | `test/plugin/failure-policy-ignore-reload-external.ci` |
| `ze doctor` while that plugin is down | → | `ProcessManager` down report | `test/plugin/failure-policy-ignore-doctor.ci` |
| SIGTERM, then a `restart` plugin exits before the drain | → | early respawn refusal in the hub stop sequence | `test/plugin/stop-fence-respawn-refused.ci` |
| A peer OPEN while a validator is in its handshake | → | running-only accessor in `askOpenValidators` | `TestAskOpenValidatorsSkipsHandshakingProcess` (seam: the interleaving is not reproducible in a `.ci`) |
| An engine event after `unsubscribe` | → | retired mark in `dispatch` | `TestEngineEventUnsubscribedHandlerSkippedInSnapshot` (engine-internal bus, no operator entry point) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A running generation ends, the daemon is not stopping | Its cleanup releases the `PluginRegistry` row, injected capabilities, families, pipe aliases and answer shapes held under its name, exactly once, before any failure-policy action | A `CodeBroken` restart that is refused keeps every registration, because that generation has not ended |
| AC-2 | A generation ends because the daemon is stopping | The cleanup runs its pointer-keyed half, route withdrawal and hooks as today, and the daemon exits 0 | Every by-name registration stays until the process exits, because the daemon ends and a release during teardown could change what closing sessions send |
| AC-3 | A plugin's generation ended with no successor (policy `ignore`, unspecified or unknown; respawn declined by the `respawn` leaf; respawn failed; a restart bound passed), then a reload whose candidate still needs that plugin | The reload starts a new generation that completes the five stages, receives its config and answers its commands, and the reload reports success even when the config did not change | A plugin the candidate no longer needs stays down. A plugin with policy `fatal` is never in this state, because its end stops the daemon |
| AC-4 | The same generation end as AC-3 | The manager slot and the loaded mark of that name are free once `applyFailurePolicy` returns, `plugin-down` is raised naming the plugin and the cause, and `plugin-crash` is raised as today | The slot of a replacement another path already stored stays, by pointer comparison. The restart counters stay, because the bound is over the daemon's life |
| AC-5 | `ze doctor` while a plugin is down as in AC-4, and again after AC-3's reload | The first run reports the plugin by name with its cause and the command `ze signal reload`; the second run reports the plugins row healthy | A `plugin-crash` error stays listed until it ages out as today, because it records that an exit happened |
| AC-6 | `unsubscribe` returned for an engine-event subscription, then an event is emitted, or a dispatch that already took its list reaches that subscription | The other subscribers receive the event, and the retired subscription is skipped at the check made immediately before each call | An invocation of that handler already running when `unsubscribe` returned finishes, because waiting for it deadlocks a handler that unsubscribes itself and a caller that holds a lock the handler takes, and Go offers no goroutine identity to tell them apart. A subscriber that replaces state its handler reads guards it itself (A-3) |
| AC-7 | A runtime sender asks the manager for processes while a replacement is between `Respawn` and the end of its handshake | The sender receives only processes at the running stage, the replacement completes its handshake undisturbed, and `go test -race` over the concurrent read and write of the registration reports clean | An OPEN that needs a validator still in its handshake is refused as unanswered and the peer retries, because accepting it would skip the policy |
| AC-8 | The stop signal arrived, and a plugin with policy `restart` exits before the drain starts | The log shows `respawn refused: ze is stopping` for that plugin, the plugin stays down, and the stop completes with exit 0 | A restart whose `Respawn` call had already returned before the signal completes its handshake, because the stop sequence then closes it like any running plugin |
| AC-10 | The 2026-09-17 early exit of `firewall-domain` and `firewall-irr` | The spec records the reproduced cause with its log lines, or the attempts and the next step, and the journal row is marked fixed only with that reproduction passing after AC-8 | The row stays open when the cause is not reproduced, because a fix claimed without it is unproven |

AC-9 (the firewall vpp backend at boot) moved to `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md`; the remaining ids are not renumbered.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a plugin that crashes, then reloads to bring it back | exit -> cleanup release -> retire -> `ze signal reload` -> start of needed plugins -> five stages | `test/plugin/failure-policy-ignore-reload.ci`, `test/plugin/failure-policy-ignore-reload-external.ci` |
| 2 | Asks `ze doctor` why a feature is missing | retire -> `plugin-down` -> doctor row with fix | `test/plugin/failure-policy-ignore-doctor.ci` |
| 3 | Stops the daemon while a plugin is unstable | signal -> refusal -> fence -> drain -> teardown | `test/plugin/stop-fence-respawn-refused.ci` |
## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCleanupReleasesNameKeyedRegistrations` | `internal/component/plugin/server/restart_test.go` | AC-1: the five by-name registrations are gone after the cleanup body | |
| `TestCleanupRunsNameKeyedReleaseOnce` | `internal/component/plugin/server/restart_test.go` | AC-1: a second `cleanupProcess` and a later `releasePluginRegistrations` release nothing more | |
| `TestCleanupKeepsNameKeyedRegistrationsWhenServerStopping` | `internal/component/plugin/server/restart_test.go` | AC-2 | |
| `TestBrokenRestartRefusedKeepsRegistrations` | `internal/component/plugin/server/restart_refused_test.go` | AC-1 survivor | |
| `TestReplacementHandshakeWaitsForPredecessorCleanup` | `internal/component/plugin/server/restart_test.go` | A-1 and A-2 | |
| `TestIgnoredExitRetiresSlotAndLoadedMark` | `internal/component/plugin/server/failure_policy_test.go` | AC-4 for `ignore` | |
| `TestRefusedRespawnAfterExitRetiresSlot` | `internal/component/plugin/server/restart_refused_exited_test.go` | AC-4 for a bound and for a declined respawn | |
| `TestRetireLeavesReplacementSlot` | `internal/component/plugin/server/failure_policy_test.go` | AC-4 survivor | |
| `TestStoppingExitRetiresNothing` | `internal/component/plugin/server/failure_policy_test.go` | AC-2: `ErrDaemonStopping` and an ended server context skip the retire | |
| `TestReportDownRaisedAndCleared` | `internal/component/plugin/process/manager_test.go` | AC-4, AC-5: raise with cause, clear at running | |
| `TestReloadStartsNeededPluginThatIsDown` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-3 with an empty diff | |
| `TestReloadEmptyDiffStartsNothingWhenAllLoaded` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-3: the no-change reload still does nothing | |
| `TestReloadLeavesUnneededPluginDown` | `internal/component/plugin/server/reload_down_plugin_test.go` | AC-3 survivor, R-5 | |
| `TestRunningProcessesOmitsHandshakingReplacement` | `internal/component/plugin/process/manager_test.go` | AC-7 | |
| `TestRegistrationConcurrentReadWrite` | `internal/component/plugin/process/process_test.go` | AC-7, under `-race` | |
| `TestAskOpenValidatorsSkipsHandshakingProcess` | `internal/component/bgp/server/rfc9234_validate_test.go` | AC-7 | |
| `TestOpenRefusedWhileValidatorRestarts` | `internal/component/bgp/server/rfc9234_validate_test.go` | AC-7 survivor, R-3 | |
| `TestEngineEventUnsubscribedHandlerSkippedInSnapshot` | `internal/component/plugin/server/engine_event_test.go` | AC-6 | |
| `TestEngineEventSelfUnsubscribeReturns` | `internal/component/plugin/server/engine_event_test.go` | AC-6: a handler that unsubscribes itself returns | |
| `TestEngineEventOtherSubscribersStillCalled` | `internal/component/plugin/server/engine_event_test.go` | AC-6 positive half | |
| `TestStopRefusesRespawnBeforeFence` | `cmd/ze/hub/drain_test.go` | AC-8: the refusal precedes the reload fence | |
### Boundary Tests (numeric inputs)
N-A: the change takes no numeric input. The restart bounds are unchanged.

### Functional Tests
Each new `.ci` is drafted under `test/draft/<suite>/` and promoted (`/ze-test`).

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `failure-policy-ignore-reload` | `test/plugin/failure-policy-ignore-reload.ci` | An internal plugin declaring `ignore` exits; the log says ze continues without it; `ze signal reload` starts it; its command answers | |
| `failure-policy-ignore-reload-external` | `test/plugin/failure-policy-ignore-reload-external.ci` | The same with a forked plugin under an explicit plugin block | |
| `failure-policy-ignore-doctor` | `test/plugin/failure-policy-ignore-doctor.ci` | `ze doctor` names the down plugin and `ze signal reload`; after the reload the row is healthy | |
| `failure-policy-restart-bound-reload` | `test/plugin/failure-policy-restart-bound-reload.ci` | A plugin past its restart bound is down with `plugin-down`; a reload starts it | |
| `failure-policy-respawn-false-reload` | `test/plugin/failure-policy-respawn-false-reload.ci` | `respawn false` on a `restart` plugin: it stays down after its exit and a reload starts it | |
| `stop-fence-respawn-refused` | `test/plugin/stop-fence-respawn-refused.ci` | A `restart` plugin exits after SIGTERM and before the drain: refused, exit 0 | |
Existing tests re-run because their contract is reached (`ai/rules/testing.md`, "The Affected Population"): `test/plugin/failure-policy-restart.ci`, `test/plugin/failure-policy-fatal.ci`, `test/plugin/failure-policy-fatal-live-peer.ci`, `test/plugin/failure-policy-disagreement.ci`, `test/plugin/drain-respawn-refused.ci`, `test/plugin/drain-plugin-failure-stop.ci`, `test/plugin/filter-wire-plugin-respawn.ci`, `test/plugin/shutdown-is-prompt.ci`, `test/reload/tx-protocol-external-plugin.ci`, `test/reload/reload-plugin-only-no-change.ci`.

Commands: `./le test unit pkg path ./internal/component/plugin/server/`, `./le test unit pkg path ./internal/component/plugin/process/`, `./le test unit pkg path ./internal/component/bgp/server/`, `./le test unit pkg path ./cmd/ze/hub/`, `./le test functional plugin`, `./le test functional reload`. The main thread runs them, one at a time.

### Lifecycle scenarios (`ai/rules/testing.md`, "Lifecycle Regressions")
| Scenario | Reached? | Proof or reason |
|----------|----------|-----------------|
| Process ends after it declared: `restart`, `ignore`, `fatal` | Yes | `test/plugin/failure-policy-restart.ci`, `test/plugin/failure-policy-ignore-reload.ci` and its external twin, `test/plugin/failure-policy-fatal.ci` |
| A plugin fails a startup stage after it declared | Yes, through `releasePluginRegistrations` | `TestCleanupRunsNameKeyedReleaseOnce` and the existing startup-failure tests of the server package; the outcome is unchanged |
| A restarting plugin passes its restart bound | Yes | `test/plugin/failure-policy-restart-bound-reload.ci` |
| The `respawn` leaf disagrees | `false` on a `restart` plugin: yes, `test/plugin/failure-policy-respawn-false-reload.ci`. `true` on a non-restart plugin: not reached, the Stage 1 refusal is untouched (`test/plugin/failure-policy-disagreement.ci` re-run) |
| A plugin exits during the stop drain or teardown | Yes | `test/plugin/drain-respawn-refused.ci`, `test/plugin/stop-fence-respawn-refused.ci` |
| A signal during the stop | The fence state is reached | `test/plugin/stop-fence-respawn-refused.ci`; the drain and teardown states keep `test/plugin/drain-plugin-failure-stop.ci` |
| A reload or commit after shutdown began | Not reached | The commit gate is not edited |
| A plugin crashes during verify or apply, misses its deadline, or reports `broken` | Yes, through `restartPlugin` | `test/reload/tx-protocol-external-plugin.ci`, `TestBrokenRestartRefusedKeepsRegistrations` |
| A reload's context ends during a handshake, or a sibling fails at the barrier | Yes, through the rollback release | Existing reload-rollback tests of the server package, re-run; outcome unchanged |
| A plugin dispatches another plugin's command at startup | Not reached | No dispatch path is edited |

Seams: AC-7's interleaving (a sender between `Respawn` and Stage 5) and A-1's blocked cleanup are Go tests with a controlled seam, because a `.ci` cannot hold a process inside its handshake.

### Interop Tests (Scope: protocol)
N-A: no wire-visible protocol behavior changes. The OPEN refusal of AC-7 is the existing `unansweredOpenValidation` outcome.

## Files to Modify
- `internal/component/plugin/server/dispatch.go` - `cleanupProcess` gains the by-name releases behind the server-context guard
- `internal/component/plugin/server/restart.go` - `releasePluginRegistrations` keeps the wait-or-run and nothing else; its comment is rewritten
- `internal/component/plugin/server/failure_policy.go` - the retire step and its callers; comments on `applyFailurePolicy`
- `internal/component/plugin/server/reload_tx.go` - `restartPlugin` comment (step 2 becomes a wait)
- `internal/component/plugin/server/startup_removal.go` - `stopRemovedProcess` unchanged in order; comment only if it names the moved release
- `internal/component/plugin/server/reload.go` - the start of needed plugins that are down, before the empty-diff return
- `internal/component/plugin/server/startup_autoload.go` - the selection of needed plugins that hold no slot and no loaded mark
- `internal/component/plugin/server/engine_event.go` - retired mark; the "IS still called once more" comment is replaced
- `internal/component/plugin/server/drain.go` - `BeginDrain` comment; the refusal entry the hub calls
- `internal/component/plugin/server/server.go` - runtime readers move to the running-only accessor where they send or decide at runtime
- `internal/component/plugin/server/source_policy.go` - same review
- `internal/component/plugin/server/resolver_cache.go` - same review
- `internal/component/plugin/process/manager.go` - running-only accessor, down report pair, comments on `Respawn`
- `internal/component/plugin/process/process.go` - atomic registration
- `internal/component/plugin/process/register.go` - doctor fix for the plugins row, pending A-7
- `internal/component/bgp/server/validate.go` - `askOpenValidators` uses the accessor
- `internal/component/bgp/server/events.go` - `addOpenRemovalObservers` uses the accessor
- `internal/component/bgp/reactor/policy_reload.go` - reader reviewed; changed only if it sends to the process
- `cmd/ze/hub/main.go` - early refusal in the stop sequence- `docs/architecture/api/process-protocol.md` - "Failure-Policy Declaration" (ignore outcome, the stop sentence), "Panic containment", "The stop drain", the event-bus unsubscribe contract
- `docs/guide/health-checks.md` - the `plugin-down` row
- `plan/journal/rollback-forgets-partial-apply.md`, `plan/journal/plugin-startup-barrier-deadlock.md`, `plan/journal/false-synchronization-claim.md`, `plan/journal/late-write-lands-on-the-successor.md`, `plan/journal/stop-leaves-the-work-running.md` - each owned row marked as its phase lands

## Files to Create
- `internal/component/plugin/server/reload_down_plugin_test.go`
- `test/plugin/failure-policy-ignore-reload.ci`
- `test/plugin/failure-policy-ignore-reload-external.ci`
- `test/plugin/failure-policy-ignore-doctor.ci`
- `test/plugin/failure-policy-restart-bound-reload.ci`
- `test/plugin/failure-policy-respawn-false-reload.ci`
- `test/plugin/stop-fence-respawn-refused.ci`
### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No RPC and no leaf is added |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command; `ze signal reload` exists |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC; the lifecycle `.ci` tests are in the TDD plan |
| Pipe completeness | N-A | No output path |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | Yes | Problem class "a plugin is down and was not restarted": code `plugin-down`, raised by `ProcessManager` at the retire step with the cause (policy, declined, bound, failed start), fix `ze signal reload`, cleared when a generation of the name reaches running; the process package owns the row (A-7 decides health row or doctor check) |
| Prometheus counters/metrics | No | `ze_plugin_status` and `ze_plugin_restarts_total` keep their meaning; the retired plugin's status label is deleted as the bound path does today |
| BGP family surface | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or scope changed? | No | A defect fix; `./le feature report` is read in phase 2 to confirm no feature page claims the old behavior |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md` (`plugin-down`), `docs/guide/plugins.md` (what `ignore` leaves behind and how to bring the plugin back) |
| 7 | Wire format changed? | No | Census |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`: the four sections named in Files to Modify; `docs/plugin-development/handlers.md` if it states the unsubscribe contract (grep in phase 4) |
| 9 | RFC behavior implemented, changed, or newly proven? | No | R-3 may add a proof for RFC 9234; if it does, the row is answered in phase 3 through `/ze-rfc` |
| 10 | Test infrastructure changed? | No | The failure-policy fixture gains an `ignore` mode; `docs/functional-tests.md` only if it lists fixture modes (grep in phase 2) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `ai/digests/plugin-transport.md` if it describes generation end or `Respawn` (read in phase 1) |
| 13 | Route metadata keys? | No | - |
| 14 | Prometheus counters? | No | - |
| 15 | Registered plugin, event type, command or inventory changed? | No | - |
| 16 | Changed source file referenced by doc source anchors? | Yes | Derived per phase with `./le spec citation anchors spec plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md`; the process-protocol page declares the failure-policy, restart, removal and engine-event files |
| 17 | Existing docs show examples for this area? | Yes | `docs/guide/plugins.md` failure-policy examples checked against the new end state |

Design pages declared by the `// Design:` header of a file this spec edits, beyond the process-protocol page. None was read by the author of this spec; each disposition is an expectation the named phase confirms against the page before its commit, and a page that describes the changed behavior is edited in that commit.

| Page | Declared by | Expected disposition | Confirmed in |
|------|-------------|----------------------|--------------|
| `docs/architecture/config/transaction-protocol.md` | the reload transaction file | "Failure Modes" may state what a `broken` restart releases; edited if it names the by-name release as a restart step | Phase 2 |
| `docs/architecture/core-design.md` | the reload files | Unaffected unless section 18 states that a reload with no difference does nothing at all; then one sentence is added for AC-3 | Phase 3 |
| `docs/architecture/hub-architecture.md` | the hub entry file | "A signal during the stop follows one table" gains the respawn refusal as the first step of the stop | Phase 5 || `docs/architecture/resolve.md` | the resolver-cache owner file | Unaffected: the owner list only stops naming a process that is still in its handshake | Phase 3 |

## Implementation Steps

Each phase is one commit, lands when its scoped tests pass, and edits its pages in the same commit. No phase runs a whole-tree check; evidence is the scoped commands in the TDD plan, run by the main thread.

1. **Phase: Wiring (MANDATORY FIRST)** -- the failing end-to-end test
   - Inputs: the fixture `test/plugin/failure-policy-restart.ci` uses (find it by its log line `FIXTURE: failure-restart generation 1 is exiting`); the plugin-system page and the plugin-transport digest read first.
   - Tests: `test/plugin/failure-policy-ignore-reload.ci` drafted with an `ignore` mode of that fixture; it fails at "reload starts it".
   - Files: the fixture, the draft `.ci`.
   - Verify: the test is red for the stated reason, with the log pasted.
2. **Phase: every generation end releases and retires** -- AC-1, AC-2, AC-4, AC-5
   - Constraint: the by-name release is skipped once the server context ended. The retire order is cleanup complete, slot removed by pointer comparison, loaded mark cleared, `plugin-down` raised. `ErrDaemonStopping` retires nothing.
   - Tests: the unit rows for AC-1, AC-2, AC-4 and `TestReportDownRaisedAndCleared`; `test/plugin/failure-policy-ignore-doctor.ci`.
   - Files: the cleanup, release, failure-policy and restart-plugin functions; the manager and the process package's registration; the process-protocol and health-checks pages.
   - Verify: A-1, A-2 and A-7 moved to confirmed or broken with evidence. A broken A-1 stops the phase and goes back to the main thread.
3. **Phase: a reload starts a needed plugin that is down; runtime senders see running processes only** -- AC-3, AC-7
   - Inputs: read how boot selects explicit plugin blocks (A-8) and `PluginsWithPerPeerOpenPolicy` in full (R-3). The AC-3 step lives in this spec (decided 2026-11-11, Blast Radius): `plan/release/fixes/spec-single-owner-reload-reconcile.md` is still awaiting its own design pass and reads this spec when it gets one, so no report to it is owed before this phase.
   - Tests: the three reload unit rows, the two manager and process rows, the two OPEN-validation rows; the four reload `.ci` files; the wiring test of phase 1 goes green and is promoted.
   - Files: the reload and auto-load functions; the process registration and manager accessor; the OPEN validation and its observers; the server's runtime readers; the process-protocol page ("Panic containment").
   - Verify: journal rows rollback 2026-08-21, barrier 2026-09-08 and false-synchronization 2026-10-03 marked fixed in this commit.
4. **Phase: engine event unsubscribe** -- AC-6
   - Inputs: the census of A-3, recorded in this spec's Design Insights.
   - Tests: the three engine-event unit rows.
   - Files: the engine event subscribers, the process-protocol page, and each subscriber the census finds unsafe.
   - Verify: journal row late-write 2026-09-09 marked fixed.
5. **Phase: the stop refuses respawns first** -- AC-8, AC-10
   - Inputs: the QEMU-guest reproduction of A-4, before the edit, with the log kept in the commit body.
   - Tests: `TestStopRefusesRespawnBeforeFence`, `test/plugin/stop-fence-respawn-refused.ci`, the two drain `.ci` re-runs.
   - Files: the hub stop sequence, the server's drain file, the process-protocol page ("A stop ends every restart").
   - Verify: journal row stop 2026-09-17 marked fixed only when AC-10's reproduction passed; otherwise it stays open with the attempt added.

Phase 6 (the firewall vpp backend) moved with AC-9 to `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named symbol |
| Feature completeness | Each user story passes in internal and forked mode |
| Correctness | The by-name release cannot run after a replacement's Stage 1; the retire step compares pointers; the reload set comes from the candidate tree |
| Naming | Log lines keep `plugin` as the key; the down cause is one of four fixed words |
| Data flow | No engine or config code names a plugin |
| Operator errors and doctor codes | `plugin-down` carries the cause and `ze signal reload`; every new `Warn` names it |
| Rule: stale-comments | The comments on `releasePluginRegistrations`, `restartPlugin`, `applyFailurePolicy`, `dispatch`, `BeginDrain`, `RefuseRespawns` and `Respawn` describe the new behavior |
| Rule: goroutine-lifecycle | No goroutine is added |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Six new `.ci` files promoted | `ls` of each path; none left under `test/draft/` |
| The five by-name releases have one call site | A grep of the server package for the alias, shape and capability release calls shows the cleanup body only |
| No runtime sender reads every slot for a send | A grep for `AllProcesses()` reviewed line by line, each hit classified in the commit body |
| Journal rows true | A grep of each owned row's date in its class file |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-closed OPEN policy | R-3: no window in which a peer's role check is skipped because its plugin restarts |
| Resource exhaustion | A reload cannot be used to restart a crash-looping plugin without bound: each generation still counts against the restart bounds once it restarts |
| Name reuse | A name released by the cleanup can be taken only by the same plugin's successor or by a plugin the config names |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 broken | Stop. Report the path to the main thread; the release stays where it is until that path waits |
| A-3 census finds an unsafe subscriber | Fix that subscriber in phase 4 || A-8 broken | AC-3 covers auto-loaded plugins; the explicit-block start goes back to DESIGN |
| Test fails on behavior mismatch | Re-read the producer in Current Behavior |
| 3 fix attempts failed | STOP. Report all three. Ask the user |

## Design Insights
- `cleanupProcess` was never purely pointer-keyed: the cache consumer, installed routes and hooks already leave by name. The split that ccc3f86b04 made is between "leaves at every end" and "leaves at restart, removal and rollback", and it is the second group that the ignore path forgets.
- The page promises "a reload starts it again" and two producers prevent it: the empty-diff return and the added-roots-only auto-load.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/rollback-forgets-partial-apply.md` | 2026-08-21 | unrestarted exit keeps by-name registrations, loaded mark, slot |
| `plan/journal/plugin-startup-barrier-deadlock.md` | 2026-09-08 | replacement published before its handshake; senders read no stage |
| `plan/journal/false-synchronization-claim.md` | 2026-10-03 | process registration unsynchronized |
| `plan/journal/late-write-lands-on-the-successor.md` | 2026-09-09 | engine event dispatch calls a retired handler |
| `plan/journal/stop-leaves-the-work-running.md` | 2026-09-17 | plugins restarted between the stop signal and the drain |
| `plan/journal/exit-cleanup-runs-after-its-transport-closed.md` | 2026-11-13 | in-process plugin's exit withdrawal refused after `Run` stops dispatch |
| `plan/journal/guard-added-to-one-half-of-a-pair.md` | 2026-11-13 | batch drained after `Stop` cancels is written, then reported failed (`writeBatchWithDeadline` skips the `ctx.Err()` check) |

## Rows considered and not owned
| Class file | Date | Surface (short) | Why not this root cause | Home |
|------------|------|-----------------|-------------------------|------|
| `plan/journal/plugin-startup-barrier-deadlock.md` | 2026-08-10 | firewall vpp backend waits for VPP in the config stage | Not a plugin-process lifecycle defect any more: the firewall now declares the ordering edge to `vpp`, and the residual (the apply waits for the VPP connection inside the config stage) decides when "config applied" is true for a data-plane backend, a forwarding design decision. This spec's AC-9, phase 6 and assumptions A-5 and A-6 went with it | `plan/release/fixes/spec-firewall-vpp-apply-before-connect.md` |

## Rows found already fixed
| Class file | Date | Fixed by | Evidence | Action |
|------------|------|----------|----------|--------|
| `plan/journal/plugin-startup-barrier-deadlock.md` | 2026-08-19 | bde0241cf4 (2026-08-20), "fix(bgp): a borrowed plugin server is stopped only by its owner" | The commit changes the reactor and the process manager and adds `test/plugin/shutdown-is-prompt.ci`, which exists; `ProcessManager.Stop` carries the re-entry guard and names the reactor fix; the process-protocol page, "Who stops the plugin server", states the rule. The `.ci` was not run by the author of this spec | The parent's journal pass marks the row fixed; this spec owns no work for it |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Move the five by-name calls into the cleanup body | A per-process list of recorded release closures | The calls are keyed by the name alone and are already safe for a process that registered nothing (the rollback path calls them so today). A list adds state with one user |
| Skip the by-name release when the server context ended | Release at stop too | The owner's survivor: daemon stop releases nothing by name. It keeps the stop identical to today |
| Retire in `applyFailurePolicy` | Retire inside the cleanup | The cleanup also runs for a generation `Respawn` is replacing; only the supervisor knows there is no successor |
| Reload starts needed plugins that are down, even with an empty diff | Retire only, as the approved design sentence says | Read at `reloadConfig`: retiring alone changes nothing an operator can reach, so the page's promise and the wiring test would stay false |
| Retired mark checked before each call | `unsubscribe` waits for a running handler | A wait deadlocks a self-unsubscribing handler and a caller holding the handler's lock; the code's own contract allows unregister from inside a dispatch |
| One running-only accessor | A stage check at each sender | One place to forget instead of nine |
| Refusal at the stop signal | Keep it in `BeginDrain` | The fence between them awaits reloads and can be long; a restart there is torn down again |
## Known Limitations
- Dependents of a plugin that went down under `ignore` keep running, as today. Stopping or restarting them is reload reconciliation, homed in `plan/release/fixes/spec-single-owner-reload-reconcile.md`.
- An engine-event handler already running when it is unsubscribed finishes (AC-6). Subscribers own that case.

## RFC Documentation (Scope: protocol)
N-A unless R-3 produces a code change, which then carries the RFC 9234 Section 4.2 quote already above `unansweredOpenValidation`.

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests and functional suites named in the TDD plan. GitHub CI verifies the pushed commit; a red CI reports gets a fix commit
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
