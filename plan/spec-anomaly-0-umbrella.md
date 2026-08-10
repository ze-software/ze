# Spec: Behavioral Security Anomaly Detection (Umbrella)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | 1046 (traffic-analysis), 1048 (anomaly-1-detect), 1049 (anomaly-2-shape) |
| Phase | - |
| Updated | 2026-07-02 |

## Correction 2026-08-03 (bookkeeping audit): the announce seam EXISTS

**Every statement in this spec that the plugin→reactor announce seam "does not exist"
or "is missing" is superseded. Read this block before you act on AC-6, on the child-8
roadmap row, or on the Key-insight bullet about `cp-survival-4`.**

Producers read on 2026-08-03, not inferred:

| Producer | Where | What it does |
|----------|-------|--------------|
| `sdkDispatcher.Dispatch` | `internal/plugins/ddos/flowspec/register.go` | calls `sdk.Plugin.UpdateRoute` with the rendered update text and the FlowSpec peer selector |
| `Plugin.UpdateRoute` -> `UpdateRouteWithMeta` | `pkg/plugin/sdk/sdk_engine.go` | sends `rpc.MethodUpdateRoute` to the engine and returns the announced and withdrawn counts |
| `Server.opUpdateRoute` -> `handleUpdateRouteSelDirect` | `internal/component/plugin/server/dispatch_registry.go`, `internal/component/plugin/server/dispatch.go` | the engine side that turns that call into a route operation |

So a system plugin CAN reach the reactor announce path today, and one does in production.
`UpdateRouteWithMeta`'s own doc comment records that plugin-originated routes go through
`AnnounceNLRIBatch`.

FlowSpec origination is NOT a stub either. `responder` (`internal/plugins/ddos/flowspec/responder.go`)
calls `dispatcher.Dispatch` on both the announce and the withdraw path, and `setAnnouncement`
publishes the resulting state for `show ddos flowspec`.

**What is still true, and it is a smaller thing:** `internal/plugins/anomaly/shape/register.go`
registers with `ConfigureEventBus` and holds only `ze.EventBus`, so `shape` as registered
carries no dispatcher. Child 8's prerequisite is therefore to give `shape` the seam that
already exists, following `ddos/flowspec`. It is not to build a seam. That is wiring work
inside one plugin, not a new cross-component mechanism, and `cp-survival-4` is not a blocker
on it.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now), starting at the Correction block above
2. `.claude/rules/planning.md` - workflow rules
3. The three closed foundations: `docs/architecture/traffic/traffic-analysis-layers.md`,
   `docs/architecture/anomaly/anomaly-1-detect.md`, `docs/architecture/anomaly/anomaly-2-shape.md`
4. The child specs listed in the Child Spec Roadmap table (written per-session as work begins)

## Task

Ze has a working Darktrace-style behavioral security spine: neutral traffic **facts** →
per-entity anomaly **judgment** → shadow-first surgical **response**, kept as a domain
strictly SEPARATE from the volumetric DDoS family (no shared event contract, detector, or
responder). Three specs shipped it end to end:

- **1046** `internal/core/stats` + `trafficstat` rebuilt on it + neutral `trafficfeature`
  (the FACTS layer: fan-out, byte-ratio, entropy, beaconing, new-peer, rare-port).
- **1048** `internal/core/anomalyevent` + `anomaly/detect` (the JUDGMENT layer: per-entity
  EWMA baseline, cohort rarity, capped correlation combine, freeze-learn + warmup; report-only).
- **1049** `anomaly/shape` (the RESPONSE layer: shadow-first firewall responder with
  auto-revert, blast-radius cap, kill-switch, allowlist).

This umbrella frames the initiative and coordinates the **follow-on child specs** that widen
and harden the spine. It owns the shared framing (the fact/judgment/response dividing line,
the separate-from-DDoS invariant) and the roadmap; each child owns its own implementation,
`/ze-review` gate, and learned summary. The umbrella itself ships **no production code** — its
deliverable is the roadmap plus a `docs/features.md` roll-up row and an operator guide once
at least the Phase-A children land.

The dividing principle (inherited from 1046, held across every child): traffic analysis
computes neutral FACTS (measurable, domain-agnostic numbers); detection plugins apply
JUDGMENT (is this a threat) and own RESPONSE. A number both DDoS and security want is a fact;
a verdict or an action is a plugin.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component/plugin registration model every child plugs into
  → Constraint: children register via `init()` in `register.go`; core discovers via registries, never imports plugins directly. Each child self-contains (`ai/rules/plugins.md`) — remove it and all its CLI/schema/doctor surface vanishes.
- [ ] `docs/architecture/traffic/traffic-analysis-layers.md` - the FACTS layer contract
  → Constraint: `trafficfeature.Snapshot()` is the neutral fact surface; children consume it, never re-measure. The facts layer holds no verdict (severity moved out to detection).
- [ ] `docs/architecture/anomaly/anomaly-1-detect.md` - the JUDGMENT layer contract
  → Constraint: `anomalyevent` namespace `anomaly-detect`, keyed on a source `netip.Prefix`; scoring is pure in `score.go`; freeze-learn (`scoreEntity` returns pending `baselineUpdate`s, `onTick` folds them only when not-anomalous or still warming). Any new entity type or feature must preserve freeze-learn + warmup.
- [ ] `docs/architecture/anomaly/anomaly-2-shape.md` - the RESPONSE layer contract
  → Constraint: responder owns one mutex, whole-owner firewall re-register under key `anomaly-shape`, responder-level monotonic generation guard, timed auto-revert, blast-radius cap, kill-switch, allowlist. `registerTables`/`applyAll` are mockable package vars for unit testing without a kernel.

### Reference Implementations (grounding, not doc)
- [ ] `internal/plugins/ddos/observe/{store.go,register.go,store_test.go}` - the incident-store SKELETON Phase-A child `observe` reuses (registration, ring, subscribe, open/finalize). NOT its surface: `list()` is test-wired only (`store.go`), there is no show handler or web card.
- [ ] `internal/test/plugins/fakeredist/` - the test-only-plugin template Phase-A child `interop-harness` copies for `fakeflow` (publishes into a core surface in-process; loaded only in the `zetest` DUT build via `cmd/ze/plugins_zetest.go`).
- [ ] `internal/plugins/flowexport/enrich/enricher.go` - `Lookup(addr) (ASEntry, bool)` — the AS-origin source. It stays INSIDE flowexport; child 6 stamps AS onto the facts surface (a direct import from `detect` fails `ze-tier-check`, `dep_audit.py`).
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `AnnounceNLRIBatch(sel, batch)` — the family-agnostic announce API. Reachable via `ctx.Reactor()` in a BGP command handler, and ~~NOT from a system plugin's `ze.EventBus`; child 8 needs a plugin→reactor seam first.~~ **(corrected 2026-08-03)** from a system plugin through `sdk.Plugin.UpdateRoute`, which `ddos/flowspec` uses in production. See the Correction block.

**Key insights:**
- The spine is complete but **narrow**: one entity type (source-IP prefix), one detector, one
  local responder, 1-second granularity, no persisted incident history. Every child widens
  exactly one axis and must not regress the invariants above.
- The facts pipeline is uniformly **1-second** (`trafficstat/service.go`,
  `trafficfeature/service.go`, `anomaly/detect/register.go`). Any sub-second signal
  (beaconing at second-fraction periods) is blocked on a new sub-second collector — it cannot
  be faked at the detector.
- The FlowSpec **wire codec** works (SAFI 133/134 + traffic-action communities are wire-tested,
  `test/encode/flow.ci`) and `AnnounceNLRIBatch` is family-agnostic — so upstream response needs
  **no new BGP-family work**. ~~But the plugin→reactor **seam** is missing: `shape` holds only
  `ze.EventBus` (Emit/Subscribe), FlowSpec origination is stubbed (`ddos/flowspec/responder.go`,
  "cp-survival-4 not yet wired"). Child 8 is blocked on that seam;~~ **Superseded 2026-08-03
  (Correction block): the seam exists and `ddos/flowspec` originates through it. Only `shape`
  is unwired.** Classic RTBH is the shortcut.
- Cross-plugin data in ze flows **producer → core feed → component global → consumer**, never
  plugin→plugin (a direct import fails `ze-tier-check`). So origin-AS for cohorts must ride the
  `observation`/`trafficfeature` surface, stamped by flowexport — not fetched by the detector.

## Current Behavior (MANDATORY)

**Source files read (shared grounding; per-child detail lives in the child specs):**
- [ ] `internal/component/trafficstat/service.go` - aggregator ticks `tickInterval = time.Second` (service.go,269), subscribes `observation.Feed`, exposes `Snapshot()`.
  → Constraint: 1s is the fundamental cadence of the whole facts→judgment chain.
- [ ] `internal/component/trafficfeature/service.go` - neutral feature service, 1s ticker (service.go,187), `Snapshot()` surface consumed by the detector.
- [ ] `internal/plugins/anomaly/detect/register.go` - detector runs its own `time.NewTicker(time.Second)` (register.go) then `d.onTick(svc.Snapshot())`, scores, emits `anomaly-detect` events + a bounded in-memory recent-incident ring surfaced by `show anomaly detect`.
  → Constraint: the ring is in-memory only — no persistence, no query-by-time. Phase-A `observe` adds that.
- [ ] `internal/plugins/anomaly/shape/responder.go` - `registerTables = firewall.RegisterTables`, `applyAll = firewall.ApplyAll` (responder.go); `match.go buildActions` builds `firewall.Action`s. Responder action is **local firewall only** — no upstream/flowspec path (grep confirms none).
- [ ] `internal/plugins/flowexport/enrich/enricher.go` - `Lookup(addr netip.Addr) (ASEntry, bool)` (enricher.go) returns origin-AS enrichment; lives in the flowexport plugin (cross-plugin access needed for AS cohorts).
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `AnnounceNLRIBatch` (reactor_api_batch.go) announces NLRI (incl. FlowSpec SAFI 133) at runtime today.
- [ ] `internal/plugins/anomaly/detect/yang/ze-anomaly-detect-conf.yang` - config is NESTED (revision 2026-07-02): `container anomaly { container detect { leaf enabled ... } }` (yang lines 13-69); the detect module OWNS the `anomaly` parent. `internal/plugins/anomaly/shape/yang/ze-anomaly-shape-conf.yang` AUGMENTS `/ad:anomaly` with `container shape` (yang lines 13-14). The `ddos` family mirrors this (`ddos { detect local flowspec flowtriq observe }`, detect owns + siblings augment `/dd:ddos`).
  → Constraint: the operator-facing config is `anomaly { detect {…} shape {…} }`, NOT flat `anomaly-detect {}`. The `observe` child (child 3) MUST augment `/ad:anomaly` with `container observe`, exactly as `ddos/observe` augments `/dd:ddos`. The show commands are `show anomaly detect` / `show anomaly shape` (the wire methods `ze-show:anomaly` / `ze-show:anomaly-shape` are unchanged internally).

**Existing behavior (do NOT regress):**
- The fact/judgment/response separation and the anomaly-vs-DDoS domain split (separate event
  contract, namespace, firewall owner key `anomaly-shape`).
- Freeze-learn + warmup in the detector; the pure scoring rule in `score.go`; leave-one-out
  cohort rarity with `+Inf` exclusion.
- Shadow-first responder default; auto-revert; blast-radius cap; kill-switch; allowlist.
- Metrics `ze_anomaly_incidents_total`, `ze_anomaly_active`, `ze_anomaly_tracked_entities`;
  doctor `anomaly-detect-feature-source`.

**Behavior to change:** None directly in the umbrella. Each child adds new, opt-in behavior.

## Data Flow (MANDATORY)

### Entry Point
- Neutral traffic observations (`observation.Feed`) → facts → per-entity judgment → response.
  Operator intent enters via config (`anomaly { detect { enabled } }`, `anomaly { shape {...} }`) and
  `show anomaly detect` / `show anomaly shape`.

### Transformation Path (target end-state across the children)
1. **Facts (1046, shipped):** `observation.Feed.Publish` → `trafficfeature.ingest` → `Snapshot()` neutral signals. (`trafficstat` is a **sibling** consumer of the same feed, not a stage upstream of `trafficfeature`.)
   - **widen (Phase B):** `trafficfeature` gains per-dest / per-port `FeatureEntry` lists (child 5) and an origin-AS field stamped by the flowexport producer (child 6) — the facts, not the detector, are where the work lives.
2. **Judgment (1048, shipped):** detector baselines each entity, scores deviation + cohort rarity, emits `anomaly-detect` events.
   - **widen (Phase B):** detector re-keys onto the new dest/port/ASN `FeatureEntry` lists; AS-origin cohorts read `fe.SrcAS` (child 7).
3. **Persist (Phase A `observe`):** events land in an incident **lifecycle** store (open→finalize) with a NEW `show anomaly observe` query surface — the lifecycle the detect ring (Detected-only) lacks.
4. **Response (1049, shipped):** the `shape` responder subscribes to events, installs shadow-first local firewall terms.
   - **widen (Phase C, ~~blocked~~ ready):** an upstream FlowSpec/RTBH action gated by the same state machine — ~~requires a plugin→reactor announce seam that does not exist yet~~ requires wiring `shape` to the existing `sdk.Plugin.UpdateRoute` seam (`shape` holds only `ze.EventBus`). Corrected 2026-08-03, see the Correction block.
5. **Explain (Phase D horizon):** correlate related events into an incident narrative via `aihelp`/`mcp`.
6. **Prove (Phase A `interop-harness`):** a test-only `fakeflow` plugin `Publish`es synthetic observations to drive the whole chain end to end in one functional test.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| trafficfeature ↔ detect | `Snapshot()` read on detector tick | [x] (1048) |
| detect ↔ shape | `anomaly-detect` typed events | [x] (1049) |
| detect ↔ observe (Phase A) | subscribe events → store | [ ] |
| flowexport enrich ↔ detect (Phase B) | `enrich.Enricher.Lookup` cross-plugin read | [ ] |
| shape ↔ reactor (Phase C) | `AnnounceNLRIBatch` → wire | [ ] |

### Integration Points
- `trafficfeature.Snapshot()` - facts surface (shipped)
- `anomalyevent` typed events - judgment surface (shipped)
- `internal/plugins/ddos/observe` - store pattern to mirror (Phase A)
- `internal/plugins/flowexport/enrich` - AS-origin source (Phase B)
- `reactor_api_batch.AnnounceNLRIBatch` - announce seam (Phase C)

### Architectural Verification
- [ ] No bypassed layers (each child uses the facts→judgment→response path, never re-measures)
- [ ] No unintended coupling (children independent; no child imports another; anomaly never imports ddos)
- [ ] No duplicated functionality (observe mirrors ddos/observe; upstream reuses AnnounceNLRIBatch)
- [ ] Registration over hardcoding (every new command/store/action registers; no per-child switch in a core package)

## Child Spec Roadmap (the heart of this umbrella)

Phase-local ordering; actual spec filenames continue the `anomaly-N` series (1=detect, 2=shape
are closed). "Blocked" = a prerequisite must land first and cannot be faked.

Classification below is **verified against the code** (five research passes, 2026-07-02); the
Assumptions table records the evidence. "Blocked" = a prerequisite must land first and cannot
be faked at the anomaly layer.

| Child | Spec file | Phase | Status | Depends | Note (verified) |
|-------|-----------|-------|--------|---------|------|
| detect | (closed, learned 1048) | — | **done** | 1046 | judgment layer |
| shape | (closed, learned 1049) | — | **done** | 1048 | response layer |
| observe | `spec-anomaly-3-observe.md` | A | drafted | 1048 | incident **lifecycle** store (open→finalize/EndTime the detect ring lacks) + a NEW `show anomaly observe` query surface. `ddos/observe` NOW has a live show surface (`show.go`, `s.list()` at `:60`), so it is a fuller template than A-2 claimed; the real divergences are source-prefix key vs dest-tuple, `anomalyevent` vs `ddosevent`, single-node show, and wiring the still-dead `sweepStale` ticker (`store_test.go` its only caller). In-memory ring (no durable store today; no web card). |
| interop-harness | (closed, learned 1054) | A | **done** | 1046,1048,1049 | ~~build a test-only `fakeflow` plugin (copy `internal/test/plugins/fakeredist`)~~ (superseded at implementation, see learned 1054: the `fakeflow` plugin approach was abandoned for an in-process Go integration test; `internal/test/plugins/fakeflow` does not exist). Row updated 2026-07-22 during plan review. |
| entity-matrix | `spec-anomaly-5-entity-matrix.md` | B | drafted | 1048 | generalize the entity axis to **dest** and **port**. Mostly a FACTS-layer (`trafficfeature`) change — dest carries only `inBytes` today, port is a per-source histogram; the detector re-key is the smaller half. Prefix cohort transfers to dest; port has no natural cohort. Excludes ASN (see below). **Also (not in the original framing, verified): an additive `EntityKind` discriminator on the anomaly event (`Entity` is `netip.Prefix`, `event.go`) + a source-only guard in the `shape` responder (`responder.go`, `match.go`) so dest/port incidents stay report-only and never filter the victim. Child 5 owns this contract change; child 7 reuses it.** |
| as-enrichment | `spec-anomaly-6-as-enrichment.md` | B | ready | 1046 | **prerequisite for all AS work.** Stamp origin-AS onto the core `observation.Observation` / `trafficfeature.FeatureEntry` surface at the flowexport producer (which already owns `enrich.Enricher`). Tier-safe; a direct `detect → flowexport/enrich` import is forbidden (fails `ze-tier-check`). |
| as-entities-cohorts | `spec-anomaly-7-as-entities-cohorts.md` | B | drafted | 5, 6 | per-ASN entities + AS-origin cohort rarity, reading `fe.SrcAS` (zero new imports). AS-origin cohort is a cohort-key swap only (reuses `score.go`, keeps a source-prefix `Entity`, stays actionable). Per-ASN **entities** reuse child 5's `EntityKind` discriminator and stay report-only. Degrades to prefix cohorts when flowexport/AS is absent (`SrcAS==0`). |
| upstream-response | `spec-anomaly-8-upstream-response.md` | C | ~~**blocked**~~ **ready (2026-08-03)** | ~~plugin→reactor announce seam (cp-survival-4)~~ wire `shape` to the existing `sdk.Plugin.UpdateRoute` seam | no new BGP-family work (FlowSpec codec + traffic-action communities are wire-tested). ~~BUT `shape` holds only `ze.EventBus` (Emit/Subscribe) and can't reach `AnnounceNLRIBatch`; FlowSpec origination is a stub (`ddos/flowspec/responder.go`).~~ **Superseded 2026-08-03, see the Correction block: FlowSpec origination is live and `ddos/flowspec/register.go` shows the pattern to copy.** Classic RTBH is the shortcut. |
| subsecond-beaconing | `spec-anomaly-9-subsecond-beaconing.md` | C | **blocked** | new sub-second collector spec | facts pipeline is 1s end to end; needs a sub-second timing collector first. |
| ai-analyst | `spec-anomaly-10-ai-analyst.md` | D | horizon | 3 (observe), aihelp/mcp | correlate events into an incident narrative; exploratory, scope TBD. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc) | If wrong | Validated by | Status |
|----|-----------|------------------|----------|--------------|--------|
| A-1 | The children are independent and commit separately | subsystem split | umbrella sequencing wrong | each child compiles + tests green alone | **partially** — 5,6,7 are chained (7 needs 6 needs the facts surface 5 touches); A/C children are independent |
| A-2 | `ddos/observe` is a faithful template for `anomaly/observe` | `ddos/observe/store.go` | observe child grows unplanned scope | read store.go + show.go (child-3 research) | **corrected (was partially)**: skeleton transfers AND the template NOW has a live query surface (`show.go` registers `ze-show:ddos-status`/`ze-show:ddos-incidents`; `handleShowDdosIncidents` calls `s.list()` at `show.go`), so the earlier "no query surface" note is stale (see Mistake Log). Remaining divergences: dest-tuple incident (`store.go`) vs source-prefix (`event.go`), `ddosevent` vs `anomalyevent`, single-node show, still-dead `sweepStale` ticker (`store_test.go`) |
| A-3 | The flowexport enricher is reachable from the anomaly domain without a layering violation | `enrich/enricher.go` in a sibling plugin | as-cohorts needs a new shared seam | `architecture.md` + `dep_audit.py` (agent 4) | **broken** — a direct `detect → flowexport/enrich` import fails `make ze-tier-check` (`dep_audit.py`: flowexport is an engine, the import flips `engine_depended`). Sanctioned path: stamp AS onto the core `observation`/`trafficfeature` surface at the flowexport producer → child 6 |
| A-4 | Upstream FlowSpec response needs only an origination seam + `shape` action, not new BGP family work | `AnnounceNLRIBatch` (reactor_api_batch.go) | child 7 grows scope | agent 5 (announce path + reachability) | **partially** — "no new BGP family" CONFIRMED (`test/encode/flow.ci` proves the codec + traffic-action communities); ~~but the plugin→reactor seam does NOT exist (`ze.EventBus` is Emit/Subscribe only, `eventbus.go`) and FlowSpec origination is stubbed (`ddos/flowspec/responder.go`). RTBH is the shortcut once the seam lands~~ **re-checked 2026-08-03: A-4 is now CONFIRMED in full. The seam exists (`sdk.Plugin.UpdateRoute`) and FlowSpec origination is live (`ddos/flowspec/responder.go` dispatches announce and withdraw). See the Correction block** |
| A-5 | Sub-second beaconing genuinely requires a new collector (cannot derive from 1s facts) | three 1s tickers | child 9 could be unblocked cheaply | `observation.Observation` has no sub-second aggregate seam; pipeline is 1s (agent 2/3) | **confirmed** |
| A-6 | The e2e harness is feasible with existing seams | — | Phase-A gate slips | agent 2 (injection seam) | **confirmed with caveat** — no seam lets a black-box `.ci` inject features; needs a small test-only `fakeflow` plugin (fakeredist template) to `Publish` into `observation.Feed` |
| A-7 | Children 5 (dest/port) and 7 (per-ASN entities) can emit incidents on the existing event contract + responder unchanged | umbrella framed both as facts/detector work | dest incident filters the victim; port/ASN cannot be a `netip.Prefix` | read `event.go`, `responder.go`, `match.go` (child-5/7 research) | **broken**: `Entity` is `netip.Prefix` (`event.go`) so a port/ASN needs an additive `EntityKind` tag; the responder acts on `e.Entity` as a source with no guard (`responder.go,90,102`) so a dest incident throttles the victim and an invalid prefix poisons `r.armed`. Fix owned by child 5 (additive discriminator + source-only guard), reused by child 7; non-source incidents stay report-only |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Entity-matrix (child 5) explodes tracked-entity memory (per-source + per-dest + per-port) | `ze_anomaly_tracked_entities` climbs; eviction churn | the 10000 `maxTrackedKey` cap exists per map (`detector.go`, `feature.go`); child 5 decides per-dimension vs shared cap and sizes the ceiling |
| R-2 | Upstream response (child 8) leaves a stale FlowSpec/RTBH announced after the anomaly clears | upstream keeps filtering | reuse the responder auto-revert + explicit withdraw; the state machine already owns timed revert |
| R-3 | AS work (children 6, 7) creates a hard dependency from the security domain onto flowexport availability | anomaly stops scoring when flowexport is disabled | AS enrichment is optional — child 7 degrades to prefix cohorts when `fe.SrcAS` is unset |
| R-4 | Doing children out of order dilutes review focus / builds on unproven ground | review churn; harness fails late | implement in the ranked order; harness (child 4) lands before Phase B widening so regressions surface early |
| R-5 | AI-analyst (child 10) scope-creeps into an ML project | endless spec | keep D a bounded correlation/narrative layer over stored events; no model training in-tree |
| R-6 | Interop-harness (child 4) `.ci` flakes on wall-clock: incident needs warmup (3 ticks) + `ConfirmDuration` consecutive 1s ticks → ~10-15s real time | `.ci` times out at 15s or fires intermittently | raise the `.ci` timeout well above 15s; inject a sustained cohort+outlier; poll `show anomaly detect` until `incidents>0` rather than fixed sleep |
| R-7 | Entity-matrix (child 5) underscoped as a "detector re-key" when it is mostly new FACTS in `trafficfeature` (dest carries only `inBytes`; port is a per-source histogram) | child 5 spec estimates too low; dest/port score all-zero | child 5 budgets the work in `trafficfeature/feature.go` (new keyed `FeatureEntry` lists), not the detector |

## Wiring Test (MANDATORY)

The umbrella has no executable feature of its own; its "wiring" is that each child's wiring
test passes and the roadmap stays truthful.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Each child spec closed | → | child wiring tests | child `/ze-review` gates + functional tests (see children) |
| `docs/features.md` behavioral-anomaly row exists & source-anchored | → | feature roll-up | `make ze-doc-test` + grep for the source anchor |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Phase-A child `observe` complete | `spec-anomaly-3-observe.md` closed: incident **lifecycle** store (open→finalize with EndTime/Active) + a NEW `show anomaly observe` query surface the detect ring lacks — not a bare mirror of `ddos/observe` |
| AC-2 | Phase-A child `interop-harness` complete | `spec-anomaly-4-interop-harness.md` closed (learned 1054). ~~a test-only `fakeflow` plugin + one `.ci`~~ Delivered as an in-process Go integration test instead of the planned `fakeflow` plugin (see learned 1054); the chain facts→judgment→response is proven end to end. Annotated 2026-07-22 during plan review |
| AC-3 | Phase-B child `entity-matrix` complete | `spec-anomaly-5-entity-matrix.md` closed: `trafficfeature` emits per-dest and per-port `FeatureEntry` lists and the detector scores them, preserving freeze-learn + warmup (prefix cohort for dest; port cohort-free) |
| AC-4 | Phase-B child `as-enrichment` complete | `spec-anomaly-6-as-enrichment.md` closed: origin-AS is stamped onto the `observation`/`trafficfeature` surface at the flowexport producer, passing `make ze-tier-check` (no `detect → flowexport/enrich` import) |
| AC-5 | Phase-B child `as-entities-cohorts` complete | `spec-anomaly-7-as-entities-cohorts.md` closed: per-ASN entities + AS-origin cohort rarity read `fe.SrcAS`, degrading to prefix cohorts when AS is absent |
| AC-6 | Phase-C child `upstream-response` | ~~`spec-anomaly-8-upstream-response.md` remains blocked and documents its prerequisite (the plugin→reactor announce seam / cp-survival-4); if the seam lands,~~ **superseded 2026-08-03, see the Correction block: the seam exists (`sdk.Plugin.UpdateRoute`) and `ddos/flowspec` uses it, so the prerequisite is to wire `shape` to it.** A classic-RTBH action fires under the responder state machine with auto-revert |
| AC-7 | Phase-C child `subsecond-beaconing` | `spec-anomaly-9-subsecond-beaconing.md` remains blocked and documents its prerequisite (a sub-second collector spec); NOT implemented against the 1s pipeline |
| AC-8 | Operator reads docs | `docs/features.md` carries a source-anchored "behavioral security anomaly detection" row; an operator guide covers detect → observe → shadow → arm once Phase A lands |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables `anomaly { detect { enabled true } }`, watches an incident appear and persist | facts → judgment → observe store → `show anomaly observe` | child 3 functional test + child 4 harness |
| 2 | arms `anomaly { shape { mode armed } }`, sees a shadow entry become an armed local firewall term, then auto-revert | judgment event → responder → firewall → revert timer | child 4 harness (existing responder tests cover the state machine) |
| 3 | (Phase C, blocked) arms an upstream action, sees an RTBH/FlowSpec announced then withdrawn | responder → announce seam → `AnnounceNLRIBatch` → wire → withdraw | child 8 functional + interop test (once the seam exists) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none — umbrella owns no executable code) | n/a | coverage lives in the children | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (per child) | (per child) | see child specs | |
| `anomaly-doc` | `make ze-doc-test` | features row + operator guide build and source anchors resolve | |

### Interop Tests
Owned by child 7 (upstream FlowSpec touches the wire). N/A for the umbrella itself.

## Files to Modify
- `docs/features.md` - add "behavioral security anomaly detection" feature row (source-anchored)
- `ai/INDEX.md` - the anomaly row already points at learned 1046/1048/1049; extend to the children as they land

## Files to Create
- The child spec files listed in the Child Spec Roadmap (written per-session as work begins)
- `docs/guide/anomaly-detection.md` - operator guide (detect → observe → shadow → arm), after Phase A lands

## Implementation Steps

The umbrella is closed by closing its children (Phase A + B; Phase C child 7; child 8 stays
blocked with a documented prerequisite; child 9 is horizon) and writing the operator guide.
Recommended order (harden, then widen, then extend):

1. **child 3 `observe`** — smallest; adds incident lifecycle + `show anomaly observe`. Gives operators history now.
2. **child 4 `interop-harness`** — builds the `fakeflow` injector and proves the whole chain through the daemon BEFORE widening it. Gate for Phase B.
3. **child 5 `entity-matrix`** — FACTS-layer work: `trafficfeature` emits per-dest/per-port `FeatureEntry` lists + the detector re-key. Biggest lever.
4. **child 6 `as-enrichment`** — stamp origin-AS onto the facts surface at the flowexport producer (tier-safe). Prerequisite for AS work.
5. **child 7 `as-entities-cohorts`** — per-ASN entities + AS-origin cohorts reading `fe.SrcAS`; degrade to prefix cohorts when absent.
6. **Operator guide** (AC-8) — after Phase A lands so examples are real.
7. **child 8 `upstream-response`** — ~~only after the plugin→reactor announce seam (cp-survival-4) exists;~~ **corrected 2026-08-03: the seam exists, so the first step is to give `shape` a dispatcher, copying `ddos/flowspec/register.go`.** Start with classic RTBH.
8. **child 9 `subsecond-beaconing`** — only after a sub-second collector spec exists (out of this umbrella's scope).
9. **child 10 `ai-analyst`** — horizon; scope its own umbrella/spec when Phase A/B are stable.

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the relevant child |
| 2. Audit | Per-child Files/Tests |
| 14. Present summary | Roll up child summaries into one umbrella learned summary (`plan/learned/NNN-anomaly-0-umbrella.md`) at final closure |

## Known Limitations
- **Sub-second beaconing is out of reach until a sub-second collector exists** (verified: the
  facts pipeline is 1s end to end at `trafficstat/service.go`, `trafficfeature/service.go`,
  `anomaly/detect/register.go`). Child 9 documents the prerequisite; not implemented against 1s.
- ~~**Upstream response is blocked on a plugin→reactor announce seam, not on BGP capability.** The
  FlowSpec codec is wire-tested, but `shape` holds only `ze.EventBus` (`eventbus.go`) and
  FlowSpec origination is a stub (`ddos/flowspec/responder.go`). Child 8 needs the seam
  (cp-survival-4) first; classic RTBH is the shortcut once it exists.~~
  **Superseded 2026-08-03 (Correction block). The insight that survives: upstream response
  is blocked on neither BGP capability nor a missing seam. The FlowSpec codec is wire-tested
  and the announce seam is `sdk.Plugin.UpdateRoute`, which `ddos/flowspec` already originates
  through. Child 8's one prerequisite is that `shape` still registers with `ze.EventBus` alone
  and must be given a dispatcher.**
- **No plugin→plugin import.** The security domain cannot call `flowexport/enrich` directly — it
  fails `ze-tier-check`. Origin-AS must ride the core `observation`/`trafficfeature` surface,
  stamped by the flowexport producer (child 6). AS work degrades to prefix cohorts when absent (R-3).
- **`observe` is an in-memory ring, not durable storage**, and **no web UI surface exists** for
  ddos or anomaly anywhere in the repo — a web card is greenfield, out of scope unless a child
  adds it explicitly.
- The umbrella ships no production code — its value is coordination and an honest, verified
  blocked/ready classification of the follow-on work.

## Design Insights
- The spine was built narrow on purpose (one entity type, one responder, 1s, no history). The
  path forward is not "more detectors" but widening each axis of the *same* spine while holding
  its invariants (freeze-learn, shadow-first, domain separation). Each child is one axis.
- **The work lives one layer lower than it looks.** Both "more entity types" and "AS cohorts"
  read as detector changes, but the detector already generalizes cleanly; the real cost is in the
  FACTS layer (`trafficfeature`) — new keyed `FeatureEntry` lists for dest/port, an origin-AS
  field stamped by the producer. Budget Phase B in `trafficfeature`, not in `detect`.
- **Verification overturned my own memory in both directions.** I first called upstream FlowSpec
  "blocked", then "ready" after a shallow grep found `AnnounceNLRIBatch`, then the deep read
  settled it: no new BGP family (ready) BUT no plugin→reactor seam (blocked). And the enricher
  that looked reachable is a `ze-tier-check` violation to import. A shallow grep is a hypothesis;
  the producing code + the gate code are the finding.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Upstream FlowSpec response is "ready(dep)" because `AnnounceNLRIBatch` exists | The reactor API is real, but `shape` can't reach it (`ze.EventBus` is Emit/Subscribe only, `eventbus.go`) and FlowSpec origination is a stub (`ddos/flowspec/responder.go`) | deep read of the announce path + plugin reachability (agent 5) | child 8 reclassified back to **blocked** on the cp-survival-4 seam; RTBH noted as the shortcut |
| The flowexport enricher can be imported by `anomaly/detect` for AS cohorts | A direct import fails `make ze-tier-check` — flowexport is an engine and the import flips `engine_depended` (`dep_audit.py`) | read `architecture.md` + the gate code (agent 4) | AS cohorts split into child 6 (stamp AS onto the facts surface, tier-safe) + child 7 (consume it) |
| Entity-matrix is mostly a detector re-key | Only sources carry a feature vector; dest/port/ASN need new FACTS in `trafficfeature` (`feature.go`), and ASN needs IP→AS that doesn't exist in the feed (`observation.go`) | read the facts layer (agent 3) | child 5 rescoped to FACTS-layer work; ASN moved behind the AS-enrichment prerequisite |
| `ddos/observe` is a faithful template with a queryable store + web card | It is capture-only: `list()` is test-wired, no show handler, no web card anywhere in the repo | read `ddos/observe` + web grep (agent 1) | child 3 rescoped to add the lifecycle + query surface the template never had |
| `ddos/observe` has no show handler and `list()` is test-only (`store.go`) — earlier finding, now stale | It NOW registers a live show surface (`show.go`) and calls `s.list()` in production (`show.go`); the template evolved since the umbrella was written | child-3 research (2026-07-02) reading `show.go` | A-2 corrected to "corrected"; child 3 justification shifts to source-prefix key + still-dead `sweepStale` ticker; the store skeleton + a show surface both transfer |
| Children 5 and 7 are pure facts/detector work on the existing event contract + responder | Non-source entities (port, ASN) cannot be a `netip.Prefix` `Entity` (`event.go`), and the `shape` responder acts on `e.Entity` as a source with no guard (`responder.go,90,102`) | child-5 and child-7 research (2026-07-02) reading `event.go`, `responder.go`, `match.go` | added A-7; child 5 owns an additive `EntityKind` discriminator + a responder source-only guard; child 7 reuses it, so child 7 truly depends on child 5's contract; dest/port/ASN incidents stay report-only |

## Implementation Audit
(Filled at closure — roll-up of child audits.)

## Review Gate
### Final status
- [ ] Phase-A children (3 observe, 4 interop-harness) closed
- [ ] Phase-B children (5 entity-matrix, 6 as-enrichment, 7 as-entities-cohorts) closed
- [ ] Child 8 (upstream-response) closed ~~OR its plugin→reactor announce-seam dependency documented as the blocker~~ (corrected 2026-08-03: there is no seam blocker to document; see the Correction block)
- [ ] Child 9 (subsecond-beaconing) remains blocked with a documented prerequisite (not implemented against 1s)
- [ ] `docs/features.md` row + operator guide pass `make ze-doc-test`

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 demonstrated (each child closed or documented-blocked + docs)
- [ ] End-to-End User Stories: every non-horizon story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests) after each child
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
<!-- Umbrella owns no executable code; these items are satisfied per-child. -->
- [ ] Tests written (N/A for umbrella — owned by each child spec)
- [ ] Tests FAIL (N/A for umbrella — paste output in each child)
- [ ] Tests PASS (N/A for umbrella — paste output in each child)
- [ ] Boundary tests for all numeric inputs (per child)
- [ ] Functional tests for end-to-end behavior (child 4 harness proves the full chain)
- [ ] Interop tests for protocol features (child 7 upstream FlowSpec; N/A for the umbrella)

### Completion (BLOCKING — before final umbrella closure)
- [ ] All non-blocked children closed
- [ ] Implementation Summary filled (roll-up of child summaries)
- [ ] Write learned summary to `plan/learned/NNN-anomaly-0-umbrella.md`
