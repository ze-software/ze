# BGP Role (RFC 9234)

BGP Role enables route leak prevention by declaring the relationship between peers. When configured, ze adds the Only To Customer (OTC) path attribute to enforce proper route propagation based on the business relationship.
<!-- source: internal/component/bgp/plugins/role/register.go -- role registration, RFC 9234, OTC attribute -->

## Configuration

```
bgp {
    peer upstream {
        remote { ip 10.0.0.1; as 65001; }
        local { ip 10.0.0.2; as 65000; }
        router-id 10.0.0.2

        role {
            import customer
            strict true
        }

        family { ipv4/unicast; }
    }
}
```

### Config Reference

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `role / import` | enum | -- | Declares local role and enables RFC 9234 ingress rules: `provider`, `customer`, `rs`, `rs-client`, `peer` |
| `role / export` | list | -- | Destination roles that may receive routes: `default`, `unknown`, or explicit role names. See [Export set](#export-set) |
| `role / strict` | bool | false | Reject peers that don't advertise the Role capability |

Role can be set at the group level and overridden per peer.
<!-- source: internal/component/bgp/plugins/role/yang/ -- ze-role YANG schema -->

## Role Values

| Role | Code | Description |
|------|------|-------------|
| `provider` | 0 | Sells transit to customers |
| `rs` | 1 | Route server at an IXP |
| `rs-client` | 2 | Client of a route server |
| `customer` | 3 | Buys transit from providers |
| `peer` | 4 | Settlement-free peering |

### Valid Peer Pairs

The local and remote roles must form a valid pair:

| Local | Remote |
|-------|--------|
| provider | customer |
| customer | provider |
| rs | rs-client |
| rs-client | rs |
| peer | peer |

Mismatched roles cause a Role Mismatch NOTIFICATION (error 2, subcode 11).

## OTC Filtering

The OTC (Only To Customer) attribute (type 35) prevents route leaks:
<!-- source: internal/component/bgp/plugins/role/otc_filter.go -- evaluateOTCIngress, evaluateOTCEgress, otcInsertions -->
<!-- source: internal/component/bgp/plugins/role/register.go -- otcAttrCode -->

### What role the procedures use

The Section 5 procedures act on what the PEER is to ze, not on what ze is to the peer. Ze resolves that value in one place, for the source peer on ingress and for the destination peer on egress.

| Source | When it is used |
|--------|-----------------|
| The Role capability in the peer's OPEN | Whenever the peer sent one |
| The complement of the local `import` role | When the peer sent no Role capability |

RFC 9234 Section 4.2 settles the second row: "The locally configured BGP Role is used for the procedures described in Section 5." Section 8 names the non-compliant remote as the case the local AS must still stamp for. The complement is the pair table read the other way: `customer` means the peer IS a Provider, `provider` means the peer IS a Customer, `rs-client` means the peer IS an RS, `rs` means the peer IS an RS-Client, and `peer` means the peer IS a Peer.

The complement feeds the RFC MUST gates only. The `export` set below still reads the capability, because `unknown` there is an operator-selected target and not a missing answer.
<!-- source: internal/component/bgp/plugins/role/otc.go -- resolvePeerRole, peerRoleComplement -->

### On Receipt

| Condition | Action |
|-----------|--------|
| OTC is malformed | Treat-as-withdraw (RFC 7606) |
| OTC present, peer is Customer or RS-Client | Route leak, mark ineligible |
| OTC present, peer is Peer, OTC value is not the peer's AS | Route leak, mark ineligible |
| OTC absent, peer is Provider, Peer, or RS | Add OTC with the peer's AS |

### On Send

Four checks run in order. The first one that fires decides.

| # | Check | Action |
|---|-------|--------|
| 1 | The route carries OTC and the destination is a Provider, Peer, or RS | Suppress (RFC 9234 Section 5, no source config needed) |
| 2 | The destination is a Provider, Peer, or RS, and ze's role toward the SOURCE is customer, peer, rs-client, or rs (the source is not a Customer) | Suppress. This is the Gao-Rexford check of RFC 9234 Section 3.1: only a route learned from a Customer, or locally originated, goes upward or sideways |
| 3 | The source states `export` and the destination's capability role is outside that set | Suppress. Operator policy, not an RFC gate |
| 4 | The destination is a Customer, Peer, or RS-Client, the UPDATE advertises a route, and OTC is absent | Add OTC with ze's local AS for this session |

Check 2 reads the ingress metadata when the route came through the ingress filter, and falls back to the source peer's own `import` role when it did not. A stored-route replay carries no ingress metadata, and a missing value used to select the permissive branch.

Check 4 does not depend on the source. A route from an iBGP peer, a route reflector client, a locally originated route, or an API-injected route reaches a Customer with OTC, because the RFC conditions that rule on the destination only.

### Scope

Both tables act on a *route*, so OTC processing is bounded twice:

| Bound | Effect |
|-------|--------|
| Address family | Only AFI 1 (IPv4) and AFI 2 (IPv6) with SAFI 1 (unicast). The family comes from MP_REACH_NLRI, or from MP_UNREACH_NLRI when the UPDATE only withdraws, so a VPN, EVPN, flowspec or multicast **withdrawal** is excluded exactly as its announcement is, per RFC 9234 Section 5's "MUST NOT be applied to other address families by default" |
| Reachable NLRI | OTC is added only to an UPDATE that advertises a route. A withdraw-only UPDATE, an MP_UNREACH-only UPDATE and an End-of-RIB marker are forwarded untouched. Adding an attribute to them would produce a message RFC 4271 Section 4.3 says must not carry path attributes, which RFC 7606 Section 5.2 lets a conforming peer escalate to a session reset |

<!-- source: internal/component/bgp/plugins/role/otc.go -- isPayloadUnicast, payloadAdvertisesNLRI -->

Suppression is not bounded by the second rule: a route that already carries OTC
is withheld from a Provider, Peer, or RS whether or not this particular UPDATE
advertises it.

## Export Set

`role { export [ ... ] }` names the destination roles that can receive routes from this peer. It is operator policy on top of the RFC gates, and it applies only when the SOURCE peer states it.

| Token | Meaning |
|-------|---------|
| `default` | The RFC 9234 Section 5 defaults for the local role. See the table below |
| `unknown` | Also send to a peer whose role ze does not know |
| An explicit role name | `provider`, `customer`, `peer`, `rs`, `rs-client` |

| Local `import` | `default` expands to |
|----------------|----------------------|
| `provider` | `customer`, `rs-client` |
| `customer` | `provider`, `rs`, `peer` |
| `rs` | `rs-client` |
| `rs-client` | `rs`, `provider` |
| `peer` | `customer`, `rs-client` |

An unrecognized token is logged as a warning and kept as written, so it matches no destination.

`unknown` matches two states of a destination: a peer whose OPEN declared no role, and a peer whose OPEN was never recorded. The two suppress identically and are counted apart, so the drop reason tells an operator which one to look at. An unrecorded role points at validate-open, not at the export policy.

## Groups and Dynamic Groups

A role stated on a group reaches every member. A configured peer already carries its group's role, merged at parse time. A member of a dynamic group has no entry in the configuration document, so ze resolves its role through its group.

Resolution is by address first and by group second, so a member that states its own role keeps it. Three decisions take that route: the Section 5 OTC gates, the Section 4.2 OPEN pair check with its strict mode, and the Role capability ze advertises to the member.

A learned role survives a configuration reload while the configuration still names the peer, or names the group a dynamic member belongs to. An established session sends no second OPEN, so a dropped learned role could not be written back.

A peer made at runtime with `create bgp peer <addr>` is named by no configuration, and its learned role survives every reload as well. Deleting that peer removes the role before `delete bgp peer` returns, and ze logs `learned role dropped: peer removed` with the role it held, so a peer later created at the same address starts with none. A rolled-back commit that had added a peer removes that peer's role the same way. One case keeps a role: an OPEN that ze was still validating when its peer was removed can store the role after the removal, and ze keeps it until a new OPEN for that name replaces it or the daemon restarts.

## Config That Cannot Be Reached

The runtime resolves role config by peer ADDRESS. A peer with no `connection > remote > ip`, and a name that is not an address, is logged as a warning and its role config is dropped. The group placeholder `dynamic` on a peer is refused the same way. Both used to be stored under a key nothing could look up, so the role was inert and silent.

Every drop is counted, and the first one of each reason raises a warning that names the peer. Later drops are counted only, with per-route detail at debug level.

| Metric | Reason labels |
|--------|---------------|
| `ze_role_route_rejects_total` | `leak`, `malformed-otc` |
| `ze_role_route_suppressions_total` | `otc-present`, `source-role`, `export-set`, `role-unrecorded` |

The label set is bounded at six series per metric. Peer identity is in the log line, never in a label.
<!-- source: internal/component/bgp/plugins/role/metrics.go -- recordDrop, dropReasonLabels -->

`role-unrecorded` and `export-set` both mean the destination was outside the export set. They are counted apart because they call for opposite actions: `export-set` points at the policy, and `role-unrecorded` points at validate-open.

## Strict Mode

When `strict true` is set, ze requires the peer to advertise the Role capability in its OPEN message. If the peer does not, ze sends a Role Mismatch NOTIFICATION and rejects the session.

When strict mode is off (default), ze proceeds even if the peer does not advertise Role. OTC filtering is applied based on the locally configured role.

## The OTC Procedures a Role Inserts

A peer whose config sets `role import` carries two more members in its filter
lists, with nothing for you to write: `bgp-role:otc-ingress` at the head of
its import list and `bgp-role:otc-egress` at the head of its export list. They
run the RFC 9234 Section 5 procedures described above, the same way whether
`bgp-role` runs inside Ze or as its own process. On import a leaked route is
withdrawn, as is an UPDATE whose OTC is malformed, and a route that must carry
OTC gets it; on export a route that must not reach the peer is not sent, and
one that must carry OTC gets the local AS.

Both members are required. RFC 9234 Section 5 says "The operator MUST NOT have
the ability to modify the procedures defined in this section." So a chain that
names either member, plain or `inactive:`, is refused at load with that
sentence, naming the peer, the list and the member. A peer with no `role
import` carries neither member.
<!-- source: internal/component/bgp/plugins/role/otc_filter.go -- otcFilterDecls, otcInsertions, evaluateOTCIngress, evaluateOTCEgress -->
<!-- source: internal/component/bgp/config/filter_insertion.go -- requiredInsertionRefusal -->

## The Upstream Check a Role Inserts

A declared role puts one more member in the peer's filter list, with nothing
for you to write: `bgp-role:upstream-asn`. It rejects a route whose AS path
carries the ASN of one of your transit providers, at any position, an AS_SET
member included. RFC 7454 Section 9 recommends it twice: a customer's routes
"must not be accepted" when their paths hold "upstream transit providers", and
"Network administrators SHOULD NOT advertise prefixes with upstream AS numbers
in the AS path to their peering AS".

| Your `role import` | The session is to | The check runs on |
|--------------------|-------------------|-------------------|
| `provider` | a customer | import |
| `peer` | a peer | import and export |
| `rs` | an RS-client | import |
| `rs-client` | a route server | export |
| `customer` | a provider | nowhere: every route a provider sends carries its own ASN |

The providers are derived from the config you already wrote: the remote ASN
of every peer whose `role import` is `customer` (RFC 9234 Section 3.1: "the
local AS is a transit customer of the remote AS"). A provider this router does
not peer with, one reached through another router of your network, goes in a
`policy asn-set`, named by `role transit-asns` on any peer or group. The check
compares the union of the two, and never the session's own remote ASN.

```
policy {
    asn-set far-transit {
        asn [ 64700 64701 ];
    }
}
bgp {
    group customers {
        role {
            import provider;
            transit-asns far-transit;
        }
    }
}
```

With no provider peer and no `transit-asns`, the set is empty and the member
rejects nothing: a network with no transit provider has no upstream ASN that
can leak. Adding or removing a provider peer, or editing the asn-set, takes
effect on the next route with no session reset. The member is not required,
because RFC 7454 is a Best Current Practice: name it as
`inactive:bgp-role:upstream-asn` in a chain to run that session without it.
<!-- source: internal/component/bgp/plugins/role/upstream.go -- upstreamInsertions, deriveUpstreamSet, evaluateUpstream -->
<!-- source: internal/component/bgp/plugins/role/yang/ze-role.yang -- transit-asns, asn-set -->

## Without Role

When role is not configured for a peer, no OTC processing occurs. Routes are forwarded without role-based filtering, and the upstream check is not inserted on that peer: no relationship is declared, so none is implied.

ze says so rather than passing in silence. The role plugin writes ONE warning naming how many peers and dynamic groups declare no role and the first few by name, each time it installs a config (at start and at every reload) and when `ze config validate` checks one. `ze doctor` reports every one of them under `doctor-bgp-peer-no-role`. All three read the same set, so they always name the same peers.

A dynamic group is named with the peers. Its members' AS arrives in the OPEN (RFC 4271 Section 4.2), so ze cannot know a listen range is eBGP, and an IXP route-server range that declares no role is exactly what the report is for. An iBGP peer is left out: a role describes an eBGP relationship, so a route reflector's sessions owe none.
<!-- source: internal/component/bgp/plugins/role/ -- role plugin implementation -->
<!-- source: internal/component/bgp/plugins/role/roleless.go -- rolelessPeers, warnPeersWithoutRole -->
<!-- source: internal/component/bgp/plugins/role/config_tx.go -- installRoleConfig, verifyRoleConfig -->
<!-- source: internal/component/bgp/plugins/role/doctor.go -- checkRolelessPeers -->
