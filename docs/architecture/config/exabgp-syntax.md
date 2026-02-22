# ExaBGP Legacy Configuration Syntax

ExaBGP-compatible syntax that ze accepts for backward compatibility. Use `ze bgp config migrate` to convert to ze-native `update { attribute {} nlri {} }` format.

The ExaBGP config parser uses its own YANG schema (`internal/exabgp/exabgp.yang`) with `ze:syntax` annotations. These annotations are not used in ze-native config.

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

The `flow { route { match {} then {} } }` block is **rejected** by ze-native config. It is only accepted by the ExaBGP migration parser. Use `ze bgp config migrate` to convert to `update { nlri { ipv4/flow add ...; } }` format.

```
flow {
    route <name> {
        match {
            source 10.0.0.0/24;
            destination 192.168.0.0/16;
            protocol =tcp;
            port [ =80 =443 ];
        }
        then {
            rate-limit 1000;
            redirect 65000:100;
        }
    }
}
```

---

**Last Updated:** 2026-02-22
