# Spec: ddos-timing-leaves-reach-no-worker

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 8/8 |
| Handoff | - |
| Updated | 2026-09-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

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

## Files to Create
- `internal/plugins/ddos/detect/interval_test.go`
- `internal/plugins/ddos/flowspec/announce_limit_test.go`
- `internal/plugins/ddos/observe/sweep_test.go`
- `test/plugin/ddos-timing-leaves.ci`
- `internal/plugins/ddos/local/max_duration_test.go`
- `test/plugin/ddos-announce-rate-limit.ci`
- `test/plugin/ddos-local-max-duration.ci`
- `test/plugin/ddos-local-cap-survives-reload.ci`

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
| 12 | Internal architecture changed? | No | `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` describes the detector's stages, not its cadence |
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
| Feature completeness | AC-2 has no `.ci`; that is the one open item and it is named in Known Limitations rather than closed over |
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
- `test/plugin/ddos-local-max-duration.ci`: green in the guest, red under the
  worker cut. `test/plugin/ddos-announce-rate-limit.ci`: written, not yet green.
  See Work Not Done.

### Bugs Found/Fixed
- None beyond the spec's own defect. No walked-into defect was met in this phase.

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
- The spec's Files to Modify names `internal/test/fixture/plugin_fixture_05_ddos.go`
  and `plan/journal/unwired-feature.md`. Both were out of this agent's write
  scope, so both are in Work Not Done rather than done.
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
| The cap survives an operator's unrelated commit | functional `.ci` | `test/plugin/ddos-local-cap-survives-reload.ci`, RUN in the guest on 2026-09-09: PASS in 24.5s, and FAIL naming the orphaned rule under the rebuilt cut that deletes `adoptMitigation`. Both pasted in the Review Gate |
| An operator reaches the announce limit end to end | functional `.ci` | `test/plugin/ddos-announce-rate-limit.ci`, RUN in the guest on 2026-09-09: PASS in 55.5s, and FAIL with a second announcement 18s into the window under the rebuilt limiter cut |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| (closed 2026-09-09) The AC-2 run of `test/plugin/ddos-announce-rate-limit.ci` | The blocker was another session's in-flight refactor of `internal/component/config/transaction` and `internal/component/iface` breaking the tree-wide build, which `ze-test` links. It is gone: `GOOS=linux GOARCH=amd64 go vet ./internal/test/fixture/ ./internal/plugins/ddos/...` exits 0, and the guest binaries cross-compile against a `go -overlay` that presents HEAD for other sessions' files. The test RAN in the guest, green, and red under the cut that stops `announce` consulting the limiter. Both are pasted in the Review Gate | nothing. AC-2 has its `.ci` |
| The ddos FLOWSPEC half of the config-apply orphan | `p.OnConfigApply` in `internal/plugins/ddos/flowspec/register.go` has the identical shape and orphans a live FlowSpec announcement the same way. It is a different spec's leaf and this spec's goal does not depend on it, so it took one row in `plan/journal/component-rebuilt-during-reload.md` (2026-09-08) and nothing else | `plan/immediate/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md` owns the flowspec cap; the journal row is what earns the fix |

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
