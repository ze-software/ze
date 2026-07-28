# BGP Policy

Ze applies ordered import and export filter chains to BGP updates. Define named policy objects once, attach them globally, to a group, or to one peer, then verify both accepted routes and the routes sent onward.

## Policy model

Policy has two parts:

1. Named filter instances under `bgp { policy { ... } }`.
2. Ordered references under `filter { import [...] export [...] }`.

```text
bgp {
    policy {
        loop-detection no-self-as {
            allow-own-as 0
        }
    }

    filter {
        import [ no-self-as rpki:validate ]
    }

    group customers {
        filter {
            import [ community:scrub ]
            export [ prefix:customer-export ]
        }
    }
}
```

The effective chain is cumulative: automatic safety filters run first, followed by global, group, and peer references. Use `insert` in the editor when order matters.

## Filter outcomes

A filter can:

- accept the update unchanged;
- reject it and stop the chain;
- modify selected attributes and pass the changed update onward.

Each plugin declares whether an internal error fails open or closed. Security filters should normally reject on failure. Operational policy should make that choice explicit rather than depending on an unavailable external service.

## Common policy building blocks

Ze ships filters for:

- AS path matching and length;
- prefix lists;
- standard, large, and extended communities;
- route attribute modification;
- private AS removal;
- address-family filtering;
- RPKI and IRR validation;
- RFC 4271 AS loop and RFC 4456 cluster-list loop detection.

The [plugin guide](plugins.md) documents the registered filter types and their current configuration. The generated [plugin catalogue](https://ze-software.net/docs/features/plugins/) shows which plugin owns each filter.

## IRR and RPKI

IRR filters build allowed prefix sets from routing registry data. RPKI validates route origin against ROAs. They answer different questions and can be used together:

- IRR: is this prefix documented for this network or AS-SET?
- RPKI: is the origin AS authorised by a cryptographically validated ROA?

Use the [IRR filtering](irr-filtering.md) and [RPKI](rpki.md) guides for cache configuration, refresh, failure behaviour, and operator commands.

## Attribute changes

Attribute modification belongs in an explicit export or import chain. Keep each named modifier narrow, then compose it with other filters. Typical operations include setting local preference or MED, changing next hop, and adding or removing communities.

Inspect the live plugin documentation before using increment or decrement operations. Attribute ranges and missing-attribute behaviour are validated by the owning plugin.

## Route-server control communities

When ze runs as a route server, a client can steer its own routes by tagging them with communities the route server reads and then removes. A peer becomes a route-server client with `rs-client true` under its `session` block; the communities below are only interpreted for such peers.

| Community | Meaning |
|-----------|---------|
| `0:<asn>` | Do not advertise this route to `<asn>`. |
| `<rs-asn>:<asn>` | Advertise this route ONLY to the listed ASNs. |
| `<rs-asn>:0` | Do not advertise this route to any client. |
| `65535:666` | RFC 7999 blackhole. |

A whitelist wins over a blacklist: if a route carries any `<rs-asn>:<asn>` tag, only the listed ASNs receive it and `0:<asn>` tags on the same route are not consulted.

These are control tags, not attributes to pass on. The route server strips every one of them from the route before forwarding it, so a client never sees another client's steering instructions or the route server's own. Communities the route server does not recognise are forwarded untouched and in their original order, so a client's own tagging survives.

```
# Client tags a route: keep it away from AS 64998, and let AS 65002 have it.
# The route server removes both tags; 65001:100 reaches AS 65002 unchanged.
community [ 0:64998 65000:65002 65001:100 ]
```

Two limits are worth knowing:

- A standard community's high half is 16 bits (RFC 1997), so a route server with a 4-octet ASN matches `<rs-asn>:<asn>` on the low 16 bits of its ASN only. Use large communities where that is ambiguous.
- Stripping is ze's own forwarding behaviour. RFC 7947 requires per-client import and export policy but does not mandate it, so a different route-server implementation may leave these tags in place.

<!-- source: internal/component/bgp/wireu/community.go — ParseCommunityPolicy, ShouldForwardTo, StripControlCommunities -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go — general forwarding rail; forward_rs.go — route-server fast path -->

## Redistribution

The top-level `redistribute` block moves selected routes between protocol RIBs. Redistribution is separate from ordinary per-peer BGP filtering, but the resulting routes still pass through the destination protocol's policy and loop-prevention rules.

Use named destinations and specify the source protocols deliberately. Avoid broad two-way redistribution unless the topology and loop controls have been proven. Connected, static, kernel, OSPF, IS-IS, and BGP route sources have different ownership and withdrawal behaviour.

See [Route filters and redistribution](redistribution.md) for the current chain rules and configuration syntax.

## Verify policy

Use the same route at each boundary:

```console
ze cli -c "show bgp rib peer transit-a"
ze cli -c "show bgp rib best"
ze cli -c "show bgp irr check peer transit-a prefix 192.0.2.0/24"
ze cli -c "show bgp peer transit-b rib"
```

Confirm:

1. The route is present or absent at the expected import boundary.
2. The best-path result carries the intended local attributes.
3. Exported attributes match the destination peer's policy.
4. A rejected route does not reappear through redistribution.
5. A policy reload completes without warnings or partial application.

## Related guides

- [BGP peering](bgp-peering.md)
- [BGP resilience](bgp-resilience.md)
- [IRR filtering](irr-filtering.md)
- [RPKI origin validation](rpki.md)
- [Route injection](route-injection.md)
