# Route Server at an IXP

Use Ze as an RFC 7947 route server when each member should maintain one eBGP session and receive eligible routes from the other members without next-hop rewriting.

## Topology

```text
                         Ze route server
                         AS 65000
                         192.0.2.254
                               |
              +----------------+----------------+
              |                |                |
        Member A          Member B          Member C
        AS 65001          AS 65002          AS 65003
        192.0.2.1         192.0.2.2         192.0.2.3
```

Each member peers only with the route server. The route server distributes accepted paths to other members while preserving the member's next hop.

## Components

A practical deployment uses:

- `bgp-rs` for route-server forwarding;
- `bgp-adj-rib-in` for accepted inbound route state and replay;
- `bgp-rib` where local RIB operations or route visibility require it;
- BGP Role for route-server and route-server-client relationships;
- RPKI and IRR policy where the IXP requires them;
- the Looking Glass for member-facing visibility.

Use the generated [plugin catalogue](../../docs/features/plugins/) to confirm current dependencies and configuration roots. Build peer groups with the [BGP peering guide](../../docs/guide/bgp-peering/) and policy chains with the [BGP policy guide](../../docs/guide/bgp-policy/).

## Complete lab configuration

Save the following as `route-server.conf`. It declares three members, origin
validation, retained inbound routes, route-server forwarding, BGP Role, and a
loopback-only Looking Glass:

```text
plugin {
    internal rib { use bgp-rib; }
    internal rs { use bgp-rs; }
    internal adj-rib-in { use bgp-adj-rib-in; }
    internal rpki { use bgp-rpki; }
    internal role { use bgp-role; }
}

environment {
    looking-glass {
        enabled true;
        server main { ip 127.0.0.1; port 8443; }
    }
}

bgp {
    router-id 192.0.2.254;
    session { asn { local 65000; } }

    rpki {
        cache-server 192.0.2.100 { port 323; }
        action { invalid reject; not-found accept; }
    }

    group members {
        session {
            family {
                ipv4/unicast { prefix { maximum 10000; } }
            }
        }
        role { import rs; strict false; }
        process rpki { receive [ update ]; }
        process adj-rib-in { receive [ update state ]; }

        peer member-a {
            connection {
                local { ip 192.0.2.254; }
                remote { ip 192.0.2.1; }
            }
            session { asn { remote 65001; } }
        }
        peer member-b {
            connection {
                local { ip 192.0.2.254; }
                remote { ip 192.0.2.2; }
            }
            session { asn { remote 65002; } }
        }
        peer member-c {
            connection {
                local { ip 192.0.2.254; }
                remote { ip 192.0.2.3; }
            }
            session { asn { remote 65003; } }
        }
    }
}
```

Replace the RTR cache address and prefix maximum with the values approved for
the IXP. Validate the exact file before starting it:

```console
ze config validate route-server.conf
ze start route-server.conf
```

This baseline sends the best eligible path to each member. Configure ADD-PATH
only after both directions and every member implementation have been tested.

## Session policy

For every member:

1. Set an explicit local and remote address and ASN.
2. Enable only the required address families.
3. Set per-family prefix limits from the member's declared policy.
4. Configure the RFC 9234 route-server relationship.
5. Apply import validation before a route reaches the forwarding path.
6. Apply member export controls per destination.
7. Preserve the original next hop.

ADD-PATH is useful when members should receive multiple eligible paths and make their own best-path decision. It must be negotiated in the required direction on both ends. Treat missing ADD-PATH as an operational condition to detect, not as proof that the route server is forwarding every path.

## Minimum import checks

An IXP policy normally rejects:

- prefixes outside the member's registered policy;
- RPKI Invalid announcements where the IXP policy drops them;
- default routes and over-specific prefixes;
- malformed or prohibited AS paths;
- control communities owned by the IXP when a member is not authorised to set them.

The exact policy is an IXP decision. Keep policy objects named and reviewable rather than embedding unrelated actions in one chain.

## Verification

After applying the configuration:

```console
ze cli -c "show bgp summary"
ze cli -c "show bgp peer member-a capabilities"
ze cli -c "show bgp rib status"
ze cli -c "show bgp rib peer member-a"
ze cli -c "show warnings"
ze cli -c "show errors"
```

Prove the complete path with a test route:

1. Announce a prefix from Member A.
2. Confirm it is accepted from Member A.
3. Confirm policy permits it toward Members B and C.
4. Confirm both members receive Member A's next hop, not the route server's address.
5. Withdraw it and confirm it disappears everywhere.
6. Reconnect one member and confirm replay affects only that member.

## Member visibility

Expose the read-only Looking Glass behind the deployment's normal TLS and rate-limiting boundary. Members should be able to inspect peer state, route lookup, and AS-path topology without receiving authenticated operator access.

See the [Looking Glass guide](../../docs/guide/looking-glass/) for the browser and Birdwatcher-compatible interfaces.

## Operational risks

- A prefix limit that is too low tears down a valid member when their route count grows.
- A limit that is too high weakens protection against a leak.
- Missing ADD-PATH can reduce the paths sent to a member.
- RPKI or IRR cache failure behaviour must match the IXP's written policy.
- A slow member can create backpressure during churn.
- Route-server role compatibility must be checked before enabling strict enforcement.

Monitor peer state, route counts, policy failures, forwarding-pool utilisation, and replay duration during every maintenance window.
