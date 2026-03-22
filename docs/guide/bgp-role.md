# BGP Role (RFC 9234)

BGP Role enables route leak prevention by declaring the relationship between peers. When configured, ze adds the Only To Customer (OTC) path attribute to enforce proper route propagation based on the business relationship.

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

## Strict Mode

When `strict true` is set, ze requires the peer to advertise the Role capability in its OPEN message. If the peer does not, ze sends a Role Mismatch NOTIFICATION and rejects the session.

When strict mode is off (default), ze proceeds even if the peer does not advertise Role. OTC filtering is applied based on the locally configured role.

## Without Role

When role is not configured for a peer, no OTC processing occurs. Routes are forwarded without role-based filtering.
