# Spec: Anomaly Child 7 -- Per-ASN Entities & AS-Origin Cohort Rarity

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-anomaly-5-entity-matrix (child 5), spec-anomaly-6-as-enrichment (child 6) |
| Phase | B |
| Updated | 2026-07-02 |

Umbrella: `plan/spec-anomaly-0-umbrella.md` (Child Spec Roadmap row `as-entities-cohorts`,
AC-5, R-3). This child widens exactly one axis of the shipped anomaly spine: it adds AS-origin
grouping to the detector. It consumes the `fe.SrcAS` fact that child 6 stamps and (where child 5
has generalized it) the generalized entity axis. It ships no facts-layer or flowexport code.

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

Two distinct uses of the origin-AS fact `fe.SrcAS` (added to `trafficfeature.FeatureEntry` by
child 6, planned):

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
  → Constraint: AS number 0 is reserved and never a legitimate origin, so `SrcAS == 0` is the
    natural "unknown / not attributed" sentinel to branch the degrade path on. FLAG: the exact
    sentinel is child 6's to define; if child 6 uses a separate `HasAS bool` rather than `0`,
    this child branches on that instead.

**Key insights:**
- The detector already generalizes cleanly. AS-origin cohort rarity is a cohort-KEY swap; the
  rarity math and the +Inf exclusion are untouched. Per-ASN entities reuse `entityState` and
  `scoreEntity` behind a second keyed map (or child 5's generalized axis).
- The only unavoidable NON-detector change is representing an ASN as an incident subject: the
  event contract's `Entity` is a `netip.Prefix` (`event.go`) and cannot hold an ASN. A small
  additive discriminator is required (or reuse of child 5's, if child 5 added one for its port
  dimension, which is likewise not a prefix).
- The shape responder keys `r.armed` on `e.Entity` (`responder.go,90,102`) with no
  `IsValid` guard, so an AS-subject incident must NOT reach it as an actionable prefix event.

## Current Behavior (MANDATORY)

**Source files read (BEFORE writing this spec):**
- [ ] `internal/plugins/anomaly/detect/detector.go` - the detector. NOTE: there is no
  `detect/feature.go` or `detect/event.go`; the feature read and entity keying live here.
  - `onTick` (detector.go) reads `trafficfeature.Snapshot`, iterates `snap.Sources`
    (`[]trafficfeature.FeatureEntry`), keys per-entity baselines on `netip.Addr` in
    `states map[netip.Addr]*entityState` (detector.go,133).
  - `buildCohorts` (detector.go) groups entities into `map[netip.Prefix]*cohortAgg` keyed by
    `cohortPrefix(fe.Addr)` (detector.go); it EXCLUDES a `+Inf` out/in ratio from the ratio
    accumulator (detector.go) so an exfil host cannot dominate the cohort baseline.
  - `cohortPrefix` (detector.go) derives the `/24` (v4) or `/48` (v6) bucket from
    `CohortPrefixLenV4/V6`.
  - `scoreEntity` (detector.go) computes `max(self-deviation, cohort.rarity)` per continuous
    feature, gated by warmup, returning pending `baselineUpdate`s (no mutation).
  - Freeze-learn (detector.go): `onTick` folds updates only when `!above || samples < warmupTicks`.
  - `stateFor` (detector.go) bounds distinct baselines by `maxTrackedEntities = 10000`
    (detector.go), returning nil at the cap.
  - `activate` (detector.go) builds `anomalyevent.AnomalyDetected{Entity: prefix, Cohort:
    cohortPrefix(addr).String(), ...}`, appends to the ring (detector.go), and EMITS on the bus
    (detector.go).
  → Constraint: score/rarity math is called from here but DEFINED in `score.go`; this child changes
    keying and event construction only, never the rarity arithmetic.
- [ ] `internal/plugins/anomaly/detect/score.go` - the PURE pinned rule.
  - `cohortStats.rarity` (score.go) is the leave-one-out rarity: `n = count-1`; returns 0 below
    `minSize` OTHER members; removes `value`'s own contribution before computing mean/variance.
  - `cohortStats.add` (score.go) accumulates sum / sumSq / count.
  → Constraint: `score.go` MUST NOT be edited. The AS cohort is the SAME `cohortStats` accumulated
    under a different grouping key; it calls the identical `rarity`.
- [ ] `internal/core/anomalyevent/event.go` - the event contract.
  - `AnomalyDetected.Entity` is a `netip.Prefix` (event.go); `Cohort` is a free-form `string`
    (event.go); there is NO entity-kind discriminator.
  → Constraint: AS-origin cohort rarity needs NO contract change (it only sets `Cohort = "AS64500"`
    on a still-prefix Entity). Per-ASN ENTITIES need an additive discriminator because an ASN is
    not a `netip.Prefix`.
- [ ] `internal/component/trafficfeature/feature.go` - the facts surface.
  - `FeatureEntry` (feature.go) has `Addr, FanOut, OutInRatio, PortEntropy, NewPeer,
    RarePort, Beaconing`. There is NO `SrcAS` field today.
  - `maxTrackedKey = 10000` (feature.go) bounds the facts-layer source map.
  → Constraint: `fe.SrcAS` does not exist yet. Reading it is a child-6 (planned) dependency. This
    child does NOT modify `trafficfeature`.
- [ ] `internal/core/observation/observation.go` - the feed carrying the facts.
  - `FlowKey` (observation.go) is `Src, Dst, SrcPort, DstPort, Proto`; `Observation`
    (observation.go) carries no AS. Confirms AS attribution is absent upstream too (child 6).
- [ ] `internal/plugins/anomaly/shape/responder.go` - the responder that consumes incidents.
  - `onDetected` (responder.go) reads `e.Entity` (responder.go) and keys `r.armed`
    (a `map[netip.Prefix]`) on it (responder.go,102) with NO `Entity.IsValid()` guard.
  → Constraint: an AS-subject incident with an invalid/zero `Entity` prefix would poison
    `r.armed`. AS-subject incidents must be report-only (ring + show), not emitted on the actionable
    `anomalyevent.Detected` bus, so the responder is never handed an invalid prefix.
- [ ] `internal/plugins/anomaly/detect/detector_test.go`, `score_test.go`,
  `chain_integration_test.go` - the test patterns this child mirrors (crafted snapshots via
  `snapOf`/`normalEntry`/`spikeEntry`; `TestCohortRarity`; `TestFreezeLearnDuringSustainedAnomaly`;
  the end-to-end `TestChainFactsToResponse`).

**Behavior to preserve (do NOT regress):**
- The fact/judgment/response split and the anomaly-vs-DDoS domain separation.
- Freeze-learn + warmup; the pure `score.go` rule; leave-one-out cohort rarity with `+Inf` exclusion.
- The source-prefix incident path: when `fe.SrcAS` is unset the detector behaves EXACTLY as today.
- The shape responder invariant: every `anomalyevent.Detected` payload it receives has a valid,
  actionable `Entity` prefix.
- Existing metrics (`ze_anomaly_incidents_total`, `ze_anomaly_active`, `ze_anomaly_tracked_entities`)
  and the doctor check `anomaly-detect-feature-source`.

**Behavior to change:** Only additive. Cohort grouping gains an AS-keyed path (degrading to prefix);
a new opt-in per-ASN entity dimension; an additive event-contract discriminator for the ASN subject.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config `anomaly { detect { enabled true } }` (plus the opt-in per-ASN leaf) starts the tick loop
  (`register.go` calls `d.onTick(svc.Snapshot())`). Operator intent and the fact `fe.SrcAS`
  (child 6, planned) enter through `trafficfeature.Snapshot`.

### Transformation Path
1. `onTick` reads `snap.Sources`; each `FeatureEntry` now (planned) carries `SrcAS`.
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
| trafficfeature -> detect | `Snapshot()` read incl. `fe.SrcAS` field access (no new import) | [ ] depends on child 6 |
| detect -> anomalyevent | additive `EntityKind`/`EntityAS` on `AnomalyDetected` | [ ] |
| detect -> shape | source-prefix incidents only on the bus; AS incidents report-only | [ ] |

### Integration Points
- `trafficfeature.FeatureEntry.SrcAS` - the fact read (child 6, planned).
- child 5's generalized entity axis - where the per-ASN entity dimension plugs in (planned).
- `anomalyevent.AnomalyDetected` - the incident contract (additive change here).
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
| A-1 | `trafficfeature.FeatureEntry` gains a `SrcAS` field | umbrella Data-Flow step 1 + AC-4; child 6 (planned) | this child cannot read AS; whole feature blocked | child 6 lands; grep `SrcAS` on `FeatureEntry`; `feature.go` has none today | unvalidated -- depends on child 6 (planned) |
| A-2 | The entity axis is generalized (dest/port) so an ASN dimension plugs in cleanly | umbrella child-5 row; child 5 (planned) | per-ASN entity adds a bespoke `map[uint32]*entityState` instead of riding the generalized axis | child 5 lands; inspect the generalized entity key/registry | unvalidated -- depends on child 5 (planned) |
| A-3 | `SrcAS == 0` is the "unset / not attributed" sentinel | RFC 7607 (AS 0 reserved); child 6 API choice (planned) | the degrade branch tests the wrong condition | child 6's chosen representation (`0` vs a `HasAS bool`) | unvalidated -- depends on child 6 (planned) |
| A-4 | AS-origin cohort rarity is a cohort-KEY swap only; `score.go` is untouched | `buildCohorts` keys by `cohortPrefix` (detector.go); `rarity` (score.go) is key-agnostic | design churn if rarity needs AS-specific math | unit test: AS cohort produces identical rarity to a prefix cohort with the same members | confirmed against code |
| A-5 | An ASN cannot be represented by `AnomalyDetected.Entity` (a `netip.Prefix`) | `event.go` | per-ASN incidents cannot be surfaced without a contract change | read `event.go` | confirmed against code |
| A-6 | Child 5 may already add an entity-kind discriminator (its port dimension is also non-prefix) | umbrella child-5 row ("port has no natural cohort"); child 5 (planned) | this child adds `EntityKind`/`EntityAS` itself instead of reusing child 5's | inspect `AnomalyDetected` after child 5 lands | unvalidated -- depends on child 5 (planned) |
| A-7 | The shape responder has no `Entity.IsValid()` guard | `responder.go,90,102` (no guard) | AS incidents on the bus would poison `r.armed` | read `responder.go`; keep AS incidents off the bus | confirmed against code |
| A-8 | Reading `fe.SrcAS` adds no import to `detect` | `detector.go` already imports `trafficfeature` | umbrella "zero new imports" claim wrong | `goimports` diff after implementation | confirmed against code |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Per-ASN entities add a second keyed map and inflate tracked-entity memory (umbrella R-1) | a new `ze_anomaly_tracked_as_entities` gauge climbs; eviction churn | bound the AS map by its own cap mirroring `maxTrackedEntities` (detector.go); make the dimension opt-in; reuse idle eviction |
| R-2 | AS-subject incident leaks onto the actionable bus and poisons `r.armed` (invalid prefix) | shape arms an all-zero/invalid prefix; `armedCount` drifts | keep AS incidents report-only (ring + show), never `Detected.Emit`; unit-test that no AS payload reaches a bus subscriber |
| R-3 | Hard dependency on flowexport: AS-keyed grouping stops the detector scoring when AS is absent (umbrella R-3) | detector emits nothing once flowexport is disabled | per-entry degrade to `cohortPrefix`; whole-snapshot all-unset path is byte-for-byte the current behavior; covered by a dedicated test |
| R-4 | AS cohort has too few members (a single-homed source AS) to score rarity | AS cohorts of size < `MinCohortSize`; rarity always 0 | `cohortStats.rarity` already returns 0 below `minSize` (score.go); self-deviation still scores; this is correct, not a bug |
| R-5 | A single `+Inf` exfil host inflates the AS cohort ratio baseline and masks peers | AS cohort ratio mean spikes | replicate the `+Inf` exclusion (detector.go) in the AS-keyed builder; test mirrors `TestBuildCohortsExcludesInfiniteRatio` |
| R-6 | Freeze-learn not wired for the per-ASN path, so a sustained AS anomaly self-clears | AS incident flaps; AS baseline drifts up | per-ASN scoring routes through the SAME `scoreEntity` + `onTick` fold; test mirrors `TestFreezeLearnDuringSustainedAnomaly` |
| R-7 | Divergence from child 5's final entity-axis or child 6's `SrcAS` API forces rework | children 5/6 land with a different shape than assumed | this spec is implementable once 5/6 are done; the A-1/A-2/A-3/A-6 rows pin the exact surfaces to re-check first |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `anomaly { detect { enabled true } }` + snapshot with `fe.SrcAS` set on peers | -> | `onTick` -> AS-keyed `buildCohorts` -> `scoreEntity` -> source-prefix incident with `Cohort="AS<n>"` | `TestASCohortDrivesIncident` |
| opt-in per-ASN leaf true + snapshot with a deviating AS | -> | `onTick` -> per-ASN `entityState` -> confirm -> ring entry with `EntityKind="as"` | `TestPerASNEntityScored` |
| snapshot with all `fe.SrcAS == 0` | -> | `onTick` -> `cohortPrefix` fallback, no per-ASN entity | `TestDegradeToPrefixWhenASUnset` |
| enabled daemon, synthetic AS-tagged flows | -> | facts -> judgment -> `show anomaly detect` lists an AS-cohort incident | `test/plugin/anomaly-as-cohort.ci` |

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
| 1 | enables detect with flowexport AS enrichment present; one host in a busy AS starts scanning | facts (`fe.SrcAS`) -> AS-keyed cohort -> rarity -> incident `Cohort="AS<n>"` -> `show anomaly detect` | `test/plugin/anomaly-as-cohort.ci` + `TestASCohortDrivesIncident` |
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
| `anomaly-as-cohort` | `test/plugin/anomaly-as-cohort.ci` | operator enables detect; AS-tagged synthetic flows drive an AS-cohort incident visible in `show anomaly detect` | FLAG: needs the fakeflow harness (child 4) to publish observations carrying AS, which needs child 6's producer stamping. Until then, `TestChainFactsToResponse`-style Go coverage crafts snapshots with `SrcAS` directly |

### Interop Tests
N/A -- this child adds no wire-protocol behavior. It reads a fact and emits an in-process event.
(Umbrella interop is owned by child 8, upstream FlowSpec.)

### Future (if deferring any tests)
- The `.ci` functional test is gated on child 6 (producer stamps `SrcAS`) and child 4 (fakeflow can
  carry it). The Go chain-level test is not gated and covers the same path against crafted snapshots.

## Files to Modify
- `internal/plugins/anomaly/detect/detector.go` - AS-keyed cohort building (key selection: `fe.SrcAS`
  when set else `cohortPrefix`, `+Inf` exclusion replicated); optional per-ASN `entityState` map +
  loop routed through `scoreEntity`/freeze-learn; `activate` sets `Cohort="AS<n>"` for the AS-cohort
  path and builds report-only AS-subject incidents; a per-ASN cap constant; the tracked-AS gauge.
  → check its `// Design:` annotation points at `plan/spec-anomaly-1-detect.md`; add this spec.
- `internal/core/anomalyevent/event.go` - additive, backward-compatible discriminator on
  `AnomalyDetected` (an `EntityKind` plus an `EntityAS uint32`, both `omitempty`) to represent the
  ASN subject. FLAG: if child 5 already added a generalized entity-kind field for its port
  dimension, REUSE it instead of adding a second (A-6).
- `internal/plugins/anomaly/detect/config.go` - opt-in `track-as-entities` bool (parse/default/validate).
- `internal/plugins/anomaly/detect/yang/ze-anomaly-detect-conf.yang` - the `track-as-entities` leaf
  (type boolean, default false) under `container detect`.
- `internal/plugins/anomaly/detect/show.go` - render `EntityKind`/`EntityAS` so `show anomaly detect`
  distinguishes an AS-subject incident from a prefix incident.

**Consumed but NOT modified here (child boundary):**
- `internal/component/trafficfeature/feature.go` - child 6 adds `SrcAS`; this child only reads it.
- child 5's generalized entity axis - this child plugs the ASN dimension in; it does not author it.
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
| Functional test for new RPC/API | [ ] Yes | `test/plugin/anomaly-as-cohort.ci` (gated per Functional Tests note) |
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
| 6 | Has a user guide page? | [ ] Yes | `docs/guide/anomaly-detection.md` (created once Phase A lands per umbrella) -- add the AS section |
| 7 | Wire format changed? | [ ] No | no wire change |
| 8 | Plugin SDK/protocol changed? | [ ] No | value-type event only |
| 9 | RFC behavior implemented? | [ ] No | RFC 7607 informs the sentinel choice only; no protocol enforcement |
| 10 | Test infrastructure changed? | [ ] No | reuses the fakeflow harness (child 4) |
| 11 | Affects daemon comparison? | [ ] No | |
| 12 | Internal architecture changed? | [ ] Yes | subsystem doc for the anomaly detector (the `// Design:` doc) -- note AS grouping + report-only AS subject |
| 13 | Route metadata keys added/changed? | [ ] No | |
| 14 | Prometheus counters added/changed? | [ ] Yes | telemetry doc -- add `ze_anomaly_tracked_as_entities` |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] No | no new event type; the `AnomalyDetected` struct gains optional fields on the existing type |
| 16 | Any changed source file referenced by existing doc source anchors? | [ ] Yes | grep `docs/` for anchors on `detector.go` / `event.go` and update stale claims |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] Yes | verify anomaly config examples against the new leaf |

## Files to Create
- `internal/plugins/anomaly/detect/as_cohort_test.go` - the unit tests above.
- `test/plugin/anomaly-as-cohort.ci` - the functional test (gated per the Functional Tests note).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella + learned 1048 |
| 2. Audit | Files to Modify/Create, TDD Plan -- confirm children 5/6 landed and re-check A-1/A-2/A-3/A-6 |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `./le verify lint run && ./le test-unit  && ./le functional` |
| 7-13 | Critical / Deliverables / Security review, re-verify |
| 14. Present summary | Executive Summary + learned summary |

### Implementation Phases
Each phase ends with a Self-Critical Review. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- confirm children 5/6 are merged; re-check the exact
   `fe.SrcAS` representation (A-3) and whether child 5 added an entity-kind field (A-6). Add the
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
   eviction + gauge) routed through `scoreEntity`/freeze-learn; add the additive event discriminator
   (or reuse child 5's); keep AS-subject incidents report-only (ring + show, no bus emit); render in
   `show.go`.
   - Tests: `TestPerASNEntityScored`, `TestFreezeLearnASEntity`, `TestASIncidentReportOnly`
   - Files: `detector.go`, `event.go`, `show.go`
5. **Functional test** -- `test/plugin/anomaly-as-cohort.ci` (gated on child 4/6 for AS-carrying flows).
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
| `SrcAS` field missing / different shape | STOP -- child 6 not landed or changed; re-validate A-1/A-3 |
| Entity axis not generalized as assumed | re-validate A-2; fall back to a bespoke per-ASN map |
| AS cohort rarity != prefix cohort rarity | bug in key selection, not `score.go`; re-read `buildCohorts` |
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
| Per-ASN incidents are report-only (ring + show), not emitted on the `Detected` bus | Emit with a discriminator and guard the responder | The shape responder keys `r.armed` on `Entity` (a prefix) with no `IsValid` guard (responder.go,90,102); keeping AS subjects off the bus preserves that invariant without editing a second plugin |
| Additive `EntityKind`/`EntityAS` on `AnomalyDetected`, reuse child 5's field if present | A synthetic prefix encoding an ASN | An ASN encoded as a `netip.Prefix` is semantically wrong and would still poison the responder; an explicit discriminator is honest and backward-compatible |
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
