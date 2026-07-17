# Spec: fixit-reject-fence-observability-deferred-external-plugin-signals

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | ~~spec-fixit-plugin-event-subscription~~ none (2026-07-17: the resolved QUERYABLE-STATE design emits no event, so it needs neither the event namespace nor that spec landing; see "## Resolved Design (2026-07-17): QUERYABLE STATE") |
| Phase | FINAL (2026-07-18): await=stderr runner primitive; convert as112 + cos to it; ratchet 132->130. Engine `exited`/observer design REVERTED -- see "## Final Design (2026-07-18): await=stderr FENCE" |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/process/process.go:791` - `monitorCmd`, the exit-detection site that emits nothing
4. `internal/component/plugin/server/dispatch.go:135` - `registerSubscriptions`, the namespace choke point
5. `internal/component/plugin/resolve.go:116` - `RegisterPluginEventTypes`, the declaration-side producer of the same namespace bug
6. `plan/spec-fixit-plugin-event-subscription.md` - risk R-6, which predicts this spec

## Task

Give `test/plugin/as112-external-refuses.ci` and `test/plugin/cos-external-warns.ci` a
queryable signal to fence on, so they can drop their `time.sleep(4.0)` blind holds
(`as112-external-refuses.ci:79`, `cos-external-warns.ci:80`) without losing their stderr
proofs. Ratchet `test/.ci-sleep-baseline` 132 -> 130.

**Provenance:** deferred from `plan/spec-fixit-reject-fence-observability.md` (its AC-3b,
Phase 2), ruled by Thomas 2026-07-16: spec it separately rather than fold it into
`spec-fixit-plugin-event-subscription`. That spec is at `design` and owns its own gaps;
loading this surface into it couples two things that fail independently. The source spec
closes at Phase 1 and its AC-3b is retired into this file.

### The source spec's name for this work is wrong, and the correction is the design

`spec-fixit-reject-fence-observability` calls the missing thing "the external-plugin-exit
signal" (`:174`, `:185-191`). Re-reading the two tests shows that is one signal too few.
**They do not want the same event:**

| Test | What it waits for | Does the plugin exit? | Evidence |
|------|-------------------|----------------------|----------|
| `as112-external-refuses.ci` | the subprocess starts, logs its ERROR refusal, and **exits** | YES | `:85` `expect=stderr:contains=refusing to start as an external plugin process` |
| `cos-external-warns.ci` | the subprocess starts and **emits a WARN**, then **keeps running** | NO | `:74-76` docstring says cos keeps running after the warning; the test tears the daemon down via the sleep timeout "rather than waiting for cos's own exit" |

So an exit event fences as112 and can never fence cos. cos needs a startup-reached /
warn-emitted signal. Designing one event and discovering this at implementation time is the
failure this table exists to prevent.

### Both tests need a second thing, independent of the signal

Neither test has an observer plugin. `wait.py` runs as a bare `cmd=foreground` process
(`as112-external-refuses.ci:83`, `cos-external-warns.ci:84`) with no `ze_api` import, no
plugin socket, and therefore nothing that could poll or receive a signal even if one
existed (`spec-fixit-reject-fence-observability.md:224-227`). Each test needs an observer
plugin added. This is half the work and is not blocked on anything.

### Why the Phase 1 counter cannot serve either test

Recorded so nobody re-litigates it: the refuse/warn fires during plugin **startup**, and
neither test performs a reload at all, so the reload-generation counter
(`internal/component/plugin/server/reload_generation.go`) can never fence them
(`spec-fixit-reject-fence-observability.md:222-224`, risk A-1).

### The dependency, and the trap inside it

`Depends | spec-fixit-plugin-event-subscription` (`Status | design` as of 2026-07-16, so
this spec waits on something not yet built).

That spec's **risk R-6** (`:454-475`) is the deconfliction this work was blocked on, and it
already did the analysis: an `external-plugin-exited` event declared via
`Registration.EventTypes` gets registered by `RegisterPluginEventTypes`
(`resolve.go:116-135`), which reads the single global default namespace at `resolve.go:121`
-- `bgp`, and only `bgp` (`RegisterDefaultEventNamespace` is called exactly once, at
`internal/component/bgp/plugin/register.go:64`). A plugin-host lifecycle event landing in
the `bgp` namespace is "wrong on its face ... and invisible to anyone subscribing to a sane
namespace" (`:456-462`).

R-6 says the collision "has not materialized" because the concurrent Phase 1 work took the
counter option and declared no event type (`:466-475`). **This spec is the next plugin R-6
predicts will hit that bug silently.** Re-check R-6 at Phase 0 before implementing (`:475`).

> **The dependency does not fully cover us.** `spec-fixit-plugin-event-subscription`'s Gaps
> A/B/C thread a namespace through `registerSubscriptions` (`dispatch.go:148`) -- the
> **subscribe** side. `resolve.go:121` is the **declaration** side, and the spec names these
> as two distinct producers of the same bug (`:461-462`). Landing that spec fixes the half
> we need for the observer to subscribe, but this spec must still either extend
> `resolve.go:121` itself or knowingly ship the event under `bgp` as debt. Decide that in
> design, not in code review.

## Resolved Design (2026-07-17): QUERYABLE STATE

`→ AUTONOMOUS DEFAULT (2026-07-17) [STAKES: arch]:` the A-1 event-vs-state fork and the
namespace sub-question are RESOLVED to **queryable state** (option (iii) of the Phase 0
"For Thomas" list), authorized by Thomas 2026-07-17. Thomas: override if you want the
pub/sub event surface instead.

**What is built (corrected 2026-07-18 by the implementation audit -- see "## Audit
Corrections"):** the signal both tests need is read through the existing
`show system subsystem list` command (`handleShowSystemSubsystemList`,
`internal/component/cmd/show/system.go:71-98`), which returns, per plugin, `{name, stage:
p.Stage().String(), running: p.Running(), command-count}` from `pm.AllProcesses()`
(`manager.go:271`). An exited external plugin stays enumerable (its `Process` is not removed
by `monitorCmd`; `RemoveProcess` at `manager.go:254` is only called from reload/autoload
paths -- `startup.go:439`, `startup_autoload.go:319,426` -- never from `monitorCmd`), so its
post-exit state is queryable. Two things the original 2026-07-17 sketch got wrong, both
grounded in source below:

1. **An observer plugin IS required.** Dispatching any command needs a plugin-socket
   connection: `API()` reads `ZE_PLUGIN_HUB_TOKEN` (TLS) or the engine FDs (`ze_api.py:103-112`),
   and those are only handed to a subprocess the daemon spawns as a plugin (`process.go:578-584`).
   The current pollers run as a bare `cmd=foreground:exec=python3 wait.py` (`as112:83`, `cos:84`)
   with none of that env, so they cannot dispatch. Each test must register a small observer
   plugin (like the sibling `reload-listener-rejected.ci:142-145,191-193`) that completes the
   handshake and then polls via `api.dispatch_until(...)`.
2. **as112 needs a monotonic `exited` marker, not `running==false`.** `running` transitions
   false->true->false (`process.go:598` then `:795`), so `running==false` also matches the
   never-started state. Worse, as112 refuses at the very top of `RunEngine`
   (`as112/register.go:223-225`), immediately after the TLS connect-back
   (`cmd_plugin_external.go:52-58`) and before any handshake, so it exits within milliseconds
   and never reaches `StageRunning`; the transient `running==true` window is un-catchable by an
   observer still completing its own handshake. The fence therefore needs a marker that, once
   set, stays set: a new `Process.exited` (`atomic.Bool`, set in `monitorCmd`) surfaced as an
   `exited` field in `show system subsystem list`.

| Test | Fence predicate on `show system subsystem list` | Grounded by |
|------|--------------------------------------------------|-------------|
| `as112-external-refuses.ci` | the `as112` entry reaches `exited==true` (it started, refused, exited -- monotonic; never true for never-started) | `internal/plugins/as112/register.go:223-225` (`log.Error` + `return 1` when `!IsInternal`, before any handshake); new `Process.exited` set in `monitorCmd` (`process.go:791-806`) |
| `cos-external-warns.ci` | the `cos` entry reaches `stage=="Running"` and stays `running==true` | `internal/plugins/cos/register.go:145` (`warnIfExternal` WARN, no return; handshake continues at `:156-174`); engine drives `StageReady->StageRunning` (`startup_driver.go:224`) |

**Why this over an event (resolves R-1, A-1, A-3):** a queryable-state poll needs **no**
non-`bgp` event namespace (dissolving the namespace fork that had no safe default), does
**not** make plugin-host lifecycle a BGP concern (nothing rides `deliverEvent`/the `bgp`
namespace, so the Architectural Verification "no unintended coupling" item holds), and
mirrors how the parent `spec-fixit-plugin-event-subscription` resolved its own R-6 coupling
(counter/queryable state, not a new event type). `deliverEvent` (`dispatch.go`),
`RegisterPluginEventTypes` (`resolve.go:121`), and a new event type are therefore **NOT**
touched. Files to Modify (corrected): the two `.ci` (add an observer plugin, poll, drop
`time.sleep(4.0)`), a new `Process.exited` marker in `process.go`, its `exited` field in
`cmd/show/system.go`, two unit tests, and the `.ci-sleep-baseline` ratchet.

This supersedes the Phase 0 "Residual HARD BLOCKER" conclusion and its "For Thomas (pick
one)" list below: option (iii) is chosen. It also supersedes the 2026-07-17 "no observer
plugin" / "running==false" sketch; see "## Audit Corrections (2026-07-18)".

## Audit Corrections (2026-07-18)

The `/ze-implement` audit (step 3) validated the Resolved Design's assumptions against
source before any code. Two supporting claims in the 2026-07-17 sketch were **broken**; the
core decision (queryable state via the existing `show system subsystem list`, no event, no
namespace) is unaffected. Recorded here as the Mistake Log for this spec.

### Mistake Log

| # | Wrong claim (2026-07-17) | Source truth | Consequence | Correction |
|---|--------------------------|--------------|-------------|------------|
| 1 | "needs **no observer plugin** added to either test" (old `:115`, `:171`); each `wait.py` just polls | Dispatching a command requires a plugin-socket connection: `API()` needs `ZE_PLUGIN_HUB_TOKEN`/engine FDs (`ze_api.py:103-112`), set only for daemon-spawned plugins (`process.go:578-584`). The current `wait.py` is a bare `cmd=foreground` (`as112:83`, `cos:84`) with no socket. | A bare poller cannot dispatch; the design as sketched is unbuildable. | Each test registers a small observer plugin that does the handshake then polls (mirror `reload-listener-rejected.ci:142-145,191-193,81-87`). The spec's own Files to Modify (`add an observer plugin`) was already correct; the Resolved Design section contradicted it. |
| 2 | as112 fences on `running==false` | `running` is false both before start and after exit (false->true->false, `process.go:598`,`:795`). as112 refuses at the top of `RunEngine` before any handshake (`as112/register.go:223-225`, `cmd_plugin_external.go:52-58`), exiting in ms, so the `running==true` window is un-catchable. | `running==false` passes vacuously against a never-started plugin; the fence is not real. | Add a monotonic `Process.exited` (`atomic.Bool`, set in `monitorCmd`), surface as an `exited` field, fence as112 on `exited==true`. This is the "exit-stage stamp" the spec's Known Limitations already anticipated. |
| 3 | The queryable command is `show system subsystem list` -> `handleShowSystemSubsystemList` (`cmd/show/system.go`) | The dispatchable command the observer uses is `system subsystem list` -> `handleSystemSubsystemList` (`internal/component/plugin/server/system.go:271`), proven by the existing `test/plugin/subsystem-list.ci` and `docs/architecture/api/commands.md:419`. `cmd/show/system.go` is a **parallel** handler (`show system subsystem list`, `ze-show:system-subsystem-list`) with an identical body. | Adding `exited` only to `cmd/show` leaves the command the observer actually calls without the field. | Add `exited` to **both** handlers (they mirror each other and must not diverge; `plugin/server` cannot import `cmd/show`, so no shared helper). Observer dispatches `system subsystem list`. The server handler is covered end-to-end by the converted `.ci`; the `cmd/show` mapping is unit-tested via the extracted `subsystemEntry` helper. |

Corrections 1-3 do not change the ratchet or the no-event/no-namespace decision, but they
were overtaken by a runtime discovery (#4) that changed the whole approach.

| 4 | as112 can be fenced by an in-daemon observer plugin polling `exited` | An external plugin that refuses to start IS a plugin-startup failure. `StartupCoordinator.PluginFailed` (`internal/component/plugin/startup_coordinator.go:147-164`) "aborts the ENTIRE startup process": every co-located plugin in the same startup phase gets `errStartupBarrierAborted`. Empirically, an `as112-observer` co-plugin timed out at stage Config ("plugin 0 (as112) failed: startup incomplete"), and the daemon does not exit on the failure (`startup.go:116-118` logs + returns, keeps running; no later phase runs). | The observer-poll design is **impossible** for the reject case: the plugin under test kills the observer via the startup barrier. The whole reject-fence bucket (as112, trafficusage, flowexport) shares this. | Fence on the daemon's relayed stderr line with a new **`await=stderr:contains=`** runner primitive (`internal/test/runner/await_stderr.go`). It needs no in-daemon plugin, so it works for the reject case AND (uniformly) the warn case (cos). The `exited` marker + observer + `show`-handler changes are **REVERTED** as unused; no engine change ships. Chosen by Thomas 2026-07-18. |

## Final Design (2026-07-18): await=stderr FENCE

**Authoritative.** Supersedes the queryable-state/observer/`exited` design in "## Resolved
Design" and Audit Corrections 1-3 above (retained for the record). Chosen by Thomas after
the startup-barrier discovery (Mistake Log #4).

**What is built:** one test-infra primitive, `await=stderr:contains=<text>[:timeout=<dur>]`
(`internal/test/runner/await_stderr.go`, wired in `runner_exec.go`/`record.go`/`record_parse.go`,
using the existing `syncWriter`). It blocks the runner until the daemon's relayed stderr
carries `<text>`, then tears the daemon down -- a deterministic replacement for the blind
`time.sleep(4.0)`. **No engine change**: `Process`, both `system subsystem list` handlers,
`deliverEvent`, and the event namespace are all untouched.

| Test | Fence | Grounded by |
|------|-------|-------------|
| `as112-external-refuses.ci` | `await=stderr:contains=refusing to start as an external plugin process` | as112 refuses at `RunEngine` top (`as112/register.go:223-225`); the ERROR is relayed to the daemon's stderr, which the runner captures |
| `cos-external-warns.ci` | `await=stderr:contains=dynamic per-interface QoS map updates` | cos warns via `warnIfExternal` (`cos/register.go:145`) then keeps running; the WARN is relayed to the daemon's stderr |

Each test pairs `await=stderr` (the fence) with the pre-existing `expect=stderr:contains=`
(the assertion) on the same line, so the stderr proof is preserved byte-for-byte and the
test cannot pass unless the line appears (await times out otherwise). Both were
mutation-proven: swapping the plugin's run command so the line never appears turns each test
RED at the await timeout (AC-6). No observer plugin, no `exited` marker, no `time.sleep`.

**Why this over the observer/exited design:** it is buildable for the reject case (the
observer is not -- Mistake Log #4), it is uniform across reject and warn, it adds no engine
surface, and it fixes the entire reject-fence bucket (trafficusage, flowexport can adopt the
same primitive). It also matches the spec's literal Task ("drop the blind holds without
losing their stderr proofs") exactly.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-design.md` - plugin/engine boundary and event surface
  → Constraint: the lifecycle signal stays engine-side observable state on `Process`, read through an existing `show` RPC; it does NOT cross the boundary as a BGP event, so the plugin/engine event surface and its namespace are untouched (this is what dissolves R-1).
- [ ] `docs/architecture/api/process-protocol.md` - the wire protocol an observer plugin speaks
  → Constraint: the observer plugin speaks only the existing command-dispatch RPC (`ze-plugin-engine:dispatch-command` -> `show system subsystem list`) plus `dispatch_until`; no new wire method, no protocol change (mirrors `test/plugin/reload-listener-rejected.ci`).
- [ ] `ai/rules/fail-closed-guards.md` - a fence that cannot advance must fail loudly
  → Constraint: `dispatch_until` returns its LAST result when attempts run out rather than raising (`test/scripts/ze_api.py:1436-1439`), so a caller that does not re-check the predicate and `runtime_fail` turns a never-advancing fence into a silent pass

**Key insights:**
- `process.go:791` `monitorCmd` is the only exit-detection site, and it discards everything: `:793` `_ = cmd.Wait()` throws away the exit code, `:795` sets `running` false, `:798-806` cancels the context and closes conns. No log, no event, no diagnostic.
- The internal-plugin path already logs its exit (`process.go:515`, `logger().Warn("internal plugin exited with non-zero code", ...)`). External has no equivalent. The asymmetry is the gap.
- `manager.go:352` `report.RaiseError(..., reportCodePluginCrash, ..., "plugin process exited unexpectedly: "+name, ...)` reads like exit detection and is NOT: it lives in `ProcessManager.Respawn` (`manager.go:324`), whose only caller is `Server.restartPlugin` (`internal/component/plugin/server/reload_tx.go:398`), a config-reload path. Verified by grepping `\.Respawn(` across `internal/` and `cmd/`: one non-test hit. Do not mistake it for a signal that already exists.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/process/process.go` - `startExternal` (`:538`) runs the plugin via `exec.CommandContext` (`:556`), `Start()` (`:593`), sets `running` (`:598`), spawns `relayStderrFrom` and `monitorCmd` (`:602-603`). `monitorCmd` (`:791`) detects exit and emits nothing.
- [ ] `internal/component/plugin/server/dispatch.go` - `deliverEvent` (`:199`) is the single fan-out point; it rejects unregistered events via `events.IsValidEvent` (`:205-209`). `registerSubscriptions` (`:135`) hardcodes `namespace := plugin.DefaultEventNamespace()` (`:148`) because the RPC carries no namespace (`:143-147`).
- [ ] `internal/component/plugin/resolve.go` - `RegisterDefaultEventNamespace` (`:89`) panics on empty (`:91`) or conflicting (`:96`) registration; `DefaultEventNamespace` (`:103`); `RegisterPluginEventTypes` (`:116`) registers into that single default (`:121`) and errors when none is registered (`:125`).
- [ ] `test/scripts/ze_api.py` - `API.wait_for_event(timeout=5.0, predicate=None)` (`:999`) blocks until a matching `deliver-event` arrives, returns `None` on timeout, and is explicitly documented as the substitute for a fixed `time.sleep` (`:1020-1022`). This is the primitive as112 wants, and it requires the observer to be subscribed.

**Behavior to preserve:**
- Both tests keep their runner-side stderr proofs byte-for-byte: `expect=stderr:contains=refusing to start as an external plugin process` (`as112:85`) and `expect=stderr:contains=dynamic per-interface QoS map updates` (`cos:86`), plus `expect=exit:code=0` on both.
- No change to as112's refuse-and-exit behavior or cos's warn-and-continue behavior. This spec adds observability; it must not alter what either plugin does.
- `show system subsystem list`'s existing per-plugin fields (`name`, `stage`, `running`, `command-count`) stay; this spec only appends `exited`. The plugin event system (`deliverEvent`, namespaces) is not touched at all.

**Behavior to change:**
- An external plugin's lifecycle becomes observable by an observer plugin. ~~Exact surface is the design question below.~~ RESOLVED 2026-07-17 (see "## Resolved Design (2026-07-17): QUERYABLE STATE"): the surface is per-plugin QUERYABLE STATE (`Process` startup stage plus a new exit observation) read via the existing `show system subsystem list` command, not a pub/sub event.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- External plugin subprocess reaches startup / emits its warn / exits (`process.go:556-603`, `:791`)

### Transformation Path
~~1. [fill during design: exit or startup observed in `process.go`]~~
~~2. [fill during design: crossing into the plugin server -- `deliverEvent` (`dispatch.go:199`) or another surface]~~
~~3. [fill during design: namespace resolution -- `resolve.go:121` is the trap, see R-6]~~
~~4. [fill during design: delivery to a subscribed observer plugin; `ze_api.py:999` `wait_for_event` on the test side]~~
RESOLVED 2026-07-17 to the QUERYABLE-STATE path (see "## Resolved Design"), corrected
2026-07-18 (see "## Audit Corrections"); the event/namespace stages above are superseded:
1. as112 refuses at the top of `RunEngine` (`!p.IsInternal()` -> `log.Error(...)` + `return 1`, `internal/plugins/as112/register.go:223-225`), before any handshake, and the subprocess exits within ms; `monitorCmd` (`process.go:791-806`) sets `Running()` false (`:795`) and (new) sets `exited` true. cos logs its WARN (`warnIfExternal`, `internal/plugins/cos/register.go:145`), completes the handshake (`:156-174`), and the engine drives it to `StageRunning` (`startup_driver.go:224`), staying `Running()` true.
2. The `Process` stays enumerable in `ProcessManager.AllProcesses()` (`manager.go:271`) after exit -- `RemoveProcess` (`manager.go:254`) is only called from reload/autoload paths (`startup.go:439`, `startup_autoload.go:319,426`) and `monitorCmd` never calls it -- so its `{stage, running, exited}` remain queryable.
3. `handleSystemSubsystemList` (`internal/component/plugin/server/system.go:271-299`, the handler the `system subsystem list` command reaches) reads `pm.AllProcesses()` and returns per-plugin `{name, stage: p.Stage().String(), running: p.Running(), exited: p.Exited(), command-count}` (the `exited` field is added by this spec; the parallel `cmd/show` handler gains the same field).
4. Each test registers a small **observer plugin** (a daemon-spawned external plugin, so it has the plugin socket the dispatch RPC needs -- see Audit Correction 1). It completes the handshake, then polls `system subsystem list` via `api.dispatch_until(...)` with a predicate on the target plugin (`as112 exited==true`, or `cos stage=="Running" && running==true`), replacing `time.sleep(4.0)`, then requests shutdown -- the same client-side poll + shutdown the sibling `test/plugin/reload-listener-rejected.ci:115-120,130-131` uses on `show reload-status`, and the same command the existing `test/plugin/subsystem-list.ci` dispatches. No `deliverEvent`, no event namespace.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin subprocess ↔ engine | `exited`/`running`/`stage` set on `Process` in `process.go` (`startExternal:598`, `monitorCmd:795`, engine `SetStage`) | [ ] |
| Observer plugin ↔ engine | command dispatch RPC `ze-plugin-engine:dispatch-command` -> `show system subsystem list` (existing; no new wire method) | [ ] |

### Integration Points
- `internal/component/plugin/process/process.go:791` (`monitorCmd`) - the exit-detection site; this spec adds `p.exited.Store(true)` here (today it discards `cmd.Wait()`'s error and emits nothing)
- `internal/component/cmd/show/system.go:71-98` (`handleShowSystemSubsystemList`) - the existing read RPC; this spec adds the `exited` field to its per-plugin output
- `test/scripts/ze_api.py:1428` (`dispatch_until` / `API.dispatch_until`) - the test-side poll primitive the observer plugin uses (returns the last result on exhaustion, `:1437-1439` -- the R-3 trap)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | ~~An event is the right shape for this signal at all~~ The signal is queryable STATE, not an event | `spec-fixit-reject-fence-observability.md:220-221` confirms ONLY that the counter does not help; it explicitly does not establish that a separate exit event is the right remedy | The whole design changes: a queryable `show`-style state (mirroring the Phase 1 counter) may fit better than a pub/sub event | Design review before Phase 1 (done 2026-07-17) | **broken/resolved** -- an event was NOT the right shape; `show system subsystem list` state is (Resolved Design 2026-07-17) |
| A-2 | One signal cannot serve both tests | `cos-external-warns.ci:74-76` (cos does not exit) vs `as112-external-refuses.ci:85` (as112 refuses and exits) | Scope shrinks to a single event | Re-read both `.ci` docstrings at Phase 0 | confirmed (2026-07-16, by reading both files) |
| A-3 | `spec-fixit-plugin-event-subscription` landing is enough to declare a non-`bgp` event | Its Gaps A/B/C thread namespace through `registerSubscriptions` (`dispatch.go:148`) | This spec must also extend `RegisterPluginEventTypes` (`resolve.go:121`), or ship under `bgp` as debt | Re-check R-6 at Phase 0 (`spec-fixit-plugin-event-subscription.md:475`) | **MOOT** -- no event is declared (queryable state), so the namespace never applies |
| A-4 | The poller can be a bare `cmd=foreground` process (implied by "no observer plugin", old Resolved Design) | -- | Design is unbuildable: dispatch needs a plugin socket | `/ze-implement` audit read `ze_api.py:103-112` + `process.go:578-584` (2026-07-18) | **BROKEN** -- an observer plugin IS required (Mistake Log #1) |
| A-5 | `running==false` is a sufficient exited fence for as112 | -- | Vacuous pass against never-started; `running` is false both before start and after exit | `/ze-implement` audit read `process.go:598,795` + `as112/register.go:223-225` (2026-07-18) | **BROKEN** -- need a monotonic `exited` marker (Mistake Log #2) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| ~~R-1~~ | ~~event registers into `bgp` namespace, observer never matches~~ | -- | **SUPERSEDED** -- no event, no namespace (queryable state) |
| ~~R-2~~ | ~~signal fires before the observer subscribes, test hangs~~ | -- | **SUPERSEDED** -- a poll has no first-send race; the observer polls until the state is observed |
| R-3 | A converted test passes without ever fencing, via the `dispatch_until` last-result trap (`ze_api.py:1437-1439`) | Test passes even when the plugin never starts | Mutation-test each conversion: break the plugin, confirm the test goes RED (AC-6). Each observer re-checks the predicate after `dispatch_until` and `runtime_fail`s if still unmet (mirror `reload-listener-rejected.ci:121-124`). |
| R-4 | as112 exits so fast the observer never catches `running==true`, so a two-phase started-then-exited fence would hang | Observer times out though as112 demonstrably refused | Fence on the monotonic `exited` marker (set once in `monitorCmd`, never cleared), not on catching the transient `running==true` |

## Wiring Test (MANDATORY — NOT deferrable)

Re-cast 2026-07-18 around the `await=stderr` fence (see "## Final Design"). No engine wiring:
the "feature code" is the runner primitive, and the entry point is the `.ci` directive.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| external plugin (as112) refuses (aborts startup, exits) | → | daemon relays as112's ERROR to its stderr → `await=stderr` fence (`await_stderr.go` `awaitDaemonStderr`, teeing the daemon stderr through a `syncWriter`) blocks until the line, then tears down | `test/plugin/as112-external-refuses.ci` (`await=stderr:contains=refusing to start as an external plugin process`; matching `expect=stderr:contains=` retained; mutation-proven RED) |
| external plugin (cos) warns and keeps running | → | daemon relays cos's WARN to its stderr → same `await=stderr` fence blocks until the line, then tears down | `test/plugin/cos-external-warns.ci` (`await=stderr:contains=dynamic per-interface QoS map updates`; matching `expect=stderr:contains=` retained; mutation-proven RED) |

Both rows are `.ci` functional tests by construction: they are the two tests this spec
exists to convert. Neither is deferrable — a row here without a real fence is the whole
defect being fixed.

### Architectural Verification
- [x] No bypassed layers (the fence reads only the daemon's own relayed stderr, which the runner already captures for `expect=stderr`; no new channel)
- [x] No unintended coupling (no engine change at all; the primitive lives entirely in the test runner)
- [x] No duplicated functionality (reuses the existing `syncWriter` pattern-wait; the `expect=stderr` assertion is unchanged)
- [x] Additive and off-by-default: a `.ci` without `await=stderr` is byte-for-byte unaffected (`teeDaemonStderr` returns the plain accumulator when the fence is nil)

## Acceptance Criteria

Re-cast 2026-07-18 around the `await=stderr` fence (see "## Final Design"). Superseded
observer/`exited` ACs are struck.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `await=stderr:contains=<text>[:timeout=<dur>]` in a `.ci` | The runner blocks until the daemon's relayed stderr contains `<text>`, then tears the daemon down; on timeout the test fails with a precise message (`await_stderr.go`) |
| AC-2 | `await=stderr` present but the line never appears | Test goes RED at the await timeout, not a vacuous pass (fail-closed) |
| AC-3 | `as112-external-refuses.ci` runs | Fences on `await=stderr:contains=refusing to start as an external plugin process`, no `time.sleep`; `expect=stderr:contains=refusing to start as an external plugin process` still passes |
| AC-4 | `cos-external-warns.ci` runs | Fences on `await=stderr:contains=dynamic per-interface QoS map updates`, no `time.sleep`; `expect=stderr:contains=dynamic per-interface QoS map updates` still passes |
| AC-5 | Both conversions land | `test/.ci-sleep-baseline` ratcheted 132 -> 130 |
| AC-6 | Each converted test, with the plugin's run swapped so its line never appears | Goes RED at the await timeout (mutation proof the fence is real) |
| AC-7 | A `.ci` without `await=stderr` | Behaves byte-for-byte as before (the primitive is additive; off unless the directive is present) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseAwaitStderr` | `internal/test/runner/await_stderr_test.go` | `await=stderr:contains=<text>[:timeout=<dur>]` parses onto the Record; empty needle, unknown type, bad timeout, and duplicate are rejected at parse time | PASS |
| `TestAwaitStderrTimeoutDefaultsOnGarbage` | `internal/test/runner/await_stderr_test.go` | the fence timeout resolver falls back to the default for empty/unparseable/zero values (never a zero timeout, which would fence on nothing) | PASS |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `as112-external-refuses` | `test/plugin/as112-external-refuses.ci` | external plugin refuses to start; operator sees the refusal | PASS (await fence, mutation-proven RED) |
| `cos-external-warns` | `test/plugin/cos-external-warns.ci` | external plugin warns and keeps running | PASS (await fence, mutation-proven RED) |

### Interop Tests
N/A — no wire protocol change, no engine change. Justify at closure.

## Files to Modify
<!-- Re-cast 2026-07-18 by "## Final Design": the await=stderr fence; NO engine change (exited marker reverted). -->
- `internal/test/runner/await_stderr.go` (new) - `parseAwait`, `awaitStderrTimeout`, `teeDaemonStderr`, `awaitDaemonStderr`, default-timeout const
- `internal/test/runner/record.go` - `AwaitStderr`/`AwaitStderrTimeout` fields on `Record`
- `internal/test/runner/record_parse.go` - dispatch `await=` to `parseAwait`
- `internal/test/runner/runner_exec.go` - create the fence `syncWriter`, tee the daemon's stderr through it, block on it before teardown (skip the `daemon.ready` wait when a fence is active)
- `internal/test/runner/runner_exec_util.go` - `newSyncWriterPattern` (custom-pattern `syncWriter` ctor)
- `internal/test/runner/await_stderr_test.go` (new) - the two unit tests above
- `test/plugin/as112-external-refuses.ci` - drop the blind sleep + `wait.py`; add `await=stderr:contains=refusing to start as an external plugin process` (keep the `expect=stderr` proof)
- `test/plugin/cos-external-warns.ci` - drop the blind sleep + `wait.py`; add `await=stderr:contains=dynamic per-interface QoS map updates` (keep the `expect=stderr` proof)
- `test/.ci-sleep-baseline` - 132 -> 130
- `docs/architecture/testing/ci-format.md` - document `await=stderr` (syntax table + section)
- REVERTED (unused, per Mistake Log #4): `Process.exited`/`Exited()` (`process.go`), `exited` field in both `system subsystem list` handlers, and their unit tests. `cmd/show/system.go` keeps only a one-line stale-comment fix the hook required.

## Implementation Steps

### Implementation Phases

~~Not planned yet: this spec is `skeleton` and blocked on `spec-fixit-plugin-event-subscription`
landing. Phases are deliberately not written, because the shape of Phase 1 depends on two
things that are not settled (A-1: is an event even the right surface; A-3/R-1: does the
dependency leave `resolve.go:121` still broken). Writing phases now would be inventing them.~~

SUPERSEDED 2026-07-17: A-1 is resolved (queryable state) and A-3/R-1 no longer apply (no
event, no namespace), so the phases below are now concrete. See "## Resolved Design".

1. **Phase 1: exit marker + surface + unit tests.** Add `Process.exited` (`atomic.Bool`) set in
   `monitorCmd` (`process.go:791-806`) with an `Exited()` accessor; add `"exited": p.Exited()`
   to `handleShowSystemSubsystemList` (`cmd/show/system.go:88-93`). Land `TestMonitorCmdMarksExited`,
   `TestAllProcessesRetainsExitedProcess`, `TestShowSystemSubsystemListReportsExited` (TDD: red first).
2. **Phase 2: convert `as112-external-refuses.ci`.** Add an observer plugin (daemon-spawned
   external plugin, mirror `reload-listener-rejected.ci`) that completes the handshake, polls
   `api.dispatch_until('show system subsystem list', predicate=as112 exited==true)`, re-checks the
   predicate + `runtime_fail`s if unmet (R-3), then requests shutdown; drop `time.sleep(4.0)`; keep
   the stderr + exit:0 proofs. Mutation-prove (R-3/AC-6).
3. **Phase 3: convert `cos-external-warns.ci`.** Same, predicate = cos `stage=="Running" && running==true`;
   drop `time.sleep(4.0)`. Mutation-prove.
4. **Phase 4: ratchet** `test/.ci-sleep-baseline` 132 -> 130 once both are green.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Each converted `.ci` mutation-proven: break the plugin, test goes RED (R-3)

### Completion (BLOCKING — before ANY commit)
- [ ] Audit (2026-07-18) validated assumptions before any code; A-4/A-5 broken assumptions recorded in the Mistake Log and the design corrected
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Critical Review Checklist

Re-cast 2026-07-18 for the await=stderr design.

| Check | What to verify | Pass? |
|-------|----------------|-------|
| Fence is real, not vacuous | Each test pairs `await=stderr` (fence) with `expect=stderr:contains=` (assertion) on the same line; if the line never appears, await times out and the test is RED. Mutation-proven: swapping the plugin's run so the line never appears turns each test RED at the await timeout (AC-6). | |
| Fail-closed on no-advance | `awaitDaemonStderr` records a precise error and returns false on timeout (`await_stderr.go`); `awaitStderrTimeout` never returns 0 (which would make `WaitFor` fence on nothing) -- `TestAwaitStderrTimeoutDefaultsOnGarbage`. | |
| Additive / off-by-default | The fence path is guarded on `rec.AwaitStderr != ""`; `teeDaemonStderr` returns the plain accumulator otherwise, so every existing `.ci` is byte-for-byte unaffected. Whole `internal/test/runner` suite passes. | |
| Stderr proofs retained byte-for-byte | `expect=stderr:contains=refusing to start as an external plugin process` (as112) and `expect=stderr:contains=dynamic per-interface QoS map updates` (cos) are unchanged. | |
| Plugin behavior unchanged | as112 still refuses-and-exits, cos still warns-and-continues. No engine change ships. | |
| No engine surface | No change to `Process`, either `system subsystem list` handler, `deliverEvent`, or the event namespace (the `exited` marker was reverted). | |
| Thread-safety | The daemon's stderr is teed through `syncWriter` (mutex-guarded); the accumulator (`clientStderr`) is only read after teardown (`terminateGracefully` calls `cmd.Wait()`, flushing I/O). No concurrent read of the plain builder. | |
| Parse validation | Malformed `await=` (empty needle, unknown type, bad timeout, duplicate) is rejected at parse time, not silently dropped -- `TestParseAwaitStderr`. | |

## Deliverables Checklist

Re-cast 2026-07-18 for the await=stderr design.

| Deliverable | Verification method | Done? |
|-------------|--------------------|-------|
| `await=stderr` runner primitive | `grep -n "await" internal/test/runner/await_stderr.go`; `TestParseAwaitStderr` + `TestAwaitStderrTimeoutDefaultsOnGarbage` pass | DONE |
| Additive / off-by-default | `grep -n "AwaitStderr" internal/test/runner/runner_exec.go` (guarded on `rec.AwaitStderr != ""`); full `internal/test/runner` unit suite passes | DONE |
| as112 conversion | `test/plugin/as112-external-refuses.ci` has no `time.sleep`, fences on `await=stderr`, passes (1.9s); mutation-proven RED at await timeout | DONE |
| cos conversion | `test/plugin/cos-external-warns.ci` has no `time.sleep`, fences on `await=stderr`, passes (2.0s); mutation-proven RED at await timeout | DONE |
| Engine untouched | `git diff --stat` shows no change under `internal/component/plugin/process`, `.../server/system.go`, `.../cmd/show/system.go` except the one-line stale-comment fix | DONE |
| Ratchet | `test/.ci-sleep-baseline` == 130; the two `.ci` in the reject/warn set carry no `time.sleep(` | DONE |
| Docs | `docs/architecture/testing/ci-format.md` documents `await=stderr` with a source anchor | DONE |

## Security Review Checklist

Re-cast 2026-07-18 for the await=stderr design. The change is entirely in the **test
runner** (never shipped in a `ze` product binary), which narrows the surface further.

| Concern | Assessment | OK? |
|---------|-----------|-----|
| Untrusted input | The `await=stderr` needle comes from a checked-in `.ci` test file (trusted test author), matched as a plain substring against captured stderr. No production/operator input path. | |
| Injection | No shell/SQL/format string building; the needle drives `strings.Contains` in `syncWriter`. The daemon is still exec'd exactly as before (no new `sh -c`). | |
| DoS / resource | `syncWriter` caps captured output at `maxOutputBytes` (10 MB, pre-existing). The fence is a bounded `WaitFor` (default 10s); no unbounded loop or new goroutine. | |
| Scope | Test-runner-only; not compiled into `ze`/`ze-appliance`/etc. No new command, route, capability, RPC, or config surface. | |
| Concurrency | The daemon's stderr is teed through the mutex-guarded `syncWriter`; the plain accumulator is read only post-teardown. No data race introduced. | |

## Documentation Update Checklist

Re-cast 2026-07-18 for the await=stderr design (no engine change; the doc change is the new
`.ci` directive).

| Category | Applies? | File + update |
|----------|----------|---------------|
| Feature list | No | No user-facing product feature; a test-runner directive. `docs/features*.md` unaffected. |
| User guide / CLI reference | No | No CLI/command change; `system subsystem list` is untouched (the `exited` field was reverted). |
| Config syntax / YANG | No | No config leaf, env var, or YANG change. |
| API / RPC docs | No | No new RPC or wire method. |
| Plugin SDK | No | No SDK surface change. |
| Wire format / RFC | No | No protocol change (Interop N/A). |
| Test infrastructure | Yes | **DONE:** `docs/architecture/testing/ci-format.md` documents the new `await=stderr:contains=` directive (syntax-overview row + a section with a source anchor to `internal/test/runner/await_stderr.go`). `test/.ci-sleep-baseline` ratchet 132 -> 130 recorded. |
| Architecture design | No | No architectural change; a test-runner-only primitive, no engine boundary. |

Doctor check: N/A -- no runtime dependency added (the change is entirely in the test runner).
No `ze doctor` check required (`ai/rules/doctor-checks.md`).

## Implementation Summary

Built the `await=stderr:contains=<text>[:timeout=<dur>]` `.ci` runner primitive
(`internal/test/runner/await_stderr.go` + wiring in `record.go`/`record_parse.go`/`runner_exec.go`/
`runner_exec_util.go`), converted both `test/plugin/as112-external-refuses.ci` and
`test/plugin/cos-external-warns.ci` to it (blind `time.sleep(4.0)` + `wait.py` removed),
ratcheted `test/.ci-sleep-baseline` 132 -> 130, and documented the directive in
`docs/architecture/testing/ci-format.md`. The originally-authorized queryable-state design
(a `Process.exited` marker polled by an in-daemon observer plugin) was implemented, then
REVERTED after runtime proof that a refusing plugin aborts the daemon's plugin-startup
coordinator (Mistake Log #4), which makes an in-daemon observer impossible for the reject
case. No engine change ships (`process.go`, `server/system.go` are byte-identical to HEAD;
`cmd/show/system.go` carries only a one-line stale-comment fix the hook required).

### Goal Validation

| Goal (from Task) | Evidence |
|------------------|----------|
| Drop the two blind `time.sleep(4.0)` holds | Both `.ci` have no `time.sleep`; `await=stderr` fences instead. Runs: as112 1.7-1.9s, cos 3.1-4.2s (were fixed 4s holds) |
| Without losing the stderr proofs | Both keep `expect=stderr:contains=`; the as112 proof was TIGHTENED to an as112-specific substring (review) |
| Fences are real, not vacuous | AC-6: inverting each plugin's producer guard (as112 `!IsInternal()`, cos warn guard) turns each test RED at the await timeout, then reverted clean |
| Ratchet 132 -> 130 | `test/.ci-sleep-baseline` == 130; counted with the ratchet's own regex |
| No regression | Full plugin `.ci` suite exit 0; `internal/test/runner` + `cmd/show` + `plugin/process` + `plugin/server` unit suites green; `make ze-lint-changed` 0 issues |

## Review Gate

Independent adversarial review of the complete diff (`ai/rules/critical-review.md`). Reviewers
were fresh subagents over the changed files, distinct lenses. Empirically re-verified each
finding before acting.

### Run 1

Reviewer A (correctness / concurrency / off-by-default) and Reviewer B (test vacuity / needle
accuracy / mutation validity).

| Severity | Finding | Location | Action |
|----------|---------|----------|--------|
| ISSUE (A) | Background-`ze` `waitReady(daemon.ready)` was NOT given the `&& rec.AwaitStderr == ""` guard the foreground path got; a future background-daemon await test would stall 5s | `runner_exec.go:806` | FIXED: added the guard, mirroring the foreground site |
| ISSUE (B) | as112 needle `refusing to start as an external plugin process` is emitted VERBATIM by three plugins (as112, traffic-usage, flow-export); under-specifies the assertion | `as112/register.go:224`, `trafficusage/register.go:68`, `flowexport/register.go:119` | FIXED: tightened needle to `... -- the address-ownership registry` (as112-unique, colon-free), both `await=` and `expect=` |
| ISSUE (B) | The swap-to-cos mutation proof was fragile: swapping to traffic-usage/flow-export would have stayed GREEN under the old generic needle | (method) | FIXED two ways: the specific needle makes any swap RED; AND ran the authoritative producer mutation (invert as112 `!IsInternal()` and cos warn guard) -> both tests RED at the await timeout, reverted |
| NOTE (A) | await without a paired `expect` fails confusingly; await precedence over `expect=exit`; timeout-message wording if testCtx<10s | -- | Accepted: both shipped tests pair await+expect; documented in `ci-format.md` ("Pair it with a matching `expect=stderr:contains=`") |
| NOTE (B) | needle containing `:` is silently truncated (same as existing `expect=stderr:contains=`); fence only honored on the orchestrated (`cmd=`) path | `record_parse.go:258`, `runner_exec.go:87` | Accepted: pre-existing parser behavior; `parseAwait` doc + `ci-format.md` warn about `:`; both tests use `cmd=foreground` |
| NOTE (B) | No unit coverage of the runtime fence (`teeDaemonStderr`) | `await_stderr_test.go` | FIXED: added `TestTeeDaemonStderr` (off-by-default + tee + needle-split-across-writes) |

Verdict Run 1: 0 BLOCKER. All ISSUEs fixed; NOTEs accepted with rationale. Both tests
re-verified GREEN (as112 1.8s, cos 3.8s); mutation-proven RED via the producer-guard inversion.

### Run 2

Fresh independent pass over the Run-1 fixes + a full sweep. Confirmed all three fixes correct
against source: (1) the as112 needle is a contiguous, colon-free, as112-unique substring and
not a substring of traffic-usage/flow-export; (2) both `waitReady` sites now carry the
`rec.AwaitStderr == ""` guard and the `daemon.pid` write is not skipped; (3) `TestTeeDaemonStderr`
asserts real behavior (off-by-default, tee, needle-split-across-writes). Also independently
re-confirmed the cos needle is unique and the `.ci-sleep-baseline` == 130 is exact.

| Severity | Finding | Location | Action |
|----------|---------|----------|--------|
| ISSUE | The `ci-format.md` await EXAMPLE used the bare ambiguous needle (the very thing Run-1 FIX-1 corrected) and did not match the file it cited | `docs/architecture/testing/ci-format.md` | FIXED: example now uses the as112-specific needle (identical to the `.ci`) + a "choose a plugin-specific needle" note. Self-verified: the doc needle now byte-matches the reviewed `.ci` line; no bare forms remain |
| NOTE | `await=` + `expect=exit:code=` would mis-assert (await case precedes the exit case in the switch) | `runner_exec.go:951` | Accepted: not a defect in this change; both shipped `.ci` removed `expect=exit:code=`; the fence replaces exit-based teardown by design |

Verdict Run 2: 0 BLOCKER, 0 surviving ISSUE (the doc example fixed and self-verified against
the already-reviewed `.ci`; the sole NOTE accepted). Gate clean.

## Known Limitations
- ~~Blocked on `spec-fixit-plugin-event-subscription` landing, by Thomas's 2026-07-16 ruling. Until then this stays `skeleton`.~~ SUPERSEDED 2026-07-17: the queryable-state design emits no event, so it needs neither that spec nor the event namespace; dependency removed, Status `ready`.
- ~~Whether the signal is an event at all is unvalidated (A-1). The source spec confirmed only the negative: the counter does not help.~~ RESOLVED 2026-07-17: A-1 answered -- the signal is queryable state (`show system subsystem list`), not a pub/sub event.
- The queryable-state fence depends on an exited external plugin staying enumerable in `AllProcesses()` (true today: `monitorCmd` sets `running=false`/`exited=true` but does not remove; `RemoveProcess` is reload/autoload-only). If a future change removes exited processes eagerly, as112's `exited==true` fence would stop matching; the observer's post-`dispatch_until` re-check + `runtime_fail` (R-3) turns that into a loud RED, not a silent pass -- flagged for the implementer.
- `Process.exited` is set only in `monitorCmd` (external-plugin exit). An internal plugin (goroutine, not a subprocess) never runs `monitorCmd`, so `Exited()` stays false for internal plugins by construction. This spec's fence targets external plugins only, matching both tests.
- The `.ci`-sleep ratchet counts only `test/**/*.ci`, so `dispatch_until`'s own internal `time.sleep` (`ze_api.py:1438`) is exempt. Converting these two tests is a real determinism win, not a ratchet trick — but the ratchet alone would not tell the difference.

## Phase 0 Findings (readiness pass, 2026-07-17; APPEND-ONLY)

> **Superseded 2026-07-17 (Thomas-authorized): the fork below is RESOLVED to queryable
> state (option (iii)); Status is now `ready`. See "## Resolved Design (2026-07-17):
> QUERYABLE STATE". The findings are retained for the record.**

Phase 0 mandated answering A-1 and A-3 before any code. This pass grounded both in
source. A-3 is now resolved with fresh evidence; ~~A-1 and the namespace decision remain a
genuine architecture fork that cannot be conservatively defaulted without violating a
constraint this spec itself records. Status therefore stays `skeleton` and this is
reported as a HARD BLOCKER, not flipped to `ready`.~~ A-1 and the namespace decision were
resolved 2026-07-17 (Thomas-authorized) to queryable state, which needs no namespace at
all -- see "## Resolved Design".

### A-3: RESOLVED (source-verified). The dependency landing does NOT unblock the declaration side.

`→ AUTONOMOUS DEFAULT (2026-07-17): A-3 confirmed BROKEN, as the row predicted.` The
dependency `spec-fixit-plugin-event-subscription` (still `Status | design`, not landed)
recorded its OWN `AUTONOMOUS DEFAULT (2026-07-17)` in risk R-6
(`spec-fixit-plugin-event-subscription.md:475-487`): it takes **option (a)** and its
namespace threading is **NOT extended to `RegisterPluginEventTypes`**
(`resolve.go:121`). So even after that spec lands, plugin-DECLARED event types still
resolve to the single `bgp` default. Verified in source: `RegisterPluginEventTypes`
(`resolve.go:116-135`) reads `DefaultEventNamespace()` at `:121` and its in-code comment
(`:118-120`) concedes the limitation; the only plugin declaring `EventTypes` today is
`rpki_decorator` (`internal/component/bgp/plugins/rpki_decorator/register.go:19`,
`update-rpki`), itself a `bgp` event, so the latent bug has never fired. The dependency
is therefore neither sufficient (it leaves `resolve.go:121` `bgp`-only) nor, see next,
necessary.

### The dependency is not strictly necessary either: the event route already works under `bgp` today.

Source-verified: the SUBSCRIBE side (`registerSubscriptions`, `dispatch.go:148`) and the
DECLARE side (`resolve.go:121`) both resolve to the same `DefaultEventNamespace()` =
`bgp` (`internal/component/bgp/plugin/register.go:64`, the sole registration call). An
event declared via `Registration.EventTypes` and subscribed via the startup RPC both land
in `bgp`, so `deliverEvent`'s `events.IsValidEvent("bgp", …)` gate (`dispatch.go:205`)
passes and delivery matches, with no dependency and no `resolve.go:121` change. The
`Depends | spec-fixit-plugin-event-subscription` line and the "stays skeleton until it
lands" note were premised on the dependency being required; that premise is false. What
the dependency actually buys is a NON-`bgp` namespace for the lifecycle event, i.e. the
architecturally-clean version only.

### ~~Residual HARD BLOCKER: two coupled decisions with no safe conservative default.~~ (RESOLVED 2026-07-17 -> queryable state, option (iii); see "## Resolved Design")

1. **A-1 (unresolved architecture fork): is a pub/sub event the right shape at all, or a
   queryable state?** Source shows the observers do not exist yet: `wait.py` in both tests
   is a bare `time.sleep(4.0)` (`as112-external-refuses.ci:79`, `cos-external-warns.ci:80`)
   run as `cmd=foreground:exec=python3 wait.py` with no `ze_api`, no plugin socket. An
   event needs an observer plugin + subscription + `wait_for_event`; a queryable state
   (mirroring the Phase 1 counter, `internal/component/plugin/server/reload_generation.go`)
   could instead be polled by `dispatch_until` without an observer plugin or the event
   namespace at all. These are materially different implementations. Choosing either still
   leaves Phase 1's Data Flow, phases, and TDD unit test to design from scratch; a fresh
   implementer would NOT have "zero questions", so this is not gap-filling that a readiness
   pass may default.
2. **The namespace decision has no fail-safe default.** The smaller/self-contained option
   (ship the lifecycle event under `bgp` as recorded debt) directly violates this spec's
   own recorded Architectural Verification item, "plugin-host lifecycle does not become a
   BGP concern" (see the Architectural Verification list and R-1). Defaulting toward the
   option that breaches a recorded architectural guard is exactly the "unsafe default" the
   readiness protocol forbids. The clean option (extend `resolve.go:121` to carry a
   namespace) is precisely the scope the dependency spec explicitly disclaimed (R-6 option
   (c), not taken); pulling a shared core-mechanism change into this spec is not a smaller
   self-contained default either.

`→ For Thomas (blocking, pick one before implementation):` (i) event under `bgp` as
migrateable debt [self-contained, breaches the no-coupling item]; (ii) event in a proper
namespace, this spec extends `resolve.go:121` [clean, widens scope to a shared mechanism];
or (iii) drop the event entirely for a queryable-state signal polled via `dispatch_until`
[sidesteps the namespace question, but is a different design than the Wiring/AC/Data Flow
tables above assume, which are all written around an event + observer].

`→ RESOLVED (Thomas-authorized, 2026-07-17): option (iii) chosen.` This spec is now
implementation-ready via the queryable-state design in "## Resolved Design (2026-07-17):
QUERYABLE STATE" (Data Flow, Implementation Phases, Wiring, ACs, and Files to Modify are
re-cast around `show system subsystem list` + `dispatch_until`, no event, no observer
plugin, no namespace change). ~~Until one is chosen this cannot be made implementation-ready;
the observer-plugin work (Ordering item 1) remains the only genuinely unblocked slice.~~
Status: `ready`.
