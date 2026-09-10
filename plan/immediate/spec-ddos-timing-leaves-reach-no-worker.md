# Spec: ddos-timing-leaves-reach-no-worker

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 8/8 |
| Handoff | - |
| Updated | 2026-09-10 |
| Review rounds | 6; CLEAN, owner-authorized final round |

Recovery after compaction: `.claude/rules/post-compaction.md`.

The Task, Current Behavior and dated progress sections below preserve the original diagnosis.
The Implementation Audit and final round-six record state the completed result.

## Task

**What the operator is promised.** Three leaves across the three DDoS plugins
declare a period or a rate that Ze does not apply.
`internal/plugins/ddos/detect/yang/ze-ddos-detect-conf.yang` declares
`leaf check-interval`, `uint16` with `range "1..3600"` and default 1, described
as "Seconds between detection evaluations."
`internal/plugins/ddos/flowspec/yang/ze-ddos-flowspec-conf.yang` declares
`leaf announce-rate-limit`, `uint16` with `range "1..600"` and default 10,
described as "Maximum FlowSpec announcements per minute."
`internal/plugins/ddos/observe/yang/ze-ddos-observe-conf.yang` declares
`leaf stale-incident-timeout`, `uint32` with `range "1..86400"` and default
3600, described as "Seconds before an open incident without a clear event is
auto-finalized." A reader takes each to be a knob on a running worker: how often
the detector looks, how often the responder is allowed to announce, and how long
an incident may stay open before Ze closes it.

**What Ze does instead.** Each value is parsed, range-checked, and reaches no
worker.

`check-interval` is read by `ParseConfig`
(`internal/plugins/ddos/detect/config.go`) into `Config.CheckInterval`, and
`(*Config).Validate` in the same file range-checks it. No other non-test code reads the field. The
detector evaluates once per tick of the rate feed it subscribes to, and
`internal/plugins/ddos/detect/register.go` subscribes to one of two: the
traffic-statistics service through `svc.SubscribeRates`, or
`iface.SubscribeCollectNotify` as the fallback when that service is absent. Both
tick once a second, so the cadence is one second whatever the leaf holds.

`announce-rate-limit` is read by `ParseConfig`
(`internal/plugins/ddos/flowspec/config.go`) into `Config.AnnounceRateLimit`,
and `(*Config).Validate` in the same file range-checks it.
No other non-test code reads the field, and no rate limiter exists anywhere in
the plugin, so the responder announces as often as the detector asks it to.

`stale-incident-timeout` gets further than the other two and stops one step
short. `internal/plugins/ddos/observe/register.go` reads
`cfg.StaleIncidentTimeout`, converts it to a duration, and passes it to
`newStore`, which stores it as `store.staleTimeout`
(`internal/plugins/ddos/observe/store.go`). `(*store).sweepStale` in that file
reads the field and finalizes every incident older than it, and its only caller
is `internal/plugins/ddos/observe/store_test.go`. No worker of ddos observe
calls it, so an incident that never gets a clear event stays open until
`incident-ring-size` evicts it.

**What closing it means.** These three are one work item because each is the
same absence in the same subsystem: a periodic or rate control the plugin's
schema declares and no worker in that plugin enforces. Closing them is three
edits under `internal/plugins/ddos/`, and the third is close to free.
`startStaleSweep` (`internal/plugins/anomaly/observe/register.go`) is the worker
ddos observe is missing: it is already written, already called from that
plugin's engine, and already tested, over a `store` with a `sweepStale` of the
same shape. Porting it is the obvious answer for `stale-incident-timeout`, and
no refusal competes with it.

The other two are a real choice between building the behavior and refusing the
leaf at commit the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses `vrf`. For `check-interval`
the design owes an answer to whether the detector should decouple its evaluation
cadence from its feed at all, because a leaf that only ever decimates a
one-second feed buys the operator little, and a refusal states honestly that the
cadence is the feed's. For `announce-rate-limit` the design owes what the limiter
protects: FlowSpec announcements go to BGP peers, so an unbounded announce rate
is a peer-facing risk rather than a local one, which argues for building it.
Neither is obviously right, and the shape `plan/spec-vrf.md` records applies to
each: if the control is wanted, the refusal is the placeholder and the spec
stays open.

`max-mitigation-duration` under `ddos flowspec` is not in this spec.
`plan/immediate/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md`
carries it, and HEAD now enforces it.

The leaf of the SAME NAME under `ddos local` IS in this spec, added on
2026-09-08. It is the fourth instance of this spec's own class, no other spec
owns it, and the Progress entry below carries the evidence for both statements.

## Progress (2026-09-06, resumed and implemented)

The paused state this section used to describe was checked and was already past
what it said: `go vet ./internal/plugins/ddos/...` was clean, and the observe
half (`startStaleSweep`, `subscribeStore`, `sweep_test.go`) plus a written
`detect/interval_test.go` were in the tree. One unit test was red,
`TestCheckIntervalDecimatesEvaluations`, which is the TDD red the detect half
needed.

All three leaves are now implemented, and each carries a recorded red. What this
work did NOT reach is the `.ci` for `announce-rate-limit`: see Known Limitations.

## Progress (2026-09-08, design phase, re-verified against HEAD)

**The defect this spec opened on is GONE from HEAD for all three named leaves.**
Commit `3db944c1f6` landed them. The verdict below was taken at the producing
functions rather than from that commit message, and `git status` reports no
modification under `internal/plugins/ddos/`, so the working tree read is HEAD's
read.

**One leaf of the same class, in the same subsystem, is still inert:**
`ddos local max-mitigation-duration`. It is the last row of the table below, and
this design takes it into scope. The reason is in Key Design Decisions.

### Every timing leaf under `internal/plugins/ddos/`, and the reader it reaches

The search behind each "none" row is `grep -rn "<GoField>" internal/plugins/ddos/<plugin>/`
with `_test.go` and the `json:` struct tag excluded, which leaves the parse site,
the `Validate` range check, and every production read. A leaf whose only hits are
the first two reaches no worker.

| Leaf | Reader in HEAD | Producer read |
|------|----------------|---------------|
| `detect check-interval` | `newDetector` sets `evalTicks` from `cfg.CheckInterval`; `(*detector).tick` folds that many feed samples into `intervalPeak` and calls `applyTick` only when the interval closes | `internal/plugins/ddos/detect/detector.go` |
| `detect confirm-duration` | `newStateMachine(cfg.ConfirmDuration, cfg.ClearConsecutive)` | `internal/plugins/ddos/detect/detector.go` |
| `detect clear-consecutive-checks` | the same `newStateMachine` call | `internal/plugins/ddos/detect/detector.go` |
| `detect baseline-window` | `newBaseline(cfg.BaselineWindow, ...)`, twice (PPS and BPS) | `internal/plugins/ddos/detect/detector.go` |
| `detect startup-grace` | `(*detector).applyTick` compares `d.tickNum` against `d.cfg.StartupGrace` | `internal/plugins/ddos/detect/detector.go` |
| `detect characterize-window` | `filterByWindow(flows, d.cfg.CharacterizeWindow, ...)` | `internal/plugins/ddos/detect/characterize.go` |
| `detect characterize-timeout` | the `time.Duration` the characterization query is bounded by | `internal/plugins/ddos/detect/characterize.go` |
| `flowspec announce-rate-limit` | `newResponder` builds `newAnnounceLimiter(cfg.AnnounceRateLimit)`; `(*responder).announce` refuses on `!r.limiter.allow(r.clock())` | `internal/plugins/ddos/flowspec/responder.go` |
| `flowspec hold-down`, `probe-interval`, `probe-window`, `backoff-cap` | the `newProbe` call in `(*responder).announce` | `internal/plugins/ddos/flowspec/responder.go` |
| `flowspec max-mitigation-duration` | `(*responder).enforceMaxDuration`, driven by the plugin's one-second worker in `runEngine` | `internal/plugins/ddos/flowspec/register.go`, `responder.go` |
| `observe stale-incident-timeout` | `runEngine` converts it to a duration for `newStore`, then starts `startStaleSweep`, whose worker calls `(*store).sweepStale` | `internal/plugins/ddos/observe/register.go`, `store.go` |
| `observe incident-ring-size` | `newStore(cfg.IncidentRingSize, staleTimeout)` | `internal/plugins/ddos/observe/register.go` |
| **`local max-mitigation-duration`** | `(*responder).enforceMaxDuration`, driven by the plugin's one-second worker `startMaxDurationWorker` that `runEngine` starts. The cap clock is `installedAt`, written by `setStatus` on the transition into a live rule | `internal/plugins/ddos/local/register.go`, `responder.go` |

### What the fourth leaf costs the operator

`(*responder).removeMitigation` has exactly two callers, both in
`internal/plugins/ddos/local/responder.go`: the `suppressMitigation` branch of
`applyMitigation`, and `onCleared`. So an `nftables` drop rule that ddos local
installs is removed on `AttackCleared` and on no other trigger. An attack that
never clears, or a detector that never emits the clear, leaves the rule
installed for the life of the daemon while the operator has set a cap that says
it will not be.

Two records already describe this and neither changed the product, which is the
shape `ai/rules/principles.md` names:

- `plan/journal/unwired-feature.md`, row 2026-09-04, lists this leaf beside two
  of the three this spec closed, and its Fix column still reads "The pair
  defects are NOT fixed".
- The leaf's own `ze:help` in `internal/plugins/ddos/local/yang/ze-ddos-local-conf.yang`
  says "Ze parses the value and refuses it outside 0 to 86400, and then no code
  reads it", and the `max-mitigation-duration` row of the ddos local table in
  `docs/guide/ddos-mitigation.md` repeats it. An operator who reads the help
  learns the leaf is broken. That is documentation of a defect in place of its
  repair, and the wiring below deletes both sentences by making them false.

### The deferral this spec wrote in July is void

The Task section defers `max-mitigation-duration` to
`plan/immediate/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md`.
That spec is still `Status: skeleton`, dated 2026-07-16, and every row that
names the leaf cites `flowspec/config.go`. It owns the FLOWSPEC leaf, which HEAD
now enforces through `enforceMaxDuration`, and it names the local leaf only in
passing. So no spec owns the local leaf, and the deferral pointed at a future
that did not arrive. `ai/rules/git-safety.md` requires a deferral to be verified
rather than assumed; this one was checked and failed.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ddos-mitigation.md` - the operator-facing table for every leaf in this spec
  -> Decision: the guide already described `check-interval` as a working cadence, so the
     repair is the code rather than the page; the page gained the fold and the
     evaluation-counting rule the change introduces
  -> Constraint: `confirm-duration`, `clear-consecutive-checks` and `baseline-window`
     count EVALUATIONS once `check-interval` decimates, so every page and `ze:help`
     that called them ticks was wrong the moment the leaf started working
- [ ] `docs/architecture/anomaly/anomaly-3-observe.md` - the twin this spec ports from
  -> Decision: the ddos observe sweep takes the anomaly shape unchanged: a one-second
     ticker, a stop that waits for the worker, and a teardown that detaches the bus
     before it stops the worker

**Key insights:**
- `baseline.go` `slowAdaptSamples` already documented `check-interval` as decimating
  ("the count is in samples, so wall-clock time scales with check-interval"). The
  codebase's own design intent was decimation; the leaf simply never reached the tick.
- `internal/component/trafficstat` starts its one-second ticker on the first
  `SubscribeRates`, over `iface.ListRates()`, whose tracker the interface plugin starts
  unconditionally. So the detector ticks with no `traffic { usage }` block and no eBPF,
  which is what lets the functional test run unprivileged.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ddos/detect/detector.go` - `tickRates`/`tickInfos` incremented
      `tickNum` and called `applyTick` on every feed sample; `applyTick` held the
      periodic baseline-save modulo
- [ ] `internal/plugins/ddos/detect/register.go` - `subscribe` takes `trafficstat`
      (`EnsureGlobal`, so always) and falls back to `iface.SubscribeCollectNotify`
- [ ] `internal/plugins/ddos/detect/baseline.go` - the window counts SAMPLES, and its
      comment already assumed one sample per `check-interval`
- [ ] `internal/plugins/ddos/flowspec/responder.go` - `announce` is the single place an
      announcement leaves the plugin; both `onDetected` (blackhole fallback) and
      `onCharacterized` reach it
- [ ] `internal/plugins/ddos/observe/store.go` - `sweepStale` finalizes every incident
      older than `staleTimeout`; its only caller was `store_test.go`
- [ ] `internal/plugins/anomaly/observe/register.go` - `startStaleSweep`, the worker this
      spec ports
- [ ] (2026-09-08) `internal/plugins/ddos/local/responder.go` - `setStatus` is the ONLY
      writer of `active` and `target`; `removeMitigation` has two callers, the
      `suppressMitigation` branch of `applyMitigation` and `onCleared`, and neither is a
      timer. The struct holds no clock
  -> Constraint: `applyMitigation` re-installs in place while active, so the cap clock
     MUST be written on the false-to-true transition and not on every install
- [ ] (2026-09-08) `internal/plugins/ddos/local/register.go` - `runEngine` starts no
      worker. It already holds `activeResponder atomic.Pointer[responder]` and builds
      `ctx, cancel := sdk.SignalContext()` before `p.Run`, which is where the cap worker
      attaches
- [ ] (2026-09-08) `internal/plugins/ddos/flowspec/register.go` and `responder.go` -
      `maxDurationCheckInterval`, the one long-lived worker, `enforceMaxDuration` and
      `clock()`: the shape ported to ddos local
  -> Decision: the port keeps the explicit no-cap guard on zero and the wall-clock
     comparison, both for the reasons `enforceMaxDuration` already states in place

**Behavior to preserve:**
- At `check-interval 1` (the default and every `.ci` in the tree) the detector evaluates
  once per second, exactly as before.
- `startup-grace` stays in seconds: it reads `tickNum`, which stays a feed-sample counter.
- The periodic baseline save stays on its ~300-second cadence at any `check-interval`.
- `show ddos incidents` and `show ddos flowspec` payload shapes are unchanged.

**Behavior to change:**
- The detector evaluates once per `check-interval`, over the interval's peak.
- The flowspec responder refuses an announcement over `announce-rate-limit`.
- ddos observe runs a sweep worker, so `stale-incident-timeout` finalizes incidents.
- (Walked-into defect, fixed) `announce` refuses an unresolved victim instead of
  dispatching `destination-ipv4 invalid Prefix`.
- (Added 2026-09-08) ddos local runs a cap worker, so an `nftables` drop rule is
  removed after `max-mitigation-duration` seconds and not only on `AttackCleared`.
  At `0`, and at the default 3600 with any attack shorter than an hour, no
  operator sees a difference.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config: `ddos { detect { check-interval } observe { stale-incident-timeout }
  flowspec { announce-rate-limit } }`, delivered to each plugin as a JSON section whose
  every leaf is a string.

### Transformation Path
1. `ParseConfig` + `Validate` in each plugin's `config.go` (unchanged).
2. detect: `newDetector` reads `CheckInterval` into `evalTicks`; the feed callback
   (`onRates`/`onRate`) folds each sample into `intervalPeak` and calls `applyTick` only
   when the interval closes.
3. flowspec: `newResponder` builds `announceLimiter` from `AnnounceRateLimit`; `announce`
   consults it before it dispatches.
4. observe: `runEngine`'s `apply` builds the store, subscribes it, and starts
   `startStaleSweep`, which calls `store.sweepStale` once a second.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> plugin | JSON section, every leaf a string | Yes - `test/plugin/ddos-timing-leaves.ci` sets both leaves in operator syntax |
| trafficstat -> detector | `SubscribeRates(d.onRates)`, one sample a second | Yes - the same `.ci` drives it with a real UDP flood |
| Detector -> observe store | `ddosevent.Detected` on the event bus | Yes - `TestDdosObserveUnsubscribeDetachesStore` and the `.ci` |
| Responder -> BGP engine | `update text ... nlri ipv4/flow add` over `UpdateRoute` | Yes - the daemon rejected the unresolved-victim form, which is how that defect was found |

### Integration Points
- `internal/plugins/anomaly/observe/register.go` `startStaleSweep` - ported, not shared:
  the two stores are different types in different plugins.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Each leaf is read by the plugin that declares it; nothing crosses a plugin boundary |
| No unintended coupling (components stay isolated) | Yes | No new import in any of the three plugins |
| No duplicated functionality (extends existing, does not recreate) | Yes | `startStaleSweep` is the anomaly shape ported; the limiter is new because no rate limiter existed |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control plane; `intervalPeak` is five words on the detector struct |
| Registration over hardcoding | Yes | No central enumeration edited; the fixture registers itself in `internal/test/fixture` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `check-interval` was always meant to decimate evaluations rather than to sample the feed | `baseline.go` `slowAdaptSamples` comment, written before this spec | The leaf would need a refusal instead, and `baseline.go` would need its comment corrected | Read of the producing comment plus the YANG range 1..3600 | confirmed |
| A-2 | Both rate feeds publish once a second, so `check-interval` seconds equal `check-interval` samples | `trafficstat/service.go` `tickInterval = time.Second`; `detect/register.go` subscribe | The fold would count the wrong unit and every derived leaf would drift | Read of both producers; `test/plugin/ddos-timing-leaves.ci` measured 19.9s for two evaluations at `check-interval 10` | confirmed |
| A-3 | Refusing an announcement over the limit is right, and deferring it is not | The leaf's own words ("maximum announcements per minute") and the `Dispatch` failure path, which already leaves the responder idle | A deferred-announce queue would be needed, with a staleness rule | Design decision recorded below; `TestAnnounceRateLimitRefusalLeavesResponderIdle` | confirmed |
| A-4 | The ddos observe sweep can take the anomaly worker unchanged | `internal/plugins/anomaly/observe/register.go`, whose `store` has a `sweepStale` of the same shape | A different lifecycle would be needed | Ported and driven by `TestDdosObserveStaleSweepTickerFinalizes` and the `.ci` | confirmed |
| A-5 | `ddos local max-mitigation-duration` is meant to be enforced, not withdrawn from the schema | Its twin under `ddos flowspec` carries the same name, the same units and the same "0 = no cap" rule, and HEAD enforces that one in `enforceMaxDuration`. `plan/journal/unwired-feature.md` records the local half as an unfixed pair defect | The leaf would be deleted from the YANG and from `Config`, and the guide row would say the cap is a flowspec-only control | The Key Design Decisions row, and Thomas overruling it if he wants the leaf gone instead | confirmed: the leaf is wired and proven. The owner can still reverse the call, and reversing it deletes the leaf rather than changing the worker |
| A-6 | One second is the right re-check cadence for the local cap | `maxDurationCheckInterval` in `internal/plugins/ddos/flowspec/register.go` is `time.Second`, and the two caps are the same control on two responders | A cap set to 1 second could overshoot by up to the cadence | Ported constant, and `TestLocalMaxDurationRemovesTheRule` asserts removal within a bounded number of ticks rather than at an exact instant | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Peak-holding the interval raises the baseline as well as the threshold, so a spiky link drifts its own threshold up | The baseline p99 climbing on a link with no attack | Both sides read the same statistic: `applyTick` feeds the peak to `baseline.Add` and compares against a threshold built from peaks. Stated in the `intervalPeak` doc comment |
| R-2 | An operator raising `check-interval` silently multiplies `confirm-duration` and `clear-consecutive-checks` in wall-clock terms | An attack confirmed far later than the operator expected | Said in the YANG `ze:help` of all three leaves and in `docs/guide/ddos-mitigation.md` |
| R-3 | The functional test's timing floor could pass against broken code on a loaded machine | A CHECK-INTERVAL-HELD value close to the floor | The margin was measured and widened: 10 to 20 seconds wired against 3 seconds with the leaf ignored, floor 8 seconds |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A detector that evaluates at the wrong cadence detects late or not at all; a limiter that refuses too much leaves an attack unmitigated upstream. A local cap that fires early lifts an on-host drop while the attack is still running, which is the direction that hurts: it is bounded by the operator's own value, and at the default 3600 no attack shorter than an hour reaches it. Nothing reaches the wire differently at the default config, where `check-interval` is 1, `announce-rate-limit` is 10 and both `max-mitigation-duration` leaves are 3600 |
| How is it reverted? | A single commit revert. No config migration: every leaf already existed, already had its range, and already had its default |
| Who else touches this path? | `plan/immediate/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md` owns `max-mitigation-duration` in the FLOWSPEC responder, which HEAD enforces. Nothing owns the ddos local leaf of the same name, which is why this spec took it (2026-09-08) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ddos detect check-interval 10` in the daemon config | -> | `(*detector).tick` folding ten feed samples into one `applyTick` | `test/plugin/ddos-timing-leaves.ci` (CHECK-INTERVAL-HELD) |
| `ddos observe stale-incident-timeout 1` in the daemon config | -> | `startStaleSweep` -> `(*store).sweepStale` | `test/plugin/ddos-timing-leaves.ci` (STALE-SWEEP-FINALIZED) |
| A rate feed sample | -> | `(*detector).tick` | `TestCheckIntervalDecimatesEvaluations`, `TestCheckIntervalEvaluatesTheIntervalPeak` |
| `ddosevent.Detected` on the bus | -> | `subscribeStore` -> `(*store).open` | `TestDdosObserveUnsubscribeDetachesStore` |
| `ddosevent.Characterized` on the bus | -> | `(*responder).announce` -> `announceLimiter.allow` | `TestAnnounceRateLimitBoundsTheWindow` |
| `ddos flowspec announce-rate-limit 1` in the daemon config | -> | `newResponder` -> `announceLimiter` -> the second generation's `announce` refused | `test/plugin/ddos-announce-rate-limit.ci` (ANNOUNCE-REFUSED) |
| `ddos local max-mitigation-duration 1` in the daemon config | -> | `runEngine`'s worker -> `(*responder).enforceMaxDuration` -> `removeMitigation` | `test/plugin/ddos-local-max-duration.ci` (LOCAL-CAP-REMOVED) |
| The plugin's one-second worker tick | -> | `(*responder).enforceMaxDuration` | `TestLocalMaxDurationRemovesTheRule`, driven through the worker `startMaxDurationWorker` returns and never by calling `enforceMaxDuration` directly |
| `ddosevent.Detected` on the bus | -> | `applyMitigation` -> `setStatus(true, ...)` -> `installedAt` | `TestLocalMaxDurationClockStartsOnTheFirstInstall` |
| The `ddos local` block deleted and committed | -> | `parseSections` -> `replaceResponder` -> `(*responder).withdrawMitigation` -> the kernel | `test/plugin/ddos-local-config-removed.ci` (CONFIG-REMOVED-WITHDRAWN), `TestLocalRemovingTheSectionRemovesTheDrop` |
| `ddos local response-level alert` committed while a drop is live | -> | `replaceResponder` -> `(*responder).withdrawMitigation` -> the kernel | `TestLocalLeavingEnforceRemovesTheDrop` |
| The parent `ddos` block deleted and committed | -> | `runEngine`'s exit defer -> `stopResponder` -> `(*responder).withdrawMitigation` -> the kernel | `test/plugin/ddos-parent-config-removed.ci` (PARENT-REMOVED-WITHDRAWN), `TestLocalEngineStopRemovesTheDrop` |
| `ddos local forward-mitigation false` committed while a FORWARD drop is live | -> | `replaceResponder` -> `(*responder).withdrawMitigation` -> the kernel | `TestLocalDisablingForwardMitigationRemovesTheForwardDrop`, `TestLocalDisablingForwardMitigationLeavesTheIngressDrop` |
| An `AttackCharacterized` already dispatched when a config apply retires its responder | -> | `(*responder).applyMitigation` returning on `retired` | `TestLocalRetiredResponderInstallsNothing`, `TestLocalRetiredResponderDoesNotReclaimACarriedRule` |
| A reload transaction that verifies here and rolls back | -> | `pendingConfig.rollback` -> `clear` -> the next apply failing closed | `TestPendingConfigRollbackUnstagesTheCandidate` |
| A `ze_ddos-local` drop rule a previous process left in the kernel | -> | `runEngine` initial `OnConfigure`, after the firewall dependency -> `clearStaleDropRule` -> claim, reconcile, withdraw, reconcile | `test/plugin/ddos-local-stale-table-swept.ci` (STALE-TABLE-SWEPT, COPP-RULE-PRESERVED), `TestLocalStartupSweepsTheConfiguredBackend` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `check-interval N` with an unbroken above-threshold feed | The detector evaluates once every N feed samples, over the interval's peak rate on the interface that carried it, so `confirm-duration` evaluations take at least `(confirm-duration - 1) * N` seconds |
| AC-2 | More announcements asked of the flowspec responder in one 60-second window than `announce-rate-limit` allows | Every announcement over the limit is refused and logged, the responder stays idle, and the budget returns 60 seconds after the announcement that held it |
| AC-3 | An incident open longer than `stale-incident-timeout` with no `AttackCleared` | A running worker finalizes it with an end time, whatever the attack is doing |
| AC-4 | The periodic baseline save at a `check-interval` that does not divide 300 | The save still runs on its ~300-second cadence |
| AC-5 | A critical `AttackDetected` whose victim was never resolved, with `blackhole-fallback` on | The responder announces nothing and consumes no announce budget (walked-into defect, `plan/journal/zero-value-as-valid-answer.md`) |
| AC-6 | A ddos local drop rule installed longer than `max-mitigation-duration` seconds, with no `AttackCleared` | A running worker removes the rule, publishes the idle status, and logs the removal with the leaf's value. `show ddos local` then reports no mitigation |
| AC-7 | `max-mitigation-duration 0` under `ddos local`, with a drop rule installed and an attack that never clears | The rule stays installed. Zero means no cap, which is what the leaf's own `description` says, and it MUST NOT be read as an expiry of zero seconds |
| AC-8 | An `AttackCharacterized` that re-installs the rule in place, arriving while the rule is already active | The cap keeps counting from the FIRST install. A refresh MUST NOT restart the clock, or an attack that re-characterizes every minute never expires |
| AC-9 | `ddos local` configured, no attack, for longer than `max-mitigation-duration` | Nothing is removed and nothing is logged. The worker is a no-op while no rule is installed |
| AC-10 | A `ze_ddos-local` drop rule in the kernel when ze starts, put there by a previous process and claimed by no owner in this one | The plugin sweeps the rule and table during initial `OnConfigure`, after the configured firewall dependency and before any responder subscribes. The sweep is best-effort with stage-specific error logging, preserves other owners' desired rules, and runs regardless of `firewall flush-on-shutdown`. Reload does not repeat it. Protection after restart comes from fresh detection (owner directive, 2026-09-09) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | sets `check-interval 10` and floods the box | config -> `newDetector` -> trafficstat feed -> `tick` fold -> `applyTick` -> `AttackDetected` -> `show ddos incidents` | `test/plugin/ddos-timing-leaves.ci` |
| 2 | sets `stale-incident-timeout 1` and reads `show ddos incidents` during an attack that never clears | config -> `runEngine` apply -> `startStaleSweep` -> `sweepStale` -> the incident carries an end time | `test/plugin/ddos-timing-leaves.ci` |
| 3 | sets `announce-rate-limit 1` and meets two attack generations in a minute | config -> `newResponder` -> `announce` -> `announceLimiter.allow` refuses the second | `test/plugin/ddos-announce-rate-limit.ci`, green in the guest on 2026-09-09 and red under the limiter cut |
| 4 | sets `max-mitigation-duration 1` under `ddos local` and floods a victim the box owns, with a flood that never stops | config -> `runEngine` worker -> `enforceMaxDuration` -> `removeMitigation` -> `show ddos local` reports no mitigation and `nft` holds no drop rule | `test/plugin/ddos-local-max-duration.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckIntervalDecimatesEvaluations` | `internal/plugins/ddos/detect/interval_test.go` | AC-1: two of three samples accumulate, the third evaluates | red then green |
| `TestCheckIntervalEvaluatesTheIntervalPeak` | `internal/plugins/ddos/detect/interval_test.go` | AC-1: the evaluation reads the interval peak and its interface | green, red under the closing-sample cut |
| `TestBaselineSaveFollowsTheFeedTickNotTheEvaluation` | `internal/plugins/ddos/detect/interval_test.go` | AC-4 | green, red under the save-inside-applyTick cut |
| `TestAnnounceRateLimitBoundsTheWindow` | `internal/plugins/ddos/flowspec/announce_limit_test.go` | AC-2 | red then green |
| `TestAnnounceRateLimitRefusalLeavesResponderIdle` | `internal/plugins/ddos/flowspec/announce_limit_test.go` | AC-2: a refusal never claims a rule | red then green |
| `TestAnnounceRateLimitUnsetTakesTheDefault` | `internal/plugins/ddos/flowspec/announce_limit_test.go` | an unvalidated Config takes the documented default, not a budget of none | red then green |
| `TestAnnounceRefusesAnUnresolvedVictim` | `internal/plugins/ddos/flowspec/announce_limit_test.go` | AC-5 | red then green |
| `TestDdosObserveStaleSweepTickerFinalizes` | `internal/plugins/ddos/observe/sweep_test.go` | AC-3 driven through the worker, never through `sweepStale` | green, red under the worker cut |
| `TestDdosObserveStaleSweepStops` | `internal/plugins/ddos/observe/sweep_test.go` | the worker stops on its stop function | green |
| `TestDdosObserveUnsubscribeDetachesStore` | `internal/plugins/ddos/observe/sweep_test.go` | the detach a config apply relies on | green |
| `TestLocalMaxDurationRemovesTheRule` | `internal/plugins/ddos/local/max_duration_test.go` | AC-6, driven through the WORKER so deleting the wiring turns it red | red then green |
| `TestLocalMaxDurationZeroMeansNoCap` | `internal/plugins/ddos/local/max_duration_test.go` | AC-7: at 0 the rule survives a worker tick well past any deadline | red then green |
| `TestLocalMaxDurationClockStartsOnTheFirstInstall` | `internal/plugins/ddos/local/max_duration_test.go` | AC-8: a characterized re-install does not restart the cap | red then green |
| `TestLocalMaxDurationIdleWorkerRemovesNothing` | `internal/plugins/ddos/local/max_duration_test.go` | AC-9: no rule installed, no removal, no log | red then green |
| `TestLocalMaxDurationWorkerStops` | `internal/plugins/ddos/local/max_duration_test.go` | the worker returns when its context is cancelled (`ai/rules/goroutine-lifecycle.md`) | red then green |
| `TestLocalRetiredResponderInstallsNothing` | `internal/plugins/ddos/local/max_duration_test.go` | AC-6's premise: an event in flight cannot re-install through a responder a removal retired | red under the guard cut, green with it |
| `TestLocalRetiredResponderDoesNotReclaimACarriedRule` | `internal/plugins/ddos/local/max_duration_test.go` | AC-6: one rule keeps one owner when the rule is CARRIED | red under the guard cut, green with it |
| `TestLocalEngineStopRemovesTheDrop` | `internal/plugins/ddos/local/max_duration_test.go` | AC-6's premise on the exit path, which is the stop that delivers no config | red under the withdrawal cut, green with it |
| `TestLocalEngineStopAfterAWithdrawTouchesTheKernelOnce` | `internal/plugins/ddos/local/max_duration_test.go` | the config boundary and the exit path do not double-withdraw over one commit | green |
| `TestLocalDisablingForwardMitigationRemovesTheForwardDrop` | `internal/plugins/ddos/local/max_duration_test.go` | AC-6's premise across a commit of `forward-mitigation false` | red under the arm cut, green with it |
| `TestLocalDisablingForwardMitigationLeavesTheIngressDrop` | `internal/plugins/ddos/local/max_duration_test.go` | the same commit leaves an INPUT drop for a box-owned victim alone | red if the arm is widened to every live rule |
| `TestLocalReturningToEnforceWaitsForTheNextDetection` | `internal/plugins/ddos/local/max_duration_test.go` | the alert-then-enforce round trip the guide now describes | red under the incoming-retirement cut |
| `TestPendingConfigRollbackUnstagesTheCandidate` | `internal/plugins/ddos/local/max_duration_test.go` | a rolled-back transaction leaves nothing a later apply can pick up | red under the unstaging cut, green with it |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `detect check-interval` | 1-3600 | 3600 | 0 | 3601 |
| `flowspec announce-rate-limit` | 1-600 | 600 | 0 | 601 |
| `observe stale-incident-timeout` | 1-86400 | 86400 | 0 | 86401 |
| `local max-mitigation-duration` | 0-86400 | 86400 | -1 | 86401 |

Each range is already enforced and tested by the owning `Config.Validate`
(`detect/config_test.go`, `flowspec/config_test.go`, `observe/config_test.go`,
`local/config_test.go`); this spec changed no range and added none.

`local max-mitigation-duration` is the one row whose LAST VALID value is not the
end of a period. Its range opens at 0, and 0 is not a one-second cap: it means
no cap, as the leaf's `description` states and as the flowspec twin's
`enforceMaxDuration` already implements with an explicit `<= 0` guard rather
than by arithmetic. So the boundary set for it is four values and not three:
`0` (no cap, AC-7), `1` (the shortest real cap), `86400` (the longest), and
`86401` (refused by `Validate`). A test that walks 1, 86400 and 86401 and skips
0 would leave the fail-open reading uncovered, which is the case
`ai/rules/principles.md` names as a zero that behaves correctly by accident.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-timing-leaves` | `test/plugin/ddos-timing-leaves.ci` | An operator sets `check-interval 10` and `stale-incident-timeout 1`, floods the box, and reads `show ddos incidents` | green; red under each of the two cuts |
| `ddos-announce-rate-limit` | `test/plugin/ddos-announce-rate-limit.ci` | An operator sets `announce-rate-limit 1` and meets two attack generations in a minute. The second announce is refused | green in the QEMU guest (2026-09-09), red under the cut that stops `announce` consulting the limiter; AC-2. `option=needs-linux:caps=net-admin,bpf` |
| `ddos-local-max-duration` | `test/plugin/ddos-local-max-duration.ci` | An operator sets `max-mitigation-duration` under `ddos local`, floods a victim the box owns, and the drop rule is gone from the kernel while the flood is still running | green in the QEMU guest, red under the worker cut; AC-6. `option=needs-linux:caps=net-admin,bpf` |
| `ddos-local-cap-survives-reload` | `test/plugin/ddos-local-cap-survives-reload.ci` | An operator commits an unrelated `ddos local` change while a drop rule is live, and the cap still removes it | green in the QEMU guest (2026-09-09), red under the cut that deletes the carry; AC-6 across a config apply. `option=needs-linux:caps=net-admin,bpf` |
| `ddos-local-config-removed` | `test/plugin/ddos-local-config-removed.ci` | An operator deletes the `ddos local` block while a drop rule is live, and the rule leaves the kernel with it | green in the QEMU guest (2026-09-09), red under the cut that deletes both withdraw arms; AC-6's premise. `option=needs-linux:caps=net-admin,bpf` |
| `ddos-parent-config-removed` | `test/plugin/ddos-parent-config-removed.ci` | An operator deletes the whole `ddos` block while a drop rule is live, which tells the plugin nothing and stops it anyway, and the rule still leaves the kernel | green in the QEMU guest (2026-09-09), red under the cut that deletes the exit-path withdrawal; AC-6's premise. `option=needs-linux:caps=net-admin,bpf` |

Both new `.ci` tests take the topology and the gate of
`test/plugin/ddos-transit-forward-drop.ci`, which already builds the veth
transit pair, drives two attack generations, and reads `nft` back from its
privileged root driver. Its header states the gate this design reuses verbatim:

```
option=needs-linux:caps=net-admin,bpf
```

`ddos-announce-rate-limit` needs the whole of that topology, because the
flowspec responder announces only for a REMOTE victim: both `onDetected` and
`onCharacterized` return early on `e.Direction == ddosevent.DirectionLocal`, and
`announce` refuses an unresolved victim (AC-5). A remote victim needs a
box-unowned destination and the eBPF traffic-usage source that names it, which
is why no unprivileged `.ci` can reach the limiter.

`ddos-local-max-duration` needs less. Its victim is LOCAL, so no veth transit
pair is required, but the assertion is an `nft` readback and the install path is
`nftables`, so it keeps `caps=net-admin` and stays in the same suite.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| none | - | - | N-A: no wire format changes. The one wire-visible effect is that FEWER FlowSpec announcements leave Ze, and one malformed origination stops being attempted | N-A |

## Files to Modify
- `internal/plugins/ddos/detect/detector.go` - `intervalPeak`, `tick`, `evalTicks`; the baseline save moves onto the feed sample
- `internal/plugins/ddos/detect/yang/ze-ddos-detect-conf.yang` - `check-interval` help rewritten; `confirm-duration`, `clear-consecutive-checks`, `baseline-window` and `startup-grace` now say what they count
- `internal/plugins/ddos/flowspec/responder.go` - `announceLimiter`, the limiter check, the unresolved-victim guard
- `internal/plugins/ddos/flowspec/yang/ze-ddos-flowspec-conf.yang` - `announce-rate-limit` help rewritten
- `internal/plugins/ddos/observe/register.go` - `subscribeStore`, `startStaleSweep`, one `apply` for both config paths
- `internal/test/fixture/plugin_fixture_05_ddos.go` - the `ddos-timing-leaves` fixture
- `docs/guide/ddos-mitigation.md` - the `check-interval` fold, and the three leaves that count evaluations
- `plan/journal/zero-value-as-valid-answer.md` - the walked-into defect

Added 2026-09-08 for AC-6 through AC-9:

- `internal/plugins/ddos/local/responder.go` - `installedAt` and `now` on the responder, `clock()`, `enforceMaxDuration`, and the `installedAt` write inside `setStatus`
- `internal/plugins/ddos/local/register.go` - `maxDurationCheckInterval` and the one long-lived worker in `runEngine`
- `internal/plugins/ddos/local/yang/ze-ddos-local-conf.yang` - the `max-mitigation-duration` `ze:help`, which today states the leaf is unread
- `docs/guide/ddos-mitigation.md` - the ddos local table row that today states the leaf is unread
- `internal/test/fixture/plugin_fixture_05_ddos.go` - the two new fixtures
- `plan/journal/unwired-feature.md` - the Fix column of the 2026-09-04 row, which names this leaf

Added 2026-09-09, clearing the round 2 review gate:

- `internal/plugins/ddos/local/config.go` - `ParseConfig` reports whether the delivered section carries a `ddos local` body
- `internal/plugins/ddos/local/register.go` - `parseSections` carries that answer, `pendingConfigured` moves it from verify to apply, and `replaceResponder` withdraws instead of carrying on the two commits that say the box must stop dropping
- `internal/plugins/ddos/local/responder.go` - `withdrawMitigation` and its two reasons; `adoptMitigation` transfers ownership off the responder it replaces
- `internal/test/fixture/plugin_fixture_06_ddos_linux.go` - the config-removal driver, and the reload driver's poll order
- `docs/guide/ddos-mitigation.md` - the commit table, which replaces a paragraph the removal case made false
- `plan/journal/component-rebuilt-during-reload.md` - two walked-into finds

Added 2026-09-09, clearing the round 3 review gate:

- `internal/plugins/ddos/local/responder.go` - `retired` and `hook` on the responder, `retire`, the retirement guard at the top of `applyMitigation`, the `hook` parameter on `setStatus`, and two more withdraw reasons
- `internal/plugins/ddos/local/register.go` - `pendingConfig` with `stage`, `take`, `clear` and the `rollback` handler; `retireResponder` and the exit defer that calls it; the forward-hook arm and the retirement in `replaceResponder`
- `internal/plugins/ddos/local/config.go` - `ParseConfig` drops its named results
- `internal/plugins/ddos/local/max_duration_test.go` - `countingTables`, and the eight lifecycle tests round 3 owes
- `internal/plugins/ddos/local/show_test.go` - the `setStatus` call site takes the hook
- `internal/test/fixture/plugin_fixture_06_ddos_linux.go` - the parent-removal driver
- `docs/guide/ddos-mitigation.md` - two more commit-table rows, and what returning to `enforce` does
- `plan/journal/late-write-lands-on-the-successor.md` - the dispatch contract behind finding 14
- `plan/journal/committed-spec-fails-its-own-write-gate.md` - this spec's own round 2 code block

Added 2026-09-09, clearing the round 4 review gate:

- `internal/plugins/ddos/local/register.go` - `clearStaleDropRule` and its call from `runEngine`; the legacy-sweep block it replaces is deleted; `retireResponder` renamed `stopResponder`, and its shutdown claim replaced by what the code holds
- `internal/plugins/ddos/local/responder.go` - `removeMitigation` reports one outcome, never a refusal followed by a success
- `internal/plugins/ddos/local/max_duration_test.go` - `countingTables` counts reconciles, and the once-only test asserts that count
- `internal/test/fixture/plugin_fixture_06_ddos_linux.go` - the stale-table planter and the sweep probe
- `docs/guide/ddos-mitigation.md` - the daemon-stop claim, the sweep, and the operator's exposure window after a restart
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - Rule 4, the Rule 3 note on ddos-local's exit withdraw, and the Rule 2a trigger list
- `plan/journal/gate-excludes-part-of-its-population.md` - the residual: nothing sweeps a ze table whose owner the current config does not start

## Files to Create
- `internal/plugins/ddos/detect/interval_test.go`
- `internal/plugins/ddos/flowspec/announce_limit_test.go`
- `internal/plugins/ddos/observe/sweep_test.go`
- `test/plugin/ddos-timing-leaves.ci`
- `internal/plugins/ddos/local/max_duration_test.go`
- `test/plugin/ddos-announce-rate-limit.ci`
- `test/plugin/ddos-local-max-duration.ci`
- `test/plugin/ddos-local-cap-survives-reload.ci`
- `test/plugin/ddos-local-config-removed.ci`
- `test/plugin/ddos-parent-config-removed.ci`
- `test/plugin/ddos-local-stale-table-swept.ci`
- `internal/plugins/ddos/local/stale_table_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Every leaf already existed, including `local max-mitigation-duration`; only `ze:help` prose changed |
| YANG validation constraints | No | Ranges unchanged. `local max-mitigation-duration` keeps `0..86400`, and 0 keeps meaning no cap |
| YANG custom validators | No | Native `range` is sufficient. A validator that REFUSED the leaf was rejected: see Key Design Decisions |
| CLI commands/flags | No | No command added. `show ddos local` already reports the mitigation status the cap clears |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/ddos-timing-leaves.ci`, `test/plugin/ddos-announce-rate-limit.ci`, `test/plugin/ddos-local-max-duration.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No `environment/` leaf |
| Doctor check for runtime dependencies | No | No new file, socket, port or module; the sweep worker and the ddos local cap worker are both in-process |
| Prometheus counters/metrics | No | The refusal and the cap removal are logged at WARN and INFO. A counter would be a new metric name, which is its own decision |
| BGP family surface | N-A | No SAFI, capability or attribute touched |
| Goroutine lifecycle | Yes | The ddos local worker takes the flowspec shape: ONE worker for the plugin, reading the live responder through `activeResponder.Load()`, selecting on the `sdk.SignalContext()` context. Never one goroutine per mitigation (`ai/rules/goroutine-lifecycle.md`) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Four declared leaves start working; `docs/features.md` describes none of them |
| 2 | Config syntax changed? | No | Syntax unchanged |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | No registration, transport or dependency change |
| 6 | Has a user guide page? | Yes | `docs/guide/ddos-mitigation.md`. The `check-interval` half is updated. The ddos local `max-mitigation-duration` row still reads "Parsed and range-checked, but the local responder does not act on it yet", which the AC-6 wiring makes false. That row and the ddos local mitigation-lifetime prose are rewritten in the SAME work as the code (`ai/rules/documentation.md`), never in a later pass |
| 6b | Does a page DOCUMENT the defect rather than the behavior? | Yes | Two do, and both are repaired by the same edit: the guide row above, and the `ze:help` of `local max-mitigation-duration` in `internal/plugins/ddos/local/yang/ze-ddos-local-conf.yang`. A help text that tells the operator the leaf they are setting is ignored is a defect report published on the product surface |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 8955 encoding is unchanged; the guard stops an origination that never encoded |
| 10 | Test infrastructure changed? | No | One fixture added to an existing file, one `.ci` to an existing suite |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes (2026-09-09) | `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`. Rule 4 states that an attack-response table is cleared at the next start and why one reconcile cannot do it; Rule 3 gains what ddos-local's per-plugin withdraw is for and what it does not promise; Rule 2a's trigger list drops ddos-local. `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` still describes the detector's stages, not its cadence, and is unaffected |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/immediate/spec-ddos-timing-leaves-reach-no-worker.md` names two more: `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md`, whose "Characterization runs off the rate tick" section now says what one tick means at a `check-interval` over 1, and `docs/architecture/ddos/ddos-detect-enhancements.md`, which describes the BPS trigger and the bits-per-second unit and says nothing about the evaluation cadence, so it is unaffected. `docs/guide/ddos-mitigation.md` is the declared page and is updated |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/flowspec-protected-router.md` shows `set ddos flowspec announce-rate-limit 10`, which is still valid syntax with the documented default |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the worker and the tick each get a failing test
   - Tests: `TestDdosObserveStaleSweepTickerFinalizes`, `TestCheckIntervalDecimatesEvaluations`
   - Files: `observe/register.go`, `observe/sweep_test.go`, `detect/interval_test.go`
   - Verify: each test fails because nothing in production reaches the code
2. **Phase: detect check-interval** -- fold the feed into an evaluation
   - Tests: the three in `interval_test.go`
   - Files: `detect/detector.go`, `detect/yang/ze-ddos-detect-conf.yang`, `docs/guide/ddos-mitigation.md`
   - Verify: green, and red again under a cut that samples the closing tick and under a cut that moves the baseline save back
3. **Phase: flowspec announce-rate-limit** -- bound the announcements per minute
   - Tests: the four in `announce_limit_test.go`
   - Files: `flowspec/responder.go`, `flowspec/yang/ze-ddos-flowspec-conf.yang`, `docs/guide/ddos-mitigation.md`
   - Verify: green, and red under a cut that skips the limiter
4. **Phase: the operator path** -- one `.ci` over one flood for the two leaves it can reach
   - Tests: `test/plugin/ddos-timing-leaves.ci`
   - Files: `internal/test/fixture/plugin_fixture_05_ddos.go`
   - Verify: green, and red under each of the two production cuts
5. **Phase: ddos local max-mitigation-duration, wiring first** (added 2026-09-08)
   - Tests: `TestLocalMaxDurationRemovesTheRule` first, driven through the worker the
     new `startMaxDurationWorker` returns and never by calling `enforceMaxDuration`
   - Files: `internal/plugins/ddos/local/max_duration_test.go`
   - Verify: it fails because no production path reaches a cap. That red is the point
     of the phase: the sibling defect was hidden for months by a `store_test.go` that
     called `sweepStale` directly, and `plan/journal/unwired-feature.md` names that
     shape as the reason nobody saw the hole
6. **Phase: ddos local max-mitigation-duration, the code**
   - Tests: the five in `max_duration_test.go`
   - Files: `internal/plugins/ddos/local/responder.go`, `register.go`,
     `internal/plugins/ddos/local/yang/ze-ddos-local-conf.yang`,
     `docs/guide/ddos-mitigation.md`
   - Verify: green, and red again under a cut that removes the worker, and under a cut
     that writes `installedAt` on every install rather than on the transition (AC-8)
   - The YANG `ze:help` and the guide row lose the sentences that say the leaf is
     unread, in THIS phase and not a later one (`ai/rules/documentation.md`)
7. **Phase: the two remaining operator paths**
   - Tests: `test/plugin/ddos-announce-rate-limit.ci` (AC-2),
     `test/plugin/ddos-local-max-duration.ci` (AC-6)
   - Files: `internal/test/fixture/plugin_fixture_05_ddos.go`
   - Verify: each red under the production cut for its own AC before it is green.
     Both need the QEMU guest: `option=needs-linux:caps=net-admin,bpf`
8. **Phase: the journal row**
   - Files: `plan/journal/unwired-feature.md`
   - Verify: the 2026-09-04 row's Fix column names which of its leaves are now wired.
     Recording never replaced the fix, so the row is closed by the fix and not before

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 `detector.go` `tick`; AC-2 `responder.go` `announce`; AC-3 `observe/register.go` `startStaleSweep`; AC-4 `detector.go` `tick` save block; AC-5 `responder.go` `announce` guard |
| Feature completeness | `test/plugin/ddos-announce-rate-limit.ci` has both QEMU GREEN and rebuilt limiter-cut RED, recorded under the AC-2 functional proof |
| Correctness | The fold reports the interval PEAK, not the closing sample, and attributes PPS and BPS to their own interfaces; a refused announce consumes no budget and leaves the responder idle |
| Naming | `evalTicks` is in feed samples and `tickNum` counts them, so the two units cannot be confused; `announceWindow` names the minute the leaf is stated over |
| Data flow | `check-interval` is read once, in `newDetector`; nothing else in the package reads `cfg.CheckInterval` |
| Rule: `ai/rules/goroutine-lifecycle.md` | The sweep worker has one owner, one stop, and a stop that waits; `teardown` detaches the bus before it stops the worker |
| Rule: `ai/rules/principles.md` | A zero `check-interval` and a zero `announce-rate-limit` are each named, floored, and (for the limiter) tested |
| Completeness (AC-6..AC-9) | `local/register.go` worker; `local/responder.go` `enforceMaxDuration` and the `installedAt` write in `setStatus` |
| Rule: `ai/rules/goroutine-lifecycle.md` (ddos local) | ONE worker for the plugin, not one per mitigation; it reads the live responder through `activeResponder.Load()` and returns on the `sdk.SignalContext()` context, so a config apply that replaces the responder needs no restart |
| Rule: `ai/rules/principles.md` (ddos local) | `0` is an explicit no-cap GUARD with a name, a comment and its own test (AC-7), never a zero that behaves correctly by accident |
| Rule: `ai/rules/documentation.md` | The YANG `ze:help` and the guide row that today DESCRIBE the defect are corrected in the phase that fixes it. A page that documents a broken leaf is a debt to a reader outside this repo |
| Naming | `installedAt` mirrors the flowspec `announcedAt`, and each names the event that starts its own cap. `maxDurationCheckInterval` keeps the sibling's name so a reader who greps one finds both |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The detector evaluates on the interval | `go test -run TestCheckInterval ./internal/plugins/ddos/detect/` |
| The responder bounds its announcements | `go test -run TestAnnounceRate ./internal/plugins/ddos/flowspec/` |
| The sweep worker runs in the plugin | `grep -n startStaleSweep internal/plugins/ddos/observe/register.go` shows the call inside `apply` |
| The operator reaches both | `ze-test bgp plugin 214` (ddos-timing-leaves) |
| The ddos local cap runs in the plugin | `grep -n startMaxDurationWorker internal/plugins/ddos/local/register.go` shows the call inside `runEngine` |
| No ddos timing leaf is inert | Every row of the leaf-reader table in the 2026-09-08 Progress entry names a production reader, with no "none" left |
| No `MUTATION-APPLIED` left applied | `grep -rn MUTATION-APPLIED internal/plugins/ddos internal/test test/` returns nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Every leaf keeps its YANG range and its `Validate` check. The limiter's slice is capped at the leaf's own maximum of 600 entries and never grows past it |
| Resource exhaustion | The fold adds five words to the detector and no allocation per sample. The sweep is one goroutine for each configured store, stopped on every apply |
| Failing open | The one direction that fails open is a Config that skipped `Validate`, where the limiter takes the documented default of 10 rather than a budget of none. Named, commented and tested |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood -> RESEARCH |
| Lint failure | Fix inline. If architectural -> DESIGN |
| Functional test fails | Check the AC: wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The codebase had already written down what `check-interval` was supposed to do:
  `baseline.go` `slowAdaptSamples` says "the count is in samples, so wall-clock time
  scales with check-interval". Implementing the leaf made an existing comment true
  rather than inventing a meaning for it. A leaf that reaches no worker can still leave
  its design intent in the tree, and that intent is evidence.
- Peak-folding is what makes decimation safe. Sampling the closing tick would blind the
  detector to every flood shorter than `check-interval`, which is the attack shape it
  exists to catch. The threshold stays comparable because the baseline is built from the
  same peaks.
- The `.ci` found a defect three unit-test suites could not: the responder dispatched
  `destination-ipv4 invalid Prefix` whenever the victim was unresolved. Only a real
  daemon with a real BGP engine answered.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `check-interval` decimates evaluations and reads the interval PEAK | Sample the closing tick; average the interval; refuse the leaf at commit | The peak is the only fold that keeps a sub-interval flood visible. Refusal was live until `baseline.go` was read: its comment already treats one sample as one `check-interval`, so the design decision had been taken and only the code was missing |
| `tickNum` stays a FEED counter, so `startup-grace` and the baseline save stay in seconds | Move both onto the evaluation | `startup-grace` is documented in seconds, and `baselineSaveInterval` is a modulo: at a `check-interval` that does not divide 300, no evaluation would ever be a multiple and the crash-safety save would stop running. `TestBaselineSaveFollowsTheFeedTickNotTheEvaluation` pins it |
| An announcement over `announce-rate-limit` is REFUSED | Defer it and announce when the window frees | Refusal is the leaf's own words, it reuses the idle path `Dispatch` failure already takes, and it needs no queue, no timer and no staleness rule. Deferral would add all three for a case the operator asked to be capped |
| The limiter is consulted in `announce`, the single exit | Consult it in `onDetected` and `onCharacterized` | A limit either path could walk around is not a limit |
| A non-positive `announce-rate-limit` takes the documented default | Treat it as unlimited; treat it as a budget of none | Unlimited is the fail-open zero the style guide names. A budget of none disables upstream mitigation for a Config that merely skipped `Validate`. The default is the only reading that is both safe and legible, and it is tested |
| `startStaleSweep` is PORTED from anomaly, not shared | Extract a common worker over an interface | The two stores are different types in different plugins, and the worker is fifteen lines. An interface to share it would be more machinery than the duplication costs |
| `local max-mitigation-duration` is WIRED, not deleted (2026-09-08) | Delete the leaf and its `Validate` arm; refuse it at commit the way `unimplementedVRFValidator` refuses `vrf` | Deleting it leaves a leaf that exists under `ddos flowspec` and not under `ddos local`, with the same name and the same units, which is a worse surface than either half. The behavior is also wanted on its own: `removeMitigation` has two callers and neither is a timer, so an attack that never clears leaves an `nftables` drop rule installed for the life of the daemon. A cap is the only thing that bounds it. A refusing validator was considered and rejected for the reason the brief names: it converts a silent broken promise into a loud one and leaves the box with no bound on a drop rule |
| The cap CLOCK is written by `setStatus`, not by `applyMitigation` (2026-09-08) | Set `installedAt` in `applyMitigation` beside the reconcile | `applyMitigation` runs on BOTH `onDetected` and `onCharacterized` and re-installs in place while active, so writing the clock there restarts the cap on every characterization and an attack that re-characterizes never expires (AC-8). `setStatus` is already the documented ONLY writer of `active` and `target`, so making it the only writer of `installedAt` keeps the three from drifting, and the transition it must detect (false to true) is visible there and nowhere else |
| The ddos local worker is PORTED from flowspec, not shared | One cap worker in a package both plugins import | The two responders are different types in different plugins, and `ai/rules/plugins.md` keeps a plugin's policy inside the plugin. The port is the same call this spec already made for `startStaleSweep`, and the flowspec original is fifteen lines |
| `0` keeps meaning NO CAP, checked explicitly | Treat 0 as the shortest cap; forbid 0 by narrowing the range to `1..86400` | The leaf's `description` already promises "0 = no cap" and the flowspec twin already implements it with an explicit `<= 0` guard, so the two same-named leaves must not disagree. The guard is explicit rather than arithmetic for the reason `enforceMaxDuration` states: a zero read as a deadline expires every rule on the first tick |

## Known Limitations

**Updated 2026-09-09. The item below is CLOSED.**
`test/plugin/ddos-announce-rate-limit.ci` exists and RAN in the QEMU guest,
green, and red under the cut that stops `announce` consulting the limiter. The
reason it costs what it costs is unchanged and is worth keeping, so the text
stays as the record of why no unprivileged `.ci` can reach the limiter.

- **(closed) `announce-rate-limit` had no `.ci`.** The unit tests drive the responder through its
  real event handlers and are discriminated, but no functional test proves an operator
  reaches the limit. Reaching the announce path in a daemon needs a victim that resolves
  to a REMOTE prefix: `flowspec` defers to on-host mitigation for a local victim, and an
  unresolved victim is now refused outright (AC-5). A remote victim needs the veth
  transit topology and the eBPF traffic-usage source that
  `test/plugin/ddos-transit-forward-drop.ci` builds, which runs as root under QEMU only.
  The test shape is known and was drafted: two attack generations inside one minute, with
  `max-mitigation-duration 1` withdrawing the first announce, the flood stopped to clear
  the attack and restarted for the second generation, and the second announce asserted
  refused only AFTER a second incident proves the responder was asked. That is the test
  that now exists, and its red-then-green walk in the guest is in the Review Gate.
- The refusal is logged and not counted. A Prometheus counter for refused announcements
  is a new metric name, which is its own decision and its own spec.
- The detector still emits a critical `AttackDetected` with no victim when no traffic
  source can name one. AC-5 stops the responder acting on it; whether the DETECTOR should
  emit it at all is the open question the journal row records.
- **The start sweep needs the plugin to start.** `clearStaleDropRule` runs from
  `runEngine`, and the plugin engine is started from the `ddos local` config root, so an
  operator who stops ze with a drop installed under `firewall { flush-on-shutdown false; }`,
  deletes `ddos local` from the config file, and restarts, leaves `ze_ddos-local` enforcing
  with nothing on the box that mentions it. Nothing in `internal/plugins/ddos/**` can close
  that: the repair belongs to the firewall engine, which is the one actor that knows every
  name a ze build can own and runs whatever the config says. Recorded as a row in
  `plan/journal/gate-excludes-part-of-its-population.md`, with `ze_flowspec`,
  `ze_anomaly-shape` and `ze_copp` in the same position.

## TDD Evidence (AC-6 through AC-9, 2026-09-08)

Every command below ran through `./le job run label unit-ddos-local`, scoped to
`./internal/plugins/ddos/local/`, with `-race`.

**RED, before any product code.** The five tests named the wiring that did not
exist, so the package did not build:

```
# github.com/ze-software/ze/internal/plugins/ddos/local [.test]
max_duration_test.go:97:4: r.now undefined (type *responder has no field or method now)
max_duration_test.go:102:2: undefined: startMaxDurationWorker
max_duration_test.go:142:4: r.now undefined (type *responder has no field or method now)
...
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local [build failed]
```

**GREEN, after the worker and the cap.**

```
=== RUN   TestLocalMaxDurationRemovesTheRule
INFO ddos-local: drop rule installed target=10.0.0.1/32 hook=ingress phase=detected
INFO ddos-local: max-mitigation-duration reached, removing the drop rule target=10.0.0.1/32 seconds=60
INFO ddos-local: drop rule removed target=10.0.0.1/32
--- PASS: TestLocalMaxDurationRemovesTheRule (0.05s)
--- PASS: TestLocalMaxDurationZeroMeansNoCap (0.10s)
--- PASS: TestLocalMaxDurationClockStartsOnTheFirstInstall (0.00s)
--- PASS: TestLocalMaxDurationIdleWorkerRemovesNothing (0.05s)
--- PASS: TestLocalMaxDurationWorkerStops (0.01s)
ok  	github.com/ze-software/ze/internal/plugins/ddos/local	1.748s
```

**Discrimination cut 1: the clock moves to every install (AC-8).** `installedAt`
was written in `applyMitigation` beside the reconcile, and the transition write
in `setStatus` was deleted. Exactly one test went red, and it is the AC-8 test:

```
--- PASS: TestLocalMaxDurationRemovesTheRule (0.05s)
--- PASS: TestLocalMaxDurationZeroMeansNoCap (0.10s)
    max_duration_test.go:206: a refresh restarted the cap clock: 61s after the FIRST install the rule must be gone
--- FAIL: TestLocalMaxDurationClockStartsOnTheFirstInstall (2.01s)
--- PASS: TestLocalMaxDurationIdleWorkerRemovesNothing (0.05s)
--- PASS: TestLocalMaxDurationWorkerStops (0.01s)
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	2.697s
```

**Discrimination cut 2: the worker tick reaches no cap.** The
`r.enforceMaxDuration()` call inside `startMaxDurationWorker` was replaced by a
bare `activeResponder.Load()`. Both cap tests went red, which is what proves
they drive the product through the worker rather than through the enforcement
function:

```
--- FAIL: TestLocalMaxDurationRemovesTheRule (2.05s)
    max_duration_test.go:126: a drop rule older than max-mitigation-duration must be removed by the worker
--- FAIL: TestLocalMaxDurationClockStartsOnTheFirstInstall (2.00s)
    max_duration_test.go:206: a refresh restarted the cap clock: 61s after the FIRST install the rule must be gone
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	4.764s
```

Both cuts were reverted, and `grep -rn MUTATION-APPLIED internal/plugins/ddos test/`
returns nothing. The whole subsystem is green at `-count=3`:
`go test -race -count=3 ./internal/plugins/ddos/...` -> `ok` for detect,
flowspec, flowtriq, local, observe.

## The AC-6 functional proof, run in the QEMU guest (2026-09-08)

`test/plugin/ddos-local-max-duration.ci` was RUN on ze's runtime kernel, through
`./le qemu run kernel tmp/kernel/build/vmlinuz packages "iproute2 libcap"` with
the guest-side `ze-test bgp plugin ddos-local-max-duration`. The probe floods a
box-owned victim without stopping, so no `AttackCleared` can arrive, and it reads
the removal back twice: from the kernel through netlink, and from
`show ddos local`.

**GREEN.**

```
9.7s     1/1  PASS  219  ddos-local-max-duration
pass  1/1  100.0%  9.7s
QEMU VM: PASS
```

The daemon's own lines, from the run of the same binary whose log the runner
dumped:

```
INFO ddos-local: drop rule installed  target=127.0.0.9/32 hook=ingress phase=detected
INFO ddos-local: max-mitigation-duration reached, removing the drop rule  target=127.0.0.9/32 seconds=5
INFO ddos-local: drop rule removed  target=127.0.0.9/32
```

**RED, under the cut that stops the worker reaching the cap.**
`r.enforceMaxDuration()` inside `startMaxDurationWorker` was replaced by a bare
`activeResponder.Load()`, `bin/ze-linux-arm64` was REBUILT so the cut reached the
guest's daemon, and the same test then failed on the kernel readback:

```
44.7s    1/1  FAIL  219  ddos-local-max-duration
ZE-OBSERVER-FAIL: the drop for 127.0.0.9 was still installed 40s after it went in,
  with max-mitigation-duration set and the flood still running (sent 672000 packets,
  show ddos local=map[]):
table=ze_ddos-local chain=ingress rules=1
QEMU VM: FAIL (exit code 1)
```

The cut was reverted, `bin/ze-linux-arm64` rebuilt, and the test re-run GREEN
(the PASS above is that run). So the `.ci` discriminates: it fails when the
worker stops reaching the cap, and the kernel readback is what catches it.

## Implementation Summary

### What Was Implemented
- `internal/plugins/ddos/local/responder.go`: `installedAt` and `now` on the
  responder, `clock()`, and `enforceMaxDuration`, which removes a live drop rule
  that has reached `max-mitigation-duration`. `0` is an explicit no-cap guard
  with its own comment and its own test, never arithmetic.
- `internal/plugins/ddos/local/responder.go` `setStatus`: the ONE writer of
  `installedAt`, on the false-to-true transition. `applyMitigation` re-installs
  in place on every `AttackCharacterized`, so a clock written there would let an
  attack that re-characterizes outlive any cap (AC-8).
- `internal/plugins/ddos/local/register.go`: `maxDurationCheckInterval`
  (one second, the flowspec sibling's name and value) and
  `startMaxDurationWorker`, ONE long-lived worker for the plugin. It reads the
  live responder through `activeResponder.Load()`, so a config apply that
  replaces the responder needs no restart, and it returns when the
  `sdk.SignalContext()` context is cancelled. `runEngine` starts it once, before
  `p.Run`.
- `internal/plugins/ddos/local/max_duration_test.go`: the five tests above, all
  driven through the worker.
- `internal/test/fixture/plugin_fixture_06_ddos_linux.go`: the two driver
  fixtures `plugin/ddos-local-max-duration-driver` and
  `plugin/ddos-announce-rate-limit-driver`; `fixture06DDOSDropState` now takes
  the victim address, because two tests read that kernel state for two
  addresses; and `fixture06OpenRemoteFlood`, the dialed flood a REMOTE victim
  needs (`p05OpenFlood` also binds a sink socket on the victim, which answers
  "cannot assign requested address" for an address the box does not hold).
- The local cap and announce-limit functional scenarios passed in QEMU and failed under their respective production cuts. The dated evidence below records each run.

### Bugs Found/Fixed
- The dated Review Gate records the reload orphan, retired-responder race, parent-removal withdrawal, startup backend ordering and their regression proofs. Findings 26–30 are resolved by the current implementation and evidence.

### Documentation Updates
- `internal/plugins/ddos/local/yang/ze-ddos-local-conf.yang`: the
  `max-mitigation-duration` `ze:help` said "Ze parses the value ... and then no
  code reads it". It now states the behavior: the age check once a second, the
  clock that a narrowing does not restart, and `0` for no cap. A
  `revision 2026-09-08` records the change.
- `docs/guide/ddos-mitigation.md`: the ddos local `max-mitigation-duration` row
  said "Parsed and range-checked, but the local responder does not act on it
  yet". It now states the cap. A "When the clear never comes" paragraph in
  "Local mode clear signal" names the cap as the second removal trigger, with a
  `<!-- source: -->` anchor on `enforceMaxDuration` and
  `startMaxDurationWorker`.
- `./le doc check links` and `./le docs-to-code index-check` report no finding on
  any file this phase touched. `./le yang glue check`: 154 directories current.

### Deviations from Plan
- Linux-only fixture implementations live in `internal/test/fixture/plugin_fixture_06_ddos_linux.go`. The earlier write-scope and functional-run limitations are resolved in the dated evidence.
- `enforceMaxDuration` splits the flowspec twin's single compound guard into two
  guards, one per reason, so the no-cap guard carries its own comment
  (`docs/contributing/ze-go-style.md`, "Control flow a reader can simulate", and
  `ai/rules/principles.md` on naming a guard). The behavior is identical.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An `nftables` drop rule ddos local installs is bounded by the operator's cap, not only by `AttackCleared` | functional (unit through the worker) | `TestLocalMaxDurationRemovesTheRule`: the rule is removed by the worker 61s into a 60s cap with no clear event. Red under the worker cut above |
| A leaf the schema declares is read by a running worker, so the leaf-reader table has no "none" row left | source | The `local max-mitigation-duration` row of the leaf table now names `(*responder).enforceMaxDuration` and `startMaxDurationWorker`. `grep -n startMaxDurationWorker internal/plugins/ddos/local/register.go` shows the call inside `runEngine` |
| `0` keeps meaning no cap, so the fix cannot disarm the box | boundary | `TestLocalMaxDurationZeroMeansNoCap`: the rule survives 72 hours of the responder's clock |
| A re-characterizing attack cannot outlive its cap | functional | `TestLocalMaxDurationClockStartsOnTheFirstInstall`, red under discrimination cut 1 with the exact message that names the failure |
| The worker costs an unattacked box nothing and stops with its owner | resilience | `TestLocalMaxDurationIdleWorkerRemovesNothing` (zero firewall reconciles over many ticks) and `TestLocalMaxDurationWorkerStops` (the worker exits on context cancel) |
| An operator reaches the cap end to end | functional `.ci` | `test/plugin/ddos-local-max-duration.ci`, RUN in the QEMU guest on ze's runtime kernel: PASS in 9.7s, and FAIL with the drop still installed under the rebuilt cut that stops the worker reaching the cap. Both pasted above |
| The cap survives an operator's unrelated commit | functional `.ci` | `test/plugin/ddos-local-cap-survives-reload.ci`, RUN in the guest on 2026-09-09: PASS in 24.5s, and FAIL naming the orphaned rule under the rebuilt cut that deletes `adoptMitigation`. Both pasted in the Review Gate. Re-run green at 24.5s after the round 2 changes |
| A drop rule is never left in the kernel with nothing able to remove it | functional `.ci` | `test/plugin/ddos-local-config-removed.ci`, RUN in the guest on 2026-09-09: PASS in 3.1s, and FAIL under the rebuilt cut that deletes both withdraw arms, naming `table=ze_ddos-local chain=ingress rules=1` still in the kernel 40s after the block was deleted. Both pasted in the Review Gate |
| The same holds for the operator gesture that tells the plugin NOTHING | functional `.ci` | `test/plugin/ddos-parent-config-removed.ci`, RUN in the guest on 2026-09-09: PASS in 3.2s, and FAIL under the rebuilt cut that deletes the exit-path withdrawal, naming the same orphaned rule 40s after the parent block was deleted. The red carries `ddos-local plugin stopped` and NO reconfigure line, which is the delivery mechanism observed at runtime. Both pasted in the Review Gate |
| A responder a config apply retired can never put a rule back | functional (unit, kernel-read) | `TestLocalRetiredResponderInstallsNothing` and `TestLocalRetiredResponderDoesNotReclaimACarriedRule` read the firewall stub's install count, not the responder's snapshot, and both are red with the guard cut. The three sibling `.ci` tests were re-run in the guest against the same binaries: 4/4 PASS |
| An operator can stop a transit drop by committing `forward-mitigation false`, without losing an on-host one | functional (unit, kernel-read) | `TestLocalDisablingForwardMitigationRemovesTheForwardDrop` (red without the arm) and `TestLocalDisablingForwardMitigationLeavesTheIngressDrop` (red if the arm is widened) |
| An operator can stop an on-host drop by committing `response-level alert` | functional (unit, kernel-read) | `TestLocalLeavingEnforceRemovesTheDrop`: the firewall stub reports the withdraw, not the responder's own snapshot. Red before the arm existed |
| An operator reaches the announce limit end to end | functional `.ci` | `test/plugin/ddos-announce-rate-limit.ci`, RUN in the guest on 2026-09-09: PASS in 55.5s, and FAIL with a second announcement 18s into the window under the rebuilt limiter cut |
| Detection evaluations honor check-interval and persistence follows the feed | Source and inherited discrimination evidence | `detect/detector.go` `tick` folds `evalTicks`, saves on `tickNum % baselineSaveInterval`, and passes the interval peak to `applyTick`; `interval_test.go` and `ddos-timing-leaves.ci` record the corresponding RED/GREEN |
| Open incidents reach their configured stale timeout | Source and inherited functional proof | `observe/register.go` `runEngine` starts `startStaleSweep`; `store.go` `sweepStale` finalizes incidents; `ddos-timing-leaves.ci` keeps the flood running while reading an end time |
| Startup cleanup uses the configured backend and preserves other owners | Current race and Linux proof | `local/register.go` `runEngine` calls `clearStaleDropRule` inside initial `OnConfigure`; configured-backend overlay is RED, fixed local suite is PASS with race, and Linux 7.2 stale-table scenario is PASS 1/1 with both required markers |
| Restart timing is stated separately for packet and bandwidth detection | Current throwaway smoke proof | `detect/detector.go` `applyTick` and `baseline.go` `Ready`: cold BPS armed/detected at 391/393, full restore 1/3, partial 50-sample restore 341/343; cold PPS at 2x floor detects at 93 and exact 5x at 3 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None within AC-1 through AC-10 | Every acceptance criterion has a producing path and recorded proof; this closure adds no product deferral | Not applicable |
| FlowSpec responder state across config apply, outside this spec's local-cap and startup repair | The previously recorded FlowSpec lifecycle finding remains outside this closure; its source is unchanged | `plan/immediate/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md` |

## Review Gate

An independent review of commit `80ec03b03` (`feat(ddos): a local mitigation
that never clears now expires`) returned 1 BLOCKER, 2 ISSUE and 3 NOTE. The
reviewer could not write to this spec, so the findings are recorded here by the
session that cleared them. Each one was re-verified at its producing function
before any edit.

| # | Severity | Finding | Verified at | Status |
|---|----------|---------|-------------|--------|
| 1 | BLOCKER | A config apply orphans a live drop rule and disarms the cap | `p.OnConfigApply` (`internal/plugins/ddos/local/register.go`), `newResponder` and `enforceMaxDuration` (`responder.go`) | reproduced, FIXED: `replaceResponder` -> `(*responder).adoptMitigation` carries `active`, `target`, `installedAt` and the clock that stamped it; `test/plugin/ddos-local-cap-survives-reload.ci` red under the cut, green with it |
| 2 | ISSUE | `runEngine` discards the `exited` channel `startMaxDurationWorker` returns, so the plugin can return with a tick in flight | `runEngine` and `startMaxDurationWorker` (`internal/plugins/ddos/local/register.go`) | reproduced, FIXED: `runEngine` defers `cancel()` then `<-workerExited`, and the tests take the same stop through `runWorker` |
| 3 | ISSUE | `fixture06DDOSAnnounceRateLimit` bounds its second generation by an iteration count, not by the window it asserts inside | `fixture06DDOSAnnounceRateLimit` (`internal/test/fixture/plugin_fixture_06_ddos_linux.go`) | reproduced, FIXED: the loop runs to a deadline taken when the announce is observed, and `ddos-announce-rate-limit.ci` RAN in the guest, green and red under the limiter cut |
| 4 | NOTE | `(*responder).clock` carries an unreachable `r.now == nil` branch | `clock` and `newResponder` (`internal/plugins/ddos/local/responder.go`) | reproduced, FIXED: the branch is deleted and the field's comment states the invariant |
| 5 | NOTE | The 2026-09-04 row in `plan/journal/unwired-feature.md` says the worker is in the working tree and not in HEAD | `plan/journal/unwired-feature.md` line 19, against `git show HEAD:internal/plugins/ddos/local/register.go` | reproduced, FIXED: the row now names commit `80ec03b03` and says the worker is in HEAD |
| 6 | NOTE | `ddos-local-max-duration.ci` discriminates AC-6 only | the `.ci` config block against `enforceMaxDuration` and `setStatus` | reproduced, RECORDED below. AC-7 and AC-8 stay unit-only and each keeps its own recorded cut |

**Found while clearing finding 1, and NOT fixed here.** `ddos flowspec` has the
identical shape: `p.OnConfigApply` (`internal/plugins/ddos/flowspec/register.go`)
builds a fresh responder and nothing carries `active`, `target` or `announcedAt`,
so a config apply orphans a live FlowSpec announcement and neither
`enforceMaxDuration` nor the clear path can withdraw it again. The flowspec cap
is a different spec's leaf and this spec's goal does not depend on it, so it took
one row in `plan/journal/component-rebuilt-during-reload.md` dated 2026-09-08 and
nothing else (`ai/rules/rule-precedence.md`).

### 1 (BLOCKER). A config apply orphaned a live drop rule and disarmed the cap

**Reproduced.** `p.OnConfigApply` built a fresh responder through
`newResponder`, which leaves `active` false, `target` zero and `installedAt`
zero, and stored it in `activeResponder`. Nothing carried the outgoing
responder's state and nothing withdrew its table. The firewall registry is keyed
by TABLE NAME (`registerTables(tableName, ...)` in `applyMitigation`), so
`ze_ddos-local` stayed registered and the drop stayed in the kernel. Both
removal paths return on `!r.active`: `enforceMaxDuration` and `onCleared`. From
the apply onward the rule was bounded by neither the cap nor a clear, and
`show ddos local` reported no mitigation, because `status()` reads the new
responder's idle snapshot.

The `onCleared` arm pre-dates the commit. It is in scope because AC-6 is the
criterion that says a drop rule is bounded, and one ordinary operator action
reached the state where it is not (`ai/rules/completion.md`).

**Fix.** `(*responder).adoptMitigation` carries `active`, `target`, `installedAt`
and the clock that stamped it from the responder a config apply replaces into the
one that replaces it, and republishes the snapshot `status()` reads. It is called
from `replaceResponder`, the one function `OnConfigure` and `OnConfigApply` both
use, so a test over that function covers the path the operator takes.

The cap clock comes across unchanged. The operator committed an unrelated change,
the kernel rule is the same rule, so its age is the same age; resetting it would
let a box under attack renew its own cap on every commit. The clock comes with the
instant because an instant is only meaningful against the source that produced it.
The NEW `cfg` governs from there, so a reload that shortens
`max-mitigation-duration` applies the shorter cap to the rule already installed.

Both removal arms are closed by the one carry, and each has its own test:
`enforceMaxDuration` by `TestLocalMitigationSurvivesAConfigApply` and by the
`.ci`, `onCleared` by `TestLocalClearSurvivesAConfigApply`. That second test reads
the WITHDRAW out of the firewall stub rather than `status()`: an orphaning
responder publishes `active=false` having withdrawn nothing, so a `status()`
assertion passes over a drop rule the box is still enforcing. It did exactly that
on its first run, and the assertion was corrected before the fix landed.

### 2 (ISSUE). The worker's exit was not waited on

**Reproduced.** `startMaxDurationWorker` returns `exited` and its doc comment
says a caller that must know the worker is gone waits on it after the cancel.
`runEngine` discarded the return value and only `defer cancel()`d, which signals
and does not wait. So `runEngine` could return with a tick in flight inside
`enforceMaxDuration` -> `removeMitigation` -> `applyAll`, a netlink reconcile,
after the process supervisor had been told the engine released what it installed.

**Fix.** `runEngine` keeps the channel and waits on it: `cancel()` then
`<-workerExited`, in one deferred pair so no early return can skip the wait. The
deferred pair is registered after `defer activeResponder.Store(nil)`, so LIFO
stops the worker before the responder it reads is detached.

The unit tests carried the same defect and it is repaired with the same shape.
`runWorker(t)` returns a stop that cancels AND waits, and it is deferred after the
firewall stub's own restore so LIFO runs it first. Without that, a tick in flight
read `registerTables` and `applyAll` while the next test swapped them back, which
the race detector reported on the first run of the new tests.

### 3 (ISSUE). The announce-rate-limit fixture raced its own window

**Reproduced.** The second-generation loop was `for range 100` with a 250ms
wait and a 4000-packet blast each turn. The assertion inside it is that NO
announcement goes out, and that is true only inside the 60 seconds the limiter
states its budget over. Nothing tied the loop to that window, so a slower guest
crossed it, the limiter legitimately freed the budget, the second announcement
was correct, and the fixture reported a red against correct code.

**Fix.** The window is a named constant, `fixture06AnnounceWindow`, and the
second-generation loop runs to a deadline taken from the instant the first
announce is OBSERVED. The observation is at most one poll and one blast after the
announce really went out, so the deadline it computes is late by that much;
`fixture06AnnounceWindowMargin` absorbs the lag and leaves the asserted window
shorter than the real one, which is the safe direction. A run that reaches the
deadline with less than `fixture06SecondGenerationFlood` behind it fails naming
the overrun, rather than printing the refusal marker over a window that was never
really tested.

### 4 (NOTE). A dead defensive branch in `clock`

**Reproduced.** `newResponder` is the only constructor of `responder` in the
package (`grep 'responder{'` finds one hit, inside it), and it always sets
`now: time.Now`. The `r.now == nil` arm is unreachable, and it makes a
zero-value responder look serviceable.

**Fix.** The branch is gone. `clock` returns `r.now()`, and the `now` field's
doc comment states the invariant `newResponder` establishes, so a reader learns
where the guarantee comes from instead of meeting a branch that suggests it is
absent.

### 5 (NOTE). The journal row was made false by the commit it describes

**Reproduced.** The 2026-09-04 row said the local cap's worker was "in the
WORKING TREE and not yet in HEAD".
`git show HEAD:internal/plugins/ddos/local/register.go` carries
`startMaxDurationWorker`, and `git status` reports no modification under
`internal/plugins/ddos/`. The row was written before `80ec03b03` landed, and the
commit made it false.

**Fix.** The row's Fix column now names commit `80ec03b03` and says the worker,
the cap and the QEMU functional proof are in HEAD.

### 6 (NOTE). What `ddos-local-max-duration.ci` discriminates

**Reproduced by source, not by a guest run.** The `.ci` sets
`characterize-enable false`, so no `AttackCharacterized` is emitted and
`applyMitigation` runs once per generation. Cutting the AC-8 transition guard
(writing `installedAt` on every install instead of on the false-to-true edge)
therefore changes nothing the `.ci` can see. It sets
`max-mitigation-duration 5`, so cutting the AC-7 `<= 0` no-cap guard changes
nothing either: that guard is not on the path a non-zero cap takes.

**Recorded, not repaired.** The `.ci` proves AC-6 and only AC-6. AC-7 and AC-8
stay unit-only, proven by `TestLocalMaxDurationZeroMeansNoCap` and
`TestLocalMaxDurationClockStartsOnTheFirstInstall`, each red under its own
recorded cut above. Neither behavior needs a kernel to be wrong, so a guest run
would buy discrimination the unit cuts already have.


### TDD evidence

**RED, finding 1, against the apply path as `80ec03b03` shipped it.**
`replaceResponder` was extracted first with NO behavior change, so the red below
is behavioral rather than a build failure. Note the second test: it passed, over
a `status()` assertion an orphaning responder also satisfies, which is why it now
reads the withdraw out of the firewall.

```
=== RUN   TestLocalMitigationSurvivesAConfigApply
    max_duration_test.go:293: a config apply left the drop rule in the kernel and the new responder reporting no mitigation: active=false target=invalid Prefix
--- FAIL: TestLocalMitigationSurvivesAConfigApply (0.00s)
=== RUN   TestLocalClearSurvivesAConfigApply
--- PASS: TestLocalClearSurvivesAConfigApply (0.00s)
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.621s
```

**RED again, with the clear test corrected, under the cut that deletes the carry
from `replaceResponder`.** That cut is byte-for-byte what `OnConfigApply` did
before this work, so both arms are shown red against the shipped behavior:

```
=== RUN   TestLocalMitigationSurvivesAConfigApply
    max_duration_test.go:331: a config apply left the drop rule in the kernel and the new responder reporting no mitigation: active=false target=invalid Prefix
--- FAIL: TestLocalMitigationSurvivesAConfigApply (0.00s)
=== RUN   TestLocalClearSurvivesAConfigApply
    max_duration_test.go:374: an AttackCleared after a config apply left the drop rule installed in the kernel
--- FAIL: TestLocalClearSurvivesAConfigApply (0.00s)
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.650s
```

**GREEN, after `adoptMitigation`.**

```
=== RUN   TestLocalMitigationSurvivesAConfigApply
INFO ddos-local: drop rule installed target=10.0.0.1/32 hook=ingress phase=detected
INFO ddos-local: max-mitigation-duration reached, removing the drop rule target=10.0.0.1/32 seconds=60
INFO ddos-local: drop rule removed target=10.0.0.1/32
--- PASS: TestLocalMitigationSurvivesAConfigApply (0.00s)
=== RUN   TestLocalClearSurvivesAConfigApply
INFO ddos-local: drop rule installed target=10.0.0.1/32 hook=ingress phase=detected
INFO ddos-local: drop rule removed target=10.0.0.1/32
--- PASS: TestLocalClearSurvivesAConfigApply (0.00s)
ok  	github.com/ze-software/ze/internal/plugins/ddos/local	1.560s
```

The whole subsystem is green at `-race -count=2`:
`go test -race -count=2 ./internal/plugins/ddos/...` -> `ok` for detect, flowspec,
flowtriq, local, observe.

### The reload proof, run in the QEMU guest (2026-09-09)

`test/plugin/ddos-local-cap-survives-reload.ci` was RUN on ze's runtime kernel,
through `./le qemu run kernel tmp/kernel/build/vmlinuz packages "iproute2 libcap"`
with the guest-side `ze-test bgp plugin ddos-local-cap-survives-reload`. The probe
installs a drop under an unbroken flood, rewrites `ze-bgp.conf` with one unrelated
`ddos local` leaf changed, SIGHUPs the daemon, and then watches the kernel and
`show ddos local` together.

**RED, under the cut that deletes the carry, with the guest binary REBUILT so the
cut reached the daemon:**

```
3.2s     1/1  FAIL  221  ddos-local-cap-survives-reload
INFO ddos-local: drop rule installed  target=127.0.0.10/32 hook=ingress phase=detected
WARN RELOAD-CAP-INSTALLED 127.0.0.10 (sent 32000)
INFO ddos-local: reconfigured  response-level=enforce forward-mitigation=false
ERROR ZE-OBSERVER-FAIL: the config apply orphaned the drop for 127.0.0.10: the kernel
  still holds the rule while show ddos local reports no mitigation, so neither
  max-mitigation-duration nor an AttackCleared can remove it again (sent 40000 packets):
table=ze_ddos-local chain=ingress rules=1
QEMU VM: FAIL (exit code 1)
```

The `ddos-local: reconfigured` line between the install and the failure is what
says the apply really ran, so the red is the orphan and not a reload that never
landed.

**GREEN, with the carry:**

```
24.5s    1/1  PASS  221  ddos-local-cap-survives-reload
pass  1/1  100.0%  24.5s
QEMU VM: PASS
```

### The AC-2 functional proof, run in the QEMU guest (2026-09-09)

`test/plugin/ddos-announce-rate-limit.ci` RAN. The build blocker Work Not Done
recorded is gone: `GOOS=linux GOARCH=amd64 go vet ./internal/test/fixture/
./internal/plugins/ddos/...` exits 0 in this tree, and the guest binaries
cross-compile against a `go -overlay` that presents HEAD for other sessions'
in-flight files and the working tree for this work's own.

**GREEN.**

```
55.5s    1/1  PASS  211  ddos-announce-rate-limit
pass  1/1  100.0%  55.5s
QEMU VM: PASS
```

**RED, under the cut that stops `announce` consulting the limiter, with the guest
binary REBUILT so the cut reached the daemon:**

```
21.4s    1/1  FAIL  211  ddos-announce-rate-limit
INFO ddos-flowspec: announced  target=203.0.113.9/32 action=discard reason="blackhole-fallback (critical)"
WARN ANNOUNCED 203.0.113.9 (sent 36000)
INFO ddos-flowspec: announced  target=203.0.113.9/32 action=discard reason="blackhole-fallback (critical)"
ERROR ZE-OBSERVER-FAIL: a second announcement went out 18s into the announce-rate-limit
  window (sent 60000 packets)
QEMU VM: FAIL (exit code 1)
```

"18s into the window" is the deadline-bounded loop reporting its own position, so
finding 3's repair is exercised by the same run. Both cuts were reverted, both
binaries rebuilt, and `grep -rn MUTATION-APPLIED internal/plugins/ddos internal/test test/`
returns nothing.

### Round 2 (independent, over commit `4008c0c98`)

Scope: the fixes round 1 drove, and what they newly touched. 1 BLOCKER, 3 ISSUE,
3 NOTE. Every finding below was read at its producing function.

**Round 1 re-check.** Findings 2, 3, 4, 5 and 6 are fixed as the table above
records, verified at `runEngine` (`register.go:260-264`), `runWorker`
(`max_duration_test.go`), `fixture06DDOSAnnounceRateLimit`
(`internal/test/fixture/plugin_fixture_06_ddos_linux.go`), `(*responder).clock`
(`responder.go:312-317`) and the 2026-09-04 row in
`plan/journal/unwired-feature.md`. Finding 1 is fixed for the path it names:
`newResponder` has exactly ONE non-test caller in the package,
`replaceResponder` (`register.go:119`), and `activeResponder.Store` has two
sites, `replaceResponder` and the `defer` in `runEngine`, so no other path
builds or publishes a responder. The ordering is right: `newResponder` ->
`adoptMitigation` -> `Store`, so a worker tick cannot observe a constructed but
unadopted responder.

**Deadline of ISSUE 3, checked at the producer.** `announceLimiter.allow`
(`internal/plugins/ddos/flowspec/responder.go:188`) is a sliding window over
`announceWindow`, evicting entries older than `now - 60s`. The window is
therefore relative to the announce instant, which is the event `spent` takes,
late by the observation lag and covered by `fixture06AnnounceWindowMargin`. The
"passes for the wrong reason" arm is closed OUTSIDE the fixture:
`test/plugin/ddos-announce-rate-limit.ci:156` expects
`ddos-flowspec: announce-rate-limit reached, not announcing`, which is the
in-run witness that the responder was ASKED. No finding.

#### 7 (BLOCKER). Removing the `ddos local` config section orphans a live drop rule, with nothing left to bound it

`runEngine` (`internal/plugins/ddos/local/register.go:126`) has no withdraw on
its return path. Its three defers are `p.Close`, `activeResponder.Store(nil)`
and the new `cancel(); <-workerExited`. None calls `registerTables(tableName,
nil)`.

`Server.autoStopForRemovedConfigPaths`
(`internal/component/plugin/server/startup_autoload.go:406`) stops a plugin
whose config root was removed, and `ddos-local` declares
`ConfigRoots: []string{"ddos/local"}`. The plugin is IN-PROCESS
(`internal/component/plugin/inprocess.go:127` calls `reg.RunEngine`), so it
shares the daemon's `tableRegistry` (`internal/component/firewall/registry.go`),
which is keyed by owner name. `firewall.FlushAllTables` runs only from the
firewall ENGINE's own shutdown (`internal/component/firewall/engine.go:457`),
not from a plugin stop.

So: an operator removes `ddos { local { ... } }` while a drop is live. The
engine stops, `ze_ddos-local` stays registered, the nftables drop stays in the
kernel, the responder is nil and the cap worker has exited. Neither
`enforceMaxDuration` nor `onCleared` exists to reach it any more. This is the
round-1 BLOCKER's shape on the sibling path, and it is strictly worse: the
fixed one left a rule the new responder could not see, this one leaves a rule
NO responder will ever exist to see.

The sibling plugin names this exact case and handles it: `runEngine`
(`internal/plugins/copp/register.go:172-177`) states that clean-shutdown
teardown is central "Config removal while running still withdraws via
OnConfigApply -> applyCoppPolicy(nil)". `ddos-local` has no equivalent, because
`parseSections` returns `DefaultConfig()` for a missing section and
`replaceResponder` then ADOPTS the live rule into a responder that is about to
be discarded.

The commit's own documentation is made false by it. `docs/guide/ddos-mitigation.md`
now says "A `commit` that changes any `ddos local` leaf while a drop rule is
live keeps the rule and its cap." A `commit` that removes the block keeps the
rule and loses the cap.

Read-verified at the four producing functions above; NOT reproduced at runtime,
and the reproduction is the next step (a `.ci` shaped like
`ddos-local-cap-survives-reload.ci` whose reload writes a config with the
`ddos local` block deleted).

#### 8 (ISSUE). Nothing in this change discriminates the `installedAt` half of the carry

Cut `r.installedAt = installedAt` to `r.installedAt = r.clock()` in
`(*responder).adoptMitigation` (`responder.go:150`). Both new tests stay GREEN.

`TestLocalMitigationSurvivesAConfigApply` (`max_duration_test.go`) installs at
T0 with a 60s cap, advances 10s, applies, then advances 61s. Under the cut the
adopted stamp is T0+10, and T0+71 minus T0+10 is 61s, still past the cap, so
the rule expires and the test passes. `ddos-local-cap-survives-reload.ci` is the
same: the reload lands seconds after the install and the probe allows 40s for a
20s cap, so 20s-from-install and 20s-from-reload are indistinguishable to it.

The test's own `VALIDATES` line says "it counts from the FIRST install", the
`adoptMitigation` doc comment says "Resetting it would let a box under attack
renew its own cap on every unrelated commit", and the guide paragraph added in
this commit says "the cap keeps counting from the FIRST install". Three claims
wider than any assertion (`ai/rules/evidence.md`).

Fix: advance 51s rather than 61s after the apply. That is 61s from the first
install and 51s from the apply, so the correct carry expires and the reset
stamp does not.

#### 9 (ISSUE). The `.ci` probe's orphan arm can false-red on correct code

`fixture06DDOSLocalCapSurvivesReload`
(`internal/test/fixture/plugin_fixture_06_ddos_linux.go`) polls the kernel
first and the daemon second:

```go
live, _, state, stateErr := fixture06DDOSDropState(victim)
row, showErr := p05ShowMap(ctx, plugin, "show ddos local")
if live && row["active"] == false { orphan = state; return true }
```

`(*responder).removeMitigation` (`responder.go:356`) removes the KERNEL rule
first (`registerTables(tableName, nil)` then `applyAll()`) and publishes
`setStatus(false, ...)` after. So a cap that fires between the two reads gives
`live=true, active=false` on correct code, and the probe returns the orphan
message: "the config apply orphaned the drop ... so neither
max-mitigation-duration nor an AttackCleared can remove it again". A false red
under the most alarming wording the fixture owns. The window is one show-RPC
round trip inside a 250ms poll, and the cap fires exactly once during the
40s poll, so it is small and real.

Fix: swap the two reads. With `show` read first, correct code's intermediate
state is `active=true, live=false`, which the orphan arm does not match, and
the false positive is gone by construction rather than by timing.

#### 10 (ISSUE). The carry keeps a drop rule alive under `response-level alert`

`adoptMitigation` carries `active` unconditionally, and neither
`enforceMaxDuration` (`responder.go:333`) nor `onCleared` (`responder.go:302`)
reads `cfg.ResponseLevel`. `alert` is the documented "detect and report, do not
block" mode (`applyMitigation`, `responder.go:189`).

So an operator who switches `response-level enforce` to `alert` mid-incident,
which is what an operator does to un-blackhole a victim, keeps the drop in the
kernel until the cap expires: up to `max-mitigation-duration`, whose default is
3600. `show ddos local` reports it as active the whole time, so the operator can
see it and cannot make it stop.

This is better than the pre-fix behavior, where the rule persisted forever
invisibly. It is still the wrong answer to an explicit operator instruction, it
is new code deciding it, and no test and no doc line covers it. The repair is in
`replaceResponder`: adopt when the new `cfg.ResponseLevel` is `responseEnforce`,
and withdraw through the outgoing responder when it is not.

#### 11 (NOTE). A live tick on the outgoing responder can make the new one over-report

`adoptMitigation` reads `prev` under `prev.mu` and returns; `replaceResponder`
publishes with `activeResponder.Store(r)` afterwards. Between those two,
`activeResponder` still points at `prev`, so the cap worker can tick on `prev`
and remove the rule, and an `onCleared` dispatched before `unsubscribe()` can do
the same. The new responder is then published with `active=true` over a kernel
with no rule.

It over-reports and never under-reports, so it cannot orphan anything: with a
cap set the next tick removes nothing and clears the status within a second.
With `max-mitigation-duration 0` it persists until the next attack cycle, and
`show ddos local` names a drop that is not installed. Closing it costs one line:
take `prev.mu` across the `Store` rather than releasing it first.

#### 12 (NOTE). The `<-workerExited` wait states no bound

`runEngine` (`register.go:261-264`) waits with no deadline. The bound is real
but transitive: a tick in flight sits in `enforceMaxDuration` ->
`removeMitigation` -> `applyAll`, which takes the process-wide `reconcileMu`
(`internal/component/firewall/registry.go:85`) behind
`firewall.MaxBackendDeadline`, 60 seconds
(`internal/component/firewall/backend.go`). Worst case is one queued reconcile
per firewall owner. The comment says why the wait is owed and not what bounds
it, which `docs/contributing/ze-go-style.md` "A limit on everything" asks for in
one sentence.

#### 13 (NOTE). `adoptMitigation` re-opens the write path round 1's NOTE 4 closed

Round 1 deleted the `r.now == nil` branch in `clock` on the ground that
"newResponder is the only constructor and always sets it". `adoptMitigation`
now writes `r.now = now` from `prev` with no check, so `now` is settable from
outside the constructor and a nil `prev.now` would propagate to a `clock()` with
no guard. No caller can produce it today, since `prev` always comes from
`newResponder`. In production the copy is also a no-op, because both sides hold
`time.Now`: it is observable only through a test double. Worth one line on the
field comment saying `adoptMitigation` is the second writer and what keeps it
non-nil.

#### Round 2 disposition

Every finding was re-verified at its producing function before any edit. The two
findings whose diagnosis did not survive that check are marked, with what the
producer actually says.

| # | Severity | Status |
|---|----------|--------|
| 7 | BLOCKER | reproduced AT RUNTIME, FIXED. `replaceResponder` withdraws through the OUTGOING responder when the delivered config no longer carries a `ddos local` section. `test/plugin/ddos-local-config-removed.ci` is red in the guest with the withdraw cut, naming the rule left in the kernel, and green with it. The mechanism is NOT the one the finding names: the removal DOES reach the plugin, as an empty body for the removed root |
| 8 | ISSUE | reproduced, FIXED, and the proposed repair would have been wrong. `movableClock` based its clock on `time.Now()`, which is what made the carry undiscriminated; `advance` stores absolutely, so advancing 51s rather than 61s would have failed the test against CORRECT code. The base is now an hour behind the wall clock and both reset cuts go red |
| 9 | ISSUE | reproduced by source, FIXED. `fixture06DDOSLocalCapSurvivesReload` reads the daemon first and the kernel second, so correct code's intermediate state matches neither arm |
| 10 | ISSUE | reproduced, FIXED. A commit that leaves `enforce` withdraws through the outgoing responder, with its own log line. `TestLocalLeavingEnforceRemovesTheDrop` reads the withdraw out of the firewall |
| 11 | NOTE | reproduced, FIXED, and the proposed one-line repair does not close it. Holding `prev.mu` across the `Store` only DELAYS the late tick: it still finds `prev.active` true and still removes the rule. Adoption now TRANSFERS ownership, so both removal paths on `prev` return on `!active`. `TestLocalAdoptionTakesOwnershipFromTheOldResponder` |
| 12 | NOTE | FIXED. The `<-workerExited` comment names the bound: `applyAll` takes the process-wide `reconcileMu` and every backend operation under it runs under `firewall.MaxBackendDeadline`, 60 seconds, so the worst case is one queued reconcile for each firewall owner |
| 13 | NOTE | FIXED. The `now` field's comment names `adoptMitigation` as its second writer and says the copy comes from a `prev` that `newResponder` built, so the non-nil invariant survives it |

**Finding 7's mechanism, corrected at the producer.** The finding says
`runEngine` returns with no withdraw and that no config-apply reaches the plugin.
The first half is true and the second is not, for the case it describes.
`(*Server).reloadConfig` (`internal/component/plugin/server/reload.go`) puts
ddos-local in `affected` because `rootHasChanges` matches the removed
`ddos/local` key, and `ExtractConfigSubtree` returns nil for a root the new tree
no longer holds, so the plugin receives `{Root: "ddos/local", Data: "{}"}` and
runs verify and apply over it BEFORE `stopCollectedProcesses` stops it. What
`ParseConfig` then did was read that empty body as DefaultConfig and report
nothing unusual, so `replaceResponder` adopted the live rule into a responder the
next line discarded. The fix is therefore at the config boundary, which is where
the sibling plugin puts it, and not on the engine's exit path.

**Why the withdrawal is on the section and not on the response-level.** A removed
section parses to `DefaultConfig`, whose `response-level` is `alert`, so finding
10's withdraw would have covered finding 7 by accident. That is the shape
`docs/contributing/ze-go-style.md` names as the sharpest zero-value failure: a
test asking WHICH value to use also answering WHETHER the action is permitted,
until a legitimate value arrives and the guard vanishes with no line deleted and
no test red. `ParseConfig` now reports whether the section was present, and each
withdrawal has its own arm, its own reason and its own test.

**Two finds, journalled and not fixed here** (`ai/rules/rule-precedence.md`), both
as rows in `plan/journal/component-rebuilt-during-reload.md` dated 2026-09-09:

- `copp` has the defect it was cited as the cure for. `OnConfigApply` returns
  early on `newPolicy == nil`, and `nil` means both "nothing staged" and "the
  section is gone", so deleting `control-plane-protection` leaves copp's table in
  the kernel. The comment at the tail of `runCoppPlugin` states the opposite.
  `internal/plugins/copp/**` was read-only for this work.
- Deleting a PARENT block stops a child-root plugin without telling it.
  `diffMapsRecursive` records one removed key for a whole subtree, so removing
  `ddos { ... }` entire puts only `ddos` in `diff.Removed`; `rootHasChanges` does
  not match an ancestor, so ddos-local is not in `affected` and gets no apply,
  while `parentRemoved` still stops it. The producer is `internal/component/**`,
  outside this work's file scope.

**Measured while proving finding 7, and it is why the `.ci` carries a copp
block.** ddos-local declares `firewall` as a dependency, so stopping ddos-local
leaves firewall with no dependents and the reload stops it too
(`Server.collectOrphanedDependencies`). The firewall engine flushes every
ze-owned table on its own clean shutdown, which removes the orphaned drop by
accident. The first guest run of the new `.ci` therefore PASSED against code that
withdraws nothing, and only the log-line expectation failed. A
`control-plane-protection` block keeps a second dependent on firewall, which is
what a real box has, and the orphan is then left where an operator meets it.

### Round 2 TDD evidence

**RED, finding 8, under the cut that re-stamps `installedAt` at adoption, with
the clock base still `time.Now()`.** Both tests stayed GREEN, which is the
finding reproduced:

```
=== RUN   TestLocalMitigationSurvivesAConfigApply
--- PASS: TestLocalMitigationSurvivesAConfigApply (0.00s)
=== RUN   TestLocalClearSurvivesAConfigApply
--- PASS: TestLocalClearSurvivesAConfigApply (0.00s)
ok  	github.com/ze-software/ze/internal/plugins/ddos/local	0.437s
```

**RED, finding 8, with the clock base an hour behind the wall clock.** Both cut
ORDERS are shown, because they stamp from different clocks: `r.installedAt =
r.clock()` written after `r.now = prev.now` reads the carried clock, and written
before it reads the real one. Each was applied on its own and each is red:

```
=== RUN   TestLocalMitigationSurvivesAConfigApply
    max_duration_test.go:354: the cap never fired after a config apply: the drop rule outlived max-mitigation-duration with the flood still running
--- FAIL: TestLocalMitigationSurvivesAConfigApply (2.00s)
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	2.419s
```

**RED, findings 7, 10 and 11, before the fix.** `replaceResponder` had taken its
new `configured` parameter and ignored it, so this is behavioral rather than a
build failure:

```
=== RUN   TestLocalRemovingTheSectionRemovesTheDrop
    max_duration_test.go:434: removing the ddos local section left the drop rule installed in the kernel, with the plugin about to stop
    max_duration_test.go:437: the responder still reports a live mitigation after the section was removed
--- FAIL: TestLocalRemovingTheSectionRemovesTheDrop (0.00s)
=== RUN   TestLocalLeavingEnforceRemovesTheDrop
    max_duration_test.go:470: a commit of response-level alert left the drop rule installed in the kernel
    max_duration_test.go:473: the alert-mode responder still reports a live mitigation
--- FAIL: TestLocalLeavingEnforceRemovesTheDrop (0.00s)
=== RUN   TestLocalAdoptionTakesOwnershipFromTheOldResponder
INFO ddos-local: max-mitigation-duration reached, removing the drop rule target=10.0.0.1/32 seconds=60
    max_duration_test.go:508: a cap tick on the replaced responder removed the rule the new responder now owns
    max_duration_test.go:514: a clear still in flight on the replaced responder removed the rule the new responder now owns
--- FAIL: TestLocalAdoptionTakesOwnershipFromTheOldResponder (0.00s)
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.620s
```

**GREEN, all eighteen `TestLocal*` cases at `-race -count=3`:**

```
      3 --- PASS: TestLocalAdoptionTakesOwnershipFromTheOldResponder (0.00s)
      3 --- PASS: TestLocalClearSurvivesAConfigApply (0.00s)
      3 --- PASS: TestLocalLeavingEnforceRemovesTheDrop (0.00s)
      3 --- PASS: TestLocalMitigationSurvivesAConfigApply (0.00s)
      3 --- PASS: TestLocalRemovingTheSectionRemovesTheDrop (0.00s)
      1 ok  	github.com/ze-software/ze/internal/plugins/ddos/local	2.146s
```

### The config-removal proof, run in the QEMU guest (2026-09-09)

`test/plugin/ddos-local-config-removed.ci` was RUN on ze's runtime kernel,
through `./le qemu run kernel tmp/kernel/build/vmlinuz packages "iproute2 libcap"`
with the guest-side `ze-test bgp plugin ddos-local-config-removed`. The probe
installs a drop under an unbroken flood, commits a config with the `ddos local`
block deleted, and reads the kernel back. The cap is 3600 and the flood never
stops, so neither the cap nor an `AttackCleared` can account for a removal.

**RED, under the cut that deletes both withdraw arms from `replaceResponder`,
which is byte-for-byte what the apply did before this work, with the guest binary
REBUILT so the cut reached the daemon:**

```
48.2s    1/1  FAIL  223  ddos-local-config-removed
INFO ddos-local: drop rule installed  target=127.0.0.11/32 hook=ingress phase=detected
WARN CONFIG-REMOVED-INSTALLED 127.0.0.11 (sent 32000)
INFO ddos-local: reconfigured  response-level=alert forward-mitigation=false
INFO ddos-local plugin stopped
WARN plugin exited and is not to be started again, so ze continues without it  plugin=ddos-local
ERROR ZE-OBSERVER-FAIL: deleting the ddos local config block left the drop for 127.0.0.11 in the
  kernel 40s later, under a flood that never stopped and a cap of 3600 seconds: the plugin that
  installed it is stopped, so no responder and no cap worker is left to remove it and the box
  blackholes the victim for the life of the daemon (sent 672000 packets):
table=ze_ddos-local chain=ingress rules=1
QEMU VM: FAIL (exit code 1)
```

The `ddos-local plugin stopped` line between the install and the failure is what
says the removal really landed, so the red is the orphan and not a reload that
never ran.

**GREEN, with the withdraw:**

```
3.1s     1/1  PASS  223  ddos-local-config-removed
pass  1/1  100.0%  3.1s
QEMU VM: PASS
```

**The two sibling `.ci` tests were re-run in the guest against the same binaries,
because the ownership transfer and the alert withdraw are on their path too:**

```
24.5s    1/1  PASS  222  ddos-local-cap-survives-reload
9.5s     1/1  PASS  224  ddos-local-max-duration
```

All three green runs above are against binaries built from the FINAL code,
rebuilt after the withdrawal and the carry were folded into one critical section
over `prev.mu`.

The cut was reverted, both guest binaries rebuilt, and
`grep -rn MUTATION-APPLIED internal/plugins/ddos internal/test test/` returns
nothing.


### Round 3

The reviewer's report, reproduced as received. It is fenced rather than inlined
because it cites four producers by line number, which `ai/rules/writing.md` and
the write gate allow only as quoted output. Nothing in it is edited.

```
### Round 3 (independent, over commit `b11fa5c23`)

Scope: the fixes round 2 made and what they newly touched. 2 BLOCKER, 3 ISSUE,
2 NOTE. Every finding below was read at its producing function.

**Round 2 re-check, at the producers.** The commit's own diagnosis is CORRECT for
the case it names. `(*Server).reloadConfig`
(`internal/component/plugin/server/reload.go:236`) builds each affected plugin's
sections from `rootHasChanges(diff, root)` and, where
`ExtractConfigSubtree(newTree, root)` is nil, appends
`rpc.ConfigSection{Root: root, Data: "{}"}`; `stopCollectedProcesses` runs AFTER
`runTxCoordinator`, so the empty body really does reach ddos-local before the
stop. `ParseConfig` (`config.go`) now returns that as `found=false`.
Findings 9, 12 and 13 are fixed as recorded. Finding 8's clock repair
discriminates: with `base = time.Now().Add(-time.Hour)` and an ABSOLUTE
`advance`, a re-stamp written before `r.now = prev.now` gives an age of
`61s - 1h` and one written after gives `51s` against a 60s cap, so both cut
orders are red in `TestLocalMitigationSurvivesAConfigApply`. The `.ci`'s copp
block is load-bearing and the guest RED proves it: the run shows
`ddos-local plugin stopped` and `rules=1` forty seconds later, so the firewall
engine stayed up and the flush accident is gone. The two sibling `.ci` files do
NOT share that blind spot, because neither removes `ddos local`, so ddos-local is
never stopped and firewall never becomes dependent-less.

#### 14 (BLOCKER). An event dispatched before the withdraw re-installs the drop through the retired responder, and nothing is left to remove it

`(*Server).dispatchEngineEvent` ->
`(*engineEventSubscribers).dispatch`
(`internal/component/plugin/server/engine_event.go:100-111`) SNAPSHOTS the
handler list under `RLock`, releases the lock, and then invokes each handler.
`unregister` after that snapshot does not stop the invocation, and handlers fire
synchronously on the EMITTING goroutine (the detector's), which is not the
goroutine running `OnConfigApply`.

`replaceResponder` (`internal/plugins/ddos/local/register.go:145`) holds
`prev.mu` across `withdrawMitigation` -> `removeMitigation` -> `applyAll()`, a
netlink reconcile. `(*responder).applyMitigation` (`responder.go:227`) is
UNCONDITIONAL on ownership: it checks `r.cfg.ResponseLevel`, `suppressMitigation`,
the hook and the prefix, and nothing else. `prev.cfg` is the OLD config, which is
`enforce`.

Failure scenario, concrete:

1. A flood on `127.0.0.11` is detected. `A.onDetected` installs `ze_ddos-local`,
   `A.active = true`.
2. `ddos-detect` emits `AttackCharacterized` for the same generation.
   `dispatch` snapshots `[A.onCharacterized]` and calls it. It blocks on `A.mu`.
3. The operator commits `delete ddos local`. `OnConfigApply` calls
   `unsubscribe()` (too late for the snapshot above), then `replaceResponder`,
   which takes `A.mu`, withdraws, transfers ownership, `activeResponder.Store(B)`,
   and releases `A.mu`.
4. `A.onCharacterized` now runs: `A.cfg.ResponseLevel == enforce`, so
   `applyMitigation` calls `registerTables("ze_ddos-local", [table])` and
   `applyAll()`. The drop is BACK in the kernel and `A.setStatus(true, ...)`.
5. `stopCollectedProcesses` stops the plugin. `A` is unreachable (not in
   `activeResponder`, unsubscribed), `B` is idle, the cap worker has exited, and
   `RegisterTables` (`internal/component/firewall/registry.go:97`) still holds
   the owner. `FlushAllTables` runs only from the firewall engine's own clean
   shutdown (`internal/component/firewall/engine.go:458`), and copp keeps that
   engine alive.

The victim is blackholed for the life of the daemon: the exact outcome finding 7
exists to prevent, reached through the code that fixes it. The window is the
netlink reconcile inside the critical section, so it is small; the consequence is
permanent and silent, and `ai/rules/planning.md` "PLAUSIBLE by default" is
explicit that a reachable concurrency race is not discarded for being narrow.

This is NEW. Before this commit `replaceResponder` never withdrew, so a late
install landed on a rule the new responder had adopted and could still remove.

`adoptMitigation`'s doc comment asserts the property this breaks: "leaves prev
owning nothing, so the rule has exactly one owner at every instant". A retired
`prev` can still CREATE a rule, so the claim is wider than the code
(`ai/rules/evidence.md`).

Shape of the fix: ownership has to be a state `applyMitigation` reads, not only a
state the removal paths read. A `retired bool` guarded by `mu`, set where
`prev.active = false` is set today and checked at the top of `applyMitigation`,
closes both the install and the two removals with one field.

#### 15 (BLOCKER). Deleting the PARENT `ddos` block still orphans the drop rule, and the commit and the guide both say it does not

`diffMapsRecursive` (`internal/component/config/diff.go:39`) records ONE removed
key for a whole subtree: removing `ddos { ... }` entire puts `"ddos"` in
`diff.Removed` and nothing else. `rootHasChanges`
(`internal/component/plugin/server/reload.go:402`) matches only `k == root` or
`strings.HasPrefix(k, root+"/")`, so `"ddos"` matches neither `"ddos/local"` nor
`"ddos/local/"`: ddos-local is not in `affected`, receives NO verify and NO
apply, and `replaceResponder` is never called. `parentRemoved`
(`internal/component/plugin/server/startup_autoload.go:520`) DOES match, so
`collectProcessesForRemovedConfigPaths` stops the plugin anyway.

`runEngine` (`register.go:171`) still has no withdraw on its return path. Its
defers are `p.Close`, `activeResponder.Store(nil)` and `cancel(); <-workerExited`.
Nothing calls `registerTables(tableName, nil)`, and nothing else unregisters the
owner: `RegisterTables` is the only writer of `tableRegistry.owners` and
`FlushAllTables` runs only from the firewall engine's clean shutdown.

Failure scenario: an operator mitigating an attack decides to turn DDoS handling
off and commits `delete ddos` rather than `delete ddos local`. The drop for the
victim stays in the kernel, bounded by nothing, for the life of the daemon. That
is finding 7 verbatim, on the more likely of the two operator gestures.

Round 2's finding named `runEngine`'s exit path. The commit calls that diagnosis
"close but wrong" and moves the guard to the config boundary, which covers
strictly LESS: the config boundary covers the deliveries the plugin receives, and
the exit path covers every stop, including this one. `internal/plugins/copp/register.go:172`
is cited as the precedent, and copp is the same shape, so the precedent does not
settle it.

Two statements in the diff are made false by this:

- the commit subject, "deleting the config block takes the drop rule with it";
- `docs/guide/ddos-mitigation.md`, table row "The `ddos local` block deleted |
  REMOVED at once", which an operator reads as covering `delete ddos`.

The journal row in `plan/journal/component-rebuilt-during-reload.md` records the
DELIVERY defect, which is real and belongs to `internal/component/**`. It does
not license leaving the plugin's own rule orphaned: AC-6's premise, as the new
`.ci` states it, is that a drop rule ddos local installs is always reachable by
something that can remove it, and this path defeats it
(`ai/rules/completion.md`, a defect that blocks the goal is fixed).

#### 16 (ISSUE). `alert` then back to `enforce` leaves the box unprotected for the rest of the attack

`characterizeAndEmit` (`internal/plugins/ddos/detect/characterize.go:97`) "runs
once per attack" and both `emitDetected` and `emitCharacterized` are gated on
`genCurrent(gen)`, so `AttackDetected` and `AttackCharacterized` fire once per
attack GENERATION. ddos-local subscribes to those two and to `AttackCleared`
(`register.go` `subscribe`) and to nothing else.

So: enforce, drop installed; the operator commits `response-level alert` and the
new withdraw arm removes the rule; the operator commits `enforce` again while the
same flood is still running. No further `AttackDetected` or `AttackCharacterized`
is emitted for that generation, so `applyMitigation` is never called and the drop
does not come back until the attack clears and a new generation starts.

Before this commit the rule was carried, so the round trip left it in place. The
new behavior is defensible, and it is undocumented: `docs/guide/ddos-mitigation.md`
says `response-level alert` means "REMOVED at once" and says nothing about what
returning to `enforce` does. No test covers the round trip.

#### 17 (ISSUE). `pendingConfigured` survives a rolled-back transaction, so the apply-without-verify guard can now withdraw a live rule

`OnConfigApply` (`register.go:271`) clears `pendingCfg` and `pendingConfigured`;
`OnConfigRollback` (`register.go:307`) is `func(_ string) error { return nil }`
and clears neither. A transaction whose verify reached this plugin and whose
apply never did leaves both staged.

The `cfg == nil` branch exists to FAIL CLOSED on an apply that arrives without a
verify, and its comment says so. A stale non-nil `pendingCfg` defeats it: the
guard passes and the stale pair is applied. Round 2 widened what that costs. The
stale pair from a rolled-back removal is `(DefaultConfig, false)`, so the apply
now takes the `!configured` arm and WITHDRAWS a live drop rule, where before it
only applied a stale config.

Precondition is a coordinator that applies without verifying, which the comment
calls a protocol violation rather than a normal state, so this is an ISSUE rather
than a BLOCKER. The fix is one line: clear both in `OnConfigRollback`, which is
also what makes the guard's own comment true.

#### 18 (ISSUE). `forward-mitigation false` is the other commit that says "stop dropping", and it does not withdraw

Finding 10's argument is that a commit saying the box must stop dropping must
withdraw, because neither `enforceMaxDuration` nor `onCleared` reads the leaf and
the rule then outlives the instruction by up to `max-mitigation-duration`. The
same argument holds for `ForwardMitigation`, and `replaceResponder` does not
apply it.

`hookForDirection` (`responder.go:324`) returns `(0, false)` for a remote victim
when `!r.cfg.ForwardMitigation`, and `applyMitigation` then logs "remote victim,
forward-mitigation disabled, deferring to flowspec" and RETURNS without removing
anything. So a FORWARD-hook drop installed under `forward-mitigation true`
survives a commit of `forward-mitigation false`, and no later event takes it out
either, because the only path that could is the one that just refused to act.
Bounded by the cap and by a clear, which is exactly what finding 10 judged
insufficient.

`ai/rules/planning.md` symmetry check: the withdraw condition is a two-arm switch
over "does the new config still ask for this rule", and the third leaf that
answers no was not added to it.

#### 19 (NOTE). `show ddos local` reports no mitigation for the width of the handover

`adoptMitigation` (`responder.go:141`) stores the idle snapshot on `prev`
(`prev.published.Store(&mitigationStatus{})`) and `replaceResponder` publishes
`r` afterwards. `handleShowDdosLocal` (`show.go:24`) reads `activeResponder`
lock-free, so between those two instructions it reads `prev` and gets
`active=false` over a rule `r` now owns and the kernel is enforcing. It
self-corrects at the `Store`, and it under-reports rather than over-reports, so
nothing acts on it. Worth knowing when reading the "exactly one owner at every
instant" comment, which finding 14 already asks to be narrowed.

#### 20 (NOTE). `ParseConfig`'s named results are shadowed by an inner `err`

`ParseConfig` (`config.go`) declares `(cfg *Config, found bool, err error)` and
then writes `if err := json.Unmarshal(...)`, so the named `err` is never
assigned and the reader has two `err` in scope. Every return is explicit, so the
names buy nothing the signature's doc comment does not already say. Dropping the
names, or dropping the shadow, removes the question.
```

### Round 3 resolution (2026-09-09)

| # | Severity | Status |
|---|----------|--------|
| 14 | BLOCKER | reproduced by unit test, FIXED. `retired` on the responder, set under `mu` in `replaceResponder` for the responder an apply replaces and in `retireResponder` for the one the engine holds at its stop; `applyMitigation` returns at its first line when it is set. The reviewer's shape was taken with ONE change, argued below. `TestLocalRetiredResponderInstallsNothing` and `TestLocalRetiredResponderDoesNotReclaimACarriedRule` are both red without the guard |
| 15 | BLOCKER | reproduced AT RUNTIME in the QEMU guest, FIXED. `runEngine`'s exit defer calls `retireResponder`, which withdraws whatever rule the responder still owns. `test/plugin/ddos-parent-config-removed.ci` is red with that defer cut, naming `table=ze_ddos-local chain=ingress rules=1` forty seconds after the block was deleted, and green with it. Both pasted below. The guide row is corrected and a new row names the parent gesture |
| 16 | ISSUE | reproduced by reading `characterizeAndEmit`, ADDRESSED as the finding asks: the guide now states what returning to `enforce` does, and `TestLocalReturningToEnforceWaitsForTheNextDetection` pins the round trip. Re-installing from the outgoing responder's target was rejected, because nothing tells the plugin the attack is still running: an `AttackCleared` delivered during the alert window returns on `!active` and leaves that target in place |
| 17 | ISSUE | reproduced by reading `OnConfigRollback`, FIXED. `pendingConfig` now carries the config and the `configured` answer as one value with `stage`, `take` and `clear`, and `rollback` IS the handler `OnConfigRollback` is given, so the test drives the wiring rather than a helper beside it. `TestPendingConfigRollbackUnstagesTheCandidate` is red without the unstaging |
| 18 | ISSUE | reproduced by reading `hookForDirection`, FIXED with one change to the finding: the withdrawal is keyed on the HOOK the live rule sits on, not on the leaf alone, so an INPUT drop for a box-owned victim survives a commit the leaf says nothing about. `TestLocalDisablingForwardMitigationRemovesTheForwardDrop` is red without the arm, and `TestLocalDisablingForwardMitigationLeavesTheIngressDrop` is red if the arm is widened to every live rule |
| 19 | NOTE | reproduced by reading `adoptMitigation` and `handleShowDdosLocal`, NOT fixed; the comment it asks to narrow IS narrowed. Closing the window means publishing the incoming responder before clearing the outgoing one, which splits one critical section into two publishes and makes both responders claim the rule in between. The reading is transient, under-reports, and self-corrects at the `Store`, so the machinery costs more than the defect |
| 20 | NOTE | reproduced, FIXED. `ParseConfig` returns `(*Config, bool, error)` with no names, and its doc comment names the second result |

**Where finding 14's shape was changed, and why.** The reviewer sets `retired`
"where `prev.active = false` is set today", which is inside `adoptMitigation`,
after its two early returns. `adoptMitigation` returns before that line when the
outgoing responder is IDLE, which is the common case, so an idle replaced
responder would stay un-retired and a late `AttackDetected` would install through
it: the same orphan, reached from the responder that had installed nothing. The
retirement is therefore unconditional on a non-nil outgoing responder, in
`replaceResponder`, inside the critical section that already covers the handover.

**Where finding 15's fix sits beside the config boundary, not instead of it.**
The config-boundary withdrawal stays: it is what makes `response-level alert`,
`forward-mitigation false` and the empty-body removal correct, and it runs while
the plugin is still alive. The exit path covers the stop that delivers nothing.
They cannot double-withdraw, because `withdrawMitigation` returns on `!active`
and a config-boundary withdrawal leaves the responder it acted on inactive while
the engine goes on holding the fresh idle one.
`TestLocalEngineStopAfterAWithdrawTouchesTheKernelOnce` asserts one kernel write
across the commit that takes both paths.

**What finding 15's fix changes at DAEMON shutdown, which the finding does not
raise and which is a behavior change under a documented leaf.** The exit path
runs at every stop, so it runs when ze itself stops. At the default `firewall
flush-on-shutdown true` nothing changes: the firewall engine already flushed
every ze-owned table, sequentially and before `CloseBackend`, and its own comment
already names ddos-local among the per-plugin withdraw paths that share its
backend, so there is no race to introduce. At `flush-on-shutdown false`, which
lets ze program rules and exit, the ddos drop is NOW removed where before it
persisted. That is deliberate: the leaf is about rules an operator PROVISIONED,
and this rule is an attack response whose `max-mitigation-duration` worker exits
with the daemon, so leaving it behind is unbounded blackholing rather than
provisioning. `docs/guide/ddos-mitigation.md` states it for the operator, and
`retireResponder`'s comment states it for the reader.
`docs/architecture/firewall/table-ownership-and-shutdown-flush.md` Rule 3 says
"Set `firewall { flush-on-shutdown false; }` to use ze as a one-shot provisioner
that programs rules and exits", which is now imprecise by one exception. That
page was outside this work's file scope, so the sentence is NOT edited and the
main thread is told.

**The commit subject that finding 15 names cannot be corrected in place.** It is
`b11fa5c23`'s subject line, "deleting the config block takes the drop rule with
it", and history is not rewritten. It is true of `delete ddos local` and was
false of `delete ddos` until this work. The guide row it made wrong IS corrected,
and the parent gesture now has a row of its own.

**Journal rows written for this round.**
`plan/journal/late-write-lands-on-the-successor.md` takes the dispatch contract
behind finding 14: `engine_event.go` was read-only for this work, so the
plugin-side guard is what landed, and the durable repair (re-checking
registration inside `dispatch`'s loop) is recorded with the pass over every
subscriber it would need. `plan/journal/component-rebuilt-during-reload.md`'s
2026-09-09 delivery row is amended: ddos local's symptom is fixed, the DELIVERY
defect is not, and every other plugin whose root is a child of a deleted block is
still stopped without being told.

### Round 3 TDD evidence

Every red below was observed by cutting the fix out of the tree, running, and
restoring. No test was written against already-passing code.

**Finding 14, RED with the `retired` check deleted from `applyMitigation`:**

```
2026/09/09 11:40:15 INFO ddos-local: drop rule installed target=10.0.0.1/32 hook=ingress phase=detected
2026/09/09 11:40:15 INFO ddos-local: the ddos local section was removed, removing the drop rule target=10.0.0.1/32
2026/09/09 11:40:15 INFO ddos-local: drop rule removed target=10.0.0.1/32
2026/09/09 11:40:15 INFO ddos-local: drop rule installed target=10.0.0.1/32 hook=ingress phase=characterized
--- FAIL: TestLocalRetiredResponderInstallsNothing (0.00s)
    an event still in flight re-installed the drop rule through the responder the config apply
    retired: the plugin is about to stop, so nothing is left that can remove it and the victim is
    blackholed for the life of the daemon (installs 2)
    the retired responder claims a live mitigation after the section was removed
--- FAIL: TestLocalRetiredResponderDoesNotReclaimACarriedRule (0.00s)
    the replaced responder re-installed over the rule the new one owns, so both now claim it
    (installs 2)
    the replaced responder claims the mitigation the new responder owns
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.414s
```

The install-then-withdraw-then-install-again sequence in the log is the defect
itself: the fourth line is the retired responder putting the rule back.

**Finding 15, RED with the withdrawal deleted from `retireResponder`:**

```
--- FAIL: TestLocalEngineStopRemovesTheDrop (0.00s)
    the plugin stopped with its drop rule still in the kernel: no config is delivered for a
    parent-block removal, so the config boundary never runs and nothing that survives this engine
    can remove the rule (removals 0)
    the responder still reports a live mitigation after the engine stopped
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.506s
```

**Finding 18, RED with the forward-hook arm deleted from `replaceResponder`:**

```
2026/09/09 11:40:45 INFO ddos-local: drop rule installed target=10.0.0.1/32 hook=forward phase=detected
--- FAIL: TestLocalDisablingForwardMitigationRemovesTheForwardDrop (0.00s)
    a commit of forward-mitigation false left the FORWARD drop in the kernel: no later event
    removes it, because the one path that could is the path the leaf now makes return early
    (removals 0)
    the new responder claims a forward mitigation the commit asked to end
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.428s
```

**Finding 17, RED with the unstaging deleted from `pendingConfig.rollback`:**

```
--- FAIL: TestPendingConfigRollbackUnstagesTheCandidate (0.00s)
    a rolled-back transaction left its candidate staged, so the next apply that arrives without a
    verify applies a config the operator abandoned
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.466s
```

**Finding 16's test pins behavior rather than repairing it, so it has no fix to
cut. It was discriminated the other way, by adding a retirement of the INCOMING
responder beside the outgoing one:**

```
2026/09/09 11:43:33 INFO ddos-local: response-level left enforce, removing the drop rule target=10.0.0.1/32
2026/09/09 11:43:33 INFO ddos-local: this responder was retired by a config apply or a plugin stop, not mitigating target=10.0.0.1/32 phase=detected
--- FAIL: TestLocalReturningToEnforceWaitsForTheNextDetection (0.00s)
    a detection after the return to enforce installed nothing, so the box stays unprotected for the
    rest of the attack (installs 1)
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.419s
```

**GREEN, whole package, with every fix in place:**

```
ok  	github.com/ze-software/ze/internal/plugins/ddos/local	0.743s
```

### The parent-removal proof, run in the QEMU guest (2026-09-09)

`test/plugin/ddos-parent-config-removed.ci` was RUN on ze's runtime kernel,
through `./le qemu run kernel tmp/kernel/build/vmlinuz packages "iproute2 libcap"`
with the guest-side `ze-test bgp plugin ddos-parent-config-removed`. The probe
installs a drop under an unbroken flood, commits a config with the WHOLE `ddos`
block deleted, and reads the kernel back. The cap is 3600 and the flood never
stops, so neither the cap nor an `AttackCleared` can account for a removal.

**RED, with the `retireResponder` call cut from `runEngine`'s exit defer, which
is byte-for-byte what the exit path did before this work, and the guest binaries
REBUILT so the cut reached the daemon:**

```
INFO msg="ddos-local: drop rule installed" target=127.0.0.12/32 hook=ingress phase=detected
WARN msg="PARENT-REMOVED-INSTALLED 127.0.0.12 (sent 32000)"
received SIGHUP, reloading config...
INFO msg="ddos-local plugin stopped"
sighup reload complete
ERROR msg="ZE-OBSERVER-FAIL: deleting the parent ddos config block left the drop for 127.0.0.12 in
  the kernel 40s later, under a flood that never stopped and a cap of 3600 seconds: the removal
  delivers no config to ddos-local, so its config-boundary withdrawal never runs, and the plugin is
  stopped anyway -- no responder and no cap worker is left to remove the rule and the box blackholes
  the victim for the life of the daemon (sent 672000 packets):
table=ze_ddos-local chain=ingress rules=1"
QEMU VM: FAIL (exit code 1)
```

Two lines make this red the ORPHAN rather than a reload that never landed.
`ddos-local plugin stopped` says the removal reached the plugin. And there is NO
`ddos-local: reconfigured` line and no withdraw line, which is the finding's own
mechanism observed at runtime: the plugin was stopped without being told
anything.

**GREEN, with the exit-path withdrawal, and the three sibling `.ci` tests re-run
against the same binaries because the retirement is on their path too:**

```
8.8s     3/4  PASS  224  ddos-local-max-duration
3.2s     4/4  PASS  225  ddos-parent-config-removed
3.1s     2/4  PASS  223  ddos-local-config-removed
23.6s    1/4  PASS  222  ddos-local-cap-survives-reload
pass  4/4  100.0%  38.8s
QEMU VM: PASS
```

The cut was reverted, the guest binaries rebuilt, and
`grep -rn MUTATION-APPLIED internal/plugins/ddos internal/test test/` returns
nothing.

### Round 4 (independent, over commit `d7104cb7b`)

Scope: the fixes round 3 made and what they newly touched. 1 BLOCKER, 2 ISSUE,
2 NOTE. Every finding below was read at its producing function.

**Round 3 re-check, at the producers.** Finding 14 is fixed, and the flag cannot
be set on a responder that can still install: `replaceResponder` (register.go)
sets `prev.retired` under `prev.mu`, inside the handover critical section, and
the only readers of `activeResponder` in the window before the publish are
`handleShowDdosLocal` and the cap worker, neither of which installs. The install
paths are `onDetected` and `onCharacterized`, each bound to one responder by
closure, and both reach the kernel only through `applyMitigation`, which now
returns first on `retired`; `registerTables` with a non-nil table has no other
caller in the package. Finding 15's ordering holds: the exit defer is registered
before the cap worker's, so LIFO joins the worker first. The two withdrawals
cannot double-withdraw, because `withdrawMitigation` returns on `!active` and
`adoptMitigation` returns on `!prev.active`, so the engine holds an idle
responder after a config-boundary withdrawal. Finding 18's key is sound in both
directions: `chainHookUnknown` is the zero `ChainHook` (firewall/model.go), so an
unset hook matches neither arm, and `setStatus` and `adoptMitigation` are the
only writers of `hook`. Findings 16, 17, 19 and 20 are resolved as recorded, and
`pending.rollback` is the method value the SDK is handed, so the test drives the
wiring rather than a helper beside it.

#### 21 (BLOCKER). The exit-path withdraw is the copp shape this repository already measured as racy and deleted, and the guide publishes it as unconditional

`retireResponder` runs from a plugin engine goroutine after `p.Run` returns.
`ProcessManager.Stop` (internal/component/plugin/process/manager.go) cancels the
context, calls the non-blocking `Process.Stop` over `pm.processes` in map order,
and only then waits on every engine concurrently, bounded by `pluginStopGrace`,
3 seconds. There is no dependency order, so ddos-local's withdrawal is not
ordered against the firewall engine's own post-Run path, which at
`flush-on-shutdown false` skips `FlushAllTables` and goes straight to
`CloseBackend` (internal/component/firewall/engine.go). `CloseBackend` sets
`activeBackend` to nil (firewall/backend.go), after which `ApplyAll` returns
`errFirewallBackendNotLoaded`, or silently nil when no other owner holds a
table, and writes nothing to the kernel (firewall/registry.go).

`docs/architecture/firewall/table-ownership-and-shutdown-flush.md` Rule 3 states
this defect as already measured, for copp, and names the repair: ProcessManager
.Stop cancels every plugin at once with no dependency order, so copp's own
post-Run withdraw raced the firewall engine's CloseBackend, and copp's own
withdraw was removed. This commit re-introduces that shape for ddos-local.

The rule then survives the daemon. `shouldDeleteTable`
(internal/plugins/firewall/nft/backend_linux.go) deletes a ze_ table only when it
is in the desired set or in this backend instance's `applied` map, so a restarted
ze does not sweep a `ze_ddos-local` table a previous process left behind.

Failure scenario, concrete. `firewall { flush-on-shutdown false; }`, an attack on
a box-owned victim, a drop live on the INPUT hook, then an orderly stop that
carries no signal to the plugin engines, which is the request-shutdown path
(`Server.signalShutdownRequested`, plugin/server/server.go). Both engines are
released by the same `Process.Stop` loop; ddos-local first cancels and joins the
cap worker, while the firewall engine has only `CloseBackend` left to run, and a
ddos-local reconcile that lands after it writes nothing. The daemon exits with
the victim blackholed and nothing in this process or the next removes the table.
The same loss happens under SIGTERM whenever the worker join or the reconcile
costs more than the 3 second grace, which the manager already logs as a plugin
that may have left resources behind.

What makes this a BLOCKER rather than the improvement it also is: the diff
publishes the guarantee. `docs/guide/ddos-mitigation.md` states that an orderly
daemon stop removes the drop too, whatever `firewall flush-on-shutdown` says,
and `retireResponder`'s doc comment states that at `flush-on-shutdown false`
this call still removes the drop. Neither is held by the code, and no test covers
the daemon stop: the new .ci covers the reload gesture, where the daemon and the
backend live on and the path IS correct. A claim wider than the code is what
stops the next reader asking (`ai/rules/evidence.md`).

Two fix shapes, neither picked here. Order the stop by the declared dependency,
so a plugin naming `firewall` stops before it, which is the general repair Rule 3
says does not exist. Or keep teardown with the one ordered actor Rule 3 names:
the firewall engine sweeps an attack-response table on shutdown whatever the leaf
says, because the leaf is about rules an operator provisioned, which is the
argument this commit already makes.

#### 22 (ISSUE). The page that contradicts this change was knowingly left unedited

`docs/architecture/firewall/table-ownership-and-shutdown-flush.md` Rule 3 says
the per-plugin post-Run withdraw was removed and that teardown belongs to the
firewall engine, gated by config. After this commit ddos-local has one again. The
round 3 resolution records the page as outside this work's file scope and tells
the main thread instead. `ai/rules/documentation.md` is always-on and carries no
file-scope exemption: the page edit lands in the same work as the code, and a page
that disagrees with the code is repaired here rather than reported. The sentence
that page owes is the one finding 21 says is not true yet, so the two resolve
together.

#### 23 (ISSUE). A failed exit-path removal is logged as a successful one

`removeMitigation` (responder.go) logs `failed to remove drop rule` on an
`applyAll` error and then logs `drop rule removed` unconditionally on the next
line, before `setStatus(false, ...)`. The line predates this commit; what is new
is that `retireResponder` makes it terminal. Every other caller has a later
reconcile to repair a failure; on the exit path the daemon is gone, so this line
is the operator's only witness and it says the opposite of what happened. The
.ci asserts the withdraw-reason line, not this one, so no test holds it.

#### 24 (NOTE). `retire` and `retireResponder` name two different jobs

`(*responder).retire` sets one flag. `retireResponder` withdraws the rule AND
retires. A reader who greps one finds the other and must read both to learn they
are not the same act (`docs/contributing/ze-go-style.md`, do not overload a name).

#### 25 (NOTE). The once-only test counts registrations, not reconciles

`TestLocalEngineStopAfterAWithdrawTouchesTheKernelOnce` measures `registerTables`
calls through `countingTables`, and its stub `applyAll` counts nothing. Its stated
subject is a second netlink round trip, so a regression that reconciled twice
while registering once would not redden it. Every removal path today pairs the
two, so the proxy is faithful as written.

#### Round 4 disposition

| # | Severity | Status |
|---|----------|--------|
| 21 | BLOCKER | ANSWERED BY THOMAS on 2026-09-09, and his answer is the specification: "We should make no claim on shutdown, the rule should not be saved (as the reboot may be to clear all state) and should be re-detected and instanciated on the next restart." A ddos drop is therefore not persistent state. Three changes follow. The claim is GONE: `stopResponder`'s doc comment and the guide both say the exit withdraw is best-effort at a daemon stop, and name the race the finding measured. The guarantee MOVED to the next process's start: `clearStaleDropRule` sweeps a `ze_ddos-local` table whatever put it there, which is the one place where one actor runs alone and no ordering is needed. And the guide now states plainly that protection after a restart comes from re-detection, with the exposure window the detector's own leaves set. Proved in the guest by `test/plugin/ddos-local-stale-table-swept.ci`, red under the cut sweep and green with it, both pasted below |
| 22 | ISSUE | FIXED. `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` gains Rule 4, which states that an attack-response table is cleared at the next start and why one reconcile cannot do it, and Rule 3 gains the paragraph that ddos-local's per-plugin withdraw is for the stop where the daemon lives on and is best-effort otherwise. Rule 2a's trigger list drops ddos-local, because the sweep always presents a non-empty desired set and so always reaches a backend: the `LegacySweepPending` block in `OnConfigure` is deleted rather than kept beside it (`ai/rules/no-layering.md`) |
| 23 | ISSUE | FIXED. `removeMitigation` computes the reconcile once, clears the status either way, and then writes ONE line: the error names that the rule is still in the kernel, or the info line reports the removal. `TestRemoveMitigationDoesNotReportARemovalTheKernelRefused` is red against the pre-fix shape, pasted below |
| 24 | NOTE | FIXED. `retireResponder` is now `stopResponder`, named for the whole act rather than for the flag it ends with, and its comment says so. `(*responder).retire` keeps its name and its one job |
| 25 | NOTE | FIXED. `countingTables` returns a third counter, the reconciles, and `TestLocalEngineStopAfterAWithdrawTouchesTheKernelOnce` asserts on it. A reconcile added to the exit path that registers nothing new reddens it, pasted below; the register counts alone do not move |

### Round 4 TDD evidence

Every command ran through `./le job run label ddos-local-unit`.

**Finding 21, RED with the claim step cut from `clearStaleDropRule`, which is
what the start path did before this work:**

```
[ddos-local-unit] previous run took 0m7s (7s)
=== RUN   TestStaleDropRuleSweepClaimsTheNameThenWithdrawsIt
    stale_table_test.go:79: the sweep must claim the table name, reconcile, withdraw it and reconcile again; got [{register 0} {reconcile 0}]
--- FAIL: TestStaleDropRuleSweepClaimsTheNameThenWithdrawsIt (0.00s)
=== RUN   TestStaleDropRuleSweepClaimsTheTableUnderTheResponderName
    stale_table_test.go:121: the sweep claimed owner "", want "ze_ddos-local": any other key leaves the responder's own table unclaimed
    stale_table_test.go:124: the sweep must claim exactly one table, got 0
--- FAIL: TestStaleDropRuleSweepClaimsTheTableUnderTheResponderName (0.00s)
=== RUN   TestStaleDropRuleSweepLeavesNoClaimBehindOnAFailedReconcile
    stale_table_test.go:162: the sweep left {reconcile 0} as its last act, want a withdraw carrying no table (whole sequence [{register 0} {reconcile 0}])
--- FAIL: TestStaleDropRuleSweepLeavesNoClaimBehindOnAFailedReconcile (0.00s)
FAIL
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.568s
FAIL
```

**GREEN with the two reconciles:**

```
[ddos-local-unit] previous run took 0m2s (2s)
=== RUN   TestStaleDropRuleSweepClaimsTheNameThenWithdrawsIt
--- PASS: TestStaleDropRuleSweepClaimsTheNameThenWithdrawsIt (0.00s)
=== RUN   TestStaleDropRuleSweepClaimsTheTableUnderTheResponderName
--- PASS: TestStaleDropRuleSweepClaimsTheTableUnderTheResponderName (0.00s)
=== RUN   TestStaleDropRuleSweepLeavesNoClaimBehindOnAFailedReconcile
--- PASS: TestStaleDropRuleSweepLeavesNoClaimBehindOnAFailedReconcile (0.00s)
PASS
ok  	github.com/ze-software/ze/internal/plugins/ddos/local	0.496s
```

**Finding 23, RED against the pre-fix `removeMitigation`, restored byte for
byte:**

```
[ddos-local-unit] previous run took 0m2s (2s)
=== RUN   TestRemoveMitigationDoesNotReportARemovalTheKernelRefused
    stale_table_test.go:213: a refused withdrawal must not also report a removal: the rule is still in the kernel and this line is what an operator acts on. Log was:
        time=2026-09-09T13:47:31.268+01:00 level=ERROR msg="ddos-local: failed to remove drop rule" error="the kernel is wedged"
        time=2026-09-09T13:47:31.269+01:00 level=INFO msg="ddos-local: drop rule removed" target=10.0.0.1/32
--- FAIL: TestRemoveMitigationDoesNotReportARemovalTheKernelRefused (0.00s)
FAIL
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.503s
FAIL
```

**Finding 25, RED with one extra reconcile on the exit path that registers
nothing:**

```
[ddos-local-unit] previous run took 0m1s (1s)
=== RUN   TestLocalEngineStopAfterAWithdrawTouchesTheKernelOnce
2026/09/09 13:47:43 INFO ddos-local: drop rule installed target=10.0.0.1/32 hook=ingress phase=detected
2026/09/09 13:47:43 INFO ddos-local: the ddos local section was removed, removing the drop rule target=10.0.0.1/32
2026/09/09 13:47:43 INFO ddos-local: drop rule removed target=10.0.0.1/32
    max_duration_test.go:730: the exit path reconciled the kernel again after the config boundary had already removed the rule, delaying the stop by a netlink round trip (reconciles 3, was 2)
--- FAIL: TestLocalEngineStopAfterAWithdrawTouchesTheKernelOnce (0.00s)
FAIL
FAIL	github.com/ze-software/ze/internal/plugins/ddos/local	0.457s
FAIL
```

### The stale-table proof, run in the QEMU guest (2026-09-09)

`test/plugin/ddos-local-stale-table-swept.ci` RAN on ze's runtime kernel,
through `./le qemu run kernel tmp/kernel/build/vmlinuz packages "iproute2 libcap"`
with the guest-side `ze-test bgp plugin ddos-local-stale-table-swept`. A fixture
plants a `ze_ddos-local` table with an ingress drop for 127.0.0.13 BEFORE ze
starts, reads it back to prove it reached the kernel, and then the daemon comes
up with `ddos local` configured and a probe that polls for the table's absence.
No flood runs, so no responder installs anything and neither an `AttackCleared`
nor `max-mitigation-duration` can account for a removal.

**RED, with the `clearStaleDropRule` call cut from `runEngine` and the guest
binary REBUILT so the cut reached the daemon:**

```
STALE-TABLE-PLANTED 127.0.0.13
table=ze_ddos-local chain=ingress rules=1
...
INFO msg="ddos-local: configured" subsystem=ddos.local response-level=enforce
ERROR msg="ZE-OBSERVER-FAIL: the ze_ddos-local table a previous process left in the kernel was still
  there 10s after the daemon came up: no responder claims that rule, so no clear and no
  max-mitigation-duration can reach it, and the firewall engine's own flush cannot either -- it
  deletes a ze_ table only when the table is in the desired set or in this backend instance's applied
  map. The victim 127.0.0.13 stays blackholed for the life of the daemon:
table=ze_ddos-local family=2"
QEMU VM: FAIL (exit code 1)
```

The red also settles the finding's own question about the shutdown flush. The
daemon in that run stopped cleanly at the default `flush-on-shutdown` of true,
and the table was still there: `FlushAllTables` reconciles an empty desired set,
and `shouldDeleteTable` will not delete a `ze_` name that is in neither the
desired set nor this backend instance's applied map.

**GREEN with the sweep, and the four sibling ddos-local `.ci` tests re-run
against the same binaries because the rename and the `removeMitigation` change
are on their path:**

```
8.7s     3/5  PASS  224  ddos-local-max-duration
1.3s     4/5  PASS  225  ddos-local-stale-table-swept
3.1s     5/5  PASS  226  ddos-parent-config-removed
25.5s    1/5  PASS  222  ddos-local-cap-survives-reload
3.5s     2/5  PASS  223  ddos-local-config-removed
pass  5/5  100.0%  42.1s
QEMU VM: PASS
```

The mutations were reverted and
`grep -rn MUTATION-APPLIED internal/ test/` returns nothing.

### Round 5 report and implementation (2026-09-10)

Thomas authorized round six on 2026-09-10: one further independent review after
these fixes. The author does not perform that review. No seventh round is
authorized, and the spec remains OPEN until verification and round six finish.
The available independent reviewer model override applies to this continuation.

The inherited report follows unchanged. Finding 26's startup boundary is
confirmed, but its claim that VPP necessarily retains the drop is not established.
`reconcileWithOps` calls `cleanupStartupOrphans`, and the firewall engine's
initial `ApplyAll` already reaches that cleanup. It dumps `ze/`-tagged ACLs,
excludes desired tags, and logs and continues on deletion failures. The repair
below addresses premature reconciliation without changing VPP cleanup.

### Round 5 (independent, over commit `c23134ce1`)

Scope: the fixes round 4 made and what they newly touched, judged against
Thomas's 2026-09-09 ruling (no claim on shutdown, the rule is not saved, the
attack is re-detected on the next start) rather than against the promise that
ruling replaced. 1 BLOCKER, 1 ISSUE, 3 NOTE. Every finding was read at its
producing function.

**Round 4 re-check, at the producers.** The two-reconcile reasoning holds.
`tableNameSet` (`internal/plugins/firewall/nft/backend_linux.go`) keys
`desiredNames` by NAME with no family, and `shouldDeleteTable` returns true for a
`ze_` table in that set or in `b.applied`, so step 1 deletes an ip and an ip6
`ze_ddos-local` together, and `b.applied = desiredNames` at the end of `Apply`
is what makes step 2's withdraw delete the empty table step 1 created. A claim
table with no chains survives `dropTablesMissingAProvidedSet`
(`internal/component/firewall/registry.go`), which examines only `MatchInSet`
terms. No other owner registers that name, and `reconcileMu` serializes the whole
snapshot-plus-apply, so no concurrently starting plugin can interleave a
half-applied set: every hazard I could construct from copp or the flowspec bridge
reconciling between the two steps converges to the same kernel. The failure arm
leaves no claim behind (`registerTables(tableName, nil)` before the return), and
finding 23's fix reads its `r.target` from a `setStatus` call that passes the
field back to itself, so the error line still names the victim. Finding 22's
claim about `LegacySweepPending` is TRUE: the one-time removal in `Apply` walks
`currentTables` independently of the desired set, so any reconcile that reaches a
backend performs it, and step 1 always presents a non-empty set. The guide's
`startup-grace` row and its two escapes match `applyTick` exactly
(`ppsQuiet := maxPps < d.cfg.AbsoluteFloor*5`, and the bps arm gated on
`BpsTriggerEnable && baselineBps.Ready()`), and `confirm-duration` 3 is three
evaluations in `stateMachine.Tick` at the default `check-interval` of 1.

#### 26 (BLOCKER). The sweep runs before the firewall engine owns a backend, so on a `backend vpp` box it clears the wrong dataplane

`clearStaleDropRule` is called from `runEngine` before `sdk.NewWithConn`, which
is earlier than the diff's own comment claims in one respect that matters: it is
before the FIREWALL engine is configured, not only before ddos-local is.

`Manager.spawnProcesses` (`internal/component/plugin/manager/manager.go`) calls
`ProcessManager.StartWithContext`, which starts every plugin's process in one
loop, and `Process.startInternal` (`internal/component/plugin/process/process.go`)
puts each internal engine straight on a goroutine. Only the 5-stage HANDSHAKE is
tier-ordered (`runPluginPhase`, `internal/component/plugin/server/startup.go`:
"All processes are started at once ... the handshake is sequenced by dependency
tiers"). So ddos-local's sweep runs while the firewall engine's goroutine is
still parked in `p.Run` waiting for its own stage-4 config, and the firewall
engine loads the operator's backend only there, in `OnConfigure`
(`internal/component/firewall/engine.go`, `LoadBackend(cfg.Backend)`).

With no backend loaded, `ApplyAll` autoloads `defaultBackendForAutoload`
(`internal/component/firewall/registry.go`), which is `defaultBackendName`,
`"nft"` on Linux (`default_linux.go`). The autoload exists for a box with no
`firewall {}` block. Here it fires on a box that HAS one and names another
backend, because the section has not been read yet.

Failure scenario, concrete. Config: `firewall { backend vpp; flush-on-shutdown
false; }` and `ddos { local { response-level enforce; } }`. An attack on
192.0.2.10 installs the drop through the VPP backend, which is a supported owner
of ddos-local's table (`internal/plugins/firewall/vpp/timeout_linux.go` names
copp, policy-routes and ddos-local by hand). The daemon is restarted. On the next
start ddos-local's sweep autoloads nft, claims `ze_ddos-local`, reconciles, and
deletes and re-creates an NFTABLES table that never held the rule; the VPP
classify state for 192.0.2.10 is untouched. The firewall engine then loads vpp.
192.0.2.10 stays blackholed for the life of the daemon, which is the exact
outcome this commit exists to prevent, and `docs/guide/ddos-mitigation.md` states
the opposite without qualification: "It removes it whatever put it there, and
whatever `flush-on-shutdown` says." A claim wider than the code is what stops the
next reader asking (`ai/rules/evidence.md`), and it is the same shape round 4
raised as finding 21.

The same root cause has a second, smaller consequence on ANY box with a
`firewall {}` section, including the default `backend nft`. `OnConfigure` calls
`LoadBackend` unconditionally, and `loadBackendLocked` replaces `activeBackend`
with a fresh instance whose `applied` map is empty. Landing between the sweep's
two reconciles, that swap makes step 2 find the name in neither the desired set
nor `applied`, so the empty `ze_ddos-local` table step 1 created stays in the
kernel until the next attack registers the name.

Nothing covers this. The unit tests stub `registerTables` and `applyAll`, so no
backend exists in them, and `test/plugin/ddos-local-stale-table-swept.ci`
deliberately carries no `firewall {}` section, which is the one configuration
where the autoload is the correct answer.

Fix shape, not picked here. The sweep needs an actor that owns the configured
backend. ddos-local declares `Dependencies: []string{"firewall"}`, so
`TopologicalTiers` puts it in a later tier and the firewall engine's `OnConfigure`
is complete before ddos-local's begins: moving the call to the first line of
ddos-local's `OnConfigure`, ahead of `replaceResponder` and `subscribe`, keeps
every property the doc comment claims (before the plugin is configured in any
sense that matters, before any event can reach a responder, once per process
because reloads arrive through `OnConfigVerify`/`OnConfigApply`) and adds the one
it lacks. The alternative is the firewall engine sweeping response tables itself,
which is the general repair the journal row already names.

#### 27 (ISSUE). The guide's `baseline-window` row states a cost the producer does not have, and omits the one it does

The row says a box with no saved baseline "re-warms over this window first",
under a column headed "What it costs after a restart". `baseline.Threshold`
(`internal/plugins/ddos/detect/baseline.go`) returns
`max(p99Cache*multiplier, floor)`, and a cold baseline's `p99Cache` is 0, so the
PPS threshold is the absolute floor from the first evaluation. PPS detection is
not delayed by the window at all, and `Ready()` gates nothing on that path.

What a cold baseline does cost is the row above's second escape and the whole
amplification path: `bpsAbove` is computed only when `d.baselineBps.Ready()`
(`applyTick`), which needs `baseline-window` samples. And those samples do not
start accumulating at tick 1. The startup-grace branch returns via `drainPending`
BEFORE `d.baseline.Add`/`d.baselineBps.Add`, so on a quiet box no sample is
admitted for the first 90 ticks. A box that has never run therefore has no armed
BPS trigger for about 390 seconds, not 300, and is blind for that whole period to
the one attack shape the trigger exists to catch, which `applyTick`'s own comment
states: "Amplification is the one attack shape that is low PPS and high
bandwidth, so the packet-rate escape alone is blind to exactly the class the BPS
trigger exists to catch." The table exists to give the operator the exposure
window after a restart, so it understates it for that class and overstates it for
the packet path.

#### 28 (NOTE). The sweep-failure warning names a blackhole that the failing arm may have already removed

`runEngine` logs one line for both arms of `clearStaleDropRule`, with
`"effect", "a victim that process was mitigating stays blackholed until an
operator removes the table"`. True when step 1 fails. False when step 2 fails:
step 1 already deleted whatever was under the name and put an empty table in its
place, so nothing is blackholed and what survives is a chainless table. On a host
with no firewall backend (`defaultBackendName` is `""` off Linux) the sweep now
fails on EVERY ddos-local start and prints that sentence where no table can
exist. Round 4's finding 23 was this class in `removeMitigation`.

#### 29 (NOTE). The `.ci` carries a second firewall owner and does not assert it survives

The copp block's stated reason is sound: `stopOrphanedDependencies` and
`collectOrphanedDependencies` (`internal/component/plugin/server/startup_autoload.go`)
do stop a dependency-only plugin once its last dependent goes, so a second
dependent pins the firewall engine. The block also gives the test a second ze
table, and the assertion the sweep's design most needs is that `ze_copp` is still
in the kernel after the sweep: "no other owner's table is touched" is a claim of
`clearStaleDropRule`'s doc comment that nothing reads back. One more probe over
`fixture06StaleTableFamilies`'s shape would hold it.

Its red is otherwise sound and its discrimination is real: the plant runs as
`seq=1` before the daemon and reads itself back, no flood runs, and the recorded
RED with the sweep cut and the guest binary rebuilt shows the probe seeing the
planted table ten seconds in. A daemon in a different namespace from the planter
would have shown that red as a false green, and it did not.

#### 30 (NOTE). Two records still name `retireResponder`

The rename left `plan/immediate/spec-ddos-timing-leaves-reach-no-worker.md`'s
Data Flow row (the parent-block line) and the 2026-09-09 row in
`plan/journal/component-rebuilt-during-reload.md` naming a symbol that no longer
exists. A finding in the record is not a finding in the product
(`ai/rules/planning.md`), so this re-opens nothing.


#### Round 5 diagnosis and disposition

The symptom is cleanup before firewall configuration. `runEngine` called the
sweep before `sdk.NewWithConn`, while `Server.runPluginPhase` starts every engine
before tier-ordered handshakes. The owning layer is ddos-local's initial callback.
[source] Run the sweep at initial `OnConfigure`, before responder creation.
[workaround] Force a backend or repeat cleanup during reload. That would ignore
the configured dependency or erase this engine's live response on an ordinary commit.

| Finding | Implementation | Observed proof |
|---------|----------------|----------------|
| 26 | `local/register.go` `runEngine` sweeps in initial `OnConfigure` after firewall configuration and before subscriptions; reload has no sweep | `job-ddos-configured-backend-red-4085114d.log`: pre-change register.go overlay leaves the configured backend's stale table and fails. `job-ospf-ddos-fixed-race-d0b752c2.log`: fixed startup/reload test PASS and all local tests PASS with race |
| 27 | Guide distinguishes PPS floor from BPS readiness and accounts for quiet grace, partial restore and exact-5x escape | `job-ddos-timing-smoke-9ec15378.log`: cold BPS 391/393; full restore 1/3; partial 50 samples 341/343; cold PPS 2x floor detects 93 and exact 5x detects 3 |
| 28 | Startup warning reports the failed stage and requests backend inspection | Current race log: `TestLocalStartupCleanupFailureStillHandlesDetection` PASS for stages 1 and 2, with each stage-specific warning followed by a fresh installed drop |
| 29 | Existing no-flood scenario selects nft explicitly with flush-on-shutdown false and reads the copp limiter | `job-ddos-stale-runtime-linux-9c564b10.log`: Linux 7.2, rebuilt private binaries, PASS 1/1. The current .ci requires both STALE-TABLE-SWEPT and COPP-RULE-PRESERVED. The inherited no-sweep RED remains under “The stale-table proof” |
| 30 | Live wiring and the reload journal name `stopResponder` | Current source and record reads; historical round reports retain the symbols they reviewed |

The three inherited sweep tests now use the real firewall registry and a stateful
backend, rather than recording helper order or field copies. They retain the
claim/withdraw, responder-name and failed-reconcile cleanup contracts. The
configured-backend startup test adds the missing lifecycle boundary. The
functional test still plants and reads a drop before daemon startup and sends no flood.

Pre-change files are preserved under
`tmp/session/2026-09-09-5bc855cc-5761-4012-8d90-7a9823029ab3/scratch/ddos-round5-baseline/`.
`startup-overlay.json` substitutes only pre-change `register.go`, so new tests
remain present for the RED run. `overlay.json` preserves the wider source fixture.
The author phase ran no validation. The parent subsequently supplied the current RED,
race, Linux and timing proofs listed above. Round six read those logs without rerunning them.
The mixed race command remains RED overall because of the separate OSPF assertion;
its DDoS local package is PASS. Earlier rounds describe their recorded source versions.

### Round 6 final independent review (2026-09-10)

CLEAN: 0 BLOCKER, 0 ISSUE. The independent reviewer was `DdosClosureSix`, using
Thomas's available-model override for unavailable Opus 5. Thomas explicitly chose
round six; no seventh round is authorized. The product reason for the extra round
was finding 26: `runEngine` reconciled before the configured firewall backend was selected.

The review read the current patch and the implementation paths for AC-1 through AC-10.
Logic and lifecycle, security and bounds, and test discrimination and documentation were reviewed inline.
`Server.runPluginPhase` completes each dependency tier before the next.
Firewall `runEngine` loads and applies its backend in `OnConfigure`; local `runEngine` then sweeps before subscribing.
`RegisterTables` and `ApplyAll` retain other owners' desired state.
Nft `shouldDeleteTable` recognizes the claimed name across address families.
Reload keeps its verify/apply path and carries the live rule through `replaceResponder`.

The current startup regression uses both production engines and the real registry with a stateful test backend.
It proves backend selection and reload preservation, rather than VPP dataplane behavior.
VPP `reconcileWithOps` already calls `cleanupStartupOrphans`.
The inherited assertion that a VPP drop necessarily survives the old ordering is unproven.
The Linux proof reads nft kernel state; the timing proof drives detector logic through a throwaway overlay.
AC-7 and AC-8 retain their inherited unit discrimination proofs.

Record-only NOTEs were corrected together: stale implementation summaries, outstanding-proof cells,
historical citers and absent closure audit sections. These corrections do not reopen the product review.

All current proof logs are under
`tmp/session/2026-09-09-5bc855cc-5761-4012-8d90-7a9823029ab3/scratch/`.
No formatter, lint, build, test suite or repository-wide gate ran in this review.
Settled RED gates remain RED: prior full lint 93 host/104 Linux/2 capability/1 coverage;
current scoped lint 22 host/22 Linux, with no DDoS source diagnostic;
repository check 24 BGP findings; commit audit 16 concurrent findings; doc verify 3759 drift/8 summary.
The pre-release owner rule permits closure with this attribution and forbids repeating unchanged gates.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ddos-timing-leaves-reach-no-worker-5bc855cc-5761-4012-8d90-7a9823029ab3.md` |
| Native review check | Recorded and checked by the closure owner after this metadata edit, before scoped commit execution |
| Rounds | 6; Thomas authorized the final pass over the pre-configure wrong-backend sweep fix |
| Reviewer lenses used | Logic and lifecycle; security and bounds; reachability, regression discrimination and documentation |
| Final verdict | CLEAN, 0 BLOCKER, 0 ISSUE |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Startup sweep ran before the SDK handshake | Engines start together; configuration is dependency-tier ordered | Round-five source trace and current configured-backend RED | Sweep in initial `OnConfigure` |
| approach | Restart exposure prose treated PPS and BPS warm-up alike | PPS has a cold floor; BPS needs a full admitted window after quiet grace | `applyTick`, `Ready`, and current timing smoke | Corrected the guide and proof record |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Four timing leaves reach running consumers | Done | `detect/detector.go` `tick`; `flowspec/responder.go` `announce`; `observe/register.go` `startStaleSweep`; `local/register.go` `startMaxDurationWorker` | Existing schema and bounds retained |
| Local mitigation retains its age through reload and leaves on operator withdrawal | Done | `local/register.go` `replaceResponder`, `stopResponder`; `local/responder.go` `adoptMitigation`, `enforceMaxDuration` | Inherited unit and QEMU RED/GREEN |
| Previous-process response table is swept at configured startup | Done | `local/register.go` `runEngine`, `clearStaleDropRule` | Best-effort stage errors; no shutdown guarantee |
| Operator restart timing matches detection | Done | `docs/guide/ddos-mitigation.md`; `detect/detector.go` `applyTick` | Current cold/full/partial/PPS smoke proof |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `tick`, `intervalPeak`; `interval_test.go`; `ddos-timing-leaves.ci` | Peak and interface survive the fold |
| AC-2 | Done | `announce`, `announceLimiter.allow`; `announce_limit_test.go`; `ddos-announce-rate-limit.ci` | Refusal stays idle; window expires at 60 seconds |
| AC-3 | Done | `startStaleSweep`, `sweepStale`; `sweep_test.go`; `ddos-timing-leaves.ci` | Flood continues while incident finalizes |
| AC-4 | Done | `tick`; `TestBaselineSaveFollowsTheFeedTickNotTheEvaluation` | Save remains feed-driven |
| AC-5 | Done | `announce`; `TestAnnounceRefusesAnUnresolvedVictim` | Prefix guard precedes budget consumption |
| AC-6 | Done | `startMaxDurationWorker`, `enforceMaxDuration`; `ddos-local-max-duration.ci` | Wall-clock removal and idle publication |
| AC-7 | Done | `enforceMaxDuration`; `TestLocalMaxDurationZeroMeansNoCap` | Explicit zero-cap guard; unit discrimination |
| AC-8 | Done | `setStatus`, `adoptMitigation`; `TestLocalMaxDurationClockStartsOnTheFirstInstall` | First-install clock retained |
| AC-9 | Done | `enforceMaxDuration`; `TestLocalMaxDurationIdleWorkerRemovesNothing` | Inactive early return |
| AC-10 | Done | `runEngine`, `clearStaleDropRule`; current startup and Linux proofs | Backend selected before sweep; fresh events survive both failure stages |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Detector interval and persistence tests | Done | `internal/plugins/ddos/detect/interval_test.go` | Inherited RED/GREEN |
| FlowSpec announcement limit tests | Done | `internal/plugins/ddos/flowspec/announce_limit_test.go` | Inherited RED/GREEN and functional limiter cut |
| Observation worker tests | Done | `internal/plugins/ddos/observe/sweep_test.go` | Inherited worker-cut RED/GREEN |
| Local cap and reload tests | Done | `internal/plugins/ddos/local/max_duration_test.go` | Current package race PASS; inherited discrimination cuts |
| Startup ownership tests | Done | `internal/plugins/ddos/local/stale_table_test.go` | Current configured-backend RED, fixed race PASS |
| Daemon scenarios | Done | `test/plugin/ddos-timing-leaves.ci`, `ddos-announce-rate-limit.ci`, `ddos-local-max-duration.ci`, `ddos-local-cap-survives-reload.ci`, `ddos-local-config-removed.ci`, `ddos-parent-config-removed.ci`, `ddos-local-stale-table-swept.ci` | Dated QEMU proofs; current Linux rerun is stale-table only |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/ddos/detect/`, `observe/`, `flowspec/`, `local/` planned producers and tests | Done | Source consumers and caller paths read |
| `internal/test/fixture/plugin_fixture_05_ddos.go`, `plugin_fixture_06_ddos_linux.go` | Done | Registered functional drivers; Linux-specific split recorded |
| `test/plugin/ddos-*.ci` scenarios named in the audit | Done | Existing functional evidence retained |
| DDoS guide, firewall ownership architecture and reload journal | Done | Current ordering, timing and lifecycle records |

### Audit Summary
Four task requirements and ten acceptance criteria are Done.
Six test groups and four file groups are accounted for.
No in-scope item is Partial or Skipped; the Linux fixture split is the recorded plan change.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| Planned DDoS producer/test files and named .ci scenarios | Yes | Source reads and inherited run output identify the files; current Linux log runs `ddos-local-stale-table-swept.ci` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-4 | Interval peak evaluation and feed-clock save | Current `detect/detector.go` `tick` read; inherited interval discrimination tests |
| AC-2, AC-5 | Bounded announces and unresolved-victim refusal | Current `flowspec/responder.go` `announce`/`allow` read; inherited limiter QEMU RED/GREEN |
| AC-3 | Running stale sweep | Current `observe/register.go` worker and `store.go` sweep read; inherited live-flood proof |
| AC-6, AC-7, AC-8, AC-9 | Local cap, zero, first-install age and idle behavior | Current local package race PASS and producer reads; inherited worker/clock cuts |
| AC-10 | Configured-backend startup sweep | Current configured-backend RED/GREEN, both cleanup failure stages PASS, Linux 7.2 PASS 1/1 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Detector interval and observe timeout | `test/plugin/ddos-timing-leaves.ci` | Config, live flood and status assertions read |
| FlowSpec announcement limit | `test/plugin/ddos-announce-rate-limit.ci` | Attached update sender, two generations and refusal assertion read |
| Local cap and reload/removal | Local max-duration, cap-survives-reload, config-removed and parent-config-removed scenarios | Inherited discriminating QEMU proofs and current lifecycle producers |
| Process startup with stale table | `test/plugin/ddos-local-stale-table-swept.ci` | Pre-daemon planter, no flood, explicit backend and both marker assertions read; current Linux PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `tick` folds evaluation peaks as the existing baseline comment specifies |
| A-2 | confirmed | Feed subscriptions and interval producer; timing proof assumes uninterrupted one-second samples |
| A-3 | confirmed | `announce` refuses and returns before publication; inherited functional refusal proof |
| A-4 | confirmed | One observation worker calls bounded `sweepStale` and joins at teardown |
| A-5 | confirmed | Local cap remains a supported leaf and reaches `enforceMaxDuration` |
| A-6 | confirmed | One-second local worker and current package race PASS |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Startup ordering and cleanup failures | `local/register.go` `runEngine`/`clearStaleDropRule`; firewall `runEngine`; `Server.runPluginPhase` | Guide and firewall ownership page match |
| PPS/BPS restart exposure and persistence | `applyTick`, `Ready`, `restore`, `saveBaselines`, `loadBaselines` | Current timing smoke matches corrected guide |
| Local mitigation lifecycle | `replaceResponder`, `stopResponder`, `adoptMitigation`, `enforceMaxDuration` | Current guide names producing paths and limits shutdown claims |
| Config syntax, CLI/API, plugin inventory, RFC and doctor surfaces | Existing leaf ranges, registrations and dispatch types remain unchanged by round-six repairs; existing source anchors were located | No new syntax, command, wire format or runtime dependency |
| Repository-wide documentation gate | Parent's settled doc verify reports 3759 drift/8 summary | RED retained; no rerun or green claim |

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
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
