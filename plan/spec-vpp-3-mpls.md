# Spec: vpp-3-mpls — MPLS Label Operations

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-vpp-2-fib (done) |
| Phase | 1/5 |
| Updated | 2026-05-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/spec-vpp-0-umbrella.md` — parent spec
4. `plan/learned/613-vpp-2-fib.md` — fib-vpp learned summary
5. `internal/plugins/fib/vpp/` — existing fibvpp code
6. `internal/component/bgp/plugins/rib/rib_bestchange.go` — best-path change tracking
7. `internal/plugins/sysrib/events/events.go` — sysrib event types
8. `internal/component/bgp/nlri/nlrisplit/` — NLRI splitter registry

## Task

Wire MPLS labels from BGP labeled unicast NLRI (RFC 8277) through the full
RIB/sysRIB chain into VPP's FIB via GoVPP. This requires work at three layers
that were previously framed as a separate "sysRIB label extension" prerequisite
but are in practice one indivisible chain:

1. **NLRI splitting**: labeled unicast (SAFI 4) has no nlrisplit splitter today,
   so these routes never enter the RIB at all. Add `SplitLabeled` to strip
   3-byte label entries and yield CIDR NLRI, then register for ipv4/ipv6
   mpls-label families.

2. **RIB label storage and best-change propagation**: store extracted labels as
   route metadata (pool handle on RouteEntry), add `Labels []uint32` to both
   bgp-rib and sysrib `BestChangeEntry`, populate labels in `resolve()` and
   pass through sysrib.

3. **fibvpp MPLS programming**: detect labels in best-change events, dispatch to
   GoVPP `IPRouteAddDel` with `LabelStack` (push) or `MplsRouteAddDel`
   (swap/pop).

### Design Validation (FRR / BIRD)

Both FRR and BIRD use the same approach as our Option A:

| Aspect | FRR | BIRD | Ze (this spec) |
|--------|-----|------|----------------|
| NLRI parse | Strip labels, extract plain prefix | Strip labels, decode as NET_IP4 | Strip labels in SplitLabeled |
| RIB key | Plain IP prefix (remaps SAFI 4 to SAFI 1) | Plain NET_IP4/IP6 | Plain netip.Prefix in BART trie |
| Label storage | `bgp_path_info_extra->labels` (interned) | `BA_MPLS_LABEL_STACK` (synthetic attr) | Pool handle on RouteEntry |
| Best-path | Same table as unlabeled | Same table as unlabeled | Same BART trie + bestPrev |
| FIB delivery | Labels on `zapi_nexthop.labels[]` | Labels on `nexthop.label[]` | Labels on `BestChangeEntry.Labels` |

This is what differentiates ze from IPng (which uses FRR LDP for MPLS) and VyOS (which does
not expose MPLS through VPP config). Direct label programming from BGP to VPP FIB is a
unique capability.

### Reference

- RFC 3107: Carrying Label Information in BGP-4
- RFC 8277: Using BGP to Bind MPLS Labels to Address Prefixes
- IPng.ch blog: MPLS label stack operations in VPP
- GoVPP mpls binapi: MplsRouteAddDel, MplsTableAddDel
- FRR: `bgpd/bgp_label.c` (bgp_nlri_parse_label), `bgpd/bgp_route.c:5169` (SAFI remap)
- BIRD: `proto/bgp/packets.c` (bgp_decode_mpls_labels, AF table .mpls flag)

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/fib/vpp/` — fib-vpp plugin (done in vpp-2)
  → Constraint: MPLS extends existing backend interface and event processing
- [ ] `internal/component/bgp/plugins/nlri/labeled/` — BGP labeled unicast NLRI plugin
  → Constraint: already decodes wire labels into `{"prefix":"...","labels":[N]}` JSON
  → Decision: labels must also flow as structured data through RIB/sysRIB events
- [ ] `internal/component/bgp/nlri/nlrisplit/` — NLRI splitter registry
  → Constraint: no splitter registered for SAFI 4 today; labeled unicast never enters RIB
  → Decision: add SplitLabeled, register for ipv4/ipv6 mpls-label
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` — best-path change tracking
  → Constraint: resolve() builds BestChangeEntry; must populate Labels from RouteEntry
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` — isCIDRFamily gate
  → Constraint: returns false for SAFI 4; must be true after label stripping
- [ ] `internal/component/bgp/plugins/rib/events/events.go` — bgp-rib BestChangeEntry
  → Constraint: no Labels field today
- [ ] `internal/plugins/sysrib/events/events.go` — sysrib BestChangeEntry
  → Constraint: no Labels field today
- [ ] `internal/plugins/sysrib/sysrib.go` — processEvent
  → Constraint: must pass labels through from incoming to outgoing entries
- [ ] `docs/architecture/core-design.md` — event payload format

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3107.md` — Carrying Label Information in BGP-4
  → Constraint: label encoded in NLRI, 20-bit label value, bottom-of-stack bit
- [ ] `rfc/short/rfc8277.md` — Using BGP to Bind MPLS Labels to Address Prefixes
  → Constraint: label binding procedures, withdraw semantics, label stack encoding

**Key insights:**
- Ze already decodes MPLS labels from BGP UPDATE NLRI (labeled unicast plugin)
- But labeled unicast routes never enter the RIB (no nlrisplit splitter for SAFI 4)
- FRR/BIRD both strip labels at parse time, store routes as plain IP prefixes
- FRR explicitly remaps SAFI_LABELED_UNICAST to SAFI_UNICAST for RIB storage
- Three MPLS operations: push (PE ingress), swap (P transit), pop (PE egress)
- VPP MPLS uses IPRouteAddDel with label stack in FibPath for push, MplsRouteAddDel for swap/pop
- MPLS must be enabled per-interface in VPP before label operations work

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/nlri/nlrisplit/register.go` — splitter registry init
  → Constraint: only CIDR (unicast/multicast) and EVPN registered; no SAFI 4
- [ ] `internal/component/bgp/nlri/nlrisplit/cidr.go` — SplitCIDR reference implementation
  → Constraint: labeled unicast wire format is [label(3)][prefix-len][prefix], not plain CIDR
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` — isCIDRFamily
  → Constraint: returns false for SAFI 4; routes go to opaque map, not BART
- [ ] `internal/component/bgp/plugins/rib/storage/routeentry.go` — RouteEntry struct
  → Constraint: no label storage; needs pool handle for labels
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` — handleReceivedStructured
  → Constraint: skips families where nlrisplit.Supported returns false
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` — checkBestPathChange + resolve
  → Constraint: resolve() builds entry without labels; parsePrevKey uses NLRIToPrefix (CIDR only)
- [ ] `internal/component/bgp/plugins/rib/events/events.go` — bgp-rib BestChangeEntry
  → Constraint: no Labels field
- [ ] `internal/plugins/sysrib/events/events.go` — sysrib BestChangeEntry
  → Constraint: no Labels field
- [ ] `internal/plugins/sysrib/sysrib.go` — processEvent
  → Constraint: copies Action/Prefix/NextHop/Protocol but no labels
- [ ] `internal/plugins/fib/vpp/fibvpp.go` — fib-vpp event processing
  → Constraint: extend processEvent to dispatch to MPLS methods when labels present
- [ ] `internal/plugins/fib/vpp/backend.go` — vppBackend interface
  → Constraint: extend with MPLS methods: addMPLSRoute, delMPLSRoute, enableMPLS
- [ ] `internal/component/bgp/plugins/nlri/labeled/` — labeled unicast NLRI plugin
  → Constraint: already decodes labels from wire format; used for CLI decode, not RIB storage

**Behavior to preserve:**
- fib-vpp IPv4/IPv6 route programming unchanged (no labels = same behavior)
- sysRIB event format backward compatible (labels field optional via omitempty)
- BGP labeled unicast plugin decode path unchanged (CLI / external plugin JSON)
- isCIDRFamily for existing families unchanged
- bestPrev machinery for unicast/multicast unchanged

**Behavior to change:**
- New nlrisplit splitter for ipv4/ipv6 mpls-label families
- Labeled unicast routes stored in BART trie after label stripping (same as FRR SAFI remap)
- Labels stored as pool-backed metadata on RouteEntry
- bgp-rib BestChangeEntry gains optional `Labels []uint32` field
- sysrib BestChangeEntry gains optional `Labels []uint32` field
- sysrib processEvent passes labels through
- fib-vpp backend gains MPLS methods
- fib-vpp processEvent checks for labels and dispatches to MPLS backend methods

## Data Flow (MANDATORY)

### Entry Point
- BGP reactor receives UPDATE with labeled unicast NLRI (SAFI 4) containing MPLS label stack
- nlrisplit.SplitLabeled strips labels, yields per-NLRI CIDR wire bytes + label metadata
- RIB stores route with labels as pool-backed metadata on RouteEntry
- Best-path selection runs on plain prefix (same as unlabeled routes, matching FRR/BIRD)

### Transformation Path
1. BGP UPDATE with MP_REACH_NLRI for ipv4/mpls-label or ipv6/mpls-label
2. `handleReceivedStructured` calls `nlrisplit.Split(fam, nlriBytes, addPath)`
3. `SplitLabeled` decodes each NLRI: strips 3-byte label entries, returns CIDR wire bytes
4. `peerRIB.Insert(fam, attrBytes, wirePrefix)` stores route with label metadata
5. `checkBestPathChange` runs on plain prefix via BART trie (label-stripped)
6. `resolve()` populates `BestChangeEntry.Labels` from RouteEntry's label pool handle
7. `publishBestChanges` emits bgp-rib (best-change) with labels
8. sysRIB receives batch, passes labels through to sysrib (best-change)
9. fibvpp receives sysrib (best-change), checks Labels field:
   - Labels present, IP route: GoVPP `IPRouteAddDel` with `LabelStack` in `FibPath` (push)
   - Labels present, MPLS swap: GoVPP `MplsRouteAddDel` with in/out labels (swap)
   - Labels present, MPLS pop: GoVPP `MplsRouteAddDel` with pop action (pop)
   - No labels: standard IP route programming (existing behavior, unchanged)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP wire → nlrisplit | SplitLabeled strips labels, returns CIDR bytes + label metadata | [ ] |
| nlrisplit → PeerRIB | Insert with label pool handle on RouteEntry | [ ] |
| PeerRIB → bestPrev | checkBestPathChange on label-stripped CIDR prefix | [ ] |
| bestPrev → bgp-rib event | resolve() populates Labels from RouteEntry label handle | [ ] |
| bgp-rib event → sysRIB | sysRIB passes Labels through to outgoing BestChangeEntry | [ ] |
| sysRIB event → fibvpp | fibvpp reads Labels, dispatches to MPLS or IP methods | [ ] |
| fibvpp → GoVPP | IPRouteAddDel (push) or MplsRouteAddDel (swap/pop) | [ ] |

### Integration Points
- `internal/component/bgp/nlri/nlrisplit/` — new SplitLabeled splitter registration
- `internal/component/bgp/plugins/rib/storage/` — RouteEntry label pool handle
- `internal/component/bgp/plugins/rib/rib_bestchange.go` — resolve() labels
- `internal/component/bgp/plugins/rib/events/` — bgp-rib BestChangeEntry.Labels
- `internal/plugins/sysrib/events/` — sysrib BestChangeEntry.Labels
- `internal/plugins/sysrib/sysrib.go` — label pass-through
- `internal/plugins/fib/vpp/` — MPLS backend methods and event dispatch
- GoVPP mpls binapi — MplsRouteAddDel, MplsTableAddDel

### Architectural Verification
- [ ] No bypassed layers (labels flow through full RIB/sysRIB chain, not direct BGP-to-VPP)
- [ ] No unintended coupling (label info carried in existing event payload, not separate channel)
- [ ] No duplicated functionality (extends existing fibvpp, not parallel implementation)
- [ ] Labels stripped before RIB storage (matches FRR SAFI remap, BIRD NET_IP4 approach)
- [ ] Unlabeled route behavior unchanged (Labels field is omitempty)
- [ ] Zero-copy preserved where applicable (label stack is small fixed array)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Labeled unicast NLRI wire bytes | → | SplitLabeled strips labels, yields CIDR | `nlrisplit/labeled_test.go` |
| SplitLabeled output → peerRIB.Insert | → | RouteEntry stores labels via pool handle | `rib_bestchange_test.go` |
| Best-path change on labeled prefix | → | BestChangeEntry.Labels populated | `rib_bestchange_test.go` |
| bgp-rib (best-change) with Labels | → | sysRIB passes Labels to sysrib (best-change) | `sysrib/sysrib_test.go` |
| sysRIB event with labels, push | → | fibvpp MPLS push via IPRouteAddDel with LabelStack | `test/vpp/005-mpls-push.ci` |
| sysRIB event with labels, transit | → | fibvpp MPLS swap via MplsRouteAddDel | `test/vpp/005-mpls-push.ci` |
| sysRIB event with labels, withdraw | → | fibvpp MPLS route deletion | `test/vpp/005-mpls-push.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BGP labeled unicast prefix with label stack received | VPP FIB has IP route with MPLS label push (LabelStack populated in FibPath) |
| AC-2 | MPLS transit: in-label mapped to out-label + next-hop | VPP MPLS FIB has swap entry (MplsRouteAddDel) |
| AC-3 | MPLS egress: in-label with pop action | VPP MPLS FIB has pop entry |
| AC-4 | BGP withdraws labeled unicast prefix | VPP MPLS route removed |
| AC-5 | MPLS interface enable | VPP interface has MPLS enabled (MplsInterfaceEnableDisable) |
| AC-6 | No labels in sysRIB event | Standard IP route programming (existing behavior unchanged) |
| AC-7 | VPP restart with MPLS routes | Replay repopulates both IP and MPLS routes |
| AC-8 | Label value 0-1048575 (20-bit) | Valid labels programmed, out-of-range rejected |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSplitLabeledSingle` | `nlrisplit/labeled_test.go` | Single label + IPv4 prefix correctly split | |
| `TestSplitLabeledStack` | `nlrisplit/labeled_test.go` | Multi-label stack (S bit parsing) | |
| `TestSplitLabeledIPv6` | `nlrisplit/labeled_test.go` | IPv6 labeled unicast | |
| `TestSplitLabeledMalformed` | `nlrisplit/labeled_test.go` | Truncated / malformed input | |
| `TestSplitLabeledAddPath` | `nlrisplit/labeled_test.go` | ADD-PATH 4-byte prefix on labeled NLRI | |
| `TestRouteEntryLabels` | `storage/*_test.go` | RouteEntry stores/retrieves labels via pool handle | |
| `TestBestChangeLabels` | `rib_bestchange_test.go` | resolve() populates Labels from RouteEntry | |
| `TestBestChangeNoLabels` | `rib_bestchange_test.go` | Unlabeled routes produce empty Labels (backward compat) | |
| `TestSysribLabelPassthrough` | `sysrib/sysrib_test.go` | sysRIB copies labels from incoming to outgoing entry | |
| `TestProcessEventWithLabels` | `fib/vpp/mpls_test.go` | Event with labels → MPLS backend methods called | |
| `TestProcessEventWithoutLabels` | `fib/vpp/mpls_test.go` | Event without labels → standard IP route (no MPLS) | |
| `TestMPLSPush` | `fib/vpp/mpls_test.go` | Label push: IPRouteAddDel with LabelStack in FibPath | |
| `TestMPLSSwap` | `fib/vpp/mpls_test.go` | Label swap: MplsRouteAddDel with in/out labels | |
| `TestMPLSPop` | `fib/vpp/mpls_test.go` | Label pop: MplsRouteAddDel with pop action | |
| `TestMPLSDelete` | `fib/vpp/mpls_test.go` | MPLS route deletion | |
| `TestMPLSInterfaceEnable` | `fib/vpp/mpls_test.go` | MPLS enabled on VPP interface | |
| `TestMPLSLabelRange` | `fib/vpp/mpls_test.go` | Label 0-1048575 accepted, >1048575 rejected | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MPLS label | 0-1048575 (20-bit) | 1048575 | N/A | 1048576 |
| Label stack depth | 1-16 | 16 | 0 (no labels = IP route) | 17 (VPP FibPath limit) |
| TTL | 1-255 | 255 | 0 | 256 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-mpls-push` | `test/vpp/005-mpls-push.ci` | BGP labeled unicast → MPLS push in VPP FIB | |

### Future (if deferring any tests)
- Multi-label stack (stacked LSPs) deferred until MPLS VPN spec
- ECMP with unequal labels deferred

## Files to Modify

- `internal/component/bgp/nlri/nlrisplit/register.go` — register SplitLabeled for mpls-label families
- `internal/component/bgp/plugins/rib/storage/familyrib.go` — labeled unicast CIDR gate (after stripping)
- `internal/component/bgp/plugins/rib/storage/routeentry.go` — add label pool handle to RouteEntry
- `internal/component/bgp/plugins/rib/rib_bestchange.go` — resolve() populates Labels from RouteEntry
- `internal/component/bgp/plugins/rib/rib_structured.go` — pass label metadata through insert path
- `internal/component/bgp/plugins/rib/events/events.go` — add Labels to BestChangeEntry
- `internal/plugins/sysrib/events/events.go` — add Labels to BestChangeEntry
- `internal/plugins/sysrib/sysrib.go` — pass labels from incoming to outgoing change entries
- `internal/plugins/fib/vpp/fibvpp.go` — extend processEvent for labels
- `internal/plugins/fib/vpp/backend.go` — extend vppBackend interface with MPLS methods

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | MPLS config is part of fib-vpp YANG from spec-vpp-2 |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| NLRI splitter registration | Yes | `nlrisplit/register.go` |
| Event type schema change | Yes | bgp-rib + sysrib events |
| Functional test | Yes | `test/vpp/005-mpls-push.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — add MPLS label programming |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — update fib-vpp with MPLS |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` — MPLS section |
| 7 | Wire format changed? | Yes | Event payload schema (Labels field added) |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc3107.md`, `rfc/short/rfc8277.md` |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — MPLS from BGP |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — labeled unicast RIB storage |

## Files to Create

- `internal/component/bgp/nlri/nlrisplit/labeled.go` — SplitLabeled: strip 3-byte label entries, return CIDR wire bytes
- `internal/component/bgp/nlri/nlrisplit/labeled_test.go` — splitter tests for RFC 8277 wire format
- `internal/component/bgp/plugins/rib/pool/labels.go` — label pool (same pattern as pool.NextHop)
- `internal/plugins/fib/vpp/mpls.go` — MPLS route programming via GoVPP mpls.RPCService
- `internal/plugins/fib/vpp/mpls_test.go` — MPLS tests
- `test/vpp/005-mpls-push.ci` — MPLS functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella + vpp-2 |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: NLRI splitter** — labeled unicast enters the RIB
   - Tests: SplitLabeled for single/multi-label, IPv4/IPv6, malformed input
   - Files: `nlrisplit/labeled.go`, `nlrisplit/labeled_test.go`, `nlrisplit/register.go`
   - Verify: tests fail → implement → tests pass
   - Key: strip 3-byte label entries per RFC 8277, return CIDR wire bytes

2. **Phase: RIB label storage** — labels survive RIB insert/lookup
   - Tests: RouteEntry with/without labels, label pool intern/release
   - Files: `pool/labels.go`, `storage/routeentry.go`, `storage/familyrib.go`, `rib_structured.go`
   - Verify: tests fail → implement → tests pass
   - Key: isCIDRFamily returns true for SAFI 4 after label stripping; label pool handle on RouteEntry

3. **Phase: Best-change label propagation** — labels flow through events
   - Tests: BestChangeEntry with Labels populated, sysRIB pass-through, round-trip
   - Files: `rib/events/events.go`, `rib_bestchange.go`, `sysrib/events/events.go`, `sysrib.go`
   - Verify: tests fail → implement → tests pass
   - Key: resolve() reads label pool handle; sysRIB copies Labels field

4. **Phase: fibvpp MPLS methods** — GoVPP MPLS API calls
   - Tests: `TestMPLSPush`, `TestMPLSSwap`, `TestMPLSPop`, `TestMPLSDelete`, `TestMPLSInterfaceEnable`, `TestMPLSLabelRange`
   - Files: `fib/vpp/mpls.go`, `fib/vpp/mpls_test.go`
   - Verify: tests fail → implement → tests pass

5. **Phase: fibvpp event dispatch** — processEvent routes to MPLS methods
   - Tests: `TestProcessEventWithLabels`, `TestProcessEventWithoutLabels`
   - Files: `fib/vpp/fibvpp.go`, `fib/vpp/backend.go`
   - Verify: tests fail → implement → tests pass

6. **Functional tests** → `test/vpp/005-mpls-push.ci`
7. **Full verification** → `make ze-verify`
8. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | MPLS label operations match RFC 3107/8277 semantics |
| Correctness | SplitLabeled handles S-bit (bottom-of-stack) correctly |
| Correctness | Label stripping produces valid CIDR NLRI for BART storage |
| Naming | Label operations use VPP API naming conventions |
| Data flow | Labels flow through full chain: nlrisplit → RIB → bestchange → sysRIB → fibvpp |
| Rule: no-layering | No separate MPLS channel, extends existing event payload |
| Rule: FRR/BIRD alignment | Label stripped at parse, prefix is RIB key, labels are metadata |
| Backward compatibility | Events without labels still work (standard IP routes) |
| Backward compatibility | Existing unlabeled unicast route behavior unchanged |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| NLRI splitter registered | `grep "mpls-label" internal/component/bgp/nlri/nlrisplit/register.go` |
| Label pool exists | `ls internal/component/bgp/plugins/rib/pool/labels.go` |
| RouteEntry has label handle | `grep "Label" internal/component/bgp/plugins/rib/storage/routeentry.go` |
| bgp-rib BestChangeEntry.Labels | `grep "Labels" internal/component/bgp/plugins/rib/events/events.go` |
| sysrib BestChangeEntry.Labels | `grep "Labels" internal/plugins/sysrib/events/events.go` |
| sysRIB passes labels through | `grep "Labels" internal/plugins/sysrib/sysrib.go` |
| MPLS backend methods | `grep "addMPLSRoute\|delMPLSRoute\|enableMPLS" internal/plugins/fib/vpp/mpls.go` |
| Labels in event processing | `grep "Labels" internal/plugins/fib/vpp/fibvpp.go` |
| Splitter tests | `go test ./internal/component/bgp/nlri/nlrisplit/ -run TestSplitLabeled` |
| MPLS tests | `go test ./internal/plugins/fib/vpp/ -run TestMPLS` |
| Functional test | `ls test/vpp/005-mpls-push.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Label range | MPLS labels validated to 20-bit range (0-1048575) before GoVPP call |
| Stack depth | Label stack depth bounded by VPP FibPath limit (16) |
| TTL | TTL values validated to 1-255 range |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## MPLS Operations

| Operation | When | VPP API | Parameters |
|-----------|------|---------|------------|
| Push | PE ingress: IP packet needs label encap | IPRouteAddDel | IP prefix, FibPath with LabelStack populated, TTL=64 |
| Swap | P transit: labeled packet, swap label | MplsRouteAddDel | In-label (MrLabel), out-label (in LabelStack), next-hop, EOS bit |
| Pop | PE egress: remove label, deliver IP | MplsRouteAddDel | In-label (MrLabel), no out-label, next-hop for IP delivery |
| Enable | Before any MPLS ops on interface | MplsInterfaceEnableDisable | SwIfIndex, Enable=true |
| Disable | Cleanup | MplsInterfaceEnableDisable | SwIfIndex, Enable=false |

## Event Payload Extension (both bgp-rib and sysrib)

Both `BestChangeEntry` structs gain a `Labels` field. The bgp-rib version is richer
(has Priority, Metric, ProtocolType); the sysrib version is the downstream consumer
interface. Both use `omitempty` for backward compatibility.

### bgp-rib BestChangeEntry (extended)

| Field | Type | Existing | Description |
|-------|------|----------|-------------|
| action | RouteAction | yes | "add", "update", "withdraw" |
| prefix | netip.Prefix | yes | IP prefix |
| add-path | bool | yes | ADD-PATH flag |
| path-id | uint32 | yes | ADD-PATH path ID |
| next-hop | netip.Addr | yes | Next-hop address |
| priority | int | yes | Admin distance (20 eBGP / 200 iBGP) |
| metric | uint32 | yes | MED |
| protocol-type | BGPProtocolType | yes | "ebgp" / "ibgp" |
| **labels** | **[]uint32** | **new** | **MPLS label stack (omitempty)** |

### sysrib BestChangeEntry (extended)

| Field | Type | Existing | Description |
|-------|------|----------|-------------|
| action | RouteAction | yes | "add", "update", "withdraw" |
| prefix | netip.Prefix | yes | IP prefix |
| next-hop | netip.Addr | yes | Next-hop address |
| protocol | string | yes | Route source protocol |
| **labels** | **[]uint32** | **new** | **MPLS label stack (omitempty)** |

Backward compatible: events without labels field are treated as pure IP routes.
External plugin processes consuming JSON see `"labels":[100,200]` only when labels are present.

## NLRI Wire Format (RFC 8277)

Labeled unicast NLRI format per entry:

```
[label-entry(3)]...[prefix-len(1)][prefix-bytes(0-16)]
```

Each 3-byte label entry:
```
[20-bit label][3-bit TC][1-bit S (bottom-of-stack)]
```

SplitLabeled reads label entries until the S bit is set (bottom of stack),
then the remaining bytes are standard CIDR `[prefix-len][prefix]` format.
The splitter returns the full wire bytes (labels + prefix) so the caller
can extract both; the RIB insert path strips labels and stores them
separately via the label pool.

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

### Option A validated by FRR and BIRD (2026-05-13)

Both major open-source implementations strip labels at NLRI parse time and store
routes as plain IP prefixes with labels as metadata:

- **FRR**: `bgp_nlri_parse_label` strips labels, passes plain `struct prefix` to
  `bgp_update`. Then `bgp_route.c:5169` remaps `SAFI_LABELED_UNICAST` to
  `SAFI_UNICAST` for RIB storage. Labels stored in interned
  `bgp_path_info_extra->labels`. FIB delivery via `zapi_nexthop.labels[]`.

- **BIRD**: AF table maps `BGP_AF_IPV4_MPLS` to `.net = NET_IP4` (same table
  type as plain unicast). `bgp_decode_mpls_labels()` strips labels during NLRI
  decode. Labels stored as `BA_MPLS_LABEL_STACK` synthetic attribute. FIB
  delivery via `nexthop.label[]`, programmed as `LWTUNNEL_ENCAP_MPLS`.

Ze's approach: `SplitLabeled` strips labels, BART trie keys on plain
`netip.Prefix`, labels stored as pool handle on `RouteEntry`, delivered
as `BestChangeEntry.Labels` field. Structurally identical to both.

## Implementation Summary

### What Was Implemented
- (To be filled after implementation)

### Bugs Found/Fixed
- (To be filled)

### Documentation Updates
- (To be filled)

### Deviations from Plan
- (To be filled)

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-vpp-3-mpls.md`
- [ ] Summary included in commit
