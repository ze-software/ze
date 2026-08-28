# Spec: fib-depth -- Routing Decision and FIB Programming Depth

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-vrf-0-umbrella (VRF/table support) |
| Phase | 7/12 |
| Updated | 2026-07-22 |

Staleness note (2026-07-22 plan review): substantial phases have landed since
2026-05-27 without this header moving -- the SRv6 phase CLOSED (learned 1113:
both FIB backends program SRv6, `nexthop_linux.go` SEG6 + `fib/vpp/srv6.go`,
with tests and interop `bgp-srv6-frr`), and ECMP (learned 774) and VPP parity
(learned 798) also landed. The spec's own Current Behavior table still says
Ze SRv6 = "no", which shipped code contradicts. The live remainder includes
best-path step 6 (IGP cost: `BestStepIGPCost` deferred at `bestpath.go,182`,
`lookupIGPCost` returns 0). Next session on this spec: recount the phases
against the three learned closures before trusting 7/12, and refresh Current
Behavior.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/plugins/fib/kernel/fibkernel.go` -- current kernel FIB plugin
4. `internal/plugins/fib/kernel/backend_linux.go` -- current netlink backend
5. `internal/plugins/fib/vpp/fibvpp.go` -- current VPP FIB plugin
6. `internal/plugins/fib/vpp/backend.go` -- current VPP backend
7. `internal/component/bgp/plugins/rib/bestpath.go` -- best-path decision
8. `internal/plugins/sysrib/events/events.go` -- BestChangeEntry struct
9. `internal/core/rib/locrib/candidate.go` -- Loc-RIB Path struct

## Task

Finish the routing decision process and deepen FIB programming beyond prefix+next-hop.

Two gaps:
1. **Best-path step 6 (IGP cost to NEXT_HOP)** is a comment placeholder (`bestpath.go`).
   Recursive next-hop resolution is required before IGP cost comparison can work.
2. **FIB programming** installs only `(prefix, gateway)` tuples. Real FIBs carry route type,
   metric, ECMP groups, VRF table ID, blackhole/prohibit, MPLS labels, and SRv6 SIDs.

This is where competing daemons (FRR, BIRD, Junos, Nokia SR OS, Arista EOS) differentiate.
Ze must reach parity on the attributes that matter for production routing.

### Competitive Context

| Daemon | Recursive NH | ECMP | Nexthop Groups | VRF/table | Route type | Metric | MPLS push | SRv6 |
|--------|-------------|------|----------------|-----------|------------|--------|-----------|------|
| FRR | yes (zebra) | yes (nhg) | yes (kernel nhg) | yes | yes | yes | yes | yes |
| BIRD | yes (recursive) | yes (merge paths) | no (expanded) | yes (tables) | yes | yes | yes | partial |
| Junos | yes | yes | yes (nhg) | yes | yes | yes | yes | yes |
| Nokia SR OS | yes | yes | yes (nhg) | yes | yes | yes | yes | yes |
| Arista EOS | yes | yes | yes (nhg) | yes | yes | yes | yes | yes |
| Ze (today) | yes | yes (multipath) | no | no | yes | yes | yes | no |

### Design Decisions (proposed, pending approval)

| Decision | Detail | Rationale |
|----------|--------|-----------|
| Recursive NH tracker lives in sysrib | sysrib already resolves cross-protocol best. NH resolution is a second phase after prefix best-path. Keeps RIB plugins (bgp-rib) unchanged | Same pattern as FRR zebra NHT / BIRD recursive |
| BestChangeEntry gains rich fields | Add RouteType, Metric, Weight, TableID, Labels (already present), SRv6SID, ECMPGroup fields | FIB backends need this data; wire it through the event, not via side-channel lookups |
| ECMP via nexthop groups | Multiple equal-cost paths publish as a single change with an ECMPGroup containing []NextHop rather than N separate changes | Matches Linux kernel nexthop group API (5.3+), VPP multi-path FibPath, and avoids transient single-path states |
| Kernel backend uses nexthop objects | Linux 5.3+ `ip nexthop` API for NH groups rather than per-route multipath expansion | Atomic failover, shared NH state across routes, matches FRR/iproute2 direction |
| IGP cost comes from Loc-RIB metric | bgp-rib queries sysrib for the IGP metric of the resolved next-hop prefix. No OSPF/IS-IS internal coupling | Loc-RIB Path.Metric is exactly this: the IGP cost for internal next-hops |
| VRF table wired through BestChangeEntry.TableID | FIB backends use this to program into the correct kernel table or VPP table | Unblocks vrf-0-umbrella FIB programming without changing backend interfaces |
| Route types: unicast, blackhole, unreachable, prohibit | Static plugin already models blackhole/unreachable. Extend to FIB event so backends handle it | Linux RTN_BLACKHOLE/RTN_UNREACHABLE/RTN_PROHIBIT, VPP drop/unreach adjacencies |
| Consistent Linux/VPP semantics | Both backends must produce identical forwarding behavior for the same BestChangeEntry | Test via ze-test functional comparisons |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- FIB plugin architecture
  → Constraint: FIB plugins subscribe to sysrib events; they do not poll
- [ ] `docs/architecture/plugin/rib-storage-design.md` -- RIB best-path
  → Decision: best-path runs in bgp-rib plugin; sysrib arbitrates across protocols

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP Decision Process Section 9.1.2.2 (step 6: IGP cost)
  → Constraint: IGP cost comparison applies only between iBGP paths after eBGP-over-iBGP
- [ ] `rfc/short/rfc4364.md` -- BGP/MPLS IP VPNs (VRF route installation)
  → Constraint: VPN routes install into per-VRF tables identified by RD
- [ ] `rfc/short/rfc7911.md` -- ADD-PATH (multiple paths per prefix)
  → Constraint: ECMP selection happens post-ADD-PATH receive, not before
- [ ] `rfc/short/rfc8277.md` -- MPLS label encoding in BGP
  → Constraint: Label stack in BestChangeEntry must preserve order

**Key insights:**
- IGP cost comparison (step 6) needs a next-hop resolution table (NH -> IGP metric)
- ECMP is orthogonal to best-path: select N equal-cost bests, group them
- FIB programming is a projection of the resolved RIB state, not a 1:1 copy

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` -- RFC 4271 best-path; step 6 is a comment
- [ ] `internal/plugins/sysrib/events/events.go` -- BestChangeEntry has: Action, Prefix, NextHop, Protocol, Labels
- [ ] `internal/plugins/sysrib/sysrib.go` -- subscribes to rib best-change, emits system-rib best-change
- [ ] `internal/core/rib/locrib/candidate.go` -- Path struct: Source, Instance, NextHop, AdminDistance, Metric
- [ ] `internal/plugins/fib/kernel/fibkernel.go` -- processes batch, calls backend.addRoute(prefix, nextHop string)
- [ ] `internal/plugins/fib/kernel/backend_linux.go` -- buildRoute creates netlink.Route with Dst+Gw+Protocol only
- [ ] `internal/plugins/fib/vpp/fibvpp.go` -- processes batch, calls backend.addRoute(prefix, nextHop netip types)
- [ ] `internal/plugins/fib/vpp/backend.go` -- routeAddDel sends IPRouteAddDel with single FibPath, tableID from config
- [ ] `internal/plugins/fib/vpp/mpls.go` -- MPLS push/swap via VPP; labels already flow through
- [ ] `internal/plugins/static/backend_linux.go` -- multipath already implemented for static routes (netlink.MultiPath)
- [ ] `internal/plugins/static/config.go` -- blackhole/reject already parsed for static routes

**Behavior to preserve:**
- Single-path route installation still works (ECMP is additive)
- Existing BestChangeEntry JSON format (add new fields with omitempty)
- fib-kernel rtprotZE identification (stale-mark-then-sweep)
- fib-vpp tableID from config (will become per-change override)
- MPLS label flow in VPP (already works, extend to kernel)
- Static route blackhole/reject (already works, extend to dynamic routes)
- Metrics: route install/update/remove counters unchanged

**Behavior to change:**
- BestChangeEntry gains: RouteType, Metric, Weight, TableID, SRv6SID, ECMPPaths fields
- Kernel backend: use nexthop objects for ECMP, set route type, set metric, set table
- VPP backend: multi-path FibPath array, table from change not just config
- sysrib: recursive NH resolution phase, ECMP grouping
- bestpath.go: implement step 6 (IGP cost query to sysrib/loc-rib)

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE received, parsed, stored in bgp-rib
- bgp-rib runs best-path selection (step 6 now queries IGP cost)
- Winner published as (rib, best-change) to EventBus

### Transformation Path
1. **bgp-rib best-path** -- RFC 4271 decision process including step 6 IGP cost
2. **sysrib arbitration** -- cross-protocol best by AdminDistance, emits (system-rib, best-change)
3. **sysrib NH resolution** -- resolves recursive next-hops to directly-connected next-hops
4. **sysrib ECMP grouping** -- gathers equal-cost paths into ECMPPaths[]
5. **BestChangeEntry enrichment** -- adds RouteType, Metric, Weight, TableID, SRv6SID
6. **FIB backend programming** -- kernel netlink or VPP binary API

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| bgp-rib → sysrib | EventBus typed handle (rib, best-change) | [ ] |
| sysrib → fib-kernel/vpp | EventBus typed handle (system-rib, best-change) | [ ] |
| sysrib → bgp-rib (IGP cost query) | EventBus request-reply or direct lookup | [ ] |
| fib-kernel → Linux kernel | netlink RTM_NEWROUTE/RTM_NEWNEXTHOP | [ ] |
| fib-vpp → VPP | GoVPP IPRouteAddDel/MplsRouteAddDel | [ ] |

### Integration Points
- `sysribevents.BestChangeEntry` -- extended struct, all FIB consumers adapt
- `locrib.Path` -- NH resolution uses Loc-RIB to find IGP metric for a next-hop
- `bestpath.comparePair` -- step 6 calls into NH resolution table
- `netlinkBackend` -- replaces string-based API with structured route object
- `govppBackend` -- multi-path and route-type support

### Architectural Verification
- [ ] No bypassed layers (data flows through sysrib, not direct bgp-rib → FIB)
- [ ] No unintended coupling (bgp-rib does not import fib packages)
- [ ] No duplicated functionality (ECMP grouping in sysrib only, not per-backend)
- [ ] Zero-copy preserved where applicable (BestChangeEntry is value-typed)

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE with iBGP recursive NH | → | sysrib NH resolution + IGP cost lookup | `TestRecursiveNHResolution` |
| Two equal-cost iBGP paths | → | sysrib ECMP grouping | `TestECMPGroupEmission` |
| BestChangeEntry with RouteType=blackhole | → | kernel backend RTN_BLACKHOLE | `TestKernelBlackhole` |
| BestChangeEntry with ECMPPaths | → | kernel backend nexthop group | `TestKernelECMPGroup` |
| BestChangeEntry with TableID | → | kernel backend route in table N | `TestKernelVRFTable` |
| BestChangeEntry with ECMPPaths | → | VPP backend multi-path | `TestVPPMultiPath` |
| BestChangeEntry with Labels | → | kernel backend MPLS push | `TestKernelMPLSPush` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two iBGP paths to same prefix, different IGP cost to NH | Best-path selects lower IGP cost (step 6 no longer deferred) |
| AC-2 | Recursive next-hop (NH not directly connected) | sysrib resolves to directly-connected NH before emitting to FIB |
| AC-3 | N equal-cost paths after best-path | BestChangeEntry.ECMPPaths contains all N next-hops |
| AC-4 | ECMP change emitted to kernel backend | Linux nexthop group created, route points to nhg ID |
| AC-5 | ECMP change emitted to VPP backend | IPRouteAddDel with NPaths=N and N FibPath entries |
| AC-6 | BestChangeEntry with RouteType=blackhole | Kernel: RTN_BLACKHOLE, VPP: drop adjacency |
| AC-7 | BestChangeEntry with RouteType=unreachable | Kernel: RTN_UNREACHABLE, VPP: unreach adjacency |
| AC-8 | BestChangeEntry with Metric set | Kernel: route.Priority = metric, VPP: route weight |
| AC-9 | BestChangeEntry with TableID != 0 | Kernel: route installed in table N, VPP: route in table N |
| AC-10 | BestChangeEntry with Labels (kernel) | Kernel: MPLS encap via lwtunnel (ip route ... encap mpls) |
| AC-11 | BestChangeEntry with SRv6SID | Kernel: SRv6 encap via seg6, VPP: SR policy |
| AC-12 | NH becomes unreachable | All routes using that NH withdrawn from FIB |
| AC-13 | NH cost changes | Best-path re-evaluated for all prefixes using that NH |
| AC-14 | ECMP member fails (NH unreachable) | Nexthop group updated (member removed), not full withdrawal |
| AC-15 | Kernel/VPP produce identical forwarding for same input | Functional test with both backends shows same reachability |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestComparePairIGPCost` | `internal/component/bgp/plugins/rib/bestpath_test.go` | Step 6 selects lower IGP cost | |
| `TestRecursiveNHResolve` | `internal/plugins/sysrib/nhresolver_test.go` | Recursive NH resolved to direct NH | |
| `TestRecursiveNHUnreachable` | `internal/plugins/sysrib/nhresolver_test.go` | Unresolvable NH returns error | |
| `TestECMPGrouping` | `internal/plugins/sysrib/ecmp_test.go` | Equal-cost paths grouped into one entry | |
| `TestECMPMemberWithdraw` | `internal/plugins/sysrib/ecmp_test.go` | NH failure removes from group, not prefix | |
| `TestKernelRouteType` | `internal/plugins/fib/kernel/fibkernel_test.go` | Blackhole/unreachable/prohibit mapped | |
| `TestKernelMetric` | `internal/plugins/fib/kernel/fibkernel_test.go` | Priority field set | |
| `TestKernelTable` | `internal/plugins/fib/kernel/fibkernel_test.go` | Route in specified table | |
| `TestKernelNexhopGroup` | `internal/plugins/fib/kernel/fibkernel_test.go` | ECMP via nexthop objects | |
| `TestKernelMPLSEncap` | `internal/plugins/fib/kernel/fibkernel_test.go` | MPLS lwtunnel encap | |
| `TestVPPMultiPath` | `internal/plugins/fib/vpp/fibvpp_test.go` | NPaths > 1 with correct FibPaths | |
| `TestVPPRouteType` | `internal/plugins/fib/vpp/fibvpp_test.go` | Drop/unreach adjacency | |
| `TestVPPTable` | `internal/plugins/fib/vpp/fibvpp_test.go` | Per-change table override | |
| `TestBestChangeEntryJSON` | `internal/plugins/sysrib/events/events_test.go` | New fields serialize with omitempty | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| TableID | 0-4294967295 | 4294967295 | N/A (0=default) | N/A (uint32) |
| Metric | 0-4294967295 | 4294967295 | N/A (0=best) | N/A (uint32) |
| Weight | 1-256 | 256 | 0 (invalid) | 257 |
| ECMPPaths count | 1-128 | 128 | 0 (single path) | 129 (Linux ECMP limit) |
| Labels (kernel) | 1-3 | 3 | 0 (no MPLS) | 4 (kernel limit) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-fib-ecmp` | `test/bgp/fib-ecmp.ci` | Two eBGP peers, same prefix, ECMP installed | |
| `test-fib-recursive` | `test/bgp/fib-recursive.ci` | iBGP with recursive NH, route installed with resolved NH | |
| `test-fib-blackhole` | `test/bgp/fib-blackhole.ci` | Blackhole route programmed correctly | |
| `test-fib-vrf-table` | `test/bgp/fib-vrf-table.ci` | Route installed in non-default table | |
| `test-fib-mpls-kernel` | `test/bgp/fib-mpls-kernel.ci` | MPLS encap route in Linux kernel | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ecmp-frr` | `test/interop/scenarios/` | FRR | Both install same ECMP set for same UPDATEs | |
| `recursive-nh-frr` | `test/interop/scenarios/` | FRR | Recursive NH resolves identically | |
| `igp-cost-bird` | `test/interop/scenarios/` | BIRD | IGP cost tiebreaker produces same winner | |

### Future (if deferring any tests)
- SRv6 encap tests: deferred until SRv6 NLRI family is fully wired (depends on bgp-nlri-srv6)
- VPP SRv6 SR policy: deferred until VPP SR steer API integration

## Files to Modify
- `internal/plugins/sysrib/events/events.go` -- extend BestChangeEntry struct
- `internal/plugins/sysrib/sysrib.go` -- NH resolution phase, ECMP grouping
- `internal/component/bgp/plugins/rib/bestpath.go` -- implement step 6
- `internal/plugins/fib/kernel/fibkernel.go` -- rich route processing
- `internal/plugins/fib/kernel/backend_linux.go` -- netlink nexthop objects, route types, table, metric, MPLS
- `internal/plugins/fib/vpp/fibvpp.go` -- multi-path processing, route types
- `internal/plugins/fib/vpp/backend.go` -- multi-path FibPath, table override

### Files to Create
- `internal/plugins/sysrib/nhresolver.go` -- recursive NH resolution and tracking
- `internal/plugins/sysrib/nhresolver_test.go` -- unit tests
- `internal/plugins/sysrib/ecmp.go` -- ECMP grouping logic
- `internal/plugins/sysrib/ecmp_test.go` -- unit tests
- `internal/plugins/fib/kernel/nexthop_linux.go` -- Linux nexthop object management
- `test/bgp/fib-ecmp.ci` -- functional test
- `test/bgp/fib-recursive.ci` -- functional test
- `test/bgp/fib-blackhole.ci` -- functional test
- `test/bgp/fib-vrf-table.ci` -- functional test
- `test/bgp/fib-mpls-kernel.ci` -- functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/plugins/sysrib/yang/` (NH resolution config) |
| CLI commands/flags | [x] | `show system-rib nexthop-table`, `show system-rib ecmp-groups` |
| CLI grammar (action before identifier) | [x] | action-first: `show nexthop table`, `show ecmp groups` |
| Editor autocomplete | [x] | YANG-driven |
| Functional test for new RPC/API | [x] | `test/bgp/fib-*.ci` |
| Doctor check for runtime dependencies | [ ] | N/A (no new external deps) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- ECMP, recursive NH, rich FIB |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- NH resolution config |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` -- show nexthop/ecmp |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` -- nexthop-table, ecmp-groups |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- sysrib enhanced |
| 6 | Has a user guide page? | [x] | `docs/guide/routing.md` -- new page: routing decision + FIB |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A (sysrib is internal) |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc4271.md` -- note step 6 implemented |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- ECMP, recursive NH |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- NH resolution layer |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `./le verify-lint run && ./le test-unit  && ./le functional` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: BestChangeEntry extension (event contract)** -- add fields to the shared event struct
   - Tests: `TestBestChangeEntryJSON`
   - Files: `internal/plugins/sysrib/events/events.go`, `events_test.go`
   - Verify: existing tests still pass (omitempty preserves wire compat)

2. **Phase: NH Resolver** -- recursive next-hop tracking and resolution
   - Tests: `TestRecursiveNHResolve`, `TestRecursiveNHUnreachable`
   - Files: `internal/plugins/sysrib/nhresolver.go`, `nhresolver_test.go`
   - Verify: resolver maps recursive NH to direct NH using Loc-RIB

3. **Phase: IGP cost in best-path (step 6)** -- query NH resolver for IGP metric
   - Tests: `TestComparePairIGPCost`
   - Files: `internal/component/bgp/plugins/rib/bestpath.go`
   - Verify: iBGP paths with different IGP costs produce correct winner

4. **Phase: ECMP grouping** -- collect equal-cost paths into groups
   - Tests: `TestECMPGrouping`, `TestECMPMemberWithdraw`
   - Files: `internal/plugins/sysrib/ecmp.go`, `ecmp_test.go`
   - Verify: N equal paths produce one BestChangeEntry with ECMPPaths

5. **Phase: Kernel backend depth** -- nexthop objects, route type, metric, table, MPLS
   - Tests: `TestKernelRouteType`, `TestKernelMetric`, `TestKernelTable`, `TestKernelNexhopGroup`, `TestKernelMPLSEncap`
   - Files: `internal/plugins/fib/kernel/backend_linux.go`, `nexthop_linux.go`, `fibkernel.go`
   - Verify: mock netlink handle receives correct route/nexthop structures

6. **Phase: VPP backend depth** -- multi-path, route type, table override
   - Tests: `TestVPPMultiPath`, `TestVPPRouteType`, `TestVPPTable`
   - Files: `internal/plugins/fib/vpp/backend.go`, `fibvpp.go`
   - Verify: mock backend receives multi-path and route-type calls

7. **Phase: NH unreachability cascade** -- NH down triggers route withdrawal
   - Tests: from nhresolver_test.go + fibkernel_test.go
   - Files: sysrib, fibkernel, fibvpp
   - Verify: NH removal cascades to all dependent routes

8. **Functional tests** -- end-to-end with ze-test
9. **Interop tests** -- FRR/BIRD comparison
10. **RFC refs** -- `// RFC 4271 Section 9.1.2.2 Step 6` comments
11. **Full verification** -- `./le verify current mode full`
12. **Complete spec** -- learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-15 has implementation with file:line |
| Correctness | ECMP group atomicity (no transient single-path), NH resolution convergence |
| Naming | RouteType enum values match Linux RTN_ names (lowercase), VPP adjacency types |
| Data flow | NH resolution in sysrib only; FIB backends never resolve recursively |
| CLI grammar | `show nexthop table` (action first), `show ecmp groups` (action first) |
| Rule: buffer-first | BestChangeEntry remains value-typed, no heap allocation per-change |
| Rule: enum-over-string | RouteType is numeric enum, not string; parse at boundary |
| Rule: no-sprintf-alloc | Hot-path route processing uses no fmt.Sprintf |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| NH resolver resolves recursive NH | `go test ./internal/plugins/sysrib/ -run TestRecursiveNH` |
| ECMP grouping produces multi-path entries | `go test ./internal/plugins/sysrib/ -run TestECMP` |
| bestpath step 6 implemented | `grep -n "Step 6" internal/component/bgp/plugins/rib/bestpath.go` shows code, not comment |
| Kernel backend creates nexthop objects | `grep -rn "nexthop" internal/plugins/fib/kernel/` shows implementation |
| VPP backend multi-path | `grep "NPaths" internal/plugins/fib/vpp/backend.go` shows >1 |
| Functional tests exist | `ls test/bgp/fib-*.ci` |
| BestChangeEntry has new fields | `grep "RouteType\|ECMPPaths\|TableID" internal/plugins/sysrib/events/events.go` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | TableID from untrusted BGP peer must be ignored (only config/sysrib sets it) |
| Resource exhaustion | NH resolution depth limit (prevent infinite recursion) |
| Resource exhaustion | ECMP group size bounded (max 128 paths) |
| Privilege | Nexthop object creation requires CAP_NET_ADMIN (already held by fib-kernel) |
| Label validation | MPLS labels validated (20-bit range) before kernel programming |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, back to DESIGN |
| Functional test fails | Check AC; if AC wrong, back to DESIGN |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Phasing Proposal

This is a large spec. Recommend splitting into sub-specs for implementation:

| Sub-spec | Scope | Depends |
|----------|-------|---------|
| `spec-fib-depth-1-nhresolver` | NH resolution + IGP cost (AC-1, AC-2, AC-12, AC-13) | none |
| `spec-fib-depth-2-ecmp` | ECMP grouping + nexthop groups (AC-3, AC-4, AC-5, AC-14) | sub-1 |
| `spec-fib-depth-3-richroute` | Route type, metric, table, MPLS kernel (AC-6-10, AC-15) | sub-1 |
| `spec-fib-depth-4-srv6` | SRv6 encap (AC-11) | sub-3 + bgp-nlri-srv6 |

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

## RFC Documentation

Add `// RFC 4271 Section 9.1.2.2 Step 6: "prefer the route with the lowest IGP metric to the BGP next-hop"` above the IGP cost comparison code.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| IGP cost tiebreaker works | unit test + functional test | TestComparePairIGPCost + test-fib-recursive.ci |
| Recursive NH resolution | unit test + functional test | TestRecursiveNHResolve + test-fib-recursive.ci |
| ECMP installed in FIB | functional test + interop | test-fib-ecmp.ci + ecmp-frr |
| Rich route attributes in kernel | functional test | test-fib-blackhole.ci + test-fib-vrf-table.ci |
| Linux/VPP parity | functional comparison test | test comparing both backends |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify current mode full` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/`
- [ ] Summary included in commit
