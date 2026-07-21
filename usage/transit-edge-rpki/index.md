# Transit Edge with RPKI

This deployment shape uses Ze at an Internet edge with two transit sessions, origin validation, explicit import policy, and local preference for deterministic failover.

## Topology

```text
                 Transit A AS 64496
                         |
                         |
                   Ze edge AS 64500
                         |
                         |
                 Transit B AS 64497
```

The two sessions may carry full tables or a constrained lab table. Size prefix limits and host resources for the intended route volume.

## Complete edge configuration

Save this two-transit example as `transit-edge.conf`. Transit A receives local
preference 200 and Transit B receives 100, so Transit A wins when both paths are
otherwise eligible. RPKI Invalid routes are rejected before they reach the RIB.

```text
plugin {
    internal rib { use bgp-rib; }
    internal adj-rib-in { use bgp-adj-rib-in; }
    internal rpki { use bgp-rpki; }
    internal role { use bgp-role; }
    internal modify { use bgp-filter-modify; }
}

bgp {
    router-id 192.0.2.10;
    session { asn { local 64500; } }

    rpki {
        cache-server 203.0.113.53 { port 323; }
        action { invalid reject; not-found accept; }
    }

    policy {
        modify TRANSIT-A-PREF {
            set { local-preference 200; }
        }
        modify TRANSIT-B-PREF {
            set { local-preference 100; }
        }
    }

    group transit {
        session {
            family {
                ipv4/unicast { prefix { maximum 1000000; } }
            }
        }
        role { import customer; strict false; }
        process rpki { receive [ update ]; }
        process adj-rib-in { receive [ update state ]; }

        peer transit-a {
            connection {
                local { ip 192.0.2.10; }
                remote { ip 192.0.2.1; }
            }
            session { asn { remote 64496; } }
            filter { import [ modify:TRANSIT-A-PREF ]; }
        }

        peer transit-b {
            connection {
                local { ip 198.51.100.10; }
                remote { ip 198.51.100.1; }
            }
            session { asn { remote 64497; } }
            filter { import [ modify:TRANSIT-B-PREF ]; }
        }
    }
}
```

Replace the documentation addresses, RTR cache, ASNs, and limits. Keep
`strict false` until both providers advertise a compatible BGP Role, then test
and enable strict enforcement deliberately.

```console
ze config validate transit-edge.conf
ze start transit-edge.conf
```

For a full table, confirm memory and convergence evidence on the
[performance page](../../performance/) before treating a configuration maximum
as a capacity claim.

## Add origin validation

Configure one or more RTR cache servers and apply RPKI validation in the import chain. The deployment policy must state what happens to Valid, Invalid, and NotFound routes.

A common starting policy is:

- accept Valid;
- reject Invalid;
- accept or mark NotFound while monitoring its proportion.

The correct choice depends on the network's policy and rollback plan. See [RPKI origin validation](../../docs/guide/rpki/) for cache configuration, validation flow, policy actions, and operator commands.

## Import policy

RPKI is one input to policy, not a replacement for prefix and AS-path controls. A transit edge normally combines:

- AS loop prevention;
- RPKI origin validation;
- bogon and default-route rejection where appropriate;
- maximum prefix protection;
- local preference or another explicit primary/backup policy;
- community handling for operator and upstream control.

Use [BGP policy](../../docs/guide/bgp-policy/) to keep these filters ordered and independently testable.

## Failure behaviour

Decide cache failure behaviour before deployment. A cache timeout can fail open, fail closed, or retain a previously validated state depending on the configured component and policy. Record that decision and test it with the cache unavailable.

A transit failure and an RPKI cache failure are different events. Monitor them separately so a routing incident does not hide a validation outage.

## Verification

```console
ze cli -c "show bgp summary"
ze cli -c "show bgp peer transit-a capabilities"
ze cli -c "show bgp rpki status"
ze cli -c "show bgp rpki cache"
ze cli -c "show bgp rpki summary"
ze cli -c "show bgp rib best"
ze cli -c "show warnings"
```

Test at least four routes:

1. A Valid route from the preferred transit.
2. An Invalid route that must be rejected.
3. A NotFound route with the deployment's chosen action.
4. The same valid prefix from both transits, proving the expected best path and failover.

Then disconnect the preferred transit. Confirm the alternate path becomes active and is installed in the FIB. Restore the session and verify the intended preference returns without stale paths.

## What to watch

- RTR session state and serial updates
- Valid, Invalid, and NotFound route counts
- Prefix-limit utilisation per peer and family
- Best-path changes and installed next hops
- Policy timeout or fail-open warnings
- Session resets during large validation updates
- Memory and convergence time for the actual table size

Do not treat an Established BGP session as proof that validation and failover work. Exercise the route state transitions directly.
