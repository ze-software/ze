# Spec: Anomaly Child 7 -- Per-ASN Entities & AS-Origin Cohort Rarity

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-02 |

Umbrella: `plan/spec-anomaly-0-umbrella.md` (Child Spec Roadmap row `as-entities-cohorts`,
AC-5, R-3). This child widens exactly one axis of the shipped anomaly spine: it adds AS-origin
grouping to the detector. It consumes the existing `fe.SrcAS` fact and the
source/destination/port entity axes. It ships no facts-layer or flowexport code.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-anomaly-0-umbrella.md` - shared framing, R-3 degrade rule, A-1 chaining of 5/6/7
4. `docs/architecture/anomaly/anomaly-1-detect.md` - the JUDGMENT-layer invariants this child must not regress
5. Source: `internal/plugins/anomaly/detect/detector.go`, `internal/plugins/anomaly/detect/score.go`,
   `internal/core/anomalyevent/event.go`, `internal/component/trafficfeature/feature.go`

## Task

Add **per-ASN entities** and **AS-origin cohort rarity** to the behavioral anomaly detector.

Two distinct uses of the existing origin-AS fact `fe.SrcAS`:

1. **AS-origin cohort rarity.** The existing per-source-prefix entities are grouped into cohorts
   by their origin-AS instead of by source `/24` (v4) / `/48` (v6). A source IP is scored as rare
   relative to other sources announced by the SAME AS. This is a change to the cohort GROUPING KEY
   only; the leave-one-out rarity math (`score.go`) and the incident subject (a source prefix) are
   unchanged.
2. **Per-ASN entities.** A NEW entity dimension whose baseline aggregates all traffic from one
   origin-AS into a single tracked entity, scored with the same self-deviation + cohort machinery.
   The incident subject is an ASN, not a prefix.

Both MUST degrade gracefully to the current prefix behavior when `fe.SrcAS` is unset (flowexport
disabled, or the source has no AS attribution), per umbrella R-3. Freeze-learn, warmup, and the
pure `score.go` rule are preserved unchanged (same invariants as the source-prefix path). This is
primarily a **detector-layer** change plus a small, backward-compatible `anomalyevent` contract
addition to represent the ASN subject; the fact field and the entity-axis generalization come from
children 6 and 5 respectively and are consumed, not modified, here.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/traffic/traffic-analysis-layers.md` - traffic analysis split into facts, judgment and response
- [ ] `plan/spec-anomaly-0-umbrella.md` - shared framing and the verified constraints for AS work
  → Constraint: no `detect -> flowexport/enrich` import (fails `./le tier check`, umbrella A-3);
    origin-AS must ride the facts surface, stamped by the producer (child 6). This child reads
    `fe.SrcAS` off the already-imported `trafficfeature.FeatureEntry` -- zero new imports.
  → Constraint: R-3 -- AS enrichment is OPTIONAL; the detector must keep scoring (prefix cohorts,
    no per-ASN entities) when `fe.SrcAS` is unset. AS availability must never gate detection.
- [ ] `docs/architecture/anomaly/anomaly-1-detect.md` - the JUDGMENT-layer contract
  → Constraint: scoring is PURE in `score.go`; freeze-learn is `scoreEntity` returning pending
    `baselineUpdate`s that `onTick` folds only when NOT anomalous or still warming; warmup gates
    self-deviation for `warmupTicks`. Any new entity type must preserve all three.
- [ ] `ai/rules/plugins.md` - remove the plugin, its surface vanishes
  → Constraint: any new config leaf, show field, or metric registers inside `anomaly/detect`; no
    AS-specific spelling leaks into a core or shared package.
- [ ] `ai/rules/config.md`, `ai/rules/config.md` - if a config leaf is added
  → Constraint: a new tracked entity DIMENSION (memory cost) is opt-in via a YANG leaf under
    `anomaly/detect`, kebab-case, with maximal native validation.

### RFC Summaries
- [ ] `rfc/short/rfc7607.md` (AS 0 is reserved / must-not appear) - grounds the unset sentinel
  → Constraint: `Observation.SrcAS` already defines zero as unknown. This child
    uses that sentinel for prefix fallback.

**Key insights:**
- AS-origin cohort rarity changes the cohort key; rarity maths and the `+Inf`
  exclusion remain unchanged. Source scoring currently uses `cohortsOf` and
  `cohortPrefix`, so the AS-specific grouping still needs implementation.
- `EntityKind` already distinguishes source, destination and port incidents.
  This child adds an ASN kind and subject field to that contract rather than
  adding another discriminator.
- The responder's `actsOn` already accepts only `EntityKindSource` for detected,
  ongoing and cleared events. Preserve that guard and the report-only AS scope.

## Current Behavior (MANDATORY)

**Source files read (BEFORE writing this spec):**
- [ ] `internal/plugins/anomaly/detect/detector.go` - `onTick` scores
  `snap.Sources`, `snap.Dests` and `snap.Ports` through separate bounded state
  maps. `scoreSources` calls `cohortsOf`, selects `cohortPrefix(fe.Addr)` and
  reports a source-kind incident. AS-keyed grouping is the remaining change.
  `stepEntity` and `scoreEntity` own warmup and freeze-learn; reuse them.
- [ ] `internal/plugins/anomaly/detect/score.go` - the PURE pinned rule.
  - `cohortStats.rarity` (score.go) is the leave-one-out rarity: `n = count-1`; returns 0 below
    `minSize` OTHER members; removes `value`'s own contribution before computing mean/variance.
  - `cohortStats.add` (score.go) accumulates sum / sumSq / count.
  → Constraint: `score.go` MUST NOT be edited. The AS cohort is the SAME `cohortStats` accumulated
    under a different grouping key; it calls the identical `rarity`.
- [ ] `internal/core/anomalyevent/event.go` - the event contract.
  - `AnomalyDetected.EntityKind` already distinguishes source, destination and
    port. `Entity` remains a prefix; there is no ASN subject field.
  → Constraint: source incidents may change `Cohort` to `"AS64500"` without
    changing their kind. Per-ASN incidents need a new kind value and ASN field.
- [ ] `internal/component/trafficfeature/feature.go` - `ingest` retains a
  nonzero observation `SrcAS` on its source state, and `finalizeAddrs` publishes
  that state as `FeatureEntry.SrcAS`. An unattributed later flow does not erase
  a previously attributed source.
  → Constraint: this child consumes that existing fact and does not modify it.
- [ ] `internal/core/observation/observation.go` - `Observation.SrcAS` is a
  `uint32` with zero meaning unknown. `flowexport.exportFlows` stamps it before
  publishing observations.
- [ ] `internal/plugins/anomaly/shape/responder.go` - `actsOn` returns true
  only for `EntityKindSource`; `onDetected`, `onOngoing` and `onCleared` call it
  before changing armed state. This safeguard already exists and must remain.
- [ ] `internal/plugins/anomaly/detect/detector_test.go`, `score_test.go`,
  `chain_integration_test.go` - the test patterns this child mirrors (crafted snapshots via
  `snapOf`/`normalEntry`/`spikeEntry`; `TestCohortRarity`; `TestFreezeLearnDuringSustainedAnomaly`;
  the end-to-end `TestChainFactsToResponse`).

**Behavior to preserve (do NOT regress):**
- The fact/judgment/response split and the anomaly-vs-DDoS domain separation.
- Freeze-learn + warmup; the pure `score.go` rule; leave-one-out cohort rarity with `+Inf` exclusion.
- The source-prefix incident path: when `fe.SrcAS` is unset the detector behaves EXACTLY as today.
- The shape responder acts only on source-kind incidents; non-source events
  cannot install, extend or withdraw a source-prefix rule.
- Existing metrics (`ze_anomaly_incidents_total`, `ze_anomaly_active`, `ze_anomaly_tracked_entities`)
  and the doctor check `anomaly-detect-feature-source`.

**Behavior to change:** Cohort grouping gains an AS-keyed path with prefix
fallback, plus an opt-in per-ASN dimension and ASN identity on the existing
event-kind contract.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config `anomaly { detect { enabled true } }` (plus the opt-in per-ASN leaf) starts the tick loop
  (`register.go` calls `d.onTick(svc.Snapshot())`). Operator intent and the fact `fe.SrcAS`
  (already produced by flowexport) enter through `trafficfeature.Snapshot`.

### Transformation Path
1. `onTick` reads `snap.Sources`; each `FeatureEntry` already carries `SrcAS`.
2. **Cohort key selection (new):** for each entry, the cohort key is `fe.SrcAS` when set, else
   `cohortPrefix(fe.Addr)` (degrade). Cohorts accumulate via the unchanged `cohortStats.add`, with
   the same `+Inf` ratio exclusion.
3. **Per-ASN entity (new, opt-in):** entries with a set `SrcAS` also fold into a per-ASN
   `entityState` keyed by ASN, scored by the same `scoreEntity` + freeze-learn as source entities.
4. **Scoring (unchanged):** `scoreEntity` -> `zScore` / `cohortStats.rarity` / `combineScore` in
   `score.go`. No arithmetic change.
5. **Event construction:** source-prefix incidents keep `Entity = prefix`, only `Cohort` becomes
   `"AS<n>"`; these emit on the bus as today. Per-ASN incidents carry the entity-kind discriminator
   and the ASN, are recorded in the ring + `show anomaly detect`, and are NOT emitted on the
   actionable `Detected` bus.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| trafficfeature -> detect | Existing `Snapshot()` and `fe.SrcAS` field | [ ] AS grouping evidence owed |
| detect -> anomalyevent | ASN kind and subject added to the existing `EntityKind` contract | [ ] |
| detect -> shape | source-prefix incidents only on the bus; AS incidents report-only | [ ] |

### Integration Points
- `trafficfeature.FeatureEntry.SrcAS` - the existing fact read.
- The detector's existing source, destination and port state maps and scoring helpers.
- `anomalyevent.AnomalyDetected` - reuse `EntityKind`, add the ASN subject.
- `internal/plugins/anomaly/detect/score.go` - reused unchanged.

### Architectural Verification
- [ ] No bypassed layers (facts read via `Snapshot`, rarity via `score.go`, no re-measurement)
- [ ] No unintended coupling (no `detect -> flowexport` import; `fe.SrcAS` is a field access)
- [ ] No duplicated functionality (AS cohort reuses `cohortStats`; per-ASN reuses `entityState`)
- [ ] Zero-copy preserved where applicable (reads `Snapshot` fields; no copy of the fact surface)
- [ ] Registration over hardcoding (any new leaf/metric registers within `anomaly/detect`; no
  per-feature switch added to a core/shared package)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `trafficfeature.FeatureEntry` carries `SrcAS` | `ingest` and `finalizeAddrs` in `feature.go` | AS grouping cannot read its input | Read the producer and snapshot construction | confirmed by source, 2026-09-19; AS grouping still unimplemented |
| A-2 | The detector already scores separate source, destination and port axes | `detector.onTick` and its three state maps | ASN integration must be redesigned | Read `onTick`, `scoreSources` and the map bounds | confirmed by source, 2026-09-19; no child-closure verdict |
| A-3 | `SrcAS == 0` means unknown | `observation.Observation` contract and `exportFlows` | The fallback branch tests the wrong condition | Read observation and producer | confirmed by source, 2026-09-19 |
| A-4 | AS-origin cohort rarity is a cohort-KEY swap only; `score.go` is untouched | `buildCohorts` keys by `cohortPrefix` (detector.go); `rarity` (score.go) is key-agnostic | design churn if rarity needs AS-specific math | unit test: AS cohort produces identical rarity to a prefix cohort with the same members | confirmed against code |
| A-5 | An ASN cannot be represented by `AnomalyDetected.Entity` (a `netip.Prefix`) | `event.go` | per-ASN incidents cannot be surfaced without a contract change | read `event.go` | confirmed against code |
| A-6 | The event contract has an entity-kind discriminator | `AnomalyDetected.EntityKind` in `event.go` | Adding another discriminator would duplicate identity | Read the existing kind values | confirmed by source, 2026-09-19; add only the ASN kind and subject |
| A-7 | The shape responder refuses every non-source kind before changing armed state | `actsOn`, `onDetected`, `onOngoing`, `onCleared` | An ASN event could affect source rules | Preserve the source-only guard and prove report-only AS behaviour | confirmed by source, 2026-09-19 |
| A-8 | Reading `fe.SrcAS` adds no import to `detect` | `detector.go` already imports `trafficfeature` | umbrella "zero new imports" claim wrong | `goimports` diff after implementation | confirmed against code |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Per-ASN entities add a second keyed map and inflate tracked-entity memory (umbrella R-1) | a new `ze_anomaly_tracked_as_entities` gauge climbs; eviction churn | bound the AS map by its own cap mirroring `maxTrackedEntities` (detector.go); make the dimension opt-in; reuse idle eviction |
| R-2 | An AS subject is mislabelled as source and reaches response | An invalid prefix appears in armed state | Keep the existing report-only AS path and source-only responder guard; prove an ASN incident never arms a prefix |
| R-3 | Hard dependency on flowexport: AS-keyed grouping stops the detector scoring when AS is absent (umbrella R-3) | detector emits nothing once flowexport is disabled | per-entry degrade to `cohortPrefix`; whole-snapshot all-unset path is byte-for-byte the current behavior; covered by a dedicated test |
| R-4 | AS cohort has too few members (a single-homed source AS) to score rarity | AS cohorts of size < `MinCohortSize`; rarity always 0 | `cohortStats.rarity` already returns 0 below `minSize` (score.go); self-deviation still scores; this is correct, not a bug |
| R-5 | A single `+Inf` exfil host inflates the AS cohort ratio baseline and masks peers | AS cohort ratio mean spikes | replicate the `+Inf` exclusion (detector.go) in the AS-keyed builder; test mirrors `TestBuildCohortsExcludesInfiniteRatio` |
| R-6 | Freeze-learn not wired for the per-ASN path, so a sustained AS anomaly self-clears | AS incident flaps; AS baseline drifts up | per-ASN scoring routes through the SAME `scoreEntity` + `onTick` fold; test mirrors `TestFreezeLearnDuringSustainedAnomaly` |
| R-7 | The consumed fact or identity contract changes before implementation | `SrcAS` or `EntityKind` differs from the source evidence above | Recheck those concrete APIs at implementation; do not wait for already-present prerequisites |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `anomaly { detect { enabled true } }` + snapshot with `fe.SrcAS` set on peers | -> | `onTick` -> AS-keyed `buildCohorts` -> `scoreEntity` -> source-prefix incident with `Cohort="AS<n>"` | `TestASCohortDrivesIncident` |
| opt-in per-ASN leaf true + snapshot with a deviating AS | -> | `onTick` -> per-ASN `entityState` -> confirm -> ring entry with `EntityKind="as"` | `TestPerASNEntityScored` |
| snapshot with all `fe.SrcAS == 0` | -> | `onTick` -> `cohortPrefix` fallback, no per-ASN entity | `TestDegradeToPrefixWhenASUnset` |
| In-process production facts/detector/show chain, synthetic AS-tagged flows | -> | observation feed -> facts -> judgment -> show handler returns an AS-cohort incident | `TestASChainFactsToShow` in `chain_integration_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A snapshot of same-AS sources, one a strong behavioral outlier, all with `fe.SrcAS` set | The outlier's cohort rarity is scored against its AS peers (leave-one-out), producing a source-prefix incident whose `Cohort` is `"AS<n>"` |
| AC-2 | The same members grouped by AS vs grouped by prefix | AS-origin cohort rarity yields the IDENTICAL value the prefix cohort would for the same member set (proves it reuses `cohortStats.rarity`, `score.go` unchanged) |
| AC-3 | A snapshot whose entries all have `fe.SrcAS == 0` (flowexport/AS absent) | The detector scores exactly as today: prefix cohorts, no per-ASN entities, identical incidents. Per-entry: an entry with `SrcAS == 0` in an otherwise AS-tagged snapshot degrades to its prefix cohort |
| AC-4 | Per-ASN entity tracking enabled; a sustained anomaly on one ASN beyond the baseline window | A per-ASN incident confirms and stays active; the per-ASN baseline is NOT poisoned (freeze-learn preserved); a never-seen ASN is not flagged during warmup |
| AC-5 | A per-ASN incident is produced | It carries an unambiguous entity-kind discriminator identifying the ASN subject, appears in the recent-incident ring and `show anomaly detect`, and is NOT delivered to the shape responder as an actionable prefix event (responder `r.armed` never keyed on an invalid prefix) |
| AC-6 | Any AS path exercised | `internal/plugins/anomaly/detect/score.go` is byte-for-byte unchanged (the pinned pure rule) |
| AC-7 | The `detect` package built after the change | No new import is added to `detect`; `fe.SrcAS` is a field access on the already-imported `trafficfeature` type |
| AC-8 | A `+Inf` (pure-sender / exfil) source inside an AS cohort | It is excluded from the AS cohort's ratio baseline (mirrors detector.go) and still scores via self-deviation |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables detect with flowexport AS enrichment present; one host in a busy AS starts scanning | facts (`fe.SrcAS`) -> AS-keyed cohort -> rarity -> incident `Cohort="AS<n>"` -> `show anomaly detect` | `TestASChainFactsToShow` + `TestASCohortDrivesIncident`; `.ci` covers operator config/show reachability |
| 2 | enables per-ASN entity tracking; an entire AS shifts behavior | facts -> per-ASN `entityState` -> confirm -> ring/show with `EntityKind="as"` | `TestPerASNEntityScored` (Go chain-level) |
| 3 | disables flowexport (no AS) | facts with `SrcAS==0` -> prefix cohorts, no per-ASN entities -> unchanged incidents | `TestDegradeToPrefixWhenASUnset` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestASCohortRarity` | `internal/plugins/anomaly/detect/as_cohort_test.go` | AS-keyed cohort leave-one-out: outlier scores high, in-distribution ~0, cohort < `MinCohortSize` scores 0 (mirrors `TestCohortRarity`) | |
| `TestASCohortMatchesPrefixCohort` | same | AC-2: identical member set grouped by AS vs prefix yields identical rarity | |
| `TestASCohortExcludesInfiniteRatio` | same | AC-8: a `+Inf` host is excluded from the AS cohort ratio baseline, counted for other features (mirrors `TestBuildCohortsExcludesInfiniteRatio`) | |
| `TestDegradeToPrefixWhenASUnset` | same | AC-3: whole-snapshot `SrcAS==0` == current behavior; per-entry unset degrades to prefix cohort | |
| `TestPerASNEntityScored` | same | AC-1/AC-4: per-ASN entity aggregates, confirms an incident, `EntityKind="as"` | |
| `TestFreezeLearnASEntity` | same | AC-4: sustained AS anomaly does not poison the per-ASN baseline (mirrors `TestFreezeLearnDuringSustainedAnomaly`) | |
| `TestASIncidentReportOnly` | same | AC-5: an AS-subject incident is in the ring but never delivered to a `Detected` bus subscriber | |
| `TestScoreGoUnchanged` (guard) | reuse `score_test.go` | AC-6: existing `score.go` tests still pass unchanged | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `fe.SrcAS` (uint32) | 0 = unset sentinel; 1..4294967295 = attributed | 4294967295 | N/A (unsigned) | N/A |
| per-ASN map size | 0..cap (mirrors `maxTrackedEntities` 10000) | cap | N/A | entity beyond cap dropped (nil, like `stateFor` detector.go) |
| `MinCohortSize` (existing) | 2..1024 (config.go) | reused unchanged | reused | reused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `anomaly-as-cohort` | `test/plugin/anomaly-as-cohort.ci` | operator enables the AS options and reads the registered show surface | planned; configuration and show reachability only |
| `TestASChainFactsToShow` | `internal/plugins/anomaly/detect/chain_integration_test.go` | real in-process observation feed -> trafficfeature -> detector -> registered show handler; AS-tagged flows produce the expected AS-cohort incident, unset AS retains prefix behaviour, and ASN subjects never arm source rules | planned; extends the supported `TestChainFactsToResponse` composition |

### Interop Tests
N/A -- this child adds no wire-protocol behavior. It reads a fact and emits an in-process event.
(Umbrella interop is owned by child 8, upstream FlowSpec.)

### Harness boundary
The `fakeflow` plugin was abandoned because its process-local observation feed
did not reach the engine. `docs/architecture/anomaly/anomaly-4-interop-harness.md`
documents the supported in-process composition. The AS chain must exercise
real facts and show output there; crafted snapshots alone do not discharge that
evidence. The `.ci` remains responsible for operator config/show reachability.

## Files to Modify
- `internal/plugins/anomaly/detect/detector.go` - AS-keyed cohort building (key selection: `fe.SrcAS`
  when set else `cohortPrefix`, `+Inf` exclusion replicated); optional per-ASN `entityState` map +
  loop routed through the existing scoring/freeze-learn helpers; incident reporting sets `Cohort="AS<n>"` for the AS-cohort
  path and builds report-only AS-subject incidents; a per-ASN cap constant; the tracked-AS gauge.
  → update the existing design record at `docs/architecture/anomaly/anomaly-1-detect.md`.
- `internal/core/anomalyevent/event.go` - add an ASN kind value and an
  `EntityAS uint32` subject on the existing contract. Reuse `EntityKind`.
- `internal/plugins/anomaly/detect/config.go` - opt-in `track-as-entities` bool (parse/default/validate).
- `internal/plugins/anomaly/detect/yang/ze-anomaly-detect-conf.yang` - the `track-as-entities` leaf
  (type boolean, default false) under `container detect`.
- `internal/plugins/anomaly/detect/show.go` - render `EntityKind`/`EntityAS` so `show anomaly detect`
  distinguishes an AS-subject incident from a prefix incident.
- `internal/plugins/anomaly/detect/chain_integration_test.go` - AS-tagged production facts through detector and show output, with response isolation

**Consumed but NOT modified here (child boundary):**
- `internal/component/trafficfeature/feature.go` - already publishes `SrcAS`; this child only reads it.
- Existing destination/port entity axes - preserve their behaviour while adding ASN state.
- `internal/plugins/anomaly/detect/score.go` - the pinned pure rule; unchanged (AC-6).
- `internal/plugins/anomaly/shape/*` - unchanged; AS incidents are kept off its bus (R-2).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] Yes | `internal/plugins/anomaly/detect/yang/ze-anomaly-detect-conf.yang` -- add `leaf track-as-entities` under `container detect` |
| YANG validation constraints | [ ] Yes | boolean leaf with explicit `default false`; no range needed (bool) |
| YANG custom validators | [ ] No | native boolean is sufficient; no dynamic completion needed |
| CLI commands/flags | [ ] No | reuses `show anomaly detect`; no new verb |
| CLI grammar (action before identifier) | [ ] N/A | no new command |
| Editor autocomplete | [ ] Yes | automatic for the boolean YANG leaf |
| Functional test for new RPC/API | [ ] Yes | `test/plugin/anomaly-as-cohort.ci` for config/show reachability; `TestASChainFactsToShow` for populated incident output |
| Pipe completeness | [ ] N/A | `show anomaly detect` output path is unchanged; only a field is added |
| Env var registration | [ ] No | config is YANG-modeled, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] No | AS enrichment is an OPTIONAL soft dependency that degrades (R-3); no new file/socket/port/binary. The existing `anomaly-detect-feature-source` check already covers the trafficfeature dependency. (Optional future: an info-level check reporting "AS unset -> degraded to prefix cohorts".) |
| Prometheus counters/metrics | [ ] Yes | add `ze_anomaly_tracked_as_entities` gauge, registered alongside the existing gauges in `bindMetrics` (detector.go) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` -- note AS-origin cohorts + per-ASN entities under behavioral anomaly detection |
| 2 | Config syntax changed? | [ ] Yes | `docs/guide/configuration.md` -- `anomaly { detect { track-as-entities } }` |
| 3 | CLI command added/changed? | [ ] No | `show anomaly detect` output gains an entity-kind field; command unchanged |
| 4 | API/RPC added/changed? | [ ] No | wire method `ze-show:anomaly` unchanged; payload gains optional fields |
| 5 | Plugin added/changed? | [ ] Yes | `docs/guide/plugins.md` -- anomaly-detect gains AS grouping |
| 6 | Has a user guide page? | [ ] Yes | `docs/guide/anomaly.md` - add the AS section |
| 7 | Wire format changed? | [ ] No | no wire change |
| 8 | Plugin SDK/protocol changed? | [ ] No | value-type event only |
| 9 | RFC behavior implemented? | [ ] No | RFC 7607 informs the sentinel choice only; no protocol enforcement |
| 10 | Test infrastructure changed? | [ ] No | extends the supported in-process chain in `chain_integration_test.go`; no fakeflow plugin |
| 11 | Affects daemon comparison? | [ ] No | |
| 12 | Internal architecture changed? | [ ] Yes | subsystem doc for the anomaly detector (the `// Design:` doc) -- note AS grouping + report-only AS subject |
| 13 | Route metadata keys added/changed? | [ ] No | |
| 14 | Prometheus counters added/changed? | [ ] Yes | telemetry doc -- add `ze_anomaly_tracked_as_entities` |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] No | no new event type; the `AnomalyDetected` struct gains optional fields on the existing type |
| 16 | Any changed source file referenced by existing doc source anchors? | [ ] Yes | grep `docs/` for anchors on `detector.go` / `event.go` and update stale claims |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] Yes | verify anomaly config examples against the new leaf |

## Files to Create
- `internal/plugins/anomaly/detect/as_cohort_test.go` - the unit tests above.
- `test/plugin/anomaly-as-cohort.ci` - operator config/show reachability; populated AS evidence uses the existing Go integration harness.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella + learned 1048 |
| 2. Audit | Files to Modify/Create, TDD Plan; recheck the existing SrcAS and entity-kind contracts in A-1/A-2/A-3/A-6 |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `./le verify lint run && ./le test-unit  && ./le functional` |
| 7-13 | Critical / Deliverables / Security review, re-verify |
| 14. Present summary | Executive Summary + learned summary |

### Implementation Phases
Each phase ends with a Self-Critical Review. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- consume the existing `SrcAS` and
   `EntityKind` contracts. Add the
   `track-as-entities` config leaf + YANG + a failing wiring test that asserts an AS-cohort incident
   from a crafted snapshot. Verify it fails because the AS path is a stub.
   - Tests: `TestASCohortDrivesIncident` (fails)
   - Files: `config.go`, `yang/...`, `detector.go` (stub key selection)
2. **Phase: AS-origin cohort rarity** -- add cohort-key selection (`fe.SrcAS` when set else
   `cohortPrefix`), replicate the `+Inf` exclusion, set `Cohort="AS<n>"`. `score.go` untouched.
   - Tests: `TestASCohortRarity`, `TestASCohortMatchesPrefixCohort`, `TestASCohortExcludesInfiniteRatio`
   - Files: `detector.go`
3. **Phase: Degrade path** -- ensure per-entry and whole-snapshot `SrcAS==0` reproduce current behavior.
   - Tests: `TestDegradeToPrefixWhenASUnset`
   - Files: `detector.go`
4. **Phase: Per-ASN entities (opt-in)** -- add the per-ASN `entityState` map (own cap + idle
   eviction + gauge) routed through `scoreEntity`/freeze-learn; add the ASN kind
   and subject to the existing contract; keep AS incidents report-only (ring + show, no bus emit); render in
   `show.go`.
   - Tests: `TestPerASNEntityScored`, `TestFreezeLearnASEntity`, `TestASIncidentReportOnly`
   - Files: `detector.go`, `event.go`, `show.go`
5. **Functional and chain evidence** -- `test/plugin/anomaly-as-cohort.ci` proves operator config/show reachability; `TestASChainFactsToShow` drives AS-tagged observations through real facts, detector and show handling.
6. **Full verification** -- `./le verify current mode full`.
7. **Complete spec** -- fill audit tables; learned summary `plan/learned/NNN-anomaly-7-as-entities-cohorts.md`;
   two commits (code+spec+learned, then `git rm` spec).

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | AS-cohort AND per-ASN entity paths both work; degrade path proven |
| Correctness | AS cohort rarity == prefix cohort rarity for the same members (AC-2); `+Inf` excluded (AC-8) |
| Naming | `Cohort="AS<n>"` format; `EntityKind` values; YANG kebab-case `track-as-entities` |
| Data flow | AS keying in `detector.go` only; `score.go` unchanged; `trafficfeature` untouched |
| Registration over hardcoding | new gauge + leaf register within the plugin; no AS spelling in a core package |
| Doctor checks | none added; degrade is graceful (justified N/A) |
| YANG validation | boolean leaf has an explicit default |
| Prometheus counters | `ze_anomaly_tracked_as_entities` defined + registered |
| Rule: preserve invariants | freeze-learn, warmup, pure `score.go`, shape gets no invalid Entity |
| Rule: no plugin->plugin import | `detect` gains no import; `fe.SrcAS` is a field access |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| AS cohort path | `go test ./internal/plugins/anomaly/detect/ -run TestASCohort` |
| Degrade path | `go test -run TestDegradeToPrefixWhenASUnset` |
| Per-ASN entity + freeze-learn | `go test -run 'TestPerASNEntityScored|TestFreezeLearnASEntity'` |
| Report-only AS incident | `go test -run TestASIncidentReportOnly` |
| `score.go` unchanged | `git diff --stat internal/plugins/anomaly/detect/score.go` shows no change |
| No new import | `goimports -l internal/plugins/anomaly/detect/detector.go` clean; import block diff empty |
| Metric registered | grep `ze_anomaly_tracked_as_entities` in `detector.go` + telemetry doc |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `SrcAS` is an untrusted fact from flow data; `0` sentinel handled; no unbounded map growth (cap + eviction) |
| Resource exhaustion | per-ASN map bounded by its cap (R-1); AS cohort map bounded by distinct AS count in a Top-N snapshot |
| Error leakage | AS-subject incidents in show output do not expose internal keys beyond the ASN |
| Responder safety | AS incidents never reach the shape responder as an actionable prefix (R-2) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `SrcAS` field missing / different shape | STOP: the consumed contract changed; revalidate A-1/A-3 |
| Entity axis not generalized as assumed | re-validate A-2; fall back to a bespoke per-ASN map |
| AS cohort rarity != prefix cohort rarity | inspect cohort-key selection and `cohortsOf`; preserve the pinned scoring rule |
| shape arms an invalid prefix | AS incident leaked to the bus; enforce report-only |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- The work splits cleanly into a zero-contract-change part (AS-origin cohort rarity: a cohort-KEY
  swap that reuses the pure rule and keeps a prefix incident subject) and a small-contract-change
  part (per-ASN entities: a new subject type an existing `netip.Prefix` field cannot express).
- The umbrella frames child 7 as "detector re-key, zero new imports." That is accurate for the
  facts read and the cohort swap, but per-ASN ENTITIES also touch `internal/core/anomalyevent`
  (a discriminator) and interact with the shape responder (which must not receive a non-prefix
  subject). The surface is detector + a thin event-contract addition, not detector-only.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| AS-origin cohort rarity sets only `Cohort="AS<n>"`, keeping the incident subject a source prefix | Make every AS-cohort incident an AS subject | The subject of a cohort-rarity finding is the rare MEMBER (a source), not the cohort; keeps these incidents fully actionable by the shape responder with zero contract change |
| Per-ASN incidents remain report-only (ring + show), without `Detected` emission | Emit non-source events through the existing guarded bus | Preserves this child's original response scope; the source-only responder guard already exists and remains defence in depth |
| Reuse `EntityKind`, add its ASN value and an `EntityAS` subject | A second discriminator or synthetic prefix | The existing event contract distinguishes dimensions; an ASN has no valid prefix representation |
| Degrade is per-entry (`SrcAS==0` -> `cohortPrefix`), not all-or-nothing | Disable AS grouping entirely when any source lacks AS | Partial AS attribution is normal; per-entry degrade keeps AS grouping for attributed sources while unattributed ones stay on prefix cohorts (R-3) |
| Per-ASN entity tracking is opt-in via `track-as-entities` (default false) | Always-on | It is a new tracked DIMENSION with memory cost (R-1, umbrella R-1); opt-in matches config-surface conventions and lets operators keep the memory ceiling flat |

## Known Limitations
- Per-ASN incidents are observational only; the shape responder cannot enforce against a whole AS
  (no single prefix), so no upstream/local action follows an AS-subject incident. AS-origin cohort
  rarity incidents (source-prefix subject) remain fully actionable.
- AS grouping is only as good as flowexport's AS attribution; when AS is absent the detector
  silently runs on prefix cohorts (this is the intended R-3 degrade, not a regression).
- The exact `fe.SrcAS` representation and the generalized entity axis are owned by children 6 and 5;
  this spec is implementable once both land and the A-1/A-2/A-3/A-6 rows are re-validated.

## Implementation Summary
### What Was Implemented
- [filled at implementation]
### Bugs Found/Fixed
- [filled at implementation]
### Documentation Updates
- [filled at implementation]
### Deviations from Plan
- [filled at implementation]

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| per-ASN entities scored | unit test | `TestPerASNEntityScored` |
| AS-origin cohort rarity mirrors leave-one-out | unit test | `TestASCohortRarity` + `TestASCohortMatchesPrefixCohort` |
| degrades to prefix cohorts when AS absent | unit test | `TestDegradeToPrefixWhenASUnset` |
| freeze-learn preserved | unit test | `TestFreezeLearnASEntity` |
| end-to-end AS incident visible | functional test | `test/plugin/anomaly-as-cohort.ci` (gated on child 4/6) |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Fixes applied
- [per BLOCKER/ISSUE]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); children-5/6 deps
  re-validated at implementation start; broken ones in Mistake Log

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC 7607 sentinel comment added at the degrade branch
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (AS cohort reuses `cohortStats`; per-ASN reuses `entityState`)
- [ ] No speculative features (per-ASN entities opt-in; no AS action layer)
- [ ] Single responsibility (grouping key + subject representation only)
- [ ] Explicit > implicit (degrade branch is explicit on the sentinel)
- [ ] Minimal coupling (no new import; `score.go` untouched; shape untouched)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests (N/A -- no wire behavior; justified)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks documented
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-anomaly-7-as-entities-cohorts.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-anomaly-7-as-entities-cohorts.md`
