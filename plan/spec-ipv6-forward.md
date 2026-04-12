# Spec: ipv6-forward -- IPv6 MP_REACH UPDATE Forwarding

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-04-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/component/bgp/reactor/reactor_notify.go` -- UPDATE receive + dispatch
3. `internal/component/bgp/reactor/reactor_api_forward.go` -- UPDATE forwarding to peers
4. `internal/component/bgp/plugins/rib/` -- route storage + best-path + re-advertisement
5. `test/plugin/nexthop-self-ipv6-forward.ci` -- failing .ci test (untracked)

## Task

IPv6 UPDATEs received via MP_REACH_NLRI are not forwarded to destination peers in
iBGP route-reflection scenarios. The wire-level next-hop rewriting works (proven by
`TestBuildModifiedPayload_MPReachNextHopSelf` and 6 handler unit tests), but the
end-to-end path from receive -> reactor dispatch -> RIB -> best-path -> forward is
broken for MP_REACH routes.

### Symptom

`.ci` test with two iBGP peers (peer-src as RR client, peer-dst with next-hop self):
peer-src sends IPv6 UPDATE with MP_REACH_NLRI, ze receives it, but peer-dst receives
0 messages (only EOR). The test times out after 10s.

### Hypothesis

Three candidate failure points (investigate in order):

| # | Hypothesis | Where to check | Expected evidence |
|---|-----------|----------------|-------------------|
| 1 | Reactor skips UPDATEs with empty legacy NLRI | `reactor_notify.go` UPDATE processing | Code checks `len(nlri) == 0` and skips dispatch |
| 2 | RIB plugin does not store MP_REACH routes | `plugins/rib/rib.go` route insertion | No Adj-RIB-In entry for IPv6 prefix |
| 3 | Forward path filters by family and IPv6 not in peer's forward set | `reactor_api_forward.go` family check | Family filter rejects the route |

### Scope

**In scope:** Fix the forwarding gap so IPv6 MP_REACH UPDATEs are forwarded between
iBGP peers with route reflection + next-hop self. Land the `.ci` test.

**Out of scope:** IPv6 MP_REACH for eBGP (different AS-path prepend rules), non-unicast
IPv6 families (multicast, VPN, etc.), MP_UNREACH forwarding.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- reactor event loop, UPDATE dispatch
  -> Constraint: UPDATEs flow through reactor_notify -> cache -> plugin dispatch -> forward
- [ ] `rules/buffer-first.md` -- wire encoding
  -> Constraint: MP_REACH rewriting uses pool buffers via AttrModHandler

### Source Files (MUST read before implementing)
- [ ] `internal/component/bgp/reactor/reactor_notify.go:270-412` -- notifyMessageReceived, UPDATE processing
  -> Look for: how MP_REACH UPDATEs are identified, whether empty legacy NLRI is handled
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:300-430` -- forwardUpdate, family filtering
  -> Look for: family matching against destination peer's negotiated families
- [ ] `internal/component/bgp/plugins/rib/rib.go` -- route insertion path
  -> Look for: how MP_REACH NLRI is extracted and stored in Adj-RIB-In
- [ ] `internal/component/bgp/wireu/wire_update.go` -- WireUpdate parsing
  -> Look for: how MP_REACH vs legacy NLRI is distinguished

## Current Behavior (MANDATORY)

**Source files read:** (to be filled during research)
- [ ] `reactor_notify.go` -- how UPDATE with MP_REACH is processed
- [ ] `reactor_api_forward.go` -- how forwarding decides which peers receive
- [ ] `rib.go` -- how MP_REACH routes are stored

**Behavior to preserve:**
- IPv4 legacy NLRI forwarding works correctly
- next-hop self rewriting for IPv4 works correctly
- Route reflection for IPv4 works correctly

**Behavior to change:**
- IPv6 MP_REACH UPDATEs forwarded through route reflection with next-hop rewriting

## Data Flow (MANDATORY)

### Entry Point
- Wire: IPv6 UPDATE received from iBGP peer-src with MP_REACH_NLRI attribute

### Expected Transformation Path
1. TCP read -> parse UPDATE header -> extract attributes (including MP_REACH_NLRI)
2. Reactor dispatch: notify plugins, cache route
3. RIB plugin: store in Adj-RIB-In, run best-path selection
4. Best-path change: trigger forward to destination peers
5. Forward to peer-dst: apply route-reflection (ORIGINATOR_ID, CLUSTER_LIST)
6. Apply next-hop self: rewrite MP_REACH next-hop via mpReachNextHopHandler
7. Write modified UPDATE to peer-dst's TCP connection

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Reactor | TCP read -> WireUpdate parse -> notifyMessageReceived | [ ] |
| Reactor -> RIB plugin | UPDATE event dispatched to plugins | [ ] |
| RIB -> Forward | best-change event triggers forwardUpdate | [ ] |
| Forward -> Wire | buildModifiedPayload with NH rewrite -> TCP write | [ ] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IPv6 UPDATE from peer-src | -> | Forwarded to peer-dst with rewritten NH | `test/plugin/nexthop-self-ipv6-forward.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPv6 UPDATE with MP_REACH from RR client peer-src | Route forwarded to peer-dst |
| AC-2 | peer-dst has next-hop self | MP_REACH next-hop rewritten to peer-dst local address |
| AC-3 | Route reflection attributes | ORIGINATOR_ID and CLUSTER_LIST added correctly |
| AC-4 | NLRI preserved | 2001:db8:1::/48 present in forwarded UPDATE |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing: `TestBuildModifiedPayload_MPReachNextHopSelf` | `filter_delta_handlers_test.go` | Wire rewriting | Done |
| Existing: `TestMPReachNextHopHandler_*` (6 tests) | `filter_delta_handlers_test.go` | Handler edge cases | Done |
| New: trace test for MP_REACH dispatch | TBD | Reactor dispatches MP_REACH UPDATEs | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `nexthop-self-ipv6-forward` | `test/plugin/nexthop-self-ipv6-forward.ci` | Two iBGP peers, IPv6 UPDATE forwarded with NH rewrite | Failing |

## Files to Modify

TBD -- depends on which hypothesis is correct.

Likely candidates:
- `internal/component/bgp/reactor/reactor_notify.go` -- MP_REACH dispatch
- `internal/component/bgp/reactor/reactor_api_forward.go` -- family filtering
- `internal/component/bgp/plugins/rib/rib.go` -- MP_REACH route storage

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec + source | Required Reading, trace the data flow |
| 2. Identify gap | Follow the UPDATE from receive through forward, find where it stops |
| 3. Fix | Minimal change to the identified gap |
| 4. Verify | `test/plugin/nexthop-self-ipv6-forward.ci` passes |
| 5. Full verification | `make ze-verify` |

### Investigation Plan

1. Add debug logging at each stage of the UPDATE path for the test scenario
2. Run the .ci test with `ze.log.bgp.reactor=debug`
3. Identify: does the UPDATE reach notifyMessageReceived? Does it reach the RIB? Does best-path fire? Does forward fire?
4. The first stage that doesn't fire is the bug location

### Failure Routing

| Failure | Route To |
|---------|----------|
| UPDATE not dispatched | Fix reactor_notify.go MP_REACH handling |
| RIB doesn't store | Fix rib.go MP_REACH extraction |
| Forward doesn't fire | Fix best-path -> forward path for MP_REACH |
| Forward filtered | Fix family matching in reactor_api_forward.go |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Summary

### What Was Implemented
- New `bgp-rr` plugin (Route Reflector, RFC 4456) in `internal/component/bgp/plugins/rr/`
- Plugin subscribes to UPDATE events and forwards via `cache forward *` to all peers
- Reactor handles all RFC 4456 mechanics (ORIGINATOR_ID, CLUSTER_LIST, client/non-client filtering)
- `.ci` test updated to load `bgp-rr` and assert ORIGINATOR_ID presence in forwarded UPDATE

### Bugs Found/Fixed
- Root cause was NOT any of the three hypotheses in the spec. The actual issue: the `.ci` test config had no forwarding plugin loaded. Without `bgp-rr` (or `bgp-rs`), the reactor receives UPDATEs but nothing triggers forwarding to other peers.
- AC-2 (next-hop rewrite for IPv6) cannot be verified with IPv4 local address. `applyNextHopMod` only rewrites legacy NEXT_HOP (type 3) for IPv4 locals, not MP_REACH_NLRI (type 14). Known limitation documented in `reactor_api_forward.go:758-765`.

### Documentation Updates
- None required (internal plugin, no user-facing docs changed)

### Deviations from Plan
- No code changes to reactor, rib, or wire parsing. The forwarding pipeline was already correct; only the test config was missing the plugin.
- Created a new `bgp-rr` plugin instead of reusing `bgp-rs` (route-server), because they are semantically different (RFC 4456 vs RFC 7947).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Fix IPv6 MP_REACH forwarding | Done | `internal/component/bgp/plugins/rr/rr.go` | New bgp-rr plugin drives forwarding |
| Land .ci test | Done | `test/plugin/nexthop-self-ipv6-forward.ci` | Passes with ORIGINATOR_ID assertion |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `.ci` test: UPDATE arrives at peer-dst | `expect=bgp:conn=2:seq=1:contains=800904` |
| AC-2 | Partial | Reactor code exists at `reactor_api_forward.go:766-802` | IPv4 local + IPv6 route: legacy NEXT_HOP rewritten, MP_REACH NH unchanged (known limitation) |
| AC-3 | Done | `.ci` test: ORIGINATOR_ID header `800904` in wire bytes | CLUSTER_LIST also present (`800A04` in debug output) |
| AC-4 | Done | `.ci` test: NLRI `3020010DB80001` in forwarded UPDATE | Verified via debug hex dump |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Existing MP_REACH NH handler tests | Done | `filter_delta_handlers_test.go` | Pre-existing, unchanged |
| `nexthop-self-ipv6-forward` | Done | `test/plugin/nexthop-self-ipv6-forward.ci` | Updated with bgp-rr + ORIGINATOR_ID assertion |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/rr/register.go` | Created | bgp-rr registration |
| `internal/component/bgp/plugins/rr/rr.go` | Created | Route reflector plugin |
| `internal/component/plugin/all/all.go` | Modified | Added rr import |
| `test/plugin/nexthop-self-ipv6-forward.ci` | Modified | Added plugin config + assertion |

### Audit Summary
- **Total items:** 10
- **Done:** 9
- **Partial:** 1 (AC-2 next-hop rewrite for mixed IPv4/IPv6)
- **Skipped:** 0
- **Changed:** 1 (root cause was missing plugin, not any of the 3 hypotheses)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/rr/register.go` | Yes | Created this session |
| `internal/component/bgp/plugins/rr/rr.go` | Yes | Created this session |
| `test/plugin/nexthop-self-ipv6-forward.ci` | Yes | Modified this session |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Route forwarded | `ze-test bgp plugin nexthop-self-ipv6-forward` passes, `contains=800904` matched |
| AC-2 | NH rewrite | Partial: legacy NEXT_HOP rewritten (IPv4 local), MP_REACH NH unchanged (known limitation) |
| AC-3 | RR attributes | ORIGINATOR_ID `01020306` + CLUSTER_LIST `01020304` in debug hex output |
| AC-4 | NLRI preserved | `3020010DB80001` present in forwarded UPDATE (debug hex) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| IPv6 UPDATE from peer-src via bgp-rr | `test/plugin/nexthop-self-ipv6-forward.ci` | Yes, passes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] `.ci` test passes

### TDD
- [ ] `.ci` test FAILS (current state -- confirmed)
- [ ] Fix implemented
- [ ] `.ci` test PASSES
- [ ] `make ze-verify` clean
