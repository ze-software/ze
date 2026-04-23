# Static Routes

Ze supports static routes with ECMP, weighted load balancing, BFD-tracked
failover, blackhole, and reject. Routes are programmed directly to the
kernel via netlink (or VPP when available).

## Configuration

```
static {
    route 0.0.0.0/0 {
        next-hop 10.0.0.1 { }
    }
}
```

### Multiple next-hops (ECMP)

All next-hops are installed simultaneously as a multipath route:

```
static {
    route 10.0.0.0/8 {
        next-hop 192.168.1.1 { }
        next-hop 192.168.1.2 { }
        next-hop 192.168.1.3 { }
    }
}
```

### Weighted ECMP

The `weight` field controls traffic distribution. Higher weight means
more traffic. Default is 1 (equal distribution).

```
static {
    route 0.0.0.0/0 {
        next-hop 10.0.0.1 {
            weight 3
        }
        next-hop 10.0.0.2 {
            weight 1
        }
    }
}
```

This sends 75% of traffic via 10.0.0.1 and 25% via 10.0.0.2.

### BFD failover

Reference a BFD profile on each next-hop. When the BFD session goes
down, that next-hop is removed from the ECMP group and the route is
reprogrammed with the remaining active next-hops. When the session
recovers, the next-hop is re-added.

```
bfd {
    profile wan-fast {
        detect-multiplier 3
        desired-min-tx-us 100000
        required-min-rx-us 100000
    }
}

static {
    route 0.0.0.0/0 {
        next-hop 10.0.0.1 {
            weight 3
            bfd-profile wan-fast
        }
        next-hop 10.0.0.2 {
            weight 1
            bfd-profile wan-fast
        }
    }
}
```

If all BFD-tracked next-hops go down, the route is withdrawn entirely.

### Blackhole and reject

Blackhole silently discards matching packets. Reject discards and sends
an ICMP unreachable reply.

```
static {
    route 192.0.2.0/24 {
        blackhole { }
    }
    route 198.51.100.0/24 {
        reject { }
    }
}
```

### IPv6

IPv6 routes work the same way. For link-local next-hops, specify the
outgoing interface:

```
static {
    route 2001:db8::/32 {
        next-hop 2001:db8::1 { }
    }
    route 2001:db8:1::/48 {
        next-hop fe80::1 {
            interface eth0
        }
    }
}
```

### Route attributes

```
static {
    route 172.16.0.0/12 {
        description "internal networks"
        metric 100
        tag 42
        next-hop 10.0.0.1 { }
    }
}
```

- `description`: operator note (not programmed to kernel)
- `metric`: kernel route priority (lower is preferred)
- `tag`: opaque value for route policy matching in redistribute

## CLI

```
ze> static show
```

Shows all configured static routes with their prefixes, next-hops,
weights, and BFD status in JSON format.

## Route programming

Static routes are programmed with `RTPROT_ZE` (protocol 250), the same
identifier used by the FIB kernel plugin. On config reload, the plugin
computes the diff between old and new routes and applies only the
changes.

Kernel ECMP uses `RTA_MULTIPATH` with per-next-hop weight mapped from
the `weight` field (kernel weight = weight - 1).

## Redistribute

Static routes register as protocol "static" in the redistribute
framework. BGP redistribute can import static routes:

```
redistribute {
    import static { }
}
```
