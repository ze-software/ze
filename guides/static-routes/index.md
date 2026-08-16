# Static Routes

Ze supports static routes with ECMP, weighted load balancing, BFD-tracked
failover, blackhole, and reject. Routes are programmed directly to the
kernel via netlink (or VPP when available). Routes are grouped under
named tables for policy-based routing support.

## Configuration

All routes live under a named table. Use `default` for the main
routing table:

```
static {
    table default {
        route 0.0.0.0/0 {
            next { hop 10.0.0.1 { } }
        }
    }
}
```

### Named routing tables

Define named tables via the `routing-table` config block, then
reference them in `static`:

```
routing-table {
    table lns {
        id 100
    }
}

static {
    table lns {
        route 0.0.0.0/0 {
            next { interface tun100 { } }
        }
    }
}
```

The same prefix can appear in different tables independently.
Routes in non-default tables are not redistributed into BGP.

Reserved table IDs (0, 253-255) are rejected. A 32-bit build additionally
rejects a table ID or route metric above 2147483647, because the netlink
bindings carry both in a machine int and would otherwise install the route in
the main table at the kernel's default metric without reporting anything.
Ze's released targets are 64-bit, where the full uint32 range is available.

<!-- source: internal/core/routingtable/registry.go — validateTableID, maxEncodableTableID -->
<!-- source: internal/plugins/static/config.go — validateRouteMetric, maxNetlinkInt -->

### Interface-only next-hops

For point-to-point links (PPPoE, GRE tunnels) where the next-hop is
the interface itself:

```
static {
    table default {
        route 0.0.0.0/0 {
            next { interface pppoe0 { } }
        }
    }
}
```

Interface-only next-hops do not support BFD profiles (BFD requires a
peer address). Weight is supported for ECMP:

```
static {
    table default {
        route 0.0.0.0/0 {
            next {
                interface pppoe0 { weight 3; }
                interface pppoe1 { weight 1; }
            }
        }
    }
}
```

### Mixed ECMP

Gateway and interface-only next-hops can coexist in the same route:

```
static {
    table default {
        route 0.0.0.0/0 {
            next {
                hop 10.0.0.1 { weight 3; }
                interface pppoe0 { weight 1; }
            }
        }
    }
}
```

### Multiple next-hops (ECMP)

All next-hops are installed simultaneously as a multipath route:

```
static {
    table default {
        route 10.0.0.0/8 {
            next {
                hop 192.168.1.1 { }
                hop 192.168.1.2 { }
                hop 192.168.1.3 { }
            }
        }
    }
}
```

### Weighted ECMP

The `weight` field controls traffic distribution. Higher weight means
more traffic. Default is 1 (equal distribution).

```
static {
    table default {
        route 0.0.0.0/0 {
            next {
                hop 10.0.0.1 {
                    weight 3
                }
                hop 10.0.0.2 {
                    weight 1
                }
            }
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
    table default {
        route 0.0.0.0/0 {
            next {
                hop 10.0.0.1 {
                    weight 3
                    bfd-profile wan-fast
                }
                hop 10.0.0.2 {
                    weight 1
                    bfd-profile wan-fast
                }
            }
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
    table default {
        route 192.0.2.0/24 {
            blackhole { }
        }
        route 198.51.100.0/24 {
            reject { }
        }
    }
}
```

### IPv6

IPv6 routes work the same way. For link-local next-hops, specify the
outgoing interface:

```
static {
    table default {
        route 2001:db8::/32 {
            next { hop 2001:db8::1 { } }
        }
        route 2001:db8:1::/48 {
            next {
                hop fe80::1 {
                    interface eth0
                }
            }
        }
    }
}
```

### Route attributes

```
static {
    table default {
        route 172.16.0.0/12 {
            description "internal networks"
            metric 100
            tag 42
            next { hop 10.0.0.1 { } }
        }
    }
}
```

- `description`: operator note (not programmed to kernel)
- `metric`: kernel route priority (lower is preferred)
- `tag`: opaque value for route policy matching in redistribute

## CLI

```
ze> show static
```

Shows all configured static routes with their prefixes, next-hops,
weights, and BFD status in JSON format.

## Route programming

Static routes are programmed with `RTPROT_ZE` (protocol 251), a
Ze-specific identifier distinct from the FIB kernel plugin (protocol 250).
On config reload, the plugin
computes the diff between old and new routes and applies only the
changes.

Kernel ECMP uses `RTA_MULTIPATH` with per-next-hop weight mapped from
the `weight` field (kernel weight = weight - 1).

## Redistribute

Static routes register as protocol "static" in the redistribute
framework. BGP redistribute can import static routes:

```
redistribute {
    destination bgp {
        import static
    }
}
```
