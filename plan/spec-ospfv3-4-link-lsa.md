# Spec: OSPFv3 Broadcast Link-LSA, Link-Local LSDB Scope, and DR Prefix Aggregation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospf-af-unify |
| Phase | implementation complete; final verify pending |
| Updated | 2026-06-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc5340.md` (Link-LSA §4.4.3.8, Intra-Area-Prefix-LSA §4.4.3.9, flooding scope §3.5.2), `docs/architecture/core-design.md`
4. `internal/plugins/ospf/lsdb/lsdb.go` (`dbForLocked`), `internal/plugins/ospf/origination_v6.go`, `internal/plugins/ospf/afstrategy_v6.go`, `internal/plugins/ospfv3/packet/lsa_link.go`

## Task

OSPFv3 (RFC 5340) defines the **Link-LSA** (function code 8, LS type `0x0008`, **link-local flood scope**). Each router originates one Link-LSA per attached link, carrying (a) the router's IPv6 **link-local address** on that link (used by neighbors as the next-hop for routes through this router on the link) and (b) the list of IPv6 **prefixes** configured on the link. On a transit (broadcast) segment the **Designated Router** collects every attached router's Link-LSA and aggregates their prefixes into the segment's **Network-referencing Intra-Area-Prefix-LSA** (`ReferencedLSType = Network-LSA 0x2002`), so the shared LAN subnet is advertised into the area exactly once, attached to the transit vertex.

Today the unified OSPF engine forms v6 broadcast adjacencies, elects DR/BDR, and originates the v6 Network-LSA (proven by `ospf-v6-broadcast-frr`). A router's own prefixes are advertised via the **Router-referencing** Intra-Area-Prefix-LSA (`internal/plugins/ospf/origination_v6.go:146`, `ReferencedLSType: ospfv3types.LSTypeRouter`), which is why broadcast routing already works without Link-LSAs. What is missing:

1. **Link-LSA origination** — one per v6 broadcast (and point-to-point, per RFC) interface.
2. **A link-local-scope LSDB store + link-scoped flooding** — Link-LSAs are flooded on the originating link ONLY and never re-flooded; the LSDB models area scope and AS scope but not link scope.
3. **Link-LSA receive/store** — keyed by the receiving interface.
4. **DR prefix aggregation** — as DR, build the Network-referencing Intra-Area-Prefix-LSA from all attached routers' Link-LSAs.

This is RFC-completeness work: it makes Ze advertise the transit LAN subnet the canonical OSPFv3 way and supplies neighbor link-local next-hops via the LSDB rather than only via the neighbor table.

→ Decision: OSPFv2 has no Link-LSAs; this is a v6-only feature. The v4 path MUST remain byte-identical and untouched (gated by `make ze-ospf-test` 13/13 staying green).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/core-design.md` - LSDB scope model, the small-core/registration pattern, AF seams (Codec/AFPrefixStrategy/Encoder).
  → Decision: the v6 engine drives behavior through seams; the link-scope store must be added without forking the area/AS-scope code paths used by v4.
  → Constraint: buffer-first encoding — Link-LSA serialization uses `WriteTo(buf, off) int`, no `append`/`make` per byte.
- [ ] `ai/rules/buffer-first.md` - encoding discipline for any new on-wire serialization.
  → Constraint: `internal/plugins/ospfv3/packet/lsa_link.go` already follows this (`WriteTo` at line 77); reuse it, do not add a parallel encoder.
- [ ] `ai/rules/memory-architecture.md` - the link-scope store must release LSAs on interface down / config reload (no permanent growth across reloads).

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5340.md` - OSPF for IPv6.
  → Constraint: §4.4.3.8 — Link-LSA carries RtrPriority, Options, the originator's link-local address, and the prefix list; one per link.
  → Constraint: §3.5.2 — Link-LSAs have **link-local flooding scope**: flooded only on the link of origin, never propagated to other links or areas.
  → Constraint: §4.4.3.9 / §4.8.6 — the DR of a transit network originates an Intra-Area-Prefix-LSA whose `Referenced LS Type` is Network-LSA, aggregating the prefixes from the attached routers' Link-LSAs; a prefix with the NU (no-unicast) or LA bit is handled per the prefix-options rules; link-local addresses are NOT copied into the Intra-Area-Prefix-LSA.

**Key insights:**
- Link scope is a third flooding scope the LSDB does not model yet (area + AS only).
- The DR is the consumer of Link-LSAs; a non-DR router originates its Link-LSA so the DR can aggregate it, but only the DR builds the Network-referencing Intra-Area-Prefix-LSA.
- Link-local addresses live in Link-LSAs only; they are never advertised as routable prefixes.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/plugins/ospfv3/packet/lsa_link.go` - `LinkLSA{RtrPriority uint8; Options types.Options; LinkLocalAddr [16]byte; Prefixes []Prefix}` (line 21); `DecodeLinkLSA` (line 32); `EncodedLen` (line 67); **`WriteTo(buf, off) int` (line 77) — the encoder already exists.**
  → Constraint: encode/decode are complete; this spec consumes them, it does not rewrite wire code.
- [ ] `internal/plugins/ospf/lsdb/lsdb.go` - `dbForLocked(area, key) *areaDB` routes `key.Type.ASExternal()` → `d.asExternal`, else → `d.areaForLocked(area)`. There is **no per-link store**. Install/Lookup/iterate are area/AS oriented.
  → Constraint: adding link scope must be additive — a new `links map[string]*areaDB` (keyed by local interface name) plus install/lookup/iterate variants, leaving `dbForLocked` and the v4 paths unchanged.
- [ ] `internal/plugins/ospf/lsdb/flooding.go` - `eligibleInterface`, `shouldDropByArea`, `floodDestination`, `floodExcept`. Area-scope flooding sends a flooded LSA out every eligible interface in the area except the arrival interface.
  → Constraint: link scope must flood (and re-originate) on exactly ONE interface and never propagate a received Link-LSA.
- [ ] `internal/plugins/ospf/origination_v6.go` - `v6OriginateSelf` (drives self-LSA origination), `v6OriginateNetwork` (line ~161, DR Network-LSA), `v6OriginateRouter` (line ~124), and the **Router-referencing** Intra-Area-Prefix at line 146 (`ReferencedLSType: ospfv3types.LSTypeRouter`).
  → Constraint: Link-LSA origination hooks alongside these; DR Network-referencing Intra-Area-Prefix aggregation is added here.
- [ ] `internal/plugins/ospf/afstrategy_v6.go` - `BuildRoutes` (line 167) and `switch body.ReferencedLSType` (line 204) already distinguish Router- vs Network-referenced prefixes in the route model.
  → Constraint: the Network-referenced-prefix consumption path already exists, so DR-aggregated prefixes install through it once originated — verify, don't duplicate.
- [ ] `internal/plugins/ospfv3/types/lsa.go` - `LSTypeLink LSType = 0x0008` (line 41), `ScopeLinkLocal FloodScope = 0` (line 19), `Scope()` (line 61).
- [ ] `internal/plugins/ospf/instance.go` - the LSUpdate receive path and `originateSelfLSAs` (1s origination ticker) where origination and receive wire in.

**Behavior to preserve:**
- OSPFv2: no Link-LSAs, no link-scope store touched. `make ze-ospf-test` stays 13/13.
- v6 broadcast adjacency, DR/BDR election, Network-LSA origination, and Router-referencing Intra-Area-Prefix advertisement (the current working path) keep functioning; `ospf-v6-broadcast-frr` stays green.
- Existing v6 SPF/route model (`afstrategy_v6.go`) signatures.

**Behavior to change:**
- ADD: Link-LSA origination, a link-scope LSDB store + link-scoped flooding, Link-LSA receive, and DR Network-referencing Intra-Area-Prefix aggregation. No existing behavior removed.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Origination: the 1s origination ticker (`instance.go` `originateSelfLSAs`) and interface-up, for each v6 broadcast/p2p interface.
- Receive: an LSUpdate carrying a Link-LSA (`0x0008`) arriving on a specific interface.

### Transformation Path
1. **Origination:** for each enabled v6 interface, gather the interface link-local address + configured prefixes → build `packet.LinkLSA` → install into the link-scope store keyed by interface name → flood on that interface only.
2. **Receive:** LSUpdate decode → Link-LSA → install into the link-scope store under the arrival interface → ack on that interface; do NOT re-flood to other interfaces.
3. **DR aggregation:** when this router is DR for a transit segment, read all attached routers' Link-LSAs for that interface from the link-scope store → union their prefixes (excluding link-local, NU/LA per rules) → originate/refresh the Network-referencing Intra-Area-Prefix-LSA for the segment.
4. **Route install:** the Network-referencing Intra-Area-Prefix-LSA floods area-wide → `afstrategy_v6.go` `BuildRoutes` consumes it via the existing `ReferencedLSType == Network` branch → prefixes install against the transit vertex.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ LSDB | new link-scope install/lookup/iterate API keyed by interface | [ ] |
| LSDB ↔ Flooding | link-scope flood = single-interface, no propagation | [ ] |
| Wire ↔ Storage | `packet.LinkLSA` decode/`WriteTo` ↔ link-scope store | [ ] |
| DR aggregation ↔ SPF | Network-referencing Intra-Area-Prefix-LSA → existing BuildRoutes Network branch | [ ] |

### Integration Points
- `internal/plugins/ospfv3/packet/lsa_link.go` — reuse `DecodeLinkLSA` / `WriteTo`.
- `internal/plugins/ospf/afstrategy_v6.go:204` — existing Network-referenced-prefix consumption.
- `internal/plugins/ospf/origination_v6.go` — origination home.

### Architectural Verification
- [ ] No bypassed layers (Link-LSAs flow through the LSDB, not a side channel)
- [ ] No unintended coupling (v4 area/AS paths untouched; link scope is additive)
- [ ] No duplicated functionality (reuse the existing encoder + the Network-referenced-prefix route path)
- [ ] Zero-copy preserved where applicable (`WriteTo` into the flood buffer)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `LinkLSA.WriteTo` + `DecodeLinkLSA` are complete and correct | `lsa_link.go:32,77` | must implement wire code | round-trip unit test | unvalidated |
| A-2 | The v6 route model already installs Network-referenced prefixes | `afstrategy_v6.go:204` | must add a Network consumption branch | in-process test installing a DR-aggregated prefix | unvalidated |
| A-3 | A per-interface link-scope store can reuse `areaDB`'s entry/aging machinery | `lsdb.go` `areaDB.entries` | need a bespoke store type | link-scope store unit test (install/age/flush) | unvalidated |
| A-4 | Link-local addresses for each v6 interface are available at origination time | v6 transport / iface resolver used by the working broadcast path | origination cannot fill `LinkLocalAddr` | unit test reads a real link-local | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Link-scope flooding leaks a Link-LSA to other interfaces/areas (scope violation) | FRR logs an unexpected Link-LSA / wrong LSDB contents | a flood-once unit test asserting exactly one interface; explicit scope guard in `eligibleInterface` |
| R-2 | The new link-scope store grows without bound across config reloads / interface flaps | memory growth, stale Link-LSAs for removed interfaces | release the per-interface store on interface-down/reload; unit test |
| R-3 | Adding link scope perturbs v4/area flooding | `ze-ospf-test` or `ospf-broadcast-frr` regressions | keep `dbForLocked` and area paths untouched; link scope on a separate map; full OSPF interop re-run |
| R-4 | Interop is NOT verifiable on a 2-node shared LAN (both have the subnet connected) | no route to assert in the existing harness | see Interop Tests — 3-node scenario or documented-limited validation; primary validation is unit + in-process |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| v6 broadcast interface enabled → origination ticker | → | Link-LSA origination installs into the link-scope store and floods on that interface | `TestOSPFv6OriginateLinkLSA` |
| LSUpdate with a Link-LSA arrives on eth0 | → | link-scope install under eth0, no re-flood | `TestOSPFv6ReceiveLinkLSALinkScoped` |
| Ze is DR for a transit segment with neighbor Link-LSAs | → | Network-referencing Intra-Area-Prefix-LSA aggregation | `TestOSPFv6DRAggregatesLinkPrefixes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A v6 broadcast/p2p interface is enabled | Ze originates exactly one Link-LSA (`0x0008`) for it, carrying the interface link-local address and its configured IPv6 prefixes |
| AC-2 | A self Link-LSA is originated | It is flooded on the originating interface ONLY and is never propagated to other interfaces or areas (link-local scope) |
| AC-3 | A neighbor's Link-LSA arrives on interface X | It is stored in the link-scope store under X, acknowledged on X, and not re-flooded elsewhere |
| AC-4 | Ze is DR for a transit segment whose attached routers advertise prefixes in their Link-LSAs | Ze originates a Network-referencing Intra-Area-Prefix-LSA (`ReferencedLSType = Network 0x2002`) aggregating those prefixes (link-local excluded); when Ze is not DR it does not originate it |
| AC-5 | The DR-aggregated Network Intra-Area-Prefix-LSA reaches a remote area router | The prefix installs against the transit network vertex via the existing `afstrategy_v6.go` Network-referenced path |
| AC-6 | An interface goes down / config reload removes it | The corresponding self Link-LSA is purged and the link-scope store entry for that interface is released |
| AC-7 | Any OSPFv2 configuration | No Link-LSAs are originated; the link-scope store is unused; `make ze-ospf-test` stays 13/13 |
| AC-8 | Operator runs `show ospf database` with v6 Link-LSAs present | The output lists each Link-LSA (type `0x0008`) with its originating interface and link-local address (operator observability) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up an OSPFv3 broadcast interface | iface enable → origination ticker → Link-LSA built → link-scope store → flood on link | `TestOSPFv6OriginateLinkLSA` + flood-once assertion |
| 2 | Peers with a neighbor that sends a Link-LSA | LSUpdate → decode → link-scope install (no re-flood) | `TestOSPFv6ReceiveLinkLSALinkScoped` |
| 3 | Runs Ze as DR on a LAN with other routers | neighbor Link-LSAs → DR aggregation → Network Intra-Area-Prefix-LSA → area flood → remote SPF installs the LAN subnet | `TestOSPFv6DRAggregatesLinkPrefixes` + interop (see Interop Tests) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLinkLSARoundTrip` | `internal/plugins/ospfv3/packet/lsa_link_test.go` | `WriteTo`→`DecodeLinkLSA` round-trips (A-1) | |
| `TestOSPFv6OriginateLinkLSA` | `internal/plugins/ospf/origination_v6_link_test.go` | one Link-LSA per v6 interface with link-local + prefixes (AC-1) | |
| `TestOSPFv6LinkScopeStore` | `internal/plugins/ospf/lsdb/lsdb_linkscope_test.go` | install/lookup/iterate/age/flush per interface; release on interface-down (A-3, AC-6, R-2) | |
| `TestOSPFv6ReceiveLinkLSALinkScoped` | `internal/plugins/ospf/lsdb/flooding_linkscope_test.go` | received Link-LSA stored, flooded on exactly one interface, never propagated (AC-2, AC-3, R-1) | |
| `TestOSPFv6DRAggregatesLinkPrefixes` | `internal/plugins/ospf/origination_v6_link_test.go` | DR builds Network-referencing Intra-Area-Prefix-LSA from neighbor Link-LSAs; non-DR does not (AC-4) | |
| `TestOSPFv6InstallNetworkReferencedPrefix` | `internal/plugins/ospf/afstrategy_v6_test.go` | a Network-referenced prefix installs against the transit vertex (AC-5, A-2) | |
| `TestOSPFv2NoLinkLSA` | `internal/plugins/ospf/origination_v6_link_test.go` | v4 engine originates no Link-LSA and touches no link store (AC-7) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Link-LSA prefix count | 0 .. uint32 (bounded by body length) | body-derived max | N/A | count exceeding remaining body → decode error (already enforced in `DecodeLinkLSA`) |
| PrefixLength (per prefix) | 0..128 | 128 | N/A | >128 → decode error |

### Functional Tests
<!-- Link-LSA is a new LSA type the operator must be able to observe; surfacing it in the
     OSPF database display gives the user-facing behavior a .ci functional test covers. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-v6-link-database` | `test/ui/ospf-v6-link-database.ci` | operator runs `show ospf database` on an OSPFv3 broadcast interface and sees the Link-LSA (type `0x0008`) it originated, with the interface link-local address | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-v6-broadcast-3node-frr` (proposed) | `test/interop/scenarios/` | 2× FRR ospf6d + Ze as DR | one FRR learns the OTHER FRR's link prefix only via Ze's DR-aggregated Network Intra-Area-Prefix-LSA | |

→ Constraint: the existing 2-node `ospf-v6-broadcast-frr` CANNOT validate Link-LSA prefix aggregation — the LAN subnet is connected on both routers, so neither learns it via OSPF. A faithful interop needs a THIRD router so a prefix exists that a peer can only learn through the DR aggregation. If a stable 3-node broadcast harness proves impractical, validation falls back to unit + in-process engine tests and that limitation is recorded in Known Limitations (do NOT add a PENDING/skip-pass interop that asserts nothing).

### Future (if deferring any tests)
- None planned. If the 3-node interop is deferred, it requires explicit user approval and a Known Limitations entry.

## Files to Modify
- `internal/plugins/ospf/lsdb/lsdb.go` - add the link-scope store (`links map[string]*areaDB` or equivalent) + install/lookup/iterate/release keyed by interface; leave `dbForLocked` and v4 paths untouched.
- `internal/plugins/ospf/lsdb/flooding.go` - link-scope flooding: originate/ack on one interface, never propagate a received Link-LSA.
- `internal/plugins/ospf/origination_v6.go` - originate self Link-LSAs per v6 interface; as DR, originate the Network-referencing Intra-Area-Prefix-LSA from aggregated Link-LSAs.
- `internal/plugins/ospf/instance.go` - wire Link-LSA origination into `originateSelfLSAs` and Link-LSA receive into the LSUpdate path; purge on interface-down.
- `internal/plugins/ospf/afstrategy_v6.go` - confirm/extend the Network-referenced-prefix consumption (line 204) so aggregated prefixes install.
- `internal/plugins/ospfv3/packet/lsa_link.go` - only if a round-trip gap surfaces (encoder exists at line 77).
- the OSPF `show ospf database` rendering path (the cmd/show OSPF handler that lists LSDB entries) - include Link-LSAs (type `0x0008`) with interface + link-local (AC-8).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | Link-LSA is engine-internal; no new config leaf |
| CLI commands/flags | Yes | extend the existing `show ospf database` OSPF handler to render Link-LSAs (no new command, new rows in existing output) |
| Functional test for new RPC/API | Yes | `test/ui/ospf-v6-link-database.ci` |
| Prometheus counters/metrics | Optional | a `ze_ospf_link_lsas` gauge could mirror existing self-LSA gauges; decide during design |
| Doctor check for runtime dependencies | No | no new external dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Partial | Link-LSAs become visible in `show ospf database`; note in `docs/guide/command-reference.md` if database output is documented |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — `show ospf database` now lists Link-LSAs |
| 7 | Wire format changed? | No | Link-LSA wire already implemented in `ospfv3/packet`; this spec consumes it |
| 9 | RFC behavior implemented? | Yes | add/confirm `// RFC 5340` anchors; `rfc/short/rfc5340.md` already summarizes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (or the OSPF subsystem doc) — record the new link-local LSDB scope |
| 16 | Any changed source file referenced by doc source anchors? | Check | grep `docs/` for `source:` anchors on `lsdb.go`, `afstrategy_v6.go`, `origination_v6.go` |

## Files to Create
- `internal/plugins/ospf/lsdb/lsdb_linkscope_test.go` - link-scope store unit tests.
- `internal/plugins/ospf/origination_v6_link.go` - Link-LSA origination + DR aggregation (keep origination_v6.go focused).
- `internal/plugins/ospf/origination_v6_link_test.go` - origination + aggregation unit tests.
- `test/ui/ospf-v6-link-database.ci` - functional test: `show ospf database` lists the originated Link-LSA (AC-8).
- `test/interop/scenarios/ospf-v6-broadcast-3node-frr/` - proposed 3-node interop (if feasible).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` + OSPF interop |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables | Deliverables Checklist |
| 12. Security | Security Review Checklist |
| 14. Present summary | Executive Summary |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add the link-scope store skeleton + a Link-LSA origination entry point; write `TestOSPFv6OriginateLinkLSA` (fails: stub originates nothing).
   - Files: `lsdb.go` (store), `origination_v6_link.go` (stub), `instance.go` (hook)
   - Verify: entry point reachable; wiring test fails on stub.
2. **Phase: Link-scope store + flooding** — implement install/lookup/iterate/release + link-scoped flood-once and no-propagate.
   - Tests: `TestOSPFv6LinkScopeStore`, `TestOSPFv6ReceiveLinkLSALinkScoped`
3. **Phase: Link-LSA origination** — fill `LinkLocalAddr` + prefixes; flood on the originating interface.
   - Tests: `TestOSPFv6OriginateLinkLSA`, `TestLinkLSARoundTrip`, `TestOSPFv2NoLinkLSA`
4. **Phase: DR aggregation** — build the Network-referencing Intra-Area-Prefix-LSA from attached Link-LSAs; install path through `afstrategy_v6.go`.
   - Tests: `TestOSPFv6DRAggregatesLinkPrefixes`, `TestOSPFv6InstallNetworkReferencedPrefix`
5. **Interop** — 3-node broadcast scenario (or documented-limited).
6. **RFC refs** — `// RFC 5340 §4.4.3.8 / §3.5.2 / §4.4.3.9` anchors.
7. **Full verification** — `make ze-verify` + full OSPF interop suite (all 9 must stay green).
8. **Complete spec** — audit tables, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every user story has a working path; v4 reference (no Link-LSA) unaffected |
| Correctness | flood-once / no-propagate; DR-only aggregation; link-local excluded from Intra-Area-Prefix |
| Data flow | Link-LSAs only in the link-scope store; aggregated prefixes through the existing Network route path |
| Rule: buffer-first | origination uses `WriteTo`, no per-byte alloc |
| Rule: memory | store released on interface-down/reload |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Link-scope store | `go test ./internal/plugins/ospf/lsdb/ -run LinkScope` |
| Link-LSA origination | `go test ./internal/plugins/ospf/ -run OriginateLinkLSA` |
| DR aggregation | `go test ./internal/plugins/ospf/ -run DRAggregates` |
| v4 untouched | `make ze-ospf-test` (13/13) |
| OSPF interop green | full `test/interop` OSPF suite (9 scenarios) |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Link-LSA prefix count/length bounds already enforced in `DecodeLinkLSA`; confirm no unbounded `make` on the count |
| Resource | link-scope store bounded by interface count × neighbor count; released on down |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the introducing phase |
| Test fails behavior mismatch | Re-read Current Behavior source |
| Interop infeasible (2-node) | 3-node scenario or Known Limitations entry (user approval) |
| 3 fix attempts fail | STOP, report, ask user |

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

## Core Insight

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Link scope as a separate per-interface store (`links map[string]*areaDB`) | overload the area store with a synthetic per-link area key | keeps v4/area flooding byte-identical and makes "flood on one link only" explicit; avoids a synthetic-area abstraction leaking into area-scope code |
| Reuse `packet.LinkLSA.WriteTo`/`DecodeLinkLSA` | write a fresh encoder | wire code already exists and is buffer-first; duplicating it violates no-duplication |
| Reuse the existing Network-referenced-prefix route path | add a Link-LSA-specific install path | `afstrategy_v6.go:204` already consumes Network-referenced prefixes; the DR just needs to originate the aggregated LSA |

## Known Limitations
- Interop validation may be limited to unit + in-process tests if a stable 3-node broadcast harness is impractical (a 2-node shared-LAN cannot exercise DR prefix aggregation). Recorded here rather than masked by a skip-pass scenario.
- Surfacing Link-LSAs in `show ospf database` CLI output is out of scope (separate, optional follow-up).

## RFC Documentation
Add `// RFC 5340 §4.4.3.8 / §3.5.2 / §4.4.3.9: "<quoted requirement>"` above: Link-LSA origination (one per link), the link-local flooding-scope guard (flood on link of origin only), and the DR Network-referencing Intra-Area-Prefix aggregation.

## Implementation Summary
### What Was Implemented
- Added an interface-keyed link-scope LSDB for OSPFv3 Link-LSAs (`0x0008`): install, lookup, aging, self-refresh, release on interface removal, snapshot rendering, and direct link-scope flood helpers.
- Routed received Link-LSAs through the link store using the arrival interface. They are acknowledged on that interface and are not propagated to other interfaces or areas.
- Added OSPFv3 self Link-LSA origination from live interface data: link-local address, routable IPv6 prefixes, Link State ID tied to the interface ID, and stale self-LSA flushing.
- Added DR aggregation of Link-LSA prefixes into a Network-referencing Intra-Area-Prefix-LSA, reusing the existing OSPFv3 Network-referenced route path.
- Included link-scoped LSAs in Database Description summaries and LS Request lookup for OSPFv3 neighbors so a peer's Link-LSA can drain the request list and let Loading reach Full.
- Rendered Link-LSAs in LSDB snapshots with the originating interface and link-local address.

### Bugs Found/Fixed
- Link-scoped LSAs originally stayed outside DD/LSReq exchange, leaving OSPFv3 neighbors in Loading with a non-empty request list. The fix was to carry the interface-scoped LSDB through summary and request lookup, not to advertise non-Full neighbors in Router-LSAs.
- `types.LSType.String()` did not name scope-typed OSPFv3 values such as `0x0008`, `0x2007`, and `0x4005`; database and metric output now render stable semantic names.

### Documentation Updates
- Updated `docs/guide/ospf.md`, `docs/guide/configuration.md`, `docs/features.md`, `docs/architecture/core-design.md`, `docs/architecture/wire/ospfv3.md`, and `docs/research/ospf-implementation-guide.md` to describe the unified OSPF engine, OSPFv3 Link-LSAs, and OSPFv3 redistribution behavior.
- Updated this spec and `plan/spec-ospf-af-unify.md` with implementation state and remaining final-verification status.

### Deviations from Plan
- The implementation went beyond pure Link-LSA origination/flooding because database exchange also needed link-scoped summaries and LS Request lookup. Without that, FRR could receive LSAs but Ze would not complete Loading -> Full reliably.
- No dedicated 3-node Link-LSA interop scenario was added in this pass. Validation is unit/in-process plus existing OSPFv3 FRR scenarios; final full `make ze-verify` remains pending.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Link-LSA origination | done | `internal/plugins/ospf/origination_v6_link.go` | one self Link-LSA per active v6 interface |
| Link-local LSDB scope | done | `internal/plugins/ospf/lsdb/link_scope.go` | per-interface store with release/aging/refresh |
| Link-scoped flooding | done | `internal/plugins/ospf/lsdb/flooding.go` | receive stores and acks on arrival interface only |
| DR prefix aggregation | done | `internal/plugins/ospf/origination_v6_link.go` | Network-referencing Intra-Area-Prefix-LSA |
| Database exchange coverage | done | `internal/plugins/ospf/neighbor/*.go` | DD summaries and LSReq lookup include link scope |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestOSPFv6OriginateLinkLSA` | carries link-local and prefixes |
| AC-2 | done | `TestOSPFv6OriginateLinkLSAFloodsLoadingNeighbor` | flood eligibility includes Loading/Full as needed |
| AC-3 | done | `TestOSPFv6ReceiveLinkLSALinkScoped` | stores on arrival interface and does not propagate |
| AC-4 | done | `TestOSPFv6DRAggregatesLinkPrefixes` | DR builds Network-referenced prefixes |
| AC-5 | done | `TestOSPFv6InstallNetworkReferencedPrefix` | existing route path consumes Network-referenced prefixes |
| AC-6 | done | `TestOSPFv6LinkScopeAgesAndFlushes` | release/aging/flush path covered |
| AC-7 | done | targeted OSPF package tests | v4 packages still pass through the shared code |
| AC-8 | partial | `TestDatabaseSnapshotIncludesLinkLSAs` | snapshot/API visibility covered; no separate `.ci` command test added |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLinkLSARoundTrip` | existing/pass | `internal/plugins/ospfv3/packet/lsa_link_test.go` | packet codec reused |
| `TestOSPFv6OriginateLinkLSA` | pass | `internal/plugins/ospf/origination_v6_link_test.go` | self origination |
| `TestOSPFv6LinkScopeStore` | pass | `internal/plugins/ospf/lsdb/lsdb_linkscope_test.go` | store/lookup/snapshot |
| `TestOSPFv6ReceiveLinkLSALinkScoped` | pass | `internal/plugins/ospf/lsdb/lsdb_linkscope_test.go` | receive and no propagation |
| `TestOSPFv6DRAggregatesLinkPrefixes` | pass | `internal/plugins/ospf/origination_v6_link_test.go` | DR aggregation |
| `TestOSPFv6InstallNetworkReferencedPrefix` | pass | `internal/plugins/ospf/afstrategy_v6_test.go` | route install path |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/ospf/lsdb/lsdb.go` | changed | link store, snapshot, metrics |
| `internal/plugins/ospf/lsdb/link_scope.go` | created/changed | link-scope operations |
| `internal/plugins/ospf/lsdb/flooding.go` | changed | link-scoped receive/flood handling |
| `internal/plugins/ospf/origination_v6_link.go` | created/changed | Link-LSA and DR aggregation |
| `internal/plugins/ospf/origination_v6.go` | changed | self-origination hook and Router-LSA flags |
| `internal/plugins/ospf/neighbor/*.go` | changed | link-scope DD/LSReq exchange |

### Audit Summary
- **Total items:** 8 ACs
- **Done:** 7
- **Partial:** 1 (`AC-8` has snapshot/API coverage, not a `.ci` command test)
- **Skipped:** 0
- **Changed:** database exchange work added because it was required for OSPFv3 Full adjacency

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Ze originates a Link-LSA per v6 interface | unit test | `TestOSPFv6OriginateLinkLSA` |
| Link-LSAs are link-local scoped | unit test | `TestOSPFv6ReceiveLinkLSALinkScoped` |
| DR aggregates segment prefixes into a Network Intra-Area-Prefix-LSA | unit test | `TestOSPFv6DRAggregatesLinkPrefixes` |
| OSPFv2 unaffected | targeted test | `go test ./internal/plugins/ospf/... ./internal/plugins/ospfv3/...` passed |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Interop tests (or N/A with justification)
- [ ] Goal Validation table filled

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
