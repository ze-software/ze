# BGP Peering

Use this guide after the quick start when a peer needs groups, multiple address families, capabilities, prefix limits, or an explicit relationship role.

## Minimal peer

A peer needs local and remote addresses, AS numbers, a router ID, and at least one address family:

```text
bgp {
    router-id 192.0.2.10

    peer transit-a {
        connection {
            local { ip 192.0.2.10 }
            remote { ip 192.0.2.1 }
        }
        session {
            asn { local 64500; remote 64496 }
            family {
                ipv4/unicast {
                    prefix { maximum 1000000 }
                }
            }
        }
    }
}
```

Validate before starting or reloading:

```console
ze config validate ze.conf
ze doctor
```

Use the generated [configuration reference](https://ze-software.net/config-reference/) for the current leaf names and constraints.

## Groups and inheritance

Put shared settings in a group and override only what differs on each peer:

```text
bgp {
    router-id 192.0.2.10

    group transit {
        session {
            asn { local 64500 }
            family {
                ipv4/unicast { prefix { maximum 1000000 } }
                ipv6/unicast { prefix { maximum 250000 } }
            }
        }
        timer { receive-hold-time 90 }

        peer transit-a {
            connection {
                local { ip 192.0.2.10 }
                remote { ip 192.0.2.1 }
            }
            session { asn { remote 64496 } }
        }

        peer transit-b {
            connection {
                local { ip 2001:db8::10 }
                remote { ip 2001:db8::1 }
            }
            session { asn { remote 64497 } }
        }
    }
}
```

Peer values override group values. Group values override the global BGP block. Inspect the resolved result with `show config dump` rather than reasoning about inheritance from the source file alone.

## Address families

The engine provides IPv4 and IPv6 unicast and multicast. Plugins register VPN, EVPN, FlowSpec, labeled-unicast, BGP-LS, MVPN, RTC, MUP, VPLS, and SR Policy families. The current encode, decode, and route-configuration status is maintained on the [BGP protocol page](../features/bgp-protocol.md).

Each enabled family requires an explicit prefix limit. Set it from expected operational volume rather than accepting a migration default. A full-table peer and a customer announcing one aggregate should not share the same limit.

## ADD-PATH

ADD-PATH is configured per family. The peers must negotiate compatible send and receive modes before additional paths can be exchanged. See the [ADD-PATH guide](add-path.md) for the mode matrix, path limits, and verification commands.

## BGP Role

RFC 9234 roles describe the business relationship between peers and let Ze enforce valid pairings and OTC handling. Configure roles deliberately on eBGP sessions. See the [BGP Role guide](bgp-role.md) for valid peer pairs and policy effects.

## Verify the session

```console
ze cli -c "show bgp summary"
ze cli -c "show bgp peer transit-a detail"
ze cli -c "show bgp peer transit-a capabilities"
ze cli -c "show bgp rib status"
```

Check that:

1. The session is Established.
2. The negotiated families match the intended configuration.
3. Prefix counts remain below the configured limits.
4. Capabilities such as ADD-PATH, Graceful Restart, and BGP Role were negotiated rather than merely configured locally.
5. `show warnings` and `show errors` are clean.

## Related guides

- [Configuration](configuration.md) covers syntax and the full configuration model.
- [BGP policy](bgp-policy.md) covers import, export, and redistribution.
- [BGP resilience](bgp-resilience.md) covers route refresh, restart, persistence, and reflection.
- [Route injection](route-injection.md) covers on-demand announcements.
