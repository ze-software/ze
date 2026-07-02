# Spec: Anomaly End-to-End Interop Harness (fakeflow injector)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | 1046 (traffic-analysis), 1048 (anomaly-1-detect), 1049 (anomaly-2-shape), spec-anomaly-0-umbrella |
| Phase | - |
| Updated | 2026-07-02 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-anomaly-0-umbrella.md` - parent umbrella (child 4, Phase A, the Phase-B gate)
4. `internal/test/plugins/fakeredist/register.go` - the test-only-plugin template this copies
5. `internal/component/trafficfeature/feature.go` - the ingest gate the injector must satisfy

## Task

The anomaly facts→judgment→response chain is unit-tested per layer, but **no test drives the
whole chain through the daemon**. The three existing anomaly functional tests prove wiring with
empty data and say so: `test/plugin/anomaly-show.ci:5-8` ("the .ci harness cannot synthesize
per-source feature deviations without a traffic generator"), `test/plugin/anomaly-shape-shadow.ci`
("cannot synthesize anomalyevent incidents without the detector + traffic").

Build the missing test: an **in-process Go integration test** (`TestChainFactsToResponse`) that
wires a real `trafficfeature.Service`, the real `anomaly/detect` detector, and the real
`anomaly/shape` responder together and drives them with synthetic `observation.Feed` records — a
same-/24 normal cohort (balanced in+out traffic) plus one outlier that goes pure-outbound with
high fan-out on rare ports. It asserts the detector confirms an incident from that real feature
data AND the responder arms the outlier specifically — proving `observation.Feed.Publish` →
`trafficfeature.ingest` → `Snapshot` → `detect.onTick` → emit `anomaly-detect` → `shape.onDetected`
→ armed action, end to end. It becomes the regression gate before Phase B widens any layer.

**Pivot (why not a `.ci`):** the original plan was a test-only `fakeflow` plugin driving the chain
through the daemon via a `.ci`. That was BUILT and then abandoned after the `.ci` proved it cannot
work: `observation.Feed` is a PROCESS-LOCAL bus (observation.go:86), and a config-`plugin`-loaded
plugin runs isolated from the engine's in-engine `trafficfeature` in the functional-test DUT, so
injected observations never reach it (a `selfcheck` diagnostic showed the injector's own publish is
received in its own process while the engine's `trafficfeature` stayed degraded). The user chose the
in-process Go integration test, which co-locates the whole chain in one process and is deterministic.
See the Mistake Log.

Scope boundary: this spec adds ONLY the integration test and a minimal test-composition helper
(`shape.SubscribeForTest`). It changes NO production behavior.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-anomaly-0-umbrella.md` - parent; the harness is child 4 and the gate for Phase B
  → Constraint: the harness proves the chain BEFORE any Phase-B widening; it must fire a real incident, not just check wiring.
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` functional-test format
  → Constraint: a `.ci` is a black-box daemon test; it cannot call Go APIs directly — it drives the DUT via `dispatch-command` RPCs, so feature injection must go through an in-process plugin command.
- [ ] `plan/learned/1048-anomaly-1-detect.md` - detector scoring + freeze-learn + warmup
  → Constraint: an incident needs warmup (`warmupTicks=3`) THEN `ConfirmDuration` consecutive above-threshold ticks; cohort rarity is leave-one-out within the source's `/24`/`/48`, so the outlier and its normal peers must share a prefix bucket to make rarity fire.
- [ ] `plan/learned/1049-anomaly-2-shape.md` - responder state machine
  → Constraint: `armed-count` only moves when `Mode != shadow` (`responder.go:82`); the DUT firewall backend must accept the re-register for arming to succeed.

### Source files read (Current Behavior grounding)
- [ ] `internal/test/plugins/fakeredist/register.go` - `init()` → `registry.Registration{Name,Description,RunEngine,ConfigureEngineLogger,ConfigureEventBus,CLIHandler}`; `runPlugin(conn)` uses `sdk.NewWithConn` + `p.OnExecuteCommand(dispatchCommand)` + `p.Run(ctx, sdk.Registration{Commands: []sdk.CommandDecl{{Name:"request fakeredist emit"}, ...}})` (register.go:30-77). The exact template to copy.
- [ ] `internal/test/plugins/all/all.go` - blank-imports each test-only plugin (+ its yang subpackage); production `cmd/ze` does NOT import this package (all.go:4-8). Add the `fakeflow` blank import here.
- [ ] `cmd/ze/plugins_zetest.go` - `//go:build zetest` blank-imports `internal/test/plugins/all`; this is how test plugins reach the DUT without appearing in production (plugins_zetest.go:1-10).
- [ ] `internal/core/observation/observation.go` - `Feed.Publish(obs)` (observation.go:180) fans out non-blocking; `Global()` returns the process-global feed (observation.go:220). `Observation{Kind,Iface,Flow,Feature,Value,At}` (observation.go:62-69); `FlowKey{Src,Dst,SrcPort,DstPort,Proto}` (observation.go:54-60), no ASN field.
- [ ] `internal/component/trafficfeature/feature.go` - `ingest` folds only `KindFlow`/`FeatureFlowBytes` with `Value>0` (feature.go:102-109); source accumulates `outBytes`/`dests`(fan-out)/`ports`; a source is emitted only when `outBytes>0` that window (feature.go:169-189) with features FanOut/OutInRatio/PortEntropy/NewPeer/RarePort/Beaconing.
- [ ] `internal/plugins/anomaly/detect/config.go` - tunable knobs: `DeviationThreshold=3.0`, `MinFeaturesToCorrelate=2`, `ConfirmDuration=3`, `BaselineWindow=300` (config.go:32-38), each validated + settable (config.go:62-111). The `.ci` lowers these for a deterministic, fast fire.
- [ ] `internal/plugins/anomaly/shape/responder.go` - `onDetected` arms unless `killed || Mode==ModeShadow` (responder.go:77-83); `registerTables`/`applyAll` hit the real firewall (responder.go:41-42).
- [ ] `test/plugin/anomaly-show.ci` - existing wiring-only test (empty ring), ALREADY migrated to the nested config restructure: config is `anomaly { detect { enabled true } }` (not flat `anomaly-detect {}`) and the driver dispatches `show anomaly detect` (anomaly-show.ci:41). Pattern to extend: `wait_for_post_startup` then `dispatch(api,'show anomaly detect')` and assert `enabled`/`incidents`, `timeout 15s` (anomaly-show.ci:117).
- [ ] `internal/plugins/anomaly/detect/yang/ze-anomaly-detect-conf.yang` - config is NESTED (revision 2026-07-02): `container anomaly { container detect { leaf enabled; leaf confirm-duration; leaf baseline-window; leaf deviation-threshold; … } }` (yang:13-69). Shape augments `/ad:anomaly` with `container shape { leaf mode … }`.
  → Constraint: the harness `.ci` config MUST be `anomaly { detect { enabled true; confirm-duration 2; … } shape { mode armed } }`; the operator-facing show commands are `show anomaly detect` / `show anomaly shape`.

**Key insights:**
- The only injection seam is `observation.Global().Publish`; a black-box `.ci` reaches it only through an in-process plugin command (`fakeredist` pattern). Test-only plugins run in-process, so `Publish` reaches the DUT's `trafficfeature` subscriber.
- Firing is about DEVIATION, not volume: freeze-learn folds a lone source's own values into its baseline, and a lone source is its own cohort. The injector must supply a same-prefix normal cohort plus one outlier, sustained across warmup+confirm ticks. The cheapest levers are the binary `new-peer`/`rare-port` signals plus a high fan-out + high out/in ratio outlier.
- Determinism comes from lowering the detector knobs (e.g. `confirm-duration 2`, `baseline-window 10`, `deviation-threshold 2`) and polling `show anomaly detect` rather than sleeping a fixed time.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/plugins/fakeredist/register.go` - template for a test-only in-process plugin exposing `request <name> <verb>` commands via `sdk.CommandDecl`.
- [ ] `internal/core/observation/observation.go` - `Publish`/`Global` publish seam; `Observation`/`FlowKey` shapes.
- [ ] `internal/component/trafficfeature/feature.go` - `ingest`/`snapshot` gates that decide whether an injected flow becomes a scored source feature.
- [ ] `internal/plugins/anomaly/detect/config.go` - the tunable thresholds the harness lowers.
- [ ] `internal/plugins/anomaly/shape/responder.go` - the non-shadow arming gate.
- [ ] `test/plugin/anomaly-show.ci` - the `.ci` driver + config shape to extend.

**Behavior to preserve:**
- Production builds MUST NOT load `fakeflow` (zetest tag + blank import only).
- The three existing anomaly `.ci`s keep passing unchanged.
- No production source file changes behavior; only test infrastructure is added.

**Behavior to change:**
- None in production. Add a test-only injector plugin and one functional test.

## Data Flow (MANDATORY)

### Entry Point
- `.ci` driver issues `dispatch-command` RPC `request fakeflow inject <src> <dst> <dstport> <bytes> [<count>]` to the DUT.

### Transformation Path
1. In-process `fakeflow` command handler parses args → builds `observation.Observation{Kind:KindFlow, Feature:FeatureFlowBytes, Flow:FlowKey{Src,Dst,DstPort,...}, Value:bytes, At:now}`.
2. `observation.Global().Publish(obs)` fans out to subscribers (observation.go:180).
3. `trafficfeature` subscriber `ingest`s it (feature.go:102); on the 1s tick `snapshot` emits the source's `FeatureEntry` (feature.go:169-189).
4. `anomaly/detect` `onTick(Snapshot())` scores, warms, confirms, emits `anomaly-detect` `AnomalyDetected`.
5. `anomaly/shape` `onDetected` (non-shadow) arms a firewall term; `show anomaly shape` reports `armed-count>0`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` driver ↔ DUT | `dispatch-command` RPC → in-process command handler | [ ] |
| fakeflow ↔ core feed | `observation.Global().Publish` | [ ] |
| feed ↔ trafficfeature | subscribe → `ingest` → `Snapshot` | [ ] |
| detect ↔ shape | `anomaly-detect` typed events | [ ] |

### Integration Points
- `internal/core/observation` `Publish`/`Global` - the publish seam
- `internal/test/plugins/all/all.go` - blank-import registration
- `test/plugin/*.ci` - functional-test location + driver pattern

### Architectural Verification
- [ ] No bypassed layers (injects at the feed, not directly into the detector)
- [ ] No unintended coupling (fakeflow imports only `observation` + sdk; no anomaly import)
- [ ] No duplicated functionality (copies the fakeredist test-plugin pattern; no new mechanism)
- [ ] Registration over hardcoding (fakeflow registers like any plugin; no core edit)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | A `zetest` in-process plugin shares `observation.Global()` with the DUT's `trafficfeature`, so `Publish` reaches `ingest` | plugins run in-process; `trafficfeature` uses `observation.Global()` (register.go:22) | the injector publishes into a different feed and nothing scores | wiring smoke test: inject one flow, assert `show traffic-feature` leaves `degraded` and lists the source | unvalidated |
| A-2 | `request fakeflow inject` reaches the in-process handler via `dispatch-command`, as `request fakeredist emit` does | fakeredist exposes the same command shape (register.go:69) | the driver can't inject | wiring test that dispatches `request fakeflow inject` and checks a non-error status | unvalidated |
| A-3 | Lowering the detector knobs + a same-`/24` normal cohort + one sustained outlier fires a confirmed incident within a raised `.ci` budget | knobs are settable (config.go:62-111); scoring rule (learned 1048) | the `.ci` never sees `incidents>0` | tune in the functional phase until deterministic; poll not sleep | unvalidated |
| A-4 | Shape in non-shadow mode arms against the DUT firewall backend in the CI environment | `registerTables`/`applyAll` are real (responder.go:41-42) | `armed-count` stays 0 despite an incident | run the `.ci`; if the backend is unavailable, fall back to asserting the shadow "would act" log line (responder.go:83) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Wall-clock flakiness: warmup (3 ticks) + confirm ticks at 1s each → ~6-15s to fire | `.ci` times out or fires intermittently | lower `confirm-duration`/`baseline-window`; raise the `.ci` timeout well above 15s; poll `show anomaly detect` in a bounded loop, no fixed sleep |
| R-2 | Firewall backend unavailable under CI → responder cannot arm | incident fires but `armed-count==0` | primary assertion is `incidents>0`; arming asserted via non-shadow, with the shadow log-line fallback (A-4) |
| R-3 | `fakeflow` leaks into a production build | it appears in `bin/ze --plugins` without `zetest` | zetest build tag on the import path only; a test asserts it is absent from a production build |
| R-4 | The injector is mistaken for a real traffic source and skews other tests | other anomaly `.ci`s change behavior | fakeflow only publishes when its `inject` command is called; idle otherwise |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `observation.Global().Publish` (synthetic flows) | → | `trafficfeature.ingest` → `Snapshot` → `detect.onTick` → `anomalyevent.Detected.Emit` | `TestChainFactsToResponse` asserts `len(d.recentIncidents())>0` from real feature data |
| detector incident → responder | → | `shape.SubscribeForTest` wires `resp.onDetected` on the bus → arm | `TestChainFactsToResponse` asserts the outlier `10.0.0.9/32` is in `armedList()` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `TestChainFactsToResponse` builds a real `trafficfeature.Service` + real detector + real responder wired via `shape.SubscribeForTest` on an in-memory `testBus` | the chain is composed from production types, in one process, no plugin/RPC/DUT |
| AC-2 | Warmup: same-/24 cohort (incl. the future outlier) gets balanced in+out traffic for 7 ticks | no source arms during warmup (asserted): balanced ratio + fan-out 1 + common port + expired new-peer = below threshold |
| AC-3 | Attack: cohort stays balanced; outlier `10.0.0.9` goes pure-outbound, fan-out 12, distinct rare ports | `d.recentIncidents()>0` — a real incident from real trafficfeature feature data (not a crafted snapshot) |
| AC-4 | Same run | `shape` arms the outlier specifically — `10.0.0.9/32 ∈ armedList()`; the normal cohort is NOT armed (discrimination proven) |
| AC-5 | Existing tests | the detect + shape unit suites and the three anomaly `.ci`s still pass unchanged; no production behavior changes |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | (CI) injects synthetic flows that look like exfil + scanning | fakeflow → Publish → trafficfeature → detect → emit | `anomaly-e2e.ci` (`incidents>0`) |
| 2 | (CI) confirms the responder acts on that incident | detect event → shape → firewall arm | `anomaly-e2e.ci` (`armed-count>0`) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFakeflowInjectPublishes` | `internal/test/plugins/fakeflow/fakeflow_test.go` | parsing `inject` args → exactly one correct `KindFlow` `Observation` on `Global()` (subscribe a probe, assert the record) | |
| `TestFakeflowInjectCount` | `internal/test/plugins/fakeflow/fakeflow_test.go` | optional `count` publishes N records; bad args return a non-zero/error status without publishing | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `dstport` | 0-65535 | 65535 | N/A (uint) | reject >65535 |
| `bytes` | >0 (feature.go:107 drops ≤0) | 1 | reject 0 / negative | N/A |
| `count` | 1-N (bounded) | small cap | reject 0 | reject absurd (unbounded alloc guard) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `anomaly-e2e` | `test/plugin/anomaly-e2e.ci` | inject cohort+outlier → real incident → armed responder, all through the daemon | |

### Interop Tests
N/A with justification: the harness exercises an INTERNAL process feed (`observation.Feed`), not a wire protocol between daemons. No BGP/IPsec/L2TP wire behavior changes. (Per `ai/rules/interop-and-goal-validation.md`, interop is for protocol behavior with a peer daemon; there is none here.)

### Future (if deferring any tests)
- None. Both the injector unit tests and the end-to-end `.ci` ship in this spec.

## Files to Modify
- `docs/functional-tests.md` - note that the anomaly facts→judgment→response chain is proven by the Go integration test `TestChainFactsToResponse` (not a `.ci`, because `observation.Feed` is process-local — see the Mistake Log); this is the only non-test codebase file touched
- `internal/plugins/anomaly/shape/testsupport.go` - NEW: `SubscribeForTest` composition helper (production package, test-only surface, `*ForTest` idiom)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | N/A - no config leaves (Go integration test only) | - |
| YANG validation constraints | N/A | - |
| CLI commands/flags | N/A - no CLI command; the test composes production types directly | - |
| CLI grammar (action before identifier) | N/A - no CLI | - |
| Editor autocomplete | N/A | - |
| Functional test | The `.ci` path is impossible here (process-local feed); the equivalent is the in-process Go integration test | `internal/plugins/anomaly/detect/chain_integration_test.go` |
| Pipe completeness | N/A | - |
| Env var registration | N/A | - |
| Doctor check for runtime dependencies | N/A - no production runtime dependency | - |
| Prometheus counters/metrics | N/A | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No - test-only | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - note the anomaly e2e proof is a Go integration test + why (process-local feed) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No (documents an existing constraint: `observation.Feed` is process-local) | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin/event/command/inventory changed? | No | - |
| 16 | Changed source referenced by doc source anchors? | Verify: grep `docs/` for anchors on the changed files | - |
| 17 | Existing docs show examples for this area? | No | - |

## Files to Create
- `internal/plugins/anomaly/detect/chain_integration_test.go` - `TestChainFactsToResponse`: wires real trafficfeature + detector + responder, drives synthetic observations, asserts incident + outlier armed (with a `testBus` mirroring `ddosevent`'s)
- `internal/plugins/anomaly/shape/testsupport.go` - `SubscribeForTest(bus)`: constructs an armed responder, mocks the firewall backend, subscribes it to the bus, returns an armed-prefix accessor + stop

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Create/Modify; confirm fakeredist shape |
| 3. Wiring phase | Wiring Test table - register fakeflow + failing unit test |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-13 | Critical/Deliverables/Security review |
| 14. Present summary | Executive Summary + learned summary |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — create `internal/test/plugins/fakeflow/{register.go,fakeflow.go}` (copy fakeredist), register `request fakeflow inject`, blank-import in `all.go`. Write `TestFakeflowInjectPublishes` — it fails because `dispatchCommand` is a stub.
   - Verify: `go vet` clean; `fakeflow` appears in a `zetest` build's plugin list, absent from production.
2. **Phase: Injector logic** — implement `dispatchCommand`: parse `inject <src> <dst> <dstport> <bytes> [<count>]`, validate (bytes>0, port≤65535, count bounded), publish `count` `KindFlow` observations via `observation.Global().Publish`. Make the unit tests pass.
   - Verify: `TestFakeflowInjectPublishes` + `TestFakeflowInjectCount` green.
3. **Phase: End-to-end `.ci`** — write `test/plugin/anomaly-e2e.ci`: config `anomaly { detect { enabled true; confirm-duration 2; baseline-window 10; deviation-threshold 2 } shape { mode armed } }` + the driver. Driver: after post-startup, each second for ~N seconds inject a same-`/24` normal cohort (several sources, low fan-out, common port, balanced bytes) plus one outlier (high fan-out across many dests, rare/varied ports, high out/in ratio); poll `show anomaly detect` until `incidents>0`, then `show anomaly shape` until `armed-count>0`; assert both; shutdown. Set the `.ci` timeout well above 15s.
   - Verify: `.ci` passes deterministically across repeated runs.
4. **Full verification** → `make ze-lint && make ze-unit-test && make ze-functional-test`
5. **Complete spec** → learned summary + two commits.

### Critical Review Checklist (/implement stage 7)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 each have file:line evidence |
| Feature completeness | the `.ci` drives a REAL incident (scoring ran), not a stubbed value |
| Production invisibility | `fakeflow` absent from a non-`zetest` build (`bin/ze --plugins` grep) |
| Determinism | `.ci` polls, never fixed-sleeps; passes on repeat runs (R-1) |
| Correctness | injected `Observation` fields match what `ingest` reads (KindFlow/FeatureFlowBytes/Flow/Value) |
| Registration over hardcoding | fakeflow registers via `registry.Register`; no core edit beyond the `all.go` blank import |
| Rule: no-test-deletion | the three existing anomaly `.ci`s remain and still pass |

### Deliverables Checklist (/implement stage 11)
| Deliverable | Verification method |
|-------------|---------------------|
| `fakeflow` plugin files | `ls internal/test/plugins/fakeflow/` |
| zetest-only load | build with/without `zetest`, grep plugin list for `fakeflow` |
| unit tests green | `go test ./internal/test/plugins/fakeflow/` |
| `anomaly-e2e.ci` fires an incident | run the `.ci`; assert exit 0 and the `incidents>0` log |

### Security Review Checklist (/implement stage 12)
| Check | What to look for |
|-------|-----------------|
| Input validation | `inject` args bounded: bytes>0, port range, `count` capped (no unbounded publish loop / alloc) |
| Production exposure | fakeflow cannot be reached in a production build (build tag), so it adds no production attack surface |
| Resource exhaustion | `count` cap prevents a flood of `Publish` calls degrading the DUT |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `.ci` never sees `incidents>0` | re-tune knobs / cohort recipe (Phase 3); if scoring is wrong → re-read learned 1048 |
| `armed-count==0` with an incident | check `Mode != shadow`; apply A-4 firewall-backend fallback |
| fakeflow visible in production | fix the build tag / import path (R-3) |
| 3 fix attempts fail | STOP, report, ask |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A config-`plugin{internal}` test plugin runs in-process with the engine's `trafficfeature`, so its `observation.Global().Publish` reaches it (A-1) | The functional-test DUT isolates the config-loaded plugin from the engine; `observation.Feed` is process-local (observation.go:86), so publishes never reach `trafficfeature` | `.ci` failed (trafficfeature degraded); a `selfcheck` command showed the injector's own publish is received in its process (self_received=1) while `trafficfeature` stayed degraded | Abandoned the `fakeflow` plugin + `.ci`; pivoted to an in-process Go integration test that co-locates the whole chain (user-chosen) |
| `startInternal` (goroutine, process.go:456) means config-loaded internal plugins share the engine's process | The in-process Go test proved the publish→ingest mechanism works when genuinely co-located; the DUT's process topology differs from that read | `TestInjectReachesTrafficfeatureService` passed in one process while the DUT `.ci` did not | Confirmed the mechanism is sound; the barrier is process topology, not the code |

## Design Insights
- The chain is proven by composing the REAL production types (trafficfeature + detector + responder)
  in one process and driving them with real observations — the only place `observation.Feed`
  (a process-local bus) can be exercised end to end. A `.ci` cannot, because a config-loaded plugin
  is isolated from the engine's feed.
- Discrimination (not just "something fired") is the real proof: a BALANCED normal cohort must stay
  unarmed while only the pure-outbound / high-fan-out / rare-port outlier arms. Pure-outbound normals
  read as exfil (`+Inf` out/in ratio) and falsely arm — the cohort needs in+out traffic.

## Known Limitations
- The test fabricates flow-byte observations only (`KindFlow`/`FeatureFlowBytes`); it cannot exercise
  sub-second beaconing (the pipeline is 1s — see the umbrella's blocked child 9).
- It is an in-process composition, not "through the daemon": it proves the layers integrate, not the
  plugin-lifecycle/RPC wiring (that is covered by the existing wiring `.ci`s).
- Discovered (out of scope, flagged to the user): `deviation-threshold` — and any `decimal-2` YANG
  leaf with a `range` — is currently unsettable ("range validation not supported for type string",
  schema.go:890) against the in-flight anomaly YANG.

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Prove facts→judgment→response end to end in-process | Go integration test | `TestChainFactsToResponse` PASS (10s): real trafficfeature+detector+responder; asserts `recentIncidents()>0` AND outlier `10.0.0.9/32` armed |
| Discrimination (only the anomaly flags) | same test | warmup asserts the balanced cohort does NOT arm; attack arms only the outlier (log: `armed source entity=10.0.0.9/32`) |

## Review Gate
### Run 1 (self-review against the ze-review checklist)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `shape.SubscribeForTest` is exported test-only surface, shipped in production builds | `shape/testsupport.go` | accepted `*ForTest` idiom (precedent: `ResetForTest`, `SetUsersForTest`); dead in prod, no cost |
| 2 | NOTE | `SubscribeForTest` mutates `registerTables`/`applyAll` package vars | `shape/testsupport.go` | contained to the detect test binary's copy of shape; shape's own test binary unaffected |
| 3 | NOTE | 10s integration test | `detect/chain_integration_test.go` | `-short`-skippable; CI gate (plain `go test`) still runs it |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (self-review: 0 BLOCKER, 0 ISSUE, 3 NOTEs above)
- [ ] All NOTEs recorded (3 above)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/anomaly/detect/chain_integration_test.go` | yes | created + `go test` builds it |
| `internal/plugins/anomaly/shape/testsupport.go` | yes | created + package compiles (`go vet` exit 0) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | chain composed from real types in one process | `chain_integration_test.go`: `trafficfeature.NewService`, `newDetector`, `shape.SubscribeForTest`, `chainTestBus` |
| AC-2 | cohort does not arm during warmup | test asserts `len(armedList()) != 0` -> fail after 7 warmup ticks; PASS |
| AC-3 | real incident fires | `TestChainFactsToResponse` PASS 10.3s; asserts `len(d.recentIncidents())>0` |
| AC-4 | outlier armed, cohort not | log `armed source entity=10.0.0.9/32` (only); `slices.Contains(armedList(), "10.0.0.9/32")` |
| AC-5 | existing suites still pass | `go test ./…/detect ./…/shape` exit 0; anomaly `.ci`s unchanged |

### Wiring Verified (end-to-end)
| Entry Point | Test | Verified |
|-------------|------|----------|
| `observation.Global().Publish` → trafficfeature → detect emit → shape arm | `TestChainFactsToResponse` | yes (PASS, discrimination) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 (in-process shared feed via config plugin) | **broken** | `.ci` degraded + `selfcheck`; pivoted to Go test (Mistake Log) |
| A-2 (command reaches handler) | moot | plugin approach abandoned |
| A-3 (deterministic fire) | confirmed | warm-then-spike fires deterministically (10s, repeatable) |
| A-4 (arms without kernel) | confirmed | `armedCount` is decision-based; firewall mocked in `SubscribeForTest` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/functional-tests.md` in-process integration test note | source-anchored to `TestChainFactsToResponse` | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A - internal feed, justified above)

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Write learned summary to `plan/learned/NNN-anomaly-4-interop-harness.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-anomaly-4-interop-harness.md`
