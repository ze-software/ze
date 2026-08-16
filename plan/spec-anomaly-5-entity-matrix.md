# Spec: anomaly-5-entity-matrix (widen the entity axis to dest + port)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | 1048 (judgment layer); facts layer 1046 |
| Phase | B (umbrella `plan/spec-anomaly-0-umbrella.md`, child 5 "entity-matrix") |
| Updated | 2026-07-02 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you are reading it now)
2. `.claude/rules/planning.md` and `ai/rules/planning.md` - workflow rules
3. `plan/spec-anomaly-0-umbrella.md` - the umbrella; this is child 5 (row "entity-matrix"), R-1/R-7 memory risks, AC-3
4. `docs/architecture/traffic/traffic-analysis-layers.md` (facts contract), `docs/architecture/anomaly/anomaly-1-detect.md` (judgment contract)
5. Source: `internal/component/trafficfeature/feature.go`, `internal/component/trafficfeature/service.go`, `internal/plugins/anomaly/detect/detector.go`, `internal/plugins/anomaly/detect/score.go`, `internal/core/anomalyevent/event.go`

## Task

Generalize the behavioral-anomaly ENTITY axis from source-IP-prefix only to also cover
DESTINATION-IP and destination-PORT entities, so the detector can flag an anomalous target
(distributed sink / probed host) and an anomalous service port (unusual spread on a port),
not only an anomalous source.

Verified split (do not re-derive): the work lives MOSTLY in the FACTS layer
(`internal/component/trafficfeature`). Today only SOURCE entities carry a full feature vector;
a destination is tracked but accumulates only inbound bytes and is never emitted, and "port"
is only a per-source byte histogram, not a first-class entity. So `trafficfeature` must grow two
new keyed feature lists (per-dest, per-port). The detector re-key onto those lists is the smaller
half for DEST (same key type, prefix cohort transfers) and a genuine structural addition for PORT
(new key type, no cohort, event-contract widening). The pure scoring rule in `score.go` and the
freeze-learn + warmup discipline MUST be preserved unchanged.

EXCLUDES ASN entities and AS-origin cohorts: those are children 6 (as-enrichment, the tier-safe
prerequisite) and 7 (as-entities-cohorts). This spec adds no AS field and no flowexport dependency.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-anomaly-0-umbrella.md` (child 5 row; R-1 line 182; R-7 line 188; AC-3 line 206; Design Insight lines 290-292)
  → Constraint: umbrella verdict is "mostly a FACTS-layer change; dest carries only `inBytes` today, port is a per-source histogram; the detector re-key is the smaller half." Budget the bulk in `trafficfeature/feature.go`, not `detect`.
  → Constraint: R-1/R-7 -- the `10000` cap is PER MAP; child 5 must decide per-dimension vs shared cap and size the ceiling; early signal is `ze_anomaly_tracked_entities`.
  → Constraint: prefix cohort transfers to DEST; PORT has NO natural cohort (cohort-free scoring). Excludes ASN.
- [ ] `docs/architecture/anomaly/anomaly-1-detect.md` (via umbrella lines 52-53)
  → Constraint: scoring stays pure in `score.go`; freeze-learn (`scoreEntity` returns pending `baselineUpdate`s, `onTick` folds only when not-anomalous or still warming); any new entity type MUST preserve freeze-learn + warmup.
- [ ] `ai/rules/performance.md` + `ai/rules/performance.md`
  → Constraint: new keyed maps run on the 1s tick (hot-ish). Store typed values (`netip.Addr`, `uint16`), not strings; compare typed. No `fmt`/`.String()` string-building on the tick path; use `textbuf` for any display formatting (cold `show` path only).
- [ ] `ai/rules/plugins.md` + `docs/architecture/core-design.md` (via umbrella line 48)
  → Constraint: no new per-feature switch/field in a core/shared package; the widened entity lives on the existing `trafficfeature.Snapshot` fact surface and the existing `anomalyevent` value contract (value types only, no wire/schema change).

### RFC Summaries
- N/A (no protocol/wire behavior; this is internal fact aggregation + scoring).

**Key insights:**
- Two new keyed feature lists on `trafficfeature.Snapshot` (`Dests`, `Ports`) are the bulk of the work.
- DEST re-key is small (reuses `entityState`, `buildCohorts`, `cohortPrefix`, `scoreEntity`); PORT re-key needs a new key type, a cohort-free path, and event-contract widening to carry a port.
- `score.go` needs ZERO change: cohort-free is achieved by passing an empty cohort (its `rarity` already returns 0 when `count-1 < minSize`, `score.go`).
- The `anomalyevent` `Entity` is `netip.Prefix` (`event.go`); a PORT cannot be expressed as a prefix, so the event value contract must gain an entity-kind tag + a port field. The `shape` responder must be guarded so it never installs a source-address firewall term for a DEST or PORT entity (would throttle the victim).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/trafficfeature/service.go` - `FeatureEntry` (fields `Addr, FanOut, OutInRatio, PortEntropy, NewPeer, RarePort, Beaconing`) is defined at `service.go`; `Snapshot` carries ONLY `Sources []FeatureEntry` (`service.go`); 1s `tickInterval` and ticker at `service.go,187,202-210`; `Snapshot()` read surface at `service.go`.
  → Constraint: adding `Dests`/`Ports` to `Snapshot` is additive; existing consumers/tests that build `Snapshot{Sources: ...}` (e.g. `detector_test.go`) keep compiling.
- [ ] `internal/component/trafficfeature/feature.go` - `maxTrackedKey = 10000` at `feature.go`; per-source fan-out/port caps `maxDestsPerSource = 4096`, `maxPortsPerSource = 1024` at `feature.go`. `sourceState` accumulator (`activity, outBytes, inBytes, dests, ports, firstTick, lastActiveTick, gaps`) at `feature.go`. ONE shared `sources map[netip.Addr]*sourceState` at `feature.go`. `ingest` folds a flow into BOTH roles at `feature.go`: the SOURCE gets `outBytes`, `dests` fan-out, `ports` histogram (`feature.go`); the DESTINATION gets ONLY `inBytes += v` (`feature.go`). `snapshot` emits a `FeatureEntry` ONLY when the entity acted as a source this window (`sent := st.outBytes > 0`, `feature.go`); a pure receiver is deliberately NOT emitted (`feature.go,189`).
  → Constraint: today a destination exists in the map only to contribute inbound bytes to sources' out/in ratio; it carries NO fan-in, NO per-dest port histogram, NO gaps, and is never surfaced. "port" is not an entity at all -- it is `sourceState.ports` (`feature.go`), a per-source destination-port byte histogram feeding `PortEntropy`/`RarePort`.
  → Constraint: reuse `stats.Entropy` (`entropy.go`), `stats.IntervalRegularity` (`beacon.go`), `stats.NewWindow` (`window.go`), `ratio`/`rarePort`/`portValues` helpers (`feature.go`); do not re-derive.
- [ ] `internal/core/observation/observation.go` - `FlowKey{Src, Dst, SrcPort, DstPort, Proto}` at `observation.go`. The raw fields to key a dest (`Flow.Dst`) and a port (`Flow.DstPort`, `Flow.Proto`) already flow through every `KindFlow`/`FeatureFlowBytes` observation.
  → Constraint: no new observation field needed; dest/port keys come from the existing `FlowKey`.
- [ ] `internal/plugins/anomaly/detect/detector.go` - `states map[netip.Addr]*entityState` at `detector.go`; `maxTrackedEntities = 10000` at `detector.go`; `warmupTicks = 3` at `detector.go`. `onTick` iterates ONLY `snap.Sources` (`detector.go`) and builds cohorts from `snap.Sources` (`buildCohorts`, `detector.go`); cohort key is source-prefix (`cohortPrefix`, `detector.go`). Freeze-learn fold at `detector.go`. `scoreEntity` (`detector.go`) reads `fe.FanOut, fe.OutInRatio, fe.PortEntropy, fe.Beaconing, fe.NewPeer, fe.RarePort` and returns pending `baselineUpdate`s. `activate` builds `Entity: netip.PrefixFrom(addr, addr.BitLen())` (`detector.go`). `tracked` gauge set from `len(d.states)` at `detector.go`; gauge registered `ze_anomaly_tracked_entities` at `detector.go`.
  → Constraint: the detector reads only `Sources`; scoring dest/port requires iterating the new `Dests`/`Ports` lists and giving each its own keyed `entityState` map (DEST reuses `map[netip.Addr]`, PORT needs a `map[portKey]`). Freeze-learn/warmup logic is reused verbatim per map.
- [ ] `internal/plugins/anomaly/detect/score.go` - `zScore` (`score.go`), `cohortStats.rarity` leave-one-out with `+Inf`-safe exclusion and `n < minSize -> 0` (`score.go`), `combineScore` capped/discounted (`score.go`).
  → Constraint: PURE, must not change. Cohort-free PORT scoring is obtained by passing an empty `cohortAgg` so `rarity` returns 0 and only self-deviation drives the port score.
- [ ] `internal/core/anomalyevent/event.go` - `AnomalyDetected.Entity`, `AnomalyOngoing.Entity`, `AnomalyCleared.Entity` are all `netip.Prefix` (`event.go`); header comment "Value types only -- no wire or schema change" (`event.go`); `Namespace = "anomaly-detect"` (`event.go`).
  → Constraint: a PORT entity cannot be a `netip.Prefix`. The contract must gain an entity-kind tag (source/dest/port) plus `Port`/`Proto` value fields; `Entity` stays for source/dest and is zero for port. Still "value types only" (no wire/schema).
- [ ] `internal/plugins/anomaly/shape/responder.go` + `match.go` - `onDetected` acts on `e.Entity` (`responder.go`); `buildSourceTerm` renders `firewall.MatchSourceAddress{Prefix: entity}` (`match.go`); armed map keyed `map[netip.Prefix]` (`responder.go`).
  → Constraint: if the detector emits a DEST or PORT incident on the same events, the UNCHANGED responder would install a source-address term on the victim's prefix (or a zero prefix for a port). Child 5 MUST guard `onDetected`/`onOngoing`/`onCleared` to act only on `EntityKind == source`; dest/port incidents are report-only here (a dest/port firewall action is future work).
- [ ] `internal/plugins/anomaly/detect/show.go` - `handleShowAnomaly` formats `e.Entity.String()` (`show.go`); cmd YANG `show anomaly detect` unchanged (`cmd/yang/ze-anomaly-cmd.yang`).
  → Constraint: for a PORT incident `e.Entity` is the zero prefix; the show handler must format the entity kind-aware (e.g. dest prefix, or `proto/port`). No YANG change (same `show anomaly detect`).
- [ ] `internal/component/trafficstat/window.go` - the SIBLING measurement layer already maintains `dests map[netip.Addr]*stats.Window` and `ports map[portKey]*stats.Window` (`window.go,74-81`) with per-key rate + history and PER-MAP `maxTrackedKey` cap (`getOrCreate`, `window.go`); `portKey{port uint16, proto uint8}` at `window.go`.
  → Constraint: `trafficstat` is a SIBLING consumer of the same feed, not upstream of `trafficfeature`; it proves per-dest/per-port keying is already an accepted pattern (reuse `portKey` shape and the per-map cap idiom) but `trafficfeature` must derive its OWN behavioral features (fan-in, entropy, gaps), not read `trafficstat`.

**Behavior to preserve (do NOT regress):**
- Only source entities that acted as a source are emitted under `Snapshot.Sources` (`feature.go`); existing source features unchanged (`feature_test.go` must stay green).
- `score.go` pure rule byte-for-byte; freeze-learn + warmup in `detector.go`; `+Inf` ratio exclusion from cohorts (`detector.go`, `buildCohorts` test `detector_test.go`).
- The `shape` responder invariants (shadow-first default, auto-revert, blast-radius cap, kill-switch, allowlist); it must never install a term for a non-source entity.
- The anomaly-vs-DDoS domain separation: no shared struct/namespace/detector; no import of `ddos*` from anomaly.
- Existing metrics `ze_anomaly_incidents_total`, `ze_anomaly_active`, `ze_anomaly_tracked_entities` remain (the last may gain a `dimension` label -- see R-1 mitigation).

**Behavior to change (requested):**
- `trafficfeature.Snapshot` gains `Dests []FeatureEntry` and `Ports []PortFeatureEntry`; `trafficfeature` accumulates per-dest and per-port feature vectors.
- The detector scores the new dest/port entities, preserving freeze-learn + warmup; DEST uses dest-prefix cohorts, PORT is cohort-free.
- The `anomalyevent` value contract gains an entity-kind tag + `Port`/`Proto`; the `shape` responder gains a source-only guard; the detect `show` handler formats kind-aware.

## Data Flow (MANDATORY)

### Entry Point
- Operator config `anomaly { detect { enabled true } }` (unchanged) attaches the detector to `trafficfeature`. Neutral flow observations (`observation.Feed`, `KindFlow`/`FeatureFlowBytes`) enter `trafficfeature.ingest`.

### Transformation Path
1. **Facts widen (bulk):** `trafficfeature.ingest` (`feature.go`) folds each flow into three accumulators: SOURCE (unchanged), DEST (extend the existing dst branch to also track distinct sources = fan-in, a per-dest destination-port histogram, and active-tick gaps), and PORT (new: keyed by `portKey{DstPort, Proto}`, tracking distinct sources/dests, bytes, gaps, and whether the port itself is uncommon). `snapshot` (`feature.go`) emits `Sources`, `Dests`, `Ports`.
2. **Detector re-key:** `onTick` (`detector.go`) additionally iterates `snap.Dests` (into `destStates map[netip.Addr]`, cohorts by dest-prefix via the existing `buildCohorts`/`cohortPrefix`) and `snap.Ports` (into `portStates map[portKey]`, empty cohort). Each map runs the identical score -> freeze-learn -> confirm/clear pipeline. `activate`/`emit*` tag the event with the entity kind and, for ports, the `Port`/`Proto`.
3. **Judgment emit:** confirmed dest/port incidents emit on the same `anomaly-detect` events (kind-tagged), land in the recent-incident ring, and surface via `show anomaly detect` (kind-aware formatting).
4. **Response (guarded):** the `shape` responder ignores dest/port incidents (source-only guard); source behavior is byte-for-byte unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| observation.Feed -> trafficfeature | `ingest` reads `Flow.Dst`, `Flow.DstPort`, `Flow.Proto` (already present, `observation.go`) | [ ] |
| trafficfeature -> detect | `Snapshot.Dests`/`Snapshot.Ports` read on detector tick (additive to `Snapshot.Sources`) | [ ] |
| detect -> anomalyevent | kind-tagged `AnomalyDetected`/`Ongoing`/`Cleared` value structs | [ ] |
| detect -> shape | same events; responder guards on `EntityKind == source` | [ ] |

### Integration Points
- `trafficfeature.Snapshot` (`service.go`) - the fact surface, extended with two lists.
- `anomalyevent.AnomalyDetected` (`event.go`) - value contract, extended with kind + port.
- `internal/plugins/anomaly/detect/score.go` - reused unchanged (empty cohort = cohort-free).
- `internal/plugins/anomaly/shape/responder.go` - guarded, not extended, in this spec.

### Architectural Verification
- [ ] No bypassed layers (dest/port features derived in `trafficfeature` from the feed, not re-measured in the detector, not read from `trafficstat`)
- [ ] No unintended coupling (no `ddos*` import; no `flowexport` import; no AS field)
- [ ] No duplicated functionality (reuse `entityState`, `buildCohorts`, `scoreEntity`, `score.go`, `stats.*`, `portKey` shape)
- [ ] Zero-copy / typed preserved (keys stay `netip.Addr`/`portKey`, no string keys on the tick path; no `fmt` on the tick path)
- [ ] Registration over hardcoding (the widened entity rides the existing `Snapshot` + `anomalyevent` surfaces; no new per-dimension switch in a core/shared package; the `show` command tree is unchanged)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Dest carries only inbound bytes and is never emitted today | `feature.go` (dst branch `inBytes += v` only), `feature.go,189` (emit only when `outBytes>0`) | dest features partly exist; scope smaller | re-read `feature.go`; `feature_test.go` asserts pure-dest absent | confirmed |
| A-2 | "port" is a per-source histogram, not an entity | `feature.go` (`ports` field in `sourceState`) | port work smaller than budgeted | re-read `feature.go`; `feature_test.go` | confirmed |
| A-3 | The prefix cohort transfers to DEST unchanged | `cohortPrefix` groups any `netip.Addr` by prefix (`detector.go`); dest is a `netip.Addr` | dest scoring needs a new cohort model | reuse `buildCohorts` with dest list; unit test dest cohort rarity | unvalidated |
| A-4 | PORT has no natural cohort; cohort-free = self-deviation only | ports are `uint16`+proto, not addresses; `cohortPrefix` needs an `Addr` | port over-flags or needs a cohort model | pass empty `cohortAgg`; `rarity` returns 0 for `count-1 < minSize` (`score.go`) | unvalidated |
| A-5 | Cohort-free scoring needs NO `score.go` change | `rarity` already returns 0 below `minSize` (`score.go`); `cont` takes `math.Max(self, cohort.rarity(...))` (`detector.go`) | must fork the scoring rule (breaks 1048 purity) | drive `scoreEntity` with an empty `cohortAgg`; assert only self-deviation contributes | unvalidated |
| A-6 | Freeze-learn + warmup are entity-map-agnostic and reuse verbatim | `onTick` freeze fold (`detector.go`) operates per `entityState`, not per address type | dest/port baselines poison under sustained anomaly | replicate `TestFreezeLearnDuringSustainedAnomaly` for dest + port | unvalidated |
| A-7 | Adding `Dests`/`Ports` to `Snapshot` is backward-compatible | consumers build `Snapshot{Sources: ...}` (`detector_test.go`); struct-field add is additive | existing tests break; source path regresses | `go build ./...`; run `feature_test.go`, `detector_test.go` unchanged | unvalidated |
| A-8 | The umbrella framing ("mostly facts, small detector re-key") is complete | umbrella lines 160,188,290-292 | scope estimate too low | see A-9 (broken) | broken -- see below |
| A-9 | Emitting a dest/port incident on the existing events is safe for the responder | umbrella scopes child 5 as detect-only | responder throttles the victim / a zero-prefix term | read `responder.go`, `match.go` | broken -- responder acts on `Entity` as a source prefix; child 5 MUST add a source-only guard + widen the event value contract (kind + port). The umbrella's "mostly facts + small detector re-key" omits the event-contract widening and the responder guard. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Tracked-entity memory blows up: three independent maps in facts AND in the detector (source + dest + port) | `ze_anomaly_tracked_entities` climbs; eviction churn; RSS grows | PER-DIMENSION caps (matches the existing per-map idiom `feature.go`, `window.go`): sources `10000` (unchanged), dests `10000`, ports `4096`; keep `evictIdleTicks` per map; convert `ze_anomaly_tracked_entities` (`detector.go`) to a `GaugeVec` labeled `dimension` (source/dest/port) via `metrics.Registry.GaugeVec` (`metrics.go`) so an operator sees WHICH map grows |
| R-2 | Shared cap lets a source flood evict dest/port baselines (or vice-versa) | one dimension's tracked count pinned at cap while others starve | reject the shared-cap option; independent maps + independent caps (the codebase already caps per map) |
| R-3 | PORT features are semantically thin (a port has no "port entropy" of its own), so reusing `FeatureEntry` fields is misleading | port scores all-zero, or fires on a nonsense feature | dedicated `PortFeatureEntry` type with port-appropriate fields (fan-out = distinct sources on the port; out/in bytes ratio; source-spread entropy; new-port; rare-port = the port itself uncommon; beaconing); score with the same primitives, documented per-dimension meaning |
| R-4 | Widening the event value contract silently changes JSON emitted on the bus / in the ring | `show anomaly detect` output or bus subscribers see new keys | add `entity-kind` and `port` as `json:",omitempty"` value fields; source incidents keep identical JSON (kind defaults/omits to source); document in `docs/architecture/meta` if applicable |
| R-5 | The responder guard is forgotten, so an armed deployment throttles a busy DEST (the victim) | shadow log shows a dest/port "would act"; armed mode installs a term on a server prefix | source-only guard in `onDetected`/`onOngoing`/`onCleared`; unit test `TestResponderIgnoresNonSourceEntity`; keep the guard in the SAME commit as the event widening |
| R-6 | Dest scoring over-fires because every busy server (many sources -> high fan-in, high in/out) looks anomalous against a young baseline | dest incidents dominate the ring immediately on enable | warmup (`warmupTicks`) + dest-prefix cohort rarity (a normal server is normal vs its cohort); MinCohortSize fallback; confirm/clear debounce all reused unchanged |
| R-7 | Port beaconing/entropy are bounded by the 1s tick (same limit as source beaconing, umbrella line 48-49) | sub-second port beacons invisible | out of scope (child 9); document the 1s-tick limit; do not attempt sub-second here |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `KindFlow` obs with distinct dests on a target | -> | `trafficfeature.ingest`+`snapshot` emit `Snapshot.Dests` | `TestFeatureDestEntry` (`trafficfeature/feature_test.go`) |
| `KindFlow` obs concentrated on one port from many sources | -> | `trafficfeature` emits `Snapshot.Ports` | `TestFeaturePortEntry` (`trafficfeature/feature_test.go`) |
| `anomaly { detect { enabled true } }` + dest outlier | -> | `detector.onTick` scores `snap.Dests`, confirms, emits kind=dest | `TestChainDestOutlier` (`anomaly/detect/chain_integration_test.go`) |
| `anomaly { detect { enabled true } }` + port outlier | -> | `detector.onTick` scores `snap.Ports`, confirms, emits kind=port | `TestChainPortOutlier` (`anomaly/detect/chain_integration_test.go`) |
| dest/port incident reaches the responder | -> | `shape.onDetected` source-only guard | `TestResponderIgnoresNonSourceEntity` (`anomaly/shape/responder_test.go`) |
| `show anomaly detect` after enable | -> | `handleShowAnomaly` kind-aware format | `test/plugin/anomaly-show.ci` (existing; still resolves, now tolerant of kind field) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Flows from many sources to one destination in a window | `trafficfeature.Snapshot.Dests` contains a `FeatureEntry` for that dest with fan-in > 1 and non-zero in/out ratio; a pure-source is NOT duplicated into `Dests` |
| AC-2 | Flows concentrated on one high port across many sources | `trafficfeature.Snapshot.Ports` contains a `PortFeatureEntry` for that `{port,proto}` with fan-out > 1 and `RarePort` true when the port is uncommon |
| AC-3 | Cardinality churn on dests and ports | per-dimension maps stay bounded: dests <= 10000, ports <= 4096; idle entities evicted after `evictIdleTicks`; `ze_anomaly_tracked_entities{dimension=...}` reflects each map |
| AC-4 | A destination whose fan-in / ratio deviate hard from its dest-prefix cohort and its own baseline for `ConfirmDuration` ticks | detector confirms and emits an `AnomalyDetected` with `EntityKind == dest`, `Entity` = the dest prefix, correct cohort |
| AC-5 | A port whose spread deviates hard from its own baseline for `ConfirmDuration` ticks | detector confirms and emits an `AnomalyDetected` with `EntityKind == port`, `Port`/`Proto` set, `Entity` zero; scored by self-deviation only (no cohort) |
| AC-6 | Sustained dest anomaly and sustained port anomaly | freeze-learn holds: neither baseline drifts up while anomalous; the incident stays active (mirrors `TestFreezeLearnDuringSustainedAnomaly` for source) |
| AC-7 | Source-only feature stream (no dest/port outliers) | source detection is byte-for-byte unchanged: `feature_test.go` and `detector_test.go` pass unmodified; source incidents keep identical JSON |
| AC-8 | A dest or port incident is emitted while `shape` is armed | the responder does NOT install any firewall term for it (source-only guard); armed source behavior unchanged |
| AC-9 | `show anomaly detect` with a mix of source/dest/port incidents | each incident renders with its kind; source rows identical to today; port row shows `proto/port`, dest row shows the dest prefix |
| AC-10 | `score.go` is unchanged | `git diff` shows no edit to `internal/plugins/anomaly/detect/score.go`; cohort-free port scoring achieved via an empty cohort |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables detect, watches a distributed target flagged | flows -> `trafficfeature.Dests` -> `detector` dest map -> confirm -> `AnomalyDetected{kind=dest}` -> ring -> `show anomaly detect` | `TestChainDestOutlier` + `anomaly-show.ci` |
| 2 | Enables detect, watches an unusual service port flagged | flows -> `trafficfeature.Ports` -> `detector` port map -> confirm -> `AnomalyDetected{kind=port}` -> ring -> `show anomaly detect` | `TestChainPortOutlier` |
| 3 | Arms shape, confirms a dest/port incident does NOT throttle the victim | dest/port event -> `shape.onDetected` guard -> no firewall term | `TestResponderIgnoresNonSourceEntity` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFeatureDestEntry` | `internal/component/trafficfeature/feature_test.go` | dest `FeatureEntry` emitted with fan-in, in/out ratio, dest-port entropy; pure-source not in `Dests` | |
| `TestFeaturePortEntry` | `internal/component/trafficfeature/feature_test.go` | port `PortFeatureEntry` emitted with fan-out, rare-port; common port not rare | |
| `TestFeatureDestPortCapsAndEviction` | `internal/component/trafficfeature/feature_test.go` | dest map <= 10000, port map <= 4096, idle eviction per map | |
| `TestDetectDestCohortRarity` | `internal/plugins/anomaly/detect/detector_test.go` | dest scored against dest-prefix cohort; `+Inf` ratio excluded (mirror `detector_test.go`) | |
| `TestDetectPortCohortFree` | `internal/plugins/anomaly/detect/detector_test.go` | port scored by self-deviation only; empty cohort -> `rarity` 0; `score.go` untouched | |
| `TestDestPortConfirmClearLifecycle` | `internal/plugins/anomaly/detect/detector_test.go` | confirm after `ConfirmDuration`, clear after `ClearConsecutive` for dest and port entities | |
| `TestFreezeLearnDestPort` | `internal/plugins/anomaly/detect/detector_test.go` | sustained dest/port anomaly does not poison its baseline (mirror `detector_test.go`) | |
| `TestTrackedGaugeByDimension` | `internal/plugins/anomaly/detect/detector_test.go` | `ze_anomaly_tracked_entities` labeled by `dimension` reports per-map counts | |
| `TestResponderIgnoresNonSourceEntity` | `internal/plugins/anomaly/shape/responder_test.go` | dest/port `AnomalyDetected` installs no term; source unchanged | |
| `TestEventKindOmitemptyForSource` | `internal/core/anomalyevent/event_test.go` | a source event marshals to identical JSON (kind omitted/defaulted); dest/port carry the new fields | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| dest tracked-map size | 0..10000 | 10000 (new dest at cap returns nil, not created) | N/A | 10001 (rejected) |
| port tracked-map size | 0..4096 | 4096 (new port at cap returns nil) | N/A | 4097 (rejected) |
| `DstPort` | 0..65535 | 65535 | N/A | N/A (uint16) |
| `Proto` | 0..255 | 255 | N/A | N/A (uint8) |
| dest fan-in | 0..`maxDestsPerSource`-analogue | cap value | N/A | rejected above cap |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestChainDestOutlier` | `internal/plugins/anomaly/detect/chain_integration_test.go` | real `trafficfeature.Service` + real detector: a distributed-target cohort with one dest outlier confirms a `kind=dest` incident (extends the `TestChainFactsToResponse` pattern, `chain_integration_test.go`) | |
| `TestChainPortOutlier` | `internal/plugins/anomaly/detect/chain_integration_test.go` | real chain: one port sees a source-spread outlier and confirms a `kind=port` incident | |
| `anomaly-show` (existing) | `test/plugin/anomaly-show.ci` | `show anomaly detect` still resolves and returns `incidents` list with the new optional kind field; source path unchanged | |
| `anomaly-entity-matrix` (optional, if child 4 `fakeflow` has landed) | `test/plugin/anomaly-entity-matrix.ci` | daemon-level: `fakeflow` injects a dest + port outlier; `show anomaly detect` lists a dest and a port incident | conditional on child 4 |

### Interop Tests
- N/A. No wire/protocol behavior; `anomalyevent` is value types only (`event.go`). Justification: this spec adds no BGP family, capability, attribute, or wire encoding.

### Future (deferred, requires user approval)
- Daemon-level `.ci` for dest/port outliers is blocked on a synthetic traffic generator; child 4 (`interop-harness`) builds the `fakeflow` plugin for exactly this. If child 4 lands first, add `anomaly-entity-matrix.ci`; otherwise the Go integration tests (`TestChainDestOutlier`/`TestChainPortOutlier`) are the functional proof.

## Files to Modify
- `internal/component/trafficfeature/service.go` - add `Dests []FeatureEntry` and `Ports []PortFeatureEntry` to `Snapshot` (`service.go`); add the `PortFeatureEntry` type (`Port uint16`, `Proto uint8`, `FanOut int`, `OutInRatio float64`, `SrcEntropy float64`, `NewPort bool`, `RarePort bool`, `Beaconing float64`).
- `internal/component/trafficfeature/feature.go` - **bulk of the work.** Extend the dst branch of `ingest` (`feature.go`) to accumulate fan-in (distinct sources), a per-dest destination-port histogram, and gaps; add a per-port accumulator keyed by `portKey{DstPort, Proto}`; emit `Dests`/`Ports` in `snapshot` (`feature.go`); add per-dimension caps `maxTrackedDest = 10000`, `maxTrackedPort = 4096` (keep `maxTrackedKey` for sources). Reuse `ratio`, `rarePort`, `portValues`, `stats.Entropy`, `stats.IntervalRegularity`, `stats.NewWindow`.
- `internal/plugins/anomaly/detect/detector.go` - add `destStates map[netip.Addr]*entityState` and `portStates map[<portKey>]*entityState`; iterate `snap.Dests` (dest-prefix cohorts via `buildCohorts`/`cohortPrefix`) and `snap.Ports` (empty cohort) in `onTick` (`detector.go`); tag `activate`/`emitOngoing`/`emitCleared` with entity kind and `Port`/`Proto`; convert `tracked` gauge to a `dimension`-labeled `GaugeVec` (`detector.go,407-416`). Reuse `scoreEntity`, freeze-learn, warmup, confirm/clear verbatim. Do NOT edit `score.go`.
- `internal/core/anomalyevent/event.go` - add `EntityKind` (typed string const: source/dest/port) and `Port uint16`/`Proto uint8` value fields with `json:",omitempty"` to `AnomalyDetected` (and `Ongoing`/`Cleared` as needed) (`event.go`); keep source JSON identical.
- `internal/plugins/anomaly/shape/responder.go` - guard `onDetected`/`onOngoing`/`onCleared` (`responder.go`) to act only on `EntityKind == source`.
- `internal/plugins/anomaly/detect/show.go` - kind-aware entity formatting in `handleShowAnomaly` (`show.go`) using `textbuf` (cold path); include `entity-kind` and `port` in the incident map.
- `internal/plugins/anomaly/detect/detector_test.go`, `internal/component/trafficfeature/feature_test.go`, `internal/plugins/anomaly/shape/responder_test.go`, `internal/core/anomalyevent/event_test.go`, `internal/plugins/anomaly/detect/chain_integration_test.go` - new tests per the TDD plan.

### BGP Family Checklist
- N/A. This spec adds no SAFI, capability, attribute, or NLRI. (Delete-block rationale: no wire/protocol surface.)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | No new config leaf: dest/port scoring reuses the existing `anomaly/detect` thresholds. (If a future toggle is wanted, it is a follow-up, not this spec.) |
| YANG validation constraints | No | -- |
| YANG custom validators | No | -- |
| CLI commands/flags | No | `show anomaly detect` unchanged (`cmd/yang/ze-anomaly-cmd.yang`); only its payload gains an optional kind field |
| CLI grammar | No | -- |
| Editor autocomplete | No | -- |
| Functional test for new RPC/API | Yes | `internal/plugins/anomaly/detect/chain_integration_test.go` (`TestChainDestOutlier`/`TestChainPortOutlier`); `test/plugin/anomaly-show.ci` still passes |
| Pipe completeness | No | `show anomaly detect` output routing unchanged |
| Env var registration | No | -- |
| Doctor check for runtime dependencies | No | No new file path/socket/port/module/binary/cert. Existing `doctor-anomaly-detect-no-feature-source` (`detect/doctor.go`) already covers the flow-source dependency; dest/port ride the same feed |
| Prometheus counters/metrics | Yes | Convert `ze_anomaly_tracked_entities` to a `GaugeVec` labeled `dimension` (values source/dest/port) in `detector.go`; `ze_anomaly_incidents_total`/`ze_anomaly_active` unchanged (may aggregate all dimensions) |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` row "Behavioral Anomaly Detection" (line 79): note entity axis now covers source, destination, and destination-port; update source anchors |
| 2 | Config syntax changed? | No | No new YANG leaf (grep `docs/guide/configuration.md` for `anomaly` -> unchanged) |
| 3 | CLI command added/changed? | No | `show anomaly detect` verb unchanged |
| 4 | API/RPC added/changed? | Partial | `docs/architecture/api/commands.md` -- if `show anomaly detect` payload is documented, note the optional `entity-kind`/`port` fields |
| 5 | Plugin added/changed? | No | Same plugins (`anomaly-detect`, `anomaly-shape`) |
| 6 | Has a user guide page? | Yes (when it exists) | `docs/guide/anomaly-detection.md` is created by the umbrella after Phase A; when present, add the dest/port entity paragraph |
| 7 | Wire format changed? | No | Value types only (`event.go`) |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented? | No | -- |
| 10 | Test infrastructure changed? | No | Reuses existing Go integration + `.ci` harness |
| 11 | Affects daemon comparison? | No | -- |
| 12 | Internal architecture changed? | Yes | The `trafficfeature`/`anomaly` subsystem doc or `docs/features.md` line 78-79: facts surface now emits per-dest/per-port lists; entity axis widened |
| 13 | Route metadata keys added/changed? | No | -- |
| 14 | Prometheus counters added/changed? | Yes | Document the `dimension` label on `ze_anomaly_tracked_entities` in the anomaly telemetry doc / `docs/plugin-development/metrics.md` |
| 15 | Registered plugin/event/command/capability inventory changed? | Partial | `anomalyevent` value contract gains `entity-kind`/`port`; note in `docs/plugin-overview.md` if event payloads are inventoried |
| 16 | Changed source referenced by doc source anchors? | Yes | Grep `docs/` for `source: internal/component/trafficfeature/feature.go`, `source: internal/plugins/anomaly/detect/detector.go`, `source: internal/core/anomalyevent/event.go`, `source: internal/plugins/anomaly/detect/show.go` (`docs/features.md`) and update each stale claim |
| 17 | Docs show config/CLI/API examples for this area? | Yes | Verify `show anomaly detect` example output against the kind-aware `show.go` |

## Files to Create
- (none required beyond tests) - all changes are additive edits to existing files. New tests live in existing `*_test.go` files.
- `test/plugin/anomaly-entity-matrix.ci` - OPTIONAL, only if child 4 `fakeflow` has landed (see Functional Tests).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella child 5 + learned 1046/1048 |
| 2. Audit | Files to Modify, TDD Test Plan -- confirm current `Snapshot`/`onTick`/event shapes |
| 3. Wiring phase | Wiring Test table -- add `Dests`/`Ports` fields + failing dest/port feature and chain tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section -- loop to 0 BLOCKER/ISSUE |
| 6. Full verification | `make ze-lint-changed` then `make ze-unit-test` then `make ze-functional-test` |
| 7-10. Critical review | Critical Review Checklist below |
| 11. Deliverables | Deliverables Checklist below |
| 12. Security | Security Review Checklist below |
| 14. Present summary | Executive Summary per `ai/rules/planning.md`; learned summary `plan/learned/NNN-anomaly-5-entity-matrix.md` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** - add `Snapshot.Dests`/`Snapshot.Ports` and the `PortFeatureEntry` type as empty/stub emission; write failing `TestFeatureDestEntry`, `TestFeaturePortEntry`, `TestChainDestOutlier`, `TestChainPortOutlier`.
   - Files: `trafficfeature/service.go`, `trafficfeature/feature.go` (stub), test files
   - Verify: struct fields exist, tests compile and FAIL (lists empty)
2. **Phase: Facts - dest accumulator** - extend the dst branch of `ingest` and `snapshot` to emit dest `FeatureEntry` (fan-in, in/out ratio, dest-port entropy, gaps, new/rare); add `maxTrackedDest` cap + eviction.
   - Tests: `TestFeatureDestEntry`, `TestFeatureDestPortCapsAndEviction`
   - Verify: dest features correct; pure-source not duplicated; caps hold
3. **Phase: Facts - port accumulator** - add the `portKey`-keyed port map; emit `PortFeatureEntry` (fan-out, out/in ratio, source-spread entropy, new/rare-port, beaconing); add `maxTrackedPort` cap.
   - Tests: `TestFeaturePortEntry`, caps test
   - Verify: port features correct; common port not rare
4. **Phase: Event contract** - add `EntityKind` + `Port`/`Proto` (`omitempty`) to `anomalyevent`; assert source JSON unchanged.
   - Tests: `TestEventKindOmitemptyForSource`
   - Verify: source marshal identical; dest/port carry new fields
5. **Phase: Detector re-key - dest** - iterate `snap.Dests` into `destStates`; reuse `buildCohorts`/`cohortPrefix` for dest-prefix cohorts; tag events kind=dest.
   - Tests: `TestDetectDestCohortRarity`, `TestDestPortConfirmClearLifecycle`, `TestFreezeLearnDestPort`
   - Verify: dest confirm/clear + freeze-learn work; `score.go` untouched
6. **Phase: Detector re-key - port** - iterate `snap.Ports` into `portStates` with an empty cohort; tag events kind=port + `Port`/`Proto`.
   - Tests: `TestDetectPortCohortFree`
   - Verify: port scored by self-deviation only; `git diff score.go` empty
7. **Phase: Metrics + guard + show** - `dimension`-labeled `GaugeVec`; `shape` source-only guard; kind-aware `show.go`.
   - Tests: `TestTrackedGaugeByDimension`, `TestResponderIgnoresNonSourceEntity`; `anomaly-show.ci` still green
   - Verify: no term for non-source; gauge per dimension
8. **Functional tests** - `TestChainDestOutlier`/`TestChainPortOutlier` pass end to end; optional `.ci` if `fakeflow` present.
9. **Full verification** - `make ze-precommit-verify` (or scoped to changed per `ai/rules/git-safety.md`).
10. **Complete spec** - fill audit tables; learned summary; two-commit closure per `.claude/rules/planning.md`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; `Dests` AND `Ports` emitted AND scored |
| Feature completeness | Dest uses prefix cohort; port is cohort-free; both preserve freeze-learn + warmup + confirm/clear |
| Correctness | Dest ratio is in/out sense (not the source out/in inverted by accident); rare-port uses `commonPorts`; `+Inf` still excluded from dest cohort |
| Purity | `git diff internal/plugins/anomaly/detect/score.go` is EMPTY (AC-10) |
| Naming | JSON keys kebab-case (`entity-kind`, `port`); `EntityKind` const values source/dest/port |
| Data flow | Dest/port features derived in `trafficfeature` from the feed; detector never reads `trafficstat`; no `flowexport`/AS/`ddos` import |
| Registration over hardcoding | Widened entity rides existing `Snapshot`/`anomalyevent`; no new switch in a core/shared struct; `show` tree unchanged |
| Prometheus counters | `ze_anomaly_tracked_entities` labeled `dimension`; documented |
| Responder non-regression | Source-only guard present; armed dest/port installs no term |
| Rule: no partial completion | Both dest AND port implemented AND tested; not "dest done, port deferred" |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `Snapshot.Dests`/`Snapshot.Ports` emitted | `go test ./internal/component/trafficfeature/ -run 'Dest|Port'` |
| Detector scores dest + port | `go test ./internal/plugins/anomaly/detect/ -run 'Dest|Port|Chain'` |
| `score.go` unchanged | `git diff --stat internal/plugins/anomaly/detect/score.go` shows no change |
| Responder guard | `go test ./internal/plugins/anomaly/shape/ -run NonSource` |
| Source path unchanged | `go test ./internal/component/trafficfeature/ ./internal/plugins/anomaly/detect/ -run 'FanOut|ConfirmClear|FreezeLearn'` (existing tests, unmodified) |
| Per-dimension gauge | `grep -n 'GaugeVec' internal/plugins/anomaly/detect/detector.go` |
| Docs updated | `make ze-doc-verify`; grep `docs/features.md` source anchors resolve |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Per-dimension caps enforced BEFORE map insert (mirror `feature.go`, `window.go`); a scan across ports cannot exceed `maxTrackedPort`; a fan-in flood cannot exceed `maxTrackedDest`; idle eviction runs per map |
| Untrusted input | `DstPort`/`Proto`/`Dst` come from the feed (attacker-influenced); they are typed (`uint16`/`uint8`/`netip.Addr`), never formatted into a map key string on the tick path |
| Victim protection | Source-only responder guard prevents an attacker from weaponizing a spoofed-dest anomaly to make Ze throttle a legitimate server |
| Error leakage | `show` formatting on the cold path uses `textbuf`; no panic on a zero prefix for a port entity |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Dest/port test fails wrong reason | Fix test setup (windowing needs a `snapshot` per tick) |
| `score.go` changed | STOP -- redesign to pass an empty cohort instead of forking the rule |
| Source test regresses | Re-read `feature.go`; the dest/port accumulation must not alter the source branch |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (during research) child 5 is "mostly facts + a small detector re-key" (umbrella) | Also needs an event-contract widening (port cannot be a `netip.Prefix`, `event.go`) AND a `shape` responder source-only guard (`responder.go` acts on `Entity` as a source term) | read `event.go` + `responder.go`/`match.go` | added A-9 (broken), R-5; event widening + responder guard pulled into scope |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| | | |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| | | | |

## Design Insights
- The bulk is in `trafficfeature`: a destination today is a bare inbound-byte counter (`feature.go`) and a port is only a per-source histogram (`feature.go`). Making each a first-class entity means a role-flipped dest accumulator (fan-in, dest-port entropy, gaps) and a brand-new port accumulator, both emitted on the `Snapshot`. That is where the estimate lives, not in the detector.
- The detector generalizes for free on DEST (same key type, same cohort machinery) but not on PORT: a port has no address and thus no prefix cohort, so it must be scored cohort-free. Elegantly, `score.go` needs no change -- an empty cohort makes `rarity` return 0 (`score.go`), so port scoring falls back to self-deviation automatically.
- The umbrella's "mostly facts, small detector re-key" framing is right about the CENTER of mass but omits two edges: a value-contract widening (to carry a port and tag the kind) and a responder guard (so an armed deployment never throttles a victim destination). Both are in scope here.

## Core Insight
Widening the entity axis is not "teach the detector new keys" -- it is "teach the FACTS layer to emit two new keyed feature vectors, then let the already-general judgment layer consume them," with the only genuinely new judgment work being cohort-free PORT scoring (achieved without touching the pinned rule) and a value-contract tag so the response layer stays victim-safe.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-dimension independent caps (src 10000, dest 10000, port 4096) | One shared 10000 cap across all dimensions | The codebase already caps per map (`feature.go`, `window.go`); a shared cap lets one dimension evict another's baselines (R-2). Ports have lower natural cardinality, so a smaller ceiling suffices |
| `ze_anomaly_tracked_entities` becomes a `dimension`-labeled `GaugeVec` | Three separate gauges; keep one summed gauge | `GaugeVec` exists (`metrics.go`); one labeled metric keeps the name (R-4) while giving the R-1 early signal per dimension |
| Dedicated `PortFeatureEntry` type | Reuse `FeatureEntry` with a meaningless `Addr` | A port has no address; a dedicated type documents port-appropriate field meaning and avoids a zero `Addr` masquerading as an entity (R-3) |
| Cohort-free PORT via empty `cohortAgg` (no `score.go` edit) | Add a port-cohort model; fork the scoring rule | Preserves 1048 purity (AC-10); `rarity` already returns 0 below `minSize` |
| Same events, kind-tagged; source-only responder guard | New port/dest event types; separate responder | Minimal contract churn; keeps subscribers working; keeps the anomaly domain single-namespace; guard keeps armed mode victim-safe (R-5) |
| Additive `Snapshot` fields | Replace `Sources` with a unified list | Backward-compatible: existing `Snapshot{Sources: ...}` builders and `feature_test.go`/`detector_test.go` keep compiling (A-7) |

## Known Limitations
- No ASN entities or AS-origin cohorts (children 6/7; behind the AS-enrichment prerequisite).
- Port beaconing/entropy are bounded by the 1s tick (same limit as source beaconing, umbrella line 48); sub-second port timing is child 9.
- No dest/port firewall RESPONSE: the `shape` responder only acts on source entities in this spec (a dest/port action is future work). Dest/port incidents are report-only.
- No new config leaf to independently toggle dest/port scoring; they enable with `anomaly { detect { enabled true } }`. A per-dimension toggle is a possible follow-up.

## RFC Documentation
- N/A (no protocol behavior).

## Implementation Summary
### What Was Implemented
- [Filled during /implement]

### Bugs Found/Fixed
- [Filled during /implement]

### Documentation Updates
- [Filled during /implement]

### Deviations from Plan
- [Filled during /implement]

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `trafficfeature` emits per-dest feature list | unit test | `TestFeatureDestEntry` |
| `trafficfeature` emits per-port feature list | unit test | `TestFeaturePortEntry` |
| Detector scores dest (prefix cohort) end to end | functional test | `TestChainDestOutlier` |
| Detector scores port (cohort-free) end to end | functional test | `TestChainPortOutlier` |
| Freeze-learn + warmup preserved for dest/port | unit test | `TestFreezeLearnDestPort` |
| Source path + `score.go` unchanged | test + diff | source tests unmodified pass; `score.go` diff empty |
| Armed responder stays victim-safe | unit test | `TestResponderIgnoresNonSourceEntity` |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | file:line | |

### Fixes applied
-

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
| Entry Point | .ci/test File | Verified |
|-------------|---------------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled - 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/trafficfeature`, `internal/plugins/anomaly/detect`, `internal/core/anomalyevent`)
- [ ] Integration completeness proven end-to-end (`TestChainDestOutlier`/`TestChainPortOutlier`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and `docs/features.md` updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (A-8/A-9 already broken, in Mistake Log)

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (dest + port are two of three concrete dimensions)
- [ ] No speculative features (ASN excluded; no per-dimension config toggle)
- [ ] Single responsibility (facts emit; detector judges; responder guarded)
- [ ] Explicit > implicit (entity-kind tag is explicit on the event)
- [ ] Minimal coupling (no `ddos`/`flowexport` import)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (caps, port/proto ranges)
- [ ] Functional tests for end-to-end behavior (`TestChainDestOutlier`/`TestChainPortOutlier`)
- [ ] Interop tests N/A (value types only, justified)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING - before ANY commit)
- [ ] Critical Review passes - all 6 checks documented pass
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-anomaly-5-entity-matrix.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-anomaly-5-entity-matrix.md`
</content>
</invoke>
