# Spec: Anomaly Child 6 -- AS Enrichment (stamp origin-AS onto the facts surface)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | 1046 (traffic-analysis restructure: `observation.Feed` + `trafficfeature`) |
| Phase | B |
| Updated | 2026-07-02 |

Umbrella: `plan/spec-anomaly-0-umbrella.md` -- this is **child 6 (`as-enrichment`)**, the
**prerequisite for all AS work** (children 7 `as-entities-cohorts` and any AS cohort in child 5).
It ships the tier-safe seam that carries origin-AS from the flowexport producer to the neutral
facts surface the detector already reads. It adds no detector logic and no operator knob.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-anomaly-0-umbrella.md` - child-6 row, R-3 (degrade-to-prefix), A-3 (tier violation)
4. `ai/rules/architecture.md` + `scripts/dev/dep_audit.py` (the `ze-tier-check` gate; `engine_depended` at dep_audit.py)
5. Source: `internal/plugins/flowexport/exporter.go` (producer), `internal/core/observation/observation.go` (feed type), `internal/component/trafficfeature/{feature.go,service.go}` (facts surface)

## Task

Add an origin-AS field to the neutral traffic-facts surface, **stamped at the flowexport
producer** that already enriches each flow and publishes it into `observation.Feed`, so a later
security-domain consumer (child 7) can read AS on the `trafficfeature.FeatureEntry` it already
consumes -- **without any plugin importing another plugin**.

The load-bearing constraint: a direct `internal/plugins/anomaly/detect` ->
`internal/plugins/flowexport/enrich` import is **forbidden** and fails `make ze-tier-check`
(`dep_audit.py` `engine_depended`, dep_audit.py: flowexport is a config-driven engine
in the edge tier; a feature importing its subtree flips its expected tier to `component`, which
is a misplacement -> exit 2). The sanctioned ze data path is **producer -> core feed -> component
global -> consumer**, never plugin->plugin. flowexport already looks the AS up; this spec has it
copy the AS it already holds onto the observation it already publishes.

AS enrichment is **optional**: when the AS is unknown (flowexport enricher absent, or RIB has no
matching prefix) the field is `0` and downstream degrades to prefix cohorts (umbrella R-3).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `ai/rules/architecture.md` - the tier taxonomy and the `ze-tier-check` gate this spec must pass
  → Constraint: tier = dependency direction. `internal/core` (observation) is imported by everyone; `internal/component` (trafficfeature) is depended on by plugins; `internal/plugins` (flowexport, anomaly) is an edge nobody depends on. Adding a field to a core or component struct is tier-safe; a plugin importing another plugin's engine subtree is not.
  → Constraint: flowexport is a config-driven engine (`sdk.NewWithConn`, register.go) living in `internal/plugins`; it is NOT in `tier_migration_baseline.txt` (clean edge). If any `internal/component/*` or `internal/plugins/*` file imports `internal/plugins/flowexport/...`, `engine_depended` returns True, its expected tier becomes `component`, and the gate fails. The stamp must add zero new importer of flowexport.
- [ ] `ai/rules/architecture.md` - cross-plugin data path rule (referenced by umbrella)
  → Constraint: cross-plugin data flows producer -> core feed -> component global -> consumer. Origin-AS must RIDE the `observation`/`trafficfeature` surface, stamped by the producer; the consumer must never fetch it.
- [ ] `ai/rules/plugins.md` - delete-the-folder invariant
  → Constraint: removing flowexport must not break the observation/trafficfeature types. The AS field is an inert `uint32` on core/component structs that defaults to `0`; it carries meaning only while flowexport is loaded and enriching. Nothing in core/component may spell "flowexport".

### Source of the constraint (gate code, MUST read)
- [ ] `scripts/dev/dep_audit.py` - `engine_depended(engine_rel, module, edges)` at dep_audit.py
  → Constraint: returns True when a non-test, non-registration importer under `internal/component/` or `internal/plugins/` (excluding the engine's own subtree and `NON_FEATURE_PREFIXES` core/chaos/test) imports the engine's package subtree. This is exactly what a `detect -> flowexport/enrich` import would trigger. The producer-stamp path adds no such importer, so the gate stays green.
  → Constraint: `make ze-tier-check` runs `dep_audit.py --selftest` then `--check` (Makefile:288-290); it is part of `_ze-verify-impl` (Makefile:274). The tier assertion in this spec is a gate run, not a Go test.

### RFC Summaries (MUST for protocol work)
- N/A -- no wire protocol change. Origin-AS already exists on the flow (populated from the BGP RIB by the flowexport enricher); this spec moves an in-process value between existing structs.

**Key insights:**
- The stamp point is `internal/plugins/flowexport/exporter.go`: `exportFlows` enriches each flow (`e.enricher.Enrich`, exporter.go; sets `flows[i].SrcAS/DstAS`, exporter.go) and then builds and publishes `observation.Observation{KindFlow, FeatureFlowBytes, ...}` (exporter.go). The AS is ALREADY materialized on the flow struct (`ConntrackFlow.SrcAS`, flowtypes.go) when the observation is constructed, so stamping is a **zero-lookup field copy** inside the existing publish loop.
- flowexport is the **sole** publisher of `KindFlow` observations (grep: only exporter.go constructs one), and `trafficfeature.ingest` processes **only** `KindFlow`+`FeatureFlowBytes` (feature.go). So the detector's entire fact surface is already 100% flowexport-derived. This spec adds an OPTIONAL field on that existing path; it does not create a new hard dependency (refines umbrella R-3).
- The consumer (child 7) already imports `internal/component/trafficfeature` and takes a `trafficfeature.FeatureEntry` by value (`detector.go scoreEntity(fe trafficfeature.FeatureEntry, ...)`). Reading a new `fe.SrcAS` field adds ZERO new imports.

## Current Behavior (MANDATORY)

**Source files read (BEFORE writing this spec):**
- [ ] `internal/core/observation/observation.go` - `Observation` struct at observation.go (Kind, Iface, Flow, Feature, Value, At); `FlowKey` at observation.go (Src, Dst, SrcPort, DstPort, Proto); `KindFlow` at observation.go. `Feed.Publish` fans an `Observation` value to subscribers (observation.go); `Global()` is the process-wide feed (observation.go).
  → Constraint: there is NO AS field on `Observation` or `FlowKey` today. `Observation` is a value copied through a `chan Observation` (bufferCap 1024, observation.go,115). A new field must be a plain scalar (no pointer, no slice) to keep the value copy-safe and allocation-free.
- [ ] `internal/plugins/flowexport/exporter.go` - `exportFlows` (exporter.go) is the producer: enriches (`Enrich`, exporter.go), taps the recent ring, fans to collectors, then publishes one `KindFlow` observation per flow (exporter.go).
  → Constraint: the publish loop reads `f := &flows[i]` (exporter.go); `f.SrcAS` is already set from enrichment above. Stamp inside this loop; do not add a second enricher call.
- [ ] `internal/plugins/flowexport/enrich/enricher.go` - `Lookup(addr) (ASEntry, bool)` at enricher.go returns the low-level radix hit; `Enrich(src,dst) Enrichment` at enricher.go is the wrapper the producer actually calls (it calls Lookup twice, enricher.go/68). `Enrichment.SrcAS/DstAS uint32` (enricher.go); `ASEntry.AS uint32` (radix.go).
  → Constraint: `SrcAS == 0` is the natural "unknown" sentinel: `Enrich` leaves `SrcAS` at 0 on a RIB miss, and the producer only enriches `if e.enricher != nil` (exporter.go). `ConntrackFlow.SrcAS` documents "0 if unknown" (flowtypes.go).
- [ ] `internal/component/trafficfeature/service.go` - `FeatureEntry` at service.go (Addr, FanOut, OutInRatio, PortEntropy, NewPeer, RarePort, Beaconing); `Snapshot` at service.go; `Service.Snapshot()` returns the latest finalized view (service.go). Service subscribes to `observation.Feed` and forwards each obs to `agg.ingest` (service.go).
  → Constraint: `FeatureEntry` is a pure value of facts (no verdict). The AS field is a fact (a measurable label), so it belongs here. It is returned by value in `Snapshot.Sources`.
- [ ] `internal/component/trafficfeature/feature.go` - `sourceState` at feature.go (window-scoped outBytes/inBytes/dests/ports reset each tick; persistent activity/firstTick/lastActiveTick/gaps carry across ticks). `ingest` folds one flow into the SOURCE role (feature.go) and the DEST role (feature.go). `snapshot` emits a `FeatureEntry` only for entities that acted as a source this window (`sent := st.outBytes > 0`, feature.go; built at feature.go; window fields cleared at feature.go).
  → Constraint: the same `netip.Addr` is one `sourceState` regardless of role. In the SOURCE branch `obs.SrcAS` is THIS entity's AS; in the DEST branch `obs.SrcAS` is the OTHER endpoint's AS. Therefore AS must be stamped ONLY in the SOURCE branch. Only source entities are emitted today, so the source AS is exactly what a `FeatureEntry` needs.
  → Constraint: AS is an entity property (its prefix's origin), so it belongs in the PERSISTENT part of `sourceState` and must NOT be cleared in the window reset (feature.go).
- [ ] `internal/plugins/anomaly/detect/detector.go` - imports `internal/component/trafficfeature` (detector.go), consumes `trafficfeature.Snapshot()` (register.go) and `trafficfeature.FeatureEntry` by value (detector.go). Does NOT import flowexport (grep: only a comment in a test).
  → Constraint: this is the future consumer (child 7). It already has the imports needed to read `fe.SrcAS`; child 6 must not require it to import anything new. Child 6 does NOT modify the detector.

**Behavior to preserve:**
- `Observation` stays a copy-safe value on `chan Observation`; `Publish` stays non-blocking.
- `trafficfeature` stays neutral facts (no verdict); it ingests only `KindFlow`+`FeatureFlowBytes`.
- flowexport stays an edge plugin in `internal/plugins`; nothing in core/component spells "flowexport".
- Existing flow-record / NetFlow / IPFIX collector fan-out and the `show flow-recent` src-as column (cmd_show.go) are unchanged.

**Behavior to change:**
- `Observation` gains an origin-AS scalar; the flowexport producer sets it from the already-enriched flow; `trafficfeature` carries it through ingest to `FeatureEntry`. All additive; `0` when unknown.

## Data Flow (MANDATORY)

### Entry Point
- A conntrack flow inside flowexport, enriched with its source origin-AS from the BGP RIB radix tree (`ConntrackFlow.SrcAS`, flowtypes.go, set at exporter.go).

### Transformation Path
1. **Stamp (producer):** in `exportFlows` publish loop (exporter.go), copy `f.SrcAS` onto the `Observation` being built. No new enricher call; the value is already on the flow.
2. **Feed (core):** the enlarged `Observation` value rides `observation.Feed.Publish` -> subscriber channels unchanged (observation.go).
3. **Ingest (component):** `trafficfeature.Service` forwards each obs to `agg.ingest` (service.go); in the SOURCE-role branch (feature.go) the entity's persistent `sourceState` records `obs.SrcAS` (only when non-zero, so an unknown-AS flow never clobbers a previously known AS).
4. **Snapshot (component):** `agg.snapshot` copies the persistent AS into the emitted `FeatureEntry.SrcAS` (feature.go); returned by value in `Snapshot.Sources`.
5. **Consume (out of scope, child 7):** the detector reads `fe.SrcAS` from the same `Snapshot()` it already reads; `0` -> degrade to prefix cohorts.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| flowexport (plugin) -> observation (core) | existing `observation.Global().Publish` at exporter.go; AS rides as a new struct field, no new import | [ ] |
| observation (core) -> trafficfeature (component) | existing feed subscription (service.go); field read in `ingest` | [ ] |
| trafficfeature (component) -> detect (plugin, child 7) | existing `Snapshot()` / `FeatureEntry` value read; `fe.SrcAS` adds no import | [ ] |
| detect -/-> flowexport (FORBIDDEN) | must remain absent; `ze-tier-check` proves no such edge exists | [ ] |

### Integration Points
- `internal/plugins/flowexport/exporter.go` - the publish loop that gains the stamp.
- `internal/core/observation/observation.go` - the `Observation` struct that gains `SrcAS`.
- `internal/component/trafficfeature/{feature.go,service.go}` - `sourceState`/`FeatureEntry` carry it.

### Architectural Verification
- [ ] No bypassed layers (AS follows producer -> core feed -> component global -> consumer; the consumer never fetches from flowexport)
- [ ] No unintended coupling (no new import edge; `detect` still does not import flowexport; `ze-tier-check` green)
- [ ] No duplicated functionality (reuses the already-computed `ConntrackFlow.SrcAS`; no second enricher lookup)
- [ ] Zero-copy preserved where applicable (AS is a `uint32` scalar on an already-copied value; no heap, no slice)
- [ ] Registration over hardcoding (no new registry needed; additive fields on existing types; no per-plugin switch in a core package)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Producer-stamping is tier-safe; only a `detect -> flowexport/enrich` import trips the gate | `dep_audit.py` `engine_depended` 486-506; flowexport is an edge engine (register.go) not in `tier_migration_baseline.txt` | design invalid | run `make ze-tier-check` after the change; grep for any `internal/plugins/anomaly` -> `internal/plugins/flowexport` import | **confirmed** (gate code read; no new importer is introduced) |
| A-2 | The AS is already on the flow at publish time, so the stamp is a field copy with no extra lookup | `ConntrackFlow.SrcAS` flowtypes.go; set at exporter.go before the publish loop at exporter.go | need a second enricher call | read exporter.go 246-324 | **confirmed** |
| A-3 | `SrcAS == 0` is a safe "unknown" sentinel (public ASNs are never 0; AS0 is reserved) | `Enrich` leaves 0 on RIB miss (enricher.go); flowtypes.go "0 if unknown"; RFC 7607 reserves AS0 | a real AS0 flow reads as unknown | none needed (AS0 is reserved and never announced) | **confirmed** |
| A-4 | Only source entities are emitted, so a source-branch stamp fully populates every `FeatureEntry` | `sent := st.outBytes > 0` gate feature.go; dest-only entities carry `inBytes` and are not emitted | dest entities would carry wrong/zero AS | read feature.go 141-209; unit test asserts `FeatureEntry.SrcAS` matches the source's AS | **confirmed** |
| A-5 | Adding a `uint32` to `Observation` does not break the value-copy feed or other consumers | `Feed.Publish` copies the value (observation.go); other consumers (`trafficstat/window.go`) read only fields they know | feed regression | `make ze-unit-test`; existing observation/trafficstat tests | unvalidated |
| A-6 | AS on a source entity is stable within a window (same prefix -> same origin AS) so last-non-zero-write-wins is correct | AS is a function of the source prefix in the RIB | flapping AS within a window | stamp only on non-zero `obs.SrcAS`, keep persistent | **confirmed** (design choice records last known AS) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Security domain becomes dependent on flowexport being enabled | detector produces nothing when flowexport is off | **pre-existing, not introduced here**: flowexport is the sole `KindFlow` publisher (exporter.go) and `trafficfeature` ingests only `KindFlow` (feature.go), so the fact surface already requires flowexport. AS is strictly additive; `SrcAS == 0` degrades to prefix cohorts (umbrella R-3) |
| R-2 | An unknown-AS flow (RIB miss) overwrites a previously known AS on the same source within a window | source entity's `SrcAS` drops to 0 mid-window | stamp only when `obs.SrcAS != 0`; keep `srcAS` in the persistent (non-reset) part of `sourceState` |
| R-3 | Enlarging `Observation` raises per-observation copy cost / channel memory | throughput regression under high flow rate | field is a single `uint32` (4 bytes, likely absorbed by struct alignment); no allocation; measure with existing flow-export benchmarks if concerned |
| R-4 | A future dest-entity axis (child 5) needs the dest AS, which this spec does not carry | child 5 finds no dest AS on the observation | documented in Known Limitations: `DstAS` is a one-line additive follow-up owned by child 5 when it emits dest entities; `ConntrackFlow.DstAS` already exists (flowtypes.go) |
| R-5 | Someone "simplifies" by importing `flowexport/enrich` from the detector to get AS directly | `make ze-tier-check` exit 2 naming flowexport | the gate is the guardrail; this spec exists precisely to make that import unnecessary |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| flowexport publishes an enriched flow (`f.SrcAS` set) | → | stamp in `exportFlows` publish loop (exporter.go) | `TestExportFlowsStampsSrcAS` (flowexport: subscribe to a feed, run `exportFlows`, assert published `Observation.SrcAS == f.SrcAS`) |
| a `KindFlow` observation with `SrcAS=N` enters the feed | → | `agg.ingest` source branch + `snapshot` (feature.go) | `TestFeatureEntryCarriesSrcAS` (trafficfeature: ingest an obs with `SrcAS=N`, tick, assert `Snapshot().Sources[i].SrcAS == N`) |
| an unknown-AS flow (`SrcAS=0`) | → | same path, sentinel behavior | `TestFeatureEntrySrcASUnsetWhenUnknown` (assert `FeatureEntry.SrcAS == 0`, no clobber of a prior known AS) |
| the whole change compiled | → | no forbidden import edge | `make ze-tier-check` passes (`dep_audit.py --check` exit 0); grep proves no `anomaly -> flowexport` import |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | flowexport enriches a flow with source origin-AS N and publishes it | the published `observation.Observation` carries `SrcAS == N` (stamped at the producer, no second enricher lookup) |
| AC-2 | a `KindFlow`+`FeatureFlowBytes` observation with `SrcAS=N` for source addr S is ingested and a tick elapses | `trafficfeature.Snapshot()` returns a `FeatureEntry` for S with `SrcAS == N` |
| AC-3 | flowexport enrichment is absent (nil enricher or RIB miss), so `SrcAS=0` | the observation and the resulting `FeatureEntry` carry `SrcAS == 0`; a later unknown-AS flow does not overwrite a previously known AS on the same source |
| AC-4 | the change is built | `make ze-tier-check` passes (exit 0); no file under `internal/plugins/anomaly` or `internal/component/trafficfeature` imports `internal/plugins/flowexport`; flowexport is not promoted to `component` |
| AC-5 | flowexport is removed / disabled | `internal/core/observation` and `internal/component/trafficfeature` still compile and behave; the AS field defaults to `0` (self-containment preserved) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs flowexport with BGP enrichment on, so flows carry origin-AS | flow enriched (exporter.go) -> `Observation.SrcAS` stamped (exporter.go:~321) -> feed -> `trafficfeature` ingest -> `FeatureEntry.SrcAS` in `Snapshot()` | `TestExportFlowsStampsSrcAS` + `TestFeatureEntryCarriesSrcAS` |
| 2 | runs flowexport without BGP enrichment (no RIB) | flow `SrcAS=0` -> observation `SrcAS=0` -> `FeatureEntry.SrcAS=0` (child 7 will degrade to prefix cohorts) | `TestFeatureEntrySrcASUnsetWhenUnknown` |
| 3 | (child 7, out of scope) sees AS-origin cohorts in `show anomaly detect` | detector reads `fe.SrcAS` from the same Snapshot | owned by `spec-anomaly-7-as-entities-cohorts.md` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExportFlowsStampsSrcAS` | `internal/plugins/flowexport/exporter_test.go` | producer stamps `Observation.SrcAS` from the enriched flow (AC-1) | |
| `TestExportFlowsSrcASZeroWhenNoEnricher` | `internal/plugins/flowexport/exporter_test.go` | with a nil enricher / RIB miss the published observation has `SrcAS == 0` (AC-3) | |
| `TestFeatureEntryCarriesSrcAS` | `internal/component/trafficfeature/feature_test.go` | ingest+tick propagates `SrcAS` to `FeatureEntry` for the source entity (AC-2) | |
| `TestFeatureEntrySrcASUnsetWhenUnknown` | `internal/component/trafficfeature/feature_test.go` | `SrcAS=0` obs yields `FeatureEntry.SrcAS=0` and does not clobber a prior known AS (AC-3, R-2) | |
| `TestFeatureEntrySrcASSourceRoleOnly` | `internal/component/trafficfeature/feature_test.go` | an addr that is the DEST of a flow does not inherit the flow's `SrcAS` on its source entity (A-4) | |
| `TestObservationSrcASFieldZeroValue` | `internal/core/observation/observation_test.go` | default `Observation{}` has `SrcAS == 0`; field survives `Publish`/subscribe copy (A-5) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `SrcAS` (uint32) | 0 .. 4294967295 | 4294967295 | N/A (0 = unknown sentinel, reserved AS0) | N/A (uint32 saturates; no validation, it is a passthrough label) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A for child 6 | -- | child 6 adds an internal facts field with no operator-facing surface of its own. The operator-visible AS surface (AS cohorts / per-ASN entities in `show anomaly detect`) lands in child 7, which owns the functional `.ci`. The end-to-end daemon proof rides child 4's `interop-harness` `fakeflow` once child 7 consumes the field. Child 6 is proven by the unit + wiring tests above and the `ze-tier-check` gate. | N/A |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | -- | -- | no wire protocol change; origin-AS is an in-process value already derived from the BGP RIB by flowexport | N/A |

### Future (if deferring any tests)
- Daemon-level end-to-end assertion of AS on a scored incident is deferred to child 7 + child 4 (the `fakeflow` harness), by design: child 6 has no operator-facing surface to assert on. No user approval needed because no in-scope behavior is deferred.

## Files to Modify
<!-- Check // Design: annotations on each file. -->
- `internal/core/observation/observation.go` - add a `SrcAS uint32` field to the `Observation` struct (observation.go).
- `internal/plugins/flowexport/exporter.go` - in the publish loop (exporter.go), stamp `obs.SrcAS = f.SrcAS`. Design ref: `docs/architecture/flowexport/flow-export-2-flow-records.md`.
- `internal/component/trafficfeature/service.go` - add `SrcAS uint32` to `FeatureEntry` (service.go).
- `internal/component/trafficfeature/feature.go` - add a persistent `srcAS uint32` to `sourceState` (feature.go); stamp it in the SOURCE-role branch of `ingest` only when `obs.SrcAS != 0` (feature.go); copy it into `FeatureEntry.SrcAS` in `snapshot` (feature.go); do NOT clear it in the window reset (feature.go).

### BGP Family Checklist (if new SAFI / capability / attribute)
N/A -- this spec adds no SAFI, capability, or attribute; no wire format changes. (Delete-per-template: not a BGP protocol extension.)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | N/A -- no new config; AS enrichment is governed by flowexport's existing enricher, not a new knob |
| YANG validation constraints | No | N/A -- no new leaf |
| YANG custom validators | No | N/A |
| CLI commands/flags | No | N/A -- no new CLI; `show flow-recent` already prints src-as (cmd_show.go); the AS-on-anomaly CLI is child 7 |
| CLI grammar (action before identifier) | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A -- no new RPC/API; proven by unit + tier-check (child 7 owns the daemon `.ci`) |
| Pipe completeness | No | N/A -- no new command output |
| Env var registration | No | N/A -- no new `environment/` leaf |
| Doctor check for runtime dependencies | No | N/A -- no new file path, socket, port, module, binary, or cert; flowexport already owns doctor checks for its enrichment/RIB feed |
| Prometheus counters/metrics | No | N/A -- passthrough label, no new observable state. (Optional future: a `ze_observation_as_known_total` counter could confirm enrichment is populated; deferred, not in scope) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A -- internal facts plumbing; the user-facing AS feature (cohorts) is child 7 |
| 2 | Config syntax changed? | No | N/A -- no config change |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A -- flowexport gains one field copy, no surface change |
| 6 | Has a user guide page? | No | N/A -- AS cohorts guide lands with child 7 |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A -- no change to `pkg/plugin` or `pkg/ze` |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/meta/` or the observation-feed / traffic-analysis subsystem doc: note that `Observation`/`FeatureEntry` now carry an optional origin-AS stamped by the flowexport producer (data-flow: producer -> core feed -> component global). Add a source anchor to `observation.go` / `feature.go` |
| 13 | Route metadata keys added/changed? | No | N/A |
| 14 | Prometheus counters added/changed? | No | N/A |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | N/A -- no new registration |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | grep `docs/` for anchors pointing at `observation.go`, `feature.go`, `service.go`, `exporter.go`; update any stale struct description |
| 17 | Existing docs show config/CLI/API examples for this area? | No | N/A -- no config/CLI/API example touches these fields |

## Files to Create
- No new source files. All changes are additive fields + one stamp line, tested by additions to existing `_test.go` files (`observation_test.go`, `feature_test.go`, `exporter_test.go`).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan -- confirm the structs/lines still match |
| 3. Wiring phase | Wiring Test table -- add the failing propagation tests first |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` (includes `ze-tier-check`) |
| 7-13 | Critical / Deliverables / Security review below |
| 14. Present summary | Executive Summary + learned summary |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- add the failing propagation + tier assertions
   - Tests: `TestFeatureEntryCarriesSrcAS`, `TestExportFlowsStampsSrcAS` (fail: field does not exist yet)
   - Files: test additions only; confirm `make ze-tier-check` is green BEFORE the change (baseline)
   - Verify: tests fail to compile (no `SrcAS` field) -- proves the surface is not yet wired
2. **Phase: core field** -- add `SrcAS uint32` to `observation.Observation`
   - Tests: `TestObservationSrcASFieldZeroValue`
   - Files: `internal/core/observation/observation.go`
   - Verify: builds; default is 0; feed copy carries it
3. **Phase: producer stamp** -- copy `f.SrcAS` into the observation in the publish loop
   - Tests: `TestExportFlowsStampsSrcAS`, `TestExportFlowsSrcASZeroWhenNoEnricher`
   - Files: `internal/plugins/flowexport/exporter.go`
   - Verify: producer test passes; no second enricher call added
4. **Phase: facts propagation** -- carry AS through `sourceState` to `FeatureEntry`
   - Tests: `TestFeatureEntryCarriesSrcAS`, `TestFeatureEntrySrcASUnsetWhenUnknown`, `TestFeatureEntrySrcASSourceRoleOnly`
   - Files: `internal/component/trafficfeature/service.go`, `internal/component/trafficfeature/feature.go`
   - Verify: source-branch-only stamp; persists across window reset; `Snapshot` carries it
5. **Phase: tier + full verify** -- prove the guardrail holds
   - Tests: `make ze-tier-check` (exit 0); grep proves no `anomaly -> flowexport` import
   - Verify: `make ze-verify` (or `ze-verify-changed`) green
6. **Docs** -- update the subsystem/meta doc (checklist row 12/16) with a source anchor
7. **Complete spec** -- fill audit tables; learned summary to `plan/learned/NNN-anomaly-6-as-enrichment.md`; two commits (A: code+tests+spec+learned; B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 each have implementation + test with file:line |
| Feature completeness | End-to-end story 1 and 2 both have a working, tested path; story 3 correctly deferred to child 7 |
| Correctness | AS stamped ONLY in the source-role ingest branch; persistent (not window-reset); non-zero-write-wins |
| Naming | field is `SrcAS` on both `Observation` and `FeatureEntry` (matches umbrella `fe.SrcAS`); no plugin name leaks into core/component |
| Data flow | stamp at the producer only; consumer (child 7) reads, never fetches; no new import edge |
| Registration over hardcoding | additive fields, no new registry, no per-plugin switch in a core package |
| Doctor checks | none added (no new runtime dependency) -- confirm N/A holds |
| Rule: module-tiers | `make ze-tier-check` green; flowexport not promoted to component |
| Rule: plugin-self-containment | removing flowexport leaves observation/trafficfeature compiling with `SrcAS` defaulting to 0 |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `Observation.SrcAS` exists | `grep -n "SrcAS" internal/core/observation/observation.go` |
| producer stamp present | `grep -n "SrcAS" internal/plugins/flowexport/exporter.go` shows the copy in the publish loop |
| `FeatureEntry.SrcAS` carried to Snapshot | `TestFeatureEntryCarriesSrcAS` passes |
| tier gate green | `make ze-tier-check` exit 0 |
| no forbidden import | `grep -rn "plugins/flowexport" internal/plugins/anomaly internal/component/trafficfeature` returns nothing (excluding test comments) |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `SrcAS` is an untrusted-derived label (from BGP RIB). It is passed through, never used as an index, size, or allocation input, so no injection/overflow risk. Confirm no code uses it to size a slice or map |
| Resource exhaustion | field adds a fixed 4 bytes per `Observation`; no per-observation allocation; confirm no new unbounded map keyed by AS in this spec (per-ASN entities are child 7) |
| Error leakage | none -- no new error path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| `ze-tier-check` fails | A forbidden import slipped in -- remove it; the whole point is to avoid it |
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
- The umbrella framed this as "stamp where the producer calls `enrich.Enricher.Lookup`." The precise producer calls the higher-level `Enrich` wrapper (exporter.go), and crucially the AS is ALREADY on the `ConntrackFlow` struct (`SrcAS`, flowtypes.go) when the observation is built (exporter.go). So the stamp is a pure field copy with **no lookup at all** -- even cheaper than the umbrella implied.
- The "hard dependency on flowexport" (umbrella R-3) is not introduced by AS work: flowexport is already the sole `KindFlow` publisher and `trafficfeature` ingests only `KindFlow`, so the detector's facts already require flowexport. AS is strictly an additive optional label on the existing path.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Stamp `SrcAS` at the flowexport producer onto `observation.Observation` | (a) import `flowexport/enrich` from the detector; (b) a separate AS side-channel feed | (a) fails `ze-tier-check` (`engine_depended`, dep_audit.py); (b) duplicates the feed. Producer-stamp reuses the value already on the flow and adds zero import edges |
| Carry `SrcAS` only (not `DstAS`) in this spec | carry both now | only source entities are emitted (`sent := st.outBytes > 0`, feature.go) and child 7 reads `fe.SrcAS`; `DstAS` is unused until child 5 emits dest entities. No-speculative-features. `DstAS` is a one-line additive follow-up (`ConntrackFlow.DstAS` already exists) |
| Stamp in the SOURCE-role ingest branch only | stamp in both roles | in the DEST branch `obs.SrcAS` is the OTHER endpoint's AS; stamping it would mislabel the entity (A-4). The source branch sees the entity-as-source, where `obs.SrcAS` is correct |
| Persist `srcAS` across window reset; overwrite only on non-zero | reset per window; always overwrite | AS is an entity property (its prefix), stable across windows; a later unknown-AS flow (RIB miss) must not clobber a known AS (R-2) |
| `SrcAS == 0` is the "unknown" sentinel | a separate `HasAS bool` | AS0 is reserved (RFC 7607) and never announced, so 0 is unambiguous; matches the existing `ConntrackFlow.SrcAS` "0 if unknown" convention and enables child 7's degrade-to-prefix cleanly |

## Known Limitations
- Carries source origin-AS only. Destination-AS (`DstAS`) is deferred to child 5 (`entity-matrix`), which introduces dest entities; the value already exists on the flow, so it is a one-field additive change when needed.
- No operator-facing surface in child 6. The AS becomes visible to operators only when child 7 scores AS-origin cohorts / per-ASN entities in `show anomaly detect`.
- No new metric to confirm AS coverage. `show flow-recent` (cmd_show.go) already exposes src-as as an indirect signal that enrichment is working; a dedicated coverage counter is a possible future add.

## Core Insight
The tier boundary is enforced by making the seam unnecessary, not by adding a shared package: the
producer already holds the AS, so it stamps it onto the core feed value it already publishes.
The consumer reads a field it already receives. No plugin imports another; the gate stays green
by construction, not by exception.

## Implementation Summary
### What Was Implemented
- (filled at implementation)
### Bugs Found/Fixed
- (filled at implementation)
### Documentation Updates
- (filled at implementation)
### Deviations from Plan
- (filled at implementation)

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
| Origin-AS stamped at the flowexport producer onto the facts surface | unit test | `TestExportFlowsStampsSrcAS`, `TestFeatureEntryCarriesSrcAS` |
| Tier-safe (no `detect -> flowexport` import) | gate run | `make ze-tier-check` exit 0 + grep for absent import |
| Optional: unset when AS unknown, degrades to prefix cohorts | unit test | `TestFeatureEntrySrcASUnsetWhenUnknown` |

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
| Entry Point | Test | Verified |
|-------------|------|----------|
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: story 1 and 2 have a working path + passing test; story 3 documented as child 7
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests, incl. `ze-tier-check`)
- [ ] Feature code integrated (`internal/core/observation`, `internal/plugins/flowexport`, `internal/component/trafficfeature`)
- [ ] Integration completeness proven end-to-end (facts surface carries `SrcAS`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture/meta doc updated (row 12/16) with a source anchor
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (N/A -- no RFC behavior)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (additive fields, no new package)
- [ ] No speculative features (`SrcAS` only; `DstAS` deferred to child 5)
- [ ] Single responsibility (facts layer carries a label; judgment stays in the detector)
- [ ] Explicit > implicit behavior (0 = unknown sentinel, documented)
- [ ] Minimal coupling (zero new import edges)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for numeric inputs (`SrcAS`)
- [ ] Functional tests (N/A for child 6 -- justified; child 7 owns the daemon `.ci`)
- [ ] Interop tests (N/A -- no wire change)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks documented
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-anomaly-6-as-enrichment.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-anomaly-6-as-enrichment.md` only
