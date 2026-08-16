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
| `role / export` | list | -- | Destination roles that may receive routes: `default`, `unknown`, or explicit role names |
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
<!-- source: internal/component/bgp/plugins/role/register.go -- OTCIngressFilter, OTCEgressFilter, otcAttrCode -->

### On Receipt

| From Role | Action |
|-----------|--------|
| Provider, Peer, RS | Add OTC with remote ASN if not present |
| Customer, RS-Client | If OTC present, route is a leak -- mark ineligible |

### On Send

| To Role | Action |
|---------|--------|
| Customer, RS-Client, Peer | Add OTC with local ASN if not present |
| Provider, Peer, RS | Do not send routes that have OTC |

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

## Strict Mode

When `strict true` is set, ze requires the peer to advertise the Role capability in its OPEN message. If the peer does not, ze sends a Role Mismatch NOTIFICATION and rejects the session.

When strict mode is off (default), ze proceeds even if the peer does not advertise Role. OTC filtering is applied based on the locally configured role.

## Without Role

When role is not configured for a peer, no OTC processing occurs. Routes are forwarded without role-based filtering.
<!-- source: internal/component/bgp/plugins/role/ -- role plugin implementation -->
