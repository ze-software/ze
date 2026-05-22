# Spec: IXP Route Server

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 9/15 |
| Updated | 2026-05-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/reactor/reactor_connection.go` - incoming connection routing
4. `internal/component/bgp/reactor/forward_rs.go` - RS fast-path forwarding (AS-path prepending at line 316)
5. `internal/component/bgp/config/resolve.go` - 3-layer config inheritance
6. `internal/component/bgp/schema/ze-bgp-conf.yang` - peer/group YANG schema
7. `internal/component/bgp/plugins/rs/server.go` - route server plugin
8. `internal/component/bgp/plugins/rs/server_forward.go` - `selectForwardTargets` (forward-all, line 106)

## Task

Make Ze a production-grade IXP route server. Four features:

1. **Dynamic peer groups**: Group-level `ip dynamic` + `range` that accepts BGP sessions
   from any IP in a prefix range. Peer ASN learned from OPEN. No individual peer config needed.

2. **Config variable substitution**: `$remote_as`, `$local_as`, `$remote_ip` in route
   attributes and filter names, resolved per-peer at session establishment.

3. **Transparent AS-path (RS client mode)**: RFC 7947 Section 2.2.2 requires the route
   server MUST NOT modify AS_PATH. Currently `forward_rs.go:316` prepends LocalAS for
   eBGP peers. A new RS-client mode skips AS-path prepending while keeping eBGP next-hop
   and loop detection semantics.

4. **Community-based selective forwarding**: RS clients use well-known communities to
   control route distribution. `selectForwardTargets` (server_forward.go:106) currently
   does forward-all. Must support: do-not-announce-to-peer, announce-only-to-peer,
   prepend-N-times-to-peer, and blackhole (RFC 7999).

### What Ze already has (not in scope)

| Capability | Plugin/Feature | Status |
|-----------|---------------|--------|
| RPKI origin validation | `bgp-rpki` (RTR, ROA, ASPA) | Done |
| Prefix-list filtering | `bgp-filter-prefix` | Done |
| AS-path regex filtering | `bgp-filter-aspath` | Done |
| Community tag/strip | `bgp-filter-community` | Done |
| Community match/reject | `bgp-filter-community-match` | Done |
| Route attribute modification | `bgp-filter-modify` | Done |
| ADD-PATH send/receive | Capability negotiation | Done |
| Per-peer prefix limits | RFC 4486 in `session_prefix.go` | Done |
| RFC 9234 BGP roles | `bgp-role` (rs_server/rs_client) | Done |
| Next-hop unchanged | YANG `session/next-hop unchanged` | Done |
| Passive connection mode | YANG `connection/local/accept` | Done |
| Bogon/martian filtering | Use existing `prefix-list` filter with IANA bogon entries | Config, not code |

Motivation: IXPs (LINX, AMS-IX, DE-CIX) run route servers that handle hundreds of
peers with templated policies, community-based forwarding control, and transparent
AS-path handling. Ze has most building blocks but is missing the four features above.

Reference: RFC 7947 (BGP Route Server), RFC 7948 (Route Server Operational Considerations),
RFC 7999 (BLACKHOLE Community).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor, peer lifecycle, plugin architecture
  → Decision: [TBD]
  → Constraint: [TBD]
- [ ] `docs/architecture/config/syntax.md` - config tree, resolution, YANG
  → Decision: [TBD]
  → Constraint: [TBD]

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7947.md` - BGP Route Server (if exists, create if not)
  → Constraint: [TBD]
- [ ] `rfc/short/rfc7948.md` - Route Server Operational Considerations (if exists, create if not)
  → Constraint: [TBD]

**Key insights:** (summary of all checkpoint lines)
- [TBD after reading]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_connection.go` - incoming connection routing; `handleConnection` (line 30) looks up peer by remote IP via `findPeerByAddr`; unknown IPs are silently dropped (`closeConnQuietly`)
  → Constraint: Dynamic peer creation must happen in this path, between `findPeerByAddr` returning false and `closeConnQuietly`
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - `findPeerByAddr` (line 35) does fast-path lookup by addr+DefaultBGPPort in `r.peers` map, then slow-path iteration by IP
  → Constraint: Dynamic group matching is a new third lookup path after both existing paths fail
- [ ] `internal/component/bgp/config/resolve.go` - `ResolveBGPTree` (line 39) applies 3-layer inheritance (bgp globals, group, peer); outputs flat `peerMap`; validates unique remote IPs and peer names
  → Constraint: Dynamic peers bypass this at config load time; resolution must happen at connection time
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - group (line 125) and peer YANG schema; `connection/remote/ip` is type `zt:ip-address`; `connection/local/accept` (line 290) and `connection/remote/connect` (line 307) control connection mode
  → Constraint: Add `dynamic` enum to `connection/remote/ip` union; add `range` (ip-prefix) and `max-peers` (uint32) leaves under `connection/remote`. `ip dynamic` at peer level rejected by programmatic validation. Dynamic peers bypass `parsePeerFromTree` (which hard-rejects PeerAS=0 and empty IP); need separate PeerSettings construction path.
- [ ] `internal/component/bgp/reactor/peersettings.go` - `PeerSettings` struct (line 73+) with Address, LocalAS, PeerAS, Connection mode, filters, static routes
  → Constraint: Dynamic peers get PeerSettings with PeerAS filled from OPEN, Address from TCP connection
- [ ] `internal/component/bgp/plugins/rs/server.go` - Route server plugin (bgp-rs) with forward-all model, per-source workers, backpressure. Handles RFC 7947 semantics.
  → Constraint: RS plugin is peer-agnostic (works on any peer); dynamic peers are transparent to it
- [ ] `internal/component/bgp/config/peers.go` - `PeersFromConfigTree` and route extraction; filter chain attachment
  → Constraint: Variable substitution in filter names and route attributes must happen after ASN is known
- [ ] `internal/component/bgp/config/bgp_routes.go` - static route parsing; as-path is `leaf-list` of strings (YANG line 235)
  → Constraint: `$remote_as` in as-path values must be resolved to actual ASN at session time
- [ ] `internal/component/bgp/reactor/forward_rs.go` - `reactorForwardRS` (line 81) is the RS fast-path. Line 316: `if peer.Settings().IsEBGP()` triggers AS-path prepending via `getEBGPWire`. For RS-client peers this must be skipped.
  → Constraint: RS-client mode needs a flag on PeerSettings that `reactorForwardRS` checks to skip the eBGP wire rewrite
- [ ] `internal/component/bgp/plugins/rs/server_forward.go` - `selectForwardTargets` (line 106) does forward-all (up + not-source + family match). No community inspection.
  → Constraint: Community-based selective forwarding must inspect COMMUNITY/LARGE_COMMUNITY attributes in the UPDATE wire bytes during target selection. This is on the hot path; must be zero-alloc.

**Behavior to preserve:**
- Explicit peer configuration continues to work unchanged
- 3-layer config inheritance (bgp, group, peer) unchanged for explicit peers
- Route server plugin (`bgp-rs`) unchanged; dynamic peers are just peers to it
- Connection collision detection (RFC 4271 Section 6.8) works for dynamic peers
- Peer name uniqueness within a group

**Behavior to change:**
- `connection/remote/ip` gains `dynamic` enum value; new `connection/remote/range` leaf for prefix
- `ip dynamic` + `range` on a group enables dynamic peer mode: connections from the range create peers
- String config values gain `$remote_as`, `$local_as`, `$remote_ip` variable substitution (works for both dynamic and explicit peers)
- Reactor `handleConnection` and `handleConnectionWithContext` check dynamic groups when `findPeerByAddr` returns false

## Data Flow (MANDATORY)

### Entry Point
- TCP connection arrives on BGP listener from an IP within a group's `connection/remote/range`
- No explicit peer config exists for this IP

### Transformation Path
1. **Listener** accepts TCP connection, calls `handleConnection(conn)` or `handleConnectionWithContext(conn, localAddr)` (reactor_connection.go:30/55)
2. **Reactor** calls `findPeerByAddr(remoteIP)` (reactor_peers.go:35) -- no match
3. **NEW: Reactor** calls `findDynamicGroup(remoteIP)` -- checks groups with `ip dynamic` whose `range` contains remoteIP (longest-prefix-match if overlapping)
4. **NEW: Reactor** builds `PeerSettings` directly (NOT through `parsePeerFromTree`, which rejects PeerAS=0 and empty IP). Uses the group's resolved template fields with:
   - Address = remoteIP (from TCP connection)
   - Connection = passive only (forced)
   - PeerAS = 0 (unknown until OPEN)
   - IsDynamic = true, GroupName = source group
   - All other fields (filters, capabilities, timers) from group template
5. **NEW: Reactor** registers the dynamic peer in `r.peers` map; increments group's dynamic peer count
6. **Reactor** proceeds with `acceptOrReject(conn, peer, cb)` as normal
7. **Session FSM** receives OPEN message via `handleOpen` (session_handlers.go:39); learns remote ASN. Note: reactor does NOT validate PeerAS against OPEN (no `NotifyOpenBadPeerAS` check exists). FSM transitions through OpenConfirm.
8. **FSM transitions to Established** (peer_run.go:320). Inside the callback, BEFORE `sendInitialRoutes` is spawned:
   - **NEW: Set PeerAS** on the shared PeerSettings struct: `p.settings.PeerAS = asnFromOpen` (mutate in place, visible to both Peer and Session)
   - **NEW: Resolve variables** in static route attributes and filter chains: create new slices with `$remote_as`/`$remote_ip` resolved, assign to `p.settings.StaticRoutes` and `p.settings.ExportFilters`/`ImportFilters`
9. `go p.sendInitialRoutes()` spawned (peer_run.go:367); reads resolved `p.settings.StaticRoutes`
10. RS plugin sees the dynamic peer as a normal peer

### Reconnection (no special handling needed)
- On disconnect, dynamic peer stays in `r.peers` during idle cleanup timeout
- Fast reconnect: `findPeerByAddr` finds existing peer; `acceptOrReject` handles the tearing-down case via `SetInboundConnection` (reactor_connection.go:144-146)
- After idle timeout expires without reconnection: peer removed from `r.peers`, group's dynamic peer count decremented

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config resolution -> Reactor | Group templates stored as `DynamicGroupConfig` in reactor | [ ] |
| TCP accept -> Peer creation | `findDynamicGroup` returns group template; reactor creates peer | [ ] |
| Established transition -> Variable resolution | peer_run.go:320 callback mutates PeerAS + replaces StaticRoute/filter slices on shared PeerSettings | [ ] |

### Integration Points
- `Reactor.handleConnection` (reactor_connection.go:30) -- insertion point for dynamic peer creation
- `ResolveBGPTree` (resolve.go:39) -- must export group templates with `ip dynamic` + `range` for reactor
- Established callback (peer_run.go:320) -- resolution point: set PeerAS, resolve variables before sendInitialRoutes
- `bgp-rs` plugin (server.go) -- transparent; sees dynamic peers as normal peers

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Design Decisions

### D1: Dynamic Peer Lifecycle

Dynamic peers are created when a connection arrives and removed when the session
closes (peer goes to Idle and no reconnection is expected). Unlike explicit peers,
dynamic peers do not attempt reconnection. If the remote disconnects, the dynamic
peer is cleaned up after an idle timeout.

The idle timeout reuses the existing `timer/connect-retry` value from the group
template (default 120s per YANG). When a dynamic peer's session goes down, the
peer stays in `r.peers` for connect-retry seconds. If no reconnection arrives,
the peer is removed and the group's dynamic peer count decremented. During this
window, a reconnection from the same IP hits `findPeerByAddr` (existing peer)
and reuses the peer object via `SetInboundConnection`.

A `max-peers` limit on the group prevents resource exhaustion from the dynamic range.

### D2: Variable Resolution Timing

Variables are resolved in two phases:
- **Config load time**: `$local_as` is known and resolved immediately. Note: `$local_as`
  resolves to the effective local-AS for that peer (which may be overridden at the peer
  level via `session/asn/local`), not necessarily the global BGP local-AS.
- **Established transition**: `$remote_as` and `$remote_ip` are resolved inside the
  Established callback (peer_run.go:320), BEFORE `sendInitialRoutes` is spawned
  (peer_run.go:367). The ASN is read from `session.PeerOpen()`.

For dynamic peers, `$remote_ip` is known at connection time (from TCP) and set on
PeerSettings immediately. `$remote_as` is known after OPEN processing but resolved
at Established (when PeerAS is set on the shared struct).

For explicit peers, `$remote_as` resolves to the configured `session/asn/remote`
value. Resolution can happen at config load time since the value is already known.

Variables are ONLY valid in:
- Static route attributes: as-path, community, large-community, extended-community
- Filter chain names

Variables are NOT valid in structural config fields (IP addresses, ASN numbers,
ports, booleans, timer values). A `$` token in a structural field is a literal
string, not a variable reference.

### D3: Config Syntax -- `ip dynamic` + `range`

Two additions to the existing `connection/remote` container:

1. **`ip dynamic`**: Add `dynamic` as an enum value to the existing `connection/remote/ip`
   union type (alongside `zt:ip-address`). At the group level, `ip dynamic` signals
   "accept peers from the range." At the peer level, `dynamic` is rejected by
   programmatic validation (peers must have a concrete IP).

2. **`range`**: New leaf-list `connection/remote/range` of type `zt:ip-prefix` (new typedef
   combining `zt:prefix-ipv4` and `zt:prefix-ipv6`). Accepts multiple ranges (e.g., one
   IPv4 peering LAN + one IPv6 peering LAN). Only meaningful when `ip dynamic` is set.
   Validated: rejected if `ip` is not `dynamic`; at least one entry required if `ip` is
   `dynamic`.

3. **`max-peers`**: New leaf `connection/remote/max-peers` (uint32, default 1000).
   Limits dynamic peer count per group.

No changes to the shared `peer-fields` grouping type for `ip`. The `dynamic` enum
is part of the existing union; the `range` and `max-peers` leaves are new siblings.

Example:
```
group ix-peers {
    connection {
        remote {
            ip dynamic;
            connect false;
            range 185.1.69.0/24;
            range 2001:7f8:4::0/64;
            max-peers 500;
        }
    }
}
```

Config validation rules:
- `ip dynamic` without `range` at group level -> error
- `ip dynamic` at peer level -> error
- `range` at group level without `ip dynamic` -> error
- `range` inherited into an explicit peer (via deepMergeMaps from a dynamic group) -> silently ignored. This happens naturally when a group has `ip dynamic` + `range` and also contains explicit peers that override `ip` with a concrete address. The explicit peer inherits `range` but since its `ip` is not `dynamic`, the `range` is inert.
- `connect true` (the default) at group level when `ip dynamic` is set -> error. Dynamic groups must set `connect false` explicitly (Ze principle: explicit > implicit).

Variable syntax uses `$` prefix (matching ARouteServer convention):
- `$remote_as` -- peer's ASN (from config or OPEN; works for explicit peers too)
- `$local_as` -- local ASN
- `$remote_ip` -- peer's IP address (string form, not numeric)

### D3b: Overlapping Group Ranges

When multiple groups have overlapping `range` values, longest-prefix-match wins.
Group A with `range 185.1.69.0/25` beats group B with `range 185.1.69.0/24` for
IP `185.1.69.10`. If two groups have identical ranges, config validation rejects
(ambiguous).

### D3c: PeerSettings Construction for Dynamic Peers

Dynamic peers bypass `parsePeerFromTree` (reactor/config.go:47) because it
hard-rejects PeerAS=0 (line 62) and empty remote IP (line 75). Instead, dynamic
peer creation builds PeerSettings directly from the group's resolved template:

1. Config load: `ResolveBGPTree` resolves group-level fields (Layer 1 + Layer 2 merge)
   and exports a `DynamicGroupTemplate` containing the resolved map + parsed range
2. Connection time: reactor creates a new `PeerSettings` from the template, sets
   Address from TCP, PeerAS=0, IsDynamic=true, Connection=passive-only
3. OPEN time: creates a copy of PeerSettings with PeerAS set from OPEN, then runs
   variable substitution on the copy. The copy replaces the original (immutability
   preserved: no mutation of shared state)

### D3d: Variable Resolution and PeerSettings Shared Pointer

Peer and Session share the same `*PeerSettings` pointer (`NewSession(settings)`
stores it as `s.settings`, session.go:343). Swapping the Peer's pointer would
leave the Session with stale values (PeerAS=0, wrong iBGP detection in
`enforceRFC7606` at session_validation.go:73).

Resolution mutates the shared struct in place:
- `p.settings.PeerAS = asnFromOpen` (uint32 field, visible to both Peer and Session)
- `p.settings.StaticRoutes = resolvedCopy` (new slice assigned to same struct field)
- Filter chains: same pattern (new slice, not element mutation)

The immutability contract on StaticRoute ELEMENTS is preserved: we replace the
slice, not modify its elements. The template's original slices are not touched.

**Resolution point**: inside the Established transition callback (peer_run.go:320),
BEFORE `go p.sendInitialRoutes()` (peer_run.go:367). The ASN is available via
`session.PeerOpen()`. No race: the callback runs in the peer's runOnce goroutine,
and sendInitialRoutes is spawned after resolution completes.

### D4: OPEN Validation for Dynamic Peers

When a dynamic peer sends an OPEN, the reactor must validate:
- The ASN is not zero
- The ASN is not the local AS (unless allow-own-as is configured on the group)
- The OPEN is otherwise valid per RFC 4271

The ASN from OPEN becomes the authoritative `PeerAS` for the dynamic peer.
This is when `$remote_as` variables in the peer's config are resolved.

### D5: Dynamic Peer Naming

Dynamic peers are auto-named: `dyn-<remote-ip>` (e.g., `dyn-185.1.69.42`).
This avoids collision with explicit peer names (which cannot be IP addresses
per `validatePeerName`). The `dyn-` prefix is reserved in peer name validation.

### D6: Show/CLI Visibility

Dynamic peers appear in `show peer list` with a `[dynamic]` flag.
`show peer detail <name>` works for dynamic peers.
The group's `ip dynamic` + `range` config is visible in `show bgp config`.

### D7: Config Reload

On config reload:
- Existing dynamic peers whose group is removed are torn down
- Existing dynamic peers whose group's prefix range changes: peers outside
  the new range are torn down; peers still in range keep their sessions
- New prefix ranges become active immediately

### D8: Transparent AS-path (RS Client Mode)

RFC 7947 Section 2.2.2: "The route server MUST NOT modify the AS_PATH or any
other transitive attribute attached to a route."

Currently `reactorForwardRS` (forward_rs.go:316) prepends LocalAS for eBGP
peers via `getEBGPWire`. For RS-client peers, this must be skipped.

**Implementation**: New boolean field `RSClient` on PeerSettings. When true:
- `reactorForwardRS` skips the eBGP wire rewrite block (line 316-331):
  uses the received wire bytes as-is (no AS-path prepending)
- Next-hop is still governed by `session/next-hop` (typically `unchanged` for RS)
- Loop detection still works: the source peer's ASN is in the AS-path already
  (put there by the source, not by the RS)

**YANG config**: New leaf `session/rs-client` (boolean, default false). When true,
implies transparent AS-path. Can be set at group level for dynamic peers.

```
group ix-peers {
    session {
        asn { local 65000; }
        next-hop unchanged;
        rs-client true;
    }
    connection {
        remote { ip dynamic; connect false; range 185.1.69.0/24; }
    }
}
```

**ForwardUpdate path** (reactor_api_forward.go): The standard ForwardUpdate path
also prepends LocalAS for eBGP. When the destination peer has `RSClient=true`,
skip the AS-path rewrite there too. Both the fast path (forward_rs.go) and the
standard path (reactor_api_forward.go) must check `RSClient`.

### D9: Community-Based Selective Forwarding

RS clients attach communities to their routes to control distribution.
The RS inspects these communities during `selectForwardTargets` and applies
the policy before forwarding.

**Community scheme** (matches Euro-IX / ARouteServer convention):

| Community | Meaning |
|-----------|---------|
| `0:<peer-ASN>` | Do NOT announce to peer with this ASN |
| `<RS-ASN>:<peer-ASN>` | Announce ONLY to peer with this ASN (whitelist mode) |
| `<RS-ASN>:0` | Do not announce to any peer (blackhole at RS) |
| `65535:666` | BLACKHOLE (RFC 7999): forward with next-hop rewritten to blackhole address |
| `65535:0` | GRACEFUL_SHUTDOWN (RFC 8326): accepted, reduced local-pref |

**Large community scheme** (for prepending):

| Large Community | Meaning |
|----------------|---------|
| `<RS-ASN>:101:<peer-ASN>` | Prepend source ASN 1x when forwarding to peer |
| `<RS-ASN>:102:<peer-ASN>` | Prepend source ASN 2x when forwarding to peer |
| `<RS-ASN>:103:<peer-ASN>` | Prepend source ASN 3x when forwarding to peer |
| `<RS-ASN>:101:0` | Prepend source ASN 1x to ALL peers |
| `<RS-ASN>:102:0` | Prepend source ASN 2x to ALL peers |
| `<RS-ASN>:103:0` | Prepend source ASN 3x to ALL peers |

**Implementation**: Community inspection hooks into two places:

1. **`selectForwardTargets`** (server_forward.go:106): Before adding a peer to
   the target list, check the UPDATE's communities. If `0:<peer-ASN>` is present,
   skip. If any `<RS-ASN>:<peer-ASN>` exists, switch to whitelist mode (only
   announce to listed peers). This requires parsing COMMUNITY attributes from
   the wire bytes. Since this is on the hot path, use zero-copy iteration
   (attribute.NewCommunityIterator) over the wire bytes.

2. **Per-target prepending** (in the forward loop): After target selection, if
   a large community `<RS-ASN>:10N:<peer-ASN>` matches, apply N prepends of
   the source peer's ASN to the forwarded wire bytes. This uses the existing
   `wireu.RewriteASPath` machinery but with the source ASN, not the RS's ASN.

**Configuration**: The RS ASN used for community matching is `session/asn/local`
from the group config. No additional config needed: the community scheme is
the standard Euro-IX convention, always enabled when `rs-client true`.

**Community stripping**: The RS MUST strip its own control communities before
forwarding. Routes leaving the RS should not carry `0:<ASN>` or `<RS-ASN>:<ASN>`
communities. The existing `bgp-filter-community` egress strip mechanism can be
used, or the RS plugin strips them in the forward path.

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| TCP connection from group's `range` prefix | -> | `Reactor.findDynamicGroup` + dynamic peer creation | `TestDynamicPeerCreation` |
| OPEN message on dynamic peer | -> | Variable resolution in PeerSettings | `TestVariableResolutionOnOpen` |
| Config with `$remote_as` in as-path | -> | Variable substitution in static routes | `TestVariableSubstitutionASPath` |
| Config reload removes dynamic group | -> | Dynamic peer teardown | `TestDynamicPeerTeardownOnReload` |
| `show peer list` | -> | Dynamic peer visibility with flag | `TestDynamicPeerCLIVisibility` |
| RS-client peer receives forwarded route | -> | AS-path NOT prepended by RS | `TestRSClientTransparentASPath` |
| UPDATE with `0:<peer-ASN>` community | -> | Route not forwarded to that peer | `TestCommunityDoNotAnnounce` |
| UPDATE with `<RS-ASN>:<peer-ASN>` community | -> | Route forwarded ONLY to that peer | `TestCommunityAnnounceOnly` |
| UPDATE with `<RS-ASN>:101:<peer-ASN>` large community | -> | Source ASN prepended 1x to that peer | `TestCommunityPrepend` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Group with `ip dynamic; range 185.1.69.0/24`; TCP connection from 185.1.69.42 | Dynamic peer created, BGP session establishes |
| AC-2 | Dynamic peer sends OPEN with AS 64512 | PeerAS set to 64512; `$remote_as` resolved in config |
| AC-3 | Group config has `as-path $remote_as` in update block | Static route AS-path contains actual remote ASN |
| AC-4 | Group config has `community $local_as:100` | Community attribute contains actual local ASN |
| AC-5 | Dynamic peer disconnects, no reconnection | Peer removed after idle timeout; resources cleaned up; group dynamic peer count decremented |
| AC-6 | `max-peers 10` on group; 11th connection from range | 11th connection rejected |
| AC-7 | Config reload removes the group | All dynamic peers from that group torn down |
| AC-8 | Explicit peer at 185.1.69.42 + group has `range 185.1.69.0/24` | Explicit peer takes precedence; no dynamic peer created |
| AC-9 | `show peer list` with dynamic peers | Dynamic peers shown with `[dynamic]` indicator |
| AC-10 | Dynamic peer + RS plugin loaded | RS plugin forwards routes from/to dynamic peers normally |
| AC-11 | Connection from IP outside all group ranges, no explicit peer | Connection dropped (existing behavior) |
| AC-12 | `$remote_ip` used in filter name or log context | Resolved to string form of peer IP (e.g., `185.1.69.42`); NOT valid in numeric community fields |
| AC-13 | Explicit peer with `as-path $remote_as` in update block | AS-path contains configured `session/asn/remote` value (variables work for explicit peers) |
| AC-14 | `ip dynamic` without `range` in config | Config validation error |
| AC-15 | `ip dynamic` at peer level (not group) | Config validation error |
| AC-16 | Two groups with overlapping ranges (`/24` and `/25`); connection from `/25` range | Longest-prefix-match: `/25` group wins |
| AC-17 | Group with `ip dynamic` and `connect true` (default) | Config validation error: `connect false` required for dynamic groups |
| AC-18 | `$remote_as` used in `connection/remote/ip` or `session/asn/remote` | Treated as literal string, not variable. Variables only valid in route attributes and filter names |
| AC-19 | Group with `ip dynamic; range 185.1.69.0/24; range 2001:7f8:4::0/64` | Both IPv4 and IPv6 ranges active; connections from either create dynamic peers |
| **Transparent AS-path** | | |
| AC-20 | RS-client peer (rs-client true) receives forwarded eBGP route | AS-path contains original AS-path only; RS ASN NOT prepended |
| AC-21 | RS-client peer receives forwarded route; next-hop unchanged | Next-hop preserves original (not rewritten to RS address) |
| AC-22 | Non-RS-client peer receives forwarded route | AS-path prepended with RS ASN (existing behavior preserved) |
| AC-23 | RS-client loop detection: route with local ASN in AS-path | Route still rejected by AS-path loop detection (transparency does not bypass loop check) |
| **Community-based selective forwarding** | | |
| AC-24 | Peer A sends route with community `0:64513`; peer B has ASN 64513 | Peer B does NOT receive the route; all other peers do |
| AC-25 | Peer A sends route with community `65000:64513` (RS ASN=65000) | ONLY peer B (ASN 64513) receives the route |
| AC-26 | Peer A sends route with communities `65000:64513` + `65000:64514` | Only peers B (64513) and C (64514) receive the route |
| AC-27 | Peer A sends route with community `65000:0` | No peer receives the route (blackholed at RS) |
| AC-28 | Peer A sends route with large community `65000:101:64513` | Peer B receives the route with source ASN prepended 1x in AS-path |
| AC-29 | Peer A sends route with large community `65000:103:0` | All peers receive the route with source ASN prepended 3x |
| AC-30 | Route forwarded by RS to peer B | RS control communities (`0:X`, `RS:X`) stripped before forwarding |
| AC-31 | Peer sends route with community `65535:666` (RFC 7999 BLACKHOLE) | Route forwarded to all peers with BLACKHOLE community preserved |
| AC-32 | Community-based forwarding + dynamic peer | Selective forwarding works with dynamic peers (uses learned ASN) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFindDynamicGroup` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | Prefix matching, multiple groups, overlap precedence | |
| `TestDynamicPeerCreation` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | Peer creation from template, correct settings | |
| `TestDynamicPeerMaxPeers` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | max-peers limit enforcement | |
| `TestDynamicPeerCleanup` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | Peer removal on disconnect | |
| `TestVariableSubstitution` | `internal/component/bgp/config/variables_test.go` | `$remote_as`, `$local_as`, `$remote_ip` replacement | |
| `TestVariableSubstitutionASPath` | `internal/component/bgp/config/variables_test.go` | AS-path leaf-list with variables | |
| `TestVariableSubstitutionCommunity` | `internal/component/bgp/config/variables_test.go` | Community string with variables | |
| `TestResolveBGPTreeWithDynamicGroup` | `internal/component/bgp/config/resolve_test.go` | Group templates with `ip dynamic` + `range` exported | |
| `TestDynamicPeerNaming` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | `dyn-<ip>` naming, uniqueness | |
| `TestExplicitPeerPrecedence` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | Explicit peer always beats dynamic group | |
| `TestVariableResolutionOnOpen` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | PeerAS set from OPEN before sendInitialRoutes; Session sees updated PeerAS; StaticRoutes resolved | |
| `TestDynamicGroupConnectFalseRequired` | `internal/component/bgp/config/resolve_test.go` | Config validation rejects `ip dynamic` with `connect true` | |
| `TestRangeInheritedByExplicitPeer` | `internal/component/bgp/config/resolve_test.go` | Explicit peer inside dynamic group inherits `range` silently; no validation error | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| max-peers | 1-100000 | 100000 | 0 | 100001 |
| range prefix length (IPv4) | /8-/30 | /30 | /7 | /31 |
| range prefix length (IPv6) | /16-/126 | /126 | /15 | /127 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-dynamic-peer-basic` | `test/bgp/dynamic-peer-basic.ci` | IX peer connects to RS, session establishes, routes forwarded | |
| `test-dynamic-peer-variables` | `test/bgp/dynamic-peer-variables.ci` | RS announces route with $remote_as prepended in AS-path | |
| `test-dynamic-peer-maxpeers` | `test/bgp/dynamic-peer-maxpeers.ci` | Connection rejected when max-peers exceeded | |
| `test-rs-transparent-aspath` | `test/bgp/rs-transparent-aspath.ci` | RS-client peer receives route without RS ASN in AS-path | |
| `test-rs-community-blacklist` | `test/bgp/rs-community-blacklist.ci` | Route with `0:<ASN>` not forwarded to that peer | |
| `test-rs-community-whitelist` | `test/bgp/rs-community-whitelist.ci` | Route with `<RS>:<ASN>` forwarded only to that peer | |
| `test-rs-community-prepend` | `test/bgp/rs-community-prepend.ci` | Source ASN prepended per large community instruction | |
| `test-rs-community-strip` | `test/bgp/rs-community-strip.ci` | Control communities stripped from forwarded routes | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dynamic-peer-bird` | `test/interop/scenarios/` | BIRD | Dynamic peer session with BIRD client, route exchange | |
| `dynamic-peer-frr` | `test/interop/scenarios/` | FRR | Dynamic peer session with FRR client | |

### Future (if deferring any tests)
- IRR filtering per dynamic peer (requires external data source: RIPE DB, RADb)
- PeeringDB max-prefix auto-lookup (requires PeeringDB API integration)
- Auto-maintained bogon prefix list (IANA special-purpose registry auto-update)

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` - add `dynamic` enum to `connection/remote/ip` union; add `range` leaf-list (ip-prefix) and `max-peers` (uint32) leaf under `connection/remote`
- `internal/component/config/yang/modules/ze-types.yang` - add `ip-prefix` typedef (union of `prefix-ipv4` and `prefix-ipv6`)
- `internal/component/bgp/config/resolve.go` - detect `ip dynamic` + `range` on groups, export group templates for reactor; validate `connect false` required; `range` inherited by explicit peers is inert (no `checkDuplicateRemoteIPs` change needed: dynamic groups contribute no peers to peerMap)
- `internal/component/bgp/config/peers.go` - support deferred variable resolution
- `internal/component/bgp/reactor/reactor.go` - store dynamic group configs
- `internal/component/bgp/reactor/reactor_connection.go` - dynamic peer creation on unknown IP
- `internal/component/bgp/reactor/reactor_peers.go` - dynamic peer cleanup on disconnect
- `internal/component/bgp/reactor/peersettings.go` - add IsDynamic flag, add RSClient flag
- `internal/component/bgp/reactor/forward_rs.go` - skip AS-path prepending when destination peer has RSClient=true (line 316 block)
- `internal/component/bgp/reactor/reactor_api_forward.go` - same RSClient check in standard ForwardUpdate path
- `internal/component/bgp/plugins/rs/server_forward.go` - `selectForwardTargets` gains community-based filtering; community parsing from wire bytes
- `internal/component/bgp/plugins/rs/server_handlers.go` - community stripping on egress

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| CLI commands/flags | [x] | Dynamic peer visibility in `show peer list` |
| CLI grammar (action before identifier) | [ ] | No new CLI commands |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/bgp/dynamic-peer-*.ci` |
| Doctor check for runtime dependencies | [ ] | No new runtime dependencies |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A (RS plugin unchanged) |
| 6 | Has a user guide page? | [x] | `docs/guide/route-server.md` (new) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc7947.md`, `rfc/short/rfc7948.md` |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` (Ze vs BIRD/OpenBGPd as RS) |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` (dynamic peer lifecycle) |

## Files to Create
- `internal/component/bgp/config/variables.go` - variable substitution engine
- `internal/component/bgp/config/variables_test.go` - variable substitution tests
- `internal/component/bgp/reactor/reactor_dynamic.go` - dynamic group matching, peer creation/cleanup
- `internal/component/bgp/reactor/reactor_dynamic_test.go` - dynamic peer tests
- `test/bgp/dynamic-peer-basic.ci` - functional test
- `test/bgp/dynamic-peer-variables.ci` - functional test
- `test/bgp/dynamic-peer-maxpeers.ci` - functional test
- `internal/component/bgp/plugins/rs/community.go` - zero-copy community parsing for selective forwarding
- `internal/component/bgp/plugins/rs/community_test.go` - community parsing tests
- `docs/guide/route-server.md` - user guide for route server setup (includes community scheme reference)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: `TestDynamicPeerCreation`, `TestVariableSubstitution`
   - Files: `reactor_dynamic.go` skeleton, `variables.go` skeleton
   - Verify: entry point exists and is reachable; wiring test fails because feature logic is a stub

2. **Phase: YANG schema** -- add `dynamic` enum to `connection/remote/ip`; add `range` and `max-peers` leaves
   - Tests: `TestResolveBGPTreeWithDynamicGroup`
   - Files: `ze-bgp-conf.yang`, `ze-types.yang`, `resolve.go`
   - Verify: config with `ip dynamic` + `range` parses; group template exported; `connect false` required

3. **Phase: Variable substitution engine** -- implement `$remote_as`, `$local_as`, `$remote_ip`
   - Tests: `TestVariableSubstitution`, `TestVariableSubstitutionASPath`, `TestVariableSubstitutionCommunity`
   - Files: `variables.go`
   - Verify: string replacement works for all variable types

4. **Phase: Dynamic group matching** -- `findDynamicGroup` in reactor
   - Tests: `TestFindDynamicGroup`, `TestExplicitPeerPrecedence`
   - Files: `reactor_dynamic.go`, `reactor_connection.go`
   - Verify: prefix matching works, explicit peers take precedence

5. **Phase: Dynamic peer lifecycle** -- creation, OPEN processing, cleanup
   - Tests: `TestDynamicPeerMaxPeers`, `TestDynamicPeerCleanup`, `TestDynamicPeerNaming`
   - Files: `reactor_dynamic.go`, `reactor_peers.go`, `peersettings.go`
   - Verify: full lifecycle works; variable resolution on OPEN; cleanup on disconnect

6. **Phase: Config reload** -- dynamic peer handling on reload
   - Tests: `TestDynamicPeerTeardownOnReload`
   - Files: `reactor_dynamic.go`
   - Verify: group removal tears down dynamic peers; range changes handled

7. **Phase: RS-client transparent AS-path** -- skip AS-path prepending for RSClient peers
   - Tests: `TestRSClientTransparentASPath`, `TestRSClientLoopDetection`, `TestNonRSClientPreserved`
   - Files: `peersettings.go`, `forward_rs.go`, `reactor_api_forward.go`, `ze-bgp-conf.yang`
   - Verify: RS-client peer receives route without RS ASN in AS-path; non-RS-client still gets prepend

8. **Phase: Community parsing** -- zero-copy community extraction from wire bytes
   - Tests: `TestCommunityParse`, `TestLargeCommunityParse`, `TestCommunityParseZeroAlloc`
   - Files: `community.go`
   - Verify: extract standard and large communities from UPDATE wire bytes without allocation

9. **Phase: Selective forwarding** -- community-based target filtering in selectForwardTargets
   - Tests: `TestCommunityDoNotAnnounce`, `TestCommunityAnnounceOnly`, `TestCommunityBlackhole`
   - Files: `server_forward.go`, `community.go`
   - Verify: do-not-announce, announce-only, and RS blackhole all work

10. **Phase: Community prepending** -- per-target AS-path prepending from large communities
    - Tests: `TestCommunityPrepend`, `TestCommunityPrependAll`
    - Files: `server_forward.go`
    - Verify: source ASN prepended N times to specific peers or all peers

11. **Phase: Community stripping** -- strip RS control communities before forwarding
    - Tests: `TestCommunityStripping`
    - Files: `server_forward.go` or egress filter
    - Verify: forwarded routes do not carry `0:X` or `RS:X` communities

12. **Functional tests** -- Create after all features work. Cover user-visible behavior.
13. **RFC refs** -- Add `// RFC 7947`, `// RFC 7948`, `// RFC 7999` comments
14. **Full verification** -- `make ze-verify`
15. **Complete spec** -- Fill audit tables, write learned summary, delete spec from `plan/`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Dynamic peer PeerAS populated from OPEN, not from config |
| Naming | Dynamic peer names follow `dyn-<ip>` pattern; YANG uses kebab-case |
| Data flow | Variable resolution happens at correct time (config load vs OPEN) |
| CLI grammar | No new CLI commands; dynamic peers visible in existing commands |
| Rule: no-layering | No wrapper structs around PeerSettings |
| Rule: explicit-precedence | Explicit peer always beats dynamic group |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `ip dynamic` enum + `range` leaf-list + `max-peers` leaf | `grep -n 'dynamic\|range\|max-peers' ze-bgp-conf.yang` |
| `variables.go` substitution engine | `go test ./internal/component/bgp/config/ -run Variable` |
| `reactor_dynamic.go` dynamic group logic | `go test ./internal/component/bgp/reactor/ -run Dynamic` |
| Dynamic peer creation on connection | Functional test `test-dynamic-peer-basic` passes |
| Variable resolution on OPEN | Functional test `test-dynamic-peer-variables` passes |
| max-peers enforcement | Functional test `test-dynamic-peer-maxpeers` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | group prefix validated (not /0, not /32 for IPv4, not /128 for IPv6) |
| Resource exhaustion | max-peers enforced; dynamic peer creation rate-limited |
| Variable injection | `$` variables only resolve to known variables; no arbitrary substitution |
| Connection flooding | Rapid connection/disconnect from dynamic range; cleanup must be prompt |
| ASN spoofing | Dynamic peer ASN comes from OPEN, not from IP; RPKI can validate later |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

- Ze already has all the building blocks: passive mode, 3-layer inheritance, RS plugin.
  The gap is specifically "create peer at connection time" rather than "create peer at config time."
- ARouteServer uses `$peer_as`, `$rs_as` as macro variables. Ze uses `$remote_as`, `$local_as`
  to match its existing naming (session/asn/remote, session/asn/local in YANG).
- IXPs in practice use static config generated from member databases (IXP Manager, custom automation).
  Dynamic peer groups are complementary: useful for smaller IXPs, development, or as a fallback.
  Both approaches can coexist (explicit peers + dynamic groups on same peering LAN).
- The `findPeerByAddr` function (reactor_peers.go:35) is the natural insertion point: after it
  returns false, check dynamic groups. This preserves explicit peer precedence by design.
- Variable resolution is two-phase because `$remote_as` is not known at config load time for
  dynamic peers. Static routes and filter chains must tolerate unresolved variables until OPEN.

## RFC Documentation

Add `// RFC 7947 Section N.N: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions.

## Implementation Summary

### What Was Implemented
- [TBD]

### Bugs Found/Fixed
- [TBD]

### Documentation Updates
- [TBD]

### Deviations from Plan
- [TBD]

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
| Dynamic peers from prefix range | functional test | `test-dynamic-peer-basic` |
| Config variable substitution | functional test | `test-dynamic-peer-variables` |
| Resource exhaustion prevention | functional test | `test-dynamic-peer-maxpeers` |
| Transparent AS-path (RFC 7947) | functional test | `test-rs-transparent-aspath` |
| Community blacklist (do-not-announce) | functional test | `test-rs-community-blacklist` |
| Community whitelist (announce-only) | functional test | `test-rs-community-whitelist` |
| Community prepending | functional test | `test-rs-community-prepend` |
| Control community stripping | functional test | `test-rs-community-strip` |
| Interop with BIRD clients | interop test | `dynamic-peer-bird` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [TBD]

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-32 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
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
- [ ] Interop tests for protocol features

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rs-dynamic-peers.md`
- [ ] Summary included in commit
