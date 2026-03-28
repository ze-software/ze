# Route Selection

## Overview

Every route received by ze goes through two phases before it can become the best path
for a prefix. Phase 1 determines whether the route is valid and eligible. Phase 2
determines which eligible route wins. A route that fails at any step gets a reason
explaining why it was not selected.

## Design: Unified Rejection Reason

Each non-best route carries a single reason (`uint8`) recording why it was not selected.
The reason is set once: either during validation (the route is ineligible) or during
best-path comparison (the route lost to a better candidate). The winning route has
reason `none` (0).

This is a single mechanism, not two separate ones. Whether a route was disqualified
before the race or lost at step N of the race, the answer is the same type of value
on the same field.

## Phase 1: Validation

Routes that fail validation never enter best-path selection.

| # | Reason | Check | RFC | Location |
|---|--------|-------|-----|----------|
| 1 | `nlri-syntax-invalid` | NLRI prefix length exceeds remaining bytes | 7606 | `message/rfc7606.go` |
| 2 | `attr-structure-malformed` | Path attribute header/length out of bounds | 7606 | `message/rfc7606.go` |
| 3 | `duplicate-mp-reach` | Multiple MP_REACH_NLRI or MP_UNREACH_NLRI | 7606 | `message/rfc7606.go` |
| 4 | `attr-flags-invalid` | Well-known attribute missing Transitive or has Optional | 7606 | `message/rfc7606.go` |
| 5 | `attr-value-invalid` | Per-attribute validation (ORIGIN range, AS_PATH structure, NEXT_HOP format, length checks for MED/LOCAL_PREF/AGGREGATOR/COMMUNITY/ORIGINATOR_ID/CLUSTER_LIST/EXT_COMMUNITY/LARGE_COMMUNITY, MP_REACH/MP_UNREACH structure) | 7606 | `message/rfc7606.go` |
| 6 | `mandatory-attr-missing` | ORIGIN, AS_PATH, or NEXT_HOP absent (when required) | 4271 | `message/rfc7606.go` |
| 7 | `family-not-negotiated` | MP_REACH/MP_UNREACH AFI/SAFI not in OPEN capabilities | 4271 | `reactor/session_validation.go` |
| 8 | `as-loop` | Local ASN found in AS_PATH (AS_SEQUENCE or AS_SET) | 4271 S9 | Not yet implemented |
| 9 | `originator-id-loop` | ORIGINATOR_ID matches local Router ID (iBGP only) | 4456 S8 | Not yet implemented |
| 10 | `cluster-list-loop` | Local Router ID found in CLUSTER_LIST (iBGP only) | 4456 S8 | Not yet implemented |
| 11 | `rpki-invalid` | Origin AS does not match any covering VRP | 6811 | `plugins/adj_rib_in/rib_validation.go` |
<!-- source: internal/component/bgp/message/rfc7606.go -- RFC 7606 validation checks -->
<!-- source: internal/component/bgp/reactor/session_validation.go -- family negotiation check (validateUpdateFamilies) -->

### RFC 7606 Error Escalation

Validation collects all errors and applies the strongest action:

| Action | Strength | Effect |
|--------|----------|--------|
| `none` | 0 | Route accepted |
| `attribute-discard` | 1 | Malformed attribute removed, route continues |
| `treat-as-withdraw` | 2 | Entire UPDATE treated as withdrawal |
| `session-reset` | 3 | NOTIFICATION sent, session closed |

Multiple errors in one UPDATE do not produce multiple reasons. The strongest action
determines the outcome. Attribute-discard marks the specific attribute in-place
(draft-mangin-idr-attr-discard-00) but the route itself continues.

### RPKI Validation

RPKI validation is asynchronous with a 30-second fail-open timeout. Routes pending
validation are held in a separate map. On timeout, the route is promoted with state
`not-validated` and enters best-path selection normally.

| State | Value | Meaning |
|-------|-------|---------|
| `not-validated` | 0 | Default or timeout (fail-open) |
| `valid` | 1 | Origin AS matches a covering VRP |
| `not-found` | 2 | No covering VRP exists |
| `invalid` | 3 | Covering VRP exists but no AS match |
| `pending` | 4 | Awaiting validation (internal only) |

Only state 3 (`invalid`) produces a rejection reason. States 0, 1, and 2 allow the
route to proceed to best-path selection.

## Phase 2: Best-Path Selection (RFC 4271 Section 9.1.2)

Eligible routes compete pairwise. The loser at each step gets tagged with the
step that eliminated it. Steps are evaluated in strict order; the first difference
decides.

| # | Reason | Rule | RFC | Notes |
|---|--------|------|-----|-------|
| 9 | `stale-deprioritized` | Route at or above depreference threshold loses to fresh route | 9494 | GR/LLGR stale-level; threshold = 2 |
| 10 | `lost-local-pref` | Highest LOCAL_PREF wins | 4271 | Default 100 if absent |
| 11 | `lost-as-path-length` | Shortest AS_PATH wins | 4271 | AS_SET counts as 1 |
| 12 | `lost-origin` | Lowest ORIGIN wins (IGP=0 < EGP=1 < INCOMPLETE=2) | 4271 | |
| 13 | `lost-med` | Lowest MED wins (same neighbor AS only) | 4271 | Compared only when first AS matches |
| 14 | `lost-ebgp-over-ibgp` | eBGP preferred over iBGP | 4271 | eBGP = PeerASN != LocalASN |
| 15 | `lost-igp-cost` | Lowest IGP cost to next-hop | 4271 | Not yet implemented |
| 16 | `lost-router-id` | Lowest Router ID / ORIGINATOR_ID wins | 4271/4456 | Numeric IP comparison |
| 17 | `lost-peer-address` | Lowest peer IP address wins (final tiebreak) | 4271 | Numeric IP comparison |
<!-- source: internal/component/bgp/plugins/rib/ -- best-path selection implementation -->

### Candidate Extraction

Before comparison, each route's attributes are extracted from pool handles into a
flat `Candidate` struct: LocalPref, ASPathLen, FirstAS, Origin, MED, PeerASN,
LocalASN, OriginatorID, PeerAddr, StaleLevel. The comparison functions operate on
these extracted values with no pool dependency.

### Not Yet Implemented

- **AS loop detection** (own ASN in AS_PATH): biorouting checks this in Adj-RIB-In.
  Ze does not currently reject routes with AS loops before selection.
- **Cluster-list loop detection** (RFC 4456, own Cluster ID in CLUSTER_LIST):
  not currently checked.
- **Originator-ID loop detection** (RFC 4456, own Router ID as ORIGINATOR_ID):
  not currently checked.
- **OTC mismatch** (RFC 9234, Only-To-Customer attribute validation):
  not currently checked.
- **IGP cost to next-hop** (step 15): requires IGP integration.

These would be additional validation reasons in Phase 1 when implemented.

## Complete Reason Table

All reasons in evaluation order. A route gets exactly one reason: the first check
it fails.

| Value | Reason | Phase | RFC |
|-------|--------|-------|-----|
| 0 | `none` | - | - |
| 1 | `nlri-syntax-invalid` | Validation | 7606 |
| 2 | `attr-structure-malformed` | Validation | 7606 |
| 3 | `duplicate-mp-reach` | Validation | 7606 |
| 4 | `attr-flags-invalid` | Validation | 7606 |
| 5 | `attr-value-invalid` | Validation | 7606 |
| 6 | `mandatory-attr-missing` | Validation | 4271 |
| 7 | `family-not-negotiated` | Validation | 4271 |
| 8 | `rpki-invalid` | Validation | 6811 |
| 9 | `stale-deprioritized` | Selection | 9494 |
| 10 | `lost-local-pref` | Selection | 4271 |
| 11 | `lost-as-path-length` | Selection | 4271 |
| 12 | `lost-origin` | Selection | 4271 |
| 13 | `lost-med` | Selection | 4271 |
| 14 | `lost-ebgp-over-ibgp` | Selection | 4271 |
| 15 | `lost-igp-cost` | Selection | 4271 |
| 16 | `lost-router-id` | Selection | 4271/4456 |
| 17 | `lost-peer-address` | Selection | 4271 |

## Implementation Notes

- **Type:** `uint8` -- 18 values (0-17), extensible up to 255.
- **One field, not two:** Biorouting splits this into `HiddenReason` (validation) and
  implicit sort order (selection). Ze uses one field because both answer the same
  question: "why is this route not the best?"
- **String conversion:** Only on JSON output. Internal representation is always `uint8`.
- **Cost:** One byte per route entry. Set once during validation or selection, never
  updated after.

## Related Documentation

- `docs/architecture/route-types.md` -- route struct inventory and data flow
- `docs/architecture/core-design.md` -- reactor, FSM, wire layer
- `docs/architecture/wire/messages.md` -- BGP message parsing
- `docs/architecture/plugin/rib-storage-design.md` -- RIB storage internals
- `rfc/short/rfc4271.md` -- BGP-4 specification
- `rfc/short/rfc7606.md` -- revised error handling for UPDATE messages
- `rfc/short/rfc6811.md` -- RPKI-based origin validation
