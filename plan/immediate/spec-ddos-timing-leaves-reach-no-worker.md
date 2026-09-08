# Spec: ddos-timing-leaves-reach-no-worker

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

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

`max-mitigation-duration` is not in this spec.
`plan/immediate/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md`
already carries it.

## Progress (2026-09-06, resumed and implemented)

The paused state this section used to describe was checked and was already past
what it said: `go vet ./internal/plugins/ddos/...` was clean, and the observe
half (`startStaleSweep`, `subscribeStore`, `sweep_test.go`) plus a written
`detect/interval_test.go` were in the tree. One unit test was red,
`TestCheckIntervalDecimatesEvaluations`, which is the TDD red the detect half
needed.

All three leaves are now implemented, and each carries a recorded red. What this
work did NOT reach is the `.ci` for `announce-rate-limit`: see Known Limitations.

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

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Peak-holding the interval raises the baseline as well as the threshold, so a spiky link drifts its own threshold up | The baseline p99 climbing on a link with no attack | Both sides read the same statistic: `applyTick` feeds the peak to `baseline.Add` and compares against a threshold built from peaks. Stated in the `intervalPeak` doc comment |
| R-2 | An operator raising `check-interval` silently multiplies `confirm-duration` and `clear-consecutive-checks` in wall-clock terms | An attack confirmed far later than the operator expected | Said in the YANG `ze:help` of all three leaves and in `docs/guide/ddos-mitigation.md` |
| R-3 | The functional test's timing floor could pass against broken code on a loaded machine | A CHECK-INTERVAL-HELD value close to the floor | The margin was measured and widened: 10 to 20 seconds wired against 3 seconds with the leaf ignored, floor 8 seconds |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A detector that evaluates at the wrong cadence detects late or not at all; a limiter that refuses too much leaves an attack unmitigated upstream. Nothing reaches the wire differently at the default config, where `check-interval` is 1 and `announce-rate-limit` is 10 |
| How is it reverted? | A single commit revert. No config migration: every leaf already existed, already had its range, and already had its default |
| Who else touches this path? | `plan/immediate/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md` owns `max-mitigation-duration` in the same responder |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ddos detect check-interval 10` in the daemon config | -> | `(*detector).tick` folding ten feed samples into one `applyTick` | `test/plugin/ddos-timing-leaves.ci` (CHECK-INTERVAL-HELD) |
| `ddos observe stale-incident-timeout 1` in the daemon config | -> | `startStaleSweep` -> `(*store).sweepStale` | `test/plugin/ddos-timing-leaves.ci` (STALE-SWEEP-FINALIZED) |
| A rate feed sample | -> | `(*detector).tick` | `TestCheckIntervalDecimatesEvaluations`, `TestCheckIntervalEvaluatesTheIntervalPeak` |
| `ddosevent.Detected` on the bus | -> | `subscribeStore` -> `(*store).open` | `TestDdosObserveUnsubscribeDetachesStore` |
| `ddosevent.Characterized` on the bus | -> | `(*responder).announce` -> `announceLimiter.allow` | `TestAnnounceRateLimitBoundsTheWindow` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `check-interval N` with an unbroken above-threshold feed | The detector evaluates once every N feed samples, over the interval's peak rate on the interface that carried it, so `confirm-duration` evaluations take at least `(confirm-duration - 1) * N` seconds |
| AC-2 | More announcements asked of the flowspec responder in one 60-second window than `announce-rate-limit` allows | Every announcement over the limit is refused and logged, the responder stays idle, and the budget returns 60 seconds after the announcement that held it |
| AC-3 | An incident open longer than `stale-incident-timeout` with no `AttackCleared` | A running worker finalizes it with an end time, whatever the attack is doing |
| AC-4 | The periodic baseline save at a `check-interval` that does not divide 300 | The save still runs on its ~300-second cadence |
| AC-5 | A critical `AttackDetected` whose victim was never resolved, with `blackhole-fallback` on | The responder announces nothing and consumes no announce budget (walked-into defect, `plan/journal/zero-value-as-valid-answer.md`) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | sets `check-interval 10` and floods the box | config -> `newDetector` -> trafficstat feed -> `tick` fold -> `applyTick` -> `AttackDetected` -> `show ddos incidents` | `test/plugin/ddos-timing-leaves.ci` |
| 2 | sets `stale-incident-timeout 1` and reads `show ddos incidents` during an attack that never clears | config -> `runEngine` apply -> `startStaleSweep` -> `sweepStale` -> the incident carries an end time | `test/plugin/ddos-timing-leaves.ci` |
| 3 | sets `announce-rate-limit 1` and meets two attack generations in a minute | config -> `newResponder` -> `announce` -> `announceLimiter.allow` refuses the second | `TestAnnounceRateLimitBoundsTheWindow` (unit only; see Known Limitations) |

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

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `check-interval` | 1-3600 | 3600 | 0 | 3601 |
| `announce-rate-limit` | 1-600 | 600 | 0 | 601 |
| `stale-incident-timeout` | 1-86400 | 86400 | 0 | 86401 |

Each range is already enforced and tested by the owning `Config.Validate`
(`detect/config_test.go`, `flowspec/config_test.go`, `observe/config_test.go`);
this spec changed no range and added none.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-timing-leaves` | `test/plugin/ddos-timing-leaves.ci` | An operator sets `check-interval 10` and `stale-incident-timeout 1`, floods the box, and reads `show ddos incidents` | green; red under each of the two cuts |
| announce-rate-limit `.ci` | not written | An operator sets `announce-rate-limit 1` and meets two attack generations in a minute | NOT DONE - see Known Limitations |

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

## Files to Create
- `internal/plugins/ddos/detect/interval_test.go`
- `internal/plugins/ddos/flowspec/announce_limit_test.go`
- `internal/plugins/ddos/observe/sweep_test.go`
- `test/plugin/ddos-timing-leaves.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Every leaf already existed; only `ze:help` prose changed |
| YANG validation constraints | No | Ranges unchanged |
| YANG custom validators | No | Native `range` is sufficient |
| CLI commands/flags | No | No command added |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/ddos-timing-leaves.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No `environment/` leaf |
| Doctor check for runtime dependencies | No | No new file, socket, port or module; the sweep worker is in-process |
| Prometheus counters/metrics | No | The refusal is logged at WARN. A counter would be a new metric name, which is its own decision |
| BGP family surface | N-A | No SAFI, capability or attribute touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Three declared leaves start working; `docs/features.md` describes neither leaf |
| 2 | Config syntax changed? | No | Syntax unchanged |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | No registration, transport or dependency change |
| 6 | Has a user guide page? | Yes | `docs/guide/ddos-mitigation.md` - updated |
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

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The detector evaluates on the interval | `go test -run TestCheckInterval ./internal/plugins/ddos/detect/` |
| The responder bounds its announcements | `go test -run TestAnnounceRate ./internal/plugins/ddos/flowspec/` |
| The sweep worker runs in the plugin | `grep -n startStaleSweep internal/plugins/ddos/observe/register.go` shows the call inside `apply` |
| The operator reaches both | `ze-test bgp plugin 214` (ddos-timing-leaves) |
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

## Known Limitations
- **`announce-rate-limit` has no `.ci`.** The unit tests drive the responder through its
  real event handlers and are discriminated, but no functional test proves an operator
  reaches the limit. Reaching the announce path in a daemon needs a victim that resolves
  to a REMOTE prefix: `flowspec` defers to on-host mitigation for a local victim, and an
  unresolved victim is now refused outright (AC-5). A remote victim needs the veth
  transit topology and the eBPF traffic-usage source that
  `test/plugin/ddos-transit-forward-drop.ci` builds, which runs as root under QEMU only.
  The test shape is known and was drafted: two attack generations inside one minute, with
  `max-mitigation-duration 1` withdrawing the first announce, the flood stopped to clear
  the attack and restarted for the second generation, and the second announce asserted
  refused only AFTER a second incident proves the responder was asked. It is not written
  here because it could not be run red-then-green in this session
  (`ai/rules/interop-and-goal-validation.md` forbids claiming a test that never went red).
- The refusal is logged and not counted. A Prometheus counter for refused announcements
  is a new metric name, which is its own decision and its own spec.
- The detector still emits a critical `AttackDetected` with no victim when no traffic
  source can name one. AC-5 stops the responder acting on it; whether the DETECTOR should
  emit it at all is the open question the journal row records.

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
