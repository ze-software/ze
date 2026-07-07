# Spec: unify-route-events

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `DESIGN-REVIEW.md` finding 2, row "Route-change events" (line 123) -- the concern this spec closes
4. Source files: `internal/component/bgp/redistribute/producer.go` (the lossy bridge), `internal/core/redistevents/events.go` (the pooled winner), `internal/component/bgp/plugins/rib/events/events.go` (the rich RIB event)

## Task

DESIGN-REVIEW finding 2 flags two route-change event types for the same notion, joined by a LOSSY bridge:

- `ribevents.BestChangeBatch` (rich, un-pooled): the BGP RIB best-path change event. `BestChangeEntry` carries 15 fields (Action, Prefix, AddPath, PathID, NextHop, Priority, Metric, ProtocolType, Labels, SRv6SID, OriginAS, ASPath, ECMPNextHops, BackupNextHop, BackupRepairLabels).
- `redistevents.RouteChangeBatch` (lean, pooled via `sync.Pool`): the generic cross-protocol redistribution event. `RouteChangeEntry` carries 5 value-type fields (Action, Prefix, NextHop, Metric, Table).
- The bridge `EmitBestChange` / `convertBestChange` (`internal/component/bgp/redistribute/producer.go:27-66`) maps ONLY Action, Prefix, NextHop and silently drops the other twelve, does not populate the lean type's own Metric or the batch-level OriginASN/Community, and silently discards entries whose action is not add/update/withdraw.

The task is to eliminate the lossy duplication: pick one route-change event as the canonical redistribution event, close the gap between what the bridge carries and what the redistribution consumer needs, and remove the silent-loss behavior that makes this a latent-bug generator. The change is a refactor that preserves all externally observable behavior while making the adapter lossless and drop-free.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `DESIGN-REVIEW.md` (findings 2, 5, 6) - the concern statement and the "protocol-agnostic core carrying protocol-specific shape" / "safety by convention on the hot path" caveats that constrain any change to redistevents
  → Decision: keep `redistevents` a lean value-type leaf; do NOT push rich FIB fields onto it just to satisfy the bridge.
  → Constraint: the batch pool has no refcount and `ReleaseBatch` zeroes entries (finding 6); any new field must be a value type that `clear()` resets and must not be retained past dispatch.
- [ ] `ai/rules/plugin-design.md` (Cross-Boundary Value Types) - why redistevents payloads are value-only
  → Constraint: no string fields, no pointers into another plugin's memory on the cross-boundary payload; a new field must be a fixed-size value type (uint32 is fine, a `[]uint32` per entry is a hot-path allocation to avoid).
- [ ] `internal/core/redistevents/events.go` package doc - the pool lifecycle and read-only-past-dispatch contract
  → Constraint: producer lifecycle is AcquireBatch -> fill -> Emit -> ReleaseBatch; subscribers must treat the payload as read-only and must not retain past dispatch.

**Key insights:**
- The two events are DIFFERENT LAYERS, not accidental duplicates: `BestChangeBatch` feeds the FIB path (via sysrib) and flow enrichment; `RouteChangeBatch` feeds cross-protocol redistribution. The redundancy is only at the bridge, and the fix is to make the bridge lossless, not to merge two structs.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - defines the rich `BestChangeEntry` (15 fields, :52-83) and `BestChangeBatch` (:88-102); un-pooled, emitted via typed `BestChange` handle. ECMPNextHops/BackupNextHop/BackupRepairLabels/FromLocRIB are `json:"-"` in-process-only FIB hints.
  → Constraint: the rich fields (AddPath, PathID, Priority, ProtocolType, Labels, SRv6SID, ECMPNextHops, Backup*) are FIB-programming data consumed by sysrib, not redistribution data.
- [ ] `internal/core/redistevents/events.go` - defines the lean `RouteChangeEntry` (:126-132, 5 value fields) and pooled `RouteChangeBatch` (:142-189, with batch-level OriginASN and Community). `IsReplay` derived from ReplayID.
- [ ] `internal/core/redistevents/pool.go` - `sync.Pool` (:30), `AcquireBatch` (:45) resets header + truncates Entries, `ReleaseBatch` (:68) `clear()`s Entries and drops Community reference.
  → Constraint: `clear(b.Entries)` zeroes every entry field, so a new value-type field on `RouteChangeEntry` is reset automatically -- no pool change needed.
- [ ] `internal/component/bgp/redistribute/producer.go` - the bridge: `EmitBestChange` (:27-46) acquires a pooled batch, sets Protocol/AFI/SAFI, converts each entry via `convertBestChange` (:48-66) which maps ONLY Action/Prefix/NextHop (:61-65), drops everything else including the lean type's own Metric, leaves batch OriginASN/Community zero/nil, and returns `false` (silent drop) for any action other than add/update/withdraw (:58-59).
  → Constraint: this is the single point of loss; the fix lives here plus one field on the payload plus one read in the consumer.
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `publishBestChanges` (:1205-1220) emits `BestChange` to in-process subscribers AND calls `bgpredist.EmitBestChange` (:1219) to feed the bridge; the batch carries the full rich `BestChangeEntry` set at this point.
- [ ] `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - the SOLE consumer of `RouteChangeBatch`. `handleBatch` (:158) reads batch Protocol/AFI/SAFI/OriginASN/Community; `dispatchEntryToConsumer` (:241) reads per-entry Action/Prefix/NextHop and passes batch OriginASN + Community to `InjectRoute`; it does NOT read per-entry Metric. bgp-source batches are dispatched to the OSPF and ISIS consumers (the bgp consumer is skipped because source == consumer, :215).
- [ ] `internal/plugins/flowexport/enrichbgp.go` - direct consumer of `BestChangeBatch` (:107); reads Prefix, Action, NextHop, OriginAS, ASPath to build the prefix-to-AS radix tree.
- [ ] `internal/component/sysrib/sysrib.go` - direct consumer of `BestChangeBatch` (:886); needs the full rich FIB set (ECMPNextHops, BackupNextHop, BackupRepairLabels, Labels, SRv6SID, Priority, Metric, ProtocolType, PathID, AddPath) to program the kernel/VPP FIB and re-emits as `sysribevents.BestChangeBatch`.

**Behavior to preserve:** (unless user explicitly said to change)
- The 8 non-BGP producers (l2tp, connected, static, as112, kernel, isis, ospf, ike/ipsec) keep filling `RouteChangeEntry` exactly as today; batch-level OriginASN (as112 single-ASN virtual router) and Community keep working unchanged.
- sysrib and flowexport keep consuming `ribevents.BestChangeBatch` directly with every rich field intact; no FIB field is moved off `BestChangeBatch`.
- The pooled lifecycle (AcquireBatch -> fill -> Emit -> ReleaseBatch) and the value-type-only payload invariant are preserved.
- Existing functional tests `test/ospf/ospf-redist-bgp.ci`, `test/isis/isis-redist-bgp.ci`, `test/plugin/bgp-redistribute-nexthop-self.ci`, `test/plugin/bgp-redistribute-metrics.ci` pass unchanged.

**Behavior to change:**
- Internal refactor only. The bridge stops losing data: it maps every action (no silent entry drop), populates the existing `RouteChangeEntry.Metric`, and populates a new per-entry `OriginAS`. Externally observable BGP-into-OSPF/ISIS redistribution output is unchanged today (those consumers do not yet propagate origin-AS/metric); the change closes a latent correctness gap, it does not alter current wire output.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A BGP best-path selection completes in the RIB. `publishBestChanges` (`rib_bestchange.go:1205`) builds a `ribevents.BestChangeBatch` of rich `BestChangeEntry` values and calls `bgpredist.EmitBestChange(eb, batch)` (`rib_bestchange.go:1219`).
- Format at entry: rich in-process `*BestChangeBatch` with all 15 per-entry fields populated for the winning path.

### Transformation Path
1. `EmitBestChange` (`producer.go:27`) acquires a pooled `redistevents.RouteChangeBatch`, sets Protocol/AFI/SAFI from the BGP batch.
2. For each rich entry, `convertBestChange` (`producer.go:48`) maps it to a lean `RouteChangeEntry`. TODAY this maps only Action/Prefix/NextHop and returns `false` (silent drop) for unmapped actions. AFTER: it also populates Metric and the new per-entry OriginAS, and never silently drops an entry (unknown action is logged and skipped, counted).
3. `EmitBestChange` emits the filled batch on the local typed `RouteChange` handle bound to ("bgp", "route-change"), then `ReleaseBatch` returns it to the pool.
4. `redistribute-orchestrator` (`redistribute_egress/redistribute.go:151`) receives the batch synchronously, filters by consumer/evaluator, and `dispatchEntryToConsumer` (:241) injects each entry into the OSPF and ISIS consumers. AFTER: it prefers per-entry OriginAS over batch-level OriginASN when nonzero.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP RIB ↔ redistribute bridge | in-process call `EmitBestChange(eb, *BestChangeBatch)` | [ ] |
| bridge ↔ EventBus | typed `RouteChange.Emit(bus, *RouteChangeBatch)` (pooled payload) | [ ] |
| EventBus ↔ orchestrator | typed `Subscribe` per producer name; synchronous fan-out | [ ] |
| orchestrator ↔ OSPF/ISIS consumer | `configredist.RedistConsumer.InjectRoute` / `WithdrawRoute` | [ ] |

### Integration Points
- `redistevents.RouteChangeEntry` -- the shared payload type; gains one value-type field (`OriginAS uint32`).
- `internal/core/redistevents/pool.go` -- `clear(b.Entries)` in `ReleaseBatch` already resets the new value field; `AcquireBatch` truncates Entries; no pool logic change required.
- `configredist.RedistConsumer` -- unchanged interface; the OSPF/ISIS/BGP consumers keep their current registration.

### Architectural Verification
- [ ] No bypassed layers (data flows RIB -> bridge -> EventBus -> orchestrator -> consumer, unchanged)
- [ ] No unintended coupling (redistevents stays a leaf; no import of BGP RIB types; the bridge owns the mapping)
- [ ] No duplicated functionality (extends the existing pooled event; does NOT create a third route-change type)
- [ ] Zero-copy preserved where applicable (new field is a `uint32` value; no allocation, pool reuse intact)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`). Here: the producer/consumer keep using the existing per-protocol typed-handle registration (`events.Register[*RouteChangeBatch](name, EventType)`); no central switch is added.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `redistribute_egress` is the ONLY subscriber of `redistevents.RouteChangeBatch`; sysrib/flowexport consume `BestChangeBatch` directly, not the bridge output | grep of `RouteChange.Subscribe` / `events.Register[*redistevents.RouteChangeBatch]` shows one Subscribe site (`redistribute_egress/redistribute.go:151`) | if another subscriber exists, migrating fields could change its behavior | grep `RouteChange.Subscribe` repo-wide; confirm single consumer | unvalidated |
| A-2 | `BestChangeEntry.OriginAS` and `.Metric` are already populated at the point `EmitBestChange` is called (add/update path) | `rib_bestchange.go:1205-1219` builds the full batch then calls the bridge; `enrichbgp.go` already reads OriginAS from the same batch | if unpopulated at bridge time, the enriched fields would be zero | read `rec.resolve` producer; add a bridge unit test asserting nonzero OriginAS on add | unvalidated |
| A-3 | A new `uint32` field on `RouteChangeEntry` is reset by the existing pool (`clear(b.Entries)` in `ReleaseBatch`, truncate in `AcquireBatch`) | `pool.go:68-87` clears the entries slice; value types are zeroed by `clear` | if not reset, a recycled batch leaks a prior route's origin-AS | pool round-trip unit test asserting OriginAS==0 after Acquire | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | LATENT BUG (present today): the bridge drops per-entry origin-AS, so BGP best-paths redistributed into OSPF/ISIS carry no origin AS. Unobservable now only because OSPF/ISIS external-route generation does not yet propagate `OriginASN`; it surfaces as wrong routes the moment a consumer reads it | a future consumer (or a BGP-into-BGP redistribution path) reads `OriginASN` and gets 0 for BGP-sourced routes | close it now: add per-entry `OriginAS`, populate in the bridge, prefer it in `dispatchEntryToConsumer` |
| R-2 | LATENT BUG (present today): `convertBestChange` returns `false` for any action outside add/update/withdraw (`producer.go:58-59`), silently discarding the entry with no log | a future `RouteAction` enumerant is added and routes vanish from redistribution with no diagnostic | map every current action explicitly; log-and-skip (counted) unknown actions instead of a silent `false` |
| R-3 | The bridge drops the lean type's own `Metric`; populating it is inert until `configredist.RouteEntry` carries Metric to the consumer, so the fix looks like dead code | reviewer flags "Metric set but never read" | document as Known Limitation: populate Metric now (lossless bridge), wire it through `configredist.RouteEntry` to OSPF/ISIS external metric as a scoped follow-up |
| R-4 | Over-enrichment: adding rich FIB fields (Labels/SRv6/ECMP/Backup) to the lean pooled type to "fully unify" would bloat the 8 non-BGP producers' hot path and violate the value-type-only leaf contract | review notes new slice fields per entry | explicitly reject: those fields stay on `BestChangeBatch`; only value-type `OriginAS` (+ existing `Metric`) are added |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP RIB best-path add | → | `EmitBestChange` / `convertBestChange` emits an enriched `RouteChangeBatch` (Metric + OriginAS populated, no silent drop) | `TestBGPProducerBridgeEmitsRouteChange` (`internal/component/bgp/redistribute/producer_test.go:68`) |
| BGP best-paths redistributed into OSPF | → | orchestrator dispatches bridge output to the OSPF consumer | `test/ospf/ospf-redist-bgp.ci` |
| BGP best-paths redistributed into ISIS | → | orchestrator dispatches bridge output to the ISIS consumer | `test/isis/isis-redist-bgp.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `BestChangeEntry` with each of Action = add, update, withdraw | `convertBestChange` maps all three to add/add/remove and returns a valid entry for each; no entry silently dropped |
| AC-2 | A `BestChangeEntry` with a non-add/update/withdraw action (hypothetical future enumerant) | the entry is skipped with a warn log and counted, NOT silently discarded |
| AC-3 | A `BestChangeEntry` on add with a nonzero `Metric` | the emitted `RouteChangeEntry.Metric` equals the source Metric (bridge no longer drops it) |
| AC-4 | A `BestChangeEntry` on add with a nonzero `OriginAS` | `RouteChangeEntry` carries a new `OriginAS` field equal to the source OriginAS |
| AC-5 | An entry with per-entry `OriginAS` nonzero reaches `dispatchEntryToConsumer` | the consumer receives that OriginAS; when per-entry OriginAS is zero it falls back to batch `OriginASN` (as112 behavior preserved) |
| AC-6 | sysrib and flowexport at runtime | both still subscribe to `ribevents.BestChangeBatch` and receive every rich field; no FIB field moved onto redistevents |
| AC-7 | Acquire a pooled batch, fill OriginAS, Release, re-Acquire | the re-acquired batch's entries have `OriginAS == 0` (pool reset intact) |
| AC-8 | `test/ospf/ospf-redist-bgp.ci` and `test/isis/isis-redist-bgp.ci` | pass unchanged (no observable redistribution behavior regression) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPProducerBridgeEmitsRouteChange` | `internal/component/bgp/redistribute/producer_test.go` | bridge emits enriched entry (Metric + OriginAS) on add (AC-3, AC-4) | |
| `TestBGPProducerBridgeMapsAllActions` | `internal/component/bgp/redistribute/producer_test.go` | add/update -> add, withdraw -> remove; no silent drop (AC-1) | |
| `TestBGPProducerBridgeUnknownActionLogged` | `internal/component/bgp/redistribute/producer_test.go` | unmapped action skipped with log, not silently dropped (AC-2) | |
| `TestRouteChangeBatchPoolResetsOriginAS` | `internal/core/redistevents/pool_test.go` | re-acquired batch has OriginAS zeroed (AC-7) | |
| `TestHandleBatchPrefersEntryOriginAS` | `internal/component/bgp/plugins/redistribute_egress/redistribute_test.go` | per-entry OriginAS preferred over batch OriginASN; zero falls back (AC-5) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `RouteChangeEntry.OriginAS` | uint32 (0 = unset / fall back to batch OriginASN) | 4294967295 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-redist-bgp` | `test/ospf/ospf-redist-bgp.ci` | BGP best-paths redistributed into OSPF still announced correctly after the bridge is made lossless | |
| `isis-redist-bgp` | `test/isis/isis-redist-bgp.ci` | BGP best-paths redistributed into ISIS unchanged | |
| `bgp-redistribute-metrics` | `test/plugin/bgp-redistribute-metrics.ci` | redistribution metrics counters unchanged; no user-facing behavior change, existing test suite passes with no regressions | |

### Interop Tests (MANDATORY for protocol features)
Not applicable: this is an internal event-plumbing refactor. It changes no BGP/OSPF/ISIS wire encoding; the existing `test/ospf` and `test/isis` functional scenarios (which drive FRR/BIRD-comparable output) are the behavior guard.

### Future (if deferring any tests)
- Wiring `RouteChangeEntry.Metric` through `configredist.RouteEntry` to the OSPF/ISIS external-route metric is a scoped follow-up (R-3), requires explicit user approval.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/core/redistevents/events.go` - add `OriginAS uint32` to `RouteChangeEntry` (value type, documented as "per-entry origin AS; 0 means fall back to batch OriginASN")
- `internal/component/bgp/redistribute/producer.go` - make `convertBestChange` lossless: map every action (log-and-skip unknown, no silent `false`), populate `Metric` and `OriginAS` from the source `BestChangeEntry`
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - in `dispatchEntryToConsumer`, prefer per-entry `OriginAS` when nonzero, else batch `OriginASN`
- `internal/core/redistevents/pool.go` - verify (no logic change expected) that `AcquireBatch` / `ReleaseBatch` reset the new value field; add a comment noting `clear()` covers it
- `internal/component/bgp/redistribute/producer_test.go` - extend bridge tests (AC-1..AC-4)
- `internal/component/bgp/plugins/redistribute_egress/redistribute_test.go` - add per-entry OriginAS preference test (AC-5)

## Implementation Steps

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — characterize current bridge behavior
   - Tests: `TestBGPProducerBridgeMapsAllActions`, `TestBGPProducerBridgeUnknownActionLogged` written to FAIL against today's silent-drop bridge
   - Files: `internal/component/bgp/redistribute/producer_test.go`
   - Verify: the new tests fail because `convertBestChange` currently drops fields / silently discards unmapped actions
2. **Phase: Enrich the pooled payload** — add the one value field the redistribution consumer needs
   - Tests: `TestRouteChangeBatchPoolResetsOriginAS`
   - Files: `internal/core/redistevents/events.go`, `internal/core/redistevents/pool.go`
   - Verify: field added; pool round-trip zeroes it; the 8 existing producers still compile unchanged (they leave OriginAS zero)
3. **Phase: Make the bridge lossless** — populate Metric + OriginAS, map all actions
   - Tests: `TestBGPProducerBridgeEmitsRouteChange`, `TestBGPProducerBridgeMapsAllActions`, `TestBGPProducerBridgeUnknownActionLogged` now PASS
   - Files: `internal/component/bgp/redistribute/producer.go`
   - Verify: no entry silently dropped; Metric and OriginAS carried; unknown action logged
4. **Phase: Consume the enrichment** — prefer per-entry OriginAS in the orchestrator
   - Tests: `TestHandleBatchPrefersEntryOriginAS`
   - Files: `internal/component/bgp/plugins/redistribute_egress/redistribute.go`
   - Verify: nonzero per-entry OriginAS wins; zero falls back to batch OriginASN (as112 preserved)
5. **Functional tests** → run `test/ospf/ospf-redist-bgp.ci`, `test/isis/isis-redist-bgp.ci`, `test/plugin/bgp-redistribute-metrics.ci`; confirm no regression
6. **Full verification** → `make ze-verify` (lint + all ze tests except fuzz)
7. **Complete spec** → fill audit tables, write learned summary to `plan/learned/NNN-unify-route-events.md`. TWO commits: commit A saves code + tests + spec + learned summary; commit B does `git rm` of the spec.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation with file:line; the bridge maps all actions and carries Metric + OriginAS |
| Feature completeness | The lean pooled event now carries every field the redistribution consumer reads; no field the consumer needs is dropped at the bridge |
| Correctness | add/update -> ActionAdd, withdraw -> ActionRemove; unknown action logged-and-skipped (never silent `false`); OriginAS fall-back order correct |
| Naming | new field named `OriginAS` (matches `BestChangeEntry.OriginAS`); no rename of existing fields |
| Data flow | sysrib/flowexport remain on `BestChangeBatch`; only the bridge output is enriched; redistevents stays a leaf (no BGP import) |
| Registration over hardcoding | producer/consumer keep the existing per-protocol typed-handle registration (`events.Register[*RouteChangeBatch]`); no central switch, no per-feature field on a core/shared struct beyond the one value field on the shared payload. See `ai/rules/plugin-self-containment.md` |
| Rule: no-layering | no third route-change type introduced; the loser is the LOSS in the adapter, deleted -- not a struct |
| Rule: pool safety (finding 6) | new field is value-type, reset by `clear()`, not retained past dispatch |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `OriginAS` on `RouteChangeEntry` | grep `OriginAS` in `internal/core/redistevents/events.go` |
| lossless bridge | read `convertBestChange`; assert no bare `return ..., false` for known actions; Metric + OriginAS assigned |
| consumer prefers per-entry OriginAS | grep `OriginAS` in `redistribute_egress/redistribute.go` |
| no rich FIB field on redistevents | grep confirms redistevents has no Labels/SRv6/ECMP/Backup/PathID fields |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `OriginAS` is a plain uint32 from a trusted in-process producer; no untrusted parsing added |
| Resource exhaustion | no new allocation on the hot path (value field reuses pooled backing array) |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Keep BOTH event types; make the bridge lossless rather than merging structs | (a) migrate sysrib/flowexport onto an enriched redistevents type; (b) emit redistevents directly from the RIB, deleting the bridge | The two events are different layers: `BestChangeBatch` is the RIB best-path change for the FIB (via sysrib) plus flow enrichment, carrying 12 FIB/enrichment-only fields (ECMP/Backup/Labels/SRv6/PathID/Priority/ProtocolType/ASPath) that have no consumer on the redistribution path; `RouteChangeBatch` is the generic cross-protocol redistribution event shared by 8 producers, pooled and value-type-only for the hot path and cross-process delivery. Migrating the FIB consumers onto redistevents would bloat the lean leaf with fields 8 producers never set and violate its value-type contract (the FIB hints are `json:"-"` in-process pointers). So the winner of the "redistribution route-change" notion is `redistevents.RouteChangeBatch`; the loser is the LOSS in the adapter, not the rich RIB type. |
| Enrich with only `OriginAS` (value) plus populate the existing `Metric`; do NOT add the other dropped fields | add all 12 dropped fields to reach parity | Audit of the sole consumer (`redistribute_egress`) shows it reads only Action/Prefix/NextHop per entry and OriginASN/Community per batch. The one field it needs at per-entry granularity that the batch cannot express (BGP entries have per-prefix origin ASes; batch OriginASN is one value) is `OriginAS`. Metric is added because the field already exists and dropping it is gratuitous loss. The FIB fields have no redistribution consumer, so adding them would be speculative. |
| Log-and-skip unknown actions instead of silent `false` | keep silent drop | A silent `return false` on an unmapped action is a latent route-loss with no diagnostic (R-2). Explicit mapping plus a warn on the unexpected default makes future enum additions fail loudly. |

## Known Limitations
- `RouteChangeEntry.Metric` is populated by the bridge but not yet forwarded to consumers: `configredist.RouteEntry` has no Metric field and `dispatchEntryToConsumer` does not pass one. Wiring the metric through to the OSPF/ISIS external-route metric is a scoped follow-up (R-3), out of scope here.
- Per-entry BGP communities are not carried: the current bgp-source consumers (OSPF, ISIS) cannot represent BGP communities, so only the existing batch-level `Community` (used by as112-style single-community virtual routers) is kept. Per-entry community carriage is deferred until a consumer needs it.
- `BestChangeBatch` remains un-pooled (finding 2 notes the pooling asymmetry). Pooling the RIB event is a separate concern from the lossy-bridge concern this spec closes; not addressed here.

## RFC Documentation

Not applicable: no RFC-governed wire behavior changes. Route-change events are internal EventBus payloads.

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

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
| Bridge no longer loses data the consumer needs | unit test | `TestBGPProducerBridgeEmitsRouteChange` asserts Metric + OriginAS carried |
| No silent entry drop | unit test | `TestBGPProducerBridgeUnknownActionLogged` |
| No redistribution behavior regression | functional test | `test/ospf/ospf-redist-bgp.ci`, `test/isis/isis-redist-bgp.ci` pass |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | file:line | |

### Fixes applied

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
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/core/redistevents`, `internal/component/bgp/redistribute`, `internal/component/bgp/plugins/redistribute_egress`)
- [ ] Integration completeness proven end-to-end (functional redistribution tests pass)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (N/A -- no RFC wire behavior)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (only the field a real consumer reads is added)
- [ ] No speculative features (rich FIB fields NOT added to redistevents)
- [ ] Single responsibility per component (bridge owns the mapping)
- [ ] Explicit > implicit behavior (unknown action logged, not silently dropped)
- [ ] Minimal coupling (redistevents stays a leaf)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (OriginAS uint32)
- [ ] Functional tests for end-to-end behavior (ospf/isis redist)
- [ ] Interop tests for protocol features (N/A -- internal refactor, no wire change)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-unify-route-events.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-unify-route-events.md` only
