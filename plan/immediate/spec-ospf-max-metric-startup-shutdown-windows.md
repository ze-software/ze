# Spec: ospf-max-metric-startup-shutdown-windows

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** Two leaves of
`internal/plugins/ospf/yang/ze-ospf-conf.yang` fail for one reason and share this
spec. `max-metric/router-lsa/on-startup` is described "Advertise as a stub router
for N seconds after startup (0 disables)." and
`max-metric/router-lsa/on-shutdown` is described "Advertise as a stub router for
N seconds during a graceful shutdown (0 disables)." Both take 0 to 86400 seconds.
A reader takes each as a timed window: Ze originates its Router-LSA with
MaxLinkMetric for that many seconds, then returns to the configured link costs.
RFC 6987 Section 1 names "Graceful introduction and removal of the router to/from
the network" as a reason to advertise the stub router, which is the case both
windows serve.

**What Ze does instead.** `parseMaxMetric` (`internal/plugins/ospf/config.go`)
stores the two values in `maxMetricConfig.OnStartupSec` and `OnShutdownSec`. Two
readers exist, and neither counts a second. `deriveRICapabilities`
(`internal/plugins/ospf/ri.go`) raises `caps.StubRouter` when either value is
above 0, which sets informational capability bit 2 of the RFC 7770 Router
Information LSA. That bit announces stub-router support, not a window in force.
The show payload built in `internal/plugins/ospf/show_summary.go` reports both
values, and sets its `Active` field from `RouterLSAAlways` alone. Origination
reads `RouterLSAAlways` alone as well: `internal/plugins/ospf/instance.go` passes
`cfg.MaxMetric.RouterLSAAlways` into `OriginateFromTopology`
(`internal/plugins/ospf/lsdb/origination.go`) and into `v6OriginateSelf`. No
timer, no clock, and no shutdown hook reads either field. An operator who sets
`on-startup 300` gets the capability bit, gets the value echoed by `show ospf`,
and gets link costs that never change. `plan/audits/ospf-v2-must-audit.md`
carries the same finding in its row "Support on-startup and on-shutdown
max-metric operation", marked Missing. That file is an audit. It records the gap
and owns no fix.

**What closing it means.** The implementer either builds the two windows or
refuses both leaves at commit, the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses the `vrf` leaf. Building is
the better-shaped answer here, because the origination path that MaxLinkMetric
needs already exists and is already driven by config. `OriginateFromTopology`
takes the max-metric flag as a parameter today, so the work is a source of that
flag that varies with time rather than a new encoder. The startup window needs a
start instant and a re-origination when the window closes. The shutdown window is
the harder half, because it needs a graceful-shutdown path that holds the process
open for up to 86400 seconds and re-originates before it exits, and this spec
does not assert that such a path exists. The design must read the OSPF plugin
shutdown sequence and say so. Refusing both leaves is the cheaper move and it
removes the false promise, but it leaves an operator with no way to drain a
router other than `always`, which never clears itself. A refusal is therefore the
placeholder and this spec stays open, which is what `plan/spec-vrf.md` records
for the `vrf` leaf.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

Research at HEAD `4dcc0e5bc4` on 2026-10-13. Collision check: no session state
file and no recent commit names this spec. `plan/pre-release/spec-rfc-verdict-fix-ospf.md`
(in progress, another session) names neither max-metric, RFC 6987, `ri.go`,
`show_summary.go` nor the origination path, so the two do not overlap.

### Architecture Docs
- [ ] `docs/architecture/ospf/ospf-13-cli-diag-interop.md` - the only page that describes the two leaves
  → Constraint: its bullet "`max-metric router-lsa on-startup` and `on-shutdown` are parsed and armed nowhere" also says `stub-router.active` is faithful because origination reads `always` only. Both sentences go false the moment the startup window is armed, so the page edit rides in the same phase as the timer.
- [ ] `docs/architecture/ospf/ospf-ext-11-ldp-igp-sync.md` and `internal/plugins/ospf/ldp_sync.go` - the existing engine timer that raises link costs
  → Decision: the startup window copies the LDP-sync timer shape: the injectable `afterFunc` seam returning a stoppable `ldpSyncTimer`, plus an epoch so a stale fire after a restart or shutdown is a no-op. No new goroutine is written; `time.AfterFunc` is the precedent this package already uses (`gr_restarter.go` restart timer, `gr_helper.go` helper timer, `ldp_sync.go` hold-down timer).
- [ ] `docs/architecture/testing/interop.md` - scenario layout and checker registration
  → Constraint: a new scenario is a named directory under `test/interop/scenarios/` with no numeric prefix, and its checker registers in the special-checker map in `internal/le/interoplab/bgp/check_special.go`, as `isis-max-metric-frr` does with `checkISISMaxLinkMetric`.
- [ ] `internal/component/plugin/process/manager.go` - the daemon shutdown budget, `pluginStopGrace`
  → Constraint: `pluginStopGrace` is 3 seconds and is the daemon's own shutdown budget (`cmd/ze/hub/main.go` gives `eng.Stop` 3 seconds). No pre-stop hold phase exists, OSPF registers no `OnBye`, and the OSPF `register.go` run function calls `shutdownAll` as soon as `p.Run` returns. A shutdown window longer than 3 seconds cannot be honoured by the OSPF plugin alone.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc6987.txt` and `rfc/short/rfc6987.md` - the stub-router mechanism
  → Constraint: RFC 6987 Section 2: "To address both problems, router X announces its router-LSA to the neighbors with the cost of all non-stub links (links of the types other than 3) being set to MaxLinkMetric (defined in Section 3)." The window changes only WHEN that flag is true; the encoders that apply it (`OriginateFromTopology`, `v6OriginateSelf`) are not touched.
  → Constraint: RFC 6987 Section 3: "It is defined to be the 16-bit binary value of all ones: 0xffff."
  → Constraint: RFC 6987 defines no timer. Section 1 names the use case only: "Graceful introduction and removal of the router to/from the network." The windows are a vendor convention (FRR and Cisco `max-metric router-lsa on-startup`), so their semantics are Ze's to state, not the RFC's. New code quotes Section 1 and Section 2 above the functions that implement the window, per `ai/rules/rfc-compliance.md`.
  → Constraint: `rfc/short/rfc6987.md` is `Enrolment: backlog` with zero requirement rows, so no `RFC requirement:` tag on RFC 6987 exists, and none may be added: the owner froze RFC scope on 2026-10-08 (no new enrolment or summaries).
- [ ] `rfc/full/rfc7770.txt` Section 2.5 - the RI capability bit
  → Decision: RFC 7770 Section 2.5 assigns bit 2 as "OSPF Stub Router support [STUB]". It announces support, not a window in force, so `deriveRICapabilities` (`ri.go`) keeps raising it from any configured stub-router mode. Its tagged test (`rfc7770_ri_test.go`, RFC7770-2.4-2, configured with `always`) does not change.

**Key insights:** (minimal context to resume after compaction)
- One flag source is missing, nothing else: origination already takes the max-metric flag as a parameter for both address families.
- The startup window is buildable inside the OSPF plugin. The shutdown window is not: the daemon gives every plugin 3 seconds and has no hold phase, so it needs an owner decision (Q-1).
- No RFC-tagged test changes in either direction (RFC 6987 has no rows; the RFC 7770 tagged test uses `always`).

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` - container `max-metric/router-lsa`: `always` (boolean, default false), `on-startup` and `on-shutdown` (uint32, range 0..86400, units seconds). The `description` of each timed leaf already says it is stored, shown and raises the RI bit, and that "Ze starts no timed window". The `ze:help` strings still promise "Advertise as a stub router for N seconds after startup (0 disables)." and "... during a graceful shutdown (0 disables).", so the CLI help is the surviving false promise.
  → Constraint: the leaves are still unconsumed at HEAD: the only readers of `OnStartupSec` and `OnShutdownSec` outside tests are `parseMaxMetric`, `deriveRICapabilities` and `processSummary`.
- [ ] `internal/plugins/ospf/config.go` - `maxMetricConfig` holds `RouterLSAAlways`, `OnStartupSec`, `OnShutdownSec`; `parseMaxMetric` fills it; `validateConfig` delegates to `validateConfigAF`, which the plugin's `OnConfigVerify` and `OnConfigure` callbacks both call.
  → Decision: a commit-time refusal lives in `validateConfigAF`, not in the parser, so `TestOSPFMaxMetricConfig` (which parses `on-shutdown 60`) keeps proving the parse.
- [ ] `internal/plugins/ospf/instance.go` - `originateSelfLSAs` is the single self-LSA chokepoint for both address families. It skips while `gr.suppressOrigination()` holds (RFC 3623 restarting router), then passes `cfg.MaxMetric.RouterLSAAlways` to `v6OriginateSelf` (OSPFv3) or `lsdb.OriginateFromTopology` (OSPFv2). `originateSelfLSAsDeferred` queues a coalesced pass on the maintenance worker and is safe to call while holding `mu`. `openInterfaces` is the per-engine start: it runs for the base engine through `instanceManager.start`, for an instance created at reload through `instanceManager.reconcile`, and for the IPv6 engines through `v6EngineSet.bringUpLocked`. `shutdown` stops the LDP-sync and GR timers, cancels the engine context and joins the workers.
  → Constraint: the window flag must be read inside `originateSelfLSAs` for both families from one function, or v4 and v6 disagree.
  → Constraint: a timer callback must call `originateSelfLSAsDeferred`, never `originateSelfLSAs` inline, and `shutdown` must stop the timer before `cancel`.
- [ ] `internal/plugins/ospf/lsdb/origination.go` - `OriginateFromTopology(router, maxMetric)` builds each area's Router-LSA with every non-stub link at MaxLinkMetric when the flag is set.
- [ ] `internal/plugins/ospf/origination_v6.go` - `v6OriginateSelf(router, maxMetric)`, same flag for OSPFv3.
- [ ] `internal/plugins/ospf/ri.go` - `deriveRICapabilities` sets `caps.StubRouter` when `always` is true or either duration is above 0.
- [ ] `internal/plugins/ospf/show_summary.go` - `stubRouterView` reports `always`, `on-startup-seconds`, `on-shutdown-seconds` and `active`; `active` is set from `RouterLSAAlways` alone.
- [ ] `internal/plugins/ospf/register.go` - the run function: `p.Run` returns on the daemon's stop, then `instances.shutdownAll()` and `v6set.shutdownAll()` run at once. No `OnBye` handler.
- [ ] `internal/plugins/ospf/multi_instance.go` - `ensureSet` tears down an instance removed at reload; `start` and `reconcile` open new ones.
- [ ] `internal/plugins/ospf/rfc7770_ri_test.go`, `internal/plugins/ospf/rfc5709_config_test.go` (`TestOSPFMaxMetricConfig`), `test/ospf/ospf-show.ci` - the existing tests on these leaves. All use `always` except `TestOSPFMaxMetricConfig`, which only parses.
- [ ] `test/interop/scenarios/isis-max-metric-frr/` and `checkISISMaxLinkMetric` in `internal/le/interoplab/bgp/check_isis.go` - the IS-IS sibling: FRR reads Ze's max link metric out of the database, then Ze's routing is asserted.

**Behavior to preserve:** (unless the user explicitly said to change it)
- `always true` originates MaxLinkMetric on every non-stub link indefinitely, for OSPFv2 and OSPFv3.
- The `show ospf` JSON keys `always`, `on-startup-seconds`, `on-shutdown-seconds`, `active` keep their names and types.
- The RI capability word: bit 2 set whenever any stub-router mode is configured (RFC 7770 Section 2.5, "support").
- `TestOSPFMaxMetricConfig`, `TestRICapabilityBitsFromState` (RFC7770-2.4-2 tagged), `TestOSPFv6OriginateRouterLSAMaxMetric` and `test/ospf/ospf-show.ci` pass unchanged.
- Stub (type 3) links keep their configured cost in every mode (RFC 6987 Section 2).
- No self-LSA is originated while the RFC 3623 restarter suppresses origination, window or not.

**Behavior to change:** (only what the user asked for)
- `on-startup N` (N above 0) makes each OSPF engine originate its Router-LSA with MaxLinkMetric from its start until N seconds later, then re-originate with configured costs.
- `show ospf` `active` is true while the startup window is open, and a new `on-startup-remaining-seconds` reports the time left.
- `on-shutdown`: route decided by the owner (Q-1). Until it is built, a commit with `on-shutdown` above 0 is refused with an error naming the leaf, and the `ze:help` and `description` say so.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `ospf { max-metric { router-lsa { on-startup <0..86400>; on-shutdown <0..86400> } } }` in the daemon config, delivered to the OSPF plugin as a JSON config section through `OnConfigVerify` then `OnConfigure` / `OnConfigApply`.
- Time: the engine's start (`openInterfaces`) and a timer expiry N seconds later.

### Transformation Path
1. `parseOSPFConfig` → `parseMaxMetric` stores the seconds in `maxMetricConfig` (unchanged).
2. `validateConfigAF` refuses `on-shutdown` above 0 while Q-1 is open (new).
3. `openInterfaces` records the engine's start instant once per engine lifetime and, when `OnStartupSec` is above 0, arms the startup timer (new).
4. `originateSelfLSAs` asks one engine function whether max-metric is in force now (`always` or the startup window open) and passes that to `OriginateFromTopology` / `v6OriginateSelf` (changed: the flag source).
5. The timer fires, bumps nothing but the window state, and calls `originateSelfLSAsDeferred`; the next pass originates with configured costs, and flooding carries the new instance to neighbours (unchanged path).
6. `processSummary` reports `active` and the remaining seconds from the same function (changed).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon config → OSPF plugin | SDK config section JSON, `max-metric/router-lsa` keys as today | No |
| OSPF plugin → wire | Router-LSA in LS Update, link metric field 0xffff during the window | No |
| OSPF plugin → CLI | `show ospf` JSON `stub-router` object | No |

### Integration Points
- `originateSelfLSAs` - its two `cfg.MaxMetric.RouterLSAAlways` arguments become the window-aware flag.
- `shutdown` - stops the startup timer with the other engine timers.
- `processSummary` - `active` and the remaining time read the same flag function.
- `validateConfigAF` - the interim refusal of `on-shutdown`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | The flag enters through the existing origination parameter; no encoder edit |
| No unintended coupling (components stay isolated) | Yes (design) | Change is confined to `internal/plugins/ospf`; no daemon or SDK edit under Q-1 option (b) |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | Reuses `OriginateFromTopology`, `v6OriginateSelf`, the deferred-origination worker and the LDP-sync timer shape |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire buffer handling changes |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes (design) | Nothing added outside the OSPF plugin; Q-1 option (a) would add a daemon hold phase and must register through the SDK, re-checked then |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `openInterfaces` runs once per engine lifetime and is reached by every start path (base engine, instance created at reload, both IPv6 engines) | Callers read at HEAD: `instanceManager.start`, `instanceManager.reconcile`, `v6EngineSet.bringUpLocked` | A path that skips it starts no window; a path that calls it twice re-arms the window | Wiring tests per path (Wiring Test table) plus a grep of `.openInterfaces()` callers at implementation start | unvalidated |
| A-2 | A Router-LSA whose only change is the link metric gets a new instance and floods, subject to MinLSInterval | `OriginateFromTopology` re-originates on content change (to confirm at the producer) | Window close leaves neighbours at 0xffff until the 30-minute refresh | `TestOSPFMaxMetricStartupWindowCloses` asserts the LSDB copy's metric and a sequence bump after expiry | unvalidated |
| A-3 | Changing `on-startup` on a running engine starts no window; the window belongs to the engine's start | FRR and Cisco document `on-startup` as a boot-time behaviour (operator knowledge, not read in source here) | Owner may want a reload to arm it; then the anchor moves to config apply | Owner answer to Q-2 | unvalidated |
| A-4 | FRR's ospfd reports a 65535 link metric for Ze's point-to-point link in `show ip ospf database router <ze-router-id>` | FRR prints TOS 0 Metric per link; the IS-IS sibling reads FRR's database the same way | Checker must read another field or the JSON form | First run of `ospf-max-metric-startup-frr` | unvalidated |
| A-5 | An FRR adjacency to Ze over a point-to-point link reaches Full well inside 60 seconds in the interop lab | Existing `ospf-p2p-frr` scenario waits on Full within its 90-second budget | The window closes before FRR sees the 0xffff LSA and assertion 2 is vacuously unreachable | Scenario uses `on-startup 120` and asserts both phases; the timing is recorded on first run | unvalidated |
| A-6 | No daemon-wide pre-stop hold exists that a plugin could join | `pluginStopGrace` (3 s) in `process/manager.go`; `cmd/ze/hub/main.go` 3 s `eng.Stop` budget; no OSPF `OnBye`; `request quiesce` is a test barrier, not a stop phase | If one exists, Q-1 option (a) is cheaper than stated | Grep for a hold or drain phase in `cmd/ze/hub` and `internal/component/plugin/server` at implementation start | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The timer fires after `shutdown` and touches freed engine state | Race detector or a panic in the shutdown test | `shutdown` stops the timer before `cancel`; the callback checks the epoch and `ctx.Err()` and only calls `originateSelfLSAsDeferred`, which already returns on a cancelled context; `TestOSPFMaxMetricStartupTimerStoppedOnShutdown` |
| R-2 | v4 and v6 engines disagree because one call site still reads `RouterLSAAlways` | v6 test shows configured cost inside the window | Both call sites read one engine function; a grep of `RouterLSAAlways` outside `config.go`, `ri.go` and that function finds none |
| R-3 | A GR restart opens the window, origination is suppressed for the grace period, and the window has closed by the time the router originates, so the window is invisible | Q-2 answer, or an operator reporting no drain after a planned restart | Q-2 decides; the default design counts from engine start (A-3) and states it in YANG |
| R-4 | Interim refusal of `on-shutdown` breaks a stored config that carries it | Config load reports the refusal at boot | Ze is pre-release; the error names the leaf and says to remove it or set 0; recorded under Known Limitations until Q-1 is built |
| R-5 | The interop checker lands in a file another session is editing: `check_special.go`, `names.go` and `prepare.go` under `internal/le/interoplab/bgp/` carried foreign uncommitted hunks at research time | `git status` shows them modified | The scenario is declarative: one entry in the scenario op table in `checkers.go` (not modified at research time) using the existing `opWaitContains` and `opWaitAbsent` kinds, so no special checker is needed; re-check `git status` before editing |
| R-6 | The window flag read under `mu` deadlocks if the function takes `mu` while `originateSelfLSAs` holds it | Test hang | The function reads `now` and the stored window end under the caller's existing lock discipline; `originateSelfLSAs` copies config outside `mu` today, and the window end is copied with it |
| R-7 | A real-time timer makes unit tests slow or flaky | Test wall time above a second | The `afterFunc` seam and an injected clock, as in `ldp_sync.go`; no real sleep in unit tests |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A router with `on-startup` set either never drains at boot (today's state) or stays drained (all transit avoids it) past the window; a config with `on-shutdown` above 0 is refused at commit |
| How is it reverted? | Single commit revert; no stored state, no wire format change |
| Who else touches this path? | `originateSelfLSAs` is shared with RFC 3623 GR suppression and LDP-IGP sync (per-interface cost-out); the RI bit with `rfc7770_ri_test.go`; no in-flight spec touches these files |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `on-startup 120` at daemon boot (base OSPFv2 engine via `instanceManager.start`) | → | `openInterfaces` arms the window; `originateSelfLSAs` reads it | `test/ospf/ospf-max-metric-startup.ci` and `TestOSPFMaxMetricStartupWindowOpensAtStart` |
| Config adds an OSPFv2 instance at reload (`instanceManager.reconcile`) | → | same, for the new engine only | `TestOSPFMaxMetricStartupWindowNewInstanceAtReload` |
| Config `on-startup` on an IPv6 family (`v6EngineSet.bringUpLocked`) | → | `v6OriginateSelf` receives the window flag | `TestOSPFv6MaxMetricStartupWindow` |
| Config `on-shutdown 60` at commit | → | `validateConfigAF` refusal | `test/ospf/ospf-max-metric-on-shutdown-refused.ci` and `TestOSPFMaxMetricOnShutdownRefused` |
| FRR neighbour over a point-to-point link | → | Router-LSA flooded with 0xffff, then with the configured cost | interop scenario `ospf-max-metric-startup-frr` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior | Proof |
|-------|-------------------|-------------------|-------|
| AC-1 | `on-startup N` (N above 0), `always false`, engine has just started | Every Router-LSA Ze originates carries 0xffff (65535) as the metric of every non-stub link (types 1, 2, 4), and the configured cost on every stub (type 3) link | `TestOSPFMaxMetricStartupWindowOpensAtStart`; interop assertion 2 |
| AC-2 | Same, N seconds after start | Ze re-originates each Router-LSA with the configured costs, with a higher sequence number, and floods it; no 0xffff remains on any link | `TestOSPFMaxMetricStartupWindowCloses`; interop assertion 4 |
| AC-3 | A topology change (neighbour up, interface up) inside the window | The re-originated Router-LSA still carries 0xffff on non-stub links | `TestOSPFMaxMetricStartupWindowSurvivesReorigination` |
| AC-4 | `on-startup 0` (default) | No window: Router-LSAs carry configured costs from the first origination, exactly as today | `TestOSPFMaxMetricStartupZeroNoWindow` |
| AC-5 | `always true` together with `on-startup N` | 0xffff is advertised before, during and after the window; window expiry does not restore configured costs | `TestOSPFMaxMetricAlwaysOverridesWindowExpiry` |
| AC-6 | OSPFv3 engine with `on-startup N` | Same as AC-1 and AC-2 for the OSPFv3 Router-LSA built by the IPv6 path | `TestOSPFv6MaxMetricStartupWindow` |
| AC-7 | An OSPFv2 instance added by a reload while the daemon runs | That instance opens its own window from its own start; existing instances are unaffected | `TestOSPFMaxMetricStartupWindowNewInstanceAtReload` |
| AC-8 | `on-startup` changed from 0 to N on a running engine by reload | No window opens (A-3, pending Q-2); `show ospf` reports the new configured value and `active false` | `TestOSPFMaxMetricStartupReloadDoesNotArm` |
| AC-9 | `show ospf` inside the window | `stub-router.active` is true and `stub-router.on-startup-remaining-seconds` is the whole seconds left (above 0, at most N); outside it `active` follows `always` and the remaining key is absent | `TestOSPFShowSummaryStartupWindow`; `test/ospf/ospf-max-metric-startup.ci` |
| AC-10 | Engine shutdown while the window is open | The timer is stopped, no origination runs after `shutdown` returns, race detector clean | `TestOSPFMaxMetricStartupTimerStoppedOnShutdown` |
| AC-11 | Commit with `on-shutdown` above 0 (interim, while Q-1 is open; replaced by the Q-1 answer's ACs) | Commit refused; the error names `ospf max-metric router-lsa on-shutdown`, says Ze has no graceful-shutdown drain, and says to set 0 or remove the leaf | `TestOSPFMaxMetricOnShutdownRefused`; `test/ospf/ospf-max-metric-on-shutdown-refused.ci` |
| AC-12 | `on-shutdown 0` or absent | Commit accepted, no change | `TestOSPFMaxMetricOnShutdownZeroAccepted` |
| AC-13 | Any stub-router mode configured | RI capability bit 2 unchanged from today: set when `always`, `on-startup` above 0 (or `on-shutdown` above 0 once built) | existing `TestRICapabilityBitsFromState` passes unchanged |
| AC-14 | `ze:help` and `description` of `on-startup` and `on-shutdown`, `docs/architecture/ospf/ospf-13-cli-diag-interop.md` | State what the code now does: the startup window and its anchor; the shutdown refusal | Prose, reviewed in `/ze-review` against the producing functions; no test asserts help text |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `on-startup 120` and restarts the router so transit avoids it while BGP converges | config → `parseMaxMetric` → `openInterfaces` arms the window → `originateSelfLSAs` flag → Router-LSA 0xffff → flood → FRR database; expiry → re-origination with costs | interop `ospf-max-metric-startup-frr` |
| 2 | Checks whether the router is still draining | `show ospf` → `processSummary` → `active`, `on-startup-remaining-seconds` | `test/ospf/ospf-max-metric-startup.ci` |
| 3 | Sets `on-shutdown 60` | commit → `OnConfigVerify` → `validateConfigAF` refusal | `test/ospf/ospf-max-metric-on-shutdown-refused.ci` |

## 🧪 TDD Test Plan

### Unit Tests
**RFC-tagged tests that change: none.** RFC 6987 has no requirement rows
(`Enrolment: backlog`) and RFC scope is frozen, so no new test carries an
`RFC requirement:` tag and no discrimination record is owed. The one tagged test
near this path, `TestRICapabilityBitsFromState` (RFC7770-2.4-2, configured with
`always`), is not edited and must stay green. New implementing functions carry the
RFC 6987 Section 1 and Section 2 quotes in comments (`ai/rules/rfc-compliance.md`,
functions added after 2026-09-24). Untagged tests that must stay unchanged:
`TestOSPFMaxMetricConfig`, `TestOSPFv6OriginateRouterLSAMaxMetric`,
`TestLDPSyncP2PMaxMetricNotWithheld`, `test/ospf/ospf-show.ci`.

All timer tests drive the injected `afterFunc` seam and an injected clock; none sleeps.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFMaxMetricStartupWindowOpensAtStart` | `internal/plugins/ospf/max_metric_window_test.go` | AC-1: first Router-LSA after `openInterfaces` has 0xffff on a type-1 and a type-2 link and the configured cost on the type-3 link | |
| `TestOSPFMaxMetricStartupWindowCloses` | same | AC-2: firing the seam timer re-originates; the LSDB copy has configured costs and a higher sequence number | |
| `TestOSPFMaxMetricStartupWindowSurvivesReorigination` | same | AC-3: a re-origination triggered inside the window keeps 0xffff | |
| `TestOSPFMaxMetricStartupZeroNoWindow` | same | AC-4: no timer armed, configured costs from the start | |
| `TestOSPFMaxMetricAlwaysOverridesWindowExpiry` | same | AC-5 | |
| `TestOSPFMaxMetricStartupWindowNewInstanceAtReload` | `internal/plugins/ospf/multi_instance_test.go` | AC-7: reconcile-created instance opens its own window; an existing instance's flag unchanged | |
| `TestOSPFMaxMetricStartupReloadDoesNotArm` | `internal/plugins/ospf/max_metric_window_test.go` | AC-8 | |
| `TestOSPFMaxMetricStartupTimerStoppedOnShutdown` | same | AC-10: after `shutdown` the seam timer reports stopped; a late fire (stale epoch) originates nothing; run under `-race` | |
| `TestOSPFv6MaxMetricStartupWindow` | `internal/plugins/ospf/origination_v6_test.go` | AC-6 | |
| `TestOSPFShowSummaryStartupWindow` | `internal/plugins/ospf/show_summary_test.go` | AC-9: `active` and `on-startup-remaining-seconds` inside and after the window | |
| `TestOSPFMaxMetricOnShutdownRefused` | `internal/plugins/ospf/max_metric_window_test.go` | AC-11: `validateConfig` returns the named error for 1 and 86400 | |
| `TestOSPFMaxMetricOnShutdownZeroAccepted` | same | AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `on-startup` | 0..86400 s (YANG range, unchanged) | 86400 (and 1, the shortest window) | N/A (uint32) | 86401, refused by the YANG range |
| `on-shutdown` (interim) | 0 only accepted while Q-1 is open | 0 | N/A (uint32) | 1, refused by `validateConfigAF` |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-max-metric-startup` | `test/ospf/ospf-max-metric-startup.ci` | Daemon boots with `on-startup 300`; `show ospf` reports `active true` and a remaining time; `show ospf database router` shows the self Router-LSA with metric 65535 on its non-stub link | |
| `ospf-max-metric-on-shutdown-refused` | `test/ospf/ospf-max-metric-on-shutdown-refused.ci` | `ze config validate` (or commit) of `on-shutdown 60` fails with the named error and non-zero exit | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-max-metric-startup-frr` | `test/interop/scenarios/ospf-max-metric-startup-frr/` (`ze.conf` with a point-to-point interface, cost 10, `on-startup 120`; `frr.conf` from `ospf-p2p-frr`) | FRR ospfd | 1: FRR adjacency Full. 2: FRR's `show ip ospf database router <ze-router-id>` shows `Metric: 65535` for Ze's point-to-point link while the stub link keeps 10. 3: `show ospf` on Ze reports `"active": true`. 4: within 180 s the same FRR query no longer contains 65535 and shows the point-to-point link at 10. Declared as one op-table entry in `internal/le/interoplab/bgp/checkers.go` with `opWaitContains` / `opWaitAbsent` | |

Discrimination (`ai/rules/interop-and-goal-validation.md`): revert the flag source to
`RouterLSAAlways`, rebuild the Ze image, run the scenario and record assertion 2 red;
restore and record green. Separately, leave the timer unarmed-on-expiry (window never
closes) and record assertion 4 red. Both results go into the spec's closure evidence.

### Goal Validation

| Goal (from Task) | Evidence owed |
|------------------|---------------|
| `on-startup N` drains transit for N seconds after start, then restores costs | Interop `ospf-max-metric-startup-frr` log: FRR reads 65535 then 10 for Ze's point-to-point link, with both discrimination reds recorded |
| The operator can see the window | `test/ospf/ospf-max-metric-startup.ci` asserting `active true` and the remaining seconds through the real `show ospf` |
| No leaf promises a behaviour nothing performs | `test/ospf/ospf-max-metric-on-shutdown-refused.ci` (negative test through the real commit path) and the rewritten `ze:help` text, until Q-1's chosen route is built |

BIRD is not used: FRR already runs the OSPFv2 point-to-point scenarios and FRR is the
vendor whose `max-metric router-lsa on-startup` the leaf mirrors.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/plugins/ospf/instance.go` - `originateSelfLSAs` reads the window-aware flag for both families; `openInterfaces` arms the window once; `shutdown` stops the timer; engine fields for the window end, timer, epoch and seams.
- `internal/plugins/ospf/show_summary.go` - `active` from the window-aware flag; new `on-startup-remaining-seconds` (omitempty); stale type comment rewritten.
- `internal/plugins/ospf/config.go` - `validateConfigAF` interim refusal of `on-shutdown` above 0 with a named sentinel error; `maxMetricConfig` comments corrected.
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - `description` and `ze:help` of `router-lsa`, `on-startup`, `on-shutdown` state the window, its anchor (engine start) and the shutdown refusal.
- `internal/plugins/ospf/ri.go` - comment only if the derivation comment goes stale; logic unchanged.
- `internal/le/interoplab/bgp/checkers.go` - the `ospf-max-metric-startup-frr` op entry.
- `docs/architecture/ospf/ospf-13-cli-diag-interop.md` - replace the "parsed and armed nowhere" bullet.
- `docs/guide/ospf.md` - `show ospf` paragraph names `on-startup-remaining-seconds`; a short max-metric subsection states the three leaves' behaviour.
- `docs/architecture/testing/interop.md` - the new scenario row, if the page lists OSPF scenarios by name.
- `plan/audits/ospf-v2-must-audit.md` - not edited by this spec (an audit records; it owns no fix); closure notes the row is answered.
- `plan/journal/unwired-feature.md` - the two max-metric rows get their disposition updated at closure.

## Files to Create
- `internal/plugins/ospf/max_metric_window.go` - the window state, the window-aware flag function, arm and stop (RFC 6987 Section 1 and Section 2 quoted above them).
- `internal/plugins/ospf/max_metric_window_test.go` - unit tests above.
- `test/ospf/ospf-max-metric-startup.ci` - functional test.
- `test/ospf/ospf-max-metric-on-shutdown-refused.ci` - functional test.
- `test/interop/scenarios/ospf-max-metric-startup-frr/ze.conf`, `frr.conf` - interop scenario.

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang`: no new leaf; `description` and `ze:help` of `router-lsa`, `on-startup`, `on-shutdown` rewritten |
| YANG validation constraints | No | Ranges unchanged (0..86400). The interim `on-shutdown` refusal is a behaviour gate, not a native constraint; narrowing the range to 0 is Q-3 |
| YANG custom validators | No | Refusal lives in the plugin's `validateConfigAF`, reached through `OnConfigVerify`, like every other OSPF semantic check |
| CLI commands/flags | No | No new command; `show ospf` gains one JSON key |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | Leaves are uint32 with a range; automatic |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-max-metric-startup.ci` covers the `show ospf` key |
| Pipe completeness | N-A | `show ospf` already routes through the existing pipe path; no new output command |
| Env var registration | N-A | No `environment/` leaf |
| Doctor check for runtime dependencies | N-A | No new file, socket, port, module, sysctl or binary; a timer only |
| Prometheus counters/metrics | No | Owner may want a `ze_ospf_stub_router_active` gauge; not proposed, `show ospf` carries the state |
| BGP family surface (new SAFI / capability / attribute) | N-A | OSPF only |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` OSPF row, if it lists max-metric; otherwise no edit (check at implementation) |
| 2 | Config syntax changed? | No | Leaves and ranges unchanged; meaning stated in YANG and `docs/guide/ospf.md` |
| 3 | CLI command added/changed? | Yes | `show ospf` gains `on-startup-remaining-seconds`: `docs/guide/ospf.md` `show ospf` paragraph |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | Behaviour inside the existing OSPF plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md`: max-metric subsection (always, on-startup window anchored at engine start, on-shutdown refused until Q-1 is built) |
| 7 | Wire format changed? | No | Router-LSA format unchanged; only when the existing 0xffff value is sent |
| 8 | Plugin SDK/protocol changed? | No | Under Q-1 (b). Option (a) would change it and re-open this row |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 6987 defines no timer and its summary is backlog; RFC scope frozen, so `rfc/short/rfc6987.md` and `docs/features/rfc-status.md` are not edited |
| 10 | Test infrastructure changed? | No | Uses existing op kinds |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: check whether it claims OSPF max-metric parity with FRR; correct to "on-startup yes, on-shutdown no" if it does |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ospf/ospf-13-cli-diag-interop.md` (the "parsed and armed nowhere" bullet) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared designs: `docs/architecture/ospf/ospf-4-component-config.md` (declared by `instance.go`, `config.go`) has no max-metric text and describes no origination flag, so it is unaffected unless the implementer adds the window to its component list; `docs/architecture/ospf/ospf-ext-3-router-information.md` (declared by `ri.go`) states stub router is router-wide, which stays true because the RI derivation is unchanged. Mentioned-only pages from `./le spec citation anchors` (`ospf-11-stub-nssa.md`, `ospf-5-interface-ism.md`, `ospf-7-lsdb-flooding.md`, `ospf-ext-16-ipsec-auth.md`, `ospf-ext-7-virtual-links.md`, `ospf-ext-9-graceful-restart.md`, `wire/ospf.md`, `guide/bfd.md`, `guide/vrrp.md`) cite other symbols of `instance.go` and `checkers.go`; re-run the command after the edit |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `test/ospf/ospf-show.ci` config and any `docs/guide/ospf.md` example using `max-metric`; verify against YANG |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

Phases below assume Q-1 option (b). If the owner picks (a), a daemon hold phase
spec is written first and this spec gains a Phase 5 for the shutdown window; if (c),
Phase 4 builds the instance-removal hold instead of the refusal.

1. **Phase: Wiring (MANDATORY FIRST)** -- the window-aware flag function exists, `originateSelfLSAs` (both families) and `processSummary` read it, it returns `always` only; `openInterfaces` calls a no-op arm.
   - Tests: `TestOSPFMaxMetricStartupWindowOpensAtStart`, `TestOSPFv6MaxMetricStartupWindow`, `TestOSPFMaxMetricStartupWindowNewInstanceAtReload` written and failing.
   - Files: `max_metric_window.go`, `instance.go`, `show_summary.go`.
   - Verify: the grep for `RouterLSAAlways` outside `config.go`, `ri.go` and the flag function returns nothing; existing tests green.
2. **Phase: Startup window** -- arm once per engine at `openInterfaces` with the `afterFunc` seam and an epoch; expiry calls `originateSelfLSAsDeferred`; `shutdown` stops it before `cancel`.
   - Tests: AC-1 to AC-8 and AC-10 unit tests.
   - Files: `max_metric_window.go`, `instance.go`.
   - Verify: tests fail, implement, pass under `-race`; `docs/architecture/ospf/ospf-13-cli-diag-interop.md` bullet rewritten in this phase.
3. **Phase: Show** -- `active` and `on-startup-remaining-seconds`.
   - Tests: `TestOSPFShowSummaryStartupWindow`, `test/ospf/ospf-max-metric-startup.ci`.
   - Files: `show_summary.go`, `docs/guide/ospf.md`.
4. **Phase: on-shutdown interim refusal** -- named sentinel error in `validateConfigAF`; YANG `description` and `ze:help` for all three leaves.
   - Tests: `TestOSPFMaxMetricOnShutdownRefused`, `TestOSPFMaxMetricOnShutdownZeroAccepted`, `test/ospf/ospf-max-metric-on-shutdown-refused.ci`.
   - Files: `config.go`, `ze-ospf-conf.yang`, `docs/guide/ospf.md`.
5. **Phase: Interop** -- scenario directory and the op-table entry; run with `INTEROP_SCENARIO=ospf-max-metric-startup-frr ./le test integration interop`, keep the log; run both discrimination reverts.
   - Files: `test/interop/scenarios/ospf-max-metric-startup-frr/`, `internal/le/interoplab/bgp/checkers.go`.

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Stub (type 3) links never take 0xffff; window end produces a new LSA instance; the window is never re-armed by a reload or by `setConfig` |
| Naming | JSON key `on-startup-remaining-seconds` kebab-case, alongside `on-startup-seconds` |
| Data flow | Exactly one function decides max-metric in force; v4, v6 and `show ospf` all read it |
| Rule: `ai/rules/goroutine-lifecycle.md` | No `go func()`; the timer callback only queues deferred origination and is stopped in `shutdown` |
| Rule: `ai/rules/user-facing-errors.md` | The `on-shutdown` refusal names the leaf path, the reason, and the fix (set 0 or remove) |
| Rule: `ai/rules/stale-comments.md` | `stubRouterView` comment, `maxMetricConfig` field comments, YANG descriptions, `deriveRICapabilities` comment all match the new behaviour |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| Window-aware flag read by both origination calls | grep `RouterLSAAlways` in `internal/plugins/ospf` (non-test): only `config.go`, `ri.go`, the flag function |
| Startup window unit tests | `./le job run` scoped `go test -race ./internal/plugins/ospf/ -run 'MaxMetric|ShowSummaryStartup'` |
| Functional tests | `./le test functional` for `ospf-max-metric-startup` and `ospf-max-metric-on-shutdown-refused` |
| Interop scenario | `INTEROP_SCENARIO=ospf-max-metric-startup-frr ./le test integration interop` green, plus the two recorded reds |
| Docs | `./le spec citation anchors spec plan/immediate/spec-ospf-max-metric-startup-shutdown-windows.md` clean |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | `on-startup` bounded by YANG to 86400; the timer duration is computed from that bounded value only, no overflow in seconds-to-duration |
| Resource exhaustion | One timer per engine, armed once; reload cannot stack timers |
| Fail open | A config that cannot be honoured (`on-shutdown` above 0) is refused, never silently stored |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

- RFC 6987 gives the mechanism and the motive, never the timers. Whatever Ze states about the windows' anchor and duration is a product decision, which is why Q-2 and Q-3 exist.
- The shutdown window is not an OSPF problem: no plugin can outlive the daemon's 3-second stop budget, so a drain on shutdown is a daemon lifecycle feature that OSPF (and BGP graceful shutdown, IS-IS overload-on-shutdown) would share.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build `on-startup`; refuse `on-shutdown` above 0 until Q-1 is answered (recommended) | (1) Build both now, adding a daemon hold phase in this spec. (2) Refuse both leaves. (3) Build `on-shutdown` only for instance removal at reload | (1) widens an OSPF spec into hub and plugin-server lifecycle with no owner decision; (2) discards the cheap half, since origination already takes the flag; (3) gives a leaf that means "drain on instance delete" but not on daemon stop, which the `ze:help` text would have to explain and an operator would still misread |
| One window-aware flag function read by both families and by `show ospf` | Pass a separate flag into each call site | One reader prevents v4/v6/show disagreement (R-2) |
| Anchor the window at the engine's first `openInterfaces` | Plugin process start; first config apply; first adjacency Full | `openInterfaces` is the single per-engine start chokepoint across base, reload-created and IPv6 engines; the process start does not exist for an instance added later |
| Timer via the `afterFunc` seam plus epoch, copying `ldp_sync.go` | A ticker in the maintenance worker; a deadline checked on every origination with no timer | A deadline with no timer never re-originates at expiry (nothing triggers origination); a ticker is a recurring cost for a one-shot event |
| Keep RI bit 2 derivation unchanged | Raise bit 2 only while a window is open | RFC 7770 Section 2.5 names it "OSPF Stub Router support", a capability, and the tagged test pins it |
| Interop as a declarative op entry in `checkers.go` | A special checker in `check_special.go` | Existing op kinds express both phases; the special-checker files carry foreign edits (R-5) |

## Owner Questions

| ID | Question | Options | Lean (not approved) |
|----|----------|---------|---------------------|
| Q-1 | How should `on-shutdown` be honoured, given the daemon gives each plugin 3 seconds to stop and has no hold phase? | (a) New spec: a daemon-wide pre-stop drain phase that plugins register into, bounded by the longest configured drain; OSPF joins it. This spec refuses the leaf until it lands and stays open. (b) Refuse `on-shutdown` above 0 now; build (a) only if wanted later, closing this spec on the startup half. (c) Honour it only when an OSPF instance is removed by reload, and refuse it for daemon stop | (a) if a drain-on-stop is wanted at all, otherwise (b); the spec is written for the refusal in both. **Answered by the owner, 2026-10-13: (a), "we need a drain phase for every protocol adding routes to the FIB/RIB". A daemon-wide drain-before-stop spec is created (plan/immediate/spec-daemon-drain-before-stop.md); OSPF `on-shutdown` joins it, and this spec refuses `on-shutdown` above 0 until it lands** |
| Q-2 | Does the startup window open on every engine start, including an RFC 3623 graceful restart and `clear ospf process`? | Every engine start (GR included; `clear ospf process` is not a start); skip it after a planned GR restart; also on `clear ospf process` | Every engine start, not on `clear ospf process`; during GR the window counts while origination is suppressed (R-3) |
| Q-3 | Should the `on-shutdown` YANG range change while it is refused? | Keep 0..86400 and refuse above 0 in the plugin; narrow the YANG range to 0..0; delete the leaf until built | Keep the range and refuse in the plugin: deleting or narrowing is a schema change that Q-1 (a) would revert |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- `on-shutdown` is refused, not built, until Q-1 is answered. If the owner picks (a), the daemon drain phase becomes its own spec (`plan/spec-daemon-shutdown-drain-phase.md`, to be written on approval) and this spec stays open until OSPF joins it.
- A startup window tied to BGP convergence ("until BGP is established") is not offered; RFC 6987 Section 1 motivates it, but no leaf promises it.
- OSPFv3 uses MaxLinkMetric, as `always` does today; the R-bit alternative of RFC 6987 Section 2.1 is not used.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

For this spec: the window-aware flag function quotes RFC 6987 Section 2 ("router X
announces its router-LSA to the neighbors with the cost of all non-stub links (links
of the types other than 3) being set to MaxLinkMetric"), and the arm function quotes
RFC 6987 Section 1 ("Graceful introduction and removal of the router to/from the
network.") and states that the RFC defines no timer. The two call sites in
`originateSelfLSAs` carry `// RFC 6987 Section 2`.

### Discovery (`ai/rules/repo-maintenance.md`)

| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md` OSPF row → `docs/guide/ospf.md` max-metric subsection and `docs/architecture/ospf/ospf-13-cli-diag-interop.md` |
| What prevents regression? | The unit tests above and the interop scenario with its recorded reds |
| What inventory prevents drift? | The YANG `description` and `ze:help` are the operator contract; `plan/journal/unwired-feature.md` rows close at closure |
| What proves it? | `ospf-max-metric-startup-frr` green log and the two functional tests |
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
