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
| 6 | `mandatory-attr-missing` | ORIGIN, AS_PATH, or NEXT_HOP absent (legacy NLRI requires NEXT_HOP even when MP_REACH is present) | 4271 / 7606 | `message/rfc7606.go` |
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
(draft-mangin-idr-attr-tombstone-00) but the route itself continues.

### Received NEXT_HOP

The legacy IPv4 NEXT_HOP must encode a unicast host address. Invalid length
or address syntax uses RFC 7606 treat-as-withdraw.

RFC 4271 Section 6.3 also rejects any address assigned to the receiving speaker.
On directly connected eBGP, a third-party next hop must share a subnet with
the receiver; the sender's TCP address is accepted. The receive path uses an
immutable interface snapshot captured at connection setup and replaced on
interface address events. It performs no kernel lookup per UPDATE.

A semantic failure withdraws the legacy announcements while preserving explicit
withdrawals and MP routes in the same UPDATE. It is logged without a NOTIFICATION
or session reset. iBGP and multihop eBGP still reject the speaker's own addresses,
but do not apply the one-hop common-subnet condition.

<!-- source: internal/component/bgp/reactor/session_next_hop.go -- invalidReceiveNextHop, withdrawLegacyAnnouncements -->
<!-- source: internal/component/bgp/reactor/reactor_iface.go -- refreshPeerLinkScopes -->

### RPKI Validation

RPKI validation is asynchronous. `validation-timeout` defaults to 30 seconds and
accepts 1 through 65535 seconds. Undecided routes wait separately from eligible
paths; expiry promotes them with state `not-validated`. A retained rejected path
cannot become eligible through timeout, and a known newer UPDATE keeps its
predecessor fenced until that generation arrives or the path is removed.

| State | Value | Meaning |
|-------|-------|---------|
| `not-validated` | 0 | Default or timeout (fail-open) |
| `valid` | 1 | Origin AS matches a covering VRP |
| `not-found` | 2 | No covering VRP exists |
| `invalid` | 3 | Covering VRP exists but no AS match |
| `pending` | 4 | Awaiting validation (internal only) |

The lookup state and policy decision are separate. RPKI policy can accept any
lookup result, while an ASPA rejection can retain the received path as ineligible
for selection and replay. A later accepting decision restores that generation
without another UPDATE.

## Phase 2: Best-Path Selection (RFC 4271 Section 9.1.2)

Eligible routes compete pairwise. The loser at each step gets tagged with the
step that eliminated it. Steps are evaluated in strict order; the first difference
decides.

| # | Reason | Rule | RFC | Notes |
|---|--------|------|-----|-------|
| 9 | `stale-deprioritized` | Route at or above depreference threshold loses to fresh route | 9494 | GR/LLGR stale-level; threshold = 2 |
| 10 | `lost-local-pref` | Highest LOCAL_PREF wins | 4271 | Default 100 if absent |
| — | `aigp` | A path carrying AIGP beats one without it; then the lowest received AIGP plus interior distance wins | 7311 | After LOCAL_PREF, before AS_PATH; unsigned sums saturate |
| 11 | `lost-as-path-length` | Shortest AS_PATH wins | 4271 | AS_SET counts as 1 |
| 12 | `lost-origin` | Lowest ORIGIN wins (IGP=0 < EGP=1 < INCOMPLETE=2) | 4271 | |
| 13 | `lost-med` | Lowest MED wins (same neighbor AS only) | 4271 | Compared only when first AS matches. Section 9.1.2.2 (c) gives a route that carries no MULTI_EXIT_DISC the lowest possible value, 0, so an absent attribute wins this step |
| 14 | `lost-ebgp-over-ibgp` | eBGP preferred over iBGP | 4271 | eBGP = PeerASN != LocalASN |
| 15 | `lost-igp-cost` | Lowest resolved interior distance to next-hop | 4271/7311 | Recursive BGP hops contribute received AIGP, not MED; unavailable distance is distinct from zero |
| 16 | `lost-router-id` | Lowest Router ID / ORIGINATOR_ID wins | 4271/4456 | Numeric IP comparison |
| 17 | `lost-cluster-list-length` | Shortest CLUSTER_LIST wins | 4456 | Section 9 inserts this between RFC 4271 steps f) and g). Counted in CLUSTER_IDs; an absent attribute counts zero. Unconditional |
| 18 | `lost-peer-address` | Lowest peer IP address wins (final tiebreak) | 4271 | Numeric IP comparison |
<!-- source: internal/component/bgp/plugins/rib/ -- best-path selection implementation -->

### Candidate Extraction

Before comparison, each route's attributes are extracted from pool handles into a
flat `Candidate` struct: LocalPref, AIGP, HasAIGP, IGPCost, ASPathLen, FirstAS,
Origin, MED, PeerASN, LocalASN, OriginatorIP, ClusterListEntries, PeerIP, PeerAddr,
StaleLevel. Router ID
and peer address comparisons use typed `netip.Addr` fields for zero-allocation
numeric ordering. `ClusterListEntries` counts CLUSTER_IDs rather than octets and
is `uint16`, because a CLUSTER_LIST can carry 16383 of them.

### Enforced Before Selection (Not Best-Path Reasons)

These checks are implemented, but as ingress filters that reject the UPDATE
before it reaches best-path selection, so they never appear as a reason in the
enum above:

- **AS loop detection** (own ASN in AS_PATH): rejected on ingress by the reactor
  loop filter on all sessions (RFC 4271 Section 9).
- **Cluster-list loop detection** (RFC 4456, own Router ID in CLUSTER_LIST):
  rejected on ingress by the same loop filter (iBGP sessions). A route that
  survives this filter still has its CLUSTER_LIST length compared at step 17.
- **Originator-ID loop detection** (RFC 4456, own Router ID as ORIGINATOR_ID):
  rejected on ingress by the same loop filter (iBGP sessions).
- **OTC mismatch** (RFC 9234, Only-To-Customer attribute validation): enforced
  by the bgp-role plugin.

<!-- source: internal/component/bgp/reactor/filter/loop.go -- ingress AS/cluster-list/originator-id loop filter -->
<!-- source: internal/component/bgp/plugins/role/otc.go -- RFC 9234 OTC validation -->

## AIGP session policy

`session aigp enabled` controls receipt and transmission per peer or group.
Without an explicit value, iBGP enables AIGP and eBGP disables it.
Malformed AIGP TLVs and attributes received on a disabled session are discarded
before the received UPDATE reaches the RIB or a forwarding consumer.

`session aigp originate` defaults to false. `link-metric` supplies a non-zero
distance for a peer link without an IGP, in units comparable to the domain's IGP.
`domain-as` lists the external ASes inside that administrative domain.
Origination is checked after export policy and requires this speaker as next hop.
Raw UPDATE injection uses the same enabled, originate, local-next-hop and
domain-AS checks; received-route forwarding follows the separate policy below.
The local-next-hop check applies to each announced section: legacy NLRI uses
NEXT_HOP, and MP_REACH uses all its decoded next-hop addresses. An unrelated
legacy attribute cannot authorize a remote MP next hop.

A disabled session strips AIGP at the final writer, including raw forwarding and
replay. Forwarding with an unchanged next hop preserves the received attribute;
next-hop-self adds the resolved interior distance or the configured source-link
metric, changing only the first metric TLV. Mixed legacy and MP announcements
are split before this edit, so each uses its governing received next hop.
Explicit withdrawals and unrelated attributes survive the split. Without a
non-zero distance, or when a recursive BGP next hop carries no AIGP, the forwarded
attribute is removed.

The received metric is retained separately from MED in best-path events and the
shared Loc-RIB. Selection resolves both legacy NEXT_HOP and MP_REACH next hops;
it reads AIGP from the RIB's pooled attribute framing. Recursive static routes
follow their next hop without adding the
static route's preference metric; terminal IGP or directly attached static routes
contribute their interior metric once. Route-install RPC carries this distinction
for forked producers. An unknown interior distance is never treated as a zero-cost
connected route.

The reactor retains received AIGP generations independently of the transient
UPDATE cache. It records a recipient only after that recipient's final UPDATE
passes egress processing and its buffered socket write succeeds. Routing changes
recompute advertisements only for those recipients. Replay uses the original
received metric and current egress policy under the original sender's authority.
Its distance comes from the next hop retained for that route, including IPv4
unicast received in MP_REACH. Reconstructing that route as legacy IPv4 replaces
any sibling NEXT_HOP from the stored attribute block with the route's own next hop.
Withdrawals, replacements and disconnects invalidate retained generations; queued
replays check the generation and destination session again before writing.

Live AIGP announcements and their later attribute-free withdrawals use the same
source FIFO. A source that has entered this path stays on it across settings
reloads. Live items also carry a session generation, checked before destination
writes, so reconnect or peer replacement cannot publish an old queued item.
A collector without a forwarding role acquires no recipients.

The RIB reads interior distances from the engine's registered `route-metrics`
RPC when it runs in a subprocess. A one-second revision poll invalidates its
next-hop cache and reruns selection, including recursive BGP metric changes.
Selected paths and withdrawals return through `route-install` and `route-remove`.
Failure of the mandatory metric feed terminates the subprocess connection.
AIGP derives received-UPDATE and state delivery to the RIB from every peer.
An explicit binding that omits these inputs is refused; no Adj-RIB-In replay
plugin or forwarding permission is added. The feature-only binding grants
neither sent-UPDATE nor refresh delivery. BGP-source redistribution and explicit
peer bindings retain their separate grants.

Origination uses configured metrics only. Ze does not automatically derive
origination policy from redistribution. ACCEPT_OWN re-import is disabled;
non-RD families still discard the community and all normal loop checks apply.

<!-- source: internal/component/bgp/reactor/aigp_readvertise.go -- retained sources and committed recipients -->
<!-- source: internal/component/bgp/plugins/rib/rib_remote.go -- forked metrics and selected-path publication -->
<!-- source: internal/component/plugin/server/dispatch_route_metrics.go -- registered metric RPC -->
<!-- source: internal/component/bgp/config/redistribute_binding.go -- mandatory selection inputs -->
<!-- source: internal/component/bgp/reactor/forward_aigp.go -- forwardUpdateSelected, aigpNextHop -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRS -->

<!-- source: internal/component/bgp/reactor/config_aigp.go -- per-session AIGP policy -->
<!-- source: internal/component/bgp/reactor/session_validation.go -- AIGP receive boundary -->

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
| 17 | `lost-cluster-list-length` | Selection | 4456 |
| 18 | `lost-peer-address` | Selection | 4271 |

## Implementation Notes

- **Type:** `uint8` -- 19 values (0-18), extensible up to 255.
- **One field, not two:** Biorouting splits this into `HiddenReason` (validation) and
  implicit sort order (selection). Ze uses one field because both give the same
  reason: why the route is not the best.
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
