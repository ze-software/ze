# Spec: ospfv3-0 -- OSPFv3 for IPv6 (Umbrella, SKELETON)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-ospf-0-umbrella.md |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-ospf-0-umbrella.md` -- the OSPFv2 umbrella; OSPFv3 is the IPv6 sibling and reuses the SAME above-the-wire infra (component, config, Loc-RIB install, redistribution, sysrib) but a DIFFERENT wire format and LSA registry
3. `docs/research/ospf-implementation-guide.md` §15 "OSPFv3 (RFC 5340) Considerations" (lines ~1628-1672) -- the differences and the "do NOT unify v2/v3" recommendation
4. RFC 5340 (OSPF for IPv6), RFC 5838 (address families in OSPFv3), RFC 7166 (OSPFv3 authentication trailer), RFC 4552 (OSPFv3 IPsec)

## Task

**This is a SKELETON umbrella.** It exists so OSPFv3 (RFC 5340, OSPF for IPv6) is
not silently assumed delivered by the OSPFv2 set (`spec-ospf-0-umbrella.md`). It
captures the scope, the architectural decision (separate component, do NOT
unify), and a child-spec sketch. It is NOT ready for implementation: it must be
expanded into a full umbrella + child specs (status `design`) AFTER the OSPFv2
set is implemented and stable, because OSPFv3 reuses the OSPFv2 above-the-wire
patterns and the second implementation goes faster once the first is proven.

OSPFv3 adds IPv6 routing as a link-state IGP. It shares the high-level
architecture with OSPFv2 (areas, LSAs, DR/BDR, ISM/NSM, the §13 flooding
procedure, the §16 SPF) but is a SEPARATE protocol on the wire. FRR ships it as a
second daemon (`ospf6d`); the guide (§15) is emphatic that v2 and v3 must NOT be
unified into one component -- the LSA registries and encodings differ enough that
sharing leaks detail into both. OSPFv3 in Ze will live in its own component
(`internal/component/ospfv3/`), modelled on but not sharing code with
`internal/component/ospf/`.

### Why a separate component (decided with user, 2026-06-20)

| Reason | Detail |
|--------|--------|
| Different transport | IPv6 protocol 89, multicast `ff02::5` (AllSPFRouters) / `ff02::6` (AllDRouters), source = interface link-local address (RFC 5340 §2.9) |
| Different common header | 16 bytes vs 24: Version=3, Type, Length, Router ID, Area ID, Checksum, Instance ID (1), Reserved (1). NO authentication fields in the header (RFC 5340 §A.3.1) |
| Different LSA registry | LSA types carry the flooding scope in the top bits (link-local `0x0000`-`0x1FFF`, area `0x2000`-`0x3FFF`, AS `0x4000`-`0x5FFF`); new LSA types (Link-LSA, Intra-Area-Prefix-LSA) and topology/prefix separation (RFC 5340 §A.4) |
| Topology/prefix separation | Router-LSA and Network-LSA describe ONLY the graph (no prefixes); prefixes live in Intra-Area-Prefix-LSAs that reference the vertices. Prefix changes do not re-originate topology |
| Interface ID | 32-bit per-router-per-interface link identifier, because IPv6 link-local addresses are not globally unique (RFC 5340 §2.11) |
| Different authentication | No header auth fields; OSPFv3 uses the RFC 7166 authentication trailer (HMAC-SHA) or IPsec AH/ESP (RFC 4552), not OSPFv2's AuType field |

The above-the-wire infra is REUSED (not shared code, same patterns): the raw-IPv6
transport (extend the RSVP-TE / ospf-3 raw-socket pattern to `AF_INET6`), the
Loc-RIB install path (`locrib.Path` with `AdminDistance` -- a new `rib.admin-distance.ospfv3`
leaf, or reuse the IPv6 family on the existing path), the redistribution
source/consumer registries, the component/config/lifecycle, and the §16 SPF
shape. Most SPF/LSDB/FSM LOGIC is identical to OSPFv2, but the differing LSA
encodings mean the code cannot be cleanly shared in a first pass (guide §15).

## Existing Foundation (to confirm during expansion)

| Capability | Reuse for OSPFv3 |
|------------|------------------|
| OSPFv2 component (`internal/component/ospf/`) | The pattern source for every above-the-wire concern; copy structure, share no code |
| Raw-IP transport (RSVP-TE `transport_linux.go`, ospf-3) | Extend to `AF_INET6 SOCK_RAW` proto 89 with IPv6 multicast membership (`ff02::5`/`ff02::6`) |
| Loc-RIB install + sysrib + ECMP path-group expansion | OSPFv3 installs IPv6 `locrib.Path`s; the ECMP expansion already exists |
| Redistribution registries | OSPFv3 registers a source `ospfv3` and an IPv6 `RedistConsumer` |
| Admin distance | Add an `ospfv3` admin-distance leaf (default 110) OR reuse `ospf` -- decide during expansion |

## Scope

### In scope (when this set is expanded)

| Area | Likely child |
|------|--------------|
| Domain types + Interface ID + the IPv6 prefix encoding (variable-length, RFC 5340 §A.4.1) | ospfv3-1 |
| Wire codec: 16-byte common header, 5 packet types (v3 variants), the v3 LSA registry incl. Link-LSA and Intra-Area-Prefix-LSA, scope-in-type | ospfv3-2 |
| Raw IPv6 transport (`AF_INET6 SOCK_RAW` proto 89, `ff02::5`/`ff02::6` membership, link-local source) | ospfv3-3 |
| Component, `ze-ospfv3-conf.yang`, instance/area/interface scaffolding | ospfv3-4 |
| ISM + Hello + DR/BDR election (Router ID based, link-local addressing) | ospfv3-5 |
| NSM + DD exchange (16-byte options, Instance ID matching) | ospfv3-6 |
| Per-area LSDB + flooding (scope-aware: link-local LSAs flood one link only) | ospfv3-7 |
| SPF over topology LSAs + Intra-Area-Prefix-LSA prefix attachment; IPv6 route install | ospfv3-8 |
| Inter-area (Inter-Area-Prefix-LSA / Inter-Area-Router-LSA) + ABR | ospfv3-9 |
| AS-External + ASBR + IPv6 redistribution | ospfv3-10 |
| Stub + NSSA (v3 NSSA-LSA `0x2007`) | ospfv3-11 |
| Authentication trailer (RFC 7166) | ospfv3-12 |
| CLI (`show ipv6 ospf6 ...`), metrics, doctor, FRR `ospf6d` interop | ospfv3-13 |

### Out of scope (future)

| Area | Reason |
|------|--------|
| Multiple address families in one instance (RFC 5838 IPv4-over-OSPFv3, multicast) | Single IPv6-unicast AF first; multi-AF via Instance ID later |
| IPsec authentication (RFC 4552) | Use the RFC 7166 auth trailer; IPsec AH/ESP is brittle and a separate undertaking |
| Opaque/TE/SR/GR/BFD/virtual-links | Same deferrals as the OSPFv2 set |

## Architecture (package layout, sketch)

`internal/component/ospfv3/` mirroring `internal/component/ospf/`: `types/`,
`packet/`, `transport/`, `iface/`, `neighbor/`, `lsdb/`, `spf/`, `redistribute/`,
`register.go`, `config.go`, `instance.go`, `area.go`, `cmd_show.go`,
`yang/ze-ospfv3-conf.yang` + `ze-ospfv3-cmd.yang`. No shared package with
`component/ospf/` in the first pass.

## Child Specs

(Sketch only -- expand to full child specs when this umbrella moves to `design`.)
See the "In scope" table above for the likely 13-child breakdown mirroring the
OSPFv2 set (`spec-ospf-1-*.md` .. `spec-ospf-13-*.md`).

## Dependency Graph

This whole set depends on `spec-ospf-0-umbrella.md` (the OSPFv2 set) being
implemented and stable. Internal dependencies mirror the OSPFv2 dependency graph.

## RFC Coverage

| RFC | Topic | Summary status |
|-----|-------|----------------|
| RFC 5340 | OSPF for IPv6 (OSPFv3) | TO CREATE `rfc/short/rfc5340.md` (at expansion) |
| RFC 5838 | Support of Address Families in OSPFv3 | TO CREATE (out-of-scope marker) |
| RFC 7166 | Authentication Trailer for OSPFv3 | TO CREATE `rfc/short/rfc7166.md` (ospfv3-12) |
| RFC 4552 | Authentication/Confidentiality for OSPFv3 (IPsec) | TO CREATE (out-of-scope marker) |

## Key Design Questions (Resolved)

| Question | Decision | Rationale |
|----------|----------|-----------|
| Unify with OSPFv2? | No -- separate component | Guide §15: different wire/LSA registry; FRR ships two daemons. Reuse patterns, share no code |
| When to build? | After the OSPFv2 set is stable | OSPFv3 reuses the v2 patterns; the second implementation goes faster once the first is proven |
| First address family? | IPv6 unicast only | Multi-AF (RFC 5838) is a later extension via Instance ID |
| Authentication? | RFC 7166 auth trailer | IPsec (RFC 4552) is brittle; the trailer mirrors OSPFv2 RFC 7474 |

## Required Reading

### Architecture Docs
- [ ] `docs/research/ospf-implementation-guide.md` §15 - OSPFv3 differences + the do-not-unify recommendation
  -> Decision: separate component `internal/component/ospfv3/`; reuse OSPFv2 patterns, share no code
- [ ] `plan/spec-ospf-0-umbrella.md` - the OSPFv2 sibling; the above-the-wire infra OSPFv3 reuses
  -> Constraint: copy the Loc-RIB install / redistribution / component conventions; do NOT couple to `component/ospf/` code

### RFC Summaries (MUST for protocol work; created at expansion)
- [ ] `rfc/short/rfc5340.md` - OSPFv3 base
- [ ] `rfc/short/rfc7166.md` - OSPFv3 authentication trailer

**Key insights:**
- OSPFv3 is a separate protocol on the wire (16-byte header, scope-in-type LSAs, Link-LSA + Intra-Area-Prefix-LSA, topology/prefix separation, Interface ID) but shares OSPFv2's high-level architecture
- It is a SEPARATE component; do NOT unify with OSPFv2 (guide §15, FRR precedent)
- Build it AFTER the OSPFv2 set is stable; reuse the patterns, share no code

## Current Behavior (MANDATORY)

**Source files read:** (skeleton -- full survey at expansion)
- [ ] Ze has no OSPF (v2 or v3) today; the OSPFv2 set (`spec-ospf-0-umbrella.md`) is the pattern source
  -> Constraint: OSPFv3 is a new component built on the OSPFv2 patterns; shares no code

**Behavior to preserve:**
- All existing route sources (BGP, IS-IS, OSPFv2, static, connected) remain independent
- Loc-RIB / sysrib / fibkernel arbitration unchanged (OSPFv3 is another IPv6 source)

**Behavior to change:**
- New `ospfv3` config container and `internal/component/ospfv3/` component (at expansion)

## Data Flow (MANDATORY)

### Entry Point
- OSPFv3 packets arrive over IPv6 (proto 89, `ff02::5`/`ff02::6`) on enabled interfaces
- Config arrives as the `ospfv3` subtree of the YANG-validated config tree

### Transformation Path
1. Receive: raw IPv6 datagram -> common-header parse + validate -> dispatch by Type
2. Interface/Hello -> ISM + DR/BDR election
3. NSM (DD/LS Request) -> Full
4. LSDB (scope-aware) -> flooding
5. SPF over topology LSAs + Intra-Area-Prefix-LSA -> IPv6 routes
6. Install IPv6 `locrib.Path` -> sysrib -> fibkernel
7. Redistribute (separate path) via `redistevents`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire <-> engine | raw `AF_INET6 SOCK_RAW` proto 89, IPv6 multicast | [ ] |
| Engine <-> Loc-RIB | IPv6 `locrib.Path` insertion | [ ] |
| Engine <-> redistribution | `redistevents` (IPv6) | [ ] |

### Integration Points
- New component `internal/component/ospfv3/` (at expansion)

### Architectural Verification
- [ ] No bypassed layers (datagram -> codec -> engine -> Loc-RIB -> sysrib -> fib)
- [ ] No unintended coupling (independent of `component/ospf/` and IS-IS)
- [ ] No duplicated FIB path (reuse sysrib)
- [ ] Zero-copy preserved (LSDB raw bytes)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | OSPFv2 patterns transfer to OSPFv3 with v3-specific wire/LSA changes | guide §15 | More code differs than expected | expansion-time design review | unvalidated |
| A-2 | The raw-IP transport extends cleanly to `AF_INET6` with IPv6 multicast membership | ospf-3 / RSVP-TE precedent (IPv4) | IPv6 multicast needs different socket options | ospfv3-3 prototype | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Premature start before OSPFv2 is stable wastes effort | churn re-deriving shared patterns | Gate on OSPFv2 set completion |
| R-2 | Temptation to unify v2/v3 leaks detail into both | a shared package with version branches | Hard separation (guide §15) |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| config `ospfv3 { ... }` present | -> | OSPFv3 component starts (at expansion) | `TestOSPFv3ComponentStart` (ospfv3-4) |

## Acceptance Criteria

(Skeleton -- detailed ACs added at expansion, mirroring the OSPFv2 umbrella AC table for IPv6.)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two Ze nodes on an IPv6 link, OSPFv3 area 0 | Adjacency reaches Full; IPv6 prefixes installed |
| AC-2 | Interop with FRR `ospf6d` | Adjacency, LSDB sync, IPv6 route convergence |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures OSPFv3 on two IPv6-linked nodes | config -> component -> interface -> Hello -> Full | `TestOSPFv3AdjacencyFull` (at expansion) |
| 2 | Meshes with FRR `ospf6d` | full protocol over IPv6 | `test/interop/scenarios/ospfv3-*-frr` (at expansion) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (at expansion) | `internal/component/ospfv3/...` | per child specs ospfv3-1..13 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Instance ID | 0..255 | 255 | n/a | 256 |
| Interface ID | 0..0xFFFFFFFF | 0xFFFFFFFF | n/a | n/a |
| IPv6 PrefixLength | 0..128 | 128 | n/a | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (at expansion) | `test/ospfv3/*.ci` | IPv6 adjacency + route install | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospfv3-p2p-frr` | `test/interop/scenarios/` | FRR ospf6d | IPv6 P2P adjacency + route convergence | |

### Future (if deferring any tests)
- Multi-AF (RFC 5838) and IPsec (RFC 4552) interop deferred with those out-of-scope features

## Files to Modify
- (At expansion.) `internal/component/plugin/all/all.go` (regenerated), `internal/component/config/redistribute/...`, possibly `ze-rib-conf.yang` (an `ospfv3` admin-distance leaf)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes (at expansion) | `internal/component/ospfv3/yang/ze-ospfv3-conf.yang` |
| Doctor check for runtime dependencies | Yes (at expansion) | `CAP_NET_RAW`, raw IPv6 socket open |
| Prometheus counters/metrics | Yes (at expansion) | `ze_ospfv3_*` series |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes (at expansion) | `docs/features.md` |
| 6 | Has a user guide page? | Yes (at expansion) | `docs/guide/ospfv3.md` |
| 7 | Wire format changed? | Yes (at expansion) | `docs/architecture/wire/ospfv3.md` |
| 11 | Affects daemon comparison? | Yes (at expansion) | `docs/comparison.md` |

## Files to Create
- (At expansion.) `internal/component/ospfv3/`, `plan/spec-ospfv3-1-*.md` .. `plan/spec-ospfv3-13-*.md`, `rfc/short/rfc5340.md`, `rfc/short/rfc7166.md`, `docs/guide/ospfv3.md`, `docs/architecture/wire/ospfv3.md`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton) -- expand before implementing |

### Implementation Phases
1. **Phase: Expand this skeleton** - move to `design`, write the 13 child specs mirroring the OSPFv2 set, AFTER the OSPFv2 set is implemented and stable
2. **Phase: Foundations .. CLI/interop** - mirror the OSPFv2 phase order for IPv6

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Separation | No shared package with `component/ospf/`; v3 is independent |
| Correctness | v3 wire matches RFC 5340 (16-byte header, scope-in-type, Link/Intra-Area-Prefix LSAs) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Expanded umbrella + 13 v3 children | `ls plan/spec-ospfv3-*.md` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Authentication | RFC 7166 trailer verification; reject on mismatch |
| Input validation | Variable-length IPv6 prefix decode bounds |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Interop mismatch with ospf6d | Capture, compare, fix codec/FSM |
| 3 fix attempts fail | STOP; report; ask user |

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
- OSPFv3 is the IPv6 sibling of OSPFv2: identical high-level architecture, different wire/LSA registry. Build it second, reuse the patterns, share no code.

## Core Insight
The cost of OSPFv3 is mostly a second wire codec and LSA registry; the SPF/LSDB/FSM
logic is the OSPFv2 logic re-expressed for IPv6. Gating it on a stable OSPFv2 set
maximises pattern reuse and minimises rework.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Separate `internal/component/ospfv3/` | Unified v2/v3 component | Different wire/LSA registry; FRR split into two daemons (guide §15) |
| Build after OSPFv2 is stable | Build in parallel | Reuse proven patterns; the second implementation is faster |

## Known Limitations
- Skeleton only: not implementable until expanded to full child specs
- Single IPv6-unicast address family first (RFC 5838 multi-AF deferred)

## RFC Documentation
Add `// RFC 5340 Section X.Y: "<quoted requirement>"` (and RFC 7166 as applicable) above enforcing code, at expansion.

## Implementation Summary

### What Was Implemented
- (Skeleton -- nothing yet.)

### Bugs Found/Fixed
- (None.)

### Documentation Updates
- (None.)

### Deviations from Plan
- (None.)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Capture OSPFv3 scope so it is not assumed delivered by the v2 set | Done | this skeleton | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-2 | (at expansion) | per child spec | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (at expansion) | (pending) | `internal/component/ospfv3/...` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/spec-ospfv3-0-umbrella.md` | Done | this skeleton |

### Audit Summary
- **Total items:** skeleton capture
- **Done:** skeleton written
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| OSPFv3 scope captured as a deferred skeleton | spec file | `ls plan/spec-ospfv3-0-umbrella.md` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | (skeleton) | not implementation-ready; expand before /implement | this file | acknowledged |

### Fixes applied
- (None -- skeleton.)

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
| `plan/spec-ospfv3-0-umbrella.md` | (verify) | `ls plan/spec-ospfv3-0-umbrella.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| (skeleton) | ACs filled at expansion | n/a |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| (at expansion) | `test/ospfv3/*.ci` | n/a |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1, A-2 | unvalidated (skeleton) | resolved at expansion |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| (at expansion) | docs added with the v3 implementation | n/a |

## Checklist

### Goal Gates (MUST pass)
- [ ] Skeleton captures v3 scope + the separate-component decision
- [ ] Cross-references the OSPFv2 umbrella
- [ ] Expanded to full child specs before any v3 implementation

### Quality Gates (SHOULD pass)
- [ ] RFC 5340 / 7166 summaries created at expansion
- [ ] Implementation Audit complete (at expansion)

### Design
- [ ] No premature abstraction (no shared v2/v3 package)
- [ ] No speculative features (out-of-scope table honoured)
- [ ] Single responsibility per child
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written (at expansion)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled (at expansion)

### Completion (BLOCKING)
- [ ] Critical Review passes (at expansion)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled (at expansion)
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospfv3-0-umbrella.md` (at expansion)
- [ ] Summary included in commit
