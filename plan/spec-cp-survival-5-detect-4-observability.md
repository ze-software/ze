# Spec: DDoS Observability — Incident Store, Status CLI, Doctor, Metrics

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-cp-survival-5-detect-1-detector, spec-cp-survival-5-detect-2-local-responder, spec-cp-survival-5-detect-3-flowspec-responder, spec-cp-survival-5-detect-0-umbrella |
| Phase | - |
| Updated | 2026-06-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-cp-survival-5-detect-1-detector.md` (event contract this consumes/extends)
3. `plan/spec-cp-survival-5-detect-0-umbrella.md` (observability requirements per plugin-self-containment)
4. `ai/rules/doctor-checks.md`, `ai/rules/pipe-completeness.md`, `internal/core/diagnostic/codes.go`

## Task

Build the **observability** surface so the DDoS auto-mitigation initiative is operable and self-contained:
a cross-cutting **incident store** that turns the detector and responder events into a queryable history,
the `show ddos status` / `show ddos incidents` CLI, a `doctor` check that the mitigation paths are
actually usable, and the cross-cutting Prometheus gauges. Per-plugin metrics are defined in their owning
plugins (children 1-3); this child owns the **aggregate** view and the incident record.

To build incident records that include the responder outcome (what action was taken, how much was dropped
or leaked), this spec **extends the `ddosevent` contract** with `MitigationApplied` / `MitigationRemoved`
events that the responders (children 2-3) emit. The store subscribes to detector + mitigation events and
needs no DirectBridge coupling to the responders.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  → Constraint: this plugin owns the incident store, the `show ddos` verbs, and the doctor check; removing it removes observability only (detection + mitigation keep working).
- [ ] `ai/rules/doctor-checks.md` + `internal/core/diagnostic/codes.go` - doctor check pattern
  → Constraint: register a diagnostic code; the check verifies the FlowSpec BGP session (flowspec mode) and firewall backend (local mode) are usable; unit + functional test required.
- [ ] `ai/rules/pipe-completeness.md` - command output
  → Constraint: `show ddos status`/`incidents` route through `ApplyPipes`/`ProcessPipes` and support all pipe operators.
- [ ] `ai/patterns/cli-command.md` + `ai/rules/cli-grammar.md` - the show verbs
  → Constraint: `show ddos status` / `show ddos incidents` (object-root operational command); derive output, do not hardcode lists (`ai/rules/derive-not-hardcode.md`).
- [ ] `ai/rules/plugin-design.md` (EventBus Typed Payloads) - the store's inputs
  → Constraint: subscribe to detector + mitigation events via `events.Register[T]`; the store is a pure consumer.

### RFC Summaries
- N/A — no wire protocol.

**Key insights:**
- The incident record is the operator's forensic artifact (the `ftagent` analogue of a pcap evidence
  record, but lightweight): target, family, vector, start/end, peak pps/bps, top sources, responder(s)
  used, action, dropped/leaked bytes.
- Metrics stay in their owning plugins (self-containment); this child adds only cross-cutting gauges
  (active incidents, active mitigations) and documents the full metric set.
- The doctor check encodes the umbrella's "the BGP session must exist" requirement: flowspec mode is
  useless if the configured session is down, and the operator should be told before an attack.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/ddosevent/event.go` - the event contract (child 1): `AttackDetected`/`AttackOngoing`/`AttackCleared`.
  → Constraint: EXTEND with `MitigationApplied{target, responder, action, …}` / `MitigationRemoved{target, responder, droppedBytes, leakedBytes, reason}`; children 2-3 become the producers.
- [ ] `internal/core/diagnostic/codes.go` - diagnostic code registry for doctor checks.
  → Constraint: add a `ddos` diagnostic code; the doctor check references it.
- [ ] `internal/core/show/` (registration) - show enricher / command registration pattern.
  → Constraint: register `show ddos status`/`incidents` via the standard show/command path; pipe-complete.
- [ ] `internal/component/telemetry/exporter/` - `metrics.Registry` (GaugeVec/CounterVec).
  → Constraint: register cross-cutting gauges here; per-plugin metrics are registered by their plugins.

**Behavior to preserve:**
- Detector and responders work unchanged; the store and CLI are pure consumers. Existing `show`/doctor/
  metrics surfaces are unaffected; the new ones are additive.

**Behavior to change:** The `ddosevent` contract gains two mitigation event types (additive); children 2-3
emit them.

## Data Flow (MANDATORY)

### Entry Point
- `ddosevent` events (detector + the two new mitigation events) on the EventBus.
- `show ddos status` / `show ddos incidents` CLI; `ze doctor` invocation.

### Transformation Path
1. The store subscribes to `AttackDetected`/`AttackOngoing`/`AttackCleared` and `MitigationApplied`/`MitigationRemoved`.
2. On `AttackDetected`, open an incident record (target, family, vector, top sources, start, peak).
3. On `MitigationApplied`/`MitigationRemoved`, attach responder action + dropped/leaked byte counts.
4. On `AttackCleared`, finalize the record (end, duration); keep it in a bounded ring.
5. `show ddos status` renders current open incidents + active mitigations + thresholds/baselines; `show ddos incidents` renders the ring; both pipe-complete.
6. The doctor check inspects responder config vs live state (BGP session up, firewall backend present).
7. Cross-cutting gauges (active incidents, active mitigations) updated on each transition.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| EventBus ↔ store | subscribe detector + mitigation events | [ ] |
| store ↔ CLI | `show ddos status`/`incidents` read the store | [ ] |
| doctor ↔ responder state/config | diagnostic check reads config + BGP session state | [ ] |
| store ↔ telemetry | cross-cutting gauges via `metrics.Registry` | [ ] |

### Integration Points
- `internal/core/ddosevent` - consume detector events; define + consume mitigation events
- `internal/core/show/` + `internal/core/diagnostic/codes.go` - CLI + doctor
- `internal/component/telemetry/exporter` - cross-cutting gauges

### Architectural Verification
- [ ] No bypassed layers (store consumes events; CLI reads the store; doctor reads config/state)
- [ ] No unintended coupling (store imports the event leaf only, not detector/responder plugins)
- [ ] No duplicated functionality (per-plugin metrics stay in their plugins)
- [ ] Zero-copy preserved (consumer only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Children 2-3 emit `MitigationApplied`/`MitigationRemoved` (added to the contract here) | this spec defines them; responders are the producers | incident records lack action/counters; fall back to DirectBridge polling | implement emission in 2-3; store unit test | unvalidated |
| A-2 | The detector's `AttackDetected` carries the top sources / vector for the record | child 1 contract | incident record thinner | validate against `ddosevent/event.go` | unvalidated |
| A-3 | The doctor framework can query a peer's BGP session state at check time | `ai/rules/doctor-checks.md`; reactor exposes session state | doctor cannot assert session up | trace session-state accessor; unit test | unvalidated |
| A-4 | `show` registration supports pipe-complete operational commands | `internal/core/show/`, pipe-completeness rule | output not pipeable | mirror an existing pipe-complete show; functional test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Unbounded incident store memory under a sustained attack storm | memory growth | bounded ring (last N incidents); evict oldest; cap open incidents |
| R-2 | Lost mitigation events (responder restart) leave an incident record without an outcome | record stuck "open" | reconcile on `AttackCleared`; timeout-finalize stale open records |
| R-3 | Doctor check false-alarms when flowspec mode is configured but intentionally idle | spurious unhealthy | check only asserts session-up when a flowspec responder is enabled with that selector |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `AttackDetected` event | → | store opens an incident record | `ddos-incident-record.ci` |
| `show ddos incidents` after an incident | → | store renders the record (pipe-complete) | `show-ddos-incidents.ci` |
| `ze doctor` with flowspec mode + down session | → | doctor check reports unhealthy | `ddos-doctor.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `AttackDetected` then `AttackCleared` | an incident record is opened and finalized (start/end, peak pps/bps, family, target, top sources) |
| AC-2 | `MitigationApplied`/`MitigationRemoved` for the incident | the record captures responder, action, dropped and leaked byte counts |
| AC-3 | `show ddos status` during an attack | shows open incidents + active mitigations + current threshold/baseline |
| AC-4 | `show ddos incidents` | shows the bounded incident history, newest first, pipe-complete |
| AC-5 | flowspec mode enabled but its BGP session is down | `ze doctor` reports the ddos check unhealthy with the session named |
| AC-6 | all dependencies healthy | the ddos doctor check passes |
| AC-7 | cross-cutting gauges | `ze_ddos_incidents_active` and `ze_ddos_mitigations_active` are exported and track transitions |
| AC-8 | incident ring exceeds its cap | the oldest record is evicted; memory bounded |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show ddos status` during an attack | store (from events) → status command | `show-ddos-status.ci` |
| 2 | reviews `show ddos incidents` after | store ring → incidents command | `show-ddos-incidents.ci` |
| 3 | runs `ze doctor` with a misconfigured flowspec session | doctor check → unhealthy | `ddos-doctor.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIncidentLifecycle` | `internal/plugins/ddosobserve/store_test.go` | open on detect, attach mitigation, finalize on clear (AC-1, AC-2) | |
| `TestIncidentRingEviction` | `internal/plugins/ddosobserve/store_test.go` | bounded ring evicts oldest (AC-8, R-1) | |
| `TestStaleOpenIncidentFinalized` | `internal/plugins/ddosobserve/store_test.go` | timeout-finalize when clear is lost (R-2) | |
| `TestDoctorFlowspecSessionDown` | `internal/plugins/ddosobserve/doctor_test.go` | unhealthy when session down + flowspec enabled (AC-5, R-3) | |
| `TestCrossCuttingGauges` | `internal/plugins/ddosobserve/metrics_test.go` | active incidents/mitigations gauges track transitions (AC-7) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| incident-ring-size | 1-100000 | 100000 | 0 | 100001 |
| stale-incident-timeout (s) | 1-86400 | 86400 | 0 | 86401 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-incident-record` | `test/plugin/ddos-incident-record.ci` | event lifecycle produces a record | |
| `show-ddos-status` | `test/ui/show-ddos-status.ci` | status command renders open incidents/mitigations | |
| `show-ddos-incidents` | `test/ui/show-ddos-incidents.ci` | incidents command renders history, pipe-complete | |
| `ddos-doctor` | `test/plugin/ddos-doctor.ci` | doctor reports unhealthy on a down flowspec session | |

### Interop Tests
N/A — observability touches no wire protocol.

### Future (deferred tests)
- Web UI panel for incidents (the umbrella mentions HTMX surfaces; deferred to a follow-on).

## Files to Modify
- `internal/core/ddosevent/event.go` - add `MitigationApplied` / `MitigationRemoved` event types
- `internal/component/plugin/all/all.go` - add the observe plugin (`make generate`)
- `internal/core/diagnostic/codes.go` - add the ddos diagnostic code
- `docs/features.md` - add the "DDoS observability" row
- `docs/guide/command-reference.md` - `show ddos status` / `show ddos incidents`
- `docs/guide/status.md` - the ddos doctor check
- `docs/plugin-development/metrics.md` - the full ddos metric set (per-plugin + cross-cutting)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (config) | Yes (minimal) | `internal/plugins/ddosobserve/yang/ze-ddos-observe-conf.yang` — ring size, stale timeout; native constraints |
| CLI commands | Yes | `show ddos status` / `show ddos incidents` — object-root per `docs/architecture/cli/command-namespacing.md` |
| Pipe completeness | Yes | route through `ApplyPipes` per `ai/rules/pipe-completeness.md` |
| Doctor check | Yes | `internal/plugins/ddosobserve/doctor.go` + `internal/core/diagnostic/codes.go` + unit + functional test |
| Prometheus counters | Yes | cross-cutting `ze_ddos_incidents_active`, `ze_ddos_mitigations_active`; document per-plugin set |
| Env var registration | No | YANG config only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin/event type/command changed? | Yes | `docs/plugin-overview.md`, `docs/guide/status.md` |

## Files to Create
- `internal/plugins/ddosobserve/register.go` - registration, event subscription, command + doctor binding
- `internal/plugins/ddosobserve/store.go` - bounded incident store (ring), open/attach/finalize
- `internal/plugins/ddosobserve/cmd_status.go` - `show ddos status`
- `internal/plugins/ddosobserve/cmd_incidents.go` - `show ddos incidents`
- `internal/plugins/ddosobserve/doctor.go` - doctor check
- `internal/plugins/ddosobserve/metrics.go` - cross-cutting gauges
- `internal/plugins/ddosobserve/config.go` - config parse + validation
- `internal/plugins/ddosobserve/yang/ze-ddos-observe-conf.yang` - config schema
- `internal/plugins/ddosobserve/*_test.go` - unit tests above
- `test/plugin/ddos-incident-record.ci`, `test/plugin/ddos-doctor.ci`, `test/ui/show-ddos-status.ci`, `test/ui/show-ddos-incidents.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + detector + responders |
| 2. Audit | Files to Create/Modify, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — extend `ddosevent` with mitigation events; observe-plugin skeleton subscribing to events + a failing `ddos-incident-record.ci`.
2. **Phase: Incident store** — open/attach/finalize, bounded ring, stale-finalize.
   - Tests: `TestIncidentLifecycle`, `TestIncidentRingEviction`, `TestStaleOpenIncidentFinalized`; `ddos-incident-record.ci`.
3. **Phase: Status/incidents CLI** — pipe-complete show commands.
   - Tests: `show-ddos-status.ci`, `show-ddos-incidents.ci`.
4. **Phase: Doctor + metrics** — ddos doctor check + cross-cutting gauges.
   - Tests: `TestDoctorFlowspecSessionDown`, `TestCrossCuttingGauges`; `ddos-doctor.ci`.
5. **Phase: Producer wiring** — children 2-3 emit `MitigationApplied`/`MitigationRemoved` (cross-spec).
6. **Full verification** → `make ze-verify-changed` + `make generate`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | each AC-N has implementation with file:line |
| Correctness | incident lifecycle correct; ring bounded; doctor only alarms when relevant |
| Naming | metric names `ze_ddos_*`; CLI object-root `show ddos …` |
| Data flow | store imports the event leaf only; per-plugin metrics not duplicated |
| Pipe completeness | show commands support all pipe operators |
| Doctor checks | diagnostic code registered; check has unit + functional test |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| observe plugin registered | `make ze-inventory` lists `ddosobserve` |
| show verbs registered | `make ze-command-list` shows `show ddos status`/`incidents` |
| doctor check registered | `ze doctor` lists the ddos check |
| functional tests pass | `ze-test ui test/ui/show-ddos-*.ci` + `ze-test bgp plugin test/plugin/ddos-*.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | incident ring bounded; open-incident cap |
| Info exposure | `show ddos` reveals source IPs — same trust level as existing show commands; no secrets |
| Input validation | YANG ranges for ring size/timeout enforced |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Incident records are in-memory (bounded ring); persistence across restart is a follow-on.
- Web UI panel for incidents is deferred (the umbrella notes HTMX surfaces exist; not in this child).

## Design Insights
- Keeping per-plugin metrics in their plugins and only aggregating here preserves the removal test: each
  responder is independently removable, and observability degrades gracefully to what remains.

## Implementation Audit

| AC | Evidence |
|----|----------|
| AC-1 | `Open` creates incident from AttackDetected, `Finalize` closes it (`store.go`); `TestIncidentLifecycle` passes |
| AC-8 | ring eviction when full (`store.go`); `TestIncidentRingEviction` passes |

### Files created
- `internal/plugins/ddosobserve/store.go` -- bounded incident ring
- `internal/plugins/ddosobserve/store_test.go` -- 4 tests
- `internal/plugins/ddosobserve/config.go` -- ring size + stale timeout
- `internal/plugins/ddosobserve/register.go` -- plugin registration
- `internal/plugins/ddosobserve/yang/` -- YANG schema, embed, register

### Deferred
- `show ddos status`/`show ddos incidents` CLI commands
- Doctor check (FlowSpec session up)
- Cross-cutting Prometheus gauges
- `MitigationApplied`/`MitigationRemoved` event types in ddosevent

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
- [ ] Feature code integrated (`internal/plugins/ddosobserve`, `internal/core/ddosevent` extension)
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
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-5-detect-4-observability.md`
