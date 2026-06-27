# Spec: DDoS Detector — Two-Stage (Rate Trigger + On-Attack Pattern Analysis)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-cp-survival-5-detect-0-umbrella, spec-cp-survival-5-detect-1a-vpp-iface-rate |
| Phase | 1/5 |
| Updated | 2026-06-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-cp-survival-5-detect-0-umbrella.md` (parent — shared research, modes, risks)
3. `.claude/rules/planning.md` - workflow rules
4. `internal/component/iface/rate.go` (rate signal), `pkg/ze/eventbus.go` + `internal/core/events/typed.go` (event emission), `pkg/plugin/rpc/bridge.go` + `plan/learned/294-inprocess-direct-transport.md` (DirectBridge for on-attack analysis)

## Task

Build the **detector**: the one genuinely new capability in the auto-mitigation initiative. It performs
**no mitigation** — responders (children 2-4) subscribe to its events and act.

It copies the Flowtriq `ftagent` **two-stage** approach:

1. **Stage 1 — cheap always-on trigger.** Watch only the `iface` aggregate/per-interface rate
   (RxPps/RxBps), learn a rolling baseline, and trigger when the rate crosses the dynamic threshold.
   This stage adds zero per-IP tracking and no cross-plugin traffic in steady state — the analogue of
   `ftagent`'s `PPSMonitor` reading `/proc/net/dev`.
2. **Stage 2 — on-attack traffic-pattern analysis.** ONLY once Stage 1 fires, run a one-shot
   characterization that pulls per-(dst-IP,port) and flow data to determine the **target**, **vector
   tuple** (proto / dst-port / src-port / tcp-flags), **source dispersion**, and **attack family** —
   the analogue of `ftagent`'s `TrafficAnalyser` invoked inside `_begin_attack`. The result both
   (a) populates the `AttackDetected` event so responders can build a surgical match, and (b) acts as a
   **false-positive filter**: a rate spike with a benign pattern (few sources, established flows, no
   flood signature) is suppressed and no event is emitted.

Steady-state detection is therefore self-contained (iface only); the expensive per-IP/flow reads happen
only during the rare incident, via DirectBridge calls to `trafficusage` / `flowexport`.

Ownership boundary (from the umbrella): the detector owns onset detection, characterization, and clear
emission **for signals it can still observe** (local-mode rate), plus the two timers governing its own
evaluation (`check-interval`, `clear-consecutive-checks`). The mitigation-lifecycle timers (`hold-down`,
`probe-window`) live in child 3, because clear-detection under upstream FlowSpec drop is a responder
concern (the detector goes blind — umbrella A-2/R-2).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-design.md` (EventBus Typed Payloads + DirectBridge) - event + analysis comms
  → Decision: emit `AttackDetected`/`AttackOngoing`/`AttackCleared` via `events.Register[T]` (broadcast); pull on-attack per-IP/flow data via DirectBridge request/response (`pkg/plugin/rpc/bridge.go`, in-process per `plan/learned/294`).
  → Constraint: Stage 1 uses neither mechanism; only Stage 2 (on-trigger) issues DirectBridge calls, so steady state has no cross-plugin chatter.
- [ ] `ai/patterns/plugin.md` + `ai/patterns/registration.md` - plugin shape
  → Constraint: `internal/plugins/ddosdetect/` registers via `init()` in `register.go`; blank-imported through the generated `internal/component/plugin/all/all.go` (`make generate`).
- [ ] `ai/rules/goroutine-lifecycle.md` - the tick loop
  → Constraint: detection goroutine started in `OnAllPluginsReady`, stopped on shutdown via stop channel; no leak; no work before iface rates are warm (startup grace).
- [ ] `ai/rules/config-surface.md` + `ai/rules/config-naming.md` - detector config
  → Decision: thresholds + the two detector timers are operational policy → YANG leaves (kebab-case); durations use `ze-types.yang` duration type with `range`.
- [ ] `ai/rules/module-tiers.md` - package placement
  → Constraint: detector is config-driven → `internal/plugins/`; the shared event-payload contract is a core leaf → `internal/core/ddosevent/` (no plugin imports), must pass `make ze-tier-check`.

### RFC Summaries
- N/A — the detector touches no wire protocol. Interop/RFC obligations belong to child 3 (origination).

**Key insights:**
- Steady-state cost is the whole point of the two-stage split: Stage 1 is a single rate comparison per
  interface per tick; Stage 2's expensive per-IP/flow analysis runs only on attack onset (rare).
- Stage 2 doubles as the false-positive filter: a rate spike with a benign pattern is suppressed, so
  flash crowds do not produce `AttackDetected`.
- The detector emits `AttackCleared` only from the rate signal it can still see; under upstream FlowSpec
  drop that goes blind, so child 3 ignores clear while mitigating and runs its own leak-probe. Every
  event carries the current rate + an "observable" flag so a blinded responder can reason about staleness.
- Baseline poisoning guard (ported `ftagent`): never feed attack-window or above-floor samples into the
  baseline; apply an absolute floor and a startup grace so a node attacked at boot is still caught.

## Current Behavior (MANDATORY)

**Source files read:** (the signals the detector consumes; all exist today)
- [ ] `internal/component/iface/rate.go` - `RegisterCollectNotify(fn CollectNotifyFunc)` (rate.go:66); `CollectNotifyFunc func([]InterfaceInfo)` (rate.go:59); rate tracker computes RxBps/TxBps/RxPps/TxPps at ~1 Hz (`rateDelta` rate.go:181). LSP-confirmed.
  → Constraint: Stage 1 subscribes to this callback only; do NOT add a second `/proc/net/dev` reader, and do NOT read per-IP data in steady state.
- [ ] `internal/plugins/trafficusage/monitor.go` - `Monitor.Snapshot() []counts` (monitor.go:191) holds per-src-IP / per-(dst-port) byte counts, but `Monitor` is plugin-private and `counts` is unexported.
  → Constraint: Stage 2 reads this via a NEW DirectBridge handler that trafficusage must expose; the detector does not import the Monitor. On-trigger only.
- [ ] `internal/plugins/flowexport/conntrack/delta.go` - `DeltaTracker` (delta.go:77) with `ComputeDelta`/`Len`; owned by flowexport's live conntrack dump.
  → Constraint: Stage 2 obtains the 5-tuple flow snapshot / new-flow count via a NEW DirectBridge handler on flowexport; do not start a second conntrack reader. On-trigger only.
- [ ] `pkg/ze/eventbus.go` + `internal/core/events/typed.go` - `events.Register[T]` typed publish/subscribe.
  → Constraint: this is the only detector→responder channel; register the three event types here.
- [ ] `pkg/plugin/rpc/bridge.go` - `DirectBridge` typed request/response (`DispatchRPC`/`SendCallback`).
  → Constraint: Stage-2 analysis calls are DirectBridge requests to trafficusage/flowexport, not EventBus; exact in-process transport per `plan/learned/294`.

**Behavior to preserve:**
- `iface` rate collection, `trafficusage`, and `flowexport` keep working unchanged; the detector is a
  read-only consumer and adds no load to their hot paths (Stage-2 reads are on-trigger, rare).
- Existing EventBus consumers are unaffected; the three new event types are additive.

**Behavior to change:** None in existing code, except trafficusage and flowexport each GAIN one new
read-only DirectBridge handler (additive, used only on attack onset).

## Data Flow (MANDATORY)

### Entry Point
- Stage 1: `iface.RegisterCollectNotify` callback delivers `[]InterfaceInfo` every ~1 Hz (rates).
- Stage 2 (on trigger only): DirectBridge requests to `trafficusage` (per-IP/port counts) and
  `flowexport` (conntrack flow snapshot).

### Transformation Path
1. Rate callback updates current per-interface and aggregate RxPps/RxBps.
2. Each `check-interval`, feed non-attack, sub-floor samples into the rolling baseline (poisoning guard).
3. Evaluate the Stage-1 trigger: rate vs `max(baseline p99 × multiplier, absolute-floor)`, must persist for `confirm-duration` (with startup grace).
4. On a Stage-1 candidate, run Stage-2 analysis: DirectBridge to trafficusage + flowexport → compute target (top dst-IP/port), vector tuple, source dispersion/entropy, new-flow rate, family.
5. If the pattern is attack-like, emit one `AttackDetected` (target, family, vector, top sources, peak rate, observable=true). If benign, suppress (false-positive filter) and resume Stage 1.
6. While active, emit `AttackOngoing` each `check-interval` with the current rate level.
7. When the observable rate stays below threshold for `clear-consecutive-checks` ticks, emit `AttackCleared`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface ↔ detector (Stage 1) | `RegisterCollectNotify` callback (1 Hz) | [ ] |
| trafficusage/flowexport ↔ detector (Stage 2) | NEW DirectBridge handlers, on-trigger | [ ] |
| detector ↔ responders | typed EventBus events (`events.Register[T]`) | [ ] |
| config tree ↔ detector | YANG `ddos-detect` container → plugin settings | [ ] |

### Integration Points
- `internal/component/iface/rate.go` `RegisterCollectNotify` - Stage-1 rate source
- `internal/plugins/trafficusage` (new DirectBridge handler) / `internal/plugins/flowexport` (new DirectBridge handler) - Stage-2 analysis
- `internal/core/ddosevent` (new leaf) - shared event-payload contract imported by responders
- `pkg/ze/eventbus.go` - publish channel; `pkg/plugin/rpc/bridge.go` - Stage-2 transport

### Architectural Verification
- [ ] No bypassed layers (Stage 2 via DirectBridge, not private imports or duplicate procfs/conntrack reads)
- [ ] No unintended coupling (detector imports no responder; responders import only the event leaf)
- [ ] No duplicated functionality (reuses iface rate, trafficusage Snapshot, conntrack DeltaTracker)
- [ ] Zero-copy preserved (read-only consumer; no new wire encoding)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `RegisterCollectNotify` fires reliably at ~1 Hz with per-interface RxPps/RxBps | LSP-confirmed rate.go:66/59 | Stage 1 needs its own sampler | unit test the callback + a functional flood test | confirmed (API exists) |
| A-2 | trafficusage can expose per-IP/port counts via a NEW read-only DirectBridge handler | grep: `Monitor.Snapshot() []counts` exists (monitor.go:191) but private | Stage-2 target/dispersion weaker; fall back to flow snapshot only | add the handler + a unit test | refined — data exists, needs a DirectBridge handler |
| A-3 | flowexport can expose a conntrack flow snapshot / new-flow count via a NEW DirectBridge handler | grep: `DeltaTracker` exported (delta.go:77), owned by flowexport | Stage-2 family classification weaker; rely on trafficusage + tcp-flag mix | add the handler + a unit test | refined — needs a DirectBridge handler |
| A-4 | `events.Register[T]` supports the three payload types for 1→N broadcast | `ai/rules/plugin-design.md` EventBus | use a different notify mechanism | implement a consumer in a unit test | unvalidated |
| A-5 | A core leaf `internal/core/ddosevent` passes `make ze-tier-check` | module-tiers (leaf, no plugin imports) | put the contract elsewhere | run `scripts/dev/dep_audit.py --check` | unvalidated |
| A-6 | Stage-2 DirectBridge calls on attack onset complete fast enough to not delay the first event | DirectBridge is in-process typed call | first `AttackDetected` lags the attack | unit test latency; cap the snapshot size | unvalidated |
| A-7 | The Stage-1 rate signal works on BOTH dataplanes | survey: netlink populates `InterfaceInfo.Stats`, but the VPP backend's `iface/vpp/query.go detailsToInfo()` leaves `Stats=nil` and `rate.go:140` skips it → **blind on DPDK VPP**. VPP counters DO exist via `vpp/telemetry.go statsProvider.GetInterfaceStats` but are not fed to `iface` | detection dead on VPP boxes | fixed at source by prerequisite `spec-cp-survival-5-detect-1a-vpp-iface-rate` (wires VPP stats into `iface.InterfaceInfo.Stats`) | resolved by 1a (prerequisite, detector unchanged) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | False positive on a flash crowd (rate spike, benign pattern) | legit traffic flagged | Stage-2 pattern analysis is the FP filter; `confirm-duration`; emit-only until a responder is enabled |
| R-2 | Baseline poisoned by an attack present at startup | threshold drifts up; later attacks missed | absolute-floor trigger + startup-grace + never-add-during-attack guard (ported ftagent) |
| R-3 | Sub-aggregate attack on one service does not move the interface rate, so Stage 1 never fires | a service degrades while iface rate is normal | documented limitation of iface-only detection; per-target detection is a future enhancement (needs continuous trafficusage, deliberately deferred) |
| R-4 | `AttackCleared` emitted while a responder mitigates upstream (detector blind) | premature clear | event carries rate level + "observable" flag; child 3 ignores clear while mitigating and uses leak-probe |
| R-5 | Stage-2 analysis adds latency or load at the worst moment (attack onset) | first event delayed | bounded one-shot snapshot; DirectBridge in-process; no packet-path processing |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| synthetic flood raises iface RxPps past threshold | → | Stage-1 trigger → Stage-2 analysis → publishes `AttackDetected` | `ddos-detect-trigger.ci` |
| flood stops, RxPps falls for N ticks | → | detector → publishes `AttackCleared` | `ddos-detect-clear.ci` |
| benign rate spike (few sources, established flows) | → | Stage-2 suppresses → no event | `ddos-detect-falsepositive.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | iface RxPps exceeds `max(baseline p99 × multiplier, absolute-floor)` for `confirm-duration` | Stage 1 fires; Stage 2 runs |
| AC-2 | Stage-2 analysis of an attack-like pattern | detector emits one `AttackDetected` with target, family, vector tuple, top sources, peak rate, observable=true |
| AC-3 | Attack persists | detector emits `AttackOngoing` every `check-interval` with the current rate level |
| AC-4 | Observable rate stays below threshold for `clear-consecutive-checks` ticks | detector emits exactly one `AttackCleared` for that target |
| AC-5 | A node is already under attack at startup | attack-window samples excluded from baseline; the absolute-floor still triggers (no baseline poisoning) |
| AC-6 | A rate spike whose Stage-2 pattern is benign (low source dispersion, established flows) | detector does NOT emit `AttackDetected` (Stage-2 false-positive filter) |
| AC-7 | Detector running with no responder subscribed | events are published and dropped harmlessly; no error, no blocking |
| AC-8 | Stage-2 cannot reach trafficusage/flowexport (handler unavailable) | detector still emits `AttackDetected` from the rate trigger with a reduced characterization; logs the degradation (fail-open detection) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables the detector; a flood hits | iface rate trigger → Stage-2 analysis → `AttackDetected` on EventBus | `ddos-detect-trigger.ci` |
| 2 | the flood stops | rate falls → hysteresis → `AttackCleared` | `ddos-detect-clear.ci` |
| 3 | a flash crowd briefly spikes | rate up, Stage-2 pattern benign → suppressed | `ddos-detect-falsepositive.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBaselineExcludesAttackSamples` | `internal/plugins/ddosdetect/baseline_test.go` | attack-window/above-floor samples never enter the baseline (AC-5) | |
| `TestThresholdFloorAndMultiplier` | `internal/plugins/ddosdetect/baseline_test.go` | threshold = max(p99×multiplier, absolute-floor) | |
| `TestStateMachineTriggerHysteresis` | `internal/plugins/ddosdetect/state_test.go` | trigger after confirm-duration; clear after clear-consecutive-checks (AC-1, AC-4) | |
| `TestCharacteriseClassifiesAndFilters` | `internal/plugins/ddosdetect/characterise_test.go` | Stage-2 builds vector/family and suppresses benign patterns (AC-2, AC-6) | |
| `TestCharacteriseDegradesWhenBridgeUnavailable` | `internal/plugins/ddosdetect/characterise_test.go` | fail-open characterization (AC-8) | |
| `TestEventsPublishNoSubscriber` | `internal/plugins/ddosdetect/detector_test.go` | publishing with zero subscribers is safe (AC-7) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| check-interval (s) | 1-3600 | 3600 | 0 | 3601 |
| clear-consecutive-checks | 1-100 | 100 | 0 | 101 |
| confirm-duration (s) | 0-3600 | 3600 | N/A (0 = immediate) | 3601 |
| absolute-floor (pps) | 1-… | n/a | 0 | N/A |
| baseline-window (s) | 10-86400 | 86400 | 9 | 86401 |
| threshold-multiplier | 1.0-100.0 | 100.0 | 0.9 | 100.1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-detect-trigger` | `test/plugin/ddos-detect-trigger.ci` | synthetic flood trips Stage 1 + Stage 2; `AttackDetected` observed | |
| `ddos-detect-clear` | `test/plugin/ddos-detect-clear.ci` | flood stops; `AttackCleared` observed after hysteresis | |
| `ddos-detect-falsepositive` | `test/plugin/ddos-detect-falsepositive.ci` | benign spike suppressed by Stage 2 | |

### Interop Tests
N/A — the detector touches no wire protocol (owned by child 3). Justified: detection is local-signal only.

### Future (deferred tests)
- Per-target (sub-aggregate) detection (R-3) — deferred enhancement requiring continuous trafficusage.

## Files to Modify
- `internal/component/plugin/all/all.go` - add the detector to the composition root (regenerated by `make generate`)
- `internal/plugins/trafficusage/register.go` - register a read-only DirectBridge handler exposing a per-IP/port count snapshot
- `internal/plugins/flowexport/register.go` - register a read-only DirectBridge handler exposing a conntrack flow snapshot / new-flow count
- `docs/features.md` - add the "automatic DDoS detection" row (source-anchored)
- `docs/guide/plugins.md` - list the `ddosdetect` plugin

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (config) | Yes | `internal/plugins/ddosdetect/yang/ze-ddos-detect-conf.yang` — thresholds + the two detector timers; native `range`/`type` per `ai/patterns/config-option.md` |
| DirectBridge handlers (Stage 2) | Yes | `trafficusage` + `flowexport` read-only snapshot handlers (`pkg/plugin/rpc/bridge.go` pattern) |
| Functional test | Yes | `test/plugin/ddos-detect-*.ci` |
| Prometheus counters | Yes | detector state gauges (threshold, baseline, attack-active, events emitted) — defined here, surfaced in child 4 |
| Doctor check | Deferred to child 4 | runtime-dependency checks live with observability |
| Env var registration | No | all settings are YANG config |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 8 | Plugin SDK/protocol changed? | Yes (new DirectBridge handlers) | `ai/rules/plugin-design.md` / `docs/architecture/api/process-protocol.md` |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin/event type changed? | Yes | `docs/plugin-overview.md` |

## Files to Create
- `internal/core/ddosevent/event.go` - shared payload contract: `AttackDetected`, `AttackOngoing`, `AttackCleared` (target, family, vector, sources, rate level, observable flag)
- `internal/plugins/ddosdetect/register.go` - plugin registration + YANG binding
- `internal/plugins/ddosdetect/detector.go` - Stage-1 rate subscription, tick loop, event emission
- `internal/plugins/ddosdetect/baseline.go` - rolling baseline, poisoning guards, floor, startup grace
- `internal/plugins/ddosdetect/state.go` - trigger/clear state machine + the two detector timers
- `internal/plugins/ddosdetect/characterise.go` - Stage-2 on-attack analysis (DirectBridge reads, entropy, classify, FP filter)
- `internal/plugins/ddosdetect/config.go` - config parse + validation
- `internal/plugins/ddosdetect/yang/ze-ddos-detect-conf.yang` - config schema
- `internal/plugins/ddosdetect/*_test.go` - unit tests above
- `test/plugin/ddos-detect-trigger.ci`, `ddos-detect-clear.ci`, `ddos-detect-falsepositive.ci` - functional tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the umbrella |
| 2. Audit | Files to Create/Modify, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — event leaf `internal/core/ddosevent` + plugin skeleton (`register.go`, YANG) + a failing `ddos-detect-trigger.ci`; detector subscribes to `RegisterCollectNotify` but emits nothing yet.
   - Verify: plugin registers; `make generate` wires it into `all.go`; wiring test fails (trigger is a stub).
2. **Phase: Stage-1 baseline + trigger** — rolling baseline (poisoning guards, floor, startup grace) + state-machine trigger/clear + event emission.
   - Tests: `TestBaselineExcludesAttackSamples`, `TestThresholdFloorAndMultiplier`, `TestStateMachineTriggerHysteresis`, `TestEventsPublishNoSubscriber`; `ddos-detect-trigger.ci` + `ddos-detect-clear.ci` pass with a minimal characterization.
3. **Phase: Stage-2 bridge handlers** — add read-only DirectBridge handlers to trafficusage + flowexport.
   - Tests: handler unit tests in each plugin.
4. **Phase: Stage-2 characterization + FP filter** — entropy, new-flow rate, classify, benign-suppression.
   - Tests: `TestCharacteriseClassifiesAndFilters`, `TestCharacteriseDegradesWhenBridgeUnavailable`; `ddos-detect-falsepositive.ci`.
5. **Full verification** → `make ze-verify-changed` + `make generate`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | each AC-N has implementation with file:line |
| Correctness | baseline never includes attack samples; Stage 2 only runs on trigger; one event per state transition |
| Naming | YANG kebab-case; event type names match `internal/core/ddosevent` |
| Data flow | detector imports no responder; Stage 2 via DirectBridge, not private imports |
| Goroutine lifecycle | tick loop stopped on shutdown; no leak |
| Prometheus counters | state gauges defined + registered |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| detector plugin registered | `make ze-inventory` lists `ddosdetect` |
| three event types | grep `events.Register` in `internal/core/ddosevent` |
| two DirectBridge handlers | grep bridge registration in trafficusage + flowexport |
| functional tests pass | `ze-test bgp plugin test/plugin/ddos-detect-*.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Stage-2 snapshot bounded (top-N sources/targets); no unbounded maps |
| False positive blast | Stage-2 filter cannot be trivially defeated; emit-only until responder enabled |
| Input validation | YANG numeric ranges enforced (boundary table) |
| Fail-open | bridge unavailable degrades characterization, never crashes detection (AC-8) |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- **Sub-aggregate attacks are not detected.** Stage 1 watches only the interface rate, so an attack on
  one service that does not move the aggregate will not fire (R-3). Per-target detection is a deferred
  enhancement requiring continuous trafficusage sampling.
- **VPP dataplane detection signal (A-7).** On a DPDK VPP dataplane the kernel-derived `iface` rate is
  blind (`detailsToInfo` leaves `Stats=nil`). VPP's own per-interface counters exist
  (`vpp/telemetry.go statsProvider.GetInterfaceStats`) but are not yet wired into `iface.InterfaceInfo`.
  Resolved at source by prerequisite `spec-cp-survival-5-detect-1a-vpp-iface-rate` (populates
  `iface.InterfaceInfo.Stats` from VPP's stats segment), making the detector dataplane-agnostic with no
  VPP-specific detector code.
- The detector emits `AttackCleared` only from the rate signal it can still observe; under upstream
  FlowSpec drop, clear is a responder concern (child 3 leak-probe), flagged via the "observable" flag.

## Design Insights
- Two-stage detection mirrors `ftagent`: a near-free always-on rate trigger, and expensive pattern
  analysis paid for only during the rare incident — which also yields the surgical match the responders
  need and a built-in false-positive filter.

## Implementation Audit

| AC | Evidence |
|----|----------|
| AC-1 | Stage-1 trigger via `baseline.Threshold()` + `stateMachine.Tick(above)` (`detector.go:105-107`); `TestDetectorEmitsOnFlood` passes |
| AC-2 | `emitDetected` emits `AttackDetected` with target, family, peak rates (`detector.go:121-131`); Stage-2 characterization deferred (AC-6/AC-8 stubs) |
| AC-3 | `AttackOngoing` emitted each tick while active (`detector.go:115-120`); event carries current rates |
| AC-4 | `stateMachine.Tick(false)` × `clearChecks` → `OnCleared` → `emitCleared` (`state.go:83-93`); `TestStateMachineClearAfterConsecutiveChecks` passes |
| AC-5 | `baseline.Add` excludes attack+above-threshold samples (`detector.go:105`); absolute floor via `baseline.Threshold()` max; `TestBaselineExcludesAttackSamples` passes |
| AC-7 | `TestEmitNoSubscriber` passes (ddosevent/event_test.go) |

### Files created
- `internal/core/ddosevent/event.go` -- shared event contract (3 event types, VectorTuple, AttackFamily)
- `internal/core/ddosevent/event_test.go` -- 5 tests
- `internal/plugins/ddosdetect/baseline.go` -- rolling baseline with poisoning guard
- `internal/plugins/ddosdetect/baseline_test.go` -- 6 tests
- `internal/plugins/ddosdetect/state.go` -- trigger/clear state machine
- `internal/plugins/ddosdetect/state_test.go` -- 7 tests
- `internal/plugins/ddosdetect/detector.go` -- Stage-1 rate subscription, tick loop, event emission
- `internal/plugins/ddosdetect/detector_test.go` -- 2 tests
- `internal/plugins/ddosdetect/config.go` -- config parse + validation
- `internal/plugins/ddosdetect/register.go` -- plugin registration, YANG, lifecycle
- `internal/plugins/ddosdetect/yang/` -- YANG schema, embed, register

### Deferred to follow-up
- Stage-2 characterization (AC-6, AC-8): DirectBridge handlers in trafficusage/flowexport not yet wired; detector emits generic family
- Functional .ci tests: require the full test harness; unit tests cover the logic

## Review Gate
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ddosdetect`, `internal/core/ddosevent`, the two bridge handlers)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-5-detect-1-detector.md`
