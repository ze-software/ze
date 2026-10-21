# ExaBGP Legacy Configuration Syntax

ExaBGP-compatible syntax that ze accepts for backward compatibility. Use `ze config migrate` to convert to ze-native `update { attribute {} nlri {} }` format.

The ExaBGP config parser uses its own YANG schema (`internal/exabgp/exabgp.yang`) with `ze:syntax` annotations. These annotations are not used in ze-native config.
<!-- source: internal/exabgp/migration/exabgp.yang -- ExaBGP YANG schema -->
<!-- source: internal/exabgp/migration/schema.go -- YANG schema loading for ExaBGP migration -->
<!-- source: internal/exabgp/migration/migrate_routes.go -- ze:syntax annotation handling -->

For ze-native config syntax, see [syntax.md](syntax.md).

---

## Static Routes

```
static {
    route <prefix> {
        next-hop <ip>;
        origin igp;
        as-path [ <asn> ... ];
        local-preference <int>;
        med <int>;
        community [ <community> ... ];
        extended-community [ <ext-community> ... ];
        large-community [ <large-community> ... ];
        label [ <label> ... ];
        rd <route-distinguisher>;
        split /<size>;
        watchdog <name>;
        withdraw;
    }
}
```

Shorthand:

```
static {
    route <prefix> next-hop <ip>;
    route <prefix> next-hop <ip> as-path [ 65001 65002 ];
}
```

---

## Announce Block

```
announce {
    ipv4 {
        unicast <prefix> { next-hop <ip>; ... }
        mpls-vpn <prefix> { rd <rd>; label <label>; ... }
        mcast-vpn <route-type> { rd <rd>; source-as <asn>; ... }
        mup <route-type> <prefix> rd <rd> ...;
    }
    ipv6 { ... }
    l2vpn {
        vpls <name> { endpoint <int>; base <int>; ... }
        evpn <route-type> <nlri> <attributes>;
    }
}
```

### MPLS-VPN (L3VPN)

RFC 4364 L3VPN routes under `announce { ipv4/ipv6 { mpls-vpn ... } }`.

```
announce {
    ipv4 {
        mpls-vpn <prefix> {
            rd <route-distinguisher>;
            label <mpls-label>;              # Single label
            labels [ <label> <label> ... ];  # Multiple labels (RFC 8277)
            next-hop <ip>;
            origin igp;
            local-preference <int>;
            med <int>;
            as-path [ <asn> ... ];
            community [ <community> ... ];
            extended-community [ <ext-community> ... ];
            originator-id <ip>;
            cluster-list <ip> <ip>;
        }
    }
}
```

Shorthand:

```
announce {
    ipv4 {
        mpls-vpn 10.0.0.0/24 rd 100:100 label 1000 next-hop 192.168.1.1;
    }
}
```

### MVPN (Multicast VPN)

RFC 6514 MVPN routes under `announce { ipv4/ipv6 { mcast-vpn ... } }`.

| Route Type | Description |
|------------|-------------|
| `source-ad` | Source Active A-D (Type 5) |
| `shared-join` | Shared Tree Join (Type 6) |
| `source-join` | Source Tree Join (Type 7) |

```
announce {
    ipv4 {
        mcast-vpn source-ad {
            rd 100:100;
            source-as 65000;
            source 10.0.0.1;
            group 239.1.1.1;
            next-hop 192.168.1.1;
            extended-community target:100:100;
        }
    }
}
```

### MUP (Mobile User Plane)

SRv6 Mobile User Plane routes (draft-mpmz-bess-mup-safi) under `announce { ipv4/ipv6 { mup ... } }`.

| Route Type | Description |
|------------|-------------|
| `mup-isd` | Interwork Segment Discovery (Type 1) |
| `mup-dsd` | Direct Segment Discovery (Type 2) |
| `mup-t1st` | Type 1 Session Transformed (Type 3) |
| `mup-t2st` | Type 2 Session Transformed (Type 4) |

```
announce {
    ipv4 {
        mup mup-isd 10.0.0.0/24 rd 100:100 next-hop 192.168.1.1;
        mup mup-t1st 192.168.0.2/32 rd 100:100 teid 12345 qfi 9 endpoint 10.0.0.1 next-hop 192.168.1.1;
    }
}
```

### L2VPN / VPLS

```
l2vpn {
    vpls <name> {
        endpoint <int>;
        base <int>;
        offset <int>;
        size <int>;
        next-hop <ip>;
        origin igp;
        as-path [ ... ];
        rd <rd>;
        route-target [ <rt> ... ];
        originator-id <ip>;
        cluster-list <ip> <ip>;
    }
}
```

### EVPN

EVPN routes use freeform syntax (passed through to ExaBGP-compatible parser):

```
announce {
    l2vpn {
        evpn <route-type> <nlri> <attributes>;
    }
}
```

---

## Flow Block

The `flow { route { match {} then {} } }` block is **rejected** by ze-native config. It is only accepted by the ExaBGP migration parser. Use `ze config migrate` to convert to `update { nlri { ipv4/flow add ...; } }` format.
<!-- source: internal/component/config/cli/cmd_migrate.go -- ze config migrate command -->
<!-- source: internal/exabgp/migration/migrate.go -- ExaBGP migration logic -->

```
flow {
    route <name> {
        match {
            source 10.0.0.0/24;
            destination 192.168.0.0/16;
            protocol =tcp;
            port [ =80 =443 ];
        }
        scope {
            interface-set [ transitive:input:1234:10 ];
        }
        then {
            rate-limit 1000;
            redirect 65000:100;
        }
    }
}
```

### Scope

The `scope` block is the third section of a flow route. It holds interface-set
extended communities, which tell the receiver to apply the filter on one set of
interfaces and in one direction (draft-ietf-idr-flowspec-interfaceset).

Each value takes one of two forms:

```
<transitive|non-transitive>:<input|output|input-output>:<asn>:<group-id>
<input|output|input-output>:<asn>:<group-id>
```

The second form is transitive. The group identifier is a 14-bit number, so it
goes from 0 to 16383.

The migration writes each value as ze's `interface-set` extended community, in
the four-field form so the migrated config states its transitivity.
`interface-set [ non-transitive:input:3405770241:1 ]` becomes
`extended-community [interface-set:non-transitive:input:3405770241:1]`, and it
reaches the wire as the same octets ExaBGP sends (`4702caffee014001`).

These stop the migration, as ExaBGP refuses them:

- a value outside the two forms
- a transitivity other than `transitive` or `non-transitive`, judged first
- an AS number holding a period, asdot (`1.234`) included, which ExaBGP
  refuses in an interface-set though ze accepts it elsewhere; the error names
  the 32-bit integer to write
- an AS number above 32 bits (4294967295)
- a direction other than the three names
- a group identifier above 16383
- an `interface-set:` item written inside `extended-community`, in a flow
  route's `then` block or a route's attributes: ExaBGP has no such extended
  community type, and the error names the `interface-set` keyword to write it
  with

The ExaBGP bridge refuses the same values in an API command. That covers the
`extended-community` of an `sr-policy` announce or withdraw too, because
ExaBGP parses the attributes of both.
<!-- source: internal/exabgp/bridge/bridge_flow.go -- CheckInterfaceSet, CheckExtendedCommunityItem -->
<!-- source: internal/exabgp/bridge/bridge_command.go -- convertAnnounceSRPolicy, convertWithdrawSRPolicy -->
<!-- source: internal/exabgp/migration/migrate_flow_scope.go -- interface-set conversion -->

### Redirect and copy

Two drafts define a redirect to an address. The IETF one
(draft-ietf-idr-flowspec-redirect-ip) carries the address in the community, with
a copy bit. The older Simpson one (draft-simpson-idr-flowspec-redirect) sends
the 0x0800 community and redirects to the route's next hop. ExaBGP 6.0 made
`redirect <ip>` and `copy <ip>` the IETF form and moved the Simpson form to the
`-simpson` spellings. The migration follows the 6.0 grammar, so a 4.x or 5.x
config that meant the Simpson form by `redirect <ip>` migrates to the IETF one,
as ExaBGP 6.0 itself reads it.

| ExaBGP `then` action | ze `extended-community` | ze next hop |
|----------------------|-------------------------|-------------|
| `redirect <asn>:<nn>` | `redirect:<asn>:<nn>` | unchanged |
| `redirect <ip>`, `redirect-to-nexthop <ip>`, `redirect-to-nexthop-ietf <ip>` | `redirect-to-nexthop <ip>` | unchanged |
| `copy <ip>` | `copy-to-nexthop <ip>` | unchanged |
| `redirect-simpson <ip>` | `redirect-to-nexthop-draft` | `<ip>` |
| `copy-simpson <ip>` | `copy-to-nexthop` | `<ip>` |
| `redirect-to-nexthop`, `redirect-to-nexthop-simpson` | `redirect-to-nexthop-draft` | unchanged |

An IPv6 address is accepted bare or in brackets. A keyword that takes an
address and is given something else stops the migration.
<!-- source: internal/exabgp/migration/migrate_flow_redirect.go -- flowRedirectAction -->

---

**Last Updated:** 2026-02-22
