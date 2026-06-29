# Spec: Shared Traffic-Observation Feed

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - (foundational; BLOCKS spec-cp-survival-5-detect-6-behavioural) |
| Phase | 9/9 |
| Updated | 2026-06-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. Source: `internal/component/iface/rate.go` (single-callback hook + tracker), `internal/component/iface/register.go:649-651` (tracker start)
4. The three current consumers: `internal/plugins/flowexport/register.go:103`, `internal/plugins/trafficusage/register.go:66`, `internal/plugins/ddos/detect/register.go:91`

## Task

Unify how traffic observations are DISPATCHED inside Ze. Today multiple plugins each
collect overlapping/complementary traffic data and each reaches for the shared 1 Hz
interface tick through a hook that supports only ONE subscriber, so they silently
conflict. Replace that with a single shared observation feed: one normalized
contract, a multi-subscriber fan-out, and a publish path so the existing collectors
(eBPF / sFlow-sample / conntrack) expose their per-source/per-flow detail to ANY
in-system consumer (metrics export, flow export, detectors) instead of each feature
reaching into the next.

This spec is FOUNDATIONAL and lands FIRST. The behavioural-detection spec
(`spec-cp-survival-5-detect-6-behavioural`) consumes this feed and is blocked on it.

### Motivating defect (found during research -- verified)

All three plugins run as goroutines in ONE process (`plugin/process/process.go:456-460`,
auto-loaded `Internal: true`). The iface rate tracker runs once in the engine
(`iface/register.go:649-651`) and exposes a SINGLE-callback hook
(`iface/rate.go:66` `RegisterCollectNotify`; `rate.go:176` invokes the one callback;
last registration wins). Three plugins register on it:
- `flowexport` `register.go:103` (`notifyFromRateTracker` -> counter export)
- `trafficusage` `register.go:66` (`mon.onSnapshot` -> interface re-resolve)
- `ddos/detect` `register.go:91` (`det.onRate` -> detection)
With more than one enabled, only the LAST-registered callback fires; the others lose
their tick-driven behaviour. No code multiplexes the callbacks. This is a latent
defect this spec fixes as a side effect of unifying the feed.

### Agreed scope (settled with user -- treat as fixed)
- IN: (1) a normalized `Observation` contract in a core leaf; (2) replace the iface
  single-callback with a MULTI-SUBSCRIBER fan-out; (3) migrate the three existing
  per-interface consumers onto it with NO behavioural regression; (4) the existing
  collectors (eBPF / sFlow-sample / conntrack) PUBLISH their per-source/per-flow
  observations into the feed (a thin publish hook, NOT a kernel-collection rewrite).
- OUT: rewriting any kernel collection mechanism (eBPF/tc-psample/conntrack stay);
  the behavioural detector itself (Spec 2); new collectors; new wire protocols.

### Collection is NOT duplicated (do not "merge" collectors)
Different kernel facilities, different fidelity -- all must be preserved:
| Collector | Mechanism | Yields |
|-----------|-----------|--------|
| trafficusage | eBPF TCX (`attach_linux.go`) | EXACT aggregate byte counters per port/IP (IPv4 per-IP) |
| flowexport sampling | tc sample + psample netlink (`sampling_worker.go:59,71`) | SAMPLED packet headers (1-in-N) |
| flowexport conntrack | netlink conntrack dump (`conntrack/reader_linux.go`) | PER-FLOW records (5-tuple) |
| iface rate tracker | `ListInterfaces()` 1 Hz (`rate.go:119`) | per-interface rate/counter deltas |

## Required Reading

### Architecture Docs
- [ ] `ai/rules/module-tiers.md` - the Observation contract + fan-out registry is config-free → `internal/core` leaf
- [ ] `ai/rules/plugin-self-containment.md` - collectors publish; consumers subscribe; no consumer imports a collector
- [ ] `docs/architecture/core-design.md` - registration + EventBus model (in-process vs cross-process delivery)

**Key insights:**
- All current consumers are in-process (the single-callback is a Go package global, in-process only). The feed can be an in-process multi-subscriber registry (extends `RegisterCollectNotify`) OR ride the EventBus (which crosses process boundaries, used by `ddosevent`). DESIGN must pick -- see A-1.
- "Fix the conflict" = turn one callback slot into N subscribers. Small and high-value on its own.
- "Serve the detector" = collectors publish per-source/per-flow `Observation`s; this is the larger, regression-sensitive part (touches flowexport + trafficusage).

## Current Behavior (MANDATORY)
- [ ] `internal/component/iface/rate.go` - single-callback `RegisterCollectNotify` (`:66`), `collectNotifyPtr` single atomic (`:61`), invoked once per 1 Hz `collect()` (`:176`); `globalTracker` single per-process (`:54`)
  → Constraint: last-registration-wins; replacing it must preserve the existing payload (`[]InterfaceInfo`) for current consumers during migration.
- [ ] `internal/plugins/flowexport/register.go:103` + `exporter.go:136 NotifySnapshot` - counter-export path driven by the tick
- [ ] `internal/plugins/trafficusage/register.go:66` + `monitor.go:128 onSnapshot` - interface re-resolve driven by the tick (eBPF poll is separate, `monitor.go:227`)
- [ ] `internal/plugins/ddos/detect/register.go:91` + `detector.go:63 onRate` - detection driven by the tick
- [ ] `internal/core/ddosevent/event.go` - existing EventBus contract pattern (candidate delivery mechanism)

**Behavior to preserve:** every current consumer's tick-driven behaviour after migration (flowexport counter export, trafficusage re-resolve, ddos detection) with NO regression.

## Data Flow (MANDATORY)

### Entry Point
- Per-interface tick: `iface` rate tracker `collect()` (1 Hz, `rate.go:119`) -> currently ONE callback.
- Per-source/per-flow: collector workers (trafficusage poll `monitor.go:248`; flowexport
  `ExportFlows` `exporter.go:230`; flowexport `ExportFlowSample` `sampling_worker.go:114`).

### Transformation Path (target)
1. iface `collect()` fans out `[]InterfaceInfo` to N subscribers (was 1). Existing
   consumers (flowexport/trafficusage/ddos-detect) all receive every tick. PAYLOAD
   UNCHANGED for them (no-regression migration).
2. A new core leaf `internal/core/observation` defines a normalized `Observation`
   {Entity(kind,key), Feature, Value, At} and a typed multi-subscriber `Feed`
   (Publish/Subscribe) -- in-process, no JSON, no diagnostic-ring append.
3. Collectors PUBLISH normalized observations additively at their existing export
   points: trafficusage -> per-source-IP byte observations; conntrack -> per-flow
   observations; (optional) sampling -> per-sample observations.
4. Any consumer (Spec 2 detector, future) SUBSCRIBES once to `Feed` and receives
   observations from all collectors -- the "one place to read" property.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface tick -> consumers | multi-subscriber fan-out (`[]InterfaceInfo`) | [ ] |
| collectors -> feed | `observation.Feed.Publish` (typed, in-process) | [ ] |
| feed -> consumers | `observation.Feed.Subscribe` | [ ] |

### Integration Points
- `internal/component/iface/rate.go` `collect()` (`:176`) - the tick fan-out point; replace the single-callback invoke with multi-subscriber dispatch
- `internal/plugins/trafficusage/monitor.go` `poll()` (`:248`) - publish per-source-IP observations here
- `internal/plugins/flowexport/exporter.go` `ExportFlows` (`:230`) - publish per-flow observations here
- `internal/core/observation` (new) - the shared `Feed` both collectors and consumers bind to

### Architectural Verification
- [ ] Core leaf has no component/plugin imports (tier-check passes)
- [ ] Collectors publish; no consumer imports a collector (self-containment)
- [ ] No per-tick / per-publish heap alloc on the hot path
- [ ] Existing consumers' behaviour unchanged after migration

## Observation Contract & Delivery (PINNED)

### `Observation` fields (alloc-free value type -- no pointers)
| Field | Type | Meaning |
|-------|------|---------|
| Kind | enum: `Interface` / `SourceIP` / `DestIP` / `Flow` | entity granularity (tells consumers which `Flow` fields are meaningful) |
| Iface | string | interface-name context (`""` when N/A) |
| Flow.Src | `netip.Addr` | source IP (set for `SourceIP` and `Flow`) |
| Flow.Dst | `netip.Addr` | dest IP (set for `Flow` only) |
| Flow.SrcPort / Flow.DstPort | uint16 | ports (set for `Flow` only) |
| Flow.Proto | uint8 | IP protocol (set for `Flow` only) |
| Feature | enum: `RxBytes` / `RxPackets` / `FlowBytes` / `FlowPackets` / `NewFlowCount` | what is measured |
| Value | float64 | collector-native value |
| At | timestamp | sample time |

→ Decision: `Flow` is an embedded value struct (Src/Dst `netip.Addr`, ports, proto), NOT
  a pointer -- keeps `Observation` heap-alloc-free on the hot path. Per-source obs set
  `Flow.Src` only; per-flow obs set all; interface obs use `Iface` with `Flow` zero.

### Value semantics (PINNED)
- Byte/packet `Feature`s carry CUMULATIVE absolute counters (collector-native).
  Consumers derive rates/deltas from consecutive `Value`+`At`, exactly as the existing
  detector already does (`detector.go:89-97`). `NewFlowCount` is a per-publish increment.
- → Decision: absolute-over-delta so the feed is lossless and stateless; rate derivation
  stays in the consumer (no per-subscriber delta state in the feed).

### Delivery / backpressure (PINNED)
- Each subscriber owns a bounded buffered channel (default cap 1024) drained on its OWN
  goroutine; subscriber handler code never runs on the publisher's goroutine.
- Publisher dispatch = non-blocking send; on full buffer, drop the INCOMING observation
  and increment `ze_observation_dropped_total{subscriber}`. Publisher never blocks.
- Subscriber set is an atomic-pointer copy-on-write snapshot: dispatch iterates a
  preallocated slice with zero per-publish allocation (AC-8).
- Buffer cap (1024) is an internal safety constant, NOT YANG (config-surface safety-cap
  exception); the drop counter makes overflow observable.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The high-volume feed needs a dedicated in-process typed transport, NOT the EventBus | EventBus `deliverEvent` appends to a fixed-size diagnostic ring per event (`dispatch.go:403-404`, `event_ring.go:47`) + per-event validation/ID lookup (`dispatch.go:398-410`); per-source/per-flow volume would thrash the 1024-entry ring | using EventBus floods diagnostics + adds per-event cost | read producers (DONE) | CONFIRMED: in-process typed multi-subscriber feed (evolved `RegisterCollectNotify`) for high-volume; EventBus reserved for low-volume incident events (as `ddosevent` does). All current consumers are in-process so the typed feed reaches them |
| A-2 | Collectors can publish per-source/per-flow observations via a thin hook without restructuring their kernel collection | eBPF/sample/conntrack producers are isolated workers | publish hook forces collector redesign | read each collector's export point | CONFIRMED: trafficusage `publishLocked` has per-source-IP `counts.ingressIP` (IPv4 uint32 keys); flowexport `ExportFlows` has `[]ConntrackFlow` with full 5-tuple. Both are additive publish points under existing locks. Note: conntrack `FlowBytes`/`FlowPackets` are deltas not cumulative; flow-sourced observations carry deltas per the `NewFlowCount` precedent |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Migrating the three tick consumers regresses one of them (lost counter export / re-resolve / detection) | a consumer's functional test fails after migration | migrate one at a time; keep payload identical; per-consumer regression test |
| R-2 | Per-source/per-flow publishing balloons into a flowexport+trafficusage rewrite | diff touches kernel-collection code | bound to an additive publish call at the existing export point; no collector logic change |
| R-3 | Fan-out adds per-tick allocation/cost on a 1 Hz hot path | alloc in pprof on `collect()` | preallocated subscriber slice; no per-tick alloc; bounded subscriber count |
| R-4 | Delivery-mechanism choice (registry vs EventBus) wrong → rework | external-mode consumer can't subscribe | A-1 prototype before committing |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| flowexport + trafficusage + ddos-detect all enabled | → | iface multi-subscriber fan-out delivers the tick to all three | `TestRateTrackerFanoutAllSubscribers` |
| collector publishes an observation | → | `observation.Feed.Publish` → subscriber handler | `TestObservationFeedDeliversToSubscriber` |
| Spec 2 detector subscribes | → | `observation.Feed.Subscribe` receives per-source observations | (Spec 2 wiring; here: a stub subscriber test) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Three callbacks registered on the iface tick, `collect()` fires | ALL three are invoked each tick (single-callback conflict fixed) |
| AC-2 | flowexport, trafficusage, ddos-detect migrated to the new API and all enabled | Each keeps its prior tick-driven behaviour; existing functional tests still pass (no regression) |
| AC-3 | Core leaf `internal/core/observation` exists | Defines `Observation{Entity,Feature,Value,At}` + typed multi-subscriber `Feed`; no JSON marshal, no diagnostic-ring append per publish |
| AC-4 | trafficusage poll with a per-source-IP byte count | An `Observation` (entity=source-IP, feature=bytes) is published to the feed |
| AC-5 | flowexport `ExportFlows` with a flow record | A per-flow `Observation` is published to the feed |
| AC-6 | One consumer Subscribes once; trafficusage AND conntrack both publish | The consumer receives observations from both sources via the single subscription |
| AC-7 | A subscriber unsubscribes (or its plugin stops) | It stops receiving; no goroutine/handler leak |
| AC-8 | High publish rate | Fan-out adds no per-tick/per-publish heap allocation (preallocated slice); alloc benchmark = 0 allocs/op on the dispatch path |
| AC-9 | A slow/blocking subscriber under load | Publisher does a NON-BLOCKING send to the subscriber's bounded buffered channel (default cap 1024); on full, the INCOMING observation is dropped and `ze_observation_dropped_total{subscriber}` increments. The publisher (iface tick / collector) is never stalled. Each subscriber drains on its OWN goroutine |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables flow-export, traffic-usage, and the DDoS detector together | iface `collect()` → fan-out → all three callbacks | `TestRateTrackerFanoutAllSubscribers` + each plugin's existing functional test |
| 2 | (operator-invisible) a new consumer reads all traffic observations from one place | collectors → `observation.Feed` → subscriber | `TestObservationFeedMultiCollector` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRateTrackerFanoutAllSubscribers` | `internal/component/iface/rate_test.go` | multi-subscriber delivery (AC-1) | |
| `TestRateTrackerUnsubscribe` | `internal/component/iface/rate_test.go` | unsubscribe stops delivery (AC-7) | |
| `TestObservationFeedDeliversToSubscriber` | `internal/core/observation/observation_test.go` | publish→subscribe typed (AC-3) | |
| `TestObservationFeedMultiCollector` | `internal/core/observation/observation_test.go` | one subscriber, many publishers (AC-6) | |
| `BenchmarkObservationFeedPublish` | `internal/core/observation/observation_test.go` | 0 allocs/op on dispatch (AC-8) | |
| `TestObservationFeedSlowSubscriber` | `internal/core/observation/observation_test.go` | publisher not stalled (AC-9) | |
| `TestTrafficUsagePublishesSourceObs` | `internal/plugins/trafficusage/*_test.go` | per-source obs published (AC-4) | |
| `TestConntrackPublishesFlowObs` | `internal/plugins/flowexport/*_test.go` | per-flow obs published (AC-5) | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| subscriber count | 0..cap | cap | N/A | cap+1 rejected/logged |
| feed buffer (if bounded) | 1..N | N | 0 | overflow drops + counter |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-observation-coexist` | `test/plugin/*.ci` | enable all three traffic plugins; all keep working | |

### Interop Tests
N/A — no wire-protocol change (internal dispatch only).

## Files to Modify
- `internal/component/iface/rate.go` - single `RegisterCollectNotify` → multi-subscriber registry (Subscribe/Unsubscribe); preserve `[]InterfaceInfo` payload
- `internal/plugins/flowexport/register.go` - migrate to Subscribe (`:103`,`:209`)
- `internal/plugins/trafficusage/register.go` - migrate to Subscribe (`:66`,`:139`,`:145`)
- `internal/plugins/ddos/detect/register.go` - migrate to Subscribe (`:91`,`:118`)
- `internal/plugins/trafficusage/monitor.go` - publish per-source-IP observations at poll (`:248`)
- `internal/plugins/flowexport/exporter.go` (or `conntrack_worker.go`) - publish per-flow observations at `ExportFlows` (`:230`)
- `internal/plugins/flowexport/sampling_worker.go` - (optional) publish per-sample observations (`:114`)
- `docs/architecture/` - new observation-feed design doc; update iface tick doc

## Files to Create
- `internal/core/observation/observation.go` - `Observation` contract + typed multi-subscriber `Feed`
- `internal/core/observation/observation_test.go` - feed unit + bench tests
- `test/plugin/observation-coexist.ci` - functional coexistence test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | feed is internal infra; no operator config |
| CLI commands | No | — (a `show` of feed stats is optional, deferred) |
| Functional test | Yes | `test/plugin/observation-coexist.ci` |
| Doctor check | No | no new runtime dependency (no new socket/path/module) |
| Prometheus counters | Yes | feed published/dropped counters + subscriber gauge; register in telemetry |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | internal infra; no user surface |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` / plugin-overview: note shared feed |
| 12 | Internal architecture changed? | Yes | new `docs/architecture/.../observation-feed.md`; update iface tick doc |
| 14 | Prometheus counters added? | Yes | telemetry doc: feed counters |
| 16 | Changed source referenced by doc anchors? | Yes | grep docs for `rate.go` / `RegisterCollectNotify` anchors; update |

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (FIRST)** — multi-subscriber iface fan-out + failing `TestRateTrackerFanoutAllSubscribers`.
   - Files: `iface/rate.go`. Verify: 3 callbacks all fire.
2. **Phase: Migrate consumers (one at a time, no regression)** — flowexport, then trafficusage, then ddos-detect move to Subscribe; each plugin's existing tests stay green; add coexistence test.
3. **Phase: Observation core leaf** — `internal/core/observation` contract + `Feed`; tests AC-3/6/8/9.
4. **Phase: trafficusage publish** — per-source-IP observations at poll; test AC-4.
5. **Phase: conntrack publish** — per-flow observations at ExportFlows; test AC-5.
6. **Phase: (optional) sampling publish** — per-sample observations.
7. **Phase: metrics + docs** — feed counters; architecture doc; doc-anchor updates.
8. **Full verification** → `make ze-verify`.
9. **Complete spec** → audit + learned summary; two-commit closure.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| In-process typed `Feed` for high-volume observations | EventBus (`ddosevent` pattern) | EventBus appends a diagnostic-ring record per event (`dispatch.go:403`) + per-event validation; per-source/per-flow volume would thrash the 1024-entry ring. Typed in-process feed = no serialize, no ring append |
| Keep `[]InterfaceInfo` payload for the iface tick fan-out | Convert all consumers to `Observation` immediately | identical payload = no-regression migration; normalization is additive, consumers move later |
| Observation `Feed` in `internal/core` leaf | put in a component/plugin | config-free pure pub/sub; tier-valid; importable by collectors AND consumers without cycles |
| Collectors keep their kernel mechanisms; publish is additive | unify collection | different facilities/fidelity (eBPF exact / sFlow sampled / conntrack per-flow); merging loses capability |

## Critical Review Checklist
| # | What to verify | Pass? |
|---|----------------|-------|
| 1 | Fan-out delivers to ALL subscribers every tick (AC-1) | |
| 2 | Existing consumers' behaviour unchanged after migration (AC-2) | |
| 3 | No per-tick/per-publish heap allocation on dispatch (AC-8) | |
| 4 | Slow subscriber cannot stall publisher (AC-9) | |
| 5 | Core leaf has no component/plugin imports (tier-check) | |
| 6 | No consumer imports a collector (self-containment) | |

## Deliverables Checklist
| # | Deliverable | Verification |
|---|-------------|-------------|
| 1 | `internal/core/observation/observation.go` | `ls` + tier-check |
| 2 | Multi-subscriber iface fan-out | `TestRateTrackerFanoutAllSubscribers` passes |
| 3 | Three consumers migrated | existing functional tests pass |
| 4 | trafficusage publishes per-source obs | `TestTrafficUsagePublishesSourceObs` passes |
| 5 | conntrack publishes per-flow obs | `TestConntrackPublishesFlowObs` passes |
| 6 | Feed metrics registered | grep for counter names |
| 7 | Architecture doc | `ls docs/architecture/` |

## Security Review Checklist
| # | Concern | Check |
|---|---------|-------|
| 1 | Unbounded subscriber count | subscriber cap or documented policy |
| 2 | Channel buffer exhaustion under load | drop + counter, publisher never blocks |
| 3 | Goroutine leak on unsubscribe | subscriber goroutine exits on close |
| 4 | Data race on subscriber list mutation | atomic-pointer CoW or mutex |

## Known Limitations
- Does not change kernel collection mechanisms or add collectors.
- Does not build the behavioural detector (Spec 2).

## Review Gate
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-observation-feed.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-observation-feed.md` only
